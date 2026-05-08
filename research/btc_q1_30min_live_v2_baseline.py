#!/usr/bin/env python3
"""BTC Q1 2026 30min live-v2 baseline research run.

Research-only script. It keeps the structural breakout and risk-management
logic, but changes reentry fills from planned-price fills to observed event
price fills. Optimization gates are intentionally left out of the new baseline
and documented as future tuning candidates.
"""

from __future__ import annotations

import argparse
import json
import time
from pathlib import Path

import pandas as pd

import eth_q1_breakout_t3_shape_compare as research_engine
from btc_q1_30min_low_vol_entry_filters import (
    BTC_LIVE_LIKE_REPLAY_KWARGS,
    DEFAULT_TICK_FILES,
    _delta,
    _pair_diagnostics,
    _paired_trades,
)


REMOVED_OPTIMIZATION_GATES = [
    "t3_min_sma_atr_separation",
    "reentry_min_stop_bps",
    "reentry_atr_percentile_gte",
    "reentry_close_confirm",
    "reentry_delay_seconds",
    "reentry_actionability_bps",
]


VARIANTS = [
    {
        "name": "legacy_planned_fill_t3_sep_0p25",
        "description": "Historical reference: planned-price fill at re_p, 1s high/low trigger envelope, t3 SMA/ATR separation 0.25.",
        "zero_initial_reentry_anchor_mode": "rolling",
        "reentry_trigger_observation_mode": "bar_extrema",
        "reentry_fill_price_mode": "planned",
        "t3_quality_filters": {"min_sma_atr_separation": 0.25},
        "quality_filter_shapes": ["t3_swing"],
    },
    {
        "name": "live_v2_no_gates_rolling",
        "description": "New baseline candidate: rolling zero-initial anchor, 1s close trigger proxy, fill at observed 1s close, no optimization gates.",
        "zero_initial_reentry_anchor_mode": "rolling",
        "reentry_trigger_observation_mode": "close",
        "reentry_fill_price_mode": "observed",
        "t3_quality_filters": {},
        "quality_filter_shapes": [],
    },
    {
        "name": "live_v2_no_gates_snapshot",
        "description": "New baseline candidate: snapshot zero-initial anchor, 1s close trigger proxy, fill at observed 1s close, no optimization gates.",
        "zero_initial_reentry_anchor_mode": "snapshot",
        "reentry_trigger_observation_mode": "close",
        "reentry_fill_price_mode": "observed",
        "t3_quality_filters": {},
        "quality_filter_shapes": [],
    },
]


def _entry_mix(summary: dict) -> str:
    return ", ".join(f"{k}:{v}" for k, v in summary["entry_reasons"].items())


def _fill_diagnostics(ledger: pd.DataFrame) -> dict:
    if ledger.empty:
        return {
            "entries": 0,
            "zero_initial_entries": 0,
            "avg_entry_price": 0.0,
            "first_entry": "",
            "last_entry": "",
        }
    entries = ledger[ledger["type"].isin(["BUY", "SHORT"])].copy()
    zero_initial = entries[entries["reason"].eq("Zero-Initial-Reentry")]
    return {
        "entries": int(len(entries)),
        "zero_initial_entries": int(len(zero_initial)),
        "avg_entry_price": round(float(entries["price"].mean()), 4) if not entries.empty else 0.0,
        "first_entry": str(pd.Timestamp(entries["time"].iloc[0]).isoformat()) if not entries.empty else "",
        "last_entry": str(pd.Timestamp(entries["time"].iloc[-1]).isoformat()) if not entries.empty else "",
    }


def run_variant(second_bars: pd.DataFrame, signal: pd.DataFrame, variant: dict, initial_balance: float) -> dict:
    started = time.time()
    ledger, diagnostics = research_engine.run_second_bar_replay(
        second_bars,
        signal,
        initial_balance=initial_balance,
        breakout_shape="baseline_plus_t3",
        replay_mode="live_intrabar_sma5",
        t3_reentry_size_schedule=[0.20, 0.10],
        t3_cooldown_bars=0,
        t3_quality_filters=variant["t3_quality_filters"],
        quality_filter_shapes=variant["quality_filter_shapes"],
        zero_initial_reentry_anchor_mode=variant["zero_initial_reentry_anchor_mode"],
        reentry_trigger_observation_mode=variant["reentry_trigger_observation_mode"],
        reentry_fill_price_mode=variant["reentry_fill_price_mode"],
    )
    pairs = _paired_trades(ledger)
    return {
        "variant": variant["name"],
        "description": variant["description"],
        "params": variant,
        "summary": research_engine.summarize_run(ledger, initial_balance),
        "pair_diagnostics": _pair_diagnostics(pairs),
        "fill_diagnostics": _fill_diagnostics(ledger),
        "attribution": research_engine.summarize_breakout_attribution(ledger),
        "diagnostics": diagnostics,
        "elapsed_seconds": round(time.time() - started, 2),
        "ledger": ledger,
    }


