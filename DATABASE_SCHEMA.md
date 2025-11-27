# Database Schema Documentation

This document describes the PostgreSQL database schema for ThinkPixelLLMGW.

## Entity Relationship Diagram

```
┌─────────────────────┐
│     providers       │
├─────────────────────┤
│ id (PK)            │◄────┐
│ name (unique)      │     │
│ display_name       │     │
│ provider_type      │     │
│ encrypted_creds    │     │
│ config (jsonb)     │     │
│ enabled            │     │
│ created_at         │     │
│ updated_at         │     │
└─────────────────────┘     │
                            │
                            │ (provider_id FK)
┌─────────────────────┐     │
│       models        │     │
├─────────────────────┤     │
│ id (PK)            │◄────┼────┐
│ model_name (unique)│     │    │
│ litellm_provider   │     │    │
│ input_cost_per_*   │     │    │
│ output_cost_per_*  │     │    │
│ max_input_tokens   │     │    │
│ max_output_tokens  │     │    │
│ max_tokens         │     │    │
│ mode               │     │    │
│ supports_* (bool)  │     │    │
│ supported_* (arr)  │     │    │
│ metadata (jsonb)   │     │    │
│ sync_source        │     │    │
│ sync_version       │     │    │
│ last_synced_at     │     │    │
│ created_at         │     │    │
│ updated_at         │     │    │
└─────────────────────┘     │    │
         ▲                  │    │
         │                  │    │
         │ (target_model_id)│    │
         │                  │    │
┌─────────────────────┐     │    │
│   model_aliases     │     │    │
├─────────────────────┤     │    │
│ id (PK)            │◄────┐│    │
│ alias (unique)     │     ││    │
│ target_model_id FK │─────┘│    │
│ provider_id FK     │──────┼────┘
│ custom_config      │      │
│ enabled            │      │
│ created_at         │      │
│ updated_at         │      │
└─────────────────────┘      │
         ▲                   │
         │                   │
         │ (model_alias_id)  │
         │                   │
┌─────────────────────┐      │
│ model_alias_tags    │      │
├─────────────────────┤      │
│ id (PK)            │      │
│ model_alias_id FK  │──────┘
│ key                │
│ value              │
│ created_at         │
└─────────────────────┘


┌─────────────────────┐
│   admin_users       │
├─────────────────────┤
│ id (PK)            │
│ email (unique)     │
│ password_hash      │
│ roles[]            │
│ enabled            │
│ last_login_at      │
│ created_at         │
│ updated_at         │
└─────────────────────┘

┌─────────────────────┐
│   admin_tokens      │
├─────────────────────┤
│ id (PK)            │
│ service_name (uniq)│
│ token_hash (unique)│
│ roles[]            │
│ enabled            │
│ expires_at         │
│ last_used_at       │
│ created_at         │
│ updated_at         │
└─────────────────────┘

┌─────────────────────┐
│     api_keys        │
├─────────────────────┤
│ id (PK)            │◄────┐
│ name               │     │
│ key_hash (unique)  │     │
│ allowed_models[]   │     │
│ rate_limit_per_min │     │
│ monthly_budget_usd │     │
│ enabled            │     │
│ expires_at         │     │
│ created_at         │     │
│ updated_at         │     │
└─────────────────────┘     │
         ▲                  │
         │                  │
         │ (api_key_id FK)  │
         │                  │
┌─────────────────────┐     │
│   api_key_tags      │     │
├─────────────────────┤     │
│ id (PK)            │     │
│ api_key_id FK      │─────┘
│ key                │
│ value              │
│ created_at         │
└─────────────────────┘
                            │
                            │ (api_key_id FK)
┌─────────────────────┐     │
│   usage_records     │     │
├─────────────────────┤     │
│ id (PK)            │     │
│ api_key_id FK      │─────┘
│ model_id FK        │────►(models.id)
│ provider_id FK     │────►(providers.id)
│ request_id         │
│ model_name         │
│ endpoint           │
│ input_tokens       │
│ output_tokens      │
│ cached_tokens      │
│ reasoning_tokens   │
│ input_cost_usd     │
│ output_cost_usd    │
│ cache_cost_usd     │
│ total_cost_usd     │
│ response_time_ms   │
│ status_code        │
│ error_message      │
│ metadata (jsonb)   │
│ created_at         │
└─────────────────────┘
         │
         │ (aggregates to)
         ▼
┌─────────────────────┐
│monthly_usage_summary│
├─────────────────────┤
│ id (PK)            │
│ api_key_id FK      │────►(api_keys.id)
│ year               │
│ month              │
│ total_requests     │
│ total_input_tokens │
│ total_output_tokens│
│ total_cost_usd     │
│ last_request_at    │
│ created_at         │
│ updated_at         │
└─────────────────────┘
```

