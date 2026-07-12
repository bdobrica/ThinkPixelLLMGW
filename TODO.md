# ThinkPixelLLMGW backlog

Last reconciled with the code review on July 12, 2026. Completed historical work has been removed so this file remains an actionable backlog. Findings are explained in [CODE_REVIEW.md](CODE_REVIEW.md).

## P0 — release blockers

- [ ] Stop and close queue workers, queues, Redis, and PostgreSQL during normal shutdown and on partial router-initialization failures.
- [ ] Fix the Web UI production launcher so nginx does not attempt to bind the gateway's port 8080.
- [ ] Require a non-default BFF signing secret in production and make secure-cookie behavior configurable and HTTPS-safe.

## P1 — correctness and security

- [ ] Make the configured BFF `COOKIE_NAME` work; the dependency currently always reads a cookie named `admin_token`.
- [ ] Protect `/metrics` when deployed outside a trusted network, or explicitly document the exposure decision.
- [ ] Define fail-open versus fail-closed policy for Redis failures in rate limiting and billing, and add operational metrics/alerts.
- [ ] Use exact decimal or integer minor units for persisted and accumulated currency values instead of `float64`.
- [ ] Add request timeouts/body limits to all BFF calls and translate gateway/network failures into stable `502/504` responses.
- [ ] Avoid persisting provider API keys from environment variables back into PostgreSQL on every startup, or document and audit that behavior.
- [ ] Validate production configuration at startup (secret strength, S3 settings when enabled, cookie security, and public bind choices).

## P2 — product completeness

- [ ] Implement the Web UI models page.
- [ ] Add a gateway billing/usage admin endpoint and implement the Web UI billing page.
- [ ] Add dashboard statistics.
- [ ] Implement Vertex AI and Bedrock providers or remove them from advertised provider support until ready.
- [ ] Add embeddings and other desired OpenAI-compatible endpoints.
- [ ] Add model-catalog/pricing synchronization with validation and rollback.
- [ ] Add provider health checks, fallback routing, and circuit breaking.

## Testing and delivery

- [ ] Add BFF tests for login, cookie expiry/name/security, proxy errors, and authorization.
- [ ] Add frontend component/end-to-end tests for API-key lifecycle and authentication expiry.
- [ ] Add a streaming test covering a response longer than 30 seconds and verifying usage/cost persistence.
- [ ] Add provider-billing reconciliation for streams whose terminal usage is unavailable.
- [ ] Add race, fuzz, and load tests; establish measured latency/throughput targets rather than advertising unverified numbers.
- [ ] Add CI for Go formatting/tests, frontend build/lint, Python tests, container build, and migration up/down checks.
- [ ] Pin container images instead of using floating `latest` tags.
- [ ] Add Kubernetes deployment resources and an operations/runbook only after the release blockers are resolved.

## Documentation follow-up

- [ ] Reconcile the detailed files under `docs/` with current environment-variable names and routes.
- [ ] Add an actual root `.env.example` or remove instructions that reference it from older guides.
- [ ] Document data retention/redaction policy for prompts, responses, request files, Redis, and S3.
