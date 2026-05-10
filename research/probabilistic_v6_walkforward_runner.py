#!/usr/bin/env python3
"""Probabilistic V6 walk-forward runner.

Research-only. Orchestrates per-symbol monthly train/validation/execute
splits over execution-labeled events, then runs the V4 1s execution simulator
with optional top-K portfolio selection.
"""

from __future__ import annotations

import argparse
import calendar
import json
import subprocess
import time
from pathlib import Path

import numpy as np
import pandas as pd


ARCHIVED_SCHEME_B_BASELINE = {
    "name": "delay60 + feature60 + post_selection gate",
    "run_dir": "research/probabilistic_v6_runs/walkforward_delay60_original_t2_feature60_postselect_gate",
    "active_silo_sum_pct": 6.0939,
    "active_months": 5,
    "trade_count": 51,
}

VALIDATION_TOPK_FEATURE_COLUMNS = {
    "prob_success": "prob_success",
    "prob_ev_atr": "prob_ev_atr",
    "prob_initial_sl": "prob_initial_sl",
    "model_notional_share": "model_notional_share",
    "sizing_score": "sizing_score",
    "sizing_ev_score": "sizing_ev_score",
    "sizing_prob_score": "sizing_prob_score",
    "sizing_markov_score": "sizing_markov_score",
    "sizing_sl_score": "sizing_sl_score",
    "speed_60s_atr": "speed_60s_atr",
    "eff_60s": "eff_60s",
    "close_pos_60s": "close_pos_60s",
    "pullback_60s_atr": "pullback_60s_atr",
    "flow_ratio_60s": "flow_ratio_60s",
}


def _as_bool_series(series: pd.Series) -> pd.Series:
    if series.dtype == bool:
        return series
    return series.astype(str).str.lower().isin({"true", "1", "yes"})


def _month_end(period: pd.Period) -> pd.Timestamp:
    last_day = calendar.monthrange(int(period.year), int(period.month))[1]
    return pd.Timestamp(f"{period.year:04d}-{period.month:02d}-{last_day:02d}T23:59:59Z")


def _month_start(period: pd.Period) -> pd.Timestamp:
    return pd.Timestamp(f"{period.year:04d}-{period.month:02d}-01T00:00:00Z")


def _run(cmd: list[str]) -> None:
    print(" ".join(cmd), flush=True)
    subprocess.run(cmd, check=True)


def _summary_value(summary: dict, path: list[str], default=0.0):
    current = summary
    for key in path:
        if not isinstance(current, dict) or key not in current:
            return default
        current = current[key]
    return current


def _load_json(path: Path) -> dict:
    return json.loads(path.read_text(encoding="utf-8"))


def _gate_model(model: dict, args: argparse.Namespace) -> tuple[bool, str]:
    validation = model.get("validation_selected", {})
    train = model.get("train_selected", {})
    if int(validation.get("events", 0)) < int(args.min_validation_events):
        return False, f"validation_events<{args.min_validation_events}"
    if float(validation.get("avg_net_first_edge_atr", 0.0)) < float(args.min_validation_edge):
        return False, f"validation_edge<{args.min_validation_edge}"
    if float(validation.get("success_rate", 0.0)) < float(args.min_validation_success_rate):
        return False, f"validation_success<{args.min_validation_success_rate}"
    if args.require_positive_train and float(train.get("avg_net_first_edge_atr", 0.0)) <= 0.0:
        return False, "train_edge<=0"
    return True, "pass"


def _apply_top_k(
    scored_path: Path,
    output_path: Path,
    *,
    execute_start: pd.Timestamp,
    execute_end: pd.Timestamp,
    top_k: int,
    rank_by: str,
    max_share: float | None,
    fixed_notional_share: float | None = None,
) -> dict:
    scored = pd.read_csv(scored_path, parse_dates=["touch_time"])
    if "quality_pass" not in scored.columns:
        raise ValueError(f"missing quality_pass column in {scored_path}")
    quality = _as_bool_series(scored["quality_pass"])
    execute_mask = (scored["touch_time"] >= execute_start) & (scored["touch_time"] <= execute_end)
    selected_mask = quality & execute_mask
    before = int(selected_mask.sum())

    if max_share is not None and "model_notional_share" in scored.columns:
        scored["model_notional_share"] = pd.to_numeric(scored["model_notional_share"], errors="coerce").clip(
            lower=0.0,
            upper=float(max_share),
        )

    if top_k > 0 and before > top_k:
        if rank_by not in scored.columns:
            raise ValueError(f"rank column {rank_by!r} not found in {scored_path}")
        rank_values = pd.to_numeric(scored.loc[selected_mask, rank_by], errors="coerce").fillna(-np.inf)
        keep_index = rank_values.sort_values(ascending=False).head(int(top_k)).index
        scored.loc[execute_mask, "quality_pass"] = False
        scored.loc[keep_index, "quality_pass"] = True
    after = int((_as_bool_series(scored["quality_pass"]) & execute_mask).sum())

    if fixed_notional_share is not None:
        if "model_notional_share" not in scored.columns:
            scored["model_notional_share"] = 0.0
        scored["model_notional_share"] = pd.to_numeric(scored["model_notional_share"], errors="coerce").fillna(0.0)
        final_mask = _as_bool_series(scored["quality_pass"]) & execute_mask
        scored.loc[final_mask, "model_notional_share"] = float(fixed_notional_share)

    output_path.parent.mkdir(parents=True, exist_ok=True)
    scored.to_csv(output_path, index=False)
    return {"selected_before_top_k": before, "selected_after_top_k": after}


def _quality_mask(frame: pd.DataFrame) -> pd.Series:
    if "quality_pass" not in frame.columns:
        raise ValueError("missing quality_pass column")
    return _as_bool_series(frame["quality_pass"])


