#!/usr/bin/env python3
"""Probabilistic V6 execution-aware event labeler.

Research-only. Replays each event independently through the V4 1s execution
model and rewrites the modeling target so probability models learn execution
profitability instead of first-edge continuation.
"""

from __future__ import annotations

import argparse
import json
import time
from pathlib import Path

import numpy as np
import pandas as pd

import probabilistic_v4_execution_runner as execution


def _as_bool(value: object) -> bool:
    if isinstance(value, bool):
        return value
    return str(value).lower() in {"true", "1", "yes"}


def _label_event(sb: pd.DataFrame, row: pd.Series, args: argparse.Namespace, params: dict) -> dict:
    dwell_col = f"dwell_{int(args.entry_delay_seconds)}s_pass"
    if int(args.entry_delay_seconds) > 0 and dwell_col in row.index and not _as_bool(row[dwell_col]):
        return {
            "execution_tradable": False,
            "execution_skip_reason": "dwell",
            "execution_return_pct": np.nan,
            "execution_win": False,
            "execution_exit_reason": "",
            "execution_mfe_r": np.nan,
            "execution_mae_r": np.nan,
            "execution_hold_seconds": np.nan,
        }

    clean_row = row.copy()
    clean_row["model_notional_share"] = np.nan
    initial_balance = float(args.initial_balance)
    logs, balance, skip_reason = execution._simulate_event(
        sb,
        clean_row,
        balance=initial_balance,
        params=params,
        entry_delay_seconds=int(args.entry_delay_seconds),
    )
    if skip_reason or not logs:
        return {
            "execution_tradable": False,
            "execution_skip_reason": skip_reason or "no_logs",
            "execution_return_pct": np.nan,
            "execution_win": False,
            "execution_exit_reason": "",
            "execution_mfe_r": np.nan,
            "execution_mae_r": np.nan,
            "execution_hold_seconds": np.nan,
        }

    entry = logs[0]
    exit_row = logs[-1]
    entry_time = pd.Timestamp(entry["time"])
    exit_time = pd.Timestamp(exit_row["time"])
    return_pct = (float(balance) / initial_balance - 1.0) * 100.0
    return {
        "execution_tradable": True,
        "execution_skip_reason": "",
        "execution_return_pct": round(return_pct, 8),
        "execution_win": bool(return_pct > 0.0),
        "execution_exit_reason": str(exit_row.get("reason", "")),
        "execution_mfe_r": exit_row.get("mfe_r", np.nan),
        "execution_mae_r": exit_row.get("mae_r", np.nan),
        "execution_hold_seconds": round((exit_time - entry_time).total_seconds(), 2),
    }


