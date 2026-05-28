PYTHON ?= python3

.PHONY: test test-go test-python lint benchmark run-collector run-api docker-up docker-down

test: test-go test-python

test-go:
	cd src/go-collector && go test ./...

test-python:
	cd src/python-pipeline && $(PYTHON) -m pytest

lint:
	cd src/go-collector && gofmt -w .
	cd src/python-pipeline && $(PYTHON) -m ruff check . ../../tests/python && $(PYTHON) -m ruff format . ../../tests/python

benchmark:
	cd src/python-pipeline && $(PYTHON) -m climate_pipeline.benchmark

run-collector:
	cd src/go-collector && go run ./cmd/collector

run-api:
	cd src/python-pipeline && uvicorn climate_pipeline.api.main:create_app --factory --host $${PYTHON_API_HOST:-0.0.0.0} --port $${PYTHON_API_PORT:-8000}

docker-up:
	docker compose up --build

docker-down:
	docker compose down --remove-orphans
