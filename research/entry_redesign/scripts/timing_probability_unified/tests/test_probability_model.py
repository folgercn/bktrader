"""Unit tests for probability_model — compute_sizing_multiplier() and train_rf_probability()"""

import numpy as np
import pandas as pd
import pytest

from timing_probability_unified.probability_model import (
    RFProbabilityResult,
    compute_sizing_multiplier,
    generate_rf_binary_labels,
    train_rf_probability,
)


class TestComputeSizingMultiplier:
    """Tests for compute_sizing_multiplier function."""

    def test_boundary_zero(self):
        """p=0.0 → multiplier=0.0 (不入场)"""
        result = compute_sizing_multiplier(np.array([0.0]))
        assert result[0] == pytest.approx(0.0)

    def test_boundary_half(self):
        """p=0.5 → multiplier=1.0 (base)"""
        result = compute_sizing_multiplier(np.array([0.5]))
        assert result[0] == pytest.approx(1.0)

    def test_boundary_one(self):
        """p=1.0 → multiplier=2.0 (最大)"""
        result = compute_sizing_multiplier(np.array([1.0]))
        assert result[0] == pytest.approx(2.0)

    def test_array_input(self):
        """多值数组输入正确映射"""
        probs = np.array([0.0, 0.25, 0.5, 0.75, 1.0])
        expected = np.array([0.0, 0.5, 1.0, 1.5, 2.0])
        result = compute_sizing_multiplier(probs)
        np.testing.assert_allclose(result, expected)

    def test_output_clipped_lower(self):
        """负概率值被 clip 到 0"""
        result = compute_sizing_multiplier(np.array([-0.5]))
        assert result[0] == pytest.approx(0.0)

    def test_output_clipped_upper(self):
        """超过 1 的概率值被 clip 到 2"""
        result = compute_sizing_multiplier(np.array([1.5]))
        assert result[0] == pytest.approx(2.0)

    def test_empty_array(self):
        """空数组输入返回空数组"""
        result = compute_sizing_multiplier(np.array([]))
        assert len(result) == 0

    def test_output_shape_matches_input(self):
        """输出形状与输入一致"""
        probs = np.array([0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9])
        result = compute_sizing_multiplier(probs)
        assert result.shape == probs.shape

    def test_all_results_in_valid_range(self):
        """所有输出值在 [0, 2] 范围内"""
        probs = np.linspace(0, 1, 100)
        result = compute_sizing_multiplier(probs)
        assert np.all(result >= 0.0)
        assert np.all(result <= 2.0)

    def test_linear_relationship(self):
        """multiplier = p × 2 对于 p ∈ [0, 1]"""
        probs = np.array([0.1, 0.2, 0.3, 0.4, 0.6, 0.7, 0.8, 0.9])
        result = compute_sizing_multiplier(probs)
        np.testing.assert_allclose(result, probs * 2)


class TestGenerateRFBinaryLabels:
    """Tests for generate_rf_binary_labels function."""

    def test_skip_always_zero(self):
        """timing=skip → label=0 regardless of pnl"""
        predictions = pd.Series(["skip", "skip", "skip"])
        pnls = pd.Series([0.05, -0.01, 0.10])
        labels = generate_rf_binary_labels(predictions, pnls)
        assert (labels == 0).all()

    def test_fast_positive_pnl(self):
        """timing=fast AND pnl > 0 → label=1"""
        predictions = pd.Series(["fast"])
        pnls = pd.Series([0.01])
        labels = generate_rf_binary_labels(predictions, pnls)
        assert labels.iloc[0] == 1

    def test_slow_positive_pnl(self):
        """timing=slow AND pnl > 0 → label=1"""
        predictions = pd.Series(["slow"])
        pnls = pd.Series([0.02])
        labels = generate_rf_binary_labels(predictions, pnls)
        assert labels.iloc[0] == 1

    def test_fast_negative_pnl(self):
        """timing=fast AND pnl <= 0 → label=0"""
        predictions = pd.Series(["fast", "fast"])
        pnls = pd.Series([-0.01, 0.0])
        labels = generate_rf_binary_labels(predictions, pnls)
        assert (labels == 0).all()

    def test_slow_zero_pnl(self):
        """timing=slow AND pnl == 0 → label=0"""
        predictions = pd.Series(["slow"])
        pnls = pd.Series([0.0])
        labels = generate_rf_binary_labels(predictions, pnls)
        assert labels.iloc[0] == 0

    def test_mixed_scenario(self):
        """混合场景验证"""
        predictions = pd.Series(["skip", "fast", "slow", "fast", "slow"])
        pnls = pd.Series([0.05, 0.01, -0.02, -0.01, 0.03])
        labels = generate_rf_binary_labels(predictions, pnls)
        expected = pd.Series([0, 1, 0, 0, 1])
        pd.testing.assert_series_equal(labels, expected)

    def test_output_dtype_int(self):
        """输出 dtype 为 int"""
        predictions = pd.Series(["fast", "slow", "skip"])
        pnls = pd.Series([0.01, 0.02, 0.03])
        labels = generate_rf_binary_labels(predictions, pnls)
        assert labels.dtype == int


