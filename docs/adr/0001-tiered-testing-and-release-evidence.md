# ADR-0001: Tiered testing and release evidence

- Status: Accepted
- Date: 2026-07-12
- Owners: project maintainers
- Supersedes: none
- Superseded by: none

## Context

The original default Go test command mixed unit tests with PostgreSQL, Redis, MinIO, and socket-dependent tests. That made ordinary feedback environment-dependent and prevented CI from being a trustworthy release gate. Live cloud-provider tests also have cost and credential implications, while production behavior cannot be inferred from an in-process benchmark.

## Decision

Use four explicit verification tiers:

1. Hermetic checks run without external services: formatting, vet, short race-enabled Go tests, BFF tests, and frontend lint/test/build.
2. Integration checks explicitly provision isolated PostgreSQL, Redis, and encrypted MinIO services and always tear them down.
3. Credential-gated provider canaries make one short request per enabled provider. They skip when configuration is absent and are never load tests.
4. Staging qualification exercises the actual load balancer, TLS, data services, provider latency, dependency loss, rolling restart, restore, reconciliation, and security operations.

CI enforces the first two tiers plus migrations and container builds. Release claims must identify the exact revision, image, environment, provider profile, and evidence. In-process load and soak results are regression baselines, not capacity claims.

## Alternatives considered

- Run all tests in one default command: rejected because it makes basic feedback depend on local infrastructure and cloud credentials.
- Mock every dependency and provider: useful for deterministic tests, but insufficient for migrations, protocol integration, cloud identity, or target-topology behavior.
- Treat CI success as production sign-off: rejected because CI does not exercise the deployed network, recovery procedures, or provider billing exports.

## Consequences

- Contributors get a dependable service-free test command and deterministic integration lifecycle.
- Paid provider use remains small and intentional.
- A locally passing release candidate is not called production-ready without staging evidence and named risk owners.
- Scanner output, dumps, credentials, and raw provider responses are excluded from committed evidence.

## Security impact

Live tests remain credential-gated and must not print secrets or raw provider responses. Separating paid canaries from load tests limits accidental spend and provider abuse. Security scans remain release gates, but findings still require reachability and deployment review.

## Operational impact

Integration jobs need Docker and deterministic teardown. Staging qualification needs a representative deployment and human owners. Every release record must distinguish local regression results from actual capacity and resilience evidence.

## References and evidence

The baseline was established in commits `9fb4042`, `e4447f9`, and `6ed7b8d`; release tooling was extended in `2f66706` and `52238c9`. See [testing guide](../testing-guide.md), [release qualification](../release-qualification.md), and [August 2026 RC evidence](../release-evidence-2026-08-26.md).
