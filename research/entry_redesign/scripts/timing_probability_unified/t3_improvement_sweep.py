"""T3 improvement sweep for no-trade overlay gates.

This is a research-only diagnostic. It runs the current rolling T2/T3 union
pipeline, enriches T3 ledgers with raw T3 event features, then sweeps
train-calibrated T3 acceptance gates. The gates are evaluated on the current
forward month for each rolling window.
"""

from __future__ import annotations

import argparse
import itertools
import json
import logging
import sys
from dataclasses import asdict, dataclass
from pathlib import Path

import numpy as np
import pandas as pd

_SCRIPTS_DIR = Path(__file__).resolve().parents[1]
if str(_SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS_DIR))

from timing_probability_unified.multi_timeframe_builder import load_all_1s_bars
from timing_probability_unified.t3_event_generator import (
    T3EventConfig,
    generate_t3_events,
)
from timing_probability_unified.union_strategy_runner import (
    RollingWindowResult,
    UnionStrategyConfig,
    _active_trade_count,
    run_rolling_windows,
)

logger = logging.getLogger(__name__)


FEATURE_COLUMNS = [
    "event_id",
    "pre_touch_seconds",
    "touch_extension_atr",
    "eff_300s",
    "prev1_body_atr",
    "prev1_range_atr",
    "prev1_close_pos_side",
    "level_to_signal_open_atr",
    "shape",
]


@dataclass(frozen=True)
class T3GateSpec:
    """A train-calibrated T3 acceptance gate."""

    rf_min: float | None = None
    speed_abs_train_quantile: float | None = None
    pre_touch_max: float | None = None
    eff_max: float | None = None
    touch_extension_abs_max: float | None = None
    side: str = "all"

    def label(self) -> str:
        parts = []
        if self.rf_min is not None:
            parts.append(f"rf>={self.rf_min:.2f}")
        if self.speed_abs_train_quantile is not None:
            parts.append(f"speed_abs>=train_q{int(self.speed_abs_train_quantile * 100)}")
        if self.pre_touch_max is not None:
            parts.append(f"pre<={int(self.pre_touch_max)}")
        if self.eff_max is not None:
            parts.append(f"eff<={self.eff_max:.2f}")
        if self.touch_extension_abs_max is not None:
            parts.append(f"ext_abs<={self.touch_extension_abs_max:.2f}")
        if self.side != "all":
            parts.append(f"side={self.side}")
        return "+".join(parts) if parts else "baseline_active"


@dataclass
class GateMetrics:
    """Forward metrics for one candidate gate."""

    label: str
    total_cs: float
    worst_sm: float
    trade_count: int
    negative_months: int
    avg_weighted_pnl: float
    months: dict[str, float]
    thresholds: dict[str, dict[str, float]]
    spec: dict


def active_entries(trades: pd.DataFrame) -> pd.DataFrame:
    """Keep rows that represent actual entries under the current runner."""
    if trades.empty:
        return trades.copy()
    mask = (
        (trades["timing_prediction"] != "skip")
        & (trades["speed_gate_pass"] == True)  # noqa: E712
    )
    return trades[mask].copy()


def enrich_with_t3_features(
    trades: pd.DataFrame,
    t3_events: pd.DataFrame,
) -> pd.DataFrame:
    """Join raw T3 event features onto a trade ledger by event_id."""
    if trades.empty:
        return trades.copy()

    feature_cols = [c for c in FEATURE_COLUMNS if c in t3_events.columns]
    features = t3_events[feature_cols].drop_duplicates("event_id")
    enriched = trades.merge(features, on="event_id", how="left", validate="m:1")
    enriched["touch_time"] = pd.to_datetime(enriched["touch_time"], utc=True)
    return enriched


def train_thresholds(train_trades: pd.DataFrame, spec: T3GateSpec) -> dict[str, float]:
    """Compute train-only thresholds needed by a gate spec."""
    thresholds: dict[str, float] = {}
    active_train = active_entries(train_trades)
    if (
        spec.speed_abs_train_quantile is not None
        and not active_train.empty
        and "speed_300s_atr" in active_train.columns
    ):
        thresholds["speed_abs_min"] = float(
            np.nanquantile(
                active_train["speed_300s_atr"].abs(),
                spec.speed_abs_train_quantile,
            )
        )
    return thresholds


