#!/usr/bin/env python3
"""ETH original_t2 intrabar breakout with micro-strength entry filtering.

Research-only. This runner uses the original T2 three-bar structure:
`prev_high_2/prev_low_2` is the breakout level, and the current signal bar is
still open. Long entries trigger on current-bar 1s high touching the level;
short entries trigger on current-bar 1s low touching the level. It then applies
a 1s micro-strength filter before opening real exposure.
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
        "micro_window_seconds": 300,
        "micro_fast_seconds": 60,
        "base_speed_atr": 0.02,
        "base_fast_atr": 0.00,
        "base_efficiency": 0.12,
        "strong_speed_atr": 0.08,
        "strong_fast_atr": 0.02,
        "strong_efficiency": 0.30,
        "strong_close_pos": 0.60,
        "skip_weak": 1.0,
        "weak_share": 0.0,
        "base_share": 1.0,
        "strong_share": 1.0,
        "strong_slot0_share": 0.20,
        "strong_slot1_share": 0.10,
        "base_slot0_share": 0.20,
        "base_slot1_share": 0.10,
        "reject_bar_on_weak": 0.0,
    }
    key_map = {
        "micro": "micro_window_seconds",
        "fast": "micro_fast_seconds",
        "base_speed": "base_speed_atr",
        "base_fast": "base_fast_atr",
        "base_eff": "base_efficiency",
        "strong_speed": "strong_speed_atr",
        "strong_fast": "strong_fast_atr",
        "strong_eff": "strong_efficiency",
        "strong_pos": "strong_close_pos",
        "skip_weak": "skip_weak",
        "base0": "base_slot0_share",
        "base1": "base_slot1_share",
        "strong0": "strong_slot0_share",
        "strong1": "strong_slot1_share",
        "reject_bar_on_weak": "reject_bar_on_weak",
    }
    for item in values.split(","):
        if not item.strip():
            continue
        key, value = item.split(":", 1)
        mapped = key_map.get(key.strip())
        if mapped is None:
            raise ValueError(f"unknown variant key {key} in {raw}")
        params[mapped] = _maybe_number(value)
    for key in ("micro_window_seconds", "micro_fast_seconds"):
        params[key] = int(params[key])
    return name, params


def slot_share(quality_tier: str, trade_slot: int, params: dict) -> float:
    if quality_tier == "weak":
        return 0.0
    tier = "strong" if quality_tier == "strong" else "base"
    key = f"{tier}_slot{min(int(trade_slot), 1)}_share"
    return float(params[key])


def annotate_entry(log: dict, quality: dict) -> None:
    log.update(
        {
            "quality_tier": quality["quality_tier"],
            "micro_speed_atr": quality["micro_speed_atr"],
            "micro_fast_atr": quality["micro_fast_atr"],
            "micro_efficiency": quality["micro_efficiency"],
            "micro_close_pos": quality["micro_close_pos"],
        }
    )


def annotate_exit(log: dict, open_entry: dict | None) -> None:
    if open_entry is None:
        return
    for key in ("quality_tier", "micro_speed_atr", "micro_fast_atr", "micro_efficiency", "micro_close_pos"):
        log[key] = open_entry.get(key, np.nan if key != "quality_tier" else "")


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
            "breakout_locks": {"long": {}, "short": {}},
            "candidate_breakouts": 0,
            "entries": 0,
            "weak_skipped": 0,
            "base_candidates": 0,
            "strong_candidates": 0,
            "max_trades_blocked": 0,
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
        bar_rejected_by_weak = False
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
            long_ready, short_ready = replay._resolve_regime_ready(sig, "1d")

            position = state["position"]
            if position is not None:
                exit_triggered, raw_exit_price, reason = direct._advance_position(position, sig, high_value, low_value)
                if exit_triggered:
                    before = len(state["trade_logs"])
                    direct._append_exit(state, position, bar_time=bar_time, raw_exit_price=raw_exit_price, reason=reason)
                    if len(state["trade_logs"]) > before:
                        annotate_exit(state["trade_logs"][-1], open_entry_log)
                    state["position"] = None
                    open_entry_log = None
                continue

            if direct_breakouts_taken > 0 and not allow_same_bar_second_breakout:
                continue
            if bar_rejected_by_weak:
                continue

            if trades_in_bar >= max_trades_per_bar:
                state["diagnostics"]["max_trades_blocked"] += 1
                continue

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

            state["diagnostics"]["candidate_breakouts"] += 1
            quality = micro.micro_context(second_bars, current_pos, pd.Series({"atr": float(sig["atr"])}), side, params)
            tier = str(quality["quality_tier"])
            state["diagnostics"][f"{tier}_candidates"] = state["diagnostics"].get(f"{tier}_candidates", 0) + 1
            notional_share = slot_share(tier, trades_in_bar, params)
            if tier == "weak" and float(params["skip_weak"]) > 0:
                state["diagnostics"]["weak_skipped"] += 1
                if float(params.get("reject_bar_on_weak", 0.0)) > 0:
                    state["diagnostics"]["bars_rejected_by_weak"] = state["diagnostics"].get("bars_rejected_by_weak", 0) + 1
                    bar_rejected_by_weak = True
                continue
            if notional_share <= 0:
                state["diagnostics"]["weak_skipped"] += 1
                continue

            state["balance"], state["position"] = direct._open_direct_position(
                state["balance"],
                sig,
                side=side,
                fill_raw=close_value,
                notional_share=notional_share,
                shape_name=shape_name,
                breakout_level=breakout_level,
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
                "reason": "OriginalT2-MicroBreakout",
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
                    (close_value - breakout_level) / breakout_level
                    if side == "long"
                    else (breakout_level - close_value) / breakout_level
                )
                * 10000.0,
                "exit_policy_name": str(exit_policy.get("name", "")),
            }
            annotate_entry(log, quality)
            state["trade_logs"].append(log)
            open_entry_log = log
            trades_in_bar += 1

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
            annotate_exit(state["trade_logs"][-1], open_entry_log)
        state["position"] = None

    ledger = pd.DataFrame(state["trade_logs"])
    pairs = direct._paired_trades(ledger)
    state["diagnostics"]["bars_with_entry"] = int(len(entries_by_bar))
    bar_guard = {
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
        "quality_diagnostics": quality_diagnostics(ledger),
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


def quality_diagnostics(ledger: pd.DataFrame) -> dict:
    if ledger.empty or "quality_tier" not in ledger:
        return {}
    rows = []
    entry = None
    for _, row in ledger.iterrows():
        if row["type"] in {"BUY", "SHORT"}:
            entry = row
            continue
        if row["type"] != "EXIT" or entry is None:
            continue
        side_mult = 1.0 if entry["type"] == "BUY" else -1.0
        rows.append(
            {
                "quality_tier": str(entry.get("quality_tier", "")),
                "pnl_pct": side_mult * (float(row["price"]) - float(entry["price"])) / float(entry["price"]),
                "notional_share": float(entry.get("notional_share", 0.0)),
                "exit_reason": str(row.get("reason", "")),
            }
        )
        entry = None
    if not rows:
        return {}
    pairs = pd.DataFrame(rows)
    out = {}
    for tier, group in pairs.groupby("quality_tier"):
        pnl = group["pnl_pct"].astype("float64")
        out[str(tier)] = {
            "trades": int(len(group)),
            "win_rate_pct": round(float((pnl > 0).mean()) * 100.0, 2),
            "avg_pnl_pct": round(float(pnl.mean()) * 100.0, 4),
            "median_pnl_pct": round(float(pnl.median()) * 100.0, 4),
            "exit_reasons": {str(k): int(v) for k, v in group["exit_reason"].value_counts().items()},
        }
    return out


def write_markdown(summary: dict, path: Path) -> None:
    lines = [
        f"# ETH original_t2 + micro strength 1s 回测（{summary['start']} 至 {summary['end']}）",
        "",
        "范围：仅限 `research`。该回测使用 `original_t2` 三根 bar 结构：long level 为 `prev_high_2`，short level 为 `prev_low_2`，当前 signal bar 未闭合，由 `1s high/low` 触发。开仓前额外计算最近 `1s` micro strength，weak 跳过，base/strong 按 slot0/slot1 仓位入场。",
        "",
        "成本：滑点 `2bps/side`，手续费 maker entry `2bps` + market exit `4bps`，realistic 包含两者。",
        "",
        "| Timeframe | Variant | 笔数 | Realistic | Raw | 2bps Slip No Fee | Fees | 胜率 | Max DD | Avg Hold | Median Hold | Exit Reasons | Quality | Candidate | Weak Skip | Max Entries/Bar |",
        "|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---|---:|---:|---:|",
    ]
    for result in summary["results"]:
        acct = result["accounting_2bps_maker_entry_market_exit"]
        s = result["summary"]
        d = result["pair_diagnostics"]
        diag = result["diagnostics"]
        lines.append(
            f"| `{result['timeframe']}` | `{result['variant']}` | {acct['round_trips']} | "
            f"{acct['realistic_return_pct']:.4f}% | {acct['raw_no_fee_no_slip_return_pct']:.4f}% | "
            f"{acct['price_pnl_with_2bps_slip_no_fee_return_pct']:.4f}% | {acct['realistic_fees_pct']:.4f}% | "
            f"{s['win_rate_pct']:.2f}% | {s['max_dd_pct']:.2f}% | {d.get('avg_hold_seconds', 0.0):.2f}s | "
            f"{d.get('median_hold_seconds', 0.0):.2f}s | `{s['exit_reasons']}` | "
            f"`{result.get('quality_diagnostics', {})}` | {diag['candidate_breakouts']} | "
            f"{diag['weak_skipped']} | {result['bar_guard_diagnostics']['max_entries_per_bar']} |"
        )
    lines.extend(["", "## 文件", ""])
    lines.append(f"- Summary JSON：`{summary['summary_path']}`")
    for result in summary["results"]:
        lines.append(f"- `{result['variant']}` ledger：`{result['ledger_path']}`")
    lines.append("")
    path.write_text("\n".join(lines), encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="ETH original_t2 intrabar breakout with micro-strength filter")
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
            "skipweak_s10b4=base_speed:0.02,base_fast:0.0,base_eff:0.12,strong_speed:0.08,strong_fast:0.02,strong_eff:0.30,strong_pos:0.60",
            "strict_s10b4=base_speed:0.04,base_fast:0.01,base_eff:0.20,strong_speed:0.10,strong_fast:0.03,strong_eff:0.40,strong_pos:0.65",
            "oneshot_s10b4=base_speed:0.02,base_fast:0.0,base_eff:0.12,strong_speed:0.08,strong_fast:0.02,strong_eff:0.30,strong_pos:0.60,reject_bar_on_weak:true",
        ],
    )
    parser.add_argument("--summary-json", default="research/eth_original_t2_micro_filter_replay_summary.json")
    parser.add_argument("--markdown", default="research/20260508_eth_original_t2_micro_filter_replay.md")
    parser.add_argument("--ledger-prefix", default="research/tmp_eth_original_t2_micro_filter_replay")
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
            "entry": "intrabar 1s high/low touches prev_high_2/prev_low_2, filled at triggering 1s close",
            "micro_filter": "weak skipped, base/strong use configured slot shares",
            "allow_same_bar_second_breakout": bool(args.allow_same_bar_second_breakout),
            "stop_loss_atr": float(args.stop_loss_atr),
        },
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
