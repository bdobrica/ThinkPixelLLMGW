# End-to-end tests

This directory contains Python tests for workflows that require a running gateway stack. Go unit and integration suites are documented separately in [docs/testing-guide.md](../docs/testing-guide.md).

## Suites

### `test_init_admin.py`

Exercises administrator bootstrap and authentication:

- creates the initial administrator with `init-admin`;
- logs in and validates the JWT flow;
- creates an additional administrator and API key;
- checks bootstrap idempotency.

It requires Docker, PostgreSQL, Redis, and the gateway, but no external provider call.

```bash
cd llm_gateway
make test-init-admin
```

### `test_e2e.py`

Exercises the complete Compose stack, including chat completion, invalid-key handling, aliases, rate limiting, Redis log buffering, and MinIO/S3 logging. Chat tests can make real OpenAI requests and therefore may incur cost.

## Setup

Prerequisites:

- Docker with Compose
- Python 3
- a root `.env` file containing valid `OPENAI_API_KEY`, `JWT_SECRET`, and `ENCRYPTION_KEY` values

Install dependencies in any active Python environment:

```bash
python3 -m venv .venv
source .venv/bin/activate
pip install -r tests/requirements.txt
```

The Makefile uses `python3` from `PATH`; it does not assume a developer-specific virtual-environment directory.

## Run

Automatic end-to-end lifecycle:

```bash
cd llm_gateway
make test-e2e
```

Manual lifecycle for debugging:

```bash
cd llm_gateway
make test-e2e-setup
make test-e2e-logs
make test-e2e-run
make test-e2e-teardown
```

Rate-limit subset against an already running stack:

```bash
cd llm_gateway
make test-rate-limit
```

Direct execution is also supported:

```bash
python3 tests/test_e2e.py
python3 tests/test_init_admin.py
```

## Failure cleanup

If an interrupted run leaves containers or volumes behind:

```bash
docker compose down -v
```

Use `docker compose ps` and `docker compose logs gateway` to diagnose service startup failures. Never place secrets or provider responses in issue reports without redaction.
