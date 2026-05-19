"""Tests for T3 overlay lead bridge accounting."""

from __future__ import annotations

import json

import pandas as pd

from timing_probability_unified import t3_overlay_lead_bridge as bridge


def test_bridge_converts_t3_percent_to_fractional_return(tmp_path):
    lead_adverse = tmp_path / "lead_adverse.csv"
    lead_same = tmp_path / "lead_same.csv"
    t3_summary = tmp_path / "t3_summary.json"
    out = tmp_path / "out"

    pd.DataFrame(
        [
            {"touch_time": "2025-06-01T00:00:00Z", "weighted_pnl": 0.10},
            {"touch_time": "2025-07-01T00:00:00Z", "weighted_pnl": -0.02},
        ]
    ).to_csv(lead_adverse, index=False)
    pd.DataFrame(
        [
            {"touch_time": "2025-06-01T00:00:00Z", "weighted_pnl": 0.12},
            {"touch_time": "2025-07-01T00:00:00Z", "weighted_pnl": -0.01},
        ]
    ).to_csv(lead_same, index=False)
    t3_summary.write_text(
        json.dumps(
            {
                "external_entry_mode": "next_second_adverse",
                "t3_size_scale": 2.0,
                "t3_reentry_size_schedule": [0.4, 0.2],
                "calendar_grid": {"months": ["2025-06", "2025-07"]},
                "candidates": [
                    {
                        "total_trades": 3,
                        "by_month": {"2025-06": 5.0, "2025-07": -1.0},
                    }
                ],
            }
        ),
        encoding="utf-8",
    )

    rows = bridge.run(
        lead_adverse_trades=lead_adverse,
        lead_same_trades=lead_same,
        t3_summary_path=t3_summary,
        output_dir=out,
    )

    by_variant = {row["variant"]: row for row in rows}
    assert by_variant["t3_overlay_eth_adverse_size2"]["calendar_sum"] == 0.04
    assert by_variant["lead_adverse10_plus_t3_overlay"]["calendar_sum"] == 0.12
    assert by_variant["lead_adverse10_plus_t3_overlay"]["trade_count"] == 5
    monthly = pd.read_csv(out / "t3_overlay_lead_bridge_monthly.csv")
    assert monthly.loc[monthly["month"] == "2025-06", "t3_overlay"].iloc[0] == 0.05
