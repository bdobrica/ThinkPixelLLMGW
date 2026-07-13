# Environment setup

This page is the practical `.env` workflow for the repository’s Docker Compose stack. The authoritative variable reference is [Environment variables](env-variables.md).

## Create the file

From the repository root:

```bash
cp .env.example .env
./llm_gateway/scripts/generate-encryption-key.sh
```

Edit `.env` and uncomment or add:

```dotenv
OPENAI_API_KEY=sk-...
JWT_SECRET=generate-at-least-32-random-characters
METRICS_AUTH_TOKEN=generate-a-different-32-character-token
ENCRYPTION_KEY=paste-the-generated-64-hex-character-value
```

`OPENAI_API_KEY` is needed for real requests through the seeded OpenAI provider, but not merely to start the gateway. The three security values are required at startup. Do not use the placeholder text literally.

Compose reads `.env` for interpolation and passes configured values into the gateway container. A gateway process started directly with `go run` does not parse `.env`; export its variables in the shell or use another environment loader.

## Start and verify

```bash
docker compose up -d --build
docker compose ps
curl --fail http://localhost:8080/ready
```

If interpolation reports a missing security value, confirm that it is uncommented in the repository-root `.env`. If the gateway exits, inspect:

```bash
docker compose logs gateway
```

Common validation failures include a JWT or metrics token shorter than 32 characters, a non-hexadecimal encryption key, or an enabled logging sink without a bucket.

## Provider credentials

Runtime overrides are applied only to enabled provider rows with the expected name:

- `OPENAI_API_KEY` → `openai`
- `VERTEX_AI_ACCESS_TOKEN` or `VERTEX_AI_SERVICE_ACCOUNT_JSON` → `vertexai`
- AWS SDK environment/workload credentials → `bedrock`

Provider credentials stored through `/admin/providers` are encrypted. Runtime overrides are never persisted. Vertex project/location and Bedrock region/model settings are provider configuration, not global gateway environment variables. See [Provider configuration](providers.md).

The gateway does not currently register a standalone Anthropic provider. Use OpenAI, Vertex AI, or Bedrock; Bedrock can route supported Anthropic model IDs.

## Local MinIO logging

Compose already configures the gateway container for its MinIO service and creates the `llm-logs` bucket. To disable archival while keeping local rotating audit files:

```dotenv
LOGGING_SINK_ENABLED=false
```

To run the gateway on the host with MinIO enabled, set:

```dotenv
LOGGING_SINK_ENABLED=true
LOGGING_SINK_S3_BUCKET=llm-logs
LOGGING_SINK_S3_REGION=us-east-1
AWS_ENDPOINT_URL_S3=http://localhost:9000
AWS_ACCESS_KEY_ID=minioadmin
AWS_SECRET_ACCESS_KEY=minioadmin
```

The MinIO credentials and static test KMS key in Compose are development-only. Production S3 requires a least-privilege workload identity, bucket encryption/lifecycle policy, private networking where applicable, and tested deletion/retention procedures.

## Rotation

Provider, JWT, metrics, BFF, and encryption secrets have different rotation effects. Follow the coordinated procedures in the [Operations runbook](operations-runbook.md), especially before changing `ENCRYPTION_KEY`.
