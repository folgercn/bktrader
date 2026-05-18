"""Unit tests for report_generator module — Go/No-Go decision logic and report generation.

Tests cover:
- Go/No-Go decision boundary conditions (Requirements 8.1, 8.2, 8.7)
- Overfitting downgrade logic
- Output file list completeness
"""

from __future__ import annotations

import numpy as np
import pandas as pd
import pytest
from pathlib import Path

from timing_probability_unified.report_generator import (
    GoNoGoDecision,
    compute_go_no_go_decision,
    generate_report,
)


# ---------------------------------------------------------------------------
# Tests for compute_go_no_go_decision
# ---------------------------------------------------------------------------


class TestComputeGoNoGoDecision:
    """Tests for compute_go_no_go_decision() boundary conditions.

    Validates: Requirements 8.1, 8.2, 8.7
    """

    def test_strong_go_all_conditions_met(self):
        """Strong Go: calendar_sum >= 10%, worst_sm > -0.5%, BTC/ETH positive, forward >= 7%.

        Validates: Requirements 8.2
        """
        result = compute_go_no_go_decision(
            calendar_sum=0.12,           # 12% >= 10%
            worst_sm=-0.003,             # -0.3% > -0.5%
            btc_calendar_sum=0.03,       # positive
            eth_calendar_sum=0.05,       # positive
            forward_calendar_sum=0.08,   # 8% >= 7%
            overfitting_flag=False,
        )

        assert result.decision == "strong_go"
        assert result.btc_positive is True
        assert result.eth_positive is True
        assert result.overfitting_downgrade is False

    def test_no_go_calendar_sum_below_7pct(self):
        """No-Go: calendar_sum < 7% → "no_go".

        Validates: Requirements 8.2
        """
        result = compute_go_no_go_decision(
            calendar_sum=0.05,           # 5% < 7%
            worst_sm=-0.003,             # fine
            btc_calendar_sum=0.02,
            eth_calendar_sum=0.03,
            forward_calendar_sum=0.08,
            overfitting_flag=False,
        )

        assert result.decision == "no_go"

    def test_no_go_worst_sm_below_minus_1pct(self):
        """No-Go: worst_sm < -1.0% → "no_go".

        Validates: Requirements 8.2
        """
        result = compute_go_no_go_decision(
            calendar_sum=0.12,           # 12% >= 10%
            worst_sm=-0.015,             # -1.5% < -1.0%
            btc_calendar_sum=0.03,
            eth_calendar_sum=0.05,
            forward_calendar_sum=0.08,
            overfitting_flag=False,
        )

        assert result.decision == "no_go"

    def test_marginal_go_calendar_sum_between_7_and_10(self):
        """Marginal Go: calendar_sum in [7%, 10%) → "marginal_go".

        Validates: Requirements 8.2
        """
        result = compute_go_no_go_decision(
            calendar_sum=0.08,           # 8% in [7%, 10%)
            worst_sm=-0.003,             # fine
            btc_calendar_sum=0.03,
            eth_calendar_sum=0.05,
            forward_calendar_sum=0.08,
            overfitting_flag=False,
        )

        assert result.decision == "marginal_go"
        assert result.overfitting_downgrade is False

    def test_overfitting_downgrade_strong_to_marginal(self):
        """Overfitting downgrade: Strong Go conditions + overfitting_flag → "marginal_go".

        When overfitting_flag=True and calendar_sum >= 10%, the decision is
        downgraded from strong_go to marginal_go with overfitting_downgrade=True.

        Validates: Requirements 8.7
        """
        result = compute_go_no_go_decision(
            calendar_sum=0.12,           # 12% >= 10%
            worst_sm=-0.003,             # -0.3% > -0.5%
            btc_calendar_sum=0.03,       # positive
            eth_calendar_sum=0.05,       # positive
            forward_calendar_sum=0.08,   # 8% >= 7%
            overfitting_flag=True,       # overfitting detected!
        )

        assert result.decision == "marginal_go"
        assert result.overfitting_downgrade is True

    def test_btc_negative_prevents_strong_go(self):
        """BTC negative prevents Strong Go → "marginal_go".

        All other Strong Go conditions met, but BTC is negative.

        Validates: Requirements 8.2
        """
        result = compute_go_no_go_decision(
            calendar_sum=0.12,           # 12% >= 10%
            worst_sm=-0.003,             # -0.3% > -0.5%
            btc_calendar_sum=-0.01,      # negative!
            eth_calendar_sum=0.05,       # positive
            forward_calendar_sum=0.08,   # 8% >= 7%
            overfitting_flag=False,
        )

        assert result.decision == "marginal_go"
        assert result.btc_positive is False
        assert result.overfitting_downgrade is False

    def test_eth_negative_prevents_strong_go(self):
        """ETH negative prevents Strong Go → "marginal_go".

        All other Strong Go conditions met, but ETH is negative.

        Validates: Requirements 8.2
        """
        result = compute_go_no_go_decision(
            calendar_sum=0.12,           # 12% >= 10%
            worst_sm=-0.003,             # -0.3% > -0.5%
            btc_calendar_sum=0.03,       # positive
            eth_calendar_sum=-0.02,      # negative!
            forward_calendar_sum=0.08,   # 8% >= 7%
            overfitting_flag=False,
        )

        assert result.decision == "marginal_go"
        assert result.eth_positive is False
        assert result.overfitting_downgrade is False

    def test_forward_below_7pct_prevents_strong_go(self):
        """Forward < 7% prevents Strong Go → "marginal_go".

        All other Strong Go conditions met, but forward_calendar_sum < 7%.

        Validates: Requirements 8.2
        """
        result = compute_go_no_go_decision(
            calendar_sum=0.12,           # 12% >= 10%
            worst_sm=-0.003,             # -0.3% > -0.5%
            btc_calendar_sum=0.03,       # positive
            eth_calendar_sum=0.05,       # positive
            forward_calendar_sum=0.05,   # 5% < 7%
            overfitting_flag=False,
        )

        assert result.decision == "marginal_go"
        assert result.overfitting_downgrade is False

    # --- Boundary value tests ---

    def test_boundary_calendar_sum_exactly_10pct(self):
        """calendar_sum exactly 10% with all other conditions → strong_go."""
        result = compute_go_no_go_decision(
            calendar_sum=0.10,           # exactly 10%
            worst_sm=-0.003,
            btc_calendar_sum=0.01,
            eth_calendar_sum=0.01,
            forward_calendar_sum=0.07,
            overfitting_flag=False,
        )

        assert result.decision == "strong_go"

    def test_boundary_calendar_sum_exactly_7pct(self):
        """calendar_sum exactly 7% → marginal_go (not no_go)."""
        result = compute_go_no_go_decision(
            calendar_sum=0.07,           # exactly 7%
            worst_sm=-0.003,
            btc_calendar_sum=0.01,
            eth_calendar_sum=0.01,
            forward_calendar_sum=0.08,
            overfitting_flag=False,
        )

        assert result.decision == "marginal_go"

    def test_boundary_worst_sm_exactly_minus_0_5pct(self):
        """worst_sm exactly -0.5% → not strong_go (requires > -0.5%).

        worst_sm = -0.005 is NOT > -0.005, so strong_go condition fails.
        """
        result = compute_go_no_go_decision(
            calendar_sum=0.12,
            worst_sm=-0.005,             # exactly -0.5%
            btc_calendar_sum=0.03,
            eth_calendar_sum=0.05,
            forward_calendar_sum=0.08,
            overfitting_flag=False,
        )

        # worst_sm > -0.005 fails (it's equal, not greater)
        assert result.decision == "marginal_go"

    def test_boundary_worst_sm_exactly_minus_1pct(self):
        """worst_sm exactly -1.0% → not no_go (requires < -1.0%).

        worst_sm = -0.01 is NOT < -0.01, so no_go condition fails.
        """
        result = compute_go_no_go_decision(
            calendar_sum=0.08,
            worst_sm=-0.01,              # exactly -1.0%
            btc_calendar_sum=0.03,
            eth_calendar_sum=0.05,
            forward_calendar_sum=0.08,
            overfitting_flag=False,
        )

        # worst_sm < -0.01 fails (it's equal, not less)
        assert result.decision == "marginal_go"

    def test_boundary_forward_exactly_7pct(self):
        """forward_calendar_sum exactly 7% → strong_go condition met (>= 7%)."""
        result = compute_go_no_go_decision(
            calendar_sum=0.12,
            worst_sm=-0.003,
            btc_calendar_sum=0.03,
            eth_calendar_sum=0.05,
            forward_calendar_sum=0.07,   # exactly 7%
            overfitting_flag=False,
        )

        assert result.decision == "strong_go"

    def test_overfitting_flag_with_calendar_below_10pct_no_downgrade(self):
        """overfitting_flag=True but calendar_sum < 10% → no downgrade applied.

        The overfitting downgrade only applies when calendar_sum >= 10%.
        """
        result = compute_go_no_go_decision(
            calendar_sum=0.08,           # 8% < 10%
            worst_sm=-0.003,
            btc_calendar_sum=0.03,
            eth_calendar_sum=0.05,
            forward_calendar_sum=0.05,
            overfitting_flag=True,       # flag set but cs < 10%
        )

        # Should be marginal_go due to cs in [7%, 10%), NOT due to overfitting downgrade
        assert result.decision == "marginal_go"
        assert result.overfitting_downgrade is False

    def test_result_fields_populated_correctly(self):
        """GoNoGoDecision fields are populated with input values."""
        result = compute_go_no_go_decision(
            calendar_sum=0.12,
            worst_sm=-0.003,
            btc_calendar_sum=0.03,
            eth_calendar_sum=0.05,
            forward_calendar_sum=0.08,
            overfitting_flag=False,
        )

        assert result.calendar_sum == 0.12
        assert result.worst_sm == -0.003
        assert result.forward_calendar_sum == 0.08
        assert isinstance(result, GoNoGoDecision)


