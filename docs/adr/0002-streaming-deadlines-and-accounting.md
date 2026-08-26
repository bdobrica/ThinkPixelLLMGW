# ADR-0002: Streaming deadlines and accounting

- Status: Accepted
- Date: 2026-07-12
- Owners: project maintainers
- Supersedes: none
- Superseded by: none

## Context

A global HTTP write timeout terminated legitimate long-lived SSE responses. The streaming path also recorded zero usage and cost unless it could observe a provider's terminal usage event. Network reads are not SSE event boundaries, and interrupted streams do not always carry terminal usage.

## Decision

Keep bounded header-read, ordinary request/read/write, idle, provider, and shutdown timeouts, but clear the response write deadline only for the active streaming response. Provider contexts and the shutdown grace period remain authoritative bounds.

For OpenAI-compatible streams, preserve caller stream options while requesting terminal usage. Parse complete SSE frames independently of network reads, capture prompt, cached, completion, and reasoning tokens, and pass them through the same pricing path as non-streaming responses. Finalize billing, durable usage, logs, and metrics exactly once.

If a stream ends without terminal usage, do not invent zero usage or cost. Persist an explicit unknown-accounting outcome, increment the missing-usage metric, and require reconciliation.

## Alternatives considered

- Disable the server write timeout globally: rejected because ordinary responses would lose an important bound.
- Buffer the entire stream and calculate after completion: rejected because it increases latency and memory and defeats direct streaming.
- Estimate or record zero when usage is missing: rejected because it silently corrupts budgets and billing.

## Consequences

- Healthy streams can exceed the ordinary write timeout without becoming unbounded upstream work.
- Complete streams are billed consistently with non-streaming requests.
- Interrupted/provider-incomplete accounting is visible, but automated provider-billing reconciliation remains deferred.
- The parser must retain state across arbitrary read boundaries without buffering the full generated response.

## Security impact

Request-scoped deadlines retain protection against stalled upstreams and slow clients. Usage parsing accepts only framed provider events and must not cause raw payloads or hidden reasoning to enter logs. Unknown accounting fails visibly instead of granting an undetected cost bypass.

## Operational impact

Operators must alert on missing-usage metrics and reconcile unknown records. Shutdown deadlines may terminate a long stream, so deployment drain periods must reflect expected stream duration.

## References and evidence

Implemented in commits `23367ef` and `149f5e4`. Operational behavior and remaining reconciliation work are documented in [metrics](../metrics.md), [operations runbook](../operations-runbook.md), and [release qualification](../release-qualification.md).
