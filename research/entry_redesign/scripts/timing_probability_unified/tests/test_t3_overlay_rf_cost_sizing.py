"""Tests for T3 overlay RF/cost sizing helpers."""

from __future__ import annotations

import pandas as pd
import pytest

from timing_probability_unified import t3_overlay_rf_cost_sizing as sizing
from timing_probability_unified.t3_lifecycle_outcome_diagnostics import pair_lifecycle_trades


def test_pair_lifecycle_trades_carries_external_rf_cost_metadata():
    second_bars = pd.DataFrame(
        {
            "open": [100.0, 101.0],
            "high": [102.0, 103.0],
            "low": [99.0, 100.0],
            "close": [101.0, 102.0],
        },
        index=pd.to_datetime(["2026-01-01T00:00:00Z", "2026-01-01T00:00:01Z"]),
    )
    ledger = pd.DataFrame(
        [
            {
                "time": pd.Timestamp("2026-01-01T00:00:00Z"),
                "type": "BUY",
                "reason": "External-NextSecond-Adverse",
                "price": 100.0,
                "notional": 40000.0,
                "breakout_shape_name": "t3_swing",
                "external_event_key": "event-1",
                "rf_probability": 0.63,
                "cost_penalty": 0.8,
                "roundtrip_cost_atr": 0.12,
                "speed_300s_atr": 0.42,
                "eff_300s": 0.9,
                "touch_extension_atr": 0.03,
                "pre_touch_seconds": 120.0,
            },
            {
                "time": pd.Timestamp("2026-01-01T00:00:01Z"),
                "type": "EXIT",
                "reason": "PT",
                "price": 101.0,
                "notional": 40000.0,
            },
        ]
    )

    trades = pair_lifecycle_trades(
        ledger,
        second_bars,
        symbol="ETHUSDT",
        month="2026-01",
        initial_balance=100000.0,
    )

    assert len(trades) == 1
    row = trades.iloc[0]
    assert row["external_event_key"] == "event-1"
    assert row["rf_probability"] == 0.63
    assert row["cost_penalty"] == 0.8
    assert row["roundtrip_cost_atr"] == 0.12


def _trades() -> pd.DataFrame:
    return pd.DataFrame(
        [
            {
                "external_event_key": "event-a",
                "month": "2026-01",
                "symbol": "ETHUSDT",
                "side": "long",
                "entry_time": "2026-01-01T00:00:00Z",
                "exit_time": "2026-01-01T01:00:00Z",
                "notional": 40000.0,
                "pnl_initial_pct": 0.40,
                "rf_probability": 0.70,
                "cost_penalty": 1.0,
                "roundtrip_cost_atr": 0.10,
                "speed_300s_atr": 0.45,
                "eff_300s": 0.9,
                "touch_extension_atr": 0.02,
                "pre_touch_seconds": 120.0,
            },
            {
                "external_event_key": "event-b",
                "month": "2026-01",
                "symbol": "ETHUSDT",
                "side": "short",
                "entry_time": "2026-01-02T00:00:00Z",
                "exit_time": "2026-01-02T01:00:00Z",
                "notional": 40000.0,
                "pnl_initial_pct": -0.20,
                "rf_probability": 0.30,
                "cost_penalty": 0.5,
                "roundtrip_cost_atr": 0.20,
                "speed_300s_atr": -0.50,
                "eff_300s": 0.8,
                "touch_extension_atr": 0.04,
                "pre_touch_seconds": 180.0,
            },
        ]
    )


def test_frozen_rf_cost_sizing_downweights_low_rf_high_cost_event():
    events = sizing.build_event_table(_trades(), initial_balance=100000.0)
    variant = sizing.SizingVariant(
        label="frozen",
        method="frozen_rf_cost",
        max_multiplier=1.0,
        min_multiplier=0.0,
        live_compatible=True,
    )

    scored = sizing.score_events_for_variant(
        events,
        variant,
        months=["2026-01"],
        cost_threshold_atr=0.10,
        random_state=42,
    )
    weighted = sizing.apply_event_scores_to_trades(_trades(), scored, initial_balance=100000.0)
    metrics = sizing.summarize_variant(
        variant=variant,
        trades=weighted,
        event_scores=scored,
        months=["2026-01"],
        fixed_overlay_pct=0.20,
    )

    multipliers = dict(zip(scored["external_event_key"], scored["event_multiplier"]))
    assert multipliers["event-a"] == pytest.approx(1.0)
    assert multipliers["event-b"] == pytest.approx(0.30)
    assert metrics.calendar_sum_pct == pytest.approx(0.34)
    assert metrics.overlay_delta_vs_fixed_pct == pytest.approx(0.14)
