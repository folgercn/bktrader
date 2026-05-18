"""Post-touch breakout structure confirmation sweep for ETH pretouch lead.

Research-only. This expands the current production-aligned breakout shape by
testing whether the first touch should require a small same-direction 1s close
confirmation before entry. The entry is repriced to the confirmation second, so
the test does not treat post-touch confirmation as free information.
"""

from __future__ import annotations

import argparse
import json
import logging
import sys
import time
from copy import deepcopy
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import numpy as np
import pandas as pd

SCRIPTS_DIR = Path(__file__).resolve().parents[1]
if str(SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPTS_DIR))

from dynamic_timing.execution_sim import DEFAULT_EXEC_PARAMS  # noqa: E402
from timing_probability_unified.breakout_shape_expansion import (  # noqa: E402
    BASE_SHARE,
    EVAL_END,
    EVAL_START,
    OUTPUT_DIR,
    _bars_cache_for_symbol,
    _evaluate_events,
    _load_all_1s_bars,
    _neg_sm_count,
)
from timing_probability_unified.combined_executor import (  # noqa: E402
    compute_calendar_sum,
    compute_worst_sm,
)

logger = logging.getLogger(__name__)

BASE_EVENTS_PATH = OUTPUT_DIR / "breakout_shape_expansion_events_restrictive_0p5bps.csv"


@dataclass(frozen=True)
class ConfirmationVariant:
    name: str
    max_seconds: int | None
    min_close_extension_atr: float | None
    max_pre_confirm_adverse_atr: float | None
    description: str


CONFIRMATION_VARIANTS: list[ConfirmationVariant] = [
    ConfirmationVariant(
        name="baseline_touch_entry",
        max_seconds=None,
        min_close_extension_atr=None,
        max_pre_confirm_adverse_atr=None,
        description="current production shape replay: enter at first touch close",
    ),
    ConfirmationVariant(
        name="touch_close_reclaim",
        max_seconds=0,
        min_close_extension_atr=0.0,
        max_pre_confirm_adverse_atr=None,
        description="touch second must close beyond the breakout level",
    ),
    ConfirmationVariant(
        name="touch_close_ext_ge_0p03atr",
        max_seconds=0,
        min_close_extension_atr=0.03,
        max_pre_confirm_adverse_atr=None,
        description="touch second close extension >= 0.03 ATR",
    ),
    ConfirmationVariant(
        name="follow_60s_ext_ge_0p03atr",
        max_seconds=60,
        min_close_extension_atr=0.03,
        max_pre_confirm_adverse_atr=None,
        description="within 60s, first close extension >= 0.03 ATR",
    ),
    ConfirmationVariant(
        name="follow_180s_ext_ge_0p05atr",
        max_seconds=180,
        min_close_extension_atr=0.05,
        max_pre_confirm_adverse_atr=None,
        description="within 180s, first close extension >= 0.05 ATR",
    ),
    ConfirmationVariant(
        name="follow_300s_ext_ge_0p05atr",
        max_seconds=300,
        min_close_extension_atr=0.05,
        max_pre_confirm_adverse_atr=None,
        description="within 300s, first close extension >= 0.05 ATR",
    ),
    ConfirmationVariant(
        name="follow_300s_ext_ge_0p10atr",
        max_seconds=300,
        min_close_extension_atr=0.10,
        max_pre_confirm_adverse_atr=None,
        description="within 300s, first close extension >= 0.10 ATR",
    ),
    ConfirmationVariant(
        name="clean_180s_ext_ge_0p05_adv_le_0p05atr",
        max_seconds=180,
        min_close_extension_atr=0.05,
        max_pre_confirm_adverse_atr=0.05,
        description="within 180s, close extension >= 0.05 ATR and adverse before confirmation <= 0.05 ATR",
    ),
    ConfirmationVariant(
        name="clean_300s_ext_ge_0p05_adv_le_0p05atr",
        max_seconds=300,
        min_close_extension_atr=0.05,
        max_pre_confirm_adverse_atr=0.05,
        description="within 300s, close extension >= 0.05 ATR and adverse before confirmation <= 0.05 ATR",
    ),
    ConfirmationVariant(
        name="clean_300s_ext_ge_0p10_adv_le_0p10atr",
        max_seconds=300,
        min_close_extension_atr=0.10,
        max_pre_confirm_adverse_atr=0.10,
        description="within 300s, close extension >= 0.10 ATR and adverse before confirmation <= 0.10 ATR",
    ),
]


