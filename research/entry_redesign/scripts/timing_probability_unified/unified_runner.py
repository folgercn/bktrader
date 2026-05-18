"""Unified Runner — 主入口 orchestrator

按 10 步流程串联 timing classification、probability sizing、speed gate 三信号融合框架。
确保 random_state=42 全局固定，单 event 失败不中断 pipeline。
"""

from __future__ import annotations

import logging
import sys
import time
from dataclasses import dataclass
from pathlib import Path

import numpy as np
import pandas as pd

# Ensure pre_breakout_timing is importable
_SCRIPTS_DIR = Path(__file__).resolve().parents[1]
if str(_SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS_DIR))

# --- 常量定义 ---

OUTPUT_DIR = Path(__file__).resolve().parents[1] / "output" / "timing_probability_unified"

RANDOM_STATE = 42

Original_10_Features = [
    "signal_atr_percentile",
    "roundtrip_cost_atr",
    "prev1_body_atr",
    "prev1_range_atr",
    "prev1_close_pos_side",
    "prev_sma5_gap_atr",
    "prev_sma5_slope_atr",
    "level_to_prev_close_atr",
    "level_to_signal_open_atr",
    "touch_extension_atr",
]


@dataclass
class PipelineError:
    """Pipeline 错误记录"""

    event_id: str
    stage: str  # "data_load" / "delay_sim" / "feature_extract"
    error_type: str
    error_message: str
    action_taken: str  # "skipped" / "fallback" / "warning"


# --- Imports from project modules ---
from timing_probability_unified.event_source_builder import build_event_pool  # noqa: E402
from timing_probability_unified.timing_classifier import (  # noqa: E402
    generate_3regime_labels,
    train_and_select,
)
from timing_probability_unified.probability_model import (  # noqa: E402
    compute_sizing_multiplier,
    generate_rf_binary_labels,
    train_rf_probability,
)
from timing_probability_unified.combined_executor import (  # noqa: E402
    compute_calendar_sum,
    compute_combined_positions,
    compute_worst_sm,
    run_sensitivity_analysis,
)
from timing_probability_unified.speed_gate import (  # noqa: E402
    analyze_speed_gate,
    compute_speed_gate,
)
from timing_probability_unified.robustness import (  # noqa: E402
    RobustnessResult,
    run_ablation_study,
    run_bootstrap,
    run_forward_validation,
)
from timing_probability_unified.report_generator import generate_report  # noqa: E402
from timing_probability_unified.feature_computer import fill_missing_features  # noqa: E402
from pre_breakout_timing.delay_simulator import DelayResult, simulate_all_delays  # noqa: E402
from pre_breakout_timing.feature_extractor import extract_features, impute_features  # noqa: E402
from dynamic_timing.execution_sim import DEFAULT_EXEC_PARAMS  # noqa: E402

logger = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# Default pullback params (consistent with V4 execution model / tick-flow)
# ---------------------------------------------------------------------------
_DEFAULT_PULLBACK_PARAMS: dict = {
    "pullback_target_atr": 0.05,
    "pullback_window_seconds": 60,
    "start_offset_seconds": 5,
}


def _get_bars_for_event(
    event: pd.Series,
    bars_cache: dict[str, pd.DataFrame],
) -> pd.DataFrame | None:
    """Retrieve 1s bar data for a single event from bars_cache.

    The bars_cache uses keys of the form "{symbol}_{YYYYMM}".
    """
    symbol = str(event["symbol"])
    touch_time = pd.Timestamp(event["touch_time"])
    if touch_time.tzinfo is None:
        touch_time = touch_time.tz_localize("UTC")
    month_key = f"{symbol}_{touch_time.strftime('%Y%m')}"
    bars = bars_cache.get(month_key)
    if bars is not None and not bars.empty:
        return bars
    return None


