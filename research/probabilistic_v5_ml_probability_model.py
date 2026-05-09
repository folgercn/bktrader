#!/usr/bin/env python3
"""Probabilistic V5 ML probability model sweep.

Research-only. Trains multiple probability models on post-touch event features,
selects the model/threshold on a validation slice, and writes scored events
with `quality_pass`, `prob_success`, `prob_ev_atr`, and optional dynamic
`model_notional_share` for the execution runner.
"""

from __future__ import annotations

import argparse
import json
from pathlib import Path

import numpy as np
import pandas as pd
from sklearn.base import clone
from sklearn.ensemble import ExtraTreesClassifier, GradientBoostingClassifier, RandomForestClassifier
from sklearn.impute import SimpleImputer
from sklearn.linear_model import LogisticRegression
from sklearn.metrics import brier_score_loss, log_loss, roc_auc_score
from sklearn.pipeline import Pipeline
from sklearn.preprocessing import StandardScaler
from sklearn.svm import SVC

from probabilistic_v4_quality_model import _summarize_subset


BASE_FEATURES = [
    "signal_atr_percentile",
    "roundtrip_cost_atr",
    "original_roundtrip_cost_atr",
    "prev1_body_atr",
    "prev1_range_atr",
    "prev1_close_pos_side",
    "prev_sma5_gap_atr",
    "prev_sma5_slope_atr",
    "level_to_prev_close_atr",
    "level_to_signal_open_atr",
    "touch_extension_atr",
    "flow_ratio_60s",
    "flow_ratio_120s",
    "speed_15s_atr",
    "speed_60s_atr",
    "speed_300s_atr",
    "eff_60s",
    "eff_300s",
    "close_pos_60s",
    "close_pos_300s",
]


def _as_utc(value: str) -> pd.Timestamp:
    ts = pd.Timestamp(value)
    if ts.tzinfo is None:
        return ts.tz_localize("UTC")
    return ts.tz_convert("UTC")


def _time_split(events: pd.DataFrame, args: argparse.Namespace) -> tuple[pd.Index, pd.Index, pd.Index, str]:
    ordered = events.sort_values("touch_time")
    if args.train_end:
        train_end = _as_utc(args.train_end)
        train_idx = ordered[ordered["touch_time"] <= train_end].index
        if args.validation_end:
            validation_end = _as_utc(args.validation_end)
            validation_idx = ordered[(ordered["touch_time"] > train_end) & (ordered["touch_time"] <= validation_end)].index
            test_idx = ordered[ordered["touch_time"] > validation_end].index
            return train_idx, validation_idx, test_idx, (
                f"train<= {train_end.isoformat()}, validation<= {validation_end.isoformat()}, "
                f"test> {validation_end.isoformat()}"
            )
        validation_idx = ordered[ordered["touch_time"] > train_end].index
        return train_idx, validation_idx, validation_idx, f"train<= {train_end.isoformat()}, validation/test> {train_end.isoformat()}"

    train_end_pos = max(1, min(len(ordered) - 2, int(len(ordered) * float(args.train_ratio))))
    validation_end_pos = max(train_end_pos + 1, min(len(ordered) - 1, int(len(ordered) * (float(args.train_ratio) + float(args.validation_ratio)))))
    train_idx = ordered.iloc[:train_end_pos].index
    validation_idx = ordered.iloc[train_end_pos:validation_end_pos].index
    test_idx = ordered.iloc[validation_end_pos:].index
    return train_idx, validation_idx, test_idx, (
        f"train_ratio={args.train_ratio}, validation_ratio={args.validation_ratio}, "
        f"train_end={ordered.iloc[train_end_pos - 1]['touch_time'].isoformat()}, "
        f"validation_end={ordered.iloc[validation_end_pos - 1]['touch_time'].isoformat()}"
    )


def _boolish(series: pd.Series) -> pd.Series:
    if series.dtype == bool:
        return series.astype("float64")
    return series.astype(str).str.lower().isin({"true", "1", "yes"}).astype("float64")


def _parse_state_seq(raw: object) -> list[int]:
    if not isinstance(raw, str):
        return []
    return [int(ch) for ch in raw.strip() if ch in {"0", "1", "2", "3"}]


def _window_sequence(raw: object, seconds: int) -> list[int]:
    seq = _parse_state_seq(raw)
    if int(seconds) <= 0:
        return seq
    return seq[-int(seconds) :]


def _sigmoid(value: float) -> float:
    return float(1.0 / (1.0 + np.exp(-float(np.clip(value, -40.0, 40.0)))))


