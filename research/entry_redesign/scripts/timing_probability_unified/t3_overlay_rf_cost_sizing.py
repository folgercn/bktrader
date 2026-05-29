"""Research-only RF/cost sizing audit for the ETH T3 overlay.

This script keeps the current T3 overlay event source fixed, then changes only
the event-level notional multiplier. It is a sizing audit, not a new lifecycle
entry rule:

- base overlay: ``all_speed_abs_ge_0p35`` ETH T3 external events
- entry model: ``next_second_adverse``
- exit contract: strict T3 60m lifecycle
- base size: current overlay 2.0x schedule

The RF variants are intentionally evaluated as walk-forward overlays before any
live wiring. Frozen lead RF scores are reported separately because prior audits
showed they are not monotonic on T3.
"""

from __future__ import annotations

import argparse
import json
import logging
import sys
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Any

import numpy as np
import pandas as pd
from sklearn.ensemble import RandomForestClassifier

PROJECT_ROOT = Path(__file__).resolve().parents[4]
RESEARCH_DIR = PROJECT_ROOT / "research"
SCRIPTS_DIR = Path(__file__).resolve().parents[1]
for path in (RESEARCH_DIR, SCRIPTS_DIR):
    if str(path) not in sys.path:
        sys.path.insert(0, str(path))

import eth_q1_breakout_t3_shape_compare as lifecycle  # noqa: E402
from timing_probability_unified.t2_lifecycle_context_sizing import EXTENDED_MONTHS  # noqa: E402
from timing_probability_unified.t3_filtered_external_event_lifecycle import (  # noqa: E402
    T3_60M_EXIT_OVERRIDES,
    FilteredT3Spec,
    apply_spec,
    load_scored_events,
)
from timing_probability_unified.t3_lifecycle_outcome_diagnostics import pair_lifecycle_trades  # noqa: E402
from timing_probability_unified.t3_overlay_lead_exposure_audit import (  # noqa: E402
    _apply_round_trip_fee_adjustment,
    _window_events,
)
from timing_probability_unified.t3_pre_touch_lifecycle import (  # noqa: E402
    INITIAL_BALANCE,
    T3_REENTRY_SIZE_SCHEDULE,
    _load_window_bars,
    _month_bounds,
    _patched_replay_kwargs,
)

logger = logging.getLogger(__name__)

OUTPUT_DIR = (
    PROJECT_ROOT
    / "research"
    / "entry_redesign"
    / "scripts"
    / "output"
    / "timing_probability_unified"
)
DEFAULT_SCORED_EVENTS = OUTPUT_DIR / "t3_probability_overlay_extended" / "t3_probability_overlay_scored_events.csv"
DEFAULT_OUTPUT = OUTPUT_DIR / "t3_overlay_rf_cost_sizing_20260520"
BASELINE_LEAD_ADVERSE10_PCT = 22.971648
REFERENCE_FIXED_OVERLAY_QUANTITY = 0.08
SHADOW_QUANTITY_MIN = 0.20
SHADOW_QUANTITY_MAX = 0.40

FEATURE_COLUMNS = [
    "rf_probability",
    "speed_300s_abs",
    "eff_300s",
    "touch_extension_abs",
    "pre_touch_seconds",
    "roundtrip_cost_atr",
    "side_is_short",
]

REQUIRED_CONTEXT_COLUMNS = {
    "external_event_key",
    "rf_probability",
    "speed_300s_atr",
    "eff_300s",
    "touch_extension_atr",
    "pre_touch_seconds",
}


def _path_label(path: Path | str) -> str:
    resolved = Path(path)
    try:
        return str(resolved.resolve().relative_to(PROJECT_ROOT))
    except (OSError, ValueError):
        return str(resolved)


@dataclass(frozen=True)
class SizingVariant:
    """One event-level sizing rule."""

    label: str
    method: str
    max_multiplier: float = 1.0
    min_multiplier: float = 0.0
    min_quantity: float = 0.0
    max_quantity: float = 0.0
    reference_quantity: float = REFERENCE_FIXED_OVERLAY_QUANTITY
    live_compatible: bool = True
    read: str = ""


@dataclass(frozen=True)
class SizingMetrics:
    """Aggregate metrics for one sizing variant."""

    variant: str
    method: str
    live_compatible: bool
    calendar_sum_pct: float
    lead_adverse10_plus_overlay_pct: float
    overlay_delta_vs_fixed_pct: float
    worst_month_pct: float
    negative_months: int
    max_drawdown_pct: float
    filled_trades: int
    active_events: int
    zeroed_events: int
    avg_event_multiplier: float
    median_event_multiplier: float
    max_event_multiplier: float
    avg_event_quantity: float
    median_event_quantity: float
    min_event_quantity: float
    max_event_quantity: float
    allocated_notional_share: float
    by_month: dict[str, float]


