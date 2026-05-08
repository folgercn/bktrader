#!/usr/bin/env python3
"""ETH multi-feature pre-breakout and post-touch continuation table.

Research-only. This script fixes the main weakness of a distance-only
pre-breakout table: the same distance to the breakout level can mean an active
push, a flat drift, or a pullback after the level was already touched. States
are therefore bucketed by phase and recent speed/efficiency, while outcomes
are measured on continuous 1s bars.
"""

from __future__ import annotations

import argparse
import json
import time
from pathlib import Path

import numpy as np
import pandas as pd

import btc_eth_micro_breakout_structure as micro
import eth_prebreakout_markov_probability as old_prob


def _bucket(value: float, edges: list[float], labels: list[str]) -> str:
    if not np.isfinite(value):
        return "nan"
    for edge, label in zip(edges, labels):
        if value < edge:
            return label
    return labels[-1]


def build_context(second_bars: pd.DataFrame, signal_timeframe: str) -> pd.DataFrame:
    minute = old_prob.build_minute_context(second_bars, signal_timeframe)
    minute["ret_1m"] = minute["close"] - minute["close"].shift(1)
    minute["prior_high_in_hour"] = minute.groupby("hour_start")["high"].transform(lambda s: s.cummax().shift(1))
    minute["prior_low_in_hour"] = minute.groupby("hour_start")["low"].transform(lambda s: s.cummin().shift(1))
    minute["prior_high_15m"] = minute["high"].shift(1).rolling(15, min_periods=3).max()
    minute["prior_low_15m"] = minute["low"].shift(1).rolling(15, min_periods=3).min()
    vol_mean = minute["volume"].shift(1).rolling(60, min_periods=20).mean()
    vol_std = minute["volume"].shift(1).rolling(60, min_periods=20).std()
    minute["volume_z60"] = (minute["volume"] - vol_mean) / vol_std.replace(0, np.nan)
    return minute


def _state_buckets(row: dict) -> dict:
    speed5 = float(row["speed5_atr"])
    if speed5 >= 0.05:
        motion = "push"
    elif speed5 >= -0.01:
        motion = "flat"
    else:
        motion = "pullback"
    phase_motion = f"{row['touch_phase']}_{motion}"
    return {
        "distance_bucket": _bucket(
            float(row["distance_atr"]),
            [0.05, 0.10, 0.20, 0.30, 0.40],
            ["0-0.05", "0.05-0.10", "0.10-0.20", "0.20-0.30", "0.30-0.40", "0.40+"],
        ),
        "speed1_bucket": _bucket(
            float(row["speed1_atr"]),
            [-0.03, 0.0, 0.03, 0.08],
            ["<-0.03", "-0.03-0", "0-0.03", "0.03-0.08", ">=0.08"],
        ),
        "speed5_bucket": _bucket(
            speed5,
            [-0.05, -0.01, 0.03, 0.08],
            ["<-0.05", "-0.05--0.01", "-0.01-0.03", "0.03-0.08", ">=0.08"],
        ),
        "speed15_bucket": _bucket(
            float(row["speed15_atr"]),
            [-0.10, -0.03, 0.05, 0.15],
            ["<-0.10", "-0.10--0.03", "-0.03-0.05", "0.05-0.15", ">=0.15"],
        ),
        "eff_bucket": _bucket(
            float(row["eff15"]),
            [0.0, 0.2, 0.4, 0.6],
            ["<0", "0-0.2", "0.2-0.4", "0.4-0.6", ">=0.6"],
        ),
        "close_pos_bucket": _bucket(
            float(row["close_pos15"]),
            [0.2, 0.4, 0.6, 0.8],
            ["0-0.2", "0.2-0.4", "0.4-0.6", "0.6-0.8", "0.8-1.0"],
        ),
        "volume_bucket": _bucket(
            float(row["volume_z60"]),
            [-1.0, 0.0, 1.0, 2.0],
            ["<-1z", "-1-0z", "0-1z", "1-2z", ">=2z"],
        ),
        "phase_motion": phase_motion,
    }


