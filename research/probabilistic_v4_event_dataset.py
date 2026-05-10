#!/usr/bin/env python3
"""Probabilistic V4 post-touch event dataset builder.

Research-only. This script does not place trades. It converts true intrabar
breakout touches into a flat event table with point-in-time features and
post-touch outcomes. Downstream scripts can train quality filters and run
execution tests without rebuilding signal semantics inside the model.
"""

from __future__ import annotations

import argparse
import json
import time
from pathlib import Path

import numpy as np
import pandas as pd

import eth_q1_breakout_t3_shape_compare as replay

try:
    import btc_eth_2026_jan_apr_impulse_bar_run as base
except ModuleNotFoundError:
    import probabilistic_v4_execution_runner as execution

    base = execution.base


DEFAULT_ARCHIVE_ROOT = Path("dataset/archive")


def _cache_path(symbol: str, start: pd.Timestamp, end: pd.Timestamp, cache_dir: str) -> Path | None:
    if not cache_dir:
        return None
    start_key = start.strftime("%Y%m%dT%H%M%S")
    end_key = end.strftime("%Y%m%dT%H%M%S")
    return Path(cache_dir) / f"{symbol}_{start_key}_{end_key}_flow_1s.pkl"


def _load_or_build_second_bars(symbol: str, start: pd.Timestamp, end: pd.Timestamp, args: argparse.Namespace):
    path = _cache_path(symbol, start, end, str(getattr(args, "bars_cache_dir", "")))
    if path is not None and path.exists():
        sb = pd.read_pickle(path)
        stats = {
            "cache_path": str(path),
            "cache_hit": True,
            "second_rows": int(len(sb)),
        }
        return sb, stats

    tick_files = base.monthly_trade_files(symbol, start, end, Path(args.archive_root))
    import order_flow_imbalance_breakout as flow

    sb, stats = flow.build_second_bars_with_flow(tick_files, start, end, int(args.chunksize))
    if path is not None:
        path.parent.mkdir(parents=True, exist_ok=True)
        sb.to_pickle(path)
        stats = {**stats, "cache_path": str(path), "cache_hit": False}
    return sb, stats


def _safe_ratio(numerator: float, denominator: float) -> float:
    if denominator <= 0 or not np.isfinite(denominator):
        return np.nan
    return float(numerator) / float(denominator)


def _direction_mult(side: str) -> float:
    return 1.0 if side == "long" else -1.0


def _finite_or_nan(value: object) -> float:
    try:
        numeric = float(value)
    except (TypeError, ValueError):
        return np.nan
    return numeric if np.isfinite(numeric) else np.nan


def _round_or_nan(value: object, digits: int = 6) -> float:
    numeric = _finite_or_nan(value)
    return round(numeric, digits) if np.isfinite(numeric) else np.nan


def _aligned_flow_ratio(sb: pd.DataFrame, pos: int, side: str, seconds: int) -> float:
    start = max(0, pos - int(seconds) + 1)
    buy_volume = float(sb["buy_volume"].iloc[start : pos + 1].sum())
    sell_volume = float(sb["sell_volume"].iloc[start : pos + 1].sum())
    total = buy_volume + sell_volume
    if side == "long":
        return _safe_ratio(buy_volume, total)
    return _safe_ratio(sell_volume, total)


def _volume_ratio(sb: pd.DataFrame, pos: int, seconds: int, reference_seconds: int = 300) -> float:
    start = max(0, pos - int(seconds) + 1)
    ref_start = max(0, pos - int(reference_seconds) + 1)
    short_volume = float(sb["volume"].iloc[start : pos + 1].sum())
    ref = sb["volume"].iloc[ref_start : pos + 1].replace(0, np.nan)
    if ref.empty:
        return np.nan
    ref_per_second = float(ref.median())
    if ref_per_second <= 0 or not np.isfinite(ref_per_second):
        return np.nan
    return short_volume / (ref_per_second * max(1, int(seconds)))


def _direction_speed(close_values: np.ndarray, pos: int, side: str, atr: float, seconds: int) -> float:
    lag = max(0, pos - int(seconds))
    if atr <= 0 or not np.isfinite(atr):
        return np.nan
    return _direction_mult(side) * (float(close_values[pos]) - float(close_values[lag])) / float(atr)


