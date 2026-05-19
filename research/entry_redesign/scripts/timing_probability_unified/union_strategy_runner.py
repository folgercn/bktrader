"""union_strategy_runner — T2+T3 union 事件池顶层 orchestrator

串联 T2/T3 事件生成 → quality filter → 互斥验证 → 独立训练 → full-size concat → 报告。

用法：
    python -m timing_probability_unified.union_strategy_runner \
        --forward-start 2026-02-01

    # Rolling window mode
    python -m timing_probability_unified.union_strategy_runner --rolling

输出：
    research/entry_redesign/scripts/output/multi_timeframe/
      ├── union_strategy_report.json
      └── union_strategy_report.md
"""

from __future__ import annotations

import json
import logging
import sys
import time
from dataclasses import dataclass, field, replace
from datetime import datetime
from pathlib import Path

import numpy as np
import pandas as pd

# Ensure parent scripts dir is importable
_SCRIPTS_DIR = Path(__file__).resolve().parents[1]
if str(_SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS_DIR))

from timing_probability_unified.event_source_builder import (
    build_event_pool,
    split_events_by_time,
)
from timing_probability_unified.t3_event_generator import (
    T3EventConfig,
    generate_t3_events,
)
from timing_probability_unified.quality_filter import (
    QualityFilterConfig,
    apply_t3_quality_filter,
)
from timing_probability_unified.multi_timeframe_builder import (
    load_all_1s_bars,
)
from timing_probability_unified.multi_timeframe_runner import (
    _load_bars_cache_for_symbol,
    _simulate_delays_for_timeframe,
)
from timing_probability_unified.timing_classifier import (
    generate_3regime_labels,
    train_and_select,
    get_selected_delay_pnl,
)
from timing_probability_unified.probability_model import (
    compute_sizing_multiplier,
    generate_rf_binary_labels,
    train_rf_probability,
)
from timing_probability_unified.combined_executor import (
    CombinedPositionConfig,
    compute_calendar_sum,
    compute_combined_positions,
    compute_worst_sm,
)
from timing_probability_unified.speed_gate import compute_speed_gate
from pre_breakout_timing.feature_extractor import extract_features, impute_features
from dynamic_timing.execution_sim import DEFAULT_EXEC_PARAMS

logger = logging.getLogger(__name__)

RANDOM_STATE = 42
MIN_TRAIN_EVENTS = 30


# ---------------------------------------------------------------------------
# Config & Result dataclasses
# ---------------------------------------------------------------------------


@dataclass
class UnionStrategyConfig:
    """Union strategy 配置"""

    # Pipeline params
    base_notional_share: float = 0.80
    speed_gate_quantile: float = 0.10
    random_state: int = 42
    t3_pre_touch_max: float = 900.0

    # Rolling window
    forward_start: str = "2026-02-01"
    rolling_months: list[str] | None = None  # None = single window

    # Execution
    trail_start_r: float = 1.5
    max_hold_hours: float = 2.0


@dataclass
class PoolPipelineResult:
    """单池 pipeline 结果"""

    pool_name: str
    total_events: int
    train_events: int
    test_events: int
    forward_events: int
    forward_trades: pd.DataFrame
    calendar_sum: float
    worst_sm: float
    trade_count: int
    neg_sm_count: int
    timing_selected_depth: int
    timing_loocv_cs: float
    rf_test_auc: float
    skipped: bool = False
    skip_reason: str = ""
    all_trades: pd.DataFrame = field(default_factory=pd.DataFrame)


@dataclass
class UnionStrategyResult:
    """Union strategy 完整结果"""

    config: UnionStrategyConfig
    t2_result: PoolPipelineResult
    t3_result: PoolPipelineResult
    union_forward_trades: pd.DataFrame
    union_calendar_sum: float
    union_worst_sm: float
    union_trade_count: int
    union_neg_sm_count: int
    t2_event_count: int
    t3_event_count: int
    t3_filter_params: dict
    pipeline_seconds: float


@dataclass
class RollingWindowResult:
    """滚动窗口汇总结果"""

    config: UnionStrategyConfig
    window_results: list[UnionStrategyResult] = field(default_factory=list)
    combined_forward_trades: pd.DataFrame = field(default_factory=pd.DataFrame)
    total_forward_cs: float = 0.0
    total_worst_sm: float = 0.0
    total_trade_count: int = 0
    monthly_attribution: list[dict] = field(default_factory=list)


# ---------------------------------------------------------------------------
# 互斥性验证
# ---------------------------------------------------------------------------


def validate_mutual_exclusion(
    t2_events: pd.DataFrame,
    t3_events: pd.DataFrame,
) -> bool:
    """验证 T2 和 T3 事件在同 signal_start + 同 side 不共存。

    T2 和 T3 结构条件在同一根 signal bar 内同方向互斥，
    此函数用于运行时验证该假设。

    Parameters
    ----------
    t2_events : pd.DataFrame
        T2 事件池，需含 touch_time 和 side 列。
    t3_events : pd.DataFrame
        T3 事件池，需含 touch_time 和 side 列。

    Returns
    -------
    bool
        True 如果验证通过（无冲突）。

    Raises
    ------
    ValueError
        如果发现同 signal bar + 同 side 有 T2 和 T3 共存。
    """
    if t2_events.empty or t3_events.empty:
        return True

    # 使用 touch_time 按小时截断作为 signal bar 代理
    # （同一根 1h bar 内的事件 signal_start 相同）
    t2_keys = set()
    for _, row in t2_events.iterrows():
        tt = pd.Timestamp(row["touch_time"])
        bar_start = tt.floor("1h")
        t2_keys.add((bar_start, row["side"]))

    conflicts = []
    for _, row in t3_events.iterrows():
        tt = pd.Timestamp(row["touch_time"])
        bar_start = tt.floor("1h")
        key = (bar_start, row["side"])
        if key in t2_keys:
            conflicts.append(
                f"  signal_start={bar_start}, side={row['side']}, "
                f"t3_event_id={row.get('event_id', 'N/A')}"
            )

    if conflicts:
        detail = "\n".join(conflicts[:20])
        raise ValueError(
            f"T2/T3 互斥性验证失败: 发现 {len(conflicts)} 个冲突\n"
            f"冲突详情（最多显示 20 条）:\n{detail}"
        )

    logger.info("互斥性验证通过: T2=%d events, T3=%d events, 无冲突",
                len(t2_events), len(t3_events))
    return True


