"""Tests for evaluate_on_test_set function."""

from __future__ import annotations

import numpy as np
import pandas as pd
import pytest

from research.entry_redesign.scripts.dynamic_timing.dynamic_entry_timing_runner import (
    compute_calendar_sum,
    evaluate_on_test_set,
)
from research.entry_redesign.scripts.dynamic_timing.execution_sim import DEFAULT_EXEC_PARAMS
from research.entry_redesign.scripts.dynamic_timing.regime_classifier import TimingParams


def _make_bars(start: str, n: int = 300, base_price: float = 50000.0) -> pd.DataFrame:
    """Create synthetic 1s bars for testing."""
    idx = pd.date_range(start, periods=n, freq="1s", tz="UTC")
    rng = np.random.default_rng(42)
    closes = base_price + np.cumsum(rng.normal(0, 1, n))
    highs = closes + rng.uniform(0, 2, n)
    lows = closes - rng.uniform(0, 2, n)
    return pd.DataFrame(
        {"open": closes - 0.5, "high": highs, "low": lows, "close": closes}, index=idx
    )


def _make_events(touch_time: str, n: int = 1, symbol: str = "BTCUSDT") -> pd.DataFrame:
    """Create synthetic events for testing."""
    rows = []
    for i in range(n):
        rows.append({
            "event_id": f"evt_{i}",
            "symbol": symbol,
            "side": "long",
            "touch_time": pd.Timestamp(touch_time, tz="UTC")
            + pd.Timedelta(seconds=i * 100),
            "level": 50000.0,
            "atr": 100.0,
            "signal_low": 49900.0,
            "signal_high": 50100.0,
        })
    return pd.DataFrame(rows)


def _make_multi_symbol_events() -> pd.DataFrame:
    """Create events with both BTC and ETH symbols."""
    rows = []
    for i in range(3):
        rows.append({
            "event_id": f"btc_evt_{i}",
            "symbol": "BTCUSDT",
            "side": "long",
            "touch_time": pd.Timestamp("2024-01-01 00:00:00", tz="UTC")
            + pd.Timedelta(seconds=i * 100),
            "level": 50000.0,
            "atr": 100.0,
            "signal_low": 49900.0,
            "signal_high": 50100.0,
        })
    for i in range(2):
        rows.append({
            "event_id": f"eth_evt_{i}",
            "symbol": "ETHUSDT",
            "side": "long",
            "touch_time": pd.Timestamp("2024-01-01 00:00:00", tz="UTC")
            + pd.Timedelta(seconds=i * 100),
            "level": 3000.0,
            "atr": 20.0,
            "signal_low": 2980.0,
            "signal_high": 3020.0,
        })
    return pd.DataFrame(rows)


