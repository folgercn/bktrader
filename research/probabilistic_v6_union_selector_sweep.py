#!/usr/bin/env python3
"""Sweep validation-only selectors and optionally execute true event unions.

Research-only. This is a second-stage tool for V6 probability-model sleeves:
it builds candidate sleeve selections from validation-period fields only, then
can call the event-level union runner to measure the actual de-duplicated 1s
execution result. The sweep ranking is still exploratory post-selection and
must be validated on a later holdout before being treated as tradable.
"""

from __future__ import annotations

import argparse
import csv
import heapq
import itertools
import json
import subprocess
import time
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
                row["validation_topk_sizing_markov_score_mean"] = _float(
                    row, "validation_topk_sizing_markov_score_mean"
                )
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
    validation_return_over_dd = _validation_return_over_dd(row)
    if validation_return_over_dd < float(gate["min_validation_return_over_dd"]):
        return False
    if validation_return_over_dd > float(gate["max_validation_return_over_dd"]):
        return False
    if _float(row, "validation_topk_sized_return_pct") > float(gate["max_validation_topk_return_pct"]):
        return False
    markov_score = _float(row, "validation_topk_sizing_markov_score_mean")
    if markov_score < float(gate["min_validation_topk_markov_score"]):
        return False
    if markov_score > float(gate["max_validation_topk_markov_score"]):
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
        if policy == "best_validation_per_symbol_month":
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
        else:
            raise ValueError(f"unsupported policy {policy!r}")
    return selected


def _selection_key(rows: list[dict]) -> tuple:
    return tuple(
        sorted(
            (
                str(row.get("source_run", "")),
                str(row.get("execute_month", "")),
                str(row.get("symbol", "")),
                int(_int(row, "top_k")),
            )
            for row in rows
        )
    )


def _selected_rows_payload(selected: list[dict]) -> list[dict]:
    return [
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
            "validation_topk_sizing_markov_score_mean": _float(row, "validation_topk_sizing_markov_score_mean"),
            "validation_return_over_dd": round(_validation_return_over_dd(row), 6),
            "test_edge": _float(row, "test_edge"),
        }
        for row in sorted(
            selected,
            key=lambda row: (
                str(row.get("execute_month", "")),
                str(row.get("symbol", "")),
                str(row.get("source_run", "")),
                _int(row, "top_k"),
            ),
        )
    ]


def _summarize(selected: list[dict], gate: dict, policy: str) -> dict:
    active_months = sorted({str(row.get("execute_month", "")) for row in selected})
    monthly: dict[str, float] = defaultdict(float)
    symbol_month_keys = []
    positive = 0.0
    negative = 0.0
    for row in selected:
        value = _float(row, "realistic_return_pct")
        monthly[str(row.get("execute_month", ""))] += value
        symbol_month_keys.append((str(row.get("execute_month", "")), str(row.get("symbol", ""))))
        positive += max(0.0, value)
        negative += min(0.0, value)
    monthly_values = list(monthly.values())
    return {
        "policy": policy,
        "gate": gate,
        "active_rows": int(len(selected)),
        "active_months": int(len(active_months)),
        "trades": int(sum(_int(row, "trades") for row in selected)),
        "row_sum_realistic_pct": round(float(sum(_float(row, "realistic_return_pct") for row in selected)), 6),
        "row_silo_profit_factor": round(positive / abs(negative), 6)
        if negative < 0.0
        else (999999.0 if positive > 0.0 else 0.0),
        "worst_month_row_sum_pct": round(float(min(monthly_values)), 6) if monthly_values else 0.0,
        "best_month_row_sum_pct": round(float(max(monthly_values)), 6) if monthly_values else 0.0,
        "unique_symbol_month_selection": len(symbol_month_keys) == len(set(symbol_month_keys)),
        "duplicate_symbol_month_rows": int(len(symbol_month_keys) - len(set(symbol_month_keys))),
        "selected_rows": _selected_rows_payload(selected),
    }


