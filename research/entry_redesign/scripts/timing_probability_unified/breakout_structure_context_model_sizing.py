"""Walk-forward context-aware sizing model for breakout-structure events.

Research-only. This tests whether 4h/12h context and structure features can be
used as a trailing, probability-style sizing overlay instead of fixed gates.
Each forward month trains only on the preceding train window, then sizes the
next month of `low_eff_low_atr` events.
"""

from __future__ import annotations

import argparse
import json
import logging
import sys
import time
from copy import deepcopy
from pathlib import Path
from typing import Any

import numpy as np
import pandas as pd
from sklearn.ensemble import RandomForestClassifier
from sklearn.metrics import roc_auc_score

SCRIPTS_DIR = Path(__file__).resolve().parents[1]
if str(SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPTS_DIR))

from dynamic_timing.execution_sim import DEFAULT_EXEC_PARAMS  # noqa: E402
from timing_probability_unified.breakout_shape_expansion import (  # noqa: E402
    BARS_CACHE_DIR,
    OUTPUT_DIR,
    _bars_cache_for_symbol,
)
from timing_probability_unified.breakout_structure_cross_asset_gate_search import (  # noqa: E402
    _add_context_features,
)
from timing_probability_unified.breakout_structure_cross_asset_validation import (  # noqa: E402
    _load_symbol_1s_bars,
)
from timing_probability_unified.breakout_structure_lead_expansion_combo import (  # noqa: E402
    _apply_conditions,
    _evaluate_events_with_adverse_trades,
    _markdown_table,
    _monthly_metrics,
    _traded_gate_on,
)

logger = logging.getLogger(__name__)

DEFAULT_BASE_GATE = "low_eff_low_atr_q20_q40"
LOW_EFF_CONTEXT_GATES = {
    "ctx4h_scaled025": "low_eff_low_atr_ctx4h_up",
    "ctx12h_scaled025": "low_eff_low_atr_ctx12h_up",
}
BROAD_CONTEXT_GATES = {
    "ctx4h_scaled025": "ctx4h_side_up_q60",
    "ctx12h_scaled025": "ctx12h_side_up_q60",
}
ADVERSE_SCENARIO = "next_adverse_xslip10bps"
FEATURE_COLUMNS = [
    "rf_probability",
    "sizing_multiplier",
    "signal_atr_percentile",
    "roundtrip_cost_atr",
    "prev1_range_atr",
    "prev1_close_pos_side",
    "level_to_signal_open_atr",
    "touch_extension_atr",
    "speed_300s_atr",
    "eff_300s",
    "pre_touch_seconds",
    "ctx4h_side_return_atr",
    "ctx12h_side_return_atr",
    "ctx4h_range_atr",
    "ctx12h_range_atr",
]
MODEL_VARIANTS = [
    "rf_prob_cap1",
    "rf_prob_floor025",
    "rf_binary_025",
    "rf_binary_000",
    "rf_rank_median_025",
    "rf_rank_median_000",
    "rf_rank_q60_000",
    "rf_rank_q70_000",
    "rf_replace_p2",
]


def _load_csv(path: Path) -> pd.DataFrame:
    if not path.exists():
        raise FileNotFoundError(path)
    df = pd.read_csv(path)
    for column in ("touch_time", "signal_start", "signal_end"):
        if column in df.columns:
            df[column] = pd.to_datetime(df[column], utc=True)
    return df


def _event_keys(events: pd.DataFrame) -> pd.Series:
    if "event_key" in events.columns:
        return events["event_key"].astype(str)
    return events.apply(
        lambda row: f"{pd.Timestamp(row['signal_start']).isoformat()}|{row['side']}",
        axis=1,
    )


def _gate_row(rows: pd.DataFrame, forward_month: str, gate: str) -> pd.Series | None:
    matched = rows[(rows["forward_month"].astype(str) == forward_month) & (rows["gate"] == gate)]
    if matched.empty:
        return None
    return matched.iloc[0]


