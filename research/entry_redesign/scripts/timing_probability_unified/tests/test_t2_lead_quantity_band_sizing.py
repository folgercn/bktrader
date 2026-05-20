"""Tests for T2/lead quantity-band sizing helpers."""

from __future__ import annotations

import pandas as pd
import pytest

from timing_probability_unified import t2_lead_quantity_band_sizing as sizing


def _lead_trades() -> pd.DataFrame:
    return pd.DataFrame(
        [
            {
                "event_id": "event-a",
                "touch_time": "2026-01-01T00:00:00Z",
                "position_size": 1.2,
                "weighted_pnl": 0.012,
            },
            {
                "event_id": "event-b",
                "touch_time": "2026-01-02T00:00:00Z",
                "position_size": 1.6,
                "weighted_pnl": -0.004,
            },
        ]
    )


def test_score_lead_quantity_band_matches_live_shadow_formula():
    scored = sizing.score_lead_quantity_band(
        _lead_trades(),
        base_order_quantity=0.10,
        base_share=0.80,
        max_production_multiplier=2.0,
        min_quantity=0.20,
        max_quantity=0.40,
        max_submitted_quantity=0.40,
        legacy_scale=1.5,
    )

    row_a = scored[scored["event_id"] == "event-a"].iloc[0]
    row_b = scored[scored["event_id"] == "event-b"].iloc[0]
    assert row_a["production_quantity"] == pytest.approx(0.12)
    assert row_a["lead_quantity_band_score"] == pytest.approx(0.75)
    assert row_a["submitted_quantity"] == pytest.approx(0.35)
    assert row_a["quantity_multiplier"] == pytest.approx(0.35 / 0.12)
    assert row_a["quantity_band_weighted_pnl_pct"] == pytest.approx(1.2 * (0.35 / 0.12))
    assert row_b["production_quantity"] == pytest.approx(0.16)
    assert row_b["lead_quantity_band_score"] == pytest.approx(1.0)
    assert row_b["submitted_quantity"] == pytest.approx(0.40)


def test_summarize_lead_reports_base_legacy_and_quantity_band():
    scored = sizing.score_lead_quantity_band(
        _lead_trades(),
        base_order_quantity=0.10,
        base_share=0.80,
        max_production_multiplier=2.0,
        min_quantity=0.20,
        max_quantity=0.40,
        max_submitted_quantity=0.40,
        legacy_scale=1.5,
    )

    metrics, baseline = sizing.summarize_lead(scored, months=["2026-01"])

    assert baseline["base_lead_adverse10_pct"] == pytest.approx(0.8)
    assert baseline["legacy_lead_1p5_adverse10_pct"] == pytest.approx(1.2)
    assert metrics.calendar_sum_pct == pytest.approx(2.5)
    assert metrics.delta_vs_base_pct == pytest.approx(1.7)
    assert metrics.delta_vs_legacy_1p5_pct == pytest.approx(1.3)
    assert metrics.avg_submitted_quantity == pytest.approx(0.375)
    assert metrics.max_submitted_quantity == pytest.approx(0.40)


def test_summarize_bundle_adds_t2_and_t3_monthly_results():
    scored = sizing.score_lead_quantity_band(
        _lead_trades(),
        base_order_quantity=0.10,
        base_share=0.80,
        max_production_multiplier=2.0,
        min_quantity=0.20,
        max_quantity=0.40,
        max_submitted_quantity=0.40,
        legacy_scale=1.5,
    )
    lead_metrics, baseline = sizing.summarize_lead(scored, months=["2026-01", "2026-02"])
    t3_metrics = {
        "calendar_sum_pct": 3.25,
        "by_month": {"2026-01": 2.0, "2026-02": 1.25},
    }
    t3_trades = pd.DataFrame(
        {
            "event_time": pd.to_datetime(["2026-01-03T00:00:00Z"], utc=True),
            "combo_pnl_pct": [2.0],
        }
    )

    bundle = sizing.summarize_bundle(
        lead_scored=scored,
        lead_metrics=lead_metrics,
        base_lead_pct=baseline["base_lead_adverse10_pct"],
        t3_metrics=t3_metrics,
        t3_trades=t3_trades,
        months=["2026-01", "2026-02"],
    )

    assert bundle is not None
    assert bundle.calendar_sum_pct == pytest.approx(5.75)
    assert bundle.delta_vs_base_lead_plus_t3_pct == pytest.approx(1.7)
    assert bundle.by_month["2026-01"] == pytest.approx(4.5)
    assert bundle.by_month["2026-02"] == pytest.approx(1.25)
