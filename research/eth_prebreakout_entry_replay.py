#!/usr/bin/env python3
"""ETH pre-breakout entry replay using empirical state hit probabilities.

Research-only. Entry can happen before the 1h breakout level is touched when a
1m state has a sufficiently high empirical probability of hitting the breakout
level. Execution and exit management use continuous 1s bars and the same
structure-trailing machinery as micro breakout structure replay.
"""

from __future__ import annotations

import argparse
import json
import time
from pathlib import Path

import numpy as np
import pandas as pd

import btc_eth_micro_breakout_structure as micro
import eth_prebreakout_markov_probability as prob


def _boolish(value: str) -> float:
    lowered = value.strip().lower()
    if lowered in {"true", "false"}:
        return 1.0 if lowered == "true" else 0.0
    try:
        return float(value)
    except ValueError:
        return value.strip()


def parse_variant(raw: str) -> tuple[str, dict]:
    name, values = raw.split("=", 1) if "=" in raw else (raw, "")
    params = micro.parse_variant(
        "pre_base=trend:signal,exit:structure,skip_weak:true,struct_start:1.0,struct_bars:4,hold:0"
    )[1]
    params.update(
        {
            "pre_entry_mode": "immediate",
            "pre_min_distance_atr": 0.02,
            "pre_max_distance_atr": 0.10,
            "pre_min_hit_rate": 70.0,
            "pre_strong_hit_rate": 75.0,
            "pre_min_samples": 80,
            "pre_horizon_minutes": 60,
            "pre_fail_atr": 0.20,
        }
    )
    key_map = {
        "mode": "pre_entry_mode",
        "min_dist": "pre_min_distance_atr",
        "max_dist": "pre_max_distance_atr",
        "prob": "pre_min_hit_rate",
        "strong_prob": "pre_strong_hit_rate",
        "samples": "pre_min_samples",
        "horizon": "pre_horizon_minutes",
        "fail": "pre_fail_atr",
        "hold": "max_hold_hours",
        "struct_start": "structure_start_atr",
        "struct_bars": "structure_bars",
        "base_share": "base_share",
        "strong_share": "strong_share",
        "sl": "initial_stop_atr",
    }
    for item in values.split(","):
        if not item.strip():
            continue
        key, value = item.split(":", 1)
        mapped = key_map.get(key.strip())
        if mapped is None:
            raise ValueError(f"unknown variant key {key} in {raw}")
        params[mapped] = _boolish(value)
    for key in ("pre_min_samples", "pre_horizon_minutes", "max_hold_hours", "structure_bars"):
        params[key] = int(params[key])
    return name, params


def load_state_table(summary_path: Path) -> dict[tuple[str, str, str], dict]:
    data = json.loads(summary_path.read_text())
    table: dict[tuple[str, str, str], dict] = {}
    for row in data["top_states"]:
        key = (str(row["distance_bucket"]), str(row["speed5_bucket"]), str(row["eff_bucket"]))
        table[key] = row
    return table


def state_key(distance_atr: float, speed5_atr: float, eff15: float) -> tuple[str, str, str]:
    return (
        prob._bucket(distance_atr, [0.10, 0.20, 0.30, 0.40], ["0-0.10", "0.10-0.20", "0.20-0.30", "0.30-0.40", "0.40+"]),
        prob._bucket(speed5_atr, [-0.03, 0.00, 0.03, 0.08], ["<-0.03", "-0.03-0", "0-0.03", "0.03-0.08", ">=0.08"]),
        prob._bucket(eff15, [0.0, 0.2, 0.4, 0.6], ["<0", "0-0.2", "0.2-0.4", "0.4-0.6", ">=0.6"]),
    )


