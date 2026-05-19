"""Bridge T3 direct-entry findings back onto the ETH pretouch research lead.

Research-only. This is an accounting bridge, not a live runner. It keeps the
canonical ETH pretouch lead ledger intact, converts the T3 lifecycle monthly
returns into the same fractional return convention, and reports additive
lead-vs-overlay-vs-combo metrics on the fixed calendar. The purpose is to test
whether the T3 probability/quality lessons are useful as a lead enhancement
rather than as a separate replacement strategy.
"""

from __future__ import annotations

import argparse
import json
from pathlib import Path
from typing import Any

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
DEFAULT_LEAD_ADVERSE_TRADES = OUTPUT_DIR / "breakout_structure_lead_combo_lead_adverse10_trades.csv"
DEFAULT_LEAD_SAME_TRADES = OUTPUT_DIR / "breakout_structure_lead_combo_lead_same_close_trades.csv"
DEFAULT_T3_SUMMARY = (
    OUTPUT_DIR
    / "t3_filtered_external_all_speed_ge035_eth_next_adverse_size2_extended"
    / "t3_filtered_external_event_lifecycle_summary.json"
)
DEFAULT_OUTPUT = OUTPUT_DIR / "t3_overlay_lead_bridge"


def _load_trades(path: Path) -> pd.DataFrame:
    if not path.exists():
        raise FileNotFoundError(path)
    trades = pd.read_csv(path)
    trades["touch_time"] = pd.to_datetime(trades["touch_time"], utc=True)
    trades["month"] = trades["touch_time"].dt.strftime("%Y-%m")
    trades["weighted_pnl"] = pd.to_numeric(trades["weighted_pnl"], errors="coerce").fillna(0.0)
    return trades


def _lead_monthly(trades: pd.DataFrame, months: list[str]) -> pd.Series:
    monthly = trades.groupby("month")["weighted_pnl"].sum()
    return pd.Series({month: float(monthly.get(month, 0.0)) for month in months}, dtype="float64")


def _load_t3_summary(path: Path) -> dict[str, Any]:
    if not path.exists():
        raise FileNotFoundError(path)
    return json.loads(path.read_text(encoding="utf-8"))


def _t3_monthly(summary: dict[str, Any], months: list[str]) -> pd.Series:
    candidates = summary.get("candidates") or []
    if not candidates:
        return pd.Series({month: 0.0 for month in months}, dtype="float64")
    by_month = candidates[0].get("by_month", {})
    # T3 lifecycle reports percent returns; lead ledgers use fractional returns.
    return pd.Series({month: float(by_month.get(month, 0.0)) / 100.0 for month in months}, dtype="float64")


def _metrics(label: str, monthly: pd.Series, trade_count: int) -> dict[str, Any]:
    return {
        "variant": label,
        "calendar_sum": round(float(monthly.sum()), 9),
        "calendar_sum_pct": round(float(monthly.sum()) * 100.0, 6),
        "worst_month": round(float(monthly.min()), 9) if len(monthly) else 0.0,
        "worst_month_pct": round(float(monthly.min()) * 100.0, 6) if len(monthly) else 0.0,
        "negative_months": int((monthly < 0.0).sum()),
        "trade_count": int(trade_count),
        "by_month": {month: round(float(value), 9) for month, value in monthly.items()},
        "by_month_pct": {month: round(float(value) * 100.0, 6) for month, value in monthly.items()},
    }


def _markdown_table(rows: list[dict[str, Any]]) -> str:
    cols = [
        "variant",
        "calendar_sum_pct",
        "worst_month_pct",
        "negative_months",
        "trade_count",
    ]
    lines = [
        "| Variant | Calendar Sum | Worst Month | Neg Months | Trades |",
        "|---|---:|---:|---:|---:|",
    ]
    for row in rows:
        lines.append(
            f"| `{row['variant']}` | {row['calendar_sum_pct']:.6f}% | "
            f"{row['worst_month_pct']:.6f}% | {int(row['negative_months'])} | {int(row['trade_count'])} |"
        )
    return "\n".join(lines)


