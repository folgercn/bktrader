import React, { useMemo, useState } from 'react';
import { useUIStore } from '../store/useUIStore';
import { useTradingStore } from '../store/useTradingStore';
import { SignalMonitorChart } from '../components/charts/SignalMonitorChart';
import { formatMoney, formatSigned, formatMaybeNumber, formatTime, shrink } from '../utils/format';
import { 
  getRecord, 
  getList,
  deriveSignalBarCandles,
  derivePrimarySignalBarState, 
  deriveRuntimeMarketSnapshot, 
  deriveSessionMarkers, 
  derivePaperSessionExecutionSummary,
  deriveHighlightedLiveSession,
  deriveLiveDispatchPreview,
  deriveLiveSessionFlow,
  deriveRuntimeReadiness,
  deriveRuntimeSourceSummary,
  deriveLiveSessionExecutionSummary,
  deriveLiveSessionHealth,
  buildTimelineNotes,
  liveSessionHealthTone,
  getNumber,
  runtimePolicyValueLabel,
  technicalStatusLabel
} from '../utils/derivation';
import { Card, CardHeader, CardTitle, CardContent } from '../components/ui/card';
import { Badge } from '../components/ui/badge';
import { Table, TableHeader, TableBody, TableHead, TableRow, TableCell } from '../components/ui/table';
import { Accordion, AccordionItem, AccordionTrigger, AccordionContent } from '../components/ui/accordion';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '../components/ui/tabs';
import { Button } from '../components/ui/button';
import { Activity, Layout, ShieldCheck, Zap, BarChart3, Clock, ArrowRightLeft, HeartPulse, LineChart, CandlestickChart, Compass, ShieldAlert, FileText, Layers, ChevronDown } from 'lucide-react';
import { Popover, PopoverContent, PopoverTrigger } from '../components/ui/popover';

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
  const summaries = useTradingStore(s => s.summaries);
  const runtimePolicy = useTradingStore(s => s.runtimePolicy);
  const monitorHealth = useTradingStore(s => s.monitorHealth);
  const accounts = useTradingStore(s => s.accounts);
  const strategySignalBindingMap = useTradingStore(s => s.strategySignalBindingMap);
  const liveSyncAction = useUIStore(s => s.liveSyncAction);
  const selectedSignalRuntimeId = useTradingStore(s => s.selectedSignalRuntimeId);
  const setSelectedSignalRuntimeId = useTradingStore(s => s.setSelectedSignalRuntimeId);

  // 1. 高亮会话选择逻辑
  const highlightedLiveSession = useMemo(
    () => {
      if (selectedSignalRuntimeId) {
        const sessionWithRuntime = liveSessions.find(s => 
          s.id === selectedSignalRuntimeId || 
          String(s.state?.signalRuntimeSessionId) === selectedSignalRuntimeId
        );
        if (sessionWithRuntime) {
          return deriveHighlightedLiveSession([sessionWithRuntime], orders, fills, positions);
        }
      }
      return deriveHighlightedLiveSession(liveSessions, orders, fills, positions);
    },
    [liveSessions, orders, fills, positions, selectedSignalRuntimeId]
  );

  // 2. 派生所有会话的全量状态（用于列表）
  const allSessionItems = useMemo(() => {
    return liveSessions.map(session => {
      const execution = deriveLiveSessionExecutionSummary(session, orders, fills, positions);
      const health = deriveLiveSessionHealth(session, execution);
      const summary = summaries.find(s => s.accountId === session.accountId) ?? null;
      return {
        session,
        execution,
        health,
        summary,
        isHighlighted: session.id === highlightedLiveSession?.session.id
      };
    }).sort((a, b) => {
      // 保持固定顺序：统一按创建时间倒序排列，不再随选中状态跳动
      return Date.parse(b.session.createdAt) - Date.parse(a.session.createdAt);
    });
  }, [liveSessions, highlightedLiveSession, orders, fills, positions, summaries]);

  const otherSessionItems = allSessionItems.filter(item => !item.isHighlighted);

  const handleSelectSession = (sid: string) => {
    const session = liveSessions.find(s => s.id === sid);
    const runtimeId = String(session?.state?.signalRuntimeSessionId ?? "");
    if (runtimeId) {
      setSelectedSignalRuntimeId(runtimeId);
    }
  };

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
  const sessionSymbol = String(monitorSession?.state?.symbol ?? "").trim().toUpperCase();

  const monitorBars = useMemo(() => {
    return deriveSignalBarCandles(getRecord(highlightedLiveRuntimeState.sourceStates), sessionSymbol);
  }, [highlightedLiveRuntimeState.sourceStates, sessionSymbol]);

  const monitorSignalState = derivePrimarySignalBarState(
    getRecord(highlightedLiveRuntimeState.signalBarStates),
    getRecord(monitorSessionState.lastStrategyEvaluationSignalBarStates)
  );
  const monitorMarket = deriveRuntimeMarketSnapshot(
    getRecord(monitorRuntimeState.sourceStates),
    getRecord(monitorRuntimeState.lastEventSummary),
    sessionSymbol
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
    deriveRuntimeSourceSummary(getRecord(highlightedLiveRuntimeState.sourceStates), runtimePolicy, sessionSymbol),
    { requireTick: true, requireOrderBook: false }
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
  const platformRuntimePolicy = monitorHealth?.runtimePolicy ?? runtimePolicy;
  const timelineLogs = buildTimelineNotes(monitorTimeline).slice(0, 50);

  const monitorSummaryItems = monitorSession ? [
    { label: "就绪预检", value: `${monitorRuntimeReadiness.status} · ${monitorRuntimeReadiness.reason}` },
    { label: "信号意图", value: `${String(monitorIntent.action ?? "无")} · ${String(monitorIntent.side ?? "--")}` },
    { label: "指令分发", value: `${String((monitorSession.state as any)?.dispatchMode ?? "--")} · 冷却 ${String((monitorSession.state as any)?.dispatchCooldownSeconds ?? "--")}s` },
    { label: "执行汇总", value: `订单 ${monitorExecutionSummary.orderCount} · 成交 ${monitorExecutionSummary.fillCount}` },
  ] : [];

  const monitorSections = monitorSession ? [
    {
      title: "运行与行情",
      items: [
        { label: "行情数据", value: `${formatMaybeNumber(monitorMarket.tradePrice)} · ${formatMaybeNumber(monitorMarket.bestBid)} / ${formatMaybeNumber(monitorMarket.bestAsk)}` },
      ],
    },
    {
      title: "信号与周期",
      items: [
        { label: "信号过滤", value: `周期 ${String(monitorSignalBarDecision.timeframe ?? "--")} · sma5 ${formatMaybeNumber(monitorSignalBarDecision.sma5)}` },
        { label: "信号备注", value: String(monitorSignalBarDecision.reason ?? "--") },
      ],
    },
    {
      title: "恢复与仓位",
      items: [
        { label: "策略持仓", value: `${String(monitorExecutionSummary.position?.side ?? "平仓")} · ${formatMaybeNumber(monitorExecutionSummary.position?.quantity)} @ ${formatMaybeNumber(monitorExecutionSummary.position?.entryPrice)}` },
        { label: "恢复统计", value: `止损 ${String((monitorSession.state as any)?.recoveredStopOrderCount ?? "--")} · 止盈 ${String((monitorSession.state as any)?.recoveredTakeProfitOrderCount ?? "--")}` },
      ],
    },
  ] : [];

 

  return (
    <div className="h-full overflow-y-auto p-6 space-y-8 animate-in fade-in duration-500">
      {/* 1. 主监控区 - 彻底迁移至 Card 现代化体系 */}
      {monitorSession ? (
      <Card tone="bento" className="rounded-[32px] overflow-hidden border-2 border-[var(--bk-border-strong)] shadow-2xl">
         <CardHeader className="border-b border-[var(--bk-border-soft)] bg-[var(--bk-surface-faint)] px-8 py-5">
           <div className="flex items-center justify-between">
             <div className="flex items-center gap-3">
               <div className="rounded-xl bg-[var(--bk-canvas-strong)] p-2 shadow-inner">
                 <LineChart className="size-5 text-[var(--bk-text-primary)]" />
               </div>
               <div>
                  <CardTitle className="text-xl font-black text-[var(--bk-text-primary)]">主监控台</CardTitle>
                  <p className="mt-0.5 text-[10px] font-bold uppercase tracking-widest text-[var(--bk-text-muted)]">Runtime K-Line & Execution Flow</p>
               </div>
             </div>
             <div className="flex items-center gap-4">
                <Badge variant="neutral" className="h-7 bg-[var(--bk-surface)] px-3 font-mono text-[10px]">
                  {monitorMode}
                </Badge>
             </div>
           </div>
         </CardHeader>
         <CardContent className="p-0">
            <div className="chart-shell relative h-[360px] overflow-hidden bg-[color-mix(in_srgb,var(--bk-surface-strong)_40%,transparent)]">
                {monitorBars.length > 0 ? (
                  <SignalMonitorChart candles={monitorBars} markers={monitorMarkers} />
                ) : (
                  <div className="absolute inset-0 flex flex-col items-center justify-center space-y-3 opacity-30">
                    <Activity className="size-16 animate-pulse" />
                    <span className="text-sm font-bold italic">等待实盘数据输入...</span>
                  </div>
                )}
            </div>
            
            <div className="grid grid-cols-2 gap-px border-t border-[var(--bk-border-soft)] bg-[color-mix(in_srgb,var(--bk-border)_40%,transparent)] md:grid-cols-4 lg:grid-cols-8">
                {[
                  { label: "模式", value: monitorMode, icon: Zap },
                  { label: "净值", value: formatMoney(monitorSummary?.netEquity), color: 'text-[var(--bk-text-primary)]' },
                  { label: "盈亏", value: formatSigned(monitorSummary?.unrealizedPnl ?? 0), color: (getNumber(monitorSummary?.unrealizedPnl) ?? 0) >= 0 ? 'text-[var(--bk-status-success)]' : 'text-[var(--bk-status-danger)]' },
                  { label: "方向", value: String(monitorExecutionSummary.position?.side ?? "FLAT"), color: 'font-black' },
                  { label: "数量", value: formatMaybeNumber(monitorExecutionSummary.position?.quantity) },
                  { label: "标记价", value: formatMaybeNumber(monitorExecutionSummary.position?.markPrice) },
                  { label: "当前价", value: formatMaybeNumber(monitorMarket.tradePrice), color: 'text-[var(--bk-status-success)]' },
                  { label: "SMA5", value: formatMaybeNumber(monitorSignalState.sma5) },
                ].map((item) => (
                  <div key={item.label} className="flex flex-col items-center justify-center bg-[var(--bk-surface-overlay)] p-4 transition-colors hover:bg-[var(--bk-surface)]">
                    <span className="mb-1 text-[9px] font-black uppercase tracking-tighter text-[var(--bk-text-muted)]">{item.label}</span>
                    <strong className={`text-[13px] tracking-tight ${item.color || 'text-[var(--bk-text-primary)]'}`}>{item.value}</strong>
                  </div>
                ))}
            </div>
         </CardContent>
      </Card>) : (
        <div className="flex flex-col items-center justify-center space-y-4 rounded-[32px] border-2 border-dashed border-[var(--bk-border)] bg-[color-mix(in_srgb,var(--bk-surface-strong)_30%,transparent)] p-20 opacity-40">
           <CandlestickChart className="size-12 text-[var(--bk-text-muted)]" />
           <p className="text-sm font-black uppercase tracking-wider italic text-[var(--bk-text-muted)]">需选择活跃焦点会话以同步实时 K 线数据</p>
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-12 gap-8">
        {/* 2. 交互与干预区 */}
        <Card tone="bento" className="lg:col-span-8 rounded-[32px] overflow-hidden border-2 border-[var(--bk-border-strong)] shadow-xl">
          <CardHeader className="border-b border-[var(--bk-border-soft)] bg-[var(--bk-surface-ghost)] px-8 py-5">
            <div className="flex items-center gap-3">
              <ShieldCheck className="size-5 text-[var(--bk-text-primary)]" />
              <CardTitle className="text-lg font-black text-[var(--bk-text-primary)]">运行监控与人工干预</CardTitle>
            </div>
          </CardHeader>
          <CardContent className="p-8">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
              {/* 左侧：优先会话详情 */}
              {highlightedLiveSession ? (
                <div className="group relative flex h-full flex-col overflow-hidden rounded-[24px] border-2 border-[var(--bk-border)] bg-[var(--bk-surface-strong)] p-6 shadow-lg">
                  

                  <div className="flex items-start justify-between mb-8">
                    <div className="space-y-2">
                      <div className="flex items-center gap-2">
                        <span className="text-[10px] font-black uppercase tracking-widest text-[var(--bk-text-muted)] opacity-70">Primary Session</span>
                        <Badge className="h-4 rounded-md border-0 bg-[var(--bk-surface-inverse)] px-1.5 text-[9px] font-black text-[var(--bk-text-contrast)]">
                          {highlightedLiveSession.health.status}
                        </Badge>
                      </div>
                      <h4 className="text-xl font-black tracking-tight text-[var(--bk-text-primary)]">活跃监控焦点会话</h4>
                    </div>
                  </div>
                  
                  <div className="flex flex-wrap gap-2 text-[10px] mb-6">
                    {/* 会话 ID 磁贴选择器 */}
                    <Popover>
                      <PopoverTrigger>
                        <button 
                          className={`flex w-fit max-w-[240px] items-center gap-2 rounded-lg border border-[var(--bk-border)] bg-[var(--bk-surface)] px-3 py-1.5 font-mono font-bold shadow-sm transition-all active:scale-95 ${allSessionItems.length > 1 ? 'cursor-pointer hover:bg-[var(--bk-surface-muted)]' : 'cursor-default'}`}
                          disabled={allSessionItems.length <= 1}
                        >
                          <span className="truncate">{highlightedLiveSession.session.id.length > 20 ? `${highlightedLiveSession.session.id.slice(0, 14)}...${highlightedLiveSession.session.id.slice(-6)}` : highlightedLiveSession.session.id}</span>
                          {allSessionItems.length > 1 && <ChevronDown className="size-3 shrink-0 text-[var(--bk-text-muted)] opacity-60" />}
                        </button>
                      </PopoverTrigger>
                      <PopoverContent align="start" className="isolate z-[60] w-[320px] rounded-[20px] border-2 border-[var(--bk-border)] bg-[var(--bk-surface-overlay-strong)] p-2 shadow-xl">
                         <div className="space-y-1.5">
                            <div className="mb-1 border-b border-[var(--bk-border-soft)] px-2 py-1.5">
                               <span className="text-[9px] font-black uppercase tracking-widest text-[var(--bk-text-muted)]">Switch Active Session</span>
                            </div>
                            {allSessionItems.map((item) => (
                              <div 
                                key={item.session.id} 
                                onClick={() => {
                                  handleSelectSession(item.session.id);
                                }}
                                className={`flex items-center justify-between p-3 rounded-xl border transition-all cursor-pointer group animate-in fade-in duration-200 ${
                                  item.isHighlighted 
                                    ? 'bg-[var(--bk-status-success-soft)] border-[var(--bk-status-success)] ring-2 ring-[color-mix(in_srgb,var(--bk-status-success)_10%,transparent)]' 
                                    : 'bg-[var(--bk-surface-overlay)] border-[var(--bk-border-soft)] hover:border-[color-mix(in_srgb,var(--bk-status-success)_50%,transparent)] hover:bg-[var(--bk-surface)]'
                                }`}
                              >
                                 <div className="flex items-center gap-3">
                                    <div className={`size-2 rounded-full ${item.health.status === "ready" ? "bg-[var(--bk-status-success)]" : String(item.session.state?.health).toLowerCase() === "recovering" ? "bg-[var(--bk-status-warning)] animate-pulse" : String(item.session.state?.health).toLowerCase() === "stale-after-reconnect" ? "bg-[var(--bk-status-danger)]" : "bg-rose-500"} ${item.isHighlighted ? "ring-4 ring-[color-mix(in_srgb,var(--bk-status-success)_20%,transparent)]" : ""}`} />
                                    <div className="flex flex-col">
                                       <span className={`text-[10px] font-black ${item.isHighlighted ? 'text-[var(--bk-status-success)]' : 'text-[var(--bk-text-primary)]'}`}>{item.session.id.length > 20 ? `${item.session.id.slice(0, 14)}...${item.session.id.slice(-6)}` : item.session.id}</span>
                                       <span className={`text-[8px] font-mono ${item.isHighlighted ? 'text-[color-mix(in_srgb,var(--bk-status-success)_70%,black)]' : 'text-[var(--bk-text-muted)]'}`}>{String(item.session.state?.symbol ?? "--")} · {String(item.session.state?.signalTimeframe ?? "--")}</span>
                                    </div>
                                 </div>
                                 <div className="text-right">
                                    <span className={`text-[10px] font-black block tabular-nums ${
                                      (getNumber(item.summary?.unrealizedPnl) ?? 0) >= 0 ? 'text-[var(--bk-status-success)]' : 'text-[var(--bk-status-danger)]'
                                    }`}>
                                       {formatSigned(item.summary?.unrealizedPnl ?? 0)}
                                    </span>
                                    <span className={`mt-0.5 block text-[8px] font-bold uppercase opacity-50 ${item.isHighlighted ? 'text-[color-mix(in_srgb,var(--bk-status-success)_60%,black)]' : 'text-[var(--bk-text-muted)]'}`}>{String(item.execution.position?.side ?? "FLAT")}</span>
                                 </div>
                              </div>
                            ))}
                         </div>
                      </PopoverContent>
                    </Popover>

                    <span className="rounded-lg border border-[var(--bk-border)] bg-[var(--bk-surface)] px-2 py-1 font-mono shadow-sm">{highlightedLiveSession.session.accountId}</span>
                    <Badge variant="success" className="font-black">
                      {String(highlightedLiveSession.session.state?.signalTimeframe ?? "--")}
                    </Badge>
                  </div>

                  <div className="space-y-4 rounded-2xl border border-[var(--bk-border-soft)] bg-[var(--bk-surface-muted)] p-5 shadow-inner">
                    <p className="text-[12px] font-medium leading-relaxed text-[var(--bk-text-primary)]">
                      <span className="mr-2 font-black text-[var(--bk-text-muted)] opacity-50">HEALTH_LOG:</span> 
                      {highlightedLiveSession.health.detail}
                    </p>
                    <div className="grid grid-cols-2 gap-4 pt-2">
                      <div className="space-y-1">
                        <span className="text-[9px] font-black uppercase text-[var(--bk-text-muted)]">执行统计</span>
                        <strong className="text-[11px] block font-black">Orders {highlightedLiveSession.execution.orderCount} · Fills {highlightedLiveSession.execution.fillCount}</strong>
                      </div>
                      <div className="space-y-1">
                        <span className="text-[9px] font-black uppercase text-[var(--bk-text-muted)]">持仓方向</span>
                        <strong className="block text-[11px] font-black text-[var(--bk-status-success)]">{String(monitorExecutionSummary.position?.side ?? "FLAT")}</strong>
                      </div>
                    </div>
                  </div>

                  {/* 新增：从右侧搬迁过来的辅助信息，用于平衡高度 */}
                  <div className="mt-auto grid grid-cols-2 gap-4 border-t-2 border-[color-mix(in_srgb,var(--bk-border)_30%,transparent)] pt-8">
                      <div className="group flex flex-col justify-between rounded-[28px] border-2 border-[color-mix(in_srgb,var(--bk-border)_60%,transparent)] bg-[var(--bk-surface-muted)] p-6 shadow-sm transition-all hover:bg-[var(--bk-surface)]">
                         <div className="flex items-center gap-2 mb-4">
                            <div className="rounded-lg bg-[var(--bk-canvas-strong)] p-1.5">
                               <ShieldAlert className="size-4 text-[var(--bk-text-primary)]" />
                            </div>
                            <span className="text-[11px] font-black uppercase tracking-widest text-[var(--bk-text-muted)]">状态恢复</span>
                         </div>
                         <div className="flex items-center justify-between text-[11px] font-black">
                            <div className="flex flex-col space-y-1">
                              <span className="text-[var(--bk-status-danger)]">SL: {String((monitorSession?.state as any)?.recoveredStopOrderCount ?? "0")}</span>
                              <span className="text-[var(--bk-status-success)]">PT: {String((monitorSession?.state as any)?.recoveredTakeProfitOrderCount ?? "0")}</span>
                            </div>
                            <div className="text-[9px] font-black text-[var(--bk-text-muted)] opacity-40">RECOVERY</div>
                         </div>
                      </div>

                      <div className="group flex flex-col justify-between rounded-[28px] border-2 border-[color-mix(in_srgb,var(--bk-border)_60%,transparent)] bg-[var(--bk-surface-muted)] p-6 shadow-sm transition-all hover:bg-[var(--bk-surface)]">
                         <div className="flex items-center gap-2 mb-4">
                            <div className="rounded-lg border border-[var(--bk-border-soft)] bg-[var(--bk-surface-strong)] p-1.5">
                               <FileText className="size-4 text-[var(--bk-text-primary)]" />
                            </div>
                            <span className="text-[11px] font-black uppercase tracking-widest text-[var(--bk-text-muted)]">执行备注</span>
                         </div>
                         <p className="line-clamp-3 text-[11px] font-bold leading-tight text-[var(--bk-text-primary)]">
                           {String(monitorSignalBarDecision.reason ?? "暂无执行信号排队或阻断说明")}
                         </p>
                      </div>
                  </div>

 
                </div>
              ) : (
                <div className="flex flex-col items-center justify-center space-y-4 rounded-[24px] border-2 border-dashed border-[var(--bk-border)] bg-[color-mix(in_srgb,var(--bk-surface-strong)_50%,transparent)] p-20 text-center text-sm font-bold italic text-[var(--bk-text-muted)]">
                  <div className="rounded-full bg-[var(--bk-surface-faint)] p-4">
                    <Activity className="size-8 opacity-20" />
                  </div>
                  <span>当前没有活跃实盘会话</span>
                </div>
              )}

              {/* 右侧：折叠详情 */}
              <div className="space-y-6">
                <div className="flex items-center justify-between border-b-2 border-[var(--bk-border)] pb-3">
                  <h4 className="text-sm font-black uppercase tracking-wider text-[var(--bk-text-primary)]">监控遥测明细</h4>
                  <Badge variant={monitorSession ? "success" : "neutral"} className={`text-[10px] font-black ${monitorSession ? '' : 'bg-[var(--bk-surface-muted)] text-[var(--bk-text-muted)] border-[var(--bk-border-soft)]'}`}>
                    {monitorSession ? "CONNECTED" : "IDLE"}
                  </Badge>
                </div>
                
                {monitorSession ? (
                  <div className="space-y-6">
                    <div className="grid grid-cols-2 gap-3">
                      {monitorSummaryItems.map((item) => (
                        <div key={item.label} className="rounded-2xl border border-[color-mix(in_srgb,var(--bk-border)_60%,transparent)] bg-[var(--bk-surface-muted)] p-3 shadow-sm transition-all hover:bg-[var(--bk-surface-soft)] hover:shadow-md">
                          <span className="mb-1 block text-[9px] font-black uppercase tracking-tighter text-[var(--bk-text-muted)] opacity-70">{item.label}</span>
                          <strong className="block truncate text-[11px] font-bold text-[var(--bk-text-primary)]">{item.value}</strong>
                        </div>
                      ))}
                    </div>
                    <div className="grid grid-cols-2 gap-4">
                      {/* 行 1: 核心行情 - 全宽 */}
                      <div className="group col-span-2 rounded-[28px] border-2 border-[var(--bk-border)] bg-gradient-to-br from-white to-[var(--bk-surface-strong)] p-6 shadow-sm transition-all hover:shadow-md">
                        <div className="flex items-center justify-between mb-4">
                          <div className="flex items-center gap-2">
                             <div className="rounded-lg bg-[var(--bk-canvas-strong)] p-1.5">
                               <CandlestickChart className="size-4 text-[var(--bk-text-primary)]" />
                             </div>
                             <span className="text-[11px] font-black uppercase tracking-widest text-[var(--bk-text-muted)]">行情核心分析</span>
                          </div>
                          <Badge variant="neutral" className="bg-[var(--bk-surface)] text-[10px] font-mono text-[var(--bk-status-success)]">LATEST</Badge>
                        </div>
                        <div className="flex items-end justify-between">
                          <div className="space-y-2">
                            <span className="block text-[10px] font-bold text-[var(--bk-text-muted)] opacity-60">PRICE / SPREAD</span>
                            <strong className="text-3xl font-black leading-none tracking-tighter tabular-nums text-[var(--bk-text-primary)]">
                              {formatMaybeNumber(monitorMarket.tradePrice)}
                            </strong>
                          </div>
                          <div className="text-right">
                             <span className="block text-[16px] font-mono font-bold leading-none text-[var(--bk-status-success)] antialiased">
                               {formatMaybeNumber(monitorMarket.bestBid)} / {formatMaybeNumber(monitorMarket.bestAsk)}
                             </span>
                             <span className="mt-1.5 block text-[10px] font-black uppercase text-[var(--bk-text-muted)]">Depth liquidity</span>
                          </div>
                        </div>
                      </div>

                      {/* 行 2: 策略状态 - 双列并列 */}
                      <div className="group rounded-[28px] border-2 border-[color-mix(in_srgb,var(--bk-border)_60%,transparent)] bg-[var(--bk-surface-muted)] p-6 shadow-sm transition-all hover:bg-[var(--bk-surface)]">
                         <div className="flex items-center gap-2 mb-4">
                            <div className="rounded-lg bg-[var(--bk-status-success-soft)] p-1.5">
                               <Activity className="size-4 text-[var(--bk-status-success)]" />
                            </div>
                            <span className="text-[11px] font-black uppercase tracking-widest text-[var(--bk-text-muted)]">策略持仓</span>
                         </div>
                         <div className="space-y-2">
                            <strong className={`block text-xl font-black leading-tight ${String(monitorExecutionSummary.position?.side).includes('LONG') ? 'text-[var(--bk-status-success)]' : String(monitorExecutionSummary.position?.side).includes('SHORT') ? 'text-[var(--bk-status-danger)]' : 'text-[var(--bk-text-primary)]'}`}>
                              {String(monitorExecutionSummary.position?.side ?? "FLAT")}
                            </strong>
                            <span className="block text-[11px] font-mono font-bold text-[var(--bk-text-muted)]">
                              {formatMaybeNumber(monitorExecutionSummary.position?.quantity)} @ {formatMaybeNumber(monitorExecutionSummary.position?.entryPrice)}
                            </span>
                         </div>
                      </div>

                      <div className="group rounded-[28px] border-2 border-[color-mix(in_srgb,var(--bk-border)_60%,transparent)] bg-[var(--bk-surface-muted)] p-6 shadow-sm transition-all hover:bg-[var(--bk-surface)]">
                         <div className="flex items-center gap-2 mb-4">
                            <div className="rounded-lg bg-[var(--bk-canvas-strong)] p-1.5">
                               <Compass className="size-4 text-[var(--bk-text-primary)]" />
                            </div>
                            <span className="text-[11px] font-black uppercase tracking-widest text-[var(--bk-text-muted)]">信号分析</span>
                         </div>
                         <div className="space-y-2">
                            <strong className="block text-xl font-black leading-tight tabular-nums text-[var(--bk-text-primary)]">
                              {formatMaybeNumber(monitorSignalBarDecision.sma5)}
                            </strong>
                            <span className="block text-[11px] font-mono font-bold text-[var(--bk-text-muted)]">
                              PERIOD: {String(monitorSignalBarDecision.timeframe ?? "--")}
                            </span>
                         </div>
                      </div>
                    </div>
                  </div>
                ) : (
                  <div className="flex h-64 flex-col items-center justify-center space-y-3 rounded-[24px] border-2 border-dashed border-[var(--bk-border-soft)] bg-[var(--bk-surface-ghost)] opacity-40">
                    <Layout className="size-8" />
                    <p className="text-xs font-bold italic">请在 Dock 中选中活跃会话</p>
                  </div>
                )}
              </div>
            </div>

            {/* 底部：常驻终端时间线 - 奶油风格 */}
            <div className="mt-8 border-t-2 border-[var(--bk-border-soft)] pt-8">
               <div className="flex items-center justify-between mb-4">
                 <div className="flex items-center gap-2">
                   <div className="size-2 animate-pulse rounded-full bg-[var(--bk-status-success)]" />
                   <h5 className="text-[11px] font-black uppercase tracking-widest text-[var(--bk-text-primary)]">Execution Timeline Terminal</h5>
                 </div>
                 <Badge variant="neutral" className="bg-[var(--bk-surface-strong)] text-[9px] font-mono text-[var(--bk-text-muted)]">
                   AUTO_SCROLL ENABLED
                 </Badge>
               </div>
               <div className="custom-scrollbar h-[280px] overflow-y-auto rounded-[24px] border-2 border-[var(--bk-border)] bg-[var(--bk-surface-soft)] p-5 shadow-inner">
                  {timelineLogs.length > 0 ? timelineLogs.map((line: string, idx: number) => (
                    <div key={idx} className="mb-1.5 border-l-2 border-[var(--bk-canvas-strong)] py-0.5 pl-3 font-mono text-[10px] leading-normal text-[var(--bk-text-primary)] transition-all hover:border-[var(--bk-status-success)] hover:bg-[var(--bk-surface-strong)]">
                      <span className="mr-3 font-bold tabular-nums text-[var(--bk-text-muted)] opacity-40">[{idx.toString().padStart(2, '0')}]</span>
                      <span className="opacity-90">{line}</span>
                    </div>
                  )) : (
                    <div className="flex h-full items-center justify-center font-mono text-[10px] italic text-[var(--bk-text-muted)] opacity-40">
                      SYSTEM: Waiting for runtime events to populate timeline...
                    </div>
                  )}
               </div>
            </div>
          </CardContent>
        </Card>

        {/* 3. 待处理订单 - 订单同步表格 */}
        <Card tone="bento" className="lg:col-span-4 flex flex-col rounded-[32px] overflow-hidden border-2 border-[var(--bk-border-strong)] shadow-xl">
          <CardHeader className="border-b border-[var(--bk-border-soft)] bg-[var(--bk-surface-ghost)] px-6 py-5">
            <div className="flex items-center gap-3">
              <ArrowRightLeft className="size-5 text-[var(--bk-text-primary)]" />
              <CardTitle className="text-lg font-black text-[var(--bk-text-primary)]">待同步订单</CardTitle>
            </div>
          </CardHeader>
          <CardContent className="p-6 flex-1 overflow-hidden">
             <div className="flex h-full flex-col overflow-hidden rounded-[24px] border-2 border-[var(--bk-border)] bg-[var(--bk-surface-faint)] shadow-inner">
               <Table tone="bento">
                 <TableHeader className="border-b-2 border-[var(--bk-border)] bg-[color-mix(in_srgb,var(--bk-canvas-strong)_60%,transparent)]">
                   <TableRow className="hover:bg-transparent">
                     <TableHead className="h-10 px-5 text-[10px] font-black uppercase">Symbol/Order</TableHead>
                     <TableHead className="h-10 px-5 text-right text-[10px] font-black uppercase">Action</TableHead>
                   </TableRow>
                 </TableHeader>
                 <TableBody className="overflow-y-auto">
                   {syncableLiveOrders.length > 0 ? (
                     syncableLiveOrders.map((order) => (
                       <TableRow key={order.id} className="border-b border-[color-mix(in_srgb,var(--bk-border)_30%,transparent)] transition-colors hover:bg-[color-mix(in_srgb,var(--bk-surface-strong)_80%,transparent)]">
                         <TableCell className="px-5 py-4">
                            <div className="flex flex-col">
                              <span className="text-xs font-black text-[var(--bk-status-success)]">{order.symbol}</span>
                              <span className="mt-1 text-[9px] font-mono text-[var(--bk-text-muted)]">{shrink(order.id)} · {order.side}</span>
                            </div>
                         </TableCell>
                         <TableCell className="px-5 text-right">
                            <Button 
                              size="sm" 
                              variant="bento-outline" 
                              className="h-8 rounded-xl border-2 bg-[var(--bk-surface)] px-4 text-[10px] font-black shadow-sm active:scale-95"
                              disabled={liveSyncAction !== null}
                              onClick={() => syncLiveOrder(order.id)}
                            >
                              Sync
                            </Button>
                         </TableCell>
                       </TableRow>
                     ))
                   ) : (
                     <TableRow>
                       <TableCell colSpan={2} className="h-40 text-center text-xs font-medium italic text-[var(--bk-text-muted)]">
                         当前网络环境无孤立订单
                       </TableCell>
                     </TableRow>
                   )}
                 </TableBody>
               </Table>
             </div>
          </CardContent>
        </Card>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
        {/* 4. 平台健康诊断 */}
        <Card tone="bento" className="rounded-[32px] overflow-hidden border-2 border-[var(--bk-border-strong)] shadow-xl">
          <CardHeader className="flex flex-row items-center justify-between border-b border-[var(--bk-border-soft)] bg-[var(--bk-surface-ghost)] px-8 pb-4 pt-6">
            <div className="flex items-center gap-3">
              <HeartPulse className="size-5 text-[var(--bk-text-primary)]" />
              <CardTitle className="text-xl font-black text-[var(--bk-text-primary)]">平台健康总览</CardTitle>
            </div>
            <Badge variant="success" className="rounded-xl border-2 px-3 py-1 text-[11px] font-black shadow-sm">
               {technicalStatusLabel(monitorHealth?.status ?? "--")}
            </Badge>
          </CardHeader>
          <CardContent className="px-8 py-8">
            {monitorHealth ? (
              <div className="grid grid-cols-3 md:grid-cols-6 gap-4">
                {[
                  { label: "Alerts", value: monitorHealth.alertCounts.total, icon: 'total' },
                  { label: "Critical", value: monitorHealth.alertCounts.critical, color: 'text-[var(--bk-status-danger)]', bg: 'bg-[color:color-mix(in_srgb,var(--bk-status-danger)_8%,transparent)]' },
                  { label: "Warning", value: monitorHealth.alertCounts.warning, color: 'text-[var(--bk-status-warning)]', bg: 'bg-[color:color-mix(in_srgb,var(--bk-status-warning)_10%,transparent)]' },
                  { label: "Quiet", value: runtimePolicyValueLabel(platformRuntimePolicy?.runtimeQuietSeconds) },
                  { label: "Eval", value: runtimePolicyValueLabel(platformRuntimePolicy?.strategyEvaluationQuietSeconds) },
                  { label: "Sync", value: runtimePolicyValueLabel(platformRuntimePolicy?.liveAccountSyncFreshnessSeconds) },
                ].map((item) => (
                  <div key={item.label} className={`rounded-[20px] border-2 border-[var(--bk-border)] p-4 shadow-sm transition-all hover:scale-105 ${item.bg || 'bg-[var(--bk-surface)]'}`}>
                    <span className="mb-2 block text-[10px] font-black uppercase tracking-tighter text-[var(--bk-text-muted)] opacity-60">{item.label}</span>
                    <strong className={`block text-xl font-black tracking-tighter ${item.color || 'text-[var(--bk-text-primary)]'}`}>
                      {String(item.value ?? 0)}
                    </strong>
                  </div>
                ))}
              </div>
            ) : (
              <div className="rounded-[24px] border-2 border-dashed border-[var(--bk-border)] bg-[color-mix(in_srgb,var(--bk-surface-strong)_40%,transparent)] p-12 text-center text-sm font-bold italic text-[var(--bk-text-muted)]">
                健康诊断模块正在预热中...
              </div>
            )}
          </CardContent>
        </Card>

        {/* 5. 记录中心与人工干预 */}
        <Card tone="bento" className="rounded-[32px] overflow-hidden border-2 border-[var(--bk-border-strong)] shadow-xl">
          <Tabs defaultValue="orders" value={dockTab} onValueChange={(val) => onDockTabChange(val as any)}>
            <CardHeader className="flex flex-row items-center justify-between border-b border-[var(--bk-border-soft)] bg-[var(--bk-surface-ghost)] px-8 pb-3 pt-6">
              <div className="flex items-center gap-3">
                <ShieldCheck className="size-5 text-[var(--bk-text-primary)]" />
                <CardTitle className="text-xl font-black text-[var(--bk-text-primary)]">运行监控与人工干预</CardTitle>
              </div>
              <TabsList variant="bento" className="flex h-10 gap-1 rounded-2xl border border-[color-mix(in_srgb,var(--bk-border)_30%,transparent)] bg-[color-mix(in_srgb,var(--bk-canvas-strong)_50%,transparent)] p-1 shadow-inner">
                <TabsTrigger value="orders" className="rounded-xl px-4 text-[10px] font-black uppercase data-[state=active]:bg-[var(--bk-surface)] data-[state=active]:text-[var(--bk-text-primary)] data-[state=active]:shadow-sm">订单</TabsTrigger>
                <TabsTrigger value="positions" className="rounded-xl px-4 text-[10px] font-black uppercase data-[state=active]:bg-[var(--bk-surface)] data-[state=active]:text-[var(--bk-text-primary)] data-[state=active]:shadow-sm">持仓</TabsTrigger>
                <TabsTrigger value="fills" className="rounded-xl px-4 text-[10px] font-black uppercase data-[state=active]:bg-[var(--bk-surface)] data-[state=active]:text-[var(--bk-text-primary)] data-[state=active]:shadow-sm">成交</TabsTrigger>
                <TabsTrigger value="alerts" className="rounded-xl px-4 text-[10px] font-black uppercase data-[state=active]:bg-[var(--bk-surface)] data-[state=active]:text-[var(--bk-text-primary)] data-[state=active]:shadow-sm">告警</TabsTrigger>
              </TabsList>
            </CardHeader>
            <CardContent className="p-0">
              <TabsContent value={dockTab} className="mt-0 animate-in slide-in-from-bottom-1 duration-300">
                <div className="min-h-[280px]">
                   {dockContent}
                </div>
              </TabsContent>
            </CardContent>
          </Tabs>
        </Card>
      </div>
    </div>
  );
}
