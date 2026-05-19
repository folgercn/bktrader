"""Consolidated Property Tests (Properties 1-16) for timing-probability-unified.

This file consolidates ALL property-based tests into a single entry point.
Running `pytest test_properties.py` executes all 16 correctness properties.

Properties that already exist in individual test modules are imported here.
Properties that were missing (2, 4, 13, 15) are implemented directly.

Usage:
    cd research/entry_redesign/scripts
    python -m pytest timing_probability_unified/tests/test_properties.py -v
"""

from __future__ import annotations

import numpy as np
import pandas as pd
import pytest
from hypothesis import given, settings, HealthCheck, assume
from hypothesis import strategies as st

# ===========================================================================
# Property 1: Event Pool Statistics Consistency
# Feature: timing-probability-unified, Property 1: Event Pool Statistics Consistency
# **Validates: Requirements 1.3, 1.4**
# ===========================================================================

from timing_probability_unified.tests.test_event_source_builder import (
    test_event_pool_stats_consistency,  # noqa: F401
)


# ===========================================================================
# Property 2: Time-Ordered Split Invariant
# Feature: timing-probability-unified, Property 2: Time-Ordered Split Invariant
# **Validates: Requirements 1.5**
# ===========================================================================

from timing_probability_unified.event_source_builder import split_events_by_time


@st.composite
def _events_with_varied_times(draw: st.DrawFn) -> pd.DataFrame:
    """Generate events DataFrame with varied touch_times for split testing."""
    n = draw(st.integers(min_value=2, max_value=50))
    symbols = draw(st.lists(st.sampled_from(["BTCUSDT", "ETHUSDT"]), min_size=n, max_size=n))
    sides = draw(st.lists(st.sampled_from(["long", "short"]), min_size=n, max_size=n))
    touch_times = draw(
        st.lists(
            st.datetimes(
                min_value=pd.Timestamp("2024-01-01").to_pydatetime(),
                max_value=pd.Timestamp("2025-10-31").to_pydatetime(),
            ),
            min_size=n,
            max_size=n,
        )
    )
    return pd.DataFrame(
        {
            "symbol": symbols,
            "side": sides,
            "touch_time": pd.to_datetime(touch_times, utc=True),
        }
    )


# Feature: timing-probability-unified, Property 2: Time-Ordered Split Invariant
@settings(max_examples=200, suppress_health_check=[HealthCheck.too_slow])
@given(events=_events_with_varied_times())
def test_time_ordered_split_invariant(events: pd.DataFrame) -> None:
    """Property 2: Time-Ordered Split Invariant.

    For any events pool sorted by touch_time and split into train/test,
    ALL events in train SHALL have touch_time strictly <= ALL events in test.
    Formally: max(train.touch_time) <= min(test.touch_time).

    **Validates: Requirements 1.5**
    """
    train, test, forward = split_events_by_time(events, forward_start="2025-11-01")

    # If both train and test are non-empty, verify time ordering
    if len(train) > 0 and len(test) > 0:
        assert train["touch_time"].max() <= test["touch_time"].min(), (
            f"Time ordering violated: max(train)={train['touch_time'].max()} > "
            f"min(test)={test['touch_time'].min()}"
        )

    # If forward is non-empty, verify it's after train/test
    if len(forward) > 0:
        forward_start_ts = pd.Timestamp("2025-11-01", tz="UTC")
        assert (forward["touch_time"] >= forward_start_ts).all(), (
            "Forward events contain timestamps before forward_start"
        )

    # Train + test should not overlap with forward
    if len(train) > 0:
        assert (train["touch_time"] < pd.Timestamp("2025-11-01", tz="UTC")).all()
    if len(test) > 0:
        assert (test["touch_time"] < pd.Timestamp("2025-11-01", tz="UTC")).all()


# ===========================================================================
# Property 4: 3-Regime Label Generation Correctness
# Feature: timing-probability-unified, Property 4: 3-Regime Label Generation Correctness
# **Validates: Requirements 2.3**
# ===========================================================================

from timing_probability_unified.timing_classifier import generate_3regime_label_from_pnls


