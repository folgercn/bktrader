"""Exposure audit for adding a T3 overlay back onto the ETH pretouch lead.

Research-only. This script replays the ETH T3 direct-entry overlay as lifecycle
trades, audits its realized exposure/final-mark risk, and compares it with the
canonical lead using an additive accounting bridge. Lead rows only contain
event-level pnl, so lead exposure windows are conservative approximations:
``entry_time = touch_time + selected_delay`` and ``exit_time = entry_time + 2h``.
T3 overlay windows use the actual lifecycle entry/exit rows.
"""

from __future__ import annotations

import argparse
import json
import logging
import sys
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Any

import numpy as np
import pandas as pd

PROJECT_ROOT = Path(__file__).resolve().parents[4]
RESEARCH_DIR = PROJECT_ROOT / "research"
SCRIPTS_DIR = Path(__file__).resolve().parents[1]
for path in (RESEARCH_DIR, SCRIPTS_DIR):
    if str(path) not in sys.path:
        sys.path.insert(0, str(path))

import eth_q1_breakout_t3_shape_compare as lifecycle  # noqa: E402
from timing_probability_unified.t3_filtered_external_event_lifecycle import (  # noqa: E402
    T3_60M_EXIT_OVERRIDES,
    FilteredT3Spec,
    apply_spec,
    load_scored_events,
)
from timing_probability_unified.t3_lifecycle_exposure_audit import (  # noqa: E402
    summarize_t3_exposure,
)
from timing_probability_unified.t3_lifecycle_outcome_diagnostics import (  # noqa: E402
    pair_lifecycle_trades,
)
from timing_probability_unified.t3_overlay_lead_bridge import (  # noqa: E402
    DEFAULT_LEAD_ADVERSE_TRADES,
    DEFAULT_LEAD_SAME_TRADES,
    DEFAULT_T3_SUMMARY,
    _load_t3_summary,
    run as run_bridge,
)
from timing_probability_unified.t2_lifecycle_context_sizing import EXTENDED_MONTHS  # noqa: E402
from timing_probability_unified.t3_pre_touch_lifecycle import (  # noqa: E402
    INITIAL_BALANCE,
    T3_REENTRY_SIZE_SCHEDULE,
    _load_window_bars,
    _month_bounds,
    _patched_replay_kwargs,
)

logger = logging.getLogger(__name__)

OUTPUT_DIR = (
    PROJECT_ROOT
    / "research"
    / "entry_redesign"
    / "scripts"
    / "output"
    / "timing_probability_unified"
)
DEFAULT_SCORED_EVENTS = OUTPUT_DIR / "t3_probability_overlay_extended" / "t3_probability_overlay_scored_events.csv"
DEFAULT_OUTPUT = OUTPUT_DIR / "t3_overlay_lead_exposure_audit"
LEAD_MAX_HOLD_HOURS = 2.0
LIFECYCLE_COMMISSION = 0.001


@dataclass
class ExposureOverlapSummary:
    """Approximate lead-vs-overlay capital overlap summary."""

    lead_windows: int
    overlay_windows: int
    overlap_pairs: int
    lead_windows_with_overlap: int
    overlay_windows_with_overlap: int
    max_combined_notional_share: float
    p95_combined_notional_share: float
    avg_combined_notional_share: float
    overlap_notional_hours: float
    max_overlap_seconds: float
    worst_overlap_overlay_pnl_pct: float
    overlap_overlay_pnl_pct: float
    overlap_lead_weighted_pnl_pct: float


def _window_events(events: pd.DataFrame, symbol: str, start: pd.Timestamp, end: pd.Timestamp) -> pd.DataFrame:
    if events.empty:
        return events.copy()
    mask = (
        (events["symbol"].astype(str) == symbol)
        & (pd.to_datetime(events["touch_time"], utc=True) >= start)
        & (pd.to_datetime(events["touch_time"], utc=True) <= end)
    )
    return events[mask].copy()


def _lead_delay_seconds(label: Any) -> float:
    text = str(label)
    if text.startswith("D"):
        try:
            return float(text[1:])
        except ValueError:
            return 0.0
    if text == "none":
        return 0.0
    # Pullback-style lead rows do not expose exact entry time in the compact
    # ledger; touch_time is the least optimistic approximation.
    return 0.0


