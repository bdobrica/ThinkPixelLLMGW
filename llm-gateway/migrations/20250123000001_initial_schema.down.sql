-- Rollback migration: 20250123000001_initial_schema
-- This script reverses all changes made in the up migration

-- Drop triggers first
DROP TRIGGER IF EXISTS update_monthly_usage_summary_updated_at ON monthly_usage_summary;
DROP TRIGGER IF EXISTS update_key_metadata_updated_at ON key_metadata;
DROP TRIGGER IF EXISTS update_api_keys_updated_at ON api_keys;
DROP TRIGGER IF EXISTS update_model_aliases_updated_at ON model_aliases;
DROP TRIGGER IF EXISTS update_models_updated_at ON models;
DROP TRIGGER IF EXISTS update_providers_updated_at ON providers;

-- Drop the trigger function
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop tables in reverse order (respecting foreign key dependencies)
DROP TABLE IF EXISTS monthly_usage_summary;
DROP TABLE IF EXISTS usage_records;
DROP TABLE IF EXISTS key_metadata;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS model_aliases;
DROP TABLE IF EXISTS models;
DROP TABLE IF EXISTS providers;

-- Drop extensions if no longer needed
-- Note: Only drop if you're certain no other tables use them
-- DROP EXTENSION IF EXISTS pg_trgm;
