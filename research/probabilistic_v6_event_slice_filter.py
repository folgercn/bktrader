#!/usr/bin/env python3
"""Create filtered V6 event datasets from fixed event-slice hypotheses.

Research-only. The default slice thresholds are frozen from the R4.3 feature
slice report and must be treated as hypotheses, not live strategy rules.
"""

from __future__ import annotations

import argparse
import json
from pathlib import Path

import numpy as np
import pandas as pd


SLICE_SPECS = {
    "btc_short_eff60_low": {
        "description": "BTCUSDT short events with low 60s efficiency",
        "clauses": [
            ("symbol", "eq", "BTCUSDT"),
            ("side", "eq", "short"),
            ("eff_60s", "le", 0.949966),
        ],
    },
    "short_speed60_high": {
        "description": "All short events with high direction speed over 60s",
        "clauses": [
            ("side", "eq", "short"),
            ("speed_60s_atr", "gt", 0.260001),
        ],
    },
    "eth_short_prevclose_midlow": {
        "description": "ETHUSDT short events with previous close position not too extended",
        "clauses": [
            ("symbol", "eq", "ETHUSDT"),
            ("side", "eq", "short"),
            ("prev1_close_pos_side", "le", 0.837915),
        ],
    },
    "btc_flow15_low": {
        "description": "BTCUSDT events with lower 15s aligned flow ratio",
        "clauses": [
            ("symbol", "eq", "BTCUSDT"),
            ("flow_ratio_15s", "le", 0.914133),
        ],
    },
    "eth_short_range_high": {
        "description": "ETHUSDT short events after a high-range previous bar",
        "clauses": [
            ("symbol", "eq", "ETHUSDT"),
            ("side", "eq", "short"),
            ("prev1_range_atr", "gt", 0.914154),
        ],
    },
}

UNION_SPECS = {
    "r4_3_top_union": [
        "btc_short_eff60_low",
        "short_speed60_high",
        "eth_short_prevclose_midlow",
        "btc_flow15_low",
    ],
    "r4_3_short_quality_union": [
        "btc_short_eff60_low",
        "short_speed60_high",
        "eth_short_prevclose_midlow",
        "eth_short_range_high",
    ],
}


def _mask_for_spec(events: pd.DataFrame, spec: dict) -> pd.Series:
    mask = pd.Series(True, index=events.index)
    for column, op, expected in spec["clauses"]:
        if column not in events.columns:
            raise ValueError(f"missing column {column!r}")
        if op == "eq":
            mask &= events[column].astype(str) == str(expected)
        else:
            values = pd.to_numeric(events[column], errors="coerce")
            if op == "le":
                mask &= values <= float(expected)
            elif op == "lt":
                mask &= values < float(expected)
            elif op == "ge":
                mask &= values >= float(expected)
            elif op == "gt":
                mask &= values > float(expected)
            else:
                raise ValueError(f"unsupported op {op!r}")
    return mask.fillna(False)


def _month_column(events: pd.DataFrame) -> pd.Series:
    if "touch_time" in events.columns:
        return pd.to_datetime(events["touch_time"], utc=True, errors="coerce").dt.strftime("%Y-%m")
    if "signal_start" in events.columns:
        return pd.to_datetime(events["signal_start"], utc=True, errors="coerce").dt.strftime("%Y-%m")
    raise ValueError("missing touch_time/signal_start")


