"""Property-based tests for unified_runner — Pipeline Determinism (Property 14).

# Feature: timing-probability-unified, Property 14: Pipeline Determinism
# **Validates: Requirements 6.4, 9.2**

Tests that for any complete pipeline input (events_pool + bars_cache + random_state=42),
two independent executions produce byte-identical output. Since running the full pipeline
twice would be very slow, we test determinism at the component level:
1. compute_combined_positions with same inputs produces identical output
2. generate_3regime_labels with same inputs produces identical output
3. compute_sizing_multiplier with same inputs produces identical output
4. A mini-pipeline (subset of steps) run twice with same random_state produces identical results.
"""

from __future__ import annotations

import numpy as np
import pandas as pd
import pytest
from hypothesis import given, settings, assume
from hypothesis import strategies as st

from pre_breakout_timing.delay_simulator import DelayResult
from timing_probability_unified.combined_executor import (
    CombinedPositionConfig,
    compute_combined_positions,
    compute_calendar_sum,
    compute_worst_sm,
)
from timing_probability_unified.timing_classifier import (
    generate_3regime_labels,
    generate_3regime_label_from_pnls,
    get_selected_delay_pnl,
)
from timing_probability_unified.probability_model import (
    compute_sizing_multiplier,
    train_rf_probability,
    generate_rf_binary_labels,
)


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _make_delay_result(
    event_id: str, delay_label: str, pnl: float, traded: bool = True
) -> DelayResult:
    """Helper to create a minimal DelayResult for testing."""
    return DelayResult(
        event_id=event_id,
        delay_label=delay_label,
        delay_seconds=0,
        entry_time=None,
        entry_price=None,
        pnl_pct=pnl if traded else None,
        exit_reason="test",
        exit_time=None,
        hold_seconds=None,
        mfe_r=None,
        mae_r=None,
        traded=traded,
    )


def _make_event_delay_results(
    event_id: str,
    d0_pnl: float,
    d5_pnl: float,
    d10_pnl: float,
    d15_pnl: float,
    pb_pnl: float,
) -> list[DelayResult]:
    """Create a full set of 5 delay results for one event."""
    return [
        _make_delay_result(event_id, "D0", d0_pnl),
        _make_delay_result(event_id, "D5", d5_pnl),
        _make_delay_result(event_id, "D10", d10_pnl),
        _make_delay_result(event_id, "D15", d15_pnl),
        _make_delay_result(event_id, "pullback", pb_pnl),
    ]


# ---------------------------------------------------------------------------
# Hypothesis Strategies
# ---------------------------------------------------------------------------

# Strategy for generating a list of PnL values for 5 delays
_pnl_strategy = st.floats(min_value=-0.05, max_value=0.05, allow_nan=False, allow_infinity=False)

# Strategy for generating a small synthetic events pool (2-10 events)
@st.composite
def _synthetic_pipeline_inputs(draw):
    """Generate a complete synthetic pipeline input for determinism testing.

    Produces:
    - events DataFrame (2-10 events)
    - delay_results (5 delays per event)
    - sizing multipliers
    - speed_gate_pass flags
    """
    n_events = draw(st.integers(min_value=2, max_value=10))

    # Generate event metadata
    symbols = draw(
        st.lists(
            st.sampled_from(["BTCUSDT", "ETHUSDT"]),
            min_size=n_events,
            max_size=n_events,
        )
    )
    sides = draw(
        st.lists(
            st.sampled_from(["long", "short"]),
            min_size=n_events,
            max_size=n_events,
        )
    )
    # Generate distinct months to ensure interesting calendar_sum behavior
    months = draw(
        st.lists(
            st.integers(min_value=1, max_value=6),
            min_size=n_events,
            max_size=n_events,
        )
    )
    days = draw(
        st.lists(
            st.integers(min_value=1, max_value=28),
            min_size=n_events,
            max_size=n_events,
        )
    )
    speed_values = draw(
        st.lists(
            st.floats(min_value=0.0, max_value=5.0, allow_nan=False, allow_infinity=False),
            min_size=n_events,
            max_size=n_events,
        )
    )

    touch_times = [
        pd.Timestamp(year=2025, month=m, day=d, hour=12, tz="UTC")
        for m, d in zip(months, days)
    ]

    events = pd.DataFrame(
        {
            "event_id": [f"evt_{i:03d}" for i in range(n_events)],
            "symbol": symbols,
            "side": sides,
            "touch_time": touch_times,
            "speed_300s_atr": speed_values,
        }
    )

    # Generate delay results for each event
    delay_results = []
    for i in range(n_events):
        d0 = draw(_pnl_strategy)
        d5 = draw(_pnl_strategy)
        d10 = draw(_pnl_strategy)
        d15 = draw(_pnl_strategy)
        pb = draw(_pnl_strategy)
        delay_results.append(
            _make_event_delay_results(f"evt_{i:03d}", d0, d5, d10, d15, pb)
        )

    # Generate sizing multipliers (0..2)
    multipliers = np.array(
        draw(
            st.lists(
                st.floats(min_value=0.0, max_value=2.0, allow_nan=False, allow_infinity=False),
                min_size=n_events,
                max_size=n_events,
            )
        )
    )

    # Generate speed gate pass flags
    speed_gate_pass = np.array(
        draw(
            st.lists(
                st.booleans(),
                min_size=n_events,
                max_size=n_events,
            )
        )
    )

    return events, delay_results, multipliers, speed_gate_pass


