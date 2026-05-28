from __future__ import annotations

import argparse
import asyncio
import html
import json
import os
import resource
import subprocess
import sys
import tempfile
import time
import tracemalloc
from collections.abc import Callable
from dataclasses import asdict, dataclass
from datetime import UTC, datetime, timedelta
from pathlib import Path
from typing import Any

from aiohttp import web

from climate_pipeline.services.python_collector import AsyncEONETCollector


@dataclass(frozen=True)
class BenchmarkConfig:
    """Configuration shared by both collectors during one benchmark run."""

    categories: tuple[str, ...]
    days: int
    status: str
    iterations: int
    events_per_category: int
    request_delay_seconds: float
    timeout_seconds: float
    output_dir: Path


@dataclass(frozen=True)
class CollectorMeasurement:
    """Normalized benchmark result for report generation."""

    collector: str
    requests: int
    events: int
    elapsed_seconds: float
    events_per_second: float
    cpu_seconds: float
    peak_memory_mb: float


async def run_benchmark(config: BenchmarkConfig) -> list[CollectorMeasurement]:
    """Run Go and Python collectors against the same deterministic EONET mock."""

    mock_server = MockEONETServer(
        categories=config.categories,
        events_per_category=config.events_per_category,
        delay_seconds=config.request_delay_seconds,
    )
    base_url = await mock_server.start()
    try:
        python_measurement = await measure_python_collector(config, base_url)
        go_measurement = await asyncio.to_thread(measure_go_collector, config, base_url)
        measurements = [go_measurement, python_measurement]
        write_artifacts(config, measurements)
        return measurements
    finally:
        await mock_server.close()


async def measure_python_collector(
    config: BenchmarkConfig,
    base_url: str,
) -> CollectorMeasurement:
    """Measure asyncio/aiohttp collector speed, CPU and Python heap peak."""

    collector = AsyncEONETCollector(base_url, timeout_seconds=config.timeout_seconds)
    start_usage = resource.getrusage(resource.RUSAGE_SELF)
    tracemalloc.start()
    started_at = time.perf_counter()
    events = 0
    for _ in range(config.iterations):
        result = await collector.collect(
            categories=list(config.categories),
            days=config.days,
            status=config.status,
        )
        events += result.events
    elapsed = time.perf_counter() - started_at
    _, peak_bytes = tracemalloc.get_traced_memory()
    tracemalloc.stop()
    end_usage = resource.getrusage(resource.RUSAGE_SELF)

    return CollectorMeasurement(
        collector="Python asyncio/aiohttp",
        requests=len(config.categories) * config.iterations,
        events=events,
        elapsed_seconds=elapsed,
        events_per_second=safe_rate(events, elapsed),
        cpu_seconds=cpu_delta(start_usage, end_usage),
        peak_memory_mb=bytes_to_mb(peak_bytes),
    )


def measure_go_collector(config: BenchmarkConfig, base_url: str) -> CollectorMeasurement:
    """Build and measure the Go benchmark command against the mock API."""

    repo_root = find_repo_root(Path(__file__))
    go_dir = repo_root / "src" / "go-collector"

    with tempfile.TemporaryDirectory(prefix="lab14-go-benchmark-") as tmp:
        go_env = os.environ.copy()
        go_env.setdefault("GOCACHE", str(Path(tmp) / "go-build-cache"))
        go_env.setdefault("GOMODCACHE", str(Path(tmp) / "go-mod-cache"))

        binary_path = Path(tmp) / "go-eonet-benchmark"
        build_command = ["go", "build", "-o", str(binary_path), "./cmd/benchmark"]
        subprocess.run(
            build_command,
            cwd=go_dir,
            env=go_env,
            check=True,
            capture_output=True,
            text=True,
        )

        command = [
            str(binary_path),
            "-base-url",
            base_url,
            "-categories",
            ",".join(config.categories),
            "-days",
            str(config.days),
            "-status",
            config.status,
            "-iterations",
            str(config.iterations),
            "-timeout-seconds",
            str(int(config.timeout_seconds)),
        ]
        start_usage = resource.getrusage(resource.RUSAGE_CHILDREN)
        process = subprocess.Popen(
            command,
            cwd=go_dir,
            env=go_env,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
        )
        peak_memory_mb = sample_child_peak_memory(process)
        stdout, stderr = process.communicate()
        end_usage = resource.getrusage(resource.RUSAGE_CHILDREN)
        if process.returncode != 0:
            raise RuntimeError(f"Go benchmark failed: {stderr.strip()}")
        if peak_memory_mb <= 0:
            peak_memory_mb = max_rss_to_mb(end_usage.ru_maxrss)

    payload = json.loads(stdout)
    elapsed = float(payload["elapsed_seconds"])
    events = int(payload["events"])
    return CollectorMeasurement(
        collector="Go goroutines",
        requests=int(payload["requests"]),
        events=events,
        elapsed_seconds=elapsed,
        events_per_second=safe_rate(events, elapsed),
        cpu_seconds=cpu_delta(start_usage, end_usage),
        peak_memory_mb=peak_memory_mb,
    )


