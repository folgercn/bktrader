"""Tests for T3 improvement sweep helpers."""

import pandas as pd
import pytest

from timing_probability_unified.t3_improvement_sweep import (
    T3GateSpec,
    active_entries,
    apply_gate,
    compute_gate_metrics,
    train_thresholds,
)


def _make_trades() -> pd.DataFrame:
    return pd.DataFrame({
        "event_id": ["a", "b", "c", "d"],
        "symbol": ["ETHUSDT"] * 4,
        "side": ["long", "long", "short", "short"],
        "touch_time": pd.to_datetime([
            "2026-02-01 00:00:00",
            "2026-02-02 00:00:00",
            "2026-02-03 00:00:00",
            "2026-02-04 00:00:00",
        ], utc=True),
        "timing_prediction": ["fast", "skip", "slow", "fast"],
        "speed_gate_pass": [True, True, True, False],
        "rf_probability": [0.70, 0.90, 0.50, 0.80],
        "speed_300s_atr": [0.40, 0.90, 0.20, 0.80],
        "pre_touch_seconds": [250.0, 200.0, 700.0, 100.0],
        "eff_300s": [0.40, 0.20, 0.80, 0.30],
        "touch_extension_atr": [0.20, 0.10, 0.60, 0.20],
        "weighted_pnl": [0.03, 0.00, -0.01, 0.05],
    })


def test_active_entries_requires_timing_and_speed_gate():
    active = active_entries(_make_trades())

    assert active["event_id"].tolist() == ["a", "c"]


def test_train_thresholds_use_active_train_only():
    spec = T3GateSpec(speed_abs_train_quantile=0.50)

    thresholds = train_thresholds(_make_trades(), spec)

    assert thresholds["speed_abs_min"] == pytest.approx(0.30)


def test_apply_gate_uses_thresholds_and_literal_filters():
    spec = T3GateSpec(
        rf_min=0.60,
        speed_abs_train_quantile=0.50,
        pre_touch_max=300.0,
        eff_max=0.50,
        touch_extension_abs_max=0.50,
        side="long",
    )
    thresholds = {"speed_abs_min": 0.30}

    gated = apply_gate(_make_trades(), spec, thresholds)

    assert gated["event_id"].tolist() == ["a"]


def test_compute_gate_metrics_rolls_months_and_counts_negatives():
    spec = T3GateSpec(rf_min=0.40)
    windows = [("2026-02-01", _make_trades(), {})]

    metrics = compute_gate_metrics(windows, spec)

    assert metrics.total_cs == pytest.approx(0.02)
    assert metrics.trade_count == 2
    assert metrics.months == {"2026-02": 0.02}
    assert metrics.negative_months == 0
