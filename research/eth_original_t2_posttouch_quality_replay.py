#!/usr/bin/env python3
"""ETH original_t2 intrabar breakout with post-touch continuation confirmation.

Research-only. This runner keeps the original T2 structure:
long touches `prev_high_2`, short touches `prev_low_2`, and the current signal
bar is still open. Unlike direct breakout, it does not fill at the first touch.
It waits for 1s close continuation beyond the touched level and rejects the
setup if adverse movement happens first.
"""

from __future__ import annotations

import argparse
import json
import time
from pathlib import Path

import numpy as np
import pandas as pd

import btc_2026_jan_apr_direct_breakout as direct
import btc_eth_micro_breakout_structure as micro
import eth_q1_breakout_t3_shape_compare as replay


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
    params = {
        "confirm_atr": 0.10,
        "fail_atr": 0.05,
        "confirm_seconds": 300,
        "persist_seconds": 0,
        "max_entry_extension_atr": 0.35,
        "one_setup_per_bar": 1.0,
        "slot0_share": 0.20,
        "slot1_share": 0.10,
    }
    key_map = {
        "confirm": "confirm_atr",
        "fail": "fail_atr",
        "confirm_s": "confirm_seconds",
        "persist_s": "persist_seconds",
        "max_ext": "max_entry_extension_atr",
        "one": "one_setup_per_bar",
        "slot0": "slot0_share",
        "slot1": "slot1_share",
    }
    for item in values.split(","):
        if not item.strip():
            continue
        key, value = item.split(":", 1)
        mapped = key_map.get(key.strip())
        if mapped is None:
            raise ValueError(f"unknown variant key {key} in {raw}")
        params[mapped] = _maybe_number(value)
    for key in ("confirm_seconds", "persist_seconds"):
        params[key] = int(params[key])
    return name, params


def slot_share(trade_slot: int, params: dict) -> float:
    return float(params["slot0_share"] if int(trade_slot) == 0 else params["slot1_share"])


def _pending_levels(side: str, level: float, atr: float, params: dict) -> tuple[float, float]:
    confirm_delta = float(params["confirm_atr"]) * atr
    fail_delta = float(params["fail_atr"]) * atr
    if side == "long":
        return level + confirm_delta, level - fail_delta
    return level - confirm_delta, level + fail_delta


def _evaluate_pending(
    pending: dict,
    *,
    bar_time: pd.Timestamp,
    high_value: float,
    low_value: float,
    close_value: float,
    params: dict,
) -> tuple[bool, str]:
    side = str(pending["side"])
    confirm_level = float(pending["confirm_level"])
    fail_level = float(pending["fail_level"])

    if side == "long":
        pending["best_favorable_atr"] = max(
            float(pending.get("best_favorable_atr", 0.0)),
            (float(high_value) - float(pending["level"])) / float(pending["atr"]),
        )
        pending["worst_adverse_atr"] = max(
            float(pending.get("worst_adverse_atr", 0.0)),
            (float(pending["level"]) - float(low_value)) / float(pending["atr"]),
        )
        above_level = close_value >= float(pending["level"])
        confirmed = close_value >= confirm_level
        failed = low_value <= fail_level
    else:
        pending["best_favorable_atr"] = max(
            float(pending.get("best_favorable_atr", 0.0)),
            (float(pending["level"]) - float(low_value)) / float(pending["atr"]),
        )
        pending["worst_adverse_atr"] = max(
            float(pending.get("worst_adverse_atr", 0.0)),
            (float(high_value) - float(pending["level"])) / float(pending["atr"]),
        )
        above_level = close_value <= float(pending["level"])
        confirmed = close_value <= confirm_level
        failed = high_value >= fail_level

    if above_level:
        pending["persist_count"] = int(pending.get("persist_count", 0)) + 1
    else:
        pending["persist_count"] = 0

    # Conservative OHLC ordering: if fail and confirm are both present in the
    # same 1s bar, reject the setup because tick order is unknown.
    if failed:
        return False, "post_touch_fail"
    if bar_time > pending["deadline"]:
        return False, "confirm_timeout"
    required_persist = int(params["persist_seconds"])
    if confirmed and int(pending.get("persist_count", 0)) >= max(1, required_persist):
        extension_atr = abs(close_value - float(pending["level"])) / float(pending["atr"])
        pending["entry_extension_atr"] = extension_atr
        if extension_atr > float(params["max_entry_extension_atr"]):
            return False, "entry_extension"
        return True, ""
    return False, ""