def _apply_gate_row(events: pd.DataFrame, row: pd.Series | None) -> pd.DataFrame:
    if row is None:
        return pd.DataFrame(columns=events.columns)
    return _apply_conditions(events, str(row["conditions"]))


def _features(events: pd.DataFrame, medians: pd.Series | None = None) -> tuple[pd.DataFrame, pd.Series]:
    out = pd.DataFrame(index=events.index)
    for column in FEATURE_COLUMNS:
        if column in events.columns:
            out[column] = pd.to_numeric(events[column], errors="coerce")
        else:
            out[column] = np.nan
    out["side_is_long"] = (events["side"].astype(str) == "long").astype(float).to_numpy()
    if medians is None:
        medians = out.median(numeric_only=True).fillna(0.0)
    out = out.fillna(medians).fillna(0.0)
    return out, medians


def _event_labels(events: pd.DataFrame, bars_cache: dict[str, pd.DataFrame]) -> tuple[pd.Series, dict[str, Any]]:
    if events.empty:
        return pd.Series(dtype=int), {"label_events": 0, "positive_labels": 0}
    _, _, adverse_trades = _evaluate_events_with_adverse_trades(events, bars_cache, ADVERSE_SCENARIO)
    adverse_trades = _traded_gate_on(adverse_trades)
    label_by_key = {
        str(row.event_key): int(float(row.delay_pnl_pct) > 0.0)
        for row in adverse_trades.itertuples(index=False)
        if hasattr(row, "event_key")
    }
    labels = _event_keys(events).map(label_by_key).fillna(0).astype(int)
    return labels, {
        "label_events": int(len(labels)),
        "positive_labels": int(labels.sum()),
        "negative_labels": int((labels == 0).sum()),
    }


def _fit_predict_context_model(
    *,
    train_events: pd.DataFrame,
    forward_events: pd.DataFrame,
    bars_cache: dict[str, pd.DataFrame],
    min_train_events: int,
) -> tuple[np.ndarray, dict[str, Any]]:
    diagnostics: dict[str, Any] = {"train_events": int(len(train_events)), "model_status": "trained"}
    if len(forward_events) == 0:
        return np.array([], dtype="float64"), {**diagnostics, "model_status": "empty_forward"}
    if len(train_events) < min_train_events:
        return np.full(len(forward_events), 0.5), {**diagnostics, "model_status": "too_few_train"}

    labels, label_diagnostics = _event_labels(train_events, bars_cache)
    diagnostics.update(label_diagnostics)
    if labels.nunique() < 2:
        return np.full(len(forward_events), 0.5), {**diagnostics, "model_status": "single_class"}

    train_x, medians = _features(train_events)
    forward_x, _ = _features(forward_events, medians)
    model = RandomForestClassifier(
        n_estimators=200,
        max_depth=3,
        min_samples_leaf=2,
        random_state=42,
        class_weight="balanced_subsample",
    )
    model.fit(train_x, labels)
    classes = list(model.classes_)
    if 1 not in classes:
        return np.full(len(forward_events), 0.5), {**diagnostics, "model_status": "missing_positive_class"}
    train_probs = model.predict_proba(train_x)[:, classes.index(1)]
    forward_probs = model.predict_proba(forward_x)[:, classes.index(1)]
    diagnostics.update(
        {
            "train_auc": float(roc_auc_score(labels, train_probs)),
            "train_prob_median": float(np.median(train_probs)),
            "train_prob_q40": float(np.quantile(train_probs, 0.40)),
            "train_prob_q60": float(np.quantile(train_probs, 0.60)),
            "train_prob_q70": float(np.quantile(train_probs, 0.70)),
            "forward_prob_mean": float(np.mean(forward_probs)),
            "forward_prob_median": float(np.median(forward_probs)),
            "feature_importance_top5": [
                [name, float(value)]
                for name, value in sorted(
                    zip(train_x.columns, model.feature_importances_),
                    key=lambda item: item[1],
                    reverse=True,
                )[:5]
            ],
        }
    )
    return forward_probs, diagnostics


