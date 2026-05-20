"""Export the T3 overlay RF/cost quality model as a Go-readable artifact.

The sizing audit in ``t3_overlay_rf_cost_sizing.py`` found that the useful T3
overlay variant can be promoted from a small multiplier band to an absolute
0.20-0.40 ETH testnet-shadow quantity band. This exporter trains the current
live/testnet artifact on all available historical T3 overlay events and writes
a ``PretouchModelBundle`` JSON that Go can load with ``LoadModelBundle``.

Important: the best research metric is walk-forward. The exported live artifact
is the current accumulated-history model for shadow collection, not a claim that
a single static model exactly reproduces the walk-forward backtest.
"""

from __future__ import annotations

import argparse
import json
import logging
import math
import sys
from datetime import datetime, timezone
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

from timing_probability_unified.t2_lifecycle_context_sizing import EXTENDED_MONTHS  # noqa: E402
from timing_probability_unified.t3_overlay_rf_cost_sizing import (  # noqa: E402
    DEFAULT_OUTPUT,
    DEFAULT_SCORED_EVENTS,
    FEATURE_COLUMNS,
    build_event_table,
    collect_overlay_trades,
    _fit_rf,
    _predict_class_one,
)
from timing_probability_unified.t3_pre_touch_lifecycle import INITIAL_BALANCE  # noqa: E402

logger = logging.getLogger(__name__)

DEFAULT_BASE_TRADES = DEFAULT_OUTPUT / "t3_overlay_rf_cost_base_trades.csv"
DEFAULT_MODEL_OUT = PROJECT_ROOT / "data" / "pretouch_t3_overlay_rf_model.json"
DEFAULT_REPORT_OUT = DEFAULT_OUTPUT / "t3_overlay_rf_model_artifact_report.md"
DEFAULT_SUMMARY = DEFAULT_OUTPUT / "t3_overlay_rf_cost_sizing_summary.json"
DEFAULT_VERSION = "20260520_t3_overlay_rf_cost_v1"


def _path_label(path: Path | str) -> str:
    resolved = Path(path)
    try:
        return str(resolved.resolve().relative_to(PROJECT_ROOT))
    except (OSError, ValueError):
        return str(resolved)


def _json_float(value: Any, fallback: float = 0.0) -> float:
    numeric = float(value)
    if math.isnan(numeric) or math.isinf(numeric):
        return float(fallback)
    return numeric


def _tree_to_node(model: RandomForestClassifier, estimator: Any, node_id: int = 0) -> dict[str, Any]:
    tree = estimator.tree_
    left = int(tree.children_left[node_id])
    right = int(tree.children_right[node_id])
    if left == right:
        values = tree.value[node_id][0].astype(float)
        total = float(values.sum())
        class_lookup = {int(label): idx for idx, label in enumerate(model.classes_)}
        class_one_idx = class_lookup.get(1)
        probability = 0.5 if total <= 0 or class_one_idx is None else float(values[class_one_idx] / total)
        probability = _json_float(probability, 0.5)
        return {
            "f": -1,
            "v": "1" if probability >= 0.5 else "0",
            "p": probability,
        }
    return {
        "f": int(tree.feature[node_id]),
        "t": _json_float(tree.threshold[node_id]),
        "l": _tree_to_node(model, estimator, left),
        "r": _tree_to_node(model, estimator, right),
    }


def export_random_forest(model: RandomForestClassifier) -> dict[str, Any]:
    """Convert a sklearn random forest to the Go ``RandomForest`` JSON shape."""
    return {
        "trees": [_tree_to_node(model, estimator) for estimator in model.estimators_],
        "n_estimators": int(len(model.estimators_)),
    }


def predict_exported_tree(node: dict[str, Any], row: list[float]) -> float:
    feature_index = int(node.get("f", -1))
    if feature_index < 0 or ("l" not in node and "r" not in node):
        return _json_float(node.get("p", 0.5), 0.5)
    threshold = _json_float(node.get("t", 0.0))
    child = node.get("l") if row[feature_index] <= threshold else node.get("r")
    if not isinstance(child, dict):
        return 0.5
    return predict_exported_tree(child, row)


def predict_exported_forest(forest: dict[str, Any], frame: pd.DataFrame, feature_columns: list[str]) -> np.ndarray:
    rows = frame[feature_columns].astype(np.float32).astype(float).to_numpy()
    trees = [tree for tree in forest.get("trees", []) if isinstance(tree, dict)]
    if not trees:
        return np.full(len(rows), 0.5)
    values = []
    for row in rows:
        values.append(float(np.mean([predict_exported_tree(tree, row.tolist()) for tree in trees])))
    return np.asarray(values, dtype=float)