def _state_features(events: pd.DataFrame, windows: list[int]) -> pd.DataFrame:
    rows = []
    usable_windows = sorted({int(window) for window in windows if int(window) > 0})
    for raw in events.get("state_seq_60s", pd.Series("", index=events.index)):
        row = {}
        for window in usable_windows:
            seq = _window_sequence(raw, window)
            counts = [0.0, 0.0, 0.0, 0.0]
            transitions = np.zeros((4, 4), dtype="float64")
            for value in seq:
                counts[value] += 1.0
            for left, right in zip(seq[:-1], seq[1:]):
                transitions[left, right] += 1.0
            total = float(len(seq)) or 1.0
            transition_total = float(max(1, len(seq) - 1))
            probs = np.array(counts, dtype="float64") / total
            entropy = float(-(probs[probs > 0] * np.log(probs[probs > 0])).sum())
            suffix = f"_{window}s"
            for idx in range(4):
                row[f"state_frac_{idx}{suffix}"] = counts[idx] / total
            row[f"state_entropy{suffix}"] = entropy
            row[f"state_last{suffix}"] = float(seq[-1]) if seq else 0.0
            for left in range(4):
                for right in range(4):
                    row[f"trans_{left}{right}{suffix}"] = float(transitions[left, right]) / transition_total

            # Keep the original 60s feature names for continuity with earlier V5 screens.
            if window == 60:
                for idx in range(4):
                    row[f"state_frac_{idx}"] = counts[idx] / total
                row["state_entropy"] = entropy
                row["state_last"] = float(seq[-1]) if seq else 0.0
                for left in range(4):
                    for right in range(4):
                        row[f"trans_{left}{right}"] = float(transitions[left, right]) / transition_total
        rows.append(row)
    return pd.DataFrame(rows, index=events.index)


def _feature_frame(events: pd.DataFrame, args: argparse.Namespace) -> pd.DataFrame:
    features = pd.DataFrame(index=events.index)
    for col in BASE_FEATURES:
        if col in events.columns:
            features[col] = pd.to_numeric(events[col], errors="coerce")

    if "side" in events.columns and "atr" in events.columns:
        side_sign = pd.Series(np.where(events["side"].astype(str) == "long", 1.0, -1.0), index=events.index)
        atr = pd.to_numeric(events["atr"], errors="coerce").replace(0.0, np.nan)
        numeric = {
            col: pd.to_numeric(events[col], errors="coerce")
            for col in [
                "signal_open",
                "level",
                "touch_close",
                "touch_high_so_far",
                "touch_low_so_far",
            ]
            if col in events.columns
        }
        if {"signal_open", "touch_close"}.issubset(numeric):
            features["touch_body_so_far_atr"] = (numeric["touch_close"] - numeric["signal_open"]) * side_sign / atr
        if {"touch_high_so_far", "touch_low_so_far"}.issubset(numeric):
            touch_range = (numeric["touch_high_so_far"] - numeric["touch_low_so_far"]).replace(0.0, np.nan)
            features["touch_range_so_far_atr"] = touch_range / atr
            if "touch_close" in numeric:
                raw_touch_pos = (numeric["touch_close"] - numeric["touch_low_so_far"]) / touch_range
                features["touch_close_pos_so_far"] = raw_touch_pos
                features["touch_close_pos_so_far_side"] = np.where(
                    events["side"].astype(str) == "long",
                    raw_touch_pos,
                    1.0 - raw_touch_pos,
                )
        if {"level", "signal_open"}.issubset(numeric):
            features["level_to_signal_open_atr"] = (numeric["signal_open"] - numeric["level"]) * side_sign / atr
        if {"level", "touch_close"}.issubset(numeric):
            features["touch_close_to_level_atr"] = (numeric["touch_close"] - numeric["level"]) * side_sign / atr

    if {"touch_time", "signal_start", "signal_end"}.issubset(events.columns):
        touch_time = pd.to_datetime(events["touch_time"], utc=True, errors="coerce")
        signal_start = pd.to_datetime(events["signal_start"], utc=True, errors="coerce")
        signal_end = pd.to_datetime(events["signal_end"], utc=True, errors="coerce")
        signal_seconds = (signal_end - signal_start).dt.total_seconds().replace(0.0, np.nan)
        touch_seconds = (touch_time - signal_start).dt.total_seconds()
        features["touch_progress"] = (touch_seconds / signal_seconds).clip(0.0, 1.0)
        features["touch_seconds_from_signal_start"] = touch_seconds.clip(lower=0.0)
        hour = touch_time.dt.hour.astype("float64")
        dow = touch_time.dt.dayofweek.astype("float64")
        features["touch_hour_sin"] = np.sin(2.0 * np.pi * hour / 24.0)
        features["touch_hour_cos"] = np.cos(2.0 * np.pi * hour / 24.0)
        features["touch_dow_sin"] = np.sin(2.0 * np.pi * dow / 7.0)
        features["touch_dow_cos"] = np.cos(2.0 * np.pi * dow / 7.0)

    for seconds in [5, 15, 30, 60]:
        if seconds <= int(args.feature_horizon_seconds):
            col = f"dwell_{seconds}s_pass"
            if col in events.columns:
                features[col] = _boolish(events[col])
            close_col = f"dwell_{seconds}s_close"
            if close_col in events.columns:
                features[f"dwell_{seconds}s_extension_atr"] = (
                    (pd.to_numeric(events[close_col], errors="coerce") - pd.to_numeric(events["level"], errors="coerce"))
                    * np.where(events["side"].astype(str) == "long", 1.0, -1.0)
                    / pd.to_numeric(events["atr"], errors="coerce")
                )
        pullback_col = f"pullback_{seconds}s_atr"
        if seconds <= int(args.feature_horizon_seconds) and pullback_col in events.columns:
            features[pullback_col] = pd.to_numeric(events[pullback_col], errors="coerce")

    for col in ["symbol", "side", "shape"]:
        if col in events.columns:
            dummies = pd.get_dummies(events[col].astype(str), prefix=col, dtype="float64")
            features = pd.concat([features, dummies], axis=1)
    features = pd.concat([features, _state_features(events, args.markov_windows)], axis=1)
    return features.replace([np.inf, -np.inf], np.nan)


