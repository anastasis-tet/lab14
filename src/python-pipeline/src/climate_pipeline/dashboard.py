from __future__ import annotations

import os

import httpx
import plotly.express as px
import streamlit as st


def main() -> None:
    """Render a lightweight real-time dashboard."""

    api_url = os.getenv("CLIMATE_API_URL", "http://localhost:8000")
    st.set_page_config(page_title="Lab14 Climate Dashboard", layout="wide")
    st.title("NASA EONET climate events")

    response = httpx.get(f"{api_url}/api/v1/summary", timeout=5.0)
    response.raise_for_status()
    rows = response.json()["rows"]

    if not rows:
        st.info("Нет агрегированных данных. Запустите анализ через API.")
        return

    categories = [row["category"] for row in rows]
    values = [row["total_events"] for row in rows]
    st.plotly_chart(px.bar(x=categories, y=values, labels={"x": "Категория", "y": "События"}))
    st.dataframe(rows, use_container_width=True)


if __name__ == "__main__":
    main()

