import React, { useState, useMemo } from 'react';
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
import { Input } from '../ui/input';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '../ui/select';
import { formatTime, formatMaybeNumber, shrink, formatSigned } from '../../utils/format';
import { technicalStatusLabel } from '../../utils/derivation';
import { useTradingStore } from '../../store/useTradingStore';
import { useUIStore } from '../../store/useUIStore';
import { useLiveTradePairs } from '../../hooks/useLiveTradePairs';
import { useOrdersPageQuery } from '../../hooks/useOrdersPageQuery';
import { useFillsPageQuery } from '../../hooks/useFillsPageQuery';
import { ShieldCheck, Loader2, ChevronLeft, ChevronRight, ArrowRightLeft, Activity, CheckCircle2, AlertCircle } from 'lucide-react';
import { 
  Dialog, 
  DialogContent, 
  DialogHeader, 
  DialogTitle, 
  DialogDescription, 
  DialogFooter,
  DialogClose
} from '../ui/dialog';
import { ManualTradeReviewDialog } from '../live/ManualTradeReviewDialog';
import { toast } from 'sonner';

import { cn } from '../../lib/utils';

interface DockContentProps {
  dockTab: 'pairs' | 'orders' | 'positions' | 'fills' | 'alerts';
  actions: any;
  sessionId?: string | null;
}

function tradePairStatusLabel(status: string) {
  return String(status).toLowerCase() === 'open' ? '持仓中' : '已平仓';
}

function tradePairVerdictLabel(verdict: string) {
  switch (String(verdict).toLowerCase()) {
    case 'normal':
      return '正常退出';
    case 'recovery-close':
      return 'Recovery 平仓';
    case 'orphan-exit':
      return '孤儿退出';
    case 'mismatch':
      return '需复核';
    default:
      return '进行中';
  }
}

function tradePairVerdictTone(verdict: string) {
  switch (String(verdict).toLowerCase()) {
    case 'normal':
      return 'text-[var(--bk-status-success)]';
    case 'mismatch':
    case 'orphan-exit':
      return 'text-[var(--bk-status-danger)]';
    case 'recovery-close':
      return 'text-[var(--bk-status-warning)]';
    default:
      return 'text-[var(--bk-text-muted)]';
  }
}

