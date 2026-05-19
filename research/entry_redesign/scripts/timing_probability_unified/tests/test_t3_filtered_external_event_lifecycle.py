"""Tests for filtered T3 external event lifecycle runner."""

from __future__ import annotations

import pandas as pd

from timing_probability_unified.t3_filtered_external_event_lifecycle import (
    FilteredT3Spec,
    apply_spec,
    build_specs,
    filter_specs,
)


def _events() -> pd.DataFrame:
    return pd.DataFrame(
        [
            {
                "symbol": "ETHUSDT",
                "side": "short",
                "speed_300s_atr": -0.42,
                "touch_extension_atr": 0.03,
                "rf_probability": 0.52,
                "timing_prediction": "fast",
            },
            {
                "symbol": "ETHUSDT",
                "side": "short",
                "speed_300s_atr": -0.20,
                "touch_extension_atr": 0.12,
                "rf_probability": 0.71,
                "timing_prediction": "fast",
            },
            {
                "symbol": "ETHUSDT",
                "side": "long",
                "speed_300s_atr": 0.45,
                "touch_extension_atr": 0.03,
                "rf_probability": 0.53,
                "timing_prediction": "skip",
            },
        ]
    )


def test_apply_spec_filters_short_speed_bucket_with_abs_speed():
    filtered = apply_spec(
        _events(),
        FilteredT3Spec(
            "candidate",
            side="short",
            speed_abs_min=0.35,
            speed_abs_max=0.50,
        ),
    )

    assert len(filtered) == 1
    assert filtered.iloc[0]["rf_probability"] == 0.52


def test_apply_spec_filters_non_monotonic_rf_bucket():
    filtered = apply_spec(
        _events(),
        FilteredT3Spec("candidate", side="short", rf_min=0.50, rf_max=0.55),
    )

    assert len(filtered) == 1
    assert filtered.iloc[0]["speed_300s_atr"] == -0.42


def test_build_and_filter_specs_keep_requested_order():
    specs = build_specs("smoke")
    assert [spec.label for spec in specs] == ["short_all", "short_speed_abs_ge_0p35"]

    focused = build_specs("focused")
    selected = filter_specs(focused, ["long_speed_abs_ge_0p35", "short_rf_0p50_0p55", "short_all"])
    assert [spec.label for spec in selected] == [
        "long_speed_abs_ge_0p35",
        "short_rf_0p50_0p55",
        "short_all",
    ]
