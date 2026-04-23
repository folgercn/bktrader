import { AccountSummary, AccountRecord, StrategyVersion, StrategyRecord, AccountEquitySnapshot, Order, Fill, Position, PaperSession, LiveSession, ChartCandle, ChartAnnotation, MarkerLegendItem, BacktestRun, BacktestOptions, LiveAdapter, SignalSourceDefinition, SignalSourceCatalog, SignalSourceType, SignalBinding, SignalRuntimeAdapter, SignalRuntimeSession, ReplayReasonStats, ReplaySample, ExecutionTrade, SourceFilter, EventFilter, TimeWindow, MarkerDetail, ChartOverrideRange, SelectedSample, SelectableSample, RuntimeMarketSnapshot, RuntimeSourceSummary, RuntimeReadiness, SignalBarCandle, AlertItem, PlatformAlert, PlatformNotification, TelegramConfig, RuntimePolicy, LivePreflightSummary, LiveNextAction, LiveDispatchPreview, LiveSessionExecutionSummary, LiveSessionHealth, HighlightedLiveSession, LiveSessionFlowStep, SessionMarker, SignalMonitorOverlay, AuthSession, TimelineConfig } from '../types/domain';


import { formatMoney, formatSigned, formatPercent, formatNumber, formatMaybeNumber, formatTime, formatShortTime, shrink } from './format';
import { createChart } from 'lightweight-charts';

export function sampleStatus(sample: ReplaySample) {
  const reason = String(sample.reason ?? "").trim().toLowerCase();
  if (reason === "entry_not_hit" || reason === "entry_missed") {
    return { label: "进场错过", tone: "missed" };
  }
  if (reason === "exit_not_hit" || reason === "exit_missed") {
    return { label: "离场错过", tone: "missed" };
  }
  if (reason.includes("invalid")) {
    return { label: "无效项", tone: "invalid" };
  }
  if (reason.includes("error")) {
    return { label: "错误项", tone: "error" };
  }
  return { label: "已完成", tone: "completed" };
}

export function buildLinePath(values: number[], width: number, height: number) {
  if (values.length === 0) {
    return { line: "", area: "" };
  }

  const min = Math.min(...values);
  const max = Math.max(...values);
  const span = max - min || 1;
  const stepX = values.length === 1 ? width / 2 : width / (values.length - 1);

  const points = values.map((value, index) => {
    const x = stepX * index;
    const y = height - ((value - min) / span) * (height - 16) - 8;
    return `${x.toFixed(2)} ${y.toFixed(2)}`;
  });

  return {
    line: `M ${points.join(" L ")}`,
    area: `M ${points.join(" L ")}`,
  };
}

export function summarizeRange(values: number[]) {
  if (values.length === 0) {
    return { min: 0, max: 0 };
  }
  return {
    min: Math.min(...values),
    max: Math.max(...values),
  };
}

export function summarizeTimeRange(values: string[]) {
  if (values.length === 0) {
    return { label: "暂无数据" };
  }
  const start = new Date(values[0]);
  const end = new Date(values[values.length - 1]);
  return {
    label: `${formatShortTime(start)} - ${formatShortTime(end)}`,
  };
}

export function filterChartAnnotations(
  items: ChartAnnotation[],
  candles: ChartCandle[],
  sessionID?: string,
  sessionSource: SourceFilter = "all",
  sourceFilter: SourceFilter = "all",
  eventFilter: EventFilter = "all"
) {
  if (candles.length === 0) {
    return [];
  }
  const start = new Date(candles[0].time).getTime();
  const end = new Date(candles[candles.length - 1].time).getTime();

  return items.filter((item) => {
    const ts = new Date(item.time).getTime();
    if (Number.isNaN(ts) || ts < start || ts > end) {
      return false;
    }
    if (sourceFilter !== "all" && item.source !== sourceFilter) {
      return false;
    }
    if (item.source === "paper" && item.metadata?.paperSession !== sessionID) {
      return false;
    }
    if (item.source === "live" && sessionSource === "live" && item.metadata?.liveSessionId !== sessionID) {
      return false;
    }
    if (!["paper", "backtest", "live"].includes(item.source) && sourceFilter !== "all") {
      return false;
    }
    if (eventFilter === "all") {
      return item.source === "paper" || item.source === "backtest" || item.source === "live";
    }
    return matchesEventFilter(item, eventFilter);
  });
}

export function matchesEventFilter(item: ChartAnnotation, filter: EventFilter) {
  switch (filter) {
    case "initial":
      return item.type.includes("initial");
    case "reentry":
      return item.type.includes("reentry");
    case "pt":
      return item.type.includes("pt");
    case "sl":
      return item.type.includes("sl");
    default:
      return true;
  }
}

export function resolveChartAnchor(session?: PaperSession | LiveSession | null, orders: Order[] = []) {
  const state = getRecord(session?.state);
  for (const key of ["lastSignalRuntimeEventAt", "lastSyncedAt", "lastDispatchedAt", "lastLedgerTime"]) {
    if (typeof state[key] === "string" && state[key]) {
      return new Date(String(state[key]));
    }
  }

  const latestReplayOrder = orders
    .slice()
    .reverse()
    .find((item) => typeof item.metadata?.eventTime === "string");
  if (latestReplayOrder && typeof latestReplayOrder.metadata?.eventTime === "string") {
    return new Date(latestReplayOrder.metadata.eventTime);
  }

  return new Date();
}

export function buildTimeRange(anchorDate: Date, window: TimeWindow) {
  const anchor = Math.floor(anchorDate.getTime() / 1000);
  const beforeByWindow: Record<TimeWindow, number> = {
    "6h": 6 * 60 * 60,
    "12h": 12 * 60 * 60,
    "1d": 24 * 60 * 60,
    "3d": 3 * 24 * 60 * 60,
  };
  const afterByWindow: Record<TimeWindow, number> = {
    "6h": 60 * 60,
    "12h": 2 * 60 * 60,
    "1d": 4 * 60 * 60,
    "3d": 8 * 60 * 60,
  };
  return {
    from: anchor - beforeByWindow[window],
    to: anchor + afterByWindow[window],
  };
}

export function buildSampleRange(sample: ReplaySample): ChartOverrideRange | null {
  const entryTime = Date.parse(String(sample.entryTime ?? ""));
  const exitTime = Date.parse(String(sample.exitTime ?? sample.bracketExitTime ?? ""));
  if (!Number.isFinite(entryTime)) {
    return null;
  }
  const end = Number.isFinite(exitTime) ? exitTime : entryTime + 60 * 60 * 1000;
  return {
    from: Math.floor((entryTime - 30 * 60 * 1000) / 1000),
    to: Math.floor((end + 30 * 60 * 1000) / 1000),
    label: "Sample Window",
  };
}

export function buildSampleKey(prefix: string, index: number, sample: ReplaySample) {
  return [
    prefix,
    index,
    String(sample.entryTime ?? ""),
    String(sample.exitTime ?? sample.bracketExitTime ?? ""),
    String(sample.entryCause ?? sample.reason ?? ""),
  ].join(":");
}

export function annotationMatchesSample(item: ChartAnnotation, sample: ReplaySample) {
  if (item.source !== "backtest") {
    return false;
  }

  const annotationTime = Date.parse(item.time);
  if (!Number.isFinite(annotationTime)) {
    return false;
  }

  const entryTime = Date.parse(String(sample.entryTime ?? ""));
  const exitTime = Date.parse(String(sample.exitTime ?? sample.bracketExitTime ?? ""));
  const reason = String(sample.entryCause ?? sample.exitCause ?? sample.reason ?? "").trim().toUpperCase();
  const annotationReason = String(item.metadata?.reason ?? item.label ?? "")
    .trim()
    .toUpperCase();
  const sameReason = reason === "" || annotationReason === reason;

  if (Number.isFinite(entryTime) && Math.abs(annotationTime - entryTime) <= 60 * 1000 && item.type.includes("entry")) {
    return sameReason;
  }
  if (Number.isFinite(exitTime) && Math.abs(annotationTime - exitTime) <= 60 * 1000 && item.type.includes("exit")) {
    return true;
  }
  return false;
}

export function findNearestAnnotation(items: ChartAnnotation[], hoveredSeconds: number) {
  let nearest: ChartAnnotation | null = null;
  let bestDelta = Number.POSITIVE_INFINITY;
  for (const item of items) {
    const itemSeconds = Math.floor(new Date(item.time).getTime() / 1000);
    const delta = Math.abs(itemSeconds - hoveredSeconds);
    if (delta < bestDelta) {
      bestDelta = delta;
      nearest = item;
    }
  }
  if (bestDelta > 45 * 60) {
    return null;
  }
  return nearest;
}

