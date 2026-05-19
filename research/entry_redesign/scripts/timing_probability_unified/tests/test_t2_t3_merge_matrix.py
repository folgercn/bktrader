"""Tests for the T2/T3 research merge matrix."""

from __future__ import annotations

import json

import pandas as pd

from timing_probability_unified.t2_t3_merge_matrix import run


def test_merge_matrix_uses_kiro_snapshot_when_t3_json_missing(tmp_path):
    lead_combo = tmp_path / "lead_combo.csv"
    pd.DataFrame(
        [
            {
                "variant": "low_eff_rf_rank_median_000",
                "base_gate": "low_eff_low_atr_q20_q40",
                "model_variant": "rf_rank_median_000",
                "combo_adverse10_calendar_sum": 0.28,
                "combo_adverse10_delta_vs_lead": 0.05,
                "combo_adverse10_worst_sm": -0.004,
                "combo_adverse10_neg_sm": 2,
                "combo_adverse10_trade_count": 89,
            }
        ]
    ).to_csv(lead_combo, index=False)

    lead_retrain = tmp_path / "lead_retrain.csv"
    pd.DataFrame(
        [
            {
                "pool": "canonical_only",
                "feature_set": "production8",
                "forward_adverse10_calendar_sum": 0.30,
                "forward_adverse10_worst_sm": 0.002,
                "forward_adverse10_neg_sm": 0,
                "forward_trade_count": 78,
            },
            {
                "pool": "combo_wf3_low_eff_low_atr_ctx4h_scaled025",
                "feature_set": "production8",
                "forward_adverse10_calendar_sum": 0.37,
                "forward_adverse10_worst_sm": 0.006,
                "forward_adverse10_neg_sm": 0,
                "forward_trade_count": 115,
            },
        ]
    ).to_csv(lead_retrain, index=False)

    output_dir = tmp_path / "out"
    run(
        output_dir=output_dir,
        lead_combo_summary=lead_combo,
        lead_retrain_summary=lead_retrain,
        t3_exit_summary=tmp_path / "missing_exit.json",
        t3_exposure_summary=tmp_path / "missing_exposure.json",
        t3_baseline_summary=tmp_path / "missing_baseline.json",
        t2_lifecycle_context_summary=tmp_path / "missing_t2_lifecycle.json",
        t2_t3_union_lifecycle_summary=tmp_path / "missing_union_lifecycle.json",
        t2_ctx4h_multiplier_sensitivity_summary=tmp_path / "missing_multiplier_sensitivity.json",
        t2_ctx4h_skipfail_summary=tmp_path / "missing_skipfail.json",
    )

    payload = json.loads((output_dir / "t2_t3_merge_matrix_summary.json").read_text())
    candidates = {row["candidate"] for row in payload["metric_rows"]}
    assert "t3_min_hold_sl_60m" in candidates
    readiness = {row["candidate"]: row for row in payload["contract_readiness"]}
    assert readiness["wf3_low_eff_low_atr_ctx4h_scaled025_combo"]["strict_lifecycle_ready"] is False
    assert any("source=.kiro/specs/t2-t3-union-strategy/tasks.md" in note for note in payload["t3_exposure_notes"])
    report = (output_dir / "t2_t3_merge_matrix_report.md").read_text()
    assert "Do not merge historical T3 lifecycle headline returns" in report
    assert "Lifecycle Contract Readiness" in report


