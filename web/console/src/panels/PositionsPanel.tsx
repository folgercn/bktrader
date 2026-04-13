import React from 'react';
import { useTradingStore } from '../store/useTradingStore';
import { SimpleTable } from '../components/ui/SimpleTable';
import { formatTime, formatMaybeNumber } from '../utils/format';

export function PositionsPanel() {
  const positions = useTradingStore((s) => s.positions);

  return (
    <div className="panel-content">
      <SimpleTable
        columns={["Symbol", "Account", "Side", "Quantity", "Entry Price", "Mark Price", "Updated"]}
        rows={positions.map((p) => [
          p.symbol,
          p.accountId.substring(0, 8),
          p.side,
          formatMaybeNumber(p.quantity),
          formatMaybeNumber(p.entryPrice),
          formatMaybeNumber(p.markPrice),
          formatTime(p.updatedAt),
        ])}
        emptyMessage="No active positions"
      />
    </div>
  );
}