def _scaled_events(
    events: pd.DataFrame,
    *,
    variant: str,
    probabilities: np.ndarray | None = None,
    train_prob_median: float | None = None,
    pass_keys: set[str] | None = None,
) -> pd.DataFrame:
    out = events.copy().reset_index(drop=True)
    original = pd.to_numeric(out["sizing_multiplier"], errors="coerce").fillna(0.0).to_numpy(dtype="float64")

    if variant == "baseline_original":
        scale = np.ones(len(out), dtype="float64")
        new_multiplier = original
    elif variant in set(LOW_EFF_CONTEXT_GATES) | set(BROAD_CONTEXT_GATES):
        if pass_keys is None:
            raise ValueError(f"{variant} needs pass_keys")
        mask = _event_keys(out).isin(pass_keys).to_numpy(dtype=bool)
        scale = np.where(mask, 1.0, 0.25)
        new_multiplier = original * scale
    elif variant == "rf_prob_cap1":
        if probabilities is None:
            raise ValueError(f"{variant} needs probabilities")
        scale = np.clip(probabilities * 2.0, 0.0, 1.0)
        new_multiplier = original * scale
    elif variant == "rf_prob_floor025":
        if probabilities is None:
            raise ValueError(f"{variant} needs probabilities")
        scale = 0.25 + 0.75 * np.clip(probabilities * 2.0, 0.0, 1.0)
        new_multiplier = original * scale
    elif variant == "rf_binary_025":
        if probabilities is None:
            raise ValueError(f"{variant} needs probabilities")
        scale = np.where(probabilities >= 0.5, 1.0, 0.25)
        new_multiplier = original * scale
    elif variant == "rf_binary_000":
        if probabilities is None:
            raise ValueError(f"{variant} needs probabilities")
        scale = np.where(probabilities >= 0.5, 1.0, 0.0)
        new_multiplier = original * scale
    elif variant == "rf_rank_median_025":
        if probabilities is None or train_prob_median is None:
            raise ValueError(f"{variant} needs probabilities and train median")
        scale = np.where(probabilities >= train_prob_median, 1.0, 0.25)
        new_multiplier = original * scale
    elif variant == "rf_rank_median_000":
        if probabilities is None or train_prob_median is None:
            raise ValueError(f"{variant} needs probabilities and train median")
        scale = np.where(probabilities >= train_prob_median, 1.0, 0.0)
        new_multiplier = original * scale
    elif variant == "rf_rank_q60_000":
        if probabilities is None or train_prob_median is None:
            raise ValueError(f"{variant} needs probabilities and train threshold")
        scale = np.where(probabilities >= train_prob_median, 1.0, 0.0)
        new_multiplier = original * scale
    elif variant == "rf_rank_q70_000":
        if probabilities is None or train_prob_median is None:
            raise ValueError(f"{variant} needs probabilities and train threshold")
        scale = np.where(probabilities >= train_prob_median, 1.0, 0.0)
        new_multiplier = original * scale
    elif variant == "rf_replace_p2":
        if probabilities is None:
            raise ValueError(f"{variant} needs probabilities")
        scale = np.clip(probabilities * 2.0, 0.0, 2.0)
        new_multiplier = scale
    else:
        raise ValueError(f"unknown variant: {variant}")

    out["sizing_multiplier"] = new_multiplier
    out["context_model_scale"] = scale
    out["context_model_variant"] = variant
    if probabilities is not None:
        out["context_model_probability"] = probabilities
    return out


