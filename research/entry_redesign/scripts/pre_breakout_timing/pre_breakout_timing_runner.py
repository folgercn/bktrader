#!/usr/bin/env python3
"""
pre_breakout_timing_runner — 主运行器

串联 Pre-Breakout Timing Classifier 的完整流程：
1. 加载数据（V6 gate events, 1s bar cache, time-split）
2. 多延迟执行模拟（D=0/5/10/15/pullback）
3. 标签生成（Optimal_Delay_Label）
4. 特征提取与填充
5. 分类器训练（Rule-based, DT, RF, LR）→ LOOCV 选最优
6. 回调参数优化
7. Test set 评估
8. 稳健性验证（Bootstrap CI, 分 symbol, overfitting, regime 稳定性）
9. 报告生成

Usage:
    cd research/entry_redesign/scripts
    python -m pre_breakout_timing.pre_breakout_timing_runner
"""

from __future__ import annotations

import sys
import time
from collections import Counter
from pathlib import Path

import pandas as pd

# ---------------------------------------------------------------------------
# Path setup
# ---------------------------------------------------------------------------

SCRIPTS_DIR = Path(__file__).resolve().parent.parent
if str(SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPTS_DIR))

# ---------------------------------------------------------------------------
# Imports — data layer
# ---------------------------------------------------------------------------

from pre_breakout_timing.data_layer import (
    load_bars_cache,
    load_v6_gate_events,
    time_split_events,
)

# ---------------------------------------------------------------------------
# Imports — delay simulation & labels
# ---------------------------------------------------------------------------

from pre_breakout_timing.delay_simulator import (
    DELAY_VALUES,
    DelayResult,
    compute_optimal_labels,
    simulate_all_delays,
)

# ---------------------------------------------------------------------------
# Imports — feature extraction (used in step 4+)
# ---------------------------------------------------------------------------

from pre_breakout_timing.feature_extractor import (
    extract_features,
    feature_statistics,
    impute_features,
)

# ---------------------------------------------------------------------------
# Imports — classifiers (used in step 5+)
# ---------------------------------------------------------------------------

from pre_breakout_timing.timing_classifier import (
    ClassifierResult,
    train_decision_tree,
    train_logistic_regression,
    train_random_forest,
    train_rule_based_classifier,
)

# ---------------------------------------------------------------------------
# Imports — pullback strategy (used in step 6+)
# ---------------------------------------------------------------------------

from pre_breakout_timing.pullback_strategy import (
    PullbackConfig,
    optimize_pullback_params,
)

# ---------------------------------------------------------------------------
# Imports — report generation (used in step 9)
# ---------------------------------------------------------------------------

from pre_breakout_timing.report_generator import generate_report

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

OUTPUT_DIR = SCRIPTS_DIR / "output" / "pre_breakout_timing"
OUTPUT_DIR.mkdir(parents=True, exist_ok=True)

INITIAL_BALANCE = 100_000.0
NOTIONAL_SHARE = 0.26

# Default pullback params (from design doc)
DEFAULT_PULLBACK_PARAMS = {
    "pullback_target_atr": 0.05,
    "pullback_window_seconds": 60,
    "start_offset_seconds": 5,
}

# Known baseline for validation
KNOWN_D5_CALENDAR_SUM = 2.74  # %


# ---------------------------------------------------------------------------
# Helper: silo-based calendar sum
# ---------------------------------------------------------------------------


def compute_calendar_sum_from_results(
    results: list[DelayResult],
    events: pd.DataFrame,
) -> float:
    """计算 silo-based calendar sum (%).

    每个 (symbol, month) 独立从 100k 开始，各 silo return 简单加和。
    使用 notional_share=0.26（与 execute_trade 默认值一致）。
    """
    # Build event_id -> symbol mapping
    event_symbol_map: dict[str, str] = {}
    for _, row in events.iterrows():
        eid = str(row.get("event_id", ""))
        event_symbol_map[eid] = str(row["symbol"])

    # Group trades by (symbol, month)
    silos: dict[str, list[DelayResult]] = {}
    for r in results:
        if not r.traded or r.pnl_pct is None:
            continue
        symbol = event_symbol_map.get(r.event_id, "unknown")
        entry_time = pd.Timestamp(r.entry_time)
        month_key = f"{symbol}_{entry_time.strftime('%Y-%m')}"
        if month_key not in silos:
            silos[month_key] = []
        silos[month_key].append(r)

    total_return_pct = 0.0
    for _silo_key, silo_results in silos.items():
        balance = INITIAL_BALANCE
        sorted_results = sorted(silo_results, key=lambda r: r.entry_time)
        for r in sorted_results:
            notional = balance * NOTIONAL_SHARE
            pnl = notional * r.pnl_pct
            balance += pnl
        silo_return = (balance - INITIAL_BALANCE) / INITIAL_BALANCE * 100.0
        total_return_pct += silo_return

    return total_return_pct


# ===========================================================================
# Main
# ===========================================================================