def _build_models(args: argparse.Namespace) -> dict[str, object]:
    requested = set(args.models)
    models: dict[str, object] = {}
    if "logistic" in requested:
        models["logistic"] = Pipeline(
            [
                ("imputer", SimpleImputer(strategy="median")),
                ("scaler", StandardScaler()),
                ("model", LogisticRegression(max_iter=2000, class_weight="balanced", C=0.8)),
            ]
        )
    if "random_forest" in requested:
        models["random_forest"] = Pipeline(
            [
                ("imputer", SimpleImputer(strategy="median")),
                (
                    "model",
                    RandomForestClassifier(
                        n_estimators=int(args.n_estimators),
                        max_depth=6,
                        min_samples_leaf=8,
                        class_weight="balanced_subsample",
                        random_state=42,
                        n_jobs=-1,
                    ),
                ),
            ]
        )
    if "extra_trees" in requested:
        models["extra_trees"] = Pipeline(
            [
                ("imputer", SimpleImputer(strategy="median")),
                (
                    "model",
                    ExtraTreesClassifier(
                        n_estimators=int(args.n_estimators),
                        max_depth=8,
                        min_samples_leaf=6,
                        class_weight="balanced",
                        random_state=43,
                        n_jobs=-1,
                    ),
                ),
            ]
        )
    if "gradient_boosting" in requested:
        models["gradient_boosting"] = Pipeline(
            [
                ("imputer", SimpleImputer(strategy="median")),
                (
                    "model",
                    GradientBoostingClassifier(
                        n_estimators=160,
                        learning_rate=0.04,
                        max_depth=2,
                        min_samples_leaf=8,
                        random_state=44,
                    ),
                ),
            ]
        )
    if "svm_rbf" in requested:
        models["svm_rbf"] = Pipeline(
            [
                ("imputer", SimpleImputer(strategy="median")),
                ("scaler", StandardScaler()),
                ("model", SVC(C=1.0, gamma="scale", probability=True, class_weight="balanced", random_state=45)),
            ]
        )
    return models


def _payoff_summary(train: pd.DataFrame) -> dict:
    success = train[train["outcome"] == "continuation"]
    non_success = train[train["outcome"] != "continuation"]
    return {
        "success_first_edge_atr": float(success["first_edge_atr"].mean()) if not success.empty else 0.5,
        "non_success_first_edge_atr": float(non_success["first_edge_atr"].mean()) if not non_success.empty else -0.2,
    }


def _transition_matrix(sequences: list[list[int]], alpha: float = 1.0) -> np.ndarray:
    matrix = np.ones((4, 4), dtype="float64") * float(alpha)
    for seq in sequences:
        if len(seq) < 2:
            continue
        for left, right in zip(seq[:-1], seq[1:]):
            matrix[int(left), int(right)] += 1.0
    matrix /= matrix.sum(axis=1, keepdims=True)
    return matrix


def _score_sequence(seq: list[int], win_matrix: np.ndarray, loss_matrix: np.ndarray) -> float:
    if len(seq) < 2:
        return 0.0
    score = 0.0
    for left, right in zip(seq[:-1], seq[1:]):
        score += float(np.log(win_matrix[int(left), int(right)] / loss_matrix[int(left), int(right)]))
    return score


