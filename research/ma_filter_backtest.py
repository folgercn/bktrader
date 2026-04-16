import argparse
import json
from typing import Optional

import pandas as pd

from backTest import (
    _normalize_reentry_sizes,
    _reentry_triggered,
    _resolve_reentry_price,
    _resolve_stop_price,
    apply_breakout_levels,
    apply_reentry_anchor_levels,
    generate_1d_signals,
)


def compute_adx(df_signal: pd.DataFrame, period: int) -> pd.Series:
    high = df_signal["high"].astype(float)
    low = df_signal["low"].astype(float)
    close = df_signal["close"].astype(float)

    up_move = high.diff()
    down_move = -low.diff()
    plus_dm = up_move.where((up_move > down_move) & (up_move > 0), 0.0)
    minus_dm = down_move.where((down_move > up_move) & (down_move > 0), 0.0)

    tr_components = pd.concat(
        [
            (high - low),
            (high - close.shift(1)).abs(),
            (low - close.shift(1)).abs(),
        ],
        axis=1,
    )
    true_range = tr_components.max(axis=1)

    atr = true_range.ewm(alpha=1 / period, adjust=False, min_periods=period).mean()
    plus_di = 100 * plus_dm.ewm(alpha=1 / period, adjust=False, min_periods=period).mean() / atr
    minus_di = 100 * minus_dm.ewm(alpha=1 / period, adjust=False, min_periods=period).mean() / atr
    dx = (100 * (plus_di - minus_di).abs() / (plus_di + minus_di)).fillna(0.0)
    return dx.ewm(alpha=1 / period, adjust=False, min_periods=period).mean()


def ensure_filter_indicator(df_signal: pd.DataFrame, filter_kind: str, period: int, adx_period: int) -> pd.DataFrame:
    period = max(1, int(period))
    prepared = df_signal.copy()
    kind = filter_kind.lower().strip()

    if kind == "sma":
        indicator_col = f"sma{period}"
        prepared[indicator_col] = prepared["close"].rolling(window=period).mean()
    elif kind == "ema":
        indicator_col = f"ema{period}"
        prepared[indicator_col] = prepared["close"].ewm(span=period, adjust=False).mean()
    else:
        raise ValueError(f"unsupported filter kind: {filter_kind}")

    prepared["filter_indicator"] = prepared[indicator_col]
    prepared["filter_indicator_prev"] = prepared["filter_indicator"].shift(1)
    prepared["filter_indicator_kind"] = kind
    prepared["filter_indicator_period"] = period
    prepared["adx"] = compute_adx(prepared, max(2, int(adx_period)))
    return prepared


def directional_filter_allows_side(sig: pd.Series, side: str, *, mode: str, band_atr: float) -> bool:
    close_price = float(sig["close"])
    filter_value = float(sig["filter_indicator"])
    filter_prev = float(sig["filter_indicator_prev"]) if not pd.isna(sig.get("filter_indicator_prev")) else filter_value
    atr_value = float(sig["atr"]) if not pd.isna(sig["atr"]) else 0.0
    band = max(0.0, band_atr) * atr_value

    if mode == "hard":
        return close_price > filter_value if side == "long" else close_price < filter_value
    if mode == "slope":
        return filter_value >= filter_prev if side == "long" else filter_value <= filter_prev
    if mode == "band":
        return close_price >= (filter_value - band) if side == "long" else close_price <= (filter_value + band)
    if mode == "slope-band":
        if side == "long":
            return close_price >= (filter_value - band) and filter_value >= filter_prev
        return close_price <= (filter_value + band) and filter_value <= filter_prev
    raise ValueError(f"unsupported ma filter mode: {mode}")


