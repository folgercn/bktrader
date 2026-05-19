"""Tests for feature_extractor.impute_features() and feature_statistics()."""

from __future__ import annotations

import numpy as np
import pandas as pd
import pytest

from feature_extractor import impute_features, feature_statistics


class TestImputeFeatures:
    """Tests for impute_features() — median imputation from train set only."""

    def test_basic_imputation(self):
        """Median computed from train fills NaN in both train and test."""
        train = pd.DataFrame({
            "feat_a": [1.0, 2.0, 3.0, np.nan, 5.0],
            "feat_b": [10.0, 20.0, np.nan, 40.0, 50.0],
        })
        test = pd.DataFrame({
            "feat_a": [np.nan, 7.0],
            "feat_b": [np.nan, 60.0],
        })

        imputed_train, imputed_test, stats = impute_features(train, test)

        # feat_a: train median = median([1,2,3,5]) = 2.5
        assert imputed_train["feat_a"].iloc[3] == pytest.approx(2.5)
        assert imputed_test["feat_a"].iloc[0] == pytest.approx(2.5)

        # feat_b: train median = median([10,20,40,50]) = 30.0
        assert imputed_train["feat_b"].iloc[2] == pytest.approx(30.0)
        assert imputed_test["feat_b"].iloc[0] == pytest.approx(30.0)

    def test_no_nan_after_imputation(self):
        """After imputation, no NaN values remain."""
        train = pd.DataFrame({
            "x": [1.0, np.nan, 3.0, np.nan, 5.0],
        })
        test = pd.DataFrame({
            "x": [np.nan, np.nan, 8.0],
        })

        imputed_train, imputed_test, _ = impute_features(train, test)

        assert imputed_train.isna().sum().sum() == 0
        assert imputed_test.isna().sum().sum() == 0

    def test_does_not_modify_input(self):
        """Input DataFrames must not be modified."""
        train = pd.DataFrame({"a": [1.0, np.nan, 3.0]})
        test = pd.DataFrame({"a": [np.nan, 5.0]})

        train_copy = train.copy()
        test_copy = test.copy()

        impute_features(train, test)

        pd.testing.assert_frame_equal(train, train_copy)
        pd.testing.assert_frame_equal(test, test_copy)

    def test_preserves_index(self):
        """Original index is preserved in output."""
        train = pd.DataFrame(
            {"a": [1.0, np.nan, 3.0]},
            index=[10, 20, 30],
        )
        test = pd.DataFrame(
            {"a": [np.nan, 5.0]},
            index=[100, 200],
        )

        imputed_train, imputed_test, _ = impute_features(train, test)

        assert list(imputed_train.index) == [10, 20, 30]
        assert list(imputed_test.index) == [100, 200]

    def test_imputation_stats_structure(self):
        """Stats dict has correct keys and values."""
        train = pd.DataFrame({
            "feat_a": [1.0, 2.0, np.nan, 4.0],
            "feat_b": [10.0, 20.0, 30.0, 40.0],
        })
        test = pd.DataFrame({
            "feat_a": [np.nan, 6.0, np.nan],
            "feat_b": [50.0, np.nan, 70.0],
        })

        _, _, stats = impute_features(train, test)

        # feat_a stats
        assert stats["feat_a"]["train_missing_count"] == 1
        assert stats["feat_a"]["test_missing_count"] == 2
        assert stats["feat_a"]["train_missing_rate"] == pytest.approx(1 / 4)
        assert stats["feat_a"]["test_missing_rate"] == pytest.approx(2 / 3)
        # median of [1, 2, 4] = 2.0
        assert stats["feat_a"]["median"] == pytest.approx(2.0)

        # feat_b stats
        assert stats["feat_b"]["train_missing_count"] == 0
        assert stats["feat_b"]["test_missing_count"] == 1
        assert stats["feat_b"]["train_missing_rate"] == pytest.approx(0.0)
        assert stats["feat_b"]["test_missing_rate"] == pytest.approx(1 / 3)
        # median of [10, 20, 30, 40] = 25.0
        assert stats["feat_b"]["median"] == pytest.approx(25.0)

    def test_no_missing_values(self):
        """When no NaN exists, imputation is a no-op."""
        train = pd.DataFrame({"a": [1.0, 2.0, 3.0]})
        test = pd.DataFrame({"a": [4.0, 5.0]})

        imputed_train, imputed_test, stats = impute_features(train, test)

        pd.testing.assert_frame_equal(imputed_train, train)
        pd.testing.assert_frame_equal(imputed_test, test)
        assert stats["a"]["train_missing_count"] == 0
        assert stats["a"]["test_missing_count"] == 0

    def test_median_uses_only_train(self):
        """Median is computed from train only, not influenced by test values."""
        # Train median of [1, 3] = 2.0
        train = pd.DataFrame({"a": [1.0, 3.0, np.nan]})
        # Test has large values that should NOT affect the median
        test = pd.DataFrame({"a": [np.nan, 1000.0, 2000.0]})

        imputed_train, imputed_test, stats = impute_features(train, test)

        assert stats["a"]["median"] == pytest.approx(2.0)
        assert imputed_train["a"].iloc[2] == pytest.approx(2.0)
        assert imputed_test["a"].iloc[0] == pytest.approx(2.0)

    def test_empty_dataframes(self):
        """Handles empty DataFrames gracefully."""
        train = pd.DataFrame({"a": pd.Series([], dtype=float)})
        test = pd.DataFrame({"a": pd.Series([], dtype=float)})

        imputed_train, imputed_test, stats = impute_features(train, test)

        assert len(imputed_train) == 0
        assert len(imputed_test) == 0


