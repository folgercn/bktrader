import React, { useEffect, useMemo, useState } from "react";
import ReactDOM from "react-dom/client";
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

const API_BASE = (import.meta.env.VITE_API_BASE as string | undefined) ?? "http://127.0.0.1:8080";

function App() {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [summaries, setSummaries] = useState<AccountSummary[]>([]);
  const [orders, setOrders] = useState<Order[]>([]);
  const [fills, setFills] = useState<Fill[]>([]);
  const [positions, setPositions] = useState<Position[]>([]);
  const [snapshots, setSnapshots] = useState<AccountEquitySnapshot[]>([]);

  const primaryAccount = summaries[0] ?? null;

  useEffect(() => {
    let active = true;

    async function load() {
      try {
        const summaryData = await fetchJSON<AccountSummary[]>("/api/v1/account-summaries");
        const [ordersData, fillsData, positionsData] = await Promise.all([
          fetchJSON<Order[]>("/api/v1/orders"),
          fetchJSON<Fill[]>("/api/v1/fills"),
          fetchJSON<Position[]>("/api/v1/positions"),
        ]);

        let snapshotData: AccountEquitySnapshot[] = [];
        if (summaryData[0]?.accountId) {
          snapshotData = await fetchJSON<AccountEquitySnapshot[]>(
            `/api/v1/account-equity-snapshots?accountId=${encodeURIComponent(summaryData[0].accountId)}`
          );
        }

        if (!active) {
          return;
        }
        setSummaries(summaryData);
        setOrders(ordersData);
        setFills(fillsData);
        setPositions(positionsData);
        setSnapshots(snapshotData);
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
  }, []);

  const chartPath = useMemo(() => buildLinePath(snapshots.map((item) => item.netEquity), 560, 180), [snapshots]);
  const chartRange = useMemo(() => summarizeRange(snapshots.map((item) => item.netEquity)), [snapshots]);

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div>
          <p className="sidebar-label">bkTrader Console</p>
          <h1>Paper Monitor</h1>
        </div>
        <nav>
          <a href="#overview">Overview</a>
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
            <h2>账户监控、净值曲线与执行流水</h2>
            <p className="hero-copy">
              当前页面直接消费平台 API，展示 paper 账户的权益、成交、持仓和净值时间序列，作为实盘监控页的第一版骨架。
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
                formatSigned(position.side === "LONG"
                  ? (position.markPrice - position.entryPrice) * position.quantity
                  : (position.entryPrice - position.markPrice) * position.quantity),
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
                  formatTime(order.createdAt),
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

async function fetchJSON<T>(path: string): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`);
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

function shrink(value: string) {
  return value.length > 16 ? `${value.slice(0, 8)}...${value.slice(-4)}` : value;
}

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