# ---------------------------------------------------------------------------
# Pool tagging
# ---------------------------------------------------------------------------


def _tag_pool(
    trades: pd.DataFrame,
    pool_name: str,
) -> pd.DataFrame:
    """Tag a pool without changing sizing.

    T2 and T3 are mutually exclusive signal structures. Each accepted event
    keeps the full research-lead sizing semantics from its own probability
    model; concat should not sleeve-scale the final position.
    """
    tagged = trades.copy()
    if "pool" not in tagged.columns:
        tagged["pool"] = pd.Series(dtype="object")
    if not tagged.empty:
        tagged["pool"] = pool_name
    return tagged


def _active_trade_count(trades: pd.DataFrame) -> int:
    """Count trades that passed timing and speed gates."""
    if trades.empty:
        return 0
    active = trades[
        (trades["timing_prediction"] != "skip")
        & (trades["speed_gate_pass"] == True)  # noqa: E712
    ]
    return len(active)


def _negative_month_count(trades: pd.DataFrame) -> int:
    """Count negative calendar months in a trade ledger."""
    if trades.empty:
        return 0
    gate_on = trades[trades["speed_gate_pass"] == True]  # noqa: E712
    if gate_on.empty:
        return 0
    gate_on = gate_on.copy()
    gate_on["year_month"] = pd.to_datetime(gate_on["touch_time"]).dt.to_period("M")
    monthly = gate_on.groupby("year_month")["weighted_pnl"].sum()
    return int((monthly < 0).sum())


def _slice_trades_to_window(
    trades: pd.DataFrame,
    window_start: pd.Timestamp,
    window_end: pd.Timestamp,
) -> pd.DataFrame:
    """Keep trades in [window_start, window_end)."""
    if trades.empty:
        return trades.copy()
    sliced = trades.copy()
    touch_time = pd.to_datetime(sliced["touch_time"], utc=True)
    mask = (touch_time >= window_start) & (touch_time < window_end)
    return sliced[mask].reset_index(drop=True)


def _slice_pool_result_to_window(
    pool_result: PoolPipelineResult,
    window_start: pd.Timestamp,
    window_end: pd.Timestamp,
) -> PoolPipelineResult:
    """Return a pool result with forward trades/metrics restricted to one window."""
    sliced_trades = _slice_trades_to_window(
        pool_result.forward_trades, window_start, window_end
    )
    return replace(
        pool_result,
        forward_events=len(sliced_trades),
        forward_trades=sliced_trades,
        calendar_sum=compute_calendar_sum(sliced_trades, gate_filter=True),
        worst_sm=compute_worst_sm(sliced_trades, gate_filter=True),
        trade_count=_active_trade_count(sliced_trades),
        neg_sm_count=_negative_month_count(sliced_trades),
    )


def _slice_result_to_window(
    result: UnionStrategyResult,
    window_start: pd.Timestamp,
    window_end: pd.Timestamp,
) -> UnionStrategyResult:
    """Restrict a union result to a single rolling forward month."""
    t2_result = _slice_pool_result_to_window(result.t2_result, window_start, window_end)
    t3_result = _slice_pool_result_to_window(result.t3_result, window_start, window_end)
    union_trades = _slice_trades_to_window(
        result.union_forward_trades, window_start, window_end
    )

    return replace(
        result,
        t2_result=t2_result,
        t3_result=t3_result,
        union_forward_trades=union_trades,
        union_calendar_sum=compute_calendar_sum(union_trades, gate_filter=True),
        union_worst_sm=compute_worst_sm(union_trades, gate_filter=True),
        union_trade_count=_active_trade_count(union_trades),
        union_neg_sm_count=_negative_month_count(union_trades),
    )


def _next_month_start(month_start: str | pd.Timestamp) -> pd.Timestamp:
    """Return the next UTC month boundary for a rolling forward window."""
    ts = pd.Timestamp(month_start)
    if ts.tzinfo is None:
        ts = ts.tz_localize("UTC")
    else:
        ts = ts.tz_convert("UTC")
    return ts + pd.DateOffset(months=1)


# ---------------------------------------------------------------------------
# 单池 pipeline
# ---------------------------------------------------------------------------


