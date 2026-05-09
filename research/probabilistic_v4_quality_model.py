#!/usr/bin/env python3
"""Probabilistic V4 quality model.

Research-only. Reads a post-touch event dataset, learns Markov transition
matrices from training events, scores validation events, and emits an explicit
quality rule. It does not simulate balances or mutate strategy semantics.
"""

from __future__ import annotations

import argparse
import json
from pathlib import Path

import numpy as np
import pandas as pd


def _parse_sequence(raw: object) -> list[int]:
    if not isinstance(raw, str):
        return []
    return [int(ch) for ch in raw.strip() if ch in {"0", "1", "2", "3"}]


def _transition_matrix(sequences: list[list[int]], alpha: float = 1.0) -> np.ndarray:
    matrix = np.ones((4, 4), dtype="float64") * float(alpha)
    for seq in sequences:
        if len(seq) < 2:
            continue
        for left, right in zip(seq[:-1], seq[1:]):
            matrix[int(left), int(right)] += 1.0
    matrix /= matrix.sum(axis=1, keepdims=True)
    return matrix


def _score_sequence(seq: list[int], win_matrix: np.ndarray, loss_matrix: np.ndarray) -> float:
    if len(seq) < 2:
        return 0.0
    score = 0.0
    for left, right in zip(seq[:-1], seq[1:]):
        score += float(np.log(win_matrix[int(left), int(right)] / loss_matrix[int(left), int(right)]))
    return score


def _time_split(events: pd.DataFrame, args: argparse.Namespace) -> tuple[pd.DataFrame, pd.DataFrame, str]:
    ordered = events.sort_values("touch_time").copy()
    if args.train_end:
        train_end = pd.Timestamp(args.train_end)
        if train_end.tzinfo is None:
            train_end = train_end.tz_localize("UTC")
        else:
            train_end = train_end.tz_convert("UTC")
        train = ordered[ordered["touch_time"] <= train_end]
        validation = ordered[ordered["touch_time"] > train_end]
        return train, validation, f"train_end={train_end.isoformat()}"

    split_idx = int(len(ordered) * float(args.train_ratio))
    split_idx = max(1, min(split_idx, max(1, len(ordered) - 1)))
    train = ordered.iloc[:split_idx]
    validation = ordered.iloc[split_idx:]
    split_time = ordered.iloc[split_idx]["touch_time"] if len(ordered) > split_idx else ordered.iloc[-1]["touch_time"]
    return train, validation, f"train_ratio={args.train_ratio}, split_time={split_time.isoformat()}"


def _add_markov_scores(events: pd.DataFrame, train: pd.DataFrame) -> tuple[pd.DataFrame, dict]:
    success_train = train[train["outcome"] == "continuation"]
    fail_train = train[train["outcome"] == "fail"]
    success_sequences = [_parse_sequence(raw) for raw in success_train["state_seq_60s"]]
    fail_sequences = [_parse_sequence(raw) for raw in fail_train["state_seq_60s"]]
    success_sequences = [seq for seq in success_sequences if len(seq) >= 2]
    fail_sequences = [seq for seq in fail_sequences if len(seq) >= 2]

    scored = events.copy()
    if len(success_sequences) < 3 or len(fail_sequences) < 3:
        scored["markov_llr"] = 0.0
        return scored, {
            "status": "insufficient_sequences",
            "success_sequences": len(success_sequences),
            "fail_sequences": len(fail_sequences),
        }

    win_matrix = _transition_matrix(success_sequences)
    loss_matrix = _transition_matrix(fail_sequences)
    scored["markov_llr"] = [
        _score_sequence(_parse_sequence(raw), win_matrix, loss_matrix) for raw in scored["state_seq_60s"]
    ]
    return scored, {
        "status": "trained",
        "success_sequences": len(success_sequences),
        "fail_sequences": len(fail_sequences),
        "win_matrix": win_matrix.round(6).tolist(),
        "loss_matrix": loss_matrix.round(6).tolist(),
    }


def _rule_mask(frame: pd.DataFrame, rule: dict) -> pd.Series:
    mask = pd.Series(True, index=frame.index)
    mask &= frame["markov_llr"] >= float(rule["llr_min"])
    mask &= frame["flow_ratio_60s"] >= float(rule["flow60_min"])
    mask &= frame["speed_60s_atr"] >= float(rule["speed60_min"])
    dwell_seconds = int(rule["dwell_seconds"])
    if dwell_seconds > 0:
        dwell_col = f"dwell_{dwell_seconds}s_pass"
        mask &= frame.get(dwell_col, False).astype(bool)
    pullback_max = rule.get("pullback30_max")
    if pullback_max is not None:
        mask &= frame["pullback_30s_atr"] <= float(pullback_max)
    return mask.fillna(False)


