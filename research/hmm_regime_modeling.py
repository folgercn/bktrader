#!/usr/bin/env python3
"""Hidden Markov Model (HMM) for dynamic Regime classification.

Trains an HMM on historical daily features (log return, volatility) to infer
latent market states, and cross-analyzes with existing strategy ledgers.
"""

from __future__ import annotations

import argparse
import json
import time
import warnings
from pathlib import Path

import numpy as np
import pandas as pd
from hmmlearn.hmm import GaussianHMM

import eth_q1_breakout_t3_shape_compare as replay
import btc_eth_2026_jan_apr_impulse_bar_run as base

# Suppress hmmlearn deprecation warnings
warnings.filterwarnings("ignore", category=DeprecationWarning)

DEFAULT_ARCHIVE_ROOT = Path("dataset/archive")


def build_daily_from_ticks(symbol: str, start: pd.Timestamp, end: pd.Timestamp, archive_root: Path, chunksize: int) -> pd.DataFrame:
    """Build daily OHLCV from 1s trade ticks."""
    tick_files = base.monthly_trade_files(symbol, start, end, archive_root)
    second_bars, _ = replay.build_continuous_second_bars(tick_files, start, end, chunksize)
    one_min = second_bars.resample("1min").agg(
        {"open": "first", "high": "max", "low": "min", "close": "last", "volume": "sum"}
    ).dropna()
    daily = one_min.resample("1d").agg(
        {"open": "first", "high": "max", "low": "min", "close": "last", "volume": "sum"}
    ).dropna()
    return daily


def extract_hmm_features(df: pd.DataFrame) -> pd.DataFrame:
    """Extract features for HMM modeling."""
    feats = df.copy()
    
    # 1. Log Return
    feats["log_return"] = np.log(feats["close"] / feats["close"].shift(1))
    
    # 2. Parkinson Volatility (High/Low estimator)
    # Volatility = sqrt( (1 / (4 * ln(2))) * ln(High/Low)^2 )
    feats["parkinson_vol"] = np.sqrt( (1.0 / (4.0 * np.log(2.0))) * (np.log(feats["high"] / feats["low"]))**2 )
    
    # Normalize features (Standardization)
    feats = feats.dropna()
    feats["log_return_norm"] = (feats["log_return"] - feats["log_return"].mean()) / feats["log_return"].std()
    feats["parkinson_vol_norm"] = (feats["parkinson_vol"] - feats["parkinson_vol"].mean()) / feats["parkinson_vol"].std()
    
    return feats


def train_and_predict_hmm(train_df: pd.DataFrame, test_df: pd.DataFrame, n_components: int = 3) -> tuple[GaussianHMM, pd.DataFrame]:
    """Train HMM on train_df and predict states for test_df."""
    
    # Features for training
    X_train = train_df[["log_return_norm", "parkinson_vol_norm"]].values
    
    # Train the HMM
    print(f"Training GaussianHMM with {n_components} components on {len(X_train)} days...")
    model = GaussianHMM(n_components=n_components, covariance_type="full", n_iter=1000, random_state=42)
    model.fit(X_train)
    
    # Analyze the learned states to assign semantic labels (e.g., Bull, Bear, Volatile)
    # We look at the means of the features for each state
    state_means = model.means_
    labels = [""] * n_components
    
    # Simple heuristic to name the states based on means
    # This is just for human readability
    returns = state_means[:, 0]
    vols = state_means[:, 1]
    
    for i in range(n_components):
        r, v = returns[i], vols[i]
        label = ""
        if r > 0.1:
            label += "Bullish"
        elif r < -0.1:
            label += "Bearish"
        else:
            label += "Ranging"
            
        if v > 0.5:
            label += "_HighVol"
        elif v < -0.5:
            label += "_LowVol"
        else:
            label += "_MidVol"
        
        labels[i] = f"State_{i}({label})"
        print(f"  {labels[i]}: MeanReturn={r:.3f}, MeanVol={v:.3f}")

    # Predict on test set
    X_test = test_df[["log_return_norm", "parkinson_vol_norm"]].values
    hidden_states = model.predict(X_test)
    
    result_df = test_df.copy()
    result_df["hmm_state_id"] = hidden_states
    result_df["hmm_regime"] = [labels[s] for s in hidden_states]
    
    return model, result_df


