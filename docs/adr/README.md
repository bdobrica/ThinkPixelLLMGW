# Architecture decision records

Architecture decision records (ADRs) preserve decisions that should outlive the implementation plan and release backlog. They describe why the gateway is built this way, the important consequences, and where the implemented behavior is documented.

An ADR is not a release checklist. Current operational qualification belongs in [release qualification](../release-qualification.md), and dated evidence belongs in the corresponding release-evidence record.

## Naming and lifecycle

- Name records `NNNN-short-kebab-case-title.md`, beginning with `0001`.
- Copy [template.md](template.md); do not repurpose an accepted ADR.
- Allowed statuses are `Proposed`, `Accepted`, `Superseded by ADR-NNNN`, and `Deprecated`.
- Materially changing an accepted decision requires a new ADR that supersedes it.
- Keep alternatives and consequences honest; an ADR is a decision record, not promotional documentation.

## Index

| ADR | Status | Decision |
|---|---|---|
| [ADR-0001: Tiered testing and release evidence](0001-tiered-testing-and-release-evidence.md) | Accepted |
| [ADR-0002: Streaming deadlines and accounting](0002-streaming-deadlines-and-accounting.md) | Accepted |
| [ADR-0003: Dependency ownership and health signals](0003-dependency-ownership-and-health-signals.md) | Accepted |
| [ADR-0004: Web BFF security and deployment boundary](0004-web-bff-security-and-deployment-boundary.md) | Accepted |
| [ADR-0005: Exact nano-USD accounting](0005-exact-nano-usd-accounting.md) | Accepted |
| [ADR-0006: Provider credential precedence](0006-provider-credential-precedence.md) | Accepted |
| [ADR-0007: Audit data minimization and retention](0007-audit-data-minimization-and-retention.md) | Accepted |
| [ADR-0008: Provider adapter contract](0008-provider-adapter-contract.md) | Accepted |
| [ADR-0009: Responses API state and routing](0009-responses-api-state-and-routing.md) | Accepted |
| [ADR-0010: Responses tools and streaming boundaries](0010-responses-tools-and-streaming-boundaries.md) | Accepted |
| [ADR-0011: Release-candidate scope and deferred work](0011-release-candidate-scope-and-deferred-work.md) | Accepted |
