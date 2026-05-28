from __future__ import annotations

from collections.abc import Sequence
from io import BytesIO

import httpx
import pyarrow.ipc as ipc

from climate_pipeline.models.schemas import ClimateAggregate
from climate_pipeline.models.validation import AggregateValidator, build_aggregate


class ArrowClient:
    """Loads Apache Arrow IPC streams from the Go collector."""

    def __init__(self, timeout_seconds: float = 10.0) -> None:
        self._timeout = timeout_seconds
        self._validator = AggregateValidator()

    async def fetch(self, source_url: str) -> list[ClimateAggregate]:
        async with httpx.AsyncClient(timeout=self._timeout) as client:
            response = await client.get(source_url)
            response.raise_for_status()
        return self.parse(response.content)

    def parse(self, payload: bytes) -> list[ClimateAggregate]:
        reader = ipc.open_stream(BytesIO(payload))
        aggregates: list[ClimateAggregate] = []
        for batch in reader:
            table = batch.to_pydict()
            rows = self._rows_from_table(table)
            for row in rows:
                aggregate = self._validator.validate(build_aggregate(row))
                aggregates.append(aggregate)
        return aggregates

    @staticmethod
    def _rows_from_table(table: dict[str, Sequence[object]]) -> list[dict[str, object]]:
        if not table:
            return []
        size = len(next(iter(table.values())))
        return [
            {column: values[index] for column, values in table.items()}
            for index in range(size)
        ]
