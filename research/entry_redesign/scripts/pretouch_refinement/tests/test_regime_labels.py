"""
Unit tests for regime_labels module.

覆盖：
- load_delay_pnl_matrix 验证失败场景
- generate_2regime_labels 确定性、Best_Global_Delay 选择、pullback 排除、enter/skip 逻辑
- generate_3regime_labels 5bps 边界行为
- compute_label_distributions 输出格式与 imbalance 检测

Requirements: 2.1, 2.2, 2.5
"""

from __future__ import annotations

import tempfile
from pathlib import Path

import numpy as np
import pandas as pd
import pytest

from pretouch_refinement.regime_labels import (
    compute_label_distributions,
    generate_2regime_labels,
    generate_3regime_labels,
    load_delay_pnl_matrix,
    GLOBAL_DELAY_CANDIDATES,
    REQUIRED_COLUMNS,
)


# ---------------------------------------------------------------------------
# 辅助函数
# ---------------------------------------------------------------------------


def _build_matrix_580(
    n_events: int = 116,
    delays: list[str] | None = None,
    extra_cols: list[str] | None = None,
) -> pd.DataFrame:
    """构建符合 580 行 × 15 列预期的合成 delay_pnl_matrix。

    默认 116 events × 5 delays = 580 行。
    """
    if delays is None:
        delays = ["D0", "D5", "D10", "D15", "pullback"]
    if extra_cols is None:
        extra_cols = []

    rows = []
    for i in range(n_events):
        eid = f"event_{i:03d}"
        symbol = "BTCUSDT" if i % 2 == 0 else "ETHUSDT"
        month = f"2024-{(i % 12) + 1:02d}"
        for delay in delays:
            rows.append({
                "event_id": eid,
                "delay_label": delay,
                "pnl_pct": np.random.default_rng(42 + i).uniform(-0.02, 0.02),
                "traded": True,
                "symbol": symbol,
                "entry_time": f"2024-{(i % 12) + 1:02d}-15T10:00:00+00:00",
                "exit_time": f"2024-{(i % 12) + 1:02d}-15T11:00:00+00:00",
                "entry_price": 50000.0 + i * 10,
                "exit_price": 50010.0 + i * 10,
                "side": "long",
                "notional": 26000.0,
                "slippage_bps": 2.0,
            })

    df = pd.DataFrame(rows)
    # 确保恰好 15 列（补齐或截断）
    # 当前有 12 列，需要补 3 列
    needed = 15 - len(df.columns)
    for j in range(needed):
        df[f"extra_col_{j}"] = 0.0

    # 截断到 15 列
    df = df.iloc[:, :15]
    return df


def _build_simple_matrix(event_pnls: list[dict[str, float]], with_silo_cols: bool = False) -> pd.DataFrame:
    """从 event pnl 字典列表构建 delay_pnl_matrix 格式的 DataFrame。

    Parameters
    ----------
    event_pnls : list[dict[str, float]]
        每个 dict 的 key 为 delay_label，value 为 pnl_pct。
    with_silo_cols : bool
        是否添加 symbol/entry_time 列（generate_2regime_labels 需要）。
    """
    rows = []
    for i, pnls in enumerate(event_pnls):
        eid = f"event_{i:03d}"
        symbol = "BTCUSDT" if i % 2 == 0 else "ETHUSDT"
        month_idx = (i % 12) + 1
        for delay_label, pnl_pct in pnls.items():
            row = {
                "event_id": eid,
                "delay_label": delay_label,
                "pnl_pct": pnl_pct,
                "traded": True,
            }
            if with_silo_cols:
                row["symbol"] = symbol
                row["entry_time"] = f"2024-{month_idx:02d}-15T10:00:00+00:00"
            rows.append(row)
    return pd.DataFrame(rows)


def _build_events(n: int) -> pd.DataFrame:
    """构建 events DataFrame。"""
    return pd.DataFrame({
        "event_id": [f"event_{i:03d}" for i in range(n)],
    })


# ===========================================================================
# Test: load_delay_pnl_matrix 验证失败场景
# ===========================================================================


