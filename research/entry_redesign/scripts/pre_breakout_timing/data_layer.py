"""
data_layer — 数据加载层（复用 dynamic_timing）

直接 import dynamic_timing.data_layer 的公开接口，无需新增数据加载逻辑。
所有数据源与 dynamic-entry-timing 实验一致：
- V6 gate 选中的 ~116 unique events
- 1s bar pickle cache
- 60/40 time-split（按 touch_time 排序）
"""

from __future__ import annotations

from dynamic_timing.data_layer import (
    load_bars_cache,
    load_v6_gate_events,
    time_split_events,
)

__all__ = [
    "load_v6_gate_events",
    "load_bars_cache",
    "time_split_events",
]