def _load_base_events(path: Path) -> pd.DataFrame:
    if not path.exists():
        raise FileNotFoundError(
            f"{path} does not exist; run breakout_shape_expansion.py first"
        )
    events = pd.read_csv(path)
    for column in ("signal_start", "signal_end", "touch_time"):
        if column in events.columns:
            events[column] = pd.to_datetime(events[column], utc=True)
    return events


def _close_extension(side: str, close: float, level: float, atr: float) -> float:
    if atr <= 0:
        return np.nan
    if side == "short":
        return (level - close) / atr
    return (close - level) / atr


def _adverse_from_level(side: str, lows: np.ndarray, highs: np.ndarray, level: float, atr: float) -> float:
    if atr <= 0 or len(lows) == 0:
        return np.nan
    if side == "short":
        return max(0.0, float(np.max(highs)) - level) / atr
    return max(0.0, level - float(np.min(lows))) / atr


def _confirm_one_event(
    event: pd.Series,
    bars: pd.DataFrame,
    variant: ConfirmationVariant,
) -> dict[str, Any] | None:
    touch_time = pd.Timestamp(event["touch_time"])
    side = str(event["side"])
    level = float(event["level"])
    atr = float(event["atr"])

    if variant.max_seconds is None or variant.min_close_extension_atr is None:
        out = event.to_dict()
        out["confirmation_time"] = touch_time
        out["confirmation_delay_seconds"] = 0.0
        out["confirmation_close"] = float(event["touch_close"])
        out["confirmation_close_extension_atr"] = float(event["touch_extension_atr"])
        out["pre_confirm_adverse_atr"] = 0.0
        out["structure_confirmation_variant"] = variant.name
        out["original_touch_time"] = touch_time
        out["event_id"] = f"{event['event_id']}|{variant.name}"
        return out

    start_pos = int(bars.index.searchsorted(touch_time, side="left"))
    if start_pos >= len(bars):
        return None
    end_time = touch_time + pd.Timedelta(seconds=variant.max_seconds)
    end_pos = int(bars.index.searchsorted(end_time, side="right"))
    if end_pos <= start_pos:
        return None

    window = bars.iloc[start_pos:end_pos]
    if window.empty:
        return None

    closes = window["close"].to_numpy(dtype="float64", copy=False)
    highs = window["high"].to_numpy(dtype="float64", copy=False)
    lows = window["low"].to_numpy(dtype="float64", copy=False)
    extensions = np.array(
        [_close_extension(side, close, level, atr) for close in closes],
        dtype="float64",
    )
    candidate_positions = np.where(extensions >= float(variant.min_close_extension_atr))[0]
    if len(candidate_positions) == 0:
        return None

    for local_pos in candidate_positions:
        confirm_lows = lows[: local_pos + 1]
        confirm_highs = highs[: local_pos + 1]
        adverse_atr = _adverse_from_level(side, confirm_lows, confirm_highs, level, atr)
        if (
            variant.max_pre_confirm_adverse_atr is not None
            and adverse_atr > float(variant.max_pre_confirm_adverse_atr)
        ):
            # Adverse is monotonic over the pre-confirmation window; later
            # candidates cannot recover the clean-structure condition.
            return None
        confirmation_time = window.index[int(local_pos)]
        confirmation_close = float(closes[int(local_pos)])
        signal_start = pd.Timestamp(event["signal_start"])
        signal_start_pos = int(bars.index.searchsorted(signal_start, side="left"))
        confirm_abs_pos = start_pos + int(local_pos)
        signal_slice = bars.iloc[signal_start_pos : confirm_abs_pos + 1]

        out = event.to_dict()
        out["original_touch_time"] = touch_time
        out["touch_time"] = confirmation_time
        out["confirmation_time"] = confirmation_time
        out["confirmation_delay_seconds"] = float((confirmation_time - touch_time).total_seconds())
        out["confirmation_close"] = confirmation_close
        out["confirmation_close_extension_atr"] = float(extensions[int(local_pos)])
        out["pre_confirm_adverse_atr"] = float(adverse_atr)
        out["touch_close"] = confirmation_close
        out["touch_extension_atr"] = float(extensions[int(local_pos)])
        out["touch_price"] = confirmation_close
        if not signal_slice.empty:
            out["touch_high_so_far"] = float(signal_slice["high"].max())
            out["touch_low_so_far"] = float(signal_slice["low"].min())
        out["structure_confirmation_variant"] = variant.name
        out["event_id"] = f"{event['event_id']}|{variant.name}"
        return out

    return None


