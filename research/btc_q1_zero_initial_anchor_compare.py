#!/usr/bin/env python3
"""BTC Q1 2026 30min zero-initial reentry anchor comparison.

Research-only experiment. It compares the current rolling zero-initial reentry
anchor with a live-like snapshot taken when the zero-initial window is armed.
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


VARIANTS = [
    {
        "name": "rolling_bar_extrema",
        "description": "Current research behavior: rolling zero-initial anchor with 1s high/low envelope triggers.",
        "zero_initial_reentry_anchor_mode": "rolling",
        "reentry_trigger_observation_mode": "bar_extrema",
    },
    {
        "name": "snapshot_bar_extrema",
        "description": "Snapshot zero-initial anchor with 1s high/low envelope triggers.",
        "zero_initial_reentry_anchor_mode": "snapshot",
        "reentry_trigger_observation_mode": "bar_extrema",
    },
    {
        "name": "rolling_close_proxy",
        "description": "Rolling zero-initial anchor with 1s close as a conservative live event-price proxy.",
        "zero_initial_reentry_anchor_mode": "rolling",
        "reentry_trigger_observation_mode": "close",
    },
    {
        "name": "snapshot_close_proxy",
        "description": "Snapshot zero-initial anchor with 1s close as a conservative live event-price proxy.",
        "zero_initial_reentry_anchor_mode": "snapshot",
        "reentry_trigger_observation_mode": "close",
    },
    {
        "name": "rolling_close_proxy_actionable_8bps",
        "description": "Rolling zero-initial anchor with 1s close proxy and live-style 8bps planned-price actionability gate.",
        "zero_initial_reentry_anchor_mode": "rolling",
        "reentry_trigger_observation_mode": "close",
        "reentry_max_deviation_bps": 8.0,
    },
    {
        "name": "snapshot_close_proxy_actionable_8bps",
        "description": "Snapshot zero-initial anchor with 1s close proxy and live-style 8bps planned-price actionability gate.",
        "zero_initial_reentry_anchor_mode": "snapshot",
        "reentry_trigger_observation_mode": "close",
        "reentry_max_deviation_bps": 8.0,
    },
]


def _entry_mix(summary: dict) -> str:
    return ", ".join(f"{k}:{v}" for k, v in summary["entry_reasons"].items())


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
        t3_quality_filters={"min_sma_atr_separation": 0.25},
        quality_filter_shapes=["t3_swing"],
        zero_initial_reentry_anchor_mode=variant["zero_initial_reentry_anchor_mode"],
        reentry_trigger_observation_mode=variant["reentry_trigger_observation_mode"],
        reentry_max_deviation_bps=variant.get("reentry_max_deviation_bps"),
    )
    pairs = _paired_trades(ledger)
    return {
        "variant": variant["name"],
        "description": variant["description"],
        "params": variant,
        "summary": research_engine.summarize_run(ledger, initial_balance),
        "pair_diagnostics": _pair_diagnostics(pairs),
        "attribution": research_engine.summarize_breakout_attribution(ledger),
        "diagnostics": diagnostics,
        "elapsed_seconds": round(time.time() - started, 2),
        "ledger": ledger,
    }


def write_markdown(summary: dict, output_path: Path) -> None:
    lines = [
        "# BTCUSDT Q1 2026 30min Zero-Initial Reentry Anchor Comparison",
        "",
        "Scope: research-only Python backtest. No live or execution path is changed by this report.",
        "",
        "## Setup",
        "",
        f"- Symbol/window: `{summary['symbol']}`, `{summary['start']}` to `{summary['end']}`",
        "- Execution source: continuous `1s` bars rebuilt from Binance trade archives",
        "- Signal timeframe: `30min`",
        "- Strategy shape: `baseline_plus_t3`, t3 SMA/ATR separation `0.25`",
        "- Baseline sizing: `dir2_zero_initial=true`, `zero_initial_mode=reentry_window`, `reentry_size_schedule=[0.20, 0.10]`, `max_trades_per_bar=2`",
        "- Compared dimensions: zero-initial reentry anchor mode and reentry trigger observation mode.",
        "- `bar_extrema` is the historical research envelope (`1s high/low`). `close_proxy` uses each 1s close as a conservative live event-price proxy.",
        "- `actionable_8bps` additionally applies the live-style planned-price actionability gate: BUY cannot be more than 8bps above planned price, SELL cannot be more than 8bps below planned price.",
        "- The snapshot anchor only affects zero-initial windows. SL/PT reentry windows still use their existing rolling anchor calculation.",
        "",
        "## Results",
        "",
        "| Anchor Mode | Final Balance | Return | Max DD | Trades | Win Rate | Sharpe | Avg Loss | Worst Loss | Entry Mix |",
        "|---|---:|---:|---:|---:|---:|---:|---:|---:|---|",
    ]
    for result in summary["results"]:
        s = result["summary"]
        d = result["pair_diagnostics"]
        lines.append(
            f"| `{result['variant']}` | {s['final_balance']:,.2f} | {s['return_pct']:.2f}% | "
            f"{s['max_dd_pct']:.2f}% | {s['trades']} | {s['win_rate_pct']:.2f}% | {s['sharpe']:.2f} | "
            f"{d['avg_loss_pct']:.4f}% | {d['worst_loss_pct']:.4f}% | `{_entry_mix(s)}` |"
        )

    lines.extend(["", "## Delta vs Rolling Bar Extrema", ""])
    lines.append("| Variant | Final Delta | Return Delta | Max DD Delta | Trades Delta | Win Delta | Sharpe Delta |")
    lines.append("|---|---:|---:|---:|---:|---:|---:|")
    for result in summary["results"][1:]:
        delta = result["delta_vs_rolling_bar_extrema"]
        lines.append(
            f"| `{result['variant']}` | {delta['final_balance_delta']:,.2f} | "
            f"{delta['return_pct_delta']:.2f} pp | {delta['max_dd_pct_delta']:.2f} pp | "
            f"{delta['trades_delta']} | {delta['win_rate_pct_delta']:.2f} pp | {delta['sharpe_delta']:.2f} |"
        )

    lines.extend(["", "## Read", ""])
    by_name = {result["variant"]: result for result in summary["results"]}
    bar_rolling = by_name.get("rolling_bar_extrema", {}).get("summary")
    bar_snapshot = by_name.get("snapshot_bar_extrema", {}).get("summary")
    close_rolling = by_name.get("rolling_close_proxy", {}).get("summary")
    close_snapshot = by_name.get("snapshot_close_proxy", {}).get("summary")
    if bar_rolling and bar_snapshot and bar_snapshot["final_balance"] == bar_rolling["final_balance"]:
        lines.append(
            "`bar_extrema` rolling and snapshot are identical in this window; the historical research envelope hides the anchor difference."
        )
    if close_rolling and close_snapshot:
        delta = _delta(close_rolling, close_snapshot)
        lines.append(
            f"Under the close-proxy live event-price approximation, snapshot vs rolling changes return by {delta['return_pct_delta']:.2f} pp "
            f"and trades by {delta['trades_delta']}."
        )
    actionable_rolling = by_name.get("rolling_close_proxy_actionable_8bps", {}).get("summary")
    actionable_snapshot = by_name.get("snapshot_close_proxy_actionable_8bps", {}).get("summary")
    if actionable_rolling and actionable_snapshot:
        delta = _delta(actionable_rolling, actionable_snapshot)
        lines.append(
            f"With the 8bps actionability gate included, snapshot vs rolling changes return by {delta['return_pct_delta']:.2f} pp "
            f"and trades by {delta['trades_delta']}."
        )
    lines.append("")
    output_path.write_text("\n".join(lines), encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="BTC Q1 2026 30min zero-initial anchor comparison")
    parser.add_argument("--tick-files", nargs="+", default=DEFAULT_TICK_FILES)
    parser.add_argument("--start", default="2026-01-01T00:00:00Z")
    parser.add_argument("--end", default="2026-03-31T23:59:59Z")
    parser.add_argument("--initial-balance", type=float, default=100000.0)
    parser.add_argument("--chunksize", type=int, default=2_000_000)
    parser.add_argument("--summary-json", default="research/btc_2026_q1_zero_initial_anchor_compare_summary.json")
    parser.add_argument("--markdown", default="research/20260504_btc_q1_zero_initial_anchor_compare.md")
    parser.add_argument("--ledger-prefix", default="research/tmp_btc_2026_q1_30min_zero_initial_anchor")
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

    rolling_summary = results[0]["summary"]
    for result in results[1:]:
        result["delta_vs_rolling_bar_extrema"] = _delta(rolling_summary, result["summary"])

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
        "variants": VARIANTS,
        "results": results,
        "note": "Research-only comparison. Snapshot mode only affects zero-initial reentry windows.",
    }
    Path(args.summary_json).write_text(json.dumps(output, indent=2, ensure_ascii=False), encoding="utf-8")
    write_markdown(output, Path(args.markdown))


if __name__ == "__main__":
    main()
