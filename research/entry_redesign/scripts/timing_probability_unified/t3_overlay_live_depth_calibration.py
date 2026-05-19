"""Calibrate lead-scale sizing candidates from live execution telemetry.

Research-only. This script consumes sanitized order-list JSON from
``bktrader-ctl order list --json`` and extracts live entry execution quality
fields that are useful for judging whether a lead-scale lift deserves further
research:

- order-book spread and top-depth coverage at decision time;
- book freshness and source-divergence guard fields;
- actual filled price drift versus the execution reference price.

It intentionally writes only derived, non-secret metrics. Do not use this as a
live sizing engine; it is a calibration report for the research proxy.
"""

from __future__ import annotations

import argparse
import json
import sys
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
DEFAULT_OUTPUT = OUTPUT_DIR / "t3_overlay_live_depth_calibration"


@dataclass(frozen=True)
class EntryTelemetry:
    """Sanitized live entry execution-quality row."""

    order_id: str
    created_at: str
    symbol: str
    side: str
    quantity: float
    fill_price: float
    reference_price: float
    adverse_fill_drift_bps: float
    spread_bps: float
    book_age_ms: float | None
    source_divergence_bps: float | None
    top_depth_qty: float | None
    top_depth_coverage: float | None
    guard_status: str
    max_slippage_bps: float
    max_book_age_ms: float
    max_source_divergence_bps: float
    min_top_depth_coverage: float
    max_spread_bps: float


@dataclass(frozen=True)
class ScaleCalibration:
    """One candidate quantity-scale calibration row."""

    quantity_scale: float
    sample_entries: int
    pre_submit_pass_entries: int
    fill_adverse_pass_entries: int
    combined_pass_entries: int
    min_scaled_top_depth_coverage: float | None
    p10_scaled_top_depth_coverage: float | None
    median_scaled_top_depth_coverage: float | None
    max_spread_bps: float | None
    max_book_age_ms: float | None
    max_source_divergence_bps: float | None
    max_adverse_fill_drift_bps: float | None
    p90_adverse_fill_drift_bps: float | None
    worst_slippage_headroom_bps: float | None


def _get(obj: Any, *path: str) -> Any:
    cur = obj
    for key in path:
        if not isinstance(cur, dict):
            return None
        cur = cur.get(key)
    return cur


def _first(obj: dict[str, Any], paths: list[tuple[str, ...]]) -> Any:
    for path in paths:
        value = _get(obj, *path)
        if value is not None:
            return value
    return None


def _as_float(value: Any) -> float | None:
    if value is None or value == "":
        return None
    try:
        out = float(value)
    except (TypeError, ValueError):
        return None
    if pd.isna(out):
        return None
    return out


def _as_text(value: Any) -> str:
    if value is None:
        return ""
    try:
        if bool(pd.isna(value)):
            return ""
    except (TypeError, ValueError):
        pass
    return str(value)


def _round_or_none(value: float | None, digits: int = 6) -> float | None:
    if value is None:
        return None
    return round(float(value), digits)


def _rows(payload: Any) -> list[dict[str, Any]]:
    if isinstance(payload, list):
        return [row for row in payload if isinstance(row, dict)]
    if isinstance(payload, dict):
        items = payload.get("items")
        if isinstance(items, list):
            return [row for row in items if isinstance(row, dict)]
    return []


def adverse_fill_drift_bps(side: str, fill_price: float | None, reference_price: float | None) -> float | None:
    """Return positive bps when the actual fill is worse than the reference."""
    if fill_price is None or reference_price is None or reference_price <= 0.0:
        return None
    side = side.upper()
    if side == "BUY":
        return max(0.0, (fill_price - reference_price) / reference_price * 10000.0)
    if side == "SELL":
        return max(0.0, (reference_price - fill_price) / reference_price * 10000.0)
    return None


