from __future__ import annotations

from datetime import UTC, datetime, timedelta

from climate_pipeline.models.schemas import ClimateAggregate
from climate_pipeline.repositories.parquet_repository import ParquetRepository
from climate_pipeline.services.analyzer import ClimateAnalyzer


def make_aggregate(category: str, count: int) -> ClimateAggregate:
    start = datetime(2026, 5, 28, tzinfo=UTC)
    return ClimateAggregate(
        window_start=start,
        window_end=start + timedelta(hours=1),
        category=category,
        count=count,
        min_latitude=1.0,
        max_latitude=5.0,
        avg_latitude=3.0,
    )


def test_analyzer_summarizes_by_category(tmp_path) -> None:
    analyzer = ClimateAnalyzer(ParquetRepository(tmp_path))
    frame = analyzer.to_frame(
        [
            make_aggregate("wildfires", 2),
            make_aggregate("wildfires", 5),
            make_aggregate("severeStorms", 3),
        ]
    )

    summary = analyzer.summarize(frame)

    assert {row.category for row in summary} == {"wildfires", "severeStorms"}
    wildfires = next(row for row in summary if row.category == "wildfires")
    assert wildfires.total_events == 7
    assert wildfires.windows_count == 2


def test_analyzer_persists_parquet_and_duckdb_reads_it(tmp_path) -> None:
    analyzer = ClimateAnalyzer(ParquetRepository(tmp_path))
    frame = analyzer.to_frame([make_aggregate("wildfires", 2)])

    parquet_path = analyzer.persist(frame)
    rows, elapsed = analyzer.query_with_duckdb(parquet_path)

    assert parquet_path.exists()
    assert rows[0].category == "wildfires"
    assert elapsed >= 0