def _ranked_top_k_frame(
    scored: pd.DataFrame,
    *,
    period_start: pd.Timestamp,
    period_end: pd.Timestamp,
    top_k: int,
    rank_by: str,
) -> tuple[pd.DataFrame, int]:
    period_mask = (scored["touch_time"] >= period_start) & (scored["touch_time"] <= period_end)
    selected = scored.loc[_quality_mask(scored) & period_mask].copy()
    before = int(len(selected))
    if int(top_k) > 0 and before > int(top_k):
        if rank_by not in selected.columns:
            raise ValueError(f"rank column {rank_by!r} not found")
        selected["_rank_value"] = pd.to_numeric(selected[rank_by], errors="coerce").fillna(-np.inf)
        selected = selected.sort_values("_rank_value", ascending=False).head(int(top_k)).drop(columns=["_rank_value"])
    return selected, before


def _validation_top_k_metrics(
    scored_path: Path,
    *,
    validation_start: pd.Timestamp,
    validation_end: pd.Timestamp,
    top_k_values: list[int],
    rank_by: str,
    default_share: float,
) -> dict[int, dict]:
    scored = pd.read_csv(scored_path, parse_dates=["touch_time"])

    def feature_metrics(selected: pd.DataFrame) -> dict:
        metrics = {}
        for name, column in VALIDATION_TOPK_FEATURE_COLUMNS.items():
            if column not in selected.columns or selected.empty:
                metrics[f"validation_topk_{name}_mean"] = 0.0
                metrics[f"validation_topk_{name}_std"] = 0.0
                continue
            values = pd.to_numeric(selected[column], errors="coerce").replace([np.inf, -np.inf], np.nan).dropna()
            metrics[f"validation_topk_{name}_mean"] = round(float(values.mean()), 6) if not values.empty else 0.0
            metrics[f"validation_topk_{name}_std"] = round(float(values.std(ddof=0)), 6) if len(values) > 1 else 0.0
        return metrics

    metrics: dict[int, dict] = {}
    for top_k in top_k_values:
        selected, before = _ranked_top_k_frame(
            scored,
            period_start=validation_start,
            period_end=validation_end,
            top_k=int(top_k),
            rank_by=rank_by,
        )
        if selected.empty:
            metrics[int(top_k)] = {
                "validation_topk_events": 0,
                "validation_topk_before": before,
                "validation_topk_return_pct": 0.0,
                "validation_topk_sized_return_pct": 0.0,
                "validation_topk_return_over_dd": 0.0,
                "validation_topk_initial_sl_rate": 0.0,
                "validation_topk_win_rate_pct": 0.0,
                "validation_topk_max_dd_pct": 0.0,
                **feature_metrics(selected),
            }
            continue
        returns = pd.to_numeric(selected.get("execution_return_pct", 0.0), errors="coerce").fillna(0.0)
        if "model_notional_share" in selected.columns:
            share = pd.to_numeric(selected["model_notional_share"], errors="coerce")
        else:
            share = pd.Series(float(default_share), index=selected.index)
        share = share.where(np.isfinite(share) & (share > 0.0), float(default_share)).fillna(float(default_share))
        sized_returns = returns * share
        ordered_returns = sized_returns.loc[selected.sort_values("touch_time").index]
        equity = ordered_returns.cumsum()
        drawdown = equity - equity.cummax()
        max_dd = round(float(drawdown.min()), 6) if not drawdown.empty else 0.0
        sized_return = round(float(sized_returns.sum()), 6)
        metrics[int(top_k)] = {
            "validation_topk_events": int(len(selected)),
            "validation_topk_before": before,
            "validation_topk_return_pct": round(float(returns.sum()), 6),
            "validation_topk_sized_return_pct": sized_return,
            "validation_topk_return_over_dd": round(sized_return / max(0.25, abs(max_dd)), 6),
            "validation_topk_initial_sl_rate": round(float(selected["execution_exit_reason"].astype(str).eq("InitialSL").mean()), 6)
            if "execution_exit_reason" in selected.columns
            else 0.0,
            "validation_topk_win_rate_pct": round(float((returns > 0.0).mean()) * 100.0, 4),
            "validation_topk_max_dd_pct": max_dd,
            **feature_metrics(selected),
        }
    return metrics


def _top_k_score(metrics: dict, args: argparse.Namespace) -> float:
    metric = str(args.top_k_selection_metric)
    if metric == "raw_return":
        return float(metrics.get("validation_topk_return_pct", 0.0))
    if metric == "win_rate":
        return float(metrics.get("validation_topk_win_rate_pct", 0.0))
    if metric == "return_over_drawdown":
        dd = abs(float(metrics.get("validation_topk_max_dd_pct", 0.0)))
        return float(metrics.get("validation_topk_sized_return_pct", 0.0)) / max(0.25, dd)
    return float(metrics.get("validation_topk_sized_return_pct", 0.0))


def _passes_validation_top_k_candidate(metrics: dict, args: argparse.Namespace) -> bool:
    events = int(metrics.get("validation_topk_events", 0))
    if events < int(args.min_validation_topk_events):
        return False
    if float(metrics.get("validation_topk_sized_return_pct", 0.0)) < float(args.min_validation_topk_return_pct):
        return False
    if float(metrics.get("validation_topk_initial_sl_rate", 0.0)) > float(args.max_validation_topk_initial_sl_rate):
        return False
    if abs(float(metrics.get("validation_topk_max_dd_pct", 0.0))) > float(args.max_validation_topk_dd_pct):
        return False
    return True


def _validation_top_k_post_gate(metrics: dict, args: argparse.Namespace) -> tuple[bool, str]:
    sized_return = float(metrics.get("validation_topk_sized_return_pct", 0.0))
    return_over_dd = float(metrics.get("validation_topk_return_over_dd", 0.0))
    if sized_return > float(args.max_validation_topk_return_pct):
        return False, f"validation_topk_return>{args.max_validation_topk_return_pct}"
    if return_over_dd < float(args.min_validation_topk_return_over_dd):
        return False, f"validation_topk_return_over_dd<{args.min_validation_topk_return_over_dd}"
    if return_over_dd > float(args.max_validation_topk_return_over_dd):
        return False, f"validation_topk_return_over_dd>{args.max_validation_topk_return_over_dd}"
    return True, "pass"