function TruncatedValue({ value, display, noShrink }: { value: string; display?: string; noShrink?: boolean }) {
  const fullValue = String(value ?? "").trim() || "--";
  const shownValue = display ?? (noShrink ? fullValue : shrink(fullValue));

  return (
    <Tooltip>
      <TooltipTrigger className="block max-w-full overflow-hidden text-ellipsis whitespace-nowrap text-left hover:text-[var(--bk-text-primary)] transition-colors">
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

function Pagination({ 
  currentPage, 
  totalItems, 
  pageSize, 
  onPageChange, 
  onPageSizeChange 
}: { 
  currentPage: number; 
  totalItems: number; 
  pageSize: number; 
  onPageChange: (page: number) => void;
  onPageSizeChange: (size: number) => void;
}) {
  const totalPages = Math.ceil(totalItems / pageSize) || 1;
  const [jumpPage, setJumpPage] = useState("");

  const handleJump = () => {
    const p = parseInt(jumpPage);
    if (!isNaN(p) && p >= 1 && p <= totalPages) {
      onPageChange(p);
      setJumpPage("");
    }
  };

  return (
    <div className="flex items-center justify-between px-6 py-3 border-t border-[var(--bk-border-soft)] bg-[var(--bk-surface-faint)]/50">
      <div className="flex items-center gap-4">
        <div className="flex items-center gap-2">
          <span className="text-[10px] font-black uppercase text-[var(--bk-text-muted)] opacity-60">每页</span>
          <Select value={String(pageSize)} onValueChange={(val) => val && onPageSizeChange(parseInt(val))}>
            <SelectTrigger tone="bento" size="sm" className="h-7 w-16 text-[10px] font-black">
              <SelectValue />
            </SelectTrigger>
            <SelectContent tone="bento" className="min-w-[80px]">
              {[5, 10, 20, 50, 100].map(size => (
                <SelectItem key={size} value={String(size)}>{size}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <span className="text-[10px] font-bold text-[var(--bk-text-muted)] opacity-60">
          共 {totalItems} 条 · {totalPages} 页
        </span>
      </div>

      <div className="flex items-center gap-3">
        <div className="flex items-center gap-1.5 mr-2">
          <Input 
            className="h-7 w-12 rounded-lg border-[var(--bk-border)] bg-[var(--bk-surface)] px-1 text-center text-[10px] font-black"
            placeholder="页码"
            value={jumpPage}
            onChange={(e) => setJumpPage(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && handleJump()}
          />
          <Button 
            variant="bento-outline" 
            size="sm" 
            className="h-7 rounded-lg px-2 text-[10px] font-black"
            onClick={handleJump}
          >
            跳转
          </Button>
        </div>

        <div className="flex items-center gap-1">
          <Button 
            variant="bento-outline" 
            size="sm" 
            className="h-7 w-7 rounded-lg p-0" 
            disabled={currentPage <= 1}
            onClick={() => onPageChange(currentPage - 1)}
          >
            <ChevronLeft className="size-3.5" />
          </Button>
          <div className="flex h-7 min-w-[28px] items-center justify-center rounded-lg border border-[var(--bk-border)] bg-[var(--bk-surface-overlay)] px-2 text-[10px] font-black text-[var(--bk-text-primary)] shadow-inner">
            {currentPage}
          </div>
          <Button 
            variant="bento-outline" 
            size="sm" 
            className="h-7 w-7 rounded-lg p-0" 
            disabled={currentPage >= totalPages}
            onClick={() => onPageChange(currentPage + 1)}
          >
            <ChevronRight className="size-3.5" />
          </Button>
        </div>
      </div>
    </div>
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
                className={cn(
                  "h-9 px-4 text-[10px] font-black uppercase tracking-wide text-[var(--bk-text-secondary)]",
                  column === "ID" && "min-w-[150px]",
                  column === "策略版本" && "min-w-[280px]",
                  column === "创建时间" && "min-w-[160px]",
                  column === "操作" && "text-right"
                )}
              >
                {column}
              </TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((row, index) => (
            <TableRow key={`row-${index}`} className="border-[var(--bk-border-soft)]">
              {row.map((cell, cellIndex) => {
                const columnName = columns[cellIndex];
                return (
                  <TableCell
                    key={`cell-${index}-${cellIndex}`}
                    className={cn(
                      "px-4 py-3 text-[12px] text-[var(--bk-text-primary)]",
                      columnName === "ID" && "min-w-[150px] font-mono",
                      columnName === "策略版本" && "min-w-[280px] font-mono",
                      columnName === "创建时间" && "min-w-[160px] font-mono",
                      columnName === "操作" && "text-right"
                    )}
                  >
                    {cell}
                  </TableCell>
                );
              })}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}

export function DockContent({ dockTab, actions, sessionId }: DockContentProps) {
  const orders = useTradingStore(s => s.orders);
  const fills = useTradingStore(s => s.fills);
  const positions = useTradingStore(s => s.positions);
  const alerts = useTradingStore(s => s.alerts);
  const positionCloseAction = useUIStore(s => s.positionCloseAction);
  const liveSyncAction = useUIStore(s => s.liveSyncAction);
  const openConfirmDialog = useUIStore(s => s.openConfirmDialog);
  const { pairs, loading: pairsLoading, error: pairsError, refetch: refetchPairs } = useLiveTradePairs(
    dockTab === 'pairs' ? (sessionId ?? null) : null, 
    200
  );

  const [selectedPairForReview, setSelectedPairForReview] = useState<any | null>(null);

  // Pagination & Sorting State
  const [pages, setPages] = useState({ pairs: 1, positions: 1, alerts: 1 });
  const [pageSize, setPageSize] = useState(5);

  const ordersPageQuery = useOrdersPageQuery(pageSize, dockTab === 'orders');
  const fillsPageQuery = useFillsPageQuery(pageSize, dockTab === 'fills');

  // Reset page when tab or pageSize changes
  const handlePageChange = (page: number) => {
    if (dockTab === 'orders') {
      ordersPageQuery.setCurrentPage(page);
    } else if (dockTab === 'fills') {
      fillsPageQuery.setCurrentPage(page);
    } else {
      setPages(prev => ({ ...prev, [dockTab]: page }));
    }
  };

  const handlePageSizeChange = (size: number) => {
    setPageSize(size);
    setPages({ pairs: 1, positions: 1, alerts: 1 });
    if (dockTab === 'orders' || dockTab === 'fills') {
      ordersPageQuery.setCurrentPage(1);
      fillsPageQuery.setCurrentPage(1);
    }
  };

  const sortedPositions = useMemo(() => [...positions].sort((a, b) => Date.parse(b.updatedAt) - Date.parse(a.updatedAt)), [positions]);
  const sortedAlerts = useMemo(() => [...alerts].sort((a, b) => Date.parse(b.eventTime ?? "") - Date.parse(a.eventTime ?? "")), [alerts]);
  const sortedPairs = useMemo(() => [...pairs].sort((a, b) => Date.parse(b.entryAt) - Date.parse(a.entryAt)), [pairs]);

  const pagedOrders = ordersPageQuery.orders;
  const pagedFills = fillsPageQuery.fills;
  const pagedPositions = useMemo(() => sortedPositions.slice((pages.positions - 1) * pageSize, pages.positions * pageSize), [sortedPositions, pages.positions, pageSize]);
  const pagedAlerts = useMemo(() => sortedAlerts.slice((pages.alerts - 1) * pageSize, pages.alerts * pageSize), [sortedAlerts, pages.alerts, pageSize]);
  const pagedPairs = useMemo(() => sortedPairs.slice((pages.pairs - 1) * pageSize, pages.pairs * pageSize), [sortedPairs, pages.pairs, pageSize]);

  const orderById = new Map(orders.map((order) => [order.id, order] as const));
  const duplicateFallbackFillCounts = fills
    .filter((fill) => !(fill.exchangeTradeId ?? "").trim())
    .reduce((counts, fill) => {
      const key = [
        fill.orderId,
        String(fill.price),
        String(fill.quantity),
        String(fill.fee),
        fill.exchangeTradeTime ?? "",
      ].join("|");
      counts.set(key, (counts.get(key) ?? 0) + 1);
      return counts;
    }, new Map<string, number>());

  return (
    <div className="h-full relative flex flex-col overflow-hidden">
      <div className="flex-1 overflow-y-auto">
        {dockTab === 'pairs' && (
          <div className="p-0">
            {pairsLoading ? (
              <div className="flex items-center justify-center gap-3 py-20 text-[var(--bk-text-muted)]">
                <Activity size={16} className="animate-spin" />
                <span className="text-[11px] font-black uppercase tracking-widest">聚合追溯中...</span>
              </div>
            ) : pairsError ? (
              <div className="p-8 text-center text-rose-500 text-[11px] font-black">{pairsError}</div>
            ) : (
              <DockTable
                columns={["状态", "方向/Symbol", "开仓细节", "平仓细节", "数量/成交", "PNL统计"]}
                rows={pagedPairs.map((pair) => {
                  const netPositive = Number(pair.netPnl ?? 0) >= 0;
                  const quantity = String(pair.status).toLowerCase() === 'open' ? pair.openQuantity : pair.entryQuantity;
                  return [
                    <div key={`${pair.id}-status`} className="space-y-1">
                      <Badge variant={String(pair.status).toLowerCase() === 'open' ? 'neutral' : 'success'}>
                        {tradePairStatusLabel(pair.status)}
                      </Badge>
                      <div 
                        className={cn(
                          'text-[10px] font-black', 
                          tradePairVerdictTone(pair.exitVerdict),
                          (String(pair.exitVerdict).toLowerCase() === 'mismatch' || String(pair.exitVerdict).toLowerCase() === 'orphan-exit') && "cursor-pointer hover:underline flex items-center gap-1"
                        )}
                        onClick={() => {
                          const v = String(pair.exitVerdict).toLowerCase();
                          if (v === 'mismatch' || v === 'orphan-exit') {
                            setSelectedPairForReview(pair);
                          }
                        }}
                      >
                        {tradePairVerdictLabel(pair.exitVerdict)}
                        {(String(pair.exitVerdict).toLowerCase() === 'mismatch' || String(pair.exitVerdict).toLowerCase() === 'orphan-exit') && <AlertCircle size={10} />}
                      </div>
                    </div>,
                    <div key={`${pair.id}-side`} className="space-y-1">
                      <div className="text-[12px] font-black text-[var(--bk-text-primary)]">{pair.side}</div>
                      <div className="text-[10px] text-[var(--bk-text-muted)] font-mono">{pair.symbol}</div>
                    </div>,
                    <div key={`${pair.id}-entry`} className="space-y-0.5">
                      <div className="font-mono text-[12px] font-black text-[var(--bk-text-primary)]">
                        {formatMaybeNumber(pair.entryAvgPrice)}
                      </div>
                      <div className="text-[9px] text-[var(--bk-text-muted)] opacity-60">{formatTime(pair.entryAt)}</div>
                      <div className="text-[9px] font-black uppercase text-[var(--bk-text-muted)] truncate max-w-[120px]">
                        {pair.entryReason || '--'}
                      </div>
                    </div>,
                    String(pair.status).toLowerCase() === 'closed' ? (
                      <div key={`${pair.id}-exit`} className="space-y-0.5">
                        <div className="font-mono text-[12px] font-black text-[var(--bk-text-primary)]">
                          {formatMaybeNumber(pair.exitAvgPrice)}
                        </div>
                        <div className="text-[9px] text-[var(--bk-text-muted)] opacity-60">
                          {pair.exitAt ? formatTime(pair.exitAt) : '--'}
                        </div>
                        <div className="text-[9px] font-black uppercase text-[var(--bk-text-muted)] truncate max-w-[120px]">
                          {pair.exitReason || '--'}
                        </div>
                      </div>
                    ) : (
                      <div key={`${pair.id}-exit-pending`} className="space-y-1">
                        <div className="text-[11px] font-black text-[var(--bk-status-success)]">运行中</div>
                        <div className="text-[9px] text-[var(--bk-text-muted)] font-mono">
                          未实现 {formatSigned(pair.unrealizedPnl)}
                        </div>
                      </div>
                    ),
                    <div key={`${pair.id}-qty`} className="space-y-0.5">
                      <div className="font-mono text-[12px] font-black text-[var(--bk-text-primary)]">
                        {formatMaybeNumber(quantity)}
                      </div>
                      <div className="text-[9px] text-[var(--bk-text-muted)] font-black">
                        {pair.entryFillCount} IN / {pair.exitFillCount} OUT
                      </div>
                    </div>,
                    <div key={`${pair.id}-pnl`} className="space-y-0.5">
                      <div className={cn('font-mono text-[13px] font-black', netPositive ? 'text-[var(--bk-status-success)]' : 'text-[var(--bk-status-danger)]')}>
                        {formatSigned(pair.netPnl)}
                      </div>
                      <div className="text-[9px] text-[var(--bk-text-muted)] flex items-center gap-2">
                        <span>费 {formatMaybeNumber(pair.fees, 5)}</span>
                      </div>
                    </div>,
                  ];
                })}
                emptyMessage="当前焦点会话无追溯记录"
              />
            )}
          </div>
        )}
        {dockTab === 'orders' && (
          <DockTable
            columns={["ID", "策略版本", "Symbol", "Side", "Type", "数量", "价格", "交易所订单ID", "状态", "创建时间", "操作"]}
            rows={pagedOrders.map((order) => {
              const exchangeId = String(order.metadata?.exchangeOrderId ?? "--");
              const isReconciled = !!(order.metadata?.orderLifecycle as any)?.synced;
              const isOrphan = order.status === "ACCEPTED" && (order.metadata?.orderLifecycle as any)?.reconciliationState === "orphaned";

              return [
                <TruncatedValue key={`${order.id}-id`} value={order.id} display={order.id.replace('order-', '')} />,
                <TruncatedValue key={`${order.id}-strategy`} value={String(order.metadata?.strategyVersionId ?? order.metadata?.source ?? "--")} noShrink />,
                order.symbol,
                <DockBadge key={`${order.id}-side`} tone={order.side === "buy" ? "ready" : "neutral"}>{order.side}</DockBadge>,
                order.type,
                formatMaybeNumber(order.quantity),
                formatMaybeNumber(order.price),
                <div key={`${order.id}-exid`} className="flex items-center gap-1.5 min-w-[120px]">
                  <TruncatedValue value={exchangeId} />
                  {isReconciled && (
                     <div className="flex size-3.5 items-center justify-center rounded-full bg-[var(--bk-status-success-soft)] text-[var(--bk-status-success)]">
                        <ShieldCheck className="size-2.5" />
                     </div>
                  )}
                </div>,
                <div key={`${order.id}-status`} className="flex items-center gap-2">
                  <DockBadge tone={isOrphan ? "blocked" : (isReconciled ? "ready" : "watch")}>
                    {technicalStatusLabel(order.status)}
                  </DockBadge>
                </div>,
                formatTime(order.createdAt),
                <div key={`${order.id}-actions`} className="inline-actions relative">
                  <DockActionButton 
                    label={liveSyncAction === order.id ? "Syncing..." : "Sync"} 
                    disabled={liveSyncAction !== null}
                    variant="ghost" 
                    onClick={() => actions.syncLiveOrder(order.id)} 
                  />
                  {liveSyncAction === order.id && (
                    <Loader2 className="absolute -right-2 top-1/2 size-3.5 -translate-y-1/2 animate-spin text-[var(--bk-text-muted)]" />
                  )}
                </div>,
              ];
            })}
            emptyMessage="暂无订单"
          />
        )}
        {dockTab === 'positions' && (
          <DockTable
            columns={["ID", "账户", "Symbol", "Side", "仓位大小", "开仓价", "标记价", "更新时间", "操作"]}
            rows={pagedPositions.map((pos) => [
              <TruncatedValue key={`${pos.id}-id`} value={pos.id} display={pos.id.replace('position-', 'pos-')} />,
              <TruncatedValue key={`${pos.id}-account`} value={pos.accountId} display={pos.accountId.replace('account-', 'acc-')} />,
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
            columns={["ID", "策略版本", "Symbol", "侧向", "价格", "数量", "手续费", "交易所成交ID", "交易所成交时间", "本地入库时间", "同步提示"]}
            rows={pagedFills.map((fill) => {
              const order = orderById.get(fill.orderId);
              const duplicateKey = [
                fill.orderId,
                String(fill.price),
                String(fill.quantity),
                String(fill.fee),
                fill.exchangeTradeTime ?? "",
              ].join("|");
              const suspiciousDuplicate = !(fill.exchangeTradeId ?? "").trim() && (duplicateFallbackFillCounts.get(duplicateKey) ?? 0) > 1;

              return [
                <TruncatedValue key={`${fill.id}-id`} value={fill.id} display={fill.id.replace('fill-', '')} />,
                String(order?.metadata?.strategyVersionId ?? fill.strategyVersion ?? "--"),
                order?.symbol ?? fill.symbol ?? "--",
                order?.side ?? fill.side ?? "--",
                formatMaybeNumber(fill.price),
                formatMaybeNumber(fill.quantity),
                formatMaybeNumber(fill.fee),
                <TruncatedValue key={`${fill.id}-exid`} value={fill.exchangeTradeId ?? "--"} />,
                formatTime(fill.exchangeTradeTime ?? ""),
                formatTime(fill.createdAt),
                suspiciousDuplicate ? (
                  <DockBadge key={`${fill.id}-dup`} tone="watch">疑似重复</DockBadge>
                ) : fill.exchangeTradeId ? (
                  <DockBadge key={`${fill.id}-ok`} tone="ready">已同步</DockBadge>
                ) : (
                  <span key={`${fill.id}-pending`} className="text-[11px] text-[var(--bk-text-muted)]">
                    等待同步
                  </span>
                ),
              ];
            })}
            emptyMessage="暂无成交记录"
          />
        )}
        {dockTab === 'alerts' && (
          <DockTable
            columns={["时间", "级别", "模块", "消息"]}
            rows={pagedAlerts.map((alert) => [
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

      <Pagination 
        currentPage={dockTab === 'orders' ? ordersPageQuery.currentPage : dockTab === 'fills' ? fillsPageQuery.currentPage : pages[dockTab]}
        totalItems={dockTab === 'orders' ? ordersPageQuery.totalCount : dockTab === 'fills' ? fillsPageQuery.totalCount : { pairs: pairs.length, positions: positions.length, alerts: alerts.length }[dockTab]}
        pageSize={pageSize}
        onPageChange={handlePageChange}
        onPageSizeChange={handlePageSizeChange}
      />

      <ManualTradeReviewDialog 
        pair={selectedPairForReview}
        sessionId={sessionId}
        onClose={() => setSelectedPairForReview(null)}
        onSuccess={() => refetchPairs?.()}
      />
    </div>
  );
}
