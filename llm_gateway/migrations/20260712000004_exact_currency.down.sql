ALTER TABLE api_keys ADD COLUMN monthly_budget_usd DOUBLE PRECISION;
UPDATE api_keys SET monthly_budget_usd = monthly_budget_nano_usd::DOUBLE PRECISION / 1000000000;

ALTER TABLE monthly_usage_summary ADD COLUMN total_cost_usd DOUBLE PRECISION NOT NULL DEFAULT 0;
UPDATE monthly_usage_summary SET total_cost_usd = total_cost_nano_usd::DOUBLE PRECISION / 1000000000;

ALTER TABLE pricing_components ALTER COLUMN price TYPE DOUBLE PRECISION USING price::DOUBLE PRECISION;

ALTER TABLE api_keys DROP COLUMN monthly_budget_nano_usd;
ALTER TABLE monthly_usage_summary DROP COLUMN total_cost_nano_usd;