def candidate_from_minute(row: pd.Series, high_value: float, low_value: float, params: dict, state_table: dict):
    atr = float(row.get("atr", np.nan))
    ema20 = float(row.get("ema20", np.nan))
    slope = float(row.get("ema20_slope", np.nan))
    close = float(row["close"])
    if not np.isfinite(atr) or atr <= 0 or not np.isfinite(ema20) or not np.isfinite(slope):
        return None

    candidates = []
    if close > ema20 and slope > 0:
        level = float(row.get("prev_high_8", np.nan))
        if np.isfinite(level) and close < level and high_value < level:
            distance = (level - close) / atr
            candidates.append(("long", level, distance))
    if close < ema20 and slope < 0:
        level = float(row.get("prev_low_8", np.nan))
        if np.isfinite(level) and close > level and low_value > level:
            distance = (close - level) / atr
            candidates.append(("short", level, distance))

    for side, level, distance in candidates:
        if distance < float(params["pre_min_distance_atr"]) or distance > float(params["pre_max_distance_atr"]):
            continue
        mult = 1.0 if side == "long" else -1.0
        speed5 = mult * float(row.get("ret_5m", np.nan)) / atr
        eff15 = mult * float(row.get("eff_15m", np.nan))
        key = state_key(distance, speed5, eff15)
        state = state_table.get(key)
        if state is None:
            continue
        if int(state["samples"]) < int(params["pre_min_samples"]):
            continue
        if float(state["hit_rate"]) < float(params["pre_min_hit_rate"]):
            continue
        tier = "strong" if float(state["hit_rate"]) >= float(params["pre_strong_hit_rate"]) else "base"
        share = float(params["strong_share"]) if tier == "strong" else float(params["base_share"])
        return {
            "side": side,
            "level": level,
            "close": close,
            "distance_atr": float(distance),
            "speed5_atr": float(speed5),
            "eff15": float(eff15),
            "state_key": "|".join(key),
            "state_hit_rate": float(state["hit_rate"]),
            "state_samples": int(state["samples"]),
            "quality_tier": tier,
            "notional_share": share,
            "atr": atr,
        }
    return None


def fake_signal(row: pd.Series, entry_raw: float, candidate: dict) -> pd.Series:
    return pd.Series(
        {
            "atr": float(candidate["atr"]),
            "low": float(entry_raw),
            "high": float(entry_raw),
            "close": float(row["close"]),
        },
        name=pd.Timestamp(row["hour_start"]),
    )


def resolve_entry(
    second_bars: pd.DataFrame,
    start_pos: int,
    candidate: dict,
    params: dict,
) -> tuple[int | None, float, str]:
    if str(params.get("pre_entry_mode", "immediate")) == "immediate":
        if start_pos >= len(second_bars.index):
            return None, 0.0, "no_second"
        raw_entry = float(second_bars["close"].to_numpy(dtype="float64", copy=False)[start_pos])
        return start_pos, raw_entry, ""

    idx = second_bars.index
    highs = second_bars["high"].to_numpy(dtype="float64", copy=False)
    lows = second_bars["low"].to_numpy(dtype="float64", copy=False)
    if start_pos >= len(idx):
        return None, 0.0, "no_second"
    horizon_end = idx[start_pos] + pd.Timedelta(minutes=int(params["pre_horizon_minutes"]))
    end_pos = min(int(idx.searchsorted(horizon_end, side="right")), len(idx))
    atr = float(candidate["atr"])
    level = float(candidate["level"])
    if candidate["side"] == "long":
        fail_level = float(candidate["close"]) - float(params["pre_fail_atr"]) * atr
        for pos in range(start_pos, end_pos):
            if float(lows[pos]) <= fail_level:
                return None, 0.0, "pre_fail"
            if float(highs[pos]) >= level:
                return pos, level, ""
    else:
        fail_level = float(candidate["close"]) + float(params["pre_fail_atr"]) * atr
        for pos in range(start_pos, end_pos):
            if float(highs[pos]) >= fail_level:
                return None, 0.0, "pre_fail"
            if float(lows[pos]) <= level:
                return pos, level, ""
    return None, 0.0, "pre_timeout"


def append_entry(logs: list[dict], position: dict, balance: float, candidate: dict) -> None:
    micro.append_entry(logs, position, balance)
    logs[-1].update(
        {
            "reason": "PreBreakout-Prob",
            "breakout_level": candidate["level"],
            "pre_distance_atr": candidate["distance_atr"],
            "pre_speed5_atr": candidate["speed5_atr"],
            "pre_eff15": candidate["eff15"],
            "state_key": candidate["state_key"],
            "state_hit_rate": candidate["state_hit_rate"],
            "state_samples": candidate["state_samples"],
        }
    )


