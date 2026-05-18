"""PosttouchQualityGate — post-touch 微结构质量 gating。

在 [trigger_ts, trigger_ts + H] 窗口内对触发后的微结构特征做质量过滤。

band 枚举：
  - none           : 直接通过。
  - continuation_1s: 在 [trigger_ts, trigger_ts + H] 内 1s 累计 return
                     绝对值 >= r × ATR，r ∈ {0.03, 0.05, 0.08}。
                     long 方向要求累计 return >= r × ATR（正向延续）；
                     short 方向要求累计 return <= -(r × ATR)（负向延续）。
  - tick_imbalance : 在 [trigger_ts, trigger_ts + H] 内 tick_imbalance
                     (AC3 公式) >= b，b ∈ {0.55, 0.60, 0.65}。
                     tick_imbalance = sum(taker_side_quote_volume)
                                    / sum(all_taker_quote_volume)
                     long 用 taker_side = buy；short 用 taker_side = sell。
  - spread_ticks   : 在 trigger_ts 对应 1s bar 结束时刻，
                     spread_ticks = (best_ask - best_bid) / tick_size,
                     spread_ticks <= s, s ∈ {1, 2, 4}。

Requirements: 2.6, 2.10
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime
from typing import Literal, Protocol, Sequence

from research.entry_redesign.spec.entry_candidate_spec import PosttouchQualityBandId


# ---------------------------------------------------------------------------
# Data Protocols
# ---------------------------------------------------------------------------


@dataclass(frozen=True)
class OneSecondBarExt:
    """扩展 1 秒 K 线数据（含 close 价格与 spread 信息）。

    Attributes:
        open_ts: 该 1s bar 的开始时间戳（UTC）。
        close_ts: 该 1s bar 的结束时间戳（UTC，秒级精度）。
        open: 该 1s bar 的开盘价。
        high: 该 1s bar 的最高价。
        low: 该 1s bar 的最低价。
        close: 该 1s bar 的收盘价。
        best_ask: 该 1s bar 结束时刻的最优卖价。
        best_bid: 该 1s bar 结束时刻的最优买价。
    """

    open_ts: datetime
    close_ts: datetime
    open: float
    high: float
    low: float
    close: float
    best_ask: float
    best_bid: float


class OneSecondBarsProtocol(Protocol):
    """1 秒 K 线窗口协议。"""

    @property
    def bars(self) -> Sequence[OneSecondBarExt]: ...


@dataclass(frozen=True)
class TakerTrade:
    """单笔 taker 成交记录。

    Attributes:
        ts: 成交时间戳（UTC）。
        side: taker 方向，"buy" 或 "sell"。
        quote_volume: 该笔成交的 quote 成交额。
    """

    ts: datetime
    side: Literal["buy", "sell"]
    quote_volume: float


class TakerTradesWindow(Protocol):
    """Taker 成交窗口协议。

    提供指定时间窗口内的 taker 成交记录序列。
    """

    @property
    def trades(self) -> Sequence[TakerTrade]: ...


@dataclass(frozen=True)
class TriggerDecisionLike:
    """TriggerDecision 的最小协议（避免循环导入）。

    Attributes:
        trigger_ts: 触发时间戳（UTC，秒级精度）。
        side: 触发方向，"long" 或 "short"。
        level: 触发参考价位。
    """

    trigger_ts: datetime
    side: Literal["long", "short"]
    level: float


# ---------------------------------------------------------------------------
# Band 参数解析
# ---------------------------------------------------------------------------

# continuation_1s 阈值映射
_CONT1S_THRESHOLDS: dict[str, float] = {
    "cont1s_r003": 0.03,
    "cont1s_r005": 0.05,
    "cont1s_r008": 0.08,
}

# tick_imbalance 阈值映射
_TICKIMB_THRESHOLDS: dict[str, float] = {
    "tickimb_b055": 0.55,
    "tickimb_b060": 0.60,
    "tickimb_b065": 0.65,
}

# spread_ticks 阈值映射
_SPREAD_THRESHOLDS: dict[str, int] = {
    "spread_s1": 1,
    "spread_s2": 2,
    "spread_s4": 4,
}


# ---------------------------------------------------------------------------
# PosttouchQualityGate
# ---------------------------------------------------------------------------


class PosttouchQualityGate:
    """Post-touch 微结构质量 gate。

    根据 band_id 确定使用哪种质量检查及其阈值参数。

    band = none         : 直接通过。
    band = continuation_1s : 在 [trigger_ts, trigger_ts + H] 内 1s 累计 return
                             (long 为正、short 为负) 绝对值 >= r × ATR，
                             r ∈ {0.03, 0.05, 0.08}。
    band = tick_imbalance  : 在 [trigger_ts, trigger_ts + H] 内 tick_imbalance
                             (定义同 AC3) >= b，b ∈ {0.55, 0.60, 0.65}。
    band = spread_ticks    : 在 trigger_ts 对应 1s bar 结束时刻，
                             spread_ticks = (best_ask - best_bid) / tick_size,
                             spread_ticks <= s, s ∈ {1, 2, 4}。
    """

    def __init__(
        self,
        band_id: PosttouchQualityBandId,
        tick_size: float,
    ) -> None:
        """初始化 PosttouchQualityGate。

        Args:
            band_id: Posttouch quality band 标识符。
                "none" 表示不做质量过滤（直接通过）。
                "cont1s_r003" / "cont1s_r005" / "cont1s_r008" 表示
                continuation_1s 检查，阈值分别为 0.03 / 0.05 / 0.08。
                "tickimb_b055" / "tickimb_b060" / "tickimb_b065" 表示
                tick_imbalance 检查，阈值分别为 0.55 / 0.60 / 0.65。
                "spread_s1" / "spread_s2" / "spread_s4" 表示
                spread_ticks 检查，阈值分别为 1 / 2 / 4。
            tick_size: 该 symbol 的最小价格变动单位。
                来自 RunnerParameterSnapshot.symbol_filters.tick_size_by_symbol。
        """
        self._band_id = band_id
        self._tick_size = tick_size

        # 解析 band 类型与阈值
        if band_id == "none":
            self._check_type = "none"
            self._threshold: float = 0.0
        elif band_id in _CONT1S_THRESHOLDS:
            self._check_type = "continuation_1s"
            self._threshold = _CONT1S_THRESHOLDS[band_id]
        elif band_id in _TICKIMB_THRESHOLDS:
            self._check_type = "tick_imbalance"
            self._threshold = _TICKIMB_THRESHOLDS[band_id]
        elif band_id in _SPREAD_THRESHOLDS:
            self._check_type = "spread_ticks"
            self._threshold = float(_SPREAD_THRESHOLDS[band_id])
        else:
            raise ValueError(f"Unknown posttouch_quality_band_id: {band_id}")

    def allow(
        self,
        trigger: TriggerDecisionLike,
        onesec_window: OneSecondBarsProtocol,
        trades_window: TakerTradesWindow,
        atr14: float,
    ) -> bool:
        """判定 post-touch 质量是否满足 gate 条件。

        Args:
            trigger: 触发判定结果，包含 trigger_ts、side、level。
            onesec_window: [trigger_ts, trigger_ts + H] 窗口内的 1s bar 序列。
                调用方负责按 H 截取正确的时间窗口。
            trades_window: [trigger_ts, trigger_ts + H] 窗口内的 taker 成交记录。
                调用方负责按 H 截取正确的时间窗口。
            atr14: signal_bar_start_ts 前最后一根已闭合 1h bar 的 ATR(14)。

        Returns:
            True 如果质量满足 gate 条件（允许入场），False 否则。
            band_id == "none" 时恒返回 True。
        """
        if self._check_type == "none":
            return True
        elif self._check_type == "continuation_1s":
            return self._check_continuation_1s(trigger, onesec_window, atr14)
        elif self._check_type == "tick_imbalance":
            return self._check_tick_imbalance(trigger, trades_window)
        elif self._check_type == "spread_ticks":
            return self._check_spread_ticks(trigger, onesec_window)
        else:
            # 不应到达此处（构造时已校验）
            return False

    # ------------------------------------------------------------------
    # Private check methods
    # ------------------------------------------------------------------

    def _check_continuation_1s(
        self,
        trigger: TriggerDecisionLike,
        onesec_window: OneSecondBarsProtocol,
        atr14: float,
    ) -> bool:
        """continuation_1s 检查。

        在 [trigger_ts, trigger_ts + H] 内计算 1s 累计 return。
        累计 return 定义为窗口内各 1s bar 的 (close - open) 之和，
        再除以 trigger 时刻的参考价格（level）得到相对 return。

        long 方向：累计 return >= r × ATR（正向延续）。
        short 方向：累计 return <= -(r × ATR)（负向延续）。
        即绝对值 >= r × ATR。
        """
        bars = onesec_window.bars
        if not bars:
            return False

        # 计算窗口内 1s bar 的累计绝对 return（价格变动之和）
        cumulative_return = 0.0
        for bar in bars:
            cumulative_return += bar.close - bar.open

        threshold = self._threshold * atr14

        if trigger.side == "long":
            # long 方向要求正向延续
            return cumulative_return >= threshold
        else:
            # short 方向要求负向延续
            return cumulative_return <= -threshold

    def _check_tick_imbalance(
        self,
        trigger: TriggerDecisionLike,
        trades_window: TakerTradesWindow,
    ) -> bool:
        """tick_imbalance 检查（AC3 公式）。

        tick_imbalance = sum(taker_side_quote_volume) / sum(all_taker_quote_volume)
        long 用 taker_side = buy；short 用 taker_side = sell。
        窗口为 [trigger_ts, trigger_ts + H]（由调用方截取）。

        阈值：tick_imbalance >= b。
        """
        trades = trades_window.trades
        if not trades:
            return False

        taker_side: Literal["buy", "sell"] = (
            "buy" if trigger.side == "long" else "sell"
        )

        total_quote_volume = 0.0
        side_quote_volume = 0.0

        for trade in trades:
            total_quote_volume += trade.quote_volume
            if trade.side == taker_side:
                side_quote_volume += trade.quote_volume

        if total_quote_volume <= 0.0:
            return False

        imbalance = side_quote_volume / total_quote_volume
        return imbalance >= self._threshold

    def _check_spread_ticks(
        self,
        trigger: TriggerDecisionLike,
        onesec_window: OneSecondBarsProtocol,
    ) -> bool:
        """spread_ticks 检查。

        在 trigger_ts 对应的 1s bar 结束时刻，
        spread_ticks = (best_ask - best_bid) / tick_size。
        要求 spread_ticks <= s。

        查找 close_ts == trigger_ts 的 1s bar 作为 trigger 对应 bar。
        """
        bars = onesec_window.bars
        trigger_bar = None

        for bar in bars:
            if bar.close_ts == trigger.trigger_ts:
                trigger_bar = bar
                break

        if trigger_bar is None:
            # 找不到 trigger_ts 对应的 1s bar → 不通过
            return False

        if self._tick_size <= 0.0:
            return False

        spread = trigger_bar.best_ask - trigger_bar.best_bid
        spread_ticks = spread / self._tick_size

        return spread_ticks <= self._threshold
