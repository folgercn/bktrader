"""Tests for sizing readiness gate."""

from __future__ import annotations

from timing_probability_unified import t3_overlay_sizing_readiness_gate as readiness


def _live_summary(sample_entries: int = 6, combined_pass: int = 6, headroom: float = 2.8) -> dict:
    return {
        "matrix": [
            {
                "quantity_scale": 1.25,
                "sample_entries": sample_entries,
                "combined_pass_entries": combined_pass,
                "min_scaled_top_depth_coverage": 10.0,
                "max_adverse_fill_drift_bps": 5.2,
                "worst_slippage_headroom_bps": headroom,
            }
        ]
    }


def _conditional_summary(
    strict15: float = 25.5,
    strict20: float = 22.0,
    severe15: float = 20.0,
) -> dict:
    return {
        "scenarios": [
            {
                "profile": "strict_top1p2_active1p0",
                "lead_impact_gate_round_trip_bps": 10.0,
                "overlay_extra_round_trip_slippage_bps": 15.0,
                "calendar_sum_pct": strict15,
            },
            {
                "profile": "strict_top1p2_active1p0",
                "lead_impact_gate_round_trip_bps": 10.0,
                "overlay_extra_round_trip_slippage_bps": 20.0,
                "calendar_sum_pct": strict20,
            },
            {
                "profile": "severe_top1p0_active2p0",
                "lead_impact_gate_round_trip_bps": 10.0,
                "overlay_extra_round_trip_slippage_bps": 15.0,
                "calendar_sum_pct": severe15,
            },
        ]
    }


def test_readiness_continues_research_when_live_sample_is_small():
    result = readiness.evaluate_readiness(_live_summary(), _conditional_summary())

    assert result.status == "research_continue_collect_live_depth"
    assert result.live_gate_pass is True
    assert result.proxy_gate_pass is True
    assert result.sample_gate_pass is False


def test_readiness_ready_when_all_gates_pass_and_sample_is_large():
    result = readiness.evaluate_readiness(
        _live_summary(sample_entries=30, combined_pass=30),
        _conditional_summary(),
    )

    assert result.status == "live_candidate_ready_for_human_review"
    assert result.sample_gate_pass is True


def test_readiness_blocks_when_proxy_no_longer_separates_stress():
    result = readiness.evaluate_readiness(
        _live_summary(sample_entries=30, combined_pass=30),
        _conditional_summary(strict20=24.0),
    )

    assert result.status == "blocked"
    assert result.proxy_gate_pass is False
    assert result.strict_20bp_kill_fails_baseline is False


def test_readiness_blocks_when_live_headroom_is_too_thin():
    result = readiness.evaluate_readiness(
        _live_summary(sample_entries=30, combined_pass=30, headroom=1.0),
        _conditional_summary(),
    )

    assert result.status == "blocked"
    assert result.live_gate_pass is False
