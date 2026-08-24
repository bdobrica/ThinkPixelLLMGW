# ThinkPixelLLMGW backlog

Last reconciled with the code review, release qualification, and Responses API plan on August 24, 2026. Completed historical work has been removed so this file remains an actionable backlog. Findings are explained in [CODE_REVIEW.md](CODE_REVIEW.md).

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
- [ ] Implement the OpenAI-compatible Responses API as a first-class, item-oriented protocol (detailed execution plan: `PLAN.md`, Steps 20–27).
  - Step 25 has a typed SSE encoder/state-machine foundation with gateway-owned sequence numbers, lifecycle/index ordering, terminal-only usage, canonical named frames, and prompt flushing; native provider translation and HTTP streaming remain pending.
  - Step 26 has begun with tenant-scoped stored-response retrieval and soft deletion, public envelope reconstruction, ordered output loading, provider-ID non-disclosure, and opt-in opaque reasoning recovery; cancellation, background work, observability, reconciliation, capacity controls, and SDK examples remain pending.
  - [x] Freeze a dated OpenAI contract snapshot and capability matrix; add typed discriminated schemas for requests, response envelopes, content parts, output items, statuses, usage, errors, and supported SSE events.
  - [ ] Add `POST /v1/responses` plus authenticated `GET`/`DELETE /v1/responses/{response_id}` and cancellation/background resource operations; generate stable `resp_` and item/call IDs. POST and tenant-scoped GET/DELETE foundations are implemented; cancellation/background operations and retrieval options remain.
  - [x] Persist tenant-owned response state, ordered items, predecessor links, lifecycle transitions, usage, tool executions, retention, opaque-payload encryption, cleanup, and orphan recovery. Event journals remain part of the streaming/background step.
  - [x] Implement `previous_response_id` continuation without implicitly inheriting prior instructions; reject missing, expired, deleted, non-stored, or cross-tenant predecessors safely. Native providers use a tenant-scoped gateway-ID-to-upstream-ID mapping; translated providers will use the stored item chain.
  - [x] Implement the provider-neutral context limiter with `truncation: "disabled"` as the default error behavior and deterministic `"auto"` removal of oldest eligible items while preserving current instructions/input and tool call/output pairs. Wiring it into translated providers remains gated with those adapters.
  - [ ] Track provider-supplied reasoning items/summaries/encrypted content separately from visible text, never synthesize or log hidden chain-of-thought, and account for reasoning tokens without double counting.
  - [ ] Support standard client-executed function tools, tool choice, strict JSON schemas, parallel calls, streamed arguments, and correlated `function_call_output`; enforce orchestration round/call/time/token limits. Initial non-streaming native support now validates bounded schemas/tool choice, correlates outputs against unresolved tenant-owned calls, rejects duplicates/unknown IDs, and normalizes ordered parallel call items; streaming and loop limits remain.
  - [ ] Add a default-disabled hosted-tool executor framework with per-key/model allowlists, cancellation, quotas, cost reporting, audit controls, health metrics, and idempotent execution. The initial typed registry/runner now provides default-deny deployment/model/key authorization, health discovery, validation, cancellation/deadlines, concurrency/call/byte limits, safe lifecycle events, separate nano-USD usage, and in-response idempotency; durable records, orchestration wiring, configuration, metrics, and circuit breakers remain.
    - [ ] Web search: configured/native backend, SSRF and redirect protection, domain/result limits, citations, and separate tool charging.
    - [ ] File search: tenant-owned ingestion/vector stores, filters/ranking, citations, malware/size checks, bounded retrieval, deletion, and retention.
    - [ ] Code interpreter: ephemeral hardened sandbox, no host secrets/mounts, default-deny network, strict CPU/memory/process/time/disk/output limits, artifact ownership, and guaranteed cleanup; remain disabled until isolation is proven.
  - [ ] Implement a Responses-specific SSE state machine with named events, monotonic sequence numbers, correct item/content indices, text/reasoning/tool deltas, exactly one terminal event, final usage, cancellation, backpressure, and optional persisted replay for background work; do not emit Chat Completions `[DONE]`.
  - [ ] Reuse gateway auth, model authorization, rate limiting, budgets, exact billing, usage queues, audit privacy, timeouts, shutdown, and readiness while recording `/v1/responses` and terminal status exactly once across multi-round/tool flows.
  - [ ] Add golden event/item fixtures, migration/repository tests, state and tenant-isolation tests, provider conformance, hosted-tool security tests, long-stream/recovery/load tests, and official OpenAI SDK smoke tests.
  - [ ] Publish a dated field/event/provider/tool compatibility matrix and SDK examples; gate rollout by feature flag and do not advertise deferred fields, background mode, provider translations, or hosted tools before their suites pass.
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
