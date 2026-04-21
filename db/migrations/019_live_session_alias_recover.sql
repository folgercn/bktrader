-- +migrate Up
ALTER TABLE live_sessions ADD COLUMN IF NOT EXISTS alias TEXT DEFAULT '';

-- 只增加列，不要写 DROP 逻辑，因为生产库之前已经执行过 018 并删除了此列，018 名字已在 schema_migrations 中，所以必须靠 019 重新触发。