class MockEONETServer:
    """Small deterministic EONET-compatible HTTP API used by both collectors."""

    def __init__(
        self,
        categories: tuple[str, ...],
        events_per_category: int,
        delay_seconds: float,
    ) -> None:
        self._categories = categories
        self._events_per_category = events_per_category
        self._delay_seconds = delay_seconds
        self._runner: web.AppRunner | None = None
        self._site: web.TCPSite | None = None

    async def start(self) -> str:
        app = web.Application()
        app.router.add_get("/events", self._handle_events)
        self._runner = web.AppRunner(app)
        await self._runner.setup()
        self._site = web.TCPSite(self._runner, "127.0.0.1", 0)
        await self._site.start()
        if self._site._server is None:
            raise RuntimeError("mock EONET server did not expose a socket")
        socket = self._site._server.sockets[0]
        host, port = socket.getsockname()[:2]
        return f"http://{host}:{port}"

    async def close(self) -> None:
        if self._runner is not None:
            await self._runner.cleanup()

    async def _handle_events(self, request: web.Request) -> web.Response:
        category = request.query.get("category", self._categories[0])
        if category not in self._categories:
            return web.json_response({"events": []})
        if self._delay_seconds > 0:
            await asyncio.sleep(self._delay_seconds)
        return web.json_response({"events": self._events_for(category)})

    def _events_for(self, category: str) -> list[dict[str, Any]]:
        base_date = datetime(2026, 1, 1, tzinfo=UTC)
        category_offset = self._categories.index(category) * 3
        events: list[dict[str, Any]] = []
        for index in range(self._events_per_category):
            latitude = -70.0 + ((index + category_offset) % 140)
            longitude = -160.0 + ((index * 2 + category_offset) % 320)
            occurred_at = base_date - timedelta(hours=index)
            events.append(
                {
                    "id": f"{category}-{index}",
                    "title": f"Mock climate event {category} #{index}",
                    "categories": [{"id": category}],
                    "sources": [{"id": "MOCK-EONET"}],
                    "geometry": [
                        {
                            "date": occurred_at.isoformat().replace("+00:00", "Z"),
                            "coordinates": [longitude, latitude],
                        }
                    ],
                }
            )
        return events


def write_artifacts(config: BenchmarkConfig, measurements: list[CollectorMeasurement]) -> None:
    """Write JSON results, SVG charts and the Markdown report."""

    charts_dir = config.output_dir / "charts"
    charts_dir.mkdir(parents=True, exist_ok=True)

    results_path = config.output_dir / "benchmark_results.json"
    results_path.write_text(
        json.dumps([asdict(measurement) for measurement in measurements], indent=2),
        encoding="utf-8",
    )

    chart_specs: list[tuple[str, str, str, Callable[[CollectorMeasurement], float]]] = [
        (
            "elapsed_seconds.svg",
            "Время выполнения",
            "секунды",
            lambda item: item.elapsed_seconds,
        ),
        (
            "events_per_second.svg",
            "Скорость сбора",
            "событий/сек",
            lambda item: item.events_per_second,
        ),
        ("cpu_seconds.svg", "CPU time", "секунды CPU", lambda item: item.cpu_seconds),
        (
            "peak_memory_mb.svg",
            "Пиковая память",
            "MB",
            lambda item: item.peak_memory_mb,
        ),
    ]
    for filename, title, unit, value_getter in chart_specs:
        write_bar_chart(charts_dir / filename, title, unit, measurements, value_getter)

    report_path = config.output_dir / "go_vs_python_benchmark.md"
    report_path.write_text(render_report(config, measurements), encoding="utf-8")


