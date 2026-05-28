from __future__ import annotations

from pathlib import Path

from climate_pipeline.benchmark.runner import (
    BenchmarkConfig,
    CollectorMeasurement,
    render_report,
    write_artifacts,
)


def build_config(output_dir: Path) -> BenchmarkConfig:
    return BenchmarkConfig(
        categories=("wildfires", "drought"),
        days=30,
        status="all",
        iterations=2,
        events_per_category=10,
        request_delay_seconds=0.0,
        timeout_seconds=5.0,
        output_dir=output_dir,
    )


def build_measurements() -> list[CollectorMeasurement]:
    return [
        CollectorMeasurement(
            collector="Go goroutines",
            requests=4,
            events=40,
            elapsed_seconds=0.2,
            events_per_second=200.0,
            cpu_seconds=0.05,
            peak_memory_mb=12.5,
        ),
        CollectorMeasurement(
            collector="Python asyncio/aiohttp",
            requests=4,
            events=40,
            elapsed_seconds=0.4,
            events_per_second=100.0,
            cpu_seconds=0.08,
            peak_memory_mb=9.5,
        ),
    ]


def test_render_report_describes_equal_workload() -> None:
    report = render_report(build_config(Path("reports/performance")), build_measurements())

    assert "HTTP-запросов на каждый коллектор: 4" in report
    assert "Go goroutines" in report
    assert "Python asyncio/aiohttp" in report
    assert "charts/elapsed_seconds.svg" in report


def test_write_artifacts_creates_report_json_and_svg_charts(tmp_path: Path) -> None:
    write_artifacts(build_config(tmp_path), build_measurements())

    assert (tmp_path / "go_vs_python_benchmark.md").exists()
    assert (tmp_path / "benchmark_results.json").exists()
    assert (tmp_path / "charts" / "elapsed_seconds.svg").exists()
    assert (tmp_path / "charts" / "events_per_second.svg").exists()
    assert (tmp_path / "charts" / "cpu_seconds.svg").exists()
    assert (tmp_path / "charts" / "peak_memory_mb.svg").exists()
