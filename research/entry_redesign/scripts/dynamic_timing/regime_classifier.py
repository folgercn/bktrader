"""
regime_classifier — 规则型决策分类器

负责：
- 基于 Tick_Feature_Vector 按优先级匹配 regime 规则，产出 Entry_Decision
- 规则顺序：Strong Momentum → Weak Signal → Over-Extended → Fading Momentum
  → Moderate Momentum → Developing → Default
- 所有阈值参数化为 TimingParams dataclass，支持 grid search 优化
"""

from __future__ import annotations

from dataclasses import dataclass
from enum import Enum
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from .feature_engine import StepFeatures


class EntryDecision(str, Enum):
    """步进式动态入场决策枚举"""

    IMMEDIATE = "immediate"
    WAIT_PULLBACK = "wait_pullback"
    CONTINUE_OBSERVE = "continue_observe"
    SKIP = "skip"


@dataclass
class TimingParams:
    """所有可调阈值参数，支持 grid search 优化"""

    max_steps: int = 4
    strong_momentum_threshold: float = 0.15  # ATR
    strong_flow_threshold: float = 0.58
    moderate_momentum_threshold: float = 0.08  # ATR
    extension_threshold: float = 0.15  # ATR
    weak_flow_threshold: float = 0.48
    fading_threshold: float = 0.02  # ATR
    min_steps_for_skip: int = 3
    pullback_target_atr: float = 0.05
    decision_window_seconds: int = 60
    abandon_extension_atr: float = 0.30


@dataclass
class DecisionResult:
    """单次决策步的输出结果"""

    decision: EntryDecision
    regime: str  # 匹配的 regime 名称
    step_index: int
    features: "StepFeatures"


def classify(features: "StepFeatures", params: TimingParams) -> DecisionResult:
    """按优先级匹配 regime 规则，返回决策。

    规则优先级（首个匹配即返回）：
    1. Strong Momentum → immediate
    2. Weak Signal → skip (需 step_index >= min_steps_for_skip)
    3. Over-Extended → wait_pullback
    4. Fading Momentum → skip (需 step_index >= 3)
    5. Moderate Momentum → immediate
    6. Developing → continue_observe (需 step_index < max_steps)
    7. Default → immediate (保底)
    """

    # 1. Strong Momentum → immediate
    # speed_cumulative_atr >= threshold AND flow_imbalance_cumulative >= threshold
    if (
        features.speed_cumulative_atr >= params.strong_momentum_threshold
        and features.flow_imbalance_cumulative >= params.strong_flow_threshold
    ):
        return DecisionResult(
            decision=EntryDecision.IMMEDIATE,
            regime="Strong Momentum",
            step_index=features.step_index,
            features=features,
        )

    # 2. Weak Signal → skip
    # flow_imbalance_cumulative < weak_flow_threshold AND step_index >= min_steps_for_skip
    if (
        features.flow_imbalance_cumulative < params.weak_flow_threshold
        and features.step_index >= params.min_steps_for_skip
    ):
        return DecisionResult(
            decision=EntryDecision.SKIP,
            regime="Weak Signal",
            step_index=features.step_index,
            features=features,
        )

    # 3. Over-Extended → wait_pullback
    # extension_atr >= extension_threshold AND pullback_from_max_atr <= 0.02
    if (
        features.extension_atr >= params.extension_threshold
        and features.pullback_from_max_atr <= 0.02
    ):
        return DecisionResult(
            decision=EntryDecision.WAIT_PULLBACK,
            regime="Over-Extended",
            step_index=features.step_index,
            features=features,
        )

    # 4. Fading Momentum → skip
    # speed_last_step_atr < fading_threshold AND step_index >= 3 AND flow_imbalance_last_step < 0.50
    if (
        features.speed_last_step_atr < params.fading_threshold
        and features.step_index >= 3
        and features.flow_imbalance_last_step < 0.50
    ):
        return DecisionResult(
            decision=EntryDecision.SKIP,
            regime="Fading Momentum",
            step_index=features.step_index,
            features=features,
        )

    # 5. Moderate Momentum → immediate
    # speed_cumulative_atr >= moderate_momentum_threshold AND continuation_ratio >= 0.6
    if (
        features.speed_cumulative_atr >= params.moderate_momentum_threshold
        and features.continuation_ratio >= 0.6
    ):
        return DecisionResult(
            decision=EntryDecision.IMMEDIATE,
            regime="Moderate Momentum",
            step_index=features.step_index,
            features=features,
        )

    # 6. Developing → continue_observe (需 step_index < max_steps)
    if features.step_index < params.max_steps:
        return DecisionResult(
            decision=EntryDecision.CONTINUE_OBSERVE,
            regime="Developing",
            step_index=features.step_index,
            features=features,
        )

    # 7. Default → immediate (保底：step_index == max_steps)
    return DecisionResult(
        decision=EntryDecision.IMMEDIATE,
        regime="Default",
        step_index=features.step_index,
        features=features,
    )
