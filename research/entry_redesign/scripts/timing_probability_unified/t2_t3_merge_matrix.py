"""Research-only merge matrix for current pretouch lead and T2/T3 union line.

The current ETH pretouch breakout-expansion line and the Kiro T2/T3 union line
use different evaluation contracts. This tool intentionally does not add their
returns together. It reads the existing research summaries and emits a compact
compatibility report that says which parts can be merged now, which parts need a
bridge replay, and which promotion blockers remain.
"""

from __future__ import annotations

import argparse
import json
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Any

import pandas as pd

PROJECT_ROOT = Path(__file__).resolve().parents[4]
OUTPUT_DIR = PROJECT_ROOT / "research" / "entry_redesign" / "scripts" / "output" / "timing_probability_unified"

LEAD_COMBO_SUMMARY = OUTPUT_DIR / "breakout_structure_context_model_lead_combo_summary.csv"
LEAD_RETRAIN_SUMMARY = OUTPUT_DIR / "breakout_structure_model_retrain_summary.csv"
DEFAULT_T3_EXIT_SUMMARY = OUTPUT_DIR / "t2_t3_merge_t3_exit_60m_extended" / "t3_lifecycle_exit_sweep_summary.json"
DEFAULT_T3_BASELINE_SUMMARY = (
    OUTPUT_DIR / "t2_t3_merge_t3_strict_baseline_extended" / "t3_lifecycle_stability_summary.json"
)
DEFAULT_T3_EXPOSURE_SUMMARY = (
    OUTPUT_DIR / "t2_t3_merge_t3_exposure_60m_extended" / "t3_lifecycle_exposure_audit_summary.json"
)
DEFAULT_T2_LIFECYCLE_CONTEXT_SUMMARY = (
    OUTPUT_DIR
    / "t2_lifecycle_context_sizing_extended"
    / "t2_lifecycle_context_sizing_summary.json"
)
DEFAULT_T2_T3_UNION_LIFECYCLE_SUMMARY = (
    OUTPUT_DIR
    / "t2_t3_lifecycle_union_combined"
    / "t2_lifecycle_context_sizing_summary.json"
)
DEFAULT_T2_CTX4H_MULTIPLIER_SENSITIVITY_SUMMARY = (
    OUTPUT_DIR
    / "t2_ctx4h_multiplier_sensitivity_extended"
    / "t2_lifecycle_context_sizing_summary.json"
)
DEFAULT_T2_CTX4H_SKIPFAIL_SUMMARY = (
    OUTPUT_DIR
    / "t2_ctx4h_skipfail_extended"
    / "t2_lifecycle_context_sizing_summary.json"
)
DEFAULT_T2_PASS_BUCKET_REFERENCE_SUMMARY = (
    OUTPUT_DIR
    / "t2_pass_bucket_gate_reference_extended"
    / "t2_lifecycle_pass_bucket_gate_summary.json"
)
DEFAULT_T2_PASS_BUCKET_PRETOUCH900_SUMMARY = (
    OUTPUT_DIR
    / "t2_pass_bucket_gate_pretouch900_extended"
    / "t2_lifecycle_pass_bucket_gate_summary.json"
)
DEFAULT_T2_PASS_BUCKET_DISABLED_SUMMARY = (
    OUTPUT_DIR
    / "t2_pass_bucket_gate_t2_disabled_extended"
    / "t2_lifecycle_pass_bucket_gate_summary.json"
)
DEFAULT_T2_EXTERNAL_LOW_EFF_RF_SUMMARY = (
    OUTPUT_DIR
    / "t2_external_low_eff_rf_median_extended"
    / "t2_lifecycle_external_event_gate_summary.json"
)

KIRO_SPEC_T3_EXIT_SNAPSHOT = {
    "source": ".kiro/specs/t2-t3-union-strategy/tasks.md Task 18/21",
    "timeframe": "1h",
    "reentry_fill_policy": "strict_next_second_cross",
    "calendar_grid": {
        "months": [
            "2025-06",
            "2025-07",
            "2025-08",
            "2025-09",
            "2025-10",
            "2025-11",
            "2025-12",
            "2026-01",
            "2026-02",
            "2026-03",
            "2026-04",
        ],
        "symbols": ["ETHUSDT", "BTCUSDT"],
        "symbol_months": 22,
    },
    "candidates": [
        {
            "candidate": "strict_baseline",
            "calendar_silo_sum_pct": -30.98,
            "worst_calendar_silo_pct": -2.19,
            "negative_calendar_silos": 22,
            "total_trades": 659,
            "t3_trades": 143,
            "t3_net_pnl_pct": -1.600810,
            "t3_win_rate_pct": 14.69,
        },
        {
            "candidate": "t3_min_hold_sl_60m",
            "calendar_silo_sum_pct": -24.48,
            "worst_calendar_silo_pct": -2.15,
            "negative_calendar_silos": 22,
            "total_trades": 610,
            "t3_trades": 100,
            "t3_net_pnl_pct": 3.845840,
            "t3_win_rate_pct": 47.00,
        },
    ],
}

