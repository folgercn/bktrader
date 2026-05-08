#!/usr/bin/env python3
"""ETH pre-breakout hitting-probability table.

Research-only. This script estimates an empirical Markov-style hitting
probability before the 1h breakout level is reached. States are discretized by
distance-to-breakout and recent 1m speed/efficiency. It does not place trades;
it answers whether entering 0.x ATR before breakout has a measurable edge.
"""

from __future__ import annotations

import argparse
import json
import time
from pathlib import Path

import numpy as np
import pandas as pd

import btc_eth_micro_breakout_structure as micro


def _true_range(frame: pd.DataFrame) -> pd.Series:
    return pd.concat(
        [
            frame["high"] - frame["low"],
            (frame["high"] - frame["close"].shift()).abs(),
            (frame["low"] - frame["close"].shift()).abs(),
        ],
        axis=1,
    ).max(axis=1)


def _bucket(value: float, edges: list[float], labels: list[str]) -> str:
    if not np.isfinite(value):
        return "nan"
    for edge, label in zip(edges, labels):
        if value < edge:
            return label
    return labels[-1]


def build_minute_context(second_bars: pd.DataFrame, signal_timeframe: str) -> pd.DataFrame:
    minute = second_bars.resample("1min").agg(
        {"open": "first", "high": "max", "low": "min", "close": "last", "volume": "sum"}
    ).dropna()
    hourly = minute.resample(signal_timeframe).agg(
        {"open": "first", "high": "max", "low": "min", "close": "last", "volume": "sum"}
    ).dropna()
    hourly["atr"] = _true_range(hourly).rolling(14).mean().shift(1)
    hourly["ema20"] = hourly["close"].ewm(span=20, adjust=False, min_periods=20).mean().shift(1)
    hourly["ema20_slope"] = (hourly["ema20"] - hourly["ema20"].shift(1)).shift(1)
    hourly["prev_high_8"] = hourly["high"].shift(1).rolling(8).max()
    hourly["prev_low_8"] = hourly["low"].shift(1).rolling(8).min()

    context = hourly[["atr", "ema20", "ema20_slope", "prev_high_8", "prev_low_8"]].copy()
    prepared = minute.reset_index(names="minute_start")
    prepared["hour_start"] = prepared["minute_start"].dt.floor(signal_timeframe)
    prepared = prepared.merge(context.reset_index(names="hour_start"), on="hour_start", how="left")

    close = prepared["close"]
    prepared["ret_5m"] = close - close.shift(5)
    prepared["ret_15m"] = close - close.shift(15)
    prepared["range_high_15m"] = prepared["high"].rolling(15, min_periods=5).max()
    prepared["range_low_15m"] = prepared["low"].rolling(15, min_periods=5).min()
    prepared["range_15m"] = (prepared["range_high_15m"] - prepared["range_low_15m"]).replace(0, np.nan)
    prepared["eff_15m"] = prepared["ret_15m"] / prepared["range_15m"]
    prepared["close_pos_15m"] = (prepared["close"] - prepared["range_low_15m"]) / prepared["range_15m"]
    return prepared


def first_outcome(
    highs: np.ndarray,
    lows: np.ndarray,
    closes: np.ndarray,
    pos: int,
    side: str,
    level: float,
    fail_level: float,
    horizon: int,
) -> tuple[str, int]:
    end = min(pos + horizon, len(closes) - 1)
    for future_pos in range(pos + 1, end + 1):
        if side == "long":
            if highs[future_pos] >= level:
                return "hit", future_pos - pos
            if lows[future_pos] <= fail_level:
                return "fail", future_pos - pos
        else:
            if lows[future_pos] <= level:
                return "hit", future_pos - pos
            if highs[future_pos] >= fail_level:
                return "fail", future_pos - pos
    return "timeout", horizon