def build_variants() -> list[SizingVariant]:
    """Return the fixed baseline plus RF/cost sizing candidates."""
    return [
        SizingVariant(
            label="fixed_overlay_2p0",
            method="fixed",
            max_multiplier=1.0,
            min_multiplier=1.0,
            live_compatible=True,
            read="Current fixed T3 overlay 2.0x sizing.",
        ),
        SizingVariant(
            label="frozen_lead_rf_cost_max1",
            method="frozen_rf_cost",
            max_multiplier=1.0,
            min_multiplier=0.0,
            live_compatible=True,
            read="Frozen lead RF/cost score, capped so it can only reduce current 2.0x overlay size.",
        ),
        SizingVariant(
            label="frozen_lead_rf_cost_floor0p25_max1",
            method="frozen_rf_cost",
            max_multiplier=1.0,
            min_multiplier=0.25,
            live_compatible=True,
            read="Frozen lead RF/cost score with a 0.25 floor to avoid binary-style overfiltering.",
        ),
        SizingVariant(
            label="frozen_lead_rf_cost_floor0p75_max1p25_shadow",
            method="frozen_rf_cost",
            max_multiplier=1.25,
            min_multiplier=0.75,
            live_compatible=False,
            read=(
                "Shadow candidate using the already-loaded frozen lead RF model on T3 event features; "
                "maps the current 0.08 ETH overlay into a 0.75x-1.25x band."
            ),
        ),
        SizingVariant(
            label="wf_t3_rf_cost_linear_max1",
            method="wf_t3_rf_linear",
            max_multiplier=1.0,
            min_multiplier=0.0,
            live_compatible=True,
            read="T3-specific walk-forward RF probability mapped linearly and capped at current overlay 2.0x.",
        ),
        SizingVariant(
            label="wf_t3_rf_cost_rank_max1",
            method="wf_t3_rf_rank",
            max_multiplier=1.0,
            min_multiplier=0.0,
            live_compatible=True,
            read="T3-specific walk-forward RF rank buckets: bottom 40% off, middle half, top full size.",
        ),
        SizingVariant(
            label="wf_t3_rf_cost_linear_max1p25_research",
            method="wf_t3_rf_linear",
            max_multiplier=1.25,
            min_multiplier=0.0,
            live_compatible=False,
            read="Research-only aggressive check that can exceed current overlay 2.0x by 25%.",
        ),
        SizingVariant(
            label="wf_t3_rf_cost_linear_floor0p75_max1p25_shadow",
            method="wf_t3_rf_linear",
            max_multiplier=1.25,
            min_multiplier=0.75,
            live_compatible=False,
            read=(
                "Shadow candidate: T3-specific walk-forward RF maps the current 0.08 ETH overlay "
                "into a 0.75x-1.25x band, or 0.06-0.10 ETH before exchange precision."
            ),
        ),
        SizingVariant(
            label="wf_t3_rf_cost_quantity_0p20_0p40_shadow",
            method="wf_t3_rf_quantity",
            max_multiplier=SHADOW_QUANTITY_MAX / REFERENCE_FIXED_OVERLAY_QUANTITY,
            min_multiplier=SHADOW_QUANTITY_MIN / REFERENCE_FIXED_OVERLAY_QUANTITY,
            min_quantity=SHADOW_QUANTITY_MIN,
            max_quantity=SHADOW_QUANTITY_MAX,
            reference_quantity=REFERENCE_FIXED_OVERLAY_QUANTITY,
            live_compatible=False,
            read=(
                "Shadow risk-on candidate: T3-specific walk-forward RF maps event probability "
                "directly into an absolute 0.20-0.40 ETH T3 overlay quantity band."
            ),
        ),
    ]


def _equity_max_drawdown_pct(values: pd.Series) -> float:
    if values.empty:
        return 0.0
    curve = pd.concat([pd.Series([0.0]), values.astype(float).cumsum().reset_index(drop=True)], ignore_index=True)
    drawdown = curve - curve.cummax()
    return round(float(drawdown.min()), 6)


def _month_grid(months: list[str], values: pd.Series) -> dict[str, float]:
    return {month: round(float(values.get(month, 0.0)), 6) for month in months}


def _numeric(frame: pd.DataFrame, column: str, default: float = 0.0) -> pd.Series:
    if column not in frame.columns:
        return pd.Series(default, index=frame.index, dtype=float)
    return pd.to_numeric(frame[column], errors="coerce").fillna(default).astype(float)