def cross_analyze_trades(
    regime_df: pd.DataFrame,
    ledger_paths: list[str],
    symbol: str,
) -> dict:
    """Tag each trade with its HMM regime and compute conditional stats."""
    regime_series = regime_df.set_index(regime_df.index.normalize())["hmm_regime"]

    all_trades = []
    for path_str in ledger_paths:
        path = Path(path_str)
        if not path.exists():
            continue
        ledger = pd.read_csv(path)
        if ledger.empty:
            continue
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
            if trade_date.tzinfo is None:
                trade_date = trade_date.tz_localize("UTC")
            else:
                trade_date = trade_date.tz_convert("UTC")
                
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

    return result


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="HMM Regime classification analysis")
    parser.add_argument("--symbols", nargs="+", default=["ETHUSDT", "BTCUSDT"])
    parser.add_argument("--train-start", default="2025-01-01T00:00:00Z")
    parser.add_argument("--train-end", default="2025-12-31T23:59:59Z")
    parser.add_argument("--test-start", default="2026-01-01T00:00:00Z")
    parser.add_argument("--test-end", default="2026-04-30T23:59:59Z")
    parser.add_argument("--archive-root", default=str(DEFAULT_ARCHIVE_ROOT))
    parser.add_argument("--chunksize", type=int, default=5_000_000)
    parser.add_argument("--n-states", type=int, default=3, help="Number of hidden states")
    parser.add_argument(
        "--trade-ledgers", nargs="*", default=[],
        help="Paths to trade ledger CSVs for cross-analysis",
    )
    parser.add_argument("--output-prefix", default="research/hmm_regime")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    started = time.time()
    
    train_start = base._as_utc(args.train_start)
    train_end = base._as_utc(args.train_end)
    test_start = base._as_utc(args.test_start)
    test_end = base._as_utc(args.test_end)
    
    all_results = {}

    for symbol in args.symbols:
        print(f"\n{'='*60}\n{symbol}: processing data...", flush=True)
        
        # 1. Build and extract training data
        print(f"  Building Training Data ({train_start.date()} to {train_end.date()})...", flush=True)
        train_daily = build_daily_from_ticks(symbol, train_start, train_end, Path(args.archive_root), args.chunksize)
        train_feats = extract_hmm_features(train_daily)
        
        # 2. Build and extract test data
        print(f"  Building Test Data ({test_start.date()} to {test_end.date()})...", flush=True)
        test_daily = build_daily_from_ticks(symbol, test_start, test_end, Path(args.archive_root), args.chunksize)
        
        # We need to normalize test data using train data stats to avoid data leakage
        test_feats = test_daily.copy().dropna()
        test_feats["log_return"] = np.log(test_feats["close"] / test_feats["close"].shift(1))
        test_feats["parkinson_vol"] = np.sqrt( (1.0 / (4.0 * np.log(2.0))) * (np.log(test_feats["high"] / test_feats["low"]))**2 )
        test_feats = test_feats.dropna()
        
        test_feats["log_return_norm"] = (test_feats["log_return"] - train_feats["log_return"].mean()) / train_feats["log_return"].std()
        test_feats["parkinson_vol_norm"] = (test_feats["parkinson_vol"] - train_feats["parkinson_vol"].mean()) / train_feats["parkinson_vol"].std()
        
        # 3. Train HMM and predict on Test
        model, predicted_test_df = train_and_predict_hmm(train_feats, test_feats, n_components=args.n_states)
        
        # 4. Save Time Series
        csv_path = Path(f"{args.output_prefix}_{symbol}_test_timeseries.csv")
        predicted_test_df.to_csv(csv_path)
        print(f"\n  Saved regime predictions to {csv_path}", flush=True)
        
        # 5. Stats
        counts = predicted_test_df["hmm_regime"].value_counts().to_dict()
        total = len(predicted_test_df)
        print(f"\n{symbol} Test Set Regime Distribution:", flush=True)
        for r, c in counts.items():
            print(f"  {r:30s}: {c:3d} days ({c/total*100:.1f}%)", flush=True)
            
        # 6. Cross Analysis
        symbol_ledgers = [p for p in args.trade_ledgers if symbol in p]
        cross = {}
        if symbol_ledgers:
            print(f"\n{symbol}: cross-analyzing {len(symbol_ledgers)} ledger(s)...", flush=True)
            cross = cross_analyze_trades(predicted_test_df, symbol_ledgers, symbol)
            if cross.get("trades_tagged", 0) > 0:
                print(f"  Tagged trades: {cross['trades_tagged']}", flush=True)
                for k, v in cross.items():
                    if k.startswith("regime_"):
                        print(f"  {k[7:]:30s}: Trades={v['count']:3d}, AvgPnL={v['avg_pnl_pct']:.4f}%, WinRate={v['win_rate']:.1f}%", flush=True)

        all_results[symbol] = {
            "test_days": len(predicted_test_df),
            "state_distribution": counts,
            "cross_analysis": cross
        }

    # Save summary
    summary = {
        "train_period": f"{train_start} to {train_end}",
        "test_period": f"{test_start} to {test_end}",
        "n_states": args.n_states,
        "results": all_results,
        "elapsed_seconds": round(time.time() - started, 2),
    }
    summary_path = Path(f"{args.output_prefix}_summary.json")
    summary_path.write_text(json.dumps(summary, indent=2, ensure_ascii=False, default=str), encoding="utf-8")
    print(f"\nSummary saved: {summary_path}", flush=True)
    print(f"Elapsed: {summary['elapsed_seconds']:.1f}s", flush=True)


if __name__ == "__main__":
    main()