KIRO_SPEC_T3_EXPOSURE_SNAPSHOT = {
    "source": ".kiro/specs/t2-t3-union-strategy/tasks.md Task 22",
    "candidates": [
        {
            "candidate": "t3_min_hold_sl_60m",
            "t3_net_pnl_pct": 3.845829,
            "t3_net_pnl_ex_final_mark_pct": 3.655098,
            "final_mark_pnl_pct": 0.190731,
            "final_mark_trades": 1,
            "t3_equity_max_dd_pct": -0.254383,
            "t3_p90_hold_seconds": 6245.20,
            "t3_worst_mae_bps": -477.5244,
        }
    ],
}


@dataclass(frozen=True)
class MetricRow:
    """One candidate metric row normalized for the merge matrix."""

    family: str
    candidate: str
    contract: str
    sample: str
    primary_metric: str
    primary_value: float
    delta_vs_baseline: float | None
    worst_metric: str
    worst_value: float | None
    negative_silos: int | None
    trades: int | None
    promotion_read: str


@dataclass(frozen=True)
class MergeDecision:
    """A compatibility decision for one merge surface."""

    surface: str
    decision: str
    reason: str
    next_action: str


@dataclass(frozen=True)
class ContractReadiness:
    """Whether a candidate is already comparable under strict lifecycle."""

    candidate: str
    current_contract: str
    strict_lifecycle_ready: bool
    blocker: str
    next_action: str


def _read_csv(path: Path) -> pd.DataFrame:
    if not path.exists():
        raise FileNotFoundError(path)
    return pd.read_csv(path)


def _read_json(path: Path) -> dict[str, Any] | None:
    if not path.exists():
        return None
    return json.loads(path.read_text(encoding="utf-8"))


def _payload_with_snapshot(path: Path, snapshot: dict[str, Any]) -> tuple[dict[str, Any], str]:
    payload = _read_json(path)
    if payload is not None:
        return payload, str(path)
    copied = json.loads(json.dumps(snapshot))
    copied["snapshot_fallback_used"] = True
    copied["missing_fresh_input"] = str(path)
    return copied, str(copied["source"])


def _float(row: pd.Series, column: str) -> float:
    return float(pd.to_numeric(row[column], errors="raise"))


def _int_or_none(value: Any) -> int | None:
    if value is None or pd.isna(value):
        return None
    return int(value)


def _lead_combo_rows(path: Path) -> list[MetricRow]:
    df = _read_csv(path)
    wanted = [
        "low_eff_rf_rank_median_000",
        "wide_rf_binary_000",
        "low_eff_rf_rank_q60_000",
    ]
    rows: list[MetricRow] = []
    for name in wanted:
        match = df[df["variant"] == name]
        if match.empty:
            continue
        row = match.iloc[0]
        rows.append(
            MetricRow(
                family="current_breakout_expansion",
                candidate=str(row["variant"]),
                contract="lead combo event ledger, next_adverse_xslip10bps",
                sample="ETHUSDT 2025-06..2026-04, train3m selected expansion",
                primary_metric="combo_adverse10_calendar_sum",
                primary_value=_float(row, "combo_adverse10_calendar_sum"),
                delta_vs_baseline=_float(row, "combo_adverse10_delta_vs_lead"),
                worst_metric="combo_adverse10_worst_sm",
                worst_value=_float(row, "combo_adverse10_worst_sm"),
                negative_silos=_int_or_none(row["combo_adverse10_neg_sm"]),
                trades=_int_or_none(row["combo_adverse10_trade_count"]),
                promotion_read=(
                    "late-ETH additive candidate only; still blocked by early ETH/BTC falsification"
                    if name == "low_eff_rf_rank_median_000"
                    else "useful sensitivity row, not the preferred merge leg"
                ),
            )
        )
    return rows


