-- Initial schema for ThinkPixelLLMGW
-- Migration: 20251125000001_initial_schema
-- Created: 2025-11-25

-- ============================================================================
-- Table: providers
-- Stores LLM provider configurations (OpenAI, VertexAI, Bedrock, etc.)
-- ============================================================================
CREATE TABLE providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL UNIQUE,
    display_name VARCHAR(255) NOT NULL,
    provider_type VARCHAR(50) NOT NULL,
    encrypted_credentials JSONB,
    config JSONB DEFAULT '{}'::jsonb,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_providers_enabled ON providers(enabled) WHERE enabled = true;
CREATE INDEX idx_providers_type ON providers(provider_type);

-- ============================================================================
-- Table: models
-- Comprehensive model catalog with all features, limits, and metadata
-- ============================================================================
CREATE TABLE models (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    
    -- Identity & lifecycle
    model_name VARCHAR(255) NOT NULL UNIQUE,
    provider_id VARCHAR(255) NOT NULL,
    source VARCHAR(255) NOT NULL,
    version VARCHAR(100),
    deprecation_date TIMESTAMPTZ,
    is_deprecated BOOLEAN NOT NULL DEFAULT false,
    
    -- Regions & resolutions
    supported_regions TEXT[],
    supported_resolutions TEXT[],
    
    -- Feature support flags (original set)
    supports_assistant_prefill BOOLEAN NOT NULL DEFAULT false,
    supports_audio_input BOOLEAN NOT NULL DEFAULT false,
    supports_audio_output BOOLEAN NOT NULL DEFAULT false,
    supports_computer_use BOOLEAN NOT NULL DEFAULT false,
    supports_embedding_image_input BOOLEAN NOT NULL DEFAULT false,
    supports_function_calling BOOLEAN NOT NULL DEFAULT false,
    supports_image_input BOOLEAN NOT NULL DEFAULT false,
    supports_native_streaming BOOLEAN NOT NULL DEFAULT false,
    supports_parallel_function_calling BOOLEAN NOT NULL DEFAULT false,
    supports_pdf_input BOOLEAN NOT NULL DEFAULT false,
    supports_prompt_caching BOOLEAN NOT NULL DEFAULT false,
    supports_reasoning BOOLEAN NOT NULL DEFAULT false,
    supports_response_schema BOOLEAN NOT NULL DEFAULT false,
    supports_service_tier BOOLEAN NOT NULL DEFAULT false,
    supports_system_messages BOOLEAN NOT NULL DEFAULT false,
    supports_tool_choice BOOLEAN NOT NULL DEFAULT false,
    supports_url_context BOOLEAN NOT NULL DEFAULT false,
    supports_video_input BOOLEAN NOT NULL DEFAULT false,
    supports_vision BOOLEAN NOT NULL DEFAULT false,
    supports_web_search BOOLEAN NOT NULL DEFAULT false,
    
    -- Extended feature flags
    supports_text_input BOOLEAN NOT NULL DEFAULT false,
    supports_text_output BOOLEAN NOT NULL DEFAULT false,
    supports_image_output BOOLEAN NOT NULL DEFAULT false,
    supports_video_output BOOLEAN NOT NULL DEFAULT false,
    supports_batch_requests BOOLEAN NOT NULL DEFAULT false,
    supports_json_output BOOLEAN NOT NULL DEFAULT false,
    supports_rerank BOOLEAN NOT NULL DEFAULT false,
    supports_embedding_text_input BOOLEAN NOT NULL DEFAULT false,
    supports_streaming_output BOOLEAN NOT NULL DEFAULT false,
    
    -- Limits & quotas - throughput
    tokens_per_minute INTEGER NOT NULL DEFAULT 0,
    requests_per_minute INTEGER NOT NULL DEFAULT 0,
    requests_per_day INTEGER NOT NULL DEFAULT 0,
    
    -- Token & context limits
    max_tokens INTEGER NOT NULL DEFAULT 0,
    max_input_tokens INTEGER NOT NULL DEFAULT 0,
    max_output_tokens INTEGER NOT NULL DEFAULT 0,
    max_query_tokens INTEGER NOT NULL DEFAULT 0,
    max_tokens_per_document_chunk INTEGER NOT NULL DEFAULT 0,
    max_document_chunks_per_query INTEGER NOT NULL DEFAULT 0,
    tool_use_system_prompt_tokens INTEGER NOT NULL DEFAULT 0,
    output_vector_size INTEGER NOT NULL DEFAULT 0,
    
    -- Modality-specific limits
    max_audio_length_hours DOUBLE PRECISION NOT NULL DEFAULT 0,
    max_audio_per_prompt INTEGER NOT NULL DEFAULT 0,
    max_images_per_prompt INTEGER NOT NULL DEFAULT 0,
    max_pdf_size_mb INTEGER NOT NULL DEFAULT 0,
    max_video_length INTEGER NOT NULL DEFAULT 0,
    max_videos_per_prompt INTEGER NOT NULL DEFAULT 0,
    
    -- Extended limits
    max_requests_per_second INTEGER NOT NULL DEFAULT 0,
    max_concurrent_requests INTEGER NOT NULL DEFAULT 0,
    max_batch_size INTEGER NOT NULL DEFAULT 0,
    max_audio_length_seconds INTEGER NOT NULL DEFAULT 0,
    max_video_length_seconds INTEGER NOT NULL DEFAULT 0,
    max_context_window_tokens INTEGER NOT NULL DEFAULT 0,
    max_output_tokens_per_request INTEGER NOT NULL DEFAULT 0,
    max_input_tokens_per_request INTEGER NOT NULL DEFAULT 0,
    
    -- Pricing (normalized)
    currency VARCHAR(10) NOT NULL DEFAULT 'USD',
    pricing_component_schema_version VARCHAR(50),
    
    -- Operational metadata
    average_latency_ms DOUBLE PRECISION NOT NULL DEFAULT 0,
    p95_latency_ms DOUBLE PRECISION NOT NULL DEFAULT 0,
    availability_slo DOUBLE PRECISION NOT NULL DEFAULT 0,
    sla_tier VARCHAR(50),
    supports_sla BOOLEAN NOT NULL DEFAULT false,
    
    -- Generic metadata
    metadata_schema_version VARCHAR(50),
    metadata JSONB DEFAULT '{}'::jsonb,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_models_provider ON models(provider_id);
CREATE INDEX idx_models_model_name ON models(model_name);
CREATE INDEX idx_models_deprecated ON models(is_deprecated) WHERE is_deprecated = false;
CREATE INDEX idx_models_metadata ON models USING GIN (metadata);

-- ============================================================================
-- Table: pricing_components
-- Granular pricing data for models
-- ============================================================================
CREATE TABLE pricing_components (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model_id UUID NOT NULL REFERENCES models(id) ON DELETE CASCADE,
    code VARCHAR(100) NOT NULL,
    direction VARCHAR(50) NOT NULL,
    modality VARCHAR(50) NOT NULL,
    unit VARCHAR(50) NOT NULL,
    tier VARCHAR(50),
    scope VARCHAR(50),
    price DOUBLE PRECISION NOT NULL DEFAULT 0,
    metadata_schema_version VARCHAR(50),
    metadata JSONB DEFAULT '{}'::jsonb,
    
    UNIQUE(model_id, code)
);

CREATE INDEX idx_pricing_components_model ON pricing_components(model_id);
CREATE INDEX idx_pricing_components_direction ON pricing_components(direction);
CREATE INDEX idx_pricing_components_modality ON pricing_components(modality);

-- ============================================================================
-- Table: model_aliases
-- Custom aliases for models
-- ============================================================================
CREATE TABLE model_aliases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    alias VARCHAR(255) NOT NULL UNIQUE,
    target_model_id UUID NOT NULL REFERENCES models(id) ON DELETE CASCADE,
    provider_id UUID REFERENCES providers(id) ON DELETE SET NULL,
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
CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    key_hash VARCHAR(64) NOT NULL UNIQUE,
    allowed_models TEXT[],
    rate_limit_per_minute INTEGER NOT NULL DEFAULT 60,
    monthly_budget_usd DOUBLE PRECISION,
    enabled BOOLEAN NOT NULL DEFAULT true,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_api_keys_hash ON api_keys(key_hash);
CREATE INDEX idx_api_keys_enabled ON api_keys(enabled) WHERE enabled = true;
CREATE INDEX idx_api_keys_expiry ON api_keys(expires_at) WHERE expires_at IS NOT NULL;

-- ============================================================================
-- Table: api_key_tags
-- Key-value tags for API keys
-- ============================================================================
CREATE TABLE api_key_tags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    api_key_id UUID NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    key VARCHAR(100) NOT NULL,
    value TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    UNIQUE(api_key_id, key)
);

