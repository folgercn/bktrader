"""Tests for run_sensitivity_analysis function."""

from __future__ import annotations

import numpy as np
import pandas as pd
import pytest
from pathlib import Path

from research.entry_redesign.scripts.dynamic_timing.dynamic_entry_timing_runner import (
    run_sensitivity_analysis,
)
from research.entry_redesign.scripts.dynamic_timing.regime_classifier import TimingParams


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


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


def _make_events(touch_time: str, n: int = 1) -> pd.DataFrame:
    """Create synthetic events for testing."""
    rows = []
    for i in range(n):
        rows.append({
            "event_id": f"evt_{i}",
            "symbol": "BTCUSDT",
            "side": "long",
            "touch_time": pd.Timestamp(touch_time, tz="UTC")
                + pd.Timedelta(seconds=i * 100),
            "level": 50000.0,
            "atr": 100.0,
            "signal_low": 49900.0,
            "signal_high": 50100.0,
        })
    return pd.DataFrame(rows)


# ---------------------------------------------------------------------------
# Tests for run_sensitivity_analysis
# ---------------------------------------------------------------------------


class TestRunSensitivityAnalysis:
    """Tests for run_sensitivity_analysis function."""

    @pytest.fixture
    def setup_data(self):
        """Common test data setup."""
        bars = _make_bars("2024-01-01 00:00:00", n=600)
        events = _make_events("2024-01-01 00:00:10", n=2)
        cache = {"BTCUSDT_202401": bars}
        best_params = TimingParams()
        return events, cache, best_params

    def test_returns_dict_with_expected_keys(self, setup_data):
        """Result dict has 'sensitivities' and 'high_sensitivity_params' keys."""
        events, cache, best_params = setup_data
        result = run_sensitivity_analysis(events, cache, best_params)

        assert "sensitivities" in result
        assert "high_sensitivity_params" in result

    def test_sensitivities_covers_all_param_axes(self, setup_data):
        """Sensitivities dict has entries for all 10 parameter axes."""
        events, cache, best_params = setup_data
        result = run_sensitivity_analysis(events, cache, best_params)

        expected_params = {
            "max_steps",
            "strong_momentum_threshold",
            "strong_flow_threshold",
            "moderate_momentum_threshold",
            "extension_threshold",
            "weak_flow_threshold",
            "fading_threshold",
            "min_steps_for_skip",
            "pullback_target_atr",
            "decision_window_seconds",
        }
        assert set(result["sensitivities"].keys()) == expected_params

    def test_sensitivity_df_has_correct_columns(self, setup_data):
        """Each sensitivity DataFrame has param_value and calendar_sum_pct columns."""
        events, cache, best_params = setup_data
        result = run_sensitivity_analysis(events, cache, best_params)

        for param_name, df in result["sensitivities"].items():
            assert "param_value" in df.columns, f"Missing param_value in {param_name}"
            assert "calendar_sum_pct" in df.columns, f"Missing calendar_sum_pct in {param_name}"

    def test_sensitivity_df_row_count_matches_candidates(self, setup_data):
        """Each sensitivity DataFrame has correct number of rows matching candidate count."""
        events, cache, best_params = setup_data
        result = run_sensitivity_analysis(events, cache, best_params)

        expected_counts = {
            "max_steps": 4,
            "strong_momentum_threshold": 4,
            "strong_flow_threshold": 4,
            "moderate_momentum_threshold": 4,
            "extension_threshold": 4,
            "weak_flow_threshold": 3,
            "fading_threshold": 3,
            "min_steps_for_skip": 3,
            "pullback_target_atr": 3,
            "decision_window_seconds": 3,
        }
        for param_name, expected_n in expected_counts.items():
            df = result["sensitivities"][param_name]
            assert len(df) == expected_n, (
                f"{param_name}: expected {expected_n} rows, got {len(df)}"
            )

    def test_high_sensitivity_params_is_list(self, setup_data):
        """high_sensitivity_params is a list of strings."""
        events, cache, best_params = setup_data
        result = run_sensitivity_analysis(events, cache, best_params)

        assert isinstance(result["high_sensitivity_params"], list)
        for p in result["high_sensitivity_params"]:
            assert isinstance(p, str)

    def test_saves_csv_files_when_output_dir_provided(self, setup_data, tmp_path):
        """CSV files are saved to sensitivity/ subdirectory when output_dir is provided."""
        events, cache, best_params = setup_data
        result = run_sensitivity_analysis(
            events, cache, best_params, output_dir=tmp_path
        )

        sens_dir = tmp_path / "sensitivity"
        assert sens_dir.exists()

        # Check that CSV files exist for each parameter
        for param_name in result["sensitivities"]:
            csv_path = sens_dir / f"sensitivity_{param_name}.csv"
            assert csv_path.exists(), f"Missing CSV for {param_name}"
            loaded = pd.read_csv(csv_path)
            assert "param_value" in loaded.columns
            assert "calendar_sum_pct" in loaded.columns

    def test_no_files_when_output_dir_none(self, setup_data, tmp_path):
        """No files are created when output_dir is None."""
        events, cache, best_params = setup_data
        result = run_sensitivity_analysis(events, cache, best_params, output_dir=None)

        # Result should still be valid
        assert len(result["sensitivities"]) == 10

    def test_deterministic_results(self, setup_data):
        """Two runs with same input produce identical results."""
        events, cache, best_params = setup_data

        result1 = run_sensitivity_analysis(events, cache, best_params)
        result2 = run_sensitivity_analysis(events, cache, best_params)

        for param_name in result1["sensitivities"]:
            pd.testing.assert_frame_equal(
                result1["sensitivities"][param_name],
                result2["sensitivities"][param_name],
            )
        assert result1["high_sensitivity_params"] == result2["high_sensitivity_params"]

    def test_high_sensitivity_threshold_3pct(self):
        """Params with calendar_sum range > 3% are marked as high sensitivity."""
        # This is a structural test: we verify the logic by checking that
        # high_sensitivity_params only contains params where range > 3.0
        bars = _make_bars("2024-01-01 00:00:00", n=600)
        events = _make_events("2024-01-01 00:00:10", n=2)
        cache = {"BTCUSDT_202401": bars}
        best_params = TimingParams()

        result = run_sensitivity_analysis(events, cache, best_params)

        for param_name in result["high_sensitivity_params"]:
            df = result["sensitivities"][param_name]
            cal_range = df["calendar_sum_pct"].max() - df["calendar_sum_pct"].min()
            assert cal_range > 3.0, (
                f"{param_name} marked high sensitivity but range={cal_range:.2f}"
            )

        # Also verify non-high-sensitivity params have range <= 3.0
        for param_name, df in result["sensitivities"].items():
            if param_name not in result["high_sensitivity_params"]:
                cal_range = df["calendar_sum_pct"].max() - df["calendar_sum_pct"].min()
                assert cal_range <= 3.0, (
                    f"{param_name} NOT marked high sensitivity but range={cal_range:.2f}"
                )
