"""multi_timeframe_runner — 多时间框架 pretouch 事件 pipeline 执行器

从 multi_timeframe_builder 产出的 30min/1h/4h 事件 CSV 出发，
对每个时间框架独立跑 delay simulation + timing classification + sizing，
最终产出跨时间框架对比报告。

用法：
    python -m timing_probability_unified.multi_timeframe_runner [--timeframes 30min,1h,4h] [--symbol ETHUSDT]

输出：
    research/entry_redesign/scripts/output/multi_timeframe/
      ├── pipeline_30min_summary.json
      ├── pipeline_1h_summary.json
      ├── pipeline_4h_summary.json
      ├── cross_timeframe_comparison.md
      └── cross_timeframe_comparison.json
"""

from __future__ import annotations

import json
import logging
import sys
import time
from dataclasses import asdict, dataclass
from pathlib import Path

import numpy as np
import pandas as pd

# Ensure parent scripts dir is importable
_SCRIPTS_DIR = Path(__file__).resolve().parents[1]
if str(_SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS_DIR))

from timing_probability_unified.multi_timeframe_builder import (
    BARS_CACHE_DIR,
    OUTPUT_DIR as BUILDER_OUTPUT_DIR,
    TIMEFRAME_CONFIGS,
    load_all_1s_bars,
)
from timing_probability_unified.event_source_builder import (
    split_events_by_time,
    compute_event_pool_stats,
)
from timing_probability_unified.timing_classifier import (
    generate_3regime_labels,
    train_and_select,
    get_selected_delay_pnl,
)
from timing_probability_unified.probability_model import (
    compute_sizing_multiplier,
    generate_rf_binary_labels,
    train_rf_probability,
)
from timing_probability_unified.combined_executor import (
    CombinedPositionConfig,
    compute_calendar_sum,
    compute_combined_positions,
    compute_worst_sm,
)
from timing_probability_unified.speed_gate import compute_speed_gate
from pre_breakout_timing.feature_extractor import extract_features, impute_features
from dynamic_timing.execution_sim import DEFAULT_EXEC_PARAMS

logger = logging.getLogger(__name__)

RANDOM_STATE = 42
PIPELINE_OUTPUT_DIR = BUILDER_OUTPUT_DIR  # same dir as builder output

# Execution params — we override DEFAULT_EXEC_PARAMS at runtime for max_hold_hours
# These are the baseline values (aligned with unified_runner)
EXEC_PARAMS_REFERENCE = {
    "initial_stop_atr": 0.45,
    "breakeven_at_r": 0.8,
    "trail_start_r": 1.5,  # worktree validated override
    "trail_buffer_atr": 0.05,
    "max_hold_hours": 4.0,
    "min_stop_bps": 12.0,
    "slippage": 0.0002,
    "entry_fee": 0.0002,
    "exit_fee": 0.0004,
}


@dataclass
class TimeframePipelineResult:
    """单个时间框架的 pipeline 结果"""

    timeframe: str
    total_events: int
    train_events: int
    test_events: int
    forward_events: int
    long_count: int
    short_count: int
    # Delay simulation
    delay_sim_traded: int
    delay_sim_skipped: int
    # Timing classifier
    timing_selected_depth: int
    timing_dt3_loocv_cs: float
    timing_dt4_loocv_cs: float
    timing_test_cs: float
    # RF probability
    rf_train_auc: float
    rf_test_auc: float
    rf_no_signal: bool
    # Speed gate
    speed_gate_threshold: float
    speed_gate_pass_rate: float
    # Combined results
    calendar_sum_gate_on: float
    calendar_sum_gate_off: float
    worst_sm_gate_on: float
    trade_count_gate_on: int
    neg_sm_count: int
    # Per-month breakdown
    monthly_silo_returns: dict[str, float]
    # Timing
    pipeline_seconds: float


def _load_timeframe_events(symbol: str, timeframe: str) -> pd.DataFrame:
    """加载 multi_timeframe_builder 产出的事件 CSV。"""
    csv_path = BUILDER_OUTPUT_DIR / f"pretouch_events_{symbol}_{timeframe}.csv"
    if not csv_path.exists():
        raise FileNotFoundError(
            f"Multi-timeframe events CSV not found: {csv_path}\n"
            f"请先运行 multi_timeframe_builder.py 生成事件。"
        )
    df = pd.read_csv(csv_path)
    df["touch_time"] = pd.to_datetime(df["touch_time"], utc=True)
    logger.info("Loaded %d events from %s", len(df), csv_path.name)
    return df


