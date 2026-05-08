#!/usr/bin/env python3
"""ETH pre-breakout watch plus post-touch confirmation replay.

Research-only. This runner uses the empirical pre-breakout state table only to
arm a setup before the 1h breakout level is touched. It does not enter at the
pre-breakout price or at the breakout level. Entry happens only after a real
1s close confirms continuation beyond the touched level.
"""

from __future__ import annotations

import argparse
import json
import time
from pathlib import Path

import numpy as np
import pandas as pd

import btc_eth_micro_breakout_structure as micro
import eth_prebreakout_entry_replay as pre
import eth_prebreakout_markov_probability as prob


def _maybe_number(value: str):
    lowered = value.strip().lower()
    if lowered in {"true", "false"}:
        return 1.0 if lowered == "true" else 0.0
    try:
        return float(value)
    except ValueError:
        return value.strip()


def parse_variant(raw: str) -> tuple[str, dict]:
    name, values = raw.split("=", 1) if "=" in raw else (raw, "")
    params = pre.parse_variant(
        "post_base=max_dist:0.10,prob:70,strong_prob:75,samples:80,hold:0"
    )[1]
    params.update(
        {
            "confirm_atr": 0.05,
            "confirm_seconds": 300,
            "post_touch_fail_atr": 0.08,
            "dedupe_one_level_per_hour": 1.0,
        }
    )
    key_map = {
        "min_dist": "pre_min_distance_atr",
        "max_dist": "pre_max_distance_atr",
        "prob": "pre_min_hit_rate",
        "strong_prob": "pre_strong_hit_rate",
        "samples": "pre_min_samples",
        "horizon": "pre_horizon_minutes",
        "fail": "pre_fail_atr",
        "confirm": "confirm_atr",
        "confirm_s": "confirm_seconds",
        "post_fail": "post_touch_fail_atr",
        "hold": "max_hold_hours",
        "struct_start": "structure_start_atr",
        "struct_bars": "structure_bars",
        "base_share": "base_share",
        "strong_share": "strong_share",
        "sl": "initial_stop_atr",
        "max_ext": "max_entry_extension_atr",
    }
    for item in values.split(","):
        if not item.strip():
            continue
        key, value = item.split(":", 1)
        mapped = key_map.get(key.strip())
        if mapped is None:
            raise ValueError(f"unknown variant key {key} in {raw}")
        params[mapped] = _maybe_number(value)
    for key in ("pre_min_samples", "pre_horizon_minutes", "max_hold_hours", "structure_bars", "confirm_seconds"):
        params[key] = int(params[key])
    return name, params


def resolve_posttouch_entry(
    second_bars: pd.DataFrame,
    start_pos: int,
    candidate: dict,
    params: dict,
) -> tuple[int | None, float, str, dict]:
    idx = second_bars.index
    highs = second_bars["high"].to_numpy(dtype="float64", copy=False)
    lows = second_bars["low"].to_numpy(dtype="float64", copy=False)
    closes = second_bars["close"].to_numpy(dtype="float64", copy=False)
    if start_pos >= len(idx):
        return None, 0.0, "no_second", {}

    atr = float(candidate["atr"])
    level = float(candidate["level"])
    side = str(candidate["side"])
    horizon_end = idx[start_pos] + pd.Timedelta(minutes=int(params["pre_horizon_minutes"]))
    end_pos = min(int(idx.searchsorted(horizon_end, side="right")), len(idx))
    confirm_delta = float(params["confirm_atr"]) * atr
    post_fail_delta = float(params["post_touch_fail_atr"]) * atr
    confirm_limit = pd.Timedelta(seconds=int(params["confirm_seconds"]))

    if side == "long":
        pre_fail_level = float(candidate["close"]) - float(params["pre_fail_atr"]) * atr
        confirm_level = level + confirm_delta
        post_fail_level = level - post_fail_delta
    else:
        pre_fail_level = float(candidate["close"]) + float(params["pre_fail_atr"]) * atr
        confirm_level = level - confirm_delta
        post_fail_level = level + post_fail_delta

    touch_pos: int | None = None
    confirm_deadline = pd.Timestamp.max.tz_localize("UTC")
    for pos in range(start_pos, end_pos):
        bar_time = idx[pos]
        high_value = float(highs[pos])
        low_value = float(lows[pos])
        close_value = float(closes[pos])

        if touch_pos is None:
            if side == "long":
                if low_value <= pre_fail_level:
                    return None, 0.0, "pre_fail", {}
                if high_value >= level:
                    touch_pos = pos
                    confirm_deadline = min(bar_time + confirm_limit, horizon_end)
            else:
                if high_value >= pre_fail_level:
                    return None, 0.0, "pre_fail", {}
                if low_value <= level:
                    touch_pos = pos
                    confirm_deadline = min(bar_time + confirm_limit, horizon_end)

        if touch_pos is not None:
            if side == "long":
                if low_value <= post_fail_level:
                    return None, 0.0, "post_touch_fail", {"touch_time": idx[touch_pos]}
                if close_value >= confirm_level:
                    extension = (close_value - level) / atr
                    if extension > float(params["max_entry_extension_atr"]):
                        return None, 0.0, "entry_extension", {"touch_time": idx[touch_pos], "extension_atr": extension}
                    return pos, close_value, "", {
                        "touch_time": idx[touch_pos],
                        "confirm_level": confirm_level,
                        "extension_atr": extension,
                    }
            else:
                if high_value >= post_fail_level:
                    return None, 0.0, "post_touch_fail", {"touch_time": idx[touch_pos]}
                if close_value <= confirm_level:
                    extension = (level - close_value) / atr
                    if extension > float(params["max_entry_extension_atr"]):
                        return None, 0.0, "entry_extension", {"touch_time": idx[touch_pos], "extension_atr": extension}
                    return pos, close_value, "", {
                        "touch_time": idx[touch_pos],
                        "confirm_level": confirm_level,
                        "extension_atr": extension,
                    }
            if bar_time >= confirm_deadline:
                return None, 0.0, "confirm_timeout", {"touch_time": idx[touch_pos]}

    if touch_pos is None:
        return None, 0.0, "pre_timeout", {}
    return None, 0.0, "confirm_timeout", {"touch_time": idx[touch_pos]}


