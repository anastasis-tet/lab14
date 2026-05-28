from __future__ import annotations

from opentelemetry import trace
from opentelemetry.sdk.trace import TracerProvider


def configure_tracing(service_name: str) -> None:
    """Configure OpenTelemetry tracing provider once for local spans."""

    provider = trace.get_tracer_provider()
    if isinstance(provider, TracerProvider):
        return
    trace.set_tracer_provider(TracerProvider())