def append_exit(
    logs: list[dict],
    position: dict,
    *,
    raw_exit: float,
    time_value,
    reason: str,
    balance: float,
    params: dict,
) -> float:
    new_balance = micro.append_exit(
        logs,
        position,
        raw_exit=raw_exit,
        time_value=time_value,
        reason=reason,
        balance=balance,
        params=params,
    )
    logs[-1].update(
        {
            "breakout_level": position.get("breakout_level", np.nan),
            "pre_distance_atr": position.get("pre_distance_atr", np.nan),
            "state_hit_rate": position.get("state_hit_rate", np.nan),
            "state_key": position.get("state_key", ""),
        }
    )
    return new_balance


def simulate_position(
    second_bars: pd.DataFrame,
    events_by_end: dict[pd.Timestamp, pd.Series],
    row: pd.Series,
    candidate: dict,
    params: dict,
    *,
    start_pos: int,
    balance: float,
):
    idx = second_bars.index
    highs = second_bars["high"].to_numpy(dtype="float64", copy=False)
    lows = second_bars["low"].to_numpy(dtype="float64", copy=False)
    closes = second_bars["close"].to_numpy(dtype="float64", copy=False)
    entry_pos, entry_raw, resolve_skip = resolve_entry(second_bars, start_pos, candidate, params)
    if resolve_skip:
        return None, balance, resolve_skip
    entry_time = idx[int(entry_pos)]
    if candidate["side"] == "long" and entry_raw >= float(candidate["level"]):
        if str(params.get("pre_entry_mode", "immediate")) == "immediate":
            return None, balance, "already_broken"
    if candidate["side"] == "short" and entry_raw <= float(candidate["level"]):
        if str(params.get("pre_entry_mode", "immediate")) == "immediate":
            return None, balance, "already_broken"

    sig = fake_signal(row, entry_raw, candidate)
    micro_info = {
        "quality_tier": candidate["quality_tier"],
        "notional_share": candidate["notional_share"],
        "micro_speed_atr": candidate["speed5_atr"],
        "micro_fast_atr": candidate["speed5_atr"],
        "micro_efficiency": candidate["eff15"],
        "micro_close_pos": np.nan,
    }
    position = micro.open_position(sig, candidate["side"], entry_raw, entry_time, balance, params, micro_info)
    if position is None:
        return None, balance, "min_stop"
    position.update(
        {
            "breakout_level": candidate["level"],
            "pre_distance_atr": candidate["distance_atr"],
            "state_hit_rate": candidate["state_hit_rate"],
            "state_key": candidate["state_key"],
        }
    )
    logs: list[dict] = []
    append_entry(logs, position, balance, candidate)

    max_hold_hours = int(params["max_hold_hours"])
    if max_hold_hours > 0:
        max_hold_end = entry_time + pd.Timedelta(hours=max_hold_hours)
        end_pos = min(int(idx.searchsorted(max_hold_end, side="left")), len(idx) - 1)
    else:
        max_hold_end = None
        end_pos = len(idx) - 1

    for pos in range(int(entry_pos) + 1, end_pos + 1):
        bar_time = idx[pos]
        high_value = float(highs[pos])
        low_value = float(lows[pos])
        close_value = float(closes[pos])
        micro.update_excursion(position, high_value, low_value, params)
        triggered, raw_exit, reason = micro.stop_trigger(position, high_value, low_value)
        if triggered:
            balance = append_exit(logs, position, raw_exit=raw_exit, time_value=bar_time, reason=reason, balance=balance, params=params)
            return logs, balance, ""

        event = events_by_end.get(bar_time)
        if event is not None and bar_time > entry_time:
            exit_now, event_reason = micro.apply_signal_event(position, event, params)
            if exit_now:
                balance = append_exit(logs, position, raw_exit=close_value, time_value=bar_time, reason=event_reason, balance=balance, params=params)
                return logs, balance, ""

        if max_hold_end is not None and bar_time >= max_hold_end:
            balance = append_exit(logs, position, raw_exit=close_value, time_value=bar_time, reason="MaxHoldExit", balance=balance, params=params)
            return logs, balance, ""

    balance = append_exit(logs, position, raw_exit=float(closes[end_pos]), time_value=idx[end_pos], reason="FinalMarkToMarket", balance=balance, params=params)
    return logs, balance, ""


