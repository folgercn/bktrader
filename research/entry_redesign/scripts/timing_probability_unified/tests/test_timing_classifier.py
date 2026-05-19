"""Property-based tests for timing_classifier — Property 5 & Property 6."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Optional

from hypothesis import given, settings
from hypothesis import strategies as st

from timing_probability_unified.timing_classifier import (
    get_selected_delay_pnl,
    select_best_depth,
)


# ---------------------------------------------------------------------------
# Mock DelayResult for property testing (avoids importing heavy dependencies)
# ---------------------------------------------------------------------------


@dataclass
class MockDelayResult:
    """Lightweight mock of pre_breakout_timing.delay_simulator.DelayResult."""

    event_id: str
    delay_label: str
    delay_seconds: int
    entry_time: object  # None or timestamp
    entry_price: Optional[float]
    pnl_pct: Optional[float]
    exit_reason: Optional[str]
    exit_time: object  # None or timestamp
    hold_seconds: Optional[float]
    mfe_r: Optional[float]
    mae_r: Optional[float]
    traded: bool


# Feature: timing-probability-unified, Property 5: LOOCV-Based Model Selection
class TestLOOCVModelSelection:
    """Property 5: LOOCV-Based Model Selection.

    For any two LOOCV calendar_sum scores (dt3_score, dt4_score), the selected
    classifier depth SHALL correspond to the higher score. If dt3_score > dt4_score,
    selected_depth == 3; if dt4_score > dt3_score, selected_depth == 4; if equal,
    selected_depth == 3 (prefer simpler model).

    **Validates: Requirements 2.5**
    """

    @settings(max_examples=200)
    @given(
        dt3_score=st.floats(min_value=-1.0, max_value=1.0, allow_nan=False, allow_infinity=False),
        dt4_score=st.floats(min_value=-1.0, max_value=1.0, allow_nan=False, allow_infinity=False),
    )
    def test_loocv_model_selection_property(self, dt3_score: float, dt4_score: float):
        """# Feature: timing-probability-unified, Property 5: LOOCV-Based Model Selection

        **Validates: Requirements 2.5**
        """
        selected_depth = select_best_depth(dt3_score, dt4_score)

        if dt4_score > dt3_score:
            assert selected_depth == 4, (
                f"Expected depth=4 when dt4_score ({dt4_score}) > dt3_score ({dt3_score}), "
                f"got {selected_depth}"
            )
        elif dt3_score > dt4_score:
            assert selected_depth == 3, (
                f"Expected depth=3 when dt3_score ({dt3_score}) > dt4_score ({dt4_score}), "
                f"got {selected_depth}"
            )
        else:
            # Equal scores → prefer simpler model (DT3)
            assert selected_depth == 3, (
                f"Expected depth=3 when scores are equal ({dt3_score} == {dt4_score}), "
                f"got {selected_depth}"
            )


# ---------------------------------------------------------------------------
# Hypothesis strategies for Property 6
# ---------------------------------------------------------------------------

# Strategy for generating a valid prediction
_prediction_strategy = st.sampled_from(["skip", "fast", "slow"])

# Strategy for generating PnL values (realistic range for percentage PnL)
_pnl_strategy = st.floats(min_value=-0.05, max_value=0.05, allow_nan=False, allow_infinity=False)


def _make_mock_delay_result(
    delay_label: str, pnl_pct: float, traded: bool = True
) -> MockDelayResult:
    """Create a MockDelayResult with the given label and PnL."""
    return MockDelayResult(
        event_id="test_event_001",
        delay_label=delay_label,
        delay_seconds=0,
        entry_time=None,
        entry_price=100.0 if traded else None,
        pnl_pct=pnl_pct if traded else None,
        exit_reason="TrailingSL" if traded else None,
        exit_time=None,
        hold_seconds=60.0 if traded else None,
        mfe_r=1.0 if traded else None,
        mae_r=-0.5 if traded else None,
        traded=traded,
    )


# Feature: timing-probability-unified, Property 6: Prediction-to-Delay Mapping
class TestPredictionToDelayMapping:
    """Property 6: Prediction-to-Delay Mapping.

    For any timing prediction and corresponding delay_results:
    - IF prediction == "fast" → selected_delay ∈ {D0, D5} (the one with higher pnl)
    - IF prediction == "slow" → selected_delay ∈ {D10, D15, pullback} (the one with higher pnl)
    - IF prediction == "skip" → position == 0, no delay selected (returns "none")

    **Validates: Requirements 2.6, 4.3**
    """

    @settings(max_examples=200)
    @given(
        prediction=_prediction_strategy,
        d0_pnl=_pnl_strategy,
        d5_pnl=_pnl_strategy,
        d10_pnl=_pnl_strategy,
        d15_pnl=_pnl_strategy,
        pullback_pnl=_pnl_strategy,
    )
    def test_prediction_to_delay_mapping_property(
        self,
        prediction: str,
        d0_pnl: float,
        d5_pnl: float,
        d10_pnl: float,
        d15_pnl: float,
        pullback_pnl: float,
    ):
        """# Feature: timing-probability-unified, Property 6: Prediction-to-Delay Mapping

        **Validates: Requirements 2.6, 4.3**
        """
        # Build mock delay results for all 5 delays
        event_delays = [
            _make_mock_delay_result("D0", d0_pnl),
            _make_mock_delay_result("D5", d5_pnl),
            _make_mock_delay_result("D10", d10_pnl),
            _make_mock_delay_result("D15", d15_pnl),
            _make_mock_delay_result("pullback", pullback_pnl),
        ]

        selected_delay, pnl = get_selected_delay_pnl(prediction, event_delays)

        if prediction == "skip":
            # Skip → no delay selected, pnl = 0
            assert selected_delay == "none", (
                f"Expected 'none' for skip prediction, got '{selected_delay}'"
            )
            assert pnl == 0.0, (
                f"Expected pnl=0.0 for skip prediction, got {pnl}"
            )

        elif prediction == "fast":
            # Fast → selected_delay must be in {D0, D5}
            assert selected_delay in {"D0", "D5"}, (
                f"Expected selected_delay in {{D0, D5}} for fast prediction, "
                f"got '{selected_delay}'"
            )
            # Must be the one with higher PnL
            fast_pnls = {"D0": d0_pnl, "D5": d5_pnl}
            best_fast_pnl = max(fast_pnls.values())
            assert pnl == best_fast_pnl, (
                f"Expected pnl={best_fast_pnl} (best of D0={d0_pnl}, D5={d5_pnl}), "
                f"got {pnl}"
            )

        elif prediction == "slow":
            # Slow → selected_delay must be in {D10, D15, pullback}
            assert selected_delay in {"D10", "D15", "pullback"}, (
                f"Expected selected_delay in {{D10, D15, pullback}} for slow prediction, "
                f"got '{selected_delay}'"
            )
            # Must be the one with higher PnL
            slow_pnls = {"D10": d10_pnl, "D15": d15_pnl, "pullback": pullback_pnl}
            best_slow_pnl = max(slow_pnls.values())
            assert pnl == best_slow_pnl, (
                f"Expected pnl={best_slow_pnl} (best of D10={d10_pnl}, D15={d15_pnl}, "
                f"pullback={pullback_pnl}), got {pnl}"
            )


# ---------------------------------------------------------------------------
# Unit Tests for timing_classifier
# Requirements: 2.3, 2.5, 2.6
# ---------------------------------------------------------------------------

import numpy as np
import pandas as pd
import pytest

from timing_probability_unified.timing_classifier import (
    evaluate_timing_predictions,
    generate_3regime_label_from_pnls,
    get_selected_delay_pnl,
    select_best_depth,
)

# We need DelayResult for constructing mock delay data
import sys
from pathlib import Path

_scripts_dir = Path(__file__).resolve().parents[2]
if str(_scripts_dir) not in sys.path:
    sys.path.insert(0, str(_scripts_dir))

from pre_breakout_timing.delay_simulator import DelayResult  # noqa: E402


def _make_delay_result(
    delay_label: str,
    pnl_pct: float | None,
    traded: bool = True,
    event_id: str = "BTCUSDT_2025-01-15_12:00:00",
) -> DelayResult:
    """Helper to create a mock DelayResult with minimal required fields."""
    return DelayResult(
        event_id=event_id,
        delay_label=delay_label,
        delay_seconds={"D0": 0, "D5": 5, "D10": 10, "D15": 15, "pullback": 20}.get(
            delay_label, 0
        ),
        entry_time=pd.Timestamp("2025-01-15 12:00:05", tz="UTC") if traded else None,
        entry_price=50000.0 if traded else None,
        pnl_pct=pnl_pct,
        exit_reason="TrailingSL" if traded else None,
        exit_time=pd.Timestamp("2025-01-15 12:30:00", tz="UTC") if traded else None,
        hold_seconds=1800.0 if traded else None,
        mfe_r=1.5 if traded else None,
        mae_r=-0.3 if traded else None,
        traded=traded,
    )


def _make_event_delays(
    d0: float, d5: float, d10: float, d15: float, pb: float, event_id: str = "BTCUSDT_2025-01-15_12:00:00"
) -> list[DelayResult]:
    """Create a full set of 5 DelayResult objects with given PnLs."""
    return [
        _make_delay_result("D0", d0, traded=True, event_id=event_id),
        _make_delay_result("D5", d5, traded=True, event_id=event_id),
        _make_delay_result("D10", d10, traded=True, event_id=event_id),
        _make_delay_result("D15", d15, traded=True, event_id=event_id),
        _make_delay_result("pullback", pb, traded=True, event_id=event_id),
    ]


class TestGenerate3RegimeLabelFromPnls:
    """Unit tests for generate_3regime_label_from_pnls — Requirements 2.3."""

    def test_all_skip_all_negative(self):
        """All 5 delay PnLs are negative → label = 'skip'."""
        label = generate_3regime_label_from_pnls(-0.01, -0.02, -0.03, -0.04, -0.005)
        assert label == "skip"

    def test_all_skip_all_zero(self):
        """All PnLs are exactly zero → fast_pnl=0 and slow_pnl=0.
        Since fast_pnl >= slow_pnl (both 0), label = 'fast'."""
        # Note: 0 is NOT < 0, so skip condition fails.
        # fast_pnl = max(0, 0) = 0, slow_pnl = max(0, 0, 0) = 0
        # fast_pnl >= slow_pnl → "fast"
        label = generate_3regime_label_from_pnls(0.0, 0.0, 0.0, 0.0, 0.0)
        assert label == "fast"

    def test_all_fast_d0_highest(self):
        """D0 has highest positive PnL among fast group → label = 'fast'."""
        label = generate_3regime_label_from_pnls(0.02, 0.01, -0.01, -0.02, -0.01)
        assert label == "fast"

    def test_all_fast_d5_highest(self):
        """D5 has highest positive PnL among fast group → label = 'fast'."""
        label = generate_3regime_label_from_pnls(0.005, 0.03, -0.01, -0.005, -0.02)
        assert label == "fast"

    def test_all_slow_d10_highest(self):
        """D10 has highest positive PnL, clearly above fast → label = 'slow'."""
        label = generate_3regime_label_from_pnls(-0.01, -0.005, 0.05, 0.01, 0.02)
        assert label == "slow"

    def test_all_slow_d15_highest(self):
        """D15 has highest positive PnL, clearly above fast → label = 'slow'."""
        label = generate_3regime_label_from_pnls(-0.01, -0.005, 0.01, 0.05, 0.02)
        assert label == "slow"

    def test_all_slow_pullback_highest(self):
        """Pullback has highest positive PnL, clearly above fast → label = 'slow'."""
        label = generate_3regime_label_from_pnls(-0.01, -0.005, 0.01, 0.02, 0.05)
        assert label == "slow"

    def test_tolerance_boundary_exactly_5bps(self):
        """slow_pnl - fast_pnl = exactly 5bps (0.0005) → 'fast' (within tolerance).

        fast_pnl = 0.01, slow_pnl = 0.0105
        diff = 0.0105 - 0.01 = 0.0005 = 5bps
        Since diff < tolerance (not <=), this is within tolerance → 'fast'.
        """
        # fast_pnl = max(0.01, 0.005) = 0.01
        # slow_pnl = max(0.0105, 0.005, 0.005) = 0.0105
        # diff = 0.0105 - 0.01 = 0.0005 = 5bps exactly
        # tolerance = 5/10000 = 0.0005
        # (slow_pnl - fast_pnl) < tolerance → 0.0005 < 0.0005 is FALSE
        # So this goes to "slow"
        # Actually: the condition is (slow_pnl - fast_pnl) < tolerance
        # 0.0005 < 0.0005 is False, so it falls through to "slow"
        label = generate_3regime_label_from_pnls(0.01, 0.005, 0.0105, 0.005, 0.005)
        assert label == "slow"

    def test_tolerance_boundary_just_below_5bps(self):
        """slow_pnl - fast_pnl = 4bps (< 5bps) → 'fast' (within tolerance).

        fast_pnl = 0.01, slow_pnl = 0.0104
        diff = 0.0104 - 0.01 = 0.0004 = 4bps < 5bps tolerance → 'fast'
        """
        label = generate_3regime_label_from_pnls(0.01, 0.005, 0.0104, 0.005, 0.005)
        assert label == "fast"

    def test_tolerance_boundary_just_above_5bps(self):
        """slow_pnl - fast_pnl = 6bps (> 5bps) → 'slow' (exceeds tolerance).

        fast_pnl = 0.01, slow_pnl = 0.0106
        diff = 0.0106 - 0.01 = 0.0006 = 6bps > 5bps tolerance → 'slow'
        """
        label = generate_3regime_label_from_pnls(0.01, 0.005, 0.0106, 0.005, 0.005)
        assert label == "slow"

    def test_fast_pnl_equals_slow_pnl(self):
        """fast_pnl == slow_pnl → 'fast' (fast_pnl >= slow_pnl)."""
        label = generate_3regime_label_from_pnls(0.02, 0.01, 0.02, 0.01, 0.015)
        assert label == "fast"


class TestGetSelectedDelayPnl:
    """Unit tests for get_selected_delay_pnl — Requirements 2.6."""

    def test_skip_returns_none_zero(self):
        """Prediction 'skip' → returns ('none', 0.0)."""
        delays = _make_event_delays(0.01, 0.02, 0.03, 0.04, 0.05)
        label, pnl = get_selected_delay_pnl("skip", delays)
        assert label == "none"
        assert pnl == 0.0

    def test_fast_returns_best_of_d0_d5(self):
        """Prediction 'fast' → returns best of D0/D5."""
        delays = _make_event_delays(0.03, 0.01, 0.05, 0.04, 0.02)
        label, pnl = get_selected_delay_pnl("fast", delays)
        assert label == "D0"
        assert pnl == 0.03

    def test_fast_returns_d5_when_higher(self):
        """Prediction 'fast' → returns D5 when D5 > D0."""
        delays = _make_event_delays(0.01, 0.04, 0.05, 0.03, 0.02)
        label, pnl = get_selected_delay_pnl("fast", delays)
        assert label == "D5"
        assert pnl == 0.04

    def test_slow_returns_best_of_d10_d15_pullback(self):
        """Prediction 'slow' → returns best of D10/D15/pullback."""
        delays = _make_event_delays(0.01, 0.02, 0.03, 0.05, 0.04)
        label, pnl = get_selected_delay_pnl("slow", delays)
        assert label == "D15"
        assert pnl == 0.05

    def test_slow_returns_pullback_when_highest(self):
        """Prediction 'slow' → returns pullback when it's highest."""
        delays = _make_event_delays(0.01, 0.02, 0.03, 0.04, 0.06)
        label, pnl = get_selected_delay_pnl("slow", delays)
        assert label == "pullback"
        assert pnl == 0.06

    def test_slow_returns_d10_when_highest(self):
        """Prediction 'slow' → returns D10 when it's highest."""
        delays = _make_event_delays(0.01, 0.02, 0.07, 0.04, 0.05)
        label, pnl = get_selected_delay_pnl("slow", delays)
        assert label == "D10"
        assert pnl == 0.07

    def test_fast_with_untradeable_delay(self):
        """Prediction 'fast' with one untradeable delay → uses the traded one."""
        delays = [
            _make_delay_result("D0", None, traded=False),
            _make_delay_result("D5", 0.02, traded=True),
            _make_delay_result("D10", 0.05, traded=True),
            _make_delay_result("D15", 0.04, traded=True),
            _make_delay_result("pullback", 0.03, traded=True),
        ]
        label, pnl = get_selected_delay_pnl("fast", delays)
        assert label == "D5"
        assert pnl == 0.02


