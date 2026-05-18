"""EntryTriggerDetector — Intrabar breakout 触发判定唯一入口。

Intrabar_Breakout_Semantic 实现：
  - long 触发条件：当前未闭合 signal bar 内存在至少一个 1s_high_i >= prev_high_2
    的 1s bar，trigger_ts 取最早一根满足条件的 1s bar 结束时间戳（秒级精度）。
  - short 镜像：1s_low_i <= prev_low_2。
  - 不满足条件时返回 None。

禁止使用任何封闭 bar 收盘语义。本模块不得出现以下禁词字面值：
  - 闭合 signal bar 收盘确认
  - 1h close confirm
  - close > prev_high_2 后入场
  - 1h K 线收盘确认突破

Requirements: 1.4, 6.1
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime
from typing import Literal, Optional, Protocol, Sequence


# ---------------------------------------------------------------------------
# Data Models
# ---------------------------------------------------------------------------


@dataclass(frozen=True)
class TriggerDecision:
    """Intrabar breakout 触发判定结果。

    Attributes:
        trigger_ts: 最早满足触发条件的 1s bar 结束时间戳（秒级精度，UTC）。
        side: 触发方向，"long" 或 "short"。
        level: 触发参考价位（long 为 prev_high_2，short 为 prev_low_2）。
    """

    trigger_ts: datetime
    side: Literal["long", "short"]
    level: float


@dataclass(frozen=True)
class OneSecondBar:
    """单根 1 秒 K 线数据。

    Attributes:
        open_ts: 该 1s bar 的开始时间戳（UTC）。
        close_ts: 该 1s bar 的结束时间戳（UTC，秒级精度）。
        high: 该 1s bar 的最高价。
        low: 该 1s bar 的最低价。
    """

    open_ts: datetime
    close_ts: datetime
    high: float
    low: float


class Event(Protocol):
    """入口事件协议（events_execution_labeled.csv 每行一条）。

    本协议仅声明 EntryTriggerDetector 所需的最小字段集合。
    完整 Event dataclass 由上游模块定义。
    """

    @property
    def symbol(self) -> str: ...

    @property
    def side(self) -> Literal["long", "short"]: ...

    @property
    def signal_bar_start_ts(self) -> datetime: ...

    @property
    def signal_bar_end_ts(self) -> datetime: ...

    @property
    def prev_high_2(self) -> float: ...

    @property
    def prev_low_2(self) -> float: ...


class OneSecondBars(Protocol):
    """1 秒 K 线窗口协议。

    提供当前未闭合 signal bar 内的 1s bar 序列，
    按时间升序排列（chronological order）。
    """

    @property
    def bars(self) -> Sequence[OneSecondBar]: ...


# ---------------------------------------------------------------------------
# EntryTriggerDetector
# ---------------------------------------------------------------------------


class EntryTriggerDetector:
    """Intrabar breakout 触发判定器。

    唯一的 Intrabar breakout 判定入口。

    保证:
      * 当 side=long 且存在至少一个 1s_high_i >= prev_high_2 的 1s bar 时，
        trigger_ts 取最早一根满足条件的 1s bar 结束时间戳（秒级精度）。
      * short 镜像，条件替换为 1s_low_i <= prev_low_2。
      * 不满足条件时返回 None；不得 spurious 生成 trigger_ts（P1）。
    """

    def __init__(self, tick_size_by_symbol: dict[str, float]) -> None:
        """初始化触发判定器。

        Args:
            tick_size_by_symbol: 每个 symbol 的 tick_size 映射。
                例如 {"BTCUSDT": 0.10, "ETHUSDT": 0.01}。
                由 RunnerParameterSnapshot.symbol_filters.tick_size_by_symbol 提供。
        """
        self._tick_size_by_symbol = tick_size_by_symbol

    def detect(
        self,
        event: Event,
        onesec_window: OneSecondBars,
    ) -> Optional[TriggerDecision]:
        """判定当前未闭合 signal bar 内是否存在 intrabar breakout 触发。

        遍历 signal bar 内的 1s bar（按时间升序），找到最早满足触发条件的
        1s bar，返回 TriggerDecision；若无满足条件的 bar 则返回 None。

        long 触发条件: 1s_high_i >= prev_high_2
        short 触发条件: 1s_low_i <= prev_low_2

        trigger_ts 取最早满足条件的 1s bar 结束时间戳（秒级精度）。

        Args:
            event: 入口事件，包含 side、prev_high_2、prev_low_2 等字段。
            onesec_window: 当前未闭合 signal bar 内的 1s bar 序列（时间升序）。

        Returns:
            TriggerDecision 如果触发条件成立，否则 None。
        """
        side = event.side
        bars = onesec_window.bars

        if side == "long":
            level = event.prev_high_2
            for bar in bars:
                if bar.high >= level:
                    return TriggerDecision(
                        trigger_ts=bar.close_ts,
                        side="long",
                        level=level,
                    )
        elif side == "short":
            level = event.prev_low_2
            for bar in bars:
                if bar.low <= level:
                    return TriggerDecision(
                        trigger_ts=bar.close_ts,
                        side="short",
                        level=level,
                    )

        # 不满足触发条件 → 返回 None
        return None
