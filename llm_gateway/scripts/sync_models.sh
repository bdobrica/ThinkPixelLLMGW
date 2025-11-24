#!/usr/bin/env bash
# Sync models from BerriAI/LiteLLM repository
# This script fetches the latest model pricing data and updates the database

set -euo pipefail

# Configuration
BERRYAI_JSON_URL="https://raw.githubusercontent.com/BerriAI/litellm/refs/heads/main/litellm/model_prices_and_context_window_backup.json"
TEMP_FILE="/tmp/litellm_models.json"
SYNC_VERSION="$(date +%Y%m%d-%H%M%S)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "🔄 Syncing models from BerriAI/LiteLLM..."

# Download latest model data
echo "📥 Downloading model pricing data..."
if ! curl -sSL -o "$TEMP_FILE" "$BERRYAI_JSON_URL"; then
    echo -e "${RED}❌ Failed to download model data${NC}"
    exit 1
fi

echo -e "${GREEN}✅ Downloaded successfully${NC}"

# Validate JSON
if ! jq empty "$TEMP_FILE" 2>/dev/null; then
    echo -e "${RED}❌ Invalid JSON file${NC}"
    rm -f "$TEMP_FILE"
    exit 1
fi

# Count models
MODEL_COUNT=$(jq '. | length' "$TEMP_FILE")
echo "📊 Found $MODEL_COUNT models in BerriAI catalog"

# TODO: Implement database sync
# For now, this is a placeholder script. The actual implementation would:
# 1. Parse JSON file
# 2. Connect to PostgreSQL
# 3. For each model:
#    - Check if it exists (by model_name)
#    - INSERT if new, UPDATE if changed
#    - Track sync_version and last_synced_at
# 4. Optionally: Mark models not in BerriAI as deprecated

echo ""
echo -e "${YELLOW}⚠️  This is a placeholder script${NC}"
echo "To implement model sync, you need to:"
echo "1. Parse the JSON file with jq or a Go/Python script"
echo "2. Connect to PostgreSQL (using psql, Go sqlx, or Python psycopg2)"
echo "3. Upsert models into the 'models' table"
echo ""
echo "Example SQL for upsert:"
echo ""
cat << 'EOF'
INSERT INTO models (
    model_name, litellm_provider, mode,
    input_cost_per_token, output_cost_per_token,
    max_input_tokens, max_output_tokens, max_tokens,
    supports_function_calling, supports_vision,
    metadata, sync_source, sync_version, last_synced_at
) VALUES (
    'gpt-5', 'openai', 'chat',
    0.00000125, 0.00001,
    272000, 128000, 128000,
    true, true,
    '{"supported_endpoints": ["/v1/chat/completions"]}'::jsonb,
    'litellm', '20250123-120000', NOW()
)
ON CONFLICT (model_name) 
DO UPDATE SET
    input_cost_per_token = EXCLUDED.input_cost_per_token,
    output_cost_per_token = EXCLUDED.output_cost_per_token,
    max_input_tokens = EXCLUDED.max_input_tokens,
    max_output_tokens = EXCLUDED.max_output_tokens,
    metadata = EXCLUDED.metadata,
    sync_version = EXCLUDED.sync_version,
    last_synced_at = NOW();
EOF

echo ""
echo "Sync version: $SYNC_VERSION"
echo "Downloaded file: $TEMP_FILE"

# Cleanup
# rm -f "$TEMP_FILE"

echo -e "${GREEN}✨ Sync preparation complete${NC}"
