from __future__ import annotations

from pathlib import Path

import polars as pl


class ParquetRepository:
    """Persists analytical dataframes as Parquet files."""

    def __init__(self, output_dir: Path) -> None:
        self._output_dir = output_dir

    def save(self, frame: pl.DataFrame, filename: str = "climate_aggregates.parquet") -> Path:
        self._output_dir.mkdir(parents=True, exist_ok=True)
        path = self._output_dir / filename
        frame.write_parquet(path)
        return path

