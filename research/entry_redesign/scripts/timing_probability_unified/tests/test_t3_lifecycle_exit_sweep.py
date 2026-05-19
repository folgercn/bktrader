"""Tests for T3 lifecycle exit sweep helpers."""

from __future__ import annotations

import pytest

import eth_q1_breakout_t3_shape_compare as lifecycle
from timing_probability_unified.t3_lifecycle_exit_sweep import (
    T3ExitSiloResult,
    T3ExitSpec,
    build_exit_specs,
    compute_exit_deltas,
    compute_exit_metrics,
    filter_exit_specs,
)


def _make_silo(
    *,
    candidate: str,
    symbol: str,
    month: str,
    return_pct: float,
    t3_net_pnl_pct: float,
    t3_trades: int,
    t3_exit_reasons: dict | None = None,
) -> T3ExitSiloResult:
    return T3ExitSiloResult(
        candidate=candidate,
        t3_exit_overrides={},
        symbol=symbol,
        month=month,
        return_pct=return_pct,
        final_balance=100000.0 + return_pct * 1000.0,
        total_trades=20,
        t2_trades=20 - t3_trades,
        t3_trades=t3_trades,
        t2_net_pnl_pct=return_pct - t3_net_pnl_pct,
        t3_net_pnl_pct=t3_net_pnl_pct,
        t3_win_rate_pct=50.0,
        t3_exit_reasons=t3_exit_reasons or {"SL": t3_trades},
        elapsed_seconds=0.1,
    )


def test_t3_exit_override_only_applies_to_t3():
    assert lifecycle._t3_exit_override("original_t2", {"max_hold_seconds": 3600.0}, "max_hold_seconds", None) is None
    assert lifecycle._t3_exit_override("t3_swing", {"max_hold_seconds": 3600.0}, "max_hold_seconds", None) == 3600.0
    assert lifecycle._t3_exit_override("t3_swing", {}, "max_hold_seconds", None) is None


def test_build_exit_specs_compact_starts_with_baseline():
    specs = build_exit_specs("compact")

    assert specs[0].label == "baseline"
    assert specs[0].t3_exit_overrides == {}
    assert {spec.label for spec in specs} >= {
        "t3_timecap_30m",
        "t3_timecap_60m",
        "t3_min_hold_sl_5m",
        "t3_min_hold_sl_10m",
        "t3_min_hold_sl_30m",
        "t3_min_hold_sl_45m",
        "t3_min_hold_sl_60m",
        "t3_trail_fast_0p3",
        "t3_stop_0p04",
    }


def test_filter_exit_specs_keeps_requested_order():
    specs = build_exit_specs("compact")
    filtered = filter_exit_specs(specs, ["t3_trail_fast_0p3", "baseline"])

    assert [spec.label for spec in filtered] == ["t3_trail_fast_0p3", "baseline"]


def test_filter_exit_specs_rejects_unknown_label():
    specs = build_exit_specs("compact")

    with pytest.raises(ValueError, match="missing"):
        filter_exit_specs(specs, ["missing"])


def test_compute_exit_metrics_zero_fills_grid_and_merges_reasons():
    spec = T3ExitSpec("baseline", {})
    metrics = compute_exit_metrics(
        spec,
        [
            _make_silo(
                candidate="baseline",
                symbol="ETHUSDT",
                month="2026-02",
                return_pct=3.0,
                t3_net_pnl_pct=1.2,
                t3_trades=4,
                t3_exit_reasons={"SL": 3, "T3TimeCap": 1},
            )
        ],
        months=["2026-02", "2026-03"],
        symbols=["ETHUSDT"],
    )

    assert metrics.calendar_silo_sum_pct == pytest.approx(3.0)
    assert metrics.calendar_avg_symbol_month_pct == pytest.approx(1.5)
    assert metrics.worst_calendar_silo_pct == pytest.approx(0.0)
    assert metrics.traded_symbol_months == 1
    assert metrics.flat_symbol_months == 1
    assert metrics.t3_trades == 4
    assert metrics.t3_net_pnl_pct == pytest.approx(1.2)
    assert metrics.t3_exit_reasons == {"SL": 3, "T3TimeCap": 1}
    assert metrics.by_month == {"2026-02": 3.0, "2026-03": 0.0}


def test_compute_exit_deltas_compare_to_first_candidate():
    baseline = compute_exit_metrics(
        T3ExitSpec("baseline", {}),
        [
            _make_silo(
                candidate="baseline",
                symbol="ETHUSDT",
                month="2026-02",
                return_pct=2.0,
                t3_net_pnl_pct=0.5,
                t3_trades=3,
            )
        ],
        months=["2026-02"],
        symbols=["ETHUSDT"],
    )
    candidate = compute_exit_metrics(
        T3ExitSpec("t3_timecap_60m", {"max_hold_seconds": 3600.0}),
        [
            _make_silo(
                candidate="t3_timecap_60m",
                symbol="ETHUSDT",
                month="2026-02",
                return_pct=2.5,
                t3_net_pnl_pct=0.8,
                t3_trades=4,
            )
        ],
        months=["2026-02"],
        symbols=["ETHUSDT"],
    )

    deltas = compute_exit_deltas([baseline, candidate])

    assert len(deltas) == 1
    assert deltas[0].calendar_silo_sum_delta_pct == pytest.approx(0.5)
    assert deltas[0].trade_count_delta == 0
    assert deltas[0].t3_trade_delta == 1
    assert deltas[0].t3_net_pnl_delta_pct == pytest.approx(0.3)
