# Environment variables

The gateway reads process environment variables. It does not load `.env` files itself; Docker Compose loads the repository-root `.env` file and passes selected values to the container.

## Required gateway variables

| Variable | Requirement |
|---|---|
| `DATABASE_URL` | PostgreSQL connection string |
| `JWT_SECRET` | Admin JWT signing secret, at least 32 characters |
| `METRICS_AUTH_TOKEN` | Dedicated `/metrics` bearer token, at least 32 characters |
| `ENCRYPTION_KEY` | AES-256 key encoded as exactly 64 hexadecimal characters |

Example for a gateway running on the host:

```dotenv
DATABASE_URL=postgres://gateway:password@localhost:5432/gateway?sslmode=disable
JWT_SECRET=0123456789abcdef0123456789abcdef
METRICS_AUTH_TOKEN=abcdef0123456789abcdef0123456789
ENCRYPTION_KEY=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
```

Generate new secret values for every environment. Never reuse the examples above or commit `.env`.

## HTTP server

| Variable | Default | Description |
|---|---:|---|
| `GATEWAY_HTTP_PORT` | `8080` | Listening port; legacy `HTTP_PORT` is used only when this variable is unset |
| `HTTP_READ_HEADER_TIMEOUT` | `10s` | Maximum time to receive request headers |
| `HTTP_READ_TIMEOUT` | `30s` | Maximum time to read a complete request |
| `HTTP_WRITE_TIMEOUT` | `30s` | Ordinary response write limit; SSE clears only this server deadline |
| `HTTP_IDLE_TIMEOUT` | `120s` | Keep-alive idle timeout |
| `HTTP_SHUTDOWN_TIMEOUT` | `30s` | Per-stage graceful shutdown limit |
| `HTTP_READINESS_TIMEOUT` | `2s` | Overall readiness-check deadline |

HTTP timeout values must be positive Go durations such as `500ms`, `30s`, or `2m`; invalid values stop startup. `PROVIDER_REQUEST_TIMEOUT` independently bounds the complete upstream provider request, including a stream.

## PostgreSQL and caches

| Variable | Default | Description |
|---|---:|---|
| `DB_MAX_OPEN_CONNS` | `25` | Maximum open PostgreSQL connections |
| `DB_MAX_IDLE_CONNS` | `5` | Maximum idle PostgreSQL connections |
| `DB_CONN_MAX_LIFETIME` | `5m` | Maximum connection lifetime |
| `DB_CONN_MAX_IDLE_TIME` | `1m` | Maximum connection idle time |
| `CACHE_API_KEY_SIZE` | `1000` | In-process API-key cache capacity |
| `CACHE_API_KEY_TTL` | `5m` | API-key cache TTL |
| `CACHE_MODEL_SIZE` | `500` | In-process model cache capacity |
| `CACHE_MODEL_TTL` | `15m` | Model cache TTL |

Non-HTTP numeric and duration settings currently fall back to their defaults when malformed. Validate deployment configuration before rollout rather than relying on that fallback.

## Redis

| Variable | Default | Description |
|---|---:|---|
| `REDIS_ADDRESS` | `localhost:6379` | Redis host and port |
| `REDIS_PASSWORD` | empty | Redis password |
| `REDIS_DB` | `0` | Redis database number |
| `REDIS_POOL_SIZE` | `10` | Maximum socket connections |
| `REDIS_MIN_IDLE_CONNS` | `2` | Minimum idle connections |
| `REDIS_DIAL_TIMEOUT` | `5s` | Connection timeout |
| `REDIS_READ_TIMEOUT` | `3s` | Socket read timeout |
| `REDIS_WRITE_TIMEOUT` | `3s` | Socket write timeout |

Redis is used for rate limits, budget state, durable billing/usage queues, dead-letter queues, and the optional S3 log buffer. Protect it with authentication, TLS/network isolation, persistence, and backups appropriate to the deployment.

## Providers

| Variable | Default | Description |
|---|---:|---|
| `PROVIDER_RELOAD_INTERVAL` | `5m` | Database provider-registry reload interval; `0` disables periodic reload |
| `PROVIDER_REQUEST_TIMEOUT` | `60s` | Default complete provider-call timeout |
| `OPENAI_API_KEY` | unset | Runtime-only credential for a provider row named `openai` |
| `VERTEX_AI_ACCESS_TOKEN` | unset | Short-lived runtime-only credential for a row named `vertexai` |
| `VERTEX_AI_SERVICE_ACCOUNT_JSON` | unset | Runtime-only service-account JSON for a row named `vertexai` |
| `GOOGLE_APPLICATION_CREDENTIALS` | unset | Standard Google ADC credential-file path |