def run_strategy(second_bars: pd.DataFrame, minute: pd.DataFrame, signal: pd.DataFrame, params: dict, state_table: dict, *, initial_balance: float) -> dict:
    idx = second_bars.index
    highs_1m = minute["high"].to_numpy(dtype="float64", copy=False)
    lows_1m = minute["low"].to_numpy(dtype="float64", copy=False)
    events_by_end = {pd.Timestamp(row["bar_end"]): row for _, row in signal.iterrows()}
    balance = float(initial_balance)
    logs: list[dict] = []
    last_exit_time = pd.Timestamp.min.tz_localize("UTC")
    diagnostics = {
        "candidate_minutes": 0,
        "entries": 0,
        "busy_skipped": 0,
        "already_broken_skipped": 0,
        "pre_fail_skipped": 0,
        "pre_timeout_skipped": 0,
        "min_stop_skipped": 0,
        "state_filtered": 0,
        "quality_counts": {"base": 0, "strong": 0},
    }
    for pos, row in minute.iterrows():
        candidate = candidate_from_minute(row, float(highs_1m[int(pos)]), float(lows_1m[int(pos)]), params, state_table)
        if candidate is None:
            diagnostics["state_filtered"] += 1
            continue
        diagnostics["candidate_minutes"] += 1
        entry_time = pd.Timestamp(row["minute_start"]) + pd.Timedelta(minutes=1)
        if entry_time <= last_exit_time:
            diagnostics["busy_skipped"] += 1
            continue
        start_pos = int(idx.searchsorted(entry_time, side="left"))
        trade_logs, new_balance, skip_reason = simulate_position(
            second_bars,
            events_by_end,
            row,
            candidate,
            params,
            start_pos=start_pos,
            balance=balance,
        )
        if skip_reason == "already_broken":
            diagnostics["already_broken_skipped"] += 1
            continue
        if skip_reason == "pre_fail":
            diagnostics["pre_fail_skipped"] += 1
            continue
        if skip_reason == "pre_timeout":
            diagnostics["pre_timeout_skipped"] += 1
            continue
        if skip_reason == "min_stop":
            diagnostics["min_stop_skipped"] += 1
            continue
        if not trade_logs:
            continue
        logs.extend(trade_logs)
        balance = new_balance
        diagnostics["entries"] += 1
        diagnostics["quality_counts"][candidate["quality_tier"]] += 1
        last_exit_time = pd.Timestamp(trade_logs[-1]["time"])

    ledger = pd.DataFrame(logs)
    return {
        "summary": micro.summarize_ledger(ledger, initial_balance, params),
        "diagnostics": diagnostics,
        "ledger": ledger,
    }


