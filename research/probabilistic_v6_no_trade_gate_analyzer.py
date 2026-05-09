#!/usr/bin/env python3
"""Analyze second-level no-trade gates for V6 walk-forward rows.

Research-only. This tool consumes existing `symbol_rows.csv` files from
probabilistic V6 walk-forward runs and sweeps portfolio-level no-trade gates
that only use validation-period fields already emitted by the runner.
"""

from __future__ import annotations

import argparse
import csv
import itertools
import json
from collections import defaultdict
from pathlib import Path


def _float(row: dict, key: str, default: float = 0.0) -> float:
    raw = row.get(key, "")
    if raw in (None, ""):
        return default
    try:
        return float(raw)
    except (TypeError, ValueError):
        return default


def _int(row: dict, key: str, default: int = 0) -> int:
    return int(round(_float(row, key, float(default))))


def _parse_float_grid(raw: list[str]) -> list[float]:
    out: list[float] = []
    for item in raw:
        for part in str(item).split(","):
            part = part.strip()
            if part:
                out.append(float(part))
    return out


def _source_name(path: Path) -> str:
    if path.name == "symbol_rows.csv":
        return path.parent.name
    return path.stem


def _load_rows(paths: list[str]) -> list[dict]:
    rows: list[dict] = []
    for raw_path in paths:
        path = Path(raw_path)
        with path.open(newline="", encoding="utf-8") as fh:
            for row in csv.DictReader(fh):
                row = dict(row)
                row["source_run"] = _source_name(path)
                row["source_csv"] = str(path)
                row["realistic_return_pct"] = _float(row, "realistic_return_pct")
                row["trades"] = _int(row, "trades")
                row["selected_events"] = _int(row, "selected_events")
                row["validation_edge"] = _float(row, "validation_edge")
                row["validation_topk_sized_return_pct"] = _float(row, "validation_topk_sized_return_pct")
                row["validation_topk_initial_sl_rate"] = _float(row, "validation_topk_initial_sl_rate")
                row["validation_topk_max_dd_pct"] = _float(row, "validation_topk_max_dd_pct")
                row["test_edge"] = _float(row, "test_edge")
                row["top_k"] = _int(row, "top_k")
                rows.append(row)
    return rows


def _candidate_rows(rows: list[dict]) -> list[dict]:
    candidates = []
    for row in rows:
        if str(row.get("gate_reason", "")) != "pass":
            continue
        if _int(row, "trades") <= 0 and _int(row, "selected_events") <= 0:
            continue
        candidates.append(row)
    return candidates


def _validation_return_over_dd(row: dict) -> float:
    validation_return = _float(row, "validation_topk_sized_return_pct")
    validation_dd = abs(_float(row, "validation_topk_max_dd_pct"))
    return validation_return / max(0.25, validation_dd)


def _passes_gate(row: dict, gate: dict) -> bool:
    if _float(row, "validation_edge") < float(gate["min_validation_edge"]):
        return False
    if _float(row, "validation_topk_sized_return_pct") < float(gate["min_validation_topk_return_pct"]):
        return False
    if _float(row, "validation_topk_initial_sl_rate") > float(gate["max_validation_topk_initial_sl_rate"]):
        return False
    if abs(_float(row, "validation_topk_max_dd_pct")) > float(gate["max_validation_topk_dd_pct"]):
        return False
    if _validation_return_over_dd(row) < float(gate["min_validation_return_over_dd"]):
        return False
    if _validation_return_over_dd(row) > float(gate["max_validation_return_over_dd"]):
        return False
    if _float(row, "validation_topk_sized_return_pct") > float(gate["max_validation_topk_return_pct"]):
        return False
    return True


