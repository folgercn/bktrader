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
  getNumber
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
        <div className="hero-side">
          <div className="hero-pill">{loading ? "加载中..." : `${liveAccounts.length} 个账户`}</div>
          <div className="hero-pill hero-pill-accent">{monitorMode}</div>
          <div className="hero-user-card">
            <div>
              <strong>{authSession?.username ?? "未登录"}</strong>
              <p>{authSession?.expiresAt ? `有效至 ${formatTime(authSession.expiresAt)}` : "登录后即可加载账户与监控"}</p>
            </div>
            {authSession ? (
              <button type="button" className="hero-menu-button hero-logout" onClick={logout}>
                退出
              </button>
            ) : null}
          </div>
          <div className="hero-menu">
            <button
              type="button"
              className="hero-menu-button"
              onClick={() => setSettingsMenuOpen((current) => !current)}
            >
              设置
            </button>
            {settingsMenuOpen ? (
              <div className="hero-menu-popover">
                <button
                  type="button"
                  className="hero-menu-item"
                  onClick={() => {
                    openLiveAccountModal();
                    setSettingsMenuOpen(false);
                  }}
                >
                  新建账户
                </button>
                <button
                  type="button"
                  className="hero-menu-item"
                  onClick={() => {
                    if (quickLiveAccountId) {
                      selectQuickLiveAccount(quickLiveAccountId);
                    }
                    openLiveBindingModal();
                    setSettingsMenuOpen(false);
                  }}
                >
                  绑定账户
                </button>
                <button
                  type="button"
                  className="hero-menu-item"
                  onClick={() => {
                    setActiveSettingsModal("telegram");
                    setSettingsMenuOpen(false);
                  }}
                >
                  Telegram 通知
                </button>
              </div>
            ) : null}
          </div>
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
                <span>{shrink(highlightedLiveSession.session.id)}</span>
                <span>{highlightedLiveSession.session.accountId}</span>
                <span>{highlightedLiveSession.session.strategyId}</span>
                <span>{String(highlightedLiveSession.session.state?.signalTimeframe ?? "--")}</span>
              </div>
              <div className="backtest-notes">
                <div className="note-item">health: {highlightedLiveSession.health.detail}</div>
                <div className="note-item">
                  recovery: {String(highlightedLiveSession.session.state?.positionRecoveryStatus ?? "--")} · protection{" "}
                  {String(highlightedLiveSession.session.state?.protectionRecoveryStatus ?? "--")} · orders{" "}
                  {String(highlightedLiveSession.session.state?.recoveredProtectionCount ?? "--")}
                </div>
                <div className="note-item">
                  execution: orders {highlightedLiveSession.execution.orderCount} · fills {highlightedLiveSession.execution.fillCount}
                </div>
                <div className="note-item">
                  latest-order: {String(highlightedLiveSession.execution.latestOrder?.status ?? "--")} · {String(highlightedLiveSession.execution.latestOrder?.side ?? "--")} · {formatMaybeNumber(highlightedLiveSession.execution.latestOrder?.price)}
                </div>
                <div className="note-item">
                  position: {String(highlightedLiveSession.execution.position?.side ?? "FLAT")} · {formatMaybeNumber(highlightedLiveSession.execution.position?.quantity)} @ {formatMaybeNumber(highlightedLiveSession.execution.position?.entryPrice)}
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
            <h4>Live Strategy Sessions</h4>
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
                          label="Edit"
                          variant="ghost"
                          disabled={liveSessionAction !== null || liveSessionDeleteAction !== null}
                          onClick={() => openLiveSessionModal(session)}
                        />
                        {String(session.state?.signalRuntimeSessionId ?? "") ? (
                          <ActionButton
                            label="Open Runtime"
                            variant="ghost"
                            onClick={() => jumpToSignalRuntimeSession(String(session.state?.signalRuntimeSessionId ?? ""))}
                          />
                        ) : null}
                        <ActionButton
                          label={liveSessionAction === `${session.id}:start` ? "Starting..." : "Start"}
                          disabled={liveSessionAction !== null || session.status === "RUNNING" || !sessionAccountReady}
                          onClick={() => runLiveSessionAction(session.id, "start")}
                        />
                        {!sessionAccountReady ? (
                          <ActionButton
                            label="Bind Adapter"
                            variant="ghost"
                            disabled={liveSessionAction !== null || liveSessionDeleteAction !== null}
                            onClick={() => {
                              selectQuickLiveAccount(session.accountId);
                              openLiveBindingModal();
                            }}
                          />
                        ) : null}
                        <ActionButton
                          label={liveSessionAction === `${session.id}:dispatch` ? "Dispatching..." : "Dispatch Intent"}
                          disabled={
                            liveSessionAction !== null ||
                            !getRecord(session.state?.lastStrategyIntent).action ||
                            String(session.state?.dispatchMode ?? "") !== "manual-review" ||
                            (primaryLiveSession?.id === session.id && primaryLiveDispatchPreview.status === "blocked")
                          }
                          onClick={() => dispatchLiveSessionIntent(session.id)}
                        />
                        <ActionButton
                          label={liveSessionAction === `${session.id}:sync` ? "Syncing..." : "Sync Latest Order"}
                          variant="ghost"
                          disabled={liveSessionAction !== null || !String(session.state?.lastDispatchedOrderId ?? "")}
                          onClick={() => syncLiveSession(session.id)}
                        />
                        <ActionButton
                          label={liveSessionAction === `${session.id}:stop` ? "Stopping..." : "Stop"}
                          variant="ghost"
                          disabled={liveSessionAction !== null || session.status === "STOPPED"}
                          onClick={() => runLiveSessionAction(session.id, "stop")}
                        />
                        <ActionButton
                          label={liveSessionDeleteAction === session.id ? "Deleting..." : "Delete"}
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
              <div className="empty-state empty-state-compact">No valid live sessions yet</div>
            )}
            {primaryLiveSession ? (
              <div className="backtest-notes">
                <div className="note-item">
                  runtime: {String(primaryLiveSession.state?.signalRuntimeStatus ?? "--")} · {formatTime(String(primaryLiveSession.state?.lastSignalRuntimeEventAt ?? ""))}
                </div>
                <div className="note-item">
                  market: {formatMaybeNumber(primaryLiveSessionMarket.tradePrice)} · {formatMaybeNumber(primaryLiveSessionMarket.bestBid)} / {formatMaybeNumber(primaryLiveSessionMarket.bestAsk)}
                </div>
                <div className="note-item">
                  readiness: {primaryLiveSessionRuntimeReadiness.status} · {primaryLiveSessionRuntimeReadiness.reason}
                </div>
                <div className="note-item">
                  intent: {String(primaryLiveSessionIntent.action ?? "none")} · {String(primaryLiveSessionIntent.side ?? "--")} · {formatMaybeNumber(primaryLiveSessionIntent.priceHint)}
                </div>
                <div className="note-item">
                  intent-preview: qty {formatMaybeNumber(primaryLiveSessionIntent.quantity)} · src {String(primaryLiveSessionIntent.priceSource ?? "--")} · kind {String(primaryLiveSessionIntent.signalKind ?? "--")}
                </div>
                <div className="note-item">
                  intent-context: spread {formatMaybeNumber(primaryLiveSessionIntent.spreadBps)} bps · bias {String(primaryLiveSessionIntent.liquidityBias ?? "--")} · ma20 {formatMaybeNumber(primaryLiveSessionIntent.ma20)} · atr14 {formatMaybeNumber(primaryLiveSessionIntent.atr14)}
                </div>
                <div className="note-item">
                  signal-filter: tf {String(primaryLiveSessionSignalBarDecision.timeframe ?? "--")} · sma5 {formatMaybeNumber(primaryLiveSessionSignalBarDecision.sma5)} · early-long{" "}
                  {boolLabel(primaryLiveSessionSignalBarDecision.longEarlyReversalReady)} · early-short {boolLabel(primaryLiveSessionSignalBarDecision.shortEarlyReversalReady)} ·{" "}
                  {String(primaryLiveSessionSignalBarDecision.reason ?? "--")}
                </div>
                <div className="note-item">
                  dispatch: {String(primaryLiveSession?.state?.dispatchMode ?? "--")} · cooldown {String(primaryLiveSession?.state?.dispatchCooldownSeconds ?? "--")}s · last-order {String(primaryLiveSession?.state?.lastDispatchedOrderId ?? "--")}
                </div>
                <div className="note-item">
                  execution-profile: {String(getRecord(primaryLiveSession?.state?.lastExecutionProfile).executionProfile ?? "--")} ·{" "}
                  {String(getRecord(primaryLiveSession?.state?.lastExecutionProfile).orderType ?? "--")} · tif{" "}
                  {String(getRecord(primaryLiveSession?.state?.lastExecutionProfile).timeInForce ?? "--")} · postOnly{" "}
                  {boolLabel(getRecord(primaryLiveSession?.state?.lastExecutionProfile).postOnly)} · reduceOnly{" "}
                  {boolLabel(getRecord(primaryLiveSession?.state?.lastExecutionProfile).reduceOnly)}
                </div>
                <div className="note-item">
                  execution-telemetry: {String(getRecord(primaryLiveSession?.state?.lastExecutionTelemetry).decision ?? "--")} · spread{" "}
                  {formatMaybeNumber(getRecord(getRecord(primaryLiveSession?.state?.lastExecutionTelemetry).book).spreadBps)} bps · imbalance{" "}
                  {formatMaybeNumber(getRecord(getRecord(primaryLiveSession?.state?.lastExecutionTelemetry).book).bookImbalance)}
                </div>
                <div className="note-item">
                  execution-dispatch: {String(getRecord(primaryLiveSession?.state?.lastExecutionDispatch).status ?? "--")} ·{" "}
                  {String(getRecord(primaryLiveSession?.state?.lastExecutionDispatch).executionMode ?? "--")} · fallback{" "}
                  {boolLabel(getRecord(primaryLiveSession?.state?.lastExecutionDispatch).fallback)} ·{" "}
                  {String(getRecord(primaryLiveSession?.state?.lastExecutionDispatch).fallbackOrderType ?? "--")}
                </div>
                <div className="note-item">
                  execution-fill: expected {formatMaybeNumber(getRecord(primaryLiveSession?.state?.lastExecutionDispatch).expectedPrice)} · drift{" "}
                  {formatMaybeNumber(getRecord(primaryLiveSession?.state?.lastExecutionDispatch).priceDriftBps)} bps
                </div>
                <div className="note-item">
                  execution-event-stats: proposals {String(getRecord(primaryLiveSession?.state?.executionEventStats).proposalCount ?? "--")} · maker{" "}
                  {String(getRecord(primaryLiveSession?.state?.executionEventStats).makerRestingDecisionCount ?? "--")} · fallback{" "}
                  {String(getRecord(primaryLiveSession?.state?.executionEventStats).fallbackDispatchCount ?? "--")} · avg drift{" "}
                  {formatMaybeNumber(getRecord(primaryLiveSession?.state?.executionEventStats).avgPriceDriftBps)} bps
                </div>
                <div className="note-item">
                  auto-dispatch: last-at {formatTime(String(primaryLiveSession?.state?.lastDispatchedAt ?? ""))} · last-error {String(primaryLiveSession?.state?.lastAutoDispatchError ?? "--")}
                </div>
                <div className="note-item">
                  sync: {String(primaryLiveSession?.state?.lastSyncedOrderStatus ?? "--")} · {formatTime(String(primaryLiveSession?.state?.lastSyncedAt ?? ""))} · error {String(primaryLiveSession?.state?.lastSyncError ?? "--")}
                </div>
                <div className="note-item">
                  recovery: {String(primaryLiveSession?.state?.lastRecoveryStatus ?? "--")} · position {String(primaryLiveSession?.state?.positionRecoveryStatus ?? "--")} · protection{" "}
                  {String(primaryLiveSession?.state?.protectionRecoveryStatus ?? "--")}
                </div>
                <div className="note-item">
                  recovery-detail: last-at {formatTime(String(primaryLiveSession?.state?.lastRecoveryAttemptAt ?? primaryLiveSession?.state?.lastProtectionRecoveryAt ?? ""))} · protection-orders{" "}
                  {String(primaryLiveSession?.state?.recoveredProtectionCount ?? "--")} · stop {String(primaryLiveSession?.state?.recoveredStopOrderCount ?? "--")} · take-profit{" "}
                  {String(primaryLiveSession?.state?.recoveredTakeProfitOrderCount ?? "--")}
                </div>
                <div className="note-item">
                  execution: orders {primaryLiveExecutionSummary.orderCount} · fills {primaryLiveExecutionSummary.fillCount} · latest-order {String(primaryLiveExecutionSummary.latestOrder?.status ?? "--")}
                </div>
                <div className="note-item">
                  position: {String(primaryLiveExecutionSummary.position?.side ?? "FLAT")} · {formatMaybeNumber(primaryLiveExecutionSummary.position?.quantity)} @ {formatMaybeNumber(primaryLiveExecutionSummary.position?.entryPrice)} · mark {formatMaybeNumber(primaryLiveExecutionSummary.position?.markPrice)}
                </div>
                <div className="note-item">
                  recovered-position: {String(getRecord(primaryLiveSession?.state?.recoveredPosition).side ?? "FLAT")} ·{" "}
                  {formatMaybeNumber(getRecord(primaryLiveSession?.state?.recoveredPosition).quantity)} @{" "}
                  {formatMaybeNumber(getRecord(primaryLiveSession?.state?.recoveredPosition).entryPrice)}
                </div>
                <div className="note-item">
                  dispatch-preview: {primaryLiveDispatchPreview.reason} · {primaryLiveDispatchPreview.detail}
                </div>
                <div className="note-item">
                  final-order: {String(primaryLiveDispatchPreview.payload.side ?? "--")} {formatMaybeNumber(primaryLiveDispatchPreview.payload.quantity)} {String(primaryLiveDispatchPreview.payload.symbol ?? "--")} · {String(primaryLiveDispatchPreview.payload.type ?? "--")} @ {formatMaybeNumber(primaryLiveDispatchPreview.payload.price)}
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
            <h4>Live Accounts</h4>
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
                        <span>{account.exchange}</span>
                        <span>{String(binding.adapterKey ?? "--")}</span>
                        <span>{String(binding.positionMode ?? "--")} / {String(binding.marginMode ?? "--")}</span>
                      </div>
                      <div className="live-account-meta">
                        <span>sync {String(syncSnapshot.syncStatus ?? "UNSYNCED")}</span>
                        <span>{formatTime(String(getRecord(account.metadata).lastLiveSyncAt ?? ""))}</span>
                        <span>{String(syncSnapshot.source ?? "--")}</span>
                      </div>
                      <div className="live-account-meta">
                        <span>{bindings.length} signal bindings</span>
                        <span>{runtimeSessionsForAccount.length} runtime sessions</span>
                        <span>{activeRuntime ? `${activeRuntime.status} · ${String(activeRuntimeState.health ?? "--")}` : "no runtime"}</span>
                      </div>
                      <div className="live-account-meta">
                        <span>{String(activeRuntimeSummary.event ?? "--")}</span>
                        <span>{formatTime(String(activeRuntimeState.lastHeartbeatAt ?? ""))}</span>
                        <span>{formatTime(String(activeRuntimeState.lastEventAt ?? ""))}</span>
                      </div>
                      <div className="live-account-meta">
                        <span>trade {formatMaybeNumber(activeRuntimeMarket.tradePrice)}</span>
                        <span>bid/ask {formatMaybeNumber(activeRuntimeMarket.bestBid)} / {formatMaybeNumber(activeRuntimeMarket.bestAsk)}</span>
                        <span>spread {formatMaybeNumber(activeRuntimeMarket.spreadBps)} bps</span>
                      </div>
                      <div className="live-account-meta">
                        <span>tick {activeRuntimeSourceSummary.tradeTickCount}</span>
                        <span>book {activeRuntimeSourceSummary.orderBookCount}</span>
                        <span>stale {activeRuntimeSourceSummary.staleCount}</span>
                        <span>{formatTime(String(activeRuntimeSourceSummary.latestEventAt ?? ""))}</span>
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
                        <span>next action</span>
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
                          {liveFlowAction === account.id ? "Launching..." : "Launch Live Flow"}
                        </button>
                        <button
                          type="button"
                          className="filter-chip"
                          onClick={() => runLiveNextAction(account, liveNextAction, activeRuntime)}
                        >
                          Open
                        </button>
                      </div>
                      <div className="live-account-meta">
                        <span>{String(activeSignalBarState.timeframe ?? "--")}</span>
                        <span>ma20 {formatMaybeNumber(activeSignalBarState.ma20)}</span>
                        <span>atr14 {formatMaybeNumber(activeSignalBarState.atr14)}</span>
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
                          live-preflight: {livePreflight.reason} · {livePreflight.detail}
                        </div>
                        <div className="note-item">
                          next-action: {liveNextAction.label} · {liveNextAction.detail}
                        </div>
                        <div className="note-item">
                          account-sync: orders {String(syncSnapshot.orderCount ?? "--")} · fills {String(syncSnapshot.fillCount ?? "--")} · positions {String(syncSnapshot.positionCount ?? "--")}
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
                          label={liveAccountSyncAction === account.id ? "Syncing..." : "Sync Account"}
                          variant="ghost"
                          disabled={liveAccountSyncAction !== null}
                          onClick={() => syncLiveAccount(account.id)}
                        />
                        {activeRuntime ? (
                          <ActionButton
                            label="Open Runtime"
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
              <div className="empty-state empty-state-compact">No live accounts yet</div>
            )}
          </div>

          <div className="backtest-list">
            <h4>Accepted Live Orders</h4>
            {syncableLiveOrders.length > 0 ? (
              <SimpleTable
                columns={["Order", "Account", "Symbol", "Side", "Qty", "Status", "Action"]}
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
                emptyMessage="No accepted live orders"
              />
            ) : (
              <div className="empty-state empty-state-compact">No accepted live orders</div>
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
            <span>{signalCatalog?.sources?.length ?? 0} sources</span>
            <span>{signalRuntimeSessions.length} sessions</span>
          </div>
        </div>

        <div className="live-grid">
          <div className="backtest-form session-form">
            <h4>Bind Account Signal Source</h4>
            <div className="form-grid">
              <label className="form-field">
                <span>Account</span>
                <select value={accountSignalForm.accountId} onChange={(event) => setAccountSignalForm((current) => ({ ...current, accountId: event.target.value }))}>
                  {liveAccounts.map((item) => (
                    <option key={item.id} value={item.id}>
                      {item.name} ({item.mode})
                    </option>
                  ))}
                </select>
              </label>
              <label className="form-field">
                <span>Source</span>
                <select value={accountSignalForm.sourceKey} onChange={(event) => setAccountSignalForm((current) => ({ ...current, sourceKey: event.target.value }))}>
                  {(signalCatalog?.sources ?? []).map((source) => (
                    <option key={source.key} value={source.key}>
                      {source.name}
                    </option>
                  ))}
                </select>
              </label>
              <label className="form-field">
                <span>Role</span>
                <select value={accountSignalForm.role} onChange={(event) => setAccountSignalForm((current) => ({ ...current, role: event.target.value }))}>
                  <option value="signal">signal</option>
                  <option value="trigger">trigger</option>
                  <option value="feature">feature</option>
                </select>
              </label>
              <label className="form-field">
                <span>Timeframe</span>
                <select value={accountSignalForm.timeframe} onChange={(event) => setAccountSignalForm((current) => ({ ...current, timeframe: event.target.value }))}>
                  <option value="4h">4h</option>
                  <option value="1d">1d</option>
                </select>
              </label>
              <label className="form-field">
                <span>Symbol</span>
                <input value={accountSignalForm.symbol} onChange={(event) => setAccountSignalForm((current) => ({ ...current, symbol: event.target.value.toUpperCase() }))} />
              </label>
            </div>
            <div className="backtest-actions">
              <ActionButton label={signalBindingAction === "account" ? "Binding..." : "Bind Account Source"} disabled={signalBindingAction !== null || !accountSignalForm.accountId} onClick={bindAccountSignalSource} />
            </div>
          </div>

          <div className="backtest-form session-form">
            <h4>Bind Strategy Signal Source</h4>
            <div className="form-grid">
              <label className="form-field">
                <span>Strategy</span>
                <select value={strategySignalForm.strategyId} onChange={(event) => setStrategySignalForm((current) => ({ ...current, strategyId: event.target.value }))}>
                  {strategyOptions.map((strategy) => (
                    <option key={strategy.value} value={strategy.value}>
                      {strategy.label}
                    </option>
                  ))}
                </select>
              </label>
              <label className="form-field">
                <span>Source</span>
                <select value={strategySignalForm.sourceKey} onChange={(event) => setStrategySignalForm((current) => ({ ...current, sourceKey: event.target.value }))}>
                  {(signalCatalog?.sources ?? []).map((source) => (
                    <option key={source.key} value={source.key}>
                      {source.name}
                    </option>
                  ))}
                </select>
              </label>
              <label className="form-field">
                <span>Role</span>
                <select value={strategySignalForm.role} onChange={(event) => setStrategySignalForm((current) => ({ ...current, role: event.target.value }))}>
                  <option value="signal">signal</option>
                  <option value="trigger">trigger</option>
                  <option value="feature">feature</option>
                </select>
              </label>
              <label className="form-field">
                <span>Timeframe</span>
                <select value={strategySignalForm.timeframe} onChange={(event) => setStrategySignalForm((current) => ({ ...current, timeframe: event.target.value }))}>
                  <option value="4h">4h</option>
                  <option value="1d">1d</option>
                </select>
              </label>
              <label className="form-field">
                <span>Symbol</span>
                <input value={strategySignalForm.symbol} onChange={(event) => setStrategySignalForm((current) => ({ ...current, symbol: event.target.value.toUpperCase() }))} />
              </label>
            </div>
            <div className="backtest-actions">
              <ActionButton label={signalBindingAction === "strategy" ? "Binding..." : "Bind Strategy Source"} disabled={signalBindingAction !== null || !strategySignalForm.strategyId} onClick={bindStrategySignalSource} />
            </div>
          </div>
        </div>

        <div className="live-grid">
          <div className="backtest-list">
            <h4>Signal Source Catalog</h4>
            {signalCatalog?.sources?.length ? (
              <SimpleTable
                columns={["Source", "Exchange", "Type", "Roles", "Env", "Transport"]}
                rows={signalCatalog.sources.map((source) => [
                  source.name,
                  source.exchange,
                  source.streamType,
                  source.roles.join(", "),
                  source.environments.join(", "),
                  source.transport,
                ])}
                emptyMessage="No signal sources"
              />
            ) : (
              <div className="empty-state empty-state-compact">No signal source catalog</div>
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
            <h4>Current Bindings</h4>
            <div className="backtest-breakdown">
              <h5>Account</h5>
              <SimpleTable
                columns={["Source", "Role", "Symbol", "Exchange", "Status", "Action"]}
                rows={accountSignalBindings.map((item) => [
                  item.sourceName,
                  item.role,
                  item.symbol || "--",
                  item.exchange,
                  item.status,
                  <ActionButton
                    key={item.id}
                    label="Unbind"
                    variant="ghost"
                    onClick={() => unbindAccountSignalSource(item.accountId || "", item.id)}
                  />
                ])}
                emptyMessage="No account bindings"
              />
            </div>
            <div className="backtest-breakdown">
              <h5>Strategy</h5>
              <SimpleTable
                columns={["Source", "Role", "Symbol", "Exchange", "Status", "Action"]}
                rows={strategySignalBindings.map((item) => [
                  item.sourceName,
                  item.role,
                  item.symbol || "--",
                  item.exchange,
                  item.status,
                  <ActionButton
                    key={item.id}
                    label="Unbind"
                    variant="ghost"
                    onClick={() => unbindStrategySignalSource(item.strategyId || "", item.id)}
                  />
                ])}
                emptyMessage="No strategy bindings"
              />
            </div>
          </div>
        </div>

        <div className="live-grid">
          <div className="backtest-form session-form">
            <h4>Runtime Policy</h4>
            <div className="form-grid">
              <label className="form-field">
                <span>Trade Tick Freshness (s)</span>
                <input
                  value={runtimePolicyForm.tradeTickFreshnessSeconds}
                  onChange={(event) =>
                    setRuntimePolicyForm((current) => ({ ...current, tradeTickFreshnessSeconds: event.target.value }))
                  }
                />
              </label>
              <label className="form-field">
                <span>Order Book Freshness (s)</span>
                <input
                  value={runtimePolicyForm.orderBookFreshnessSeconds}
                  onChange={(event) =>
                    setRuntimePolicyForm((current) => ({ ...current, orderBookFreshnessSeconds: event.target.value }))
                  }
                />
              </label>
              <label className="form-field">
                <span>Signal Bar Freshness (s)</span>
                <input
                  value={runtimePolicyForm.signalBarFreshnessSeconds}
                  onChange={(event) =>
                    setRuntimePolicyForm((current) => ({ ...current, signalBarFreshnessSeconds: event.target.value }))
                  }
                />
              </label>
              <label className="form-field">
                <span>Runtime Quiet (s)</span>
                <input
                  value={runtimePolicyForm.runtimeQuietSeconds}
                  onChange={(event) =>
                    setRuntimePolicyForm((current) => ({ ...current, runtimeQuietSeconds: event.target.value }))
                  }
                />
              </label>
              <label className="form-field">
                <span>Paper Start Timeout (s)</span>
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
                label={runtimePolicyAction ? "Saving..." : "Save Runtime Policy"}
                disabled={runtimePolicyAction}
                onClick={updateRuntimePolicy}
              />
            </div>
            <div className="backtest-notes">
              <div className="note-item">
                active policy: tick {runtimePolicy?.tradeTickFreshnessSeconds ?? "--"}s · book {runtimePolicy?.orderBookFreshnessSeconds ?? "--"}s ·
                bar {runtimePolicy?.signalBarFreshnessSeconds ?? "--"}s
              </div>
              <div className="note-item">
                quiet {runtimePolicy?.runtimeQuietSeconds ?? "--"}s · paper preflight {runtimePolicy?.paperStartReadinessTimeoutSeconds ?? "--"}s
              </div>
            </div>
          </div>

          <div className="backtest-form session-form">
            <h4>Create Runtime Session</h4>
            <div className="form-grid">
              <label className="form-field">
                <span>Account</span>
                <select value={signalRuntimeForm.accountId} onChange={(event) => setSignalRuntimeForm((current) => ({ ...current, accountId: event.target.value }))}>
                  {liveAccounts.map((item) => (
                    <option key={item.id} value={item.id}>
                      {item.name} ({item.mode})
                    </option>
                  ))}
                </select>
              </label>
              <label className="form-field">
                <span>Strategy</span>
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
              <ActionButton label={signalRuntimeAction === "create" ? "Creating..." : "Create Runtime Session"} disabled={signalRuntimeAction !== null || !signalRuntimeForm.accountId || !signalRuntimeForm.strategyId} onClick={createSignalRuntimeSession} />
            </div>
            <div className="detail-grid">
              <div className="detail-item">
                <span>Plan Ready</span>
                <strong>{boolLabel(signalRuntimePlan?.ready)}</strong>
              </div>
              <div className="detail-item">
                <span>Required</span>
                <strong>{String((signalRuntimePlan?.requiredBindings as unknown[] | undefined)?.length ?? 0)}</strong>
              </div>
              <div className="detail-item">
                <span>Matched</span>
                <strong>{String((signalRuntimePlan?.matchedBindings as unknown[] | undefined)?.length ?? 0)}</strong>
              </div>
              <div className="detail-item">
                <span>Missing</span>
                <strong>{String((signalRuntimePlan?.missingBindings as unknown[] | undefined)?.length ?? 0)}</strong>
              </div>
            </div>
            <div className="backtest-notes">
              <div className="note-item">runtime adapters: {signalRuntimeAdapters.map((item) => item.key).join(", ") || "--"}</div>
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

          <div className="backtest-list">
            <h4>Runtime Sessions</h4>
            {signalRuntimeSessions.length > 0 ? (
              <>
                <div className="table-wrap">
                  <table>
                    <thead>
                      <tr>
                        <th>Session</th>
                        <th>Status</th>
                        <th>Adapter</th>
                        <th>Subs</th>
                        <th>Heartbeat</th>
                        <th>Action</th>
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
                          <td>{session.status}</td>
                          <td>{session.runtimeAdapter || "--"}</td>
                          <td>{String(session.subscriptionCount)}</td>
                          <td>{formatTime(String(session.state?.lastHeartbeatAt ?? ""))}</td>
                          <td>
                            <div className="inline-actions">
                              <ActionButton
                                label={signalRuntimeAction === `${session.id}:start` ? "Starting..." : "Start"}
                                disabled={signalRuntimeAction !== null || session.status === "RUNNING"}
                                onClick={() => runSignalRuntimeAction(session.id, "start")}
                              />
                              <ActionButton
                                label={signalRuntimeAction === `${session.id}:stop` ? "Stopping..." : "Stop"}
                                variant="ghost"
                                disabled={signalRuntimeAction !== null || session.status === "STOPPED"}
                                onClick={() => runSignalRuntimeAction(session.id, "stop")}
                              />
                              <ActionButton
                                label="Delete"
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
                  <div className="panel-header">
                    <div>
                      <p className="panel-kicker">Signal Session</p>
                      <h3>选中 Runtime Session 详情</h3>
                    </div>
                    <div className="range-box">
                      <span>{selectedSignalRuntime?.status ?? "NO SESSION"}</span>
                      <span>{selectedSignalRuntime?.runtimeAdapter ?? "--"}</span>
                    </div>
                  </div>
                  {selectedSignalRuntime ? (
                    <>
                      <div className="detail-grid">
                        <div className="detail-item">
                          <span>Session ID</span>
                          <strong>{shrink(selectedSignalRuntime.id)}</strong>
                        </div>
                        <div className="detail-item">
                          <span>Account</span>
                          <strong>{shrink(selectedSignalRuntime.accountId)}</strong>
                        </div>
                        <div className="detail-item">
                          <span>Strategy</span>
                          <strong>{shrink(selectedSignalRuntime.strategyId)}</strong>
                        </div>
                        <div className="detail-item">
                          <span>Transport</span>
                          <strong>{selectedSignalRuntime.transport || "--"}</strong>
                        </div>
                        <div className="detail-item">
                          <span>Health</span>
                          <strong>{String(selectedSignalRuntimeState.health ?? "--")}</strong>
                        </div>
                        <div className="detail-item">
                          <span>Signal Events</span>
                          <strong>{String(Math.trunc(getNumber(selectedSignalRuntimeState.signalEventCount) ?? 0))}</strong>
                        </div>
                        <div className="detail-item">
                          <span>Heartbeat</span>
                          <strong>{formatTime(String(selectedSignalRuntimeState.lastHeartbeatAt ?? ""))}</strong>
                        </div>
                        <div className="detail-item">
                          <span>Last Event</span>
                          <strong>{formatTime(String(selectedSignalRuntimeState.lastEventAt ?? ""))}</strong>
                        </div>
                        <div className="detail-item">
                          <span>Source States</span>
                          <strong>{String(Object.keys(selectedSignalRuntimeSourceStates).length)}</strong>
                        </div>
                        <div className="detail-item">
                          <span>Plan Ready</span>
                          <strong>{boolLabel(selectedSignalRuntimePlan.ready)}</strong>
                        </div>
                      </div>

                      <div className="backtest-breakdown">
                        <h4>Subscriptions</h4>
                        <SimpleTable
                          columns={["Source", "Role", "Symbol", "Channel", "Adapter"]}
                          rows={selectedSignalRuntimeSubscriptions.map((item) => [
                            String(item.sourceKey ?? "--"),
                            String(item.role ?? "--"),
                            String(item.symbol ?? "--"),
                            String(item.channel ?? "--"),
                            String(item.adapterKey ?? "--"),
                          ])}
                          emptyMessage="No subscriptions"
                        />
                      </div>

                      <div className="backtest-breakdown">
                        <h4>Signal Bars</h4>
                        {selectedSignalRuntimeSignalBars.length > 0 ? (
                          <div className="chart-shell">
                            <SignalBarChart candles={selectedSignalRuntimeSignalBars} />
                          </div>
                        ) : (
                          <div className="empty-state empty-state-compact">No 4h/1d signal bars cached yet</div>
                        )}
                      </div>

                      <div className="backtest-breakdown">
                        <h4>Signal States</h4>
                        <div className="backtest-notes">
                          {Object.entries(selectedSignalBarStates).length > 0 ? (
                            Object.entries(selectedSignalBarStates).map(([key, value]) => {
                              const state = getRecord(value);
                              const current = getRecord(state.current);
                              const prevBar1 = getRecord(state.prevBar1);
                              const prevBar2 = getRecord(state.prevBar2);
                              return (
                                <div key={key} className="note-item">
                                  {[
                                    key,
                                    `tf=${String(state.timeframe ?? "--")}`,
                                    `bars=${String(state.barCount ?? "--")}`,
                                    `ma20=${formatMaybeNumber(state.ma20)}`,
                                    `atr14=${formatMaybeNumber(state.atr14)}`,
                                    `t-1=${formatMaybeNumber(prevBar1.open)}/${formatMaybeNumber(prevBar1.high)}/${formatMaybeNumber(prevBar1.low)}/${formatMaybeNumber(prevBar1.close)}`,
                                    `t-2=${formatMaybeNumber(prevBar2.open)}/${formatMaybeNumber(prevBar2.high)}/${formatMaybeNumber(prevBar2.low)}/${formatMaybeNumber(prevBar2.close)}`,
                                    `current=${formatMaybeNumber(current.open)}/${formatMaybeNumber(current.high)}/${formatMaybeNumber(current.low)}/${formatMaybeNumber(current.close)}`,
                                  ].join(" · ")}
                                </div>
                              );
                            })
                          ) : (
                            <div className="empty-state empty-state-compact">No signal states yet</div>
                          )}
                        </div>
                      </div>

                      <div className="backtest-breakdown">
                        <h4>Runtime Timeline</h4>
                        <div className="backtest-notes">
                          {buildTimelineNotes(selectedSignalRuntimeTimeline).map((line: string) => (
                            <div key={line} className="note-item">
                              {line}
                            </div>
                          ))}
                        </div>
                      </div>

                      <div className="backtest-breakdown">
                        <h4>Last Event Summary</h4>
                        <div className="backtest-notes">
                          {Object.entries(selectedSignalRuntimeLastSummary).length > 0 ? (
                            Object.entries(selectedSignalRuntimeLastSummary).map(([key, value]) => (
                              <div key={key} className="note-item">
                                {key}: {typeof value === "object" ? JSON.stringify(value) : String(value)}
                              </div>
                            ))
                          ) : (
                            <div className="empty-state empty-state-compact">No event summary yet</div>
                          )}
                        </div>
                      </div>

                      <div className="backtest-breakdown">
                        <h4>Source States</h4>
                        <div className="backtest-notes">
                          {Object.entries(selectedSignalRuntimeSourceStates).length > 0 ? (
                            Object.entries(selectedSignalRuntimeSourceStates).slice(0, 8).map(([key, value]) => (
                              <div key={key} className="note-item">
                                {key}: {typeof value === "object" ? JSON.stringify(value) : String(value)}
                              </div>
                            ))
                          ) : (
                            <div className="empty-state empty-state-compact">No source states yet</div>
                          )}
                        </div>
                      </div>
                    </>
                  ) : (
                    <div className="empty-state empty-state-compact">No runtime session selected</div>
                  )}
                </div>
              </>
            ) : (
              <div className="empty-state empty-state-compact">No runtime sessions</div>
            )}
          </div>
        </div>
      </section>
    </div>
  );
}
