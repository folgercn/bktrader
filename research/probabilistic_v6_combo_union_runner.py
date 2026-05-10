#!/usr/bin/env python3
"""Run event-level union execution for selected V6 walk-forward sleeves.

Research-only. This consumes a no-trade gate analyzer JSON, reconstructs the
selected topK event CSVs, merges them per execute-month/symbol, de-duplicates
events, then re-runs the existing 1s execution runner on the union.
"""

from __future__ import annotations

import argparse
import calendar
import json
import subprocess
import time
from collections import defaultdict
from pathlib import Path

import numpy as np
import pandas as pd


def _run(cmd: list[str], *, quiet: bool = False) -> None:
    if not quiet:
        print(" ".join(cmd), flush=True)
        subprocess.run(cmd, check=True)
        return
    completed = subprocess.run(cmd, text=True, capture_output=True)
    if completed.returncode != 0:
        if completed.stdout:
            print(completed.stdout, flush=True)
        if completed.stderr:
            print(completed.stderr, flush=True)
        completed.check_returncode()


def _load_json(path: Path) -> dict:
    return json.loads(path.read_text(encoding="utf-8"))


def _month_start(month: str) -> pd.Timestamp:
    return pd.Timestamp(f"{month}-01T00:00:00Z")


def _month_end(month: str) -> pd.Timestamp:
    year, mon = [int(part) for part in month.split("-")]
    day = calendar.monthrange(year, mon)[1]
    return pd.Timestamp(f"{year:04d}-{mon:02d}-{day:02d}T23:59:59Z")


def _as_bool_series(series: pd.Series) -> pd.Series:
    if series.dtype == bool:
        return series
    return series.astype(str).str.lower().isin({"true", "1", "yes"})


def _event_key_columns(frame: pd.DataFrame) -> list[str]:
    if "event_id" in frame.columns:
        return ["event_id"]
    candidates = ["symbol", "signal_start", "touch_time", "side", "shape"]
    return [col for col in candidates if col in frame.columns]


def _source_event_csv(source_run: str, execute_month: str, symbol: str, top_k: int) -> Path:
    return (
        Path("research/probabilistic_v6_runs")
        / source_run
        / f"execute_{execute_month}"
        / symbol
        / f"topk_{int(top_k)}"
        / "events_scored_for_execution.csv"
    )


def _selected_rows(selection: dict, key: str) -> list[dict]:
    section = selection.get(key)
    if not isinstance(section, dict):
        raise SystemExit(f"selection key not found or not an object: {key}")
    rows = section.get("selected_rows", [])
    if not isinstance(rows, list) or not rows:
        raise SystemExit(f"selection key has no selected_rows: {key}")
    return rows


def _clip01(value: float) -> float:
    return max(0.0, min(1.0, float(value)))


def _row_float(row: dict, key: str, default: float = 0.0) -> float:
    raw = row.get(key, default)
    if raw in (None, ""):
        return float(default)
    try:
        return float(raw)
    except (TypeError, ValueError):
        return float(default)


def _validation_return_over_dd(row: dict) -> float:
    if "validation_return_over_dd" in row:
        return _row_float(row, "validation_return_over_dd")
    validation_return = _row_float(row, "validation_topk_sized_return_pct")
    validation_dd = abs(_row_float(row, "validation_topk_max_dd_pct"))
    return validation_return / max(0.25, validation_dd)


