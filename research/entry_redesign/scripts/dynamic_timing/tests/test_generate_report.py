"""Tests for generate_report() function."""

from pathlib import Path

import pytest

from research.entry_redesign.scripts.dynamic_timing.dynamic_entry_timing_runner import (
    generate_report,
)
from research.entry_redesign.scripts.dynamic_timing.regime_classifier import TimingParams


@pytest.fixture
def best_params():
    return TimingParams(
        max_steps=4,
        strong_momentum_threshold=0.15,
        strong_flow_threshold=0.58,
        moderate_momentum_threshold=0.08,
        extension_threshold=0.15,
        weak_flow_threshold=0.48,
        fading_threshold=0.02,
        min_steps_for_skip=3,
        pullback_target_atr=0.05,
        decision_window_seconds=60,
    )


@pytest.fixture
def bootstrap_ci():
    return {
        "btc_ci": {"p5": 1.0, "p95": 5.0, "mean": 3.0},
        "eth_ci": {"p5": 0.5, "p95": 4.0, "mean": 2.0},
        "combined_ci": {"p5": 2.0, "p95": 8.0, "mean": 5.0},
        "small_sample_warning": True,
    }


@pytest.fixture
def regime_stability():
    return {
        "train_distribution": {"StrongMomentum": 0.3, "Developing": 0.5, "WeakSignal": 0.2},
        "test_distribution": {"StrongMomentum": 0.35, "Developing": 0.45, "WeakSignal": 0.2},
        "regime_distribution_shift": False,
        "shifted_regimes": [],
        "regime_stats": {
            "StrongMomentum": {
                "count": 40,
                "trade_count": 40,
                "win_rate": 0.75,
                "avg_pnl_pct": 0.15,
                "calendar_contribution_pct": 3.5,
            },
            "Developing": {
                "count": 50,
                "trade_count": 50,
                "win_rate": 0.60,
                "avg_pnl_pct": 0.05,
                "calendar_contribution_pct": 1.0,
            },
            "WeakSignal": {
                "count": 20,
                "trade_count": 0,
                "win_rate": 0.0,
                "avg_pnl_pct": 0.0,
                "calendar_contribution_pct": 0.0,
            },
        },
    }


@pytest.fixture
def sensitivity_no_high():
    return {
        "sensitivities": {
            "max_steps": {"calendar_sum_pct": [4.0, 4.5, 4.2, 4.1]},
            "strong_momentum_threshold": {"calendar_sum_pct": [3.8, 4.5, 4.3, 4.0]},
        },
        "high_sensitivity_params": [],
    }


@pytest.fixture
def sensitivity_with_high():
    return {
        "sensitivities": {
            "max_steps": {"calendar_sum_pct": [4.0, 4.5, 4.2, 4.1]},
        },
        "high_sensitivity_params": ["max_steps", "extension_threshold"],
    }


@pytest.fixture
def output_files_normal():
    return {
        "data_gap_events": [],
        "pullback_benefit_marginal": False,
    }


def _make_evaluation(dynamic_cal, baseline_a_cal, overfitting=False):
    """Helper to build evaluation dict."""
    return {
        "metrics": {
            "dynamic": {
                "calendar_sum_pct": dynamic_cal,
                "trade_count": 30,
                "win_rate": 0.65,
                "avg_win_pct": 0.20,
                "avg_loss_pct": -0.10,
                "payoff_ratio": 2.0,
                "skip_rate": 0.15,
                "pullback_fill_rate": 0.60,
                "per_trade_quality_bps": 8.0,
            },
            "baseline_a": {
                "calendar_sum_pct": baseline_a_cal,
                "trade_count": 35,
                "win_rate": 0.60,
                "avg_win_pct": 0.18,
                "avg_loss_pct": -0.12,
                "payoff_ratio": 1.5,
                "skip_rate": 0.0,
                "pullback_fill_rate": 0.0,
                "per_trade_quality_bps": 5.0,
            },
            "baseline_b": {
                "calendar_sum_pct": 2.0,
                "trade_count": 35,
                "win_rate": 0.55,
                "avg_win_pct": 0.15,
                "avg_loss_pct": -0.14,
                "payoff_ratio": 1.1,
                "skip_rate": 0.0,
                "pullback_fill_rate": 0.0,
                "per_trade_quality_bps": 3.0,
            },
            "baseline_c": {
                "calendar_sum_pct": 2.5,
                "trade_count": 35,
                "win_rate": 0.58,
                "avg_win_pct": 0.16,
                "avg_loss_pct": -0.13,
                "payoff_ratio": 1.2,
                "skip_rate": 0.0,
                "pullback_fill_rate": 0.0,
                "per_trade_quality_bps": 4.0,
            },
        },
        "overfitting_flag": overfitting,
        "train_calendar_sum": 6.0,
        "test_calendar_sum": dynamic_cal,
        "symbol_metrics": {
            "BTCUSDT": {
                "calendar_sum_pct": dynamic_cal * 0.6,
                "win_rate": 0.70,
                "trade_count": 18,
                "negative_flag": False,
            },
            "ETHUSDT": {
                "calendar_sum_pct": dynamic_cal * 0.4,
                "win_rate": 0.60,
                "trade_count": 12,
                "negative_flag": False,
            },
        },
    }


