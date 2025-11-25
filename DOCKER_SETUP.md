# Docker Setup Guide

This guide explains how to run the LLM Gateway using Docker and Docker Compose.

## Quick Start

### 1. Prerequisites

- Docker 20.10+ ([install](https://docs.docker.com/get-docker/))
- Docker Compose 2.0+ (included with Docker Desktop)

### 2. Start All Services

```bash
# From the repository root
docker-compose up -d
```

This will start:
- **PostgreSQL** (port 5432) - Database
- **Redis** (port 6379) - Rate limiting & billing cache
- **MinIO** (ports 9000, 9001) - S3-compatible storage
- **LLM Gateway** (port 8080) - Main application

### 3. Check Service Health

```bash
# View all containers
docker-compose ps

# View logs
docker-compose logs -f gateway

# Check specific service
docker-compose logs postgres
docker-compose logs redis
```

### 4. Apply Database Migrations

The migrations should run automatically on first startup. To verify:

```bash
# Connect to PostgreSQL
docker-compose exec postgres psql -U llmgateway -d llmgateway

# Inside psql, check tables
\dt

# You should see:
# - providers
# - models
# - model_aliases
# - api_keys
# - key_metadata
# - usage_records
# - monthly_usage_summary
```

If migrations didn't run, apply them manually:

```bash
# Copy migration files into container (if needed)
docker-compose exec postgres psql -U llmgateway -d llmgateway -f /docker-entrypoint-initdb.d/20250123000001_initial_schema.up.sql
```

### 5. Seed Test Data

```bash
# Connect to database
docker-compose exec postgres psql -U llmgateway -d llmgateway

# Run seed SQL (paste the following)
```

```sql
-- Insert OpenAI provider
INSERT INTO providers (id, name, display_name, provider_type, encrypted_credentials, config, enabled)
VALUES (
    gen_random_uuid(),
    'openai-main',
    'OpenAI',
    'openai',
    '{"api_key": "sk-proj-YOUR_OPENAI_API_KEY_HERE"}',
    '{"base_url": "https://api.openai.com/v1"}',
    true
);

-- Create test API key (key: test-key-12345)
INSERT INTO api_keys (id, name, key_hash, enabled, rate_limit, monthly_budget)
VALUES (
    gen_random_uuid(),
    'Test Key',
    encode(sha256('test-key-12345'::bytea), 'hex'),
    true,
    100,  -- 100 requests per minute
    10.0  -- $10 monthly budget
);

-- Add GPT-4 model
INSERT INTO models (id, model_name, litellm_provider, input_cost_per_token, output_cost_per_token, mode)
VALUES (
    gen_random_uuid(),
    'gpt-4',
    'openai',
    0.00003,  -- $0.03 per 1K input tokens
    0.00006,  -- $0.06 per 1K output tokens
    'chat'
);

-- Verify data
SELECT name, provider_type, enabled FROM providers;
SELECT name, enabled, rate_limit, monthly_budget FROM api_keys;
SELECT model_name, litellm_provider FROM models;
```

### 6. Test the Gateway

```bash
# Non-streaming request
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer test-key-12345" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'

# Streaming request
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer test-key-12345" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Count to 5"}],
    "stream": true
  }'

# Health check
curl http://localhost:8080/health
```

## Service Details

### PostgreSQL

- **Port:** 5432
- **Database:** llmgateway
- **User:** llmgateway
- **Password:** llmgateway_dev_password

**Connect:**
```bash
docker-compose exec postgres psql -U llmgateway -d llmgateway
```

**Backup:**
```bash
docker-compose exec postgres pg_dump -U llmgateway llmgateway > backup.sql
```

**Restore:**
```bash
docker-compose exec -T postgres psql -U llmgateway -d llmgateway < backup.sql
```

### Redis

- **Port:** 6379
- **Password:** None (development only)

**Connect:**
```bash
docker-compose exec redis redis-cli
```

**Monitor:**
```bash
# Watch commands in real-time
docker-compose exec redis redis-cli MONITOR

# Check memory usage
docker-compose exec redis redis-cli INFO memory

# List all keys
docker-compose exec redis redis-cli KEYS '*'

# Check rate limit for an API key
docker-compose exec redis redis-cli ZCARD 'rate_limit:<api-key-id>'

# Check billing
docker-compose exec redis redis-cli GET 'cost:<api-key-id>:2025:11'

# Check log buffer size
docker-compose exec redis redis-cli LLEN 'gateway:logs'
```

### MinIO (S3)

- **API Port:** 9000
- **Console Port:** 9001
- **Access Key:** minioadmin
- **Secret Key:** minioadmin
- **Bucket:** llm-logs

**Access Console:**
Open http://localhost:9001 in your browser

**CLI Access:**
```bash
# Install MinIO client
# macOS: brew install minio/stable/mc
# Linux: wget https://dl.min.io/client/mc/release/linux-amd64/mc
# Windows: download from https://dl.min.io/client/mc/release/windows-amd64/mc.exe

# Configure
mc alias set local http://localhost:9000 minioadmin minioadmin

# List buckets
mc ls local

# List files in bucket
mc ls local/llm-logs

# Download logs
mc cp local/llm-logs/logs/ ./downloaded-logs/ --recursive
```

### LLM Gateway

- **Port:** 8080
- **Health:** http://localhost:8080/health
- **Metrics:** http://localhost:8080/metrics (placeholder)

**View Logs:**
```bash
# Follow logs
docker-compose logs -f gateway

# Last 100 lines
docker-compose logs --tail=100 gateway

# Search logs
docker-compose logs gateway | grep ERROR
```

**Restart:**
```bash
docker-compose restart gateway
```

**Rebuild after code changes:**
```bash
docker-compose up -d --build gateway
```

## Development Workflow

### 1. Make Code Changes

Edit files in `llm-gateway/` directory on your host machine.

### 2. Rebuild and Restart

```bash
# Stop gateway
docker-compose stop gateway

# Rebuild image
docker-compose build gateway

# Start gateway
docker-compose up -d gateway

# Or all in one command
docker-compose up -d --build gateway
```

### 3. View Logs

```bash
docker-compose logs -f gateway
```

### 4. Test Changes

```bash
curl http://localhost:8080/health
```

## Troubleshooting

### Gateway Won't Start

**Check logs:**
```bash
docker-compose logs gateway
```

**Common issues:**
- Database not ready: Wait a few seconds and check `docker-compose ps`
- Port already in use: Change `8080:8080` to `8081:8080` in docker-compose.yaml
- Migration failed: Check PostgreSQL logs and apply migrations manually

### Database Connection Failed

**Check PostgreSQL status:**
```bash
docker-compose ps postgres
docker-compose logs postgres
```

**Test connection:**
```bash
docker-compose exec postgres psql -U llmgateway -d llmgateway -c "SELECT 1;"
```

### Redis Connection Failed

**Check Redis status:**
```bash
docker-compose ps redis
docker-compose logs redis
```

**Test connection:**
```bash
docker-compose exec redis redis-cli PING
# Should return: PONG
```

### Clean Slate (Reset Everything)

**Warning:** This deletes all data!

```bash
# Stop and remove containers, networks, volumes
docker-compose down -v

# Remove images (optional)
docker-compose down -v --rmi all

# Start fresh
docker-compose up -d
```

## Environment Variables

To customize configuration, create a `.env` file in the repository root:

```bash
# Example .env file
GATEWAY_HTTP_PORT=8080
DATABASE_URL=postgres://llmgateway:llmgateway_dev_password@postgres:5432/llmgateway?sslmode=disable
REDIS_ADDRESS=redis:6379

# Add your OpenAI key here
OPENAI_API_KEY=sk-proj-your-key-here
```

Then update `docker-compose.yaml` to use environment variables:

```yaml
services:
  gateway:
    env_file:
      - .env
```

## Production Considerations

**⚠️ This setup is for DEVELOPMENT ONLY. For production:**

1. **Change Passwords:**
   - PostgreSQL password
   - Redis password (enable AUTH)
   - MinIO credentials

2. **Use Secrets:**
   - Store sensitive data in Docker secrets or environment variables
   - Never commit `.env` files

3. **Enable TLS:**
   - Use HTTPS for gateway
   - Enable SSL for PostgreSQL
   - Enable TLS for Redis

4. **Resource Limits:**
   - Set memory and CPU limits in docker-compose.yaml
   - Configure connection pools appropriately

5. **Persistent Storage:**
   - Use named volumes or mount points for data
   - Regular backups of PostgreSQL

6. **Monitoring:**
   - Add Prometheus and Grafana
   - Configure alerts
   - Enable structured logging

7. **Networking:**
   - Use reverse proxy (nginx, traefik)
   - Isolate services with networks
   - Configure firewall rules

## Additional Commands

### Scale Services

```bash
# Run multiple gateway instances (requires load balancer)
docker-compose up -d --scale gateway=3
```

### Export Logs

```bash
# Export all logs
docker-compose logs > all-logs.txt

# Export specific service
docker-compose logs gateway > gateway-logs.txt
```

### Database Queries

```bash
# Check API key usage
docker-compose exec postgres psql -U llmgateway -d llmgateway -c "
SELECT 
    ak.name,
    COUNT(ur.*) as total_requests,
    SUM(ur.total_cost_usd) as total_cost,
    MAX(ur.created_at) as last_request
FROM api_keys ak
LEFT JOIN usage_records ur ON ur.api_key_id = ak.id
GROUP BY ak.id, ak.name;
"

# Check monthly summaries
docker-compose exec postgres psql -U llmgateway -d llmgateway -c "
SELECT 
    ak.name,
    mus.year,
    mus.month,
    mus.total_requests,
    mus.total_cost_usd
FROM monthly_usage_summary mus
JOIN api_keys ak ON ak.id = mus.api_key_id
ORDER BY mus.year DESC, mus.month DESC;
"
```

## Next Steps

1. **Configure Providers:** Add your OpenAI, Vertex AI, or Bedrock credentials
2. **Create API Keys:** Generate production API keys with proper budgets
3. **Set Up Monitoring:** Add Prometheus metrics endpoint
4. **Enable S3 Upload:** Implement S3 writer to drain Redis log buffer
5. **Add Admin API:** Implement JWT-protected admin endpoints

For complete testing instructions, see [TESTING_GUIDE.md](TESTING_GUIDE.md).
