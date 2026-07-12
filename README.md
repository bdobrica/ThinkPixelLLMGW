# ThinkPixelLLMGW

ThinkPixelLLMGW is an OpenAI-compatible LLM gateway written in Go. It centralizes API-key authentication, model routing, provider credentials, per-key rate limits and budgets, usage accounting, audit logging, and administration.

> Project status (reviewed July 12, 2026): active MVP. The OpenAI chat-completions path and the core admin API are implemented. The project is not yet production-ready; see [CODE_REVIEW.md](CODE_REVIEW.md) and [TODO.md](TODO.md).

## Implemented

- `POST /v1/chat/completions`, including SSE-framed OpenAI streaming with terminal token and cost accounting
- PostgreSQL repositories for keys, providers, models, aliases, admins, usage, and monthly summaries
- Redis-backed atomic sliding-window rate limiting and budget tracking
- Database-driven pricing and asynchronous billing/usage queues
- Admin authentication with Argon2id credentials and HS256 JWTs
- Admin CRUD for API keys, providers, models, and model aliases
- AES-256-GCM provider-credential encryption
- Prometheus metrics at `GET /metrics`
- File request logging plus an optional Redis-to-S3/MinIO sink
- React 19 admin UI with a FastAPI backend-for-frontend (BFF); API-key management is implemented, while models, billing, and dashboard statistics remain placeholders

Only the OpenAI provider currently performs requests. Vertex AI and Bedrock are scaffolds.

## Architecture

```text
LLM client ── API key ──> Go gateway ──> OpenAI
                              │
                              ├── PostgreSQL (configuration and durable usage)
                              ├── Redis (rate limits, budgets, queues, log buffer)
                              ├── S3/MinIO (optional audit-log archive)
                              └── /metrics (Prometheus)

Browser ── signed HttpOnly cookie ──> FastAPI BFF ── JWT ──> Go admin API
```

## Quick start with Docker Compose

Prerequisites: Docker with Compose and an OpenAI API key.

```bash
cp .env.example .env
```

Edit `.env` and set at least:

```dotenv
JWT_SECRET=replace-with-a-long-random-value
ENCRYPTION_KEY=replace-with-64-hex-characters
OPENAI_API_KEY=sk-...
```

Generate an encryption key with:

```bash
./llm_gateway/scripts/generate-encryption-key.sh
```

Start the stack and bootstrap an administrator:

```bash
docker compose up -d --build
docker compose run --rm gateway /app/init-admin
```

The gateway listens on `http://localhost:8080`. See [docs/quickstart.md](docs/quickstart.md) and [docs/bootstrap-admin.md](docs/bootstrap-admin.md) for the full workflow.

## Local development

The gateway requires PostgreSQL, Redis, `DATABASE_URL`, `JWT_SECRET`, and `ENCRYPTION_KEY` at startup.

```bash
docker compose up -d postgres redis minio minio-create-bucket
cd llm_gateway
go run ./cmd/gateway
```

Useful checks:

```bash
cd llm_gateway
GOCACHE=/tmp/thinkpixel-go-cache go test ./...

cd ../webui/frontend
pnpm install
pnpm run build

cd ../bff
python3 -m compileall -q app
```

`go test -short ./...` is the hermetic Go unit suite. PostgreSQL, Redis-server, and MinIO tests are explicitly tagged and run with `make test-integration-all`; see the [testing guide](docs/testing-guide.md).

For OpenAI streaming requests, the gateway requests terminal usage, parses complete SSE events independently of network read boundaries, and records input, output, cached, and reasoning tokens. If a stream is interrupted or completes without provider usage, its accounting status is recorded as unknown and no zero-cost success is silently claimed; operators should alert and reconcile those requests from provider billing data.

Runnable Go examples are isolated as independent commands:

```bash
cd llm_gateway
go run ./examples/encryption
go run ./examples/s3-logging
```

The S3 example requires the environment and services described in [llm_gateway/examples/README.md](llm_gateway/examples/README.md).

## API surface

| Endpoint | Authentication | Purpose |
|---|---|---|
| `GET /health` | none | Process liveness |
| `GET /metrics` | none | Prometheus metrics |
| `POST /v1/chat/completions` | gateway API key | Chat proxy |
| `POST /admin/auth/login` | none | Email/password login |
| `POST /admin/auth/token` | none | Service-token login |
| `/admin/keys[/{id}]` | admin JWT | API-key CRUD and regeneration |
| `/admin/providers[/{id}]` | admin JWT | Provider CRUD |
| `/admin/models[/{id}]` | admin JWT | Model CRUD |
| `/admin/aliases[/{id}]` | admin JWT | Alias CRUD |

Read operations require the `viewer` role; mutations require `admin`. There is no `editor` role in the current implementation.

## Documentation

- [Quick start](docs/quickstart.md)
- [Environment variables](docs/env-variables.md)
- [Database schema](docs/database-schema.md)
- [Metrics](docs/metrics.md)
- [Testing guide](docs/testing-guide.md)
- [Web UI](webui/README.md)
- [Code review](CODE_REVIEW.md)
- [Development backlog](TODO.md)

## Continuous integration

GitHub Actions validates Go formatting/vet, race-enabled unit tests, Docker-backed integration tests, migration down/up round trips, frontend lint/build, BFF compilation, and the production container build. Pull-request checks require no provider secrets.

## License

See [LICENSE](LICENSE).
