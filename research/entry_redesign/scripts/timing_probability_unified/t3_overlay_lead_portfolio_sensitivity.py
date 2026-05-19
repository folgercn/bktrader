"""Portfolio-level sensitivity for the ETH T3 overlay lead enhancement.

Research-only. This script consumes lead/overlay exposure windows, then
simulates a simple notional-cap allocator so the lead and overlay cannot
silently double-use the same capacity. It also applies additional round-trip
slippage to the T3 overlay after the next-second adverse lifecycle accounting.

Lead windows may be approximate or exact depending on the input file. Treat this
as a research risk haircut, not a live allocator.
"""

from __future__ import annotations

import argparse
import json
from dataclasses import asdict, dataclass
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
DEFAULT_EXPOSURE_DIR = OUTPUT_DIR / "t3_overlay_lead_exposure_audit"
DEFAULT_OUTPUT = OUTPUT_DIR / "t3_overlay_lead_portfolio_sensitivity"


@dataclass(frozen=True)
class PortfolioScenario:
    """One allocator/slippage stress result."""

    policy: str
    capital_capacity: float
    overlay_extra_round_trip_slippage_bps: float
    requested_trades: int
    filled_trades: int
    skipped_trades: int
    scaled_trades: int
    requested_notional_share: float
    allocated_notional_share: float
    allocation_ratio: float
    peak_active_notional_share: float
    calendar_sum_pct: float
    worst_month_pct: float
    negative_months: int
    max_drawdown_pct: float
    lead_pnl_pct: float
    overlay_pnl_pct: float
    lead_filled_trades: int
    overlay_filled_trades: int


def _equity_max_drawdown_pct(values: pd.Series) -> float:
    if values.empty:
        return 0.0
    curve = pd.concat([pd.Series([0.0]), values.astype(float).cumsum().reset_index(drop=True)], ignore_index=True)
    drawdown = curve - curve.cummax()
    return round(float(drawdown.min()), 6)


def _load_window_file(path: Path, source: str) -> pd.DataFrame:
    frame = pd.read_csv(path)
    if frame.empty:
        return pd.DataFrame()
    frame = frame.copy()
    frame["source"] = source
    frame["entry_time"] = pd.to_datetime(frame["entry_time"], utc=True)
    frame["exit_time"] = pd.to_datetime(frame["exit_time"], utc=True)
    frame["desired_notional_share"] = pd.to_numeric(frame["notional_share"], errors="coerce").fillna(0.0)
    frame["base_pnl_pct"] = pd.to_numeric(frame["weighted_pnl_pct"], errors="coerce").fillna(0.0)
    frame["entry_month"] = frame["entry_time"].dt.strftime("%Y-%m")
    return frame


def load_windows(lead_windows: Path, overlay_windows: Path) -> pd.DataFrame:
    """Load lead and overlay windows into one allocator input frame."""
    lead = _load_window_file(lead_windows, "lead")
    overlay = _load_window_file(overlay_windows, "overlay")
    combined = pd.concat([lead, overlay], ignore_index=True)
    if combined.empty:
        return combined
    combined = combined.sort_values(["entry_time", "source", "exit_time"]).reset_index(drop=True)
    combined["trade_id"] = [f"{row.source}-{idx}" for idx, row in combined.iterrows()]
    return combined


def _adjusted_pnl(row: pd.Series, overlay_extra_round_trip_slippage_bps: float) -> float:
    pnl = float(row["base_pnl_pct"])
    if row["source"] == "overlay" and overlay_extra_round_trip_slippage_bps > 0.0:
        # Convert round-trip bps on notional into pct of initial capital:
        # notional_share * bps / 10000 * 100.
        pnl -= float(row["desired_notional_share"]) * float(overlay_extra_round_trip_slippage_bps) / 100.0
    return pnl


