import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useUIStore } from '../store/useUIStore';
import { useTradingStore } from '../store/useTradingStore';
import { SignalMonitorChart } from '../components/charts/SignalMonitorChart';
import { LiveTradePairsCard } from '../components/live/LiveTradePairsCard';
import { formatMoney, formatSigned, formatMaybeNumber, formatTime, shrink } from '../utils/format';
import { 
  getRecord, 
  getList,
  resolveChartAnchor,
  buildTimeRange,
  deriveSignalBarCandles,
  deriveSignalBarStateCandles,
  mapChartCandlesToSignalBarCandles,
  derivePrimarySignalBarState, 
  deriveRuntimeMarketSnapshot, 
  deriveSessionMarkers,
  deriveSignalMonitorDecorations,
  derivePaperSessionExecutionSummary,
  deriveSelectedOrHighlightedLiveSession,
  deriveLiveDispatchPreview,
  deriveLiveSessionFlow,
  deriveRuntimeSourceSummary,
  deriveRuntimeReadiness,
  deriveLiveSessionExecutionSummary,
  deriveLiveSessionHealth,
  mergeLivePriceIntoSignalBars,
  buildTimelineNotes,
  liveSessionHealthTone,
  getNumber,
  runtimePolicyValueLabel,
  technicalStatusLabel
} from '../utils/derivation';
import { fetchJSON } from '../utils/api';
import { hasLiveSessionDetailFields, mergeLiveSessionDetail } from '../utils/liveSessionDetail';
import { Card, CardHeader, CardTitle, CardContent } from '../components/ui/card';
import { Badge } from '../components/ui/badge';
import { Table, TableHeader, TableBody, TableHead, TableRow, TableCell } from '../components/ui/table';
import { Accordion, AccordionItem, AccordionTrigger, AccordionContent } from '../components/ui/accordion';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '../components/ui/tabs';
import { Button } from '../components/ui/button';
import { Activity, Layout, ShieldCheck, Zap, BarChart3, Clock, ArrowRightLeft, HeartPulse, LineChart, CandlestickChart, Compass, ShieldAlert, FileText, Layers, ChevronDown, Settings, Filter } from 'lucide-react';

import { Popover, PopoverContent, PopoverTrigger } from '../components/ui/popover';
import { Input } from '../components/ui/input';
import { Label } from '../components/ui/label';

import { cn } from '../lib/utils';
import type { ChartCandle, LiveSession, SignalBarCandle } from '../types/domain';

const MONITOR_HISTORY_CANDLE_LIMIT = 240;
const MONITOR_CANDLE_EDGE_THRESHOLD = 24;
const MONITOR_CANDLE_CACHE_LIMIT = 1500;
const MONITOR_LIVE_SESSION_DETAIL_FIELDS = [
  "timeline",
  "breakoutHistory",
  "lastStrategyEvaluationSignalBarStates",
];

function resolveMonitorFallbackResolution(timeframe: string) {
  const normalized = String(timeframe ?? "").trim().toLowerCase();
  const supported: Record<string, string> = {
    "5m": "5",
    "15m": "15",
    "30m": "30",
    "1h": "60",
    "2h": "120",
    "4h": "240",
    "1d": "1D",
  };
  return supported[normalized] ?? "5";
}

function monitorResolutionSeconds(resolution: string) {
  const normalized = String(resolution ?? "").trim().toUpperCase();
  if (normalized === "1D") {
    return 24 * 60 * 60;
  }
  const minutes = Number.parseInt(normalized, 10);
  return Number.isFinite(minutes) && minutes > 0 ? minutes * 60 : 5 * 60;
}

function buildMonitorCandleRange(
  anchorDate: Date,
  resolution: string,
  limit: number,
  requestedRange: { from: number; to: number }
) {
  const anchor = Math.floor(anchorDate.getTime() / 1000);
  const seconds = monitorResolutionSeconds(resolution);
  const historyRange = {
    from: anchor - seconds * Math.max(limit - 5, 1),
    to: anchor + seconds * 5,
  };
  return {
    from: Math.min(requestedRange.from, historyRange.from),
    to: Math.max(requestedRange.to, historyRange.to),
  };
}

function mergeMonitorCandles(existing: ChartCandle[], incoming: ChartCandle[], direction: "older" | "newer") {
  const byTime = new Map<string, ChartCandle>();
  for (const item of existing) {
    if (item.time) {
      byTime.set(item.time, item);
    }
  }
  for (const item of incoming) {
    if (item.time) {
      byTime.set(item.time, item);
    }
  }
  const merged = Array.from(byTime.values()).sort((a, b) => Date.parse(a.time) - Date.parse(b.time));
  if (merged.length <= MONITOR_CANDLE_CACHE_LIMIT) {
    return merged;
  }
  return direction === "older"
    ? merged.slice(0, MONITOR_CANDLE_CACHE_LIMIT)
    : merged.slice(merged.length - MONITOR_CANDLE_CACHE_LIMIT);
}

