from __future__ import annotations

from fastapi.testclient import TestClient

from climate_pipeline.api.main import create_app


def test_health_endpoint_returns_ok() -> None:
    with TestClient(create_app()) as client:
        response = client.get("/health")

    assert response.status_code == 200
    assert response.json() == {"status": "ok"}


def test_summary_endpoint_returns_empty_initial_state() -> None:
    with TestClient(create_app()) as client:
        response = client.get("/api/v1/summary")

    assert response.status_code == 200
    assert response.json() == {"rows": [], "parquet_path": None}

