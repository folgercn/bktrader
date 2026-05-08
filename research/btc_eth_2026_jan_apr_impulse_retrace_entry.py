#!/usr/bin/env python3
"""Impulse Retrace Entry — enter on pullback instead of at bar close.

Same impulse bar signal detection as impulse_bar_run, but instead of entering
immediately at the 1s close after the signal bar, we wait for price to retrace
back toward the breakout level (0.25 ATR pullback). This gives:
  1. Better entry price → tighter stop → better R:R
  2. Confirmation that pullback holds → higher quality entries
  3. Fewer but higher-conviction trades

Additional filters vs impulse_bar_run:
  - Volume filter: signal bar volume > 1.2x 20-bar MA
  - No NoNewExtreme exit (killed 24% of trades prematurely)
  - Longer max hold (10h vs 6h)
"""

from __future__ import annotations

import argparse
import json
import time
from pathlib import Path

import numpy as np
import pandas as pd

import eth_q1_breakout_t3_shape_compare as replay
import btc_eth_2026_jan_apr_impulse_bar_run as base

DEFAULT_ARCHIVE_ROOT = Path("dataset/archive")


def build_frames(second_bars: pd.DataFrame):
    """Same as base but adds volume_ma20 and close_to_ema20_atr."""
    one_min, signal, trend = base.build_frames(
        second_bars, signal_timeframe="1h", trend_timeframe="4h",
    )
    signal["volume_ma20"] = signal["volume"].rolling(20).mean()
    signal["close_to_ema20_atr"] = (
        (signal["close"] - signal["ema20"]).abs()
        / signal["atr"].replace(0, np.nan)
    )
    return one_min, signal, trend


def parse_variant(raw: str) -> tuple[str, dict]:
    presets = {
        "retrace25_vol12": {"retrace_depth_atr": 0.25, "vol_mult": 1.2},
        "retrace15_vol12": {"retrace_depth_atr": 0.15, "vol_mult": 1.2},
        "retrace25_novol": {"retrace_depth_atr": 0.25, "vol_mult": 0.0},
    }
    defaults = {
        "break_lookback": 8,
        "body_min": 0.55,
        "close_top": 0.75,
        "range_min_atr": 0.85,
        "range_max_atr": 2.0,
        "pre_range_atr": 3.0,
        "max_atr_percentile": 95.0,
        "vol_mult": 1.2,
        "max_close_ema20_atr": 2.5,
        "retrace_depth_atr": 0.25,
        "max_away_atr": 0.50,
        "entry_window_seconds": 7200,
        "initial_stop_atr": 0.40,
        "stop_buffer_atr": 0.05,
        "stop_cap_atr": 0.70,
        "min_stop_bps": 12.0,
        "breakeven_at_r": 1.0,
        "cost_lock_bps": 10.0,
        "trail_start_r": 1.5,
        "trail_buffer_atr": 0.04,
        "max_hold_hours": 10,
        "notional_share": 0.20,
        "slippage": 0.0002,
        "entry_fee": 0.0002,
        "exit_fee": 0.0004,
    }
    if raw in presets:
        defaults.update(presets[raw])
        return raw, defaults
    return raw, defaults


