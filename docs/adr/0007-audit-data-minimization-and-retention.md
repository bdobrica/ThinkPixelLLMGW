# ADR-0007: Audit data minimization and retention

- Status: Accepted
- Date: 2026-07-12
- Owners: project maintainers
- Supersedes: none
- Superseded by: none

## Context

Prompts and responses may contain personal, confidential, or regulated data. Header redaction alone does not establish a safe retention policy, and local files, Redis buffering, S3 archives, replicas, and backups have different deletion semantics.

## Decision

Use a shared audit-body policy with `none`, `hash`, and `redacted` modes. Default local bodies to hashes and byte counts and omit S3 request/response payloads. Redacted mode is opt-in, recursively removes default and configured sensitive fields, enforces a size cap, and falls back safely for oversized or non-JSON input. Known credential headers, query values, bearer strings, and API-key patterns are never retained.

Local files use restrictive permissions and bounded rotation. Redis is a bounded transient queue whose records are acknowledged only after successful S3 upload. S3 objects are compressed and request server-side AES-256 encryption. Operators own lifecycle expiry, version/back-up deletion, least-privilege access, regional placement, lawful basis, and mapping deletion requests to identifiers.

## Alternatives considered

- Log complete payloads by default: rejected because observability does not justify broad content retention.
- Never retain any body-derived signal: safer for content, but hashes and sizes are useful for correlation without plaintext.
- Rely only on a fixed redaction list: rejected because arbitrary prose and tenant-specific fields cannot be comprehensively classified.

## Consequences

- Useful operational metadata remains available without retaining content by default.
- Redaction cannot identify every secret embedded in arbitrary prose; deployments handling sensitive content should choose `none` or `hash`.
- Changing prefixes or sampling does not delete historical objects.
- Production sign-off requires testing the actual storage lifecycle and deletion policy.

## Security impact

The default minimizes data exposure and strips common credential forms. Hashes can still be sensitive for low-entropy content, and opt-in redacted bodies can retain confidential prose. Access, encryption, deletion, and regional controls remain deployment responsibilities.

## Operational impact

Sampling and size caps bound storage growth. S3 outages accumulate bounded Redis work; operators must monitor capacity and verify drain after recovery. Lifecycle rules must cover versions, replicas, and backups rather than only current objects.

## References and evidence

Implemented in commit `1133d34`. See [audit logging privacy and retention](../logging-privacy.md) and [release qualification](../release-qualification.md).
