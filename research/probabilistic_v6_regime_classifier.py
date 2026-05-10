#!/usr/bin/env python3
"""Walk-forward regime classifier over V6 symbol/topK rows.

Research-only. This script consumes already executed `symbol_rows.csv` files and
tests whether pre-execute validation/topK fields can learn a no-trade regime
gate. It never uses the current execute month label for model or threshold
selection.
"""

from __future__ import annotations

import argparse
import json
from pathlib import Path

import numpy as np
import pandas as pd
from sklearn.ensemble import ExtraTreesClassifier, GradientBoostingClassifier, RandomForestClassifier
from sklearn.impute import SimpleImputer
from sklearn.linear_model import LogisticRegression
from sklearn.pipeline import Pipeline
from sklearn.preprocessing import StandardScaler
from sklearn.svm import SVC


NUMERIC_FEATURES = [
    "top_k",
    "selected_events",
    "validation_topk_events",
    "validation_topk_before",
    "validation_topk_return_pct",
    "validation_topk_sized_return_pct",
    "validation_topk_return_over_dd",
    "validation_topk_initial_sl_rate",
    "validation_topk_win_rate_pct",
    "validation_topk_max_dd_pct",
    "validation_topk_prob_success_mean",
    "validation_topk_prob_success_std",
    "validation_topk_prob_ev_atr_mean",
    "validation_topk_prob_ev_atr_std",
    "validation_topk_prob_initial_sl_mean",
    "validation_topk_prob_initial_sl_std",
    "validation_topk_model_notional_share_mean",
    "validation_topk_model_notional_share_std",
    "validation_topk_sizing_score_mean",
    "validation_topk_sizing_score_std",
    "validation_topk_sizing_ev_score_mean",
    "validation_topk_sizing_ev_score_std",
    "validation_topk_sizing_prob_score_mean",
    "validation_topk_sizing_prob_score_std",
    "validation_topk_sizing_markov_score_mean",
    "validation_topk_sizing_markov_score_std",
    "validation_topk_sizing_sl_score_mean",
    "validation_topk_sizing_sl_score_std",
    "validation_topk_speed_60s_atr_mean",
    "validation_topk_speed_60s_atr_std",
    "validation_topk_eff_60s_mean",
    "validation_topk_eff_60s_std",
    "validation_topk_close_pos_60s_mean",
    "validation_topk_close_pos_60s_std",
    "validation_topk_pullback_60s_atr_mean",
    "validation_topk_pullback_60s_atr_std",
    "validation_topk_flow_ratio_60s_mean",
    "validation_topk_flow_ratio_60s_std",
    "validation_events",
]

CATEGORICAL_FEATURES = ["symbol", "model_name", "sizing_mode", "final_sizing_result"]


def _float_grid(values: list[str]) -> list[float]:
    return [float(value) for value in values]


def _load_rows(paths: list[str]) -> pd.DataFrame:
    frames = []
    for raw_path in paths:
        path = Path(raw_path)
        frame = pd.read_csv(path)
        frame["source_run"] = path.parent.name
        frames.append(frame)
    if not frames:
        return pd.DataFrame()
    rows = pd.concat(frames, ignore_index=True)
    rows["execute_month"] = rows["execute_month"].astype(str)
    rows["realistic_return_pct"] = pd.to_numeric(rows["realistic_return_pct"], errors="coerce").fillna(0.0)
    rows["trades"] = pd.to_numeric(rows["trades"], errors="coerce").fillna(0).astype(int)
    rows["top_k"] = pd.to_numeric(rows["top_k"], errors="coerce").fillna(0).astype(int)
    return rows[rows["trades"] > 0].copy()


def _feature_frame(rows: pd.DataFrame) -> pd.DataFrame:
    features = pd.DataFrame(index=rows.index)
    for col in NUMERIC_FEATURES:
        if col in rows.columns:
            features[col] = pd.to_numeric(rows[col], errors="coerce")
    for col in CATEGORICAL_FEATURES:
        if col in rows.columns:
            features[col] = rows[col].astype(str).fillna("")
    if "validation_topk_return_pct" in features.columns and "validation_topk_max_dd_pct" in features.columns:
        dd = pd.to_numeric(features["validation_topk_max_dd_pct"], errors="coerce").abs().replace(0.0, np.nan)
        ret = pd.to_numeric(features["validation_topk_return_pct"], errors="coerce")
        features["validation_return_over_abs_dd"] = ret / dd
    return pd.get_dummies(features, columns=[col for col in CATEGORICAL_FEATURES if col in features.columns])