def _lead_retrain_rows(path: Path) -> list[MetricRow]:
    df = _read_csv(path)
    df = df[df["feature_set"] == "production8"].copy()
    wanted = [
        "canonical_only",
        "combo_wf3_low_eff_low_atr",
        "combo_wf3_low_eff_low_atr_ctx4h_scaled025",
        "combo_wf3_low_eff_low_atr_ctx12h_up",
    ]
    canonical = df[df["pool"] == "canonical_only"]
    baseline_value = (
        _float(canonical.iloc[0], "forward_adverse10_calendar_sum") if not canonical.empty else None
    )
    rows: list[MetricRow] = []
    for name in wanted:
        match = df[df["pool"] == name]
        if match.empty:
            continue
        row = match.iloc[0]
        value = _float(row, "forward_adverse10_calendar_sum")
        rows.append(
            MetricRow(
                family="current_breakout_retrain",
                candidate=str(row["pool"]),
                contract="production8 retrain, forward next_adverse_xslip10bps",
                sample="ETHUSDT forward >= 2025-11, live-like exit trail_start_r=1.5 max_hold_hours=2.0",
                primary_metric="forward_adverse10_calendar_sum",
                primary_value=value,
                delta_vs_baseline=(value - baseline_value) if baseline_value is not None else None,
                worst_metric="forward_adverse10_worst_sm",
                worst_value=_float(row, "forward_adverse10_worst_sm"),
                negative_silos=_int_or_none(row["forward_adverse10_neg_sm"]),
                trades=_int_or_none(row["forward_trade_count"]),
                promotion_read=(
                    "best current ETH-local sizing-control shape"
                    if name == "combo_wf3_low_eff_low_atr_ctx4h_scaled025"
                    else "reference row for current breakout line"
                ),
            )
        )
    return rows


def _strict_baseline_from_stability(path: Path | None) -> dict[str, Any] | None:
    if path is None:
        return None
    payload = _read_json(path)
    if payload is None:
        return None
    metrics = payload.get("metrics", {})
    return {
        "candidate": "strict_baseline",
        "calendar_silo_sum_pct": float(metrics["calendar_silo_sum_pct"]),
        "worst_calendar_silo_pct": float(metrics["worst_calendar_silo_pct"]),
        "negative_calendar_silos": int(metrics["negative_calendar_silos"]),
        "total_trades": int(metrics["total_trades"]),
        "t3_trades": int(metrics["t3_trades"]),
        "t3_net_pnl_pct": float(metrics["t3_net_pnl_pct"]),
        "t3_win_rate_pct": float(metrics["t3_win_rate_pct"]),
        "_source": str(path),
    }


def _t3_exit_rows(path: Path, baseline_path: Path | None) -> list[MetricRow]:
    payload, source = _payload_with_snapshot(path, KIRO_SPEC_T3_EXIT_SNAPSHOT)
    rows: list[MetricRow] = []
    candidates = [dict(item) for item in payload.get("candidates", [])]
    fresh_baseline = _strict_baseline_from_stability(baseline_path)
    if fresh_baseline is not None:
        candidates = [
            item for item in candidates
            if str(item.get("candidate")) != "strict_baseline"
        ]
        candidates = [fresh_baseline, *candidates]
    labels = {str(item.get("candidate")) for item in candidates}
    if "strict_baseline" not in labels:
        baseline = dict(KIRO_SPEC_T3_EXIT_SNAPSHOT["candidates"][0])
        baseline["_source"] = KIRO_SPEC_T3_EXIT_SNAPSHOT["source"]
        candidates = [baseline, *candidates]
    baseline = next(
        (item for item in candidates if str(item.get("candidate")) == "strict_baseline"),
        candidates[0] if candidates else None,
    )
    baseline_value = float(baseline["calendar_silo_sum_pct"]) if baseline else None
    for item in candidates:
        value = float(item["calendar_silo_sum_pct"])
        row_source = str(item.get("_source", source))
        rows.append(
            MetricRow(
                family="t3_strict_lifecycle",
                candidate=str(item["candidate"]),
                contract=f"baseline_plus_t3 lifecycle, {payload.get('reentry_fill_policy', 'unknown')}",
                sample=_grid_label(payload),
                primary_metric="calendar_silo_sum_pct",
                primary_value=value,
                delta_vs_baseline=(value - baseline_value) if baseline_value is not None else None,
                worst_metric="worst_calendar_silo_pct",
                worst_value=float(item["worst_calendar_silo_pct"]),
                negative_silos=int(item["negative_calendar_silos"]),
                trades=int(item["total_trades"]),
                promotion_read=(
                    f"strict T3 watch leg; positive T3 split but total lifecycle still weak; source={row_source}"
                    if item["candidate"] == "t3_min_hold_sl_60m"
                    else f"strict lifecycle baseline/control; source={row_source}"
                ),
            )
        )
    return rows


