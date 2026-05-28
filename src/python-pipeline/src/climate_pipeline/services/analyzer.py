from __future__ import annotations

import time
from pathlib import Path

import duckdb
import polars as pl

from climate_pipeline.models.schemas import ClimateAggregate, SummaryRow
from climate_pipeline.repositories.parquet_repository import ParquetRepository


class ClimateAnalyzer:
    """Transforms, stores and analyzes climate aggregates."""

    def __init__(self, repository: ParquetRepository) -> None:
        self._repository = repository

    def to_frame(self, aggregates: list[ClimateAggregate]) -> pl.DataFrame:
        rows = [aggregate.model_dump(mode="json") for aggregate in aggregates]
        frame = pl.DataFrame(rows)
        if frame.is_empty():
            return pl.DataFrame(
                schema={
                    "window_start": pl.String,
                    "window_end": pl.String,
                    "category": pl.String,
                    "count": pl.Int64,
                    "min_latitude": pl.Float64,
                    "max_latitude": pl.Float64,
                    "avg_latitude": pl.Float64,
                }
            )
        return frame.unique().with_columns(
            pl.col("count").cast(pl.Int64),
            pl.col("avg_latitude").cast(pl.Float64),
        )

    def summarize(self, frame: pl.DataFrame) -> list[SummaryRow]:
        if frame.is_empty():
            return []
        summary = (
            frame.group_by("category")
            .agg(
                pl.col("count").sum().alias("total_events"),
                pl.len().alias("windows_count"),
                pl.col("count").mean().alias("avg_events_per_window"),
            )
            .sort("total_events", descending=True)
        )
        return [SummaryRow.model_validate(row) for row in summary.to_dicts()]

    def persist(self, frame: pl.DataFrame) -> Path:
        return self._repository.save(frame)

    def query_with_duckdb(self, parquet_path: Path) -> tuple[list[SummaryRow], float]:
        started_at = time.perf_counter()
        connection = duckdb.connect()
        try:
            rows = connection.execute(
                """
                SELECT
                    category,
                    SUM(count)::INTEGER AS total_events,
                    COUNT(*)::INTEGER AS windows_count,
                    AVG(count)::DOUBLE AS avg_events_per_window
                FROM read_parquet($path)
                WHERE count >= 0
                GROUP BY category
                ORDER BY total_events DESC
                """,
                {"path": str(parquet_path)},
            ).fetchall()
        finally:
            connection.close()
        elapsed = time.perf_counter() - started_at
        result = [
            SummaryRow(
                category=row[0],
                total_events=row[1],
                windows_count=row[2],
                avg_events_per_window=row[3],
            )
            for row in rows
        ]
        return result, elapsed
