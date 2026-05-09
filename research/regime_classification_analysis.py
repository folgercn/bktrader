#!/usr/bin/env python3
"""Daily regime classification analysis — pure classification, no trading.

Research-only. Classifies each trading day into one of five regimes
(TrendingUp, TrendingDown, MeanReverting, Volatile, Unclear) and
cross-references with existing impulse bar confirm sweep trade results
to measure regime-conditional PnL.
"""

from __future__ import annotations

import argparse
import json
import time
from pathlib import Path

import numpy as np
import pandas as pd

import eth_q1_breakout_t3_shape_compare as replay
import btc_eth_2026_jan_apr_impulse_bar_run as base


DEFAULT_ARCHIVE_ROOT = Path("dataset/archive")


# ---------------------------------------------------------------------------
# Regime classification
# ---------------------------------------------------------------------------

def classify_regime(daily: pd.DataFrame) -> pd.DataFrame:
    """Classify each day into a regime.

    Rules (first match wins):
    - Volatile:       ATR percentile > 80
    - TrendingUp:     EMA20 > EMA50 AND EMA20 rising 3 consecutive days
    - TrendingDown:   EMA20 < EMA50 AND EMA20 falling 3 consecutive days
    - MeanReverting:  Bollinger %B in [0.2, 0.8] AND ATR percentile < 40
    - Unclear:        otherwise
    """
    df = daily.copy()
    df["ema20"] = df["close"].ewm(span=20, adjust=False, min_periods=20).mean()
    df["ema50"] = df["close"].ewm(span=50, adjust=False, min_periods=50).mean()
    df["ema20_slope"] = df["ema20"] - df["ema20"].shift(1)

    # ATR
    true_range = pd.concat([
        df["high"] - df["low"],
        (df["high"] - df["close"].shift()).abs(),
        (df["low"] - df["close"].shift()).abs(),
    ], axis=1).max(axis=1)
    df["atr"] = true_range.rolling(14).mean()
    df["atr_percentile"] = df["atr"].rolling(90, min_periods=30).apply(
        lambda x: float((x[~np.isnan(x)] <= x[~np.isnan(x)][-1]).mean() * 100.0) if len(x[~np.isnan(x)]) > 0 else np.nan,
        raw=True,
    )

    # Bollinger Bands %B
    bb_sma = df["close"].rolling(20).mean()
    bb_std = df["close"].rolling(20).std()
    df["bb_upper"] = bb_sma + 2.0 * bb_std
    df["bb_lower"] = bb_sma - 2.0 * bb_std
    bb_width = df["bb_upper"] - df["bb_lower"]
    df["bb_pctb"] = (df["close"] - df["bb_lower"]) / bb_width.replace(0, np.nan)

    # EMA20 slope streak
    df["ema20_up"] = df["ema20_slope"] > 0
    df["ema20_down"] = df["ema20_slope"] < 0
    df["ema20_up_streak"] = df["ema20_up"].groupby((~df["ema20_up"]).cumsum()).cumsum()
    df["ema20_down_streak"] = df["ema20_down"].groupby((~df["ema20_down"]).cumsum()).cumsum()

    # Classification
    regimes = []
    for idx, row in df.iterrows():
        atr_pct = row.get("atr_percentile", np.nan)
        if pd.notna(atr_pct) and float(atr_pct) > 80:
            regimes.append("Volatile")
        elif (pd.notna(row.get("ema20")) and pd.notna(row.get("ema50"))
              and float(row["ema20"]) > float(row["ema50"])
              and int(row.get("ema20_up_streak", 0)) >= 3):
            regimes.append("TrendingUp")
        elif (pd.notna(row.get("ema20")) and pd.notna(row.get("ema50"))
              and float(row["ema20"]) < float(row["ema50"])
              and int(row.get("ema20_down_streak", 0)) >= 3):
            regimes.append("TrendingDown")
        elif (pd.notna(row.get("bb_pctb")) and pd.notna(atr_pct)
              and 0.2 <= float(row["bb_pctb"]) <= 0.8
              and float(atr_pct) < 40):
            regimes.append("MeanReverting")
        else:
            regimes.append("Unclear")

    df["regime"] = regimes
    return df


