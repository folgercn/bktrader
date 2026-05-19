"""Tests for T3 overlay lead exposure-audit helpers."""

from __future__ import annotations

import pandas as pd

from timing_probability_unified import t3_overlay_lead_exposure_audit as audit


def test_lead_windows_parse_delay_and_fractional_pnl(tmp_path):
    lead_path = tmp_path / "lead.csv"
    pd.DataFrame(
        [
            {
                "touch_time": "2025-06-01T00:00:00Z",
                "selected_delay": "D30",
                "position_size": 0.2,
                "weighted_pnl": 0.0125,
            },
            {
                "touch_time": "2025-07-01T00:00:00Z",
                "selected_delay": "pullback",
                "position_size": 0.1,
                "weighted_pnl": -0.002,
            },
        ]
    ).to_csv(lead_path, index=False)

    windows = audit._load_lead_windows(lead_path, ["2025-06"])

    assert len(windows) == 1
    assert windows.loc[0, "entry_time"] == pd.Timestamp("2025-06-01T00:00:30Z")
    assert windows.loc[0, "exit_time"] == pd.Timestamp("2025-06-01T02:00:30Z")
    assert windows.loc[0, "notional_share"] == 0.2
    assert windows.loc[0, "weighted_pnl_pct"] == 1.25


def test_overlap_summary_uses_unique_window_pnl_once():
    lead = pd.DataFrame(
        [
            {
                "entry_time": pd.Timestamp("2025-06-01T00:00:00Z"),
                "exit_time": pd.Timestamp("2025-06-01T01:00:00Z"),
                "notional_share": 0.2,
                "weighted_pnl_pct": 1.0,
            },
            {
                "entry_time": pd.Timestamp("2025-06-01T00:30:00Z"),
                "exit_time": pd.Timestamp("2025-06-01T01:30:00Z"),
                "notional_share": 0.1,
                "weighted_pnl_pct": 2.0,
            },
        ]
    )
    overlay = pd.DataFrame(
        [
            {
                "entry_time": pd.Timestamp("2025-06-01T00:45:00Z"),
                "exit_time": pd.Timestamp("2025-06-01T01:15:00Z"),
                "notional_share": 0.4,
                "pnl_initial_pct": -0.5,
                "month": "2025-06",
            }
        ]
    )

    overlaps = audit._overlap_pairs(lead, overlay)
    summary = audit.summarize_overlaps(lead, overlay, overlaps)

    assert len(overlaps) == 2
    assert summary.lead_windows_with_overlap == 2
    assert summary.overlay_windows_with_overlap == 1
    assert summary.max_combined_notional_share == 0.6
    assert summary.overlap_overlay_pnl_pct == -0.5
    assert summary.overlap_lead_weighted_pnl_pct == 3.0


def test_round_trip_fee_adjustment_matches_lifecycle_commission():
    trades = pd.DataFrame(
        [
            {
                "notional": 40000.0,
                "pnl_initial_pct": 0.5,
                "pnl_bps": 12.5,
            }
        ]
    )

    adjusted = audit._apply_round_trip_fee_adjustment(trades, initial_balance=100000.0)

    assert adjusted.loc[0, "gross_pnl_initial_pct"] == 0.5
    assert adjusted.loc[0, "round_trip_fee_initial_pct"] == 0.08
    assert adjusted.loc[0, "pnl_initial_pct"] == 0.42
    assert adjusted.loc[0, "gross_pnl_bps"] == 12.5
    assert adjusted.loc[0, "round_trip_fee_bps"] == 20.0
    assert adjusted.loc[0, "pnl_bps"] == -7.5
    assert adjusted.loc[0, "outcome"] == "win"


def test_combined_equity_uses_lead_approx_exit_time_ordering():
    lead = pd.DataFrame(
        [
            {
                "entry_time": pd.Timestamp("2025-06-01T00:00:00Z"),
                "exit_time": pd.Timestamp("2025-06-01T02:00:00Z"),
                "weighted_pnl_pct": -2.0,
            }
        ]
    )
    overlay = pd.DataFrame(
        [
            {
                "entry_time": pd.Timestamp("2025-06-01T00:30:00Z"),
                "exit_time": pd.Timestamp("2025-06-01T01:00:00Z"),
                "pnl_initial_pct": 1.0,
            }
        ]
    )

    summary = audit._combined_equity_summary(lead, overlay)

    assert summary["combined_pnl_pct"] == -1.0
    assert summary["combined_equity_max_dd_pct"] == -2.0