def _t2_lifecycle_context_rows(path: Path, family: str) -> list[MetricRow]:
    payload = _read_json(path)
    if payload is None:
        return []
    candidates = [dict(item) for item in payload.get("candidates", [])]
    if not candidates:
        return []
    baseline = next(
        (item for item in candidates if str(item.get("candidate")) == "strict_baseline"),
        None,
    )
    baseline_value = float(baseline["calendar_silo_sum_pct"]) if baseline is not None else None
    reentry_policy = str(payload.get("reentry_fill_policy", "unknown"))
    rows: list[MetricRow] = []
    for item in candidates:
        if str(item.get("candidate")) == "strict_baseline":
            continue
        value = float(item["calendar_silo_sum_pct"])
        t3_overrides = dict(item.get("t3_exit_overrides", {}))
        read_prefix = (
            "strict lifecycle union row with T2 ctx4h sizing and T3 exit overrides"
            if t3_overrides
            else "strict lifecycle bridge for the current ctx4h scaled-sizing idea"
        )
        rows.append(
            MetricRow(
                family=family,
                candidate=str(item["candidate"]),
                contract=f"baseline_plus_t3 lifecycle, {reentry_policy}, original_t2 sizing filter",
                sample=_grid_label(payload),
                primary_metric="calendar_silo_sum_pct",
                primary_value=value,
                delta_vs_baseline=round(value - baseline_value, 6)
                if baseline_value is not None
                else None,
                worst_metric="worst_calendar_silo_pct",
                worst_value=float(item["worst_calendar_silo_pct"]),
                negative_silos=int(item["negative_calendar_silos"]),
                trades=int(item["total_trades"]),
                promotion_read=(
                    f"{read_prefix}; "
                    f"T2 PnL {float(item.get('t2_net_pnl_pct', 0.0)):.6f}%, "
                    f"T3 PnL {float(item.get('t3_net_pnl_pct', 0.0)):.6f}%, "
                    f"size fails {int(item.get('t2_size_filter_fails', 0))}; "
                    f"T3 overrides={json.dumps(t3_overrides, sort_keys=True)}; "
                    f"source={path}"
                ),
            )
        )
    return rows


def _external_event_rows(path: Path, family: str) -> list[MetricRow]:
    payload = _read_json(path)
    if payload is None:
        return []
    reentry_policy = str(payload.get("reentry_fill_policy", "unknown"))
    rows: list[MetricRow] = []
    for item in payload.get("candidates", []):
        value = float(item["calendar_silo_sum_pct"])
        rows.append(
            MetricRow(
                family=family,
                candidate=str(item["candidate"]),
                contract=f"baseline_plus_t3 lifecycle, {reentry_policy}, external probability event locks",
                sample=_grid_label(payload),
                primary_metric="calendar_silo_sum_pct",
                primary_value=value,
                delta_vs_baseline=None,
                worst_metric="worst_calendar_silo_pct",
                worst_value=float(item["worst_calendar_silo_pct"]),
                negative_silos=int(item["negative_calendar_silos"]),
                trades=int(item["total_trades"]),
                promotion_read=(
                    "strict lifecycle bridge for probability-selected external T2 events; "
                    f"external events {int(item.get('external_events_available', 0))}, "
                    f"locks {int(item.get('external_locks', 0))}, "
                    f"trades {int(item.get('external_trades', 0))}, "
                    f"external PnL {float(item.get('external_net_pnl_pct', 0.0)):.6f}%, "
                    f"T3 PnL {float(item.get('t3_net_pnl_pct', 0.0)):.6f}%; "
                    f"source={path}"
                ),
            )
        )
    return rows


def _t3_exposure_notes(path: Path) -> list[str]:
    payload, source = _payload_with_snapshot(path, KIRO_SPEC_T3_EXPOSURE_SNAPSHOT)
    candidates = payload.get("candidates", [])
    notes: list[str] = []
    for item in candidates:
        notes.append(
            (
                f"{item['candidate']}: T3 PnL {item['t3_net_pnl_pct']:.6f}%, "
                f"ex-final-mark {item['t3_net_pnl_ex_final_mark_pct']:.6f}%, "
                f"FinalMark {item['final_mark_pnl_pct']:.6f}%/{item['final_mark_trades']}, "
                f"T3 DD {item['t3_equity_max_dd_pct']:.6f}%, "
                f"p90 hold {item['t3_p90_hold_seconds']:.2f}s, "
                f"worst MAE {item['t3_worst_mae_bps']:.4f}bp "
                f"(source={source})"
            )
        )
    return notes


def _grid_label(payload: dict[str, Any]) -> str:
    grid = payload.get("calendar_grid", {})
    months = grid.get("months", [])
    symbols = grid.get("symbols", [])
    month_label = f"{months[0]}..{months[-1]}" if months else "unknown months"
    symbol_label = "/".join(symbols) if symbols else "unknown symbols"
    return f"{month_label} x {symbol_label}"


