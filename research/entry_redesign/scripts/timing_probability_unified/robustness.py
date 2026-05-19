"""Robustness — 稳健性验证（Bootstrap、Forward、Ablation）"""

from __future__ import annotations

import sys
from dataclasses import dataclass
from pathlib import Path

import numpy as np
import pandas as pd

# Ensure pre_breakout_timing is importable
_SCRIPTS_DIR = Path(__file__).resolve().parents[1]
if str(_SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS_DIR))

from pre_breakout_timing.delay_simulator import DelayResult, simulate_all_delays  # noqa: E402
from pre_breakout_timing.feature_extractor import extract_features, impute_features  # noqa: E402

from timing_probability_unified.combined_executor import (  # noqa: E402
    CombinedPositionConfig,
    compute_calendar_sum,
    compute_combined_positions,
    compute_worst_sm,
)
from timing_probability_unified.probability_model import compute_sizing_multiplier  # noqa: E402
from timing_probability_unified.timing_classifier import (  # noqa: E402
    generate_3regime_labels,
    get_selected_delay_pnl,
)


@dataclass
class BootstrapResult:
    """Bootstrap CI 结果"""

    calendar_sum_mean: float
    calendar_sum_ci_5: float
    calendar_sum_ci_95: float
    ci_width: float
    btc_calendar_sum: float
    btc_ci_5: float
    btc_ci_95: float
    eth_calendar_sum: float
    eth_ci_5: float
    eth_ci_95: float


@dataclass
class ForwardResult:
    """Forward split 验证结果"""

    forward_calendar_sum: float
    forward_worst_sm: float
    forward_trade_count: int
    overfitting_flag: bool  # forward < 0.5 × full_window
    forward_risk_flag: bool  # forward worst_sm < -1.0%
    forward_underperformance: bool  # forward < 7%


@dataclass
class AblationRow:
    """Ablation study 单行"""

    config_name: str  # "timing_only" / "probability_only" / "no_speed_gate" / "full_unified"
    calendar_sum: float
    worst_sm: float
    trade_count: int
    avg_pnl_per_trade: float


@dataclass
class RobustnessResult:
    """完整稳健性验证结果"""

    bootstrap: BootstrapResult
    forward: ForwardResult
    ablation: list[AblationRow]


def _bootstrap_calendar_sum(trades: pd.DataFrame, rng: np.random.Generator) -> float:
    """对 trades 做一次 bootstrap 重采样并计算 calendar_sum（简单 sum of weighted_pnl）。

    Parameters
    ----------
    trades : pd.DataFrame
        需含 weighted_pnl 列。
    rng : np.random.Generator
        随机数生成器。

    Returns
    -------
    float
        重采样后的 calendar_sum。
    """
    n = len(trades)
    indices = rng.integers(0, n, size=n)
    resampled = trades.iloc[indices]
    return float(resampled["weighted_pnl"].sum())


