#!/usr/bin/env python3
"""BTC Q1 2026 30min breakout confirmation filters, 1s research replay.

Research-only script. It reuses the existing Python 1s replay engine and
temporarily wraps breakout detection to compare:
- baseline single-observation breakout lock
- ATR-distance breakout threshold
- consecutive 1s breakout confirmation as a multi-tick proxy
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

PRODUCTION_LIKE_REPLAY_KWARGS = {
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

REPLAY_PROFILES = {
    "btc_live_like": PRODUCTION_LIKE_REPLAY_KWARGS,
    "eth_research": dict(replay.COMMON_REPLAY_KWARGS),
}

VARIANTS = [
    {
        "name": "baseline_single_observation",
        "description": "Current research behavior: one 1s high/low crossing can lock the zero-initial window.",
    },
    {
        "name": "margin_0p01atr",
        "description": "Require breakout probe to clear the level by at least 0.01 ATR.",
        "margin_atr": 0.01,
    },
    {
        "name": "margin_0p02atr",
        "description": "Require breakout probe to clear the level by at least 0.02 ATR.",
        "margin_atr": 0.02,
    },
    {
        "name": "margin_0p03atr",
        "description": "Require breakout probe to clear the level by at least 0.03 ATR.",
        "margin_atr": 0.03,
    },
    {
        "name": "confirm_2x_1s",
        "description": "Require two consecutive 1s observations crossing the same breakout level.",
        "confirm_observations": 2,
    },
    {
        "name": "confirm_3x_1s",
        "description": "Require three consecutive 1s observations crossing the same breakout level.",
        "confirm_observations": 3,
    },
    {
        "name": "confirm_2x_1s_margin_0p01atr",
        "description": "Require two consecutive 1s observations and 0.01 ATR clearance.",
        "confirm_observations": 2,
        "margin_atr": 0.01,
    },
]


def _shape_key(side: str, shape_name: str, level: float) -> str:
    if not shape_name or pd.isna(level):
        return ""
    return f"{side}|{shape_name}|{float(level):.8f}"


def _passes_margin(side: str, probe_price: float, level: float, atr: float, margin_atr: float) -> tuple[bool, float, float]:
    if margin_atr <= 0:
        distance = float(probe_price) - float(level)
        if side == "short":
            distance = float(level) - float(probe_price)
        return True, distance, 0.0
    if not np.isfinite(probe_price) or not np.isfinite(level) or not np.isfinite(atr) or atr <= 0:
        return False, 0.0, 0.0
    distance = float(probe_price) - float(level)
    if side == "short":
        distance = float(level) - float(probe_price)
    required = float(margin_atr) * float(atr)
    return distance >= required, distance, required


def _confirm_state(sig: dict, side: str) -> dict:
    key = f"_breakout_confirm_{side}"
    state = sig.get(key)
    if not isinstance(state, dict):
        state = {"key": "", "count": 0}
        sig[key] = state
    return state


def _reset_confirm_state(sig: dict, side: str) -> None:
    if isinstance(sig, dict):
        sig[f"_breakout_confirm_{side}"] = {"key": "", "count": 0}


@contextmanager
def patched_breakout_filters(variant: dict, diagnostics: dict):
    original_long = replay._long_breakout
    original_short = replay._short_breakout
    margin_atr = float(variant.get("margin_atr", 0.0) or 0.0)
    confirm_observations = int(variant.get("confirm_observations", 1) or 1)

    def long_breakout(sig, current_high: float, breakout_shape: str):
        triggered, level, shape_name = original_long(sig, current_high, breakout_shape)
        if not triggered:
            _reset_confirm_state(sig, "long")
            return triggered, level, shape_name

        atr = float(sig.get("atr", np.nan))
        margin_ok, distance, required = _passes_margin("long", current_high, level, atr, margin_atr)
        if not margin_ok:
            diagnostics["margin_rejects"]["long"] += 1
            diagnostics["margin_reject_distance_sum"]["long"] += float(distance)
            diagnostics["margin_required"] = float(required)
            _reset_confirm_state(sig, "long")
            return False, np.nan, ""

        key = _shape_key("long", shape_name, level)
        state = _confirm_state(sig, "long")
        state["count"] = int(state["count"]) + 1 if state.get("key") == key else 1
        state["key"] = key
        diagnostics["max_confirm_count"]["long"] = max(diagnostics["max_confirm_count"]["long"], int(state["count"]))
        if state["count"] < confirm_observations:
            diagnostics["confirm_rejects"]["long"] += 1
            return False, np.nan, ""
        return triggered, level, shape_name

    def short_breakout(sig, current_low: float, breakout_shape: str):
        triggered, level, shape_name = original_short(sig, current_low, breakout_shape)
        if not triggered:
            _reset_confirm_state(sig, "short")
            return triggered, level, shape_name

        atr = float(sig.get("atr", np.nan))
        margin_ok, distance, required = _passes_margin("short", current_low, level, atr, margin_atr)
        if not margin_ok:
            diagnostics["margin_rejects"]["short"] += 1
            diagnostics["margin_reject_distance_sum"]["short"] += float(distance)
            diagnostics["margin_required"] = float(required)
            _reset_confirm_state(sig, "short")
            return False, np.nan, ""

        key = _shape_key("short", shape_name, level)
        state = _confirm_state(sig, "short")
        state["count"] = int(state["count"]) + 1 if state.get("key") == key else 1
        state["key"] = key
        diagnostics["max_confirm_count"]["short"] = max(diagnostics["max_confirm_count"]["short"], int(state["count"]))
        if state["count"] < confirm_observations:
            diagnostics["confirm_rejects"]["short"] += 1
            return False, np.nan, ""
        return triggered, level, shape_name

    replay._long_breakout = long_breakout
    replay._short_breakout = short_breakout
    try:
        yield
    finally:
        replay._long_breakout = original_long
        replay._short_breakout = original_short


def _empty_filter_diagnostics() -> dict:
    return {
        "margin_rejects": {"long": 0, "short": 0},
        "confirm_rejects": {"long": 0, "short": 0},
        "margin_reject_distance_sum": {"long": 0.0, "short": 0.0},
        "margin_required": 0.0,
        "max_confirm_count": {"long": 0, "short": 0},
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


def run_variant(second_bars: pd.DataFrame, signal: pd.DataFrame, variant: dict, initial_balance: float):
    filter_diagnostics = _empty_filter_diagnostics()
    started = time.time()
    with patched_breakout_filters(variant, filter_diagnostics):
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
    return {
        "variant": variant["name"],
        "description": variant["description"],
        "params": variant,
        "summary": replay.summarize_run(ledger, initial_balance),
        "attribution": replay.summarize_breakout_attribution(ledger),
        "diagnostics": {
            "filters": filter_diagnostics,
            "replay": replay_diagnostics,
        },
        "elapsed_seconds": round(time.time() - started, 2),
        "ledger": ledger,
    }


def write_markdown(summary: dict, output_path: Path) -> None:
    lines = [
        f"# {summary['symbol']} Q1 2026 30min Breakout Confirmation Filters",
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
        f"- Replay profile: `{summary['profile']}`",
        f"- Risk params: `stop_loss_atr={summary['replay_kwargs']['stop_loss_atr']}`, "
        f"`trailing_stop_atr={summary['replay_kwargs']['trailing_stop_atr']}`, "
        f"`delayed_trailing_activation={summary['replay_kwargs']['delayed_trailing_activation']}`",
        "- `confirm_2x_1s` / `confirm_3x_1s` are multi-tick proxies: they require consecutive 1s observations to cross the same breakout level.",
        "",
        "## Results",
        "",
        "| Variant | Final Balance | Return | Max DD | Trades | Win Rate | Sharpe | Entry Mix | Filter Rejects |",
        "|---|---:|---:|---:|---:|---:|---:|---|---|",
    ]
    for result in summary["results"]:
        s = result["summary"]
        filters = result["diagnostics"]["filters"]
        entry_mix = ", ".join(f"{k}:{v}" for k, v in s["entry_reasons"].items())
        rejects = (
            f"margin L/S {filters['margin_rejects']['long']}/{filters['margin_rejects']['short']}; "
            f"confirm L/S {filters['confirm_rejects']['long']}/{filters['confirm_rejects']['short']}"
        )
        lines.append(
            f"| `{result['variant']}` | {s['final_balance']:,.2f} | {s['return_pct']:.2f}% | "
            f"{s['max_dd_pct']:.2f}% | {s['trades']} | {s['win_rate_pct']:.2f}% | "
            f"{s['sharpe']:.2f} | `{entry_mix}` | `{rejects}` |"
        )

    lines.extend(["", "## Delta vs Baseline", ""])
    lines.append("| Variant | Final Balance Delta | Return Delta | Max DD Delta | Trades Delta | Win Rate Delta | Sharpe Delta |")
    lines.append("|---|---:|---:|---:|---:|---:|---:|")
    for result in summary["results"][1:]:
        d = result["delta_vs_baseline"]
        lines.append(
            f"| `{result['variant']}` | {d['final_balance_delta']:,.2f} | {d['return_pct_delta']:.2f} pp | "
            f"{d['max_dd_pct_delta']:.2f} pp | {d['trades_delta']} | {d['win_rate_pct_delta']:.2f} pp | "
            f"{d['sharpe_delta']:.2f} |"
        )

    lines.extend(["", "## Breakout Attribution", ""])
    lines.append("| Variant | Shape | Trades | Win Rate | Avg PnL | Net PnL | Worst PnL | Profit Factor |")
    lines.append("|---|---|---:|---:|---:|---:|---:|---:|")
    for result in summary["results"]:
        for shape_name, item in result.get("attribution", {}).items():
            lines.append(
                f"| `{result['variant']}` | `{shape_name}` | {item['trades']} | {item['win_rate_pct']:.2f}% | "
                f"{item['avg_pnl_pct']:.4f}% | {item['net_pnl_value']:,.2f} | "
                f"{item['worst_pnl_pct']:.4f}% | {item['profit_factor']} |"
            )

    lines.extend(["", "## Variants", ""])
    for variant in summary.get("selected_variants", VARIANTS):
        lines.append(f"- `{variant['name']}`: {variant['description']}")
    lines.append("")
    output_path.write_text("\n".join(lines), encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="BTC Q1 2026 30min breakout confirmation filters")
    parser.add_argument("--symbol", default="BTCUSDT")
    parser.add_argument("--profile", choices=sorted(REPLAY_PROFILES), default="btc_live_like")
    parser.add_argument("--tick-files", nargs="+", default=DEFAULT_TICK_FILES)
    parser.add_argument("--start", default="2026-01-01T00:00:00Z")
    parser.add_argument("--end", default="2026-03-31T23:59:59Z")
    parser.add_argument("--initial-balance", type=float, default=100000.0)
    parser.add_argument("--chunksize", type=int, default=2_000_000)
    parser.add_argument("--summary-json", default="research/btc_2026_q1_30min_breakout_confirmation_filters_summary.json")
    parser.add_argument("--markdown", default="research/20260429_btc_q1_30min_breakout_confirmation_filters.md")
    parser.add_argument("--ledger-prefix", default="research/tmp_btc_2026_q1_30min_1s_breakout_confirmation")
    parser.add_argument("--variants", nargs="+", default=[item["name"] for item in VARIANTS])
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    replay_kwargs = dict(REPLAY_PROFILES[args.profile])
    replay.COMMON_REPLAY_KWARGS.clear()
    replay.COMMON_REPLAY_KWARGS.update(replay_kwargs)
    start = replay._as_utc_timestamp(args.start)
    end = replay._as_utc_timestamp(args.end)
    second_bars, build_stats = replay.build_continuous_second_bars(args.tick_files, start, end, args.chunksize)
    _, signal = replay.build_signal_frame(second_bars, "30min")

    selected_variants = [item for item in VARIANTS if item["name"] in set(args.variants)]
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
        print(
            f"{variant['name']}: return={s['return_pct']:.2f}% trades={s['trades']} "
            f"win={s['win_rate_pct']:.2f}% elapsed={result['elapsed_seconds']}s",
            flush=True,
        )

    base_summary = results[0]["summary"]
    for result in results[1:]:
        result["delta_vs_baseline"] = _delta(base_summary, result["summary"])

    summary = {
        "symbol": args.symbol,
        "profile": args.profile,
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
        "replay_kwargs": replay_kwargs,
        "selected_variants": selected_variants,
        "results": results,
        "note": "Research-only comparison. Consecutive 1s confirmation is used as a multi-tick proxy because this research replay operates on rebuilt 1s bars.",
    }
    Path(args.summary_json).write_text(json.dumps(summary, indent=2, ensure_ascii=False), encoding="utf-8")
    write_markdown(summary, Path(args.markdown))


if __name__ == "__main__":
    main()
