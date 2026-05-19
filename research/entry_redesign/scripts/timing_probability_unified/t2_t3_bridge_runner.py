"""Bridge report for T2 breakout expansion and strict T3 lifecycle research.

This is a research-only bridge, not a portfolio simulator. It normalizes the
current T2 adverse-fill event ledger and the Kiro T3 strict lifecycle snapshots
into percent units on a shared month axis, then marks the remaining semantic
gap. The output is meant to guide the next replay implementation without
pretending that event-ledger adverse10 returns and lifecycle returns are
directly additive.
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

LEAD_ADVERSE_TRADES = OUTPUT_DIR / "breakout_structure_lead_combo_lead_adverse10_trades.csv"
LOW_EFF_RF_MEDIAN_EXTRA_TRADES = (
    OUTPUT_DIR / "breakout_structure_context_model_lead_combo_extra_adverse10_trades_low_eff_rf_rank_median_000.csv"
)
WF3_CTX4H_UP_EXTRA_TRADES = OUTPUT_DIR / "breakout_structure_lead_combo_extra_adverse10_trades_wf3_low_eff_low_atr_ctx4h_up.csv"
WF3_LOW_EFF_EXTRA_TRADES = OUTPUT_DIR / "breakout_structure_lead_combo_extra_adverse10_trades_wf3_low_eff_low_atr.csv"
T3_EXIT_SUMMARY = OUTPUT_DIR / "t2_t3_merge_t3_exit_60m_extended" / "t3_lifecycle_exit_sweep_summary.json"
SCALED_CTX4H_EXTRA_TRADES = (
    OUTPUT_DIR / "breakout_structure_lead_combo_extra_adverse10_trades_wf3_low_eff_low_atr_ctx4h_scaled025.csv"
)
SCALED_CTX4H_COMBO_TRADES = (
    OUTPUT_DIR / "breakout_structure_lead_combo_combo_adverse10_trades_wf3_low_eff_low_atr_ctx4h_scaled025.csv"
)

DEFAULT_MONTHS = [
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
]

T3_MIN_HOLD_60M_MONTHLY_PCT = {
    "2025-06": -0.057750,
    "2025-07": 0.679510,
    "2025-08": 0.127230,
    "2025-09": 1.022350,
    "2025-10": 0.618490,
    "2025-11": 0.168500,
    "2025-12": -0.228120,
    "2026-01": 0.059650,
    "2026-02": 0.772600,
    "2026-03": 0.636550,
    "2026-04": 0.046830,
}


@dataclass(frozen=True)
class BridgeCandidate:
    """One row in the bridge candidate table."""

    family: str
    candidate: str
    contract: str
    scope: str
    calendar_sum_pct: float
    delta_vs_baseline_pct: float | None
    worst_month_pct: float
    negative_months: int
    trades: int | None
    source: str
    read: str


@dataclass(frozen=True)
class BridgeMonthRow:
    """Shared month-axis row for non-additive comparison."""

    month: str
    t2_lead_adverse10_pct: float
    t2_low_eff_rf_median_combo_pct: float
    t2_low_eff_rf_median_extra_pct: float
    t2_ctx4h_scaled025_combo_pct: float
    t2_ctx4h_scaled025_extra_pct: float
    t3_min_hold_60m_t3_pnl_pct: float
    bridge_read: str


def _read_csv(path: Path) -> pd.DataFrame:
    if not path.exists():
        raise FileNotFoundError(path)
    df = pd.read_csv(path)
    if "touch_time" in df.columns:
        df["touch_time"] = pd.to_datetime(df["touch_time"], utc=True)
    return df


def _read_json(path: Path) -> dict[str, Any] | None:
    if not path.exists():
        return None
    return json.loads(path.read_text(encoding="utf-8"))


def _monthly_pct(trades: pd.DataFrame, months: list[str]) -> dict[str, float]:
    if trades.empty:
        return {month: 0.0 for month in months}
    df = trades.copy()
    df["month"] = pd.to_datetime(df["touch_time"], utc=True).dt.strftime("%Y-%m")
    grouped = df.groupby("month")["weighted_pnl"].sum()
    return {month: round(float(grouped.get(month, 0.0)) * 100.0, 6) for month in months}


def _candidate_from_monthly(
    *,
    family: str,
    candidate: str,
    contract: str,
    scope: str,
    monthly: dict[str, float],
    baseline_sum_pct: float | None,
    trades: int | None,
    source: str,
    read: str,
) -> BridgeCandidate:
    values = list(monthly.values())
    total = round(float(sum(values)), 6)
    return BridgeCandidate(
        family=family,
        candidate=candidate,
        contract=contract,
        scope=scope,
        calendar_sum_pct=total,
        delta_vs_baseline_pct=round(total - baseline_sum_pct, 6) if baseline_sum_pct is not None else None,
        worst_month_pct=round(float(min(values, default=0.0)), 6),
        negative_months=int(sum(1 for value in values if value < 0.0)),
        trades=trades,
        source=source,
        read=read,
    )


def _scaled_ctx4h_extra_ledger(wf3_extra: pd.DataFrame, ctx4h_hard_extra: pd.DataFrame) -> pd.DataFrame:
    """Derive the ctx4h fail_weight=0.25 ledger from existing full/hard ledgers."""
    if wf3_extra.empty:
        return wf3_extra.copy()
    pass_keys = set(ctx4h_hard_extra.get("event_key", pd.Series(dtype=object)).astype(str))
    scaled = wf3_extra.copy().reset_index(drop=True)
    scaled["context_pass"] = scaled["event_key"].astype(str).isin(pass_keys)
    scaled["context_fail_weight"] = 0.25
    fail_mask = ~scaled["context_pass"]
    for column in ("sizing_multiplier", "position_size", "weighted_pnl"):
        if column in scaled.columns:
            scaled.loc[fail_mask, column] = pd.to_numeric(
                scaled.loc[fail_mask, column],
                errors="coerce",
            ).fillna(0.0) * 0.25
    scaled["source_leg"] = "wf3_low_eff_low_atr_ctx4h_scaled025"
    return scaled


def _t3_monthly_from_json(path: Path, months: list[str]) -> tuple[dict[str, float], int | None, str]:
    payload = _read_json(path)
    if payload is None:
        return (
            {month: float(T3_MIN_HOLD_60M_MONTHLY_PCT.get(month, 0.0)) for month in months},
            100,
            ".kiro spec Task 21 monthly snapshot",
        )

    silos = payload.get("silos", [])
    rows = [
        row for row in silos
        if row.get("candidate") == "t3_min_hold_sl_60m"
    ]
    if not rows:
        return (
            {month: float(T3_MIN_HOLD_60M_MONTHLY_PCT.get(month, 0.0)) for month in months},
            100,
            ".kiro spec Task 21 monthly snapshot",
        )

    monthly = {month: 0.0 for month in months}
    trades = 0
    for row in rows:
        month = str(row.get("month"))
        if month not in monthly:
            continue
        monthly[month] = round(monthly[month] + float(row.get("t3_net_pnl_pct", 0.0)), 6)
        trades += int(row.get("t3_trades", 0))
    return monthly, trades, str(path)


def build_bridge(months: list[str]) -> tuple[list[BridgeCandidate], list[BridgeMonthRow]]:
    lead_trades = _read_csv(LEAD_ADVERSE_TRADES)
    low_eff_extra = _read_csv(LOW_EFF_RF_MEDIAN_EXTRA_TRADES)
    wf3_full_extra = _read_csv(WF3_LOW_EFF_EXTRA_TRADES)
    wf3_ctx4h_extra = _read_csv(WF3_CTX4H_UP_EXTRA_TRADES)
    wf3_ctx4h_scaled_extra = _scaled_ctx4h_extra_ledger(wf3_full_extra, wf3_ctx4h_extra)
    wf3_ctx4h_scaled_combo = pd.concat([lead_trades, wf3_ctx4h_scaled_extra], ignore_index=True)
    wf3_ctx4h_scaled_extra.to_csv(SCALED_CTX4H_EXTRA_TRADES, index=False)
    wf3_ctx4h_scaled_combo.to_csv(SCALED_CTX4H_COMBO_TRADES, index=False)

    lead_monthly = _monthly_pct(lead_trades, months)
    low_eff_extra_monthly = _monthly_pct(low_eff_extra, months)
    low_eff_combo_monthly = {
        month: round(lead_monthly[month] + low_eff_extra_monthly[month], 6)
        for month in months
    }
    wf3_ctx4h_combo_monthly = {
        month: round(lead_monthly[month] + _monthly_pct(wf3_ctx4h_extra, months)[month], 6)
        for month in months
    }
    wf3_ctx4h_scaled_extra_monthly = _monthly_pct(wf3_ctx4h_scaled_extra, months)
    wf3_ctx4h_scaled_combo_monthly = {
        month: round(lead_monthly[month] + wf3_ctx4h_scaled_extra_monthly[month], 6)
        for month in months
    }
    t3_monthly, t3_trades, t3_source = _t3_monthly_from_json(T3_EXIT_SUMMARY, months)

    lead_sum = round(sum(lead_monthly.values()), 6)
    candidates = [
        _candidate_from_monthly(
            family="t2_event_ledger",
            candidate="canonical_lead",
            contract="next_adverse_xslip10bps",
            scope="ETHUSDT only",
            monthly=lead_monthly,
            baseline_sum_pct=lead_sum,
            trades=int(len(lead_trades)),
            source=str(LEAD_ADVERSE_TRADES),
            read="current production-aligned T2 lead under adverse10",
        ),
        _candidate_from_monthly(
            family="t2_event_ledger",
            candidate="low_eff_rf_rank_median_000_combo",
            contract="next_adverse_xslip10bps",
            scope="ETHUSDT only",
            monthly=low_eff_combo_monthly,
            baseline_sum_pct=lead_sum,
            trades=int(len(lead_trades) + len(low_eff_extra)),
            source=str(LOW_EFF_RF_MEDIAN_EXTRA_TRADES),
            read="preferred hard-select additive T2 expansion to falsify",
        ),
        _candidate_from_monthly(
            family="t2_event_ledger",
            candidate="wf3_low_eff_low_atr_ctx4h_up_hard_combo",
            contract="next_adverse_xslip10bps",
            scope="ETHUSDT only",
            monthly=wf3_ctx4h_combo_monthly,
            baseline_sum_pct=lead_sum,
            trades=int(len(lead_trades) + len(wf3_ctx4h_extra)),
            source=str(WF3_CTX4H_UP_EXTRA_TRADES),
            read="hard 4h context leg, useful proxy for scaled context control",
        ),
        _candidate_from_monthly(
            family="t2_event_ledger",
            candidate="wf3_low_eff_low_atr_ctx4h_scaled025_combo",
            contract="next_adverse_xslip10bps",
            scope="ETHUSDT only",
            monthly=wf3_ctx4h_scaled_combo_monthly,
            baseline_sum_pct=lead_sum,
            trades=int(len(wf3_ctx4h_scaled_combo)),
            source=str(SCALED_CTX4H_EXTRA_TRADES),
            read="derived per-trade scaled ledger; best current T2 risk-control leg",
        ),
        _candidate_from_monthly(
            family="t3_strict_lifecycle",
            candidate="t3_min_hold_sl_60m_t3_split",
            contract="strict_next_second_cross lifecycle",
            scope="ETHUSDT/BTCUSDT T3 split",
            monthly=t3_monthly,
            baseline_sum_pct=None,
            trades=t3_trades,
            source=t3_source,
            read="watch-only T3 risk-shaping leg; not additive to T2 event ledger yet",
        ),
    ]
    month_rows: list[BridgeMonthRow] = []
    for month in months:
        t2_combo = low_eff_combo_monthly[month]
        t3_value = round(float(t3_monthly.get(month, 0.0)), 6)
        if t2_combo < 0.0 and t3_value > 0.0:
            read = "T3 split offsets a weak T2 month, but contracts differ"
        elif t2_combo > 0.0 and t3_value < 0.0:
            read = "T3 split weakens a positive T2 month under strict lifecycle"
        elif t2_combo > 0.0 and t3_value > 0.0:
            read = "both positive; bridge replay should test coexistence"
        elif t2_combo < 0.0 and t3_value < 0.0:
            read = "both weak; residual-risk month"
        else:
            read = "flat/missing on one side"
        month_rows.append(
            BridgeMonthRow(
                month=month,
                t2_lead_adverse10_pct=lead_monthly[month],
                t2_low_eff_rf_median_combo_pct=t2_combo,
                t2_low_eff_rf_median_extra_pct=low_eff_extra_monthly[month],
                t2_ctx4h_scaled025_combo_pct=wf3_ctx4h_scaled_combo_monthly[month],
                t2_ctx4h_scaled025_extra_pct=wf3_ctx4h_scaled_extra_monthly[month],
                t3_min_hold_60m_t3_pnl_pct=t3_value,
                bridge_read=read,
            )
        )
    return candidates, month_rows


def _markdown_table(rows: list[list[Any]], headers: list[str]) -> str:
    lines = [
        "| " + " | ".join(headers) + " |",
        "| " + " | ".join(["---"] * len(headers)) + " |",
    ]
    for row in rows:
        lines.append("| " + " | ".join(str(value) for value in row) + " |")
    return "\n".join(lines)


def _fmt(value: float | None) -> str:
    if value is None:
        return "n/a"
    return f"{value:.6f}%"


def write_outputs(output_dir: Path, candidates: list[BridgeCandidate], months: list[BridgeMonthRow]) -> None:
    output_dir.mkdir(parents=True, exist_ok=True)
    lead_active_months = [
        row.month for row in months
        if row.t2_lead_adverse10_pct != 0.0
    ]
    payload = {
        "note": "Research-only bridge. T2 adverse10 event-ledger and T3 strict lifecycle metrics are normalized to percent but not added.",
        "caveats": {
            "t2_lead_trade_ledger_active_months": lead_active_months,
            "t2_lead_trade_ledger_note": (
                "The lead adverse10 per-trade artifact used here only contains active canonical lead rows "
                "for these months. Forward retrain canonical metrics are covered by the merge matrix, but "
                "not by this per-month bridge ledger."
            ),
        },
        "candidates": [asdict(row) for row in candidates],
        "monthly_bridge": [asdict(row) for row in months],
    }
    (output_dir / "t2_t3_bridge_summary.json").write_text(
        json.dumps(payload, indent=2, ensure_ascii=False),
        encoding="utf-8",
    )

    candidate_rows = [
        [
            row.family,
            f"`{row.candidate}`",
            row.contract,
            row.scope,
            _fmt(row.calendar_sum_pct),
            _fmt(row.delta_vs_baseline_pct),
            _fmt(row.worst_month_pct),
            row.negative_months,
            row.trades if row.trades is not None else "n/a",
            row.read,
        ]
        for row in candidates
    ]
    month_rows = [
        [
            row.month,
            _fmt(row.t2_lead_adverse10_pct),
            _fmt(row.t2_low_eff_rf_median_combo_pct),
            _fmt(row.t2_low_eff_rf_median_extra_pct),
            _fmt(row.t2_ctx4h_scaled025_combo_pct),
            _fmt(row.t2_ctx4h_scaled025_extra_pct),
            _fmt(row.t3_min_hold_60m_t3_pnl_pct),
            row.bridge_read,
        ]
        for row in months
    ]
    lines = [
        "# T2/T3 Bridge Runner - 2026-05-18",
        "",
        "Scope: research-only. This bridge report normalizes current T2 adverse10 event-ledger results and strict T3 lifecycle split results into percent units on the same month axis. It does not claim the two contracts are additive.",
        "",
        "## Candidate Summary",
        "",
        _markdown_table(
            candidate_rows,
            [
                "Family",
                "Candidate",
                "Contract",
                "Scope",
                "Calendar Sum",
                "Delta",
                "Worst Month",
                "Neg Months",
                "Trades",
                "Read",
            ],
        ),
        "",
        "## Monthly Bridge",
        "",
        _markdown_table(
            month_rows,
            [
                "Month",
                "T2 Lead Adverse10",
                "T2 Low-Eff RF Combo",
                "T2 Extra",
                "T2 Ctx4h Scaled",
                "Ctx4h Extra",
                "T3 60m Strict Split",
                "Read",
            ],
        ),
        "",
        "## Data Caveat",
        "",
        "- The T2 lead adverse10 per-trade artifact used by this bridge has active canonical lead rows in: "
        + (", ".join(lead_active_months) if lead_active_months else "none")
        + ". Later months are zero in this bridge table because the current per-trade source is sparse, not because the retrain-forward lead has no value.",
        "- Use `t2_t3_merge_matrix_report.md` for the production8 retrain-forward summary; use this bridge for month-axis comparison of currently available trade ledgers.",
        "",
        "## Generated Ledgers",
        "",
        f"- Scaled ctx4h extra ledger: `{SCALED_CTX4H_EXTRA_TRADES}`",
        f"- Scaled ctx4h combo ledger: `{SCALED_CTX4H_COMBO_TRADES}`",
        "",
        "## Decision",
        "",
        "- `low_eff_rf_rank_median_000_combo` remains the cleanest T2 additive leg to falsify under adverse10.",
        "- `wf3_low_eff_low_atr_ctx4h_scaled025_combo` now has a generated per-month scaled ledger and is the cleaner T2 risk-control leg versus hard ctx4h.",
        "- `t3_min_hold_sl_60m_t3_split` is a watch-only strict lifecycle leg; its positive split cannot be added to T2 adverse10 without a unified lifecycle bridge.",
        "- Next implementation target: refresh T3 strict JSON from a long replay and then build a true unified lifecycle bridge.",
        "",
    ]
    (output_dir / "t2_t3_bridge_report.md").write_text("\n".join(lines), encoding="utf-8")


def run(output_dir: Path, months: list[str]) -> None:
    candidates, month_rows = build_bridge(months)
    write_outputs(output_dir, candidates, month_rows)


def main() -> None:
    parser = argparse.ArgumentParser(description="Build T2/T3 research bridge report")
    parser.add_argument("--output-dir", default=str(OUTPUT_DIR))
    parser.add_argument("--months", nargs="+", default=DEFAULT_MONTHS)
    args = parser.parse_args()
    run(Path(args.output_dir), [str(value) for value in args.months])
    print(f"Report: {Path(args.output_dir) / 't2_t3_bridge_report.md'}")


if __name__ == "__main__":
    main()