def _is_target_order(
    order: dict[str, Any],
    *,
    symbol: str,
    strategy_version_id: str | None,
    strategy_engine_key: str | None,
) -> bool:
    if symbol and str(order.get("symbol", "")).upper() != symbol.upper():
        return False
    if strategy_version_id and order.get("strategyVersionId") == strategy_version_id:
        return True
    engine = _first(
        order,
        [
            ("metadata", "executionContext", "strategyEngineKey"),
            ("metadata", "executionProposal", "metadata", "executionContext", "strategyEngineKey"),
            ("metadata", "intent", "metadata", "executionContext", "strategyEngineKey"),
        ],
    )
    if strategy_engine_key and engine == strategy_engine_key:
        return True
    return not strategy_version_id and not strategy_engine_key


def extract_entry_telemetry(
    payload: Any,
    *,
    symbol: str = "ETHUSDT",
    strategy_version_id: str | None = "strategy-version-bk-eth-pretouch-timing-v010",
    strategy_engine_key: str | None = "bk-live-eth-pretouch-timing",
) -> list[EntryTelemetry]:
    """Extract sanitized entry rows from order-list JSON."""
    entries: list[EntryTelemetry] = []
    for order in _rows(payload):
        if not _is_target_order(
            order,
            symbol=symbol,
            strategy_version_id=strategy_version_id,
            strategy_engine_key=strategy_engine_key,
        ):
            continue
        metadata = order.get("metadata") if isinstance(order.get("metadata"), dict) else {}
        signal_kind = _first(
            order,
            [
                ("metadata", "signalKind"),
                ("metadata", "executionProposal", "signalKind"),
                ("metadata", "intent", "signalKind"),
            ],
        )
        if signal_kind != "entry" or bool(order.get("reduceOnly")):
            continue

        side = str(order.get("side", "")).upper()
        quantity = _as_float(order.get("quantity"))
        fill_price = _as_float(order.get("price"))
        reference_price = _as_float(
            _first(
                order,
                [
                    ("metadata", "adapterSubmission", "rawPriceReference"),
                    ("metadata", "lastExecutionDispatch", "rawPriceReference"),
                    ("metadata", "executionProposal", "priceHint"),
                    ("metadata", "intent", "priceHint"),
                ],
            )
        )
        guard = _first(
            order,
            [
                ("metadata", "executionProposal", "metadata", "entrySubmissionSlippageGuard"),
                ("metadata", "intent", "metadata", "entrySubmissionSlippageGuard"),
            ],
        )
        guard = guard if isinstance(guard, dict) else {}
        snapshot = _first(
            order,
            [
                ("metadata", "executionProposal", "metadata", "orderBookSnapshot"),
                ("metadata", "intent", "metadata", "orderBookSnapshot"),
            ],
        )
        snapshot = snapshot if isinstance(snapshot, dict) else {}

        spread_bps = _as_float(
            _first(
                order,
                [
                    ("metadata", "executionProposal", "spreadBps"),
                    ("metadata", "intent", "spreadBps"),
                    ("metadata", "executionProposal", "metadata", "spreadBps"),
                    ("metadata", "intent", "metadata", "spreadBps"),
                    ("metadata", "lastExecutionDispatch", "spreadBps"),
                ],
            )
        )
        top_depth_qty = _as_float(guard.get("topDepthQty"))
        if top_depth_qty is None:
            if side == "BUY":
                top_depth_qty = _as_float(snapshot.get("bestAskQty"))
            elif side == "SELL":
                top_depth_qty = _as_float(snapshot.get("bestBidQty"))
        top_depth_coverage = _as_float(guard.get("topDepthCoverage"))
        if top_depth_coverage is None and top_depth_qty is not None and quantity and quantity > 0.0:
            top_depth_coverage = top_depth_qty / quantity

        max_spread_bps = _as_float(
            _first(
                order,
                [
                    ("metadata", "executionProposal", "metadata", "executionProfileMaxSpreadBps"),
                    ("metadata", "intent", "metadata", "executionProfileMaxSpreadBps"),
                    ("metadata", "executionProposal", "metadata", "executionDecisionContext", "maxSpreadBps"),
                    ("metadata", "intent", "metadata", "executionDecisionContext", "maxSpreadBps"),
                ],
            )
        )
        entries.append(
            EntryTelemetry(
                order_id=str(order.get("id", "")),
                created_at=str(order.get("createdAt", "")),
                symbol=str(order.get("symbol", "")),
                side=side,
                quantity=_round_or_none(quantity, 8) or 0.0,
                fill_price=_round_or_none(fill_price, 8) or 0.0,
                reference_price=_round_or_none(reference_price, 8) or 0.0,
                adverse_fill_drift_bps=_round_or_none(
                    adverse_fill_drift_bps(side, fill_price, reference_price),
                    6,
                )
                or 0.0,
                spread_bps=_round_or_none(spread_bps, 6) or 0.0,
                book_age_ms=_round_or_none(_as_float(guard.get("bookAgeMs")), 6),
                source_divergence_bps=_round_or_none(_as_float(guard.get("sourceDivergenceBps")), 6),
                top_depth_qty=_round_or_none(top_depth_qty, 8),
                top_depth_coverage=_round_or_none(top_depth_coverage, 6),
                guard_status=str(guard.get("status", "")),
                max_slippage_bps=_round_or_none(_as_float(guard.get("maxSlippageBps")), 6) or 8.0,
                max_book_age_ms=_round_or_none(_as_float(guard.get("maxBookAgeMs")), 6) or 500.0,
                max_source_divergence_bps=_round_or_none(_as_float(guard.get("maxSourceDivergenceBps")), 6)
                or 8.0,
                min_top_depth_coverage=_round_or_none(_as_float(guard.get("minTopDepthCoverage")), 6) or 0.5,
                max_spread_bps=_round_or_none(max_spread_bps, 6) or 8.0,
            )
        )
    return sorted(entries, key=lambda row: row.created_at)


