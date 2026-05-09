#!/usr/bin/env python3
"""Probabilistic V4 execution runner.

Research-only. Consumes scored V4 events and a selected quality rule, then
runs a simple 1s execution simulation. The model layer never mutates execution
logic; this runner is the only place that touches balance and position state.
"""

from __future__ import annotations

import argparse
import json
from types import SimpleNamespace
import time
from pathlib import Path

import numpy as np
import pandas as pd

try:
    import btc_eth_2026_jan_apr_impulse_bar_run as base
except ModuleNotFoundError:
    def _fallback_as_utc(value: str) -> pd.Timestamp:
        ts = pd.Timestamp(value)
        if ts.tzinfo is None:
            return ts.tz_localize("UTC")
        return ts.tz_convert("UTC")

    def _fallback_monthly_trade_files(symbol: str, start: pd.Timestamp, end: pd.Timestamp, archive_root: Path) -> list[str]:
        months = pd.period_range(
            start=start.tz_convert(None).to_period("M"),
            end=end.tz_convert(None).to_period("M"),
            freq="M",
        )
        paths = []
        missing = []
        for month in months:
            ym = str(month)
            base_dir = archive_root / f"{symbol}-trades-{ym}"
            stem = f"{symbol}-trades-{ym}"
            zip_path = base_dir / f"{stem}.zip"
            csv_path = base_dir / f"{stem}.csv"
            if zip_path.exists():
                paths.append(str(zip_path))
            elif csv_path.exists():
                paths.append(str(csv_path))
            else:
                missing.append(str(zip_path))
        if missing:
            raise FileNotFoundError("missing monthly trade archives:\n" + "\n".join(missing))
        return paths

    def _fallback_open_position(sig: pd.Series, side: str, entry_raw: float, entry_time, balance: float, params: dict) -> dict | None:
        atr = float(sig["atr"])
        slippage = float(params["slippage"])
        entry_p = float(entry_raw) * (1.0 + slippage if side == "long" else 1.0 - slippage)
        if side == "long":
            raw_stop = min(float(sig["low"]) - params["stop_buffer_atr"] * atr, entry_p - params["initial_stop_atr"] * atr)
            capped_stop = entry_p - params["stop_cap_atr"] * atr
            stop = max(raw_stop, capped_stop)
            risk = entry_p - stop
        else:
            raw_stop = max(float(sig["high"]) + params["stop_buffer_atr"] * atr, entry_p + params["initial_stop_atr"] * atr)
            capped_stop = entry_p + params["stop_cap_atr"] * atr
            stop = min(raw_stop, capped_stop)
            risk = stop - entry_p
        if risk <= 0 or risk < entry_p * float(params["min_stop_bps"]) / 10000.0:
            return None
        return {
            "side": side,
            "entry_time": entry_time,
            "entry_p": entry_p,
            "entry_raw": float(entry_raw),
            "sl": stop,
            "initial_sl": stop,
            "risk": risk,
            "atr_at_entry": atr,
            "notional": balance * float(params["notional_share"]),
            "notional_share": float(params["notional_share"]),
            "signal_bar_time": sig.name,
            "signal_close": float(sig["close"]),
            "signal_high": float(sig["high"]),
            "signal_low": float(sig["low"]),
            "protected": False,
            "trailing_active": False,
            "hwm": entry_p,
            "lwm": entry_p,
            "mfe_r": 0.0,
            "mae_r": 0.0,
        }

    def _fallback_append_entry(logs: list[dict], position: dict, balance: float) -> None:
        logs.append(
            {
                "time": position["entry_time"],
                "type": "BUY" if position["side"] == "long" else "SHORT",
                "price": position["entry_p"],
                "raw_price": position["entry_raw"],
                "reason": "ProbV4-QualityEntry",
                "notional": position["notional"],
                "notional_share": position["notional_share"],
                "bal": balance,
                "signal_bar_time": position["signal_bar_time"],
                "signal_close": position["signal_close"],
                "risk": position["risk"],
                "mfe_r": np.nan,
                "mae_r": np.nan,
            }
        )

    def _fallback_append_exit(logs: list[dict], position: dict, *, raw_exit: float, time_value, reason: str, balance: float, params: dict) -> float:
        slippage = float(params["slippage"])
        side_mult = 1.0 if position["side"] == "long" else -1.0
        exit_p = float(raw_exit) * (1.0 - slippage if position["side"] == "long" else 1.0 + slippage)
        pnl_pct = side_mult * (exit_p - float(position["entry_p"])) / float(position["entry_p"])
        new_balance = balance + pnl_pct * float(position["notional"])
        logs.append(
            {
                "time": time_value,
                "type": "EXIT",
                "price": exit_p,
                "raw_price": float(raw_exit),
                "reason": reason,
                "notional": position["notional"],
                "notional_share": position["notional_share"],
                "bal": new_balance,
                "signal_bar_time": position["signal_bar_time"],
                "signal_close": position["signal_close"],
                "risk": position["risk"],
                "mfe_r": float(position["mfe_r"]),
                "mae_r": float(position["mae_r"]),
                "stop_price": float(position["sl"]),
            }
        )
        return new_balance

    def _fallback_update_excursion(position: dict, high_value: float, low_value: float, params: dict) -> None:
        entry = float(position["entry_p"])
        risk = float(position["risk"])
        if position["side"] == "long":
            position["hwm"] = max(float(position["hwm"]), float(high_value))
            favorable = max(0.0, float(position["hwm"]) - entry)
            adverse = max(0.0, entry - float(low_value))
            if favorable / risk >= float(params["breakeven_at_r"]):
                be_sl = entry * (1.0 + float(params["cost_lock_bps"]) / 10000.0)
                if be_sl > float(position["sl"]):
                    position["sl"] = be_sl
                    position["protected"] = True
        else:
            position["lwm"] = min(float(position["lwm"]), float(low_value))
            favorable = max(0.0, entry - float(position["lwm"]))
            adverse = max(0.0, float(high_value) - entry)
            if favorable / risk >= float(params["breakeven_at_r"]):
                be_sl = entry * (1.0 - float(params["cost_lock_bps"]) / 10000.0)
                if be_sl < float(position["sl"]):
                    position["sl"] = be_sl
                    position["protected"] = True
        position["mfe_r"] = max(float(position["mfe_r"]), favorable / risk)
        position["mae_r"] = max(float(position["mae_r"]), adverse / risk)

    def _fallback_stop_trigger(position: dict, high_value: float, low_value: float) -> tuple[bool, float, str]:
        if position["side"] == "long":
            if low_value <= float(position["sl"]):
                if position["trailing_active"]:
                    return True, float(position["sl"]), "TrailingSL"
                if position["protected"]:
                    return True, float(position["sl"]), "BreakevenSL"
                return True, float(position["sl"]), "InitialSL"
        else:
            if high_value >= float(position["sl"]):
                if position["trailing_active"]:
                    return True, float(position["sl"]), "TrailingSL"
                if position["protected"]:
                    return True, float(position["sl"]), "BreakevenSL"
                return True, float(position["sl"]), "InitialSL"
        return False, 0.0, ""

    def _fallback_paired_trades(ledger: pd.DataFrame) -> pd.DataFrame:
        rows = []
        entry = None
        if ledger.empty:
            return pd.DataFrame(rows)
        for _, row in ledger.iterrows():
            if row["type"] in {"BUY", "SHORT"}:
                entry = row
                continue
            if row["type"] != "EXIT" or entry is None:
                continue
            side_mult = 1.0 if entry["type"] == "BUY" else -1.0
            entry_price = float(entry["price"])
            exit_price = float(row["price"])
            raw_entry = float(entry["raw_price"])
            raw_exit = float(row["raw_price"])
            rows.append(
                {
                    "entry_time": entry["time"],
                    "exit_time": row["time"],
                    "side": entry["type"],
                    "exit_reason": row["reason"],
                    "slip_pnl_pct": side_mult * (exit_price - entry_price) / entry_price,
                    "raw_pnl_pct": side_mult * (raw_exit - raw_entry) / raw_entry,
                    "notional_share": float(entry["notional_share"]),
                    "hold_seconds": (pd.Timestamp(row["time"]) - pd.Timestamp(entry["time"])).total_seconds(),
                    "mfe_r": float(row.get("mfe_r", 0.0)),
                    "mae_r": float(row.get("mae_r", 0.0)),
                }
            )
            entry = None
        return pd.DataFrame(rows)

    def _fallback_summarize_ledger(ledger: pd.DataFrame, initial_balance: float, params: dict) -> dict:
        pairs = _fallback_paired_trades(ledger)
        if pairs.empty:
            return {
                "trades": 0,
                "raw_no_fee_no_slip_return_pct": 0.0,
                "price_pnl_with_2bps_slip_no_fee_return_pct": 0.0,
                "realistic_return_pct": 0.0,
                "win_rate_pct": 0.0,
                "max_dd_pct": 0.0,
                "exit_reasons": {},
            }
        raw_balance = float(initial_balance)
        slip_balance = float(initial_balance)
        realistic_balance = float(initial_balance)
        fee_rate = float(params["entry_fee"]) + float(params["exit_fee"])
        equity = [realistic_balance]
        for _, pair in pairs.iterrows():
            share = float(pair["notional_share"])
            raw_balance += raw_balance * share * float(pair["raw_pnl_pct"])
            slip_balance += slip_balance * share * float(pair["slip_pnl_pct"])
            realistic_notional = realistic_balance * share
            realistic_balance += realistic_notional * float(pair["slip_pnl_pct"]) - realistic_notional * fee_rate
            equity.append(realistic_balance)
        equity_arr = np.array(equity, dtype="float64")
        peak = np.maximum.accumulate(equity_arr)
        dd = equity_arr / peak - 1.0
        return {
            "trades": int(len(pairs)),
            "raw_no_fee_no_slip_return_pct": round((raw_balance / initial_balance - 1.0) * 100.0, 4),
            "price_pnl_with_2bps_slip_no_fee_return_pct": round((slip_balance / initial_balance - 1.0) * 100.0, 4),
            "realistic_return_pct": round((realistic_balance / initial_balance - 1.0) * 100.0, 4),
            "win_rate_pct": round(float((pairs["slip_pnl_pct"] > 0).mean()) * 100.0, 2),
            "max_dd_pct": round(float(dd.min()) * 100.0, 4),
            "avg_pnl_pct": round(float(pairs["slip_pnl_pct"].mean()) * 100.0, 4),
            "median_pnl_pct": round(float(pairs["slip_pnl_pct"].median()) * 100.0, 4),
            "median_hold_seconds": round(float(pairs["hold_seconds"].median()), 2),
            "avg_hold_seconds": round(float(pairs["hold_seconds"].mean()), 2),
            "median_mfe_r": round(float(pairs["mfe_r"].median()), 4),
            "median_mae_r": round(float(pairs["mae_r"].median()), 4),
            "exit_reasons": {str(k): int(v) for k, v in pairs["exit_reason"].value_counts().items()},
        }

    base = SimpleNamespace(
        _as_utc=_fallback_as_utc,
        monthly_trade_files=_fallback_monthly_trade_files,
        open_position=_fallback_open_position,
        append_entry=_fallback_append_entry,
        append_exit=_fallback_append_exit,
        update_excursion=_fallback_update_excursion,
        stop_trigger=_fallback_stop_trigger,
        paired_trades=_fallback_paired_trades,
        summarize_ledger=_fallback_summarize_ledger,
    )


