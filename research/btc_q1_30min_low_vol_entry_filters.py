#!/usr/bin/env python3
"""BTC Q1 2026 30min low-volatility entry gate research replay.

Research-only script. It evaluates gates that block reentry-window entries
when the ATR-derived stop is too small in bps, or when the signal ATR is in a
low rolling percentile. It does not filter stop-loss exits.
"""

from __future__ import annotations

import argparse
import json
import time
from contextlib import contextmanager
from pathlib import Path

import numpy as np
import pandas as pd

import eth_q1_breakout_t3_shape_compare as replay


DEFAULT_TICK_FILES = [
    "dataset/archive/BTCUSDT-trades-2026-01/BTCUSDT-trades-2026-01.csv",
    "dataset/archive/BTCUSDT-trades-2026-02/BTCUSDT-trades-2026-02.csv",
    "dataset/archive/BTCUSDT-trades-2026-03/BTCUSDT-trades-2026-03.csv",
]

BTC_LIVE_LIKE_REPLAY_KWARGS = {
    "dir2_zero_initial": True,
    "zero_initial_mode": "reentry_window",
    "fixed_slippage": 0.0005,
    "stop_loss_atr": 0.3,
    "stop_mode": "atr",
    "max_trades_per_bar": 2,
    "reentry_size_schedule": [0.20, 0.10],
    "long_reentry_atr": 0.1,
    "short_reentry_atr": 0.0,
    "profit_protect_atr": 1.0,
    "trailing_stop_atr": 0.3,
    "delayed_trailing_activation": 0.5,
    "reentry_anchor_levels": "wick",
    "reentry_trigger_mode": "reclaim",
}

VARIANTS = [
    {
        "name": "baseline",
        "description": "No low-volatility entry gate.",
    },
    {
        "name": "min_stop_bps_6",
        "description": "Require stop_loss_atr * ATR to be at least 6 bps of entry reference price.",
        "min_stop_bps": 6.0,
    },
    {
        "name": "min_stop_bps_8",
        "description": "Require stop_loss_atr * ATR to be at least 8 bps of entry reference price.",
        "min_stop_bps": 8.0,
    },
    {
        "name": "atr_pct_gte_25",
        "description": "Require signal ATR percentile over the rolling 240 signal bars to be at least 25.",
        "atr_percentile_gte": 25.0,
    },
    {
        "name": "atr_pct_gte_35",
        "description": "Require signal ATR percentile over the rolling 240 signal bars to be at least 35.",
        "atr_percentile_gte": 35.0,
    },
    {
        "name": "min_stop_bps_6_atr_pct_gte_25",
        "description": "Require both min stop distance of 6 bps and ATR percentile at least 25.",
        "min_stop_bps": 6.0,
        "atr_percentile_gte": 25.0,
    },
    {
        "name": "min_stop_bps_6_atr_pct_gte_35",
        "description": "Require both min stop distance of 6 bps and ATR percentile at least 35.",
        "min_stop_bps": 6.0,
        "atr_percentile_gte": 35.0,
    },
]


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
            "entry_reasons": {},
            "exit_reasons": {},
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
        "entry_reasons": {str(k): int(v) for k, v in pairs["entry_reason"].value_counts().items()},
        "exit_reasons": {str(k): int(v) for k, v in pairs["exit_reason"].value_counts().items()},
    }


def _delta(base: dict, current: dict) -> dict:
    return {
        "final_balance_delta": round(current["final_balance"] - base["final_balance"], 2),
        "return_pct_delta": round(current["return_pct"] - base["return_pct"], 2),
        "max_dd_pct_delta": round(current["max_dd_pct"] - base["max_dd_pct"], 2),
        "trades_delta": int(current["trades"] - base["trades"]),
        "win_rate_pct_delta": round(current["win_rate_pct"] - base["win_rate_pct"], 2),
        "sharpe_delta": round(current["sharpe"] - base["sharpe"], 2),
    }


def _empty_gate_diagnostics() -> dict:
    return {
        "calls": 0,
        "allowed_calls": 0,
        "rejected_calls": 0,
        "reject_reasons": {},
        "min_observed_stop_bps": None,
        "max_observed_stop_bps": None,
        "min_observed_atr_percentile": None,
        "max_observed_atr_percentile": None,
    }


def _record_range(diagnostics: dict, key_min: str, key_max: str, value: float) -> None:
    if not np.isfinite(value):
        return
    diagnostics[key_min] = value if diagnostics[key_min] is None else min(diagnostics[key_min], value)
    diagnostics[key_max] = value if diagnostics[key_max] is None else max(diagnostics[key_max], value)


