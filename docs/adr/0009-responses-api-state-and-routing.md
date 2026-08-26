# ADR-0009: Responses API state and routing

- Status: Accepted
- Date: 2026-08-25
- Owners: project maintainers
- Supersedes: none
- Superseded by: none

## Context

OpenAI's Responses API is item-oriented, stateful, and has a different lifecycle from Chat Completions. A gateway must maintain tenant authorization, retention, stable public IDs, billing, and portability even when an upstream provider has native response state. Replaying history through the gateway when a native provider can continue it would waste tokens and risk semantic drift.

## Decision

Implement `/v1/responses` as a separate typed protocol and route family, disabled by default behind `RESPONSES_API_ENABLED`.

The gateway always owns the public `resp_`, item, call, and tool-execution namespaces and tenant boundary. It persists response envelopes, ordered items, predecessor links, lifecycle transitions, usage/errors, tool executions, expiry/deletion state, and encrypted opaque provider state. Lookups always include the API-key owner; missing, foreign, expired, deleted, and non-stored resources are indistinguishable. Terminalization is transactional and idempotent; bounded cleanup and stale-work reconciliation are multi-instance safe.

For an upstream with native Responses support, store a tenant-scoped mapping from gateway response ID to upstream ID and forward only the upstream predecessor ID. The provider manages model conversation state; the gateway still owns authorization, audit, deletion/retention, routing, and public identity. Provider failover is not promised for that chain.

For translated providers, reconstruct context from ordered stored items. Never inherit predecessor instructions implicitly. `truncation: disabled` fails closed when context is too large; `auto` deterministically removes the oldest eligible items while preserving current instructions/input and keeping tool call/output pairs indivisible. An estimator reserves context headroom when an exact tokenizer is unavailable.

This decision defines the architecture, not a general-availability claim. The route family remains experimental and default-disabled until the compatibility suites named below pass.

## Alternatives considered

- Blindly proxy native provider IDs to clients: rejected because it breaks tenant scoping, retention, deletion, routing, and provider abstraction.
- Always replay the complete gateway history: rejected for native providers because it wastes tokens and can change provider-managed semantics.
- Rely entirely on provider state: rejected because translated providers and gateway-owned authorization/resource operations still need durable state.
- Silently truncate by default: rejected because hidden history loss changes behavior; disabled/fail-closed is the default.

## Consequences

- Native pass-through avoids unnecessary replay while preserving gateway policy.
- Gateway-owned IDs prevent leaking provider IDs and cross-tenant continuation.
- `store: false` still needs short-lived orchestration state but is unavailable for later public retrieval.
- Native chains cannot transparently fail over to a translated provider.
- New upstream fields are unsupported until typed, validated, gated, and covered by fixtures; permissive decoding does not imply compatibility.

## Security impact

All resource and predecessor queries include tenant ownership and use indistinguishable not-found behavior. Provider correlation IDs are not public. Opaque reasoning state is encrypted and excluded from audit-visible JSON; hidden chain-of-thought must never be synthesized or logged.

## Operational impact

State introduces PostgreSQL retention, cleanup, reconciliation, and capacity obligations. Native chains are pinned to their provider and cannot transparently fail over. The feature flag is the immediate rollback control; dormant records still obey retention.

## References and evidence

Non-streaming native OpenAI create plus tenant-scoped retrieve/delete foundations exist. Background execution, cancellation, replay options, translated Vertex/Bedrock routes, and full official-SDK qualification remain deferred. See [Responses API compatibility](../responses-api.md) for the dated capability matrix and exact current surface.