DEFAULT_ARCHIVE_ROOT = Path("dataset/archive")


DEFAULT_EXECUTION_PARAMS = {
    "initial_stop_atr": 0.45,
    "stop_buffer_atr": 0.05,
    "stop_cap_atr": 0.80,
    "min_stop_bps": 12.0,
    "breakeven_at_r": 1.0,
    "cost_lock_bps": 10.0,
    "trail_start_r": 1.5,
    "trail_buffer_atr": 0.05,
    "max_hold_hours": 6.0,
    "notional_share": 0.20,
    "slippage": 0.0002,
    "entry_fee": 0.0002,
    "exit_fee": 0.0004,
}


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


def _bool_series(frame: pd.DataFrame, col: str) -> pd.Series:
    if col not in frame.columns:
        return pd.Series(False, index=frame.index)
    raw = frame[col]
    if raw.dtype == bool:
        return raw
    return raw.astype(str).str.lower().isin({"true", "1", "yes"})


def _load_rule(path: str) -> dict:
    data = json.loads(Path(path).read_text(encoding="utf-8"))
    return data.get("selected_rule", {})


def _apply_rule(frame: pd.DataFrame, rule: dict) -> pd.Series:
    if "quality_pass" in frame.columns:
        return _bool_series(frame, "quality_pass")
    mask = pd.Series(True, index=frame.index)
    mask &= frame["markov_llr"] >= float(rule.get("llr_min", -999.0))
    mask &= frame["flow_ratio_60s"] >= float(rule.get("flow60_min", 0.0))
    mask &= frame["speed_60s_atr"] >= float(rule.get("speed60_min", -999.0))
    dwell_seconds = int(rule.get("dwell_seconds", 0))
    if dwell_seconds > 0:
        mask &= _bool_series(frame, f"dwell_{dwell_seconds}s_pass")
    pullback_max = rule.get("pullback30_max")
    if pullback_max is not None:
        mask &= frame["pullback_30s_atr"] <= float(pullback_max)
    return mask.fillna(False)