def _evaluate_variant(
    *,
    events: pd.DataFrame,
    variant: str,
    bars_cache: dict[str, pd.DataFrame],
    forward_month: str,
    model_status: str,
) -> dict[str, Any]:
    _, same_trades, adverse_trades = _evaluate_events_with_adverse_trades(events, bars_cache, ADVERSE_SCENARIO)
    same_trades = _traded_gate_on(same_trades)
    adverse_trades = _traded_gate_on(adverse_trades)
    same = _monthly_metrics(same_trades)
    adverse10 = _monthly_metrics(adverse_trades)
    active_adverse = adverse_trades[pd.to_numeric(adverse_trades["position_size"], errors="coerce").fillna(0.0) > 0.0]
    return {
        "forward_month": forward_month,
        "variant": variant,
        "model_status": model_status,
        "events": int(len(events)),
        "avg_scale": float(pd.to_numeric(events.get("context_model_scale", pd.Series([1.0] * len(events))), errors="coerce").mean())
        if len(events)
        else 0.0,
        "same_close_calendar_sum": same["calendar_sum"],
        "same_close_worst_sm": same["worst_sm"],
        "same_close_neg_sm": same["neg_sm"],
        "same_close_trade_count": same["trade_count"],
        "adverse10_calendar_sum": adverse10["calendar_sum"],
        "adverse10_worst_sm": adverse10["worst_sm"],
        "adverse10_neg_sm": adverse10["neg_sm"],
        "adverse10_trade_count": adverse10["trade_count"],
        "adverse10_active_trade_count": int(len(active_adverse)),
    }


def _aggregate(rows: list[dict[str, Any]]) -> pd.DataFrame:
    df = pd.DataFrame(rows)
    out_rows: list[dict[str, Any]] = []
    for variant, group in df.groupby("variant"):
        out_rows.append(
            {
                "variant": variant,
                "forward_months": int(group["forward_month"].nunique()),
                "events": int(group["events"].sum()),
                "avg_scale": float(group["avg_scale"].mean()),
                "same_close_calendar_sum": float(group["same_close_calendar_sum"].sum()),
                "same_close_worst_month": float(group["same_close_calendar_sum"].min()),
                "same_close_neg_months": int((group["same_close_calendar_sum"] < 0).sum()),
                "adverse10_calendar_sum": float(group["adverse10_calendar_sum"].sum()),
                "adverse10_worst_month": float(group["adverse10_calendar_sum"].min()),
                "adverse10_neg_months": int((group["adverse10_calendar_sum"] < 0).sum()),
                "adverse10_trade_count": int(group["adverse10_trade_count"].sum()),
                "adverse10_active_trade_count": int(group["adverse10_active_trade_count"].sum()),
                "trained_months": int((group["model_status"] == "trained").sum()),
            }
        )
    return pd.DataFrame(out_rows).sort_values(
        ["adverse10_calendar_sum", "adverse10_worst_month"],
        ascending=[False, False],
    )


def _write_report(
    *,
    summary: pd.DataFrame,
    monthly: pd.DataFrame,
    diagnostics: dict[str, Any],
    output_path: Path,
) -> None:
    summary_cols = [
        "variant",
        "forward_months",
        "events",
        "avg_scale",
        "same_close_calendar_sum",
        "same_close_worst_month",
        "same_close_neg_months",
        "adverse10_calendar_sum",
        "adverse10_worst_month",
        "adverse10_neg_months",
        "adverse10_trade_count",
        "adverse10_active_trade_count",
        "trained_months",
    ]
    monthly_cols = [
        "forward_month",
        "variant",
        "model_status",
        "events",
        "avg_scale",
        "same_close_calendar_sum",
        "adverse10_calendar_sum",
        "adverse10_trade_count",
        "adverse10_active_trade_count",
    ]
    lines = [
        "# Breakout Structure Context Model Sizing",
        "",
        f"Generated: {pd.Timestamp.utcnow().isoformat()}",
        "",
        "Scope: research-only. This trains a trailing context-aware sizing overlay for the selected `--base-gate` event pool.",
        "",
        "## Aggregate",
        "",
        _markdown_table(summary, summary_cols),
        "",
        "## Monthly Rows",
        "",
        _markdown_table(monthly, monthly_cols),
        "",
        "## Interpretation",
        "",
        "- `baseline_original` keeps the event source's original RF sizing.",
        "- `ctx*_scaled025` are fixed 4h/12h context overlays for comparison.",
        "- `rf_*` variants train only on prior-window adverse10 labels, then size the next month.",
        "- `*_000` variants are hard-select checks: rejected events receive zero size and should be read with `adverse10_active_trade_count`.",
        "- Promotion requires improvement on ETH late without breaking ETH early or BTC late.",
        "",
        "## Diagnostics",
        "",
        "```json",
        json.dumps(diagnostics, indent=2, ensure_ascii=False, default=str),
        "```",
    ]
    output_path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def _context_gates_for(base_gate: str) -> dict[str, str]:
    if base_gate == "baseline_model_advance":
        return BROAD_CONTEXT_GATES
    return LOW_EFF_CONTEXT_GATES