def write_markdown(summary: dict, path: Path) -> None:
    lines = [
        f"# ETH Pre-Breakout Entry 1s Replay ({summary['start']} to {summary['end']})",
        "",
        "Scope: research-only. Entry uses 2025-trained empirical state hit probabilities before the 1h breakout level is touched. Execution and exits use continuous 1s bars and structure trailing.",
        "",
        "| Variant | Trades | Realistic | Raw | 2bps Slip No Fee | Win Rate | Max DD | Avg Share | Med Hold | Med MFE ATR | Exits | Quality | Candidate Min | Entries | Busy | Pre Fail | Pre Timeout | Already Broken |",
        "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---|---:|---:|---:|---:|---:|---:|",
    ]
    for result in summary["results"]:
        s = result["summary"]
        d = result["diagnostics"]
        lines.append(
            f"| `{result['variant']}` | {s['trades']} | {s['realistic_return_pct']:.4f}% | "
            f"{s['raw_no_fee_no_slip_return_pct']:.4f}% | {s['price_pnl_with_2bps_slip_no_fee_return_pct']:.4f}% | "
            f"{s['win_rate_pct']:.2f}% | {s['max_dd_pct']:.4f}% | {s.get('avg_notional_share', 0.0):.4f} | "
            f"{s.get('median_hold_seconds', 0.0):.2f}s | {s.get('median_mfe_atr', 0.0):.4f} | "
            f"`{s['exit_reasons']}` | `{s.get('quality_trades', {})}` | {d['candidate_minutes']} | "
            f"{d['entries']} | {d['busy_skipped']} | {d.get('pre_fail_skipped', 0)} | "
            f"{d.get('pre_timeout_skipped', 0)} | {d['already_broken_skipped']} |"
        )
    lines.extend(["", "## Files", ""])
    lines.append(f"- Summary JSON: `{summary['summary_path']}`")
    for result in summary["results"]:
        lines.append(f"- `{result['variant']}` ledger: `{result['ledger_path']}`")
    lines.append("")
    path.write_text("\n".join(lines), encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="ETH pre-breakout entry replay")
    parser.add_argument("--symbol", default="ETHUSDT")
    parser.add_argument("--start", default="2026-01-01T00:00:00Z")
    parser.add_argument("--end", default="2026-04-30T23:59:59Z")
    parser.add_argument("--train-summary-json", default="research/eth_2025_prebreakout_markov_probability_summary.json")
    parser.add_argument("--archive-root", default=str(micro.DEFAULT_ARCHIVE_ROOT))
    parser.add_argument("--cache-root", default=str(micro.DEFAULT_CACHE_ROOT))
    parser.add_argument("--no-cache", action="store_true")
    parser.add_argument("--chunksize", type=int, default=2_000_000)
    parser.add_argument("--initial-balance", type=float, default=100000.0)
    parser.add_argument(
        "--variants",
        nargs="+",
        default=[
            "pre_d010_p70=max_dist:0.10,prob:70,strong_prob:75,samples:80",
            "pre_d010_p75=max_dist:0.10,prob:75,strong_prob:78,samples:80",
            "pre_d020_p65=max_dist:0.20,prob:65,strong_prob:75,samples:80",
        ],
    )
    parser.add_argument("--summary-json", default="research/eth_prebreakout_entry_replay_summary.json")
    parser.add_argument("--markdown", default="research/20260507_eth_prebreakout_entry_replay.md")
    parser.add_argument("--ledger-prefix", default="research/tmp_eth_prebreakout_entry_replay")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    started = time.time()
    start = micro.base._as_utc(args.start)
    end = micro.base._as_utc(args.end)
    second_bars, build_stats = micro.load_or_build_second_bars(
        args.symbol,
        start,
        end,
        Path(args.archive_root),
        args.chunksize,
        Path(args.cache_root),
        not args.no_cache,
    )
    minute = prob.build_minute_context(second_bars, "1h")
    _, signal, trend = micro.build_frames(second_bars, "1h", "4h")
    state_table = load_state_table(Path(args.train_summary_json))
    variants = [parse_variant(raw) for raw in args.variants]
    results = []
    print(
        f"{args.symbol}: second_rows={len(second_bars)} minute_rows={len(minute)} signal_rows={len(signal)} train_states={len(state_table)}",
        flush=True,
    )
    for variant_name, params in variants:
        print(f"running {variant_name}", flush=True)
        result = run_strategy(second_bars, minute, signal, params, state_table, initial_balance=args.initial_balance)
        ledger_path = Path(f"{args.ledger_prefix}_{variant_name}_ledger.csv")
        result["ledger"].to_csv(ledger_path, index=False)
        del result["ledger"]
        result.update({"variant": variant_name, "params": params, "ledger_path": str(ledger_path)})
        s = result["summary"]
        print(
            f"{variant_name}: realistic={s['realistic_return_pct']:.4f}% raw={s['raw_no_fee_no_slip_return_pct']:.4f}% "
            f"trades={s['trades']} win={s['win_rate_pct']:.2f}% dd={s['max_dd_pct']:.4f}% diag={result['diagnostics']}",
            flush=True,
        )
        results.append(result)

    summary = {
        "start": start.isoformat(),
        "end": end.isoformat(),
        "symbol": args.symbol,
        "train_summary_json": args.train_summary_json,
        "build_stats": build_stats,
        "results": results,
        "summary_path": args.summary_json,
        "markdown_path": args.markdown,
        "elapsed_seconds": round(time.time() - started, 2),
    }
    Path(args.summary_json).write_text(json.dumps(summary, indent=2, ensure_ascii=False), encoding="utf-8")
    write_markdown(summary, Path(args.markdown))
    print(json.dumps({"summary_path": args.summary_json, "markdown_path": args.markdown, "elapsed_seconds": summary["elapsed_seconds"]}, indent=2), flush=True)


if __name__ == "__main__":
    main()