def _markov_features(events: pd.DataFrame, train_idx: pd.Index, args: argparse.Namespace) -> tuple[pd.DataFrame, dict]:
    features = pd.DataFrame(index=events.index)
    summary: dict[str, dict] = {}
    train = events.loc[train_idx]
    success_train = train[train["outcome"] == "continuation"]
    fail_train = train[train["outcome"] == "fail"]
    llr_cols: list[str] = []
    for window in sorted({int(value) for value in args.markov_windows if int(value) > 0}):
        success_sequences = [_window_sequence(raw, window) for raw in success_train.get("state_seq_60s", [])]
        fail_sequences = [_window_sequence(raw, window) for raw in fail_train.get("state_seq_60s", [])]
        success_sequences = [seq for seq in success_sequences if len(seq) >= 2]
        fail_sequences = [seq for seq in fail_sequences if len(seq) >= 2]
        llr_col = f"markov_llr_{window}s"
        prob_col = f"markov_prob_{window}s"
        llr_cols.append(llr_col)
        if len(success_sequences) < int(args.markov_min_sequences) or len(fail_sequences) < int(args.markov_min_sequences):
            features[llr_col] = 0.0
            features[prob_col] = 0.5
            summary[str(window)] = {
                "status": "insufficient_sequences",
                "success_sequences": int(len(success_sequences)),
                "fail_sequences": int(len(fail_sequences)),
            }
            continue

        win_matrix = _transition_matrix(success_sequences)
        loss_matrix = _transition_matrix(fail_sequences)
        scores = []
        for raw in events.get("state_seq_60s", pd.Series("", index=events.index)):
            seq = _window_sequence(raw, window)
            raw_score = _score_sequence(seq, win_matrix, loss_matrix)
            scores.append(raw_score / float(max(1, len(seq) - 1)))
        features[llr_col] = scores
        features[prob_col] = [_sigmoid(score) for score in scores]
        summary[str(window)] = {
            "status": "trained",
            "success_sequences": int(len(success_sequences)),
            "fail_sequences": int(len(fail_sequences)),
            "win_matrix": win_matrix.round(6).tolist(),
            "loss_matrix": loss_matrix.round(6).tolist(),
        }

    if llr_cols:
        features["markov_combo_score"] = features[llr_cols].mean(axis=1)
        features["markov_combo_prob"] = features["markov_combo_score"].map(_sigmoid)
    else:
        features["markov_combo_score"] = 0.0
        features["markov_combo_prob"] = 0.5
    summary["combo"] = {"llr_columns": llr_cols}
    return features, summary


def _add_ev(events: pd.DataFrame, probabilities: np.ndarray, payoff: dict) -> np.ndarray:
    cost = pd.to_numeric(events["roundtrip_cost_atr"], errors="coerce").fillna(0.0).to_numpy(dtype="float64")
    return (
        probabilities * float(payoff["success_first_edge_atr"])
        + (1.0 - probabilities) * float(payoff["non_success_first_edge_atr"])
        - cost
    )


def _summary(frame: pd.DataFrame) -> dict:
    base = _summarize_subset(frame)
    if frame.empty:
        base.update(
            {
                "avg_prob_success": 0.0,
                "avg_prob_ev_atr": 0.0,
                "avg_prob_initial_sl": 0.0,
                "sum_net_first_edge_atr": 0.0,
            }
        )
        return base
    base.update(
        {
            "avg_prob_success": round(float(frame["prob_success"].mean()), 6),
            "avg_prob_ev_atr": round(float(frame["prob_ev_atr"].mean()), 6),
            "sum_net_first_edge_atr": round(float(frame["net_first_edge_atr"].sum()), 6),
        }
    )
    if "prob_initial_sl" in frame.columns:
        base["avg_prob_initial_sl"] = round(float(frame["prob_initial_sl"].mean()), 6)
    return base


def _scored_summary(events: pd.DataFrame, idx: pd.Index, prob: np.ndarray, payoff: dict) -> dict:
    frame = events.loc[idx].copy()
    if frame.empty:
        frame["prob_success"] = []
        frame["prob_ev_atr"] = []
        return _summary(frame)
    prob_series = pd.Series(prob, index=events.index)
    frame["prob_success"] = prob_series.loc[idx].to_numpy(dtype="float64")
    frame["prob_ev_atr"] = _add_ev(frame, frame["prob_success"].to_numpy(dtype="float64"), payoff)
    return _summary(frame)


def _initial_sl_target(events: pd.DataFrame) -> pd.Series:
    if "execution_exit_reason" not in events.columns:
        return pd.Series(0, index=events.index, dtype="int8")
    return events["execution_exit_reason"].astype(str).eq("InitialSL").astype("int8")


def _constant_probability(y_train: np.ndarray, length: int) -> np.ndarray:
    if len(y_train) == 0:
        return np.zeros(length, dtype="float64")
    return np.full(length, float(np.mean(y_train)), dtype="float64")


def _fit_predict_probability(model: object, x_train: pd.DataFrame, y_train: np.ndarray, x_all: pd.DataFrame) -> np.ndarray:
    if len(np.unique(y_train)) < 2:
        return _constant_probability(y_train, len(x_all))
    fitted = clone(model)
    fitted.fit(x_train, y_train)
    return fitted.predict_proba(x_all)[:, 1]