def run_ma_filter_backtest(
    df_1min: pd.DataFrame,
    df_signal: pd.DataFrame,
    *,
    initial_balance: float = 100000.0,
    fixed_slippage: float = 0.0005,
    stop_loss_atr: float = 0.05,
    stop_mode: str = "atr",
    max_trades_per_bar: int = 3,
    reentry_size_schedule=None,
    profit_protect_atr: float = 1.0,
    dir2_zero_initial: bool = True,
    enable_directional_filter: bool = False,
    directional_filter_mode: str = "hard",
    directional_band_atr: float = 0.3,
    enable_adx_filter: bool = False,
    adx_threshold: float = 20.0,
    enable_early_reversal_gate: bool = False,
    early_reversal_band_atr: float = 0.15,
    reentry_anchor_levels: str = "wick",
    reentry_trigger_mode: str = "reclaim",
) -> pd.DataFrame:
    df_signal = apply_reentry_anchor_levels(df_signal, reentry_anchor_levels)
    balance = initial_balance
    position = None
    trade_logs = []

    reentry_atr = 0.1
    commission = 0.0010
    cash_usage_initial = 0.0 if dir2_zero_initial else 0.10
    reentry_sizes = _normalize_reentry_sizes(reentry_size_schedule)
    last_exit_bar_index = -999
    reentry_timeout = 1
    last_exit_reason = None
    last_exit_side = None

    print(
        "🚀 Regime filter回测 | "
        f"InitialSize:{cash_usage_initial:.0%} | "
        f"Stop:{stop_mode} | "
        f"MaxTrades:{max_trades_per_bar} | "
        f"ReentrySizes:{reentry_sizes} | "
        f"Slippage:{fixed_slippage} | "
        f"FilterKind:{str(df_signal['filter_indicator_kind'].iloc[0]) if 'filter_indicator_kind' in df_signal.columns and not df_signal.empty else 'sma'} | "
        f"FilterPeriod:{int(df_signal['filter_indicator_period'].iloc[0]) if 'filter_indicator_period' in df_signal.columns and not df_signal.empty else 20} | "
        f"DirectionalEnabled:{enable_directional_filter} | "
        f"DirectionalMode:{directional_filter_mode} | "
        f"DirectionalBandATR:{directional_band_atr} | "
        f"ADXEnabled:{enable_adx_filter} | "
        f"ADXThreshold:{adx_threshold} | "
        f"EarlyReversalEnabled:{enable_early_reversal_gate} | "
        f"EarlyReversalBandATR:{early_reversal_band_atr} | "
        f"ReentryAnchor:{reentry_anchor_levels} | "
        f"ReentryTrigger:{reentry_trigger_mode}"
    )

    for i in range(len(df_signal) - 1):
        start_t, end_t = df_signal.index[i], df_signal.index[i + 1]
        window = df_1min.loc[start_t:end_t]
        if window.empty:
            continue

        sig = df_signal.iloc[i]
        if pd.isna(sig["atr"]):
            continue

        trades_in_bar = 0
        current_idx = 0
        if i - last_exit_bar_index > reentry_timeout:
            last_exit_side = None

        while current_idx < len(window):
            bar = window.iloc[current_idx]
            bar_time = window.index[current_idx]
            prev_bar = window.iloc[current_idx - 1] if current_idx > 0 else None

            if not position:
                long_allowed = True
                short_allowed = True
                if enable_directional_filter:
                    long_allowed = directional_filter_allows_side(
                        sig,
                        "long",
                        mode=directional_filter_mode,
                        band_atr=directional_band_atr,
                    )
                    short_allowed = directional_filter_allows_side(
                        sig,
                        "short",
                        mode=directional_filter_mode,
                        band_atr=directional_band_atr,
                    )
                if enable_adx_filter:
                    adx_ok = (not pd.isna(sig["adx"])) and float(sig["adx"]) >= float(adx_threshold)
                    long_allowed = long_allowed and adx_ok
                    short_allowed = short_allowed and adx_ok

                close_price = float(sig["close"])
                filter_value = float(sig["filter_indicator"])
                atr_value = float(sig["atr"]) if not pd.isna(sig["atr"]) else 0.0
                early_band = max(0.0, early_reversal_band_atr) * atr_value
                long_early_reversal = (
                    enable_early_reversal_gate
                    and (not long_allowed)
                    and close_price >= (filter_value - early_band)
                    and float(sig["prev_high_2"]) > float(sig["prev_high_1"])
                    and float(sig["prev_low_1"]) >= float(sig["prev_low_2"])
                )
                short_early_reversal = (
                    enable_early_reversal_gate
                    and (not short_allowed)
                    and close_price <= (filter_value + early_band)
                    and float(sig["prev_low_2"]) < float(sig["prev_low_1"])
                    and float(sig["prev_high_1"]) <= float(sig["prev_high_2"])
                )

                if long_allowed or long_early_reversal:
                    re_p = _resolve_reentry_price(sig, "long", reentry_anchor_levels, reentry_atr)
                    if trades_in_bar == 0 and sig["prev_high_2"] > sig["prev_high_1"] and bar["high"] >= sig["prev_high_2"]:
                        entry = max(bar["open"], sig["prev_high_2"]) * (1 + fixed_slippage)
                        notional = balance * cash_usage_initial
                        position = {
                            "side": "long",
                            "entry_p": entry,
                            "sl": _resolve_stop_price("long", entry, sig, stop_mode, stop_loss_atr),
                            "protected": False,
                            "notional": notional,
                        }
                        balance -= notional * commission
                        trade_logs.append(
                            {"time": bar_time, "type": "BUY", "price": entry, "reason": "Initial", "notional": notional, "bal": balance}
                        )
                        trades_in_bar += 1
                        current_idx += 1
                        continue

                    prev_close = prev_bar["close"] if prev_bar is not None else None
                    is_reentry_triggered, entry_p_raw = _reentry_triggered(
                        "long",
                        reentry_trigger_mode,
                        bar["high"],
                        bar["low"],
                        bar["close"],
                        prev_close,
                        re_p,
                    )
                    if last_exit_side == "long" and is_reentry_triggered and (i - last_exit_bar_index <= reentry_timeout):
                        reason = "SL-Reentry" if last_exit_reason == "SL" else "PT-Reentry"
                        if (reason == "SL-Reentry" and trades_in_bar < max_trades_per_bar) or reason == "PT-Reentry":
                            size_index = trades_in_bar
                            reentry_size = reentry_sizes[min(size_index, len(reentry_sizes) - 1)]
                            notional = balance * reentry_size
                            entry = entry_p_raw * (1 + fixed_slippage)
                            position = {
                                "side": "long",
                                "entry_p": entry,
                                "sl": _resolve_stop_price("long", entry, sig, stop_mode, stop_loss_atr),
                                "protected": reason == "PT-Reentry",
                                "notional": notional,
                            }
                            balance -= notional * commission
                            trade_logs.append(
                                {"time": bar_time, "type": "BUY", "price": entry, "reason": reason, "notional": notional, "bal": balance}
                            )
                            if reason == "SL-Reentry":
                                trades_in_bar += 1
                        last_exit_side = None
                        current_idx += 1
                        continue

                if short_allowed or short_early_reversal:
                    re_p = _resolve_reentry_price(sig, "short", reentry_anchor_levels, 0.0)
                    if trades_in_bar == 0 and sig["prev_low_2"] < sig["prev_low_1"] and bar["low"] <= sig["prev_low_2"]:
                        entry = min(bar["open"], sig["prev_low_2"]) * (1 - fixed_slippage)
                        notional = balance * cash_usage_initial
                        position = {
                            "side": "short",
                            "entry_p": entry,
                            "sl": _resolve_stop_price("short", entry, sig, stop_mode, stop_loss_atr),
                            "protected": False,
                            "notional": notional,
                        }
                        balance -= notional * commission
                        trade_logs.append(
                            {"time": bar_time, "type": "SHORT", "price": entry, "reason": "Initial", "notional": notional, "bal": balance}
                        )
                        trades_in_bar += 1
                        current_idx += 1
                        continue

                    prev_close = prev_bar["close"] if prev_bar is not None else None
                    is_reentry_triggered, entry_p_raw = _reentry_triggered(
                        "short",
                        reentry_trigger_mode,
                        bar["high"],
                        bar["low"],
                        bar["close"],
                        prev_close,
                        re_p,
                    )
                    if last_exit_side == "short" and is_reentry_triggered and (i - last_exit_bar_index <= reentry_timeout):
                        reason = "SL-Reentry" if last_exit_reason == "SL" else "PT-Reentry"
                        if (reason == "SL-Reentry" and trades_in_bar < max_trades_per_bar) or reason == "PT-Reentry":
                            size_index = trades_in_bar
                            reentry_size = reentry_sizes[min(size_index, len(reentry_sizes) - 1)]
                            notional = balance * reentry_size
                            entry = entry_p_raw * (1 - fixed_slippage)
                            position = {
                                "side": "short",
                                "entry_p": entry,
                                "sl": _resolve_stop_price("short", entry, sig, stop_mode, stop_loss_atr),
                                "protected": reason == "PT-Reentry",
                                "notional": notional,
                            }
                            balance -= notional * commission
                            trade_logs.append(
                                {"time": bar_time, "type": "SHORT", "price": entry, "reason": reason, "notional": notional, "bal": balance}
                            )
                            if reason == "SL-Reentry":
                                trades_in_bar += 1
                        last_exit_side = None
                        current_idx += 1
                        continue

                current_idx += 1
                continue

            exit_triggered = False
            if position["side"] == "long":
                if not position["protected"] and bar["high"] >= position["entry_p"] + profit_protect_atr * sig["atr"]:
                    position["protected"] = True
                if bar["low"] <= position["sl"]:
                    exit_p, reason, exit_triggered = position["sl"], "SL", True
                elif position["protected"] and bar["low"] <= sig["prev_low_1"]:
                    exit_p, reason, exit_triggered = sig["prev_low_1"], "PT", True
            else:
                if not position["protected"] and bar["low"] <= position["entry_p"] - profit_protect_atr * sig["atr"]:
                    position["protected"] = True
                if bar["high"] >= position["sl"]:
                    exit_p, reason, exit_triggered = position["sl"], "SL", True
                elif position["protected"] and bar["high"] >= sig["prev_high_1"]:
                    exit_p, reason, exit_triggered = sig["prev_high_1"], "PT", True

            if exit_triggered:
                side_mult = 1 if position["side"] == "long" else -1
                exit_p = exit_p * (1 - fixed_slippage) if position["side"] == "long" else exit_p * (1 + fixed_slippage)
                pnl = 0.0
                if position["notional"] > 0:
                    pnl = side_mult * (exit_p - position["entry_p"]) / position["entry_p"] * position["notional"]
                    balance += pnl - position["notional"] * commission
                trade_logs.append(
                    {"time": bar_time, "type": "EXIT", "price": exit_p, "reason": reason, "notional": position["notional"], "bal": balance}
                )
                last_exit_reason = reason
                last_exit_side = position["side"]
                last_exit_bar_index = i
                position = None

            current_idx += 1

    return pd.DataFrame(trade_logs)