def test_merge_matrix_combines_fresh_t3_candidate_with_snapshot_baseline(tmp_path):
    lead_combo = tmp_path / "lead_combo.csv"
    pd.DataFrame(
        [
            {
                "variant": "low_eff_rf_rank_median_000",
                "combo_adverse10_calendar_sum": 0.28,
                "combo_adverse10_delta_vs_lead": 0.05,
                "combo_adverse10_worst_sm": -0.004,
                "combo_adverse10_neg_sm": 2,
                "combo_adverse10_trade_count": 89,
            }
        ]
    ).to_csv(lead_combo, index=False)

    lead_retrain = tmp_path / "lead_retrain.csv"
    pd.DataFrame(
        [
            {
                "pool": "canonical_only",
                "feature_set": "production8",
                "forward_adverse10_calendar_sum": 0.30,
                "forward_adverse10_worst_sm": 0.002,
                "forward_adverse10_neg_sm": 0,
                "forward_trade_count": 78,
            }
        ]
    ).to_csv(lead_retrain, index=False)

    fresh_t3 = tmp_path / "fresh_exit.json"
    fresh_t3.write_text(
        json.dumps(
            {
                "reentry_fill_policy": "strict_next_second_cross",
                "calendar_grid": {
                    "months": ["2025-06"],
                    "symbols": ["ETHUSDT", "BTCUSDT"],
                },
                "candidates": [
                    {
                        "candidate": "t3_min_hold_sl_60m",
                        "calendar_silo_sum_pct": -24.48,
                        "worst_calendar_silo_pct": -2.15,
                        "negative_calendar_silos": 22,
                        "total_trades": 610,
                        "t3_trades": 100,
                        "t3_net_pnl_pct": 3.845840,
                        "t3_win_rate_pct": 47.00,
                    }
                ],
            }
        ),
        encoding="utf-8",
    )

    output_dir = tmp_path / "out"
    run(
        output_dir=output_dir,
        lead_combo_summary=lead_combo,
        lead_retrain_summary=lead_retrain,
        t3_exit_summary=fresh_t3,
        t3_exposure_summary=tmp_path / "missing_exposure.json",
        t3_baseline_summary=tmp_path / "missing_baseline.json",
        t2_lifecycle_context_summary=tmp_path / "missing_t2_lifecycle.json",
        t2_t3_union_lifecycle_summary=tmp_path / "missing_union_lifecycle.json",
        t2_ctx4h_multiplier_sensitivity_summary=tmp_path / "missing_multiplier_sensitivity.json",
        t2_ctx4h_skipfail_summary=tmp_path / "missing_skipfail.json",
    )

    payload = json.loads((output_dir / "t2_t3_merge_matrix_summary.json").read_text())
    rows = {row["candidate"]: row for row in payload["metric_rows"]}
    assert rows["strict_baseline"]["primary_value"] == -30.98
    assert rows["t3_min_hold_sl_60m"]["delta_vs_baseline"] == 6.5
    assert str(fresh_t3) in rows["t3_min_hold_sl_60m"]["promotion_read"]


def test_merge_matrix_uses_fresh_strict_baseline_when_available(tmp_path):
    lead_combo = tmp_path / "lead_combo.csv"
    pd.DataFrame(
        [
            {
                "variant": "low_eff_rf_rank_median_000",
                "combo_adverse10_calendar_sum": 0.28,
                "combo_adverse10_delta_vs_lead": 0.05,
                "combo_adverse10_worst_sm": -0.004,
                "combo_adverse10_neg_sm": 2,
                "combo_adverse10_trade_count": 89,
            }
        ]
    ).to_csv(lead_combo, index=False)

    lead_retrain = tmp_path / "lead_retrain.csv"
    pd.DataFrame(
        [
            {
                "pool": "canonical_only",
                "feature_set": "production8",
                "forward_adverse10_calendar_sum": 0.30,
                "forward_adverse10_worst_sm": 0.002,
                "forward_adverse10_neg_sm": 0,
                "forward_trade_count": 78,
            }
        ]
    ).to_csv(lead_retrain, index=False)

    fresh_t3 = tmp_path / "fresh_exit.json"
    fresh_t3.write_text(
        json.dumps(
            {
                "reentry_fill_policy": "strict_next_second_cross",
                "calendar_grid": {
                    "months": ["2025-06"],
                    "symbols": ["ETHUSDT", "BTCUSDT"],
                },
                "candidates": [
                    {
                        "candidate": "t3_min_hold_sl_60m",
                        "calendar_silo_sum_pct": -24.48,
                        "worst_calendar_silo_pct": -2.15,
                        "negative_calendar_silos": 22,
                        "total_trades": 610,
                        "t3_trades": 100,
                        "t3_net_pnl_pct": 3.845840,
                        "t3_win_rate_pct": 47.00,
                    }
                ],
            }
        ),
        encoding="utf-8",
    )
    fresh_baseline = tmp_path / "fresh_baseline.json"
    fresh_baseline.write_text(
        json.dumps(
            {
                "metrics": {
                    "calendar_silo_sum_pct": -30.98,
                    "worst_calendar_silo_pct": -2.19,
                    "negative_calendar_silos": 22,
                    "total_trades": 659,
                    "t3_trades": 143,
                    "t3_net_pnl_pct": -1.600810,
                    "t3_win_rate_pct": 14.685035,
                }
            }
        ),
        encoding="utf-8",
    )

    output_dir = tmp_path / "out"
    run(
        output_dir=output_dir,
        lead_combo_summary=lead_combo,
        lead_retrain_summary=lead_retrain,
        t3_exit_summary=fresh_t3,
        t3_exposure_summary=tmp_path / "missing_exposure.json",
        t3_baseline_summary=fresh_baseline,
        t2_lifecycle_context_summary=tmp_path / "missing_t2_lifecycle.json",
        t2_t3_union_lifecycle_summary=tmp_path / "missing_union_lifecycle.json",
        t2_ctx4h_multiplier_sensitivity_summary=tmp_path / "missing_multiplier_sensitivity.json",
        t2_ctx4h_skipfail_summary=tmp_path / "missing_skipfail.json",
    )

    payload = json.loads((output_dir / "t2_t3_merge_matrix_summary.json").read_text())
    rows = {row["candidate"]: row for row in payload["metric_rows"]}
    assert str(fresh_baseline) in rows["strict_baseline"]["promotion_read"]
    assert rows["t3_min_hold_sl_60m"]["delta_vs_baseline"] == 6.5


