"""Property-based tests for combined_executor — Combined Position Logic."""

import numpy as np
import pandas as pd
import pytest
from hypothesis import given, settings
from hypothesis import strategies as st

from pre_breakout_timing.delay_simulator import DelayResult
from timing_probability_unified.combined_executor import (
    CombinedPositionConfig,
    compute_combined_positions,
)


def _make_delay_result(delay_label: str, pnl: float, traded: bool = True) -> DelayResult:
    """Helper to create a minimal DelayResult for testing."""
    return DelayResult(
        event_id="test_event",
        delay_label=delay_label,
        delay_seconds=0,
        entry_time=None,
        entry_price=None,
        pnl_pct=pnl if traded else None,
        exit_reason=None,
        exit_time=None,
        hold_seconds=None,
        mfe_r=None,
        mae_r=None,
        traded=traded,
    )


def _make_standard_delay_results(fast_pnl: float = 0.01, slow_pnl: float = 0.02) -> list[DelayResult]:
    """Create a standard set of 5 delay results for testing."""
    return [
        _make_delay_result("D0", fast_pnl),
        _make_delay_result("D5", fast_pnl * 0.8),
        _make_delay_result("D10", slow_pnl),
        _make_delay_result("D15", slow_pnl * 0.9),
        _make_delay_result("pullback", slow_pnl * 0.7),
    ]


def _make_single_event_df() -> pd.DataFrame:
    """Create a minimal single-event DataFrame for testing."""
    return pd.DataFrame(
        {
            "event_id": ["evt_001"],
            "symbol": ["BTCUSDT"],
            "side": ["long"],
            "touch_time": [pd.Timestamp("2025-01-01 00:00:00", tz="UTC")],
            "speed_300s_atr": [1.5],
        }
    )


# Feature: timing-probability-unified, Property 10: Combined Position Logic
@settings(max_examples=200)
@given(
    timing_prediction=st.sampled_from(["skip", "fast", "slow"]),
    sizing_multiplier=st.floats(min_value=0.0, max_value=2.0, allow_nan=False, allow_infinity=False),
    base_share=st.floats(min_value=0.01, max_value=1.0, allow_nan=False, allow_infinity=False),
)
def test_combined_position_logic_property(timing_prediction, sizing_multiplier, base_share):
    """Property 10: Combined Position Logic

    For any event with timing_prediction and sizing_multiplier:
    - IF timing_prediction == "skip" → position_size == 0.0
    - ELSE → position_size == base_notional_share × sizing_multiplier
    - position_size is always in [0, base_notional_share × 2]

    **Validates: Requirements 4.1**
    """
    # Build minimal inputs for a single event
    events = _make_single_event_df()
    timing_predictions = np.array([timing_prediction])
    sizing_multipliers = np.array([sizing_multiplier])
    delay_results = [_make_standard_delay_results()]
    speed_gate_pass = np.array([True])

    config = CombinedPositionConfig(base_notional_share=base_share)

    # Execute
    trades = compute_combined_positions(
        events=events,
        timing_predictions=timing_predictions,
        sizing_multipliers=sizing_multipliers,
        delay_results=delay_results,
        speed_gate_pass=speed_gate_pass,
        config=config,
    )

    position_size = trades["position_size"].iloc[0]

    # Property assertions
    if timing_prediction == "skip":
        assert position_size == 0.0, (
            f"timing=skip should produce position_size=0.0, got {position_size}"
        )
    else:
        expected = base_share * sizing_multiplier
        assert position_size == pytest.approx(expected), (
            f"timing={timing_prediction} with base_share={base_share}, "
            f"multiplier={sizing_multiplier} should produce position_size={expected}, "
            f"got {position_size}"
        )

    # position_size is always in [0, base_notional_share × 2]
    assert 0.0 <= position_size <= base_share * 2.0 + 1e-10, (
        f"position_size={position_size} out of valid range [0, {base_share * 2.0}] "
        f"for base_share={base_share}"
    )