def _execution_params(args: argparse.Namespace) -> dict:
    params = dict(DEFAULT_EXECUTION_PARAMS)
    params.update(
        {
            "initial_stop_atr": float(args.initial_stop_atr),
            "breakeven_at_r": float(args.breakeven_at_r),
            "trail_start_r": float(args.trail_start_r),
            "trail_buffer_atr": float(args.trail_buffer_atr),
            "max_hold_hours": float(args.max_hold_hours),
            "notional_share": float(args.notional_share),
        }
    )
    return params


def _execution_start(args: argparse.Namespace) -> pd.Timestamp:
    return base._as_utc(args.execute_start) if str(args.execute_start).strip() else base._as_utc(args.start)


def _execution_end(args: argparse.Namespace) -> pd.Timestamp:
    return base._as_utc(args.execute_end) if str(args.execute_end).strip() else base._as_utc(args.end)


def _event_signal(row: pd.Series) -> pd.Series:
    high_value = row.get("touch_high_so_far", row.get("signal_high"))
    low_value = row.get("touch_low_so_far", row.get("signal_low"))
    return pd.Series(
        {
            "atr": float(row["atr"]),
            "low": float(low_value),
            "high": float(high_value),
            "close": float(row["signal_close"]),
        },
        name=pd.Timestamp(row["signal_start"]),
    )