def regime_stats(regime_df: pd.DataFrame) -> dict:
    """Compute distribution statistics for regimes."""
    counts = regime_df["regime"].value_counts().to_dict()
    total = len(regime_df)
    pct = {k: round(v / total * 100, 2) for k, v in counts.items()}

    # Average streak length per regime
    streaks = {}
    current_regime = None
    current_len = 0
    all_streaks: dict[str, list[int]] = {}
    for _, row in regime_df.iterrows():
        r = row["regime"]
        if r == current_regime:
            current_len += 1
        else:
            if current_regime is not None:
                all_streaks.setdefault(current_regime, []).append(current_len)
            current_regime = r
            current_len = 1
    if current_regime is not None:
        all_streaks.setdefault(current_regime, []).append(current_len)

    for regime, lengths in all_streaks.items():
        streaks[regime] = {
            "count": len(lengths),
            "avg_days": round(np.mean(lengths), 1),
            "median_days": round(float(np.median(lengths)), 1),
            "max_days": int(max(lengths)),
        }

    switches = sum(1 for i in range(1, len(regime_df)) if regime_df.iloc[i]["regime"] != regime_df.iloc[i - 1]["regime"])

    return {
        "total_days": total,
        "counts": counts,
        "percentages": pct,
        "streaks": streaks,
        "switches": switches,
        "switch_rate_per_day": round(switches / max(1, total - 1), 4),
    }


# ---------------------------------------------------------------------------
# Cross-analysis with trade ledgers
# ---------------------------------------------------------------------------

def cross_analyze_trades(
    regime_df: pd.DataFrame,
    ledger_paths: list[str],
    symbol: str,
) -> dict:
    """Tag each trade with its regime and compute conditional stats."""
    regime_series = regime_df.set_index(regime_df.index.normalize())["regime"]

    all_trades = []
    for path_str in ledger_paths:
        path = Path(path_str)
        if not path.exists():
            continue
        ledger = pd.read_csv(path)
        if ledger.empty:
            continue
        # Pair entries and exits
        entries = ledger[ledger["type"].isin(["BUY", "SHORT"])].copy()
        exits = ledger[ledger["type"] == "EXIT"].copy()
        if entries.empty or exits.empty:
            continue
        entries = entries.reset_index(drop=True)
        exits = exits.reset_index(drop=True)
        n = min(len(entries), len(exits))
        for i in range(n):
            entry_time = pd.Timestamp(entries.iloc[i]["time"])
            exit_time = pd.Timestamp(exits.iloc[i]["time"])
            entry_price = float(entries.iloc[i]["price"])
            exit_price = float(exits.iloc[i]["price"])
            side = "long" if entries.iloc[i]["type"] == "BUY" else "short"
            if side == "long":
                pnl_pct = (exit_price - entry_price) / entry_price * 100.0
            else:
                pnl_pct = (entry_price - exit_price) / entry_price * 100.0

            trade_date = entry_time.normalize()
            if trade_date.tzinfo is not None:
                trade_date = trade_date.tz_localize(None)
            regime_idx = regime_series.index.get_indexer([trade_date], method="pad")
            if regime_idx[0] >= 0:
                regime = regime_series.iloc[regime_idx[0]]
            else:
                regime = "Unknown"

            all_trades.append({
                "entry_time": str(entry_time),
                "exit_time": str(exit_time),
                "side": side,
                "pnl_pct": round(pnl_pct, 6),
                "regime": regime,
                "source": path.stem,
            })

    if not all_trades:
        return {"trades_tagged": 0}

    trades_df = pd.DataFrame(all_trades)
    result = {"trades_tagged": len(trades_df)}

    # Regime-conditional PnL
    for regime in trades_df["regime"].unique():
        subset = trades_df[trades_df["regime"] == regime]
        result[f"regime_{regime}"] = {
            "count": len(subset),
            "avg_pnl_pct": round(float(subset["pnl_pct"].mean()), 4),
            "median_pnl_pct": round(float(subset["pnl_pct"].median()), 4),
            "total_pnl_pct": round(float(subset["pnl_pct"].sum()), 4),
            "win_rate": round(float((subset["pnl_pct"] > 0).mean()) * 100, 2),
        }

    # What-if: filter to Trending only
    trending = trades_df[trades_df["regime"].isin(["TrendingUp", "TrendingDown"])]
    non_trending = trades_df[~trades_df["regime"].isin(["TrendingUp", "TrendingDown"])]
    result["trending_only"] = {
        "count": len(trending),
        "avg_pnl_pct": round(float(trending["pnl_pct"].mean()), 4) if len(trending) > 0 else 0.0,
        "total_pnl_pct": round(float(trending["pnl_pct"].sum()), 4) if len(trending) > 0 else 0.0,
    }
    result["non_trending"] = {
        "count": len(non_trending),
        "avg_pnl_pct": round(float(non_trending["pnl_pct"].mean()), 4) if len(non_trending) > 0 else 0.0,
        "total_pnl_pct": round(float(non_trending["pnl_pct"].sum()), 4) if len(non_trending) > 0 else 0.0,
    }

    return result


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

