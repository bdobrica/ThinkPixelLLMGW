# Docker setup

The repository-root `docker-compose.yaml` provides a development and integration environment. It is not a production topology.

## Services and ports

| Service | Container | Loopback port | Purpose |
|---|---|---:|---|
| PostgreSQL 16 | `gw-postgres` | `5432` | Configuration, administrators, durable usage and billing summaries |
| Redis 8 | `gw-redis` | `6379` | Rate limits, budgets, queues, dead letters, log buffer |
| MinIO | `gw-minio` | `9000`, console `9001` | Development S3-compatible audit archive |
| Gateway | `gw-gateway` | `8080` | Proxy and Admin API |

Host ports can be changed with `POSTGRES_HOST_PORT`, `REDIS_HOST_PORT`, `MINIO_API_HOST_PORT`, `MINIO_CONSOLE_HOST_PORT`, and `GATEWAY_HOST_PORT`. All published ports bind to `127.0.0.1`.

## Start the stack

Create `.env` first; Compose refuses to start the gateway without valid security values:

```bash
cp .env.example .env
./llm_gateway/scripts/generate-encryption-key.sh
```

Set `JWT_SECRET`, `METRICS_AUTH_TOKEN`, `ENCRYPTION_KEY`, and any provider credentials in `.env`, then run:

```bash
docker compose up -d --build
docker compose ps
docker compose logs gateway
```

The PostgreSQL image applies every mounted `.up.sql` migration, including seed data, only when it initializes an empty volume. Adding a migration does not apply it to an existing volume automatically. Use the deployment migration process from the [Operations runbook](operations-runbook.md) for persistent environments.

Verify the gateway:

```bash
curl --fail http://localhost:8080/health
curl --fail http://localhost:8080/ready
```

`/health` is liveness and remains successful during dependency failures. `/ready` checks PostgreSQL, Redis, provider-registry access, and billing/usage workers within `HTTP_READINESS_TIMEOUT`.

## Database access

Compose credentials are development-only:

```text
database: gateway
user:     gateway
password: password
```

Connect and inspect the actual migrated schema:

```bash
docker compose exec postgres psql -U gateway -d gateway
```

Useful read-only queries:

```sql
SELECT name, provider_type, enabled FROM providers ORDER BY name;
SELECT model_name, provider_id, source FROM models ORDER BY model_name;
SELECT name, rate_limit_per_minute, monthly_budget_nano_usd, enabled
FROM api_keys ORDER BY name;
SELECT api_key_id, year, month, total_requests, total_cost_nano_usd
FROM monthly_usage_summary ORDER BY year DESC, month DESC;
```

Do not insert provider credentials manually. `encrypted_credentials` contains AES-GCM ciphertext produced by the Admin API; plaintext JSON in that column is neither secure nor usable.

For a disposable reset that reapplies every migration:

```bash
docker compose down -v
docker compose up -d --build
```

This deletes all local PostgreSQL, Redis, MinIO, and request-log data.

## Redis access

```bash
docker compose exec redis redis-cli PING
docker compose exec redis redis-cli INFO memory
docker compose exec redis redis-cli LLEN queue:billing
docker compose exec redis redis-cli LLEN queue:usage
docker compose exec redis redis-cli LLEN dlq:billing
docker compose exec redis redis-cli LLEN dlq:usage
```

Avoid `KEYS *` or `MONITOR` on shared/production Redis instances. Queue names are implementation details useful for development diagnosis, not a stable external API.

## MinIO access

Open `http://localhost:9001` and sign in with `minioadmin` / `minioadmin`, or use the MinIO client:

```bash
mc alias set local http://localhost:9000 minioadmin minioadmin
mc ls local/llm-logs
mc stat local/llm-logs/<object-key>
```

Compose creates the `llm-logs` bucket and configures a static development-only KMS key so SSE-S3 uploads can be exercised. Production must use an approved external KMS/workload identity and a private bucket policy.

## Gateway administration and requests

Create the first administrator through the supported initializer rather than SQL:

```bash
docker compose run --rm \
  -e ADMIN_BOOTSTRAP_EMAIL=admin@example.com \
  -e ADMIN_BOOTSTRAP_PASSWORD='replace-with-a-strong-password' \
  --entrypoint /app/init-admin \
  gateway
```

The seed migration creates the development proxy key `demo-key-12345`. With `OPENAI_API_KEY` configured, send:

```bash
curl --fail-with-body http://localhost:8080/v1/chat/completions \
  -H 'Authorization: Bearer demo-key-12345' \
  -H 'Content-Type: application/json' \
  --data '{
    "model":"gpt-4o-mini",
    "messages":[{"role":"user","content":"Reply with hello."}]
  }'
```

For streaming, include `"stream": true`. Provider calls may incur charges. Replace the seeded key through the Admin API for any persistent environment.

Metrics require a separate bearer credential:

```bash
curl --fail http://localhost:8080/metrics \
  -H "Authorization: Bearer $METRICS_AUTH_TOKEN"
```

## Development workflow

Rebuild only the gateway after source changes:

```bash
docker compose up -d --build gateway
docker compose logs -f gateway
```

Start only infrastructure for a host-run gateway:

```bash
docker compose up -d postgres redis minio minio-create-bucket
```

Use the isolated integration stack and automatic cleanup for tests:

```bash
cd llm_gateway
make test-integration-all
make test-migrations
```

See [Testing guide](testing-guide.md) for ports and focused suites.

## Backup and restore

For a development snapshot:

```bash
docker compose exec -T postgres pg_dump -U gateway -d gateway -Fc > gateway.dump
docker compose exec -T postgres pg_restore -U gateway -d gateway --clean --if-exists < gateway.dump
```

The restore command modifies the current database and is appropriate only for a deliberately disposable development environment. Production backups must be encrypted and restored into an isolated database for verification; follow the [Operations runbook](operations-runbook.md).

## Troubleshooting

- Interpolation error: uncomment and replace all three required security values in the repository-root `.env`.
- Gateway exits before listening: inspect `docker compose logs gateway`; configuration validation occurs before dependency connections.
- Database schema appears old: migrations run automatically only for a new PostgreSQL volume.
- Port collision: set the corresponding `*_HOST_PORT` value in `.env` and reconnect to that port.
- `/health` succeeds but `/ready` fails: inspect PostgreSQL, Redis, registry, and worker logs rather than treating liveness as readiness.
- MinIO uploads fail: verify the bucket initializer completed with `docker compose logs minio-create-bucket` and inspect Redis buffer growth.

## Production boundary

Do not scale the Compose gateway as a production deployment. Supply immutable image digests, TLS ingress, workload identities, managed/persistent dependencies, resource limits, non-root/read-only security settings, private metrics, readiness/liveness probes, controlled migrations, backups, monitoring, and a termination grace period of at least two `HTTP_SHUTDOWN_TIMEOUT` windows. Complete [Release qualification](release-qualification.md) for the target environment.
