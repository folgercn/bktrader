#!/usr/bin/env python3
"""Probabilistic Baseline Runner V3.

Implements the Full-Spectrum Probabilistic Framework with parameter sweeps:
1. Pre-touch Anticipatory Entry: reduced to 0.2 ATR distance, threshold LLR > 4.0.
2. Configurable Cooldown & Max Trades per Bar.
3. Configurable Fixed SL.
4. Trailing SL & Minute-level TP.
"""

from __future__ import annotations

import argparse
import json
import time
from pathlib import Path
from collections import defaultdict

import numpy as np
import pandas as pd

import eth_q1_breakout_t3_shape_compare as replay
import btc_eth_2026_jan_apr_impulse_bar_run as base
import order_flow_imbalance_breakout as of_base
import markov_order_flow_scoring as markov

DEFAULT_ARCHIVE_ROOT = Path("dataset/archive")

M_WIN_BTC = np.array([[0.787, 0.072, 0.085, 0.056], [0.262, 0.315, 0.174, 0.248], [0.265, 0.172, 0.331, 0.232], [0.111, 0.133, 0.111, 0.645]])
M_LOSS_BTC = np.array([[0.524, 0.125, 0.212, 0.139], [0.242, 0.289, 0.223, 0.246], [0.270, 0.143, 0.363, 0.224], [0.161, 0.242, 0.168, 0.429]])
M_WIN_ETH = np.array([[0.690, 0.085, 0.128, 0.098], [0.188, 0.318, 0.267, 0.227], [0.264, 0.191, 0.331, 0.213], [0.180, 0.225, 0.126, 0.468]])
M_LOSS_ETH = np.array([[0.588, 0.158, 0.164, 0.090], [0.299, 0.337, 0.120, 0.245], [0.306, 0.127, 0.353, 0.214], [0.136, 0.155, 0.121, 0.589]])

def get_matrices(symbol: str):
    if "BTC" in symbol:
        return M_WIN_BTC, M_LOSS_BTC
    return M_WIN_ETH, M_LOSS_ETH

def get_regime_multiplier(regime_name: str) -> float:
    if not isinstance(regime_name, str): return 0.5
    if "Bullish_MidVol" in regime_name: return 1.0
    elif "Bearish_HighVol" in regime_name: return 0.8
    elif "Bearish_MidVol" in regime_name: return 0.3
    elif "Ranging_LowVol" in regime_name: return 0.0
    return 0.5

