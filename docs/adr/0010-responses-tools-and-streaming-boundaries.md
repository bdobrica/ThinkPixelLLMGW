# ADR-0010: Responses tools and streaming boundaries

- Status: Accepted
- Date: 2026-08-25
- Owners: project maintainers
- Supersedes: none
- Superseded by: none

## Context

Responses function tools are client-executed, while web search, file search, and code interpreter are server-managed and create materially different security, cost, ownership, and recovery obligations. Responses streaming also uses named lifecycle events rather than Chat Completions chunks and `[DONE]`.

## Decision

Treat custom functions and hosted tools as separate execution models.

Client functions are validated against bounded object JSON Schemas and explicit tool-choice rules. The gateway returns ordered `function_call` items; a later tenant-owned request submits the matching unresolved `function_call_output`. Unknown, duplicate, or cross-chain call IDs fail before provider work. Parallel call order is preserved.

Hosted tools use a provider-neutral registry/runner but remain default-disabled. Registration is not authorization: deployment, selected model, and API key must all allow a tool. Execution has cancellation/deadlines, health checks, concurrency/call/byte quotas, idempotent call IDs, separate nano-USD usage, and safe lifecycle events that exclude arguments, results, credentials, and raw backend errors. No production web/file/code executor is enabled until its egress, ownership, isolation, durable execution, observability, and recovery reviews pass.

The gateway owns the Responses SSE state machine. Frames have named `event` and JSON `data` lines, monotonic sequence numbers, valid item/content indices, deltas only after add events, exactly one terminal event, and terminal-only usage. It never forwards provider-specific frames or emits Chat Completions `[DONE]`.

This decision fixes the security and protocol boundaries; it does not enable hosted tools or HTTP streaming. Those surfaces remain disabled until their acceptance evidence exists.

## Alternatives considered

- Execute all declared functions inside the gateway: rejected because client functions are caller-owned code and have no server executor contract.
- Let executor registration imply availability: rejected because installation does not establish tenant/model authorization or safe operations.
- Forward provider SSE frames directly: rejected because IDs, ordering, lifecycle, usage, and event names are gateway contract responsibilities.
- Reuse Chat Completions `[DONE]`: rejected because the Responses protocol has typed terminal events.

## Consequences

- Client code never executes inside the gateway merely because it was declared as a function.
- Hosted execution cannot be enabled by installing an executor alone.
- The state machine can reject invalid provider event order before exposing it to clients.
- Stream transport, provider-frame translation, cancellation propagation, terminal persistence/billing, durable replay, and orchestration round/call/time/token limits must pass before `stream: true` is advertised.

## Security impact

Hosted tools are default-deny at deployment, model, and key scopes. Executors require bounded input/output, time, concurrency, cancellation, and safe events. Web search needs SSRF/redirect/egress controls; file search needs tenant ownership and malware/retention controls; code execution needs a hardened ephemeral sandbox with no host secrets or mounts.

## Operational impact

Hosted execution creates separate cost, quota, health, circuit-breaker, durable-record, and cleanup requirements. Streaming needs backpressure, cancellation, terminal persistence, exact-once billing, and optional event journals for background replay.

## References and evidence

The typed function validation/correlation, hosted-tool runner foundation, and SSE encoder/state machine exist. Production hosted executors and HTTP streaming remain disabled. See [Responses API compatibility](../responses-api.md).