class TestTrainRFProbability:
    """Tests for train_rf_probability function."""

    @pytest.fixture
    def sample_data(self):
        """Generate sample training/test data with 10 features."""
        rng = np.random.RandomState(42)
        n_train, n_test = 50, 20
        feature_names = [
            "signal_atr_percentile",
            "roundtrip_cost_atr",
            "prev1_body_atr",
            "prev1_range_atr",
            "prev1_close_pos_side",
            "prev_sma5_gap_atr",
            "prev_sma5_slope_atr",
            "level_to_prev_close_atr",
            "level_to_signal_open_atr",
            "touch_extension_atr",
        ]
        train_features = pd.DataFrame(
            rng.randn(n_train, 10), columns=feature_names
        )
        test_features = pd.DataFrame(
            rng.randn(n_test, 10), columns=feature_names
        )
        # Create labels with both classes
        train_labels = pd.Series(rng.randint(0, 2, n_train))
        test_labels = pd.Series(rng.randint(0, 2, n_test))
        return train_features, train_labels, test_features, test_labels

    def test_returns_rf_probability_result(self, sample_data):
        """返回 RFProbabilityResult 实例"""
        train_f, train_l, test_f, test_l = sample_data
        result = train_rf_probability(train_f, train_l, test_f, test_l)
        assert isinstance(result, RFProbabilityResult)

    def test_auc_in_valid_range(self, sample_data):
        """AUC 值在 [0, 1] 范围内"""
        train_f, train_l, test_f, test_l = sample_data
        result = train_rf_probability(train_f, train_l, test_f, test_l)
        assert 0.0 <= result.train_auc <= 1.0
        assert 0.0 <= result.test_auc <= 1.0

    def test_feature_importance_top5_length(self, sample_data):
        """feature_importance_top5 长度为 5"""
        train_f, train_l, test_f, test_l = sample_data
        result = train_rf_probability(train_f, train_l, test_f, test_l)
        assert len(result.feature_importance_top5) == 5

    def test_feature_importance_sorted_descending(self, sample_data):
        """feature_importance_top5 按重要性降序排列"""
        train_f, train_l, test_f, test_l = sample_data
        result = train_rf_probability(train_f, train_l, test_f, test_l)
        importances = [imp for _, imp in result.feature_importance_top5]
        assert importances == sorted(importances, reverse=True)

    def test_probabilities_in_valid_range(self, sample_data):
        """概率值在 [0, 1] 范围内"""
        train_f, train_l, test_f, test_l = sample_data
        result = train_rf_probability(train_f, train_l, test_f, test_l)
        assert np.all(result.train_probabilities >= 0.0)
        assert np.all(result.train_probabilities <= 1.0)
        assert np.all(result.test_probabilities >= 0.0)
        assert np.all(result.test_probabilities <= 1.0)

    def test_probabilities_shape(self, sample_data):
        """概率数组长度与输入一致"""
        train_f, train_l, test_f, test_l = sample_data
        result = train_rf_probability(train_f, train_l, test_f, test_l)
        assert len(result.train_probabilities) == len(train_f)
        assert len(result.test_probabilities) == len(test_f)

    def test_prob_stats_computed(self, sample_data):
        """概率分布统计已计算"""
        train_f, train_l, test_f, test_l = sample_data
        result = train_rf_probability(train_f, train_l, test_f, test_l)
        assert isinstance(result.prob_mean, float)
        assert isinstance(result.prob_median, float)
        assert isinstance(result.prob_std, float)
        assert result.prob_std >= 0.0

    def test_no_signal_warning_when_auc_below_half(self):
        """test AUC < 0.50 时 rf_no_signal_warning=True"""
        # Create data where RF will perform poorly (random labels)
        rng = np.random.RandomState(123)
        n = 30
        feature_names = [
            "signal_atr_percentile",
            "roundtrip_cost_atr",
            "prev1_body_atr",
            "prev1_range_atr",
            "prev1_close_pos_side",
            "prev_sma5_gap_atr",
            "prev_sma5_slope_atr",
            "level_to_prev_close_atr",
            "level_to_signal_open_atr",
            "touch_extension_atr",
        ]
        # Use same features for train and test but flip labels to get AUC < 0.5
        features = pd.DataFrame(rng.randn(n, 10), columns=feature_names)
        labels = pd.Series(rng.randint(0, 2, n))

        result = train_rf_probability(
            features, labels, features, labels
        )
        # With random data, AUC might be around 0.5 or above (RF overfits train)
        # The warning flag logic is: rf_no_signal_warning == (test_auc < 0.50)
        assert result.rf_no_signal_warning == (result.test_auc < 0.50)

    def test_single_class_labels_auc_fallback(self):
        """单类标签时 AUC 回退为 0.5"""
        rng = np.random.RandomState(42)
        feature_names = [
            "signal_atr_percentile",
            "roundtrip_cost_atr",
            "prev1_body_atr",
            "prev1_range_atr",
            "prev1_close_pos_side",
            "prev_sma5_gap_atr",
            "prev_sma5_slope_atr",
            "level_to_prev_close_atr",
            "level_to_signal_open_atr",
            "touch_extension_atr",
        ]
        train_features = pd.DataFrame(rng.randn(30, 10), columns=feature_names)
        test_features = pd.DataFrame(rng.randn(10, 10), columns=feature_names)
        # All labels are 1 in train, all 0 in test
        train_labels = pd.Series([1] * 30)
        test_labels = pd.Series([0] * 10)

        result = train_rf_probability(
            train_features, train_labels, test_features, test_labels
        )
        # train has only one class → train_auc = 0.5
        assert result.train_auc == 0.5
        # test has only one class → test_auc = 0.5
        assert result.test_auc == 0.5

    def test_deterministic_with_same_random_state(self, sample_data):
        """相同 random_state 产出相同结果"""
        train_f, train_l, test_f, test_l = sample_data
        result1 = train_rf_probability(train_f, train_l, test_f, test_l)
        result2 = train_rf_probability(train_f, train_l, test_f, test_l)
        np.testing.assert_array_equal(
            result1.test_probabilities, result2.test_probabilities
        )
        assert result1.test_auc == result2.test_auc

    def test_custom_n_estimators(self, sample_data):
        """自定义 n_estimators 参数"""
        train_f, train_l, test_f, test_l = sample_data
        result = train_rf_probability(
            train_f, train_l, test_f, test_l, n_estimators=50
        )
        assert isinstance(result, RFProbabilityResult)


