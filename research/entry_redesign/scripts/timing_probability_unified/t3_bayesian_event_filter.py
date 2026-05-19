"""Walk-forward Bayesian quality scores for T3 event sources.

Research-only. The frozen pretouch RF score was built for touch timing and has
not been monotonic on strict T3 lifecycle trades. This runner learns a small
walk-forward bucket model from already-realized strict T3 lifecycle trade
labels, scores all generated T3 events with only prior-month labels, and writes
thresholded event CSVs that can be replayed by
``t3_filtered_external_event_lifecycle.py``.
"""

from __future__ import annotations

import argparse
import json
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import numpy as np
import pandas as pd

PROJECT_ROOT = Path(__file__).resolve().parents[4]
OUTPUT_DIR = (
    PROJECT_ROOT
    / "research"
    / "entry_redesign"
    / "scripts"
    / "output"
    / "timing_probability_unified"
)
DEFAULT_OVERLAY_DIR = OUTPUT_DIR / "t3_probability_overlay_extended"
DEFAULT_TRADES = DEFAULT_OVERLAY_DIR / "t3_probability_overlay_trades.csv"
DEFAULT_EVENTS = DEFAULT_OVERLAY_DIR / "t3_probability_overlay_scored_events.csv"
DEFAULT_OUTPUT = OUTPUT_DIR / "t3_bayesian_event_filter_extended"
DEFAULT_THRESHOLDS = [-0.01, 0.0, 0.005, 0.01, 0.02]
SPEED_THRESHOLD = 0.2281


@dataclass(frozen=True)
class GroupScore:
    """Posterior score for one event under one backoff group."""

    mean_net_pct: float
    train_trades: int
    observed_mean_pct: float
    group: str


GROUP_BACKOFFS: tuple[tuple[str, tuple[str, ...]], ...] = (
    ("symbol_side_speed_ext", ("symbol", "side", "speed_abs_bucket", "extension_abs_bucket")),
    ("side_speed_ext", ("side", "speed_abs_bucket", "extension_abs_bucket")),
    ("side_speed", ("side", "speed_abs_bucket")),
    ("side", ("side",)),
    ("all", ()),
)


def _month_from_timestamp(series: pd.Series) -> pd.Series:
    timestamps = pd.to_datetime(series, utc=True, errors="coerce")
    return timestamps.dt.strftime("%Y-%m")


def _bucket_numeric(series: pd.Series, bins: list[float], labels: list[str]) -> pd.Series:
    numeric = pd.to_numeric(series, errors="coerce")
    bucketed = pd.cut(numeric, bins=bins, labels=labels, include_lowest=True, right=False)
    return bucketed.astype(object).where(bucketed.notna(), "missing").astype(str)


def add_feature_buckets(frame: pd.DataFrame) -> pd.DataFrame:
    """Add stable explanatory buckets shared by events and trade labels."""

    out = frame.copy()
    out["symbol"] = out["symbol"].astype(str)
    out["side"] = out["side"].astype(str)
    out["speed_abs_bucket"] = _bucket_numeric(
        pd.to_numeric(out["speed_300s_atr"], errors="coerce").abs(),
        [-np.inf, 0.10, SPEED_THRESHOLD, 0.35, 0.50, np.inf],
        ["<0.10", f"0.10-{SPEED_THRESHOLD:.4f}", f"{SPEED_THRESHOLD:.4f}-0.35", "0.35-0.50", ">=0.50"],
    )
    out["extension_abs_bucket"] = _bucket_numeric(
        pd.to_numeric(out["touch_extension_atr"], errors="coerce").abs(),
        [-np.inf, 0.02, 0.05, 0.10, 0.20, np.inf],
        ["<0.02", "0.02-0.05", "0.05-0.10", "0.10-0.20", ">=0.20"],
    )
    out["rf_bucket"] = _bucket_numeric(
        out.get("rf_probability", pd.Series(np.nan, index=out.index)),
        [-np.inf, 0.35, 0.45, 0.50, 0.55, 0.60, 0.70, np.inf],
        ["<0.35", "0.35-0.45", "0.45-0.50", "0.50-0.55", "0.55-0.60", "0.60-0.70", ">=0.70"],
    )
    out["timing_bucket"] = out.get("timing_prediction", pd.Series("missing", index=out.index)).fillna(
        "missing"
    ).astype(str)
    return out


