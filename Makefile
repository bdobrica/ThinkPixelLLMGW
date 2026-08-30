.PHONY: verify verify-go verify-frontend verify-bff

BFF_PYTHON ?= $(if $(wildcard webui/bff/venv/bin/python),venv/bin/python,python3)

verify: verify-go verify-frontend verify-bff ## Run the hermetic repository verification gate

verify-go: ## Check Go formatting, vet, and unit tests
	@test -z "$$(gofmt -l llm_gateway)" || { gofmt -l llm_gateway; exit 1; }
	cd llm_gateway && go vet ./...
	cd llm_gateway && go test -short ./...

verify-frontend: ## Lint, test, and build the installed frontend workspace
	cd webui/frontend && pnpm run lint
	cd webui/frontend && pnpm run test
	cd webui/frontend && pnpm run build

verify-bff: ## Compile and test the installed BFF environment
	cd webui/bff && $(BFF_PYTHON) -m compileall -q app
	cd webui/bff && $(BFF_PYTHON) -m pytest -q