## Table Descriptions

### providers

Stores LLM provider configurations (OpenAI, Google VertexAI, AWS Bedrock, etc.).

**Key Features**:
- Encrypted credentials stored in `encrypted_credentials` JSONB column
- Provider-specific config in `config` JSONB for flexibility
- Can be enabled/disabled without deletion

**Example Data**:
```sql
{
    "name": "openai",
    "display_name": "OpenAI",
    "provider_type": "openai",
    "encrypted_credentials": {
        "api_key": "encrypted_blob_here"
    },
    "config": {
        "base_url": "https://api.openai.com/v1",
        "timeout_seconds": 30
    },
    "enabled": true
}
```

### models

Master catalog of all available LLM models, synced from BerriAI/LiteLLM repository.

**Key Features**:
- Comprehensive pricing data (input/output costs, cache costs, audio costs, etc.)
- Tiered pricing for different context window sizes
- Feature flags for capabilities (function calling, vision, audio, etc.)
- Full BerriAI metadata preserved in `metadata` JSONB
- Sync tracking (`sync_source`, `sync_version`, `last_synced_at`)

**Example Data**:
```sql
{
    "model_name": "gpt-5",
    "litellm_provider": "openai",
    "input_cost_per_token": 0.00000125,
    "output_cost_per_token": 0.00001,
    "max_input_tokens": 272000,
    "max_output_tokens": 128000,
    "supports_function_calling": true,
    "supports_vision": true,
    "metadata": {
        "supported_endpoints": ["/v1/chat/completions", "/v1/batch"],
        "supported_modalities": ["text", "image"]
    }
}
```

### model_aliases

User-friendly aliases for models (e.g., "gpt5" → "gpt-5", "my-custom-gpt" → "gpt-5").

**Key Features**:
- One alias maps to one model
- Optional provider override
- Custom configuration per alias
- Can be enabled/disabled

**Example Use Cases**:
- Short names: `gpt5` instead of `gpt-5`
- Version pinning: `gpt-latest` always points to newest GPT
- Custom routing: `cheap-model` points to lowest-cost option

### model_alias_tags

Flexible tagging system for model aliases (categories, use cases, custom labels).

**Why Separate Table?**:
- ✅ Add new tags without schema changes
- ✅ Efficient queries: "Find all aliases with category X"
- ✅ Multiple tags per alias
- ✅ Better organization and filtering

**Example Queries**:
```sql
-- Get all cost-effective aliases
SELECT ma.* FROM model_aliases ma
JOIN model_alias_tags mat ON ma.id = mat.model_alias_id
WHERE mat.key = 'category' AND mat.value = 'cost-effective';

-- Get all tags for an alias
SELECT key, value 
FROM model_alias_tags
WHERE model_alias_id = '77777777-7777-7777-7777-777777777777';
```

**Common Tag Keys**:
- `category`: Classification (premium, cost-effective, advanced)
- `use_case`: Purpose (general, high-volume, complex-reasoning)
- `tier`: Service tier (standard, enterprise)
- `region`: Geographic preference

### admin_users

Human accounts for management API access with email/password authentication.

**Key Features**:
- Email-based authentication
- Argon2 password hashing (secure, memory-hard algorithm)
- Role-based access control (e.g., admin, editor, viewer)
- Can be enabled/disabled without deletion
- Last login tracking for security auditing

**Security**:
```go
// Always use Argon2 for password hashing
hash := argon2.IDKey(password, salt, 1, 64*1024, 4, 32)
// Store the hash in the database
```

**Common Roles**:
- `admin`: Full access to all management API endpoints
- `editor`: Can create/update resources but not delete
- `viewer`: Read-only access to management API

**Example Data**:
```sql
{
    "email": "admin@example.com",
    "password_hash": "$argon2id$v=19$m=65536,t=1,p=4$...",
    "roles": ["admin", "viewer"],
    "enabled": true
}
```

### admin_tokens

Service accounts for management API access with token-based authentication.

**Key Features**:
- Token-based authentication for automated systems
- Argon2 token hashing for secure storage
- Role-based access control (same as admin_users)
- Optional expiration support
- Last used tracking for monitoring
- Can be enabled/disabled without deletion