def test_merge_matrix_includes_t2_lifecycle_context_result(tmp_path):
    lead_combo = tmp_path / "lead_combo.csv"
    pd.DataFrame(
        [
            {
                "variant": "low_eff_rf_rank_median_000",
                "combo_adverse10_calendar_sum": 0.28,
                "combo_adverse10_delta_vs_lead": 0.05,
                "combo_adverse10_worst_sm": -0.004,
                "combo_adverse10_neg_sm": 2,
                "combo_adverse10_trade_count": 89,
            }
        ]
    ).to_csv(lead_combo, index=False)

    lead_retrain = tmp_path / "lead_retrain.csv"
    pd.DataFrame(
        [
            {
                "pool": "canonical_only",
                "feature_set": "production8",
                "forward_adverse10_calendar_sum": 0.30,
                "forward_adverse10_worst_sm": 0.002,
                "forward_adverse10_neg_sm": 0,
                "forward_trade_count": 78,
            }
        ]
    ).to_csv(lead_retrain, index=False)

    t2_lifecycle = tmp_path / "t2_lifecycle.json"
    t2_lifecycle.write_text(
        json.dumps(
            {
                "reentry_fill_policy": "strict_next_second_cross",
                "calendar_grid": {
                    "months": ["2025-06", "2025-07"],
                    "symbols": ["ETHUSDT", "BTCUSDT"],
                },
                "candidates": [
                    {
                        "candidate": "strict_baseline",
                        "calendar_silo_sum_pct": -3.0,
                        "worst_calendar_silo_pct": -1.0,
                        "negative_calendar_silos": 4,
                        "total_trades": 40,
                    },
                    {
                        "candidate": "original_t2_ctx4h_scaled025",
                        "calendar_silo_sum_pct": -2.4,
                        "worst_calendar_silo_pct": -0.8,
                        "negative_calendar_silos": 3,
                        "total_trades": 40,
                        "t2_net_pnl_pct": 0.6,
                        "t2_size_filter_fails": 12,
                    },
                ],
            }
        ),
        encoding="utf-8",
    )

    output_dir = tmp_path / "out"
    run(
        output_dir=output_dir,
        lead_combo_summary=lead_combo,
        lead_retrain_summary=lead_retrain,
        t3_exit_summary=tmp_path / "missing_exit.json",
        t3_exposure_summary=tmp_path / "missing_exposure.json",
        t3_baseline_summary=tmp_path / "missing_baseline.json",
        t2_lifecycle_context_summary=t2_lifecycle,
        t2_t3_union_lifecycle_summary=tmp_path / "missing_union_lifecycle.json",
        t2_ctx4h_multiplier_sensitivity_summary=tmp_path / "missing_multiplier_sensitivity.json",
        t2_ctx4h_skipfail_summary=tmp_path / "missing_skipfail.json",
    )

    payload = json.loads((output_dir / "t2_t3_merge_matrix_summary.json").read_text())
    rows = {row["candidate"]: row for row in payload["metric_rows"]}
    assert rows["original_t2_ctx4h_scaled025"]["family"] == "t2_strict_lifecycle_context_sizing"
    assert rows["original_t2_ctx4h_scaled025"]["delta_vs_baseline"] == 0.6
    readiness = {row["candidate"]: row for row in payload["contract_readiness"]}
    assert readiness["wf3_low_eff_low_atr_ctx4h_scaled025_combo"]["strict_lifecycle_ready"] is True