def _efficiency(
    high_values: np.ndarray,
    low_values: np.ndarray,
    close_values: np.ndarray,
    pos: int,
    side: str,
    seconds: int,
) -> float:
    start = max(0, pos - int(seconds))
    span_high = float(np.nanmax(high_values[start : pos + 1]))
    span_low = float(np.nanmin(low_values[start : pos + 1]))
    span = span_high - span_low
    if span <= 0 or not np.isfinite(span):
        return np.nan
    return _direction_mult(side) * (float(close_values[pos]) - float(close_values[start])) / span


def _close_pos(
    high_values: np.ndarray,
    low_values: np.ndarray,
    close_values: np.ndarray,
    pos: int,
    side: str,
    seconds: int,
) -> float:
    start = max(0, pos - int(seconds))
    span_high = float(np.nanmax(high_values[start : pos + 1]))
    span_low = float(np.nanmin(low_values[start : pos + 1]))
    span = span_high - span_low
    if span <= 0 or not np.isfinite(span):
        return np.nan
    raw = (float(close_values[pos]) - span_low) / span
    return raw if side == "long" else 1.0 - raw


def _order_flow_states(sb: pd.DataFrame) -> np.ndarray:
    volume = sb["volume"].astype("float64")
    median_volume = volume.replace(0, np.nan).rolling(300, min_periods=30).median().ffill().fillna(0.0)
    is_buy = sb["buy_volume"].to_numpy(dtype="float64", copy=False) > sb["sell_volume"].to_numpy(
        dtype="float64", copy=False
    )
    is_strong = volume.to_numpy(dtype="float64", copy=False) > median_volume.to_numpy(dtype="float64", copy=False)
    states = np.zeros(len(sb), dtype=np.int8)
    states[(~is_buy) & is_strong] = 1
    states[is_buy & (~is_strong)] = 2
    states[is_buy & is_strong] = 3
    return states


def _state_sequence(states: np.ndarray, pos: int, side: str, seconds: int) -> str:
    start = max(0, pos - int(seconds) + 1)
    seq = states[start : pos + 1]
    if side == "short":
        seq = 3 - seq
    return "".join(str(int(v)) for v in seq)


def _post_touch_pullback(
    high_values: np.ndarray,
    low_values: np.ndarray,
    pos: int,
    side: str,
    level: float,
    atr: float,
    seconds: int,
) -> float:
    end = min(pos + int(seconds), len(high_values) - 1)
    if end <= pos or atr <= 0:
        return 0.0
    if side == "long":
        adverse = max(0.0, float(level) - float(np.nanmin(low_values[pos : end + 1])))
    else:
        adverse = max(0.0, float(np.nanmax(high_values[pos : end + 1])) - float(level))
    return adverse / float(atr)


def _dwell_feature(
    close_values: np.ndarray,
    pos: int,
    side: str,
    level: float,
    seconds: int,
) -> tuple[bool, float]:
    end = min(pos + int(seconds), len(close_values) - 1)
    if end <= pos:
        return False, np.nan
    closes = close_values[pos : end + 1]
    if side == "long":
        ok = bool(np.nanmin(closes) >= float(level))
    else:
        ok = bool(np.nanmax(closes) <= float(level))
    return ok, float(close_values[end])


