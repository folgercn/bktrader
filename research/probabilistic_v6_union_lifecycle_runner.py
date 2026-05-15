#!/usr/bin/env python3
"""Replay selected probabilistic V6 union events through full reentry-window lifecycle.

Research-only. This consumes an existing union sizing sweep output and uses the
selected probability events as a breakout-lock gate for the full zero-initial
reentry-window replay. Event-level model_notional_share is interpreted as a
multiplier over the baseline schedule, not as an absolute order share.
"""

from __future__ import annotations

import argparse
import calendar
import json
import re
import time
from pathlib import Path

import numpy as np
import pandas as pd

import btc_eth_2026_jan_apr_impulse_bar_run as base
import eth_q1_breakout_t3_shape_compare as lifecycle


FLOW_CACHE_RE = re.compile(r"^(?P<symbol>[^_]+)_(?P<start>\d{8}T\d{6})_(?P<end>\d{8}T\d{6})_flow_1s\.pkl$")


def _as_utc(value: str | pd.Timestamp) -> pd.Timestamp:
    ts = pd.Timestamp(value)
    if ts.tzinfo is None:
        return ts.tz_localize("UTC")
    return ts.tz_convert("UTC")


def _month_start(month: str) -> pd.Timestamp:
    return pd.Timestamp(f"{month}-01T00:00:00Z")


def _month_end(month: str) -> pd.Timestamp:
    year, mon = [int(part) for part in month.split("-")]
    day = calendar.monthrange(year, mon)[1]
    return pd.Timestamp(f"{year:04d}-{mon:02d}-{day:02d}T23:59:59Z")


def _cache_path(symbol: str, start: pd.Timestamp, end: pd.Timestamp, cache_dir: str) -> Path | None:
    if not cache_dir:
        return None
    return (
        Path(cache_dir)
        / f"{symbol}_{start.strftime('%Y%m%dT%H%M%S')}_{end.strftime('%Y%m%dT%H%M%S')}_plain_1s.pkl"
    )


def _parse_flow_cache(path: Path, symbol: str) -> dict | None:
    match = FLOW_CACHE_RE.match(path.name)
    if not match or match.group("symbol") != symbol:
        return None
    return {
        "path": path,
        "start": pd.to_datetime(match.group("start"), format="%Y%m%dT%H%M%S", utc=True),
        "end": pd.to_datetime(match.group("end"), format="%Y%m%dT%H%M%S", utc=True),
    }


def _load_from_source_bars_cache(
    symbol: str,
    start: pd.Timestamp,
    end: pd.Timestamp,
    source_dirs: list[str],
) -> tuple[pd.DataFrame, dict] | None:
    candidates = []
    for raw_dir in source_dirs:
        source_dir = Path(raw_dir)
        if not source_dir.exists():
            continue
        for path in source_dir.glob(f"{symbol}_*_flow_1s.pkl"):
            parsed = _parse_flow_cache(path, symbol)
            if parsed and parsed["end"] >= start and parsed["start"] <= end:
                candidates.append(parsed)
    if not candidates:
        return None

    selected = []
    cursor = start
    while cursor <= end:
        covering = [item for item in candidates if item["start"] <= cursor and item["end"] >= cursor]
        if not covering:
            return None
        chosen = max(covering, key=lambda item: item["end"])
        selected.append(chosen)
        if chosen["end"] >= end:
            break
        cursor = chosen["end"] + pd.Timedelta(seconds=1)

    frames = []
    selected_paths = []
    for item in selected:
        frame = pd.read_pickle(item["path"])
        frame = frame.copy(deep=False)
        frame.index = pd.to_datetime(frame.index, utc=True)
        frames.append(frame)
        selected_paths.append(str(item["path"]))
    combined = pd.concat(frames).sort_index()
    combined = combined[~combined.index.duplicated(keep="last")]
    combined = combined[(combined.index >= start) & (combined.index <= end)].copy()
    if combined.empty or combined.index[0] > start or combined.index[-1] < end:
        return None
    return combined, {
        "source_cache_hit": True,
        "source_cache_paths": selected_paths,
        "continuous_second_rows": int(len(combined)),
        "first_second": combined.index[0].isoformat(),
        "last_second": combined.index[-1].isoformat(),
    }