def run_bootstrap(
    trades: pd.DataFrame,
    n_resamples: int = 1000,
    random_state: int = 42,
) -> BootstrapResult:
    """1000 次 bootstrap 重采样，产出 5th/95th CI。

    对 full-window trades 执行 bootstrap 重采样，计算 calendar_sum 的置信区间。
    同时分 symbol 独立评估 BTC-only 和 ETH-only 的 CI。

    Parameters
    ----------
    trades : pd.DataFrame
        unified_trades DataFrame，需含 symbol, weighted_pnl 列。
    n_resamples : int
        重采样次数，默认 1000。
    random_state : int
        随机种子，默认 42。

    Returns
    -------
    BootstrapResult
        包含 overall 和 per-symbol 的 calendar_sum 均值及 5th/95th CI。
    """
    if trades.empty:
        return BootstrapResult(
            calendar_sum_mean=0.0,
            calendar_sum_ci_5=0.0,
            calendar_sum_ci_95=0.0,
            ci_width=0.0,
            btc_calendar_sum=0.0,
            btc_ci_5=0.0,
            btc_ci_95=0.0,
            eth_calendar_sum=0.0,
            eth_ci_5=0.0,
            eth_ci_95=0.0,
        )

    rng = np.random.default_rng(random_state)

    # --- Overall bootstrap ---
    overall_dist = np.array(
        [_bootstrap_calendar_sum(trades, rng) for _ in range(n_resamples)]
    )
    calendar_sum_mean = float(np.mean(overall_dist))
    calendar_sum_ci_5 = float(np.percentile(overall_dist, 5))
    calendar_sum_ci_95 = float(np.percentile(overall_dist, 95))

    # --- BTC-only bootstrap ---
    btc_trades = trades[trades["symbol"].str.contains("BTC", case=False)]
    if btc_trades.empty:
        btc_calendar_sum = 0.0
        btc_ci_5 = 0.0
        btc_ci_95 = 0.0
    else:
        btc_rng = np.random.default_rng(random_state)
        btc_dist = np.array(
            [_bootstrap_calendar_sum(btc_trades, btc_rng) for _ in range(n_resamples)]
        )
        btc_calendar_sum = float(np.mean(btc_dist))
        btc_ci_5 = float(np.percentile(btc_dist, 5))
        btc_ci_95 = float(np.percentile(btc_dist, 95))

    # --- ETH-only bootstrap ---
    eth_trades = trades[trades["symbol"].str.contains("ETH", case=False)]
    if eth_trades.empty:
        eth_calendar_sum = 0.0
        eth_ci_5 = 0.0
        eth_ci_95 = 0.0
    else:
        eth_rng = np.random.default_rng(random_state)
        eth_dist = np.array(
            [_bootstrap_calendar_sum(eth_trades, eth_rng) for _ in range(n_resamples)]
        )
        eth_calendar_sum = float(np.mean(eth_dist))
        eth_ci_5 = float(np.percentile(eth_dist, 5))
        eth_ci_95 = float(np.percentile(eth_dist, 95))

    ci_width = calendar_sum_ci_95 - calendar_sum_ci_5

    return BootstrapResult(
        calendar_sum_mean=calendar_sum_mean,
        calendar_sum_ci_5=calendar_sum_ci_5,
        calendar_sum_ci_95=calendar_sum_ci_95,
        ci_width=ci_width,
        btc_calendar_sum=btc_calendar_sum,
        btc_ci_5=btc_ci_5,
        btc_ci_95=btc_ci_95,
        eth_calendar_sum=eth_calendar_sum,
        eth_ci_5=eth_ci_5,
        eth_ci_95=eth_ci_95,
    )


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
    bars_cache: dict,
) -> pd.DataFrame | None:
    """Retrieve 1s bar data for a single event from bars_cache.

    The bars_cache uses keys of the form "{symbol}_{YYYYMM}".

    Parameters
    ----------
    event : pd.Series
        Must contain 'symbol' and 'touch_time'.
    bars_cache : dict
        Mapping from "{symbol}_{YYYYMM}" to 1s bar DataFrame.

    Returns
    -------
    pd.DataFrame | None
        The 1s bar DataFrame, or None if not available.
    """
    symbol = str(event["symbol"])
    touch_time = pd.Timestamp(event["touch_time"])
    month_key = f"{symbol}_{touch_time.strftime('%Y%m')}"
    bars = bars_cache.get(month_key)
    if bars is not None and not bars.empty:
        return bars
    return None