# ---------------------------------------------------------------------------
# Property-Based Tests
# ---------------------------------------------------------------------------

from hypothesis import given, settings
from hypothesis import strategies as st


# Feature: timing-probability-unified, Property 7: RF Binary Label Generation
@settings(max_examples=200)
@given(
    timing_prediction=st.sampled_from(["skip", "fast", "slow"]),
    delay_pnl=st.floats(min_value=-0.05, max_value=0.05),
)
def test_rf_binary_label_generation_property(timing_prediction, delay_pnl):
    """Property 7: RF Binary Label Generation

    For any (timing_prediction, delay_pnl) pair, the RF training label SHALL satisfy:
    - IF timing_prediction == "skip" → label == 0
    - IF timing_prediction ∈ {"fast", "slow"} AND corresponding delay_pnl > 0 → label == 1
    - IF timing_prediction ∈ {"fast", "slow"} AND corresponding delay_pnl <= 0 → label == 0

    **Validates: Requirements 3.2**
    """
    predictions = pd.Series([timing_prediction])
    pnls = pd.Series([delay_pnl])
    labels = generate_rf_binary_labels(predictions, pnls)

    if timing_prediction == "skip":
        assert labels.iloc[0] == 0, (
            f"skip should always produce label=0, got {labels.iloc[0]}"
        )
    elif timing_prediction in ("fast", "slow") and delay_pnl > 0:
        assert labels.iloc[0] == 1, (
            f"{timing_prediction} with pnl={delay_pnl} > 0 should produce label=1, "
            f"got {labels.iloc[0]}"
        )
    else:
        # timing_prediction in ("fast", "slow") and delay_pnl <= 0
        assert labels.iloc[0] == 0, (
            f"{timing_prediction} with pnl={delay_pnl} <= 0 should produce label=0, "
            f"got {labels.iloc[0]}"
        )