function mergeSignalBars(fallbackBars: SignalBarCandle[], runtimeBars: SignalBarCandle[]) {
  const byTime = new Map<string, SignalBarCandle>();
  for (const item of fallbackBars) {
    if (item.time) {
      byTime.set(item.time, item);
    }
  }
  for (const item of runtimeBars) {
    if (item.time) {
      byTime.set(item.time, item);
    }
  }
  return Array.from(byTime.values()).sort((a, b) => Date.parse(a.time) - Date.parse(b.time));
}

type MonitorStageProps = {
  syncLiveOrder: (id: string) => void;
  dockTab: 'pairs' | 'orders' | 'positions' | 'fills' | 'alerts';
  onDockTabChange: (tab: 'pairs' | 'orders' | 'positions' | 'fills' | 'alerts') => void;
  dockContent: React.ReactNode;
};

export function MonitorStage({ syncLiveOrder, dockTab, onDockTabChange, dockContent }: MonitorStageProps) {
  const liveSessions = useTradingStore(s => s.liveSessions);
  const setLiveSessions = useTradingStore(s => s.setLiveSessions);
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
  const timeWindow = useUIStore(s => s.timeWindow);
  const chartOverrideRange = useUIStore(s => s.chartOverrideRange);
  const selectedSignalRuntimeId = useTradingStore(s => s.selectedSignalRuntimeId);
  const setSelectedSignalRuntimeId = useTradingStore(s => s.setSelectedSignalRuntimeId);
  const monitorCandles = useTradingStore(s => s.monitorCandles);
  const setMonitorCandles = useTradingStore(s => s.setMonitorCandles);
  const timelineConfig = useUIStore(s => s.timelineConfig);
  const setTimelineConfig = useUIStore(s => s.setTimelineConfig);
  const fallbackRequestKeyRef = useRef<string>("");
  const candleCacheRef = useRef<Record<string, { candles: ChartCandle[]; fetchedAt: number }>>({});
  const candleExpansionRequestKeyRef = useRef<string>("");
  const [liveSessionDetailStatus, setLiveSessionDetailStatus] = useState<"idle" | "loading" | "loaded" | "error">("idle");


  // 1. 高亮会话选择逻辑
  const highlightedLiveSession = useMemo(
    () => deriveSelectedOrHighlightedLiveSession(liveSessions, selectedSignalRuntimeId, orders, fills, positions),
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
    // 直接设置会话 ID，highlightedLiveSession 逻辑会负责匹配 s.id 或 runtimeId
    setSelectedSignalRuntimeId(sid);
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

  useEffect(() => {
    if (!monitorSession?.id) {
      setLiveSessionDetailStatus("idle");
      return;
    }
    if (hasLiveSessionDetailFields(monitorSession, MONITOR_LIVE_SESSION_DETAIL_FIELDS)) {
      setLiveSessionDetailStatus("loaded");
      return;
    }

    let active = true;
    setLiveSessionDetailStatus("loading");
    fetchJSON<LiveSession>(
      `/api/v1/live/sessions/${encodeURIComponent(monitorSession.id)}/detail?fields=${MONITOR_LIVE_SESSION_DETAIL_FIELDS.map(encodeURIComponent).join(",")}`
    )
      .then((detail) => {
        if (!active) {
          return;
        }
        setLiveSessions((current) => mergeLiveSessionDetail(current, detail, MONITOR_LIVE_SESSION_DETAIL_FIELDS));
        setLiveSessionDetailStatus("loaded");
      })
      .catch((error) => {
        if (!active) {
          return;
        }
        console.warn("Failed to load live session detail", error);
        setLiveSessionDetailStatus("error");
      });

    return () => {
      active = false;
    };
  }, [monitorSession, setLiveSessions]);

  const sessionSymbol = String(monitorSession?.state?.symbol ?? "").trim().toUpperCase();
  const monitorSignalContext = getRecord(monitorSessionState.lastStrategyEvaluationContext);
  const monitorDecisionMeta = getRecord(getRecord(monitorSession?.state?.lastStrategyDecision).metadata);
  const monitorSignalTimeframeHint = String(
    monitorSessionState.signalTimeframe ?? monitorSignalContext.signalTimeframe ?? monitorDecisionMeta.signalTimeframe ?? ""
  ).trim().toLowerCase();
  const monitorSignalBarStateKey = String(
    monitorSessionState.lastStrategyEvaluationSignalBarStateKey ?? monitorDecisionMeta.signalBarStateKey ?? ""
  ).trim();
  const monitorSignalState = derivePrimarySignalBarState(
    getRecord(highlightedLiveRuntimeState.signalBarStates),
    {
      fallbackStates: getRecord(monitorSessionState.lastStrategyEvaluationSignalBarStates),
      targetSymbol: sessionSymbol,
      targetTimeframe: monitorSignalTimeframeHint,
      targetStateKey: monitorSignalBarStateKey,
    }
  );
  const monitorSignalSymbol = String(
    monitorSignalState.symbol ?? getRecord(monitorSignalState.current).symbol ?? sessionSymbol
  ).trim().toUpperCase();
  const monitorSignalTimeframe = String(
    monitorSignalState.timeframe ??
      getRecord(monitorSignalState.current).timeframe ??
      monitorSessionState.signalTimeframe ??
      monitorSignalContext.signalTimeframe ??
      monitorDecisionMeta.signalTimeframe ??
      ""
  ).trim().toLowerCase();
  const monitorSymbol = monitorSignalSymbol || sessionSymbol;

  const monitorBars = useMemo(() => {
    const sourceBars = deriveSignalBarCandles(getRecord(highlightedLiveRuntimeState.sourceStates), {
      targetSymbol: monitorSymbol,
      targetTimeframe: monitorSignalTimeframe,
      targetStateKey: monitorSignalBarStateKey,
    });
    const stateBars = deriveSignalBarStateCandles(getRecord(highlightedLiveRuntimeState.signalBarStates), {
      targetSymbol: monitorSymbol,
      targetTimeframe: monitorSignalTimeframe,
      targetStateKey: monitorSignalBarStateKey,
    });
    return mergeSignalBars(sourceBars, stateBars);
  }, [
    highlightedLiveRuntimeState.sourceStates,
    highlightedLiveRuntimeState.signalBarStates,
    monitorSymbol,
    monitorSignalTimeframe,
    monitorSignalBarStateKey,
  ]);

  const fallbackResolution = useMemo(
    () => resolveMonitorFallbackResolution(monitorSignalTimeframe),
    [monitorSignalTimeframe]
  );
  const fallbackMonitorBars = useMemo(
    () => mapChartCandlesToSignalBarCandles(monitorCandles, fallbackResolution),
    [monitorCandles, fallbackResolution]
  );
  const monitorMarket = deriveRuntimeMarketSnapshot(
    getRecord(monitorRuntimeState.sourceStates),
    getRecord(monitorRuntimeState.lastEventSummary),
    monitorSymbol
  );
  const monitorSourceSummary = deriveRuntimeSourceSummary(
    getRecord(monitorRuntimeState.sourceStates),
    runtimePolicy,
    monitorSymbol
  );
  const displayMonitorBars = useMemo(
    () =>
      mergeLivePriceIntoSignalBars(
        mergeSignalBars(fallbackMonitorBars, monitorBars),
        monitorMarket.tradePrice,
        fallbackResolution,
        monitorMarket.tradePriceAt
      ),
    [fallbackMonitorBars, fallbackResolution, monitorBars, monitorMarket.tradePrice, monitorMarket.tradePriceAt]
  );

  useEffect(() => {
    const monitorSessionId = monitorSession?.id ?? "";
    if (!monitorSessionId || !monitorSymbol) {
      fallbackRequestKeyRef.current = "";
      setMonitorCandles([]);
      return;
    }

    const anchorDate = resolveChartAnchor(monitorSession);
    const requestedRange = chartOverrideRange ?? buildTimeRange(anchorDate, timeWindow);
    const range = buildMonitorCandleRange(anchorDate, fallbackResolution, MONITOR_HISTORY_CANDLE_LIMIT, requestedRange);

    const requestKey = [
      monitorSessionId,
      monitorSymbol,
      fallbackResolution,
      timeWindow,
      chartOverrideRange?.from ?? "",
      chartOverrideRange?.to ?? "",
      range.from,
      range.to
    ].join(":");

    const now = Date.now();
    const cached = candleCacheRef.current[requestKey];
    const isSameKey = fallbackRequestKeyRef.current === requestKey;
    const isExpired = !cached || now - cached.fetchedAt > 30000;

    if (isSameKey && !isExpired) {
      return;
    }

    if (cached && !isExpired) {
      setMonitorCandles(cached.candles);
      fallbackRequestKeyRef.current = requestKey;
      return;
    }

    fallbackRequestKeyRef.current = requestKey;

    fetchJSON<{ candles: ChartCandle[] }>(
      `/api/v1/chart/candles?symbol=${encodeURIComponent(monitorSymbol)}&resolution=${fallbackResolution}&from=${range.from}&to=${range.to}&limit=${MONITOR_HISTORY_CANDLE_LIMIT}`
    )
      .then((payload) => {
        if (fallbackRequestKeyRef.current !== requestKey) {
          return;
        }
        const candles = Array.isArray(payload?.candles) ? payload.candles : [];
        candleCacheRef.current[requestKey] = {
          candles,
          fetchedAt: Date.now(),
        };
        setMonitorCandles(candles);
      })
      .catch((error) => {
        if (fallbackRequestKeyRef.current !== requestKey) {
          return;
        }
        console.warn("Failed to load monitor fallback candles", error);
        candleCacheRef.current[requestKey] = {
          candles: [],
          fetchedAt: Date.now(),
        };
        setMonitorCandles([]);
      });

  }, [
    chartOverrideRange,
    fallbackResolution,
    monitorSession,
    monitorSymbol,
    setMonitorCandles,
    timeWindow,
  ]);

  const expandMonitorCandles = useCallback(
    (direction: "older" | "newer") => {
      if (!monitorSymbol || monitorCandles.length === 0) {
        return;
      }
      const ordered = [...monitorCandles].sort((a, b) => Date.parse(a.time) - Date.parse(b.time));
      const firstTime = Date.parse(ordered[0]?.time ?? "");
      const lastTime = Date.parse(ordered[ordered.length - 1]?.time ?? "");
      if (!Number.isFinite(firstTime) || !Number.isFinite(lastTime)) {
        return;
      }
      const stepSeconds = monitorResolutionSeconds(fallbackResolution);
      const stepMs = stepSeconds * 1000;
      const fromMs =
        direction === "older"
          ? firstTime - stepMs * MONITOR_HISTORY_CANDLE_LIMIT
          : lastTime + stepMs;
      const toMs =
        direction === "older"
          ? firstTime - stepMs
          : lastTime + stepMs * MONITOR_HISTORY_CANDLE_LIMIT;
      const from = Math.floor(fromMs / 1000);
      const to = Math.floor(toMs / 1000);
      const requestKey = [monitorSymbol, fallbackResolution, direction, from, to].join(":");
      if (candleExpansionRequestKeyRef.current === requestKey) {
        return;
      }
      candleExpansionRequestKeyRef.current = requestKey;

      fetchJSON<{ candles: ChartCandle[] }>(
        `/api/v1/chart/candles?symbol=${encodeURIComponent(monitorSymbol)}&resolution=${fallbackResolution}&from=${from}&to=${to}&limit=${MONITOR_HISTORY_CANDLE_LIMIT}`
      )
        .then((payload) => {
          const candles = Array.isArray(payload?.candles) ? payload.candles : [];
          if (candles.length === 0) {
            return;
          }
          setMonitorCandles((current) => mergeMonitorCandles(current, candles, direction));
        })
        .catch((error) => {
          console.warn(`Failed to expand monitor ${direction} candles`, error);
        })
        .finally(() => {
          if (candleExpansionRequestKeyRef.current === requestKey) {
            candleExpansionRequestKeyRef.current = "";
          }
        });
    },
    [fallbackResolution, monitorCandles, monitorSymbol, setMonitorCandles]
  );

  const handleMonitorVisibleRangeChange = useCallback(
    (range: { from: number; to: number; barCount: number }) => {
      if (range.barCount <= 0) {
        return;
      }
      if (range.from <= MONITOR_CANDLE_EDGE_THRESHOLD) {
        expandMonitorCandles("older");
        return;
      }
      if (range.to >= range.barCount - MONITOR_CANDLE_EDGE_THRESHOLD) {
        expandMonitorCandles("newer");
      }
    },
    [expandMonitorCandles]
  );

  const monitorSummary =
    monitorSession ? summaries.find((item) => item.accountId === monitorSession.accountId) ?? null : null;
  const monitorMarkers = deriveSessionMarkers(monitorSession, orders, fills);
  const monitorDecorations = useMemo(
    () =>
      deriveSignalMonitorDecorations(
        monitorSession,
        displayMonitorBars,
        monitorExecutionSummary.position,
        orders,
        fills
      ),
    [displayMonitorBars, monitorExecutionSummary.position, monitorSession, orders, fills]
  );
  const monitorChartMarkers = useMemo(
    () => [...monitorMarkers, ...monitorDecorations.markers],
    [monitorMarkers, monitorDecorations.markers]
  );
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
    monitorSourceSummary,
    { requireTick: true, requireOrderBook: false }
  );
  const monitorIntent = getRecord(monitorSession?.state?.lastStrategyIntent);
  const monitorSignalBarDecision = getRecord(
    monitorSession?.state?.lastStrategyEvaluationSignalBarDecision ?? monitorDecisionMeta.signalBarDecision
  );
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
  const timelineLogs = buildTimelineNotes(monitorTimeline, timelineConfig, monitorSession?.id).slice(0, 50);


  const reconciledOrders = orders.filter(o => !!(o.metadata?.orderLifecycle as any)?.synced);
  const orphanedOrders = orders.filter(o => (o.metadata?.orderLifecycle as any)?.reconciliationState === 'orphaned');
  const reconAuditLabel = orphanedOrders.length > 0 ? `${orphanedOrders.length} 异常` : (reconciledOrders.length > 0 ? "已审计" : "平衡");

  const monitorSummaryItems = monitorSession ? [
    { label: "就绪预检", value: `${monitorRuntimeReadiness.status} · ${monitorRuntimeReadiness.reason}` },
    { label: "信号意图", value: `${String(monitorIntent.action ?? "无")} · ${String(monitorIntent.side ?? "--")}` },
    { label: "指令分发", value: `${String((monitorSession.state as any)?.dispatchMode ?? "--")} · 冷却 ${String((monitorSession.state as any)?.dispatchCooldownSeconds ?? "--")}s` },
    { label: "执行汇总", value: `订单 ${monitorExecutionSummary.orderCount} · 成交 ${monitorExecutionSummary.fillCount}` },
    { label: "对账审计", value: reconAuditLabel },
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
                 <Badge variant="metal">
                   {monitorMode}
                 </Badge>
             </div>
           </div>
         </CardHeader>
         <CardContent className="p-0">
            <div className="chart-shell relative h-[360px] overflow-hidden bg-[color-mix(in_srgb,var(--bk-surface-strong)_40%,transparent)]">
                {displayMonitorBars.length > 0 ? (
                  <SignalMonitorChart
                    candles={displayMonitorBars}
                    markers={monitorChartMarkers}
                    overlays={monitorDecorations.overlays}
                    onVisibleLogicalRangeChange={handleMonitorVisibleRangeChange}
                  />
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


      <div className="grid grid-cols-1 lg:grid-cols-12 gap-6">
        {/* 2. 实时执行与指标监控 (三柱式 Bento 架构) */}
        
        {/* 柱 1: 活跃会话控制 */}
        <Card tone="bento" className="lg:col-span-4 rounded-[32px] overflow-hidden border-2 border-[var(--bk-border-strong)] shadow-xl flex flex-col">
          <CardHeader className="flex flex-row items-center justify-between border-b border-[var(--bk-border-soft)] bg-[var(--bk-surface-ghost)] px-8 py-3.5">
            <div className="flex items-center gap-3">
              <ShieldCheck className="size-5 text-[var(--bk-text-primary)]" />
              <CardTitle className="text-lg font-black text-[var(--bk-text-primary)]">会话控制</CardTitle>
            </div>
            {highlightedLiveSession && (
              <Badge variant="metal">
                {highlightedLiveSession.health.status}
              </Badge>
            )}
          </CardHeader>
          <CardContent className="p-5 flex-1 flex flex-col">
            {highlightedLiveSession ? (
              <div className="flex-1 flex flex-col space-y-5">
                <div className="space-y-3">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                       <span className="text-[10px] font-black uppercase tracking-widest text-[var(--bk-text-muted)] opacity-70">Focus Session</span>
                       <Badge className="h-4 rounded-md border-0 bg-[var(--bk-surface-inverse)] px-1.5 text-[9px] font-black text-[var(--bk-text-contrast)]">
                         {highlightedLiveSession.health.status}
                       </Badge>
                    </div>
                  </div>

                    <div className="flex items-center gap-1.5 overflow-hidden">
                      <Popover>
                        <PopoverTrigger
                          className={`flex h-6 shrink-0 items-center gap-1.5 rounded-lg border border-[var(--bk-border)] bg-[var(--bk-surface)] px-2.5 font-mono text-[10px] font-bold text-[var(--bk-text-primary)] shadow-sm transition-all active:scale-95 ${allSessionItems.length > 1 ? 'cursor-pointer hover:bg-[var(--bk-surface-muted)]' : 'cursor-default'}`}
                          disabled={allSessionItems.length <= 1}
                        >
                          <span className="truncate max-w-[320px]">{highlightedLiveSession.session.alias || highlightedLiveSession.session.id}</span>
                          {allSessionItems.length > 1 && <ChevronDown className="size-2.5 shrink-0 text-[var(--bk-text-muted)] opacity-60" />}
                        </PopoverTrigger>
                        <PopoverContent align="start" className="isolate z-[60] w-[320px] rounded-[20px] border-2 border-[var(--bk-border)] bg-[var(--bk-surface-overlay-strong)] p-2 shadow-xl">
                          <div className="space-y-1.5">
                              {allSessionItems.map((item) => (
                                <div 
                                  key={item.session.id} 
                                  onClick={() => handleSelectSession(item.session.id)}
                                  className={`flex items-center justify-between p-3 rounded-xl border transition-all cursor-pointer ${
                                    item.isHighlighted 
                                      ? 'bg-[var(--bk-status-success-soft)] border-[var(--bk-status-success)]' 
                                      : 'bg-[var(--bk-surface-overlay)] border-[var(--bk-border-soft)] hover:bg-[var(--bk-surface)]'
                                  }`}
                                >
                                  <div className="flex items-center gap-3">
                                      <div className={`size-2 rounded-full transition-colors ${
                                        (item.health.status === "ready" || item.health.status === "active" || item.health.status === "idle") 
                                          ? "bg-[var(--bk-status-success)]" 
                                          : item.health.status === "waiting-sync" 
                                            ? "bg-amber-400"
                                            : item.health.status === "neutral"
                                              ? "bg-[var(--bk-text-muted)] opacity-50"
                                              : "bg-rose-500"
                                      }`} />
                                      <div className="flex flex-col gap-0.5">
                                        <span className="font-mono text-[10px] font-black leading-none">{item.session.alias || item.session.id}</span>
                                        {item.session.alias && (
                                          <span className="font-mono text-[8px] opacity-40 leading-none">{item.session.id}</span>
                                        )}
                                        <span className={cn(
                                          "text-[8px] font-bold uppercase tracking-wider opacity-60",
                                          item.session.status.toLowerCase() === "running" ? "text-[var(--bk-status-success)]" : "text-[var(--bk-text-muted)]"
                                        )}>
                                          {technicalStatusLabel(item.session.status)}
                                        </span>
                                      </div>
                                  </div>
                                  <span className="font-mono text-[10px] font-black tabular-nums">
                                    {formatSigned(item.summary?.unrealizedPnl ?? 0)}
                                  </span>
                                </div>
                              ))}
                          </div>
                        </PopoverContent>
                      </Popover>
                      <div className="flex h-6 shrink-0 items-center rounded-lg border border-[var(--bk-border-soft)] bg-[var(--bk-surface-muted)] px-2.5 font-mono text-[10px] font-bold text-[var(--bk-text-muted)]">
                        {highlightedLiveSession.session.accountId}
                      </div>
                    </div>


                  <div className="rounded-2xl border border-[var(--bk-border-soft)] bg-[var(--bk-surface-muted)] p-5 shadow-inner">
                    <p className="text-[13px] font-bold leading-relaxed text-[var(--bk-text-primary)]">
                       {highlightedLiveSession.health.detail}
                    </p>
                  </div>
                </div>

                <div className="grid grid-cols-2 gap-3">
                    <div className="rounded-xl border-2 border-[color-mix(in_srgb,var(--bk-border)_60%,transparent)] bg-[var(--bk-surface-muted)] p-4 shadow-sm">
                       <div className="flex items-center gap-2 mb-2">
                          <ShieldAlert className="size-3 text-[var(--bk-text-primary)]" />
                          <span className="text-[10px] font-black uppercase tracking-widest text-[var(--bk-text-muted)]">状态恢复</span>
                       </div>
                       <div className="flex flex-col text-[12px] font-black tracking-tight">
                         <span className="text-[var(--bk-status-danger)]">SL: {String((monitorSession?.state as any)?.recoveredStopOrderCount ?? "0")}</span>
                         <span className="text-[var(--bk-status-success)]">PT: {String((monitorSession?.state as any)?.recoveredTakeProfitOrderCount ?? "0")}</span>
                       </div>
                    </div>
                    <div className="rounded-xl border-2 border-[color-mix(in_srgb,var(--bk-border)_60%,transparent)] bg-[var(--bk-surface-muted)] p-4 shadow-sm">
                       <div className="flex items-center gap-2 mb-2">
                          <Compass className="size-3 text-[var(--bk-text-primary)]" />
                          <span className="text-[10px] font-black uppercase tracking-widest text-[var(--bk-text-muted)]">持仓方向</span>
                       </div>
                       <strong className="block text-[15px] font-black text-[var(--bk-status-success)]">{String(monitorExecutionSummary.position?.side ?? "FLAT")}</strong>
                    </div>
                </div>
              </div>
            ) : (
              <div className="flex-1 flex flex-col items-center justify-center text-center opacity-40 italic text-sm">
                载入会话...
              </div>
            )}
          </CardContent>
        </Card>

        {/* 柱 2: 实时行情遥测 */}
        <Card tone="bento" className="lg:col-span-4 rounded-[32px] overflow-hidden border-2 border-[var(--bk-border-strong)] shadow-xl flex flex-col">
          <CardHeader className="flex flex-row items-center justify-between border-b border-[var(--bk-border-soft)] bg-[var(--bk-surface-ghost)] px-8 py-3.5">
            <div className="flex items-center gap-3">
              <Zap className="size-5 text-[var(--bk-text-primary)]" />
              <CardTitle className="text-lg font-black text-[var(--bk-text-primary)]">行情遥测</CardTitle>
            </div>
            <Badge variant="metal" className="text-[var(--bk-text-muted)]">
              {monitorSymbol || "NO SIGNAL"}
            </Badge>
          </CardHeader>
          <CardContent className="p-5 flex-1 flex flex-col justify-center">
            <div className="space-y-6">
              <div className="text-center space-y-1">
                <span className="text-[10px] font-black uppercase tracking-widest text-[var(--bk-text-muted)]">Current Market Price</span>
                <div className="text-5xl font-black tracking-tighter tabular-nums text-[var(--bk-text-primary)]">
                  {formatMaybeNumber(monitorMarket.tradePrice)}
                </div>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div className="rounded-xl border border-[var(--bk-border)] bg-[var(--bk-surface-primary-faint)] p-3 text-center shadow-sm">
                  <span className="block text-[9px] font-black uppercase text-[var(--bk-text-muted)] mb-1">Spread Bid/Ask</span>
                  <strong className="text-[12px] font-mono font-black text-[var(--bk-status-success)]">
                    {formatMaybeNumber(monitorMarket.bestBid)} / {formatMaybeNumber(monitorMarket.bestAsk)}
                  </strong>
                </div>
                <div className="rounded-xl border border-[var(--bk-border)] bg-[var(--bk-surface-primary-faint)] p-3 text-center shadow-sm">
                   <span className="block text-[9px] font-black uppercase text-[var(--bk-text-muted)] mb-1">Technical SMA5</span>
                   <strong className="text-[12px] font-mono font-black text-[var(--bk-text-primary)]">
                     {formatMaybeNumber(monitorSignalState.sma5)}
                   </strong>
                </div>
              </div>

              <p className="text-center text-[11px] font-bold leading-tight text-[var(--bk-text-muted)] italic opacity-70">
                {String(monitorSignalBarDecision.reason ?? "当前无执行信号或正在等待波动...")}
              </p>
            </div>
          </CardContent>
        </Card>

        {/* 柱 3: 平台健康与安全 (集成原底部卡片) */}
        <Card tone="bento" className="lg:col-span-4 rounded-[32px] overflow-hidden border-2 border-[var(--bk-border-strong)] shadow-xl flex flex-col">
          <CardHeader className="flex flex-row items-center justify-between border-b border-[var(--bk-border-soft)] bg-[var(--bk-surface-ghost)] px-8 py-3.5">
            <div className="flex items-center gap-3">
              <HeartPulse className="size-5 text-[var(--bk-text-primary)]" />
              <CardTitle className="text-lg font-black text-[var(--bk-text-primary)]">系统健康概览</CardTitle>
            </div>
            <Badge 
              variant="metal"
              className={cn(
                "px-2 py-0.5",
                monitorHealth?.status === "healthy" 
                  ? "border-[var(--bk-status-success-soft)] text-[var(--bk-status-success)]" 
                  : "border-[var(--bk-status-danger-soft)] text-[var(--bk-status-danger)]"
              )}
            >
               {technicalStatusLabel(monitorHealth?.status ?? "--")}
            </Badge>
          </CardHeader>
          <CardContent className="p-5 flex-1">
            <div className="grid grid-cols-2 gap-3 h-full">
              {[
                { label: "Active Alerts", value: monitorHealth?.alertCounts.total ?? 0, icon: ShieldAlert },
                { label: "Critical Issues", value: monitorHealth?.alertCounts.critical ?? 0, color: 'text-[var(--bk-status-danger)]' },
                { label: "Runtime Quiet", value: runtimePolicyValueLabel(platformRuntimePolicy?.runtimeQuietSeconds) },
                { label: "Eval Cooldown", value: runtimePolicyValueLabel(platformRuntimePolicy?.strategyEvaluationQuietSeconds) },
                { label: "Account Sync", value: runtimePolicyValueLabel(platformRuntimePolicy?.liveAccountSyncFreshnessSeconds) },
                { 
                  label: "Orphaned Orders", 
                  value: orphanedOrders.length > 0 ? `${orphanedOrders.length} ERR` : "None", 
                  color: orphanedOrders.length > 0 ? 'text-[var(--bk-status-danger)]' : 'text-[var(--bk-status-success)]',
                  icon: orphanedOrders.length > 0 ? ShieldAlert : ShieldCheck 
                }
              ].map((item, idx) => (
                <div key={idx} className="rounded-xl border-2 border-[color-mix(in_srgb,var(--bk-border)_40%,transparent)] bg-[var(--bk-surface-muted)] p-4 flex flex-col justify-center transition-all hover:bg-[var(--bk-surface-soft)] shadow-sm">
                  <span className="text-[10px] font-black uppercase tracking-tighter text-[var(--bk-text-muted)] mb-1">{item.label}</span>
                  <strong className={`text-[15px] font-black tabular-nums tracking-tight ${item.color || 'text-[var(--bk-text-primary)]'}`}>
                    {String(item.value ?? "--")}
                  </strong>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>

        {/* 下沉底座: 执行时间线终端 (全宽) */}
        <Card tone="bento" className="lg:col-span-12 rounded-[32px] overflow-hidden border-2 border-[var(--bk-border-strong)] shadow-xl bg-[var(--bk-surface-faint)]">
          <CardHeader className="border-b border-[var(--bk-border-soft)] bg-[color-mix(in_srgb,var(--bk-surface-ghost)_60%,transparent)] px-8 py-4">
             <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <div className="size-2 animate-pulse rounded-full bg-[var(--bk-status-success)]" />
                  <h5 className="text-[10px] font-black uppercase tracking-widest text-[var(--bk-text-primary)]">Execution Timeline Terminal</h5>
                </div>
                <div className="flex items-center gap-3">
                  <Popover>
                    <PopoverTrigger className="flex items-center gap-1.5 rounded-lg border border-[var(--bk-border)] bg-[var(--bk-surface)] px-2 py-1 font-mono text-[9px] font-black uppercase shadow-sm text-[var(--bk-text-muted)] transition-colors hover:bg-[var(--bk-surface-muted)]">
                      <Settings className="size-3" />
                      终端配置
                    </PopoverTrigger>
                    <PopoverContent align="end" className="w-[280px] p-5 rounded-3xl border-2 border-[var(--bk-border)] bg-[var(--bk-surface-overlay-strong)] shadow-2xl isolate z-[70]">
                      <div className="space-y-5">
                         <div className="flex items-center justify-between">
                            <Label className="text-[10px] font-black uppercase tracking-widest text-[var(--bk-text-primary)]">过滤冗余决策</Label>
                            <input 
                              type="checkbox" 
                              checked={timelineConfig.deduplicationEnabled}
                              onChange={(e) => setTimelineConfig({ ...timelineConfig, deduplicationEnabled: e.target.checked })}
                              className="size-4 rounded border-[var(--bk-border)] bg-[var(--bk-surface)] accent-[var(--bk-status-success)] cursor-pointer"
                            />
                         </div>
                         <div className="space-y-2">
                            <Label className="text-[9px] font-black uppercase tracking-tighter text-[var(--bk-text-muted)]">静默触发时间 (秒)</Label>
                            <Input 
                              type="number" 
                              value={timelineConfig.quietSeconds}
                              onChange={(e) => setTimelineConfig({ ...timelineConfig, quietSeconds: Math.max(0, parseInt(e.target.value) || 0) })}
                              className="h-8 text-[11px] font-black rounded-xl border-[var(--bk-border)] bg-[var(--bk-surface-faint)]"
                              placeholder="60"
                            />
                            <p className="text-[10px] text-[var(--bk-text-muted)] leading-tight italic opacity-60">定义静默隔离的时间长度。在此时间内的重复事件将被限制显示频率。</p>
                         </div>

                         <div className="space-y-2">
                            <Label className="text-[9px] font-black uppercase tracking-tighter text-[var(--bk-text-muted)]">窗口内最大显示数</Label>
                            <Input 
                              type="number" 
                              value={timelineConfig.maxRepeats}
                              onChange={(e) => setTimelineConfig({ ...timelineConfig, maxRepeats: Math.max(1, parseInt(e.target.value) || 1) })}
                              className="h-8 text-[11px] font-black rounded-xl border-[var(--bk-border)] bg-[var(--bk-surface-faint)]"
                              placeholder="1"
                            />
                            <p className="text-[10px] text-[var(--bk-text-muted)] leading-tight italic opacity-60">定义在静默周期内允许显示的该类事件的最大总次数（包含首条）。</p>
                         </div>

                      </div>
                    </PopoverContent>
                  </Popover>
                  <div className="rounded-lg border border-[var(--bk-border)] bg-[var(--bk-surface)] px-2 py-1 font-mono text-[9px] font-black uppercase shadow-sm text-[var(--bk-text-muted)]">
                    {liveSessionDetailStatus === "loading"
                      ? "DETAIL_LOADING"
                      : liveSessionDetailStatus === "error"
                        ? "DETAIL_ERROR"
                        : "AUTO_SCROLL_MONITOR"}
                  </div>
                </div>
             </div>
          </CardHeader>

          <CardContent className="p-0">
            <div className="custom-scrollbar h-[260px] overflow-y-auto px-8 py-5 font-mono text-[10px] leading-relaxed">
               {timelineLogs.length > 0 ? timelineLogs.map((line: string, idx: number) => (
                 <div key={idx} className="mb-1 border-l-2 border-[var(--bk-canvas-strong)] pl-4 hover:bg-[color-mix(in_srgb,var(--bk-surface-strong)_40%,transparent)] transition-colors">
                   <span className="mr-4 font-bold tabular-nums text-[var(--bk-text-muted)] opacity-30">[{idx.toString().padStart(2, '0')}]</span>
                   <span className="text-[var(--bk-text-primary)] tracking-tight">{line}</span>
                 </div>
               )) : (
                 <div className="flex h-full items-center justify-center italic text-[var(--bk-text-muted)] opacity-40">
                   Waiting for execution events...
                 </div>
               )}
            </div>
          </CardContent>
        </Card>
      </div>

      <div className="grid grid-cols-1 gap-8">
        {/* 3. 事务审计与管理控制 (下部 Dock) */}
        <Card tone="bento" className="rounded-[32px] overflow-hidden border-2 border-[var(--bk-border-strong)] shadow-xl bg-[var(--bk-surface)]">
          <Tabs defaultValue="orders" value={dockTab} onValueChange={(val) => onDockTabChange(val as any)}>
            <CardHeader className="flex flex-row items-center justify-between border-b border-[var(--bk-border-soft)] bg-[var(--bk-surface-ghost)] px-8 pb-3 pt-6">
              <div className="flex items-center gap-3">
                <Layers className="size-5 text-[var(--bk-text-primary)]" />
                <CardTitle className="text-xl font-black text-[var(--bk-text-primary)]">事务审计与管理控制</CardTitle>
              </div>
              <TabsList variant="bento" className="flex h-10 gap-1 rounded-2xl border border-[var(--bk-border-soft)] bg-[var(--bk-surface-strong)] p-1 shadow-inner">
                <TabsTrigger value="pairs" className="rounded-xl px-4 text-[10px] font-black uppercase">追溯</TabsTrigger>
                <TabsTrigger value="orders" className="rounded-xl px-4 text-[10px] font-black uppercase">订单</TabsTrigger>
                <TabsTrigger value="positions" className="rounded-xl px-4 text-[10px] font-black uppercase">持仓</TabsTrigger>
                <TabsTrigger value="fills" className="rounded-xl px-4 text-[10px] font-black uppercase">成交</TabsTrigger>
                <TabsTrigger value="alerts" className="rounded-xl px-4 text-[10px] font-black uppercase">告警</TabsTrigger>
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