def render_report(config: BenchmarkConfig, measurements: list[CollectorMeasurement]) -> str:
    """Render a compact report with workload, metrics and chart links."""

    generated_at = datetime.now(UTC).isoformat(timespec="seconds").replace("+00:00", "Z")
    rows = "\n".join(
        (
            f"| {item.collector} | {item.requests} | {item.events} | "
            f"{item.elapsed_seconds:.4f} | {item.events_per_second:.2f} | "
            f"{item.cpu_seconds:.4f} | {item.peak_memory_mb:.2f} |"
        )
        for item in measurements
    )
    fastest = min(measurements, key=lambda item: item.elapsed_seconds)

    requests_count = len(config.categories) * config.iterations

    return f"""# Benchmark Go vs Python для задания 6

Дата генерации: {generated_at}

## Нагрузка

- Источник данных: локальный mock NASA EONET API с endpoint `/events`.
- Категории: {", ".join(config.categories)}.
- Итерации: {config.iterations}.
- Событий на категорию: {config.events_per_category}.
- HTTP-запросов на каждый коллектор: {requests_count}.
- Задержка ответа mock API: {config.request_delay_seconds:.3f} секунды.
- Общие параметры: `days={config.days}`, `status={config.status}`.

## Результаты

| Tool | Requests | Events | Time | Events/s | CPU | MB |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
{rows}

Самый быстрый запуск по wall-clock времени: **{fastest.collector}**.

## Графики

![Время выполнения](charts/elapsed_seconds.svg)

![Скорость сбора](charts/events_per_second.svg)

![CPU time](charts/cpu_seconds.svg)

![Пиковая память](charts/peak_memory_mb.svg)

## Как воспроизвести

```bash
make benchmark
```

Скрипт поднимает mock API и прогоняет оба коллектора.
Затем он обновляет отчёт, JSON-результаты и SVG-графики.
"""


def write_bar_chart(
    path: Path,
    title: str,
    unit: str,
    measurements: list[CollectorMeasurement],
    value_getter: Callable[[CollectorMeasurement], float],
) -> None:
    """Write a dependency-free SVG bar chart for the benchmark report."""

    width = 760
    height = 420
    margin_left = 170
    margin_right = 48
    margin_top = 66
    margin_bottom = 72
    plot_width = width - margin_left - margin_right
    plot_height = height - margin_top - margin_bottom
    values = [max(0.0, value_getter(item)) for item in measurements]
    max_value = max(values) if values else 1.0
    if max_value == 0:
        max_value = 1.0

    bar_gap = 30
    bar_height = (plot_height - bar_gap * (len(measurements) - 1)) / len(measurements)
    bars: list[str] = []
    palette = ["#2563eb", "#16a34a", "#dc2626", "#9333ea"]
    for index, item in enumerate(measurements):
        value = values[index]
        bar_width = (value / max_value) * plot_width
        y = margin_top + index * (bar_height + bar_gap)
        label = html.escape(item.collector)
        formatted = f"{value:.2f} {unit}"
        bars.append(
            f'<text x="24" y="{y + bar_height / 2 + 5:.1f}" '
            f'font-size="16" fill="#111827">{label}</text>'
        )
        bars.append(
            f'<rect x="{margin_left}" y="{y:.1f}" width="{bar_width:.1f}" '
            f'height="{bar_height:.1f}" rx="6" fill="{palette[index % len(palette)]}" />'
        )
        bars.append(
            f'<text x="{margin_left + bar_width + 10:.1f}" y="{y + bar_height / 2 + 5:.1f}" '
            f'font-size="15" fill="#374151">{html.escape(formatted)}</text>'
        )

    view_box = f"0 0 {width} {height}"
    axis_y = margin_top + plot_height
    axis_x2 = margin_left + plot_width
    svg = f"""<svg xmlns="http://www.w3.org/2000/svg"
  width="{width}" height="{height}" viewBox="{view_box}">
  <rect width="100%" height="100%" fill="#ffffff"/>
  <text x="24" y="38" font-size="24" font-weight="700" fill="#111827">{html.escape(title)}</text>
  <text x="24" y="392" font-size="13" fill="#6b7280">Единица: {html.escape(unit)}</text>
  <line x1="{margin_left}" y1="{axis_y}" x2="{axis_x2}" y2="{axis_y}" stroke="#d1d5db"/>
  {"".join(bars)}
</svg>
"""
    path.write_text(svg, encoding="utf-8")