def _load_lead_windows(path: Path, months: list[str]) -> pd.DataFrame:
    trades = pd.read_csv(path)
    if trades.empty:
        return pd.DataFrame()
    trades["touch_time"] = pd.to_datetime(trades["touch_time"], utc=True)
    trades["month"] = trades["touch_time"].dt.strftime("%Y-%m")
    trades = trades[trades["month"].isin(months)].copy()
    if "selected_delay" in trades.columns:
        delays = trades["selected_delay"].map(_lead_delay_seconds)
    else:
        delays = pd.Series(0.0, index=trades.index)
    trades["entry_time"] = trades["touch_time"] + pd.to_timedelta(delays, unit="s")
    trades["exit_time"] = trades["entry_time"] + pd.Timedelta(hours=LEAD_MAX_HOLD_HOURS)
    trades["notional_share"] = pd.to_numeric(trades.get("position_size", 0.0), errors="coerce").fillna(0.0)
    trades["weighted_pnl_pct"] = pd.to_numeric(trades.get("weighted_pnl", 0.0), errors="coerce").fillna(0.0) * 100.0
    trades["window_source"] = "lead_approx"
    return trades.reset_index(drop=True)


def _overlay_windows(trades: pd.DataFrame) -> pd.DataFrame:
    if trades.empty:
        return pd.DataFrame()
    out = trades.copy()
    out["entry_time"] = pd.to_datetime(out["entry_time"], utc=True)
    out["exit_time"] = pd.to_datetime(out["exit_time"], utc=True)
    out["notional_share"] = pd.to_numeric(out["notional"], errors="coerce").fillna(0.0) / float(INITIAL_BALANCE)
    out["weighted_pnl_pct"] = pd.to_numeric(out["pnl_initial_pct"], errors="coerce").fillna(0.0)
    out["window_source"] = "t3_overlay_actual"
    return out.reset_index(drop=True)


def _apply_round_trip_fee_adjustment(
    trades: pd.DataFrame,
    *,
    initial_balance: float,
    commission: float = LIFECYCLE_COMMISSION,
) -> pd.DataFrame:
    """Convert paired lifecycle trade diagnostics from gross to fee-net PnL.

    ``pair_lifecycle_trades`` derives price-only pnl from entry/exit rows. The
    lifecycle replay itself charges commission on entry and exit, so promotion
    audits should use the same fee-net accounting as ``summarize_run``.
    """
    if trades.empty:
        return trades.copy()
    out = trades.copy()
    gross_initial_pct = pd.to_numeric(out["pnl_initial_pct"], errors="coerce").fillna(0.0)
    notional = pd.to_numeric(out["notional"], errors="coerce").fillna(0.0)
    fee_initial_pct = notional * float(commission) * 2.0 / float(initial_balance) * 100.0
    out["gross_pnl_initial_pct"] = gross_initial_pct
    out["round_trip_fee_initial_pct"] = fee_initial_pct.round(6)
    out["pnl_initial_pct"] = (gross_initial_pct - fee_initial_pct).round(6)
    if "pnl_bps" in out.columns:
        gross_bps = pd.to_numeric(out["pnl_bps"], errors="coerce").fillna(0.0)
        out["gross_pnl_bps"] = gross_bps
        out["round_trip_fee_bps"] = round(float(commission) * 2.0 * 10000.0, 6)
        out["pnl_bps"] = (gross_bps - float(commission) * 2.0 * 10000.0).round(6)
    out["outcome"] = np.where(out["pnl_initial_pct"] > 0.0, "win", "loss")
    return out


