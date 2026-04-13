import React from 'react';
import { useTradingStore } from '../store/useTradingStore';
import { SimpleTable } from '../components/ui/SimpleTable';
import { formatTime, formatMaybeNumber, shrink } from '../utils/format';


export function FillsPanel() {
  const fills = useTradingStore((s) => s.fills);
  const orders = useTradingStore((s) => s.orders);

  // 映射订单信息以便查询 symbol 和 side
  const orderMap = new Map(orders.map((o) => [o.id, o]));

  return (
    <div className="panel-content">
      <SimpleTable
        columns={["Order ID", "Symbol", "Side", "Price", "Quantity", "Fee", "Time"]}
        rows={fills.map((f) => {
          const order = orderMap.get(f.orderId);
          return [
            shrink(f.orderId),
            order?.symbol ?? "--",
            order?.side ?? "--",
            formatMaybeNumber(f.price),
            formatMaybeNumber(f.quantity),
            formatMaybeNumber(f.fee),
            formatTime(f.createdAt),
          ];
        })}
        emptyMessage="No recent fills"
      />
    </div>
  );
}

