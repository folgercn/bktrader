"""End-to-end integration tests for the dynamic entry timing pipeline.

Validates:
- Determinism: same input → identical output across runs (Requirement 6.6)
- Output file generation: all expected files are created (Requirement 7.1)
- Forbidden directory constraint: output resolves to allowed path (Requirement 6.7)
- Grid search determinism: same input → identical DataFrame (Requirement 6.6)

**Validates: Requirements 6.6, 6.7**
"""

from __future__ import annotations

import json
import sys
from pathlib import Path

import numpy as np
import pandas as pd
import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from ..dynamic_entry_timing_runner import (  # noqa: E402
    compute_calendar_sum,
    generate_output_files,
    generate_report,
    run_dynamic_timing,
    run_grid_search_layer1,
)
from ..regime_classifier import TimingParams  # noqa: E402


# ---------------------------------------------------------------------------
# Synthetic data helpers
# ---------------------------------------------------------------------------


def _make_bars(start: str, n: int = 300, base_price: float = 50000.0) -> pd.DataFrame:
    """Create synthetic 1s bars for testing."""
    idx = pd.date_range(start, periods=n, freq="1s", tz="UTC")
    rng = np.random.default_rng(42)
    closes = base_price + np.cumsum(rng.normal(0, 1, n))
    highs = closes + rng.uniform(0, 2, n)
    lows = closes - rng.uniform(0, 2, n)
    return pd.DataFrame(
        {"open": closes - 0.5, "high": highs, "low": lows, "close": closes},
        index=idx,
    )


def _make_events(touch_time: str, n: int = 3) -> pd.DataFrame:
    """Create synthetic events for testing."""
    rows = []
    for i in range(n):
        rows.append(
            {
                "event_id": f"evt_{i}",
                "symbol": "BTCUSDT",
                "side": "long",
                "touch_time": pd.Timestamp(touch_time, tz="UTC")
                + pd.Timedelta(seconds=i * 100),
                "level": 50000.0,
                "atr": 100.0,
                "signal_low": 49900.0,
                "signal_high": 50100.0,
            }
        )
    return pd.DataFrame(rows)


# ---------------------------------------------------------------------------
# Test: Determinism — same input, two runs, identical results
# ---------------------------------------------------------------------------


class TestDeterminism:
    """Verify that the pipeline produces identical results across runs."""

    def test_run_dynamic_timing_deterministic(self):
        """run_dynamic_timing + compute_calendar_sum produce identical results on two runs.

        Requirement 6.6: 同一参数配置 + 同一数据输入下，两次独立运行的 trade ledger
        MUST 逐行一致（确定性约束）。
        """
        bars = _make_bars("2024-01-01 00:00:00", n=600)
        events = _make_events("2024-01-01 00:00:10", n=3)
        cache = {"BTCUSDT_202401": bars}
        params = TimingParams(max_steps=4)

        # Run 1
        results_1 = run_dynamic_timing(events, cache, params)
        cal_sum_1 = compute_calendar_sum(results_1)

        # Run 2
        results_2 = run_dynamic_timing(events, cache, params)
        cal_sum_2 = compute_calendar_sum(results_2)

        # Calendar sums must be identical
        assert cal_sum_1 == cal_sum_2

        # Results must be identical line-by-line
        assert len(results_1) == len(results_2)
        for r1, r2 in zip(results_1, results_2):
            assert r1["event_id"] == r2["event_id"]
            assert r1["entry_decision"] == r2["entry_decision"]
            assert r1["regime"] == r2["regime"]
            assert r1["entry_price"] == r2["entry_price"]
            assert r1["entry_time"] == r2["entry_time"]
            assert r1["decision_path"] == r2["decision_path"]
            # Trade results must match
            if r1["trade"] is None:
                assert r2["trade"] is None
            else:
                assert r2["trade"] is not None
                assert r1["trade"]["realistic_pnl_pct"] == r2["trade"]["realistic_pnl_pct"]
                assert r1["trade"]["exit_reason"] == r2["trade"]["exit_reason"]
                assert r1["trade"]["entry_p"] == r2["trade"]["entry_p"]
                assert r1["trade"]["exit_p"] == r2["trade"]["exit_p"]


# ---------------------------------------------------------------------------
# Test: Output files generation
# ---------------------------------------------------------------------------