def _evaluate_outcome(
    high_values: np.ndarray,
    low_values: np.ndarray,
    close_values: np.ndarray,
    pos: int,
    side: str,
    level: float,
    atr: float,
    *,
    continuation_atr: float,
    fail_atr: float,
    horizon_seconds: int,
) -> dict:
    end = min(pos + int(horizon_seconds), len(close_values) - 1)
    if end <= pos or atr <= 0:
        return {
            "outcome": "timeout",
            "seconds_to_outcome": 0,
            "max_favorable_atr": 0.0,
            "max_adverse_atr": 0.0,
            "horizon_edge_atr": 0.0,
            "first_edge_atr": 0.0,
        }

    cont_level = float(level) + _direction_mult(side) * float(continuation_atr) * float(atr)
    fail_level = float(level) - _direction_mult(side) * float(fail_atr) * float(atr)
    max_favorable = 0.0
    max_adverse = 0.0
    outcome = "timeout"
    seconds_to_outcome = int(horizon_seconds)

    for future_pos in range(pos + 1, end + 1):
        if side == "long":
            max_favorable = max(max_favorable, float(high_values[future_pos]) - float(level))
            max_adverse = max(max_adverse, float(level) - float(low_values[future_pos]))
            failed = float(low_values[future_pos]) <= fail_level
            continued = float(high_values[future_pos]) >= cont_level
        else:
            max_favorable = max(max_favorable, float(level) - float(low_values[future_pos]))
            max_adverse = max(max_adverse, float(high_values[future_pos]) - float(level))
            failed = float(high_values[future_pos]) >= fail_level
            continued = float(low_values[future_pos]) <= cont_level

        if failed:
            outcome = "fail"
            seconds_to_outcome = future_pos - pos
            break
        if continued:
            outcome = "continuation"
            seconds_to_outcome = future_pos - pos
            break

    horizon_edge = _direction_mult(side) * (float(close_values[end]) - float(close_values[pos])) / float(atr)
    if outcome == "continuation":
        first_edge = float(continuation_atr)
    elif outcome == "fail":
        first_edge = -float(fail_atr)
    else:
        first_edge = horizon_edge

    return {
        "outcome": outcome,
        "seconds_to_outcome": int(seconds_to_outcome),
        "max_favorable_atr": round(max_favorable / float(atr), 6),
        "max_adverse_atr": round(max_adverse / float(atr), 6),
        "horizon_edge_atr": round(horizon_edge, 6),
        "first_edge_atr": round(first_edge, 6),
    }