Vertex AI can use any Google Application Default Credentials source. Bedrock uses the AWS SDK v2 default credential chain, including `AWS_REGION`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`, web identity, shared configuration, ECS, and EC2 roles. Prefer workload identity over static cloud credentials.

Only OpenAI, Vertex AI, and Bedrock provider types are registered. `ANTHROPIC_API_KEY` is recognized by the generic override collector but there is no standalone `anthropic` adapter; Anthropic models may instead be reached through the implemented Bedrock adapter when supported.

Overrides are held in memory, reapplied on registry reload, and never persisted. An override targeting a missing or disabled provider row causes a distinct startup/reload error. Project, region, endpoint, and model configuration belong in provider/model records; see [Provider configuration](providers.md).

## Local audit logger

| Variable | Default | Description |
|---|---:|---|
| `REQUEST_LOGGER_FILE_PATH_TEMPLATE` | `/var/log/llm-gateway/requests-%s.jsonl` | Rotating file template; must contain the timestamp placeholder |
| `REQUEST_LOGGER_MAX_SIZE` | `10485760` | Rotation size in bytes |
| `REQUEST_LOGGER_MAX_FILES` | `5` | Rotated files retained |
| `REQUEST_LOGGER_BUFFER_SIZE` | `100` | In-memory write buffer |
| `REQUEST_LOGGER_FLUSH_INTERVAL` | `60s` | Periodic disk flush interval |
| `AUDIT_BODY_MODE` | `hash` | `none`, `hash`, or `redacted` |
| `AUDIT_MAX_BODY_BYTES` | `4096` | Maximum retained JSON body size in redacted mode |
| `AUDIT_SAMPLE_RATE` | `1` | Fraction from `0` through `1` |
| `AUDIT_SENSITIVE_FIELDS` | empty | Comma-separated additional JSON field names to redact |

Review [Audit logging privacy and retention](logging-privacy.md) before retaining request or response bodies.

## S3 audit sink

| Variable | Default | Description |
|---|---:|---|
| `LOGGING_SINK_ENABLED` | `false` | Enables the Redis-to-S3 worker |
| `LOGGING_SINK_BUFFER_SIZE` | `10000` | Maximum queued log records |
| `LOGGING_SINK_FLUSH_SIZE` | `1000` | Maximum upload batch size |
| `LOGGING_SINK_FLUSH_INTERVAL` | `5m` | Maximum interval between upload attempts |
| `LOGGING_SINK_S3_BUCKET` | none | Required when enabled |
| `LOGGING_SINK_S3_REGION` | `us-east-1` | Required, nonempty region when enabled |
| `LOGGING_SINK_S3_PREFIX` | `logs/` | Object-key prefix |
| `POD_NAME` | `gateway-0` | Instance identifier included in object names |

The S3 client also honors standard AWS SDK settings such as credentials and `AWS_ENDPOINT_URL_S3`; the latter is used by the Compose MinIO environment. Obsolete names such as `S3_BUCKET`, `S3_ACCESS_KEY`, `REDIS_URL`, and `LOGGING_SINK_BATCH_SIZE` do not configure the gateway.

When the sink is enabled, bucket, region, buffer size, flush size, and flush interval are validated. Objects are gzip-compressed and written with SSE-S3 AES-256 encryption.

## Complete host-development example

```dotenv
GATEWAY_HTTP_PORT=8080
DATABASE_URL=postgres://gateway:password@localhost:5432/gateway?sslmode=disable
REDIS_ADDRESS=localhost:6379
JWT_SECRET=0123456789abcdef0123456789abcdef
METRICS_AUTH_TOKEN=abcdef0123456789abcdef0123456789
ENCRYPTION_KEY=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
OPENAI_API_KEY=sk-replace-me
AUDIT_BODY_MODE=hash
LOGGING_SINK_ENABLED=false
```

Use generated secrets, not these deterministic documentation values. Compose supplies its own container-facing database, Redis, and MinIO addresses; normally only uncomment and replace secret/provider values in `.env.example`.

## Production notes

- Inject secrets through a secret manager or workload identity and restrict environment inspection.
- Use TLS for public traffic and managed-service connections.
- Keep `/metrics`, PostgreSQL, Redis, MinIO administration, and the BFF off the public ingress.
- Set orchestrator termination grace to at least two `HTTP_SHUTDOWN_TIMEOUT` windows. A `preStop` sleep may allow load-balancer propagation but occurs before `SIGTERM` and does not flush the gateway itself.
- Do not rotate `ENCRYPTION_KEY` without the maintenance procedure in the [Operations runbook](operations-runbook.md).