export function toMarkerDetail(item: ChartAnnotation): MarkerDetail {
  return {
    id: item.id,
    source: item.source,
    type: item.type,
    label: item.label,
    time: item.time,
    price: item.price,
    reason: typeof item.metadata?.reason === "string" ? item.metadata.reason : undefined,
    paperSession: typeof item.metadata?.paperSession === "string" ? item.metadata.paperSession : undefined,
    liveSession: typeof item.metadata?.liveSessionId === "string" ? item.metadata.liveSessionId : undefined,
  };
}

export function markerShape(type: string) {
  if (type.includes("initial")) {
    return "square";
  }
  if (type.includes("pt-reentry") || type.includes("sl-reentry") || type.includes("entry-long")) {
    return "arrowUp";
  }
  if (type.includes("entry-short")) {
    return "arrowDown";
  }
  if (type.includes("exit")) {
    return "circle";
  }
  if (type.includes("buy")) {
    return "arrowUp";
  }
  if (type.includes("sell")) {
    return "arrowDown";
  }
  return "circle";
}

export function markerPosition(type: string) {
  if (type.includes("entry") || type.includes("buy")) {
    return "belowBar";
  }
  return "aboveBar";
}

export function markerColor(item: ChartAnnotation, highlighted = false) {
  if (highlighted) {
    return "#f0b429";
  }
  if (item.source === "paper") {
    if (item.type.includes("exit-sl")) {
      return "#7d5877";
    }
    if (item.type.includes("exit-pt")) {
      return "#284d86";
    }
    return "#284d86";
  }
  if (item.source === "live") {
    if (item.type.includes("exit-sl")) {
      return "#b04a37";
    }
    if (item.type.includes("exit-pt")) {
      return "#c58b2d";
    }
    if (item.type.includes("entry")) {
      return "#0e6d60";
    }
  }
  if (item.type.includes("initial")) {
    return "#7a8791";
  }
  if (item.type.includes("pt-reentry")) {
    return "#0e6d60";
  }
  if (item.type.includes("sl-reentry")) {
    return "#1f8f7d";
  }
  if (item.type.includes("exit-pt")) {
    return "#c58b2d";
  }
  if (item.type.includes("exit-sl")) {
    return "#b04a37";
  }
  return "#5d6971";
}

export function markerText(item: ChartAnnotation, highlighted = false) {
  return highlighted ? `★ ${item.label}` : item.label;
}

export function annotationTone(item: ChartAnnotation) {
  if (item.source === "paper") {
    return "paper";
  }
  if (item.type.includes("initial")) {
    return "initial";
  }
  if (item.type.includes("pt-reentry")) {
    return "pt";
  }
  if (item.type.includes("sl-reentry")) {
    return "sl";
  }
  if (item.type.includes("exit-pt")) {
    return "pt";
  }
  if (item.type.includes("exit-sl")) {
    return "sl";
  }
  return "中性";
}

export function paperAccountsFromSummaries(items: AccountSummary[]) {
  return items.filter((item) => item.mode === "PAPER");
}

export function strategyLabel(strategy: Partial<StrategyRecord> | null | undefined) {
  if (!strategy) {
    return "--";
  }
  const name = String(strategy.name ?? "").trim();
  const id = String(strategy.id ?? "").trim();
  const version = String(strategy.currentVersion?.version ?? "").trim();
  if (name && version) {
    return `${name} · ${version}`;
  }
  if (name) {
    return name;
  }
  if (id && version) {
    return `${id} · ${version}`;
  }
  if (id) {
    return id;
  }
  return "未命名策略";
}

export function getNumber(value: unknown) {
  if (typeof value === "number") {
    return value;
  }
  if (typeof value === "string") {
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : undefined;
  }
  return undefined;
}

export function getRecord(value: unknown): Record<string, unknown> {
  if (value && typeof value === "object" && !Array.isArray(value)) {
    return value as Record<string, unknown>;
  }
  return {};
}

export function getList(value: unknown): Array<Record<string, unknown>> {
  if (!Array.isArray(value)) {
    return [];
  }
  return value.filter((item): item is Record<string, unknown> => !!item && typeof item === "object" && !Array.isArray(item));
}

export function deriveRuntimeMarketSnapshot(
  sourceStates: Record<string, unknown>,
  summary: Record<string, unknown>,
  targetSymbol?: string
): RuntimeMarketSnapshot {
  const snapshot: RuntimeMarketSnapshot = {};
  const normalizedTarget = (targetSymbol ?? "").trim().toUpperCase();
  const states = Object.values(sourceStates)
    .map((value) => getRecord(value))
    .filter((state) => {
      if (!normalizedTarget) return true;
      const stateSymbol = String(state.symbol ?? "").trim().toUpperCase();
      // If state doesn't have a symbol, we might still want to include it if it's a global source,
      // but usually signal sources have symbols.
      return stateSymbol === "" || stateSymbol === normalizedTarget;
    });

  for (const state of states) {
    if (snapshot.tradePrice == null) {
      snapshot.tradePrice = getNumber(state.price);
    }
    if (snapshot.bestBid == null) {
      snapshot.bestBid = getNumber(state.bestBid);
    }
    if (snapshot.bestAsk == null) {
      snapshot.bestAsk = getNumber(state.bestAsk);
    }
  }

  snapshot.tradePrice ??= getNumber(summary.price);
  snapshot.bestBid ??= getNumber(summary.bestBid);
  snapshot.bestAsk ??= getNumber(summary.bestAsk);

  if (snapshot.bestBid != null && snapshot.bestAsk != null && snapshot.bestBid > 0 && snapshot.bestAsk >= snapshot.bestBid) {
    const mid = (snapshot.bestBid + snapshot.bestAsk) / 2;
    if (mid > 0) {
      snapshot.spreadBps = ((snapshot.bestAsk - snapshot.bestBid) / mid) * 10000;
    }
  }

  return snapshot;
}

export function deriveRuntimeSourceSummary(
  sourceStates: Record<string, unknown>,
  policy: RuntimePolicy | null,
  targetSymbol?: string
): RuntimeSourceSummary {
  const normalizedTarget = (targetSymbol ?? "").trim().toUpperCase();
  const states = Object.values(sourceStates)
    .map((value) => getRecord(value))
    .filter((state) => {
      if (!normalizedTarget) return true;
      const stateSymbol = String(state.symbol ?? "").trim().toUpperCase();
      return stateSymbol === "" || stateSymbol === normalizedTarget;
    });
  const now = Date.now();
  let tradeTickCount = 0;
  let orderBookCount = 0;
  let staleCount = 0;
  let latestEventAt = "";

  for (const state of states) {
    const streamType = String(state.streamType ?? "").trim().toLowerCase();
    const freshnessSeconds =
      streamType === "trade_tick"
        ? policy?.tradeTickFreshnessSeconds ?? 15
        : streamType === "order_book"
          ? policy?.orderBookFreshnessSeconds ?? 10
          : streamType === "signal_bar"
            ? policy?.signalBarFreshnessSeconds ?? 30
            : policy?.runtimeQuietSeconds ?? 30;
    if (streamType === "trade_tick") {
      tradeTickCount += 1;
    }
    if (streamType === "order_book") {
      orderBookCount += 1;
    }
    const lastEventAt = String(state.lastEventAt ?? "");
    const parsed = Date.parse(lastEventAt);
    if (Number.isFinite(parsed)) {
      if (latestEventAt === "" || parsed > Date.parse(latestEventAt)) {
        latestEventAt = lastEventAt;
      }
      if (now-parsed > freshnessSeconds*1000) {
        staleCount += 1;
      }
    }
  }

  return {
    tradeTickCount,
    orderBookCount,
    staleCount,
    latestEventAt: latestEventAt || undefined,
  };
}

export function deriveRuntimeReadiness(
  runtimeState: Record<string, unknown>,
  sourceSummary: RuntimeSourceSummary,
  requirements: { requireTick: boolean; requireOrderBook: boolean }
): RuntimeReadiness {
  const health = String(runtimeState.health ?? "").trim().toLowerCase();
  if (!runtimeState || Object.keys(runtimeState).length === 0) {
    return { ready: false, status: "blocked", reason: "环境未就绪" };
  }
  if (health !== "" && health !== "healthy") {
    return { ready: false, status: "blocked", reason: `运行时-${health}` };
  }
  if (requirements.requireTick && sourceSummary.tradeTickCount <= 0) {
    return { ready: false, status: "blocked", reason: "缺失成交数据" };
  }
  if (requirements.requireOrderBook && sourceSummary.orderBookCount <= 0) {
    return { ready: false, status: "blocked", reason: "缺失盘口数据" };
  }
  if (sourceSummary.staleCount > 0) {
    return { ready: false, status: "warning", reason: "数据源陈旧" };
  }
  return { ready: true, status: "ready", reason: "健康" };
}