def build_merge_decisions() -> list[MergeDecision]:
    return [
        MergeDecision(
            surface="T2 canonical + current breakout expansion",
            decision="merge as additive candidate",
            reason="Both are event-ledger/retrain outputs under next_adverse_xslip10bps and already de-overlap by (signal_start, side).",
            next_action="Keep low_eff_rf_rank_median_000 and ctx4h_scaled025 as the current ETH-local falsification set.",
        ),
        MergeDecision(
            surface="T2 canonical + T3 generator/quality/probability harness",
            decision="merge the framework and keep strict lifecycle as the comparison contract",
            reason="T3 structure is mutually exclusive with T2 and uses compatible features; the bridge replay now prevents adding adverse10 ledger returns to lifecycle returns.",
            next_action="Use the strict lifecycle union row as the research comparison surface; keep adverse10 ledgers as diagnostic provenance only.",
        ),
        MergeDecision(
            surface="T3 historical lifecycle positives",
            decision="do not merge",
            reason="Historical reentry_window allowed optimistic same-second/non-cross re_p fills; strict replay invalidated the large T3 contribution.",
            next_action="Treat Task 14-17 historical T3 positive results as suspect-only provenance.",
        ),
        MergeDecision(
            surface="T3 min_hold_sl_60m",
            decision="watch-only merge leg",
            reason="It improves strict T3 split and survives final-mark exclusion, but increases exposure and total strict lifecycle remains negative.",
            next_action="Keep it inside strict lifecycle union tests only; do not promote it to live stop semantics.",
        ),
        MergeDecision(
            surface="T2 ctx4h sizing + T3 min_hold_sl_60m",
            decision="promising bridge result, not promotion-ready",
            reason="The two risk-shaping layers stack under strict_next_second_cross, but the combined fixed-calendar lifecycle remains negative.",
            next_action="Attack residual T2 loss and BTC drag before any live/default discussion.",
        ),
        MergeDecision(
            surface="T2 ctx4h fail multiplier sensitivity",
            decision="zero-fail exposure is the best strict research row; skip-fail is the cleaner executable challenger",
            reason="Reducing failed-context original_t2 exposure improves total lifecycle and worst silo; skip-fail keeps most of the improvement without relying on zero-notional lock occupancy.",
            next_action="Use skip-fail as the cleaner next implementation surface, while retaining scaled000 as the upper-bound research control.",
        ),
        MergeDecision(
            surface="T2 pass/full bucket",
            decision="reject current original_t2 pass bucket as a trading leg",
            reason="Strict lifecycle attribution shows all simple pass buckets remain fee-negative; disabling original_t2 beats ctx4h skip-fail and pre_touch<=900.",
            next_action="Do not spend more sweeps on ctx4h/pass timing knobs; either disable original_t2 or implement an exact event-time probability/RF lifecycle hook.",
        ),
        MergeDecision(
            surface="low_eff/RF external event hook",
            decision="reject as-is under strict lifecycle",
            reason="The RF-selected event set can be injected as explicit locks, but only a minority become strict reentry trades and the external leg is still net negative.",
            next_action="Use RF only with a post-touch entry/confirmation redesign; do not treat the adverse10 event-ledger result as lifecycle-ready.",
        ),
    ]


def _has_t2_lifecycle_context_result(path: Path) -> bool:
    payload = _read_json(path)
    if payload is None:
        return False
    return any(
        str(item.get("candidate")) == "original_t2_ctx4h_scaled025"
        for item in payload.get("candidates", [])
    )


def build_contract_readiness(t2_lifecycle_context_summary: Path) -> list[ContractReadiness]:
    has_ctx4h_lifecycle = _has_t2_lifecycle_context_result(t2_lifecycle_context_summary)
    return [
        ContractReadiness(
            candidate="low_eff_rf_rank_median_000_combo",
            current_contract="T2 next_adverse_xslip10bps event ledger",
            strict_lifecycle_ready=False,
            blocker="No executable lifecycle breakout-lock rule yet; current row is additive ledger selection.",
            next_action="Translate the selected low-eff/RF leg into a replay-time original_t2 quality gate or keep it ledger-only.",
        ),
        ContractReadiness(
            candidate="wf3_low_eff_low_atr_ctx4h_scaled025_combo",
            current_contract="T2 next_adverse_xslip10bps event ledger with derived scaled sizing",
            strict_lifecycle_ready=has_ctx4h_lifecycle,
            blocker=(
                "Bridge hook has a full fixed-calendar lifecycle result for the original_t2 ctx4h scaled-sizing approximation; exact event-ledger parity is still pending."
                if has_ctx4h_lifecycle
                else "Research hook exists, but only ETHUSDT 2026-04 smoke has run; full fixed-calendar lifecycle grid is still missing."
            ),
            next_action=(
                "Use the lifecycle bridge result to decide whether ctx4h sizing deserves exact parity work or should be rejected."
                if has_ctx4h_lifecycle
                else "Run `t2_lifecycle_context_sizing.py` over ETHUSDT/BTCUSDT 2025-06..2026-04 before promotion comparison."
            ),
        ),
        ContractReadiness(
            candidate="strict_baseline",
            current_contract="baseline_plus_t3 strict_next_second_cross lifecycle",
            strict_lifecycle_ready=True,
            blocker="None for T3 baseline; it is already the strict lifecycle control.",
            next_action="Use as lifecycle control, not as a profitability benchmark for adverse10 T2 ledgers.",
        ),
        ContractReadiness(
            candidate="t3_min_hold_sl_60m",
            current_contract="baseline_plus_t3 strict_next_second_cross lifecycle with T3-only exit override",
            strict_lifecycle_ready=True,
            blocker="Watch-only because total lifecycle remains negative and exposure risk must stay visible.",
            next_action="Only compare against T2 once T2 expansion is replayed through the same lifecycle contract.",
        ),
    ]