def _simulate_delays_for_events(
    events: pd.DataFrame,
    bars_cache: dict[str, pd.DataFrame],
) -> tuple[list[list[DelayResult]], list[PipelineError]]:
    """对事件池中所有 events 执行 5 种 delay 模拟。

    单个 event 失败不中断 pipeline：失败的 event 产出 untradeable placeholder
    并记录到 errors 列表。

    Returns
    -------
    tuple[list[list[DelayResult]], list[PipelineError]]
        - delay_results: 每个 event 的 5 个 DelayResult
        - errors: 模拟失败的错误记录
    """
    delay_results: list[list[DelayResult]] = []
    errors: list[PipelineError] = []

    for idx in range(len(events)):
        event = events.iloc[idx]
        event_id = str(event.get("event_id", f"evt_{idx}"))

        bars = _get_bars_for_event(event, bars_cache)
        if bars is None:
            # 无 bar 数据 → 产出 untradeable placeholder
            placeholder = []
            for label in ["D0", "D5", "D10", "D15", "pullback"]:
                placeholder.append(
                    DelayResult(
                        event_id=event_id,
                        delay_label=label,
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
                    )
                )
            delay_results.append(placeholder)
            errors.append(
                PipelineError(
                    event_id=event_id,
                    stage="delay_sim",
                    error_type="NoBarData",
                    error_message=f"No bar data available for event {event_id}",
                    action_taken="skipped",
                )
            )
            continue

        try:
            event_delays = simulate_all_delays(
                event=event,
                second_bars=bars,
                pullback_params=_DEFAULT_PULLBACK_PARAMS,
            )
            delay_results.append(event_delays)
        except Exception as e:
            # 模拟失败 → 产出 untradeable placeholder
            placeholder = []
            for label in ["D0", "D5", "D10", "D15", "pullback"]:
                placeholder.append(
                    DelayResult(
                        event_id=event_id,
                        delay_label=label,
                        delay_seconds=0,
                        entry_time=None,
                        entry_price=None,
                        pnl_pct=None,
                        exit_reason="SimError",
                        exit_time=None,
                        hold_seconds=None,
                        mfe_r=None,
                        mae_r=None,
                        traded=False,
                    )
                )
            delay_results.append(placeholder)
            errors.append(
                PipelineError(
                    event_id=event_id,
                    stage="delay_sim",
                    error_type=type(e).__name__,
                    error_message=str(e)[:200],
                    action_taken="skipped",
                )
            )

    return delay_results, errors


def _compute_delay_traded_stats(
    delay_results: list[list[DelayResult]],
) -> dict[str, dict[str, float | int]]:
    """计算各 delay 的 traded 率统计。

    Returns
    -------
    dict[str, dict]
        key 为 delay_label，value 含 total, traded, traded_rate。
    """
    stats: dict[str, dict[str, int]] = {}
    for label in ["D0", "D5", "D10", "D15", "pullback"]:
        stats[label] = {"total": 0, "traded": 0}

    for event_delays in delay_results:
        for dr in event_delays:
            if dr.delay_label in stats:
                stats[dr.delay_label]["total"] += 1
                if dr.traded:
                    stats[dr.delay_label]["traded"] += 1

    result: dict[str, dict[str, float | int]] = {}
    for label, counts in stats.items():
        total = counts["total"]
        traded = counts["traded"]
        result[label] = {
            "total": total,
            "traded": traded,
            "traded_rate": traded / total if total > 0 else 0.0,
        }
    return result