def build_confirmed_events(
    events: pd.DataFrame,
    bars_cache: dict[str, pd.DataFrame],
    variant: ConfirmationVariant,
) -> pd.DataFrame:
    rows: list[dict[str, Any]] = []
    for _, event in events.iterrows():
        touch_time = pd.Timestamp(event["touch_time"])
        bars = bars_cache.get(f"{event['symbol']}_{touch_time.strftime('%Y%m')}")
        if bars is None or bars.empty:
            continue
        confirmed = _confirm_one_event(event, bars, variant)
        if confirmed is not None:
            rows.append(confirmed)

    confirmed_events = pd.DataFrame(rows)
    for column in ("signal_start", "signal_end", "touch_time", "confirmation_time", "original_touch_time"):
        if column in confirmed_events.columns:
            confirmed_events[column] = pd.to_datetime(confirmed_events[column], utc=True)
    return confirmed_events


def _monthly_table(trades_by_variant: dict[str, pd.DataFrame]) -> pd.DataFrame:
    rows: list[dict[str, Any]] = []
    for variant, trades in trades_by_variant.items():
        if trades.empty:
            continue
        df = trades[trades["speed_gate_pass"] == True].copy()  # noqa: E712
        if df.empty:
            continue
        df["year_month"] = pd.to_datetime(df["touch_time"], utc=True).dt.strftime("%Y-%m")
        monthly = df.groupby("year_month")["weighted_pnl"].sum()
        for month, pnl in monthly.items():
            rows.append({"variant": variant, "year_month": month, "weighted_pnl": float(pnl)})
    return pd.DataFrame(rows)


def _summarize_variant(
    variant: ConfirmationVariant,
    base_count: int,
    confirmed_events: pd.DataFrame,
    matrix: pd.DataFrame,
    trades: pd.DataFrame,
) -> dict[str, Any]:
    gate_on = trades[trades["speed_gate_pass"] == True].copy() if not trades.empty else pd.DataFrame()  # noqa: E712
    row: dict[str, Any] = {
        "variant": variant.name,
        "description": variant.description,
        "confirmed_events": len(confirmed_events),
        "confirmed_rate": len(confirmed_events) / base_count if base_count else 0.0,
        "confirmed_advance_events": int((confirmed_events["timing_prediction"] != "skip").sum())
        if not confirmed_events.empty
        else 0,
        "d0_traded_events": int((trades["selected_delay"] != "none").sum()) if not trades.empty else 0,
        "avg_rf_probability": float(confirmed_events["rf_probability"].mean()) if not confirmed_events.empty else 0.0,
        "median_confirmation_delay_seconds": float(confirmed_events["confirmation_delay_seconds"].median())
        if not confirmed_events.empty
        else 0.0,
        "avg_confirmation_close_extension_atr": float(confirmed_events["confirmation_close_extension_atr"].mean())
        if not confirmed_events.empty
        else 0.0,
        "avg_pre_confirm_adverse_atr": float(confirmed_events["pre_confirm_adverse_atr"].mean())
        if not confirmed_events.empty
        else 0.0,
        "same_close_calendar_sum_direct": compute_calendar_sum(trades, gate_filter=True)
        if not trades.empty
        else 0.0,
        "same_close_worst_sm_direct": compute_worst_sm(trades, gate_filter=True)
        if not trades.empty
        else 0.0,
        "same_close_neg_sm_direct": _neg_sm_count(gate_on) if not gate_on.empty else 0,
    }
    if not matrix.empty:
        for scenario in ("same_close_xslip0bps", "next_adverse_xslip10bps"):
            view = matrix[matrix["scenario"] == scenario]
            if view.empty:
                continue
            prefix = "same_close" if scenario.startswith("same_close") else "adverse10"
            row[f"{prefix}_calendar_sum"] = float(view.iloc[0]["calendar_sum_gate_on"])
            row[f"{prefix}_worst_sm"] = float(view.iloc[0]["worst_sm_gate_on"])
            row[f"{prefix}_neg_sm"] = int(view.iloc[0]["neg_sm_count"])
    return row