def _annotate_exit(log: dict, open_entry: dict | None) -> None:
    if open_entry is None:
        return
    for key in (
        "touch_time",
        "confirm_level",
        "fail_level",
        "entry_extension_atr",
        "post_touch_seconds",
        "post_touch_adverse_atr",
        "post_touch_favorable_atr",
    ):
        log[key] = open_entry.get(key, np.nan)


def run_strategy(
    second_bars: pd.DataFrame,
    signal: pd.DataFrame,
    *,
    initial_balance: float,
    timeframe: str,
    variant_name: str,
    params: dict,
    allow_same_bar_second_breakout: bool,
    exit_policy: dict,
) -> dict:
    started = time.time()
    second_index = second_bars.index
    high_values = second_bars["high"].to_numpy(dtype="float64", copy=False)
    low_values = second_bars["low"].to_numpy(dtype="float64", copy=False)
    close_values = second_bars["close"].to_numpy(dtype="float64", copy=False)
    max_trades_per_bar = int(replay.COMMON_REPLAY_KWARGS["max_trades_per_bar"])

    state = {
        "balance": float(initial_balance),
        "position": None,
        "trade_logs": [],
        "diagnostics": {
            "candidate_touches": 0,
            "entries": 0,
            "post_touch_fail": 0,
            "confirm_timeout": 0,
            "signal_bar_end_timeout": 0,
            "entry_extension": 0,
            "max_trades_blocked": 0,
            "setup_blocked_by_one_per_bar": 0,
            "breakout_locks": {"long": {}, "short": {}},
            "allow_same_bar_second_breakout": bool(allow_same_bar_second_breakout),
        },
    }
    entries_by_bar: dict[pd.Timestamp, int] = {}
    open_entry_log: dict | None = None

    for i in range(len(signal) - 1):
        start_t, end_t = signal.index[i], signal.index[i + 1]
        start_pos = int(second_index.searchsorted(start_t, side="left"))
        end_pos = int(second_index.searchsorted(end_t, side="left"))
        if start_pos >= end_pos:
            continue
        base_sig = signal.iloc[i]
        if pd.isna(base_sig["atr"]):
            continue

        trades_in_bar = 0
        direct_breakouts_taken = 0
        setup_consumed = False
        setup_block_counted = False
        pending: dict | None = None
        bar_high_so_far = -np.inf
        bar_low_so_far = np.inf
        live_sig = base_sig.to_dict()
        live_sig["_closed_atr"] = float(base_sig["atr"])

        for current_pos in range(start_pos, end_pos):
            bar_time = second_index[current_pos]
            high_value = float(high_values[current_pos])
            low_value = float(low_values[current_pos])
            close_value = float(close_values[current_pos])
            bar_high_so_far = max(bar_high_so_far, high_value)
            bar_low_so_far = min(bar_low_so_far, low_value)
            sig = replay._intrabar_signal(live_sig, bar_high_so_far, bar_low_so_far, close_value)

            position = state["position"]
            if position is not None:
                exit_triggered, raw_exit_price, reason = direct._advance_position(position, sig, high_value, low_value)
                if exit_triggered:
                    before = len(state["trade_logs"])
                    direct._append_exit(state, position, bar_time=bar_time, raw_exit_price=raw_exit_price, reason=reason)
                    if len(state["trade_logs"]) > before:
                        _annotate_exit(state["trade_logs"][-1], open_entry_log)
                    state["position"] = None
                    open_entry_log = None
                continue

            if pending is not None:
                confirmed, skip_reason = _evaluate_pending(
                    pending,
                    bar_time=bar_time,
                    high_value=high_value,
                    low_value=low_value,
                    close_value=close_value,
                    params=params,
                )
                if skip_reason:
                    state["diagnostics"][skip_reason] = state["diagnostics"].get(skip_reason, 0) + 1
                    pending = None
                    continue
                if not confirmed:
                    continue

                notional_share = slot_share(trades_in_bar, params)
                state["balance"], state["position"] = direct._open_direct_position(
                    state["balance"],
                    sig,
                    side=str(pending["side"]),
                    fill_raw=close_value,
                    notional_share=notional_share,
                    shape_name=str(pending["shape_name"]),
                    breakout_level=float(pending["level"]),
                    signal_bar_time=start_t,
                    trade_slot=trades_in_bar,
                    exit_policy=exit_policy,
                )
                position = state["position"]
                locks = state["diagnostics"]["breakout_locks"][str(pending["side"])]
                locks[str(pending["shape_name"])] = locks.get(str(pending["shape_name"]), 0) + 1
                state["diagnostics"]["entries"] += 1
                entries_by_bar[start_t] = entries_by_bar.get(start_t, 0) + 1
                direct_breakouts_taken += 1
                post_touch_seconds = float((bar_time - pending["touch_time"]).total_seconds())
                log = {
                    "time": bar_time,
                    "type": "BUY" if pending["side"] == "long" else "SHORT",
                    "price": position["entry_p"],
                    "reason": "OriginalT2-PostTouchConfirm",
                    "notional": position["notional"],
                    "notional_share": notional_share,
                    "bal": state["balance"],
                    "breakout_shape_name": pending["shape_name"],
                    "breakout_level": float(pending["level"]),
                    "observed_fill_raw": float(close_value),
                    "signal_bar_time": start_t,
                    "trade_slot": trades_in_bar,
                    "raw_exit_price": np.nan,
                    "real_stop_price": position["sl"],
                    "entry_vs_breakout_bps": (
                        (close_value - float(pending["level"])) / float(pending["level"])
                        if pending["side"] == "long"
                        else (float(pending["level"]) - close_value) / float(pending["level"])
                    )
                    * 10000.0,
                    "exit_policy_name": str(exit_policy.get("name", "")),
                    "touch_time": pending["touch_time"],
                    "confirm_level": float(pending["confirm_level"]),
                    "fail_level": float(pending["fail_level"]),
                    "entry_extension_atr": float(pending.get("entry_extension_atr", np.nan)),
                    "post_touch_seconds": post_touch_seconds,
                    "post_touch_adverse_atr": float(pending.get("worst_adverse_atr", 0.0)),
                    "post_touch_favorable_atr": float(pending.get("best_favorable_atr", 0.0)),
                }
                state["trade_logs"].append(log)
                open_entry_log = log
                trades_in_bar += 1
                pending = None
                continue

            if direct_breakouts_taken > 0 and not allow_same_bar_second_breakout:
                continue
            if trades_in_bar >= max_trades_per_bar:
                state["diagnostics"]["max_trades_blocked"] += 1
                continue
            if setup_consumed and float(params["one_setup_per_bar"]) > 0:
                if not setup_block_counted:
                    state["diagnostics"]["setup_blocked_by_one_per_bar"] += 1
                    setup_block_counted = True
                continue

            long_ready, short_ready = replay._resolve_regime_ready(sig, "1d")
            side = ""
            breakout_level = np.nan
            shape_name = ""
            if long_ready:
                triggered, breakout_level, shape_name = replay._long_breakout(sig, high_value, "original_t2")
                if triggered:
                    side = "long"
            elif short_ready:
                triggered, breakout_level, shape_name = replay._short_breakout(sig, low_value, "original_t2")
                if triggered:
                    side = "short"
            if not side:
                continue

            atr = float(sig["atr"])
            confirm_level, fail_level = _pending_levels(side, float(breakout_level), atr, params)
            deadline = min(bar_time + pd.Timedelta(seconds=int(params["confirm_seconds"])), end_t - pd.Timedelta(seconds=1))
            pending = {
                "side": side,
                "level": float(breakout_level),
                "shape_name": shape_name,
                "atr": atr,
                "touch_time": bar_time,
                "confirm_level": confirm_level,
                "fail_level": fail_level,
                "deadline": deadline,
                "persist_count": 0,
                "best_favorable_atr": 0.0,
                "worst_adverse_atr": 0.0,
            }
            setup_consumed = True
            state["diagnostics"]["candidate_touches"] += 1
            confirmed, skip_reason = _evaluate_pending(
                pending,
                bar_time=bar_time,
                high_value=high_value,
                low_value=low_value,
                close_value=close_value,
                params=params,
            )
            if skip_reason:
                state["diagnostics"][skip_reason] = state["diagnostics"].get(skip_reason, 0) + 1
                pending = None
                continue
            if confirmed:
                # Re-evaluate on the next loop iteration through the same
                # codepath would miss this 1s close, so leave it pending and
                # force a tiny local loop by falling through to the pending
                # branch logic in-place.
                notional_share = slot_share(trades_in_bar, params)
                state["balance"], state["position"] = direct._open_direct_position(
                    state["balance"],
                    sig,
                    side=side,
                    fill_raw=close_value,
                    notional_share=notional_share,
                    shape_name=shape_name,
                    breakout_level=float(breakout_level),
                    signal_bar_time=start_t,
                    trade_slot=trades_in_bar,
                    exit_policy=exit_policy,
                )
                position = state["position"]
                locks = state["diagnostics"]["breakout_locks"][side]
                locks[shape_name] = locks.get(shape_name, 0) + 1
                state["diagnostics"]["entries"] += 1
                entries_by_bar[start_t] = entries_by_bar.get(start_t, 0) + 1
                direct_breakouts_taken += 1
                log = {
                    "time": bar_time,
                    "type": "BUY" if side == "long" else "SHORT",
                    "price": position["entry_p"],
                    "reason": "OriginalT2-PostTouchConfirm",
                    "notional": position["notional"],
                    "notional_share": notional_share,
                    "bal": state["balance"],
                    "breakout_shape_name": shape_name,
                    "breakout_level": float(breakout_level),
                    "observed_fill_raw": float(close_value),
                    "signal_bar_time": start_t,
                    "trade_slot": trades_in_bar,
                    "raw_exit_price": np.nan,
                    "real_stop_price": position["sl"],
                    "entry_vs_breakout_bps": (
                        (close_value - float(breakout_level)) / float(breakout_level)
                        if side == "long"
                        else (float(breakout_level) - close_value) / float(breakout_level)
                    )
                    * 10000.0,
                    "exit_policy_name": str(exit_policy.get("name", "")),
                    "touch_time": bar_time,
                    "confirm_level": float(confirm_level),
                    "fail_level": float(fail_level),
                    "entry_extension_atr": float(pending.get("entry_extension_atr", np.nan)),
                    "post_touch_seconds": 0.0,
                    "post_touch_adverse_atr": float(pending.get("worst_adverse_atr", 0.0)),
                    "post_touch_favorable_atr": float(pending.get("best_favorable_atr", 0.0)),
                }
                state["trade_logs"].append(log)
                open_entry_log = log
                trades_in_bar += 1
                pending = None

        if pending is not None:
            state["diagnostics"]["signal_bar_end_timeout"] += 1

    if state["position"] is not None and len(close_values) > 0:
        before = len(state["trade_logs"])
        direct._append_exit(
            state,
            state["position"],
            bar_time=second_index[-1],
            raw_exit_price=float(close_values[-1]),
            reason="FinalMarkToMarket",
        )
        if len(state["trade_logs"]) > before:
            _annotate_exit(state["trade_logs"][-1], open_entry_log)
        state["position"] = None

    ledger = pd.DataFrame(state["trade_logs"])
    pairs = direct._paired_trades(ledger)
    bar_guard = {
        "bars_with_entry": int(len(entries_by_bar)),
        "bars_with_multi_entries": int(sum(1 for v in entries_by_bar.values() if v > 1)),
        "extra_entries": int(sum(max(0, v - 1) for v in entries_by_bar.values())),
        "max_entries_per_bar": int(max(entries_by_bar.values()) if entries_by_bar else 0),
    }
    return {
        "timeframe": timeframe,
        "variant": variant_name,
        "breakout_shape": "original_t2",
        "exit_policy": exit_policy,
        "summary": replay.summarize_run(ledger, initial_balance),
        "pair_diagnostics": direct._pair_diagnostics(pairs),
        "attribution": replay.summarize_breakout_attribution(ledger),
        "diagnostics": state["diagnostics"],
        "bar_guard_diagnostics": bar_guard,
        "entry_distance_diagnostics": direct._entry_distance_diagnostics(ledger),
        "exit_hold_diagnostics": direct._exit_hold_diagnostics(pairs),
        "trade_slot_diagnostics": direct._trade_slot_diagnostics(ledger),
        "mfe_mae_diagnostics": direct._mfe_mae_diagnostics(ledger),
        "post_touch_diagnostics": post_touch_diagnostics(ledger),
        "accounting_2bps_maker_entry_market_exit": direct._accounting_from_ledger(
            ledger,
            initial_balance=initial_balance,
            slippage=float(replay.COMMON_REPLAY_KWARGS["fixed_slippage"]),
            entry_fee=0.0002,
            exit_fee=0.0004,
        ),
        "elapsed_seconds": round(time.time() - started, 2),
        "ledger": ledger,
    }