def _markdown_table(rows: list[list[Any]], headers: list[str]) -> str:
    lines = [
        "| " + " | ".join(headers) + " |",
        "| " + " | ".join(["---"] * len(headers)) + " |",
    ]
    for row in rows:
        lines.append("| " + " | ".join(str(value) for value in row) + " |")
    return "\n".join(lines)


def _fmt_value(value: float | None) -> str:
    if value is None:
        return "n/a"
    return f"{value:.6f}"


def write_outputs(
    *,
    output_dir: Path,
    metric_rows: list[MetricRow],
    decisions: list[MergeDecision],
    exposure_notes: list[str],
    contract_readiness: list[ContractReadiness],
    inputs: dict[str, str],
) -> None:
    output_dir.mkdir(parents=True, exist_ok=True)
    payload = {
        "note": "Research-only merge matrix. Metrics with different contracts are intentionally not added together.",
        "inputs": inputs,
        "metric_rows": [asdict(row) for row in metric_rows],
        "merge_decisions": [asdict(row) for row in decisions],
        "contract_readiness": [asdict(row) for row in contract_readiness],
        "t3_exposure_notes": exposure_notes,
    }
    (output_dir / "t2_t3_merge_matrix_summary.json").write_text(
        json.dumps(payload, indent=2, ensure_ascii=False),
        encoding="utf-8",
    )

    metric_table = []
    for row in metric_rows:
        metric_table.append(
            [
                row.family,
                f"`{row.candidate}`",
                row.primary_metric,
                _fmt_value(row.primary_value),
                _fmt_value(row.delta_vs_baseline),
                row.worst_metric,
                _fmt_value(row.worst_value),
                row.negative_silos if row.negative_silos is not None else "n/a",
                row.trades if row.trades is not None else "n/a",
                row.promotion_read,
            ]
        )
    decision_table = [
        [row.surface, row.decision, row.reason, row.next_action]
        for row in decisions
    ]
    readiness_table = [
        [
            f"`{row.candidate}`",
            row.current_contract,
            "yes" if row.strict_lifecycle_ready else "no",
            row.blocker,
            row.next_action,
        ]
        for row in contract_readiness
    ]

    lines = [
        "# T2/T3 Merge Matrix - 2026-05-18",
        "",
        "Scope: research-only. This report evaluates whether the current ETH pretouch breakout research line can be merged with the Kiro `t2-t3-union-strategy` line. It does not change live defaults.",
        "",
        "## Key Read",
        "",
        "- Merge the T3 framework and strict-fill audit into the current research process.",
        "- Do not merge historical T3 lifecycle headline returns; they depended on optimistic `re_p` fills.",
        "- Treat `t3_min_hold_sl_60m` as a strict-fill watch leg inside research union tests, not as a default stop-loss rule.",
        "- Current strict lifecycle pass-bucket read: original_t2 is the main drag; T2 disabled + T3 60m is the new strict floor to beat.",
        "- Exact RF-event injection is now tested and does not beat the T2-disabled floor; the next probability-model lever must change post-touch entry, not only event selection.",
        "",
        "## Metric Matrix",
        "",
        _markdown_table(
            metric_table,
            [
                "Family",
                "Candidate",
                "Primary Metric",
                "Value",
                "Delta",
                "Worst Metric",
                "Worst",
                "Neg",
                "Trades",
                "Read",
            ],
        ),
        "",
        "## T3 Exposure Notes",
        "",
        *[f"- {note}" for note in exposure_notes],
        "",
        "## Merge Decisions",
        "",
        _markdown_table(decision_table, ["Surface", "Decision", "Reason", "Next Action"]),
        "",
        "## Lifecycle Contract Readiness",
        "",
        _markdown_table(
            readiness_table,
            ["Candidate", "Current Contract", "Strict Lifecycle Ready", "Blocker", "Next Action"],
        ),
        "",
        "## Next Promotion Contract",
        "",
        "1. Use strict lifecycle over ETHUSDT/BTCUSDT `2025-06..2026-04` as the only promotion-comparable contract.",
        "2. Keep `next_adverse_xslip10bps` T2 ledgers as diagnostics; do not add them to lifecycle returns.",
        "3. Keep long context filters bounded to 4h-12h and avoid month-level gates.",
        "4. Next research lever: attack remaining pass bucket loss or implement an exact low-eff/RF lifecycle hook.",
        "5. Promotion gate: no `re_p`, no live/default change unless total lifecycle, worst silo, and exposure audit all improve.",
        "",
    ]
    (output_dir / "t2_t3_merge_matrix_report.md").write_text("\n".join(lines), encoding="utf-8")


