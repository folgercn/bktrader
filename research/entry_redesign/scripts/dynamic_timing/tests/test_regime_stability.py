"""Tests for compute_regime_stability function."""

from __future__ import annotations

import pytest

from research.entry_redesign.scripts.dynamic_timing.dynamic_entry_timing_runner import (
    compute_regime_stability,
)


def _make_result(regime: str, pnl_pct: float = 0.01, symbol: str = "BTCUSDT") -> dict:
    """Create a synthetic result dict with a trade."""
    return {
        "event_id": f"evt_{regime}",
        "symbol": symbol,
        "side": "long",
        "touch_time": "2024-01-01 00:00:00",
        "entry_decision": "immediate",
        "regime": regime,
        "decision_path": [(1, "immediate", regime)],
        "entry_time": "2024-01-01 00:00:05",
        "entry_price": 50000.0,
        "entry_delay_seconds": 5.0,
        "trade": {
            "entry_time": "2024-01-01 00:00:05",
            "exit_time": "2024-01-01 01:00:00",
            "realistic_pnl_pct": pnl_pct,
            "notional_share": 0.20,
        },
    }


def _make_skip_result(regime: str, symbol: str = "BTCUSDT") -> dict:
    """Create a synthetic result dict without a trade (skipped)."""
    return {
        "event_id": f"evt_skip_{regime}",
        "symbol": symbol,
        "side": "long",
        "touch_time": "2024-01-01 00:00:00",
        "entry_decision": "skip",
        "regime": regime,
        "decision_path": [(1, "skip", regime)],
        "entry_time": None,
        "entry_price": None,
        "entry_delay_seconds": None,
        "trade": None,
    }


