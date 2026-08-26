# ADR-0006: Provider credential precedence

- Status: Accepted
- Date: 2026-07-12
- Owners: project maintainers
- Supersedes: none
- Superseded by: none

## Context

Gateway startup previously copied environment credentials into provider rows. Startup therefore mutated durable configuration, made rotation surprising, and risked persisting plaintext supplied by the deployment environment.

## Decision

Provider records remain the durable configuration source and their stored credential fields are encrypted. Matching environment credentials are runtime-only overrides applied after database decryption whenever the registry loads or reloads. They take precedence without updating PostgreSQL.

Missing/disabled providers and storage failures remain distinct typed failures. Audit records may contain provider name, source, and credential field names, but never values. Rotation means updating the deployment secret and restarting the gateway. Prefer cloud workload identity/ADC/IAM roles over static credentials where supported.

Gateway JWT, metrics, BFF, encryption, and enabled S3 configuration are validated before dependency startup. The encryption key is exactly 32 bytes encoded as 64 hexadecimal characters.

## Alternatives considered

- Synchronize environment credentials into PostgreSQL at every startup: rejected because startup would mutate durable state and persist deployment-supplied values unexpectedly.
- Use only database credentials: rejected because workload identities and secret-manager injection are preferable in many deployments.
- Add an explicit synchronization command: viable for some operators, but unnecessary for the selected runtime-override model and not implemented.

## Consequences

- Starting the process is not a credential synchronization operation.
- Environment rotation is auditable and leaves no new database plaintext.
- Runtime overrides require a restart and must match an existing enabled provider record.
- Changing the maintenance encryption key requires the documented re-encryption procedure rather than ordinary secret rotation.

## Security impact

Secrets are never included in audit values or configuration JSON. Stored credential fields remain encrypted, while workload identity can avoid static secrets entirely. Weak shared secrets and malformed encryption keys fail before dependencies start.

## Operational impact

Credential rotation requires deployment-secret update plus restart. Operators must retain the existing encryption key for stored data or perform the maintenance re-encryption procedure. Override/provider-name mismatches cause explicit startup/reload failures.

## References and evidence

Implemented in commits `d790acd` and `a812dab`; key generation was corrected in `8a85e73`. See [provider configuration](../providers.md), [environment setup](../env-setup.md), and [operations runbook](../operations-runbook.md).
