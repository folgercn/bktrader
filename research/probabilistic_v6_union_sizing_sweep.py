#!/usr/bin/env python3
"""Sweep probability-linked sizing transforms for a V6 union selection.

Research-only. This driver keeps the selected sleeve set fixed and varies only
the per-event `model_notional_share` transform inside the combo union runner.
It is meant to test whether probability quality can improve the union result
without using execute-period returns as a pass/fail selector.
"""

from __future__ import annotations

import argparse
import json
import subprocess
import time
from pathlib import Path


FOCUSED_CONFIGS = [
    {"name": "baseline", "args": []},
    {"name": "cap_1p20", "args": ["--share-cap", "1.20"]},
    {"name": "cap_1p00", "args": ["--share-cap", "1.00"]},
    {"name": "cap_0p80", "args": ["--share-cap", "0.80"]},
    {"name": "mult_1p10", "args": ["--share-multiplier", "1.10"]},
    {"name": "mult_1p20", "args": ["--share-multiplier", "1.20"]},
    {"name": "mult_1p30_cap_1p80", "args": ["--share-multiplier", "1.30", "--share-cap", "1.80"]},
    {"name": "power0_fixed_1p00", "args": ["--share-power", "0.00"]},
    {"name": "power0_fixed_1p20", "args": ["--share-power", "0.00", "--share-multiplier", "1.20"]},
    {"name": "power0_fixed_1p30", "args": ["--share-power", "0.00", "--share-multiplier", "1.30"]},
    {"name": "power0p50_mult_1p30_cap_1p80", "args": ["--share-power", "0.50", "--share-multiplier", "1.30", "--share-cap", "1.80"]},
    {"name": "power0p50_mult_1p40_cap_1p80", "args": ["--share-power", "0.50", "--share-multiplier", "1.40", "--share-cap", "1.80"]},
    {"name": "power0p25_mult_1p35_cap_1p80", "args": ["--share-power", "0.25", "--share-multiplier", "1.35", "--share-cap", "1.80"]},
    {
        "name": "quality_balanced_0p6_1p25",
        "args": [
            "--sizing-profile",
            "source_quality",
            "--sizing-min-scale",
            "0.60",
            "--sizing-max-scale",
            "1.25",
        ],
    },
    {
        "name": "quality_balanced_0p5_1p40",
        "args": [
            "--sizing-profile",
            "source_quality",
            "--sizing-min-scale",
            "0.50",
            "--sizing-max-scale",
            "1.40",
        ],
    },
    {
        "name": "quality_return_heavy_0p6_1p35",
        "args": [
            "--sizing-profile",
            "source_quality",
            "--sizing-min-scale",
            "0.60",
            "--sizing-max-scale",
            "1.35",
            "--sizing-edge-weight",
            "0.10",
            "--sizing-return-weight",
            "0.35",
            "--sizing-return-dd-weight",
            "0.35",
            "--sizing-markov-weight",
            "0.20",
        ],
    },
    {
        "name": "quality_edge_return_0p6_1p35",
        "args": [
            "--sizing-profile",
            "source_quality",
            "--sizing-min-scale",
            "0.60",
            "--sizing-max-scale",
            "1.35",
            "--sizing-edge-weight",
            "0.35",
            "--sizing-return-weight",
            "0.35",
            "--sizing-return-dd-weight",
            "0.15",
            "--sizing-markov-weight",
            "0.15",
        ],
    },
    {
        "name": "quality_edge_return_mult_1p20_cap_1p80",
        "args": [
            "--sizing-profile",
            "source_quality",
            "--sizing-min-scale",
            "0.60",
            "--sizing-max-scale",
            "1.35",
            "--sizing-edge-weight",
            "0.35",
            "--sizing-return-weight",
            "0.35",
            "--sizing-return-dd-weight",
            "0.15",
            "--sizing-markov-weight",
            "0.15",
            "--share-multiplier",
            "1.20",
            "--share-cap",
            "1.80",
        ],
    },
    {
        "name": "quality_return_heavy_mult_1p15_cap_1p80",
        "args": [
            "--sizing-profile",
            "source_quality",
            "--sizing-min-scale",
            "0.60",
            "--sizing-max-scale",
            "1.35",
            "--sizing-edge-weight",
            "0.10",
            "--sizing-return-weight",
            "0.35",
            "--sizing-return-dd-weight",
            "0.35",
            "--sizing-markov-weight",
            "0.20",
            "--share-multiplier",
            "1.15",
            "--share-cap",
            "1.80",
        ],
    },
]