def _load_bars_cache_for_symbol(symbol: str) -> dict[str, pd.DataFrame]:
    """加载指定 symbol 的所有月份 1s bar cache。"""
    pattern = f"{symbol}_*_flow_1s.pkl"
    files = sorted(BARS_CACHE_DIR.glob(pattern))
    cache: dict[str, pd.DataFrame] = {}

    for f in files:
        # Extract month from filename: ETHUSDT_20250601T000000_20250630T235959_flow_1s.pkl
        parts = f.stem.replace("_flow_1s", "").split("_")
        if len(parts) < 3:
            continue
        try:
            start_str = parts[1]  # "20250601T000000"
            month_key = f"{symbol}_{start_str[:6]}"  # "ETHUSDT_202506"
            bars = pd.read_pickle(f)
            if month_key in cache:
                # Merge with existing (handle overlapping files)
                cache[month_key] = pd.concat([cache[month_key], bars]).sort_index()
                cache[month_key] = cache[month_key][~cache[month_key].index.duplicated(keep='first')]
            else:
                cache[month_key] = bars
        except Exception as e:
            logger.warning("Failed to load %s: %s", f.name, e)

    logger.info("Loaded bars cache: %d months for %s", len(cache), symbol)
    return cache


def _simulate_delays_for_timeframe(
    events: pd.DataFrame,
    bars_cache: dict[str, pd.DataFrame],
    max_hold_hours: float,
) -> list[list]:
    """对事件池执行 delay simulation。

    对每个事件模拟 D=0, D=5, D=10, D=15 + pullback 五种 delay。
    接口与 unified_runner._simulate_delays_for_events 对齐。
    """
    from pre_breakout_timing.delay_simulator import DelayResult, simulate_all_delays

    # Override max_hold_hours in global exec params for this timeframe
    original_max_hold = DEFAULT_EXEC_PARAMS.get("max_hold_hours", 4.0)
    DEFAULT_EXEC_PARAMS["max_hold_hours"] = max_hold_hours

    pullback_params = {
        "pullback_target_atr": 0.05,
        "pullback_window_seconds": 60,
        "start_offset_seconds": 5,
    }

    results = []
    errors = 0

    for idx in range(len(events)):
        event = events.iloc[idx]
        event_id = str(event.get("event_id", f"evt_{idx}"))

        symbol = str(event["symbol"])
        touch_time = pd.Timestamp(event["touch_time"])
        if touch_time.tzinfo is None:
            touch_time = touch_time.tz_localize("UTC")

        month_key = f"{symbol}_{touch_time.strftime('%Y%m')}"
        bars = bars_cache.get(month_key)

        if bars is None or bars.empty:
            # No data — produce untradeable placeholder
            placeholder = [
                DelayResult(
                    event_id=event_id, delay_label=label, delay_seconds=0,
                    entry_time=None, entry_price=None, pnl_pct=None,
                    exit_reason="NoData", exit_time=None, hold_seconds=None,
                    mfe_r=None, mae_r=None, traded=False,
                )
                for label in ["D0", "D5", "D10", "D15", "pullback"]
            ]
            results.append(placeholder)
            errors += 1
            continue

        try:
            event_delays = simulate_all_delays(
                event=event,
                second_bars=bars,
                pullback_params=pullback_params,
            )
            results.append(event_delays)
        except Exception as e:
            placeholder = [
                DelayResult(
                    event_id=event_id, delay_label=label, delay_seconds=0,
                    entry_time=None, entry_price=None, pnl_pct=None,
                    exit_reason="SimError", exit_time=None, hold_seconds=None,
                    mfe_r=None, mae_r=None, traded=False,
                )
                for label in ["D0", "D5", "D10", "D15", "pullback"]
            ]
            results.append(placeholder)
            errors += 1
            if errors <= 5:
                logger.warning("Delay sim error for event %s: %s", event_id, e)

    # Restore original max_hold_hours
    DEFAULT_EXEC_PARAMS["max_hold_hours"] = original_max_hold

    logger.info("Delay simulation: %d events, %d errors", len(results), errors)
    return results