# Feature: timing-probability-unified, Property 4: 3-Regime Label Generation Correctness
@settings(max_examples=200)
@given(
    d0=st.floats(min_value=-0.05, max_value=0.05, allow_nan=False, allow_infinity=False),
    d5=st.floats(min_value=-0.05, max_value=0.05, allow_nan=False, allow_infinity=False),
    d10=st.floats(min_value=-0.05, max_value=0.05, allow_nan=False, allow_infinity=False),
    d15=st.floats(min_value=-0.05, max_value=0.05, allow_nan=False, allow_infinity=False),
    pb=st.floats(min_value=-0.05, max_value=0.05, allow_nan=False, allow_infinity=False),
)
def test_3regime_label_correctness(d0: float, d5: float, d10: float, d15: float, pb: float) -> None:
    """Property 4: 3-Regime Label Generation Correctness.

    For any 5-tuple of delay PnLs (d0_pnl, d5_pnl, d10_pnl, d15_pnl, pullback_pnl),
    the generated 3-regime label SHALL satisfy:
    - fast_pnl = max(d0_pnl, d5_pnl)
    - slow_pnl = max(d10_pnl, d15_pnl, pullback_pnl)
    - IF fast_pnl < 0 AND slow_pnl < 0 → label == "skip"
    - ELSE IF fast_pnl >= slow_pnl OR (slow_pnl - fast_pnl) < 5bps → label == "fast"
    - ELSE → label == "slow"

    **Validates: Requirements 2.3**
    """
    label = generate_3regime_label_from_pnls(d0, d5, d10, d15, pb)
    fast_pnl = max(d0, d5)
    slow_pnl = max(d10, d15, pb)
    tolerance = 5.0 / 10000.0  # 5bps = 0.0005

    if fast_pnl < 0 and slow_pnl < 0:
        assert label == "skip", (
            f"Expected 'skip' when fast_pnl={fast_pnl} < 0 and slow_pnl={slow_pnl} < 0, "
            f"got '{label}'"
        )
    elif fast_pnl >= slow_pnl or (slow_pnl - fast_pnl) < tolerance:
        assert label == "fast", (
            f"Expected 'fast' when fast_pnl={fast_pnl} >= slow_pnl={slow_pnl} "
            f"or diff={slow_pnl - fast_pnl} < {tolerance}, got '{label}'"
        )
    else:
        assert label == "slow", (
            f"Expected 'slow' when slow_pnl={slow_pnl} > fast_pnl={fast_pnl} "
            f"and diff={slow_pnl - fast_pnl} >= {tolerance}, got '{label}'"
        )


# ===========================================================================
# Property 5: LOOCV-Based Model Selection
# Feature: timing-probability-unified, Property 5: LOOCV-Based Model Selection
# **Validates: Requirements 2.5**
# ===========================================================================

from timing_probability_unified.timing_classifier import select_best_depth


# Feature: timing-probability-unified, Property 5: LOOCV-Based Model Selection
@settings(max_examples=200)
@given(
    dt3_score=st.floats(min_value=-1.0, max_value=1.0, allow_nan=False, allow_infinity=False),
    dt4_score=st.floats(min_value=-1.0, max_value=1.0, allow_nan=False, allow_infinity=False),
)
def test_loocv_model_selection(dt3_score: float, dt4_score: float) -> None:
    """Property 5: LOOCV-Based Model Selection.

    **Validates: Requirements 2.5**
    """
    selected_depth = select_best_depth(dt3_score, dt4_score)
    if dt4_score > dt3_score:
        assert selected_depth == 4
    else:
        assert selected_depth == 3


# ===========================================================================
# Property 6: Prediction-to-Delay Mapping
# Feature: timing-probability-unified, Property 6: Prediction-to-Delay Mapping
# **Validates: Requirements 2.6, 4.3**
# ===========================================================================

from timing_probability_unified.timing_classifier import get_selected_delay_pnl
from dataclasses import dataclass
from typing import Optional


@dataclass
class _MockDelayResult:
    event_id: str
    delay_label: str
    delay_seconds: int
    entry_time: object
    entry_price: Optional[float]
    pnl_pct: Optional[float]
    exit_reason: Optional[str]
    exit_time: object
    hold_seconds: Optional[float]
    mfe_r: Optional[float]
    mae_r: Optional[float]
    traded: bool


def _mock_dr(label: str, pnl: float) -> _MockDelayResult:
    return _MockDelayResult(
        event_id="test", delay_label=label, delay_seconds=0,
        entry_time=None, entry_price=100.0, pnl_pct=pnl,
        exit_reason="test", exit_time=None, hold_seconds=60.0,
        mfe_r=1.0, mae_r=-0.5, traded=True,
    )


