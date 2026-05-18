"""Tests for T3 pre-touch fixed-calendar holdout helpers."""

from types import SimpleNamespace

import pandas as pd
import pytest

from timing_probability_unified.t3_pre_touch_holdout import (
    CalendarCandidateMetrics,
    _active_trades,
    _fixed_calendar_silos,
    _pool_month_totals,
    compute_calendar_candidate_metrics,
    compute_deltas,
)


def _make_trades() -> pd.DataFrame:
    return pd.DataFrame(
        {
            "symbol": ["ETHUSDT", "ETHUSDT", "ETHUSDT", "ETHUSDT"],
            "pool": ["T2", "T3", "T3", "T2"],
            "touch_time": pd.to_datetime(
                [
                    "2026-02-01 00:00:00",
                    "2026-02-02 00:00:00",
                    "2026-03-03 00:00:00",
                    "2026-03-04 00:00:00",
                ],
                utc=True,
            ),
            "timing_prediction": ["fast", "slow", "skip", "fast"],
            "speed_gate_pass": [True, True, True, False],
            "weighted_pnl": [0.10, -0.03, 0.20, 0.05],
        }
    )


def test_active_trades_requires_timing_and_speed_gate():
    active = _active_trades(_make_trades())

    assert active["weighted_pnl"].tolist() == pytest.approx([0.10, -0.03])


def test_fixed_calendar_silos_zero_fills_missing_months():
    silos = _fixed_calendar_silos(
        _make_trades(),
        months=["2026-02", "2026-03"],
        symbols=["ETHUSDT"],
    )

    assert silos == [
        {
            "month": "2026-02",
            "symbol": "ETHUSDT",
            "calendar_pnl": 0.07,
            "trades": 2,
            "flat": False,
        },
        {
            "month": "2026-03",
            "symbol": "ETHUSDT",
            "calendar_pnl": 0.0,
            "trades": 0,
            "flat": True,
        },
    ]


def test_pool_month_totals_use_active_rows_only():
    totals = _pool_month_totals(_make_trades(), months=["2026-02", "2026-03"])

    assert totals == {
        "2026-02": {"T2": 0.1, "T3": -0.03, "union": 0.07},
        "2026-03": {"T2": 0.0, "T3": 0.0, "union": 0.0},
    }


def test_compute_calendar_candidate_metrics_counts_flat_silos():
    rolling = SimpleNamespace(
        combined_forward_trades=_make_trades(),
        total_trade_count=2,
    )

    metrics = compute_calendar_candidate_metrics(
        label="pre<=900",
        t3_pre_touch_max=900.0,
        rolling=rolling,
        months=["2026-02", "2026-03"],
        symbols=["ETHUSDT"],
    )

    assert metrics.calendar_silo_sum == pytest.approx(0.07)
    assert metrics.calendar_avg_symbol_month == pytest.approx(0.035)
    assert metrics.worst_calendar_silo == pytest.approx(0.0)
    assert metrics.negative_calendar_silos == 0
    assert metrics.traded_symbol_months == 1
    assert metrics.flat_symbol_months == 1
    assert metrics.t2_calendar_sum == pytest.approx(0.1)
    assert metrics.t3_calendar_sum == pytest.approx(-0.03)


def test_compute_deltas_compare_to_first_candidate():
    baseline = CalendarCandidateMetrics(
        label="pre<=900",
        t3_pre_touch_max=900.0,
        calendar_silo_sum=0.10,
        calendar_avg_symbol_month=0.05,
        worst_calendar_silo=-0.02,
        negative_calendar_silos=1,
        traded_symbol_months=2,
        flat_symbol_months=0,
        union_trade_count=10,
        t2_calendar_sum=0.12,
        t3_calendar_sum=-0.02,
        month_totals={},
        pool_month_totals={},
        silos=[],
    )
    candidate = CalendarCandidateMetrics(
        label="pre<=600",
        t3_pre_touch_max=600.0,
        calendar_silo_sum=0.15,
        calendar_avg_symbol_month=0.075,
        worst_calendar_silo=-0.01,
        negative_calendar_silos=1,
        traded_symbol_months=2,
        flat_symbol_months=0,
        union_trade_count=12,
        t2_calendar_sum=0.12,
        t3_calendar_sum=0.03,
        month_totals={},
        pool_month_totals={},
        silos=[],
    )

    deltas = compute_deltas([baseline, candidate])

    assert len(deltas) == 1
    assert deltas[0].calendar_silo_sum_delta == pytest.approx(0.05)
    assert deltas[0].t3_calendar_sum_delta == pytest.approx(0.05)
    assert deltas[0].worst_calendar_silo_delta == pytest.approx(0.01)
    assert deltas[0].trade_count_delta == 2
    assert deltas[0].negative_calendar_silos_delta == 0