def _load_json(path: Path) -> dict:
    return json.loads(path.read_text(encoding="utf-8"))


def _run_config(args: argparse.Namespace, config: dict) -> dict:
    run_dir = Path(args.output_dir) / config["name"]
    cmd = [
        "python3",
        "research/probabilistic_v6_combo_union_runner.py",
        "--selection-json",
        args.selection_json,
        "--selection-key",
        args.selection_key,
        "--run-dir",
        str(run_dir),
        "--quiet",
    ] + list(config["args"])
    subprocess.run(cmd, check=True)
    summary = _load_json(run_dir / "summary.json")
    return {
        "name": config["name"],
        "args": config["args"],
        "run_dir": str(run_dir),
        "active_silo_sum_pct": summary["active_silo_sum_pct"],
        "active_months": summary["active_months"],
        "active_silos": summary["active_silos"],
        "trade_count": summary["trade_count"],
        "worst_active_silo_pct": summary["worst_active_silo_pct"],
        "best_active_silo_pct": summary["best_active_silo_pct"],
        "negative_active_silos": summary["negative_active_silos"],
        "group_rows": summary["group_rows"],
        "summary_json": str(run_dir / "summary.json"),
    }


def _write_markdown(result: dict, path: Path) -> None:
    lines = [
        "# Probabilistic V6 Union Sizing Sweep",
        "",
        "范围：仅限 `research`。固定 selection，只改变 per-event `model_notional_share` transform。",
        "",
        "## Results",
        "",
        "| Rank | Config | Return | Trades | Active Months | Worst Silo | Negative Silos | Args |",
        "|---:|---|---:|---:|---:|---:|---:|---|",
    ]
    ranked = sorted(
        result["results"],
        key=lambda row: (
            float(row["active_silo_sum_pct"]),
            float(row["worst_active_silo_pct"]),
            int(row["trade_count"]),
        ),
        reverse=True,
    )
    for idx, row in enumerate(ranked, start=1):
        args_text = " ".join(row["args"]) if row["args"] else "baseline"
        lines.append(
            f"| {idx} | `{row['name']}` | {row['active_silo_sum_pct']:.4f}% | "
            f"{row['trade_count']} | {row['active_months']} | {row['worst_active_silo_pct']:.4f}% | "
            f"{row['negative_active_silos']} | `{args_text}` |"
        )
    best = ranked[0] if ranked else None
    if best is not None:
        lines.extend(
            [
                "",
                "## Best Groups",
                "",
                "| Month | Symbol | Return | Trades | Mean Share | Mean Scale |",
                "|---|---|---:|---:|---:|---:|",
            ]
        )
        for group in best["group_rows"]:
            lines.append(
                f"| `{group['execute_month']}` | `{group['symbol']}` | "
                f"{group['realistic_return_pct']:.4f}% | {group['trades']} | "
                f"{group.get('mean_model_notional_share', 0.0):.4f} | "
                f"{group.get('mean_source_sizing_scale', 1.0):.4f} |"
            )
    path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Sweep V6 union sizing transforms")
    parser.add_argument("--selection-json", required=True)
    parser.add_argument("--selection-key", default="candidate_001")
    parser.add_argument("--output-dir", required=True)
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    started = time.time()
    output_dir = Path(args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)
    results = [_run_config(args, config) for config in FOCUSED_CONFIGS]
    result = {
        "selection_json": args.selection_json,
        "selection_key": args.selection_key,
        "output_dir": str(output_dir),
        "elapsed_seconds": round(time.time() - started, 2),
        "results": results,
    }
    (output_dir / "summary.json").write_text(json.dumps(result, indent=2, ensure_ascii=False), encoding="utf-8")
    _write_markdown(result, output_dir / "summary.md")
    print(json.dumps(result, indent=2, ensure_ascii=False), flush=True)


if __name__ == "__main__":
    main()
