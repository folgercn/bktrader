"""Tests for order-book impact proxy helpers."""

from __future__ import annotations

import pandas as pd
import pytest

from timing_probability_unified import t3_overlay_orderbook_impact_sensitivity as impact


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


def test_apply_lead_scale_only_changes_lead_rows():
    windows = impact.apply_lead_scale(_windows(), 1.25)

    assert windows.loc[0, "desired_notional_share"] == 1.0
    assert windows.loc[0, "base_pnl_pct"] == 1.25
    assert windows.loc[1, "desired_notional_share"] == 0.6
    assert windows.loc[1, "base_pnl_pct"] == 0.6


def test_apply_leg_scales_can_scale_overlay_rows():
    windows = impact.apply_leg_scales(_windows(), lead_scale=1.25, overlay_scale=1.5)

    assert windows.loc[0, "desired_notional_share"] == 1.0
    assert windows.loc[0, "base_pnl_pct"] == 1.25
    assert windows.loc[1, "desired_notional_share"] == pytest.approx(0.9)
    assert windows.loc[1, "base_pnl_pct"] == pytest.approx(0.9)
    assert windows["overlay_scale"].tolist() == [1.5, 1.5]


def test_impact_round_trip_bps_uses_concentration_and_active_notional():
    profile = impact.ImpactProfile(
        name="test",
        top_book_capacity_share=0.5,
        excess_round_trip_bps_per_1x=10.0,
        active_round_trip_bps_per_1x=2.0,
    )

    bps = impact.impact_round_trip_bps(
        allocated_notional_share=0.8,
        active_notional_before=0.4,
        profile=profile,
    )

    assert bps == 3.8


def test_simulate_impact_applies_overlay_slip_and_impact_cost():
    profile = impact.ImpactProfile(
        name="test",
        top_book_capacity_share=1.0,
        excess_round_trip_bps_per_1x=10.0,
        active_round_trip_bps_per_1x=1.0,
    )

    scenario, ledger = impact.simulate_impact(
        _windows(),
        profile=profile,
        capital_capacity=2.0,
        overlay_extra_round_trip_slippage_bps=10.0,
        lead_scale=1.0,
        overlay_scale=1.0,
    )

    assert scenario.filled_trades == 2
    assert scenario.overlay_scale == 1.0
    assert scenario.impact_cost_pct == 0.0048
    assert scenario.overlay_extra_cost_pct == 0.06
    assert scenario.lead_pnl_pct == 1.0
    assert scenario.overlay_pnl_pct == 0.5352
    assert scenario.calendar_sum_pct == 1.5352
    assert ledger.loc[1, "impact_round_trip_bps"] == 0.8
