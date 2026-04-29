CREATE TABLE monthly_usage_summary (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    api_key_id UUID NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    year INTEGER NOT NULL,
    month INTEGER NOT NULL,
    total_requests INTEGER NOT NULL DEFAULT 0,
    total_input_tokens INTEGER NOT NULL DEFAULT 0,
    total_output_tokens INTEGER NOT NULL DEFAULT 0,
    total_cached_tokens INTEGER NOT NULL DEFAULT 0,
    total_reasoning_tokens INTEGER NOT NULL DEFAULT 0,
    total_cost_usd DOUBLE PRECISION NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT monthly_usage_summary_api_key_month_unique UNIQUE (api_key_id, year, month),
    CONSTRAINT monthly_usage_summary_month_check CHECK (month BETWEEN 1 AND 12)
);

CREATE INDEX idx_monthly_usage_summary_api_key ON monthly_usage_summary(api_key_id, year DESC, month DESC);

CREATE TRIGGER update_monthly_usage_summary_updated_at BEFORE UPDATE ON monthly_usage_summary
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