def _choose_validation_top_k(metrics_by_k: dict[int, dict], args: argparse.Namespace) -> tuple[int | None, str]:
    candidates: list[tuple[float, int, dict]] = []
    for top_k, metrics in metrics_by_k.items():
        if not _passes_validation_top_k_candidate(metrics, args):
            continue
        if str(args.validation_topk_gate_stage) == "candidate_filter":
            post_pass, _ = _validation_top_k_post_gate(metrics, args)
            if not post_pass:
                continue
        candidates.append((_top_k_score(metrics, args), int(top_k), metrics))
    if not candidates:
        return None, "validation_topk_no_candidate"
    candidates.sort(
        key=lambda item: (
            item[0],
            item[2].get("validation_topk_sized_return_pct", 0.0),
            -abs(item[2].get("validation_topk_max_dd_pct", 0.0)),
            -item[1] if item[1] > 0 else -9999,
        ),
        reverse=True,
    )
    selected_top_k = int(candidates[0][1])
    if str(args.validation_topk_gate_stage) == "post_selection":
        post_pass, post_reason = _validation_top_k_post_gate(candidates[0][2], args)
        if not post_pass:
            return None, post_reason
    return selected_top_k, "pass"


def _add_validation_top_k_fields(row: dict, metrics_by_k: dict[int, dict], top_k: int, selected_top_k: int | None, args: argparse.Namespace) -> dict:
    metrics = metrics_by_k.get(int(top_k), {})
    row.update(metrics)
    row["top_k_policy"] = str(args.top_k_policy)
    row["validation_selected_top_k"] = "" if selected_top_k is None else int(selected_top_k)
    row["is_validation_selected_top_k"] = selected_top_k is not None and int(top_k) == int(selected_top_k)
    row.setdefault("sizing_mode", "hybrid_markov")
    row.setdefault("sizing_fallback_reason", "")
    row.setdefault("final_sizing_result", "")
    row.setdefault("dynamic_summary_json", "")
    row.setdefault("fixed_fallback_summary_json", "")
    row.setdefault("dynamic_trades", 0)
    row.setdefault("dynamic_realistic_return_pct", 0.0)
    row.setdefault("fixed_fallback_trades", 0)
    row.setdefault("fixed_fallback_realistic_return_pct", 0.0)
    row.setdefault("lifecycle_claim", "Baseline_Derived_Sizing")
    row.setdefault("full_reentry_window_lifecycle", False)
    return row


def _sizing_decision(symbol: str, metrics: dict, args: argparse.Namespace) -> tuple[str, str, bool]:
    """Return final sizing mode, fallback reason, and whether fixed execution is required."""
    if str(symbol).upper() != "BTCUSDT":
        return "hybrid_markov", "", False
    if str(args.btc_sizing_fallback_mode) == "off":
        return "hybrid_markov", "", False

    initial_sl_rate = float(metrics.get("validation_topk_initial_sl_rate", 0.0))
    threshold = float(args.btc_dynamic_initial_sl_rate_max)
    if initial_sl_rate > threshold:
        return (
            "fixed20",
            f"btc_validation_topk_initial_sl_rate={initial_sl_rate:.4f}>{threshold:.4f}",
            True,
        )
    return "hybrid_markov", "", False


def _run_execution_variant(
    *,
    execution_scored_csv: Path,
    model_json: Path,
    summary_json: Path,
    markdown: Path,
    ledger_prefix: Path,
    symbol: str,
    execute_start: pd.Timestamp,
    execute_end: pd.Timestamp,
    args: argparse.Namespace,
) -> dict:
    execution_cmd = [
        "python3",
        "research/probabilistic_v4_execution_runner.py",
        "--events-csv",
        str(execution_scored_csv),
        "--rules-json",
        str(model_json),
        "--symbols",
        symbol,
        "--start",
        args.start,
        "--end",
        args.end,
        "--execute-start",
        execute_start.isoformat(),
        "--execute-end",
        execute_end.isoformat(),
        "--archive-root",
        args.archive_root,
        "--chunksize",
        str(args.chunksize),
        "--entry-delay-seconds",
        str(args.entry_delay_seconds),
        "--initial-stop-atr",
        str(args.initial_stop_atr),
        "--breakeven-at-r",
        str(args.breakeven_at_r),
        "--trail-start-r",
        str(args.trail_start_r),
        "--max-hold-hours",
        str(args.max_hold_hours),
        "--initial-balance",
        str(args.initial_balance),
        "--notional-share",
        str(args.notional_share),
        "--summary-json",
        str(summary_json),
        "--markdown",
        str(markdown),
        "--ledger-prefix",
        str(ledger_prefix),
    ]
    if args.bars_cache_dir:
        execution_cmd.extend(["--bars-cache-dir", args.bars_cache_dir])
    _run(execution_cmd)
    return _load_json(summary_json)


def _first_result(summary: dict) -> dict:
    return summary.get("results", [{}])[0]


def _event_source_contract(args: argparse.Namespace) -> tuple[str, str]:
    labeled_path = str(args.labeled_csv).lower()
    if "baseline_plus_t3" in labeled_path:
        return (
            "baseline_plus_t3 intrabar touch",
            "long/short use Original_T2 prev_high_2/prev_low_2 intrabar touch plus t3_swing structural touch when enabled",
        )
    return (
        "true Original_T2 intrabar touch",
        "long uses current unclosed signal bar 1s high >= prev_high_2; short uses 1s low <= prev_low_2",
    )


