import React from 'react';
import { MetricCard } from '../ui/MetricCard';
import { useTradingStore } from '../../store/useTradingStore';
import { useMemo } from 'react';
import { deriveHighlightedLiveSession } from '../../utils/derivation';

export function HeaderMetrics() {
  const accounts = useTradingStore(s => s.accounts);
  const liveSessions = useTradingStore(s => s.liveSessions);
  const orders = useTradingStore(s => s.orders);
  const fills = useTradingStore(s => s.fills);
  const positions = useTradingStore(s => s.positions);
  const strategies = useTradingStore(s => s.strategies);
  const signalRuntimeSessions = useTradingStore(s => s.signalRuntimeSessions);
  const signalCatalog = useTradingStore(s => s.signalCatalog);

  const highlightedLiveSession = useMemo(
    () => deriveHighlightedLiveSession(liveSessions, orders, fills, positions),
    [liveSessions, orders, fills, positions]
  );
  
  const strategyIds = useMemo(() => new Set(strategies.map((item) => item.id)), [strategies]);
  const validLiveSessions = useMemo(
    () => liveSessions.filter((item) => strategyIds.has(item.strategyId)),
    [liveSessions, strategyIds]
  );

  const monitorMode = highlightedLiveSession?.session ? "LIVE" : "--";

  return (
    <div className="flex space-x-2">
      <MetricCard label="账户" value={monitorMode} />
      <MetricCard label="策略" value={String(highlightedLiveSession?.session?.strategyId ?? "--")} />
      <MetricCard label="实盘会话" value={String(validLiveSessions.length)} />
      <MetricCard label="运行时会话" value={String(signalRuntimeSessions.length)} />
      <MetricCard label="可用信号源" value={String(signalCatalog?.sources?.length ?? 0)} />
      <MetricCard label="实盘状态" value={highlightedLiveSession?.health.status ?? "--"} tone={highlightedLiveSession?.health.status === "ready" ? "accent" : undefined} />
    </div>
  );
}