def run_timeframe_pipeline(
    symbol: str,
    timeframe: str,
    forward_start: str = "2025-11-01",
) -> TimeframePipelineResult:
    """对单个时间框架执行完整 pipeline。"""
    pipeline_start = time.time()
    tf_config = TIMEFRAME_CONFIGS[timeframe]

    logger.info("=" * 60)
    logger.info("Pipeline: %s %s (max_hold=%.1fh)", symbol, timeframe, tf_config.max_hold_hours)
    logger.info("=" * 60)

    # Step 1: Load events
    events = _load_timeframe_events(symbol, timeframe)

    # Step 2: Load bars cache
    bars_cache = _load_bars_cache_for_symbol(symbol)

    # Step 3: Split events
    train_events, test_events, forward_events = split_events_by_time(
        events, forward_start=forward_start, train_ratio=0.6
    )
    logger.info("Split: train=%d, test=%d, forward=%d",
                len(train_events), len(test_events), len(forward_events))

    # Step 4: Delay simulation (train + test only for full-window)
    full_window_events = pd.concat([train_events, test_events], ignore_index=True)
    n_train = len(train_events)
    n_test = len(test_events)

    # Override trail_start_r (worktree validated +4pp improvement)
    _original_trail_start = DEFAULT_EXEC_PARAMS.get("trail_start_r", 0.9)
    DEFAULT_EXEC_PARAMS["trail_start_r"] = 1.5

    logger.info("Running delay simulation for %d full-window events...", len(full_window_events))
    delay_results_full = _simulate_delays_for_timeframe(
        full_window_events, bars_cache, tf_config.max_hold_hours
    )

    delay_results_train = delay_results_full[:n_train]
    delay_results_test = delay_results_full[n_train:]

    # Restore trail_start_r
    DEFAULT_EXEC_PARAMS["trail_start_r"] = _original_trail_start

    # Count traded
    traded_count = sum(
        1 for dr_list in delay_results_full
        if any(dr.traded for dr in dr_list)
    )

    # Step 5: Generate 3-regime labels
    train_labels = generate_3regime_labels(delay_results_train)
    test_labels = generate_3regime_labels(delay_results_test)

    # Step 6: Extract features
    train_features_raw, used_features, excluded_features = extract_features(train_events)
    test_features_raw, _, _ = extract_features(test_events)

    # Align test features to train columns
    for col in used_features:
        if col not in test_features_raw.columns:
            test_features_raw[col] = np.nan
    test_features_raw = test_features_raw[used_features]

    # Impute
    train_features, test_features, _ = impute_features(train_features_raw, test_features_raw)

    # Step 7: Train timing classifier
    logger.info("Training timing classifier...")
    timing_result = train_and_select(
        train_features=train_features,
        train_labels=train_labels,
        delay_results_train=delay_results_train,
        test_features=test_features,
        test_labels=test_labels,
        delay_results_test=delay_results_test,
        train_events=train_events,
        test_events=test_events,
    )

    full_window_predictions = np.concatenate(
        [timing_result.train_predictions, timing_result.test_predictions]
    )

    # Step 8: Train RF probability model
    logger.info("Training RF probability model...")
    train_delay_pnls = pd.Series([
        get_selected_delay_pnl(timing_result.train_predictions[i], delay_results_train[i])[1]
        for i in range(n_train)
    ], dtype=float)
    test_delay_pnls = pd.Series([
        get_selected_delay_pnl(timing_result.test_predictions[i], delay_results_test[i])[1]
        for i in range(n_test)
    ], dtype=float)

    train_rf_labels = generate_rf_binary_labels(
        pd.Series(timing_result.train_predictions), train_delay_pnls
    )
    test_rf_labels = generate_rf_binary_labels(
        pd.Series(timing_result.test_predictions), test_delay_pnls
    )

    rf_result = train_rf_probability(
        train_features=train_features,
        train_labels=train_rf_labels,
        test_features=test_features,
        test_labels=test_rf_labels,
        n_estimators=200,
        random_state=RANDOM_STATE,
    )

    full_window_probabilities = np.concatenate(
        [rf_result.train_probabilities, rf_result.test_probabilities]
    )
    full_window_multipliers = compute_sizing_multiplier(full_window_probabilities)

    if rf_result.rf_no_signal_warning:
        logger.warning("RF no signal — degrading to uniform sizing")
        full_window_multipliers = np.ones(len(full_window_events), dtype=np.float64)

    # Step 9: Speed gate
    speed_gate_pass, speed_gate_threshold = compute_speed_gate(
        events=full_window_events,
        train_events=train_events,
        quantile=0.10,
    )

    # Step 10: Combined positions
    config = CombinedPositionConfig(base_notional_share=0.80)
    unified_trades = compute_combined_positions(
        events=full_window_events,
        timing_predictions=full_window_predictions,
        sizing_multipliers=full_window_multipliers,
        delay_results=delay_results_full,
        speed_gate_pass=speed_gate_pass,
        config=config,
    )

    # Compute metrics
    cs_gate_on = compute_calendar_sum(unified_trades, gate_filter=True)
    cs_gate_off = compute_calendar_sum(unified_trades, gate_filter=False)
    ws_gate_on = compute_worst_sm(unified_trades, gate_filter=True)

    # Trade count and neg SM
    gate_on_trades = unified_trades[unified_trades["speed_gate_pass"] == True]  # noqa: E712
    active_trades = gate_on_trades[gate_on_trades["timing_prediction"] != "skip"]
    trade_count = len(active_trades)

    # Monthly silo returns
    monthly_silo_returns: dict[str, float] = {}
    neg_sm = 0
    if not gate_on_trades.empty:
        df = gate_on_trades.copy()
        df["year_month"] = pd.to_datetime(df["touch_time"]).dt.to_period("M")
        monthly = df.groupby(["symbol", "year_month"])["weighted_pnl"].sum()
        for (sym, ym), val in monthly.items():
            monthly_silo_returns[f"{sym}_{ym}"] = float(val)
        neg_sm = int((monthly < 0).sum())

    pipeline_seconds = time.time() - pipeline_start

    result = TimeframePipelineResult(
        timeframe=timeframe,
        total_events=len(events),
        train_events=n_train,
        test_events=n_test,
        forward_events=len(forward_events),
        long_count=int((events["side"] == "long").sum()),
        short_count=int((events["side"] == "short").sum()),
        delay_sim_traded=traded_count,
        delay_sim_skipped=len(full_window_events) - traded_count,
        timing_selected_depth=timing_result.selected_depth,
        timing_dt3_loocv_cs=timing_result.dt3_loocv_calendar_sum,
        timing_dt4_loocv_cs=timing_result.dt4_loocv_calendar_sum,
        timing_test_cs=timing_result.test_calendar_sum,
        rf_train_auc=rf_result.train_auc,
        rf_test_auc=rf_result.test_auc,
        rf_no_signal=rf_result.rf_no_signal_warning,
        speed_gate_threshold=speed_gate_threshold,
        speed_gate_pass_rate=float(np.mean(speed_gate_pass)),
        calendar_sum_gate_on=cs_gate_on,
        calendar_sum_gate_off=cs_gate_off,
        worst_sm_gate_on=ws_gate_on,
        trade_count_gate_on=trade_count,
        neg_sm_count=neg_sm,
        monthly_silo_returns=monthly_silo_returns,
        pipeline_seconds=pipeline_seconds,
    )

    logger.info("Pipeline %s done: CS(gate ON)=%.4f, worst_sm=%.4f, trades=%d, time=%.1fs",
                timeframe, cs_gate_on, ws_gate_on, trade_count, pipeline_seconds)

    return result


