"""Exact exposure windows for the ETH pretouch lead.

Research-only. The compact lead trade ledger stores selected delay labels and
weighted PnL, but not the selected ``DelayResult`` entry/exit timestamps. This
script replays the current production-aligned lead, attaches the selected delay
metadata, and emits a portfolio allocator input with exact lead windows.
"""

from __future__ import annotations

import argparse
import json
import logging
import sys
from copy import deepcopy
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Any

import numpy as np
import pandas as pd

SCRIPTS_DIR = Path(__file__).resolve().parents[1]
PROJECT_ROOT = Path(__file__).resolve().parents[4]
if str(SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPTS_DIR))

from dynamic_timing.execution_sim import DEFAULT_EXEC_PARAMS  # noqa: E402
from pre_breakout_timing.delay_simulator import DelayResult  # noqa: E402
from timing_probability_unified.breakout_shape_expansion import (  # noqa: E402
    BASE_SHARE,
    OUTPUT_DIR,
    _bars_cache_for_symbol,
    _load_all_1s_bars,
    _reprice_delay_results_fast,
)
from timing_probability_unified.breakout_structure_lead_expansion_combo import (  # noqa: E402
    LEAD_REPLAY_EXEC_OVERRIDES,
    _fill_scenario,
    _lead_events_for_trades,
    _lead_trades_all,
)
from timing_probability_unified.combined_executor import (  # noqa: E402
    CombinedPositionConfig,
    compute_combined_positions,
)
from timing_probability_unified.timing_classifier import get_selected_delay_pnl  # noqa: E402
from timing_probability_unified.unified_runner import _simulate_delays_for_events  # noqa: E402

logger = logging.getLogger(__name__)

DEFAULT_OUTPUT = OUTPUT_DIR / "t3_overlay_lead_exact_exposure"
DEFAULT_REFERENCE_ADVERSE = OUTPUT_DIR / "breakout_structure_lead_combo_lead_adverse10_trades.csv"
DEFAULT_SCENARIO = "next_adverse_xslip10bps"
WINDOW_SOURCE = "lead_exact_adverse10"


@dataclass(frozen=True)
class ParitySummary:
    """Parity check between exact-window rebuild and compact reference ledger."""

    reference_rows: int
    exact_rows: int
    missing_exact_events: int
    extra_exact_events: int
    selected_delay_mismatches: int
    max_abs_weighted_pnl_diff: float
    max_abs_position_size_diff: float
    max_abs_delay_pnl_diff: float


def _selected_delay_result(
    prediction: str,
    event_delays: list[DelayResult],
) -> tuple[str, float, DelayResult | None]:
    """Return the selected delay label, pnl, and DelayResult metadata."""
    label, pnl = get_selected_delay_pnl(prediction, event_delays)
    if label == "none":
        return label, pnl, None
    for result in event_delays:
        if result.delay_label == label:
            return label, pnl, result
    raise ValueError(f"selected delay {label!r} not present in event delays")


