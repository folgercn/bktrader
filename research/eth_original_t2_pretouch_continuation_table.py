#!/usr/bin/env python3
"""ETH original_t2 pre-touch continuation state table.

Research-only. This script keeps the live/research original_t2 semantics:
long level is `prev_high_2`, short level is `prev_low_2`, and the current
signal bar is still open. It samples pre-touch 1m states while execution labels
are resolved with continuous 1s high/low bars.
"""

from __future__ import annotations

import argparse
import json
import time
from pathlib import Path

import numpy as np
import pandas as pd

import btc_eth_micro_breakout_structure as micro
import eth_q1_breakout_t3_shape_compare as replay


def _bucket(value: float, edges: list[float], labels: list[str]) -> str:
    if not np.isfinite(value):
        return "nan"
    for edge, label in zip(edges, labels):
        if value < edge:
            return label
    return labels[-1]


def _first_true(mask: np.ndarray) -> int | None:
    hits = np.flatnonzero(mask)
    if len(hits) == 0:
        return None
    return int(hits[0])


def _last_close_at(closes: np.ndarray, pos: int, fallback: float) -> float:
    if len(closes) == 0:
        return float(fallback)
    safe_pos = min(max(int(pos), 0), len(closes) - 1)
    return float(closes[safe_pos])


def build_minute_context(second_bars: pd.DataFrame, signal_timeframe: str) -> tuple[pd.DataFrame, pd.DataFrame]:
    one_min, signal = replay.build_signal_frame(second_bars, signal_timeframe)
    signal["prev_high_8"] = signal["high"].shift(1).rolling(8).max()
    signal["prev_low_8"] = signal["low"].shift(1).rolling(8).min()
    minute = one_min.copy()
    minute["signal_bar_time"] = minute.index.floor(signal_timeframe)
    minute["minute_start"] = minute.index
    minute["ret_60s"] = minute["close"] - minute["close"].shift(1)
    minute["ret_300s"] = minute["close"] - minute["close"].shift(5)
    minute["ret_900s"] = minute["close"] - minute["close"].shift(15)
    minute["range_high_300s"] = minute["high"].rolling(5, min_periods=2).max()
    minute["range_low_300s"] = minute["low"].rolling(5, min_periods=2).min()
    minute["range_300s"] = (minute["range_high_300s"] - minute["range_low_300s"]).replace(0.0, np.nan)
    minute["eff_300s"] = minute["ret_300s"] / minute["range_300s"]
    minute["close_pos_300s"] = (minute["close"] - minute["range_low_300s"]) / minute["range_300s"]
    minute["range_high_900s"] = minute["high"].rolling(15, min_periods=5).max()
    minute["range_low_900s"] = minute["low"].rolling(15, min_periods=5).min()
    minute["range_900s"] = (minute["range_high_900s"] - minute["range_low_900s"]).replace(0.0, np.nan)
    minute["eff_900s"] = minute["ret_900s"] / minute["range_900s"]
    volume_mean = minute["volume"].shift(1).rolling(60, min_periods=20).mean()
    volume_std = minute["volume"].shift(1).rolling(60, min_periods=20).std()
    minute["volume_z60m"] = (minute["volume"] - volume_mean) / volume_std.replace(0.0, np.nan)
    minute["bar_minute"] = minute.groupby("signal_bar_time").cumcount()
    minute["bar_high_so_far"] = minute.groupby("signal_bar_time")["high"].cummax()
    minute["bar_low_so_far"] = minute.groupby("signal_bar_time")["low"].cummin()
    return minute, signal


