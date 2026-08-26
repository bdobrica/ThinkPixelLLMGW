# ADR-0011: Release-candidate scope and deferred work

- Status: Accepted
- Date: 2026-08-26
- Owners: project maintainers
- Supersedes: none
- Superseded by: none

## Context

The gateway reached a strong local qualification milestone, but the experimental Responses API and several deployment-specific controls are incomplete. Calling every typed foundation a supported feature would overstate compatibility. Calling local testing production sign-off would overstate resilience and capacity.

## Decision

Define the August 2026 release-candidate scope as the OpenAI-compatible Chat Completions gateway, administration UI/API, and enabled OpenAI, Vertex AI, and AWS Bedrock chat adapters. Keep `RESPONSES_API_ENABLED=false` in the RC deployment profile.

Compatibility claims come from a dated, machine-readable manifest and executable gates. An internal type, upstream capability, or partial foundation does not expand the public claim. Rollout of Responses, translated providers, streaming, background work, cancellation, or hosted tools requires its own conformance evidence and canary controls.

The following work is deliberately deferred and remains visible here or in the linked qualification documents:

- Responses official-SDK suites, provider translation, SSE HTTP wiring, cancellation/background recovery, event replay, reasoning handling, exact terminal reconciliation, operational guardrails, and hosted-tool security qualification;
- embeddings and other desired OpenAI-compatible endpoints;
- model-catalog/pricing synchronization with validation and rollback;
- provider health-based fallback and circuit breaking;
- automatic reconciliation when streamed terminal usage is absent;
- frontend end-to-end coverage for authentication expiry and API-key lifecycle;
- hardened deployment-specific Kubernetes workload, Service, Ingress, network policy, and secret-manager configuration;
- target-environment full-stack load, S3/provider outage recovery, rolling restart, provider-billing export reconciliation, secret rotation, audit deletion, and named risk-owner sign-off.

## Alternatives considered

- Include the partial Responses API in the RC claim: rejected because streaming, background operations, translations, and official-SDK qualification are incomplete.
- Delay any RC milestone until production sign-off: rejected because the locally qualified Chat Completions artifact is useful for controlled staging and canary work when labeled accurately.
- Remove dormant Responses foundations: rejected because they are isolated behind a default-off flag and provide a tested base for the next milestone.

## Consequences

- The milestone can be tagged as a release candidate without presenting it as production-ready.
- Deferred work is an explicit product/release boundary rather than an accidental omission.
- The Responses database state may ship dormant; retention still applies to any prior canary records.
- Production promotion remains blocked until the staging evidence and human approvals in the release checklist are current.

## Security impact

Default-disabling incomplete routes prevents accidental exposure of unqualified state and tool surfaces. Production still requires named acceptance of residual Redis, provider, accounting, catalog, and deployment risks.

## Operational impact

The RC can proceed to staging with short paid canaries, but not production promotion. Operators must preserve the exact image/config/provider identity and keep Responses disabled. Deferred staging drills require real infrastructure and cannot be replaced by local Compose evidence.

## References and evidence

See [August 2026 RC evidence](../release-evidence-2026-08-26.md), [release qualification](../release-qualification.md), and [Responses API compatibility](../responses-api.md).