def _load_or_build_second_bars(
    symbol: str,
    start: pd.Timestamp,
    end: pd.Timestamp,
    args: argparse.Namespace,
) -> tuple[pd.DataFrame, dict]:
    cache_path = _cache_path(symbol, start, end, str(args.bars_cache_dir))
    if cache_path is not None and cache_path.exists():
        second_bars = pd.read_pickle(cache_path)
        return second_bars, {
            "cache_path": str(cache_path),
            "cache_hit": True,
            "continuous_second_rows": int(len(second_bars)),
            "first_second": second_bars.index[0].isoformat() if not second_bars.empty else "",
            "last_second": second_bars.index[-1].isoformat() if not second_bars.empty else "",
        }

    source_cached = _load_from_source_bars_cache(symbol, start, end, list(args.source_bars_cache_dirs))
    if source_cached is not None:
        second_bars, stats = source_cached
        if cache_path is not None:
            cache_path.parent.mkdir(parents=True, exist_ok=True)
            second_bars.to_pickle(cache_path)
            stats = {**stats, "cache_path": str(cache_path), "cache_hit": False}
        return second_bars, stats

    tick_files = base.monthly_trade_files(symbol, start, end, Path(args.archive_root))
    second_bars, stats = lifecycle.build_continuous_second_bars(tick_files, start, end, int(args.chunksize))
    if cache_path is not None:
        cache_path.parent.mkdir(parents=True, exist_ok=True)
        second_bars.to_pickle(cache_path)
        stats = {**stats, "cache_path": str(cache_path), "cache_hit": False}
    return second_bars, stats


def _bool_series(series: pd.Series) -> pd.Series:
    if series.dtype == bool:
        return series
    return series.astype(str).str.lower().isin({"true", "1", "yes"})


def _load_config_summary(config_dir: Path) -> dict:
    path = config_dir / "summary.json"
    if not path.exists():
        raise FileNotFoundError(path)
    return json.loads(path.read_text(encoding="utf-8"))


def _events_for_group(config_dir: Path, month: str, symbol: str) -> pd.DataFrame:
    path = config_dir / f"execute_{month}" / symbol / "events_scored_union.csv"
    if not path.exists():
        raise FileNotFoundError(path)
    events = pd.read_csv(path, parse_dates=["signal_start", "signal_end", "touch_time"])
    month_mask = events["touch_time"].dt.strftime("%Y-%m").eq(month)
    if "quality_pass" in events.columns:
        month_mask &= _bool_series(events["quality_pass"])
    return events[month_mask].copy()


def _gate_key(signal_start, side: str, shape: str) -> tuple[str, str, str]:
    return (_as_utc(signal_start).isoformat(), str(side), str(shape))


def _selected_breakout_gate(
    events: pd.DataFrame,
    args: argparse.Namespace,
) -> tuple[dict[tuple[str, str, str], dict], dict]:
    gate: dict[tuple[str, str, str], dict] = {}
    if events.empty:
        return gate, {
            "selected_events": 0,
            "gate_keys": 0,
            "mean_sizing_scale": 0.0,
            "min_sizing_scale": 0.0,
            "max_sizing_scale": 0.0,
        }

    scales = []
    for _, row in events.iterrows():
        scale = float(row.get("model_notional_share", args.default_sizing_scale))
        if float(args.sizing_scale_floor) > 0.0:
            scale = max(scale, float(args.sizing_scale_floor))
        if float(args.sizing_scale_cap) > 0.0:
            scale = min(scale, float(args.sizing_scale_cap))
        scales.append(scale)
        key = _gate_key(row["signal_start"], row["side"], row["shape"])
        current = gate.get(key)
        if current is None or scale > float(current.get("sizing_scale", 0.0)):
            gate[key] = {
                "allow": True,
                "sizing_scale": scale,
                "event_id": str(row.get("event_id", "")),
                "touch_time": _as_utc(row["touch_time"]).isoformat(),
                "model_name": str(row.get("model_name", "")),
                "source_run": str(row.get("source_run", "")),
                "source_top_k": int(row.get("source_top_k", 0)) if pd.notna(row.get("source_top_k", np.nan)) else 0,
            }

    scale_values = np.array(scales, dtype="float64")
    return gate, {
        "selected_events": int(len(events)),
        "gate_keys": int(len(gate)),
        "mean_sizing_scale": round(float(scale_values.mean()), 6),
        "min_sizing_scale": round(float(scale_values.min()), 6),
        "max_sizing_scale": round(float(scale_values.max()), 6),
    }


