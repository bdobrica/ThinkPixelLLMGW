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
# Provider API Keys
OPENAI_API_KEY=sk-proj-xxxxxxxxxxxxxxxxxxxxx

# Optional: Override defaults
GATEWAY_HTTP_PORT=8080
ENCRYPTION_KEY=generate-this-with-the-script
JWT_SECRET=your-secure-jwt-secret
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

## Development vs Production

### Development (.env)
```env
OPENAI_API_KEY=sk-dev-key
ENCRYPTION_KEY=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
JWT_SECRET=dev-secret
```

### Production
- Use a secrets management system (AWS Secrets Manager, HashiCorp Vault, etc.)
- Generate strong encryption keys
- Rotate credentials regularly
- Enable audit logging
- Use least-privilege API keys

## Additional Resources

- [OpenAI API Keys](https://platform.openai.com/api-keys)
- [Anthropic Console](https://console.anthropic.com/)
- [Google Cloud Setup](https://cloud.google.com/vertex-ai/docs/start/cloud-environment)
- [AWS Bedrock Setup](https://aws.amazon.com/bedrock/)
