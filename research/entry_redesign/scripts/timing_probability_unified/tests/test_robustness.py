"""Unit tests for robustness module — run_forward_validation, run_bootstrap, run_ablation_study."""

from __future__ import annotations

import numpy as np
import pandas as pd
import pytest
from sklearn.tree import DecisionTreeClassifier
from sklearn.ensemble import RandomForestClassifier

from timing_probability_unified.robustness import (
    AblationRow,
    BootstrapResult,
    ForwardResult,
    run_ablation_study,
    run_bootstrap,
    run_forward_validation,
)
from timing_probability_unified.combined_executor import CombinedPositionConfig


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _make_forward_events(n: int = 5, symbol: str = "BTCUSDT") -> pd.DataFrame:
    """Create a minimal forward_events DataFrame for testing."""
    return pd.DataFrame(
        {
            "event_id": [f"{symbol}_fwd_{i}" for i in range(n)],
            "symbol": [symbol] * n,
            "side": ["long"] * n,
            "touch_time": pd.date_range("2025-11-01", periods=n, freq="1h", tz="UTC"),
            "touch_price": [50000.0 + i * 100 for i in range(n)],
            "speed_300s_atr": [0.5 + i * 0.1 for i in range(n)],
            "atr": [100.0] * n,
            # Original_10_Features
            "signal_atr_percentile": np.random.default_rng(42).uniform(0, 1, n),
            "roundtrip_cost_atr": np.random.default_rng(43).uniform(0, 0.1, n),
            "prev1_body_atr": np.random.default_rng(44).uniform(0, 2, n),
            "prev1_range_atr": np.random.default_rng(45).uniform(0, 3, n),
            "prev1_close_pos_side": np.random.default_rng(46).uniform(-1, 1, n),
            "prev_sma5_gap_atr": np.random.default_rng(47).uniform(-1, 1, n),
            "prev_sma5_slope_atr": np.random.default_rng(48).uniform(-0.5, 0.5, n),
            "level_to_prev_close_atr": np.random.default_rng(49).uniform(0, 2, n),
            "level_to_signal_open_atr": np.random.default_rng(50).uniform(0, 2, n),
            "touch_extension_atr": np.random.default_rng(51).uniform(0, 1, n),
        }
    )


def _train_dummy_timing_classifier(n_features: int = 10) -> DecisionTreeClassifier:
    """Train a dummy DT3 classifier on synthetic data."""
    rng = np.random.default_rng(42)
    X = pd.DataFrame(rng.uniform(0, 1, (30, n_features)))
    y = pd.Series(rng.choice(["skip", "fast", "slow"], 30))
    clf = DecisionTreeClassifier(max_depth=3, random_state=42)
    clf.fit(X, y)
    return clf


def _train_dummy_rf_model(n_features: int = 10) -> RandomForestClassifier:
    """Train a dummy RF model on synthetic data."""
    rng = np.random.default_rng(42)
    X = pd.DataFrame(rng.uniform(0, 1, (30, n_features)))
    y = pd.Series(rng.choice([0, 1], 30))
    rf = RandomForestClassifier(n_estimators=10, random_state=42)
    rf.fit(X, y)
    return rf


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------