def run_single_pool_pipeline(
    events: pd.DataFrame,
    bars_cache: dict[str, pd.DataFrame],
    forward_start: str,
    max_hold_hours: float,
    pool_name: str = "pool",
    random_state: int = RANDOM_STATE,
    base_notional_share: float = 0.80,
    speed_gate_quantile: float = 0.10,
    trail_start_r: float = 1.5,
) -> PoolPipelineResult:
    """对单个事件池执行完整 pipeline。

    流程: split → delay_sim → timing → RF → speed_gate → combined_positions

    Parameters
    ----------
    events : pd.DataFrame
        事件池 DataFrame（已过滤）。
    bars_cache : dict[str, pd.DataFrame]
        按 "{symbol}_{YYYYMM}" 索引的 1s bar cache。
    forward_start : str
        Forward split 起始日期。
    max_hold_hours : float
        最大持仓时间。
    pool_name : str
        池名称（用于日志）。
    random_state : int
        随机种子。
    base_notional_share : float
        基础仓位份额。
    speed_gate_quantile : float
        Speed gate 分位数。
    trail_start_r : float
        Trailing stop 启动 R 值。

    Returns
    -------
    PoolPipelineResult
        包含 forward trades DataFrame 和模型指标。
    """
    logger.info("=" * 50)
    logger.info("run_single_pool_pipeline: %s (%d events)", pool_name, len(events))
    logger.info("=" * 50)

    # Split
    train_events, test_events, forward_events = split_events_by_time(
        events, forward_start=forward_start, train_ratio=0.6
    )
    logger.info("Split: train=%d, test=%d, forward=%d",
                len(train_events), len(test_events), len(forward_events))

    # 训练样本不足保护
    n_train = len(train_events)
    n_test = len(test_events)

    if n_train < MIN_TRAIN_EVENTS:
        logger.warning(
            "[%s] 训练样本不足: train=%d < %d，跳过训练，forward trades 为空",
            pool_name, n_train, MIN_TRAIN_EVENTS,
        )
        return PoolPipelineResult(
            pool_name=pool_name,
            total_events=len(events),
            train_events=n_train,
            test_events=n_test,
            forward_events=len(forward_events),
            forward_trades=pd.DataFrame(),
            all_trades=pd.DataFrame(),
            calendar_sum=0.0,
            worst_sm=0.0,
            trade_count=0,
            neg_sm_count=0,
            timing_selected_depth=0,
            timing_loocv_cs=0.0,
            rf_test_auc=0.0,
            skipped=True,
            skip_reason=f"训练样本不足 (train={n_train} < {MIN_TRAIN_EVENTS})",
        )

    # Override trail_start_r
    original_trail_start = DEFAULT_EXEC_PARAMS.get("trail_start_r", 0.9)
    DEFAULT_EXEC_PARAMS["trail_start_r"] = trail_start_r

    # Delay simulation on ALL events (train + test + forward)
    # We need forward events in the pipeline to produce forward trades.
    n_forward = len(forward_events)
    all_events = pd.concat([train_events, test_events, forward_events], ignore_index=True)
    logger.info("Running delay simulation for %d all events (train+test+forward)...",
                len(all_events))
    delay_results_all = _simulate_delays_for_timeframe(
        all_events, bars_cache, max_hold_hours
    )
    delay_results_train = delay_results_all[:n_train]
    delay_results_test = delay_results_all[n_train:n_train + n_test]
    delay_results_forward = delay_results_all[n_train + n_test:]

    # Restore trail_start_r
    DEFAULT_EXEC_PARAMS["trail_start_r"] = original_trail_start

    # Generate 3-regime labels (train + test for model training)
    train_labels = generate_3regime_labels(delay_results_train)
    test_labels = generate_3regime_labels(delay_results_test)

    # Extract features
    train_features_raw, used_features, _ = extract_features(train_events)
    test_features_raw, _, _ = extract_features(test_events)
    forward_features_raw, _, _ = extract_features(forward_events)

    # Align test/forward features to train columns
    for col in used_features:
        if col not in test_features_raw.columns:
            test_features_raw[col] = np.nan
        if col not in forward_features_raw.columns:
            forward_features_raw[col] = np.nan
    test_features_raw = test_features_raw[used_features]
    forward_features_raw = forward_features_raw[used_features]

    # Impute: fit on train, apply to test and forward
    train_features, test_features, _ = impute_features(train_features_raw, test_features_raw)
    _, forward_features, _ = impute_features(train_features_raw, forward_features_raw)

    # Train timing classifier
    logger.info("[%s] Training timing classifier...", pool_name)
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

    # Predict forward events using trained timing model
    if n_forward > 0 and timing_result.classifier is not None:
        forward_timing_preds = timing_result.classifier.predict(forward_features)
    else:
        forward_timing_preds = np.array(["skip"] * n_forward)

    all_predictions = np.concatenate(
        [timing_result.train_predictions, timing_result.test_predictions, forward_timing_preds]
    )

    # Train RF probability model
    logger.info("[%s] Training RF probability model...", pool_name)
    train_delay_pnls = pd.Series([
        get_selected_delay_pnl(timing_result.train_predictions[i], delay_results_train[i])[1]
        for i in range(n_train)
    ], dtype=float)
    test_delay_pnls = pd.Series([
        get_selected_delay_pnl(timing_result.test_predictions[i], delay_results_test[i])[1]
        for i in range(n_test)
    ], dtype=float)

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
        random_state=random_state,
    )

    # Predict forward probabilities using trained RF
    if n_forward > 0 and rf_result.model is not None:
        forward_rf_probs = rf_result.model.predict_proba(forward_features)[:, 1]
    else:
        forward_rf_probs = np.full(n_forward, 0.5)

    all_probabilities = np.concatenate(
        [rf_result.train_probabilities, rf_result.test_probabilities, forward_rf_probs]
    )
    all_multipliers = compute_sizing_multiplier(all_probabilities)

    if rf_result.rf_no_signal_warning:
        logger.warning("[%s] RF no signal — degrading to uniform sizing", pool_name)
        all_multipliers = np.ones(len(all_events), dtype=np.float64)

    # Speed gate (computed on all events, threshold from train)
    speed_gate_pass, speed_gate_threshold = compute_speed_gate(
        events=all_events,
        train_events=train_events,
        quantile=speed_gate_quantile,
    )
    logger.info("[%s] Speed gate threshold: %.6f", pool_name, speed_gate_threshold)

    # Combined positions on all events
    config = CombinedPositionConfig(base_notional_share=base_notional_share)
    unified_trades = compute_combined_positions(
        events=all_events,
        timing_predictions=all_predictions,
        sizing_multipliers=all_multipliers,
        delay_results=delay_results_all,
        speed_gate_pass=speed_gate_pass,
        config=config,
    )
    unified_trades["split"] = (
        ["train"] * n_train
        + ["test"] * n_test
        + ["forward"] * n_forward
    )

    # Compute forward trades (filter by touch_time >= forward_start)
    forward_ts = pd.Timestamp(forward_start, tz="UTC")
    unified_trades["touch_time"] = pd.to_datetime(unified_trades["touch_time"], utc=True)
    forward_trades = unified_trades[
        unified_trades["touch_time"] >= forward_ts
    ].reset_index(drop=True)

    # Metrics on forward trades
    cs = compute_calendar_sum(forward_trades, gate_filter=True)
    ws = compute_worst_sm(forward_trades, gate_filter=True)

    # Trade count: timing != skip AND speed_gate_pass
    active_forward = forward_trades[
        (forward_trades["timing_prediction"] != "skip")
        & (forward_trades["speed_gate_pass"] == True)  # noqa: E712
    ]
    trade_count = len(active_forward)

    # Neg SM count
    neg_sm = 0
    if not forward_trades.empty:
        gate_on = forward_trades[forward_trades["speed_gate_pass"] == True]  # noqa: E712
        if not gate_on.empty:
            gate_on = gate_on.copy()
            gate_on["year_month"] = pd.to_datetime(gate_on["touch_time"]).dt.to_period("M")
            monthly = gate_on.groupby("year_month")["weighted_pnl"].sum()
            neg_sm = int((monthly < 0).sum())

    logger.info("[%s] Forward results: CS=%.4f, worst_sm=%.4f, trades=%d",
                pool_name, cs, ws, trade_count)

    return PoolPipelineResult(
        pool_name=pool_name,
        total_events=len(events),
        train_events=n_train,
        test_events=n_test,
        forward_events=len(forward_events),
        forward_trades=forward_trades,
        all_trades=unified_trades,
        calendar_sum=cs,
        worst_sm=ws,
        trade_count=trade_count,
        neg_sm_count=neg_sm,
        timing_selected_depth=timing_result.selected_depth,
        timing_loocv_cs=timing_result.dt3_loocv_calendar_sum,
        rf_test_auc=rf_result.test_auc,
    )