class TestLoadDelayPnlMatrix:
    """测试 load_delay_pnl_matrix 的验证逻辑。"""

    def test_load_delay_pnl_matrix_file_not_found(self, tmp_path: Path):
        """ValueError when file doesn't exist."""
        fake_path = str(tmp_path / "nonexistent.csv")
        with pytest.raises(ValueError, match="不存在"):
            load_delay_pnl_matrix(fake_path)

    def test_load_delay_pnl_matrix_wrong_shape(self, tmp_path: Path):
        """ValueError when shape is wrong (not 580×15)."""
        # 创建一个 100 行 × 10 列的 CSV
        df = pd.DataFrame(
            np.random.default_rng(42).random((100, 10)),
            columns=[f"col_{i}" for i in range(10)],
        )
        csv_path = tmp_path / "wrong_shape.csv"
        df.to_csv(csv_path, index=False)

        with pytest.raises(ValueError, match="shape 不符"):
            load_delay_pnl_matrix(str(csv_path))

    def test_load_delay_pnl_matrix_missing_columns(self, tmp_path: Path):
        """ValueError when required columns missing."""
        # 创建 580 行 × 15 列但缺少 required columns
        df = pd.DataFrame(
            np.random.default_rng(42).random((580, 15)),
            columns=[f"col_{i}" for i in range(15)],
        )
        csv_path = tmp_path / "missing_cols.csv"
        df.to_csv(csv_path, index=False)

        with pytest.raises(ValueError, match="缺少必要列"):
            load_delay_pnl_matrix(str(csv_path))


# ===========================================================================
# Test: generate_2regime_labels
# ===========================================================================