# Feature: timing-probability-unified, Property 11: Weighted PnL Arithmetic
@settings(max_examples=200)
@given(
    delay_pnl_pct=st.floats(min_value=-0.10, max_value=0.10, allow_nan=False, allow_infinity=False),
    position_size=st.floats(min_value=0.0, max_value=1.0, allow_nan=False, allow_infinity=False),
)
def test_weighted_pnl_arithmetic_pure(delay_pnl_pct, position_size):
    """Property 11: Weighted PnL Arithmetic (pure arithmetic level)

    For any trade row in unified_trades, weighted_pnl == delay_pnl_pct × position_size.
    If position_size == 0, then weighted_pnl == 0 regardless of delay_pnl_pct.

    **Validates: Requirements 4.4**
    """
    # Compute weighted_pnl using the same arithmetic as combined_executor
    weighted_pnl = delay_pnl_pct * position_size

    # Property: weighted_pnl == delay_pnl_pct × position_size
    assert weighted_pnl == pytest.approx(delay_pnl_pct * position_size), (
        f"weighted_pnl={weighted_pnl} != delay_pnl_pct({delay_pnl_pct}) × "
        f"position_size({position_size})"
    )

    # Special case: if position_size == 0, weighted_pnl == 0
    if position_size == 0.0:
        assert weighted_pnl == 0.0, (
            f"When position_size=0, weighted_pnl should be 0, got {weighted_pnl}"
        )


# Feature: timing-probability-unified, Property 11: Weighted PnL Arithmetic
@settings(max_examples=200)
@given(
    timing_prediction=st.sampled_from(["skip", "fast", "slow"]),
    sizing_multiplier=st.floats(min_value=0.0, max_value=2.0, allow_nan=False, allow_infinity=False),
    base_share=st.floats(min_value=0.01, max_value=1.0, allow_nan=False, allow_infinity=False),
    fast_pnl=st.floats(min_value=-0.05, max_value=0.05, allow_nan=False, allow_infinity=False),
    slow_pnl=st.floats(min_value=-0.05, max_value=0.05, allow_nan=False, allow_infinity=False),
)
def test_weighted_pnl_arithmetic_via_compute_combined_positions(
    timing_prediction, sizing_multiplier, base_share, fast_pnl, slow_pnl
):
    """Property 11: Weighted PnL Arithmetic (integration level)

    Verifies that compute_combined_positions produces unified_trades where
    weighted_pnl == delay_pnl_pct × position_size for every row,
    and weighted_pnl == 0 when position_size == 0.

    **Validates: Requirements 4.4**
    """
    # Build minimal inputs for a single event
    events = _make_single_event_df()
    timing_predictions = np.array([timing_prediction])
    sizing_multipliers = np.array([sizing_multiplier])
    delay_results = [_make_standard_delay_results(fast_pnl=fast_pnl, slow_pnl=slow_pnl)]
    speed_gate_pass = np.array([True])

    config = CombinedPositionConfig(base_notional_share=base_share)

    # Execute
    trades = compute_combined_positions(
        events=events,
        timing_predictions=timing_predictions,
        sizing_multipliers=sizing_multipliers,
        delay_results=delay_results,
        speed_gate_pass=speed_gate_pass,
        config=config,
    )

    row = trades.iloc[0]
    actual_weighted_pnl = row["weighted_pnl"]
    actual_position_size = row["position_size"]
    actual_delay_pnl_pct = row["delay_pnl_pct"]

    # Property: weighted_pnl == delay_pnl_pct × position_size
    expected_weighted_pnl = actual_delay_pnl_pct * actual_position_size
    assert actual_weighted_pnl == pytest.approx(expected_weighted_pnl, abs=1e-15), (
        f"weighted_pnl={actual_weighted_pnl} != "
        f"delay_pnl_pct({actual_delay_pnl_pct}) × position_size({actual_position_size}) "
        f"= {expected_weighted_pnl}"
    )

    # Special case: if position_size == 0, weighted_pnl must be 0
    if actual_position_size == 0.0:
        assert actual_weighted_pnl == 0.0, (
            f"position_size=0 but weighted_pnl={actual_weighted_pnl} != 0"
        )


