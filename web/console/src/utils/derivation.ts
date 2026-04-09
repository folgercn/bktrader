import { AccountSummary, AccountRecord, StrategyVersion, StrategyRecord, AccountEquitySnapshot, Order, Fill, Position, PaperSession, LiveSession, ChartCandle, ChartAnnotation, MarkerLegendItem, BacktestRun, BacktestOptions, LiveAdapter, SignalSourceDefinition, SignalSourceCatalog, SignalSourceType, SignalBinding, SignalRuntimeAdapter, SignalRuntimeSession, ReplayReasonStats, ReplaySample, ExecutionTrade, SourceFilter, EventFilter, TimeWindow, MarkerDetail, ChartOverrideRange, SelectedSample, SelectableSample, RuntimeMarketSnapshot, RuntimeSourceSummary, RuntimeReadiness, SignalBarCandle, AlertItem, PlatformAlert, PlatformNotification, TelegramConfig, RuntimePolicy, LivePreflightSummary, LiveNextAction, LiveDispatchPreview, LiveSessionExecutionSummary, LiveSessionHealth, HighlightedLiveSession, LiveSessionFlowStep, SessionMarker, AuthSession } from '../types/domain';

import { formatMoney, formatSigned, formatPercent, formatNumber, formatMaybeNumber, formatTime, formatShortTime, shrink } from './format';
import { createChart } from 'lightweight-charts';