type SignalBarSelectionOptions = {
  fallbackStates?: Record<string, unknown>;
  targetSymbol?: string;
  targetTimeframe?: string;
  targetStateKey?: string;
};

function normalizeSignalSymbol(value: unknown) {
  return String(value ?? "").trim().toUpperCase();
}

function normalizeSignalTimeframe(value: unknown) {
  return String(value ?? "").trim().toLowerCase();
}

function resolveSignalBarEntry(
  signalBarStates: Record<string, unknown>,
  options?: Omit<SignalBarSelectionOptions, "fallbackStates">
) {
  const targetStateKey = String(options?.targetStateKey ?? "").trim();
  const targetSymbol = normalizeSignalSymbol(options?.targetSymbol);
  const targetTimeframe = normalizeSignalTimeframe(options?.targetTimeframe);

  if (targetStateKey) {
    const exact = getRecord(signalBarStates[targetStateKey]);
    if (Object.keys(exact).length > 0) {
      return exact;
    }
  }

  for (const value of Object.values(signalBarStates)) {
    const entry = getRecord(value);
    if (Object.keys(entry).length === 0) {
      continue;
    }
    const entrySymbol = normalizeSignalSymbol(entry.symbol ?? getRecord(entry.current).symbol);
    const entryTimeframe = normalizeSignalTimeframe(entry.timeframe ?? getRecord(entry.current).timeframe);
    if (targetSymbol && entrySymbol && entrySymbol !== targetSymbol) {
      continue;
    }
    if (targetTimeframe && entryTimeframe && entryTimeframe !== targetTimeframe) {
      continue;
    }
    return entry;
  }

  return {};
}

export function deriveSignalBarCandles(
  sourceStates: Record<string, unknown>,
  options?: Omit<SignalBarSelectionOptions, "fallbackStates">
): SignalBarCandle[] {
  const candles: SignalBarCandle[] = [];
  const targetStateKey = String(options?.targetStateKey ?? "").trim();
  const targetSymbol = normalizeSignalSymbol(options?.targetSymbol);
  const targetTimeframe = normalizeSignalTimeframe(options?.targetTimeframe);
  const candidateEntries = targetStateKey
    ? Object.entries(sourceStates).filter(([key]) => key === targetStateKey)
    : Object.entries(sourceStates);

  for (const [, value] of candidateEntries) {
    const state = getRecord(value);
    if (String(state.streamType ?? "") !== "signal_bar") {
      continue;
    }
    if (!targetStateKey) {
      const stateSymbol = normalizeSignalSymbol(state.symbol);
      const stateTimeframe = normalizeSignalTimeframe(state.timeframe);
      if (targetSymbol && stateSymbol && stateSymbol !== targetSymbol) {
        continue;
      }
      if (targetTimeframe && stateTimeframe && stateTimeframe !== targetTimeframe) {
        continue;
      }
    }
    const bars = Array.isArray(state.bars) ? (state.bars as Array<Record<string, unknown>>) : [];
    for (const bar of bars) {
      const barSymbol = normalizeSignalSymbol(bar.symbol);
      const barTimeframe = normalizeSignalTimeframe(bar.timeframe ?? state.timeframe);
      if (targetSymbol && barSymbol && barSymbol !== targetSymbol) {
        continue;
      }
      if (targetTimeframe && barTimeframe && barTimeframe !== targetTimeframe) {
        continue;
      }
      const barStart = String(bar.barStart ?? "");
      const parsed = Number(barStart);
      const time = Number.isFinite(parsed) && parsed > 0 ? new Date(parsed).toISOString() : "";
      const open = getNumber(bar.open);
      const high = getNumber(bar.high);
      const low = getNumber(bar.low);
      const close = getNumber(bar.close);
      if (!time || open == null || high == null || low == null || close == null) {
        continue;
      }
      candles.push({
        time,
        open,
        high,
        low,
        close,
        timeframe: String(bar.timeframe ?? state.timeframe ?? "--"),
        isClosed: Boolean(bar.isClosed),
      });
    }
  }
  if (candles.length === 0 && targetStateKey) {
    return deriveSignalBarCandles(sourceStates, {
      targetSymbol: options?.targetSymbol,
      targetTimeframe: options?.targetTimeframe,
    });
  }
  candles.sort((a, b) => Date.parse(a.time) - Date.parse(b.time));
  return candles.slice(-120);
}

export function mapChartCandlesToSignalBarCandles(candles: ChartCandle[], timeframe: string): SignalBarCandle[] {
  return candles.map((item) => ({
    time: item.time,
    open: item.open,
    high: item.high,
    low: item.low,
    close: item.close,
    timeframe,
    isClosed: true,
  }));
}

export function applyDefaultChartWindow(chart: ReturnType<typeof createChart>, candleCount: number, preferredBars: number) {
  if (candleCount <= 0) {
    return;
  }
  chart.timeScale().fitContent();
  const visibleBars = Math.min(Math.max(preferredBars, 24), candleCount);
  const to = candleCount + 5;
  const from = Math.max(0, to-visibleBars);
  chart.timeScale().setVisibleLogicalRange({ from, to });
}

export function derivePrimarySignalBarState(signalBarStates: Record<string, unknown>, options?: SignalBarSelectionOptions) {
  const primary = resolveSignalBarEntry(signalBarStates, options);
  if (Object.keys(primary).length > 0) {
    return primary;
  }
  return resolveSignalBarEntry(getRecord(options?.fallbackStates), options);
}

export function buildRuntimeEventNotes(summary: Record<string, unknown>) {
  if (Object.keys(summary).length === 0) {
    return ["event: --"];
  }
  const notes: string[] = [];
  const event = String(summary.event ?? summary.type ?? "--");
  notes.push(`event: ${event}`);
  if (summary.symbol != null) {
    notes.push(`symbol: ${String(summary.symbol)}`);
  }
  if (summary.price != null) {
    notes.push(`price: ${formatMaybeNumber(summary.price)}`);
  }
  if (summary.bestBid != null || summary.bestAsk != null) {
    notes.push(`book: ${formatMaybeNumber(summary.bestBid)} / ${formatMaybeNumber(summary.bestAsk)}`);
  }
  return notes.slice(0, 3);
}

export function buildSourceStateNotes(sourceStates: Record<string, unknown>) {
  const entries = Object.entries(sourceStates).slice(0, 2);
  if (entries.length === 0) {
    return ["source-state: --"];
  }
  return entries.map(([key, value]) => {
    const state = getRecord(value);
    return [
      key,
      String(state.streamType ?? "--"),
      String(state.role ?? "--"),
      formatMaybeNumber(state.price ?? state.bestAsk ?? state.bestBid),
    ].join(" · ");
  });
}

export function buildSignalBarDecisionNotes(signalBarDecision: Record<string, unknown>, signalBarState: Record<string, unknown>) {
  if (Object.keys(signalBarDecision).length === 0 && Object.keys(signalBarState).length === 0) {
    return ["signal-bar: --"];
  }
  const current = getRecord(signalBarDecision.current);
  const prevBar1 = getRecord(signalBarDecision.prevBar1);
  const prevBar2 = getRecord(signalBarDecision.prevBar2);
  const timeframe = String(signalBarDecision.timeframe ?? signalBarState.timeframe ?? "--");
  const usesLegacyFallback = signalBarDecision.usedLegacyMA20Fallback === true;
  const filterLabel =
    timeframe === "1d"
      ? `sma5 ${formatMaybeNumber(signalBarDecision.sma5)} · early-long=${boolLabel(signalBarDecision.longEarlyReversalReady)} · early-short=${boolLabel(signalBarDecision.shortEarlyReversalReady)}`
      : usesLegacyFallback
        ? `ma20 ${formatMaybeNumber(signalBarDecision.ma20)} · legacy fallback`
        : `sma5 ${formatMaybeNumber(signalBarDecision.sma5)}`;
  return [
    `signal-bar: ${String(signalBarDecision.reason ?? "--")} · longReady=${boolLabel(signalBarDecision.longReady)} · shortReady=${boolLabel(signalBarDecision.shortReady)}`,
    `filter: tf=${timeframe} · ${filterLabel}`,
    `current: ${formatMaybeNumber(current.open)} / ${formatMaybeNumber(current.high)} / ${formatMaybeNumber(current.low)} / ${formatMaybeNumber(current.close)}`,
    `t-1: ${formatMaybeNumber(prevBar1.open)} / ${formatMaybeNumber(prevBar1.high)} / ${formatMaybeNumber(prevBar1.low)} / ${formatMaybeNumber(prevBar1.close)}`,
    `t-2: ${formatMaybeNumber(prevBar2.open)} / ${formatMaybeNumber(prevBar2.high)} / ${formatMaybeNumber(prevBar2.low)} / ${formatMaybeNumber(prevBar2.close)}`,
  ];
}

