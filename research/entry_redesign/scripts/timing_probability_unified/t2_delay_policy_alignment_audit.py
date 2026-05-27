"""Research-only audit for deployable T2 lead delay policies.

The current T2 lead headline uses a selected-delay ledger: for each model
prediction, research evaluates candidate delay results and keeps the best
delay inside the predicted bucket (fast: D0/D5, slow: D10/D15/pullback).
That is an evaluation contract, not directly deployable in live because live
does not know future PnL at the touch tick.

This script replays the same canonical lead events and adverse10 fill scenario,
then scores live-deployable fixed delay policies under the same lead
0.20..0.40 ETH quantity-band sizing.
"""

from __future__ import annotations

import argparse
import json
import logging
import math
import sys
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Any, Callable

import numpy as np
import pandas as pd

SCRIPTS_DIR = Path(__file__).resolve().parents[1]
PROJECT_ROOT = Path(__file__).resolve().parents[4]
if str(SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPTS_DIR))

from dynamic_timing.execution_sim import DEFAULT_EXEC_PARAMS  # noqa: E402
from timing_probability_unified.breakout_shape_expansion import (  # noqa: E402
    BASE_SHARE,
    OUTPUT_DIR,
    _bars_cache_for_symbol,
    _load_all_1s_bars,
    _reprice_delay_results_fast,
)
from timing_probability_unified.breakout_structure_lead_expansion_combo import (  # noqa: E402
    LEAD_REPLAY_EXEC_OVERRIDES,
    _fill_scenario,
    _lead_events_for_trades,
    _lead_trades_all,
)
from timing_probability_unified.t2_lead_quantity_band_sizing import (  # noqa: E402
    score_lead_quantity_band,
)
from timing_probability_unified.unified_runner import _simulate_delays_for_events  # noqa: E402

logger = logging.getLogger(__name__)

DEFAULT_OUTPUT = OUTPUT_DIR / "t2_delay_policy_alignment_audit_20260527"
DEFAULT_SCENARIO = "next_adverse_xslip10bps"
DEFAULT_MODEL = PROJECT_ROOT / "data" / "pretouch_model.json"
CURRENT_QBAND_LEAD_PCT = 61.07091667649647
BASE_LEAD_ADVERSE10_PCT = 22.97164769930056
DELAY_LABELS = ("D0", "D5", "D10", "D15", "pullback")


@dataclass(frozen=True)
class PolicySpec:
    policy_id: str
    description: str
    selector: Callable[[str], str]


@dataclass(frozen=True)
class PolicyMetrics:
    policy_id: str
    calendar_sum_pct: float
    delta_vs_current_qband_lead_pct: float
    delta_vs_base_lead_adverse10_pct: float
    worst_month_pct: float
    negative_months: int
    trade_count: int
    untraded_count: int
    avg_submitted_quantity: float
    max_drawdown_pct: float
    selected_delay_counts: dict[str, int]
    exit_reason_counts: dict[str, int]
    by_month: dict[str, float]


def _tree_feature_index(node: dict[str, Any]) -> int:
    # Go's int zero value plus `omitempty` means internal tree nodes using
    # feature 0 often omit `f` from JSON.
    return int(node.get("f", 0))


def _predict_tree(node: dict[str, Any] | None, features: list[float]) -> str:
    if not node:
        return ""
    if not node.get("l") and not node.get("r"):
        return str(node.get("v", ""))
    feature_index = _tree_feature_index(node)
    if feature_index < 0 or feature_index >= len(features):
        return str(node.get("v", ""))
    child = node.get("l") if features[feature_index] <= float(node.get("t", 0.0)) else node.get("r")
    return _predict_tree(child, features) if child else str(node.get("v", ""))


def _predict_tree_proba(node: dict[str, Any] | None, features: list[float]) -> float:
    if not node:
        return 0.5
    if not node.get("l") and not node.get("r"):
        return float(node.get("p", 0.0))
    feature_index = _tree_feature_index(node)
    if feature_index < 0 or feature_index >= len(features):
        return 0.5
    child = node.get("l") if features[feature_index] <= float(node.get("t", 0.0)) else node.get("r")
    return _predict_tree_proba(child, features) if child else 0.5


