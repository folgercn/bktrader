"""Conditional risk-on sizing policy for the ETH lead + T3 overlay candidate.

Research-only. This script tests a dynamic sizing rule: keep current lead and
overlay sizing by default, but allow larger lead/overlay notional/PnL scale when
the order-book impact proxy is below configured round-trip bps gates. It uses
the same proxy profiles as ``t3_overlay_orderbook_impact_sensitivity.py``.
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
from timing_probability_unified.t3_overlay_orderbook_impact_sensitivity import (  # noqa: E402
    DEFAULT_LEAD_WINDOWS,
    DEFAULT_OVERLAY_WINDOWS,
    DEFAULT_PROFILES,
    ImpactProfile,
    impact_round_trip_bps,
    parse_profiles,
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
DEFAULT_OUTPUT = OUTPUT_DIR / "t3_overlay_conditional_lead_scale"


@dataclass(frozen=True)
class ConditionalScenario:
    """One conditional risk-on sizing policy result."""

    profile: str
    target_lead_scale: float
    target_overlay_scale: float
    lead_impact_gate_round_trip_bps: float
    overlay_impact_gate_round_trip_bps: float
    capital_capacity: float
    overlay_extra_round_trip_slippage_bps: float
    requested_trades: int
    filled_trades: int
    skipped_trades: int
    scaled_trades: int
    lead_scale_applied_trades: int
    lead_scale_blocked_trades: int
    overlay_scale_applied_trades: int
    overlay_scale_blocked_trades: int
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


def simulate_conditional_policy(
    windows: pd.DataFrame,
    *,
    profile: ImpactProfile,
    target_lead_scale: float,
    lead_impact_gate_round_trip_bps: float,
    capital_capacity: float,
    overlay_extra_round_trip_slippage_bps: float,
    target_overlay_scale: float = 1.0,
    overlay_impact_gate_round_trip_bps: float | None = None,
) -> tuple[ConditionalScenario, pd.DataFrame]:
    """Simulate conditional leg scaling under impact-bps gates."""
    active: list[dict[str, Any]] = []
    rows: list[dict[str, Any]] = []
    peak_active = 0.0
    lead_scaled = 0
    lead_blocked = 0
    overlay_scaled = 0
    overlay_blocked = 0
    overlay_gate = (
        float(overlay_impact_gate_round_trip_bps)
        if overlay_impact_gate_round_trip_bps is not None
        else float(lead_impact_gate_round_trip_bps)
    )

    for _, row in windows.sort_values(["entry_time", "source", "exit_time"]).iterrows():
        entry_time = pd.Timestamp(row["entry_time"])
        active = [item for item in active if pd.Timestamp(item["exit_time"]) > entry_time]
        active_notional = sum(float(item["allocated_notional_share"]) for item in active)
        available = max(0.0, float(capital_capacity) - active_notional)
        base_requested = max(0.0, float(row["desired_notional_share"]))
        base_pnl = float(row["base_pnl_pct"])
        chosen_scale = 1.0
        scale_decision = "base"
        source = str(row["source"])

        if source == "lead" and target_lead_scale > 1.0:
            proposed_requested = base_requested * float(target_lead_scale)
            proposed_allocation_scale = min(1.0, available / proposed_requested) if proposed_requested > 0 else 0.0
            proposed_allocated = proposed_requested * proposed_allocation_scale
            proposed_impact = impact_round_trip_bps(
                allocated_notional_share=proposed_allocated,
                active_notional_before=active_notional,
                profile=profile,
            )
            if proposed_allocation_scale >= 0.999999 and proposed_impact <= float(lead_impact_gate_round_trip_bps):
                chosen_scale = float(target_lead_scale)
                scale_decision = "scaled"
                lead_scaled += 1
            else:
                scale_decision = "blocked"
                lead_blocked += 1
        elif source == "overlay" and target_overlay_scale > 1.0:
            proposed_requested = base_requested * float(target_overlay_scale)
            proposed_allocation_scale = min(1.0, available / proposed_requested) if proposed_requested > 0 else 0.0
            proposed_allocated = proposed_requested * proposed_allocation_scale
            proposed_impact = impact_round_trip_bps(
                allocated_notional_share=proposed_allocated,
                active_notional_before=active_notional,
                profile=profile,
            )
            if proposed_allocation_scale >= 0.999999 and proposed_impact <= overlay_gate:
                chosen_scale = float(target_overlay_scale)
                scale_decision = "scaled"
                overlay_scaled += 1
            else:
                scale_decision = "blocked"
                overlay_blocked += 1

        requested = base_requested * chosen_scale
        chosen_base_pnl = base_pnl * chosen_scale
        allocation_scale = min(1.0, available / requested) if requested > 0 else 0.0
        allocated = requested * allocation_scale
        status = "filled"
        if allocation_scale <= 0.0:
            status = "skipped"
        elif allocation_scale < 0.999999:
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
        base_realized = chosen_base_pnl * allocation_scale
        realized = base_realized - impact_cost_pct - overlay_extra_cost_pct

        out = row.to_dict()
        out.update(
            {
                "profile": profile.name,
                "target_lead_scale": float(target_lead_scale),
                "target_overlay_scale": float(target_overlay_scale),
                "lead_impact_gate_round_trip_bps": float(lead_impact_gate_round_trip_bps),
                "overlay_impact_gate_round_trip_bps": float(overlay_gate),
                "capital_capacity": float(capital_capacity),
                "overlay_extra_round_trip_slippage_bps": float(overlay_extra_round_trip_slippage_bps),
                "requested_notional_share": round(float(requested), 8),
                "chosen_lead_scale": float(chosen_scale) if source == "lead" else 1.0,
                "chosen_overlay_scale": float(chosen_scale) if source == "overlay" else 1.0,
                "scale_decision": scale_decision,
                "lead_scale_decision": scale_decision if source == "lead" else "not_lead",
                "overlay_scale_decision": scale_decision if source == "overlay" else "not_overlay",
                "active_notional_before": round(float(active_notional), 8),
                "available_notional_before": round(float(available), 8),
                "allocation_scale": round(float(allocation_scale), 8),
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
    requested_notional = float(ledger["requested_notional_share"].sum())
    allocated_notional = float(ledger["allocated_notional_share"].sum())
    monthly = filled.groupby("entry_month")["realized_pnl_pct"].sum() if not filled.empty else pd.Series(dtype=float)
    realized = filled.sort_values("exit_time")["realized_pnl_pct"] if not filled.empty else pd.Series(dtype=float)
    impact_series = filled["impact_round_trip_bps"] if not filled.empty else pd.Series(dtype=float)
    scenario = ConditionalScenario(
        profile=profile.name,
        target_lead_scale=round(float(target_lead_scale), 6),
        target_overlay_scale=round(float(target_overlay_scale), 6),
        lead_impact_gate_round_trip_bps=round(float(lead_impact_gate_round_trip_bps), 6),
        overlay_impact_gate_round_trip_bps=round(float(overlay_gate), 6),
        capital_capacity=round(float(capital_capacity), 6),
        overlay_extra_round_trip_slippage_bps=round(float(overlay_extra_round_trip_slippage_bps), 6),
        requested_trades=int(len(ledger)),
        filled_trades=int(len(filled)),
        skipped_trades=int((ledger["allocation_status"] == "skipped").sum()),
        scaled_trades=int((ledger["allocation_status"] == "scaled").sum()),
        lead_scale_applied_trades=int(lead_scaled),
        lead_scale_blocked_trades=int(lead_blocked),
        overlay_scale_applied_trades=int(overlay_scaled),
        overlay_scale_blocked_trades=int(overlay_blocked),
        requested_notional_share=round(requested_notional, 6),
        allocated_notional_share=round(allocated_notional, 6),
        allocation_ratio=round(allocated_notional / requested_notional, 6) if requested_notional > 0 else 0.0,
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
    )
    return scenario, ledger


def _write_report(output_dir: Path, scenarios: pd.DataFrame, *, profiles: list[ImpactProfile]) -> None:
    ordered = scenarios.sort_values(
        [
            "profile",
            "capital_capacity",
            "lead_impact_gate_round_trip_bps",
            "overlay_impact_gate_round_trip_bps",
            "overlay_extra_round_trip_slippage_bps",
        ]
    )
    lines = [
        "# T3 Overlay Conditional Lead Scale",
        "",
        "Research-only conditional sizing candidate.",
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
            "| Profile | Lead x | Overlay x | Capacity | Lead gate | Overlay gate | Overlay slip | Calendar | Worst month | Neg months | DD | Lead PnL | Overlay PnL | Scaled lead | Scaled overlay | Impact cost | Max impact bps |",
            "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|",
        ]
    )
    for _, row in ordered.iterrows():
        lines.append(
            f"| `{row['profile']}` "
            f"| {float(row['target_lead_scale']):.2f}x "
            f"| {float(row['target_overlay_scale']):.2f}x "
            f"| {float(row['capital_capacity']):.2f} "
            f"| {float(row['lead_impact_gate_round_trip_bps']):.1f}bp "
            f"| {float(row['overlay_impact_gate_round_trip_bps']):.1f}bp "
            f"| {float(row['overlay_extra_round_trip_slippage_bps']):.1f}bp "
            f"| {float(row['calendar_sum_pct']):.6f}% "
            f"| {float(row['worst_month_pct']):.6f}% "
            f"| {int(row['negative_months'])} "
            f"| {float(row['max_drawdown_pct']):.6f}% "
            f"| {float(row['lead_pnl_pct']):.6f}% "
            f"| {float(row['overlay_pnl_pct']):.6f}% "
            f"| {int(row['lead_scale_applied_trades'])} "
            f"| {int(row['overlay_scale_applied_trades'])} "
            f"| {float(row['impact_cost_pct']):.6f}% "
            f"| {float(row['max_impact_round_trip_bps']):.6f} |"
        )
    lines.extend(
        [
            "",
            "## Read",
            "",
            "- This is the research shape closest to a live guardrail: scale each leg only when its impact gate passes.",
            "- If strict profile at 15bp passes but severe profile blocks most scaling, the policy should be depth-gated before live sizing changes.",
            "- This still needs real depth replay before any template sizing change.",
        ]
    )
    (output_dir / "t3_overlay_conditional_lead_scale_report.md").write_text(
        "\n".join(lines) + "\n",
        encoding="utf-8",
    )


def run(
    *,
    lead_windows: Path,
    overlay_windows: Path,
    output_dir: Path,
    profiles: list[ImpactProfile],
    target_lead_scale: float,
    target_overlay_scale: float,
    lead_impact_gates: list[float],
    overlay_impact_gates: list[float] | None,
    capital_capacity: float | None,
    capital_capacities: list[float] | None,
    overlay_extra_round_trip_slippage_bps: list[float],
    write_ledgers: bool,
) -> dict[str, Any]:
    output_dir.mkdir(parents=True, exist_ok=True)
    windows = load_windows(lead_windows, overlay_windows)
    scenario_rows: list[dict[str, Any]] = []
    capacities = (
        [float(value) for value in capital_capacities]
        if capital_capacities
        else [float(capital_capacity) if capital_capacity is not None else 2.0]
    )
    overlay_gates = [float(value) for value in overlay_impact_gates] if overlay_impact_gates else [None]
    for profile in profiles:
        for capacity in capacities:
            for lead_gate in lead_impact_gates:
                for overlay_gate in overlay_gates:
                    for slippage_bps in overlay_extra_round_trip_slippage_bps:
                        scenario, ledger = simulate_conditional_policy(
                            windows,
                            profile=profile,
                            target_lead_scale=target_lead_scale,
                            target_overlay_scale=target_overlay_scale,
                            lead_impact_gate_round_trip_bps=lead_gate,
                            overlay_impact_gate_round_trip_bps=overlay_gate,
                            capital_capacity=capacity,
                            overlay_extra_round_trip_slippage_bps=slippage_bps,
                        )
                        scenario_rows.append(asdict(scenario))
                        if write_ledgers:
                            overlay_gate_label = (
                                "same"
                                if overlay_gate is None
                                else str(overlay_gate).replace(".", "p")
                            )
                            name = (
                                f"conditional_ledger_{profile.name}"
                                f"_cap{str(capacity).replace('.', 'p')}"
                                f"_leadgate{str(lead_gate).replace('.', 'p')}"
                                f"_overlaygate{overlay_gate_label}"
                                f"_slip{str(slippage_bps).replace('.', 'p')}bps.csv"
                            )
                            ledger.to_csv(output_dir / name, index=False)
    scenarios = pd.DataFrame(scenario_rows)
    scenarios.to_csv(output_dir / "t3_overlay_conditional_lead_scale_matrix.csv", index=False)
    _write_report(output_dir, scenarios, profiles=profiles)
    payload = {
        "note": "Research-only conditional lead-scale policy using order-book impact proxy gates.",
        "lead_windows": str(lead_windows),
        "overlay_windows": str(overlay_windows),
        "profiles": [asdict(profile) for profile in profiles],
        "target_lead_scale": float(target_lead_scale),
        "target_overlay_scale": float(target_overlay_scale),
        "lead_impact_gates": lead_impact_gates,
        "overlay_impact_gates": overlay_impact_gates,
        "capital_capacities": capacities,
        "overlay_extra_round_trip_slippage_bps": overlay_extra_round_trip_slippage_bps,
        "scenarios": scenario_rows,
    }
    (output_dir / "t3_overlay_conditional_lead_scale_summary.json").write_text(
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
    parser.add_argument("--target-lead-scale", type=float, default=1.25)
    parser.add_argument("--target-overlay-scale", type=float, default=1.0)
    parser.add_argument("--lead-impact-gates", nargs="+", type=float, default=[6.0, 8.0, 10.0])
    parser.add_argument("--overlay-impact-gates", nargs="+", type=float)
    parser.add_argument("--capital-capacity", type=float, default=2.0)
    parser.add_argument("--capital-capacities", nargs="+", type=float)
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
    profiles = parse_profiles(args.profiles) if args.profiles else DEFAULT_PROFILES
    payload = run(
        lead_windows=Path(args.lead_windows),
        overlay_windows=Path(args.overlay_windows),
        output_dir=Path(args.output_dir),
        profiles=profiles,
        target_lead_scale=float(args.target_lead_scale),
        target_overlay_scale=float(args.target_overlay_scale),
        lead_impact_gates=[float(value) for value in args.lead_impact_gates],
        overlay_impact_gates=[float(value) for value in args.overlay_impact_gates]
        if args.overlay_impact_gates
        else None,
        capital_capacity=float(args.capital_capacity),
        capital_capacities=[float(value) for value in args.capital_capacities]
        if args.capital_capacities
        else None,
        overlay_extra_round_trip_slippage_bps=[
            float(value) for value in args.overlay_extra_round_trip_slippage_bps
        ],
        write_ledgers=bool(args.write_ledgers),
    )
    rows = pd.DataFrame(payload["scenarios"])
    selected = rows[
        (rows["profile"] == "strict_top1p2_active1p0")
        & (rows["lead_impact_gate_round_trip_bps"] == 10.0)
        & (rows["overlay_impact_gate_round_trip_bps"] == 10.0)
        & (rows["overlay_extra_round_trip_slippage_bps"] == 15.0)
    ]
    row = selected.iloc[0] if not selected.empty else rows.iloc[0]
    print(
        "conditional_lead_scale "
        f"profile={row['profile']} "
        f"lead_scale={row['target_lead_scale']:.2f}x "
        f"overlay_scale={row['target_overlay_scale']:.2f}x "
        f"lead_gate={row['lead_impact_gate_round_trip_bps']:.1f}bps "
        f"overlay_gate={row['overlay_impact_gate_round_trip_bps']:.1f}bps "
        f"capacity={row['capital_capacity']:.2f} "
        f"overlay_rt_slip={row['overlay_extra_round_trip_slippage_bps']:.1f}bps "
        f"calendar_sum={row['calendar_sum_pct']:.6f}% "
        f"scaled_lead_trades={int(row['lead_scale_applied_trades'])} "
        f"scaled_overlay_trades={int(row['overlay_scale_applied_trades'])}"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