def _cost_penalty(events: pd.DataFrame, threshold_atr: float) -> pd.Series:
    existing = _numeric(events, "cost_penalty", 1.0).clip(lower=0.0, upper=1.0)
    cost = _numeric(events, "roundtrip_cost_atr", threshold_atr)
    computed = pd.Series(1.0, index=events.index, dtype=float)
    positive = cost > 0
    computed.loc[positive] = (float(threshold_atr) / cost.loc[positive]).clip(lower=0.25, upper=1.0)
    return pd.concat([existing, computed], axis=1).min(axis=1)


def _variant_reference_quantity(variant: SizingVariant) -> float:
    return float(variant.reference_quantity or REFERENCE_FIXED_OVERLAY_QUANTITY)


def _quantity_band_from_probability(
    probability: pd.Series | np.ndarray,
    cost_penalty: pd.Series,
    variant: SizingVariant,
) -> pd.Series:
    index = cost_penalty.index
    probability_series = pd.Series(probability, index=index, dtype=float).clip(lower=0.0, upper=1.0)
    min_quantity = float(variant.min_quantity or SHADOW_QUANTITY_MIN)
    max_quantity = float(variant.max_quantity or SHADOW_QUANTITY_MAX)
    if min_quantity > max_quantity:
        min_quantity = max_quantity
    raw_quantity = min_quantity + probability_series * (max_quantity - min_quantity)
    return (raw_quantity * cost_penalty).clip(lower=min_quantity, upper=max_quantity)


def _prepare_features(events: pd.DataFrame) -> pd.DataFrame:
    out = events.copy()
    out["speed_300s_abs"] = _numeric(out, "speed_300s_atr").abs()
    out["touch_extension_abs"] = _numeric(out, "touch_extension_atr").abs()
    out["side_is_short"] = (out.get("side", pd.Series("", index=out.index)).astype(str) == "short").astype(float)
    for column in FEATURE_COLUMNS:
        if column not in out.columns:
            out[column] = 0.0
    return out


def build_event_table(trades: pd.DataFrame, *, initial_balance: float) -> pd.DataFrame:
    """Collapse paired trades to one row per external T3 event."""
    missing = sorted(column for column in REQUIRED_CONTEXT_COLUMNS if column not in trades.columns)
    if missing:
        raise ValueError(
            "overlay trades are missing event metadata columns: "
            + ", ".join(missing)
            + ". Regenerate them with the current pair_lifecycle_trades helper."
        )

    work = trades.copy()
    work["entry_time"] = pd.to_datetime(work["entry_time"], utc=True)
    work["exit_time"] = pd.to_datetime(work["exit_time"], utc=True)
    work["event_month"] = work["month"].astype(str)
    work["event_pnl_pct"] = _numeric(work, "pnl_initial_pct")
    work["event_notional_share"] = _numeric(work, "notional") / float(initial_balance)
    first_cols = [
        "external_event_key",
        "event_month",
        "symbol",
        "side",
        "external_touch_time",
        "signal_start",
        "timing_prediction",
        "rf_probability",
        "sizing_multiplier",
        "cost_penalty",
        "roundtrip_cost_atr",
        "speed_300s_atr",
        "eff_300s",
        "touch_extension_atr",
        "pre_touch_seconds",
    ]
    first_cols = [column for column in first_cols if column in work.columns]
    grouped = work.sort_values("entry_time").groupby("external_event_key", dropna=False)
    events = grouped[first_cols].first()
    events["event_trades"] = grouped.size()
    events["event_net_pnl_pct"] = grouped["event_pnl_pct"].sum()
    events["event_notional_share"] = grouped["event_notional_share"].sum()
    events["first_entry_time"] = grouped["entry_time"].min()
    events["last_exit_time"] = grouped["exit_time"].max()
    events["label_win"] = (events["event_net_pnl_pct"] > 0.0).astype(int)
    events = events.reset_index(drop=True)
    return _prepare_features(events)