# ---------------------------------------------------------------------------
# Tests for generate_report output file completeness
# ---------------------------------------------------------------------------


class TestGenerateReportOutputFiles:
    """Tests for generate_report() output file completeness.

    Validates: Requirements 8.1
    """

    @pytest.fixture
    def output_dir(self, tmp_path):
        """Create a temporary output directory."""
        return tmp_path / "output"

    @pytest.fixture
    def minimal_inputs(self):
        """Create minimal valid inputs for generate_report."""
        from timing_probability_unified.timing_classifier import TimingClassifierResult
        from timing_probability_unified.probability_model import RFProbabilityResult
        from timing_probability_unified.speed_gate import SpeedGateResult
        from timing_probability_unified.robustness import (
            AblationRow,
            BootstrapResult,
            ForwardResult,
            RobustnessResult,
        )
        from timing_probability_unified.event_source_builder import EventPoolStats
        from timing_probability_unified.combined_executor import SensitivityRow

        trades = pd.DataFrame(
            {
                "event_id": ["evt_0", "evt_1"],
                "symbol": ["BTCUSDT", "ETHUSDT"],
                "side": ["long", "long"],
                "touch_time": pd.date_range("2025-01-01", periods=2, freq="1D", tz="UTC"),
                "timing_prediction": ["fast", "slow"],
                "selected_delay": ["D0", "D10"],
                "rf_probability": [0.6, 0.7],
                "sizing_multiplier": [1.2, 1.4],
                "position_size": [0.36, 0.42],
                "delay_pnl_pct": [0.01, 0.005],
                "weighted_pnl": [0.0036, 0.0021],
                "speed_300s_atr": [0.5, 0.6],
                "speed_gate_pass": [True, True],
            }
        )

        timing_result = TimingClassifierResult(
            selected_depth=3,
            dt3_loocv_calendar_sum=0.08,
            dt4_loocv_calendar_sum=0.07,
            test_calendar_sum=0.09,
            regime_distribution={"skip": 5, "fast": 10, "slow": 8},
            rules_text="IF feature_1 <= 0.5 THEN fast\nELSE slow",
            classifier=None,
            train_predictions=np.array(["fast", "slow"]),
            test_predictions=np.array(["fast", "slow"]),
        )

        rf_result = RFProbabilityResult(
            train_auc=0.60,
            test_auc=0.55,
            feature_importance_top5=[
                ("signal_atr_percentile", 0.15),
                ("roundtrip_cost_atr", 0.12),
                ("prev1_body_atr", 0.10),
                ("prev1_range_atr", 0.09),
                ("prev1_close_pos_side", 0.08),
            ],
            prob_mean=0.52,
            prob_median=0.50,
            prob_std=0.15,
            rf_no_signal_warning=False,
            model=None,
            train_probabilities=np.array([0.6, 0.7]),
            test_probabilities=np.array([0.6, 0.7]),
        )

        speed_gate_result = SpeedGateResult(
            threshold=0.3,
            gate_pass_rate=0.85,
            gate_on_calendar_sum=0.10,
            gate_off_calendar_sum=0.08,
            gate_on_worst_sm=-0.003,
            gate_off_worst_sm=-0.005,
            filtered_avg_pnl=-0.001,
            retained_avg_pnl=0.005,
            aggressive_gate_warning=False,
        )

        bootstrap = BootstrapResult(
            calendar_sum_mean=0.10,
            calendar_sum_ci_5=0.05,
            calendar_sum_ci_95=0.15,
            ci_width=0.10,
            btc_calendar_sum=0.04,
            btc_ci_5=0.01,
            btc_ci_95=0.07,
            eth_calendar_sum=0.06,
            eth_ci_5=0.02,
            eth_ci_95=0.10,
        )

        forward = ForwardResult(
            forward_calendar_sum=0.08,
            forward_worst_sm=-0.002,
            forward_trade_count=50,
            overfitting_flag=False,
            forward_risk_flag=False,
            forward_underperformance=False,
        )

        ablation = [
            AblationRow(config_name="timing_only", calendar_sum=0.07, worst_sm=-0.004, trade_count=80, avg_pnl_per_trade=0.001),
            AblationRow(config_name="probability_only", calendar_sum=0.06, worst_sm=-0.005, trade_count=100, avg_pnl_per_trade=0.0008),
            AblationRow(config_name="no_speed_gate", calendar_sum=0.08, worst_sm=-0.006, trade_count=90, avg_pnl_per_trade=0.001),
            AblationRow(config_name="full_unified", calendar_sum=0.10, worst_sm=-0.003, trade_count=75, avg_pnl_per_trade=0.0015),
        ]

        robustness_result = RobustnessResult(
            bootstrap=bootstrap,
            forward=forward,
            ablation=ablation,
        )

        sensitivity_rows = [
            SensitivityRow(base_share=0.25, calendar_sum=0.08, worst_sm=-0.003, trade_count=75, avg_pnl_per_trade=0.001),
            SensitivityRow(base_share=0.30, calendar_sum=0.10, worst_sm=-0.004, trade_count=75, avg_pnl_per_trade=0.0013),
            SensitivityRow(base_share=0.35, calendar_sum=0.11, worst_sm=-0.005, trade_count=75, avg_pnl_per_trade=0.0015),
            SensitivityRow(base_share=0.40, calendar_sum=0.12, worst_sm=-0.006, trade_count=75, avg_pnl_per_trade=0.0016),
        ]

        event_pool_stats = EventPoolStats(
            total_events=500,
            btc_count=200,
            eth_count=300,
            btc_pct=40.0,
            eth_pct=60.0,
            long_count=260,
            short_count=240,
            long_pct=52.0,
            short_pct=48.0,
            earliest_touch_time=pd.Timestamp("2024-01-01", tz="UTC"),
            latest_touch_time=pd.Timestamp("2025-10-31", tz="UTC"),
            small_pool_warning=False,
        )

        events_pool = pd.DataFrame(
            {
                "event_id": ["evt_0", "evt_1"],
                "symbol": ["BTCUSDT", "ETHUSDT"],
                "side": ["long", "short"],
                "touch_time": pd.date_range("2025-01-01", periods=2, freq="1D", tz="UTC"),
            }
        )

        execution_stats = {
            "D0_traded_rate": 0.95,
            "D5_traded_rate": 0.90,
            "D10_traded_rate": 0.85,
            "D15_traded_rate": 0.80,
            "pullback_traded_rate": 0.60,
        }

        return {
            "trades": trades,
            "timing_result": timing_result,
            "rf_result": rf_result,
            "speed_gate_result": speed_gate_result,
            "robustness_result": robustness_result,
            "sensitivity_rows": sensitivity_rows,
            "event_pool_stats": event_pool_stats,
            "events_pool": events_pool,
            "execution_stats": execution_stats,
        }

    def test_all_13_output_files_created(self, output_dir, minimal_inputs):
        """generate_report() produces all 13 expected output files.

        Validates: Requirements 8.1
        """
        decision = generate_report(output_dir=output_dir, **minimal_inputs)

        expected_files = [
            "unified_report.md",
            "unified_summary.json",
            "unified_trades.csv",
            "timing_classifier_results.json",
            "rf_probability_results.json",
            "speed_gate_analysis.json",
            "execution_stats.json",
            "ablation_results.json",
            "bootstrap_results.json",
            "sensitivity_analysis.json",
            "events_pool_stats.json",
            "events_pool.csv",
            "timing_rules.md",
        ]

        for filename in expected_files:
            filepath = output_dir / filename
            assert filepath.exists(), f"Missing output file: {filename}"
            assert filepath.stat().st_size > 0, f"Empty output file: {filename}"

    def test_returns_go_no_go_decision(self, output_dir, minimal_inputs):
        """generate_report() returns a GoNoGoDecision instance."""
        decision = generate_report(output_dir=output_dir, **minimal_inputs)

        assert isinstance(decision, GoNoGoDecision)
        assert decision.decision in ("strong_go", "marginal_go", "no_go")

    def test_output_dir_created_if_not_exists(self, tmp_path, minimal_inputs):
        """generate_report() creates output directory if it doesn't exist."""
        nested_dir = tmp_path / "deep" / "nested" / "output"
        assert not nested_dir.exists()

        generate_report(output_dir=nested_dir, **minimal_inputs)

        assert nested_dir.exists()
        assert (nested_dir / "unified_report.md").exists()
