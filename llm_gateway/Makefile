# Makefile for LLM Gateway

INTEGRATION_POSTGRES_PORT ?= 15432
INTEGRATION_REDIS_PORT ?= 16379
INTEGRATION_MINIO_API_PORT ?= 19000
INTEGRATION_MINIO_CONSOLE_PORT ?= 19001
INTEGRATION_COMPOSE_ENV = POSTGRES_HOST_PORT=$(INTEGRATION_POSTGRES_PORT) REDIS_HOST_PORT=$(INTEGRATION_REDIS_PORT) MINIO_API_HOST_PORT=$(INTEGRATION_MINIO_API_PORT) MINIO_CONSOLE_HOST_PORT=$(INTEGRATION_MINIO_CONSOLE_PORT)

.PHONY: help test test-unit test-integration test-integration-all test-integration-setup test-integration-teardown test-httpapi test-aliases test-e2e test-e2e-setup test-e2e-run test-e2e-teardown test-e2e-logs test-rate-limit test-init-admin build run clean

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-30s %s\n", $$1, $$2}'

test: test-unit ## Run the hermetic unit-test suite

test-unit: ## Run hermetic unit tests (no external services)
	go test -short ./...

test-integration-setup: ## Start integration test services (Postgres, Minio, Redis)
	@echo "Starting integration test services..."
	@echo "Waiting for service health checks..."
	cd .. && $(INTEGRATION_COMPOSE_ENV) docker compose up --wait postgres minio redis
	cd .. && $(INTEGRATION_COMPOSE_ENV) docker compose up minio-create-bucket
	@echo "Integration services are ready."
	@echo ""
	@echo "PostgreSQL:    localhost:$(INTEGRATION_POSTGRES_PORT) (gateway/password)"
	@echo "Minio Console: http://localhost:$(INTEGRATION_MINIO_CONSOLE_PORT) (minioadmin/minioadmin)"
	@echo "Minio API:     http://localhost:$(INTEGRATION_MINIO_API_PORT)"
	@echo "Redis:         localhost:$(INTEGRATION_REDIS_PORT)"

test-integration-teardown: ## Stop integration test services
	@echo "Stopping integration test services..."
	cd .. && $(INTEGRATION_COMPOSE_ENV) docker compose down -v
	@echo "Services stopped and cleaned up"

test-integration: ## Run tagged integration tests (requires setup target)
	@echo "Running S3 integration tests..."
	MINIO_ENDPOINT=http://localhost:$(INTEGRATION_MINIO_API_PORT) go test -tags=integration -v ./internal/logging -run '^TestS3Integration'
	@echo ""
	@echo "Running Redis queue integration tests..."
	REDIS_TEST_ADDR=localhost:$(INTEGRATION_REDIS_PORT) go test -tags=integration -v ./internal/queue -run '^TestRedis'
	@echo ""
	@echo "Running HTTP API integration tests..."
	DATABASE_URL="postgres://gateway:password@localhost:$(INTEGRATION_POSTGRES_PORT)/gateway?sslmode=disable" go test -tags=integration -v ./internal/httpapi -run '^TestAdmin'

test-integration-all: test-integration-setup ## Setup, run, and always tear down integration services
	@status=0; $(MAKE) test-integration || status=$$?; $(MAKE) test-integration-teardown; exit $$status

test-httpapi: ## Run HTTP API integration tests (requires postgres to be running)
	@echo "Running HTTP API integration tests..."
	@echo "Make sure PostgreSQL is running: make dev-up"
	@echo ""
	@echo "Testing Admin Providers..."
	DATABASE_URL="postgres://gateway:password@localhost:$(INTEGRATION_POSTGRES_PORT)/gateway?sslmode=disable" go test -tags=integration -v ./internal/httpapi -run '^TestAdminProviders'
	@echo ""
	@echo "Testing Admin Models..."
	DATABASE_URL="postgres://gateway:password@localhost:$(INTEGRATION_POSTGRES_PORT)/gateway?sslmode=disable" go test -tags=integration -v ./internal/httpapi -run '^TestAdminModels'
	@echo ""
	@echo "Testing Admin Aliases..."
	DATABASE_URL="postgres://gateway:password@localhost:$(INTEGRATION_POSTGRES_PORT)/gateway?sslmode=disable" go test -tags=integration -v ./internal/httpapi -run '^TestAdminAliases'

