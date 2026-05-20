"""Research-only sizing audit for the ETH T2/lead quantity band.

This report keeps the current canonical ETH pretouch lead events, timing
predictions, and adverse10 fill ledger fixed. It changes only the submitted
lead quantity to mirror the testnet-shadow live sizing contract:

``production_quantity = pretouchBaseOrderQuantity * position_size``
``quality_score = production_quantity / max_production_quantity``
``submitted_quantity = min_qty + quality_score * (max_qty - min_qty)``

The result is a linear notional replay. It is not a new entry rule and it does
not model additional market impact beyond the existing adverse10 fill ledger.
"""

from __future__ import annotations

import argparse
import json
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Any

import numpy as np
import pandas as pd

PROJECT_ROOT = Path(__file__).resolve().parents[4]
OUTPUT_DIR = (
    PROJECT_ROOT
    / "research"
    / "entry_redesign"
    / "scripts"
    / "output"
    / "timing_probability_unified"
)

DEFAULT_OUTPUT = OUTPUT_DIR / "t2_lead_quantity_band_sizing_20260520"
DEFAULT_LEAD_ADVERSE_TRADES = OUTPUT_DIR / "breakout_structure_lead_combo_lead_adverse10_trades.csv"
DEFAULT_LEAD_SAME_TRADES = OUTPUT_DIR / "breakout_structure_lead_combo_lead_same_close_trades.csv"
DEFAULT_T3_SUMMARY = OUTPUT_DIR / "t3_overlay_rf_cost_sizing_20260520" / "t3_overlay_rf_cost_sizing_summary.json"
DEFAULT_T3_WEIGHTED_TRADES = OUTPUT_DIR / "t3_overlay_rf_cost_sizing_20260520" / "t3_overlay_rf_cost_weighted_trades.csv"
T3_QUANTITY_BAND_VARIANT = "wf_t3_rf_cost_quantity_0p20_0p40_shadow"


def _path_label(path: Path | str) -> str:
    resolved = Path(path)
    try:
        return str(resolved.resolve().relative_to(PROJECT_ROOT))
    except (OSError, ValueError):
        return str(resolved)


def _numeric(frame: pd.DataFrame, column: str, default: float = 0.0) -> pd.Series:
    if column not in frame.columns:
        return pd.Series(default, index=frame.index, dtype=float)
    return pd.to_numeric(frame[column], errors="coerce").fillna(default).astype(float)


def _equity_max_drawdown_pct(values_pct: pd.Series) -> float:
    if values_pct.empty:
        return 0.0
    curve = pd.concat([pd.Series([0.0]), values_pct.astype(float).cumsum().reset_index(drop=True)], ignore_index=True)
    drawdown = curve - curve.cummax()
    return round(float(drawdown.min()), 6)


def _month_grid(months: list[str], values: pd.Series) -> dict[str, float]:
    return {month: round(float(values.get(month, 0.0)), 6) for month in months}


def _normalize_band(min_quantity: float, max_quantity: float, max_submitted_quantity: float) -> tuple[float, float]:
    min_q = min(float(min_quantity), float(max_submitted_quantity))
    max_q = min(float(max_quantity), float(max_submitted_quantity))
    if min_q <= 0:
        min_q = max_q
    if max_q <= 0:
        max_q = min_q
    if min_q > max_q:
        min_q = max_q
    return min_q, max_q


@dataclass(frozen=True)
class LeadQuantityMetrics:
    variant: str
    calendar_sum_pct: float
    delta_vs_base_pct: float
    delta_vs_legacy_1p5_pct: float
    worst_month_pct: float
    negative_months: int
    max_drawdown_pct: float
    trade_count: int
    avg_submitted_quantity: float
    median_submitted_quantity: float
    min_submitted_quantity: float
    max_submitted_quantity: float
    avg_quantity_multiplier: float
    median_quantity_multiplier: float
    max_quantity_multiplier: float
    avg_quality_score: float
    by_month: dict[str, float]


@dataclass(frozen=True)
class BundleMetrics:
    variant: str
    calendar_sum_pct: float
    delta_vs_base_lead_plus_t3_pct: float
    worst_month_pct: float
    negative_months: int
    max_drawdown_pct: float | None
    lead_calendar_sum_pct: float
    t3_calendar_sum_pct: float
    by_month: dict[str, float]


