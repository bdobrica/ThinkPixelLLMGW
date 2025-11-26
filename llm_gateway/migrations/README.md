# Database Migrations

This directory contains SQL migrations for the ThinkPixelLLMGW database schema.

## Tools

We use **sqlx-cli** for database migrations. Install it with:

```bash
cargo install sqlx-cli --no-default-features --features postgres
```

## Setup

1. **Configure Database URL**

   Set the `DATABASE_URL` environment variable:

   ```bash
   export DATABASE_URL="postgres://username:password@localhost:5432/llmgateway"
   ```

   Or use a `.env` file in the project root:

   ```env
   DATABASE_URL=postgres://username:password@localhost:5432/llmgateway
   ```

2. **Create Database**

   ```bash
   sqlx database create
   ```

3. **Run Migrations**

   ```bash
   sqlx migrate run
   ```

## Migration Management

### Create a New Migration

```bash
sqlx migrate add <migration_name>
```

This creates two files:
- `<timestamp>_<migration_name>.up.sql` - Forward migration
- `<timestamp>_<migration_name>.down.sql` - Rollback migration

### Rollback Migrations

```bash
sqlx migrate revert
```

Reverts the last applied migration using the `.down.sql` file.

### Check Migration Status

```bash
sqlx migrate info
```

Shows which migrations have been applied.

## Migration Philosophy

### Backward Compatibility

**All migrations must be backward compatible** to support zero-downtime deployments and horizontal scaling.

Key principles:
- **Additive changes only**: Add new columns/tables instead of modifying existing ones
- **No breaking renames**: If you must rename, create a new column and deprecate the old one
- **Default values**: Always provide defaults for new NOT NULL columns
- **Graceful transitions**: Use multi-phase migrations for complex changes

### Multi-Phase Migration Example

If you need to rename a column:

**Phase 1** (Migration 001):
```sql
-- Add new column
ALTER TABLE api_keys ADD COLUMN new_column_name TEXT;

-- Copy data
UPDATE api_keys SET new_column_name = old_column_name;
```

**Phase 2** (After deploying code that uses new column):
```sql
-- Add NOT NULL constraint
ALTER TABLE api_keys ALTER COLUMN new_column_name SET NOT NULL;
```

**Phase 3** (After verifying everything works):
```sql
-- Drop old column
ALTER TABLE api_keys DROP COLUMN old_column_name;
```

## Migration Files

### 20250123000001_initial_schema

Creates the core tables:
- `providers` - LLM provider configurations (OpenAI, VertexAI, Bedrock, etc.)
- `models` - Model catalog synced from BerriAI/LiteLLM
- `model_aliases` - User-friendly model aliases
- `model_alias_tags` - Flexible tagging for model aliases (categories, use cases)
- `api_keys` - Client API keys with rate limiting and budgets
- `api_key_tags` - Flexible tagging for API keys (environment, ownership, metadata)
- `usage_records` - Request audit log for billing and analytics
- `monthly_usage_summary` - Pre-aggregated monthly usage statistics

### 20250123000002_seed_data

Adds development seed data:
- Sample provider (OpenAI)
- Popular models (GPT-4o, GPT-4o-mini, o1, o1-mini)
- Model aliases with tags
- Demo API key with tags

## Model Sync Strategy

The `models` table is designed to be populated from the BerriAI/LiteLLM repository:

**Source**: https://raw.githubusercontent.com/BerriAI/litellm/refs/heads/main/litellm/model_prices_and_context_window_backup.json

### Sync Script (Planned)

```bash
# Manual sync (for now)
./scripts/sync_models.sh

# Scheduled sync (future)
# Run daily via cron or Kubernetes CronJob
```

The sync script should:
1. Fetch latest JSON from BerriAI repo
2. Parse model definitions
3. Upsert into `models` table (INSERT ON CONFLICT UPDATE)
4. Track sync version and timestamp
5. Log changes for audit

## Schema Design Decisions

### Why Separate Tag Tables?

Instead of storing tags directly in `api_keys` or `model_aliases`, we use separate tables (`api_key_tags` and `model_alias_tags`):

