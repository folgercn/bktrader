#!/usr/bin/env python3
"""Impulse bar + delayed confirm V2 research replay.

Research-only. Extends the confirm_sweep framework with additional confirmation
dimensions: VWAP slope, volume acceleration, intrabar close position, and
post-confirm retrace rejection.

Signals are closed 1h bars; execution uses continuous 1s OHLC bars built from
local Binance trade ticks.
"""

from __future__ import annotations

import argparse
import json
import time
from pathlib import Path

import numpy as np
import pandas as pd

import btc_eth_2026_jan_apr_impulse_bar_run as base
import eth_q1_breakout_t3_shape_compare as replay


DEFAULT_ARCHIVE_ROOT = Path("dataset/archive")


def _maybe_number(value: str):
    lowered = value.strip().lower()
    if lowered in {"true", "false"}:
        return 1.0 if lowered == "true" else 0.0
    try:
        return float(value)
    except ValueError:
        return value.strip()


# ---------------------------------------------------------------------------
# Variant parsing
# ---------------------------------------------------------------------------

DEFAULT_PARAMS = {
    "break_lookback": 8,
    "body_min": 0.65,
    "close_top": 0.75,
    "range_min_atr": 0.80,
    "range_max_atr": 2.20,
    "pre_range_atr": 3.00,
    "max_atr_percentile": 95.0,
    "max_entry_extension_atr": 0.15,
    # confirm v2 params
    "confirm_atr": 0.03,
    "confirm_window_seconds": 120,
    "fail_retrace_atr": 0.10,
    # new v2 dimensions
    "vwap_confirm": 1.0,          # 1=enabled, 0=disabled
    "volume_acceleration": 1.0,   # 1=enabled, 0=disabled
    "vol_accel_ratio": 1.2,       # volume must be >= prev window * this
    "vwap_window_seconds": 300,   # vwap slope window
    "intrabar_close_gate": 0.0,   # 0=disabled; > 0 = min close_pos for long
    "post_confirm_retrace": 0.0,  # 0=disabled; > 0 = max retrace ratio after confirm
    "post_confirm_watch_seconds": 30,
    # exit
    "initial_stop_atr": 0.45,
    "stop_buffer_atr": 0.05,
    "stop_cap_atr": 0.80,
    "min_stop_bps": 12.0,
    "breakeven_at_r": 1.0,
    "cost_lock_bps": 10.0,
    "trail_start_r": 1.5,
    "trail_buffer_atr": 0.05,
    "max_hold_bars": 6,
    "notional_share": 0.20,
    "slippage": 0.0002,
    "entry_fee": 0.0002,
    "exit_fee": 0.0004,
}

