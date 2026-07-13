CREATE INDEX idx_usage_records_status_created ON usage_records(status_code, created_at DESC);
CREATE INDEX idx_usage_records_model_name_created ON usage_records(model_name, created_at DESC);