def run(
    *,
    symbol: str,
    events_csv: Path,
    candidate_rows_csv: Path,
    bars_cache_dir: Path,
    eval_start: pd.Timestamp,
    eval_end: pd.Timestamp,
    train_months: int,
    min_train_events: int,
    base_gate: str,
    output_tag: str,
) -> None:
    started = time.time()
    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)

    events = _load_csv(events_csv)
    events = events[(events["touch_time"] >= eval_start) & (events["touch_time"] < eval_end)].copy()
    bars_1s = _load_symbol_1s_bars(symbol, bars_cache_dir, eval_start, eval_end)
    events = _add_context_features(events.reset_index(drop=True), bars_1s)
    bars_cache = _bars_cache_for_symbol(symbol, bars_1s)
    candidate_rows = _load_csv(candidate_rows_csv)

    context_gates = _context_gates_for(base_gate)
    months = sorted(candidate_rows[candidate_rows["gate"] == base_gate]["forward_month"].astype(str).unique())
    saved_params = deepcopy(DEFAULT_EXEC_PARAMS)
    replay_params = {
        "initial_stop_atr": 0.45,
        "breakeven_at_r": 0.8,
        "trail_start_r": 1.5,
        "trail_buffer_atr": 0.05,
        "max_hold_hours": 2.0,
    }
    DEFAULT_EXEC_PARAMS.update(replay_params)

    rows: list[dict[str, Any]] = []
    split_diagnostics: list[dict[str, Any]] = []
    try:
        for forward_month in months:
            forward_start = pd.Timestamp(f"{forward_month}-01", tz="UTC")
            if forward_start < eval_start or forward_start >= eval_end:
                continue
            forward_end = forward_start + pd.DateOffset(months=1)
            train_start = forward_start - pd.DateOffset(months=train_months)
            base_row = _gate_row(candidate_rows, forward_month, base_gate)
            if base_row is None:
                continue
            train_all = events[(events["touch_time"] >= train_start) & (events["touch_time"] < forward_start)].copy()
            forward_all = events[(events["touch_time"] >= forward_start) & (events["touch_time"] < forward_end)].copy()
            train_events = _apply_gate_row(train_all, base_row)
            forward_events = _apply_gate_row(forward_all, base_row)
            if forward_events.empty:
                continue

            probabilities, model_diag = _fit_predict_context_model(
                train_events=train_events,
                forward_events=forward_events,
                bars_cache=bars_cache,
                min_train_events=min_train_events,
            )
            split_diagnostics.append(
                {
                    "forward_month": forward_month,
                    "train_start": train_start.isoformat(),
                    "train_events_after_base_gate": int(len(train_events)),
                    "forward_events_after_base_gate": int(len(forward_events)),
                    **model_diag,
                }
            )

            variants: dict[str, pd.DataFrame] = {
                "baseline_original": _scaled_events(forward_events, variant="baseline_original"),
            }
            for variant, gate in context_gates.items():
                context_row = _gate_row(candidate_rows, forward_month, gate)
                context_events = _apply_gate_row(forward_all, context_row)
                pass_keys = set(_event_keys(context_events))
                variants[variant] = _scaled_events(forward_events, variant=variant, pass_keys=pass_keys)

            thresholds = {
                "rf_rank_median_025": model_diag.get("train_prob_median", 0.5),
                "rf_rank_median_000": model_diag.get("train_prob_median", 0.5),
                "rf_rank_q60_000": model_diag.get("train_prob_q60", 0.5),
                "rf_rank_q70_000": model_diag.get("train_prob_q70", 0.5),
            }
            for variant in MODEL_VARIANTS:
                variants[variant] = _scaled_events(
                    forward_events,
                    variant=variant,
                    probabilities=probabilities,
                    train_prob_median=float(thresholds.get(variant, model_diag.get("train_prob_median", 0.5))),
                )

            for variant, variant_events in variants.items():
                rows.append(
                    _evaluate_variant(
                        events=variant_events,
                        variant=variant,
                        bars_cache=bars_cache,
                        forward_month=forward_month,
                        model_status=str(model_diag.get("model_status", "")),
                    )
                )
    finally:
        DEFAULT_EXEC_PARAMS.clear()
        DEFAULT_EXEC_PARAMS.update(saved_params)

    monthly = pd.DataFrame(rows)
    summary = _aggregate(rows)
    summary_path = OUTPUT_DIR / f"breakout_structure_context_model_sizing_{output_tag}_summary.csv"
    monthly_path = OUTPUT_DIR / f"breakout_structure_context_model_sizing_{output_tag}_monthly.csv"
    diagnostics_path = OUTPUT_DIR / f"breakout_structure_context_model_sizing_{output_tag}_diagnostics.json"
    report_path = OUTPUT_DIR / f"breakout_structure_context_model_sizing_{output_tag}_report.md"
    diagnostics = {
        "symbol": symbol,
        "events_csv": str(events_csv),
        "candidate_rows_csv": str(candidate_rows_csv),
        "bars_cache_dir": str(bars_cache_dir),
        "eval_start": eval_start.isoformat(),
        "eval_end_exclusive": eval_end.isoformat(),
        "train_months": train_months,
        "min_train_events": min_train_events,
        "base_gate": base_gate,
        "context_gates": context_gates,
        "model_variants": MODEL_VARIANTS,
        "feature_columns": FEATURE_COLUMNS + ["side_is_long"],
        "exec_params": {**saved_params, **replay_params},
        "splits": split_diagnostics,
        "runtime_seconds": time.time() - started,
    }
    summary.to_csv(summary_path, index=False)
    monthly.to_csv(monthly_path, index=False)
    diagnostics_path.write_text(json.dumps(diagnostics, indent=2, ensure_ascii=False, default=str) + "\n", encoding="utf-8")
    _write_report(summary=summary, monthly=monthly, diagnostics=diagnostics, output_path=report_path)
    logger.info("written %s", summary_path)
    logger.info("written %s", report_path)


