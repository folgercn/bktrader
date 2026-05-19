"""Tests for T2 lifecycle pass-bucket gate runner."""

from __future__ import annotations

import pandas as pd

from timing_probability_unified.t2_lifecycle_pass_bucket_gates import (
    _pair_t2_trades,
    build_gate_candidates,
    summarize_reference_buckets,
)


def test_gate_candidates_keep_skipfail_t3_contract():
    candidates = build_gate_candidates("smoke")

    assert [candidate.label for candidate in candidates] == [
        "ctx4h_skipfail_t3_60m",
        "ctx4h_min020_skipfail_t3_60m",
        "ctx12h_min000_skipfail_t3_60m",
    ]
    assert candidates[0].filters == {
        "max_atr_percentile": 40.0,
        "min_ctx_side_return_atr": 0.0,
        "ctx_return_lookback_bars": 4,
    }
    assert candidates[-1].filters["min_ctx12h_side_return_atr"] == 0.0


def test_pair_t2_trades_preserves_entry_context_metadata():
    ledger = pd.DataFrame(
        [
            {
                "time": pd.Timestamp("2026-01-01T00:00:02Z"),
                "type": "BUY",
                "price": 100.0,
                "reason": "Zero-Initial-Reentry",
                "notional": 2000.0,
                "breakout_shape_name": "original_t2",
                "size_multiplier": 1.0,
                "side": "long",
                "signal_start": pd.Timestamp("2026-01-01T00:00:00Z"),
                "atr_percentile": 25.0,
                "ctx4h_side_return_atr": 0.25,
                "ctx12h_side_return_atr": -0.10,
                "breakout_pre_touch_seconds": 1.0,
                "breakout_extension_atr": 0.04,
            },
            {
                "time": pd.Timestamp("2026-01-01T00:01:00Z"),
                "type": "EXIT",
                "price": 101.0,
                "reason": "PT",
                "notional": 2000.0,
                "breakout_shape_name": "original_t2",
            },
        ]
    )

    rows = _pair_t2_trades(
        ledger,
        candidate="candidate",
        symbol="ETHUSDT",
        month="2026-01",
        initial_balance=10_000.0,
    )

    assert len(rows) == 1
    assert rows[0]["net_after_fee_pct"] == 0.16
    assert rows[0]["side"] == "long"
    assert rows[0]["signal_start"] == "2026-01-01T00:00:00+00:00"
    assert rows[0]["atr_percentile"] == 25.0


def test_reference_bucket_summary_splits_pass_full_features():
    trades = pd.DataFrame(
        [
            {
                "size_multiplier": 1.0,
                "net_after_fee_pct": -0.20,
                "gross_pnl_pct": -0.10,
                "fee_pct": 0.10,
                "notional_pct": 20.0,
                "side": "long",
                "exit_reason": "SL",
                "entry_reason": "Zero-Initial-Reentry",
                "atr_percentile": 25.0,
                "ctx4h_side_return_atr": 0.15,
                "ctx12h_side_return_atr": -0.10,
                "breakout_pre_touch_seconds": 120.0,
                "breakout_extension_atr": 0.04,
            },
            {
                "size_multiplier": 0.25,
                "net_after_fee_pct": 0.05,
                "gross_pnl_pct": 0.06,
                "fee_pct": 0.01,
                "notional_pct": 5.0,
                "side": "short",
                "exit_reason": "PT",
                "entry_reason": "Zero-Initial-Reentry",
                "atr_percentile": 35.0,
                "ctx4h_side_return_atr": 0.05,
                "ctx12h_side_return_atr": 0.20,
                "breakout_pre_touch_seconds": 700.0,
                "breakout_extension_atr": 0.12,
            },
        ]
    )

    summary = summarize_reference_buckets(trades)

    all_row = summary[(summary["family"] == "all_original_t2") & (summary["bucket"] == "all")].iloc[0]
    pass_row = summary[
        (summary["family"] == "pass_full_original_t2") & (summary["bucket"] == "all")
    ].iloc[0]
    ctx12_bad = summary[
        (summary["family"] == "ctx12h_side_return_atr") & (summary["bucket"] == "-0.50-0")
    ].iloc[0]
    assert all_row["trades"] == 2
    assert all_row["net_after_fee_pct"] == -0.15
    assert pass_row["trades"] == 1
    assert ctx12_bad["net_after_fee_pct"] == -0.20