# ---------------------------------------------------------------------------
# Property 14: Pipeline Determinism — Component-Level Tests
# ---------------------------------------------------------------------------


# Feature: timing-probability-unified, Property 14: Pipeline Determinism
@settings(max_examples=50)
@given(data=_synthetic_pipeline_inputs())
def test_generate_3regime_labels_determinism(data):
    """Property 14: Pipeline Determinism — generate_3regime_labels

    Two calls to generate_3regime_labels with identical delay_results
    SHALL produce identical label sequences.

    **Validates: Requirements 6.4, 9.2**
    """
    _events, delay_results, _multipliers, _speed_gate_pass = data

    # Run twice with identical inputs
    labels_run1 = generate_3regime_labels(delay_results)
    labels_run2 = generate_3regime_labels(delay_results)

    # Verify byte-identical output
    assert labels_run1.tolist() == labels_run2.tolist(), (
        f"generate_3regime_labels produced different results on two runs:\n"
        f"  Run 1: {labels_run1.tolist()}\n"
        f"  Run 2: {labels_run2.tolist()}"
    )


# Feature: timing-probability-unified, Property 14: Pipeline Determinism
@settings(max_examples=50)
@given(
    probabilities=st.lists(
        st.floats(min_value=0.0, max_value=1.0, allow_nan=False, allow_infinity=False),
        min_size=1,
        max_size=20,
    )
)
def test_compute_sizing_multiplier_determinism(probabilities):
    """Property 14: Pipeline Determinism — compute_sizing_multiplier

    Two calls to compute_sizing_multiplier with identical probability arrays
    SHALL produce identical multiplier arrays.

    **Validates: Requirements 6.4, 9.2**
    """
    probs = np.array(probabilities)

    # Run twice with identical inputs
    multipliers_run1 = compute_sizing_multiplier(probs.copy())
    multipliers_run2 = compute_sizing_multiplier(probs.copy())

    # Verify byte-identical output
    np.testing.assert_array_equal(
        multipliers_run1,
        multipliers_run2,
        err_msg="compute_sizing_multiplier produced different results on two runs",
    )


# Feature: timing-probability-unified, Property 14: Pipeline Determinism
@settings(max_examples=50)
@given(data=_synthetic_pipeline_inputs())
def test_compute_combined_positions_determinism(data):
    """Property 14: Pipeline Determinism — compute_combined_positions

    Two calls to compute_combined_positions with identical inputs
    SHALL produce byte-identical unified_trades DataFrames.

    **Validates: Requirements 6.4, 9.2**
    """
    events, delay_results, multipliers, speed_gate_pass = data

    # Generate timing predictions from delay_results (deterministic)
    labels = generate_3regime_labels(delay_results)
    timing_predictions = labels.values

    config = CombinedPositionConfig(base_notional_share=0.30)

    # Run 1
    trades_run1 = compute_combined_positions(
        events=events.copy(),
        timing_predictions=timing_predictions.copy(),
        sizing_multipliers=multipliers.copy(),
        delay_results=delay_results,
        speed_gate_pass=speed_gate_pass.copy(),
        config=config,
    )

    # Run 2
    trades_run2 = compute_combined_positions(
        events=events.copy(),
        timing_predictions=timing_predictions.copy(),
        sizing_multipliers=multipliers.copy(),
        delay_results=delay_results,
        speed_gate_pass=speed_gate_pass.copy(),
        config=config,
    )

    # Verify byte-identical output (compare all columns)
    pd.testing.assert_frame_equal(
        trades_run1,
        trades_run2,
        check_exact=True,
        obj="compute_combined_positions determinism check",
    )


