"""
test_arm_runner — arm_runner 模块的单元测试

覆盖：
- rebuild_delay_results_from_matrix 结构正确性
- 按 event 分组、按 delay 排序
- 验证 DelayResult 数量与 matrix 一致
- 处理 NaN/None 字段
"""

from __future__ import annotations

import sys
from pathlib import Path

import numpy as np
import pandas as pd
import pytest

# Path setup
_SCRIPTS_DIR = Path(__file__).resolve().parent.parent.parent
if str(_SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS_DIR))

from pretouch_refinement.arm_runner import (
    rebuild_delay_results_from_matrix,
    predict_and_evaluate_test,
    _find_best_in_group,
    _compute_calendar_sum_silo,
)
from pre_breakout_timing.delay_simulator import DelayResult


# ---------------------------------------------------------------------------
# 辅助函数
# ---------------------------------------------------------------------------


def _build_matrix(n_events: int = 4) -> tuple[pd.DataFrame, pd.DataFrame]:
    """构建合成 delay_pnl_matrix 和 events DataFrame。

    Parameters
    ----------
    n_events : int
        事件数量，默认 4。

    Returns
    -------
    tuple[pd.DataFrame, pd.DataFrame]
        (matrix, events)
    """
    delay_labels = ["D0", "D5", "D10", "D15", "pullback"]
    rows = []
    event_rows = []

    for i in range(n_events):
        event_id = f"evt_{i:03d}"
        symbol = "BTCUSDT" if i % 2 == 0 else "ETHUSDT"
        side = "long" if i % 2 == 0 else "short"
        touch_time = f"2024-01-{10 + i:02d} 08:00:00+00:00"

        event_rows.append({
            "event_id": event_id,
            "symbol": symbol,
            "side": side,
            "touch_time": touch_time,
        })

        for j, dl in enumerate(delay_labels):
            delay_seconds = [0, 5, 10, 15, 3][j]
            traded = True if j < 3 else False  # D0, D5, D10 traded; D15, pullback not

            row = {
                "event_id": event_id,
                "symbol": symbol,
                "side": side,
                "touch_time": touch_time,
                "delay_label": dl,
                "delay_seconds": delay_seconds,
                "entry_time": f"2024-01-{10 + i:02d} 08:00:{delay_seconds:02d}+00:00" if traded else np.nan,
                "entry_price": 40000.0 + i * 100 + j * 10 if traded else np.nan,
                "pnl_pct": 0.01 * (j + 1) - 0.02 if traded else np.nan,
                "exit_reason": "TrailingSL" if traded else np.nan,
                "exit_time": f"2024-01-{10 + i:02d} 09:00:00+00:00" if traded else np.nan,
                "hold_seconds": 3600.0 if traded else np.nan,
                "mfe_r": 1.5 if traded else np.nan,
                "mae_r": -0.3 if traded else np.nan,
                "traded": traded,
            }
            rows.append(row)

    matrix = pd.DataFrame(rows)
    events = pd.DataFrame(event_rows)
    return matrix, events


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------


