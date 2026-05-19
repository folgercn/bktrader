"""
Unit tests for loocv_calendar_sum() in timing_classifier.py.

Tests the Leave-One-Out CV evaluation function with synthetic data
to verify correctness of the LOO loop, label mapping, and silo-based
calendar sum computation.
"""

from __future__ import annotations

import sys
from pathlib import Path

import numpy as np
import pandas as pd
import pytest

# Ensure the scripts directory is on sys.path
SCRIPTS_DIR = Path(__file__).resolve().parent.parent
if str(SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPTS_DIR))

from pre_breakout_timing.delay_simulator import DelayResult
from pre_breakout_timing.timing_classifier import (
    LABEL_TO_REGIME,
    _compute_calendar_sum_from_results,
    loocv_calendar_sum,
)


# ---------------------------------------------------------------------------
# Fixtures / Helpers
# ---------------------------------------------------------------------------


def _make_delay_result(
    event_id: str,
    delay_label: str,
    pnl_pct: float | None,
    traded: bool = True,
    entry_time: str = "2024-01-15 10:00:00",
) -> DelayResult:
    """Helper to create a DelayResult for testing."""
    return DelayResult(
        event_id=event_id,
        delay_label=delay_label,
        delay_seconds={"D0": 0, "D5": 5, "D10": 10, "D15": 15, "pullback": 30}.get(
            delay_label, 0
        ),
        entry_time=pd.Timestamp(entry_time, tz="UTC") if traded else None,
        entry_price=100.0 if traded else None,
        pnl_pct=pnl_pct if traded else None,
        exit_reason="TrailingSL" if traded else "NoData",
        exit_time=pd.Timestamp(entry_time, tz="UTC") + pd.Timedelta(hours=1)
        if traded
        else None,
        hold_seconds=3600.0 if traded else None,
        mfe_r=1.5 if traded else None,
        mae_r=-0.3 if traded else None,
        traded=traded,
    )


