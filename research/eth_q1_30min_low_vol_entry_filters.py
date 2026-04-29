#!/usr/bin/env python3
"""ETH Q1 2026 30min low-volatility entry gate research replay.

Research-only script. It evaluates bps/ATR-percentile entry gates on the ETH
Q1 30min replay path. The gate is applied at reentry price resolution, so it
can block Zero-Initial-Reentry and SL-Reentry entries without filtering
stop-loss exits.
"""

from __future__ import annotations

import argparse
import json
import time
from pathlib import Path

import numpy as np

import btc_q1_30min_low_vol_entry_filters as low_vol
import eth_q1_breakout_t3_shape_compare as replay


BASE_REPLAY_KWARGS = dict(replay.COMMON_REPLAY_KWARGS)

VARIANTS = [
    {
        "name": "baseline",
        "description": "No low-volatility entry gate.",
    },
    {
        "name": "min_stop_bps_2",
        "description": "Require stop_loss_atr * ATR to be at least 2 bps of entry reference price.",
        "min_stop_bps": 2.0,
    },
    {
        "name": "min_stop_bps_4",
        "description": "Require stop_loss_atr * ATR to be at least 4 bps of entry reference price.",
        "min_stop_bps": 4.0,
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
        "name": "min_stop_bps_4_atr_pct_gte_25",
        "description": "Require both min stop distance of 4 bps and ATR percentile at least 25.",
        "min_stop_bps": 4.0,
        "atr_percentile_gte": 25.0,
    },
]


def _stop_bps_stats(signal, stop_loss_atr: float) -> dict:
    stop_bps = stop_loss_atr * signal["atr"].astype("float64") / signal["close"].astype("float64") * 10000.0
    stop_bps = stop_bps.replace([np.inf, -np.inf], np.nan).dropna()
    if stop_bps.empty:
        return {}
    return {
        "count": int(len(stop_bps)),
        "min": round(float(stop_bps.min()), 4),
        "p05": round(float(stop_bps.quantile(0.05)), 4),
        "p10": round(float(stop_bps.quantile(0.10)), 4),
        "p25": round(float(stop_bps.quantile(0.25)), 4),
        "p50": round(float(stop_bps.quantile(0.50)), 4),
        "p75": round(float(stop_bps.quantile(0.75)), 4),
        "p90": round(float(stop_bps.quantile(0.90)), 4),
        "p95": round(float(stop_bps.quantile(0.95)), 4),
        "max": round(float(stop_bps.max()), 4),
    }


def _write_markdown(output: dict, path: Path) -> None:
    lines = [
        "# ETHUSDT Q1 2026 30min Low-Volatility Entry Gates",
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
        f"- Risk profile: `stop_loss_atr={output['replay_kwargs']['stop_loss_atr']}`, `trailing_stop_atr={output['replay_kwargs']['trailing_stop_atr']}`, `delayed_trailing_activation={output['replay_kwargs']['delayed_trailing_activation']}`",
        "- Gate point: reentry price resolution, so the gate blocks both Zero-Initial-Reentry and SL-Reentry entries. Stop-loss exits are not filtered.",
        "",
        "## Stop Distance Distribution",
        "",
        "| Count | Min | P05 | P10 | P25 | P50 | P75 | P90 | P95 | Max |",
        "|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|",
    ]
    bps = output["signal_stop_bps_stats"]
    lines.append(
        f"| {bps.get('count', 0)} | {bps.get('min', 0):.4f} | {bps.get('p05', 0):.4f} | "
        f"{bps.get('p10', 0):.4f} | {bps.get('p25', 0):.4f} | {bps.get('p50', 0):.4f} | "
        f"{bps.get('p75', 0):.4f} | {bps.get('p90', 0):.4f} | {bps.get('p95', 0):.4f} | "
        f"{bps.get('max', 0):.4f} |"
    )

    lines.extend(
        [
            "",
            "## Results",
            "",
            "| Variant | Final Balance | Return | Max DD | Trades | Win Rate | Sharpe | Avg Loss | Worst Loss | Entry Mix | Gate Reject Calls |",
            "|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|",
        ]
    )
    for result in output["results"]:
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
    for result in output["results"][1:]:
        delta = result["delta_vs_baseline"]
        lines.append(
            f"| `{result['variant']}` | {delta['final_balance_delta']:,.2f} | "
            f"{delta['return_pct_delta']:.2f} pp | {delta['max_dd_pct_delta']:.2f} pp | "
            f"{delta['trades_delta']} | {delta['win_rate_pct_delta']:.2f} pp | {delta['sharpe_delta']:.2f} |"
        )

    lines.extend(["", "## Variants", ""])
    for variant in output["selected_variants"]:
        lines.append(f"- `{variant['name']}`: {variant['description']}")
    lines.append("")
    path.write_text("\n".join(lines), encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="ETH Q1 2026 30min low-volatility entry gate replay")
    parser.add_argument("--tick-files", nargs="+", default=replay.DEFAULT_TICK_FILES)
    parser.add_argument("--start", default="2026-01-01T00:00:00Z")
    parser.add_argument("--end", default="2026-03-31T23:59:59Z")
    parser.add_argument("--initial-balance", type=float, default=100000.0)
    parser.add_argument("--chunksize", type=int, default=2_000_000)
    parser.add_argument("--stop-loss-atr", type=float, default=BASE_REPLAY_KWARGS["stop_loss_atr"])
    parser.add_argument("--variants", nargs="+", default=[item["name"] for item in VARIANTS])
    parser.add_argument("--summary-json", default="research/eth_2026_q1_30min_low_vol_entry_filters_summary.json")
    parser.add_argument("--markdown", default="research/20260429_eth_q1_30min_low_vol_entry_filters.md")
    parser.add_argument("--ledger-prefix", default="research/tmp_eth_2026_q1_30min_1s_low_vol_entry_filter")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    replay_kwargs = dict(BASE_REPLAY_KWARGS)
    replay_kwargs["stop_loss_atr"] = float(args.stop_loss_atr)
    replay.COMMON_REPLAY_KWARGS.clear()
    replay.COMMON_REPLAY_KWARGS.update(replay_kwargs)

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
        result = low_vol.run_variant(second_bars, signal, variant, args.initial_balance)
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
        result["delta_vs_baseline"] = low_vol._delta(base_summary, result["summary"])

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
            "valid_atr_percentile_rows": int(signal["atr_percentile"].notna().sum()),
        },
        "signal_stop_bps_stats": _stop_bps_stats(signal, replay_kwargs["stop_loss_atr"]),
        "replay_kwargs": replay_kwargs,
        "selected_variants": selected_variants,
        "results": results,
        "elapsed_at": time.strftime("%Y-%m-%dT%H:%M:%S%z"),
        "note": "Research-only comparison. Entry gates patch reentry price resolution and do not alter stop-loss exit behavior.",
    }
    Path(args.summary_json).write_text(json.dumps(output, indent=2, ensure_ascii=False), encoding="utf-8")
    _write_markdown(output, Path(args.markdown))


if __name__ == "__main__":
    main()