def load_inputs(trades_path: Path, events_path: Path) -> tuple[pd.DataFrame, pd.DataFrame]:
    trades = pd.read_csv(trades_path)
    events = pd.read_csv(events_path)
    trades["month"] = trades.get("month", _month_from_timestamp(trades["entry_time"])).astype(str)
    events["month"] = _month_from_timestamp(events["touch_time"]).astype(str)
    return add_feature_buckets(trades), add_feature_buckets(events)


def _group_key(row: pd.Series, columns: tuple[str, ...]) -> tuple[Any, ...]:
    if not columns:
        return ("__all__",)
    return tuple(row[column] for column in columns)


def _build_group_tables(train: pd.DataFrame) -> dict[str, dict[tuple[Any, ...], tuple[int, float]]]:
    pnl = pd.to_numeric(train["net_after_fee_pct"], errors="coerce").fillna(0.0)
    work = train.copy()
    work["_pnl"] = pnl
    tables: dict[str, dict[tuple[Any, ...], tuple[int, float]]] = {}
    for label, columns in GROUP_BACKOFFS:
        if not columns:
            tables[label] = {("__all__",): (int(len(work)), float(work["_pnl"].mean()) if len(work) else 0.0)}
            continue
        grouped = work.groupby(list(columns), dropna=False)["_pnl"].agg(["count", "mean"])
        tables[label] = {
            key if isinstance(key, tuple) else (key,): (int(row["count"]), float(row["mean"]))
            for key, row in grouped.iterrows()
        }
    return tables


def score_one_event(
    row: pd.Series,
    tables: dict[str, dict[tuple[Any, ...], tuple[int, float]]],
    *,
    global_mean: float,
    min_group_trades: int,
    prior_strength: float,
) -> GroupScore:
    """Score one event using the most specific sufficiently-populated group."""

    fallback = tables.get("all", {}).get(("__all__",), (0, global_mean))
    for label, columns in GROUP_BACKOFFS:
        key = _group_key(row, columns)
        count, observed_mean = tables.get(label, {}).get(key, (0, np.nan))
        if label == "all" or count >= min_group_trades:
            posterior = (float(prior_strength) * global_mean + float(count) * float(observed_mean)) / (
                float(prior_strength) + float(count)
            )
            return GroupScore(
                mean_net_pct=float(posterior),
                train_trades=int(count),
                observed_mean_pct=float(observed_mean),
                group=f"{label}:{'|'.join(str(part) for part in key)}",
            )
    count, observed_mean = fallback
    posterior = (float(prior_strength) * global_mean + float(count) * float(observed_mean)) / (
        float(prior_strength) + float(count)
    )
    return GroupScore(
        mean_net_pct=float(posterior),
        train_trades=int(count),
        observed_mean_pct=float(observed_mean),
        group="all:__all__",
    )


def walk_forward_scores(
    trades: pd.DataFrame,
    events: pd.DataFrame,
    *,
    min_train_months: int,
    min_group_trades: int,
    prior_strength: float,
) -> pd.DataFrame:
    """Assign scores to events using only labels from months before each event."""

    if events.empty:
        return events.copy()
    scored_parts = []
    months = sorted(month for month in events["month"].dropna().unique() if str(month) != "NaT")
    trade_months = sorted(month for month in trades["month"].dropna().unique() if str(month) != "NaT")
    for month in months:
        view = events[events["month"] == month].copy()
        prior_months = [item for item in trade_months if item < month]
        train = trades[trades["month"].isin(prior_months)].copy()
        ready = len(set(prior_months)) >= int(min_train_months) and not train.empty
        if not ready:
            view["bayes_score_ready"] = False
            view["bayes_mean_net_pct"] = np.nan
            view["bayes_train_trades"] = 0
            view["bayes_observed_mean_pct"] = np.nan
            view["bayes_group"] = "not_ready"
            scored_parts.append(view)
            continue
        global_mean = float(pd.to_numeric(train["net_after_fee_pct"], errors="coerce").fillna(0.0).mean())
        tables = _build_group_tables(train)
        scores = [
            score_one_event(
                row,
                tables,
                global_mean=global_mean,
                min_group_trades=int(min_group_trades),
                prior_strength=float(prior_strength),
            )
            for _, row in view.iterrows()
        ]
        view["bayes_score_ready"] = True
        view["bayes_mean_net_pct"] = [score.mean_net_pct for score in scores]
        view["bayes_train_trades"] = [score.train_trades for score in scores]
        view["bayes_observed_mean_pct"] = [score.observed_mean_pct for score in scores]
        view["bayes_group"] = [score.group for score in scores]
        scored_parts.append(view)
    return pd.concat(scored_parts, ignore_index=True)