class TestGoDecision:
    """Test Go/No-Go determination logic."""

    def test_go_decision(
        self, best_params, bootstrap_ci, regime_stability, sensitivity_no_high, output_files_normal, tmp_path
    ):
        """Go: improvement > 1.0% AND no overfitting AND no high_sensitivity."""
        evaluation = _make_evaluation(dynamic_cal=5.5, baseline_a_cal=3.0)
        report = generate_report(
            evaluation, best_params, bootstrap_ci, regime_stability,
            sensitivity_no_high, output_files_normal, tmp_path,
        )
        assert "✅ Go（推荐采用动态 timing）" in report
        assert "进入 design 阶段" in report

    def test_conditional_go_small_improvement(
        self, best_params, bootstrap_ci, regime_stability, sensitivity_no_high, output_files_normal, tmp_path
    ):
        """Conditional Go: improvement > 0 but <= 1.0%."""
        evaluation = _make_evaluation(dynamic_cal=3.5, baseline_a_cal=3.0)
        report = generate_report(
            evaluation, best_params, bootstrap_ci, regime_stability,
            sensitivity_no_high, output_files_normal, tmp_path,
        )
        assert "Conditional Go" in report
        assert "扩大样本量验证" in report

    def test_conditional_go_high_sensitivity(
        self, best_params, bootstrap_ci, regime_stability, sensitivity_with_high, output_files_normal, tmp_path
    ):
        """Conditional Go: improvement > 1% but has high_sensitivity_params."""
        evaluation = _make_evaluation(dynamic_cal=5.5, baseline_a_cal=3.0)
        report = generate_report(
            evaluation, best_params, bootstrap_ci, regime_stability,
            sensitivity_with_high, output_files_normal, tmp_path,
        )
        assert "Conditional Go" in report
        assert "max_steps" in report
        assert "extension_threshold" in report

    def test_no_go_not_superior(
        self, best_params, bootstrap_ci, regime_stability, sensitivity_no_high, output_files_normal, tmp_path
    ):
        """No-Go: improvement <= 0."""
        evaluation = _make_evaluation(dynamic_cal=2.5, baseline_a_cal=3.0)
        report = generate_report(
            evaluation, best_params, bootstrap_ci, regime_stability,
            sensitivity_no_high, output_files_normal, tmp_path,
        )
        assert "No-Go" in report
        assert "dynamic_timing_not_superior" in report
        assert "回退到" in report

    def test_no_go_overfitting(
        self, best_params, bootstrap_ci, regime_stability, sensitivity_no_high, output_files_normal, tmp_path
    ):
        """No-Go: overfitting_flag=true."""
        evaluation = _make_evaluation(dynamic_cal=5.5, baseline_a_cal=3.0, overfitting=True)
        report = generate_report(
            evaluation, best_params, bootstrap_ci, regime_stability,
            sensitivity_no_high, output_files_normal, tmp_path,
        )
        assert "No-Go" in report
        assert "overfitting_flag=true" in report


