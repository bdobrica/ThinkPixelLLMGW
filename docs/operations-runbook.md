# Operations runbook

This runbook covers deployment, rollback, secret rotation, dependency outages, billing reconciliation, backup/restore, and audit-log retention for the current gateway release. The Docker Compose file is a development and qualification environment, not a production topology. Production operators must supply TLS, an internal metrics network, managed data services, workload identity, monitoring, and an orchestrator-specific deployment definition.

## Ownership and prerequisites

Assign a release commander, database owner, security owner, and billing owner before each deployment. Record their names in the change ticket together with the image digest, migration version, configuration revision, rollback image, backup identifier, and provider status pages.

Required preflight checks:

1. Complete the [release qualification checklist](release-qualification.md) from a clean checkout.
2. Back up PostgreSQL and verify a restore into an isolated database.
3. Confirm Redis persistence/replication and S3 lifecycle rules match the approved recovery and retention objectives.
4. Confirm the new image runs as a non-root user, has a read-only root filesystem where the platform supports it, and can write only its audit-log volume.
5. Verify `/metrics` is reachable only by the scraper with `METRICS_AUTH_TOKEN`; never expose it through the public ingress.
6. Confirm provider quotas, budgets, credentials, regions, and model/pricing records.

## Deploy and verify

Apply database migrations once, before rolling out code that requires them. Migrations are SQL files in `llm_gateway/migrations`; execute them in timestamp order with `psql -v ON_ERROR_STOP=1` under a migration identity, not the gateway runtime identity.

Roll out one instance at a time:

1. Remove the instance from service and wait for the load balancer to stop new requests.
2. Send `SIGTERM`. Allow at least two `HTTP_SHUTDOWN_TIMEOUT` windows: one for HTTP streams and one for workers, billing, logs, queues, and infrastructure clients.
3. Start the new image with the recorded configuration and immutable image digest.
4. Require `GET /health` to return `200` and `GET /ready` to return `200` before adding traffic.
5. Make a credential-gated canary request for each enabled provider and verify its usage record, monthly billing delta, audit record, latency/error metrics, and terminal stream usage when streaming.
6. Compare error rate, readiness, queue depth, missing stream usage, provider latency, and cost with the pre-deploy baseline before continuing.

Redis-backed billing/usage queues survive an orderly restart. In-memory queues do not and must not be used in a multi-instance production deployment.

## Rollback

Stop the rollout when readiness does not recover, canary accounting differs from provider usage, error/latency budgets regress, or queues grow without draining.

1. Remove affected instances from service and preserve logs and metrics.
2. Redeploy the previous immutable image and its compatible configuration.
3. Prefer a forward database fix. Run a `.down.sql` migration only after confirming no newer instance is running and no post-migration data would be truncated or rounded.
4. Restore PostgreSQL only for corruption or an explicitly approved point-in-time recovery. A restore discards newer administration and accounting records; reconcile provider invoices afterward.
5. Re-run readiness, provider canaries, usage persistence, billing, and queue-drain checks before restoring traffic.

## Dependency outage and recovery

`/health` remains `200` while dependencies are unavailable. `/ready` returns `503` when PostgreSQL, Redis, provider-registry access, or either asynchronous worker is unavailable. The load balancer must route only to ready instances.

| Dependency | Expected behavior | Operator response | Recovery proof |
|---|---|---|---|
| PostgreSQL | Readiness fails; authentication/model/admin operations fail. | Stop routing, restore connectivity or fail over, and do not bypass migrations. | `/ready` is `200`; model/admin reads work; queued usage drains without duplicates. |
| Redis | Readiness fails. Rate-limit checks return an error response. Budget reads deliberately fail open, while billing/log/usage queue writes may fail and are surfaced in logs/metrics. | Stop routing, restore/fail over Redis, inspect DLQs and queue depth, then reconcile the outage interval. | `/ready` is `200`; rate limiting works; queues drain; monthly totals match durable usage/provider data. |
| S3 | Core readiness can remain healthy; upload retries leave records in the Redis log buffer. | Keep Redis available, restore S3 permissions/network/bucket, and watch buffer capacity to prevent eviction. | New objects use SSE-S3; the Redis log buffer returns to baseline; a sampled object passes privacy checks. |
| Provider | Only requests routed to that provider fail or time out. There is no automatic fallback or circuit breaker. | Disable/reroute affected models administratively, respect provider status/quota guidance, and avoid retry storms. | Credential validation and non-stream/stream canaries pass; usage agrees with provider reporting. |