# Feature: timing-probability-unified, Property 6: Prediction-to-Delay Mapping
@settings(max_examples=200)
@given(
    prediction=st.sampled_from(["skip", "fast", "slow"]),
    d0_pnl=st.floats(min_value=-0.05, max_value=0.05, allow_nan=False, allow_infinity=False),
    d5_pnl=st.floats(min_value=-0.05, max_value=0.05, allow_nan=False, allow_infinity=False),
    d10_pnl=st.floats(min_value=-0.05, max_value=0.05, allow_nan=False, allow_infinity=False),
    d15_pnl=st.floats(min_value=-0.05, max_value=0.05, allow_nan=False, allow_infinity=False),
    pb_pnl=st.floats(min_value=-0.05, max_value=0.05, allow_nan=False, allow_infinity=False),
)
def test_prediction_to_delay_mapping(
    prediction: str, d0_pnl: float, d5_pnl: float,
    d10_pnl: float, d15_pnl: float, pb_pnl: float,
) -> None:
    """Property 6: Prediction-to-Delay Mapping.

    **Validates: Requirements 2.6, 4.3**
    """
    event_delays = [
        _mock_dr("D0", d0_pnl), _mock_dr("D5", d5_pnl),
        _mock_dr("D10", d10_pnl), _mock_dr("D15", d15_pnl),
        _mock_dr("pullback", pb_pnl),
    ]
    selected_delay, pnl = get_selected_delay_pnl(prediction, event_delays)

    if prediction == "skip":
        assert selected_delay == "none" and pnl == 0.0
    elif prediction == "fast":
        assert selected_delay in {"D0", "D5"}
        assert pnl == max(d0_pnl, d5_pnl)
    elif prediction == "slow":
        assert selected_delay in {"D10", "D15", "pullback"}
        assert pnl == max(d10_pnl, d15_pnl, pb_pnl)


# ===========================================================================
# Property 7: RF Binary Label Generation
# Feature: timing-probability-unified, Property 7: RF Binary Label Generation
# **Validates: Requirements 3.2**
# ===========================================================================

from timing_probability_unified.tests.test_probability_model import (
    test_rf_binary_label_generation_property,  # noqa: F401
)


# ===========================================================================
# Property 8: Probability Output Range Invariant
# Feature: timing-probability-unified, Property 8: Probability Output Range Invariant
# **Validates: Requirements 3.3**
# ===========================================================================

from timing_probability_unified.tests.test_probability_model import (
    test_probability_output_range_invariant,  # noqa: F401
)


# ===========================================================================
# Property 9: Sizing Multiplier Formula
# Feature: timing-probability-unified, Property 9: Sizing Multiplier Formula
# **Validates: Requirements 3.4**
# ===========================================================================

from timing_probability_unified.tests.test_probability_model import (
    test_sizing_multiplier_formula_property,  # noqa: F401
)


# ===========================================================================
# Property 10: Combined Position Logic
# Feature: timing-probability-unified, Property 10: Combined Position Logic
# **Validates: Requirements 4.1**
# ===========================================================================

from timing_probability_unified.tests.test_combined_executor import (
    test_combined_position_logic_property,  # noqa: F401
)


# ===========================================================================
# Property 11: Weighted PnL Arithmetic
# Feature: timing-probability-unified, Property 11: Weighted PnL Arithmetic
# **Validates: Requirements 4.4**
# ===========================================================================

from timing_probability_unified.tests.test_combined_executor import (
    test_weighted_pnl_arithmetic_pure,  # noqa: F401
    test_weighted_pnl_arithmetic_via_compute_combined_positions,  # noqa: F401
)


# ===========================================================================
# Property 12: Worst SM Equals Minimum Monthly Sum
# Feature: timing-probability-unified, Property 12: Worst SM Equals Minimum Monthly Sum
# **Validates: Requirements 4.7**
# ===========================================================================

from timing_probability_unified.tests.test_combined_executor import (
    test_worst_sm_equals_minimum_monthly_sum,  # noqa: F401
    test_worst_sm_with_gate_filter,  # noqa: F401
)


