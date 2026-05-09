#!/usr/bin/env python3
"""Probabilistic V4 probability / EV quality model.

Research-only. This script keeps the probability model in the quality layer:
it estimates post-touch continuation probability, converts it into an expected
net first-edge ATR score, and emits `quality_pass` for the execution runner.
It does not simulate balances or mutate live/internal strategy semantics.
"""

from __future__ import annotations

import argparse
import json
from pathlib import Path

import numpy as np
import pandas as pd

from probabilistic_v4_quality_model import _add_markov_scores, _summarize_subset, _time_split


FEATURE_COLUMNS = [
    "markov_llr",
    "markov_prob_success",
    "flow_ratio_60s",
    "flow_ratio_120s",
    "speed_15s_atr",
    "speed_60s_atr",
    "speed_300s_atr",
    "eff_60s",
    "eff_300s",
    "close_pos_60s",
    "close_pos_300s",
    "pullback_15s_atr",
    "pullback_30s_atr",
    "pullback_60s_atr",
    "touch_extension_atr",
    "roundtrip_cost_atr",
    "dwell_5s_pass",
    "dwell_15s_pass",
    "dwell_30s_pass",
]


def _sigmoid(values: np.ndarray) -> np.ndarray:
    return 1.0 / (1.0 + np.exp(-np.clip(values, -35.0, 35.0)))


def _logit(value: float) -> float:
    clipped = min(max(float(value), 1e-6), 1.0 - 1e-6)
    return float(np.log(clipped / (1.0 - clipped)))


def _boolish(series: pd.Series) -> pd.Series:
    if series.dtype == bool:
        return series.astype("float64")
    return series.astype(str).str.lower().isin({"true", "1", "yes"}).astype("float64")


def _add_markov_probability(scored: pd.DataFrame, train_index: pd.Index) -> tuple[pd.DataFrame, dict]:
    train = scored.loc[train_index]
    success = int((train["outcome"] == "continuation").sum())
    total = int(len(train))
    prior = (success + 1.0) / (total + 2.0) if total else 0.5
    result = scored.copy()
    result["markov_prob_success"] = _sigmoid(result["markov_llr"].astype("float64").fillna(0.0).to_numpy() + _logit(prior))
    return result, {"train_success": success, "train_total": total, "prior_success": round(prior, 6)}


def _feature_frame(frame: pd.DataFrame, feature_columns: list[str]) -> pd.DataFrame:
    features = pd.DataFrame(index=frame.index)
    for col in feature_columns:
        if col not in frame.columns:
            features[col] = 0.0
            continue
        if col.endswith("_pass"):
            features[col] = _boolish(frame[col])
        else:
            features[col] = pd.to_numeric(frame[col], errors="coerce")
    return features


def _fit_transform_features(
    scored: pd.DataFrame,
    train_index: pd.Index,
    feature_columns: list[str],
) -> tuple[np.ndarray, np.ndarray, np.ndarray, dict]:
    raw = _feature_frame(scored, feature_columns)
    train_raw = raw.loc[train_index]
    medians = train_raw.median(numeric_only=True).replace([np.inf, -np.inf], np.nan).fillna(0.0)
    filled = raw.replace([np.inf, -np.inf], np.nan).fillna(medians).fillna(0.0)
    train_filled = filled.loc[train_index]
    means = train_filled.mean()
    stds = train_filled.std(ddof=0).replace(0.0, 1.0).fillna(1.0)
    scaled = (filled - means) / stds
    y = (scored.loc[train_index, "outcome"] == "continuation").astype("float64").to_numpy()
    return (
        scaled.to_numpy(dtype="float64"),
        scaled.loc[train_index].to_numpy(dtype="float64"),
        y,
        {
            "feature_columns": feature_columns,
            "medians": {key: round(float(value), 8) for key, value in medians.items()},
            "means": {key: round(float(value), 8) for key, value in means.items()},
            "stds": {key: round(float(value), 8) for key, value in stds.items()},
        },
    )


