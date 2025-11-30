# End-to-End Testing

This directory contains end-to-end tests for the LLM Gateway.

## Overview

The e2e test (`test_e2e.py`) validates the complete LLM Gateway stack by:

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
pip install openai

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

### Option 3: Direct Python Execution

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

► Stopping docker-compose services...
✓ Docker-compose services stopped and cleaned up

============================================================
Test Results
============================================================

Tests run: 3
Tests passed: 3
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