class TestFeatureStatistics:
    """Tests for feature_statistics() — descriptive stats + per-regime distribution."""

    def test_basic_output_structure(self):
        """Output has correct index (feature names) and expected columns."""
        features = pd.DataFrame({
            "feat_a": [1.0, 2.0, 3.0, 4.0],
            "feat_b": [10.0, 20.0, 30.0, 40.0],
        })
        labels = pd.Series(["D0", "D5", "D0", "D5"])

        result = feature_statistics(features, labels)

        # Index should be feature names
        assert list(result.index) == ["feat_a", "feat_b"]
        # Must have global stats columns
        for col in ["mean", "std", "min", "max", "missing_rate"]:
            assert col in result.columns
        # Must have per-regime columns
        for regime in ["D0", "D5"]:
            assert f"{regime}_mean" in result.columns
            assert f"{regime}_std" in result.columns

    def test_global_statistics_correctness(self):
        """Global mean, std, min, max, missing_rate are computed correctly."""
        features = pd.DataFrame({
            "x": [1.0, 2.0, 3.0, 4.0, 5.0],
        })
        labels = pd.Series(["D0", "D0", "D5", "D5", "D10"])

        result = feature_statistics(features, labels)

        assert result.loc["x", "mean"] == pytest.approx(3.0)
        assert result.loc["x", "std"] == pytest.approx(pd.Series([1, 2, 3, 4, 5.0]).std())
        assert result.loc["x", "min"] == pytest.approx(1.0)
        assert result.loc["x", "max"] == pytest.approx(5.0)
        assert result.loc["x", "missing_rate"] == pytest.approx(0.0)

    def test_missing_rate_calculation(self):
        """Missing rate correctly reflects NaN proportion."""
        features = pd.DataFrame({
            "a": [1.0, np.nan, 3.0, np.nan],
        })
        labels = pd.Series(["D0", "D0", "D5", "D5"])

        result = feature_statistics(features, labels)

        assert result.loc["a", "missing_rate"] == pytest.approx(0.5)

    def test_per_regime_statistics(self):
        """Per-regime mean and std are computed correctly."""
        features = pd.DataFrame({
            "x": [10.0, 20.0, 100.0, 200.0],
        })
        labels = pd.Series(["D0", "D0", "D5", "D5"])

        result = feature_statistics(features, labels)

        # D0 group: [10, 20] → mean=15, std=std([10,20])
        assert result.loc["x", "D0_mean"] == pytest.approx(15.0)
        assert result.loc["x", "D0_std"] == pytest.approx(pd.Series([10.0, 20.0]).std())
        # D5 group: [100, 200] → mean=150, std=std([100,200])
        assert result.loc["x", "D5_mean"] == pytest.approx(150.0)
        assert result.loc["x", "D5_std"] == pytest.approx(pd.Series([100.0, 200.0]).std())

    def test_all_nan_feature(self):
        """Feature with all NaN values produces NaN for mean/std/min/max."""
        features = pd.DataFrame({
            "all_nan": [np.nan, np.nan, np.nan],
            "normal": [1.0, 2.0, 3.0],
        })
        labels = pd.Series(["D0", "D5", "D10"])

        result = feature_statistics(features, labels)

        assert pd.isna(result.loc["all_nan", "mean"])
        assert pd.isna(result.loc["all_nan", "std"])
        assert pd.isna(result.loc["all_nan", "min"])
        assert pd.isna(result.loc["all_nan", "max"])
        assert result.loc["all_nan", "missing_rate"] == pytest.approx(1.0)

    def test_single_sample_regime_std_is_nan(self):
        """Regime with single sample produces NaN for std."""
        features = pd.DataFrame({
            "x": [1.0, 2.0, 3.0],
        })
        labels = pd.Series(["D0", "D5", "D10"])  # each regime has 1 sample

        result = feature_statistics(features, labels)

        # Single sample → std is NaN
        assert pd.isna(result.loc["x", "D0_std"])
        assert pd.isna(result.loc["x", "D5_std"])
        assert pd.isna(result.loc["x", "D10_std"])
        # But mean should still be valid
        assert result.loc["x", "D0_mean"] == pytest.approx(1.0)
        assert result.loc["x", "D5_mean"] == pytest.approx(2.0)
        assert result.loc["x", "D10_mean"] == pytest.approx(3.0)

    def test_multiple_regimes(self):
        """Handles all expected regime labels correctly."""
        features = pd.DataFrame({
            "x": [1.0, 2.0, 3.0, 4.0, 5.0, 6.0],
        })
        labels = pd.Series(["D0", "D5", "D10", "D15", "pullback", "skip"])

        result = feature_statistics(features, labels)

        # All regimes should have columns
        for regime in ["D0", "D5", "D10", "D15", "pullback", "skip"]:
            assert f"{regime}_mean" in result.columns
            assert f"{regime}_std" in result.columns

    def test_nan_in_regime_group(self):
        """NaN values within a regime group are handled (excluded from mean/std)."""
        features = pd.DataFrame({
            "x": [1.0, np.nan, 3.0, 4.0],
        })
        labels = pd.Series(["D0", "D0", "D5", "D5"])

        result = feature_statistics(features, labels)

        # D0 group: [1.0, NaN] → mean=1.0, std=NaN (only 1 non-NaN)
        assert result.loc["x", "D0_mean"] == pytest.approx(1.0)
        assert pd.isna(result.loc["x", "D0_std"])
        # D5 group: [3.0, 4.0] → mean=3.5
        assert result.loc["x", "D5_mean"] == pytest.approx(3.5)

    def test_index_name(self):
        """Result DataFrame index is named 'feature'."""
        features = pd.DataFrame({"a": [1.0, 2.0]})
        labels = pd.Series(["D0", "D5"])

        result = feature_statistics(features, labels)

        assert result.index.name == "feature"