def label_events(events: pd.DataFrame, args: argparse.Namespace) -> tuple[pd.DataFrame, dict]:
    params = execution._execution_params(args)
    labeled_frames: list[pd.DataFrame] = []
    per_symbol: dict[str, dict] = {}

    for symbol, symbol_events in events.groupby("symbol", sort=False):
        symbol_events = symbol_events.sort_values("touch_time").copy()
        if symbol_events.empty:
            continue
        start = execution.base._as_utc(args.start)
        end = execution.base._as_utc(args.end)
        sb, build_stats = execution._load_or_build_second_bars(str(symbol), start, end, args)
        rows = []
        for _, row in symbol_events.iterrows():
            rows.append(_label_event(sb, row, args, params))
        labels = pd.DataFrame(rows, index=symbol_events.index)
        merged = pd.concat([symbol_events, labels], axis=1)
        labeled_frames.append(merged)
        per_symbol[str(symbol)] = {
            "events": int(len(symbol_events)),
            "tradable": int(merged["execution_tradable"].sum()),
            "wins": int((merged["execution_tradable"] & merged["execution_win"]).sum()),
            "avg_execution_return_pct": round(float(merged.loc[merged["execution_tradable"], "execution_return_pct"].mean()), 8)
            if bool(merged["execution_tradable"].any())
            else 0.0,
            "skip_reasons": {str(k): int(v) for k, v in merged["execution_skip_reason"].value_counts().items()},
            "build_stats": build_stats,
        }

    labeled = pd.concat(labeled_frames).sort_values("touch_time").reset_index(drop=True) if labeled_frames else events.copy()
    if not args.keep_untradable:
        labeled = labeled[labeled["execution_tradable"]].copy()

    labeled["original_outcome"] = labeled["outcome"]
    labeled["original_first_edge_atr"] = labeled["first_edge_atr"]
    labeled["original_net_first_edge_atr"] = labeled["net_first_edge_atr"]
    labeled["original_roundtrip_cost_atr"] = labeled["roundtrip_cost_atr"]

    labeled["outcome"] = np.where(labeled["execution_win"], "continuation", "fail")
    labeled["first_edge_atr"] = pd.to_numeric(labeled["execution_return_pct"], errors="coerce").fillna(0.0)
    labeled["net_first_edge_atr"] = labeled["first_edge_atr"]
    # Execution return is already fee/slippage-aware; keep the original cost as a feature under
    # original_roundtrip_cost_atr and zero this modeling-cost field to avoid double-counting costs.
    labeled["roundtrip_cost_atr"] = 0.0

    return labeled, per_symbol


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Label events with independent execution outcomes")
    parser.add_argument("--events-csv", required=True)
    parser.add_argument("--output-csv", required=True)
    parser.add_argument("--summary-json", required=True)
    parser.add_argument("--symbols", nargs="+", default=["BTCUSDT", "ETHUSDT"])
    parser.add_argument("--start", required=True)
    parser.add_argument("--end", required=True)
    parser.add_argument("--execute-start", default="")
    parser.add_argument("--execute-end", default="")
    parser.add_argument("--archive-root", default=str(execution.DEFAULT_ARCHIVE_ROOT))
    parser.add_argument("--chunksize", type=int, default=5_000_000)
    parser.add_argument("--bars-cache-dir", default="")
    parser.add_argument("--initial-balance", type=float, default=100000.0)
    parser.add_argument("--entry-delay-seconds", type=int, default=5)
    parser.add_argument("--initial-stop-atr", type=float, default=0.45)
    parser.add_argument("--breakeven-at-r", type=float, default=0.8)
    parser.add_argument("--trail-start-r", type=float, default=0.9)
    parser.add_argument("--trail-buffer-atr", type=float, default=0.05)
    parser.add_argument("--max-hold-hours", type=float, default=4.0)
    parser.add_argument("--notional-share", type=float, default=1.0)
    parser.add_argument("--keep-untradable", action="store_true")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    started = time.time()
    events = pd.read_csv(args.events_csv, parse_dates=["touch_time", "signal_start", "signal_end"])
    events = events[events["symbol"].isin(args.symbols)].copy()
    labeled, per_symbol = label_events(events, args)
    output_path = Path(args.output_csv)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    labeled.to_csv(output_path, index=False)
    summary = {
        "events_csv": args.events_csv,
        "output_csv": str(output_path),
        "rows": int(len(labeled)),
        "per_symbol": per_symbol,
        "execution": {
            "entry_delay_seconds": int(args.entry_delay_seconds),
            "initial_stop_atr": float(args.initial_stop_atr),
            "breakeven_at_r": float(args.breakeven_at_r),
            "trail_start_r": float(args.trail_start_r),
            "max_hold_hours": float(args.max_hold_hours),
            "label_notional_share": float(args.notional_share),
        },
        "elapsed_seconds": round(time.time() - started, 2),
    }
    Path(args.summary_json).write_text(json.dumps(summary, indent=2, ensure_ascii=False, default=str), encoding="utf-8")
    print(json.dumps({"output_csv": str(output_path), "rows": len(labeled), "summary_json": args.summary_json}, indent=2), flush=True)


if __name__ == "__main__":
    main()
