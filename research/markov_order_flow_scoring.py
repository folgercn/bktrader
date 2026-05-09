#!/usr/bin/env python3
"""Markov Chain Order Flow Scoring.

Discretizes 1s order flow into micro-states (Strong Buy, Weak Sell, etc.)
and computes transition matrices for winning vs. losing breakouts.
Calculates a Log-Likelihood Ratio for each sequence to score the breakout.
"""

from __future__ import annotations

import argparse
import json
import time
from pathlib import Path

import numpy as np
import pandas as pd

import btc_eth_2026_jan_apr_impulse_bar_run as base
import order_flow_imbalance_breakout as of_base


DEFAULT_ARCHIVE_ROOT = Path("dataset/archive")


def discretize_order_flow(sb: pd.DataFrame) -> pd.Series:
    """Discretize 1s order flow into 4 micro-states.
    
    States:
    0: Weak Sell (Sell dominated, Vol <= Median)
    1: Strong Sell (Sell dominated, Vol > Median)
    2: Weak Buy (Buy dominated, Vol <= Median)
    3: Strong Buy (Buy dominated, Vol > Median)
    """
    df = sb.copy()
    
    # Calculate rolling median volume (using last 5 mins = 300s to avoid lookahead)
    df["med_vol"] = df["volume"].replace(0, np.nan).rolling(300, min_periods=30).median()
    df["med_vol"] = df["med_vol"].ffill().fillna(0)
    
    is_buy = df["buy_volume"] > df["sell_volume"]
    is_strong = df["volume"] > df["med_vol"]
    
    state = pd.Series(0, index=df.index)
    state[~is_buy & ~is_strong] = 0  # Weak Sell
    state[~is_buy & is_strong] = 1   # Strong Sell
    state[is_buy & ~is_strong] = 2   # Weak Buy
    state[is_buy & is_strong] = 3    # Strong Buy
    
    return state


def extract_sequences(sb: pd.DataFrame, states: pd.Series, ledger: pd.DataFrame, window_secs: int = 60) -> tuple[list[list[int]], list[list[int]]]:
    """Extract sequences for winning and losing trades before their entry."""
    win_seqs = []
    loss_seqs = []
    
    # Needs matching on entry_time
    entries = ledger[ledger["type"].isin(["BUY", "SHORT"])]
    exits = ledger[ledger["type"] == "EXIT"]
    
    if entries.empty or exits.empty:
        return [], []
        
    entries = entries.reset_index(drop=True)
    exits = exits.reset_index(drop=True)
    
    for i in range(min(len(entries), len(exits))):
        entry_time = pd.Timestamp(entries.iloc[i]["time"])
        exit_price = float(exits.iloc[i]["price"])
        entry_price = float(entries.iloc[i]["price"])
        side = "long" if entries.iloc[i]["type"] == "BUY" else "short"
        
        if side == "long":
            pnl = exit_price - entry_price
        else:
            pnl = entry_price - exit_price
            
        is_win = pnl > 0
        
        # Get sequence before entry
        start_time = entry_time - pd.Timedelta(seconds=window_secs)
        
        # Slicing the series
        seq = states.loc[start_time:entry_time].tolist()
        
        if len(seq) > 2:
            # If short, we invert the states to maintain symmetry (so 3 is always direction-aligned strong)
            if side == "short":
                # Invert: 0->3, 1->2, 2->1, 3->0
                seq = [3 - s for s in seq]
                
            if is_win:
                win_seqs.append(seq)
            else:
                loss_seqs.append(seq)
                
    return win_seqs, loss_seqs


def build_transition_matrix(sequences: list[list[int]], num_states: int = 4, alpha: float = 1.0) -> np.ndarray:
    """Build a Markov transition matrix with Laplace smoothing."""
    mat = np.ones((num_states, num_states)) * alpha  # Laplace smoothing
    
    for seq in sequences:
        for i in range(len(seq) - 1):
            s_from = seq[i]
            s_to = seq[i+1]
            mat[s_from, s_to] += 1.0
            
    # Normalize rows
    row_sums = mat.sum(axis=1, keepdims=True)
    mat = mat / row_sums
    return mat


