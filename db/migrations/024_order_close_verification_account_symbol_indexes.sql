-- Support authoritative flat-position self-heal checks without scanning close verifications.
UPDATE order_close_verifications
SET symbol = UPPER(BTRIM(symbol))
WHERE symbol <> UPPER(BTRIM(symbol));

CREATE INDEX IF NOT EXISTS idx_order_close_verif_account_symbol_event_time
    ON order_close_verifications (account_id, symbol, event_time DESC, recorded_at DESC, id DESC);