def _models(names: list[str]) -> dict[str, Pipeline]:
    available = {
        "logistic": Pipeline(
            [
                ("imputer", SimpleImputer(strategy="median")),
                ("scaler", StandardScaler()),
                ("model", LogisticRegression(max_iter=1000, class_weight="balanced", random_state=45)),
            ]
        ),
        "random_forest": Pipeline(
            [
                ("imputer", SimpleImputer(strategy="median")),
                (
                    "model",
                    RandomForestClassifier(
                        n_estimators=250,
                        max_depth=4,
                        min_samples_leaf=2,
                        class_weight="balanced_subsample",
                        random_state=45,
                    ),
                ),
            ]
        ),
        "extra_trees": Pipeline(
            [
                ("imputer", SimpleImputer(strategy="median")),
                (
                    "model",
                    ExtraTreesClassifier(
                        n_estimators=300,
                        max_depth=4,
                        min_samples_leaf=2,
                        class_weight="balanced",
                        random_state=45,
                    ),
                ),
            ]
        ),
        "gradient_boosting": Pipeline(
            [
                ("imputer", SimpleImputer(strategy="median")),
                ("model", GradientBoostingClassifier(n_estimators=120, learning_rate=0.04, max_depth=2, random_state=45)),
            ]
        ),
        "svm_rbf": Pipeline(
            [
                ("imputer", SimpleImputer(strategy="median")),
                ("scaler", StandardScaler()),
                ("model", SVC(C=1.0, gamma="scale", probability=True, class_weight="balanced", random_state=45)),
            ]
        ),
    }
    return {name: available[name] for name in names if name in available}


def _align(train_x: pd.DataFrame, score_x: pd.DataFrame) -> tuple[pd.DataFrame, pd.DataFrame]:
    train_x, score_x = train_x.align(score_x, join="outer", axis=1, fill_value=0.0)
    return train_x, score_x


def _fit_predict(model: Pipeline, train_rows: pd.DataFrame, score_rows: pd.DataFrame, args: argparse.Namespace) -> np.ndarray | None:
    labels = (train_rows["realistic_return_pct"] >= float(args.label_return_min_pct)).astype(int)
    if labels.nunique() < 2:
        return None
    train_x = _feature_frame(train_rows)
    score_x = _feature_frame(score_rows)
    train_x, score_x = _align(train_x, score_x)
    model.fit(train_x, labels)
    return model.predict_proba(score_x)[:, 1]


def _select_rows(scored: pd.DataFrame, prob_min: float, rank_col: str = "regime_prob") -> pd.DataFrame:
    if scored.empty:
        return scored.copy()
    passing = scored[scored[rank_col] >= float(prob_min)].copy()
    if passing.empty:
        return passing
    passing = passing.sort_values(
        [rank_col, "validation_topk_sized_return_pct", "validation_topk_sizing_markov_score_mean"],
        ascending=[False, False, False],
    )
    return passing.groupby(["execute_month", "symbol"], as_index=False).head(1).copy()


def _summarize(selected: pd.DataFrame, candidates: pd.DataFrame) -> dict:
    if selected.empty:
        return {
            "active_rows": 0,
            "active_months": 0,
            "trades": 0,
            "total_realistic_pct": 0.0,
            "worst_month_realistic_pct": 0.0,
            "best_month_realistic_pct": 0.0,
            "unique_symbol_month_selection": True,
            "skipped_candidate_rows": int(len(candidates)),
        }
    monthly = selected.groupby("execute_month")["realistic_return_pct"].sum()
    keys = list(zip(selected["execute_month"].astype(str), selected["symbol"].astype(str)))
    return {
        "active_rows": int(len(selected)),
        "active_months": int(selected["execute_month"].nunique()),
        "trades": int(selected["trades"].sum()),
        "total_realistic_pct": round(float(selected["realistic_return_pct"].sum()), 6),
        "worst_month_realistic_pct": round(float(monthly.min()), 6),
        "best_month_realistic_pct": round(float(monthly.max()), 6),
        "unique_symbol_month_selection": len(keys) == len(set(keys)),
        "skipped_candidate_rows": int(max(0, len(candidates) - len(selected))),
    }


def _meets_constraints(summary: dict, args: argparse.Namespace) -> bool:
    if int(summary["active_rows"]) < int(args.min_active_rows):
        return False
    if int(summary["active_months"]) < int(args.min_active_months):
        return False
    if int(summary["trades"]) < int(args.min_trades):
        return False
    if float(summary["worst_month_realistic_pct"]) < float(args.min_worst_month_realistic_pct):
        return False
    if not bool(summary["unique_symbol_month_selection"]):
        return False
    return True


def _validation_score(summary: dict) -> tuple[float, float, int, int]:
    return (
        float(summary["total_realistic_pct"]),
        float(summary["worst_month_realistic_pct"]),
        int(summary["active_months"]),
        int(summary["trades"]),
    )