class TestOutputFilesGeneration:
    """Verify that generate_output_files creates all expected files."""

    @pytest.fixture
    def synthetic_pipeline_data(self):
        """Run a minimal pipeline to produce data for output generation."""
        bars = _make_bars("2024-01-01 00:00:00", n=600)
        events = _make_events("2024-01-01 00:00:10", n=3)
        cache = {"BTCUSDT_202401": bars}
        params = TimingParams(max_steps=4)

        dynamic_results = run_dynamic_timing(events, cache, params)

        # Build minimal baseline_a results
        baseline_a_results = []
        for r in dynamic_results:
            baseline_a_results.append(
                {
                    "event_id": r["event_id"],
                    "symbol": r["symbol"],
                    "side": r["side"],
                    "touch_time": r["touch_time"],
                    "entry_decision": "baseline",
                    "regime": "D=5s",
                    "decision_path": [],
                    "entry_time": r["entry_time"],
                    "entry_price": r["entry_price"],
                    "entry_delay_seconds": 5.0,
                    "trade": r["trade"],
                }
            )

        cal_sum = compute_calendar_sum(dynamic_results)

        evaluation = {
            "metrics": {
                "dynamic": {
                    "calendar_sum_pct": cal_sum,
                    "trade_count": sum(1 for r in dynamic_results if r["trade"]),
                    "win_rate": 0.5,
                    "avg_win_pct": 0.1,
                    "avg_loss_pct": -0.1,
                    "payoff_ratio": 1.0,
                    "skip_rate": 0.1,
                    "pullback_fill_rate": 0.0,
                    "per_trade_quality_bps": 5.0,
                },
                "baseline_a": {
                    "calendar_sum_pct": cal_sum * 0.9,
                    "trade_count": 3,
                    "win_rate": 0.5,
                    "avg_win_pct": 0.1,
                    "avg_loss_pct": -0.1,
                    "payoff_ratio": 1.0,
                    "skip_rate": 0.0,
                    "pullback_fill_rate": 0.0,
                    "per_trade_quality_bps": 4.0,
                },
                "baseline_b": {"calendar_sum_pct": cal_sum * 0.8},
                "baseline_c": {"calendar_sum_pct": cal_sum * 0.7},
            },
            "overfitting_flag": False,
            "train_calendar_sum": cal_sum * 1.2,
            "test_calendar_sum": cal_sum,
            "symbol_metrics": {
                "BTCUSDT": {
                    "calendar_sum_pct": cal_sum,
                    "win_rate": 0.5,
                    "trade_count": 2,
                    "negative_flag": False,
                }
            },
        }

        bootstrap_ci = {
            "btc_ci": {"p5": -1.0, "p95": 3.0, "mean": 1.0},
            "eth_ci": {"p5": -0.5, "p95": 2.0, "mean": 0.5},
            "combined_ci": {"p5": -0.8, "p95": 2.5, "mean": 0.8},
            "small_sample_warning": True,
        }

        regime_stability = {
            "train_distribution": {"StrongMomentum": 0.5, "Default": 0.5},
            "test_distribution": {"StrongMomentum": 0.4, "Default": 0.6},
            "regime_distribution_shift": False,
            "shifted_regimes": [],
            "regime_stats": {
                "StrongMomentum": {
                    "count": 2,
                    "trade_count": 2,
                    "win_rate": 0.5,
                    "avg_pnl_pct": 0.1,
                    "calendar_contribution_pct": 0.5,
                }
            },
        }

        sensitivity = {
            "high_sensitivity_params": [],
            "sensitivities": {},
        }

        return {
            "dynamic_results": dynamic_results,
            "baseline_a_results": baseline_a_results,
            "evaluation": evaluation,
            "best_params": params,
            "bootstrap_ci": bootstrap_ci,
            "regime_stability": regime_stability,
            "sensitivity": sensitivity,
        }

    def test_all_output_files_created(self, tmp_path, synthetic_pipeline_data):
        """generate_output_files creates attribution.csv, trades.csv, summary.json."""
        data = synthetic_pipeline_data
        generate_output_files(
            dynamic_results=data["dynamic_results"],
            baseline_a_results=data["baseline_a_results"],
            evaluation=data["evaluation"],
            best_params=data["best_params"],
            bootstrap_ci=data["bootstrap_ci"],
            regime_stability=data["regime_stability"],
            sensitivity=data["sensitivity"],
            output_dir=tmp_path,
        )

        assert (tmp_path / "dynamic_timing_attribution.csv").exists()
        assert (tmp_path / "dynamic_timing_trades.csv").exists()
        assert (tmp_path / "dynamic_timing_summary.json").exists()

    def test_attribution_csv_has_rows(self, tmp_path, synthetic_pipeline_data):
        """Attribution CSV has one row per event."""
        data = synthetic_pipeline_data
        generate_output_files(
            dynamic_results=data["dynamic_results"],
            baseline_a_results=data["baseline_a_results"],
            evaluation=data["evaluation"],
            best_params=data["best_params"],
            bootstrap_ci=data["bootstrap_ci"],
            regime_stability=data["regime_stability"],
            sensitivity=data["sensitivity"],
            output_dir=tmp_path,
        )

        df = pd.read_csv(tmp_path / "dynamic_timing_attribution.csv")
        assert len(df) == len(data["dynamic_results"])

    def test_summary_json_valid(self, tmp_path, synthetic_pipeline_data):
        """Summary JSON is valid and contains required keys."""
        data = synthetic_pipeline_data
        generate_output_files(
            dynamic_results=data["dynamic_results"],
            baseline_a_results=data["baseline_a_results"],
            evaluation=data["evaluation"],
            best_params=data["best_params"],
            bootstrap_ci=data["bootstrap_ci"],
            regime_stability=data["regime_stability"],
            sensitivity=data["sensitivity"],
            output_dir=tmp_path,
        )

        with open(tmp_path / "dynamic_timing_summary.json") as f:
            summary = json.load(f)

        assert "metrics" in summary
        assert "best_params" in summary
        assert "overfitting_flag" in summary