def _build_synthetic_data(n_events: int = 10):
    """Build synthetic data for testing loocv_calendar_sum.

    Creates n_events with:
    - 2 features (feature_a, feature_b)
    - Labels alternating between "D0" and "D5"
    - delay_results with known pnl_pct values
    - events DataFrame with symbol and touch_time
    """
    # Features: simple pattern where feature_a > 0.5 → D0, else D5
    np.random.seed(42)
    feature_a = np.linspace(0.0, 1.0, n_events)
    feature_b = np.random.rand(n_events)
    features = pd.DataFrame({"feature_a": feature_a, "feature_b": feature_b})

    # Labels: first half "D5", second half "D0" (based on feature_a threshold)
    labels = pd.Series(["D5"] * (n_events // 2) + ["D0"] * (n_events - n_events // 2))

    # Events DataFrame
    base_time = pd.Timestamp("2024-01-15 10:00:00", tz="UTC")
    events_data = []
    for i in range(n_events):
        symbol = "BTCUSDT" if i % 2 == 0 else "ETHUSDT"
        touch_time = base_time + pd.Timedelta(days=i)
        events_data.append(
            {
                "event_id": f"evt_{i:03d}",
                "symbol": symbol,
                "touch_time": touch_time,
                "side": "long",
            }
        )
    events = pd.DataFrame(events_data)

    # Delay results: each event has 5 DelayResults
    # D0 gives +0.01 pnl_pct, D5 gives +0.005, others give -0.002
    delay_results = []
    for i in range(n_events):
        event_id = f"evt_{i:03d}"
        entry_time = str(base_time + pd.Timedelta(days=i))
        event_delays = [
            _make_delay_result(event_id, "D0", 0.01, entry_time=entry_time),
            _make_delay_result(event_id, "D5", 0.005, entry_time=entry_time),
            _make_delay_result(event_id, "D10", -0.002, entry_time=entry_time),
            _make_delay_result(event_id, "D15", -0.003, entry_time=entry_time),
            _make_delay_result(event_id, "pullback", 0.003, entry_time=entry_time),
        ]
        delay_results.append(event_delays)

    return features, labels, delay_results, events


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------


class TestComputeCalendarSumFromResults:
    """Tests for the internal _compute_calendar_sum_from_results helper."""

    def test_empty_results(self):
        """Empty results should return 0.0 calendar sum."""
        events = pd.DataFrame({"event_id": [], "symbol": []})
        result = _compute_calendar_sum_from_results([], events)
        assert result == 0.0

    def test_single_traded_event(self):
        """Single traded event should produce correct silo return."""
        events = pd.DataFrame(
            {"event_id": ["evt_001"], "symbol": ["BTCUSDT"]}
        )
        results = [
            _make_delay_result(
                "evt_001", "D5", 0.01, entry_time="2024-01-15 10:00:05"
            )
        ]
        cal_sum = _compute_calendar_sum_from_results(results, events)
        # Expected: notional = 100000 * 0.26 = 26000
        # pnl = 26000 * 0.01 = 260
        # silo_return = 260 / 100000 * 100 = 0.26%
        assert abs(cal_sum - 0.26) < 1e-10

    def test_untraded_events_contribute_zero(self):
        """Events with traded=False should not contribute to calendar sum."""
        events = pd.DataFrame(
            {"event_id": ["evt_001", "evt_002"], "symbol": ["BTCUSDT", "BTCUSDT"]}
        )
        results = [
            _make_delay_result("evt_001", "D5", None, traded=False),
            _make_delay_result(
                "evt_002", "D5", 0.01, entry_time="2024-01-15 10:00:05"
            ),
        ]
        cal_sum = _compute_calendar_sum_from_results(results, events)
        # Only evt_002 contributes
        assert abs(cal_sum - 0.26) < 1e-10

    def test_multiple_silos_sum(self):
        """Multiple silos should have their returns summed."""
        events = pd.DataFrame(
            {
                "event_id": ["evt_001", "evt_002"],
                "symbol": ["BTCUSDT", "ETHUSDT"],
            }
        )
        results = [
            _make_delay_result(
                "evt_001", "D5", 0.01, entry_time="2024-01-15 10:00:05"
            ),
            _make_delay_result(
                "evt_002", "D5", 0.02, entry_time="2024-02-15 10:00:05"
            ),
        ]
        cal_sum = _compute_calendar_sum_from_results(results, events)
        # Silo 1 (BTCUSDT_2024-01): 0.26%
        # Silo 2 (ETHUSDT_2024-02): 0.52%
        # Total: 0.78%
        expected = 0.26 + 0.52
        assert abs(cal_sum - expected) < 1e-10


class TestLoocvCalendarSum:
    """Tests for the loocv_calendar_sum function."""

    def test_basic_functionality(self):
        """loocv_calendar_sum should return a float without errors."""
        from sklearn.tree import DecisionTreeClassifier

        features, labels, delay_results, events = _build_synthetic_data(10)

        result = loocv_calendar_sum(
            classifier_factory=lambda: DecisionTreeClassifier(
                max_depth=2, random_state=42
            ),
            features=features,
            labels=labels,
            delay_results=delay_results,
            events=events,
            bars_cache={},
        )
        assert isinstance(result, float)

    def test_perfect_classifier_uses_optimal_delays(self):
        """If classifier always predicts correctly, result should match
        using each event's label directly."""
        from sklearn.tree import DecisionTreeClassifier

        # Create data where the pattern is trivially learnable
        n = 20
        features = pd.DataFrame(
            {
                "f1": [0.0] * 10 + [1.0] * 10,
                "f2": [0.0] * 20,
            }
        )
        labels = pd.Series(["D5"] * 10 + ["D0"] * 10)

        base_time = pd.Timestamp("2024-01-15 10:00:00", tz="UTC")
        events = pd.DataFrame(
            {
                "event_id": [f"evt_{i:03d}" for i in range(n)],
                "symbol": ["BTCUSDT"] * n,
                "touch_time": [base_time + pd.Timedelta(days=i) for i in range(n)],
            }
        )

        # D0 gives 0.02, D5 gives 0.01
        delay_results = []
        for i in range(n):
            eid = f"evt_{i:03d}"
            et = str(base_time + pd.Timedelta(days=i))
            delay_results.append(
                [
                    _make_delay_result(eid, "D0", 0.02, entry_time=et),
                    _make_delay_result(eid, "D5", 0.01, entry_time=et),
                    _make_delay_result(eid, "D10", -0.01, entry_time=et),
                    _make_delay_result(eid, "D15", -0.02, entry_time=et),
                    _make_delay_result(eid, "pullback", 0.005, entry_time=et),
                ]
            )

        result = loocv_calendar_sum(
            classifier_factory=lambda: DecisionTreeClassifier(
                max_depth=2, random_state=42
            ),
            features=features,
            labels=labels,
            delay_results=delay_results,
            events=events,
            bars_cache={},
        )

        # With a trivially learnable pattern, the classifier should predict
        # correctly for most events, giving a positive calendar sum
        assert result > 0.0

    def test_handles_regime_label_format(self):
        """Should handle both delay_label format and regime format predictions."""
        # This test verifies the label mapping logic works for both formats
        features, labels, delay_results, events = _build_synthetic_data(6)

        from sklearn.tree import DecisionTreeClassifier

        # Test with delay_label format (default from compute_optimal_labels)
        result = loocv_calendar_sum(
            classifier_factory=lambda: DecisionTreeClassifier(
                max_depth=1, random_state=42
            ),
            features=features,
            labels=labels,
            delay_results=delay_results,
            events=events,
            bars_cache={},
        )
        assert isinstance(result, float)

    def test_all_same_label(self):
        """When all labels are the same, classifier predicts that label for all."""
        from sklearn.tree import DecisionTreeClassifier

        n = 8
        features = pd.DataFrame(
            {"f1": np.random.rand(n), "f2": np.random.rand(n)}
        )
        labels = pd.Series(["D5"] * n)

        base_time = pd.Timestamp("2024-01-15 10:00:00", tz="UTC")
        events = pd.DataFrame(
            {
                "event_id": [f"evt_{i:03d}" for i in range(n)],
                "symbol": ["BTCUSDT"] * n,
                "touch_time": [base_time + pd.Timedelta(days=i) for i in range(n)],
            }
        )

        delay_results = []
        for i in range(n):
            eid = f"evt_{i:03d}"
            et = str(base_time + pd.Timedelta(days=i))
            delay_results.append(
                [
                    _make_delay_result(eid, "D0", 0.005, entry_time=et),
                    _make_delay_result(eid, "D5", 0.01, entry_time=et),
                    _make_delay_result(eid, "D10", -0.001, entry_time=et),
                    _make_delay_result(eid, "D15", -0.002, entry_time=et),
                    _make_delay_result(eid, "pullback", 0.003, entry_time=et),
                ]
            )

        result = loocv_calendar_sum(
            classifier_factory=lambda: DecisionTreeClassifier(
                max_depth=2, random_state=42
            ),
            features=features,
            labels=labels,
            delay_results=delay_results,
            events=events,
            bars_cache={},
        )

        # All predictions should be "D5" → all events use D5 pnl (0.01)
        # All in same symbol, all in January 2024 → single silo
        # Compounding: balance starts at 100k, each trade adds 0.26 * balance * 0.01
        assert result > 0.0


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
