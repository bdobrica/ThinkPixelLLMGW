-- Seed data for ThinkPixelLLMGW
-- Migration: 20251125000002_seed_data
-- Created: 2025-11-25
-- This migration adds initial demo data for development and testing

-- ============================================================================
-- Seed: Demo Provider (OpenAI)
-- ============================================================================
INSERT INTO providers (id, name, display_name, provider_type, enabled, config) VALUES
    ('11111111-1111-1111-1111-111111111111', 'openai', 'OpenAI', 'openai', true, 
     '{"base_url": "https://api.openai.com/v1"}'::jsonb);

-- Note: In production, encrypted_credentials would be populated via admin API
-- Example structure: {"api_key": "<encrypted_api_key>"}

-- ============================================================================
-- Seed: Demo API Key (for testing only)
-- Hash of "demo-key-12345" = SHA-256
-- ============================================================================
INSERT INTO api_keys (
    id, name, key_hash,
    allowed_models, rate_limit_per_minute, monthly_budget_usd,
    enabled
) VALUES (
    '66666666-6666-6666-6666-666666666666',
    'Demo Development Key',
    '367fe8933ad8bba8f7ff02c047bcb5c00a4fff3ad6e82fef2bf4ee0c850d7c36', -- SHA-256 of 'demo-key-12345'
    ARRAY['gpt-4o', 'gpt-4o-mini', 'o1', 'o1-mini'],
    60,
    100.00,
    true
);

-- ============================================================================
-- Seed: Demo Key Tags
-- ============================================================================
INSERT INTO api_key_tags (api_key_id, key, value) VALUES
    ('66666666-6666-6666-6666-666666666666', 'environment', 'development'),
    ('66666666-6666-6666-6666-666666666666', 'team', 'engineering'),
    ('66666666-6666-6666-6666-666666666666', 'purpose', 'Demo and Testing'),
    ('66666666-6666-6666-6666-666666666666', 'project', 'ThinkPixelLLMGW'),
    ('66666666-6666-6666-6666-666666666666', 'owner', 'bdobrica');

-- ============================================================================
-- Seed: Demo Models (OpenAI)
-- ============================================================================
INSERT INTO models (
    id, model_name, provider_id, source, version,
    supports_text_input, supports_text_output,
    supports_function_calling, supports_streaming_output,
    max_tokens, max_input_tokens, max_output_tokens,
    currency, pricing_component_schema_version
) VALUES
    -- GPT-4o
    ('22222222-2222-2222-2222-222222222222', 'gpt-4o', 'openai', 'openai', '2024-11-20',
     true, true, true, true,
     128000, 128000, 16384,
     'USD', 'v1'),
    -- GPT-4o-mini
    ('33333333-3333-3333-3333-333333333333', 'gpt-4o-mini', 'openai', 'openai', '2024-07-18',
     true, true, true, true,
     128000, 128000, 16384,
     'USD', 'v1'),
    -- o1
    ('44444444-4444-4444-4444-444444444444', 'o1', 'openai', 'openai', '2024-12-17',
     true, true, false, false,
     200000, 200000, 100000,
     'USD', 'v1'),
    -- o1-mini
    ('55555555-5555-5555-5555-555555555555', 'o1-mini', 'openai', 'openai', '2024-09-12',
     true, true, false, false,
     128000, 128000, 65536,
     'USD', 'v1');

-- ============================================================================
-- Seed: Pricing Components for Demo Models
-- ============================================================================
INSERT INTO pricing_components (model_id, code, direction, modality, unit, price) VALUES
    -- GPT-4o pricing
    ('22222222-2222-2222-2222-222222222222', 'input_tokens', 'input', 'text', 'token', 0.0000025),    -- $2.50 per 1M tokens
    ('22222222-2222-2222-2222-222222222222', 'output_tokens', 'output', 'text', 'token', 0.00001),     -- $10.00 per 1M tokens
    -- GPT-4o-mini pricing
    ('33333333-3333-3333-3333-333333333333', 'input_tokens', 'input', 'text', 'token', 0.00000015),   -- $0.15 per 1M tokens
    ('33333333-3333-3333-3333-333333333333', 'output_tokens', 'output', 'text', 'token', 0.0000006),   -- $0.60 per 1M tokens
    -- o1 pricing
    ('44444444-4444-4444-4444-444444444444', 'input_tokens', 'input', 'text', 'token', 0.000015),     -- $15.00 per 1M tokens
    ('44444444-4444-4444-4444-444444444444', 'output_tokens', 'output', 'text', 'token', 0.00006),     -- $60.00 per 1M tokens
    -- o1-mini pricing
    ('55555555-5555-5555-5555-555555555555', 'input_tokens', 'input', 'text', 'token', 0.000003),     -- $3.00 per 1M tokens
    ('55555555-5555-5555-5555-555555555555', 'output_tokens', 'output', 'text', 'token', 0.000012);    -- $12.00 per 1M tokens

-- ============================================================================
-- Comments
-- ============================================================================
COMMENT ON TABLE providers IS 'Note: Use model sync script to populate models table from provider catalogs';