def _train_logistic(
    x_train: np.ndarray,
    y_train: np.ndarray,
    *,
    learning_rate: float,
    iterations: int,
    l2: float,
) -> tuple[np.ndarray, float, dict]:
    if len(y_train) == 0 or float(y_train.min()) == float(y_train.max()):
        weights = np.zeros(x_train.shape[1], dtype="float64")
        intercept = _logit(float(y_train.mean()) if len(y_train) else 0.5)
        return weights, intercept, {"status": "constant_label", "iterations": 0}

    weights = np.zeros(x_train.shape[1], dtype="float64")
    intercept = _logit(float(y_train.mean()))
    n = float(len(y_train))
    last_loss = 0.0
    for _ in range(int(iterations)):
        logits = x_train @ weights + intercept
        prob = _sigmoid(logits)
        error = prob - y_train
        grad_w = (x_train.T @ error) / n + float(l2) * weights
        grad_b = float(error.mean())
        weights -= float(learning_rate) * grad_w
        intercept -= float(learning_rate) * grad_b
        eps = 1e-9
        last_loss = float(
            -np.mean(y_train * np.log(prob + eps) + (1.0 - y_train) * np.log(1.0 - prob + eps))
            + 0.5 * float(l2) * float(np.dot(weights, weights))
        )
    return weights, float(intercept), {"status": "trained", "iterations": int(iterations), "loss": round(last_loss, 8)}


def _payoff_summary(train: pd.DataFrame) -> dict:
    success = train[train["outcome"] == "continuation"]
    non_success = train[train["outcome"] != "continuation"]
    success_edge = float(success["first_edge_atr"].mean()) if not success.empty else 0.5
    non_success_edge = float(non_success["first_edge_atr"].mean()) if not non_success.empty else -0.2
    return {
        "success_first_edge_atr": round(success_edge, 8),
        "non_success_first_edge_atr": round(non_success_edge, 8),
        "train_success_rate": round(float((train["outcome"] == "continuation").mean()) * 100.0, 4) if not train.empty else 0.0,
    }


def _score_probability_model(
    events: pd.DataFrame,
    train: pd.DataFrame,
    args: argparse.Namespace,
) -> tuple[pd.DataFrame, dict]:
    scored, markov_summary = _add_markov_scores(events, train)
    scored, markov_prior = _add_markov_probability(scored, train.index)
    feature_columns = [col for col in FEATURE_COLUMNS if col in scored.columns or col.endswith("_pass")]
    x_all, x_train, y_train, transform = _fit_transform_features(scored, train.index, feature_columns)
    weights, intercept, train_summary = _train_logistic(
        x_train,
        y_train,
        learning_rate=float(args.learning_rate),
        iterations=int(args.iterations),
        l2=float(args.l2),
    )
    probabilities = _sigmoid(x_all @ weights + intercept)
    payoff = _payoff_summary(scored.loc[train.index])
    success_edge = float(payoff["success_first_edge_atr"])
    non_success_edge = float(payoff["non_success_first_edge_atr"])
    scored["prob_success"] = probabilities
    scored["prob_ev_atr"] = (
        probabilities * success_edge
        + (1.0 - probabilities) * non_success_edge
        - pd.to_numeric(scored["roundtrip_cost_atr"], errors="coerce").fillna(0.0).to_numpy()
    )
    coefs = [
        {"feature": feature, "weight": round(float(weight), 8)}
        for feature, weight in sorted(zip(feature_columns, weights), key=lambda item: abs(float(item[1])), reverse=True)
    ]
    return scored, {
        "markov": markov_summary,
        "markov_prior": markov_prior,
        "logistic": train_summary,
        "intercept": round(float(intercept), 8),
        "coefficients": coefs,
        "feature_transform": transform,
        "payoff": payoff,
    }


def _summarize_probability_subset(frame: pd.DataFrame) -> dict:
    summary = _summarize_subset(frame)
    if frame.empty:
        summary.update({"avg_prob_success": 0.0, "avg_prob_ev_atr": 0.0})
        return summary
    summary.update(
        {
            "avg_prob_success": round(float(frame["prob_success"].mean()), 6),
            "avg_prob_ev_atr": round(float(frame["prob_ev_atr"].mean()), 6),
        }
    )
    return summary