export function buildSignalBarStateNotes(signalBarState: Record<string, unknown>) {
  if (Object.keys(signalBarState).length === 0) {
    return ["signal-state: --"];
  }
  const current = getRecord(signalBarState.current);
  const prevBar1 = getRecord(signalBarState.prevBar1);
  const prevBar2 = getRecord(signalBarState.prevBar2);
  return [
    `signal-state: tf=${String(signalBarState.timeframe ?? "--")} · bars=${String(signalBarState.barCount ?? "--")}`,
    `current: ${formatMaybeNumber(current.open)} / ${formatMaybeNumber(current.high)} / ${formatMaybeNumber(current.low)} / ${formatMaybeNumber(current.close)}`,
    `t-1: ${formatMaybeNumber(prevBar1.open)} / ${formatMaybeNumber(prevBar1.high)} / ${formatMaybeNumber(prevBar1.low)} / ${formatMaybeNumber(prevBar1.close)}`,
    `t-2: ${formatMaybeNumber(prevBar2.open)} / ${formatMaybeNumber(prevBar2.high)} / ${formatMaybeNumber(prevBar2.low)} / ${formatMaybeNumber(prevBar2.close)}`,
  ];
}

export function deriveSignalActionSummary(signalBarState: Record<string, unknown>) {
  const current = getRecord(signalBarState.current);
  const prevBar1 = getRecord(signalBarState.prevBar1);
  const prevBar2 = getRecord(signalBarState.prevBar2);
  const close = getNumber(current.close);
  const timeframe = String(signalBarState.timeframe ?? "").trim().toLowerCase();
  const sma5 = getNumber(signalBarState.sma5);
  const ma20 = getNumber(signalBarState.ma20);
  const atr14 = getNumber(signalBarState.atr14);
  const prevHigh1 = getNumber(prevBar1.high);
  const prevHigh2 = getNumber(prevBar2.high);
  const prevLow1 = getNumber(prevBar1.low);
  const prevLow2 = getNumber(prevBar2.low);
  if (close == null || prevHigh1 == null || prevHigh2 == null || prevLow1 == null || prevLow2 == null) {
    return { bias: "中性", state: "等待中", reason: "信号 K 线不足" };
  }
  const longBreakoutShape = prevHigh2 > prevHigh1;
  const shortBreakoutShape = prevLow2 < prevLow1;
  let longReady = false;
  let shortReady = false;
  let longReason = "";
  let shortReason = "";
  if (timeframe === "1d" && sma5 != null && atr14 != null) {
    const earlyBand = 0.06 * atr14;
    const longHard = close > sma5;
    const shortHard = close < sma5;
    const longEarly = close >= sma5 - earlyBand && longBreakoutShape && prevLow1 >= prevLow2;
    const shortEarly = close <= sma5 + earlyBand && shortBreakoutShape && prevHigh1 <= prevHigh2;
    longReady = longHard || longEarly;
    shortReady = shortHard || shortEarly;
    longReason = longHard ? "收盘>sma5" : longEarly ? "早期反转触发" : "1d 做多过滤阻断";
    shortReason = shortHard ? "收盘<sma5" : shortEarly ? "早期反转触发" : "1d 做空过滤阻断";
  } else if (sma5 != null) {
    longReady = close > sma5 && longBreakoutShape;
    shortReady = close < sma5 && shortBreakoutShape;
    longReason = longReady ? "收盘>sma5且高点突破" : "趋势/形态未就绪";
    shortReason = shortReady ? "收盘<sma5且低点突破" : "趋势/形态未就绪";
  } else if (ma20 != null) {
    longReady = close > ma20 && longBreakoutShape;
    shortReady = close < ma20 && shortBreakoutShape;
    longReason = longReady ? "收盘>ma20且高点突破（legacy fallback）" : "趋势/形态未就绪";
    shortReason = shortReady ? "收盘<ma20且低点突破（legacy fallback）" : "趋势/形态未就绪";
  } else {
    return { bias: "中性", state: "等待中", reason: "信号 K 线不足" };
  }
  if (longReady && !shortReady) {
    return { bias: "看多", state: "就绪", reason: longReason };
  }
  if (shortReady && !longReady) {
    return { bias: "看空", state: "就绪", reason: shortReason };
  }
  if (timeframe === "1d" && sma5 != null) {
    if (close > sma5) {
      return { bias: "看多", state: "观察中", reason: "位于 sma5 上方，突破未就绪" };
    }
    if (close < sma5) {
      return { bias: "看空", state: "观察中", reason: "位于 sma5 下方，突破未就绪" };
    }
    return { bias: "中性", state: "观察中", reason: "收盘于 sma5 附近" };
  }
  if (sma5 != null && close > sma5) {
    return { bias: "看多", state: "观察中", reason: "位于 sma5 上方，形态未就绪" };
  }
  if (sma5 != null && close < sma5) {
    return { bias: "看空", state: "观察中", reason: "位于 sma5 下方，形态未就绪" };
  }
  if (ma20 != null && close > ma20) {
    return { bias: "看多", state: "观察中", reason: "位于 ma20 上方，形态未就绪（legacy fallback）" };
  }
  if (ma20 != null && close < ma20) {
    return { bias: "看空", state: "观察中", reason: "位于 ma20 下方，形态未就绪（legacy fallback）" };
  }
  return { bias: "中性", state: "观察中", reason: "收盘于均线附近" };
}

export function deriveLivePreflightSummary(
  account: AccountRecord,
  bindings: SignalBinding[],
  runtimeSessionsForAccount: SignalRuntimeSession[],
  activeRuntime: SignalRuntimeSession | null,
  readiness: RuntimeReadiness
): LivePreflightSummary {
  if (account.status !== "CONFIGURED" && account.status !== "READY") {
    return {
      status: "blocked",
      reason: "账户未配置",
      detail: `账户状态为 ${account.status}`,
    };
  }
  if (bindings.length === 0) {
    return {
      status: "blocked",
      reason: "缺失信号绑定",
      detail: "在提交实盘前请先绑定所需的信号源",
    };
  }
  if (runtimeSessionsForAccount.length === 0) {
    return {
      status: "blocked",
      reason: "无运行时会话",
      detail: "请先创建并启动信号运行时会话",
    };
  }
  if (runtimeSessionsForAccount.length > 1) {
    return {
      status: "watch",
      reason: "需指定策略版本",
      detail: "检测到多个关联会话；实盘订单应明确指定 strategyVersionId",
    };
  }
  if (!activeRuntime) {
    return {
      status: "blocked",
      reason: "无活跃运行时",
      detail: "当前实盘账户暂无可用运行时会话",
    };
  }
  if (activeRuntime.status !== "RUNNING") {
    return {
      status: "blocked",
      reason: "运行时未启动",
      detail: `运行时状态为 ${activeRuntime.status}`,
    };
  }
  if (readiness.status === "blocked") {
    return {
      status: "blocked",
      reason: readiness.reason,
      detail: "运行时预检将拒绝实盘提交",
    };
  }
  if (readiness.status === "warning") {
    return {
      status: "watch",
      reason: readiness.reason,
      detail: "运行时性能下降；实盘指令可能很快会被阻断",
    };
  }
  return {
    status: "ready",
    reason: "运行时已就绪",
    detail: "实盘运行环境预检通过",
  };
}