def attach_selected_delay_metadata(
    trades: pd.DataFrame,
    delay_results: list[list[DelayResult]],
    *,
    scenario_name: str,
    window_source: str = WINDOW_SOURCE,
) -> pd.DataFrame:
    """Attach selected DelayResult entry/exit metadata to a combined trade ledger."""
    if len(trades) != len(delay_results):
        raise ValueError(f"trade/result length mismatch: {len(trades)} != {len(delay_results)}")

    rows: list[dict[str, Any]] = []
    for idx, trade in trades.reset_index(drop=True).iterrows():
        prediction = str(trade["timing_prediction"])
        selected_label, selected_pnl, selected_result = _selected_delay_result(prediction, delay_results[idx])
        ledger_label = str(trade["selected_delay"])
        if selected_label != ledger_label:
            raise ValueError(
                f"selected delay mismatch for {trade.get('event_id', idx)}: "
                f"ledger={ledger_label} replay={selected_label}"
            )
        ledger_pnl = float(trade["delay_pnl_pct"])
        if not np.isclose(ledger_pnl, float(selected_pnl), atol=1e-12, rtol=0.0):
            raise ValueError(
                f"selected pnl mismatch for {trade.get('event_id', idx)}: "
                f"ledger={ledger_pnl} replay={selected_pnl}"
            )

        out = trade.to_dict()
        out["month"] = pd.Timestamp(trade["touch_time"]).strftime("%Y-%m")
        out["notional_share"] = float(trade.get("position_size", 0.0))
        out["weighted_pnl_pct"] = float(trade.get("weighted_pnl", 0.0)) * 100.0
        out["fill_scenario"] = scenario_name
        out["window_source"] = window_source
        out["lead_window_model"] = "selected_delay_result"

        if selected_result is None:
            out.update(
                {
                    "entry_time": pd.NaT,
                    "entry_price": np.nan,
                    "exit_time": pd.NaT,
                    "exit_reason": "",
                    "hold_seconds": np.nan,
                    "mfe_r": np.nan,
                    "mae_r": np.nan,
                    "delay_seconds": 0,
                    "delay_traded": False,
                }
            )
        else:
            out.update(
                {
                    "entry_time": selected_result.entry_time,
                    "entry_price": selected_result.entry_price,
                    "exit_time": selected_result.exit_time,
                    "exit_reason": selected_result.exit_reason,
                    "hold_seconds": selected_result.hold_seconds,
                    "mfe_r": selected_result.mfe_r,
                    "mae_r": selected_result.mae_r,
                    "delay_seconds": selected_result.delay_seconds,
                    "delay_traded": bool(selected_result.traded),
                }
            )
        rows.append(out)

    out = pd.DataFrame(rows)
    for column in ("touch_time", "entry_time", "exit_time"):
        if column in out.columns:
            out[column] = pd.to_datetime(out[column], utc=True)
    return out


def _traded_gate_on(trades: pd.DataFrame) -> pd.DataFrame:
    if trades.empty:
        return trades.copy()
    return trades[
        (trades["speed_gate_pass"] == True)  # noqa: E712
        & (trades["selected_delay"] != "none")
    ].copy().reset_index(drop=True)


def _replay_lead_exact(
    *,
    symbol: str,
    scenario_name: str,
) -> tuple[pd.DataFrame, dict[str, Any]]:
    lead_all = _lead_trades_all()
    lead_all = lead_all[lead_all["symbol"].astype(str) == symbol].reset_index(drop=True)
    events = _lead_events_for_trades(lead_all)
    bars_1s = _load_all_1s_bars(symbol)
    bars_cache = _bars_cache_for_symbol(symbol, bars_1s)
    scenario = _fill_scenario(scenario_name)

    saved_params = deepcopy(DEFAULT_EXEC_PARAMS)
    lead_params = deepcopy(DEFAULT_EXEC_PARAMS)
    lead_params.update(LEAD_REPLAY_EXEC_OVERRIDES)
    try:
        DEFAULT_EXEC_PARAMS.clear()
        DEFAULT_EXEC_PARAMS.update(lead_params)
        same_delay_results, errors = _simulate_delays_for_events(events, bars_cache)
    finally:
        DEFAULT_EXEC_PARAMS.clear()
        DEFAULT_EXEC_PARAMS.update(saved_params)

    repriced_delay_results = _reprice_delay_results_fast(
        delay_results=same_delay_results,
        events=events,
        bars_cache=bars_cache,
        scenario=scenario,
    )
    trades = compute_combined_positions(
        events=events,
        timing_predictions=lead_all["timing_prediction"].to_numpy(dtype=object),
        sizing_multipliers=lead_all["sizing_multiplier"].to_numpy(dtype="float64"),
        delay_results=repriced_delay_results,
        speed_gate_pass=lead_all["speed_gate_pass"].to_numpy(dtype=bool),
        config=CombinedPositionConfig(base_notional_share=BASE_SHARE),
    )
    trades["source_leg"] = "canonical_lead"
    exact_all = attach_selected_delay_metadata(
        trades,
        repriced_delay_results,
        scenario_name=scenario_name,
    )
    exact_gate_on = _traded_gate_on(exact_all)
    diagnostics = {
        "symbol": symbol,
        "scenario": scenario_name,
        "lead_replayed_events": int(len(events)),
        "lead_delay_errors": int(len(errors)),
        "lead_exec_params": lead_params,
        "exact_rows_all": int(len(exact_all)),
        "exact_rows_gate_on": int(len(exact_gate_on)),
        "missing_entry_times_gate_on": int(exact_gate_on["entry_time"].isna().sum()) if not exact_gate_on.empty else 0,
        "missing_exit_times_gate_on": int(exact_gate_on["exit_time"].isna().sum()) if not exact_gate_on.empty else 0,
    }
    return exact_gate_on.reset_index(drop=True), diagnostics