def _passes_portfolio(summary: dict, args: argparse.Namespace) -> bool:
    if int(summary["active_rows"]) < int(args.min_active_rows):
        return False
    if int(summary["active_months"]) < int(args.min_active_months):
        return False
    if int(summary["trades"]) < int(args.min_trades):
        return False
    if float(summary["worst_month_row_sum_pct"]) < float(args.min_worst_month_row_sum_pct):
        return False
    return True


def _sort_key(item: dict) -> tuple:
    return (
        float(item["row_sum_realistic_pct"]),
        float(item["worst_month_row_sum_pct"]),
        int(item["trades"]),
        -int(item["active_rows"]),
    )


def _push_top(heap: list[tuple], item: dict, *, counter: int, top_n: int) -> None:
    heapq.heappush(heap, (_sort_key(item), counter, item))
    if len(heap) > int(top_n):
        heapq.heappop(heap)


def _gate_grid(args: argparse.Namespace):
    keys = [
        "min_validation_edge",
        "min_validation_topk_return_pct",
        "max_validation_topk_initial_sl_rate",
        "max_validation_topk_dd_pct",
        "min_validation_return_over_dd",
        "max_validation_return_over_dd",
        "max_validation_topk_return_pct",
        "min_validation_topk_markov_score",
        "max_validation_topk_markov_score",
    ]
    values = [
        args.min_validation_edges,
        args.min_validation_topk_returns,
        args.max_validation_topk_initial_sl_rates,
        args.max_validation_topk_dds,
        args.min_validation_return_over_dds,
        args.max_validation_return_over_dds,
        args.max_validation_topk_returns,
        args.min_validation_topk_markov_scores,
        args.max_validation_topk_markov_scores,
    ]
    for combo in itertools.product(*values):
        yield dict(zip(keys, combo))


def _run_union(selection_json: Path, key: str, run_dir: Path, *, verbose: bool = False) -> dict:
    cmd = [
        "python3",
        "research/probabilistic_v6_combo_union_runner.py",
        "--selection-json",
        str(selection_json),
        "--selection-key",
        key,
        "--run-dir",
        str(run_dir),
    ]
    if verbose:
        subprocess.run(cmd, check=True)
    else:
        completed = subprocess.run(cmd, text=True, capture_output=True)
        if completed.returncode != 0:
            if completed.stdout:
                print(completed.stdout)
            if completed.stderr:
                print(completed.stderr)
            completed.check_returncode()
    return json.loads((run_dir / "summary.json").read_text(encoding="utf-8"))


