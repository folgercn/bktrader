"""Tests for generate_output_files function."""

import json
from dataclasses import asdict
from pathlib import Path

import pandas as pd
import pytest

from ..dynamic_entry_timing_runner import generate_output_files
from ..regime_classifier import TimingParams


@pytest.fixture
def sample_dynamic_results():
    """Sample dynamic timing results for testing."""
    return [
        {
            "event_id": "evt_001",
            "symbol": "BTCUSDT",
            "side": "long",
            "touch_time": pd.Timestamp("2024-01-15 10:00:00", tz="UTC"),
            "entry_decision": "immediate",
            "regime": "StrongMomentum",
            "decision_path": [(1, "immediate", "StrongMomentum")],
            "entry_time": pd.Timestamp("2024-01-15 10:00:05", tz="UTC"),
            "entry_price": 42000.0,
            "entry_delay_seconds": 5.0,
            "trade": {
                "entry_time": pd.Timestamp("2024-01-15 10:00:05", tz="UTC"),
                "exit_time": pd.Timestamp("2024-01-15 11:00:00", tz="UTC"),
                "entry_p": 42000.0,
                "exit_p": 42200.0,
                "pnl_pct": 0.0048,
                "realistic_pnl_pct": 0.0040,
                "exit_reason": "trailing_stop",
                "mfe_r": 1.5,
                "mae_r": 0.3,
                "hold_seconds": 3595,
                "notional_share": 0.20,
            },
        },
        {
            "event_id": "evt_002",
            "symbol": "ETHUSDT",
            "side": "short",
            "touch_time": pd.Timestamp("2024-01-16 14:00:00", tz="UTC"),
            "entry_decision": "wait_pullback",
            "regime": "OverExtended",
            "decision_path": [
                (1, "continue_observe", "Developing"),
                (2, "wait_pullback", "OverExtended"),
            ],
            "entry_time": pd.Timestamp("2024-01-16 14:00:30", tz="UTC"),
            "entry_price": 2250.0,
            "entry_delay_seconds": 30.0,
            "trade": {
                "entry_time": pd.Timestamp("2024-01-16 14:00:30", tz="UTC"),
                "exit_time": pd.Timestamp("2024-01-16 15:00:00", tz="UTC"),
                "entry_p": 2250.0,
                "exit_p": 2230.0,
                "pnl_pct": 0.0089,
                "realistic_pnl_pct": 0.0080,
                "exit_reason": "trailing_stop",
                "mfe_r": 2.0,
                "mae_r": 0.2,
                "hold_seconds": 3570,
                "notional_share": 0.20,
            },
        },
        {
            "event_id": "evt_003",
            "symbol": "BTCUSDT",
            "side": "long",
            "touch_time": pd.Timestamp("2024-01-17 08:00:00", tz="UTC"),
            "entry_decision": "skip",
            "regime": "WeakSignal",
            "decision_path": [
                (1, "continue_observe", "Developing"),
                (2, "continue_observe", "Developing"),
                (3, "skip", "WeakSignal"),
            ],
            "entry_time": None,
            "entry_price": None,
            "entry_delay_seconds": None,
            "trade": None,
        },
        {
            "event_id": "evt_004",
            "symbol": "BTCUSDT",
            "side": "long",
            "touch_time": pd.Timestamp("2024-01-18 12:00:00", tz="UTC"),
            "entry_decision": "skip",
            "regime": "ConsecutiveDataMissing",
            "decision_path": [
                (1, "continue_observe", "DataMissing"),
                (2, "continue_observe", "DataMissing"),
                (3, "continue_observe", "DataMissing"),
            ],
            "entry_time": None,
            "entry_price": None,
            "entry_delay_seconds": None,
            "trade": None,
        },
    ]