def _fixed_stats(frame: pd.DataFrame, months: list[str], symbols: list[str]) -> dict[str, Any]:
    if frame.empty:
        by_key = pd.Series(dtype=float)
    else:
        work = frame.copy()
        work["_key"] = work["month"].astype(str) + "|" + work["symbol"].astype(str)
        by_key = work.groupby("_key")["net_after_fee_pct"].sum()
    values = []
    by_month: dict[str, float] = {}
    by_symbol = {symbol: 0.0 for symbol in symbols}
    for month in months:
        month_total = 0.0
        for symbol in symbols:
            value = float(by_key.get(f"{month}|{symbol}", 0.0))
            values.append(value)
            month_total += value
            by_symbol[symbol] += value
        by_month[month] = round(month_total, 6)
    return {
        "label_net_after_fee_pct": round(float(sum(values)), 6),
        "label_worst_symbol_month_pct": round(float(min(values, default=0.0)), 6),
        "label_negative_symbol_months": int(sum(1 for value in values if value < 0.0)),
        "label_by_month": by_month,
        "label_by_symbol": {symbol: round(value, 6) for symbol, value in by_symbol.items()},
    }


def _threshold_label(threshold: float) -> str:
    prefix = "m" if threshold < 0 else ""
    value = abs(float(threshold))
    return f"bayes_ge_{prefix}{value:.3f}".replace(".", "p")


def _ready_mask(series: pd.Series) -> pd.Series:
    return series.astype("boolean").fillna(False).astype(bool)


def summarize_thresholds(
    scored_events: pd.DataFrame,
    trades: pd.DataFrame,
    *,
    thresholds: list[float],
    months: list[str],
    symbols: list[str],
) -> pd.DataFrame:
    if scored_events.empty:
        return pd.DataFrame()
    score_cols = [
        "event_id",
        "bayes_score_ready",
        "bayes_mean_net_pct",
        "bayes_train_trades",
        "bayes_group",
    ]
    scored_lookup = scored_events[score_cols].drop_duplicates("event_id")
    labeled = trades.merge(scored_lookup, on="event_id", how="left", validate="m:1")
    pnl = pd.to_numeric(labeled["net_after_fee_pct"], errors="coerce").fillna(0.0)
    labeled = labeled.assign(_pnl=pnl)

    rows = []
    for threshold in thresholds:
        label = _threshold_label(threshold)
        event_mask = _ready_mask(scored_events["bayes_score_ready"]) & (
            pd.to_numeric(scored_events["bayes_mean_net_pct"], errors="coerce") >= float(threshold)
        )
        trade_mask = _ready_mask(labeled["bayes_score_ready"]) & (
            pd.to_numeric(labeled["bayes_mean_net_pct"], errors="coerce") >= float(threshold)
        )
        selected_trades = labeled[trade_mask].copy()
        selected_pnl = selected_trades["_pnl"]
        rows.append(
            {
                "threshold_label": label,
                "score_threshold_pct": float(threshold),
                "selected_events": int(event_mask.sum()),
                "selected_labeled_trades": int(len(selected_trades)),
                "avg_selected_score_pct": round(
                    float(pd.to_numeric(scored_events.loc[event_mask, "bayes_mean_net_pct"], errors="coerce").mean()),
                    6,
                )
                if event_mask.any()
                else 0.0,
                "avg_labeled_trade_net_pct": round(float(selected_pnl.mean()), 6) if len(selected_pnl) else 0.0,
                "labeled_win_rate_pct": round(float((selected_pnl > 0.0).mean()) * 100.0, 2)
                if len(selected_pnl)
                else 0.0,
                **_fixed_stats(selected_trades, months, symbols),
            }
        )
    return pd.DataFrame(rows).sort_values(
        ["label_net_after_fee_pct", "selected_labeled_trades"],
        ascending=[False, False],
    )


