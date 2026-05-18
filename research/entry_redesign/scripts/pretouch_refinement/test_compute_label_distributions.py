"""
Tests for compute_label_distributions() in regime_labels.py
"""

import pandas as pd
import pytest

from pretouch_refinement.regime_labels import compute_label_distributions


def _make_test_data():
    """Create minimal test data for compute_label_distributions."""
    # 10 events total: 7 train, 3 test
    n = 10
    labels_5regime = pd.Series(
        ["D0", "D5", "D10", "D15", "pullback", "D0", "D5", "D10", "D15", "pullback"],
        name="regime_5_label",
    )
    labels_3regime = pd.Series(
        ["fast", "fast", "slow", "slow", "slow", "skip", "fast", "slow", "skip", "fast"],
        name="regime_3_label",
    )
    labels_2regime = pd.Series(
        ["enter", "enter", "skip", "enter", "skip", "enter", "enter", "skip", "enter", "skip"],
        name="regime_2_label",
    )
    train_mask = pd.Series([True] * 7 + [False] * 3)
    return labels_5regime, labels_3regime, labels_2regime, train_mask


class TestComputeLabelDistributions:
    """Tests for compute_label_distributions."""

    def test_returns_dataframe_and_bool(self):
        """Should return a tuple of (DataFrame, bool)."""
        labels_5, labels_3, labels_2, train_mask = _make_test_data()
        result = compute_label_distributions(labels_5, labels_3, labels_2, train_mask)
        assert isinstance(result, tuple)
        assert len(result) == 2
        assert isinstance(result[0], pd.DataFrame)
        assert isinstance(result[1], bool)

    def test_dataframe_columns(self):
        """DataFrame should have correct columns."""
        labels_5, labels_3, labels_2, train_mask = _make_test_data()
        df, _ = compute_label_distributions(labels_5, labels_3, labels_2, train_mask)
        assert list(df.columns) == ["regime_schema", "label", "split", "count", "pct"]

    def test_all_three_schemas_present(self):
        """All three regime schemas should be present in output."""
        labels_5, labels_3, labels_2, train_mask = _make_test_data()
        df, _ = compute_label_distributions(labels_5, labels_3, labels_2, train_mask)
        schemas = set(df["regime_schema"].unique())
        assert schemas == {"5-regime", "3-regime", "2-regime"}

    def test_train_test_splits_present(self):
        """Both train and test splits should be present."""
        labels_5, labels_3, labels_2, train_mask = _make_test_data()
        df, _ = compute_label_distributions(labels_5, labels_3, labels_2, train_mask)
        splits = set(df["split"].unique())
        assert splits == {"train", "test"}

    def test_pct_sums_to_100(self):
        """Percentages within each (schema, split) should sum to ~100."""
        labels_5, labels_3, labels_2, train_mask = _make_test_data()
        df, _ = compute_label_distributions(labels_5, labels_3, labels_2, train_mask)
        for (schema, split), group in df.groupby(["regime_schema", "split"]):
            total_pct = group["pct"].sum()
            assert abs(total_pct - 100.0) < 0.1, (
                f"Pct sum for {schema}/{split} = {total_pct}, expected ~100"
            )

    def test_counts_match_data(self):
        """Counts should match actual label occurrences."""
        labels_5, labels_3, labels_2, train_mask = _make_test_data()
        df, _ = compute_label_distributions(labels_5, labels_3, labels_2, train_mask)

        # Check 2-regime train: 7 train events, labels are enter,enter,skip,enter,skip,enter,enter
        regime2_train = df[(df["regime_schema"] == "2-regime") & (df["split"] == "train")]
        enter_row = regime2_train[regime2_train["label"] == "enter"]
        skip_row = regime2_train[regime2_train["label"] == "skip"]
        assert enter_row["count"].values[0] == 5
        assert skip_row["count"].values[0] == 2

    def test_regime2_imbalanced_false_normal(self):
        """Should return False when enter pct is between 40% and 90%."""
        labels_5, labels_3, labels_2, train_mask = _make_test_data()
        # In our test data: 5/7 = 71.4% enter in train → not imbalanced
        _, imbalanced = compute_label_distributions(labels_5, labels_3, labels_2, train_mask)
        assert imbalanced is False

    def test_regime2_imbalanced_true_low_enter(self):
        """Should return True when enter pct < 40%."""
        n = 10
        labels_5 = pd.Series(["D0"] * n)
        labels_3 = pd.Series(["fast"] * n)
        # Only 2 out of 7 train events are "enter" → 28.6% < 40%
        labels_2 = pd.Series(
            ["enter", "enter", "skip", "skip", "skip", "skip", "skip", "enter", "skip", "skip"]
        )
        train_mask = pd.Series([True] * 7 + [False] * 3)
        _, imbalanced = compute_label_distributions(labels_5, labels_3, labels_2, train_mask)
        assert imbalanced is True

    def test_regime2_imbalanced_true_high_enter(self):
        """Should return True when enter pct > 90%."""
        n = 10
        labels_5 = pd.Series(["D0"] * n)
        labels_3 = pd.Series(["fast"] * n)
        # All 7 train events are "enter" → 100% > 90%
        labels_2 = pd.Series(["enter"] * 7 + ["skip"] * 3)
        train_mask = pd.Series([True] * 7 + [False] * 3)
        _, imbalanced = compute_label_distributions(labels_5, labels_3, labels_2, train_mask)
        assert imbalanced is True

    def test_regime2_imbalanced_boundary_40_not_triggered(self):
        """Exactly 40% enter should NOT be imbalanced."""
        n = 10
        labels_5 = pd.Series(["D0"] * n)
        labels_3 = pd.Series(["fast"] * n)
        # 4 out of 10 train events are "enter" → exactly 40%
        labels_2 = pd.Series(
            ["enter", "enter", "enter", "enter", "skip", "skip", "skip", "skip", "skip", "skip"]
        )
        train_mask = pd.Series([True] * 10)  # all train
        _, imbalanced = compute_label_distributions(labels_5, labels_3, labels_2, train_mask)
        assert imbalanced is False

    def test_regime2_imbalanced_boundary_90_not_triggered(self):
        """Exactly 90% enter should NOT be imbalanced."""
        n = 10
        labels_5 = pd.Series(["D0"] * n)
        labels_3 = pd.Series(["fast"] * n)
        # 9 out of 10 train events are "enter" → exactly 90%
        labels_2 = pd.Series(["enter"] * 9 + ["skip"])
        train_mask = pd.Series([True] * 10)  # all train
        _, imbalanced = compute_label_distributions(labels_5, labels_3, labels_2, train_mask)
        assert imbalanced is False