export function deriveLiveDispatchPreview(
  session: LiveSession | null,
  account: AccountRecord | null,
  bindings: SignalBinding[],
  runtimeSessionsForAccount: SignalRuntimeSession[],
  activeRuntime: SignalRuntimeSession | null,
  readiness: RuntimeReadiness,
  intent: Record<string, unknown>
): LiveDispatchPreview {
  const payload: Record<string, unknown> = {
    symbol: intent.symbol ?? session?.state?.symbol ?? "--",
    side: intent.side ?? "--",
    type: intent.type ?? "--",
    quantity: intent.quantity ?? session?.state?.defaultOrderQuantity ?? undefined,
    price: intent.priceHint ?? undefined,
    priceSource: intent.priceSource ?? "--",
    signalKind: intent.signalKind ?? "--",
    spreadBps: intent.spreadBps ?? undefined,
    liquidityBias: intent.liquidityBias ?? "--",
  };
  if (!session) {
    return {
      status: "blocked",
      reason: "会话缺失",
      detail: "请先创建一个实盘会话",
      payload,
    };
  }
  if (!account) {
    return {
      status: "blocked",
      reason: "账户缺失",
      detail: "关联的实盘账户不存在",
      payload,
    };
  }
  const preflight = deriveLivePreflightSummary(account, bindings, runtimeSessionsForAccount, activeRuntime, readiness);
  if (preflight.status === "blocked") {
    return {
      status: "blocked",
      reason: preflight.reason,
      detail: preflight.detail,
      payload,
    };
  }
  if (!intent.action) {
    return {
      status: "watch",
      reason: "无信号意图",
      detail: "正在等待策略计算产生实盘信号意图",
      payload,
    };
  }
  const dispatchMode = String(session.state?.dispatchMode ?? "");
  if (dispatchMode === "auto-dispatch") {
    return {
      status: preflight.status === "watch" ? "watch" : "ready",
      reason: preflight.status === "watch" ? "预检警告" : "自动分发已就绪",
      detail:
        preflight.status === "watch"
          ? `自动分发已开启，但运行时仍有警告: ${preflight.reason}`
          : "自动分发已开启，下一个就绪信号将自动提交",
      payload,
    };
  }
  return {
    status: preflight.status === "watch" ? "watch" : "ready",
    reason: preflight.status === "watch" ? "预检警告" : "分发就绪",
    detail:
      preflight.status === "watch"
        ? `允许分发，但运行时仍有警告: ${preflight.reason}`
        : "信号意图、运行时和实盘预检均满足条件",
    payload,
  };
}

export function deriveLiveSessionExecutionSummary(
  session: LiveSession | null,
  orders: Order[],
  fills: Fill[],
  positions: Position[]
): LiveSessionExecutionSummary {
  if (!session) {
    return {
      orderCount: 0,
      fillCount: 0,
      latestOrder: null,
      latestFill: null,
      position: null,
    };
  }
  const sessionOrders = orders
    .filter((order) => String(order.metadata?.liveSessionId ?? "") === session.id)
    .sort((left, right) => new Date(right.createdAt).getTime() - new Date(left.createdAt).getTime());
  const sessionOrderIds = new Set(sessionOrders.map((order) => order.id));
  const sessionFills = fills
    .filter((fill) => sessionOrderIds.has(fill.orderId))
    .sort((left, right) => new Date(right.createdAt).getTime() - new Date(left.createdAt).getTime());
  const symbol = String(
    sessionOrders[0]?.symbol ??
      session.state?.symbol ??
      getRecord(session.state?.lastStrategyIntent).symbol ??
      ""
  );
  const position =
    positions.find((item) => item.accountId === session.accountId && item.symbol === symbol) ??
    null;
  return {
    orderCount: sessionOrders.length,
    fillCount: sessionFills.length,
    latestOrder: sessionOrders[0] ?? null,
    latestFill: sessionFills[0] ?? null,
    position,
  };
}

export function derivePaperSessionExecutionSummary(
  session: PaperSession | null,
  orders: Order[],
  fills: Fill[],
  positions: Position[]
): LiveSessionExecutionSummary {
  if (!session) {
    return {
      orderCount: 0,
      fillCount: 0,
      latestOrder: null,
      latestFill: null,
      position: null,
    };
  }
  const sessionOrders = orders
    .filter((order) => String(order.metadata?.paperSession ?? "") === session.id)
    .sort((left, right) => new Date(right.createdAt).getTime() - new Date(left.createdAt).getTime());
  const sessionOrderIds = new Set(sessionOrders.map((order) => order.id));
  const sessionFills = fills
    .filter((fill) => sessionOrderIds.has(fill.orderId))
    .sort((left, right) => new Date(right.createdAt).getTime() - new Date(left.createdAt).getTime());
  const symbol = String(
    sessionOrders[0]?.symbol ??
      session.state?.symbol ??
      ""
  );
  const position =
    positions.find((item) => item.accountId === session.accountId && item.symbol === symbol) ??
    null;
  return {
    orderCount: sessionOrders.length,
    fillCount: sessionFills.length,
    latestOrder: sessionOrders[0] ?? null,
    latestFill: sessionFills[0] ?? null,
    position,
  };
}

export function deriveSessionMarkers(session: LiveSession | PaperSession | null, orders: Order[], fills: Fill[]): SessionMarker[] {
  if (!session) {
    return [];
  }
  const isLiveSession = "strategyId" in session && !("startEquity" in session);
  const sessionOrders = orders
    .filter((order) =>
      isLiveSession
        ? String(order.metadata?.liveSessionId ?? "") === session.id
        : String(order.metadata?.paperSession ?? "") === session.id
    )
    .sort((left, right) => Date.parse(left.createdAt) - Date.parse(right.createdAt));
  const fillByOrderId = new Map(fills.map((fill) => [fill.orderId, fill] as const));

  return sessionOrders.slice(-24).map((order) => {
    const fill = fillByOrderId.get(order.id);
    const side = String(order.side ?? "").toUpperCase();
    const isBuy = side === "BUY";
    return {
      time: fill?.createdAt || order.createdAt,
      position: isBuy ? "belowBar" : "aboveBar",
      color: isBuy ? "#0e6d60" : "#b04a37",
      shape: isBuy ? "arrowUp" : "arrowDown",
      text: `${isBuy ? "开" : "平"} ${formatMaybeNumber(order.price)}`,
    };
  });
}

function resolveSessionOrders(session: LiveSession | PaperSession | null, orders: Order[]) {
  if (!session) {
    return [];
  }
  const isLiveSession = "strategyId" in session && !("startEquity" in session);
  return orders
    .filter((order) =>
      isLiveSession
        ? String(order.metadata?.liveSessionId ?? "") === session.id
        : String(order.metadata?.paperSession ?? "") === session.id
    )
    .sort((left, right) => Date.parse(left.createdAt) - Date.parse(right.createdAt));
}

function clampAnnotationTime(time: string, visibleStart: string, visibleEnd: string) {
  const parsed = Date.parse(time);
  const start = Date.parse(visibleStart);
  const end = Date.parse(visibleEnd);
  if (!Number.isFinite(parsed) || !Number.isFinite(start) || !Number.isFinite(end)) {
    return "";
  }
  if (parsed <= start) {
    return visibleStart;
  }
  if (parsed >= end) {
    return visibleEnd;
  }
  return new Date(parsed).toISOString();
}

