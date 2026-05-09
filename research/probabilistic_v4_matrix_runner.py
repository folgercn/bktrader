#!/usr/bin/env python3
"""Probabilistic V4 batch orchestrator.

Research-only. Runs the V4 pipeline end-to-end:
event dataset -> quality model -> execution variants -> aggregate report.

This script intentionally shells out to the three V4 layer scripts so their
standalone CLIs stay the source of truth.
"""

from __future__ import annotations

import argparse
import json
import subprocess
import sys
import time
from datetime import datetime, timezone
from pathlib import Path


DEFAULT_RUN_ROOT = Path("research/probabilistic_v4_runs")


def _slug(value: str) -> str:
    return (
        value.replace(":", "")
        .replace("+", "")
        .replace("-", "")
        .replace("T", "_")
        .replace("Z", "z")
        .replace(",", "_")
        .replace("=", "_")
    )


def _run_command(cmd: list[str], *, dry_run: bool) -> None:
    print(" ".join(cmd), flush=True)
    if dry_run:
        return
    subprocess.run(cmd, check=True)


def _parse_exec_variant(raw: str) -> tuple[str, dict]:
    name, values = raw.split("=", 1) if "=" in raw else (raw, "")
    params = {
        "entry_delay_seconds": 15,
        "initial_stop_atr": 0.45,
        "breakeven_at_r": 1.0,
        "trail_start_r": 1.5,
        "trail_buffer_atr": 0.05,
        "max_hold_hours": 6.0,
        "notional_share": 0.20,
    }
    key_map = {
        "delay": "entry_delay_seconds",
        "stop": "initial_stop_atr",
        "be": "breakeven_at_r",
        "trail": "trail_start_r",
        "trailbuf": "trail_buffer_atr",
        "hold": "max_hold_hours",
        "share": "notional_share",
    }
    for item in values.split(","):
        if not item.strip():
            continue
        key, value = item.split(":", 1)
        mapped = key_map.get(key.strip())
        if mapped is None:
            raise ValueError(f"unknown execution variant key {key!r} in {raw!r}")
        params[mapped] = float(value)
    params["entry_delay_seconds"] = int(params["entry_delay_seconds"])
    return name, params


def _quality_args(args: argparse.Namespace) -> list[str]:
    if args.quality_mode == "probability":
        parts: list[str] = [
            "--train-ratio",
            str(args.train_ratio),
            "--min-events",
            str(args.min_events),
            "--prob-mins",
            *[str(v) for v in args.prob_mins],
            "--ev-atr-mins",
            *[str(v) for v in args.ev_atr_mins],
            "--learning-rate",
            str(args.learning_rate),
            "--iterations",
            str(args.iterations),
            "--l2",
            str(args.l2),
            "--calibration-bins",
            str(args.calibration_bins),
        ]
        if args.train_end:
            parts.extend(["--train-end", args.train_end])
        return parts

    parts: list[str] = [
        "--train-ratio",
        str(args.train_ratio),
        "--min-events",
        str(args.min_events),
        "--llr-mins",
        *[str(v) for v in args.llr_mins],
        "--flow60-mins",
        *[str(v) for v in args.flow60_mins],
        "--speed60-mins",
        *[str(v) for v in args.speed60_mins],
        "--dwell-seconds",
        *[str(v) for v in args.dwell_seconds],
        "--pullback30-maxes",
        *[str(v) for v in args.pullback30_maxes],
    ]
    if args.train_end:
        parts.extend(["--train-end", args.train_end])
    return parts


def _read_json(path: Path) -> dict:
    return json.loads(path.read_text(encoding="utf-8"))


def _portfolio_rows(rows: list[dict]) -> list[dict]:
    grouped: dict[tuple[str, str], list[dict]] = {}
    for row in rows:
        key = (str(row["shape"]), str(row["execution_variant"]))
        grouped.setdefault(key, []).append(row)

    portfolio: list[dict] = []
    for (shape, execution_variant), group in sorted(grouped.items()):
        symbols = [str(row["symbol"]) for row in group]
        count = len(group)
        if count == 0:
            continue
        portfolio.append(
            {
                "shape": shape,
                "execution_variant": execution_variant,
                "symbols": symbols,
                "selected_events": int(sum(int(row["selected_events"]) for row in group)),
                "trades": int(sum(int(row["trades"]) for row in group)),
                "equal_weight_realistic_return_pct": round(
                    sum(float(row["realistic_return_pct"]) for row in group) / count,
                    6,
                ),
                "equal_weight_raw_return_pct": round(sum(float(row["raw_return_pct"]) for row in group) / count, 6),
                "equal_weight_slip_return_pct": round(sum(float(row["slip_return_pct"]) for row in group) / count, 6),
                "positive_symbols": int(sum(1 for row in group if float(row["realistic_return_pct"]) > 0.0)),
                "min_profit_factor": round(min(float(row["profit_factor"]) for row in group), 6),
                "max_dd_pct": min(float(row["max_dd_pct"]) for row in group),
            }
        )
    return portfolio


