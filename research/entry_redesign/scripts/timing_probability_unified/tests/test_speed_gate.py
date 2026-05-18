"""Unit tests for speed_gate — speed_300s_atr 质量过滤。

Tests cover:
1. All pass (threshold very low): all speed_300s_atr values above threshold → all True
2. All filtered (threshold very high): all values below threshold → all False
3. aggressive_gate_warning triggered: pass_rate < 0.70
4. aggressive_gate_warning NOT triggered: pass_rate >= 0.70
5. compute_speed_gate returns correct length array
6. analyze_speed_gate returns correct SpeedGateResult fields

Requirements: 5.2, 5.5
"""

from __future__ import annotations

from typing import List, Optional

import numpy as np
import pandas as pd
import pytest

from timing_probability_unified.speed_gate import (
    SpeedGateResult,
    analyze_speed_gate,
    compute_speed_gate,
)


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _make_events(speed_values: List[float], n: Optional[int] = None) -> pd.DataFrame:
    """Create a minimal events DataFrame with given speed_300s_atr values."""
    if n is not None:
        speed_values = speed_values[:n]
    n_events = len(speed_values)
    return pd.DataFrame(
        {
            "event_id": [f"evt_{i:03d}" for i in range(n_events)],
            "symbol": ["BTCUSDT" if i % 2 == 0 else "ETHUSDT" for i in range(n_events)],
            "side": ["long"] * n_events,
            "touch_time": pd.date_range("2025-01-01", periods=n_events, freq="h", tz="UTC"),
            "speed_300s_atr": speed_values,
        }
    )


def _make_trades(
    n: int,
    speed_gate_pass: List[bool],
    weighted_pnls: Optional[List[float]] = None,
) -> pd.DataFrame:
    """Create a minimal unified_trades DataFrame for analyze_speed_gate tests."""
    if weighted_pnls is None:
        weighted_pnls = [0.01] * n
    return pd.DataFrame(
        {
            "event_id": [f"evt_{i:03d}" for i in range(n)],
            "symbol": ["BTCUSDT" if i % 2 == 0 else "ETHUSDT" for i in range(n)],
            "side": ["long"] * n,
            "touch_time": pd.date_range("2025-01-01", periods=n, freq="h", tz="UTC"),
            "timing_prediction": ["fast"] * n,
            "selected_delay": ["D0"] * n,
            "rf_probability": [0.5] * n,
            "sizing_multiplier": [1.0] * n,
            "position_size": [0.30] * n,
            "delay_pnl_pct": weighted_pnls,
            "weighted_pnl": weighted_pnls,
            "speed_300s_atr": [1.0 + i * 0.1 for i in range(n)],
            "speed_gate_pass": speed_gate_pass,
        }
    )


# ---------------------------------------------------------------------------
# Test 1: All pass (threshold very low)
# ---------------------------------------------------------------------------


class TestComputeSpeedGateAllPass:
    """When threshold is very low, all events should pass the speed gate."""

    def test_all_pass_with_low_quantile_train(self):
        """All speed_300s_atr values above threshold → all True.

        Validates: Requirements 5.2
        """
        # All events have speed >= 1.0; train set has very low values → low threshold
        events = _make_events([1.0, 1.5, 2.0, 2.5, 3.0])
        # Train events with very low speed values → q10 threshold will be very low
        train_events = _make_events([0.01, 0.02, 0.03, 0.04, 0.05])

        speed_gate_pass, threshold = compute_speed_gate(events, train_events, quantile=0.10)

        # Threshold should be very low (around 0.014)
        assert threshold < 0.1
        # All events should pass since their speed values are all >= threshold
        assert np.all(speed_gate_pass), (
            f"Expected all True, got {speed_gate_pass} with threshold={threshold}"
        )

    def test_all_pass_with_zero_quantile(self):
        """With quantile=0.0, threshold is the minimum → all pass.

        Validates: Requirements 5.2
        """
        events = _make_events([1.0, 2.0, 3.0, 4.0, 5.0])
        train_events = _make_events([1.0, 2.0, 3.0, 4.0, 5.0])

        speed_gate_pass, threshold = compute_speed_gate(events, train_events, quantile=0.0)

        # q0 = minimum of train set = 1.0
        assert threshold == pytest.approx(1.0)
        # All events have speed >= 1.0
        assert np.all(speed_gate_pass)


# ---------------------------------------------------------------------------
# Test 2: All filtered (threshold very high)
# ---------------------------------------------------------------------------