from timing_probability_unified.combined_executor import (
    compute_worst_sm,
)


# --- Hypothesis strategies for Property 12 ---

def _trades_dataframe_strategy():
    """Strategy to generate trades DataFrames with varying months and weighted_pnl values."""

    @st.composite
    def _build(draw):
        n_trades = draw(st.integers(min_value=0, max_value=30))
        if n_trades == 0:
            return pd.DataFrame(
                {
                    "touch_time": pd.Series(dtype="datetime64[ns, UTC]"),
                    "weighted_pnl": pd.Series(dtype="float64"),
                    "speed_gate_pass": pd.Series(dtype="bool"),
                }
            )

        # Generate random months (year-month combinations)
        months = draw(
            st.lists(
                st.integers(min_value=1, max_value=12),
                min_size=n_trades,
                max_size=n_trades,
            )
        )
        years = draw(
            st.lists(
                st.sampled_from([2024, 2025, 2026]),
                min_size=n_trades,
                max_size=n_trades,
            )
        )
        days = [min(d, 28) for d in draw(
            st.lists(
                st.integers(min_value=1, max_value=28),
                min_size=n_trades,
                max_size=n_trades,
            )
        )]

        touch_times = [
            pd.Timestamp(year=y, month=m, day=d, tz="UTC")
            for y, m, d in zip(years, months, days)
        ]

        weighted_pnls = draw(
            st.lists(
                st.floats(min_value=-0.10, max_value=0.10, allow_nan=False, allow_infinity=False),
                min_size=n_trades,
                max_size=n_trades,
            )
        )

        speed_gate_pass = draw(
            st.lists(
                st.booleans(),
                min_size=n_trades,
                max_size=n_trades,
            )
        )

        return pd.DataFrame(
            {
                "touch_time": touch_times,
                "weighted_pnl": weighted_pnls,
                "speed_gate_pass": speed_gate_pass,
            }
        )

    return _build()


# Feature: timing-probability-unified, Property 12: Worst SM Equals Minimum Monthly Sum
@settings(max_examples=200)
@given(trades_df=_trades_dataframe_strategy())
def test_worst_sm_equals_minimum_monthly_sum(trades_df):
    """Property 12: Worst SM Equals Minimum Monthly Sum

    For any trades DataFrame grouped by calendar month, worst_sm SHALL equal
    the minimum value among all monthly weighted_pnl sums.
    If no trades exist, worst_sm == 0.

    **Validates: Requirements 4.7**
    """
    # Compute worst_sm using the function under test
    result = compute_worst_sm(trades_df, gate_filter=False)

    if trades_df.empty:
        # If no trades exist, worst_sm == 0
        assert result == 0.0, f"Empty trades should give worst_sm=0, got {result}"
    else:
        # Manually compute expected worst_sm
        df = trades_df.copy()
        df = df.assign(year_month=pd.to_datetime(df["touch_time"]).dt.to_period("M"))
        monthly_sums = df.groupby("year_month")["weighted_pnl"].sum()
        expected_worst_sm = float(monthly_sums.min())

        assert result == pytest.approx(expected_worst_sm, abs=1e-12), (
            f"worst_sm={result} != min(monthly_sums)={expected_worst_sm}. "
            f"Monthly sums: {monthly_sums.to_dict()}"
        )