def _record_event(
    *,
    symbol: str,
    signal_start: pd.Timestamp,
    signal_end: pd.Timestamp,
    sig: pd.Series,
    side: str,
    shape: str,
    level: float,
    touch_pos: int,
    touch_high_so_far: float,
    touch_low_so_far: float,
    sb: pd.DataFrame,
    states: np.ndarray,
    high_values: np.ndarray,
    low_values: np.ndarray,
    close_values: np.ndarray,
    args: argparse.Namespace,
) -> dict:
    touch_time = sb.index[touch_pos]
    atr = _finite_or_nan(sig.get("prev_atr_1", np.nan))
    atr_source = "prev_atr_1"
    if not np.isfinite(atr) or atr <= 0:
        atr = float(sig["atr"])
        atr_source = "signal_atr"
    touch_close = float(close_values[touch_pos])
    cost_rate = (2.0 * float(args.slippage_bps) + float(args.entry_fee_bps) + float(args.exit_fee_bps)) / 10000.0
    side_mult = _direction_mult(side)
    prev_open_1 = _finite_or_nan(sig.get("prev_open_1", np.nan))
    prev_high_1 = _finite_or_nan(sig.get("prev_high_1", np.nan))
    prev_low_1 = _finite_or_nan(sig.get("prev_low_1", np.nan))
    prev_close_1 = _finite_or_nan(sig.get("prev_close_1", np.nan))
    prev_range_1 = prev_high_1 - prev_low_1 if np.isfinite(prev_high_1) and np.isfinite(prev_low_1) else np.nan
    prev_close_pos_1 = (prev_close_1 - prev_low_1) / prev_range_1 if np.isfinite(prev_range_1) and prev_range_1 > 0 else np.nan
    prev_close_pos_1_side = prev_close_pos_1 if side == "long" else 1.0 - prev_close_pos_1
    prev_sma5_1 = _finite_or_nan(sig.get("prev_sma5_1", np.nan))
    prev_sma5_2 = _finite_or_nan(sig.get("prev_sma5_2", np.nan))
    flow_ratio_5s = _aligned_flow_ratio(sb, touch_pos, side, 5)
    flow_ratio_15s = _aligned_flow_ratio(sb, touch_pos, side, 15)
    flow_ratio_30s = _aligned_flow_ratio(sb, touch_pos, side, 30)
    flow_ratio_60s = _aligned_flow_ratio(sb, touch_pos, side, 60)
    flow_ratio_120s = _aligned_flow_ratio(sb, touch_pos, side, 120)
    speed_5s_atr = _direction_speed(close_values, touch_pos, side, atr, 5)
    speed_15s_atr = _direction_speed(close_values, touch_pos, side, atr, 15)
    speed_60s_atr = _direction_speed(close_values, touch_pos, side, atr, 60)
    speed_300s_atr = _direction_speed(close_values, touch_pos, side, atr, 300)
    row = {
        "event_id": f"{symbol}|{signal_start.isoformat()}|{side}|{shape}",
        "symbol": symbol,
        "signal_start": signal_start.isoformat(),
        "signal_end": signal_end.isoformat(),
        "touch_time": touch_time.isoformat(),
        "side": side,
        "shape": shape,
        "level": round(float(level), 8),
        "touch_close": round(touch_close, 8),
        "touch_extension_atr": round(abs(touch_close - float(level)) / atr, 6),
        "atr": round(atr, 8),
        "atr_source": atr_source,
        "signal_open": round(float(sig["open"]), 8),
        "signal_high": round(float(sig["high"]), 8),
        "signal_low": round(float(sig["low"]), 8),
        "signal_close": round(float(sig["close"]), 8),
        "touch_high_so_far": round(float(touch_high_so_far), 8),
        "touch_low_so_far": round(float(touch_low_so_far), 8),
        "signal_atr_percentile": _round_or_nan(sig.get("prev_atr_percentile_1", sig.get("atr_percentile", np.nan))),
        "prev1_body_atr": _round_or_nan((prev_close_1 - prev_open_1) * side_mult / atr),
        "prev1_range_atr": _round_or_nan(prev_range_1 / atr),
        "prev1_close_pos_side": _round_or_nan(prev_close_pos_1_side),
        "prev_sma5_gap_atr": _round_or_nan((float(sig["open"]) - prev_sma5_1) * side_mult / atr),
        "prev_sma5_slope_atr": _round_or_nan((prev_sma5_1 - prev_sma5_2) * side_mult / atr),
        "level_to_prev_close_atr": _round_or_nan((prev_close_1 - float(level)) * side_mult / atr),
        "level_to_signal_open_atr": _round_or_nan((float(sig["open"]) - float(level)) * side_mult / atr),
        "roundtrip_cost_atr": round(touch_close * cost_rate / atr, 6),
        "flow_ratio_5s": round(flow_ratio_5s, 6),
        "flow_ratio_15s": round(flow_ratio_15s, 6),
        "flow_ratio_30s": round(flow_ratio_30s, 6),
        "flow_ratio_60s": round(flow_ratio_60s, 6),
        "flow_ratio_120s": round(flow_ratio_120s, 6),
        "flow_delta_5_60s": _round_or_nan(flow_ratio_5s - flow_ratio_60s),
        "flow_delta_15_60s": _round_or_nan(flow_ratio_15s - flow_ratio_60s),
        "flow_delta_30_120s": _round_or_nan(flow_ratio_30s - flow_ratio_120s),
        "volume_ratio_5s": _round_or_nan(_volume_ratio(sb, touch_pos, 5)),
        "volume_ratio_15s": _round_or_nan(_volume_ratio(sb, touch_pos, 15)),
        "volume_ratio_60s": _round_or_nan(_volume_ratio(sb, touch_pos, 60)),
        "speed_5s_atr": round(speed_5s_atr, 6),
        "speed_15s_atr": round(speed_15s_atr, 6),
        "speed_60s_atr": round(speed_60s_atr, 6),
        "speed_300s_atr": round(speed_300s_atr, 6),
        "speed_decay_5_60s_atr": _round_or_nan(speed_5s_atr - speed_60s_atr),
        "speed_decay_15_60s_atr": _round_or_nan(speed_15s_atr - speed_60s_atr),
        "speed_decay_60_300s_atr": _round_or_nan(speed_60s_atr - speed_300s_atr),
        "eff_15s": round(_efficiency(high_values, low_values, close_values, touch_pos, side, 15), 6),
        "eff_60s": round(_efficiency(high_values, low_values, close_values, touch_pos, side, 60), 6),
        "eff_300s": round(_efficiency(high_values, low_values, close_values, touch_pos, side, 300), 6),
        "close_pos_15s": round(_close_pos(high_values, low_values, close_values, touch_pos, side, 15), 6),
        "close_pos_60s": round(_close_pos(high_values, low_values, close_values, touch_pos, side, 60), 6),
        "close_pos_300s": round(_close_pos(high_values, low_values, close_values, touch_pos, side, 300), 6),
        "state_seq_60s": _state_sequence(states, touch_pos, side, 60),
    }
    for seconds in args.dwell_seconds:
        ok, entry_close = _dwell_feature(close_values, touch_pos, side, float(level), int(seconds))
        row[f"dwell_{int(seconds)}s_pass"] = bool(ok)
        row[f"dwell_{int(seconds)}s_close"] = round(entry_close, 8) if np.isfinite(entry_close) else np.nan
    for seconds in args.pullback_seconds:
        row[f"pullback_{int(seconds)}s_atr"] = round(
            _post_touch_pullback(high_values, low_values, touch_pos, side, float(level), atr, int(seconds)), 6
        )
    row.update(
        _evaluate_outcome(
            high_values,
            low_values,
            close_values,
            touch_pos,
            side,
            float(level),
            atr,
            continuation_atr=float(args.continuation_atr),
            fail_atr=float(args.fail_atr),
            horizon_seconds=int(args.horizon_seconds),
        )
    )
    row["net_first_edge_atr"] = round(float(row["first_edge_atr"]) - float(row["roundtrip_cost_atr"]), 6)
    return row