PRESETS = {
    "c03_f10": {"confirm_atr": 0.03, "fail_retrace_atr": 0.10, "vwap_confirm": 0.0, "volume_acceleration": 0.0},
    "c03_f10_vwap": {"confirm_atr": 0.03, "fail_retrace_atr": 0.10, "vwap_confirm": 1.0, "volume_acceleration": 0.0},
    "c03_f10_vol": {"confirm_atr": 0.03, "fail_retrace_atr": 0.10, "vwap_confirm": 0.0, "volume_acceleration": 1.0},
    "c03_f10_full": {"confirm_atr": 0.03, "fail_retrace_atr": 0.10, "vwap_confirm": 1.0, "volume_acceleration": 1.0},
    "c05_f15": {"confirm_atr": 0.05, "fail_retrace_atr": 0.15, "vwap_confirm": 0.0, "volume_acceleration": 0.0},
    "c05_f15_vwap": {"confirm_atr": 0.05, "fail_retrace_atr": 0.15, "vwap_confirm": 1.0, "volume_acceleration": 0.0},
    "c05_f15_vol": {"confirm_atr": 0.05, "fail_retrace_atr": 0.15, "vwap_confirm": 0.0, "volume_acceleration": 1.0},
    "c05_f15_full": {"confirm_atr": 0.05, "fail_retrace_atr": 0.15, "vwap_confirm": 1.0, "volume_acceleration": 1.0},
    "c08_f20": {"confirm_atr": 0.08, "fail_retrace_atr": 0.20, "vwap_confirm": 0.0, "volume_acceleration": 0.0},
    "c08_f20_full": {"confirm_atr": 0.08, "fail_retrace_atr": 0.20, "vwap_confirm": 1.0, "volume_acceleration": 1.0},
    "c03_f10_closepos": {"confirm_atr": 0.03, "fail_retrace_atr": 0.10, "vwap_confirm": 0.0, "volume_acceleration": 0.0, "intrabar_close_gate": 0.6},
    "c05_f15_closepos_full": {"confirm_atr": 0.05, "fail_retrace_atr": 0.15, "vwap_confirm": 1.0, "volume_acceleration": 1.0, "intrabar_close_gate": 0.6},
    "c03_f10_retrace": {"confirm_atr": 0.03, "fail_retrace_atr": 0.10, "vwap_confirm": 0.0, "volume_acceleration": 0.0, "post_confirm_retrace": 0.5, "post_confirm_watch_seconds": 30},
    "c03_f10_all": {"confirm_atr": 0.03, "fail_retrace_atr": 0.10, "vwap_confirm": 1.0, "volume_acceleration": 1.0, "intrabar_close_gate": 0.6, "post_confirm_retrace": 0.5},
}

KEY_MAP = {
    "break": "break_lookback", "body": "body_min", "confirm": "confirm_atr",
    "window": "confirm_window_seconds", "fail": "fail_retrace_atr",
    "vwap": "vwap_confirm", "volaccel": "volume_acceleration",
    "volratio": "vol_accel_ratio", "closepos": "intrabar_close_gate",
    "retrace": "post_confirm_retrace", "retracewin": "post_confirm_watch_seconds",
    "hold": "max_hold_bars", "trail_start": "trail_start_r",
}


def parse_variant(raw: str) -> tuple[str, dict]:
    params = dict(DEFAULT_PARAMS)
    if raw in PRESETS:
        params.update(PRESETS[raw])
        return raw, params
    name, values = raw.split("=", 1) if "=" in raw else (raw, "")
    if name in PRESETS:
        params.update(PRESETS[name])
    for item in values.split(","):
        if not item.strip():
            continue
        key, value = item.split(":", 1)
        mapped = KEY_MAP.get(key.strip())
        if mapped is None:
            if key.strip() in params:
                mapped = key.strip()
            else:
                raise ValueError(f"unknown variant key {key} in {raw}")
        params[mapped] = _maybe_number(value)
    for int_key in ("break_lookback", "max_hold_bars", "confirm_window_seconds", "post_confirm_watch_seconds"):
        params[int_key] = int(params[int_key])
    return name if name else raw, params


# ---------------------------------------------------------------------------
# V2 confirm logic with VWAP / volume / close_pos / retrace gates
# ---------------------------------------------------------------------------

def _compute_vwap_slope(
    second_bars: pd.DataFrame,
    start_pos: int,
    end_pos: int,
    side: str,
    window_seconds: int,
) -> bool:
    """Check if trailing VWAP slope is directionally consistent."""
    if end_pos - start_pos < window_seconds:
        # not enough bars for slope check
        return True  # permissive fallback
    window_start = max(start_pos, end_pos - window_seconds)
    closes = second_bars["close"].iloc[window_start:end_pos + 1].values.astype("float64")
    volumes = second_bars["volume"].iloc[window_start:end_pos + 1].values.astype("float64")
    cum_vol = np.cumsum(volumes)
    cum_pv = np.cumsum(closes * volumes)
    if cum_vol[-1] <= 0:
        return True
    vwap_end = cum_pv[-1] / cum_vol[-1]
    mid = len(closes) // 2
    if cum_vol[mid] <= 0:
        return True
    vwap_mid = cum_pv[mid] / cum_vol[mid]
    if side == "long":
        return vwap_end >= vwap_mid
    else:
        return vwap_end <= vwap_mid