# Feature: timing-probability-unified, Property 14: Pipeline Determinism
@settings(max_examples=50)
@given(data=_synthetic_pipeline_inputs())
def test_mini_pipeline_determinism(data):
    """Property 14: Pipeline Determinism — Mini-Pipeline

    A mini-pipeline (labels → multipliers → combined positions → calendar_sum)
    run twice with the same random_state=42 SHALL produce identical results.

    This tests the full chain of deterministic functions together.

    **Validates: Requirements 6.4, 9.2**
    """
    events, delay_results, _multipliers, speed_gate_pass = data

    def _run_mini_pipeline():
        """Execute a mini-pipeline and return key outputs."""
        # Step 1: Generate 3-regime labels
        labels = generate_3regime_labels(delay_results)
        timing_predictions = labels.values

        # Step 2: Compute sizing multipliers from fixed probabilities
        # Use a deterministic probability based on delay results
        probs = np.array([
            0.5 + 0.1 * i / len(delay_results)
            for i in range(len(delay_results))
        ])
        multipliers = compute_sizing_multiplier(probs)

        # Step 3: Compute combined positions
        config = CombinedPositionConfig(base_notional_share=0.30)
        trades = compute_combined_positions(
            events=events.copy(),
            timing_predictions=timing_predictions.copy(),
            sizing_multipliers=multipliers.copy(),
            delay_results=delay_results,
            speed_gate_pass=speed_gate_pass.copy(),
            config=config,
        )

        # Step 4: Compute metrics
        calendar_sum = compute_calendar_sum(trades, gate_filter=False)
        worst_sm = compute_worst_sm(trades, gate_filter=False)

        return trades, calendar_sum, worst_sm

    # Run the mini-pipeline twice
    trades_1, cs_1, wsm_1 = _run_mini_pipeline()
    trades_2, cs_2, wsm_2 = _run_mini_pipeline()

    # Verify identical trades DataFrames
    pd.testing.assert_frame_equal(
        trades_1,
        trades_2,
        check_exact=True,
        obj="mini-pipeline trades determinism",
    )

    # Verify identical scalar metrics
    assert cs_1 == cs_2, (
        f"calendar_sum differs between runs: {cs_1} vs {cs_2}"
    )
    assert wsm_1 == wsm_2, (
        f"worst_sm differs between runs: {wsm_1} vs {wsm_2}"
    )