def simulate_portfolio(
    windows: pd.DataFrame,
    *,
    capital_capacity: float,
    overlay_extra_round_trip_slippage_bps: float,
    policy: str = "scale_to_available",
) -> tuple[PortfolioScenario, pd.DataFrame]:
    """Allocate trades under a simple active-notional capacity."""
    if policy not in {"scale_to_available", "skip_if_insufficient"}:
        raise ValueError(f"unknown policy: {policy}")
    if windows.empty:
        scenario = PortfolioScenario(
            policy=policy,
            capital_capacity=float(capital_capacity),
            overlay_extra_round_trip_slippage_bps=float(overlay_extra_round_trip_slippage_bps),
            requested_trades=0,
            filled_trades=0,
            skipped_trades=0,
            scaled_trades=0,
            requested_notional_share=0.0,
            allocated_notional_share=0.0,
            allocation_ratio=0.0,
            peak_active_notional_share=0.0,
            calendar_sum_pct=0.0,
            worst_month_pct=0.0,
            negative_months=0,
            max_drawdown_pct=0.0,
            lead_pnl_pct=0.0,
            overlay_pnl_pct=0.0,
            lead_filled_trades=0,
            overlay_filled_trades=0,
        )
        return scenario, pd.DataFrame()

    active: list[dict[str, Any]] = []
    rows: list[dict[str, Any]] = []
    peak_active = 0.0
    for _, row in windows.sort_values(["entry_time", "source", "exit_time"]).iterrows():
        entry_time = pd.Timestamp(row["entry_time"])
        active = [item for item in active if pd.Timestamp(item["exit_time"]) > entry_time]
        active_notional = sum(float(item["allocated_notional_share"]) for item in active)
        available = max(0.0, float(capital_capacity) - active_notional)
        requested = max(0.0, float(row["desired_notional_share"]))

        if requested <= 0.0:
            scale = 0.0
        elif policy == "skip_if_insufficient" and requested > available:
            scale = 0.0
        else:
            scale = min(1.0, available / requested)

        allocated = requested * scale
        adjusted_pnl = _adjusted_pnl(row, overlay_extra_round_trip_slippage_bps)
        realized_pnl = adjusted_pnl * scale
        status = "filled"
        if scale <= 0.0:
            status = "skipped"
        elif scale < 0.999999:
            status = "scaled"

        out = row.to_dict()
        out.update(
            {
                "capital_capacity": float(capital_capacity),
                "overlay_extra_round_trip_slippage_bps": float(overlay_extra_round_trip_slippage_bps),
                "policy": policy,
                "active_notional_before": round(float(active_notional), 6),
                "available_notional_before": round(float(available), 6),
                "allocation_scale": round(float(scale), 8),
                "allocated_notional_share": round(float(allocated), 8),
                "adjusted_base_pnl_pct": round(float(adjusted_pnl), 8),
                "realized_pnl_pct": round(float(realized_pnl), 8),
                "allocation_status": status,
            }
        )
        rows.append(out)

        if allocated > 0.0:
            active.append(
                {
                    "exit_time": row["exit_time"],
                    "allocated_notional_share": allocated,
                }
            )
        peak_active = max(peak_active, sum(float(item["allocated_notional_share"]) for item in active))

    ledger = pd.DataFrame(rows)
    filled = ledger[ledger["allocation_status"] != "skipped"].copy()
    requested_notional = float(ledger["desired_notional_share"].sum())
    allocated_notional = float(ledger["allocated_notional_share"].sum())
    monthly = filled.groupby("entry_month")["realized_pnl_pct"].sum() if not filled.empty else pd.Series(dtype=float)
    realized = filled.sort_values("exit_time")["realized_pnl_pct"] if not filled.empty else pd.Series(dtype=float)
    scenario = PortfolioScenario(
        policy=policy,
        capital_capacity=round(float(capital_capacity), 6),
        overlay_extra_round_trip_slippage_bps=round(float(overlay_extra_round_trip_slippage_bps), 6),
        requested_trades=int(len(ledger)),
        filled_trades=int(len(filled)),
        skipped_trades=int((ledger["allocation_status"] == "skipped").sum()),
        scaled_trades=int((ledger["allocation_status"] == "scaled").sum()),
        requested_notional_share=round(requested_notional, 6),
        allocated_notional_share=round(allocated_notional, 6),
        allocation_ratio=round(allocated_notional / requested_notional, 6) if requested_notional > 0.0 else 0.0,
        peak_active_notional_share=round(float(peak_active), 6),
        calendar_sum_pct=round(float(filled["realized_pnl_pct"].sum()), 6) if not filled.empty else 0.0,
        worst_month_pct=round(float(monthly.min()), 6) if not monthly.empty else 0.0,
        negative_months=int((monthly < 0.0).sum()) if not monthly.empty else 0,
        max_drawdown_pct=_equity_max_drawdown_pct(realized),
        lead_pnl_pct=round(float(filled.loc[filled["source"] == "lead", "realized_pnl_pct"].sum()), 6)
        if not filled.empty
        else 0.0,
        overlay_pnl_pct=round(float(filled.loc[filled["source"] == "overlay", "realized_pnl_pct"].sum()), 6)
        if not filled.empty
        else 0.0,
        lead_filled_trades=int((filled["source"] == "lead").sum()) if not filled.empty else 0,
        overlay_filled_trades=int((filled["source"] == "overlay").sum()) if not filled.empty else 0,
    )
    return scenario, ledger


