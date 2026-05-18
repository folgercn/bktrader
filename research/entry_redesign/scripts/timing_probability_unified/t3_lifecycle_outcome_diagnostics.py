"""Trade-level diagnostics for T3 lifecycle outcomes.

This research-only tool runs the stricter ``baseline_plus_t3`` +
``reentry_window`` replay, pairs entry/exit ledger rows, and slices realized T3
outcomes by lifecycle facts such as entry reason, exit reason, side, hold time,
and early adverse excursion. It is intentionally diagnostic: post-entry fields
such as hold time or MAE are not promotion gates by themselves.
"""

from __future__ import annotations

import argparse
import json
import logging
import sys
import time
from dataclasses import asdict, dataclass
from pathlib import Path

import numpy as np
import pandas as pd

PROJECT_ROOT = Path(__file__).resolve().parents[4]
RESEARCH_DIR = PROJECT_ROOT / "research"
_SCRIPTS_DIR = Path(__file__).resolve().parents[1]
for path in (RESEARCH_DIR, _SCRIPTS_DIR):
    if str(path) not in sys.path:
        sys.path.insert(0, str(path))

import eth_q1_breakout_t3_shape_compare as lifecycle  # noqa: E402
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
class DiagnosticSiloSummary:
    """One baseline lifecycle diagnostic replay result."""

    symbol: str
    month: str
    t3_pre_touch_max: float
    return_pct: float
    total_trades: int
    t3_trades: int
    t3_net_pnl_pct: float
    t3_win_rate_pct: float
    t3_sl_exit_rate_pct: float
    t3_avg_hold_seconds: float
    t3_avg_mae_300s_bps: float
    t3_avg_mfe_300s_bps: float
    elapsed_seconds: float


@dataclass
class OutcomeSlice:
    """Aggregate metrics for a T3 trade slice."""

    group_by: str
    bucket: str
    trades: int
    win_rate_pct: float
    net_pnl_pct: float
    avg_pnl_bps: float
    median_pnl_bps: float
    worst_pnl_bps: float
    sl_exit_rate_pct: float
    avg_hold_seconds: float
    avg_mae_300s_bps: float
    avg_mfe_300s_bps: float


@dataclass
class OutcomeOverlay:
    """Diagnostic no-trade overlay measured on realized T3 rows only."""

    label: str
    kept_t3_trades: int
    removed_t3_trades: int
    kept_t3_net_pnl_pct: float
    removed_t3_net_pnl_pct: float
    t3_net_pnl_delta_pct: float
    kept_win_rate_pct: float


def _hold_bucket(hold_seconds: float) -> str:
    if hold_seconds <= 300:
        return "0-5m"
    if hold_seconds <= 900:
        return "5-15m"
    if hold_seconds <= 3600:
        return "15-60m"
    return "60m+"


def _mae_bucket(mae_300s_bps: float) -> str:
    if mae_300s_bps >= 0:
        return "no_adverse"
    if mae_300s_bps >= -5:
        return "0_to_5bps"
    if mae_300s_bps >= -10:
        return "5_to_10bps"
    if mae_300s_bps >= -20:
        return "10_to_20bps"
    return "gt20bps"


def _side_from_entry_type(entry_type: str) -> str:
    if entry_type == "BUY":
        return "long"
    if entry_type == "SHORT":
        return "short"
    return "unknown"


def _excursion_pct(
    *,
    side: str,
    entry_price: float,
    bars: pd.DataFrame,
) -> tuple[float, float]:
    """Return MFE and MAE in percent for a side over a bar window."""
    if bars.empty or entry_price <= 0:
        return 0.0, 0.0
    high = float(bars["high"].max())
    low = float(bars["low"].min())
    if side == "long":
        mfe = (high - entry_price) / entry_price * 100.0
        mae = (low - entry_price) / entry_price * 100.0
    elif side == "short":
        mfe = (entry_price - low) / entry_price * 100.0
        mae = (entry_price - high) / entry_price * 100.0
    else:
        return 0.0, 0.0
    return float(mfe), float(mae)