Test each failure in staging by stopping or firewalling one dependency at a time, asserting the behavior above, restoring it, and recording timestamps plus evidence. Never perform fault injection first in production.

## Shutdown with active streams and queued work

During a rolling-restart rehearsal, keep a stream active longer than the ordinary HTTP write timeout and enqueue billable requests immediately before `SIGTERM`. Verify that the stream drains within the first shutdown window and that usage, billing, and S3 records flush within the second. If the deadline expires, Redis queue entries remain for the next instance; an interrupted stream is stored with unknown accounting and must enter reconciliation.

## Backup and restore

Create a PostgreSQL custom-format backup and retain the manifest alongside it:

```bash
pg_dump --format=custom --no-owner --file=gateway.dump "$DATABASE_URL"
pg_restore --list gateway.dump > gateway.dump.manifest
```

Verify, do not assume, restorability:

```bash
createdb llmgateway_restore_test
pg_restore --exit-on-error --no-owner --dbname=llmgateway_restore_test gateway.dump
psql --dbname=llmgateway_restore_test -v ON_ERROR_STOP=1 -c \
  'SELECT count(*) FROM providers; SELECT count(*) FROM usage_records;'
dropdb llmgateway_restore_test
```

Encrypt backups, restrict access, test point-in-time recovery if enabled, and record recovery time and recovery point results. PostgreSQL backups do not cover Redis persistence, S3 audit objects, deployment secrets, or provider-side billing exports; manage and test those separately.

## Secret rotation

- Provider credentials: update the deployment secret/workload identity and restart for runtime overrides, or update the provider through the admin API. Validate the new credential before revoking the old one. Values must never enter logs or tickets.
- `METRICS_AUTH_TOKEN`: update Prometheus and gateway secrets as one coordinated rollout; verify anonymous `401` and authenticated `200` responses.
- `JWT_SECRET`: rotate during a coordinated rollout. Existing administrator JWTs and BFF sessions become invalid, so operators must sign in again.
- BFF `SECRET_KEY`: rotate all BFF instances together. Existing signed cookies become invalid.
- `ENCRYPTION_KEY`: this release has no online re-encryption command. During planned maintenance, export provider credentials into an approved secret manager while the old key is active, clear the stored encrypted credential maps, stop all gateway instances, rotate the key, start with runtime provider overrides, and write credentials back through the admin API so they are encrypted with the new key. Verify the backup and every provider before revoking the old key. Never run old- and new-key instances concurrently.

## Billing reconciliation

For a UTC interval and API key/model/provider grouping:

1. Export provider usage/invoice data and freeze its currency/time-zone/version metadata.
2. Query `usage_records` and `monthly_usage_summary`; isolate records with errors, interrupted streams, or missing terminal usage.
3. Recompute expected cost with the model pricing components effective for that interval. Pricing history is not versioned automatically, so preserve catalog revisions with the release evidence.
4. Compare durable usage, Redis monthly totals, summaries, and provider data in integer nano-USD. Do not compare rounded UI dollars.
5. Investigate duplicates by request ID and gaps by provider request ID/time window. Record an approved adjustment rather than editing raw usage silently.
6. Alert on any increase in `llm_gateway_stream_usage_missing_total`. Automatic provider-billing reconciliation is not implemented in this release.

## Audit-log retention and deletion

Follow [Audit Logging Privacy and Retention](logging-privacy.md). The deployment record must name the data owner, region, body mode, sample rate, local rotation, Redis persistence expiry, S3 lifecycle duration, backup expiry, deletion-request procedure, and authorized readers. Test lifecycle expiry and one identifier-based deletion before launch and after policy changes.

## Incident evidence

Preserve timestamps, request IDs, image/config revisions, dependency status, readiness transitions, queue/DLQ depth, and redacted provider responses. Never copy API keys, authorization headers, cookies, raw prompts, encryption material, or unreviewed audit bodies into incident systems.