def _predict_rf_proba(rf_model: dict[str, Any] | None, features: list[float]) -> float:
    trees = (rf_model or {}).get("trees") or []
    if not trees:
        return 0.5
    return float(sum(_predict_tree_proba(tree, features) for tree in trees) / len(trees))


def _model_feature_vector(row: pd.Series, feature_names: list[str], medians: list[float]) -> list[float]:
    values: list[float] = []
    for idx, name in enumerate(feature_names):
        value = row.get(name, np.nan)
        try:
            numeric = float(value)
        except (TypeError, ValueError):
            numeric = float("nan")
        if math.isnan(numeric):
            numeric = float(medians[idx]) if idx < len(medians) else 0.0
        values.append(numeric)
    return values


def _attach_artifact_predictions(lead_all: pd.DataFrame, events: pd.DataFrame, model_path: Path) -> pd.DataFrame:
    payload = json.loads(model_path.read_text(encoding="utf-8"))
    feature_names = [str(name) for name in payload.get("feature_names", [])]
    medians = [float(value) for value in payload.get("medians", [])]
    if not feature_names:
        raise ValueError(f"model has no feature_names: {model_path}")

    event_features = events.reset_index(drop=True)
    out = lead_all.reset_index(drop=True).copy()
    timing: list[str] = []
    rf_probs: list[float] = []
    for idx, row in event_features.iterrows():
        features = _model_feature_vector(row, feature_names, medians)
        timing.append(_predict_tree(payload.get("timing_tree"), features))
        rf_probs.append(_predict_rf_proba(payload.get("rf_model"), features))
    out["artifact_timing_prediction"] = timing
    out["artifact_rf_probability"] = rf_probs
    return out


def _equity_max_drawdown_pct(values_pct: pd.Series) -> float:
    if values_pct.empty:
        return 0.0
    curve = pd.concat([pd.Series([0.0]), values_pct.astype(float).cumsum().reset_index(drop=True)], ignore_index=True)
    drawdown = curve - curve.cummax()
    return round(float(drawdown.min()), 6)


def _load_repriced_delay_results(*, symbol: str, scenario_name: str):
    lead_all = _lead_trades_all()
    lead_all = lead_all[lead_all["symbol"].astype(str) == symbol].reset_index(drop=True)
    events = _lead_events_for_trades(lead_all)
    bars_1s = _load_all_1s_bars(symbol)
    bars_cache = _bars_cache_for_symbol(symbol, bars_1s)
    scenario = _fill_scenario(scenario_name)

    saved_params = dict(DEFAULT_EXEC_PARAMS)
    lead_params = dict(DEFAULT_EXEC_PARAMS)
    lead_params.update(LEAD_REPLAY_EXEC_OVERRIDES)
    try:
        DEFAULT_EXEC_PARAMS.clear()
        DEFAULT_EXEC_PARAMS.update(lead_params)
        delay_results, errors = _simulate_delays_for_events(events, bars_cache)
    finally:
        DEFAULT_EXEC_PARAMS.clear()
        DEFAULT_EXEC_PARAMS.update(saved_params)

    repriced = _reprice_delay_results_fast(
        delay_results=delay_results,
        events=events,
        bars_cache=bars_cache,
        scenario=scenario,
    )
    diagnostics = {
        "symbol": symbol,
        "scenario": scenario_name,
        "lead_replayed_events": int(len(events)),
        "lead_delay_errors": int(len(errors)),
        "lead_exec_params": lead_params,
    }
    return lead_all, events, repriced, diagnostics


def _delay_result_map(event_delays: list[Any]) -> dict[str, Any]:
    return {str(result.delay_label): result for result in event_delays}