def _summary(frame: pd.DataFrame, total_events: int) -> dict:
    if frame.empty:
        return {
            "events": 0,
            "coverage_pct": 0.0,
            "months": [],
            "symbols": {},
            "total_execution_return_pct": 0.0,
            "avg_execution_return_pct": 0.0,
            "win_rate": 0.0,
            "initial_sl_rate": 0.0,
            "positive_months": 0,
            "worst_month_return_pct": 0.0,
            "best_month_return_pct": 0.0,
            "month_returns": {},
        }
    returns = pd.to_numeric(frame.get("execution_return_pct", 0.0), errors="coerce").fillna(0.0)
    months = _month_column(frame)
    monthly = returns.groupby(months).sum().sort_index()
    exit_reason = frame.get("execution_exit_reason", pd.Series("", index=frame.index)).astype(str)
    tradable = frame.get("execution_tradable", pd.Series(True, index=frame.index))
    if tradable.dtype != bool:
        tradable = tradable.astype(str).str.lower().isin({"true", "1", "yes"})
    return {
        "events": int(len(frame)),
        "coverage_pct": round(float(len(frame)) / max(1, int(total_events)) * 100.0, 6),
        "months": [str(value) for value in sorted(months.dropna().unique().tolist())],
        "symbols": {str(k): int(v) for k, v in frame["symbol"].value_counts().sort_index().items()}
        if "symbol" in frame.columns
        else {},
        "tradable_events": int(tradable.sum()),
        "total_execution_return_pct": round(float(returns.sum()), 6),
        "avg_execution_return_pct": round(float(returns.mean()), 6),
        "win_rate": round(float((returns > 0.0).mean()), 6),
        "initial_sl_rate": round(float(exit_reason.eq("InitialSL").mean()), 6),
        "positive_months": int((monthly > 0.0).sum()),
        "worst_month_return_pct": round(float(monthly.min()), 6),
        "best_month_return_pct": round(float(monthly.max()), 6),
        "month_returns": {str(k): round(float(v), 6) for k, v in monthly.items()},
    }


def _write_markdown(result: dict, path: Path) -> None:
    lines = [
        "# Probabilistic V6 Event Slice Filter",
        "",
        "范围：仅限 `research`。这些 slice 来自 R4.3 label 复盘，是下一轮 walk-forward 假设，不是实盘规则。",
        "",
        f"- input_events: `{result['input_events']}`",
        f"- output_root: `{result['output_root']}`",
        "",
        "| Slice | Events | Coverage | Label Return | Win | InitialSL | Pos Months | Worst Month | Symbols |",
        "|---|---:|---:|---:|---:|---:|---:|---:|---|",
    ]
    for item in result["slices"]:
        summary = item["summary"]
        symbols = ", ".join(f"{k}:{v}" for k, v in summary["symbols"].items())
        lines.append(
            f"| `{item['name']}` | {summary['events']} | {summary['coverage_pct']:.2f}% | "
            f"{summary['total_execution_return_pct']:.4f}% | {summary['win_rate']:.4f} | "
            f"{summary['initial_sl_rate']:.4f} | {summary['positive_months']} | "
            f"{summary['worst_month_return_pct']:.4f}% | {symbols} |"
        )
    path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Filter V6 event datasets by fixed slices")
    parser.add_argument("--events-csv", required=True)
    parser.add_argument("--output-root", required=True)
    parser.add_argument("--slices", nargs="+", default=list(SLICE_SPECS.keys()) + list(UNION_SPECS.keys()))
    parser.add_argument("--summary-json", required=True)
    parser.add_argument("--markdown", required=True)
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    events = pd.read_csv(args.events_csv)
    output_root = Path(args.output_root)
    output_root.mkdir(parents=True, exist_ok=True)
    masks = {name: _mask_for_spec(events, spec) for name, spec in SLICE_SPECS.items()}
    results = []
    for name in args.slices:
        if name in masks:
            mask = masks[name]
            description = SLICE_SPECS[name]["description"]
            clauses = SLICE_SPECS[name]["clauses"]
        elif name in UNION_SPECS:
            mask = pd.Series(False, index=events.index)
            for child in UNION_SPECS[name]:
                mask |= masks[child]
            description = " OR ".join(UNION_SPECS[name])
            clauses = UNION_SPECS[name]
        else:
            raise ValueError(f"unknown slice {name!r}")
        filtered = events.loc[mask].copy()
        slice_dir = output_root / name
        slice_dir.mkdir(parents=True, exist_ok=True)
        events_path = slice_dir / "events_execution_labeled.csv"
        filtered.to_csv(events_path, index=False)
        results.append(
            {
                "name": name,
                "description": description,
                "clauses": clauses,
                "events_csv": str(events_path),
                "summary": _summary(filtered, len(events)),
            }
        )
    result = {
        "input_csv": args.events_csv,
        "input_events": int(len(events)),
        "output_root": str(output_root),
        "slices": results,
    }
    summary_path = Path(args.summary_json)
    summary_path.parent.mkdir(parents=True, exist_ok=True)
    summary_path.write_text(json.dumps(result, indent=2, ensure_ascii=False), encoding="utf-8")
    _write_markdown(result, Path(args.markdown))
    print(json.dumps({"summary_json": str(summary_path), "markdown": args.markdown, "slices": results}, indent=2, ensure_ascii=False))


if __name__ == "__main__":
    main()
