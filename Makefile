GO_SERVICE := services/api-go
PY_SERVICE := services/nlp-python
WEB_APP := apps/desktop-tauri/web

.PHONY: test-go
test-go:
	cd $(GO_SERVICE) && go test ./...

.PHONY: test-go-integration
test-go-integration:
	cd $(GO_SERVICE) && TEST_DATABASE_URL=$$TEST_DATABASE_URL go test ./... -run PostgresStore

.PHONY: check-py
check-py:
	python3 -m py_compile $(PY_SERVICE)/server/*.py $(PY_SERVICE)/pipelines/*.py

.PHONY: check
check: test-go check-py check-web

.PHONY: check-web
check-web:
	cd $(WEB_APP) && bun run check
