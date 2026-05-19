"""Tests for T3 pre-touch lifecycle comparison helpers."""

import pytest

import eth_q1_breakout_t3_shape_compare as lifecycle
from timing_probability_unified.t3_pre_touch_lifecycle import (
    LifecycleSiloResult,
    compute_candidate_metrics,
    compute_deltas,
)


def _make_silo(
    *,
    candidate: str,
    symbol: str,
    month: str,
    return_pct: float,
    trades: int,
    t3_trades: int,
    t3_net_pnl_pct: float,
    t3_rejects: int = 0,
) -> LifecycleSiloResult:
    return LifecycleSiloResult(
        candidate=candidate,
        t3_pre_touch_max=600.0 if candidate == "pre<=600" else 900.0,
        symbol=symbol,
        month=month,
        return_pct=return_pct,
        final_balance=100000.0 + return_pct * 1000.0,
        trades=trades,
        win_rate_pct=50.0,
        max_dd_pct=-1.0,
        t2_trades=trades - t3_trades,
        t3_trades=t3_trades,
        t3_net_pnl_pct=t3_net_pnl_pct,
        t2_net_pnl_pct=return_pct - t3_net_pnl_pct,
        t3_rejects=t3_rejects,
        breakout_locks={},
        entry_reasons={},
        exit_reasons={},
        elapsed_seconds=0.1,
    )


def test_t3_quality_filter_rejects_late_pre_touch():
    reason = lifecycle._t3_quality_reject_reason(
        {},
        "long",
        current_price=101.0,
        breakout_level=100.0,
        filters={"max_pre_touch_seconds": 600.0},
        pre_touch_seconds=601.0,
    )

    assert reason == "pre_touch_seconds"


def test_t3_quality_filter_allows_early_pre_touch():
    reason = lifecycle._t3_quality_reject_reason(
        {},
        "short",
        current_price=99.0,
        breakout_level=100.0,
        filters={"max_pre_touch_seconds": 600.0},
        pre_touch_seconds=600.0,
    )

    assert reason == ""


def test_t3_quality_filter_rejects_disallowed_side():
    reason = lifecycle._t3_quality_reject_reason(
        {},
        "long",
        current_price=101.0,
        breakout_level=100.0,
        filters={"allowed_sides": ["short"]},
        pre_touch_seconds=100.0,
    )

    assert reason == "side"


def test_compute_candidate_metrics_zero_fills_grid():
    metrics = compute_candidate_metrics(
        "pre<=600",
        600.0,
        [
            _make_silo(
                candidate="pre<=600",
                symbol="ETHUSDT",
                month="2026-02",
                return_pct=4.0,
                trades=10,
                t3_trades=2,
                t3_net_pnl_pct=1.5,
                t3_rejects=3,
            )
        ],
        months=["2026-02", "2026-03"],
        symbols=["ETHUSDT"],
    )

    assert metrics.calendar_silo_sum_pct == pytest.approx(4.0)
    assert metrics.calendar_avg_symbol_month_pct == pytest.approx(2.0)
    assert metrics.worst_calendar_silo_pct == pytest.approx(0.0)
    assert metrics.traded_symbol_months == 1
    assert metrics.flat_symbol_months == 1
    assert metrics.total_trades == 10
    assert metrics.t3_trades == 2
    assert metrics.t3_net_pnl_pct == pytest.approx(1.5)
    assert metrics.t3_rejects == 3
    assert metrics.by_month == {"2026-02": 4.0, "2026-03": 0.0}


def test_compute_deltas_compare_to_first_candidate():
    baseline = compute_candidate_metrics(
        "pre<=900",
        900.0,
        [
            _make_silo(
                candidate="pre<=900",
                symbol="ETHUSDT",
                month="2026-02",
                return_pct=1.0,
                trades=12,
                t3_trades=4,
                t3_net_pnl_pct=-0.5,
                t3_rejects=1,
            )
        ],
        months=["2026-02"],
        symbols=["ETHUSDT"],
    )
    candidate = compute_candidate_metrics(
        "pre<=600",
        600.0,
        [
            _make_silo(
                candidate="pre<=600",
                symbol="ETHUSDT",
                month="2026-02",
                return_pct=2.0,
                trades=10,
                t3_trades=2,
                t3_net_pnl_pct=0.25,
                t3_rejects=5,
            )
        ],
        months=["2026-02"],
        symbols=["ETHUSDT"],
    )

    deltas = compute_deltas([baseline, candidate])

    assert len(deltas) == 1
    assert deltas[0].calendar_silo_sum_delta_pct == pytest.approx(1.0)
    assert deltas[0].trade_count_delta == -2
    assert deltas[0].t3_trade_delta == -2
    assert deltas[0].t3_net_pnl_delta_pct == pytest.approx(0.75)
    assert deltas[0].t3_reject_delta == 4