class TestReportSections:
    """Test that all required sections are present."""

    def test_all_sections_present(
        self, best_params, bootstrap_ci, regime_stability, sensitivity_no_high, output_files_normal, tmp_path
    ):
        """Report must contain all 8 sections."""
        evaluation = _make_evaluation(dynamic_cal=5.5, baseline_a_cal=3.0)
        report = generate_report(
            evaluation, best_params, bootstrap_ci, regime_stability,
            sensitivity_no_high, output_files_normal, tmp_path,
        )
        assert "## 1. 实验设计" in report
        assert "## 2. 结果对比" in report
        assert "## 3. Regime 分析" in report
        assert "## 4. 参数敏感性" in report
        assert "## 5. Bootstrap 置信区间" in report
        assert "## 6. 与 V6 Baseline 差距分析" in report
        assert "## 7. Go/No-Go 判定" in report
        assert "## 8. 下一步行动建议" in report

    def test_v6_baseline_gap_analysis(
        self, best_params, bootstrap_ci, regime_stability, sensitivity_no_high, output_files_normal, tmp_path
    ):
        """Report must include V6 baseline (33.02%) gap analysis."""
        evaluation = _make_evaluation(dynamic_cal=5.5, baseline_a_cal=3.0)
        report = generate_report(
            evaluation, best_params, bootstrap_ci, regime_stability,
            sensitivity_no_high, output_files_normal, tmp_path,
        )
        assert "33.02" in report
        assert "差距" in report
        assert "Reentry-trigger" in report

    def test_report_written_to_file(
        self, best_params, bootstrap_ci, regime_stability, sensitivity_no_high, output_files_normal, tmp_path
    ):
        """Report must be written to output_dir/dynamic_timing_report.md."""
        evaluation = _make_evaluation(dynamic_cal=5.5, baseline_a_cal=3.0)
        report = generate_report(
            evaluation, best_params, bootstrap_ci, regime_stability,
            sensitivity_no_high, output_files_normal, tmp_path,
        )
        report_path = tmp_path / "dynamic_timing_report.md"
        assert report_path.exists()
        content = report_path.read_text(encoding="utf-8")
        assert content == report

    def test_returns_string(
        self, best_params, bootstrap_ci, regime_stability, sensitivity_no_high, output_files_normal, tmp_path
    ):
        """generate_report must return the report content as a string."""
        evaluation = _make_evaluation(dynamic_cal=5.5, baseline_a_cal=3.0)
        result = generate_report(
            evaluation, best_params, bootstrap_ci, regime_stability,
            sensitivity_no_high, output_files_normal, tmp_path,
        )
        assert isinstance(result, str)
        assert len(result) > 0


class TestFlags:
    """Test flag annotations in the report."""

    def test_pullback_benefit_marginal_flag(
        self, best_params, bootstrap_ci, regime_stability, sensitivity_no_high, tmp_path
    ):
        """pullback_benefit_marginal flag should appear when set."""
        evaluation = _make_evaluation(dynamic_cal=5.5, baseline_a_cal=3.0)
        output_files = {"pullback_benefit_marginal": True, "data_gap_events": []}
        report = generate_report(
            evaluation, best_params, bootstrap_ci, regime_stability,
            sensitivity_no_high, output_files, tmp_path,
        )
        assert "pullback_benefit_marginal=true" in report

    def test_small_sample_warning_flag(
        self, best_params, bootstrap_ci, regime_stability, sensitivity_no_high, output_files_normal, tmp_path
    ):
        """small_sample_warning flag should appear."""
        evaluation = _make_evaluation(dynamic_cal=5.5, baseline_a_cal=3.0)
        report = generate_report(
            evaluation, best_params, bootstrap_ci, regime_stability,
            sensitivity_no_high, output_files_normal, tmp_path,
        )
        assert "small_sample_warning=true" in report

    def test_regime_distribution_shift_flag(
        self, best_params, bootstrap_ci, sensitivity_no_high, output_files_normal, tmp_path
    ):
        """regime_distribution_shift flag should appear when shift detected."""
        evaluation = _make_evaluation(dynamic_cal=5.5, baseline_a_cal=3.0)
        regime_stability = {
            "train_distribution": {"StrongMomentum": 0.5, "Developing": 0.5},
            "test_distribution": {"StrongMomentum": 0.2, "Developing": 0.8},
            "regime_distribution_shift": True,
            "shifted_regimes": ["StrongMomentum"],
            "regime_stats": {},
        }
        report = generate_report(
            evaluation, best_params, bootstrap_ci, regime_stability,
            sensitivity_no_high, output_files_normal, tmp_path,
        )
        assert "regime_distribution_shift=true" in report
        assert "StrongMomentum" in report

    def test_payoff_ratio_inf_handling(
        self, best_params, bootstrap_ci, regime_stability, sensitivity_no_high, output_files_normal, tmp_path
    ):
        """payoff_ratio=inf should be formatted gracefully (not crash)."""
        evaluation = _make_evaluation(dynamic_cal=5.5, baseline_a_cal=3.0)
        # Set payoff_ratio to inf (happens when avg_loss_pct == 0)
        evaluation["metrics"]["dynamic"]["payoff_ratio"] = float("inf")
        evaluation["metrics"]["baseline_b"]["payoff_ratio"] = float("inf")
        report = generate_report(
            evaluation, best_params, bootstrap_ci, regime_stability,
            sensitivity_no_high, output_files_normal, tmp_path,
        )
        # Should contain ∞ symbol instead of crashing
        assert "∞" in report
        assert isinstance(report, str)