def _threshold_mask(frame: pd.DataFrame, rule: dict) -> pd.Series:
    mask = pd.Series(True, index=frame.index)
    mask &= frame["prob_success"] >= float(rule["prob_min"])
    mask &= frame["prob_ev_atr"] >= float(rule["ev_atr_min"])
    return mask.fillna(False)


def _candidate_rules(args: argparse.Namespace) -> list[dict]:
    rules = []
    for prob_min in args.prob_mins:
        for ev_min in args.ev_atr_mins:
            rules.append({"prob_min": float(prob_min), "ev_atr_min": float(ev_min)})
    return rules


def _select_threshold_rule(validation: pd.DataFrame, args: argparse.Namespace) -> tuple[dict, list[dict]]:
    candidates: list[dict] = []
    for rule in _candidate_rules(args):
        subset = validation[_threshold_mask(validation, rule)]
        summary = _summarize_probability_subset(subset)
        if summary["events"] < int(args.min_events):
            continue
        candidates.append({"rule": rule, "validation": summary})
    if not candidates:
        fallback = {"prob_min": 0.0, "ev_atr_min": -999.0}
        return fallback, [{"rule": fallback, "validation": _summarize_probability_subset(validation)}]
    candidates.sort(
        key=lambda item: (
            item["validation"]["avg_net_first_edge_atr"],
            item["validation"]["avg_prob_ev_atr"],
            item["validation"]["success_rate"],
            item["validation"]["events"],
        ),
        reverse=True,
    )
    return candidates[0]["rule"], candidates


def _calibration_table(frame: pd.DataFrame, bins: int) -> list[dict]:
    if frame.empty:
        return []
    tmp = frame.copy()
    tmp["prob_bin"] = pd.cut(tmp["prob_success"], bins=np.linspace(0.0, 1.0, int(bins) + 1), include_lowest=True)
    rows = []
    for prob_bin, group in tmp.groupby("prob_bin", observed=True):
        if group.empty:
            continue
        rows.append(
            {
                "bin": str(prob_bin),
                "events": int(len(group)),
                "avg_prob_success": round(float(group["prob_success"].mean()), 6),
                "realized_success_rate": round(float((group["outcome"] == "continuation").mean()) * 100.0, 4),
                "avg_net_first_edge_atr": round(float(group["net_first_edge_atr"].mean()), 6),
            }
        )
    return rows


