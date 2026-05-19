"""Tests for compute_bootstrap_ci function."""

import numpy as np
import pytest

from research.entry_redesign.scripts.dynamic_timing.dynamic_entry_timing_runner import (
    compute_bootstrap_ci,
)


def _make_result(symbol: str, pnl: float, month: str = "2024-01") -> dict:
    """Helper to create a minimal result dict with a trade."""
    return {
        "event_id": f"{symbol}_{month}_{pnl}",
        "symbol": symbol,
        "side": "long",
        "touch_time": f"2024-{month.split('-')[1]}-15 10:00:00",
        "entry_decision": "immediate",
        "regime": "StrongMomentum",
        "decision_path": [],
        "entry_time": f"2024-{month.split('-')[1]}-15 10:00:05",
        "entry_price": 50000.0,
        "entry_delay_seconds": 5.0,
        "trade": {
            "entry_time": f"2024-{month.split('-')[1]}-15 10:00:05",
            "exit_time": f"2024-{month.split('-')[1]}-15 11:00:00",
            "entry_p": 50000.0,
            "exit_p": 50000.0 * (1 + pnl),
            "pnl_pct": pnl,
            "realistic_pnl_pct": pnl,
            "exit_reason": "trail_stop",
            "mfe_r": 1.5,
            "mae_r": 0.3,
            "hold_seconds": 3600,
            "notional_share": 0.20,
        },
    }


def _make_skip_result(symbol: str) -> dict:
    """Helper to create a result with no trade (skip)."""
    return {
        "event_id": f"{symbol}_skip",
        "symbol": symbol,
        "side": "long",
        "touch_time": "2024-01-15 10:00:00",
        "entry_decision": "skip",
        "regime": "WeakSignal",
        "decision_path": [],
        "entry_time": None,
        "entry_price": None,
        "entry_delay_seconds": None,
        "trade": None,
    }


