# Testing the LLM Gateway

This guide shows how to test the gateway end-to-end.

## Prerequisites

1. **PostgreSQL Database** running with migrations applied
2. **Redis** running for rate limiting and billing
3. **OpenAI API Key** (or other provider credentials)
4. **Environment variables** configured

## Quick Start

### 1. Setup Database

```bash
# Apply migrations
cd llm-gateway
sqlx database create
sqlx migrate run
```

### 2. Configure Environment

Create a `.env` file in `llm-gateway/` directory:

```bash
# Database
DATABASE_URL=postgres://postgres:password@localhost:5432/llmgateway?sslmode=disable

# Redis
REDIS_ADDRESS=localhost:6379
REDIS_PASSWORD=
REDIS_DB=0

# Server
GATEWAY_HTTP_PORT=8080

# Provider settings
PROVIDER_RELOAD_INTERVAL=5m
PROVIDER_REQUEST_TIMEOUT=60s

# Cache settings
CACHE_API_KEY_SIZE=1000
CACHE_API_KEY_TTL=5m
CACHE_MODEL_SIZE=500
CACHE_MODEL_TTL=15m
```

### 3. Setup Test Data

```sql
-- Connect to PostgreSQL
psql postgres://postgres:password@localhost:5432/llmgateway

-- Create an OpenAI provider
INSERT INTO providers (id, name, display_name, provider_type, encrypted_credentials, config, enabled)
VALUES (
    gen_random_uuid(),
    'openai-main',
    'OpenAI',
    'openai',
    '{"api_key": "sk-proj-YOUR_OPENAI_API_KEY_HERE"}',  -- Will be encrypted by app
    '{"base_url": "https://api.openai.com/v1"}',
    true
);

-- Create a test API key
-- Key will be: "test-key-12345"
-- Hash: SHA256("test-key-12345")
INSERT INTO api_keys (id, name, key_hash, enabled, rate_limit, monthly_budget)
VALUES (
    gen_random_uuid(),
    'Test Key',
    encode(sha256('test-key-12345'::bytea), 'hex'),
    true,
    100,  -- 100 requests per minute
    10.0  -- $10 monthly budget
);

-- Add GPT-4 model (assuming you have the BerriAI model catalog synced)
-- If not, you can add it manually:
INSERT INTO models (id, model_name, litellm_provider, input_cost_per_token, output_cost_per_token, mode)
VALUES (
    gen_random_uuid(),
    'gpt-4',
    'openai',
    0.00003,  -- $0.03 per 1K input tokens
    0.00006,  -- $0.06 per 1K output tokens
    'chat'
);

-- Create a model alias (optional)
INSERT INTO model_aliases (id, alias, target_model_id, provider_id, enabled)
VALUES (
    gen_random_uuid(),
    'my-gpt4',
    (SELECT id FROM models WHERE model_name = 'gpt-4'),
    (SELECT id FROM providers WHERE name = 'openai-main'),
    true
);
```

### 4. Start the Gateway

```bash
cd llm-gateway
go run cmd/gateway/main.go
```

You should see:
```
LLM Gateway listening on :8080
```

## Testing with cURL

### Basic Chat Completion

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test-key-12345" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {"role": "user", "content": "Say hello!"}
    ],
    "temperature": 0.7
  }'
```

Expected response:
```json
{
  "id": "chatcmpl-...",
  "object": "chat.completion",
  "created": 1234567890,
  "model": "gpt-4",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Hello! How can I assist you today?"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 10,
    "completion_tokens": 9,
    "total_tokens": 19
  }
}
```

### Streaming Chat Completion

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test-key-12345" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {"role": "user", "content": "Count to 5"}
    ],
    "stream": true
  }'
```

Expected response (Server-Sent Events):
```
data: {"id":"chatcmpl-...","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}

data: {"id":"chatcmpl-...","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"1"},"finish_reason":null}]}

data: {"id":"chatcmpl-...","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"content":", "},"finish_reason":null}]}

...

data: [DONE]
```

### Using Model Alias

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test-key-12345" \
  -d '{
    "model": "my-gpt4",
    "messages": [
      {"role": "user", "content": "Hello!"}
    ]
  }'
```

### Health Check

```bash
curl http://localhost:8080/health
```

Expected response:
```
OK
```

## Error Scenarios

### Invalid API Key

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer invalid-key" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

Expected response (401):
```json
{
  "error": {
    "message": "invalid API key",
    "type": "invalid_request_error",
    "code": 401
  }
}
```

### Missing Authorization Header

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

Expected response (401):
```json
{
  "error": {
    "message": "missing or invalid Authorization header",
    "type": "invalid_request_error",
    "code": 401
  }
}
```

### Unknown Model

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type": application/json" \
  -H "Authorization: Bearer test-key-12345" \
  -d '{
    "model": "gpt-99",
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

Expected response (400):
```json
{
  "error": {
    "message": "unknown model: gpt-99",
    "type": "invalid_request_error",
    "code": 400
  }
}
```

### Rate Limit Exceeded

Make 101+ requests within 60 seconds:

```bash
for i in {1..101}; do
  curl -X POST http://localhost:8080/v1/chat/completions \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer test-key-12345" \
    -d '{
      "model": "gpt-4",
      "messages": [{"role": "user", "content": "Test"}],
      "max_tokens": 1
    }' &