def main() -> None:
    """执行统一框架完整 pipeline。

    步骤：
    1. build_event_pool() → events, bars_cache, splits
    2. simulate_all_delays() → delay_results
    3. generate_3regime_labels() → labels
    4. extract_features() + impute_features() → feature matrices
    5. train_and_select() → timing classifier + predictions
    6. train_rf_probability() → RF model + probabilities
    7. compute_speed_gate() → gate flags
    8. compute_combined_positions() → unified_trades
    9. run_robustness() → bootstrap, forward, ablation
    10. generate_report() → output files
    """
    # --- Setup ---
    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
    )
    np.random.seed(RANDOM_STATE)

    pipeline_start = time.time()
    logger.info("=" * 70)
    logger.info("Timing-Probability Unified Framework — Pipeline Start")
    logger.info("random_state=%d, output_dir=%s", RANDOM_STATE, OUTPUT_DIR)
    logger.info("=" * 70)

    # Ensure output directory exists
    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)

    # Collect pipeline errors
    all_errors: list[PipelineError] = []

    # ======================================================================
    # Step 1: build_event_pool() → events, bars_cache, splits
    # ======================================================================
    logger.info("[Step 1/10] Building event pool...")
    step1_start = time.time()

    pool_result = build_event_pool(
        seed_source="canonical",
        forward_start="2025-11-01",
    )

    events_pool = pool_result.events_pool
    train_events = pool_result.train_events
    test_events = pool_result.test_events
    forward_events = pool_result.forward_events
    bars_cache = pool_result.bars_cache
    event_pool_stats = pool_result.stats

    step1_time = time.time() - step1_start
    logger.info(
        "[Step 1/10] Done: %d events (train=%d, test=%d, forward=%d) in %.1fs",
        len(events_pool),
        len(train_events),
        len(test_events),
        len(forward_events),
        step1_time,
    )

    # --- Step 1.5: 补全缺失特征（从 1s bar cache 计算）---
    logger.info("[Step 1.5] Filling missing features from 1s bar cache...")
    step1_5_start = time.time()
    events_pool = fill_missing_features(events_pool, bars_cache)
    # 同步更新 train/test splits（重新 split 以保持一致性）
    from timing_probability_unified.event_source_builder import split_events_by_time
    train_events, test_events, forward_events = split_events_by_time(
        events_pool, forward_start="2025-11-01", train_ratio=0.6
    )
    step1_5_time = time.time() - step1_5_start
    logger.info("[Step 1.5] Done in %.1fs", step1_5_time)

    # --- Step 1.6: ETH-only filter + cost_q50_cut050 sizing ---
    # worktree 最新结论：BTC 在 hard slippage 下只贡献 +0.09% 但增加 8 个负 SM
    # 去掉 BTC 后 neg SM 从 11 降到 3。
    logger.info("[Step 1.6] Applying ETH-only filter + cost sizing...")

    # ETH-only: 只保留 ETHUSDT 事件
    eth_mask = events_pool["symbol"] == "ETHUSDT"
    events_pool = events_pool[eth_mask].reset_index(drop=True)
    logger.info("  ETH-only filter: %d → %d events", int(eth_mask.sum()) + int((~eth_mask).sum()), len(events_pool))

    # 重新 split
    train_events, test_events, forward_events = split_events_by_time(
        events_pool, forward_start="2025-11-01", train_ratio=0.6
    )
    logger.info(
        "  After ETH-only: train=%d, test=%d, forward=%d",
        len(train_events), len(test_events), len(forward_events),
    )

    # cost_q50_cut050: 计算 train set 的 roundtrip_cost_atr q50 阈值
    # 高于该阈值的事件在后续 sizing 时乘以 0.5
    if "roundtrip_cost_atr" in train_events.columns:
        cost_q50_threshold = float(train_events["roundtrip_cost_atr"].quantile(0.50))
        logger.info("  cost_q50 threshold: %.6f", cost_q50_threshold)
        # 标记 high-cost events（后续在 sizing multiplier 中使用）
        events_pool = events_pool.copy()
        events_pool["_cost_penalty"] = np.where(
            events_pool["roundtrip_cost_atr"] >= cost_q50_threshold, 0.5, 1.0
        )
        # 同步到 splits
        train_events, test_events, forward_events = split_events_by_time(
            events_pool, forward_start="2025-11-01", train_ratio=0.6
        )
        high_cost_count = int((events_pool["_cost_penalty"] == 0.5).sum())
        logger.info("  cost_q50_cut050: %d events at half-size, %d at full-size",
                    high_cost_count, len(events_pool) - high_cost_count)
    else:
        cost_q50_threshold = None
        events_pool["_cost_penalty"] = 1.0
        logger.warning("  roundtrip_cost_atr not available, skipping cost sizing")

    # ======================================================================
    # Step 2: simulate_all_delays() → delay_results
    # ======================================================================
    logger.info("[Step 2/10] Simulating all delays for full-window events...")
    step2_start = time.time()

    # Override trail_start_r from 0.9 → 1.5 (worktree validated +4pp improvement)
    _original_trail_start = DEFAULT_EXEC_PARAMS["trail_start_r"]
    DEFAULT_EXEC_PARAMS["trail_start_r"] = 1.5
    logger.info("  trail_start_r override: %.1f → %.1f", _original_trail_start, 1.5)

    # Combine train + test for full-window delay simulation
    full_window_events = pd.concat(
        [train_events, test_events], ignore_index=True
    )

    delay_results_full, delay_errors = _simulate_delays_for_events(
        full_window_events, bars_cache
    )
    all_errors.extend(delay_errors)

    # Split delay_results back into train and test
    n_train = len(train_events)
    n_test = len(test_events)
    delay_results_train = delay_results_full[:n_train]
    delay_results_test = delay_results_full[n_train:]

    step2_time = time.time() - step2_start
    logger.info(
        "[Step 2/10] Done: %d events simulated (%d errors) in %.1fs",
        len(delay_results_full),
        len(delay_errors),
        step2_time,
    )

    # Restore trail_start_r
    DEFAULT_EXEC_PARAMS["trail_start_r"] = _original_trail_start

    # ======================================================================
    # Step 3: generate_3regime_labels() → labels
    # ======================================================================
    logger.info("[Step 3/10] Generating 3-regime labels...")
    step3_start = time.time()

    train_labels = generate_3regime_labels(delay_results_train)
    test_labels = generate_3regime_labels(delay_results_test)

    step3_time = time.time() - step3_start
    logger.info(
        "[Step 3/10] Done: train labels=%s, test labels=%s in %.1fs",
        dict(train_labels.value_counts()),
        dict(test_labels.value_counts()),
        step3_time,
    )

    # ======================================================================
    # Step 4: extract_features() + impute_features() → feature matrices
    # ======================================================================
    logger.info("[Step 4/10] Extracting and imputing features...")
    step4_start = time.time()

    train_features_raw, used_features, excluded_features = extract_features(
        train_events
    )
    test_features_raw, _, _ = extract_features(test_events)

    # Ensure both have the same columns (use used_features from train)
    # Test may have different columns available; align to train's used_features
    for col in used_features:
        if col not in test_features_raw.columns:
            test_features_raw[col] = np.nan
    test_features_raw = test_features_raw[used_features]

    # Impute missing values (train medians applied to both)
    train_features, test_features, imputation_stats = impute_features(
        train_features_raw, test_features_raw
    )

    step4_time = time.time() - step4_start
    logger.info(
        "[Step 4/10] Done: %d features used, %d excluded in %.1fs",
        len(used_features),
        len(excluded_features),
        step4_time,
    )

    # ======================================================================
    # Step 5: train_and_select() → timing classifier + predictions
    # ======================================================================
    logger.info("[Step 5/10] Training timing classifier (DT3/DT4 + LOOCV)...")
    step5_start = time.time()

    timing_result = train_and_select(
        train_features=train_features,
        train_labels=train_labels,
        delay_results_train=delay_results_train,
        test_features=test_features,
        test_labels=test_labels,
        delay_results_test=delay_results_test,
        train_events=train_events,
        test_events=test_events,
    )

    step5_time = time.time() - step5_start
    logger.info(
        "[Step 5/10] Done: selected DT%d (DT3 LOOCV=%.4f, DT4 LOOCV=%.4f, "
        "test CS=%.4f) in %.1fs",
        timing_result.selected_depth,
        timing_result.dt3_loocv_calendar_sum,
        timing_result.dt4_loocv_calendar_sum,
        timing_result.test_calendar_sum,
        step5_time,
    )

    # Get full-window predictions (train + test combined)
    full_window_predictions = np.concatenate(
        [timing_result.train_predictions, timing_result.test_predictions]
    )

    # ======================================================================
    # Step 6: train_rf_probability() → RF model + probabilities
    # ======================================================================
    logger.info("[Step 6/10] Training RF probability model...")
    step6_start = time.time()

    # Generate binary labels for RF training
    # For each event: get the delay pnl corresponding to its timing prediction
    from timing_probability_unified.timing_classifier import get_selected_delay_pnl

    train_delay_pnls = pd.Series(
        [
            get_selected_delay_pnl(timing_result.train_predictions[i], delay_results_train[i])[1]
            for i in range(n_train)
        ],
        dtype=float,
    )
    test_delay_pnls = pd.Series(
        [
            get_selected_delay_pnl(timing_result.test_predictions[i], delay_results_test[i])[1]
            for i in range(n_test)
        ],
        dtype=float,
    )

    train_rf_labels = generate_rf_binary_labels(
        pd.Series(timing_result.train_predictions), train_delay_pnls
    )
    test_rf_labels = generate_rf_binary_labels(
        pd.Series(timing_result.test_predictions), test_delay_pnls
    )

    rf_result = train_rf_probability(
        train_features=train_features,
        train_labels=train_rf_labels,
        test_features=test_features,
        test_labels=test_rf_labels,
        n_estimators=200,
        random_state=RANDOM_STATE,
    )

    step6_time = time.time() - step6_start
    logger.info(
        "[Step 6/10] Done: train AUC=%.4f, test AUC=%.4f, "
        "no_signal_warning=%s in %.1fs",
        rf_result.train_auc,
        rf_result.test_auc,
        rf_result.rf_no_signal_warning,
        step6_time,
    )

    # Compute sizing multipliers for full-window
    full_window_probabilities = np.concatenate(
        [rf_result.train_probabilities, rf_result.test_probabilities]
    )
    full_window_multipliers = compute_sizing_multiplier(full_window_probabilities)

    # If RF has no signal, degrade to uniform sizing (multiplier=1.0)
    if rf_result.rf_no_signal_warning:
        logger.warning(
            "RF test AUC < 0.50 — degrading to uniform sizing (multiplier=1.0)"
        )
        full_window_multipliers = np.ones(len(full_window_events), dtype=np.float64)

    # Apply cost_q50_cut050: high-cost events get multiplier × 0.5
    if "_cost_penalty" in full_window_events.columns:
        cost_penalty = full_window_events["_cost_penalty"].values
        full_window_multipliers = full_window_multipliers * cost_penalty
        logger.info(
            "  Applied cost_q50_cut050: %d events at half-size",
            int((cost_penalty < 1.0).sum()),
        )

    # ======================================================================
    # Step 7: compute_speed_gate() → gate flags
    # ======================================================================
    logger.info("[Step 7/10] Computing speed gate...")
    step7_start = time.time()

    speed_gate_pass, speed_gate_threshold = compute_speed_gate(
        events=full_window_events,
        train_events=train_events,
        quantile=0.10,
    )

    step7_time = time.time() - step7_start
    logger.info(
        "[Step 7/10] Done: threshold=%.6f, pass_rate=%.1f%% in %.1fs",
        speed_gate_threshold,
        np.mean(speed_gate_pass) * 100,
        step7_time,
    )

    # ======================================================================
    # Step 8: compute_combined_positions() → unified_trades
    # ======================================================================
    logger.info("[Step 8/10] Computing combined positions...")
    step8_start = time.time()

    from timing_probability_unified.combined_executor import CombinedPositionConfig
    eth_config = CombinedPositionConfig(base_notional_share=0.80)

    unified_trades = compute_combined_positions(
        events=full_window_events,
        timing_predictions=full_window_predictions,
        sizing_multipliers=full_window_multipliers,
        delay_results=delay_results_full,
        speed_gate_pass=speed_gate_pass,
        config=eth_config,
    )

    # Compute key metrics
    calendar_sum_gate_on = compute_calendar_sum(unified_trades, gate_filter=True)
    calendar_sum_gate_off = compute_calendar_sum(unified_trades, gate_filter=False)
    worst_sm_gate_on = compute_worst_sm(unified_trades, gate_filter=True)

    # Speed gate analysis
    speed_gate_result = analyze_speed_gate(
        trades=unified_trades,
        speed_gate_pass=speed_gate_pass,
        threshold=speed_gate_threshold,
    )

    # Sensitivity analysis
    sensitivity_rows = run_sensitivity_analysis(
        events=full_window_events,
        timing_predictions=full_window_predictions,
        sizing_multipliers=full_window_multipliers,
        delay_results=delay_results_full,
        speed_gate_pass=speed_gate_pass,
    )

    step8_time = time.time() - step8_start
    logger.info(
        "[Step 8/10] Done: calendar_sum(gate ON)=%.4f, "
        "calendar_sum(gate OFF)=%.4f, worst_sm=%.4f in %.1fs",
        calendar_sum_gate_on,
        calendar_sum_gate_off,
        worst_sm_gate_on,
        step8_time,
    )

    # --- Step 8.5: Adverse fill 压力测试 ---
    logger.info("[Step 8.5] Running adverse fill stress matrix...")
    step8_5_start = time.time()
    from timing_probability_unified.adverse_fill import (  # noqa: E402
        evaluate_fill_scenarios,
        STANDARD_FILL_SCENARIOS,
    )

    # Full-window adverse fill 矩阵
    adverse_fill_full = evaluate_fill_scenarios(
        delay_results=delay_results_full,
        events=full_window_events,
        bars_cache=bars_cache,
        timing_predictions=full_window_predictions,
        sizing_multipliers=full_window_multipliers,
        speed_gate_pass=speed_gate_pass,
        base_share=0.80,  # ETH-only allocation (worktree lead)
    )
    logger.info("Full-window adverse fill matrix:")
    for _, row in adverse_fill_full.iterrows():
        logger.info(
            "  %-30s cs_on=%.4f wsm=%.4f neg_sm=%d btc=%.4f eth=%.4f",
            row["scenario"],
            row["calendar_sum_gate_on"],
            row["worst_sm_gate_on"],
            row["neg_sm_count"],
            row["btc_cs_gate_on"],
            row["eth_cs_gate_on"],
        )

    # 保存到 output dir
    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)
    adverse_fill_full.to_csv(OUTPUT_DIR / "adverse_fill_full_window.csv", index=False)
    step8_5_time = time.time() - step8_5_start
    logger.info("[Step 8.5] Done in %.1fs", step8_5_time)

    # ======================================================================
    # Step 9: run_robustness() → bootstrap, forward, ablation
    # ======================================================================
    logger.info("[Step 9/10] Running robustness validation...")
    step9_start = time.time()

    # Bootstrap (on gate-ON trades only)
    gate_on_trades = unified_trades[unified_trades["speed_gate_pass"] == True].copy()  # noqa: E712
    bootstrap_result = run_bootstrap(
        trades=gate_on_trades,
        n_resamples=1000,
        random_state=RANDOM_STATE,
    )

    # Forward validation
    forward_result = run_forward_validation(
        forward_events=forward_events,
        timing_classifier=timing_result.classifier,
        rf_model=rf_result.model,
        speed_gate_threshold=speed_gate_threshold,
        bars_cache=bars_cache,
        full_window_calendar_sum=calendar_sum_gate_on,
    )

    # Ablation study
    ablation_rows = run_ablation_study(
        events=full_window_events,
        timing_predictions=full_window_predictions,
        sizing_multipliers=full_window_multipliers,
        delay_results=delay_results_full,
        speed_gate_pass=speed_gate_pass,
    )

    robustness_result = RobustnessResult(
        bootstrap=bootstrap_result,
        forward=forward_result,
        ablation=ablation_rows,
    )

    step9_time = time.time() - step9_start
    logger.info(
        "[Step 9/10] Done: bootstrap CI=[%.4f, %.4f], "
        "forward CS=%.4f, ablation configs=%d in %.1fs",
        bootstrap_result.calendar_sum_ci_5,
        bootstrap_result.calendar_sum_ci_95,
        forward_result.forward_calendar_sum,
        len(ablation_rows),
        step9_time,
    )

    # ======================================================================
    # Step 10: generate_report() → output files
    # ======================================================================
    logger.info("[Step 10/10] Generating report...")
    step10_start = time.time()

    # Build execution_stats
    pipeline_total_time = time.time() - pipeline_start
    delay_traded_stats = _compute_delay_traded_stats(delay_results_full)

    execution_stats: dict = {
        "random_state": RANDOM_STATE,
        "total_events": len(events_pool),
        "full_window_events": len(full_window_events),
        "train_events": n_train,
        "test_events": n_test,
        "forward_events": len(forward_events),
        "skipped_events": len(pool_result.skipped_events),
        "delay_simulation_errors": len(delay_errors),
        "delay_traded_stats": delay_traded_stats,
        "features_used": used_features,
        "features_excluded": excluded_features,
        "timing_selected_depth": timing_result.selected_depth,
        "rf_test_auc": rf_result.test_auc,
        "rf_no_signal_warning": rf_result.rf_no_signal_warning,
        "speed_gate_threshold": speed_gate_threshold,
        "speed_gate_pass_rate": float(np.mean(speed_gate_pass)),
        "calendar_sum_gate_on": calendar_sum_gate_on,
        "calendar_sum_gate_off": calendar_sum_gate_off,
        "worst_sm_gate_on": worst_sm_gate_on,
        "pipeline_total_time_seconds": pipeline_total_time,
        "step_times": {
            "step1_event_pool": step1_time,
            "step2_delay_sim": step2_time,
            "step3_labels": step3_time,
            "step4_features": step4_time,
            "step5_timing_classifier": step5_time,
            "step6_rf_probability": step6_time,
            "step7_speed_gate": step7_time,
            "step8_combined_positions": step8_time,
            "step9_robustness": step9_time,
            "step10_report": 0.0,  # will be updated after report generation
        },
        "pipeline_errors": [
            {
                "event_id": e.event_id,
                "stage": e.stage,
                "error_type": e.error_type,
                "error_message": e.error_message,
                "action_taken": e.action_taken,
            }
            for e in all_errors
        ],
    }

    # Generate report (writes all 13 output files)
    decision = generate_report(
        output_dir=OUTPUT_DIR,
        trades=unified_trades,
        timing_result=timing_result,
        rf_result=rf_result,
        speed_gate_result=speed_gate_result,
        robustness_result=robustness_result,
        sensitivity_rows=sensitivity_rows,
        event_pool_stats=event_pool_stats,
        events_pool=events_pool,
        execution_stats=execution_stats,
    )

    # Save skipped_events.csv
    if not pool_result.skipped_events.empty:
        pool_result.skipped_events.to_csv(
            OUTPUT_DIR / "skipped_events.csv", index=False
        )
        logger.info("Written: skipped_events.csv (%d events)", len(pool_result.skipped_events))

    step10_time = time.time() - step10_start
    # Update step10 time in execution_stats (already written to file, but log it)
    execution_stats["step_times"]["step10_report"] = step10_time

    pipeline_total_time = time.time() - pipeline_start
    logger.info(
        "[Step 10/10] Done in %.1fs. Total pipeline time: %.1fs",
        step10_time,
        pipeline_total_time,
    )

    # ======================================================================
    # Summary
    # ======================================================================
    logger.info("=" * 70)
    logger.info("Pipeline Complete — Go/No-Go Decision: %s", decision.decision.upper())
    logger.info("  Calendar Sum (gate ON): %.4f (%.2f%%)", calendar_sum_gate_on, calendar_sum_gate_on * 100)
    logger.info("  Worst SM (gate ON): %.4f (%.2f%%)", worst_sm_gate_on, worst_sm_gate_on * 100)
    logger.info("  Forward CS: %.4f (%.2f%%)", forward_result.forward_calendar_sum, forward_result.forward_calendar_sum * 100)
    logger.info("  Bootstrap CI: [%.4f, %.4f]", bootstrap_result.calendar_sum_ci_5, bootstrap_result.calendar_sum_ci_95)
    logger.info("  Total time: %.1fs", pipeline_total_time)
    logger.info("  Output: %s", OUTPUT_DIR)
    logger.info("=" * 70)


if __name__ == "__main__":
    main()
