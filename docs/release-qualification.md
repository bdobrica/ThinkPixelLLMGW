# Release qualification

This checklist qualifies a specific source revision and deployment profile; it is not a blanket throughput or production-safety claim. Store completed evidence in the release/change record. Do not commit secrets, raw provider payloads, database dumps, coverage files, or scanner caches.

## Release identity and environment

Record:

- Git commit and whether the checkout contains tracked changes;
- date, operator, OS/kernel, CPU model/count, memory, Go/Node/pnpm/Python/Docker versions;
- container image digest, configuration revision, migration version, provider/region/model set;
- load generator location, concurrency, duration, request mix, streaming ratio, prompt/output sizes, and provider latency conditions.

## Automated checks

From a clean checkout, run:

```bash
cd llm_gateway
test -z "$(gofmt -l .)"
go vet ./...
go test -short -race ./...
make test-integration-all
make test-migrations
go test -tags=integration -count=1 -run 'Test(OpenAI|VertexAI|Bedrock)LiveChat' ./internal/providers
make test-init-admin
make docker-build

cd ../webui/frontend
pnpm install --frozen-lockfile
pnpm run lint
pnpm run test
pnpm run build

cd ../bff
python3 -m compileall -q app
python3 -m pytest -q
```

The provider command compiles in every environment and skips calls whose documented credential/model variables are absent. A release must attach a passing, non-skipped canary from the target identity, region, and model for every enabled provider. Each canary makes one short request capped at eight output tokens; never use live providers for load or soak qualification.

Scan the tracked filesystem and built image with the organization's approved, current vulnerability database. This repository can use Trivy when installed:

```bash
trivy fs --scanners vuln,secret,misconfig --severity HIGH,CRITICAL --exit-code 1 .
trivy image --scanners vuln,secret,misconfig --severity HIGH,CRITICAL --exit-code 1 llm-gateway:latest
```

Triage every result against reachable code and the deployed image. Time-bounded exceptions require a security owner, rationale, compensating control, expiry date, and ticket.

## Load and soak profiles

The committed Go benchmark measures only the parallel in-process non-streaming response/accounting path. It excludes authentication, routing, provider/network time, PostgreSQL, Redis, S3, and TLS, so its result is useful for regression detection but must not be presented as gateway capacity.

```bash
cd llm_gateway
make test-load LOAD_DURATION=30s
make test-soak SOAK_DURATION=10m
```

For production capacity, run a separate staging profile through the real load balancer and data services. Include non-streaming and streaming traffic, realistic provider latency, at least one rate-limited key, usage/billing/log persistence, and a duration long enough to cross worker flush/reload intervals. Report achieved requests/second, p50/p95/p99 latency, errors, CPU, memory, connections, queue depth, missing usage, and cost; derive limits from the first exhausted resource with safety margin.

## Resilience and recovery checks

In staging, perform and attach evidence for every procedure in the [operations runbook](operations-runbook.md):

- PostgreSQL loss and recovery;
- Redis loss and recovery, including the rate-limit error and budget fail-open policy;
- S3 loss/recovery with Redis buffer drain;
- each enabled provider loss/recovery without automatic fallback;
- a rolling restart with an active stream and queued billing/usage/log records;
- PostgreSQL backup restored into an isolated database;
- billing reconciliation for at least one non-streaming request, one complete stream, and one interrupted/unknown-usage stream;
- secret rotation rehearsal for metrics, JWT/BFF signing, provider credentials, and the maintenance-only encryption-key procedure;
- audit-log lifecycle expiry and deletion.

## Security and privacy review

Confirm that:

- public ingress exposes only intended UI/API routes; PostgreSQL, Redis, S3 administration, BFF, and metrics stay private;
- gateway/BFF secrets meet validation requirements and come from a secret manager/workload identity;
- TLS is enforced at the trusted proxy and BFF production cookies are Secure, HttpOnly, and appropriately SameSite;
- admin roles are least privilege, bootstrap credentials are removed, and provider/cloud/database identities are scoped;
- the gateway/container runs non-root without unnecessary Linux capabilities or writable paths;
- audit body mode, sensitive fields, sampling, retention, region, access, backup expiry, and deletion meet policy;
- errors, metrics, logs, and support bundles contain no credentials or unapproved prompt/response content.

## Accepted release risks

These are not Critical/High release blockers, but every production deployment must explicitly accept or mitigate them.

| Risk | Current policy/rationale | Owner | Review date |
|---|---|---|---|
| Redis fails during rate limiting | Request fails with an internal error; this prevents an unbounded rate-limit bypass. | Gateway maintainer | October 13, 2026 |
| Redis fails during budget lookup | Budget check fails open to avoid turning a transient cache outage into a full provider outage; affected usage must be reconciled. | Billing owner | October 13, 2026 |
| Stream ends without terminal usage | Store unknown accounting, increment the missing-usage metric, and reconcile manually; automatic provider reconciliation is pending. | Billing owner | October 13, 2026 |
| Provider outage | No fallback/circuit breaker; operators disable or reroute models and clients receive bounded errors. | Gateway maintainer | October 13, 2026 |
| Model/pricing catalog changes | Synchronization/versioned rollback is manual; preserve the catalog revision with release evidence. | Billing owner | October 13, 2026 |
| Kubernetes deployment | Only an init-admin job example exists; the operator owns hardened workload/service/ingress manifests. | Platform owner | October 13, 2026 |

Replace role owners with named people in the deployment change record. Expired risks block a release until re-reviewed.

## Sign-off

A release passes only when all applicable automated checks pass, all enabled providers have non-skipped canaries, resilience and restore exercises have current evidence, no unresolved Critical/High finding remains, and accepted Medium/Low risks have named owners and unexpired review dates. Otherwise label it a development or release-candidate build and document the missing evidence.

## Local qualification evidence — July 13, 2026

Revision under qualification was tested on WSL2 Linux with an Intel Core i5-9300H (8 logical CPUs), 15 GiB RAM, Go 1.26.5, Node 24.11.1, pnpm 10.24.0, and Python 3.13.5.

- Go formatting/vet and the full short suite passed; the race suite exposed and drove a Go 1.26 no-body response compatibility fix.
- Docker-backed PostgreSQL, Redis, encrypted MinIO/S3, admin API, and migration down/up suites passed with clean teardown.
- Frontend lint, 3 component tests, production build, BFF compilation, and 20 BFF tests passed.
- The production image built with Go 1.26.5/Alpine 3.23 and its Trivy High/Critical scan reported zero findings. Focused Go-module and pnpm-lock scans also reported zero.
- The five-second in-process parallel regression profile completed 1,251,073 operations at 4,625 ns/op, 3,263 B/op, and 50 allocations/op. It is not a full-stack capacity result.
- Vertex AI and Bedrock live tests compiled but skipped because their test project/model/credentials were absent.

This is partial release-candidate evidence, not sign-off. Live provider canaries, the 10-minute soak, staging dependency loss/recovery, rolling restart with active streams/queued records, isolated backup restore, and provider billing reconciliation remain required.