def _label_number(value: float) -> str:
    return str(value).replace(".", "p").replace("-", "m")


def _lead_window_precision(windows: pd.DataFrame) -> tuple[str, list[str], str]:
    if windows.empty or "source" not in windows.columns:
        return "unknown", [], "Lead window precision could not be inferred from the input rows."
    lead_rows = windows[windows["source"] == "lead"]
    if lead_rows.empty:
        return "none", [], "No lead windows were provided."
    if "window_source" in lead_rows.columns:
        sources = sorted(
            str(value)
            for value in lead_rows["window_source"].dropna().unique().tolist()
            if str(value)
        )
    else:
        sources = []
    if any("exact" in source for source in sources):
        return "exact", sources, "Lead windows use selected DelayResult entry/exit timestamps."
    if any("approx" in source for source in sources) or not sources:
        return "approximate", sources, "Lead windows are approximate; overlay windows are actual lifecycle windows."
    return "custom", sources, "Lead window precision follows the supplied input file."


def _write_report(
    output_dir: Path,
    scenarios: pd.DataFrame,
    *,
    lead_windows: Path,
    overlay_windows: Path,
    lead_window_precision: str,
    lead_window_sources: list[str],
    lead_window_note: str,
) -> None:
    best_rows = scenarios.sort_values(
        ["capital_capacity", "overlay_extra_round_trip_slippage_bps", "policy"]
    )
    lines = [
        "# T3 Overlay Lead Portfolio Sensitivity",
        "",
        "Research-only allocator/slippage haircut for the ETH T3 overlay lead enhancement.",
        "",
        f"- Lead windows: `{lead_windows}`",
        f"- Overlay windows: `{overlay_windows}`",
        f"- Lead window precision: `{lead_window_precision}`"
        + (f" ({', '.join(lead_window_sources)})" if lead_window_sources else ""),
        f"- {lead_window_note}",
        "- Extra slippage is round-trip bps applied only to the T3 overlay after next-second adverse lifecycle accounting.",
        "- No monthly gate is used.",
        "",
        "## Matrix",
        "",
        "| Policy | Capacity | Extra overlay RT slip | Calendar sum | Worst month | Neg months | Max DD | Allocation ratio | Skipped | Scaled | Lead PnL | Overlay PnL | Peak active |",
        "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|",
    ]
    for _, row in best_rows.iterrows():
        lines.append(
            f"| `{row['policy']}` | {float(row['capital_capacity']):.2f} "
            f"| {float(row['overlay_extra_round_trip_slippage_bps']):.1f}bp "
            f"| {float(row['calendar_sum_pct']):.6f}% "
            f"| {float(row['worst_month_pct']):.6f}% "
            f"| {int(row['negative_months'])} "
            f"| {float(row['max_drawdown_pct']):.6f}% "
            f"| {float(row['allocation_ratio']):.6f} "
            f"| {int(row['skipped_trades'])} "
            f"| {int(row['scaled_trades'])} "
            f"| {float(row['lead_pnl_pct']):.6f}% "
            f"| {float(row['overlay_pnl_pct']):.6f}% "
            f"| {float(row['peak_active_notional_share']):.6f} |"
        )
    lines.extend(
        [
            "",
            "## Read",
            "",
            "- Capacity `2.5` should approximate the additive bridge because observed active notional stays below that cap.",
            "- Capacity `1.6` is a useful stress because it roughly matches the lead's maximum single-trade desired notional share.",
            "- Capacity `1.0` is intentionally harsh and forces leverage-style lead sizing to scale down.",
            "- If the overlay remains accretive after `10-20bp` extra round-trip slippage and realistic capacity, it is still a credible lead enhancement research direction.",
        ]
    )
    (output_dir / "t3_overlay_lead_portfolio_sensitivity_report.md").write_text(
        "\n".join(lines) + "\n",
        encoding="utf-8",
    )


