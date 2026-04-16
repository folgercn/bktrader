import React, { useMemo, useState } from 'react';
import { useUIStore } from '../store/useUIStore';
import { useTradingStore } from '../store/useTradingStore';
import { ActionButton } from '../components/ui/ActionButton';
import { SimpleTable } from '../components/ui/SimpleTable';
import { StatusPill } from '../components/ui/StatusPill';
import { SignalMonitorChart } from '../components/charts/SignalMonitorChart';
import { formatMoney, formatSigned, formatMaybeNumber, formatTime, shrink } from '../utils/format';
import { 
  getRecord, 
  getList,
  mapChartCandlesToSignalBarCandles, 
  derivePrimarySignalBarState, 
  deriveRuntimeMarketSnapshot, 
  deriveSessionMarkers, 
  derivePaperSessionExecutionSummary,
  deriveHighlightedLiveSession,
  deriveLiveDispatchPreview,
  deriveLiveSessionFlow,
  deriveRuntimeReadiness,
  deriveRuntimeSourceSummary,
  buildTimelineNotes,
  boolLabel,
  liveSessionHealthTone,
  runtimePolicyValueLabel,
  technicalStatusLabel
} from '../utils/derivation';

type MonitorStageProps = {
  syncLiveOrder: (id: string) => void;
  dockTab: 'orders' | 'positions' | 'fills' | 'alerts';
  onDockTabChange: (tab: 'orders' | 'positions' | 'fills' | 'alerts') => void;
  dockContent: React.ReactNode;
};