def score_lead_quantity_band(
    trades: pd.DataFrame,
    *,
    base_order_quantity: float,
    base_share: float,
    max_production_multiplier: float,
    min_quantity: float,
    max_quantity: float,
    max_submitted_quantity: float,
    legacy_scale: float,
) -> pd.DataFrame:
    """Attach live-compatible lead quantity-band sizing columns."""
    required = {"touch_time", "weighted_pnl", "position_size"}
    missing = sorted(required - set(trades.columns))
    if missing:
        raise ValueError(f"lead trades missing required columns: {', '.join(missing)}")

    scored = trades.copy()
    scored["touch_time"] = pd.to_datetime(scored["touch_time"], utc=True)
    position_size = _numeric(scored, "position_size")
    production_quantity = position_size * float(base_order_quantity)
    max_production_quantity = float(base_order_quantity) * float(base_share) * float(max_production_multiplier)
    quality_score = pd.Series(0.0, index=scored.index, dtype=float)
    if max_production_quantity > 0:
        quality_score = (production_quantity / max_production_quantity).clip(lower=0.0, upper=1.0)

    min_q, max_q = _normalize_band(min_quantity, max_quantity, max_submitted_quantity)
    submitted_quantity = min_q + quality_score * (max_q - min_q)
    submitted_quantity = submitted_quantity.clip(upper=float(max_submitted_quantity))

    quantity_multiplier = pd.Series(0.0, index=scored.index, dtype=float)
    positive = production_quantity > 0
    quantity_multiplier.loc[positive] = submitted_quantity.loc[positive] / production_quantity.loc[positive]

    legacy_quantity = (production_quantity * float(legacy_scale)).clip(upper=float(max_submitted_quantity))
    legacy_multiplier = pd.Series(0.0, index=scored.index, dtype=float)
    legacy_multiplier.loc[positive] = legacy_quantity.loc[positive] / production_quantity.loc[positive]

    base_weighted_pnl_pct = _numeric(scored, "weighted_pnl") * 100.0
    scored["production_quantity"] = production_quantity
    scored["lead_quantity_band_score"] = quality_score
    scored["lead_quantity_band_min_quantity"] = min_q
    scored["lead_quantity_band_max_quantity"] = max_q
    scored["submitted_quantity"] = submitted_quantity
    scored["quantity_multiplier"] = quantity_multiplier
    scored["base_weighted_pnl_pct"] = base_weighted_pnl_pct
    scored["quantity_band_weighted_pnl_pct"] = base_weighted_pnl_pct * quantity_multiplier
    scored["legacy_1p5_quantity"] = legacy_quantity
    scored["legacy_1p5_multiplier"] = legacy_multiplier
    scored["legacy_1p5_weighted_pnl_pct"] = base_weighted_pnl_pct * legacy_multiplier
    scored["year_month"] = scored["touch_time"].dt.strftime("%Y-%m")
    return scored


def summarize_lead(scored: pd.DataFrame, *, months: list[str]) -> tuple[LeadQuantityMetrics, dict[str, Any]]:
    active_monthly = scored.groupby("year_month")["quantity_band_weighted_pnl_pct"].sum()
    by_month = _month_grid(months, active_monthly)
    base_pct = round(float(scored["base_weighted_pnl_pct"].sum()), 6)
    legacy_pct = round(float(scored["legacy_1p5_weighted_pnl_pct"].sum()), 6)
    calendar_sum = round(float(sum(by_month.values())), 6)
    metrics = LeadQuantityMetrics(
        variant="lead_quantity_0p20_0p40_adverse10",
        calendar_sum_pct=calendar_sum,
        delta_vs_base_pct=round(calendar_sum - base_pct, 6),
        delta_vs_legacy_1p5_pct=round(calendar_sum - legacy_pct, 6),
        worst_month_pct=round(float(active_monthly.min()), 6) if len(active_monthly) else 0.0,
        negative_months=int((active_monthly < 0.0).sum()),
        max_drawdown_pct=_equity_max_drawdown_pct(scored.sort_values("touch_time")["quantity_band_weighted_pnl_pct"]),
        trade_count=int(len(scored)),
        avg_submitted_quantity=round(float(scored["submitted_quantity"].mean()), 6),
        median_submitted_quantity=round(float(scored["submitted_quantity"].median()), 6),
        min_submitted_quantity=round(float(scored["submitted_quantity"].min()), 6),
        max_submitted_quantity=round(float(scored["submitted_quantity"].max()), 6),
        avg_quantity_multiplier=round(float(scored["quantity_multiplier"].mean()), 6),
        median_quantity_multiplier=round(float(scored["quantity_multiplier"].median()), 6),
        max_quantity_multiplier=round(float(scored["quantity_multiplier"].max()), 6),
        avg_quality_score=round(float(scored["lead_quantity_band_score"].mean()), 6),
        by_month=by_month,
    )
    baseline = {
        "base_lead_adverse10_pct": base_pct,
        "legacy_lead_1p5_adverse10_pct": legacy_pct,
        "legacy_lead_1p5_delta_vs_base_pct": round(legacy_pct - base_pct, 6),
    }
    return metrics, baseline


