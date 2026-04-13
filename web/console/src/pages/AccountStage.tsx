import React, { useMemo } from 'react';
import { useUIStore } from '../store/useUIStore';
import { useTradingStore } from '../store/useTradingStore';
import { ActionButton } from '../components/ui/ActionButton';
import { SimpleTable } from '../components/ui/SimpleTable';
import { StatusPill } from '../components/ui/StatusPill';
import { SignalBarChart } from '../components/charts/SignalBarChart';
import { formatTime, formatMaybeNumber, shrink } from '../utils/format';
import { 
  getRecord, 
  getList,
  strategyLabel, 
  deriveLiveSessionExecutionSummary, 
  deriveLiveSessionHealth, 
  deriveLiveSessionFlow,
  deriveLiveNextAction,
  deriveHighlightedLiveSession,
  deriveRuntimeMarketSnapshot,
  deriveRuntimeSourceSummary,
  derivePrimarySignalBarState,
  deriveSignalBarCandles,
  deriveSignalActionSummary,
  deriveLivePreflightSummary,
  deriveLiveAlerts,
  deriveLiveDispatchPreview,
  deriveRuntimeReadiness,
  buildAlertNotes,
  buildSignalActionNotes,
  buildSignalBarStateNotes,
  buildRuntimeEventNotes,
  buildSourceStateNotes,
  buildTimelineNotes,
  runtimeReadinessTone,
  signalActionTone,
  decisionStateTone,
  boolLabel,
  liveSessionHealthTone,
  getNumber,
  technicalStatusLabel
} from '../utils/derivation';
import { AccountRecord, LiveSession, SignalRuntimeSession, LiveNextAction, ActiveSettingsModal } from '../types/domain';

interface AccountStageProps {
  logout: () => void;
  openLiveAccountModal: () => void;
  openLiveBindingModal: () => void;
  openLiveSessionModal: (session?: LiveSession | null) => void;
  launchLiveFlow: (account: AccountRecord) => void;
  runLiveSessionAction: (id: string, action: "start" | "stop") => void;
  dispatchLiveSessionIntent: (id: string) => void;
  syncLiveSession: (id: string) => void;
  deleteLiveSession: (id: string) => Promise<void>;
  syncLiveAccount: (id: string) => void;
  syncLiveOrder: (id: string) => void;
  jumpToSignalRuntimeSession: (id: string) => void;
  runLiveNextAction: (account: AccountRecord, nextAction: LiveNextAction, runtime: SignalRuntimeSession | null) => void;
  selectQuickLiveAccount: (id: string) => void;
  bindAccountSignalSource: () => void;
  unbindAccountSignalSource: (accountId: string, bindingId: string) => void;
  bindStrategySignalSource: () => void;
  unbindStrategySignalSource: (strategyId: string, bindingId: string) => void;
  updateRuntimePolicy: () => void;
  createSignalRuntimeSession: () => void;
  deleteSignalRuntimeSession: (sessionId: string) => void;
  runSignalRuntimeAction: (id: string, action: "start" | "stop") => void;
}

