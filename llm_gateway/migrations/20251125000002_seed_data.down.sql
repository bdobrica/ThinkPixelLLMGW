-- Rollback migration: 20251125000002_seed_data
-- This script removes all seed data added in the up migration

-- Delete in reverse order to respect foreign key constraints
DELETE FROM api_key_tags WHERE api_key_id = '66666666-6666-6666-6666-666666666666';
DELETE FROM api_keys WHERE id = '66666666-6666-6666-6666-666666666666';
DELETE FROM providers WHERE id = '11111111-1111-1111-1111-111111111111';