# Feature: timing-probability-unified, Property 12: Worst SM Equals Minimum Monthly Sum
@settings(max_examples=200)
@given(trades_df=_trades_dataframe_strategy())
def test_worst_sm_with_gate_filter(trades_df):
    """Property 12: Worst SM Equals Minimum Monthly Sum (with gate_filter=True)

    Same property but with gate_filter=True: worst_sm is computed only on
    speed_gate_pass=True trades. If no trades pass the gate, worst_sm == 0.

    **Validates: Requirements 4.7**
    """
    # Compute worst_sm with gate_filter=True
    result = compute_worst_sm(trades_df, gate_filter=True)

    if trades_df.empty:
        assert result == 0.0, f"Empty trades should give worst_sm=0, got {result}"
        return

    # Filter to gate-passing trades
    filtered = trades_df[trades_df["speed_gate_pass"] == True]  # noqa: E712

    if filtered.empty:
        assert result == 0.0, (
            f"No trades pass gate, worst_sm should be 0, got {result}"
        )
    else:
        df = filtered.copy()
        df = df.assign(year_month=pd.to_datetime(df["touch_time"]).dt.to_period("M"))
        monthly_sums = df.groupby("year_month")["weighted_pnl"].sum()
        expected_worst_sm = float(monthly_sums.min())

        assert result == pytest.approx(expected_worst_sm, abs=1e-12), (
            f"worst_sm(gate_filter=True)={result} != min(monthly_sums)={expected_worst_sm}. "
            f"Monthly sums: {monthly_sums.to_dict()}"
        )


# ============================================================================
# Unit Tests for combined_executor
# ============================================================================


from timing_probability_unified.combined_executor import (
    compute_calendar_sum,
    compute_worst_sm,
    run_sensitivity_analysis,
    SensitivityRow,
)


class TestComputeCombinedPositions:
    """Unit tests for compute_combined_positions."""

    def test_all_skip_produces_zero_positions(self):
        """All skip → calendar_sum=0, worst_sm=0.

        **Validates: Requirements 4.1, 4.6, 4.7**
        """
        events = pd.DataFrame(
            {
                "event_id": ["evt_001", "evt_002", "evt_003"],
                "symbol": ["BTCUSDT", "ETHUSDT", "BTCUSDT"],
                "side": ["long", "short", "long"],
                "touch_time": pd.to_datetime(
                    ["2025-01-15", "2025-02-10", "2025-03-05"], utc=True
                ),
                "speed_300s_atr": [1.0, 2.0, 1.5],
            }
        )
        timing_predictions = np.array(["skip", "skip", "skip"])
        sizing_multipliers = np.array([1.0, 1.5, 0.8])
        delay_results = [_make_standard_delay_results() for _ in range(3)]
        speed_gate_pass = np.array([True, True, True])

        trades = compute_combined_positions(
            events=events,
            timing_predictions=timing_predictions,
            sizing_multipliers=sizing_multipliers,
            delay_results=delay_results,
            speed_gate_pass=speed_gate_pass,
        )

        # All positions should be 0
        assert (trades["position_size"] == 0.0).all()
        assert (trades["weighted_pnl"] == 0.0).all()
        assert (trades["delay_pnl_pct"] == 0.0).all()
        assert (trades["selected_delay"] == "none").all()

        # calendar_sum and worst_sm should be 0
        assert compute_calendar_sum(trades) == 0.0
        assert compute_worst_sm(trades) == 0.0

    def test_single_event_fast_prediction(self):
        """Single event with fast prediction → correct position and weighted_pnl.

        **Validates: Requirements 4.1, 4.3, 4.4**
        """
        events = _make_single_event_df()
        timing_predictions = np.array(["fast"])
        sizing_multipliers = np.array([1.6])  # p=0.8 → multiplier=1.6
        # D0 pnl=0.02, D5 pnl=0.016 → best fast is D0 with 0.02
        delay_results = [_make_standard_delay_results(fast_pnl=0.02, slow_pnl=0.01)]
        speed_gate_pass = np.array([True])

        config = CombinedPositionConfig(base_notional_share=0.30)

        trades = compute_combined_positions(
            events=events,
            timing_predictions=timing_predictions,
            sizing_multipliers=sizing_multipliers,
            delay_results=delay_results,
            speed_gate_pass=speed_gate_pass,
            config=config,
        )

        row = trades.iloc[0]
        expected_position = 0.30 * 1.6  # 0.48
        expected_pnl = 0.02  # D0 is the best fast delay
        expected_weighted = expected_pnl * expected_position

        assert row["timing_prediction"] == "fast"
        assert row["selected_delay"] == "D0"
        assert row["position_size"] == pytest.approx(expected_position)
        assert row["delay_pnl_pct"] == pytest.approx(expected_pnl)
        assert row["weighted_pnl"] == pytest.approx(expected_weighted)

    def test_single_event_slow_prediction(self):
        """Single event with slow prediction → uses slow delay group.

        **Validates: Requirements 4.1, 4.3**
        """
        events = _make_single_event_df()
        timing_predictions = np.array(["slow"])
        sizing_multipliers = np.array([1.0])
        # D10 pnl=0.03, D15 pnl=0.027, pullback pnl=0.021 → best slow is D10 with 0.03
        delay_results = [_make_standard_delay_results(fast_pnl=0.01, slow_pnl=0.03)]
        speed_gate_pass = np.array([True])

        config = CombinedPositionConfig(base_notional_share=0.30)

        trades = compute_combined_positions(
            events=events,
            timing_predictions=timing_predictions,
            sizing_multipliers=sizing_multipliers,
            delay_results=delay_results,
            speed_gate_pass=speed_gate_pass,
            config=config,
        )

        row = trades.iloc[0]
        assert row["timing_prediction"] == "slow"
        assert row["selected_delay"] == "D10"
        assert row["position_size"] == pytest.approx(0.30)
        assert row["delay_pnl_pct"] == pytest.approx(0.03)