# ---------------------------------------------------------------------------
# 主函数
# ---------------------------------------------------------------------------


def run_union_strategy(config: UnionStrategyConfig) -> UnionStrategyResult:
    """执行 T2+T3 union strategy pipeline。

    流程:
    1. 加载 T2 canonical events (ETH-only)
    2. 生成 T3 events 并应用 quality filter
    3. 验证互斥性
    4. 对 T2/T3 各自独立调用 run_single_pool_pipeline
    5. full-size concat forward trades
    6. 返回 UnionStrategyResult

    Parameters
    ----------
    config : UnionStrategyConfig
        Union strategy 配置。

    Returns
    -------
    UnionStrategyResult
        包含 T2、T3 各自结果和 union 合并结果。
    """
    pipeline_start = time.time()

    logger.info("=" * 60)
    logger.info("run_union_strategy")
    logger.info(
        "  forward_start=%s, sizing=full-size concat with per-pool probability multiplier, "
        "t3_pre_touch_max=%.0f",
        config.forward_start,
        config.t3_pre_touch_max,
    )
    logger.info("=" * 60)

    # --- Step 1: 加载 T2 canonical events (ETH-only) ---
    logger.info("Step 1: Loading T2 canonical events...")
    t2_pool = build_event_pool(
        seed_source="canonical",
        forward_start=config.forward_start,
    )
    # ETH-only filter
    t2_events = t2_pool.events_pool[
        t2_pool.events_pool["symbol"] == "ETHUSDT"
    ].reset_index(drop=True)
    logger.info("T2 events (ETH-only): %d", len(t2_events))

    # --- Step 2: 生成 T3 events + quality filter ---
    logger.info("Step 2: Generating T3 events...")
    bars_1s = load_all_1s_bars("ETHUSDT")
    t3_config = T3EventConfig(symbol="ETHUSDT")
    t3_all_events = generate_t3_events(bars_1s, t3_config)
    logger.info("T3 raw events: %d", len(t3_all_events))

    # T3 quality filter: 需要 train set 来计算阈值
    # 先 split T3 events 获取 train set
    if not t3_all_events.empty:
        t3_train, _, _ = split_events_by_time(
            t3_all_events, forward_start=config.forward_start, train_ratio=0.6
        )
        qf_config = QualityFilterConfig(t3_pre_touch_max=config.t3_pre_touch_max)
        t3_events, t3_filter_params = apply_t3_quality_filter(
            t3_all_events, t3_train, qf_config
        )
    else:
        t3_events = t3_all_events
        t3_filter_params = {}

    logger.info("T3 events after quality filter: %d", len(t3_events))

    # --- Step 3: 验证互斥性 ---
    logger.info("Step 3: Validating mutual exclusion...")
    validate_mutual_exclusion(t2_events, t3_events)

    # --- Step 4: 独立训练 ---
    logger.info("Step 4: Loading bars cache...")
    bars_cache = _load_bars_cache_for_symbol("ETHUSDT")

    logger.info("Step 4a: Running T2 pipeline...")
    t2_result = run_single_pool_pipeline(
        events=t2_events,
        bars_cache=bars_cache,
        forward_start=config.forward_start,
        max_hold_hours=config.max_hold_hours,
        pool_name="T2",
        random_state=config.random_state,
        base_notional_share=config.base_notional_share,
        speed_gate_quantile=config.speed_gate_quantile,
        trail_start_r=config.trail_start_r,
    )

    logger.info("Step 4b: Running T3 pipeline...")
    t3_result = run_single_pool_pipeline(
        events=t3_events,
        bars_cache=bars_cache,
        forward_start=config.forward_start,
        max_hold_hours=config.max_hold_hours,
        pool_name="T3",
        random_state=config.random_state,
        base_notional_share=config.base_notional_share,
        speed_gate_quantile=config.speed_gate_quantile,
        trail_start_r=config.trail_start_r,
    )

    # --- Step 5: 合并 forward trades ---
    # T2/T3 是互斥入场结构，不是 portfolio sleeve；各自保留概率模型给出的完整 sizing。
    logger.info("Step 5: Merging forward trades via full-size concat")

    t2_fwd = _tag_pool(t2_result.forward_trades, "T2")
    t3_fwd = _tag_pool(t3_result.forward_trades, "T3")

    union_forward_trades = pd.concat(
        [t2_fwd, t3_fwd], ignore_index=True
    ).sort_values("touch_time").reset_index(drop=True)

    # Union metrics
    union_cs = compute_calendar_sum(union_forward_trades, gate_filter=True)
    union_ws = compute_worst_sm(union_forward_trades, gate_filter=True)

    union_trade_count = _active_trade_count(union_forward_trades)
    union_neg_sm = _negative_month_count(union_forward_trades)

    pipeline_seconds = time.time() - pipeline_start

    logger.info("=" * 60)
    logger.info("Union strategy 完成 (%.1fs)", pipeline_seconds)
    logger.info("  T2: CS=%.4f, trades=%d", t2_result.calendar_sum, t2_result.trade_count)
    logger.info("  T3: CS=%.4f, trades=%d", t3_result.calendar_sum, t3_result.trade_count)
    logger.info("  Union: CS=%.4f, trades=%d", union_cs, union_trade_count)
    logger.info("=" * 60)

    return UnionStrategyResult(
        config=config,
        t2_result=t2_result,
        t3_result=t3_result,
        union_forward_trades=union_forward_trades,
        union_calendar_sum=union_cs,
        union_worst_sm=union_ws,
        union_trade_count=union_trade_count,
        union_neg_sm_count=union_neg_sm,
        t2_event_count=len(t2_events),
        t3_event_count=len(t3_events),
        t3_filter_params=t3_filter_params,
        pipeline_seconds=pipeline_seconds,
    )