def _policy_rows(
    *,
    lead_all: pd.DataFrame,
    events: pd.DataFrame,
    delay_results: list[list[Any]],
    policy: PolicySpec,
) -> pd.DataFrame:
    rows: list[dict[str, Any]] = []
    for idx, event in events.reset_index(drop=True).iterrows():
        lead = lead_all.iloc[idx]
        timing_prediction = str(lead["timing_prediction"])
        selected_delay = policy.selector(timing_prediction)
        result = _delay_result_map(delay_results[idx]).get(selected_delay)
        speed_gate_pass = bool(lead["speed_gate_pass"])
        traded = bool(
            timing_prediction != "skip"
            and speed_gate_pass
            and result is not None
            and result.traded
            and result.pnl_pct is not None
        )
        position_size = float(lead["position_size"])
        pnl = float(result.pnl_pct) if traded else 0.0
        rows.append(
            {
                "event_id": str(lead["event_id"]),
                "symbol": str(lead["symbol"]),
                "side": str(lead["side"]),
                "touch_time": pd.Timestamp(lead["touch_time"]),
                "timing_prediction": timing_prediction,
                "selected_delay": selected_delay if traded else "none",
                "policy_selected_delay": selected_delay,
                "rf_probability": float(lead["rf_probability"]),
                "sizing_multiplier": float(lead["sizing_multiplier"]),
                "position_size": position_size,
                "delay_pnl_pct": pnl,
                "weighted_pnl": pnl * position_size,
                "speed_300s_atr": float(lead["speed_300s_atr"]),
                "speed_gate_pass": speed_gate_pass,
                "source_leg": "canonical_lead",
                "entry_time": result.entry_time if traded else pd.NaT,
                "entry_price": result.entry_price if traded else np.nan,
                "exit_time": result.exit_time if traded else pd.NaT,
                "exit_reason": str(result.exit_reason or "") if traded else "Untraded",
                "delay_seconds": int(result.delay_seconds) if result is not None else 0,
                "delay_traded": traded,
            }
        )
    frame = pd.DataFrame(rows)
    frame["touch_time"] = pd.to_datetime(frame["touch_time"], utc=True)
    frame["entry_time"] = pd.to_datetime(frame["entry_time"], utc=True)
    frame["exit_time"] = pd.to_datetime(frame["exit_time"], utc=True)
    return frame


def _score_policy(frame: pd.DataFrame, *, policy_id: str) -> tuple[PolicyMetrics, pd.DataFrame]:
    scored = score_lead_quantity_band(
        frame,
        base_order_quantity=0.100,
        base_share=BASE_SHARE,
        max_production_multiplier=2.0,
        min_quantity=0.20,
        max_quantity=0.40,
        max_submitted_quantity=0.40,
        legacy_scale=1.5,
    )
    monthly = scored.groupby("year_month")["quantity_band_weighted_pnl_pct"].sum()
    by_month = {str(month): round(float(value), 6) for month, value in monthly.items()}
    calendar_sum = round(float(monthly.sum()), 6)
    metrics = PolicyMetrics(
        policy_id=policy_id,
        calendar_sum_pct=calendar_sum,
        delta_vs_current_qband_lead_pct=round(calendar_sum - CURRENT_QBAND_LEAD_PCT, 6),
        delta_vs_base_lead_adverse10_pct=round(calendar_sum - BASE_LEAD_ADVERSE10_PCT, 6),
        worst_month_pct=round(float(monthly.min()), 6) if len(monthly) else 0.0,
        negative_months=int((monthly < 0.0).sum()),
        trade_count=int(scored["delay_traded"].sum()),
        untraded_count=int((~scored["delay_traded"].astype(bool)).sum()),
        avg_submitted_quantity=round(float(scored["submitted_quantity"].mean()), 6),
        max_drawdown_pct=_equity_max_drawdown_pct(scored.sort_values("touch_time")["quantity_band_weighted_pnl_pct"]),
        selected_delay_counts={str(k): int(v) for k, v in scored["selected_delay"].value_counts().sort_index().items()},
        exit_reason_counts={str(k): int(v) for k, v in scored["exit_reason"].value_counts().sort_index().items()},
        by_month=by_month,
    )
    return metrics, scored