# Feature: timing-probability-unified, Property 14: Pipeline Determinism
@settings(max_examples=50)
@given(data=_synthetic_pipeline_inputs())
def test_rf_probability_model_determinism(data):
    """Property 14: Pipeline Determinism — RF Probability Model

    Training an RF model twice with the same features, labels, and random_state=42
    SHALL produce identical probability outputs.

    **Validates: Requirements 6.4, 9.2**
    """
    events, delay_results, _multipliers, _speed_gate_pass = data

    # Generate labels and features for RF training
    labels = generate_3regime_labels(delay_results)
    timing_predictions = labels

    # Create synthetic features (deterministic from event index)
    n = len(events)
    np.random.seed(42)
    features = pd.DataFrame(
        np.random.randn(n, 5),
        columns=[f"feat_{i}" for i in range(5)],
    )

    # Generate delay PnLs for RF binary labels
    delay_pnls = pd.Series([
        get_selected_delay_pnl(timing_predictions.iloc[i], delay_results[i])[1]
        for i in range(n)
    ])

    rf_labels = generate_rf_binary_labels(timing_predictions, delay_pnls)

    # Need at least 2 samples for train/test split
    assume(n >= 4)

    # Split into train/test (deterministic split)
    split_idx = n // 2
    train_features = features.iloc[:split_idx].reset_index(drop=True)
    test_features = features.iloc[split_idx:].reset_index(drop=True)
    train_labels = rf_labels.iloc[:split_idx].reset_index(drop=True)
    test_labels = rf_labels.iloc[split_idx:].reset_index(drop=True)

    # Run 1
    result_1 = train_rf_probability(
        train_features=train_features.copy(),
        train_labels=train_labels.copy(),
        test_features=test_features.copy(),
        test_labels=test_labels.copy(),
        n_estimators=10,  # fewer trees for speed in testing
        random_state=42,
    )

    # Run 2
    result_2 = train_rf_probability(
        train_features=train_features.copy(),
        train_labels=train_labels.copy(),
        test_features=test_features.copy(),
        test_labels=test_labels.copy(),
        n_estimators=10,
        random_state=42,
    )

    # Verify identical probability outputs
    np.testing.assert_array_equal(
        result_1.train_probabilities,
        result_2.train_probabilities,
        err_msg="RF train_probabilities differ between runs",
    )
    np.testing.assert_array_equal(
        result_1.test_probabilities,
        result_2.test_probabilities,
        err_msg="RF test_probabilities differ between runs",
    )

    # Verify identical AUC scores
    assert result_1.train_auc == result_2.train_auc, (
        f"RF train_auc differs: {result_1.train_auc} vs {result_2.train_auc}"
    )
    assert result_1.test_auc == result_2.test_auc, (
        f"RF test_auc differs: {result_1.test_auc} vs {result_2.test_auc}"
    )


# ===========================================================================
# Unit Tests for unified_runner (Task 11.3)
# 测试 pipeline 不因单 event 失败而中断
# 测试输出目录约束（不写入禁止目录）
# Requirements: 9.2, 9.3, 9.4
# ===========================================================================

from dataclasses import fields
from pathlib import Path
from unittest.mock import patch

from timing_probability_unified.unified_runner import (
    OUTPUT_DIR,
    RANDOM_STATE,
    Original_10_Features,
    PipelineError,
    _simulate_delays_for_events,
)


# ---------------------------------------------------------------------------
# Test: OUTPUT_DIR is within the allowed directory
# Requirement 9.3: 输出仅写入 research/entry_redesign/scripts/output/timing_probability_unified/
# ---------------------------------------------------------------------------


class TestOutputDirConstraints:
    """验证 OUTPUT_DIR 指向允许的输出目录。"""

    def test_output_dir_within_allowed_directory(self):
        """OUTPUT_DIR 必须位于 research/entry_redesign/scripts/output/timing_probability_unified/"""
        parts = OUTPUT_DIR.parts
        # Check that the path ends with the expected suffix
        assert "research" in parts
        assert "entry_redesign" in parts
        assert "scripts" in parts
        assert "output" in parts
        assert "timing_probability_unified" in parts

        # More specifically, check the relative structure
        assert OUTPUT_DIR.name == "timing_probability_unified"
        assert OUTPUT_DIR.parent.name == "output"

    def test_output_dir_not_in_internal(self):
        """OUTPUT_DIR 不得指向 internal/ 目录"""
        path_str = str(OUTPUT_DIR)
        assert "/internal/" not in path_str
        assert not path_str.startswith("internal/")

    def test_output_dir_not_in_deployments(self):
        """OUTPUT_DIR 不得指向 deployments/ 目录"""
        path_str = str(OUTPUT_DIR)
        assert "/deployments/" not in path_str
        assert not path_str.startswith("deployments/")

    def test_output_dir_not_in_github_workflows(self):
        """OUTPUT_DIR 不得指向 .github/workflows/ 目录"""
        path_str = str(OUTPUT_DIR)
        assert "/.github/workflows/" not in path_str

    def test_output_dir_not_in_cmd(self):
        """OUTPUT_DIR 不得指向 cmd/ 目录"""
        path_str = str(OUTPUT_DIR)
        assert "/cmd/" not in path_str
        assert not path_str.startswith("cmd/")

    def test_output_dir_not_in_web(self):
        """OUTPUT_DIR 不得指向 web/ 目录"""
        path_str = str(OUTPUT_DIR)
        assert "/web/" not in path_str
        assert not path_str.startswith("web/")