@contextmanager
def patched_entry_gate(variant: dict, diagnostics: dict):
    original_resolve_reentry_price = replay._resolve_reentry_price
    min_stop_bps = variant.get("min_stop_bps")
    atr_percentile_gte = variant.get("atr_percentile_gte")

    def resolve_reentry_price(sig, side: str, reentry_anchor_levels: str, reentry_atr: float):
        reentry_price = original_resolve_reentry_price(sig, side, reentry_anchor_levels, reentry_atr)
        diagnostics["calls"] += 1

        atr = float(sig.get("atr", np.nan))
        price_ref = float(reentry_price)
        if not np.isfinite(price_ref) or price_ref <= 0:
            price_ref = float(sig.get("close", np.nan))
        stop_loss_atr = float(replay.COMMON_REPLAY_KWARGS["stop_loss_atr"])
        stop_bps = stop_loss_atr * atr / price_ref * 10000.0 if np.isfinite(atr) and atr > 0 and price_ref > 0 else np.nan
        atr_percentile = float(sig.get("atr_percentile", np.nan))
        _record_range(diagnostics, "min_observed_stop_bps", "max_observed_stop_bps", stop_bps)
        _record_range(diagnostics, "min_observed_atr_percentile", "max_observed_atr_percentile", atr_percentile)

        reject_reasons = []
        if min_stop_bps is not None and (not np.isfinite(stop_bps) or stop_bps < float(min_stop_bps)):
            reject_reasons.append("min_stop_bps")
        if atr_percentile_gte is not None and (
            not np.isfinite(atr_percentile) or atr_percentile < float(atr_percentile_gte)
        ):
            reject_reasons.append("atr_percentile")

        if reject_reasons:
            diagnostics["rejected_calls"] += 1
            for reason in reject_reasons:
                diagnostics["reject_reasons"][reason] = diagnostics["reject_reasons"].get(reason, 0) + 1
            return np.inf if side == "long" else -np.inf

        diagnostics["allowed_calls"] += 1
        return reentry_price

    replay._resolve_reentry_price = resolve_reentry_price
    try:
        yield
    finally:
        replay._resolve_reentry_price = original_resolve_reentry_price


def run_variant(second_bars: pd.DataFrame, signal: pd.DataFrame, variant: dict, initial_balance: float):
    gate_diagnostics = _empty_gate_diagnostics()
    started = time.time()
    with patched_entry_gate(variant, gate_diagnostics):
        ledger, replay_diagnostics = replay.run_second_bar_replay(
            second_bars,
            signal,
            initial_balance=initial_balance,
            breakout_shape="baseline_plus_t3",
            replay_mode="live_intrabar_sma5",
            t3_reentry_size_schedule=[0.20, 0.10],
            t3_cooldown_bars=0,
            t3_quality_filters={"min_sma_atr_separation": 0.25},
            quality_filter_shapes=["t3_swing"],
        )
    pairs = _paired_trades(ledger)
    return {
        "variant": variant["name"],
        "description": variant["description"],
        "params": variant,
        "summary": replay.summarize_run(ledger, initial_balance),
        "pair_diagnostics": _pair_diagnostics(pairs),
        "attribution": replay.summarize_breakout_attribution(ledger),
        "diagnostics": {
            "entry_gate": gate_diagnostics,
            "replay": replay_diagnostics,
        },
        "elapsed_seconds": round(time.time() - started, 2),
        "ledger": ledger,
    }


