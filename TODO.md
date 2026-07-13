# ThinkPixelLLMGW backlog

Last reconciled with the code review and release qualification on July 13, 2026. Completed historical work has been removed so this file remains an actionable backlog. Findings are explained in [CODE_REVIEW.md](CODE_REVIEW.md).

## P0 — release blockers

- [x] Stop and close queue workers, queues, Redis, PostgreSQL, providers, billing, and log sinks during normal shutdown and on partial router-initialization failures.
- [x] Fix the Web UI production launcher so nginx does not attempt to bind the gateway's port 8080.
- [x] Require a non-default BFF signing secret in production and make secure-cookie behavior configurable and HTTPS-safe.

## P1 — correctness and security

- [x] Add separate process liveness and dependency/worker readiness endpoints with transition metrics.
- [x] Honor the configured BFF `COOKIE_NAME` consistently for writing, reading, and deleting the authentication cookie.
- [x] Protect `/metrics` with a dedicated strong bearer token and document Prometheus authentication.
- [ ] Add dedicated operational metrics/alerts for Redis failures. The current policy is documented: rate-limit errors reject the request, while budget reads fail open and require reconciliation.
- [x] Use exact integer nano-USD for persisted and accumulated currency values, with half-away-from-zero rounding and legacy Redis dual reads.
- [x] Add request timeouts/body limits to all BFF calls and translate gateway/network failures into stable `502/504` responses.
- [x] Apply provider API keys from environment variables as auditable runtime-only overrides without mutating PostgreSQL.
- [x] Complete production configuration validation for Web UI cookie security and public bind choices; gateway secret strength and enabled S3 settings are validated.

## P2 — product completeness

- [x] Implement the paginated, searchable Web UI models page with capabilities, limits, pricing, and frontend/BFF coverage.
- [x] Add bounded gateway billing/usage admin endpoints and implement the Web UI billing page.
- [x] Add bounded dashboard statistics for keys, models, providers, usage, errors, latency, rankings, and monthly cost.
- [x] Implement Vertex AI and AWS Bedrock chat providers with authentication, translation, streaming, usage, validation, cancellation, and error mapping.
- [ ] Add embeddings and other desired OpenAI-compatible endpoints.
- [ ] Add model-catalog/pricing synchronization with validation and rollback.
- [ ] Add provider health checks, fallback routing, and circuit breaking.

## Testing and delivery

- [x] Add BFF tests for login, cookie expiry/name/security, proxy errors, and authorization.
- [ ] Add frontend component/end-to-end tests for API-key lifecycle and authentication expiry.
- [ ] Add a streaming test covering a response longer than 30 seconds and verifying usage/cost persistence.
- [ ] Add provider-billing reconciliation for streams whose terminal usage is unavailable.
- [ ] Add fuzz targets and full-stack staging load profiles; the race suite and in-process response/accounting load/soak regression benchmark are implemented, but no production throughput target is claimed.
- [x] Add CI for Go formatting/tests, frontend lint/component tests/build, Python tests, container build, and migration up/down checks.
- [x] Pin third-party Docker Compose images instead of using floating `latest` tags.
- [ ] Add hardened Kubernetes workload/service/ingress resources; the operations and release-qualification runbooks are now available.

## Documentation follow-up

- [x] Reconcile the detailed files under `docs/` with current environment-variable names, routes, providers, and operational behavior.
- [x] Add an actual tracked root `.env.example` and keep quick-start instructions aligned with it.
- [x] Document and enforce audit body modes, sampling, redaction, size limits, file permissions, Redis lifecycle, and encrypted S3 retention responsibilities.
