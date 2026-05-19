"""Tests for live depth calibration helpers."""

from __future__ import annotations

from dataclasses import asdict, replace

import pandas as pd

from timing_probability_unified import t3_overlay_live_depth_calibration as calibration


def _orders() -> list[dict]:
    return [
        {
            "id": "order-entry-buy",
            "strategyVersionId": "strategy-version-bk-eth-pretouch-timing-v010",
            "symbol": "ETHUSDT",
            "side": "BUY",
            "quantity": 0.1,
            "price": 100.05,
            "reduceOnly": False,
            "createdAt": "2026-05-19T00:00:00Z",
            "metadata": {
                "signalKind": "entry",
                "adapterSubmission": {"rawPriceReference": 100.0},
                "executionProposal": {
                    "spreadBps": 2.0,
                    "metadata": {
                        "entrySubmissionSlippageGuard": {
                            "bookAgeMs": 100.0,
                            "maxBookAgeMs": 500.0,
                            "maxSlippageBps": 8.0,
                            "maxSourceDivergenceBps": 8.0,
                            "minTopDepthCoverage": 0.5,
                            "sourceDivergenceBps": 0.25,
                            "status": "passed",
                            "topDepthCoverage": 2.0,
                            "topDepthQty": 0.2,
                        },
                        "executionContext": {"strategyEngineKey": "bk-live-eth-pretouch-timing"},
                        "executionDecisionContext": {"maxSpreadBps": 8.0},
                    },
                    "signalKind": "entry",
                },
            },
        },
        {
            "id": "order-entry-sell",
            "strategyVersionId": "strategy-version-bk-eth-pretouch-timing-v010",
            "symbol": "ETHUSDT",
            "side": "SELL",
            "quantity": 0.1,
            "price": 99.90,
            "reduceOnly": False,
            "createdAt": "2026-05-19T01:00:00Z",
            "metadata": {
                "signalKind": "entry",
                "adapterSubmission": {"rawPriceReference": 100.0},
                "executionProposal": {
                    "spreadBps": 9.0,
                    "metadata": {
                        "entrySubmissionSlippageGuard": {
                            "bookAgeMs": 100.0,
                            "maxBookAgeMs": 500.0,
                            "maxSlippageBps": 8.0,
                            "maxSourceDivergenceBps": 8.0,
                            "minTopDepthCoverage": 0.5,
                            "sourceDivergenceBps": 0.0,
                            "status": "passed",
                            "topDepthCoverage": 1.0,
                            "topDepthQty": 0.1,
                        },
                        "executionContext": {"strategyEngineKey": "bk-live-eth-pretouch-timing"},
                        "executionDecisionContext": {"maxSpreadBps": 8.0},
                    },
                    "signalKind": "entry",
                },
            },
        },
        {
            "id": "order-exit",
            "strategyVersionId": "strategy-version-bk-eth-pretouch-timing-v010",
            "symbol": "ETHUSDT",
            "side": "SELL",
            "quantity": 0.1,
            "price": 99.0,
            "reduceOnly": True,
            "createdAt": "2026-05-19T02:00:00Z",
            "metadata": {"signalKind": "risk-exit"},
        },
    ]


def test_adverse_fill_drift_bps_uses_side():
    assert round(calibration.adverse_fill_drift_bps("BUY", 100.05, 100.0), 6) == 5.0
    assert calibration.adverse_fill_drift_bps("BUY", 99.95, 100.0) == 0.0
    assert round(calibration.adverse_fill_drift_bps("SELL", 99.95, 100.0), 6) == 5.0
    assert calibration.adverse_fill_drift_bps("SELL", 100.05, 100.0) == 0.0


def test_extract_entry_telemetry_filters_exits_and_derives_fields():
    entries = calibration.extract_entry_telemetry(_orders())

    assert [entry.order_id for entry in entries] == ["order-entry-buy", "order-entry-sell"]
    assert entries[0].adverse_fill_drift_bps == 5.0
    assert entries[1].adverse_fill_drift_bps == 10.0
    assert entries[0].top_depth_coverage == 2.0
    assert entries[0].max_spread_bps == 8.0


def test_calibrate_scales_counts_pre_submit_and_fill_passes():
    entries = calibration.extract_entry_telemetry(_orders())
    matrix = calibration.calibrate_scales(entries, [1.0, 2.5])

    assert matrix[0].quantity_scale == 1.0
    assert matrix[0].sample_entries == 2
    assert matrix[0].pre_submit_pass_entries == 1
    assert matrix[0].fill_adverse_pass_entries == 1
    assert matrix[0].combined_pass_entries == 1
    assert matrix[0].min_scaled_top_depth_coverage == 1.0

    assert matrix[1].quantity_scale == 2.5
    assert matrix[1].pre_submit_pass_entries == 1
    assert matrix[1].min_scaled_top_depth_coverage == 0.4


def test_merge_entry_history_dedupes_by_order_id_and_prefers_current():
    entries = calibration.extract_entry_telemetry(_orders())
    stale_history = [replace(entries[0], spread_bps=99.0)]

    merged = calibration.merge_entry_history(entries, stale_history)

    assert len(merged) == 2
    assert [entry.order_id for entry in merged] == ["order-entry-buy", "order-entry-sell"]
    assert merged[0].spread_bps == 2.0


def test_run_merges_prior_sanitized_history(tmp_path):
    current_entries = calibration.extract_entry_telemetry(_orders())
    old_entry = replace(current_entries[0], order_id="old-order", created_at="2026-05-18T00:00:00Z")
    history_csv = tmp_path / "history.csv"
    pd.DataFrame([asdict(current_entries[0]), asdict(old_entry)]).to_csv(history_csv, index=False)

    result = calibration.run(
        payload=_orders(),
        output_dir=tmp_path / "out",
        scales=[1.0],
        symbol="ETHUSDT",
        strategy_version_id="strategy-version-bk-eth-pretouch-timing-v010",
        strategy_engine_key="bk-live-eth-pretouch-timing",
        source_note="test",
        history_csv=history_csv,
    )

    assert result["current_entry_count"] == 2
    assert result["history_entry_count"] == 2
    assert result["deduped_entry_count"] == 3
    assert result["matrix"][0]["sample_entries"] == 3
