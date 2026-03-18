# Environment Variables Setup

This guide explains how to configure environment variables for ThinkPixelLLMGW.

## Quick Start

1. **Copy the example file:**
   ```bash
   cp .env.example .env
   ```

2. **Edit `.env` and add your API keys:**
   ```bash
   nano .env  # or use your preferred editor
   ```

3. **Add your OpenAI API key:**
   ```env
   OPENAI_API_KEY=sk-your-actual-openai-api-key-here
   ```

4. **Start the services:**
   ```bash
   docker-compose up --build
   ```

## Required Environment Variables

### Database & Core Services

- **`DATABASE_URL`** - PostgreSQL connection string
  - Example: `postgres://postgres:password@localhost:5432/llmgateway?sslmode=disable`
  - Required for all operations

- **`REDIS_ADDRESS`** - Redis server address
  - Example: `localhost:6379`
  - Required for rate limiting, billing cache, and log buffering

### Provider API Keys

The gateway needs at least one provider API key to function:

- **`OPENAI_API_KEY`** - Your OpenAI API key
  - Get it from: https://platform.openai.com/api-keys
  - Format: `sk-...`

### Optional Provider Keys

If you plan to use additional providers:

- **`ANTHROPIC_API_KEY`** - For Claude models
  - Get it from: https://console.anthropic.com/
  - Format: `sk-ant-...`

- **Google Vertex AI** - For Gemini models
  - `GOOGLE_APPLICATION_CREDENTIALS` - Path to service account JSON
  - `GOOGLE_PROJECT_ID` - Your GCP project ID
  - `GOOGLE_REGION` - Region (e.g., `us-central1`)

- **AWS Bedrock** - For AWS hosted models
  - `AWS_ACCESS_KEY_ID` - AWS access key
  - `AWS_SECRET_ACCESS_KEY` - AWS secret key
  - `AWS_REGION` - AWS region (e.g., `us-east-1`)

### S3 Logging Configuration (Optional)

Enable background worker to drain logs from Redis to S3:

- **`LOGGING_SINK_ENABLED`** - Enable S3 logging (`true` or `false`)
  - Default: `false`
  - Set to `true` to enable automatic log archival

- **`LOGGING_SINK_S3_BUCKET`** - S3 bucket name for logs
  - Example: `my-llm-gateway-logs`
  - Must be created before enabling

- **`LOGGING_SINK_S3_REGION`** - AWS region for S3 bucket
  - Example: `us-east-1`
  - Default: `us-east-1`

- **`LOGGING_SINK_S3_PREFIX`** - Prefix for log files in S3
  - Example: `logs/` or `production/logs/`
  - Default: `logs/`

- **`LOGGING_SINK_FLUSH_SIZE`** - Number of records before flushing to S3
  - Example: `1000`
  - Default: `1000`
  - Lower values = more frequent uploads, higher S3 costs
  - Higher values = less frequent uploads, more memory usage

- **`LOGGING_SINK_FLUSH_INTERVAL`** - Time duration before flushing to S3
  - Example: `5m`, `10m`, `1h`
  - Default: `5m`
  - Uses Go duration format: `s` (seconds), `m` (minutes), `h` (hours)

- **`POD_NAME`** - Pod/instance identifier for multi-instance deployments
  - Example: `gateway-0`, `gateway-1`
  - Default: `gateway-0`
  - Used in S3 file naming to prevent conflicts
  - In Kubernetes, set from `metadata.name` via fieldRef

## How It Works

1. When the gateway starts, it reads environment variables from the `.env` file
2. If provider API keys are found (e.g., `OPENAI_API_KEY`), they are:
   - Encrypted using the `ENCRYPTION_KEY`
   - Stored securely in the database
   - Associated with the corresponding provider

3. The provider registry loads these credentials and uses them for API calls

## Security Notes

⚠️ **Important Security Considerations:**

- **Never commit `.env` to version control** - It's already in `.gitignore`
- **Change default encryption key in production** - Set `ENCRYPTION_KEY` to a secure random value
- **Use secure key generation:**
  ```bash
  ./llm_gateway/scripts/generate-encryption-key.sh
  ```
- **Rotate API keys regularly**
- **Use environment-specific `.env` files** for different deployments

## Configuration Priority

Environment variables can be set in multiple ways with this priority (highest to lowest):