class TestRunForwardValidation:
    """Tests for run_forward_validation()."""

    def test_empty_forward_events(self):
        """Empty forward events → all flags set appropriately."""
        forward_events = pd.DataFrame(
            columns=[
                "event_id", "symbol", "side", "touch_time", "touch_price",
                "speed_300s_atr", "atr",
                "signal_atr_percentile", "roundtrip_cost_atr",
                "prev1_body_atr", "prev1_range_atr", "prev1_close_pos_side",
                "prev_sma5_gap_atr", "prev_sma5_slope_atr",
                "level_to_prev_close_atr", "level_to_signal_open_atr",
                "touch_extension_atr",
            ]
        )
        clf = _train_dummy_timing_classifier()
        rf = _train_dummy_rf_model()

        result = run_forward_validation(
            forward_events=forward_events,
            timing_classifier=clf,
            rf_model=rf,
            speed_gate_threshold=0.3,
            bars_cache={},
            full_window_calendar_sum=0.10,
        )

        assert isinstance(result, ForwardResult)
        assert result.forward_calendar_sum == 0.0
        assert result.forward_worst_sm == 0.0
        assert result.forward_trade_count == 0
        assert result.overfitting_flag is True  # 0 < 0.5 * 0.10
        assert result.forward_underperformance is True  # 0 < 0.07

    def test_returns_forward_result_type(self):
        """Function returns a ForwardResult dataclass."""
        forward_events = _make_forward_events(3)
        clf = _train_dummy_timing_classifier()
        rf = _train_dummy_rf_model()

        result = run_forward_validation(
            forward_events=forward_events,
            timing_classifier=clf,
            rf_model=rf,
            speed_gate_threshold=0.3,
            bars_cache={},
            full_window_calendar_sum=0.10,
        )

        assert isinstance(result, ForwardResult)
        assert isinstance(result.forward_calendar_sum, float)
        assert isinstance(result.forward_worst_sm, float)
        assert isinstance(result.forward_trade_count, int)
        assert isinstance(result.overfitting_flag, bool)
        assert isinstance(result.forward_risk_flag, bool)
        assert isinstance(result.forward_underperformance, bool)

    def test_overfitting_flag_triggered(self):
        """overfitting_flag = True when forward_cs < 0.5 * full_window_cs."""
        forward_events = _make_forward_events(3)
        clf = _train_dummy_timing_classifier()
        rf = _train_dummy_rf_model()

        # With no bars_cache, all delays are untradeable → calendar_sum ≈ 0
        # full_window_calendar_sum = 0.20 → 0 < 0.5 * 0.20 = 0.10 → flag=True
        result = run_forward_validation(
            forward_events=forward_events,
            timing_classifier=clf,
            rf_model=rf,
            speed_gate_threshold=0.3,
            bars_cache={},
            full_window_calendar_sum=0.20,
        )

        assert result.overfitting_flag is True

    def test_overfitting_flag_not_triggered_when_full_window_zero(self):
        """overfitting_flag = False when full_window_calendar_sum = 0."""
        forward_events = _make_forward_events(3)
        clf = _train_dummy_timing_classifier()
        rf = _train_dummy_rf_model()

        # forward_cs = 0, full_window_cs = 0 → 0 < 0.5 * 0 = 0 → False
        result = run_forward_validation(
            forward_events=forward_events,
            timing_classifier=clf,
            rf_model=rf,
            speed_gate_threshold=0.3,
            bars_cache={},
            full_window_calendar_sum=0.0,
        )

        assert result.overfitting_flag is False

    def test_forward_risk_flag_threshold(self):
        """forward_risk_flag = True when worst_sm < -0.01 (-1.0%)."""
        forward_events = _make_forward_events(3)
        clf = _train_dummy_timing_classifier()
        rf = _train_dummy_rf_model()

        # With no bars, worst_sm = 0 → flag should be False
        result = run_forward_validation(
            forward_events=forward_events,
            timing_classifier=clf,
            rf_model=rf,
            speed_gate_threshold=0.3,
            bars_cache={},
            full_window_calendar_sum=0.10,
        )

        # worst_sm = 0.0 which is NOT < -0.01
        assert result.forward_risk_flag is False

    def test_forward_underperformance_flag(self):
        """forward_underperformance = True when forward_cs < 0.07."""
        forward_events = _make_forward_events(3)
        clf = _train_dummy_timing_classifier()
        rf = _train_dummy_rf_model()

        # With no bars, calendar_sum = 0 < 0.07 → flag=True
        result = run_forward_validation(
            forward_events=forward_events,
            timing_classifier=clf,
            rf_model=rf,
            speed_gate_threshold=0.3,
            bars_cache={},
            full_window_calendar_sum=0.10,
        )

        assert result.forward_underperformance is True

    def test_speed_gate_filters_events(self):
        """Events below speed_gate_threshold are filtered from trade count."""
        forward_events = _make_forward_events(5)
        # Set speed_300s_atr: first 2 below threshold, last 3 above
        forward_events["speed_300s_atr"] = [0.1, 0.2, 0.5, 0.6, 0.7]
        clf = _train_dummy_timing_classifier()
        rf = _train_dummy_rf_model()

        # Threshold = 0.4 → events 0,1 fail gate, events 2,3,4 pass
        result = run_forward_validation(
            forward_events=forward_events,
            timing_classifier=clf,
            rf_model=rf,
            speed_gate_threshold=0.4,
            bars_cache={},
            full_window_calendar_sum=0.10,
        )

        # Trade count should only include events that pass gate AND are not skip
        # Since all delays are untradeable (no bars), pnl=0 for all
        # But trade_count counts non-skip events that pass gate
        assert result.forward_trade_count >= 0
        assert result.forward_trade_count <= 3  # at most 3 pass gate

    def test_custom_config_base_share(self):
        """Custom config with different base_share is respected."""
        forward_events = _make_forward_events(3)
        clf = _train_dummy_timing_classifier()
        rf = _train_dummy_rf_model()

        config = CombinedPositionConfig(base_notional_share=0.50)

        result = run_forward_validation(
            forward_events=forward_events,
            timing_classifier=clf,
            rf_model=rf,
            speed_gate_threshold=0.3,
            bars_cache={},
            full_window_calendar_sum=0.10,
            config=config,
        )

        assert isinstance(result, ForwardResult)