def _update_trailing(position: dict, params: dict) -> None:
    atr = float(position["atr_at_entry"])
    if float(position["mfe_r"]) < float(params["trail_start_r"]):
        return
    if position["side"] == "long":
        trail = float(position["hwm"]) - float(params["trail_buffer_atr"]) * atr
        if trail > float(position["sl"]):
            position["sl"] = trail
            position["trailing_active"] = True
    else:
        trail = float(position["lwm"]) + float(params["trail_buffer_atr"]) * atr
        if trail < float(position["sl"]):
            position["sl"] = trail
            position["trailing_active"] = True


def _annotate_last(logs: list[dict], row: pd.Series, reason: str | None = None) -> None:
    if not logs:
        return
    last = logs[-1]
    if reason is not None:
        last["reason"] = reason
    last["event_id"] = row.get("event_id", "")
    last["shape"] = row.get("shape", "")
    last["quality_bucket"] = row.get("quality_bucket", "")
    last["markov_llr"] = row.get("markov_llr", np.nan)
    last["flow_ratio_60s"] = row.get("flow_ratio_60s", np.nan)
    last["speed_60s_atr"] = row.get("speed_60s_atr", np.nan)
    last["prob_success"] = row.get("prob_success", np.nan)
    last["prob_ev_atr"] = row.get("prob_ev_atr", np.nan)
    last["model_name"] = row.get("model_name", "")
    last["model_notional_share"] = row.get("model_notional_share", np.nan)