**Benefits**:
- **Flexible schema**: Add new tags without migrations
- **Better reporting**: Easy to query "all keys/aliases with tag X"
- **Indexable**: Efficient searches on tags/labels
- **Backward compatible**: Adding tags doesn't change parent table schema

**Example Queries**:
```sql
-- Find all API keys with environment tag "production"
SELECT ak.* FROM api_keys ak
JOIN api_key_tags akt ON ak.id = akt.api_key_id
WHERE akt.key = 'environment' AND akt.value = 'production';

-- Find all model aliases in "cost-effective" category
SELECT ma.* FROM model_aliases ma
JOIN model_alias_tags mat ON ma.id = mat.model_alias_id
WHERE mat.key = 'category' AND mat.value = 'cost-effective';

-- Get all tags for a key
SELECT key, value FROM api_key_tags
WHERE api_key_id = '<key-id>';
```

### Why Store Full BerriAI Metadata?

The `models.metadata` JSONB column stores the complete BerriAI model definition because:

1. **Future-proofing**: New fields added by BerriAI are automatically captured
2. **Feature detection**: Can query for specific capabilities without schema changes
3. **Extensibility**: Custom fields can be added without migrations
4. **Audit trail**: Original source data preserved for debugging

## Testing Migrations

### Local Testing

1. **Apply migration**:
   ```bash
   sqlx migrate run
   ```

2. **Verify schema**:
   ```bash
   psql $DATABASE_URL -c "\d+ api_keys"
   ```

3. **Test rollback**:
   ```bash
   sqlx migrate revert
   sqlx migrate run
   ```

### Integration Tests

All migrations should be tested in CI/CD:

```bash
# Create test database
sqlx database create --database-url $TEST_DATABASE_URL

# Run migrations
sqlx migrate run --database-url $TEST_DATABASE_URL

# Run tests
cargo test --features postgres

# Cleanup
sqlx database drop --database-url $TEST_DATABASE_URL
```

## Production Deployment

### Pre-Deployment Checklist

- [ ] Migration is backward compatible
- [ ] Down migration tested locally
- [ ] Indexes created for new columns
- [ ] Foreign keys have proper ON DELETE behavior
- [ ] Triggers updated if needed
- [ ] Migration tested on production-like dataset
- [ ] Estimated migration time < 5 minutes (for large tables, consider batching)

### Deployment Process

1. **Backup database** (always!)
2. **Apply migration**: `sqlx migrate run`
3. **Verify migration**: `sqlx migrate info`
4. **Deploy application code**
5. **Monitor errors and performance**
6. **If issues arise**: Rollback code first, then revert migration if needed

### Large Table Migrations

For tables with millions of rows (e.g., `usage_records`):

1. **Avoid table locks**: Use `CREATE INDEX CONCURRENTLY`
2. **Batch updates**: Update in chunks, not all at once
3. **Schedule during low traffic**: Run during maintenance window
4. **Monitor query time**: Use `EXPLAIN ANALYZE`

Example:
```sql
-- Bad: Locks entire table
ALTER TABLE usage_records ADD COLUMN new_field TEXT NOT NULL DEFAULT 'value';

-- Good: Non-blocking index creation
CREATE INDEX CONCURRENTLY idx_usage_records_new_field 
ON usage_records(new_field);
```

## Troubleshooting

### Migration Failed Mid-Way

SQLx wraps migrations in transactions, so they're atomic. If a migration fails, it's automatically rolled back.

Check the error:
```bash
sqlx migrate run
```

Fix the SQL, then re-run.

### Wrong Migration Order

If migrations are applied out of order:
1. Revert all problematic migrations: `sqlx migrate revert`
2. Re-apply in correct order: `sqlx migrate run`

### Migration History Mismatch

If `_sqlx_migrations` table is out of sync with actual schema:

**DO NOT** manually edit `_sqlx_migrations` unless you know what you're doing.

Instead:
1. Create a new migration to fix the schema
2. Mark it as "repair" migration in comments

## References

- [sqlx-cli Documentation](https://github.com/launchbadge/sqlx/tree/main/sqlx-cli)
- [PostgreSQL Documentation](https://www.postgresql.org/docs/current/)
- [BerriAI LiteLLM Model Prices](https://github.com/BerriAI/litellm/blob/main/model_prices_and_context_window.json)