def signal_side(sig: pd.Series, params: dict) -> str:
    """Same filters as impulse_bar_run + volume + EMA20 extension."""
    atr = sig.get("atr", np.nan)
    bar_range = sig.get("range", np.nan)
    if not base._finite_positive(atr, bar_range, sig.get("pre_range_6", np.nan)):
        return ""
    atr_pct = sig.get("atr_percentile", np.nan)
    if pd.notna(atr_pct) and float(atr_pct) > float(params["max_atr_percentile"]):
        return ""
    if float(sig["pre_range_6"]) > float(params["pre_range_atr"]) * float(atr):
        return ""
    range_atr = float(bar_range) / float(atr)
    if range_atr < float(params["range_min_atr"]) or range_atr > float(params["range_max_atr"]):
        return ""
    if float(sig.get("body_ratio", 0.0)) < float(params["body_min"]):
        return ""

    # Volume filter
    vol_mult = float(params["vol_mult"])
    if vol_mult > 0:
        vol_ma = sig.get("volume_ma20", np.nan)
        if pd.notna(vol_ma) and float(vol_ma) > 0:
            if float(sig["volume"]) < vol_mult * float(vol_ma):
                return ""

    # EMA20 extension filter
    ext = sig.get("close_to_ema20_atr", np.nan)
    if pd.notna(ext) and float(ext) > float(params["max_close_ema20_atr"]):
        return ""

    lookback = int(params["break_lookback"])
    if (
        base._trend_ready(sig, "long")
        and float(sig["close"]) > float(sig[f"prev_high_{lookback}"])
        and float(sig["close_pos"]) >= float(params["close_top"])
    ):
        return "long"
    if (
        base._trend_ready(sig, "short")
        and float(sig["close"]) < float(sig[f"prev_low_{lookback}"])
        and float(sig["close_pos"]) <= 1.0 - float(params["close_top"])
    ):
        return "short"
    return ""


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
    """Two-phase simulation: find retrace entry, then manage position."""
    idx = second_bars.index
    highs = second_bars["high"].to_numpy(dtype="float64", copy=False)
    lows = second_bars["low"].to_numpy(dtype="float64", copy=False)
    closes = second_bars["close"].to_numpy(dtype="float64", copy=False)

    atr = float(sig["atr"])
    sig_close = float(sig["close"])
    retrace_depth = float(params["retrace_depth_atr"]) * atr
    max_away = float(params["max_away_atr"]) * atr
    window_end = idx[start_pos] + pd.Timedelta(seconds=int(params["entry_window_seconds"]))

    # --- Phase 1: find retrace entry ---
    entry_pos = None
    for pos in range(start_pos, len(idx)):
        if idx[pos] > window_end:
            break
        h, l = float(highs[pos]), float(lows[pos])
        if side == "long":
            if h > sig_close + max_away:
                return None, balance, "ran_away"
            if l <= sig_close - retrace_depth:
                entry_pos = pos
                break
        else:
            if l < sig_close - max_away:
                return None, balance, "ran_away"
            if h >= sig_close + retrace_depth:
                entry_pos = pos
                break

    if entry_pos is None:
        return None, balance, "no_retrace"

    # Entry at the retrace target level (+ slippage)
    slippage = float(params["slippage"])
    if side == "long":
        entry_raw = sig_close - retrace_depth
        entry_p = entry_raw * (1.0 + slippage)
    else:
        entry_raw = sig_close + retrace_depth
        entry_p = entry_raw * (1.0 - slippage)

    # Stop placement
    if side == "long":
        raw_stop = entry_p - float(params["initial_stop_atr"]) * atr
        capped = entry_p - float(params["stop_cap_atr"]) * atr
        stop = max(raw_stop, capped)
        risk = entry_p - stop
    else:
        raw_stop = entry_p + float(params["initial_stop_atr"]) * atr
        capped = entry_p + float(params["stop_cap_atr"]) * atr
        stop = min(raw_stop, capped)
        risk = stop - entry_p

    if risk <= 0 or risk < entry_p * float(params["min_stop_bps"]) / 10000.0:
        return None, balance, "min_stop"

    position = {
        "side": side, "entry_time": idx[entry_pos],
        "entry_p": entry_p, "entry_raw": entry_raw,
        "sl": stop, "initial_sl": stop, "risk": risk,
        "atr_at_entry": atr,
        "notional": balance * float(params["notional_share"]),
        "notional_share": float(params["notional_share"]),
        "signal_bar_time": sig.name, "signal_close": sig_close,
        "signal_high": float(sig["high"]), "signal_low": float(sig["low"]),
        "protected": False, "trailing_active": False,
        "hwm": entry_p, "lwm": entry_p,
        "mfe_r": 0.0, "mae_r": 0.0,
    }
    logs: list[dict] = []
    base.append_entry(logs, position, balance)

    # --- Phase 2: position management ---
    max_hold_end = idx[entry_pos] + pd.Timedelta(hours=int(params["max_hold_hours"]))
    end_pos = min(int(idx.searchsorted(max_hold_end, side="left")), len(idx) - 1)

    for pos in range(entry_pos + 1, end_pos + 1):
        h, l = float(highs[pos]), float(lows[pos])
        base.update_excursion(position, h, l, params)

        triggered, raw_exit, reason = base.stop_trigger(position, h, l)
        if triggered:
            balance = base.append_exit(
                logs, position, raw_exit=raw_exit, time_value=idx[pos],
                reason=reason, balance=balance, params=params,
            )
            return logs, balance, ""

        # Hourly trailing stop update (no NoNewExtreme/EMA8 early exit)
        event = events_by_end.get(idx[pos])
        if event is not None and idx[pos] > idx[entry_pos]:
            if float(position["mfe_r"]) >= float(params["trail_start_r"]):
                if side == "long":
                    trail = float(event["low"]) - float(params["trail_buffer_atr"]) * atr
                    if trail > float(position["sl"]):
                        position["sl"] = trail
                        position["trailing_active"] = True
                else:
                    trail = float(event["high"]) + float(params["trail_buffer_atr"]) * atr
                    if trail < float(position["sl"]):
                        position["sl"] = trail
                        position["trailing_active"] = True

        if idx[pos] >= max_hold_end:
            balance = base.append_exit(
                logs, position, raw_exit=float(closes[pos]), time_value=idx[pos],
                reason="MaxHoldExit", balance=balance, params=params,
            )
            return logs, balance, ""

    balance = base.append_exit(
        logs, position, raw_exit=float(closes[end_pos]), time_value=idx[end_pos],
        reason="FinalMarkToMarket", balance=balance, params=params,
    )
    return logs, balance, ""