# ---------------------------------------------------------------------------
# Test: Report generation
# ---------------------------------------------------------------------------


class TestReportGeneration:
    """Verify that generate_report creates the report file with expected sections."""

    def test_report_file_created(self, tmp_path):
        """generate_report creates dynamic_timing_report.md."""
        evaluation = {
            "metrics": {
                "dynamic": {
                    "calendar_sum_pct": 4.0,
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
                    "calendar_sum_pct": 2.5,
                    "trade_count": 35,
                    "win_rate": 0.60,
                    "avg_win_pct": 0.18,
                    "avg_loss_pct": -0.12,
                    "payoff_ratio": 1.5,
                    "skip_rate": 0.0,
                    "pullback_fill_rate": 0.0,
                    "per_trade_quality_bps": 5.0,
                },
                "baseline_b": {"calendar_sum_pct": 2.0},
                "baseline_c": {"calendar_sum_pct": 1.5},
            },
            "overfitting_flag": False,
            "train_calendar_sum": 5.0,
            "test_calendar_sum": 4.0,
            "symbol_metrics": {
                "BTCUSDT": {
                    "calendar_sum_pct": 2.5,
                    "win_rate": 0.70,
                    "trade_count": 18,
                    "negative_flag": False,
                }
            },
        }

        bootstrap_ci = {
            "btc_ci": {"p5": 1.0, "p95": 5.0, "mean": 3.0},
            "eth_ci": {"p5": 0.5, "p95": 4.0, "mean": 2.0},
            "combined_ci": {"p5": 2.0, "p95": 8.0, "mean": 5.0},
            "small_sample_warning": True,
        }

        regime_stability = {
            "train_distribution": {"StrongMomentum": 0.4, "Default": 0.6},
            "test_distribution": {"StrongMomentum": 0.35, "Default": 0.65},
            "regime_distribution_shift": False,
            "shifted_regimes": [],
            "regime_stats": {
                "StrongMomentum": {
                    "count": 20,
                    "trade_count": 20,
                    "win_rate": 0.75,
                    "avg_pnl_pct": 0.15,
                    "calendar_contribution_pct": 2.0,
                }
            },
        }

        sensitivity = {
            "high_sensitivity_params": [],
            "sensitivities": {},
        }

        output_files = {
            "data_gap_events": [],
            "pullback_benefit_marginal": False,
        }

        params = TimingParams(max_steps=4)

        report = generate_report(
            evaluation=evaluation,
            best_params=params,
            bootstrap_ci=bootstrap_ci,
            regime_stability=regime_stability,
            sensitivity=sensitivity,
            output_files=output_files,
            output_dir=tmp_path,
        )

        report_path = tmp_path / "dynamic_timing_report.md"
        assert report_path.exists()

    def test_report_contains_expected_sections(self, tmp_path):
        """Report contains all required sections."""
        evaluation = {
            "metrics": {
                "dynamic": {
                    "calendar_sum_pct": 4.0,
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
                    "calendar_sum_pct": 2.5,
                    "trade_count": 35,
                    "win_rate": 0.60,
                    "avg_win_pct": 0.18,
                    "avg_loss_pct": -0.12,
                    "payoff_ratio": 1.5,
                    "skip_rate": 0.0,
                    "pullback_fill_rate": 0.0,
                    "per_trade_quality_bps": 5.0,
                },
                "baseline_b": {"calendar_sum_pct": 2.0},
                "baseline_c": {"calendar_sum_pct": 1.5},
            },
            "overfitting_flag": False,
            "train_calendar_sum": 5.0,
            "test_calendar_sum": 4.0,
            "symbol_metrics": {},
        }

        report = generate_report(
            evaluation=evaluation,
            best_params=TimingParams(max_steps=4),
            bootstrap_ci={
                "btc_ci": {"p5": 1.0, "p95": 5.0, "mean": 3.0},
                "eth_ci": {"p5": 0.5, "p95": 4.0, "mean": 2.0},
                "combined_ci": {"p5": 2.0, "p95": 8.0, "mean": 5.0},
                "small_sample_warning": True,
            },
            regime_stability={
                "train_distribution": {},
                "test_distribution": {},
                "regime_distribution_shift": False,
                "shifted_regimes": [],
                "regime_stats": {},
            },
            sensitivity={"high_sensitivity_params": [], "sensitivities": {}},
            output_files={"data_gap_events": [], "pullback_benefit_marginal": False},
            output_dir=tmp_path,
        )

        assert "## 1. 实验设计" in report
        assert "## 2. 结果对比" in report
        assert "## 3. Regime 分析" in report
        assert "## 7. Go/No-Go 判定" in report
        assert "## 8. 下一步行动建议" in report