def _check_volume_acceleration(
    second_bars: pd.DataFrame,
    confirm_pos: int,
    window_seconds: int,
    ratio: float,
) -> bool:
    """Check if volume in recent window exceeds prior window by ratio."""
    half = window_seconds // 2
    if half < 10:
        half = 10
    recent_start = max(0, confirm_pos - half)
    prior_start = max(0, recent_start - half)
    if prior_start == recent_start:
        return True
    recent_vol = float(second_bars["volume"].iloc[recent_start:confirm_pos + 1].sum())
    prior_vol = float(second_bars["volume"].iloc[prior_start:recent_start].sum())
    if prior_vol <= 0:
        return recent_vol > 0
    return recent_vol >= prior_vol * ratio


def simulate_position(
    second_bars: pd.DataFrame,
    events_by_end: dict,
    sig: pd.Series,
    side: str,
    params: dict,
    *,
    start_pos: int,
    balance: float,
):
    """Run the position simulation with V2 confirm gates."""
    second_index = second_bars.index
    high_values = second_bars["high"].to_numpy(dtype="float64", copy=False)
    low_values = second_bars["low"].to_numpy(dtype="float64", copy=False)
    close_values = second_bars["close"].to_numpy(dtype="float64", copy=False)
    atr = float(sig["atr"])

    confirm_atr = float(params.get("confirm_atr", 0.0))
    confirm_window_seconds = int(params.get("confirm_window_seconds", 0))
    fail_retrace_atr = float(params.get("fail_retrace_atr", 0.0))

    # --- V2 confirm phase ---
    if confirm_window_seconds > 0 and confirm_atr > 0:
        window_end = second_index[start_pos] + pd.Timedelta(seconds=confirm_window_seconds)
        end_pos = min(int(second_index.searchsorted(window_end, side="right")), len(second_index))

        if side == "long":
            confirm_level = float(sig["close"]) + confirm_atr * atr
            fail_level = float(sig["close"]) - fail_retrace_atr * atr if fail_retrace_atr > 0 else -np.inf
            confirm_pos = None
            for pos in range(start_pos, end_pos):
                if float(low_values[pos]) <= fail_level:
                    return None, balance, "early_reversal"
                if float(high_values[pos]) >= confirm_level:
                    confirm_pos = pos
                    break
        else:
            confirm_level = float(sig["close"]) - confirm_atr * atr
            fail_level = float(sig["close"]) + fail_retrace_atr * atr if fail_retrace_atr > 0 else np.inf
            confirm_pos = None
            for pos in range(start_pos, end_pos):
                if float(high_values[pos]) >= fail_level:
                    return None, balance, "early_reversal"
                if float(low_values[pos]) <= confirm_level:
                    confirm_pos = pos
                    break

        if confirm_pos is None:
            return None, balance, "confirm_timeout"

        # V2 gate: VWAP slope
        if float(params.get("vwap_confirm", 0.0)) > 0:
            vwap_window = int(params.get("vwap_window_seconds", 300))
            if not _compute_vwap_slope(second_bars, start_pos, confirm_pos, side, vwap_window):
                return None, balance, "vwap_reject"

        # V2 gate: volume acceleration
        if float(params.get("volume_acceleration", 0.0)) > 0:
            vol_ratio = float(params.get("vol_accel_ratio", 1.2))
            if not _check_volume_acceleration(second_bars, confirm_pos, confirm_window_seconds, vol_ratio):
                return None, balance, "vol_accel_reject"

        # V2 gate: intrabar close position
        close_gate = float(params.get("intrabar_close_gate", 0.0))
        if close_gate > 0:
            bar_high = float(np.max(high_values[start_pos:confirm_pos + 1]))
            bar_low = float(np.min(low_values[start_pos:confirm_pos + 1]))
            bar_range = bar_high - bar_low
            if bar_range > 0:
                close_pos_val = (float(close_values[confirm_pos]) - bar_low) / bar_range
                if side == "long" and close_pos_val < close_gate:
                    return None, balance, "closepos_reject"
                if side == "short" and close_pos_val > (1.0 - close_gate):
                    return None, balance, "closepos_reject"

        # V2 gate: post-confirm retrace rejection
        retrace_ratio = float(params.get("post_confirm_retrace", 0.0))
        if retrace_ratio > 0:
            watch_secs = int(params.get("post_confirm_watch_seconds", 30))
            watch_end = min(confirm_pos + watch_secs, end_pos)
            confirm_close = float(close_values[confirm_pos])
            signal_close = float(sig["close"])
            confirm_move = abs(confirm_close - signal_close)
            if confirm_move > 0 and watch_end > confirm_pos + 1:
                for wpos in range(confirm_pos + 1, watch_end):
                    if side == "long":
                        retrace_amt = confirm_close - float(low_values[wpos])
                    else:
                        retrace_amt = float(high_values[wpos]) - confirm_close
                    if retrace_amt > 0 and retrace_amt / confirm_move > retrace_ratio:
                        return None, balance, "post_retrace_reject"

        start_pos = confirm_pos

    # --- Entry ---
    entry_time = second_index[start_pos]
    entry_raw = float(close_values[start_pos])

    if side == "long":
        if entry_raw - float(sig["close"]) > float(params["max_entry_extension_atr"]) * atr:
            return None, balance, "entry_extension"
    else:
        if float(sig["close"]) - entry_raw > float(params["max_entry_extension_atr"]) * atr:
            return None, balance, "entry_extension"

    position = base.open_position(sig, side, entry_raw, entry_time, balance, params)
    if position is None:
        return None, balance, "min_stop"
    logs: list[dict] = []
    base.append_entry(logs, position, balance)

    max_hold_end = entry_time + pd.Timedelta(hours=int(params["max_hold_bars"]))
    end_pos_sim = min(int(second_index.searchsorted(max_hold_end, side="left")), len(second_index) - 1)

    for pos in range(start_pos + 1, end_pos_sim + 1):
        bar_time = second_index[pos]
        hv = float(high_values[pos])
        lv = float(low_values[pos])
        cv = float(close_values[pos])
        base.update_excursion(position, hv, lv, params)

        triggered, raw_exit, reason = base.stop_trigger(position, hv, lv)
        if triggered:
            balance = base.append_exit(logs, position, raw_exit=raw_exit, time_value=bar_time, reason=reason, balance=balance, params=params)
            return logs, balance, ""

        event = events_by_end.get(bar_time)
        if event is not None and bar_time > entry_time:
            exit_now, event_reason = base.apply_hourly_event(position, event, params)
            if exit_now:
                balance = base.append_exit(logs, position, raw_exit=cv, time_value=bar_time, reason=event_reason, balance=balance, params=params)
                return logs, balance, ""

        if bar_time >= max_hold_end:
            balance = base.append_exit(logs, position, raw_exit=cv, time_value=bar_time, reason="MaxHoldExit", balance=balance, params=params)
            return logs, balance, ""

    balance = base.append_exit(logs, position, raw_exit=float(close_values[end_pos_sim]), time_value=second_index[end_pos_sim], reason="FinalMarkToMarket", balance=balance, params=params)
    return logs, balance, ""