def _monthly_metrics(windows: pd.DataFrame) -> dict[str, Any]:
    if windows.empty:
        return {"calendar_sum_pct": 0.0, "worst_month_pct": 0.0, "negative_months": 0, "trade_count": 0}
    frame = windows.copy()
    frame["month"] = pd.to_datetime(frame["touch_time"], utc=True).dt.strftime("%Y-%m")
    monthly = frame.groupby("month")["weighted_pnl_pct"].sum()
    return {
        "calendar_sum_pct": round(float(monthly.sum()), 6),
        "worst_month_pct": round(float(monthly.min()), 6),
        "negative_months": int((monthly < 0.0).sum()),
        "trade_count": int(len(frame)),
    }


def compare_to_reference(exact: pd.DataFrame, reference_path: Path) -> ParitySummary:
    """Compare exact-window ledger with the compact adverse lead reference."""
    reference = pd.read_csv(reference_path)
    if reference.empty and exact.empty:
        return ParitySummary(0, 0, 0, 0, 0, 0.0, 0.0, 0.0)

    ref = reference.copy()
    ex = exact.copy()
    ref["event_id"] = ref["event_id"].astype(str)
    ex["event_id"] = ex["event_id"].astype(str)
    merged = ref.merge(
        ex,
        on="event_id",
        how="outer",
        suffixes=("_reference", "_exact"),
        indicator=True,
    )
    both = merged[merged["_merge"] == "both"].copy()
    selected_mismatch = int(
        (
            both["selected_delay_reference"].astype(str)
            != both["selected_delay_exact"].astype(str)
        ).sum()
    )

    def max_abs_delta(column: str) -> float:
        if both.empty:
            return 0.0
        diff = (
            pd.to_numeric(both[f"{column}_reference"], errors="coerce")
            - pd.to_numeric(both[f"{column}_exact"], errors="coerce")
        ).abs()
        return round(float(diff.max()), 15) if len(diff) else 0.0

    return ParitySummary(
        reference_rows=int(len(ref)),
        exact_rows=int(len(ex)),
        missing_exact_events=int((merged["_merge"] == "left_only").sum()),
        extra_exact_events=int((merged["_merge"] == "right_only").sum()),
        selected_delay_mismatches=selected_mismatch,
        max_abs_weighted_pnl_diff=max_abs_delta("weighted_pnl"),
        max_abs_position_size_diff=max_abs_delta("position_size"),
        max_abs_delay_pnl_diff=max_abs_delta("delay_pnl_pct"),
    )


def _write_report(
    *,
    output_dir: Path,
    windows: pd.DataFrame,
    diagnostics: dict[str, Any],
    parity: ParitySummary,
) -> None:
    metrics = _monthly_metrics(windows)
    hold_seconds = pd.to_numeric(windows.get("hold_seconds", pd.Series(dtype=float)), errors="coerce")
    lines = [
        "# Lead Exact Exposure Windows",
        "",
        "Research-only rebuild of the ETH pretouch lead exposure ledger.",
        "",
        "## Summary",
        "",
        "| Calendar sum | Worst month | Neg months | Trades | Missing entry | Missing exit | Max hold |",
        "|---:|---:|---:|---:|---:|---:|---:|",
        f"| {metrics['calendar_sum_pct']:.6f}% "
        f"| {metrics['worst_month_pct']:.6f}% "
        f"| {int(metrics['negative_months'])} "
        f"| {int(metrics['trade_count'])} "
        f"| {int(diagnostics['missing_entry_times_gate_on'])} "
        f"| {int(diagnostics['missing_exit_times_gate_on'])} "
        f"| {float(hold_seconds.max()) if not hold_seconds.dropna().empty else 0.0:.2f}s |",
        "",
        "## Reference Parity",
        "",
        "| Reference rows | Exact rows | Missing exact events | Extra exact events | Delay mismatches | Max weighted PnL diff | Max position diff | Max delay PnL diff |",
        "|---:|---:|---:|---:|---:|---:|---:|---:|",
        f"| {parity.reference_rows} | {parity.exact_rows} "
        f"| {parity.missing_exact_events} | {parity.extra_exact_events} "
        f"| {parity.selected_delay_mismatches} "
        f"| {parity.max_abs_weighted_pnl_diff:.15f} "
        f"| {parity.max_abs_position_size_diff:.15f} "
        f"| {parity.max_abs_delay_pnl_diff:.15f} |",
        "",
        "## Read",
        "",
        "- Windows use the selected `DelayResult` entry/exit timestamps from the production-aligned lead replay.",
        "- PnL parity is checked against the existing compact adverse10 lead ledger before using this in portfolio sensitivity.",
        "- The adverse fill proxy keeps the execution simulator's exit time and applies next-second adverse entry repricing as a first-order stress.",
        "",
        "## Diagnostics",
        "",
        "```json",
        json.dumps(diagnostics, indent=2, ensure_ascii=False, default=str),
        "```",
    ]
    (output_dir / "lead_exact_adverse10_exposure_report.md").write_text(
        "\n".join(lines) + "\n",
        encoding="utf-8",
    )