def _candidate_choice(train_rows: pd.DataFrame, validation_rows: pd.DataFrame, models: dict[str, Pipeline], args: argparse.Namespace) -> dict | None:
    best: dict | None = None
    for model_name, model in models.items():
        probs = _fit_predict(model, train_rows, validation_rows, args)
        if probs is None:
            continue
        scored = validation_rows.copy()
        scored["regime_prob"] = probs
        for prob_min in args.prob_thresholds:
            selected = _select_rows(scored, prob_min)
            summary = _summarize(selected, validation_rows)
            if int(summary["active_rows"]) < int(args.min_validation_active_rows):
                continue
            item = {
                "model_name": model_name,
                "prob_min": float(prob_min),
                "validation": summary,
            }
            if best is None or _validation_score(summary) > _validation_score(best["validation"]):
                best = item
    return best


def _oracle_positive_per_symbol_month(rows: pd.DataFrame) -> dict:
    positives = []
    for _, group in rows.groupby(["execute_month", "symbol"]):
        best = group.sort_values("realistic_return_pct", ascending=False).head(1)
        if float(best.iloc[0]["realistic_return_pct"]) > 0.0:
            positives.append(best)
    if not positives:
        return _summarize(pd.DataFrame(columns=rows.columns), rows)
    return _summarize(pd.concat(positives, ignore_index=True), rows)


def _write_markdown(result: dict, path: Path) -> None:
    lines = [
        "# Probabilistic V6 Regime Classifier",
        "",
        "范围：仅限 `research`。本报告只消费已生成的 `symbol_rows.csv`，用于验证 no-trade / tradeable regime classifier 是否值得进入下一轮 runner 集成。",
        "",
        "## Summary",
        "",
        f"- rows_csv: `{', '.join(result['rows_csv'])}`",
        f"- months: `{', '.join(result['months'])}`",
        f"- target_hit: `{result['target_hit']}`",
        f"- oracle_positive_per_symbol_month: `{result['oracle_positive_per_symbol_month']['total_realistic_pct']:.4f}%`",
        "",
        "| Metric | Value |",
        "|---|---:|",
    ]
    summary = result["summary"]
    for key in [
        "active_rows",
        "active_months",
        "trades",
        "total_realistic_pct",
        "worst_month_realistic_pct",
        "best_month_realistic_pct",
    ]:
        value = summary[key]
        suffix = "%" if key.endswith("_pct") else ""
        lines.append(f"| {key} | {value}{suffix} |")

    lines.extend(
        [
            "",
            "## Walk-Forward Decisions",
            "",
            "| Execute Month | Validation Month | Model | Prob Min | Val Return | Selected | Trades | Execute Return | Worst Month |",
            "|---|---|---|---:|---:|---:|---:|---:|---:|",
        ]
    )
    for item in result["walkforward"]:
        execution = item["execution"]
        validation = item["validation"]
        lines.append(
            f"| `{item['execute_month']}` | `{item['validation_month']}` | `{item['model_name']}` | "
            f"{item['prob_min']:.2f} | {validation['total_realistic_pct']:.4f}% | "
            f"{execution['active_rows']} | {execution['trades']} | {execution['total_realistic_pct']:.4f}% | "
            f"{execution['worst_month_realistic_pct']:.4f}% |"
        )

    lines.extend(
        [
            "",
            "## Selected Rows",
            "",
            "| Month | Symbol | TopK | Model | Regime Prob | Trades | Realistic | Val Return/DD | Val Markov |",
            "|---|---|---:|---|---:|---:|---:|---:|---:|",
        ]
    )
    for row in result["selected_rows"]:
        lines.append(
            f"| `{row['execute_month']}` | `{row['symbol']}` | {row['top_k']} | `{row['row_model_name']}` | "
            f"{row['regime_prob']:.4f} | {row['trades']} | {row['realistic_return_pct']:.4f}% | "
            f"{row['validation_topk_return_over_dd']:.4f} | {row['validation_topk_sizing_markov_score_mean']:.4f} |"
        )

    lines.extend(
        [
            "",
            "## Interpretation",
            "",
            "- 模型和阈值只用历史月份与上一个已完成 execute month 选择；当前 execute month 的收益只用于 OOS 评分。",
            "- 若结果仍低于 `10%`，说明仅靠 symbol-row 级 validation metrics 不足，需要更上游的 regime label 或事件簇标签。",
        ]
    )
    path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Walk-forward V6 regime classifier")
    parser.add_argument("--rows-csv", nargs="+", required=True)
    parser.add_argument("--output-json", required=True)
    parser.add_argument("--markdown", required=True)
    parser.add_argument("--selected-rows-csv", required=True)
    parser.add_argument("--models", nargs="+", default=["logistic", "random_forest", "extra_trees", "gradient_boosting", "svm_rbf"])
    parser.add_argument("--prob-thresholds", nargs="+", default=["0.50", "0.55", "0.60", "0.65", "0.70", "0.75"])
    parser.add_argument("--min-train-months", type=int, default=4)
    parser.add_argument("--label-return-min-pct", type=float, default=0.0)
    parser.add_argument("--min-validation-active-rows", type=int, default=1)
    parser.add_argument("--target-realistic-pct", type=float, default=10.0)
    parser.add_argument("--min-active-rows", type=int, default=4)
    parser.add_argument("--min-active-months", type=int, default=6)
    parser.add_argument("--min-trades", type=int, default=40)
    parser.add_argument("--min-worst-month-realistic-pct", type=float, default=-2.0)
    args = parser.parse_args()
    args.prob_thresholds = _float_grid(args.prob_thresholds)
    return args