# ---------------------------------------------------------------------------
# 滚动窗口 (Task 4)
# ---------------------------------------------------------------------------


def run_rolling_windows(config: UnionStrategyConfig) -> RollingWindowResult:
    """执行滚动窗口模式的 union strategy pipeline。

    遍历 config.rolling_months，每个月独立执行 run_union_strategy，
    然后汇总所有 window 的 forward 结果。

    Parameters
    ----------
    config : UnionStrategyConfig
        Union strategy 配置。rolling_months 若为 None 则使用默认
        ["2026-02-01", "2026-03-01", "2026-04-01"]。

    Returns
    -------
    RollingWindowResult
        包含各 window 独立结果和汇总指标。
    """
    rolling_months = config.rolling_months or [
        "2026-02-01", "2026-03-01", "2026-04-01"
    ]

    logger.info("=" * 60)
    logger.info("run_rolling_windows: %d windows", len(rolling_months))
    logger.info("  months: %s", rolling_months)
    logger.info("=" * 60)

    window_results: list[UnionStrategyResult] = []

    for i, forward_start in enumerate(rolling_months):
        logger.info("-" * 40)
        logger.info("Window %d/%d: forward_start=%s", i + 1, len(rolling_months), forward_start)
        logger.info("-" * 40)

        # 为每个 window 创建独立 config
        window_config = UnionStrategyConfig(
            base_notional_share=config.base_notional_share,
            speed_gate_quantile=config.speed_gate_quantile,
            random_state=config.random_state,
            t3_pre_touch_max=config.t3_pre_touch_max,
            forward_start=forward_start,
            rolling_months=None,  # 单 window 不需要 rolling
            trail_start_r=config.trail_start_r,
            max_hold_hours=config.max_hold_hours,
        )

        result = run_union_strategy(window_config)

        window_start = pd.Timestamp(forward_start, tz="UTC")
        if i + 1 < len(rolling_months):
            window_end = pd.Timestamp(rolling_months[i + 1], tz="UTC")
        else:
            window_end = _next_month_start(window_start)

        sliced_result = _slice_result_to_window(result, window_start, window_end)
        logger.info(
            "Window %d forward slice [%s, %s): CS=%.4f, trades=%d",
            i + 1,
            window_start.date(),
            window_end.date(),
            sliced_result.union_calendar_sum,
            sliced_result.union_trade_count,
        )
        window_results.append(sliced_result)

    # --- 拼接所有 window 的 forward trades ---
    all_forward_trades = []
    for wr in window_results:
        if not wr.union_forward_trades.empty:
            all_forward_trades.append(wr.union_forward_trades)

    if all_forward_trades:
        combined_forward_trades = pd.concat(
            all_forward_trades, ignore_index=True
        ).sort_values("touch_time").reset_index(drop=True)
    else:
        combined_forward_trades = pd.DataFrame()

    # --- 汇总指标 ---
    total_cs = compute_calendar_sum(combined_forward_trades, gate_filter=True)
    total_ws = compute_worst_sm(combined_forward_trades, gate_filter=True)
    total_trades = sum(wr.union_trade_count for wr in window_results)

    # --- Monthly attribution ---
    monthly_attribution = _compute_monthly_attribution(combined_forward_trades, window_results)

    logger.info("=" * 60)
    logger.info("Rolling windows 完成: %d windows", len(window_results))
    logger.info("  Total forward CS: %.4f", total_cs)
    logger.info("  Total worst SM: %.4f", total_ws)
    logger.info("  Total trades: %d", total_trades)
    logger.info("  Monthly coverage: %s", [m["month"] for m in monthly_attribution])
    logger.info("=" * 60)

    return RollingWindowResult(
        config=config,
        window_results=window_results,
        combined_forward_trades=combined_forward_trades,
        total_forward_cs=total_cs,
        total_worst_sm=total_ws,
        total_trade_count=total_trades,
        monthly_attribution=monthly_attribution,
    )


def _compute_monthly_attribution(
    combined_trades: pd.DataFrame,
    window_results: list[UnionStrategyResult],
) -> list[dict]:
    """计算逐月 attribution，标注 T2/T3 各自贡献。

    Returns
    -------
    list[dict]
        每个 dict 包含 month、t2_pnl、t3_pnl、union_pnl、t2_trades、t3_trades。
    """
    if combined_trades.empty:
        return []

    trades = combined_trades.copy()
    trades["touch_time"] = pd.to_datetime(trades["touch_time"], utc=True)

    # 只保留 gate on 的交易
    gate_on = trades[trades.get("speed_gate_pass", pd.Series(dtype=bool)) == True]  # noqa: E712
    if gate_on.empty:
        # Fallback: if speed_gate_pass col missing, use all
        gate_on = trades

    gate_on = gate_on.copy()
    gate_on["year_month"] = gate_on["touch_time"].dt.to_period("M")

    attribution = []
    for period, group in gate_on.groupby("year_month"):
        t2_mask = group["pool"] == "T2" if "pool" in group.columns else pd.Series(False, index=group.index)
        t3_mask = group["pool"] == "T3" if "pool" in group.columns else pd.Series(False, index=group.index)
        active_mask = (
            (group["timing_prediction"] != "skip")
            & (group["speed_gate_pass"] == True)  # noqa: E712
        )

        t2_pnl = float(group.loc[t2_mask, "weighted_pnl"].sum()) if t2_mask.any() else 0.0
        t3_pnl = float(group.loc[t3_mask, "weighted_pnl"].sum()) if t3_mask.any() else 0.0
        union_pnl = float(group["weighted_pnl"].sum())

        attribution.append({
            "month": str(period),
            "t2_pnl": round(t2_pnl, 6),
            "t3_pnl": round(t3_pnl, 6),
            "union_pnl": round(union_pnl, 6),
            "t2_trades": int((t2_mask & active_mask).sum()),
            "t3_trades": int((t3_mask & active_mask).sum()),
        })

    return sorted(attribution, key=lambda x: x["month"])


