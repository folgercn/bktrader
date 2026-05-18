"""TriggerConfirmationEvaluator — 在 [trigger_ts, trigger_ts + D] 区间内做二次验证。

四类 confirmation：
  - none：对照 baseline，仅需 intrabar trigger 成立即可，直接 confirmed=True。
  - persistence_n_seconds N∈{1,3,5,10}：1s_high >= prev_high_2（short 镜像）
    持续至少 N 秒。
  - retest_to_level retest_tick_buffer∈{0,1,2}：触发后价格回撤到
    prev_high_2 ± retest_tick_buffer × tick_size（short 镜像）再次与 level 接触。
  - min_volume_at_trigger min_notional_bps∈{50,100,200}：触发 1s bar 的
    notional >= min_notional_bps × pre-touch 20-bar median notional / 10000。
    分母 = signal_bar_start_ts 前 20 根已闭合 1h bar 的 quote-notional 中位数；
    20 根不足 → 跳过 + skip_reason=insufficient_history_skip。

tick_imbalance 公式（作为 Trigger_Confirmation 的可选组件）：
  tick_imbalance = sum(taker_side_quote_volume) / sum(all_taker_quote_volume)
  long 用 taker_side = buy、short 用 taker_side = sell
  窗口 [trigger_ts, trigger_ts + H]

任何新增 Trigger_Confirmation 变体 MUST 先扩展 TriggerConfirmationId 枚举。

Requirements: 2.3
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timedelta
from typing import Literal, Optional, Protocol, Sequence

from research.entry_redesign.detector.entry_trigger_detector import (
    OneSecondBar,
    OneSecondBars,
    TriggerDecision,
)
from research.entry_redesign.snapshot.runner_parameter_snapshot import (
    RunnerParameterSnapshot,
)


# ---------------------------------------------------------------------------
# Data Models
# ---------------------------------------------------------------------------


@dataclass(frozen=True)
class ConfirmationOutcome:
    """TriggerConfirmationEvaluator 的评估结果。

    Attributes:
        confirmed: 是否通过二次验证。
        skip_reason: 未通过时的原因标识；通过时为 None。
    """

    confirmed: bool
    skip_reason: Optional[
        Literal[
            "insufficient_history_skip",
            "persistence_timeout",
            "retest_timeout",
            "tick_imbalance_below_threshold",
        ]
    ] = None


# ---------------------------------------------------------------------------
# TakerTrade 与 TakerTradesWindow 协议
# ---------------------------------------------------------------------------


class TakerTrade(Protocol):
    """单笔 taker 成交记录协议。"""

    @property
    def ts(self) -> datetime:
        """成交时间戳（UTC）。"""
        ...

    @property
    def quote_volume(self) -> float:
        """该笔成交的 quote-notional（quote 币种计价的成交额）。"""
        ...

    @property
    def is_buyer_maker(self) -> bool:
        """True 表示 taker 为 sell 方向；False 表示 taker 为 buy 方向。"""
        ...


class TakerTradesWindow(Protocol):
    """Taker 成交窗口协议。

    提供指定时间范围内的 taker 成交记录序列。
    """

    @property
    def trades(self) -> Sequence[TakerTrade]:
        """按时间升序排列的 taker 成交记录。"""
        ...


# ---------------------------------------------------------------------------
# HourlyBar 协议（用于 min_volume 分母计算）
# ---------------------------------------------------------------------------


class HourlyBar(Protocol):
    """已闭合 1h bar 协议，用于 min_volume_at_trigger 分母计算。"""

    @property
    def close_ts(self) -> datetime:
        """该 1h bar 的结束时间戳（UTC）。"""
        ...

    @property
    def quote_volume(self) -> float:
        """该 1h bar 的 quote-notional（quote 币种计价的成交额）。"""
        ...


class HourlyBarsHistory(Protocol):
    """已闭合 1h bar 历史窗口协议。

    提供 signal_bar_start_ts 之前的已闭合 1h bar 序列。
    """

    @property
    def bars(self) -> Sequence[HourlyBar]:
        """按时间升序排列的已闭合 1h bar。"""
        ...


# ---------------------------------------------------------------------------
# 内部辅助：从 trigger_confirmation_id 解析参数
# ---------------------------------------------------------------------------


def _parse_persistence_n(confirmation_id: str) -> int:
    """从 'persistence_nN' 解析 N 值。"""
    # persistence_n1 / persistence_n3 / persistence_n5 / persistence_n10
    return int(confirmation_id.replace("persistence_n", ""))


def _parse_retest_tick_buffer(confirmation_id: str) -> int:
    """从 'retest_tbN' 解析 retest_tick_buffer 值。"""
    # retest_tb0 / retest_tb1 / retest_tb2
    return int(confirmation_id.replace("retest_tb", ""))


def _parse_minvol_bps(confirmation_id: str) -> int:
    """从 'minvol_bpsN' 解析 min_notional_bps 值。"""
    # minvol_bps50 / minvol_bps100 / minvol_bps200
    return int(confirmation_id.replace("minvol_bps", ""))


# ---------------------------------------------------------------------------
# TriggerConfirmationEvaluator
# ---------------------------------------------------------------------------


class TriggerConfirmationEvaluator:
    """在 [trigger_ts, trigger_ts + D] 区间内对 trigger 有效性的二次验证。

    根据 RunnerParameterSnapshot 中 candidate spec 的 trigger_confirmation_id
    选择对应的验证逻辑。
    """

    def __init__(self, snapshot: RunnerParameterSnapshot) -> None:
        """初始化。

        Args:
            snapshot: Runner 参数快照，包含 candidate spec 与 symbol_filters。
        """
        self._snapshot = snapshot
        self._confirmation_id = snapshot.candidate.trigger_confirmation_id
        self._entry_delay_seconds = snapshot.candidate.entry_delay_seconds
        self._feature_horizon_seconds = snapshot.candidate.feature_horizon_seconds
        self._tick_size_by_symbol = snapshot.symbol_filters.tick_size_by_symbol

    def evaluate(
        self,
        trigger: TriggerDecision,
        onesec_window: OneSecondBars,
        trades_window: TakerTradesWindow,
        hourly_history: Optional[HourlyBarsHistory] = None,
    ) -> ConfirmationOutcome:
        """评估 trigger 是否通过二次验证。

        Args:
            trigger: EntryTriggerDetector 产出的触发判定结果。
            onesec_window: 当前 signal bar 内的 1s bar 序列（时间升序）。
            trades_window: [trigger_ts, trigger_ts + H] 窗口内的 taker 成交记录。
            hourly_history: signal_bar_start_ts 之前的已闭合 1h bar 历史。
                仅 min_volume_at_trigger 类型需要；其他类型可为 None。

        Returns:
            ConfirmationOutcome，包含 confirmed 布尔值与可选的 skip_reason。
        """
        cid = self._confirmation_id

        if cid == "none":
            return self._evaluate_none()
        elif cid.startswith("persistence_n"):
            return self._evaluate_persistence(trigger, onesec_window)
        elif cid.startswith("retest_tb"):
            return self._evaluate_retest(trigger, onesec_window)
        elif cid.startswith("minvol_bps"):
            return self._evaluate_min_volume(
                trigger, trades_window, hourly_history
            )
        else:
            # 不应到达此处；类型系统已约束 TriggerConfirmationId
            raise ValueError(f"Unknown trigger_confirmation_id: {cid!r}")

    # -----------------------------------------------------------------------
    # none
    # -----------------------------------------------------------------------

    def _evaluate_none(self) -> ConfirmationOutcome:
        """none：对照 baseline，直接 confirmed。"""
        return ConfirmationOutcome(confirmed=True)

    # -----------------------------------------------------------------------
    # persistence_n_seconds
    # -----------------------------------------------------------------------

    def _evaluate_persistence(
        self,
        trigger: TriggerDecision,
        onesec_window: OneSecondBars,
    ) -> ConfirmationOutcome:
        """persistence_n_seconds：1s_high >= level（short 镜像）持续至少 N 秒。

        从 trigger_ts 开始，检查后续连续 N 根 1s bar 是否都满足
        触发条件（long: high >= level, short: low <= level）。
        每根 1s bar 代表 1 秒，连续 N 根即持续 N 秒。

        在 [trigger_ts, trigger_ts + D] 窗口内找不到连续 N 根满足条件
        的区间 → persistence_timeout。
        """
        n_seconds = _parse_persistence_n(self._confirmation_id)
        trigger_ts = trigger.trigger_ts
        side = trigger.side
        level = trigger.level
        d_seconds = self._entry_delay_seconds

        # 确认窗口上界
        window_end = trigger_ts + timedelta(seconds=d_seconds)

        # 筛选 [trigger_ts, window_end] 内的 1s bars（含 trigger_ts 本身的 bar）
        bars_in_window = [
            bar
            for bar in onesec_window.bars
            if trigger_ts <= bar.close_ts <= window_end
        ]

        if not bars_in_window:
            return ConfirmationOutcome(
                confirmed=False, skip_reason="persistence_timeout"
            )

        # 检查是否存在连续 N 根 1s bar 满足条件
        consecutive_count = 0

        for bar in bars_in_window:
            condition_met = (
                (side == "long" and bar.high >= level)
                or (side == "short" and bar.low <= level)
            )
            if condition_met:
                consecutive_count += 1
                if consecutive_count >= n_seconds:
                    return ConfirmationOutcome(confirmed=True)
            else:
                consecutive_count = 0

        return ConfirmationOutcome(
            confirmed=False, skip_reason="persistence_timeout"
        )

    # -----------------------------------------------------------------------
    # retest_to_level
    # -----------------------------------------------------------------------

    def _evaluate_retest(
        self,
        trigger: TriggerDecision,
        onesec_window: OneSecondBars,
    ) -> ConfirmationOutcome:
        """retest_to_level：触发后价格回撤到 level ± buffer 再次接触。

        long：触发后价格向上突破 level，retest 是价格回撤使得某根 1s bar
              的 low 落入 [level - buffer, level + buffer] 区间。
        short：触发后价格向下突破 level，retest 是价格回撤使得某根 1s bar
              的 high 落入 [level - buffer, level + buffer] 区间。

        buffer = retest_tick_buffer × tick_size
        tick_size 从 RunnerParameterSnapshot.symbol_filters.tick_size_by_symbol 读取。

        在 (trigger_ts, trigger_ts + D] 窗口内未发生 retest → retest_timeout。
        """
        retest_tick_buffer = _parse_retest_tick_buffer(self._confirmation_id)
        trigger_ts = trigger.trigger_ts
        side = trigger.side
        level = trigger.level
        d_seconds = self._entry_delay_seconds

        # 获取 tick_size：使用所有 symbol 中最小的 tick_size 作为保守估计。
        # 在实际 pipeline 中，每个 event 只对应一个 symbol，
        # 上游应确保 tick_size 与 event.symbol 一致。
        # 为支持 per-symbol 精确性，pipeline 可在构造 evaluator 时
        # 只传入当前 symbol 的 tick_size。
        tick_sizes = list(self._tick_size_by_symbol.values())
        tick_size = min(tick_sizes) if tick_sizes else 0.01

        buffer = retest_tick_buffer * tick_size
        window_end = trigger_ts + timedelta(seconds=d_seconds)

        # 筛选 trigger_ts 之后、window_end 之前的 bars
        # 使用严格大于 trigger_ts（retest 发生在触发之后）
        bars_after_trigger = [
            bar
            for bar in onesec_window.bars
            if trigger_ts < bar.close_ts <= window_end
        ]

        if not bars_after_trigger:
            return ConfirmationOutcome(
                confirmed=False, skip_reason="retest_timeout"
            )

        level_lower = level - buffer
        level_upper = level + buffer

        if side == "long":
            # long：retest = 价格回撤到 level 附近（low 落入 buffer 区间）
            for bar in bars_after_trigger:
                if level_lower <= bar.low <= level_upper:
                    return ConfirmationOutcome(confirmed=True)
        elif side == "short":
            # short：retest = 价格回撤到 level 附近（high 落入 buffer 区间）
            for bar in bars_after_trigger:
                if level_lower <= bar.high <= level_upper:
                    return ConfirmationOutcome(confirmed=True)

        return ConfirmationOutcome(
            confirmed=False, skip_reason="retest_timeout"
        )

    # -----------------------------------------------------------------------
    # min_volume_at_trigger
    # -----------------------------------------------------------------------

    def _evaluate_min_volume(
        self,
        trigger: TriggerDecision,
        trades_window: TakerTradesWindow,
        hourly_history: Optional[HourlyBarsHistory],
    ) -> ConfirmationOutcome:
        """min_volume_at_trigger：触发 1s bar 的 notional >= 阈值。

        阈值 = min_notional_bps × median_hourly_notional / 10000

        分母 = signal_bar_start_ts 前 20 根已闭合 1h bar 的 quote-notional 中位数。
        20 根不足 → 跳过 + skip_reason=insufficient_history_skip。
        禁止静默近似或回填。

        触发 1s bar 的 notional 从 trades_window 中筛选
        [trigger_bar.open_ts, trigger_bar.close_ts] 范围内的成交额总和计算。
        """
        min_notional_bps = _parse_minvol_bps(self._confirmation_id)

        # 检查 hourly_history 是否提供且有足够数据（>= 20 根）
        if hourly_history is None or len(hourly_history.bars) < 20:
            return ConfirmationOutcome(
                confirmed=False, skip_reason="insufficient_history_skip"
            )

        # 取最近 20 根已闭合 1h bar 的 quote-notional
        recent_20_bars = hourly_history.bars[-20:]
        hourly_notionals = [bar.quote_volume for bar in recent_20_bars]

        # 计算中位数
        median_notional = _median(hourly_notionals)

        # 阈值
        threshold = min_notional_bps * median_notional / 10000.0

        # 计算触发 1s bar 的 notional：
        # 从 trades_window 中筛选 trigger_ts 对应时间范围内的成交额
        trigger_ts = trigger.trigger_ts
        # 触发 bar 的时间范围：[trigger_ts - 1s, trigger_ts]
        # 因为 trigger_ts = bar.close_ts，bar 持续 1 秒
        bar_start = trigger_ts - timedelta(seconds=1)
        bar_end = trigger_ts

        trigger_bar_notional = sum(
            t.quote_volume
            for t in trades_window.trades
            if bar_start <= t.ts <= bar_end
        )

        if trigger_bar_notional >= threshold:
            return ConfirmationOutcome(confirmed=True)

        return ConfirmationOutcome(
            confirmed=False, skip_reason="insufficient_history_skip"
        )

    # -----------------------------------------------------------------------
    # tick_imbalance 辅助（供 PosttouchQualityGate 或组合使用）
    # -----------------------------------------------------------------------

    def compute_tick_imbalance(
        self,
        trigger: TriggerDecision,
        trades_window: TakerTradesWindow,
    ) -> Optional[float]:
        """计算 tick_imbalance。

        tick_imbalance = sum(taker_side_quote_volume) / sum(all_taker_quote_volume)
        long 用 taker_side = buy、short 用 taker_side = sell
        窗口 [trigger_ts, trigger_ts + H]

        Args:
            trigger: 触发判定结果。
            trades_window: 包含 [trigger_ts, trigger_ts + H] 窗口内成交的记录。

        Returns:
            tick_imbalance 值（0.0 ~ 1.0），或 None 如果窗口内无成交。
        """
        trigger_ts = trigger.trigger_ts
        h_seconds = self._feature_horizon_seconds
        window_end = trigger_ts + timedelta(seconds=h_seconds)
        side = trigger.side

        # 筛选 [trigger_ts, trigger_ts + H] 窗口内的 trades
        trades_in_window = [
            t
            for t in trades_window.trades
            if trigger_ts <= t.ts <= window_end
        ]

        if not trades_in_window:
            return None

        total_quote_volume = sum(t.quote_volume for t in trades_in_window)
        if total_quote_volume == 0.0:
            return None

        # taker_side: long 用 buy (is_buyer_maker=False 表示 taker 是 buyer)
        #             short 用 sell (is_buyer_maker=True 表示 taker 是 seller)
        if side == "long":
            taker_side_volume = sum(
                t.quote_volume for t in trades_in_window if not t.is_buyer_maker
            )
        else:
            taker_side_volume = sum(
                t.quote_volume for t in trades_in_window if t.is_buyer_maker
            )

        return taker_side_volume / total_quote_volume


# ---------------------------------------------------------------------------
# 辅助函数
# ---------------------------------------------------------------------------


def _median(values: list[float]) -> float:
    """计算中位数（不依赖 numpy，保持确定性）。"""
    sorted_values = sorted(values)
    n = len(sorted_values)
    if n == 0:
        return 0.0
    mid = n // 2
    if n % 2 == 0:
        return (sorted_values[mid - 1] + sorted_values[mid]) / 2.0
    return sorted_values[mid]
