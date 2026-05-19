"""Tests for T3 lifecycle outcome diagnostics helpers."""

import pandas as pd
import pytest

from timing_probability_unified.t3_lifecycle_outcome_diagnostics import (
    compute_diagnostic_overlays,
    pair_lifecycle_trades,
    summarize_slices,
)


def _bars() -> pd.DataFrame:
    idx = pd.date_range("2026-02-01 00:00:00Z", periods=8, freq="1min")
    return pd.DataFrame(
        {
            "high": [100.5, 101.0, 102.0, 101.5, 101.0, 100.5, 100.2, 100.1],
            "low": [99.8, 99.5, 99.0, 99.2, 99.4, 99.7, 99.9, 100.0],
        },
        index=idx,
    )


def test_pair_lifecycle_trades_enriches_t3_long_excursion():
    ledger = pd.DataFrame(
        [
            {
                "time": pd.Timestamp("2026-02-01 00:00:00Z"),
                "type": "BUY",
                "price": 100.0,
                "reason": "Zero-Initial-Reentry",
                "notional": 20000.0,
                "breakout_shape_name": "t3_swing",
            },
            {
                "time": pd.Timestamp("2026-02-01 00:03:00Z"),
                "type": "EXIT",
                "price": 101.0,
                "reason": "PT",
                "notional": 20000.0,
                "breakout_shape_name": "t3_swing",
            },
        ]
    )

    trades = pair_lifecycle_trades(
        ledger,
        _bars(),
        symbol="ETHUSDT",
        month="2026-02",
        initial_balance=100000.0,
        early_horizon_seconds=300,
    )

    assert len(trades) == 1
    row = trades.iloc[0]
    assert row["side"] == "long"
    assert row["symbol_month"] == "ETHUSDT:2026-02"
    assert row["symbol_side"] == "ETHUSDT:long"
    assert row["side_entry_reason"] == "long:Zero-Initial-Reentry"
    assert row["symbol_month_side"] == "ETHUSDT:2026-02:long"
    assert row["symbol_month_entry_reason"] == "ETHUSDT:2026-02:Zero-Initial-Reentry"
    assert (
        row["symbol_month_side_entry_reason"]
        == "ETHUSDT:2026-02:long:Zero-Initial-Reentry"
    )
    assert row["outcome"] == "win"
    assert row["pnl_bps"] == pytest.approx(100.0)
    assert row["pnl_initial_pct"] == pytest.approx(0.2)
    assert row["hold_bucket"] == "0-5m"
    assert row["mfe_300s_bps"] == pytest.approx(200.0)
    assert row["mae_300s_bps"] == pytest.approx(-100.0)
    assert row["mae_300s_bucket"] == "gt20bps"


def test_pair_lifecycle_trades_handles_t3_short_loss():
    ledger = pd.DataFrame(
        [
            {
                "time": pd.Timestamp("2026-02-01 00:00:00Z"),
                "type": "SHORT",
                "price": 100.0,
                "reason": "SL-Reentry",
                "notional": 10000.0,
                "breakout_shape_name": "t3_swing",
            },
            {
                "time": pd.Timestamp("2026-02-01 00:02:00Z"),
                "type": "EXIT",
                "price": 101.0,
                "reason": "SL",
                "notional": 10000.0,
                "breakout_shape_name": "t3_swing",
            },
        ]
    )

    trades = pair_lifecycle_trades(
        ledger,
        _bars(),
        symbol="BTCUSDT",
        month="2026-02",
        initial_balance=100000.0,
        early_horizon_seconds=300,
    )

    row = trades.iloc[0]
    assert row["side"] == "short"
    assert row["pnl_bps"] == pytest.approx(-100.0)
    assert row["pnl_initial_pct"] == pytest.approx(-0.1)
    assert row["exit_reason"] == "SL"
    assert row["mfe_300s_bps"] == pytest.approx(100.0)
    assert row["mae_300s_bps"] == pytest.approx(-200.0)


def test_summarize_slices_uses_t3_only_rows():
    trades = pd.DataFrame(
        [
            {
                "breakout_shape_name": "t3_swing",
                "side": "long",
                "entry_reason": "Zero-Initial-Reentry",
                "exit_reason": "PT",
                "pnl_initial_pct": 0.2,
                "pnl_bps": 100.0,
                "hold_seconds": 100.0,
                "mae_300s_bps": -5.0,
                "mfe_300s_bps": 50.0,
            },
            {
                "breakout_shape_name": "original_t2",
                "side": "short",
                "entry_reason": "Zero-Initial-Reentry",
                "exit_reason": "SL",
                "pnl_initial_pct": -9.0,
                "pnl_bps": -900.0,
                "hold_seconds": 100.0,
                "mae_300s_bps": -50.0,
                "mfe_300s_bps": 10.0,
            },
        ]
    )

    slices = summarize_slices(trades, ["side", "exit_reason"])
    all_slice = next(row for row in slices if row.group_by == "all")

    assert all_slice.trades == 1
    assert all_slice.net_pnl_pct == pytest.approx(0.2)
    assert all_slice.win_rate_pct == pytest.approx(100.0)


def test_compute_diagnostic_overlays_scores_entry_reason_drops():
    trades = pd.DataFrame(
        [
            {
                "breakout_shape_name": "t3_swing",
                "entry_reason": "Zero-Initial-Reentry",
                "side": "long",
                "pnl_initial_pct": 0.3,
            },
            {
                "breakout_shape_name": "t3_swing",
                "entry_reason": "SL-Reentry",
                "side": "short",
                "pnl_initial_pct": -0.2,
            },
        ]
    )

    overlays = {row.label: row for row in compute_diagnostic_overlays(trades)}

    assert overlays["keep_all"].kept_t3_net_pnl_pct == pytest.approx(0.1)
    assert overlays["drop_sl_reentry"].kept_t3_net_pnl_pct == pytest.approx(0.3)
    assert overlays["drop_sl_reentry"].t3_net_pnl_delta_pct == pytest.approx(0.2)
    assert overlays["zero_initial_only"].kept_t3_trades == 1
