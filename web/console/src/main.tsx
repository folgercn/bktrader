import React, { useEffect, useMemo, useRef, useState } from "react";
import ReactDOM from "react-dom/client";
import { CandlestickSeries, ColorType, CrosshairMode, LineStyle, createChart, createSeriesMarkers } from "lightweight-charts";
import "./styles.css";

type AccountSummary = {
  accountId: string;
  accountName: string;
  mode: string;
  exchange: string;
  status: string;
  startEquity: number;
  realizedPnl: number;
  unrealizedPnl: number;
  fees: number;
  netEquity: number;
  exposureNotional: number;
  openPositionCount: number;
  updatedAt: string;
};

type AccountRecord = {
  id: string;
  name: string;
  mode: string;
  exchange: string;
  status: string;
  metadata?: Record<string, unknown>;
  createdAt: string;
};

type StrategyVersion = {
  id: string;
  strategyId: string;
  version: string;
  signalTimeframe: string;
  executionTimeframe: string;
  parameters?: Record<string, unknown>;
  createdAt: string;
};

type StrategyRecord = {
  id: string;
  name: string;
  status: string;
  description: string;
  createdAt: string;
  currentVersion?: StrategyVersion;
};

type AccountEquitySnapshot = {
  id: string;
  accountId: string;
  startEquity: number;
  realizedPnl: number;
  unrealizedPnl: number;
  fees: number;
  netEquity: number;
  exposureNotional: number;
  openPositionCount: number;
  createdAt: string;
};

type Order = {
  id: string;
  accountId: string;
  symbol: string;
  side: string;
  type: string;
  status: string;
  quantity: number;
  price: number;
  metadata?: Record<string, unknown>;
  createdAt: string;
};

type Fill = {
  id: string;
  orderId: string;
  price: number;
  quantity: number;
  fee: number;
  createdAt: string;
};

type Position = {
  id: string;
  accountId: string;
  symbol: string;
  side: string;
  quantity: number;
  entryPrice: number;
  markPrice: number;
  updatedAt: string;
};

type PaperSession = {
  id: string;
  accountId: string;
  strategyId: string;
  status: string;
  startEquity: number;
  state?: Record<string, unknown>;
  createdAt: string;
};

type ChartCandle = {
  symbol: string;
  resolution: string;
  time: string;
  open: number;
  high: number;
  low: number;
  close: number;
  volume: number;
};

type ChartAnnotation = {
  id: string;
  source: string;
  type: string;
  symbol: string;
  time: string;
  price: number;
  label: string;
  metadata?: Record<string, unknown>;
};

type MarkerLegendItem = {
  label: string;
  color: string;
};

type BacktestRun = {
  id: string;
  strategyVersionId: string;
  status: string;
  parameters?: Record<string, unknown>;
  resultSummary?: Record<string, unknown>;
  createdAt: string;
};

type BacktestOptions = {
  signalTimeframes: string[];
  executionDataSources: string[];
  defaultSignalTimeframe: string;
  defaultExecutionDataSource: string;
  dataDirectories?: Record<string, string>;
  availability?: Record<string, string>;
  datasets?: Record<string, Array<{ name: string; path: string; symbol?: string; format?: string; fileCount?: number; timeColumn?: string }>>;
  supportedSymbols?: Record<string, string[]>;
  schema?: Record<string, { requiredColumns?: string[]; optionalColumns?: string[]; filenameExamples?: string[] }>;
  notes: string[];
};

type LiveAdapter = {
  key: string;
  name: string;
  environments?: string[];
  positionModes?: string[];
  marginModes?: string[];
  feeSource?: string;
  fundingSource?: string;
};

type ReplayReasonStats = Record<string, Record<string, number>>;
type ReplaySample = Record<string, unknown>;
type ExecutionTrade = Record<string, unknown>;

type SourceFilter = "all" | "paper" | "backtest";
type EventFilter = "all" | "initial" | "reentry" | "pt" | "sl";
type TimeWindow = "6h" | "12h" | "1d" | "3d";
type MarkerDetail = {
  id: string;
  source: string;
  type: string;
  label: string;
  time: string;
  price: number;
  reason?: string;
  paperSession?: string;
};

type ChartOverrideRange = {
  from: number;
  to: number;
  label: string;
};

type SelectedSample = {
  key: string;
  sample: ReplaySample;
};

type SelectableSample = SelectedSample & {
  group: "completed" | "skipped";
};

const API_BASE = (import.meta.env.VITE_API_BASE as string | undefined) ?? "http://127.0.0.1:8080";

