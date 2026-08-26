# ADR-0008: Provider adapter contract

- Status: Accepted
- Date: 2026-07-13
- Owners: project maintainers
- Supersedes: none
- Superseded by: none

## Context

The gateway promises an OpenAI-compatible Chat Completions surface while routing to providers with different authentication, request schemas, streaming protocols, usage fields, and errors. Provider names must not be advertised merely because structural stubs exist.

## Decision

Enable a provider only after its adapter implements authentication, validation, request/response translation, streaming, usage extraction, cancellation/timeouts, error mapping, and cleanup with deterministic conformance tests.

- OpenAI uses its compatible Chat Completions transport.
- Vertex AI uses Google's OpenAI-compatible endpoint with OAuth from service-account JSON, short-lived tokens, or ADC; `global` uses the global control-plane hostname.
- Bedrock uses AWS SDK v2 Converse/ConverseStream with SigV4 and the default credential chain or encrypted static/session credentials. Unsupported tool-role or non-text inputs fail explicitly rather than being dropped.

Catalog capability flags can narrow a provider's support, but cannot enable behavior the adapter does not implement. Live qualification uses one short credential-gated canary for every enabled provider; no paid-provider load tests are run.

## Alternatives considered

- Forward every request unchanged to every provider: rejected because Bedrock is not wire-compatible and provider capabilities differ.
- Advertise structural stubs and fail at runtime: rejected because it creates misleading, potentially billable failures.
- Normalize only non-streaming text: rejected for enabled Chat Completions providers because streaming and usage are core gateway behavior.

## Consequences

- Callers receive one gateway contract while unsupported combinations fail before billable work.
- Provider-specific IDs, stop reasons, errors, and token/cache usage require explicit translation.
- There is no automatic fallback or circuit breaker yet; operators disable or reroute models during an outage.
- Model/pricing catalog synchronization and native Bedrock tool/multimodal translation remain deferred.

## Security impact

Cloud authentication uses scoped OAuth/IAM identities and preserves cancellation/timeouts. Unsupported content fails before billable work instead of being silently discarded. Provider errors must be normalized without leaking credentials or internal identity details.

## Operational impact

Every enabled provider needs a target-region/model canary and outage runbook. There is no automatic failover, and adapters must track upstream protocol changes. Live qualification is intentionally tiny to control cost.

## References and evidence

Vertex AI and Bedrock were implemented in commits `d085408` and `0380d2c`; global Vertex routing and repeatable live canaries were corrected in `8a85e73`. See [provider configuration](../providers.md) and [August 2026 RC evidence](../release-evidence-2026-08-26.md).
