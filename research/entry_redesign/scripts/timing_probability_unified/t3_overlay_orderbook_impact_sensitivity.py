"""Order-book impact proxy for the ETH lead + T3 overlay sizing candidates.

Research-only. Historical research artifacts do not include full depth snapshots,
so this script applies a transparent proxy on top of exact lead/overlay exposure
windows:

- active-notional capacity allocation, same as the portfolio sensitivity script;
- optional lead scaling as a linear diagnostic;
- extra overlay round-trip slippage;
- a notional-impact haircut based on active notional and a top-book capacity
  proxy.

The output is not a live fill model. It is a sizing stress gate for deciding
which candidate deserves a real order-book replay.
"""

from __future__ import annotations

import argparse
import json
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Any

import pandas as pd

from timing_probability_unified.t3_overlay_lead_portfolio_sensitivity import (  # noqa: E402
    _equity_max_drawdown_pct,
    load_windows,
)

PROJECT_ROOT = Path(__file__).resolve().parents[4]
OUTPUT_DIR = (
    PROJECT_ROOT
    / "research"
    / "entry_redesign"
    / "scripts"
    / "output"
    / "timing_probability_unified"
)
DEFAULT_LEAD_WINDOWS = OUTPUT_DIR / "t3_overlay_lead_exact_exposure" / "lead_exact_adverse10_exposure_windows.csv"
DEFAULT_OVERLAY_WINDOWS = (
    OUTPUT_DIR / "t3_overlay_lead_exposure_audit" / "t3_overlay_actual_exposure_windows.csv"
)
DEFAULT_OUTPUT = OUTPUT_DIR / "t3_overlay_orderbook_impact_sensitivity"


@dataclass(frozen=True)
class ImpactProfile:
    """Proxy parameters for order-book impact stress."""

    name: str
    top_book_capacity_share: float
    excess_round_trip_bps_per_1x: float
    active_round_trip_bps_per_1x: float


@dataclass(frozen=True)
class ImpactScenario:
    """One order-book impact stress result."""

    profile: str
    lead_scale: float
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
    max_trade_notional_share: float
    max_impact_round_trip_bps: float
    avg_impact_round_trip_bps: float
    impact_cost_pct: float
    overlay_extra_cost_pct: float
    calendar_sum_pct: float
    worst_month_pct: float
    negative_months: int
    max_drawdown_pct: float
    lead_pnl_pct: float
    overlay_pnl_pct: float
    lead_filled_trades: int
    overlay_filled_trades: int


DEFAULT_PROFILES: list[ImpactProfile] = [
    ImpactProfile(
        name="moderate_top1p6_active0p5",
        top_book_capacity_share=1.6,
        excess_round_trip_bps_per_1x=8.0,
        active_round_trip_bps_per_1x=0.5,
    ),
    ImpactProfile(
        name="strict_top1p2_active1p0",
        top_book_capacity_share=1.2,
        excess_round_trip_bps_per_1x=12.0,
        active_round_trip_bps_per_1x=1.0,
    ),
    ImpactProfile(
        name="severe_top1p0_active2p0",
        top_book_capacity_share=1.0,
        excess_round_trip_bps_per_1x=20.0,
        active_round_trip_bps_per_1x=2.0,
    ),
]


def _label_number(value: float) -> str:
    return str(value).replace(".", "p").replace("-", "m")


def parse_profiles(values: list[str] | None) -> list[ImpactProfile]:
    """Parse profile specs as name:top:excess_bps_per_1x:active_bps_per_1x."""
    if not values:
        return DEFAULT_PROFILES
    profiles: list[ImpactProfile] = []
    for value in values:
        parts = value.split(":")
        if len(parts) != 4:
            raise ValueError(f"invalid impact profile {value!r}; expected name:top:excess:active")
        name, top, excess, active = parts
        profiles.append(
            ImpactProfile(
                name=name,
                top_book_capacity_share=float(top),
                excess_round_trip_bps_per_1x=float(excess),
                active_round_trip_bps_per_1x=float(active),
            )
        )
    return profiles


