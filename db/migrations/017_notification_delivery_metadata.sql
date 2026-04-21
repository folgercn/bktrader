-- 为通知发送记录表增加 metadata 字段用于保存告警标题等快照信息
ALTER TABLE notification_deliveries ADD COLUMN IF NOT EXISTS metadata JSONB NOT NULL DEFAULT '{}';