**Security**:
```go
// Generate a secure random token
token := generateSecureToken() // e.g., 32 bytes random
// Hash with Argon2 before storing
hash := argon2.IDKey([]byte(token), salt, 1, 64*1024, 4, 32)
// Store hash in database, return token to user ONCE
```

**Use Cases**:
- CI/CD pipelines: Automate provider/model management
- Monitoring tools: Read-only access to usage statistics
- Integration services: Programmatic API key creation

**Example Data**:
```sql
{
    "service_name": "ci-pipeline",
    "token_hash": "$argon2id$v=19$m=65536,t=1,p=4$...",
    "roles": ["editor"],
    "enabled": true,
    "expires_at": "2026-01-01T00:00:00Z"
}
```

### api_keys

Client API keys with rate limiting and budget controls.

**Why Separate Table?**:
- ✅ Add new tags without schema changes
- ✅ Efficient queries: "Find all aliases with category X"
- ✅ Multiple tags per alias
- ✅ Better organization and filtering

**Example Queries**:
```sql
-- Get all cost-effective aliases
SELECT ma.* FROM model_aliases ma
JOIN model_alias_tags mat ON ma.id = mat.model_alias_id
WHERE mat.key = 'category' AND mat.value = 'cost-effective';

-- Get all tags for an alias
SELECT key, value 
FROM model_alias_tags
WHERE model_alias_id = '77777777-7777-7777-7777-777777777777';
```

**Common Tag Keys**:
- `category`: Classification (premium, cost-effective, advanced)
- `use_case`: Purpose (general, high-volume, complex-reasoning)
- `tier`: Service tier (standard, enterprise)
- `region`: Geographic preference

### api_keys

Client API keys with rate limiting and budget controls.

**Key Features**:
- SHA-256 hashed keys (never store plaintext)
- Per-key allowed models list
- Rate limiting (per minute)
- Monthly budget caps (USD)
- Expiration support
- Enable/disable without deletion

**Security**:
```go
// Never store the actual key
actualKey := "sk-abc123..."
hash := sha256.Sum256([]byte(actualKey))
keyHash := hex.EncodeToString(hash[:])
// Store keyHash in database
```

### api_key_tags

Flexible tagging system for API keys (environment, ownership, custom metadata).

**Why Separate Table?**:
- ✅ Add new tags without schema changes
- ✅ Efficient queries: "Find all keys with tag X"
- ✅ Multiple tags per API key
- ✅ Better reporting and analytics

**Example Queries**:
```sql
-- Get all production keys
SELECT ak.* FROM api_keys ak
JOIN api_key_tags akt ON ak.id = akt.api_key_id
WHERE akt.key = 'environment' AND akt.value = 'production';

-- Get all tags for a key
SELECT key, value 
FROM api_key_tags
WHERE api_key_id = '66666666-6666-6666-6666-666666666666';
```

**Common Tag Keys**:
- `environment`: Environment (development, staging, production)
- `team`: Team ownership (engineering, sales, support)
- `owner`: Owner email or username
- `project`: Project name
- `purpose`: Purpose or description

### usage_records

Audit log of all API requests for billing, analytics, and debugging.

**Key Features**:
- Full token usage breakdown (input, output, cached, reasoning)
- Precise cost calculation per request
- Response metadata (latency, status code, errors)
- Request correlation via `request_id`
- Flexible `metadata` JSONB for additional context

**Partitioning Strategy**:
For large-scale deployments, partition by month:
```sql
-- Partition by created_at month for efficient archiving
CREATE TABLE usage_records_2025_01 PARTITION OF usage_records
FOR VALUES FROM ('2025-01-01') TO ('2025-02-01');
```

**Cost Calculation**:
```sql
-- Example cost calculation
total_cost_usd = 
    (input_tokens * model.input_cost_per_token) +
    (output_tokens * model.output_cost_per_token) +
    (cached_tokens * model.cache_read_input_token_cost) +
    (reasoning_tokens * model.output_cost_per_reasoning_token)
```

### monthly_usage_summary

Pre-aggregated monthly usage statistics for fast budget checks.

**Why Pre-Aggregate?**:
- ✅ Fast budget checks: No need to SUM millions of rows
- ✅ Dashboard queries: Instant monthly reports
- ✅ Redis cache seed: Use for distributed rate limiting

