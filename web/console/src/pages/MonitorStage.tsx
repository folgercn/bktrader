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
import { Activity, Layout, ShieldCheck, Zap, BarChart3, Clock, ArrowRightLeft, HeartPulse, LineChart } from 'lucide-react';

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

  const monitorSummaryItems = monitorSession ? [
    { label: "就绪预检", value: `${monitorRuntimeReadiness.status} · ${monitorRuntimeReadiness.reason}` },
    { label: "信号意图", value: `${String(monitorIntent.action ?? "无")} · ${String(monitorIntent.side ?? "--")}` },
    { label: "指令分发", value: `${String(monitorSession.state?.dispatchMode ?? "--")} · 冷却 ${String(monitorSession.state?.dispatchCooldownSeconds ?? "--")}s` },
    { label: "执行汇总", value: `订单 ${monitorExecutionSummary.orderCount} · 成交 ${monitorExecutionSummary.fillCount}` },
  ] : [];

  const monitorSections = monitorSession ? [
    {
      title: "运行与行情",
      items: [
        { label: "行情数据", value: `${formatMaybeNumber(monitorMarket.tradePrice)} · ${formatMaybeNumber(monitorMarket.bestBid)} / ${formatMaybeNumber(monitorMarket.bestAsk)}` },
        { label: "时间线", value: buildTimelineNotes(monitorTimeline).slice(0, 2).join(" · ") || "--" },
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
        { label: "恢复统计", value: `止损 ${String(monitorSession.state?.recoveredStopOrderCount ?? "--")} · 止盈 ${String(monitorSession.state?.recoveredTakeProfitOrderCount ?? "--")}` },
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
                  { label: "盈亏", value: formatSigned(monitorSummary?.unrealizedPnl), color: getNumber(monitorSummary?.unrealizedPnl) >= 0 ? 'text-[#0e6d60]' : 'text-rose-600' },
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
                <div className="bg-[#fff8ea] rounded-[24px] p-6 border-2 border-[#d8cfba] shadow-lg relative overflow-hidden group">
                  <div className="absolute top-0 right-0 p-4 opacity-10 group-hover:opacity-20 transition-opacity">
                    <BarChart3 className="size-20" />
                  </div>
                  
                  <div className="flex items-center justify-between mb-6">
                    <div>
                      <span className="text-[10px] text-[#687177] font-black uppercase tracking-widest">Primary Session</span>
                      <h4 className="text-lg font-black text-[#1f2328] mt-1">当前由于焦点会话</h4>
                    </div>
                    <Badge className="bg-[#1f2328] text-white border-0 font-black text-[10px] px-3 py-1 rounded-lg">
                      {highlightedLiveSession.health.status}
                    </Badge>
                  </div>
                  
                  <div className="flex flex-wrap gap-2 text-[10px] mb-6">
                    <span className="bg-white px-2 py-1 rounded-lg border border-[#d8cfba] font-mono font-bold shadow-sm">{shrink(highlightedLiveSession.session.id)}</span>
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

                  <div className="mt-8 flex flex-wrap gap-2">
                    {monitorFlow.map((step) => (
                      <Badge 
                        key={step.key}
                        variant="secondary"
                        className="text-[9px] bg-white text-[#687177] border-[#d8cfba] font-black shadow-sm"
                      >
                        {step.label}
                      </Badge>
                    ))}
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
                  <div className="space-y-4">
                    <div className="grid grid-cols-2 gap-3">
                      {monitorSummaryItems.map((item) => (
                        <div key={item.label} className="bg-white/60 p-3 rounded-2xl border border-[#d8cfba]/60 shadow-sm transition-all hover:shadow-md hover:bg-white/80">
                          <span className="text-[9px] text-[#687177] font-black uppercase mb-1 block tracking-tighter opacity-70">{item.label}</span>
                          <strong className="text-[11px] text-[#1f2328] font-bold block truncate">{item.value}</strong>
                        </div>
                      ))}
                    </div>
                    
                    <Accordion type="multiple" className="w-full space-y-2">
                      {monitorSections.map((section) => (
                        <AccordionItem key={section.title} value={section.title} className="border-2 border-[#d8cfba]/30 rounded-2xl px-4 bg-white/30">
                          <AccordionTrigger className="hover:no-underline py-4 text-[12px] font-black text-[#1f2328] uppercase tracking-wide">
                            {section.title}
                          </AccordionTrigger>
                          <AccordionContent className="pb-4">
                            <div className="space-y-2">
                              {section.items.map((item) => (
                                <div key={item.label} className="flex justify-between items-center text-[11px] p-2.5 rounded-xl bg-[#fff8ea]/60 border border-[#d8cfba]/40 hover:bg-white transition-colors">
                                  <span className="text-[#687177] font-medium">{item.label}</span>
                                  <strong className="text-[#1f2328] font-mono text-[10px]">{item.value}</strong>
                                </div>
                              ))}
                            </div>
                          </AccordionContent>
                        </AccordionItem>
                      ))}
                    </Accordion>
                  </div>
                ) : (
                  <div className="h-64 flex flex-col items-center justify-center space-y-3 bg-white/20 rounded-[24px] border-2 border-dashed border-[#d8cfba]/50 opacity-40">
                    <Layout className="size-8" />
                    <p className="text-xs font-bold italic">请在 Dock 中选中活跃会话</p>
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