def _write_markdown(result: dict, path: Path) -> None:
    lines = [
        "# Probabilistic V6 Union Selector Sweep",
        "",
        "范围：仅限 `research`。本报告扫描 validation-only selector，并对入选候选执行事件级 union 回测。",
        "",
        "## Caveat",
        "",
        "- Gate 字段只来自 validation period；`test_edge` 不参与筛选。",
        "- 候选排序仍然使用 execute-period 结果做探索性 post-selection；命中 10% 只能作为下一轮 holdout 候选，不是实盘结论。",
        "- 当前 union runner 是 one-shot 1s execution，还不是完整 `reentry_window` 生命周期。",
        "",
        "## Summary",
        "",
        f"- candidate_rows: `{result['candidate_rows']}`",
        f"- scanned_unique_selections: `{result['scanned_unique_selections']}`",
        f"- emitted_candidates: `{len(result['candidates'])}`",
        f"- union_executed: `{result['union_executed']}`",
        "",
        "## Candidate Results",
        "",
        "| Rank | Key | Policy | Rows | Months | Trades | Row Sum | Row Worst Month | Union Sum | Union Worst Silo | Union Trades | Target | Gate |",
        "|---:|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|",
    ]
    for rank, item in enumerate(result["candidates"], start=1):
        union = item.get("union_summary") or {}
        gate = item["gate"]
        gate_text = (
            f"edge>={gate['min_validation_edge']}, "
            f"ret>={gate['min_validation_topk_return_pct']}%, "
            f"SL<={gate['max_validation_topk_initial_sl_rate']}, "
            f"DD<={gate['max_validation_topk_dd_pct']}%, "
            f"ret/DD={gate['min_validation_return_over_dd']}..{gate['max_validation_return_over_dd']}, "
            f"ret<={gate['max_validation_topk_return_pct']}%, "
            f"markov={gate['min_validation_topk_markov_score']}..{gate['max_validation_topk_markov_score']}"
        )
        union_sum = union.get("active_silo_sum_pct")
        union_worst = union.get("worst_active_silo_pct")
        union_trades = union.get("trade_count")
        target = bool(union and float(union_sum) >= float(result["target_union_pct"]))
        lines.append(
            f"| {rank} | `{item['key']}` | `{item['policy']}` | {item['active_rows']} | "
            f"{item['active_months']} | {item['trades']} | {item['row_sum_realistic_pct']:.4f}% | "
            f"{item['worst_month_row_sum_pct']:.4f}% | "
            f"{float(union_sum):.4f}% | {float(union_worst):.4f}% | {int(union_trades)} | "
            f"`{target}` | {gate_text} |"
            if union
            else f"| {rank} | `{item['key']}` | `{item['policy']}` | {item['active_rows']} | "
            f"{item['active_months']} | {item['trades']} | {item['row_sum_realistic_pct']:.4f}% | "
            f"{item['worst_month_row_sum_pct']:.4f}% |  |  |  | `False` | {gate_text} |"
        )
    path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Sweep V6 union-level validation selectors")
    parser.add_argument("--rows-csv", nargs="+", required=True)
    parser.add_argument("--output-dir", required=True)
    parser.add_argument("--policies", nargs="+", default=["all_sleeves", "best_validation_per_symbol_month"])
    parser.add_argument("--top-n", type=int, default=12)
    parser.add_argument("--run-union", action="store_true")
    parser.add_argument("--verbose-union-output", action="store_true")
    parser.add_argument("--min-validation-edges", nargs="+", default=["0.0", "0.05", "0.10", "0.20"])
    parser.add_argument("--min-validation-topk-returns", nargs="+", default=["0.0", "0.3", "0.5", "1.0"])
    parser.add_argument("--max-validation-topk-initial-sl-rates", nargs="+", default=["1.0", "0.6", "0.4", "0.3"])
    parser.add_argument("--max-validation-topk-dds", nargs="+", default=["100", "3.0", "2.0"])
    parser.add_argument("--min-validation-return-over-dds", nargs="+", default=["-999", "0.0"])
    parser.add_argument("--max-validation-return-over-dds", nargs="+", default=["999", "20", "10", "8", "6"])
    parser.add_argument("--max-validation-topk-returns", nargs="+", default=["999", "6", "5", "4"])
    parser.add_argument("--min-validation-topk-markov-scores", nargs="+", default=["-999", "0.4"])
    parser.add_argument("--max-validation-topk-markov-scores", nargs="+", default=["999", "0.9", "0.85", "0.8", "0.75", "0.7"])
    parser.add_argument("--target-union-pct", type=float, default=10.0)
    parser.add_argument("--min-active-rows", type=int, default=4)
    parser.add_argument("--min-active-months", type=int, default=4)
    parser.add_argument("--min-trades", type=int, default=20)
    parser.add_argument("--min-worst-month-row-sum-pct", type=float, default=-3.0)
    args = parser.parse_args()
    args.min_validation_edges = _parse_float_grid(args.min_validation_edges)
    args.min_validation_topk_returns = _parse_float_grid(args.min_validation_topk_returns)
    args.max_validation_topk_initial_sl_rates = _parse_float_grid(args.max_validation_topk_initial_sl_rates)
    args.max_validation_topk_dds = _parse_float_grid(args.max_validation_topk_dds)
    args.min_validation_return_over_dds = _parse_float_grid(args.min_validation_return_over_dds)
    args.max_validation_return_over_dds = _parse_float_grid(args.max_validation_return_over_dds)
    args.max_validation_topk_returns = _parse_float_grid(args.max_validation_topk_returns)
    args.min_validation_topk_markov_scores = _parse_float_grid(args.min_validation_topk_markov_scores)
    args.max_validation_topk_markov_scores = _parse_float_grid(args.max_validation_topk_markov_scores)
    return args


