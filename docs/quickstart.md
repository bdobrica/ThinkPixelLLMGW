# Quick start

This guide starts the development stack, creates the first administrator, and sends a real OpenAI-compatible request. The Compose stack is for development and release qualification, not production.

## Prerequisites

- Docker with the Compose v2 plugin
- Git
- An OpenAI API key for the seeded OpenAI provider

Go 1.26.6 or newer is needed only when running the gateway or its tests outside Docker.

## 1. Configure secrets

From the repository root:

```bash
cp .env.example .env
./llm_gateway/scripts/generate-encryption-key.sh
```

Edit `.env` and replace the examples with real values:

```dotenv
OPENAI_API_KEY=sk-...
JWT_SECRET=generate-at-least-32-random-characters
METRICS_AUTH_TOKEN=generate-a-different-32-character-token
ENCRYPTION_KEY=paste-the-64-hex-character-generated-value
```

`JWT_SECRET` and `METRICS_AUTH_TOKEN` must each be at least 32 characters. `ENCRYPTION_KEY` must be exactly 64 hexadecimal characters. Do not commit `.env`.

## 2. Start the stack

```bash
docker compose up -d --build
docker compose ps
```

Compose starts PostgreSQL, Redis, MinIO, the bucket initializer, and the gateway. The database migrations and development seed data are applied automatically to a new PostgreSQL volume.

Check liveness and dependency readiness:

```bash
curl --fail http://localhost:8080/health
curl --fail http://localhost:8080/ready
```

`/health` reports process liveness. `/ready` returns success only when PostgreSQL, Redis, the provider registry, and asynchronous workers are available.

## 3. Bootstrap an administrator

Choose a strong password and run the one-time initializer using the gateway image and configuration:

```bash
docker compose run --rm \
  -e ADMIN_BOOTSTRAP_EMAIL=admin@example.com \
  -e ADMIN_BOOTSTRAP_PASSWORD='replace-with-a-strong-password' \
  --entrypoint /app/init-admin \
  gateway
```

The command is idempotent: if any administrator already exists, it exits successfully without creating another. See [Bootstrap admin](bootstrap-admin.md) for Kubernetes and troubleshooting instructions.

Log in and save the returned JWT:

```bash
curl --fail-with-body http://localhost:8080/admin/auth/login \
  -H 'Content-Type: application/json' \
  --data '{"email":"admin@example.com","password":"replace-with-a-strong-password"}'
```

The response contains a `token`. Use it as `Authorization: Bearer <token>` for Admin API requests.

## 4. Use the seeded development API key

The development migration creates the plaintext test key `demo-key-12345`, the OpenAI provider row named `openai`, and several model records. `OPENAI_API_KEY` is applied to that provider in memory and is never written to PostgreSQL.

Send a non-streaming request:

```bash
curl --fail-with-body http://localhost:8080/v1/chat/completions \
  -H 'Authorization: Bearer demo-key-12345' \
  -H 'Content-Type: application/json' \
  --data '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Reply with hello."}]
  }'
```

For streaming, add `"stream": true`. Real provider requests can incur charges.

The seeded key is development-only. Create a replacement through `POST /admin/keys`, store the returned plaintext key immediately, and remove or disable the seeded key before using a persistent environment.

## 5. Inspect the services

```bash
docker compose logs -f gateway
docker compose exec postgres psql -U gateway -d gateway
docker compose exec redis redis-cli
```

MinIO is available at `http://localhost:9001` with the development credentials `minioadmin` / `minioadmin`. Prometheus metrics require their dedicated token:

```bash
curl --fail http://localhost:8080/metrics \
  -H "Authorization: Bearer $METRICS_AUTH_TOKEN"
```

## Local Go development

Start only the dependencies from the repository root:

```bash
docker compose up -d postgres redis minio minio-create-bucket
```

When running the gateway on the host, export host-facing values rather than Compose service names:

```bash
export DATABASE_URL='postgres://gateway:password@localhost:5432/gateway?sslmode=disable'
export REDIS_ADDRESS='localhost:6379'
export JWT_SECRET='generate-at-least-32-random-characters'
export METRICS_AUTH_TOKEN='generate-a-different-32-character-token'
export ENCRYPTION_KEY='0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef'
export LOGGING_SINK_ENABLED=false

cd llm_gateway
go run ./cmd/gateway
```

Run the hermetic test suite with `make test`; use [Testing guide](testing-guide.md) for integration, end-to-end, load, and release checks.

## Stop or reset

Stop containers while retaining data:

```bash
docker compose down
```

Delete all development PostgreSQL, Redis, MinIO, and request-log volumes:

```bash
docker compose down -v
```

The second command is destructive. For configuration details, continue with [Environment variables](env-variables.md), [Provider configuration](providers.md), and [Docker setup](docker-setup.md).
