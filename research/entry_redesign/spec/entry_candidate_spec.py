"""Entry_Candidate 六元组定义与 Literal 枚举。

本模块定义 EntryCandidateSpec frozen dataclass 及其六个维度的类型约束，
以及 validate() 校验方法与 InvalidCandidateError 异常。
字段顺序固定：entry_delay_seconds → feature_horizon_seconds →
trigger_confirmation_id → entry_price_mode_id →
pretouch_state_band_id → posttouch_quality_band_id。

Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 2.7, 4.10
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Literal


# ---------------------------------------------------------------------------
# Literal 枚举类型
# ---------------------------------------------------------------------------

TriggerConfirmationId = Literal[
    "none",
    "persistence_n1",
    "persistence_n3",
    "persistence_n5",
    "persistence_n10",
    "retest_tb0",
    "retest_tb1",
    "retest_tb2",
    "minvol_bps50",
    "minvol_bps100",
    "minvol_bps200",
]

EntryPriceModeId = Literal[
    "market_on_touch",
    "limit_at_level",
    "limit_tb_k0",
    "limit_tb_k1",
    "limit_tb_k2",
    "limit_tb_k4",
    "pullback_p000",
    "pullback_p002",
    "pullback_p005",
    "pullback_p010",
]

PretouchStateBandId = Literal["none", "fast_clean", "fast_clean_strict"]

PosttouchQualityBandId = Literal[
    "none",
    "cont1s_r003",
    "cont1s_r005",
    "cont1s_r008",
    "tickimb_b055",
    "tickimb_b060",
    "tickimb_b065",
    "spread_s1",
    "spread_s2",
    "spread_s4",
]


# ---------------------------------------------------------------------------
# 合法值域常量
# ---------------------------------------------------------------------------

VALID_D: set[int] = {0, 5, 15, 30, 60, 120}
"""Entry_Delay_Seconds 合法值域。D=0 仅作为对照 baseline。"""

VALID_H: set[int] = {0, 5, 15, 30, 60}
"""Feature_Horizon_Seconds 合法值域。约束 H <= D (Point_In_Time_Feature)。"""


# ---------------------------------------------------------------------------
# InvalidCandidateError 异常
# ---------------------------------------------------------------------------


class InvalidCandidateError(Exception):
    """runner 启动阶段校验失败时抛出的异常。

    由上游在 runner 启动阶段（events 未读取前）捕获并写入
    runner_rejected_combinations.json。

    Attributes:
        reject_reason: 拒绝原因标识，封闭枚举值。
    """

    def __init__(self, reject_reason: str) -> None:
        self.reject_reason = reject_reason
        super().__init__(f"Invalid candidate: {reject_reason}")


# ---------------------------------------------------------------------------
# EntryCandidateSpec frozen dataclass
# ---------------------------------------------------------------------------


@dataclass(frozen=True)
class EntryCandidateSpec:
    """Entry_Candidate 六元组定义。

    字段顺序固定，不可变（frozen=True）。
    D ∈ {0, 5, 15, 30, 60, 120}；H ∈ {0, 5, 15, 30, 60}；H <= D。
    """

    entry_delay_seconds: int
    """D ∈ {0, 5, 15, 30, 60, 120}"""

    feature_horizon_seconds: int
    """H ∈ {0, 5, 15, 30, 60}, H <= D"""

    trigger_confirmation_id: TriggerConfirmationId
    entry_price_mode_id: EntryPriceModeId
    pretouch_state_band_id: PretouchStateBandId
    posttouch_quality_band_id: PosttouchQualityBandId

    def validate(self) -> None:
        """校验六元组合法性。

        在 runner 启动阶段（events 未读取前）调用。违反约束时抛出
        InvalidCandidateError，由上游捕获并写 runner_rejected_combinations.json。

        校验规则：
          1. D 必须在 VALID_D 中
          2. H 必须在 VALID_H 中
          3. H <= D（Point_In_Time_Feature 约束）

        Raises:
            InvalidCandidateError: reject_reason="invalid_D" 当 D 不在合法值域
            InvalidCandidateError: reject_reason="invalid_H" 当 H 不在合法值域
            InvalidCandidateError: reject_reason="H_gt_D" 当 H > D
        """
        if self.entry_delay_seconds not in VALID_D:
            raise InvalidCandidateError(reject_reason="invalid_D")
        if self.feature_horizon_seconds not in VALID_H:
            raise InvalidCandidateError(reject_reason="invalid_H")
        if self.feature_horizon_seconds > self.entry_delay_seconds:
            raise InvalidCandidateError(reject_reason="H_gt_D")
