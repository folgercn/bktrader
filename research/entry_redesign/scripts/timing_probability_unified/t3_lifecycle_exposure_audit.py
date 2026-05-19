"""Exposure and final-mark audit for strict T3 lifecycle candidates.

This research-only tool runs executable T3 exit candidates through the same
``baseline_plus_t3`` + strict reentry lifecycle replay, then audits realized T3
trades for exposure, adverse excursion, final mark-to-market sensitivity, and a
T3-only equity drawdown curve. It is a risk review companion to
``t3_lifecycle_exit_sweep.py``; it does not change live behavior.
"""

from __future__ import annotations

import argparse
import json
import logging
import sys
from dataclasses import asdict, dataclass
from pathlib import Path

import pandas as pd

PROJECT_ROOT = Path(__file__).resolve().parents[4]
RESEARCH_DIR = PROJECT_ROOT / "research"
_SCRIPTS_DIR = Path(__file__).resolve().parents[1]
for path in (RESEARCH_DIR, _SCRIPTS_DIR):
    if str(path) not in sys.path:
        sys.path.insert(0, str(path))

import eth_q1_breakout_t3_shape_compare as lifecycle  # noqa: E402
from timing_probability_unified.t3_lifecycle_exit_sweep import (  # noqa: E402
    T3ExitSpec,
    build_exit_specs,
    filter_exit_specs,
)
from timing_probability_unified.t3_lifecycle_outcome_diagnostics import (  # noqa: E402
    _t3_trades,
    pair_lifecycle_trades,
)
from timing_probability_unified.t3_pre_touch_lifecycle import (  # noqa: E402
    DEFAULT_MONTHS,
    DEFAULT_SYMBOLS,
    INITIAL_BALANCE,
    T3_REENTRY_SIZE_SCHEDULE,
    _load_window_bars,
    _month_bounds,
    _patched_replay_kwargs,
)

logger = logging.getLogger(__name__)


@dataclass
class T3ExposureSummary:
    """Aggregate T3 risk metrics for one candidate or silo."""

    scope: str
    candidate: str
    t3_exit_overrides: dict
    calendar_silo_sum_pct: float
    worst_calendar_silo_pct: float
    negative_calendar_silos: int
    total_trades: int
    t3_trades: int
    t3_net_pnl_pct: float
    t3_net_pnl_ex_final_mark_pct: float
    final_mark_trades: int
    final_mark_pnl_pct: float
    t3_win_rate_pct: float
    t3_equity_max_dd_pct: float
    t3_max_loss_streak: int
    t3_avg_hold_seconds: float
    t3_median_hold_seconds: float
    t3_p90_hold_seconds: float
    t3_max_hold_seconds: float
    t3_sum_hold_hours: float
    t3_avg_mae_bps: float
    t3_worst_mae_bps: float
    t3_avg_mfe_bps: float
    t3_avg_pnl_bps: float
    t3_worst_pnl_bps: float
    t3_exit_reasons: dict


def _merge_counts(dst: dict[str, int], src: pd.Series) -> None:
    for key, value in src.to_dict().items():
        dst[str(key)] = int(dst.get(str(key), 0)) + int(value)


def _max_loss_streak(t3: pd.DataFrame) -> int:
    streak = 0
    best = 0
    for value in t3["pnl_initial_pct"].tolist():
        if float(value) <= 0.0:
            streak += 1
            best = max(best, streak)
        else:
            streak = 0
    return int(best)


def _equity_max_drawdown_pct(t3: pd.DataFrame) -> float:
    """Compute max drawdown on a T3-only cumulative pnl-initial-pct curve."""
    if t3.empty:
        return 0.0
    curve = pd.concat(
        [
            pd.Series([0.0]),
            t3["pnl_initial_pct"].astype(float).cumsum().reset_index(drop=True),
        ],
        ignore_index=True,
    )
    peak = curve.cummax()
    drawdown = curve - peak
    return round(float(drawdown.min()), 6)