class TestEvaluateTimingPredictions:
    """Unit tests for evaluate_timing_predictions — Requirements 2.6."""

    def test_all_skip_returns_zero(self):
        """All predictions are 'skip' → calendar_sum = 0.0."""
        predictions = np.array(["skip", "skip", "skip"])
        delay_results = [
            _make_event_delays(0.01, 0.02, 0.03, 0.04, 0.05, "BTCUSDT_2025-01-15_12:00:00"),
            _make_event_delays(0.01, 0.02, 0.03, 0.04, 0.05, "ETHUSDT_2025-02-15_12:00:00"),
            _make_event_delays(0.01, 0.02, 0.03, 0.04, 0.05, "BTCUSDT_2025-03-15_12:00:00"),
        ]
        events = pd.DataFrame(
            {
                "symbol": ["BTCUSDT", "ETHUSDT", "BTCUSDT"],
                "touch_time": pd.to_datetime(
                    ["2025-01-15", "2025-02-15", "2025-03-15"]
                ).tz_localize("UTC"),
            }
        )
        result = evaluate_timing_predictions(predictions, delay_results, events)
        assert result == 0.0


class TestSelectBestDepth:
    """Unit tests for select_best_depth — Requirements 2.5."""

    def test_dt3_wins(self):
        """DT3 score > DT4 score → depth = 3."""
        assert select_best_depth(0.10, 0.05) == 3

    def test_dt4_wins(self):
        """DT4 score > DT3 score → depth = 4."""
        assert select_best_depth(0.05, 0.10) == 4

    def test_tie_prefers_dt3(self):
        """Equal scores → depth = 3 (prefer simpler model)."""
        assert select_best_depth(0.08, 0.08) == 3

    def test_both_negative_dt3_less_negative(self):
        """Both negative, DT3 less negative → depth = 3."""
        assert select_best_depth(-0.02, -0.05) == 3

    def test_both_negative_dt4_less_negative(self):
        """Both negative, DT4 less negative → depth = 4."""
        assert select_best_depth(-0.05, -0.02) == 4