class TestRebuildDelayResultsFromMatrix:
    """测试 rebuild_delay_results_from_matrix 的结构正确性。"""

    def test_basic_structure(self):
        """验证返回结构：外层 list 长度 = events 数，内层 list 长度 = 5。"""
        matrix, events = _build_matrix(n_events=4)
        result = rebuild_delay_results_from_matrix(matrix, events)

        assert len(result) == 4
        for inner in result:
            assert len(inner) == 5

    def test_total_count_matches_matrix(self):
        """验证重建后的 DelayResult 总数与 matrix 行数一致。"""
        matrix, events = _build_matrix(n_events=4)
        result = rebuild_delay_results_from_matrix(matrix, events)

        total = sum(len(inner) for inner in result)
        assert total == len(matrix)

    def test_delay_order(self):
        """验证内层 list 的 delay 顺序为 D0, D5, D10, D15, pullback。"""
        matrix, events = _build_matrix(n_events=2)
        result = rebuild_delay_results_from_matrix(matrix, events)

        expected_order = ["D0", "D5", "D10", "D15", "pullback"]
        for inner in result:
            labels = [dr.delay_label for dr in inner]
            assert labels == expected_order

    def test_event_order_matches_events_df(self):
        """验证外层 list 的顺序与 events DataFrame 行顺序一致。"""
        matrix, events = _build_matrix(n_events=4)
        # 反转 events 顺序
        events_reversed = events.iloc[::-1].reset_index(drop=True)
        result = rebuild_delay_results_from_matrix(matrix, events_reversed)

        # 第一个 event 应该是 evt_003（反转后）
        assert result[0][0].event_id == "evt_003"
        assert result[-1][0].event_id == "evt_000"

    def test_delay_result_fields(self):
        """验证 DelayResult 字段正确解析。"""
        matrix, events = _build_matrix(n_events=1)
        result = rebuild_delay_results_from_matrix(matrix, events)

        # D0 (traded=True)
        dr_d0 = result[0][0]
        assert dr_d0.event_id == "evt_000"
        assert dr_d0.delay_label == "D0"
        assert dr_d0.delay_seconds == 0
        assert dr_d0.traded is True
        assert dr_d0.entry_price is not None
        assert dr_d0.pnl_pct is not None
        assert dr_d0.exit_reason == "TrailingSL"
        assert dr_d0.entry_time is not None
        assert dr_d0.exit_time is not None
        assert dr_d0.hold_seconds == 3600.0
        assert dr_d0.mfe_r == 1.5
        assert dr_d0.mae_r == -0.3

    def test_nan_fields_become_none(self):
        """验证 NaN 字段正确转换为 None。"""
        matrix, events = _build_matrix(n_events=1)
        result = rebuild_delay_results_from_matrix(matrix, events)

        # D15 (traded=False)
        dr_d15 = result[0][3]
        assert dr_d15.delay_label == "D15"
        assert dr_d15.traded is False
        assert dr_d15.entry_time is None
        assert dr_d15.entry_price is None
        assert dr_d15.pnl_pct is None
        assert dr_d15.exit_reason is None
        assert dr_d15.exit_time is None
        assert dr_d15.hold_seconds is None
        assert dr_d15.mfe_r is None
        assert dr_d15.mae_r is None

    def test_delayresult_type(self):
        """验证返回的对象类型为 DelayResult。"""
        matrix, events = _build_matrix(n_events=2)
        result = rebuild_delay_results_from_matrix(matrix, events)

        for inner in result:
            for dr in inner:
                assert isinstance(dr, DelayResult)

    def test_missing_event_raises_error(self):
        """验证 events 中有 matrix 中不存在的 event_id 时抛出 ValueError。"""
        matrix, events = _build_matrix(n_events=2)
        # 添加一个 matrix 中不存在的 event
        extra_event = pd.DataFrame([{
            "event_id": "evt_999",
            "symbol": "BTCUSDT",
            "side": "long",
            "touch_time": "2024-01-20 08:00:00+00:00",
        }])
        events_with_extra = pd.concat([events, extra_event], ignore_index=True)

        with pytest.raises(ValueError, match="evt_999"):
            rebuild_delay_results_from_matrix(matrix, events_with_extra)

    def test_count_mismatch_raises_error(self):
        """验证 matrix 行数与重建结果不一致时抛出 ValueError。"""
        matrix, events = _build_matrix(n_events=2)
        # 删除一行使得 matrix 不完整
        matrix_incomplete = matrix.iloc[:-1].copy()

        # 这应该在找不到某个 delay_label 时报错
        with pytest.raises(ValueError):
            rebuild_delay_results_from_matrix(matrix_incomplete, events)

    def test_timestamp_parsing_with_timezone(self):
        """验证时间戳正确解析并带有 UTC 时区。"""
        matrix, events = _build_matrix(n_events=1)
        result = rebuild_delay_results_from_matrix(matrix, events)

        dr_d0 = result[0][0]
        assert dr_d0.entry_time is not None
        assert dr_d0.entry_time.tzinfo is not None
        assert str(dr_d0.entry_time.tzinfo) == "UTC"


# ---------------------------------------------------------------------------
# Mock classifier for predict_and_evaluate_test tests
# ---------------------------------------------------------------------------