def run_strategy(second_bars, signal, params, *, initial_balance):
    idx = second_bars.index
    events_by_end = {pd.Timestamp(row["bar_end"]): row for _, row in signal.iterrows()}
    balance = float(initial_balance)
    logs: list[dict] = []
    last_exit_time = pd.Timestamp.min.tz_localize("UTC")
    diag = {
        "candidate_signals": 0, "entries": 0, "busy_skipped": 0,
        "no_retrace_skipped": 0, "ran_away_skipped": 0,
        "min_stop_skipped": 0, "long_signals": 0, "short_signals": 0,
    }

    for _, sig in signal.iterrows():
        side = signal_side(sig, params)
        if not side:
            continue
        diag["candidate_signals"] += 1
        diag[f"{side}_signals"] += 1
        entry_time = pd.Timestamp(sig["bar_end"])
        if entry_time <= last_exit_time:
            diag["busy_skipped"] += 1
            continue
        start_pos = int(idx.searchsorted(entry_time, side="left"))
        if start_pos >= len(idx):
            continue
        trade_logs, new_balance, skip = simulate_position(
            second_bars, events_by_end, sig, side, params,
            start_pos=start_pos, balance=balance,
        )
        if skip == "no_retrace":
            diag["no_retrace_skipped"] += 1
            continue
        if skip == "ran_away":
            diag["ran_away_skipped"] += 1
            continue
        if skip == "min_stop":
            diag["min_stop_skipped"] += 1
            continue
        if not trade_logs:
            continue
        logs.extend(trade_logs)
        balance = new_balance
        diag["entries"] += 1
        last_exit_time = pd.Timestamp(trade_logs[-1]["time"])

    ledger = pd.DataFrame(logs)
    return {
        "summary": base.summarize_ledger(ledger, initial_balance, params),
        "diagnostics": diag,
        "ledger": ledger,
    }


def write_markdown(summary: dict, output_path: Path) -> None:
    lines = [
        "# Impulse Retrace Entry 1s Replay",
        "",
        "Scope: research-only. Same impulse bar signals as impulse_bar_run but enters "
        "on first pullback (retrace) instead of at bar close. 2 bps/side slippage, "
        "maker entry 2 bps + market exit 4 bps.",
        "",
        "| Symbol | Variant | Trades | Realistic | Raw | 2bps Slip | Win Rate "
        "| Max DD | Med Hold | Med MFE R | Exits | Cands | Entries "
        "| NoRetrace | RanAway | Busy |",
        "|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|---:|",
    ]
    for r in summary["results"]:
        s, d = r["summary"], r["diagnostics"]
        lines.append(
            f"| `{r['symbol']}` | `{r['variant']}` | {s['trades']} "
            f"| {s['realistic_return_pct']:.4f}% | {s['raw_no_fee_no_slip_return_pct']:.4f}% "
            f"| {s['price_pnl_with_2bps_slip_no_fee_return_pct']:.4f}% "
            f"| {s['win_rate_pct']:.2f}% | {s['max_dd_pct']:.4f}% "
            f"| {s.get('median_hold_seconds', 0):.0f}s | {s.get('median_mfe_r', 0):.4f} "
            f"| `{s['exit_reasons']}` | {d['candidate_signals']} | {d['entries']} "
            f"| {d['no_retrace_skipped']} | {d['ran_away_skipped']} | {d['busy_skipped']} |"
        )
    lines.extend(["", "## Files", ""])
    lines.append(f"- Summary JSON: `{summary['summary_path']}`")
    for r in summary["results"]:
        lines.append(f"- `{r['symbol']} {r['variant']}` ledger: `{r['ledger_path']}`")
    lines.append("")
    output_path.write_text("\n".join(lines), encoding="utf-8")