def run(
    *,
    output_dir: Path,
    lead_combo_summary: Path,
    lead_retrain_summary: Path,
    t3_exit_summary: Path,
    t3_exposure_summary: Path,
    t3_baseline_summary: Path | None = DEFAULT_T3_BASELINE_SUMMARY,
    t2_lifecycle_context_summary: Path = DEFAULT_T2_LIFECYCLE_CONTEXT_SUMMARY,
    t2_t3_union_lifecycle_summary: Path = DEFAULT_T2_T3_UNION_LIFECYCLE_SUMMARY,
    t2_ctx4h_multiplier_sensitivity_summary: Path = DEFAULT_T2_CTX4H_MULTIPLIER_SENSITIVITY_SUMMARY,
    t2_ctx4h_skipfail_summary: Path = DEFAULT_T2_CTX4H_SKIPFAIL_SUMMARY,
    t2_pass_bucket_reference_summary: Path = DEFAULT_T2_PASS_BUCKET_REFERENCE_SUMMARY,
    t2_pass_bucket_pretouch900_summary: Path = DEFAULT_T2_PASS_BUCKET_PRETOUCH900_SUMMARY,
    t2_pass_bucket_disabled_summary: Path = DEFAULT_T2_PASS_BUCKET_DISABLED_SUMMARY,
    t2_external_low_eff_rf_summary: Path | None = None,
) -> None:
    metric_rows: list[MetricRow] = []
    metric_rows.extend(_lead_combo_rows(lead_combo_summary))
    metric_rows.extend(_lead_retrain_rows(lead_retrain_summary))
    metric_rows.extend(_t3_exit_rows(t3_exit_summary, t3_baseline_summary))
    metric_rows.extend(
        _t2_lifecycle_context_rows(
            t2_lifecycle_context_summary,
            family="t2_strict_lifecycle_context_sizing",
        )
    )
    metric_rows.extend(
        _t2_lifecycle_context_rows(
            t2_t3_union_lifecycle_summary,
            family="t2_t3_strict_lifecycle_union",
        )
    )
    metric_rows.extend(
        _t2_lifecycle_context_rows(
            t2_ctx4h_multiplier_sensitivity_summary,
            family="t2_t3_strict_lifecycle_multiplier_sensitivity",
        )
    )
    metric_rows.extend(
        _t2_lifecycle_context_rows(
            t2_ctx4h_skipfail_summary,
            family="t2_t3_strict_lifecycle_skipfail",
        )
    )
    metric_rows.extend(
        _t2_lifecycle_context_rows(
            t2_pass_bucket_reference_summary,
            family="t2_t3_strict_lifecycle_pass_bucket_reference",
        )
    )
    metric_rows.extend(
        _t2_lifecycle_context_rows(
            t2_pass_bucket_pretouch900_summary,
            family="t2_t3_strict_lifecycle_pass_bucket_gate",
        )
    )
    metric_rows.extend(
        _t2_lifecycle_context_rows(
            t2_pass_bucket_disabled_summary,
            family="t2_t3_strict_lifecycle_t2_disabled_floor",
        )
    )
    if t2_external_low_eff_rf_summary is not None:
        metric_rows.extend(
            _external_event_rows(
                t2_external_low_eff_rf_summary,
                family="t2_t3_strict_lifecycle_external_probability",
            )
        )
    write_outputs(
        output_dir=output_dir,
        metric_rows=metric_rows,
        decisions=build_merge_decisions(),
        exposure_notes=_t3_exposure_notes(t3_exposure_summary),
        contract_readiness=build_contract_readiness(t2_lifecycle_context_summary),
        inputs={
            "lead_combo_summary": str(lead_combo_summary),
            "lead_retrain_summary": str(lead_retrain_summary),
            "t3_exit_summary": str(t3_exit_summary),
            "t3_baseline_summary": str(t3_baseline_summary) if t3_baseline_summary is not None else "",
            "t3_exposure_summary": str(t3_exposure_summary),
            "t2_lifecycle_context_summary": str(t2_lifecycle_context_summary),
            "t2_t3_union_lifecycle_summary": str(t2_t3_union_lifecycle_summary),
            "t2_ctx4h_multiplier_sensitivity_summary": str(t2_ctx4h_multiplier_sensitivity_summary),
            "t2_ctx4h_skipfail_summary": str(t2_ctx4h_skipfail_summary),
            "t2_pass_bucket_reference_summary": str(t2_pass_bucket_reference_summary),
            "t2_pass_bucket_pretouch900_summary": str(t2_pass_bucket_pretouch900_summary),
            "t2_pass_bucket_disabled_summary": str(t2_pass_bucket_disabled_summary),
            "t2_external_low_eff_rf_summary": str(t2_external_low_eff_rf_summary)
            if t2_external_low_eff_rf_summary is not None
            else "",
        },
    )