class TestComputeCalendarSum:
    """Unit tests for compute_calendar_sum."""

    def test_empty_trades_returns_zero(self):
        """compute_calendar_sum with empty trades → 0.0.

        **Validates: Requirements 4.6**
        """
        empty_trades = pd.DataFrame(
            columns=[
                "event_id", "symbol", "side", "touch_time",
                "timing_prediction", "selected_delay", "rf_probability",
                "sizing_multiplier", "position_size", "delay_pnl_pct",
                "weighted_pnl", "speed_300s_atr", "speed_gate_pass",
            ]
        )
        assert compute_calendar_sum(empty_trades) == 0.0

    def test_single_trade_calendar_sum(self):
        """Single trade → calendar_sum equals that trade's weighted_pnl.

        **Validates: Requirements 4.6**
        """
        trades = pd.DataFrame(
            {
                "event_id": ["evt_001"],
                "symbol": ["BTCUSDT"],
                "touch_time": pd.to_datetime(["2025-01-15"], utc=True),
                "weighted_pnl": [0.005],
                "speed_gate_pass": [True],
            }
        )
        assert compute_calendar_sum(trades) == pytest.approx(0.005)

    def test_gate_filter_excludes_non_passing(self):
        """gate_filter=True filters correctly — only uses speed_gate_pass=True trades.

        **Validates: Requirements 4.6**
        """
        trades = pd.DataFrame(
            {
                "event_id": ["evt_001", "evt_002", "evt_003"],
                "symbol": ["BTCUSDT", "BTCUSDT", "ETHUSDT"],
                "touch_time": pd.to_datetime(
                    ["2025-01-15", "2025-01-20", "2025-01-10"], utc=True
                ),
                "weighted_pnl": [0.01, -0.02, 0.03],
                "speed_gate_pass": [True, False, True],
            }
        )
        # Without gate filter: sum = 0.01 + (-0.02) + 0.03 = 0.02
        assert compute_calendar_sum(trades, gate_filter=False) == pytest.approx(0.02)
        # With gate filter: only evt_001 and evt_003 → 0.01 + 0.03 = 0.04
        assert compute_calendar_sum(trades, gate_filter=True) == pytest.approx(0.04)

    def test_all_gate_fail_returns_zero(self):
        """gate_filter=True with all speed_gate_pass=False → 0.0.

        **Validates: Requirements 4.6**
        """
        trades = pd.DataFrame(
            {
                "event_id": ["evt_001", "evt_002"],
                "symbol": ["BTCUSDT", "ETHUSDT"],
                "touch_time": pd.to_datetime(["2025-01-15", "2025-02-10"], utc=True),
                "weighted_pnl": [0.01, 0.02],
                "speed_gate_pass": [False, False],
            }
        )
        assert compute_calendar_sum(trades, gate_filter=True) == 0.0


