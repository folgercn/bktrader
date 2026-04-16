import React, { useMemo, useState } from 'react';
import { useUIStore } from '../store/useUIStore';
import { useTradingStore } from '../store/useTradingStore';
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
  const monitorCandles = useTradingStore(s => s.monitorCandles);
  const summaries = useTradingStore(s => s.summaries);
  const runtimePolicy = useTradingStore(s => s.runtimePolicy);
  const monitorHealth = useTradingStore(s => s.monitorHealth);
  const accounts = useTradingStore(s => s.accounts);
  const strategySignalBindingMap = useTradingStore(s => s.strategySignalBindingMap);
  const monitorResolution = useUIStore(s => s.monitorResolution);
  const setMonitorResolution = useUIStore(s => s.setMonitorResolution);
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
      <Card className="border-[#d8cfba] bg-[var(--panel)] shadow-2xl rounded-[32px] overflow-hidden border-2">
         <CardHeader className="bg-white/40 border-b border-[#d8cfba]/50 px-8 py-5">
           <div className="flex items-center justify-between">
             <div className="flex items-center gap-3">
               <div className="p-2 bg-[#ebe5d5] rounded-xl shadow-inner">
                 <LineChart className="size-5 text-[#1f2328]" />
               </div>
               <div>
                  <CardTitle className="text-xl font-black text-[#1f2328]">主监控台</CardTitle>
                  <p className="text-[10px] text-[#687177] font-bold uppercase tracking-widest mt-0.5">Runtime K-Line & Execution Flow</p>
               </div>
             </div>
             <div className="flex items-center gap-4">
                <div className="flex items-center gap-1.5 px-3 py-1 bg-[#ebe5d5]/50 border border-[#d8cfba] rounded-2xl shadow-sm">
                  {[
                    { label: '5m', value: '5' },
                    { label: '15m', value: '15' },
                    { label: '1h', value: '60' },
                    { label: '4h', value: '240' },
                    { label: '1d', value: '1D' },
                  ].map((tf) => (
                    <button
                      key={tf.label}
                      onClick={() => setMonitorResolution(tf.value)}
                      className={`px-2.5 py-1 rounded-lg text-[10px] uppercase font-black transition-all ${
                        monitorResolution === tf.value
                          ? 'bg-[#1f2328] text-white shadow-md scale-105'
                          : 'text-[#687177] hover:bg-white/60 hover:text-[#1f2328]'
                      }`}
                    >
                      {tf.label}
                    </button>
                  ))}
                </div>
                <Badge variant="outline" className="h-7 px-3 border-[#d8cfba] font-mono text-[10px] bg-white text-[#1f2328]">
                  {monitorMode}
                </Badge>
             </div>
           </div>
         </CardHeader>
         <CardContent className="p-0">
            <div className="chart-shell h-[360px] bg-[#fffbf2]/40 relative overflow-hidden">
                {monitorBars.length > 0 ? (
                  <SignalMonitorChart candles={monitorBars} markers={monitorMarkers} />
                ) : (
                  <div className="absolute inset-0 flex flex-col items-center justify-center space-y-3 opacity-30">
                    <Activity className="size-16 animate-pulse" />
                    <span className="text-sm font-bold italic">等待实盘数据输入...</span>
                  </div>
                )}
            </div>
            
            <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-8 gap-px bg-[#d8cfba]/40 border-t border-[#d8cfba]/50">
                {[
                  { label: "模式", value: monitorMode, icon: Zap },
                  { label: "净值", value: formatMoney(monitorSummary?.netEquity), color: 'text-[#1f2328]' },
                  { label: "盈亏", value: formatSigned(monitorSummary?.unrealizedPnl ?? 0), color: (getNumber(monitorSummary?.unrealizedPnl) ?? 0) >= 0 ? 'text-[#0e6d60]' : 'text-rose-600' },
                  { label: "方向", value: String(monitorExecutionSummary.position?.side ?? "FLAT"), color: 'font-black' },
                  { label: "数量", value: formatMaybeNumber(monitorExecutionSummary.position?.quantity) },
                  { label: "标记价", value: formatMaybeNumber(monitorExecutionSummary.position?.markPrice) },
                  { label: "当前价", value: formatMaybeNumber(monitorMarket.tradePrice), color: 'text-[#0e6d60]' },
                  { label: "SMA5", value: formatMaybeNumber(monitorSignalState.sma5) },
                ].map((item) => (
                  <div key={item.label} className="bg-white/40 p-4 flex flex-col items-center justify-center transition-colors hover:bg-white/60">
                    <span className="text-[9px] text-[#687177] font-black uppercase tracking-tighter mb-1">{item.label}</span>
                    <strong className={`text-[13px] tracking-tight ${item.color || 'text-[#1f2328]'}`}>{item.value}</strong>
                  </div>
                ))}
            </div>
         </CardContent>
      </Card>

      <div className="grid grid-cols-1 lg:grid-cols-12 gap-8">
        {/* 2. 交互与干预区 */}
        <Card className="lg:col-span-8 border-[#d8cfba] bg-[var(--panel)] shadow-xl rounded-[32px] overflow-hidden border-2">
          <CardHeader className="bg-white/30 px-8 py-5 border-b border-[#d8cfba]/50">
            <div className="flex items-center gap-3">
              <ShieldCheck className="size-5 text-[#1f2328]" />
              <CardTitle className="text-lg font-black text-[#1f2328]">运行监控与人工干预</CardTitle>
            </div>
          </CardHeader>
          <CardContent className="p-8">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
              {/* 左侧：优先会话详情 */}
              {highlightedLiveSession ? (
                <div className="bg-[#fff8ea] rounded-[24px] p-6 border-2 border-[#d8cfba] shadow-lg relative overflow-hidden group flex flex-col h-full">
                  

                  <div className="flex items-start justify-between mb-8">
                    <div className="space-y-2">
                      <div className="flex items-center gap-2">
                        <span className="text-[10px] text-[#687177] font-black uppercase tracking-widest opacity-70">Primary Session</span>
                        <Badge className="bg-[#1f2328] text-white border-0 font-black text-[9px] h-4 px-1.5 rounded-md">
                          {highlightedLiveSession.health.status}
                        </Badge>
                      </div>
                      <h4 className="text-xl font-black text-[#1f2328] tracking-tight">活跃监控焦点会话</h4>
                    </div>
                  </div>
                  
                  <div className="flex flex-wrap gap-2 text-[10px] mb-6">
                    {/* 会话 ID 磁贴选择器 */}
                    <Popover>
                      <PopoverTrigger>
                        <button 
                          className={`flex items-center gap-2 w-fit max-w-[240px] bg-white px-3 py-1.5 rounded-lg border border-[#d8cfba] font-mono font-bold shadow-sm transition-all active:scale-95 ${allSessionItems.length > 1 ? 'hover:bg-[#f0ece0] cursor-pointer' : 'cursor-default'}`}
                          disabled={allSessionItems.length <= 1}
                        >
                          <span className="truncate">{highlightedLiveSession.session.id.length > 20 ? `${highlightedLiveSession.session.id.slice(0, 14)}...${highlightedLiveSession.session.id.slice(-6)}` : highlightedLiveSession.session.id}</span>
                          {allSessionItems.length > 1 && <ChevronDown className="size-3 text-[#687177] opacity-60 flex-shrink-0" />}
                        </button>
                      </PopoverTrigger>
                      <PopoverContent align="start" className="w-[320px] p-2 bg-[#fffbf2] border-2 border-[#d8cfba] shadow-xl rounded-[20px] isolate z-[60]">
                         <div className="space-y-1.5">
                            <div className="px-2 py-1.5 mb-1 border-b border-[#d8cfba]/40">
                               <span className="text-[9px] font-black text-[#687177] uppercase tracking-widest">Switch Active Session</span>
                            </div>
                            {allSessionItems.map((item) => (
                              <div 
                                key={item.session.id} 
                                onClick={() => {
                                  handleSelectSession(item.session.id);
                                }}
                                className={`flex items-center justify-between p-3 rounded-xl border transition-all cursor-pointer group animate-in fade-in duration-200 ${
                                  item.isHighlighted 
                                    ? 'bg-[#d9eee8] border-[#0e6d60] ring-2 ring-[#0e6d60]/10' 
                                    : 'bg-white/60 border-[#d8cfba]/40 hover:bg-white hover:border-[#0e6d60]/50'
                                }`}
                              >
                                 <div className="flex items-center gap-3">
                                    <div className={`size-2 rounded-full ${item.health.status === 'ready' ? 'bg-[#0e6d60]' : 'bg-rose-500'} ${item.isHighlighted ? 'ring-4 ring-[#0e6d60]/20' : 'animate-pulse'}`} />
                                    <div className="flex flex-col">
                                       <span className={`text-[10px] font-black ${item.isHighlighted ? 'text-[#0e6d60]' : 'text-[#1f2328]'}`}>{item.session.id.length > 20 ? `${item.session.id.slice(0, 14)}...${item.session.id.slice(-6)}` : item.session.id}</span>
                                       <span className={`text-[8px] font-mono ${item.isHighlighted ? 'text-[#064e44]/70' : 'text-[#687177]'}`}>{String(item.session.state?.symbol ?? "--")} · {String(item.session.state?.signalTimeframe ?? "--")}</span>
                                    </div>
                                 </div>
                                 <div className="text-right">
                                    <span className={`text-[10px] font-black block tabular-nums ${
                                      (getNumber(item.summary?.unrealizedPnl) ?? 0) >= 0 ? 'text-[#0e6d60]' : 'text-rose-600'
                                    }`}>
                                       {formatSigned(item.summary?.unrealizedPnl ?? 0)}
                                    </span>
                                    <span className={`text-[8px] uppercase font-bold opacity-50 block mt-0.5 ${item.isHighlighted ? 'text-[#064e44]/60' : 'text-[#687177]'}`}>{String(item.execution.position?.side ?? "FLAT")}</span>
                                 </div>
                              </div>
                            ))}
                         </div>
                      </PopoverContent>
                    </Popover>

                    <span className="bg-white px-2 py-1 rounded-lg border border-[#d8cfba] font-mono shadow-sm">{highlightedLiveSession.session.accountId}</span>
                    <Badge variant="secondary" className="bg-[#d9eee8] text-[#0e6d60] border-[#0e6d60]/20 font-black">
                      {String(highlightedLiveSession.session.state?.signalTimeframe ?? "--")}
                    </Badge>
                  </div>

                  <div className="space-y-4 bg-white/60 p-5 rounded-2xl border border-[#d8cfba]/50 shadow-inner">
                    <p className="text-[12px] text-[#1f2328] leading-relaxed font-medium">
                      <span className="text-[#687177] font-black mr-2 opacity-50">HEALTH_LOG:</span> 
                      {highlightedLiveSession.health.detail}
                    </p>
                    <div className="grid grid-cols-2 gap-4 pt-2">
                      <div className="space-y-1">
                        <span className="text-[9px] text-[#687177] font-black uppercase">执行统计</span>
                        <strong className="text-[11px] block font-black">Orders {highlightedLiveSession.execution.orderCount} · Fills {highlightedLiveSession.execution.fillCount}</strong>
                      </div>
                      <div className="space-y-1">
                        <span className="text-[9px] text-[#687177] font-black uppercase">持仓方向</span>
                        <strong className="text-[11px] block font-black text-[#0e6d60]">{String(monitorExecutionSummary.position?.side ?? "FLAT")}</strong>
                      </div>
                    </div>
                  </div>

                  {/* 新增：从右侧搬迁过来的辅助信息，用于平衡高度 */}
                  <div className="mt-auto grid grid-cols-2 gap-4 pt-8 border-t-2 border-[#d8cfba]/30">
                      <div className="bg-white/60 p-6 rounded-[28px] border-2 border-[#d8cfba]/60 shadow-sm flex flex-col justify-between hover:bg-white transition-all group">
                         <div className="flex items-center gap-2 mb-4">
                            <div className="p-1.5 bg-[#ebe5d5] rounded-lg">
                               <ShieldAlert className="size-4 text-[#1f2328]" />
                            </div>
                            <span className="text-[11px] text-[#687177] font-black uppercase tracking-widest">状态恢复</span>
                         </div>
                         <div className="flex items-center justify-between text-[11px] font-black">
                            <div className="flex flex-col space-y-1">
                              <span className="text-rose-600">SL: {String((monitorSession?.state as any)?.recoveredStopOrderCount ?? "0")}</span>
                              <span className="text-[#0e6d60]">PT: {String((monitorSession?.state as any)?.recoveredTakeProfitOrderCount ?? "0")}</span>
                            </div>
                            <div className="text-[9px] text-[#687177] font-black opacity-40">RECOVERY</div>
                         </div>
                      </div>

                      <div className="bg-white/60 p-6 rounded-[28px] border-2 border-[#d8cfba]/60 shadow-sm flex flex-col justify-between hover:bg-white transition-all group">
                         <div className="flex items-center gap-2 mb-4">
                            <div className="p-1.5 bg-[#fff8ea] rounded-lg border border-[#d8cfba]/40">
                               <FileText className="size-4 text-[#1f2328]" />
                            </div>
                            <span className="text-[11px] text-[#687177] font-black uppercase tracking-widest">执行备注</span>
                         </div>
                         <p className="text-[11px] font-bold text-[#1f2328] leading-tight line-clamp-3">
                           {String(monitorSignalBarDecision.reason ?? "暂无执行信号排队或阻断说明")}
                         </p>
                      </div>
                  </div>

 
                </div>
              ) : (
                <div className="p-20 text-center text-[#687177] text-sm font-bold italic bg-[#fff8ea]/50 rounded-[24px] border-2 border-dashed border-[#d8cfba] flex flex-col items-center justify-center space-y-4">
                  <div className="p-4 bg-white/40 rounded-full">
                    <Activity className="size-8 opacity-20" />
                  </div>
                  <span>当前没有活跃实盘会话</span>
                </div>
              )}

              {/* 右侧：折叠详情 */}
              <div className="space-y-6">
                <div className="flex items-center justify-between border-b-2 border-[#d8cfba] pb-3">
                  <h4 className="text-sm font-black text-[#1f2328] uppercase tracking-wider">监控遥测明细</h4>
                  <Badge variant="outline" className={`text-[10px] font-black ${monitorSession ? 'bg-[#d9eee8] text-[#0e6d60] border-[#0e6d60]/40' : 'bg-zinc-100 text-zinc-400 border-zinc-200'}`}>
                    {monitorSession ? "CONNECTED" : "IDLE"}
                  </Badge>
                </div>
                
                {monitorSession ? (
                  <div className="space-y-6">
                    <div className="grid grid-cols-2 gap-3">
                      {monitorSummaryItems.map((item) => (
                        <div key={item.label} className="bg-white/60 p-3 rounded-2xl border border-[#d8cfba]/60 shadow-sm transition-all hover:shadow-md hover:bg-white/80">
                          <span className="text-[9px] text-[#687177] font-black uppercase mb-1 block tracking-tighter opacity-70">{item.label}</span>
                          <strong className="text-[11px] text-[#1f2328] font-bold block truncate">{item.value}</strong>
                        </div>
                      ))}
                    </div>
                    <div className="grid grid-cols-2 gap-4">
                      {/* 行 1: 核心行情 - 全宽 */}
                      <div className="col-span-2 bg-gradient-to-br from-white to-[#fff8ea] p-6 rounded-[28px] border-2 border-[#d8cfba] shadow-sm hover:shadow-md transition-all group">
                        <div className="flex items-center justify-between mb-4">
                          <div className="flex items-center gap-2">
                             <div className="p-1.5 bg-[#ebe5d5] rounded-lg">
                               <CandlestickChart className="size-4 text-[#1f2328]" />
                             </div>
                             <span className="text-[11px] text-[#687177] font-black uppercase tracking-widest">行情核心分析</span>
                          </div>
                          <Badge variant="outline" className="text-[10px] font-mono border-[#d8cfba] text-[#0e6d60] bg-white">LATEST</Badge>
                        </div>
                        <div className="flex items-end justify-between">
                          <div className="space-y-2">
                            <span className="text-[10px] text-[#687177] font-bold block opacity-60">PRICE / SPREAD</span>
                            <strong className="text-3xl font-black text-[#1f2328] tracking-tighter tabular-nums leading-none">
                              {formatMaybeNumber(monitorMarket.tradePrice)}
                            </strong>
                          </div>
                          <div className="text-right">
                             <span className="text-[16px] font-mono font-bold text-[#0e6d60] block leading-none antialiased">
                               {formatMaybeNumber(monitorMarket.bestBid)} / {formatMaybeNumber(monitorMarket.bestAsk)}
                             </span>
                             <span className="text-[10px] text-[#687177] font-black uppercase mt-1.5 block">Depth liquidity</span>
                          </div>
                        </div>
                      </div>

                      {/* 行 2: 策略状态 - 双列并列 */}
                      <div className="bg-white/60 p-6 rounded-[28px] border-2 border-[#d8cfba]/60 shadow-sm hover:bg-white transition-all group">
                         <div className="flex items-center gap-2 mb-4">
                            <div className="p-1.5 bg-[#d9eee8] rounded-lg">
                               <Activity className="size-4 text-[#0e6d60]" />
                            </div>
                            <span className="text-[11px] text-[#687177] font-black uppercase tracking-widest">策略持仓</span>
                         </div>
                         <div className="space-y-2">
                            <strong className={`text-xl font-black block leading-tight ${String(monitorExecutionSummary.position?.side).includes('LONG') ? 'text-[#0e6d60]' : String(monitorExecutionSummary.position?.side).includes('SHORT') ? 'text-rose-600' : 'text-[#1f2328]'}`}>
                              {String(monitorExecutionSummary.position?.side ?? "FLAT")}
                            </strong>
                            <span className="text-[11px] font-mono text-[#687177] font-bold block">
                              {formatMaybeNumber(monitorExecutionSummary.position?.quantity)} @ {formatMaybeNumber(monitorExecutionSummary.position?.entryPrice)}
                            </span>
                         </div>
                      </div>

                      <div className="bg-white/60 p-6 rounded-[28px] border-2 border-[#d8cfba]/60 shadow-sm hover:bg-white transition-all group">
                         <div className="flex items-center gap-2 mb-4">
                            <div className="p-1.5 bg-[#ebe5d5] rounded-lg">
                               <Compass className="size-4 text-[#1f2328]" />
                            </div>
                            <span className="text-[11px] text-[#687177] font-black uppercase tracking-widest">信号分析</span>
                         </div>
                         <div className="space-y-2">
                            <strong className="text-xl font-black text-[#1f2328] block leading-tight tabular-nums">
                              {formatMaybeNumber(monitorSignalBarDecision.sma5)}
                            </strong>
                            <span className="text-[11px] font-mono text-[#687177] font-bold block">
                              PERIOD: {String(monitorSignalBarDecision.timeframe ?? "--")}
                            </span>
                         </div>
                      </div>
                    </div>
                  </div>
                ) : (
                  <div className="h-64 flex flex-col items-center justify-center space-y-3 bg-white/20 rounded-[24px] border-2 border-dashed border-[#d8cfba]/50 opacity-40">
                    <Layout className="size-8" />
                    <p className="text-xs font-bold italic">请在 Dock 中选中活跃会话</p>
                  </div>
                )}
              </div>
            </div>

            {/* 底部：常驻终端时间线 - 奶油风格 */}
            <div className="mt-8 pt-8 border-t-2 border-[#d8cfba]/50">
               <div className="flex items-center justify-between mb-4">
                 <div className="flex items-center gap-2">
                   <div className="size-2 rounded-full bg-[#0e6d60] animate-pulse" />
                   <h5 className="text-[11px] font-black text-[#1f2328] uppercase tracking-widest">Execution Timeline Terminal</h5>
                 </div>
                 <Badge variant="outline" className="text-[9px] font-mono border-[#d8cfba] text-[#687177] bg-[#fffbf2]">
                   AUTO_SCROLL ENABLED
                 </Badge>
               </div>
               <div className="h-[280px] overflow-y-auto p-5 bg-[#fffcfe] rounded-[24px] border-2 border-[#d8cfba] shadow-inner custom-scrollbar">
                  {timelineLogs.length > 0 ? timelineLogs.map((line: string, idx: number) => (
                    <div key={idx} className="text-[10px] font-mono text-[#1f2328] mb-1.5 leading-normal border-l-2 border-[#ebe5d5] pl-3 py-0.5 hover:bg-[#fff8ea] hover:border-[#0e6d60] transition-all">
                      <span className="text-[#687177] mr-3 opacity-40 font-bold tabular-nums">[{idx.toString().padStart(2, '0')}]</span>
                      <span className="opacity-90">{line}</span>
                    </div>
                  )) : (
                    <div className="h-full flex items-center justify-center text-[10px] font-mono text-[#687177] italic opacity-40">
                      SYSTEM: Waiting for runtime events to populate timeline...
                    </div>
                  )}
               </div>
            </div>
          </CardContent>
        </Card>

        {/* 3. 待处理订单 - 订单同步表格 */}
        <Card className="lg:col-span-4 border-[#d8cfba] bg-[var(--panel)] shadow-xl rounded-[32px] overflow-hidden border-2 flex flex-col">
          <CardHeader className="bg-white/30 border-b border-[#d8cfba]/50 px-6 py-5">
            <div className="flex items-center gap-3">
              <ArrowRightLeft className="size-5 text-[#1f2328]" />
              <CardTitle className="text-lg font-black text-[#1f2328]">待同步订单</CardTitle>
            </div>
          </CardHeader>
          <CardContent className="p-6 flex-1 overflow-hidden">
             <div className="h-full rounded-[24px] border-2 border-[#d8cfba] bg-white/50 overflow-hidden flex flex-col shadow-inner">
               <Table>
                 <TableHeader className="bg-[#ebe5d5]/60 border-b-2 border-[#d8cfba]">
                   <TableRow className="hover:bg-transparent">
                     <TableHead className="h-10 text-[10px] font-black text-[#687177] uppercase px-5">Symbol/Order</TableHead>
                     <TableHead className="h-10 text-right text-[10px] font-black text-[#687177] uppercase px-5">Action</TableHead>
                   </TableRow>
                 </TableHeader>
                 <TableBody className="overflow-y-auto">
                   {syncableLiveOrders.length > 0 ? (
                     syncableLiveOrders.map((order) => (
                       <TableRow key={order.id} className="border-b border-[#d8cfba]/30 hover:bg-[#fff8ea]/80 transition-colors">
                         <TableCell className="px-5 py-4">
                            <div className="flex flex-col">
                              <span className="font-black text-[#0e6d60] text-xs">{order.symbol}</span>
                              <span className="text-[9px] font-mono text-[#687177] mt-1">{shrink(order.id)} · {order.side}</span>
                            </div>
                         </TableCell>
                         <TableCell className="px-5 text-right">
                            <Button 
                              size="sm" 
                              variant="outline" 
                              className="h-8 px-4 rounded-xl border-2 border-[#d8cfba] bg-white hover:bg-[#ebe5d5] text-[#1f2328] font-black text-[10px] shadow-sm active:scale-95"
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
                       <TableCell colSpan={2} className="h-40 text-center text-xs text-[#687177] italic font-medium">
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
        <Card className="border-[#d8cfba] bg-[var(--panel)] shadow-xl rounded-[32px] overflow-hidden border-2">
          <CardHeader className="bg-white/30 flex flex-row items-center justify-between pb-4 px-8 pt-6 border-b border-[#d8cfba]/50">
            <div className="flex items-center gap-3">
              <HeartPulse className="size-5 text-[#1f2328]" />
              <CardTitle className="text-xl font-black text-[#1f2328]">平台健康总览</CardTitle>
            </div>
            <Badge className="bg-[#d9eee8] text-[#0e6d60] border-2 border-[#0e6d60]/20 font-black px-3 py-1 text-[11px] rounded-xl shadow-sm">
               {technicalStatusLabel(monitorHealth?.status ?? "--")}
            </Badge>
          </CardHeader>
          <CardContent className="px-8 py-8">
            {monitorHealth ? (
              <div className="grid grid-cols-3 md:grid-cols-6 gap-4">
                {[
                  { label: "Alerts", value: monitorHealth.alertCounts.total, icon: 'total' },
                  { label: "Critical", value: monitorHealth.alertCounts.critical, color: 'text-rose-600', bg: 'bg-rose-50' },
                  { label: "Warning", value: monitorHealth.alertCounts.warning, color: 'text-amber-700', bg: 'bg-amber-50' },
                  { label: "Quiet", value: runtimePolicyValueLabel(platformRuntimePolicy?.runtimeQuietSeconds) },
                  { label: "Eval", value: runtimePolicyValueLabel(platformRuntimePolicy?.strategyEvaluationQuietSeconds) },
                  { label: "Sync", value: runtimePolicyValueLabel(platformRuntimePolicy?.liveAccountSyncFreshnessSeconds) },
                ].map((item) => (
                  <div key={item.label} className={`p-4 rounded-[20px] border-2 border-[#d8cfba] shadow-sm transition-all hover:scale-105 ${item.bg || 'bg-white'}`}>
                    <span className="text-[10px] text-[#687177] uppercase font-black block mb-2 opacity-60 tracking-tighter">{item.label}</span>
                    <strong className={`text-xl tracking-tighter block font-black ${item.color || 'text-[#1f2328]'}`}>
                      {String(item.value ?? 0)}
                    </strong>
                  </div>
                ))}
              </div>
            ) : (
              <div className="p-12 text-center text-[#687177] text-sm font-bold italic bg-[#fff8ea]/40 rounded-[24px] border-2 border-dashed border-[#d8cfba]">
                健康诊断模块正在预热中...
              </div>
            )}
          </CardContent>
        </Card>

        {/* 5. 记录中心 Tabs */}
        <Card className="border-[#d8cfba] bg-[var(--panel)] shadow-xl rounded-[32px] overflow-hidden border-2">
          <CardContent className="pt-8 px-8">
            <Tabs defaultValue="orders" value={dockTab} onValueChange={(val) => onDockTabChange(val as any)}>
              <TabsList className="bg-[#ebe5d5] p-1.5 mb-6 h-12 rounded-[20px] shadow-inner grid grid-cols-4 gap-2">
                <TabsTrigger value="orders" className="rounded-xl text-[11px] font-black uppercase data-[state=active]:bg-white data-[state=active]:text-[#1f2328] data-[state=active]:shadow-md">订单</TabsTrigger>
                <TabsTrigger value="positions" className="rounded-xl text-[11px] font-black uppercase data-[state=active]:bg-white data-[state=active]:text-[#1f2328] data-[state=active]:shadow-md">持仓</TabsTrigger>
                <TabsTrigger value="fills" className="rounded-xl text-[11px] font-black uppercase data-[state=active]:bg-white data-[state=active]:text-[#1f2328] data-[state=active]:shadow-md">成交</TabsTrigger>
                <TabsTrigger value="alerts" className="rounded-xl text-[11px] font-black uppercase data-[state=active]:bg-white data-[state=active]:text-[#1f2328] data-[state=active]:shadow-md">告警</TabsTrigger>
              </TabsList>
              <TabsContent value={dockTab} className="mt-0 animate-in slide-in-from-bottom-2 duration-300">
                <div className="rounded-[24px] border-2 border-[#d8cfba] bg-[#fffbf2]/60 overflow-hidden shadow-2xl min-h-[300px]">
                   {dockContent}
                </div>
              </TabsContent>
            </Tabs>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