def _select_rows(rows: list[dict], policy: str) -> list[dict]:
    if policy == "all_sleeves":
        return list(rows)

    grouped: dict[tuple[str, str], list[dict]] = defaultdict(list)
    for row in rows:
        grouped[(str(row.get("execute_month", "")), str(row.get("symbol", "")))].append(row)

    selected: list[dict] = []
    for group_rows in grouped.values():
        selected.append(
            max(
                group_rows,
                key=lambda row: (
                    _validation_return_over_dd(row),
                    _float(row, "validation_topk_sized_return_pct"),
                    -abs(_float(row, "validation_topk_max_dd_pct")),
                ),
            )
        )
    return selected


def _summarize(selected: list[dict], all_candidates: list[dict]) -> dict:
    active_months = sorted({str(row.get("execute_month", "")) for row in selected})
    active_symbols = sorted({str(row.get("symbol", "")) for row in selected})
    monthly: dict[str, float] = defaultdict(float)
    for row in selected:
        monthly[str(row.get("execute_month", ""))] += _float(row, "realistic_return_pct")
    monthly_values = list(monthly.values())
    skipped = max(0, len(all_candidates) - len(selected))
    return {
        "active_rows": len(selected),
        "skipped_candidate_rows": skipped,
        "active_months": len(active_months),
        "active_symbols": ",".join(active_symbols),
        "trades": sum(_int(row, "trades") for row in selected),
        "total_realistic_pct": round(sum(_float(row, "realistic_return_pct") for row in selected), 6),
        "avg_row_realistic_pct": round(
            sum(_float(row, "realistic_return_pct") for row in selected) / max(1, len(selected)),
            6,
        ),
        "worst_month_realistic_pct": round(min(monthly_values), 6) if monthly_values else 0.0,
        "best_month_realistic_pct": round(max(monthly_values), 6) if monthly_values else 0.0,
    }


def _evaluate(candidates: list[dict], gate: dict, policy: str) -> dict:
    gated = [row for row in candidates if _passes_gate(row, gate)]
    selected = _select_rows(gated, policy)
    summary = _summarize(selected, candidates)
    summary["gate"] = gate
    summary["policy"] = policy
    summary["selected_rows"] = [
        {
            "source_run": row.get("source_run", ""),
            "execute_month": row.get("execute_month", ""),
            "symbol": row.get("symbol", ""),
            "top_k": _int(row, "top_k"),
            "model_name": row.get("model_name", ""),
            "trades": _int(row, "trades"),
            "realistic_return_pct": _float(row, "realistic_return_pct"),
            "validation_edge": _float(row, "validation_edge"),
            "validation_topk_sized_return_pct": _float(row, "validation_topk_sized_return_pct"),
            "validation_topk_initial_sl_rate": _float(row, "validation_topk_initial_sl_rate"),
            "validation_topk_max_dd_pct": _float(row, "validation_topk_max_dd_pct"),
            "validation_return_over_dd": round(_validation_return_over_dd(row), 6),
            "test_edge": _float(row, "test_edge"),
        }
        for row in selected
    ]
    return summary


def _gate_grid(args: argparse.Namespace):
    keys = [
        "min_validation_edge",
        "min_validation_topk_return_pct",
        "max_validation_topk_initial_sl_rate",
        "max_validation_topk_dd_pct",
        "min_validation_return_over_dd",
        "max_validation_return_over_dd",
        "max_validation_topk_return_pct",
    ]
    values = [
        args.min_validation_edges,
        args.min_validation_topk_returns,
        args.max_validation_topk_initial_sl_rates,
        args.max_validation_topk_dds,
        args.min_validation_return_over_dds,
        args.max_validation_return_over_dds,
        args.max_validation_topk_returns,
    ]
    for combo in itertools.product(*values):
        yield dict(zip(keys, combo))