def generate_comparison_report(
    results: dict[str, TimeframePipelineResult],
    output_dir: Path,
) -> None:
    """生成跨时间框架对比报告。"""

    # JSON summary
    json_data = {}
    for tf, r in results.items():
        json_data[tf] = {
            "total_events": r.total_events,
            "train_events": r.train_events,
            "test_events": r.test_events,
            "forward_events": r.forward_events,
            "long_count": r.long_count,
            "short_count": r.short_count,
            "delay_sim_traded": r.delay_sim_traded,
            "timing_selected_depth": r.timing_selected_depth,
            "timing_dt3_loocv_cs": r.timing_dt3_loocv_cs,
            "timing_dt4_loocv_cs": r.timing_dt4_loocv_cs,
            "timing_test_cs": r.timing_test_cs,
            "rf_train_auc": r.rf_train_auc,
            "rf_test_auc": r.rf_test_auc,
            "rf_no_signal": r.rf_no_signal,
            "speed_gate_threshold": r.speed_gate_threshold,
            "speed_gate_pass_rate": r.speed_gate_pass_rate,
            "calendar_sum_gate_on": r.calendar_sum_gate_on,
            "calendar_sum_gate_off": r.calendar_sum_gate_off,
            "worst_sm_gate_on": r.worst_sm_gate_on,
            "trade_count_gate_on": r.trade_count_gate_on,
            "neg_sm_count": r.neg_sm_count,
            "pipeline_seconds": r.pipeline_seconds,
        }

    json_path = output_dir / "cross_timeframe_comparison.json"
    with open(json_path, "w") as f:
        json.dump(json_data, f, indent=2, default=str)
    logger.info("Written: %s", json_path)

    # Markdown report
    md_lines = [
        "# 多时间框架 Pretouch 事件 Pipeline 对比",
        "",
        f"生成时间: {pd.Timestamp.now().strftime('%Y-%m-%d %H:%M:%S')}",
        "",
        "## 概要",
        "",
        "| 指标 | 30min | 1h | 4h |",
        "|------|-------|----|----|",
    ]

    tfs = ["30min", "1h", "4h"]
    available_tfs = [tf for tf in tfs if tf in results]

    def _row(label: str, key: str, fmt: str = "{}") -> str:
        cells = [label]
        for tf in available_tfs:
            r = results[tf]
            val = getattr(r, key, None)
            if val is None:
                cells.append("-")
            else:
                cells.append(fmt.format(val))
        return "| " + " | ".join(cells) + " |"

    md_lines.append(_row("事件总数", "total_events"))
    md_lines.append(_row("Long / Short", "long_count", "{}") + "  *(long only)*")
    md_lines.append(_row("Delay 可交易", "delay_sim_traded"))
    md_lines.append(_row("Timing depth", "timing_selected_depth", "DT{}"))
    md_lines.append(_row("DT3 LOOCV CS", "timing_dt3_loocv_cs", "{:.4f}"))
    md_lines.append(_row("DT4 LOOCV CS", "timing_dt4_loocv_cs", "{:.4f}"))
    md_lines.append(_row("Test CS", "timing_test_cs", "{:.4f}"))
    md_lines.append(_row("RF train AUC", "rf_train_auc", "{:.4f}"))
    md_lines.append(_row("RF test AUC", "rf_test_auc", "{:.4f}"))
    md_lines.append(_row("RF no signal", "rf_no_signal"))
    md_lines.append(_row("Speed gate threshold", "speed_gate_threshold", "{:.6f}"))
    md_lines.append(_row("Speed gate pass rate", "speed_gate_pass_rate", "{:.1%}"))
    md_lines.append(_row("**Calendar Sum (gate ON)**", "calendar_sum_gate_on", "**{:.4f}**"))
    md_lines.append(_row("Calendar Sum (gate OFF)", "calendar_sum_gate_off", "{:.4f}"))
    md_lines.append(_row("Worst SM (gate ON)", "worst_sm_gate_on", "{:.4f}"))
    md_lines.append(_row("Trade count (gate ON)", "trade_count_gate_on"))
    md_lines.append(_row("Neg SM count", "neg_sm_count"))
    md_lines.append(_row("Pipeline time (s)", "pipeline_seconds", "{:.1f}"))

    md_lines.extend([
        "",
        "## 月度 Silo 明细",
        "",
    ])

    for tf in available_tfs:
        r = results[tf]
        md_lines.append(f"### {tf}")
        md_lines.append("")
        if r.monthly_silo_returns:
            md_lines.append("| Silo | Return |")
            md_lines.append("|------|--------|")
            for silo, ret in sorted(r.monthly_silo_returns.items()):
                sign = "+" if ret >= 0 else ""
                md_lines.append(f"| {silo} | {sign}{ret:.4f} |")
        else:
            md_lines.append("*无交易*")
        md_lines.append("")

    md_lines.extend([
        "## 结论",
        "",
        "- 对比 1h baseline（unified_runner 产出的 ETH-only pipeline）",
        "- 30min 事件池更大但可能噪声更多",
        "- 4h 事件池小但结构信号更强",
        "- 需关注 RF AUC 和 speed gate pass rate 的跨时间框架差异",
        "",
        "## 参数",
        "",
        "```",
        f"exec_params = {json.dumps(EXEC_PARAMS_REFERENCE, indent=2)}",
        f"base_notional_share = 0.80",
        f"speed_gate_quantile = 0.10",
        f"forward_start = 2025-11-01",
        f"random_state = {RANDOM_STATE}",
        "```",
    ])

    md_path = output_dir / "cross_timeframe_comparison.md"
    with open(md_path, "w") as f:
        f.write("\n".join(md_lines))
    logger.info("Written: %s", md_path)