function App() {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [summaries, setSummaries] = useState<AccountSummary[]>([]);
  const [accounts, setAccounts] = useState<AccountRecord[]>([]);
  const [orders, setOrders] = useState<Order[]>([]);
  const [fills, setFills] = useState<Fill[]>([]);
  const [positions, setPositions] = useState<Position[]>([]);
  const [snapshots, setSnapshots] = useState<AccountEquitySnapshot[]>([]);
  const [strategies, setStrategies] = useState<StrategyRecord[]>([]);
  const [backtests, setBacktests] = useState<BacktestRun[]>([]);
  const [backtestOptions, setBacktestOptions] = useState<BacktestOptions | null>(null);
  const [paperSessions, setPaperSessions] = useState<PaperSession[]>([]);
  const [liveAdapters, setLiveAdapters] = useState<LiveAdapter[]>([]);
  const [candles, setCandles] = useState<ChartCandle[]>([]);
  const [annotations, setAnnotations] = useState<ChartAnnotation[]>([]);
  const [sessionAction, setSessionAction] = useState<string | null>(null);
  const [paperCreateAction, setPaperCreateAction] = useState(false);
  const [liveCreateAction, setLiveCreateAction] = useState(false);
  const [liveBindAction, setLiveBindAction] = useState(false);
  const [liveSyncAction, setLiveSyncAction] = useState<string | null>(null);
  const [backtestAction, setBacktestAction] = useState(false);
  const [sourceFilter, setSourceFilter] = useState<SourceFilter>("all");
  const [eventFilter, setEventFilter] = useState<EventFilter>("all");
  const [timeWindow, setTimeWindow] = useState<TimeWindow>("12h");
  const [focusNonce, setFocusNonce] = useState(0);
  const [hoveredMarker, setHoveredMarker] = useState<MarkerDetail | null>(null);
  const [selectedBacktestId, setSelectedBacktestId] = useState<string | null>(null);
  const [chartOverrideRange, setChartOverrideRange] = useState<ChartOverrideRange | null>(null);
  const [selectedSample, setSelectedSample] = useState<SelectedSample | null>(null);
  const [backtestForm, setBacktestForm] = useState({
    strategyVersionId: "",
    signalTimeframe: "1d",
    executionDataSource: "1min",
    symbol: "BTCUSDT",
    from: "",
    to: "",
  });
  const [paperForm, setPaperForm] = useState({
    accountId: "",
    strategyId: "",
    startEquity: "100000",
    signalTimeframe: "1d",
    executionDataSource: "1min",
    symbol: "BTCUSDT",
    from: "",
    to: "",
    tradingFeeBps: "10",
    fundingRateBps: "0",
    fundingIntervalHours: "8",
  });
  const [liveAccountForm, setLiveAccountForm] = useState({
    name: "Live Binance",
    exchange: "binance-futures",
  });
  const [liveBindingForm, setLiveBindingForm] = useState({
    accountId: "",
    adapterKey: "binance-futures",
    positionMode: "ONE_WAY",
    marginMode: "CROSSED",
    sandbox: true,
    apiKeyRef: "",
    apiSecretRef: "",
  });

  const primaryAccount = summaries[0] ?? null;
  const primarySession = paperSessions[0] ?? null;
  const primarySessionSourceStates = getRecord(primarySession?.state?.lastStrategyEvaluationSourceStates);
  const primarySessionTriggerSource = getRecord(primarySession?.state?.lastStrategyEvaluationTriggerSource);
  const primarySessionSourceGate = getRecord(primarySession?.state?.lastStrategyEvaluationSourceGate);
  const paperAccounts = summaries.filter((item) => item.mode === "PAPER");
  const liveAccounts = accounts.filter((item) => item.mode === "LIVE");
  const syncableLiveOrders = orders.filter((item) => item.metadata?.executionMode === "live" && item.status === "ACCEPTED");
  const selectedExecutionAvailability = backtestOptions?.availability?.[backtestForm.executionDataSource] ?? "unknown";
  const selectedExecutionDatasets = backtestOptions?.datasets?.[backtestForm.executionDataSource] ?? [];
  const selectedExecutionSymbols = backtestOptions?.supportedSymbols?.[backtestForm.executionDataSource] ?? [];
  const selectedExecutionSchema = backtestOptions?.schema?.[backtestForm.executionDataSource];
  const selectedSymbolAvailable =
    selectedExecutionSymbols.length === 0 || selectedExecutionSymbols.includes(backtestForm.symbol.trim().toUpperCase());
  const backtestItems = backtests.slice().reverse().slice(0, 8);
  const selectedBacktest =
    backtests.find((item) => item.id === selectedBacktestId) ??
    (backtests.length > 0 ? backtests[backtests.length - 1] : null);
  const latestBacktestSummary = (selectedBacktest?.resultSummary ?? {}) as Record<string, unknown>;
  const latestExecutionSource = String(latestBacktestSummary.executionDataSource ?? selectedBacktest?.parameters?.executionDataSource ?? "");
  const previewCountLabel = latestExecutionSource === "tick" ? "Preview Ticks" : "Preview Bars";
  const processedCountLabel = latestExecutionSource === "tick" ? "Processed Ticks" : "Processed Bars";
  const processedCountValue =
    latestExecutionSource === "tick"
      ? String(latestBacktestSummary.processedTicks ?? "--")
      : String(latestBacktestSummary.processedBars ?? "--");
  const latestReplayByReason = (latestBacktestSummary.replayLedgerByReason ?? {}) as ReplayReasonStats;
  const latestExecutionTrades = Array.isArray(latestBacktestSummary.executionTrades)
    ? (latestBacktestSummary.executionTrades as ExecutionTrade[])
    : [];
  const latestReplaySkippedSamples = Array.isArray(latestBacktestSummary.replayLedgerSkippedSamples)
    ? (latestBacktestSummary.replayLedgerSkippedSamples as ReplaySample[])
    : [];
  const latestReplayCompletedSamples = Array.isArray(latestBacktestSummary.replayLedgerCompletedSamples)
    ? (latestBacktestSummary.replayLedgerCompletedSamples as ReplaySample[])
    : [];
  const selectableSamples = useMemo<SelectableSample[]>(
    () => [
      ...latestReplayCompletedSamples.map((sample, index) => ({
        key: buildSampleKey("completed", index, sample),
        sample,
        group: "completed" as const,
      })),
      ...latestReplaySkippedSamples.map((sample, index) => ({
        key: buildSampleKey("skipped", index, sample),
        sample,
        group: "skipped" as const,
      })),
    ],
    [latestReplayCompletedSamples, latestReplaySkippedSamples]
  );

  async function loadDashboard() {
    const [summaryData, accountData, ordersData, fillsData, positionsData, paperSessionData, strategyData, backtestData, backtestOptionsData, liveAdapterData] = await Promise.all([
      fetchJSON<AccountSummary[]>("/api/v1/account-summaries"),
      fetchJSON<AccountRecord[]>("/api/v1/accounts"),
      fetchJSON<Order[]>("/api/v1/orders"),
      fetchJSON<Fill[]>("/api/v1/fills"),
      fetchJSON<Position[]>("/api/v1/positions"),
      fetchJSON<PaperSession[]>("/api/v1/paper/sessions"),
      fetchJSON<StrategyRecord[]>("/api/v1/strategies"),
      fetchJSON<BacktestRun[]>("/api/v1/backtests"),
      fetchJSON<BacktestOptions>("/api/v1/backtests/options"),
      fetchJSON<LiveAdapter[]>("/api/v1/live-adapters"),
    ]);

    const anchorDate = resolveChartAnchor(paperSessionData[0], ordersData);
    const range = chartOverrideRange ?? buildTimeRange(anchorDate, timeWindow);
    const from = range.from;
    const to = range.to;

    const [snapshotData, candleData, annotationData] = await Promise.all([
      summaryData[0]?.accountId
        ? fetchJSON<AccountEquitySnapshot[]>(
            `/api/v1/account-equity-snapshots?accountId=${encodeURIComponent(summaryData[0].accountId)}`
          )
        : Promise.resolve([]),
      fetchJSON<{ candles: ChartCandle[] }>(
        `/api/v1/chart/candles?symbol=BTCUSDT&resolution=1&from=${from}&to=${to}&limit=840`
      ),
      fetchJSON<ChartAnnotation[]>(
        `/api/v1/chart/annotations?symbol=BTCUSDT&from=${from}&to=${to}&limit=300`
      ),
    ]);

    setSummaries(summaryData);
    setAccounts(accountData);
    setOrders(ordersData);
    setFills(fillsData);
    setPositions(positionsData);
    setSnapshots(snapshotData);
    setStrategies(strategyData);
    setBacktests(backtestData);
    setSelectedBacktestId((current) => {
      if (current && backtestData.some((item) => item.id === current)) {
        return current;
      }
      return backtestData.length > 0 ? backtestData[backtestData.length - 1].id : null;
    });
    setBacktestOptions(backtestOptionsData);
    setPaperSessions(paperSessionData);
    setLiveAdapters(liveAdapterData);
    setCandles(candleData.candles ?? []);
    setAnnotations(annotationData);
    setBacktestForm((current) => ({
      strategyVersionId: current.strategyVersionId || strategyData[0]?.currentVersion?.id || "",
      signalTimeframe: current.signalTimeframe || backtestOptionsData.defaultSignalTimeframe,
      executionDataSource: current.executionDataSource || backtestOptionsData.defaultExecutionDataSource,
      symbol: current.symbol || "BTCUSDT",
      from: current.from || "",
      to: current.to || "",
    }));
    setPaperForm((current) => ({
      accountId: current.accountId || paperAccountsFromSummaries(summaryData)[0]?.accountId || "",
      strategyId: current.strategyId || strategyData[0]?.id || "",
      startEquity: current.startEquity || "100000",
      signalTimeframe: current.signalTimeframe || backtestOptionsData.defaultSignalTimeframe,
      executionDataSource: current.executionDataSource || "1min",
      symbol: current.symbol || "BTCUSDT",
      from: current.from || "",
      to: current.to || "",
      tradingFeeBps: current.tradingFeeBps || "10",
      fundingRateBps: current.fundingRateBps || "0",
      fundingIntervalHours: current.fundingIntervalHours || "8",
    }));
    setLiveBindingForm((current) => ({
      accountId: current.accountId || accountData.find((item) => item.mode === "LIVE")?.id || "",
      adapterKey: current.adapterKey || liveAdapterData[0]?.key || "binance-futures",
      positionMode: current.positionMode || "ONE_WAY",
      marginMode: current.marginMode || "CROSSED",
      sandbox: current.sandbox,
      apiKeyRef: current.apiKeyRef,
      apiSecretRef: current.apiSecretRef,
    }));
  }

  useEffect(() => {
    let active = true;

    async function load() {
      try {
        await loadDashboard();
        if (!active) {
          return;
        }
        setError(null);
      } catch (err) {
        if (!active) {
          return;
        }
        setError(err instanceof Error ? err.message : "Failed to load monitoring data");
      } finally {
        if (active) {
          setLoading(false);
        }
      }
    }

    load();
    const timer = window.setInterval(load, 5000);
    return () => {
      active = false;
      window.clearInterval(timer);
    };
  }, [timeWindow, chartOverrideRange]);

  useEffect(() => {
    setSelectedSample(null);
  }, [selectedBacktest?.id]);

  const chartPath = useMemo(() => buildLinePath(snapshots.map((item) => item.netEquity), 560, 180), [snapshots]);
  const chartRange = useMemo(() => summarizeRange(snapshots.map((item) => item.netEquity)), [snapshots]);
  const candleRange = useMemo(() => summarizeTimeRange(candles.map((item) => item.time)), [candles]);
  const chartAnnotations = useMemo(
    () => filterChartAnnotations(annotations, candles, primarySession?.id, sourceFilter, eventFilter),
    [annotations, candles, primarySession?.id, sourceFilter, eventFilter]
  );
  const selectedAnnotationIds = useMemo(() => {
    if (!selectedSample) {
      return [];
    }
    return chartAnnotations.filter((item) => annotationMatchesSample(item, selectedSample.sample)).map((item) => item.id);
  }, [chartAnnotations, selectedSample]);
  const selectedAnnotationFocusTime = useMemo(() => {
    if (selectedAnnotationIds.length === 0) {
      return undefined;
    }
    return chartAnnotations.find((item) => item.id === selectedAnnotationIds[0])?.time;
  }, [chartAnnotations, selectedAnnotationIds]);
  const selectedMarkerDetail = useMemo<MarkerDetail | null>(() => {
    if (selectedAnnotationIds.length === 0) {
      return null;
    }
    const item = chartAnnotations.find((annotation) => annotation.id === selectedAnnotationIds[0]);
    return item ? toMarkerDetail(item) : null;
  }, [chartAnnotations, selectedAnnotationIds]);
  const latestVisibleAnnotationTime = useMemo(
    () => (chartAnnotations.length > 0 ? chartAnnotations[chartAnnotations.length - 1].time : undefined),
    [chartAnnotations]
  );
  const markerDetail = useMemo<MarkerDetail | null>(() => {
    if (hoveredMarker) {
      return hoveredMarker;
    }
    if (selectedMarkerDetail) {
      return selectedMarkerDetail;
    }
    const latest = chartAnnotations[chartAnnotations.length - 1];
    return latest ? toMarkerDetail(latest) : null;
  }, [chartAnnotations, hoveredMarker, selectedMarkerDetail]);
  const markerLegend = useMemo<MarkerLegendItem[]>(
    () => [
      { label: "Initial", color: "#7a8791" },
      { label: "PT-Reentry", color: "#0e6d60" },
      { label: "SL-Reentry", color: "#1f8f7d" },
      { label: "PT Exit", color: "#c58b2d" },
      { label: "SL Exit", color: "#b04a37" },
      { label: "Paper Fill", color: "#284d86" },
    ],
    []
  );

  async function runSessionAction(sessionId: string, action: "start" | "stop" | "tick") {
    try {
      setSessionAction(`${sessionId}:${action}`);
      setError(null);
      await fetchJSON(`/api/v1/paper/sessions/${sessionId}/${action}`, { method: "POST" });
      await loadDashboard();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to execute paper session action");
    } finally {
      setSessionAction(null);
    }
  }

  async function createPaperSession() {
    if (!paperForm.accountId || !paperForm.strategyId) {
      setError("Paper session needs an account and strategy");
      return;
    }
    setPaperCreateAction(true);
    try {
      await fetchJSON("/api/v1/paper/sessions", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          accountId: paperForm.accountId,
          strategyId: paperForm.strategyId,
          startEquity: Number(paperForm.startEquity) || 100000,
          signalTimeframe: paperForm.signalTimeframe,
          executionDataSource: paperForm.executionDataSource,
          symbol: paperForm.symbol,
          from: paperForm.from || undefined,
          to: paperForm.to || undefined,
          tradingFeeBps: Number(paperForm.tradingFeeBps) || 0,
          fundingRateBps: Number(paperForm.fundingRateBps) || 0,
          fundingIntervalHours: Number(paperForm.fundingIntervalHours) || 8,
        }),
      });
      await loadDashboard();
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create paper session");
    } finally {
      setPaperCreateAction(false);
    }
  }

  async function createLiveAccount() {
    setLiveCreateAction(true);
    try {
      await fetchJSON("/api/v1/accounts", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          name: liveAccountForm.name,
          mode: "LIVE",
          exchange: liveAccountForm.exchange,
        }),
      });
      await loadDashboard();
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create live account");
    } finally {
      setLiveCreateAction(false);
    }
  }

  async function bindLiveAccount() {
    if (!liveBindingForm.accountId) {
      setError("Live binding needs an account");
      return;
    }
    setLiveBindAction(true);
    try {
      await fetchJSON(`/api/v1/live/accounts/${liveBindingForm.accountId}/binding`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          adapterKey: liveBindingForm.adapterKey,
          positionMode: liveBindingForm.positionMode,
          marginMode: liveBindingForm.marginMode,
          sandbox: liveBindingForm.sandbox,
          credentialRefs: {
            apiKeyRef: liveBindingForm.apiKeyRef,
            apiSecretRef: liveBindingForm.apiSecretRef,
          },
        }),
      });
      await loadDashboard();
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to bind live account");
    } finally {
      setLiveBindAction(false);
    }
  }

  async function syncLiveOrder(orderId: string) {
    setLiveSyncAction(orderId);
    try {
      await fetchJSON(`/api/v1/orders/${orderId}/sync`, { method: "POST" });
      await loadDashboard();
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to sync live order");
    } finally {
      setLiveSyncAction(null);
    }
  }

  async function createBacktestRun() {
    try {
      setBacktestAction(true);
      setError(null);
      await fetchJSON<BacktestRun>("/api/v1/backtests", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          strategyVersionId: backtestForm.strategyVersionId,
          parameters: {
            signalTimeframe: backtestForm.signalTimeframe,
            executionDataSource: backtestForm.executionDataSource,
            symbol: backtestForm.symbol,
            from: backtestForm.from || undefined,
            to: backtestForm.to || undefined,
          },
        }),
      });
      await loadDashboard();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create backtest");
    } finally {
      setBacktestAction(false);
    }
  }

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div>
          <p className="sidebar-label">bkTrader Console</p>
          <h1>Paper Monitor</h1>
        </div>
        <nav>
          <a href="#overview">Overview</a>
          <a href="#paper">Paper</a>
          <a href="#live">Live</a>
          <a href="#backtests">Backtests</a>
          <a href="#market">Market</a>
          <a href="#equity">Equity</a>
          <a href="#positions">Positions</a>
          <a href="#orders">Orders</a>
          <a href="#fills">Fills</a>
        </nav>
        <div className="status-panel">
          <span className={error ? "status-dot status-bad" : "status-dot status-good"} />
          <div>
            <strong>{error ? "Feed Error" : "Feed Live"}</strong>
            <p>{error ?? `Polling ${API_BASE} every 5s`}</p>
          </div>
        </div>
      </aside>

      <main className="main">
        <section id="overview" className="hero">
          <div>
            <p className="eyebrow">Paper Trading Operations</p>
            <h2>账户监控、K 线回放与执行流水</h2>
            <p className="hero-copy">
              当前页面直接消费平台 API，展示 paper 账户的权益、成交、持仓，以及基于执行数据源回放的 BTCUSDT 行情与订单标记。
            </p>
          </div>
          <div className="hero-side">
            <div className="hero-pill">{loading ? "Loading..." : `${summaries.length} account`}</div>
            <div className="hero-pill hero-pill-accent">{primaryAccount?.mode ?? "No account"}</div>
          </div>
        </section>

        <section className="metrics-grid">
          <MetricCard label="Net Equity" value={formatMoney(primaryAccount?.netEquity)} tone="accent" />
          <MetricCard label="Realized PnL" value={formatSigned(primaryAccount?.realizedPnl)} />
          <MetricCard label="Unrealized PnL" value={formatSigned(primaryAccount?.unrealizedPnl)} />
          <MetricCard label="Fees" value={formatMoney(primaryAccount?.fees)} />
          <MetricCard label="Exposure" value={formatMoney(primaryAccount?.exposureNotional)} />
          <MetricCard label="Open Positions" value={String(primaryAccount?.openPositionCount ?? 0)} />
        </section>

        <section id="backtests" className="panel panel-backtests">
          <div className="panel-header">
            <div>
              <p className="panel-kicker">Backtests</p>
              <h3>回测配置与运行记录</h3>
            </div>
            <div className="range-box">
              <span>{backtests.length} runs</span>
              <span>{strategies.length} strategies</span>
            </div>
          </div>
          <div className="backtest-layout">
            <div className="backtest-form">
              <div className="form-grid">
                <label className="form-field">
                  <span>Strategy Version</span>
                  <select
                    value={backtestForm.strategyVersionId}
                    onChange={(event) => setBacktestForm((current) => ({ ...current, strategyVersionId: event.target.value }))}
                  >
                    {strategies.map((strategy) => (
                      <option key={strategy.id} value={strategy.currentVersion?.id ?? ""}>
                        {strategy.name} · {strategy.currentVersion?.version ?? "no-version"}
                      </option>
                    ))}
                  </select>
                </label>
                <label className="form-field">
                  <span>Signal Timeframe</span>
                  <select
                    value={backtestForm.signalTimeframe}
                    onChange={(event) => setBacktestForm((current) => ({ ...current, signalTimeframe: event.target.value }))}
                  >
                    {(backtestOptions?.signalTimeframes ?? ["4h", "1d"]).map((item) => (
                      <option key={item} value={item}>
                        {item}
                      </option>
                    ))}
                  </select>
                </label>
                <label className="form-field">
                  <span>Execution Source</span>
                  <select
                    value={backtestForm.executionDataSource}
                    onChange={(event) => setBacktestForm((current) => ({ ...current, executionDataSource: event.target.value }))}
                  >
                    {(backtestOptions?.executionDataSources ?? ["tick", "1min"]).map((item) => (
                      <option key={item} value={item}>
                        {item}
                      </option>
                    ))}
                  </select>
                </label>
                <label className="form-field">
                  <span>Symbol</span>
                  <input
                    value={backtestForm.symbol}
                    onChange={(event) => setBacktestForm((current) => ({ ...current, symbol: event.target.value.toUpperCase() }))}
                    placeholder="BTCUSDT"
                  />
                </label>
                <label className="form-field">
                  <span>From (RFC3339)</span>
                  <input
                    value={backtestForm.from}
                    onChange={(event) => setBacktestForm((current) => ({ ...current, from: event.target.value }))}
                    placeholder="2020-01-01T00:00:00Z"
                  />
                </label>
                <label className="form-field">
                  <span>To (RFC3339)</span>
                  <input
                    value={backtestForm.to}
                    onChange={(event) => setBacktestForm((current) => ({ ...current, to: event.target.value }))}
                    placeholder="2020-01-31T23:59:59Z"
                  />
                </label>
              </div>
              <div className="backtest-actions">
                <ActionButton
                  label={backtestAction ? "Submitting..." : "Create Backtest"}
                  disabled={
                    backtestAction ||
                    backtestForm.strategyVersionId.trim() === "" ||
                    backtestForm.symbol.trim() === "" ||
                    selectedExecutionAvailability === "missing" ||
                    !selectedSymbolAvailable
                  }
                  onClick={createBacktestRun}
                />
              </div>
              {backtestOptions ? (
                <div className="backtest-notes">
                  <div className="note-item">
                    tick: {String(backtestOptions.availability?.tick ?? "unknown")} · dir: {String(backtestOptions.dataDirectories?.tick ?? "--")}
                  </div>
                  <div className="note-item">
                    1min: {String(backtestOptions.availability?.["1min"] ?? "unknown")} · dir: {String(backtestOptions.dataDirectories?.["1min"] ?? "--")}
                  </div>
                  <div className="note-item">
                    selected source: {backtestForm.executionDataSource} · {selectedExecutionDatasets.length} dataset file(s)
                  </div>
                  <div className="note-item">
                    symbols: {selectedExecutionSymbols.length > 0 ? selectedExecutionSymbols.join(", ") : "--"}
                  </div>
                  <div className="note-item">
                    required columns: {selectedExecutionSchema?.requiredColumns?.join(", ") ?? "--"}
                  </div>
                  <div className="note-item">
                    file examples: {selectedExecutionSchema?.filenameExamples?.join(", ") ?? "--"}
                  </div>
                  {!selectedSymbolAvailable ? (
                    <div className="note-item">
                      symbol {backtestForm.symbol.trim().toUpperCase()} is not available for {backtestForm.executionDataSource}
                    </div>
                  ) : null}
                  {selectedExecutionDatasets.slice(0, 3).map((dataset) => (
                    <div key={dataset.path} className="note-item">
                      {dataset.name} · {dataset.symbol}
                      {dataset.format ? ` · ${dataset.format}` : ""}
                      {dataset.fileCount ? ` · files ${dataset.fileCount}` : ""}
                    </div>
                  ))}
                  {backtestOptions.notes.map((note) => (
                    <div key={note} className="note-item">
                      {note}
                    </div>
                  ))}
                </div>
              ) : null}
            </div>
            <div className="backtest-list">
              {backtestItems.length > 0 ? (
                <div className="table-wrap">
                  <table>
                    <thead>
                      <tr>
                        <th>Time</th>
                        <th>Mode</th>
                        <th>Symbol</th>
                        <th>Status</th>
                        <th>Return</th>
                        <th>DD</th>
                      </tr>
                    </thead>
                    <tbody>
                      {backtestItems.map((item) => (
                        <tr
                          key={item.id}
                          className={item.id === selectedBacktest?.id ? "table-row-active" : ""}
                          onClick={() => setSelectedBacktestId(item.id)}
                        >
                          <td>{formatTime(item.createdAt)}</td>
                          <td>{String(item.parameters?.backtestMode ?? "--")}</td>
                          <td>{String(item.parameters?.symbol ?? "--")}</td>
                          <td>{item.status}</td>
                          <td>{formatPercent(item.resultSummary?.return)}</td>
                          <td>{formatPercent(item.resultSummary?.maxDrawdown)}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              ) : (
                <div className="empty-state">No backtests yet</div>
              )}
              <div className="backtest-detail-card">
                <div className="panel-header">
                  <div>
                    <p className="panel-kicker">Strategy Replay</p>
                    <h3>选中回测详情</h3>
                  </div>
                  <div className="range-box range-box-wrap">
                    <span>{selectedBacktest?.status ?? "NO RUN"}</span>
                    <span>{String(selectedBacktest?.parameters?.backtestMode ?? "--")}</span>
                    <button
                      type="button"
                      className="filter-chip"
                      disabled={!selectedBacktest?.parameters?.from || !selectedBacktest?.parameters?.to}
                      onClick={() => {
                        const from = Date.parse(String(selectedBacktest?.parameters?.from ?? ""));
                        const to = Date.parse(String(selectedBacktest?.parameters?.to ?? ""));
                        if (!Number.isFinite(from) || !Number.isFinite(to)) {
                          return;
                        }
                        setChartOverrideRange({
                          from: Math.floor(from / 1000),
                          to: Math.floor(to / 1000),
                          label: "Backtest Window",
                        });
                        setFocusNonce((value) => value + 1);
                      }}
                    >
                      Use Backtest Window
                    </button>
                    <button
                      type="button"
                      className="filter-chip"
                      disabled={!chartOverrideRange}
                      onClick={() => setChartOverrideRange(null)}
                    >
                      Back To Live Window
                    </button>
                    <a
                      className={`filter-chip ${selectedBacktest ? "" : "filter-chip-disabled"}`}
                      href={selectedBacktest ? `${API_BASE}/api/v1/backtests/${selectedBacktest.id}/execution-trades.csv` : undefined}
                    >
                      Export Trades CSV
                    </a>
                  </div>
                </div>
                {selectedBacktest ? (
                  <>
                    <div className="detail-grid">
                      <div className="detail-item">
                        <span>Execution Source</span>
                        <strong>{latestExecutionSource || "--"}</strong>
                      </div>
                      <div className="detail-item">
                        <span>Matched Files</span>
                        <strong>{String(latestBacktestSummary.matchedArchiveFiles ?? "--")}</strong>
                      </div>
                      <div className="detail-item">
                        <span>{previewCountLabel}</span>
                        <strong>{String(latestBacktestSummary.streamPreviewTicks ?? "--")}</strong>
                      </div>
                      <div className="detail-item">
                        <span>{processedCountLabel}</span>
                        <strong>{processedCountValue}</strong>
                      </div>
                      <div className="detail-item">
                        <span>Trade Count</span>
                        <strong>{String(latestBacktestSummary.executionTradeCount ?? "--")}</strong>
                      </div>
                      <div className="detail-item">
                        <span>Closed Trades</span>
                        <strong>{String(latestBacktestSummary.executionClosedCount ?? "--")}</strong>
                      </div>
                      <div className="detail-item">
                        <span>Win Rate</span>
                        <strong>{formatPercent(latestBacktestSummary.executionWinRate)}</strong>
                      </div>
                      <div className="detail-item">
                        <span>Total PnL</span>
                        <strong>{formatSigned(getNumber(latestBacktestSummary.executionRealizedPnL))}</strong>
                      </div>
                    </div>

                    <div className="backtest-breakdown">
                      <h4>Execution Trades</h4>
                      {latestExecutionTrades.length > 0 ? (
                        <SimpleTable
                          columns={["Status", "Source", "Side", "Qty", "Entry", "Exit", "Exit Type", "PnL"]}
                          rows={latestExecutionTrades.map((trade) => [
                            String(trade.status ?? "--"),
                            String(trade.source ?? "--"),
                            String(trade.side ?? "--"),
                            formatMaybeNumber(trade.quantity),
                            `${formatMaybeNumber(trade.entryPrice)} @ ${formatTime(String(trade.entryTime ?? ""))}`,
                            `${formatMaybeNumber(trade.exitPrice)} @ ${formatTime(String(trade.exitTime ?? ""))}`,
                            String(trade.exitType ?? "--"),
                            formatSigned(getNumber(trade.realizedPnL)),
                          ])}
                          emptyMessage="No execution trades"
                        />
                      ) : (
                        <div className="empty-state empty-state-compact">No execution trades yet</div>
                      )}
                    </div>

                    {Boolean(latestBacktestSummary.replayLedgerTrades) ? (
                      <>
                        <div className="backtest-breakdown">
                          <h4>Optional Ledger Audit</h4>
                          {Object.keys(latestReplayByReason).length > 0 ? (
                            <SimpleTable
                              columns={["Reason", "Trades", "Completed", "Skipped", "Entry", "Exit"]}
                              rows={Object.entries(latestReplayByReason).map(([reason, stats]) => [
                                reason,
                                String(stats.trades ?? 0),
                                String(stats.completed ?? 0),
                                String(stats.skipped ?? 0),
                                String(stats.skippedEntry ?? 0),
                                String(stats.skippedExit ?? 0),
                              ])}
                              emptyMessage="No grouped replay stats"
                            />
                          ) : (
                            <div className="empty-state empty-state-compact">No optional ledger audit data</div>
                          )}
                        </div>

                        <div className="backtest-samples-grid">
                          <div className="backtest-sample-panel">
                            <h4>Completed Samples</h4>
                            {latestReplayCompletedSamples.length > 0 ? (
                              latestReplayCompletedSamples.map((sample, index) => (
                                <SampleCard
                                  key={`completed-${index}`}
                                  sample={sample}
                                  selected={selectedSample?.key === buildSampleKey("completed", index, sample)}
                                  onSelect={() => {
                                    const range = buildSampleRange(sample);
                                    if (!range) {
                                      return;
                                    }
                                    setSelectedSample({ key: buildSampleKey("completed", index, sample), sample });
                                    setChartOverrideRange(range);
                                    setSourceFilter("backtest");
                                    setEventFilter("all");
                                    setFocusNonce((value) => value + 1);
                                  }}
                                />
                              ))
                            ) : (
                              <div className="empty-state empty-state-compact">No completed samples</div>
                            )}
                          </div>
                          <div className="backtest-sample-panel">
                            <h4>Skipped Samples</h4>
                            {latestReplaySkippedSamples.length > 0 ? (
                              latestReplaySkippedSamples.map((sample, index) => (
                                <SampleCard
                                  key={`skipped-${index}`}
                                  sample={sample}
                                  selected={selectedSample?.key === buildSampleKey("skipped", index, sample)}
                                  onSelect={() => {
                                    const range = buildSampleRange(sample);
                                    if (!range) {
                                      return;
                                    }
                                    setSelectedSample({ key: buildSampleKey("skipped", index, sample), sample });
                                    setChartOverrideRange(range);
                                    setSourceFilter("backtest");
                                    setEventFilter("all");
                                    setFocusNonce((value) => value + 1);
                                  }}
                                />
                              ))
                            ) : (
                              <div className="empty-state empty-state-compact">No skipped samples</div>
                            )}
                          </div>
                        </div>
                      </>
                    ) : null}
                  </>
                ) : (
                  <div className="empty-state empty-state-compact">No backtest detail yet</div>
                )}
              </div>
            </div>
          </div>
        </section>

        <section id="paper" className="panel panel-session">
          <div className="panel-header">
            <div>
              <p className="panel-kicker">Paper Session</p>
              <h3>模拟盘运行控制</h3>
            </div>
            {primarySession ? (
              <div className={`session-badge session-${primarySession.status.toLowerCase()}`}>
                {primarySession.status}
              </div>
            ) : null}
          </div>
          <div className="backtest-form session-form">
            <div className="form-grid">
              <label className="form-field">
                <span>Paper Account</span>
                <select
                  value={paperForm.accountId}
                  onChange={(event) => setPaperForm((current) => ({ ...current, accountId: event.target.value }))}
                >
                  {paperAccounts.map((account) => (
                    <option key={account.accountId} value={account.accountId}>
                      {account.accountName} ({account.accountId})
                    </option>
                  ))}
                </select>
              </label>
              <label className="form-field">
                <span>Strategy</span>
                <select
                  value={paperForm.strategyId}
                  onChange={(event) => setPaperForm((current) => ({ ...current, strategyId: event.target.value }))}
                >
                  {strategies.map((strategy) => (
                    <option key={strategy.id} value={strategy.id}>
                      {strategy.name}
                    </option>
                  ))}
                </select>
              </label>
              <label className="form-field">
                <span>Start Equity</span>
                <input
                  value={paperForm.startEquity}
                  onChange={(event) => setPaperForm((current) => ({ ...current, startEquity: event.target.value }))}
                />
              </label>
              <label className="form-field">
                <span>Symbol</span>
                <input value={paperForm.symbol} onChange={(event) => setPaperForm((current) => ({ ...current, symbol: event.target.value.toUpperCase() }))} />
              </label>
              <label className="form-field">
                <span>Signal Timeframe</span>
                <select
                  value={paperForm.signalTimeframe}
                  onChange={(event) => setPaperForm((current) => ({ ...current, signalTimeframe: event.target.value }))}
                >
                  {(backtestOptions?.signalTimeframes ?? ["4h", "1d"]).map((item) => (
                    <option key={item} value={item}>
                      {item}
                    </option>
                  ))}
                </select>
              </label>
              <label className="form-field">
                <span>Execution Source</span>
                <select
                  value={paperForm.executionDataSource}
                  onChange={(event) => setPaperForm((current) => ({ ...current, executionDataSource: event.target.value }))}
                >
                  {(backtestOptions?.executionDataSources ?? ["tick", "1min"]).map((item) => (
                    <option key={item} value={item}>
                      {item}
                    </option>
                  ))}
                </select>
              </label>
              <label className="form-field">
                <span>From (RFC3339)</span>
                <input value={paperForm.from} onChange={(event) => setPaperForm((current) => ({ ...current, from: event.target.value }))} />
              </label>
              <label className="form-field">
                <span>To (RFC3339)</span>
                <input value={paperForm.to} onChange={(event) => setPaperForm((current) => ({ ...current, to: event.target.value }))} />
              </label>
              <label className="form-field">
                <span>Trading Fee (bps)</span>
                <input
                  value={paperForm.tradingFeeBps}
                  onChange={(event) => setPaperForm((current) => ({ ...current, tradingFeeBps: event.target.value }))}
                />
              </label>
              <label className="form-field">
                <span>Funding Rate (bps)</span>
                <input
                  value={paperForm.fundingRateBps}
                  onChange={(event) => setPaperForm((current) => ({ ...current, fundingRateBps: event.target.value }))}
                />
              </label>
              <label className="form-field">
                <span>Funding Interval (hours)</span>
                <input
                  value={paperForm.fundingIntervalHours}
                  onChange={(event) => setPaperForm((current) => ({ ...current, fundingIntervalHours: event.target.value }))}
                />
              </label>
            </div>
            <div className="backtest-actions">
              <ActionButton
                label={paperCreateAction ? "Creating..." : "Create Paper Session"}
                disabled={paperCreateAction || !paperForm.accountId || !paperForm.strategyId}
                onClick={createPaperSession}
              />
            </div>
          </div>
          {primarySession ? (
            <div className="session-layout">
              <div className="session-meta">
                <div className="session-stat">
                  <span>Session ID</span>
                  <strong>{shrink(primarySession.id)}</strong>
                </div>
                <div className="session-stat">
                  <span>Strategy</span>
                  <strong>{shrink(primarySession.strategyId)}</strong>
                </div>
                <div className="session-stat">
                  <span>Started Equity</span>
                  <strong>{formatMoney(primarySession.startEquity)}</strong>
                </div>
                <div className="session-stat">
                  <span>Plan Progress</span>
                  <strong>
                    {String(Math.trunc(getNumber(primarySession.state?.planIndex) ?? 0))} /{" "}
                    {String(Math.trunc(getNumber(primarySession.state?.planLength) ?? 0))}
                  </strong>
                </div>
                <div className="session-stat">
                  <span>Signal / Execution</span>
                  <strong>
                    {String(primarySession.state?.signalTimeframe ?? "--")} / {String(primarySession.state?.executionDataSource ?? "--")}
                  </strong>
                </div>
                <div className="session-stat">
                  <span>Trading / Funding</span>
                  <strong>
                    {formatMaybeNumber(primarySession.state?.tradingFeeBps)} bps /{" "}
                    {formatMaybeNumber(primarySession.state?.fundingRateBps)} bps
                  </strong>
                </div>
                <div className="session-stat">
                  <span>Last Event</span>
                  <strong>{String(primarySession.state?.lastEventReason ?? "--")}</strong>
                </div>
                <div className="session-stat">
                  <span>Signal Events / Sources</span>
                  <strong>
                    {String(Math.trunc(getNumber(primarySession.state?.signalEventCount) ?? 0))} / {String(Object.keys(primarySessionSourceStates).length)}
                  </strong>
                </div>
                <div className="session-stat">
                  <span>Eval Trigger</span>
                  <strong>
                    {String(primarySessionTriggerSource.streamType ?? "--")} · {String(primarySessionTriggerSource.role ?? "--")}
                  </strong>
                </div>
                <div className="session-stat">
                  <span>Eval Status</span>
                  <strong>{String(primarySession.state?.lastStrategyEvaluationStatus ?? "--")}</strong>
                </div>
                <div className="session-stat">
                  <span>Source Gate</span>
                  <strong>
                    {boolLabel(primarySessionSourceGate.ready)} · miss {String(Math.trunc(getNumber(primarySessionSourceGate.missing?.length) ?? 0))} · stale{" "}
                    {String(Math.trunc(getNumber(primarySessionSourceGate.stale?.length) ?? 0))}
                  </strong>
                </div>
                <div className="session-stat">
                  <span>Created</span>
                  <strong>{formatTime(primarySession.createdAt)}</strong>
                </div>
              </div>
              <div className="session-actions">
                <ActionButton
                  label="Start"
                  disabled={sessionAction !== null || primarySession.status === "RUNNING"}
                  onClick={() => runSessionAction(primarySession.id, "start")}
                />
                <ActionButton
                  label="Tick"
                  disabled={sessionAction !== null}
                  onClick={() => runSessionAction(primarySession.id, "tick")}
                />
                <ActionButton
                  label="Stop"
                  variant="ghost"
                  disabled={sessionAction !== null || primarySession.status === "STOPPED"}
                  onClick={() => runSessionAction(primarySession.id, "stop")}
                />
              </div>
            </div>
          ) : (
            <div className="empty-state">No paper session yet</div>
          )}
        </section>

        <section id="live" className="panel panel-session">
          <div className="panel-header">
            <div>
              <p className="panel-kicker">Live Trading</p>
              <h3>实盘账户与订单同步</h3>
            </div>
          </div>
          <div className="live-grid">
            <div className="backtest-form session-form">
              <h4>Create Live Account</h4>
              <div className="form-grid">
                <label className="form-field">
                  <span>Name</span>
                  <input value={liveAccountForm.name} onChange={(event) => setLiveAccountForm((current) => ({ ...current, name: event.target.value }))} />
                </label>
                <label className="form-field">
                  <span>Exchange</span>
                  <input value={liveAccountForm.exchange} onChange={(event) => setLiveAccountForm((current) => ({ ...current, exchange: event.target.value }))} />
                </label>
              </div>
              <div className="backtest-actions">
                <ActionButton label={liveCreateAction ? "Creating..." : "Create Live Account"} disabled={liveCreateAction} onClick={createLiveAccount} />
              </div>
            </div>

            <div className="backtest-form session-form">
              <h4>Bind Live Adapter</h4>
              <div className="form-grid">
                <label className="form-field">
                  <span>Live Account</span>
                  <select
                    value={liveBindingForm.accountId}
                    onChange={(event) => setLiveBindingForm((current) => ({ ...current, accountId: event.target.value }))}
                  >
                    {liveAccounts.map((account) => (
                      <option key={account.id} value={account.id}>
                        {account.name} ({account.status})
                      </option>
                    ))}
                  </select>
                </label>
                <label className="form-field">
                  <span>Adapter</span>
                  <select
                    value={liveBindingForm.adapterKey}
                    onChange={(event) => setLiveBindingForm((current) => ({ ...current, adapterKey: event.target.value }))}
                  >
                    {liveAdapters.map((adapter) => (
                      <option key={adapter.key} value={adapter.key}>
                        {adapter.name}
                      </option>
                    ))}
                  </select>
                </label>
                <label className="form-field">
                  <span>Position Mode</span>
                  <select
                    value={liveBindingForm.positionMode}
                    onChange={(event) => setLiveBindingForm((current) => ({ ...current, positionMode: event.target.value }))}
                  >
                    <option value="ONE_WAY">ONE_WAY</option>
                    <option value="HEDGE">HEDGE</option>
                  </select>
                </label>
                <label className="form-field">
                  <span>Margin Mode</span>
                  <select
                    value={liveBindingForm.marginMode}
                    onChange={(event) => setLiveBindingForm((current) => ({ ...current, marginMode: event.target.value }))}
                  >
                    <option value="CROSSED">CROSSED</option>
                    <option value="ISOLATED">ISOLATED</option>
                  </select>
                </label>
                <label className="form-field">
                  <span>API Key Ref</span>
                  <input value={liveBindingForm.apiKeyRef} onChange={(event) => setLiveBindingForm((current) => ({ ...current, apiKeyRef: event.target.value }))} />
                </label>
                <label className="form-field">
                  <span>API Secret Ref</span>
                  <input value={liveBindingForm.apiSecretRef} onChange={(event) => setLiveBindingForm((current) => ({ ...current, apiSecretRef: event.target.value }))} />
                </label>
                <label className="form-field form-field-checkbox">
                  <span>Sandbox</span>
                  <input
                    type="checkbox"
                    checked={liveBindingForm.sandbox}
                    onChange={(event) => setLiveBindingForm((current) => ({ ...current, sandbox: event.target.checked }))}
                  />
                </label>
              </div>
              <div className="backtest-actions">
                <ActionButton label={liveBindAction ? "Binding..." : "Bind Live Adapter"} disabled={liveBindAction || !liveBindingForm.accountId} onClick={bindLiveAccount} />
              </div>
            </div>
          </div>

          <div className="live-grid">
            <div className="backtest-list">
              <h4>Live Accounts</h4>
              {liveAccounts.length > 0 ? (
                <div className="live-card-list">
                  {liveAccounts.map((account) => {
                    const binding = (account.metadata?.liveBinding as Record<string, unknown> | undefined) ?? {};
                    return (
                      <div key={account.id} className="session-stat">
                        <span>{account.name}</span>
                        <strong>{account.status}</strong>
                        <div className="live-account-meta">
                          <span>{account.exchange}</span>
                          <span>{String(binding.adapterKey ?? "--")}</span>
                          <span>{String(binding.positionMode ?? "--")} / {String(binding.marginMode ?? "--")}</span>
                        </div>
                      </div>
                    );
                  })}
                </div>
              ) : (
                <div className="empty-state empty-state-compact">No live accounts yet</div>
              )}
            </div>

            <div className="backtest-list">
              <h4>Accepted Live Orders</h4>
              {syncableLiveOrders.length > 0 ? (
                <SimpleTable
                  columns={["Order", "Account", "Symbol", "Side", "Qty", "Status", "Action"]}
                  rows={syncableLiveOrders.map((order) => [
                    shrink(order.id),
                    order.accountId,
                    order.symbol,
                    order.side,
                    formatMaybeNumber(order.quantity),
                    order.status,
                    <ActionButton
                      key={order.id}
                      label={liveSyncAction === order.id ? "Syncing..." : "Sync"}
                      disabled={liveSyncAction !== null}
                      onClick={() => syncLiveOrder(order.id)}
                    />,
                  ])}
                  emptyMessage="No accepted live orders"
                />
              ) : (
                <div className="empty-state empty-state-compact">No accepted live orders</div>
              )}
            </div>
          </div>
        </section>

        <section id="market" className="panel panel-market">
          <div className="panel-header">
            <div>
              <p className="panel-kicker">Market Replay</p>
              <h3>BTCUSDT 1 分钟 K 线与开平仓标记</h3>
            </div>
            <div className="range-box">
              <span>{candles.length} bars</span>
              <span>{chartAnnotations.length} markers</span>
              <span>{chartOverrideRange?.label ?? timeWindow}</span>
              <span>{candleRange.label}</span>
            </div>
          </div>
          <div className="chart-shell chart-shell-market">
            {candles.length > 0 ? (
              <TradingChart
                candles={candles}
                annotations={chartAnnotations}
                focusTime={selectedAnnotationFocusTime ?? latestVisibleAnnotationTime}
                focusNonce={focusNonce}
                selectedAnnotationIds={selectedAnnotationIds}
                onSelectAnnotation={(annotation) => {
                  const matchedSample = selectableSamples.find((item) => annotationMatchesSample(annotation, item.sample));
                  if (!matchedSample) {
                    return;
                  }
                  setSelectedSample({ key: matchedSample.key, sample: matchedSample.sample });
                  setSourceFilter("backtest");
                  setEventFilter("all");
                  const range = buildSampleRange(matchedSample.sample);
                  if (range) {
                    setChartOverrideRange(range);
                  }
                  setFocusNonce((value) => value + 1);
                }}
                onHoverMarker={setHoveredMarker}
              />
            ) : (
              <div className="empty-state">No market candles yet</div>
            )}
          </div>
          <div className="filter-row">
            <div className="filter-group filter-group-actions">
              <span className="filter-label">Focus</span>
              <div className="filter-chip-row">
                <button
                  type="button"
                  className="filter-chip"
                  disabled={!latestVisibleAnnotationTime}
                  onClick={() => setFocusNonce((value) => value + 1)}
                >
                  Latest Trade
                </button>
              </div>
            </div>
            <FilterGroup
              label="Window"
              value={timeWindow}
              options={[
                { value: "6h", label: "6h" },
                { value: "12h", label: "12h" },
                { value: "1d", label: "1d" },
                { value: "3d", label: "3d" },
              ]}
              onChange={(value) => setTimeWindow(value as TimeWindow)}
            />
            <FilterGroup
              label="Source"
              value={sourceFilter}
              options={[
                { value: "all", label: "All" },
                { value: "paper", label: "Paper" },
                { value: "backtest", label: "Backtest" },
              ]}
              onChange={(value) => setSourceFilter(value as SourceFilter)}
            />
            <FilterGroup
              label="Event"
              value={eventFilter}
              options={[
                { value: "all", label: "All" },
                { value: "initial", label: "Initial" },
                { value: "reentry", label: "Reentry" },
                { value: "pt", label: "PT" },
                { value: "sl", label: "SL" },
              ]}
              onChange={(value) => setEventFilter(value as EventFilter)}
            />
          </div>
          <div className="marker-legend">
            {markerLegend.map((item) => (
              <div key={item.label} className="legend-item">
                <span className="legend-dot" style={{ backgroundColor: item.color }} />
                <span>{item.label}</span>
              </div>
            ))}
          </div>
          <div className="detail-card">
            <p className="panel-kicker">Marker Detail</p>
            {markerDetail ? (
              <div className="detail-grid">
                <div className="detail-item">
                  <span>Label</span>
                  <strong>{markerDetail.label}</strong>
                </div>
                <div className="detail-item">
                  <span>Source</span>
                  <strong>{markerDetail.source.toUpperCase()}</strong>
                </div>
                <div className="detail-item">
                  <span>Type</span>
                  <strong>{markerDetail.type}</strong>
                </div>
                <div className="detail-item">
                  <span>Price</span>
                  <strong>{formatMoney(markerDetail.price)}</strong>
                </div>
                <div className="detail-item">
                  <span>Time</span>
                  <strong>{formatTime(markerDetail.time)}</strong>
                </div>
                <div className="detail-item">
                  <span>Paper Session</span>
                  <strong>{markerDetail.paperSession ? shrink(markerDetail.paperSession) : "--"}</strong>
                </div>
              </div>
            ) : (
              <div className="empty-state empty-state-compact">Move over the chart to inspect a trade marker</div>
            )}
          </div>
          <div className="snapshot-strip">
            {chartAnnotations.slice(-4).map((item) => (
              <div key={item.id} className={`snapshot-item snapshot-item-${annotationTone(item)}`}>
                <strong>{item.label}</strong>
                <span>
                  {item.source.toUpperCase()} · {formatMoney(item.price)} · {formatTime(item.time)}
                </span>
              </div>
            ))}
          </div>
        </section>

        <section id="equity" className="panel panel-chart">
          <div className="panel-header">
            <div>
              <p className="panel-kicker">Equity History</p>
              <h3>账户净值曲线</h3>
            </div>
            <div className="range-box">
              <span>Low {formatMoney(chartRange.min)}</span>
              <span>High {formatMoney(chartRange.max)}</span>
            </div>
          </div>
          <div className="chart-shell">
            {snapshots.length > 0 ? (
              <svg viewBox="0 0 560 180" className="equity-chart" preserveAspectRatio="none" role="img">
                <defs>
                  <linearGradient id="equityFill" x1="0" x2="0" y1="0" y2="1">
                    <stop offset="0%" stopColor="rgba(13,108,95,0.28)" />
                    <stop offset="100%" stopColor="rgba(13,108,95,0.02)" />
                  </linearGradient>
                </defs>
                <path d={`${chartPath.area} L 560 180 L 0 180 Z`} fill="url(#equityFill)" />
                <path d={chartPath.line} fill="none" stroke="#0d6c5f" strokeWidth="3" strokeLinejoin="round" strokeLinecap="round" />
              </svg>
            ) : (
              <div className="empty-state">No equity snapshots yet</div>
            )}
          </div>
          <div className="snapshot-strip">
            {snapshots.slice(-4).map((item) => (
              <div key={item.id} className="snapshot-item">
                <strong>{formatMoney(item.netEquity)}</strong>
                <span>{formatTime(item.createdAt)}</span>
              </div>
            ))}
          </div>
        </section>

        <section className="content-grid">
          <article id="positions" className="panel">
            <div className="panel-header">
              <div>
                <p className="panel-kicker">Positions</p>
                <h3>当前持仓</h3>
              </div>
            </div>
            <SimpleTable
              columns={["Symbol", "Side", "Qty", "Entry", "Mark", "PnL"]}
              rows={positions.map((position) => [
                position.symbol,
                position.side,
                formatNumber(position.quantity, 4),
                formatMoney(position.entryPrice),
                formatMoney(position.markPrice),
                formatSigned(
                  position.side === "LONG"
                    ? (position.markPrice - position.entryPrice) * position.quantity
                    : (position.entryPrice - position.markPrice) * position.quantity
                ),
              ])}
              emptyMessage="No open positions"
            />
          </article>

          <article id="orders" className="panel">
            <div className="panel-header">
              <div>
                <p className="panel-kicker">Orders</p>
                <h3>最新订单</h3>
              </div>
            </div>
            <SimpleTable
              columns={["Time", "Symbol", "Side", "Qty", "Price", "Status"]}
              rows={orders
                .slice()
                .reverse()
                .slice(0, 8)
                .map((order) => [
                  formatTime(String(order.metadata?.eventTime ?? order.createdAt)),
                  order.symbol,
                  order.side,
                  formatNumber(order.quantity, 4),
                  formatMoney(order.price),
                  order.status,
                ])}
              emptyMessage="No orders"
            />
          </article>
        </section>

        <section id="fills" className="panel">
          <div className="panel-header">
            <div>
              <p className="panel-kicker">Fills</p>
              <h3>成交流水</h3>
            </div>
          </div>
          <SimpleTable
            columns={["Time", "Order", "Qty", "Price", "Fee"]}
            rows={fills
              .slice()
              .reverse()
              .slice(0, 10)
              .map((fill) => [
                formatTime(fill.createdAt),
                shrink(fill.orderId),
                formatNumber(fill.quantity, 4),
                formatMoney(fill.price),
                formatMoney(fill.fee),
              ])}
            emptyMessage="No fills"
          />
        </section>
      </main>
    </div>
  );
}

function TradingChart(props: {
  candles: ChartCandle[];
  annotations: ChartAnnotation[];
  focusTime?: string;
  focusNonce: number;
  selectedAnnotationIds: string[];
  onSelectAnnotation: (annotation: ChartAnnotation) => void;
  onHoverMarker: (detail: MarkerDetail | null) => void;
}) {
  const containerRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!containerRef.current || props.candles.length === 0) {
      return;
    }

    const chart = createChart(containerRef.current, {
      autoSize: true,
      height: 360,
      layout: {
        background: { type: ColorType.Solid, color: "rgba(255, 251, 242, 0.24)" },
        textColor: "#4f585d",
      },
      grid: {
        vertLines: { color: "rgba(216, 207, 186, 0.35)", style: LineStyle.Dotted },
        horzLines: { color: "rgba(216, 207, 186, 0.35)", style: LineStyle.Dotted },
      },
      crosshair: {
        mode: CrosshairMode.Normal,
      },
      rightPriceScale: {
        borderColor: "rgba(216, 207, 186, 0.9)",
      },
      timeScale: {
        borderColor: "rgba(216, 207, 186, 0.9)",
        timeVisible: true,
        secondsVisible: false,
      },
    });

    const series = chart.addSeries(CandlestickSeries, {
      upColor: "#0e6d60",
      downColor: "#b04a37",
      wickUpColor: "#0e6d60",
      wickDownColor: "#b04a37",
      borderVisible: false,
      priceLineVisible: true,
    });

    series.setData(
      props.candles.map((item) => ({
        time: Math.floor(new Date(item.time).getTime() / 1000),
        open: item.open,
        high: item.high,
        low: item.low,
        close: item.close,
      }))
    );

    const markers = createSeriesMarkers(
      series,
      props.annotations.map((item) => ({
        time: Math.floor(new Date(item.time).getTime() / 1000),
        position: markerPosition(item.type),
        color: markerColor(item, props.selectedAnnotationIds.includes(item.id)),
        shape: markerShape(item.type),
        text: markerText(item, props.selectedAnnotationIds.includes(item.id)),
      }))
    );
    markers.setMarkers(
      props.annotations.map((item) => ({
        time: Math.floor(new Date(item.time).getTime() / 1000),
        position: markerPosition(item.type),
        color: markerColor(item, props.selectedAnnotationIds.includes(item.id)),
        shape: markerShape(item.type),
        text: markerText(item, props.selectedAnnotationIds.includes(item.id)),
      }))
    );

    chart.subscribeCrosshairMove((param) => {
      if (param.time == null) {
        props.onHoverMarker(null);
        return;
      }
      const hoveredTime = Number(param.time);
      if (!Number.isFinite(hoveredTime)) {
        props.onHoverMarker(null);
        return;
      }

      const nearest = findNearestAnnotation(props.annotations, hoveredTime);
      props.onHoverMarker(nearest ? toMarkerDetail(nearest) : null);
    });

    chart.subscribeClick((param) => {
      if (param.time == null) {
        return;
      }
      const clickedTime = Number(param.time);
      if (!Number.isFinite(clickedTime)) {
        return;
      }
      const nearest = findNearestAnnotation(props.annotations, clickedTime);
      if (nearest) {
        props.onSelectAnnotation(nearest);
      }
    });

    if (props.focusTime && props.focusNonce > 0) {
      const focusSeconds = Math.floor(new Date(props.focusTime).getTime() / 1000);
      const firstSeconds = Math.floor(new Date(props.candles[0].time).getTime() / 1000);
      const lastSeconds = Math.floor(new Date(props.candles[props.candles.length - 1].time).getTime() / 1000);
      const span = Math.max(lastSeconds - firstSeconds, 60 * 60);
      const padding = Math.max(Math.floor(span / 6), 30 * 60);
      chart.timeScale().setVisibleRange({
        from: focusSeconds - padding,
        to: focusSeconds + padding,
      });
    } else {
      chart.timeScale().fitContent();
    }
    return () => {
      props.onHoverMarker(null);
      chart.remove();
    };
  }, [
    props.annotations,
    props.candles,
    props.focusNonce,
    props.focusTime,
    props.onHoverMarker,
    props.onSelectAnnotation,
    props.selectedAnnotationIds,
  ]);

  return <div ref={containerRef} className="tv-chart" />;
}