**Update Strategy**:
- **Option 1**: Trigger on INSERT to `usage_records`
- **Option 2**: Periodic job (every 5 minutes) to aggregate recent records
- **Option 3**: Real-time via application code

**Example Query**:
```sql
-- Check if key is over budget
SELECT 
    monthly_budget_usd,
    total_cost_usd,
    (total_cost_usd / NULLIF(monthly_budget_usd, 0) * 100) AS budget_used_percent
FROM api_keys ak
JOIN monthly_usage_summary mus ON ak.id = mus.api_key_id
WHERE ak.id = '<key-id>'
  AND mus.year = EXTRACT(YEAR FROM NOW())
  AND mus.month = EXTRACT(MONTH FROM NOW());
```

## Indexes

### Performance-Critical Indexes

```sql
-- api_keys: Fast lookup by hash (authentication)
CREATE INDEX idx_api_keys_hash ON api_keys(key_hash);

-- usage_records: Fast queries by key and time range
CREATE INDEX idx_usage_records_api_key_created 
ON usage_records(api_key_id, created_at DESC);

-- models: Fast model name searches (fuzzy matching)
CREATE INDEX idx_models_name_pattern 
ON models USING GIN (model_name gin_trgm_ops);

-- api_key_tags: Fast tag/label queries
CREATE INDEX idx_api_key_tags_api_key ON api_key_tags(api_key_id);
CREATE INDEX idx_api_key_tags_key ON api_key_tags(key);

-- model_alias_tags: Fast tag/label queries
CREATE INDEX idx_model_alias_tags_alias ON model_alias_tags(model_alias_id);
CREATE INDEX idx_model_alias_tags_key ON model_alias_tags(key);

-- admin_users: Fast email lookups and enabled status
CREATE INDEX idx_admin_users_email ON admin_users(email);
CREATE INDEX idx_admin_users_enabled ON admin_users(enabled) WHERE enabled = true;

-- admin_tokens: Fast token lookups and service name queries
CREATE INDEX idx_admin_tokens_service_name ON admin_tokens(service_name);
CREATE INDEX idx_admin_tokens_token_hash ON admin_tokens(token_hash);
CREATE INDEX idx_admin_tokens_enabled ON admin_tokens(enabled) WHERE enabled = true;
CREATE INDEX idx_admin_tokens_expiry ON admin_tokens(expires_at) WHERE expires_at IS NOT NULL;

-- monthly_usage_summary: Fast monthly lookups
CREATE INDEX idx_monthly_summary_api_key_period 
ON monthly_usage_summary(api_key_id, year DESC, month DESC);
```

### Full-Text Search Indexes

```sql
-- models: Search by name, provider, mode
CREATE INDEX idx_models_features 
ON models USING GIN (
    to_tsvector('english', 
        COALESCE(model_name, '') || ' ' ||
        COALESCE(litellm_provider, '') || ' ' ||
        COALESCE(mode, '')
    )
);

-- usage_records: Search metadata
CREATE INDEX idx_usage_records_metadata 
ON usage_records USING GIN (metadata);
```

## Data Flow

### Request Processing

```
1. Client → Gateway (with API key)
   ↓
2. Gateway → api_keys (lookup by key_hash)
   ↓
3. Gateway → key_metadata (get tags/metadata)
   ↓
4. Gateway → monthly_usage_summary (check budget)
   ↓
5. Gateway → models (get pricing for requested model)
   ↓
6. Gateway → providers (get credentials)
   ↓
7. Gateway → LLM Provider (forward request)
   ↓
8. Gateway ← LLM Provider (response)
   ↓
9. Gateway → usage_records (INSERT usage log)
   ↓
10. Gateway → monthly_usage_summary (UPDATE aggregates)
```

### Model Sync

```
1. Cron/Scheduled Job → BerriAI GitHub
   ↓
2. Download model_prices_and_context_window_backup.json
   ↓
3. Parse JSON
   ↓
4. For each model:
   - Check if exists (by model_name)
   - INSERT if new
   - UPDATE if changed
   - Track sync_version and last_synced_at
   ↓
5. Log sync results
```

## Query Examples

### Find Cheapest Model for Chat

```sql
SELECT 
    model_name,
    litellm_provider,
    input_cost_per_token,
    output_cost_per_token,
    (input_cost_per_token + output_cost_per_token) AS total_cost
FROM models
WHERE mode = 'chat'
  AND supports_function_calling = true
ORDER BY total_cost ASC
LIMIT 10;
```