def _policy_specs() -> list[PolicySpec]:
    fixed = [
        PolicySpec(
            policy_id=f"fixed_{label.lower()}_all_non_skip",
            description=f"Every non-skip timing event enters with fixed {label}.",
            selector=lambda _prediction, label=label: label,
        )
        for label in DELAY_LABELS
    ]
    paired = [
        PolicySpec("fast_d0_slow_d10", "fast -> D0, slow -> D10", lambda p: "D10" if p == "slow" else "D0"),
        PolicySpec("fast_d0_slow_d15", "fast -> D0, slow -> D15", lambda p: "D15" if p == "slow" else "D0"),
        PolicySpec(
            "fast_d0_slow_pullback",
            "fast -> D0, slow -> pullback",
            lambda p: "pullback" if p == "slow" else "D0",
        ),
        PolicySpec("fast_d5_slow_d10", "fast -> D5, slow -> D10", lambda p: "D10" if p == "slow" else "D5"),
        PolicySpec("fast_d5_slow_d15", "fast -> D5, slow -> D15", lambda p: "D15" if p == "slow" else "D5"),
        PolicySpec(
            "fast_d5_slow_pullback",
            "fast -> D5, slow -> pullback",
            lambda p: "pullback" if p == "slow" else "D5",
        ),
    ]
    return fixed + paired


def _markdown_table(rows: list[dict[str, Any]], columns: list[str]) -> str:
    if not rows:
        return "_empty_"
    values = [[str(row.get(column, "")) for column in columns] for row in rows]
    widths = [max(len(column), *(len(row[idx]) for row in values)) for idx, column in enumerate(columns)]
    header = "| " + " | ".join(column.ljust(widths[idx]) for idx, column in enumerate(columns)) + " |"
    sep = "| " + " | ".join("-" * widths[idx] for idx in range(len(columns))) + " |"
    body = ["| " + " | ".join(row[idx].ljust(widths[idx]) for idx in range(len(columns))) + " |" for row in values]
    return "\n".join([header, sep, *body])


