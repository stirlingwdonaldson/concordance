GO_SERVICE := services/api-go
PY_SERVICE := services/nlp-python

.PHONY: test-go
test-go:
	cd $(GO_SERVICE) && go test ./...

.PHONY: check-py
check-py:
	python3 -m py_compile $(PY_SERVICE)/server/*.py $(PY_SERVICE)/pipelines/*.py

.PHONY: check
check: test-go check-py