def _simulate_event(
    sb: pd.DataFrame,
    row: pd.Series,
    *,
    balance: float,
    params: dict,
    entry_delay_seconds: int,
) -> tuple[list[dict] | None, float, str]:
    touch_time = pd.Timestamp(row["touch_time"])
    if touch_time.tzinfo is None:
        touch_time = touch_time.tz_localize("UTC")
    entry_time = touch_time + pd.Timedelta(seconds=int(entry_delay_seconds))
    idx = sb.index
    start_pos = int(idx.searchsorted(entry_time, side="left"))
    if start_pos >= len(idx):
        return None, balance, "missing_entry_second"

    entry_raw = float(sb["close"].iloc[start_pos])
    sig = _event_signal(row)
    event_params = dict(params)
    row_share = row.get("model_notional_share", np.nan)
    if pd.notna(row_share) and np.isfinite(float(row_share)) and float(row_share) > 0.0:
        event_params["notional_share"] = float(row_share)
    position = base.open_position(sig, str(row["side"]), entry_raw, idx[start_pos], balance, event_params)
    if position is None:
        return None, balance, "min_stop"

    logs: list[dict] = []
    base.append_entry(logs, position, balance)
    _annotate_last(logs, row, "ProbV4-QualityEntry")

    end_time = idx[start_pos] + pd.Timedelta(hours=float(params["max_hold_hours"]))
    end_pos = min(int(idx.searchsorted(end_time, side="left")), len(idx) - 1)
    high_values = sb["high"].to_numpy(dtype="float64", copy=False)
    low_values = sb["low"].to_numpy(dtype="float64", copy=False)
    close_values = sb["close"].to_numpy(dtype="float64", copy=False)

    for pos in range(start_pos + 1, end_pos + 1):
        base.update_excursion(position, float(high_values[pos]), float(low_values[pos]), params)
        _update_trailing(position, event_params)
        triggered, raw_exit, reason = base.stop_trigger(position, float(high_values[pos]), float(low_values[pos]))
        if triggered:
            balance = base.append_exit(
                logs,
                position,
                raw_exit=raw_exit,
                time_value=idx[pos],
                reason=reason,
                balance=balance,
                params=event_params,
            )
            _annotate_last(logs, row)
            return logs, balance, ""
        if idx[pos] >= end_time:
            balance = base.append_exit(
                logs,
                position,
                raw_exit=float(close_values[pos]),
                time_value=idx[pos],
                reason="MaxHoldExit",
                balance=balance,
                params=event_params,
            )
            _annotate_last(logs, row)
            return logs, balance, ""

    balance = base.append_exit(
        logs,
        position,
        raw_exit=float(close_values[end_pos]),
        time_value=idx[end_pos],
        reason="FinalMarkToMarket",
        balance=balance,
        params=event_params,
    )
    _annotate_last(logs, row)
    return logs, balance, ""


def _execution_attribution(ledger: pd.DataFrame, params: dict) -> dict:
    pairs = base.paired_trades(ledger)
    if pairs.empty:
        return {
            "profit_factor": 0.0,
            "gross_profit_pct": 0.0,
            "gross_loss_pct": 0.0,
            "monthly": [],
            "exit_reasons": {},
        }

    fee_rate = float(params["entry_fee"]) + float(params["exit_fee"])
    pairs = pairs.copy()
    pairs["entry_time"] = pd.to_datetime(pairs["entry_time"], utc=True)
    pairs["realistic_trade_pct"] = pairs["slip_pnl_pct"] - fee_rate
    pairs["weighted_realistic_pct"] = pairs["realistic_trade_pct"] * pairs["notional_share"] * 100.0
    gross_profit = float(pairs.loc[pairs["weighted_realistic_pct"] > 0, "weighted_realistic_pct"].sum())
    gross_loss = float(pairs.loc[pairs["weighted_realistic_pct"] < 0, "weighted_realistic_pct"].sum())
    profit_factor = gross_profit / abs(gross_loss) if gross_loss < 0 else np.inf if gross_profit > 0 else 0.0

    monthly_rows = []
    month_keys = pairs["entry_time"].dt.tz_convert(None).dt.to_period("M")
    for month, group in pairs.groupby(month_keys):
        monthly_rows.append(
            {
                "month": str(month),
                "trades": int(len(group)),
                "weighted_realistic_pct": round(float(group["weighted_realistic_pct"].sum()), 6),
                "win_rate_pct": round(float((group["realistic_trade_pct"] > 0).mean()) * 100.0, 4),
            }
        )

    exit_rows = {}
    for reason, group in pairs.groupby("exit_reason"):
        exit_rows[str(reason)] = {
            "trades": int(len(group)),
            "weighted_realistic_pct": round(float(group["weighted_realistic_pct"].sum()), 6),
            "win_rate_pct": round(float((group["realistic_trade_pct"] > 0).mean()) * 100.0, 4),
            "avg_mfe_r": round(float(group["mfe_r"].mean()), 6),
            "avg_mae_r": round(float(group["mae_r"].mean()), 6),
        }

    return {
        "profit_factor": round(float(profit_factor), 6) if np.isfinite(profit_factor) else "inf",
        "gross_profit_pct": round(gross_profit, 6),
        "gross_loss_pct": round(gross_loss, 6),
        "monthly": monthly_rows,
        "exit_reasons": exit_rows,
    }