def _simulate_forward_delays(
    forward_events: pd.DataFrame,
    bars_cache: dict,
) -> tuple[list[list[DelayResult]], list[int]]:
    """Simulate all delays for forward events using bars_cache.

    Parameters
    ----------
    forward_events : pd.DataFrame
        Forward split events.
    bars_cache : dict
        Mapping from "{symbol}_{YYYYMM}" to 1s bar DataFrame.

    Returns
    -------
    tuple[list[list[DelayResult]], list[int]]
        - delay_results: list of 5 DelayResult per event (only for valid events)
        - valid_indices: indices of events that had valid bar data
    """
    delay_results: list[list[DelayResult]] = []
    valid_indices: list[int] = []

    for idx in range(len(forward_events)):
        event = forward_events.iloc[idx]
        bars = _get_bars_for_event(event, bars_cache)

        if bars is None:
            # Skip events without bar data — create untradeable placeholder results
            event_id = str(event.get("event_id", f"fwd_{idx}"))
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
            valid_indices.append(idx)
            continue

        try:
            event_delays = simulate_all_delays(
                event=event,
                second_bars=bars,
                pullback_params=_DEFAULT_PULLBACK_PARAMS,
            )
            delay_results.append(event_delays)
            valid_indices.append(idx)
        except Exception:
            # If simulation fails, create untradeable placeholders
            event_id = str(event.get("event_id", f"fwd_{idx}"))
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
            valid_indices.append(idx)

    return delay_results, valid_indices


def run_forward_validation(
    forward_events: pd.DataFrame,
    timing_classifier,
    rf_model,
    speed_gate_threshold: float,
    bars_cache: dict,
    full_window_calendar_sum: float,
    config: CombinedPositionConfig | None = None,
) -> ForwardResult:
    """Forward split 验证：使用 train set 训练的模型在 forward split 上评估。

    流程：
    1. 对 forward_events 模拟所有 delays（使用 bars_cache）
    2. 提取特征并用 timing_classifier 预测 skip/fast/slow
    3. 用 rf_model 获取概率并计算 sizing multiplier
    4. 应用 speed_gate_threshold 标记 gate pass/fail
    5. 计算 combined positions → calendar_sum 和 worst_sm
    6. 设置 overfitting_flag、forward_risk_flag、forward_underperformance

    Parameters
    ----------
    forward_events : pd.DataFrame
        Forward split 事件 DataFrame，需含 event_id, symbol, side,
        touch_time, speed_300s_atr, 以及 Original_10_Features 列。
    timing_classifier : object
        已训练的 sklearn DecisionTreeClassifier（来自 train set）。
    rf_model : object
        已训练的 sklearn RandomForestClassifier（来自 train set）。
    speed_gate_threshold : float
        Speed gate 阈值（train set q10）。
    bars_cache : dict
        Mapping from "{symbol}_{YYYYMM}" to 1s bar DataFrame。
    full_window_calendar_sum : float
        Full-window 的 calendar_sum，用于计算 overfitting_flag。
    config : CombinedPositionConfig | None
        组合仓位配置。若为 None 则使用默认配置。

    Returns
    -------
    ForwardResult
        包含 forward_calendar_sum, forward_worst_sm, forward_trade_count,
        overfitting_flag, forward_risk_flag, forward_underperformance。
    """
    if config is None:
        config = CombinedPositionConfig()

    # Handle empty forward events
    if forward_events.empty:
        return ForwardResult(
            forward_calendar_sum=0.0,
            forward_worst_sm=0.0,
            forward_trade_count=0,
            overfitting_flag=True,  # 0 < 0.5 * any positive full_window
            forward_risk_flag=False,
            forward_underperformance=True,  # 0 < 7%
        )

    # --- Step 1: Simulate delays for forward events ---
    delay_results, valid_indices = _simulate_forward_delays(forward_events, bars_cache)

    # Use all forward events (valid_indices should cover all)
    fwd_events = forward_events.reset_index(drop=True)
    n = len(fwd_events)

    # --- Step 2: Extract features and predict timing ---
    # Extract features from forward_events
    features_df, used_features, _ = extract_features(fwd_events)

    # Impute missing values using median (self-imputation for forward set)
    # In production pipeline, train medians would be passed in. Here we use
    # the forward set's own medians as a reasonable approximation.
    imputed_features = features_df.fillna(features_df.median())

    # Predict timing regime using the pre-trained classifier
    if len(used_features) > 0 and len(imputed_features) > 0:
        timing_predictions = timing_classifier.predict(imputed_features)
    else:
        # Fallback: all skip if no features available
        timing_predictions = np.array(["skip"] * n, dtype=object)

    # --- Step 3: Get RF probabilities and sizing multipliers ---
    if len(used_features) > 0 and len(imputed_features) > 0:
        # Get probability of class 1 from RF model
        if hasattr(rf_model, "classes_") and len(rf_model.classes_) >= 2:
            class_1_idx = list(rf_model.classes_).index(1)
            rf_probabilities = rf_model.predict_proba(imputed_features)[:, class_1_idx]
        else:
            # Degenerate model — uniform probability
            rf_probabilities = np.full(n, 0.5)
    else:
        rf_probabilities = np.full(n, 0.5)

    sizing_multipliers = compute_sizing_multiplier(rf_probabilities)

    # --- Step 4: Apply speed gate ---
    speed_gate_pass = (fwd_events["speed_300s_atr"] >= speed_gate_threshold).values

    # --- Step 5: Compute combined positions ---
    trades = compute_combined_positions(
        events=fwd_events,
        timing_predictions=timing_predictions,
        sizing_multipliers=sizing_multipliers,
        delay_results=delay_results,
        speed_gate_pass=speed_gate_pass,
        config=config,
    )

    # --- Step 6: Compute metrics (gate ON) ---
    forward_calendar_sum = compute_calendar_sum(trades, gate_filter=True)
    forward_worst_sm = compute_worst_sm(trades, gate_filter=True)

    # Trade count: non-skip events that pass gate
    active_trades = trades[
        (trades["timing_prediction"] != "skip")
        & (trades["speed_gate_pass"] == True)  # noqa: E712
    ]
    forward_trade_count = len(active_trades)

    # --- Step 7: Set flags ---
    overfitting_flag = forward_calendar_sum < 0.5 * full_window_calendar_sum
    forward_risk_flag = forward_worst_sm < -0.01  # worst_sm < -1.0%
    forward_underperformance = forward_calendar_sum < 0.07  # forward < 7%

    return ForwardResult(
        forward_calendar_sum=forward_calendar_sum,
        forward_worst_sm=forward_worst_sm,
        forward_trade_count=forward_trade_count,
        overfitting_flag=overfitting_flag,
        forward_risk_flag=forward_risk_flag,
        forward_underperformance=forward_underperformance,
    )