def _scheme_semantic_contract(args: argparse.Namespace) -> dict:
    entry_source, breakout_semantic = _event_source_contract(args)
    return {
        "scheme": "Scheme B: delay60 + feature60 + post_selection gate",
        "scope": "research-only",
        "entry_source": entry_source,
        "breakout_semantic": breakout_semantic,
        "feature_horizon_seconds": int(args.feature_horizon_seconds),
        "entry_delay_seconds": int(args.entry_delay_seconds),
        "feature_horizon_lte_entry_delay": int(args.feature_horizon_seconds) <= int(args.entry_delay_seconds),
        "execution_model": "research/probabilistic_v4_execution_runner.py 1s execution runner",
        "sizing_mode": "ETH uses hybrid_markov dynamic event sizing; BTC falls back to fixed20 when validation InitialSL rate is too high",
        "btc_sizing_fallback": {
            "mode": str(args.btc_sizing_fallback_mode),
            "dynamic_initial_sl_rate_max": float(args.btc_dynamic_initial_sl_rate_max),
            "fixed_notional_share": float(args.notional_share),
        },
        "baseline_derived_sizing": True,
        "full_reentry_window_lifecycle": False,
        "research_baseline_reference": {
            "source": "AGENTS §2 Core Memory",
            "dir2_zero_initial": True,
            "zero_initial_mode": "reentry_window",
            "reentry_size_schedule": [0.20, 0.10],
            "max_trades_per_bar": 2,
        },
        "current_runner_gap": (
            "Current V6 runner performs event selection plus one-shot 1s execution. "
            "It does not implement current+next signal-bar reentry windows or slot0/slot1 lifecycle."
        ),
    }


def _final_selection_rows(rows: list[dict], args: argparse.Namespace) -> tuple[list[dict], str]:
    gate_pass_rows = [row for row in rows if bool(row.get("gate_pass", False))]
    if str(args.top_k_policy) == "validation_best":
        return (
            [row for row in gate_pass_rows if bool(row.get("is_validation_selected_top_k", False))],
            "validation_best_selected_rows",
        )
    top_k_values = [int(value) for value in args.top_k_values]
    if len(top_k_values) == 1:
        only_top_k = int(top_k_values[0])
        return (
            [row for row in gate_pass_rows if int(row.get("top_k", -1)) == only_top_k],
            f"single_top_k:{only_top_k}",
        )
    return gate_pass_rows, "all_gate_pass_rows_multiple_topk_variants"


def _metrics_for_rows(
    selected_rows: list[dict],
    *,
    execute_months: list[str],
    symbols: list[str],
    selection_scope: str,
) -> dict:
    active_rows = [row for row in selected_rows if int(row.get("trades", 0)) > 0]
    active_month_set = {str(row.get("execute_month", "")) for row in active_rows}
    execute_month_count = len(execute_months)
    calendar_symbol_month_count = execute_month_count * len(symbols)
    active_silo_sum = float(np.sum([float(row.get("realistic_return_pct", 0.0)) for row in active_rows])) if active_rows else 0.0
    trade_count = int(np.sum([int(row.get("trades", 0)) for row in active_rows])) if active_rows else 0
    active_returns = [float(row.get("realistic_return_pct", 0.0)) for row in active_rows]
    calendar_normalized = active_silo_sum / float(calendar_symbol_month_count) if calendar_symbol_month_count else 0.0
    return {
        "selection_scope": selection_scope,
        "active_silo_sum_pct": round(active_silo_sum, 6),
        "calendar_normalized_return_pct": round(calendar_normalized, 6),
        "active_months": int(len(active_month_set)),
        "empty_months": int(max(0, execute_month_count - len(active_month_set))),
        "execute_months": execute_months,
        "execute_month_count": int(execute_month_count),
        "symbol_count": int(len(symbols)),
        "calendar_symbol_month_count": int(calendar_symbol_month_count),
        "active_silos": int(len(active_rows)),
        "trade_count": trade_count,
        "worst_active_silo_pct": round(float(min(active_returns)), 6) if active_returns else 0.0,
        "best_active_silo_pct": round(float(max(active_returns)), 6) if active_returns else 0.0,
        "negative_active_silos": int(sum(1 for value in active_returns if value < 0.0)),
    }


def _compute_run_metrics(rows: list[dict], execute_months: list[str], symbols: list[str], args: argparse.Namespace) -> dict:
    final_rows, selection_scope = _final_selection_rows(rows, args)
    final_metrics = _metrics_for_rows(
        final_rows,
        execute_months=execute_months,
        symbols=symbols,
        selection_scope=selection_scope,
    )
    final_metrics["baseline_comparison"] = {
        "baseline": ARCHIVED_SCHEME_B_BASELINE,
        "active_silo_sum_delta_pct": round(
            final_metrics["active_silo_sum_pct"] - float(ARCHIVED_SCHEME_B_BASELINE["active_silo_sum_pct"]),
            6,
        ),
        "active_month_delta": int(final_metrics["active_months"] - int(ARCHIVED_SCHEME_B_BASELINE["active_months"])),
        "trade_count_delta": int(final_metrics["trade_count"] - int(ARCHIVED_SCHEME_B_BASELINE["trade_count"])),
    }
    by_top_k = []
    for top_k in [int(value) for value in args.top_k_values]:
        top_k_rows = [
            row
            for row in rows
            if bool(row.get("gate_pass", False)) and int(row.get("top_k", -1)) == int(top_k)
        ]
        by_top_k.append(
            {
                "top_k": int(top_k),
                **_metrics_for_rows(
                    top_k_rows,
                    execute_months=execute_months,
                    symbols=symbols,
                    selection_scope=f"top_k:{top_k}",
                ),
            }
        )
    final_metrics["by_top_k"] = by_top_k
    return final_metrics