def append_entry(logs: list[dict], position: dict, balance: float, candidate: dict) -> None:
    micro.append_entry(logs, position, balance)
    logs[-1].update(
        {
            "reason": "PostTouch-Confirm",
            "breakout_level": candidate["level"],
            "pre_distance_atr": candidate["distance_atr"],
            "pre_speed5_atr": candidate["speed5_atr"],
            "pre_eff15": candidate["eff15"],
            "state_key": candidate["state_key"],
            "state_hit_rate": candidate["state_hit_rate"],
            "state_samples": candidate["state_samples"],
            "touch_time": position.get("touch_time", pd.NaT),
            "confirm_level": position.get("confirm_level", np.nan),
            "entry_extension_atr": position.get("entry_extension_atr", np.nan),
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
            "touch_time": position.get("touch_time", pd.NaT),
            "confirm_level": position.get("confirm_level", np.nan),
            "entry_extension_atr": position.get("entry_extension_atr", np.nan),
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
    entry_pos, entry_raw, resolve_skip, resolve_meta = resolve_posttouch_entry(second_bars, start_pos, candidate, params)
    if resolve_skip:
        return None, balance, resolve_skip, resolve_meta

    entry_time = idx[int(entry_pos)]
    sig = pre.fake_signal(row, entry_raw, candidate)
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
        return None, balance, "min_stop", resolve_meta
    position.update(
        {
            "breakout_level": candidate["level"],
            "pre_distance_atr": candidate["distance_atr"],
            "state_hit_rate": candidate["state_hit_rate"],
            "state_key": candidate["state_key"],
            "touch_time": resolve_meta.get("touch_time", pd.NaT),
            "confirm_level": resolve_meta.get("confirm_level", np.nan),
            "entry_extension_atr": resolve_meta.get("extension_atr", np.nan),
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
            return logs, balance, "", resolve_meta

        event = events_by_end.get(bar_time)
        if event is not None and bar_time > entry_time:
            exit_now, event_reason = micro.apply_signal_event(position, event, params)
            if exit_now:
                balance = append_exit(logs, position, raw_exit=close_value, time_value=bar_time, reason=event_reason, balance=balance, params=params)
                return logs, balance, "", resolve_meta

        if max_hold_end is not None and bar_time >= max_hold_end:
            balance = append_exit(logs, position, raw_exit=close_value, time_value=bar_time, reason="MaxHoldExit", balance=balance, params=params)
            return logs, balance, "", resolve_meta

    balance = append_exit(logs, position, raw_exit=float(closes[end_pos]), time_value=idx[end_pos], reason="FinalMarkToMarket", balance=balance, params=params)
    return logs, balance, "", resolve_meta


def setup_key(candidate: dict, row: pd.Series) -> tuple:
    hour_start = pd.Timestamp(row["hour_start"])
    level_key = round(float(candidate["level"]), 8)
    return hour_start, str(candidate["side"]), level_key


def run_strategy(second_bars: pd.DataFrame, minute: pd.DataFrame, signal: pd.DataFrame, params: dict, state_table: dict, *, initial_balance: float) -> dict:
    idx = second_bars.index
    highs_1m = minute["high"].to_numpy(dtype="float64", copy=False)
    lows_1m = minute["low"].to_numpy(dtype="float64", copy=False)
    events_by_end = {pd.Timestamp(row["bar_end"]): row for _, row in signal.iterrows()}
    balance = float(initial_balance)
    logs: list[dict] = []
    last_exit_time = pd.Timestamp.min.tz_localize("UTC")
    consumed: set[tuple] = set()
    diagnostics = {
        "armed_setups": 0,
        "entries": 0,
        "busy_skipped": 0,
        "dedupe_skipped": 0,
        "pre_fail_skipped": 0,
        "pre_timeout_skipped": 0,
        "post_touch_fail_skipped": 0,
        "confirm_timeout_skipped": 0,
        "entry_extension_skipped": 0,
        "min_stop_skipped": 0,
        "state_filtered": 0,
        "touched_without_entry": 0,
        "quality_counts": {"base": 0, "strong": 0},
    }
    for pos, row in minute.iterrows():
        candidate = pre.candidate_from_minute(row, float(highs_1m[int(pos)]), float(lows_1m[int(pos)]), params, state_table)
        if candidate is None:
            diagnostics["state_filtered"] += 1
            continue
        key = setup_key(candidate, row)
        if key in consumed:
            diagnostics["dedupe_skipped"] += 1
            continue
        consumed.add(key)
        diagnostics["armed_setups"] += 1

        entry_time = pd.Timestamp(row["minute_start"]) + pd.Timedelta(minutes=1)
        if entry_time <= last_exit_time:
            diagnostics["busy_skipped"] += 1
            continue
        start_pos = int(idx.searchsorted(entry_time, side="left"))
        trade_logs, new_balance, skip_reason, resolve_meta = simulate_position(
            second_bars,
            events_by_end,
            row,
            candidate,
            params,
            start_pos=start_pos,
            balance=balance,
        )
        if resolve_meta.get("touch_time") is not None:
            diagnostics["touched_without_entry"] += 1
        if skip_reason == "pre_fail":
            diagnostics["pre_fail_skipped"] += 1
            continue
        if skip_reason == "pre_timeout":
            diagnostics["pre_timeout_skipped"] += 1
            continue
        if skip_reason == "post_touch_fail":
            diagnostics["post_touch_fail_skipped"] += 1
            continue
        if skip_reason == "confirm_timeout":
            diagnostics["confirm_timeout_skipped"] += 1
            continue
        if skip_reason == "entry_extension":
            diagnostics["entry_extension_skipped"] += 1
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
        f"# ETH 突破前 Watch + Touch 后确认 1s 回测（{summary['start']} 至 {summary['end']}）",
        "",
        "范围：仅限 `research`。突破前概率表只负责为每个 `1h` level arm 一个 setup。进场必须先等价格触碰 breakout level，然后再要求 `1s close` 穿过确认距离。执行和退出使用连续 `1s` bar 与结构移动止损。",
        "",
        "| Variant | 笔数 | Realistic | Raw | 2bps Slip No Fee | 胜率 | Max DD | Avg Share | Med Hold | Med MFE ATR | Exits | Quality | Armed | Entries | Busy | Pre Fail | Pre Timeout | Post Fail | Confirm Timeout | Dedupe |",
        "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---|---:|---:|---:|---:|---:|---:|---:|---:|",
    ]
    for result in summary["results"]:
        s = result["summary"]
        d = result["diagnostics"]
        lines.append(
            f"| `{result['variant']}` | {s['trades']} | {s['realistic_return_pct']:.4f}% | "
            f"{s['raw_no_fee_no_slip_return_pct']:.4f}% | {s['price_pnl_with_2bps_slip_no_fee_return_pct']:.4f}% | "
            f"{s['win_rate_pct']:.2f}% | {s['max_dd_pct']:.4f}% | {s.get('avg_notional_share', 0.0):.4f} | "
            f"{s.get('median_hold_seconds', 0.0):.2f}s | {s.get('median_mfe_atr', 0.0):.4f} | "
            f"`{s['exit_reasons']}` | `{s.get('quality_trades', {})}` | {d['armed_setups']} | "
            f"{d['entries']} | {d['busy_skipped']} | {d['pre_fail_skipped']} | {d['pre_timeout_skipped']} | "
            f"{d['post_touch_fail_skipped']} | {d['confirm_timeout_skipped']} | {d['dedupe_skipped']} |"
        )
    lines.extend(["", "## 文件", ""])
    lines.append(f"- Summary JSON：`{summary['summary_path']}`")
    for result in summary["results"]:
        lines.append(f"- `{result['variant']}` ledger：`{result['ledger_path']}`")
    lines.append("")
    path.write_text("\n".join(lines), encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="ETH pre-breakout watch plus post-touch confirmation replay")
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
            "pt_d010_p70_c002=max_dist:0.10,prob:70,strong_prob:75,samples:80,confirm:0.02,post_fail:0.08",
            "pt_d010_p70_c005=max_dist:0.10,prob:70,strong_prob:75,samples:80,confirm:0.05,post_fail:0.08",
            "pt_d010_p75_c005=max_dist:0.10,prob:75,strong_prob:78,samples:80,confirm:0.05,post_fail:0.08",
            "pt_d020_p65_c005=max_dist:0.20,prob:65,strong_prob:75,samples:80,confirm:0.05,post_fail:0.10",
        ],
    )
    parser.add_argument("--summary-json", default="research/eth_prebreakout_posttouch_confirm_replay_summary.json")
    parser.add_argument("--markdown", default="research/20260507_eth_prebreakout_posttouch_confirm_replay.md")
    parser.add_argument("--ledger-prefix", default="research/tmp_eth_prebreakout_posttouch_confirm_replay")
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
    state_table = pre.load_state_table(Path(args.train_summary_json))
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
        "execution": "continuous 1s OHLC bars from local trade ticks",
        "accounting": "2bps/side slippage plus maker entry 2bps and market exit 4bps realistic accounting",
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