def _entry_from_mapping(row: dict[str, Any]) -> EntryTelemetry:
    """Build a sanitized telemetry row from a CSV/JSON mapping."""
    return EntryTelemetry(
        order_id=_as_text(row.get("order_id")),
        created_at=_as_text(row.get("created_at")),
        symbol=_as_text(row.get("symbol")),
        side=_as_text(row.get("side")).upper(),
        quantity=_round_or_none(_as_float(row.get("quantity")), 8) or 0.0,
        fill_price=_round_or_none(_as_float(row.get("fill_price")), 8) or 0.0,
        reference_price=_round_or_none(_as_float(row.get("reference_price")), 8) or 0.0,
        adverse_fill_drift_bps=_round_or_none(_as_float(row.get("adverse_fill_drift_bps")), 6) or 0.0,
        spread_bps=_round_or_none(_as_float(row.get("spread_bps")), 6) or 0.0,
        book_age_ms=_round_or_none(_as_float(row.get("book_age_ms")), 6),
        source_divergence_bps=_round_or_none(_as_float(row.get("source_divergence_bps")), 6),
        top_depth_qty=_round_or_none(_as_float(row.get("top_depth_qty")), 8),
        top_depth_coverage=_round_or_none(_as_float(row.get("top_depth_coverage")), 6),
        guard_status=_as_text(row.get("guard_status")),
        max_slippage_bps=_round_or_none(_as_float(row.get("max_slippage_bps")), 6) or 8.0,
        max_book_age_ms=_round_or_none(_as_float(row.get("max_book_age_ms")), 6) or 500.0,
        max_source_divergence_bps=_round_or_none(_as_float(row.get("max_source_divergence_bps")), 6) or 8.0,
        min_top_depth_coverage=_round_or_none(_as_float(row.get("min_top_depth_coverage")), 6) or 0.5,
        max_spread_bps=_round_or_none(_as_float(row.get("max_spread_bps")), 6) or 8.0,
    )


def load_entry_history(path: Path | None) -> list[EntryTelemetry]:
    """Load prior sanitized entry samples, returning an empty list if absent."""
    if path is None or not path.exists():
        return []
    frame = pd.read_csv(path)
    if frame.empty:
        return []
    return [_entry_from_mapping(row) for row in frame.to_dict(orient="records")]