class TestGenerate2RegimeLabels:
    """测试 generate_2regime_labels 的标签分配逻辑。"""

    def test_generate_2regime_labels_deterministic(self):
        """两次运行产生完全相同的结果（确定性保证）。"""
        # 构建含 silo 信息的 matrix
        event_pnls = [
            {"D0": 0.01, "D5": 0.005, "D10": -0.002, "D15": 0.003, "pullback": -0.001},
            {"D0": -0.005, "D5": 0.008, "D10": 0.012, "D15": 0.001, "pullback": 0.002},
            {"D0": 0.003, "D5": -0.001, "D10": 0.005, "D15": 0.007, "pullback": -0.003},
            {"D0": -0.01, "D5": -0.002, "D10": -0.005, "D15": -0.001, "pullback": -0.004},
        ]
        matrix = _build_simple_matrix(event_pnls, with_silo_cols=True)
        train_events = _build_events(3)  # 前 3 个为 train
        all_events = _build_events(4)

        labels1, delay1 = generate_2regime_labels(matrix, train_events, all_events)
        labels2, delay2 = generate_2regime_labels(matrix, train_events, all_events)

        assert list(labels1) == list(labels2)
        assert delay1 == delay2

    def test_generate_2regime_labels_best_global_delay_from_train(self):
        """验证 best_global_delay 仅从 train set 计算。

        构造场景：train set 中 D5 的 pnl 明显最高，
        test set 中 D10 的 pnl 最高。best_global_delay 应为 D5。
        """
        # 4 个 events: 前 2 个为 train，后 2 个为 test
        # Train events: D5 有高正 pnl
        # Test events: D10 有高正 pnl（不应影响 best_global_delay 选择）
        rows = []
        # Train event 0 (BTCUSDT, 2024-01)
        for delay, pnl in [("D0", 0.001), ("D5", 0.05), ("D10", 0.002), ("D15", 0.001), ("pullback", 0.0)]:
            rows.append({"event_id": "event_000", "delay_label": delay, "pnl_pct": pnl,
                         "traded": True, "symbol": "BTCUSDT", "entry_time": "2024-01-15T10:00:00+00:00"})
        # Train event 1 (ETHUSDT, 2024-02)
        for delay, pnl in [("D0", 0.002), ("D5", 0.04), ("D10", 0.003), ("D15", 0.001), ("pullback", 0.0)]:
            rows.append({"event_id": "event_001", "delay_label": delay, "pnl_pct": pnl,
                         "traded": True, "symbol": "ETHUSDT", "entry_time": "2024-02-15T10:00:00+00:00"})
        # Test event 2 (BTCUSDT, 2024-03) - D10 最高
        for delay, pnl in [("D0", 0.001), ("D5", 0.001), ("D10", 0.10), ("D15", 0.001), ("pullback", 0.0)]:
            rows.append({"event_id": "event_002", "delay_label": delay, "pnl_pct": pnl,
                         "traded": True, "symbol": "BTCUSDT", "entry_time": "2024-03-15T10:00:00+00:00"})
        # Test event 3 (ETHUSDT, 2024-04) - D10 最高
        for delay, pnl in [("D0", 0.001), ("D5", 0.001), ("D10", 0.08), ("D15", 0.001), ("pullback", 0.0)]:
            rows.append({"event_id": "event_003", "delay_label": delay, "pnl_pct": pnl,
                         "traded": True, "symbol": "ETHUSDT", "entry_time": "2024-04-15T10:00:00+00:00"})

        matrix = pd.DataFrame(rows)
        train_events = pd.DataFrame({"event_id": ["event_000", "event_001"]})
        all_events = pd.DataFrame({"event_id": ["event_000", "event_001", "event_002", "event_003"]})

        labels, best_delay = generate_2regime_labels(matrix, train_events, all_events)

        # best_global_delay 应从 train set 得出，D5 在 train 中 pnl 最高
        assert best_delay == "D5"

    def test_generate_2regime_labels_no_pullback(self):
        """验证 pullback 不作为 Best_Global_Delay 候选。

        即使 pullback 在 train set 中 pnl 最高，也不会被选为 best_global_delay。
        """
        # 构造 pullback pnl 远高于其他 delay 的场景
        rows = []
        for delay, pnl in [("D0", 0.001), ("D5", 0.002), ("D10", 0.001), ("D15", 0.001), ("pullback", 0.10)]:
            rows.append({"event_id": "event_000", "delay_label": delay, "pnl_pct": pnl,
                         "traded": True, "symbol": "BTCUSDT", "entry_time": "2024-01-15T10:00:00+00:00"})
        for delay, pnl in [("D0", 0.001), ("D5", 0.003), ("D10", 0.001), ("D15", 0.001), ("pullback", 0.08)]:
            rows.append({"event_id": "event_001", "delay_label": delay, "pnl_pct": pnl,
                         "traded": True, "symbol": "ETHUSDT", "entry_time": "2024-02-15T10:00:00+00:00"})

        matrix = pd.DataFrame(rows)
        train_events = pd.DataFrame({"event_id": ["event_000", "event_001"]})
        all_events = pd.DataFrame({"event_id": ["event_000", "event_001"]})

        labels, best_delay = generate_2regime_labels(matrix, train_events, all_events)

        # pullback 不在候选中，best_delay 应为 D5（train 中 D5 的 silo calendar_sum 最高）
        assert best_delay != "pullback"
        assert best_delay in GLOBAL_DELAY_CANDIDATES

    def test_generate_2regime_labels_enter_skip_logic(self):
        """验证 pnl > 0 → enter, else → skip 的标签分配逻辑。"""
        # 构造明确的正/负/零 pnl 场景
        rows = []
        # event_000: D0 pnl = 0.01 (正) → enter
        for delay, pnl in [("D0", 0.01), ("D5", 0.005), ("D10", 0.002), ("D15", 0.001), ("pullback", 0.0)]:
            rows.append({"event_id": "event_000", "delay_label": delay, "pnl_pct": pnl,
                         "traded": True, "symbol": "BTCUSDT", "entry_time": "2024-01-15T10:00:00+00:00"})
        # event_001: D0 pnl = -0.005 (负) → skip
        for delay, pnl in [("D0", -0.005), ("D5", -0.002), ("D10", -0.001), ("D15", -0.003), ("pullback", 0.0)]:
            rows.append({"event_id": "event_001", "delay_label": delay, "pnl_pct": pnl,
                         "traded": True, "symbol": "ETHUSDT", "entry_time": "2024-02-15T10:00:00+00:00"})
        # event_002: D0 pnl = 0.0 (零) → skip (pnl > 0 不满足)
        for delay, pnl in [("D0", 0.0), ("D5", -0.001), ("D10", -0.002), ("D15", -0.001), ("pullback", 0.0)]:
            rows.append({"event_id": "event_002", "delay_label": delay, "pnl_pct": pnl,
                         "traded": True, "symbol": "BTCUSDT", "entry_time": "2024-03-15T10:00:00+00:00"})

        matrix = pd.DataFrame(rows)
        # 所有 events 都是 train（确保 D0 被选为 best_global_delay）
        train_events = pd.DataFrame({"event_id": ["event_000", "event_001", "event_002"]})
        all_events = pd.DataFrame({"event_id": ["event_000", "event_001", "event_002"]})

        labels, best_delay = generate_2regime_labels(matrix, train_events, all_events)

        # D0 在 train 中 calendar_sum 最高（event_000 有 0.01 正 pnl）
        assert best_delay == "D0"

        # 验证标签逻辑
        assert labels.iloc[0] == "enter"   # pnl = 0.01 > 0
        assert labels.iloc[1] == "skip"    # pnl = -0.005 <= 0
        assert labels.iloc[2] == "skip"    # pnl = 0.0, not > 0