class TestComputeSpeedGateAllFiltered:
    """When threshold is very high, all events should be filtered."""

    def test_all_filtered_with_high_threshold(self):
        """All speed_300s_atr values below threshold → all False.

        Validates: Requirements 5.2
        """
        # Events have low speed values
        events = _make_events([0.1, 0.2, 0.3, 0.4, 0.5])
        # Train events with very high speed values → high threshold
        train_events = _make_events([10.0, 20.0, 30.0, 40.0, 50.0])

        speed_gate_pass, threshold = compute_speed_gate(events, train_events, quantile=0.10)

        # Threshold should be high (around 14.0)
        assert threshold > 1.0
        # All events should fail since their speed values are all < threshold
        assert not np.any(speed_gate_pass), (
            f"Expected all False, got {speed_gate_pass} with threshold={threshold}"
        )

    def test_all_filtered_with_quantile_1(self):
        """With quantile=1.0, threshold is the maximum → only events at max pass.

        Validates: Requirements 5.2
        """
        events = _make_events([0.5, 1.0, 1.5, 2.0, 2.5])
        train_events = _make_events([1.0, 2.0, 3.0, 4.0, 5.0])

        speed_gate_pass, threshold = compute_speed_gate(events, train_events, quantile=1.0)

        # q100 = maximum of train set = 5.0
        assert threshold == pytest.approx(5.0)
        # All events have speed < 5.0, so all should fail
        assert not np.any(speed_gate_pass)


# ---------------------------------------------------------------------------
# Test 3: aggressive_gate_warning triggered (pass_rate < 0.70)
# ---------------------------------------------------------------------------


class TestAggressiveGateWarningTriggered:
    """aggressive_gate_warning should be True when pass_rate < 0.70."""

    def test_warning_triggered_low_pass_rate(self):
        """pass_rate < 0.70 → aggressive_gate_warning=True.

        Validates: Requirements 5.5
        """
        n = 10
        # Only 6 out of 10 pass → pass_rate = 0.60 < 0.70
        speed_gate_pass = np.array([True] * 6 + [False] * 4)
        trades = _make_trades(n, speed_gate_pass.tolist())

        result = analyze_speed_gate(trades, speed_gate_pass, threshold=1.0)

        assert result.aggressive_gate_warning is True
        assert result.gate_pass_rate == pytest.approx(0.60)

    def test_warning_triggered_zero_pass_rate(self):
        """pass_rate = 0.0 → aggressive_gate_warning=True.

        Validates: Requirements 5.5
        """
        n = 5
        speed_gate_pass = np.array([False] * n)
        trades = _make_trades(n, speed_gate_pass.tolist())

        result = analyze_speed_gate(trades, speed_gate_pass, threshold=10.0)

        assert result.aggressive_gate_warning is True
        assert result.gate_pass_rate == pytest.approx(0.0)

    def test_warning_triggered_at_69_percent(self):
        """pass_rate = 0.69 (just below 0.70) → aggressive_gate_warning=True.

        Validates: Requirements 5.5
        """
        # 69 out of 100 pass → pass_rate = 0.69
        n = 100
        speed_gate_pass = np.array([True] * 69 + [False] * 31)
        trades = _make_trades(n, speed_gate_pass.tolist())

        result = analyze_speed_gate(trades, speed_gate_pass, threshold=1.0)

        assert result.aggressive_gate_warning is True
        assert result.gate_pass_rate == pytest.approx(0.69)


# ---------------------------------------------------------------------------
# Test 4: aggressive_gate_warning NOT triggered (pass_rate >= 0.70)
# ---------------------------------------------------------------------------


class TestAggressiveGateWarningNotTriggered:
    """aggressive_gate_warning should be False when pass_rate >= 0.70."""

    def test_no_warning_high_pass_rate(self):
        """pass_rate >= 0.70 → aggressive_gate_warning=False.

        Validates: Requirements 5.5
        """
        n = 10
        # 8 out of 10 pass → pass_rate = 0.80 >= 0.70
        speed_gate_pass = np.array([True] * 8 + [False] * 2)
        trades = _make_trades(n, speed_gate_pass.tolist())

        result = analyze_speed_gate(trades, speed_gate_pass, threshold=1.0)

        assert result.aggressive_gate_warning is False
        assert result.gate_pass_rate == pytest.approx(0.80)

    def test_no_warning_all_pass(self):
        """pass_rate = 1.0 → aggressive_gate_warning=False.

        Validates: Requirements 5.5
        """
        n = 10
        speed_gate_pass = np.array([True] * n)
        trades = _make_trades(n, speed_gate_pass.tolist())

        result = analyze_speed_gate(trades, speed_gate_pass, threshold=0.1)

        assert result.aggressive_gate_warning is False
        assert result.gate_pass_rate == pytest.approx(1.0)

    def test_no_warning_at_exactly_70_percent(self):
        """pass_rate = 0.70 (boundary) → aggressive_gate_warning=False.

        Validates: Requirements 5.5
        """
        # 7 out of 10 pass → pass_rate = 0.70
        n = 10
        speed_gate_pass = np.array([True] * 7 + [False] * 3)
        trades = _make_trades(n, speed_gate_pass.tolist())

        result = analyze_speed_gate(trades, speed_gate_pass, threshold=1.0)

        assert result.aggressive_gate_warning is False
        assert result.gate_pass_rate == pytest.approx(0.70)


