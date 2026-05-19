"""PretouchStateGate — pre-touch 状态分箱 gating。

band = none         : 直接通过。
band = fast_clean   : distance_bucket ∈ [0.10, 0.15] ATR
                      AND speed300_bucket >= 0.20 ATR
                      AND pullback_bucket ∈ [0.00, 0.02] ATR
band = fast_clean_strict :
                      fast_clean 额外叠加
                      distance_bucket ∈ [0.10, 0.12] ATR
                      AND speed300_bucket >= 0.30 ATR
ATR 锚点 = signal_bar_start_ts 前最后一根已闭合 1h bar 的 ATR(14)。

event 的 pretouch_distance_bucket_atr / pretouch_speed300_bucket_atr /
pretouch_pullback_bucket_atr 字段已经是 ATR 归一化后的值（即 bucket / ATR）。

Requirements: 2.5
"""

from __future__ import annotations

from typing import Protocol

from research.entry_redesign.spec.entry_candidate_spec import PretouchStateBandId


# ---------------------------------------------------------------------------
# Event Protocol (最小字段集合)
# ---------------------------------------------------------------------------


class PretouchEvent(Protocol):
    """PretouchStateGate 所需的最小事件协议。

    字段已经过 ATR 归一化：值 = 原始 bucket 值 / ATR(14)。
    """

    @property
    def pretouch_distance_bucket_atr(self) -> float: ...

    @property
    def pretouch_speed300_bucket_atr(self) -> float: ...

    @property
    def pretouch_pullback_bucket_atr(self) -> float: ...


# ---------------------------------------------------------------------------
# PretouchStateGate
# ---------------------------------------------------------------------------


class PretouchStateGate:
    """Pre-touch 状态分箱 gate。

    根据 band_id 决定是否允许事件通过：
      - none: 直接通过（不做任何过滤）。
      - fast_clean: distance_bucket ∈ [0.10, 0.15] ATR
                    AND speed300_bucket >= 0.20 ATR
                    AND pullback_bucket ∈ [0.00, 0.02] ATR
      - fast_clean_strict: fast_clean 条件
                    + distance_bucket ∈ [0.10, 0.12] ATR
                    + speed300_bucket >= 0.30 ATR

    ATR 锚点 = signal_bar_start_ts 前最后一根已闭合 1h bar 的 ATR(14)，
    由调用方通过 atr14 参数传入。

    event 的 pretouch_*_bucket_atr 字段已经是 ATR 归一化后的值。
    """

    def __init__(self, band_id: PretouchStateBandId) -> None:
        """初始化 PretouchStateGate。

        Args:
            band_id: pre-touch 状态分箱标识。
                "none" — 不做过滤，直接通过。
                "fast_clean" — 标准 fast_clean band。
                "fast_clean_strict" — 严格 fast_clean band。
        """
        self._band_id: PretouchStateBandId = band_id

    def allow(self, event: PretouchEvent, atr14: float) -> bool:
        """判定事件是否通过 pre-touch 状态分箱 gate。

        Args:
            event: 入口事件，包含 pretouch_distance_bucket_atr、
                pretouch_speed300_bucket_atr、pretouch_pullback_bucket_atr
                三个已 ATR 归一化的字段。
            atr14: signal_bar_start_ts 前最后一根已闭合 1h bar 的 ATR(14)。
                用于将归一化 bucket 值转换回绝对值后与阈值比较。
                注意：event 字段已经是归一化值（= 绝对值 / ATR），
                因此比较时直接用归一化值与归一化阈值比较即可。

        Returns:
            True 如果事件通过 gate，False 如果被过滤。
        """
        if self._band_id == "none":
            return True

        # event 字段已经是 ATR 归一化值（bucket_value / atr14）
        distance = event.pretouch_distance_bucket_atr
        speed300 = event.pretouch_speed300_bucket_atr
        pullback = event.pretouch_pullback_bucket_atr

        if self._band_id == "fast_clean":
            return self._check_fast_clean(distance, speed300, pullback)

        if self._band_id == "fast_clean_strict":
            return self._check_fast_clean_strict(distance, speed300, pullback)

        # 不应到达此处（PretouchStateBandId 类型约束）
        return False  # pragma: no cover

    @staticmethod
    def _check_fast_clean(
        distance: float, speed300: float, pullback: float
    ) -> bool:
        """fast_clean band 判定。

        条件：
          - distance_bucket ∈ [0.10, 0.15] ATR
          - speed300_bucket >= 0.20 ATR
          - pullback_bucket ∈ [0.00, 0.02] ATR
        """
        if not (0.10 <= distance <= 0.15):
            return False
        if speed300 < 0.20:
            return False
        if not (0.00 <= pullback <= 0.02):
            return False
        return True

    @staticmethod
    def _check_fast_clean_strict(
        distance: float, speed300: float, pullback: float
    ) -> bool:
        """fast_clean_strict band 判定。

        条件 = fast_clean 全部条件 + 额外收紧：
          - distance_bucket ∈ [0.10, 0.12] ATR (收紧上界)
          - speed300_bucket >= 0.30 ATR (收紧下界)
          - pullback_bucket ∈ [0.00, 0.02] ATR (与 fast_clean 相同)
        """
        # fast_clean_strict 是 fast_clean 的子集：
        # distance 上界从 0.15 收紧到 0.12，speed300 下界从 0.20 收紧到 0.30
        if not (0.10 <= distance <= 0.12):
            return False
        if speed300 < 0.30:
            return False
        if not (0.00 <= pullback <= 0.02):
            return False
        return True