@pytest.fixture
def sample_baseline_a_results():
    """Sample baseline A results for testing."""
    return [
        {
            "event_id": "evt_001",
            "symbol": "BTCUSDT",
            "side": "long",
            "touch_time": pd.Timestamp("2024-01-15 10:00:00", tz="UTC"),
            "entry_decision": "baseline",
            "regime": "D=5s",
            "decision_path": [],
            "entry_time": pd.Timestamp("2024-01-15 10:00:05", tz="UTC"),
            "entry_price": 42010.0,
            "entry_delay_seconds": 5.0,
            "trade": {
                "entry_time": pd.Timestamp("2024-01-15 10:00:05", tz="UTC"),
                "exit_time": pd.Timestamp("2024-01-15 11:00:00", tz="UTC"),
                "entry_p": 42010.0,
                "exit_p": 42200.0,
                "pnl_pct": 0.0045,
                "realistic_pnl_pct": 0.0037,
                "exit_reason": "trailing_stop",
                "mfe_r": 1.4,
                "mae_r": 0.3,
                "hold_seconds": 3595,
                "notional_share": 0.20,
            },
        },
        {
            "event_id": "evt_002",
            "symbol": "ETHUSDT",
            "side": "short",
            "touch_time": pd.Timestamp("2024-01-16 14:00:00", tz="UTC"),
            "entry_decision": "baseline",
            "regime": "D=5s",
            "decision_path": [],
            "entry_time": pd.Timestamp("2024-01-16 14:00:05", tz="UTC"),
            "entry_price": 2248.0,
            "entry_delay_seconds": 5.0,
            "trade": {
                "entry_time": pd.Timestamp("2024-01-16 14:00:05", tz="UTC"),
                "exit_time": pd.Timestamp("2024-01-16 15:00:00", tz="UTC"),
                "entry_p": 2248.0,
                "exit_p": 2230.0,
                "pnl_pct": 0.0080,
                "realistic_pnl_pct": 0.0072,
                "exit_reason": "trailing_stop",
                "mfe_r": 1.8,
                "mae_r": 0.2,
                "hold_seconds": 3570,
                "notional_share": 0.20,
            },
        },
        {
            "event_id": "evt_003",
            "symbol": "BTCUSDT",
            "side": "long",
            "touch_time": pd.Timestamp("2024-01-17 08:00:00", tz="UTC"),
            "entry_decision": "baseline",
            "regime": "D=5s",
            "decision_path": [],
            "entry_time": pd.Timestamp("2024-01-17 08:00:05", tz="UTC"),
            "entry_price": 43000.0,
            "entry_delay_seconds": 5.0,
            "trade": {
                "entry_time": pd.Timestamp("2024-01-17 08:00:05", tz="UTC"),
                "exit_time": pd.Timestamp("2024-01-17 09:00:00", tz="UTC"),
                "entry_p": 43000.0,
                "exit_p": 42800.0,
                "pnl_pct": -0.0047,
                "realistic_pnl_pct": -0.0055,
                "exit_reason": "stop_loss",
                "mfe_r": 0.2,
                "mae_r": 1.0,
                "hold_seconds": 3600,
                "notional_share": 0.20,
            },
        },
        {
            "event_id": "evt_004",
            "symbol": "BTCUSDT",
            "side": "long",
            "touch_time": pd.Timestamp("2024-01-18 12:00:00", tz="UTC"),
            "entry_decision": "baseline",
            "regime": "D=5s",
            "decision_path": [],
            "entry_time": pd.Timestamp("2024-01-18 12:00:05", tz="UTC"),
            "entry_price": 44000.0,
            "entry_delay_seconds": 5.0,
            "trade": {
                "entry_time": pd.Timestamp("2024-01-18 12:00:05", tz="UTC"),
                "exit_time": pd.Timestamp("2024-01-18 13:00:00", tz="UTC"),
                "entry_p": 44000.0,
                "exit_p": 44100.0,
                "pnl_pct": 0.0023,
                "realistic_pnl_pct": 0.0015,
                "exit_reason": "trailing_stop",
                "mfe_r": 0.9,
                "mae_r": 0.4,
                "hold_seconds": 3600,
                "notional_share": 0.20,
            },
        },
    ]


