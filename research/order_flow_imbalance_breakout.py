#!/usr/bin/env python3
"""Order flow imbalance breakout research replay.

Research-only. Uses raw tick buy/sell aggressor imbalance as a confirmation
gate after closed-bar impulse breakout, instead of price continuation.
"""
from __future__ import annotations
import argparse, json, time
from pathlib import Path
import numpy as np, pandas as pd
import eth_q1_breakout_t3_shape_compare as replay

try:
    import btc_eth_2026_jan_apr_impulse_bar_run as base
except ModuleNotFoundError:
    base = None

DEFAULT_ARCHIVE_ROOT = Path("dataset/archive")

# ---------------------------------------------------------------------------
# Raw tick loader with is_buyer_maker
# ---------------------------------------------------------------------------
def _read_tick_chunks_with_side(path: str, chunksize: int):
    try:
        reader = pd.read_csv(path, header=0,
            usecols=["price","qty","time","is_buyer_maker"],
            dtype={"price":"float32","qty":"float32","time":"int64","is_buyer_maker":"object"},
            chunksize=chunksize, compression="infer")
    except ValueError:
        reader = pd.read_csv(path, header=None,
            names=["id","price","qty","quote_qty","time","is_buyer_maker","is_best_match"],
            usecols=["price","qty","time","is_buyer_maker"],
            dtype={"price":"float32","qty":"float32","time":"int64","is_buyer_maker":"object"},
            chunksize=chunksize, compression="infer")
    for chunk in reader:
        if "time" in chunk.columns:
            chunk.rename(columns={"time":"timestamp"}, inplace=True)
        if not chunk.empty and int(chunk["timestamp"].iloc[0]) > 10_000_000_000_000:
            chunk["timestamp"] = chunk["timestamp"] // 1000
        # is_buyer_maker: "true"=seller-initiated, "false"=buyer-initiated
        chunk["is_buy"] = chunk["is_buyer_maker"].astype(str).str.lower() != "true"
        yield chunk

