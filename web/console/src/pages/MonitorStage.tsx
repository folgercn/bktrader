import React, { useMemo } from 'react';
import { useUIStore } from '../store/useUIStore';
import { useTradingStore } from '../store/useTradingStore';
import { SignalMonitorChart } from '../components/charts/SignalMonitorChart';
import { formatMoney, formatSigned, formatMaybeNumber, formatTime, shrink } from '../utils/format';
import { 
  getRecord, 
  mapChartCandlesToSignalBarCandles, 
  derivePrimarySignalBarState, 
  deriveRuntimeMarketSnapshot, 
  deriveSessionMarkers, 
  derivePaperSessionExecutionSummary,
  deriveHighlightedLiveSession
} from '../utils/derivation';

export function MonitorStage() {
  const liveSessions = useTradingStore(s => s.liveSessions);
  const orders = useTradingStore(s => s.orders);
  const fills = useTradingStore(s => s.fills);
  const positions = useTradingStore(s => s.positions);
  const signalRuntimeSessions = useTradingStore(s => s.signalRuntimeSessions);
  const monitorCandles = useTradingStore(s => s.monitorCandles);
  const summaries = useTradingStore(s => s.summaries);
  const runtimePolicy = useTradingStore(s => s.runtimePolicy);

  // Re-calculating derived state locally to keep App clean
  const highlightedLiveSession = useMemo(
    () => deriveHighlightedLiveSession(liveSessions, orders, fills, positions),
    [liveSessions, orders, fills, positions]
  );

  const highlightedLiveRuntime =
    highlightedLiveSession?.session
      ? signalRuntimeSessions.find((item) => item.id === String(highlightedLiveSession.session.state?.signalRuntimeSessionId ?? "")) ??
        signalRuntimeSessions.find(
          (item) =>
            item.accountId === highlightedLiveSession.session.accountId &&
            item.strategyId === highlightedLiveSession.session.strategyId
        ) ??
        null
      : null;

  const highlightedLiveRuntimeState = getRecord(highlightedLiveRuntime?.state);
  const monitorSession = highlightedLiveSession?.session ?? null;
  const monitorMode = highlightedLiveSession?.session ? "LIVE" : "--";
  const monitorExecutionSummary = highlightedLiveSession?.execution ?? derivePaperSessionExecutionSummary(null, orders, fills, positions);
  const monitorRuntimeState = highlightedLiveSession?.session ? highlightedLiveRuntimeState : {};
  const monitorSessionState = getRecord(monitorSession?.state);
  const monitorBars = mapChartCandlesToSignalBarCandles(
    monitorCandles,
    String(monitorSessionState.signalTimeframe ?? "1d")
  );
  const monitorSignalState = derivePrimarySignalBarState(
    getRecord(monitorRuntimeState.signalBarStates),
    getRecord(monitorSessionState.lastStrategyEvaluationSignalBarStates)
  );
  const monitorMarket = deriveRuntimeMarketSnapshot(
    getRecord(monitorRuntimeState.sourceStates),
    getRecord(monitorRuntimeState.lastEventSummary)
  );
  const monitorSummary =
    monitorSession ? summaries.find((item) => item.accountId === monitorSession.accountId) ?? null : null;
  const monitorMarkers = deriveSessionMarkers(monitorSession, orders, fills);

  return (
    <div className="flex flex-col p-4 bg-zinc-950/20">
      <section id="monitor" className="panel panel-market panel-compact monitor-panel-main w-full">
        <div className="panel-header">
          <div>
            <p className="panel-kicker">主监控</p>
            <h3>运行中会话的大周期 K 线与执行状态</h3>
          </div>
          <div className="range-box">
            <span>{monitorMode}</span>
            <span>{monitorBars.length} 根 K 线</span>
            <span>{monitorMarkers.length} 个标记</span>
            <span>{String(monitorSignalState.timeframe ?? "--")}</span>
          </div>
        </div>
        <div className="chart-shell chart-shell-market h-[320px] min-h-[260px]">
          {monitorBars.length > 0 ? (
            <SignalMonitorChart candles={monitorBars} markers={monitorMarkers} />
          ) : (
            <div className="empty-state">当前运行会话还没有交易所大周期 K 线缓存</div>
          )}
        </div>
        <div className="detail-grid detail-grid-compact">
          <div className="detail-item">
            <span>会话模式</span>
            <strong>{monitorMode}</strong>
          </div>
          <div className="detail-item">
            <span>账户净值</span>
            <strong>{formatMoney(monitorSummary?.netEquity)}</strong>
          </div>
          <div className="detail-item">
            <span>未实现盈亏</span>
            <strong>{formatSigned(monitorSummary?.unrealizedPnl)}</strong>
          </div>
          <div className="detail-item">
            <span>持仓方向</span>
            <strong>{String(monitorExecutionSummary.position?.side ?? "FLAT")}</strong>
          </div>
          <div className="detail-item">
            <span>持仓数量</span>
            <strong>{formatMaybeNumber(monitorExecutionSummary.position?.quantity)}</strong>
          </div>
          <div className="detail-item">
            <span>标记价格</span>
            <strong>{formatMaybeNumber(monitorExecutionSummary.position?.markPrice)}</strong>
          </div>
          <div className="detail-item">
            <span>盘口</span>
            <strong>{formatMaybeNumber(monitorMarket.bestBid)} / {formatMaybeNumber(monitorMarket.bestAsk)}</strong>
          </div>
          <div className="detail-item">
            <span>SMA5 / ATR14</span>
            <strong>{formatMaybeNumber(monitorSignalState.sma5)} / {formatMaybeNumber(monitorSignalState.atr14)}</strong>
          </div>
        </div>
        <div className="backtest-notes notes-compact">
          <div className="note-item">
            当前会话：{monitorSession ? shrink(monitorSession.id) : "--"} · 订单 {monitorExecutionSummary.orderCount} · 成交 {monitorExecutionSummary.fillCount}
          </div>
          <div className="note-item">
            最新订单：{String(monitorExecutionSummary.latestOrder?.side ?? "--")} · {String(monitorExecutionSummary.latestOrder?.status ?? "--")} · {formatTime(String(monitorExecutionSummary.latestOrder?.createdAt ?? ""))}
          </div>
          <div className="note-item">
            最新成交：{formatMaybeNumber(monitorExecutionSummary.latestFill?.price)} · 手续费 {formatMaybeNumber(monitorExecutionSummary.latestFill?.fee)} · {formatTime(String(monitorExecutionSummary.latestFill?.createdAt ?? ""))}
          </div>
        </div>
      </section>
    </div>
  );
}
