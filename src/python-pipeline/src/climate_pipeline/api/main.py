from __future__ import annotations

from collections.abc import AsyncIterator
from contextlib import asynccontextmanager
from pathlib import Path
from typing import Annotated

from fastapi import Depends, FastAPI, WebSocket, WebSocketDisconnect

from climate_pipeline.clients.arrow_client import ArrowClient
from climate_pipeline.models.schemas import (
    AnalysisResponse,
    AnalyzeRequest,
    HealthResponse,
    SummaryRow,
)
from climate_pipeline.repositories.parquet_repository import ParquetRepository
from climate_pipeline.services.analyzer import ClimateAnalyzer
from climate_pipeline.telemetry.tracing import configure_tracing


class AppState:
    """Mutable API state owned by FastAPI lifespan."""

    def __init__(self) -> None:
        self.summary: list[SummaryRow] = []
        self.parquet_path: str | None = None


def get_state(app: FastAPI) -> AppState:
    return app.state.climate_state


def get_arrow_client() -> ArrowClient:
    return ArrowClient()


def get_analyzer() -> ClimateAnalyzer:
    repository = ParquetRepository(Path("data"))
    return ClimateAnalyzer(repository)


ArrowClientDependency = Annotated[ArrowClient, Depends(get_arrow_client)]
AnalyzerDependency = Annotated[ClimateAnalyzer, Depends(get_analyzer)]


def create_app() -> FastAPI:
    """Create FastAPI app without module-level mutable state."""

    @asynccontextmanager
    async def lifespan(app: FastAPI) -> AsyncIterator[None]:
        configure_tracing("lab14-python-pipeline")
        app.state.climate_state = AppState()
        yield

    app = FastAPI(title="Lab14 Climate Pipeline", version="0.1.0", lifespan=lifespan)

    @app.get("/health", response_model=HealthResponse)
    async def health() -> HealthResponse:
        return HealthResponse()

    @app.get("/api/v1/summary", response_model=AnalysisResponse)
    async def summary() -> AnalysisResponse:
        state = get_state(app)
        return AnalysisResponse(rows=state.summary, parquet_path=state.parquet_path)

    @app.post("/api/v1/analyze", response_model=AnalysisResponse)
    async def analyze(
        request: AnalyzeRequest,
        arrow_client: ArrowClientDependency,
        analyzer: AnalyzerDependency,
    ) -> AnalysisResponse:
        aggregates = await arrow_client.fetch(str(request.source_url))
        frame = analyzer.to_frame(aggregates)
        parquet_path = analyzer.persist(frame)
        rows = analyzer.summarize(frame)
        state = get_state(app)
        state.summary = rows
        state.parquet_path = str(parquet_path)
        return AnalysisResponse(rows=rows, parquet_path=str(parquet_path))

    @app.websocket("/ws/summary")
    async def websocket_summary(websocket: WebSocket) -> None:
        await websocket.accept()
        try:
            state = get_state(app)
            response = AnalysisResponse(rows=state.summary, parquet_path=state.parquet_path)
            await websocket.send_json(response.model_dump())
        except WebSocketDisconnect:
            return

    return app
