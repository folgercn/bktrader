import React from 'react';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '../ui/table';
import { Tooltip, TooltipContent, TooltipTrigger } from '../ui/tooltip';
import { formatTime, formatMaybeNumber, shrink } from '../../utils/format';
import { technicalStatusLabel } from '../../utils/derivation';
import { useTradingStore } from '../../store/useTradingStore';
import { useUIStore } from '../../store/useUIStore';

interface DockContentProps {
  dockTab: 'orders' | 'positions' | 'fills' | 'alerts';
  actions: any;
}

function TruncatedValue({ value, display }: { value: string; display?: string }) {
  const fullValue = String(value ?? "").trim() || "--";
  const shownValue = display ?? shrink(fullValue);

  return (
    <Tooltip>
      <TooltipTrigger className="block max-w-full overflow-hidden text-ellipsis whitespace-nowrap text-left">
        {shownValue}
      </TooltipTrigger>
      <TooltipContent className="max-w-sm rounded-xl border-[var(--bk-border)] bg-[var(--bk-surface-overlay-strong)] px-3 py-2 text-[11px] text-[var(--bk-text-primary)] shadow-xl">
        {fullValue}
      </TooltipContent>
    </Tooltip>
  );
}

function DockBadge({
  tone,
  children,
}: {
  tone: "ready" | "watch" | "blocked" | "neutral";
  children: React.ReactNode;
}) {
  if (tone === "ready") {
    return <Badge variant="success">{children}</Badge>;
  }

  if (tone === "blocked") {
    return (
      <Badge className="border-[var(--bk-status-danger)] bg-[color:color-mix(in_srgb,var(--bk-status-danger)_12%,transparent)] text-[var(--bk-status-danger)]">
        {children}
      </Badge>
    );
  }

  if (tone === "watch") {
    return (
      <Badge className="border-[var(--bk-status-warning)]/35 bg-[color:color-mix(in_srgb,var(--bk-status-warning)_12%,transparent)] text-[var(--bk-status-warning)]">
        {children}
      </Badge>
    );
  }

  return <Badge variant="neutral">{children}</Badge>;
}

function DockActionButton({
  label,
  variant = "ghost",
  disabled,
  onClick,
}: {
  label: string;
  variant?: "ghost" | "danger";
  disabled?: boolean;
  onClick: () => void;
}) {
  return (
    <Button
      type="button"
      size="sm"
      variant={variant === "danger" ? "bento-danger" : "bento-outline"}
      disabled={disabled}
      className="h-8 rounded-xl px-3 text-[11px] font-black"
      onClick={onClick}
    >
      {label}
    </Button>
  );
}

function DockTable({
  columns,
  rows,
  emptyMessage,
}: {
  columns: string[];
  rows: React.ReactNode[][];
  emptyMessage: string;
}) {
  if (rows.length === 0) {
    return (
      <div className="rounded-[24px] border border-dashed border-[var(--bk-border)] bg-[var(--bk-surface-overlay)] px-4 py-8 text-center text-sm italic text-[var(--bk-text-secondary)]">
        {emptyMessage}
      </div>
    );
  }

  return (
    <div className="overflow-hidden rounded-[24px] border border-[var(--bk-border)] bg-[var(--bk-surface-strong)] shadow-inner">
      <Table tone="bento">
        <TableHeader className="bg-[var(--bk-surface-muted)]/40">
          <TableRow className="border-[var(--bk-border-soft)] hover:bg-transparent">
            {columns.map((column) => (
              <TableHead
                key={column}
                className="h-9 px-4 text-[10px] font-black uppercase tracking-wide text-[var(--bk-text-secondary)]"
              >
                {column}
              </TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((row, index) => (
            <TableRow key={`row-${index}`} className="border-[var(--bk-border-soft)]">
              {row.map((cell, cellIndex) => (
                <TableCell
                  key={`cell-${index}-${cellIndex}`}
                  className="px-4 py-3 text-[12px] text-[var(--bk-text-primary)]"
                >
                  {cell}
                </TableCell>
              ))}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
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
        <DockTable
          columns={["ID", "策略版本", "Symbol", "Side", "Type", "数量", "价格", "状态", "创建时间", "操作"]}
          rows={orders.map((order) => [
            <TruncatedValue key={`${order.id}-id`} value={order.id} display={shrink(order.id).replace('order-', '')} />,
            <TruncatedValue key={`${order.id}-strategy`} value={String(order.metadata?.strategyVersionId ?? order.metadata?.source ?? "--")} />,
            order.symbol,
            <DockBadge key={`${order.id}-side`} tone={order.side === "buy" ? "ready" : "neutral"}>{order.side}</DockBadge>,
            order.type,
            formatMaybeNumber(order.quantity),
            formatMaybeNumber(order.price),
            technicalStatusLabel(order.status),
            formatTime(order.createdAt),
            <div key={`${order.id}-actions`} className="inline-actions">
              <DockActionButton label="Sync" variant="ghost" onClick={() => actions.syncLiveOrder(order.id)} />
            </div>,
          ])}
          emptyMessage="暂无订单"
        />
      )}
      {dockTab === 'positions' && (
        <DockTable
          columns={["ID", "账户", "Symbol", "Side", "仓位大小", "开仓价", "标记价", "更新时间", "操作"]}
          rows={positions.map((pos) => [
            <TruncatedValue key={`${pos.id}-id`} value={pos.id} display={shrink(pos.id).replace('position-', 'pos-')} />,
            <TruncatedValue key={`${pos.id}-account`} value={pos.accountId} display={shrink(pos.accountId).replace('account-', 'acc-')} />,
            pos.symbol,
            <DockBadge key={`${pos.id}-side`} tone={pos.side === "long" ? "ready" : "neutral"}>{pos.side}</DockBadge>,
            formatMaybeNumber(pos.quantity),
            formatMaybeNumber(pos.entryPrice),
            formatMaybeNumber(pos.markPrice),
            formatTime(pos.updatedAt),
            <div key={`${pos.id}-actions`} className="inline-actions">
              <DockActionButton 
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
        <DockTable
          columns={["ID", "订单ID", "成交量", "成交价", "费用", "时间"]}
          rows={fills.map((fill) => [
            <TruncatedValue key={`${fill.id}-id`} value={fill.id} display={shrink(fill.id).replace('fill-', '')} />,
            <TruncatedValue key={`${fill.id}-order`} value={fill.orderId} display={shrink(fill.orderId).replace('order-', '')} />,
            formatMaybeNumber(fill.quantity),
            formatMaybeNumber(fill.price),
            formatMaybeNumber(fill.fee),
            formatTime(fill.createdAt),
          ])}
          emptyMessage="暂无成交记录"
        />
      )}
      {dockTab === 'alerts' && (
        <DockTable
          columns={["时间", "级别", "模块", "消息"]}
          rows={alerts.map((alert) => [
            formatTime(alert.eventTime ?? ""),
            <DockBadge key={`${alert.id}-level`} tone={alert.level === "critical" ? "blocked" : alert.level === "warning" ? "watch" : "neutral"}>
              {alert.level}
            </DockBadge>,
            alert.title,
            alert.detail,
          ])}
          emptyMessage="暂无告警信息"
        />
      )}
    </div>
  );
}