def _candidate_rows(minute: pd.DataFrame, pos: int, args: argparse.Namespace) -> list[dict]:
    row = minute.iloc[pos]
    atr = float(row.get("atr", np.nan))
    ema20 = float(row.get("ema20", np.nan))
    slope = float(row.get("ema20_slope", np.nan))
    close = float(row["close"])
    if not np.isfinite(atr) or atr <= 0 or not np.isfinite(ema20) or not np.isfinite(slope):
        return []

    rows: list[dict] = []
    if close > ema20 and slope > 0:
        level = float(row.get("prev_high_8", np.nan))
        if np.isfinite(level) and close < level and float(row["high"]) < level:
            distance = (level - close) / atr
            if float(args.min_distance_atr) <= distance <= float(args.max_distance_atr):
                prior_touched = bool(pd.notna(row.get("prior_high_in_hour", np.nan)) and float(row["prior_high_in_hour"]) >= level)
                rows.append(_build_candidate(row, "long", level, distance, prior_touched, args))
    if close < ema20 and slope < 0:
        level = float(row.get("prev_low_8", np.nan))
        if np.isfinite(level) and close > level and float(row["low"]) > level:
            distance = (close - level) / atr
            if float(args.min_distance_atr) <= distance <= float(args.max_distance_atr):
                prior_touched = bool(pd.notna(row.get("prior_low_in_hour", np.nan)) and float(row["prior_low_in_hour"]) <= level)
                rows.append(_build_candidate(row, "short", level, distance, prior_touched, args))
    return rows


def _build_candidate(row: pd.Series, side: str, level: float, distance: float, prior_touched: bool, args: argparse.Namespace) -> dict:
    atr = float(row["atr"])
    side_mult = 1.0 if side == "long" else -1.0
    close_pos = float(row.get("close_pos_15m", np.nan))
    if side == "short" and np.isfinite(close_pos):
        close_pos = 1.0 - close_pos
    out = {
        "time": row["minute_start"],
        "hour_start": row["hour_start"],
        "side": side,
        "touch_phase": "post_touch_pullback" if prior_touched else "fresh",
        "level": level,
        "close": float(row["close"]),
        "atr": atr,
        "distance_atr": float(distance),
        "speed1_atr": side_mult * float(row.get("ret_1m", np.nan)) / atr,
        "speed5_atr": side_mult * float(row.get("ret_5m", np.nan)) / atr,
        "speed15_atr": side_mult * float(row.get("ret_15m", np.nan)) / atr,
        "eff15": side_mult * float(row.get("eff_15m", np.nan)),
        "close_pos15": close_pos,
        "volume_z60": float(row.get("volume_z60", np.nan)),
        "pre_fail_atr": float(args.pre_fail_atr),
        "continuation_atr": float(args.continuation_atr),
        "post_fail_atr": float(args.post_fail_atr),
    }
    out.update(_state_buckets(out))
    return out


def _first_true(mask: np.ndarray) -> int | None:
    hits = np.flatnonzero(mask)
    if len(hits) == 0:
        return None
    return int(hits[0])