def run_ablation_study(
    events: pd.DataFrame,
    timing_predictions: np.ndarray,
    sizing_multipliers: np.ndarray,
    delay_results: list,
    speed_gate_pass: np.ndarray,
    config: CombinedPositionConfig | None = None,
) -> list[AblationRow]:
    """Ablation study：逐一移除组件，量化各组件贡献。

    四种配置：
    - timing_only: 使用 timing predictions 但 multiplier=1.0（无 probability sizing）。Gate ON。
    - probability_only: 所有事件入场（无 timing skip），使用原始 multipliers。Gate ON。
    - no_speed_gate: 使用 timing + probability 但 gate OFF（所有事件通过）。
    - full_unified: 使用所有组件（timing + probability + gate ON）。

    对每种配置计算：
    - calendar_sum（使用 gate_filter 对应配置）
    - worst_sm
    - trade_count（非 skip 且通过 gate 的事件数）
    - avg_pnl_per_trade

    Parameters
    ----------
    events : pd.DataFrame
        事件池 DataFrame，需含 event_id, symbol, side, touch_time,
        speed_300s_atr 列。
    timing_predictions : np.ndarray
        每个事件的 timing 预测 ("skip", "fast", "slow")。
    sizing_multipliers : np.ndarray
        每个事件的 sizing 乘数 (0..2)。
    delay_results : list
        每个事件的 5 种 delay 模拟结果。
    speed_gate_pass : np.ndarray
        每个事件的 speed gate 通过标记 (bool)。
    config : CombinedPositionConfig | None
        组合仓位配置。若为 None 则使用默认配置。

    Returns
    -------
    list[AblationRow]
        4 行 ablation 结果，顺序为 timing_only, probability_only,
        no_speed_gate, full_unified。
    """
    if config is None:
        config = CombinedPositionConfig()

    n = len(events)
    rows: list[AblationRow] = []

    # --- Configuration 1: timing_only ---
    # Use timing predictions (skip/fast/slow) but set multiplier=1.0 for all.
    # Gate ON (use original speed_gate_pass).
    timing_only_multipliers = np.ones(n, dtype=np.float64)
    trades_timing_only = compute_combined_positions(
        events=events,
        timing_predictions=timing_predictions,
        sizing_multipliers=timing_only_multipliers,
        delay_results=delay_results,
        speed_gate_pass=speed_gate_pass,
        config=config,
    )
    rows.append(_compute_ablation_row("timing_only", trades_timing_only, gate_filter=True))

    # --- Configuration 2: probability_only ---
    # All events enter (no timing skip) — replace "skip" with "fast" so all events trade.
    # Use original multipliers. Gate ON.
    prob_only_predictions = np.array(
        ["fast" if p == "skip" else p for p in timing_predictions], dtype=object
    )
    trades_prob_only = compute_combined_positions(
        events=events,
        timing_predictions=prob_only_predictions,
        sizing_multipliers=sizing_multipliers,
        delay_results=delay_results,
        speed_gate_pass=speed_gate_pass,
        config=config,
    )
    rows.append(_compute_ablation_row("probability_only", trades_prob_only, gate_filter=True))

    # --- Configuration 3: no_speed_gate ---
    # Use timing + probability but gate OFF (all events pass).
    all_pass_gate = np.ones(n, dtype=bool)
    trades_no_gate = compute_combined_positions(
        events=events,
        timing_predictions=timing_predictions,
        sizing_multipliers=sizing_multipliers,
        delay_results=delay_results,
        speed_gate_pass=all_pass_gate,
        config=config,
    )
    rows.append(_compute_ablation_row("no_speed_gate", trades_no_gate, gate_filter=True))

    # --- Configuration 4: full_unified ---
    # Use all components as-is (timing + probability + gate ON).
    trades_full = compute_combined_positions(
        events=events,
        timing_predictions=timing_predictions,
        sizing_multipliers=sizing_multipliers,
        delay_results=delay_results,
        speed_gate_pass=speed_gate_pass,
        config=config,
    )
    rows.append(_compute_ablation_row("full_unified", trades_full, gate_filter=True))

    return rows