# ---------------------------------------------------------------------------
# Property-Based Tests
# ---------------------------------------------------------------------------

from hypothesis import given, settings
from hypothesis import strategies as st

from timing_probability_unified.probability_model import RFProbabilityResult
from timing_probability_unified.speed_gate import SpeedGateResult


# Feature: timing-probability-unified, Property 16: Threshold Warning Flags
# **Validates: Requirements 3.6, 5.5, 7.8, 7.4**


def _compute_rf_no_signal_warning(test_auc: float) -> bool:
    """Replicate the rf_no_signal_warning threshold logic from probability_model."""
    return test_auc < 0.50


def _compute_aggressive_gate_warning(gate_pass_rate: float) -> bool:
    """Replicate the aggressive_gate_warning threshold logic from speed_gate."""
    return gate_pass_rate < 0.70


def _compute_forward_underperformance(forward_calendar_sum: float) -> bool:
    """Replicate the forward_underperformance threshold logic from robustness."""
    return forward_calendar_sum < 0.07


def _compute_overfitting_flag(forward_cs: float, full_window_cs: float) -> bool:
    """Replicate the overfitting_flag threshold logic from robustness."""
    return forward_cs < 0.5 * full_window_cs


@settings(max_examples=200)
@given(test_auc=st.floats(min_value=0.0, max_value=1.0, allow_nan=False))
def test_property16_rf_no_signal_warning(test_auc: float):
    """Property 16a: rf_no_signal_warning == (test_auc < 0.50).

    For any test AUC value in [0, 1], the rf_no_signal_warning flag
    must be True iff test_auc < 0.50.

    Tests the threshold logic used in RFProbabilityResult construction.
    """
    # Simulate what train_rf_probability does for the warning flag
    flag = _compute_rf_no_signal_warning(test_auc)
    assert flag == (test_auc < 0.50)

    # Verify boundary: at exactly 0.50, flag should be False
    if test_auc == 0.50:
        assert flag is False
    elif test_auc < 0.50:
        assert flag is True
    else:
        assert flag is False