def collect_overlay_trades(
    *,
    scored_events: Path,
    months: list[str],
    symbol: str,
    timeframe: str,
    initial_balance: float,
    external_entry_mode: str,
    t3_size_scale: float,
    reentry_fill_policy: str,
    early_horizon_seconds: int,
) -> pd.DataFrame:
    """Replay the base T3 overlay and return fee-net paired lifecycle trades."""
    spec = FilteredT3Spec("all_speed_abs_ge_0p35", speed_abs_min=0.35)
    events = apply_spec(load_scored_events(scored_events), spec)
    parts: list[pd.DataFrame] = []
    for month in months:
        logger.info("Collecting T3 overlay trades %s %s", symbol, month)
        start, end = _month_bounds(month)
        external_events = _window_events(events, symbol, start, end)
        second_bars = _load_window_bars(symbol, start, end)
        _, signal = lifecycle.build_signal_frame(second_bars, timeframe)
        t3_schedule = [float(size) * float(t3_size_scale) for size in T3_REENTRY_SIZE_SCHEDULE]
        with _patched_replay_kwargs(symbol):
            ledger, _diagnostics = lifecycle.run_second_bar_replay(
                second_bars,
                signal,
                initial_balance=initial_balance,
                breakout_shape="baseline_plus_t3",
                replay_mode="live_intrabar_sma5",
                t3_reentry_size_schedule=t3_schedule,
                t3_cooldown_bars=0,
                t3_quality_filters={"allowed_sides": []},
                quality_filter_shapes=["t3_swing"],
                shape_sizing_filters={"allowed_sides": []},
                sizing_filter_shapes=["original_t2"],
                sizing_filter_fail_multiplier=0.0,
                sizing_filter_fail_action="skip_lock",
                t3_exit_overrides=dict(T3_60M_EXIT_OVERRIDES),
                external_breakout_events=external_events,
                external_breakout_shape_name="t3_swing",
                external_entry_mode=external_entry_mode,
                reentry_fill_policy=reentry_fill_policy,
            )
        trades = pair_lifecycle_trades(
            ledger,
            second_bars,
            symbol=symbol,
            month=month,
            initial_balance=initial_balance,
            early_horizon_seconds=early_horizon_seconds,
        )
        if not trades.empty:
            trades = trades[trades["breakout_shape_name"] == "t3_swing"].copy()
            trades = _apply_round_trip_fee_adjustment(trades, initial_balance=initial_balance)
            parts.append(trades)
    return pd.concat(parts, ignore_index=True) if parts else pd.DataFrame()


def _fit_rf(train: pd.DataFrame, feature_columns: list[str], random_state: int) -> RandomForestClassifier | None:
    labels = train["label_win"].astype(int)
    if len(train) < 12 or labels.nunique() < 2:
        return None
    model = RandomForestClassifier(
        n_estimators=240,
        max_depth=3,
        min_samples_leaf=4,
        random_state=random_state,
        class_weight="balanced_subsample",
    )
    model.fit(train[feature_columns], labels)
    return model


def _predict_class_one(model: RandomForestClassifier, frame: pd.DataFrame, feature_columns: list[str]) -> np.ndarray:
    proba = model.predict_proba(frame[feature_columns])
    class_lookup = {int(label): idx for idx, label in enumerate(model.classes_)}
    if 1 not in class_lookup:
        return np.full(len(frame), 0.5)
    return proba[:, class_lookup[1]]