def _run_group(
    *,
    config_name: str,
    config_dir: Path,
    group: dict,
    output_dir: Path,
    args: argparse.Namespace,
) -> dict:
    month = str(group["execute_month"])
    symbol = str(group["symbol"])
    execute_start = _month_start(month)
    execute_end = _month_end(month)
    data_start = execute_start - pd.Timedelta(days=int(args.warmup_days))
    data_end = execute_end + pd.Timedelta(days=int(args.tail_days))
    events = _events_for_group(config_dir, month, symbol)
    breakout_gate, gate_stats = _selected_breakout_gate(events, args)

    started = time.time()
    second_bars, build_stats = _load_or_build_second_bars(symbol, data_start, data_end, args)
    _, signal = lifecycle.build_signal_frame(second_bars, str(args.signal_timeframe))
    signal = signal[(signal.index >= data_start) & (signal.index <= data_end)].copy()

    ledger, diagnostics = lifecycle.run_second_bar_replay(
        second_bars,
        signal,
        initial_balance=float(args.initial_balance),
        breakout_shape=str(args.breakout_shape),
        replay_mode=str(args.replay_mode),
        zero_initial_reentry_anchor_mode=str(args.zero_initial_reentry_anchor_mode),
        reentry_trigger_observation_mode=str(args.reentry_trigger_observation_mode),
        reentry_max_deviation_bps=(
            None if float(args.reentry_max_deviation_bps) < 0.0 else float(args.reentry_max_deviation_bps)
        ),
        reentry_fill_price_mode=str(args.reentry_fill_price_mode),
        breakout_gate=breakout_gate,
    )

    group_dir = output_dir / config_name / f"execute_{month}" / symbol
    group_dir.mkdir(parents=True, exist_ok=True)
    ledger_path = group_dir / "lifecycle_ledger.csv"
    ledger.to_csv(ledger_path, index=False)
    summary = lifecycle.summarize_run(ledger, float(args.initial_balance))
    attribution = lifecycle.summarize_breakout_attribution(ledger)

    result = {
        "config": config_name,
        "execute_month": month,
        "symbol": symbol,
        "timeframe": str(args.signal_timeframe),
        "window": {
            "data_start": data_start.isoformat(),
            "data_end": data_end.isoformat(),
            "execute_start": execute_start.isoformat(),
            "execute_end": execute_end.isoformat(),
        },
        "input_group": group,
        "gate_stats": gate_stats,
        "summary": summary,
        "attribution": attribution,
        "diagnostics": diagnostics,
        "build_stats": build_stats,
        "ledger_path": str(ledger_path),
        "elapsed_seconds": round(time.time() - started, 2),
    }
    (group_dir / "lifecycle_summary.json").write_text(
        json.dumps(result, indent=2, ensure_ascii=False, default=str),
        encoding="utf-8",
    )
    return result


def _config_metrics(results: list[dict], one_shot_summary: dict) -> dict:
    active = [row for row in results if int(row["summary"].get("trades", 0)) > 0]
    returns = [float(row["summary"].get("return_pct", 0.0)) for row in active]
    one_shot_by_group = {
        (str(row["execute_month"]), str(row["symbol"])): row
        for row in one_shot_summary.get("group_rows", [])
    }
    one_shot_returns = [
        float(one_shot_by_group.get((row["execute_month"], row["symbol"]), {}).get("realistic_return_pct", 0.0))
        for row in active
    ]
    return {
        "active_silo_sum_pct": round(float(np.sum(returns)), 6) if returns else 0.0,
        "one_shot_active_silo_sum_pct": round(float(np.sum(one_shot_returns)), 6) if one_shot_returns else 0.0,
        "active_months": int(len({row["execute_month"] for row in active})),
        "active_silos": int(len(active)),
        "trade_count": int(np.sum([int(row["summary"].get("trades", 0)) for row in active])) if active else 0,
        "worst_active_silo_pct": round(float(min(returns)), 6) if returns else 0.0,
        "best_active_silo_pct": round(float(max(returns)), 6) if returns else 0.0,
        "negative_active_silos": int(sum(1 for value in returns if value < 0.0)),
    }