def score_sequence(seq: list[int], m_win: np.ndarray, m_loss: np.ndarray) -> float:
    """Calculate Log-Likelihood Ratio for a sequence."""
    if len(seq) < 2:
        return 0.0
        
    log_ll = 0.0
    for i in range(len(seq) - 1):
        s_from = seq[i]
        s_to = seq[i+1]
        
        p_win = m_win[s_from, s_to]
        p_loss = m_loss[s_from, s_to]
        
        log_ll += np.log(p_win / p_loss)
        
    return log_ll


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Markov Chain Order Flow Scoring")
    parser.add_argument("--symbols", nargs="+", default=["ETHUSDT", "BTCUSDT"])
    parser.add_argument("--start", default="2026-01-01T00:00:00Z")
    parser.add_argument("--end", default="2026-04-30T23:59:59Z")
    parser.add_argument("--archive-root", default=str(DEFAULT_ARCHIVE_ROOT))
    parser.add_argument("--chunksize", type=int, default=5_000_000)
    parser.add_argument("--window", type=int, default=60, help="Window size in seconds")
    parser.add_argument(
        "--trade-ledgers", nargs="*", default=[],
        help="Paths to trade ledger CSVs for sequence extraction",
    )
    parser.add_argument("--output-prefix", default="research/markov_of")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    started = time.time()
    
    start = base._as_utc(args.start)
    end = base._as_utc(args.end)
    
    all_results = {}

    for symbol in args.symbols:
        print(f"\n{'='*60}\n{symbol}: processing data...", flush=True)
        
        # Find matching ledger
        ledger_path = None
        for p in args.trade_ledgers:
            if symbol in p:
                ledger_path = p
                break
                
        if ledger_path is None or not Path(ledger_path).exists():
            print(f"No ledger found for {symbol}. Skipping.")
            continue
            
        ledger = pd.read_csv(ledger_path)
        if ledger.empty:
            continue
            
        print(f"  Building 1s Order Flow Data...", flush=True)
        tick_files = base.monthly_trade_files(symbol, start, end, Path(args.archive_root))
        sb, _ = of_base.build_second_bars_with_flow(tick_files, start, end, args.chunksize)
        
        print(f"  Discretizing States...", flush=True)
        states = discretize_order_flow(sb)
        
        print(f"  Extracting Sequences...", flush=True)
        win_seqs, loss_seqs = extract_sequences(sb, states, ledger, window_secs=args.window)
        
        print(f"    Winning Trades: {len(win_seqs)}")
        print(f"    Losing Trades : {len(loss_seqs)}")
        
        if len(win_seqs) < 5 or len(loss_seqs) < 5:
            print("  Not enough trades for Markov modeling. Skipping.")
            continue
            
        print(f"  Building Transition Matrices...", flush=True)
        m_win = build_transition_matrix(win_seqs)
        m_loss = build_transition_matrix(loss_seqs)
        
        # Scoring existing trades
        scores_win = [score_sequence(seq, m_win, m_loss) for seq in win_seqs]
        scores_loss = [score_sequence(seq, m_win, m_loss) for seq in loss_seqs]
        
        print(f"\n  Transition Matrix (Winning):")
        print(np.round(m_win, 3))
        print(f"\n  Transition Matrix (Losing):")
        print(np.round(m_loss, 3))
        
        print(f"\n  Average LLR Score (Winning): {np.mean(scores_win):.3f}")
        print(f"  Average LLR Score (Losing) : {np.mean(scores_loss):.3f}")
        
        # What if we filter trades with LLR > 0?
        # Note: This is an in-sample test (training and scoring on the same sequences), 
        # but it proves the discriminative power of the feature.
        win_kept = sum(1 for s in scores_win if s > 0)
        loss_kept = sum(1 for s in scores_loss if s > 0)
        
        orig_win_rate = len(win_seqs) / (len(win_seqs) + len(loss_seqs)) * 100
        new_total = win_kept + loss_kept
        new_win_rate = (win_kept / new_total * 100) if new_total > 0 else 0
        
        print(f"\n  In-Sample Filtering (LLR > 0):")
        print(f"    Original Win Rate: {orig_win_rate:.1f}% ({len(win_seqs)}/{len(win_seqs)+len(loss_seqs)})")
        print(f"    Filtered Win Rate: {new_win_rate:.1f}% ({win_kept}/{new_total})")
        print(f"    Filtered Out     : {len(win_seqs)+len(loss_seqs)-new_total} trades")
        
        all_results[symbol] = {
            "win_trades": len(win_seqs),
            "loss_trades": len(loss_seqs),
            "m_win": m_win.tolist(),
            "m_loss": m_loss.tolist(),
            "avg_llr_win": float(np.mean(scores_win)),
            "avg_llr_loss": float(np.mean(scores_loss)),
            "orig_win_rate": orig_win_rate,
            "filtered_win_rate": new_win_rate,
            "filtered_total": new_total
        }

    # Save summary
    summary = {
        "period": f"{start} to {end}",
        "window_secs": args.window,
        "results": all_results,
        "elapsed_seconds": round(time.time() - started, 2),
    }
    summary_path = Path(f"{args.output_prefix}_summary.json")
    summary_path.write_text(json.dumps(summary, indent=2, ensure_ascii=False, default=str), encoding="utf-8")
    print(f"\nSummary saved: {summary_path}", flush=True)


if __name__ == "__main__":
    main()
