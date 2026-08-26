# ADR-0005: Exact nano-USD accounting

- Status: Accepted
- Date: 2026-07-12
- Owners: project maintainers
- Supersedes: none
- Superseded by: none

## Context

Binary floating-point accumulation can drift and behave unpredictably at budget boundaries. Provider prices can be small enough that micro-USD precision is insufficient, while existing JSON and Prometheus consumers expect dollar-valued numbers.

## Decision

Represent accumulated and persisted currency as signed integer nano-USD (`10^-9 USD`). Convert at system boundaries using half-away-from-zero rounding. Use exact SQL numeric values for unit prices, integer increments and comparisons for budgets/totals, versioned queue fields, and dual-read/rewrite behavior for legacy Redis dollar totals.

Dollar-valued JSON and Prometheus fields remain compatibility/display boundaries only; calculations occur in the exact representation.

## Alternatives considered

- Keep `float64`: rejected because repeated accumulation and boundary comparisons are not exact.
- Use integer micro-USD: rejected because it cannot represent all configured low per-token prices with sufficient precision.
- Use an arbitrary-precision decimal at every layer: viable, but more complex for Redis atomic increments and queue compatibility than a fixed integer scale.

## Consequences

- Repeated accumulation is deterministic and budget equality/one-unit-over comparisons are exact.
- All new monetary fields must state their unit explicitly.
- Conversion overflow and rounding are boundary concerns that require tests.
- External provider billing still requires reconciliation; exact internal arithmetic cannot repair missing provider usage.

## Security impact

Exact comparisons reduce budget-bypass risk at rounding boundaries. Inputs still need overflow, sign, and scale validation so malicious prices or usage cannot wrap stored totals.

## Operational impact

Schema and queue consumers must preserve the nano-USD unit. Legacy Redis values are migrated on read. Dashboards and metrics convert to USD only for presentation, and provider-export reconciliation remains necessary.

## References and evidence

Implemented in commit `ad9e444` and migration `20260712000004`. See [database schema](../database-schema.md), [metrics](../metrics.md), and the billing/usage API documentation in the root README.
