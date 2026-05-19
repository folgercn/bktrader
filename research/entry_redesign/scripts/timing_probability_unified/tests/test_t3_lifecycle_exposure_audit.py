"""Tests for T3 lifecycle exposure audit helpers."""

from __future__ import annotations

import pandas as pd
import pytest

from timing_probability_unified.t3_lifecycle_exposure_audit import (
    _equity_max_drawdown_pct,
    _max_loss_streak,
    summarize_t3_exposure,
)


def _trade(
    *,
    shape: str = "t3_swing",
    pnl_initial_pct: float,
    exit_reason: str = "SL",
    exit_time: str,
    pnl_bps: float | None = None,
    hold_seconds: float = 120.0,
    mae_bps: float = -10.0,
    mfe_bps: float = 20.0,
) -> dict:
    return {
        "breakout_shape_name": shape,
        "pnl_initial_pct": pnl_initial_pct,
        "exit_reason": exit_reason,
        "exit_time": exit_time,
        "hold_seconds": hold_seconds,
        "mae_bps": mae_bps,
        "mfe_bps": mfe_bps,
        "pnl_bps": pnl_initial_pct * 100.0 if pnl_bps is None else pnl_bps,
    }


def test_equity_max_drawdown_includes_initial_zero_peak():
    trades = pd.DataFrame(
        [
            _trade(pnl_initial_pct=-0.4, exit_time="2026-02-01T00:00:00Z"),
            _trade(pnl_initial_pct=0.1, exit_time="2026-02-01T00:01:00Z"),
            _trade(pnl_initial_pct=-0.2, exit_time="2026-02-01T00:02:00Z"),
        ]
    )

    assert _equity_max_drawdown_pct(trades) == pytest.approx(-0.5)


def test_equity_max_drawdown_tracks_peak_to_trough_after_wins():
    trades = pd.DataFrame(
        [
            _trade(pnl_initial_pct=0.8, exit_time="2026-02-01T00:00:00Z"),
            _trade(pnl_initial_pct=-0.3, exit_time="2026-02-01T00:01:00Z"),
            _trade(pnl_initial_pct=-0.6, exit_time="2026-02-01T00:02:00Z"),
            _trade(pnl_initial_pct=0.2, exit_time="2026-02-01T00:03:00Z"),
        ]
    )

    assert _equity_max_drawdown_pct(trades) == pytest.approx(-0.9)


def test_max_loss_streak_treats_breakeven_as_non_win():
    trades = pd.DataFrame(
        [
            _trade(pnl_initial_pct=0.2, exit_time="2026-02-01T00:00:00Z"),
            _trade(pnl_initial_pct=-0.1, exit_time="2026-02-01T00:01:00Z"),
            _trade(pnl_initial_pct=0.0, exit_time="2026-02-01T00:02:00Z"),
            _trade(pnl_initial_pct=-0.3, exit_time="2026-02-01T00:03:00Z"),
            _trade(pnl_initial_pct=0.4, exit_time="2026-02-01T00:04:00Z"),
        ]
    )

    assert _max_loss_streak(trades) == 3


def test_summarize_t3_exposure_excludes_non_t3_and_final_mark():
    trades = pd.DataFrame(
        [
            _trade(
                pnl_initial_pct=0.4,
                exit_reason="SL",
                exit_time="2026-02-01T00:02:00Z",
                pnl_bps=20.0,
                hold_seconds=120.0,
                mae_bps=-7.5,
                mfe_bps=25.0,
            ),
            _trade(
                pnl_initial_pct=0.3,
                exit_reason="FinalMarkToMarket",
                exit_time="2026-02-01T00:03:00Z",
                pnl_bps=15.0,
                hold_seconds=3600.0,
                mae_bps=-15.0,
                mfe_bps=40.0,
            ),
            _trade(
                shape="original_t2",
                pnl_initial_pct=-9.0,
                exit_reason="SL",
                exit_time="2026-02-01T00:04:00Z",
            ),
        ]
    )

    summary = summarize_t3_exposure(
        candidate="t3_min_hold_sl_60m",
        t3_exit_overrides={"min_hold_seconds_before_sl": 3600.0},
        scope="aggregate",
        calendar_returns=[1.2, -0.4],
        total_trades=10,
        trades=trades,
    )

    assert summary.calendar_silo_sum_pct == pytest.approx(0.8)
    assert summary.worst_calendar_silo_pct == pytest.approx(-0.4)
    assert summary.negative_calendar_silos == 1
    assert summary.total_trades == 10
    assert summary.t3_trades == 2
    assert summary.t3_net_pnl_pct == pytest.approx(0.7)
    assert summary.t3_net_pnl_ex_final_mark_pct == pytest.approx(0.4)
    assert summary.final_mark_trades == 1
    assert summary.final_mark_pnl_pct == pytest.approx(0.3)
    assert summary.t3_exit_reasons == {"FinalMarkToMarket": 1, "SL": 1}
    assert summary.t3_win_rate_pct == pytest.approx(100.0)
    assert summary.t3_equity_max_dd_pct == pytest.approx(0.0)
    assert summary.t3_max_loss_streak == 0
    assert summary.t3_avg_hold_seconds == pytest.approx(1860.0)
    assert summary.t3_p90_hold_seconds == pytest.approx(3252.0)
    assert summary.t3_sum_hold_hours == pytest.approx(1.033333)
    assert summary.t3_worst_mae_bps == pytest.approx(-15.0)
    assert summary.t3_worst_pnl_bps == pytest.approx(15.0)