def merge_entry_history(
    current_entries: list[EntryTelemetry],
    history_entries: list[EntryTelemetry],
) -> list[EntryTelemetry]:
    """Merge sanitized samples by order id, preferring current observations."""
    merged: dict[str, EntryTelemetry] = {}
    anonymous: list[EntryTelemetry] = []
    for entry in history_entries + current_entries:
        if entry.order_id:
            merged[entry.order_id] = entry
        else:
            anonymous.append(entry)
    return sorted(
        list(merged.values()) + anonymous,
        key=lambda row: (row.created_at, row.order_id),
    )


def _pre_submit_pass(row: EntryTelemetry, scale: float) -> bool:
    if row.guard_status and row.guard_status != "passed":
        return False
    scaled_coverage = None if row.top_depth_coverage is None else row.top_depth_coverage / float(scale)
    if scaled_coverage is None or scaled_coverage < row.min_top_depth_coverage:
        return False
    if row.spread_bps > row.max_spread_bps:
        return False
    if row.book_age_ms is None or row.book_age_ms > row.max_book_age_ms:
        return False
    if row.source_divergence_bps is None or row.source_divergence_bps > row.max_source_divergence_bps:
        return False
    return True


def calibrate_scales(entries: list[EntryTelemetry], scales: list[float]) -> list[ScaleCalibration]:
    """Calibrate candidate quantity scales against observed guard fields."""
    rows: list[ScaleCalibration] = []
    for scale in scales:
        scaled_coverages = [
            float(entry.top_depth_coverage) / float(scale)
            for entry in entries
            if entry.top_depth_coverage is not None
        ]
        adverse = [float(entry.adverse_fill_drift_bps) for entry in entries]
        pre_submit_pass = [_pre_submit_pass(entry, scale) for entry in entries]
        fill_pass = [entry.adverse_fill_drift_bps <= entry.max_slippage_bps for entry in entries]
        combined = [a and b for a, b in zip(pre_submit_pass, fill_pass)]
        coverage_series = pd.Series(scaled_coverages, dtype=float)
        adverse_series = pd.Series(adverse, dtype=float)
        rows.append(
            ScaleCalibration(
                quantity_scale=round(float(scale), 6),
                sample_entries=len(entries),
                pre_submit_pass_entries=int(sum(pre_submit_pass)),
                fill_adverse_pass_entries=int(sum(fill_pass)),
                combined_pass_entries=int(sum(combined)),
                min_scaled_top_depth_coverage=_round_or_none(float(coverage_series.min()), 6)
                if not coverage_series.empty
                else None,
                p10_scaled_top_depth_coverage=_round_or_none(float(coverage_series.quantile(0.10)), 6)
                if not coverage_series.empty
                else None,
                median_scaled_top_depth_coverage=_round_or_none(float(coverage_series.median()), 6)
                if not coverage_series.empty
                else None,
                max_spread_bps=_round_or_none(max((entry.spread_bps for entry in entries), default=0.0), 6)
                if entries
                else None,
                max_book_age_ms=_round_or_none(
                    max((entry.book_age_ms for entry in entries if entry.book_age_ms is not None), default=0.0),
                    6,
                )
                if entries
                else None,
                max_source_divergence_bps=_round_or_none(
                    max(
                        (
                            entry.source_divergence_bps
                            for entry in entries
                            if entry.source_divergence_bps is not None
                        ),
                        default=0.0,
                    ),
                    6,
                )
                if entries
                else None,
                max_adverse_fill_drift_bps=_round_or_none(float(adverse_series.max()), 6)
                if not adverse_series.empty
                else None,
                p90_adverse_fill_drift_bps=_round_or_none(float(adverse_series.quantile(0.90)), 6)
                if not adverse_series.empty
                else None,
                worst_slippage_headroom_bps=_round_or_none(
                    min((entry.max_slippage_bps - entry.adverse_fill_drift_bps for entry in entries), default=0.0),
                    6,
                )
                if entries
                else None,
            )
        )
    return rows


