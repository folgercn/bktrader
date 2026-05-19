"""EntryPriceResolver — 唯一的下单价决策入口。

四类 Entry_Price_Mode：
  - market_on_touch：触发即 market，entry_price = trigger.level。
  - limit_at_level：limit 挂单在 prev_high_2 / prev_low_2。
  - limit_at_level_plus_tick_buffer：long = prev_high_2 + k × tick_size，
    short = prev_low_2 - k × tick_size，k ∈ {0, 1, 2, 4}。
  - post_touch_pullback_limit：触发后在 [trigger_ts, trigger_ts + D] 内
    pullback 到 prev_high_2 - p × ATR (long) / prev_low_2 + p × ATR (short)，
    p ∈ {0.00, 0.02, 0.05, 0.10}。

post_touch_pullback_limit 未成交 fallback 行为固定为"空仓（跳过该笔）"：
  - MUST NOT 二次落到下一根 bar
  - MUST NOT 用 market_on_touch 兜底
  - MUST NOT 在 D 耗尽后继续追加 D'

未成交事件写入 attribution pullback_limit_unfilled_count，不进 ledger。

Requirements: 2.4, 6.11
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timedelta
from typing import Literal, Optional

from research.entry_redesign.detector.entry_trigger_detector import (
    OneSecondBars,
    TriggerDecision,
)
from research.entry_redesign.snapshot.runner_parameter_snapshot import (
    RunnerParameterSnapshot,
)


# ---------------------------------------------------------------------------
# PriceResolution dataclass
# ---------------------------------------------------------------------------


@dataclass(frozen=True)
class PriceResolution:
    """下单价决策结果。

    Attributes:
        filled: 是否成交。True 表示在窗口内成交，False 表示未成交（跳过）。
        entry_price: 成交价格。filled=True 时为 float，filled=False 时为 None。
        entry_ts: 成交时间戳（UTC）。filled=True 时为 datetime，
            filled=False 时为 None。
        unfilled_reason: 未成交原因。filled=False 时为字面值字符串，
            filled=True 时为 None。
    """

    filled: bool
    entry_price: Optional[float]
    entry_ts: Optional[datetime]
    unfilled_reason: Optional[Literal["pullback_limit_not_touched_in_window"]]


# ---------------------------------------------------------------------------
# pullback_limit p 值映射
# ---------------------------------------------------------------------------

_PULLBACK_P_MAP: dict[str, float] = {
    "pullback_p000": 0.00,
    "pullback_p002": 0.02,
    "pullback_p005": 0.05,
    "pullback_p010": 0.10,
}

# ---------------------------------------------------------------------------
# limit_at_level_plus_tick_buffer k 值映射
# ---------------------------------------------------------------------------

_TICK_BUFFER_K_MAP: dict[str, int] = {
    "limit_tb_k0": 0,
    "limit_tb_k1": 1,
    "limit_tb_k2": 2,
    "limit_tb_k4": 4,
}


# ---------------------------------------------------------------------------
# EntryPriceResolver
# ---------------------------------------------------------------------------


class EntryPriceResolver:
    """唯一的下单价决策入口。

    根据 RunnerParameterSnapshot 中的 entry_price_mode_id 决定入场价格。
    post_touch_pullback_limit 在 [trigger_ts, trigger_ts + D] 内未成交时
    fallback 行为固定为"空仓（跳过该笔）"——MUST NOT 二次落到下一根 bar、
    MUST NOT market_on_touch 兜底、MUST NOT 在 D 耗尽后继续追加 D'。
    """

    def __init__(self, snapshot: RunnerParameterSnapshot) -> None:
        """初始化 EntryPriceResolver。

        Args:
            snapshot: Runner 参数快照，提供 entry_price_mode_id、
                symbol_filters.tick_size_by_symbol、features.atr14_source、
                candidate.entry_delay_seconds 等参数。
        """
        self._snapshot = snapshot
        self._mode = snapshot.candidate.entry_price_mode_id
        self._delay_seconds = snapshot.candidate.entry_delay_seconds
        self._tick_size_by_symbol = snapshot.symbol_filters.tick_size_by_symbol
        # 上下文字段，由 pipeline 在每次 resolve() 前通过 set_context() 注入
        self._current_atr14: float = 0.0
        self._current_symbol: str = ""

    def set_context(self, *, atr14: float, symbol: str) -> None:
        """由 pipeline 在每次 resolve() 调用前注入上下文。

        ATR 取 signal_bar_start_ts 前最后一根已闭合 1h bar 的 ATR(14)，
        来自 features.atr14_source CSV。symbol 用于查找 tick_size。

        Args:
            atr14: signal_bar_start_ts 前最后一根已闭合 1h bar 的 ATR(14)。
            symbol: 当前 event 的 symbol（用于查找 tick_size）。
        """
        self._current_atr14 = atr14
        self._current_symbol = symbol

    def resolve(
        self,
        trigger: TriggerDecision,
        onesec_window: OneSecondBars,
    ) -> PriceResolution:
        """决定入场价格。

        根据 entry_price_mode_id 分派到对应的定价逻辑。

        Args:
            trigger: 触发判定结果，包含 trigger_ts、side、level。
            onesec_window: 当前未闭合 signal bar 内的 1s bar 序列（时间升序）。

        Returns:
            PriceResolution 表示成交或未成交结果。
        """
        mode = self._mode

        if mode == "market_on_touch":
            return self._resolve_market_on_touch(trigger)
        elif mode == "limit_at_level":
            return self._resolve_limit_at_level(trigger)
        elif mode in _TICK_BUFFER_K_MAP:
            return self._resolve_limit_at_level_plus_tick_buffer(trigger)
        elif mode in _PULLBACK_P_MAP:
            return self._resolve_post_touch_pullback_limit(
                trigger, onesec_window
            )
        else:
            # 不应到达此处——entry_price_mode_id 由 Literal 类型约束
            raise ValueError(f"Unknown entry_price_mode_id: {mode!r}")

    # ------------------------------------------------------------------
    # market_on_touch：触发即 market
    # ------------------------------------------------------------------

    def _resolve_market_on_touch(
        self,
        trigger: TriggerDecision,
    ) -> PriceResolution:
        """market_on_touch：触发即以 trigger.level 成交。

        entry_price = trigger.level（即 prev_high_2 或 prev_low_2）。
        entry_ts = trigger.trigger_ts。
        """
        return PriceResolution(
            filled=True,
            entry_price=trigger.level,
            entry_ts=trigger.trigger_ts,
            unfilled_reason=None,
        )

    # ------------------------------------------------------------------
    # limit_at_level：limit 挂单在 prev_high_2 / prev_low_2
    # ------------------------------------------------------------------

    def _resolve_limit_at_level(
        self,
        trigger: TriggerDecision,
    ) -> PriceResolution:
        """limit_at_level：以 level 价格成交。

        由于 trigger 已经确认价格触及 level（long: high >= level,
        short: low <= level），limit 挂单在 level 必然成交。
        entry_price = trigger.level。
        entry_ts = trigger.trigger_ts。
        """
        return PriceResolution(
            filled=True,
            entry_price=trigger.level,
            entry_ts=trigger.trigger_ts,
            unfilled_reason=None,
        )

    # ------------------------------------------------------------------
    # limit_at_level_plus_tick_buffer：level ± k × tick_size
    # ------------------------------------------------------------------

    def _resolve_limit_at_level_plus_tick_buffer(
        self,
        trigger: TriggerDecision,
    ) -> PriceResolution:
        """limit_at_level_plus_tick_buffer：在 level 基础上加 tick buffer。

        long: entry_price = prev_high_2 + k × tick_size
        short: entry_price = prev_low_2 - k × tick_size

        由于 trigger 已确认价格触及 level，且 tick buffer 方向与突破方向一致
        （long 加价、short 减价），挂单在 level + buffer 处必然成交
        （价格已经越过 level，buffer 方向是更有利于成交的方向）。
        """
        k = _TICK_BUFFER_K_MAP[self._mode]
        tick_size = self._get_tick_size()

        if trigger.side == "long":
            entry_price = trigger.level + k * tick_size
        else:  # short
            entry_price = trigger.level - k * tick_size

        return PriceResolution(
            filled=True,
            entry_price=entry_price,
            entry_ts=trigger.trigger_ts,
            unfilled_reason=None,
        )

    # ------------------------------------------------------------------
    # post_touch_pullback_limit：pullback 到 level ∓ p × ATR
    # ------------------------------------------------------------------

    def _resolve_post_touch_pullback_limit(
        self,
        trigger: TriggerDecision,
        onesec_window: OneSecondBars,
    ) -> PriceResolution:
        """post_touch_pullback_limit：触发后等待 pullback 到目标价。

        long: 目标价 = prev_high_2 - p × ATR（等待价格回撤到 level 下方）
        short: 目标价 = prev_low_2 + p × ATR（等待价格回撤到 level 上方）

        在 [trigger_ts, trigger_ts + D] 窗口内扫描 1s bar：
          - long: 若存在 1s_low <= 目标价，则成交，entry_price = 目标价，
            entry_ts = 该 1s bar 的 close_ts。
          - short: 若存在 1s_high >= 目标价，则成交。

        未成交 fallback 固定为"空仓（跳过该笔）"：
          - MUST NOT 二次落到下一根 bar
          - MUST NOT 用 market_on_touch 兜底
          - MUST NOT 在 D 耗尽后继续追加 D'
        """
        p = _PULLBACK_P_MAP[self._mode]
        atr14 = self._current_atr14
        delay_seconds = self._delay_seconds

        # 计算目标价
        if trigger.side == "long":
            # long pullback: 价格从 level 上方回撤到 level - p * ATR
            target_price = trigger.level - p * atr14
        else:  # short
            # short pullback: 价格从 level 下方回弹到 level + p * ATR
            target_price = trigger.level + p * atr14

        # 定义时间窗口 [trigger_ts, trigger_ts + D]
        window_start = trigger.trigger_ts
        window_end = window_start + timedelta(seconds=delay_seconds)

        # 扫描窗口内的 1s bar，寻找 pullback 成交
        for bar in onesec_window.bars:
            # 只看 trigger_ts 之后（含）且在 window_end 之前（含）的 bar
            if bar.close_ts < window_start:
                continue
            if bar.close_ts > window_end:
                break

            if trigger.side == "long":
                # long pullback: 需要价格回撤到目标价以下
                if bar.low <= target_price:
                    return PriceResolution(
                        filled=True,
                        entry_price=target_price,
                        entry_ts=bar.close_ts,
                        unfilled_reason=None,
                    )
            else:  # short
                # short pullback: 需要价格回弹到目标价以上
                if bar.high >= target_price:
                    return PriceResolution(
                        filled=True,
                        entry_price=target_price,
                        entry_ts=bar.close_ts,
                        unfilled_reason=None,
                    )

        # 窗口内未成交 → 空仓（跳过该笔）
        # MUST NOT 二次落到下一根 bar
        # MUST NOT 用 market_on_touch 兜底
        # MUST NOT 在 D 耗尽后继续追加 D'
        return PriceResolution(
            filled=False,
            entry_price=None,
            entry_ts=None,
            unfilled_reason="pullback_limit_not_touched_in_window",
        )

    # ------------------------------------------------------------------
    # 辅助方法
    # ------------------------------------------------------------------

    def _get_tick_size(self) -> float:
        """获取当前 symbol 的 tick_size。

        优先使用 set_context() 注入的 symbol 对应的 tick_size；
        若 symbol 未设置或不在映射中，fallback 到 tick_size_by_symbol 中
        唯一的值或按字典序第一个。

        Returns:
            当前 symbol 的 tick_size。

        Raises:
            ValueError: tick_size_by_symbol 为空。
        """
        if not self._tick_size_by_symbol:
            raise ValueError("tick_size_by_symbol is empty")

        # 优先使用 set_context() 注入的 symbol
        if self._current_symbol and self._current_symbol in self._tick_size_by_symbol:
            return self._tick_size_by_symbol[self._current_symbol]

        # fallback：单 symbol 直接返回
        if len(self._tick_size_by_symbol) == 1:
            return next(iter(self._tick_size_by_symbol.values()))

        # 多 symbol 无上下文：按字典序第一个
        return self._tick_size_by_symbol[sorted(self._tick_size_by_symbol.keys())[0]]