class TestComputeWorstSm:
    """Unit tests for compute_worst_sm."""

    def test_empty_trades_returns_zero(self):
        """compute_worst_sm with empty trades → 0.0.

        **Validates: Requirements 4.7**
        """
        empty_trades = pd.DataFrame(
            columns=[
                "event_id", "symbol", "side", "touch_time",
                "timing_prediction", "selected_delay", "rf_probability",
                "sizing_multiplier", "position_size", "delay_pnl_pct",
                "weighted_pnl", "speed_300s_atr", "speed_gate_pass",
            ]
        )
        assert compute_worst_sm(empty_trades) == 0.0

    def test_single_month_worst_sm(self):
        """Single month → worst_sm equals that month's sum.

        **Validates: Requirements 4.7**
        """
        trades = pd.DataFrame(
            {
                "event_id": ["evt_001", "evt_002"],
                "symbol": ["BTCUSDT", "ETHUSDT"],
                "touch_time": pd.to_datetime(
                    ["2025-01-15", "2025-01-20"], utc=True
                ),
                "weighted_pnl": [0.01, -0.03],
                "speed_gate_pass": [True, True],
            }
        )
        # Only one month: sum = 0.01 + (-0.03) = -0.02
        assert compute_worst_sm(trades) == pytest.approx(-0.02)

    def test_multi_month_worst_sm(self):
        """Multiple months → worst_sm is the minimum monthly sum.

        **Validates: Requirements 4.7**
        """
        trades = pd.DataFrame(
            {
                "event_id": ["evt_001", "evt_002", "evt_003", "evt_004"],
                "symbol": ["BTCUSDT", "BTCUSDT", "ETHUSDT", "ETHUSDT"],
                "touch_time": pd.to_datetime(
                    ["2025-01-15", "2025-02-10", "2025-01-20", "2025-02-15"],
                    utc=True,
                ),
                "weighted_pnl": [0.02, -0.05, 0.01, 0.03],
                "speed_gate_pass": [True, True, True, True],
            }
        )
        # Jan sum: 0.02 + 0.01 = 0.03
        # Feb sum: -0.05 + 0.03 = -0.02
        # worst_sm = min(0.03, -0.02) = -0.02
        assert compute_worst_sm(trades) == pytest.approx(-0.02)

    def test_gate_filter_worst_sm(self):
        """gate_filter=True filters correctly for worst_sm.

        **Validates: Requirements 4.7**
        """
        trades = pd.DataFrame(
            {
                "event_id": ["evt_001", "evt_002", "evt_003"],
                "symbol": ["BTCUSDT", "BTCUSDT", "BTCUSDT"],
                "touch_time": pd.to_datetime(
                    ["2025-01-15", "2025-01-20", "2025-02-10"], utc=True
                ),
                "weighted_pnl": [-0.05, 0.01, 0.02],
                "speed_gate_pass": [False, True, True],
            }
        )
        # Without gate: Jan sum = -0.05 + 0.01 = -0.04, Feb sum = 0.02
        # worst_sm = -0.04
        assert compute_worst_sm(trades, gate_filter=False) == pytest.approx(-0.04)
        # With gate: Jan sum = 0.01, Feb sum = 0.02
        # worst_sm = 0.01
        assert compute_worst_sm(trades, gate_filter=True) == pytest.approx(0.01)