def _write_markdown(summary: dict, path: Path) -> None:
    lines = [
        "# Probabilistic V4 Probability Model",
        "",
        "范围：仅限 `research`。概率模型只输出 post-touch continuation probability / EV，不直接决定仓位或 live 语义。",
        "",
        f"- Selection scope: `{summary['selection_scope']}`",
        f"- Split: `{summary['split']}`",
        "",
    ]
    if summary["selection_scope"] == "global":
        selected = summary["selected_rule"]
        validation = summary["selected_validation"]
        lines.extend(
            [
                "## Selected Threshold",
                "",
                f"- `prob_min`: `{selected['prob_min']}`",
                f"- `ev_atr_min`: `{selected['ev_atr_min']}`",
                "",
                "| Events | Success | Net Edge ATR | Avg Prob | Avg Prob EV ATR |",
                "|---:|---:|---:|---:|---:|",
                f"| {validation['events']} | {validation['success_rate']:.4f}% | "
                f"{validation['avg_net_first_edge_atr']:.6f} | {validation['avg_prob_success']:.6f} | "
                f"{validation['avg_prob_ev_atr']:.6f} |",
                "",
                "## Top Coefficients",
                "",
                "| Feature | Weight |",
                "|---|---:|",
            ]
        )
        for row in summary["model"]["coefficients"][:12]:
            lines.append(f"| `{row['feature']}` | {row['weight']:.8f} |")
        lines.extend(["", "## Calibration", "", "| Bin | Events | Avg Prob | Realized Success | Net Edge ATR |", "|---|---:|---:|---:|---:|"])
        for row in summary["calibration"]:
            lines.append(
                f"| `{row['bin']}` | {row['events']} | {row['avg_prob_success']:.6f} | "
                f"{row['realized_success_rate']:.4f}% | {row['avg_net_first_edge_atr']:.6f} |"
            )
    else:
        lines.extend(
            [
                "## Selected Thresholds By Symbol",
                "",
                "| Symbol | Events | Prob Min | EV Min | Success | Net Edge ATR | Avg Prob | Avg Prob EV ATR |",
                "|---|---:|---:|---:|---:|---:|---:|---:|",
            ]
        )
        for symbol, selected in summary["selected_by_symbol"].items():
            rule = selected["rule"]
            validation = selected["validation"]
            lines.append(
                f"| `{symbol}` | {validation['events']} | {rule['prob_min']} | {rule['ev_atr_min']} | "
                f"{validation['success_rate']:.4f}% | {validation['avg_net_first_edge_atr']:.6f} | "
                f"{validation['avg_prob_success']:.6f} | {validation['avg_prob_ev_atr']:.6f} |"
            )
        lines.extend(["", "## Top Coefficients By Symbol", ""])
        for symbol, model in summary["model_by_symbol"].items():
            lines.extend([f"### {symbol}", "", "| Feature | Weight |", "|---|---:|"])
            for row in model["coefficients"][:12]:
                lines.append(f"| `{row['feature']}` | {row['weight']:.8f} |")
            lines.append("")
    path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def _run_global(events: pd.DataFrame, train: pd.DataFrame, validation: pd.DataFrame, args: argparse.Namespace) -> tuple[pd.DataFrame, dict]:
    scored, model = _score_probability_model(events, train, args)
    scored_validation = scored.loc[validation.index]
    selected_rule, rules = _select_threshold_rule(scored_validation, args)
    scored["quality_pass"] = _threshold_mask(scored, selected_rule)
    scored["quality_bucket"] = np.where(scored["quality_pass"], "selected", "rejected")
    summary = {
        "selection_scope": "global",
        "model": model,
        "selected_rule": selected_rule,
        "selected_validation": _summarize_probability_subset(scored_validation[_threshold_mask(scored_validation, selected_rule)]),
        "validation_baseline": _summarize_probability_subset(scored_validation),
        "top_rules": rules[:25],
        "calibration": _calibration_table(scored_validation, int(args.calibration_bins)),
    }
    return scored, summary