def run_probabilistic_matrix(symbol: str, start: pd.Timestamp, end: pd.Timestamp, archive_root: Path, sl_atrs: list[float], cooldowns: list[int], max_trades: int):
    print(f"\n[{symbol}] Loading ticks & states for matrix run...")
    regime_path = Path(f"research/hmm_regime_{symbol}_test_timeseries.csv")
    if regime_path.exists():
        regime_df = pd.read_csv(regime_path, index_col=0, parse_dates=True)
        if regime_df.index.tz is None: regime_df.index = regime_df.index.tz_localize("UTC")
        else: regime_df.index = regime_df.index.tz_convert("UTC")
        regime_series = regime_df.set_index(regime_df.index.normalize())["hmm_regime"]
    else:
        regime_series = pd.Series(dtype=str)

    m_win, m_loss = get_matrices(symbol)

    tick_files = base.monthly_trade_files(symbol, start, end, archive_root)
    sb, _ = of_base.build_second_bars_with_flow(tick_files, start, end, chunksize=5_000_000)
    states_series = markov.discretize_order_flow(sb)
    
    one_min = sb.resample("1min").agg({
        "open": "first", "high": "max", "low": "min", "close": "last", "volume": "sum",
        "buy_volume": "sum", "sell_volume": "sum"
    }).dropna()
    one_min["is_buy"] = one_min["buy_volume"] > one_min["sell_volume"]
    
    signal_df = one_min.resample("1h").agg({
        "open": "first", "high": "max", "low": "min", "close": "last", "volume": "sum"
    }).dropna()
    
    if len(signal_df) < 50: return {}

    prev_close = signal_df["close"].shift(1)
    tr = pd.concat([
        signal_df["high"] - signal_df["low"],
        (signal_df["high"] - prev_close).abs(),
        (signal_df["low"] - prev_close).abs()
    ], axis=1).max(axis=1)
    signal_df["atr"] = tr.rolling(14).mean()
    
    signal_df["prev_high_1"] = signal_df["high"].shift(1)
    signal_df["prev_high_2"] = signal_df["high"].shift(2)
    
    t2_ready = (signal_df["prev_high_2"] > signal_df["prev_high_1"])
    candidates = signal_df[t2_ready].copy()

    all_results = []

    for sl_atr in sl_atrs:
        for cd_secs in cooldowns:
            print(f"  -> Running SL={sl_atr}, Cooldown={cd_secs}s")
            results = {
                "sl_atr": sl_atr, "cooldown": cd_secs,
                "trades": 0, "wins": 0, "losses": 0, "total_pnl": 0.0,
                "pre_touch_entries": 0, "breakout_entries": 0,
                "stop_exits": 0, "trailing_exits": 0, "smart_tp_exits": 0, "timeout_exits": 0,
                "volume_traded": 0.0, "diag": defaultdict(int)
            }
            
            next_allowed_entry_time = start

            for dt, row in candidates.iterrows():
                target_price = float(row["prev_high_2"])
                end_time = dt + pd.Timedelta(hours=1)
                bar_ticks = sb.loc[dt:end_time - pd.Timedelta(milliseconds=1)]
                if bar_ticks.empty: continue
                    
                atr = float(row["atr"])
                if pd.isna(atr) or atr == 0: continue
                    
                pre_touch_level = target_price - (0.2 * atr)
                
                trades_in_bar = 0
                search_start_idx = 0
                
                while trades_in_bar < max_trades:
                    if search_start_idx >= len(bar_ticks): break
                    if not bar_ticks.empty and bar_ticks.index[search_start_idx] < next_allowed_entry_time:
                        search_start_idx = bar_ticks.index.searchsorted(next_allowed_entry_time)
                    if search_start_idx >= len(bar_ticks): break
                        
                    entered = False
                    entry_price = 0.0
                    entry_time = None
                    entry_type = ""
                    
                    for i in range(search_start_idx, len(bar_ticks)):
                        s_dt = bar_ticks.index[i]
                        s_row = bar_ticks.iloc[i]
                        curr_high = float(s_row["high"])
                        
                        if not entered and curr_high >= pre_touch_level:
                            seq = states_series.loc[s_dt - pd.Timedelta(seconds=60) : s_dt].tolist()
                            llr = markov.score_sequence(seq, m_win, m_loss)
                            if llr > 4.0:
                                entered = True
                                entry_price = max(pre_touch_level, float(s_row["open"]))
                                entry_time = s_dt
                                entry_type = "Pre-Touch"
                                results["pre_touch_entries"] += 1
                                
                        if not entered and curr_high >= target_price:
                            seq = states_series.loc[s_dt - pd.Timedelta(seconds=60) : s_dt].tolist()
                            llr = markov.score_sequence(seq, m_win, m_loss)
                            if llr > 0.0:
                                entered = True
                                entry_price = max(target_price, float(s_row["open"]))
                                entry_time = s_dt
                                entry_type = "Breakout"
                                results["breakout_entries"] += 1
                            else:
                                results["diag"]["rejected_breakouts"] += 1
                                
                        if entered: break
                            
                    if not entered: break
                        
                    trade_date = dt.normalize()
                    if not regime_series.empty:
                        ridx = regime_series.index.get_indexer([trade_date], method="pad")[0]
                        regime = regime_series.iloc[ridx] if ridx >= 0 else "Unknown"
                    else:
                        regime = "Unknown"
                        
                    regime_mult = get_regime_multiplier(regime)
                    if regime_mult == 0.0:
                        results["diag"]["rejected_by_regime"] += 1
                        search_start_idx = i + 1
                        continue
                        
                    scale_factor = 1.0 if trades_in_bar == 0 else 0.5
                    position_size = 1000.0 * regime_mult * scale_factor
                    
                    results["volume_traded"] += position_size
                    results["trades"] += 1
                    trades_in_bar += 1
                    
                    exit_limit = entry_time + pd.Timedelta(hours=24)
                    holding_ticks = sb.loc[entry_time : exit_limit]
                    
                    exited = False
                    exit_price = 0.0
                    exit_type = ""
                    
                    current_sl = entry_price - (sl_atr * atr)
                    max_reached_profit_atr = 0.0
                    
                    for h_dt, h_row in holding_ticks.iterrows():
                        h_low = float(h_row["low"])
                        h_high = float(h_row["high"])
                        h_close = float(h_row["close"])
                        
                        profit_atr = (h_high - entry_price) / atr
                        if profit_atr > max_reached_profit_atr:
                            max_reached_profit_atr = profit_atr
                            
                        if max_reached_profit_atr >= 0.50 and current_sl < entry_price + (0.05 * atr):
                            current_sl = entry_price + (0.05 * atr)
                            
                        if max_reached_profit_atr >= 1.00:
                            current_min_floor = h_dt.floor("1min")
                            if current_min_floor in one_min.index:
                                if not one_min.loc[current_min_floor, "is_buy"]:
                                    exited = True
                                    exit_price = h_close
                                    exit_type = "Smart TP"
                                    results["smart_tp_exits"] += 1
                                    break

                        if h_low <= current_sl:
                            exited = True
                            exit_price = min(current_sl, float(h_row["open"]))
                            exit_type = "Trailing SL" if current_sl > entry_price else "Stop Loss"
                            if exit_type == "Stop Loss": results["stop_exits"] += 1
                            else: results["trailing_exits"] += 1
                            break
                                
                    if not exited:
                        if not holding_ticks.empty: exit_price = float(holding_ticks.iloc[-1]["close"])
                        else: exit_price = entry_price
                        exit_type = "Timeout"
                        results["timeout_exits"] += 1
                        
                    pnl_pct = (exit_price - entry_price) / entry_price * 100.0 - 0.1
                    abs_pnl = position_size * (pnl_pct / 100.0)
                    
                    results["total_pnl"] += abs_pnl
                    if abs_pnl > 0: results["wins"] += 1
                    else: results["losses"] += 1
                        
                    exit_time_val = h_dt if exited else exit_limit
                    next_allowed_entry_time = exit_time_val + pd.Timedelta(seconds=cd_secs)
                    search_start_idx = bar_ticks.index.searchsorted(next_allowed_entry_time)

            if results["trades"] > 0:
                results["win_rate"] = results["wins"] / results["trades"] * 100
                results["roi_pct"] = results["total_pnl"] / results["volume_traded"] * 100
            else:
                results["win_rate"] = 0
                results["roi_pct"] = 0
                
            all_results.append(results)

    return all_results