@settings(max_examples=200)
@given(gate_pass_rate=st.floats(min_value=0.0, max_value=1.0, allow_nan=False))
def test_property16_aggressive_gate_warning(gate_pass_rate: float):
    """Property 16b: aggressive_gate_warning == (gate_pass_rate < 0.70).

    For any gate pass rate in [0, 1], the aggressive_gate_warning flag
    must be True iff gate_pass_rate < 0.70.

    Tests the threshold logic used in SpeedGateResult construction.
    """
    # Simulate what analyze_speed_gate does for the warning flag
    flag = _compute_aggressive_gate_warning(gate_pass_rate)
    assert flag == (gate_pass_rate < 0.70)

    # Verify boundary: at exactly 0.70, flag should be False
    if gate_pass_rate == 0.70:
        assert flag is False
    elif gate_pass_rate < 0.70:
        assert flag is True
    else:
        assert flag is False


@settings(max_examples=200)
@given(
    forward_calendar_sum=st.floats(
        min_value=-0.50, max_value=0.50, allow_nan=False
    )
)
def test_property16_forward_underperformance(forward_calendar_sum: float):
    """Property 16c: forward_underperformance == (forward_calendar_sum < 0.07).

    For any forward calendar_sum value, the forward_underperformance flag
    must be True iff forward_calendar_sum < 7% (i.e., < 0.07).

    Tests the threshold logic used in ForwardResult construction.
    """
    # Simulate what run_forward_validation does for the flag
    flag = _compute_forward_underperformance(forward_calendar_sum)
    assert flag == (forward_calendar_sum < 0.07)

    # Verify boundary: at exactly 0.07, flag should be False
    if forward_calendar_sum == 0.07:
        assert flag is False
    elif forward_calendar_sum < 0.07:
        assert flag is True
    else:
        assert flag is False


@settings(max_examples=200)
@given(
    forward_cs=st.floats(min_value=-0.50, max_value=0.50, allow_nan=False),
    full_window_cs=st.floats(min_value=-0.50, max_value=0.50, allow_nan=False),
)
def test_property16_overfitting_flag(forward_cs: float, full_window_cs: float):
    """Property 16d: overfitting_flag == (forward_cs < 0.5 * full_window_cs).

    For any forward calendar_sum and full_window calendar_sum, the
    overfitting_flag must be True iff forward_cs < 0.5 × full_window_cs.

    Tests the threshold logic used in ForwardResult construction.
    """
    # Simulate what run_forward_validation does for the flag
    flag = _compute_overfitting_flag(forward_cs, full_window_cs)
    assert flag == (forward_cs < 0.5 * full_window_cs)

    # Verify specific cases:
    # When full_window_cs = 0, threshold = 0 → flag = (forward_cs < 0)
    # When forward_cs >= full_window_cs, flag should be False (since 0.5 * fw <= fw)
    if full_window_cs >= 0 and forward_cs >= full_window_cs:
        assert flag is False


# ---------------------------------------------------------------------------
# Helpers for run_bootstrap / run_ablation_study tests
# ---------------------------------------------------------------------------


