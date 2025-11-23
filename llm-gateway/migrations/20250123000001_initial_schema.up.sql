-- Initial schema for ThinkPixelLLMGW
-- Migration: 20250123000001_initial_schema

-- ============================================================================
-- Table: providers
-- Stores LLM provider configurations (OpenAI, VertexAI, Bedrock, etc.)
-- ============================================================================
CREATE TABLE IF NOT EXISTS providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL UNIQUE, -- e.g., "openai", "vertexai", "bedrock"
    display_name VARCHAR(255) NOT NULL, -- e.g., "OpenAI", "Google Vertex AI"
    provider_type VARCHAR(50) NOT NULL, -- openai, vertexai, bedrock, etc.
    
    -- Encrypted credentials stored as JSONB
    -- Example: {"api_key": "encrypted_value"} or {"project_id": "...", "credentials": "..."}
    encrypted_credentials JSONB,
    
    -- Provider-specific configuration
    config JSONB DEFAULT '{}'::jsonb,
    
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_providers_enabled ON providers(enabled) WHERE enabled = true;
CREATE INDEX idx_providers_type ON providers(provider_type);

-- ============================================================================
-- Table: models
-- Stores model catalog synced from BerriAI/LiteLLM model_prices_and_context_window_backup.json
-- ============================================================================
CREATE TABLE IF NOT EXISTS models (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model_name VARCHAR(255) NOT NULL UNIQUE, -- e.g., "gpt-5", "gemini-2.5-flash"
    litellm_provider VARCHAR(100) NOT NULL, -- e.g., "openai", "vertex_ai-language-models"
    
    -- Pricing information (all in USD per token unless specified)
    input_cost_per_token DECIMAL(20, 15),
    output_cost_per_token DECIMAL(20, 15),
    input_cost_per_token_batches DECIMAL(20, 15),
    output_cost_per_token_batches DECIMAL(20, 15),
    cache_read_input_token_cost DECIMAL(20, 15),
    cache_creation_input_token_cost DECIMAL(20, 15),
    input_cost_per_audio_token DECIMAL(20, 15),
    output_cost_per_audio_token DECIMAL(20, 15),
    output_cost_per_reasoning_token DECIMAL(20, 15),
    
    -- Tiered pricing for context windows
    input_cost_per_token_above_128k_tokens DECIMAL(20, 15),
    output_cost_per_token_above_128k_tokens DECIMAL(20, 15),
    input_cost_per_token_above_200k_tokens DECIMAL(20, 15),
    output_cost_per_token_above_200k_tokens DECIMAL(20, 15),
    
    -- Alternative pricing models
    input_cost_per_image DECIMAL(20, 15),
    output_cost_per_image DECIMAL(20, 15),
    input_cost_per_character DECIMAL(20, 15),
    output_cost_per_character DECIMAL(20, 15),
    input_cost_per_query DECIMAL(20, 15),
    
    -- Context window limits
    max_input_tokens INTEGER,
    max_output_tokens INTEGER,
    max_tokens INTEGER,
    
    -- Model mode: chat, completion, embedding, image_generation, etc.
    mode VARCHAR(50),
    
    -- Supported features (as JSONB for flexibility)
    supports_function_calling BOOLEAN DEFAULT false,
    supports_parallel_function_calling BOOLEAN DEFAULT false,
    supports_tool_choice BOOLEAN DEFAULT false,
    supports_vision BOOLEAN DEFAULT false,
    supports_audio_input BOOLEAN DEFAULT false,
    supports_audio_output BOOLEAN DEFAULT false,
    supports_prompt_caching BOOLEAN DEFAULT false,
    supports_reasoning BOOLEAN DEFAULT false,
    supports_response_schema BOOLEAN DEFAULT false,
    supports_system_messages BOOLEAN DEFAULT false,
    supports_native_streaming BOOLEAN DEFAULT false,
    supports_pdf_input BOOLEAN DEFAULT false,
    supports_web_search BOOLEAN DEFAULT false,
    
    -- Additional metadata from BerriAI
    supported_endpoints TEXT[], -- ["/v1/chat/completions", "/v1/batch"]
    supported_modalities TEXT[], -- ["text", "image", "audio"]
    supported_output_modalities TEXT[], -- ["text", "audio"]
    
    -- Full BerriAI metadata stored as JSONB for future extensibility
    metadata JSONB DEFAULT '{}'::jsonb,
    
    -- Sync tracking
    sync_source VARCHAR(255) DEFAULT 'litellm', -- Source of this model data
    sync_version VARCHAR(100), -- Version/commit of the source data
    last_synced_at TIMESTAMPTZ,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_models_provider ON models(litellm_provider);
CREATE INDEX idx_models_mode ON models(mode);
CREATE INDEX idx_models_name_pattern ON models USING GIN (model_name gin_trgm_ops);
CREATE INDEX idx_models_features ON models USING GIN (
    to_tsvector('english', 
        COALESCE(model_name, '') || ' ' ||
        COALESCE(litellm_provider, '') || ' ' ||
        COALESCE(mode, '')
    )
);

-- Enable trigram similarity for model name searches
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- ============================================================================
-- Table: model_aliases
-- Custom aliases for models (e.g., "gpt-5" -> "my-custom-gpt5")
-- ============================================================================
CREATE TABLE IF NOT EXISTS model_aliases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    alias VARCHAR(255) NOT NULL UNIQUE,
    target_model_id UUID NOT NULL REFERENCES models(id) ON DELETE CASCADE,
    provider_id UUID REFERENCES providers(id) ON DELETE SET NULL,
    
    -- Override default model settings
    custom_config JSONB DEFAULT '{}'::jsonb,
    
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_model_aliases_target ON model_aliases(target_model_id);
CREATE INDEX idx_model_aliases_provider ON model_aliases(provider_id);
CREATE INDEX idx_model_aliases_enabled ON model_aliases(enabled) WHERE enabled = true;

-- ============================================================================
-- Table: api_keys
-- Stores API keys for client authentication
-- ============================================================================
CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    key_hash VARCHAR(64) NOT NULL UNIQUE, -- SHA-256 hash of the actual key
    
    -- Access control
    allowed_models TEXT[], -- Array of model names or aliases that this key can access
    
    -- Rate limiting (per minute)
    rate_limit_per_minute INTEGER DEFAULT 60,
    
    -- Budget control
    monthly_budget_usd DECIMAL(10, 2), -- NULL = unlimited
    
    -- Key status
    enabled BOOLEAN NOT NULL DEFAULT true,
    expires_at TIMESTAMPTZ, -- NULL = never expires
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_api_keys_hash ON api_keys(key_hash);
CREATE INDEX idx_api_keys_enabled ON api_keys(enabled) WHERE enabled = true;
CREATE INDEX idx_api_keys_expiry ON api_keys(expires_at) WHERE expires_at IS NOT NULL;

-- ============================================================================
-- Table: key_metadata
-- Flexible key-value store for API key metadata (tags, custom fields, etc.)
-- Allows for better reporting without modifying the api_keys schema
-- ============================================================================
CREATE TABLE IF NOT EXISTS key_metadata (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    api_key_id UUID NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    
    -- Metadata type: tag, label, custom_field, etc.
    metadata_type VARCHAR(50) NOT NULL,
    
    -- Key-value pair
    key VARCHAR(100) NOT NULL,
    value TEXT NOT NULL,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- Ensure unique combinations
    UNIQUE(api_key_id, metadata_type, key)
);

CREATE INDEX idx_key_metadata_api_key ON key_metadata(api_key_id);
CREATE INDEX idx_key_metadata_type_key ON key_metadata(metadata_type, key);
CREATE INDEX idx_key_metadata_value ON key_metadata USING GIN (value gin_trgm_ops);

-- ============================================================================
-- Table: usage_records
-- Tracks all API requests for billing, analytics, and audit
-- ============================================================================
CREATE TABLE IF NOT EXISTS usage_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    
    -- Request metadata
    api_key_id UUID NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    model_id UUID REFERENCES models(id) ON DELETE SET NULL,
    provider_id UUID REFERENCES providers(id) ON DELETE SET NULL,
    
    -- Request details
    request_id VARCHAR(255), -- Correlation ID for tracing
    model_name VARCHAR(255) NOT NULL, -- Snapshot of model name at request time
    endpoint VARCHAR(100), -- e.g., "/v1/chat/completions"
    
    -- Token usage
    input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    cached_tokens INTEGER DEFAULT 0,
    reasoning_tokens INTEGER DEFAULT 0,
    
    -- Cost calculation (USD)
    input_cost_usd DECIMAL(15, 10) NOT NULL DEFAULT 0,
    output_cost_usd DECIMAL(15, 10) NOT NULL DEFAULT 0,
    cache_cost_usd DECIMAL(15, 10) DEFAULT 0,
    total_cost_usd DECIMAL(15, 10) NOT NULL DEFAULT 0,
    
    -- Response metadata
    response_time_ms INTEGER, -- Latency in milliseconds
    status_code INTEGER, -- HTTP status code
    error_message TEXT, -- Error details if request failed
    
    -- Additional request context (stored as JSONB for flexibility)
    metadata JSONB DEFAULT '{}'::jsonb,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Partitioning by month for efficient querying and archiving
-- Note: This would be implemented via PostgreSQL native partitioning in production
CREATE INDEX idx_usage_records_api_key_created ON usage_records(api_key_id, created_at DESC);
CREATE INDEX idx_usage_records_model_created ON usage_records(model_id, created_at DESC);
CREATE INDEX idx_usage_records_created ON usage_records(created_at DESC);
CREATE INDEX idx_usage_records_request_id ON usage_records(request_id);

-- GIN index for JSONB metadata queries
CREATE INDEX idx_usage_records_metadata ON usage_records USING GIN (metadata);

-- ============================================================================
-- Table: monthly_usage_summary
-- Pre-aggregated monthly usage for fast budget checks
-- Populated via trigger or periodic job
-- ============================================================================
CREATE TABLE IF NOT EXISTS monthly_usage_summary (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    api_key_id UUID NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    
    year INTEGER NOT NULL,
    month INTEGER NOT NULL,
    
    -- Aggregated metrics
    total_requests BIGINT NOT NULL DEFAULT 0,
    total_input_tokens BIGINT NOT NULL DEFAULT 0,
    total_output_tokens BIGINT NOT NULL DEFAULT 0,
    total_cost_usd DECIMAL(15, 2) NOT NULL DEFAULT 0,
    
    -- Last request timestamp
    last_request_at TIMESTAMPTZ,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    UNIQUE(api_key_id, year, month)
);

CREATE INDEX idx_monthly_summary_api_key_period ON monthly_usage_summary(api_key_id, year DESC, month DESC);

-- ============================================================================
-- Triggers for updated_at timestamps
-- ============================================================================
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_providers_updated_at BEFORE UPDATE ON providers
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_models_updated_at BEFORE UPDATE ON models
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_model_aliases_updated_at BEFORE UPDATE ON model_aliases
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_api_keys_updated_at BEFORE UPDATE ON api_keys
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_key_metadata_updated_at BEFORE UPDATE ON key_metadata
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_monthly_usage_summary_updated_at BEFORE UPDATE ON monthly_usage_summary
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================================================
-- Comments for documentation
-- ============================================================================
COMMENT ON TABLE providers IS 'LLM provider configurations (OpenAI, VertexAI, Bedrock, etc.)';
COMMENT ON TABLE models IS 'Model catalog synced from BerriAI/LiteLLM with pricing and capabilities';
COMMENT ON TABLE model_aliases IS 'Custom model aliases for user-friendly naming';
COMMENT ON TABLE api_keys IS 'Client API keys with rate limiting and budget controls';
COMMENT ON TABLE key_metadata IS 'Flexible metadata store for API keys (tags, labels, custom fields)';
COMMENT ON TABLE usage_records IS 'Request audit log for billing and analytics';
COMMENT ON TABLE monthly_usage_summary IS 'Pre-aggregated monthly usage statistics';
