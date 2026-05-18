"""Tests for T3 lifecycle quality-gate sweep helpers."""

import pytest

from timing_probability_unified.t3_lifecycle_gate_sweep import (
    LifecycleGateSiloResult,
    LifecycleGateSpec,
    build_gate_specs,
    compute_gate_deltas,
    compute_gate_metrics,
    filter_gate_specs,
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
) -> LifecycleGateSiloResult:
    return LifecycleGateSiloResult(
        candidate=candidate,
        filters={"max_pre_touch_seconds": 900.0},
        symbol=symbol,
        month=month,
        return_pct=return_pct,
        final_balance=100000.0 + return_pct * 1000.0,
        trades=trades,
        win_rate_pct=50.0,
        max_dd_pct=-1.0,
        t2_trades=trades - t3_trades,
        t3_trades=t3_trades,
        t2_net_pnl_pct=return_pct - t3_net_pnl_pct,
        t3_net_pnl_pct=t3_net_pnl_pct,
        t3_rejects=t3_rejects,
        t3_reject_reasons={},
        breakout_locks={},
        entry_reasons={},
        exit_reasons={},
        elapsed_seconds=0.1,
    )


def test_build_gate_specs_compact_uses_pre900_baseline_first():
    specs = build_gate_specs("compact")

    assert specs[0].label == "pre900"
    assert specs[0].filters == {"max_pre_touch_seconds": 900.0}
    assert {spec.label for spec in specs} >= {
        "pre600",
        "pre900_sep0p25",
        "pre900_ext0p75",
        "pre900_long_only",
        "pre900_short_only",
    }


def test_filter_gate_specs_keeps_requested_order():
    specs = build_gate_specs("compact")
    filtered = filter_gate_specs(specs, ["pre600_short_only", "pre900"])

    assert [spec.label for spec in filtered] == ["pre600_short_only", "pre900"]


def test_filter_gate_specs_rejects_unknown_label():
    specs = build_gate_specs("compact")

    with pytest.raises(ValueError, match="missing"):
        filter_gate_specs(specs, ["missing"])


def test_compute_gate_metrics_zero_fills_grid():
    spec = LifecycleGateSpec("pre900", {"max_pre_touch_seconds": 900.0})
    metrics = compute_gate_metrics(
        spec,
        [
            _make_silo(
                candidate="pre900",
                symbol="ETHUSDT",
                month="2026-02",
                return_pct=3.0,
                trades=20,
                t3_trades=4,
                t3_net_pnl_pct=1.25,
                t3_rejects=2,
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
    assert metrics.total_trades == 20
    assert metrics.t3_trades == 4
    assert metrics.t3_net_pnl_pct == pytest.approx(1.25)
    assert metrics.t3_rejects == 2
    assert metrics.by_month == {"2026-02": 3.0, "2026-03": 0.0}


def test_compute_gate_deltas_compare_to_first_candidate():
    baseline = compute_gate_metrics(
        LifecycleGateSpec("pre900", {"max_pre_touch_seconds": 900.0}),
        [
            _make_silo(
                candidate="pre900",
                symbol="ETHUSDT",
                month="2026-02",
                return_pct=2.0,
                trades=10,
                t3_trades=3,
                t3_net_pnl_pct=0.5,
                t3_rejects=1,
            )
        ],
        months=["2026-02"],
        symbols=["ETHUSDT"],
    )
    candidate = compute_gate_metrics(
        LifecycleGateSpec(
            "pre900_sep0p25",
            {"max_pre_touch_seconds": 900.0, "min_sma_atr_separation": 0.25},
        ),
        [
            _make_silo(
                candidate="pre900_sep0p25",
                symbol="ETHUSDT",
                month="2026-02",
                return_pct=2.5,
                trades=8,
                t3_trades=2,
                t3_net_pnl_pct=0.9,
                t3_rejects=4,
            )
        ],
        months=["2026-02"],
        symbols=["ETHUSDT"],
    )

    deltas = compute_gate_deltas([baseline, candidate])

    assert len(deltas) == 1
    assert deltas[0].calendar_silo_sum_delta_pct == pytest.approx(0.5)
    assert deltas[0].trade_count_delta == -2
    assert deltas[0].t3_trade_delta == -1
    assert deltas[0].t3_net_pnl_delta_pct == pytest.approx(0.4)
    assert deltas[0].t3_reject_delta == 3