def _run_symbol(symbol: str, events: pd.DataFrame, args: argparse.Namespace, params: dict) -> dict:
    if events.empty:
        empty_ledger = pd.DataFrame()
        return {
            "summary": base.summarize_ledger(empty_ledger, float(args.initial_balance), params),
            "attribution": _execution_attribution(empty_ledger, params),
            "diagnostics": {"candidate_events": 0, "entries": 0},
            "ledger": empty_ledger,
        }

    start = min(pd.Timestamp(events["touch_time"].min()), _execution_start(args))
    end = max(
        pd.Timestamp(events["touch_time"].max()) + pd.Timedelta(hours=float(params["max_hold_hours"]) + 1),
        _execution_end(args),
    )
    sb, build_stats = _load_or_build_second_bars(symbol, start, end, args)

    balance = float(args.initial_balance)
    logs: list[dict] = []
    last_exit_time = pd.Timestamp.min.tz_localize("UTC")
    consumed_signal_bars: set[str] = set()
    diagnostics = {
        "candidate_events": int(len(events)),
        "entries": 0,
        "busy_skipped": 0,
        "same_signal_bar_skipped": 0,
        "dwell_skipped": 0,
        "min_stop_skipped": 0,
        "missing_entry_second": 0,
    }

    dwell_col = f"dwell_{int(args.entry_delay_seconds)}s_pass"
    for _, row in events.sort_values("touch_time").iterrows():
        if int(args.entry_delay_seconds) > 0 and not _bool_series(pd.DataFrame([row]), dwell_col).iloc[0]:
            diagnostics["dwell_skipped"] += 1
            continue
        signal_key = str(row["signal_start"])
        if signal_key in consumed_signal_bars:
            diagnostics["same_signal_bar_skipped"] += 1
            continue
        touch_time = pd.Timestamp(row["touch_time"])
        if touch_time.tzinfo is None:
            touch_time = touch_time.tz_localize("UTC")
        if touch_time <= last_exit_time:
            diagnostics["busy_skipped"] += 1
            continue

        trade_logs, new_balance, skip_reason = _simulate_event(
            sb,
            row,
            balance=balance,
            params=params,
            entry_delay_seconds=int(args.entry_delay_seconds),
        )
        if skip_reason:
            key = f"{skip_reason}_skipped"
            diagnostics[key] = diagnostics.get(key, 0) + 1
            continue
        if not trade_logs:
            continue
        logs.extend(trade_logs)
        balance = new_balance
        last_exit_time = pd.Timestamp(trade_logs[-1]["time"])
        consumed_signal_bars.add(signal_key)
        diagnostics["entries"] += 1

    ledger = pd.DataFrame(logs)
    summary = base.summarize_ledger(ledger, float(args.initial_balance), params)
    return {
        "summary": summary,
        "attribution": _execution_attribution(ledger, params),
        "diagnostics": diagnostics,
        "ledger": ledger,
        "build_stats": build_stats,
    }


