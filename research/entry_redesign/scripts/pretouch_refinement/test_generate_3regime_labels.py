"""
Unit tests for generate_3regime_labels().

验证三分类标签分配逻辑：skip / fast / slow（含 5bps 容差规则）。
"""

from __future__ import annotations

import pandas as pd
import numpy as np
import pytest

from pretouch_refinement.regime_labels import generate_3regime_labels, FAST_DELAYS, SLOW_DELAYS


def _build_matrix(event_pnls: list[dict[str, float]]) -> pd.DataFrame:
    """辅助函数：从 event pnl 字典列表构建 delay_pnl_matrix 格式的 DataFrame。

    Parameters
    ----------
    event_pnls : list[dict[str, float]]
        每个 dict 的 key 为 delay_label，value 为 pnl_pct。
    """
    rows = []
    for i, pnls in enumerate(event_pnls):
        eid = f"event_{i:03d}"
        for delay_label, pnl_pct in pnls.items():
            rows.append({
                "event_id": eid,
                "delay_label": delay_label,
                "pnl_pct": pnl_pct,
                "traded": True,
            })
    return pd.DataFrame(rows)


def _build_events(n: int) -> pd.DataFrame:
    """辅助函数：构建 events DataFrame。"""
    return pd.DataFrame({
        "event_id": [f"event_{i:03d}" for i in range(n)],
    })