def pair_lifecycle_trades(
    ledger: pd.DataFrame,
    second_bars: pd.DataFrame,
    *,
    symbol: str,
    month: str,
    initial_balance: float,
    early_horizon_seconds: int = 300,
) -> pd.DataFrame:
    """Pair entry/exit rows and enrich with outcome diagnostics."""
    if ledger.empty:
        return pd.DataFrame()

    bars = second_bars.copy()
    bars.index = pd.to_datetime(bars.index, utc=True)

    rows = []
    open_entry = None
    for _, row in ledger.iterrows():
        row_type = str(row["type"])
        if row_type in {"BUY", "SHORT"}:
            open_entry = row
            continue
        if row_type != "EXIT" or open_entry is None:
            continue

        entry_time = pd.Timestamp(open_entry["time"])
        exit_time = pd.Timestamp(row["time"])
        if entry_time.tzinfo is None:
            entry_time = entry_time.tz_localize("UTC")
        else:
            entry_time = entry_time.tz_convert("UTC")
        if exit_time.tzinfo is None:
            exit_time = exit_time.tz_localize("UTC")
        else:
            exit_time = exit_time.tz_convert("UTC")

        side = _side_from_entry_type(str(open_entry["type"]))
        side_mult = 1.0 if side == "long" else -1.0
        entry_price = float(open_entry["price"])
        exit_price = float(row["price"])
        notional = float(open_entry["notional"])
        pnl_pct = side_mult * (exit_price - entry_price) / entry_price if entry_price > 0 else 0.0
        pnl_value = pnl_pct * notional
        hold_seconds = max(0.0, (exit_time - entry_time).total_seconds())

        trade_window = bars[(bars.index >= entry_time) & (bars.index <= exit_time)]
        early_end = min(exit_time, entry_time + pd.Timedelta(seconds=early_horizon_seconds))
        early_window = bars[(bars.index >= entry_time) & (bars.index <= early_end)]
        mfe_pct, mae_pct = _excursion_pct(side=side, entry_price=entry_price, bars=trade_window)
        mfe_early_pct, mae_early_pct = _excursion_pct(
            side=side,
            entry_price=entry_price,
            bars=early_window,
        )

        rows.append(
            {
                "symbol": symbol,
                "month": month,
                "symbol_month": f"{symbol}:{month}",
                "breakout_shape_name": str(open_entry.get("breakout_shape_name", "")),
                "side": side,
                "symbol_side": f"{symbol}:{side}",
                "entry_time": entry_time.isoformat(),
                "exit_time": exit_time.isoformat(),
                "entry_type": str(open_entry["type"]),
                "entry_reason": str(open_entry["reason"]),
                "side_entry_reason": f"{side}:{open_entry['reason']}",
                "symbol_month_side": f"{symbol}:{month}:{side}",
                "symbol_month_entry_reason": f"{symbol}:{month}:{open_entry['reason']}",
                "symbol_month_side_entry_reason": f"{symbol}:{month}:{side}:{open_entry['reason']}",
                "exit_reason": str(row["reason"]),
                "entry_price": entry_price,
                "exit_price": exit_price,
                "notional": notional,
                "pnl_pct": round(float(pnl_pct * 100.0), 6),
                "pnl_bps": round(float(pnl_pct * 10000.0), 6),
                "pnl_value": round(float(pnl_value), 6),
                "pnl_initial_pct": round(float(pnl_value / initial_balance * 100.0), 6),
                "hold_seconds": round(float(hold_seconds), 3),
                "hold_bucket": _hold_bucket(hold_seconds),
                "mfe_pct": round(float(mfe_pct), 6),
                "mae_pct": round(float(mae_pct), 6),
                "mfe_bps": round(float(mfe_pct * 100.0), 6),
                "mae_bps": round(float(mae_pct * 100.0), 6),
                "mfe_300s_pct": round(float(mfe_early_pct), 6),
                "mae_300s_pct": round(float(mae_early_pct), 6),
                "mfe_300s_bps": round(float(mfe_early_pct * 100.0), 6),
                "mae_300s_bps": round(float(mae_early_pct * 100.0), 6),
                "mae_300s_bucket": _mae_bucket(float(mae_early_pct * 100.0)),
                "outcome": "win" if pnl_value > 0 else "loss",
            }
        )
        open_entry = None

    return pd.DataFrame(rows)