class MockClassifier:
    """简单 mock 分类器，返回预设标签序列。"""

    def __init__(self, predictions: list[str]):
        self._predictions = predictions

    def predict(self, X: pd.DataFrame) -> np.ndarray:
        return np.array(self._predictions[: len(X)])


# ---------------------------------------------------------------------------
# 辅助函数：构建 DelayResult 列表用于 predict_and_evaluate_test 测试
# ---------------------------------------------------------------------------


def _make_delay_result(
    event_id: str,
    delay_label: str,
    pnl_pct: float | None,
    traded: bool = True,
    entry_time: str = "2024-01-10 08:00:00+00:00",
) -> DelayResult:
    """快速构建 DelayResult 用于测试。"""
    return DelayResult(
        event_id=event_id,
        delay_label=delay_label,
        delay_seconds={"D0": 0, "D5": 5, "D10": 10, "D15": 15, "pullback": 3}.get(
            delay_label, 0
        ),
        entry_time=pd.Timestamp(entry_time) if traded else None,
        entry_price=40000.0 if traded else None,
        pnl_pct=pnl_pct if traded else None,
        exit_reason="TrailingSL" if traded else None,
        exit_time=pd.Timestamp("2024-01-10 09:00:00+00:00") if traded else None,
        hold_seconds=3600.0 if traded else None,
        mfe_r=1.0 if traded else None,
        mae_r=-0.5 if traded else None,
        traded=traded,
    )


def _build_test_delay_results(n_events: int = 3) -> list[list[DelayResult]]:
    """构建 n_events 个 event 的 delay_results，每个 event 有 5 个 delay。

    pnl 设计：
    - D0: 0.01 * (i+1)
    - D5: 0.02 * (i+1)
    - D10: 0.03 * (i+1)
    - D15: 0.015 * (i+1)
    - pullback: 0.025 * (i+1)

    所以 fast 组 (D0, D5) 最优 = D5
       slow 组 (D10, D15, pullback) 最优 = D10
    """
    all_results = []
    for i in range(n_events):
        symbol = "BTCUSDT" if i % 2 == 0 else "ETHUSDT"
        event_id = f"{symbol}_evt_{i:03d}"
        entry_time = f"2024-01-{10 + i:02d} 08:00:00+00:00"
        event_delays = [
            _make_delay_result(event_id, "D0", 0.01 * (i + 1), entry_time=entry_time),
            _make_delay_result(event_id, "D5", 0.02 * (i + 1), entry_time=entry_time),
            _make_delay_result(event_id, "D10", 0.03 * (i + 1), entry_time=entry_time),
            _make_delay_result(event_id, "D15", 0.015 * (i + 1), entry_time=entry_time),
            _make_delay_result(event_id, "pullback", 0.025 * (i + 1), entry_time=entry_time),
        ]
        all_results.append(event_delays)
    return all_results


def _build_test_events(n_events: int = 3) -> pd.DataFrame:
    """构建与 _build_test_delay_results 对应的 events DataFrame。"""
    rows = []
    for i in range(n_events):
        symbol = "BTCUSDT" if i % 2 == 0 else "ETHUSDT"
        rows.append({
            "event_id": f"{symbol}_evt_{i:03d}",
            "symbol": symbol,
            "side": "long",
            "touch_time": f"2024-01-{10 + i:02d} 08:00:00+00:00",
        })
    return pd.DataFrame(rows)


def _build_test_features(n_events: int = 3) -> pd.DataFrame:
    """构建简单的测试特征矩阵。"""
    return pd.DataFrame({
        "feat_a": np.random.randn(n_events),
        "feat_b": np.random.randn(n_events),
    })


# ---------------------------------------------------------------------------
# Tests: predict_and_evaluate_test
# ---------------------------------------------------------------------------