def collect_candidates(minute: pd.DataFrame, args: argparse.Namespace) -> pd.DataFrame:
    highs = minute["high"].to_numpy(dtype="float64", copy=False)
    lows = minute["low"].to_numpy(dtype="float64", copy=False)
    closes = minute["close"].to_numpy(dtype="float64", copy=False)
    rows: list[dict] = []
    max_distance_atr = float(args.max_distance_atr)
    min_distance_atr = float(args.min_distance_atr)
    fail_atr = float(args.fail_atr)
    horizon = int(args.horizon_minutes)
    for pos, row in minute.iterrows():
        atr = float(row.get("atr", np.nan))
        ema20 = float(row.get("ema20", np.nan))
        slope = float(row.get("ema20_slope", np.nan))
        if not np.isfinite(atr) or atr <= 0 or not np.isfinite(ema20) or not np.isfinite(slope):
            continue
        close = float(row["close"])
        state_rows = []
        if close > ema20 and slope > 0:
            level = float(row.get("prev_high_8", np.nan))
            if np.isfinite(level) and close < level:
                if highs[int(pos)] >= level:
                    continue
                distance = (level - close) / atr
                if min_distance_atr <= distance <= max_distance_atr:
                    fail_level = close - fail_atr * atr
                    state_rows.append(("long", level, distance, fail_level))
        if close < ema20 and slope < 0:
            level = float(row.get("prev_low_8", np.nan))
            if np.isfinite(level) and close > level:
                if lows[int(pos)] <= level:
                    continue
                distance = (close - level) / atr
                if min_distance_atr <= distance <= max_distance_atr:
                    fail_level = close + fail_atr * atr
                    state_rows.append(("short", level, distance, fail_level))

        for side, level, distance, fail_level in state_rows:
            mult = 1.0 if side == "long" else -1.0
            speed5 = mult * float(row.get("ret_5m", np.nan)) / atr
            speed15 = mult * float(row.get("ret_15m", np.nan)) / atr
            eff15 = mult * float(row.get("eff_15m", np.nan))
            close_pos = float(row.get("close_pos_15m", np.nan))
            if side == "short" and np.isfinite(close_pos):
                close_pos = 1.0 - close_pos
            outcome, minutes_to_event = first_outcome(highs, lows, closes, int(pos), side, level, fail_level, horizon)
            rows.append(
                {
                    "time": row["minute_start"],
                    "side": side,
                    "distance_atr": distance,
                    "speed5_atr": speed5,
                    "speed15_atr": speed15,
                    "eff15": eff15,
                    "close_pos15": close_pos,
                    "outcome": outcome,
                    "minutes_to_event": minutes_to_event,
                    "level": level,
                    "close": close,
                }
            )
    return pd.DataFrame(rows)


def summarize(candidates: pd.DataFrame, min_count: int) -> tuple[pd.DataFrame, pd.DataFrame]:
    if candidates.empty:
        return pd.DataFrame(), pd.DataFrame()
    df = candidates.copy()
    df["distance_bucket"] = df["distance_atr"].map(
        lambda v: _bucket(v, [0.10, 0.20, 0.30, 0.40], ["0-0.10", "0.10-0.20", "0.20-0.30", "0.30-0.40", "0.40+"])
    )
    df["speed5_bucket"] = df["speed5_atr"].map(
        lambda v: _bucket(v, [-0.03, 0.00, 0.03, 0.08], ["<-0.03", "-0.03-0", "0-0.03", "0.03-0.08", ">=0.08"])
    )
    df["eff_bucket"] = df["eff15"].map(
        lambda v: _bucket(v, [0.0, 0.2, 0.4, 0.6], ["<0", "0-0.2", "0.2-0.4", "0.4-0.6", ">=0.6"])
    )
    group_cols = ["distance_bucket", "speed5_bucket", "eff_bucket"]
    grouped = (
        df.groupby(group_cols)
        .agg(
            samples=("outcome", "size"),
            hit_rate=("outcome", lambda s: float((s == "hit").mean()) * 100.0),
            fail_rate=("outcome", lambda s: float((s == "fail").mean()) * 100.0),
            timeout_rate=("outcome", lambda s: float((s == "timeout").mean()) * 100.0),
            median_minutes=("minutes_to_event", "median"),
            avg_distance=("distance_atr", "mean"),
            avg_speed5=("speed5_atr", "mean"),
            avg_eff=("eff15", "mean"),
        )
        .reset_index()
    )
    grouped = grouped[grouped["samples"] >= int(min_count)].sort_values(["hit_rate", "samples"], ascending=[False, False])
    side_summary = (
        df.groupby(["side", "distance_bucket"])
        .agg(
            samples=("outcome", "size"),
            hit_rate=("outcome", lambda s: float((s == "hit").mean()) * 100.0),
            fail_rate=("outcome", lambda s: float((s == "fail").mean()) * 100.0),
            median_minutes=("minutes_to_event", "median"),
        )
        .reset_index()
        .sort_values(["side", "distance_bucket"])
    )
    for col in ("hit_rate", "fail_rate", "timeout_rate", "avg_distance", "avg_speed5", "avg_eff", "median_minutes"):
        if col in grouped:
            grouped[col] = grouped[col].round(4)
        if col in side_summary:
            side_summary[col] = side_summary[col].round(4)
    return grouped, side_summary


