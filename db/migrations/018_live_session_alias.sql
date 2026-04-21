-- +migrate Up (保持此注释以防止某些环境下的解析问题，但确保后面没有 Down 逻辑)
ALTER TABLE live_sessions ADD COLUMN IF NOT EXISTS alias TEXT DEFAULT '';