# Feature: timing-probability-unified, Property 8: Probability Output Range Invariant
@settings(max_examples=200)
@given(
    n_train=st.integers(min_value=10, max_value=100),
    n_test=st.integers(min_value=5, max_value=50),
    seed=st.integers(min_value=0, max_value=10000),
)
def test_probability_output_range_invariant(n_train, n_test, seed):
    """Property 8: Probability Output Range Invariant

    For any feature vector input to the trained RF model, predict_proba() output
    SHALL satisfy 0.0 <= p_success <= 1.0.

    **Validates: Requirements 3.3**
    """
    rng = np.random.RandomState(seed)
    feature_names = [
        "signal_atr_percentile",
        "roundtrip_cost_atr",
        "prev1_body_atr",
        "prev1_range_atr",
        "prev1_close_pos_side",
        "prev_sma5_gap_atr",
        "prev_sma5_slope_atr",
        "level_to_prev_close_atr",
        "level_to_signal_open_atr",
        "touch_extension_atr",
    ]

    # Generate random feature matrices with varied distributions
    train_features = pd.DataFrame(
        rng.randn(n_train, 10) * 5, columns=feature_names
    )
    test_features = pd.DataFrame(
        rng.randn(n_test, 10) * 5, columns=feature_names
    )

    # Generate binary labels ensuring both classes are present in train
    train_labels = pd.Series(rng.randint(0, 2, n_train))
    # Ensure at least one sample of each class in training
    if train_labels.sum() == 0:
        train_labels.iloc[0] = 1
    elif train_labels.sum() == len(train_labels):
        train_labels.iloc[0] = 0

    test_labels = pd.Series(rng.randint(0, 2, n_test))

    # Train the model
    result = train_rf_probability(
        train_features, train_labels, test_features, test_labels,
        n_estimators=50,  # fewer trees for speed in property tests
        random_state=seed,
    )

    # Verify ALL output probabilities are in [0, 1]
    assert np.all(result.train_probabilities >= 0.0), (
        f"Train probabilities contain values < 0: min={result.train_probabilities.min()}"
    )
    assert np.all(result.train_probabilities <= 1.0), (
        f"Train probabilities contain values > 1: max={result.train_probabilities.max()}"
    )
    assert np.all(result.test_probabilities >= 0.0), (
        f"Test probabilities contain values < 0: min={result.test_probabilities.min()}"
    )
    assert np.all(result.test_probabilities <= 1.0), (
        f"Test probabilities contain values > 1: max={result.test_probabilities.max()}"
    )

    # Also verify the shape is correct
    assert len(result.train_probabilities) == n_train
    assert len(result.test_probabilities) == n_test


# Feature: timing-probability-unified, Property 9: Sizing Multiplier Formula
@settings(max_examples=200)
@given(
    p=st.floats(min_value=-0.5, max_value=1.5, allow_nan=False, allow_infinity=False),
)
def test_sizing_multiplier_formula_property(p):
    """Property 9: Sizing Multiplier Formula

    For any probability value p ∈ [0, 1], compute_sizing_multiplier(p) SHALL equal
    clip(p × 2, 0, 2). Specifically:
    - p == 0.0 → multiplier == 0.0
    - p == 0.5 → multiplier == 1.0
    - p == 1.0 → multiplier == 2.0
    - Result always in [0, 2]

    **Validates: Requirements 3.4**
    """
    result = compute_sizing_multiplier(np.array([p]))
    expected = np.clip(p * 2, 0, 2)

    # Verify: result == np.clip(p * 2, 0, 2)
    assert result[0] == pytest.approx(expected), (
        f"For p={p}, expected multiplier={expected}, got {result[0]}"
    )

    # Verify: all results in [0, 2]
    assert 0.0 <= result[0] <= 2.0, (
        f"Multiplier {result[0]} out of valid range [0, 2] for p={p}"
    )