# ---------------------------------------------------------------------------
# Test: RANDOM_STATE == 42
# Requirement 9.2: 确定性 random_state=42
# ---------------------------------------------------------------------------


class TestRandomState:
    """验证 RANDOM_STATE 常量。"""

    def test_random_state_is_42(self):
        """RANDOM_STATE 必须为 42 以保证确定性。"""
        assert RANDOM_STATE == 42

    def test_random_state_is_int(self):
        """RANDOM_STATE 必须为整数类型。"""
        assert isinstance(RANDOM_STATE, int)


# ---------------------------------------------------------------------------
# Test: Original_10_Features has exactly 10 elements
# Requirement 9.2: 使用 Original_10_Features
# ---------------------------------------------------------------------------


class TestOriginal10Features:
    """验证 Original_10_Features 列表。"""

    def test_exactly_10_features(self):
        """Original_10_Features 必须恰好包含 10 个元素。"""
        assert len(Original_10_Features) == 10

    def test_all_strings(self):
        """所有特征名必须为字符串。"""
        for feat in Original_10_Features:
            assert isinstance(feat, str), f"Feature {feat!r} is not a string"

    def test_no_duplicates(self):
        """特征名不得重复。"""
        assert len(set(Original_10_Features)) == len(Original_10_Features)

    def test_expected_features_present(self):
        """验证关键特征名存在。"""
        expected = {
            "signal_atr_percentile",
            "roundtrip_cost_atr",
            "prev1_body_atr",
            "prev1_range_atr",
            "prev1_close_pos_side",
            "prev_sma5_gap_atr",
            "prev_sma5_slope_atr",
            "level_to_prev_close_atr",
            "level_to_signal_open_atr",
            "touch_extension_atr",
        }
        assert set(Original_10_Features) == expected


# ---------------------------------------------------------------------------
# Test: PipelineError dataclass has all required fields
# ---------------------------------------------------------------------------


class TestPipelineError:
    """验证 PipelineError 数据类完整性。"""

    def test_has_all_required_fields(self):
        """PipelineError 必须包含所有必需字段。"""
        field_names = {f.name for f in fields(PipelineError)}
        expected_fields = {"event_id", "stage", "error_type", "error_message", "action_taken"}
        assert field_names == expected_fields

    def test_can_instantiate(self):
        """PipelineError 可以正常实例化。"""
        err = PipelineError(
            event_id="test_001",
            stage="delay_sim",
            error_type="ValueError",
            error_message="test error",
            action_taken="skipped",
        )
        assert err.event_id == "test_001"
        assert err.stage == "delay_sim"
        assert err.error_type == "ValueError"
        assert err.error_message == "test error"
        assert err.action_taken == "skipped"

    def test_field_types_are_str(self):
        """所有字段类型标注为 str。"""
        for f in fields(PipelineError):
            assert f.type == "str" or f.type is str, (
                f"Field {f.name} has type {f.type}, expected str"
            )


# ---------------------------------------------------------------------------
# Test: _simulate_delays_for_events — single event exception produces
#       placeholder and error record (not crash)
# Requirement 9.3, 9.4: 单 event 失败不中断 pipeline
# ---------------------------------------------------------------------------