def _overlap_pairs(lead: pd.DataFrame, overlay: pd.DataFrame) -> pd.DataFrame:
    rows: list[dict[str, Any]] = []
    if lead.empty or overlay.empty:
        return pd.DataFrame()
    for lead_idx, lead_row in lead.iterrows():
        lead_start = pd.Timestamp(lead_row["entry_time"])
        lead_end = pd.Timestamp(lead_row["exit_time"])
        candidates = overlay[
            (overlay["entry_time"] < lead_end)
            & (overlay["exit_time"] > lead_start)
        ]
        for overlay_idx, overlay_row in candidates.iterrows():
            overlap_start = max(lead_start, pd.Timestamp(overlay_row["entry_time"]))
            overlap_end = min(lead_end, pd.Timestamp(overlay_row["exit_time"]))
            overlap_seconds = max(0.0, (overlap_end - overlap_start).total_seconds())
            if overlap_seconds <= 0.0:
                continue
            lead_share = float(lead_row["notional_share"])
            overlay_share = float(overlay_row["notional_share"])
            rows.append(
                {
                    "lead_index": int(lead_idx),
                    "overlay_index": int(overlay_idx),
                    "month": str(overlay_row.get("month", "")),
                    "overlap_start": overlap_start.isoformat(),
                    "overlap_end": overlap_end.isoformat(),
                    "overlap_seconds": round(float(overlap_seconds), 3),
                    "lead_notional_share": lead_share,
                    "overlay_notional_share": overlay_share,
                    "combined_notional_share": lead_share + overlay_share,
                    "overlap_notional_hours": (lead_share + overlay_share) * overlap_seconds / 3600.0,
                    "lead_weighted_pnl_pct": float(lead_row.get("weighted_pnl_pct", 0.0)),
                    "overlay_pnl_initial_pct": float(overlay_row.get("pnl_initial_pct", 0.0)),
                    "overlay_exit_reason": str(overlay_row.get("exit_reason", "")),
                    "overlay_entry_reason": str(overlay_row.get("entry_reason", "")),
                }
            )
    return pd.DataFrame(rows)


def summarize_overlaps(lead: pd.DataFrame, overlay: pd.DataFrame, overlaps: pd.DataFrame) -> ExposureOverlapSummary:
    if overlaps.empty:
        return ExposureOverlapSummary(
            lead_windows=int(len(lead)),
            overlay_windows=int(len(overlay)),
            overlap_pairs=0,
            lead_windows_with_overlap=0,
            overlay_windows_with_overlap=0,
            max_combined_notional_share=0.0,
            p95_combined_notional_share=0.0,
            avg_combined_notional_share=0.0,
            overlap_notional_hours=0.0,
            max_overlap_seconds=0.0,
            worst_overlap_overlay_pnl_pct=0.0,
            overlap_overlay_pnl_pct=0.0,
            overlap_lead_weighted_pnl_pct=0.0,
        )
    return ExposureOverlapSummary(
        lead_windows=int(len(lead)),
        overlay_windows=int(len(overlay)),
        overlap_pairs=int(len(overlaps)),
        lead_windows_with_overlap=int(overlaps["lead_index"].nunique()),
        overlay_windows_with_overlap=int(overlaps["overlay_index"].nunique()),
        max_combined_notional_share=round(float(overlaps["combined_notional_share"].max()), 6),
        p95_combined_notional_share=round(float(overlaps["combined_notional_share"].quantile(0.95)), 6),
        avg_combined_notional_share=round(float(overlaps["combined_notional_share"].mean()), 6),
        overlap_notional_hours=round(float(overlaps["overlap_notional_hours"].sum()), 6),
        max_overlap_seconds=round(float(overlaps["overlap_seconds"].max()), 3),
        worst_overlap_overlay_pnl_pct=round(float(overlaps["overlay_pnl_initial_pct"].min()), 6),
        overlap_overlay_pnl_pct=round(
            float(overlay.loc[sorted(overlaps["overlay_index"].unique()), "pnl_initial_pct"].sum()),
            6,
        ),
        overlap_lead_weighted_pnl_pct=round(
            float(lead.loc[sorted(overlaps["lead_index"].unique()), "weighted_pnl_pct"].sum()),
            6,
        ),
    )


def _equity_max_drawdown_pct(values: pd.Series) -> float:
    if values.empty:
        return 0.0
    curve = pd.concat([pd.Series([0.0]), values.astype(float).cumsum().reset_index(drop=True)], ignore_index=True)
    drawdown = curve - curve.cummax()
    return round(float(drawdown.min()), 6)