class TestGenerate3RegimeLabels:
    """测试 generate_3regime_labels 的标签分配逻辑。"""

    def test_skip_when_both_negative(self):
        """fast_pnl < 0 AND slow_pnl < 0 → skip"""
        matrix = _build_matrix([
            {"D0": -0.01, "D5": -0.005, "D10": -0.02, "D15": -0.015, "pullback": -0.008},
        ])
        events = _build_events(1)
        result = generate_3regime_labels(matrix, events)
        assert result.iloc[0] == "skip"

    def test_fast_when_fast_greater(self):
        """fast_pnl > slow_pnl → fast"""
        matrix = _build_matrix([
            {"D0": 0.02, "D5": 0.015, "D10": 0.005, "D15": 0.003, "pullback": 0.001},
        ])
        events = _build_events(1)
        result = generate_3regime_labels(matrix, events)
        assert result.iloc[0] == "fast"

    def test_slow_when_slow_much_greater(self):
        """slow_pnl > fast_pnl 且差距 > 5bps → slow"""
        matrix = _build_matrix([
            {"D0": 0.001, "D5": 0.002, "D10": 0.01, "D15": 0.015, "pullback": 0.012},
        ])
        events = _build_events(1)
        result = generate_3regime_labels(matrix, events)
        assert result.iloc[0] == "slow"

    def test_fast_when_equal(self):
        """fast_pnl == slow_pnl → fast（优先 fast）"""
        matrix = _build_matrix([
            {"D0": 0.01, "D5": 0.005, "D10": 0.01, "D15": 0.005, "pullback": 0.003},
        ])
        events = _build_events(1)
        result = generate_3regime_labels(matrix, events)
        assert result.iloc[0] == "fast"

    def test_fast_within_5bps_tolerance(self):
        """slow_pnl - fast_pnl < 5bps (0.0005) → fast（容差规则）"""
        # slow_pnl = 0.0104, fast_pnl = 0.01 → diff = 0.0004 < 0.0005
        matrix = _build_matrix([
            {"D0": 0.01, "D5": 0.008, "D10": 0.0104, "D15": 0.005, "pullback": 0.003},
        ])
        events = _build_events(1)
        result = generate_3regime_labels(matrix, events)
        assert result.iloc[0] == "fast"

    def test_slow_at_exactly_5bps_boundary(self):
        """slow_pnl - fast_pnl == 5bps (0.0005) → slow（不满足 < tolerance）"""
        # slow_pnl = 0.0105, fast_pnl = 0.01 → diff = 0.0005 == tolerance → slow
        matrix = _build_matrix([
            {"D0": 0.01, "D5": 0.008, "D10": 0.0105, "D15": 0.005, "pullback": 0.003},
        ])
        events = _build_events(1)
        result = generate_3regime_labels(matrix, events)
        assert result.iloc[0] == "slow"

    def test_fast_when_fast_positive_slow_negative(self):
        """fast_pnl > 0, slow_pnl < 0 → fast"""
        matrix = _build_matrix([
            {"D0": 0.005, "D5": 0.003, "D10": -0.01, "D15": -0.005, "pullback": -0.002},
        ])
        events = _build_events(1)
        result = generate_3regime_labels(matrix, events)
        assert result.iloc[0] == "fast"

    def test_slow_when_slow_positive_fast_negative(self):
        """fast_pnl < 0, slow_pnl > 0 → slow（差距必然 > 5bps）"""
        matrix = _build_matrix([
            {"D0": -0.005, "D5": -0.003, "D10": 0.01, "D15": 0.005, "pullback": 0.002},
        ])
        events = _build_events(1)
        result = generate_3regime_labels(matrix, events)
        assert result.iloc[0] == "slow"

    def test_multiple_events(self):
        """多个 event 的标签分配"""
        matrix = _build_matrix([
            # event_000: both negative → skip
            {"D0": -0.01, "D5": -0.005, "D10": -0.02, "D15": -0.015, "pullback": -0.008},
            # event_001: fast > slow → fast
            {"D0": 0.02, "D5": 0.015, "D10": 0.005, "D15": 0.003, "pullback": 0.001},
            # event_002: slow >> fast → slow
            {"D0": 0.001, "D5": 0.002, "D10": 0.01, "D15": 0.015, "pullback": 0.012},
        ])
        events = _build_events(3)
        result = generate_3regime_labels(matrix, events)
        assert list(result) == ["skip", "fast", "slow"]

    def test_deterministic(self):
        """确定性保证：两次运行结果逐 event 一致"""
        matrix = _build_matrix([
            {"D0": -0.01, "D5": -0.005, "D10": -0.02, "D15": -0.015, "pullback": -0.008},
            {"D0": 0.02, "D5": 0.015, "D10": 0.005, "D15": 0.003, "pullback": 0.001},
            {"D0": 0.001, "D5": 0.002, "D10": 0.01, "D15": 0.015, "pullback": 0.012},
            {"D0": 0.01, "D5": 0.008, "D10": 0.0104, "D15": 0.005, "pullback": 0.003},
        ])
        events = _build_events(4)
        result1 = generate_3regime_labels(matrix, events)
        result2 = generate_3regime_labels(matrix, events)
        assert list(result1) == list(result2)

    def test_index_aligned_with_events(self):
        """结果 Series 的 index 与 events 对齐"""
        matrix = _build_matrix([
            {"D0": 0.01, "D5": 0.005, "D10": 0.003, "D15": 0.002, "pullback": 0.001},
        ])
        events = pd.DataFrame({"event_id": ["event_000"]}, index=[42])
        result = generate_3regime_labels(matrix, events)
        assert result.index.tolist() == [42]

    def test_output_values_in_valid_labels(self):
        """输出值只包含 skip / fast / slow"""
        matrix = _build_matrix([
            {"D0": -0.01, "D5": -0.005, "D10": -0.02, "D15": -0.015, "pullback": -0.008},
            {"D0": 0.02, "D5": 0.015, "D10": 0.005, "D15": 0.003, "pullback": 0.001},
            {"D0": 0.001, "D5": 0.002, "D10": 0.01, "D15": 0.015, "pullback": 0.012},
        ])
        events = _build_events(3)
        result = generate_3regime_labels(matrix, events)
        assert set(result.unique()).issubset({"skip", "fast", "slow"})

    def test_custom_tolerance(self):
        """自定义容差阈值"""
        # slow_pnl = 0.011, fast_pnl = 0.01 → diff = 0.001
        # 默认 5bps (0.0005): diff > tolerance → slow
        # 自定义 15bps (0.0015): diff < tolerance → fast
        matrix = _build_matrix([
            {"D0": 0.01, "D5": 0.008, "D10": 0.011, "D15": 0.005, "pullback": 0.003},
        ])
        events = _build_events(1)

        result_default = generate_3regime_labels(matrix, events, tolerance_bps=5.0)
        assert result_default.iloc[0] == "slow"

        result_custom = generate_3regime_labels(matrix, events, tolerance_bps=15.0)
        assert result_custom.iloc[0] == "fast"
