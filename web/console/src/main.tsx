import React from "react";
import ReactDOM from "react-dom/client";
import "./styles.css";

function App() {
  return (
    <div className="app-shell">
      <aside className="sidebar">
        <h1>bkTrader</h1>
        <nav>
          <a href="#overview">Overview</a>
          <a href="#strategies">Strategies</a>
          <a href="#accounts">Accounts</a>
          <a href="#orders">Orders</a>
          <a href="#backtests">Backtests</a>
          <a href="#paper">Paper</a>
        </nav>
      </aside>

      <main className="main">
        <section className="hero">
          <div>
            <p className="eyebrow">Signal-driven trading workspace</p>
            <h2>Live, paper, and backtest operations in one console</h2>
            <p>
              TradingView will be used only as a market and annotation surface.
              Order, account, and strategy operations remain inside the platform console.
            </p>
          </div>
        </section>

        <section className="grid">
          <article className="card">
            <h3>Strategy Default</h3>
            <p>1D signal / 1m execution</p>
            <p>Zero initial risk, ATR stop, 10% then 20% reentry sizing</p>
          </article>
          <article className="card">
            <h3>Modules</h3>
            <p>Signals, strategies, accounts, orders, monitoring, paper trading, backtests</p>
          </article>
          <article className="card">
            <h3>Chart Mode</h3>
            <p>Kline display and trade markers only</p>
          </article>
        </section>
      </main>
    </div>
  );
}

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);