def _compute_ablation_row(
    config_name: str,
    trades: pd.DataFrame,
    gate_filter: bool,
) -> AblationRow:
    """从 trades DataFrame 计算单行 ablation 指标。

    Parameters
    ----------
    config_name : str
        配置名称。
    trades : pd.DataFrame
        unified_trades DataFrame。
    gate_filter : bool
        是否使用 speed_gate_pass 过滤。

    Returns
    -------
    AblationRow
        包含 calendar_sum, worst_sm, trade_count, avg_pnl_per_trade。
    """
    calendar_sum = compute_calendar_sum(trades, gate_filter=gate_filter)
    worst_sm = compute_worst_sm(trades, gate_filter=gate_filter)

    # Trade count: non-skip events that pass gate
    active_trades = trades[
        (trades["timing_prediction"] != "skip") & (trades["speed_gate_pass"] == True)  # noqa: E712
    ]
    trade_count = len(active_trades)

    if trade_count > 0:
        avg_pnl_per_trade = float(active_trades["weighted_pnl"].sum() / trade_count)
    else:
        avg_pnl_per_trade = 0.0

    return AblationRow(
        config_name=config_name,
        calendar_sum=calendar_sum,
        worst_sm=worst_sm,
        trade_count=trade_count,
        avg_pnl_per_trade=avg_pnl_per_trade,
    )