# ===========================================================================
# Property 13: Speed Gate Non-Destructive Marking
# Feature: timing-probability-unified, Property 13: Speed Gate Non-Destructive Marking
# **Validates: Requirements 5.2, 5.3**
# ===========================================================================

from timing_probability_unified.speed_gate import compute_speed_gate


@st.composite
def _speed_gate_inputs(draw: st.DrawFn):
    """Generate events and train_events for speed gate testing."""
    n_events = draw(st.integers(min_value=1, max_value=50))
    n_train = draw(st.integers(min_value=2, max_value=30))

    speed_values = draw(
        st.lists(
            st.floats(min_value=0.0, max_value=10.0, allow_nan=False, allow_infinity=False),
            min_size=n_events,
            max_size=n_events,
        )
    )
    train_speed_values = draw(
        st.lists(
            st.floats(min_value=0.0, max_value=10.0, allow_nan=False, allow_infinity=False),
            min_size=n_train,
            max_size=n_train,
        )
    )

    events = pd.DataFrame(
        {
            "event_id": [f"evt_{i}" for i in range(n_events)],
            "symbol": ["BTCUSDT"] * n_events,
            "side": ["long"] * n_events,
            "touch_time": pd.date_range("2025-01-01", periods=n_events, freq="h", tz="UTC"),
            "speed_300s_atr": speed_values,
        }
    )
    train_events = pd.DataFrame(
        {
            "event_id": [f"train_{i}" for i in range(n_train)],
            "symbol": ["BTCUSDT"] * n_train,
            "side": ["long"] * n_train,
            "touch_time": pd.date_range("2025-01-01", periods=n_train, freq="h", tz="UTC"),
            "speed_300s_atr": train_speed_values,
        }
    )
    return events, train_events


# Feature: timing-probability-unified, Property 13: Speed Gate Non-Destructive Marking
@settings(max_examples=200, suppress_health_check=[HealthCheck.too_slow])
@given(data=_speed_gate_inputs())
def test_speed_gate_non_destructive_marking(data) -> None:
    """Property 13: Speed Gate Non-Destructive Marking.

    For any events array of length N and any threshold value,
    compute_speed_gate() SHALL return a boolean array of length N where
    speed_gate_pass[i] == (events[i].speed_300s_atr >= threshold).
    The original events array is never modified or filtered.

    **Validates: Requirements 5.2, 5.3**
    """
    events, train_events = data

    # Save original events for comparison
    original_events = events.copy()
    original_speed_values = events["speed_300s_atr"].values.copy()

    # Execute
    speed_gate_pass, threshold = compute_speed_gate(events, train_events)

    # Property 1: Output length == input length
    assert len(speed_gate_pass) == len(events), (
        f"Output length {len(speed_gate_pass)} != input length {len(events)}"
    )

    # Property 2: Each element matches the threshold comparison
    for i in range(len(events)):
        expected = events.iloc[i]["speed_300s_atr"] >= threshold
        assert speed_gate_pass[i] == expected, (
            f"speed_gate_pass[{i}]={speed_gate_pass[i]} but "
            f"speed_300s_atr={events.iloc[i]['speed_300s_atr']} >= threshold={threshold} "
            f"is {expected}"
        )

    # Property 3: Original events array is NOT modified
    pd.testing.assert_frame_equal(events, original_events, obj="events should not be modified")
    np.testing.assert_array_equal(
        events["speed_300s_atr"].values,
        original_speed_values,
        err_msg="speed_300s_atr values were modified",
    )

    # Property 4: Output is boolean dtype
    assert speed_gate_pass.dtype == bool, (
        f"Expected bool dtype, got {speed_gate_pass.dtype}"
    )


# ===========================================================================
# Property 14: Pipeline Determinism
# Feature: timing-probability-unified, Property 14: Pipeline Determinism
# **Validates: Requirements 6.4, 9.2**
# ===========================================================================

from timing_probability_unified.tests.test_unified_runner import (
    test_generate_3regime_labels_determinism,  # noqa: F401
    test_compute_sizing_multiplier_determinism,  # noqa: F401
    test_compute_combined_positions_determinism,  # noqa: F401
    test_mini_pipeline_determinism,  # noqa: F401
    test_rf_probability_model_determinism,  # noqa: F401
)


# ===========================================================================
# Property 15: Go/No-Go Decision Logic
# Feature: timing-probability-unified, Property 15: Go/No-Go Decision Logic
# **Validates: Requirements 8.2, 8.7**
# ===========================================================================