def summarize_t3_exposure(
    *,
    candidate: str,
    t3_exit_overrides: dict,
    scope: str,
    calendar_returns: list[float],
    total_trades: int,
    trades: pd.DataFrame,
) -> T3ExposureSummary:
    """Summarize T3 exposure and final-mark sensitivity."""
    t3 = _t3_trades(trades)
    t3 = t3.sort_values("exit_time").reset_index(drop=True) if not t3.empty else t3.copy()
    final_mark = t3[t3["exit_reason"] == "FinalMarkToMarket"].copy() if not t3.empty else t3.copy()
    normal_exit = t3[t3["exit_reason"] != "FinalMarkToMarket"].copy() if not t3.empty else t3.copy()

    exit_reasons: dict[str, int] = {}
    if not t3.empty:
        _merge_counts(exit_reasons, t3["exit_reason"].value_counts())

    return T3ExposureSummary(
        scope=scope,
        candidate=candidate,
        t3_exit_overrides=dict(t3_exit_overrides),
        calendar_silo_sum_pct=round(float(sum(calendar_returns)), 6),
        worst_calendar_silo_pct=round(float(min(calendar_returns, default=0.0)), 6),
        negative_calendar_silos=int(sum(1 for value in calendar_returns if value < 0.0)),
        total_trades=int(total_trades),
        t3_trades=int(len(t3)),
        t3_net_pnl_pct=round(float(t3["pnl_initial_pct"].sum()), 6) if not t3.empty else 0.0,
        t3_net_pnl_ex_final_mark_pct=round(float(normal_exit["pnl_initial_pct"].sum()), 6)
        if not normal_exit.empty
        else 0.0,
        final_mark_trades=int(len(final_mark)),
        final_mark_pnl_pct=round(float(final_mark["pnl_initial_pct"].sum()), 6)
        if not final_mark.empty
        else 0.0,
        t3_win_rate_pct=round(float((t3["pnl_initial_pct"] > 0).mean()) * 100.0, 6)
        if not t3.empty
        else 0.0,
        t3_equity_max_dd_pct=_equity_max_drawdown_pct(t3),
        t3_max_loss_streak=_max_loss_streak(t3) if not t3.empty else 0,
        t3_avg_hold_seconds=round(float(t3["hold_seconds"].mean()), 6) if not t3.empty else 0.0,
        t3_median_hold_seconds=round(float(t3["hold_seconds"].median()), 6) if not t3.empty else 0.0,
        t3_p90_hold_seconds=round(float(t3["hold_seconds"].quantile(0.9)), 6) if not t3.empty else 0.0,
        t3_max_hold_seconds=round(float(t3["hold_seconds"].max()), 6) if not t3.empty else 0.0,
        t3_sum_hold_hours=round(float(t3["hold_seconds"].sum()) / 3600.0, 6)
        if not t3.empty
        else 0.0,
        t3_avg_mae_bps=round(float(t3["mae_bps"].mean()), 6) if not t3.empty else 0.0,
        t3_worst_mae_bps=round(float(t3["mae_bps"].min()), 6) if not t3.empty else 0.0,
        t3_avg_mfe_bps=round(float(t3["mfe_bps"].mean()), 6) if not t3.empty else 0.0,
        t3_avg_pnl_bps=round(float(t3["pnl_bps"].mean()), 6) if not t3.empty else 0.0,
        t3_worst_pnl_bps=round(float(t3["pnl_bps"].min()), 6) if not t3.empty else 0.0,
        t3_exit_reasons=exit_reasons,
    )