def score_events_for_variant(
    events: pd.DataFrame,
    variant: SizingVariant,
    *,
    months: list[str],
    cost_threshold_atr: float,
    random_state: int,
) -> pd.DataFrame:
    """Attach event multiplier and model diagnostics for one variant."""
    scored = events.copy()
    scored["sizing_variant"] = variant.label
    scored["model_probability"] = np.nan
    scored["model_status"] = "not_used"
    scored["event_quantity"] = np.nan

    if variant.method == "fixed":
        scored["event_multiplier"] = 1.0
        scored["event_quantity"] = _variant_reference_quantity(variant)
        scored["model_status"] = "fixed"
        return scored

    cost = _cost_penalty(scored, cost_threshold_atr)

    if variant.method == "frozen_rf_cost":
        rf = _numeric(scored, "rf_probability", 0.5)
        scored["model_probability"] = rf
        scored["event_multiplier"] = (rf * 2.0).clip(lower=variant.min_multiplier, upper=variant.max_multiplier) * cost
        scored["event_multiplier"] = scored["event_multiplier"].clip(lower=variant.min_multiplier, upper=variant.max_multiplier)
        scored["event_quantity"] = scored["event_multiplier"] * _variant_reference_quantity(variant)
        scored["model_status"] = "frozen_lead_rf"
        return scored

    if variant.method not in {"wf_t3_rf_linear", "wf_t3_rf_rank", "wf_t3_rf_quantity"}:
        raise ValueError(f"unknown sizing method: {variant.method}")

    if variant.method == "wf_t3_rf_quantity":
        warmup_probability = pd.Series(0.5, index=scored.index, dtype=float)
        warmup_quantity = _quantity_band_from_probability(warmup_probability, cost, variant)
        scored["model_probability"] = warmup_probability
        scored["event_quantity"] = warmup_quantity
        scored["event_multiplier"] = warmup_quantity / _variant_reference_quantity(variant)
        scored["model_status"] = "warmup_mid_quantity"
    else:
        scored["event_multiplier"] = 1.0
        scored["event_quantity"] = _variant_reference_quantity(variant)
        scored["model_status"] = "warmup_fixed"
    for month in months:
        month_mask = scored["event_month"] == month
        if not month_mask.any():
            continue
        train = scored[scored["event_month"] < month].copy()
        current = scored[month_mask].copy()
        model = _fit_rf(train, FEATURE_COLUMNS, random_state=random_state)
        if model is None:
            scored.loc[month_mask, "model_probability"] = 0.5
            continue

        train_prob = _predict_class_one(model, train, FEATURE_COLUMNS)
        current_prob = _predict_class_one(model, current, FEATURE_COLUMNS)
        month_cost = cost.loc[month_mask]
        scored.loc[month_mask, "model_probability"] = current_prob
        scored.loc[month_mask, "model_status"] = "walk_forward_rf"
        if variant.method == "wf_t3_rf_linear":
            multiplier = pd.Series(current_prob * 2.0, index=current.index)
            multiplier = multiplier.clip(lower=variant.min_multiplier, upper=variant.max_multiplier) * month_cost
            multiplier = multiplier.clip(lower=variant.min_multiplier, upper=variant.max_multiplier)
            scored.loc[month_mask, "event_multiplier"] = multiplier
            scored.loc[month_mask, "event_quantity"] = multiplier * _variant_reference_quantity(variant)
        elif variant.method == "wf_t3_rf_quantity":
            quantity = _quantity_band_from_probability(current_prob, month_cost, variant)
            scored.loc[month_mask, "event_quantity"] = quantity
            scored.loc[month_mask, "event_multiplier"] = quantity / _variant_reference_quantity(variant)
        else:
            q40 = float(np.quantile(train_prob, 0.40))
            q60 = float(np.quantile(train_prob, 0.60))
            multiplier = pd.Series(1.0, index=current.index)
            multiplier.loc[current_prob < q40] = 0.0
            multiplier.loc[(current_prob >= q40) & (current_prob < q60)] = 0.5
            multiplier = multiplier.clip(lower=variant.min_multiplier, upper=variant.max_multiplier) * month_cost
            multiplier = multiplier.clip(lower=variant.min_multiplier, upper=variant.max_multiplier)
            scored.loc[month_mask, "event_multiplier"] = multiplier
            scored.loc[month_mask, "event_quantity"] = multiplier * _variant_reference_quantity(variant)
    return scored


def apply_event_scores_to_trades(trades: pd.DataFrame, event_scores: pd.DataFrame, *, initial_balance: float) -> pd.DataFrame:
    scored = trades.merge(
        event_scores[["external_event_key", "event_multiplier", "event_quantity", "model_probability", "model_status"]],
        on="external_event_key",
        how="left",
        validate="m:1",
    )
    scored["event_multiplier"] = _numeric(scored, "event_multiplier", 1.0)
    scored["event_quantity"] = _numeric(scored, "event_quantity", REFERENCE_FIXED_OVERLAY_QUANTITY)
    scored["weighted_pnl_pct"] = _numeric(scored, "pnl_initial_pct") * scored["event_multiplier"]
    scored["weighted_notional_share"] = _numeric(scored, "notional") / float(initial_balance) * scored["event_multiplier"]
    scored["entry_time"] = pd.to_datetime(scored["entry_time"], utc=True)
    scored["exit_time"] = pd.to_datetime(scored["exit_time"], utc=True)
    return scored