def _summarize_subset(frame: pd.DataFrame) -> dict:
    if frame.empty:
        return {
            "events": 0,
            "success_rate": 0.0,
            "fail_rate": 0.0,
            "timeout_rate": 0.0,
            "avg_first_edge_atr": 0.0,
            "avg_cost_atr": 0.0,
            "avg_net_first_edge_atr": 0.0,
            "median_seconds_to_outcome": 0.0,
        }
    return {
        "events": int(len(frame)),
        "success_rate": round(float((frame["outcome"] == "continuation").mean()) * 100.0, 4),
        "fail_rate": round(float((frame["outcome"] == "fail").mean()) * 100.0, 4),
        "timeout_rate": round(float((frame["outcome"] == "timeout").mean()) * 100.0, 4),
        "avg_first_edge_atr": round(float(frame["first_edge_atr"].mean()), 6),
        "avg_cost_atr": round(float(frame["roundtrip_cost_atr"].mean()), 6),
        "avg_net_first_edge_atr": round(float(frame["net_first_edge_atr"].mean()), 6),
        "median_seconds_to_outcome": round(float(frame["seconds_to_outcome"].median()), 2),
    }


def _candidate_rules(args: argparse.Namespace) -> list[dict]:
    rules: list[dict] = []
    for llr_min in args.llr_mins:
        for flow_min in args.flow60_mins:
            for speed_min in args.speed60_mins:
                for dwell_seconds in args.dwell_seconds:
                    for pullback_raw in args.pullback30_maxes:
                        pullback_max = None if str(pullback_raw).lower() in {"none", "nan"} else float(pullback_raw)
                        rules.append(
                            {
                                "llr_min": float(llr_min),
                                "flow60_min": float(flow_min),
                                "speed60_min": float(speed_min),
                                "dwell_seconds": int(dwell_seconds),
                                "pullback30_max": pullback_max,
                            }
                        )
    return rules


def _select_rule(validation: pd.DataFrame, args: argparse.Namespace) -> tuple[dict, list[dict]]:
    scored_rules: list[dict] = []
    for rule in _candidate_rules(args):
        subset = validation[_rule_mask(validation, rule)]
        summary = _summarize_subset(subset)
        if summary["events"] < int(args.min_events):
            continue
        scored_rules.append({"rule": rule, "validation": summary})
    if not scored_rules:
        relaxed = {
            "llr_min": -999.0,
            "flow60_min": 0.0,
            "speed60_min": -999.0,
            "dwell_seconds": 0,
            "pullback30_max": None,
        }
        return relaxed, [{"rule": relaxed, "validation": _summarize_subset(validation)}]

    scored_rules.sort(
        key=lambda item: (
            item["validation"]["avg_net_first_edge_atr"],
            item["validation"]["success_rate"],
            item["validation"]["events"],
        ),
        reverse=True,
    )
    return scored_rules[0]["rule"], scored_rules