def _candidate_rules(args: argparse.Namespace) -> list[dict]:
    return [
        {"prob_min": float(prob_min), "ev_atr_min": float(ev_min), "sl_prob_max": float(sl_max)}
        for prob_min in args.prob_mins
        for ev_min in args.ev_atr_mins
        for sl_max in args.sl_prob_maxes
    ]


def _reference_scaled_score(values: pd.Series, reference: pd.Series, *, low: float | None = None) -> pd.Series:
    values = pd.to_numeric(values, errors="coerce").astype("float64")
    reference = pd.to_numeric(reference, errors="coerce").replace([np.inf, -np.inf], np.nan).dropna()
    if reference.empty:
        return pd.Series(0.5, index=values.index)
    ref_low = float(reference.quantile(0.10)) if low is None else float(low)
    ref_high = float(reference.quantile(0.90))
    if not np.isfinite(ref_high) or ref_high <= ref_low:
        ref_high = float(reference.max())
    if not np.isfinite(ref_low) or not np.isfinite(ref_high) or ref_high <= ref_low:
        return pd.Series(0.5, index=values.index)
    return ((values - ref_low) / max(1e-9, ref_high - ref_low)).clip(0.0, 1.0).fillna(0.5)


def _sizing_scores(frame: pd.DataFrame, reference: pd.DataFrame, rule: dict, args: argparse.Namespace) -> pd.DataFrame:
    result = pd.DataFrame(index=frame.index)
    result["sizing_ev_score"] = _reference_scaled_score(
        frame["prob_ev_atr"],
        reference["prob_ev_atr"],
        low=float(rule.get("ev_atr_min", float(args.ev_atr_mins[0]))),
    )
    result["sizing_prob_score"] = _reference_scaled_score(
        frame["prob_success"],
        reference["prob_success"],
        low=float(rule.get("prob_min", 0.0)),
    )
    if "markov_combo_score" in frame.columns and "markov_combo_score" in reference.columns:
        result["sizing_markov_score"] = _reference_scaled_score(frame["markov_combo_score"], reference["markov_combo_score"])
    else:
        result["sizing_markov_score"] = 0.5
    if "prob_initial_sl" in frame.columns:
        result["sizing_sl_score"] = (1.0 - pd.to_numeric(frame["prob_initial_sl"], errors="coerce")).clip(0.0, 1.0).fillna(0.5)
    else:
        result["sizing_sl_score"] = 0.5

    if args.sizing_mode == "ev_rank":
        result["sizing_score"] = result["sizing_ev_score"]
        return result

    weights = {
        "sizing_ev_score": max(0.0, float(args.sizing_ev_weight)),
        "sizing_prob_score": max(0.0, float(args.sizing_prob_weight)),
        "sizing_markov_score": max(0.0, float(args.sizing_markov_weight)),
        "sizing_sl_score": max(0.0, float(args.sizing_sl_weight)),
    }
    total_weight = sum(weights.values())
    if total_weight <= 0:
        result["sizing_score"] = result["sizing_ev_score"]
        return result
    result["sizing_score"] = sum(result[col] * weight for col, weight in weights.items()) / total_weight
    return result


def _select_model(
    events: pd.DataFrame,
    validation_idx: pd.Index,
    model_scores: dict[str, np.ndarray],
    sl_scores: dict[str, np.ndarray],
    payoff: dict,
    args: argparse.Namespace,
) -> tuple[dict, list[dict]]:
    candidates: list[dict] = []
    for model_name, prob in model_scores.items():
        scored = events.copy()
        scored["prob_success"] = prob
        scored["prob_ev_atr"] = _add_ev(events, prob, payoff)
        scored["prob_initial_sl"] = sl_scores.get(model_name, np.zeros(len(events), dtype="float64"))
        validation = scored.loc[validation_idx]
        for rule in _candidate_rules(args):
            mask = (validation["prob_success"] >= rule["prob_min"]) & (validation["prob_ev_atr"] >= rule["ev_atr_min"])
            mask &= validation["prob_initial_sl"] <= rule["sl_prob_max"]
            subset = validation[mask]
            summary = _summary(subset)
            if summary["events"] < int(args.min_events):
                continue
            candidate = {"model_name": model_name, "rule": rule, "validation": summary}
            if args.objective == "avg_net_edge":
                candidate["objective_score"] = summary["avg_net_first_edge_atr"]
            elif args.objective == "sum_prob_ev":
                candidate["objective_score"] = summary["avg_prob_ev_atr"] * np.sqrt(float(summary["events"]))
            elif args.objective == "sum_sized_net_edge":
                sizing = _sizing_scores(subset, subset, rule, args)
                share = float(args.min_share) + sizing["sizing_score"] * (float(args.max_share) - float(args.min_share))
                candidate["objective_score"] = float((subset["net_first_edge_atr"].astype("float64") * share).sum())
            else:
                candidate["objective_score"] = summary["sum_net_first_edge_atr"]
            candidates.append(candidate)
    if not candidates:
        fallback_name = next(iter(model_scores.keys()))
        fallback = {
            "model_name": fallback_name,
            "rule": {"prob_min": 0.0, "ev_atr_min": -999.0, "sl_prob_max": 1.0},
            "validation": _scored_summary(events, validation_idx, model_scores[fallback_name], payoff),
            "objective_score": -999.0,
        }
        return fallback, [fallback]
    candidates.sort(
        key=lambda item: (
            item["objective_score"],
            item["validation"]["avg_net_first_edge_atr"],
            item["validation"]["events"],
        ),
        reverse=True,
    )
    return candidates[0], candidates


