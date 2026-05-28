# PROMPT_LOG

## AI-инструменты

- OpenAI Codex в локальном workspace.
- Веб-поиск по официальной документации NASA EONET v3.

## Исходный промпт

Пользователь попросил выполнить лабораторную работу №14 по варианту 25: анализ климатических изменений через NASA Earth Observatory API, обязательно повышенная сложность, код на GitHub, исходники в `src/`, тесты в `tests/`, README, Docker Compose, понятные conventional commits и production-like архитектура без монолитов.

## Что сгенерировано

- Go-сервис распределённого сбора NASA EONET событий.
- Координация шардов через etcd с fallback на in-memory coordinator.
- Оконная tumbling-window агрегация на стороне Go.
- Apache Arrow IPC endpoint для передачи агрегатов.
- NATS publisher для streaming pipeline.
- Python FastAPI pipeline: Arrow client, Polars/DuckDB analyzer, API endpoints.
- Rust/PyO3 validator с Python fallback.
- Dockerfile для Go и Python сервисов.
- `docker-compose.yml`, `.env.example`, `Makefile`, Kubernetes manifests.
- Тесты Go и Python для ключевой бизнес-логики.
- Запускаемый benchmark Go vs Python для задания 6: Go-команда `cmd/benchmark`,
  Python runner с локальным mock EONET API, JSON-результаты, Markdown-отчёт и SVG-графики.

## Runtime issues и исправления

- Первоначально рассматривался NASA POWER API, но пользователь уточнил, что нужен именно NASA Earth Observatory API. Решение: использовать NASA EONET v3 как официальный API Earth Observatory Natural Event Tracker.
- Локальная папка не разрешила создать `.git` обычным `git init`. Решение: использовать отдельный git-dir вне рабочей папки при создании истории коммитов, если ограничение сохранится.
- Валидация Rust/PyO3 может отсутствовать на машине проверяющего. Решение: добавить fallback-валидатор на Pydantic без изменения публичного API.
- Telegram review bot отклонил первую попытку из-за частичного выполнения задания 6:
  был только класс `AsyncEONETCollector`, но не было запускаемого сравнения Go vs Python
  и отчёта с графиками. Решение: добавить воспроизводимый benchmark на одинаковой
  mock-нагрузке, измерять wall-clock время, скорость, CPU и память, сохранять отчёт в Git.

## Что исправлялось вручную после генерации

- Разделение Go-кода на `client`, `coordination`, `aggregation`, `arrowserver`, `natsstream`, `config`, `app`.
- Разделение Python-кода на `api`, `clients`, `models`, `repositories`, `services`, `telemetry`.
- Добавлены тестируемые pure-функции вместо логики в handlers.
- Добавлен graceful shutdown, таймауты HTTP-сервера и централизованная конфигурация.
- Обновлён `.gitignore`: папка `reports/performance` разрешена к коммиту, чтобы
  benchmark-отчёт и графики задания 6 были доступны проверяющему боту.

## Улучшения после генерации

- Убраны хардкоды категорий в UI/API: категории передаются через конфигурацию.
- Добавлена деградация при недоступности etcd/NATS.
- Добавлены health endpoints и структурированные JSON-логи.
- Добавлены тесты negative/edge cases для конфигурации, валидации и агрегации.
- Добавлены тесты генерации benchmark-отчёта и команды `make benchmark` для
  воспроизводимого обновления результатов.