export function deriveSignalMonitorDecorations(
  session: LiveSession | PaperSession | null,
  candles: SignalBarCandle[],
  position: Position | null,
  orders: Order[],
  fills: Fill[]
): { markers: SessionMarker[]; overlays: SignalMonitorOverlay[] } {
  if (!session || candles.length < 3) {
    return { markers: [], overlays: [] };
  }

  const markers: SessionMarker[] = [];
  const overlays: SignalMonitorOverlay[] = [];
  const visibleStart = candles[0]?.time ?? "";
  const visibleEnd = candles[candles.length - 1]?.time ?? "";
  const state = getRecord(session.state);

  const breakoutEntries = getList(state.breakoutHistory);
  if (breakoutEntries.length === 0 && Object.keys(getRecord(state.lastBreakoutSignal)).length > 0) {
    breakoutEntries.push(getRecord(state.lastBreakoutSignal));
  }

  for (const rawEntry of breakoutEntries.slice(-12)) {
    const breakout = getRecord(rawEntry);
    const breakoutTime = clampAnnotationTime(
      String(breakout.barTime ?? breakout.eventAt ?? ""),
      visibleStart,
      visibleEnd
    );
    const breakoutLevel = getNumber(breakout.level);
    const breakoutSide = String(breakout.side ?? "").trim().toUpperCase();
    if (!breakoutTime || breakoutLevel == null || breakoutLevel <= 0) {
      continue;
    }
    const breakoutColor = breakoutSide === "SELL" ? "#b04a37" : "#0e6d60";
    const nextCandleTime =
      candles.find((item) => Date.parse(item.time) > Date.parse(breakoutTime))?.time ?? visibleEnd;
    overlays.push({
      startTime: breakoutTime,
      endTime: nextCandleTime,
      price: breakoutLevel,
      color: breakoutColor,
      lineStyle: "dotted",
    });
    markers.push({
      time: breakoutTime,
      position: breakoutSide === "SELL" ? "aboveBar" : "belowBar",
      color: breakoutColor,
      shape: "circle",
      text: "BO",
    });
  }

  if (!position || Math.abs(Number(position.quantity ?? 0)) <= 0 || !visibleEnd) {
    return { markers, overlays };
  }

  const livePositionState =
    getRecord(state.lastLivePositionState).found === true
      ? getRecord(state.lastLivePositionState)
      : getRecord(state.livePositionState);
  const stopLoss = getNumber(livePositionState.stopLoss);
  if (stopLoss == null || stopLoss <= 0) {
    return { markers, overlays };
  }

  const sessionOrders = resolveSessionOrders(session, orders);
  const fillByOrderId = new Map(fills.map((fill) => [fill.orderId, fill] as const));
  const normalizedSide = String(position.side ?? livePositionState.side ?? "").trim().toUpperCase();
  const entrySide = normalizedSide === "SHORT" ? "SELL" : "BUY";
  const entryOrder = [...sessionOrders]
    .reverse()
    .find((order) => String(order.side ?? "").trim().toUpperCase() === entrySide);
  const entryTime = clampAnnotationTime(
    fillByOrderId.get(entryOrder?.id ?? "")?.createdAt ?? entryOrder?.createdAt ?? visibleStart,
    visibleStart,
    visibleEnd
  );
  const trailingActive =
    livePositionState.trailingStopActive === true ||
    String(livePositionState.stopLossSource ?? "").trim().toLowerCase() === "trailing-stop";
  const stopMarkerPosition = normalizedSide === "SHORT" ? "aboveBar" : "belowBar";
  const stopMarkerColor = trailingActive ? "#2563eb" : "#b04a37";

  overlays.push({
    startTime: entryTime || visibleStart,
    endTime: visibleEnd,
    price: stopLoss,
    color: stopMarkerColor,
    lineStyle: "dashed",
  });
  markers.push({
    time: visibleEnd,
    position: stopMarkerPosition,
    color: stopMarkerColor,
    shape: "square",
    text: trailingActive ? "TSL" : "SL",
  });

  return { markers, overlays };
}

export function deriveLiveSessionHealth(session: LiveSession, summary: LiveSessionExecutionSummary): LiveSessionHealth {
  const recoveryError = String(session.state?.lastRecoveryError ?? "").trim();
  const syncError = String(session.state?.lastSyncError ?? "").trim();
  const protectionRecoveryStatus = String(session.state?.protectionRecoveryStatus ?? "").trim();
  const isRunning = String(session.status).toUpperCase() === "RUNNING";
  const exitDispatchFailure = resolveLiveExitDispatchFailure(session, summary);

  // 1. 恢复错误（具备最高健康度权重）
  if (recoveryError) {
    return {
      status: "error",
      detail: `恢复异常: ${recoveryError}`,
    };
  }

  // 2. 保护状态错误
  if (protectionRecoveryStatus === "unprotected-open-position") {
    return {
      status: "error",
      detail: "风险: 恢复的持仓缺失止损/止盈保护",
    };
  }

  // 3. 自动平仓失败
  if (exitDispatchFailure) {
    return {
      status: "error",
      detail: exitDispatchFailure.detail,
    };
  }

  // 4. 数据同步错误
  if (syncError) {
    return {
      status: "error",
      detail: `同步异常: ${syncError}`,
    };
  }

  // 5. 流程阻塞状态
  if (summary.latestOrder && !["FILLED", "CANCELLED", "REJECTED"].includes(String(summary.latestOrder.status ?? "").toUpperCase())) {
    return {
      status: "waiting-sync",
      detail: `订单 ${summary.latestOrder.status} 正在等待终端同步`,
    };
  }
  
  // 6. 活跃持仓状态
  if (summary.position && Math.abs(Number(summary.position.quantity ?? 0)) > 0) {
    return {
      status: "active",
      detail: `持有 ${summary.position.side ?? "仓位"} ${formatMaybeNumber(summary.position.quantity)} @ ${formatMaybeNumber(summary.position.entryPrice)}`,
    };
  }

  // 7. 正常就绪/等待
  if (String(session.state?.lastStrategyEvaluationStatus ?? "") === "intent-ready") {
    return {
      status: "ready",
      detail: "策略意图已就绪，可执行分发",
    };
  }

  return {
    status: isRunning ? "idle" : "neutral",
    detail: isRunning ? "正在等待下一个有效的策略意图" : "会话已停止",
  };
}

export function deriveHighlightedLiveSession(
  sessions: LiveSession[],
  orders: Order[],
  fills: Fill[],
  positions: Position[]
): HighlightedLiveSession {
  if (sessions.length === 0) {
    return null;
  }
  const ranked = sessions
    .map((session) => {
      const execution = deriveLiveSessionExecutionSummary(session, orders, fills, positions);
      const health = deriveLiveSessionHealth(session, execution);
      return {
        session,
        execution,
        health,
        priority: liveSessionHealthPriority(health.status),
      };
    })
    .sort((left, right) => right.priority - left.priority || Date.parse(right.session.createdAt) - Date.parse(left.session.createdAt));
  return ranked[0] ?? null;
}

export function deriveLiveSessionFlow(session: LiveSession, summary: LiveSessionExecutionSummary): LiveSessionFlowStep[] {
  const runtimeStatus = String(session.state?.signalRuntimeStatus ?? "").toUpperCase();
  const hasIntent = !!getRecord(session.state?.lastStrategyIntent).action;
  const lastOrderStatus = String(summary.latestOrder?.status ?? "").toUpperCase();
  const hasPosition = !!summary.position && Math.abs(Number(summary.position.quantity ?? 0)) > 0;
  const syncError = String(session.state?.lastSyncError ?? "").trim();
  const exitDispatchFailure = resolveLiveExitDispatchFailure(session, summary);

  return [
    {
      key: "runtime",
      label: "运行环境",
      status: runtimeStatus === "RUNNING" ? "ready" : runtimeStatus === "" ? "neutral" : "blocked",
      detail: runtimeStatus === "RUNNING" ? "运行中" : runtimeStatus || "未关联",
    },
    {
      key: "intent",
      label: "信号意图",
      status: hasIntent ? "ready" : "watch",
      detail: hasIntent ? String(getRecord(session.state?.lastStrategyIntent).signalKind ?? "就绪") : "等待中",
    },
    {
      key: "dispatch",
      label: "订单分发",
      status:
        exitDispatchFailure
          ? "blocked"
          : summary.latestOrder == null
          ? "watch"
          : ["NEW", "ACCEPTED"].includes(lastOrderStatus)
            ? "watch"
            : ["FILLED"].includes(lastOrderStatus)
              ? "ready"
              : "blocked",
      detail: exitDispatchFailure?.shortDetail ?? (summary.latestOrder ? lastOrderStatus : "未发送"),
    },
    {
      key: "sync",
      label: "数据同步",
      status: syncError || exitDispatchFailure ? "blocked" : ["FILLED", "CANCELLED", "REJECTED"].includes(lastOrderStatus) ? "ready" : summary.latestOrder ? "watch" : "neutral",
      detail: syncError ? "同步错误" : exitDispatchFailure ? "平仓失败待处理" : String(session.state?.lastSyncedOrderStatus ?? (summary.latestOrder ? "待同步" : "--")),
    },
    {
      key: "position",
      label: "当前持仓",
      status: hasPosition ? "ready" : "neutral",
      detail: hasPosition ? `${String(summary.position?.side ?? "OPEN")} ${formatMaybeNumber(summary.position?.quantity)}` : "空仓",
    },
  ];
}

