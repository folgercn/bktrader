#!/usr/bin/env python3
"""ETH Q1 2026 30min stop-loss ATR sweep on the current 1s replay path.

Research-only script. It keeps the same replay setup used by
btc_2026_q1_breakout_confirmation_filters.py for the ETH Q1 breakout-filter
run, and changes only stop_loss_atr.
"""

from __future__ import annotations

import argparse
import json
import time
from pathlib import Path

import pandas as pd

import eth_q1_breakout_t3_shape_compare as replay


BASE_REPLAY_KWARGS = dict(replay.COMMON_REPLAY_KWARGS)


def _delta(base: dict, current: dict) -> dict:
    return {
        "final_balance_delta": round(current["final_balance"] - base["final_balance"], 2),
        "return_pct_delta": round(current["return_pct"] - base["return_pct"], 2),
        "max_dd_pct_delta": round(current["max_dd_pct"] - base["max_dd_pct"], 2),
        "trades_delta": int(current["trades"] - base["trades"]),
        "win_rate_pct_delta": round(current["win_rate_pct"] - base["win_rate_pct"], 2),
        "sharpe_delta": round(current["sharpe"] - base["sharpe"], 2),
    }


def _paired_trades(ledger: pd.DataFrame) -> pd.DataFrame:
    rows = []
    open_entry = None
    for _, row in ledger.iterrows():
        if row["type"] in {"BUY", "SHORT"}:
            open_entry = row
            continue
        if row["type"] != "EXIT" or open_entry is None:
            continue

        entry_time = pd.Timestamp(open_entry["time"])
        exit_time = pd.Timestamp(row["time"])
        side_mult = 1.0 if open_entry["type"] == "BUY" else -1.0
        entry_price = float(open_entry["price"])
        exit_price = float(row["price"])
        notional = float(open_entry["notional"])
        pnl_pct = side_mult * (exit_price - entry_price) / entry_price if entry_price > 0 else 0.0
        rows.append(
            {
                "entry_time": entry_time,
                "exit_time": exit_time,
                "entry_type": str(open_entry["type"]),
                "entry_reason": str(open_entry["reason"]),
                "exit_reason": str(row["reason"]),
                "breakout_shape_name": str(open_entry.get("breakout_shape_name", "")),
                "entry_price": entry_price,
                "exit_price": exit_price,
                "notional": notional,
                "pnl_pct": pnl_pct,
                "pnl_value": pnl_pct * notional,
                "hold_seconds": float((exit_time - entry_time).total_seconds()),
            }
        )
        open_entry = None
    return pd.DataFrame(rows)


def _pair_diagnostics(pairs: pd.DataFrame) -> dict:
    if pairs.empty:
        return {
            "paired_trades": 0,
            "avg_loss_pct": 0.0,
            "worst_loss_pct": 0.0,
            "avg_win_pct": 0.0,
            "profit_factor": None,
            "median_hold_seconds": 0.0,
            "avg_hold_seconds": 0.0,
            "exit_reasons": {},
            "entry_reasons": {},
        }

    pnl_pct = pairs["pnl_pct"].astype("float64")
    pnl_value = pairs["pnl_value"].astype("float64")
    losses = pnl_pct[pnl_pct < 0]
    wins = pnl_pct[pnl_pct > 0]
    gross_profit = float(pnl_value[pnl_value > 0].sum())
    gross_loss = abs(float(pnl_value[pnl_value < 0].sum()))
    return {
        "paired_trades": int(len(pairs)),
        "avg_loss_pct": round(float(losses.mean()) * 100, 4) if not losses.empty else 0.0,
        "worst_loss_pct": round(float(losses.min()) * 100, 4) if not losses.empty else 0.0,
        "avg_win_pct": round(float(wins.mean()) * 100, 4) if not wins.empty else 0.0,
        "profit_factor": round(gross_profit / gross_loss, 4) if gross_loss > 0 else None,
        "median_hold_seconds": round(float(pairs["hold_seconds"].median()), 2),
        "avg_hold_seconds": round(float(pairs["hold_seconds"].mean()), 2),
        "exit_reasons": {str(k): int(v) for k, v in pairs["exit_reason"].value_counts().items()},
        "entry_reasons": {str(k): int(v) for k, v in pairs["entry_reason"].value_counts().items()},
    }