def main() -> None:
    ap = argparse.ArgumentParser(description="Impulse retrace entry 1s replay")
    ap.add_argument("--symbols", nargs="+", default=["BTCUSDT", "ETHUSDT"])
    ap.add_argument("--start", default="2026-01-01T00:00:00Z")
    ap.add_argument("--end", default="2026-04-30T23:59:59Z")
    ap.add_argument("--archive-root", default=str(DEFAULT_ARCHIVE_ROOT))
    ap.add_argument("--chunksize", type=int, default=2_000_000)
    ap.add_argument("--initial-balance", type=float, default=100000.0)
    ap.add_argument(
        "--variants", nargs="+",
        default=["retrace25_vol12", "retrace15_vol12", "retrace25_novol"],
    )
    ap.add_argument("--summary-json", default="research/btc_eth_impulse_retrace_entry_summary.json")
    ap.add_argument("--markdown", default="research/20260507_btc_eth_impulse_retrace_entry.md")
    ap.add_argument("--ledger-prefix", default="research/tmp_impulse_retrace_entry")
    args = ap.parse_args()
    started = time.time()
    start = base._as_utc(args.start)
    end = base._as_utc(args.end)
    variants = [parse_variant(v) for v in args.variants]
    results = []

    for symbol in args.symbols:
        tick_files = base.monthly_trade_files(symbol, start, end, Path(args.archive_root))
        second_bars, build_stats = replay.build_continuous_second_bars(
            tick_files, start, end, args.chunksize,
        )
        _, signal, _ = build_frames(second_bars)
        print(f"{symbol}: second_rows={len(second_bars)} signal_rows={len(signal)}", flush=True)
        for vname, params in variants:
            print(f"running {symbol} {vname}", flush=True)
            result = run_strategy(second_bars, signal, params, initial_balance=args.initial_balance)
            lpath = Path(f"{args.ledger_prefix}_{symbol}_{vname}_ledger.csv")
            result["ledger"].to_csv(lpath, index=False)
            del result["ledger"]
            result.update({
                "symbol": symbol, "variant": vname, "params": params,
                "ledger_path": str(lpath), "build_stats": build_stats,
            })
            s = result["summary"]
            print(
                f"{symbol} {vname}: realistic={s['realistic_return_pct']:.4f}% "
                f"raw={s['raw_no_fee_no_slip_return_pct']:.4f}% trades={s['trades']} "
                f"win={s['win_rate_pct']:.2f}% dd={s['max_dd_pct']:.4f}% "
                f"diag={result['diagnostics']}",
                flush=True,
            )
            results.append(result)

    summary = {
        "start": start.isoformat(), "end": end.isoformat(),
        "signal": "impulse bar signals with retrace entry",
        "execution": "1s OHLC from local trade ticks, entry on pullback",
        "accounting": "2bps/side slip + maker 2bps + taker 4bps",
        "variants": [{"name": n, "params": p} for n, p in variants],
        "results": results,
        "summary_path": str(args.summary_json),
        "markdown_path": str(args.markdown),
        "elapsed_seconds": round(time.time() - started, 2),
    }
    Path(args.summary_json).write_text(json.dumps(summary, indent=2, ensure_ascii=False), encoding="utf-8")
    write_markdown(summary, Path(args.markdown))
    print(json.dumps({
        "summary_path": args.summary_json,
        "markdown_path": args.markdown,
        "elapsed_seconds": summary["elapsed_seconds"],
    }, indent=2), flush=True)


if __name__ == "__main__":
    main()