def summarize_variant(
    *,
    variant: SizingVariant,
    trades: pd.DataFrame,
    event_scores: pd.DataFrame,
    months: list[str],
    fixed_overlay_pct: float,
    baseline_lead_adverse10_pct: float,
) -> SizingMetrics:
    active_events = event_scores[event_scores["event_multiplier"] > 0.0]
    active_trades = trades[trades["event_multiplier"] > 0.0]
    monthly = trades.groupby("month")["weighted_pnl_pct"].sum() if not trades.empty else pd.Series(dtype=float)
    by_month = _month_grid(months, monthly)
    calendar_sum = round(float(sum(by_month.values())), 6)
    event_multipliers = pd.to_numeric(event_scores["event_multiplier"], errors="coerce").fillna(0.0)
    event_quantities = pd.to_numeric(event_scores["event_quantity"], errors="coerce").fillna(0.0)
    return SizingMetrics(
        variant=variant.label,
        method=variant.method,
        live_compatible=variant.live_compatible,
        calendar_sum_pct=calendar_sum,
        lead_adverse10_plus_overlay_pct=round(float(baseline_lead_adverse10_pct) + calendar_sum, 6),
        overlay_delta_vs_fixed_pct=round(calendar_sum - fixed_overlay_pct, 6),
        worst_month_pct=round(float(min(by_month.values(), default=0.0)), 6),
        negative_months=int(sum(1 for value in by_month.values() if value < 0.0)),
        max_drawdown_pct=_equity_max_drawdown_pct(trades.sort_values("exit_time")["weighted_pnl_pct"]),
        filled_trades=int(len(active_trades)),
        active_events=int(len(active_events)),
        zeroed_events=int((event_multipliers <= 0.0).sum()),
        avg_event_multiplier=round(float(event_multipliers.mean()), 6) if len(event_multipliers) else 0.0,
        median_event_multiplier=round(float(event_multipliers.median()), 6) if len(event_multipliers) else 0.0,
        max_event_multiplier=round(float(event_multipliers.max()), 6) if len(event_multipliers) else 0.0,
        avg_event_quantity=round(float(event_quantities.mean()), 6) if len(event_quantities) else 0.0,
        median_event_quantity=round(float(event_quantities.median()), 6) if len(event_quantities) else 0.0,
        min_event_quantity=round(float(event_quantities.min()), 6) if len(event_quantities) else 0.0,
        max_event_quantity=round(float(event_quantities.max()), 6) if len(event_quantities) else 0.0,
        allocated_notional_share=round(float(trades["weighted_notional_share"].sum()), 6) if not trades.empty else 0.0,
        by_month=by_month,
    )


def _markdown_table(metrics: list[SizingMetrics]) -> str:
    if not metrics:
        return "_empty_"
    lines = [
        "| Variant | Live-compatible | Overlay PnL | Delta vs fixed | Lead adverse10 + overlay | Worst Month | Neg Months | DD | Events | Avg Mult | Avg Qty | Max Qty | Read |",
        "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|",
    ]
    reads = {variant.label: variant.read for variant in build_variants()}
    for row in metrics:
        lines.append(
            f"| `{row.variant}` | {str(row.live_compatible).lower()} "
            f"| {row.calendar_sum_pct:.6f}% "
            f"| {row.overlay_delta_vs_fixed_pct:.6f}pp "
            f"| {row.lead_adverse10_plus_overlay_pct:.6f}% "
            f"| {row.worst_month_pct:.6f}% "
            f"| {row.negative_months} "
            f"| {row.max_drawdown_pct:.6f}% "
            f"| {row.active_events} "
            f"| {row.avg_event_multiplier:.6f} "
            f"| {row.avg_event_quantity:.6f} "
            f"| {row.max_event_quantity:.6f} "
            f"| {reads.get(row.variant, '')} |"
        )
    return "\n".join(lines)


def _verdict_lines(metrics: list[SizingMetrics]) -> list[str]:
    lookup = {row.variant: row for row in metrics}
    fixed = lookup.get("fixed_overlay_2p0")
    live_rows = [row for row in metrics if row.live_compatible and row.variant != "fixed_overlay_2p0"]
    research_rows = [row for row in metrics if not row.live_compatible]
    if fixed is None:
        return ["- Fixed overlay baseline is missing; verdict cannot be computed."]

    lines = []
    best_live = max(live_rows, key=lambda row: row.calendar_sum_pct) if live_rows else None
    if best_live is None:
        lines.append("- No live-compatible RF/cost variant was evaluated.")
    elif best_live.calendar_sum_pct > fixed.calendar_sum_pct:
        lines.append(
            f"- Best live-compatible RF/cost variant is `{best_live.variant}` with "
            f"{best_live.calendar_sum_pct:.6f}% overlay PnL, "
            f"{best_live.overlay_delta_vs_fixed_pct:.6f}pp above fixed."
        )
    else:
        lines.append(
            f"- No live-compatible RF/cost variant beat fixed overlay 2.0x; best was "
            f"`{best_live.variant}` at {best_live.calendar_sum_pct:.6f}% "
            f"({best_live.overlay_delta_vs_fixed_pct:.6f}pp vs fixed)."
        )

    best_research = max(research_rows, key=lambda row: row.calendar_sum_pct) if research_rows else None
    if best_research is not None:
        lines.append(
            f"- Best research-only variant is `{best_research.variant}` at "
            f"{best_research.calendar_sum_pct:.6f}% "
            f"({best_research.overlay_delta_vs_fixed_pct:.6f}pp vs fixed), but it can exceed "
            "the current 2.0x overlay cap and therefore needs a separate shadow risk decision."
        )
    return lines