def _state_buckets(row: dict) -> dict:
    return {
        "distance_bucket": _bucket(
            float(row["distance_atr"]),
            [0.05, 0.10, 0.15, 0.20, 0.30],
            ["0-0.05", "0.05-0.10", "0.10-0.15", "0.15-0.20", "0.20-0.30", "0.30+"],
        ),
        "speed60_bucket": _bucket(
            float(row["speed60_atr"]),
            [-0.03, 0.0, 0.03, 0.08, 0.15],
            ["<-0.03", "-0.03-0", "0-0.03", "0.03-0.08", "0.08-0.15", ">=0.15"],
        ),
        "speed300_bucket": _bucket(
            float(row["speed300_atr"]),
            [-0.08, -0.02, 0.03, 0.10, 0.20],
            ["<-0.08", "-0.08--0.02", "-0.02-0.03", "0.03-0.10", "0.10-0.20", ">=0.20"],
        ),
        "eff300_bucket": _bucket(
            float(row["eff300"]),
            [0.0, 0.20, 0.40, 0.60, 0.80],
            ["<0", "0-0.20", "0.20-0.40", "0.40-0.60", "0.60-0.80", ">=0.80"],
        ),
        "close_pos_bucket": _bucket(
            float(row["close_pos300"]),
            [0.20, 0.40, 0.60, 0.80],
            ["0-0.20", "0.20-0.40", "0.40-0.60", "0.60-0.80", "0.80-1.00"],
        ),
        "pullback_bucket": _bucket(
            float(row["pullback300_atr"]),
            [0.02, 0.05, 0.10, 0.20],
            ["0-0.02", "0.02-0.05", "0.05-0.10", "0.10-0.20", ">=0.20"],
        ),
        "bar_progress_bucket": _bucket(
            float(row["bar_progress"]),
            [0.20, 0.40, 0.60, 0.80],
            ["0-20%", "20-40%", "40-60%", "60-80%", "80-100%"],
        ),
        "donchian_gap_bucket": _bucket(
            float(row["donchian_gap_atr"]),
            [0.02, 0.05, 0.10, 0.20, 0.40],
            ["0-0.02", "0.02-0.05", "0.05-0.10", "0.10-0.20", "0.20-0.40", "0.40+"],
        ),
    }