def _write_report(
    output_dir: Path,
    *,
    entries: list[EntryTelemetry],
    matrix: list[ScaleCalibration],
    source_note: str,
    current_entry_count: int,
    history_entry_count: int,
) -> None:
    lines = [
        "# T3 Overlay Live Depth Calibration",
        "",
        "Research-only calibration from live execution telemetry. Raw production JSON is not stored here;",
        "only sanitized execution-quality metrics are written.",
        "",
        "## Source",
        "",
        f"- Source: `{source_note}`",
        f"- Current extracted entries: `{current_entry_count}`",
        f"- Prior sanitized history entries: `{history_entry_count}`",
        f"- Entry samples: `{len(entries)}`",
        "",
        "## Observed Entry Quality",
        "",
        "| Metric | Value |",
        "|---|---:|",
    ]
    if entries:
        entry_frame = pd.DataFrame([asdict(entry) for entry in entries])
        lines.extend(
            [
                f"| Max spread bps | {float(entry_frame['spread_bps'].max()):.6f} |",
                f"| Max book age ms | {float(entry_frame['book_age_ms'].max()):.6f} |",
                f"| Max source divergence bps | {float(entry_frame['source_divergence_bps'].max()):.6f} |",
                f"| Min top-depth coverage | {float(entry_frame['top_depth_coverage'].min()):.6f} |",
                f"| Max adverse fill drift bps | {float(entry_frame['adverse_fill_drift_bps'].max()):.6f} |",
                f"| P90 adverse fill drift bps | {float(entry_frame['adverse_fill_drift_bps'].quantile(0.90)):.6f} |",
            ]
        )
    else:
        lines.append("| No matching entries | 0 |")
    lines.extend(
        [
            "",
            "## Quantity Scale Matrix",
            "",
            "| Scale | Pre-submit pass | Fill pass | Combined pass | Min scaled coverage | P10 scaled coverage | Max adverse drift | Worst 8bp headroom |",
            "|---:|---:|---:|---:|---:|---:|---:|---:|",
        ]
    )
    for row in matrix:
        sample = max(1, row.sample_entries)
        lines.append(
            f"| {row.quantity_scale:.2f}x "
            f"| {row.pre_submit_pass_entries}/{row.sample_entries} ({row.pre_submit_pass_entries / sample:.0%}) "
            f"| {row.fill_adverse_pass_entries}/{row.sample_entries} ({row.fill_adverse_pass_entries / sample:.0%}) "
            f"| {row.combined_pass_entries}/{row.sample_entries} ({row.combined_pass_entries / sample:.0%}) "
            f"| {row.min_scaled_top_depth_coverage if row.min_scaled_top_depth_coverage is not None else 'n/a'} "
            f"| {row.p10_scaled_top_depth_coverage if row.p10_scaled_top_depth_coverage is not None else 'n/a'} "
            f"| {row.max_adverse_fill_drift_bps if row.max_adverse_fill_drift_bps is not None else 'n/a'} "
            f"| {row.worst_slippage_headroom_bps if row.worst_slippage_headroom_bps is not None else 'n/a'} |"
        )
    lines.extend(
        [
            "",
            "## Read",
            "",
            "- Current live telemetry supports using top-depth coverage, book freshness, source divergence, and observed fill drift as the calibration surface.",
            "- Passing this matrix is necessary but not sufficient for promotion: the sample is small and comes from testnet execution, so it should not override the research impact proxy by itself.",
            "- A lead-scale lift should remain conditional; thin-book or high-drift cases must fall back to current sizing.",
        ]
    )
    (output_dir / "t3_overlay_live_depth_calibration_report.md").write_text(
        "\n".join(lines) + "\n",
        encoding="utf-8",
    )


