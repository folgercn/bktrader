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

const API_BASE = (import.meta.env.VITE_API_BASE as string | undefined) ?? "http://127.0.0.1:8080";

function App() {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [summaries, setSummaries] = useState<AccountSummary[]>([]);
  const [orders, setOrders] = useState<Order[]>([]);
  const [fills, setFills] = useState<Fill[]>([]);
  const [positions, setPositions] = useState<Position[]>([]);
  const [snapshots, setSnapshots] = useState<AccountEquitySnapshot[]>([]);
  const [paperSessions, setPaperSessions] = useState<PaperSession[]>([]);
  const [candles, setCandles] = useState<ChartCandle[]>([]);
  const [annotations, setAnnotations] = useState<ChartAnnotation[]>([]);
  const [sessionAction, setSessionAction] = useState<string | null>(null);
  const [sourceFilter, setSourceFilter] = useState<SourceFilter>("all");
  const [eventFilter, setEventFilter] = useState<EventFilter>("all");
  const [timeWindow, setTimeWindow] = useState<TimeWindow>("12h");
  const [focusNonce, setFocusNonce] = useState(0);
  const [hoveredMarker, setHoveredMarker] = useState<MarkerDetail | null>(null);

  const primaryAccount = summaries[0] ?? null;
  const primarySession = paperSessions[0] ?? null;

  async function loadDashboard() {
    const [summaryData, ordersData, fillsData, positionsData, paperSessionData] = await Promise.all([
      fetchJSON<AccountSummary[]>("/api/v1/account-summaries"),
      fetchJSON<Order[]>("/api/v1/orders"),
      fetchJSON<Fill[]>("/api/v1/fills"),
      fetchJSON<Position[]>("/api/v1/positions"),
      fetchJSON<PaperSession[]>("/api/v1/paper/sessions"),
    ]);

    const anchorDate = resolveChartAnchor(paperSessionData[0], ordersData);
    const range = buildTimeRange(anchorDate, timeWindow);
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
    setOrders(ordersData);
    setFills(fillsData);
    setPositions(positionsData);
    setSnapshots(snapshotData);
    setPaperSessions(paperSessionData);
    setCandles(candleData.candles ?? []);
    setAnnotations(annotationData);
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
  }, [timeWindow]);

  const chartPath = useMemo(() => buildLinePath(snapshots.map((item) => item.netEquity), 560, 180), [snapshots]);
  const chartRange = useMemo(() => summarizeRange(snapshots.map((item) => item.netEquity)), [snapshots]);
  const candleRange = useMemo(() => summarizeTimeRange(candles.map((item) => item.time)), [candles]);
  const chartAnnotations = useMemo(
    () => filterChartAnnotations(annotations, candles, primarySession?.id, sourceFilter, eventFilter),
    [annotations, candles, primarySession?.id, sourceFilter, eventFilter]
  );
  const latestVisibleAnnotationTime = useMemo(
    () => (chartAnnotations.length > 0 ? chartAnnotations[chartAnnotations.length - 1].time : undefined),
    [chartAnnotations]
  );
  const markerDetail = useMemo<MarkerDetail | null>(() => {
    if (hoveredMarker) {
      return hoveredMarker;
    }
    const latest = chartAnnotations[chartAnnotations.length - 1];
    return latest ? toMarkerDetail(latest) : null;
  }, [chartAnnotations, hoveredMarker]);
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
              当前页面直接消费平台 API，展示 paper 账户的权益、成交、持仓，以及基于项目策略账本回放的 BTCUSDT 真实 1 分钟 K 线与开平仓标记。
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
                  <span>Ledger Index</span>
                  <strong>{String(Math.trunc(getNumber(primarySession.state?.ledgerIndex) ?? 0))}</strong>
                </div>
                <div className="session-stat">
                  <span>Last Replay Event</span>
                  <strong>{String(primarySession.state?.lastLedgerReason ?? "--")}</strong>
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

        <section id="market" className="panel panel-market">
          <div className="panel-header">
            <div>
              <p className="panel-kicker">Market Replay</p>
              <h3>BTCUSDT 1 分钟 K 线与开平仓标记</h3>
            </div>
            <div className="range-box">
              <span>{candles.length} bars</span>
              <span>{chartAnnotations.length} markers</span>
              <span>{timeWindow}</span>
              <span>{candleRange.label}</span>
            </div>
          </div>
          <div className="chart-shell chart-shell-market">
            {candles.length > 0 ? (
              <TradingChart
                candles={candles}
                annotations={chartAnnotations}
                focusTime={latestVisibleAnnotationTime}
                focusNonce={focusNonce}
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
        color: markerColor(item),
        shape: markerShape(item.type),
        text: item.label,
      }))
    );
    markers.setMarkers(
      props.annotations.map((item) => ({
        time: Math.floor(new Date(item.time).getTime() / 1000),
        position: markerPosition(item.type),
        color: markerColor(item),
        shape: markerShape(item.type),
        text: item.label,
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
  }, [props.annotations, props.candles, props.focusNonce, props.focusTime, props.onHoverMarker]);

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

function SimpleTable(props: { columns: string[]; rows: string[][]; emptyMessage: string }) {
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
            <tr key={`${row.join("-")}-${index}`}>
              {row.map((cell, cellIndex) => (
                <td key={`${cell}-${cellIndex}`}>{cell}</td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
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

function markerColor(item: ChartAnnotation) {
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

function formatNumber(value?: number, digits = 2) {
  if (value == null) {
    return "--";
  }
  return value.toFixed(digits);
}

function formatTime(value: string) {
  return new Date(value).toLocaleString("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
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

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
