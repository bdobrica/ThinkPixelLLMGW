# ADR-0012: Optional guardrails evaluator boundary

- Status: Accepted
- Date: 2026-08-30
- Owners: project maintainers
- Supersedes: none
- Superseded by: none

## Context

The ThinkPixel platform separates guardrail evaluation from enforcement. ThinkPixelGR owns policy resolution, detectors, findings, and decisions; the service protecting a boundary owns enforcement. Coupling GR types into the gateway domain, sharing its storage, or treating an `allow` as authorization would violate component ownership. Enabling post-model checks without buffering would also expose content before a decision and conflict with the existing streaming lifecycle and accounting guarantees.

## Decision

Introduce an optional guardrails port and a replaceable JSON/HTTP adapter for the versioned ThinkPixelGR evaluation API. The gateway consumes only the wire contract and does not import GR implementation packages or access GR storage.

When runtime enforcement is qualified, the gateway will invoke `pre_model` before provider dispatch and `post_model` before output release. It will enforce `block` and valid transformations locally. Guardrail decisions can narrow behavior but cannot grant model access, budget, tenant, Run, tool, or other authority.

The adapter validates status, response size, JSON shape, decision action, and request correlation. Transport and protocol errors remain explicit; failure policy belongs to the enforcement orchestration and must be configured and tested per stage.

This ADR accepts the boundary and adapter, not runtime enablement. Chat Completions and Responses enforcement remain disabled until buffering, SSE behavior, cancellation, billing/accounting, audit minimization, readiness, and deployment failure modes have qualification evidence.

## Alternatives considered

- Embed guardrail logic in the gateway: rejected because policy evaluation belongs to ThinkPixelGR and must remain replaceable.
- Import ThinkPixelGR internal Go types: rejected because cross-component integration uses a versioned wire contract.
- Treat GR as an authorizer: rejected because content evaluation cannot create Run or model authority.
- Immediately inspect streaming output chunk by chunk: rejected because partial checks can release disallowed content and do not preserve transformation or terminal accounting semantics.

## Consequences

- The gateway has a testable integration seam without making an unsupported release claim.
- ThinkPixelGR can be replaced by a contract-compatible evaluator.
- Runtime rollout requires explicit orchestration, configuration, metrics, and release evidence rather than merely setting an endpoint.
- Post-model enforcement adds buffering and latency and may require a distinct supported streaming mode.

## Security impact

The integration sends only required model-adjacent content over an authenticated, bounded request. Credentials and hidden reasoning are excluded. Unknown or malformed decisions are never interpreted as permission. Profiles are operator-controlled and guardrail results cannot broaden upstream authority.

## Operational impact

Operators must budget a separate timeout, monitor GR dependency health and latency, and choose documented failure behavior. Readiness semantics and rollback controls must be settled before enablement. A GR outage cannot be silently reclassified as an allow decision by the adapter.

## References and evidence

See the [guardrails integration contract](../contracts/guardrails.md), [ADR-0002](0002-streaming-deadlines-and-accounting.md), [ADR-0003](0003-dependency-ownership-and-health-signals.md), [ADR-0007](0007-audit-data-minimization-and-retention.md), and [ADR-0011](0011-release-candidate-scope-and-deferred-work.md).
