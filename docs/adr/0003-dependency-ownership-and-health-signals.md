# ADR-0003: Dependency ownership and health signals

- Status: Accepted
- Date: 2026-07-12
- Owners: project maintainers
- Supersedes: none
- Superseded by: none

## Context

Router construction created workers, queues, providers, log sinks, Redis, and PostgreSQL resources without one owner. Partial startup failures and normal shutdown could leak them or lose queued work. The original health endpoint also reported process liveness even when required dependencies were unavailable.

## Decision

The gateway dependency container owns every resource it constructs. Construction is transactional: a failure closes all previously created resources. Shutdown is idempotent, deadline-aware, aggregates errors, stops producers/workers in a documented order, flushes where possible, and then closes queues, providers, clients, and storage. Redis-backed queues survive process restart; in-memory work does not.

Expose separate signals:

- `/health` is a cheap process-liveness check.
- `/ready` concurrently checks PostgreSQL, Redis, provider-registry access, and essential workers within a short bound and returns a stable, non-sensitive `503` on failure.
- readiness state and transitions use low-cardinality metrics.
- `/metrics` requires a dedicated strong bearer token and should remain on a private deployment boundary.

## Alternatives considered

- Let each package manage its own process lifetime: rejected because partial construction and shutdown ordering span package boundaries.
- Use one health endpoint for both liveness and readiness: rejected because dependency outages would either restart healthy processes or route traffic to unusable ones.
- Publish metrics anonymously: rejected because operational labels and state aid reconnaissance.

## Consequences

- Startup rollback and shutdown behavior can be tested with owned fakes and bounded deadlines.
- Orchestrators can keep a live process running while removing an unhealthy instance from service.
- Readiness does not promise that every external provider is healthy; provider outage policy remains operational and model-specific.

## Security impact

Health responses deliberately omit dependency details. Metrics use a separate credential and constant-time comparison; deployment networking should still keep them private. Readiness failure is fail-closed for traffic admission.

## Operational impact

Shutdown and startup rollback are bounded and observable. Redis-backed work is recoverable across restart, while in-memory work is explicitly lossy. Orchestrators must configure distinct liveness/readiness probes and sufficient termination grace.

## References and evidence

Implemented in commits `d4d7508`, `ad13fec`, and `a812dab`. See [operations runbook](../operations-runbook.md), [metrics](../metrics.md), and [environment variables](../env-variables.md).