class TestPredictAndEvaluateTest5Regime:
    """测试 5-regime 预测映射到正确的 delay labels。"""

    def test_predict_and_evaluate_test_5regime(self):
        """验证 5-regime 预测直接映射到对应 delay_label 的 DelayResult。"""
        n_events = 3
        delay_results = _build_test_delay_results(n_events)
        events = _build_test_events(n_events)
        features = _build_test_features(n_events)

        # 预测 D0, D5, D10 分别对应 3 个 event
        model = MockClassifier(["D0", "D5", "D10"])

        cs, preds = predict_and_evaluate_test(
            model=model,
            test_features=features,
            delay_results_test=delay_results,
            test_events=events,
            regime_schema="5-regime",
        )

        # 验证预测标签正确
        assert list(preds) == ["D0", "D5", "D10"]
        # calendar_sum 应该 > 0（所有 pnl 都为正）
        assert cs > 0.0


class TestPredictAndEvaluateTest2Regime:
    """测试 2-regime 预测逻辑：enter 使用 best_global_delay，skip → pnl=0。"""

    def test_predict_and_evaluate_test_2regime(self):
        """验证 enter 使用 best_global_delay 对应的 DelayResult。"""
        n_events = 3
        delay_results = _build_test_delay_results(n_events)
        events = _build_test_events(n_events)
        features = _build_test_features(n_events)

        # 全部预测为 enter，使用 D5 作为 best_global_delay
        model = MockClassifier(["enter", "enter", "enter"])

        cs, preds = predict_and_evaluate_test(
            model=model,
            test_features=features,
            delay_results_test=delay_results,
            test_events=events,
            regime_schema="2-regime",
            best_global_delay="D5",
        )

        assert list(preds) == ["enter", "enter", "enter"]
        # 所有 event 使用 D5 的 pnl（均为正），calendar_sum > 0
        assert cs > 0.0

    def test_predict_and_evaluate_test_2regime_skip_contributes_zero(self):
        """验证 skip 预测不贡献 pnl（等效 pnl=0）。"""
        n_events = 3
        delay_results = _build_test_delay_results(n_events)
        events = _build_test_events(n_events)
        features = _build_test_features(n_events)

        # 全部预测为 skip
        model = MockClassifier(["skip", "skip", "skip"])

        cs, preds = predict_and_evaluate_test(
            model=model,
            test_features=features,
            delay_results_test=delay_results,
            test_events=events,
            regime_schema="2-regime",
            best_global_delay="D5",
        )

        assert list(preds) == ["skip", "skip", "skip"]
        # 全部 skip → calendar_sum = 0
        assert cs == 0.0

    def test_predict_and_evaluate_test_2regime_requires_best_global_delay(self):
        """验证 2-regime 下 enter 预测但 best_global_delay=None 时抛出 ValueError。"""
        n_events = 2
        delay_results = _build_test_delay_results(n_events)
        events = _build_test_events(n_events)
        features = _build_test_features(n_events)

        model = MockClassifier(["enter", "enter"])

        with pytest.raises(ValueError, match="best_global_delay"):
            predict_and_evaluate_test(
                model=model,
                test_features=features,
                delay_results_test=delay_results,
                test_events=events,
                regime_schema="2-regime",
                best_global_delay=None,
            )


class TestPredictAndEvaluateTest3Regime:
    """测试 3-regime 预测逻辑：fast/slow 使用对应组内最优 delay。"""

    def test_predict_and_evaluate_test_3regime(self):
        """验证 fast 使用 {D0,D5} 中最优，slow 使用 {D10,D15,pullback} 中最优。"""
        n_events = 3
        delay_results = _build_test_delay_results(n_events)
        events = _build_test_events(n_events)
        features = _build_test_features(n_events)

        # 预测 fast, slow, fast
        model = MockClassifier(["fast", "slow", "fast"])

        cs, preds = predict_and_evaluate_test(
            model=model,
            test_features=features,
            delay_results_test=delay_results,
            test_events=events,
            regime_schema="3-regime",
        )

        assert list(preds) == ["fast", "slow", "fast"]
        # fast 组最优 = D5 (pnl=0.02*(i+1))
        # slow 组最优 = D10 (pnl=0.03*(i+1))
        # 所有 pnl 为正 → calendar_sum > 0
        assert cs > 0.0

    def test_predict_and_evaluate_test_3regime_skip_zero(self):
        """验证 3-regime 下 skip 预测贡献 pnl=0。"""
        n_events = 2
        delay_results = _build_test_delay_results(n_events)
        events = _build_test_events(n_events)
        features = _build_test_features(n_events)

        # 全部 skip
        model = MockClassifier(["skip", "skip"])

        cs, preds = predict_and_evaluate_test(
            model=model,
            test_features=features,
            delay_results_test=delay_results,
            test_events=events,
            regime_schema="3-regime",
        )

        assert cs == 0.0