def load_t3_quantity_band_metrics(summary_path: Path) -> dict[str, Any] | None:
    if not summary_path.exists():
        return None
    payload = json.loads(summary_path.read_text(encoding="utf-8"))
    for row in payload.get("metrics", []):
        if row.get("variant") == T3_QUANTITY_BAND_VARIANT:
            return row
    return None


def load_t3_quantity_band_trades(weighted_trades_path: Path) -> pd.DataFrame:
    if not weighted_trades_path.exists():
        return pd.DataFrame()
    trades = pd.read_csv(weighted_trades_path)
    if "sizing_variant" not in trades.columns:
        return pd.DataFrame()
    trades = trades[trades["sizing_variant"] == T3_QUANTITY_BAND_VARIANT].copy()
    if trades.empty:
        return trades
    trades["event_time"] = pd.to_datetime(trades.get("exit_time", trades.get("entry_time")), utc=True)
    trades["year_month"] = trades["month"].astype(str)
    trades["combo_pnl_pct"] = _numeric(trades, "weighted_pnl_pct")
    return trades


def summarize_bundle(
    *,
    lead_scored: pd.DataFrame,
    lead_metrics: LeadQuantityMetrics,
    base_lead_pct: float,
    t3_metrics: dict[str, Any] | None,
    t3_trades: pd.DataFrame,
    months: list[str],
) -> BundleMetrics | None:
    if t3_metrics is None:
        return None
    t3_by_month = {str(month): float(value) for month, value in (t3_metrics.get("by_month") or {}).items()}
    lead_by_month = lead_metrics.by_month
    by_month = {
        month: round(float(lead_by_month.get(month, 0.0)) + float(t3_by_month.get(month, 0.0)), 6)
        for month in months
    }
    t3_sum = round(float(t3_metrics.get("calendar_sum_pct", sum(t3_by_month.values()))), 6)
    base_plus_t3 = round(float(base_lead_pct) + t3_sum, 6)
    calendar_sum = round(float(sum(by_month.values())), 6)

    max_dd: float | None = None
    if not t3_trades.empty:
        lead_events = pd.DataFrame(
            {
                "event_time": lead_scored["touch_time"],
                "combo_pnl_pct": lead_scored["quantity_band_weighted_pnl_pct"],
            }
        )
        combo_trades = pd.concat(
            [lead_events, t3_trades[["event_time", "combo_pnl_pct"]]],
            ignore_index=True,
        ).sort_values("event_time")
        max_dd = _equity_max_drawdown_pct(combo_trades["combo_pnl_pct"])

    return BundleMetrics(
        variant="lead_q020_q040_plus_t3_q020_q040",
        calendar_sum_pct=calendar_sum,
        delta_vs_base_lead_plus_t3_pct=round(calendar_sum - base_plus_t3, 6),
        worst_month_pct=round(float(min(by_month.values(), default=0.0)), 6),
        negative_months=int(sum(1 for value in by_month.values() if value < 0.0)),
        max_drawdown_pct=max_dd,
        lead_calendar_sum_pct=lead_metrics.calendar_sum_pct,
        t3_calendar_sum_pct=t3_sum,
        by_month=by_month,
    )