def continuation_outcome(
    idx: pd.DatetimeIndex,
    highs: np.ndarray,
    lows: np.ndarray,
    candidate: dict,
    args: argparse.Namespace,
) -> dict:
    start_time = pd.Timestamp(candidate["time"]) + pd.Timedelta(minutes=1)
    start_pos = int(idx.searchsorted(start_time, side="left"))
    if start_pos >= len(idx):
        return {"outcome": "no_second", "touch_seconds": np.nan, "continuation_seconds": np.nan}

    level = float(candidate["level"])
    close = float(candidate["close"])
    atr = float(candidate["atr"])
    side = str(candidate["side"])
    pre_end_time = start_time + pd.Timedelta(minutes=int(args.pre_horizon_minutes))
    pre_end_pos = min(int(idx.searchsorted(pre_end_time, side="right")), len(idx))
    if side == "long":
        pre_fail = close - float(args.pre_fail_atr) * atr
        cont_level = level + float(args.continuation_atr) * atr
        post_fail = level - float(args.post_fail_atr) * atr
    else:
        pre_fail = close + float(args.pre_fail_atr) * atr
        cont_level = level - float(args.continuation_atr) * atr
        post_fail = level + float(args.post_fail_atr) * atr

    if pre_end_pos <= start_pos:
        return {"outcome": "pre_timeout", "touch_seconds": np.nan, "continuation_seconds": np.nan}

    pre_highs = highs[start_pos:pre_end_pos]
    pre_lows = lows[start_pos:pre_end_pos]
    if side == "long":
        pre_fail_offset = _first_true(pre_lows <= pre_fail)
        touch_offset = _first_true(pre_highs >= level)
    else:
        pre_fail_offset = _first_true(pre_highs >= pre_fail)
        touch_offset = _first_true(pre_lows <= level)
    if touch_offset is None:
        return {"outcome": "pre_timeout", "touch_seconds": np.nan, "continuation_seconds": np.nan}
    if pre_fail_offset is not None and pre_fail_offset <= touch_offset:
        return {"outcome": "pre_fail", "touch_seconds": np.nan, "continuation_seconds": np.nan}

    touch_pos = start_pos + touch_offset

    touch_time = idx[touch_pos]
    post_end_time = touch_time + pd.Timedelta(minutes=int(args.post_horizon_minutes))
    post_end_pos = min(int(idx.searchsorted(post_end_time, side="right")), len(idx))
    post_highs = highs[touch_pos:post_end_pos]
    post_lows = lows[touch_pos:post_end_pos]
    if side == "long":
        post_fail_offset = _first_true(post_lows <= post_fail)
        continuation_offset = _first_true(post_highs >= cont_level)
    else:
        post_fail_offset = _first_true(post_highs >= post_fail)
        continuation_offset = _first_true(post_lows <= cont_level)
    if continuation_offset is not None and (post_fail_offset is None or continuation_offset < post_fail_offset):
        return {
            "outcome": "continuation",
            "touch_seconds": (touch_time - start_time).total_seconds(),
            "continuation_seconds": (idx[touch_pos + continuation_offset] - touch_time).total_seconds(),
        }
    if post_fail_offset is not None:
        return {
            "outcome": "post_fail",
            "touch_seconds": (touch_time - start_time).total_seconds(),
            "continuation_seconds": np.nan,
        }
    return {
        "outcome": "post_timeout",
        "touch_seconds": (touch_time - start_time).total_seconds(),
        "continuation_seconds": np.nan,
    }


def collect_candidates(second_bars: pd.DataFrame, minute: pd.DataFrame, args: argparse.Namespace) -> pd.DataFrame:
    rows: list[dict] = []
    seen: set[tuple] = set()
    idx = second_bars.index
    highs = second_bars["high"].to_numpy(dtype="float64", copy=False)
    lows = second_bars["low"].to_numpy(dtype="float64", copy=False)
    for pos in range(len(minute)):
        for candidate in _candidate_rows(minute, pos, args):
            if int(args.one_per_hour_level) > 0:
                key = (
                    pd.Timestamp(candidate["hour_start"]),
                    str(candidate["side"]),
                    round(float(candidate["level"]), 8),
                    str(candidate["touch_phase"]),
                )
                if key in seen:
                    continue
                seen.add(key)
            outcome = continuation_outcome(idx, highs, lows, candidate, args)
            candidate.update(outcome)
            rows.append(candidate)
    return pd.DataFrame(rows)


def _rate(series: pd.Series, value: str) -> float:
    if series.empty:
        return 0.0
    return float((series == value).mean()) * 100.0