def write_markdown(summary: dict, output_path: Path) -> None:
    lines = [
        "# BTCUSDT Q1 2026 30min Live-V2 Baseline",
        "",
        "Scope: research-only Python backtest. No live or execution path is changed by this report.",
        "",
        "## Setup",
        "",
        f"- Symbol/window: `{summary['symbol']}`, `{summary['start']}` to `{summary['end']}`",
        "- Execution source: continuous `1s` bars rebuilt from Binance trade archives",
        "- Signal timeframe: `30min`",
        "- Structural logic retained: `baseline_plus_t3` breakout shape, zero-initial reentry window, SL, trailing stop, and profit protection",
        "- New fill semantics: reentry trigger and fill use the observed `1s close` event-price proxy; fills no longer backfill to `prev_low_1 + ATR` / `prev_high_1 - ATR` planned price",
        "- Optimization gates removed from the new baseline and kept for later sweeps.",
        "",
        "## Removed Optimization Gates",
        "",
    ]
    for gate in REMOVED_OPTIMIZATION_GATES:
        lines.append(f"- `{gate}`")

    lines.extend(
        [
            "",
            "## Results",
            "",
            "| Variant | Final Balance | Return | Max DD | Trades | Win Rate | Sharpe | Avg Loss | Worst Loss | Entry Mix |",
            "|---|---:|---:|---:|---:|---:|---:|---:|---:|---|",
        ]
    )
    for result in summary["results"]:
        s = result["summary"]
        d = result["pair_diagnostics"]
        lines.append(
            f"| `{result['variant']}` | {s['final_balance']:,.2f} | {s['return_pct']:.2f}% | "
            f"{s['max_dd_pct']:.2f}% | {s['trades']} | {s['win_rate_pct']:.2f}% | {s['sharpe']:.2f} | "
            f"{d['avg_loss_pct']:.4f}% | {d['worst_loss_pct']:.4f}% | `{_entry_mix(s)}` |"
        )

    lines.extend(["", "## Delta vs Legacy", ""])
    lines.append("| Variant | Final Delta | Return Delta | Max DD Delta | Trades Delta | Win Delta | Sharpe Delta |")
    lines.append("|---|---:|---:|---:|---:|---:|---:|")
    for result in summary["results"][1:]:
        delta = result["delta_vs_legacy"]
        lines.append(
            f"| `{result['variant']}` | {delta['final_balance_delta']:,.2f} | "
            f"{delta['return_pct_delta']:.2f} pp | {delta['max_dd_pct_delta']:.2f} pp | "
            f"{delta['trades_delta']} | {delta['win_rate_pct_delta']:.2f} pp | {delta['sharpe_delta']:.2f} |"
        )

    lines.extend(
        [
            "",
            "## Read",
            "",
            "The legacy row is a historical reference only. It uses planned-price fills at `re_p`, which is not a realistic live execution model after a breakout has already moved the market.",
            "",
            "The live-v2 rows are intended to become the new optimization baseline. Gate sweeps should start from this baseline instead of from the legacy planned-fill result.",
            "",
        ]
    )
    output_path.write_text("\n".join(lines), encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="BTC Q1 2026 30min live-v2 baseline")
    parser.add_argument("--tick-files", nargs="+", default=DEFAULT_TICK_FILES)
    parser.add_argument("--start", default="2026-01-01T00:00:00Z")
    parser.add_argument("--end", default="2026-03-31T23:59:59Z")
    parser.add_argument("--initial-balance", type=float, default=100000.0)
    parser.add_argument("--chunksize", type=int, default=2_000_000)
    parser.add_argument("--summary-json", default="research/btc_2026_q1_30min_live_v2_baseline_summary.json")
    parser.add_argument("--markdown", default="research/20260505_btc_q1_30min_live_v2_baseline.md")
    parser.add_argument("--ledger-prefix", default="research/tmp_btc_2026_q1_30min_live_v2_baseline")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    research_engine.COMMON_REPLAY_KWARGS.clear()
    research_engine.COMMON_REPLAY_KWARGS.update(BTC_LIVE_LIKE_REPLAY_KWARGS)

    start = research_engine._as_utc_timestamp(args.start)
    end = research_engine._as_utc_timestamp(args.end)
    second_bars, build_stats = research_engine.build_continuous_second_bars(
        args.tick_files,
        start,
        end,
        args.chunksize,
    )
    _, signal = research_engine.build_signal_frame(second_bars, "30min")

    results = []
    for variant in VARIANTS:
        result = run_variant(second_bars, signal, variant, args.initial_balance)
        ledger_path = Path(f"{args.ledger_prefix}_{variant['name']}_ledger.csv")
        result["ledger"].to_csv(ledger_path, index=False)
        del result["ledger"]
        result["ledger_path"] = str(ledger_path)
        results.append(result)
        s = result["summary"]
        print(
            f"{variant['name']}: return={s['return_pct']:.2f}% trades={s['trades']} "
            f"win={s['win_rate_pct']:.2f}% max_dd={s['max_dd_pct']:.2f}% "
            f"elapsed={result['elapsed_seconds']}s",
            flush=True,
        )

    legacy_summary = results[0]["summary"]
    for result in results[1:]:
        result["delta_vs_legacy"] = _delta(legacy_summary, result["summary"])

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
        "baseline_kwargs": dict(BTC_LIVE_LIKE_REPLAY_KWARGS),
        "removed_optimization_gates": REMOVED_OPTIMIZATION_GATES,
        "variants": VARIANTS,
        "results": results,
        "note": "Research-only live-v2 baseline. Reentry fills use observed 1s close proxy instead of planned-price fills.",
    }
    Path(args.summary_json).write_text(json.dumps(output, indent=2, ensure_ascii=False), encoding="utf-8")
    write_markdown(output, Path(args.markdown))


if __name__ == "__main__":
    main()