export function AccountStage({
  logout,
  openLiveAccountModal,
  openLiveBindingModal,
  openLiveSessionModal,
  launchLiveFlow,
  runLiveSessionAction,
  dispatchLiveSessionIntent,
  syncLiveSession,
  deleteLiveSession,
  syncLiveAccount,
  syncLiveOrder,
  jumpToSignalRuntimeSession,
  runLiveNextAction,
  selectQuickLiveAccount,
  bindAccountSignalSource,
  unbindAccountSignalSource,
  bindStrategySignalSource,
  unbindStrategySignalSource,
  updateRuntimePolicy,
  createSignalRuntimeSession,
  deleteSignalRuntimeSession,
  runSignalRuntimeAction
}: AccountStageProps) {
  const loading = useUIStore(s => s.loading);
  const liveAccounts = useTradingStore(s => s.accounts);
  const authSession = useUIStore(s => s.authSession);
  const settingsMenuOpen = useUIStore(s => s.settingsMenuOpen);
  const setSettingsMenuOpen = useUIStore(s => s.setSettingsMenuOpen);
  const liveSessions = useTradingStore(s => s.liveSessions);
  const orders = useTradingStore(s => s.orders);
  const fills = useTradingStore(s => s.fills);
  const positions = useTradingStore(s => s.positions);
  const liveSessionForm = useUIStore(s => s.liveSessionForm);
  const liveBindingForm = useUIStore(s => s.liveBindingForm);
  const quickLiveAccountId = liveSessionForm.accountId || liveBindingForm.accountId || liveAccounts[0]?.id || "";
  const quickLiveAccount = useTradingStore(s => s.accounts.find(a => a.id === quickLiveAccountId) || null);
  const strategies = useTradingStore(s => s.strategies);
  const liveSessionAction = useUIStore(s => s.liveSessionAction);
  const liveSessionDeleteAction = useUIStore(s => s.liveSessionDeleteAction);
  const liveAccountSyncAction = useUIStore(s => s.liveAccountSyncAction);
  const liveSyncAction = useUIStore(s => s.liveSyncAction);
  const liveFlowAction = useUIStore(s => s.liveFlowAction);
  const liveBindAction = useUIStore(s => s.liveBindAction);
  const signalBindingAction = useUIStore(s => s.signalBindingAction);
  const signalRuntimeAction = useUIStore(s => s.signalRuntimeAction);
  const liveSessionCreateAction = useUIStore(s => s.liveSessionCreateAction);
  const liveSessionLaunchAction = useUIStore(s => s.liveSessionLaunchAction);
  const signalRuntimeSessions = useTradingStore(s => s.signalRuntimeSessions);
  const accountSignalBindingMap = useTradingStore(s => s.accountSignalBindingMap);
  const runtimePolicy = useTradingStore(s => s.runtimePolicy);
  const signalCatalog = useTradingStore(s => s.signalCatalog);
  const accountSignalForm = useUIStore(s => s.accountSignalForm);
  const setAccountSignalForm = useUIStore(s => s.setAccountSignalForm);
  const strategySignalForm = useUIStore(s => s.strategySignalForm);
  const setStrategySignalForm = useUIStore(s => s.setStrategySignalForm);
  const signalSourceTypes = useTradingStore(s => s.signalSourceTypes);
  const accountSignalBindings = useTradingStore(s => s.accountSignalBindings);
  const strategySignalBindings = useTradingStore(s => s.strategySignalBindings);
  const runtimePolicyForm = useUIStore(s => s.runtimePolicyForm);
  const setRuntimePolicyForm = useUIStore(s => s.setRuntimePolicyForm);
  const runtimePolicyAction = useUIStore(s => s.runtimePolicyAction);
  const signalRuntimeForm = useUIStore(s => s.signalRuntimeForm);
  const setSignalRuntimeForm = useUIStore(s => s.setSignalRuntimeForm);
  const signalRuntimePlan = useTradingStore(s => s.signalRuntimePlan);
  const signalRuntimeAdapters = useTradingStore(s => s.signalRuntimeAdapters);
  const selectedSignalRuntimeId = useTradingStore(s => s.selectedSignalRuntimeId);
  const setSelectedSignalRuntimeId = useTradingStore(s => s.setSelectedSignalRuntimeId);

  // Derived states
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

  const highlightedLiveSessionFlow = useMemo(
    () =>
      highlightedLiveSession
        ? deriveLiveSessionFlow(highlightedLiveSession.session, highlightedLiveSession.execution)
        : [],
    [highlightedLiveSession]
  );

  const monitorMode = highlightedLiveSession?.session ? "LIVE" : "--";
  const strategyIds = useMemo(() => new Set(strategies.map((item) => item.id)), [strategies]);
  const validLiveSessions = useMemo(
    () => liveSessions.filter((item) => strategyIds.has(item.strategyId)),
    [liveSessions, strategyIds]
  );

  const primaryLiveSession = highlightedLiveSession?.session ?? null;
  const primaryLiveRuntimeState = getRecord(highlightedLiveRuntime?.state);
  const primaryLiveRuntimeSummary = getRecord(primaryLiveRuntimeState.lastEventSummary);
  const primaryLiveSessionMarket = deriveRuntimeMarketSnapshot(
    getRecord(primaryLiveRuntimeState.sourceStates),
    primaryLiveRuntimeSummary
  );
  const primaryLiveSessionRuntimeReadiness = deriveRuntimeReadiness(
    primaryLiveRuntimeState,
    deriveRuntimeSourceSummary(getRecord(primaryLiveRuntimeState.sourceStates), runtimePolicy),
    {
      requireTick: true,
      requireOrderBook: false
    }
  );
  const primaryLiveSessionIntent = getRecord(primaryLiveSession?.state?.lastStrategyIntent);
  const primaryLiveSessionSignalBarDecision = getRecord(primaryLiveSession?.state?.lastStrategyEvaluationSignalBarDecision);
  const primaryLiveExecutionSummary = highlightedLiveSession?.execution ?? deriveLiveSessionExecutionSummary(null, orders, fills, positions);
  const primaryLiveSessionTimeline = getList(primaryLiveSession?.state?.timeline);
  
  const primaryLiveAccount = primaryLiveSession ? liveAccounts.find(a => a.id === primaryLiveSession.accountId) || null : null;
  const primaryLiveBindings = primaryLiveSession ? accountSignalBindingMap[primaryLiveSession.accountId] || [] : [];
  const primaryLiveRuntimeSessions = primaryLiveSession ? signalRuntimeSessions.filter(s => s.accountId === primaryLiveSession.accountId) : [];
  
  const primaryLiveDispatchPreview = deriveLiveDispatchPreview(
    primaryLiveSession,
    primaryLiveAccount,
    primaryLiveBindings,
    primaryLiveRuntimeSessions,
    highlightedLiveRuntime,
    primaryLiveSessionRuntimeReadiness,
    primaryLiveSessionIntent
  );
  
  const strategyOptions = useMemo(() => strategies.map((strategy) => ({
    value: strategy.id,
    label: strategyLabel(strategy),
  })), [strategies]);

  const selectedSignalRuntime = useMemo(() =>
    signalRuntimeSessions.find((item) => item.id === selectedSignalRuntimeId) ?? signalRuntimeSessions[0] ?? null,
    [signalRuntimeSessions, selectedSignalRuntimeId]
  );
  const selectedSignalRuntimeState = getRecord(selectedSignalRuntime?.state);
  const selectedSignalRuntimePlan = getRecord(selectedSignalRuntimeState.plan);
  const selectedSignalRuntimeLastSummary = getRecord(selectedSignalRuntimeState.lastEventSummary);
  const selectedSignalRuntimeSourceStates = getRecord(selectedSignalRuntimeState.sourceStates);
  const selectedSignalBarStates = getRecord(selectedSignalRuntimeState.signalBarStates);
  const selectedSignalRuntimeTimeline = getList(selectedSignalRuntimeState.timeline);
  const selectedSignalRuntimeSignalBars = deriveSignalBarCandles(selectedSignalRuntimeSourceStates);
  const selectedSignalRuntimeSubscriptions = Array.isArray(selectedSignalRuntimeState.subscriptions)
    ? (selectedSignalRuntimeState.subscriptions as Array<Record<string, unknown>>)
    : [];

  const syncableLiveOrders = orders.filter((item) => item.metadata?.executionMode === "live" && item.status === "ACCEPTED");

  function setActiveSettingsModal(modal: ActiveSettingsModal) {
    useUIStore.getState().setActiveSettingsModal(modal);
  }

  return (
    <div className="absolute inset-0 overflow-y-auto p-6 space-y-6 bg-zinc-950/50">
      <section id="overview" className="hero">
        <div>
          <p className="eyebrow">交易主控</p>
          <h2>实盘 / 模拟统一监控与执行运行台</h2>
          <p className="hero-copy">
            当前页面直接消费平台 API，主监控优先展示正在运行的实盘会话；如果当前没有实盘会话，则回退展示模拟盘。大周期 K 线直接来自交易所信号源，执行侧信息则展示实时 tick、盘口、持仓、订单和盈亏。
          </p>
        </div>

      </section>

      <section id="live" className="panel panel-session">
        <div className="panel-header">
          <div>
            <p className="panel-kicker">Live Trading</p>
            <h3>实盘账户与订单同步</h3>
          </div>
        </div>
        {highlightedLiveSession ? (
          <div className="live-grid">
            <div className="session-card session-card-primary">
              <div className="session-card-header">
                <div>
                  <p className="panel-kicker">Live Overview</p>
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
                {highlightedLiveSessionFlow.map((step) => (
                  <div key={step.key} className="flow-step">
                    <StatusPill tone={step.status}>{step.label}</StatusPill>
                    <span>{step.detail}</span>
                  </div>
                ))}
              </div>
            </div>
          </div>
        ) : null}
        <div className="live-grid">
          <div className="backtest-form session-form panel-compact">
            <h4>配置入口已收进弹窗</h4>
            <div className="backtest-notes notes-compact">
              <div className="note-item">首屏顶部提供账户下拉、创建账户、绑定适配器、创建会话和一键拉起。</div>
              <div className="note-item">当前选中账户：{quickLiveAccount?.name ?? "--"} · {quickLiveAccount?.status ?? "--"} · {quickLiveAccount?.exchange ?? "--"}</div>
              <div className="note-item">sandbox=true 时默认从 `.env` 读取 `BINANCE_TESTNET_API_KEY` / `BINANCE_TESTNET_API_SECRET`。</div>
            </div>
            <div className="backtest-actions inline-actions">
              <ActionButton label="新建账户" variant="ghost" onClick={openLiveAccountModal} />
              <ActionButton
                label="绑定适配器"
                variant="ghost"
                disabled={!quickLiveAccountId}
                onClick={() => {
                  if (quickLiveAccountId) {
                    selectQuickLiveAccount(quickLiveAccountId);
                  }
                  openLiveBindingModal();
                }}
              />
              <ActionButton
                label="创建会话"
                variant="ghost"
                disabled={!quickLiveAccountId}
                onClick={() => {
                  if (quickLiveAccountId) {
                    selectQuickLiveAccount(quickLiveAccountId);
                  }
                  openLiveSessionModal();
                }}
              />
            </div>
          </div>

          <div className="backtest-list">
            <h4>实盘策略会话</h4>
            <div className="backtest-notes notes-compact">
              <div className="note-item">有效会话：{validLiveSessions.length}</div>
            </div>
            {validLiveSessions.length > 0 ? (
              <div className="live-card-list">
                {validLiveSessions.map((session) => {
                  const intent = getRecord(session.state?.lastStrategyIntent);
                  const executionSummary = deriveLiveSessionExecutionSummary(session, orders, fills, positions);
                  const sessionHealth = deriveLiveSessionHealth(session, executionSummary);
                  const sessionAccount = liveAccounts.find((account) => account.id === session.accountId) ?? null;
                  const sessionBinding = getRecord(sessionAccount?.metadata?.liveBinding);
                  const sessionAccountReady =
                    sessionAccount?.status === "CONFIGURED" ||
                    sessionAccount?.status === "READY" ||
                    (String(sessionBinding.connectionMode ?? "") !== "" && String(sessionBinding.connectionMode ?? "") !== "disabled");
                  return (
                    <div key={session.id} className="session-row">
                      <div className="session-row-main">
                        <div className="session-row-title">
                          <strong>{shrink(session.id)}</strong>
                          <StatusPill tone={liveSessionHealthTone(sessionHealth.status)}>{sessionHealth.status}</StatusPill>
                          <StatusPill tone={session.status === "RUNNING" ? "ready" : session.status === "STOPPED" ? "watch" : "neutral"}>
                            {session.status}
                          </StatusPill>
                        </div>
                        <div className="live-account-meta session-row-meta">
                          <span>{session.accountId}</span>
                          <span>{strategyLabel(strategies.find((strategy) => strategy.id === session.strategyId))}</span>
                          <span>{String(session.state?.signalTimeframe ?? "--")}</span>
                          <span>{sessionAccount?.status ?? "--"}</span>
                          <span>{String(intent.action ?? "no-intent")}</span>
                          <span>{String(executionSummary.position?.side ?? "FLAT")}</span>
                          <span>{formatMaybeNumber(executionSummary.position?.quantity)}</span>
                          <span>{executionSummary.orderCount}/{executionSummary.fillCount}</span>
                          {!sessionAccountReady ? <span>先绑定适配器</span> : null}
                        </div>
                      </div>
                      <div className="session-row-actions inline-actions">
                        <ActionButton
                          label="编辑"
                          variant="ghost"
                          disabled={liveSessionAction !== null || liveSessionDeleteAction !== null}
                          onClick={() => openLiveSessionModal(session)}
                        />
                        {String(session.state?.signalRuntimeSessionId ?? "") ? (
                          <ActionButton
                            label="打开运行时"
                            variant="ghost"
                            onClick={() => jumpToSignalRuntimeSession(String(session.state?.signalRuntimeSessionId ?? ""))}
                          />
                        ) : null}
                        <ActionButton
                          label={liveSessionAction === `${session.id}:start` ? "启动中..." : "启动"}
                          disabled={liveSessionAction !== null || session.status === "RUNNING" || !sessionAccountReady}
                          onClick={() => runLiveSessionAction(session.id, "start")}
                        />
                        {!sessionAccountReady ? (
                          <ActionButton
                            label="绑定适配器"
                            variant="ghost"
                            disabled={liveSessionAction !== null || liveSessionDeleteAction !== null}
                            onClick={() => {
                              selectQuickLiveAccount(session.accountId);
                              openLiveBindingModal();
                            }}
                          />
                        ) : null}
                        <ActionButton
                          label={liveSessionAction === `${session.id}:dispatch` ? "分发中..." : "分发意图"}
                          disabled={
                            liveSessionAction !== null ||
                            !getRecord(session.state?.lastStrategyIntent).action ||
                            String(session.state?.dispatchMode ?? "") !== "manual-review" ||
                            (primaryLiveSession?.id === session.id && primaryLiveDispatchPreview.status === "blocked")
                          }
                          onClick={() => dispatchLiveSessionIntent(session.id)}
                        />
                        <ActionButton
                          label={liveSessionAction === `${session.id}:sync` ? "同步中..." : "同步最新订单"}
                          variant="ghost"
                          disabled={liveSessionAction !== null || !String(session.state?.lastDispatchedOrderId ?? "")}
                          onClick={() => syncLiveSession(session.id)}
                        />
                        <ActionButton
                          label={liveSessionAction === `${session.id}:stop` ? "停止中..." : "停止"}
                          variant="ghost"
                          disabled={liveSessionAction !== null || session.status === "STOPPED"}
                          onClick={() => runLiveSessionAction(session.id, "stop")}
                        />
                        <ActionButton
                          label={liveSessionDeleteAction === session.id ? "删除中..." : "删除"}
                          variant="ghost"
                          disabled={liveSessionAction !== null || liveSessionDeleteAction !== null}
                          onClick={() => void deleteLiveSession(session.id)}
                        />
                      </div>
                    </div>
                  );
                })}
              </div>
            ) : (
              <div className="empty-state empty-state-compact">暂无有效实盘会话</div>
            )}
            {primaryLiveSession ? (
              <div className="backtest-notes">
                <div className="note-item">
                  运行环境: {String(primaryLiveSession.state?.signalRuntimeStatus ?? "--")} · {formatTime(String(primaryLiveSession.state?.lastSignalRuntimeEventAt ?? ""))}
                </div>
                <div className="note-item">
                  行情数据: {formatMaybeNumber(primaryLiveSessionMarket.tradePrice)} · {formatMaybeNumber(primaryLiveSessionMarket.bestBid)} / {formatMaybeNumber(primaryLiveSessionMarket.bestAsk)}
                </div>
                <div className="note-item">
                  就续预检: {primaryLiveSessionRuntimeReadiness.status} · {primaryLiveSessionRuntimeReadiness.reason}
                </div>
                <div className="note-item">
                  信号意图: {String(primaryLiveSessionIntent.action ?? "无")} · {String(primaryLiveSessionIntent.side ?? "--")} · {formatMaybeNumber(primaryLiveSessionIntent.priceHint)}
                </div>
                <div className="note-item">
                  意图预览: 数量 {formatMaybeNumber(primaryLiveSessionIntent.quantity)} · 报价源 {String(primaryLiveSessionIntent.priceSource ?? "--")} · 信号种类 {String(primaryLiveSessionIntent.signalKind ?? "--")}
                </div>
                <div className="note-item">
                  意图上下文: 价差 {formatMaybeNumber(primaryLiveSessionIntent.spreadBps)} bps · 偏置 {String(primaryLiveSessionIntent.liquidityBias ?? "--")} · ma20 {formatMaybeNumber(primaryLiveSessionIntent.ma20)} · atr14 {formatMaybeNumber(primaryLiveSessionIntent.atr14)}
                </div>
                <div className="note-item">
                  信号过滤: 周期 {String(primaryLiveSessionSignalBarDecision.timeframe ?? "--")} · sma5 {formatMaybeNumber(primaryLiveSessionSignalBarDecision.sma5)} · 做多反转触发{" "}
                  {boolLabel(primaryLiveSessionSignalBarDecision.longEarlyReversalReady)} · 做空反转触发 {boolLabel(primaryLiveSessionSignalBarDecision.shortEarlyReversalReady)} ·{" "}
                  {String(primaryLiveSessionSignalBarDecision.reason ?? "--")}
                </div>
                <div className="note-item">
                  指令分发: {String(primaryLiveSession?.state?.dispatchMode ?? "--")} · 冷却 {String(primaryLiveSession?.state?.dispatchCooldownSeconds ?? "--")}s · 最后订单 {String(primaryLiveSession?.state?.lastDispatchedOrderId ?? "--")}
                </div>
                <div className="note-item">
                  执行配置: {String(getRecord(primaryLiveSession?.state?.lastExecutionProfile).executionProfile ?? "--")} ·{" "}
                  {String(getRecord(primaryLiveSession?.state?.lastExecutionProfile).orderType ?? "--")} · TIF {" "}
                  {String(getRecord(primaryLiveSession?.state?.lastExecutionProfile).timeInForce ?? "--")} · 只减仓{" "}
                  {boolLabel(getRecord(primaryLiveSession?.state?.lastExecutionProfile).reduceOnly)}
                </div>
                <div className="note-item">
                  执行遥测: {String(getRecord(primaryLiveSession?.state?.lastExecutionTelemetry).decision ?? "--")} · 价差{" "}
                  {formatMaybeNumber(getRecord(getRecord(primaryLiveSession?.state?.lastExecutionTelemetry).book).spreadBps)} bps · 盘口不平衡{" "}
                  {formatMaybeNumber(getRecord(getRecord(primaryLiveSession?.state?.lastExecutionTelemetry).book).bookImbalance)}
                </div>
                <div className="note-item">
                  分发状态: {String(getRecord(primaryLiveSession?.state?.lastExecutionDispatch).status ?? "--")} ·{" "}
                  {String(getRecord(primaryLiveSession?.state?.lastExecutionDispatch).executionMode ?? "--")} · 备选方案{" "}
                  {boolLabel(getRecord(primaryLiveSession?.state?.lastExecutionDispatch).fallback)}
                </div>
                <div className="note-item">
                  成交分析: 预期价格 {formatMaybeNumber(getRecord(primaryLiveSession?.state?.lastExecutionDispatch).expectedPrice)} · 滑点偏移{" "}
                  {formatMaybeNumber(getRecord(primaryLiveSession?.state?.lastExecutionDispatch).priceDriftBps)} bps
                </div>
                <div className="note-item">
                  执行统计: 方案数 {String(getRecord(primaryLiveSession?.state?.executionEventStats).proposalCount ?? "--")} · Maker 决策{" "}
                  {String(getRecord(primaryLiveSession?.state?.executionEventStats).makerRestingDecisionCount ?? "--")} · 备选分发{" "}
                  {String(getRecord(primaryLiveSession?.state?.executionEventStats).fallbackDispatchCount ?? "--")} · 平均偏移{" "}
                  {formatMaybeNumber(getRecord(primaryLiveSession?.state?.executionEventStats).avgPriceDriftBps)} bps
                </div>
                <div className="note-item">
                  自动分发: 最后触发 {formatTime(String(primaryLiveSession?.state?.lastDispatchedAt ?? ""))} · 最后错误 {String(primaryLiveSession?.state?.lastAutoDispatchError ?? "--")}
                </div>
                <div className="note-item">
                  数据同步: {String(primaryLiveSession?.state?.lastSyncedOrderStatus ?? "--")} · {formatTime(String(primaryLiveSession?.state?.lastSyncedAt ?? ""))} · 错误 {String(primaryLiveSession?.state?.lastSyncError ?? "--")}
                </div>
                <div className="note-item">
                  恢复详情: {String(primaryLiveSession?.state?.lastRecoveryStatus ?? "--")} · 仓位恢复 {String(primaryLiveSession?.state?.positionRecoveryStatus ?? "--")} · 保护恢复{" "}
                  {String(primaryLiveSession?.state?.protectionRecoveryStatus ?? "--")}
                </div>
                <div className="note-item">
                  恢复统计: 最后尝试 {formatTime(String(primaryLiveSession?.state?.lastRecoveryAttemptAt ?? primaryLiveSession?.state?.lastProtectionRecoveryAt ?? ""))} · 保护订单数{" "}
                  {String(primaryLiveSession?.state?.recoveredProtectionCount ?? "--")} · 止损 {String(primaryLiveSession?.state?.recoveredStopOrderCount ?? "--")} · 止盈{" "}
                  {String(primaryLiveSession?.state?.recoveredTakeProfitOrderCount ?? "--")}
                </div>
                <div className="note-item">
                  执行汇总: 订单 {primaryLiveExecutionSummary.orderCount} · 成交 {primaryLiveExecutionSummary.fillCount} · 最新订单状态 {String(primaryLiveExecutionSummary.latestOrder?.status ?? "--")}
                </div>
                <div className="note-item">
                  策略持仓: {String(primaryLiveExecutionSummary.position?.side ?? "平仓")} · {formatMaybeNumber(primaryLiveExecutionSummary.position?.quantity)} @ {formatMaybeNumber(primaryLiveExecutionSummary.position?.entryPrice)} · 标记价 {formatMaybeNumber(primaryLiveExecutionSummary.position?.markPrice)}
                </div>
                <div className="note-item">
                  已恢复持仓: {String(getRecord(primaryLiveSession?.state?.recoveredPosition).side ?? "平仓")} ·{" "}
                  {formatMaybeNumber(getRecord(primaryLiveSession?.state?.recoveredPosition).quantity)} @{" "}
                  {formatMaybeNumber(getRecord(primaryLiveSession?.state?.recoveredPosition).entryPrice)}
                </div>
                <div className="note-item">
                  分发预览: {primaryLiveDispatchPreview.reason} · {primaryLiveDispatchPreview.detail}
                </div>
                <div className="note-item">
                  最终指令: {String(primaryLiveDispatchPreview.payload.side ?? "--")} {formatMaybeNumber(primaryLiveDispatchPreview.payload.quantity)} {String(primaryLiveDispatchPreview.payload.symbol ?? "--")} · {String(primaryLiveDispatchPreview.payload.type ?? "--")} @ {formatMaybeNumber(primaryLiveDispatchPreview.payload.price)}
                </div>
                {buildTimelineNotes(primaryLiveSessionTimeline).slice(0, 4).map((line) => (
                  <div key={line} className="note-item">
                    {line}
                  </div>
                ))}
              </div>
            ) : null}
          </div>
        </div>

        <div className="live-grid">
          <div className="backtest-list">
            <h4>实盘账户</h4>
            {liveAccounts.length > 0 ? (
              <div className="live-card-list">
                {liveAccounts.map((account) => {
                  const binding = (account.metadata?.liveBinding as Record<string, unknown> | undefined) ?? {};
                  const syncSnapshot = getRecord(getRecord(account.metadata).liveSyncSnapshot);
                  const bindings = accountSignalBindingMap[account.id] ?? [];
                  const runtimeSessionsForAccount = signalRuntimeSessions.filter((item) => item.accountId === account.id);
                  const activeRuntime = runtimeSessionsForAccount.find((item) => item.status === "RUNNING") ?? runtimeSessionsForAccount[0] ?? null;
                  const activeRuntimeState = getRecord(activeRuntime?.state);
                  const activeRuntimeSummary = getRecord(activeRuntimeState.lastEventSummary);
                  const activeRuntimeMarket = deriveRuntimeMarketSnapshot(getRecord(activeRuntimeState.sourceStates), activeRuntimeSummary);
                  const activeRuntimeSourceSummary = deriveRuntimeSourceSummary(
                    getRecord(activeRuntimeState.sourceStates),
                    runtimePolicy
                  );
                  const activeSignalBarState = derivePrimarySignalBarState(getRecord(activeRuntimeState.signalBarStates));
                  const activeSignalAction = deriveSignalActionSummary(activeSignalBarState);
                  const activeRuntimeTimeline = getList(activeRuntimeState.timeline);
                  const activeRuntimeReadiness = deriveRuntimeReadiness(activeRuntimeState, activeRuntimeSourceSummary, {
                    requireTick: bindings.some((item) => item.streamType === "trade_tick"),
                    requireOrderBook: bindings.some((item) => item.streamType === "order_book"),
                  });
                  const livePreflight = deriveLivePreflightSummary(
                    account,
                    bindings,
                    runtimeSessionsForAccount,
                    activeRuntime,
                    activeRuntimeReadiness
                  );
                  const liveNextAction = deriveLiveNextAction(livePreflight);
                  const liveAlerts = deriveLiveAlerts(
                    account,
                    activeRuntimeState,
                    activeRuntimeSourceSummary,
                    activeRuntimeReadiness,
                    activeSignalAction,
                    runtimePolicy
                  );
                  return (
                    <div key={account.id} className="session-stat">
                      <span>{account.name}</span>
                      <strong>{account.status}</strong>
                      <div className="live-account-meta">
                        <span>交易所: {account.exchange}</span>
                        <span>适配器: {String(binding.adapterKey ?? "--")}</span>
                        <span>持仓模式: {String(binding.positionMode ?? "--")} / {String(binding.marginMode ?? "--")}</span>
                      </div>
                      <div className="live-account-meta">
                        <span>同步状态: {String(syncSnapshot.syncStatus ?? "未同步")}</span>
                        <span>最后同步: {formatTime(String(getRecord(account.metadata).lastLiveSyncAt ?? ""))}</span>
                        <span>来源: {String(syncSnapshot.source ?? "--")}</span>
                      </div>
                      <div className="live-account-meta">
                        <span>{bindings.length} 个信号绑定</span>
                        <span>{runtimeSessionsForAccount.length} 个运行实例</span>
                        <span>{activeRuntime ? `${activeRuntime.status} · ${String(activeRuntimeState.health ?? "--")}` : "无运行实例"}</span>
                      </div>
                      <div className="live-account-meta">
                        <span>最后事件: {String(activeRuntimeSummary.event ?? "--")}</span>
                        <span>最后心跳: {formatTime(String(activeRuntimeState.lastHeartbeatAt ?? ""))}</span>
                        <span>更新时间: {formatTime(String(activeRuntimeState.lastEventAt ?? ""))}</span>
                      </div>
                      <div className="live-account-meta">
                        <span>成交价: {formatMaybeNumber(activeRuntimeMarket.tradePrice)}</span>
                        <span>买/卖: {formatMaybeNumber(activeRuntimeMarket.bestBid)} / {formatMaybeNumber(activeRuntimeMarket.bestAsk)}</span>
                        <span>价差: {formatMaybeNumber(activeRuntimeMarket.spreadBps)} bps</span>
                      </div>
                      <div className="live-account-meta">
                        <span>成交笔数: {activeRuntimeSourceSummary.tradeTickCount}</span>
                        <span>盘口笔数: {activeRuntimeSourceSummary.orderBookCount}</span>
                        <span>陈旧数: {activeRuntimeSourceSummary.staleCount}</span>
                        <span>最后采集: {formatTime(String(activeRuntimeSourceSummary.latestEventAt ?? ""))}</span>
                      </div>
                      <div className="live-account-meta">
                        <span>
                          <StatusPill tone={runtimeReadinessTone(activeRuntimeReadiness.status)}>
                            {activeRuntimeReadiness.status}
                          </StatusPill>
                        </span>
                        <span>{activeRuntimeReadiness.reason}</span>
                      </div>
                      <div className="live-account-meta">
                        <span>
                          <StatusPill tone={runtimeReadinessTone(livePreflight.status)}>
                            {livePreflight.status}
                          </StatusPill>
                        </span>
                        <span>{livePreflight.reason}</span>
                        <span>{livePreflight.detail}</span>
                      </div>
                      <div className="live-account-meta">
                        <span>下一步操作</span>
                        <span>{liveNextAction.label}</span>
                        <span>{liveNextAction.detail}</span>
                        <button
                          type="button"
                          className="filter-chip"
                          disabled={
                            liveFlowAction !== null ||
                            liveBindAction ||
                            signalBindingAction !== null ||
                            signalRuntimeAction !== null ||
                            liveSessionAction !== null ||
                            liveSessionCreateAction ||
                            liveSessionLaunchAction
                          }
                          onClick={() => launchLiveFlow(account)}
                        >
                          {liveFlowAction === account.id ? "启动中..." : "启动实盘流程"}
                        </button>
                        <button
                          type="button"
                          className="filter-chip"
                          onClick={() => runLiveNextAction(account, liveNextAction, activeRuntime)}
                        >
                          详情/查看
                        </button>
                      </div>
                      <div className="live-account-meta">
                        <span>周期: {String(activeSignalBarState.timeframe ?? "--")}</span>
                        <span>ma20: {formatMaybeNumber(activeSignalBarState.ma20)}</span>
                        <span>atr14: {formatMaybeNumber(activeSignalBarState.atr14)}</span>
                      </div>
                      <div className="live-account-meta">
                        <span>
                          <StatusPill tone={signalActionTone(activeSignalAction.bias, activeSignalAction.state)}>
                            {activeSignalAction.bias}
                          </StatusPill>
                        </span>
                        <span>
                          <StatusPill tone={decisionStateTone(activeSignalAction.state)}>
                            {activeSignalAction.state}
                          </StatusPill>
                        </span>
                        <span>{activeSignalAction.reason}</span>
                      </div>
                      <div className="backtest-notes">
                        {buildAlertNotes(liveAlerts).map((item) => (
                          <div key={`${account.id}-${item.title}-${item.detail}`} className={`note-item note-item-alert note-item-alert-${item.level}`}>
                            <strong>{item.title}</strong> {item.detail}
                          </div>
                        ))}
                        {buildSignalActionNotes(activeSignalAction).map((line) => (
                          <div key={line} className="note-item">
                            {line}
                          </div>
                        ))}
                        <div className="note-item">
                          实盘预检: {livePreflight.reason} · {livePreflight.detail}
                        </div>
                        <div className="note-item">
                          下一步操作: {liveNextAction.label} · {liveNextAction.detail}
                        </div>
                        <div className="note-item">
                          账户同步: 订单 {String(syncSnapshot.orderCount ?? "--")} · 成交 {String(syncSnapshot.fillCount ?? "--")} · 持仓 {String(syncSnapshot.positionCount ?? "--")}
                        </div>
                        {buildSignalBarStateNotes(activeSignalBarState).map((line) => (
                          <div key={line} className="note-item">
                            {line}
                          </div>
                        ))}
                        {buildRuntimeEventNotes(activeRuntimeSummary).map((line) => (
                          <div key={line} className="note-item">
                            {line}
                          </div>
                        ))}
                        {buildSourceStateNotes(getRecord(activeRuntimeState.sourceStates)).map((line) => (
                          <div key={line} className="note-item">
                            {line}
                          </div>
                        ))}
                        {buildTimelineNotes(activeRuntimeTimeline).slice(0, 3).map((line) => (
                          <div key={line} className="note-item">
                            {line}
                          </div>
                        ))}
                      </div>
                      <div className="inline-actions">
                        <ActionButton
                          label={liveAccountSyncAction === account.id ? "同步中..." : "同步账户"}
                          variant="ghost"
                          disabled={liveAccountSyncAction !== null}
                          onClick={() => syncLiveAccount(account.id)}
                        />
                        {activeRuntime ? (
                          <ActionButton
                            label="打开运行环境"
                            variant="ghost"
                            onClick={() => jumpToSignalRuntimeSession(activeRuntime.id)}
                          />
                        ) : null}
                      </div>
                    </div>
                  );
                })}
              </div>
            ) : (
              <div className="empty-state empty-state-compact">暂无实盘账户</div>
            )}
          </div>

          <div className="backtest-list">
            <h4>已接受的实盘订单</h4>
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

      <section id="signals" className="panel panel-session">
        <div className="panel-header">
          <div>
            <p className="panel-kicker">Signal Runtime</p>
            <h3>信号源绑定与市场数据运行时</h3>
          </div>
          <div className="range-box">
            <span>{signalCatalog?.sources?.length ?? 0} 个源</span>
            <span>{signalRuntimeSessions.length} 个会话</span>
          </div>
        </div>

        <div className="live-grid">
          <div className="backtest-form session-form">
            <h4>绑定账户信号源</h4>
            <div className="form-grid">
              <label className="form-field">
                <span>账户</span>
                <select value={accountSignalForm.accountId} onChange={(event) => setAccountSignalForm((current) => ({ ...current, accountId: event.target.value }))}>
                  {liveAccounts.map((item) => (
                    <option key={item.id} value={item.id}>
                      {item.name} ({technicalStatusLabel(item.mode)})
                    </option>
                  ))}
                </select>
              </label>
              <label className="form-field">
                <span>信号源</span>
                <select value={accountSignalForm.sourceKey} onChange={(event) => setAccountSignalForm((current) => ({ ...current, sourceKey: event.target.value }))}>
                  {(signalCatalog?.sources ?? []).map((source) => (
                    <option key={source.key} value={source.key}>
                      {source.name}
                    </option>
                  ))}
                </select>
              </label>
              <label className="form-field">
                <span>角色</span>
                <select value={accountSignalForm.role} onChange={(event) => setAccountSignalForm((current) => ({ ...current, role: event.target.value }))}>
                  <option value="signal">信号 (Signal)</option>
                  <option value="trigger">触发器 (Trigger)</option>
                  <option value="feature">特征项 (Feature)</option>
                </select>
              </label>
              <label className="form-field">
                <span>信号周期</span>
                <select value={accountSignalForm.timeframe} onChange={(event) => setAccountSignalForm((current) => ({ ...current, timeframe: event.target.value }))}>
                  <option value="4h">4h</option>
                  <option value="1d">1d</option>
                </select>
              </label>
              <label className="form-field">
                <span>交易对 (Symbol)</span>
                <input value={accountSignalForm.symbol} onChange={(event) => setAccountSignalForm((current) => ({ ...current, symbol: event.target.value.toUpperCase() }))} />
              </label>
            </div>
            <div className="backtest-actions">
              <ActionButton label={signalBindingAction === "account" ? "绑定中..." : "执行账户绑定"} disabled={signalBindingAction !== null || !accountSignalForm.accountId} onClick={bindAccountSignalSource} />
            </div>
          </div>

          <div className="backtest-form session-form">
            <h4>绑定策略信号源</h4>
            <div className="form-grid">
              <label className="form-field">
                <span>绑定策略</span>
                <select value={strategySignalForm.strategyId} onChange={(event) => setStrategySignalForm((current) => ({ ...current, strategyId: event.target.value }))}>
                  {strategyOptions.map((strategy) => (
                    <option key={strategy.value} value={strategy.value}>
                      {strategy.label}
                    </option>
                  ))}
                </select>
              </label>
              <label className="form-field">
                <span>信号源</span>
                <select value={strategySignalForm.sourceKey} onChange={(event) => setStrategySignalForm((current) => ({ ...current, sourceKey: event.target.value }))}>
                  {(signalCatalog?.sources ?? []).map((source) => (
                    <option key={source.key} value={source.key}>
                      {source.name}
                    </option>
                  ))}
                </select>
              </label>
              <label className="form-field">
                <span>角色</span>
                <select value={strategySignalForm.role} onChange={(event) => setStrategySignalForm((current) => ({ ...current, role: event.target.value }))}>
                  <option value="signal">信号 (Signal)</option>
                  <option value="trigger">触发器 (Trigger)</option>
                  <option value="feature">特征项 (Feature)</option>
                </select>
              </label>
              <label className="form-field">
                <span>信号周期</span>
                <select value={strategySignalForm.timeframe} onChange={(event) => setStrategySignalForm((current) => ({ ...current, timeframe: event.target.value }))}>
                  <option value="4h">4h</option>
                  <option value="1d">1d</option>
                </select>
              </label>
              <label className="form-field">
                <span>交易对 (Symbol)</span>
                <input value={strategySignalForm.symbol} onChange={(event) => setStrategySignalForm((current) => ({ ...current, symbol: event.target.value.toUpperCase() }))} />
              </label>
            </div>
            <div className="backtest-actions">
              <ActionButton label={signalBindingAction === "strategy" ? "绑定中..." : "执行策略绑定"} disabled={signalBindingAction !== null || !strategySignalForm.strategyId} onClick={bindStrategySignalSource} />
            </div>
          </div>
        </div>

        <div className="live-grid">
          <div className="backtest-list">
            <h4>信号源目录</h4>
            {signalCatalog?.sources?.length ? (
              <SimpleTable
                columns={["名称", "交易所", "流类型", "角色", "环境", "传输方式"]}
                rows={signalCatalog.sources.map((source) => [
                  source.name,
                  source.exchange,
                  source.streamType,
                  source.roles.join(", "),
                  source.environments.join(", "),
                  source.transport,
                ])}
                emptyMessage="暂无信号源"
              />
            ) : (
              <div className="empty-state empty-state-compact">信号源目录为空</div>
            )}
            <div className="backtest-notes">
              {(signalCatalog?.notes ?? []).map((note) => (
                <div key={note} className="note-item">
                  {note}
                </div>
              ))}
              {(signalSourceTypes ?? []).map((item) => (
                <div key={item.streamType} className="note-item">
                  {item.streamType}: {item.description}
                </div>
              ))}
            </div>
          </div>

          <div className="backtest-list">
            <h4>当前信号绑定</h4>
            <div className="backtest-breakdown">
              <h5>账户级别</h5>
              <SimpleTable
                columns={["信号源", "角色", "代码 (Symbol)", "交易所", "状态", "操作"]}
                rows={accountSignalBindings.map((item) => [
                  item.sourceName,
                  item.role,
                  item.symbol || "--",
                  item.exchange,
                  technicalStatusLabel(item.status),
                  <ActionButton
                    key={item.id}
                    label="Unbind"
                    variant="ghost"
                    onClick={() => unbindAccountSignalSource(item.accountId || "", item.id)}
                  />
                ])}
                emptyMessage="暂无账户绑定信息"
              />
            </div>
            <div className="backtest-breakdown">
              <h5>策略级别</h5>
              <SimpleTable
                columns={["信号源", "角色", "代码 (Symbol)", "交易所", "状态", "操作"]}
                rows={strategySignalBindings.map((item) => [
                  item.sourceName,
                  item.role,
                  item.symbol || "--",
                  item.exchange,
                  technicalStatusLabel(item.status),
                  <ActionButton
                    key={item.id}
                    label="Unbind"
                    variant="ghost"
                    onClick={() => unbindStrategySignalSource(item.strategyId || "", item.id)}
                  />
                ])}
                emptyMessage="暂无策略绑定信息"
              />
            </div>
          </div>
        </div>

        <div className="live-grid">
          <div className="backtest-form session-form">
            <h4>运行时策略</h4>
            <div className="form-grid">
              <label className="form-field">
                <span>成交价格新鲜度 (秒)</span>
                <input
                  value={runtimePolicyForm.tradeTickFreshnessSeconds}
                  onChange={(event) =>
                    setRuntimePolicyForm((current) => ({ ...current, tradeTickFreshnessSeconds: event.target.value }))
                  }
                />
              </label>
              <label className="form-field">
                <span>盘口数据新鲜度 (秒)</span>
                <input
                  value={runtimePolicyForm.orderBookFreshnessSeconds}
                  onChange={(event) =>
                    setRuntimePolicyForm((current) => ({ ...current, orderBookFreshnessSeconds: event.target.value }))
                  }
                />
              </label>
              <label className="form-field">
                <span>信号 K 线新鲜度 (秒)</span>
                <input
                  value={runtimePolicyForm.signalBarFreshnessSeconds}
                  onChange={(event) =>
                    setRuntimePolicyForm((current) => ({ ...current, signalBarFreshnessSeconds: event.target.value }))
                  }
                />
              </label>
              <label className="form-field">
                <span>运行时静默期 (秒)</span>
                <input
                  value={runtimePolicyForm.runtimeQuietSeconds}
                  onChange={(event) =>
                    setRuntimePolicyForm((current) => ({ ...current, runtimeQuietSeconds: event.target.value }))
                  }
                />
              </label>
              <label className="form-field">
                <span>模拟盘启动超时 (秒)</span>
                <input
                  value={runtimePolicyForm.paperStartReadinessTimeoutSeconds}
                  onChange={(event) =>
                    setRuntimePolicyForm((current) => ({
                      ...current,
                      paperStartReadinessTimeoutSeconds: event.target.value,
                    }))
                  }
                />
              </label>
            </div>
            <div className="backtest-actions">
              <ActionButton
                label={runtimePolicyAction ? "保存中..." : "保存运行时策略"}
                disabled={runtimePolicyAction}
                onClick={updateRuntimePolicy}
              />
            </div>
            <div className="backtest-notes">
              <div className="note-item">
                活跃策略: 成交价格 {runtimePolicy?.tradeTickFreshnessSeconds ?? "--"}秒 · 盘口 {runtimePolicy?.orderBookFreshnessSeconds ?? "--"}秒 ·
                信号 K 线 {runtimePolicy?.signalBarFreshnessSeconds ?? "--"}秒
              </div>
              <div className="note-item">
                静默期 {runtimePolicy?.runtimeQuietSeconds ?? "--"}秒 · 模拟盘预检 {runtimePolicy?.paperStartReadinessTimeoutSeconds ?? "--"}秒
              </div>
            </div>
          </div>

          <div className="backtest-form session-form">
            <h4>创建信号运行时</h4>
            <div className="form-grid">
              <label className="form-field">
                <span>账户</span>
                <select value={signalRuntimeForm.accountId} onChange={(event) => setSignalRuntimeForm((current) => ({ ...current, accountId: event.target.value }))}>
                  {liveAccounts.map((item) => (
                    <option key={item.id} value={item.id}>
                      {item.name} ({technicalStatusLabel(item.mode)})
                    </option>
                  ))}
                </select>
              </label>
              <label className="form-field">
                <span>策略</span>
                <select value={signalRuntimeForm.strategyId} onChange={(event) => setSignalRuntimeForm((current) => ({ ...current, strategyId: event.target.value }))}>
                  {strategyOptions.map((strategy) => (
                    <option key={strategy.value} value={strategy.value}>
                      {strategy.label}
                    </option>
                  ))}
                </select>
              </label>
            </div>
            <div className="backtest-actions">
              <ActionButton label={signalRuntimeAction === "create" ? "创建中..." : "创建运行时会话"} disabled={signalRuntimeAction !== null || !signalRuntimeForm.accountId || !signalRuntimeForm.strategyId} onClick={createSignalRuntimeSession} />
            </div>
            <div className="detail-grid">
              <div className="detail-item">
                <span>计划就绪</span>
                <strong>{boolLabel(signalRuntimePlan?.ready)}</strong>
              </div>
              <div className="detail-item">
                <span>所需绑定</span>
                <strong>{String((signalRuntimePlan?.requiredBindings as unknown[] | undefined)?.length ?? 0)}</strong>
              </div>
              <div className="detail-item">
                <span>已匹配</span>
                <strong>{String((signalRuntimePlan?.matchedBindings as unknown[] | undefined)?.length ?? 0)}</strong>
              </div>
              <div className="detail-item">
                <span>缺失项</span>
                <strong>{String((signalRuntimePlan?.missingBindings as unknown[] | undefined)?.length ?? 0)}</strong>
              </div>
            </div>
            <div className="backtest-notes">
              <div className="note-item">运行时适配器: {signalRuntimeAdapters.map((item) => item.key).join(", ") || "--"}</div>
              {signalRuntimePlan?.missingBindings ? (
                getList(signalRuntimePlan.missingBindings).map((item, index) => (
                  <div key={index} className="note-item note-item-alert note-item-alert-warning">
                    Missing: {String(item.sourceKey)} · {String(item.role)} · {String(item.symbol)} · {String(item.timeframe)}
                  </div>
                ))
              ) : null}
              {signalRuntimePlan?.matchedBindings ? (
                getList(signalRuntimePlan.matchedBindings).map((item, index) => (
                  <div key={index} className="note-item">
                  Matched: {String(item.sourceName)} · {String(item.role)} · {String(item.symbol)}
                  </div>
                ))
              ) : null}
            </div>
          </div>
        </div>

        <div className="backtest-list mt-8 pt-8 border-t border-white/5">
          <h4 className="text-sm font-medium text-emerald-400 mb-4">运行时会话</h4>
            {signalRuntimeSessions.length > 0 ? (
              <>
                <div className="table-wrap">
                  <table>
                    <thead>
                      <tr>
                        <th>会话 ID</th>
                        <th>状态</th>
                        <th>适配器</th>
                        <th>订阅数</th>
                        <th>心跳</th>
                        <th>操作</th>
                      </tr>
                    </thead>
                    <tbody>
                      {signalRuntimeSessions.map((session) => (
                        <tr
                          key={session.id}
                          className={session.id === selectedSignalRuntime?.id ? "table-row-active" : ""}
                          onClick={() => setSelectedSignalRuntimeId(session.id)}
                        >
                          <td>{shrink(session.id)}</td>
                          <td>{technicalStatusLabel(session.status)}</td>
                          <td>{session.runtimeAdapter || "--"}</td>
                          <td>{String(session.subscriptionCount)}</td>
                          <td>{formatTime(String(session.state?.lastHeartbeatAt ?? ""))}</td>
                          <td>
                            <div className="inline-actions">
                              <ActionButton
                                label={signalRuntimeAction === `${session.id}:start` ? "启动中..." : "启动"}
                                disabled={signalRuntimeAction !== null || session.status === "RUNNING"}
                                onClick={() => runSignalRuntimeAction(session.id, "start")}
                              />
                              <ActionButton
                                label={signalRuntimeAction === `${session.id}:stop` ? "停止中..." : "停止"}
                                variant="ghost"
                                disabled={signalRuntimeAction !== null || session.status === "STOPPED"}
                                onClick={() => runSignalRuntimeAction(session.id, "stop")}
                              />
                              <ActionButton
                                label="删除"
                                variant="ghost"
                                disabled={signalRuntimeAction !== null}
                                onClick={() => deleteSignalRuntimeSession(session.id)}
                              />
                            </div>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>

                <div className="backtest-detail-card">
                  <div className="flex items-center justify-between mb-4 mt-8 pt-8 border-t border-white/5">
                    <h5 className="text-sm font-medium text-emerald-400">选中运行时细节</h5>
                    <div className="flex items-center space-x-3 text-[10px] text-zinc-500 bg-white/5 px-2 py-1 rounded-md">
                      <span>状态: {selectedSignalRuntime?.status ?? "未选择"}</span>
                      <span className="opacity-30">|</span>
                      <span>适配器: {selectedSignalRuntime?.runtimeAdapter ?? "--"}</span>
                    </div>
                  </div>
                  {selectedSignalRuntime ? (
                    <>
                      <div className="detail-grid">
                        <div className="detail-item" title={selectedSignalRuntime.id}>
                          <span>会话 ID</span>
                          <strong>{shrink(selectedSignalRuntime.id)}</strong>
                        </div>
                        <div className="detail-item" title={selectedSignalRuntime.accountId}>
                          <span>关联账户</span>
                          <strong>{shrink(selectedSignalRuntime.accountId)}</strong>
                        </div>
                        <div className="detail-item" title={selectedSignalRuntime.strategyId}>
                          <span>执行策略</span>
                          <strong>{shrink(selectedSignalRuntime.strategyId)}</strong>
                        </div>
                        <div className="detail-item">
                          <span>传输协议</span>
                          <strong>{selectedSignalRuntime.transport || "--"}</strong>
                        </div>
                        <div className="detail-item">
                          <span>健康状态</span>
                          <strong>{String(selectedSignalRuntimeState.health ?? "--")}</strong>
                        </div>
                        <div className="detail-item">
                          <span>信号事件数</span>
                          <strong>{String(Math.trunc(getNumber(selectedSignalRuntimeState.signalEventCount) ?? 0))}</strong>
                        </div>
                        <div className="detail-item">
                          <span>最后心跳</span>
                          <strong>{formatTime(String(selectedSignalRuntimeState.lastHeartbeatAt ?? ""))}</strong>
                        </div>
                        <div className="detail-item">
                          <span>最后事件</span>
                          <strong>{formatTime(String(selectedSignalRuntimeState.lastEventAt ?? ""))}</strong>
                        </div>
                        <div className="detail-item">
                          <span>资源状态</span>
                          <strong>{String(Object.keys(selectedSignalRuntimeSourceStates).length)}</strong>
                        </div>
                        <div className="detail-item">
                          <span>运行计划</span>
                          <strong>{boolLabel(selectedSignalRuntimePlan.ready)}</strong>
                        </div>
                      </div>

                      <div className="space-y-6 mt-6">
                        <div className="panel-compact bg-white/5 rounded-2xl p-4 border border-white/5">
                          <h4 className="text-xs font-bold text-zinc-500 uppercase tracking-widest mb-4">信号源订阅</h4>
                          <SimpleTable
                            columns={["来源", "角色", "交易对", "频道", "适配器"]}
                            rows={selectedSignalRuntimeSubscriptions.map((item) => [
                              String(item.sourceKey ?? "--"),
                              String(item.role ?? "--"),
                              String(item.symbol ?? "--"),
                              String(item.channel ?? "--"),
                              String(item.adapterKey ?? "--"),
                            ])}
                            emptyMessage="暂无订阅信息"
                          />
                        </div>

                        <div className="panel-compact bg-white/5 rounded-2xl p-4 border border-white/5">
                          <h4 className="text-xs font-bold text-zinc-500 uppercase tracking-widest mb-4">信号 K 线 (Signal Bars)</h4>
                          {selectedSignalRuntimeSignalBars.length > 0 ? (
                            <div className="chart-shell bg-transparent border-none p-0 min-height-0">
                              <SignalBarChart candles={selectedSignalRuntimeSignalBars} />
                            </div>
                          ) : (
                            <div className="empty-state empty-state-compact">尚无缓存的 4h/1d 信号 K 线</div>
                          )}
                        </div>

                        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                          <div className="panel-compact bg-white/5 rounded-2xl p-4 border border-white/5">
                            <h4 className="text-xs font-bold text-zinc-500 uppercase tracking-widest mb-4">信号状态 (Signal States)</h4>
                            <div className="space-y-2 max-h-[300px] overflow-y-auto pr-2 custom-scrollbar">
                              {Object.entries(selectedSignalBarStates).length > 0 ? (
                                Object.entries(selectedSignalBarStates).map(([key, value]) => {
                                  const state = getRecord(value);
                                  const current = getRecord(state.current);
                                  return (
                                    <div key={key} className="note-item bg-white/5 border border-white/5 text-[10px] leading-relaxed">
                                      <span className="text-emerald-400 font-bold">{key}</span> · {String(state.timeframe)} · {formatMaybeNumber(state.ma20)} · {formatMaybeNumber(current.close)}
                                    </div>
                                  );
                                })
                              ) : (
                                <div className="empty-state empty-state-compact">暂无信号状态数据</div>
                              )}
                            </div>
                          </div>

                          <div className="panel-compact bg-white/5 rounded-2xl p-4 border border-white/5">
                            <h4 className="text-xs font-bold text-zinc-500 uppercase tracking-widest mb-4">时间线</h4>
                            <div className="space-y-2 max-h-[300px] overflow-y-auto pr-2 custom-scrollbar">
                              {buildTimelineNotes(selectedSignalRuntimeTimeline).map((line: string) => (
                                <div key={line} className="note-item bg-white/5 border border-white/5 text-[10px] leading-relaxed">
                                  {line}
                                </div>
                              ))}
                            </div>
                          </div>

                          <div className="panel-compact bg-white/5 rounded-2xl p-4 border border-white/5">
                            <h4 className="text-xs font-bold text-zinc-500 uppercase tracking-widest mb-4">最后事件摘要</h4>
                            <div className="space-y-2 max-h-[300px] overflow-y-auto pr-2 custom-scrollbar">
                              {Object.entries(selectedSignalRuntimeLastSummary).length > 0 ? (
                                Object.entries(selectedSignalRuntimeLastSummary).map(([key, value]) => (
                                  <div key={key} className="note-item bg-white/5 border border-white/5 text-[10px] leading-relaxed">
                                    <span className="text-zinc-500">{key}:</span> {typeof value === "object" ? JSON.stringify(value) : String(value)}
                                  </div>
                                ))
                              ) : (
                                <div className="empty-state empty-state-compact">暂无事件摘要</div>
                              )}
                            </div>
                          </div>

                          <div className="panel-compact bg-white/5 rounded-2xl p-4 border border-white/5">
                            <h4 className="text-xs font-bold text-zinc-500 uppercase tracking-widest mb-4">源数据状态 (Source States)</h4>
                            <div className="space-y-2 max-h-[300px] overflow-y-auto pr-2 custom-scrollbar">
                              {Object.entries(selectedSignalRuntimeSourceStates).length > 0 ? (
                                Object.entries(selectedSignalRuntimeSourceStates).slice(0, 12).map(([key, value]) => (
                                  <div key={key} className="note-item bg-white/5 border border-white/5 text-[10px] leading-relaxed">
                                    <span className="text-zinc-500">{key}:</span> {typeof value === "object" ? JSON.stringify(value) : String(value)}
                                  </div>
                                ))
                              ) : (
                                <div className="empty-state empty-state-compact">暂无源数据状态</div>
                              )}
                            </div>
                          </div>
                        </div>
                      </div>
                    </>
                  ) : (
                    <div className="empty-state empty-state-compact">未选择运行时会话</div>
                  )}
                </div>
              </>
            ) : (
              <div className="empty-state empty-state-compact">暂无运行时会话</div>
            )}
          </div>
        </section>
      </div>
    );
}