@pytest.fixture
def sample_evaluation():
    """Sample evaluation dict."""
    return {
        "metrics": {
            "dynamic": {
                "calendar_sum_pct": 2.4,
                "trade_count": 2,
                "win_rate": 1.0,
                "avg_win_pct": 0.60,
                "avg_loss_pct": 0.0,
                "payoff_ratio": float("inf"),
                "skip_rate": 0.5,
                "pullback_fill_rate": 1.0,
                "per_trade_quality_bps": 60.0,
            },
            "baseline_a": {
                "calendar_sum_pct": 1.8,
                "trade_count": 4,
                "win_rate": 0.75,
                "avg_win_pct": 0.41,
                "avg_loss_pct": -0.55,
                "payoff_ratio": 0.75,
                "skip_rate": 0.0,
                "pullback_fill_rate": 0.0,
                "per_trade_quality_bps": 17.0,
            },
            "baseline_b": {"calendar_sum_pct": 1.5},
            "baseline_c": {"calendar_sum_pct": 1.2},
        },
        "overfitting_flag": False,
        "train_calendar_sum": 3.0,
        "test_calendar_sum": 2.4,
        "symbol_metrics": {
            "BTCUSDT": {
                "calendar_sum_pct": 0.8,
                "win_rate": 1.0,
                "trade_count": 1,
                "negative_flag": False,
            },
            "ETHUSDT": {
                "calendar_sum_pct": 1.6,
                "win_rate": 1.0,
                "trade_count": 1,
                "negative_flag": False,
            },
        },
    }


@pytest.fixture
def sample_bootstrap_ci():
    """Sample bootstrap CI results."""
    return {
        "BTCUSDT": {"ci_5th": -1.2, "ci_95th": 4.5, "mean": 1.5},
        "ETHUSDT": {"ci_5th": -0.5, "ci_95th": 3.8, "mean": 1.8},
        "small_sample_warning": True,
    }


@pytest.fixture
def sample_regime_stability():
    """Sample regime stability results."""
    return {
        "train_distribution": {"StrongMomentum": 0.4, "Developing": 0.3, "WeakSignal": 0.3},
        "test_distribution": {"StrongMomentum": 0.35, "Developing": 0.35, "WeakSignal": 0.3},
        "regime_distribution_shift": False,
        "shifted_regimes": [],
        "regime_stats": {},
    }


@pytest.fixture
def sample_sensitivity():
    """Sample sensitivity analysis results."""
    return {
        "high_sensitivity_params": ["max_steps"],
        "sensitivities": {},
    }