def apply_gate(
    trades: pd.DataFrame,
    spec: T3GateSpec,
    thresholds: dict[str, float],
) -> pd.DataFrame:
    """Apply an acceptance gate to active forward trades."""
    gated = active_entries(trades)
    if gated.empty:
        return gated

    mask = pd.Series(True, index=gated.index)
    if spec.rf_min is not None:
        mask &= gated["rf_probability"] >= spec.rf_min
    if "speed_abs_min" in thresholds:
        mask &= gated["speed_300s_atr"].abs() >= thresholds["speed_abs_min"]
    if spec.pre_touch_max is not None and "pre_touch_seconds" in gated.columns:
        mask &= gated["pre_touch_seconds"] <= spec.pre_touch_max
    if spec.eff_max is not None and "eff_300s" in gated.columns:
        mask &= gated["eff_300s"] <= spec.eff_max
    if (
        spec.touch_extension_abs_max is not None
        and "touch_extension_atr" in gated.columns
    ):
        mask &= gated["touch_extension_atr"].abs() <= spec.touch_extension_abs_max
    if spec.side != "all":
        mask &= gated["side"] == spec.side
    return gated[mask].copy()


def _monthly_returns(trades: pd.DataFrame) -> pd.Series:
    """Return per-month weighted PnL without emitting timezone warnings."""
    if trades.empty:
        return pd.Series(dtype=float)
    df = trades.copy()
    touch_time = pd.to_datetime(df["touch_time"], utc=True)
    df["year_month"] = touch_time.dt.strftime("%Y-%m")
    return df.groupby("year_month")["weighted_pnl"].sum()


def compute_gate_metrics(
    windows: list[tuple[str, pd.DataFrame, dict[str, float]]],
    spec: T3GateSpec,
) -> GateMetrics:
    """Compute aggregate forward metrics for a spec across rolling windows."""
    gated_parts = []
    threshold_by_window: dict[str, dict[str, float]] = {}

    for forward_start, forward_trades, thresholds in windows:
        gated = apply_gate(forward_trades, spec, thresholds)
        if not gated.empty:
            gated_parts.append(gated)
        threshold_by_window[forward_start] = thresholds

    if gated_parts:
        gated_all = pd.concat(gated_parts, ignore_index=True)
    else:
        gated_all = pd.DataFrame()

    trade_count = _active_trade_count(gated_all) if not gated_all.empty else 0

    monthly = _monthly_returns(gated_all)
    total_cs = float(monthly.sum()) if not monthly.empty else 0.0
    worst_sm = float(monthly.min()) if not monthly.empty else 0.0
    months = {str(k): round(float(v), 6) for k, v in monthly.items()}
    negative_months = int((monthly < 0).sum()) if not monthly.empty else 0

    avg_weighted_pnl = (
        float(gated_all["weighted_pnl"].sum() / trade_count)
        if trade_count > 0
        else 0.0
    )

    return GateMetrics(
        label=spec.label(),
        total_cs=round(float(total_cs), 6),
        worst_sm=round(float(worst_sm), 6),
        trade_count=trade_count,
        negative_months=negative_months,
        avg_weighted_pnl=round(avg_weighted_pnl, 6),
        months=months,
        thresholds=threshold_by_window,
        spec=asdict(spec),
    )


def build_gate_specs() -> list[T3GateSpec]:
    """Build a bounded discovery grid for T3 no-trade overlays."""
    rf_mins = [None, 0.55, 0.60, 0.65, 0.70]
    speed_qs = [None, 0.25, 0.50, 0.75]
    pre_touch_maxes = [None, 900.0, 600.0, 300.0]
    eff_maxes = [None, 0.75, 0.50]
    ext_maxes = [None, 0.75, 0.50]
    sides = ["all", "long", "short"]

    specs = [
        T3GateSpec(
            rf_min=rf_min,
            speed_abs_train_quantile=speed_q,
            pre_touch_max=pre_touch_max,
            eff_max=eff_max,
            touch_extension_abs_max=ext_max,
            side=side,
        )
        for rf_min, speed_q, pre_touch_max, eff_max, ext_max, side in itertools.product(
            rf_mins,
            speed_qs,
            pre_touch_maxes,
            eff_maxes,
            ext_maxes,
            sides,
        )
    ]
    return specs


def collect_t3_windows(
    rolling: RollingWindowResult,
    t3_events: pd.DataFrame,
) -> list[tuple[str, pd.DataFrame, pd.DataFrame]]:
    """Return (forward_start, train_trades, forward_trades) for each window."""
    rows = []
    for window in rolling.window_results:
        forward_start = window.config.forward_start
        all_trades = enrich_with_t3_features(window.t3_result.all_trades, t3_events)
        forward_trades = enrich_with_t3_features(
            window.t3_result.forward_trades,
            t3_events,
        )
        train_trades = all_trades[all_trades.get("split") == "train"].copy()
        rows.append((forward_start, train_trades, forward_trades))
    return rows