def _run_per_symbol(events: pd.DataFrame, train: pd.DataFrame, validation: pd.DataFrame, args: argparse.Namespace) -> tuple[pd.DataFrame, dict]:
    scored = events.copy()
    scored["markov_llr"] = 0.0
    scored["markov_prob_success"] = 0.0
    scored["prob_success"] = 0.0
    scored["prob_ev_atr"] = 0.0
    scored["quality_pass"] = False
    selected_by_symbol: dict[str, dict] = {}
    model_by_symbol: dict[str, dict] = {}
    top_rules_by_symbol: dict[str, list[dict]] = {}
    calibration_by_symbol: dict[str, list[dict]] = {}

    for symbol in sorted(events["symbol"].dropna().unique()):
        symbol_events = events[events["symbol"] == symbol]
        symbol_train = train[train["symbol"] == symbol]
        symbol_validation = validation[validation["symbol"] == symbol]
        symbol_scored, model = _score_probability_model(symbol_events, symbol_train, args)
        scored.loc[symbol_scored.index, ["markov_llr", "markov_prob_success", "prob_success", "prob_ev_atr"]] = symbol_scored[
            ["markov_llr", "markov_prob_success", "prob_success", "prob_ev_atr"]
        ]
        scored_validation = scored.loc[symbol_validation.index]
        selected_rule, rules = _select_threshold_rule(scored_validation, args)
        symbol_pass = _threshold_mask(scored.loc[symbol_events.index], selected_rule)
        scored.loc[symbol_events.index, "quality_pass"] = symbol_pass
        selected_by_symbol[str(symbol)] = {
            "rule": selected_rule,
            "validation": _summarize_probability_subset(scored_validation[_threshold_mask(scored_validation, selected_rule)]),
            "validation_baseline": _summarize_probability_subset(scored_validation),
        }
        model_by_symbol[str(symbol)] = model
        top_rules_by_symbol[str(symbol)] = rules[:25]
        calibration_by_symbol[str(symbol)] = _calibration_table(scored_validation, int(args.calibration_bins))

    scored["quality_bucket"] = np.where(scored["quality_pass"], "selected", "rejected")
    summary = {
        "selection_scope": "per_symbol",
        "selected_rule": {"scope": "per_symbol", "rules_by_symbol": {k: v["rule"] for k, v in selected_by_symbol.items()}},
        "selected_by_symbol": selected_by_symbol,
        "model_by_symbol": model_by_symbol,
        "top_rules_by_symbol": top_rules_by_symbol,
        "calibration_by_symbol": calibration_by_symbol,
        "validation_baseline": _summarize_probability_subset(scored.loc[validation.index]),
    }
    return scored, summary


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Train Probabilistic V4 probability / EV quality model")
    parser.add_argument("--events-csv", default="research/probabilistic_v4_events.csv")
    parser.add_argument("--scored-csv", default="research/probabilistic_v4_events_probability_scored.csv")
    parser.add_argument("--model-json", default="research/probabilistic_v4_probability_model.json")
    parser.add_argument("--markdown", default="research/20260508_probabilistic_v4_probability_model.md")
    parser.add_argument("--train-end", default="")
    parser.add_argument("--train-ratio", type=float, default=0.70)
    parser.add_argument("--selection-scope", choices=["global", "per_symbol"], default="global")
    parser.add_argument("--min-events", type=int, default=20)
    parser.add_argument("--prob-mins", nargs="+", type=float, default=[0.40, 0.45, 0.50, 0.55, 0.60, 0.65])
    parser.add_argument("--ev-atr-mins", nargs="+", type=float, default=[-0.05, 0.0, 0.05, 0.10, 0.15])
    parser.add_argument("--learning-rate", type=float, default=0.05)
    parser.add_argument("--iterations", type=int, default=2500)
    parser.add_argument("--l2", type=float, default=0.01)
    parser.add_argument("--calibration-bins", type=int, default=5)
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    events = pd.read_csv(args.events_csv, parse_dates=["touch_time", "signal_start", "signal_end"])
    if events.empty:
        raise SystemExit(f"empty events dataset: {args.events_csv}")

    train, validation, split_description = _time_split(events, args)
    if args.selection_scope == "global":
        scored, summary = _run_global(events, train, validation, args)
    else:
        scored, summary = _run_per_symbol(events, train, validation, args)

    scored_path = Path(args.scored_csv)
    scored_path.parent.mkdir(parents=True, exist_ok=True)
    scored.to_csv(scored_path, index=False)
    summary.update(
        {
            "events_csv": args.events_csv,
            "scored_csv": str(scored_path),
            "split": split_description,
            "train_rows": int(len(train)),
            "validation_rows": int(len(validation)),
            "train_baseline": _summarize_probability_subset(scored.loc[train.index]),
            "validation_baseline_all": _summarize_probability_subset(scored.loc[validation.index]),
        }
    )
    model_path = Path(args.model_json)
    model_path.parent.mkdir(parents=True, exist_ok=True)
    model_path.write_text(json.dumps(summary, indent=2, ensure_ascii=False), encoding="utf-8")
    _write_markdown(summary, Path(args.markdown))
    print(
        json.dumps(
            {
                "model_json": str(model_path),
                "scored_csv": str(scored_path),
                "selected_rule": summary["selected_rule"],
            },
            indent=2,
            ensure_ascii=False,
        ),
        flush=True,
    )


if __name__ == "__main__":
    main()