def write_outputs(
    *,
    output_dir: Path,
    base_trades: pd.DataFrame,
    event_scores: pd.DataFrame,
    weighted_trades: pd.DataFrame,
    metrics: list[SizingMetrics],
    months: list[str],
    scored_events: Path,
    overlay_trades: Path | None,
    cost_threshold_atr: float,
    baseline_lead_adverse10_pct: float,
) -> None:
    output_dir.mkdir(parents=True, exist_ok=True)
    cached_base_trades = output_dir / "t3_overlay_rf_cost_base_trades.csv"
    overlay_trades_source = "replayed in this run"
    if overlay_trades is not None:
        overlay_trades_source = (
            "cached base trades in output dir"
            if overlay_trades.resolve() == cached_base_trades.resolve()
            else _path_label(overlay_trades)
        )
    base_trades.to_csv(output_dir / "t3_overlay_rf_cost_base_trades.csv", index=False)
    event_scores.to_csv(output_dir / "t3_overlay_rf_cost_event_scores.csv", index=False)
    weighted_trades.to_csv(output_dir / "t3_overlay_rf_cost_weighted_trades.csv", index=False)
    payload = {
        "note": (
            "Research-only T3 overlay event-level RF/cost sizing audit. The base lifecycle event source, "
            "entry mode, and exit contract are fixed; variants change only notional multiplier."
        ),
        "scored_events": _path_label(scored_events),
        "overlay_trades_input": overlay_trades_source,
        "months": months,
        "cost_threshold_atr": float(cost_threshold_atr),
        "baseline_lead_adverse10_pct": float(baseline_lead_adverse10_pct),
        "metrics": [asdict(row) for row in metrics],
    }
    (output_dir / "t3_overlay_rf_cost_sizing_summary.json").write_text(
        json.dumps(payload, indent=2, ensure_ascii=False, default=str) + "\n",
        encoding="utf-8",
    )
    lines = [
        "# T3 Overlay RF/Cost Sizing",
        "",
        "Research-only audit for applying event-level RF/cost sizing to the current ETH T3 overlay.",
        "",
        f"- Scored events: `{_path_label(scored_events)}`",
        f"- Overlay trades input: `{overlay_trades_source}`",
        f"- Months: {', '.join(months)}",
        f"- Cost threshold ATR: `{cost_threshold_atr}`",
        f"- Baseline lead adverse10: `{baseline_lead_adverse10_pct:.6f}%`",
        "",
        "## Variant Summary",
        "",
        _markdown_table(metrics),
        "",
        "## Verdict",
        "",
        *_verdict_lines(metrics),
        "",
        "## Read",
        "",
        "- `fixed_overlay_2p0` is the current T3 overlay sizing reference.",
        "- `live-compatible=true` rows never exceed the current fixed 2.0x overlay notional; they can only downweight events.",
        "- `shadow` rows intentionally exceed the current fixed overlay notional; the promoted risk-on row maps RF/cost quality into a 0.20-0.40 ETH testnet-shadow quantity band.",
        "- The T3-WF-RF quantity-band row is walk-forward evidence. Live/testnet wiring uses a separate accumulated-history T3 model artifact and must be monitored as shadow telemetry before mainnet consideration.",
        "- A promoted variant should beat fixed overlay PnL without making worst-month or drawdown materially worse.",
    ]
    (output_dir / "t3_overlay_rf_cost_sizing_report.md").write_text("\n".join(lines) + "\n", encoding="utf-8")