function resolveLiveExitDispatchFailure(session: LiveSession, summary: LiveSessionExecutionSummary): { detail: string; shortDetail: string } | null {
  const dispatchIntent = getRecord(session.state?.lastDispatchedIntent);
  const dispatchSummary = getRecord(session.state?.lastExecutionDispatch);
  const rejectedStatus = String(
    session.state?.lastDispatchRejectedStatus ??
    session.state?.lastDispatchedOrderStatus ??
    dispatchSummary.status ??
    ""
  ).trim();
  const dispatchError = String(session.state?.lastAutoDispatchError ?? dispatchSummary.error ?? "").trim();
  const hasOpenPosition = !!summary.position && Math.abs(Number(summary.position.quantity ?? 0)) > 0;
  const role = String(dispatchIntent.role ?? dispatchSummary.role ?? "").trim().toLowerCase();
  const executionProfile = String(dispatchSummary.executionProfile ?? dispatchIntent.executionProfile ?? "").trim().toLowerCase();
  const signalKind = String(dispatchIntent.signalKind ?? dispatchSummary.signalKind ?? "").trim().toLowerCase();
  const reduceOnly = Boolean(dispatchIntent.reduceOnly ?? dispatchSummary.reduceOnly);
  const isExit = role === "exit" || reduceOnly || executionProfile.includes("exit") || executionProfile.includes("close") || signalKind.includes("exit") || signalKind.includes("watchdog");

  if (!hasOpenPosition || !isExit || (!rejectedStatus && !dispatchError)) {
    return null;
  }

  const reason = String(dispatchIntent.reason ?? dispatchSummary.reason ?? dispatchIntent.signalKind ?? "").trim();
  const detailParts = [`自动平仓失败`];
  if (rejectedStatus) {
    detailParts.push(`状态=${rejectedStatus}`);
  }
  if (reason) {
    detailParts.push(`触发=${reason}`);
  }
  if (dispatchError) {
    detailParts.push(`错误=${dispatchError}`);
  }

  return {
    detail: detailParts.join(" · "),
    shortDetail: rejectedStatus ? `平仓失败 · ${rejectedStatus}` : "平仓失败",
  };
}

export function liveSessionHealthPriority(status: string) {
  switch (status) {
    case "error":
      return 5;
    case "waiting-sync":
      return 4;
    case "ready":
      return 3;
    case "active":
      return 2;
    case "idle":
      return 1;
    default:
      return 0;
  }
}

export function deriveLiveNextAction(preflight: LivePreflightSummary): LiveNextAction {
  switch (preflight.reason) {
    case "账户未配置":
      return {
        key: "bind-live-adapter",
        label: "绑定实盘适配器",
        detail: "请先完成账户绑定与凭证导入",
      };
    case "缺失信号绑定":
      return {
        key: "bind-signals",
        label: "绑定信号源",
        detail: "请挂载所需的信号、触发器及特征源",
      };
    case "无运行时会话":
      return {
        key: "create-runtime",
        label: "创建运行时",
        detail: "为此账户和策略创建一个信号运行时会话",
      };
    case "运行时未启动":
    case "无活跃运行时":
      return {
        key: "start-runtime",
        label: "启动运行时",
        detail: "启动关联的信号运行时并等待健康数据流",
      };
    case "缺失成交数据":
      return {
        key: "inspect-runtime",
        label: "恢复成交数据流",
        detail: "确保成交数据源绑定活跃且数据正在流入",
      };
    case "缺失盘口数据":
      return {
        key: "inspect-runtime",
        label: "恢复盘口数据流",
        detail: "确保盘口特征源绑定活跃且数据新鲜",
      };
    case "数据源陈旧":
      return {
        key: "inspect-runtime",
        label: "等待数据更新",
        detail: "提交实盘订单前需等待运行时刷新陈旧的数据源状态",
      };
    case "需指定策略版本":
      return {
        key: "pass-strategy-version",
        label: "指定策略版本 ID",
        detail: "存在多个关联运行时，实盘提交必须明确选择策略版本",
      };
    case "运行时已就绪":
      return {
        key: "submit-live-order",
        label: "提交实盘订单",
        detail: "实盘运行时预检通过",
      };
    default:
      return {
        key: "inspect-runtime",
        label: "检查运行时",
        detail: "打开关联的运行时会话查看详细的就绪状态",
      };
  }
}

export function liveSessionHealthTone(status: string): "ready" | "watch" | "blocked" | "neutral" {
  switch (status) {
    case "ready":
      return "ready";
    case "active":
    case "waiting-sync":
      return "watch";
    case "error":
      return "blocked";
    default:
      return "neutral";
  }
}

export function buildSignalActionNotes(signalAction: { bias: string; state: string; reason: string }) {
  return [`信号活动: ${signalAction.bias} · ${signalAction.state} · ${signalAction.reason}`];
}

export function buildTimelineNotes(items: Array<Record<string, unknown>>, config?: TimelineConfig, sessionId?: string) {
  if (items.length === 0) {
    return ["时间线: --"];
  }

  const { deduplicationEnabled = true, quietSeconds = 60, maxRepeats = 1 } = config ?? {};
  let processedItems = items;

  if (deduplicationEnabled) {
    const filtered: Array<Record<string, unknown>> = [];
    // 摘要去重追踪：Digest -> { lastTime, displayCount }
    const lastSeenMap = new Map<string, { lastTime: number; displayCount: number }>();

    for (const item of items) {
      const metadata = getRecord(item.metadata);
      const category = String(item.category ?? "");
      const title = String(item.title ?? "");
      const reason = String(metadata.reason ?? "");
      const action = String(metadata.action ?? "");
      const eventTimeStr = String(item.time ?? "");
      const eventTime = Date.parse(eventTimeStr);

      // 生成增强版摘要：加入 sessionId 以实现会话隔离
      const activeSessionId = sessionId || String(metadata.liveSessionId ?? metadata.signalRuntimeSessionId ?? "");
      const digest = `${activeSessionId}|${category}|${title}|${reason}|${action}|${metadata.symbol ?? ""}`;

      const currentTime = !Number.isNaN(eventTime) ? eventTime : 0;
      const isDeduplicatable = category === "strategy" || category === "reconcile" || category === "recovery" || title === "waiting-source-states";

      if (isDeduplicatable) {
        const record = lastSeenMap.get(digest);
        // 判断是否在静默时间窗口内（与该摘要上一次出现的时间对比）
        const isWithinQuietPeriod = record && record.lastTime > 0 && (currentTime - record.lastTime) < quietSeconds * 1000;

        if (isWithinQuietPeriod) {
          if (record.displayCount < maxRepeats) {
            record.displayCount++;
            filtered.push(item);
          }
          // 无论是否显示，都更新该摘要的最后触达时间，确保“静默”是相对于上一次脉冲的
          record.lastTime = currentTime;
        } else {
          // 超出窗口或首次出现：开启新窗口，重置计数
          lastSeenMap.set(digest, { lastTime: currentTime, displayCount: 1 });
          filtered.push(item);
        }
      } else {
        // 非初筛决策类（如重要告警、成交记录等）始终保留，不参与去重
        filtered.push(item);
      }
    }
    processedItems = filtered;
  }

  return processedItems
    .slice(-50)
    .reverse()
    .map((item) => {
      const metadata = getRecord(item.metadata);
      const fragments = [
        formatTime(String(item.time ?? "")),
        String(item.category ?? "--"),
        String(item.title ?? "--"),
      ];
      if (metadata.symbol != null) {
        fragments.push(String(metadata.symbol));
      }
      if (metadata.timeframe != null) {
        fragments.push(String(metadata.timeframe));
      }
      if (metadata.reason != null) {
        fragments.push(String(metadata.reason));
      }
      if (metadata.signalKind != null) {
        fragments.push(String(metadata.signalKind));
      }
      if (metadata.action != null) {
        fragments.push(String(metadata.action));
      }
      if (metadata.executionProfile != null) {
        fragments.push(`profile=${String(metadata.executionProfile)}`);
      }
      if (metadata.orderType != null) {
        fragments.push(`type=${String(metadata.orderType)}`);
      }
      if (metadata.executionMode != null && String(metadata.executionMode) !== "") {
        fragments.push(`mode=${String(metadata.executionMode)}`);
      }
      if (metadata.reduceOnly != null) {
        fragments.push(`reduceOnly=${boolLabel(metadata.reduceOnly)}`);
      }
      if (metadata.fallback != null && boolLabel(metadata.fallback) !== "--") {
        fragments.push(`fallback=${boolLabel(metadata.fallback)}`);
      }
      return fragments.join(" · ");
    });
}



export function summarizeOrderPreflight(value: unknown) {
  const preflight = getRecord(value);
  if (Object.keys(preflight).length === 0) {
    return "--";
  }
  return [
    `ready=${boolLabel(preflight.ready)}`,
    `missing=${String(getList(preflight.missing).length)}`,
    `stale=${String(getList(preflight.stale).length)}`,
  ].join(" · ");
}