def write_markdown(summary: dict, path: Path) -> None:
    lines = [
        f"# ETH Pre-Breakout Hitting Probability ({summary['start']} to {summary['end']})",
        "",
        "Scope: research-only. Empirical Markov-style state table using 1m bars before the 1h breakout level is touched. Outcome is whether price hits the breakout level before an adverse fail move within the configured horizon.",
        "",
        f"- Horizon: {summary['horizon_minutes']}m",
        f"- Max distance: {summary['max_distance_atr']} ATR",
        f"- Fail distance: {summary['fail_atr']} ATR",
        f"- Candidates: {summary['candidate_count']}",
        "",
        "## Top States",
        "",
        "| Distance | Speed5 | Eff15 | Samples | Hit | Fail | Timeout | Median Min | Avg Dist | Avg Speed5 | Avg Eff |",
        "|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|",
    ]
    for row in summary["top_states"][:30]:
        lines.append(
            f"| `{row['distance_bucket']}` | `{row['speed5_bucket']}` | `{row['eff_bucket']}` | "
            f"{row['samples']} | {row['hit_rate']:.2f}% | {row['fail_rate']:.2f}% | "
            f"{row['timeout_rate']:.2f}% | {row['median_minutes']:.2f} | {row['avg_distance']:.4f} | "
            f"{row['avg_speed5']:.4f} | {row['avg_eff']:.4f} |"
        )
    lines.extend(
        [
            "",
            "## Side Distance Summary",
            "",
            "| Side | Distance | Samples | Hit | Fail | Median Min |",
            "|---|---|---:|---:|---:|---:|",
        ]
    )
    for row in summary["side_summary"]:
        lines.append(
            f"| `{row['side']}` | `{row['distance_bucket']}` | {row['samples']} | "
            f"{row['hit_rate']:.2f}% | {row['fail_rate']:.2f}% | {row['median_minutes']:.2f} |"
        )
    lines.extend(["", "## Files", ""])
    lines.append(f"- Summary JSON: `{summary['summary_path']}`")
    lines.append(f"- Candidates CSV: `{summary['candidates_path']}`")
    lines.append("")
    path.write_text("\n".join(lines), encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="ETH pre-breakout hitting-probability table")
    parser.add_argument("--symbol", default="ETHUSDT")
    parser.add_argument("--start", default="2025-01-01T00:00:00Z")
    parser.add_argument("--end", default="2025-12-31T23:59:59Z")
    parser.add_argument("--archive-root", default=str(micro.DEFAULT_ARCHIVE_ROOT))
    parser.add_argument("--cache-root", default=str(micro.DEFAULT_CACHE_ROOT))
    parser.add_argument("--no-cache", action="store_true")
    parser.add_argument("--chunksize", type=int, default=2_000_000)
    parser.add_argument("--signal-timeframe", default="1h")
    parser.add_argument("--min-distance-atr", type=float, default=0.02)
    parser.add_argument("--max-distance-atr", type=float, default=0.40)
    parser.add_argument("--fail-atr", type=float, default=0.20)
    parser.add_argument("--horizon-minutes", type=int, default=60)
    parser.add_argument("--min-count", type=int, default=50)
    parser.add_argument("--summary-json", default="research/eth_prebreakout_markov_probability_summary.json")
    parser.add_argument("--markdown", default="research/20260507_eth_prebreakout_markov_probability.md")
    parser.add_argument("--candidates-csv", default="research/tmp_eth_prebreakout_markov_probability_candidates.csv")
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
    minute = build_minute_context(second_bars, args.signal_timeframe)
    candidates = collect_candidates(minute, args)
    top_states, side_summary = summarize(candidates, args.min_count)
    candidates.to_csv(args.candidates_csv, index=False)

    summary = {
        "start": start.isoformat(),
        "end": end.isoformat(),
        "symbol": args.symbol,
        "horizon_minutes": args.horizon_minutes,
        "min_distance_atr": args.min_distance_atr,
        "max_distance_atr": args.max_distance_atr,
        "fail_atr": args.fail_atr,
        "candidate_count": int(len(candidates)),
        "top_states": top_states.to_dict(orient="records"),
        "side_summary": side_summary.to_dict(orient="records"),
        "build_stats": build_stats,
        "summary_path": args.summary_json,
        "markdown_path": args.markdown,
        "candidates_path": args.candidates_csv,
        "elapsed_seconds": round(time.time() - started, 2),
    }
    Path(args.summary_json).write_text(json.dumps(summary, indent=2, ensure_ascii=False), encoding="utf-8")
    write_markdown(summary, Path(args.markdown))
    print(json.dumps({"summary_path": args.summary_json, "markdown_path": args.markdown, "candidate_count": len(candidates), "elapsed_seconds": summary["elapsed_seconds"]}, indent=2), flush=True)


if __name__ == "__main__":
    main()