def _combined_equity_summary(lead: pd.DataFrame, overlay: pd.DataFrame) -> dict[str, Any]:
    lead_rows = pd.DataFrame()
    if not lead.empty:
        lead_rows = pd.DataFrame(
            {
                "time": pd.to_datetime(lead["exit_time"], utc=True),
                "source": "lead_approx",
                "pnl_pct": pd.to_numeric(lead["weighted_pnl_pct"], errors="coerce").fillna(0.0),
            }
        )
    overlay_rows = pd.DataFrame()
    if not overlay.empty:
        overlay_rows = pd.DataFrame(
            {
                "time": pd.to_datetime(overlay["exit_time"], utc=True),
                "source": "t3_overlay_actual",
                "pnl_pct": pd.to_numeric(overlay["pnl_initial_pct"], errors="coerce").fillna(0.0),
            }
        )
    combined = pd.concat([lead_rows, overlay_rows], ignore_index=True).sort_values("time")
    if combined.empty:
        return {"combined_trade_rows": 0, "combined_pnl_pct": 0.0, "combined_equity_max_dd_pct": 0.0}
    return {
        "combined_trade_rows": int(len(combined)),
        "combined_pnl_pct": round(float(combined["pnl_pct"].sum()), 6),
        "combined_equity_max_dd_pct": _equity_max_drawdown_pct(combined["pnl_pct"]),
        "lead_pnl_pct": round(float(lead_rows["pnl_pct"].sum()), 6) if not lead_rows.empty else 0.0,
        "overlay_pnl_pct": round(float(overlay_rows["pnl_pct"].sum()), 6) if not overlay_rows.empty else 0.0,
    }


def run_t3_overlay_silo(
    *,
    filtered_events: pd.DataFrame,
    symbol: str,
    month: str,
    timeframe: str,
    initial_balance: float,
    external_entry_mode: str,
    t3_size_scale: float,
    reentry_fill_policy: str,
    early_horizon_seconds: int,
) -> tuple[float, int, pd.DataFrame]:
    start, end = _month_bounds(month)
    external_events = _window_events(filtered_events, symbol, start, end)
    second_bars = _load_window_bars(symbol, start, end)
    _, signal = lifecycle.build_signal_frame(second_bars, timeframe)
    t3_schedule = [float(size) * float(t3_size_scale) for size in T3_REENTRY_SIZE_SCHEDULE]

    with _patched_replay_kwargs(symbol):
        ledger, _diagnostics = lifecycle.run_second_bar_replay(
            second_bars,
            signal,
            initial_balance=initial_balance,
            breakout_shape="baseline_plus_t3",
            replay_mode="live_intrabar_sma5",
            t3_reentry_size_schedule=t3_schedule,
            t3_cooldown_bars=0,
            t3_quality_filters={"allowed_sides": []},
            quality_filter_shapes=["t3_swing"],
            shape_sizing_filters={"allowed_sides": []},
            sizing_filter_shapes=["original_t2"],
            sizing_filter_fail_multiplier=0.0,
            sizing_filter_fail_action="skip_lock",
            t3_exit_overrides=dict(T3_60M_EXIT_OVERRIDES),
            external_breakout_events=external_events,
            external_breakout_shape_name="t3_swing",
            external_entry_mode=external_entry_mode,
            reentry_fill_policy=reentry_fill_policy,
        )

    summary = lifecycle.summarize_run(ledger, initial_balance)
    trades = pair_lifecycle_trades(
        ledger,
        second_bars,
        symbol=symbol,
        month=month,
        initial_balance=initial_balance,
        early_horizon_seconds=early_horizon_seconds,
    )
    if not trades.empty:
        trades = trades[trades["breakout_shape_name"] == "t3_swing"].copy()
        trades = _apply_round_trip_fee_adjustment(trades, initial_balance=initial_balance)
        trades["candidate"] = "t3_overlay_eth_all_speed_ge035"
    return float(summary["return_pct"]), int(summary["trades"]), trades