def build_daily_from_ticks(symbol: str, start: pd.Timestamp, end: pd.Timestamp, archive_root: Path, chunksize: int) -> pd.DataFrame:
    """Build daily OHLCV from 1min bars (resampled from 1s bars)."""
    tick_files = base.monthly_trade_files(symbol, start, end, archive_root)
    second_bars, _ = replay.build_continuous_second_bars(tick_files, start, end, chunksize)
    one_min = second_bars.resample("1min").agg(
        {"open": "first", "high": "max", "low": "min", "close": "last", "volume": "sum"}
    ).dropna()
    daily = one_min.resample("1d").agg(
        {"open": "first", "high": "max", "low": "min", "close": "last", "volume": "sum"}
    ).dropna()
    return daily


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Daily regime classification analysis")
    parser.add_argument("--symbols", nargs="+", default=["ETHUSDT", "BTCUSDT"])
    parser.add_argument("--start", default="2025-01-01T00:00:00Z")
    parser.add_argument("--end", default="2026-04-30T23:59:59Z")
    parser.add_argument("--archive-root", default=str(DEFAULT_ARCHIVE_ROOT))
    parser.add_argument("--chunksize", type=int, default=5_000_000)
    parser.add_argument(
        "--trade-ledgers", nargs="*", default=[],
        help="Paths to trade ledger CSVs for cross-analysis",
    )
    parser.add_argument("--output-prefix", default="research/regime_classification")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    started = time.time()
    start = base._as_utc(args.start)
    end = base._as_utc(args.end)
    all_results = {}

    for symbol in args.symbols:
        print(f"\n{'='*60}\n{symbol}: building daily bars...", flush=True)
        daily = build_daily_from_ticks(symbol, start, end, Path(args.archive_root), args.chunksize)
        print(f"{symbol}: {len(daily)} daily bars", flush=True)

        regime_df = classify_regime(daily)
        stats = regime_stats(regime_df)

        print(f"\n{symbol} Regime Distribution:", flush=True)
        for regime, pct in stats["percentages"].items():
            cnt = stats["counts"][regime]
            print(f"  {regime:16s}: {cnt:4d} days ({pct:5.1f}%)", flush=True)
        print(f"  Switches: {stats['switches']} (rate {stats['switch_rate_per_day']:.4f}/day)", flush=True)

        # Save regime time series
        regime_csv_path = Path(f"{args.output_prefix}_{symbol}_regime_timeseries.csv")
        regime_df[["open", "high", "low", "close", "volume", "ema20", "ema50", "atr", "atr_percentile", "bb_pctb", "regime"]].to_csv(regime_csv_path)
        print(f"  Saved: {regime_csv_path}", flush=True)

        # Cross-analyze with trade ledgers
        symbol_ledgers = [p for p in args.trade_ledgers if symbol in p]
        cross = {}
        if symbol_ledgers:
            print(f"\n{symbol}: cross-analyzing {len(symbol_ledgers)} ledger(s)...", flush=True)
            cross = cross_analyze_trades(regime_df, symbol_ledgers, symbol)
            if cross.get("trades_tagged", 0) > 0:
                print(f"  Tagged trades: {cross['trades_tagged']}", flush=True)
                if "trending_only" in cross:
                    t = cross["trending_only"]
                    nt = cross["non_trending"]
                    print(f"  Trending trades: {t['count']}, avg PnL={t['avg_pnl_pct']:.4f}%, total={t['total_pnl_pct']:.4f}%", flush=True)
                    print(f"  Non-trending:    {nt['count']}, avg PnL={nt['avg_pnl_pct']:.4f}%, total={nt['total_pnl_pct']:.4f}%", flush=True)

        all_results[symbol] = {
            "daily_bars": len(daily),
            "first_day": str(daily.index[0]),
            "last_day": str(daily.index[-1]),
            "regime_stats": stats,
            "regime_csv": str(regime_csv_path),
            "cross_analysis": cross,
        }

    # Save summary
    summary = {
        "start": start.isoformat(),
        "end": end.isoformat(),
        "results": all_results,
        "elapsed_seconds": round(time.time() - started, 2),
    }
    summary_path = Path(f"{args.output_prefix}_summary.json")
    summary_path.write_text(json.dumps(summary, indent=2, ensure_ascii=False, default=str), encoding="utf-8")
    print(f"\nSummary saved: {summary_path}", flush=True)
    print(f"Elapsed: {summary['elapsed_seconds']:.1f}s", flush=True)


if __name__ == "__main__":
    main()