1. Docker Compose `environment` section (overrides)
2. `.env` file (loaded by docker-compose)
3. Default values in `docker-compose.yaml`

## Example .env File

```env
# Database & Redis (Required)
DATABASE_URL=postgres://postgres:password@localhost:5432/llmgateway?sslmode=disable
REDIS_ADDRESS=localhost:6379

# Provider API Keys
OPENAI_API_KEY=sk-proj-xxxxxxxxxxxxxxxxxxxxx

# Security
ENCRYPTION_KEY=generate-this-with-the-script
JWT_SECRET=your-secure-jwt-secret

# S3 Logging (Optional)
LOGGING_SINK_ENABLED=true
LOGGING_SINK_S3_BUCKET=my-llm-logs
LOGGING_SINK_S3_REGION=us-east-1
LOGGING_SINK_S3_PREFIX=logs/
LOGGING_SINK_FLUSH_SIZE=1000
LOGGING_SINK_FLUSH_INTERVAL=5m
POD_NAME=gateway-0

# Optional: Override defaults
GATEWAY_HTTP_PORT=8080
```

## Troubleshooting

### Error: "api_key is required for OpenAI provider"

**Cause:** The `OPENAI_API_KEY` is not set or is empty.

**Solution:**
1. Ensure `.env` file exists in the project root
2. Verify `OPENAI_API_KEY=sk-...` is set correctly
3. Restart the services: `docker-compose down && docker-compose up`

### API Key Not Being Used

**Symptoms:** Provider credentials aren't being updated.

**Solutions:**
1. Check that the provider exists in the database (seeded by migrations)
2. Verify the encryption key is valid (64 hex characters)
3. Check container logs: `docker-compose logs gateway`

### Changes Not Reflected

After updating `.env`:
```bash
docker-compose down
docker-compose up --build
```

## S3 Logging Features

When S3 logging is enabled, the gateway:

1. **Buffers logs in Redis**: All request/response logs are buffered in Redis for fast writes
2. **Background worker**: Drains Redis buffer to S3 periodically
3. **Gzip compression**: Reduces storage costs by ~80%
4. **Structured file naming**: `logs/YYYY/MM/DD/pod-timestamp-nano.jsonl.gz`
5. **JSON Lines format**: One JSON object per line for easy parsing
6. **Graceful shutdown**: Flushes remaining logs before exit

**Storage costs**: With 1000 requests/day at ~2KB/request:
- Uncompressed: ~60MB/month = ~$0.0014/month in S3 Standard
- Gzip compressed: ~12MB/month = ~$0.0003/month in S3 Standard

**IAM permissions needed**:
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "s3:PutObject",
        "s3:PutObjectAcl"
      ],
      "Resource": "arn:aws:s3:::my-llm-logs/logs/*"
    }
  ]
}
```

## Development vs Production

### Development (.env)
```env
DATABASE_URL=postgres://postgres:password@localhost:5432/llmgateway?sslmode=disable
REDIS_ADDRESS=localhost:6379
OPENAI_API_KEY=sk-dev-key
ENCRYPTION_KEY=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
JWT_SECRET=dev-secret

# Use MinIO for local S3 testing
LOGGING_SINK_ENABLED=true
LOGGING_SINK_S3_BUCKET=test-logs
LOGGING_SINK_S3_REGION=us-east-1
AWS_ENDPOINT_URL_S3=http://localhost:9000
AWS_ACCESS_KEY_ID=minioadmin
AWS_SECRET_ACCESS_KEY=minioadmin
```

### Production
- Use a secrets management system (AWS Secrets Manager, HashiCorp Vault, etc.)
- Generate strong encryption keys
- Rotate credentials regularly
- Enable S3 logging for audit compliance
- Use least-privilege API keys and IAM roles
- Configure S3 lifecycle policies (e.g., move to Glacier after 90 days)
- Set up S3 bucket encryption and versioning
- Monitor S3 upload metrics in CloudWatch

## Additional Resources

- [OpenAI API Keys](https://platform.openai.com/api-keys)
- [Anthropic Console](https://console.anthropic.com/)
- [Google Cloud Setup](https://cloud.google.com/vertex-ai/docs/start/cloud-environment)
- [AWS Bedrock Setup](https://aws.amazon.com/bedrock/)