# ===========================================================================
# Test: generate_3regime_labels 5bps 边界行为
# ===========================================================================


class TestGenerate3RegimeLabelsToleranceBoundary:
    """测试 generate_3regime_labels 的 5bps 容差边界行为。"""

    def test_generate_3regime_labels_tolerance_boundary(self):
        """精确测试 5bps (0.0005) 边界行为。

        - diff < 5bps → fast
        - diff == 5bps → slow
        - diff > 5bps → slow
        """
        # Case 1: diff = 0.0004 < 0.0005 → fast
        # fast_pnl = 0.01, slow_pnl = 0.0104, diff = 0.0004
        matrix_below = _build_simple_matrix([
            {"D0": 0.01, "D5": 0.008, "D10": 0.0104, "D15": 0.005, "pullback": 0.003},
        ])
        events = _build_events(1)
        result_below = generate_3regime_labels(matrix_below, events)
        assert result_below.iloc[0] == "fast", "diff < 5bps should be fast"

        # Case 2: diff = 0.0005 == tolerance → slow (not strictly less than)
        # fast_pnl = 0.01, slow_pnl = 0.0105, diff = 0.0005
        matrix_exact = _build_simple_matrix([
            {"D0": 0.01, "D5": 0.008, "D10": 0.0105, "D15": 0.005, "pullback": 0.003},
        ])
        result_exact = generate_3regime_labels(matrix_exact, events)
        assert result_exact.iloc[0] == "slow", "diff == 5bps should be slow"

        # Case 3: diff = 0.0006 > 0.0005 → slow
        # fast_pnl = 0.01, slow_pnl = 0.0106, diff = 0.0006
        matrix_above = _build_simple_matrix([
            {"D0": 0.01, "D5": 0.008, "D10": 0.0106, "D15": 0.005, "pullback": 0.003},
        ])
        result_above = generate_3regime_labels(matrix_above, events)
        assert result_above.iloc[0] == "slow", "diff > 5bps should be slow"

        # Case 4: diff = 0.00049 (just below) → fast
        # fast_pnl = 0.01, slow_pnl = 0.01049, diff = 0.00049
        matrix_just_below = _build_simple_matrix([
            {"D0": 0.01, "D5": 0.008, "D10": 0.01049, "D15": 0.005, "pullback": 0.003},
        ])
        result_just_below = generate_3regime_labels(matrix_just_below, events)
        assert result_just_below.iloc[0] == "fast", "diff just below 5bps should be fast"


# ===========================================================================
# Test: compute_label_distributions
# ===========================================================================