def _write_markdown(summary: dict, path: Path) -> None:
    lines = [
        "# Probabilistic V4 Execution Runner",
        "",
        "范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。",
        "",
        "| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |",
        "|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|",
    ]
    for result in summary["results"]:
        s = result["summary"]
        lines.append(
            f"| `{result['symbol']}` | {s['trades']} | {s['realistic_return_pct']:.4f}% | "
            f"{s['raw_no_fee_no_slip_return_pct']:.4f}% | {s['price_pnl_with_2bps_slip_no_fee_return_pct']:.4f}% | "
            f"{result.get('attribution', {}).get('profit_factor', 0.0)} | "
            f"{s['win_rate_pct']:.2f}% | {s['max_dd_pct']:.4f}% | {s.get('median_hold_seconds', 0.0):.2f}s | "
            f"`{s.get('exit_reasons', {})}` | `{result['diagnostics']}` |"
        )
    lines.extend(["", "## Monthly Attribution", ""])
    for result in summary["results"]:
        lines.append(f"### {result['symbol']}")
        lines.extend(
            [
                "",
                "| Month | Trades | Weighted Realistic | Win |",
                "|---|---:|---:|---:|",
            ]
        )
        for row in result.get("attribution", {}).get("monthly", []):
            lines.append(
                f"| `{row['month']}` | {row['trades']} | {row['weighted_realistic_pct']:.6f}% | "
                f"{row['win_rate_pct']:.4f}% |"
            )
        lines.append("")
    path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Run Probabilistic V4 scored-event execution")
    parser.add_argument("--events-csv", default="research/probabilistic_v4_events_scored.csv")
    parser.add_argument("--rules-json", default="research/probabilistic_v4_quality_rules.json")
    parser.add_argument("--symbols", nargs="+", default=["BTCUSDT", "ETHUSDT"])
    parser.add_argument("--start", default="2026-01-01T00:00:00Z")
    parser.add_argument("--end", default="2026-04-30T23:59:59Z")
    parser.add_argument("--execute-start", default="", help="Optional touch_time lower bound for out-of-sample execution")
    parser.add_argument("--execute-end", default="", help="Optional touch_time upper bound for out-of-sample execution")
    parser.add_argument("--archive-root", default=str(DEFAULT_ARCHIVE_ROOT))
    parser.add_argument("--chunksize", type=int, default=2_000_000)
    parser.add_argument("--bars-cache-dir", default="")
    parser.add_argument("--initial-balance", type=float, default=100000.0)
    parser.add_argument("--entry-delay-seconds", type=int, default=15)
    parser.add_argument("--initial-stop-atr", type=float, default=0.45)
    parser.add_argument("--breakeven-at-r", type=float, default=1.0)
    parser.add_argument("--trail-start-r", type=float, default=1.5)
    parser.add_argument("--trail-buffer-atr", type=float, default=0.05)
    parser.add_argument("--max-hold-hours", type=float, default=6.0)
    parser.add_argument("--notional-share", type=float, default=0.20)
    parser.add_argument("--summary-json", default="research/probabilistic_v4_execution_summary.json")
    parser.add_argument("--markdown", default="research/20260508_probabilistic_v4_execution.md")
    parser.add_argument("--ledger-prefix", default="research/tmp_probabilistic_v4_execution")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    started = time.time()
    events = pd.read_csv(args.events_csv, parse_dates=["touch_time", "signal_start", "signal_end"])
    rule = _load_rule(args.rules_json)
    params = _execution_params(args)
    quality_mask = _apply_rule(events, rule)
    selected_events = events[quality_mask].copy()
    execute_start = _execution_start(args)
    execute_end = _execution_end(args)
    selected_events = selected_events[
        (selected_events["touch_time"] >= execute_start) & (selected_events["touch_time"] <= execute_end)
    ].copy()

    results = []
    for symbol in args.symbols:
        symbol_events = selected_events[selected_events["symbol"] == symbol].copy()
        print(f"{symbol}: selected_events={len(symbol_events)}", flush=True)
        result = _run_symbol(symbol, symbol_events, args, params)
        ledger_path = Path(f"{args.ledger_prefix}_{symbol}_ledger.csv")
        result["ledger"].to_csv(ledger_path, index=False)
        del result["ledger"]
        result.update({"symbol": symbol, "ledger_path": str(ledger_path)})
        results.append(result)
        print(
            f"{symbol}: trades={result['summary']['trades']} realistic={result['summary']['realistic_return_pct']:.4f}% "
            f"diag={result['diagnostics']}",
            flush=True,
        )

    summary = {
        "events_csv": args.events_csv,
        "rules_json": args.rules_json,
        "selected_rule": rule,
        "execute_start": execute_start.isoformat(),
        "execute_end": execute_end.isoformat(),
        "execution_params": params,
        "entry_delay_seconds": int(args.entry_delay_seconds),
        "results": results,
        "elapsed_seconds": round(time.time() - started, 2),
    }
    Path(args.summary_json).write_text(json.dumps(summary, indent=2, ensure_ascii=False, default=str), encoding="utf-8")
    _write_markdown(summary, Path(args.markdown))
    print(json.dumps({"summary_json": args.summary_json, "markdown": args.markdown}, indent=2), flush=True)


if __name__ == "__main__":
    main()
