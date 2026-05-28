from __future__ import annotations

import asyncio
import time
from dataclasses import dataclass
from typing import Any

import aiohttp


@dataclass(frozen=True)
class BenchmarkResult:
    collector: str
    categories: int
    events: int
    elapsed_seconds: float


class AsyncEONETCollector:
    """Python asyncio collector used only for Go vs Python comparison."""

    def __init__(self, base_url: str, timeout_seconds: float = 10.0) -> None:
        self._base_url = base_url.rstrip("/")
        self._timeout = aiohttp.ClientTimeout(total=timeout_seconds)

    async def collect(
        self,
        categories: list[str],
        days: int,
        status: str = "all",
    ) -> BenchmarkResult:
        started_at = time.perf_counter()
        async with aiohttp.ClientSession(timeout=self._timeout) as session:
            tasks = [
                self._fetch_category(session, category, days, status)
                for category in categories
            ]
            responses = await asyncio.gather(*tasks)
        elapsed = time.perf_counter() - started_at
        return BenchmarkResult(
            collector="python-asyncio",
            categories=len(categories),
            events=sum(len(response.get("events", [])) for response in responses),
            elapsed_seconds=elapsed,
        )

    async def _fetch_category(
        self,
        session: aiohttp.ClientSession,
        category: str,
        days: int,
        status: str,
    ) -> dict[str, Any]:
        async with session.get(
            f"{self._base_url}/events",
            params={"category": category, "days": days, "status": status, "limit": 500},
        ) as response:
            response.raise_for_status()
            return await response.json()