class TestSimulateDelaysForEvents:
    """验证 _simulate_delays_for_events 的容错行为。"""

    def _make_events_df(self, n: int = 3) -> pd.DataFrame:
        """创建测试用事件 DataFrame。"""
        return pd.DataFrame(
            {
                "event_id": [f"evt_{i}" for i in range(n)],
                "symbol": ["BTCUSDT"] * n,
                "side": ["long"] * n,
                "touch_time": pd.date_range("2025-01-01", periods=n, freq="h", tz="UTC"),
                "touch_price": [50000.0 + i * 100 for i in range(n)],
                "speed_300s_atr": [0.5] * n,
            }
        )

    def test_no_bar_data_produces_placeholder_not_crash(self):
        """当 bars_cache 为空时，所有 event 产出 placeholder 而非崩溃。"""
        events = self._make_events_df(2)
        empty_cache: dict = {}

        delay_results, errors = _simulate_delays_for_events(events, empty_cache)

        # Pipeline 不崩溃
        assert len(delay_results) == 2
        assert len(errors) == 2

        # 每个 event 产出 5 个 placeholder DelayResult
        for event_delays in delay_results:
            assert len(event_delays) == 5
            for dr in event_delays:
                assert dr.traded is False
                assert dr.exit_reason == "NoData"

    def test_exception_in_simulate_produces_placeholder(self):
        """当 simulate_all_delays 抛出异常时，产出 placeholder 和 error record。"""
        events = self._make_events_df(2)

        # 提供有效的 bars_cache 使得 _get_bars_for_event 返回非 None
        fake_bars = pd.DataFrame(
            {
                "open": [50000.0],
                "high": [50100.0],
                "low": [49900.0],
                "close": [50050.0],
                "volume": [1.0],
            },
            index=pd.DatetimeIndex(
                [pd.Timestamp("2025-01-01 00:00:00", tz="UTC")], name="timestamp"
            ),
        )
        bars_cache = {
            "BTCUSDT_202501": fake_bars,
        }

        with patch(
            "timing_probability_unified.unified_runner.simulate_all_delays",
            side_effect=RuntimeError("Simulated failure"),
        ):
            delay_results, errors = _simulate_delays_for_events(events, bars_cache)

        # Pipeline 不崩溃
        assert len(delay_results) == 2
        assert len(errors) == 2

        # 每个 event 产出 5 个 placeholder
        for event_delays in delay_results:
            assert len(event_delays) == 5
            for dr in event_delays:
                assert dr.traded is False
                assert dr.exit_reason == "SimError"

        # Error records 包含正确信息
        for err in errors:
            assert isinstance(err, PipelineError)
            assert err.stage == "delay_sim"
            assert err.error_type == "RuntimeError"
            assert "Simulated failure" in err.error_message
            assert err.action_taken == "skipped"

    def test_empty_events_produces_empty_results(self):
        """空事件池不崩溃，返回空列表。"""
        events = pd.DataFrame(
            columns=["event_id", "symbol", "side", "touch_time", "touch_price"]
        )
        delay_results, errors = _simulate_delays_for_events(events, {})

        assert delay_results == []
        assert errors == []

    def test_mixed_success_and_failure_does_not_crash(self):
        """部分 event 有 bar 数据、部分没有时，pipeline 继续执行不中断。"""
        events = self._make_events_df(3)

        # Only provide bars for the first event's month
        # All 3 events are in 2025-01, so all will find bars
        # But we mock simulate_all_delays to fail on the second call
        fake_bars = pd.DataFrame(
            {
                "open": [50000.0],
                "high": [50100.0],
                "low": [49900.0],
                "close": [50050.0],
                "volume": [1.0],
            },
            index=pd.DatetimeIndex(
                [pd.Timestamp("2025-01-01 00:00:00", tz="UTC")], name="timestamp"
            ),
        )
        bars_cache = {"BTCUSDT_202501": fake_bars}

        # First call succeeds, second raises, third succeeds
        success_result = [
            _make_delay_result("evt_x", label, 0.001)
            for label in ["D0", "D5", "D10", "D15", "pullback"]
        ]

        call_count = [0]

        def _side_effect(*args, **kwargs):
            call_count[0] += 1
            if call_count[0] == 2:
                raise ValueError("Bar data corrupt")
            return success_result

        with patch(
            "timing_probability_unified.unified_runner.simulate_all_delays",
            side_effect=_side_effect,
        ):
            delay_results, errors = _simulate_delays_for_events(events, bars_cache)

        # All 3 events produce results (pipeline not interrupted)
        assert len(delay_results) == 3

        # First event succeeded
        assert all(dr.traded is True for dr in delay_results[0])

        # Second event failed → placeholder
        assert all(dr.traded is False for dr in delay_results[1])
        assert all(dr.exit_reason == "SimError" for dr in delay_results[1])

        # Third event succeeded
        assert all(dr.traded is True for dr in delay_results[2])

        # Only 1 error recorded (the second event)
        assert len(errors) == 1
        assert errors[0].event_id == "evt_1"
        assert errors[0].error_type == "ValueError"
        assert "Bar data corrupt" in errors[0].error_message
