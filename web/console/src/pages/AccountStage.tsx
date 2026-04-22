import React, { useMemo, useState } from 'react';
import { HelpCircle, Zap, Edit3, Square, Trash2, Play, ArrowRight, ShieldCheck, Activity, RotateCw, AlertTriangle } from 'lucide-react';
import { useUIStore } from '../store/useUIStore';
import { useTradingStore } from '../store/useTradingStore';
import { SignalBarChart } from '../components/charts/SignalBarChart';
import { LiveTradePairsCard } from '../components/live/LiveTradePairsCard';
import { useLiveTradePairs } from '../hooks/useLiveTradePairs';
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
import { 
  AlertDialog, 
  AlertDialogAction, 
  AlertDialogCancel, 
  AlertDialogContent, 
  AlertDialogDescription, 
  AlertDialogFooter, 
  AlertDialogHeader, 
  AlertDialogTitle 
} from "../components/ui/alert-dialog";
import { toast } from "sonner";
import { AccountRecord, LiveSession, SignalRuntimeSession, LiveNextAction, ActiveSettingsModal } from '../types/domain';
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from "../components/ui/card";
import { Button } from "../components/ui/button";
import { Badge } from "../components/ui/badge";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "../components/ui/tooltip";
import { 
  Select, 
  SelectContent, 
  SelectItem, 
  SelectTrigger, 
  SelectValue 
} from "../components/ui/select";
import { Input } from "../components/ui/input";
import { Separator } from "../components/ui/separator";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "../components/ui/table";
import { cn } from "../lib/utils";

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
  updateRuntimePolicy: () => void;
  createSignalRuntimeSession: () => void;
  deleteSignalRuntimeSession: (sessionId: string) => void;
  runSignalRuntimeAction: (id: string, action: "start" | "stop") => void;
  executeLaunchTemplate: (template: any, accountId: string) => void;
  unbindLiveAccount: (accountId: string) => void;
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
  updateRuntimePolicy,
  createSignalRuntimeSession,
  deleteSignalRuntimeSession,
  runSignalRuntimeAction,
  executeLaunchTemplate,
  unbindLiveAccount
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
  const summaries = useTradingStore(s => s.summaries);
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
  const signalRuntimeAction = useUIStore(s => s.signalRuntimeAction);
  const liveSessionCreateAction = useUIStore(s => s.liveSessionCreateAction);
  const liveSessionLaunchAction = useUIStore(s => s.liveSessionLaunchAction);
  const signalRuntimeSessions = useTradingStore(s => s.signalRuntimeSessions);
  const runtimePolicy = useTradingStore(s => s.runtimePolicy);
  const signalCatalog = useTradingStore(s => s.signalCatalog);
  const strategySignalBindingMap = useTradingStore(s => s.strategySignalBindingMap);
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
  const [showSignalNotes, setShowSignalNotes] = useState(false);
  const [showAccountHelp, setShowAccountHelp] = useState(false);
  const [showPolicyHelp, setShowPolicyHelp] = useState(false);
  const [showRuntimeHelp, setShowRuntimeHelp] = useState(false);
  const launchTemplates = useTradingStore(s => s.launchTemplates);
  const launchingTemplate = useUIStore(s => s.launchingTemplate);

  const [confirmConfig, setConfirmConfig] = useState<{
    open: boolean;
    title: string;
    description: string;
    onConfirm: () => Promise<void> | void;
  }>({ open: false, title: "", description: "", onConfirm: () => {} });

  const activeLiveSession = liveSessions.find(s => s.accountId === quickLiveAccountId);
  const activeTemplateKey = (activeLiveSession?.metadata as any)?.launchTemplateKey;

  const openConfirm = (title: string, description: string, onConfirm: () => Promise<void> | void) => {
    setConfirmConfig({ open: true, title, description, onConfirm });
  };

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
  const primaryTradePairs = useLiveTradePairs(primaryLiveSession?.id ?? null, 6);
  
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
  const selectedRuntimeSymbol = String(selectedSignalRuntimeState.symbol ?? "").trim().toUpperCase();
  const selectedRuntimeSession = selectedSignalRuntime
    ? validLiveSessions.find((item) => String(item.state?.signalRuntimeSessionId ?? "") === selectedSignalRuntime.id) ??
      validLiveSessions.find(
        (item) => item.accountId === selectedSignalRuntime.accountId && item.strategyId === selectedSignalRuntime.strategyId
      ) ??
      null
    : null;
  const selectedRuntimeSignalTimeframe = String(selectedRuntimeSession?.state?.signalTimeframe ?? "").trim().toLowerCase();
  const selectedRuntimeSignalBarStateKey = String(selectedRuntimeSession?.state?.lastStrategyEvaluationSignalBarStateKey ?? "").trim();
  const selectedSignalRuntimeSignalBars = deriveSignalBarCandles(selectedSignalRuntimeSourceStates, {
    targetSymbol: selectedRuntimeSymbol,
    targetTimeframe: selectedRuntimeSignalTimeframe,
    targetStateKey: selectedRuntimeSignalBarStateKey,
  });
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
    <div className="absolute inset-0 overflow-y-auto space-y-8 bg-[var(--bk-canvas)] p-8">
      {/* 顶部总控 - 扁平化重构 */}
      <Card tone="bento" className="overflow-hidden rounded-[24px] border border-[var(--bk-border-strong)] shadow-sm">
        <div className="py-3 px-6 flex flex-col md:flex-row items-center justify-between gap-4">
           <div className="flex items-center gap-6 overflow-hidden">
              <div className="shrink-0">
                <p className="mb-0.5 font-mono text-[9px] font-black uppercase tracking-widest text-[var(--bk-status-success)]">Control Center</p>
                <h2 className="whitespace-nowrap text-lg font-black tracking-tight text-[var(--bk-text-primary)]">账户与信号实盘总控</h2>
              </div>
              
              <Separator orientation="vertical" className="hidden h-8 bg-[color-mix(in_srgb,var(--bk-border)_40%,transparent)] lg:block" />
              
              <div className="hidden h-8 items-center gap-2 rounded-xl border border-[color-mix(in_srgb,var(--bk-border)_50%,transparent)] bg-[var(--bk-surface-strong)] px-3 transition-all lg:flex">
                <span className="text-[9px] font-black uppercase text-[var(--bk-text-muted)] opacity-40">Live:</span>
                <span className="text-[10px] font-black text-[var(--bk-text-primary)]">{quickLiveAccount?.name ?? "--"}</span>
                <div className={cn(
                  "rounded-lg border px-1.5 py-0.5 font-mono text-[8px] font-black uppercase shadow-sm",
                  quickLiveAccount?.status === "READY" || quickLiveAccount?.status === "CONFIGURED"
                    ? "border-[var(--bk-status-success-soft)] text-[var(--bk-status-success)] bg-[var(--bk-surface)]"
                    : "border-[var(--bk-border)] text-[var(--bk-text-muted)] bg-[var(--bk-surface)]"
                )}>
                  {quickLiveAccount?.status ?? "no_state"}
                </div>
              </div>
           </div>
           
           <div className="flex items-center gap-2">
              <div className="flex items-center rounded-xl border border-[color-mix(in_srgb,var(--bk-border)_20%,transparent)] bg-[var(--bk-surface-faint)] p-1">
                <Button 
                  variant="bento-ghost" 
                  size="sm" 
                  className="h-8 rounded-lg px-4 text-[10px] font-black shadow-none hover:bg-[var(--bk-surface)]" 
                  onClick={openLiveAccountModal}
                >
                  新建账户
                </Button>
                <Separator orientation="vertical" className="mx-1 h-4 bg-[color-mix(in_srgb,var(--bk-border)_30%,transparent)]" />
                <Button 
                   variant="bento-ghost" 
                   size="sm" 
                   className="h-8 rounded-lg px-4 text-[10px] font-black shadow-none hover:bg-[var(--bk-surface)]"
                   disabled={!quickLiveAccountId}
                   onClick={() => {
                     if (quickLiveAccountId) selectQuickLiveAccount(quickLiveAccountId);
                     openLiveBindingModal();
                   }}
                >
                  绑定适配器
                </Button>
                <Separator orientation="vertical" className="mx-1 h-4 bg-[color-mix(in_srgb,var(--bk-border)_30%,transparent)]" />
                <Button 
                   variant="bento-ghost" 
                   size="sm" 
                   className="h-8 rounded-lg px-4 text-[10px] font-black shadow-none hover:bg-[var(--bk-surface)]"
                   disabled={!quickLiveAccountId}
                   onClick={() => {
                     if (quickLiveAccountId) selectQuickLiveAccount(quickLiveAccountId);
                     openLiveSessionModal();
                   }}
                >
                  创建会话
                </Button>
              </div>
           </div>
        </div>
      </Card>

      {/* Workflow 引导区域 */}
      <Card tone="bento" className="rounded-[24px] shadow-[var(--bk-shadow-card)]">
        <CardHeader className="pb-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="font-mono text-[10px] font-bold uppercase tracking-widest text-[var(--bk-status-success)]">Step by Step</p>
              <CardTitle className="text-lg font-black text-[var(--bk-text-primary)]">建立一条可运行的实盘链路</CardTitle>
            </div>
            <Activity size={20} className="text-[var(--bk-border)] opacity-50" />
          </div>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
            {onboardingSteps.map((step, index) => (
              <div 
                key={step.key} 
                className={`relative p-4 rounded-[20px] border transition-all ${
                  step.status === "done" 
                    ? "bg-[var(--bk-status-success-soft)] border-[color:color-mix(in_srgb,var(--bk-status-success)_20%,transparent)]" 
                    : step.status === "current" 
                      ? "bg-[var(--bk-surface-strong)] border-[var(--bk-border)] shadow-sm" 
                      : "bg-[var(--bk-surface-muted)]/45 border-transparent opacity-60"
                }`}
              >
                <div className="flex items-center justify-between mb-3">
                   <div className={`flex items-center justify-center w-6 h-6 rounded-lg text-[10px] font-bold border ${
                     step.status === "done" ? "bg-[var(--bk-status-success)] text-[var(--bk-canvas)] border-transparent" : "bg-[var(--bk-surface)] border-[var(--bk-border)] text-[var(--bk-text-primary)]"
                   }`}>
                     {index + 1}
                   </div>
                   <div className={cn(
                     "rounded-lg border px-2 py-0.5 font-mono text-[9px] font-black uppercase shadow-sm border-inherit",
                     step.status === "done" ? "text-[var(--bk-status-success)] bg-[var(--bk-surface)]" : step.status === "current" ? "text-[var(--bk-status-warning)] bg-[var(--bk-surface)]" : "text-[var(--bk-text-muted)] bg-[var(--bk-surface)]"
                   )}>
                     {step.status === "done" ? "已完成" : step.status === "current" ? "进行中" : "待处理"}
                   </div>
                </div>
                <h4 className={`mb-1.5 text-sm font-black ${step.status === "pending" ? "text-[var(--bk-text-muted)]" : "text-[var(--bk-text-primary)]"}`}>{step.title}</h4>
                <p className="text-xs font-medium leading-relaxed text-[var(--bk-text-muted)]">{step.detail}</p>
              </div>
            ))}
          </div>
        </CardContent>
      </Card>

      <Card tone="bento" className="rounded-[24px] shadow-[var(--bk-shadow-card)]">
        <CardHeader className="pb-4 flex flex-row items-center justify-between">
          <div className="flex items-center gap-2">
            <div>
              <p className="font-mono text-[10px] font-bold uppercase tracking-widest text-[var(--bk-status-success)]">Step 1 / Accounts</p>
              <CardTitle className="text-lg font-black text-[var(--bk-text-primary)]">第一步：准备账户</CardTitle>
            </div>
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger>
                  <HelpCircle size={14} className="cursor-help text-[var(--bk-text-muted)] transition-colors hover:text-[var(--bk-status-success)]" />
                </TooltipTrigger>
                <TooltipContent className="w-80 rounded-xl border-[var(--bk-border)] bg-[var(--bk-surface-overlay-strong)] p-4 shadow-xl">
                  <div className="space-y-3">
                    <p className="text-[10px] font-bold uppercase text-[var(--bk-status-success)]">账户准备指南</p>
                    <div className="space-y-2 text-[11px] leading-relaxed text-[var(--bk-text-primary)]">
                      <p>• <strong>适配器绑定</strong>：账户需绑定具体的交易所适配器（如 Binance-Live）才能与实盘环境交互。</p>
                      <p>• <strong>数据同步</strong>：实盘账户需定期点击同步，以刷新订单、成交和资产快照。</p>
                      <p>• <strong>环境预检</strong>：系统会自动检查网络延迟、API 权限和资产充足度。</p>
                    </div>
                  </div>
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>
          </div>
          <Button variant="bento-ghost" size="sm" className="h-8 font-bold text-[var(--bk-status-success)]" onClick={openLiveAccountModal}>
            + 新建账户
          </Button>
        </CardHeader>
        <CardContent>
          {liveAccounts.length > 0 ? (
            <div className="grid grid-cols-1 gap-4">
              {liveAccounts.map((account) => {
                const binding = (account.metadata?.liveBinding as Record<string, unknown> | undefined) ?? {};
                const syncSnapshot = getRecord(getRecord(account.metadata).liveSyncSnapshot);
                const summary = summaries.find(s => s.accountId === account.id);
                const runtimeSessionsForAccount = signalRuntimeSessions.filter((item) => item.accountId === account.id);
                const activeRuntime = runtimeSessionsForAccount.find((item) => item.status === "RUNNING") ?? runtimeSessionsForAccount[0] ?? null;
                const activeRuntimeState = getRecord(activeRuntime?.state);
                const activeRuntimeSummary = getRecord(activeRuntimeState.lastEventSummary);
                const accountSession = validLiveSessions.find((item) => item.accountId === account.id);
                 const accountSymbol = String(accountSession?.state?.symbol ?? activeRuntime?.state?.symbol ?? "").trim().toUpperCase();
                 const activeRuntimeMarket = deriveRuntimeMarketSnapshot(getRecord(activeRuntimeState.sourceStates), activeRuntimeSummary, accountSymbol);
                const strategyBindings = (activeRuntime?.strategyId ? strategySignalBindingMap[activeRuntime.strategyId] : undefined) ?? strategySignalBindingMap[validLiveSessions.find((item) => item.accountId === account.id)?.strategyId ?? ""] ?? [];
                const activeRuntimeSourceSummary = deriveRuntimeSourceSummary(getRecord(activeRuntimeState.sourceStates), runtimePolicy, accountSymbol);
                const activeSignalTimeframe = String(accountSession?.state?.signalTimeframe ?? "").trim().toLowerCase();
                const activeSignalBarStateKey = String(accountSession?.state?.lastStrategyEvaluationSignalBarStateKey ?? "").trim();
                const activeSignalBarState = derivePrimarySignalBarState(getRecord(activeRuntimeState.signalBarStates), {
                  targetSymbol: accountSymbol,
                  targetTimeframe: activeSignalTimeframe,
                  targetStateKey: activeSignalBarStateKey,
                });
                const activeSignalAction = deriveSignalActionSummary(activeSignalBarState);
                const activeRuntimeTimeline = getList(activeRuntimeState.timeline);
                const activeRuntimeReadiness = deriveRuntimeReadiness(activeRuntimeState, activeRuntimeSourceSummary, {
                  requireTick: strategyBindings.some((item) => item.streamType === "trade_tick"),
                  requireOrderBook: strategyBindings.some((item) => item.streamType === "order_book"),
                });
                const hasRunningRuntime = runtimeSessionsForAccount.some((item) => item.status === "RUNNING");
                const hasRunningLiveSession = validLiveSessions.some((item) => item.accountId === account.id && item.status === "RUNNING");
                const isLiveFlowRunning = hasRunningRuntime || hasRunningLiveSession;
                const livePreflight = deriveLivePreflightSummary(account, strategyBindings, runtimeSessionsForAccount, activeRuntime, activeRuntimeReadiness);
                const liveNextAction = deriveLiveNextAction(livePreflight);
                const liveAlerts = deriveLiveAlerts(account, activeRuntimeState, activeRuntimeSourceSummary, activeRuntimeReadiness, activeSignalAction, runtimePolicy);
                const accountDetailOpen = expandedAccountId === account.id;

                return (
                  <div key={account.id} className="group overflow-hidden rounded-[24px] border border-[var(--bk-border)] bg-[var(--bk-surface-strong)] shadow-sm transition-all duration-300 hover:translate-y-[-2px] hover:shadow-xl">
                    <div className="flex flex-col lg:flex-row items-stretch">
                       {/* 左侧：身份与环境状态 (Identity & Context) */}
                       <div className="space-y-4 border-r border-[color-mix(in_srgb,var(--bk-border)_30%,transparent)] p-6 lg:w-1/3 xl:w-1/4">
                          <div className="flex items-center gap-4">
                             <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl border-2 border-[var(--bk-border)] bg-[var(--bk-surface)] text-xl font-black text-[var(--bk-status-success)] shadow-sm transition-colors group-hover:border-[var(--bk-text-primary)]">
                               {account.exchange?.charAt(0) || "A"}
                             </div>
                             <div className="space-y-1 min-w-0">
                                <h4 className="truncate text-lg font-black tracking-tight text-[var(--bk-text-primary)]">{account.name}</h4>
                                <div className="flex flex-wrap gap-1.5">
                                   <Badge variant="neutral" className="h-4.5 bg-[var(--bk-surface)] text-[10px] font-bold text-[var(--bk-text-muted)]">{account.exchange}</Badge>
                                   <Badge variant="neutral" className="h-4.5 bg-[var(--bk-surface)] text-[10px] font-bold uppercase text-[var(--bk-text-muted)]">{String(binding.adapterKey ?? "NO_ADAPTER")}</Badge>
                                </div>
                             </div>
                          </div>
                          
                          <div className="flex items-center gap-2 pt-1">
                             <Badge 
                               variant="metal" 
                               className={cn(
                                 "gap-1.5 px-2.5 py-1",
                                 activeRuntimeReadiness.status === "ready" 
                                   ? "border-[var(--bk-status-success-soft)] text-[var(--bk-status-success)]" 
                                   : "border-[var(--bk-status-warning-soft)] text-[var(--bk-status-warning)]"
                               )}
                             >
                                <div className={cn(
                                  "size-1.5 rounded-full",
                                  activeRuntimeReadiness.status === "ready" ? "bg-[var(--bk-status-success)] animate-pulse" : (String(activeRuntimeState.health).toLowerCase() === "recovering" ? "bg-[var(--bk-status-warning)] animate-pulse" : "bg-amber-600")
                                )} />
                                环境：{statusLabelZh(activeRuntimeReadiness.status)}
                             </Badge>
                             <Badge variant="metal" className="text-[var(--bk-text-muted)] px-2.5 py-1">
                                预检：{statusLabelZh(livePreflight.status)}
                             </Badge>
                          </div>
                       </div>

                       <div className="flex flex-1 flex-col justify-center border-r border-[color-mix(in_srgb,var(--bk-border)_30%,transparent)] bg-[var(--bk-surface-faint)] p-6">
                          <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
                             <div className="rounded-2xl border border-[color-mix(in_srgb,var(--bk-border)_40%,transparent)] bg-[var(--bk-surface-muted)] p-3.5 shadow-sm transition-all hover:border-[var(--bk-text-primary)] hover:bg-[var(--bk-surface)]">
                                <span className="mb-1.5 block text-[9px] font-black uppercase tracking-widest text-[var(--bk-text-muted)] opacity-60">Net Equity</span>
                                <p className="font-mono text-xl font-black tracking-tighter text-[var(--bk-status-success)] tabular-nums">{summary ? formatMaybeNumber(summary.netEquity) : "--"}</p>
                             </div>
                             <div className="rounded-2xl border border-[color-mix(in_srgb,var(--bk-border)_40%,transparent)] bg-[var(--bk-surface-muted)] p-3.5 shadow-sm transition-all hover:border-[var(--bk-text-primary)] hover:bg-[var(--bk-surface)]">
                                <span className="mb-1.5 block text-[9px] font-black uppercase tracking-widest text-[var(--bk-text-muted)] opacity-60">Unrealized PnL</span>
                                <p className={`text-xl font-mono font-black tracking-tighter tabular-nums ${summary && summary.unrealizedPnl >= 0 ? 'text-[var(--bk-status-success)]' : 'text-[var(--bk-status-danger)]'}`}>
                                  {summary ? (summary.unrealizedPnl > 0 ? "+" : "") + formatMaybeNumber(summary.unrealizedPnl) : "--"}
                                </p>
                             </div>
                             <div className="rounded-2xl border border-[color-mix(in_srgb,var(--bk-border)_40%,transparent)] bg-[var(--bk-surface-muted)] p-3.5 shadow-sm transition-all hover:border-[var(--bk-text-primary)] hover:bg-[var(--bk-surface)]">
                                <span className="mb-1.5 block text-[9px] font-black uppercase tracking-widest text-[var(--bk-text-muted)] opacity-60">Market Price</span>
                                <p className="font-mono text-xl font-black tracking-tighter text-[var(--bk-text-primary)] tabular-nums">{formatMaybeNumber(activeRuntimeMarket.tradePrice)}</p>
                             </div>
                             <div className="rounded-2xl border border-[color-mix(in_srgb,var(--bk-border)_40%,transparent)] bg-[var(--bk-surface-muted)] p-3.5 shadow-sm transition-all hover:border-[var(--bk-text-primary)] hover:bg-[var(--bk-surface)]">
                                <span className="mb-1.5 block text-[9px] font-black uppercase tracking-widest text-[var(--bk-text-muted)] opacity-60">Next Advice</span>
                                <p className="line-clamp-2 text-[11px] font-black uppercase leading-tight text-[var(--bk-text-primary)]">{liveNextAction.label}</p>
                             </div>
                          </div>
                          
                          <div className="mt-4 flex items-start gap-3 rounded-xl border border-[color-mix(in_srgb,var(--bk-border)_40%,transparent)] bg-[var(--bk-surface-soft)] p-3 text-[10px] text-[var(--bk-text-muted)] shadow-inner">
                             <div className="flex size-5 shrink-0 items-center justify-center rounded-lg border border-[var(--bk-border)] bg-[var(--bk-surface-strong)]">
                               <HelpCircle size={10} className="opacity-50" />
                             </div>
                             <p className="font-medium leading-relaxed">
                               <strong className="text-[9px] uppercase text-[var(--bk-text-primary)]">Preflight Feedback:</strong> {livePreflight.reason} · <span className="opacity-70">{shrink(livePreflight.detail)}</span>
                             </p>
                          </div>
                       </div>

                       {/* 右侧：操作指挥塔 (Control Tower) */}
                       <div className="bg-[var(--bk-surface-ghost)] p-6 lg:w-56 flex flex-col justify-center gap-3">
                          <Button 
                            variant={isLiveFlowRunning ? "bento-danger" : "bento"}
                            className="h-11 w-full rounded-xl text-xs font-black shadow-md transition-transform active:scale-95"
                            disabled={liveFlowAction !== null || liveBindAction || signalRuntimeAction !== null}
                            onClick={() => isLiveFlowRunning ? stopLiveFlow(account.id) : launchLiveFlow(account)}
                          >
                             {isLiveFlowRunning ? (
                               <div className="flex items-center gap-2"><Square size={14} fill="currentColor" /> 停止实盘流程</div>
                             ) : (
                               <div className="flex items-center gap-2"><Play size={14} fill="currentColor" /> 启动实盘流程</div>
                             )}
                          </Button>
                          <div className="grid grid-cols-2 gap-2">
                            <Button variant="bento-outline" className="h-9 rounded-lg bg-[var(--bk-surface)] text-[10px] font-black shadow-sm hover:border-[var(--bk-text-primary)]" onClick={() => syncLiveAccount(account.id)}>
                              同步账户
                            </Button>
                            <Button variant="bento-outline" className="h-9 rounded-lg bg-[var(--bk-surface)] text-[10px] font-black shadow-sm hover:border-[var(--bk-text-primary)]" onClick={() => setExpandedAccountId((current) => current === account.id ? null : account.id)}>
                              {accountDetailOpen ? "隐藏详情" : "账户详情"}
                            </Button>
                          </div>
                          {activeRuntime && (
                            <Button variant="bento-ghost" className="h-8 text-[10px] font-black text-[color-mix(in_srgb,var(--bk-status-success)_90%,transparent)] group-hover:underline hover:text-[var(--bk-status-success)]" onClick={() => jumpToSignalRuntimeSession(activeRuntime.id)}>
                              打开运行环境 <ArrowRight size={12} className="ml-1" />
                            </Button>
                          )}
                       </div>
                    </div>

                    {accountDetailOpen && (
                      <div className="animate-in slide-in-from-top-2 border-t border-[color-mix(in_srgb,var(--bk-border)_50%,transparent)] bg-[var(--bk-surface-faint)] px-5 pb-5 pt-4 duration-300">
                        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-8">
                           <div className="space-y-4">
                              <h5 className="border-l-2 border-[var(--bk-border)] pl-2 text-[10px] font-black uppercase tracking-widest text-[var(--bk-text-muted)]">资产分布与保证金 / ASSETS</h5>
                              <div className="grid grid-cols-3 gap-3">
                                 <div className="rounded-2xl border border-[var(--bk-border)] bg-[var(--bk-surface)] p-3 text-center shadow-sm">
                                    <span className="mb-1 block text-[8px] font-black uppercase text-[var(--bk-text-muted)]">Wallet</span>
                                    <strong className="text-sm tabular-nums text-[var(--bk-text-primary)]">{summary ? formatMaybeNumber(summary.walletBalance) : "--"}</strong>
                                 </div>
                                 <div className="rounded-2xl border border-[var(--bk-border)] bg-[var(--bk-surface)] p-3 text-center shadow-sm">
                                    <span className="mb-1 block text-[8px] font-black uppercase text-[var(--bk-text-muted)]">Margin</span>
                                    <strong className="text-sm tabular-nums text-[var(--bk-text-primary)]">{summary ? formatMaybeNumber(summary.marginBalance) : "--"}</strong>
                                 </div>
                                 <div className="rounded-2xl border border-[var(--bk-border)] bg-[var(--bk-surface)] p-3 text-center shadow-sm">
                                    <span className="mb-1 block text-[8px] font-black uppercase text-[var(--bk-text-muted)]">Available</span>
                                    <strong className="text-sm tabular-nums text-[var(--bk-text-primary)]">{summary ? formatMaybeNumber(summary.availableBalance) : "--"}</strong>
                                 </div>
                              </div>
                           </div>
                           <div className="space-y-4">
                              <h5 className="border-l-2 border-[var(--bk-border)] pl-2 text-[10px] font-black uppercase tracking-widest text-[var(--bk-text-muted)]">账户同步状态 / SNAPSHOT</h5>
                              <div className="grid grid-cols-3 gap-3">
                                 <div className="rounded-2xl border border-[var(--bk-border)] bg-[var(--bk-surface)] p-3 text-center shadow-sm">
                                    <span className="mb-1 block text-[8px] font-black uppercase text-[var(--bk-text-muted)]">Orders</span>
                                    <strong className="text-sm tabular-nums text-[var(--bk-text-primary)]">{String(syncSnapshot.orderCount ?? "0")}</strong>
                                 </div>
                                 <div className="rounded-2xl border border-[var(--bk-border)] bg-[var(--bk-surface)] p-3 text-center shadow-sm">
                                    <span className="mb-1 block text-[8px] font-black uppercase text-[var(--bk-text-muted)]">Fills</span>
                                    <strong className="text-sm tabular-nums text-[var(--bk-text-primary)]">{String(syncSnapshot.fillCount ?? "0")}</strong>
                                 </div>
                                 <div className="rounded-2xl border border-[var(--bk-border)] bg-[var(--bk-surface)] p-3 text-center shadow-sm">
                                    <span className="mb-1 block text-[8px] font-black uppercase text-[var(--bk-text-muted)]">Positions</span>
                                    <strong className="text-sm tabular-nums text-[var(--bk-text-primary)]">{String(syncSnapshot.positionCount ?? "0")}</strong>
                                 </div>
                              </div>
                           </div>
                           <div className="space-y-4">
                              <h5 className="border-l-2 border-[var(--bk-border)] pl-2 text-[10px] font-black uppercase tracking-widest text-[var(--bk-text-muted)]">诊断与事件流 / DIAGNOSTICS</h5>
                              <div className="space-y-2">
                                {buildAlertNotes(liveAlerts).map((item) => (
                                  <div key={`${account.id}-${item.title}`} className={`text-[10px] p-3 rounded-xl border-l-4 shadow-sm ${item.level === 'critical' ? 'bg-[color:color-mix(in_srgb,var(--bk-status-danger)_8%,transparent)] border-[var(--bk-status-danger)] text-[var(--bk-status-danger)]' : 'bg-[color:color-mix(in_srgb,var(--bk-status-warning)_10%,transparent)] border-[var(--bk-status-warning)] text-[var(--bk-status-warning)]'}`}>
                                    <strong className="uppercase">{item.title}:</strong> {item.detail}
                                  </div>
                                ))}
                                {buildSignalActionNotes(activeSignalAction).slice(0, 2).map((line) => (
                                  <div key={line} className="border-l border-[var(--bk-border)] py-0.5 pl-3 text-[10px] italic text-[var(--bk-text-muted)]">{line}</div>
                                ))}
                              </div>
                           </div>
                        </div>
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          ) : (
            <div className="flex flex-col items-center justify-center rounded-[32px] border-2 border-dashed border-[var(--bk-border)] p-16 text-[var(--bk-text-muted)] opacity-60">
              <Activity size={32} className="mb-4 opacity-20" />
              <p className="text-sm font-black uppercase tracking-widest">暂无活跃实盘账户</p>
              <p className="text-[11px] font-medium mt-1">需先新建账户并绑定交易所适配器</p>
            </div>
          )}
        </CardContent>
      </Card>

      <Card tone="bento" className="rounded-[24px] shadow-[var(--bk-shadow-card)]">
        <CardHeader className="pb-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="font-mono text-[10px] font-bold uppercase tracking-widest text-[var(--bk-status-success)]">Step 2 / Signal Pipeline</p>
              <CardTitle className="text-lg font-black text-[var(--bk-text-primary)]">第二步：接通信号源并启动运行时</CardTitle>
            </div>
            <div className="flex items-center gap-2 rounded-full border border-[var(--bk-border)] bg-[var(--bk-surface-strong)] px-3 py-1 text-[10px] font-bold text-[var(--bk-text-muted)]">
               <span>{signalCatalog?.sources?.length ?? 0} 个源</span>
               <Separator orientation="vertical" className="h-3 bg-[var(--bk-border)]" />
               <span>{signalRuntimeSessions.length} 个会话</span>
            </div>
          </div>
        </CardHeader>
        <CardContent className="space-y-8">
          {/* 启动模板区域 */}
          <div className="space-y-4">
             <div className="flex items-center gap-2">
               <Zap size={16} className="text-[var(--bk-status-warning)]" />
               <h4 className="text-xs font-black uppercase tracking-wider text-[var(--bk-text-primary)]">2.1 推荐启动模板</h4>
             </div>
             
             {launchTemplates.length > 0 ? (
               <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
                 {launchTemplates.map((tpl) => (
                   <div key={tpl.key} className="group flex min-h-[160px] flex-col justify-between rounded-[20px] border border-[var(--bk-border)] bg-[var(--bk-surface-strong)] p-4 transition-all duration-300 hover:translate-y-[-2px] hover:bg-[var(--bk-surface)] hover:shadow-lg">
                      <div className="space-y-3">
                         <div className="flex justify-between items-start pt-1">
                             <div className="flex items-center gap-1.5 min-w-0">
                                <div className="h-1.5 w-1.5 shrink-0 rounded-full bg-[var(--bk-status-success)]" />
                                <span className="truncate text-[13px] font-black leading-tight text-[var(--bk-text-primary)]">{tpl.name}</span>
                             </div>
                             <div className="flex items-center gap-1 select-none">
                               {activeTemplateKey === tpl.key && (
                                 <Badge variant="metal" className="h-3.5 bg-[var(--bk-status-success-soft)] text-[8px] text-[var(--bk-status-success)] border-[var(--bk-status-success-soft)]">
                                   RUNNING
                                 </Badge>
                               )}
                               <Badge variant="neutral" className="h-3.5 shrink-0 bg-[var(--bk-surface)] text-[8px] text-[var(--bk-text-primary)]">
                                 {tpl.symbol}
                               </Badge>
                             </div>
                         </div>
                         <p className="h-9 overflow-hidden text-[10px] font-medium leading-relaxed text-[var(--bk-text-muted)] line-clamp-2">{tpl.description}</p>
                      </div>
                      
                      <div className="space-y-3 pt-2">
                        <div className="flex flex-wrap gap-1">
                          {tpl.strategySignalBindings?.slice(0, 3).map((b: any, idx: number) => (
                            <div key={idx} className="rounded border border-[var(--bk-border-soft)] bg-[var(--bk-surface)] px-1.5 py-0.25 font-mono text-[7px] font-black uppercase tracking-tighter text-[var(--bk-text-muted)] shadow-xs">
                              {b.role}
                            </div>
                          ))}
                        </div>
                        <Button 
                          variant="bento-outline"
                          className="h-8 w-full rounded-lg bg-[var(--bk-surface)] text-[10px] font-black transition-all hover:border-transparent hover:bg-[var(--bk-surface-inverse)] hover:text-[var(--bk-text-contrast)]"
                          disabled={launchingTemplate !== null}
                          onClick={() => {
                            const isSwitching = activeLiveSession && activeTemplateKey !== tpl.key;
                            setConfirmConfig({
                              open: true,
                              title: isSwitching ? "确认切换发射模板？" : "确认应用发射模板？",
                              description: isSwitching 
                                ? `警告：你正在从 ${activeTemplateKey || "当前模板"} 切换到 ${tpl.name}。这将会清空策略下不属于新模板的所有绑定，并强制重启运行时以刷新计划（非热切换）。`
                                : `这将会为策略 "${tpl.strategyId}" 配置 ${tpl.symbol} 信号源。注意：此流程会触发运行时重启以应用新订阅（非热切换），请确认。`,
                              onConfirm: () => executeLaunchTemplate(tpl, quickLiveAccountId)
                            });
                          }}
                        >
                          {launchingTemplate === tpl.key ? "启动中..." : "一键切换并启动"}
                        </Button>
                      </div>
                   </div>
                 ))}
               </div>
             ) : (
               <div className="rounded-[24px] border-2 border-dashed border-[var(--bk-border)] p-12 text-center">
                 <p className="text-xs font-bold italic text-[var(--bk-text-muted)] opacity-40">正在获取推荐模板...</p>
               </div>
             )}
          </div>

          <Separator className="bg-[color-mix(in_srgb,var(--bk-border)_30%,transparent)]" />

          {/* 信号绑定结果表格 */}
          <div className="space-y-4">
            <h4 className="text-xs font-black uppercase tracking-wider text-[var(--bk-text-primary)]">当前信号绑定结果</h4>
            <div className="overflow-hidden rounded-[18px] border border-[var(--bk-border)] bg-[var(--bk-surface-strong)]">
              <Table tone="bento">
                <TableHeader className="bg-[var(--bk-surface-muted)]/45">
                  <TableRow className="border-[var(--bk-border)] hover:bg-transparent">
                    {["信号源", "角色", "代码 (Symbol)", "周期", "交易所", "状态"].map((h) => (
                      <TableHead key={h} className="px-3 py-3 text-[10px] font-black uppercase tracking-wider text-[var(--bk-text-muted)]">
                        {h}
                      </TableHead>
                    ))}
                  </TableRow>
                </TableHeader>
                <TableBody className="divide-y divide-[color-mix(in_srgb,var(--bk-border)_50%,transparent)]">
                  {strategySignalBindings.length > 0 ? (
                    strategySignalBindings.map((item, idx) => (
                      <TableRow key={idx} className="transition-colors hover:bg-[var(--bk-surface-faint)]">
                        <TableCell className="p-3 text-[11px] font-bold text-[var(--bk-text-primary)]">{item.sourceName}</TableCell>
                        <TableCell className="p-3">
                          <div className="rounded border border-[var(--bk-border-soft)] bg-[var(--bk-surface)] px-2 py-0.5 font-mono text-[9px] font-black uppercase text-[var(--bk-text-primary)] shadow-sm">{item.role}</div>
                        </TableCell>
                        <TableCell className="p-3 font-mono text-[11px] font-bold text-[var(--bk-status-success)]">{item.symbol || "--"}</TableCell>
                        <TableCell className="p-3 text-[11px] text-[var(--bk-text-muted)]">{displaySignalBindingTimeframe(item)}</TableCell>
                        <TableCell className="p-3 text-[11px] text-[var(--bk-text-muted)]">{item.exchange}</TableCell>
                        <TableCell className="p-3">
                          <div className={cn(
                            "rounded border px-2 py-0.5 font-mono text-[9px] font-black uppercase shadow-sm",
                            item.status === 'READY' 
                              ? "border-[var(--bk-status-success-soft)] text-[var(--bk-status-success)] bg-[var(--bk-surface)]" 
                              : "border-[var(--bk-status-warning-soft)] text-[var(--bk-status-warning)] bg-[var(--bk-surface)]"
                          )}>
                            {technicalStatusLabel(item.status)}
                          </div>
                        </TableCell>
                      </TableRow>
                    ))
                  ) : (
                    <TableRow>
                      <TableCell colSpan={6} className="p-8 text-center text-xs italic text-[var(--bk-text-muted)]">暂无策略绑定信息</TableCell>
                    </TableRow>
                  )}
                </TableBody>
              </Table>
            </div>
          </div>
        </CardContent>
      </Card>
      {/* 信号源目录 - 降噪处理 */}
      <Card tone="bento" className="rounded-[24px] shadow-[var(--bk-shadow-card)]">
        <CardHeader className="pb-4 flex flex-row items-center justify-between">
          <div className="flex items-center gap-2">
            <CardTitle className="text-xs font-black uppercase tracking-wider text-[var(--bk-text-primary)]">信号源目录与诊断说明</CardTitle>
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger>
                  <HelpCircle size={14} className="cursor-help text-[var(--bk-text-muted)]" />
                </TooltipTrigger>
                <TooltipContent className="w-80 border-[var(--bk-border)] bg-[var(--bk-surface-overlay-strong)] p-4">
                  <div className="space-y-3 text-[11px] text-[var(--bk-text-primary)]">
                    <p className="font-bold text-[var(--bk-status-success)]">操作建议</p>
                    {(signalCatalog?.notes ?? []).map((note, idx) => (
                      <p key={idx}>• {note}</p>
                    ))}
                  </div>
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>
          </div>
        </CardHeader>
        <CardContent>
          <div className="overflow-hidden rounded-[18px] border border-[var(--bk-border)] bg-[var(--bk-surface-strong)]">
            <Table tone="bento">
              <TableHeader className="bg-[var(--bk-surface-muted)]/45">
                <TableRow className="border-[var(--bk-border)] hover:bg-transparent">
                  {["名称", "交易所", "流类型", "角色", "环境", "传输方式"].map((h) => (
                    <TableHead key={h} className="px-3 py-3 text-[10px] font-black uppercase tracking-wider text-[var(--bk-text-muted)]">
                      {h}
                    </TableHead>
                  ))}
                </TableRow>
              </TableHeader>
              <TableBody className="divide-y divide-[color-mix(in_srgb,var(--bk-border)_50%,transparent)]">
                {signalCatalog?.sources?.length ? (
                  signalCatalog.sources.map((source, idx) => (
                    <TableRow key={idx} className="transition-colors hover:bg-[var(--bk-surface-faint)] text-[11px]">
                      <TableCell className="p-3 font-bold text-[var(--bk-text-primary)]">{source.name}</TableCell>
                      <TableCell className="p-3 text-[var(--bk-text-muted)]">{source.exchange}</TableCell>
                      <TableCell className="p-3 text-[var(--bk-text-muted)]">{source.streamType}</TableCell>
                      <TableCell className="p-3">
                        <div className="rounded border border-[var(--bk-border-soft)] bg-[var(--bk-surface)] px-2 py-0.5 font-mono text-[9px] font-black uppercase text-[var(--bk-text-primary)] shadow-sm">
                          {source.roles.join(", ")}
                        </div>
                      </TableCell>
                      <TableCell className="p-3 text-[var(--bk-text-muted)]">{source.environments.join(", ")}</TableCell>
                      <TableCell className="p-3 text-[var(--bk-text-muted)]">{source.transport}</TableCell>
                    </TableRow>
                  ))
                ) : (
                  <TableRow>
                    <TableCell colSpan={6} className="p-8 text-center text-xs italic text-[var(--bk-text-muted)]">信号源目录为空</TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </div>
        </CardContent>
      </Card>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
        {/* 运行时策略设置 */}
        <Card tone="bento" className="rounded-[24px] shadow-[var(--bk-shadow-card)]">
          <CardHeader className="pb-4 flex flex-row items-center justify-between">
            <div className="flex items-center gap-2">
              <div>
                <p className="font-mono text-[10px] font-bold uppercase tracking-widest text-[var(--bk-status-success)]">Step 3 / Policy</p>
                <CardTitle className="text-lg font-black text-[var(--bk-text-primary)]">第三步：执行策略设置</CardTitle>
              </div>
              <TooltipProvider>
                <Tooltip>
                  <TooltipTrigger>
                    <HelpCircle size={14} className="cursor-help text-[var(--bk-text-muted)]" />
                  </TooltipTrigger>
                  <TooltipContent className="w-80 border-[var(--bk-border)] bg-[var(--bk-surface-overlay-strong)] p-4">
                    <div className="space-y-3 text-[11px] text-[var(--bk-text-primary)]">
                      <p className="font-bold text-[var(--bk-status-success)]">当前生效阈值</p>
                      <div className="space-y-1">
                        <p>• 账户同步：{runtimePolicyValueLabel(platformRuntimePolicy?.liveAccountSyncFreshnessSeconds)}</p>
                        <p>• 运行时静默：{runtimePolicyValueLabel(platformRuntimePolicy?.runtimeQuietSeconds)}</p>
                      </div>
                    </div>
                  </TooltipContent>
                </Tooltip>
              </TooltipProvider>
            </div>
          </CardHeader>
          <CardContent className="space-y-6">
             <div className="space-y-5 rounded-[24px] border border-[var(--bk-border)] bg-[var(--bk-surface-strong)] p-6">
                <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                  <div className="space-y-1.5">
                    <label className="text-[9px] font-black uppercase whitespace-nowrap text-[var(--bk-text-muted)]">成交新鲜度(秒)</label>
                    <Input 
                      className="h-9 border-[var(--bk-border)] bg-[var(--bk-surface)] text-xs font-bold shadow-sm"
                      value={runtimePolicyForm.tradeTickFreshnessSeconds}
                      onChange={(e) => setRuntimePolicyForm(c => ({ ...c, tradeTickFreshnessSeconds: e.target.value }))}
                      placeholder="默认: 15"
                    />
                  </div>
                  <div className="space-y-1.5">
                    <label className="text-[9px] font-black uppercase whitespace-nowrap text-[var(--bk-text-muted)]">盘口新鲜度(秒)</label>
                    <Input 
                      className="h-9 border-[var(--bk-border)] bg-[var(--bk-surface)] text-xs font-bold shadow-sm"
                      value={runtimePolicyForm.orderBookFreshnessSeconds}
                      onChange={(e) => setRuntimePolicyForm(c => ({ ...c, orderBookFreshnessSeconds: e.target.value }))}
                      placeholder="默认: 10"
                    />
                  </div>
                  <div className="space-y-1.5">
                    <label className="text-[9px] font-black uppercase whitespace-nowrap text-[var(--bk-text-muted)]">信号新鲜度(秒)</label>
                    <Input 
                      className="h-9 border-[var(--bk-border)] bg-[var(--bk-surface)] text-xs font-bold shadow-sm"
                      value={runtimePolicyForm.signalBarFreshnessSeconds}
                      onChange={(e) => setRuntimePolicyForm(c => ({ ...c, signalBarFreshnessSeconds: e.target.value }))}
                      placeholder="默认: 30"
                    />
                  </div>
                  <div className="space-y-1.5">
                    <label className="text-[9px] font-black uppercase whitespace-nowrap text-[var(--bk-text-muted)]">同步过期(秒)</label>
                    <Input 
                      className="h-9 border-[var(--bk-border)] bg-[var(--bk-surface)] text-xs font-bold shadow-sm"
                      value={runtimePolicyForm.liveAccountSyncFreshnessSeconds}
                      onChange={(e) => setRuntimePolicyForm(c => ({ ...c, liveAccountSyncFreshnessSeconds: e.target.value }))}
                      placeholder="默认: 60"
                    />
                  </div>
                  <div className="space-y-1.5">
                    <label className="text-[9px] font-black uppercase whitespace-nowrap text-[var(--bk-text-muted)]">运行时静默(秒)</label>
                    <Input 
                      className="h-9 border-[var(--bk-border)] bg-[var(--bk-surface)] text-xs font-bold shadow-sm"
                      value={runtimePolicyForm.runtimeQuietSeconds}
                      onChange={(e) => setRuntimePolicyForm(c => ({ ...c, runtimeQuietSeconds: e.target.value }))}
                      placeholder="默认: 30"
                    />
                  </div>
                  <div className="space-y-1.5">
                    <label className="text-[9px] font-black uppercase whitespace-nowrap text-[var(--bk-text-muted)]">评估静默(秒)</label>
                    <Input 
                      className="h-9 border-[var(--bk-border)] bg-[var(--bk-surface)] text-xs font-bold shadow-sm"
                      value={runtimePolicyForm.strategyEvaluationQuietSeconds}
                      onChange={(e) => setRuntimePolicyForm(c => ({ ...c, strategyEvaluationQuietSeconds: e.target.value }))}
                      placeholder="默认: 0"
                    />
                  </div>
                  <div className="space-y-1.5">
                    <label className="text-[9px] font-black uppercase whitespace-nowrap text-[var(--bk-text-muted)]">模拟启动超时(秒)</label>
                    <Input 
                      className="h-9 border-[var(--bk-border)] bg-[var(--bk-surface)] text-xs font-bold shadow-sm"
                      value={runtimePolicyForm.paperStartReadinessTimeoutSeconds}
                      onChange={(e) => setRuntimePolicyForm(c => ({ ...c, paperStartReadinessTimeoutSeconds: e.target.value }))}
                      placeholder="默认: 5"
                    />
                  </div>
                  <div className="space-y-1.5">
                    <label className="text-[9px] font-black uppercase whitespace-nowrap text-[var(--bk-text-muted)]">派发模式</label>
                    <Select 
                       value={runtimePolicyForm.dispatchMode}
                       onValueChange={(val: any) => setRuntimePolicyForm(c => ({ ...c, dispatchMode: val }))}
                    >
                       <SelectTrigger tone="bento" className="h-9 bg-[var(--bk-surface)] px-2 text-xs font-bold shadow-sm">
                         <SelectValue />
                       </SelectTrigger>
                       <SelectContent tone="bento" className="bg-[var(--bk-surface)]">
                          <SelectItem value="manual-review" className="text-xs font-bold">人工审核</SelectItem>
                          <SelectItem value="auto-dispatch" className="text-xs font-bold">自动派发</SelectItem>
                       </SelectContent>
                    </Select>
                  </div>
                </div>
                <Button 
                   variant="bento"
                   className="h-10 w-full text-xs font-bold shadow-sm"
                   disabled={!!runtimePolicyAction}
                   onClick={updateRuntimePolicy}
                >
                   {runtimePolicyAction ? "保存中..." : "保存运行时策略"}
                </Button>
             </div>
          </CardContent>
        </Card>

        {/* 实盘会话管理区 */}
        <Card tone="bento" className="rounded-[24px] shadow-[var(--bk-shadow-card)]">
          <CardHeader className="pb-4 flex flex-row items-center justify-between">
            <div>
              <p className="font-mono text-[10px] font-bold uppercase tracking-widest text-[var(--bk-status-success)]">Operations</p>
              <CardTitle className="text-lg font-black text-[var(--bk-text-primary)]">实盘会话控制</CardTitle>
            </div>
            <Button variant="bento-outline" size="sm" className="h-8 text-[10px] font-bold" onClick={openMonitorStage}>
              打开监控台
            </Button>
          </CardHeader>
          <CardContent>
             <div className="space-y-3">
               {validLiveSessions.length > 0 ? (
                 validLiveSessions.map((session) => {
                   const isRunning = session.status === "RUNNING";
                   return (
                     <div key={session.id} className="group flex items-center justify-between rounded-[20px] border border-[var(--bk-border)] bg-[var(--bk-surface-strong)] p-4 transition-all hover:bg-[var(--bk-surface)]">
                        <div className="space-y-1">
                           <div className="flex items-center gap-2">
                              <span className="text-sm font-black text-[var(--bk-text-primary)]">
                                {session.alias || (session.id.length > 28 ? session.id.slice(0, 18) + '...' + session.id.slice(-6) : session.id)}
                              </span>
                              <Badge variant={isRunning ? "success" : "neutral"} className={`h-4 text-[8px] ${isRunning ? '' : 'bg-[var(--bk-text-muted)] text-[var(--bk-canvas)] border-transparent'}`}>
                                {session.status}
                              </Badge>
                           </div>
                           <p className="text-[10px] font-mono text-[var(--bk-text-muted)]">{String(getRecord(session.state).symbol || "--")} · {session.strategyId}</p>
                        </div>
                        <div className="flex items-center gap-1">
                           <Button 
                              variant="bento-ghost" 
                              size="icon" 
                              className={`h-8 w-8 ${isRunning ? 'text-[var(--bk-status-danger)]' : 'text-[var(--bk-status-success)]'}`}
                              disabled={liveSessionAction !== null}
                              onClick={() => runLiveSessionAction(session.id, isRunning ? "stop" : "start")}
                            >
                              {isRunning ? <Square size={14} /> : <Play size={14} fill="currentColor" />}
                           </Button>
                           <Button 
                              variant="bento-ghost" 
                              size="icon" 
                              className="h-8 w-8 text-[var(--bk-text-muted)]"
                              onClick={() => openLiveSessionModal(session)}
                            >
                              <Edit3 size={14} />
                           </Button>
                           <Button 
                              variant="bento-ghost" 
                              size="icon" 
                              className="h-8 w-8 text-[var(--bk-status-danger)] opacity-0 group-hover:opacity-100 transition-opacity"
                              disabled={liveSessionDeleteAction !== null}
                              onClick={() => openConfirm("删除会话？", "确定要彻底删除该实盘会话吗？删除后相关监控快照将无法恢复。", () => deleteLiveSession(session.id))}
                            >
                             <Trash2 size={14} />
                           </Button>
                        </div>
                     </div>
                   );
                 })
               ) : (
                 <div className="rounded-[24px] border-2 border-dashed border-[var(--bk-border)] p-12 text-center opacity-40">
                    <p className="text-xs font-bold italic text-[var(--bk-text-muted)]">暂无实盘会话</p>
                 </div>
               )}
             </div>


          </CardContent>
        </Card>

        <LiveTradePairsCard
          title="开平订单对追溯"
          description={
            primaryLiveSession
              ? `聚合焦点会话 ${primaryLiveSession.alias || shrink(primaryLiveSession.id)} 的 round-trip 交易，直接判断退出是否正常。`
              : '选中一个活跃实盘会话后，这里会显示可追溯的开平订单对与盈亏。'
          }
          pairs={primaryTradePairs.pairs}
          loading={primaryTradePairs.loading}
          error={primaryTradePairs.error}
        />
      </div>

      <AlertDialog 
        open={confirmConfig.open} 
        onOpenChange={(open) => {
          if (!open && liveSessionDeleteAction !== null) return;
          if (!open) setConfirmConfig(c => ({ ...c, open: false }));
        }}
      >
        <AlertDialogContent tone="bento" className="rounded-[32px] border-[var(--bk-border)] bg-[var(--bk-surface-overlay-strong)] p-8 shadow-2xl">
          <AlertDialogHeader>
            <AlertDialogTitle className="text-xl font-black text-[var(--bk-text-primary)]">{confirmConfig.title}</AlertDialogTitle>
            <AlertDialogDescription className="py-2 text-sm leading-relaxed text-[var(--bk-text-muted)]">
              {confirmConfig.description}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter className="pt-6">
            <AlertDialogCancel 
              disabled={liveSessionDeleteAction !== null}
              variant="bento-outline"
              className="h-11 rounded-xl px-6 font-bold"
            >
              取消
            </AlertDialogCancel>
            <Button 
              loading={liveSessionDeleteAction !== null}
              onClick={async () => {
                await confirmConfig.onConfirm();
                setConfirmConfig(c => ({ ...c, open: false }));
              }}
              variant="bento-danger"
              className="h-11 rounded-xl px-6 font-bold shadow-md"
            >
              确 认 执 行
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