class TestPredictAndEvaluateTestSkipPnlZero:
    """验证 skip 预测在所有 regime schema 下贡献 0 到 calendar_sum。"""

    def test_predict_and_evaluate_test_skip_pnl_zero(self):
        """混合 skip 与非 skip 预测，验证 skip 不影响 calendar_sum。"""
        n_events = 4
        delay_results = _build_test_delay_results(n_events)
        events = _build_test_events(n_events)
        features = _build_test_features(n_events)

        # 只有 event 0 和 2 enter，event 1 和 3 skip
        model_with_skip = MockClassifier(["enter", "skip", "enter", "skip"])
        model_enter_only = MockClassifier(["enter", "enter"])

        cs_with_skip, _ = predict_and_evaluate_test(
            model=model_with_skip,
            test_features=features,
            delay_results_test=delay_results,
            test_events=events,
            regime_schema="2-regime",
            best_global_delay="D5",
        )

        # 构建只有 event 0 和 2 的子集
        delay_results_subset = [delay_results[0], delay_results[2]]
        events_subset = events.iloc[[0, 2]].reset_index(drop=True)
        features_subset = features.iloc[[0, 2]].reset_index(drop=True)

        cs_enter_only, _ = predict_and_evaluate_test(
            model=model_enter_only,
            test_features=features_subset,
            delay_results_test=delay_results_subset,
            test_events=events_subset,
            regime_schema="2-regime",
            best_global_delay="D5",
        )

        # skip 不贡献 pnl，两者应该相等
        assert abs(cs_with_skip - cs_enter_only) < 1e-10


# ---------------------------------------------------------------------------
# Tests: _find_best_in_group
# ---------------------------------------------------------------------------


class TestFindBestInGroup:
    """测试 _find_best_in_group 选择组内 pnl 最高的 DelayResult。"""

    def test_find_best_in_group(self):
        """验证从 fast 组 {D0, D5} 中选出 pnl 最高的 DelayResult。"""
        event_delays = [
            _make_delay_result("evt_001", "D0", pnl_pct=0.01),
            _make_delay_result("evt_001", "D5", pnl_pct=0.05),
            _make_delay_result("evt_001", "D10", pnl_pct=0.10),
            _make_delay_result("evt_001", "D15", pnl_pct=0.02),
            _make_delay_result("evt_001", "pullback", pnl_pct=0.08),
        ]

        # fast 组: D0=0.01, D5=0.05 → 选 D5
        best_fast = _find_best_in_group(event_delays, ["D0", "D5"])
        assert best_fast is not None
        assert best_fast.delay_label == "D5"
        assert best_fast.pnl_pct == 0.05

        # slow 组: D10=0.10, D15=0.02, pullback=0.08 → 选 D10
        best_slow = _find_best_in_group(event_delays, ["D10", "D15", "pullback"])
        assert best_slow is not None
        assert best_slow.delay_label == "D10"
        assert best_slow.pnl_pct == 0.10

    def test_find_best_in_group_all_untraded(self):
        """验证组内所有 delay 均未交易时返回 None。"""
        event_delays = [
            _make_delay_result("evt_001", "D0", pnl_pct=None, traded=False),
            _make_delay_result("evt_001", "D5", pnl_pct=None, traded=False),
            _make_delay_result("evt_001", "D10", pnl_pct=0.03),
            _make_delay_result("evt_001", "D15", pnl_pct=0.01),
            _make_delay_result("evt_001", "pullback", pnl_pct=0.02),
        ]

        # fast 组全部 untraded → None
        best = _find_best_in_group(event_delays, ["D0", "D5"])
        assert best is None

    def test_find_best_in_group_negative_pnl(self):
        """验证组内有负 pnl 时仍选最高（最不负）的。"""
        event_delays = [
            _make_delay_result("evt_001", "D0", pnl_pct=-0.05),
            _make_delay_result("evt_001", "D5", pnl_pct=-0.01),
            _make_delay_result("evt_001", "D10", pnl_pct=-0.10),
            _make_delay_result("evt_001", "D15", pnl_pct=-0.02),
            _make_delay_result("evt_001", "pullback", pnl_pct=-0.03),
        ]

        # fast 组: D0=-0.05, D5=-0.01 → 选 D5（最不负）
        best = _find_best_in_group(event_delays, ["D0", "D5"])
        assert best is not None
        assert best.delay_label == "D5"
        assert best.pnl_pct == -0.01