def _source_quality_scale(row: dict, args: argparse.Namespace) -> float:
    edge_score = _clip01(_row_float(row, "validation_edge") / max(1e-9, float(args.sizing_edge_ref)))
    return_score = _clip01(
        _row_float(row, "validation_topk_sized_return_pct") / max(1e-9, float(args.sizing_return_ref))
    )
    return_dd_score = _clip01(_validation_return_over_dd(row) / max(1e-9, float(args.sizing_return_dd_ref)))
    markov = _row_float(row, "validation_topk_sizing_markov_score_mean", 0.5)
    if float(args.sizing_markov_high) <= float(args.sizing_markov_low):
        markov_score = 1.0
    else:
        markov_score = _clip01(
            (markov - float(args.sizing_markov_low))
            / max(1e-9, float(args.sizing_markov_high) - float(args.sizing_markov_low))
        )
    score = (
        float(args.sizing_edge_weight) * edge_score
        + float(args.sizing_return_weight) * return_score
        + float(args.sizing_return_dd_weight) * return_dd_score
        + float(args.sizing_markov_weight) * markov_score
    )
    weight_sum = (
        float(args.sizing_edge_weight)
        + float(args.sizing_return_weight)
        + float(args.sizing_return_dd_weight)
        + float(args.sizing_markov_weight)
    )
    if weight_sum <= 0:
        score = 1.0
    else:
        score /= weight_sum
    return float(args.sizing_min_scale) + (
        float(args.sizing_max_scale) - float(args.sizing_min_scale)
    ) * _clip01(score)


def _apply_sizing_transform(selected: pd.DataFrame, row: dict, args: argparse.Namespace) -> pd.DataFrame:
    work = selected.copy()
    if "model_notional_share" in work.columns:
        share = pd.to_numeric(work["model_notional_share"], errors="coerce")
    else:
        share = pd.Series(np.nan, index=work.index)
    share = share.fillna(float(args.notional_share))

    source_scale = 1.0
    if args.sizing_profile == "source_quality":
        source_scale = _source_quality_scale(row, args)
    if float(args.share_power) != 1.0:
        share = share.clip(lower=1e-9).pow(float(args.share_power))
    share = share * float(args.share_multiplier) * float(source_scale)
    if float(args.share_cap) > 0.0:
        share = share.clip(upper=float(args.share_cap))
    if float(args.share_floor) > 0.0:
        share = share.clip(lower=float(args.share_floor))
    work["model_notional_share"] = share
    work["source_sizing_profile"] = str(args.sizing_profile)
    work["source_sizing_scale"] = float(source_scale)
    work["source_validation_edge"] = _row_float(row, "validation_edge")
    work["source_validation_topk_return_pct"] = _row_float(row, "validation_topk_sized_return_pct")
    work["source_validation_return_over_dd"] = _validation_return_over_dd(row)
    work["source_validation_topk_markov_score"] = _row_float(row, "validation_topk_sizing_markov_score_mean")
    work["source_validation_topk_initial_sl_rate"] = _row_float(row, "validation_topk_initial_sl_rate")
    return work


def _load_selected_event_frame(row: dict, args: argparse.Namespace) -> pd.DataFrame:
    path = _source_event_csv(
        str(row["source_run"]),
        str(row["execute_month"]),
        str(row["symbol"]),
        int(row["top_k"]),
    )
    if not path.exists():
        raise FileNotFoundError(path)
    frame = pd.read_csv(path, parse_dates=["touch_time", "signal_start", "signal_end"])
    if "quality_pass" not in frame.columns:
        raise ValueError(f"missing quality_pass column: {path}")
    selected = frame[_as_bool_series(frame["quality_pass"])].copy()
    selected["source_run"] = str(row["source_run"])
    selected["source_top_k"] = int(row["top_k"])
    selected["source_realistic_return_pct"] = float(row.get("realistic_return_pct", 0.0))
    return _apply_sizing_transform(selected, row, args)