def build_events_for_symbol(symbol: str, args: argparse.Namespace) -> tuple[pd.DataFrame, dict]:
    start = base._as_utc(args.start)
    end = base._as_utc(args.end)
    sb, build_stats = _load_or_build_second_bars(symbol, start, end, args)
    plain_second_bars = sb[["open", "high", "low", "close", "volume"]].copy()
    _, signal = replay.build_signal_frame(plain_second_bars, args.signal_timeframe)
    signal["prev_open_1"] = signal["open"].shift(1)
    signal["prev_atr_1"] = signal["atr"].shift(1)
    signal["prev_atr_percentile_1"] = signal["atr_percentile"].shift(1)
    signal["prev_sma5_2"] = signal["sma5"].shift(2)
    states = _order_flow_states(sb)

    high_values = sb["high"].to_numpy(dtype="float64", copy=False)
    low_values = sb["low"].to_numpy(dtype="float64", copy=False)
    close_values = sb["close"].to_numpy(dtype="float64", copy=False)
    second_index = sb.index
    events: list[dict] = []

    for i in range(len(signal) - 1):
        signal_start = pd.Timestamp(signal.index[i])
        signal_end = pd.Timestamp(signal.index[i + 1])
        sig = signal.iloc[i]
        if pd.isna(sig.get("atr", np.nan)) or float(sig["atr"]) <= 0:
            continue
        start_pos = int(second_index.searchsorted(signal_start, side="left"))
        end_pos = int(second_index.searchsorted(signal_end, side="left"))
        if start_pos >= end_pos:
            continue
        seen: set[tuple[str, str]] = set()
        high_so_far = -np.inf
        low_so_far = np.inf
        for pos in range(start_pos, end_pos):
            high_so_far = max(high_so_far, float(high_values[pos]))
            low_so_far = min(low_so_far, float(low_values[pos]))

            long_hit, long_level, long_shape = replay._long_breakout(sig, high_so_far, args.breakout_shape)
            if long_hit and ("long", long_shape) not in seen:
                seen.add(("long", long_shape))
                events.append(
                    _record_event(
                        symbol=symbol,
                        signal_start=signal_start,
                        signal_end=signal_end,
                        sig=sig,
                        side="long",
                        shape=long_shape,
                        level=float(long_level),
                        touch_pos=pos,
                        touch_high_so_far=high_so_far,
                        touch_low_so_far=low_so_far,
                        sb=sb,
                        states=states,
                        high_values=high_values,
                        low_values=low_values,
                        close_values=close_values,
                        args=args,
                    )
                )

            short_hit, short_level, short_shape = replay._short_breakout(sig, low_so_far, args.breakout_shape)
            if short_hit and ("short", short_shape) not in seen:
                seen.add(("short", short_shape))
                events.append(
                    _record_event(
                        symbol=symbol,
                        signal_start=signal_start,
                        signal_end=signal_end,
                        sig=sig,
                        side="short",
                        shape=short_shape,
                        level=float(short_level),
                        touch_pos=pos,
                        touch_high_so_far=high_so_far,
                        touch_low_so_far=low_so_far,
                        sb=sb,
                        states=states,
                        high_values=high_values,
                        low_values=low_values,
                        close_values=close_values,
                        args=args,
                    )
                )

    return pd.DataFrame(events), {"build_stats": build_stats, "signal_rows": int(len(signal))}


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Build Probabilistic V4 post-touch event dataset")
    parser.add_argument("--symbols", nargs="+", default=["BTCUSDT", "ETHUSDT"])
    parser.add_argument("--start", default="2026-01-01T00:00:00Z")
    parser.add_argument("--end", default="2026-04-30T23:59:59Z")
    parser.add_argument("--archive-root", default=str(DEFAULT_ARCHIVE_ROOT))
    parser.add_argument("--signal-timeframe", default="1h")
    parser.add_argument("--breakout-shape", choices=["original_t2", "baseline_plus_t3"], default="baseline_plus_t3")
    parser.add_argument("--continuation-atr", type=float, default=0.5)
    parser.add_argument("--fail-atr", type=float, default=0.2)
    parser.add_argument("--horizon-seconds", type=int, default=7200)
    parser.add_argument("--dwell-seconds", nargs="+", type=int, default=[5, 15, 30])
    parser.add_argument("--pullback-seconds", nargs="+", type=int, default=[5, 15, 30, 60])
    parser.add_argument("--slippage-bps", type=float, default=2.0)
    parser.add_argument("--entry-fee-bps", type=float, default=2.0)
    parser.add_argument("--exit-fee-bps", type=float, default=4.0)
    parser.add_argument("--chunksize", type=int, default=2_000_000)
    parser.add_argument("--bars-cache-dir", default="")
    parser.add_argument("--output-csv", default="research/probabilistic_v4_events.csv")
    parser.add_argument("--summary-json", default="research/probabilistic_v4_events_summary.json")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    started = time.time()
    all_events: list[pd.DataFrame] = []
    per_symbol: dict[str, dict] = {}
    for symbol in args.symbols:
        print(f"\n{symbol}: building probabilistic V4 events", flush=True)
        events, stats = build_events_for_symbol(symbol, args)
        print(f"{symbol}: events={len(events)} outcomes={events['outcome'].value_counts().to_dict() if not events.empty else {}}", flush=True)
        all_events.append(events)
        per_symbol[symbol] = {
            **stats,
            "events": int(len(events)),
            "outcomes": {str(k): int(v) for k, v in events["outcome"].value_counts().items()} if not events.empty else {},
        }

    dataset = pd.concat(all_events, ignore_index=True) if all_events else pd.DataFrame()
    output_path = Path(args.output_csv)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    dataset.to_csv(output_path, index=False)
    summary = {
        "start": args.start,
        "end": args.end,
        "symbols": args.symbols,
        "signal_timeframe": args.signal_timeframe,
        "breakout_shape": args.breakout_shape,
        "continuation_atr": args.continuation_atr,
        "fail_atr": args.fail_atr,
        "horizon_seconds": args.horizon_seconds,
        "event_csv": str(output_path),
        "rows": int(len(dataset)),
        "per_symbol": per_symbol,
        "elapsed_seconds": round(time.time() - started, 2),
    }
    Path(args.summary_json).write_text(json.dumps(summary, indent=2, ensure_ascii=False), encoding="utf-8")
    print(json.dumps({"event_csv": str(output_path), "rows": len(dataset), "summary_json": args.summary_json}, indent=2), flush=True)


if __name__ == "__main__":
    main()
