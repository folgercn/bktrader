#!/usr/bin/env python3
"""Feature-slice analyzer for execution-labeled probabilistic events.

Research-only. This is a hypothesis generator: it scans visible event features
and reports single/pair slices whose execution labels are positive across
multiple months. It does not create a live strategy and does not replace
walk-forward validation.
"""

from __future__ import annotations

import argparse
import csv
import json
import math
from collections import defaultdict
from pathlib import Path


DEFAULT_NUMERIC_FEATURES = [
    "prev1_body_atr",
    "prev1_range_atr",
    "prev1_close_pos_side",
    "prev_sma5_gap_atr",
    "prev_sma5_slope_atr",
    "level_to_prev_close_atr",
    "level_to_signal_open_atr",
    "touch_extension_atr",
    "flow_ratio_60s",
    "flow_ratio_120s",
    "speed_15s_atr",
    "speed_60s_atr",
    "speed_300s_atr",
    "eff_60s",
    "eff_300s",
    "close_pos_60s",
    "close_pos_300s",
    "pullback_15s_atr",
    "pullback_30s_atr",
    "pullback_60s_atr",
]

DEFAULT_CATEGORICAL_FEATURES = ["symbol", "side", "shape"]


def _float(row: dict, key: str) -> float | None:
    raw = row.get(key, "")
    if raw in ("", None):
        return None
    try:
        value = float(raw)
    except (TypeError, ValueError):
        return None
    return value if math.isfinite(value) else None


def _source_name(path: Path) -> str:
    parent = path.parent.name
    return parent if parent else path.stem


def _load_events(paths: list[str]) -> list[dict]:
    rows: list[dict] = []
    for raw_path in paths:
        path = Path(raw_path)
        source = _source_name(path)
        with path.open(newline="", encoding="utf-8") as fh:
            for row in csv.DictReader(fh):
                if str(row.get("execution_tradable", "")).lower() not in {"true", "1", "yes"}:
                    continue
                ret = _float(row, "execution_return_pct")
                if ret is None:
                    continue
                row = dict(row)
                row["_source"] = source
                row["_month"] = str(row.get("touch_time", ""))[:7]
                row["_execution_return_pct"] = ret
                rows.append(row)
    return rows


def _quantiles(rows: list[dict], features: list[str]) -> dict[str, list[float]]:
    thresholds: dict[str, list[float]] = {}
    for feature in features:
        values = sorted(value for row in rows if (value := _float(row, feature)) is not None)
        if len(values) < 100:
            continue
        thresholds[feature] = [values[int((len(values) - 1) * q)] for q in (0.25, 0.50, 0.75)]
    return thresholds


def _bin_label(row: dict, feature: str, thresholds: dict[str, list[float]]) -> str | None:
    value = _float(row, feature)
    if value is None or feature not in thresholds:
        return None
    q1, q2, q3 = thresholds[feature]
    if value <= q1:
        return f"{feature}:q1<={q1:.6g}"
    if value <= q2:
        return f"{feature}:q2<={q2:.6g}"
    if value <= q3:
        return f"{feature}:q3<={q3:.6g}"
    return f"{feature}:q4>{q3:.6g}"


def _summarize_slice(key: tuple[str, ...], rows: list[dict]) -> dict:
    monthly: dict[str, list[float]] = defaultdict(list)
    source_months: set[str] = set()
    wins = 0
    initial_sl = 0
    for row in rows:
        ret = float(row["_execution_return_pct"])
        monthly[str(row["_month"])].append(ret)
        source_months.add(f"{row['_source']}:{row['_month']}")
        if ret > 0.0:
            wins += 1
        if str(row.get("execution_exit_reason", "")) == "InitialSL":
            initial_sl += 1

    month_sums = {month: round(sum(values), 6) for month, values in monthly.items()}
    month_values = list(month_sums.values())
    total = sum(float(row["_execution_return_pct"]) for row in rows)
    return {
        "slice": " & ".join(key),
        "events": len(rows),
        "months": len(monthly),
        "source_months": len(source_months),
        "total_execution_return_pct": round(total, 6),
        "avg_execution_return_pct": round(total / max(1, len(rows)), 6),
        "win_rate": round(wins / max(1, len(rows)), 6),
        "initial_sl_rate": round(initial_sl / max(1, len(rows)), 6),
        "positive_months": sum(1 for value in month_values if value > 0.0),
        "worst_month_return_pct": round(min(month_values), 6) if month_values else 0.0,
        "best_month_return_pct": round(max(month_values), 6) if month_values else 0.0,
        "month_returns": month_sums,
    }


def _rank_score(item: dict) -> tuple[float, float, int, int]:
    return (
        float(item["total_execution_return_pct"]),
        float(item["worst_month_return_pct"]),
        int(item["positive_months"]),
        int(item["events"]),
    )


def _collect_groups(
    rows: list[dict],
    thresholds: dict[str, list[float]],
    categorical_features: list[str],
    pair_prefixes: list[str],
) -> tuple[dict[tuple[str, ...], list[dict]], dict[tuple[str, ...], list[dict]]]:
    singles: dict[tuple[str, ...], list[dict]] = defaultdict(list)
    pairs: dict[tuple[str, ...], list[dict]] = defaultdict(list)
    for row in rows:
        for feature in categorical_features:
            value = str(row.get(feature, ""))
            if value:
                singles[(f"{feature}={value}",)].append(row)
        for feature in thresholds:
            label = _bin_label(row, feature, thresholds)
            if label:
                singles[(label,)].append(row)

        for prefix in pair_prefixes:
            if prefix == "symbol_side":
                prefix_label = f"symbol_side={row.get('symbol', '')}_{row.get('side', '')}"
            else:
                value = str(row.get(prefix, ""))
                if not value:
                    continue
                prefix_label = f"{prefix}={value}"
            for feature in thresholds:
                label = _bin_label(row, feature, thresholds)
                if label:
                    pairs[(prefix_label, label)].append(row)
    return singles, pairs