# ---------------------------------------------------------------------------
# 对比报告生成 (Task 5)
# ---------------------------------------------------------------------------


def generate_union_report_json(
    result: UnionStrategyResult | RollingWindowResult,
    output_dir: str | Path,
) -> Path:
    """生成 union_strategy_report.json。

    Parameters
    ----------
    result : UnionStrategyResult | RollingWindowResult
        Pipeline 执行结果。
    output_dir : str | Path
        输出目录。

    Returns
    -------
    Path
        输出文件路径。
    """
    output_dir = Path(output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)

    report: dict = {}

    if isinstance(result, RollingWindowResult):
        # Rolling window 模式
        base_config = result.config
        report["config"] = _config_to_dict(base_config)
        report["mode"] = "rolling"

        # 使用第一个 window 的数据作为 event_counts 汇总
        if result.window_results:
            first = result.window_results[0]
            report["event_counts"] = {
                "t2": first.t2_event_count,
                "t3": first.t3_event_count,
                "total": first.t2_event_count + first.t3_event_count,
            }

        # 各 window 的结果
        report["rolling_windows"] = []
        for wr in result.window_results:
            report["rolling_windows"].append({
                "forward_start": wr.config.forward_start,
                "t2_only": _pool_result_to_dict(wr.t2_result),
                "t3_only": _pool_result_to_dict(wr.t3_result),
                "union": {
                    "forward_cs": round(wr.union_calendar_sum, 6),
                    "worst_sm": round(wr.union_worst_sm, 6),
                    "trades": wr.union_trade_count,
                    "neg_sm": wr.union_neg_sm_count,
                },
            })

        # 汇总结果
        report["results"] = {
            "total_forward_cs": round(result.total_forward_cs, 6),
            "total_worst_sm": round(result.total_worst_sm, 6),
            "total_trades": result.total_trade_count,
        }

        report["monthly_attribution"] = result.monthly_attribution

    else:
        # 单 window 模式
        report["config"] = _config_to_dict(result.config)
        report["mode"] = "single"
        report["event_counts"] = {
            "t2": result.t2_event_count,
            "t3": result.t3_event_count,
            "total": result.t2_event_count + result.t3_event_count,
        }
        report["results"] = {
            "t2_only": _pool_result_to_dict(result.t2_result),
            "t3_only": _pool_result_to_dict(result.t3_result),
            "union": {
                "forward_cs": round(result.union_calendar_sum, 6),
                "worst_sm": round(result.union_worst_sm, 6),
                "trades": result.union_trade_count,
                "neg_sm": result.union_neg_sm_count,
            },
        }

        # Monthly attribution for single window
        monthly_attr = _compute_monthly_attribution(
            result.union_forward_trades, [result]
        )
        report["monthly_attribution"] = monthly_attr

    report["generated_at"] = datetime.utcnow().isoformat() + "Z"

    output_path = output_dir / "union_strategy_report.json"
    with open(output_path, "w", encoding="utf-8") as f:
        json.dump(report, f, indent=2, ensure_ascii=False, default=str)

    logger.info("JSON 报告已生成: %s", output_path)
    return output_path


def generate_union_report_md(
    result: UnionStrategyResult | RollingWindowResult,
    output_dir: str | Path,
) -> Path:
    """生成 union_strategy_report.md。

    Parameters
    ----------
    result : UnionStrategyResult | RollingWindowResult
        Pipeline 执行结果。
    output_dir : str | Path
        输出目录。

    Returns
    -------
    Path
        输出文件路径。
    """
    output_dir = Path(output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)

    lines: list[str] = []
    lines.append("# T2+T3 Union Strategy Report")
    lines.append("")
    lines.append(f"Generated: {datetime.utcnow().isoformat()}Z")
    lines.append("")

    if isinstance(result, RollingWindowResult):
        _write_rolling_report_md(result, lines)
    else:
        _write_single_report_md(result, lines)

    output_path = output_dir / "union_strategy_report.md"
    with open(output_path, "w", encoding="utf-8") as f:
        f.write("\n".join(lines))

    logger.info("MD 报告已生成: %s", output_path)
    return output_path