def main() -> None:
    parser = argparse.ArgumentParser(description="Build T2/T3 research merge matrix")
    parser.add_argument("--output-dir", default=str(OUTPUT_DIR))
    parser.add_argument("--lead-combo-summary", default=str(LEAD_COMBO_SUMMARY))
    parser.add_argument("--lead-retrain-summary", default=str(LEAD_RETRAIN_SUMMARY))
    parser.add_argument("--t3-exit-summary", default=str(DEFAULT_T3_EXIT_SUMMARY))
    parser.add_argument("--t3-baseline-summary", default=str(DEFAULT_T3_BASELINE_SUMMARY))
    parser.add_argument("--t3-exposure-summary", default=str(DEFAULT_T3_EXPOSURE_SUMMARY))
    parser.add_argument("--t2-lifecycle-context-summary", default=str(DEFAULT_T2_LIFECYCLE_CONTEXT_SUMMARY))
    parser.add_argument("--t2-t3-union-lifecycle-summary", default=str(DEFAULT_T2_T3_UNION_LIFECYCLE_SUMMARY))
    parser.add_argument(
        "--t2-ctx4h-multiplier-sensitivity-summary",
        default=str(DEFAULT_T2_CTX4H_MULTIPLIER_SENSITIVITY_SUMMARY),
    )
    parser.add_argument("--t2-ctx4h-skipfail-summary", default=str(DEFAULT_T2_CTX4H_SKIPFAIL_SUMMARY))
    parser.add_argument("--t2-pass-bucket-reference-summary", default=str(DEFAULT_T2_PASS_BUCKET_REFERENCE_SUMMARY))
    parser.add_argument("--t2-pass-bucket-pretouch900-summary", default=str(DEFAULT_T2_PASS_BUCKET_PRETOUCH900_SUMMARY))
    parser.add_argument("--t2-pass-bucket-disabled-summary", default=str(DEFAULT_T2_PASS_BUCKET_DISABLED_SUMMARY))
    parser.add_argument("--t2-external-low-eff-rf-summary", default=str(DEFAULT_T2_EXTERNAL_LOW_EFF_RF_SUMMARY))
    args = parser.parse_args()
    run(
        output_dir=Path(args.output_dir),
        lead_combo_summary=Path(args.lead_combo_summary),
        lead_retrain_summary=Path(args.lead_retrain_summary),
        t3_exit_summary=Path(args.t3_exit_summary),
        t3_baseline_summary=Path(args.t3_baseline_summary),
        t3_exposure_summary=Path(args.t3_exposure_summary),
        t2_lifecycle_context_summary=Path(args.t2_lifecycle_context_summary),
        t2_t3_union_lifecycle_summary=Path(args.t2_t3_union_lifecycle_summary),
        t2_ctx4h_multiplier_sensitivity_summary=Path(args.t2_ctx4h_multiplier_sensitivity_summary),
        t2_ctx4h_skipfail_summary=Path(args.t2_ctx4h_skipfail_summary),
        t2_pass_bucket_reference_summary=Path(args.t2_pass_bucket_reference_summary),
        t2_pass_bucket_pretouch900_summary=Path(args.t2_pass_bucket_pretouch900_summary),
        t2_pass_bucket_disabled_summary=Path(args.t2_pass_bucket_disabled_summary),
        t2_external_low_eff_rf_summary=Path(args.t2_external_low_eff_rf_summary),
    )
    print(f"Report: {Path(args.output_dir) / 't2_t3_merge_matrix_report.md'}")


if __name__ == "__main__":
    main()
