-- Seed data for ThinkPixelLLMGW
-- Migration: 20250123000002_seed_data
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
-- Seed: Sample Models from BerriAI
-- Note: In production, this would be populated via a sync script that fetches
-- the latest data from BerriAI/LiteLLM repository
-- ============================================================================

-- GPT-5 (latest from BerriAI)
INSERT INTO models (
    id, model_name, litellm_provider, mode,
    input_cost_per_token, output_cost_per_token,
    cache_read_input_token_cost, cache_read_input_token_cost_flex,
    input_cost_per_token_flex, output_cost_per_token_flex,
    input_cost_per_token_priority, output_cost_per_token_priority,
    max_input_tokens, max_output_tokens, max_tokens,
    supports_function_calling, supports_parallel_function_calling,
    supports_native_streaming, supports_tool_choice, supports_vision,
    supports_pdf_input, supports_prompt_caching, supports_reasoning,
    supports_response_schema, supports_system_messages,
    supported_endpoints, supported_modalities, supported_output_modalities,
    sync_source, last_synced_at
) VALUES (
    '22222222-2222-2222-2222-222222222222',
    'gpt-5', 'openai', 'chat',
    0.00000125, 0.00001,
    0.000000125, 0.0000000625,
    0.000000625, 0.000005,
    0.0000025, 0.00002,
    272000, 128000, 128000,
    true, true, true, true, true,
    true, true, true, true, true,
    ARRAY['/v1/chat/completions', '/v1/batch', '/v1/responses'],
    ARRAY['text', 'image'],
    ARRAY['text'],
    'litellm', NOW()
);

-- GPT-5-mini (cost-effective option)
INSERT INTO models (
    id, model_name, litellm_provider, mode,
    input_cost_per_token, output_cost_per_token,
    cache_read_input_token_cost,
    max_input_tokens, max_output_tokens, max_tokens,
    supports_function_calling, supports_parallel_function_calling,
    supports_native_streaming, supports_tool_choice, supports_vision,
    supports_pdf_input, supports_prompt_caching, supports_reasoning,
    supports_response_schema, supports_system_messages,
    supported_endpoints, supported_modalities, supported_output_modalities,
    sync_source, last_synced_at
) VALUES (
    '33333333-3333-3333-3333-333333333333',
    'gpt-5-mini', 'openai', 'chat',
    0.00000025, 0.000002,
    0.000000025,
    272000, 128000, 128000,
    true, true, true, true, true,
    true, true, true, true, true,
    ARRAY['/v1/chat/completions', '/v1/batch', '/v1/responses'],
    ARRAY['text', 'image'],
    ARRAY['text'],
    'litellm', NOW()
);

-- Gemini 2.5 Flash (Google Vertex AI)
INSERT INTO models (
    id, model_name, litellm_provider, mode,
    input_cost_per_token, output_cost_per_token,
    input_cost_per_audio_token, output_cost_per_reasoning_token,
    cache_read_input_token_cost,
    max_input_tokens, max_output_tokens, max_tokens,
    supports_function_calling, supports_parallel_function_calling,
    supports_tool_choice, supports_vision, supports_audio_input,
    supports_pdf_input, supports_prompt_caching, supports_reasoning,
    supports_response_schema, supports_system_messages, supports_web_search,
    supported_endpoints, supported_modalities, supported_output_modalities,
    metadata,
    sync_source, last_synced_at
) VALUES (
    '44444444-4444-4444-4444-444444444444',
    'gemini-2.5-flash', 'vertex_ai-language-models', 'chat',
    0.0000003, 0.0000025,
    0.000001, 0.0000025,
    0.00000003,
    1048576, 65535, 65535,
    true, true, true, true, true,
    true, true, true, true, true, true,
    ARRAY['/v1/chat/completions', '/v1/completions', '/v1/batch'],
    ARRAY['text', 'image', 'audio', 'video'],
    ARRAY['text'],
    '{"max_audio_length_hours": 8.4, "max_audio_per_prompt": 1, "max_images_per_prompt": 3000, "max_pdf_size_mb": 30, "max_video_length": 1, "max_videos_per_prompt": 10}'::jsonb,
    'litellm', NOW()
);

-- Claude 3.7 Sonnet (Anthropic)
INSERT INTO models (
    id, model_name, litellm_provider, mode,
    input_cost_per_token, output_cost_per_token,
    cache_creation_input_token_cost, cache_read_input_token_cost,
    max_input_tokens, max_output_tokens, max_tokens,
    supports_function_calling, supports_tool_choice, supports_vision,
    supports_pdf_input, supports_prompt_caching, supports_reasoning,
    supports_response_schema, supports_system_messages,
    supported_modalities, supported_output_modalities,
    metadata,
    sync_source, last_synced_at
) VALUES (
    '55555555-5555-5555-5555-555555555555',
    'claude-3-7-sonnet', 'anthropic', 'chat',
    0.000003, 0.000015,
    0.00000375, 0.0000003,
    200000, 8192, 8192,
    true, true, true,
    true, true, true, true, true,
    ARRAY['text', 'image'],
    ARRAY['text'],
    '{"supports_assistant_prefill": true, "supports_computer_use": true, "tool_use_system_prompt_tokens": 159}'::jsonb,
    'litellm', NOW()
);

-- ============================================================================
-- Seed: Model Aliases (user-friendly names)
-- ============================================================================
INSERT INTO model_aliases (alias, target_model_id, enabled) VALUES
    ('gpt5', '22222222-2222-2222-2222-222222222222', true),
    ('gpt-5-latest', '22222222-2222-2222-2222-222222222222', true),
    ('gpt5-mini', '33333333-3333-3333-3333-333333333333', true),
    ('gemini-flash', '44444444-4444-4444-4444-444444444444', true),
    ('claude-sonnet', '55555555-5555-5555-5555-555555555555', true);

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
    '5e884898da28047151d0e56f8dc6292773603d0d6aabbdd62a11ef721d1542d8', -- SHA-256 of 'demo-key-12345'
    ARRAY['gpt-5', 'gpt-5-mini', 'gemini-2.5-flash', 'claude-3-7-sonnet'],
    60,
    100.00,
    true
);

-- ============================================================================
-- Seed: Demo Key Metadata (tags, labels, custom fields)
-- ============================================================================
INSERT INTO key_metadata (api_key_id, metadata_type, key, value) VALUES
    ('66666666-6666-6666-6666-666666666666', 'tag', 'environment', 'development'),
    ('66666666-6666-6666-6666-666666666666', 'tag', 'team', 'engineering'),
    ('66666666-6666-6666-6666-666666666666', 'label', 'purpose', 'Demo and Testing'),
    ('66666666-6666-6666-6666-666666666666', 'custom_field', 'project', 'ThinkPixelLLMGW'),
    ('66666666-6666-6666-6666-666666666666', 'custom_field', 'owner', 'bdobrica');

-- ============================================================================
-- Comments
-- ============================================================================
COMMENT ON TABLE providers IS 'Note: In production, run a sync script to populate models table from https://raw.githubusercontent.com/BerriAI/litellm/refs/heads/main/litellm/model_prices_and_context_window_backup.json';