# ---------------------------------------------------------------------------
# Test 5: compute_speed_gate returns correct length array
# ---------------------------------------------------------------------------


class TestComputeSpeedGateLength:
    """compute_speed_gate must return a bool array of the same length as events."""

    def test_returns_correct_length(self):
        """Output array length == input events length.

        Validates: Requirements 5.2
        """
        for n in [1, 5, 10, 50]:
            events = _make_events([1.0 + i * 0.1 for i in range(n)])
            train_events = _make_events([0.5, 1.0, 1.5, 2.0, 3.0])

            speed_gate_pass, threshold = compute_speed_gate(events, train_events)

            assert len(speed_gate_pass) == n, (
                f"Expected length {n}, got {len(speed_gate_pass)}"
            )

    def test_returns_bool_dtype(self):
        """Output array should be boolean dtype.

        Validates: Requirements 5.2
        """
        events = _make_events([1.0, 2.0, 3.0])
        train_events = _make_events([0.5, 1.0, 1.5])

        speed_gate_pass, threshold = compute_speed_gate(events, train_events)

        assert speed_gate_pass.dtype == bool, (
            f"Expected bool dtype, got {speed_gate_pass.dtype}"
        )

    def test_single_event(self):
        """Single event should return array of length 1.

        Validates: Requirements 5.2
        """
        events = _make_events([2.0])
        train_events = _make_events([1.0, 1.5, 2.0, 2.5, 3.0])

        speed_gate_pass, threshold = compute_speed_gate(events, train_events)

        assert len(speed_gate_pass) == 1


# ---------------------------------------------------------------------------
# Test 6: analyze_speed_gate returns correct SpeedGateResult fields
# ---------------------------------------------------------------------------


class TestAnalyzeSpeedGateResult:
    """analyze_speed_gate should return a SpeedGateResult with all fields populated."""

    def test_returns_speed_gate_result_type(self):
        """Return type should be SpeedGateResult.

        Validates: Requirements 5.2, 5.5
        """
        n = 5
        speed_gate_pass = np.array([True, True, True, False, False])
        trades = _make_trades(n, speed_gate_pass.tolist())

        result = analyze_speed_gate(trades, speed_gate_pass, threshold=1.5)

        assert isinstance(result, SpeedGateResult)

    def test_threshold_field_matches_input(self):
        """threshold field should match the input threshold value.

        Validates: Requirements 5.2
        """
        n = 5
        speed_gate_pass = np.array([True] * 3 + [False] * 2)
        trades = _make_trades(n, speed_gate_pass.tolist())

        result = analyze_speed_gate(trades, speed_gate_pass, threshold=2.345)

        assert result.threshold == pytest.approx(2.345)

    def test_gate_pass_rate_correct(self):
        """gate_pass_rate should equal count(True) / total.

        Validates: Requirements 5.5
        """
        n = 10
        speed_gate_pass = np.array([True] * 7 + [False] * 3)
        trades = _make_trades(n, speed_gate_pass.tolist())

        result = analyze_speed_gate(trades, speed_gate_pass, threshold=1.0)

        assert result.gate_pass_rate == pytest.approx(0.70)

    def test_avg_pnl_fields(self):
        """retained_avg_pnl and filtered_avg_pnl should be correct averages.

        Validates: Requirements 5.2
        """
        n = 4
        speed_gate_pass = np.array([True, True, False, False])
        # Retained events (index 0, 1) have pnl 0.02, 0.04 → avg = 0.03
        # Filtered events (index 2, 3) have pnl -0.01, -0.03 → avg = -0.02
        weighted_pnls = [0.02, 0.04, -0.01, -0.03]
        trades = _make_trades(n, speed_gate_pass.tolist(), weighted_pnls=weighted_pnls)

        result = analyze_speed_gate(trades, speed_gate_pass, threshold=1.0)

        assert result.retained_avg_pnl == pytest.approx(0.03)
        assert result.filtered_avg_pnl == pytest.approx(-0.02)

    def test_all_fields_present(self):
        """All SpeedGateResult fields should be populated (not None).

        Validates: Requirements 5.2, 5.5
        """
        n = 6
        speed_gate_pass = np.array([True] * 4 + [False] * 2)
        trades = _make_trades(n, speed_gate_pass.tolist())

        result = analyze_speed_gate(trades, speed_gate_pass, threshold=1.0)

        assert result.threshold is not None
        assert result.gate_pass_rate is not None
        assert result.gate_on_calendar_sum is not None
        assert result.gate_off_calendar_sum is not None
        assert result.gate_on_worst_sm is not None
        assert result.gate_off_worst_sm is not None
        assert result.filtered_avg_pnl is not None
        assert result.retained_avg_pnl is not None
        assert result.aggressive_gate_warning is not None