def summarize(candidates: pd.DataFrame, min_count: int) -> dict:
    if candidates.empty:
        return {"top_states": [], "phase_distance": [], "speed_phase": []}

    df = candidates.copy()
    df["touched"] = df["outcome"].isin(["continuation", "post_fail", "post_timeout"])
    grouped = (
        df.groupby(["touch_phase", "distance_bucket", "speed5_bucket", "eff_bucket"])
        .agg(
            samples=("outcome", "size"),
            touch_rate=("touched", lambda s: float(s.mean()) * 100.0),
            continuation_rate=("outcome", lambda s: _rate(s, "continuation")),
            post_fail_rate=("outcome", lambda s: _rate(s, "post_fail")),
            pre_fail_rate=("outcome", lambda s: _rate(s, "pre_fail")),
            avg_speed1=("speed1_atr", "mean"),
            avg_speed5=("speed5_atr", "mean"),
            avg_speed15=("speed15_atr", "mean"),
            avg_eff=("eff15", "mean"),
            median_touch_seconds=("touch_seconds", "median"),
            median_continuation_seconds=("continuation_seconds", "median"),
        )
        .reset_index()
    )
    grouped = grouped[grouped["samples"] >= int(min_count)]
    grouped = grouped.sort_values(["continuation_rate", "touch_rate", "samples"], ascending=[False, False, False])

    phase_distance = (
        df.groupby(["side", "touch_phase", "distance_bucket"])
        .agg(
            samples=("outcome", "size"),
            touch_rate=("touched", lambda s: float(s.mean()) * 100.0),
            continuation_rate=("outcome", lambda s: _rate(s, "continuation")),
            post_fail_rate=("outcome", lambda s: _rate(s, "post_fail")),
            pre_fail_rate=("outcome", lambda s: _rate(s, "pre_fail")),
            avg_speed5=("speed5_atr", "mean"),
        )
        .reset_index()
        .sort_values(["side", "touch_phase", "distance_bucket"])
    )

    speed_phase = (
        df.groupby(["touch_phase", "phase_motion", "speed5_bucket"])
        .agg(
            samples=("outcome", "size"),
            touch_rate=("touched", lambda s: float(s.mean()) * 100.0),
            continuation_rate=("outcome", lambda s: _rate(s, "continuation")),
            post_fail_rate=("outcome", lambda s: _rate(s, "post_fail")),
            avg_distance=("distance_atr", "mean"),
        )
        .reset_index()
        .sort_values(["continuation_rate", "samples"], ascending=[False, False])
    )

    for frame in (grouped, phase_distance, speed_phase):
        for col in frame.columns:
            if col.endswith("_rate") or col.startswith("avg_") or col.startswith("median_"):
                frame[col] = frame[col].astype(float).round(4)

    return {
        "top_states": grouped.to_dict("records"),
        "phase_distance": phase_distance.to_dict("records"),
        "speed_phase": speed_phase.to_dict("records"),
    }


