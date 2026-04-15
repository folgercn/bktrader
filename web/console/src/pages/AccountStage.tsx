import React, { useMemo, useState } from 'react';
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
  technicalStatusLabel,
  displaySignalBindingTimeframe,
  runtimePolicyValueLabel
} from '../utils/derivation';
import { AccountRecord, LiveSession, SignalRuntimeSession, LiveNextAction, ActiveSettingsModal } from '../types/domain';

interface AccountStageProps {
  logout: () => void;
  openLiveAccountModal: () => void;
  openLiveBindingModal: () => void;
  openLiveSessionModal: (session?: LiveSession | null) => void;
  openMonitorStage: () => void;
  launchLiveFlow: (account: AccountRecord) => void;
  stopLiveFlow: (accountId: string) => void;
  runLiveSessionAction: (id: string, action: "start" | "stop") => void;
  dispatchLiveSessionIntent: (id: string) => void;
  syncLiveSession: (id: string) => void;
  deleteLiveSession: (id: string) => Promise<void>;
  syncLiveAccount: (id: string) => void;
  jumpToSignalRuntimeSession: (id: string) => void;
  runLiveNextAction: (account: AccountRecord, nextAction: LiveNextAction, runtime: SignalRuntimeSession | null) => void;
  selectQuickLiveAccount: (id: string) => void;
  bindStrategySignalSource: () => void;
  unbindStrategySignalSource: (strategyId: string, bindingId: string) => void;
  updateRuntimePolicy: () => void;
  createSignalRuntimeSession: () => void;
  deleteSignalRuntimeSession: (sessionId: string) => void;
  runSignalRuntimeAction: (id: string, action: "start" | "stop") => void;
}

function statusLabelZh(status: string): string {
  switch (String(status).trim().toLowerCase()) {
    case "ready":
      return "就绪";
    case "watch":
      return "关注";
    case "warning":
      return "预警";
    case "blocked":
      return "阻塞";
    case "neutral":
      return "未激活";
    case "active":
      return "运行中";
    case "error":
      return "异常";
    case "idle":
      return "空闲";
    default:
      return status || "--";
  }
}