def _write_single_report_md(result: UnionStrategyResult, lines: list[str]) -> None:
    """写入单 window 报告内容。"""
    cfg = result.config

    # --- 参数快照 ---
    lines.append("## 参数快照")
    lines.append("")
    lines.append(f"| 参数 | 值 |")
    lines.append(f"|------|-----|")
    lines.append("| Sizing Mode | Full-size concat; each pool keeps RF probability multiplier |")
    lines.append(f"| Forward Start | {cfg.forward_start} |")
    lines.append(f"| Base Notional Share | {cfg.base_notional_share:.2f} |")
    lines.append(f"| Speed Gate Quantile | {cfg.speed_gate_quantile:.2f} |")
    lines.append(f"| T3 Pre Touch Max | {cfg.t3_pre_touch_max:.0f}s |")
    lines.append(f"| Trail Start R | {cfg.trail_start_r:.1f} |")
    lines.append(f"| Max Hold Hours | {cfg.max_hold_hours:.1f} |")
    lines.append("")

    # --- 事件统计 ---
    lines.append("## 事件统计")
    lines.append("")
    lines.append(f"- T2 events: {result.t2_event_count}")
    lines.append(f"- T3 events: {result.t3_event_count}")
    lines.append(f"- Total: {result.t2_event_count + result.t3_event_count}")
    lines.append("")

    # --- 对比表格 ---
    lines.append("## 策略对比")
    lines.append("")
    lines.append("| 指标 | T2-only | T3-only | Union (T2+T3) |")
    lines.append("|------|---------|---------|---------------|")
    lines.append(
        f"| Forward CS | {result.t2_result.calendar_sum:.4f} "
        f"| {result.t3_result.calendar_sum:.4f} "
        f"| {result.union_calendar_sum:.4f} |"
    )
    lines.append(
        f"| Worst SM | {result.t2_result.worst_sm:.4f} "
        f"| {result.t3_result.worst_sm:.4f} "
        f"| {result.union_worst_sm:.4f} |"
    )
    lines.append(
        f"| Trade Count | {result.t2_result.trade_count} "
        f"| {result.t3_result.trade_count} "
        f"| {result.union_trade_count} |"
    )
    lines.append(
        f"| Neg SM Count | {result.t2_result.neg_sm_count} "
        f"| {result.t3_result.neg_sm_count} "
        f"| {result.union_neg_sm_count} |"
    )
    lines.append("")

    # --- T3 增量分析 ---
    _write_t3_increment_analysis(result, lines)

    # --- 月度明细 ---
    monthly_attr = _compute_monthly_attribution(
        result.union_forward_trades, [result]
    )
    _write_monthly_detail(monthly_attr, lines)


def _write_rolling_report_md(result: RollingWindowResult, lines: list[str]) -> None:
    """写入 rolling window 报告内容。"""
    cfg = result.config

    # --- 参数快照 ---
    lines.append("## 参数快照")
    lines.append("")
    lines.append(f"| 参数 | 值 |")
    lines.append(f"|------|-----|")
    lines.append("| Sizing Mode | Full-size concat; each pool keeps RF probability multiplier |")
    lines.append(f"| Rolling Months | {cfg.rolling_months or ['2026-02-01', '2026-03-01', '2026-04-01']} |")
    lines.append(f"| Base Notional Share | {cfg.base_notional_share:.2f} |")
    lines.append(f"| Speed Gate Quantile | {cfg.speed_gate_quantile:.2f} |")
    lines.append(f"| T3 Pre Touch Max | {cfg.t3_pre_touch_max:.0f}s |")
    lines.append(f"| Trail Start R | {cfg.trail_start_r:.1f} |")
    lines.append(f"| Max Hold Hours | {cfg.max_hold_hours:.1f} |")
    lines.append("")

    # --- 汇总结果 ---
    lines.append("## 汇总结果")
    lines.append("")
    lines.append(f"- Total Forward CS: {result.total_forward_cs:.4f}")
    lines.append(f"- Total Worst SM: {result.total_worst_sm:.4f}")
    lines.append(f"- Total Trades: {result.total_trade_count}")
    lines.append("")

    # --- 各 Window 对比 ---
    lines.append("## Rolling Window 明细")
    lines.append("")
    lines.append("| Window | Forward Start | T2 CS | T3 CS | Union CS | Union Trades |")
    lines.append("|--------|---------------|-------|-------|----------|--------------|")
    for i, wr in enumerate(result.window_results):
        lines.append(
            f"| {i + 1} | {wr.config.forward_start} "
            f"| {wr.t2_result.calendar_sum:.4f} "
            f"| {wr.t3_result.calendar_sum:.4f} "
            f"| {wr.union_calendar_sum:.4f} "
            f"| {wr.union_trade_count} |"
        )
    lines.append("")

    # --- T3 增量分析 (per window) ---
    lines.append("## T3 增量分析")
    lines.append("")
    for i, wr in enumerate(result.window_results):
        lines.append(f"### Window {i + 1} ({wr.config.forward_start})")
        lines.append("")
        _write_t3_increment_analysis(wr, lines)

    # --- 月度明细 ---
    _write_monthly_detail(result.monthly_attribution, lines)


def _write_t3_increment_analysis(result: UnionStrategyResult, lines: list[str]) -> None:
    """写入 T3 增量分析。

    标注哪些月份 T3 有交易而 T2 无交易或 T2 为弱势月。
    """
    lines.append("### T3 增量分析")
    lines.append("")

    t3_increment = result.union_calendar_sum - result.t2_result.calendar_sum
    lines.append(f"- Union CS vs T2-only CS: {result.union_calendar_sum:.4f} vs {result.t2_result.calendar_sum:.4f}")
    lines.append(f"- T3 增量: {t3_increment:+.4f}")
    lines.append("")

    # 分析 T3 在哪些月份有独特贡献
    if not result.union_forward_trades.empty and "pool" in result.union_forward_trades.columns:
        trades = result.union_forward_trades.copy()
        trades["touch_time"] = pd.to_datetime(trades["touch_time"], utc=True)
        gate_on = trades[trades["speed_gate_pass"] == True]  # noqa: E712

        if not gate_on.empty:
            gate_on = gate_on.copy()
            gate_on["year_month"] = gate_on["touch_time"].dt.to_period("M")

            t3_months_analysis = []
            for period, group in gate_on.groupby("year_month"):
                t2_group = group[group["pool"] == "T2"]
                t3_group = group[group["pool"] == "T3"]

                t2_pnl = float(t2_group["weighted_pnl"].sum()) if not t2_group.empty else 0.0
                t3_pnl = float(t3_group["weighted_pnl"].sum()) if not t3_group.empty else 0.0

                note = ""
                if t3_group.empty:
                    continue  # T3 无交易的月份跳过
                if t2_group.empty:
                    note = "⚠️ T3 独占（T2 无交易）"
                elif t2_pnl < 0:
                    note = "⚠️ T2 弱势月（T2 PnL < 0）"
                elif t3_pnl > 0:
                    note = "✅ T3 正向增量"
                else:
                    note = "T3 贡献为负"

                t3_months_analysis.append({
                    "month": str(period),
                    "t2_pnl": t2_pnl,
                    "t3_pnl": t3_pnl,
                    "note": note,
                })

            if t3_months_analysis:
                lines.append("| 月份 | T2 PnL | T3 PnL | 备注 |")
                lines.append("|------|--------|--------|------|")
                for item in t3_months_analysis:
                    lines.append(
                        f"| {item['month']} | {item['t2_pnl']:.4f} "
                        f"| {item['t3_pnl']:.4f} | {item['note']} |"
                    )
                lines.append("")
            else:
                lines.append("_T3 无 forward 交易_")
                lines.append("")
    else:
        lines.append("_无 forward 交易数据_")
        lines.append("")