def _utc_timestamp(text: str) -> pd.Timestamp:
    ts = pd.Timestamp(text)
    return ts.tz_localize("UTC") if ts.tzinfo is None else ts.tz_convert("UTC")


def main() -> None:
    parser = argparse.ArgumentParser(description="Walk-forward context model sizing")
    parser.add_argument("--symbol", default="ETHUSDT", choices=["BTCUSDT", "ETHUSDT"])
    parser.add_argument("--events-csv", type=Path, required=True)
    parser.add_argument("--candidate-rows-csv", type=Path, required=True)
    parser.add_argument("--bars-cache-dir", type=Path, default=BARS_CACHE_DIR)
    parser.add_argument("--eval-start", required=True)
    parser.add_argument("--eval-end", required=True)
    parser.add_argument("--train-months", type=int, required=True)
    parser.add_argument("--min-train-events", type=int, default=8)
    parser.add_argument("--base-gate", default=DEFAULT_BASE_GATE)
    parser.add_argument("--output-tag", required=True)
    parser.add_argument("--log-level", default="INFO")
    args = parser.parse_args()
    logging.basicConfig(
        level=getattr(logging, str(args.log_level).upper(), logging.INFO),
        format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
    )
    run(
        symbol=args.symbol,
        events_csv=args.events_csv,
        candidate_rows_csv=args.candidate_rows_csv,
        bars_cache_dir=args.bars_cache_dir,
        eval_start=_utc_timestamp(args.eval_start),
        eval_end=_utc_timestamp(args.eval_end),
        train_months=args.train_months,
        min_train_events=args.min_train_events,
        base_gate=args.base_gate,
        output_tag=args.output_tag,
    )


if __name__ == "__main__":
    main()
