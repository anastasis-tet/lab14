from __future__ import annotations

from datetime import datetime

from pydantic import ValidationError

from climate_pipeline.models.schemas import ClimateAggregate

try:
    from climate_validator import validate_aggregate as rust_validate_aggregate
except ImportError:
    rust_validate_aggregate = None


class AggregateValidationError(ValueError):
    """Raised when aggregate validation fails."""


class AggregateValidator:
    """Validates climate aggregates through Rust/PyO3 when available."""

    def validate(self, aggregate: ClimateAggregate) -> ClimateAggregate:
        if rust_validate_aggregate is None:
            return aggregate
        try:
            rust_validate_aggregate(
                aggregate.category,
                aggregate.count,
                aggregate.min_latitude,
                aggregate.max_latitude,
                aggregate.avg_latitude,
                aggregate.window_start.isoformat(),
                aggregate.window_end.isoformat(),
            )
        except ValueError as exc:
            raise AggregateValidationError(str(exc)) from exc
        return aggregate


def build_aggregate(data: dict[str, object]) -> ClimateAggregate:
    """Build a timezone-aware aggregate model from raw dict data."""

    try:
        aggregate = ClimateAggregate.model_validate(data)
    except ValidationError as exc:
        raise AggregateValidationError(str(exc)) from exc
    if aggregate.window_start.tzinfo is None or aggregate.window_end.tzinfo is None:
        raise AggregateValidationError("window timestamps must be timezone-aware")
    if aggregate.window_end <= aggregate.window_start:
        raise AggregateValidationError("window_end must be greater than window_start")
    return aggregate


def ensure_datetime(value: datetime) -> datetime:
    """Return datetime only when it is timezone-aware."""

    if value.tzinfo is None:
        raise AggregateValidationError("datetime must be timezone-aware")
    return value