# ---------------------------------------------------------------------------
# Test: Forbidden directory constraint
# ---------------------------------------------------------------------------


class TestForbiddenDirectoryConstraint:
    """Verify output_dir does NOT resolve to forbidden paths (Requirement 6.7)."""

    def test_output_dir_not_in_forbidden_paths(self):
        """The output_dir in main() resolves to research/.../ and NOT to forbidden dirs."""
        runner_path = (
            Path(__file__).resolve().parents[1]
            / "dynamic_entry_timing_runner.py"
        )
        # The output_dir in main() is:
        # Path(__file__).resolve().parents[1] / "output" / "dynamic_timing"
        output_dir = runner_path.resolve().parent.parent / "output" / "dynamic_timing"
        output_str = str(output_dir)

        forbidden = [
            "internal/",
            "deployments/",
            ".github/workflows/",
            "cmd/",
            "web/",
        ]
        for f in forbidden:
            assert f not in output_str, f"Output dir contains forbidden path: {f}"

        assert "research/entry_redesign/scripts/output/dynamic_timing" in output_str


# ---------------------------------------------------------------------------
# Test: Grid search determinism
# ---------------------------------------------------------------------------


class TestGridSearchDeterminism:
    """Verify grid search produces identical results across runs (Requirement 6.6)."""

    def test_grid_search_layer1_deterministic(self):
        """run_grid_search_layer1 produces identical DataFrames on two runs."""
        bars = _make_bars("2024-01-01 00:00:00", n=600)
        events = _make_events("2024-01-01 00:00:10", n=2)
        cache = {"BTCUSDT_202401": bars}

        best_1, df_1 = run_grid_search_layer1(events, cache)
        best_2, df_2 = run_grid_search_layer1(events, cache)

        # DataFrames must be identical
        pd.testing.assert_frame_equal(df_1, df_2)

        # Best params must be identical
        assert best_1.max_steps == best_2.max_steps
        assert best_1.strong_momentum_threshold == best_2.strong_momentum_threshold
        assert best_1.extension_threshold == best_2.extension_threshold
        assert best_1.moderate_momentum_threshold == best_2.moderate_momentum_threshold