def post_touch_diagnostics(ledger: pd.DataFrame) -> dict:
    if ledger.empty:
        return {}
    entries = ledger[ledger["type"].isin(["BUY", "SHORT"])].copy()
    if entries.empty:
        return {}
    out = {}
    for key in ("entry_extension_atr", "post_touch_seconds", "post_touch_adverse_atr", "post_touch_favorable_atr"):
        values = pd.to_numeric(entries[key], errors="coerce").dropna() if key in entries else pd.Series(dtype="float64")
        if values.empty:
            continue
        out[key] = {
            "avg": round(float(values.mean()), 4),
            "median": round(float(values.median()), 4),
            "p75": round(float(values.quantile(0.75)), 4),
            "p90": round(float(values.quantile(0.90)), 4),
        }
    return out


def write_markdown(summary: dict, path: Path) -> None:
    lines = [
        f"# ETH 1h original_t2 touch 后延续确认回测（{summary['start']} 至 {summary['end']}）",
        "",
        "范围：仅限 `research`。本回测使用真正 `original_t2`：long level 为 `prev_high_2`，short level 为 `prev_low_2`，当前 1h signal bar 未闭合，由 `1s high/low` 触发 touch。",
        "",
        "成交语义：touch 后不立即成交；只有在同一根 signal bar 内，`1s close` 先到达突破方向 `confirm_atr`，且没有先触达反向 `fail_atr`，才按确认那根 `1s close` 市价成交。若同一根 1s bar 同时满足 fail 和 confirm，按保守顺序记为 fail。",
        "",
        "成本：滑点 `2bps/side`，手续费 maker entry `2bps` + market exit `4bps`，realistic 包含两者。",
        "",
        "| Variant | 笔数 | Realistic | Raw | 2bps Slip No Fee | Fees | 胜率 | Max DD | Avg Hold | Median Hold | Touch | Fail | Timeout | Entry Ext | PostTouch(s) | Exit Reasons |",
        "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---|---|",
    ]
    for result in summary["results"]:
        acct = result["accounting_2bps_maker_entry_market_exit"]
        s = result["summary"]
        d = result["pair_diagnostics"]
        diag = result["diagnostics"]
        post = result.get("post_touch_diagnostics", {})
        ext = post.get("entry_extension_atr", {})
        secs = post.get("post_touch_seconds", {})
        lines.append(
            f"| `{result['variant']}` | {acct['round_trips']} | "
            f"{acct['realistic_return_pct']:.4f}% | {acct['raw_no_fee_no_slip_return_pct']:.4f}% | "
            f"{acct['price_pnl_with_2bps_slip_no_fee_return_pct']:.4f}% | {acct['realistic_fees_pct']:.4f}% | "
            f"{s['win_rate_pct']:.2f}% | {s['max_dd_pct']:.2f}% | {d.get('avg_hold_seconds', 0.0):.2f}s | "
            f"{d.get('median_hold_seconds', 0.0):.2f}s | {diag.get('candidate_touches', 0)} | "
            f"{diag.get('post_touch_fail', 0)} | {diag.get('confirm_timeout', 0) + diag.get('signal_bar_end_timeout', 0)} | "
            f"`{ext}` | `{secs}` | `{s['exit_reasons']}` |"
        )
    lines.extend(["", "## 参数", ""])
    for result in summary["results"]:
        lines.append(f"- `{result['variant']}`: `{result['params']}`")
    lines.extend(["", "## 文件", ""])
    lines.append(f"- Summary JSON：`{summary['summary_path']}`")
    for result in summary["results"]:
        lines.append(f"- `{result['variant']}` ledger：`{result['ledger_path']}`")
    lines.append("")
    path.write_text("\n".join(lines), encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="ETH original_t2 post-touch continuation replay")
    parser.add_argument("--symbol", default="ETHUSDT")
    parser.add_argument("--start", default="2026-01-01T00:00:00Z")
    parser.add_argument("--end", default="2026-04-30T23:59:59Z")
    parser.add_argument("--timeframes", nargs="+", default=["1h"])
    parser.add_argument("--archive-root", default=str(micro.DEFAULT_ARCHIVE_ROOT))
    parser.add_argument("--cache-root", default=str(micro.DEFAULT_CACHE_ROOT))
    parser.add_argument("--no-cache", action="store_true")
    parser.add_argument("--chunksize", type=int, default=2_000_000)
    parser.add_argument("--initial-balance", type=float, default=100000.0)
    parser.add_argument("--stop-loss-atr", type=float, default=0.30)
    parser.add_argument("--allow-same-bar-second-breakout", action="store_true")
    parser.add_argument(
        "--variants",
        nargs="+",
        default=[
            "c05_f03_one=confirm:0.05,fail:0.03,confirm_s:300,max_ext:0.25,one:true",
            "c10_f05_one=confirm:0.10,fail:0.05,confirm_s:300,max_ext:0.35,one:true",
            "c15_f05_one=confirm:0.15,fail:0.05,confirm_s:300,max_ext:0.45,one:true",
            "c20_f10_one=confirm:0.20,fail:0.10,confirm_s:300,max_ext:0.55,one:true",
            "c50_f20_one=confirm:0.50,fail:0.20,confirm_s:900,max_ext:0.80,one:true",
            "p10_c10_f05_one=confirm:0.10,fail:0.05,confirm_s:300,persist_s:10,max_ext:0.35,one:true",
        ],
    )
    parser.add_argument("--summary-json", default="research/eth_original_t2_posttouch_quality_summary.json")
    parser.add_argument("--markdown", default="research/20260508_eth_original_t2_posttouch_quality.md")
    parser.add_argument("--ledger-prefix", default="research/tmp_eth_original_t2_posttouch_quality")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    started = time.time()
    start = micro.base._as_utc(args.start)
    end = micro.base._as_utc(args.end)

    replay.COMMON_REPLAY_KWARGS.update(direct.BTC_LIVE_LIKE_REPLAY_KWARGS)
    replay.COMMON_REPLAY_KWARGS.update(
        {
            "fixed_slippage": 0.0002,
            "commission": 0.0,
            "stop_loss_atr": float(args.stop_loss_atr),
            "stop_mode": "atr",
            "max_trades_per_bar": 2,
            "reentry_size_schedule": [0.20, 0.10],
        }
    )
    exit_policy = direct._base_exit_policy()
    variants = [parse_variant(raw) for raw in args.variants]
    second_bars, build_stats = micro.load_or_build_second_bars(
        args.symbol,
        start,
        end,
        Path(args.archive_root),
        args.chunksize,
        Path(args.cache_root),
        not args.no_cache,
    )

    results = []
    for timeframe in args.timeframes:
        _, signal = replay.build_signal_frame(second_bars, timeframe)
        run_signal = direct._add_right_boundary(signal, timeframe)
        print(f"{args.symbol} timeframe={timeframe} second_rows={len(second_bars)} signal_rows={len(signal)}", flush=True)
        for variant_name, params in variants:
            result = run_strategy(
                second_bars,
                run_signal,
                initial_balance=args.initial_balance,
                timeframe=timeframe,
                variant_name=variant_name,
                params=params,
                allow_same_bar_second_breakout=args.allow_same_bar_second_breakout,
                exit_policy=exit_policy,
            )
            ledger_path = Path(f"{args.ledger_prefix}_{timeframe}_{variant_name}_ledger.csv")
            result["ledger"].to_csv(ledger_path, index=False)
            del result["ledger"]
            result.update({"params": params, "ledger_path": str(ledger_path)})
            acct = result["accounting_2bps_maker_entry_market_exit"]
            print(
                f"{timeframe} {variant_name}: realistic={acct['realistic_return_pct']:.4f}% "
                f"raw={acct['raw_no_fee_no_slip_return_pct']:.4f}% trades={acct['round_trips']} "
                f"diag={result['diagnostics']}",
                flush=True,
            )
            results.append(result)

    summary = {
        "symbol": args.symbol,
        "start": start.isoformat(),
        "end": end.isoformat(),
        "timeframes": args.timeframes,
        "build_stats": build_stats,
        "mode": {
            "breakout_shape": "original_t2",
            "entry": "intrabar 1s high/low touch, then 1s close post-touch confirmation",
            "fill": "confirming 1s close with 2bps side slippage in accounting",
            "allow_same_bar_second_breakout": bool(args.allow_same_bar_second_breakout),
            "stop_loss_atr": float(args.stop_loss_atr),
            "max_trades_per_bar": 2,
            "reentry_size_schedule": [0.20, 0.10],
        },
        "results": results,
        "summary_path": args.summary_json,
        "markdown_path": args.markdown,
        "elapsed_seconds": round(time.time() - started, 2),
    }
    Path(args.summary_json).write_text(json.dumps(summary, indent=2, ensure_ascii=False), encoding="utf-8")
    write_markdown(summary, Path(args.markdown))
    print(
        json.dumps(
            {"summary_path": args.summary_json, "markdown_path": args.markdown, "elapsed_seconds": summary["elapsed_seconds"]},
            indent=2,
        ),
        flush=True,
    )


if __name__ == "__main__":
    main()