def _candidate_from_minute(row: pd.Series, sig: dict, side: str, level: float, args: argparse.Namespace) -> dict | None:
    atr = float(sig.get("atr", np.nan))
    close = float(row["close"])
    if not np.isfinite(atr) or atr <= 0 or not np.isfinite(level) or level <= 0:
        return None
    if side == "long":
        if float(row["bar_high_so_far"]) >= level or close >= level:
            return None
        distance = (level - close) / atr
        side_mult = 1.0
        close_pos = float(row.get("close_pos_300s", np.nan))
        pullback = (float(row.get("range_high_300s", np.nan)) - close) / atr
    else:
        if float(row["bar_low_so_far"]) <= level or close <= level:
            return None
        distance = (close - level) / atr
        side_mult = -1.0
        raw_close_pos = float(row.get("close_pos_300s", np.nan))
        close_pos = 1.0 - raw_close_pos if np.isfinite(raw_close_pos) else np.nan
        pullback = (close - float(row.get("range_low_300s", np.nan))) / atr

    if distance < float(args.min_distance_atr) or distance > float(args.max_distance_atr):
        return None

    out = {
        "time": row["minute_start"],
        "signal_bar_time": row["signal_bar_time"],
        "side": side,
        "level": float(level),
        "close": close,
        "atr": atr,
        "distance_atr": float(distance),
        "speed60_atr": side_mult * float(row.get("ret_60s", np.nan)) / atr,
        "speed300_atr": side_mult * float(row.get("ret_300s", np.nan)) / atr,
        "speed900_atr": side_mult * float(row.get("ret_900s", np.nan)) / atr,
        "eff300": side_mult * float(row.get("eff_300s", np.nan)),
        "eff900": side_mult * float(row.get("eff_900s", np.nan)),
        "close_pos300": close_pos,
        "pullback300_atr": float(pullback),
        "range300_atr": float(row.get("range_300s", np.nan)) / atr,
        "volume_z60m": float(row.get("volume_z60m", np.nan)),
        "bar_minute": int(row.get("bar_minute", 0)),
        "bar_progress": float(row.get("bar_minute", 0)) / max(1.0, float(pd.Timedelta(args.signal_timeframe).seconds // 60 or 60)),
    }
    if side == "long":
        donchian_level = float(sig.get("prev_high_8", np.nan))
        out["donchian_level"] = donchian_level
        out["donchian_gap_atr"] = max(0.0, (donchian_level - level) / atr) if np.isfinite(donchian_level) else np.nan
        out["distance_to_donchian_atr"] = (donchian_level - close) / atr if np.isfinite(donchian_level) else np.nan
    else:
        donchian_level = float(sig.get("prev_low_8", np.nan))
        out["donchian_level"] = donchian_level
        out["donchian_gap_atr"] = max(0.0, (level - donchian_level) / atr) if np.isfinite(donchian_level) else np.nan
        out["distance_to_donchian_atr"] = (close - donchian_level) / atr if np.isfinite(donchian_level) else np.nan
    out.update(_state_buckets(out))
    return out


def collect_candidates(minute: pd.DataFrame, signal: pd.DataFrame, args: argparse.Namespace) -> pd.DataFrame:
    rows: list[dict] = []
    seen: set[tuple] = set()
    signal_context = {ts: row.to_dict() for ts, row in signal.iterrows()}
    for _, row in minute.iterrows():
        bar_time = pd.Timestamp(row["signal_bar_time"])
        base = signal_context.get(bar_time)
        if base is None or pd.isna(base.get("atr", np.nan)):
            continue
        sig = dict(base)
        sig["_closed_atr"] = float(base["atr"])
        sig = replay._intrabar_signal(
            sig,
            float(row["bar_high_so_far"]),
            float(row["bar_low_so_far"]),
            float(row["close"]),
        )
        long_ready, short_ready = replay._resolve_regime_ready(sig, "1d")
        candidates: list[dict] = []
        if long_ready:
            p1 = sig.get("prev_high_1", np.nan)
            p2 = sig.get("prev_high_2", np.nan)
            if pd.notna(p1) and pd.notna(p2) and float(p2) > float(p1):
                candidate = _candidate_from_minute(row, sig, "long", float(p2), args)
                if candidate is not None:
                    candidates.append(candidate)
        if short_ready:
            p1 = sig.get("prev_low_1", np.nan)
            p2 = sig.get("prev_low_2", np.nan)
            if pd.notna(p1) and pd.notna(p2) and float(p2) < float(p1):
                candidate = _candidate_from_minute(row, sig, "short", float(p2), args)
                if candidate is not None:
                    candidates.append(candidate)

        for candidate in candidates:
            if int(args.dedupe_one_per_bar_side_bucket) > 0:
                key = (
                    pd.Timestamp(candidate["signal_bar_time"]),
                    str(candidate["side"]),
                    str(candidate["distance_bucket"]),
                )
                if key in seen:
                    continue
                seen.add(key)
            rows.append(candidate)
    return pd.DataFrame(rows)


def label_candidates(candidates: pd.DataFrame, second_bars: pd.DataFrame, args: argparse.Namespace) -> pd.DataFrame:
    if candidates.empty:
        return candidates
    idx = second_bars.index
    highs = second_bars["high"].to_numpy(dtype="float64", copy=False)
    lows = second_bars["low"].to_numpy(dtype="float64", copy=False)
    closes = second_bars["close"].to_numpy(dtype="float64", copy=False)
    signal_offset = pd.tseries.frequencies.to_offset(args.signal_timeframe)
    continuation_atrs = [float(v) for v in args.continuation_atrs]
    post_fail_atrs = [float(v) for v in args.post_fail_atrs]
    pre_fail_atr = float(args.pre_fail_atr)

    labelled: list[dict] = []
    for row in candidates.to_dict("records"):
        start_time = pd.Timestamp(row["time"]) + pd.Timedelta(minutes=1)
        signal_end = pd.Timestamp(row["signal_bar_time"]) + signal_offset
        pre_end_time = min(signal_end, start_time + pd.Timedelta(minutes=int(args.pre_horizon_minutes)))
        start_pos = int(idx.searchsorted(start_time, side="left"))
        pre_end_pos = min(int(idx.searchsorted(pre_end_time, side="right")), len(idx))
        side = str(row["side"])
        entry = float(row["close"])
        level = float(row["level"])
        atr = float(row["atr"])
        if start_pos >= pre_end_pos:
            touch_pos = None
            pre_fail_pos = None
        elif side == "long":
            pre_fail_level = entry - pre_fail_atr * atr
            pre_fail_pos = _first_true(lows[start_pos:pre_end_pos] <= pre_fail_level)
            touch_pos = _first_true(highs[start_pos:pre_end_pos] >= level)
        else:
            pre_fail_level = entry + pre_fail_atr * atr
            pre_fail_pos = _first_true(highs[start_pos:pre_end_pos] >= pre_fail_level)
            touch_pos = _first_true(lows[start_pos:pre_end_pos] <= level)

        touch_abs = None if touch_pos is None else start_pos + int(touch_pos)
        pre_fail_abs = None if pre_fail_pos is None else start_pos + int(pre_fail_pos)
        row["touch_seconds"] = np.nan if touch_abs is None else float((idx[touch_abs] - start_time).total_seconds())
        row["pre_touch_outcome"] = "touch"
        if touch_abs is None:
            row["pre_touch_outcome"] = "pre_timeout"
        if pre_fail_abs is not None and (touch_abs is None or pre_fail_abs <= touch_abs):
            row["pre_touch_outcome"] = "pre_fail"

        for continuation_atr in continuation_atrs:
            for post_fail_atr in post_fail_atrs:
                key = f"c{str(continuation_atr).replace('.', 'p')}_f{str(post_fail_atr).replace('.', 'p')}"
                if row["pre_touch_outcome"] != "touch" or touch_abs is None:
                    if row["pre_touch_outcome"] == "pre_fail":
                        outcome = "pre_fail"
                        exit_raw = entry - pre_fail_atr * atr if side == "long" else entry + pre_fail_atr * atr
                    else:
                        outcome = "pre_timeout"
                        exit_raw = _last_close_at(closes, pre_end_pos - 1, entry)
                    event_seconds = np.nan
                    continuation_seconds = np.nan
                else:
                    post_end_time = min(signal_end, idx[touch_abs] + pd.Timedelta(minutes=int(args.post_horizon_minutes)))
                    post_end_pos = min(int(idx.searchsorted(post_end_time, side="right")), len(idx))
                    if side == "long":
                        cont_level = level + continuation_atr * atr
                        post_fail_level = level - post_fail_atr * atr
                        cont_pos = _first_true(highs[touch_abs:post_end_pos] >= cont_level)
                        post_fail_pos = _first_true(lows[touch_abs:post_end_pos] <= post_fail_level)
                        if cont_pos is not None and (post_fail_pos is None or cont_pos < post_fail_pos):
                            outcome = "continuation"
                            exit_raw = cont_level
                            continuation_seconds = float((idx[touch_abs + cont_pos] - idx[touch_abs]).total_seconds())
                        elif post_fail_pos is not None:
                            outcome = "post_fail"
                            exit_raw = post_fail_level
                            continuation_seconds = np.nan
                        else:
                            outcome = "post_timeout"
                            exit_raw = _last_close_at(closes, post_end_pos - 1, entry)
                            continuation_seconds = np.nan
                    else:
                        cont_level = level - continuation_atr * atr
                        post_fail_level = level + post_fail_atr * atr
                        cont_pos = _first_true(lows[touch_abs:post_end_pos] <= cont_level)
                        post_fail_pos = _first_true(highs[touch_abs:post_end_pos] >= post_fail_level)
                        if cont_pos is not None and (post_fail_pos is None or cont_pos < post_fail_pos):
                            outcome = "continuation"
                            exit_raw = cont_level
                            continuation_seconds = float((idx[touch_abs + cont_pos] - idx[touch_abs]).total_seconds())
                        elif post_fail_pos is not None:
                            outcome = "post_fail"
                            exit_raw = post_fail_level
                            continuation_seconds = np.nan
                        else:
                            outcome = "post_timeout"
                            exit_raw = _last_close_at(closes, post_end_pos - 1, entry)
                            continuation_seconds = np.nan
                    event_seconds = (
                        np.nan
                        if touch_abs is None
                        else float((idx[min(post_end_pos - 1, len(idx) - 1)] - start_time).total_seconds())
                    )
                side_mult = 1.0 if side == "long" else -1.0
                row[f"{key}_outcome"] = outcome
                row[f"{key}_return_bps"] = side_mult * (float(exit_raw) - entry) / entry * 10000.0 if entry > 0 else np.nan
                row[f"{key}_event_seconds"] = event_seconds
                row[f"{key}_continuation_seconds"] = continuation_seconds
        labelled.append(row)
    return pd.DataFrame(labelled)


def _summarize_group(group: pd.DataFrame, outcome_col: str, return_col: str) -> dict:
    returns = pd.to_numeric(group[return_col], errors="coerce")
    outcomes = group[outcome_col].astype(str)
    return {
        "samples": int(len(group)),
        "continuation_rate_pct": round(float((outcomes == "continuation").mean()) * 100.0, 4),
        "pre_fail_rate_pct": round(float((outcomes == "pre_fail").mean()) * 100.0, 4),
        "post_fail_rate_pct": round(float((outcomes == "post_fail").mean()) * 100.0, 4),
        "timeout_rate_pct": round(float(outcomes.str.contains("timeout", regex=False).mean()) * 100.0, 4),
        "avg_return_bps": round(float(returns.mean()), 4),
        "median_return_bps": round(float(returns.median()), 4),
        "p25_return_bps": round(float(returns.quantile(0.25)), 4),
        "p75_return_bps": round(float(returns.quantile(0.75)), 4),
    }


def summarize(candidates: pd.DataFrame, args: argparse.Namespace) -> dict:
    results = []
    if candidates.empty:
        return {"results": results}
    state_cols = ["distance_bucket", "speed60_bucket", "speed300_bucket", "eff300_bucket", "pullback_bucket"]
    compact_state_cols = ["distance_bucket", "speed300_bucket", "pullback_bucket"]
    hybrid_state_cols = ["distance_bucket", "speed300_bucket", "pullback_bucket", "donchian_gap_bucket"]
    dimension_cols = [
        "distance_bucket",
        "speed60_bucket",
        "speed300_bucket",
        "eff300_bucket",
        "close_pos_bucket",
        "pullback_bucket",
        "donchian_gap_bucket",
    ]
    for continuation_atr in [float(v) for v in args.continuation_atrs]:
        for post_fail_atr in [float(v) for v in args.post_fail_atrs]:
            key = f"c{str(continuation_atr).replace('.', 'p')}_f{str(post_fail_atr).replace('.', 'p')}"
            outcome_col = f"{key}_outcome"
            return_col = f"{key}_return_bps"
            combo = {
                "key": key,
                "continuation_atr": continuation_atr,
                "post_fail_atr": post_fail_atr,
                "overall": _summarize_group(candidates, outcome_col, return_col),
                "outcome_counts": {str(k): int(v) for k, v in candidates[outcome_col].value_counts().items()},
                "top_states": [],
                "top_compact_states": [],
                "top_hybrid_states": [],
                "by_side_distance": [],
                "by_dimension": {},
            }
            for cols, target_name, limit in (
                (state_cols, "top_states", int(args.top_state_limit)),
                (compact_state_cols, "top_compact_states", int(args.top_state_limit)),
                (hybrid_state_cols, "top_hybrid_states", int(args.top_state_limit)),
            ):
                grouped = []
                for values, group in candidates.groupby(cols, dropna=False):
                    if len(group) < int(args.min_state_samples):
                        continue
                    row = {col: str(value) for col, value in zip(cols, values if isinstance(values, tuple) else (values,))}
                    row.update(_summarize_group(group, outcome_col, return_col))
                    grouped.append(row)
                grouped.sort(key=lambda r: (float(r["avg_return_bps"]), float(r["continuation_rate_pct"]), int(r["samples"])), reverse=True)
                combo[target_name] = grouped[:limit]

            grouped = []
            for values, group in candidates.groupby(["side", "distance_bucket"], dropna=False):
                row = {"side": str(values[0]), "distance_bucket": str(values[1])}
                row.update(_summarize_group(group, outcome_col, return_col))
                grouped.append(row)
            grouped.sort(key=lambda r: (str(r["side"]), str(r["distance_bucket"])))
            combo["by_side_distance"] = grouped

            for col in dimension_cols:
                rows = []
                for value, group in candidates.groupby(col, dropna=False):
                    if len(group) < max(5, int(args.min_state_samples) // 2):
                        continue
                    row = {col: str(value)}
                    row.update(_summarize_group(group, outcome_col, return_col))
                    rows.append(row)
                rows.sort(key=lambda r: float(r["avg_return_bps"]), reverse=True)
                combo["by_dimension"][col] = rows
            results.append(combo)
    return {"results": results}


def write_markdown(summary: dict, path: Path) -> None:
    lines = [
        f"# {summary['symbol']} original_t2 pre-touch 延续概率表（{summary['start']} 至 {summary['end']}）",
        "",
        "范围：仅限 `research`。本报告使用真正 `original_t2`：long level 为 `prev_high_2`，short level 为 `prev_low_2`；当前 1h signal bar 未闭合。候选点来自尚未 touch 的 1m close，后续标签使用连续 `1s high/low` 判定。",
        "",
        f"- 候选距离：`{summary['min_distance_atr']}` 到 `{summary['max_distance_atr']}` ATR",
        f"- pre-fail：`{summary['pre_fail_atr']}` ATR",
        f"- 成本参考：滑点 `2bps/side` + 手续费 `6bps`，约 `10bps/notional`",
        f"- 候选样本：`{summary['candidate_count']}`",
        f"- 去重：`dedupe_one_per_bar_side_bucket={summary['dedupe_one_per_bar_side_bucket']}`",
        "",
        "## Overall",
        "",
        "| Label | Samples | Continuation | Avg Return | Median Return | PreFail | PostFail | Timeout | Outcome Counts |",
        "|---|---:|---:|---:|---:|---:|---:|---:|---|",
    ]
    for result in summary["summary"]["results"]:
        overall = result["overall"]
        lines.append(
            f"| `{result['key']}` | {overall['samples']} | {overall['continuation_rate_pct']:.2f}% | "
            f"{overall['avg_return_bps']:.4f}bps | {overall['median_return_bps']:.4f}bps | "
            f"{overall['pre_fail_rate_pct']:.2f}% | {overall['post_fail_rate_pct']:.2f}% | "
            f"{overall['timeout_rate_pct']:.2f}% | `{result['outcome_counts']}` |"
        )

    for result in summary["summary"]["results"]:
        lines.extend(["", f"## Top Compact States `{result['key']}`", ""])
        lines.append("| State | Samples | Continuation | Avg Return | Median Return | PreFail | PostFail | Timeout |")
        lines.append("|---|---:|---:|---:|---:|---:|---:|---:|")
        for row in result["top_compact_states"][:20]:
            state = f"dist={row['distance_bucket']}, speed300={row['speed300_bucket']}, pullback={row['pullback_bucket']}"
            lines.append(
                f"| `{state}` | {row['samples']} | {row['continuation_rate_pct']:.2f}% | "
                f"{row['avg_return_bps']:.4f}bps | {row['median_return_bps']:.4f}bps | "
                f"{row['pre_fail_rate_pct']:.2f}% | {row['post_fail_rate_pct']:.2f}% | {row['timeout_rate_pct']:.2f}% |"
            )

        lines.extend(["", f"## Top Hybrid States `{result['key']}`", ""])
        lines.append("| State | Samples | Continuation | Avg Return | Median Return | PreFail | PostFail | Timeout |")
        lines.append("|---|---:|---:|---:|---:|---:|---:|---:|")
        for row in result["top_hybrid_states"][:20]:
            state = (
                f"dist={row['distance_bucket']}, speed300={row['speed300_bucket']}, "
                f"pullback={row['pullback_bucket']}, d8gap={row['donchian_gap_bucket']}"
            )
            lines.append(
                f"| `{state}` | {row['samples']} | {row['continuation_rate_pct']:.2f}% | "
                f"{row['avg_return_bps']:.4f}bps | {row['median_return_bps']:.4f}bps | "
                f"{row['pre_fail_rate_pct']:.2f}% | {row['post_fail_rate_pct']:.2f}% | {row['timeout_rate_pct']:.2f}% |"
            )

        lines.extend(["", f"## Side x Distance `{result['key']}`", ""])
        lines.append("| Side | Distance | Samples | Continuation | Avg Return | Median Return |")
        lines.append("|---|---|---:|---:|---:|---:|")
        for row in result["by_side_distance"]:
            lines.append(
                f"| `{row['side']}` | `{row['distance_bucket']}` | {row['samples']} | "
                f"{row['continuation_rate_pct']:.2f}% | {row['avg_return_bps']:.4f}bps | {row['median_return_bps']:.4f}bps |"
            )

    lines.extend(["", "## 文件", ""])
    lines.append(f"- Summary JSON：`{summary['summary_path']}`")
    lines.append(f"- Candidate CSV：`{summary['candidate_csv']}`")
    lines.append("")
    path.write_text("\n".join(lines), encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="ETH original_t2 pre-touch continuation table")
    parser.add_argument("--symbol", default="ETHUSDT")
    parser.add_argument("--start", default="2026-01-01T00:00:00Z")
    parser.add_argument("--end", default="2026-04-30T23:59:59Z")
    parser.add_argument("--signal-timeframe", default="1h")
    parser.add_argument("--archive-root", default=str(micro.DEFAULT_ARCHIVE_ROOT))
    parser.add_argument("--cache-root", default=str(micro.DEFAULT_CACHE_ROOT))
    parser.add_argument("--no-cache", action="store_true")
    parser.add_argument("--chunksize", type=int, default=2_000_000)
    parser.add_argument("--min-distance-atr", type=float, default=0.05)
    parser.add_argument("--max-distance-atr", type=float, default=0.20)
    parser.add_argument("--pre-fail-atr", type=float, default=0.20)
    parser.add_argument("--pre-horizon-minutes", type=int, default=60)
    parser.add_argument("--post-horizon-minutes", type=int, default=60)
    parser.add_argument("--continuation-atrs", nargs="+", type=float, default=[0.50, 1.00])
    parser.add_argument("--post-fail-atrs", nargs="+", type=float, default=[0.20, 0.30])
    parser.add_argument("--dedupe-one-per-bar-side-bucket", type=int, default=1)
    parser.add_argument("--min-state-samples", type=int, default=30)
    parser.add_argument("--top-state-limit", type=int, default=30)
    parser.add_argument("--summary-json", default="research/eth_original_t2_pretouch_continuation_summary.json")
    parser.add_argument("--markdown", default="research/20260508_eth_original_t2_pretouch_continuation.md")
    parser.add_argument("--candidate-csv", default="research/tmp_eth_original_t2_pretouch_continuation_candidates.csv")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    started = time.time()
    start = micro.base._as_utc(args.start)
    end = micro.base._as_utc(args.end)
    second_bars, build_stats = micro.load_or_build_second_bars(
        args.symbol,
        start,
        end,
        Path(args.archive_root),
        args.chunksize,
        Path(args.cache_root),
        not args.no_cache,
    )
    minute, signal = build_minute_context(second_bars, args.signal_timeframe)
    print(
        f"{args.symbol} second_rows={len(second_bars)} minute_rows={len(minute)} signal_rows={len(signal)}",
        flush=True,
    )
    candidates = collect_candidates(minute, signal, args)
    print(f"candidates={len(candidates)}", flush=True)
    candidates = label_candidates(candidates, second_bars, args)
    Path(args.candidate_csv).parent.mkdir(parents=True, exist_ok=True)
    candidates.to_csv(args.candidate_csv, index=False)
    table_summary = summarize(candidates, args)
    summary = {
        "symbol": args.symbol,
        "start": start.isoformat(),
        "end": end.isoformat(),
        "signal_timeframe": args.signal_timeframe,
        "build_stats": build_stats,
        "mode": {
            "breakout_shape": "original_t2",
            "candidate": "1m close before intrabar touch of prev_high_2/prev_low_2",
            "label": "continuous 1s high/low continuation before fail",
        },
        "min_distance_atr": float(args.min_distance_atr),
        "max_distance_atr": float(args.max_distance_atr),
        "pre_fail_atr": float(args.pre_fail_atr),
        "pre_horizon_minutes": int(args.pre_horizon_minutes),
        "post_horizon_minutes": int(args.post_horizon_minutes),
        "continuation_atrs": [float(v) for v in args.continuation_atrs],
        "post_fail_atrs": [float(v) for v in args.post_fail_atrs],
        "dedupe_one_per_bar_side_bucket": int(args.dedupe_one_per_bar_side_bucket),
        "candidate_count": int(len(candidates)),
        "summary": table_summary,
        "summary_path": args.summary_json,
        "markdown_path": args.markdown,
        "candidate_csv": args.candidate_csv,
        "elapsed_seconds": round(time.time() - started, 2),
    }
    Path(args.summary_json).write_text(json.dumps(summary, indent=2, ensure_ascii=False), encoding="utf-8")
    write_markdown(summary, Path(args.markdown))
    print(
        json.dumps(
            {"summary_path": args.summary_json, "markdown_path": args.markdown, "candidate_csv": args.candidate_csv, "elapsed_seconds": summary["elapsed_seconds"]},
            indent=2,
            ensure_ascii=False,
        ),
        flush=True,
    )


if __name__ == "__main__":
    main()