def _dedupe_events(frame: pd.DataFrame) -> tuple[pd.DataFrame, dict]:
    if frame.empty:
        return frame.copy(), {"input_rows": 0, "deduped_rows": 0, "duplicate_rows": 0}
    key_cols = _event_key_columns(frame)
    work = frame.copy()
    if "model_notional_share" in work.columns:
        work["_share_sort"] = pd.to_numeric(work["model_notional_share"], errors="coerce").fillna(0.0)
    else:
        work["_share_sort"] = 0.0
    work["_source_sort"] = work["source_run"].astype(str)
    work = work.sort_values(["_share_sort", "_source_sort"], ascending=[False, True])
    deduped = work.drop_duplicates(subset=key_cols, keep="first").drop(columns=["_share_sort", "_source_sort"])
    return (
        deduped.sort_values("touch_time").reset_index(drop=True),
        {
            "input_rows": int(len(frame)),
            "deduped_rows": int(len(deduped)),
            "duplicate_rows": int(len(frame) - len(deduped)),
            "dedupe_keys": key_cols,
        },
    )


def _summary_value(summary: dict, path: list[str], default=0.0):
    current = summary
    for key in path:
        if not isinstance(current, dict) or key not in current:
            return default
        current = current[key]
    return current


def _first_result(summary: dict) -> dict:
    return summary.get("results", [{}])[0]