def _model_metrics(y_true: np.ndarray, prob: np.ndarray) -> dict:
    if len(y_true) == 0:
        return {"brier": 0.0, "log_loss": 0.0, "auc": 0.0}
    metrics = {
        "brier": round(float(brier_score_loss(y_true, prob)), 8),
        "log_loss": round(float(log_loss(y_true, np.clip(prob, 1e-6, 1.0 - 1e-6), labels=[0, 1])), 8),
    }
    if len(np.unique(y_true)) > 1:
        metrics["auc"] = round(float(roc_auc_score(y_true, prob)), 8)
    else:
        metrics["auc"] = 0.0
    return metrics


def _split_range(events: pd.DataFrame, idx: pd.Index) -> dict:
    frame = events.loc[idx]
    if frame.empty:
        return {"rows": 0, "start": "", "end": ""}
    return {
        "rows": int(len(frame)),
        "start": frame["touch_time"].min().isoformat(),
        "end": frame["touch_time"].max().isoformat(),
    }


def _assign_sizing(
    scored: pd.DataFrame,
    selected_idx: pd.Index,
    reference_idx: pd.Index,
    selected_rule: dict,
    args: argparse.Namespace,
) -> pd.DataFrame:
    result = scored.copy()
    result["model_notional_share"] = 0.0
    result["sizing_score"] = 0.0
    result["sizing_ev_score"] = 0.0
    result["sizing_prob_score"] = 0.0
    result["sizing_markov_score"] = 0.0
    result["sizing_sl_score"] = 0.0
    selected = result.loc[selected_idx]
    if selected.empty:
        return result
    reference = result.loc[reference_idx.intersection(selected_idx)]
    if reference.empty:
        reference = selected
    sizing = _sizing_scores(selected, reference, selected_rule, args)
    share = float(args.min_share) + sizing["sizing_score"] * (float(args.max_share) - float(args.min_share))
    for col in ["sizing_score", "sizing_ev_score", "sizing_prob_score", "sizing_markov_score", "sizing_sl_score"]:
        result.loc[selected_idx, col] = sizing[col]
    result.loc[selected_idx, "model_notional_share"] = share.clip(float(args.min_share), float(args.max_share))
    return result