export function MonitorStage({ syncLiveOrder, dockTab, onDockTabChange, dockContent }: MonitorStageProps) {
  const liveSessions = useTradingStore(s => s.liveSessions);
  const orders = useTradingStore(s => s.orders);
  const fills = useTradingStore(s => s.fills);
  const positions = useTradingStore(s => s.positions);
  const signalRuntimeSessions = useTradingStore(s => s.signalRuntimeSessions);
  const monitorCandles = useTradingStore(s => s.monitorCandles);
  const summaries = useTradingStore(s => s.summaries);
  const runtimePolicy = useTradingStore(s => s.runtimePolicy);
  const monitorHealth = useTradingStore(s => s.monitorHealth);
  const accounts = useTradingStore(s => s.accounts);
  const strategySignalBindingMap = useTradingStore(s => s.strategySignalBindingMap);
  const monitorResolution = useUIStore(s => s.monitorResolution);
  const setMonitorResolution = useUIStore(s => s.setMonitorResolution);
  const liveSyncAction = useUIStore(s => s.liveSyncAction);

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
  const monitorFlow = useMemo(
    () =>
      highlightedLiveSession
        ? deriveLiveSessionFlow(highlightedLiveSession.session, highlightedLiveSession.execution)
        : [],
    [highlightedLiveSession]
  );
  const primaryLiveAccount = monitorSession ? accounts.find((item) => item.id === monitorSession.accountId) ?? null : null;
  const primaryLiveRuntimeSessions = monitorSession
    ? signalRuntimeSessions.filter((item) => item.accountId === monitorSession.accountId)
    : [];
  const monitorRuntimeReadiness = deriveRuntimeReadiness(
    highlightedLiveRuntimeState,
    deriveRuntimeSourceSummary(getRecord(highlightedLiveRuntimeState.sourceStates), runtimePolicy),
    {
      requireTick: true,
      requireOrderBook: false,
    }
  );
  const monitorIntent = getRecord(monitorSession?.state?.lastStrategyIntent);
  const monitorSignalBarDecision = getRecord(monitorSession?.state?.lastStrategyEvaluationSignalBarDecision);
  const monitorTimeline = getList(monitorSession?.state?.timeline);
  const monitorDispatchPreview = deriveLiveDispatchPreview(
    monitorSession,
    primaryLiveAccount,
    monitorSession ? strategySignalBindingMap[monitorSession.strategyId] ?? [] : [],
    primaryLiveRuntimeSessions,
    highlightedLiveRuntime,
    monitorRuntimeReadiness,
    monitorIntent
  );
  const syncableLiveOrders = orders.filter((item) => item.metadata?.executionMode === "live" && item.status === "ACCEPTED");
  const [expandedLiveSections, setExpandedLiveSections] = useState<Record<string, boolean>>({});
  const platformRuntimePolicy = monitorHealth?.runtimePolicy ?? runtimePolicy;

  const monitorSummaryItems = monitorSession ? [
    { label: "运行环境", value: `${String(monitorSession.state?.signalRuntimeStatus ?? "--")} · ${formatTime(String(monitorSession.state?.lastSignalRuntimeEventAt ?? ""))}` },
    { label: "就绪预检", value: `${monitorRuntimeReadiness.status} · ${monitorRuntimeReadiness.reason}` },
    { label: "信号意图", value: `${String(monitorIntent.action ?? "无")} · ${String(monitorIntent.side ?? "--")} · ${formatMaybeNumber(monitorIntent.priceHint)}` },
    { label: "指令分发", value: `${String(monitorSession.state?.dispatchMode ?? "--")} · 冷却 ${String(monitorSession.state?.dispatchCooldownSeconds ?? "--")}s` },
    { label: "恢复状态", value: `${String(monitorSession.state?.positionRecoveryStatus ?? "--")} / ${String(monitorSession.state?.protectionRecoveryStatus ?? "--")}` },
    { label: "执行汇总", value: `订单 ${monitorExecutionSummary.orderCount} · 成交 ${monitorExecutionSummary.fillCount} · ${String(monitorExecutionSummary.latestOrder?.status ?? "--")}` },
  ] : [];

  const monitorSections = monitorSession ? [
    {
      title: "运行与行情",
      items: [
        { label: "行情数据", value: `${formatMaybeNumber(monitorMarket.tradePrice)} · ${formatMaybeNumber(monitorMarket.bestBid)} / ${formatMaybeNumber(monitorMarket.bestAsk)}` },
        { label: "数据同步", value: `${String(monitorSession.state?.lastSyncedOrderStatus ?? "--")} · ${formatTime(String(monitorSession.state?.lastSyncedAt ?? ""))} · 错误 ${String(monitorSession.state?.lastSyncError ?? "--")}` },
        { label: "自动分发", value: `最后触发 ${formatTime(String(monitorSession.state?.lastDispatchedAt ?? ""))} · 最后错误 ${String(monitorSession.state?.lastAutoDispatchError ?? "--")}` },
        { label: "时间线", value: buildTimelineNotes(monitorTimeline).slice(0, 2).join(" · ") || "--" },
      ],
    },
    {
      title: "信号与意图",
      items: [
        { label: "意图预览", value: `数量 ${formatMaybeNumber(monitorIntent.quantity)} · 报价源 ${String(monitorIntent.priceSource ?? "--")} · 信号种类 ${String(monitorIntent.signalKind ?? "--")}` },
        { label: "意图上下文", value: `价差 ${formatMaybeNumber(monitorIntent.spreadBps)} bps · 偏置 ${String(monitorIntent.liquidityBias ?? "--")} · ma20 ${formatMaybeNumber(monitorIntent.ma20)} · atr14 ${formatMaybeNumber(monitorIntent.atr14)}` },
        { label: "信号过滤", value: `周期 ${String(monitorSignalBarDecision.timeframe ?? "--")} · sma5 ${formatMaybeNumber(monitorSignalBarDecision.sma5)} · 多头 ${boolLabel(monitorSignalBarDecision.longEarlyReversalReady)} · 空头 ${boolLabel(monitorSignalBarDecision.shortEarlyReversalReady)}` },
        { label: "信号备注", value: String(monitorSignalBarDecision.reason ?? "--") },
      ],
    },
    {
      title: "执行与分发",
      items: [
        { label: "执行配置", value: `${String(getRecord(monitorSession.state?.lastExecutionProfile).executionProfile ?? "--")} · ${String(getRecord(monitorSession.state?.lastExecutionProfile).orderType ?? "--")} · TIF ${String(getRecord(monitorSession.state?.lastExecutionProfile).timeInForce ?? "--")} · 只减仓 ${boolLabel(getRecord(monitorSession.state?.lastExecutionProfile).reduceOnly)}` },
        { label: "执行遥测", value: `${String(getRecord(monitorSession.state?.lastExecutionTelemetry).decision ?? "--")} · 价差 ${formatMaybeNumber(getRecord(getRecord(monitorSession.state?.lastExecutionTelemetry).book).spreadBps)} bps · 盘口不平衡 ${formatMaybeNumber(getRecord(getRecord(monitorSession.state?.lastExecutionTelemetry).book).bookImbalance)}` },
        { label: "分发状态", value: `${String(getRecord(monitorSession.state?.lastExecutionDispatch).status ?? "--")} · ${String(getRecord(monitorSession.state?.lastExecutionDispatch).executionMode ?? "--")} · 备选方案 ${boolLabel(getRecord(monitorSession.state?.lastExecutionDispatch).fallback)}` },
        { label: "成交分析", value: `预期价格 ${formatMaybeNumber(getRecord(monitorSession.state?.lastExecutionDispatch).expectedPrice)} · 滑点偏移 ${formatMaybeNumber(getRecord(monitorSession.state?.lastExecutionDispatch).priceDriftBps)} bps` },
        { label: "执行统计", value: `方案数 ${String(getRecord(monitorSession.state?.executionEventStats).proposalCount ?? "--")} · Maker ${String(getRecord(monitorSession.state?.executionEventStats).makerRestingDecisionCount ?? "--")} · 备选 ${String(getRecord(monitorSession.state?.executionEventStats).fallbackDispatchCount ?? "--")} · 平均偏移 ${formatMaybeNumber(getRecord(monitorSession.state?.executionEventStats).avgPriceDriftBps)} bps` },
        { label: "分发预览", value: `${monitorDispatchPreview.reason} · ${monitorDispatchPreview.detail}` },
        { label: "最终指令", value: `${String(monitorDispatchPreview.payload.side ?? "--")} ${formatMaybeNumber(monitorDispatchPreview.payload.quantity)} ${String(monitorDispatchPreview.payload.symbol ?? "--")} · ${String(monitorDispatchPreview.payload.type ?? "--")} @ ${formatMaybeNumber(monitorDispatchPreview.payload.price)}` },
      ],
    },
    {
      title: "恢复与仓位",
      items: [
        { label: "恢复详情", value: `${String(monitorSession.state?.lastRecoveryStatus ?? "--")} · 仓位恢复 ${String(monitorSession.state?.positionRecoveryStatus ?? "--")} · 保护恢复 ${String(monitorSession.state?.protectionRecoveryStatus ?? "--")}` },
        { label: "恢复统计", value: `最后尝试 ${formatTime(String(monitorSession.state?.lastRecoveryAttemptAt ?? monitorSession.state?.lastProtectionRecoveryAt ?? ""))} · 保护订单 ${String(monitorSession.state?.recoveredProtectionCount ?? "--")} · 止损 ${String(monitorSession.state?.recoveredStopOrderCount ?? "--")} · 止盈 ${String(monitorSession.state?.recoveredTakeProfitOrderCount ?? "--")}` },
        { label: "策略持仓", value: `${String(monitorExecutionSummary.position?.side ?? "平仓")} · ${formatMaybeNumber(monitorExecutionSummary.position?.quantity)} @ ${formatMaybeNumber(monitorExecutionSummary.position?.entryPrice)} · 标记价 ${formatMaybeNumber(monitorExecutionSummary.position?.markPrice)}` },
        { label: "已恢复持仓", value: `${String(getRecord(monitorSession.state?.recoveredPosition).side ?? "平仓")} · ${formatMaybeNumber(getRecord(monitorSession.state?.recoveredPosition).quantity)} @ ${formatMaybeNumber(getRecord(monitorSession.state?.recoveredPosition).entryPrice)}` },
      ],
    },
  ] : [];

  return (
    <div className="h-full overflow-y-auto p-4 space-y-4 bg-zinc-950/20">
      <section id="monitor" className="panel panel-market panel-compact monitor-panel-main w-full">
        <div className="panel-header">
          <div>
            <p className="panel-kicker">主监控</p>
            <h3>运行中会话的大周期 K 线与执行状态</h3>
          </div>
          <div className="range-box">
            <div className="flex items-center gap-1.5 mr-2 pr-2 border-r border-white/10">
              {[
                { label: 'Auto', value: null },
                { label: '5m', value: '5' },
                { label: '15m', value: '15' },
                { label: '1h', value: '60' },
                { label: '4h', value: '240' },
                { label: '1d', value: '1D' },
              ].map((tf) => (
                <button
                  key={tf.label}
                  onClick={() => setMonitorResolution(tf.value)}
                  className={`px-2 py-0.5 rounded-md text-[10px] uppercase font-medium transition-all ${
                    monitorResolution === tf.value
                      ? 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/30'
                      : 'text-zinc-500 hover:text-zinc-300'
                  }`}
                >
                  {tf.label}
                </button>
              ))}
            </div>
            <span>{monitorMode}</span>
            <span>{monitorBars.length} 根 K 线</span>
            <span>{monitorMarkers.length} 个标记</span>
            <span className={monitorResolution ? "text-emerald-400 font-bold" : ""}>
              {monitorResolution ? 
                [ {l:'5m',v:'5'},{l:'15m',v:'15'},{l:'1h',v:'60'},{l:'4h',v:'240'},{l:'1d',v:'1D'} ].find(x=>x.v===monitorResolution)?.l 
                : String(monitorSignalState.timeframe ?? "--")}
            </span>
          </div>
        </div>
        <div className="chart-shell chart-shell-market h-[320px] min-h-[260px]">
          {monitorBars.length > 0 ? (
            <SignalMonitorChart candles={monitorBars} markers={monitorMarkers} />
          ) : (
            <div className="empty-state">当前运行会话还没有交易所大周期 K 线缓存</div>
          )}
        </div>
        <div className="grid grid-cols-8 gap-2 mt-4">
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

      <section id="monitoring-detail" className="panel panel-session">
        <div className="panel-header">
          <div>
            <p className="panel-kicker">Monitoring</p>
            <h3>运行监控与人工干预</h3>
          </div>
        </div>
        <div className="live-grid">
          {highlightedLiveSession ? (
            <div className="session-card session-card-primary">
              <div className="session-card-header">
                <div>
                  <p className="panel-kicker">Primary Session</p>
                  <h4>当前优先处理会话</h4>
                </div>
                <StatusPill tone={liveSessionHealthTone(highlightedLiveSession.health.status)}>
                  {highlightedLiveSession.health.status}
                </StatusPill>
              </div>
              <div className="live-account-meta">
                <span title="会话 ID">{shrink(highlightedLiveSession.session.id)}</span>
                <span title="账户 ID">{highlightedLiveSession.session.accountId}</span>
                <span title="策略 ID">{highlightedLiveSession.session.strategyId}</span>
                <span title="信号周期">{String(highlightedLiveSession.session.state?.signalTimeframe ?? "--")}</span>
              </div>
              <div className="backtest-notes">
                <div className="note-item">健康状态: {highlightedLiveSession.health.detail}</div>
                <div className="backtest-grid-notes">
                  <div className="note-item">恢复状态: {String(highlightedLiveSession.session.state?.positionRecoveryStatus ?? "--")}</div>
                  <div className="note-item">保护恢复: {String(highlightedLiveSession.session.state?.protectionRecoveryStatus ?? "--")} ({String(highlightedLiveSession.session.state?.recoveredProtectionCount ?? "--")})</div>
                  <div className="note-item">执行统计: 订单 {highlightedLiveSession.execution.orderCount} · 成交 {highlightedLiveSession.execution.fillCount}</div>
                  <div className="note-item">最后订单: {String(highlightedLiveSession.execution.latestOrder?.status ?? "--")} · {String(highlightedLiveSession.execution.latestOrder?.side ?? "--")} @ {formatMaybeNumber(highlightedLiveSession.execution.latestOrder?.price)}</div>
                  <div className="note-item">当前持仓: {String(highlightedLiveSession.execution.position?.side ?? "平仓")} · {formatMaybeNumber(highlightedLiveSession.execution.position?.quantity)} @ {formatMaybeNumber(highlightedLiveSession.execution.position?.entryPrice)}</div>
                </div>
              </div>
              <div className="flow-row">
                {monitorFlow.map((step) => (
                  <div key={step.key} className="flow-step">
                    <StatusPill tone={step.status}>{step.label}</StatusPill>
                    <span>{step.detail}</span>
                  </div>
                ))}
              </div>
            </div>
          ) : (
            <div className="backtest-list">
              <div className="empty-state empty-state-compact">当前没有可优先处理的运行中实盘会话</div>
            </div>
          )}
          <div className="backtest-list">
            <h4>当前会话监控细节</h4>
            {monitorSession ? (
              <div className="live-detail-layout">
                <div className="live-summary-grid">
                  {monitorSummaryItems.map((item) => (
                    <div key={item.label} className="detail-item">
                      <span>{item.label}</span>
                      <strong>{item.value}</strong>
                    </div>
                  ))}
                </div>
                <div className="live-section-grid">
                  {monitorSections.map((section) => (
                    <section key={section.title} className="live-section-card">
                      <button
                        type="button"
                        className="live-section-toggle"
                        onClick={() => setExpandedLiveSections(prev => ({ ...prev, [section.title]: !prev[section.title] }))}
                      >
                        <div>
                          <h5>{section.title}</h5>
                          <span>{expandedLiveSections[section.title] ? "收起详情" : "点击查看详情"}</span>
                        </div>
                        <strong>{expandedLiveSections[section.title] ? "−" : "+"}</strong>
                      </button>
                      {expandedLiveSections[section.title] ? (
                        <div className="live-section-items">
                          {section.items.map((item) => (
                            <div key={`${section.title}-${item.label}`} className="detail-item detail-item-compact">
                              <span>{item.label}</span>
                              <strong>{item.value}</strong>
                            </div>
                          ))}
                        </div>
                      ) : null}
                    </section>
                  ))}
                </div>
              </div>
            ) : (
              <div className="empty-state empty-state-compact">启动并选中一个实盘会话后，这里会显示监控细节</div>
            )}
          </div>
        </div>
        <div className="live-grid">
          <div className="backtest-list live-grid-span-2">
            <h4>待同步的实盘订单</h4>
            {syncableLiveOrders.length > 0 ? (
              <SimpleTable
                columns={["订单", "账户", "代码", "方向", "数量", "状态", "操作"]}
                rows={syncableLiveOrders.map((order) => [
                  shrink(order.id),
                  order.accountId,
                  order.symbol,
                  order.side,
                  formatMaybeNumber(order.quantity),
                  order.status,
                  <ActionButton
                    key={order.id}
                    label={liveSyncAction === order.id ? "Syncing..." : "Sync"}
                    disabled={liveSyncAction !== null}
                    onClick={() => syncLiveOrder(order.id)}
                  />,
                ])}
                emptyMessage="暂无已接受的实盘订单"
              />
            ) : (
              <div className="empty-state empty-state-compact">暂无已接受的实盘订单</div>
            )}
          </div>
        </div>
      </section>

      <section id="platform-health" className="panel panel-session">
        <div className="panel-header">
          <div>
            <p className="panel-kicker">Platform Health</p>
            <h3>平台健康总览</h3>
          </div>
          <div className="range-box">
            <span>{technicalStatusLabel(monitorHealth?.status ?? "--")}</span>
            <span>{formatTime(String(monitorHealth?.generatedAt ?? ""))}</span>
          </div>
        </div>
        {monitorHealth ? (
          <>
            <div className="detail-grid detail-grid-compact">
              <div className="detail-item">
                <span>平台状态</span>
                <strong>{technicalStatusLabel(monitorHealth.status)}</strong>
              </div>
              <div className="detail-item">
                <span>告警总数</span>
                <strong>{String(monitorHealth.alertCounts.total ?? 0)}</strong>
              </div>
              <div className="detail-item">
                <span>Critical / Warning</span>
                <strong>{String(monitorHealth.alertCounts.critical ?? 0)} / {String(monitorHealth.alertCounts.warning ?? 0)}</strong>
              </div>
              <div className="detail-item">
                <span>运行时静默阈值</span>
                <strong>{runtimePolicyValueLabel(platformRuntimePolicy?.runtimeQuietSeconds)}</strong>
              </div>
              <div className="detail-item">
                <span>策略评估静默阈值</span>
                <strong>{runtimePolicyValueLabel(platformRuntimePolicy?.strategyEvaluationQuietSeconds)}</strong>
              </div>
              <div className="detail-item">
                <span>账户同步阈值</span>
                <strong>{runtimePolicyValueLabel(platformRuntimePolicy?.liveAccountSyncFreshnessSeconds)}</strong>
              </div>
            </div>
            <div className="live-grid mt-4">
              <div className="backtest-list">
                <h4>Live Accounts</h4>
                <SimpleTable
                  columns={["账户", "状态", "同步年龄", "是否 stale", "运行时", "运行中实盘"]}
                  rows={monitorHealth.liveAccounts.map((item) => [
                    item.name,
                    technicalStatusLabel(item.status),
                    `${String(item.syncAgeSeconds ?? 0)}s`,
                    item.syncStale ? "是" : "否",
                    String(item.runtimeSessionCount ?? 0),
                    String(item.runningLiveSessionCount ?? 0),
                  ])}
                  emptyMessage="暂无 live account 健康数据"
                />
              </div>
              <div className="backtest-list">
                <h4>Runtime Sessions</h4>
                <SimpleTable
                  columns={["策略", "状态", "健康", "静默", "最后事件", "最后心跳"]}
                  rows={monitorHealth.runtimeSessions.map((item) => [
                    item.strategyName || shrink(item.strategyId),
                    technicalStatusLabel(item.status),
                    technicalStatusLabel(item.health),
                    item.quiet ? "是" : "否",
                    formatTime(String(item.lastEventAt ?? "")),
                    formatTime(String(item.lastHeartbeatAt ?? "")),
                  ])}
                  emptyMessage="暂无 runtime 健康数据"
                />
              </div>
            </div>
            <div className="live-grid mt-4">
              <div className="backtest-list">
                <h4>Live Sessions</h4>
                <SimpleTable
                  columns={["策略", "状态", "评估静默", "最后评估", "最后运行时事件", "同步状态"]}
                  rows={monitorHealth.liveSessions.map((item) => [
                    item.strategyName || shrink(item.strategyId),
                    technicalStatusLabel(item.status),
                    item.evaluationQuiet ? "是" : "否",
                    formatTime(String(item.lastStrategyEvaluationAt ?? "")),
                    formatTime(String(item.lastSignalRuntimeEventAt ?? "")),
                    String(item.lastSyncedOrderStatus ?? "--"),
                  ])}
                  emptyMessage="暂无 live session 健康数据"
                />
              </div>
              <div className="backtest-list">
                <h4>健康备注</h4>
                <div className="backtest-notes">
                  <div className="note-item">
                    `/api/v1/monitor/health` 现在作为平台级主摘要，alerts 列表继续保留做事件明细，不再和这里重复堆相同字段。
                  </div>
                  <div className="note-item">
                    `syncStale` 和 `evaluationQuiet` 都已经直接展示，方便区分是账户同步老化还是策略评估静默。
                  </div>
                  <div className="note-item">
                    阈值显示支持 `0 秒 (disabled)`，可以直观看出哪些健康门槛已被显式关闭。
                  </div>
                </div>
              </div>
            </div>
          </>
        ) : (
          <div className="empty-state empty-state-compact">平台健康快照尚未加载</div>
        )}
      </section>

      <section id="runtime-records" className="panel panel-session">
        <div className="panel-header">
          <div>
            <p className="panel-kicker">Records</p>
            <h3>订单、持仓、成交与告警</h3>
          </div>
        </div>
        <div className="flex flex-wrap gap-2 mb-4">
          {[
            { key: 'orders', label: '全部订单' },
            { key: 'positions', label: '持仓' },
            { key: 'fills', label: '成交明细' },
            { key: 'alerts', label: '异常告警' },
          ].map((tab) => (
            <button
              key={tab.key}
              type="button"
              className={`px-3 py-2 rounded-xl text-xs border transition-colors ${
                dockTab === tab.key
                  ? 'border-emerald-400/40 bg-emerald-500/10 text-emerald-300'
                  : 'border-white/5 bg-white/5 text-zinc-400 hover:text-zinc-200 hover:bg-white/10'
              }`}
              onClick={() => onDockTabChange(tab.key as 'orders' | 'positions' | 'fills' | 'alerts')}
            >
              {tab.label}
            </button>
          ))}
        </div>
        <div className="panel-compact bg-white/5 rounded-2xl p-3 border border-white/5 overflow-x-auto">
          {dockContent}
        </div>
      </section>
    </div>
  );
}