def main() -> None:
    args = parse_args()
    started = time.time()
    output_dir = Path(args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)
    rows = _load_rows(args.rows_csv)
    candidates = _candidate_rows(rows)

    top_heap: list[tuple] = []
    seen: set[tuple] = set()
    scanned_unique = 0
    counter = 0
    for policy in args.policies:
        for gate in _gate_grid(args):
            selected = _select_rows([row for row in candidates if _passes_gate(row, gate)], policy)
            if not selected:
                continue
            key = _selection_key(selected)
            if key in seen:
                continue
            seen.add(key)
            scanned_unique += 1
            item = _summarize(selected, gate, policy)
            if not _passes_portfolio(item, args):
                continue
            _push_top(top_heap, item, counter=counter, top_n=args.top_n)
            counter += 1

    selected_items = [item for _, _, item in sorted(top_heap, key=lambda entry: entry[0], reverse=True)]
    selection_json = output_dir / "selection_candidates.json"
    selection_payload: dict[str, dict] = {}
    for idx, item in enumerate(selected_items, start=1):
        key = f"candidate_{idx:03d}"
        item["key"] = key
        selection_payload[key] = {
            "policy": item["policy"],
            "gate": item["gate"],
            "selected_rows": item["selected_rows"],
            "row_sum_realistic_pct": item["row_sum_realistic_pct"],
            "trades": item["trades"],
            "active_months": item["active_months"],
        }
    selection_json.write_text(json.dumps(selection_payload, indent=2, ensure_ascii=False), encoding="utf-8")

    if args.run_union:
        for item in selected_items:
            run_dir = output_dir / f"{item['key']}_union"
            item["union_summary"] = _run_union(
                selection_json,
                item["key"],
                run_dir,
                verbose=bool(args.verbose_union_output),
            )

    if args.run_union:
        selected_items.sort(
            key=lambda item: (
                float(item.get("union_summary", {}).get("active_silo_sum_pct", -999999.0)),
                float(item.get("union_summary", {}).get("worst_active_silo_pct", -999999.0)),
                int(item.get("union_summary", {}).get("trade_count", 0)),
            ),
            reverse=True,
        )
        for idx, item in enumerate(selected_items, start=1):
            item["rank_after_union"] = idx

    result = {
        "rows_csv": args.rows_csv,
        "output_dir": str(output_dir),
        "elapsed_seconds": round(time.time() - started, 2),
        "candidate_rows": len(candidates),
        "scanned_unique_selections": scanned_unique,
        "union_executed": bool(args.run_union),
        "target_union_pct": float(args.target_union_pct),
        "constraints": {
            "min_active_rows": int(args.min_active_rows),
            "min_active_months": int(args.min_active_months),
            "min_trades": int(args.min_trades),
            "min_worst_month_row_sum_pct": float(args.min_worst_month_row_sum_pct),
        },
        "candidates": selected_items,
    }
    (output_dir / "summary.json").write_text(json.dumps(result, indent=2, ensure_ascii=False, default=str), encoding="utf-8")
    _write_markdown(result, output_dir / "summary.md")
    print(json.dumps(result, indent=2, ensure_ascii=False, default=str), flush=True)


if __name__ == "__main__":
    main()