def _aggregate(run_dir: Path, rows: list[dict], args: argparse.Namespace) -> dict:
    portfolio_rows = _portfolio_rows(rows)
    aggregate = {
        "run_dir": str(run_dir),
        "start": args.start,
        "end": args.end,
        "symbols": args.symbols,
        "breakout_shapes": args.breakout_shapes,
        "quality_mode": args.quality_mode,
        "selection_scope": args.selection_scope,
        "execute_start": args.execute_start,
        "execute_end": args.execute_end,
        "bars_cache_dir": args.bars_cache_dir,
        "execution_variants": args.execution_variants,
        "rows": rows,
        "portfolio_rows": portfolio_rows,
    }
    (run_dir / "summary.json").write_text(json.dumps(aggregate, indent=2, ensure_ascii=False, default=str), encoding="utf-8")

    lines = [
        "# Probabilistic V4 Matrix Run",
        "",
        f"- Period: `{args.start}` to `{args.end}`",
        f"- Symbols: `{', '.join(args.symbols)}`",
        f"- Shapes: `{', '.join(args.breakout_shapes)}`",
        f"- Quality mode: `{args.quality_mode}`",
        f"- Quality selection scope: `{args.selection_scope}`",
        f"- Execute window: `{args.execute_start or args.start}` to `{args.execute_end or args.end}`",
        "",
        "| Shape | Exec Variant | Symbol | Events | Selected | Trades | Realistic | Raw | Slip | PF | Win | DD |",
        "|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|",
    ]
    for row in rows:
        lines.append(
            f"| `{row['shape']}` | `{row['execution_variant']}` | `{row['symbol']}` | "
            f"{row['events']} | {row['selected_events']} | {row['trades']} | "
            f"{row['realistic_return_pct']:.4f}% | {row['raw_return_pct']:.4f}% | "
            f"{row['slip_return_pct']:.4f}% | {row['profit_factor']} | "
            f"{row['win_rate_pct']:.2f}% | {row['max_dd_pct']:.4f}% |"
        )
    if portfolio_rows:
        lines.extend(
            [
                "",
                "## Equal-Weight Portfolio Rows",
                "",
                "| Shape | Exec Variant | Symbols | Selected | Trades | Realistic | Raw | Slip | Positive Symbols | Min PF | Worst DD |",
                "|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|",
            ]
        )
        for row in portfolio_rows:
            lines.append(
                f"| `{row['shape']}` | `{row['execution_variant']}` | `{', '.join(row['symbols'])}` | "
                f"{row['selected_events']} | {row['trades']} | "
                f"{row['equal_weight_realistic_return_pct']:.4f}% | "
                f"{row['equal_weight_raw_return_pct']:.4f}% | {row['equal_weight_slip_return_pct']:.4f}% | "
                f"{row['positive_symbols']} | {row['min_profit_factor']} | {row['max_dd_pct']:.4f}% |"
            )
    (run_dir / "summary.md").write_text("\n".join(lines) + "\n", encoding="utf-8")
    return aggregate


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Run Probabilistic V4 matrix")
    parser.add_argument("--run-name", default="")
    parser.add_argument("--run-root", default=str(DEFAULT_RUN_ROOT))
    parser.add_argument("--symbols", nargs="+", default=["BTCUSDT", "ETHUSDT"])
    parser.add_argument("--start", default="2026-01-01T00:00:00Z")
    parser.add_argument("--end", default="2026-04-30T23:59:59Z")
    parser.add_argument("--execute-start", default="")
    parser.add_argument("--execute-end", default="")
    parser.add_argument("--archive-root", default="dataset/archive")
    parser.add_argument("--signal-timeframe", default="1h")
    parser.add_argument("--breakout-shapes", nargs="+", default=["original_t2", "baseline_plus_t3"])
    parser.add_argument("--continuation-atr", type=float, default=0.5)
    parser.add_argument("--fail-atr", type=float, default=0.2)
    parser.add_argument("--horizon-seconds", type=int, default=7200)
    parser.add_argument("--chunksize", type=int, default=2_000_000)
    parser.add_argument("--bars-cache-dir", default="")
    parser.add_argument("--quality-mode", choices=["rule", "probability"], default="rule")
    parser.add_argument("--train-ratio", type=float, default=0.70)
    parser.add_argument("--train-end", default="")
    parser.add_argument("--selection-scope", choices=["global", "per_symbol"], default="global")
    parser.add_argument("--min-events", type=int, default=20)
    parser.add_argument("--llr-mins", nargs="+", type=float, default=[-999.0, 0.0, 2.0, 4.0, 6.0])
    parser.add_argument("--flow60-mins", nargs="+", type=float, default=[0.0, 0.55, 0.60, 0.65])
    parser.add_argument("--speed60-mins", nargs="+", type=float, default=[-999.0, 0.0, 0.03, 0.08])
    parser.add_argument("--dwell-seconds", nargs="+", type=int, default=[0, 5, 15, 30])
    parser.add_argument("--pullback30-maxes", nargs="+", default=["none", "0.05", "0.10", "0.20"])
    parser.add_argument("--prob-mins", nargs="+", type=float, default=[0.40, 0.45, 0.50, 0.55, 0.60, 0.65])
    parser.add_argument("--ev-atr-mins", nargs="+", type=float, default=[-0.05, 0.0, 0.05, 0.10, 0.15])
    parser.add_argument("--learning-rate", type=float, default=0.05)
    parser.add_argument("--iterations", type=int, default=2500)
    parser.add_argument("--l2", type=float, default=0.01)
    parser.add_argument("--calibration-bins", type=int, default=5)
    parser.add_argument(
        "--execution-variants",
        nargs="+",
        default=["base=delay:15,stop:0.45,be:1.0,trail:1.5,hold:6"],
        help="name=delay:15,stop:0.45,be:1.0,trail:1.5,trailbuf:0.05,hold:6,share:0.2",
    )
    parser.add_argument("--initial-balance", type=float, default=100000.0)
    parser.add_argument("--dry-run", action="store_true")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    started = time.time()
    run_name = args.run_name or datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    run_dir = Path(args.run_root) / _slug(run_name)
    run_dir.mkdir(parents=True, exist_ok=True)

    rows: list[dict] = []
    commands_path = run_dir / "commands.txt"
    command_lines: list[str] = []
    execution_variants = [_parse_exec_variant(raw) for raw in args.execution_variants]
    bars_cache_dir = Path(args.bars_cache_dir) if args.bars_cache_dir else run_dir / "bars_cache"

    for shape in args.breakout_shapes:
        shape_dir = run_dir / shape
        shape_dir.mkdir(parents=True, exist_ok=True)
        events_csv = shape_dir / "events.csv"
        events_summary = shape_dir / "events_summary.json"
        scored_csv = shape_dir / "events_scored.csv"
        rules_json = shape_dir / "quality_rules.json"
        quality_md = shape_dir / "quality_model.md"

        event_cmd = [
            sys.executable,
            "research/probabilistic_v4_event_dataset.py",
            "--symbols",
            *args.symbols,
            "--start",
            args.start,
            "--end",
            args.end,
            "--archive-root",
            args.archive_root,
            "--signal-timeframe",
            args.signal_timeframe,
            "--breakout-shape",
            shape,
            "--continuation-atr",
            str(args.continuation_atr),
            "--fail-atr",
            str(args.fail_atr),
            "--horizon-seconds",
            str(args.horizon_seconds),
            "--chunksize",
            str(args.chunksize),
            "--bars-cache-dir",
            str(bars_cache_dir),
            "--output-csv",
            str(events_csv),
            "--summary-json",
            str(events_summary),
        ]
        command_lines.append(" ".join(event_cmd))
        _run_command(event_cmd, dry_run=args.dry_run)

        if args.quality_mode == "probability":
            quality_cmd = [
                sys.executable,
                "research/probabilistic_v4_probability_model.py",
                "--events-csv",
                str(events_csv),
                "--scored-csv",
                str(scored_csv),
                "--model-json",
                str(rules_json),
                "--markdown",
                str(quality_md),
                "--selection-scope",
                args.selection_scope,
                *_quality_args(args),
            ]
        else:
            quality_cmd = [
                sys.executable,
                "research/probabilistic_v4_quality_model.py",
                "--events-csv",
                str(events_csv),
                "--scored-csv",
                str(scored_csv),
                "--rules-json",
                str(rules_json),
                "--markdown",
                str(quality_md),
                "--selection-scope",
                args.selection_scope,
                *_quality_args(args),
            ]
        command_lines.append(" ".join(quality_cmd))
        _run_command(quality_cmd, dry_run=args.dry_run)

        event_data = _read_json(events_summary) if not args.dry_run else {}

        for exec_name, exec_params in execution_variants:
            exec_summary = shape_dir / f"execution_{_slug(exec_name)}_summary.json"
            exec_md = shape_dir / f"execution_{_slug(exec_name)}.md"
            ledger_prefix = shape_dir / f"tmp_execution_{_slug(exec_name)}"
            exec_cmd = [
                sys.executable,
                "research/probabilistic_v4_execution_runner.py",
                "--events-csv",
                str(scored_csv),
                "--rules-json",
                str(rules_json),
                "--symbols",
                *args.symbols,
                "--start",
                args.start,
                "--end",
                args.end,
                "--execute-start",
                args.execute_start,
                "--execute-end",
                args.execute_end,
                "--archive-root",
                args.archive_root,
                "--chunksize",
                str(args.chunksize),
                "--bars-cache-dir",
                str(bars_cache_dir),
                "--initial-balance",
                str(args.initial_balance),
                "--entry-delay-seconds",
                str(exec_params["entry_delay_seconds"]),
                "--initial-stop-atr",
                str(exec_params["initial_stop_atr"]),
                "--breakeven-at-r",
                str(exec_params["breakeven_at_r"]),
                "--trail-start-r",
                str(exec_params["trail_start_r"]),
                "--trail-buffer-atr",
                str(exec_params["trail_buffer_atr"]),
                "--max-hold-hours",
                str(exec_params["max_hold_hours"]),
                "--notional-share",
                str(exec_params["notional_share"]),
                "--summary-json",
                str(exec_summary),
                "--markdown",
                str(exec_md),
                "--ledger-prefix",
                str(ledger_prefix),
            ]
            command_lines.append(" ".join(exec_cmd))
            _run_command(exec_cmd, dry_run=args.dry_run)
            if args.dry_run:
                continue
            exec_data = _read_json(exec_summary)
            for result in exec_data.get("results", []):
                summary = result["summary"]
                rows.append(
                    {
                        "shape": shape,
                        "execution_variant": exec_name,
                        "symbol": result["symbol"],
                        "events": int(event_data.get("per_symbol", {}).get(result["symbol"], {}).get("events", 0)),
                        "selected_events": int(result.get("diagnostics", {}).get("candidate_events", 0)),
                        "trades": int(summary["trades"]),
                        "realistic_return_pct": float(summary["realistic_return_pct"]),
                        "raw_return_pct": float(summary["raw_no_fee_no_slip_return_pct"]),
                        "slip_return_pct": float(summary["price_pnl_with_2bps_slip_no_fee_return_pct"]),
                        "profit_factor": result.get("attribution", {}).get("profit_factor", 0.0),
                        "win_rate_pct": float(summary["win_rate_pct"]),
                        "max_dd_pct": float(summary["max_dd_pct"]),
                        "diagnostics": result.get("diagnostics", {}),
                        "execution_summary": str(exec_summary),
                    }
                )

    commands_path.write_text("\n".join(command_lines) + "\n", encoding="utf-8")
    if not args.dry_run:
        aggregate = _aggregate(run_dir, rows, args)
        aggregate["elapsed_seconds"] = round(time.time() - started, 2)
        (run_dir / "summary.json").write_text(json.dumps(aggregate, indent=2, ensure_ascii=False, default=str), encoding="utf-8")
        print(json.dumps({"run_dir": str(run_dir), "rows": len(rows), "elapsed_seconds": aggregate["elapsed_seconds"]}, indent=2), flush=True)
    else:
        print(json.dumps({"run_dir": str(run_dir), "commands": str(commands_path), "dry_run": True}, indent=2), flush=True)


if __name__ == "__main__":
    main()
