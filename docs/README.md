# Documentation

The root README is the concise entry point. Durable architecture, contracts, security/operations guidance, and release evidence live here.

## Architecture and contracts

- [Repository alignment and ownership](../ALIGNMENT.md)
- [Architecture decision records](adr/README.md)
- [Guardrails integration contract](contracts/guardrails.md)
- [Responses API compatibility](responses-api.md)
- [Database schema](database-schema.md)

## Development and operation

- [Quick start](quickstart.md)
- [Environment variables](env-variables.md)
- [Provider configuration](providers.md)
- [Testing guide](testing-guide.md)
- [Metrics](metrics.md)
- [Logging privacy](logging-privacy.md)
- [Operations runbook](operations-runbook.md)
- [Release qualification](release-qualification.md)
- [Release-candidate evidence](release-evidence-2026-08-26.md)

Current implementation sequencing belongs in `PLAN.md` or `TODO.md` when those files exist. Accepted decisions are not rewritten to match later implementation; a material change requires a superseding ADR.
