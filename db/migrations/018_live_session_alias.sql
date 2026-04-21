-- +migrate Up
ALTER TABLE live_sessions ADD COLUMN IF NOT EXISTS alias TEXT DEFAULT '';

-- +migrate Down
ALTER TABLE live_sessions DROP COLUMN IF EXISTS alias;