def run_t3_gate_sweep(config: UnionStrategyConfig) -> tuple[list[GateMetrics], RollingWindowResult]:
    """Run rolling pipeline and sweep T3 acceptance gates."""
    rolling = run_rolling_windows(config)

    bars_1s = load_all_1s_bars("ETHUSDT")
    t3_events = generate_t3_events(bars_1s, T3EventConfig(symbol="ETHUSDT"))
    t3_windows = collect_t3_windows(rolling, t3_events)

    specs = build_gate_specs()
    scored_windows = []
    for spec in specs:
        per_window = []
        for forward_start, train_trades, forward_trades in t3_windows:
            thresholds = train_thresholds(train_trades, spec)
            per_window.append((forward_start, forward_trades, thresholds))
        scored_windows.append(compute_gate_metrics(per_window, spec))

    scored_windows.sort(
        key=lambda row: (
            row.total_cs,
            -row.negative_months,
            row.trade_count,
            row.worst_sm,
        ),
        reverse=True,
    )
    return scored_windows, rolling


def write_outputs(
    metrics: list[GateMetrics],
    rolling: RollingWindowResult,
    output_dir: Path,
    top_n: int,
) -> None:
    """Write JSON, CSV, and Markdown reports."""
    output_dir.mkdir(parents=True, exist_ok=True)

    baseline_t3 = next(m for m in metrics if m.label == "baseline_active")
    t2_total = round(
        float(sum(w.t2_result.calendar_sum for w in rolling.window_results)),
        6,
    )
    top = metrics[:top_n]
    top_min_4 = [m for m in metrics if m.trade_count >= 4][:top_n]
    top_min_8 = [m for m in metrics if m.trade_count >= 8][:top_n]

    payload = {
        "note": (
            "Discovery-only T3 no-trade overlay sweep. Speed thresholds are "
            "calibrated on each window's train split; candidate selection is "
            "still based on the observed forward window and needs holdout."
        ),
        "baseline": {
            "t2_total_cs": t2_total,
            "t3_total_cs": baseline_t3.total_cs,
            "union_total_cs": round(t2_total + baseline_t3.total_cs, 6),
            "t3_trades": baseline_t3.trade_count,
            "t3_months": baseline_t3.months,
        },
        "top": [asdict(m) for m in top],
        "top_min_4_trades": [asdict(m) for m in top_min_4],
        "top_min_8_trades": [asdict(m) for m in top_min_8],
    }

    (output_dir / "t3_gate_sweep_summary.json").write_text(
        json.dumps(payload, indent=2, ensure_ascii=False),
        encoding="utf-8",
    )

    csv_rows = []
    for metric in metrics:
        row = asdict(metric)
        row.pop("thresholds", None)
        row.pop("spec", None)
        row["months"] = json.dumps(metric.months, sort_keys=True)
        row["union_total_cs"] = round(t2_total + metric.total_cs, 6)
        csv_rows.append(row)
    pd.DataFrame(csv_rows).to_csv(output_dir / "t3_gate_sweep_candidates.csv", index=False)

    lines = [
        "# T3 Gate Sweep Report",
        "",
        "Discovery-only no-trade overlay sweep. Speed thresholds are train-calibrated per window; promoted candidates still need fixed holdout validation.",
        "",
        "## Baseline",
        "",
        f"- T2 total CS: {t2_total:.6f}",
        f"- T3 baseline CS: {baseline_t3.total_cs:.6f} ({baseline_t3.trade_count} trades)",
        f"- Union baseline CS: {t2_total + baseline_t3.total_cs:.6f}",
        "",
        "## Top Candidates",
        "",
        "| Rank | Label | T3 CS | Union CS | Trades | Neg Months | Worst SM | Months |",
        "|---:|---|---:|---:|---:|---:|---:|---|",
    ]
    for idx, metric in enumerate(top_min_4, start=1):
        lines.append(
            f"| {idx} | `{metric.label}` | {metric.total_cs:.6f} "
            f"| {t2_total + metric.total_cs:.6f} | {metric.trade_count} "
            f"| {metric.negative_months} | {metric.worst_sm:.6f} "
            f"| `{json.dumps(metric.months, sort_keys=True)}` |"
        )

    (output_dir / "t3_gate_sweep_report.md").write_text(
        "\n".join(lines),
        encoding="utf-8",
    )


def main() -> None:
    parser = argparse.ArgumentParser(description="T3 improvement gate sweep")
    parser.add_argument("--output-dir", default="/tmp/bktrader_t3_gate_sweep")
    parser.add_argument("--top-n", type=int, default=20)
    args = parser.parse_args()

    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
    )

    config = UnionStrategyConfig(
        rolling_months=["2026-02-01", "2026-03-01", "2026-04-01"],
    )
    metrics, rolling = run_t3_gate_sweep(config)
    write_outputs(metrics, rolling, Path(args.output_dir), args.top_n)
    best = next(m for m in metrics if m.trade_count >= 4)
    print(
        f"Best min-4 candidate: {best.label} "
        f"T3_CS={best.total_cs:.6f}, trades={best.trade_count}, months={best.months}"
    )
    print(f"Reports: {Path(args.output_dir)}/t3_gate_sweep_{{summary.json,report.md,candidates.csv}}")


if __name__ == "__main__":
    main()
