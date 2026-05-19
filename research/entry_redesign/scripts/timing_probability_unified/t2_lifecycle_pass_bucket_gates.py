"""Lifecycle pass-bucket attribution and stricter T2 gate sweep.

Research-only. This is the next bridge after ctx4h skip-fail sizing: the fail
bucket is mostly removable, but the context-pass/full-size T2 bucket still
bleeds. This runner keeps strict next-second fills and T3 60m SL hold, then
tests whether simple signal-time context gates can shrink that pass bucket.
"""

from __future__ import annotations

import argparse
import json
import logging
import sys
import time
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Any

import numpy as np
import pandas as pd

PROJECT_ROOT = Path(__file__).resolve().parents[4]
RESEARCH_DIR = PROJECT_ROOT / "research"
_SCRIPTS_DIR = Path(__file__).resolve().parents[1]
for path in (RESEARCH_DIR, _SCRIPTS_DIR):
    if str(path) not in sys.path:
        sys.path.insert(0, str(path))

import eth_q1_breakout_t3_shape_compare as lifecycle  # noqa: E402
from timing_probability_unified.t2_lifecycle_context_sizing import (  # noqa: E402
    EXTENDED_MONTHS,
    T2LifecycleSizingSilo,
    T2LifecycleSizingSpec,
    compute_deltas,
    compute_metrics,
    run_sizing_validation,
    summarize_t2_size_multiplier_attribution,
)
from timing_probability_unified.t3_pre_touch_lifecycle import (  # noqa: E402
    DEFAULT_SYMBOLS,
    INITIAL_BALANCE,
    T3_REENTRY_SIZE_SCHEDULE,
    _load_window_bars,
    _month_bounds,
    _net_pnl_pct,
    _patched_replay_kwargs,
    _shape_attr,
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

BASE_CTX4H_FILTERS = {
    "max_atr_percentile": 40.0,
    "min_ctx_side_return_atr": 0.0,
    "ctx_return_lookback_bars": 4,
}
T3_60M_EXIT_OVERRIDES = {"min_hold_seconds_before_sl": 3600.0}
ENTRY_METADATA_COLUMNS = [
    "side",
    "signal_start",
    "signal_bar_index",
    "breakout_level",
    "breakout_pre_touch_seconds",
    "breakout_extension_atr",
    "level_to_signal_open_atr",
    "atr",
    "atr_percentile",
    "ctx4h_side_return_atr",
    "ctx12h_side_return_atr",
    "prev1_range_atr",
    "prev1_close_pos_side",
    "sizing_reject_reason",
]


@dataclass(frozen=True)
class GateCandidate:
    """Human-readable gate spec wrapper."""

    label: str
    filters: dict[str, Any]
    read: str


def _base_filters(**updates: Any) -> dict[str, Any]:
    out = dict(BASE_CTX4H_FILTERS)
    out.update(updates)
    return out


def build_gate_candidates(candidate_set: str) -> list[GateCandidate]:
    """Build bounded pass-bucket candidate gates."""
    candidates = [
        GateCandidate(
            "ctx4h_skipfail_t3_60m",
            _base_filters(),
            "current executable reference: fail context skips lock; pass context trades full size",
        ),
        GateCandidate(
            "ctx4h_min010_skipfail_t3_60m",
            _base_filters(min_ctx_side_return_atr=0.10),
            "requires stronger 4h side-normalized drift before allowing original_t2",
        ),
        GateCandidate(
            "ctx4h_min020_skipfail_t3_60m",
            _base_filters(min_ctx_side_return_atr=0.20),
            "medium 4h continuation threshold",
        ),
        GateCandidate(
            "ctx4h_min030_skipfail_t3_60m",
            _base_filters(min_ctx_side_return_atr=0.30),
            "hard 4h continuation threshold",
        ),
        GateCandidate(
            "atr_max30_skipfail_t3_60m",
            _base_filters(max_atr_percentile=30.0),
            "restricts original_t2 to lower-volatility signal bars",
        ),
        GateCandidate(
            "atr_max25_skipfail_t3_60m",
            _base_filters(max_atr_percentile=25.0),
            "harder low-ATR pass bucket",
        ),
        GateCandidate(
            "pretouch_max900_skipfail_t3_60m",
            _base_filters(max_pre_touch_seconds=900.0),
            "mirrors the T3 touch-timing cap for original_t2 and removes very late signal-bar touches",
        ),
        GateCandidate(
            "pretouch_max300_skipfail_t3_60m",
            _base_filters(max_pre_touch_seconds=300.0),
            "requires touch within first five minutes of the signal bar",
        ),
        GateCandidate(
            "pretouch_max600_skipfail_t3_60m",
            _base_filters(max_pre_touch_seconds=600.0),
            "requires touch within first ten minutes of the signal bar",
        ),
        GateCandidate(
            "extension_max005_skipfail_t3_60m",
            _base_filters(max_breakout_extension_atr=0.05),
            "avoids already-extended touch seconds",
        ),
        GateCandidate(
            "extension_max010_skipfail_t3_60m",
            _base_filters(max_breakout_extension_atr=0.10),
            "looser extension cap",
        ),
        GateCandidate(
            "ctx12h_min000_skipfail_t3_60m",
            _base_filters(min_ctx12h_side_return_atr=0.0),
            "keeps 4h pass and also requires 12h context not opposing the side",
        ),
        GateCandidate(
            "ctx12h_min010_skipfail_t3_60m",
            _base_filters(min_ctx12h_side_return_atr=0.10),
            "requires modest 12h side-normalized drift",
        ),
        GateCandidate(
            "ctx4h_min010_ctx12h_min000_skipfail_t3_60m",
            _base_filters(min_ctx_side_return_atr=0.10, min_ctx12h_side_return_atr=0.0),
            "combines stronger 4h drift with non-opposing 12h context",
        ),
        GateCandidate(
            "atr_max30_ctx12h_min000_skipfail_t3_60m",
            _base_filters(max_atr_percentile=30.0, min_ctx12h_side_return_atr=0.0),
            "low-ATR pass bucket plus non-opposing 12h context",
        ),
        GateCandidate(
            "long_only_skipfail_t3_60m",
            _base_filters(allowed_sides=["long"]),
            "side attribution candidate: long original_t2 only",
        ),
        GateCandidate(
            "short_only_skipfail_t3_60m",
            _base_filters(allowed_sides=["short"]),
            "side attribution candidate: short original_t2 only",
        ),
        GateCandidate(
            "t2_disabled_t3_60m",
            {"allowed_sides": []},
            "strict lifecycle floor: disable original_t2 and keep only T3 60m behavior",
        ),
    ]
    if candidate_set == "focused":
        return candidates
    if candidate_set == "smoke":
        labels = {
            "ctx4h_skipfail_t3_60m",
            "ctx4h_min020_skipfail_t3_60m",
            "ctx12h_min000_skipfail_t3_60m",
        }
        return [candidate for candidate in candidates if candidate.label in labels]
    raise ValueError(f"unknown candidate set: {candidate_set}")


def _to_sizing_spec(candidate: GateCandidate) -> T2LifecycleSizingSpec:
    return T2LifecycleSizingSpec(
        label=candidate.label,
        shape_sizing_filters=dict(candidate.filters),
        fail_multiplier=0.0,
        t3_exit_overrides=dict(T3_60M_EXIT_OVERRIDES),
        sizing_filter_fail_action="skip_lock",
    )


def _run_ledger_for_spec(
    *,
    spec: T2LifecycleSizingSpec,
    symbol: str,
    month: str,
    timeframe: str,
    initial_balance: float,
    reentry_fill_policy: str,
) -> tuple[pd.DataFrame, dict[str, Any]]:
    start, end = _month_bounds(month)
    second_bars = _load_window_bars(symbol, start, end)
    _, signal = lifecycle.build_signal_frame(second_bars, timeframe)
    with _patched_replay_kwargs(symbol):
        ledger, diagnostics = lifecycle.run_second_bar_replay(
            second_bars,
            signal,
            initial_balance=initial_balance,
            breakout_shape="baseline_plus_t3",
            replay_mode="live_intrabar_sma5",
            t3_reentry_size_schedule=T3_REENTRY_SIZE_SCHEDULE,
            t3_cooldown_bars=0,
            t3_quality_filters={"max_pre_touch_seconds": 900.0},
            quality_filter_shapes=["t3_swing"],
            shape_sizing_filters=dict(spec.shape_sizing_filters),
            sizing_filter_shapes=["original_t2"],
            sizing_filter_fail_multiplier=float(spec.fail_multiplier),
            sizing_filter_fail_action=str(spec.sizing_filter_fail_action),
            t3_exit_overrides=dict(spec.t3_exit_overrides),
            reentry_fill_policy=reentry_fill_policy,
        )
    return ledger, diagnostics


def _flat_counts(nested: dict[str, Any]) -> dict[str, int]:
    out: dict[str, int] = {}
    for side_counts in nested.values():
        for reason, count in dict(side_counts).items():
            out[str(reason)] = int(out.get(str(reason), 0)) + int(count)
    return out


def _silo_from_ledger(
    *,
    spec: T2LifecycleSizingSpec,
    symbol: str,
    month: str,
    ledger: pd.DataFrame,
    diagnostics: dict[str, Any],
    initial_balance: float,
    elapsed_seconds: float,
) -> T2LifecycleSizingSilo:
    summary = lifecycle.summarize_run(ledger, initial_balance)
    attribution = lifecycle.summarize_breakout_attribution(ledger)
    t2_attr = _shape_attr(attribution, "original_t2")
    t3_attr = _shape_attr(attribution, "t3_swing")
    sizing_fails = _flat_counts(diagnostics.get("shape_sizing_filter_fails", {}))
    return T2LifecycleSizingSilo(
        candidate=spec.label,
        shape_sizing_filters=dict(spec.shape_sizing_filters),
        fail_multiplier=float(spec.fail_multiplier),
        t3_exit_overrides=dict(spec.t3_exit_overrides),
        sizing_filter_fail_action=str(spec.sizing_filter_fail_action),
        symbol=symbol,
        month=month,
        return_pct=round(float(summary["return_pct"]), 6),
        final_balance=round(float(summary["final_balance"]), 2),
        total_trades=int(summary["trades"]),
        t2_trades=int(t2_attr.get("trades", 0)),
        t3_trades=int(t3_attr.get("trades", 0)),
        t2_net_pnl_pct=round(_net_pnl_pct(attribution, "original_t2", initial_balance), 6),
        t3_net_pnl_pct=round(_net_pnl_pct(attribution, "t3_swing", initial_balance), 6),
        t2_size_multiplier_attribution=summarize_t2_size_multiplier_attribution(ledger, initial_balance),
        t2_size_filter_fails=int(sum(sizing_fails.values())),
        t2_size_filter_reasons=sizing_fails,
        reentry_fill_rejects=dict(diagnostics.get("reentry_fill_rejects", {})),
        elapsed_seconds=round(float(elapsed_seconds), 2),
    )


def _pair_t2_trades(
    ledger: pd.DataFrame,
    *,
    candidate: str,
    symbol: str,
    month: str,
    initial_balance: float,
) -> list[dict[str, Any]]:
    if ledger.empty:
        return []
    rows: list[dict[str, Any]] = []
    open_entry = None
    for _, row in ledger.iterrows():
        if row["type"] in {"BUY", "SHORT"}:
            open_entry = row
            continue
        if row["type"] != "EXIT" or open_entry is None:
            continue
        if str(open_entry.get("breakout_shape_name", "")) != "original_t2":
            open_entry = None
            continue

        side_mult = 1.0 if open_entry["type"] == "BUY" else -1.0
        entry_price = float(open_entry["price"])
        exit_price = float(row["price"])
        notional = float(open_entry.get("notional", 0.0))
        gross_value = (
            side_mult * (exit_price - entry_price) / entry_price * notional
            if entry_price > 0.0 and notional > 0.0
            else 0.0
        )
        fee_value = notional * 0.002
        entry_time = pd.Timestamp(open_entry["time"])
        exit_time = pd.Timestamp(row["time"])
        item = {
            "candidate": candidate,
            "symbol": symbol,
            "month": month,
            "entry_time": entry_time.isoformat(),
            "exit_time": exit_time.isoformat(),
            "hold_seconds": float((exit_time - entry_time).total_seconds()),
            "entry_type": str(open_entry["type"]),
            "entry_reason": str(open_entry["reason"]),
            "exit_reason": str(row["reason"]),
            "entry_price": entry_price,
            "exit_price": exit_price,
            "size_multiplier": float(open_entry.get("size_multiplier", 1.0)),
            "notional_pct": notional / initial_balance * 100.0,
            "gross_pnl_pct": gross_value / initial_balance * 100.0,
            "fee_pct": fee_value / initial_balance * 100.0,
            "net_after_fee_pct": (gross_value - fee_value) / initial_balance * 100.0,
        }
        for column in ENTRY_METADATA_COLUMNS:
            value = open_entry.get(column, np.nan)
            if isinstance(value, pd.Timestamp):
                value = value.isoformat()
            item[column] = value
        rows.append(item)
        open_entry = None
    return rows


def collect_reference_trades(
    *,
    spec: T2LifecycleSizingSpec,
    months: list[str],
    symbols: list[str],
    timeframe: str,
    initial_balance: float,
    reentry_fill_policy: str,
) -> pd.DataFrame:
    """Collect original_t2 trade pairs for the reference gate."""
    all_rows: list[dict[str, Any]] = []
    for symbol in symbols:
        for month in months:
            logger.info("Collecting T2 pass-bucket trades %s %s %s", spec.label, symbol, month)
            ledger, _ = _run_ledger_for_spec(
                spec=spec,
                symbol=symbol,
                month=month,
                timeframe=timeframe,
                initial_balance=initial_balance,
                reentry_fill_policy=reentry_fill_policy,
            )
            all_rows.extend(
                _pair_t2_trades(
                    ledger,
                    candidate=spec.label,
                    symbol=symbol,
                    month=month,
                    initial_balance=initial_balance,
                )
            )
    return pd.DataFrame(all_rows)


def run_single_candidate_with_trades(
    *,
    spec: T2LifecycleSizingSpec,
    months: list[str],
    symbols: list[str],
    timeframe: str,
    initial_balance: float,
    reentry_fill_policy: str,
) -> tuple[list[T2LifecycleSizingSilo], list[Any], pd.DataFrame]:
    """Run one candidate once per silo, collecting both metrics and trade pairs."""
    silos: list[T2LifecycleSizingSilo] = []
    trade_rows: list[dict[str, Any]] = []
    for symbol in symbols:
        for month in months:
            logger.info("Running T2 pass-bucket reference %s %s %s", spec.label, symbol, month)
            started = time.time()
            ledger, diagnostics = _run_ledger_for_spec(
                spec=spec,
                symbol=symbol,
                month=month,
                timeframe=timeframe,
                initial_balance=initial_balance,
                reentry_fill_policy=reentry_fill_policy,
            )
            silos.append(
                _silo_from_ledger(
                    spec=spec,
                    symbol=symbol,
                    month=month,
                    ledger=ledger,
                    diagnostics=diagnostics,
                    initial_balance=initial_balance,
                    elapsed_seconds=time.time() - started,
                )
            )
            trade_rows.extend(
                _pair_t2_trades(
                    ledger,
                    candidate=spec.label,
                    symbol=symbol,
                    month=month,
                    initial_balance=initial_balance,
                )
            )
    metrics = [compute_metrics(spec, silos, months, symbols)]
    return silos, metrics, pd.DataFrame(trade_rows)


def _bucket_label(value: Any) -> str:
    if pd.isna(value):
        return "missing"
    return str(value)


def _numeric_bucket(series: pd.Series, bins: list[float], labels: list[str]) -> pd.Series:
    numeric = pd.to_numeric(series, errors="coerce")
    bucketed = pd.cut(numeric, bins=bins, labels=labels, include_lowest=True, right=False)
    return bucketed.astype(object).where(bucketed.notna(), "missing").astype(str)


def _stats(group: pd.DataFrame) -> dict[str, Any]:
    pnl = pd.to_numeric(group["net_after_fee_pct"], errors="coerce").fillna(0.0)
    gross = pd.to_numeric(group["gross_pnl_pct"], errors="coerce").fillna(0.0)
    fees = pd.to_numeric(group["fee_pct"], errors="coerce").fillna(0.0)
    notional = pd.to_numeric(group["notional_pct"], errors="coerce").fillna(0.0)
    return {
        "trades": int(len(group)),
        "net_after_fee_pct": round(float(pnl.sum()), 6),
        "gross_pnl_pct": round(float(gross.sum()), 6),
        "fee_pct": round(float(fees.sum()), 6),
        "avg_net_after_fee_pct": round(float(pnl.mean()), 6) if len(group) else 0.0,
        "win_rate_pct": round(float((pnl > 0.0).mean()) * 100.0, 2) if len(group) else 0.0,
        "worst_trade_pct": round(float(pnl.min()), 6) if len(group) else 0.0,
        "notional_pct": round(float(notional.sum()), 6),
    }


def _bucket_rows(trades: pd.DataFrame, family: str, bucket_values: pd.Series) -> list[dict[str, Any]]:
    rows = []
    frame = trades.copy()
    frame["_bucket"] = bucket_values.map(_bucket_label)
    for bucket, group in frame.groupby("_bucket", dropna=False):
        rows.append({"family": family, "bucket": str(bucket), **_stats(group)})
    return rows


def summarize_reference_buckets(trades: pd.DataFrame) -> pd.DataFrame:
    """Summarize original_t2 reference trades by currently available features."""
    if trades.empty:
        return pd.DataFrame()
    pass_trades = trades[pd.to_numeric(trades["size_multiplier"], errors="coerce").fillna(1.0) >= 0.999].copy()
    rows = [{"family": "all_original_t2", "bucket": "all", **_stats(trades)}]
    if not pass_trades.empty:
        rows.append({"family": "pass_full_original_t2", "bucket": "all", **_stats(pass_trades)})
        rows.extend(_bucket_rows(pass_trades, "side", pass_trades["side"].astype(str)))
        rows.extend(_bucket_rows(pass_trades, "exit_reason", pass_trades["exit_reason"].astype(str)))
        rows.extend(_bucket_rows(pass_trades, "entry_reason", pass_trades["entry_reason"].astype(str)))
        rows.extend(
            _bucket_rows(
                pass_trades,
                "atr_percentile",
                _numeric_bucket(
                    pass_trades["atr_percentile"],
                    [-np.inf, 10.0, 20.0, 30.0, 40.0, np.inf],
                    ["<10", "10-20", "20-30", "30-40", ">=40"],
                ),
            )
        )
        rows.extend(
            _bucket_rows(
                pass_trades,
                "ctx4h_side_return_atr",
                _numeric_bucket(
                    pass_trades["ctx4h_side_return_atr"],
                    [-np.inf, 0.0, 0.10, 0.20, 0.30, 0.50, np.inf],
                    ["<0", "0-0.10", "0.10-0.20", "0.20-0.30", "0.30-0.50", ">=0.50"],
                ),
            )
        )
        rows.extend(
            _bucket_rows(
                pass_trades,
                "ctx12h_side_return_atr",
                _numeric_bucket(
                    pass_trades["ctx12h_side_return_atr"],
                    [-np.inf, -0.50, 0.0, 0.10, 0.30, np.inf],
                    ["<-0.50", "-0.50-0", "0-0.10", "0.10-0.30", ">=0.30"],
                ),
            )
        )
        rows.extend(
            _bucket_rows(
                pass_trades,
                "pre_touch_seconds",
                _numeric_bucket(
                    pass_trades["breakout_pre_touch_seconds"],
                    [-np.inf, 60.0, 180.0, 300.0, 600.0, 900.0, np.inf],
                    ["<60", "60-180", "180-300", "300-600", "600-900", ">=900"],
                ),
            )
        )
        rows.extend(
            _bucket_rows(
                pass_trades,
                "breakout_extension_atr",
                _numeric_bucket(
                    pass_trades["breakout_extension_atr"],
                    [-np.inf, 0.02, 0.05, 0.10, 0.20, np.inf],
                    ["<0.02", "0.02-0.05", "0.05-0.10", "0.10-0.20", ">=0.20"],
                ),
            )
        )
    return pd.DataFrame(rows).sort_values(["family", "net_after_fee_pct", "bucket"]).reset_index(drop=True)


def write_outputs(
    *,
    output_dir: Path,
    candidates: list[GateCandidate],
    metrics: list[Any],
    silos: list[Any],
    reference_trades: pd.DataFrame,
    bucket_summary: pd.DataFrame,
    months: list[str],
    symbols: list[str],
    timeframe: str,
    reentry_fill_policy: str,
    elapsed_seconds: float,
) -> None:
    output_dir.mkdir(parents=True, exist_ok=True)
    deltas = compute_deltas(metrics)
    reads = {candidate.label: candidate.read for candidate in candidates}
    payload = {
        "note": (
            "Research-only pass-bucket lifecycle sweep. All candidates use strict next-second reentry fills, "
            "skip failed original_t2 locks, and T3 min-hold-before-SL=60m."
        ),
        "timeframe": timeframe,
        "reentry_fill_policy": reentry_fill_policy,
        "calendar_grid": {
            "months": months,
            "symbols": symbols,
            "symbol_months": len(months) * len(symbols),
        },
        "elapsed_seconds": round(float(elapsed_seconds), 2),
        "candidate_reads": reads,
        "candidate_specs": [
            {
                "label": candidate.label,
                "filters": candidate.filters,
                "read": candidate.read,
            }
            for candidate in candidates
        ],
        "candidates": [asdict(row) for row in metrics],
        "deltas_vs_first_candidate": [asdict(row) for row in deltas],
        "silos": [asdict(row) for row in silos],
        "reference_bucket_summary": bucket_summary.to_dict(orient="records")
        if not bucket_summary.empty
        else [],
    }
    (output_dir / "t2_lifecycle_pass_bucket_gate_summary.json").write_text(
        json.dumps(payload, indent=2, ensure_ascii=False),
        encoding="utf-8",
    )
    reference_trades.to_csv(output_dir / "t2_lifecycle_pass_bucket_reference_trades.csv", index=False)
    bucket_summary.to_csv(output_dir / "t2_lifecycle_pass_bucket_reference_buckets.csv", index=False)

    lines = [
        "# T2 Lifecycle Pass-Bucket Gates",
        "",
        "Research-only strict lifecycle sweep for shrinking the original_t2 full-size pass bucket.",
        "",
        f"- Timeframe: `{timeframe}`",
        f"- Reentry fill policy: `{reentry_fill_policy}`",
        f"- Months: {', '.join(months)}",
        f"- Symbols: {', '.join(symbols)}",
        "",
        "## Candidate Summary",
        "",
        "| Candidate | Calendar Sum | Delta | Worst Silo | Neg Silos | Trades | T2 Trades | T2 Net PnL | T3 Net PnL | Filters | Read |",
        "|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|",
    ]
    delta_by_candidate = {row.candidate: row for row in deltas}
    for row in metrics:
        delta = delta_by_candidate.get(row.candidate)
        delta_text = f"{delta.calendar_silo_sum_delta_pct:+.6f}%" if delta is not None else "baseline"
        lines.append(
            f"| `{row.candidate}` | {row.calendar_silo_sum_pct:.6f}% "
            f"| {delta_text} "
            f"| {row.worst_calendar_silo_pct:.6f}% "
            f"| {row.negative_calendar_silos} "
            f"| {row.total_trades} "
            f"| {row.t2_trades} "
            f"| {row.t2_net_pnl_pct:.6f}% "
            f"| {row.t3_net_pnl_pct:.6f}% "
            f"| `{json.dumps(row.shape_sizing_filters, sort_keys=True)}` "
            f"| {reads.get(row.candidate, '')} |"
        )

    if not bucket_summary.empty:
        top_bad = bucket_summary.sort_values("net_after_fee_pct").head(18)
        lines.extend(
            [
                "",
                "## Reference Pass-Bucket Attribution",
                "",
                "| Family | Bucket | Trades | Net After Fee | Gross | Fee | Win Rate | Worst Trade | Notional |",
                "|---|---|---:|---:|---:|---:|---:|---:|---:|",
            ]
        )
        for row in top_bad.itertuples(index=False):
            lines.append(
                f"| `{row.family}` | `{row.bucket}` | {int(row.trades)} "
                f"| {float(row.net_after_fee_pct):.6f}% "
                f"| {float(row.gross_pnl_pct):.6f}% "
                f"| {float(row.fee_pct):.6f}% "
                f"| {float(row.win_rate_pct):.2f}% "
                f"| {float(row.worst_trade_pct):.6f}% "
                f"| {float(row.notional_pct):.6f}% |"
            )

    lines.extend(
        [
            "",
            "## Read",
            "",
            "- These results are promotion-comparable lifecycle returns, not adverse10 event-ledger returns.",
            "- A candidate only matters if it improves calendar sum without creating a worse worst-silo profile or collapsing T3 contribution.",
            "- Exact low_eff/RF remains a separate hook because lifecycle replay still needs event-time speed/efficiency/RF features.",
        ]
    )
    (output_dir / "t2_lifecycle_pass_bucket_gate_report.md").write_text(
        "\n".join(lines) + "\n",
        encoding="utf-8",
    )


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--candidate-set", choices=["focused", "smoke"], default="focused")
    parser.add_argument("--labels", nargs="*", default=None)
    parser.add_argument("--months", nargs="+", default=EXTENDED_MONTHS)
    parser.add_argument("--symbols", nargs="+", default=DEFAULT_SYMBOLS)
    parser.add_argument("--timeframe", default="1h")
    parser.add_argument("--initial-balance", type=float, default=INITIAL_BALANCE)
    parser.add_argument("--reentry-fill-policy", default="strict_next_second_cross")
    parser.add_argument(
        "--output-dir",
        type=Path,
        default=OUTPUT_DIR / "t2_pass_bucket_gate_sweep_extended",
    )
    parser.add_argument("--log-level", default="INFO")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    logging.basicConfig(
        level=getattr(logging, str(args.log_level).upper(), logging.INFO),
        format="%(asctime)s %(levelname)s %(message)s",
    )
    candidates = build_gate_candidates(args.candidate_set)
    if args.labels:
        lookup = {candidate.label: candidate for candidate in candidates}
        missing = [label for label in args.labels if label not in lookup]
        if missing:
            raise ValueError(f"unknown labels: {', '.join(missing)}")
        candidates = [lookup[label] for label in args.labels]
    specs = [_to_sizing_spec(candidate) for candidate in candidates]
    started = time.time()
    if len(specs) == 1:
        silos, metrics, reference_trades = run_single_candidate_with_trades(
            spec=specs[0],
            months=list(args.months),
            symbols=list(args.symbols),
            timeframe=str(args.timeframe),
            initial_balance=float(args.initial_balance),
            reentry_fill_policy=str(args.reentry_fill_policy),
        )
    else:
        silos, metrics = run_sizing_validation(
            specs=specs,
            months=list(args.months),
            symbols=list(args.symbols),
            timeframe=str(args.timeframe),
            initial_balance=float(args.initial_balance),
            reentry_fill_policy=str(args.reentry_fill_policy),
        )
        reference_trades = collect_reference_trades(
            spec=specs[0],
            months=list(args.months),
            symbols=list(args.symbols),
            timeframe=str(args.timeframe),
            initial_balance=float(args.initial_balance),
            reentry_fill_policy=str(args.reentry_fill_policy),
        )
    bucket_summary = summarize_reference_buckets(reference_trades)
    write_outputs(
        output_dir=Path(args.output_dir),
        candidates=candidates,
        metrics=metrics,
        silos=silos,
        reference_trades=reference_trades,
        bucket_summary=bucket_summary,
        months=list(args.months),
        symbols=list(args.symbols),
        timeframe=str(args.timeframe),
        reentry_fill_policy=str(args.reentry_fill_policy),
        elapsed_seconds=time.time() - started,
    )
    logger.info("Wrote %s", args.output_dir)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