done
```

Expected response (429) after 100 requests:
```json
{
  "error": {
    "message": "rate limit exceeded",
    "type": "invalid_request_error",
    "code": 429
  }
}
```

### Budget Exceeded

After spending $10 in a month:

```json
{
  "error": {
    "message": "monthly budget exceeded",
    "type": "invalid_request_error",
    "code": 402
  }
}
```

## Monitoring

### Check Rate Limit Status (Redis)

```bash
redis-cli
> ZCARD rate_limit:<api-key-id>
> ZRANGE rate_limit:<api-key-id> 0 -1 WITHSCORES
```

### Check Billing Status (Redis)

```bash
redis-cli
> GET cost:<api-key-id>:2025:01
```

### Check Log Buffer (Redis)

```bash
redis-cli
> LLEN gateway:logs
> LRANGE gateway:logs 0 10
```

### Check Database

```sql
-- Check API key usage
SELECT 
    ak.name,
    COUNT(ur.*) as total_requests,
    SUM(ur.total_cost_usd) as total_cost,
    MAX(ur.created_at) as last_request
FROM api_keys ak
LEFT JOIN usage_records ur ON ur.api_key_id = ak.id
GROUP BY ak.id, ak.name;

-- Check monthly summaries
SELECT 
    ak.name,
    mus.year,
    mus.month,
    mus.total_requests,
    mus.total_cost_usd
FROM monthly_usage_summary mus
JOIN api_keys ak ON ak.id = mus.api_key_id
ORDER BY mus.year DESC, mus.month DESC;

-- Check provider status
SELECT 
    name,
    display_name,
    provider_type,
    enabled,
    created_at
FROM providers;
```

## Using with OpenAI SDK

### Python

```python
from openai import OpenAI

# Point to your gateway
client = OpenAI(
    api_key="test-key-12345",
    base_url="http://localhost:8080/v1"
)

# Use as normal
response = client.chat.completions.create(
    model="gpt-4",
    messages=[
        {"role": "user", "content": "Hello!"}
    ]
)

print(response.choices[0].message.content)
```

### Node.js

```javascript
import OpenAI from 'openai';

const client = new OpenAI({
  apiKey: 'test-key-12345',
  baseURL: 'http://localhost:8080/v1'
});

const response = await client.chat.completions.create({
  model: 'gpt-4',
  messages: [
    { role: 'user', content: 'Hello!' }
  ]
});

console.log(response.choices[0].message.content);
```

## Load Testing

### Using Apache Bench

```bash
# 1000 requests, 10 concurrent
ab -n 1000 -c 10 \
   -H "Authorization: Bearer test-key-12345" \
   -H "Content-Type: application/json" \
   -p payload.json \
   http://localhost:8080/v1/chat/completions
```

### payload.json

```json
{
  "model": "gpt-4",
  "messages": [
    {"role": "user", "content": "Test"}
  ],
  "max_tokens": 1
}
```

## Troubleshooting

### Gateway Won't Start

**Error**: `Failed to initialize database`

**Solution**: Check DATABASE_URL is correct and PostgreSQL is running

```bash
psql $DATABASE_URL -c "SELECT 1"
```

**Error**: `Failed to initialize Redis`

**Solution**: Check Redis is running

```bash
redis-cli ping
```

### Requests Failing

**Error**: `unknown model: gpt-4`

**Solution**: Ensure model exists in database and provider mapping is correct

```sql
SELECT model_name, litellm_provider FROM models WHERE model_name = 'gpt-4';
```

**Error**: `provider error`

**Solution**: Check provider credentials are correct

```sql
SELECT name, provider_type, enabled FROM providers;
```

Update credentials if needed (app will decrypt/re-encrypt):

```sql
UPDATE providers 
SET encrypted_credentials = '{"api_key": "sk-proj-YOUR_NEW_KEY"}'
WHERE name = 'openai-main';
```

### High Latency

1. **Check Redis**: `redis-cli --latency`
2. **Check Database**: `SELECT pg_stat_statements FROM pg_stat_activity;`
3. **Check Provider**: Time actual OpenAI API calls
4. **Check Logs**: Look for slow queries or timeouts

### Memory Issues

1. **Check cache sizes**: Reduce CACHE_API_KEY_SIZE and CACHE_MODEL_SIZE
2. **Check Redis memory**: `redis-cli info memory`
3. **Check connection pools**: Reduce DB_MAX_OPEN_CONNS

## Next Steps

1. **Add More Providers**: Insert Vertex AI or Bedrock providers
2. **Add More Models**: Import BerriAI model catalog
3. **Create More API Keys**: For different projects/teams
4. **Set up Monitoring**: Add Prometheus metrics
5. **Enable Logging**: Configure S3 writer for log persistence
6. **Add Admin API**: Manage keys and providers via API
