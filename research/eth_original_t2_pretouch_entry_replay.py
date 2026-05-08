#!/usr/bin/env python3
"""ETH original_t2 pre-touch state-filtered entry replay.

Research-only. This runner consumes the original_t2 pre-touch candidate table
and opens real exposure at the next 1s close when a configured state matches.
It is intended to validate whether high-proxy-edge pre-touch buckets survive
the same 1s execution and baseline exit machinery used by direct breakout.
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


PRESET_STATES: dict[str, list[dict[str, str]]] = {
    "fast_clean": [
        {"distance_bucket": "0.10-0.15", "speed300_bucket": ">=0.20", "pullback_bucket": "0-0.02"},
    ],
    "fast_clean_or_small_pullback": [
        {"distance_bucket": "0.10-0.15", "speed300_bucket": ">=0.20", "pullback_bucket": "0-0.02"},
        {"distance_bucket": "0.10-0.15", "speed300_bucket": ">=0.20", "pullback_bucket": "0.02-0.05"},
    ],
    "edge10_c1f03": [
        {"distance_bucket": "0.10-0.15", "speed300_bucket": ">=0.20", "pullback_bucket": "0-0.02"},
        {"distance_bucket": "0.10-0.15", "speed300_bucket": ">=0.20", "pullback_bucket": "0.02-0.05"},
        {"distance_bucket": "0.15-0.20", "speed300_bucket": "0.03-0.10", "pullback_bucket": "0.02-0.05"},
    ],
    "edge8_c05f03": [
        {"distance_bucket": "0.10-0.15", "speed300_bucket": ">=0.20", "pullback_bucket": "0-0.02"},
        {"distance_bucket": "0.10-0.15", "speed300_bucket": ">=0.20", "pullback_bucket": "0.02-0.05"},
        {"distance_bucket": "0.15-0.20", "speed300_bucket": "0.10-0.20", "pullback_bucket": "0.02-0.05"},
    ],
    "fast_clean_d8_near": [
        {
            "distance_bucket": "0.10-0.15",
            "speed300_bucket": ">=0.20",
            "pullback_bucket": "0-0.02",
            "donchian_gap_bucket": "0-0.02",
        },
        {
            "distance_bucket": "0.10-0.15",
            "speed300_bucket": ">=0.20",
            "pullback_bucket": "0-0.02",
            "donchian_gap_bucket": "0.02-0.05",
        },
        {
            "distance_bucket": "0.10-0.15",
            "speed300_bucket": ">=0.20",
            "pullback_bucket": "0-0.02",
            "donchian_gap_bucket": "0.05-0.10",
        },
    ],
    "fast_clean_d8_exact": [
        {
            "distance_bucket": "0.10-0.15",
            "speed300_bucket": ">=0.20",
            "pullback_bucket": "0-0.02",
            "donchian_gap_bucket": "0-0.02",
        },
        {
            "distance_bucket": "0.10-0.15",
            "speed300_bucket": ">=0.20",
            "pullback_bucket": "0-0.02",
            "donchian_gap_bucket": "0.02-0.05",
        },
    ],
    "edge10_d8_near": [
        {
            "distance_bucket": "0.10-0.15",
            "speed300_bucket": ">=0.20",
            "pullback_bucket": "0-0.02",
            "donchian_gap_bucket": "0-0.02",
        },
        {
            "distance_bucket": "0.10-0.15",
            "speed300_bucket": ">=0.20",
            "pullback_bucket": "0-0.02",
            "donchian_gap_bucket": "0.02-0.05",
        },
        {
            "distance_bucket": "0.10-0.15",
            "speed300_bucket": ">=0.20",
            "pullback_bucket": "0-0.02",
            "donchian_gap_bucket": "0.05-0.10",
        },
        {
            "distance_bucket": "0.10-0.15",
            "speed300_bucket": ">=0.20",
            "pullback_bucket": "0.02-0.05",
            "donchian_gap_bucket": "0-0.02",
        },
        {
            "distance_bucket": "0.10-0.15",
            "speed300_bucket": ">=0.20",
            "pullback_bucket": "0.02-0.05",
            "donchian_gap_bucket": "0.02-0.05",
        },
        {
            "distance_bucket": "0.10-0.15",
            "speed300_bucket": ">=0.20",
            "pullback_bucket": "0.02-0.05",
            "donchian_gap_bucket": "0.05-0.10",
        },
    ],
    "fast_clean_d8_far": [
        {
            "distance_bucket": "0.10-0.15",
            "speed300_bucket": ">=0.20",
            "pullback_bucket": "0-0.02",
            "donchian_gap_bucket": "0.40+",
        },
    ],
    "headroom_top3": [
        {
            "distance_bucket": "0.10-0.15",
            "speed300_bucket": ">=0.20",
            "pullback_bucket": "0-0.02",
            "donchian_gap_bucket": "0.40+",
        },
        {
            "distance_bucket": "0.05-0.10",
            "speed300_bucket": "0.10-0.20",
            "pullback_bucket": "0.02-0.05",
            "donchian_gap_bucket": "0.40+",
        },
        {
            "distance_bucket": "0.15-0.20",
            "speed300_bucket": "0.10-0.20",
            "pullback_bucket": "0.02-0.05",
            "donchian_gap_bucket": "0.40+",
        },
    ],
    "headroom_fast_small_pullback": [
        {
            "distance_bucket": "0.10-0.15",
            "speed300_bucket": ">=0.20",
            "pullback_bucket": "0-0.02",
            "donchian_gap_bucket": "0.40+",
        },
        {
            "distance_bucket": "0.10-0.15",
            "speed300_bucket": ">=0.20",
            "pullback_bucket": "0.02-0.05",
            "donchian_gap_bucket": "0.40+",
        },
    ],
}


def _parse_schedule(raw: str) -> list[float]:
    return [float(item) for item in raw.split(",") if item.strip()]


CUSTOM_EXIT_PRESETS: dict[str, dict] = {
    # Research-only bridge back to btc_eth_micro_breakout_structure.py semantics:
    # disable the direct runner's early PT/trailing, then trail by completed
    # signal-bar structure once the position has at least 1 ATR of MFE.
    "structure1p0_b4": {
        "trailing_stop_atr": 0.5,
        "delayed_trailing_activation": 99.0,
        "profit_protect_atr": 99.0,
        "structure_start_atr": 1.0,
        "structure_bars": 4,
        "structure_buffer_atr": 0.05,
    },
    "structure1p0_b4_cost0p5": {
        "trailing_stop_atr": 0.5,
        "delayed_trailing_activation": 99.0,
        "profit_protect_atr": 99.0,
        "profit_lock_activation_atr": 0.5,
        "profit_lock_bps": 10.0,
        "structure_start_atr": 1.0,
        "structure_bars": 4,
        "structure_buffer_atr": 0.05,
    },
    "structure0p8_b4": {
        "trailing_stop_atr": 0.5,
        "delayed_trailing_activation": 99.0,
        "profit_protect_atr": 99.0,
        "structure_start_atr": 0.8,
        "structure_bars": 4,
        "structure_buffer_atr": 0.05,
    },
}


def _parse_exit_variants(raw_values: list[str] | None) -> list[dict]:
    policies: list[dict] = []
    for raw in raw_values or ["baseline"]:
        if raw in CUSTOM_EXIT_PRESETS:
            base = direct._base_exit_policy()
            base.update(CUSTOM_EXIT_PRESETS[raw])
            base["name"] = raw
            policies.append(base)
            continue
        policies.extend(direct._parse_exit_variants([raw]))
    return policies


def _structure_events(signal: pd.DataFrame, timeframe: str) -> dict[pd.Timestamp, pd.Series]:
    events = signal.copy()
    for lookback in (1, 2, 3, 4):
        events[f"struct_low_{lookback}"] = events["low"].rolling(lookback, min_periods=1).min()
        events[f"struct_high_{lookback}"] = events["high"].rolling(lookback, min_periods=1).max()
    offset = pd.tseries.frequencies.to_offset(timeframe)
    return {pd.Timestamp(idx + offset): row for idx, row in events.iterrows()}


def _apply_structure_exit_policy(position: dict, event: pd.Series) -> None:
    exit_policy = position.get("exit_policy", {})
    structure_start_atr = exit_policy.get("structure_start_atr")
    if structure_start_atr is None:
        return
    if float(position.get("mfe_atr", 0.0)) < float(structure_start_atr):
        return
    lookback = int(exit_policy.get("structure_bars", 4))
    buffer_atr = float(exit_policy.get("structure_buffer_atr", 0.05))
    atr = float(position.get("atr_at_entry", np.nan))
    if not np.isfinite(atr) or atr <= 0:
        return
    if position["side"] == "long":
        trail = float(event[f"struct_low_{lookback}"]) - buffer_atr * atr
        if np.isfinite(trail) and trail > float(position["sl"]):
            position["sl"] = trail
            position["structure_stop_active"] = True
    else:
        trail = float(event[f"struct_high_{lookback}"]) + buffer_atr * atr
        if np.isfinite(trail) and trail < float(position["sl"]):
            position["sl"] = trail
            position["structure_stop_active"] = True


def _state_matches(row: pd.Series, states: list[dict[str, str]]) -> bool:
    for state in states:
        matched = True
        for key, value in state.items():
            if str(row.get(key, "")) != str(value):
                matched = False
                break
        if matched:
            return True
    return False


def _signal_for_pos(
    signal_context: dict[pd.Timestamp, dict],
    second_index: pd.DatetimeIndex,
    bar_high_so_far: np.ndarray,
    bar_low_so_far: np.ndarray,
    close_values: np.ndarray,
    timeframe: str,
    pos: int,
) -> dict | None:
    bar_time = second_index[pos].floor(timeframe)
    base = signal_context.get(bar_time)
    if base is None or pd.isna(base.get("atr", np.nan)):
        return None
    sig = dict(base)
    sig["_closed_atr"] = float(base["atr"])
    return replay._intrabar_signal(
        sig,
        float(bar_high_so_far[pos]),
        float(bar_low_so_far[pos]),
        float(close_values[pos]),
    )


def _append_entry_log(
    logs: list[dict],
    position: dict,
    *,
    bar_time: pd.Timestamp,
    raw_entry: float,
    candidate: pd.Series,
    variant_name: str,
) -> dict:
    side = str(position["side"])
    level = float(candidate["level"])
    entry_vs_breakout_bps = (
        (float(raw_entry) - level) / level if side == "long" else (level - float(raw_entry)) / level
    ) * 10000.0
    log = {
        "time": bar_time,
        "type": "BUY" if side == "long" else "SHORT",
        "price": float(position["entry_p"]),
        "reason": "OriginalT2-PreTouchState",
        "notional": float(position["notional"]),
        "notional_share": float(position["notional_share"]),
        "bal": np.nan,
        "breakout_shape_name": "original_t2",
        "breakout_level": level,
        "observed_fill_raw": float(raw_entry),
        "signal_bar_time": candidate["signal_bar_time"],
        "trade_slot": int(position.get("trade_slot", 0)),
        "raw_exit_price": np.nan,
        "real_stop_price": float(position["sl"]),
        "entry_vs_breakout_bps": entry_vs_breakout_bps,
        "exit_policy_name": str(position.get("exit_policy", {}).get("name", "")),
        "variant": variant_name,
        "pre_candidate_time": candidate["time"],
        "pre_distance_atr": float(candidate["distance_atr"]),
        "pre_speed60_atr": float(candidate["speed60_atr"]),
        "pre_speed300_atr": float(candidate["speed300_atr"]),
        "pre_eff300": float(candidate["eff300"]),
        "pre_pullback300_atr": float(candidate["pullback300_atr"]),
        "donchian_level": float(candidate.get("donchian_level", np.nan)),
        "donchian_gap_atr": float(candidate.get("donchian_gap_atr", np.nan)),
        "distance_to_donchian_atr": float(candidate.get("distance_to_donchian_atr", np.nan)),
        "distance_bucket": str(candidate["distance_bucket"]),
        "speed300_bucket": str(candidate["speed300_bucket"]),
        "pullback_bucket": str(candidate["pullback_bucket"]),
        "donchian_gap_bucket": str(candidate.get("donchian_gap_bucket", "")),
    }
    logs.append(log)
    return log


def run_variant(
    *,
    second_bars: pd.DataFrame,
    signal: pd.DataFrame,
    candidates: pd.DataFrame,
    timeframe: str,
    variant_name: str,
    states: list[dict[str, str]],
    initial_balance: float,
    max_trades_per_bar: int,
    schedule: list[float],
    exit_policy: dict,
) -> dict:
    started = time.time()
    second_index = second_bars.index
    high_values = second_bars["high"].to_numpy(dtype="float64", copy=False)
    low_values = second_bars["low"].to_numpy(dtype="float64", copy=False)
    close_values = second_bars["close"].to_numpy(dtype="float64", copy=False)
    grouped = second_bars.groupby(second_bars.index.floor(timeframe), sort=False)
    bar_high_so_far = grouped["high"].cummax().to_numpy(dtype="float64", copy=False)
    bar_low_so_far = grouped["low"].cummin().to_numpy(dtype="float64", copy=False)
    signal_context = {ts: row.to_dict() for ts, row in signal.iterrows()}
    structure_events = _structure_events(signal, timeframe)

    state = {
        "balance": float(initial_balance),
        "position": None,
        "trade_logs": [],
        "diagnostics": {
            "matching_candidates": 0,
            "entries": 0,
            "skipped_position_open": 0,
            "skipped_entry_time_processed": 0,
            "skipped_max_trades_per_bar": 0,
            "skipped_bad_signal": 0,
            "skipped_no_second": 0,
            "skipped_zero_share": 0,
        },
    }
    entries_by_bar: dict[pd.Timestamp, int] = {}
    open_entry_log: dict | None = None
    current_floor_pos = 0

    filtered = candidates[candidates.apply(lambda row: _state_matches(row, states), axis=1)].copy()
    if filtered.empty:
        ledger = pd.DataFrame()
    else:
        filtered["entry_time"] = pd.to_datetime(filtered["time"], utc=True) + pd.Timedelta(minutes=1)
        filtered.sort_values(["entry_time", "side"], inplace=True)

    for _, candidate in filtered.iterrows():
        state["diagnostics"]["matching_candidates"] += 1
        entry_pos = int(second_index.searchsorted(pd.Timestamp(candidate["entry_time"]), side="left"))
        if entry_pos >= len(second_index):
            state["diagnostics"]["skipped_no_second"] += 1
            continue
        if entry_pos < current_floor_pos:
            state["diagnostics"]["skipped_entry_time_processed"] += 1
            continue
        signal_bar_time = pd.Timestamp(candidate["signal_bar_time"])
        trades_in_bar = int(entries_by_bar.get(signal_bar_time, 0))
        if trades_in_bar >= int(max_trades_per_bar):
            state["diagnostics"]["skipped_max_trades_per_bar"] += 1
            continue
        if state["position"] is not None:
            state["diagnostics"]["skipped_position_open"] += 1
            continue
        notional_share = float(schedule[min(trades_in_bar, len(schedule) - 1)]) if schedule else 0.0
        if notional_share <= 0:
            state["diagnostics"]["skipped_zero_share"] += 1
            continue
        sig = _signal_for_pos(signal_context, second_index, bar_high_so_far, bar_low_so_far, close_values, timeframe, entry_pos)
        if sig is None:
            state["diagnostics"]["skipped_bad_signal"] += 1
            continue
        side = str(candidate["side"])
        raw_entry = float(close_values[entry_pos])
        state["balance"], state["position"] = direct._open_direct_position(
            state["balance"],
            sig,
            side=side,
            fill_raw=raw_entry,
            notional_share=notional_share,
            shape_name="original_t2",
            breakout_level=float(candidate["level"]),
            signal_bar_time=signal_bar_time,
            trade_slot=trades_in_bar,
            exit_policy=exit_policy,
        )
        position = state["position"]
        position["trade_slot"] = trades_in_bar
        position["notional_share"] = notional_share
        open_entry_log = _append_entry_log(
            state["trade_logs"],
            position,
            bar_time=second_index[entry_pos],
            raw_entry=raw_entry,
            candidate=candidate,
            variant_name=variant_name,
        )
        entries_by_bar[signal_bar_time] = trades_in_bar + 1
        state["diagnostics"]["entries"] += 1

        exit_pos = None
        for pos in range(entry_pos + 1, len(second_index)):
            sig = _signal_for_pos(signal_context, second_index, bar_high_so_far, bar_low_so_far, close_values, timeframe, pos)
            if sig is None:
                continue
            exit_triggered, raw_exit_price, reason = direct._advance_position(
                state["position"],
                sig,
                float(high_values[pos]),
                float(low_values[pos]),
            )
            if exit_triggered:
                if state["position"].get("structure_stop_active", False) and reason == "InitialSL":
                    reason = "StructureSL"
                before = len(state["trade_logs"])
                direct._append_exit(
                    state,
                    state["position"],
                    bar_time=second_index[pos],
                    raw_exit_price=raw_exit_price,
                    reason=reason,
                )
                if len(state["trade_logs"]) > before and open_entry_log is not None:
                    for key in (
                        "variant",
                        "pre_candidate_time",
                        "pre_distance_atr",
                        "pre_speed60_atr",
                        "pre_speed300_atr",
                        "pre_eff300",
                        "pre_pullback300_atr",
                        "donchian_level",
                        "donchian_gap_atr",
                        "distance_to_donchian_atr",
                        "distance_bucket",
                        "speed300_bucket",
                        "pullback_bucket",
                        "donchian_gap_bucket",
                    ):
                        state["trade_logs"][-1][key] = open_entry_log.get(key, np.nan)
                state["position"] = None
                open_entry_log = None
                exit_pos = pos
                break
            event = structure_events.get(second_index[pos])
            if event is not None:
                _apply_structure_exit_policy(state["position"], event)
        if exit_pos is None:
            break
        current_floor_pos = exit_pos + 1

    if state["position"] is not None and len(close_values) > 0:
        before = len(state["trade_logs"])
        direct._append_exit(
            state,
            state["position"],
            bar_time=second_index[-1],
            raw_exit_price=float(close_values[-1]),
            reason="FinalMarkToMarket",
        )
        if len(state["trade_logs"]) > before and open_entry_log is not None:
            for key in (
                "variant",
                "pre_candidate_time",
                "pre_distance_atr",
                "pre_speed300_atr",
                "donchian_gap_atr",
                "distance_bucket",
                "speed300_bucket",
                "pullback_bucket",
                "donchian_gap_bucket",
            ):
                state["trade_logs"][-1][key] = open_entry_log.get(key, np.nan)
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
        "states": states,
        "max_trades_per_bar": int(max_trades_per_bar),
        "reentry_size_schedule": schedule,
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


def write_markdown(summary: dict, path: Path) -> None:
    lines = [
        f"# {summary['symbol']} original_t2 pre-touch 状态入场回测（{summary['start']} 至 {summary['end']}）",
        "",
        "范围：仅限 `research`。本回测消费 true `original_t2` pre-touch 样本表，只在高 proxy edge 分箱出现时，于下一根 `1s close` 市价入场。退出默认沿用 direct-breakout baseline 的 `InitialSL/TrailingSL/PT` 逻辑；`structure*` exit variant 会在达到配置 MFE 后，改用已闭合 signal bar 的结构低/高移动止损。",
        "",
        "成本：滑点 `2bps/side`，手续费 maker entry `2bps` + market exit `4bps`，realistic 包含两者。",
        "",
        "| Variant | States | 笔数 | Realistic | Raw | 2bps Slip No Fee | Fees | 胜率 | Max DD | Avg Hold | Median Hold | Max Entries/Bar | Exit Reasons |",
        "|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|",
    ]
    for result in summary["results"]:
        acct = result["accounting_2bps_maker_entry_market_exit"]
        s = result["summary"]
        d = result["pair_diagnostics"]
        slot0 = result.get("trade_slot_diagnostics", {}).get("0", {})
        win_rate = float(slot0.get("win_rate_pct", s.get("win_rate_pct", 0.0)))
        lines.append(
            f"| `{result['variant']}` | `{result['states']}` | {acct['round_trips']} | "
            f"{acct['realistic_return_pct']:.4f}% | {acct['raw_no_fee_no_slip_return_pct']:.4f}% | "
            f"{acct['price_pnl_with_2bps_slip_no_fee_return_pct']:.4f}% | {acct['realistic_fees_pct']:.4f}% | "
            f"{win_rate:.2f}% | {s['max_dd_pct']:.2f}% | {d.get('avg_hold_seconds', 0.0):.2f}s | "
            f"{d.get('median_hold_seconds', 0.0):.2f}s | {result['bar_guard_diagnostics']['max_entries_per_bar']} | "
            f"`{s['exit_reasons']}` |"
        )
    lines.extend(["", "## 文件", ""])
    lines.append(f"- Summary JSON：`{summary['summary_path']}`")
    for result in summary["results"]:
        lines.append(f"- `{result['variant']}` ledger：`{result['ledger_path']}`")
    lines.append("")
    path.write_text("\n".join(lines), encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="ETH original_t2 pre-touch entry replay")
    parser.add_argument("--symbol", default="ETHUSDT")
    parser.add_argument("--start", default="2026-01-01T00:00:00Z")
    parser.add_argument("--end", default="2026-04-30T23:59:59Z")
    parser.add_argument("--timeframe", default="1h")
    parser.add_argument("--archive-root", default=str(micro.DEFAULT_ARCHIVE_ROOT))
    parser.add_argument("--cache-root", default=str(micro.DEFAULT_CACHE_ROOT))
    parser.add_argument("--no-cache", action="store_true")
    parser.add_argument("--chunksize", type=int, default=2_000_000)
    parser.add_argument("--candidate-csv", default="research/tmp_eth_2026_jan_apr_1h_original_t2_pretouch_continuation_candidates.csv")
    parser.add_argument("--initial-balance", type=float, default=100000.0)
    parser.add_argument("--stop-loss-atr", type=float, default=0.30)
    parser.add_argument("--max-trades-per-bar", type=int, default=1)
    parser.add_argument("--schedule", default="0.20")
    parser.add_argument("--variants", nargs="+", default=["fast_clean", "fast_clean_or_small_pullback", "edge10_c1f03", "edge8_c05f03"])
    parser.add_argument(
        "--exit-variants",
        nargs="+",
        default=["baseline"],
        help=(
            "Exit policy presets from btc_2026_jan_apr_direct_breakout.py plus research structure exits, "
            "e.g. baseline cost0p5 lock1p0_20bps trail0p5_act1p0 tp1p0 structure1p0_b4."
        ),
    )
    parser.add_argument("--summary-json", default="research/eth_original_t2_pretouch_entry_summary.json")
    parser.add_argument("--markdown", default="research/20260508_eth_original_t2_pretouch_entry.md")
    parser.add_argument("--ledger-prefix", default="research/tmp_eth_original_t2_pretouch_entry")
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
            "max_trades_per_bar": int(args.max_trades_per_bar),
            "reentry_size_schedule": _parse_schedule(args.schedule),
        }
    )
    exit_policies = _parse_exit_variants(args.exit_variants)
    candidates = pd.read_csv(args.candidate_csv)
    candidates["time"] = pd.to_datetime(candidates["time"], utc=True)
    candidates["signal_bar_time"] = pd.to_datetime(candidates["signal_bar_time"], utc=True)
    second_bars, build_stats = micro.load_or_build_second_bars(
        args.symbol,
        start,
        end,
        Path(args.archive_root),
        args.chunksize,
        Path(args.cache_root),
        not args.no_cache,
    )
    _, signal = replay.build_signal_frame(second_bars, args.timeframe)
    print(
        f"{args.symbol} second_rows={len(second_bars)} signal_rows={len(signal)} candidates={len(candidates)}",
        flush=True,
    )

    results = []
    for variant_name in args.variants:
        states = PRESET_STATES.get(variant_name)
        if states is None:
            raise ValueError(f"unknown variant {variant_name}; presets={sorted(PRESET_STATES)}")
        for exit_policy in exit_policies:
            policy_name = str(exit_policy.get("name", ""))
            result_name = variant_name if policy_name in {"", "baseline"} else f"{variant_name}_{policy_name}"
            result = run_variant(
                second_bars=second_bars,
                signal=signal,
                candidates=candidates,
                timeframe=args.timeframe,
                variant_name=result_name,
                states=states,
                initial_balance=float(args.initial_balance),
                max_trades_per_bar=int(args.max_trades_per_bar),
                schedule=_parse_schedule(args.schedule),
                exit_policy=exit_policy,
            )
            ledger_path = Path(f"{args.ledger_prefix}_{args.timeframe}_{result_name}_ledger.csv")
            result["ledger"].to_csv(ledger_path, index=False)
            del result["ledger"]
            result["ledger_path"] = str(ledger_path)
            acct = result["accounting_2bps_maker_entry_market_exit"]
            print(
                f"{result_name}: realistic={acct['realistic_return_pct']:.4f}% "
                f"raw={acct['raw_no_fee_no_slip_return_pct']:.4f}% trades={acct['round_trips']} "
                f"diag={result['diagnostics']}",
                flush=True,
            )
            results.append(result)

    summary = {
        "symbol": args.symbol,
        "start": start.isoformat(),
        "end": end.isoformat(),
        "timeframe": args.timeframe,
        "build_stats": build_stats,
        "candidate_csv": args.candidate_csv,
        "mode": {
            "breakout_shape": "original_t2",
            "entry": "pre-touch candidate state, filled at next 1s close",
            "max_trades_per_bar": int(args.max_trades_per_bar),
            "schedule": _parse_schedule(args.schedule),
            "stop_loss_atr": float(args.stop_loss_atr),
        },
        "results": results,
        "summary_path": args.summary_json,
        "markdown_path": args.markdown,
        "elapsed_seconds": round(time.time() - started, 2),
    }
    Path(args.summary_json).write_text(json.dumps(summary, indent=2, ensure_ascii=False), encoding="utf-8")
    write_markdown(summary, Path(args.markdown))
    print(
        json.dumps({"summary_path": args.summary_json, "markdown_path": args.markdown, "elapsed_seconds": summary["elapsed_seconds"]}, indent=2),
        flush=True,
    )


if __name__ == "__main__":
    main()