test-aliases: ## Run alias management integration tests (requires postgres to be running)
	@echo "Running Admin Aliases integration tests..."
	@echo "Make sure PostgreSQL is running: make dev-up"
	@echo ""
	DATABASE_URL="postgres://gateway:password@localhost:$(INTEGRATION_POSTGRES_PORT)/gateway?sslmode=disable" go test -tags=integration -v ./internal/httpapi -run '^TestAdminAliases'

build: ## Build the gateway binary
	go build -o bin/gateway ./cmd/gateway

run: ## Run the gateway (requires DATABASE_URL)
	go run ./cmd/gateway/main.go

clean: ## Clean build artifacts and test caches
	rm -rf bin/
	go clean -testcache

docker-build: ## Build Docker image
	docker build -t llm-gateway:latest .

docker-run: ## Run Docker container (requires .env file)
	docker run --env-file .env -p 8080:8080 llm-gateway:latest

# Development targets
dev-up: ## Start development dependencies (Postgres, Redis, Minio)
	cd .. && docker compose up -d postgres redis minio minio-create-bucket

dev-down: ## Stop development dependencies
	cd .. && docker compose down

dev-logs: ## Show logs from development dependencies
	cd .. && docker compose logs -f

# Database targets
db-migrate-up: ## Run database migrations
	@echo "Running migrations..."
	# Add migration command here

db-migrate-down: ## Rollback database migrations
	@echo "Rolling back migrations..."
	# Add migration rollback command here

# Linting and formatting
fmt: ## Format code
	go fmt ./...
	gofmt -s -w .

lint: ## Run linter
	golangci-lint run ./...

# CI/CD helpers
ci-test: ## Run hermetic tests with race detection and coverage
	go test -short ./... -race -coverprofile=coverage.out

ci-coverage: ## Generate coverage report
	go tool cover -html=coverage.out -o coverage.html

# End-to-End Testing
test-e2e: ## Run end-to-end test (starts docker-compose, tests gateway, cleans up)
	@echo "Running end-to-end test..."
	@echo "This will start all services, run tests, and clean up"
	@echo ""
	cd .. && python3 tests/test_e2e.py

# Bootstrap Admin Testing
test-init-admin: ## Run init-admin integration test (bootstrap → login → create user → create API key)
	@echo "Running init-admin integration test..."
	@echo "This will test the complete admin bootstrap workflow"
	@echo ""
	@echo "Building Docker image with init-admin binary..."
	cd .. && docker compose build gateway
	@echo ""
	cd .. && python3 tests/test_init_admin.py

test-e2e-setup: ## Start services for manual e2e testing
	@echo "Starting all services for e2e testing..."
	cd .. && docker compose up -d
	@echo ""
	@echo "Services are starting up. Gateway will be available at http://localhost:8080"
	@echo "Run 'make test-e2e-run' to execute the test suite"
	@echo "Run 'make test-e2e-teardown' when done"

test-e2e-run: ## Run e2e test script (assumes services already running)
	@echo "Running e2e test against running services..."
	cd .. && python3 -c "from tests.test_e2e import test_chat_completion, test_invalid_api_key, test_model_alias, print_step; import sys; tests = [test_chat_completion(), test_invalid_api_key(), test_model_alias()]; passed = sum(tests); total = len(tests); print_step(f'Results: {passed}/{total} passed'); sys.exit(0 if passed == total else 1)"

test-e2e-teardown: ## Stop all services after e2e testing
	@echo "Stopping all services..."
	cd .. && docker compose down -v
	@echo "Services stopped and cleaned up"

test-e2e-logs: ## Show gateway logs
	@echo "Gateway logs (last 50 lines):"
	@echo ""
	docker logs --tail 50 gw-gateway

test-rate-limit: ## Run rate limiting tests only (requires services running)
	@echo "Running rate limiting tests..."
	@echo "NOTE: This requires services to be running (make test-e2e-setup)"
	@echo "This will make 65+ rapid requests to test rate limiting"
	@echo ""
	cd .. && python3 -c "from tests.test_e2e import test_rate_limiting, test_rate_limit_headers, print_step; import sys; tests = [test_rate_limiting(), test_rate_limit_headers()]; passed = sum(tests); total = len(tests); print_step(f'Rate Limit Tests: {passed}/{total} passed'); sys.exit(0 if passed == total else 1)"
