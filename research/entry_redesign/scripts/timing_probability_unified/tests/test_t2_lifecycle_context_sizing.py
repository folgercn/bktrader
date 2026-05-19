"""Tests for original_t2 lifecycle context sizing runner."""

from __future__ import annotations

import json

import pandas as pd

from timing_probability_unified.t2_lifecycle_context_sizing import (
    T2LifecycleSizingSilo,
    T2LifecycleSizingSpec,
    build_sizing_specs,
    compute_deltas,
    compute_metrics,
    filter_sizing_specs,
    summarize_t2_size_multiplier_attribution,
    write_outputs,
)


def _silo(candidate: str, month: str, ret: float, t2_pnl: float) -> T2LifecycleSizingSilo:
    return T2LifecycleSizingSilo(
        candidate=candidate,
        shape_sizing_filters={},
        fail_multiplier=1.0,
        t3_exit_overrides={},
        sizing_filter_fail_action="scale",
        symbol="ETHUSDT",
        month=month,
        return_pct=ret,
        final_balance=10_000.0,
        total_trades=2,
        t2_trades=1,
        t3_trades=1,
        t2_net_pnl_pct=t2_pnl,
        t3_net_pnl_pct=0.01,
        t2_size_multiplier_attribution={
            "fail_scaled_0.25": {
                "trades": 1,
                "gross_pnl_pct": t2_pnl,
                "fee_pct": 0.01,
                "net_after_fee_pct": t2_pnl - 0.01,
                "notional_pct": 5.0,
                "avg_multiplier": 0.25,
            }
        },
        t2_size_filter_fails=1,
        t2_size_filter_reasons={"ctx_side_return_atr": 1},
        reentry_fill_rejects={"long": {"no_reclaim_cross": 2}},
        elapsed_seconds=0.1,
    )


def test_sizing_specs_are_filterable_by_label():
    specs = build_sizing_specs("compact")
    labels = [spec.label for spec in specs]
    assert labels == [
        "strict_baseline",
        "original_t2_ctx4h_scaled025",
        "original_t2_ctx12h_scaled025",
        "original_t2_ctx4h_scaled025_t3_min_hold_sl_60m",
    ]
    assert filter_sizing_specs(specs, ["original_t2_ctx4h_scaled025"])[0].fail_multiplier == 0.25
    union = filter_sizing_specs(specs, ["original_t2_ctx4h_scaled025_t3_min_hold_sl_60m"])[0]
    assert union.t3_exit_overrides == {"min_hold_seconds_before_sl": 3600.0}


def test_multiplier_sensitivity_specs_cover_fail_size_ladder():
    specs = build_sizing_specs("ctx4h_multiplier_sensitivity")
    assert [spec.fail_multiplier for spec in specs] == [1.0, 0.5, 0.25, 0.1, 0.0, 0.0]
    assert specs[0].label == "original_t2_ctx4h_scaled100_t3_min_hold_sl_60m"
    assert specs[-2].label == "original_t2_ctx4h_scaled000_t3_min_hold_sl_60m"
    assert specs[-1].label == "original_t2_ctx4h_skipfail_t3_min_hold_sl_60m"
    assert specs[-1].sizing_filter_fail_action == "skip_lock"
    assert all(spec.t3_exit_overrides == {"min_hold_seconds_before_sl": 3600.0} for spec in specs)


def test_size_multiplier_attribution_splits_original_t2_trade_buckets():
    ledger = pd.DataFrame(
        [
            {
                "type": "BUY",
                "price": 100.0,
                "notional": 1000.0,
                "breakout_shape_name": "original_t2",
                "size_multiplier": 1.0,
            },
            {"type": "EXIT", "price": 101.0, "breakout_shape_name": "original_t2"},
            {
                "type": "SHORT",
                "price": 100.0,
                "notional": 250.0,
                "breakout_shape_name": "original_t2",
                "size_multiplier": 0.25,
            },
            {"type": "EXIT", "price": 101.0, "breakout_shape_name": "original_t2"},
            {
                "type": "BUY",
                "price": 100.0,
                "notional": 500.0,
                "breakout_shape_name": "t3_swing",
                "size_multiplier": 1.0,
            },
            {"type": "EXIT", "price": 110.0, "breakout_shape_name": "t3_swing"},
        ]
    )

    attribution = summarize_t2_size_multiplier_attribution(ledger, initial_balance=10_000.0)

    assert attribution["pass_full_or_unfiltered"]["trades"] == 1
    assert attribution["pass_full_or_unfiltered"]["gross_pnl_pct"] == 0.1
    assert attribution["pass_full_or_unfiltered"]["fee_pct"] == 0.02
    assert attribution["pass_full_or_unfiltered"]["net_after_fee_pct"] == 0.08
    assert attribution["fail_scaled_0.25"]["trades"] == 1
    assert attribution["fail_scaled_0.25"]["gross_pnl_pct"] == -0.025
    assert "t3_swing" not in attribution


def test_compute_metrics_zero_fills_calendar_and_deltas():
    spec = T2LifecycleSizingSpec("candidate", {}, 0.25)
    metrics = compute_metrics(
        spec,
        [_silo("candidate", "2026-02", 1.0, 0.2)],
        months=["2026-02", "2026-03"],
        symbols=["ETHUSDT"],
    )
    assert metrics.calendar_silo_sum_pct == 1.0
    assert metrics.worst_calendar_silo_pct == 0.0
    assert metrics.flat_symbol_months == 1
    assert metrics.t2_size_filter_reasons == {"ctx_side_return_atr": 1}
    assert metrics.reentry_fill_rejects == {"no_reclaim_cross": 2}
    assert metrics.t2_size_multiplier_attribution["fail_scaled_0.25"]["trades"] == 1

    baseline = compute_metrics(
        T2LifecycleSizingSpec("baseline", {}, 1.0),
        [_silo("baseline", "2026-02", 0.5, 0.1)],
        months=["2026-02"],
        symbols=["ETHUSDT"],
    )
    delta = compute_deltas([baseline, metrics])[0]
    assert delta.calendar_silo_sum_delta_pct == 0.5
    assert delta.t2_net_pnl_delta_pct == 0.1


def test_write_outputs_includes_summary_and_report(tmp_path):
    spec = T2LifecycleSizingSpec("candidate", {"max_atr_percentile": 40.0}, 0.25)
    silos = [_silo("candidate", "2026-02", 1.0, 0.2)]
    metrics = [compute_metrics(spec, silos, ["2026-02"], ["ETHUSDT"])]
    write_outputs(
        specs=[spec],
        silos=silos,
        metrics=metrics,
        output_dir=tmp_path,
        months=["2026-02"],
        symbols=["ETHUSDT"],
        timeframe="1h",
        reentry_fill_policy="strict_next_second_cross",
    )

    payload = json.loads((tmp_path / "t2_lifecycle_context_sizing_summary.json").read_text())
    assert payload["candidates"][0]["candidate"] == "candidate"
    assert "t2_size_multiplier_attribution" in payload["candidates"][0]
    report = (tmp_path / "t2_lifecycle_context_sizing_report.md").read_text()
    assert "T2 Lifecycle Context Sizing" in report
    assert "T2 Size Multiplier Attribution" in report