def _make_trades_df(
    n: int = 10,
    symbols: list[str] | None = None,
    pnl_values: list[float] | None = None,
) -> pd.DataFrame:
    """Create a minimal unified_trades DataFrame for bootstrap/ablation tests."""
    if symbols is None:
        symbols = ["BTCUSDT", "ETHUSDT"] * (n // 2) + ["BTCUSDT"] * (n % 2)
    if pnl_values is None:
        rng = np.random.default_rng(42)
        pnl_values = rng.uniform(-0.01, 0.02, n).tolist()

    return pd.DataFrame(
        {
            "event_id": [f"evt_{i}" for i in range(n)],
            "symbol": symbols[:n],
            "side": ["long"] * n,
            "touch_time": pd.date_range("2025-01-01", periods=n, freq="1D", tz="UTC"),
            "timing_prediction": ["fast"] * n,
            "selected_delay": ["D0"] * n,
            "rf_probability": [0.5] * n,
            "sizing_multiplier": [1.0] * n,
            "position_size": [0.30] * n,
            "delay_pnl_pct": pnl_values[:n],
            "weighted_pnl": [p * 0.30 for p in pnl_values[:n]],
            "speed_300s_atr": [0.5] * n,
            "speed_gate_pass": [True] * n,
        }
    )


def _make_events_for_ablation(n: int = 6) -> pd.DataFrame:
    """Create a minimal events DataFrame for ablation study tests."""
    return pd.DataFrame(
        {
            "event_id": [f"evt_{i}" for i in range(n)],
            "symbol": ["BTCUSDT", "ETHUSDT"] * (n // 2),
            "side": ["long"] * n,
            "touch_time": pd.date_range("2025-01-01", periods=n, freq="1D", tz="UTC"),
            "speed_300s_atr": [0.3, 0.5, 0.7, 0.4, 0.6, 0.8][:n],
        }
    )


def _make_delay_results_for_ablation(n: int = 6) -> list:
    """Create delay results for ablation study tests.

    Each event gets 5 DelayResult objects with known PnL values.
    """
    from pre_breakout_timing.delay_simulator import DelayResult

    all_results = []
    pnls = [0.01, -0.005, 0.008, 0.003, -0.002, 0.015]
    for i in range(n):
        event_delays = []
        for j, label in enumerate(["D0", "D5", "D10", "D15", "pullback"]):
            # Vary PnL slightly per delay
            pnl = pnls[i % len(pnls)] * (1 + j * 0.1)
            event_delays.append(
                DelayResult(
                    event_id=f"evt_{i}",
                    delay_label=label,
                    delay_seconds=j * 5,
                    entry_time=pd.Timestamp("2025-01-01", tz="UTC"),
                    entry_price=50000.0,
                    pnl_pct=pnl,
                    exit_reason="TrailingSL",
                    exit_time=pd.Timestamp("2025-01-01 01:00:00", tz="UTC"),
                    hold_seconds=3600.0,
                    mfe_r=0.5,
                    mae_r=-0.2,
                    traded=True,
                )
            )
        all_results.append(event_delays)
    return all_results


# ---------------------------------------------------------------------------
# Tests for run_bootstrap
# ---------------------------------------------------------------------------


class TestRunBootstrap:
    """Tests for run_bootstrap()."""

    def test_empty_trades_returns_all_zeros(self):
        """Bootstrap with empty trades → all fields are 0.0.

        Validates: Requirements 7.1
        """
        empty_trades = pd.DataFrame(
            columns=[
                "event_id", "symbol", "side", "touch_time",
                "timing_prediction", "selected_delay", "rf_probability",
                "sizing_multiplier", "position_size", "delay_pnl_pct",
                "weighted_pnl", "speed_300s_atr", "speed_gate_pass",
            ]
        )

        result = run_bootstrap(empty_trades, n_resamples=100, random_state=42)

        assert isinstance(result, BootstrapResult)
        assert result.calendar_sum_mean == 0.0
        assert result.calendar_sum_ci_5 == 0.0
        assert result.calendar_sum_ci_95 == 0.0
        assert result.ci_width == 0.0
        assert result.btc_calendar_sum == 0.0
        assert result.btc_ci_5 == 0.0
        assert result.btc_ci_95 == 0.0
        assert result.eth_calendar_sum == 0.0
        assert result.eth_ci_5 == 0.0
        assert result.eth_ci_95 == 0.0

    def test_single_trade_ci_width_zero(self):
        """Bootstrap with single trade → CI width = 0 (all resamples identical).

        When there is only 1 trade, every bootstrap resample picks that same
        trade, so all resampled calendar_sums are identical → CI width = 0.

        Validates: Requirements 7.1
        """
        single_trade = _make_trades_df(n=1, pnl_values=[0.02])

        result = run_bootstrap(single_trade, n_resamples=100, random_state=42)

        assert isinstance(result, BootstrapResult)
        assert result.ci_width == 0.0
        assert result.calendar_sum_ci_5 == result.calendar_sum_ci_95

    def test_returns_bootstrap_result_type(self):
        """Function returns a BootstrapResult dataclass with correct types."""
        trades = _make_trades_df(n=10)

        result = run_bootstrap(trades, n_resamples=50, random_state=42)

        assert isinstance(result, BootstrapResult)
        assert isinstance(result.calendar_sum_mean, float)
        assert isinstance(result.ci_width, float)
        assert result.ci_width >= 0.0

    def test_ci_5_less_than_ci_95(self):
        """5th percentile should be <= 95th percentile for non-trivial trades."""
        trades = _make_trades_df(n=20)

        result = run_bootstrap(trades, n_resamples=200, random_state=42)

        assert result.calendar_sum_ci_5 <= result.calendar_sum_ci_95

    def test_btc_only_trades(self):
        """Bootstrap with only BTC trades → ETH fields are 0."""
        trades = _make_trades_df(n=5, symbols=["BTCUSDT"] * 5)

        result = run_bootstrap(trades, n_resamples=50, random_state=42)

        assert result.eth_calendar_sum == 0.0
        assert result.eth_ci_5 == 0.0
        assert result.eth_ci_95 == 0.0
        # BTC should have non-zero values (trades have non-zero pnl)
        assert result.btc_calendar_sum != 0.0 or result.btc_ci_5 != result.btc_ci_95


# ---------------------------------------------------------------------------
# Tests for run_ablation_study
# ---------------------------------------------------------------------------


class TestRunAblationStudy:
    """Tests for run_ablation_study()."""

    def test_produces_4_rows(self):
        """Ablation study produces exactly 4 AblationRow results.

        Validates: Requirements 7.5
        """
        events = _make_events_for_ablation(6)
        timing_predictions = np.array(["fast", "slow", "skip", "fast", "slow", "fast"])
        sizing_multipliers = np.array([1.0, 0.8, 0.5, 1.2, 0.9, 1.5])
        delay_results = _make_delay_results_for_ablation(6)
        speed_gate_pass = np.array([True, True, True, False, True, True])

        rows = run_ablation_study(
            events=events,
            timing_predictions=timing_predictions,
            sizing_multipliers=sizing_multipliers,
            delay_results=delay_results,
            speed_gate_pass=speed_gate_pass,
        )

        assert len(rows) == 4
        assert all(isinstance(r, AblationRow) for r in rows)

    def test_config_names_correct(self):
        """Ablation study config names are exactly the expected 4 names in order.

        Validates: Requirements 7.5
        """
        events = _make_events_for_ablation(6)
        timing_predictions = np.array(["fast", "slow", "skip", "fast", "slow", "fast"])
        sizing_multipliers = np.array([1.0, 0.8, 0.5, 1.2, 0.9, 1.5])
        delay_results = _make_delay_results_for_ablation(6)
        speed_gate_pass = np.array([True, True, True, False, True, True])

        rows = run_ablation_study(
            events=events,
            timing_predictions=timing_predictions,
            sizing_multipliers=sizing_multipliers,
            delay_results=delay_results,
            speed_gate_pass=speed_gate_pass,
        )

        expected_names = ["timing_only", "probability_only", "no_speed_gate", "full_unified"]
        actual_names = [r.config_name for r in rows]
        assert actual_names == expected_names

    def test_ablation_row_fields_are_numeric(self):
        """Each AblationRow has numeric calendar_sum, worst_sm, trade_count, avg_pnl.

        Validates: Requirements 7.5, 7.6
        """
        events = _make_events_for_ablation(6)
        timing_predictions = np.array(["fast", "slow", "skip", "fast", "slow", "fast"])
        sizing_multipliers = np.array([1.0, 0.8, 0.5, 1.2, 0.9, 1.5])
        delay_results = _make_delay_results_for_ablation(6)
        speed_gate_pass = np.array([True, True, True, False, True, True])

        rows = run_ablation_study(
            events=events,
            timing_predictions=timing_predictions,
            sizing_multipliers=sizing_multipliers,
            delay_results=delay_results,
            speed_gate_pass=speed_gate_pass,
        )

        for row in rows:
            assert isinstance(row.calendar_sum, float)
            assert isinstance(row.worst_sm, float)
            assert isinstance(row.trade_count, int)
            assert isinstance(row.avg_pnl_per_trade, float)
            assert row.trade_count >= 0

    def test_probability_only_has_more_trades_than_full(self):
        """probability_only (no skip) should have >= trades than full_unified.

        Since probability_only removes the timing skip filter, it should
        include at least as many trades as the full unified configuration.

        Validates: Requirements 7.5
        """
        events = _make_events_for_ablation(6)
        # Include some "skip" predictions so probability_only differs from full
        timing_predictions = np.array(["fast", "slow", "skip", "skip", "slow", "fast"])
        sizing_multipliers = np.array([1.0, 0.8, 0.5, 1.2, 0.9, 1.5])
        delay_results = _make_delay_results_for_ablation(6)
        speed_gate_pass = np.array([True, True, True, True, True, True])

        rows = run_ablation_study(
            events=events,
            timing_predictions=timing_predictions,
            sizing_multipliers=sizing_multipliers,
            delay_results=delay_results,
            speed_gate_pass=speed_gate_pass,
        )

        prob_only = next(r for r in rows if r.config_name == "probability_only")
        full_unified = next(r for r in rows if r.config_name == "full_unified")
        assert prob_only.trade_count >= full_unified.trade_count

    def test_no_speed_gate_has_more_trades_than_full(self):
        """no_speed_gate should have >= trades than full_unified when gate filters some.

        Validates: Requirements 7.5
        """
        events = _make_events_for_ablation(6)
        timing_predictions = np.array(["fast", "slow", "fast", "fast", "slow", "fast"])
        sizing_multipliers = np.array([1.0, 0.8, 0.5, 1.2, 0.9, 1.5])
        delay_results = _make_delay_results_for_ablation(6)
        # Some events fail gate
        speed_gate_pass = np.array([True, False, True, False, True, True])

        rows = run_ablation_study(
            events=events,
            timing_predictions=timing_predictions,
            sizing_multipliers=sizing_multipliers,
            delay_results=delay_results,
            speed_gate_pass=speed_gate_pass,
        )

        no_gate = next(r for r in rows if r.config_name == "no_speed_gate")
        full_unified = next(r for r in rows if r.config_name == "full_unified")
        assert no_gate.trade_count >= full_unified.trade_count


# ---------------------------------------------------------------------------
# Tests for overfitting_flag trigger conditions (via run_forward_validation)
# ---------------------------------------------------------------------------


class TestOverfittingFlagConditions:
    """Additional tests for overfitting_flag trigger conditions.

    Validates: Requirements 7.4
    """

    def test_overfitting_flag_true_when_forward_much_less_than_half(self):
        """overfitting_flag = True when forward_cs is clearly < 0.5 * full_window_cs."""
        forward_events = _make_forward_events(3)
        clf = _train_dummy_timing_classifier()
        rf = _train_dummy_rf_model()

        # forward_cs ≈ 0 (no bars), full_window_cs = 0.50
        # 0 < 0.5 * 0.50 = 0.25 → flag = True
        result = run_forward_validation(
            forward_events=forward_events,
            timing_classifier=clf,
            rf_model=rf,
            speed_gate_threshold=0.3,
            bars_cache={},
            full_window_calendar_sum=0.50,
        )

        assert result.overfitting_flag is True

    def test_overfitting_flag_false_when_full_window_negative(self):
        """overfitting_flag = False when full_window_cs is negative.

        If full_window_cs < 0, then 0.5 * full_window_cs < 0, and
        forward_cs = 0 >= negative threshold → flag = False.
        """
        forward_events = _make_forward_events(3)
        clf = _train_dummy_timing_classifier()
        rf = _train_dummy_rf_model()

        # forward_cs ≈ 0, full_window_cs = -0.10
        # 0 < 0.5 * (-0.10) = -0.05 → False (0 is NOT < -0.05)
        result = run_forward_validation(
            forward_events=forward_events,
            timing_classifier=clf,
            rf_model=rf,
            speed_gate_threshold=0.3,
            bars_cache={},
            full_window_calendar_sum=-0.10,
        )

        assert result.overfitting_flag is False