def _calendar_metrics(results: list[dict], calendar_months: list[str], calendar_symbols: list[str]) -> dict:
    result_by_group = {
        (str(row["execute_month"]), str(row["symbol"])): row
        for row in results
    }
    rows = []
    by_year: dict[str, float] = {}
    by_symbol: dict[str, float] = {}
    by_month: dict[str, float] = {}
    for month in calendar_months:
        for symbol in calendar_symbols:
            result = result_by_group.get((str(month), str(symbol)))
            if result is None:
                row = {
                    "execute_month": str(month),
                    "symbol": str(symbol),
                    "selected_events": 0,
                    "gate_keys": 0,
                    "trades": 0,
                    "return_pct": 0.0,
                    "max_dd_pct": 0.0,
                    "active": False,
                }
            else:
                summary = result.get("summary", {})
                row = {
                    "execute_month": str(month),
                    "symbol": str(symbol),
                    "selected_events": int(result.get("gate_stats", {}).get("selected_events", 0)),
                    "gate_keys": int(result.get("gate_stats", {}).get("gate_keys", 0)),
                    "trades": int(summary.get("trades", 0)),
                    "return_pct": float(summary.get("return_pct", 0.0)),
                    "max_dd_pct": float(summary.get("max_dd_pct", 0.0)),
                    "active": int(summary.get("trades", 0)) > 0,
                }
            rows.append(row)
            year = str(month)[:4]
            by_year[year] = by_year.get(year, 0.0) + float(row["return_pct"])
            by_symbol[str(symbol)] = by_symbol.get(str(symbol), 0.0) + float(row["return_pct"])
            by_month[str(month)] = by_month.get(str(month), 0.0) + float(row["return_pct"])

    returns = [float(row["return_pct"]) for row in rows]
    active_returns = [float(row["return_pct"]) for row in rows if bool(row["active"])]
    calendar_sum = float(np.sum(returns)) if returns else 0.0
    return {
        "calendar_months": [str(month) for month in calendar_months],
        "calendar_symbols": [str(symbol) for symbol in calendar_symbols],
        "calendar_symbol_months": int(len(rows)),
        "calendar_silo_sum_pct": round(calendar_sum, 6),
        "calendar_avg_symbol_month_pct": round(calendar_sum / float(len(rows)), 6) if rows else 0.0,
        "traded_symbol_months": int(sum(1 for row in rows if bool(row["active"]))),
        "flat_symbol_months": int(sum(1 for row in rows if not bool(row["active"]))),
        "trade_count": int(np.sum([int(row["trades"]) for row in rows])) if rows else 0,
        "worst_calendar_silo_pct": round(float(min(returns)), 6) if returns else 0.0,
        "best_calendar_silo_pct": round(float(max(returns)), 6) if returns else 0.0,
        "negative_calendar_silos": int(sum(1 for value in returns if value < 0.0)),
        "worst_active_silo_pct": round(float(min(active_returns)), 6) if active_returns else 0.0,
        "year_silo_sum_pct": {key: round(float(value), 6) for key, value in sorted(by_year.items())},
        "symbol_silo_sum_pct": {key: round(float(value), 6) for key, value in sorted(by_symbol.items())},
        "month_silo_sum_pct": {key: round(float(value), 6) for key, value in sorted(by_month.items())},
        "rows": rows,
    }