def write_outputs(
    *,
    output_dir: Path,
    scored_events: pd.DataFrame,
    threshold_summary: pd.DataFrame,
    thresholds: list[float],
    trades_path: Path,
    events_path: Path,
    min_train_months: int,
    min_group_trades: int,
    prior_strength: float,
) -> None:
    output_dir.mkdir(parents=True, exist_ok=True)
    scored_events.to_csv(output_dir / "t3_bayesian_event_scores.csv", index=False)
    threshold_summary.to_csv(output_dir / "t3_bayesian_threshold_summary.csv", index=False)

    selected_files = {}
    for threshold in thresholds:
        label = _threshold_label(threshold)
        selected = scored_events[
            _ready_mask(scored_events["bayes_score_ready"])
            & (pd.to_numeric(scored_events["bayes_mean_net_pct"], errors="coerce") >= float(threshold))
        ].copy()
        path = output_dir / f"selected_events_{label}.csv"
        selected.to_csv(path, index=False)
        selected_files[label] = str(path)

    payload = {
        "note": (
            "Research-only walk-forward Bayesian T3 event quality scores. Scores use only prior-month "
            "strict lifecycle trade labels; selected event CSVs are intended for external-event lifecycle replay."
        ),
        "trades_path": str(trades_path),
        "events_path": str(events_path),
        "min_train_months": int(min_train_months),
        "min_group_trades": int(min_group_trades),
        "prior_strength": float(prior_strength),
        "events_scored": int(len(scored_events)),
        "events_score_ready": int(_ready_mask(scored_events["bayes_score_ready"]).sum()),
        "selected_files": selected_files,
        "thresholds": threshold_summary.to_dict(orient="records"),
    }
    (output_dir / "t3_bayesian_event_filter_summary.json").write_text(
        json.dumps(payload, indent=2, ensure_ascii=False, default=str) + "\n",
        encoding="utf-8",
    )

    lines = [
        "# T3 Bayesian Event Filter",
        "",
        "Walk-forward Bayesian bucket scores for T3 event quality.",
        "",
        f"- Trades: `{trades_path}`",
        f"- Events: `{events_path}`",
        f"- Min train months: `{min_train_months}`",
        f"- Min group trades: `{min_group_trades}`",
        f"- Prior strength: `{prior_strength}`",
        "",
        "## Threshold Summary",
        "",
        "| Threshold | Events | Labeled Trades | Label Net | Worst Silo | Neg Silos | Avg Score | Avg Trade | Win Rate |",
        "|---|---:|---:|---:|---:|---:|---:|---:|---:|",
    ]
    for _, row in threshold_summary.iterrows():
        lines.append(
            f"| `{row['threshold_label']}` "
            f"| {int(row['selected_events'])} "
            f"| {int(row['selected_labeled_trades'])} "
            f"| {float(row['label_net_after_fee_pct']):.6f}% "
            f"| {float(row['label_worst_symbol_month_pct']):.6f}% "
            f"| {int(row['label_negative_symbol_months'])} "
            f"| {float(row['avg_selected_score_pct']):.6f}% "
            f"| {float(row['avg_labeled_trade_net_pct']):.6f}% "
            f"| {float(row['labeled_win_rate_pct']):.2f}% |"
        )
    lines.extend(
        [
            "",
            "## Read",
            "",
            "- This is an out-of-sample label audit for event selection, not a lifecycle result by itself.",
            "- A selected-event CSV must still be replayed with strict lifecycle and adverse/next-second checks.",
            "- Sparse groups back off to broader buckets, so high scores are intentionally conservative.",
        ]
    )
    (output_dir / "t3_bayesian_event_filter_report.md").write_text("\n".join(lines) + "\n", encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--trades", type=Path, default=DEFAULT_TRADES)
    parser.add_argument("--events", type=Path, default=DEFAULT_EVENTS)
    parser.add_argument("--output-dir", type=Path, default=DEFAULT_OUTPUT)
    parser.add_argument("--score-thresholds", nargs="+", type=float, default=DEFAULT_THRESHOLDS)
    parser.add_argument("--min-train-months", type=int, default=3)
    parser.add_argument("--min-group-trades", type=int, default=3)
    parser.add_argument("--prior-strength", type=float, default=8.0)
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    trades, events = load_inputs(Path(args.trades), Path(args.events))
    scored_events = walk_forward_scores(
        trades,
        events,
        min_train_months=int(args.min_train_months),
        min_group_trades=int(args.min_group_trades),
        prior_strength=float(args.prior_strength),
    )
    months = sorted(month for month in trades["month"].dropna().unique() if str(month) != "NaT")
    symbols = sorted(trades["symbol"].dropna().astype(str).unique())
    threshold_summary = summarize_thresholds(
        scored_events,
        trades,
        thresholds=list(args.score_thresholds),
        months=months,
        symbols=symbols,
    )
    write_outputs(
        output_dir=Path(args.output_dir),
        scored_events=scored_events,
        threshold_summary=threshold_summary,
        thresholds=list(args.score_thresholds),
        trades_path=Path(args.trades),
        events_path=Path(args.events),
        min_train_months=int(args.min_train_months),
        min_group_trades=int(args.min_group_trades),
        prior_strength=float(args.prior_strength),
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