def _write_markdown(
    rows: list[dict],
    portfolio_rows: list[dict],
    run_metrics: dict,
    scheme_contract: dict,
    path: Path,
) -> None:
    lines = [
        "# Probabilistic V6 Walk-Forward",
        "",
        "范围：仅限 `research`。本报告用 execution-aware labels 做按月 walk-forward，并用真实 1s execution runner 回测 selected events。",
        "",
        "## Scheme Semantic Contract",
        "",
        "| Field | Value |",
        "|---|---|",
        f"| Scheme | `{scheme_contract['scheme']}` |",
        f"| Entry Source | `{scheme_contract['entry_source']}` |",
        f"| Breakout Semantic | `{scheme_contract['breakout_semantic']}` |",
        f"| Feature / Entry Delay | `{scheme_contract['feature_horizon_seconds']}s <= {scheme_contract['entry_delay_seconds']}s` |",
        f"| Execution Model | `{scheme_contract['execution_model']}` |",
        f"| Sizing Mode | `{scheme_contract['sizing_mode']}` |",
        f"| BTC Fallback | `mode={scheme_contract['btc_sizing_fallback']['mode']}`, `InitialSL_rate<={scheme_contract['btc_sizing_fallback']['dynamic_initial_sl_rate_max']}`, `fixed_share={scheme_contract['btc_sizing_fallback']['fixed_notional_share']}` |",
        f"| Lifecycle Claim | `Baseline_Derived_Sizing` |",
        f"| Full Reentry Window Lifecycle | `{scheme_contract['full_reentry_window_lifecycle']}` |",
        f"| Research Baseline | `dir2_zero_initial=true`, `zero_initial_mode=reentry_window`, `reentry_size_schedule=[0.20, 0.10]`, `max_trades_per_bar=2` (AGENTS §2 Core Memory) |",
        f"| Current Runner Gap | {scheme_contract['current_runner_gap']} |",
        "",
        "## Run Metrics",
        "",
        "Calendar_Normalized_Return 将空仓 symbol-month silo 按 0% 计入后，对 execute_month × symbol 固定网格取平均。",
        "",
        "| Metric | Value |",
        "|---|---:|",
        f"| Active_Silo_Sum | {run_metrics['active_silo_sum_pct']:.4f}% |",
        f"| Calendar_Normalized_Return | {run_metrics['calendar_normalized_return_pct']:.4f}% |",
        f"| Active Months | {run_metrics['active_months']} |",
        f"| Empty Months | {run_metrics['empty_months']} |",
        f"| Active Silos | {run_metrics['active_silos']} |",
        f"| Calendar Symbol-Month Count | {run_metrics['calendar_symbol_month_count']} |",
        f"| Trades | {run_metrics['trade_count']} |",
        f"| Worst Active Silo | {run_metrics['worst_active_silo_pct']:.4f}% |",
        "",
        "## Baseline Comparison",
        "",
        "| Baseline | Active_Silo_Sum | Active Months | Trades | Delta Active_Silo_Sum |",
        "|---|---:|---:|---:|---:|",
        f"| `{ARCHIVED_SCHEME_B_BASELINE['name']}` | {ARCHIVED_SCHEME_B_BASELINE['active_silo_sum_pct']:.4f}% | "
        f"{ARCHIVED_SCHEME_B_BASELINE['active_months']} | {ARCHIVED_SCHEME_B_BASELINE['trade_count']} | "
        f"{run_metrics['baseline_comparison']['active_silo_sum_delta_pct']:.4f}% |",
        "",
        "## Portfolio",
        "",
        "| Month | TopK | Active | Equal Weight Realistic | Silo Sum Realistic | Symbols |",
        "|---|---:|---:|---:|---:|---|",
    ]
    for row in portfolio_rows:
        lines.append(
            f"| `{row['execute_month']}` | {row['top_k']} | {row['active_symbols']} | "
            f"{row['equal_weight_realistic_pct']:.4f}% | {row['silo_sum_realistic_pct']:.4f}% | "
            f"`{row['symbols']}` |"
        )

    lines.extend(
        [
            "",
            "## Symbol Rows",
            "",
            "| Month | Symbol | TopK | Gate | Sizing | Final | Fallback | Dynamic Ret | Fixed Ret | Model | Selected | Trades | Realistic | PF | Win | DD | Val Edge | Val TopK Return | Val Ret/DD | Val TopK SL | Val Markov | Test Label Edge |",
            "|---|---|---:|---|---|---|---|---:|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|",
        ]
    )
    for row in rows:
        fallback_reason = str(row.get("sizing_fallback_reason", ""))
        lines.append(
            f"| `{row['execute_month']}` | `{row['symbol']}` | {row['top_k']} | `{row['gate_reason']}` | "
            f"`{row.get('sizing_mode', '')}` | `{row.get('final_sizing_result', '')}` | `{fallback_reason}` | "
            f"{float(row.get('dynamic_realistic_return_pct', 0.0)):.4f}% | "
            f"{float(row.get('fixed_fallback_realistic_return_pct', 0.0)):.4f}% | "
            f"`{row['model_name']}` | {row['selected_events']} | {row['trades']} | "
            f"{row['realistic_return_pct']:.4f}% | {row['profit_factor']} | {row['win_rate_pct']:.2f}% | "
            f"{row['max_dd_pct']:.4f}% | {row['validation_edge']:.6f} | "
            f"{row.get('validation_topk_sized_return_pct', 0.0):.6f}% | "
            f"{row.get('validation_topk_return_over_dd', 0.0):.6f} | "
            f"{row.get('validation_topk_initial_sl_rate', 0.0):.4f} | "
            f"{row.get('validation_topk_sizing_markov_score_mean', 0.0):.4f} | {row['test_edge']:.6f} |"
        )
    path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Run V6 execution-aware monthly walk-forward")
    parser.add_argument("--labeled-csv", required=True)
    parser.add_argument("--run-dir", required=True)
    parser.add_argument("--symbols", nargs="+", default=["BTCUSDT", "ETHUSDT"])
    parser.add_argument("--start", required=True)
    parser.add_argument("--end", required=True)
    parser.add_argument("--train-months", type=int, default=1)
    parser.add_argument("--validation-months", type=int, default=1)
    parser.add_argument("--models", nargs="+", default=["logistic", "random_forest", "extra_trees", "gradient_boosting", "svm_rbf"])
    parser.add_argument("--objective", default="sum_net_edge", choices=["sum_net_edge", "avg_net_edge", "sum_prob_ev", "sum_sized_net_edge"])
    parser.add_argument("--min-events", type=int, default=8)
    parser.add_argument("--prob-mins", nargs="+", default=["0.40", "0.45", "0.50", "0.55", "0.60", "0.65", "0.70", "0.75", "0.80"])
    parser.add_argument("--ev-atr-mins", nargs="+", default=["-0.05", "0.00", "0.02", "0.05", "0.08", "0.10", "0.15", "0.20"])
    parser.add_argument("--sl-prob-maxes", nargs="+", default=["0.35", "0.50", "0.65", "0.80", "1.00"])
    parser.add_argument("--top-k-values", nargs="+", type=int, default=[0, 5, 10, 15, 20])
    parser.add_argument("--rank-by", default="model_notional_share")
    parser.add_argument("--top-k-policy", default="all", choices=["all", "validation_best"])
    parser.add_argument(
        "--top-k-selection-metric",
        default="sized_return",
        choices=["sized_return", "raw_return", "win_rate", "return_over_drawdown"],
    )
    parser.add_argument("--min-validation-topk-events", type=int, default=0)
    parser.add_argument("--min-validation-topk-return-pct", type=float, default=0.0)
    parser.add_argument("--max-validation-topk-return-pct", type=float, default=999999.0)
    parser.add_argument("--min-validation-topk-return-over-dd", type=float, default=-999999.0)
    parser.add_argument("--max-validation-topk-return-over-dd", type=float, default=999999.0)
    parser.add_argument("--max-validation-topk-initial-sl-rate", type=float, default=1.0)
    parser.add_argument("--max-validation-topk-dd-pct", type=float, default=100.0)
    parser.add_argument(
        "--validation-topk-gate-stage",
        default="candidate_filter",
        choices=["candidate_filter", "post_selection"],
        help="candidate_filter can fall back to another topK; post_selection rejects the chosen topK instead.",
    )
    parser.add_argument("--cap-dynamic-share", type=float, default=0.0, help="0 keeps model share unchanged; >0 caps model_notional_share")
    parser.add_argument("--min-validation-events", type=int, default=8)
    parser.add_argument("--min-validation-edge", type=float, default=0.0)
    parser.add_argument("--min-validation-success-rate", type=float, default=0.0)
    parser.add_argument("--require-positive-train", action="store_true")
    parser.add_argument("--skip-failed-validation", action="store_true")
    parser.add_argument("--bars-cache-dir", default="")
    parser.add_argument("--archive-root", default="dataset/archive")
    parser.add_argument("--chunksize", type=int, default=5_000_000)
    parser.add_argument("--entry-delay-seconds", type=int, default=5)
    parser.add_argument(
        "--feature-horizon-seconds",
        type=int,
        default=5,
        help="Maximum post-touch feature horizon passed to V5; must not exceed entry delay.",
    )
    parser.add_argument("--initial-stop-atr", type=float, default=0.45)
    parser.add_argument("--breakeven-at-r", type=float, default=0.8)
    parser.add_argument("--trail-start-r", type=float, default=0.9)
    parser.add_argument("--max-hold-hours", type=float, default=4.0)
    parser.add_argument("--initial-balance", type=float, default=100000.0)
    parser.add_argument("--notional-share", type=float, default=0.20)
    parser.add_argument(
        "--btc-sizing-fallback-mode",
        default="fixed20_on_initial_sl",
        choices=["fixed20_on_initial_sl", "off"],
        help="For BTCUSDT, select fixed notional_share when validation topK InitialSL rate is too high.",
    )
    parser.add_argument("--btc-dynamic-initial-sl-rate-max", type=float, default=0.30)
    parser.add_argument("--sizing-ev-weight", type=float, default=0.45)
    parser.add_argument("--sizing-prob-weight", type=float, default=0.25)
    parser.add_argument("--sizing-markov-weight", type=float, default=0.30)
    parser.add_argument("--sizing-sl-weight", type=float, default=0.20)
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    if int(args.feature_horizon_seconds) > int(args.entry_delay_seconds):
        raise SystemExit(
            f"feature_horizon_seconds={args.feature_horizon_seconds} exceeds "
            f"entry_delay_seconds={args.entry_delay_seconds}; that would leak post-entry data"
        )
    started = time.time()
    run_dir = Path(args.run_dir)
    run_dir.mkdir(parents=True, exist_ok=True)

    events = pd.read_csv(args.labeled_csv, parse_dates=["touch_time", "signal_start", "signal_end"])
    events = events[events["symbol"].isin(args.symbols)].copy()
    if events.empty:
        raise SystemExit(f"empty labeled event dataset: {args.labeled_csv}")
    events["month"] = events["touch_time"].dt.tz_convert(None).dt.to_period("M")
    months = sorted(events["month"].unique())
    min_required = int(args.train_months) + int(args.validation_months)
    if len(months) <= min_required:
        raise SystemExit(f"not enough months for walk-forward: months={months}")

    rows: list[dict] = []
    portfolio_rows: list[dict] = []
    for execute_pos in range(min_required, len(months)):
        execute_month = months[execute_pos]
        train_months = months[execute_pos - min_required : execute_pos - int(args.validation_months)]
        validation_months = months[execute_pos - int(args.validation_months) : execute_pos]
        train_end = _month_end(train_months[-1])
        validation_start = _month_start(validation_months[0])
        validation_end = _month_end(validation_months[-1])
        execute_start = _month_start(execute_month)
        execute_end = _month_end(execute_month)
        split_dir = run_dir / f"execute_{execute_month}"
        split_dir.mkdir(parents=True, exist_ok=True)

        top_k_symbol_rows: dict[int, list[dict]] = {int(value): [] for value in args.top_k_values}
        for symbol in args.symbols:
            symbol_events = events[events["symbol"] == symbol].drop(columns=["month"]).copy()
            if symbol_events.empty:
                continue
            symbol_dir = split_dir / symbol
            symbol_dir.mkdir(parents=True, exist_ok=True)
            symbol_csv = symbol_dir / "events_labeled_symbol.csv"
            symbol_events.to_csv(symbol_csv, index=False)

            model_json = symbol_dir / "ml_model.json"
            scored_csv = symbol_dir / "events_scored.csv"
            markdown = symbol_dir / "ml_model.md"
            model_cmd = [
                "python3",
                "research/probabilistic_v5_ml_probability_model.py",
                "--events-csv",
                str(symbol_csv),
                "--scored-csv",
                str(scored_csv),
                "--model-json",
                str(model_json),
                "--markdown",
                str(markdown),
                "--train-end",
                train_end.isoformat(),
                "--validation-end",
                validation_end.isoformat(),
                "--min-events",
                str(args.min_events),
                "--objective",
                args.objective,
                "--sizing-mode",
                "hybrid_markov",
                "--feature-horizon-seconds",
                str(args.feature_horizon_seconds),
                "--min-share",
                "0.20",
                "--max-share",
                "1.50",
                "--models",
                *args.models,
                "--prob-mins",
                *[str(value) for value in args.prob_mins],
                "--ev-atr-mins",
                *[str(value) for value in args.ev_atr_mins],
                "--sl-prob-maxes",
                *[str(value) for value in args.sl_prob_maxes],
                "--sizing-ev-weight",
                str(args.sizing_ev_weight),
                "--sizing-prob-weight",
                str(args.sizing_prob_weight),
                "--sizing-markov-weight",
                str(args.sizing_markov_weight),
                "--sizing-sl-weight",
                str(args.sizing_sl_weight),
            ]
            _run(model_cmd)
            model = _load_json(model_json)
            gate_pass, gate_reason = _gate_model(model, args)
            validation_top_k_metrics = _validation_top_k_metrics(
                scored_csv,
                validation_start=validation_start,
                validation_end=validation_end,
                top_k_values=[int(value) for value in args.top_k_values],
                rank_by=args.rank_by,
                default_share=float(args.notional_share),
            )
            selected_top_k: int | None = None
            if gate_pass and args.top_k_policy == "validation_best":
                selected_top_k, top_k_gate_reason = _choose_validation_top_k(validation_top_k_metrics, args)
                if selected_top_k is None:
                    gate_pass = False
                    gate_reason = top_k_gate_reason
            if args.skip_failed_validation and not gate_pass:
                for top_k in args.top_k_values:
                    row = {
                        "execute_month": str(execute_month),
                        "symbol": symbol,
                        "top_k": int(top_k),
                        "gate_pass": False,
                        "gate_reason": gate_reason,
                        "model_name": model.get("selected_rule", {}).get("model_name", ""),
                        "selected_events": 0,
                        "trades": 0,
                        "realistic_return_pct": 0.0,
                        "profit_factor": 0.0,
                        "win_rate_pct": 0.0,
                        "max_dd_pct": 0.0,
                        "validation_edge": float(model.get("validation_selected", {}).get("avg_net_first_edge_atr", 0.0)),
                        "test_edge": float(model.get("test_selected", {}).get("avg_net_first_edge_atr", 0.0)),
                        "summary_json": "",
                    }
                    rows.append(_add_validation_top_k_fields(row, validation_top_k_metrics, int(top_k), selected_top_k, args))
                continue

            for top_k in args.top_k_values:
                if args.top_k_policy == "validation_best" and selected_top_k is not None and int(top_k) != int(selected_top_k):
                    row = {
                        "execute_month": str(execute_month),
                        "symbol": symbol,
                        "top_k": int(top_k),
                        "gate_pass": False,
                        "gate_reason": f"top_k_not_selected:{selected_top_k}",
                        "model_name": model.get("selected_rule", {}).get("model_name", ""),
                        "selected_events": 0,
                        "trades": 0,
                        "realistic_return_pct": 0.0,
                        "profit_factor": 0.0,
                        "win_rate_pct": 0.0,
                        "max_dd_pct": 0.0,
                        "validation_edge": float(model.get("validation_selected", {}).get("avg_net_first_edge_atr", 0.0)),
                        "validation_events": int(model.get("validation_selected", {}).get("events", 0)),
                        "test_edge": float(model.get("test_selected", {}).get("avg_net_first_edge_atr", 0.0)),
                        "test_events": int(model.get("test_selected", {}).get("events", 0)),
                        "summary_json": "",
                    }
                    rows.append(_add_validation_top_k_fields(row, validation_top_k_metrics, int(top_k), selected_top_k, args))
                    continue
                variant_dir = symbol_dir / f"topk_{int(top_k)}"
                variant_dir.mkdir(parents=True, exist_ok=True)
                execution_scored_csv = variant_dir / "events_scored_for_execution.csv"
                top_k_metrics = validation_top_k_metrics.get(int(top_k), {})
                sizing_mode, sizing_fallback_reason, use_fixed_fallback = _sizing_decision(symbol, top_k_metrics, args)

                dynamic_summary_json = variant_dir / "execution_dynamic_summary.json"
                dynamic_control_summary: dict | None = None
                if use_fixed_fallback:
                    dynamic_scored_csv = variant_dir / "events_scored_dynamic_control.csv"
                    _apply_top_k(
                        scored_csv,
                        dynamic_scored_csv,
                        execute_start=execute_start,
                        execute_end=execute_end,
                        top_k=int(top_k),
                        rank_by=args.rank_by,
                        max_share=float(args.cap_dynamic_share) if float(args.cap_dynamic_share) > 0 else None,
                    )
                    dynamic_control_summary = _run_execution_variant(
                        execution_scored_csv=dynamic_scored_csv,
                        model_json=model_json,
                        summary_json=dynamic_summary_json,
                        markdown=variant_dir / "execution_dynamic.md",
                        ledger_prefix=variant_dir / "tmp_execution_dynamic",
                        symbol=symbol,
                        execute_start=execute_start,
                        execute_end=execute_end,
                        args=args,
                    )

                topk_info = _apply_top_k(
                    scored_csv,
                    execution_scored_csv,
                    execute_start=execute_start,
                    execute_end=execute_end,
                    top_k=int(top_k),
                    rank_by=args.rank_by,
                    max_share=float(args.cap_dynamic_share) if float(args.cap_dynamic_share) > 0 else None,
                    fixed_notional_share=float(args.notional_share) if use_fixed_fallback else None,
                )
                summary_json = variant_dir / "execution_summary.json"
                execution_summary = _run_execution_variant(
                    execution_scored_csv=execution_scored_csv,
                    model_json=model_json,
                    summary_json=summary_json,
                    markdown=variant_dir / "execution.md",
                    ledger_prefix=variant_dir / "tmp_execution",
                    symbol=symbol,
                    execute_start=execute_start,
                    execute_end=execute_end,
                    args=args,
                )
                result = _first_result(execution_summary)
                dynamic_result = _first_result(dynamic_control_summary) if dynamic_control_summary is not None else result
                row = {
                    "execute_month": str(execute_month),
                    "symbol": symbol,
                    "top_k": int(top_k),
                    "gate_pass": bool(gate_pass),
                    "gate_reason": gate_reason,
                    "model_name": model.get("selected_rule", {}).get("model_name", ""),
                    "selected_events": int(topk_info["selected_after_top_k"]),
                    "selected_before_top_k": int(topk_info["selected_before_top_k"]),
                    "trades": int(_summary_value(result, ["summary", "trades"], 0)),
                    "realistic_return_pct": float(_summary_value(result, ["summary", "realistic_return_pct"], 0.0)),
                    "profit_factor": _summary_value(result, ["attribution", "profit_factor"], 0.0),
                    "win_rate_pct": float(_summary_value(result, ["summary", "win_rate_pct"], 0.0)),
                    "max_dd_pct": float(_summary_value(result, ["summary", "max_dd_pct"], 0.0)),
                    "validation_edge": float(model.get("validation_selected", {}).get("avg_net_first_edge_atr", 0.0)),
                    "validation_events": int(model.get("validation_selected", {}).get("events", 0)),
                    "test_edge": float(model.get("test_selected", {}).get("avg_net_first_edge_atr", 0.0)),
                    "test_events": int(model.get("test_selected", {}).get("events", 0)),
                    "summary_json": str(summary_json),
                    "sizing_mode": sizing_mode,
                    "sizing_fallback_reason": sizing_fallback_reason,
                    "final_sizing_result": "fixed_fallback" if use_fixed_fallback else "dynamic",
                    "dynamic_summary_json": str(dynamic_summary_json) if use_fixed_fallback else str(summary_json),
                    "fixed_fallback_summary_json": str(summary_json) if use_fixed_fallback else "",
                    "dynamic_trades": int(_summary_value(dynamic_result, ["summary", "trades"], 0)),
                    "dynamic_realistic_return_pct": float(_summary_value(dynamic_result, ["summary", "realistic_return_pct"], 0.0)),
                    "fixed_fallback_trades": int(_summary_value(result, ["summary", "trades"], 0)) if use_fixed_fallback else 0,
                    "fixed_fallback_realistic_return_pct": float(_summary_value(result, ["summary", "realistic_return_pct"], 0.0))
                    if use_fixed_fallback
                    else 0.0,
                }
                row = _add_validation_top_k_fields(row, validation_top_k_metrics, int(top_k), selected_top_k, args)
                rows.append(row)
                if gate_pass:
                    top_k_symbol_rows[int(top_k)].append(row)

        for top_k, symbol_rows in top_k_symbol_rows.items():
            active = [row for row in symbol_rows if int(row.get("trades", 0)) > 0]
            if active:
                equal_weight = float(np.mean([float(row["realistic_return_pct"]) for row in active]))
                silo_sum = float(np.sum([float(row["realistic_return_pct"]) for row in active]))
                symbols = ",".join(row["symbol"] for row in active)
            else:
                equal_weight = 0.0
                silo_sum = 0.0
                symbols = ""
            portfolio_rows.append(
                {
                    "execute_month": str(execute_month),
                    "top_k": int(top_k),
                    "active_symbols": int(len(active)),
                    "equal_weight_realistic_pct": round(equal_weight, 6),
                    "silo_sum_realistic_pct": round(silo_sum, 6),
                    "symbols": symbols,
                }
            )

    rows_df = pd.DataFrame(rows)
    portfolio_df = pd.DataFrame(portfolio_rows)
    rows_path = run_dir / "symbol_rows.csv"
    portfolio_path = run_dir / "portfolio_rows.csv"
    rows_df.to_csv(rows_path, index=False)
    portfolio_df.to_csv(portfolio_path, index=False)
    execute_months = [str(period) for period in months[min_required:]]
    run_metrics = _compute_run_metrics(rows, execute_months, [str(symbol) for symbol in args.symbols], args)
    scheme_contract = _scheme_semantic_contract(args)
    summary = {
        "labeled_csv": args.labeled_csv,
        "run_dir": str(run_dir),
        "rows_csv": str(rows_path),
        "portfolio_csv": str(portfolio_path),
        "elapsed_seconds": round(time.time() - started, 2),
        "scheme_semantic_contract": scheme_contract,
        "run_metrics": run_metrics,
        "active_silo_sum_pct": run_metrics["active_silo_sum_pct"],
        "calendar_normalized_return_pct": run_metrics["calendar_normalized_return_pct"],
        "active_months": run_metrics["active_months"],
        "empty_months": run_metrics["empty_months"],
        "calendar_symbol_month_count": run_metrics["calendar_symbol_month_count"],
        "baseline_comparison": run_metrics["baseline_comparison"],
        "config": vars(args),
    }
    (run_dir / "summary.json").write_text(json.dumps(summary, indent=2, ensure_ascii=False, default=str), encoding="utf-8")
    _write_markdown(rows, portfolio_rows, run_metrics, scheme_contract, run_dir / "summary.md")
    print(json.dumps(summary, indent=2, ensure_ascii=False, default=str), flush=True)


if __name__ == "__main__":
    main()