def _write_markdown(summary: dict, path: Path) -> None:
    lines = [
        "# Probabilistic V6 Union Lifecycle Replay",
        "",
        "范围：仅限 `research`。本阶段把概率 union 选中的 breakout 作为完整 reentry-window 生命周期的 breakout-lock gate。",
        "",
        "## Setup",
        "",
        f"- Source sweep: `{summary['source_sizing_run_dir']}`",
        f"- Signal timeframe: `{summary['config']['signal_timeframe']}`",
        f"- Breakout shape: `{summary['config']['breakout_shape']}`",
        "- Lifecycle: `dir2_zero_initial=true`, `zero_initial_mode=reentry_window`, `reentry_size_schedule=[0.20, 0.10]`, `max_trades_per_bar=2`",
        "- Sizing: `model_notional_share` 作为 baseline schedule 的倍率，例如 `1.30 => [0.26, 0.13]`，不是单笔 130% 仓位。",
        "",
        "## Config Metrics",
        "",
        "| Config | Lifecycle Return | One-shot Return | Trades | Active Silos | Worst Silo | Negative Silos |",
        "|---|---:|---:|---:|---:|---:|---:|",
    ]
    for row in summary["configs"]:
        m = row["metrics"]
        lines.append(
            f"| `{row['config']}` | {m['active_silo_sum_pct']:.4f}% | "
            f"{m['one_shot_active_silo_sum_pct']:.4f}% | {m['trade_count']} | "
            f"{m['active_silos']} | {m['worst_active_silo_pct']:.4f}% | {m['negative_active_silos']} |"
        )

    if any(row.get("calendar_metrics") for row in summary["configs"]):
        lines.extend(
            [
                "",
                "## Calendar Metrics",
                "",
                "未入选的 calendar symbol-month 按 `0%` 计入，避免把 active silo sum 误读成连续组合净值。",
                "",
                "| Config | Calendar Sum | Avg / Symbol-Month | Traded Silos | Flat Silos | Worst Calendar Silo | Negative Silos | By Symbol | By Year |",
                "|---|---:|---:|---:|---:|---:|---:|---|---|",
            ]
        )
        for row in summary["configs"]:
            cm = row.get("calendar_metrics") or {}
            if not cm:
                continue
            by_symbol = ", ".join(
                f"{symbol}:{value:.4f}%"
                for symbol, value in cm.get("symbol_silo_sum_pct", {}).items()
            )
            by_year = ", ".join(
                f"{year}:{value:.4f}%"
                for year, value in cm.get("year_silo_sum_pct", {}).items()
            )
            lines.append(
                f"| `{row['config']}` | {cm['calendar_silo_sum_pct']:.4f}% | "
                f"{cm['calendar_avg_symbol_month_pct']:.4f}% | {cm['traded_symbol_months']} | "
                f"{cm['flat_symbol_months']} | {cm['worst_calendar_silo_pct']:.4f}% | "
                f"{cm['negative_calendar_silos']} | `{by_symbol}` | `{by_year}` |"
            )

    lines.extend(
        [
            "",
            "## Group Rows",
            "",
            "| Config | Month | Symbol | Events | Gate Keys | Allowed Locks | Rejected Locks | Trades | Return | Max DD | Entry Reasons |",
            "|---|---|---|---:|---:|---:|---:|---:|---:|---:|---|",
        ]
    )
    for config in summary["configs"]:
        for result in config["results"]:
            gate = result.get("diagnostics", {}).get("probability_gate", {})
            allowed = sum(int(value) for value in gate.get("allowed", {}).values())
            rejected = sum(int(value) for value in gate.get("rejected", {}).values())
            s = result["summary"]
            lines.append(
                f"| `{config['config']}` | `{result['execute_month']}` | `{result['symbol']}` | "
                f"{result['gate_stats']['selected_events']} | {result['gate_stats']['gate_keys']} | "
                f"{allowed} | {rejected} | {s['trades']} | {s['return_pct']:.2f}% | "
                f"{s['max_dd_pct']:.2f}% | `{s.get('entry_reasons', {})}` |"
            )

    lines.extend(
        [
            "",
            "## Read",
            "",
            "这轮复测解决的是上一阶段的最大 caveat：不再只看 one-shot 1s execution，而是让概率模型先决定哪些 breakout lock 可以进入完整 reentry-window 生命周期。",
            "",
            "结果仍然是 research post-selection，尚未做 cross-year / cross-asset 外推；不能视为实盘候选。",
            "",
        ]
    )
    path.write_text("\n".join(lines), encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Run full lifecycle replay for V6 union sizing outputs")
    parser.add_argument(
        "--source-sizing-run-dir",
        default="research/probabilistic_v6_runs/walkforward_2025m06_2026apr_combo_baseline_short_speed/union_sizing_calibration_sweep_candidate_001",
    )
    parser.add_argument(
        "--configs",
        nargs="+",
        default=["power0_fixed_1p30", "quality_edge_return_mult_1p20_cap_1p80"],
    )
    parser.add_argument(
        "--output-dir",
        default="research/probabilistic_v6_runs/walkforward_2025m06_2026apr_combo_baseline_short_speed/union_lifecycle_reentry_window_candidate_001",
    )
    parser.add_argument("--symbols", nargs="+", default=["BTCUSDT", "ETHUSDT"])
    parser.add_argument("--months", nargs="+", default=[])
    parser.add_argument(
        "--calendar-months",
        nargs="+",
        default=[],
        help="Optional fixed calendar months to report; missing month/symbol groups are counted as 0%.",
    )
    parser.add_argument(
        "--calendar-symbols",
        nargs="+",
        default=[],
        help="Optional fixed calendar symbols to report; defaults to --symbols.",
    )
    parser.add_argument("--max-groups", type=int, default=0, help="0 runs all matching month/symbol groups")
    parser.add_argument("--signal-timeframe", default="1h")
    parser.add_argument("--breakout-shape", default="original_t2")
    parser.add_argument("--replay-mode", default="live_intrabar_sma5")
    parser.add_argument("--zero-initial-reentry-anchor-mode", default="rolling")
    parser.add_argument("--reentry-trigger-observation-mode", default="bar_extrema")
    parser.add_argument("--reentry-max-deviation-bps", type=float, default=-1.0)
    parser.add_argument("--reentry-fill-price-mode", default="planned")
    parser.add_argument("--warmup-days", type=int, default=2)
    parser.add_argument("--tail-days", type=int, default=1)
    parser.add_argument("--archive-root", default="dataset/archive")
    parser.add_argument("--chunksize", type=int, default=5_000_000)
    parser.add_argument("--bars-cache-dir", default="research/probabilistic_v6_runs/lifecycle_bars_cache")
    parser.add_argument(
        "--source-bars-cache-dirs",
        nargs="+",
        default=[
            "research/probabilistic_v6_runs/walkforward_delay60_original_t2_feature60_valbest/bars_cache",
            "research/probabilistic_v6_runs/2025_m03_m06_original_t2_delay60/bars_cache",
            "research/probabilistic_v6_runs/2025_q3_original_t2/bars_cache",
            "research/probabilistic_v6_runs/2026_jan_mar_baseline_plus_t3_delay60/bars_cache",
            "research/probabilistic_v6_runs/2026_04_original_t2_delay60_fresh/bars_cache",
        ],
    )
    parser.add_argument("--initial-balance", type=float, default=100000.0)
    parser.add_argument("--default-sizing-scale", type=float, default=1.0)
    parser.add_argument("--sizing-scale-cap", type=float, default=1.80)
    parser.add_argument("--sizing-scale-floor", type=float, default=0.0)
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    started = time.time()
    source_dir = Path(args.source_sizing_run_dir)
    output_dir = Path(args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)

    config_rows = []
    for config_name in args.configs:
        config_dir = source_dir / config_name
        one_shot_summary = _load_config_summary(config_dir)
        group_rows = [
            row
            for row in one_shot_summary.get("group_rows", [])
            if str(row.get("symbol", "")) in set(str(symbol) for symbol in args.symbols)
        ]
        if args.months:
            month_filter = {str(month) for month in args.months}
            group_rows = [row for row in group_rows if str(row.get("execute_month", "")) in month_filter]
        if int(args.max_groups) > 0:
            group_rows = group_rows[: int(args.max_groups)]
        results = []
        for group in group_rows:
            print(
                f"running config={config_name} month={group['execute_month']} symbol={group['symbol']}",
                flush=True,
            )
            result = _run_group(
                config_name=config_name,
                config_dir=config_dir,
                group=group,
                output_dir=output_dir,
                args=args,
            )
            results.append(result)
            print(
                f"  return={result['summary']['return_pct']:.2f}% trades={result['summary']['trades']} "
                f"events={result['gate_stats']['selected_events']} elapsed={result['elapsed_seconds']}s",
                flush=True,
            )
        config_rows.append(
            {
                "config": config_name,
                "source_summary_json": str(config_dir / "summary.json"),
                "metrics": _config_metrics(results, one_shot_summary),
                "calendar_metrics": _calendar_metrics(
                    results,
                    [str(month) for month in args.calendar_months],
                    [str(symbol) for symbol in (args.calendar_symbols or args.symbols)],
                )
                if args.calendar_months
                else {},
                "results": results,
            }
        )

    summary = {
        "source_sizing_run_dir": str(source_dir),
        "output_dir": str(output_dir),
        "elapsed_seconds": round(time.time() - started, 2),
        "config": vars(args),
        "configs": config_rows,
    }
    (output_dir / "summary.json").write_text(
        json.dumps(summary, indent=2, ensure_ascii=False, default=str),
        encoding="utf-8",
    )
    _write_markdown(summary, output_dir / "summary.md")
    print(json.dumps({"summary_json": str(output_dir / "summary.json")}, indent=2), flush=True)


if __name__ == "__main__":
    main()