def main() -> None:
    parser = argparse.ArgumentParser(description="Probabilistic Baseline Matrix Runner")
    parser.add_argument("--symbols", nargs="+", default=["ETHUSDT", "BTCUSDT"])
    parser.add_argument("--start", default="2026-01-01T00:00:00Z")
    parser.add_argument("--end", default="2026-04-30T23:59:59Z")
    parser.add_argument("--archive-root", default=str(DEFAULT_ARCHIVE_ROOT))
    parser.add_argument("--sl-atrs", type=float, nargs="+", default=[0.10, 0.30, 0.50])
    parser.add_argument("--cooldowns", type=int, nargs="+", default=[0, 30, 60])
    parser.add_argument("--max-trades", type=int, default=2)
    parser.add_argument("--output-file", default="research/probabilistic_v3_matrix.json")
    args = parser.parse_args()
    
    start = base._as_utc(args.start)
    end = base._as_utc(args.end)
    
    res = {}
    for sym in args.symbols:
        res[sym] = run_probabilistic_matrix(sym, start, end, Path(args.archive_root), args.sl_atrs, args.cooldowns, args.max_trades)
        
    out = Path(args.output_file)
    data = {"args": vars(args), "results": res}
    
    with out.open("w") as f:
        json.dump(data, f, indent=2, ensure_ascii=False, default=str)
        
    print(f"\nMatrix Run Complete. Saved to {out}")

if __name__ == "__main__":
    main()