def _write_markdown(result: dict, path: Path) -> None:
    lines = [
        "# Probabilistic V6 No-Trade Gate Analyzer",
        "",
        "范围：仅限 `research`。本报告只消费已有 `symbol_rows.csv`，扫描二级 no-trade gate；它不是新的实盘策略结论。",
        "",
        "## Baseline Candidates",
        "",
        "| Source | Month | Symbol | TopK | Model | Trades | Realistic | Val Edge | Val TopK Return | Val TopK SL | Val TopK DD | Val Return/DD | Test Edge |",
        "|---|---|---|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|",
    ]
    for row in result["baseline_candidates"]:
        lines.append(
            f"| `{row['source_run']}` | `{row['execute_month']}` | `{row['symbol']}` | {row['top_k']} | "
            f"`{row['model_name']}` | {row['trades']} | {row['realistic_return_pct']:.4f}% | "
            f"{row['validation_edge']:.6f} | {row['validation_topk_sized_return_pct']:.6f}% | "
            f"{row['validation_topk_initial_sl_rate']:.4f} | {row['validation_topk_max_dd_pct']:.6f}% | "
            f"{row['validation_return_over_dd']:.4f} | {row['test_edge']:.6f} |"
        )

    lines.extend(
        [
            "",
            "## Top Gate Sweeps",
            "",
            "| Rank | Policy | Active | Trades | Total Realistic | Worst Month | Gate |",
            "|---:|---|---:|---:|---:|---:|---|",
        ]
    )
    for idx, item in enumerate(result["top_results"], start=1):
        gate = item["gate"]
        gate_text = (
            f"edge>={gate['min_validation_edge']}, "
            f"ret>={gate['min_validation_topk_return_pct']}%, "
            f"SL<={gate['max_validation_topk_initial_sl_rate']}, "
            f"DD<={gate['max_validation_topk_dd_pct']}%, "
            f"ret/DD>={gate['min_validation_return_over_dd']}, "
            f"ret/DD<={gate['max_validation_return_over_dd']}, "
            f"ret<={gate['max_validation_topk_return_pct']}%"
        )
        lines.append(
            f"| {idx} | `{item['policy']}` | {item['active_rows']} | {item['trades']} | "
            f"{item['total_realistic_pct']:.4f}% | {item['worst_month_realistic_pct']:.4f}% | {gate_text} |"
        )

    if result["best_non_empty"] is not None:
        lines.extend(["", "## Best Non-Empty Selection", ""])
        best = result["best_non_empty"]
        lines.append(
            f"- policy=`{best['policy']}`，active_rows={best['active_rows']}，trades={best['trades']}，"
            f"total_realistic={best['total_realistic_pct']:.4f}%，worst_month={best['worst_month_realistic_pct']:.4f}%。"
        )
        lines.extend(
            [
                "",
                "| Source | Month | Symbol | TopK | Trades | Realistic | Val Return/DD |",
                "|---|---|---|---:|---:|---:|---:|",
            ]
        )
        for row in best["selected_rows"]:
            lines.append(
                f"| `{row['source_run']}` | `{row['execute_month']}` | `{row['symbol']}` | {row['top_k']} | "
                f"{row['trades']} | {row['realistic_return_pct']:.4f}% | {row['validation_return_over_dd']:.4f} |"
            )

    lines.extend(
        [
            "",
            "## Interpretation",
            "",
            "- 若最佳结果来自 `active_rows=0`，说明当前验证月字段只能选择空仓，不能证明概率模型已经具备可交易收益。",
            "- 若非空最佳只保留单个样本，也只能作为下一轮特征/事件来源假设，不能作为实盘候选。",
            "- Gate 只使用验证月字段；`test_edge` 只用于复盘解释，不参与筛选。",
        ]
    )
    path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Analyze V6 validation-only no-trade gates")
    parser.add_argument("--rows-csv", nargs="+", required=True)
    parser.add_argument("--output-json", required=True)
    parser.add_argument("--markdown", required=True)
    parser.add_argument("--policies", nargs="+", default=["all_sleeves", "best_validation_per_symbol_month"])
    parser.add_argument("--top-n", type=int, default=20)
    parser.add_argument("--min-validation-edges", nargs="+", default=["-999", "0.0", "0.05", "0.10", "0.20", "0.30"])
    parser.add_argument("--min-validation-topk-returns", nargs="+", default=["-999", "0.0", "0.30", "1.0", "2.0", "3.0", "4.0", "5.0"])
    parser.add_argument("--max-validation-topk-initial-sl-rates", nargs="+", default=["1.0", "0.35", "0.30", "0.25", "0.20", "0.15"])
    parser.add_argument("--max-validation-topk-dds", nargs="+", default=["100", "2.0", "1.5", "1.0", "0.75", "0.50"])
    parser.add_argument("--min-validation-return-over-dds", nargs="+", default=["-999", "0.0", "1.0", "2.0", "3.0", "4.0", "5.0"])
    parser.add_argument("--max-validation-return-over-dds", nargs="+", default=["999", "20", "10", "5", "3"])
    parser.add_argument("--max-validation-topk-returns", nargs="+", default=["999", "8", "6", "5", "3", "2"])
    args = parser.parse_args()
    args.min_validation_edges = _parse_float_grid(args.min_validation_edges)
    args.min_validation_topk_returns = _parse_float_grid(args.min_validation_topk_returns)
    args.max_validation_topk_initial_sl_rates = _parse_float_grid(args.max_validation_topk_initial_sl_rates)
    args.max_validation_topk_dds = _parse_float_grid(args.max_validation_topk_dds)
    args.min_validation_return_over_dds = _parse_float_grid(args.min_validation_return_over_dds)
    args.max_validation_return_over_dds = _parse_float_grid(args.max_validation_return_over_dds)
    args.max_validation_topk_returns = _parse_float_grid(args.max_validation_topk_returns)
    return args