def train_t3_overlay_model(events: pd.DataFrame, *, random_state: int) -> RandomForestClassifier:
    model = _fit_rf(events, FEATURE_COLUMNS, random_state=random_state)
    if model is None:
        raise ValueError("insufficient T3 overlay events to train RF model")
    return model


def build_model_bundle(
    events: pd.DataFrame,
    *,
    model: RandomForestClassifier,
    version: str,
    trained_at: str,
    source: str,
    random_state: int,
) -> dict[str, Any]:
    medians = [_json_float(events[column].astype(float).median(), 0.0) for column in FEATURE_COLUMNS]
    probabilities = _predict_class_one(model, events, FEATURE_COLUMNS)
    labels = events["label_win"].astype(int).to_numpy()
    predictions = (probabilities >= 0.5).astype(int)
    accuracy = float((predictions == labels).mean()) if len(labels) else 0.0
    months = sorted(str(month) for month in events["event_month"].dropna().unique())
    exported_forest = export_random_forest(model)
    return {
        "timing_tree": {"f": -1, "v": "fast", "p": 1.0},
        "rf_model": exported_forest,
        "feature_names": list(FEATURE_COLUMNS),
        "medians": medians,
        "version": version,
        "trained_at": trained_at,
        "timing_loocv": 1.0,
        "rf_accuracy": accuracy,
        "artifact_kind": "pretouch_t3_overlay_rf_quality_sizing",
        "training_source": source,
        "training_rows": int(len(events)),
        "training_months": months,
        "random_state": int(random_state),
        "sizing_policy": {
            "method": "t3_rf_cost_quantity_band",
            "min_quantity": 0.20,
            "max_quantity": 0.40,
            "reference_fixed_overlay_quantity": 0.08,
            "equivalent_multiplier_band": [2.5, 5.0],
            "cost_threshold_atr": 0.10,
            "base_overlay_quantity_formula": "pretouchBaseOrderQuantity * pretouchShadowOverlayBaseShare * pretouchShadowOverlayScale",
        },
        "target": "label_win = event_net_pnl_pct > 0 after strict T3 60m lifecycle pairing",
    }


def load_or_collect_base_trades(args: argparse.Namespace) -> pd.DataFrame:
    base_trades = Path(args.base_trades)
    if base_trades.exists():
        logger.info("Loading cached T3 overlay trades from %s", base_trades)
        return pd.read_csv(base_trades)
    if not args.replay_if_missing:
        raise FileNotFoundError(
            f"{base_trades} is missing. Re-run t3_overlay_rf_cost_sizing.py or pass --replay-if-missing."
        )
    logger.info("Cached base trades missing; replaying T3 overlay lifecycle")
    return collect_overlay_trades(
        scored_events=Path(args.scored_events),
        months=[str(month) for month in args.months],
        symbol=str(args.symbol),
        timeframe=str(args.timeframe),
        initial_balance=float(args.initial_balance),
        external_entry_mode=str(args.external_entry_mode),
        t3_size_scale=float(args.t3_size_scale),
        reentry_fill_policy=str(args.reentry_fill_policy),
        early_horizon_seconds=int(args.early_horizon_seconds),
    )