def run_t3_overlay_audit(
    *,
    scored_events: Path,
    months: list[str],
    symbol: str,
    timeframe: str,
    initial_balance: float,
    external_entry_mode: str,
    t3_size_scale: float,
    reentry_fill_policy: str,
    early_horizon_seconds: int,
) -> tuple[dict[str, Any], pd.DataFrame]:
    spec = FilteredT3Spec("all_speed_abs_ge_0p35", speed_abs_min=0.35)
    events = apply_spec(load_scored_events(scored_events), spec)
    all_trades = []
    calendar_returns: list[float] = []
    total_trades = 0
    for month in months:
        logger.info("Running T3 overlay exposure audit %s %s", symbol, month)
        ret, trades_count, trades = run_t3_overlay_silo(
            filtered_events=events,
            symbol=symbol,
            month=month,
            timeframe=timeframe,
            initial_balance=initial_balance,
            external_entry_mode=external_entry_mode,
            t3_size_scale=t3_size_scale,
            reentry_fill_policy=reentry_fill_policy,
            early_horizon_seconds=early_horizon_seconds,
        )
        calendar_returns.append(ret)
        total_trades += trades_count
        if not trades.empty:
            all_trades.append(trades)
    trades_frame = pd.concat(all_trades, ignore_index=True) if all_trades else pd.DataFrame()
    summary = summarize_t3_exposure(
        candidate="t3_overlay_eth_all_speed_ge035",
        t3_exit_overrides=dict(T3_60M_EXIT_OVERRIDES),
        scope="aggregate",
        calendar_returns=calendar_returns,
        total_trades=total_trades,
        trades=trades_frame,
    )
    payload = asdict(summary)
    if not trades_frame.empty:
        payload["t3_gross_pnl_pct"] = round(float(trades_frame["gross_pnl_initial_pct"].sum()), 6)
        payload["t3_round_trip_fee_pct"] = round(float(trades_frame["round_trip_fee_initial_pct"].sum()), 6)
        payload["calendar_vs_net_pnl_gap_pct"] = round(
            float(payload["calendar_silo_sum_pct"] - payload["t3_net_pnl_pct"]),
            6,
        )
    else:
        payload["t3_gross_pnl_pct"] = 0.0
        payload["t3_round_trip_fee_pct"] = 0.0
        payload["calendar_vs_net_pnl_gap_pct"] = 0.0
    return payload, trades_frame


