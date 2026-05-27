"""Research-only revalidation of lead-side T2 reentry scheduling.

The current ETH pretouch research lead uses the canonical selected-delay
adverse10 lead ledger, then maps submitted lead quantity into the 0.20..0.40
ETH absolute quantity band. This audit asks a narrower question: what if we put
the old T2 ``reentry_window`` lifecycle back on the same canonical lead events,
with ``reentry_size_schedule=[0.20, 0.10]`` and ``max_trades_per_bar=2``?

To keep the comparison scoped, native original_t2 and native t3_swing locks are
disabled. The canonical lead events are injected as external breakout locks and
then replayed through the existing strict lifecycle engine.
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

import pandas as pd

PROJECT_ROOT = Path(__file__).resolve().parents[4]
RESEARCH_DIR = PROJECT_ROOT / "research"
SCRIPTS_DIR = Path(__file__).resolve().parents[1]
for path in (RESEARCH_DIR, SCRIPTS_DIR):
    if str(path) not in sys.path:
        sys.path.insert(0, str(path))

import eth_q1_breakout_t3_shape_compare as lifecycle  # noqa: E402
from timing_probability_unified.t3_pre_touch_lifecycle import (  # noqa: E402
    INITIAL_BALANCE,
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
DEFAULT_LEAD_EVENTS = OUTPUT_DIR / "t3_overlay_lead_exact_exposure" / "lead_exact_adverse10_exposure_windows.csv"
DEFAULT_OUTPUT = OUTPUT_DIR / "t2_reentry_schedule_revalidation_20260526"
EXTERNAL_SHAPE_NAME = "canonical_lead_t2_reentry"
CURRENT_QBAND_LEAD_PCT = 61.07091667649647
BASE_LEAD_ADVERSE10_PCT = 22.97164769930056
REENTRY_SCHEDULE = [0.20, 0.10]


@dataclass
class SiloResult:
    month: str
    return_pct: float
    final_balance: float
    total_trades: int
    external_trades: int
    external_net_pnl_pct: float
    max_dd_pct: float
    entry_reasons: dict[str, int]
    exit_reasons: dict[str, int]
    external_events_available: int
    external_locks: int
    reentry_fill_rejects: dict[str, int]
    elapsed_seconds: float


def _parse_signal_start(event_id: str) -> pd.Timestamp:
    parts = str(event_id).split("|")
    if len(parts) < 2:
        raise ValueError(f"cannot parse signal_start from event_id={event_id!r}")
    return pd.Timestamp(parts[1]).tz_convert("UTC")


def _load_canonical_events(path: Path) -> pd.DataFrame:
    frame = pd.read_csv(path)
    if frame.empty:
        raise ValueError(f"empty lead events: {path}")
    frame = frame.copy()
    frame["event_id"] = frame["event_id"].astype(str)
    frame["event_key"] = frame["event_id"]
    frame["signal_start"] = frame["event_id"].map(_parse_signal_start)
    frame["touch_time"] = pd.to_datetime(frame["touch_time"], utc=True)
    frame["side"] = frame["side"].astype(str).str.lower()
    frame["symbol"] = frame["symbol"].astype(str)
    frame["external_breakout_shape_name"] = EXTERNAL_SHAPE_NAME
    if "level" not in frame.columns:
        frame["level"] = pd.NA
    return frame


def _month_events(events: pd.DataFrame, month: str) -> pd.DataFrame:
    start, end = _month_bounds(month)
    return events[(events["touch_time"] >= start) & (events["touch_time"] <= end)].copy()


def _flat_counts(nested: dict[str, Any]) -> dict[str, int]:
    out: dict[str, int] = {}
    for side_counts in nested.values():
        for reason, count in dict(side_counts).items():
            out[str(reason)] = int(out.get(str(reason), 0)) + int(count)
    return out


def _run_month(
    *,
    events: pd.DataFrame,
    month: str,
    timeframe: str,
    initial_balance: float,
    reentry_fill_policy: str,
    write_ledger: bool,
    output_dir: Path,
) -> SiloResult:
    start, end = _month_bounds(month)
    second_bars = _load_window_bars("ETHUSDT", start, end)
    _, signal = lifecycle.build_signal_frame(second_bars, timeframe)
    month_events = _month_events(events, month)

    started = time.time()
    with _patched_replay_kwargs("ETHUSDT"):
        ledger, diagnostics = lifecycle.run_second_bar_replay(
            second_bars,
            signal,
            initial_balance=initial_balance,
            breakout_shape="baseline_plus_t3",
            replay_mode="live_intrabar_sma5",
            t3_reentry_size_schedule=REENTRY_SCHEDULE,
            t3_cooldown_bars=0,
            t3_quality_filters={"allowed_sides": []},
            quality_filter_shapes=["t3_swing"],
            shape_sizing_filters={"allowed_sides": []},
            sizing_filter_shapes=["original_t2"],
            sizing_filter_fail_multiplier=0.0,
            sizing_filter_fail_action="skip_lock",
            external_breakout_events=month_events,
            external_breakout_shape_name=EXTERNAL_SHAPE_NAME,
            external_entry_mode="reentry_window",
            reentry_fill_policy=reentry_fill_policy,
        )

    if write_ledger:
        ledger.to_csv(output_dir / f"t2_reentry_schedule_ledger_{month}.csv", index=False)

    summary = lifecycle.summarize_run(ledger, initial_balance)
    attribution = lifecycle.summarize_breakout_attribution(ledger)
    external_attr = _shape_attr(attribution, EXTERNAL_SHAPE_NAME)
    external_locks = _flat_counts(diagnostics.get("external_breakout_locks", {}))
    reentry_rejects = _flat_counts(diagnostics.get("reentry_fill_rejects", {}))
    return SiloResult(
        month=month,
        return_pct=round(float(summary["return_pct"]), 6),
        final_balance=round(float(summary["final_balance"]), 2),
        total_trades=int(summary["trades"]),
        external_trades=int(external_attr.get("trades", 0)),
        external_net_pnl_pct=round(_net_pnl_pct(attribution, EXTERNAL_SHAPE_NAME, initial_balance), 6),
        max_dd_pct=round(float(summary["max_dd_pct"]), 6),
        entry_reasons={str(k): int(v) for k, v in summary.get("entry_reasons", {}).items()},
        exit_reasons={str(k): int(v) for k, v in summary.get("exit_reasons", {}).items()},
        external_events_available=int(len(month_events)),
        external_locks=int(sum(external_locks.values())),
        reentry_fill_rejects=reentry_rejects,
        elapsed_seconds=round(time.time() - started, 2),
    )


def _metrics(silos: list[SiloResult]) -> dict[str, Any]:
    by_month = {row.month: row.external_net_pnl_pct for row in silos}
    values = list(by_month.values())
    entry_reasons: dict[str, int] = {}
    exit_reasons: dict[str, int] = {}
    rejects: dict[str, int] = {}
    for row in silos:
        for key, value in row.entry_reasons.items():
            entry_reasons[key] = int(entry_reasons.get(key, 0)) + int(value)
        for key, value in row.exit_reasons.items():
            exit_reasons[key] = int(exit_reasons.get(key, 0)) + int(value)
        for key, value in row.reentry_fill_rejects.items():
            rejects[key] = int(rejects.get(key, 0)) + int(value)
    reentry_pct = round(float(sum(values)), 6)
    return {
        "variant": "canonical_lead_t2_reentry_schedule",
        "calendar_sum_pct": reentry_pct,
        "delta_vs_current_qband_lead_pct": round(reentry_pct - CURRENT_QBAND_LEAD_PCT, 6),
        "delta_vs_base_lead_adverse10_pct": round(reentry_pct - BASE_LEAD_ADVERSE10_PCT, 6),
        "worst_month_pct": round(float(min(values, default=0.0)), 6),
        "negative_months": int(sum(1 for value in values if value < 0.0)),
        "total_trades": int(sum(row.total_trades for row in silos)),
        "external_trades": int(sum(row.external_trades for row in silos)),
        "external_events_available": int(sum(row.external_events_available for row in silos)),
        "external_locks": int(sum(row.external_locks for row in silos)),
        "entry_reasons": entry_reasons,
        "exit_reasons": exit_reasons,
        "reentry_fill_rejects": rejects,
        "by_month": by_month,
    }


def _markdown_table(rows: list[dict[str, Any]], columns: list[str]) -> str:
    if not rows:
        return "_empty_"
    values = [[str(row.get(column, "")) for column in columns] for row in rows]
    widths = [max(len(column), *(len(row[idx]) for row in values)) for idx, column in enumerate(columns)]
    header = "| " + " | ".join(column.ljust(widths[idx]) for idx, column in enumerate(columns)) + " |"
    sep = "| " + " | ".join("-" * widths[idx] for idx in range(len(columns))) + " |"
    body = ["| " + " | ".join(row[idx].ljust(widths[idx]) for idx in range(len(columns))) + " |" for row in values]
    return "\n".join([header, sep, *body])


def _write_report(output_dir: Path, metrics: dict[str, Any], silos: list[SiloResult], *, input_path: Path) -> None:
    silo_rows = [
        {
            "month": row.month,
            "external_net_pnl_pct": f"{row.external_net_pnl_pct:.6f}",
            "events": row.external_events_available,
            "locks": row.external_locks,
            "trades": row.external_trades,
            "max_dd_pct": f"{row.max_dd_pct:.6f}",
            "entry_reasons": json.dumps(row.entry_reasons, sort_keys=True),
        }
        for row in silos
    ]
    lines = [
        "# T2 Reentry Schedule Revalidation",
        "",
        "Research-only audit. Canonical ETH pretouch lead events are replayed with lead-side",
        "`reentry_window` lifecycle semantics and `reentry_size_schedule=[0.20, 0.10]`.",
        "",
        f"- Input lead events: `{input_path}`",
        f"- External shape: `{EXTERNAL_SHAPE_NAME}`",
        "- Native original_t2 and native t3_swing locks are disabled to isolate canonical lead events.",
        "- This is a lifecycle reentry schedule check, not a live runtime change.",
        "",
        "## Summary",
        "",
        "| Variant | Calendar Sum | Delta vs current qband lead | Delta vs base adverse10 | Worst month | Negative months | Events | Locks | Trades |",
        "| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |",
        f"| `{metrics['variant']}` | {metrics['calendar_sum_pct']:.6f}% "
        f"| {metrics['delta_vs_current_qband_lead_pct']:.6f}pp "
        f"| {metrics['delta_vs_base_lead_adverse10_pct']:.6f}pp "
        f"| {metrics['worst_month_pct']:.6f}% "
        f"| {metrics['negative_months']} "
        f"| {metrics['external_events_available']} "
        f"| {metrics['external_locks']} "
        f"| {metrics['external_trades']} |",
        "",
        "Reference:",
        "",
        f"- Current `lead_quantity_0p20_0p40_adverse10`: `{CURRENT_QBAND_LEAD_PCT:.6f}%`.",
        f"- Base `base_lead_adverse10_exact`: `{BASE_LEAD_ADVERSE10_PCT:.6f}%`.",
        "",
        "## Month Detail",
        "",
        _markdown_table(
            silo_rows,
            ["month", "external_net_pnl_pct", "events", "locks", "trades", "max_dd_pct", "entry_reasons"],
        ),
        "",
        "## Read",
        "",
        "- If this lifecycle schedule is below the current qband lead, do not restore lead-side reentry as the research lead.",
        "- The current qband lead remains a selected-delay adverse10 ledger with absolute `0.20..0.40 ETH` sizing.",
        "- T3 overlay lifecycle reentries should remain a separate overlay research surface.",
    ]
    (output_dir / "t2_reentry_schedule_revalidation_report.md").write_text("\n".join(lines) + "\n", encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--lead-events", type=Path, default=DEFAULT_LEAD_EVENTS)
    parser.add_argument("--output-dir", type=Path, default=DEFAULT_OUTPUT)
    parser.add_argument("--timeframe", default="1h")
    parser.add_argument("--initial-balance", type=float, default=INITIAL_BALANCE)
    parser.add_argument("--reentry-fill-policy", choices=["historical", "strict_next_second_cross"], default="strict_next_second_cross")
    parser.add_argument("--write-ledgers", action="store_true")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(name)s: %(message)s")
    args.output_dir.mkdir(parents=True, exist_ok=True)
    events = _load_canonical_events(args.lead_events)
    months = sorted(events["touch_time"].dt.strftime("%Y-%m").unique().tolist())

    silos = [
        _run_month(
            events=events,
            month=month,
            timeframe=str(args.timeframe),
            initial_balance=float(args.initial_balance),
            reentry_fill_policy=str(args.reentry_fill_policy),
            write_ledger=bool(args.write_ledgers),
            output_dir=args.output_dir,
        )
        for month in months
    ]
    metrics = _metrics(silos)
    pd.DataFrame([asdict(row) for row in silos]).to_csv(
        args.output_dir / "t2_reentry_schedule_revalidation_silos.csv",
        index=False,
    )
    payload = {
        "note": "Research-only lead-side T2 reentry schedule revalidation.",
        "lead_events": str(args.lead_events),
        "reentry_size_schedule": REENTRY_SCHEDULE,
        "max_trades_per_bar": 2,
        "reentry_fill_policy": args.reentry_fill_policy,
        "metrics": metrics,
        "silos": [asdict(row) for row in silos],
    }
    (args.output_dir / "t2_reentry_schedule_revalidation_summary.json").write_text(
        json.dumps(payload, indent=2, ensure_ascii=False, default=str) + "\n",
        encoding="utf-8",
    )
    _write_report(args.output_dir, metrics, silos, input_path=args.lead_events)
    print(
        "t2_reentry_schedule_revalidation "
        f"calendar_sum={metrics['calendar_sum_pct']:.6f}% "
        f"delta_vs_qband={metrics['delta_vs_current_qband_lead_pct']:.6f}pp "
        f"worst_month={metrics['worst_month_pct']:.6f}% "
        f"trades={metrics['external_trades']}"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
