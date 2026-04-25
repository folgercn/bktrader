create index if not exists idx_order_execution_events_telegram_trade_candidates
    on order_execution_events (event_time asc, recorded_at asc, id asc)
    where event_type = 'filled'
      and failed = false
      and coalesce(error, '') = '';

create index if not exists idx_notification_deliveries_telegram_trade_event
    on notification_deliveries ((metadata->>'eventId'))
    where channel = 'telegram'
      and metadata->>'kind' = 'trade-event';
