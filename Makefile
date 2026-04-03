GO_SERVICE := services/api-go
PY_SERVICE := services/nlp-python
WEB_APP := apps/desktop-tauri/web
DOCKER_COMPOSE := infra/docker/docker-compose.dev.yml

.PHONY: test-go
test-go:
	cd $(GO_SERVICE) && go test ./...

.PHONY: test-go-integration
test-go-integration:
	cd $(GO_SERVICE) && TEST_DATABASE_URL=$$TEST_DATABASE_URL go test -p 1 ./... -run 'PostgresStore|ProjectUploadStatusFlow'

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