def _t3_trades(trades: pd.DataFrame) -> pd.DataFrame:
    if trades.empty:
        return trades.copy()
    return trades[trades["breakout_shape_name"] == "t3_swing"].copy()


def summarize_outcome_slice(
    trades: pd.DataFrame,
    *,
    group_by: str,
    bucket: str,
) -> OutcomeSlice:
    if trades.empty:
        return OutcomeSlice(
            group_by=group_by,
            bucket=bucket,
            trades=0,
            win_rate_pct=0.0,
            net_pnl_pct=0.0,
            avg_pnl_bps=0.0,
            median_pnl_bps=0.0,
            worst_pnl_bps=0.0,
            sl_exit_rate_pct=0.0,
            avg_hold_seconds=0.0,
            avg_mae_300s_bps=0.0,
            avg_mfe_300s_bps=0.0,
        )

    return OutcomeSlice(
        group_by=group_by,
        bucket=bucket,
        trades=int(len(trades)),
        win_rate_pct=round(float((trades["pnl_initial_pct"] > 0).mean()) * 100.0, 6),
        net_pnl_pct=round(float(trades["pnl_initial_pct"].sum()), 6),
        avg_pnl_bps=round(float(trades["pnl_bps"].mean()), 6),
        median_pnl_bps=round(float(trades["pnl_bps"].median()), 6),
        worst_pnl_bps=round(float(trades["pnl_bps"].min()), 6),
        sl_exit_rate_pct=round(float((trades["exit_reason"] == "SL").mean()) * 100.0, 6),
        avg_hold_seconds=round(float(trades["hold_seconds"].mean()), 6),
        avg_mae_300s_bps=round(float(trades["mae_300s_bps"].mean()), 6),
        avg_mfe_300s_bps=round(float(trades["mfe_300s_bps"].mean()), 6),
    )


def summarize_slices(trades: pd.DataFrame, group_columns: list[str]) -> list[OutcomeSlice]:
    """Summarize T3 trades by each requested column."""
    t3 = _t3_trades(trades)
    slices: list[OutcomeSlice] = [summarize_outcome_slice(t3, group_by="all", bucket="all")]
    for column in group_columns:
        if t3.empty or column not in t3.columns:
            continue
        for bucket, group in t3.groupby(column, dropna=False):
            slices.append(
                summarize_outcome_slice(
                    group,
                    group_by=column,
                    bucket=str(bucket),
                )
            )
    return slices


def compute_diagnostic_overlays(trades: pd.DataFrame) -> list[OutcomeOverlay]:
    """Score simple diagnostic no-trade overlays on realized T3 rows only."""
    t3 = _t3_trades(trades)
    baseline_pnl = float(t3["pnl_initial_pct"].sum()) if not t3.empty else 0.0
    overlay_masks = {
        "keep_all": pd.Series(True, index=t3.index),
        "drop_sl_reentry": t3["entry_reason"] != "SL-Reentry" if not t3.empty else pd.Series(dtype=bool),
        "zero_initial_only": t3["entry_reason"] == "Zero-Initial-Reentry" if not t3.empty else pd.Series(dtype=bool),
        "drop_long": t3["side"] != "long" if not t3.empty else pd.Series(dtype=bool),
        "drop_short": t3["side"] != "short" if not t3.empty else pd.Series(dtype=bool),
    }

    overlays = []
    for label, mask in overlay_masks.items():
        kept = t3[mask].copy() if not t3.empty else t3.copy()
        removed = t3[~mask].copy() if not t3.empty else t3.copy()
        kept_pnl = float(kept["pnl_initial_pct"].sum()) if not kept.empty else 0.0
        removed_pnl = float(removed["pnl_initial_pct"].sum()) if not removed.empty else 0.0
        overlays.append(
            OutcomeOverlay(
                label=label,
                kept_t3_trades=int(len(kept)),
                removed_t3_trades=int(len(removed)),
                kept_t3_net_pnl_pct=round(kept_pnl, 6),
                removed_t3_net_pnl_pct=round(removed_pnl, 6),
                t3_net_pnl_delta_pct=round(kept_pnl - baseline_pnl, 6),
                kept_win_rate_pct=round(float((kept["pnl_initial_pct"] > 0).mean()) * 100.0, 6)
                if not kept.empty
                else 0.0,
            )
        )
    return overlays