def main():
    """Pre-Breakout Timing Classifier 完整流程。"""

    print("=" * 70)
    print("Pre-Breakout Timing Classifier — Full Pipeline")
    print("=" * 70)

    # ===================================================================
    # Step 1: 加载数据
    # ===================================================================

    print("\n" + "─" * 70)
    print("Step 1: 加载数据")
    print("─" * 70)

    print("\n  [1.1] Loading V6 gate events...")
    t0 = time.time()
    events = load_v6_gate_events()
    # Sort by touch_time and reset index so all downstream code uses
    # a consistent RangeIndex [0..n_events-1].
    events = events.sort_values("touch_time").reset_index(drop=True)
    n_events = len(events)
    print(f"        Loaded {n_events} events in {time.time() - t0:.1f}s")

    print("\n  [1.2] Loading 1s bar cache...")
    t0 = time.time()
    bars_cache = load_bars_cache(events)
    print(f"        Loaded {len(bars_cache)} month caches in {time.time() - t0:.1f}s")

    print("\n  [1.3] Time-split events (60/40)...")
    # time_split_events sorts by touch_time and resets index.
    # Since events is already sorted, train = positions [0..split-1],
    # test = positions [split..n-1]. We use positional split directly
    # to maintain index alignment with the features DataFrame.
    split_idx = int(n_events * 0.6)
    train_events = events.iloc[:split_idx].copy().reset_index(drop=True)
    test_events = events.iloc[split_idx:].copy().reset_index(drop=True)
    print(f"        Train: {len(train_events)} events")
    print(f"        Test:  {len(test_events)} events")

    # ===================================================================
    # Step 2: 多延迟执行模拟
    # ===================================================================

    print("\n" + "─" * 70)
    print("Step 2: 多延迟执行模拟")
    print("─" * 70)

    print(f"\n  Running multi-delay simulation on {n_events} events...")
    print(f"  Delays: D=0s, D=5s, D=10s, D=15s, pullback")
    print(f"  Pullback params: {DEFAULT_PULLBACK_PARAMS}")

    all_delay_results: list[list[DelayResult]] = []
    t0 = time.time()

    for idx, (_, event) in enumerate(events.iterrows()):
        symbol = event["symbol"]
        touch_time = pd.Timestamp(event["touch_time"])
        if touch_time.tzinfo is None:
            touch_time = touch_time.tz_localize("UTC")

        # Get bars for this event's month
        month_key = f"{symbol}_{touch_time.strftime('%Y%m')}"
        second_bars = bars_cache.get(month_key)

        if second_bars is None or second_bars.empty:
            # No data: create empty results for all delays
            event_id = str(event.get("event_id", ""))
            empty_results = []
            for delay in DELAY_VALUES:
                label = f"D{delay}"
                empty_results.append(DelayResult(
                    event_id=event_id,
                    delay_label=label,
                    delay_seconds=delay,
                    entry_time=None,
                    entry_price=None,
                    pnl_pct=None,
                    exit_reason="NoData",
                    exit_time=None,
                    hold_seconds=None,
                    mfe_r=None,
                    mae_r=None,
                    traded=False,
                ))
            empty_results.append(DelayResult(
                event_id=event_id,
                delay_label="pullback",
                delay_seconds=0,
                entry_time=None,
                entry_price=None,
                pnl_pct=None,
                exit_reason="NoData",
                exit_time=None,
                hold_seconds=None,
                mfe_r=None,
                mae_r=None,
                traded=False,
            ))
            all_delay_results.append(empty_results)
            continue

        # Run simulation for all delays
        event_results = simulate_all_delays(
            event, second_bars, pullback_params=DEFAULT_PULLBACK_PARAMS
        )
        all_delay_results.append(event_results)

        # Progress
        if (idx + 1) % 20 == 0 or idx == len(events) - 1:
            elapsed = time.time() - t0
            print(f"    Processed {idx + 1}/{n_events} events ({elapsed:.1f}s)")

    sim_elapsed = time.time() - t0
    print(f"\n  Simulation complete in {sim_elapsed:.1f}s")

    # --- Compute baselines for each delay ---
    print("\n  Computing baseline calendar_sum for each delay...")

    delay_labels = ["D0", "D5", "D10", "D15", "pullback"]
    baselines: dict[str, dict] = {}

    for label in delay_labels:
        label_results = [
            dr
            for event_results in all_delay_results
            for dr in event_results
            if dr.delay_label == label
        ]
        cal_sum = compute_calendar_sum_from_results(label_results, events)
        traded = [dr for dr in label_results if dr.traded]
        wins = [dr for dr in traded if dr.pnl_pct is not None and dr.pnl_pct > 0]
        win_rate = len(wins) / len(traded) * 100.0 if traded else 0.0
        avg_pnl = (
            sum(dr.pnl_pct for dr in traded if dr.pnl_pct is not None) / len(traded)
            if traded
            else 0.0
        )

        baselines[label] = {
            "calendar_sum": cal_sum,
            "trade_count": len(traded),
            "win_rate": win_rate,
            "avg_pnl": avg_pnl,
        }
        print(f"    {label:>10s}: calendar_sum={cal_sum:+.4f}%, "
              f"trades={len(traded)}, win_rate={win_rate:.1f}%")

    # --- Save delay_pnl_matrix.csv ---
    print("\n  Saving delay_pnl_matrix.csv...")
    rows = []
    for event_idx, event_results in enumerate(all_delay_results):
        event_row = events.iloc[event_idx]
        for dr in event_results:
            rows.append({
                "event_id": dr.event_id,
                "symbol": event_row["symbol"],
                "side": event_row["side"],
                "touch_time": event_row["touch_time"],
                "delay_label": dr.delay_label,
                "delay_seconds": dr.delay_seconds,
                "entry_time": dr.entry_time,
                "entry_price": dr.entry_price,
                "pnl_pct": dr.pnl_pct,
                "exit_reason": dr.exit_reason,
                "exit_time": dr.exit_time,
                "hold_seconds": dr.hold_seconds,
                "mfe_r": dr.mfe_r,
                "mae_r": dr.mae_r,
                "traded": dr.traded,
            })

    matrix_df = pd.DataFrame(rows)
    matrix_path = OUTPUT_DIR / "delay_pnl_matrix.csv"
    matrix_df.to_csv(matrix_path, index=False)
    print(f"    Saved: {matrix_path} ({matrix_df.shape})")

    # ===================================================================
    # Step 3: 标签生成
    # ===================================================================

    print("\n" + "─" * 70)
    print("Step 3: 标签生成 (Optimal_Delay_Label)")
    print("─" * 70)

    optimal_labels = compute_optimal_labels(all_delay_results, tolerance_bps=5.0)
    label_series = pd.Series(optimal_labels, index=events.index, name="optimal_label")

    # 统计标签分布
    label_counts = Counter(optimal_labels)
    print("\n  Optimal_Delay_Label 分布:")
    for label_name in ["D0", "D5", "D10", "D15", "pullback", "skip"]:
        count = label_counts.get(label_name, 0)
        pct = count / n_events * 100.0
        print(f"    {label_name:>10s}: {count:3d} events ({pct:5.1f}%)")

    # 计算 Oracle baseline（每个 event 用最优 delay）
    oracle_results: list[DelayResult] = []
    for i, label_name in enumerate(optimal_labels):
        if label_name == "skip":
            # skip events 不参与 Oracle 计算
            continue
        # 从 delay_results[i] 中找到对应 label 的结果
        for dr in all_delay_results[i]:
            if dr.delay_label == label_name:
                oracle_results.append(dr)
                break

    oracle_calendar_sum = compute_calendar_sum_from_results(oracle_results, events)
    baseline_b_calendar_sum = baselines["D5"]["calendar_sum"]

    print(f"\n  Oracle calendar_sum (理论上限): {oracle_calendar_sum:+.4f}%")
    print(f"  Baseline B (D=5s):              {baseline_b_calendar_sum:+.4f}%")
    print(f"  Oracle vs Baseline B 差异:      {oracle_calendar_sum - baseline_b_calendar_sum:+.4f}%")

    if abs(oracle_calendar_sum - baseline_b_calendar_sum) < 1.0:
        print("  ⚠ delay_optimization_ceiling_low=true: "
              "Oracle 与 Baseline B 差异 < 1.0%，delay 优化空间有限")

    # 保存标签分布
    label_dist_df = pd.DataFrame([
        {"label": k, "count": v, "pct": v / n_events * 100.0}
        for k, v in sorted(label_counts.items())
    ])
    label_dist_path = OUTPUT_DIR / "label_distribution.csv"
    label_dist_df.to_csv(label_dist_path, index=False)
    print(f"\n  Saved: {label_dist_path}")

    # --- 分 train/test 的标签分布 ---
    # Use positional slicing: train = first split_idx, test = rest
    train_labels = optimal_labels[:split_idx]
    test_labels = optimal_labels[split_idx:]

    print(f"\n  Train set 标签分布 ({len(train_labels)} events):")
    train_label_counts = Counter(train_labels)
    for label_name in ["D0", "D5", "D10", "D15", "pullback", "skip"]:
        count = train_label_counts.get(label_name, 0)
        pct = count / len(train_labels) * 100.0 if train_labels else 0.0
        print(f"    {label_name:>10s}: {count:3d} ({pct:5.1f}%)")

    print(f"\n  Test set 标签分布 ({len(test_labels)} events):")
    test_label_counts = Counter(test_labels)
    for label_name in ["D0", "D5", "D10", "D15", "pullback", "skip"]:
        count = test_label_counts.get(label_name, 0)
        pct = count / len(test_labels) * 100.0 if test_labels else 0.0
        print(f"    {label_name:>10s}: {count:3d} ({pct:5.1f}%)")

    # ===================================================================
    # Step 4: 特征提取
    # ===================================================================

    print("\n" + "─" * 70)
    print("Step 4: 特征提取")
    print("─" * 70)

    print("\n  [4.1] Extracting pre-breakout features...")
    features, used_features, excluded_features = extract_features(events)
    print(f"        Used features:     {len(used_features)}")
    print(f"        Excluded features: {len(excluded_features)}")
    if excluded_features:
        print(f"        Excluded list: {excluded_features}")

    print("\n  [4.2] Splitting features by train/test...")
    # Use positional slicing (iloc) since train = first split_idx rows,
    # test = remaining rows of the sorted events.
    train_features = features.iloc[:split_idx].reset_index(drop=True)
    test_features = features.iloc[split_idx:].reset_index(drop=True)
    print(f"        Train features shape: {train_features.shape}")
    print(f"        Test features shape:  {test_features.shape}")

    print("\n  [4.3] Imputing missing values (median from train set)...")
    imputed_train, imputed_test, imputation_stats = impute_features(
        train_features, test_features
    )

    # Print imputation summary
    total_train_imputed = sum(
        s["train_missing_count"] for s in imputation_stats.values()
    )
    total_test_imputed = sum(
        s["test_missing_count"] for s in imputation_stats.values()
    )
    print(f"        Train NaN cells filled: {total_train_imputed}")
    print(f"        Test NaN cells filled:  {total_test_imputed}")

    # Show features with non-zero imputation
    features_with_missing = [
        (col, stats)
        for col, stats in imputation_stats.items()
        if stats["train_missing_count"] > 0 or stats["test_missing_count"] > 0
    ]
    if features_with_missing:
        print("        Features with missing values:")
        for col, stats in features_with_missing:
            print(f"          {col}: train={stats['train_missing_count']} "
                  f"({stats['train_missing_rate']:.1%}), "
                  f"test={stats['test_missing_count']} "
                  f"({stats['test_missing_rate']:.1%}), "
                  f"median={stats['median']:.4f}")

    # ===================================================================
    # Step 5: 分类器训练（仅 train set）
    # ===================================================================

    print("\n" + "─" * 70)
    print("Step 5: 分类器训练")
    print("─" * 70)

    # --- 5.1 过滤 skip labels（不参与分类器训练）---
    train_label_series = pd.Series(train_labels, name="optimal_label")

    # Positional mapping: for train events, position in all_delay_results = index in train_events
    # For test events, position in all_delay_results = split_idx + index in test_events

    # Filter out "skip" labels from training
    train_non_skip_mask = train_label_series != "skip"
    train_labels_filtered = train_label_series[train_non_skip_mask]
    imputed_train_filtered = imputed_train.loc[train_non_skip_mask]

    # Build train_delay_results_filtered: aligned with filtered train events
    train_delay_results_filtered: list[list[DelayResult]] = []
    for idx in train_labels_filtered.index:
        # idx is the position within train set, which equals position in all_delay_results
        train_delay_results_filtered.append(all_delay_results[idx])

    n_train_filtered = len(train_labels_filtered)
    n_skip_train = int((~train_non_skip_mask).sum())
    print(f"\n  Train events: {len(train_label_series)}")
    print(f"  Skip events (excluded from training): {n_skip_train}")
    print(f"  Training samples (non-skip): {n_train_filtered}")

    # --- 5.2 训练 4 种分类器 ---
    print("\n  Training classifiers...")

    print("    [1/4] Rule-based (DT max_depth=3)...")
    t0 = time.time()
    result_rule = train_rule_based_classifier(
        imputed_train_filtered, train_labels_filtered, train_delay_results_filtered
    )
    print(f"          LOOCV calendar_sum: {result_rule.loocv_calendar_sum:+.4f}% "
          f"({time.time() - t0:.1f}s)")

    print("    [2/4] Decision Tree (max_depth ∈ {2,3,4,5})...")
    t0 = time.time()
    result_dt = train_decision_tree(
        imputed_train_filtered, train_labels_filtered, train_delay_results_filtered
    )
    print(f"          Best max_depth: {result_dt.best_params.get('max_depth')}")
    print(f"          LOOCV calendar_sum: {result_dt.loocv_calendar_sum:+.4f}% "
          f"({time.time() - t0:.1f}s)")

    print("    [3/4] Random Forest (n_estimators=100, max_depth ∈ {3,4,5})...")
    t0 = time.time()
    result_rf = train_random_forest(
        imputed_train_filtered, train_labels_filtered, train_delay_results_filtered
    )
    print(f"          Best max_depth: {result_rf.best_params.get('max_depth')}")
    print(f"          LOOCV calendar_sum: {result_rf.loocv_calendar_sum:+.4f}% "
          f"({time.time() - t0:.1f}s)")

    print("    [4/4] Logistic Regression (multinomial)...")
    t0 = time.time()
    result_lr = train_logistic_regression(
        imputed_train_filtered, train_labels_filtered, train_delay_results_filtered
    )
    print(f"          LOOCV calendar_sum: {result_lr.loocv_calendar_sum:+.4f}% "
          f"({time.time() - t0:.1f}s)")

    # --- 5.3 选择 LOOCV calendar_sum 最优的分类器 ---
    classifier_results = [result_rule, result_dt, result_rf, result_lr]
    best_classifier = max(classifier_results, key=lambda r: r.loocv_calendar_sum)

    print(f"\n  ┌─────────────────────────────────────────────────────────────┐")
    print(f"  │ Classifier Comparison (LOOCV calendar_sum)                   │")
    print(f"  ├─────────────────────────────────────────────────────────────┤")
    for cr in classifier_results:
        marker = " ★" if cr.name == best_classifier.name else "  "
        print(f"  │{marker} {cr.name:<22s} LOOCV={cr.loocv_calendar_sum:+.4f}%  "
              f"Train={cr.train_calendar_sum:+.4f}%  "
              f"Acc={cr.train_accuracy:.1%} │")
    print(f"  └─────────────────────────────────────────────────────────────┘")
    print(f"\n  Best classifier: {best_classifier.name} "
          f"(LOOCV calendar_sum={best_classifier.loocv_calendar_sum:+.4f}%)")

    # ===================================================================
    # Step 6: 回调参数优化
    # ===================================================================

    print("\n" + "─" * 70)
    print("Step 6: 回调参数优化")
    print("─" * 70)

    # Get train events labeled as "pullback"
    pullback_mask = train_label_series == "pullback"
    pullback_events = train_events.loc[pullback_mask]
    n_pullback = len(pullback_events)

    print(f"\n  Train set pullback events: {n_pullback}")

    if n_pullback > 0:
        print("  Running grid search for optimal pullback params...")
        t0 = time.time()
        best_pullback_config = optimize_pullback_params(pullback_events, bars_cache)
        pb_elapsed = time.time() - t0
        print(f"  Grid search complete in {pb_elapsed:.1f}s")
        print(f"\n  Optimal pullback config:")
        print(f"    pullback_target_atr:    {best_pullback_config.pullback_target_atr}")
        print(f"    pullback_window_seconds: {best_pullback_config.pullback_window_seconds}")
        print(f"    start_offset_seconds:   {best_pullback_config.start_offset_seconds}")
    else:
        print("  No pullback events in train set — using default config.")
        best_pullback_config = PullbackConfig()

    # ===================================================================
    # Step 7: Test set 评估
    # ===================================================================

    print("\n" + "─" * 70)
    print("Step 7: Test set 评估")
    print("─" * 70)

    # --- 7.1 Re-create the best classifier and fit on all non-skip train data ---
    print(f"\n  [7.1] Re-creating best classifier: {best_classifier.name}")
    print(f"        Params: {best_classifier.best_params}")

    from sklearn.tree import DecisionTreeClassifier
    from sklearn.ensemble import RandomForestClassifier
    from sklearn.linear_model import LogisticRegression
    from sklearn.pipeline import Pipeline
    from sklearn.preprocessing import StandardScaler

    best_name = best_classifier.name
    best_params = best_classifier.best_params

    if best_name == "RuleBased_DT3":
        final_model = DecisionTreeClassifier(max_depth=3, random_state=42)
    elif best_name == "DecisionTree":
        final_model = DecisionTreeClassifier(
            max_depth=best_params["max_depth"], random_state=42
        )
    elif best_name == "RandomForest":
        final_model = RandomForestClassifier(
            n_estimators=100,
            max_depth=best_params["max_depth"],
            random_state=42,
        )
    elif best_name == "LogisticRegression":
        final_model = Pipeline([
            ("scaler", StandardScaler()),
            ("clf", LogisticRegression(
                multi_class="multinomial",
                solver="lbfgs",
                random_state=42,
                max_iter=1000,
            )),
        ])
    else:
        raise ValueError(f"Unknown classifier name: {best_name}")

    # Fit on all non-skip train data
    final_model.fit(imputed_train_filtered, train_labels_filtered)
    print(f"        Fitted on {len(train_labels_filtered)} non-skip train events")

    # --- 7.2 Predict labels for ALL test events ---
    print("\n  [7.2] Predicting regime for test set...")
    import numpy as np

    test_predictions = final_model.predict(imputed_test)
    print(f"        Test events: {len(test_predictions)}")
    print(f"        Predicted label distribution:")
    test_pred_counts = Counter(test_predictions)
    for lbl in ["D0", "D5", "D10", "D15", "pullback"]:
        cnt = test_pred_counts.get(lbl, 0)
        pct = cnt / len(test_predictions) * 100.0 if len(test_predictions) > 0 else 0.0
        print(f"          {lbl:>10s}: {cnt:3d} ({pct:5.1f}%)")

    # --- 7.3 Look up predicted label's DelayResult for each test event ---
    print("\n  [7.3] Looking up DelayResults for predicted regimes...")

    # Import helper from timing_classifier
    from pre_breakout_timing.timing_classifier import _find_delay_result

    test_results: list[DelayResult] = []
    for i in range(len(test_events)):
        pos = split_idx + i  # position in all_delay_results
        predicted_label = test_predictions[i]
        matched_result = _find_delay_result(all_delay_results[pos], predicted_label)
        test_results.append(matched_result)

    traded_test = [r for r in test_results if r.traded]
    print(f"        Traded test events: {len(traded_test)} / {len(test_results)}")

    # --- 7.4 Compute test set calendar_sum ---
    print("\n  [7.4] Computing test set calendar_sum...")
    test_calendar_sum = compute_calendar_sum_from_results(test_results, test_events)
    print(f"        Classifier test calendar_sum: {test_calendar_sum:+.4f}%")

    # --- 7.5 Compute test accuracy (predicted vs actual optimal labels) ---
    test_actual_labels = test_labels  # positional list from Step 3
    test_accuracy = float(np.mean(test_predictions == np.array(test_actual_labels)))
    print(f"        Test accuracy (vs optimal labels): {test_accuracy:.1%}")

    # --- 7.6 Compute Baseline B (D=5s) test set calendar_sum for comparison ---
    print("\n  [7.6] Computing Baseline B (D=5s) test set calendar_sum...")
    baseline_b_test_results: list[DelayResult] = []
    for i in range(len(test_events)):
        pos = split_idx + i
        # Find D5 result for this event
        d5_result = _find_delay_result(all_delay_results[pos], "D5")
        baseline_b_test_results.append(d5_result)

    baseline_b_test_calendar_sum = compute_calendar_sum_from_results(
        baseline_b_test_results, test_events
    )
    print(f"        Baseline B (D=5s) test calendar_sum: {baseline_b_test_calendar_sum:+.4f}%")

    # --- 7.7 Update best_classifier fields ---
    best_classifier.test_calendar_sum = test_calendar_sum
    best_classifier.test_accuracy = test_accuracy
    best_classifier.predictions_test = test_predictions

    # --- 7.8 Print comparison summary ---
    delta = test_calendar_sum - baseline_b_test_calendar_sum
    print(f"\n  ┌─────────────────────────────────────────────────────────────┐")
    print(f"  │ Test Set Evaluation Summary                                  │")
    print(f"  ├─────────────────────────────────────────────────────────────┤")
    print(f"  │ Classifier ({best_classifier.name}):                         │")
    print(f"  │   Test calendar_sum:  {test_calendar_sum:+.4f}%                          │")
    print(f"  │   Test accuracy:      {test_accuracy:.1%}                              │")
    print(f"  │   Traded events:      {len(traded_test)}/{len(test_results)}                              │")
    print(f"  ├─────────────────────────────────────────────────────────────┤")
    print(f"  │ Baseline B (D=5s):                                          │")
    print(f"  │   Test calendar_sum:  {baseline_b_test_calendar_sum:+.4f}%                          │")
    print(f"  ├─────────────────────────────────────────────────────────────┤")
    print(f"  │ Delta (Classifier - Baseline B): {delta:+.4f}%                  │")
    if delta > 0.5:
        print(f"  │ ✓ Classifier SUPERIOR to Baseline B by >{0.5}%              │")
    elif delta > 0:
        print(f"  │ ~ Classifier marginally better (< 0.5% improvement)        │")
    else:
        print(f"  │ ✗ Classifier NOT superior to Baseline B                     │")
    print(f"  └─────────────────────────────────────────────────────────────┘")
    # ===================================================================
    # Step 8: 稳健性验证
    # ===================================================================

    print("\n" + "─" * 70)
    print("Step 8: 稳健性验证")
    print("─" * 70)

    # --- 8.1 Bootstrap CI (1000 次重采样，分 BTC/ETH 独立执行) ---
    print("\n  [8.1] Bootstrap CI (1000 resamples, per-symbol)")

    N_BOOTSTRAP = 1000
    rng = np.random.default_rng(42)

    def _bootstrap_calendar_sum(
        results: list[DelayResult],
        events_df: pd.DataFrame,
        rng: np.random.Generator,
        n_bootstrap: int = 1000,
    ) -> dict:
        """对 results 进行 bootstrap 重采样，计算 calendar_sum 的置信区间。

        Args:
            results: 待 bootstrap 的 DelayResult 列表
            events_df: 对应的 events DataFrame（用于 silo 计算）
            rng: numpy random generator（确保可复现）
            n_bootstrap: 重采样次数

        Returns:
            dict with keys: mean, std, ci_5, ci_95, samples
        """
        n = len(results)
        if n == 0:
            return {"mean": 0.0, "std": 0.0, "ci_5": 0.0, "ci_95": 0.0, "samples": []}

        bootstrap_sums: list[float] = []
        for _ in range(n_bootstrap):
            # 有放回抽样（同样大小）
            indices = rng.integers(0, n, size=n)
            resampled_results = [results[i] for i in indices]
            # 对应的 events 行也需要重采样以保持 silo 计算一致
            resampled_events_indices = [events_df.index[i] for i in indices]
            resampled_events = events_df.loc[resampled_events_indices].copy()
            # 重置 index 避免重复 index 问题
            resampled_events = resampled_events.reset_index(drop=True)

            cal_sum = compute_calendar_sum_from_results(resampled_results, resampled_events)
            bootstrap_sums.append(cal_sum)

        bootstrap_arr = np.array(bootstrap_sums)
        return {
            "mean": float(np.mean(bootstrap_arr)),
            "std": float(np.std(bootstrap_arr)),
            "ci_5": float(np.percentile(bootstrap_arr, 5)),
            "ci_95": float(np.percentile(bootstrap_arr, 95)),
            "samples": bootstrap_sums,
        }

    # --- Overall bootstrap CI ---
    print(f"    Computing overall bootstrap CI ({N_BOOTSTRAP} resamples)...")
    bootstrap_overall = _bootstrap_calendar_sum(
        test_results, test_events, rng, N_BOOTSTRAP
    )
    print(f"      Overall CI [5th, 95th]: "
          f"[{bootstrap_overall['ci_5']:+.4f}%, {bootstrap_overall['ci_95']:+.4f}%]")
    print(f"      Mean: {bootstrap_overall['mean']:+.4f}%, Std: {bootstrap_overall['std']:.4f}%")

    # --- Per-symbol bootstrap CI (BTC / ETH 独立) ---
    # Build symbol-aligned test_results and test_events subsets
    test_symbols = test_events["symbol"].values

    btc_mask = [s.upper().startswith("BTC") for s in test_symbols]
    eth_mask = [s.upper().startswith("ETH") for s in test_symbols]

    btc_results = [r for r, m in zip(test_results, btc_mask) if m]
    eth_results = [r for r, m in zip(test_results, eth_mask) if m]

    btc_test_events = test_events.loc[[idx for idx, m in zip(test_events.index, btc_mask) if m]]
    eth_test_events = test_events.loc[[idx for idx, m in zip(test_events.index, eth_mask) if m]]

    print(f"\n    BTC test events: {len(btc_results)}")
    print(f"    ETH test events: {len(eth_results)}")

    # BTC bootstrap
    print(f"    Computing BTC bootstrap CI ({N_BOOTSTRAP} resamples)...")
    bootstrap_btc = _bootstrap_calendar_sum(
        btc_results, btc_test_events.reset_index(drop=True), rng, N_BOOTSTRAP
    )
    print(f"      BTC CI [5th, 95th]: "
          f"[{bootstrap_btc['ci_5']:+.4f}%, {bootstrap_btc['ci_95']:+.4f}%]")
    print(f"      Mean: {bootstrap_btc['mean']:+.4f}%, Std: {bootstrap_btc['std']:.4f}%")

    # ETH bootstrap
    print(f"    Computing ETH bootstrap CI ({N_BOOTSTRAP} resamples)...")
    bootstrap_eth = _bootstrap_calendar_sum(
        eth_results, eth_test_events.reset_index(drop=True), rng, N_BOOTSTRAP
    )
    print(f"      ETH CI [5th, 95th]: "
          f"[{bootstrap_eth['ci_5']:+.4f}%, {bootstrap_eth['ci_95']:+.4f}%]")
    print(f"      Mean: {bootstrap_eth['mean']:+.4f}%, Std: {bootstrap_eth['std']:.4f}%")

    # Store bootstrap results
    bootstrap_ci = {
        "overall": {
            "calendar_sum": test_calendar_sum,
            "ci_5": bootstrap_overall["ci_5"],
            "ci_95": bootstrap_overall["ci_95"],
            "mean": bootstrap_overall["mean"],
            "std": bootstrap_overall["std"],
        },
        "BTC": {
            "n_events": len(btc_results),
            "ci_5": bootstrap_btc["ci_5"],
            "ci_95": bootstrap_btc["ci_95"],
            "mean": bootstrap_btc["mean"],
            "std": bootstrap_btc["std"],
        },
        "ETH": {
            "n_events": len(eth_results),
            "ci_5": bootstrap_eth["ci_5"],
            "ci_95": bootstrap_eth["ci_95"],
            "mean": bootstrap_eth["mean"],
            "std": bootstrap_eth["std"],
        },
        "n_bootstrap": N_BOOTSTRAP,
        "random_state": 42,
    }

    print(f"\n  ┌─────────────────────────────────────────────────────────────┐")
    print(f"  │ Bootstrap CI Summary (1000 resamples, random_state=42)       │")
    print(f"  ├─────────────────────────────────────────────────────────────┤")
    print(f"  │ Overall:  calendar_sum={test_calendar_sum:+.4f}%                        │")
    print(f"  │           CI [5th, 95th] = [{bootstrap_overall['ci_5']:+.4f}%, "
          f"{bootstrap_overall['ci_95']:+.4f}%]     │")
    print(f"  ├─────────────────────────────────────────────────────────────┤")
    print(f"  │ BTC ({len(btc_results)} events):                                          │")
    print(f"  │           CI [5th, 95th] = [{bootstrap_btc['ci_5']:+.4f}%, "
          f"{bootstrap_btc['ci_95']:+.4f}%]     │")
    print(f"  ├─────────────────────────────────────────────────────────────┤")
    print(f"  │ ETH ({len(eth_results)} events):                                          │")
    print(f"  │           CI [5th, 95th] = [{bootstrap_eth['ci_5']:+.4f}%, "
          f"{bootstrap_eth['ci_95']:+.4f}%]     │")
    print(f"  └─────────────────────────────────────────────────────────────┘")

    # --- 8.2 分 symbol 验证 (btc_only / eth_only) ---
    print("\n  [8.2] 分 symbol 验证 (btc_only / eth_only)")

    # --- Test set per-symbol: classifier results ---
    btc_test_calendar_sum = compute_calendar_sum_from_results(
        btc_results, btc_test_events.reset_index(drop=True)
    )
    eth_test_calendar_sum = compute_calendar_sum_from_results(
        eth_results, eth_test_events.reset_index(drop=True)
    )

    # Win rate per symbol (test set, classifier predictions)
    btc_traded = [r for r in btc_results if r.traded]
    eth_traded = [r for r in eth_results if r.traded]
    btc_wins = [r for r in btc_traded if r.pnl_pct is not None and r.pnl_pct > 0]
    eth_wins = [r for r in eth_traded if r.pnl_pct is not None and r.pnl_pct > 0]
    btc_win_rate = len(btc_wins) / len(btc_traded) * 100.0 if btc_traded else 0.0
    eth_win_rate = len(eth_wins) / len(eth_traded) * 100.0 if eth_traded else 0.0

    # --- Test set per-symbol: Baseline B (D=5s) results ---
    btc_baseline_b_test = [
        r for r, m in zip(baseline_b_test_results, btc_mask) if m
    ]
    eth_baseline_b_test = [
        r for r, m in zip(baseline_b_test_results, eth_mask) if m
    ]
    btc_baseline_b_calendar_sum = compute_calendar_sum_from_results(
        btc_baseline_b_test, btc_test_events.reset_index(drop=True)
    )
    eth_baseline_b_calendar_sum = compute_calendar_sum_from_results(
        eth_baseline_b_test, eth_test_events.reset_index(drop=True)
    )

    # Baseline B win rate per symbol (test set)
    btc_baseline_traded = [r for r in btc_baseline_b_test if r.traded]
    eth_baseline_traded = [r for r in eth_baseline_b_test if r.traded]
    btc_baseline_wins = [
        r for r in btc_baseline_traded if r.pnl_pct is not None and r.pnl_pct > 0
    ]
    eth_baseline_wins = [
        r for r in eth_baseline_traded if r.pnl_pct is not None and r.pnl_pct > 0
    ]
    btc_baseline_win_rate = (
        len(btc_baseline_wins) / len(btc_baseline_traded) * 100.0
        if btc_baseline_traded else 0.0
    )
    eth_baseline_win_rate = (
        len(eth_baseline_wins) / len(eth_baseline_traded) * 100.0
        if eth_baseline_traded else 0.0
    )

    # --- Train set per-symbol: classifier results ---
    train_symbols = train_events["symbol"].values
    btc_train_mask = [s.upper().startswith("BTC") for s in train_symbols]
    eth_train_mask = [s.upper().startswith("ETH") for s in train_symbols]

    # Get train set results using best classifier predictions (full-fit on train)
    train_predictions = final_model.predict(imputed_train)
    train_results_all: list[DelayResult] = []
    for i in range(len(train_events)):
        pos = i  # train events are positions 0..split_idx-1 in all_delay_results
        predicted_label = train_predictions[i]
        matched_result = _find_delay_result(all_delay_results[pos], predicted_label)
        train_results_all.append(matched_result)

    btc_train_results = [r for r, m in zip(train_results_all, btc_train_mask) if m]
    eth_train_results = [r for r, m in zip(train_results_all, eth_train_mask) if m]

    btc_train_events_df = train_events.loc[
        [idx for idx, m in zip(train_events.index, btc_train_mask) if m]
    ]
    eth_train_events_df = train_events.loc[
        [idx for idx, m in zip(train_events.index, eth_train_mask) if m]
    ]

    btc_train_calendar_sum = compute_calendar_sum_from_results(
        btc_train_results, btc_train_events_df.reset_index(drop=True)
    )
    eth_train_calendar_sum = compute_calendar_sum_from_results(
        eth_train_results, eth_train_events_df.reset_index(drop=True)
    )

    # Train set win rate per symbol
    btc_train_traded = [r for r in btc_train_results if r.traded]
    eth_train_traded = [r for r in eth_train_results if r.traded]
    btc_train_wins = [
        r for r in btc_train_traded if r.pnl_pct is not None and r.pnl_pct > 0
    ]
    eth_train_wins = [
        r for r in eth_train_traded if r.pnl_pct is not None and r.pnl_pct > 0
    ]
    btc_train_win_rate = (
        len(btc_train_wins) / len(btc_train_traded) * 100.0
        if btc_train_traded else 0.0
    )
    eth_train_win_rate = (
        len(eth_train_wins) / len(eth_train_traded) * 100.0
        if eth_train_traded else 0.0
    )

    # Train set Baseline B per symbol
    btc_train_baseline_results: list[DelayResult] = []
    eth_train_baseline_results: list[DelayResult] = []
    for i in range(len(train_events)):
        pos = i  # train events are positions 0..split_idx-1
        d5_result = _find_delay_result(all_delay_results[pos], "D5")
        if btc_train_mask[i]:
            btc_train_baseline_results.append(d5_result)
        if eth_train_mask[i]:
            eth_train_baseline_results.append(d5_result)

    btc_train_baseline_calendar_sum = compute_calendar_sum_from_results(
        btc_train_baseline_results, btc_train_events_df.reset_index(drop=True)
    )
    eth_train_baseline_calendar_sum = compute_calendar_sum_from_results(
        eth_train_baseline_results, eth_train_events_df.reset_index(drop=True)
    )

    # --- Superiority flags ---
    symbol_not_superior_flag_btc = bool(btc_test_calendar_sum < btc_baseline_b_calendar_sum)
    symbol_not_superior_flag_eth = bool(eth_test_calendar_sum < eth_baseline_b_calendar_sum)

    # --- Assemble symbol_results dict ---
    symbol_results = {
        "BTC": {
            "test": {
                "n_events": len(btc_results),
                "classifier_calendar_sum": btc_test_calendar_sum,
                "baseline_b_calendar_sum": btc_baseline_b_calendar_sum,
                "win_rate": btc_win_rate,
                "baseline_b_win_rate": btc_baseline_win_rate,
                "traded_count": len(btc_traded),
                "wins_count": len(btc_wins),
            },
            "train": {
                "n_events": len(btc_train_results),
                "classifier_calendar_sum": btc_train_calendar_sum,
                "baseline_b_calendar_sum": btc_train_baseline_calendar_sum,
                "win_rate": btc_train_win_rate,
                "traded_count": len(btc_train_traded),
                "wins_count": len(btc_train_wins),
            },
            "symbol_not_superior_flag": symbol_not_superior_flag_btc,
        },
        "ETH": {
            "test": {
                "n_events": len(eth_results),
                "classifier_calendar_sum": eth_test_calendar_sum,
                "baseline_b_calendar_sum": eth_baseline_b_calendar_sum,
                "win_rate": eth_win_rate,
                "baseline_b_win_rate": eth_baseline_win_rate,
                "traded_count": len(eth_traded),
                "wins_count": len(eth_wins),
            },
            "train": {
                "n_events": len(eth_train_results),
                "classifier_calendar_sum": eth_train_calendar_sum,
                "baseline_b_calendar_sum": eth_train_baseline_calendar_sum,
                "win_rate": eth_train_win_rate,
                "traded_count": len(eth_train_traded),
                "wins_count": len(eth_train_wins),
            },
            "symbol_not_superior_flag": symbol_not_superior_flag_eth,
        },
    }

    # --- Print summary ---
    print(f"\n  ┌─────────────────────────────────────────────────────────────┐")
    print(f"  │ Per-Symbol Validation (Test Set)                             │")
    print(f"  ├─────────────────────────────────────────────────────────────┤")
    print(f"  │ BTC ({len(btc_results)} events):                                          │")
    print(f"  │   Classifier calendar_sum: {btc_test_calendar_sum:+.4f}%                  │")
    print(f"  │   Baseline B calendar_sum: {btc_baseline_b_calendar_sum:+.4f}%                  │")
    print(f"  │   Classifier win_rate:     {btc_win_rate:.1f}%                            │")
    print(f"  │   Baseline B win_rate:     {btc_baseline_win_rate:.1f}%                            │")
    if symbol_not_superior_flag_btc:
        print(f"  │   ⚠ symbol_not_superior_flag_btc = true                     │")
    else:
        print(f"  │   ✓ Classifier superior for BTC                             │")
    print(f"  ├─────────────────────────────────────────────────────────────┤")
    print(f"  │ ETH ({len(eth_results)} events):                                          │")
    print(f"  │   Classifier calendar_sum: {eth_test_calendar_sum:+.4f}%                  │")
    print(f"  │   Baseline B calendar_sum: {eth_baseline_b_calendar_sum:+.4f}%                  │")
    print(f"  │   Classifier win_rate:     {eth_win_rate:.1f}%                            │")
    print(f"  │   Baseline B win_rate:     {eth_baseline_win_rate:.1f}%                            │")
    if symbol_not_superior_flag_eth:
        print(f"  │   ⚠ symbol_not_superior_flag_eth = true                     │")
    else:
        print(f"  │   ✓ Classifier superior for ETH                             │")
    print(f"  └─────────────────────────────────────────────────────────────┘")

    print(f"\n  ┌─────────────────────────────────────────────────────────────┐")
    print(f"  │ Per-Symbol Validation (Train Set — for comparison)           │")
    print(f"  ├─────────────────────────────────────────────────────────────┤")
    print(f"  │ BTC ({len(btc_train_results)} events):                                          │")
    print(f"  │   Classifier calendar_sum: {btc_train_calendar_sum:+.4f}%                  │")
    print(f"  │   Baseline B calendar_sum: {btc_train_baseline_calendar_sum:+.4f}%                  │")
    print(f"  │   Classifier win_rate:     {btc_train_win_rate:.1f}%                            │")
    print(f"  ├─────────────────────────────────────────────────────────────┤")
    print(f"  │ ETH ({len(eth_train_results)} events):                                          │")
    print(f"  │   Classifier calendar_sum: {eth_train_calendar_sum:+.4f}%                  │")
    print(f"  │   Baseline B calendar_sum: {eth_train_baseline_calendar_sum:+.4f}%                  │")
    print(f"  │   Classifier win_rate:     {eth_train_win_rate:.1f}%                            │")
    print(f"  └─────────────────────────────────────────────────────────────┘")

    # --- 8.3 Overfitting 检查 (train vs test calendar_sum) ---
    print("\n  [8.3] Overfitting 检查")

    train_cs = best_classifier.train_calendar_sum
    test_cs = test_calendar_sum
    loocv_cs = best_classifier.loocv_calendar_sum

    # Compute drop_pct: (train_cs - test_cs) / abs(train_cs) * 100
    if train_cs != 0:
        drop_pct = (train_cs - test_cs) / abs(train_cs) * 100.0
    else:
        drop_pct = 0.0

    # Overfitting flag: test calendar_sum drops > 50% relative to train
    overfitting_flag = bool(drop_pct > 50.0)

    # LOOCV degradation flag: LOOCV vs train full-fit differs > 30%
    if train_cs != 0:
        loocv_drop_pct = (train_cs - loocv_cs) / abs(train_cs) * 100.0
    else:
        loocv_drop_pct = 0.0

    loocv_degradation_flag = bool(abs(loocv_drop_pct) > 30.0)

    # Assemble overfitting_check dict
    overfitting_check = {
        "train_cs": train_cs,
        "test_cs": test_cs,
        "loocv_cs": loocv_cs,
        "drop_pct": drop_pct,
        "overfitting_flag": overfitting_flag,
        "loocv_degradation_flag": loocv_degradation_flag,
    }

    print(f"    Train calendar_sum:  {train_cs:+.4f}%")
    print(f"    Test calendar_sum:   {test_cs:+.4f}%")
    print(f"    LOOCV calendar_sum:  {loocv_cs:+.4f}%")
    print(f"    Drop (train→test):   {drop_pct:+.2f}%")
    print(f"    Drop (train→LOOCV):  {loocv_drop_pct:+.2f}%")
    print(f"    overfitting_flag:    {overfitting_flag}")
    print(f"    loocv_degradation_flag: {loocv_degradation_flag}")

    if overfitting_flag:
        print("    ⚠ overfitting_flag=true: test calendar_sum 下降超过 50%，建议简化模型")
    if loocv_degradation_flag:
        print("    ⚠ loocv_degradation_flag=true: LOOCV 与 train full-fit 差异超过 30%")

    print(f"\n  ┌─────────────────────────────────────────────────────────────┐")
    print(f"  │ Overfitting Check                                            │")
    print(f"  ├─────────────────────────────────────────────────────────────┤")
    print(f"  │ Train calendar_sum:     {train_cs:+.4f}%                          │")
    print(f"  │ Test calendar_sum:      {test_cs:+.4f}%                          │")
    print(f"  │ LOOCV calendar_sum:     {loocv_cs:+.4f}%                          │")
    print(f"  │ Drop (train→test):      {drop_pct:+.2f}%                              │")
    print(f"  │ Drop (train→LOOCV):     {loocv_drop_pct:+.2f}%                              │")
    print(f"  ├─────────────────────────────────────────────────────────────┤")
    if overfitting_flag:
        print(f"  │ ⚠ overfitting_flag = TRUE                                   │")
        print(f"  │   建议：简化模型（降低 max_depth 或使用规则型分类器）         │")
    else:
        print(f"  │ ✓ overfitting_flag = false                                   │")
    if loocv_degradation_flag:
        print(f"  │ ⚠ loocv_degradation_flag = TRUE                             │")
        print(f"  │   LOOCV 与 full-fit 差异过大，泛化能力存疑                   │")
    else:
        print(f"  │ ✓ loocv_degradation_flag = false                             │")
    print(f"  └─────────────────────────────────────────────────────────────┘")

    # --- 8.4 Regime 稳定性分析 (train/test label 分布差异) ---
    print("\n  [8.4] Regime 稳定性分析 (train/test label 分布差异)")

    n_train = len(train_labels)
    n_test = len(test_labels)
    all_regimes = ["D0", "D5", "D10", "D15", "pullback", "skip"]

    regime_stability_rows: list[dict] = []
    max_diff_pp = 0.0  # track maximum difference in percentage points

    for regime in all_regimes:
        train_count = train_label_counts.get(regime, 0)
        test_count = test_label_counts.get(regime, 0)
        train_prop = train_count / n_train * 100.0 if n_train > 0 else 0.0
        test_prop = test_count / n_test * 100.0 if n_test > 0 else 0.0
        diff_pp = abs(train_prop - test_prop)
        max_diff_pp = max(max_diff_pp, diff_pp)

        regime_stability_rows.append({
            "regime": regime,
            "train_count": train_count,
            "train_pct": train_prop,
            "test_count": test_count,
            "test_pct": test_prop,
            "diff_pp": diff_pp,
        })

    # Flag: any regime proportion difference > 15 percentage points
    label_distribution_shift = max_diff_pp > 15.0

    # Assemble regime_stability dict
    regime_stability = {
        "n_train": n_train,
        "n_test": n_test,
        "per_regime": regime_stability_rows,
        "max_diff_pp": max_diff_pp,
        "label_distribution_shift": label_distribution_shift,
    }

    # Print comparison table
    print(f"\n    {'Regime':<12s} {'Train':>8s} {'Train%':>8s} {'Test':>8s} {'Test%':>8s} {'Diff(pp)':>9s}")
    print(f"    {'─' * 12} {'─' * 8} {'─' * 8} {'─' * 8} {'─' * 8} {'─' * 9}")
    for row in regime_stability_rows:
        flag = " ⚠" if row["diff_pp"] > 15.0 else ""
        print(f"    {row['regime']:<12s} {row['train_count']:>8d} {row['train_pct']:>7.1f}% "
              f"{row['test_count']:>8d} {row['test_pct']:>7.1f}% {row['diff_pp']:>8.1f}pp{flag}")
    print(f"    {'─' * 12} {'─' * 8} {'─' * 8} {'─' * 8} {'─' * 8} {'─' * 9}")

    print(f"\n    Max proportion difference: {max_diff_pp:.1f} pp")
    print(f"    label_distribution_shift:  {label_distribution_shift}")

    if label_distribution_shift:
        # Identify which regimes shifted
        shifted_regimes = [r["regime"] for r in regime_stability_rows if r["diff_pp"] > 15.0]
        print(f"    ⚠ label_distribution_shift=true: "
              f"regime(s) {shifted_regimes} 在 train/test 间比例差异超过 15pp")
    else:
        print(f"    ✓ 所有 regime 在 train/test 间比例差异均 ≤ 15pp，分布稳定")

    print(f"\n  ┌─────────────────────────────────────────────────────────────┐")
    print(f"  │ Regime Stability Analysis                                    │")
    print(f"  ├─────────────────────────────────────────────────────────────┤")
    print(f"  │ Train events: {n_train:<4d}  Test events: {n_test:<4d}                      │")
    print(f"  ├─────────────────────────────────────────────────────────────┤")
    for row in regime_stability_rows:
        flag_str = "⚠" if row["diff_pp"] > 15.0 else " "
        print(f"  │ {flag_str} {row['regime']:<10s} "
              f"train={row['train_pct']:5.1f}%  test={row['test_pct']:5.1f}%  "
              f"Δ={row['diff_pp']:5.1f}pp          │")
    print(f"  ├─────────────────────────────────────────────────────────────┤")
    if label_distribution_shift:
        print(f"  │ ⚠ label_distribution_shift = TRUE (max Δ={max_diff_pp:.1f}pp > 15pp)  │")
    else:
        print(f"  │ ✓ label_distribution_shift = false (max Δ={max_diff_pp:.1f}pp ≤ 15pp) │")
    print(f"  └─────────────────────────────────────────────────────────────┘")

    # ===================================================================
    # Step 9: 报告生成
    # ===================================================================

    print("\n" + "─" * 70)
    print("Step 9: 报告生成")
    print("─" * 70)

    # Compute feature statistics for the report
    print("\n  [9.1] Computing feature statistics...")
    feature_stats = feature_statistics(imputed_train_filtered, train_labels_filtered)
    print(f"        Feature stats shape: {feature_stats.shape}")

    # Generate all output files
    print("\n  [9.2] Generating report files...")
    generate_report(
        classifier_results=classifier_results,
        best_classifier=best_classifier,
        baselines=baselines,
        oracle_calendar_sum=oracle_calendar_sum,
        bootstrap_ci=bootstrap_ci,
        symbol_results=symbol_results,
        overfitting_check=overfitting_check,
        regime_stability=regime_stability,
        feature_stats=feature_stats,
        used_features=used_features,
        excluded_features=excluded_features,
        events=events,
        test_events=test_events,
        test_results=test_results,
        test_predictions=test_predictions,
        optimal_labels=optimal_labels,
        all_delay_results=all_delay_results,
        best_pullback_config=best_pullback_config,
        baseline_b_test_calendar_sum=baseline_b_test_calendar_sum,
        output_dir=OUTPUT_DIR,
    )

    # ===================================================================
    # Summary
    # ===================================================================

    print("\n" + "=" * 70)
    print("Pipeline Complete (Steps 1-9)")
    print("=" * 70)
    print(f"\n  Events loaded:       {n_events}")
    print(f"  Train/Test split:    {len(train_events)}/{len(test_events)}")
    print(f"  Simulation time:     {sim_elapsed:.1f}s")
    print(f"  Oracle calendar_sum: {oracle_calendar_sum:+.4f}%")
    print(f"  Baseline B (D=5s):   {baseline_b_calendar_sum:+.4f}% (all)")
    print(f"  Baseline B (D=5s):   {baseline_b_test_calendar_sum:+.4f}% (test only)")
    print(f"  Label distribution:  {dict(label_counts)}")
    print(f"  Features used:       {len(used_features)}")
    print(f"  Features excluded:   {len(excluded_features)}")
    print(f"  Best classifier:     {best_classifier.name} "
          f"(LOOCV={best_classifier.loocv_calendar_sum:+.4f}%)")
    print(f"  Test calendar_sum:   {test_calendar_sum:+.4f}%")
    print(f"  Test accuracy:       {test_accuracy:.1%}")
    print(f"  Pullback events:     {n_pullback}")
    if n_pullback > 0:
        print(f"  Pullback config:     target_atr={best_pullback_config.pullback_target_atr}, "
              f"window={best_pullback_config.pullback_window_seconds}s")
    print(f"\n  Output directory:    {OUTPUT_DIR}")
    print(f"  Files generated:")
    print(f"    - delay_pnl_matrix.csv")
    print(f"    - label_distribution.csv")
    print(f"    - pre_breakout_timing_report.md")
    print(f"    - pre_breakout_timing_summary.json")
    print(f"    - pre_breakout_timing_attribution.csv")
    print(f"    - pre_breakout_timing_trades.csv")
    print(f"    - classifier_rules.md")
    print(f"    - feature_importance.csv")
    print("=" * 70)

    return {
        "events": events,
        "train_events": train_events,
        "test_events": test_events,
        "bars_cache": bars_cache,
        "all_delay_results": all_delay_results,
        "optimal_labels": optimal_labels,
        "label_series": label_series,
        "baselines": baselines,
        "oracle_calendar_sum": oracle_calendar_sum,
        "features": features,
        "used_features": used_features,
        "excluded_features": excluded_features,
        "imputed_train": imputed_train,
        "imputed_test": imputed_test,
        "imputation_stats": imputation_stats,
        "classifier_results": classifier_results,
        "best_classifier": best_classifier,
        "train_labels_filtered": train_labels_filtered,
        "train_delay_results_filtered": train_delay_results_filtered,
        "best_pullback_config": best_pullback_config,
        "test_predictions": test_predictions,
        "test_results": test_results,
        "test_calendar_sum": test_calendar_sum,
        "test_accuracy": test_accuracy,
        "baseline_b_test_calendar_sum": baseline_b_test_calendar_sum,
        "baseline_b_test_results": baseline_b_test_results,
        "bootstrap_ci": bootstrap_ci,
        "symbol_results": symbol_results,
        "overfitting_check": overfitting_check,
        "regime_stability": regime_stability,
    }


if __name__ == "__main__":
    main()