def apply_lead_scale(windows: pd.DataFrame, lead_scale: float) -> pd.DataFrame:
    """Scale lead notional and PnL linearly as a what-if diagnostic."""
    out = windows.copy()
    lead_mask = out["source"] == "lead"
    out.loc[lead_mask, "desired_notional_share"] = (
        pd.to_numeric(out.loc[lead_mask, "desired_notional_share"], errors="coerce").fillna(0.0)
        * float(lead_scale)
    )
    out.loc[lead_mask, "base_pnl_pct"] = (
        pd.to_numeric(out.loc[lead_mask, "base_pnl_pct"], errors="coerce").fillna(0.0)
        * float(lead_scale)
    )
    out["lead_scale"] = float(lead_scale)
    return out


def impact_round_trip_bps(
    *,
    allocated_notional_share: float,
    active_notional_before: float,
    profile: ImpactProfile,
) -> float:
    """Compute extra round-trip bps from concentration and active-notional pressure."""
    concentration = max(0.0, float(allocated_notional_share) - float(profile.top_book_capacity_share))
    return round(
        concentration * float(profile.excess_round_trip_bps_per_1x)
        + float(active_notional_before) * float(profile.active_round_trip_bps_per_1x),
        8,
    )


def simulate_impact(
    windows: pd.DataFrame,
    *,
    profile: ImpactProfile,
    capital_capacity: float,
    overlay_extra_round_trip_slippage_bps: float,
    lead_scale: float,
) -> tuple[ImpactScenario, pd.DataFrame]:
    """Allocate windows and apply order-book impact proxy costs."""
    if windows.empty:
        scenario = ImpactScenario(
            profile=profile.name,
            lead_scale=float(lead_scale),
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
            max_trade_notional_share=0.0,
            max_impact_round_trip_bps=0.0,
            avg_impact_round_trip_bps=0.0,
            impact_cost_pct=0.0,
            overlay_extra_cost_pct=0.0,
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
        scale = min(1.0, available / requested) if requested > 0.0 else 0.0
        allocated = requested * scale
        status = "filled"
        if scale <= 0.0:
            status = "skipped"
        elif scale < 0.999999:
            status = "scaled"

        impact_bps = impact_round_trip_bps(
            allocated_notional_share=allocated,
            active_notional_before=active_notional,
            profile=profile,
        )
        impact_cost_pct = allocated * impact_bps / 100.0
        overlay_extra_cost_pct = 0.0
        if row["source"] == "overlay" and overlay_extra_round_trip_slippage_bps > 0.0:
            overlay_extra_cost_pct = allocated * float(overlay_extra_round_trip_slippage_bps) / 100.0

        base_realized = float(row["base_pnl_pct"]) * scale
        realized = base_realized - impact_cost_pct - overlay_extra_cost_pct
        out = row.to_dict()
        out.update(
            {
                "profile": profile.name,
                "lead_scale": float(lead_scale),
                "capital_capacity": float(capital_capacity),
                "overlay_extra_round_trip_slippage_bps": float(overlay_extra_round_trip_slippage_bps),
                "active_notional_before": round(float(active_notional), 8),
                "available_notional_before": round(float(available), 8),
                "allocation_scale": round(float(scale), 8),
                "allocated_notional_share": round(float(allocated), 8),
                "impact_round_trip_bps": round(float(impact_bps), 8),
                "impact_cost_pct": round(float(impact_cost_pct), 8),
                "overlay_extra_cost_pct": round(float(overlay_extra_cost_pct), 8),
                "base_realized_pnl_pct": round(float(base_realized), 8),
                "realized_pnl_pct": round(float(realized), 8),
                "allocation_status": status,
            }
        )
        rows.append(out)

        if allocated > 0.0:
            active.append({"exit_time": row["exit_time"], "allocated_notional_share": allocated})
        peak_active = max(peak_active, sum(float(item["allocated_notional_share"]) for item in active))

    ledger = pd.DataFrame(rows)
    filled = ledger[ledger["allocation_status"] != "skipped"].copy()
    requested_notional = float(ledger["desired_notional_share"].sum())
    allocated_notional = float(ledger["allocated_notional_share"].sum())
    monthly = filled.groupby("entry_month")["realized_pnl_pct"].sum() if not filled.empty else pd.Series(dtype=float)
    realized = filled.sort_values("exit_time")["realized_pnl_pct"] if not filled.empty else pd.Series(dtype=float)
    impact_series = filled["impact_round_trip_bps"] if not filled.empty else pd.Series(dtype=float)
    scenario = ImpactScenario(
        profile=profile.name,
        lead_scale=round(float(lead_scale), 6),
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
        max_trade_notional_share=round(float(filled["allocated_notional_share"].max()), 6) if not filled.empty else 0.0,
        max_impact_round_trip_bps=round(float(impact_series.max()), 6) if not impact_series.empty else 0.0,
        avg_impact_round_trip_bps=round(float(impact_series.mean()), 6) if not impact_series.empty else 0.0,
        impact_cost_pct=round(float(filled["impact_cost_pct"].sum()), 6) if not filled.empty else 0.0,
        overlay_extra_cost_pct=round(float(filled["overlay_extra_cost_pct"].sum()), 6) if not filled.empty else 0.0,
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


def _write_report(
    output_dir: Path,
    scenarios: pd.DataFrame,
    *,
    lead_windows: Path,
    overlay_windows: Path,
    profiles: list[ImpactProfile],
) -> None:
    ordered = scenarios.sort_values(
        [
            "profile",
            "lead_scale",
            "capital_capacity",
            "overlay_extra_round_trip_slippage_bps",
        ]
    )
    lines = [
        "# T3 Overlay Order-Book Impact Sensitivity",
        "",
        "Research-only order-book impact proxy for sizing candidates.",
        "",
        f"- Lead windows: `{lead_windows}`",
        f"- Overlay windows: `{overlay_windows}`",
        "- Lead scaling is linear and diagnostic only.",
        "- Impact bps are additional round-trip costs on allocated notional.",
        "",
        "## Profiles",
        "",
        "| Profile | Top-book capacity | Excess RT bps / 1x | Active RT bps / 1x |",
        "|---|---:|---:|---:|",
    ]
    for profile in profiles:
        lines.append(
            f"| `{profile.name}` | {profile.top_book_capacity_share:.2f} "
            f"| {profile.excess_round_trip_bps_per_1x:.2f} "
            f"| {profile.active_round_trip_bps_per_1x:.2f} |"
        )
    lines.extend(
        [
            "",
            "## Matrix",
            "",
            "| Profile | Lead scale | Capacity | Overlay slip | Calendar | Worst month | Neg months | DD | Lead PnL | Overlay PnL | Impact cost | Overlay slip cost | Max impact bps | Peak active |",
            "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|",
        ]
    )
    for _, row in ordered.iterrows():
        lines.append(
            f"| `{row['profile']}` | {float(row['lead_scale']):.2f} "
            f"| {float(row['capital_capacity']):.2f} "
            f"| {float(row['overlay_extra_round_trip_slippage_bps']):.1f}bp "
            f"| {float(row['calendar_sum_pct']):.6f}% "
            f"| {float(row['worst_month_pct']):.6f}% "
            f"| {int(row['negative_months'])} "
            f"| {float(row['max_drawdown_pct']):.6f}% "
            f"| {float(row['lead_pnl_pct']):.6f}% "
            f"| {float(row['overlay_pnl_pct']):.6f}% "
            f"| {float(row['impact_cost_pct']):.6f}% "
            f"| {float(row['overlay_extra_cost_pct']):.6f}% "
            f"| {float(row['max_impact_round_trip_bps']):.6f} "
            f"| {float(row['peak_active_notional_share']):.6f} |"
        )
    lines.extend(
        [
            "",
            "## Read",
            "",
            "- Use this only to rank sizing candidates before a real historical depth replay exists.",
            "- A candidate that fails the strict/severe proxy should not be promoted by headline PnL.",
            "- A candidate that passes still needs live/replay event-time parity and real depth validation.",
        ]
    )
    (output_dir / "t3_overlay_orderbook_impact_sensitivity_report.md").write_text(
        "\n".join(lines) + "\n",
        encoding="utf-8",
    )


def run(
    *,
    lead_windows: Path,
    overlay_windows: Path,
    output_dir: Path,
    profiles: list[ImpactProfile],
    lead_scales: list[float],
    capacities: list[float],
    overlay_extra_round_trip_slippage_bps: list[float],
    write_ledgers: bool,
) -> dict[str, Any]:
    output_dir.mkdir(parents=True, exist_ok=True)
    base_windows = load_windows(lead_windows, overlay_windows)
    scenario_rows: list[dict[str, Any]] = []
    for lead_scale in lead_scales:
        windows = apply_lead_scale(base_windows, lead_scale)
        for profile in profiles:
            for capacity in capacities:
                for slippage_bps in overlay_extra_round_trip_slippage_bps:
                    scenario, ledger = simulate_impact(
                        windows,
                        profile=profile,
                        capital_capacity=capacity,
                        overlay_extra_round_trip_slippage_bps=slippage_bps,
                        lead_scale=lead_scale,
                    )
                    scenario_rows.append(asdict(scenario))
                    if write_ledgers:
                        name = (
                            f"impact_ledger_{profile.name}_lead{_label_number(lead_scale)}"
                            f"_cap{_label_number(capacity)}_slip{_label_number(slippage_bps)}bps.csv"
                        )
                        ledger.to_csv(output_dir / name, index=False)

    scenarios = pd.DataFrame(scenario_rows)
    scenarios.to_csv(output_dir / "t3_overlay_orderbook_impact_sensitivity_matrix.csv", index=False)
    _write_report(
        output_dir,
        scenarios,
        lead_windows=lead_windows,
        overlay_windows=overlay_windows,
        profiles=profiles,
    )
    payload = {
        "note": (
            "Research-only order-book impact proxy. Lead scale is linear; "
            "impact costs are additional round-trip bps on allocated notional."
        ),
        "lead_windows": str(lead_windows),
        "overlay_windows": str(overlay_windows),
        "profiles": [asdict(profile) for profile in profiles],
        "lead_scales": lead_scales,
        "capacities": capacities,
        "overlay_extra_round_trip_slippage_bps": overlay_extra_round_trip_slippage_bps,
        "scenarios": scenario_rows,
    }
    (output_dir / "t3_overlay_orderbook_impact_sensitivity_summary.json").write_text(
        json.dumps(payload, indent=2, ensure_ascii=False, default=str) + "\n",
        encoding="utf-8",
    )
    return payload


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--lead-windows", type=Path, default=DEFAULT_LEAD_WINDOWS)
    parser.add_argument("--overlay-windows", type=Path, default=DEFAULT_OVERLAY_WINDOWS)
    parser.add_argument("--output-dir", type=Path, default=DEFAULT_OUTPUT)
    parser.add_argument("--profiles", nargs="+")
    parser.add_argument("--lead-scales", nargs="+", type=float, default=[1.0, 1.25, 1.5])
    parser.add_argument("--capacities", nargs="+", type=float, default=[1.6, 2.0, 2.5])
    parser.add_argument(
        "--overlay-extra-round-trip-slippage-bps",
        nargs="+",
        type=float,
        default=[10.0, 15.0, 20.0],
    )
    parser.add_argument("--write-ledgers", action="store_true")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    profiles = parse_profiles(args.profiles)
    payload = run(
        lead_windows=Path(args.lead_windows),
        overlay_windows=Path(args.overlay_windows),
        output_dir=Path(args.output_dir),
        profiles=profiles,
        lead_scales=[float(value) for value in args.lead_scales],
        capacities=[float(value) for value in args.capacities],
        overlay_extra_round_trip_slippage_bps=[
            float(value) for value in args.overlay_extra_round_trip_slippage_bps
        ],
        write_ledgers=bool(args.write_ledgers),
    )
    rows = pd.DataFrame(payload["scenarios"])
    selected = rows[
        (rows["profile"] == "strict_top1p2_active1p0")
        & (rows["lead_scale"] == 1.25)
        & (rows["capital_capacity"] == 2.0)
        & (rows["overlay_extra_round_trip_slippage_bps"] == 15.0)
    ]
    row = selected.iloc[0] if not selected.empty else rows.iloc[0]
    print(
        "orderbook_impact "
        f"profile={row['profile']} "
        f"lead_scale={row['lead_scale']:.2f} "
        f"capacity={row['capital_capacity']:.2f} "
        f"overlay_rt_slip={row['overlay_extra_round_trip_slippage_bps']:.1f}bps "
        f"calendar_sum={row['calendar_sum_pct']:.6f}% "
        f"max_dd={row['max_drawdown_pct']:.6f}% "
        f"impact_cost={row['impact_cost_pct']:.6f}%"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