# ---------------------------------------------------------------------------
# Strategy runner
# ---------------------------------------------------------------------------

def run_strategy(second_bars: pd.DataFrame, signal: pd.DataFrame, params: dict, *, initial_balance: float) -> dict:
    second_index = second_bars.index
    events_by_end = {pd.Timestamp(row["bar_end"]): row for _, row in signal.iterrows()}
    balance = float(initial_balance)
    logs: list[dict] = []
    last_exit_time = pd.Timestamp.min.tz_localize("UTC")
    diagnostics = {
        "candidate_signals": 0, "entries": 0,
        "entry_extension_skipped": 0, "busy_skipped": 0,
        "confirm_timeout_skipped": 0, "early_reversal_skipped": 0,
        "vwap_reject_skipped": 0, "vol_accel_reject_skipped": 0,
        "closepos_reject_skipped": 0, "post_retrace_reject_skipped": 0,
        "min_stop_skipped": 0,
    }
    diag_map = {
        "entry_extension": "entry_extension_skipped",
        "confirm_timeout": "confirm_timeout_skipped",
        "early_reversal": "early_reversal_skipped",
        "vwap_reject": "vwap_reject_skipped",
        "vol_accel_reject": "vol_accel_reject_skipped",
        "closepos_reject": "closepos_reject_skipped",
        "post_retrace_reject": "post_retrace_reject_skipped",
        "min_stop": "min_stop_skipped",
    }

    tf_delta = base._freq_to_timedelta("1h")
    for bar_start, sig in signal.iterrows():
        side = base.signal_side(sig, params)
        if not side:
            continue
        diagnostics["candidate_signals"] += 1
        bar_end = pd.Timestamp(sig["bar_end"])
        if bar_end <= last_exit_time:
            diagnostics["busy_skipped"] += 1
            continue
        start_pos = int(second_index.searchsorted(bar_end, side="left"))
        if start_pos >= len(second_index):
            continue

        result, balance, skip_reason = simulate_position(
            second_bars, events_by_end, sig, side, params,
            start_pos=start_pos, balance=balance,
        )
        if result is None:
            key = diag_map.get(skip_reason, "")
            if key:
                diagnostics[key] += 1
            continue
        diagnostics["entries"] += 1
        logs.extend(result)
        last_exit_time = pd.Timestamp(result[-1]["time"])

    ledger = pd.DataFrame(logs)
    summary = base.summarize_ledger(ledger, initial_balance, params) if not ledger.empty else {
        "trades": 0, "raw_no_fee_no_slip_return_pct": 0.0,
        "price_pnl_with_2bps_slip_no_fee_return_pct": 0.0,
        "realistic_return_pct": 0.0, "win_rate_pct": 0.0,
        "max_dd_pct": 0.0,
    }
    return {"summary": summary, "diagnostics": diagnostics, "ledger": ledger}


