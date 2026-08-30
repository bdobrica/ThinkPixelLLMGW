# Guardrails integration contract

ThinkPixelLLMGW integrates with a guardrails evaluator through the ThinkPixelGR `0.1.0` JSON/HTTP contract. The canonical schema is owned and versioned by ThinkPixelGR; this document defines how the gateway consumes that contract without importing GR implementation types.

## Supported wire operation

The adapter calls `POST /v1/evaluations` with `Content-Type: application/json` and, when configured, a bearer token. Requests and responses use the `EvaluationRequest` and `EvaluationResponse` shapes from the ThinkPixelGR OpenAPI contract.

The gateway integration is limited to:

- `pre_model` evaluation before provider dispatch; and
- `post_model` evaluation before releasing model output.

The local adapter supports the wire operation, but runtime enforcement is not enabled in the current release candidate. Enabling it requires a separate qualified change.

## Decision semantics

The gateway, not ThinkPixelGR, enforces the response:

| Action | Gateway behavior when enforcement is enabled |
|---|---|
| `allow` | Continue without expanding any existing authority. |
| `monitor` | Continue and emit privacy-safe evaluation evidence. |
| `block` | Stop before the protected boundary and return a stable gateway policy error. |
| `redact` | Continue only with valid `transformed_content`; otherwise fail closed. |

Unknown actions, malformed responses, mismatched `request_id` values, transport failures, timeouts, and oversized responses are errors. A future rollout must explicitly choose and document fail-open or fail-closed behavior per stage; the adapter itself never invents a decision.

## Identity and data handling

- `request_id` is a gateway correlation identifier and must match in the response.
- `tenant_id` and subject/target identifiers are context only. They do not grant model or Run authority.
- Profile selection is deployment configuration. Untrusted request content cannot select a less restrictive profile or disable mandatory GR policy.
- Only the minimum content required for the configured evaluation is sent. Credentials, provider authentication material, hidden reasoning, and raw internal errors must not be included.
- Evaluation findings are non-authoritative evidence. They must follow the gateway's audit minimization and retention controls.

## Compatibility and rollout

The client rejects wire responses outside the understood decision set. Before runtime enablement, compatibility fixtures must cover the configured GR version, and qualification must cover pre-model blocking/redaction, post-model buffering/redaction, streaming cancellation and accounting, timeout behavior, audit minimization, and dependency health semantics.

See [ADR-0012](../adr/0012-optional-guardrails-boundary.md) for the architectural decision.