# ---------------------------------------------------------------------------
# Tests: _compute_calendar_sum_silo
# ---------------------------------------------------------------------------


class TestComputeCalendarSumSilo:
    """测试 silo-based calendar sum 计算。"""

    def test_compute_calendar_sum_silo(self):
        """验证 silo-based 计算：每个 (symbol, month) 独立从 100k 开始。"""
        # 构建 2 个 BTC event（同月）和 1 个 ETH event
        results = [
            _make_delay_result(
                "BTCUSDT_evt_000", "D5", pnl_pct=0.01,
                entry_time="2024-01-10 08:00:00+00:00",
            ),
            _make_delay_result(
                "BTCUSDT_evt_001", "D5", pnl_pct=0.02,
                entry_time="2024-01-12 08:00:00+00:00",
            ),
            _make_delay_result(
                "ETHUSDT_evt_002", "D10", pnl_pct=0.03,
                entry_time="2024-01-15 08:00:00+00:00",
            ),
        ]

        events = pd.DataFrame([
            {"event_id": "BTCUSDT_evt_000", "symbol": "BTCUSDT", "side": "long",
             "touch_time": "2024-01-10 08:00:00+00:00"},
            {"event_id": "BTCUSDT_evt_001", "symbol": "BTCUSDT", "side": "long",
             "touch_time": "2024-01-12 08:00:00+00:00"},
            {"event_id": "ETHUSDT_evt_002", "symbol": "ETHUSDT", "side": "long",
             "touch_time": "2024-01-15 08:00:00+00:00"},
        ])

        cs = _compute_calendar_sum_silo(results, events)

        # BTC silo (2024-01):
        #   balance = 100_000
        #   trade 1: notional = 100_000 * 0.26 = 26_000; pnl = 26_000 * 0.01 = 260
        #   balance = 100_260
        #   trade 2: notional = 100_260 * 0.26 = 26_067.6; pnl = 26_067.6 * 0.02 = 521.352
        #   balance = 100_781.352
        #   silo_return = (100_781.352 - 100_000) / 100_000 * 100 = 0.781352%
        #
        # ETH silo (2024-01):
        #   balance = 100_000
        #   trade 1: notional = 100_000 * 0.26 = 26_000; pnl = 26_000 * 0.03 = 780
        #   balance = 100_780
        #   silo_return = (100_780 - 100_000) / 100_000 * 100 = 0.78%
        #
        # total = 0.781352 + 0.78 = 1.561352%

        expected_btc = (100_781.352 - 100_000) / 100_000 * 100.0
        expected_eth = (100_780.0 - 100_000) / 100_000 * 100.0
        expected_total = expected_btc + expected_eth

        assert abs(cs - expected_total) < 1e-6

    def test_compute_calendar_sum_silo_empty(self):
        """验证空 results 返回 0。"""
        events = pd.DataFrame(columns=["event_id", "symbol", "side", "touch_time"])
        cs = _compute_calendar_sum_silo([], events)
        assert cs == 0.0

    def test_compute_calendar_sum_silo_untraded_excluded(self):
        """验证 traded=False 的 results 不参与计算。"""
        results = [
            _make_delay_result(
                "BTCUSDT_evt_000", "D5", pnl_pct=None, traded=False,
                entry_time="2024-01-10 08:00:00+00:00",
            ),
        ]
        events = pd.DataFrame([
            {"event_id": "BTCUSDT_evt_000", "symbol": "BTCUSDT", "side": "long",
             "touch_time": "2024-01-10 08:00:00+00:00"},
        ])

        cs = _compute_calendar_sum_silo(results, events)
        assert cs == 0.0