# ---------------------------------------------------------------------------
# Report
# ---------------------------------------------------------------------------

def write_markdown(summary: dict, output_path: Path) -> None:
    lines = [
        f"# Impulse Bar Delayed Confirm V2 1s Replay ({summary['start']} to {summary['end']})",
        "",
        "Scope: research-only. Extends impulse bar confirm sweep with VWAP slope, volume acceleration, intrabar close position, and post-confirm retrace gates.",
        "",
        f"成本：滑点 2bps/side，手续费 maker entry 2bps + market exit 4bps。",
        "",
        "| Symbol | Variant | Trades | Realistic | Raw | 2bps Slip | Win Rate | Max DD | Diag |",
        "|---|---|---:|---:|---:|---:|---:|---:|---|",
    ]
    for result in summary["results"]:
        s = result["summary"]
        d = result["diagnostics"]
        lines.append(
            f"| `{result['symbol']}` | `{result['variant']}` | {s['trades']} | "
            f"{s['realistic_return_pct']:.4f}% | {s['raw_no_fee_no_slip_return_pct']:.4f}% | "
            f"{s['price_pnl_with_2bps_slip_no_fee_return_pct']:.4f}% | "
            f"{s['win_rate_pct']:.2f}% | {s['max_dd_pct']:.4f}% | `{d}` |"
        )
    lines.extend(["", "## Files", ""])
    lines.append(f"- Summary JSON: `{summary['summary_path']}`")
    for result in summary["results"]:
        lines.append(f"- `{result['symbol']} {result['variant']}` ledger: `{result['ledger_path']}`")
    lines.append("")
    output_path.write_text("\n".join(lines), encoding="utf-8")


# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------

def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Impulse bar delayed confirm V2 1s replay")
    parser.add_argument("--symbols", nargs="+", default=["BTCUSDT", "ETHUSDT"])
    parser.add_argument("--start", default="2026-01-01T00:00:00Z")
    parser.add_argument("--end", default="2026-04-30T23:59:59Z")
    parser.add_argument("--archive-root", default=str(DEFAULT_ARCHIVE_ROOT))
    parser.add_argument("--signal-timeframe", default="1h")
    parser.add_argument("--trend-timeframe", default="4h")
    parser.add_argument("--chunksize", type=int, default=2_000_000)
    parser.add_argument("--initial-balance", type=float, default=100000.0)
    parser.add_argument(
        "--variants", nargs="+",
        default=["c03_f10", "c03_f10_vwap", "c03_f10_vol", "c03_f10_full",
                 "c05_f15", "c05_f15_full", "c03_f10_closepos", "c03_f10_retrace", "c03_f10_all"],
    )
    parser.add_argument("--summary-json", default="research/impulse_bar_delayed_confirm_v2_summary.json")
    parser.add_argument("--markdown", default="research/20260508_impulse_bar_delayed_confirm_v2.md")
    parser.add_argument("--ledger-prefix", default="research/tmp_impulse_delayed_confirm_v2")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    started = time.time()
    start = base._as_utc(args.start)
    end = base._as_utc(args.end)
    variants = [parse_variant(raw) for raw in args.variants]
    results = []

    for symbol in args.symbols:
        tick_files = base.monthly_trade_files(symbol, start, end, Path(args.archive_root))
        second_bars, build_stats = replay.build_continuous_second_bars(tick_files, start, end, args.chunksize)
        _, signal, trend = base.build_frames(
            second_bars,
            signal_timeframe=args.signal_timeframe,
            trend_timeframe=args.trend_timeframe,
        )
        print(
            f"{symbol}: second_rows={len(second_bars)} signal_rows={len(signal)}",
            flush=True,
        )
        for variant_name, params in variants:
            print(f"running {symbol} {variant_name}", flush=True)
            result = run_strategy(second_bars, signal, params, initial_balance=args.initial_balance)
            ledger_path = Path(f"{args.ledger_prefix}_{symbol}_{variant_name}_ledger.csv")
            result["ledger"].to_csv(ledger_path, index=False)
            del result["ledger"]
            result.update({
                "symbol": symbol, "variant": variant_name,
                "params": params, "ledger_path": str(ledger_path),
                "build_stats": build_stats,
            })
            s = result["summary"]
            print(
                f"{symbol} {variant_name}: realistic={s['realistic_return_pct']:.4f}% "
                f"raw={s['raw_no_fee_no_slip_return_pct']:.4f}% trades={s['trades']} "
                f"win={s['win_rate_pct']:.2f}% diag={result['diagnostics']}",
                flush=True,
            )
            results.append(result)

    summary_path = Path(args.summary_json)
    markdown_path = Path(args.markdown)
    summary = {
        "start": start.isoformat(), "end": end.isoformat(),
        "signal_timeframe": args.signal_timeframe,
        "trend_timeframe": args.trend_timeframe,
        "execution": "continuous 1s OHLC bars from local trade ticks",
        "accounting": "2bps/side slippage plus maker entry 2bps and market exit 4bps",
        "variants": [{"name": n, "params": p} for n, p in variants],
        "results": results,
        "summary_path": str(summary_path),
        "markdown_path": str(markdown_path),
        "elapsed_seconds": round(time.time() - started, 2),
    }
    summary_path.write_text(json.dumps(summary, indent=2, ensure_ascii=False), encoding="utf-8")
    write_markdown(summary, markdown_path)
    print(json.dumps({"summary_path": str(summary_path), "elapsed_seconds": summary["elapsed_seconds"]}, indent=2), flush=True)


if __name__ == "__main__":
    main()