def run(
    *,
    lead_windows: Path,
    overlay_windows: Path,
    output_dir: Path,
    capacities: list[float],
    overlay_extra_round_trip_slippage_bps: list[float],
    policies: list[str],
    write_ledgers: bool,
) -> dict[str, Any]:
    output_dir.mkdir(parents=True, exist_ok=True)
    windows = load_windows(lead_windows, overlay_windows)
    lead_window_precision, lead_window_sources, lead_window_note = _lead_window_precision(windows)
    scenario_rows: list[dict[str, Any]] = []
    for policy in policies:
        for capacity in capacities:
            for slippage_bps in overlay_extra_round_trip_slippage_bps:
                scenario, ledger = simulate_portfolio(
                    windows,
                    capital_capacity=capacity,
                    overlay_extra_round_trip_slippage_bps=slippage_bps,
                    policy=policy,
                )
                scenario_rows.append(asdict(scenario))
                if write_ledgers:
                    ledger_name = (
                        f"portfolio_ledger_{policy}_cap{_label_number(capacity)}"
                        f"_slip{_label_number(slippage_bps)}bps.csv"
                    )
                    ledger.to_csv(output_dir / ledger_name, index=False)

    scenarios = pd.DataFrame(scenario_rows)
    scenarios.to_csv(output_dir / "t3_overlay_lead_portfolio_sensitivity_matrix.csv", index=False)
    _write_report(
        output_dir,
        scenarios,
        lead_windows=lead_windows,
        overlay_windows=overlay_windows,
        lead_window_precision=lead_window_precision,
        lead_window_sources=lead_window_sources,
        lead_window_note=lead_window_note,
    )
    payload = {
        "note": (
            f"Research-only portfolio allocator/slippage sensitivity. Lead windows are {lead_window_precision}; "
            "extra slippage is round-trip bps applied to overlay only."
        ),
        "lead_windows": str(lead_windows),
        "overlay_windows": str(overlay_windows),
        "lead_window_precision": lead_window_precision,
        "lead_window_sources": lead_window_sources,
        "capacities": capacities,
        "overlay_extra_round_trip_slippage_bps": overlay_extra_round_trip_slippage_bps,
        "policies": policies,
        "scenarios": scenario_rows,
    }
    (output_dir / "t3_overlay_lead_portfolio_sensitivity_summary.json").write_text(
        json.dumps(payload, indent=2, ensure_ascii=False, default=str) + "\n",
        encoding="utf-8",
    )
    return payload


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--lead-windows",
        type=Path,
        default=DEFAULT_EXPOSURE_DIR / "lead_approx_exposure_windows.csv",
    )
    parser.add_argument(
        "--overlay-windows",
        type=Path,
        default=DEFAULT_EXPOSURE_DIR / "t3_overlay_actual_exposure_windows.csv",
    )
    parser.add_argument("--output-dir", type=Path, default=DEFAULT_OUTPUT)
    parser.add_argument("--capacities", nargs="+", type=float, default=[1.0, 1.5, 1.6, 2.0, 2.5])
    parser.add_argument(
        "--overlay-extra-round-trip-slippage-bps",
        nargs="+",
        type=float,
        default=[0.0, 2.0, 5.0, 8.0, 10.0, 15.0, 20.0],
    )
    parser.add_argument(
        "--policies",
        nargs="+",
        choices=["scale_to_available", "skip_if_insufficient"],
        default=["scale_to_available"],
    )
    parser.add_argument("--write-ledgers", action="store_true")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    payload = run(
        lead_windows=Path(args.lead_windows),
        overlay_windows=Path(args.overlay_windows),
        output_dir=Path(args.output_dir),
        capacities=[float(value) for value in args.capacities],
        overlay_extra_round_trip_slippage_bps=[
            float(value) for value in args.overlay_extra_round_trip_slippage_bps
        ],
        policies=[str(value) for value in args.policies],
        write_ledgers=bool(args.write_ledgers),
    )
    rows = payload["scenarios"]
    base = [
        row
        for row in rows
        if row["policy"] == "scale_to_available"
        and abs(row["capital_capacity"] - 1.6) < 1e-9
        and abs(row["overlay_extra_round_trip_slippage_bps"] - 10.0) < 1e-9
    ]
    selected = base[0] if base else rows[0]
    print(
        "portfolio_sensitivity "
        f"policy={selected['policy']} "
        f"capacity={selected['capital_capacity']:.2f} "
        f"overlay_rt_slip={selected['overlay_extra_round_trip_slippage_bps']:.1f}bps "
        f"calendar_sum={selected['calendar_sum_pct']:.6f}% "
        f"max_dd={selected['max_drawdown_pct']:.6f}% "
        f"allocation_ratio={selected['allocation_ratio']:.6f}"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
