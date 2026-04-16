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
import { Card, CardHeader, CardTitle, CardContent, CardDescription } from '../components/ui/card';
import { Badge } from '../components/ui/badge';
import { Table, TableHeader, TableBody, TableHead, TableRow, TableCell } from '../components/ui/table';
import { Accordion, AccordionItem, AccordionTrigger, AccordionContent } from '../components/ui/accordion';
import { Separator } from '../components/ui/separator';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '../components/ui/tabs';
import { Button } from '../components/ui/button';

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
      {/* 1. 主监控区 - K 线与核心状态 */}
      <Card className="border-white/5 bg-zinc-900/50 backdrop-blur-sm overflow-hidden">
        <CardHeader className="flex flex-row items-center justify-between pb-4 space-y-0">
          <div>
            <CardDescription className="text-emerald-500/80 font-mono text-[10px] uppercase tracking-wider">Market Overview</CardDescription>
            <CardTitle className="text-xl font-bold tracking-tight text-zinc-100">交易所大周期监控</CardTitle>
          </div>
          <div className="flex items-center gap-2">
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
        </CardHeader>
        <CardContent>
          <div className="chart-shell h-[320px] rounded-lg border border-white/5 bg-zinc-950/40 relative">
            {monitorBars.length > 0 ? (
              <SignalMonitorChart candles={monitorBars} markers={monitorMarkers} />
            ) : (
              <div className="absolute inset-0 flex items-center justify-center text-zinc-600 text-xs italic">
                当前运行会话还没有交易所大周期 K 线缓存
              </div>
            )}
          </div>
          <div className="grid grid-cols-4 md:grid-cols-8 gap-4 mt-6 p-4 rounded-xl bg-zinc-950/30 border border-white/5">
            {[
              { label: "会话模式", value: monitorMode },
              { label: "账户净值", value: formatMoney(monitorSummary?.netEquity) },
              { label: "未实现盈亏", value: formatSigned(monitorSummary?.unrealizedPnl) },
              { label: "方向", value: String(monitorExecutionSummary.position?.side ?? "FLAT") },
              { label: "数量", value: formatMaybeNumber(monitorExecutionSummary.position?.quantity) },
              { label: "标记价格", value: formatMaybeNumber(monitorExecutionSummary.position?.markPrice) },
              { label: "盘口", value: `${formatMaybeNumber(monitorMarket.bestBid)} / ${formatMaybeNumber(monitorMarket.bestAsk)}` },
              { label: "SMA5 / ATR14", value: `${formatMaybeNumber(monitorSignalState.sma5)} / ${formatMaybeNumber(monitorSignalState.atr14)}` },
            ].map((item) => (
              <div key={item.label} className="flex flex-col gap-1">
                <span className="text-[10px] text-zinc-500 uppercase font-mono">{item.label}</span>
                <strong className="text-xs text-zinc-300 font-semibold">{item.value}</strong>
              </div>
            ))}
          </div>
        </CardContent>
      </Card>

      {/* 2. 运行监控与人工干预 */}
      <Card className="border-white/5 bg-zinc-900/50 backdrop-blur-sm overflow-hidden">
        <CardHeader className="pb-3 border-b border-white/5">
          <CardDescription className="text-emerald-500/80 font-mono text-[10px] uppercase tracking-wider">Operation & Intervention</CardDescription>
          <CardTitle className="text-xl font-bold tracking-tight text-zinc-100">运行监控与人工干预</CardTitle>
        </CardHeader>
        <CardContent className="pt-6 space-y-6">
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
            {/* 左侧：核心会话卡片 */}
            {highlightedLiveSession ? (
              <Card className="bg-zinc-950/40 border-white/5 shadow-2xl">
                <CardHeader className="flex flex-row items-center justify-between pb-2 space-y-0">
                  <div>
                    <CardDescription className="text-zinc-500 font-mono text-[9px] uppercase">Primary Session</CardDescription>
                    <CardTitle className="text-sm font-semibold text-zinc-200">优先处理会话</CardTitle>
                  </div>
                  <Badge 
                    variant={highlightedLiveSession.health.status === "error" ? "destructive" : "secondary"}
                    className="font-mono text-[10px]"
                  >
                    {highlightedLiveSession.health.status}
                  </Badge>
                </CardHeader>
                <CardContent>
                  <div className="flex flex-wrap gap-2 text-[10px] text-zinc-400 font-mono mb-4">
                    <span className="bg-white/5 px-1.5 py-0.5 rounded">{shrink(highlightedLiveSession.session.id)}</span>
                    <span className="bg-white/5 px-1.5 py-0.5 rounded">{highlightedLiveSession.session.accountId}</span>
                    <Badge variant="outline" className="h-4 px-1.5 text-[9px] border-emerald-500/30 text-emerald-400">
                      {String(highlightedLiveSession.session.state?.signalTimeframe ?? "--")}
                    </Badge>
                  </div>
                  
                  <div className="space-y-4">
                    <p className="text-[11px] text-zinc-300 leading-relaxed bg-white/5 p-2 rounded">
                      <span className="text-zinc-500 font-medium">健康摘要:</span> {highlightedLiveSession.health.detail}
                    </p>
                    <div className="grid grid-cols-2 gap-3 text-[10px]">
                      <div className="p-2 rounded bg-white/5">
                        <span className="text-zinc-500 block mb-1">执行统计</span>
                        <strong className="text-zinc-300">订单 {highlightedLiveSession.execution.orderCount} · 成交 {highlightedLiveSession.execution.fillCount}</strong>
                      </div>
                      <div className="p-2 rounded bg-white/5">
                        <span className="text-zinc-500 block mb-1">仓位状态</span>
                        <strong className="text-zinc-300">{String(highlightedLiveSession.session.state?.positionRecoveryStatus ?? "FLAT")}</strong>
                      </div>
                    </div>
                  </div>

                  <div className="mt-6 pt-4 border-t border-white/5 flex flex-wrap gap-2">
                    {monitorFlow.map((step) => (
                      <div key={step.key} className="flex items-center gap-2 group">
                        <Badge 
                          variant={step.status === "blocked" ? "destructive" : step.status === "watch" ? "outline" : "secondary"}
                          className={`h-5 px-2 text-[9px] tracking-tight ${step.status === "watch" ? "text-zinc-400 border-zinc-700" : ""}`}
                        >
                          {step.label}
                        </Badge>
                        <span className="text-[9px] text-zinc-500 opacity-0 group-hover:opacity-100 transition-opacity truncate max-w-[80px]">{step.detail}</span>
                      </div>
                    ))}
                  </div>
                </CardContent>
              </Card>
            ) : (
              <div className="flex items-center justify-center border-2 border-dashed border-white/5 rounded-xl h-48 text-zinc-600 text-xs italic">
                当前没有活跃的优先实盘会话
              </div>
            )}

            {/* 右侧：监控明细折叠面板 */}
            <div className="space-y-4">
              <div className="flex items-center justify-between border-b border-white/5 pb-2">
                <h4 className="text-sm font-medium text-zinc-300">当前会话监控细节</h4>
                <Badge variant="outline" className="text-[10px] border-zinc-700 text-zinc-500">{monitorSession ? "Connected" : "Idle"}</Badge>
              </div>
              {monitorSession ? (
                <div className="space-y-4">
                  <div className="grid grid-cols-2 gap-2">
                    {monitorSummaryItems.map((item) => (
                      <div key={item.label} className="p-2 rounded-lg bg-zinc-800/30 border border-white/5">
                        <span className="text-[9px] text-zinc-500 uppercase font-mono block mb-0.5">{item.label}</span>
                        <strong className="text-[10px] text-zinc-200 truncate block">{item.value}</strong>
                      </div>
                    ))}
                  </div>
                  
                  <Accordion multiple className="w-full">
                    {monitorSections.map((section) => (
                      <AccordionItem key={section.title} value={section.title} className="border-white/5">
                        <AccordionTrigger className="hover:no-underline py-2 text-xs font-medium text-zinc-400">
                          {section.title}
                        </AccordionTrigger>
                        <AccordionContent>
                          <div className="grid grid-cols-1 gap-1.5 pt-1 pb-3">
                            {section.items.map((item) => (
                              <div key={`${section.title}-${item.label}`} className="flex justify-between items-center text-[10px] px-2 py-1.5 rounded bg-white/5">
                                <span className="text-zinc-500">{item.label}</span>
                                <strong className="text-zinc-300 font-mono">{item.value}</strong>
                              </div>
                            ))}
                          </div>
                        </AccordionContent>
                      </AccordionItem>
                    ))}
                  </Accordion>
                </div>
              ) : (
                <div className="flex flex-col items-center justify-center h-48 rounded-xl border border-white/5 bg-zinc-950/20 text-zinc-600">
                  <p className="text-xs">选中会话后显示监控明细</p>
                </div>
              )}
            </div>
          </div>
        </CardContent>
      </Card>

      {/* 3. 待同步订单 - 单独一栏 */}
      <Card className="border-white/5 bg-zinc-900/50 backdrop-blur-sm overflow-hidden">
        <CardHeader className="pb-3">
          <CardTitle className="text-sm font-medium text-zinc-300">待同步的实盘订单</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="rounded-md border border-white/5 bg-zinc-950/20 overflow-hidden">
            <Table>
              <TableHeader className="bg-white/5">
                <TableRow className="border-white/5 hover:bg-transparent">
                  <TableHead className="w-[100px] h-9 text-[10px] uppercase font-mono text-zinc-500">订单</TableHead>
                  <TableHead className="h-9 text-[10px] uppercase font-mono text-zinc-500">账户</TableHead>
                  <TableHead className="h-9 text-[10px] uppercase font-mono text-zinc-500">代码</TableHead>
                  <TableHead className="h-9 text-[10px] uppercase font-mono text-zinc-500">方向</TableHead>
                  <TableHead className="h-9 text-[10px] uppercase font-mono text-zinc-500">数量</TableHead>
                  <TableHead className="h-9 text-[10px] uppercase font-mono text-zinc-500">状态</TableHead>
                  <TableHead className="h-9 text-right text-[10px] uppercase font-mono text-zinc-500">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {syncableLiveOrders.length > 0 ? (
                  syncableLiveOrders.map((order) => (
                    <TableRow key={order.id} className="border-white/5 hover:bg-white/5 transition-colors">
                      <TableCell className="font-mono text-xs text-zinc-400">{shrink(order.id)}</TableCell>
                      <TableCell className="text-xs text-zinc-300">{order.accountId}</TableCell>
                      <TableCell className="text-xs font-medium text-emerald-400">{order.symbol}</TableCell>
                      <TableCell className="text-xs">
                        <Badge variant="outline" className={order.side === "BUY" ? "border-emerald-500/30 text-emerald-400" : "border-rose-500/30 text-rose-400"}>
                          {order.side}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-xs text-zinc-300 font-mono">{formatMaybeNumber(order.quantity)}</TableCell>
                      <TableCell className="text-xs">
                         <span className="inline-flex items-center gap-1.5">
                           <span className="h-1.5 w-1.5 rounded-full bg-emerald-500 animate-pulse" />
                           {order.status}
                         </span>
                      </TableCell>
                      <TableCell className="text-right">
                        <Button 
                          size="sm" 
                          variant="outline" 
                          className="h-7 px-3 text-[10px] border-emerald-500/20 hover:bg-emerald-500/10 hover:text-emerald-300"
                          disabled={liveSyncAction !== null}
                          onClick={() => syncLiveOrder(order.id)}
                        >
                          {liveSyncAction === order.id ? "Syncing..." : "Sync"}
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))
                ) : (
                  <TableRow>
                    <TableCell colSpan={7} className="h-24 text-center text-xs text-zinc-600 italic">
                      暂无已接受的实盘订单
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </div>
        </CardContent>
      </Card>

      {/* 4. 平台健康总览 */}
      <Card className="border-white/5 bg-zinc-900/50 backdrop-blur-sm overflow-hidden">
        <CardHeader className="flex flex-row items-center justify-between pb-4 space-y-0">
          <div>
            <CardDescription className="text-emerald-500/80 font-mono text-[10px] uppercase tracking-wider">Platform Health</CardDescription>
            <CardTitle className="text-xl font-bold tracking-tight text-zinc-100">平台健康总览</CardTitle>
          </div>
          <div className="flex items-center gap-3 text-[10px] font-mono">
            <Badge variant="secondary" className="bg-emerald-500/10 text-emerald-400 border-emerald-500/20">
              {technicalStatusLabel(monitorHealth?.status ?? "--")}
            </Badge>
            <span className="text-zinc-500">{formatTime(String(monitorHealth?.generatedAt ?? ""))}</span>
          </div>
        </CardHeader>
        <CardContent className="space-y-6">
          {monitorHealth ? (
            <div className="space-y-6">
              <div className="grid grid-cols-6 gap-2">
                {[
                  { label: "告警总数", value: monitorHealth.alertCounts.total, color: "zinc" },
                  { label: "Critical", value: monitorHealth.alertCounts.critical, color: "rose" },
                  { label: "Warning", value: monitorHealth.alertCounts.warning, color: "amber" },
                  { label: "静默阈值", value: runtimePolicyValueLabel(platformRuntimePolicy?.runtimeQuietSeconds), color: "zinc" },
                  { label: "评估阈值", value: runtimePolicyValueLabel(platformRuntimePolicy?.strategyEvaluationQuietSeconds), color: "zinc" },
                  { label: "同步阈值", value: runtimePolicyValueLabel(platformRuntimePolicy?.liveAccountSyncFreshnessSeconds), color: "zinc" },
                ].map((item) => (
                  <div key={item.label} className={`flex flex-col gap-1 p-2 rounded-lg border ${item.color === 'rose' ? 'bg-rose-500/5 border-rose-500/10' : item.color === 'amber' ? 'bg-amber-500/5 border-amber-500/10' : 'bg-zinc-800/30 border-white/5'}`}>
                    <span className={`text-[10px] uppercase font-mono ${item.color === 'rose' ? 'text-rose-500/70' : item.color === 'amber' ? 'text-amber-500/70' : 'text-zinc-500'}`}>{item.label}</span>
                    <strong className={`text-sm ${item.color === 'rose' ? 'text-rose-400' : item.color === 'amber' ? 'text-amber-400' : 'text-zinc-200'}`}>{String(item.value ?? 0)}</strong>
                  </div>
                ))}
              </div>

              <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                <div className="space-y-3">
                  <h4 className="text-xs font-semibold text-zinc-400 flex items-center gap-2">
                    <span className="h-1 w-1 rounded-full bg-blue-500" />
                    Live Accounts
                  </h4>
                  <div className="rounded-md border border-white/5 overflow-hidden">
                    <Table>
                      <TableHeader className="bg-white/5">
                        <TableRow className="hover:bg-transparent border-white/5">
                          <TableHead className="h-8 text-[9px] uppercase font-mono">账户</TableHead>
                          <TableHead className="h-8 text-[9px] uppercase font-mono">状态</TableHead>
                          <TableHead className="h-8 text-[9px] uppercase font-mono text-right">同步年龄</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {monitorHealth.liveAccounts.map((item) => (
                          <TableRow key={item.name} className="border-white/5 hover:bg-white/5">
                            <TableCell className="py-2 text-[11px] text-zinc-300 font-medium">{item.name}</TableCell>
                            <TableCell className="py-2">
                              <Badge variant={item.syncStale ? "destructive" : "secondary"} className="h-4 px-1 text-[9px]">
                                {technicalStatusLabel(item.status)}
                              </Badge>
                            </TableCell>
                            <TableCell className="py-2 text-right text-[10px] font-mono text-zinc-400">{String(item.syncAgeSeconds ?? 0)}s</TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </div>
                </div>

                <div className="space-y-3">
                  <h4 className="text-xs font-semibold text-zinc-400 flex items-center gap-2">
                    <span className="h-1 w-1 rounded-full bg-purple-500" />
                    Runtime Sessions
                  </h4>
                  <div className="rounded-md border border-white/5 overflow-hidden">
                    <Table>
                      <TableHeader className="bg-white/5">
                        <TableRow className="hover:bg-transparent border-white/5">
                          <TableHead className="h-8 text-[9px] uppercase font-mono">策略</TableHead>
                          <TableHead className="h-8 text-[9px] uppercase font-mono">健康</TableHead>
                          <TableHead className="h-8 text-[9px] uppercase font-mono text-right">最后事件</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {monitorHealth.runtimeSessions.map((item) => (
                          <TableRow key={item.strategyId} className="border-white/5 hover:bg-white/5">
                            <TableCell className="py-2 text-[11px] text-zinc-300 font-medium">{item.strategyName || shrink(item.strategyId)}</TableCell>
                            <TableCell className="py-2">
                              <Badge variant={item.health === "CRITICAL" ? "destructive" : "secondary"} className="h-4 px-1 text-[9px]">
                                {technicalStatusLabel(item.health)}
                              </Badge>
                            </TableCell>
                            <TableCell className="py-2 text-right text-[10px] font-mono text-zinc-400">{formatTime(String(item.lastEventAt ?? ""))}</TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </div>
                </div>
              </div>
            </div>
          ) : (
            <div className="h-48 flex items-center justify-center border border-white/5 rounded-xl text-zinc-600 italic text-xs bg-zinc-950/20">
              平台健康快照尚未加载
            </div>
          )}
        </CardContent>
      </Card>

      {/* 5. 记录中心 */}
      <Card className="border-white/5 bg-zinc-900/50 backdrop-blur-sm overflow-hidden">
        <CardHeader className="pb-4">
          <CardDescription className="text-zinc-500 font-mono text-[10px] uppercase tracking-wider">Historical Records</CardDescription>
          <CardTitle className="text-xl font-bold tracking-tight text-zinc-100">订单、持仓、成交与异常</CardTitle>
        </CardHeader>
        <CardContent>
          <Tabs defaultValue="orders" value={dockTab} onValueChange={(val) => onDockTabChange(val as 'orders' | 'positions' | 'fills' | 'alerts')}>
            <TabsList className="grid grid-cols-4 w-full max-w-md bg-zinc-950/40 border border-white/5 p-1 mb-4">
              <TabsTrigger value="orders" className="text-xs transition-all data-[state=active]:bg-emerald-500/10 data-[state=active]:text-emerald-300">全部订单</TabsTrigger>
              <TabsTrigger value="positions" className="text-xs transition-all data-[state=active]:bg-emerald-500/10 data-[state=active]:text-emerald-300">持仓</TabsTrigger>
              <TabsTrigger value="fills" className="text-xs transition-all data-[state=active]:bg-emerald-500/10 data-[state=active]:text-emerald-300">成交明细</TabsTrigger>
              <TabsTrigger value="alerts" className="text-xs transition-all data-[state=active]:bg-emerald-500/10 data-[state=active]:text-emerald-300">异常告警</TabsTrigger>
            </TabsList>
            <TabsContent value={dockTab} className="mt-0 ring-offset-zinc-950 focus-visible:ring-0">
              <div className="rounded-xl bg-zinc-950/20 border border-white/5 overflow-hidden">
                {dockContent}
              </div>
            </TabsContent>
          </Tabs>
        </CardContent>
      </Card>
    </div>
  );
}
