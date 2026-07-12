-- Currency contract: integer nano-USD (10^-9 USD), rounded half away from zero.
ALTER TABLE api_keys ADD COLUMN monthly_budget_nano_usd BIGINT;
UPDATE api_keys SET monthly_budget_nano_usd = ROUND(monthly_budget_usd * 1000000000)::BIGINT
WHERE monthly_budget_usd IS NOT NULL;

ALTER TABLE monthly_usage_summary ADD COLUMN total_cost_nano_usd BIGINT NOT NULL DEFAULT 0;
UPDATE monthly_usage_summary SET total_cost_nano_usd = ROUND(total_cost_usd * 1000000000)::BIGINT;

ALTER TABLE pricing_components ALTER COLUMN price TYPE NUMERIC(30, 12) USING price::NUMERIC(30, 12);

ALTER TABLE api_keys DROP COLUMN monthly_budget_usd;
ALTER TABLE monthly_usage_summary DROP COLUMN total_cost_usd;
