-- +migrate Up
ALTER TABLE live_sessions ADD COLUMN alias TEXT DEFAULT '';

-- +migrate Down
ALTER TABLE live_sessions DROP COLUMN alias;