def run(
    *,
    output_dir: Path,
    symbol: str,
    scenario_name: str,
    reference_adverse_trades: Path,
    strict_parity: bool,
) -> dict[str, Any]:
    output_dir.mkdir(parents=True, exist_ok=True)
    windows, diagnostics = _replay_lead_exact(symbol=symbol, scenario_name=scenario_name)
    parity = compare_to_reference(windows, reference_adverse_trades)

    if strict_parity and (
        parity.missing_exact_events
        or parity.extra_exact_events
        or parity.selected_delay_mismatches
        or parity.max_abs_weighted_pnl_diff > 1e-10
        or parity.max_abs_position_size_diff > 1e-12
        or parity.max_abs_delay_pnl_diff > 1e-10
    ):
        raise ValueError(f"exact lead parity check failed: {asdict(parity)}")

    windows.to_csv(output_dir / "lead_exact_adverse10_exposure_windows.csv", index=False)
    pd.DataFrame([_monthly_metrics(windows)]).to_csv(
        output_dir / "lead_exact_adverse10_metrics.csv",
        index=False,
    )
    payload = {
        "note": (
            "Research-only exact lead exposure ledger. Windows use selected DelayResult entry/exit times "
            "and preserve adverse10 compact-ledger PnL parity."
        ),
        "output_windows": str(output_dir / "lead_exact_adverse10_exposure_windows.csv"),
        "reference_adverse_trades": str(reference_adverse_trades),
        "metrics": _monthly_metrics(windows),
        "parity": asdict(parity),
        "diagnostics": diagnostics,
    }
    (output_dir / "lead_exact_adverse10_exposure_summary.json").write_text(
        json.dumps(payload, indent=2, ensure_ascii=False, default=str) + "\n",
        encoding="utf-8",
    )
    _write_report(output_dir=output_dir, windows=windows, diagnostics=diagnostics, parity=parity)
    return payload


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output-dir", type=Path, default=DEFAULT_OUTPUT)
    parser.add_argument("--symbol", default="ETHUSDT")
    parser.add_argument("--scenario", default=DEFAULT_SCENARIO)
    parser.add_argument("--reference-adverse-trades", type=Path, default=DEFAULT_REFERENCE_ADVERSE)
    parser.add_argument("--no-strict-parity", action="store_true")
    parser.add_argument("--log-level", default="INFO")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    logging.basicConfig(
        level=getattr(logging, str(args.log_level).upper(), logging.INFO),
        format="%(asctime)s %(levelname)s %(message)s",
    )
    payload = run(
        output_dir=Path(args.output_dir),
        symbol=str(args.symbol),
        scenario_name=str(args.scenario),
        reference_adverse_trades=Path(args.reference_adverse_trades),
        strict_parity=not bool(args.no_strict_parity),
    )
    metrics = payload["metrics"]
    parity = payload["parity"]
    print(
        "lead_exact_exposure "
        f"calendar_sum={metrics['calendar_sum_pct']:.6f}% "
        f"worst_month={metrics['worst_month_pct']:.6f}% "
        f"trades={metrics['trade_count']} "
        f"max_abs_weighted_pnl_diff={parity['max_abs_weighted_pnl_diff']:.3g}"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
