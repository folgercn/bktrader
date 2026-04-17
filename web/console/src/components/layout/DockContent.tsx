import React from 'react';
import { SimpleTable } from '../ui/SimpleTable';
import { StatusPill } from '../ui/StatusPill';
import { ActionButton } from '../ui/ActionButton';
import { formatTime, formatMaybeNumber, shrink } from '../../utils/format';
import { technicalStatusLabel } from '../../utils/derivation';
import { useTradingStore } from '../../store/useTradingStore';
import { useUIStore } from '../../store/useUIStore';

interface DockContentProps {
  dockTab: 'orders' | 'positions' | 'fills' | 'alerts';
  actions: any;
}

export function DockContent({ dockTab, actions }: DockContentProps) {
  const orders = useTradingStore(s => s.orders);
  const fills = useTradingStore(s => s.fills);
  const positions = useTradingStore(s => s.positions);
  const alerts = useTradingStore(s => s.alerts);
  const positionCloseAction = useUIStore(s => s.positionCloseAction);
  const openConfirmDialog = useUIStore(s => s.openConfirmDialog);

  return (
    <div className="h-full relative overflow-hidden">
      {dockTab === 'orders' && (
        <SimpleTable
          columns={["ID", "策略版本", "Symbol", "Side", "Type", "数量", "价格", "状态", "创建时间", "操作"]}
          rows={orders.map((order) => [
            shrink(order.id).replace('order-', ''),
            shrink(String(order.metadata?.strategyVersionId ?? order.metadata?.source ?? "--")),
            order.symbol,
            <StatusPill key={`${order.id}-side`} tone={order.side === "buy" ? "ready" : "neutral"}>{order.side}</StatusPill>,
            order.type,
            formatMaybeNumber(order.quantity),
            formatMaybeNumber(order.price),
            technicalStatusLabel(order.status),
            formatTime(order.createdAt),
            <div key={`${order.id}-actions`} className="inline-actions">
              <ActionButton label="Sync" variant="ghost" onClick={() => actions.syncLiveOrder(order.id)} />
            </div>,
          ])}
          emptyMessage="暂无订单"
        />
      )}
      {dockTab === 'positions' && (
        <SimpleTable
          columns={["ID", "账户", "Symbol", "Side", "仓位大小", "开仓价", "标记价", "更新时间", "操作"]}
          rows={positions.map((pos) => [
            shrink(pos.id).replace('position-', 'pos-'),
            shrink(pos.accountId).replace('account-', 'acc-'),
            pos.symbol,
            <StatusPill key={`${pos.id}-side`} tone={pos.side === "long" ? "ready" : "neutral"}>{pos.side}</StatusPill>,
            formatMaybeNumber(pos.quantity),
            formatMaybeNumber(pos.entryPrice),
            formatMaybeNumber(pos.markPrice),
            formatTime(pos.updatedAt),
            <div key={`${pos.id}-actions`} className="inline-actions">
              <ActionButton 
                label={positionCloseAction === pos.id ? "平仓中..." : "强平"} 
                variant="danger" 
                disabled={positionCloseAction !== null}
                onClick={() => {
                  openConfirmDialog(
                    "强制市价平仓风险确认",
                    "您即将放弃策略托管，使用系统市价单直接平仓。注意：此接管动作可能产生额外滑点，是否确认强平？",
                    () => actions.closePosition(pos.id)
                  );
                }} 
              />
            </div>,
          ])}
          emptyMessage="暂无持仓"
        />
      )}
      {dockTab === 'fills' && (
        <SimpleTable
          columns={["ID", "订单ID", "成交量", "成交价", "费用", "时间"]}
          rows={fills.map((fill) => [
            shrink(fill.id).replace('fill-', ''),
            shrink(fill.orderId).replace('order-', ''),
            formatMaybeNumber(fill.quantity),
            formatMaybeNumber(fill.price),
            formatMaybeNumber(fill.fee),
            formatTime(fill.createdAt),
          ])}
          emptyMessage="暂无成交记录"
        />
      )}
      {dockTab === 'alerts' && (
        <SimpleTable
          columns={["时间", "级别", "模块", "消息"]}
          rows={alerts.map((alert) => [
            formatTime(alert.eventTime ?? ""),
            <StatusPill key={`${alert.id}-level`} tone={alert.level === "critical" ? "blocked" : alert.level === "warning" ? "watch" : "neutral"}>
              {alert.level}
            </StatusPill>,
            alert.title,
            alert.detail,
          ])}
          emptyMessage="暂无告警信息"
        />
      )}
    </div>
  );
}