def run_audit_silo(
    *,
    spec: T3ExitSpec,
    symbol: str,
    month: str,
    timeframe: str,
    initial_balance: float,
    early_horizon_seconds: int,
    reentry_fill_policy: str,
) -> tuple[T3ExposureSummary, pd.DataFrame]:
    """Run one symbol-month audit and return paired trades."""
    start, end = _month_bounds(month)
    second_bars = _load_window_bars(symbol, start, end)
    _, signal = lifecycle.build_signal_frame(second_bars, timeframe)

    with _patched_replay_kwargs(symbol):
        ledger, _diagnostics = lifecycle.run_second_bar_replay(
            second_bars,
            signal,
            initial_balance=initial_balance,
            breakout_shape="baseline_plus_t3",
            replay_mode="live_intrabar_sma5",
            t3_reentry_size_schedule=T3_REENTRY_SIZE_SCHEDULE,
            t3_cooldown_bars=0,
            t3_quality_filters={"max_pre_touch_seconds": 900.0},
            quality_filter_shapes=["t3_swing"],
            t3_exit_overrides=dict(spec.t3_exit_overrides),
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
        trades = trades.assign(candidate=spec.label)
    row = summarize_t3_exposure(
        candidate=spec.label,
        t3_exit_overrides=spec.t3_exit_overrides,
        scope=f"{symbol}:{month}",
        calendar_returns=[float(summary["return_pct"])],
        total_trades=int(summary["trades"]),
        trades=trades,
    )
    return row, trades


def run_exposure_audit(
    *,
    specs: list[T3ExitSpec],
    months: list[str],
    symbols: list[str],
    timeframe: str,
    initial_balance: float,
    early_horizon_seconds: int,
    reentry_fill_policy: str,
) -> tuple[list[T3ExposureSummary], list[T3ExposureSummary], pd.DataFrame]:
    """Run exposure audit for all candidates."""
    silo_rows: list[T3ExposureSummary] = []
    candidate_rows: list[T3ExposureSummary] = []
    trade_parts = []

    for spec in specs:
        candidate_trade_parts = []
        calendar_returns = []
        total_trades = 0
        for symbol in symbols:
            for month in months:
                logger.info("Running T3 exposure audit %s %s %s", spec.label, symbol, month)
                silo, trades = run_audit_silo(
                    spec=spec,
                    symbol=symbol,
                    month=month,
                    timeframe=timeframe,
                    initial_balance=initial_balance,
                    early_horizon_seconds=early_horizon_seconds,
                    reentry_fill_policy=reentry_fill_policy,
                )
                silo_rows.append(silo)
                calendar_returns.append(silo.calendar_silo_sum_pct)
                total_trades += silo.total_trades
                if not trades.empty:
                    candidate_trade_parts.append(trades)
                    trade_parts.append(trades)

        candidate_trades = (
            pd.concat(candidate_trade_parts, ignore_index=True) if candidate_trade_parts else pd.DataFrame()
        )
        candidate_rows.append(
            summarize_t3_exposure(
                candidate=spec.label,
                t3_exit_overrides=spec.t3_exit_overrides,
                scope="aggregate",
                calendar_returns=calendar_returns,
                total_trades=total_trades,
                trades=candidate_trades,
            )
        )

    all_trades = pd.concat(trade_parts, ignore_index=True) if trade_parts else pd.DataFrame()
    return silo_rows, candidate_rows, all_trades


def _write_markdown(
    *,
    output_dir: Path,
    months: list[str],
    symbols: list[str],
    timeframe: str,
    reentry_fill_policy: str,
    candidate_rows: list[T3ExposureSummary],
    silo_rows: list[T3ExposureSummary],
) -> None:
    lines = [
        "# T3 Lifecycle Exposure Audit",
        "",
        "Research-only exposure and final-mark sensitivity audit for strict T3 lifecycle candidates.",
        "",
        f"- Timeframe: `{timeframe}`",
        f"- Reentry fill policy: `{reentry_fill_policy}`",
        f"- Months: {', '.join(months)}",
        f"- Symbols: {', '.join(symbols)}",
        "",
        "## Candidate Summary",
        "",
        "| Candidate | Calendar Sum | Worst Silo | Neg Silos | Trades | T3 Trades | T3 PnL | Ex Final Mark | Final Mark | Win Rate | T3 DD | Loss Streak | Avg Hold | P90 Hold | Max Hold | Sum Hold Hours | Worst MAE | Worst PnL |",
        "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|",
    ]
    for row in candidate_rows:
        lines.append(
            f"| `{row.candidate}` | {row.calendar_silo_sum_pct:.6f}% "
            f"| {row.worst_calendar_silo_pct:.6f}% | {row.negative_calendar_silos} "
            f"| {row.total_trades} | {row.t3_trades} | {row.t3_net_pnl_pct:.6f}% "
            f"| {row.t3_net_pnl_ex_final_mark_pct:.6f}% "
            f"| {row.final_mark_pnl_pct:.6f}%/{row.final_mark_trades} "
            f"| {row.t3_win_rate_pct:.2f}% | {row.t3_equity_max_dd_pct:.6f}% "
            f"| {row.t3_max_loss_streak} | {row.t3_avg_hold_seconds:.2f}s "
            f"| {row.t3_p90_hold_seconds:.2f}s | {row.t3_max_hold_seconds:.2f}s "
            f"| {row.t3_sum_hold_hours:.2f} | {row.t3_worst_mae_bps:.4f}bp "
            f"| {row.t3_worst_pnl_bps:.4f}bp |"
        )

    lines.extend(
        [
            "",
            "## Silo Detail",
            "",
            "| Candidate | Silo | Calendar Return | T3 Trades | T3 PnL | Ex Final Mark | Final Mark | T3 DD | Max Hold | Worst MAE | Exit Reasons |",
            "|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|",
        ]
    )
    for row in silo_rows:
        lines.append(
            f"| `{row.candidate}` | `{row.scope}` | {row.calendar_silo_sum_pct:.6f}% "
            f"| {row.t3_trades} | {row.t3_net_pnl_pct:.6f}% "
            f"| {row.t3_net_pnl_ex_final_mark_pct:.6f}% "
            f"| {row.final_mark_pnl_pct:.6f}%/{row.final_mark_trades} "
            f"| {row.t3_equity_max_dd_pct:.6f}% | {row.t3_max_hold_seconds:.2f}s "
            f"| {row.t3_worst_mae_bps:.4f}bp "
            f"| `{json.dumps(row.t3_exit_reasons, sort_keys=True)}` |"
        )

    (output_dir / "t3_lifecycle_exposure_audit_report.md").write_text(
        "\n".join(lines),
        encoding="utf-8",
    )


def write_outputs(
    *,
    output_dir: Path,
    months: list[str],
    symbols: list[str],
    timeframe: str,
    reentry_fill_policy: str,
    specs: list[T3ExitSpec],
    silo_rows: list[T3ExposureSummary],
    candidate_rows: list[T3ExposureSummary],
    trades: pd.DataFrame,
) -> None:
    output_dir.mkdir(parents=True, exist_ok=True)
    payload = {
        "note": (
            "Research-only T3 exposure audit. FinalMarkToMarket-adjusted PnL "
            "is included because month-end mark rows are not normal stop exits."
        ),
        "timeframe": timeframe,
        "reentry_fill_policy": reentry_fill_policy,
        "calendar_grid": {
            "months": months,
            "symbols": symbols,
            "symbol_months": len(months) * len(symbols),
        },
        "candidate_specs": [asdict(spec) for spec in specs],
        "candidates": [asdict(row) for row in candidate_rows],
        "silos": [asdict(row) for row in silo_rows],
    }
    (output_dir / "t3_lifecycle_exposure_audit_summary.json").write_text(
        json.dumps(payload, indent=2, ensure_ascii=False),
        encoding="utf-8",
    )
    if not trades.empty:
        trades.to_csv(output_dir / "t3_lifecycle_exposure_audit_trades.csv", index=False)
    pd.DataFrame([asdict(row) for row in candidate_rows]).to_csv(
        output_dir / "t3_lifecycle_exposure_audit_candidates.csv",
        index=False,
    )
    _write_markdown(
        output_dir=output_dir,
        months=months,
        symbols=symbols,
        timeframe=timeframe,
        reentry_fill_policy=reentry_fill_policy,
        candidate_rows=candidate_rows,
        silo_rows=silo_rows,
    )


def main() -> None:
    parser = argparse.ArgumentParser(description="T3 lifecycle exposure audit")
    parser.add_argument("--output-dir", default="/tmp/bktrader_t3_lifecycle_exposure_audit")
    parser.add_argument("--candidate-set", choices=["compact"], default="compact")
    parser.add_argument("--labels", nargs="+", default=["t3_min_hold_sl_60m"])
    parser.add_argument("--months", nargs="+", default=DEFAULT_MONTHS)
    parser.add_argument("--symbols", nargs="+", default=DEFAULT_SYMBOLS)
    parser.add_argument("--timeframe", default="1h")
    parser.add_argument("--initial-balance", type=float, default=INITIAL_BALANCE)
    parser.add_argument("--early-horizon-seconds", type=int, default=300)
    parser.add_argument(
        "--reentry-fill-policy",
        choices=["historical", "strict_next_second_cross"],
        default="strict_next_second_cross",
    )
    args = parser.parse_args()

    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
    )

    specs = filter_exit_specs(build_exit_specs(str(args.candidate_set)), args.labels)
    months = [str(value) for value in args.months]
    symbols = [str(value) for value in args.symbols]
    silo_rows, candidate_rows, trades = run_exposure_audit(
        specs=specs,
        months=months,
        symbols=symbols,
        timeframe=str(args.timeframe),
        initial_balance=float(args.initial_balance),
        early_horizon_seconds=int(args.early_horizon_seconds),
        reentry_fill_policy=str(args.reentry_fill_policy),
    )
    output_dir = Path(args.output_dir)
    write_outputs(
        output_dir=output_dir,
        months=months,
        symbols=symbols,
        timeframe=str(args.timeframe),
        reentry_fill_policy=str(args.reentry_fill_policy),
        specs=specs,
        silo_rows=silo_rows,
        candidate_rows=candidate_rows,
        trades=trades,
    )

    best = max(candidate_rows, key=lambda row: row.t3_net_pnl_pct)
    print(
        f"Best T3 exposure candidate: {best.candidate} "
        f"t3_pnl={best.t3_net_pnl_pct:.6f}%, "
        f"ex_final_mark={best.t3_net_pnl_ex_final_mark_pct:.6f}%, "
        f"dd={best.t3_equity_max_dd_pct:.6f}%, "
        f"max_hold={best.t3_max_hold_seconds:.2f}s"
    )
    print(f"Reports: {output_dir}/t3_lifecycle_exposure_audit_{{summary.json,report.md,candidates.csv,trades.csv}}")


if __name__ == "__main__":
    main()