function ActionButton(props: {
  label: string;
  disabled?: boolean;
  variant?: "ghost";
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      className={`action-button ${props.variant === "ghost" ? "action-button-ghost" : ""}`}
      disabled={props.disabled}
      onClick={props.onClick}
    >
      {props.label}
    </button>
  );
}

function FilterGroup(props: {
  label: string;
  value: string;
  options: Array<{ value: string; label: string }>;
  onChange: (value: string) => void;
}) {
  return (
    <div className="filter-group">
      <span className="filter-label">{props.label}</span>
      <div className="filter-chip-row">
        {props.options.map((option) => (
          <button
            key={option.value}
            type="button"
            className={`filter-chip ${props.value === option.value ? "filter-chip-active" : ""}`}
            onClick={() => props.onChange(option.value)}
          >
            {option.label}
          </button>
        ))}
      </div>
    </div>
  );
}

function MetricCard(props: { label: string; value: string; tone?: "accent" }) {
  return (
    <article className={`metric-card ${props.tone === "accent" ? "metric-card-accent" : ""}`}>
      <p>{props.label}</p>
      <strong>{props.value}</strong>
    </article>
  );
}

function SimpleTable(props: { columns: string[]; rows: React.ReactNode[][]; emptyMessage: string }) {
  if (props.rows.length === 0) {
    return <div className="empty-state">{props.emptyMessage}</div>;
  }

  return (
    <div className="table-wrap">
      <table>
        <thead>
          <tr>
            {props.columns.map((column) => (
              <th key={column}>{column}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {props.rows.map((row, index) => (
            <tr key={`row-${index}`}>
              {row.map((cell, cellIndex) => (
                <td key={`cell-${index}-${cellIndex}`}>{cell}</td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function SampleCard(props: { sample: ReplaySample; selected?: boolean; onSelect: () => void }) {
  const sample = props.sample;
  const status = sampleStatus(sample);
  return (
    <button
      type="button"
      className={`sample-card sample-card-button ${props.selected ? "sample-card-selected" : ""}`}
      onClick={props.onSelect}
    >
      <div className="sample-header">
        <div className="sample-title">{String(sample.reason ?? sample.entryCause ?? "sample")}</div>
        <span className={`sample-status sample-status-${status.tone}`}>{status.label}</span>
      </div>
      <div className="sample-line">
        entry: {String(sample.entryTime ?? "--")} · {formatMaybeNumber(sample.entryPrice)}
      </div>
      <div className="sample-line">
        exit: {String(sample.exitTime ?? "--")} · {formatMaybeNumber(sample.exitPrice)}
      </div>
      <div className="sample-line">
        fill: {formatMaybeNumber(sample.bracketEntryFill)} → {formatMaybeNumber(sample.bracketExitPrice)}
      </div>
      <div className="sample-line">
        cause: {String(sample.entryCause ?? "--")} / {String(sample.exitCause ?? sample.bracketExitType ?? "--")}
      </div>
      <div className="sample-line">pnl: {formatMaybeNumber(sample.bracketRealizedPnL)}</div>
    </button>
  );
}

function sampleStatus(sample: ReplaySample) {
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

async function fetchJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, init);
  if (!response.ok) {
    throw new Error(`HTTP ${response.status} for ${path}`);
  }
  return (await response.json()) as T;
}

function buildLinePath(values: number[], width: number, height: number) {
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

function summarizeRange(values: number[]) {
  if (values.length === 0) {
    return { min: 0, max: 0 };
  }
  return {
    min: Math.min(...values),
    max: Math.max(...values),
  };
}

function summarizeTimeRange(values: string[]) {
  if (values.length === 0) {
    return { label: "No data" };
  }
  const start = new Date(values[0]);
  const end = new Date(values[values.length - 1]);
  return {
    label: `${formatShortTime(start)} - ${formatShortTime(end)}`,
  };
}

function filterChartAnnotations(
  items: ChartAnnotation[],
  candles: ChartCandle[],
  sessionID?: string,
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
    if (item.source !== "paper" && item.source !== "backtest" && sourceFilter !== "all") {
      return false;
    }
    if (eventFilter === "all") {
      return item.source === "paper" || item.source === "backtest";
    }
    return matchesEventFilter(item, eventFilter);
  });
}

function matchesEventFilter(item: ChartAnnotation, filter: EventFilter) {
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

function resolveChartAnchor(session?: PaperSession, orders: Order[] = []) {
  const sessionEventTime = typeof session?.state?.lastLedgerTime === "string" ? session.state.lastLedgerTime : undefined;
  if (sessionEventTime) {
    return new Date(sessionEventTime);
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

function buildTimeRange(anchorDate: Date, window: TimeWindow) {
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

function buildSampleRange(sample: ReplaySample): ChartOverrideRange | null {
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

function buildSampleKey(prefix: string, index: number, sample: ReplaySample) {
  return [
    prefix,
    index,
    String(sample.entryTime ?? ""),
    String(sample.exitTime ?? sample.bracketExitTime ?? ""),
    String(sample.entryCause ?? sample.reason ?? ""),
  ].join(":");
}

function annotationMatchesSample(item: ChartAnnotation, sample: ReplaySample) {
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

function findNearestAnnotation(items: ChartAnnotation[], hoveredSeconds: number) {
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

function toMarkerDetail(item: ChartAnnotation): MarkerDetail {
  return {
    id: item.id,
    source: item.source,
    type: item.type,
    label: item.label,
    time: item.time,
    price: item.price,
    reason: typeof item.metadata?.reason === "string" ? item.metadata.reason : undefined,
    paperSession: typeof item.metadata?.paperSession === "string" ? item.metadata.paperSession : undefined,
  };
}

function markerShape(type: string) {
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

function markerPosition(type: string) {
  if (type.includes("entry") || type.includes("buy")) {
    return "belowBar";
  }
  return "aboveBar";
}

function markerColor(item: ChartAnnotation, highlighted = false) {
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

function markerText(item: ChartAnnotation, highlighted = false) {
  return highlighted ? `★ ${item.label}` : item.label;
}

function annotationTone(item: ChartAnnotation) {
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

function formatMoney(value?: number) {
  if (value == null) {
    return "--";
  }
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
    maximumFractionDigits: 2,
  }).format(value);
}

function formatSigned(value?: number) {
  if (value == null) {
    return "--";
  }
  const prefix = value > 0 ? "+" : "";
  return `${prefix}${formatMoney(value)}`;
}

function formatPercent(value?: unknown) {
  const number = typeof value === "number" ? value : Number(value);
  if (!Number.isFinite(number)) {
    return "--";
  }
  return `${number >= 0 ? "+" : ""}${(number * 100).toFixed(2)}%`;
}

function formatNumber(value?: number, digits = 2) {
  if (value == null) {
    return "--";
  }
  return value.toFixed(digits);
}

function formatMaybeNumber(value: unknown) {
  const number = getNumber(value);
  if (number == null) {
    return "--";
  }
  return number.toFixed(2);
}

function formatTime(value: string) {
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return "--";
  }
  return parsed.toLocaleString("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function paperAccountsFromSummaries(items: AccountSummary[]) {
  return items.filter((item) => item.mode === "PAPER");
}

function formatShortTime(value: Date) {
  return value.toLocaleString("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function shrink(value: string) {
  return value.length > 16 ? `${value.slice(0, 8)}...${value.slice(-4)}` : value;
}

function getNumber(value: unknown) {
  if (typeof value === "number") {
    return value;
  }
  if (typeof value === "string") {
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : undefined;
  }
  return undefined;
}

function getRecord(value: unknown): Record<string, unknown> {
  if (value && typeof value === "object" && !Array.isArray(value)) {
    return value as Record<string, unknown>;
  }
  return {};
}

function boolLabel(value: unknown) {
  if (value === true) {
    return "ready";
  }
  if (value === false) {
    return "blocked";
  }
  return "--";
}

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
