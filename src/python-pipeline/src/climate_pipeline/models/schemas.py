from __future__ import annotations

from datetime import datetime
from typing import Annotated

from pydantic import BaseModel, ConfigDict, Field, HttpUrl


class ClimateAggregate(BaseModel):
    """Validated aggregate received from the Go collector."""

    model_config = ConfigDict(str_strip_whitespace=True)

    window_start: datetime
    window_end: datetime
    category: Annotated[str, Field(min_length=1, max_length=80)]
    count: Annotated[int, Field(ge=0)]
    min_latitude: Annotated[float, Field(ge=-90, le=90)]
    max_latitude: Annotated[float, Field(ge=-90, le=90)]
    avg_latitude: Annotated[float, Field(ge=-90, le=90)]


class AnalyzeRequest(BaseModel):
    """Request for loading Arrow data from the Go collector."""

    source_url: HttpUrl


class SummaryRow(BaseModel):
    """Aggregated summary row returned by the API."""

    category: str
    total_events: int
    windows_count: int
    avg_events_per_window: float


class AnalysisResponse(BaseModel):
    """High-level analysis result."""

    rows: list[SummaryRow]
    parquet_path: str | None = None


class HealthResponse(BaseModel):
    """Service health response."""

    status: str = "ok"

