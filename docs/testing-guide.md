# Testing guide

The repository has three intentionally separate test tiers. Run commands from `llm_gateway` unless noted otherwise.

## 1. Hermetic Go tests

These tests do not require PostgreSQL, Redis, MinIO, Docker, or provider credentials:

```bash
make test
# equivalent to:
go test -short ./...
```

`make test-unit` is an alias for the same suite. Redis-server, PostgreSQL, and MinIO tests carry the `integration` build tag and are not compiled into this tier. Tests using in-process fakes such as miniredis remain unit tests.

The HTTP/provider unit suites include streaming coverage for SSE events split across reads, multiple events in one read, terminal cached/reasoning usage, interruption and cancellation, unknown accounting, and exact-once billing/usage enqueue behavior. Run that focused set with:

```bash
go test -short ./internal/httpapi ./internal/providers ./internal/metrics ./internal/models
```

For a stricter local/CI run:

```bash
make ci-test
```

This enables the race detector and writes `coverage.out`.

## 2. Go integration tests

Prerequisite: Docker with Compose. Integration targets use isolated host ports by default: PostgreSQL 15432, Redis 16379, MinIO API 19000, and MinIO console 19001. Override the corresponding `INTEGRATION_*_PORT` Make variables when necessary.

The safest command starts dependencies, runs every tagged suite, and tears dependencies down even if a test fails:

```bash
make test-integration-all
```

Manual lifecycle:

```bash
make test-integration-setup
make test-integration
make test-integration-teardown
```

The tagged suites cover:

- PostgreSQL-backed admin API handlers
- Redis queues and dead-letter queues
- MinIO/S3 log writing and shutdown

Focused commands are also available:

```bash
make test-httpapi
make test-aliases
```

Validate every migration down and back up on a clean PostgreSQL volume:

```bash
make test-migrations
```

To invoke a tagged package directly, supply its service configuration:

```bash
DATABASE_URL='postgres://gateway:password@localhost:15432/gateway?sslmode=disable' \
  go test -tags=integration -v ./internal/httpapi -run '^TestAdmin'

REDIS_TEST_ADDR=localhost:16379 \
  go test -tags=integration -v ./internal/queue -run '^TestRedis'

MINIO_ENDPOINT=http://localhost:19000 \
  go test -tags=integration -v ./internal/logging -run '^TestS3Integration'
```

## 3. End-to-end tests

The Python tests exercise the running Docker Compose stack and may make real OpenAI requests. Create the root `.env` file first and set `OPENAI_API_KEY`, `JWT_SECRET`, and `ENCRYPTION_KEY`.

Install dependencies into any active Python environment; the Makefile does not assume a developer-specific virtual-environment path:

```bash
python3 -m venv .venv
source .venv/bin/activate
pip install -r tests/requirements.txt
```

Then run:

```bash
cd llm_gateway
make test-e2e
```

For manual control:

```bash
make test-e2e-setup
make test-e2e-logs
make test-e2e-run
make test-e2e-teardown
```

`make test-init-admin` exercises bootstrap, login, user creation, and API-key creation. `make test-rate-limit` runs the rate-limit subset against an already running stack.

## Frontend and BFF checks

```bash
cd webui/frontend
pnpm install
pnpm run build

cd ../bff
python3 -m compileall -q app
```

Automated BFF and frontend tests are still tracked in `TODO.md`.

## Continuous integration

`.github/workflows/ci.yml` runs the commands above on pushes to `main` and on pull requests. It also checks Go formatting/vet, runs unit tests with the race detector, validates the frontend and BFF, builds the Docker image, and performs a complete migration round trip. These checks do not use an OpenAI key or make provider requests.

## Troubleshooting

- Use `docker compose ps` and `docker compose logs <service>` when integration setup fails.
- Ensure no local database, Redis, or MinIO already owns the required ports.
- Set `GOCACHE=/tmp/thinkpixel-go-cache` if the default Go build cache is not writable.
- A missing provider key should not affect hermetic Go tests; it is required only for provider-backed end-to-end requests.
