"""Tests for walk-forward Bayesian T3 event filtering."""

from __future__ import annotations

import pandas as pd

from timing_probability_unified import t3_bayesian_event_filter as bayes


def _row(month: str, event_id: str, side: str, speed: float, extension: float, pnl: float | None = None):
    base = {
        "event_id": event_id,
        "symbol": "ETHUSDT",
        "side": side,
        "month": month,
        "touch_time": f"{month}-15T00:00:00Z",
        "entry_time": f"{month}-15T00:01:00Z",
        "speed_300s_atr": speed,
        "touch_extension_atr": extension,
        "rf_probability": 0.52,
        "timing_prediction": "fast",
    }
    if pnl is not None:
        base["net_after_fee_pct"] = pnl
    return base


def test_walk_forward_scores_use_prior_months_only():
    trades = bayes.add_feature_buckets(
        pd.DataFrame(
            [
                _row("2025-06", "june_win_1", "short", -0.42, 0.03, 0.12),
                _row("2025-06", "june_win_2", "short", -0.45, 0.04, 0.08),
                _row("2025-07", "july_loss", "short", -0.43, 0.03, -0.50),
            ]
        )
    )
    events = bayes.add_feature_buckets(
        pd.DataFrame(
            [
                _row("2025-07", "july_event", "short", -0.44, 0.03),
                _row("2025-08", "aug_event", "short", -0.44, 0.03),
            ]
        )
    )

    scored = bayes.walk_forward_scores(
        trades,
        events,
        min_train_months=1,
        min_group_trades=2,
        prior_strength=0.0,
    )

    july_score = scored.loc[scored["event_id"] == "july_event", "bayes_mean_net_pct"].iloc[0]
    aug_score = scored.loc[scored["event_id"] == "aug_event", "bayes_mean_net_pct"].iloc[0]

    assert round(july_score, 6) == 0.1
    assert round(aug_score, 6) < 0.0


def test_threshold_summary_uses_scored_trade_labels():
    trades = bayes.add_feature_buckets(
        pd.DataFrame(
                [
                    _row("2025-06", "train_win_1", "short", -0.42, 0.03, 0.10),
                    _row("2025-06", "train_win_2", "short", -0.44, 0.04, 0.12),
                    _row("2025-06", "train_loss_1", "long", 0.20, 0.01, -0.10),
                    _row("2025-06", "train_loss_2", "long", 0.21, 0.01, -0.12),
                    _row("2025-07", "selected_trade", "short", -0.43, 0.03, 0.08),
                    _row("2025-07", "rejected_trade", "long", 0.20, 0.01, -0.20),
                ]
        )
    )
    events = bayes.add_feature_buckets(
        pd.DataFrame(
            [
                _row("2025-07", "selected_trade", "short", -0.43, 0.03),
                _row("2025-07", "rejected_trade", "long", 0.20, 0.01),
            ]
        )
    )
    scored = bayes.walk_forward_scores(
        trades,
        events,
        min_train_months=1,
        min_group_trades=2,
        prior_strength=0.0,
    )

    summary = bayes.summarize_thresholds(
        scored,
        trades,
        thresholds=[0.01],
        months=["2025-06", "2025-07"],
        symbols=["ETHUSDT"],
    )

    row = summary.iloc[0]
    assert row["threshold_label"] == "bayes_ge_0p010"
    assert row["selected_events"] == 1
    assert row["selected_labeled_trades"] == 1
    assert row["label_net_after_fee_pct"] == 0.08
