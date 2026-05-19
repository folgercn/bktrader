"""Tests for T3 overlay lead portfolio sensitivity helpers."""

from __future__ import annotations

import pandas as pd

from timing_probability_unified import t3_overlay_lead_portfolio_sensitivity as sensitivity


def _windows() -> pd.DataFrame:
    return pd.DataFrame(
        [
            {
                "source": "lead",
                "entry_time": pd.Timestamp("2025-06-01T00:00:00Z"),
                "exit_time": pd.Timestamp("2025-06-01T02:00:00Z"),
                "entry_month": "2025-06",
                "desired_notional_share": 0.8,
                "base_pnl_pct": 1.0,
            },
            {
                "source": "overlay",
                "entry_time": pd.Timestamp("2025-06-01T01:00:00Z"),
                "exit_time": pd.Timestamp("2025-06-01T03:00:00Z"),
                "entry_month": "2025-06",
                "desired_notional_share": 0.6,
                "base_pnl_pct": 0.6,
            },
        ]
    )


def test_scale_to_available_scales_overlapping_trade():
    scenario, ledger = sensitivity.simulate_portfolio(
        _windows(),
        capital_capacity=1.0,
        overlay_extra_round_trip_slippage_bps=0.0,
        policy="scale_to_available",
    )

    assert scenario.filled_trades == 2
    assert scenario.scaled_trades == 1
    assert scenario.skipped_trades == 0
    assert scenario.peak_active_notional_share == 1.0
    assert round(float(ledger.loc[1, "allocation_scale"]), 6) == round(0.2 / 0.6, 6)
    assert scenario.calendar_sum_pct == 1.2


def test_skip_if_insufficient_rejects_overlapping_trade():
    scenario, ledger = sensitivity.simulate_portfolio(
        _windows(),
        capital_capacity=1.0,
        overlay_extra_round_trip_slippage_bps=0.0,
        policy="skip_if_insufficient",
    )

    assert scenario.filled_trades == 1
    assert scenario.skipped_trades == 1
    assert scenario.overlay_filled_trades == 0
    assert ledger.loc[1, "allocation_status"] == "skipped"
    assert scenario.calendar_sum_pct == 1.0


def test_overlay_extra_round_trip_slippage_haircuts_overlay_only():
    scenario, ledger = sensitivity.simulate_portfolio(
        _windows(),
        capital_capacity=2.0,
        overlay_extra_round_trip_slippage_bps=10.0,
        policy="scale_to_available",
    )

    assert scenario.lead_pnl_pct == 1.0
    assert scenario.overlay_pnl_pct == 0.54
    assert ledger.loc[1, "adjusted_base_pnl_pct"] == 0.54
    assert scenario.calendar_sum_pct == 1.54


def test_lead_window_precision_detects_exact_source():
    windows = _windows()
    windows["window_source"] = ["lead_exact_adverse10", "t3_overlay_actual"]

    precision, sources, note = sensitivity._lead_window_precision(windows)

    assert precision == "exact"
    assert sources == ["lead_exact_adverse10"]
    assert "DelayResult" in note