class TestComputeRegimeStability:
    """Tests for compute_regime_stability."""

    def test_return_structure(self):
        """Returns dict with all expected keys."""
        train = [_make_result("StrongMomentum")]
        test = [_make_result("StrongMomentum")]

        result = compute_regime_stability(train, test)

        expected_keys = {
            "train_distribution",
            "test_distribution",
            "regime_distribution_shift",
            "shifted_regimes",
            "regime_stats",
        }
        assert set(result.keys()) == expected_keys

    def test_distribution_sums_to_one(self):
        """Each distribution sums to approximately 1.0."""
        train = [
            _make_result("StrongMomentum"),
            _make_result("StrongMomentum"),
            _make_result("OverExtended"),
        ]
        test = [
            _make_result("StrongMomentum"),
            _make_result("OverExtended"),
        ]

        result = compute_regime_stability(train, test)

        assert abs(sum(result["train_distribution"].values()) - 1.0) < 1e-9
        assert abs(sum(result["test_distribution"].values()) - 1.0) < 1e-9

    def test_distribution_proportions(self):
        """Distribution proportions are computed correctly."""
        train = [
            _make_result("StrongMomentum"),
            _make_result("StrongMomentum"),
            _make_result("OverExtended"),
            _make_result("WeakSignal"),
        ]
        test = [
            _make_result("StrongMomentum"),
            _make_result("OverExtended"),
        ]

        result = compute_regime_stability(train, test)

        assert result["train_distribution"]["StrongMomentum"] == pytest.approx(0.5)
        assert result["train_distribution"]["OverExtended"] == pytest.approx(0.25)
        assert result["train_distribution"]["WeakSignal"] == pytest.approx(0.25)
        assert result["test_distribution"]["StrongMomentum"] == pytest.approx(0.5)
        assert result["test_distribution"]["OverExtended"] == pytest.approx(0.5)

    def test_no_shift_when_similar_distribution(self):
        """regime_distribution_shift is False when distributions are similar."""
        train = [
            _make_result("StrongMomentum"),
            _make_result("StrongMomentum"),
            _make_result("OverExtended"),
        ]
        test = [
            _make_result("StrongMomentum"),
            _make_result("StrongMomentum"),
            _make_result("OverExtended"),
        ]

        result = compute_regime_stability(train, test)

        assert result["regime_distribution_shift"] is False
        assert result["shifted_regimes"] == []

    def test_shift_detected_when_large_difference(self):
        """regime_distribution_shift is True when a regime differs > 20pp."""
        # Train: 100% StrongMomentum, Test: 100% OverExtended
        train = [_make_result("StrongMomentum")] * 5
        test = [_make_result("OverExtended")] * 5

        result = compute_regime_stability(train, test)

        assert result["regime_distribution_shift"] is True
        assert "StrongMomentum" in result["shifted_regimes"]
        assert "OverExtended" in result["shifted_regimes"]

    def test_shift_boundary_not_triggered_at_20pp(self):
        """Shift is NOT triggered at exactly 20 percentage points (> 20, not >=)."""
        # Train: 60% A, 40% B → Test: 80% A, 20% B → diff = 20pp exactly
        train = [_make_result("A")] * 3 + [_make_result("B")] * 2
        test = [_make_result("A")] * 4 + [_make_result("B")] * 1

        result = compute_regime_stability(train, test)

        # 60% vs 80% = 20pp exactly, should NOT trigger (> 20, not >=)
        assert result["regime_distribution_shift"] is False

    def test_shift_triggered_above_20pp(self):
        """Shift IS triggered when difference exceeds 20 percentage points."""
        # Train: 50% A, 50% B → Test: 80% A, 20% B → diff = 30pp
        train = [_make_result("A")] * 5 + [_make_result("B")] * 5
        test = [_make_result("A")] * 8 + [_make_result("B")] * 2

        result = compute_regime_stability(train, test)

        assert result["regime_distribution_shift"] is True
        assert "A" in result["shifted_regimes"]
        assert "B" in result["shifted_regimes"]

    def test_regime_stats_count(self):
        """regime_stats reports correct event count per regime."""
        train = [_make_result("A"), _make_result("A"), _make_result("B")]
        test = [_make_result("A"), _make_result("B")]

        result = compute_regime_stability(train, test)

        assert result["regime_stats"]["A"]["count"] == 3
        assert result["regime_stats"]["B"]["count"] == 2

    def test_regime_stats_trade_count(self):
        """regime_stats reports correct trade count (excludes skipped events)."""
        train = [_make_result("A"), _make_skip_result("A")]
        test = [_make_result("A")]

        result = compute_regime_stability(train, test)

        assert result["regime_stats"]["A"]["count"] == 3
        assert result["regime_stats"]["A"]["trade_count"] == 2

    def test_regime_stats_win_rate(self):
        """regime_stats computes correct win rate."""
        train = [
            _make_result("A", pnl_pct=0.01),  # win
            _make_result("A", pnl_pct=-0.005),  # loss
        ]
        test = [
            _make_result("A", pnl_pct=0.02),  # win
        ]

        result = compute_regime_stability(train, test)

        # 2 wins out of 3 trades
        assert result["regime_stats"]["A"]["win_rate"] == pytest.approx(2 / 3)

    def test_regime_stats_avg_pnl(self):
        """regime_stats computes correct avg_pnl_pct."""
        train = [
            _make_result("A", pnl_pct=0.01),
            _make_result("A", pnl_pct=-0.005),
        ]
        test = [
            _make_result("A", pnl_pct=0.02),
        ]

        result = compute_regime_stability(train, test)

        # avg = (0.01 + (-0.005) + 0.02) / 3 * 100 = 0.833...%
        expected_avg = (0.01 + (-0.005) + 0.02) / 3 * 100
        assert result["regime_stats"]["A"]["avg_pnl_pct"] == pytest.approx(expected_avg)

    def test_regime_stats_no_trades(self):
        """regime_stats handles regime with no trades (all skipped)."""
        train = [_make_skip_result("WeakSignal")]
        test = [_make_skip_result("WeakSignal")]

        result = compute_regime_stability(train, test)

        stats = result["regime_stats"]["WeakSignal"]
        assert stats["count"] == 2
        assert stats["trade_count"] == 0
        assert stats["win_rate"] == 0.0
        assert stats["avg_pnl_pct"] == 0.0
        assert stats["calendar_contribution_pct"] == 0.0

    def test_empty_train_results(self):
        """Handles empty train_results gracefully."""
        test = [_make_result("A")]

        result = compute_regime_stability([], test)

        assert result["train_distribution"] == {}
        assert result["test_distribution"]["A"] == 1.0
        # A is 0% in train, 100% in test → 100pp shift
        assert result["regime_distribution_shift"] is True
        assert "A" in result["shifted_regimes"]

    def test_empty_test_results(self):
        """Handles empty test_results gracefully."""
        train = [_make_result("A")]

        result = compute_regime_stability(train, [])

        assert result["test_distribution"] == {}
        assert result["train_distribution"]["A"] == 1.0
        # A is 100% in train, 0% in test → 100pp shift
        assert result["regime_distribution_shift"] is True

    def test_both_empty(self):
        """Handles both empty results gracefully."""
        result = compute_regime_stability([], [])

        assert result["train_distribution"] == {}
        assert result["test_distribution"] == {}
        assert result["regime_distribution_shift"] is False
        assert result["shifted_regimes"] == []
        assert result["regime_stats"] == {}

    def test_regime_missing_in_one_set(self):
        """Regime present in train but not test is handled correctly."""
        train = [_make_result("A"), _make_result("B")]
        test = [_make_result("A")]

        result = compute_regime_stability(train, test)

        # B: 50% in train, 0% in test → 50pp shift
        assert "B" in result["shifted_regimes"]
        # A: 50% in train, 100% in test → 50pp shift
        assert "A" in result["shifted_regimes"]