### Get Key Usage by Month

```sql
SELECT 
    ak.name,
    mus.year,
    mus.month,
    mus.total_requests,
    mus.total_cost_usd,
    ak.monthly_budget_usd,
    ROUND((mus.total_cost_usd / NULLIF(ak.monthly_budget_usd, 0) * 100)::numeric, 2) AS budget_used_percent
FROM api_keys ak
LEFT JOIN monthly_usage_summary mus ON ak.id = mus.api_key_id
WHERE ak.id = '<key-id>'
ORDER BY mus.year DESC, mus.month DESC;
```

### Top 10 Most Expensive Requests

```sql
SELECT 
    ur.created_at,
    ak.name AS api_key_name,
    ur.model_name,
    ur.input_tokens,
    ur.output_tokens,
    ur.total_cost_usd,
    ur.response_time_ms
FROM usage_records ur
JOIN api_keys ak ON ur.api_key_id = ak.id
ORDER BY ur.total_cost_usd DESC
LIMIT 10;
```

### Keys Approaching Budget Limit

```sql
SELECT 
    ak.name,
    ak.monthly_budget_usd,
    mus.total_cost_usd,
    ROUND((mus.total_cost_usd / NULLIF(ak.monthly_budget_usd, 0) * 100)::numeric, 2) AS budget_used_percent
FROM api_keys ak
JOIN monthly_usage_summary mus ON ak.id = mus.api_key_id
WHERE mus.year = EXTRACT(YEAR FROM NOW())
  AND mus.month = EXTRACT(MONTH FROM NOW())
  AND ak.monthly_budget_usd IS NOT NULL
  AND (mus.total_cost_usd / NULLIF(ak.monthly_budget_usd, 0)) > 0.8
ORDER BY budget_used_percent DESC;
```

### Find Models by Feature

```sql
-- All models that support vision and function calling
SELECT 
    model_name,
    litellm_provider,
    max_input_tokens,
    input_cost_per_token,
    output_cost_per_token
FROM models
WHERE supports_vision = true
  AND supports_function_calling = true
ORDER BY input_cost_per_token ASC;
```

### Get Tag Summary

```sql
-- Count API keys by tag
SELECT 
    key,
    value,
    COUNT(*) AS key_count
FROM api_key_tags
GROUP BY key, value
ORDER BY key, key_count DESC;

-- Count model aliases by tag
SELECT 
    key,
    value,
    COUNT(*) AS alias_count
FROM model_alias_tags
GROUP BY key, value
ORDER BY key, alias_count DESC;
```

## Backup and Maintenance

### Daily Backups

```bash
# Full database backup
pg_dump -Fc $DATABASE_URL > backup_$(date +%Y%m%d).dump

# Compressed backup
pg_dump $DATABASE_URL | gzip > backup_$(date +%Y%m%d).sql.gz
```

### Partitioning Strategy

For `usage_records` table with high volume:

```sql
-- Create parent table as partitioned
CREATE TABLE usage_records_partitioned (
    LIKE usage_records INCLUDING ALL
) PARTITION BY RANGE (created_at);

-- Create monthly partitions
CREATE TABLE usage_records_2025_01 
PARTITION OF usage_records_partitioned
FOR VALUES FROM ('2025-01-01') TO ('2025-02-01');

CREATE TABLE usage_records_2025_02 
PARTITION OF usage_records_partitioned
FOR VALUES FROM ('2025-02-01') TO ('2025-03-01');
```

### Archiving Old Data

```sql
-- Archive usage_records older than 90 days to separate table
INSERT INTO usage_records_archive
SELECT * FROM usage_records
WHERE created_at < NOW() - INTERVAL '90 days';

DELETE FROM usage_records
WHERE created_at < NOW() - INTERVAL '90 days';

-- Or use partitioning and detach old partitions
ALTER TABLE usage_records_partitioned 
DETACH PARTITION usage_records_2024_12;
```

## Schema Versioning

All schema changes are tracked via sqlx migrations in `migrations/`.

Current schema version: **20250123000002** (seed_data)

To check current version:
```bash
sqlx migrate info
```

## References

- [PostgreSQL JSONB](https://www.postgresql.org/docs/current/datatype-json.html)
- [PostgreSQL Partitioning](https://www.postgresql.org/docs/current/ddl-partitioning.html)
- [BerriAI LiteLLM Models](https://github.com/BerriAI/litellm/blob/main/model_prices_and_context_window.json)