def _markdown_month_table(
    *,
    months: list[str],
    base_monthly: dict[str, float],
    lead_monthly: dict[str, float],
    t3_monthly: dict[str, float],
    combo_monthly: dict[str, float],
) -> str:
    lines = [
        "| Month | Base Lead | Lead Qty Band | T3 Qty Band | Bundle |",
        "|---|---:|---:|---:|---:|",
    ]
    for month in months:
        lines.append(
            f"| {month} | {base_monthly.get(month, 0.0):.6f}% "
            f"| {lead_monthly.get(month, 0.0):.6f}% "
            f"| {t3_monthly.get(month, 0.0):.6f}% "
            f"| {combo_monthly.get(month, 0.0):.6f}% |"
        )
    return "\n".join(lines)


def write_outputs(
    *,
    output_dir: Path,
    lead_scored: pd.DataFrame,
    lead_metrics: LeadQuantityMetrics,
    baseline: dict[str, Any],
    bundle_metrics: BundleMetrics | None,
    t3_metrics: dict[str, Any] | None,
    lead_adverse_trades: Path,
    lead_same_trades: Path | None,
    t3_summary: Path,
    t3_weighted_trades: Path,
    params: dict[str, Any],
) -> None:
    output_dir.mkdir(parents=True, exist_ok=True)
    lead_scored.to_csv(output_dir / "t2_lead_quantity_band_scored_trades.csv", index=False)

    months = params["months"]
    base_monthly = _month_grid(months, lead_scored.groupby("year_month")["base_weighted_pnl_pct"].sum())
    active_base_monthly = lead_scored.groupby("year_month")["base_weighted_pnl_pct"].sum()
    lead_monthly = lead_metrics.by_month
    t3_monthly = {str(month): float(value) for month, value in ((t3_metrics or {}).get("by_month") or {}).items()}
    combo_monthly = bundle_metrics.by_month if bundle_metrics is not None else {
        month: lead_monthly.get(month, 0.0) for month in months
    }

    payload = {
        "note": (
            "Research-only T2/lead quantity-band sizing audit. The canonical lead adverse10 ledger "
            "is fixed; this changes only submitted quantity using the live testnet-shadow formula."
        ),
        "lead_adverse_trades": _path_label(lead_adverse_trades),
        "lead_same_trades": _path_label(lead_same_trades) if lead_same_trades is not None else None,
        "t3_summary": _path_label(t3_summary),
        "t3_weighted_trades": _path_label(t3_weighted_trades),
        "params": params,
        "baseline": baseline,
        "lead_quantity_band": asdict(lead_metrics),
        "t3_quantity_band": t3_metrics,
        "bundle": asdict(bundle_metrics) if bundle_metrics is not None else None,
    }
    (output_dir / "t2_lead_quantity_band_summary.json").write_text(
        json.dumps(payload, indent=2, ensure_ascii=False, default=str) + "\n",
        encoding="utf-8",
    )

    lines = [
        "# T2 Lead Quantity-Band Sizing",
        "",
        "Research-only audit for applying the testnet-shadow `0.20..0.40 ETH` lead quantity band to the current ETH pretouch lead.",
        "",
        "## Inputs",
        "",
        f"- Lead adverse10 trades: `{_path_label(lead_adverse_trades)}`",
        f"- Lead same-close trades: `{_path_label(lead_same_trades) if lead_same_trades is not None else 'not provided'}`",
        f"- T3 RF/cost summary: `{_path_label(t3_summary)}`",
        f"- T3 weighted trades: `{_path_label(t3_weighted_trades)}`",
        "",
        "## Formula",
        "",
        f"- `base_order_quantity={params['base_order_quantity']}`",
        f"- `base_share={params['base_share']}` and `max_production_multiplier={params['max_production_multiplier']}`",
        f"- `production_quantity = base_order_quantity * position_size`",
        f"- `quality_score = clip(production_quantity / (base_order_quantity * base_share * max_production_multiplier), 0, 1)`",
        f"- `submitted_quantity = {params['min_quantity']} + quality_score * ({params['max_quantity']} - {params['min_quantity']})`, capped by `max_submitted_quantity={params['max_submitted_quantity']}`",
        f"- PnL is linearly rescaled from the existing adverse10 ledger by `submitted_quantity / production_quantity`.",
        "",
        "## Summary",
        "",
        "| Variant | Calendar Sum | Delta vs Base | Delta vs Legacy 1.5x | Worst Month | Neg Months | DD | Trades | Avg Qty | Max Qty | Avg Mult |",
        "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|",
        f"| `base_lead_adverse10` | {baseline['base_lead_adverse10_pct']:.6f}% | 0.000000pp | - | {float(active_base_monthly.min()) if len(active_base_monthly) else 0.0:.6f}% | {int((active_base_monthly < 0).sum())} | - | {lead_metrics.trade_count} | - | - | - |",
        f"| `legacy_lead_1p5_adverse10` | {baseline['legacy_lead_1p5_adverse10_pct']:.6f}% | {baseline['legacy_lead_1p5_delta_vs_base_pct']:.6f}pp | 0.000000pp | - | - | - | {lead_metrics.trade_count} | - | - | 1.500000 |",
        f"| `{lead_metrics.variant}` | {lead_metrics.calendar_sum_pct:.6f}% | {lead_metrics.delta_vs_base_pct:.6f}pp | {lead_metrics.delta_vs_legacy_1p5_pct:.6f}pp | {lead_metrics.worst_month_pct:.6f}% | {lead_metrics.negative_months} | {lead_metrics.max_drawdown_pct:.6f}% | {lead_metrics.trade_count} | {lead_metrics.avg_submitted_quantity:.6f} | {lead_metrics.max_submitted_quantity:.6f} | {lead_metrics.avg_quantity_multiplier:.6f} |",
    ]
    if bundle_metrics is not None:
        dd = "-" if bundle_metrics.max_drawdown_pct is None else f"{bundle_metrics.max_drawdown_pct:.6f}%"
        lines.append(
            f"| `{bundle_metrics.variant}` | {bundle_metrics.calendar_sum_pct:.6f}% | "
            f"{bundle_metrics.delta_vs_base_lead_plus_t3_pct:.6f}pp vs base lead + T3 | - | "
            f"{bundle_metrics.worst_month_pct:.6f}% | {bundle_metrics.negative_months} | {dd} | - | - | - | - |"
        )
    lines.extend(
        [
            "",
            "## Monthly",
            "",
            _markdown_month_table(
                months=months,
                base_monthly=base_monthly,
                lead_monthly=lead_monthly,
                t3_monthly=t3_monthly,
                combo_monthly=combo_monthly,
            ),
            "",
            "## Read",
            "",
            "- The T2 quantity-band result is a formal linear-notional backtest view of the testnet-shadow sizing contract, not a mainnet promotion result.",
            "- The base event set, timing decisions, exit contract, and adverse10 fill ledger are unchanged.",
            "- `legacy_lead_1p5_adverse10` is retained only as a continuity reference for the previous shadow sizing.",
            "- The bundle row adds T2 quantity-band lead PnL and T3 RF/cost quantity-band overlay PnL month-by-month. It does not yet model additional slippage or exchange depth degradation from larger submitted quantity.",
        ]
    )
    (output_dir / "t2_lead_quantity_band_report.md").write_text("\n".join(lines) + "\n", encoding="utf-8")