def run(
    *,
    payload: Any,
    output_dir: Path,
    scales: list[float],
    symbol: str,
    strategy_version_id: str | None,
    strategy_engine_key: str | None,
    source_note: str,
    history_csv: Path | None = None,
) -> dict[str, Any]:
    output_dir.mkdir(parents=True, exist_ok=True)
    current_entries = extract_entry_telemetry(
        payload,
        symbol=symbol,
        strategy_version_id=strategy_version_id,
        strategy_engine_key=strategy_engine_key,
    )
    history_entries = load_entry_history(history_csv)
    entries = merge_entry_history(current_entries, history_entries)
    matrix = calibrate_scales(entries, scales)
    entry_rows = [asdict(entry) for entry in entries]
    matrix_rows = [asdict(row) for row in matrix]
    pd.DataFrame(entry_rows).to_csv(output_dir / "t3_overlay_live_depth_entry_samples.csv", index=False)
    pd.DataFrame(matrix_rows).to_csv(output_dir / "t3_overlay_live_depth_scale_matrix.csv", index=False)
    payload_out = {
        "note": "Research-only live depth calibration. Raw bktrader-ctl JSON is intentionally not stored.",
        "source": source_note,
        "filters": {
            "symbol": symbol,
            "strategy_version_id": strategy_version_id,
            "strategy_engine_key": strategy_engine_key,
            "signal_kind": "entry",
            "reduce_only": False,
        },
        "scales": scales,
        "history_csv": str(history_csv) if history_csv is not None else "",
        "current_entry_count": len(current_entries),
        "history_entry_count": len(history_entries),
        "deduped_entry_count": len(entries),
        "entries": entry_rows,
        "matrix": matrix_rows,
    }
    (output_dir / "t3_overlay_live_depth_calibration_summary.json").write_text(
        json.dumps(payload_out, indent=2, ensure_ascii=False, default=str) + "\n",
        encoding="utf-8",
    )
    _write_report(
        output_dir,
        entries=entries,
        matrix=matrix,
        source_note=source_note,
        current_entry_count=len(current_entries),
        history_entry_count=len(history_entries),
    )
    return payload_out


def _load_json_arg(value: str) -> Any:
    if value == "-":
        return json.load(sys.stdin)
    with Path(value).open("r", encoding="utf-8") as handle:
        return json.load(handle)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--orders-json", default="-", help="Path to order-list JSON, or '-' for stdin.")
    parser.add_argument(
        "--history-csv",
        type=Path,
        help="Optional prior sanitized entry-sample CSV to merge and dedupe by order_id.",
    )
    parser.add_argument("--output-dir", type=Path, default=DEFAULT_OUTPUT)
    parser.add_argument("--scales", nargs="+", type=float, default=[1.0, 1.25, 1.5, 2.0, 2.5])
    parser.add_argument("--symbol", default="ETHUSDT")
    parser.add_argument("--strategy-version-id", default="strategy-version-bk-eth-pretouch-timing-v010")
    parser.add_argument("--strategy-engine-key", default="bk-live-eth-pretouch-timing")
    parser.add_argument("--source-note", default="bktrader-ctl order list --json")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    payload = _load_json_arg(args.orders_json)
    result = run(
        payload=payload,
        output_dir=Path(args.output_dir),
        scales=[float(value) for value in args.scales],
        symbol=str(args.symbol),
        strategy_version_id=str(args.strategy_version_id) or None,
        strategy_engine_key=str(args.strategy_engine_key) or None,
        source_note=str(args.source_note),
        history_csv=args.history_csv,
    )
    matrix = pd.DataFrame(result["matrix"])
    selected = matrix[matrix["quantity_scale"] == 1.25]
    row = selected.iloc[0] if not selected.empty else matrix.iloc[0]
    print(
        "live_depth_calibration "
        f"entries={len(result['entries'])} "
        f"scale={row['quantity_scale']:.2f}x "
        f"combined_pass={int(row['combined_pass_entries'])}/{int(row['sample_entries'])} "
        f"min_scaled_coverage={row['min_scaled_top_depth_coverage']} "
        f"max_adverse_fill_drift_bps={row['max_adverse_fill_drift_bps']}"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