def _write_monthly_detail(monthly_attribution: list[dict], lines: list[str]) -> None:
    """写入月度明细表格。"""
    lines.append("## 月度明细")
    lines.append("")

    if not monthly_attribution:
        lines.append("_无月度数据_")
        lines.append("")
        return

    lines.append("| 月份 | T2 PnL | T3 PnL | Union PnL | T2 Trades | T3 Trades |")
    lines.append("|------|--------|--------|-----------|-----------|-----------|")
    for attr in monthly_attribution:
        lines.append(
            f"| {attr['month']} | {attr['t2_pnl']:.4f} "
            f"| {attr['t3_pnl']:.4f} | {attr['union_pnl']:.4f} "
            f"| {attr['t2_trades']} | {attr['t3_trades']} |"
        )
    lines.append("")


def _config_to_dict(config: UnionStrategyConfig) -> dict:
    """将 config 转为可 JSON 序列化的 dict。"""
    return {
        "sizing_mode": "full_size_concat_probability_multiplier",
        "base_notional_share": config.base_notional_share,
        "speed_gate_quantile": config.speed_gate_quantile,
        "random_state": config.random_state,
        "t3_pre_touch_max": config.t3_pre_touch_max,
        "forward_start": config.forward_start,
        "rolling_months": config.rolling_months,
        "trail_start_r": config.trail_start_r,
        "max_hold_hours": config.max_hold_hours,
    }


def _pool_result_to_dict(pool_result: PoolPipelineResult) -> dict:
    """将单池结果转为可 JSON 序列化的 dict。"""
    return {
        "forward_cs": round(pool_result.calendar_sum, 6),
        "worst_sm": round(pool_result.worst_sm, 6),
        "trades": pool_result.trade_count,
        "neg_sm": pool_result.neg_sm_count,
        "timing_depth": pool_result.timing_selected_depth,
        "timing_loocv_cs": round(pool_result.timing_loocv_cs, 6),
        "rf_test_auc": round(pool_result.rf_test_auc, 6),
        "skipped": pool_result.skipped,
        "skip_reason": pool_result.skip_reason,
    }


# ---------------------------------------------------------------------------
# CLI Entry
# ---------------------------------------------------------------------------


if __name__ == "__main__":
    import argparse

    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
    )

    parser = argparse.ArgumentParser(description="T2+T3 Union Strategy Runner")
    parser.add_argument("--forward-start", default="2026-02-01",
                        help="Forward split 起始日期 (default: 2026-02-01)")
    parser.add_argument("--t2-weight", type=float, default=None,
                        help="Deprecated; ignored. T2/T3 now use full-size concat.")
    parser.add_argument("--t3-weight", type=float, default=None,
                        help="Deprecated; ignored. T2/T3 now use full-size concat.")
    parser.add_argument("--base-notional-share", type=float, default=0.80,
                        help="Base notional share (default: 0.80)")
    parser.add_argument("--max-hold-hours", type=float, default=2.0,
                        help="最大持仓时间 (default: 2.0)")
    parser.add_argument("--t3-pre-touch-max", type=float, default=900.0,
                        help="T3 pre_touch_seconds 上限 (default: 900)")
    parser.add_argument("--rolling", action="store_true",
                        help="启用 rolling window 模式 (3 windows: Feb/Mar/Apr 2026)")
    parser.add_argument("--output-dir", default=None,
                        help="报告输出目录 (default: output/multi_timeframe/)")

    args = parser.parse_args()
    if args.t2_weight is not None or args.t3_weight is not None:
        logger.warning(
            "--t2-weight/--t3-weight are deprecated and ignored; "
            "T2/T3 use full-size concat with per-pool probability sizing."
        )

    # 确定输出目录
    if args.output_dir:
        out_dir = Path(args.output_dir)
    else:
        out_dir = _SCRIPTS_DIR / "output" / "multi_timeframe"

    cfg = UnionStrategyConfig(
        forward_start=args.forward_start,
        base_notional_share=args.base_notional_share,
        t3_pre_touch_max=args.t3_pre_touch_max,
        max_hold_hours=args.max_hold_hours,
        rolling_months=["2026-02-01", "2026-03-01", "2026-04-01"] if args.rolling else None,
    )

    if args.rolling:
        rolling_result = run_rolling_windows(cfg)
        generate_union_report_json(rolling_result, out_dir)
        generate_union_report_md(rolling_result, out_dir)

        print(f"\n{'=' * 60}")
        print("Rolling Window Results")
        print(f"{'=' * 60}")
        for i, wr in enumerate(rolling_result.window_results):
            print(f"  Window {i + 1} (forward={wr.config.forward_start}): "
                  f"CS={wr.union_calendar_sum:.4f}, trades={wr.union_trade_count}")
        print(f"  Total: CS={rolling_result.total_forward_cs:.4f}, "
              f"trades={rolling_result.total_trade_count}")
    else:
        result = run_union_strategy(cfg)
        generate_union_report_json(result, out_dir)
        generate_union_report_md(result, out_dir)

        print(f"\n{'=' * 60}")
        print("Union Strategy Results")
        print(f"{'=' * 60}")
        print(f"  T2: CS={result.t2_result.calendar_sum:.4f}, "
              f"trades={result.t2_result.trade_count}")
        print(f"  T3: CS={result.t3_result.calendar_sum:.4f}, "
              f"trades={result.t3_result.trade_count}")
        print(f"  Union: CS={result.union_calendar_sum:.4f}, "
              f"trades={result.union_trade_count}")
        print(f"  Pipeline time: {result.pipeline_seconds:.1f}s")

    print(f"\n  Reports: {out_dir}/union_strategy_report.{{json,md}}")