export function AccountStage({
  logout,
  openLiveAccountModal,
  openLiveBindingModal,
  openLiveSessionModal,
  openMonitorStage,
  launchLiveFlow,
  stopLiveFlow,
  runLiveSessionAction,
  dispatchLiveSessionIntent,
  syncLiveSession,
  deleteLiveSession,
  syncLiveAccount,
  jumpToSignalRuntimeSession,
  runLiveNextAction,
  selectQuickLiveAccount,
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
  const liveFlowAction = useUIStore(s => s.liveFlowAction);
  const liveBindAction = useUIStore(s => s.liveBindAction);
  const signalBindingAction = useUIStore(s => s.signalBindingAction);
  const signalRuntimeAction = useUIStore(s => s.signalRuntimeAction);
  const liveSessionCreateAction = useUIStore(s => s.liveSessionCreateAction);
  const liveSessionLaunchAction = useUIStore(s => s.liveSessionLaunchAction);
  const signalRuntimeSessions = useTradingStore(s => s.signalRuntimeSessions);
  const runtimePolicy = useTradingStore(s => s.runtimePolicy);
  const signalCatalog = useTradingStore(s => s.signalCatalog);
  const strategySignalBindingMap = useTradingStore(s => s.strategySignalBindingMap);
  const strategySignalForm = useUIStore(s => s.strategySignalForm);
  const setStrategySignalForm = useUIStore(s => s.setStrategySignalForm);
  const signalSourceTypes = useTradingStore(s => s.signalSourceTypes);
  const strategySignalBindings = useTradingStore(s => s.strategySignalBindings);
  const monitorHealth = useTradingStore(s => s.monitorHealth);
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

  const strategyIds = useMemo(() => new Set(strategies.map((item) => item.id)), [strategies]);
  const validLiveSessions = useMemo(
    () => liveSessions.filter((item) => strategyIds.has(item.strategyId)),
    [liveSessions, strategyIds]
  );

  const primaryLiveSession = highlightedLiveSession?.session ?? null;
  const primaryLiveSessionIntent = getRecord(primaryLiveSession?.state?.lastStrategyIntent);
  const primaryLiveAccount = primaryLiveSession ? liveAccounts.find(a => a.id === primaryLiveSession.accountId) || null : null;
  const primaryStrategyBindings = primaryLiveSession ? strategySignalBindingMap[primaryLiveSession.strategyId] ?? [] : [];
  const primaryLiveRuntimeSessions = primaryLiveSession ? signalRuntimeSessions.filter(s => s.accountId === primaryLiveSession.accountId) : [];
  const primaryLiveRuntime =
    primaryLiveSession
      ? signalRuntimeSessions.find((item) => item.id === String(primaryLiveSession.state?.signalRuntimeSessionId ?? "")) ??
        signalRuntimeSessions.find(
          (item) =>
            item.accountId === primaryLiveSession.accountId &&
            item.strategyId === primaryLiveSession.strategyId
        ) ??
        null
      : null;
  const primaryLiveRuntimeState = getRecord(primaryLiveRuntime?.state);
  const primaryLiveSessionRuntimeReadiness = deriveRuntimeReadiness(
    primaryLiveRuntimeState,
    deriveRuntimeSourceSummary(getRecord(primaryLiveRuntimeState.sourceStates), runtimePolicy),
    {
      requireTick: true,
      requireOrderBook: false
    }
  );
  const primaryLiveDispatchPreview = deriveLiveDispatchPreview(
    primaryLiveSession,
    primaryLiveAccount,
    primaryStrategyBindings,
    primaryLiveRuntimeSessions,
    primaryLiveRuntime,
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
  const [expandedAccountId, setExpandedAccountId] = useState<string | null>(null);

  const hasConfiguredAccount = liveAccounts.some((account) => account.status === "CONFIGURED" || account.status === "READY");
  const hasSignalBinding = strategySignalBindings.length > 0;
  const hasRunningRuntime = signalRuntimeSessions.some((session) => session.status === "RUNNING");
  const hasLiveSession = validLiveSessions.length > 0;
  const hasRunningLiveSession = validLiveSessions.some((session) => session.status === "RUNNING");
  const platformRuntimePolicy = monitorHealth?.runtimePolicy ?? runtimePolicy;

  const onboardingSteps = [
    {
      key: "account",
      title: "准备账户",
      detail: hasConfiguredAccount ? "已具备可用实盘账户" : "先新建账户并绑定交易所适配器",
      status: hasConfiguredAccount ? "done" : "current",
    },
    {
      key: "signal",
      title: "接通信号",
      detail: hasSignalBinding ? "已配置策略级 signal bindings" : "先配置策略级 signal bindings",
      status: !hasConfiguredAccount ? "pending" : hasSignalBinding ? "done" : "current",
    },
    {
      key: "runtime",
      title: "启动运行时",
      detail: hasRunningRuntime ? "signal runtime 正在运行" : "创建并启动 signal runtime session",
      status: !hasSignalBinding ? "pending" : hasRunningRuntime ? "done" : "current",
    },
    {
      key: "session",
      title: "创建会话",
      detail: hasRunningLiveSession
        ? "已有运行中的实盘会话，可转到监控台盯盘"
        : hasLiveSession
          ? "已有实盘策略会话，启动后进入监控台"
          : "选择账户 + 策略 + 交易对创建会话",
      status: !hasRunningRuntime ? "pending" : hasRunningLiveSession ? "done" : "current",
    },
  ];

  function setActiveSettingsModal(modal: ActiveSettingsModal) {
    useUIStore.getState().setActiveSettingsModal(modal);
  }

  return (
    <div className="absolute inset-0 overflow-y-auto p-6 space-y-6 bg-zinc-950/50">
      <section id="overview" className="hero">
        <div>
          <p className="eyebrow">交易主控</p>
          <h2>先准备账户，再接通信号，然后创建并启动实盘会话</h2>
          <p className="hero-copy">
            这页只负责把链路搭起来。按顺序完成账户准备、信号接通和实盘会话创建后，再进入监控台处理运行状态与人工干预。
          </p>
        </div>
        <div className="hero-side hero-account-toolbar">
          <div className="hero-user-card hero-account-card">
            <div>
              <strong>当前选中账户</strong>
              <p>{quickLiveAccount?.name ?? "--"} · {quickLiveAccount?.status ?? "--"} · {quickLiveAccount?.exchange ?? "--"}</p>
            </div>
          </div>
          <div className="session-actions hero-actions">
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
      </section>

      <section className="panel panel-session">
        <div className="panel-header">
          <div>
            <p className="panel-kicker">Workflow</p>
            <h3>创建一条可运行的实盘链路</h3>
          </div>
        </div>
        <div className="workflow-grid">
          {onboardingSteps.map((step, index) => (
            <div key={step.key} className={`workflow-card workflow-card-${step.status}`}>
              <div className="workflow-card-head">
                <span className="workflow-step-index">{index + 1}</span>
                <StatusPill tone={step.status === "done" ? "ready" : step.status === "current" ? "watch" : "neutral"}>
                  {step.status === "done" ? "已就绪" : step.status === "current" ? "当前步骤" : "待完成"}
                </StatusPill>
              </div>
              <h4>{step.title}</h4>
              <p>{step.detail}</p>
            </div>
          ))}
        </div>
      </section>

      <section id="accounts" className="panel panel-session">
        <div className="panel-header">
          <div>
            <p className="panel-kicker">Accounts</p>
            <h3>第一步：准备账户</h3>
          </div>
        </div>
        <div className="live-grid">
          <div className="live-grid-span-2">
            {liveAccounts.length > 0 ? (
              <div className="live-card-list">
                {liveAccounts.map((account) => {
                  const binding = (account.metadata?.liveBinding as Record<string, unknown> | undefined) ?? {};
                  const syncSnapshot = getRecord(getRecord(account.metadata).liveSyncSnapshot);
                  const runtimeSessionsForAccount = signalRuntimeSessions.filter((item) => item.accountId === account.id);
                  const activeRuntime = runtimeSessionsForAccount.find((item) => item.status === "RUNNING") ?? runtimeSessionsForAccount[0] ?? null;
                  const activeRuntimeState = getRecord(activeRuntime?.state);
                  const activeRuntimeSummary = getRecord(activeRuntimeState.lastEventSummary);
                  const activeRuntimeMarket = deriveRuntimeMarketSnapshot(getRecord(activeRuntimeState.sourceStates), activeRuntimeSummary);
                  const strategyBindings =
                    (activeRuntime?.strategyId ? strategySignalBindingMap[activeRuntime.strategyId] : undefined) ??
                    strategySignalBindingMap[
                      validLiveSessions.find((item) => item.accountId === account.id)?.strategyId ?? ""
                    ] ??
                    [];
                  const activeRuntimeSourceSummary = deriveRuntimeSourceSummary(
                    getRecord(activeRuntimeState.sourceStates),
                    runtimePolicy
                  );
                  const activeSignalBarState = derivePrimarySignalBarState(getRecord(activeRuntimeState.signalBarStates));
                  const activeSignalAction = deriveSignalActionSummary(activeSignalBarState);
                  const activeRuntimeTimeline = getList(activeRuntimeState.timeline);
                  const activeRuntimeReadiness = deriveRuntimeReadiness(activeRuntimeState, activeRuntimeSourceSummary, {
                    requireTick: strategyBindings.some((item) => item.streamType === "trade_tick"),
                    requireOrderBook: strategyBindings.some((item) => item.streamType === "order_book"),
                  });
                  const hasRunningRuntime = runtimeSessionsForAccount.some((item) => item.status === "RUNNING");
                  const hasRunningLiveSession = validLiveSessions.some(
                    (item) => item.accountId === account.id && item.status === "RUNNING"
                  );
                  const isLiveFlowRunning = hasRunningRuntime || hasRunningLiveSession;
                  const livePreflight = deriveLivePreflightSummary(
                    account,
                    strategyBindings,
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
                  const accountDetailOpen = expandedAccountId === account.id;
                  return (
                    <div key={account.id} className="live-account-card">
                      <div className="live-account-card-header">
                        <div className="session-stat">
                          <span>{account.name}</span>
                          <strong>{account.status}</strong>
                        </div>
                        <div className="live-account-status">
                          <StatusPill tone={runtimeReadinessTone(activeRuntimeReadiness.status)}>
                            {`环境：${statusLabelZh(activeRuntimeReadiness.status)}`}
                          </StatusPill>
                          <StatusPill tone={runtimeReadinessTone(livePreflight.status)}>
                            {`预检：${statusLabelZh(livePreflight.status)}`}
                          </StatusPill>
                        </div>
                      </div>
                      <div className="live-account-meta">
                        <span>交易所: {account.exchange}</span>
                        <span>适配器: {String(binding.adapterKey ?? "--")}</span>
                        <span>{activeRuntime ? `${activeRuntime.status} · ${String(activeRuntimeState.health ?? "--")}` : "无运行实例"}</span>
                      </div>
                      <div className="live-account-metrics">
                        <div className="detail-item detail-item-compact">
                          <span>最新价</span>
                          <strong>{formatMaybeNumber(activeRuntimeMarket.tradePrice)}</strong>
                        </div>
                        <div className="detail-item detail-item-compact">
                          <span>买/卖</span>
                          <strong>{formatMaybeNumber(activeRuntimeMarket.bestBid)} / {formatMaybeNumber(activeRuntimeMarket.bestAsk)}</strong>
                        </div>
                        <div className="detail-item detail-item-compact">
                          <span>价差</span>
                          <strong>{formatMaybeNumber(activeRuntimeMarket.spreadBps)} bps</strong>
                        </div>
                        <div className="detail-item detail-item-compact">
                          <span>最后心跳</span>
                          <strong>{formatTime(String(activeRuntimeState.lastHeartbeatAt ?? ""))}</strong>
                        </div>
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
                      <div className="live-account-summary">
                        <div className="note-item">实盘预检: {livePreflight.reason} · {livePreflight.detail}</div>
                        <div className="note-item">下一步操作: {liveNextAction.label} · {liveNextAction.detail}</div>
                      </div>
                      <div className="inline-actions live-account-actions">
                        <ActionButton
                          label={
                            liveFlowAction === account.id
                              ? isLiveFlowRunning ? "停止中..." : "启动中..."
                              : isLiveFlowRunning ? "停止实盘流程" : "启动实盘流程"
                          }
                          variant={isLiveFlowRunning ? "danger" : undefined}
                          disabled={
                            liveFlowAction !== null ||
                            liveBindAction ||
                            signalBindingAction !== null ||
                            signalRuntimeAction !== null ||
                            liveSessionAction !== null ||
                            liveSessionCreateAction ||
                            liveSessionLaunchAction
                          }
                          onClick={() => {
                            if (isLiveFlowRunning) {
                              stopLiveFlow(account.id);
                              return;
                            }
                            launchLiveFlow(account);
                          }}
                        />
                        <ActionButton
                          label={accountDetailOpen ? "收起详情" : "查看详情"}
                          variant="ghost"
                          onClick={() => setExpandedAccountId((current) => current === account.id ? null : account.id)}
                        />
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
                      {accountDetailOpen ? (
                        <div className="live-account-detail">
                          <div className="live-account-detail-grid">
                            <div className="detail-item detail-item-compact">
                              <span>同步状态</span>
                              <strong>{String(syncSnapshot.syncStatus ?? "未同步")} · {formatTime(String(getRecord(account.metadata).lastLiveSyncAt ?? ""))}</strong>
                            </div>
                            <div className="detail-item detail-item-compact">
                              <span>来源与实例</span>
                              <strong>{String(syncSnapshot.source ?? "--")} · {strategyBindings.length} 策略绑定 · {runtimeSessionsForAccount.length} 实例</strong>
                            </div>
                            <div className="detail-item detail-item-compact">
                              <span>指标</span>
                              <strong>周期 {String(activeSignalBarState.timeframe ?? "--")} · ma20 {formatMaybeNumber(activeSignalBarState.ma20)} · atr14 {formatMaybeNumber(activeSignalBarState.atr14)}</strong>
                            </div>
                            <div className="detail-item detail-item-compact">
                              <span>账户同步</span>
                              <strong>订单 {String(syncSnapshot.orderCount ?? "--")} · 成交 {String(syncSnapshot.fillCount ?? "--")} · 持仓 {String(syncSnapshot.positionCount ?? "--")}</strong>
                            </div>
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
                        </div>
                      ) : null}
                    </div>
                  );
                })}
              </div>
            ) : (
              <div className="empty-state empty-state-compact">暂无实盘账户</div>
            )}
          </div>
        </div>
      </section>

      <section id="signals" className="panel panel-session">
        <div className="panel-header">
          <div>
            <p className="panel-kicker">Signal Pipeline</p>
            <h3>第二步：接通信号源并启动运行时</h3>
          </div>
          <div className="range-box">
            <span>{signalCatalog?.sources?.length ?? 0} 个源</span>
            <span>{signalRuntimeSessions.length} 个会话</span>
          </div>
        </div>

        <div className="live-grid">
          <div className="backtest-form session-form">
            <h4>2.1 绑定策略信号源</h4>
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

          <div className="backtest-list">
            <h4>当前信号绑定结果</h4>
            <div className="backtest-breakdown">
              <h5>策略级别</h5>
              <SimpleTable
                columns={["信号源", "角色", "代码 (Symbol)", "周期", "交易所", "状态", "操作"]}
                rows={strategySignalBindings.map((item) => [
                  item.sourceName,
                  item.role,
                  item.symbol || "--",
                  displaySignalBindingTimeframe(item),
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
          <div className="backtest-list live-grid-span-2">
            <h4>信号源目录与说明</h4>
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
        </div>

        <div className="live-grid">
          <div className="backtest-form session-form">
            <h4>2.2 运行时策略</h4>
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
                <span>策略评估静默期 (秒)</span>
                <input
                  value={runtimePolicyForm.strategyEvaluationQuietSeconds}
                  onChange={(event) =>
                    setRuntimePolicyForm((current) => ({
                      ...current,
                      strategyEvaluationQuietSeconds: event.target.value,
                    }))
                  }
                />
              </label>
              <label className="form-field">
                <span>账户同步新鲜度 (秒)</span>
                <input
                  value={runtimePolicyForm.liveAccountSyncFreshnessSeconds}
                  onChange={(event) =>
                    setRuntimePolicyForm((current) => ({
                      ...current,
                      liveAccountSyncFreshnessSeconds: event.target.value,
                    }))
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
                活跃策略: 成交价格 {runtimePolicyValueLabel(platformRuntimePolicy?.tradeTickFreshnessSeconds)} · 盘口 {runtimePolicyValueLabel(platformRuntimePolicy?.orderBookFreshnessSeconds)} ·
                信号 K 线 {runtimePolicyValueLabel(platformRuntimePolicy?.signalBarFreshnessSeconds)}
              </div>
              <div className="note-item">
                运行时静默 {runtimePolicyValueLabel(platformRuntimePolicy?.runtimeQuietSeconds)} · 策略评估静默 {runtimePolicyValueLabel(platformRuntimePolicy?.strategyEvaluationQuietSeconds)}
              </div>
              <div className="note-item">
                账户同步新鲜度 {runtimePolicyValueLabel(platformRuntimePolicy?.liveAccountSyncFreshnessSeconds)} · 模拟盘预检 {runtimePolicyValueLabel(platformRuntimePolicy?.paperStartReadinessTimeoutSeconds)}
              </div>
              <div className="note-item">
                表单支持显式保存 `0`，表示关闭对应阈值，而不是丢失该字段。
              </div>
            </div>
          </div>

          <div className="backtest-form session-form">
            <h4>2.3 创建信号运行时</h4>
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
                    Missing: {String(item.sourceKey)} · {String(item.role)} · {String(item.symbol)} · {displaySignalBindingTimeframe(item)}
                  </div>
                ))
              ) : null}
              {signalRuntimePlan?.matchedBindings ? (
                getList(signalRuntimePlan.matchedBindings).map((item, index) => (
                  <div key={index} className="note-item">
                  Matched: {String(item.sourceName ?? item.sourceKey ?? "--")} · {String(item.role)} · {String(item.symbol)} · {displaySignalBindingTimeframe(item)}
                  </div>
                ))
              ) : null}
            </div>
          </div>
        </div>

        <div className="backtest-list mt-8 pt-8 border-t border-white/5">
          <h4 className="text-sm font-medium text-emerald-400 mb-4">2.4 运行时会话与结果</h4>
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
                    <h5 className="text-sm font-medium text-emerald-400">运行时详情</h5>
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

      <section id="sessions" className="panel panel-session">
        <div className="panel-header">
          <div>
            <p className="panel-kicker">Sessions</p>
            <h3>第三步：创建实盘策略会话</h3>
          </div>
        </div>
        <div className="live-grid">
          <div className="backtest-list live-grid-span-2">
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
            <div className="panel-compact bg-white/5 rounded-2xl p-5 border border-white/5 mt-6">
              <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
                <div className="space-y-2">
                  <p className="text-[10px] font-bold uppercase tracking-[0.3em] text-zinc-500">Next</p>
                  <div>
                    <h4 className="text-sm font-semibold text-zinc-100">运行中状态已经移到监控台</h4>
                    <p className="text-xs text-zinc-400">
                      完成账户、信号源和实盘会话配置后，到监控台查看当前优先处理会话、执行状态和人工干预入口。
                    </p>
                  </div>
                </div>
                <div className="session-actions">
                  <ActionButton label="打开监控台" onClick={openMonitorStage} />
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>
      </div>
    );
}
