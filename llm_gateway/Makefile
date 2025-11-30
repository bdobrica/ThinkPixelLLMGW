# Makefile for LLM Gateway

.PHONY: help test test-unit test-integration test-integration-setup test-integration-teardown build run clean

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-30s %s\n", $$1, $$2}'

test: ## Run all tests (unit + integration if services available)
	go test ./... -v

test-unit: ## Run only unit tests
	go test ./... -v -short

test-integration-setup: ## Start integration test services (Minio, Redis)
	@echo "Starting integration test services..."
	cd .. && docker compose up -d minio minio-create-bucket redis
	@echo "Waiting for services to be ready..."
	@sleep 5
	@echo "Services ready!"
	@echo ""
	@echo "Minio Console: http://localhost:9001 (minioadmin/minioadmin)"
	@echo "Minio API:     http://localhost:9000"
	@echo "Redis:         localhost:6379"

test-integration-teardown: ## Stop integration test services
	@echo "Stopping integration test services..."
	cd .. && docker compose down
	@echo "Services stopped and cleaned up"

test-integration: ## Run integration tests (requires services to be running)
	@echo "Running S3 integration tests..."
	MINIO_ENDPOINT=http://localhost:9000 go test -v ./internal/logging/... -run TestS3Integration
	@echo ""
	@echo "Running Redis queue integration tests..."
	REDIS_TEST_ADDR=localhost:6379 go test -v ./internal/queue/... -run TestRedis
	@echo ""
	@echo "Running HTTP API integration tests..."
	DATABASE_URL="postgres://gateway:password@localhost:5432/gateway?sslmode=disable" go test -v ./internal/httpapi/... -run TestAdminProviders

test-integration-all: test-integration-setup test-integration test-integration-teardown ## Setup, run, and teardown integration tests

test-httpapi: ## Run HTTP API integration tests (requires postgres to be running)
	@echo "Running HTTP API integration tests..."
	@echo "Make sure PostgreSQL is running: make dev-up"
	@echo ""
	DATABASE_URL="postgres://gateway:password@localhost:5432/gateway?sslmode=disable" go test -v ./internal/httpapi/... -run TestAdminProviders

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
	docker-compose up -d

dev-down: ## Stop development dependencies
	docker-compose down

dev-logs: ## Show logs from development dependencies
	docker-compose logs -f

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
ci-test: ## Run tests in CI environment
	go test ./... -v -race -coverprofile=coverage.out

ci-coverage: ## Generate coverage report
	go tool cover -html=coverage.out -o coverage.html