def run(
    *,
    output_dir: Path,
    lead_adverse_trades: Path,
    lead_same_trades: Path | None,
    t3_summary: Path,
    t3_weighted_trades: Path,
    months: list[str] | None,
    base_order_quantity: float,
    base_share: float,
    max_production_multiplier: float,
    min_quantity: float,
    max_quantity: float,
    max_submitted_quantity: float,
    legacy_scale: float,
) -> dict[str, Any]:
    lead_trades = pd.read_csv(lead_adverse_trades)
    scored = score_lead_quantity_band(
        lead_trades,
        base_order_quantity=base_order_quantity,
        base_share=base_share,
        max_production_multiplier=max_production_multiplier,
        min_quantity=min_quantity,
        max_quantity=max_quantity,
        max_submitted_quantity=max_submitted_quantity,
        legacy_scale=legacy_scale,
    )
    if months is None:
        month_set = set(scored["year_month"].astype(str))
        t3_metrics_for_months = load_t3_quantity_band_metrics(t3_summary)
        month_set.update(str(month) for month in ((t3_metrics_for_months or {}).get("by_month") or {}).keys())
        months = sorted(month_set)
        t3_metrics = t3_metrics_for_months
    else:
        t3_metrics = load_t3_quantity_band_metrics(t3_summary)

    lead_metrics, baseline = summarize_lead(scored, months=months)
    t3_trades = load_t3_quantity_band_trades(t3_weighted_trades)
    bundle_metrics = summarize_bundle(
        lead_scored=scored,
        lead_metrics=lead_metrics,
        base_lead_pct=float(baseline["base_lead_adverse10_pct"]),
        t3_metrics=t3_metrics,
        t3_trades=t3_trades,
        months=months,
    )
    params = {
        "months": months,
        "base_order_quantity": float(base_order_quantity),
        "base_share": float(base_share),
        "max_production_multiplier": float(max_production_multiplier),
        "min_quantity": float(min_quantity),
        "max_quantity": float(max_quantity),
        "max_submitted_quantity": float(max_submitted_quantity),
        "legacy_scale": float(legacy_scale),
    }
    write_outputs(
        output_dir=output_dir,
        lead_scored=scored,
        lead_metrics=lead_metrics,
        baseline=baseline,
        bundle_metrics=bundle_metrics,
        t3_metrics=t3_metrics,
        lead_adverse_trades=lead_adverse_trades,
        lead_same_trades=lead_same_trades,
        t3_summary=t3_summary,
        t3_weighted_trades=t3_weighted_trades,
        params=params,
    )
    return {
        "lead_quantity_band": asdict(lead_metrics),
        "baseline": baseline,
        "bundle": asdict(bundle_metrics) if bundle_metrics is not None else None,
        "output_dir": str(output_dir),
    }


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output-dir", type=Path, default=DEFAULT_OUTPUT)
    parser.add_argument("--lead-adverse-trades", type=Path, default=DEFAULT_LEAD_ADVERSE_TRADES)
    parser.add_argument("--lead-same-trades", type=Path, default=DEFAULT_LEAD_SAME_TRADES)
    parser.add_argument("--t3-summary", type=Path, default=DEFAULT_T3_SUMMARY)
    parser.add_argument("--t3-weighted-trades", type=Path, default=DEFAULT_T3_WEIGHTED_TRADES)
    parser.add_argument("--months", nargs="+", default=None)
    parser.add_argument("--base-order-quantity", type=float, default=0.10)
    parser.add_argument("--base-share", type=float, default=0.80)
    parser.add_argument("--max-production-multiplier", type=float, default=2.0)
    parser.add_argument("--min-quantity", type=float, default=0.20)
    parser.add_argument("--max-quantity", type=float, default=0.40)
    parser.add_argument("--max-submitted-quantity", type=float, default=0.40)
    parser.add_argument("--legacy-scale", type=float, default=1.5)
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    payload = run(
        output_dir=Path(args.output_dir),
        lead_adverse_trades=Path(args.lead_adverse_trades),
        lead_same_trades=Path(args.lead_same_trades) if args.lead_same_trades is not None else None,
        t3_summary=Path(args.t3_summary),
        t3_weighted_trades=Path(args.t3_weighted_trades),
        months=[str(month) for month in args.months] if args.months is not None else None,
        base_order_quantity=float(args.base_order_quantity),
        base_share=float(args.base_share),
        max_production_multiplier=float(args.max_production_multiplier),
        min_quantity=float(args.min_quantity),
        max_quantity=float(args.max_quantity),
        max_submitted_quantity=float(args.max_submitted_quantity),
        legacy_scale=float(args.legacy_scale),
    )
    lead = payload["lead_quantity_band"]
    bundle = payload.get("bundle") or {}
    print(
        "lead_quantity_band="
        f"{float(lead['calendar_sum_pct']):.6f}% "
        f"delta_vs_base={float(lead['delta_vs_base_pct']):.6f}pp "
        f"bundle={float(bundle.get('calendar_sum_pct', 0.0)):.6f}%"
    )
    print(f"wrote={payload['output_dir']}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