export function sampleStatus(sample: ReplaySample) {
  const reason = String(sample.reason ?? "").trim().toLowerCase();
  if (reason === "entry_not_hit" || reason === "entry_missed") {
    return { label: "Entry Missed", tone: "missed" };
  }
  if (reason === "exit_not_hit" || reason === "exit_missed") {
    return { label: "Exit Missed", tone: "missed" };
  }
  if (reason.includes("invalid")) {
    return { label: "Invalid", tone: "invalid" };
  }
  if (reason.includes("error")) {
    return { label: "Error", tone: "error" };
  }
  return { label: "Completed", tone: "completed" };
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
    return { label: "No data" };
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
  return "neutral";
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
  return "Unnamed strategy";
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

export function deriveRuntimeMarketSnapshot(sourceStates: Record<string, unknown>, summary: Record<string, unknown>): RuntimeMarketSnapshot {
  const snapshot: RuntimeMarketSnapshot = {};
  const states = Object.values(sourceStates).map((value) => getRecord(value));

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

export function deriveRuntimeSourceSummary(sourceStates: Record<string, unknown>, policy: RuntimePolicy | null): RuntimeSourceSummary {
  const states = Object.values(sourceStates).map((value) => getRecord(value));
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
    return { ready: false, status: "blocked", reason: "no-runtime" };
  }
  if (health !== "" && health !== "healthy") {
    return { ready: false, status: "blocked", reason: `runtime-${health}` };
  }
  if (requirements.requireTick && sourceSummary.tradeTickCount <= 0) {
    return { ready: false, status: "blocked", reason: "missing-trade-tick" };
  }
  if (requirements.requireOrderBook && sourceSummary.orderBookCount <= 0) {
    return { ready: false, status: "blocked", reason: "missing-order-book" };
  }
  if (sourceSummary.staleCount > 0) {
    return { ready: false, status: "warning", reason: "stale-source-states" };
  }
  return { ready: true, status: "ready", reason: "runtime-healthy" };
}

export function deriveSignalBarCandles(sourceStates: Record<string, unknown>): SignalBarCandle[] {
  const candles: SignalBarCandle[] = [];
  for (const value of Object.values(sourceStates)) {
    const state = getRecord(value);
    if (String(state.streamType ?? "") !== "signal_bar") {
      continue;
    }
    const bars = Array.isArray(state.bars) ? (state.bars as Array<Record<string, unknown>>) : [];
    for (const bar of bars) {
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
        timeframe: String(bar.timeframe ?? "--"),
        isClosed: Boolean(bar.isClosed),
      });
    }
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

export function derivePrimarySignalBarState(signalBarStates: Record<string, unknown>, fallbackStates?: Record<string, unknown>) {
  const primary = Object.values(signalBarStates)[0];
  if (primary != null) {
    return getRecord(primary);
  }
  const first = Object.values(fallbackStates ?? {})[0];
  return getRecord(first);
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
  const filterLabel =
    timeframe === "1d"
      ? `sma5 ${formatMaybeNumber(signalBarDecision.sma5)} · early-long=${boolLabel(signalBarDecision.longEarlyReversalReady)} · early-short=${boolLabel(signalBarDecision.shortEarlyReversalReady)}`
      : `ma20 ${formatMaybeNumber(signalBarDecision.ma20)}`;
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
    return { bias: "neutral", state: "waiting", reason: "insufficient-signal-bars" };
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
    longReason = longHard ? "close>sma5" : longEarly ? "early reversal gate" : "1d long filter blocked";
    shortReason = shortHard ? "close<sma5" : shortEarly ? "early reversal gate" : "1d short filter blocked";
  } else if (ma20 != null) {
    longReady = close > ma20 && longBreakoutShape;
    shortReady = close < ma20 && shortBreakoutShape;
    longReason = longReady ? "close>ma20 and high2>high1" : "trend/structure not ready";
    shortReason = shortReady ? "close<ma20 and low2<low1" : "trend/structure not ready";
  } else {
    return { bias: "neutral", state: "waiting", reason: "insufficient-signal-bars" };
  }
  if (longReady && !shortReady) {
    return { bias: "long", state: "ready", reason: longReason };
  }
  if (shortReady && !longReady) {
    return { bias: "short", state: "ready", reason: shortReason };
  }
  if (timeframe === "1d" && sma5 != null) {
    if (close > sma5) {
      return { bias: "long", state: "watch", reason: "above sma5, breakout not ready" };
    }
    if (close < sma5) {
      return { bias: "short", state: "watch", reason: "below sma5, breakout not ready" };
    }
    return { bias: "neutral", state: "watch", reason: "close around sma5" };
  }
  if (ma20 != null && close > ma20) {
    return { bias: "long", state: "watch", reason: "trend ok, structure not ready" };
  }
  if (ma20 != null && close < ma20) {
    return { bias: "short", state: "watch", reason: "trend ok, structure not ready" };
  }
  return { bias: "neutral", state: "watch", reason: "close around filter" };
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
      reason: "account-not-configured",
      detail: `account status is ${account.status}`,
    };
  }
  if (bindings.length === 0) {
    return {
      status: "blocked",
      reason: "no-signal-bindings",
      detail: "bind required signal sources before live submission",
    };
  }
  if (runtimeSessionsForAccount.length === 0) {
    return {
      status: "blocked",
      reason: "no-runtime-session",
      detail: "create and start a signal runtime session first",
    };
  }
  if (runtimeSessionsForAccount.length > 1) {
    return {
      status: "watch",
      reason: "strategy-version-required",
      detail: "multiple runtime sessions linked; live orders should specify strategyVersionId",
    };
  }
  if (!activeRuntime) {
    return {
      status: "blocked",
      reason: "no-active-runtime",
      detail: "no runtime session available for this live account",
    };
  }
  if (activeRuntime.status !== "RUNNING") {
    return {
      status: "blocked",
      reason: "runtime-not-running",
      detail: `runtime status is ${activeRuntime.status}`,
    };
  }
  if (readiness.status === "blocked") {
    return {
      status: "blocked",
      reason: readiness.reason,
      detail: "runtime preflight would reject live submission",
    };
  }
  if (readiness.status === "warning") {
    return {
      status: "watch",
      reason: readiness.reason,
      detail: "runtime is degraded; live submission may be blocked soon",
    };
  }
  return {
    status: "ready",
    reason: "runtime-ready",
    detail: "live runtime preflight is satisfied",
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
      reason: "no-session",
      detail: "create a live session first",
      payload,
    };
  }
  if (!account) {
    return {
      status: "blocked",
      reason: "no-live-account",
      detail: "linked live account is missing",
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
      reason: "no-intent",
      detail: "waiting for a ready live intent from strategy evaluation",
      payload,
    };
  }
  const dispatchMode = String(session.state?.dispatchMode ?? "");
  if (dispatchMode === "auto-dispatch") {
    return {
      status: preflight.status === "watch" ? "watch" : "ready",
      reason: preflight.status === "watch" ? "preflight-warning" : "auto-dispatch-armed",
      detail:
        preflight.status === "watch"
          ? `auto-dispatch is armed, but runtime still has a warning: ${preflight.reason}`
          : "auto-dispatch is armed and the next ready intent can submit automatically",
      payload,
    };
  }
  return {
    status: preflight.status === "watch" ? "watch" : "ready",
    reason: preflight.status === "watch" ? "preflight-warning" : "dispatch-ready",
    detail:
      preflight.status === "watch"
        ? `dispatch is possible, but runtime still has a warning: ${preflight.reason}`
        : "intent, runtime, and live preflight are aligned",
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

export function deriveLiveSessionHealth(session: LiveSession, summary: LiveSessionExecutionSummary): LiveSessionHealth {
  const recoveryError = String(session.state?.lastRecoveryError ?? "").trim();
  if (recoveryError) {
    return {
      status: "error",
      detail: `recovery error: ${recoveryError}`,
    };
  }
  const protectionRecoveryStatus = String(session.state?.protectionRecoveryStatus ?? "").trim();
  if (protectionRecoveryStatus === "unprotected-open-position") {
    return {
      status: "error",
      detail: "recovered open position has no stop-loss or take-profit protection",
    };
  }
  const syncError = String(session.state?.lastSyncError ?? "").trim();
  if (syncError) {
    return {
      status: "error",
      detail: `sync error: ${syncError}`,
    };
  }
  if (summary.latestOrder && !["FILLED", "CANCELLED", "REJECTED"].includes(String(summary.latestOrder.status ?? "").toUpperCase())) {
    return {
      status: "waiting-sync",
      detail: `latest order ${summary.latestOrder.status} is still waiting for terminal sync`,
    };
  }
  if (summary.position && Math.abs(Number(summary.position.quantity ?? 0)) > 0) {
    return {
      status: "active",
      detail: `open ${summary.position.side ?? "position"} ${formatMaybeNumber(summary.position.quantity)} @ ${formatMaybeNumber(summary.position.entryPrice)}`,
    };
  }
  if (String(session.state?.lastStrategyEvaluationStatus ?? "") === "intent-ready") {
    return {
      status: "ready",
      detail: "intent is ready and session can dispatch",
    };
  }
  return {
    status: "idle",
    detail: "waiting for the next valid strategy intent",
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

  return [
    {
      key: "runtime",
      label: "runtime",
      status: runtimeStatus === "RUNNING" ? "ready" : runtimeStatus === "" ? "neutral" : "blocked",
      detail: runtimeStatus || "not-linked",
    },
    {
      key: "intent",
      label: "intent",
      status: hasIntent ? "ready" : "watch",
      detail: hasIntent ? String(getRecord(session.state?.lastStrategyIntent).signalKind ?? "ready") : "waiting",
    },
    {
      key: "dispatch",
      label: "dispatch",
      status:
        summary.latestOrder == null
          ? "watch"
          : ["NEW", "ACCEPTED"].includes(lastOrderStatus)
            ? "watch"
            : ["FILLED"].includes(lastOrderStatus)
              ? "ready"
              : "blocked",
      detail: summary.latestOrder ? lastOrderStatus : "not-sent",
    },
    {
      key: "sync",
      label: "sync",
      status: syncError ? "blocked" : ["FILLED", "CANCELLED", "REJECTED"].includes(lastOrderStatus) ? "ready" : summary.latestOrder ? "watch" : "neutral",
      detail: syncError || String(session.state?.lastSyncedOrderStatus ?? (summary.latestOrder ? "pending" : "--")),
    },
    {
      key: "position",
      label: "position",
      status: hasPosition ? "ready" : "neutral",
      detail: hasPosition ? `${String(summary.position?.side ?? "OPEN")} ${formatMaybeNumber(summary.position?.quantity)}` : "flat",
    },
  ];
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
    case "account-not-configured":
      return {
        key: "bind-live-adapter",
        label: "bind live adapter",
        detail: "finish account binding and credentials first",
      };
    case "no-signal-bindings":
      return {
        key: "bind-signals",
        label: "bind signals",
        detail: "attach required signal, trigger, and feature sources",
      };
    case "no-runtime-session":
      return {
        key: "create-runtime",
        label: "create runtime",
        detail: "create a signal runtime session for this account and strategy",
      };
    case "runtime-not-running":
    case "no-active-runtime":
      return {
        key: "start-runtime",
        label: "start runtime",
        detail: "start the linked signal runtime session and wait for healthy data flow",
      };
    case "missing-trade-tick":
      return {
        key: "inspect-runtime",
        label: "restore tick feed",
        detail: "ensure trade tick binding is active and source states are flowing",
      };
    case "missing-order-book":
      return {
        key: "inspect-runtime",
        label: "restore order book",
        detail: "ensure order book feature binding is active and fresh",
      };
    case "stale-source-states":
      return {
        key: "inspect-runtime",
        label: "wait for fresh data",
        detail: "let the runtime refresh source states before submitting live orders",
      };
    case "strategy-version-required":
      return {
        key: "pass-strategy-version",
        label: "pass strategyVersionId",
        detail: "multiple runtimes are linked, so live submission must choose one strategy version",
      };
    case "runtime-ready":
      return {
        key: "submit-live-order",
        label: "submit live order",
        detail: "runtime preflight is satisfied",
      };
    default:
      return {
        key: "inspect-runtime",
        label: "inspect runtime",
        detail: "open the linked runtime session and review readiness details",
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
  return [`signal-action: ${signalAction.bias} · ${signalAction.state} · ${signalAction.reason}`];
}

export function buildTimelineNotes(items: Array<Record<string, unknown>>) {
  if (items.length === 0) {
    return ["timeline: --"];
  }
  return items
    .slice(-5)
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
    alerts.push({ level: "critical", title: "Runtime blocked", detail: readiness.reason });
  } else if (readiness.status === "warning") {
    alerts.push({ level: "warning", title: "Runtime warning", detail: readiness.reason });
  }
  if (sourceSummary.staleCount > 0) {
    alerts.push({ level: "warning", title: "Stale sources", detail: `${sourceSummary.staleCount} source state(s) outdated` });
  }
  if (evaluationStatus === "decision-error") {
    alerts.push({ level: "critical", title: "Decision error", detail: "latest strategy evaluation returned an error" });
  }
  if (decisionState === "waiting-signal-bars") {
    alerts.push({ level: "warning", title: "Signal bars missing", detail: "runtime has not collected enough higher-timeframe bars yet" });
  }
  if (signalReason === "insufficient-signal-bars") {
    alerts.push({ level: "warning", title: "Signal filter blocked", detail: "insufficient closed signal bars for MA20 / t-1 / t-2" });
  }
  if (Number.isFinite(lastEventAt) && Date.now()-lastEventAt > runtimeQuietMs) {
    alerts.push({
      level: "warning",
      title: "Runtime quiet",
      detail: `no runtime events observed in the last ${policy?.runtimeQuietSeconds ?? 30}s`,
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
    alerts.push({ level: "warning", title: "Account not configured", detail: `status=${account.status}` });
  }
  if (readiness.status === "blocked") {
    alerts.push({ level: "critical", title: "Runtime blocked", detail: readiness.reason });
  } else if (readiness.status === "warning") {
    alerts.push({ level: "warning", title: "Runtime warning", detail: readiness.reason });
  }
  if (health !== "" && health !== "healthy") {
    alerts.push({ level: "critical", title: "Runtime health", detail: `health=${health}` });
  }
  if (sourceSummary.staleCount > 0) {
    alerts.push({ level: "warning", title: "Stale sources", detail: `${sourceSummary.staleCount} source state(s) outdated` });
  }
  if (signalAction.state === "waiting") {
    alerts.push({ level: "info", title: "Signal waiting", detail: signalAction.reason });
  }
  if (Number.isFinite(lastEventAt) && Date.now()-lastEventAt > runtimeQuietMs) {
    alerts.push({
      level: "warning",
      title: "Runtime quiet",
      detail: `no runtime events observed in the last ${policy?.runtimeQuietSeconds ?? 30}s`,
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
  switch (status) {
    case "ready":
      return "ready";
    case "warning":
      return "watch";
    case "blocked":
      return "blocked";
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
    return "ready";
  }
  if (value === false) {
    return "blocked";
  }
  return "--";
}

