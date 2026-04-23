-- Create order_close_verifications table
CREATE TABLE IF NOT EXISTS order_close_verifications (
    id TEXT PRIMARY KEY,
    live_session_id TEXT NOT NULL,
    order_id TEXT NOT NULL,
    decision_event_id TEXT,
    account_id TEXT NOT NULL,
    strategy_id TEXT NOT NULL,
    symbol TEXT NOT NULL,
    verified_closed BOOLEAN NOT NULL,
    remaining_position_qty DOUBLE PRECISION NOT NULL,
    verification_source TEXT NOT NULL,
    event_time TIMESTAMPTZ NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL,
    metadata JSONB
);

CREATE INDEX IF NOT EXISTS idx_order_close_verif_order_event_time ON order_close_verifications (order_id, event_time DESC);
CREATE INDEX IF NOT EXISTS idx_order_close_verif_session_event_time ON order_close_verifications (live_session_id, event_time DESC);
CREATE INDEX IF NOT EXISTS idx_order_close_verif_sess_ord_time ON order_close_verifications (live_session_id, order_id, event_time DESC);