def main() -> None:
    """执行多时间框架 pipeline 并生成对比报告。"""
    import argparse

    parser = argparse.ArgumentParser(description="Multi-timeframe pretouch pipeline runner")
    parser.add_argument("--timeframes", default="30min,1h,4h",
                        help="Comma-separated timeframes to run (default: 30min,1h,4h)")
    parser.add_argument("--symbol", default="ETHUSDT",
                        help="Symbol to process (default: ETHUSDT)")
    parser.add_argument("--forward-start", default="2025-11-01",
                        help="Forward split start date (default: 2025-11-01)")
    args = parser.parse_args()

    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
    )
    np.random.seed(RANDOM_STATE)

    timeframes = [tf.strip() for tf in args.timeframes.split(",")]
    symbol = args.symbol

    logger.info("=" * 70)
    logger.info("Multi-Timeframe Pipeline Runner")
    logger.info("  Symbol: %s", symbol)
    logger.info("  Timeframes: %s", timeframes)
    logger.info("  Forward start: %s", args.forward_start)
    logger.info("=" * 70)

    results: dict[str, TimeframePipelineResult] = {}

    for tf in timeframes:
        if tf not in TIMEFRAME_CONFIGS:
            logger.error("Unknown timeframe: %s (available: %s)", tf, list(TIMEFRAME_CONFIGS.keys()))
            continue
        try:
            result = run_timeframe_pipeline(
                symbol=symbol,
                timeframe=tf,
                forward_start=args.forward_start,
            )
            results[tf] = result

            # Save individual summary
            summary_path = PIPELINE_OUTPUT_DIR / f"pipeline_{tf}_summary.json"
            with open(summary_path, "w") as f:
                json.dump({
                    "timeframe": result.timeframe,
                    "total_events": result.total_events,
                    "calendar_sum_gate_on": result.calendar_sum_gate_on,
                    "worst_sm_gate_on": result.worst_sm_gate_on,
                    "trade_count_gate_on": result.trade_count_gate_on,
                    "neg_sm_count": result.neg_sm_count,
                    "rf_test_auc": result.rf_test_auc,
                    "timing_selected_depth": result.timing_selected_depth,
                    "pipeline_seconds": result.pipeline_seconds,
                    "monthly_silo_returns": result.monthly_silo_returns,
                }, f, indent=2, default=str)
            logger.info("Written: %s", summary_path)

        except Exception as e:
            logger.error("Pipeline failed for %s: %s", tf, e, exc_info=True)

    if results:
        generate_comparison_report(results, PIPELINE_OUTPUT_DIR)

    # Final summary
    logger.info("=" * 70)
    logger.info("Multi-Timeframe Pipeline Complete")
    for tf, r in results.items():
        logger.info("  %s: CS=%.4f, worst_sm=%.4f, trades=%d, neg_sm=%d",
                    tf, r.calendar_sum_gate_on, r.worst_sm_gate_on,
                    r.trade_count_gate_on, r.neg_sm_count)
    logger.info("=" * 70)


if __name__ == "__main__":
    main()