def summarize_ledger(ledger: pd.DataFrame, initial_balance: float) -> dict:
    if ledger.empty:
        return {"final_balance": initial_balance, "return": 0.0, "max_drawdown": 0.0, "trade_pairs": 0, "entry_reasons": {}}
    result = ledger.copy()
    result["cum_max"] = result["bal"].cummax()
    result["drawdown"] = result["bal"] / result["cum_max"] - 1
    entries = result[result["type"].isin(["BUY", "SHORT"])]
    return {
        "final_balance": float(result.iloc[-1]["bal"]),
        "return": float(result.iloc[-1]["bal"] / initial_balance - 1),
        "max_drawdown": float(result["drawdown"].min()),
        "trade_pairs": int((result["type"] == "EXIT").sum()),
        "entry_reasons": {str(k): int(v) for k, v in entries["reason"].value_counts().sort_index().items()},
    }


def load_signal_frame(df_1min: pd.DataFrame, timeframe: str, signal_csv: Optional[str], breakout_levels: str) -> pd.DataFrame:
    timeframe = timeframe.lower().strip()
    if timeframe == "1d":
        return generate_1d_signals(df_1min, breakout_levels=breakout_levels)
    if signal_csv:
        df_signal = pd.read_csv(signal_csv, index_col=0, parse_dates=True)
    else:
        df_signal = pd.read_csv("BTC_4H_Signals.csv", index_col=0, parse_dates=True)
    return df_signal if breakout_levels == "wick" else apply_breakout_levels(df_signal, breakout_levels)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Regime filter research backtest")
    parser.add_argument("--one-min-csv", default="BTC_1min_Clean.csv")
    parser.add_argument("--signal-csv", default="")
    parser.add_argument("--timeframe", choices=["4h", "1d"], default="1d")
    parser.add_argument("--breakout-levels", choices=["wick", "body"], default="wick")
    parser.add_argument("--start", default="2023-01-01")
    parser.add_argument("--end", default="2026-02-28")
    parser.add_argument("--initial-balance", type=float, default=100000.0)
    parser.add_argument("--fixed-slippage", type=float, default=0.0005)
    parser.add_argument("--stop-loss-atr", type=float, default=0.05)
    parser.add_argument("--stop-mode", default="atr")
    parser.add_argument("--max-trades-per-bar", type=int, default=3)
    parser.add_argument("--reentry-size-schedule", default="0.10,0.20")
    parser.add_argument("--reentry-anchor-levels", choices=["wick", "body"], default="wick")
    parser.add_argument("--reentry-trigger-mode", choices=["reclaim", "pullback"], default="reclaim")
    parser.add_argument("--profit-protect-atr", type=float, default=1.0)
    parser.add_argument("--disable-zero-initial", action="store_true")
    parser.add_argument("--filter-kind", choices=["sma", "ema"], default="ema")
    parser.add_argument("--filter-period", type=int, default=5)
    parser.add_argument("--enable-directional-filter", action="store_true")
    parser.add_argument("--directional-filter-mode", choices=["hard", "slope", "band", "slope-band"], default="hard")
    parser.add_argument("--directional-band-atr", type=float, default=0.3)
    parser.add_argument("--adx-period", type=int, default=14)
    parser.add_argument("--enable-adx-filter", action="store_true")
    parser.add_argument("--adx-threshold", type=float, default=20.0)
    parser.add_argument("--enable-early-reversal-gate", action="store_true")
    parser.add_argument("--early-reversal-band-atr", type=float, default=0.15)
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    reentry_sizes = [float(item) for item in args.reentry_size_schedule.split(",") if item.strip()]

    df_1min = pd.read_csv(args.one_min_csv, index_col=0, parse_dates=True).loc[args.start : args.end]
    df_signal = load_signal_frame(df_1min, args.timeframe, args.signal_csv or None, args.breakout_levels).loc[args.start : args.end]
    df_signal = ensure_filter_indicator(df_signal, args.filter_kind, args.filter_period, args.adx_period)

    ledger = run_ma_filter_backtest(
        df_1min,
        df_signal,
        initial_balance=args.initial_balance,
        fixed_slippage=args.fixed_slippage,
        stop_loss_atr=args.stop_loss_atr,
        stop_mode=args.stop_mode,
        max_trades_per_bar=args.max_trades_per_bar,
        reentry_size_schedule=reentry_sizes,
        profit_protect_atr=args.profit_protect_atr,
        dir2_zero_initial=not args.disable_zero_initial,
        enable_directional_filter=args.enable_directional_filter,
        directional_filter_mode=args.directional_filter_mode,
        directional_band_atr=args.directional_band_atr,
        enable_adx_filter=args.enable_adx_filter,
        adx_threshold=args.adx_threshold,
        enable_early_reversal_gate=args.enable_early_reversal_gate,
        early_reversal_band_atr=args.early_reversal_band_atr,
        reentry_anchor_levels=args.reentry_anchor_levels,
        reentry_trigger_mode=args.reentry_trigger_mode,
    )

    result = {
        "timeframe": args.timeframe,
        "breakout_levels": args.breakout_levels,
        "reentry_anchor_levels": args.reentry_anchor_levels,
        "reentry_trigger_mode": args.reentry_trigger_mode,
        "window": {"start": args.start, "end": args.end},
        "filter_kind": args.filter_kind,
        "filter_period": args.filter_period,
        "directional_filter_mode": args.directional_filter_mode,
        "adx_period": args.adx_period,
        "adx_threshold": args.adx_threshold,
        "early_reversal_band_atr": args.early_reversal_band_atr,
        "summary": summarize_ledger(ledger, args.initial_balance),
    }
    print(json.dumps(result, indent=2, ensure_ascii=False))


if __name__ == "__main__":
    main()
