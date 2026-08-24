# Provider configuration

Provider records are managed through `/admin/providers`. Credentials are encrypted at rest; configuration fields are non-secret JSON. A model routes to a provider when its catalog `provider_id` matches that provider type.

The provider-specific Responses API transport and tool decisions are tracked separately in the [Responses compatibility matrix](responses-api.md). Those entries describe planned native or translated paths; none enables the `/v1/responses` route by itself.

## OpenAI

- Type: `openai`
- Credential: `api_key`
- Optional configuration: `base_url`, `request_timeout`
- Environment override for a provider row named `openai`: `OPENAI_API_KEY`

## Google Vertex AI

- Type: `vertexai`
- Required configuration: `project_id`
- Optional configuration: `location` (default `us-central1`), `request_timeout`, `base_url`
- Authentication, in priority order: encrypted `access_token`; encrypted `service_account_json` (or `credentials_json`); Google Application Default Credentials (ADC)
- Runtime-only overrides for a provider row named `vertexai`: `VERTEX_AI_ACCESS_TOKEN` and `VERTEX_AI_SERVICE_ACCOUNT_JSON`

For production, prefer ADC/workload identity or a service account with only `aiplatform.endpoints.predict` and the permissions needed to list publisher models during credential validation. `GOOGLE_APPLICATION_CREDENTIALS` may point to a service-account file for ADC. Static `VERTEX_AI_ACCESS_TOKEN` values are useful for short-lived development sessions but are not refreshed.

Example request body for the admin API:

```json
{
  "name": "vertexai",
  "display_name": "Vertex AI",
  "type": "vertexai",
  "credentials": {
    "service_account_json": "{...}"
  },
  "config": {
    "project_id": "my-project",
    "location": "us-central1",
    "request_timeout": "60s"
  },
  "enabled": true
}
```

The adapter calls Vertex AI's OpenAI-compatible Chat Completions endpoint, so text/multimodal messages, tools, structured responses, non-streaming usage, and SSE streaming follow the subset supported by the selected Vertex model. Provider HTTP status codes and response bodies are preserved. Credential validation acquires an OAuth token and performs a non-generation publisher-model list request.

## AWS Bedrock

- Type: `bedrock`
- Optional configuration: `region` (default `us-east-1`), `request_timeout`, `base_url`, `validation_model`
- Authentication: encrypted `access_key_id`, `secret_access_key`, and optional `session_token`; or the AWS SDK default credential chain (environment, shared configuration, web identity, ECS/EC2 IAM role, and other SDK-supported sources)

Example request body for explicit credentials:

```json
{
  "name": "bedrock",
  "display_name": "AWS Bedrock",
  "type": "bedrock",
  "credentials": {
    "access_key_id": "AKIA...",
    "secret_access_key": "...",
    "session_token": "..."
  },
  "config": {
    "region": "us-east-1",
    "validation_model": "amazon.nova-lite-v1:0",
    "request_timeout": "60s"
  },
  "enabled": true
}
```

For production, leave the credential object empty and use a workload IAM role with `bedrock:InvokeModel` and `bedrock:InvokeModelWithResponseStream` on only the required model or inference-profile resources. Standard `AWS_REGION`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`, web-identity, shared-config, and container/instance-role mechanisms are resolved by AWS SDK v2. The optional `validation_model` makes credential validation call the non-generation `CountTokens` operation, proving Bedrock permission and model access; without it, validation confirms only that the SDK can retrieve credentials.

The adapter translates OpenAI system/developer, user, and assistant text messages plus `max_tokens`/`max_completion_tokens`, `temperature`, `top_p`, and `stop` into Bedrock Converse. It translates Bedrock output, stop reasons, errors, token/cache usage, and ConverseStream events back into the gateway's OpenAI-compatible JSON/SSE contract. Caller cancellation and the provider request timeout cover both initial calls and the full stream. The current Bedrock adapter rejects tool-role messages and non-text content parts instead of silently dropping them; add native tool and multimodal translation before enabling those capabilities for Bedrock catalog entries.

## Live provider qualification

The default tests use deterministic local cloud fakes and require no provider credentials. To make real, potentially billable smoke requests, set `VERTEX_TEST_PROJECT_ID` and `VERTEX_TEST_MODEL` (optionally `VERTEX_TEST_LOCATION`) with ADC configured, and/or set `BEDROCK_TEST_MODEL` (optionally `BEDROCK_TEST_REGION`) with AWS SDK credentials configured, then run:

```bash
cd llm_gateway
go test -tags=integration -run 'Test(VertexAI|Bedrock)LiveChat' ./internal/providers
```

Never place credential JSON in `config`; only values in `credentials` receive field-level encryption.