def _monthly_summary(pairs: pd.DataFrame) -> list[dict]:
    if pairs.empty:
        return []
    rows = []
    work = pairs.copy()
    work["month"] = work["exit_time"].dt.strftime("%Y-%m")
    for month, group in work.groupby("month", sort=True):
        pnl_value = group["pnl_value"].astype("float64")
        pnl_pct = group["pnl_pct"].astype("float64")
        losses = pnl_pct[pnl_pct < 0]
        gross_profit = float(pnl_value[pnl_value > 0].sum())
        gross_loss = abs(float(pnl_value[pnl_value < 0].sum()))
        rows.append(
            {
                "month": str(month),
                "trades": int(len(group)),
                "win_rate_pct": round(float((pnl_value > 0).mean()) * 100, 2),
                "net_pnl_value": round(float(pnl_value.sum()), 2),
                "avg_pnl_pct": round(float(pnl_pct.mean()) * 100, 4),
                "avg_loss_pct": round(float(losses.mean()) * 100, 4) if not losses.empty else 0.0,
                "worst_loss_pct": round(float(losses.min()) * 100, 4) if not losses.empty else 0.0,
                "profit_factor": round(gross_profit / gross_loss, 4) if gross_loss > 0 else None,
            }
        )
    return rows


def _write_markdown(output: dict, path: Path) -> None:
    lines = [
        "# ETHUSDT Q1 2026 30min Stop-Loss ATR Sweep",
        "",
        "Scope: research-only Python replay. No live or execution path is changed by this report.",
        "",
        "## Setup",
        "",
        f"- Symbol/window: `{output['symbol']}`, `{output['start']}` to `{output['end']}`",
        "- Execution bars: continuous `1s` bars rebuilt from Binance trade archives",
        "- Signal timeframe: `30min`",
        "- Replay mode: `live_intrabar_sma5`, breakout shape `baseline_plus_t3`, t3 SMA/ATR separation `0.25`",
        "- Sizing: `dir2_zero_initial=true`, `zero_initial_mode=reentry_window`, `reentry_size_schedule=[0.20, 0.10]`, `max_trades_per_bar=2`",
        "- Sweep changes only `stop_loss_atr`; `trailing_stop_atr=0.3` and `delayed_trailing_activation=0.5` stay fixed.",
        "",
        "## Results",
        "",
        "| stop_loss_atr | Final Balance | Return | Max DD | Trades | Win Rate | Sharpe | Avg Loss | Worst Loss | Profit Factor | Median Hold |",
        "|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|",
    ]
    for result in output["results"]:
        s = result["summary"]
        d = result["pair_diagnostics"]
        lines.append(
            f"| {result['stop_loss_atr']:.2f} | {s['final_balance']:,.2f} | {s['return_pct']:.2f}% | "
            f"{s['max_dd_pct']:.2f}% | {s['trades']} | {s['win_rate_pct']:.2f}% | {s['sharpe']:.2f} | "
            f"{d['avg_loss_pct']:.4f}% | {d['worst_loss_pct']:.4f}% | {d['profit_factor']} | "
            f"{d['median_hold_seconds']:.0f}s |"
        )

    lines.extend(["", "## Delta vs 0.05 ATR", ""])
    lines.append("| stop_loss_atr | Final Delta | Return Delta | Max DD Delta | Trades Delta | Win Delta | Sharpe Delta |")
    lines.append("|---:|---:|---:|---:|---:|---:|---:|")
    for result in output["results"][1:]:
        d = result["delta_vs_0p05"]
        lines.append(
            f"| {result['stop_loss_atr']:.2f} | {d['final_balance_delta']:,.2f} | "
            f"{d['return_pct_delta']:.2f} pp | {d['max_dd_pct_delta']:.2f} pp | "
            f"{d['trades_delta']} | {d['win_rate_pct_delta']:.2f} pp | {d['sharpe_delta']:.2f} |"
        )
    lines.append("")
    path.write_text("\n".join(lines), encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="ETH Q1 2026 30min stop-loss ATR sweep")
    parser.add_argument("--tick-files", nargs="+", default=replay.DEFAULT_TICK_FILES)
    parser.add_argument("--start", default="2026-01-01T00:00:00Z")
    parser.add_argument("--end", default="2026-03-31T23:59:59Z")
    parser.add_argument("--initial-balance", type=float, default=100000.0)
    parser.add_argument("--chunksize", type=int, default=2_000_000)
    parser.add_argument("--stop-loss-atrs", nargs="+", type=float, default=[0.05, 0.10, 0.20, 0.30, 0.40])
    parser.add_argument("--summary-json", default="research/eth_2026_q1_30min_stop_loss_atr_sweep_summary.json")
    parser.add_argument("--markdown", default="research/20260429_eth_q1_30min_stop_loss_atr_sweep.md")
    parser.add_argument("--ledger-prefix", default="research/tmp_eth_2026_q1_30min_1s_stop_loss_atr_sweep")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    start = replay._as_utc_timestamp(args.start)
    end = replay._as_utc_timestamp(args.end)
    second_bars, build_stats = replay.build_continuous_second_bars(args.tick_files, start, end, args.chunksize)
    _, signal = replay.build_signal_frame(second_bars, "30min")

    results = []
    for stop_loss_atr in args.stop_loss_atrs:
        replay_kwargs = dict(BASE_REPLAY_KWARGS)
        replay_kwargs["stop_loss_atr"] = float(stop_loss_atr)
        replay.COMMON_REPLAY_KWARGS.clear()
        replay.COMMON_REPLAY_KWARGS.update(replay_kwargs)

        started = time.time()
        ledger, diagnostics = replay.run_second_bar_replay(
            second_bars,
            signal,
            initial_balance=args.initial_balance,
            breakout_shape="baseline_plus_t3",
            replay_mode="live_intrabar_sma5",
            t3_reentry_size_schedule=[0.20, 0.10],
            t3_cooldown_bars=0,
            t3_quality_filters={"min_sma_atr_separation": 0.25},
            quality_filter_shapes=["t3_swing"],
        )
        elapsed = round(time.time() - started, 2)
        label = str(stop_loss_atr).replace(".", "p")
        ledger_path = Path(f"{args.ledger_prefix}_{label}_ledger.csv")
        ledger.to_csv(ledger_path, index=False)
        pairs = _paired_trades(ledger)
        result = {
            "stop_loss_atr": float(stop_loss_atr),
            "summary": replay.summarize_run(ledger, args.initial_balance),
            "pair_diagnostics": _pair_diagnostics(pairs),
            "monthly": _monthly_summary(pairs),
            "breakout_attribution": replay.summarize_breakout_attribution(ledger),
            "diagnostics": diagnostics,
            "ledger_path": str(ledger_path),
            "elapsed_seconds": elapsed,
            "replay_kwargs": replay_kwargs,
        }
        results.append(result)
        s = result["summary"]
        d = result["pair_diagnostics"]
        print(
            f"stop_loss_atr={stop_loss_atr:.2f}: return={s['return_pct']:.2f}% "
            f"trades={s['trades']} win={s['win_rate_pct']:.2f}% "
            f"max_dd={s['max_dd_pct']:.2f}% worst_loss={d['worst_loss_pct']:.4f}% "
            f"elapsed={elapsed}s",
            flush=True,
        )

    base_summary = results[0]["summary"]
    for result in results[1:]:
        result["delta_vs_0p05"] = _delta(base_summary, result["summary"])

    output = {
        "symbol": "ETHUSDT",
        "start": start.isoformat(),
        "end": end.isoformat(),
        "build_stats": build_stats,
        "signal_stats": {
            "signal_rows": int(len(signal)),
            "signal_start": signal.index[0].isoformat() if not signal.empty else "",
            "signal_end": signal.index[-1].isoformat() if not signal.empty else "",
            "valid_sma5_rows": int(signal["sma5"].notna().sum()),
            "valid_atr_rows": int(signal["atr"].notna().sum()),
        },
        "results": results,
        "note": "Research-only exact-semantics sweep for the ETH Q1 breakout-filter replay path.",
    }
    Path(args.summary_json).write_text(json.dumps(output, indent=2, ensure_ascii=False), encoding="utf-8")
    _write_markdown(output, Path(args.markdown))


if __name__ == "__main__":
    main()