def build_second_bars_with_flow(paths, start: pd.Timestamp, end: pd.Timestamp, chunksize: int):
    """Build 1s bars with buy/sell volume columns."""
    start_ms, end_ms = int(start.timestamp()*1000), int(end.timestamp()*1000)
    pending, summaries = None, []
    raw_rows, kept_rows = 0, 0
    for path in paths:
        print(f"reading ticks: {path}", flush=True)
        for ci, chunk in enumerate(_read_tick_chunks_with_side(path, chunksize), 1):
            if chunk.empty: continue
            raw_rows += len(chunk)
            if chunk["timestamp"].iloc[0] > end_ms: break
            if chunk["timestamp"].iloc[-1] < start_ms: continue
            chunk = chunk[(chunk["timestamp"]>=start_ms)&(chunk["timestamp"]<=end_ms)].copy()
            if chunk.empty: continue
            kept_rows += len(chunk)
            chunk["buy_qty"] = chunk["qty"].where(chunk["is_buy"], 0.0)
            chunk["sell_qty"] = chunk["qty"].where(~chunk["is_buy"], 0.0)
            if pending is not None and not pending.empty:
                chunk = pd.concat([pending, chunk], ignore_index=True)
                pending = None
            chunk["second_ms"] = (chunk["timestamp"]//1000)*1000
            last_s = chunk["second_ms"].iloc[-1]
            pending = chunk[chunk["second_ms"]==last_s].copy()
            complete = chunk[chunk["second_ms"]!=last_s]
            if complete.empty: continue
            sdf = complete.groupby("second_ms", sort=False).agg(
                open=("price","first"), high=("price","max"),
                low=("price","min"), close=("price","last"),
                volume=("qty","sum"), buy_volume=("buy_qty","sum"),
                sell_volume=("sell_qty","sum"), trade_count=("qty","count"),
            )
            summaries.append(sdf.reset_index())
            if ci % 50 == 0: print(f"  chunks={ci} kept={kept_rows:,}", flush=True)
    if pending is not None and not pending.empty:
        pending["buy_qty"] = pending["qty"].where(pending["is_buy"], 0.0)
        pending["sell_qty"] = pending["qty"].where(~pending["is_buy"], 0.0)
        sdf = pending.groupby("second_ms", sort=False).agg(
            open=("price","first"), high=("price","max"),
            low=("price","min"), close=("price","last"),
            volume=("qty","sum"), buy_volume=("buy_qty","sum"),
            sell_volume=("sell_qty","sum"), trade_count=("qty","count"),
        )
        summaries.append(sdf.reset_index())
    if not summaries: raise RuntimeError("no tick data aggregated")
    sb = pd.concat(summaries, ignore_index=True).sort_values("second_ms")
    sb["timestamp"] = pd.to_datetime(sb["second_ms"], unit="ms", utc=True)
    sb.set_index("timestamp", inplace=True)
    sb = sb[["open","high","low","close","volume","buy_volume","sell_volume","trade_count"]]
    full_idx = pd.date_range(start=start, end=end, freq="1s", tz="UTC")
    sb = sb.reindex(full_idx)
    fc = float(sb["close"].dropna().iloc[0])
    sb["close"] = sb["close"].ffill().fillna(fc)
    for c in ("open","high","low"): sb[c] = sb[c].fillna(sb["close"])
    for c in ("volume","buy_volume","sell_volume","trade_count"): sb[c] = sb[c].fillna(0.0)
    stats = {"raw_tick_rows":int(raw_rows),"kept_tick_rows":int(kept_rows),"second_rows":int(len(sb))}
    return sb, stats

# ---------------------------------------------------------------------------
# Order flow imbalance check
# ---------------------------------------------------------------------------
def check_order_flow_imbalance(sb: pd.DataFrame, pos: int, side: str, params: dict) -> tuple[bool,str]:
    window = int(params.get("imbalance_window_seconds", 120))
    w_start = max(0, pos - window)
    buy_vol = float(sb["buy_volume"].iloc[w_start:pos+1].sum())
    sell_vol = float(sb["sell_volume"].iloc[w_start:pos+1].sum())
    total = buy_vol + sell_vol
    if total <= 0: return False, "no_volume"
    buy_ratio = buy_vol / total
    min_ratio = float(params.get("min_buy_ratio", 0.55))
    if side == "long" and buy_ratio < min_ratio: return False, "imbalance_reject"
    if side == "short" and (1.0 - buy_ratio) < min_ratio: return False, "imbalance_reject"
    # Large trade imbalance
    mult = float(params.get("large_trade_multiplier", 3.0))
    min_large = float(params.get("min_large_imbalance", 0.0))
    if min_large > 0 and window > 10:
        volumes = sb["volume"].iloc[w_start:pos+1].values.astype("float64")
        med_vol = float(np.median(volumes[volumes > 0])) if (volumes > 0).any() else 0.0
        if med_vol > 0:
            buys = sb["buy_volume"].iloc[w_start:pos+1].values.astype("float64")
            sells = sb["sell_volume"].iloc[w_start:pos+1].values.astype("float64")
            threshold = med_vol * mult
            large_buy = float(buys[buys >= threshold].sum())
            large_sell = float(sells[sells >= threshold].sum())
            large_total = large_buy + large_sell
            if large_total > 0:
                large_ratio = large_buy / large_total if side == "long" else large_sell / large_total
                if large_ratio < min_large: return False, "large_trade_reject"
    return True, "ok"

# ---------------------------------------------------------------------------
# Position simulation
# ---------------------------------------------------------------------------
def simulate_position(sb, events_by_end, sig, side, params, *, start_pos, balance):
    idx = sb.index
    hv = sb["high"].to_numpy(dtype="float64", copy=False)
    lv = sb["low"].to_numpy(dtype="float64", copy=False)
    cv = sb["close"].to_numpy(dtype="float64", copy=False)
    atr = float(sig["atr"])
    # Confirm via price first (reuse base logic)
    confirm_atr = float(params.get("confirm_atr", 0.03))
    window_secs = int(params.get("confirm_window_seconds", 120))
    fail_atr = float(params.get("fail_retrace_atr", 0.10))
    if window_secs > 0 and confirm_atr > 0:
        wend_t = idx[start_pos] + pd.Timedelta(seconds=window_secs)
        end_p = min(int(idx.searchsorted(wend_t, side="right")), len(idx))
        if side == "long":
            cl = float(sig["close"]) + confirm_atr * atr
            fl = float(sig["close"]) - fail_atr * atr if fail_atr > 0 else -np.inf
            cp = None
            for p in range(start_pos, end_p):
                if float(lv[p]) <= fl: return None, balance, "early_reversal"
                if float(hv[p]) >= cl: cp = p; break
        else:
            cl = float(sig["close"]) - confirm_atr * atr
            fl = float(sig["close"]) + fail_atr * atr if fail_atr > 0 else np.inf
            cp = None
            for p in range(start_pos, end_p):
                if float(hv[p]) >= fl: return None, balance, "early_reversal"
                if float(lv[p]) <= cl: cp = p; break
        if cp is None: return None, balance, "confirm_timeout"
        # Order flow check at confirm point
        ok, reason = check_order_flow_imbalance(sb, cp, side, params)
        if not ok: return None, balance, reason
        start_pos = cp

    entry_time = idx[start_pos]
    entry_raw = float(cv[start_pos])
    if side == "long":
        if entry_raw - float(sig["close"]) > float(params["max_entry_extension_atr"]) * atr:
            return None, balance, "entry_extension"
    else:
        if float(sig["close"]) - entry_raw > float(params["max_entry_extension_atr"]) * atr:
            return None, balance, "entry_extension"
    position = base.open_position(sig, side, entry_raw, entry_time, balance, params)
    if position is None: return None, balance, "min_stop"
    logs = []
    base.append_entry(logs, position, balance)
    max_hold = entry_time + pd.Timedelta(hours=int(params["max_hold_bars"]))
    ep = min(int(idx.searchsorted(max_hold, side="left")), len(idx)-1)
    for p in range(start_pos+1, ep+1):
        bt, h, l, c = idx[p], float(hv[p]), float(lv[p]), float(cv[p])
        base.update_excursion(position, h, l, params)
        trig, re, reason = base.stop_trigger(position, h, l)
        if trig:
            balance = base.append_exit(logs, position, raw_exit=re, time_value=bt, reason=reason, balance=balance, params=params)
            return logs, balance, ""
        ev = events_by_end.get(bt)
        if ev is not None and bt > entry_time:
            ex, er = base.apply_hourly_event(position, ev, params)
            if ex:
                balance = base.append_exit(logs, position, raw_exit=c, time_value=bt, reason=er, balance=balance, params=params)
                return logs, balance, ""
        if bt >= max_hold:
            balance = base.append_exit(logs, position, raw_exit=c, time_value=bt, reason="MaxHoldExit", balance=balance, params=params)
            return logs, balance, ""
    balance = base.append_exit(logs, position, raw_exit=float(cv[ep]), time_value=idx[ep], reason="FinalMTM", balance=balance, params=params)
    return logs, balance, ""

# ---------------------------------------------------------------------------
# Strategy runner
# ---------------------------------------------------------------------------
def run_strategy(sb, signal, params, *, initial_balance):
    idx = sb.index
    events_by_end = {pd.Timestamp(r["bar_end"]): r for _, r in signal.iterrows()}
    balance = float(initial_balance)
    logs, last_exit = [], pd.Timestamp.min.tz_localize("UTC")
    diag = {"candidates":0,"entries":0,"busy":0,"confirm_timeout":0,"early_reversal":0,
            "imbalance_reject":0,"large_trade_reject":0,"no_volume":0,
            "entry_extension":0,"min_stop":0}
    for _, sig in signal.iterrows():
        side = base.signal_side(sig, params)
        if not side: continue
        diag["candidates"] += 1
        bar_end = pd.Timestamp(sig["bar_end"])
        if bar_end <= last_exit: diag["busy"] += 1; continue
        sp = int(idx.searchsorted(bar_end, side="left"))
        if sp >= len(idx): continue
        result, balance, skip = simulate_position(sb, events_by_end, sig, side, params, start_pos=sp, balance=balance)
        if result is None:
            key = skip.replace("_skipped","")
            if key in diag: diag[key] += 1
            continue
        diag["entries"] += 1
        logs.extend(result)
        last_exit = pd.Timestamp(result[-1]["time"])
    ledger = pd.DataFrame(logs)
    summary = base.summarize_ledger(ledger, initial_balance, params) if not ledger.empty else {
        "trades":0,"raw_no_fee_no_slip_return_pct":0.0,
        "price_pnl_with_2bps_slip_no_fee_return_pct":0.0,
        "realistic_return_pct":0.0,"win_rate_pct":0.0,"max_dd_pct":0.0}
    return {"summary": summary, "diagnostics": diag, "ledger": ledger}

# ---------------------------------------------------------------------------
# Variant parsing
# ---------------------------------------------------------------------------
DEFAULTS = {
    "break_lookback":8,"body_min":0.65,"close_top":0.75,
    "range_min_atr":0.80,"range_max_atr":2.20,"pre_range_atr":3.00,
    "max_atr_percentile":95.0,"max_entry_extension_atr":0.15,
    "confirm_atr":0.03,"confirm_window_seconds":120,"fail_retrace_atr":0.10,
    "imbalance_window_seconds":120,"min_buy_ratio":0.55,
    "large_trade_multiplier":3.0,"min_large_imbalance":0.0,
    "initial_stop_atr":0.45,"stop_buffer_atr":0.05,"stop_cap_atr":0.80,
    "min_stop_bps":12.0,"breakeven_at_r":1.0,"cost_lock_bps":10.0,
    "trail_start_r":1.5,"trail_buffer_atr":0.05,"max_hold_bars":6,
    "notional_share":0.20,"slippage":0.0002,"entry_fee":0.0002,"exit_fee":0.0004,
}
PRESETS = {
    "of_r55":{"min_buy_ratio":0.55},
    "of_r60":{"min_buy_ratio":0.60},
    "of_r65":{"min_buy_ratio":0.65},
    "of_r55_w60":{"min_buy_ratio":0.55,"imbalance_window_seconds":60},
    "of_r55_w300":{"min_buy_ratio":0.55,"imbalance_window_seconds":300},
    "of_r60_large":{"min_buy_ratio":0.60,"min_large_imbalance":0.55},
    "of_r55_large5x":{"min_buy_ratio":0.55,"large_trade_multiplier":5.0,"min_large_imbalance":0.55},
    "of_r60_c05":{"min_buy_ratio":0.60,"confirm_atr":0.05,"fail_retrace_atr":0.15},
    "of_noconfirm_r55":{"min_buy_ratio":0.55,"confirm_atr":0.0,"confirm_window_seconds":0},
    "of_noconfirm_r60":{"min_buy_ratio":0.60,"confirm_atr":0.0,"confirm_window_seconds":0},
}

def parse_variant(raw):
    params = dict(DEFAULTS)
    if raw in PRESETS: params.update(PRESETS[raw]); return raw, params
    name = raw.split("=",1)[0] if "=" in raw else raw
    if name in PRESETS: params.update(PRESETS[name])
    for k in ("break_lookback","max_hold_bars","confirm_window_seconds","imbalance_window_seconds"):
        params[k] = int(params[k])
    return name, params

# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------
def parse_args():
    p = argparse.ArgumentParser(description="Order flow imbalance breakout 1s replay")
    p.add_argument("--symbols", nargs="+", default=["BTCUSDT","ETHUSDT"])
    p.add_argument("--start", default="2026-01-01T00:00:00Z")
    p.add_argument("--end", default="2026-04-30T23:59:59Z")
    p.add_argument("--archive-root", default=str(DEFAULT_ARCHIVE_ROOT))
    p.add_argument("--signal-timeframe", default="1h")
    p.add_argument("--trend-timeframe", default="4h")
    p.add_argument("--chunksize", type=int, default=2_000_000)
    p.add_argument("--initial-balance", type=float, default=100000.0)
    p.add_argument("--variants", nargs="+",
        default=["of_r55","of_r60","of_r65","of_r55_w60","of_r55_w300","of_r60_large","of_r60_c05","of_noconfirm_r55","of_noconfirm_r60"])
    p.add_argument("--summary-json", default="research/order_flow_imbalance_breakout_summary.json")
    p.add_argument("--markdown", default="research/20260508_order_flow_imbalance_breakout.md")
    p.add_argument("--ledger-prefix", default="research/tmp_order_flow_imbalance")
    return p.parse_args()

def main():
    args = parse_args()
    started = time.time()
    start, end = base._as_utc(args.start), base._as_utc(args.end)
    variants = [parse_variant(r) for r in args.variants]
    results = []
    for symbol in args.symbols:
        tick_files = base.monthly_trade_files(symbol, start, end, Path(args.archive_root))
        print(f"\n{symbol}: building second bars with order flow...", flush=True)
        sb, bstats = build_second_bars_with_flow(tick_files, start, end, args.chunksize)
        # Also need plain second bars for signal frame building
        plain_sb = sb[["open","high","low","close","volume"]].copy()
        _, signal, _ = base.build_frames(plain_sb, signal_timeframe=args.signal_timeframe, trend_timeframe=args.trend_timeframe)
        print(f"{symbol}: second_rows={len(sb)} signal_rows={len(signal)}", flush=True)
        for vn, params in variants:
            print(f"running {symbol} {vn}", flush=True)
            result = run_strategy(sb, signal, params, initial_balance=args.initial_balance)
            lp = Path(f"{args.ledger_prefix}_{symbol}_{vn}_ledger.csv")
            result["ledger"].to_csv(lp, index=False); del result["ledger"]
            result.update({"symbol":symbol,"variant":vn,"params":params,"ledger_path":str(lp),"build_stats":bstats})
            s = result["summary"]
            print(f"{symbol} {vn}: realistic={s['realistic_return_pct']:.4f}% raw={s['raw_no_fee_no_slip_return_pct']:.4f}% trades={s['trades']} diag={result['diagnostics']}", flush=True)
            results.append(result)
    sp, mp = Path(args.summary_json), Path(args.markdown)
    summary = {"start":start.isoformat(),"end":end.isoformat(),
        "execution":"continuous 1s OHLC+flow bars from local trade ticks",
        "accounting":"2bps/side slippage plus maker entry 2bps and market exit 4bps",
        "variants":[{"name":n,"params":p} for n,p in variants],
        "results":results,"summary_path":str(sp),"elapsed_seconds":round(time.time()-started,2)}
    sp.write_text(json.dumps(summary, indent=2, ensure_ascii=False), encoding="utf-8")
    # Markdown
    lines = [f"# Order Flow Imbalance Breakout ({summary['start']} to {summary['end']})","",
        "| Symbol | Variant | Trades | Realistic | Raw | Win | DD | Diag |",
        "|---|---|---:|---:|---:|---:|---:|---|"]
    for r in results:
        s = r["summary"]
        lines.append(f"| `{r['symbol']}` | `{r['variant']}` | {s['trades']} | {s['realistic_return_pct']:.4f}% | {s['raw_no_fee_no_slip_return_pct']:.4f}% | {s['win_rate_pct']:.2f}% | {s['max_dd_pct']:.4f}% | `{r['diagnostics']}` |")
    mp.write_text("\n".join(lines), encoding="utf-8")
    print(json.dumps({"summary_path":str(sp),"elapsed":summary["elapsed_seconds"]}, indent=2), flush=True)

if __name__ == "__main__":
    main()