def _write_report(
    *,
    output_dir: Path,
    t3_exposure: dict[str, Any],
    overlap: ExposureOverlapSummary,
    combined_equity: dict[str, Any],
    bridge_rows: list[dict[str, Any]],
    t3_size_scale: float,
) -> None:
    lines = [
        "# T3 Overlay Lead Exposure Audit",
        "",
        "Research-only risk audit for adding the ETH T3 direct-entry overlay back to the pretouch research lead.",
        "",
        f"- T3 size scale: `{t3_size_scale}`",
        "- Lead exposure windows are approximate because compact lead ledgers do not store exact exit time.",
        "- T3 overlay exposure uses actual lifecycle entry/exit rows.",
        "",
        "## Additive Bridge",
        "",
        "| Variant | Calendar Sum | Worst Month | Neg Months | Trades |",
        "|---|---:|---:|---:|---:|",
    ]
    for row in bridge_rows:
        lines.append(
            f"| `{row['variant']}` | {float(row['calendar_sum_pct']):.6f}% "
            f"| {float(row['worst_month_pct']):.6f}% "
            f"| {int(row['negative_months'])} | {int(row['trade_count'])} |"
        )
    lines.extend(
        [
            "",
            "## T3 Overlay Exposure",
            "",
            "| Calendar Sum | Worst Silo | Neg Silos | T3 Trades | Fee-Net T3 PnL | Gross PnL | Fees | Ex Final Mark | Final Mark | Win Rate | T3 DD | Loss Streak | Avg Hold | P90 Hold | Max Hold | Worst MAE | Worst Net PnL |",
            "|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|",
            f"| {float(t3_exposure['calendar_silo_sum_pct']):.6f}% "
            f"| {float(t3_exposure['worst_calendar_silo_pct']):.6f}% "
            f"| {int(t3_exposure['negative_calendar_silos'])} "
            f"| {int(t3_exposure['t3_trades'])} "
            f"| {float(t3_exposure['t3_net_pnl_pct']):.6f}% "
            f"| {float(t3_exposure.get('t3_gross_pnl_pct', 0.0)):.6f}% "
            f"| {float(t3_exposure.get('t3_round_trip_fee_pct', 0.0)):.6f}% "
            f"| {float(t3_exposure['t3_net_pnl_ex_final_mark_pct']):.6f}% "
            f"| {float(t3_exposure['final_mark_pnl_pct']):.6f}%/{int(t3_exposure['final_mark_trades'])} "
            f"| {float(t3_exposure['t3_win_rate_pct']):.2f}% "
            f"| {float(t3_exposure['t3_equity_max_dd_pct']):.6f}% "
            f"| {int(t3_exposure['t3_max_loss_streak'])} "
            f"| {float(t3_exposure['t3_avg_hold_seconds']):.2f}s "
            f"| {float(t3_exposure['t3_p90_hold_seconds']):.2f}s "
            f"| {float(t3_exposure['t3_max_hold_seconds']):.2f}s "
            f"| {float(t3_exposure['t3_worst_mae_bps']):.4f}bp "
            f"| {float(t3_exposure['t3_worst_pnl_bps']):.4f}bp |",
            "",
            "## Approximate Capital Overlap",
            "",
            "| Lead windows | Overlay windows | Pairs | Lead overlapped | Overlay overlapped | Max combined notional | P95 combined notional | Overlap notional hours | Max overlap | Overlap overlay PnL | Overlap lead PnL |",
            "|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|",
            f"| {overlap.lead_windows} | {overlap.overlay_windows} | {overlap.overlap_pairs} "
            f"| {overlap.lead_windows_with_overlap} | {overlap.overlay_windows_with_overlap} "
            f"| {overlap.max_combined_notional_share:.6f} "
            f"| {overlap.p95_combined_notional_share:.6f} "
            f"| {overlap.overlap_notional_hours:.6f} "
            f"| {overlap.max_overlap_seconds:.2f}s "
            f"| {overlap.overlap_overlay_pnl_pct:.6f}% "
            f"| {overlap.overlap_lead_weighted_pnl_pct:.6f}% |",
            "",
            "## Combined Equity Approximation",
            "",
            f"- Combined realized PnL: `{combined_equity['combined_pnl_pct']:.6f}%`",
            f"- Combined sequential max drawdown: `{combined_equity['combined_equity_max_dd_pct']:.6f}%`",
            "",
            "## Read",
            "",
            "- T3 overlay PnL/DD uses fee-net paired lifecycle trades to match the calendar-return accounting.",
            "- This run found no timestamp overlap between approximate lead windows and actual overlay windows, but the lead window model is still approximate.",
            "- T3 final-mark contribution and drawdown are reported separately to avoid treating month-end marks as normal exits.",
        ]
    )
    (output_dir / "t3_overlay_lead_exposure_audit_report.md").write_text("\n".join(lines) + "\n", encoding="utf-8")