def main() -> None:
    args = parse_args()
    rows = _load_rows(args.rows_csv)
    months = sorted(rows["execute_month"].unique().tolist())
    models = _models(args.models)
    selected_frames = []
    walkforward = []
    for idx in range(int(args.min_train_months) + 1, len(months)):
        validation_month = months[idx - 1]
        execute_month = months[idx]
        train_rows = rows[rows["execute_month"].isin(months[: idx - 1])].copy()
        validation_rows = rows[rows["execute_month"] == validation_month].copy()
        execute_rows = rows[rows["execute_month"] == execute_month].copy()
        choice = _candidate_choice(train_rows, validation_rows, models, args)
        if choice is None:
            continue
        model = models[choice["model_name"]]
        fit_rows = rows[rows["execute_month"].isin(months[:idx])].copy()
        probs = _fit_predict(model, fit_rows, execute_rows, args)
        if probs is None:
            continue
        scored = execute_rows.copy()
        scored["regime_prob"] = probs
        selected = _select_rows(scored, float(choice["prob_min"]))
        selected["regime_model_name"] = choice["model_name"]
        selected["regime_prob_min"] = float(choice["prob_min"])
        selected_frames.append(selected)
        execution_summary = _summarize(selected, execute_rows)
        walkforward.append(
            {
                "execute_month": execute_month,
                "validation_month": validation_month,
                "model_name": choice["model_name"],
                "prob_min": float(choice["prob_min"]),
                "validation": choice["validation"],
                "execution": execution_summary,
            }
        )

    selected_rows = pd.concat(selected_frames, ignore_index=True) if selected_frames else pd.DataFrame(columns=rows.columns)
    summary = _summarize(selected_rows, rows)
    target_hit = bool(_meets_constraints(summary, args) and float(summary["total_realistic_pct"]) >= float(args.target_realistic_pct))
    selected_output = Path(args.selected_rows_csv)
    selected_output.parent.mkdir(parents=True, exist_ok=True)
    selected_rows.to_csv(selected_output, index=False)
    result = {
        "rows_csv": args.rows_csv,
        "months": months,
        "config": vars(args),
        "summary": summary,
        "target_hit": target_hit,
        "oracle_positive_per_symbol_month": _oracle_positive_per_symbol_month(rows),
        "walkforward": walkforward,
        "selected_rows_csv": str(selected_output),
        "selected_rows": [
            {
                "execute_month": str(row.get("execute_month", "")),
                "symbol": str(row.get("symbol", "")),
                "top_k": int(row.get("top_k", 0)),
                "row_model_name": str(row.get("model_name", "")),
                "regime_model_name": str(row.get("regime_model_name", "")),
                "regime_prob": float(row.get("regime_prob", 0.0)),
                "trades": int(row.get("trades", 0)),
                "realistic_return_pct": float(row.get("realistic_return_pct", 0.0)),
                "validation_topk_return_over_dd": float(row.get("validation_topk_return_over_dd", 0.0)),
                "validation_topk_sizing_markov_score_mean": float(
                    row.get("validation_topk_sizing_markov_score_mean", 0.0)
                ),
            }
            for _, row in selected_rows.iterrows()
        ],
    }
    output_path = Path(args.output_json)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(json.dumps(result, indent=2, ensure_ascii=False, default=str), encoding="utf-8")
    markdown_path = Path(args.markdown)
    markdown_path.parent.mkdir(parents=True, exist_ok=True)
    _write_markdown(result, markdown_path)
    print(json.dumps({"output_json": str(output_path), "markdown": str(markdown_path), "summary": summary, "target_hit": target_hit}, indent=2))


if __name__ == "__main__":
    main()