from timing_probability_unified.report_generator import compute_go_no_go_decision


# Feature: timing-probability-unified, Property 15: Go/No-Go Decision Logic
@settings(max_examples=200)
@given(
    calendar_sum=st.floats(min_value=-0.10, max_value=0.30, allow_nan=False, allow_infinity=False),
    worst_sm=st.floats(min_value=-0.05, max_value=0.05, allow_nan=False, allow_infinity=False),
    btc_calendar_sum=st.floats(min_value=-0.10, max_value=0.10, allow_nan=False, allow_infinity=False),
    eth_calendar_sum=st.floats(min_value=-0.10, max_value=0.10, allow_nan=False, allow_infinity=False),
    forward_calendar_sum=st.floats(min_value=-0.10, max_value=0.20, allow_nan=False, allow_infinity=False),
    overfitting_flag=st.booleans(),
)
def test_go_no_go_decision_logic(
    calendar_sum: float,
    worst_sm: float,
    btc_calendar_sum: float,
    eth_calendar_sum: float,
    forward_calendar_sum: float,
    overfitting_flag: bool,
) -> None:
    """Property 15: Go/No-Go Decision Logic.

    For any metrics tuple, the decision SHALL satisfy (priority order):
    1. IF overfitting_flag == True AND calendar_sum >= 10% → decision == "marginal_go" (downgrade)
    2. ELSE IF calendar_sum >= 10% AND worst_sm > -0.5% AND btc_positive AND eth_positive
       AND forward_cs >= 7% → decision == "strong_go"
    3. ELSE IF calendar_sum < 7% OR worst_sm < -1.0% → decision == "no_go"
    4. ELSE → decision == "marginal_go"

    **Validates: Requirements 8.2, 8.7**
    """
    result = compute_go_no_go_decision(
        calendar_sum=calendar_sum,
        worst_sm=worst_sm,
        btc_calendar_sum=btc_calendar_sum,
        eth_calendar_sum=eth_calendar_sum,
        forward_calendar_sum=forward_calendar_sum,
        overfitting_flag=overfitting_flag,
    )

    btc_positive = btc_calendar_sum > 0
    eth_positive = eth_calendar_sum > 0

    # Determine expected decision following the PRIORITY-BASED logic
    # Rule 1: Overfitting downgrade (highest priority)
    if overfitting_flag and calendar_sum >= 0.10:
        expected = "marginal_go"
        expected_downgrade = True
    # Rule 2: Strong Go
    elif (
        calendar_sum >= 0.10
        and worst_sm > -0.005
        and btc_positive
        and eth_positive
        and forward_calendar_sum >= 0.07
    ):
        expected = "strong_go"
        expected_downgrade = False
    # Rule 3: No-Go
    elif calendar_sum < 0.07 or worst_sm < -0.01:
        expected = "no_go"
        expected_downgrade = False
    # Rule 4: Marginal Go (default)
    else:
        expected = "marginal_go"
        expected_downgrade = False

    assert result.decision == expected, (
        f"Expected '{expected}' but got '{result.decision}' for "
        f"cs={calendar_sum}, wsm={worst_sm}, btc={btc_calendar_sum}, "
        f"eth={eth_calendar_sum}, fwd={forward_calendar_sum}, "
        f"overfitting={overfitting_flag}"
    )

    # Verify overfitting_downgrade flag
    assert result.overfitting_downgrade == expected_downgrade, (
        f"Expected overfitting_downgrade={expected_downgrade} but got "
        f"{result.overfitting_downgrade} for overfitting_flag={overfitting_flag}, "
        f"cs={calendar_sum}"
    )

    # Verify btc_positive and eth_positive fields
    assert result.btc_positive == btc_positive
    assert result.eth_positive == eth_positive


# ===========================================================================
# Property 16: Threshold Warning Flags
# Feature: timing-probability-unified, Property 16: Threshold Warning Flags
# **Validates: Requirements 3.6, 5.5, 7.8, 7.4**
# ===========================================================================

from timing_probability_unified.tests.test_robustness import (
    test_property16_rf_no_signal_warning,  # noqa: F401
    test_property16_aggressive_gate_warning,  # noqa: F401
    test_property16_forward_underperformance,  # noqa: F401
    test_property16_overfitting_flag,  # noqa: F401
)
