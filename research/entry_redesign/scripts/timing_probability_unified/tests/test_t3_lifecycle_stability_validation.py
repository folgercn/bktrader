"""Tests for T3 lifecycle stability validation helpers."""

import json

import pytest

from timing_probability_unified.t3_lifecycle_stability_validation import (
    T3StabilitySiloResult,
    compute_stability_metrics,
    write_outputs,
)


def _silo(
    *,
    symbol: str,
    month: str,
    return_pct: float,
    t3_net_pnl_pct: float,
    t3_trades: int = 2,
    t3_win_rate_pct: float = 50.0,
) -> T3StabilitySiloResult:
    return T3StabilitySiloResult(
        symbol=symbol,
        month=month,
        return_pct=return_pct,
        final_balance=100000.0 + return_pct * 1000.0,
        total_trades=10,
        t2_trades=8,
        t3_trades=t3_trades,
        t2_net_pnl_pct=return_pct - t3_net_pnl_pct,
        t3_net_pnl_pct=t3_net_pnl_pct,
        t3_win_rate_pct=t3_win_rate_pct,
        t3_exit_reasons={"SL": t3_trades},
        reentry_fill_rejects={"long": {"same_second_zero_initial": 2}, "short": {}},
        elapsed_seconds=1.0,
    )


def test_compute_stability_metrics_zero_fills_calendar_grid():
    metrics = compute_stability_metrics(
        [
            _silo(symbol="ETHUSDT", month="2025-12", return_pct=1.0, t3_net_pnl_pct=0.5),
            _silo(
                symbol="BTCUSDT",
                month="2026-01",
                return_pct=-2.0,
                t3_net_pnl_pct=-0.25,
                t3_trades=4,
                t3_win_rate_pct=75.0,
            ),
        ],
        months=["2025-12", "2026-01"],
        symbols=["ETHUSDT", "BTCUSDT"],
    )

    assert metrics.calendar_silo_sum_pct == pytest.approx(-1.0)
    assert metrics.calendar_avg_symbol_month_pct == pytest.approx(-0.25)
    assert metrics.worst_calendar_silo_pct == pytest.approx(-2.0)
    assert metrics.negative_calendar_silos == 1
    assert metrics.traded_symbol_months == 2
    assert metrics.flat_symbol_months == 2
    assert metrics.t3_net_pnl_pct == pytest.approx(0.25)
    assert metrics.t3_win_rate_pct == pytest.approx((50.0 * 2 + 75.0 * 4) / 6)
    assert metrics.positive_t3_silos == 1
    assert metrics.negative_t3_silos == 1
    assert metrics.flat_t3_silos == 2
    assert metrics.worst_t3_silo_pct == pytest.approx(-0.25)
    assert metrics.by_year == {"2025": 1.0, "2026": -2.0}
    assert metrics.t3_by_symbol == {"ETHUSDT": 0.5, "BTCUSDT": -0.25}
    assert metrics.t3_exit_reasons == {"SL": 6}
    assert metrics.reentry_fill_rejects == {"same_second_zero_initial": 4}


def test_write_outputs_emits_json_markdown_and_csv(tmp_path):
    silos = [
        _silo(symbol="ETHUSDT", month="2026-02", return_pct=3.0, t3_net_pnl_pct=0.7),
    ]
    metrics = compute_stability_metrics(
        silos,
        months=["2026-02"],
        symbols=["ETHUSDT"],
    )

    write_outputs(
        silos=silos,
        metrics=metrics,
        output_dir=tmp_path,
        months=["2026-02"],
        symbols=["ETHUSDT"],
        timeframe="1h",
        pre_touch_max=900.0,
        reentry_fill_policy="strict_next_second_cross",
    )

    summary = json.loads((tmp_path / "t3_lifecycle_stability_summary.json").read_text())
    report = (tmp_path / "t3_lifecycle_stability_report.md").read_text()

    assert summary["metrics"]["calendar_silo_sum_pct"] == pytest.approx(3.0)
    assert summary["reentry_fill_policy"] == "strict_next_second_cross"
    assert summary["calendar_grid"]["symbol_months"] == 1
    assert "T3 Lifecycle Stability Validation" in report
    assert "same_second_zero_initial" in report
    assert "ETHUSDT" in report
    assert (tmp_path / "t3_lifecycle_stability_silos.csv").exists()