def _write_markdown(summary: dict, path: Path) -> None:
    lines = [
        "# Probabilistic V6 Combo Union Runner",
        "",
        "范围：仅限 `research`。本报告把 gate 扫描选中的多个 sleeve 合并到事件级，再用同一个 1s execution runner 回测。",
        "",
        "## Metrics",
        "",
        "| Metric | Value |",
        "|---|---:|",
        f"| Active_Silo_Sum | {summary['active_silo_sum_pct']:.4f}% |",
        f"| Active Months | {summary['active_months']} |",
        f"| Trades | {summary['trade_count']} |",
        f"| Worst Silo | {summary['worst_active_silo_pct']:.4f}% |",
        f"| Best Silo | {summary['best_active_silo_pct']:.4f}% |",
        f"| Input Sleeve Rows | {summary['input_sleeve_rows']} |",
        f"| Union Groups | {summary['union_groups']} |",
        f"| Duplicate Events Removed | {summary['duplicate_events_removed']} |",
        f"| Sizing Profile | `{summary['sizing_profile']}` |",
        f"| Share Multiplier | {summary['share_multiplier']:.4f} |",
        f"| Share Power | {summary['share_power']:.4f} |",
        f"| Share Cap | {summary['share_cap']:.4f} |",
        "",
        "## Groups",
        "",
        "| Month | Symbol | Sleeves | Input Events | Union Events | Dups | Trades | Return | Mean Share | Mean Scale | Sources |",
        "|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|",
    ]
    for row in summary["group_rows"]:
        sources = ", ".join(row["sources"])
        lines.append(
            f"| `{row['execute_month']}` | `{row['symbol']}` | {row['sleeves']} | "
            f"{row['input_events']} | {row['union_events']} | {row['duplicate_events']} | "
            f"{row['trades']} | {row['realistic_return_pct']:.4f}% | "
            f"{row['mean_model_notional_share']:.4f} | {row['mean_source_sizing_scale']:.4f} | `{sources}` |"
        )
    path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Run event-level union for selected V6 combo sleeves")
    parser.add_argument("--selection-json", required=True)
    parser.add_argument("--selection-key", default="best_qualified")
    parser.add_argument("--run-dir", required=True)
    parser.add_argument("--start", default="2025-03-01T00:00:00Z")
    parser.add_argument("--end", default="2026-04-30T23:59:59Z")
    parser.add_argument("--archive-root", default="dataset/archive")
    parser.add_argument("--chunksize", type=int, default=5_000_000)
    parser.add_argument("--bars-cache-dir", default="research/probabilistic_v6_runs/walkforward_delay60_original_t2_feature60_valbest/bars_cache")
    parser.add_argument("--entry-delay-seconds", type=int, default=60)
    parser.add_argument("--initial-stop-atr", type=float, default=0.45)
    parser.add_argument("--breakeven-at-r", type=float, default=0.8)
    parser.add_argument("--trail-start-r", type=float, default=0.9)
    parser.add_argument("--max-hold-hours", type=float, default=4.0)
    parser.add_argument("--initial-balance", type=float, default=100000.0)
    parser.add_argument("--notional-share", type=float, default=0.20)
    parser.add_argument("--quiet", action="store_true")
    parser.add_argument("--sizing-profile", choices=["none", "source_quality"], default="none")
    parser.add_argument("--share-multiplier", type=float, default=1.0)
    parser.add_argument("--share-power", type=float, default=1.0)
    parser.add_argument("--share-cap", type=float, default=0.0, help="0 disables upper cap")
    parser.add_argument("--share-floor", type=float, default=0.0, help="0 disables lower floor")
    parser.add_argument("--sizing-min-scale", type=float, default=0.50)
    parser.add_argument("--sizing-max-scale", type=float, default=1.20)
    parser.add_argument("--sizing-edge-ref", type=float, default=0.30)
    parser.add_argument("--sizing-return-ref", type=float, default=3.0)
    parser.add_argument("--sizing-return-dd-ref", type=float, default=6.0)
    parser.add_argument("--sizing-markov-low", type=float, default=0.40)
    parser.add_argument("--sizing-markov-high", type=float, default=0.90)
    parser.add_argument("--sizing-edge-weight", type=float, default=0.25)
    parser.add_argument("--sizing-return-weight", type=float, default=0.25)
    parser.add_argument("--sizing-return-dd-weight", type=float, default=0.25)
    parser.add_argument("--sizing-markov-weight", type=float, default=0.25)
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    started = time.time()
    run_dir = Path(args.run_dir)
    run_dir.mkdir(parents=True, exist_ok=True)
    selection = _load_json(Path(args.selection_json))
    rows = _selected_rows(selection, args.selection_key)

    grouped: dict[tuple[str, str], list[dict]] = defaultdict(list)
    for row in rows:
        grouped[(str(row["execute_month"]), str(row["symbol"]))].append(row)

    group_rows = []
    duplicate_events_removed = 0
    for (execute_month, symbol), sleeve_rows in sorted(grouped.items()):
        frames = [_load_selected_event_frame(row, args) for row in sleeve_rows]
        combined = pd.concat(frames, ignore_index=True) if frames else pd.DataFrame()
        union_frame, dedupe_stats = _dedupe_events(combined)
        duplicate_events_removed += int(dedupe_stats["duplicate_rows"])
        group_dir = run_dir / f"execute_{execute_month}" / symbol
        group_dir.mkdir(parents=True, exist_ok=True)
        union_csv = group_dir / "events_scored_union.csv"
        union_frame.to_csv(union_csv, index=False)

        summary_json = group_dir / "execution_summary.json"
        markdown = group_dir / "execution.md"
        cmd = [
            "python3",
            "research/probabilistic_v4_execution_runner.py",
            "--events-csv",
            str(union_csv),
            "--rules-json",
            str(group_dir / "union_rule.json"),
            "--symbols",
            symbol,
            "--start",
            args.start,
            "--end",
            args.end,
            "--execute-start",
            _month_start(execute_month).isoformat(),
            "--execute-end",
            _month_end(execute_month).isoformat(),
            "--archive-root",
            args.archive_root,
            "--chunksize",
            str(args.chunksize),
            "--entry-delay-seconds",
            str(args.entry_delay_seconds),
            "--initial-stop-atr",
            str(args.initial_stop_atr),
            "--breakeven-at-r",
            str(args.breakeven_at_r),
            "--trail-start-r",
            str(args.trail_start_r),
            "--max-hold-hours",
            str(args.max_hold_hours),
            "--initial-balance",
            str(args.initial_balance),
            "--notional-share",
            str(args.notional_share),
            "--summary-json",
            str(summary_json),
            "--markdown",
            str(markdown),
            "--ledger-prefix",
            str(group_dir / "tmp_execution"),
        ]
        if args.bars_cache_dir:
            cmd.extend(["--bars-cache-dir", args.bars_cache_dir])
        (group_dir / "union_rule.json").write_text(json.dumps({"selected_rule": {}}, indent=2), encoding="utf-8")
        _run(cmd, quiet=bool(args.quiet))
        execution_summary = _load_json(summary_json)
        result = _first_result(execution_summary)
        sources = sorted({f"{row['source_run']}:top{int(row['top_k'])}" for row in sleeve_rows})
        union_share = (
            pd.to_numeric(union_frame.get("model_notional_share", pd.Series(dtype=float)), errors="coerce")
            if not union_frame.empty
            else pd.Series(dtype=float)
        )
        union_scale = (
            pd.to_numeric(union_frame.get("source_sizing_scale", pd.Series(dtype=float)), errors="coerce")
            if not union_frame.empty
            else pd.Series(dtype=float)
        )
        group_rows.append(
            {
                "execute_month": execute_month,
                "symbol": symbol,
                "sleeves": int(len(sleeve_rows)),
                "sources": sources,
                "input_events": int(dedupe_stats["input_rows"]),
                "union_events": int(dedupe_stats["deduped_rows"]),
                "duplicate_events": int(dedupe_stats["duplicate_rows"]),
                "trades": int(_summary_value(result, ["summary", "trades"], 0)),
                "realistic_return_pct": float(_summary_value(result, ["summary", "realistic_return_pct"], 0.0)),
                "profit_factor": result.get("attribution", {}).get("profit_factor", 0.0),
                "win_rate_pct": float(_summary_value(result, ["summary", "win_rate_pct"], 0.0)),
                "max_dd_pct": float(_summary_value(result, ["summary", "max_dd_pct"], 0.0)),
                "mean_model_notional_share": round(float(union_share.mean()), 6) if not union_share.empty else 0.0,
                "min_model_notional_share": round(float(union_share.min()), 6) if not union_share.empty else 0.0,
                "max_model_notional_share": round(float(union_share.max()), 6) if not union_share.empty else 0.0,
                "mean_source_sizing_scale": round(float(union_scale.mean()), 6) if not union_scale.empty else 1.0,
                "summary_json": str(summary_json),
            }
        )

    active = [row for row in group_rows if int(row["trades"]) > 0]
    returns = [float(row["realistic_return_pct"]) for row in active]
    summary = {
        "selection_json": args.selection_json,
        "selection_key": args.selection_key,
        "run_dir": str(run_dir),
        "elapsed_seconds": round(time.time() - started, 2),
        "input_sleeve_rows": int(len(rows)),
        "union_groups": int(len(group_rows)),
        "duplicate_events_removed": int(duplicate_events_removed),
        "active_silo_sum_pct": round(float(np.sum(returns)), 6) if returns else 0.0,
        "active_months": int(len({row["execute_month"] for row in active})),
        "active_silos": int(len(active)),
        "trade_count": int(np.sum([int(row["trades"]) for row in active])) if active else 0,
        "worst_active_silo_pct": round(float(min(returns)), 6) if returns else 0.0,
        "best_active_silo_pct": round(float(max(returns)), 6) if returns else 0.0,
        "negative_active_silos": int(sum(1 for value in returns if value < 0.0)),
        "sizing_profile": str(args.sizing_profile),
        "share_multiplier": float(args.share_multiplier),
        "share_power": float(args.share_power),
        "share_cap": float(args.share_cap),
        "share_floor": float(args.share_floor),
        "group_rows": group_rows,
        "config": vars(args),
    }
    (run_dir / "summary.json").write_text(json.dumps(summary, indent=2, ensure_ascii=False, default=str), encoding="utf-8")
    _write_markdown(summary, run_dir / "summary.md")
    print(json.dumps(summary, indent=2, ensure_ascii=False, default=str), flush=True)


if __name__ == "__main__":
    main()
