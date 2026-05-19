"""quality_filter — T2/T3 事件 quality filter 封装

T3 filter 逻辑：
1. 从 train_events 计算 speed_300s_atr 绝对值的 q25 阈值
2. 应用 speed_300s_atr_abs >= threshold 过滤
3. 应用 pre_touch_seconds <= 900 过滤

T2 filter：
T2 已由 canonical CSV 预过滤（rf_q50 + speed_ge_q10 + touch30m + eff300le1），
本模块仅提供 get_t2_filter_info() 返回参数描述供文档使用。
"""

from __future__ import annotations

import logging
from dataclasses import dataclass

import numpy as np
import pandas as pd

logger = logging.getLogger(__name__)


@dataclass
class QualityFilterConfig:
    """T3 quality filter 配置

    Attributes
    ----------
    t3_speed_quantile : float
        训练集 speed_300s_atr 绝对值的分位数阈值，默认 0.25（q25）。
    t3_pre_touch_max : float
        T3 事件 pre_touch_seconds 上限，默认 900.0（15 分钟）。
    t3_speed_threshold : float | None
        运行时从 train set 计算的 speed 阈值。初始为 None，
        在 apply_t3_quality_filter 中自动填充。
    """

    t3_speed_quantile: float = 0.25
    t3_pre_touch_max: float = 900.0
    t3_speed_threshold: float | None = None


def apply_t3_quality_filter(
    events: pd.DataFrame,
    train_events: pd.DataFrame,
    config: QualityFilterConfig | None = None,
) -> tuple[pd.DataFrame, dict]:
    """应用 T3 quality filter，返回过滤后事件 and 参数快照。

    过滤步骤：
    1. 从 train_events 计算 speed_300s_atr 绝对值的指定分位数阈值
    2. 对 events 应用 abs(speed_300s_atr) >= threshold
    3. 对 events 应用 pre_touch_seconds <= t3_pre_touch_max

    Parameters
    ----------
    events : pd.DataFrame
        待过滤的 T3 事件池，需含 speed_300s_atr 和 pre_touch_seconds 列。
    train_events : pd.DataFrame
        训练集事件，用于计算 speed threshold。需含 speed_300s_atr 列。
    config : QualityFilterConfig | None
        过滤配置。None 时使用默认配置。

    Returns
    -------
    tuple[pd.DataFrame, dict]
        (filtered_events, params_snapshot)
        - filtered_events: 过滤后的事件 DataFrame
        - params_snapshot: 参数快照字典，包含阈值和过滤统计
    """
    if config is None:
        config = QualityFilterConfig()

    n_before = len(events)

    # Step 1: 从 train_events 计算 speed threshold
    if train_events.empty:
        logger.warning(
            "train_events 为空，无法计算 speed threshold，使用 0.0 作为 fallback"
        )
        speed_threshold = 0.0
    else:
        train_speed_abs = train_events["speed_300s_atr"].abs()
        speed_threshold = float(
            np.nanquantile(train_speed_abs, config.t3_speed_quantile)
        )

    # 更新 config 中的运行时阈值
    config.t3_speed_threshold = speed_threshold

    logger.info(
        "T3 quality filter: speed threshold = %.6f "
        "(q%.0f of train_events speed_300s_atr abs, n_train=%d)",
        speed_threshold,
        config.t3_speed_quantile * 100,
        len(train_events),
    )

    if events.empty:
        params_snapshot = {
            "t3_speed_quantile": config.t3_speed_quantile,
            "t3_speed_threshold": speed_threshold,
            "t3_pre_touch_max": config.t3_pre_touch_max,
            "n_train_events": len(train_events),
            "n_before": 0,
            "n_after_speed": 0,
            "n_after_pre_touch": 0,
            "n_final": 0,
        }
        return events.copy(), params_snapshot

    # Step 2: 应用 speed filter
    speed_abs = events["speed_300s_atr"].abs()
    speed_mask = speed_abs >= speed_threshold
    after_speed = events[speed_mask].copy()
    n_after_speed = len(after_speed)

    logger.info(
        "  speed filter: %d -> %d (dropped %d, threshold=%.6f)",
        n_before,
        n_after_speed,
        n_before - n_after_speed,
        speed_threshold,
    )

    # Step 3: 应用 pre_touch_seconds filter
    pre_touch_mask = after_speed["pre_touch_seconds"] <= config.t3_pre_touch_max
    filtered = after_speed[pre_touch_mask].reset_index(drop=True)
    n_final = len(filtered)

    logger.info(
        "  pre_touch filter: %d -> %d (dropped %d, max=%.0fs)",
        n_after_speed,
        n_final,
        n_after_speed - n_final,
        config.t3_pre_touch_max,
    )

    logger.info(
        "T3 quality filter 完成: %d -> %d events (%.1f%% retained)",
        n_before,
        n_final,
        (n_final / n_before * 100) if n_before > 0 else 0.0,
    )

    # 构建参数快照
    params_snapshot = {
        "t3_speed_quantile": config.t3_speed_quantile,
        "t3_speed_threshold": speed_threshold,
        "t3_pre_touch_max": config.t3_pre_touch_max,
        "n_train_events": len(train_events),
        "n_before": n_before,
        "n_after_speed": n_after_speed,
        "n_after_pre_touch": n_final,
        "n_final": n_final,
    }

    return filtered, params_snapshot


def get_t2_filter_info() -> dict:
    """返回 T2 canonical filter 参数描述（仅文档用）。

    T2 事件已由 canonical CSV 预过滤，不需要在本框架中再次过滤。
    此函数仅提供参数描述，方便报告输出 and 参数对比。

    T2 canonical filter chain:
    1. pretouch_small_pullback seed
    2. RF q50 (probability ranking quantile)
    3. speed_300s_atr >= train q10 (speed gate)
    4. pre_touch_seconds <= 1800 (30min touch window)
    5. eff_300s <= 1.0 (排除过度推进)

    Returns
    -------
    dict
        T2 filter 参数描述字典。
    """
    return {
        "source": "canonical CSV (prefiltered)",
        "description": (
            "T2 events are prefiltered by the canonical seed pipeline. "
            "No additional filtering is applied in this framework."
        ),
        "filter_chain": [
            "pretouch_small_pullback seed",
            "RF q50 (probability ranking quantile)",
            "speed_300s_atr >= train q10 (speed gate)",
            "pre_touch_seconds <= 1800 (30min touch window)",
            "eff_300s <= 1.0 (排除过度推进)",
        ],
        "parameters": {
            "rf_quantile": 0.50,
            "speed_gate_quantile": 0.10,
            "pre_touch_max_seconds": 1800.0,
            "eff_300s_max": 1.0,
        },
        "total_events": 394,
        "note": "T2 filtering is handled upstream; this info is for documentation only.",
    }