class TestComputeLabelDistributions:
    """测试 compute_label_distributions 的输出格式与 imbalance 检测。"""

    def test_compute_label_distributions_output_format(self):
        """验证 DataFrame 的列和结构。"""
        n = 10
        labels_5regime = pd.Series(
            ["D0", "D5", "D10", "D15", "pullback", "skip", "D0", "D5", "D10", "D15"],
            name="regime_5_label",
        )
        labels_3regime = pd.Series(
            ["fast", "fast", "slow", "slow", "slow", "skip", "fast", "fast", "slow", "slow"],
            name="regime_3_label",
        )
        labels_2regime = pd.Series(
            ["enter", "enter", "enter", "skip", "skip", "skip", "enter", "enter", "skip", "skip"],
            name="regime_2_label",
        )
        train_mask = pd.Series([True] * 6 + [False] * 4)

        dist_df, imbalanced = compute_label_distributions(
            labels_5regime, labels_3regime, labels_2regime, train_mask
        )

        # 验证列名
        assert list(dist_df.columns) == ["regime_schema", "label", "split", "count", "pct"]

        # 验证包含所有 3 种 regime schema
        assert set(dist_df["regime_schema"].unique()) == {"5-regime", "3-regime", "2-regime"}

        # 验证包含 train 和 test split
        assert set(dist_df["split"].unique()) == {"train", "test"}

        # 验证 count 为整数
        assert dist_df["count"].dtype in [np.int64, np.int32, int]

        # 验证 pct 列的值在合理范围
        assert (dist_df["pct"] >= 0).all()
        assert (dist_df["pct"] <= 100).all()

        # 验证每个 (regime_schema, split) 组的 count 之和等于该 split 的总数
        for schema in ["5-regime", "3-regime", "2-regime"]:
            train_total = dist_df[
                (dist_df["regime_schema"] == schema) & (dist_df["split"] == "train")
            ]["count"].sum()
            assert train_total == 6  # 6 train events

            test_total = dist_df[
                (dist_df["regime_schema"] == schema) & (dist_df["split"] == "test")
            ]["count"].sum()
            assert test_total == 4  # 4 test events

    def test_compute_label_distributions_imbalance_flag(self):
        """验证 regime2_imbalanced 检测逻辑。

        - enter 占比 < 40% → imbalanced = True
        - enter 占比 > 90% → imbalanced = True
        - enter 占比在 [40%, 90%] → imbalanced = False
        """
        n = 10
        labels_5regime = pd.Series(["D0"] * n)
        labels_3regime = pd.Series(["fast"] * n)

        # Case 1: enter 占比 = 30% (3/10 in train) → imbalanced
        labels_2regime_low = pd.Series(
            ["enter"] * 3 + ["skip"] * 7
        )
        train_mask = pd.Series([True] * 10)  # 全部为 train
        _, imbalanced_low = compute_label_distributions(
            labels_5regime, labels_3regime, labels_2regime_low, train_mask
        )
        assert imbalanced_low is True, "enter < 40% should flag imbalanced"

        # Case 2: enter 占比 = 100% (10/10 in train) → imbalanced
        labels_2regime_high = pd.Series(["enter"] * 10)
        _, imbalanced_high = compute_label_distributions(
            labels_5regime, labels_3regime, labels_2regime_high, train_mask
        )
        assert imbalanced_high is True, "enter > 90% should flag imbalanced"

        # Case 3: enter 占比 = 60% (6/10 in train) → not imbalanced
        labels_2regime_balanced = pd.Series(
            ["enter"] * 6 + ["skip"] * 4
        )
        _, imbalanced_balanced = compute_label_distributions(
            labels_5regime, labels_3regime, labels_2regime_balanced, train_mask
        )
        assert imbalanced_balanced is False, "enter 60% should not flag imbalanced"

        # Case 4: enter 占比 = 40% (4/10) → not imbalanced (boundary)
        labels_2regime_boundary_low = pd.Series(
            ["enter"] * 4 + ["skip"] * 6
        )
        _, imbalanced_boundary_low = compute_label_distributions(
            labels_5regime, labels_3regime, labels_2regime_boundary_low, train_mask
        )
        assert imbalanced_boundary_low is False, "enter == 40% should not flag imbalanced"

        # Case 5: enter 占比 = 90% (9/10) → not imbalanced (boundary)
        labels_2regime_boundary_high = pd.Series(
            ["enter"] * 9 + ["skip"] * 1
        )
        _, imbalanced_boundary_high = compute_label_distributions(
            labels_5regime, labels_3regime, labels_2regime_boundary_high, train_mask
        )
        assert imbalanced_boundary_high is False, "enter == 90% should not flag imbalanced"