export function derivePaperAlerts(
  session: PaperSession | null,
  runtimeState: Record<string, unknown>,
  sourceSummary: RuntimeSourceSummary,
  readiness: RuntimeReadiness,
  decision: Record<string, unknown>,
  decisionMeta: Record<string, unknown>,
  signalBarDecision: Record<string, unknown>,
  policy: RuntimePolicy | null
): AlertItem[] {
  const alerts: AlertItem[] = [];
  const lastEventAt = Date.parse(String(runtimeState.lastEventAt ?? ""));
  const runtimeQuietMs = (policy?.runtimeQuietSeconds ?? 30) * 1000;
  const evaluationStatus = String(session?.state?.lastStrategyEvaluationStatus ?? "").trim().toLowerCase();
  const decisionState = String(decisionMeta.decisionState ?? decision.action ?? "").trim().toLowerCase();
  const signalReason = String(signalBarDecision.reason ?? "").trim().toLowerCase();

  if (readiness.status === "blocked") {
    alerts.push({ level: "critical", title: "环境阻断", detail: readiness.reason });
  } else if (readiness.status === "warning") {
    alerts.push({ level: "warning", title: "环境警告", detail: readiness.reason });
  }
  if (sourceSummary.staleCount > 0) {
    alerts.push({ level: "warning", title: "源数据陈旧", detail: `${sourceSummary.staleCount} 个数据源状态过期` });
  }
  if (evaluationStatus === "decision-error") {
    alerts.push({ level: "critical", title: "决策错误", detail: "最新的策略评估返回错误" });
  }
  if (decisionState === "waiting-signal-bars") {
    alerts.push({ level: "warning", title: "缺失 K 线", detail: "运行时尚未采集到足够的周期 K 线" });
  }
  if (signalReason === "insufficient-signal-bars") {
    alerts.push({ level: "warning", title: "信号过滤阻断", detail: "SMA5 / t-1 / t-2 所需的已收盘 K 线不足" });
  }
  if (Number.isFinite(lastEventAt) && Date.now()-lastEventAt > runtimeQuietMs) {
    alerts.push({
      level: "warning",
      title: "运行时无响应",
      detail: `过去 ${policy?.runtimeQuietSeconds ?? 30} 秒内未监测到任何运行时事件`,
    });
  }
  return dedupeAlerts(alerts);
}

export function deriveLiveAlerts(
  account: AccountRecord,
  runtimeState: Record<string, unknown>,
  sourceSummary: RuntimeSourceSummary,
  readiness: RuntimeReadiness,
  signalAction: { bias: string; state: string; reason: string },
  policy: RuntimePolicy | null
): AlertItem[] {
  const alerts: AlertItem[] = [];
  const health = String(runtimeState.health ?? "").trim().toLowerCase();
  const lastEventAt = Date.parse(String(runtimeState.lastEventAt ?? ""));
  const runtimeQuietMs = (policy?.runtimeQuietSeconds ?? 30) * 1000;

  if (account.status !== "CONFIGURED") {
    alerts.push({ level: "warning", title: "账户未配置", detail: `status=${account.status}` });
  }
  if (readiness.status === "blocked") {
    alerts.push({ level: "critical", title: "环境阻断", detail: readiness.reason });
  } else if (readiness.status === "warning") {
    alerts.push({ level: "warning", title: "环境警告", detail: readiness.reason });
  }
  if (health !== "" && health !== "healthy") {
    alerts.push({ level: "critical", title: "环境健康异常", detail: `health=${health}` });
  }
  if (sourceSummary.staleCount > 0) {
    alerts.push({ level: "warning", title: "源数据陈odd", detail: `${sourceSummary.staleCount} 个数据源状态过期` });
  }
  if (signalAction.state === "waiting") {
    alerts.push({ level: "info", title: "信号等待中", detail: signalAction.reason });
  }
  if (Number.isFinite(lastEventAt) && Date.now()-lastEventAt > runtimeQuietMs) {
    alerts.push({
      level: "warning",
      title: "运行时无响应",
      detail: `过去 ${policy?.runtimeQuietSeconds ?? 30} 秒内未监测到任何运行时事件`,
    });
  }
  return dedupeAlerts(alerts);
}

export function dedupeAlerts(items: AlertItem[]) {
  const seen = new Set<string>();
  return items.filter((item) => {
    const key = `${item.level}:${item.title}:${item.detail}`;
    if (seen.has(key)) {
      return false;
    }
    seen.add(key);
    return true;
  });
}

export function buildAlertNotes(items: AlertItem[]) {
  return items;
}

export function alertLevelTone(level: string): "ready" | "watch" | "blocked" | "neutral" {
  switch (level.trim().toLowerCase()) {
    case "critical":
      return "blocked";
    case "warning":
      return "watch";
    case "info":
      return "neutral";
    default:
      return "neutral";
  }
}

export function alertScopeTone(scope: string): "ready" | "watch" | "blocked" | "neutral" {
  switch (scope.trim().toLowerCase()) {
    case "paper":
      return "watch";
    case "live":
      return "blocked";
    case "runtime":
      return "neutral";
    default:
      return "neutral";
  }
}

export function telegramDeliveryTone(status: unknown): "ready" | "watch" | "blocked" | "neutral" {
  switch (String(status ?? "").trim().toLowerCase()) {
    case "sent":
      return "ready";
    case "failed":
      return "blocked";
    case "pending":
      return "watch";
    default:
      return "watch";
  }
}

export function runtimeReadinessTone(status: string): "ready" | "watch" | "blocked" | "neutral" {
  switch (status.trim().toLowerCase()) {
    case "ready":
      return "ready";
    case "warning":
      return "watch";
    case "blocked":
      return "blocked";
    case "recovering":
      return "watch"; // Yellow status
    case "stale-after-reconnect":
      return "blocked"; // Red status
    default:
      return "neutral";
  }
}

export function decisionStateTone(state: string): "ready" | "watch" | "blocked" | "neutral" {
  const normalized = state.trim().toLowerCase();
  if (normalized.includes("ready")) {
    return "ready";
  }
  if (normalized.startsWith("waiting") || normalized === "watch") {
    return "watch";
  }
  if (normalized.includes("blocked") || normalized.includes("error")) {
    return "blocked";
  }
  return "neutral";
}

export function signalKindTone(kind: string): "ready" | "watch" | "blocked" | "neutral" {
  const normalized = kind.trim().toLowerCase();
  if (normalized.includes("near") || normalized.includes("entry") || normalized.includes("exit")) {
    return "ready";
  }
  if (normalized.includes("watch") || normalized.includes("hold")) {
    return "watch";
  }
  if (normalized === "ignore") {
    return "neutral";
  }
  return "neutral";
}

export function signalActionTone(bias: string, state: string): "ready" | "watch" | "blocked" | "neutral" {
  const normalizedState = state.trim().toLowerCase();
  if (normalizedState === "ready") {
    return "ready";
  }
  if (normalizedState === "watch" || normalizedState === "waiting") {
    return "watch";
  }
  if (bias.trim().toLowerCase() === "neutral") {
    return "neutral";
  }
  return "neutral";
}

export function boolTone(value: unknown): "ready" | "watch" | "blocked" | "neutral" {
  if (value === true) {
    return "ready";
  }
  if (value === false) {
    return "blocked";
  }
  return "neutral";
}

export function boolLabel(value: unknown) {
  if (value === true) {
    return "就绪";
  }
  if (value === false) {
    return "阻断";
  }
  return "--";
}

export function technicalStatusLabel(value: unknown): string {
  const s = String(value ?? "").trim().toLowerCase();
  switch (s) {
    case "healthy":
      return "健康";
    case "degraded":
      return "降级";
    case "running":
      return "运行中";
    case "stopped":
      return "已停止";
    case "neutral":
      return "中性";
    case "buying":
    case "long":
      return "看多";
    case "selling":
    case "short":
      return "看空";
    case "waiting":
      return "等待中";
    case "watch":
      return "观察中";
    case "ready":
      return "就绪";
    case "blocked":
      return "阻断";
    case "configured":
      return "已配置";
    case "active":
      return "活跃";
    case "error":
      return "错误";
    default:
      return String(value ?? "--");
  }
}

export function resolveSignalBindingTimeframe(binding: SignalBinding | Record<string, unknown> | null | undefined): string {
  const record = getRecord(binding);
  const topLevel = String(record.timeframe ?? "").trim().toLowerCase();
  if (topLevel) {
    return topLevel;
  }
  const fallback = String(getRecord(record.options).timeframe ?? "").trim().toLowerCase();
  if (fallback) {
    return fallback;
  }
  return "";
}

export function displaySignalBindingTimeframe(binding: SignalBinding | Record<string, unknown> | null | undefined): string {
  return resolveSignalBindingTimeframe(binding) || "--";
}

export function runtimePolicyValueLabel(value: unknown): string {
  const num = Number(value);
  if (!Number.isFinite(num)) {
    return "--";
  }
  if (num === 0) {
    return "0 秒 (disabled)";
  }
  return `${Math.trunc(num)} 秒`;
}