def _write_outputs(
    *,
    output_dir: Path,
    lead_adverse_path: Path,
    lead_same_path: Path,
    t3_summary_path: Path,
    lead_adverse: pd.Series,
    lead_same: pd.Series,
    t3_overlay: pd.Series,
    rows: list[dict[str, Any]],
    t3_summary: dict[str, Any],
) -> None:
    output_dir.mkdir(parents=True, exist_ok=True)
    pd.DataFrame(rows).to_csv(output_dir / "t3_overlay_lead_bridge_summary.csv", index=False)
    monthly = pd.DataFrame(
        {
            "month": lead_adverse.index,
            "lead_adverse": lead_adverse.values,
            "lead_same": lead_same.values,
            "t3_overlay": t3_overlay.values,
            "lead_plus_t3_adverse": (lead_adverse + t3_overlay).values,
            "lead_plus_t3_same": (lead_same + t3_overlay).values,
        }
    )
    for column in monthly.columns:
        if column != "month":
            monthly[f"{column}_pct"] = monthly[column] * 100.0
    monthly.to_csv(output_dir / "t3_overlay_lead_bridge_monthly.csv", index=False)

    payload = {
        "note": (
            "Research-only additive accounting bridge. Lead legs use weighted_pnl fractional returns; "
            "T3 lifecycle percent returns are converted to fractional returns before combining. This does "
            "not model portfolio exposure collisions or live execution semantics."
        ),
        "lead_adverse_trades": str(lead_adverse_path),
        "lead_same_trades": str(lead_same_path),
        "t3_summary": str(t3_summary_path),
        "t3_entry_mode": t3_summary.get("external_entry_mode"),
        "t3_size_scale": t3_summary.get("t3_size_scale"),
        "t3_reentry_size_schedule": t3_summary.get("t3_reentry_size_schedule"),
        "rows": rows,
    }
    (output_dir / "t3_overlay_lead_bridge_summary.json").write_text(
        json.dumps(payload, indent=2, ensure_ascii=False) + "\n",
        encoding="utf-8",
    )

    lines = [
        "# T3 Overlay Lead Bridge",
        "",
        "Research-only additive bridge from the T3 direct-entry findings back to the ETH pretouch research lead.",
        "",
        f"- Lead adverse ledger: `{lead_adverse_path}`",
        f"- Lead same-close ledger: `{lead_same_path}`",
        f"- T3 lifecycle summary: `{t3_summary_path}`",
        f"- T3 entry mode: `{t3_summary.get('external_entry_mode')}`",
        f"- T3 size scale: `{t3_summary.get('t3_size_scale')}`",
        f"- T3 schedule: `{json.dumps(t3_summary.get('t3_reentry_size_schedule'))}`",
        "",
        "## Summary",
        "",
        _markdown_table(rows),
        "",
        "## Read",
        "",
        "- This supports using T3 quality/direct-entry lessons as a lead enhancement layer, not as a replacement strategy.",
        "- The combined rows are additive fixed-calendar accounting; promotion still requires exposure, drawdown, final-mark and slippage stress.",
        "- The current overlay is ETH-only; BTC stayed negative in direct T3 tests and is intentionally excluded here.",
    ]
    (output_dir / "t3_overlay_lead_bridge_report.md").write_text("\n".join(lines) + "\n", encoding="utf-8")


def run(
    *,
    lead_adverse_trades: Path,
    lead_same_trades: Path,
    t3_summary_path: Path,
    output_dir: Path,
) -> list[dict[str, Any]]:
    lead_adverse_trades_df = _load_trades(lead_adverse_trades)
    lead_same_trades_df = _load_trades(lead_same_trades)
    t3_summary = _load_t3_summary(t3_summary_path)
    months = list(t3_summary.get("calendar_grid", {}).get("months") or sorted(lead_adverse_trades_df["month"].unique()))
    lead_adverse = _lead_monthly(lead_adverse_trades_df, months)
    lead_same = _lead_monthly(lead_same_trades_df, months)
    t3_overlay = _t3_monthly(t3_summary, months)

    rows = [
        _metrics("lead_same_close", lead_same, len(lead_same_trades_df)),
        _metrics("lead_adverse10", lead_adverse, len(lead_adverse_trades_df)),
        _metrics("t3_overlay_eth_adverse_size2", t3_overlay, int((t3_summary.get("candidates") or [{}])[0].get("total_trades", 0))),
        _metrics(
            "lead_same_close_plus_t3_overlay",
            lead_same + t3_overlay,
            len(lead_same_trades_df) + int((t3_summary.get("candidates") or [{}])[0].get("total_trades", 0)),
        ),
        _metrics(
            "lead_adverse10_plus_t3_overlay",
            lead_adverse + t3_overlay,
            len(lead_adverse_trades_df) + int((t3_summary.get("candidates") or [{}])[0].get("total_trades", 0)),
        ),
    ]
    _write_outputs(
        output_dir=output_dir,
        lead_adverse_path=lead_adverse_trades,
        lead_same_path=lead_same_trades,
        t3_summary_path=t3_summary_path,
        lead_adverse=lead_adverse,
        lead_same=lead_same,
        t3_overlay=t3_overlay,
        rows=rows,
        t3_summary=t3_summary,
    )
    return rows


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--lead-adverse-trades", type=Path, default=DEFAULT_LEAD_ADVERSE_TRADES)
    parser.add_argument("--lead-same-trades", type=Path, default=DEFAULT_LEAD_SAME_TRADES)
    parser.add_argument("--t3-summary", type=Path, default=DEFAULT_T3_SUMMARY)
    parser.add_argument("--output-dir", type=Path, default=DEFAULT_OUTPUT)
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    rows = run(
        lead_adverse_trades=Path(args.lead_adverse_trades),
        lead_same_trades=Path(args.lead_same_trades),
        t3_summary_path=Path(args.t3_summary),
        output_dir=Path(args.output_dir),
    )
    best = max(rows, key=lambda row: row["calendar_sum"])
    print(f"best={best['variant']} calendar_sum={best['calendar_sum_pct']:.6f}% trades={best['trade_count']}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
