"""GateCompositionLayer — nogate vs gate001 并行评估。

gate=nogate     : 原始 event 不叠加 candidate_001 gate，始终通过。
gate=candidate_001 (gate001):
    validation_return_over_dd          <= 10
    AND validation_topk_sizing_markov_score_mean <= 0.9
    AND validation_topk_sized_return_pct         >= 0.5

快照 sha256 写入 summary JSON 的 gate001_snapshot_ref。

每笔 trade 同时产出两份 ledger 行（gate_mode 列区分）。

Requirements: 3.3, 5.1, 5.2
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Literal

from research.entry_redesign.cost.cost_model_applier import PricedTrade
from research.entry_redesign.gate.candidate_001_snapshot_loader import (
    Gate001Thresholds,
)


# ---------------------------------------------------------------------------
# GateDecision — gate 评估结果
# ---------------------------------------------------------------------------


@dataclass(frozen=True)
class GateDecision:
    """Gate 评估结果。

    Attributes:
        passed: 是否通过 gate 检查。
            - nogate 模式下恒为 True。
            - gate001 模式下需三阈值全部满足才为 True。
        gate_mode: 评估所用的 gate 模式（"nogate" 或 "gate001"）。
    """

    passed: bool
    gate_mode: str


# ---------------------------------------------------------------------------
# ValidationMetrics — validation 窗口产出的指标（供 gate001 判定）
# ---------------------------------------------------------------------------


@dataclass(frozen=True)
class ValidationMetrics:
    """Validation 窗口产出的指标，用于 gate001 三阈值判定。

    来源：walk-forward validation 窗口的回测结果。

    Attributes:
        validation_return_over_dd: validation 窗口 return / max drawdown 比值。
        validation_topk_sizing_markov_score_mean: validation 窗口 top-k sizing
            markov score 均值。
        validation_topk_sized_return_pct: validation 窗口 top-k sized return
            百分比。
    """

    validation_return_over_dd: float
    validation_topk_sizing_markov_score_mean: float
    validation_topk_sized_return_pct: float


# ---------------------------------------------------------------------------
# GateCompositionLayer
# ---------------------------------------------------------------------------


class GateCompositionLayer:
    """Gate 组合层：nogate 与 gate001 并行评估。

    职责：
      - 对每笔 PricedTrade，根据 gate_mode 返回 GateDecision。
      - nogate: 始终通过（不叠加任何 gate）。
      - gate001: 使用 candidate_001 三阈值对 validation 指标做判定。

    三层结构中 Gate Layer 为只读层（本 spec 不修改 gate 本身），
    仅复用 research/20260511_probabilistic_v6_calendar_holdout_validation.md
    产物中的 candidate_001 gate 参数快照。

    用法：
        thresholds = Gate001Thresholds()
        metrics = ValidationMetrics(
            validation_return_over_dd=8.5,
            validation_topk_sizing_markov_score_mean=0.85,
            validation_topk_sized_return_pct=0.6,
        )
        layer = GateCompositionLayer(thresholds, metrics)

        # 并行评估两种 gate_mode
        decision_nogate = layer.evaluate(priced_trade, "nogate")
        decision_gate001 = layer.evaluate(priced_trade, "gate001")
    """

    def __init__(
        self,
        gate_thresholds: Gate001Thresholds,
        validation_metrics: ValidationMetrics,
    ) -> None:
        """初始化 GateCompositionLayer。

        Args:
            gate_thresholds: candidate_001 gate 三阈值参数快照（frozen）。
            validation_metrics: 当前 execute 月对应的 validation 窗口指标。
                用于 gate001 模式下的三阈值判定。
        """
        self._thresholds = gate_thresholds
        self._validation_metrics = validation_metrics

    @property
    def gate_thresholds(self) -> Gate001Thresholds:
        """返回 gate 三阈值参数快照。"""
        return self._thresholds

    @property
    def validation_metrics(self) -> ValidationMetrics:
        """返回 validation 窗口指标。"""
        return self._validation_metrics

    def evaluate(
        self,
        trade: PricedTrade,
        gate_mode: Literal["nogate", "gate001"],
    ) -> GateDecision:
        """评估单笔 trade 在指定 gate_mode 下是否通过。

        Args:
            trade: 已计算成本的交易记录（PricedTrade）。
            gate_mode: gate 模式，"nogate" 或 "gate001"。

        Returns:
            GateDecision: 包含 passed (bool) 和 gate_mode (str)。

        Raises:
            ValueError: gate_mode 不在 {"nogate", "gate001"} 中。
        """
        if gate_mode == "nogate":
            return GateDecision(passed=True, gate_mode="nogate")

        if gate_mode == "gate001":
            passed = self._evaluate_gate001()
            return GateDecision(passed=passed, gate_mode="gate001")

        raise ValueError(
            f"Invalid gate_mode: {gate_mode!r}. "
            f"Must be 'nogate' or 'gate001'."
        )

    def _evaluate_gate001(self) -> bool:
        """应用 candidate_001 三阈值检查。

        三阈值条件（全部满足才通过）：
          1. validation_return_over_dd <= 10
          2. validation_topk_sizing_markov_score_mean <= 0.9
          3. validation_topk_sized_return_pct >= 0.5

        Returns:
            True 如果三个条件全部满足，False 否则。
        """
        metrics = self._validation_metrics
        thresholds = self._thresholds

        cond1 = (
            metrics.validation_return_over_dd
            <= thresholds.validation_return_over_dd_threshold
        )
        cond2 = (
            metrics.validation_topk_sizing_markov_score_mean
            <= thresholds.validation_topk_sizing_markov_score_mean_threshold
        )
        cond3 = (
            metrics.validation_topk_sized_return_pct
            >= thresholds.validation_topk_sized_return_pct_threshold
        )

        return cond1 and cond2 and cond3
