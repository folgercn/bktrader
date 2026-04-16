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
    <div className="h-full overflow-y-auto p-4 space-y-4">
      {/* 1. 主监控区 - 严格保留原生 .panel 样式 */}
      <section className="panel panel-market panel-compact monitor-panel-main w-full">
         <div className="panel-header">
           <div>
             <p className="panel-kicker">主监控</p>
             <h3>运行中会话的大周期 K 线与执行状态</h3>
           </div>
           <div className="range-box">
             <div className="flex items-center gap-1.5 mr-2 pr-2 border-r border-[#d8cfba]">
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
                       ? 'bg-[#1f2328] text-white'
                       : 'text-[#687177] hover:text-[#1f2328]'
                   }`}
                 >
                   {tf.label}
                 </button>
               ))}
             </div>
             <span>{monitorMode}</span>
           </div>
         </div>
         <div className="chart-shell h-[320px]">
            {monitorBars.length > 0 ? (
              <SignalMonitorChart candles={monitorBars} markers={monitorMarkers} />
            ) : (
              <div className="empty-state">当前运行会话还没有交易所大周期 K 线缓存</div>
            )}
         </div>
         <div className="grid grid-cols-4 md:grid-cols-8 gap-1.5 mt-4">
            {[
              { label: "模式", value: monitorMode },
              { label: "账户净值", value: formatMoney(monitorSummary?.netEquity) },
              { label: "未解盈亏", value: formatSigned(monitorSummary?.unrealizedPnl) },
              { label: "方向", value: String(monitorExecutionSummary.position?.side ?? "FLAT") },
              { label: "数量", value: formatMaybeNumber(monitorExecutionSummary.position?.quantity) },
              { label: "标记价", value: formatMaybeNumber(monitorExecutionSummary.position?.markPrice) },
              { label: "盘口", value: `${formatMaybeNumber(monitorMarket.tradePrice)}` },
              { label: "SMA5", value: formatMaybeNumber(monitorSignalState.sma5) },
            ].map((item) => (
              <div key={item.label} className="detail-item">
                <span>{item.label}</span>
                <strong>{item.value}</strong>
              </div>
            ))}
         </div>
      </section>

      {/* 2. 重构区 - 使用 shadcn 组件但强制映射原生配色 (Cream/Panel-style) */}
      <Card className="border-[#d8cfba] bg-[var(--panel)] shadow-[var(--shadow)] rounded-[24px]">
        <CardHeader className="pb-3 px-4">
          <CardTitle className="text-xl font-bold text-[#1f2328]">运行监控与人工干预</CardTitle>
        </CardHeader>
        <CardContent className="space-y-6 px-4">
          <div className="live-grid">
            {/* 左侧：优先会话面板 */}
            {highlightedLiveSession ? (
              <div className="bg-[#fff8ea] rounded-[18px] p-4 border border-[#d8cfba] shadow-sm">
                <div className="flex items-center justify-between mb-4">
                  <div>
                    <span className="text-[10px] text-[#687177] font-mono uppercase tracking-wider">Primary Session</span>
                    <h4 className="text-sm font-bold text-[#1f2328]">当前优先处理会话</h4>
                  </div>
                  <Badge className="bg-[#ebe5d5] text-[#1f2328] border-[#d8cfba] font-mono text-[9px] px-1.5 py-0">
                    {highlightedLiveSession.health.status}
                  </Badge>
                </div>
                
                <div className="flex flex-wrap gap-1.5 text-[10px] text-[#687177] font-mono mb-4">
                  <span className="bg-white/60 px-1.5 py-0.5 rounded border border-[#d8cfba]/50 font-bold">{shrink(highlightedLiveSession.session.id)}</span>
                  <span className="bg-white/60 px-1.5 py-0.5 rounded border border-[#d8cfba]/50">{highlightedLiveSession.session.accountId}</span>
                  <span className="bg-[#d9eee8] text-[#0e6d60] px-1.5 py-0.5 rounded border border-[#0e6d60]/20">{String(highlightedLiveSession.session.state?.signalTimeframe ?? "--")}</span>
                </div>

                <div className="space-y-3 bg-white/40 p-3 rounded-xl border border-[#d8cfba]/30">
                  <p className="text-[11px] text-[#1f2328] leading-relaxed">
                    <span className="text-[#687177] font-medium">健康摘要:</span> {highlightedLiveSession.health.detail}
                  </p>
                  <div className="grid grid-cols-2 gap-x-4 gap-y-2 text-[10px]">
                    <div className="detail-item !bg-transparent !border-0 !p-0">
                      <span>执行统计</span>
                      <strong className="!text-[11px] !mt-0.5">订单 {highlightedLiveSession.execution.orderCount} · 成交 {highlightedLiveSession.execution.fillCount}</strong>
                    </div>
                    <div className="detail-item !bg-transparent !border-0 !p-0">
                      <span>持仓方向</span>
                      <strong className="!text-[11px] !mt-0.5">{String(monitorExecutionSummary.position?.side ?? "FLAT")}</strong>
                    </div>
                  </div>
                </div>

                <div className="mt-5 flex flex-wrap gap-2">
                  {monitorFlow.map((step) => (
                    <Badge 
                      key={step.key}
                      variant="outline"
                      className="text-[9px] bg-white/80 text-[#687177] border-[#d8cfba] font-medium"
                    >
                      {step.label}
                    </Badge>
                  ))}
                </div>
              </div>
            ) : (
              <div className="p-12 text-center text-[#687177] text-xs italic bg-[#fff8ea]/50 rounded-[18px] border border-dashed border-[#d8cfba]">
                当前没有活跃实盘会话
              </div>
            )}

            {/* 右侧：监控明细折叠面板 */}
            <div className="space-y-4">
              <div className="flex items-center justify-between border-b border-[#d8cfba] pb-2">
                <h4 className="text-sm font-bold text-[#1f2328]">当前会话监控细节</h4>
                <Badge variant="outline" className="text-[9px] border-[#d8cfba] text-[#687177] font-mono tracking-tighter">
                  {monitorSession ? "CONNECTED" : "IDLE"}
                </Badge>
              </div>
              
              {monitorSession ? (
                <div className="space-y-4">
                  <div className="grid grid-cols-2 gap-2">
                    {monitorSummaryItems.map((item) => (
                      <div key={item.label} className="session-stat !bg-white/50 !p-2 !rounded-xl !border-[#d8cfba]/40">
                        <span className="!text-[9px] !mb-1 uppercase tracking-tight font-bold">{item.label}</span>
                        <strong className="!text-[10px] !text-[#1f2328] truncate">{item.value}</strong>
                      </div>
                    ))}
                  </div>
                  
                  <Accordion multiple className="w-full">
                    {monitorSections.map((section) => (
                      <AccordionItem key={section.title} value={section.title} className="border-[#d8cfba]">
                        <AccordionTrigger className="hover:no-underline py-2 text-xs font-bold text-[#1f2328] opacity-80">
                          {section.title}
                        </AccordionTrigger>
                        <AccordionContent>
                          <div className="grid grid-cols-1 gap-1 pt-1 pb-3">
                            {section.items.map((item) => (
                              <div key={`${section.title}-${item.label}`} className="flex justify-between items-center text-[10px] px-2 py-1.5 rounded bg-[#fff8ea]/40 border border-[#d8cfba]/20 hover:bg-[#fff8ea]/60 transition-colors">
                                <span className="text-[#687177]">{item.label}</span>
                                <strong className="text-[#1f2328] font-mono">{item.value}</strong>
                              </div>
                            ))}
                          </div>
                        </AccordionContent>
                      </AccordionItem>
                    ))}
                  </Accordion>
                </div>
              ) : (
                <div className="p-20 text-center text-[#687177] text-xs italic bg-white/30 rounded-[18px] border border-dashed border-[#d8cfba]">
                  选中会话后显示明细
                </div>
              )}
            </div>
          </div>
        </CardContent>
      </Card>

      {/* 3. 待同步订单 - 严格映射原生配色表 */}
      <Card className="border-[#d8cfba] bg-[var(--panel)] shadow-[var(--shadow)] rounded-[24px] overflow-hidden">
        <CardHeader className="pb-3 px-4">
          <CardTitle className="text-sm font-bold text-[#1f2328]">待同步的实盘订单</CardTitle>
        </CardHeader>
        <CardContent className="px-4">
          <div className="rounded-xl border border-[#d8cfba] bg-white/40 overflow-hidden">
            <Table>
              <TableHeader className="bg-[#ebe5d5]/40">
                <TableRow className="hover:bg-transparent border-[#d8cfba]">
                  <TableHead className="h-9 text-[10px] uppercase font-bold text-[#687177]">订单</TableHead>
                  <TableHead className="h-9 text-[10px] uppercase font-bold text-[#687177]">账户</TableHead>
                  <TableHead className="h-9 text-[10px] uppercase font-bold text-[#687177]">代码</TableHead>
                  <TableHead className="h-9 text-[10px] uppercase font-bold text-[#687177]">方向</TableHead>
                  <TableHead className="h-9 text-[10px] uppercase font-bold text-[#687177]">数量</TableHead>
                  <TableHead className="h-9 text-[10px] uppercase font-bold text-[#687177]">状态</TableHead>
                  <TableHead className="h-9 text-right text-[10px] uppercase font-bold text-[#687177]">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {syncableLiveOrders.length > 0 ? (
                  syncableLiveOrders.map((order) => (
                    <TableRow key={order.id} className="hover:bg-[#fff8ea]/50 border-[#d8cfba]/30 transition-colors">
                      <TableCell className="font-mono text-xs text-[#1f2328]/80">{shrink(order.id)}</TableCell>
                      <TableCell className="text-xs text-[#687177]">{order.accountId}</TableCell>
                      <TableCell className="text-xs font-bold text-[#0e6d60]">{order.symbol}</TableCell>
                      <TableCell className="text-xs font-bold text-[#1f2328]">
                        {order.side}
                      </TableCell>
                      <TableCell className="text-xs text-[#1f2328] font-mono">{formatMaybeNumber(order.quantity)}</TableCell>
                      <TableCell className="text-xs">
                        <Badge variant="secondary" className="bg-[#ebe5d5] text-[#687177] font-mono text-[9px] h-4">
                          {order.status}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-right">
                        <Button 
                          size="sm" 
                          variant="outline" 
                          className="h-7 px-3 text-[10px] border-[#d8cfba] bg-white hover:bg-[#fff8ea] text-[#1f2328] font-bold"
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
                    <TableCell colSpan={7} className="h-24 text-center text-xs text-[#687177] italic">
                      暂无待同步订单
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </div>
        </CardContent>
      </Card>

      {/* 4. 底层健康面板 */}
      <Card className="border-[#d8cfba] bg-[var(--panel)] shadow-[var(--shadow)] rounded-[24px]">
        <CardHeader className="flex flex-row items-center justify-between pb-4 space-y-0 px-4 pt-4">
          <CardTitle className="text-xl font-bold text-[#1f2328]">平台健康总览</CardTitle>
          <Badge className="bg-[#d9eee8] text-[#0e6d60] border-[#0e6d60]/20 font-bold">
             {technicalStatusLabel(monitorHealth?.status ?? "--")}
          </Badge>
        </CardHeader>
        <CardContent className="px-4 pb-4">
          {monitorHealth ? (
            <div className="grid grid-cols-6 gap-2">
              {[
                { label: "告警", value: monitorHealth.alertCounts.total },
                { label: "Critical", value: monitorHealth.alertCounts.critical, color: 'text-[#b04a37]' },
                { label: "Warning", value: monitorHealth.alertCounts.warning, color: 'text-amber-700' },
                { label: "静默", value: runtimePolicyValueLabel(platformRuntimePolicy?.runtimeQuietSeconds) },
                { label: "评估", value: runtimePolicyValueLabel(platformRuntimePolicy?.strategyEvaluationQuietSeconds) },
                { label: "刷新", value: runtimePolicyValueLabel(platformRuntimePolicy?.liveAccountSyncFreshnessSeconds) },
              ].map((item) => (
                <div key={item.label} className="p-2 rounded-xl bg-white/60 border border-[#d8cfba]">
                  <span className="text-[9px] text-[#687177] uppercase font-bold block mb-1">{item.label}</span>
                  <strong className={`text-sm tracking-tight ${item.color || 'text-[#1f2328]'}`}>
                    {String(item.value ?? 0)}
                  </strong>
                </div>
              ))}
            </div>
          ) : (
            <div className="p-10 text-center text-[#687177] text-xs italic bg-[#fff8ea]/30 rounded-[18px] border border-[#d8cfba]">
              健康监控未就绪
            </div>
          )}
        </CardContent>
      </Card>

      {/* 5. 记录中心 Tabs */}
      <Card className="border-[#d8cfba] bg-[var(--panel)] shadow-[var(--shadow)] rounded-[24px]">
        <CardContent className="pt-6 px-4">
          <Tabs defaultValue="orders" value={dockTab} onValueChange={(val) => onDockTabChange(val as any)}>
            <TabsList className="bg-[#ebe5d5] p-1 mb-4 h-9 rounded-xl">
              <TabsTrigger value="orders" className="text-xs data-[state=active]:bg-white data-[state=active]:text-[#1f2328]">订单</TabsTrigger>
              <TabsTrigger value="positions" className="text-xs data-[state=active]:bg-white data-[state=active]:text-[#1f2328]">持仓</TabsTrigger>
              <TabsTrigger value="fills" className="text-xs data-[state=active]:bg-white data-[state=active]:text-[#1f2328]">成交</TabsTrigger>
              <TabsTrigger value="alerts" className="text-xs data-[state=active]:bg-white data-[state=active]:text-[#1f2328]">告警</TabsTrigger>
            </TabsList>
            <div className="rounded-2xl border border-[#d8cfba] bg-white/40 overflow-hidden shadow-inner">
               {dockContent}
            </div>
          </Tabs>
        </CardContent>
      </Card>
    </div>
  );
}
