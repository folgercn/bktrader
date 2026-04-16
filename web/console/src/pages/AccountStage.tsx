import React, { useMemo, useState } from 'react';
import { HelpCircle, Zap, Edit3, Square, Trash2, Play, ArrowRight, ShieldCheck, Activity, RotateCw, AlertTriangle } from 'lucide-react';
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
    onConfirm: () => void;
  }>({ open: false, title: "", description: "", onConfirm: () => {} });

  const openConfirm = (title: string, description: string, onConfirm: () => void) => {
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
    <div className="absolute inset-0 overflow-y-auto p-8 space-y-8 bg-[#f3f0e7]">
      {/* 顶部总控 - 现代 Card 重构 */}
      <Card className="border-[#d8cfba] bg-[var(--panel)] shadow-[var(--shadow)] rounded-[32px] overflow-hidden">
        <div className="flex flex-col md:flex-row">
           <div className="flex-1 p-6 space-y-3">
             <div>
               <p className="text-[#0e6d60] text-[9px] font-black uppercase tracking-widest mb-1.5 font-mono">交易中心 / CONTROL CENTER</p>
               <h2 className="text-xl font-black text-[#1f2328] tracking-tight">准备账户，接通信号，启动实盘</h2>
             </div>
             <p className="text-xs text-[#687177] leading-relaxed max-w-2xl font-medium">
               按顺序完成账户准备、信号接通和实盘会话创建后，再进入监控台处理运行状态与人工干预。
             </p>
           </div>
           
           <div className="md:w-96 bg-[#fff8ea] border-l border-[#d8cfba] p-8 flex flex-col justify-center gap-6">
              <div className="space-y-1">
                <strong className="text-[11px] text-[#687177] uppercase font-bold">当前选中账户</strong>
                <div className="flex items-center gap-2">
                   <span className="text-sm font-bold text-[#1f2328]">{quickLiveAccount?.name ?? "--"}</span>
                   <Badge variant="outline" className="text-[9px] h-4 border-[#d8cfba] text-[#0e6d60]">{quickLiveAccount?.status ?? "--"}</Badge>
                </div>
                <p className="text-[10px] text-[#687177]">{quickLiveAccount?.exchange ?? "未解析交易所"}</p>
              </div>

              <div className="flex flex-wrap gap-2">
                <Button variant="outline" size="sm" className="h-9 border-[#d8cfba] bg-white text-[11px] font-bold text-[#1f2328]" onClick={openLiveAccountModal}>
                  新建账户
                </Button>
                <Button 
                   variant="outline" 
                   size="sm" 
                   className="h-9 border-[#d8cfba] bg-white text-[11px] font-bold text-[#1f2328]"
                   disabled={!quickLiveAccountId}
                   onClick={() => {
                     if (quickLiveAccountId) {
                       selectQuickLiveAccount(quickLiveAccountId);
                     }
                     openLiveBindingModal();
                   }}
                >
                  绑定适配器
                </Button>
                <Button 
                   size="sm" 
                   className="h-9 bg-[#0e6d60] hover:bg-[#0a5a4f] text-white text-[11px] font-bold shadow-md"
                   disabled={!quickLiveAccountId}
                   onClick={() => {
                     if (quickLiveAccountId) {
                       selectQuickLiveAccount(quickLiveAccountId);
                     }
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
      <Card className="border-[#d8cfba] bg-[var(--panel)] shadow-[var(--shadow)] rounded-[24px]">
        <CardHeader className="pb-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-[#0e6d60] text-[10px] font-bold uppercase tracking-widest font-mono">Step by Step</p>
              <CardTitle className="text-lg font-black text-[#1f2328]">建立一条可运行的实盘链路</CardTitle>
            </div>
            <Activity size={20} className="text-[#d8cfba] opacity-50" />
          </div>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
            {onboardingSteps.map((step, index) => (
              <div 
                key={step.key} 
                className={`relative p-4 rounded-[20px] border transition-all ${
                  step.status === "done" 
                    ? "bg-[#d9eee8] border-[#0e6d60]/20" 
                    : step.status === "current" 
                      ? "bg-[#fff8ea] border-[#d8cfba] shadow-sm" 
                      : "bg-[#f8f6f0] border-transparent opacity-60"
                }`}
              >
                <div className="flex items-center justify-between mb-3">
                   <div className={`flex items-center justify-center w-6 h-6 rounded-lg text-[10px] font-bold border ${
                     step.status === "done" ? "bg-[#0e6d60] text-white border-transparent" : "bg-white border-[#d8cfba] text-[#1f2328]"
                   }`}>
                     {index + 1}
                   </div>
                   <Badge variant="outline" className={`text-[9px] h-4 border-inherit font-bold ${
                     step.status === "done" ? "text-[#0e6d60]" : step.status === "current" ? "text-amber-700 font-black" : "text-[#687177]"
                   }`}>
                     {step.status === "done" ? "已完成" : step.status === "current" ? "进行中" : "待处理"}
                   </Badge>
                </div>
                <h4 className={`text-sm font-black mb-1.5 ${step.status === "pending" ? "text-[#687177]" : "text-[#1f2328]"}`}>{step.title}</h4>
                <p className="text-xs leading-relaxed text-[#687177] font-medium">{step.detail}</p>
              </div>
            ))}
          </div>
        </CardContent>
      </Card>

      <Card className="border-[#d8cfba] bg-[var(--panel)] shadow-[var(--shadow)] rounded-[24px]">
        <CardHeader className="pb-4 flex flex-row items-center justify-between">
          <div className="flex items-center gap-2">
            <div>
              <p className="text-[#0e6d60] text-[10px] font-bold uppercase tracking-widest font-mono">Step 1 / Accounts</p>
              <CardTitle className="text-lg font-black text-[#1f2328]">第一步：准备账户</CardTitle>
            </div>
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger>
                  <HelpCircle size={14} className="text-[#687177] cursor-help hover:text-[#0e6d60] transition-colors" />
                </TooltipTrigger>
                <TooltipContent className="w-80 p-4 border-[#d8cfba] bg-[#fffbf2] shadow-xl rounded-xl">
                  <div className="space-y-3">
                    <p className="text-[10px] text-[#0e6d60] uppercase font-bold">账户准备指南</p>
                    <div className="space-y-2 text-[11px] text-[#1f2328] leading-relaxed">
                      <p>• <strong>适配器绑定</strong>：账户需绑定具体的交易所适配器（如 Binance-Live）才能与实盘环境交互。</p>
                      <p>• <strong>数据同步</strong>：实盘账户需定期点击同步，以刷新订单、成交和资产快照。</p>
                      <p>• <strong>环境预检</strong>：系统会自动检查网络延迟、API 权限和资产充足度。</p>
                    </div>
                  </div>
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>
          </div>
          <Button variant="ghost" size="sm" className="text-[#0e6d60] font-bold h-8" onClick={openLiveAccountModal}>
            + 新建账户
          </Button>
        </CardHeader>
        <CardContent>
          {liveAccounts.length > 0 ? (
            <div className="grid grid-cols-1 gap-4">
              {liveAccounts.map((account) => {
                const binding = (account.metadata?.liveBinding as Record<string, unknown> | undefined) ?? {};
                const syncSnapshot = getRecord(getRecord(account.metadata).liveSyncSnapshot);
                const runtimeSessionsForAccount = signalRuntimeSessions.filter((item) => item.accountId === account.id);
                const activeRuntime = runtimeSessionsForAccount.find((item) => item.status === "RUNNING") ?? runtimeSessionsForAccount[0] ?? null;
                const activeRuntimeState = getRecord(activeRuntime?.state);
                const activeRuntimeSummary = getRecord(activeRuntimeState.lastEventSummary);
                const activeRuntimeMarket = deriveRuntimeMarketSnapshot(getRecord(activeRuntimeState.sourceStates), activeRuntimeSummary);
                const strategyBindings = (activeRuntime?.strategyId ? strategySignalBindingMap[activeRuntime.strategyId] : undefined) ?? strategySignalBindingMap[validLiveSessions.find((item) => item.accountId === account.id)?.strategyId ?? ""] ?? [];
                const activeRuntimeSourceSummary = deriveRuntimeSourceSummary(getRecord(activeRuntimeState.sourceStates), runtimePolicy);
                const activeSignalBarState = derivePrimarySignalBarState(getRecord(activeRuntimeState.signalBarStates));
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
                  <div key={account.id} className="group border border-[#d8cfba] bg-[#fff8ea] rounded-[20px] overflow-hidden shadow-sm hover:shadow-md transition-all">
                    <div className="p-5 flex flex-col md:flex-row gap-6">
                       {/* 账户核心信息 */}
                       <div className="flex-1 space-y-4">
                          <div className="flex items-center justify-between">
                             <div className="flex items-center gap-3">
                                <div className="w-10 h-10 rounded-xl bg-white border border-[#d8cfba] flex items-center justify-center font-black text-[#0e6d60]">
                                  {account.exchange?.charAt(0) || "A"}
                                </div>
                                <div>
                                  <h4 className="text-base font-black text-[#1f2328]">{account.name}</h4>
                                  <div className="flex items-center gap-2">
                                     <Badge variant="outline" className="text-[9px] h-4 border-[#d8cfba] bg-white text-[#1f2328]">{account.exchange}</Badge>
                                     <Badge variant="outline" className="text-[9px] h-4 border-[#d8cfba] bg-white text-[#1f2328]">{String(binding.adapterKey ?? "未适配")}</Badge>
                                  </div>
                                </div>
                             </div>
                             <div className="flex items-center gap-2">
                                <Badge className={`text-[10px] h-5 ${activeRuntimeReadiness.status === 'ready' ? 'bg-[#0e6d60]' : 'bg-amber-600'}`}>
                                  环境：{statusLabelZh(activeRuntimeReadiness.status)}
                                </Badge>
                                <Badge variant="secondary" className="text-[10px] h-5 bg-white border-[#d8cfba]">
                                  预检：{statusLabelZh(livePreflight.status)}
                                </Badge>
                             </div>
                          </div>

                          <div className="grid grid-cols-2 md:grid-cols-4 gap-4 py-2">
                             <div className="space-y-1">
                                <span className="text-[11px] text-[#687177] uppercase font-black tracking-tighter block">最新价</span>
                                <p className="text-base font-mono font-black text-[#1f2328] tracking-tighter">{formatMaybeNumber(activeRuntimeMarket.tradePrice)}</p>
                             </div>
                             <div className="space-y-1">
                                <span className="text-[11px] text-[#687177] uppercase font-black tracking-tighter block">心跳</span>
                                <p className="text-[10px] font-mono text-[#687177] font-bold">{formatTime(String(activeRuntimeState.lastHeartbeatAt ?? ""))}</p>
                             </div>
                             <div className="space-y-1">
                                <span className="text-[11px] text-[#687177] uppercase font-black tracking-tighter block">信号偏差</span>
                                <div className="h-6 flex items-center">
                                  <Badge className={`h-4.5 px-1.5 text-[9px] font-black tracking-widest ${signalActionTone(activeSignalAction.bias, activeSignalAction.state) === 'ready' ? 'bg-[#0e6d60]' : 'bg-rose-600'}`}>
                                    {String(activeSignalAction.bias || "--").toUpperCase()}
                                  </Badge>
                                </div>
                             </div>
                             <div className="space-y-1">
                                <span className="text-[11px] text-[#687177] uppercase font-black tracking-tighter block">操作建议</span>
                                <p className="text-[10px] text-[#1f2328] font-bold truncate">{liveNextAction.label}</p>
                             </div>
                          </div>

                          <div className="p-3 rounded-xl bg-white/50 border border-[#d8cfba]/50 text-[10px] text-[#687177] leading-relaxed">
                             <strong>预检反馈:</strong> {livePreflight.reason} · {shrink(livePreflight.detail)}
                          </div>
                       </div>

                       {/* 账户操作区 */}
                       <div className="md:w-48 flex flex-col gap-2">
                          <Button 
                            className={`w-full h-9 font-black text-[11px] shadow-sm rounded-xl ${isLiveFlowRunning ? 'bg-rose-600 hover:bg-rose-700' : 'bg-[#0e6d60] hover:bg-[#0a5a4f]'}`}
                            disabled={liveFlowAction !== null || liveBindAction || signalRuntimeAction !== null}
                            onClick={() => isLiveFlowRunning ? stopLiveFlow(account.id) : launchLiveFlow(account)}
                          >
                            {isLiveFlowRunning ? "停止实盘流程" : "启动实盘流程"}
                          </Button>
                          <div className="grid grid-cols-2 gap-2">
                            <Button variant="outline" className="h-9 text-[10px] border-[#d8cfba] bg-white" onClick={() => syncLiveAccount(account.id)}>
                              同步账户
                            </Button>
                            <Button variant="outline" className="h-9 text-[10px] border-[#d8cfba] bg-white" onClick={() => setExpandedAccountId((current) => current === account.id ? null : account.id)}>
                              {accountDetailOpen ? "收起" : "详情"}
                            </Button>
                          </div>
                          {activeRuntime && (
                            <Button variant="ghost" className="h-8 text-[10px] text-[#0e6d60] font-bold" onClick={() => jumpToSignalRuntimeSession(activeRuntime.id)}>
                              打开运行环境
                            </Button>
                          )}
                       </div>
                    </div>

                    {accountDetailOpen && (
                      <div className="px-5 pb-5 pt-2 border-t border-[#d8cfba]/50 bg-white/30">
                        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                           <div className="space-y-3">
                              <h5 className="text-[10px] font-black text-[#1f2328] uppercase">同步与资产快照</h5>
                              <div className="grid grid-cols-3 gap-2">
                                 <div className="p-3 rounded-xl bg-[#fff8ea] border border-[#d8cfba] text-center">
                                    <span className="block text-[8px] text-[#687177]">订单</span>
                                    <strong className="text-xs text-[#1f2328]">{String(syncSnapshot.orderCount ?? "0")}</strong>
                                 </div>
                                 <div className="p-3 rounded-xl bg-[#fff8ea] border border-[#d8cfba] text-center">
                                    <span className="block text-[8px] text-[#687177]">成交</span>
                                    <strong className="text-xs text-[#1f2328]">{String(syncSnapshot.fillCount ?? "0")}</strong>
                                 </div>
                                 <div className="p-3 rounded-xl bg-[#fff8ea] border border-[#d8cfba] text-center">
                                    <span className="block text-[8px] text-[#687177]">持仓</span>
                                    <strong className="text-xs text-[#1f2328]">{String(syncSnapshot.positionCount ?? "0")}</strong>
                                 </div>
                              </div>
                           </div>
                           <div className="space-y-2">
                              <h5 className="text-[10px] font-black text-[#1f2328] uppercase">实时诊断数据</h5>
                              <div className="space-y-1">
                                {buildAlertNotes(liveAlerts).map((item) => (
                                  <div key={`${account.id}-${item.title}`} className={`text-[10px] p-2 rounded-lg border ${item.level === 'critical' ? 'bg-rose-50 border-rose-200 text-rose-800' : 'bg-amber-50 border-amber-200 text-amber-800'}`}>
                                    <strong>{item.title}:</strong> {item.detail}
                                  </div>
                                ))}
                                {buildSignalActionNotes(activeSignalAction).slice(0, 2).map((line) => (
                                  <div key={line} className="text-[10px] text-[#687177] pl-2 border-l-2 border-[#d8cfba]">{line}</div>
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
            <div className="flex flex-col items-center justify-center p-12 border-2 border-dashed border-[#d8cfba] rounded-[24px] text-[#687177] opacity-40">
              <p className="text-sm font-bold">暂无实盘账户</p>
            </div>
          )}
        </CardContent>
      </Card>

      <Card className="border-[#d8cfba] bg-[var(--panel)] shadow-[var(--shadow)] rounded-[24px]">
        <CardHeader className="pb-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-[#0e6d60] text-[10px] font-bold uppercase tracking-widest font-mono">Step 2 / Signal Pipeline</p>
              <CardTitle className="text-lg font-black text-[#1f2328]">第二步：接通信号源并启动运行时</CardTitle>
            </div>
            <div className="flex items-center gap-2 bg-[#fff8ea] px-3 py-1 rounded-full border border-[#d8cfba] text-[10px] font-bold text-[#687177]">
               <span>{signalCatalog?.sources?.length ?? 0} 个源</span>
               <Separator orientation="vertical" className="h-3 bg-[#d8cfba]" />
               <span>{signalRuntimeSessions.length} 个会话</span>
            </div>
          </div>
        </CardHeader>
        <CardContent className="space-y-8">
          {/* 启动模板区域 */}
          <div className="space-y-4">
             <div className="flex items-center gap-2">
               <Zap size={16} className="text-amber-500" />
               <h4 className="text-xs font-black text-[#1f2328] uppercase tracking-wider">2.1 推荐启动模板</h4>
             </div>
             
             {launchTemplates.length > 0 ? (
               <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                 {launchTemplates.map((tpl) => (
                   <div key={tpl.key} className="group p-5 rounded-[20px] border border-[#d8cfba] bg-[#fff8ea] hover:bg-white hover:shadow-lg transition-all space-y-4">
                      <div className="flex justify-between items-start">
                         <div className="space-y-1">
                            <div className="flex items-center gap-2">
                               <div className="w-2 h-2 rounded-full bg-[#0e6d60]" />
                               <span className="text-sm font-black text-[#1f2328]">{tpl.name}</span>
                            </div>
                            <p className="text-[10px] text-[#687177] leading-relaxed line-clamp-2">{tpl.description}</p>
                         </div>
                         <Badge variant="outline" className="text-[9px] h-4 border-[#d8cfba] bg-white text-[#1f2328]">
                           {tpl.symbol} · {tpl.signalTimeframe}
                         </Badge>
                      </div>
                      
                      <div className="space-y-3 pt-2">
                        <div className="flex flex-wrap gap-1">
                          {tpl.strategySignalBindings?.slice(0, 3).map((b: any, idx: number) => (
                            <Badge key={idx} variant="secondary" className="text-[8px] h-3.5 bg-white border-[#d8cfba]/50 text-[#687177]">
                              {b.role}
                            </Badge>
                          ))}
                        </div>
                        <Button 
                          className="w-full h-8 bg-white border border-[#d8cfba] text-[#1f2328] hover:bg-[#0e6d60] hover:text-white hover:border-transparent text-[10px] font-bold transition-all"
                          disabled={launchingTemplate !== null}
                          onClick={() => executeLaunchTemplate(tpl, quickLiveAccountId)}
                        >
                          {launchingTemplate === tpl.key ? "启动中..." : "一键应用并启动"}
                        </Button>
                      </div>
                   </div>
                 ))}
               </div>
             ) : (
               <div className="p-12 text-center border-2 border-dashed border-[#d8cfba] rounded-[24px]">
                 <p className="text-xs text-[#687177] font-bold italic">正在获取推荐模板...</p>
               </div>
             )}
          </div>

          <Separator className="bg-[#d8cfba]/30" />

          {/* 信号绑定结果表格 */}
          <div className="space-y-4">
            <h4 className="text-xs font-black text-[#1f2328] uppercase tracking-wider">当前信号绑定结果</h4>
            <div className="rounded-[18px] border border-[#d8cfba] bg-[#fff8ea] overflow-hidden overflow-x-auto">
              <table className="w-full text-left border-collapse">
                <thead>
                  <tr className="bg-[#f8f6f0] border-b border-[#d8cfba]">
                    {["信号源", "角色", "代码 (Symbol)", "周期", "交易所", "状态"].map((h) => (
                      <th key={h} className="p-3 text-[10px] font-black text-[#687177] uppercase tracking-wider">{h}</th>
                    ))}
                  </tr>
                </thead>
                <tbody className="divide-y divide-[#d8cfba]/50">
                  {strategySignalBindings.length > 0 ? (
                    strategySignalBindings.map((item, idx) => (
<tr key={idx} className="hover:bg-white/50 transition-colors">
                        <td className="p-3 text-[11px] font-bold text-[#1f2328]">{item.sourceName}</td>
                        <td className="p-3">
                          <Badge variant="outline" className="text-[9px] h-4 border-[#d8cfba] bg-white text-[#1f2328]">{item.role}</Badge>
                        </td>
                        <td className="p-3 text-[11px] font-mono font-bold text-[#0e6d60]">{item.symbol || "--"}</td>
                        <td className="p-3 text-[11px] text-[#687177]">{displaySignalBindingTimeframe(item)}</td>
                        <td className="p-3 text-[11px] text-[#687177]">{item.exchange}</td>
                        <td className="p-3">
                          <Badge className={`text-[9px] h-4 bg-white border border-inherit ${item.status === 'READY' ? 'text-[#0e6d60] border-[#0e6d60]/20' : 'text-amber-700'}`}>
                            {technicalStatusLabel(item.status)}
                          </Badge>
                        </td>
                      </tr>
                    ))
                  ) : (
                    <tr>
                      <td colSpan={6} className="p-8 text-center text-xs text-[#687177] italic">暂无策略绑定信息</td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          </div>
        </CardContent>
      </Card>
      {/* 信号源目录 - 降噪处理 */}
      <Card className="border-[#d8cfba] bg-[var(--panel)] shadow-[var(--shadow)] rounded-[24px]">
        <CardHeader className="pb-4 flex flex-row items-center justify-between">
          <div className="flex items-center gap-2">
            <CardTitle className="text-xs font-black text-[#1f2328] uppercase tracking-wider">信号源目录与诊断说明</CardTitle>
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger>
                  <HelpCircle size={14} className="text-[#687177] cursor-help" />
                </TooltipTrigger>
                <TooltipContent className="w-80 p-4 border-[#d8cfba] bg-[#fffbf2]">
                  <div className="space-y-3 text-[11px] text-[#1f2328]">
                    <p className="font-bold text-[#0e6d60]">操作建议</p>
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
          <div className="rounded-[18px] border border-[#d8cfba] bg-[#fff8ea] overflow-hidden overflow-x-auto">
            <table className="w-full text-left border-collapse">
              <thead>
                <tr className="bg-[#f8f6f0] border-b border-[#d8cfba]">
                  {["名称", "交易所", "流类型", "角色", "环境", "传输方式"].map((h) => (
                    <th key={h} className="p-3 text-[10px] font-black text-[#687177] uppercase tracking-wider">{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody className="divide-y divide-[#d8cfba]/50">
                {signalCatalog?.sources?.length ? (
                  signalCatalog.sources.map((source, idx) => (
                    <tr key={idx} className="hover:bg-white/50 transition-colors text-[11px]">
                      <td className="p-3 font-bold text-[#1f2328]">{source.name}</td>
                      <td className="p-3 text-[#687177]">{source.exchange}</td>
                      <td className="p-3 text-[#687177]">{source.streamType}</td>
                      <td className="p-3">
                         <Badge variant="outline" className="text-[9px] h-4 border-[#d8cfba] bg-white">{source.roles.join(", ")}</Badge>
                      </td>
                      <td className="p-3 text-[#687177]">{source.environments.join(", ")}</td>
                      <td className="p-3 text-[#687177]">{source.transport}</td>
                    </tr>
                  ))
                ) : (
                  <tr>
                    <td colSpan={6} className="p-8 text-center text-xs text-[#687177] italic">信号源目录为空</td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </CardContent>
      </Card>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
        {/* 运行时策略设置 */}
        <Card className="border-[#d8cfba] bg-[var(--panel)] shadow-[var(--shadow)] rounded-[24px]">
          <CardHeader className="pb-4 flex flex-row items-center justify-between">
            <div className="flex items-center gap-2">
              <div>
                <p className="text-[#0e6d60] text-[10px] font-bold uppercase tracking-widest font-mono">Step 3 / Policy</p>
                <CardTitle className="text-lg font-black text-[#1f2328]">第三步：执行策略设置</CardTitle>
              </div>
              <TooltipProvider>
                <Tooltip>
                  <TooltipTrigger>
                    <HelpCircle size={14} className="text-[#687177] cursor-help" />
                  </TooltipTrigger>
                  <TooltipContent className="w-80 p-4 border-[#d8cfba] bg-[#fffbf2]">
                    <div className="space-y-3 text-[11px] text-[#1f2328]">
                      <p className="font-bold text-[#0e6d60]">当前生效阈值</p>
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
             <div className="space-y-5 p-6 rounded-[24px] bg-[#fff8ea] border border-[#d8cfba]">
                <div className="grid grid-cols-1 gap-4">
                  <div className="space-y-2">
                    <label className="text-[10px] font-black text-[#687177] uppercase">成交价格新鲜度 (秒)</label>
                    <Input 
                      className="bg-white border-[#d8cfba] h-10 text-xs font-bold"
                      value={runtimePolicyForm.tradeTickFreshnessSeconds}
                      onChange={(e) => setRuntimePolicyForm(c => ({ ...c, tradeTickFreshnessSeconds: e.target.value }))}
                    />
                  </div>
                  <div className="space-y-2">
                    <label className="text-[10px] font-black text-[#687177] uppercase">账户同步间隔 (秒)</label>
                    <Input 
                      className="bg-white border-[#d8cfba] h-10 text-xs font-bold"
                      value={runtimePolicyForm.liveAccountSyncFreshnessSeconds}
                      onChange={(e) => setRuntimePolicyForm(c => ({ ...c, liveAccountSyncFreshnessSeconds: e.target.value }))}
                    />
                  </div>
                  <div className="space-y-2">
                    <label className="text-[10px] font-black text-[#687177] uppercase">派发模式</label>
                    <Select 
                       value={runtimePolicyForm.dispatchMode}
                       onValueChange={(val: any) => setRuntimePolicyForm(c => ({ ...c, dispatchMode: val }))}
                    >
                       <SelectTrigger className="bg-white border-[#d8cfba] h-10 text-xs font-bold">
                         <SelectValue />
                       </SelectTrigger>
                       <SelectContent className="bg-white border-[#d8cfba]">
                          <SelectItem value="manual-review" className="text-xs">人工审核 (Manual)</SelectItem>
                          <SelectItem value="auto-dispatch" className="text-xs">自动派发 (Auto)</SelectItem>
                       </SelectContent>
                    </Select>
                  </div>
                </div>
                <Button 
                   className="w-full bg-[#0e6d60] hover:bg-[#0a5a4f] text-white font-bold text-xs h-10 shadow-sm"
                   disabled={runtimePolicyAction !== null}
                   onClick={updateRuntimePolicy}
                >
                   {runtimePolicyAction ? "保存中..." : "保存运行时策略"}
                </Button>
             </div>
          </CardContent>
        </Card>

        {/* 实盘会话管理区 */}
        <Card className="border-[#d8cfba] bg-[var(--panel)] shadow-[var(--shadow)] rounded-[24px]">
          <CardHeader className="pb-4 flex flex-row items-center justify-between">
            <div>
              <p className="text-[#0e6d60] text-[10px] font-bold uppercase tracking-widest font-mono">Operations</p>
              <CardTitle className="text-lg font-black text-[#1f2328]">实盘会话控制</CardTitle>
            </div>
            <Button variant="outline" size="sm" className="h-8 border-[#d8cfba] text-[#1f2328] font-bold text-[10px]" onClick={openMonitorStage}>
              打开监控台
            </Button>
          </CardHeader>
          <CardContent>
             <div className="space-y-3">
               {validLiveSessions.length > 0 ? (
                 validLiveSessions.map((session) => {
                   const isRunning = session.status === "RUNNING";
                   return (
                     <div key={session.id} className="p-4 rounded-[20px] bg-[#fff8ea] border border-[#d8cfba] flex items-center justify-between hover:bg-white transition-all group">
                        <div className="space-y-1">
                           <div className="flex items-center gap-2">
                              <span className="text-sm font-black text-[#1f2328]">{shrink(session.id)}</span>
                              <Badge className={`h-4 text-[8px] ${isRunning ? 'bg-[#0e6d60]' : 'bg-zinc-400'}`}>
                                {session.status}
                              </Badge>
                           </div>
                           <p className="text-[10px] text-[#687177] font-mono">{String(getRecord(session.state).symbol || "--")} · {session.strategyId}</p>
                        </div>
                        <div className="flex items-center gap-1">
                           <Button 
                              variant="ghost" 
                              size="icon" 
                              className={`h-8 w-8 ${isRunning ? 'text-rose-600' : 'text-[#0e6d60]'}`}
                              disabled={liveSessionAction !== null}
                              onClick={() => runLiveSessionAction(session.id, isRunning ? "stop" : "start")}
                            >
                              {isRunning ? <Square size={14} /> : <Play size={14} fill="currentColor" />}
                           </Button>
                           <Button 
                              variant="ghost" 
                              size="icon" 
                              className="h-8 w-8 text-[#687177]"
                              onClick={() => openLiveSessionModal(session)}
                            >
                              <Edit3 size={14} />
                           </Button>
                           <Button 
                              variant="ghost" 
                              size="icon" 
                              className="h-8 w-8 text-rose-600 opacity-0 group-hover:opacity-100 transition-opacity"
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
                 <div className="p-12 text-center border-2 border-dashed border-[#d8cfba] rounded-[24px] opacity-40">
                    <p className="text-xs text-[#687177] font-bold italic">暂无实盘会话</p>
                 </div>
               )}
             </div>

             <div className="mt-8 p-5 rounded-[24px] bg-[#d9eee8] border border-[#0e6d60]/10">
                <div className="flex flex-col gap-4">
                  <div className="space-y-1">
                    <h5 className="text-xs font-black text-[#1f2328]">运行状态已集成</h5>
                    <p className="text-[10px] text-[#0e6d60]/80 leading-relaxed">
                      配置完成后，请转至监控台查看详细的 K 线信号、活跃订单与资产对账详情。
                    </p>
                  </div>
                  <Button className="w-full bg-[#0e6d60] hover:bg-[#0a5a4f] text-white font-bold h-9 text-[11px]" onClick={openMonitorStage}>
                    立即体验监控台
                  </Button>
                </div>
             </div>
          </CardContent>
        </Card>
      </div>

      <AlertDialog 
        open={confirmConfig.open} 
        onOpenChange={(open) => {
          if (!open && liveSessionDeleteAction !== null) return;
          if (!open) setConfirmConfig(c => ({ ...c, open: false }));
        }}
      >
        <AlertDialogContent className="bg-[#fffbf2] border-[#d8cfba] rounded-[32px] p-8 shadow-2xl">
          <AlertDialogHeader>
            <AlertDialogTitle className="text-xl font-black text-[#1f2328]">{confirmConfig.title}</AlertDialogTitle>
            <AlertDialogDescription className="text-sm text-[#687177] leading-relaxed py-2">
              {confirmConfig.description}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter className="pt-6">
            <AlertDialogCancel 
              disabled={liveSessionDeleteAction !== null}
              className="h-11 px-6 rounded-xl border-[#d8cfba] font-bold text-[#1f2328]"
            >
              取消
            </AlertDialogCancel>
            <Button 
              loading={liveSessionDeleteAction !== null}
              onClick={confirmConfig.onConfirm}
              className="h-11 px-6 rounded-xl bg-rose-600 hover:bg-rose-700 text-white font-bold"
            >
              确 认 执 行
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}