def main() -> None:
    args = parse_args()
    rows = _load_rows(args.rows_csv)
    candidates = _candidate_rows(rows)
    baseline_candidates = []
    for row in candidates:
        baseline_candidates.append(
            {
                "source_run": row.get("source_run", ""),
                "execute_month": row.get("execute_month", ""),
                "symbol": row.get("symbol", ""),
                "top_k": _int(row, "top_k"),
                "model_name": row.get("model_name", ""),
                "trades": _int(row, "trades"),
                "realistic_return_pct": _float(row, "realistic_return_pct"),
                "validation_edge": _float(row, "validation_edge"),
                "validation_topk_sized_return_pct": _float(row, "validation_topk_sized_return_pct"),
                "validation_topk_initial_sl_rate": _float(row, "validation_topk_initial_sl_rate"),
                "validation_topk_max_dd_pct": _float(row, "validation_topk_max_dd_pct"),
                "validation_return_over_dd": round(_validation_return_over_dd(row), 6),
                "test_edge": _float(row, "test_edge"),
            }
        )

    results = []
    for policy in args.policies:
        for gate in _gate_grid(args):
            results.append(_evaluate(candidates, gate, policy))
    results.sort(
        key=lambda item: (
            item["total_realistic_pct"],
            item["worst_month_realistic_pct"],
            item["active_rows"],
            item["trades"],
        ),
        reverse=True,
    )
    non_empty = [item for item in results if item["active_rows"] > 0]
    best_non_empty = non_empty[0] if non_empty else None
    baseline = _summarize(_select_rows(candidates, "all_sleeves"), candidates)
    output = {
        "rows_csv": args.rows_csv,
        "candidate_rows": len(candidates),
        "baseline": baseline,
        "baseline_candidates": baseline_candidates,
        "top_results": results[: int(args.top_n)],
        "best_non_empty": best_non_empty,
    }
    output_path = Path(args.output_json)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(json.dumps(output, indent=2, ensure_ascii=False), encoding="utf-8")
    markdown_path = Path(args.markdown)
    markdown_path.parent.mkdir(parents=True, exist_ok=True)
    _write_markdown(output, markdown_path)
    print(json.dumps({"output_json": str(output_path), "markdown": str(markdown_path), "baseline": baseline, "best_non_empty": best_non_empty}, indent=2, ensure_ascii=False), flush=True)


if __name__ == "__main__":
    main()