def _filter_and_rank(groups: dict[tuple[str, ...], list[dict]], min_events: int, min_months: int, top_n: int) -> list[dict]:
    ranked = []
    for key, group_rows in groups.items():
        summary = _summarize_slice(key, group_rows)
        if int(summary["events"]) < int(min_events):
            continue
        if int(summary["months"]) < int(min_months):
            continue
        ranked.append(summary)
    ranked.sort(key=_rank_score, reverse=True)
    return ranked[: int(top_n)]


def _write_markdown(result: dict, path: Path) -> None:
    lines = [
        "# Probabilistic V6 Feature Slice Analyzer",
        "",
        "范围：仅限 `research`。本报告用于生成下一轮 walk-forward 假设；slice 阈值来自输入样本分位数，不能直接视为 OOS 规则。",
        "",
        "## Dataset",
        "",
        f"- events: `{result['events']}`",
        f"- months: `{', '.join(result['months'])}`",
        f"- sources: `{', '.join(result['sources'])}`",
        "",
        "## Top Single Slices",
        "",
        "| Slice | Events | Months | Total Label Return | Avg | Win | InitialSL | Pos Months | Worst Month |",
        "|---|---:|---:|---:|---:|---:|---:|---:|---:|",
    ]
    for item in result["top_single_slices"]:
        lines.append(
            f"| `{item['slice']}` | {item['events']} | {item['months']} | "
            f"{item['total_execution_return_pct']:.4f}% | {item['avg_execution_return_pct']:.6f}% | "
            f"{item['win_rate']:.4f} | {item['initial_sl_rate']:.4f} | {item['positive_months']} | "
            f"{item['worst_month_return_pct']:.4f}% |"
        )

    lines.extend(
        [
            "",
            "## Top Pair Slices",
            "",
            "| Slice | Events | Months | Total Label Return | Avg | Win | InitialSL | Pos Months | Worst Month |",
            "|---|---:|---:|---:|---:|---:|---:|---:|---:|",
        ]
    )
    for item in result["top_pair_slices"]:
        lines.append(
            f"| `{item['slice']}` | {item['events']} | {item['months']} | "
            f"{item['total_execution_return_pct']:.4f}% | {item['avg_execution_return_pct']:.6f}% | "
            f"{item['win_rate']:.4f} | {item['initial_sl_rate']:.4f} | {item['positive_months']} | "
            f"{item['worst_month_return_pct']:.4f}% |"
        )

    lines.extend(
        [
            "",
            "## Interpretation",
            "",
            "- 这些结果使用 execution labels 做切片复盘，下一步必须把候选 slice 固定成规则，再用 runner 做真实 busy/same-bar 约束执行。",
            "- 优先关注 `positive_months` 较高、`worst_month_return_pct` 不深、且不是极小样本的 slice。",
            "- 如果 slice 需要全样本分位数才能成立，应在下一轮改成 train/validation 内部确定阈值。",
        ]
    )
    path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Scan feature slices over execution-labeled events")
    parser.add_argument("--events-csv", nargs="+", required=True)
    parser.add_argument("--output-json", required=True)
    parser.add_argument("--markdown", required=True)
    parser.add_argument("--numeric-features", nargs="+", default=DEFAULT_NUMERIC_FEATURES)
    parser.add_argument("--categorical-features", nargs="+", default=DEFAULT_CATEGORICAL_FEATURES)
    parser.add_argument("--pair-prefixes", nargs="+", default=["symbol", "side", "symbol_side"])
    parser.add_argument("--min-single-events", type=int, default=80)
    parser.add_argument("--min-pair-events", type=int, default=40)
    parser.add_argument("--min-months", type=int, default=5)
    parser.add_argument("--top-n", type=int, default=25)
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    rows = _load_events(args.events_csv)
    if not rows:
        raise SystemExit("no tradable execution-labeled events loaded")
    thresholds = _quantiles(rows, args.numeric_features)
    singles, pairs = _collect_groups(rows, thresholds, args.categorical_features, args.pair_prefixes)
    result = {
        "events_csv": args.events_csv,
        "events": len(rows),
        "months": sorted({str(row["_month"]) for row in rows}),
        "sources": sorted({str(row["_source"]) for row in rows}),
        "thresholds": thresholds,
        "top_single_slices": _filter_and_rank(singles, args.min_single_events, args.min_months, args.top_n),
        "top_pair_slices": _filter_and_rank(pairs, args.min_pair_events, args.min_months, args.top_n),
    }
    output_path = Path(args.output_json)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(json.dumps(result, indent=2, ensure_ascii=False), encoding="utf-8")
    markdown_path = Path(args.markdown)
    markdown_path.parent.mkdir(parents=True, exist_ok=True)
    _write_markdown(result, markdown_path)
    print(
        json.dumps(
            {
                "output_json": str(output_path),
                "markdown": str(markdown_path),
                "events": result["events"],
                "top_pair": result["top_pair_slices"][0] if result["top_pair_slices"] else None,
            },
            indent=2,
            ensure_ascii=False,
        ),
        flush=True,
    )


if __name__ == "__main__":
    main()