CREATE INDEX idx_api_key_tags_api_key ON api_key_tags(api_key_id);
CREATE INDEX idx_api_key_tags_key ON api_key_tags(key);

-- ============================================================================
-- Table: model_alias_tags
-- Key-value tags for model aliases
-- ============================================================================
CREATE TABLE model_alias_tags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model_alias_id UUID NOT NULL REFERENCES model_aliases(id) ON DELETE CASCADE,
    key VARCHAR(100) NOT NULL,
    value TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    UNIQUE(model_alias_id, key)
);

CREATE INDEX idx_model_alias_tags_alias ON model_alias_tags(model_alias_id);
CREATE INDEX idx_model_alias_tags_key ON model_alias_tags(key);

-- ============================================================================
-- Table: usage_records
-- Tracks all API requests for billing, analytics, and audit
-- ============================================================================
CREATE TABLE usage_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    api_key_id UUID NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    model_id UUID REFERENCES models(id) ON DELETE SET NULL,
    provider_id UUID REFERENCES providers(id) ON DELETE SET NULL,
    request_id UUID NOT NULL,
    model_name VARCHAR(255) NOT NULL,
    endpoint VARCHAR(100) NOT NULL,
    input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    cached_tokens INTEGER NOT NULL DEFAULT 0,
    reasoning_tokens INTEGER NOT NULL DEFAULT 0,
    response_time_ms INTEGER NOT NULL DEFAULT 0,
    status_code INTEGER NOT NULL DEFAULT 0,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_usage_records_api_key_created ON usage_records(api_key_id, created_at DESC);
CREATE INDEX idx_usage_records_model_created ON usage_records(model_id, created_at DESC);
CREATE INDEX idx_usage_records_created ON usage_records(created_at DESC);
CREATE INDEX idx_usage_records_request_id ON usage_records(request_id);

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

-- ============================================================================
-- Comments for documentation
-- ============================================================================
COMMENT ON TABLE providers IS 'LLM provider configurations (OpenAI, VertexAI, Bedrock, etc.)';
COMMENT ON TABLE models IS 'Model catalog with comprehensive features, limits, and pricing information';
COMMENT ON TABLE pricing_components IS 'Granular pricing data for each model';
COMMENT ON TABLE model_aliases IS 'Custom model aliases for user-friendly naming';
COMMENT ON TABLE api_keys IS 'Client API keys with rate limiting and budget controls';
COMMENT ON TABLE api_key_tags IS 'Key-value tags for API keys';
COMMENT ON TABLE model_alias_tags IS 'Key-value tags for model aliases';
COMMENT ON TABLE usage_records IS 'Request audit log for billing and analytics';