def test_merge_matrix_includes_union_lifecycle_result_without_inline_baseline(tmp_path):
    lead_combo = tmp_path / "lead_combo.csv"
    pd.DataFrame(
        [
            {
                "variant": "low_eff_rf_rank_median_000",
                "combo_adverse10_calendar_sum": 0.28,
                "combo_adverse10_delta_vs_lead": 0.05,
                "combo_adverse10_worst_sm": -0.004,
                "combo_adverse10_neg_sm": 2,
                "combo_adverse10_trade_count": 89,
            }
        ]
    ).to_csv(lead_combo, index=False)

    lead_retrain = tmp_path / "lead_retrain.csv"
    pd.DataFrame(
        [
            {
                "pool": "canonical_only",
                "feature_set": "production8",
                "forward_adverse10_calendar_sum": 0.30,
                "forward_adverse10_worst_sm": 0.002,
                "forward_adverse10_neg_sm": 0,
                "forward_trade_count": 78,
            }
        ]
    ).to_csv(lead_retrain, index=False)

    union_lifecycle = tmp_path / "union_lifecycle.json"
    union_lifecycle.write_text(
        json.dumps(
            {
                "reentry_fill_policy": "strict_next_second_cross",
                "calendar_grid": {
                    "months": ["2025-06"],
                    "symbols": ["ETHUSDT"],
                },
                "candidates": [
                    {
                        "candidate": "original_t2_ctx4h_scaled025_t3_min_hold_sl_60m",
                        "calendar_silo_sum_pct": -1.2,
                        "worst_calendar_silo_pct": -0.4,
                        "negative_calendar_silos": 1,
                        "total_trades": 10,
                        "t2_net_pnl_pct": -0.3,
                        "t3_net_pnl_pct": 0.2,
                        "t3_exit_overrides": {"min_hold_seconds_before_sl": 3600.0},
                        "t2_size_filter_fails": 5,
                    }
                ],
            }
        ),
        encoding="utf-8",
    )

    output_dir = tmp_path / "out"
    run(
        output_dir=output_dir,
        lead_combo_summary=lead_combo,
        lead_retrain_summary=lead_retrain,
        t3_exit_summary=tmp_path / "missing_exit.json",
        t3_exposure_summary=tmp_path / "missing_exposure.json",
        t3_baseline_summary=tmp_path / "missing_baseline.json",
        t2_lifecycle_context_summary=tmp_path / "missing_t2_lifecycle.json",
        t2_t3_union_lifecycle_summary=union_lifecycle,
        t2_ctx4h_multiplier_sensitivity_summary=tmp_path / "missing_multiplier_sensitivity.json",
        t2_ctx4h_skipfail_summary=tmp_path / "missing_skipfail.json",
    )

    payload = json.loads((output_dir / "t2_t3_merge_matrix_summary.json").read_text())
    rows = {row["candidate"]: row for row in payload["metric_rows"]}
    union = rows["original_t2_ctx4h_scaled025_t3_min_hold_sl_60m"]
    assert union["family"] == "t2_t3_strict_lifecycle_union"
    assert union["delta_vs_baseline"] is None
    assert "T3 overrides" in union["promotion_read"]