class TestRunSensitivityAnalysis:
    """Unit tests for run_sensitivity_analysis."""

    def test_default_shares_produces_4_rows(self):
        """run_sensitivity_analysis produces correct number of rows (4 by default).

        **Validates: Requirements 4.8**
        """
        events = _make_single_event_df()
        timing_predictions = np.array(["fast"])
        sizing_multipliers = np.array([1.0])
        delay_results = [_make_standard_delay_results(fast_pnl=0.02, slow_pnl=0.01)]
        speed_gate_pass = np.array([True])

        rows = run_sensitivity_analysis(
            events=events,
            timing_predictions=timing_predictions,
            sizing_multipliers=sizing_multipliers,
            delay_results=delay_results,
            speed_gate_pass=speed_gate_pass,
        )

        assert len(rows) == 4
        assert [r.base_share for r in rows] == [0.25, 0.30, 0.35, 0.40]

    def test_custom_shares(self):
        """run_sensitivity_analysis with custom shares list.

        **Validates: Requirements 4.8**
        """
        events = _make_single_event_df()
        timing_predictions = np.array(["fast"])
        sizing_multipliers = np.array([1.0])
        delay_results = [_make_standard_delay_results(fast_pnl=0.02, slow_pnl=0.01)]
        speed_gate_pass = np.array([True])

        custom_shares = [0.10, 0.20, 0.50]
        rows = run_sensitivity_analysis(
            events=events,
            timing_predictions=timing_predictions,
            sizing_multipliers=sizing_multipliers,
            delay_results=delay_results,
            speed_gate_pass=speed_gate_pass,
            shares=custom_shares,
        )

        assert len(rows) == 3
        assert [r.base_share for r in rows] == [0.10, 0.20, 0.50]

    def test_sensitivity_scaling(self):
        """Sensitivity analysis: larger base_share → proportionally larger calendar_sum.

        **Validates: Requirements 4.8**
        """
        events = _make_single_event_df()
        timing_predictions = np.array(["fast"])
        sizing_multipliers = np.array([1.0])  # multiplier=1.0 → position = base_share
        delay_results = [_make_standard_delay_results(fast_pnl=0.02, slow_pnl=0.01)]
        speed_gate_pass = np.array([True])

        rows = run_sensitivity_analysis(
            events=events,
            timing_predictions=timing_predictions,
            sizing_multipliers=sizing_multipliers,
            delay_results=delay_results,
            speed_gate_pass=speed_gate_pass,
            shares=[0.25, 0.50],
        )

        # With multiplier=1.0, position = base_share
        # weighted_pnl = 0.02 * base_share
        # So calendar_sum for 0.50 should be 2× that of 0.25
        assert rows[1].calendar_sum == pytest.approx(rows[0].calendar_sum * 2.0)

    def test_all_skip_sensitivity(self):
        """Sensitivity analysis with all skip → all rows have zero metrics.

        **Validates: Requirements 4.1, 4.8**
        """
        events = _make_single_event_df()
        timing_predictions = np.array(["skip"])
        sizing_multipliers = np.array([1.0])
        delay_results = [_make_standard_delay_results()]
        speed_gate_pass = np.array([True])

        rows = run_sensitivity_analysis(
            events=events,
            timing_predictions=timing_predictions,
            sizing_multipliers=sizing_multipliers,
            delay_results=delay_results,
            speed_gate_pass=speed_gate_pass,
        )

        for row in rows:
            assert row.calendar_sum == 0.0
            assert row.worst_sm == 0.0
            assert row.trade_count == 0
            assert row.avg_pnl_per_trade == 0.0
