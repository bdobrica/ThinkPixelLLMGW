# Provider configuration

Provider records are managed through `/admin/providers`. Credentials are encrypted at rest; configuration fields are non-secret JSON. A model routes to a provider when its catalog `provider_id` matches that provider type.

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

Never place credential JSON in `config`; only values in `credentials` receive field-level encryption.
