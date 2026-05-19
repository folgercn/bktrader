"""Tests for conditional lead-scale policy helpers."""

from __future__ import annotations

import pandas as pd

from timing_probability_unified import t3_overlay_conditional_lead_scale as conditional
from timing_probability_unified.t3_overlay_orderbook_impact_sensitivity import ImpactProfile


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
                "entry_time": pd.Timestamp("2025-06-01T03:00:00Z"),
                "exit_time": pd.Timestamp("2025-06-01T04:00:00Z"),
                "entry_month": "2025-06",
                "desired_notional_share": 0.4,
                "base_pnl_pct": 0.4,
            },
        ]
    )


def test_conditional_policy_applies_scale_when_gate_passes():
    profile = ImpactProfile("test", 1.0, 10.0, 0.0)

    scenario, ledger = conditional.simulate_conditional_policy(
        _windows(),
        profile=profile,
        target_lead_scale=1.25,
        lead_impact_gate_round_trip_bps=0.0,
        capital_capacity=2.0,
        overlay_extra_round_trip_slippage_bps=0.0,
    )

    assert scenario.lead_scale_applied_trades == 1
    assert scenario.lead_scale_blocked_trades == 0
    assert scenario.lead_pnl_pct == 1.25
    assert scenario.calendar_sum_pct == 1.65
    assert ledger.loc[0, "chosen_lead_scale"] == 1.25


def test_conditional_policy_blocks_scale_when_impact_gate_fails():
    profile = ImpactProfile("test", 0.5, 10.0, 0.0)

    scenario, ledger = conditional.simulate_conditional_policy(
        _windows(),
        profile=profile,
        target_lead_scale=1.25,
        lead_impact_gate_round_trip_bps=1.0,
        capital_capacity=2.0,
        overlay_extra_round_trip_slippage_bps=0.0,
    )

    assert scenario.lead_scale_applied_trades == 0
    assert scenario.lead_scale_blocked_trades == 1
    assert scenario.lead_pnl_pct == 0.976
    assert scenario.calendar_sum_pct == 1.376
    assert ledger.loc[0, "chosen_lead_scale"] == 1.0
    assert ledger.loc[0, "impact_round_trip_bps"] == 3.0


def test_conditional_policy_applies_overlay_scale_when_gate_passes():
    profile = ImpactProfile("test", 1.0, 10.0, 0.0)

    scenario, ledger = conditional.simulate_conditional_policy(
        _windows(),
        profile=profile,
        target_lead_scale=1.0,
        target_overlay_scale=2.0,
        lead_impact_gate_round_trip_bps=0.0,
        overlay_impact_gate_round_trip_bps=0.0,
        capital_capacity=2.0,
        overlay_extra_round_trip_slippage_bps=0.0,
    )

    assert scenario.overlay_scale_applied_trades == 1
    assert scenario.overlay_scale_blocked_trades == 0
    assert scenario.overlay_pnl_pct == 0.8
    assert scenario.calendar_sum_pct == 1.8
    assert ledger.loc[1, "chosen_overlay_scale"] == 2.0


def test_conditional_policy_blocks_overlay_scale_when_impact_gate_fails():
    profile = ImpactProfile("test", 0.2, 10.0, 0.0)

    scenario, ledger = conditional.simulate_conditional_policy(
        _windows(),
        profile=profile,
        target_lead_scale=1.0,
        target_overlay_scale=2.0,
        lead_impact_gate_round_trip_bps=0.0,
        overlay_impact_gate_round_trip_bps=1.0,
        capital_capacity=2.0,
        overlay_extra_round_trip_slippage_bps=0.0,
    )

    assert scenario.overlay_scale_applied_trades == 0
    assert scenario.overlay_scale_blocked_trades == 1
    assert scenario.overlay_pnl_pct == 0.392
    assert ledger.loc[1, "chosen_overlay_scale"] == 1.0
    assert ledger.loc[1, "impact_round_trip_bps"] == 2.0