def write_report(
    *,
    report_out: Path,
    model_out: Path,
    bundle: dict[str, Any],
    summary_path: Path,
) -> None:
    report_out.parent.mkdir(parents=True, exist_ok=True)
    wf_line = ""
    if summary_path.exists():
        summary = json.loads(summary_path.read_text(encoding="utf-8"))
        metrics = {row["variant"]: row for row in summary.get("metrics", [])}
        wf = metrics.get("wf_t3_rf_cost_quantity_0p20_0p40_shadow")
        fixed = metrics.get("fixed_overlay_2p0")
        if wf and fixed:
            wf_line = (
                f"- Walk-forward evidence: `wf_t3_rf_cost_quantity_0p20_0p40_shadow` "
                f"overlay `{float(wf['calendar_sum_pct']):.6f}%`, delta vs fixed "
                f"`{float(wf['overlay_delta_vs_fixed_pct']):.6f}pp`, lead adverse10 + overlay "
                f"`{float(wf['lead_adverse10_plus_overlay_pct']):.6f}%`; fixed overlay "
                f"`{float(fixed['calendar_sum_pct']):.6f}%`, avg quantity "
                f"`{float(wf.get('avg_event_quantity', 0.0)):.6f}` ETH.\n"
            )
    lines = [
        "# T3 Overlay RF Model Artifact",
        "",
        "This artifact is for ETHUSDT 1h testnet shadow T3 overlay quality sizing.",
        "",
        f"- Artifact: `{_path_label(model_out)}`",
        f"- Version: `{bundle['version']}`",
        f"- Trained at: `{bundle['trained_at']}`",
        f"- Training source: `{bundle['training_source']}`",
        f"- Training rows: `{bundle['training_rows']}`",
        f"- Training months: `{', '.join(bundle['training_months'])}`",
        f"- Features: `{', '.join(bundle['feature_names'])}`",
        f"- In-sample RF accuracy: `{float(bundle['rf_accuracy']):.6f}`",
        "- Live sizing policy: map RF probability into absolute T3 overlay quantity `0.20..0.40` ETH, apply cost penalty, then clamp back to `0.20..0.40` ETH.",
        "- Default testnet quantity band at base `0.100` ETH: `0.20..0.40` ETH, equivalent to `0.08 * 2.5..5.0`.",
        "- This is an accumulated-history shadow artifact; it does not claim to exactly reproduce the walk-forward research curve.",
    ]
    if wf_line:
        lines.extend(["", wf_line.rstrip()])
    report_out.write_text("\n".join(lines) + "\n", encoding="utf-8")


def run(args: argparse.Namespace) -> dict[str, Any]:
    base_trades = load_or_collect_base_trades(args)
    if base_trades.empty:
        raise ValueError("no T3 overlay trades available")
    events = build_event_table(base_trades, initial_balance=float(args.initial_balance))
    model = train_t3_overlay_model(events, random_state=int(args.random_state))
    trained_at = args.trained_at or datetime.now(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")
    bundle = build_model_bundle(
        events,
        model=model,
        version=str(args.version),
        trained_at=str(trained_at),
        source=_path_label(Path(args.base_trades)),
        random_state=int(args.random_state),
    )
    model_out = Path(args.model_out)
    model_out.parent.mkdir(parents=True, exist_ok=True)
    model_out.write_text(json.dumps(bundle, indent=2, sort_keys=False) + "\n", encoding="utf-8")
    write_report(
        report_out=Path(args.report_out),
        model_out=model_out,
        bundle=bundle,
        summary_path=Path(args.summary),
    )
    exported_prob = predict_exported_forest(bundle["rf_model"], events, FEATURE_COLUMNS)
    sklearn_prob = _predict_class_one(model, events, FEATURE_COLUMNS)
    max_abs_diff = float(np.max(np.abs(exported_prob - sklearn_prob))) if len(events) else 0.0
    if max_abs_diff > 1e-12:
        raise ValueError(f"exported forest probability mismatch: max_abs_diff={max_abs_diff}")
    return {
        "model_out": str(model_out),
        "report_out": str(args.report_out),
        "version": bundle["version"],
        "training_rows": bundle["training_rows"],
        "rf_accuracy": bundle["rf_accuracy"],
        "max_abs_diff": max_abs_diff,
    }


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base-trades", type=Path, default=DEFAULT_BASE_TRADES)
    parser.add_argument("--model-out", type=Path, default=DEFAULT_MODEL_OUT)
    parser.add_argument("--report-out", type=Path, default=DEFAULT_REPORT_OUT)
    parser.add_argument("--summary", type=Path, default=DEFAULT_SUMMARY)
    parser.add_argument("--version", default=DEFAULT_VERSION)
    parser.add_argument("--trained-at", default="2026-05-20T00:00:00Z")
    parser.add_argument("--random-state", type=int, default=42)
    parser.add_argument("--replay-if-missing", action="store_true")
    parser.add_argument("--scored-events", type=Path, default=DEFAULT_SCORED_EVENTS)
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
    parser.add_argument("--log-level", default="INFO")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    logging.basicConfig(
        level=getattr(logging, str(args.log_level).upper(), logging.INFO),
        format="%(asctime)s %(levelname)s %(message)s",
    )
    result = run(args)
    print(
        "model_out={model_out} report_out={report_out} version={version} "
        "training_rows={training_rows} rf_accuracy={rf_accuracy:.6f} max_abs_diff={max_abs_diff:.3g}".format(
            **result
        )
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