def write_markdown(summary: dict, output_path: Path) -> None:
    lines = [
        "# BTCUSDT Q1 2026 30min Low-Volatility Entry Gates",
        "",
        "Scope: research-only Python replay. No live or execution path is changed by this report.",
        "",
        "## Setup",
        "",
        f"- Symbol/window: `{summary['symbol']}`, `{summary['start']}` to `{summary['end']}`",
        "- Execution bars: continuous `1s` bars rebuilt from Binance trade archives",
        "- Signal timeframe: `30min`",
        "- Replay mode: `live_intrabar_sma5`, breakout shape `baseline_plus_t3`, t3 SMA/ATR separation `0.25`",
        "- Sizing: `dir2_zero_initial=true`, `zero_initial_mode=reentry_window`, `reentry_size_schedule=[0.20, 0.10]`, `max_trades_per_bar=2`",
        "- Gate point: reentry price resolution, so the gate blocks both Zero-Initial-Reentry and SL-Reentry entries. Stop-loss exits are not filtered.",
        "",
        "## Results",
        "",
        "| Variant | Final Balance | Return | Max DD | Trades | Win Rate | Sharpe | Avg Loss | Worst Loss | Entry Mix | Gate Reject Calls |",
        "|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|",
    ]
    for result in summary["results"]:
        s = result["summary"]
        d = result["pair_diagnostics"]
        gate = result["diagnostics"]["entry_gate"]
        entry_mix = ", ".join(f"{k}:{v}" for k, v in s["entry_reasons"].items())
        lines.append(
            f"| `{result['variant']}` | {s['final_balance']:,.2f} | {s['return_pct']:.2f}% | "
            f"{s['max_dd_pct']:.2f}% | {s['trades']} | {s['win_rate_pct']:.2f}% | {s['sharpe']:.2f} | "
            f"{d['avg_loss_pct']:.4f}% | {d['worst_loss_pct']:.4f}% | `{entry_mix}` | "
            f"{gate['rejected_calls']} |"
        )

    lines.extend(["", "## Delta vs Baseline", ""])
    lines.append("| Variant | Final Delta | Return Delta | Max DD Delta | Trades Delta | Win Delta | Sharpe Delta |")
    lines.append("|---|---:|---:|---:|---:|---:|---:|")
    for result in summary["results"][1:]:
        delta = result["delta_vs_baseline"]
        lines.append(
            f"| `{result['variant']}` | {delta['final_balance_delta']:,.2f} | "
            f"{delta['return_pct_delta']:.2f} pp | {delta['max_dd_pct_delta']:.2f} pp | "
            f"{delta['trades_delta']} | {delta['win_rate_pct_delta']:.2f} pp | {delta['sharpe_delta']:.2f} |"
        )

    lines.extend(["", "## Variants", ""])
    for variant in summary["selected_variants"]:
        lines.append(f"- `{variant['name']}`: {variant['description']}")
    lines.append("")
    output_path.write_text("\n".join(lines), encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="BTC Q1 2026 30min low-volatility entry gate replay")
    parser.add_argument("--tick-files", nargs="+", default=DEFAULT_TICK_FILES)
    parser.add_argument("--start", default="2026-01-01T00:00:00Z")
    parser.add_argument("--end", default="2026-03-31T23:59:59Z")
    parser.add_argument("--initial-balance", type=float, default=100000.0)
    parser.add_argument("--chunksize", type=int, default=2_000_000)
    parser.add_argument("--variants", nargs="+", default=[item["name"] for item in VARIANTS])
    parser.add_argument("--summary-json", default="research/btc_2026_q1_30min_low_vol_entry_filters_summary.json")
    parser.add_argument("--markdown", default="research/20260429_btc_q1_30min_low_vol_entry_filters.md")
    parser.add_argument("--ledger-prefix", default="research/tmp_btc_2026_q1_30min_1s_low_vol_entry_filter")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    replay.COMMON_REPLAY_KWARGS.clear()
    replay.COMMON_REPLAY_KWARGS.update(BTC_LIVE_LIKE_REPLAY_KWARGS)
    start = replay._as_utc_timestamp(args.start)
    end = replay._as_utc_timestamp(args.end)
    second_bars, build_stats = replay.build_continuous_second_bars(args.tick_files, start, end, args.chunksize)
    _, signal = replay.build_signal_frame(second_bars, "30min")

    selected_names = set(args.variants)
    selected_variants = [item for item in VARIANTS if item["name"] in selected_names]
    if not selected_variants:
        raise ValueError(f"no variants selected from {args.variants}")

    results = []
    for variant in selected_variants:
        result = run_variant(second_bars, signal, variant, args.initial_balance)
        ledger_path = Path(f"{args.ledger_prefix}_{variant['name']}_ledger.csv")
        result["ledger"].to_csv(ledger_path, index=False)
        del result["ledger"]
        result["ledger_path"] = str(ledger_path)
        results.append(result)
        s = result["summary"]
        gate = result["diagnostics"]["entry_gate"]
        print(
            f"{variant['name']}: return={s['return_pct']:.2f}% trades={s['trades']} "
            f"win={s['win_rate_pct']:.2f}% max_dd={s['max_dd_pct']:.2f}% "
            f"gate_reject_calls={gate['rejected_calls']} elapsed={result['elapsed_seconds']}s",
            flush=True,
        )

    base_summary = results[0]["summary"]
    for result in results[1:]:
        result["delta_vs_baseline"] = _delta(base_summary, result["summary"])

    output = {
        "symbol": "BTCUSDT",
        "start": start.isoformat(),
        "end": end.isoformat(),
        "build_stats": build_stats,
        "signal_stats": {
            "signal_rows": int(len(signal)),
            "signal_start": signal.index[0].isoformat() if not signal.empty else "",
            "signal_end": signal.index[-1].isoformat() if not signal.empty else "",
            "valid_sma5_rows": int(signal["sma5"].notna().sum()),
            "valid_atr_rows": int(signal["atr"].notna().sum()),
            "valid_atr_percentile_rows": int(signal["atr_percentile"].notna().sum()),
        },
        "replay_kwargs": dict(BTC_LIVE_LIKE_REPLAY_KWARGS),
        "selected_variants": selected_variants,
        "results": results,
        "note": "Research-only comparison. Entry gates patch reentry price resolution and do not alter stop-loss exit behavior.",
    }
    Path(args.summary_json).write_text(json.dumps(output, indent=2, ensure_ascii=False), encoding="utf-8")
    write_markdown(output, Path(args.markdown))


if __name__ == "__main__":
    main()
