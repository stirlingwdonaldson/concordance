SHELL := /bin/bash

GO_SERVICE := services/api-go
PY_SERVICE := services/nlp-python
WEB_APP := apps/desktop-tauri/web
DOCKER_COMPOSE := infra/docker/docker-compose.dev.yml
NEXT_LOCALSTORAGE_FILE := .concordance-data/next-localstorage.json
NLP_VENV := .concordance-data/nlp-venv

.PHONY: test-go
test-go:
	cd $(GO_SERVICE) && go test ./...

.PHONY: test-go-integration
test-go-integration:
	cd $(GO_SERVICE) && TEST_DATABASE_URL=$$TEST_DATABASE_URL go test -p 1 ./... -run 'PostgresStore|ProjectUploadStatusFlow|LargeDocument|DuplicateUpload|MessyEncodings'

.PHONY: check-py
check-py:
	python3 -m py_compile $(PY_SERVICE)/server/*.py $(PY_SERVICE)/pipelines/*.py

.PHONY: check
check: test-go check-py check-web

.PHONY: check-web
check-web:
	cd $(WEB_APP) && bun run check

.PHONY: dev-db-up
dev-db-up:
	docker compose -f $(DOCKER_COMPOSE) up -d

.PHONY: dev-db-down
dev-db-down:
	docker compose -f $(DOCKER_COMPOSE) down

.PHONY: run-api
run-api:
	cd $(GO_SERVICE) && go run ./cmd/api

.PHONY: run-web
run-web:
	mkdir -p .concordance-data
	touch $(NEXT_LOCALSTORAGE_FILE)
	cd $(WEB_APP) && NODE_OPTIONS="--localstorage-file=$(abspath $(NEXT_LOCALSTORAGE_FILE))" bun run dev

.PHONY: setup-nlp-sidecar
setup-nlp-sidecar:
	mkdir -p .concordance-data
	if [[ ! -x "$(NLP_VENV)/bin/python3" ]]; then python3 -m venv $(NLP_VENV); fi
	$(NLP_VENV)/bin/python3 -m pip install --quiet --disable-pip-version-check -r $(PY_SERVICE)/requirements.txt

.PHONY: run-dev
run-dev:
	@set -euo pipefail; \
	ENV_FILE=".env"; \
	if [[ ! -f "$$ENV_FILE" ]]; then ENV_FILE=".env.example"; fi; \
	echo "Using $$ENV_FILE"; \
	$(MAKE) setup-nlp-sidecar; \
	set -a; . "$$ENV_FILE"; set +a; \
	export NLP_SIDECAR_CMD="$(abspath $(NLP_VENV)/bin/python3)"; \
	docker compose -f $(DOCKER_COMPOSE) up -d --remove-orphans; \
	mkdir -p .concordance-data; \
	touch $(NEXT_LOCALSTORAGE_FILE); \
	cleanup() { \
		echo "Stopping local services..."; \
		if [[ -n "$${API_PID:-}" ]] && kill -0 $$API_PID 2>/dev/null; then pkill -TERM -P $$API_PID 2>/dev/null || true; kill $$API_PID 2>/dev/null || true; fi; \
		if [[ -n "$${WEB_PID:-}" ]] && kill -0 $$WEB_PID 2>/dev/null; then pkill -TERM -P $$WEB_PID 2>/dev/null || true; kill $$WEB_PID 2>/dev/null || true; fi; \
		wait $${API_PID:-} 2>/dev/null || true; \
		wait $${WEB_PID:-} 2>/dev/null || true; \
		docker compose -f $(DOCKER_COMPOSE) down --remove-orphans; \
	}; \
	trap cleanup EXIT INT TERM; \
	( cd $(GO_SERVICE) && go run ./cmd/api ) & API_PID=$$!; \
	( cd $(WEB_APP) && NODE_OPTIONS="--localstorage-file=$(abspath $(NEXT_LOCALSTORAGE_FILE))" bun run dev ) & WEB_PID=$$!; \
	echo "API PID=$$API_PID, WEB PID=$$WEB_PID"; \
	while true; do \
		if ! kill -0 $$API_PID 2>/dev/null; then break; fi; \
		if ! kill -0 $$WEB_PID 2>/dev/null; then break; fi; \
		sleep 1; \
	done