def _write_report(output_dir: Path, metrics: list[PolicyMetrics], diagnostics: dict[str, Any]) -> None:
    ranked = sorted(metrics, key=lambda row: row.calendar_sum_pct, reverse=True)
    artifact_alignment = diagnostics.get("artifact_alignment") or {}
    rows = [
        {
            "policy": row.policy_id,
            "calendar_sum": f"{row.calendar_sum_pct:.6f}%",
            "delta_vs_qband": f"{row.delta_vs_current_qband_lead_pct:.6f}pp",
            "delta_vs_base": f"{row.delta_vs_base_lead_adverse10_pct:.6f}pp",
            "worst_month": f"{row.worst_month_pct:.6f}%",
            "neg_months": row.negative_months,
            "trades": row.trade_count,
            "untraded": row.untraded_count,
            "max_dd": f"{row.max_drawdown_pct:.6f}%",
        }
        for row in ranked
    ]
    lines = [
        "# T2 Delay Policy Alignment Audit",
        "",
        "Research-only audit for replacing ex-post selected-delay evaluation with deployable fixed delay policies.",
        "",
        f"- Scenario: `{diagnostics['scenario']}`",
        f"- Events replayed: `{diagnostics['lead_replayed_events']}`",
        f"- Current qband selected-delay lead reference: `{CURRENT_QBAND_LEAD_PCT:.6f}%`",
        f"- Base lead adverse10 reference: `{BASE_LEAD_ADVERSE10_PCT:.6f}%`",
        f"- Ledger timing counts: `{artifact_alignment.get('ledger_timing_counts', {})}`",
        f"- Current artifact timing counts: `{artifact_alignment.get('artifact_timing_counts', {})}`",
        f"- Current artifact slow count: `{artifact_alignment.get('artifact_slow_count', 0)}`",
        "",
        "## Ranked Policies",
        "",
        _markdown_table(
            rows,
            [
                "policy",
                "calendar_sum",
                "delta_vs_qband",
                "delta_vs_base",
                "worst_month",
                "neg_months",
                "trades",
                "untraded",
                "max_dd",
            ],
        ),
        "",
        "## Read",
        "",
        "- `fixed_d0_all_non_skip` is the closest proxy for current live immediate-entry timing.",
        "- The reference `lead_quantity_0p20_0p40_adverse10` uses ex-post selected delay inside each predicted timing bucket.",
        "- If fixed policies materially underperform the selected-delay reference, live alignment needs a deployable delay policy/model before treating the headline as live-equivalent.",
    ]
    (output_dir / "t2_delay_policy_alignment_report.md").write_text("\n".join(lines) + "\n", encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--symbol", default="ETHUSDT")
    parser.add_argument("--scenario", default=DEFAULT_SCENARIO)
    parser.add_argument("--output-dir", type=Path, default=DEFAULT_OUTPUT)
    parser.add_argument("--model-path", type=Path, default=DEFAULT_MODEL)
    parser.add_argument("--write-ledgers", action="store_true")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(name)s: %(message)s")
    args.output_dir.mkdir(parents=True, exist_ok=True)

    lead_all, events, delay_results, diagnostics = _load_repriced_delay_results(
        symbol=str(args.symbol),
        scenario_name=str(args.scenario),
    )
    metrics: list[PolicyMetrics] = []
    artifact_lead = _attach_artifact_predictions(lead_all, events, args.model_path)
    artifact_alignment = {
        "model_path": str(args.model_path),
        "ledger_timing_counts": {str(k): int(v) for k, v in artifact_lead["timing_prediction"].value_counts().sort_index().items()},
        "artifact_timing_counts": {
            str(k): int(v) for k, v in artifact_lead["artifact_timing_prediction"].value_counts().sort_index().items()
        },
        "timing_match_count": int((artifact_lead["timing_prediction"] == artifact_lead["artifact_timing_prediction"]).sum()),
        "timing_mismatch_count": int((artifact_lead["timing_prediction"] != artifact_lead["artifact_timing_prediction"]).sum()),
        "artifact_slow_count": int((artifact_lead["artifact_timing_prediction"] == "slow").sum()),
    }
    for policy in _policy_specs():
        rows = _policy_rows(lead_all=lead_all, events=events, delay_results=delay_results, policy=policy)
        policy_metrics, scored = _score_policy(rows, policy_id=policy.policy_id)
        metrics.append(policy_metrics)
        if args.write_ledgers:
            scored.to_csv(args.output_dir / f"t2_delay_policy_ledger_{policy.policy_id}.csv", index=False)

    artifact_policy_lead = lead_all.copy()
    artifact_policy_lead["timing_prediction"] = artifact_lead["artifact_timing_prediction"].values
    for policy in [
        PolicySpec(
            "artifact_model_fixed_d0",
            "Current data/pretouch_model.json timing, then D0 for non-skip events.",
            lambda _prediction: "D0",
        ),
        PolicySpec(
            "artifact_model_fast_d0_slow_pullback",
            "Current data/pretouch_model.json timing, fast -> D0 and slow -> pullback.",
            lambda prediction: "pullback" if prediction == "slow" else "D0",
        ),
    ]:
        rows = _policy_rows(lead_all=artifact_policy_lead, events=events, delay_results=delay_results, policy=policy)
        policy_metrics, scored = _score_policy(rows, policy_id=policy.policy_id)
        metrics.append(policy_metrics)
        if args.write_ledgers:
            scored.to_csv(args.output_dir / f"t2_delay_policy_ledger_{policy.policy_id}.csv", index=False)

    summary = {
        "note": "Research-only deployable delay policy audit for T2 lead alignment.",
        "diagnostics": diagnostics,
        "artifact_alignment": artifact_alignment,
        "references": {
            "current_qband_selected_delay_lead_pct": CURRENT_QBAND_LEAD_PCT,
            "base_lead_adverse10_pct": BASE_LEAD_ADVERSE10_PCT,
        },
        "metrics": [asdict(row) for row in metrics],
    }
    (args.output_dir / "t2_delay_policy_alignment_summary.json").write_text(
        json.dumps(summary, indent=2, ensure_ascii=False, default=str) + "\n",
        encoding="utf-8",
    )
    pd.DataFrame([asdict(row) for row in metrics]).sort_values(
        "calendar_sum_pct",
        ascending=False,
    ).to_csv(args.output_dir / "t2_delay_policy_alignment_metrics.csv", index=False)
    _write_report(args.output_dir, metrics, {**diagnostics, "artifact_alignment": artifact_alignment})
    best = max(metrics, key=lambda row: row.calendar_sum_pct)
    live_proxy = next(row for row in metrics if row.policy_id == "fixed_d0_all_non_skip")
    print(
        "t2_delay_policy_alignment "
        f"best={best.policy_id}:{best.calendar_sum_pct:.6f}% "
        f"live_proxy_d0={live_proxy.calendar_sum_pct:.6f}% "
        f"live_proxy_delta_vs_qband={live_proxy.delta_vs_current_qband_lead_pct:.6f}pp"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