def _write_markdown(summary: dict, path: Path) -> None:
    if summary.get("selection_scope") == "per_symbol":
        lines = [
            "# Probabilistic V4 Quality Model",
            "",
            "范围：仅限 `research`。本文件只描述 event quality 规则，不代表 live 语义或资金曲线结论。",
            "",
            "## Selection Scope",
            "",
            "- `per_symbol`: 每个 symbol 单独训练 Markov transition matrices，并在各自 validation subset 上选择规则。",
            "",
            "## Selected Rules",
            "",
            "| Symbol | Events | LLR | Flow60 | Speed60 | Dwell | Pullback30 | Net Edge ATR | Success | Fail | Baseline Net Edge ATR |",
            "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|",
        ]
        for symbol, selected in summary["selected_by_symbol"].items():
            rule = selected["rule"]
            validation = selected["validation"]
            baseline = summary["validation_baseline_by_symbol"].get(symbol, {})
            lines.append(
                f"| `{symbol}` | {validation['events']} | {rule['llr_min']} | {rule['flow60_min']} | "
                f"{rule['speed60_min']} | {rule['dwell_seconds']} | {rule['pullback30_max']} | "
                f"{validation['avg_net_first_edge_atr']:.6f} | {validation['success_rate']:.4f}% | "
                f"{validation['fail_rate']:.4f}% | {baseline.get('avg_net_first_edge_atr', 0.0):.6f} |"
            )
        lines.extend(["", "## Top Rules By Symbol", ""])
        for symbol, items in summary["top_rules_by_symbol"].items():
            lines.extend(
                [
                    f"### {symbol}",
                    "",
                    "| Rank | Events | LLR | Flow60 | Speed60 | Dwell | Pullback30 | Net Edge ATR | Success | Fail |",
                    "|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|",
                ]
            )
            for idx, item in enumerate(items, 1):
                rule = item["rule"]
                validation = item["validation"]
                lines.append(
                    f"| {idx} | {validation['events']} | {rule['llr_min']} | {rule['flow60_min']} | "
                    f"{rule['speed60_min']} | {rule['dwell_seconds']} | {rule['pullback30_max']} | "
                    f"{validation['avg_net_first_edge_atr']:.6f} | {validation['success_rate']:.4f}% | "
                    f"{validation['fail_rate']:.4f}% |"
                )
            lines.append("")
        path.write_text("\n".join(lines) + "\n", encoding="utf-8")
        return

    selected = summary["selected_rule"]
    lines = [
        "# Probabilistic V4 Quality Model",
        "",
        "范围：仅限 `research`。本文件只描述 event quality 规则，不代表 live 语义或资金曲线结论。",
        "",
        "## Selected Rule",
        "",
        f"- `llr_min`: `{selected['llr_min']}`",
        f"- `flow60_min`: `{selected['flow60_min']}`",
        f"- `speed60_min`: `{selected['speed60_min']}`",
        f"- `dwell_seconds`: `{selected['dwell_seconds']}`",
        f"- `pullback30_max`: `{selected['pullback30_max']}`",
        "",
        "## Validation",
        "",
        "| Events | Success | Fail | Timeout | Avg Net Edge ATR | Avg Cost ATR | Median Outcome Seconds |",
        "|---:|---:|---:|---:|---:|---:|---:|",
    ]
    v = summary["selected_validation"]
    lines.append(
        f"| {v['events']} | {v['success_rate']:.4f}% | {v['fail_rate']:.4f}% | {v['timeout_rate']:.4f}% | "
        f"{v['avg_net_first_edge_atr']:.6f} | {v['avg_cost_atr']:.6f} | {v['median_seconds_to_outcome']:.2f} |"
    )
    lines.extend(
        [
            "",
            "## Top Rules",
            "",
            "| Rank | Events | LLR | Flow60 | Speed60 | Dwell | Pullback30 | Net Edge ATR | Success | Fail |",
            "|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|",
        ]
    )
    for idx, item in enumerate(summary["top_rules"], 1):
        rule = item["rule"]
        validation = item["validation"]
        lines.append(
            f"| {idx} | {validation['events']} | {rule['llr_min']} | {rule['flow60_min']} | "
            f"{rule['speed60_min']} | {rule['dwell_seconds']} | {rule['pullback30_max']} | "
            f"{validation['avg_net_first_edge_atr']:.6f} | {validation['success_rate']:.4f}% | "
            f"{validation['fail_rate']:.4f}% |"
        )
    path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Train Probabilistic V4 quality model")
    parser.add_argument("--events-csv", default="research/probabilistic_v4_events.csv")
    parser.add_argument("--scored-csv", default="research/probabilistic_v4_events_scored.csv")
    parser.add_argument("--rules-json", default="research/probabilistic_v4_quality_rules.json")
    parser.add_argument("--markdown", default="research/20260508_probabilistic_v4_quality_model.md")
    parser.add_argument("--train-end", default="")
    parser.add_argument("--train-ratio", type=float, default=0.70)
    parser.add_argument("--selection-scope", choices=["global", "per_symbol"], default="global")
    parser.add_argument("--min-events", type=int, default=20)
    parser.add_argument("--llr-mins", nargs="+", type=float, default=[-999.0, 0.0, 2.0, 4.0, 6.0])
    parser.add_argument("--flow60-mins", nargs="+", type=float, default=[0.0, 0.55, 0.60, 0.65])
    parser.add_argument("--speed60-mins", nargs="+", type=float, default=[-999.0, 0.0, 0.03, 0.08])
    parser.add_argument("--dwell-seconds", nargs="+", type=int, default=[0, 5, 15, 30])
    parser.add_argument("--pullback30-maxes", nargs="+", default=["none", "0.05", "0.10", "0.20"])
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    events = pd.read_csv(args.events_csv, parse_dates=["touch_time", "signal_start", "signal_end"])
    if events.empty:
        raise SystemExit(f"empty events dataset: {args.events_csv}")

    train, validation, split_description = _time_split(events, args)
    if args.selection_scope == "global":
        scored, markov_summary = _add_markov_scores(events, train)
        scored_train = scored.loc[train.index]
        scored_validation = scored.loc[validation.index]
        selected_rule, scored_rules = _select_rule(scored_validation, args)
        scored["quality_pass"] = _rule_mask(scored, selected_rule)
        scored["quality_bucket"] = np.where(scored["quality_pass"], "selected", "rejected")

        top_rules = scored_rules[:25]
        summary = {
            "events_csv": args.events_csv,
            "split": split_description,
            "selection_scope": "global",
            "train_rows": int(len(scored_train)),
            "validation_rows": int(len(scored_validation)),
            "train_baseline": _summarize_subset(scored_train),
            "validation_baseline": _summarize_subset(scored_validation),
            "markov": markov_summary,
            "selected_rule": selected_rule,
            "selected_validation": _summarize_subset(scored_validation[_rule_mask(scored_validation, selected_rule)]),
            "top_rules": top_rules,
        }
    else:
        scored = events.copy()
        scored["markov_llr"] = 0.0
        scored["quality_pass"] = False
        selected_by_symbol: dict[str, dict] = {}
        top_rules_by_symbol: dict[str, list[dict]] = {}
        markov_by_symbol: dict[str, dict] = {}
        train_baseline_by_symbol: dict[str, dict] = {}
        validation_baseline_by_symbol: dict[str, dict] = {}

        for symbol in sorted(events["symbol"].dropna().unique()):
            symbol_mask = events["symbol"] == symbol
            symbol_events = events[symbol_mask]
            symbol_train = train[train["symbol"] == symbol]
            symbol_validation = validation[validation["symbol"] == symbol]
            symbol_scored, symbol_markov = _add_markov_scores(symbol_events, symbol_train)
            scored.loc[symbol_scored.index, "markov_llr"] = symbol_scored["markov_llr"]
            scored_train = scored.loc[symbol_train.index]
            scored_validation = scored.loc[symbol_validation.index]
            selected_rule, scored_rules = _select_rule(scored_validation, args)
            scored.loc[symbol_events.index, "quality_pass"] = _rule_mask(scored.loc[symbol_events.index], selected_rule)

            selected_by_symbol[str(symbol)] = {
                "rule": selected_rule,
                "validation": _summarize_subset(scored_validation[_rule_mask(scored_validation, selected_rule)]),
            }
            top_rules_by_symbol[str(symbol)] = scored_rules[:25]
            markov_by_symbol[str(symbol)] = symbol_markov
            train_baseline_by_symbol[str(symbol)] = _summarize_subset(scored_train)
            validation_baseline_by_symbol[str(symbol)] = _summarize_subset(scored_validation)

        scored["quality_bucket"] = np.where(scored["quality_pass"], "selected", "rejected")
        summary = {
            "events_csv": args.events_csv,
            "split": split_description,
            "selection_scope": "per_symbol",
            "train_rows": int(len(train)),
            "validation_rows": int(len(validation)),
            "train_baseline": _summarize_subset(scored.loc[train.index]),
            "validation_baseline": _summarize_subset(scored.loc[validation.index]),
            "train_baseline_by_symbol": train_baseline_by_symbol,
            "validation_baseline_by_symbol": validation_baseline_by_symbol,
            "markov_by_symbol": markov_by_symbol,
            "selected_rule": {"scope": "per_symbol", "rules_by_symbol": {k: v["rule"] for k, v in selected_by_symbol.items()}},
            "selected_by_symbol": selected_by_symbol,
            "top_rules_by_symbol": top_rules_by_symbol,
        }

    scored_path = Path(args.scored_csv)
    scored.to_csv(scored_path, index=False)
    summary["scored_csv"] = str(scored_path)
    rules_path = Path(args.rules_json)
    rules_path.write_text(json.dumps(summary, indent=2, ensure_ascii=False), encoding="utf-8")
    _write_markdown(summary, Path(args.markdown))
    print(
        json.dumps(
            {"rules_json": str(rules_path), "scored_csv": str(scored_path), "selected_rule": summary["selected_rule"]},
            indent=2,
        ),
        flush=True,
    )


if __name__ == "__main__":
    main()