def sample_child_peak_memory(process: subprocess.Popen[str]) -> float:
    """Poll child process RSS and return peak memory in megabytes."""

    peak_mb = 0.0
    while process.poll() is None:
        peak_mb = max(peak_mb, read_process_rss_mb(process.pid))
        time.sleep(0.02)
    return max(peak_mb, read_process_rss_mb(process.pid))


def read_process_rss_mb(pid: int) -> float:
    """Read RSS for a process using /proc on Linux and ps as portable fallback."""

    status_path = Path(f"/proc/{pid}/status")
    if status_path.exists():
        for line in status_path.read_text(encoding="utf-8").splitlines():
            if line.startswith("VmRSS:"):
                parts = line.split()
                if len(parts) >= 2:
                    return float(parts[1]) / 1024.0

    completed = subprocess.run(
        ["ps", "-o", "rss=", "-p", str(pid)],
        check=False,
        capture_output=True,
        text=True,
    )
    value = completed.stdout.strip()
    if not value:
        return 0.0
    return float(value.splitlines()[0].strip()) / 1024.0


def cpu_delta(start: resource.struct_rusage, end: resource.struct_rusage) -> float:
    """Return user + system CPU seconds between two resource snapshots."""

    return (end.ru_utime + end.ru_stime) - (start.ru_utime + start.ru_stime)


def bytes_to_mb(value: int) -> float:
    return value / (1024 * 1024)


def max_rss_to_mb(value: int) -> float:
    if sys.platform == "darwin":
        return value / (1024 * 1024)
    return value / 1024


def safe_rate(events: int, elapsed_seconds: float) -> float:
    if elapsed_seconds <= 0:
        return 0.0
    return events / elapsed_seconds


def find_repo_root(start: Path) -> Path:
    """Find repository root by walking upward to README and Makefile."""

    for candidate in [start, *start.parents]:
        if (candidate / "README.md").exists() and (candidate / "Makefile").exists():
            return candidate
    raise RuntimeError("repository root was not found")


def parse_args(argv: list[str]) -> BenchmarkConfig:
    parser = argparse.ArgumentParser(description="Compare Go and Python EONET collectors")
    parser.add_argument(
        "--categories",
        default="wildfires,severeStorms,drought,seaLakeIce,dustHaze",
        help="comma-separated categories used by both collectors",
    )
    parser.add_argument("--days", type=int, default=365)
    parser.add_argument("--status", default="all")
    parser.add_argument("--iterations", type=int, default=8)
    parser.add_argument("--events-per-category", type=int, default=250)
    parser.add_argument("--request-delay-seconds", type=float, default=0.01)
    parser.add_argument("--timeout-seconds", type=float, default=10.0)
    parser.add_argument(
        "--output-dir",
        type=Path,
        default=find_repo_root(Path(__file__)) / "reports" / "performance",
    )
    args = parser.parse_args(argv)

    categories = tuple(item.strip() for item in args.categories.split(",") if item.strip())
    if not categories:
        parser.error("--categories must contain at least one category")
    if args.days <= 0:
        parser.error("--days must be positive")
    if args.iterations <= 0:
        parser.error("--iterations must be positive")
    if args.events_per_category <= 0:
        parser.error("--events-per-category must be positive")
    if args.request_delay_seconds < 0:
        parser.error("--request-delay-seconds must be non-negative")
    if args.timeout_seconds <= 0:
        parser.error("--timeout-seconds must be positive")

    return BenchmarkConfig(
        categories=categories,
        days=args.days,
        status=args.status,
        iterations=args.iterations,
        events_per_category=args.events_per_category,
        request_delay_seconds=args.request_delay_seconds,
        timeout_seconds=args.timeout_seconds,
        output_dir=args.output_dir,
    )


async def async_main(argv: list[str]) -> int:
    config = parse_args(argv)
    measurements = await run_benchmark(config)
    for item in measurements:
        print(
            f"{item.collector}: {item.events} events, "
            f"{item.elapsed_seconds:.4f}s, {item.events_per_second:.2f} events/s"
        )
    print(f"Benchmark report written to {config.output_dir / 'go_vs_python_benchmark.md'}")
    return 0


def main() -> int:
    return asyncio.run(async_main(sys.argv[1:]))


if __name__ == "__main__":
    raise SystemExit(main())
