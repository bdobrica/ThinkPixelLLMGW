# Testing Guide

This directory contains integration and end-to-end tests for the LLM Gateway.

## Test Suites

### 1. Bootstrap Admin Integration Test (`test_init_admin.py`)

Tests the admin bootstrap and authentication workflow:

- **Purpose**: Validates that init-admin tool creates working admin users
- **Coverage**:
  - Bootstrap admin creation via init-admin
  - Admin login and JWT token generation
  - API key creation with admin credentials
  - API key verification
  - init-admin idempotency
- **Runtime**: ~30-45 seconds
- **Dependencies**: PostgreSQL, Redis, Gateway only (no external APIs)

**Run with:**
```bash
cd llm_gateway
make test-init-admin
```

### 2. End-to-End Test (`test_e2e.py`)

The e2e test validates the complete LLM Gateway stack by:

1. **Starting Services**: Launches all docker-compose services (Postgres, Redis, MinIO, Gateway)
2. **Health Checks**: Waits for services to be healthy and ready
3. **API Testing**: Makes real API calls through the gateway to OpenAI
4. **Validation**: Verifies response structure, content, and error handling
5. **Cleanup**: Stops and removes all services

## Prerequisites

### Required

- Docker and docker-compose installed
- `.env` file in the repository root with `OPENAI_API_KEY` set to a valid OpenAI API key
- Python virtual environment with OpenAI SDK installed at `~/.venvs/py-openai`

### Setup Python Environment

If you haven't already set up the Python virtual environment:

```bash
# Create virtual environment
python3 -m venv ~/.venvs/py-openai

# Activate it
source ~/.venvs/py-openai/bin/activate

# Install dependencies
pip install -r tests/requirements.txt

# Deactivate
deactivate
```

## Running Tests

### Option 1: Automatic (Recommended)

Run the full e2e test suite with automatic setup and teardown:

```bash
cd llm_gateway
make test-e2e
```

This will:
- Start all docker-compose services
- Wait for the gateway to be healthy
- Run all e2e tests
- Stop and clean up all services

### Option 2: Manual Control

For debugging or development, you can control each step manually:

```bash
cd llm_gateway

# Start services
make test-e2e-setup

# Wait for services to be ready (check logs)
make test-e2e-logs

# Run the tests (services must be running)
make test-e2e-run

# When done, stop services
make test-e2e-teardown
```

### Option 3: Rate Limiting Tests Only

To test only the rate limiting functionality:

```bash
cd llm_gateway

# Make sure services are running first
make test-e2e-setup

# Run rate limiting tests
make test-rate-limit

# Or run manually
cd ..
~/.venvs/py-openai/bin/python -c "from tests.test_e2e import test_rate_limiting, test_rate_limit_headers; import sys; tests = [test_rate_limiting(), test_rate_limit_headers()]; sys.exit(0 if all(tests) else 1)"
```

### Option 4: Direct Python Execution

You can also run the test script directly:

```bash
cd /path/to/ThinkPixelLLMGW
~/.venvs/py-openai/bin/python tests/test_e2e.py
```

## Test Cases

The e2e test suite includes:

### 1. Chat Completion Test
- **Purpose**: Validates basic chat completion functionality
- **Test**: Sends a simple chat request through the gateway to OpenAI
- **Validates**: 
  - Response structure (choices, message, content)
  - Response metadata (model, usage, tokens)
  - Non-empty response content

### 2. Invalid API Key Test
- **Purpose**: Ensures authentication is working
- **Test**: Attempts to use an invalid API key
- **Validates**: Request is rejected with an error

### 3. Model Alias Test
- **Purpose**: Validates model alias resolution
- **Test**: Uses a model alias (e.g., 'gpt-4' → 'gpt-4o')
- **Validates**: Alias is correctly resolved to actual model

### 4. Redis Log Buffer Test
- **Purpose**: Validates Redis log buffering
- **Test**: Checks that logs are being buffered in Redis
- **Validates**:
  - Connection to Redis
  - Log queue existence and size
  - Log record structure in Redis

### 5. S3 Logging Pipeline Test
- **Purpose**: Validates complete S3 logging workflow
- **Test**: Makes API request, waits for flush, verifies S3 logs
- **Validates**:
  - Logs are written to Minio S3
  - Files are gzip compressed
  - JSON Lines format is correct
  - Required fields present (timestamp, request_id, provider, model, etc.)

### 6. Rate Limiting Test
- **Purpose**: Validates per-API-key rate limiting
- **Test**: Makes 65 rapid requests (exceeding 60/min limit)
- **Validates**:
  - Rate limiter blocks requests after limit reached
  - 429 Too Many Requests responses
  - Rate limit is enforced correctly

### 7. Rate Limit Headers Test
- **Purpose**: Validates rate limit response headers
- **Test**: Inspects HTTP response headers
- **Validates**:
  - X-RateLimit-Limit header present
  - X-RateLimit-Remaining header present
  - X-RateLimit-Reset header present

## Test Output

The test script provides colored, detailed output:

```
============================================================
LLM Gateway - End-to-End Test
============================================================

► Running pre-flight checks...
✓ .env file found with OPENAI_API_KEY
✓ Pre-flight checks passed

► Starting docker-compose services...
✓ Docker-compose services started

► Waiting for LLM Gateway to be healthy...
  Waiting for service at http://localhost:8080/health (timeout: 120s)...
✓ LLM Gateway is healthy and ready

Running Tests...

► Testing chat completion through gateway...
  Sending chat completion request...
  Response: Hello from LLM Gateway!
✓ Chat completion successful!
  Model: gpt-4o-2024-05-13
  Tokens: 25 total (15 prompt + 10 completion)

► Testing invalid API key handling...
  Sending request with invalid API key...
✓ Invalid API key correctly rejected
  Error: Error code: 401 - {'error': {'message': 'Invalid API key'}}

► Testing model alias resolution...
  Sending request with model alias 'gpt-4'...
✓ Model alias resolved successfully
  Requested: gpt-4 (alias)
  Actual model: gpt-4o-2024-05-13

► Testing Redis log buffering...
✓ Connected to Redis
  Redis log queue size: 3
✓ Logs are being buffered in Redis (3 pending)

► Testing S3 logging pipeline...
  Making request to generate logs...
✓ Request completed, logs should be buffered in Redis
  Waiting 10 seconds for background worker to flush to S3...
  Checking for logs in Minio S3...
✓ Found 2 log file(s) in S3!
  Latest log: logs/2024-01-15T12-30-00Z.jsonl.gz (1234 bytes)
✓ Log file is gzip compressed ✓
  Log contains 5 record(s)
✓ Log structure validated ✓
  Sample: Provider=openai, Model=gpt-4o-2024-05-13

► Stopping docker-compose services...
✓ Docker-compose services stopped and cleaned up

============================================================
Test Results
============================================================

Tests run: 5
Tests passed: 5
Tests failed: 0

✓ All tests passed!
```

## Debugging

### View Gateway Logs

```bash
make test-e2e-logs
```

Or directly:

```bash
docker logs gw-gateway
docker logs --tail 50 gw-gateway
docker logs --follow gw-gateway
```

### Check Minio S3 Console

The Minio console is available at http://localhost:9001

- Username: `minioadmin`
- Password: `minioadmin`

You can browse the `llm-logs` bucket to inspect log files.

### Check Redis Logs

```bash
docker exec -it gw-redis redis-cli

# In Redis CLI:
LLEN gateway:logs           # Check queue size
LINDEX gateway:logs 0       # Peek at first log
LRANGE gateway:logs 0 10    # Get first 10 logs
```

### Check Service Health

```bash
# Check if services are running
docker compose ps

# Check gateway health endpoint
curl http://localhost:8080/health

# Check PostgreSQL
docker exec gw-postgres pg_isready -U gateway

# Check Redis
docker exec gw-redis redis-cli ping
```

### Manual API Testing

Once services are running (`make test-e2e-setup`), you can test manually:

```bash
# Test with the example script
~/.venvs/py-openai/bin/python examples/call_chat_completion.py

# Or with curl
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer demo-key-12345" \
  -d '{
    "model": "gpt-4o",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

## Troubleshooting

### Tests Fail to Start

- **Check Docker**: Ensure Docker daemon is running
- **Check Ports**: Ensure ports 8080, 5432, 6379, 9000, 9001 are available
- **Check .env**: Verify OPENAI_API_KEY is set correctly

### Gateway Not Healthy

- **Check Logs**: `make test-e2e-logs`
- **Check Database**: Ensure migrations ran successfully
- **Wait Longer**: Gateway may need more time to initialize (default timeout: 120s)

### API Calls Fail

- **Check OpenAI Key**: Verify your OPENAI_API_KEY is valid and has credits
- **Check Seed Data**: Ensure database has seed data (demo-key-12345 should exist)
- **Check Provider Config**: Verify OpenAI provider is configured in the database

### Cleanup Issues

If services don't stop cleanly:

```bash
# Force stop and remove everything
cd /path/to/ThinkPixelLLMGW
docker compose down -v --remove-orphans

# Remove dangling volumes
docker volume prune
```

## CI/CD Integration

To integrate this test into CI/CD pipelines:

```yaml
# Example GitHub Actions workflow
- name: Run E2E Tests
  env:
    OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
  run: |
    echo "OPENAI_API_KEY=$OPENAI_API_KEY" > .env
    python3 -m venv ~/.venvs/py-openai
    source ~/.venvs/py-openai/bin/activate
    pip install openai
    cd llm_gateway
    make test-e2e
```

## Contributing

When adding new e2e tests:

1. Add test functions to `test_e2e.py` following the pattern:
   ```python
   def test_new_feature() -> bool:
       """Test description."""
       print_step("Testing new feature...")
       try:
           # Test implementation
           print_success("Test passed")
           return True
       except Exception as e:
           print_error(f"Test failed: {e}")
           return False
   ```

2. Add the test to the main execution in `main()`:
   ```python
   tests_run += 1
   if test_new_feature():
       tests_passed += 1
   ```

3. Document the test case in this README
4. Test locally before submitting PR

