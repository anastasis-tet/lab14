from __future__ import annotations

from datetime import UTC, datetime, timedelta

import pytest

from climate_pipeline.models.validation import AggregateValidationError, build_aggregate


def valid_payload() -> dict[str, object]:
    start = datetime(2026, 5, 28, tzinfo=UTC)
    return {
        "window_start": start,
        "window_end": start + timedelta(hours=1),
        "category": "wildfires",
        "count": 3,
        "min_latitude": 10.0,
        "max_latitude": 20.0,
        "avg_latitude": 15.0,
    }


def test_build_aggregate_accepts_valid_payload() -> None:
    aggregate = build_aggregate(valid_payload())

    assert aggregate.category == "wildfires"
    assert aggregate.count == 3
    assert aggregate.window_end > aggregate.window_start


def test_build_aggregate_rejects_missing_category() -> None:
    payload = valid_payload()
    payload["category"] = ""

    with pytest.raises(AggregateValidationError):
        build_aggregate(payload)


def test_build_aggregate_rejects_naive_datetime() -> None:
    payload = valid_payload()
    payload["window_start"] = datetime(2026, 5, 28)

    with pytest.raises(AggregateValidationError):
        build_aggregate(payload)

