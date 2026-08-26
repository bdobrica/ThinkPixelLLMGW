# Release-candidate evidence — August 26, 2026

This record qualifies the Chat Completions release scope at source revision `52238c96d198946902ca8e139f4305221469a019`. The experimental Responses API remained disabled by default. This is local release-candidate evidence, not production sign-off or a production-capacity claim.

## Identity

- Environment: WSL2 Linux 6.6.87.2, Intel Core i5-9300H, 8 logical CPUs, 15.5 GiB memory.
- Tooling: Go module toolchain 1.26.6, Node 24.11.1, pnpm 10.24.0, Python 3.13.5, Docker 29.0.1.
- Latest migration: `20260824000006_responses_state.up.sql`.
- Image: `llm-gateway@sha256:de3e55a1ff36e31e67fa6091ca6a8e8caf1dcad5d3a1e3aca5756aacd60e3e28`.
- Provider canary profile: OpenAI `gpt-4o-mini`; Vertex AI `google/gemini-2.5-flash-lite` at `global`; Bedrock `eu.amazon.nova-micro-v1:0` in `eu-central-1`.
- Secrets and raw provider responses were not recorded.

## Passed checks

- Go formatting, repository-wide vet, and the full short suite with the race detector.
- Docker-backed PostgreSQL, Redis, encrypted MinIO/S3, queue, and Admin API integration suites.
- Complete migration rollback and reapplication.
- Bootstrap workflow: 7/7 checks passed, including login, API-key creation/verification, and idempotency.
- Frontend lint, 3 component tests, and production build.
- BFF compilation and 20 tests.
- Production container build with pinned Go 1.26.6 and Alpine 3.23 builder/runtime images.
- One short, non-streaming live canary passed for OpenAI, Vertex AI/Gemini, and AWS Bedrock. Each output was capped at eight tokens; no provider load tests were run.
- Trivy 0.69.3 with a database updated August 25, 2026 reported zero High/Critical vulnerabilities, secrets, or misconfigurations in Git-tracked files and the final image. The Go 1.26.5 image initially exposed eight High standard-library findings; upgrading to Go 1.26.6 removed them, and the patched image was rebuilt, retested, and rescanned.
- Thirty-second in-process response/accounting load baseline: 6,860,077 operations, 5,171 ns/op, 3,263 B/op, 50 allocations/op.
- Ten-minute wall-clock in-process soak: 121,737,013 operations with 8 workers and no response-status failure. The original benchmark-calibrated target was replaced with an explicitly bounded soak test after it exceeded its documented wall time.
- Isolated PostgreSQL custom-format backup restored into a separate database; all 11 public tables and seeded provider rows matched before cleanup.
- Live gateway readiness transitioned from 200 to 503 and back to 200 for both Redis and PostgreSQL loss/recovery on an isolated Compose stack.
- Focused tests passed for interrupted streams, client cancellation, unknown accounting, missing-usage metrics, and non-streaming accounting behavior.

## Evidence not established locally

The following require the target staging/production topology and remain release-sign-off blockers, although they do not prevent labeling this revision as a release candidate:

- full-stack load through the real load balancer, TLS, PostgreSQL, Redis, S3, and realistic provider latency;
- S3 outage/recovery with queued Redis buffer drain in a long-running gateway deployment;
- provider outage/recovery drills for all enabled providers without automatic fallback;
- rolling restart with an active provider stream and queued billing, usage, and audit-log records;
- reconciliation against provider billing exports for a non-streaming request, complete stream, and interrupted/unknown-usage stream;
- secret-rotation rehearsals and audit-log lifecycle deletion in the target environment;
- named human owners and refreshed review dates for every accepted risk in `release-qualification.md`;
- hardened deployment-specific Kubernetes workload, Service, Ingress, network policy, and secret-manager configuration.

Do not call the deployment production-ready until those items have current evidence and sign-off. Keep `RESPONSES_API_ENABLED=false` for this release scope.