class TestComputeBootstrapCI:
    """Tests for compute_bootstrap_ci."""

    def test_returns_required_keys(self):
        """Output dict contains all required keys."""
        results = [
            _make_result("BTCUSDT", 0.01),
            _make_result("ETHUSDT", 0.005),
        ]
        ci = compute_bootstrap_ci(results)

        assert "btc_ci" in ci
        assert "eth_ci" in ci
        assert "combined_ci" in ci
        assert "small_sample_warning" in ci

    def test_small_sample_warning_always_true(self):
        """small_sample_warning is always True (Requirement 5.6)."""
        results = [_make_result("BTCUSDT", 0.01)]
        ci = compute_bootstrap_ci(results)
        assert ci["small_sample_warning"] is True

    def test_ci_structure(self):
        """Each CI dict has p5, p95, mean keys."""
        results = [
            _make_result("BTCUSDT", 0.01),
            _make_result("BTCUSDT", 0.02),
            _make_result("ETHUSDT", -0.005),
        ]
        ci = compute_bootstrap_ci(results)

        for key in ["btc_ci", "eth_ci", "combined_ci"]:
            assert "p5" in ci[key]
            assert "p95" in ci[key]
            assert "mean" in ci[key]

    def test_p5_le_mean_le_p95(self):
        """p5 <= mean <= p95 for non-trivial data."""
        results = [
            _make_result("BTCUSDT", 0.01, "2024-01"),
            _make_result("BTCUSDT", 0.02, "2024-02"),
            _make_result("BTCUSDT", -0.005, "2024-03"),
            _make_result("BTCUSDT", 0.015, "2024-04"),
            _make_result("BTCUSDT", -0.01, "2024-05"),
        ]
        ci = compute_bootstrap_ci(results)

        assert ci["btc_ci"]["p5"] <= ci["btc_ci"]["mean"]
        assert ci["btc_ci"]["mean"] <= ci["btc_ci"]["p95"]

    def test_empty_symbol_returns_zeros(self):
        """If no events for a symbol, CI is all zeros."""
        results = [_make_result("BTCUSDT", 0.01)]
        ci = compute_bootstrap_ci(results)

        assert ci["eth_ci"]["p5"] == 0.0
        assert ci["eth_ci"]["p95"] == 0.0
        assert ci["eth_ci"]["mean"] == 0.0

    def test_deterministic_with_same_seed(self):
        """Same seed produces identical results (Requirement 6.6)."""
        results = [
            _make_result("BTCUSDT", 0.01, "2024-01"),
            _make_result("BTCUSDT", -0.005, "2024-02"),
            _make_result("ETHUSDT", 0.02, "2024-01"),
            _make_result("ETHUSDT", -0.01, "2024-02"),
        ]

        ci1 = compute_bootstrap_ci(results, seed=42)
        ci2 = compute_bootstrap_ci(results, seed=42)

        assert ci1 == ci2

    def test_different_seed_different_results(self):
        """Different seeds produce different results."""
        results = [
            _make_result("BTCUSDT", 0.01, "2024-01"),
            _make_result("BTCUSDT", -0.005, "2024-02"),
            _make_result("BTCUSDT", 0.02, "2024-03"),
            _make_result("BTCUSDT", -0.01, "2024-04"),
        ]

        ci1 = compute_bootstrap_ci(results, seed=42)
        ci2 = compute_bootstrap_ci(results, seed=123)

        # With different seeds, at least one value should differ
        assert ci1["btc_ci"]["p5"] != ci2["btc_ci"]["p5"] or \
               ci1["btc_ci"]["p95"] != ci2["btc_ci"]["p95"]

    def test_btc_eth_independent(self):
        """BTC and ETH bootstrap are computed independently."""
        btc_results = [
            _make_result("BTCUSDT", 0.05, "2024-01"),
            _make_result("BTCUSDT", 0.03, "2024-02"),
        ]
        eth_results = [
            _make_result("ETHUSDT", -0.02, "2024-01"),
            _make_result("ETHUSDT", -0.03, "2024-02"),
        ]

        ci = compute_bootstrap_ci(btc_results + eth_results)

        # BTC should have positive CI, ETH should have negative CI
        assert ci["btc_ci"]["mean"] > 0
        assert ci["eth_ci"]["mean"] < 0

    def test_skip_results_handled(self):
        """Results with no trade (skip) are handled gracefully."""
        results = [
            _make_result("BTCUSDT", 0.01),
            _make_skip_result("BTCUSDT"),
            _make_result("ETHUSDT", 0.005),
            _make_skip_result("ETHUSDT"),
        ]
        ci = compute_bootstrap_ci(results)

        # Should not raise, and should produce valid output
        assert ci["small_sample_warning"] is True
        assert isinstance(ci["btc_ci"]["mean"], float)

    def test_n_bootstrap_parameter(self):
        """Custom n_bootstrap is respected."""
        results = [
            _make_result("BTCUSDT", 0.01, "2024-01"),
            _make_result("BTCUSDT", -0.005, "2024-02"),
        ]

        # With n_bootstrap=10, should still work (just less precise)
        ci = compute_bootstrap_ci(results, n_bootstrap=10)
        assert isinstance(ci["btc_ci"]["p5"], float)
        assert isinstance(ci["btc_ci"]["p95"], float)

    def test_all_values_are_float(self):
        """All CI values are Python floats (not numpy types)."""
        results = [
            _make_result("BTCUSDT", 0.01, "2024-01"),
            _make_result("BTCUSDT", 0.02, "2024-02"),
            _make_result("ETHUSDT", -0.01, "2024-01"),
        ]
        ci = compute_bootstrap_ci(results)

        for key in ["btc_ci", "eth_ci", "combined_ci"]:
            assert type(ci[key]["p5"]) is float
            assert type(ci[key]["p95"]) is float
            assert type(ci[key]["mean"]) is float