def _write_markdown(summary: dict, path: Path) -> None:
    selected = summary["selected"]
    lines = [
        "# Probabilistic V5 ML Probability Sweep",
        "",
        "范围：仅限 `research`。本文件比较多种 ML 概率模型，并输出给执行层消费的 `quality_pass` 与动态仓位。",
        "",
        f"- Split: `{summary['split']}`",
        f"- Selected model: `{selected['model_name']}`",
        f"- Selected rule: `prob_min={selected['rule']['prob_min']}`, `ev_atr_min={selected['rule']['ev_atr_min']}`, "
        f"`sl_prob_max={selected['rule']['sl_prob_max']}`",
        f"- Markov windows: `{summary['markov_windows']}`",
        f"- Sizing: `mode={summary['sizing']['mode']}`, `min_share={summary['sizing']['min_share']}`, `max_share={summary['sizing']['max_share']}`",
        "",
        "## Validation",
        "",
        "| Events | Success | Net Edge ATR | Sum Net Edge ATR | Avg Prob | Avg Prob EV ATR | Avg InitialSL Prob |",
        "|---:|---:|---:|---:|---:|---:|---:|",
    ]
    validation = selected["validation"]
    lines.append(
        f"| {validation['events']} | {validation['success_rate']:.4f}% | {validation['avg_net_first_edge_atr']:.6f} | "
        f"{validation['sum_net_first_edge_atr']:.6f} | {validation['avg_prob_success']:.6f} | "
        f"{validation['avg_prob_ev_atr']:.6f} | {validation.get('avg_prob_initial_sl', 0.0):.6f} |"
    )
    lines.extend(["", "## Model Leaderboard", "", "| Rank | Model | Prob Min | EV Min | SL Max | Events | Success | Net Edge ATR | Sum Net Edge ATR | Score |", "|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|"])
    for idx, item in enumerate(summary["leaderboard"][:20], 1):
        v = item["validation"]
        rule = item["rule"]
        lines.append(
            f"| {idx} | `{item['model_name']}` | {rule['prob_min']} | {rule['ev_atr_min']} | "
            f"{rule.get('sl_prob_max', 1.0)} | {v['events']} | "
            f"{v['success_rate']:.4f}% | {v['avg_net_first_edge_atr']:.6f} | {v['sum_net_first_edge_atr']:.6f} | "
            f"{item['objective_score']:.6f} |"
        )
    lines.extend(["", "## Metrics", "", "| Model | Brier | Log Loss | AUC | SL Brier | SL Log Loss | SL AUC |", "|---|---:|---:|---:|---:|---:|---:|"])
    for model_name, metrics in summary["model_metrics"].items():
        sl_metrics = summary.get("sl_model_metrics", {}).get(model_name, {})
        lines.append(
            f"| `{model_name}` | {metrics['brier']:.8f} | {metrics['log_loss']:.8f} | {metrics['auc']:.8f} | "
            f"{sl_metrics.get('brier', 0.0):.8f} | {sl_metrics.get('log_loss', 0.0):.8f} | "
            f"{sl_metrics.get('auc', 0.0):.8f} |"
        )
    path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Train Probabilistic V5 ML probability model sweep")
    parser.add_argument("--events-csv", default="research/probabilistic_v4_events.csv")
    parser.add_argument("--scored-csv", default="research/probabilistic_v5_events_scored.csv")
    parser.add_argument("--model-json", default="research/probabilistic_v5_ml_model.json")
    parser.add_argument("--markdown", default="research/20260509_probabilistic_v5_ml_model.md")
    parser.add_argument("--train-end", default="")
    parser.add_argument("--validation-end", default="")
    parser.add_argument("--train-ratio", type=float, default=0.60)
    parser.add_argument("--validation-ratio", type=float, default=0.20)
    parser.add_argument("--feature-horizon-seconds", type=int, default=5)
    parser.add_argument("--markov-windows", nargs="+", type=int, default=[5, 15, 30, 60])
    parser.add_argument("--markov-min-sequences", type=int, default=5)
    parser.add_argument("--models", nargs="+", default=["logistic", "random_forest", "extra_trees", "gradient_boosting", "svm_rbf"])
    parser.add_argument("--n-estimators", type=int, default=400)
    parser.add_argument("--min-events", type=int, default=30)
    parser.add_argument("--prob-mins", nargs="+", type=float, default=[0.35, 0.40, 0.45, 0.50, 0.55, 0.60, 0.65])
    parser.add_argument("--ev-atr-mins", nargs="+", type=float, default=[-0.10, -0.05, 0.0, 0.05, 0.10, 0.15])
    parser.add_argument("--sl-prob-maxes", nargs="+", type=float, default=[0.35, 0.50, 0.65, 0.80, 1.00])
    parser.add_argument("--objective", choices=["sum_net_edge", "avg_net_edge", "sum_prob_ev", "sum_sized_net_edge"], default="sum_net_edge")
    parser.add_argument("--sizing-mode", choices=["ev_rank", "hybrid_markov"], default="hybrid_markov")
    parser.add_argument("--sizing-ev-weight", type=float, default=0.45)
    parser.add_argument("--sizing-prob-weight", type=float, default=0.25)
    parser.add_argument("--sizing-markov-weight", type=float, default=0.30)
    parser.add_argument("--sizing-sl-weight", type=float, default=0.20)
    parser.add_argument("--min-share", type=float, default=0.20)
    parser.add_argument("--max-share", type=float, default=1.00)
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    events = pd.read_csv(args.events_csv, parse_dates=["touch_time", "signal_start", "signal_end"])
    if events.empty:
        raise SystemExit(f"empty events dataset: {args.events_csv}")
    has_initial_sl_labels = "execution_exit_reason" in events.columns
    if not has_initial_sl_labels:
        args.sl_prob_maxes = [1.0]
    train_idx, validation_idx, test_idx, split = _time_split(events, args)
    features = _feature_frame(events, args)
    markov_features, markov_summary = _markov_features(events, train_idx, args)
    features = pd.concat([features, markov_features], axis=1)
    y = (events["outcome"] == "continuation").astype("int8")
    payoff = _payoff_summary(events.loc[train_idx])

    models = _build_models(args)
    if not models:
        raise SystemExit("no models requested")
    model_scores: dict[str, np.ndarray] = {}
    sl_scores: dict[str, np.ndarray] = {}
    model_metrics: dict[str, dict] = {}
    sl_model_metrics: dict[str, dict] = {}
    x_train = features.loc[train_idx]
    y_train = y.loc[train_idx].to_numpy()
    x_validation = features.loc[validation_idx]
    y_validation = y.loc[validation_idx].to_numpy()
    y_sl = _initial_sl_target(events)
    y_sl_train = y_sl.loc[train_idx].to_numpy()
    y_sl_validation = y_sl.loc[validation_idx].to_numpy()
    for model_name, model in models.items():
        prob_all = _fit_predict_probability(model, x_train, y_train, features)
        prob_validation = pd.Series(prob_all, index=events.index).loc[validation_idx].to_numpy(dtype="float64")
        model_scores[model_name] = prob_all
        model_metrics[model_name] = _model_metrics(y_validation, prob_validation)
        if has_initial_sl_labels:
            sl_prob_all = _fit_predict_probability(model, x_train, y_sl_train, features)
        else:
            sl_prob_all = np.zeros(len(events), dtype="float64")
        sl_prob_validation = pd.Series(sl_prob_all, index=events.index).loc[validation_idx].to_numpy(dtype="float64")
        sl_scores[model_name] = sl_prob_all
        sl_model_metrics[model_name] = _model_metrics(y_sl_validation, sl_prob_validation)

    selected, leaderboard = _select_model(events, validation_idx, model_scores, sl_scores, payoff, args)
    selected_prob = model_scores[selected["model_name"]]
    selected_sl_prob = sl_scores[selected["model_name"]]
    scored = events.copy()
    scored["model_name"] = selected["model_name"]
    scored["prob_success"] = selected_prob
    scored["prob_initial_sl"] = selected_sl_prob
    scored["prob_ev_atr"] = _add_ev(events, selected_prob, payoff)
    for col in markov_features.columns:
        scored[col] = markov_features[col]
    selected_mask = (scored["prob_success"] >= selected["rule"]["prob_min"]) & (
        scored["prob_ev_atr"] >= selected["rule"]["ev_atr_min"]
    )
    selected_mask &= scored["prob_initial_sl"] <= selected["rule"].get("sl_prob_max", 1.0)
    scored["quality_pass"] = selected_mask
    scored["quality_bucket"] = np.where(scored["quality_pass"], "selected", "rejected")
    reference_idx = train_idx.union(validation_idx)
    scored = _assign_sizing(scored, scored[selected_mask].index, reference_idx, selected["rule"], args)

    scored_path = Path(args.scored_csv)
    scored_path.parent.mkdir(parents=True, exist_ok=True)
    scored.to_csv(scored_path, index=False)
    summary = {
        "events_csv": args.events_csv,
        "scored_csv": str(scored_path),
        "split": split,
        "split_ranges": {
            "train": _split_range(events, train_idx),
            "validation": _split_range(events, validation_idx),
            "test": _split_range(events, test_idx),
        },
        "train_rows": int(len(train_idx)),
        "validation_rows": int(len(validation_idx)),
        "test_rows": int(len(test_idx)),
        "feature_horizon_seconds": int(args.feature_horizon_seconds),
        "markov_windows": [int(value) for value in args.markov_windows],
        "features": features.columns.tolist(),
        "markov": markov_summary,
        "payoff": payoff,
        "selected": selected,
        "leaderboard": leaderboard[:50],
        "model_metrics": model_metrics,
        "sl_model_metrics": sl_model_metrics,
        "validation_baseline": _summary(scored.loc[validation_idx]),
        "test_baseline": _summary(scored.loc[test_idx]) if len(test_idx) else {},
        "train_selected": _summary(scored.loc[train_idx][scored.loc[train_idx, "quality_pass"]]),
        "validation_selected": _summary(scored.loc[validation_idx][scored.loc[validation_idx, "quality_pass"]]),
        "test_selected": _summary(scored.loc[test_idx][scored.loc[test_idx, "quality_pass"]]) if len(test_idx) else {},
        "sizing": {
            "mode": args.sizing_mode,
            "min_share": float(args.min_share),
            "max_share": float(args.max_share),
            "ev_weight": float(args.sizing_ev_weight),
            "prob_weight": float(args.sizing_prob_weight),
            "markov_weight": float(args.sizing_markov_weight),
            "sl_weight": float(args.sizing_sl_weight),
            "reference": "train+validation selected events",
        },
        "selected_rule": {
            "model_name": selected["model_name"],
            "prob_min": selected["rule"]["prob_min"],
            "ev_atr_min": selected["rule"]["ev_atr_min"],
            "sl_prob_max": selected["rule"].get("sl_prob_max", 1.0),
            "objective": args.objective,
        },
    }
    model_path = Path(args.model_json)
    model_path.parent.mkdir(parents=True, exist_ok=True)
    model_path.write_text(json.dumps(summary, indent=2, ensure_ascii=False, default=str), encoding="utf-8")
    _write_markdown(summary, Path(args.markdown))
    print(
        json.dumps(
            {
                "model_json": str(model_path),
                "scored_csv": str(scored_path),
                "selected": summary["selected_rule"],
            },
            indent=2,
            ensure_ascii=False,
        ),
        flush=True,
    )


if __name__ == "__main__":
    main()