def run_diagnostic_silo(
    *,
    symbol: str,
    month: str,
    pre_touch_max: float,
    timeframe: str,
    initial_balance: float,
    early_horizon_seconds: int,
    reentry_fill_policy: str = "strict_next_second_cross",
) -> tuple[DiagnosticSiloSummary, pd.DataFrame]:
    """Run one baseline lifecycle replay and return paired trades."""
    start, end = _month_bounds(month)
    second_bars = _load_window_bars(symbol, start, end)
    _, signal = lifecycle.build_signal_frame(second_bars, timeframe)

    started = time.time()
    with _patched_replay_kwargs(symbol):
        ledger, _diagnostics = lifecycle.run_second_bar_replay(
            second_bars,
            signal,
            initial_balance=initial_balance,
            breakout_shape="baseline_plus_t3",
            replay_mode="live_intrabar_sma5",
            t3_reentry_size_schedule=T3_REENTRY_SIZE_SCHEDULE,
            t3_cooldown_bars=0,
            t3_quality_filters={"max_pre_touch_seconds": float(pre_touch_max)},
            quality_filter_shapes=["t3_swing"],
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
    t3 = _t3_trades(trades)
    t3_pnl = float(t3["pnl_initial_pct"].sum()) if not t3.empty else 0.0

    silo = DiagnosticSiloSummary(
        symbol=symbol,
        month=month,
        t3_pre_touch_max=float(pre_touch_max),
        return_pct=round(float(summary["return_pct"]), 6),
        total_trades=int(summary["trades"]),
        t3_trades=int(len(t3)),
        t3_net_pnl_pct=round(t3_pnl, 6),
        t3_win_rate_pct=round(float((t3["pnl_initial_pct"] > 0).mean()) * 100.0, 6)
        if not t3.empty
        else 0.0,
        t3_sl_exit_rate_pct=round(float((t3["exit_reason"] == "SL").mean()) * 100.0, 6)
        if not t3.empty
        else 0.0,
        t3_avg_hold_seconds=round(float(t3["hold_seconds"].mean()), 6) if not t3.empty else 0.0,
        t3_avg_mae_300s_bps=round(float(t3["mae_300s_bps"].mean()), 6) if not t3.empty else 0.0,
        t3_avg_mfe_300s_bps=round(float(t3["mfe_300s_bps"].mean()), 6) if not t3.empty else 0.0,
        elapsed_seconds=round(time.time() - started, 2),
    )
    return silo, trades


def run_diagnostics(
    *,
    months: list[str],
    symbols: list[str],
    pre_touch_max: float,
    timeframe: str,
    initial_balance: float,
    early_horizon_seconds: int,
    reentry_fill_policy: str = "strict_next_second_cross",
) -> tuple[list[DiagnosticSiloSummary], pd.DataFrame, list[OutcomeSlice], list[OutcomeOverlay]]:
    silos: list[DiagnosticSiloSummary] = []
    trade_parts = []
    for symbol in symbols:
        for month in months:
            logger.info("Running T3 lifecycle outcome diagnostics %s %s", symbol, month)
            silo, trades = run_diagnostic_silo(
                symbol=symbol,
                month=month,
                pre_touch_max=pre_touch_max,
                timeframe=timeframe,
                initial_balance=initial_balance,
                early_horizon_seconds=early_horizon_seconds,
                reentry_fill_policy=reentry_fill_policy,
            )
            silos.append(silo)
            if not trades.empty:
                trade_parts.append(trades)

    all_trades = pd.concat(trade_parts, ignore_index=True) if trade_parts else pd.DataFrame()
    slices = summarize_slices(
        all_trades,
        [
            "symbol",
            "month",
            "symbol_month",
            "side",
            "symbol_side",
            "entry_reason",
            "side_entry_reason",
            "symbol_month_side",
            "symbol_month_entry_reason",
            "symbol_month_side_entry_reason",
            "exit_reason",
            "hold_bucket",
            "mae_300s_bucket",
            "outcome",
        ],
    )
    overlays = compute_diagnostic_overlays(all_trades)
    return silos, all_trades, slices, overlays


def write_outputs(
    *,
    silos: list[DiagnosticSiloSummary],
    trades: pd.DataFrame,
    slices: list[OutcomeSlice],
    overlays: list[OutcomeOverlay],
    output_dir: Path,
    months: list[str],
    symbols: list[str],
    timeframe: str,
    early_horizon_seconds: int,
    reentry_fill_policy: str,
) -> None:
    output_dir.mkdir(parents=True, exist_ok=True)
    t3 = _t3_trades(trades)
    payload = {
        "note": (
            "Research-only T3 lifecycle outcome diagnostics. Post-entry fields "
            "such as hold time and early MAE identify loss modes; they are not "
            "direct promotion gates without a separate executable lifecycle test."
        ),
        "timeframe": timeframe,
        "reentry_fill_policy": reentry_fill_policy,
        "early_horizon_seconds": int(early_horizon_seconds),
        "calendar_grid": {
            "months": months,
            "symbols": symbols,
            "symbol_months": len(months) * len(symbols),
        },
        "aggregate": {
            "t3_trades": int(len(t3)),
            "t3_net_pnl_pct": round(float(t3["pnl_initial_pct"].sum()), 6) if not t3.empty else 0.0,
            "t3_win_rate_pct": round(float((t3["pnl_initial_pct"] > 0).mean()) * 100.0, 6)
            if not t3.empty
            else 0.0,
            "t3_sl_exit_rate_pct": round(float((t3["exit_reason"] == "SL").mean()) * 100.0, 6)
            if not t3.empty
            else 0.0,
        },
        "silos": [asdict(row) for row in silos],
        "slices": [asdict(row) for row in slices],
        "diagnostic_overlays": [asdict(row) for row in overlays],
    }
    (output_dir / "t3_lifecycle_outcome_diagnostics_summary.json").write_text(
        json.dumps(payload, indent=2, ensure_ascii=False),
        encoding="utf-8",
    )

    if not trades.empty:
        trades.to_csv(output_dir / "t3_lifecycle_outcome_trades.csv", index=False)

    lines = [
        "# T3 Lifecycle Outcome Diagnostics",
        "",
        "Research-only realized trade diagnostics. Post-entry fields are loss-mode evidence, not executable gates by themselves.",
        "",
        f"- Timeframe: `{timeframe}`",
        f"- Reentry fill policy: `{reentry_fill_policy}`",
        f"- Early adverse horizon: `{early_horizon_seconds}s`",
        f"- Months: {', '.join(months)}",
        f"- Symbols: {', '.join(symbols)}",
        "",
        "## Silo Summary",
        "",
        "| Symbol | Month | Return | Total Trades | T3 Trades | T3 Net PnL | T3 Win Rate | T3 SL Exit Rate | Avg Hold | Avg MAE 300s | Avg MFE 300s |",
        "|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|",
    ]
    for row in silos:
        lines.append(
            f"| `{row.symbol}` | {row.month} | {row.return_pct:.6f}% "
            f"| {row.total_trades} | {row.t3_trades} | {row.t3_net_pnl_pct:.6f}% "
            f"| {row.t3_win_rate_pct:.2f}% | {row.t3_sl_exit_rate_pct:.2f}% "
            f"| {row.t3_avg_hold_seconds:.2f}s | {row.t3_avg_mae_300s_bps:.4f}bp "
            f"| {row.t3_avg_mfe_300s_bps:.4f}bp |"
        )

    lines.extend(
        [
            "",
            "## Outcome Slices",
            "",
            "| Group | Bucket | Trades | Net PnL | Win Rate | SL Exit Rate | Avg PnL | Worst PnL | Avg Hold | Avg MAE 300s | Avg MFE 300s |",
            "|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|",
        ]
    )
    for row in slices:
        lines.append(
            f"| `{row.group_by}` | `{row.bucket}` | {row.trades} "
            f"| {row.net_pnl_pct:.6f}% | {row.win_rate_pct:.2f}% "
            f"| {row.sl_exit_rate_pct:.2f}% | {row.avg_pnl_bps:.4f}bp "
            f"| {row.worst_pnl_bps:.4f}bp | {row.avg_hold_seconds:.2f}s "
            f"| {row.avg_mae_300s_bps:.4f}bp | {row.avg_mfe_300s_bps:.4f}bp |"
        )

    lines.extend(
        [
            "",
            "## Diagnostic No-Trade Overlays",
            "",
            "| Overlay | Kept T3 Trades | Removed T3 Trades | Kept T3 Net PnL | Removed T3 Net PnL | Delta vs Keep-All | Kept Win Rate |",
            "|---|---:|---:|---:|---:|---:|---:|",
        ]
    )
    for row in overlays:
        lines.append(
            f"| `{row.label}` | {row.kept_t3_trades} | {row.removed_t3_trades} "
            f"| {row.kept_t3_net_pnl_pct:.6f}% | {row.removed_t3_net_pnl_pct:.6f}% "
            f"| {row.t3_net_pnl_delta_pct:+.6f}% | {row.kept_win_rate_pct:.2f}% |"
        )

    (output_dir / "t3_lifecycle_outcome_diagnostics_report.md").write_text(
        "\n".join(lines),
        encoding="utf-8",
    )


def main() -> None:
    parser = argparse.ArgumentParser(description="T3 lifecycle outcome diagnostics")
    parser.add_argument("--output-dir", default="/tmp/bktrader_t3_lifecycle_outcome_diagnostics")
    parser.add_argument("--pre-touch-max", type=float, default=900.0)
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

    silos, trades, slices, overlays = run_diagnostics(
        months=[str(value) for value in args.months],
        symbols=[str(value) for value in args.symbols],
        pre_touch_max=float(args.pre_touch_max),
        timeframe=str(args.timeframe),
        initial_balance=float(args.initial_balance),
        early_horizon_seconds=int(args.early_horizon_seconds),
        reentry_fill_policy=str(args.reentry_fill_policy),
    )
    output_dir = Path(args.output_dir)
    write_outputs(
        silos=silos,
        trades=trades,
        slices=slices,
        overlays=overlays,
        output_dir=output_dir,
        months=[str(value) for value in args.months],
        symbols=[str(value) for value in args.symbols],
        timeframe=str(args.timeframe),
        early_horizon_seconds=int(args.early_horizon_seconds),
        reentry_fill_policy=str(args.reentry_fill_policy),
    )

    t3 = _t3_trades(trades)
    t3_pnl = float(t3["pnl_initial_pct"].sum()) if not t3.empty else 0.0
    worst_slice = min(
        (row for row in slices if row.group_by != "all" and row.trades > 0),
        key=lambda row: row.net_pnl_pct,
        default=None,
    )
    print(
        f"T3 lifecycle diagnostics: trades={len(t3)}, "
        f"t3_net_pnl={t3_pnl:.6f}%, output={output_dir}"
    )
    if worst_slice is not None:
        print(
            f"Worst slice: {worst_slice.group_by}={worst_slice.bucket} "
            f"net_pnl={worst_slice.net_pnl_pct:.6f}%, trades={worst_slice.trades}"
        )


if __name__ == "__main__":
    main()