def write_markdown(summary: dict, path: Path) -> None:
    lines = [
        f"# ETH 突破前多变量延续性表（{summary['start']} 至 {summary['end']}）",
        "",
        "范围：仅限 `research`。状态在接近 `1h` breakout level 的 `1m` bar 上采样，但 outcome 使用连续 `1s` bar 计算。该表区分 `fresh` 突破前推进和 `post_touch_pullback` 触碰后回撤，并加入涨速与 efficiency 分桶，因为距离单变量会混合不同市场状态。",
        "",
        f"- 候选 setup 数：{summary['candidate_count']}",
        f"- 每个 hour/level/phase 只保留一次：{bool(summary['one_per_hour_level'])}",
        f"- Touch 前观察窗口：{summary['pre_horizon_minutes']}m",
        f"- Touch 后 continuation 目标：{summary['continuation_atr']} ATR",
        f"- Touch 后失败阈值：{summary['post_fail_atr']} ATR",
        "",
        "## 高信号多变量状态",
        "",
        "| Phase | Distance | Speed5 | Eff15 | 样本 | Touch | Continue | PostFail | PreFail | Avg S1 | Avg S5 | Avg S15 | Avg Eff | Med Touch s | Med Cont s |",
        "|---|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|",
    ]
    for row in summary["top_states"][:40]:
        lines.append(
            f"| `{row['touch_phase']}` | `{row['distance_bucket']}` | `{row['speed5_bucket']}` | `{row['eff_bucket']}` | "
            f"{row['samples']} | {row['touch_rate']:.2f}% | {row['continuation_rate']:.2f}% | "
            f"{row['post_fail_rate']:.2f}% | {row['pre_fail_rate']:.2f}% | {row['avg_speed1']:.4f} | "
            f"{row['avg_speed5']:.4f} | {row['avg_speed15']:.4f} | {row['avg_eff']:.4f} | "
            f"{row['median_touch_seconds']:.2f} | {row['median_continuation_seconds']:.2f} |"
        )
    lines.extend(
        [
            "",
            "## Phase 与距离汇总",
            "",
            "| Side | Phase | Distance | 样本 | Touch | Continue | PostFail | PreFail | Avg Speed5 |",
            "|---|---|---|---:|---:|---:|---:|---:|---:|",
        ]
    )
    for row in summary["phase_distance"]:
        lines.append(
            f"| `{row['side']}` | `{row['touch_phase']}` | `{row['distance_bucket']}` | {row['samples']} | "
            f"{row['touch_rate']:.2f}% | {row['continuation_rate']:.2f}% | {row['post_fail_rate']:.2f}% | "
            f"{row['pre_fail_rate']:.2f}% | {row['avg_speed5']:.4f} |"
        )
    lines.extend(
        [
            "",
            "## 涨速与 Phase 汇总",
            "",
            "| Phase | Motion | Speed5 | 样本 | Touch | Continue | PostFail | Avg Dist |",
            "|---|---|---|---:|---:|---:|---:|---:|",
        ]
    )
    for row in summary["speed_phase"]:
        lines.append(
            f"| `{row['touch_phase']}` | `{row['phase_motion']}` | `{row['speed5_bucket']}` | {row['samples']} | "
            f"{row['touch_rate']:.2f}% | {row['continuation_rate']:.2f}% | {row['post_fail_rate']:.2f}% | "
            f"{row['avg_distance']:.4f} |"
        )
    lines.extend(["", "## 文件", ""])
    lines.append(f"- Summary JSON：`{summary['summary_path']}`")
    lines.append(f"- 候选 CSV：`{summary['candidates_path']}`")
    lines.append("")
    path.write_text("\n".join(lines), encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="ETH multi-feature pre-breakout continuation probability")
    parser.add_argument("--symbol", default="ETHUSDT")
    parser.add_argument("--start", default="2026-01-01T00:00:00Z")
    parser.add_argument("--end", default="2026-04-30T23:59:59Z")
    parser.add_argument("--archive-root", default=str(micro.DEFAULT_ARCHIVE_ROOT))
    parser.add_argument("--cache-root", default=str(micro.DEFAULT_CACHE_ROOT))
    parser.add_argument("--no-cache", action="store_true")
    parser.add_argument("--chunksize", type=int, default=2_000_000)
    parser.add_argument("--min-distance-atr", type=float, default=0.02)
    parser.add_argument("--max-distance-atr", type=float, default=0.40)
    parser.add_argument("--pre-fail-atr", type=float, default=0.20)
    parser.add_argument("--pre-horizon-minutes", type=int, default=60)
    parser.add_argument("--continuation-atr", type=float, default=0.50)
    parser.add_argument("--post-fail-atr", type=float, default=0.20)
    parser.add_argument("--post-horizon-minutes", type=int, default=120)
    parser.add_argument("--one-per-hour-level", type=int, default=1)
    parser.add_argument("--min-count", type=int, default=20)
    parser.add_argument("--summary-json", default="research/eth_prebreakout_multifeature_continuation_summary.json")
    parser.add_argument("--markdown", default="research/20260507_eth_prebreakout_multifeature_continuation.md")
    parser.add_argument("--candidates-csv", default="research/tmp_eth_prebreakout_multifeature_continuation_candidates.csv")
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
    minute = build_context(second_bars, "1h")
    print(f"{args.symbol}: second_rows={len(second_bars)} minute_rows={len(minute)}", flush=True)
    candidates = collect_candidates(second_bars, minute, args)
    candidates.to_csv(args.candidates_csv, index=False)
    summarized = summarize(candidates, args.min_count)
    summary = {
        "start": start.isoformat(),
        "end": end.isoformat(),
        "symbol": args.symbol,
        "execution": "continuous 1s OHLC bars from local trade ticks",
        "candidate_count": int(len(candidates)),
        "one_per_hour_level": int(args.one_per_hour_level),
        "min_distance_atr": args.min_distance_atr,
        "max_distance_atr": args.max_distance_atr,
        "pre_fail_atr": args.pre_fail_atr,
        "pre_horizon_minutes": args.pre_horizon_minutes,
        "continuation_atr": args.continuation_atr,
        "post_fail_atr": args.post_fail_atr,
        "post_horizon_minutes": args.post_horizon_minutes,
        "min_count": args.min_count,
        "build_stats": build_stats,
        "summary_path": args.summary_json,
        "markdown_path": args.markdown,
        "candidates_path": args.candidates_csv,
        "elapsed_seconds": round(time.time() - started, 2),
    }
    summary.update(summarized)
    Path(args.summary_json).write_text(json.dumps(summary, indent=2, ensure_ascii=False), encoding="utf-8")
    write_markdown(summary, Path(args.markdown))
    print(
        json.dumps(
            {
                "summary_path": args.summary_json,
                "markdown_path": args.markdown,
                "candidates": len(candidates),
                "elapsed_seconds": summary["elapsed_seconds"],
            },
            indent=2,
        ),
        flush=True,
    )


if __name__ == "__main__":
    main()
