"""Tests for exact lead exposure-window helpers."""

from __future__ import annotations

import pandas as pd
import pytest

from pre_breakout_timing.delay_simulator import DelayResult
from timing_probability_unified import t3_overlay_lead_exact_exposure as exact


def _delay(
    label: str,
    pnl: float,
    *,
    entry_time: str,
    exit_time: str,
    traded: bool = True,
) -> DelayResult:
    return DelayResult(
        event_id="evt-1",
        delay_label=label,
        delay_seconds=5 if label == "D5" else 0,
        entry_time=pd.Timestamp(entry_time),
        entry_price=100.0 if traded else None,
        pnl_pct=pnl if traded else None,
        exit_reason="TrailingSL" if traded else "NoData",
        exit_time=pd.Timestamp(exit_time) if traded else None,
        hold_seconds=120.0 if traded else None,
        mfe_r=1.2 if traded else None,
        mae_r=-0.3 if traded else None,
        traded=traded,
    )


def test_attach_selected_delay_metadata_carries_exact_times_and_pct():
    trades = pd.DataFrame(
        [
            {
                "event_id": "evt-1",
                "symbol": "ETHUSDT",
                "side": "long",
                "touch_time": pd.Timestamp("2025-06-01T00:00:00Z"),
                "timing_prediction": "fast",
                "selected_delay": "D5",
                "position_size": 0.8,
                "delay_pnl_pct": 0.012,
                "weighted_pnl": 0.0096,
                "speed_gate_pass": True,
            }
        ]
    )
    delays = [
        [
            _delay(
                "D0",
                0.001,
                entry_time="2025-06-01T00:00:00Z",
                exit_time="2025-06-01T00:01:00Z",
            ),
            _delay(
                "D5",
                0.012,
                entry_time="2025-06-01T00:00:06Z",
                exit_time="2025-06-01T00:02:06Z",
            ),
        ]
    ]

    windows = exact.attach_selected_delay_metadata(trades, delays, scenario_name="next_adverse_xslip10bps")

    assert windows.loc[0, "entry_time"] == pd.Timestamp("2025-06-01T00:00:06Z")
    assert windows.loc[0, "exit_time"] == pd.Timestamp("2025-06-01T00:02:06Z")
    assert windows.loc[0, "notional_share"] == 0.8
    assert windows.loc[0, "weighted_pnl_pct"] == pytest.approx(0.96)
    assert windows.loc[0, "window_source"] == "lead_exact_adverse10"
    assert bool(windows.loc[0, "delay_traded"]) is True


def test_attach_selected_delay_metadata_rejects_label_drift():
    trades = pd.DataFrame(
        [
            {
                "event_id": "evt-1",
                "symbol": "ETHUSDT",
                "side": "long",
                "touch_time": pd.Timestamp("2025-06-01T00:00:00Z"),
                "timing_prediction": "fast",
                "selected_delay": "D0",
                "position_size": 0.8,
                "delay_pnl_pct": 0.012,
                "weighted_pnl": 0.0096,
                "speed_gate_pass": True,
            }
        ]
    )
    delays = [
        [
            _delay(
                "D0",
                0.001,
                entry_time="2025-06-01T00:00:00Z",
                exit_time="2025-06-01T00:01:00Z",
            ),
            _delay(
                "D5",
                0.012,
                entry_time="2025-06-01T00:00:05Z",
                exit_time="2025-06-01T00:02:05Z",
            ),
        ]
    ]

    with pytest.raises(ValueError, match="selected delay mismatch"):
        exact.attach_selected_delay_metadata(trades, delays, scenario_name="next_adverse_xslip10bps")


def test_compare_to_reference_detects_parity(tmp_path):
    reference = pd.DataFrame(
        [
            {
                "event_id": "evt-1",
                "selected_delay": "D5",
                "weighted_pnl": 0.0096,
                "position_size": 0.8,
                "delay_pnl_pct": 0.012,
            }
        ]
    )
    reference_path = tmp_path / "reference.csv"
    reference.to_csv(reference_path, index=False)
    exact_rows = reference.assign(entry_time=pd.Timestamp("2025-06-01T00:00:06Z"))

    parity = exact.compare_to_reference(exact_rows, reference_path)

    assert parity.reference_rows == 1
    assert parity.exact_rows == 1
    assert parity.missing_exact_events == 0
    assert parity.extra_exact_events == 0
    assert parity.selected_delay_mismatches == 0
    assert parity.max_abs_weighted_pnl_diff == 0.0
    assert parity.max_abs_position_size_diff == 0.0
    assert parity.max_abs_delay_pnl_diff == 0.0
