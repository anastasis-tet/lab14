# Лабораторная работа №14

**Студент:** Тетерина Анастасия  
**Группа:** 220032-11  
**Вариант:** 25  
**Тип варианта:** повышенная сложность  
**Тема:** Анализ климатических изменений  
**Источник данных:** NASA Earth Observatory Natural Event Tracker API (EONET v3)

## Описание программы

Проект реализует конвейер обработки данных для анализа климатически значимых природных событий по данным NASA EONET: лесные пожары, сильные штормы, засухи, ледовые явления, пыль и дым. EONET является API NASA Earth Observatory Natural Event Tracker; актуальная версия API v3 предоставляет endpoint `/api/v3/events` с фильтрами по категории, датам, статусу и лимиту записей.

Архитектура конвейера:

```text
Go-сборщики -> etcd coordination -> tumbling windows -> Apache Arrow HTTP endpoint
          \-> NATS stream -> Python analyzer -> Polars -> Parquet -> DuckDB -> FastAPI dashboard
                                      \-> Rust/PyO3 validation
```

## Используемые технологии

- Go 1.23+: параллельный сбор, graceful shutdown, structured logging, health endpoint.
- etcd: распределение шардов категорий между несколькими экземплярами сборщика.
- Apache Arrow IPC: передача агрегированных оконных данных из Go в Python.
- NATS JetStream: потоковая передача агрегатов для real-time обработки.
- Python 3.12+: FastAPI, Polars, DuckDB, PyArrow, Plotly.
- Rust + PyO3: библиотека валидации входных климатических событий.
- Docker Compose: локальный запуск etcd, NATS, Go-сборщика и Python API.
- Kubernetes manifests: Deployment, Service и HPA для автоскалирования сборщика.

## Структура проекта

```text
src/
  go-collector/       # распределённый Go-сборщик NASA EONET
  python-pipeline/    # анализ, API, dashboard и benchmark Python-сборщика
  rust-validator/     # PyO3-библиотека для валидации данных
tests/                # дополнительные интеграционные тесты
deploy/k8s/           # Kubernetes deployment/service/hpa
```

## Сборка проекта

```bash
cp .env.example .env
python3 -m venv .venv
source .venv/bin/activate
python -m pip install -e "src/python-pipeline[dev]"
make test
docker compose build
```

Для Rust-валидации в Python:

```bash
cd src/rust-validator
maturin develop
```

Если Rust-модуль не установлен, Python-анализатор использует безопасный Pydantic fallback, чтобы локальные тесты и демонстрация не зависели от нативной сборки.

## Запуск

Локальный запуск всей инфраструктуры:

```bash
cp .env.example .env
docker compose up --build
```

Запуск Go-сборщика без Docker:

```bash
cd src/go-collector
go run ./cmd/collector
```

Запуск Python API:

```bash
cd src/python-pipeline
uvicorn climate_pipeline.api.main:create_app --factory --host 0.0.0.0 --port 8000
```

Запуск сравнения производительности Go vs Python для задания 6:

```bash
make benchmark
```

Команда поднимает локальный mock NASA EONET API, запускает Go-сборщик и Python
asyncio/aiohttp-сборщик при одинаковой нагрузке, измеряет скорость, CPU и память,
после чего обновляет:

- `reports/performance/go_vs_python_benchmark.md` — отчёт с таблицей и графиками.
- `reports/performance/benchmark_results.json` — машинно-читаемые результаты.
- `reports/performance/charts/*.svg` — графики сравнения.

## Примеры запросов

Проверка Go-сборщика:

```bash
curl http://localhost:8080/health
curl http://localhost:8080/metrics
curl http://localhost:8080/arrow --output climate.arrow
```

Проверка Python API:

```bash
curl http://localhost:8000/health
curl http://localhost:8000/api/v1/summary
curl -X POST http://localhost:8000/api/v1/analyze \
  -H "Content-Type: application/json" \
  -d '{"source_url":"http://go-collector:8080/arrow"}'
```

## Повышенная сложность

- Распределённый сборщик на Go использует etcd для назначения категорий-шардов.
- Go выполняет tumbling-window агрегацию до передачи данных в Python.
- Агрегаты доступны через Apache Arrow IPC endpoint.
- Python принимает Arrow, валидирует данные через Rust/PyO3 или fallback, анализирует в Polars и сохраняет Parquet.
- DuckDB выполняет SQL-агрегации по Parquet.
- NATS используется для потоковой передачи агрегатов.
- FastAPI отдаёт real-time API для dashboard и WebSocket обновлений.
- Kubernetes HPA масштабирует Go-сборщик по CPU; NATS monitoring endpoint доступен для метрик очереди.
- Добавлен запускаемый benchmark Go vs Python: одинаковая mock-нагрузка EONET,
  замеры wall-clock времени, скорости сбора, CPU, памяти и отчёт с SVG-графиками.

## Источник

NASA EONET v3 API: https://eonet.gsfc.nasa.gov/docs/v3