class TestGenerateOutputFiles:
    """Tests for generate_output_files function."""

    def test_creates_attribution_csv(
        self,
        tmp_path,
        sample_dynamic_results,
        sample_baseline_a_results,
        sample_evaluation,
        sample_bootstrap_ci,
        sample_regime_stability,
        sample_sensitivity,
    ):
        """Test that attribution CSV is created with correct columns."""
        result = generate_output_files(
            dynamic_results=sample_dynamic_results,
            baseline_a_results=sample_baseline_a_results,
            evaluation=sample_evaluation,
            best_params=TimingParams(),
            bootstrap_ci=sample_bootstrap_ci,
            regime_stability=sample_regime_stability,
            sensitivity=sample_sensitivity,
            output_dir=tmp_path,
        )

        csv_path = tmp_path / "dynamic_timing_attribution.csv"
        assert csv_path.exists()

        df = pd.read_csv(csv_path)
        expected_cols = [
            "event_id", "symbol", "touch_time", "entry_decision", "regime",
            "entry_delay_actual_seconds", "entry_price", "baseline_a_entry_price",
            "pnl_pct", "baseline_a_pnl_pct", "delta_pnl_pct",
        ]
        for col in expected_cols:
            assert col in df.columns, f"Missing column: {col}"

        assert len(df) == 4  # All 4 events

    def test_creates_trades_csv(
        self,
        tmp_path,
        sample_dynamic_results,
        sample_baseline_a_results,
        sample_evaluation,
        sample_bootstrap_ci,
        sample_regime_stability,
        sample_sensitivity,
    ):
        """Test that trades CSV is created with correct columns."""
        generate_output_files(
            dynamic_results=sample_dynamic_results,
            baseline_a_results=sample_baseline_a_results,
            evaluation=sample_evaluation,
            best_params=TimingParams(),
            bootstrap_ci=sample_bootstrap_ci,
            regime_stability=sample_regime_stability,
            sensitivity=sample_sensitivity,
            output_dir=tmp_path,
        )

        csv_path = tmp_path / "dynamic_timing_trades.csv"
        assert csv_path.exists()

        df = pd.read_csv(csv_path)
        expected_cols = [
            "event_id", "symbol", "side", "touch_time", "entry_decision",
            "regime", "entry_time", "exit_time", "entry_p", "exit_p",
            "pnl_pct", "realistic_pnl_pct", "exit_reason", "mfe_r",
            "mae_r", "hold_seconds", "notional_share", "decision_path",
        ]
        for col in expected_cols:
            assert col in df.columns, f"Missing column: {col}"

        # Only events with trades (2 out of 4)
        assert len(df) == 2

    def test_creates_summary_json(
        self,
        tmp_path,
        sample_dynamic_results,
        sample_baseline_a_results,
        sample_evaluation,
        sample_bootstrap_ci,
        sample_regime_stability,
        sample_sensitivity,
    ):
        """Test that summary JSON is created with all required fields."""
        generate_output_files(
            dynamic_results=sample_dynamic_results,
            baseline_a_results=sample_baseline_a_results,
            evaluation=sample_evaluation,
            best_params=TimingParams(),
            bootstrap_ci=sample_bootstrap_ci,
            regime_stability=sample_regime_stability,
            sensitivity=sample_sensitivity,
            output_dir=tmp_path,
        )

        json_path = tmp_path / "dynamic_timing_summary.json"
        assert json_path.exists()

        with open(json_path) as f:
            summary = json.load(f)

        assert "metrics" in summary
        assert "overfitting_flag" in summary
        assert "train_calendar_sum" in summary
        assert "test_calendar_sum" in summary
        assert "symbol_metrics" in summary
        assert "best_params" in summary
        assert "bootstrap_ci" in summary
        assert "regime_stability" in summary
        assert "high_sensitivity_params" in summary
        assert "data_gap_events" in summary
        assert "pullback_benefit_marginal" in summary

    def test_data_gap_events_detected(
        self,
        tmp_path,
        sample_dynamic_results,
        sample_baseline_a_results,
        sample_evaluation,
        sample_bootstrap_ci,
        sample_regime_stability,
        sample_sensitivity,
    ):
        """Test that events with > 10s consecutive data gaps are detected."""
        result = generate_output_files(
            dynamic_results=sample_dynamic_results,
            baseline_a_results=sample_baseline_a_results,
            evaluation=sample_evaluation,
            best_params=TimingParams(),
            bootstrap_ci=sample_bootstrap_ci,
            regime_stability=sample_regime_stability,
            sensitivity=sample_sensitivity,
            output_dir=tmp_path,
        )

        # evt_004 has 3 consecutive DataMissing steps (15s > 10s)
        assert "evt_004" in result["data_gap_events"]
        # evt_001, evt_002, evt_003 should NOT be flagged
        assert "evt_001" not in result["data_gap_events"]
        assert "evt_002" not in result["data_gap_events"]
        assert "evt_003" not in result["data_gap_events"]

    def test_pullback_benefit_marginal_flag(
        self,
        tmp_path,
        sample_dynamic_results,
        sample_baseline_a_results,
        sample_evaluation,
        sample_bootstrap_ci,
        sample_regime_stability,
        sample_sensitivity,
    ):
        """Test pullback_benefit_marginal flag when median improvement < 5 bps."""
        result = generate_output_files(
            dynamic_results=sample_dynamic_results,
            baseline_a_results=sample_baseline_a_results,
            evaluation=sample_evaluation,
            best_params=TimingParams(),
            bootstrap_ci=sample_bootstrap_ci,
            regime_stability=sample_regime_stability,
            sensitivity=sample_sensitivity,
            output_dir=tmp_path,
        )

        # evt_002 is wait_pullback with trade, short side
        # entry_price=2250, baseline_a_entry_price=2248
        # For short: improvement = (2250 - 2248) / 2248 * 10000 ≈ 8.9 bps
        # Median of [8.9] = 8.9 > 5 → pullback_benefit_marginal = False
        assert result["pullback_benefit_marginal"] is False

    def test_attribution_delta_pnl_calculation(
        self,
        tmp_path,
        sample_dynamic_results,
        sample_baseline_a_results,
        sample_evaluation,
        sample_bootstrap_ci,
        sample_regime_stability,
        sample_sensitivity,
    ):
        """Test that delta_pnl_pct is correctly calculated."""
        generate_output_files(
            dynamic_results=sample_dynamic_results,
            baseline_a_results=sample_baseline_a_results,
            evaluation=sample_evaluation,
            best_params=TimingParams(),
            bootstrap_ci=sample_bootstrap_ci,
            regime_stability=sample_regime_stability,
            sensitivity=sample_sensitivity,
            output_dir=tmp_path,
        )

        df = pd.read_csv(tmp_path / "dynamic_timing_attribution.csv")

        # evt_001: dynamic pnl = 0.0040 * 100 = 0.40, baseline_a pnl = 0.0037 * 100 = 0.37
        # delta = 0.40 - 0.37 = 0.03
        evt_001 = df[df["event_id"] == "evt_001"].iloc[0]
        assert abs(evt_001["pnl_pct"] - 0.40) < 0.01
        assert abs(evt_001["baseline_a_pnl_pct"] - 0.37) < 0.01
        assert abs(evt_001["delta_pnl_pct"] - 0.03) < 0.01

    def test_skipped_events_have_null_pnl(
        self,
        tmp_path,
        sample_dynamic_results,
        sample_baseline_a_results,
        sample_evaluation,
        sample_bootstrap_ci,
        sample_regime_stability,
        sample_sensitivity,
    ):
        """Test that skipped events have null pnl in attribution."""
        generate_output_files(
            dynamic_results=sample_dynamic_results,
            baseline_a_results=sample_baseline_a_results,
            evaluation=sample_evaluation,
            best_params=TimingParams(),
            bootstrap_ci=sample_bootstrap_ci,
            regime_stability=sample_regime_stability,
            sensitivity=sample_sensitivity,
            output_dir=tmp_path,
        )

        df = pd.read_csv(tmp_path / "dynamic_timing_attribution.csv")

        # evt_003 is skipped → pnl_pct should be NaN
        evt_003 = df[df["event_id"] == "evt_003"].iloc[0]
        assert pd.isna(evt_003["pnl_pct"])

    def test_output_dir_created_if_not_exists(
        self,
        tmp_path,
        sample_dynamic_results,
        sample_baseline_a_results,
        sample_evaluation,
        sample_bootstrap_ci,
        sample_regime_stability,
        sample_sensitivity,
    ):
        """Test that output directory is created if it doesn't exist."""
        nested_dir = tmp_path / "nested" / "output" / "dir"
        assert not nested_dir.exists()

        generate_output_files(
            dynamic_results=sample_dynamic_results,
            baseline_a_results=sample_baseline_a_results,
            evaluation=sample_evaluation,
            best_params=TimingParams(),
            bootstrap_ci=sample_bootstrap_ci,
            regime_stability=sample_regime_stability,
            sensitivity=sample_sensitivity,
            output_dir=nested_dir,
        )

        assert nested_dir.exists()
        assert (nested_dir / "dynamic_timing_attribution.csv").exists()
        assert (nested_dir / "dynamic_timing_trades.csv").exists()
        assert (nested_dir / "dynamic_timing_summary.json").exists()