def run(
    *,
    output_dir: Path,
    scored_events: Path,
    overlay_trades: Path | None,
    months: list[str],
    symbol: str,
    timeframe: str,
    initial_balance: float,
    external_entry_mode: str,
    t3_size_scale: float,
    reentry_fill_policy: str,
    early_horizon_seconds: int,
    cost_threshold_atr: float,
    random_state: int,
    baseline_lead_adverse10_pct: float,
) -> dict[str, Any]:
    if overlay_trades is not None and overlay_trades.exists():
        base_trades = pd.read_csv(overlay_trades)
    else:
        base_trades = collect_overlay_trades(
            scored_events=scored_events,
            months=months,
            symbol=symbol,
            timeframe=timeframe,
            initial_balance=initial_balance,
            external_entry_mode=external_entry_mode,
            t3_size_scale=t3_size_scale,
            reentry_fill_policy=reentry_fill_policy,
            early_horizon_seconds=early_horizon_seconds,
        )
        overlay_trades = None
    if base_trades.empty:
        raise ValueError("no T3 overlay trades available")

    events = build_event_table(base_trades, initial_balance=initial_balance)
    variants = build_variants()
    all_event_scores = []
    all_weighted_trades = []
    metrics: list[SizingMetrics] = []
    fixed_overlay_pct = 0.0
    for variant in variants:
        scored = score_events_for_variant(
            events,
            variant,
            months=months,
            cost_threshold_atr=cost_threshold_atr,
            random_state=random_state,
        )
        weighted = apply_event_scores_to_trades(base_trades, scored, initial_balance=initial_balance)
        scored["sizing_variant"] = variant.label
        weighted["sizing_variant"] = variant.label
        if variant.label == "fixed_overlay_2p0":
            fixed_overlay_pct = round(float(weighted.groupby("month")["weighted_pnl_pct"].sum().sum()), 6)
        row = summarize_variant(
            variant=variant,
            trades=weighted,
            event_scores=scored,
            months=months,
            fixed_overlay_pct=fixed_overlay_pct,
            baseline_lead_adverse10_pct=baseline_lead_adverse10_pct,
        )
        metrics.append(row)
        all_event_scores.append(scored)
        all_weighted_trades.append(weighted)

    event_scores = pd.concat(all_event_scores, ignore_index=True)
    weighted_trades = pd.concat(all_weighted_trades, ignore_index=True)
    metrics = sorted(metrics, key=lambda row: row.calendar_sum_pct, reverse=True)
    write_outputs(
        output_dir=output_dir,
        base_trades=base_trades,
        event_scores=event_scores,
        weighted_trades=weighted_trades,
        metrics=metrics,
        months=months,
        scored_events=scored_events,
        overlay_trades=overlay_trades,
        cost_threshold_atr=cost_threshold_atr,
        baseline_lead_adverse10_pct=baseline_lead_adverse10_pct,
    )
    return {
        "metrics": [asdict(row) for row in metrics],
        "output_dir": str(output_dir),
    }


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output-dir", type=Path, default=DEFAULT_OUTPUT)
    parser.add_argument("--scored-events", type=Path, default=DEFAULT_SCORED_EVENTS)
    parser.add_argument("--overlay-trades", type=Path, default=None)
    parser.add_argument("--months", nargs="+", default=EXTENDED_MONTHS)
    parser.add_argument("--symbol", default="ETHUSDT")
    parser.add_argument("--timeframe", default="1h")
    parser.add_argument("--initial-balance", type=float, default=INITIAL_BALANCE)
    parser.add_argument(
        "--external-entry-mode",
        choices=["next_second_open", "next_second_close", "next_second_adverse"],
        default="next_second_adverse",
    )
    parser.add_argument("--t3-size-scale", type=float, default=2.0)
    parser.add_argument(
        "--reentry-fill-policy",
        choices=["historical", "strict_next_second_cross"],
        default="strict_next_second_cross",
    )
    parser.add_argument("--early-horizon-seconds", type=int, default=300)
    parser.add_argument("--cost-threshold-atr", type=float, default=0.10)
    parser.add_argument("--random-state", type=int, default=42)
    parser.add_argument(
        "--baseline-lead-adverse10-pct",
        type=float,
        default=BASELINE_LEAD_ADVERSE10_PCT,
        help="Lead baseline pct to add to T3 overlay metrics; use 61.070916 for the current q020-q040 research lead.",
    )
    parser.add_argument("--log-level", default="INFO")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    logging.basicConfig(
        level=getattr(logging, str(args.log_level).upper(), logging.INFO),
        format="%(asctime)s %(levelname)s %(message)s",
    )
    payload = run(
        output_dir=Path(args.output_dir),
        scored_events=Path(args.scored_events),
        overlay_trades=Path(args.overlay_trades) if args.overlay_trades is not None else None,
        months=[str(month) for month in args.months],
        symbol=str(args.symbol),
        timeframe=str(args.timeframe),
        initial_balance=float(args.initial_balance),
        external_entry_mode=str(args.external_entry_mode),
        t3_size_scale=float(args.t3_size_scale),
        reentry_fill_policy=str(args.reentry_fill_policy),
        early_horizon_seconds=int(args.early_horizon_seconds),
        cost_threshold_atr=float(args.cost_threshold_atr),
        random_state=int(args.random_state),
        baseline_lead_adverse10_pct=float(args.baseline_lead_adverse10_pct),
    )
    best = payload["metrics"][0]
    print(
        "best_variant="
        f"{best['variant']} overlay={float(best['calendar_sum_pct']):.6f}% "
        f"lead_plus_overlay={float(best['lead_adverse10_plus_overlay_pct']):.6f}% "
        f"delta_vs_fixed={float(best['overlay_delta_vs_fixed_pct']):.6f}pp"
    )
    print(f"wrote={payload['output_dir']}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
