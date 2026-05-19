"""Tests for the T2/T3 bridge report."""

from __future__ import annotations

import json

import pandas as pd

from timing_probability_unified import t2_t3_bridge_runner as bridge


def _trades(path, rows):
    pd.DataFrame(rows).to_csv(path, index=False)


def test_bridge_runner_normalizes_t2_decimal_to_percent_and_uses_t3_snapshot(tmp_path, monkeypatch):
    lead = tmp_path / "lead.csv"
    extra = tmp_path / "extra.csv"
    wf3_full = tmp_path / "wf3_full.csv"
    wf3_hard = tmp_path / "wf3_hard.csv"
    missing_t3 = tmp_path / "missing_t3.json"
    scaled_extra = tmp_path / "scaled_extra.csv"
    scaled_combo = tmp_path / "scaled_combo.csv"

    _trades(
        lead,
        [
            {"touch_time": "2025-06-01T00:00:00Z", "weighted_pnl": 0.01},
            {"touch_time": "2025-07-01T00:00:00Z", "weighted_pnl": 0.02},
        ],
    )
    _trades(extra, [{"touch_time": "2025-07-02T00:00:00Z", "weighted_pnl": 0.005}])
    _trades(
        wf3_full,
        [
            {
                "touch_time": "2025-07-03T00:00:00Z",
                "weighted_pnl": 0.004,
                "event_key": "pass",
                "sizing_multiplier": 1.0,
                "position_size": 0.8,
            },
            {
                "touch_time": "2025-07-04T00:00:00Z",
                "weighted_pnl": 0.008,
                "event_key": "fail",
                "sizing_multiplier": 1.0,
                "position_size": 0.8,
            },
        ],
    )
    _trades(
        wf3_hard,
        [
            {
                "touch_time": "2025-07-03T00:00:00Z",
                "weighted_pnl": 0.004,
                "event_key": "pass",
                "sizing_multiplier": 1.0,
                "position_size": 0.8,
            }
        ],
    )
    monkeypatch.setattr(bridge, "LEAD_ADVERSE_TRADES", lead)
    monkeypatch.setattr(bridge, "LOW_EFF_RF_MEDIAN_EXTRA_TRADES", extra)
    monkeypatch.setattr(bridge, "WF3_LOW_EFF_EXTRA_TRADES", wf3_full)
    monkeypatch.setattr(bridge, "WF3_CTX4H_UP_EXTRA_TRADES", wf3_hard)
    monkeypatch.setattr(bridge, "T3_EXIT_SUMMARY", missing_t3)
    monkeypatch.setattr(bridge, "SCALED_CTX4H_EXTRA_TRADES", scaled_extra)
    monkeypatch.setattr(bridge, "SCALED_CTX4H_COMBO_TRADES", scaled_combo)

    out = tmp_path / "out"
    bridge.run(out, ["2025-06", "2025-07"])

    payload = json.loads((out / "t2_t3_bridge_summary.json").read_text())
    candidates = {row["candidate"]: row for row in payload["candidates"]}
    assert candidates["canonical_lead"]["calendar_sum_pct"] == 3.0
    assert candidates["low_eff_rf_rank_median_000_combo"]["calendar_sum_pct"] == 3.5
    assert candidates["wf3_low_eff_low_atr_ctx4h_scaled025_combo"]["calendar_sum_pct"] == 3.6
    assert candidates["t3_min_hold_sl_60m_t3_split"]["calendar_sum_pct"] == 0.62176
    scaled = pd.read_csv(scaled_extra)
    fail_row = scaled[scaled["event_key"] == "fail"].iloc[0]
    assert fail_row["weighted_pnl"] == 0.002
    assert payload["caveats"]["t2_lead_trade_ledger_active_months"] == ["2025-06", "2025-07"]
    assert "Data Caveat" in (out / "t2_t3_bridge_report.md").read_text()