class TestEvaluateOnTestSet:
    """Tests for evaluate_on_test_set function."""

    def test_return_structure(self):
        """evaluate_on_test_set returns dict with all expected keys."""
        bars = _make_bars("2024-01-01 00:00:00", n=600)
        events = _make_events("2024-01-01 00:00:00", n=2)
        cache = {"BTCUSDT_202401": bars}
        params = TimingParams(max_steps=2)

        result = evaluate_on_test_set(events, cache, params, train_calendar_sum=5.0)

        expected_keys = {
            "dynamic_results",
            "baselines",
            "metrics",
            "overfitting_flag",
            "train_calendar_sum",
            "test_calendar_sum",
            "symbol_metrics",
        }
        assert set(result.keys()) == expected_keys

    def test_metrics_structure(self):
        """metrics dict contains dynamic and all 3 baselines."""
        bars = _make_bars("2024-01-01 00:00:00", n=600)
        events = _make_events("2024-01-01 00:00:00", n=2)
        cache = {"BTCUSDT_202401": bars}
        params = TimingParams(max_steps=2)

        result = evaluate_on_test_set(events, cache, params, train_calendar_sum=5.0)

        assert set(result["metrics"].keys()) == {
            "dynamic",
            "baseline_a",
            "baseline_b",
            "baseline_c",
        }

    def test_metrics_fields(self):
        """Each metrics entry has all required comparison fields."""
        bars = _make_bars("2024-01-01 00:00:00", n=600)
        events = _make_events("2024-01-01 00:00:00", n=2)
        cache = {"BTCUSDT_202401": bars}
        params = TimingParams(max_steps=2)

        result = evaluate_on_test_set(events, cache, params, train_calendar_sum=5.0)

        expected_metric_keys = {
            "calendar_sum_pct",
            "trade_count",
            "win_rate",
            "avg_win_pct",
            "avg_loss_pct",
            "payoff_ratio",
            "skip_rate",
            "pullback_fill_rate",
            "per_trade_quality_bps",
        }
        for key in ("dynamic", "baseline_a", "baseline_b", "baseline_c"):
            assert set(result["metrics"][key].keys()) == expected_metric_keys

    def test_overfitting_flag_true(self):
        """overfitting_flag is True when test cal_sum drops > 50% from train."""
        bars = _make_bars("2024-01-01 00:00:00", n=600)
        events = _make_events("2024-01-01 00:00:00", n=2)
        cache = {"BTCUSDT_202401": bars}
        params = TimingParams(max_steps=2)

        # Use a very large train_calendar_sum so any test result is > 50% drop
        result = evaluate_on_test_set(
            events, cache, params, train_calendar_sum=9999.0
        )

        assert result["overfitting_flag"] == True

    def test_overfitting_flag_false_when_train_negative(self):
        """overfitting_flag is False when train_calendar_sum <= 0."""
        bars = _make_bars("2024-01-01 00:00:00", n=600)
        events = _make_events("2024-01-01 00:00:00", n=2)
        cache = {"BTCUSDT_202401": bars}
        params = TimingParams(max_steps=2)

        result = evaluate_on_test_set(
            events, cache, params, train_calendar_sum=-5.0
        )

        assert result["overfitting_flag"] == False

    def test_overfitting_flag_false_when_no_drop(self):
        """overfitting_flag is False when test cal_sum does not drop > 50%."""
        bars = _make_bars("2024-01-01 00:00:00", n=600)
        events = _make_events("2024-01-01 00:00:00", n=2)
        cache = {"BTCUSDT_202401": bars}
        params = TimingParams(max_steps=2)

        # First run to get actual test_calendar_sum
        result = evaluate_on_test_set(events, cache, params, train_calendar_sum=0.001)

        # If test_cal_sum is close to train, no overfitting
        # Use a train value that's close to test value
        test_cal = result["test_calendar_sum"]
        result2 = evaluate_on_test_set(
            events, cache, params, train_calendar_sum=test_cal * 1.1
        )
        # Drop is only ~9%, should not flag
        assert result2["overfitting_flag"] == False

    def test_symbol_metrics_per_symbol(self):
        """symbol_metrics contains entry for each unique symbol."""
        btc_bars = _make_bars("2024-01-01 00:00:00", n=600, base_price=50000.0)
        eth_bars = _make_bars("2024-01-01 00:00:00", n=600, base_price=3000.0)
        events = _make_multi_symbol_events()
        cache = {
            "BTCUSDT_202401": btc_bars,
            "ETHUSDT_202401": eth_bars,
        }
        params = TimingParams(max_steps=2)

        result = evaluate_on_test_set(events, cache, params, train_calendar_sum=5.0)

        assert "BTCUSDT" in result["symbol_metrics"]
        assert "ETHUSDT" in result["symbol_metrics"]

    def test_symbol_metrics_fields(self):
        """Each symbol_metrics entry has calendar_sum_pct, win_rate, trade_count, negative_flag."""
        bars = _make_bars("2024-01-01 00:00:00", n=600)
        events = _make_events("2024-01-01 00:00:00", n=2)
        cache = {"BTCUSDT_202401": bars}
        params = TimingParams(max_steps=2)

        result = evaluate_on_test_set(events, cache, params, train_calendar_sum=5.0)

        sym = result["symbol_metrics"]["BTCUSDT"]
        assert "calendar_sum_pct" in sym
        assert "win_rate" in sym
        assert "trade_count" in sym
        assert "negative_flag" in sym

    def test_symbol_negative_flag(self):
        """negative_flag is True when symbol's calendar_sum < 0."""
        bars = _make_bars("2024-01-01 00:00:00", n=600)
        events = _make_events("2024-01-01 00:00:00", n=2)
        cache = {"BTCUSDT_202401": bars}
        params = TimingParams(max_steps=2)

        result = evaluate_on_test_set(events, cache, params, train_calendar_sum=5.0)

        sym = result["symbol_metrics"]["BTCUSDT"]
        # negative_flag should be consistent with calendar_sum_pct sign
        assert sym["negative_flag"] == (sym["calendar_sum_pct"] < 0)

    def test_baselines_dict_structure(self):
        """baselines contains baseline_a, baseline_b, baseline_c."""
        bars = _make_bars("2024-01-01 00:00:00", n=600)
        events = _make_events("2024-01-01 00:00:00", n=2)
        cache = {"BTCUSDT_202401": bars}
        params = TimingParams(max_steps=2)

        result = evaluate_on_test_set(events, cache, params, train_calendar_sum=5.0)

        assert set(result["baselines"].keys()) == {
            "baseline_a",
            "baseline_b",
            "baseline_c",
        }

    def test_dynamic_results_length(self):
        """dynamic_results has one entry per event."""
        bars = _make_bars("2024-01-01 00:00:00", n=600)
        events = _make_events("2024-01-01 00:00:00", n=3)
        cache = {"BTCUSDT_202401": bars}
        params = TimingParams(max_steps=2)

        result = evaluate_on_test_set(events, cache, params, train_calendar_sum=5.0)

        assert len(result["dynamic_results"]) == 3

    def test_train_calendar_sum_passthrough(self):
        """train_calendar_sum is passed through to the result."""
        bars = _make_bars("2024-01-01 00:00:00", n=600)
        events = _make_events("2024-01-01 00:00:00", n=2)
        cache = {"BTCUSDT_202401": bars}
        params = TimingParams(max_steps=2)

        result = evaluate_on_test_set(events, cache, params, train_calendar_sum=12.34)

        assert result["train_calendar_sum"] == 12.34

    def test_empty_events(self):
        """Handles empty events DataFrame gracefully."""
        bars = _make_bars("2024-01-01 00:00:00", n=600)
        events = pd.DataFrame(
            columns=[
                "event_id", "symbol", "side", "touch_time",
                "level", "atr", "signal_low", "signal_high",
            ]
        )
        cache = {"BTCUSDT_202401": bars}
        params = TimingParams(max_steps=2)

        result = evaluate_on_test_set(events, cache, params, train_calendar_sum=5.0)

        assert result["dynamic_results"] == []
        assert result["metrics"]["dynamic"]["trade_count"] == 0
        assert result["metrics"]["dynamic"]["skip_rate"] == 1.0
        assert result["symbol_metrics"] == {}

    def test_custom_exec_params(self):
        """Custom exec_params are forwarded to dynamic timing and baselines."""
        bars = _make_bars("2024-01-01 00:00:00", n=600)
        events = _make_events("2024-01-01 00:00:00", n=2)
        cache = {"BTCUSDT_202401": bars}
        params = TimingParams(max_steps=2)

        # Very large min_stop_bps forces all trades to None
        custom_params = {**DEFAULT_EXEC_PARAMS, "min_stop_bps": 99999.0}
        result = evaluate_on_test_set(
            events, cache, params, train_calendar_sum=5.0, exec_params=custom_params
        )

        # All trades should be None
        assert result["metrics"]["dynamic"]["trade_count"] == 0
        assert result["metrics"]["baseline_a"]["trade_count"] == 0