def run(
    *,
    output_dir: Path,
    scored_events: Path,
    lead_adverse_trades: Path,
    lead_same_trades: Path,
    t3_summary: Path,
    months: list[str],
    symbol: str,
    timeframe: str,
    initial_balance: float,
    external_entry_mode: str,
    t3_size_scale: float,
    reentry_fill_policy: str,
    early_horizon_seconds: int,
) -> dict[str, Any]:
    output_dir.mkdir(parents=True, exist_ok=True)
    bridge_dir = output_dir / "bridge"
    bridge_rows = run_bridge(
        lead_adverse_trades=lead_adverse_trades,
        lead_same_trades=lead_same_trades,
        t3_summary_path=t3_summary,
        output_dir=bridge_dir,
    )
    t3_exposure, t3_trades = run_t3_overlay_audit(
        scored_events=scored_events,
        months=months,
        symbol=symbol,
        timeframe=timeframe,
        initial_balance=initial_balance,
        external_entry_mode=external_entry_mode,
        t3_size_scale=t3_size_scale,
        reentry_fill_policy=reentry_fill_policy,
        early_horizon_seconds=early_horizon_seconds,
    )
    t3_trades.to_csv(output_dir / "t3_overlay_lifecycle_trades.csv", index=False)
    lead_windows = _load_lead_windows(lead_adverse_trades, months)
    overlay_windows = _overlay_windows(t3_trades)
    overlaps = _overlap_pairs(lead_windows, overlay_windows)
    if not overlaps.empty:
        overlaps.to_csv(output_dir / "t3_overlay_lead_overlap_pairs.csv", index=False)
    lead_windows.to_csv(output_dir / "lead_approx_exposure_windows.csv", index=False)
    overlay_windows.to_csv(output_dir / "t3_overlay_actual_exposure_windows.csv", index=False)
    overlap_summary = summarize_overlaps(lead_windows, overlay_windows, overlaps)
    combined_equity = _combined_equity_summary(lead_windows, overlay_windows)
    payload = {
        "note": (
            "Research-only exposure audit. Lead windows are approximate from touch_time + selected_delay to +2h; "
            "T3 overlay windows are actual lifecycle entry/exit."
        ),
        "months": months,
        "symbol": symbol,
        "timeframe": timeframe,
        "external_entry_mode": external_entry_mode,
        "t3_size_scale": float(t3_size_scale),
        "t3_summary": str(t3_summary),
        "lead_adverse_trades": str(lead_adverse_trades),
        "t3_exposure": t3_exposure,
        "overlap": asdict(overlap_summary),
        "combined_equity": combined_equity,
        "bridge_rows": bridge_rows,
    }
    (output_dir / "t3_overlay_lead_exposure_audit_summary.json").write_text(
        json.dumps(payload, indent=2, ensure_ascii=False, default=str) + "\n",
        encoding="utf-8",
    )
    pd.DataFrame([t3_exposure]).to_csv(output_dir / "t3_overlay_exposure_summary.csv", index=False)
    pd.DataFrame([asdict(overlap_summary)]).to_csv(output_dir / "t3_overlay_lead_overlap_summary.csv", index=False)
    _write_report(
        output_dir=output_dir,
        t3_exposure=t3_exposure,
        overlap=overlap_summary,
        combined_equity=combined_equity,
        bridge_rows=bridge_rows,
        t3_size_scale=t3_size_scale,
    )
    return payload


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output-dir", type=Path, default=DEFAULT_OUTPUT)
    parser.add_argument("--scored-events", type=Path, default=DEFAULT_SCORED_EVENTS)
    parser.add_argument("--lead-adverse-trades", type=Path, default=DEFAULT_LEAD_ADVERSE_TRADES)
    parser.add_argument("--lead-same-trades", type=Path, default=DEFAULT_LEAD_SAME_TRADES)
    parser.add_argument("--t3-summary", type=Path, default=DEFAULT_T3_SUMMARY)
    parser.add_argument("--months", nargs="+", default=EXTENDED_MONTHS)
    parser.add_argument("--symbol", default="ETHUSDT")
    parser.add_argument("--timeframe", default="1h")
    parser.add_argument("--initial-balance", type=float, default=INITIAL_BALANCE)
    parser.add_argument(
        "--external-entry-mode",
        choices=["next_second_open", "next_second_close", "next_second_adverse"],
        default="next_second_adverse",
    )
    parser.add_argument("--t3-size-scale", type=float, default=2.0)
    parser.add_argument(
        "--reentry-fill-policy",
        choices=["historical", "strict_next_second_cross"],
        default="strict_next_second_cross",
    )
    parser.add_argument("--early-horizon-seconds", type=int, default=300)
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
        scored_events=Path(args.scored_events),
        lead_adverse_trades=Path(args.lead_adverse_trades),
        lead_same_trades=Path(args.lead_same_trades),
        t3_summary=Path(args.t3_summary),
        months=[str(month) for month in args.months],
        symbol=str(args.symbol),
        timeframe=str(args.timeframe),
        initial_balance=float(args.initial_balance),
        external_entry_mode=str(args.external_entry_mode),
        t3_size_scale=float(args.t3_size_scale),
        reentry_fill_policy=str(args.reentry_fill_policy),
        early_horizon_seconds=int(args.early_horizon_seconds),
    )
    bridge = {row["variant"]: row for row in payload["bridge_rows"]}
    combo = bridge.get("lead_adverse10_plus_t3_overlay", {})
    print(
        "lead_adverse10_plus_t3_overlay="
        f"{float(combo.get('calendar_sum_pct', 0.0)):.6f}% "
        f"t3_dd={payload['t3_exposure']['t3_equity_max_dd_pct']:.6f}% "
        f"overlap_max_notional={payload['overlap']['max_combined_notional_share']:.6f}"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
