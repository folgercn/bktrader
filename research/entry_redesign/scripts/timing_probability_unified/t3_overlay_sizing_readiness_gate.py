"""Readiness gate for the ETH risk-on sizing candidate.

Research-only. This script combines:

- live execution telemetry calibration;
- conditional lead/overlay-scale proxy results;
- the conservative lead adverse baseline.

It turns the current evidence into a simple promotion status so future research
does not over-read a single good PnL row or a tiny live sample.
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
DEFAULT_LIVE_SUMMARY = (
    OUTPUT_DIR
    / "t3_overlay_live_depth_calibration_20260519"
    / "t3_overlay_live_depth_calibration_summary.json"
)
DEFAULT_CONDITIONAL_SUMMARY = (
    OUTPUT_DIR
    / "t3_overlay_conditional_risk_appetite_1p5x2p0_20260519"
    / "t3_overlay_conditional_lead_scale_summary.json"
)
DEFAULT_OUTPUT = OUTPUT_DIR / "t3_overlay_sizing_readiness_gate_risk_appetite_20260519"


@dataclass(frozen=True)
class ReadinessConfig:
    """Thresholds for sizing-readiness classification."""

    target_lead_scale: float = 1.5
    target_overlay_scale: float = 2.0
    min_live_entries_for_live_candidate: int = 30
    min_live_combined_pass_ratio: float = 1.0
    min_worst_slippage_headroom_bps: float = 2.0
    max_strict_impact_gate_bps: float = 20.0
    overlay_pass_slippage_bps: float = 15.0
    overlay_kill_slippage_bps: float = 20.0
    lead_adverse_baseline_pct: float = 22.971648


@dataclass(frozen=True)
class ReadinessResult:
    """Combined sizing readiness result."""

    status: str
    target_lead_scale: float
    target_overlay_scale: float
    live_sample_entries: int
    live_combined_pass_entries: int
    live_combined_pass_ratio: float
    live_min_scaled_top_depth_coverage: float | None
    live_worst_slippage_headroom_bps: float | None
    live_max_adverse_fill_drift_bps: float | None
    strict_15bp_calendar_sum_pct: float | None
    strict_20bp_calendar_sum_pct: float | None
    severe_15bp_calendar_sum_pct: float | None
    strict_15bp_passes_baseline: bool
    strict_20bp_kill_fails_baseline: bool
    severe_15bp_fails_baseline: bool
    live_gate_pass: bool
    proxy_gate_pass: bool
    sample_gate_pass: bool
    reasons: list[str]


def _load(path: Path) -> dict[str, Any]:
    with path.open("r", encoding="utf-8") as handle:
        payload = json.load(handle)
    if not isinstance(payload, dict):
        raise ValueError(f"expected JSON object in {path}")
    return payload


def _float_or_none(value: Any) -> float | None:
    if value is None or value == "":
        return None
    try:
        return float(value)
    except (TypeError, ValueError):
        return None


def _find_live_scale(live_summary: dict[str, Any], target_scale: float) -> dict[str, Any]:
    rows = live_summary.get("matrix", [])
    if not isinstance(rows, list):
        return {}
    for row in rows:
        if isinstance(row, dict) and abs(float(row.get("quantity_scale", 0.0)) - target_scale) < 1e-9:
            return row
    return {}


def _find_conditional(
    conditional_summary: dict[str, Any],
    *,
    profile: str,
    gate_bps: float,
    overlay_slippage_bps: float,
    target_lead_scale: float,
    target_overlay_scale: float,
) -> dict[str, Any]:
    rows = conditional_summary.get("scenarios", [])
    if not isinstance(rows, list):
        return {}
    candidates: list[dict[str, Any]] = []
    for row in rows:
        if not isinstance(row, dict):
            continue
        if str(row.get("profile")) != profile:
            continue
        if abs(float(row.get("target_lead_scale", target_lead_scale)) - target_lead_scale) > 1e-9:
            continue
        if abs(float(row.get("target_overlay_scale", target_overlay_scale)) - target_overlay_scale) > 1e-9:
            continue
        if float(row.get("lead_impact_gate_round_trip_bps", 999999.0)) > gate_bps:
            continue
        if float(row.get("overlay_impact_gate_round_trip_bps", row.get("lead_impact_gate_round_trip_bps", 999999.0))) > gate_bps:
            continue
        if abs(float(row.get("overlay_extra_round_trip_slippage_bps", -1.0)) - overlay_slippage_bps) > 1e-9:
            continue
        candidates.append(row)
    if not candidates:
        return {}
    return max(candidates, key=lambda item: float(item.get("calendar_sum_pct", float("-inf"))))


def evaluate_readiness(
    live_summary: dict[str, Any],
    conditional_summary: dict[str, Any],
    *,
    config: ReadinessConfig = ReadinessConfig(),
) -> ReadinessResult:
    """Evaluate combined live/proxy readiness."""
    live_row = _find_live_scale(live_summary, config.target_lead_scale)
    live_entries = int(live_row.get("sample_entries", 0) or 0)
    live_combined = int(live_row.get("combined_pass_entries", 0) or 0)
    live_pass_ratio = round(float(live_combined) / live_entries, 6) if live_entries else 0.0
    live_headroom = _float_or_none(live_row.get("worst_slippage_headroom_bps"))
    live_gate_pass = (
        live_entries > 0
        and live_pass_ratio >= config.min_live_combined_pass_ratio
        and live_headroom is not None
        and live_headroom >= config.min_worst_slippage_headroom_bps
    )
    sample_gate_pass = live_entries >= config.min_live_entries_for_live_candidate

    strict15 = _find_conditional(
        conditional_summary,
        profile="strict_top1p2_active1p0",
        gate_bps=config.max_strict_impact_gate_bps,
        overlay_slippage_bps=config.overlay_pass_slippage_bps,
        target_lead_scale=config.target_lead_scale,
        target_overlay_scale=config.target_overlay_scale,
    )
    strict20 = _find_conditional(
        conditional_summary,
        profile="strict_top1p2_active1p0",
        gate_bps=config.max_strict_impact_gate_bps,
        overlay_slippage_bps=config.overlay_kill_slippage_bps,
        target_lead_scale=config.target_lead_scale,
        target_overlay_scale=config.target_overlay_scale,
    )
    severe15 = _find_conditional(
        conditional_summary,
        profile="severe_top1p0_active2p0",
        gate_bps=config.max_strict_impact_gate_bps,
        overlay_slippage_bps=config.overlay_pass_slippage_bps,
        target_lead_scale=config.target_lead_scale,
        target_overlay_scale=config.target_overlay_scale,
    )
    strict15_calendar = _float_or_none(strict15.get("calendar_sum_pct"))
    strict20_calendar = _float_or_none(strict20.get("calendar_sum_pct"))
    severe15_calendar = _float_or_none(severe15.get("calendar_sum_pct"))
    strict15_pass = strict15_calendar is not None and strict15_calendar > config.lead_adverse_baseline_pct
    strict20_fails = strict20_calendar is not None and strict20_calendar <= config.lead_adverse_baseline_pct
    severe15_fails = severe15_calendar is not None and severe15_calendar <= config.lead_adverse_baseline_pct
    proxy_gate_pass = strict15_pass and strict20_fails and severe15_fails

    reasons: list[str] = []
    if live_gate_pass:
        reasons.append("live telemetry passes current guard for target scale")
    else:
        reasons.append("live telemetry does not pass current guard for target scale")
    if not sample_gate_pass:
        reasons.append(
            f"live sample size {live_entries} is below promotion threshold "
            f"{config.min_live_entries_for_live_candidate}"
        )
    if strict15_pass:
        reasons.append("strict 15bp proxy remains above lead adverse baseline")
    else:
        reasons.append("strict 15bp proxy does not beat lead adverse baseline")
    if strict20_fails:
        reasons.append("strict 20bp remains a kill-stress failure")
    else:
        reasons.append("strict 20bp no longer behaves as kill-stress")
    if severe15_fails:
        reasons.append("severe 15bp fails by design, so thin-book scale should be blocked")
    else:
        reasons.append("severe 15bp does not fail, proxy separation is weak")

    if proxy_gate_pass and live_gate_pass and sample_gate_pass:
        status = "live_candidate_ready_for_human_review"
    elif proxy_gate_pass and live_gate_pass:
        status = "research_continue_collect_live_depth"
    else:
        status = "blocked"

    return ReadinessResult(
        status=status,
        target_lead_scale=round(float(config.target_lead_scale), 6),
        target_overlay_scale=round(float(config.target_overlay_scale), 6),
        live_sample_entries=live_entries,
        live_combined_pass_entries=live_combined,
        live_combined_pass_ratio=live_pass_ratio,
        live_min_scaled_top_depth_coverage=_float_or_none(live_row.get("min_scaled_top_depth_coverage")),
        live_worst_slippage_headroom_bps=live_headroom,
        live_max_adverse_fill_drift_bps=_float_or_none(live_row.get("max_adverse_fill_drift_bps")),
        strict_15bp_calendar_sum_pct=strict15_calendar,
        strict_20bp_calendar_sum_pct=strict20_calendar,
        severe_15bp_calendar_sum_pct=severe15_calendar,
        strict_15bp_passes_baseline=bool(strict15_pass),
        strict_20bp_kill_fails_baseline=bool(strict20_fails),
        severe_15bp_fails_baseline=bool(severe15_fails),
        live_gate_pass=bool(live_gate_pass),
        proxy_gate_pass=bool(proxy_gate_pass),
        sample_gate_pass=bool(sample_gate_pass),
        reasons=reasons,
    )


def _write_report(output_dir: Path, result: ReadinessResult, config: ReadinessConfig) -> None:
    lines = [
        "# T3 Overlay Sizing Readiness Gate",
        "",
        "Research-only readiness gate for risk-on sizing.",
        "",
        "## Verdict",
        "",
        f"- Status: `{result.status}`",
        f"- Target lead scale: `{result.target_lead_scale:.2f}x`",
        f"- Target overlay scale: `{result.target_overlay_scale:.2f}x`",
        "",
        "## Evidence",
        "",
        "| Check | Result |",
        "|---|---:|",
        f"| Live samples | {result.live_sample_entries} |",
        f"| Live combined pass | {result.live_combined_pass_entries}/{result.live_sample_entries} ({result.live_combined_pass_ratio:.0%}) |",
        f"| Live min scaled top-depth coverage | {result.live_min_scaled_top_depth_coverage} |",
        f"| Live worst 8bp slippage headroom | {result.live_worst_slippage_headroom_bps}bp |",
        f"| Live max adverse fill drift | {result.live_max_adverse_fill_drift_bps}bp |",
        f"| Strict 15bp proxy calendar | {result.strict_15bp_calendar_sum_pct}% |",
        f"| Strict 20bp proxy calendar | {result.strict_20bp_calendar_sum_pct}% |",
        f"| Severe 15bp proxy calendar | {result.severe_15bp_calendar_sum_pct}% |",
        "",
        "## Thresholds",
        "",
        "| Threshold | Value |",
        "|---|---:|",
        f"| Lead adverse baseline | {config.lead_adverse_baseline_pct:.6f}% |",
        f"| Min live samples for live-candidate review | {config.min_live_entries_for_live_candidate} |",
        f"| Min live combined pass ratio | {config.min_live_combined_pass_ratio:.0%} |",
        f"| Min worst slippage headroom | {config.min_worst_slippage_headroom_bps:.2f}bp |",
        f"| Strict impact gate upper bound | {config.max_strict_impact_gate_bps:.2f}bp |",
        "",
        "## Reasons",
        "",
    ]
    for reason in result.reasons:
        lines.append(f"- {reason}")
    lines.extend(
        [
            "",
            "## Read",
            "",
            "- This is not a live sizing change and does not alter submitted quantity.",
            "- `research_continue_collect_live_depth` means the candidate shape is still alive, but the sample size is too small for live promotion.",
            "- If promoted later, the live-facing rule should remain conditional and fail closed to current sizing.",
        ]
    )
    (output_dir / "t3_overlay_sizing_readiness_gate_report.md").write_text(
        "\n".join(lines) + "\n",
        encoding="utf-8",
    )


def run(
    *,
    live_summary_path: Path,
    conditional_summary_path: Path,
    output_dir: Path,
    config: ReadinessConfig,
) -> dict[str, Any]:
    output_dir.mkdir(parents=True, exist_ok=True)
    live_summary = _load(live_summary_path)
    conditional_summary = _load(conditional_summary_path)
    result = evaluate_readiness(live_summary, conditional_summary, config=config)
    payload = {
        "note": "Research-only sizing readiness gate.",
        "live_summary_path": str(live_summary_path),
        "conditional_summary_path": str(conditional_summary_path),
        "config": asdict(config),
        "result": asdict(result),
    }
    (output_dir / "t3_overlay_sizing_readiness_gate_summary.json").write_text(
        json.dumps(payload, indent=2, ensure_ascii=False, default=str) + "\n",
        encoding="utf-8",
    )
    pd.DataFrame([asdict(result)]).to_csv(
        output_dir / "t3_overlay_sizing_readiness_gate_summary.csv",
        index=False,
    )
    _write_report(output_dir, result, config)
    return payload


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--live-summary", type=Path, default=DEFAULT_LIVE_SUMMARY)
    parser.add_argument("--conditional-summary", type=Path, default=DEFAULT_CONDITIONAL_SUMMARY)
    parser.add_argument("--output-dir", type=Path, default=DEFAULT_OUTPUT)
    parser.add_argument("--target-lead-scale", type=float, default=1.5)
    parser.add_argument("--target-overlay-scale", type=float, default=2.0)
    parser.add_argument("--max-strict-impact-gate-bps", type=float, default=20.0)
    parser.add_argument("--min-live-entries", type=int, default=30)
    parser.add_argument("--min-worst-slippage-headroom-bps", type=float, default=2.0)
    parser.add_argument("--lead-adverse-baseline-pct", type=float, default=22.971648)
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    config = ReadinessConfig(
        target_lead_scale=float(args.target_lead_scale),
        target_overlay_scale=float(args.target_overlay_scale),
        max_strict_impact_gate_bps=float(args.max_strict_impact_gate_bps),
        min_live_entries_for_live_candidate=int(args.min_live_entries),
        min_worst_slippage_headroom_bps=float(args.min_worst_slippage_headroom_bps),
        lead_adverse_baseline_pct=float(args.lead_adverse_baseline_pct),
    )
    payload = run(
        live_summary_path=Path(args.live_summary),
        conditional_summary_path=Path(args.conditional_summary),
        output_dir=Path(args.output_dir),
        config=config,
    )
    result = payload["result"]
    print(
        "sizing_readiness "
        f"status={result['status']} "
        f"lead_scale={result['target_lead_scale']:.2f}x "
        f"overlay_scale={result['target_overlay_scale']:.2f}x "
        f"live_pass={result['live_combined_pass_entries']}/{result['live_sample_entries']} "
        f"proxy_gate_pass={result['proxy_gate_pass']} "
        f"sample_gate_pass={result['sample_gate_pass']}"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