def _markdown_table(df: pd.DataFrame, cols: list[str]) -> str:
    if df.empty:
        return "_empty_"
    view = df[cols].copy()

    def fmt(value: Any) -> str:
        if isinstance(value, (float, np.floating)):
            if not np.isfinite(value):
                return ""
            return f"{float(value):.6f}"
        if isinstance(value, (int, np.integer)):
            return str(int(value))
        if pd.isna(value):
            return ""
        return str(value)

    rows = [[fmt(value) for value in row] for row in view.to_numpy()]
    widths = [
        max(len(str(col)), *(len(row[idx]) for row in rows)) if rows else len(str(col))
        for idx, col in enumerate(view.columns)
    ]
    header = "| " + " | ".join(str(col).ljust(widths[idx]) for idx, col in enumerate(view.columns)) + " |"
    sep = "| " + " | ".join("-" * widths[idx] for idx in range(len(widths))) + " |"
    body = [
        "| " + " | ".join(row[idx].ljust(widths[idx]) for idx in range(len(widths))) + " |"
        for row in rows
    ]
    return "\n".join([header, sep] + body)


def _write_report(
    summary: pd.DataFrame,
    adverse: pd.DataFrame,
    monthly: pd.DataFrame,
    diagnostics: dict[str, Any],
    output_path: Path,
) -> None:
    lines: list[str] = []
    lines.append("# Breakout Structure Confirmation Sweep — ETH Pretouch Timing Lead")
    lines.append("")
    lines.append(f"Generated: {pd.Timestamp.utcnow().isoformat()}")
    lines.append("")
    lines.append(
        "Scope: research-only, ETHUSDT 1h, current production shape "
        "`restrictive_0p5bps`, frozen `data/pretouch_model.json` `20260515_v1`."
    )
    lines.append(
        "Each confirmation variant reprices entry to the first confirming 1s close. "
        "No live defaults are changed."
    )
    lines.append("")
    lines.append("## Summary")
    lines.append("")
    summary_cols = [
        "variant",
        "confirmed_events",
        "confirmed_advance_events",
        "d0_traded_events",
        "same_close_calendar_sum",
        "same_close_worst_sm",
        "same_close_neg_sm",
        "adverse10_calendar_sum",
        "adverse10_worst_sm",
        "adverse10_neg_sm",
        "median_confirmation_delay_seconds",
        "avg_confirmation_close_extension_atr",
        "avg_pre_confirm_adverse_atr",
    ]
    lines.append(_markdown_table(summary, summary_cols))
    lines.append("")
    lines.append("## Adverse Fill Matrix")
    lines.append("")
    adverse_cols = [
        "variant",
        "scenario",
        "calendar_sum_gate_on",
        "worst_sm_gate_on",
        "neg_sm_count",
        "trade_count_gate_on",
    ]
    lines.append(_markdown_table(adverse, adverse_cols))
    lines.append("")
    lines.append("## Monthly PnL")
    lines.append("")
    if monthly.empty:
        lines.append("_empty_")
    else:
        pivot = (
            monthly.pivot_table(
                index="year_month",
                columns="variant",
                values="weighted_pnl",
                aggfunc="sum",
                fill_value=0.0,
            )
            .reset_index()
            .rename_axis(None, axis=1)
        )
        lines.append(_markdown_table(pivot, list(pivot.columns)))
    lines.append("")
    lines.append("## Interpretation")
    lines.append("")
    lines.append("- `baseline_touch_entry` is the current production-shape D0 lens from the prior sweep.")
    lines.append("- Confirmation variants are post-touch structure filters, not live defaults.")
    lines.append("- A variant is only interesting if it improves calendar sum and worst single month under same-close and remains tolerable under next-second adverse stress.")
    lines.append("- Positive subset results here would still need canonical event-source alignment before promotion, because this lens rebuilds a broader live-like event pool.")
    lines.append("")
    lines.append("## Diagnostics")
    lines.append("")
    lines.append("```json")
    lines.append(json.dumps(diagnostics, indent=2, ensure_ascii=False, default=str))
    lines.append("```")
    output_path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def run() -> None:
    start = time.time()
    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)

    base_events = _load_base_events(BASE_EVENTS_PATH)
    base_events = base_events[
        (base_events["touch_time"] >= EVAL_START) & (base_events["touch_time"] < EVAL_END)
    ].reset_index(drop=True)
    logger.info("loaded base events=%d from %s", len(base_events), BASE_EVENTS_PATH)

    bars_1s = _load_all_1s_bars("ETHUSDT")
    bars_cache = _bars_cache_for_symbol("ETHUSDT", bars_1s)

    original_exec_params = deepcopy(DEFAULT_EXEC_PARAMS)
    DEFAULT_EXEC_PARAMS.update(
        {
            "initial_stop_atr": 0.45,
            "breakeven_at_r": 0.8,
            "trail_start_r": 1.5,
            "trail_buffer_atr": 0.05,
            "max_hold_hours": 2.0,
        }
    )

    summary_rows: list[dict[str, Any]] = []
    adverse_rows: list[pd.DataFrame] = []
    trades_by_variant: dict[str, pd.DataFrame] = {}
    diagnostics: dict[str, Any] = {
        "base_events_path": str(BASE_EVENTS_PATH),
        "base_events": len(base_events),
        "eval_start": EVAL_START.isoformat(),
        "eval_end_exclusive": EVAL_END.isoformat(),
        "base_share": BASE_SHARE,
        "exec_params": {k: DEFAULT_EXEC_PARAMS[k] for k in sorted(DEFAULT_EXEC_PARAMS)},
        "variants": {},
    }

    try:
        for variant in CONFIRMATION_VARIANTS:
            logger.info("=" * 72)
            logger.info("variant %s", variant.name)
            confirmed_events = build_confirmed_events(base_events, bars_cache, variant)
            logger.info("%s confirmed events=%d", variant.name, len(confirmed_events))

            matrix, trades = _evaluate_events(confirmed_events, bars_cache)
            if not matrix.empty:
                matrix.insert(0, "variant", variant.name)
                adverse_rows.append(matrix)
            if not trades.empty:
                trades["structure_confirmation_variant"] = variant.name
                trades.to_csv(
                    OUTPUT_DIR / f"breakout_structure_confirmation_trades_{variant.name}.csv",
                    index=False,
                )
            confirmed_events.to_csv(
                OUTPUT_DIR / f"breakout_structure_confirmation_events_{variant.name}.csv",
                index=False,
            )
            trades_by_variant[variant.name] = trades
            summary_rows.append(_summarize_variant(variant, len(base_events), confirmed_events, matrix, trades))
            diagnostics["variants"][variant.name] = {
                "confirmed_events": len(confirmed_events),
                "confirmed_rate": len(confirmed_events) / len(base_events) if len(base_events) else 0.0,
                "confirmed_advance_events": int((confirmed_events["timing_prediction"] != "skip").sum())
                if not confirmed_events.empty
                else 0,
                "description": variant.description,
            }
    finally:
        DEFAULT_EXEC_PARAMS.clear()
        DEFAULT_EXEC_PARAMS.update(original_exec_params)

    summary = pd.DataFrame(summary_rows)
    adverse = pd.concat(adverse_rows, ignore_index=True) if adverse_rows else pd.DataFrame()
    monthly = _monthly_table(trades_by_variant)

    summary_path = OUTPUT_DIR / "breakout_structure_confirmation_summary.csv"
    adverse_path = OUTPUT_DIR / "breakout_structure_confirmation_adverse_matrix.csv"
    monthly_path = OUTPUT_DIR / "breakout_structure_confirmation_monthly.csv"
    diagnostics_path = OUTPUT_DIR / "breakout_structure_confirmation_diagnostics.json"
    report_path = OUTPUT_DIR / "breakout_structure_confirmation_report.md"

    summary.to_csv(summary_path, index=False)
    adverse.to_csv(adverse_path, index=False)
    monthly.to_csv(monthly_path, index=False)
    diagnostics["runtime_seconds"] = time.time() - start
    diagnostics_path.write_text(json.dumps(diagnostics, indent=2, ensure_ascii=False, default=str) + "\n", encoding="utf-8")
    _write_report(summary, adverse, monthly, diagnostics, report_path)

    logger.info("written %s", summary_path)
    logger.info("written %s", adverse_path)
    logger.info("written %s", report_path)


def main() -> None:
    parser = argparse.ArgumentParser(description="ETH pretouch breakout structure confirmation sweep")
    parser.add_argument("--log-level", default="INFO")
    args = parser.parse_args()

    logging.basicConfig(
        level=getattr(logging, str(args.log_level).upper(), logging.INFO),
        format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
    )
    run()


if __name__ == "__main__":
    main()
