"""
dynamic_entry_timing_runner — 主运行器

负责：
- 串联完整流程：加载数据 → time split → grid search (train) → evaluate (test)
  → baselines → 对比 → 报告产出
- 对每个 event 执行步进式决策循环（特征提取 → classify → 执行）
- 实现 grid search 分层优化（第一层粗搜 + 第二层精搜）
- 产出所有输出文件到 output/dynamic_timing/ 目录
- 确保确定性：不使用 datetime.now()、未 seed 随机源
"""

from __future__ import annotations

import itertools
import json
from dataclasses import asdict
from pathlib import Path

import numpy as np
import pandas as pd

from .execution_sim import DEFAULT_EXEC_PARAMS, execute_trade
from .feature_engine import extract_step_features
from .pullback_monitor import wait_for_pullback
from .regime_classifier import EntryDecision, TimingParams, classify


def run_dynamic_timing(
    events: pd.DataFrame,
    bars_cache: dict[str, pd.DataFrame],
    timing_params: TimingParams,
    exec_params: dict | None = None,
) -> list[dict]:
    """对每个 event 执行步进式决策循环，返回所有 event 的决策与交易结果。

    决策循环逻辑：
    - 每步提取特征 → classify → 根据决策执行对应动作
    - immediate：以当前 step 结束后 1s bar close 入场 → execute_trade
    - wait_pullback：调用 wait_for_pullback → 若触发则 execute_trade
    - continue_observe：进入下一步
    - skip：记录并跳过
    - 连续 3 步数据缺失 → 强制 skip
    - max_steps 到达仍为 continue_observe → 强制 immediate（保底）
    - 记录完整决策路径 [(step, decision, regime), ...]

    Parameters
    ----------
    events : pd.DataFrame
        V6 gate 筛选后的 events，需含 symbol, touch_time, side, level, atr 等列。
    bars_cache : dict[str, pd.DataFrame]
        按 symbol_YYYYMM key 索引的 1s bar cache。
    timing_params : TimingParams
        决策分类器的阈值参数。
    exec_params : dict | None
        V4 执行模型参数，None 时使用 DEFAULT_EXEC_PARAMS。

    Returns
    -------
    list[dict]
        每个 event 的决策与交易结果记录。
    """
    if exec_params is None:
        exec_params = DEFAULT_EXEC_PARAMS

    results: list[dict] = []

    for _, event in events.iterrows():
        symbol = event["symbol"]
        touch_time = pd.Timestamp(event["touch_time"])
        if touch_time.tzinfo is None:
            touch_time = touch_time.tz_localize("UTC")

        # Get bars for this event's month
        month_key = f"{symbol}_{touch_time.strftime('%Y%m')}"
        second_bars = bars_cache.get(month_key)
        if second_bars is None or second_bars.empty:
            results.append({
                "event_id": event.get("event_id", ""),
                "symbol": symbol,
                "side": event["side"],
                "touch_time": touch_time,
                "entry_decision": "skip",
                "regime": "NoData",
                "decision_path": [],
                "entry_time": None,
                "entry_price": None,
                "entry_delay_seconds": None,
                "trade": None,
            })
            continue

        decision_path: list[tuple[int, str, str]] = []
        consecutive_missing = 0
        trade = None
        final_decision = "skip"
        final_regime = "NoData"
        entry_time = None
        entry_price = None

        for step in range(1, timing_params.max_steps + 1):
            features = extract_step_features(second_bars, event, step)

            if features is None:
                consecutive_missing += 1
                decision_path.append((step, "continue_observe", "DataMissing"))
                if consecutive_missing >= 3:
                    final_decision = "skip"
                    final_regime = "ConsecutiveDataMissing"
                    break
                continue

            consecutive_missing = 0
            result = classify(features, timing_params)
            decision_path.append((step, result.decision.value, result.regime))

            if result.decision == EntryDecision.IMMEDIATE:
                # Entry at step end: touch_time + step * step_interval
                step_end = touch_time + pd.Timedelta(seconds=step * 5)
                entry_mask = second_bars.index >= step_end
                if entry_mask.any():
                    entry_idx = second_bars.index[entry_mask][0]
                    entry_price = float(second_bars.loc[entry_idx, "close"])
                    entry_time = entry_idx
                    trade = execute_trade(
                        second_bars, event, entry_time, entry_price, params=exec_params
                    )
                final_decision = "immediate"
                final_regime = result.regime
                break

            elif result.decision == EntryDecision.WAIT_PULLBACK:
                step_end = touch_time + pd.Timedelta(seconds=step * 5)
                # decision_price = close at step_end
                entry_mask = second_bars.index >= step_end
                if entry_mask.any():
                    decision_idx = second_bars.index[entry_mask][0]
                    decision_price = float(second_bars.loc[decision_idx, "close"])
                    pullback_result = wait_for_pullback(
                        second_bars, decision_idx, decision_price, event, timing_params
                    )
                    if pullback_result.triggered:
                        entry_time = pullback_result.entry_time
                        entry_price = pullback_result.entry_price
                        trade = execute_trade(
                            second_bars,
                            event,
                            entry_time,
                            entry_price,
                            params=exec_params,
                        )
                final_decision = "wait_pullback"
                final_regime = result.regime
                break

            elif result.decision == EntryDecision.SKIP:
                final_decision = "skip"
                final_regime = result.regime
                break

            # CONTINUE_OBSERVE → next step (loop continues)
        else:
            # max_steps reached without terminal decision → 保底 immediate
            step_end = touch_time + pd.Timedelta(
                seconds=timing_params.max_steps * 5
            )
            entry_mask = second_bars.index >= step_end
            if entry_mask.any():
                entry_idx = second_bars.index[entry_mask][0]
                entry_price = float(second_bars.loc[entry_idx, "close"])
                entry_time = entry_idx
                trade = execute_trade(
                    second_bars, event, entry_time, entry_price, params=exec_params
                )
            final_decision = "immediate"
            final_regime = "Default"
            decision_path.append((timing_params.max_steps, "immediate", "Default"))

        # Build result record
        record = {
            "event_id": event.get("event_id", ""),
            "symbol": symbol,
            "side": event["side"],
            "touch_time": touch_time,
            "entry_decision": final_decision,
            "regime": final_regime,
            "decision_path": decision_path,
            "entry_time": entry_time,
            "entry_price": entry_price,
            "entry_delay_seconds": (
                (entry_time - touch_time).total_seconds() if entry_time else None
            ),
            "trade": trade,
        }
        results.append(record)

    return results


# ---------------------------------------------------------------------------
# Baseline 回测
# ---------------------------------------------------------------------------


def run_baseline(
    events: pd.DataFrame,
    bars_cache: dict[str, pd.DataFrame],
    delay_seconds: int,
    exec_params: dict | None = None,
) -> list[dict]:
    """运行固定 delay baseline 回测。

    Args:
        events: V6 gate events
        bars_cache: 1s bar cache by symbol_month
        delay_seconds: 入场延迟秒数 (0=Baseline C, 5=Baseline A, 60=Baseline B)
        exec_params: V4 execution params, defaults to DEFAULT_EXEC_PARAMS

    Returns:
        list of trade result dicts (same structure as run_dynamic_timing results)
    """
    if exec_params is None:
        exec_params = DEFAULT_EXEC_PARAMS

    results: list[dict] = []

    for _, event in events.iterrows():
        symbol = event["symbol"]
        touch_time = pd.Timestamp(event["touch_time"])
        if touch_time.tzinfo is None:
            touch_time = touch_time.tz_localize("UTC")

        month_key = f"{symbol}_{touch_time.strftime('%Y%m')}"
        second_bars = bars_cache.get(month_key)
        if second_bars is None or second_bars.empty:
            results.append({
                "event_id": event.get("event_id", ""),
                "symbol": symbol,
                "side": event["side"],
                "touch_time": touch_time,
                "entry_decision": "baseline",
                "regime": f"D={delay_seconds}s",
                "decision_path": [],
                "entry_time": None,
                "entry_price": None,
                "entry_delay_seconds": None,
                "trade": None,
            })
            continue

        # Find entry bar: first bar at or after touch_time + delay_seconds
        target_time = touch_time + pd.Timedelta(seconds=delay_seconds)
        entry_mask = second_bars.index >= target_time
        if not entry_mask.any():
            results.append({
                "event_id": event.get("event_id", ""),
                "symbol": symbol,
                "side": event["side"],
                "touch_time": touch_time,
                "entry_decision": "baseline",
                "regime": f"D={delay_seconds}s",
                "decision_path": [],
                "entry_time": None,
                "entry_price": None,
                "entry_delay_seconds": None,
                "trade": None,
            })
            continue

        entry_idx = second_bars.index[entry_mask][0]
        entry_price = float(second_bars.loc[entry_idx, "close"])
        entry_time = entry_idx

        trade = execute_trade(
            second_bars, event, entry_time, entry_price, params=exec_params
        )

        results.append({
            "event_id": event.get("event_id", ""),
            "symbol": symbol,
            "side": event["side"],
            "touch_time": touch_time,
            "entry_decision": "baseline",
            "regime": f"D={delay_seconds}s",
            "decision_path": [],
            "entry_time": entry_time,
            "entry_price": entry_price,
            "entry_delay_seconds": (entry_time - touch_time).total_seconds(),
            "trade": trade,
        })

    return results


def run_all_baselines(
    events: pd.DataFrame,
    bars_cache: dict[str, pd.DataFrame],
    exec_params: dict | None = None,
) -> dict[str, list[dict]]:
    """运行所有 3 个 baseline。

    Returns:
        {"baseline_a": [...], "baseline_b": [...], "baseline_c": [...]}
    """
    return {
        "baseline_a": run_baseline(events, bars_cache, delay_seconds=5, exec_params=exec_params),
        "baseline_b": run_baseline(events, bars_cache, delay_seconds=60, exec_params=exec_params),
        "baseline_c": run_baseline(events, bars_cache, delay_seconds=0, exec_params=exec_params),
    }


# ---------------------------------------------------------------------------
# Calendar Sum 计算
# ---------------------------------------------------------------------------


def compute_calendar_sum(results: list[dict]) -> float:
    """计算 silo-based calendar sum (%).

    每个 (symbol, month) 独立从 100k 开始，各 silo return 简单加和。
    """
    # Group trades by (symbol, month)
    silos: dict[str, list[dict]] = {}  # key: symbol_YYYY-MM -> list of trades
    for r in results:
        trade = r.get("trade")
        if trade is None:
            continue
        symbol = r["symbol"]
        entry_time = pd.Timestamp(trade["entry_time"])
        month_key = f"{symbol}_{entry_time.strftime('%Y-%m')}"
        if month_key not in silos:
            silos[month_key] = []
        silos[month_key].append(trade)

    total_return_pct = 0.0
    for silo_key, trades in silos.items():
        balance = 100000.0
        for trade in sorted(trades, key=lambda t: t["entry_time"]):
            pnl = balance * trade["notional_share"] * trade["realistic_pnl_pct"]
            balance += pnl
        silo_return = (balance - 100000.0) / 100000.0 * 100.0
        total_return_pct += silo_return

    return total_return_pct


# ---------------------------------------------------------------------------
# Bootstrap 置信区间估计
# ---------------------------------------------------------------------------


def compute_bootstrap_ci(
    results: list[dict],
    n_bootstrap: int = 1000,
    seed: int = 42,
) -> dict:
    """对 BTC 和 ETH events 独立执行 bootstrap 重采样，计算置信区间。

    对每个 symbol 独立执行 bootstrap 1000 次重采样，计算 calendar_sum 的
    5th/95th percentile 置信区间。

    Parameters
    ----------
    results : list[dict]
        run_dynamic_timing 或 run_baseline 的输出结果列表。
    n_bootstrap : int
        Bootstrap 重采样次数，默认 1000。
    seed : int
        随机种子，确保确定性（Requirement 6.6）。

    Returns
    -------
    dict with:
    - btc_ci: {"p5": float, "p95": float, "mean": float}
    - eth_ci: {"p5": float, "p95": float, "mean": float}
    - combined_ci: {"p5": float, "p95": float, "mean": float}
    - small_sample_warning: True
    """
    rng = np.random.default_rng(seed)

    def _bootstrap_symbol(symbol_results: list[dict]) -> dict:
        if not symbol_results:
            return {"p5": 0.0, "p95": 0.0, "mean": 0.0}

        n = len(symbol_results)
        bootstrap_sums: list[float] = []

        for _ in range(n_bootstrap):
            indices = rng.integers(0, n, size=n)
            sample = [symbol_results[i] for i in indices]
            cal_sum = compute_calendar_sum(sample)
            bootstrap_sums.append(cal_sum)

        bootstrap_sums_arr = np.array(bootstrap_sums)
        return {
            "p5": float(np.percentile(bootstrap_sums_arr, 5)),
            "p95": float(np.percentile(bootstrap_sums_arr, 95)),
            "mean": float(np.mean(bootstrap_sums_arr)),
        }

    btc_results = [r for r in results if r["symbol"] == "BTCUSDT"]
    eth_results = [r for r in results if r["symbol"] == "ETHUSDT"]

    return {
        "btc_ci": _bootstrap_symbol(btc_results),
        "eth_ci": _bootstrap_symbol(eth_results),
        "combined_ci": _bootstrap_symbol(results),
        "small_sample_warning": True,
    }


# ---------------------------------------------------------------------------
# Grid Search 第一层粗搜
# ---------------------------------------------------------------------------


def run_grid_search_layer1(
    train_events: pd.DataFrame,
    bars_cache: dict[str, pd.DataFrame],
    exec_params: dict | None = None,
    output_dir: Path | None = None,
) -> tuple[TimingParams, pd.DataFrame]:
    """第一层粗搜：搜索核心参数。

    搜索空间：
    - max_steps: {2, 4, 6, 12}
    - strong_momentum_threshold: {0.10, 0.15, 0.20, 0.30}
    - extension_threshold: {0.10, 0.15, 0.20, 0.30}
    - moderate_momentum_threshold: {0.05, 0.10}

    共 128 组合。其余参数使用 TimingParams 默认值。
    优化目标：calendar_sum_pct

    Parameters
    ----------
    train_events : pd.DataFrame
        训练集 events（time-split 前 60%）。
    bars_cache : dict[str, pd.DataFrame]
        1s bar cache by symbol_month key。
    exec_params : dict | None
        V4 执行模型参数，None 时使用 DEFAULT_EXEC_PARAMS。
    output_dir : Path | None
        输出目录，若提供则保存 grid_search_layer1.csv。

    Returns
    -------
    tuple[TimingParams, pd.DataFrame]
        (best_params, results_df) - 最优参数和所有组合的结果 DataFrame。
    """
    max_steps_candidates = [2, 4, 6, 12]
    strong_momentum_candidates = [0.10, 0.15, 0.20, 0.30]
    extension_candidates = [0.10, 0.15, 0.20, 0.30]
    moderate_momentum_candidates = [0.05, 0.10]

    records: list[dict] = []
    best_score = float("-inf")
    best_params = TimingParams()

    for max_steps, strong_mom, ext_thresh, mod_mom in itertools.product(
        max_steps_candidates,
        strong_momentum_candidates,
        extension_candidates,
        moderate_momentum_candidates,
    ):
        params = TimingParams(
            max_steps=max_steps,
            strong_momentum_threshold=strong_mom,
            extension_threshold=ext_thresh,
            moderate_momentum_threshold=mod_mom,
        )

        results = run_dynamic_timing(train_events, bars_cache, params, exec_params)
        cal_sum = compute_calendar_sum(results)

        # Count trades and skips
        trade_count = sum(1 for r in results if r["trade"] is not None)
        skip_count = sum(1 for r in results if r["entry_decision"] == "skip")

        records.append({
            "max_steps": max_steps,
            "strong_momentum_threshold": strong_mom,
            "extension_threshold": ext_thresh,
            "moderate_momentum_threshold": mod_mom,
            "calendar_sum_pct": cal_sum,
            "trade_count": trade_count,
            "skip_count": skip_count,
        })

        if cal_sum > best_score:
            best_score = cal_sum
            best_params = params

    results_df = pd.DataFrame(records)

    # Save to CSV if output_dir provided
    if output_dir is not None:
        output_dir.mkdir(parents=True, exist_ok=True)
        results_df.to_csv(output_dir / "grid_search_layer1.csv", index=False)

    return best_params, results_df


# ---------------------------------------------------------------------------
# Grid Search 第二层精搜
# ---------------------------------------------------------------------------


def run_grid_search_layer2(
    train_events: pd.DataFrame,
    bars_cache: dict[str, pd.DataFrame],
    layer1_best: TimingParams,
    exec_params: dict | None = None,
    output_dir: Path | None = None,
) -> tuple[TimingParams, pd.DataFrame]:
    """第二层精搜：固定第一层最优参数，搜索辅助参数。

    搜索空间：
    - strong_flow_threshold: {0.55, 0.58, 0.60, 0.65}
    - weak_flow_threshold: {0.45, 0.48, 0.50}
    - fading_threshold: {0.01, 0.02, 0.03}
    - min_steps_for_skip: {2, 3, 4}
    - pullback_target_atr: {0.02, 0.05, 0.10}
    - decision_window_seconds: {60, 120, 300}

    共 972 组合。
    优化目标：calendar_sum_pct

    Parameters
    ----------
    train_events : pd.DataFrame
        训练集 events。
    bars_cache : dict[str, pd.DataFrame]
        1s bar cache。
    layer1_best : TimingParams
        第一层最优参数（max_steps, strong_momentum_threshold, extension_threshold,
        moderate_momentum_threshold 已固定）。
    exec_params : dict | None
        V4 执行模型参数。
    output_dir : Path | None
        输出目录，若提供则保存 grid_search_layer2.csv 和 dynamic_timing_params.json。

    Returns
    -------
    tuple[TimingParams, pd.DataFrame]
        (best_params, results_df) - 最终最优参数和所有组合的结果 DataFrame。
    """
    if exec_params is None:
        exec_params = DEFAULT_EXEC_PARAMS

    strong_flow_candidates = [0.55, 0.58, 0.60, 0.65]
    weak_flow_candidates = [0.45, 0.48, 0.50]
    fading_candidates = [0.01, 0.02, 0.03]
    min_steps_for_skip_candidates = [2, 3, 4]
    pullback_target_candidates = [0.02, 0.05, 0.10]
    decision_window_candidates = [60, 120, 300]

    records: list[dict] = []
    best_score = float("-inf")
    best_params = layer1_best  # start with layer1 best

    for sf, wf, fad, mss, pt, dw in itertools.product(
        strong_flow_candidates,
        weak_flow_candidates,
        fading_candidates,
        min_steps_for_skip_candidates,
        pullback_target_candidates,
        decision_window_candidates,
    ):
        params = TimingParams(
            max_steps=layer1_best.max_steps,
            strong_momentum_threshold=layer1_best.strong_momentum_threshold,
            extension_threshold=layer1_best.extension_threshold,
            moderate_momentum_threshold=layer1_best.moderate_momentum_threshold,
            strong_flow_threshold=sf,
            weak_flow_threshold=wf,
            fading_threshold=fad,
            min_steps_for_skip=mss,
            pullback_target_atr=pt,
            decision_window_seconds=dw,
        )

        results = run_dynamic_timing(train_events, bars_cache, params, exec_params)
        cal_sum = compute_calendar_sum(results)
        trade_count = sum(1 for r in results if r["trade"] is not None)
        skip_count = sum(1 for r in results if r["entry_decision"] == "skip")

        records.append({
            "strong_flow_threshold": sf,
            "weak_flow_threshold": wf,
            "fading_threshold": fad,
            "min_steps_for_skip": mss,
            "pullback_target_atr": pt,
            "decision_window_seconds": dw,
            "calendar_sum_pct": cal_sum,
            "trade_count": trade_count,
            "skip_count": skip_count,
        })

        if cal_sum > best_score:
            best_score = cal_sum
            best_params = params

    results_df = pd.DataFrame(records)

    if output_dir is not None:
        output_dir.mkdir(parents=True, exist_ok=True)
        results_df.to_csv(output_dir / "grid_search_layer2.csv", index=False)
        # Save best params as JSON
        params_dict = asdict(best_params)
        with open(output_dir / "dynamic_timing_params.json", "w") as f:
            json.dump(params_dict, f, indent=2)

    return best_params, results_df


# ---------------------------------------------------------------------------
# 参数敏感性分析
# ---------------------------------------------------------------------------


def run_sensitivity_analysis(
    train_events: pd.DataFrame,
    bars_cache: dict[str, pd.DataFrame],
    best_params: TimingParams,
    exec_params: dict | None = None,
    output_dir: Path | None = None,
) -> dict:
    """对每个参数轴进行敏感性分析。

    固定其他参数为最优值，扫描该轴全部候选值，输出 calendar_sum 随参数变化的数据。

    Parameters
    ----------
    train_events : pd.DataFrame
        训练集 events。
    bars_cache : dict[str, pd.DataFrame]
        1s bar cache by symbol_month key。
    best_params : TimingParams
        Grid search 产出的最优参数。
    exec_params : dict | None
        V4 执行模型参数，None 时使用 DEFAULT_EXEC_PARAMS。
    output_dir : Path | None
        输出目录，若提供则保存 sensitivity/ 子目录下的 CSV 文件。

    Returns
    -------
    dict with:
    - sensitivities: dict[param_name, pd.DataFrame] - each has columns [param_value, calendar_sum_pct]
    - high_sensitivity_params: list[str] - params where calendar_sum varies > ±3%
    """
    if exec_params is None:
        exec_params = DEFAULT_EXEC_PARAMS

    # Define candidate values for each parameter axis
    param_candidates: dict[str, list] = {
        "max_steps": [2, 4, 6, 12],
        "strong_momentum_threshold": [0.10, 0.15, 0.20, 0.30],
        "strong_flow_threshold": [0.55, 0.58, 0.60, 0.65],
        "moderate_momentum_threshold": [0.05, 0.08, 0.10, 0.15],
        "extension_threshold": [0.10, 0.15, 0.20, 0.30],
        "weak_flow_threshold": [0.45, 0.48, 0.50],
        "fading_threshold": [0.01, 0.02, 0.03],
        "min_steps_for_skip": [2, 3, 4],
        "pullback_target_atr": [0.02, 0.05, 0.10],
        "decision_window_seconds": [60, 120, 300],
    }

    sensitivities: dict[str, pd.DataFrame] = {}
    high_sensitivity_params: list[str] = []

    for param_name, candidates in param_candidates.items():
        records: list[dict] = []
        for value in candidates:
            # Create params with this one axis varied, others fixed at best
            params_dict = asdict(best_params)
            params_dict[param_name] = value
            params = TimingParams(**params_dict)

            results = run_dynamic_timing(train_events, bars_cache, params, exec_params)
            cal_sum = compute_calendar_sum(results)
            records.append({"param_value": value, "calendar_sum_pct": cal_sum})

        df = pd.DataFrame(records)
        sensitivities[param_name] = df

        # Check if high sensitivity (range > 3% absolute)
        cal_range = df["calendar_sum_pct"].max() - df["calendar_sum_pct"].min()
        if cal_range > 3.0:
            high_sensitivity_params.append(param_name)

        # Save to CSV if output_dir provided
        if output_dir is not None:
            sens_dir = output_dir / "sensitivity"
            sens_dir.mkdir(parents=True, exist_ok=True)
            df.to_csv(sens_dir / f"sensitivity_{param_name}.csv", index=False)

    return {
        "sensitivities": sensitivities,
        "high_sensitivity_params": high_sensitivity_params,
    }


# ---------------------------------------------------------------------------
# Test Set 评估与对比指标计算
# ---------------------------------------------------------------------------


def evaluate_on_test_set(
    test_events: pd.DataFrame,
    bars_cache: dict[str, pd.DataFrame],
    best_params: TimingParams,
    train_calendar_sum: float,
    exec_params: dict | None = None,
) -> dict:
    """在 test set 上评估最优参数，计算对比指标。

    用最优参数在 test set 上运行 dynamic timing 和所有 baselines，
    计算对比指标、overfitting_flag、分 symbol 独立验证。

    Parameters
    ----------
    test_events : pd.DataFrame
        测试集 events（time-split 后 40%）。
    bars_cache : dict[str, pd.DataFrame]
        1s bar cache by symbol_month key。
    best_params : TimingParams
        grid search 选出的最优参数。
    train_calendar_sum : float
        训练集上最优参数的 calendar_sum_pct，用于计算 overfitting_flag。
    exec_params : dict | None
        V4 执行模型参数，None 时使用 DEFAULT_EXEC_PARAMS。

    Returns
    -------
    dict
        包含以下 key：
        - dynamic_results: list[dict] - dynamic timing 原始结果
        - baselines: dict[str, list[dict]] - baseline 原始结果
        - metrics: dict - 对比指标 (dynamic / baseline_a / baseline_b / baseline_c)
        - overfitting_flag: bool - train→test calendar_sum 下降超 50%
        - train_calendar_sum: float - 训练集 calendar_sum
        - test_calendar_sum: float - 测试集 dynamic calendar_sum
        - symbol_metrics: dict - 分 symbol 指标（含 negative_flag）
    """
    if exec_params is None:
        exec_params = DEFAULT_EXEC_PARAMS

    # Run dynamic timing on test set
    dynamic_results = run_dynamic_timing(test_events, bars_cache, best_params, exec_params)

    # Run baselines on test set
    baselines = run_all_baselines(test_events, bars_cache, exec_params)

    # Helper to compute metrics from results
    def _compute_metrics(results: list[dict]) -> dict:
        """从 results 列表计算对比指标。"""
        trades = [r["trade"] for r in results if r["trade"] is not None]
        total_events = len(results)
        trade_count = len(trades)

        if trade_count == 0:
            return {
                "calendar_sum_pct": 0.0,
                "trade_count": 0,
                "win_rate": 0.0,
                "avg_win_pct": 0.0,
                "avg_loss_pct": 0.0,
                "payoff_ratio": 0.0,
                "skip_rate": 1.0,
                "pullback_fill_rate": 0.0,
                "per_trade_quality_bps": 0.0,
            }

        wins = [t for t in trades if t["realistic_pnl_pct"] > 0]
        losses = [t for t in trades if t["realistic_pnl_pct"] <= 0]

        win_rate = len(wins) / trade_count if trade_count > 0 else 0.0
        avg_win_pct = (
            sum(t["realistic_pnl_pct"] for t in wins) / len(wins) * 100
            if wins
            else 0.0
        )
        avg_loss_pct = (
            sum(t["realistic_pnl_pct"] for t in losses) / len(losses) * 100
            if losses
            else 0.0
        )
        payoff_ratio = (
            abs(avg_win_pct / avg_loss_pct) if avg_loss_pct != 0 else float("inf")
        )

        skip_count = sum(1 for r in results if r["entry_decision"] == "skip")
        skip_rate = skip_count / total_events if total_events > 0 else 0.0

        pullback_events = [r for r in results if r["entry_decision"] == "wait_pullback"]
        pullback_filled = [r for r in pullback_events if r["trade"] is not None]
        pullback_fill_rate = (
            len(pullback_filled) / len(pullback_events) if pullback_events else 0.0
        )

        # per_trade_quality_bps = avg realistic_pnl_pct * 10000
        per_trade_quality_bps = (
            sum(t["realistic_pnl_pct"] for t in trades) / trade_count * 10000
        )

        cal_sum = compute_calendar_sum(results)

        return {
            "calendar_sum_pct": cal_sum,
            "trade_count": trade_count,
            "win_rate": win_rate,
            "avg_win_pct": avg_win_pct,
            "avg_loss_pct": avg_loss_pct,
            "payoff_ratio": payoff_ratio,
            "skip_rate": skip_rate,
            "pullback_fill_rate": pullback_fill_rate,
            "per_trade_quality_bps": per_trade_quality_bps,
        }

    dynamic_metrics = _compute_metrics(dynamic_results)
    baseline_a_metrics = _compute_metrics(baselines["baseline_a"])
    baseline_b_metrics = _compute_metrics(baselines["baseline_b"])
    baseline_c_metrics = _compute_metrics(baselines["baseline_c"])

    # Overfitting flag: train→test calendar_sum 下降超 50%
    test_cal_sum = dynamic_metrics["calendar_sum_pct"]
    if train_calendar_sum > 0:
        drop_pct = (train_calendar_sum - test_cal_sum) / train_calendar_sum * 100
        overfitting_flag = drop_pct > 50
    else:
        # train_calendar_sum <= 0 时无法判断过拟合（训练集本身为负）
        overfitting_flag = False

    # Per-symbol metrics: 分 symbol 独立验证
    symbol_metrics: dict[str, dict] = {}
    for symbol in test_events["symbol"].unique():
        sym_results = [r for r in dynamic_results if r["symbol"] == symbol]
        sym_cal_sum = compute_calendar_sum(sym_results)
        sym_trades = [r["trade"] for r in sym_results if r["trade"] is not None]
        sym_wins = [t for t in sym_trades if t["realistic_pnl_pct"] > 0]
        sym_win_rate = len(sym_wins) / len(sym_trades) if sym_trades else 0.0

        symbol_metrics[symbol] = {
            "calendar_sum_pct": sym_cal_sum,
            "win_rate": sym_win_rate,
            "trade_count": len(sym_trades),
            "negative_flag": sym_cal_sum < 0,
        }

    return {
        "dynamic_results": dynamic_results,
        "baselines": baselines,
        "metrics": {
            "dynamic": dynamic_metrics,
            "baseline_a": baseline_a_metrics,
            "baseline_b": baseline_b_metrics,
            "baseline_c": baseline_c_metrics,
        },
        "overfitting_flag": overfitting_flag,
        "train_calendar_sum": train_calendar_sum,
        "test_calendar_sum": test_cal_sum,
        "symbol_metrics": symbol_metrics,
    }


# ---------------------------------------------------------------------------
# Regime 稳定性分析
# ---------------------------------------------------------------------------


def compute_regime_stability(
    train_results: list[dict],
    test_results: list[dict],
) -> dict:
    """对比 train/test 中各 regime 分布比例，产出 regime 分布统计。

    分析 regime 在 train set 和 test set 之间的分布是否稳定，
    并产出每个 regime 的详细统计（event 数量、win rate、avg pnl、calendar sum 贡献）。

    Parameters
    ----------
    train_results : list[dict]
        训练集上 run_dynamic_timing 的输出结果。
    test_results : list[dict]
        测试集上 run_dynamic_timing 的输出结果。

    Returns
    -------
    dict with:
    - train_distribution: dict[regime, float] - proportion in train
    - test_distribution: dict[regime, float] - proportion in test
    - regime_distribution_shift: bool - True if any regime differs > 20 percentage points
    - shifted_regimes: list[str] - regimes with > 20pp shift
    - regime_stats: dict[regime, {count, trade_count, win_rate, avg_pnl_pct, calendar_contribution_pct}]
    """

    def _regime_distribution(results: list[dict]) -> dict[str, float]:
        """Compute proportion of each regime in results."""
        if not results:
            return {}
        regime_counts: dict[str, int] = {}
        for r in results:
            regime = r.get("regime", "Unknown")
            regime_counts[regime] = regime_counts.get(regime, 0) + 1
        total = len(results)
        return {k: v / total for k, v in regime_counts.items()}

    def _regime_stats(results: list[dict]) -> dict[str, dict]:
        """Compute per-regime statistics."""
        regime_groups: dict[str, list[dict]] = {}
        for r in results:
            regime = r.get("regime", "Unknown")
            if regime not in regime_groups:
                regime_groups[regime] = []
            regime_groups[regime].append(r)

        stats: dict[str, dict] = {}
        for regime, group in regime_groups.items():
            trades = [r["trade"] for r in group if r.get("trade") is not None]
            count = len(group)

            if trades:
                wins = [t for t in trades if t["realistic_pnl_pct"] > 0]
                win_rate = len(wins) / len(trades)
                avg_pnl_pct = (
                    sum(t["realistic_pnl_pct"] for t in trades) / len(trades) * 100
                )
                # Calendar contribution: compute_calendar_sum for this regime's results
                cal_contribution = compute_calendar_sum(group)
            else:
                win_rate = 0.0
                avg_pnl_pct = 0.0
                cal_contribution = 0.0

            stats[regime] = {
                "count": count,
                "trade_count": len(trades),
                "win_rate": win_rate,
                "avg_pnl_pct": avg_pnl_pct,
                "calendar_contribution_pct": cal_contribution,
            }

        return stats

    train_dist = _regime_distribution(train_results)
    test_dist = _regime_distribution(test_results)

    # Check for distribution shift > 20 percentage points
    all_regimes = set(list(train_dist.keys()) + list(test_dist.keys()))
    shifted_regimes: list[str] = []
    for regime in all_regimes:
        train_pct = train_dist.get(regime, 0.0) * 100
        test_pct = test_dist.get(regime, 0.0) * 100
        if abs(train_pct - test_pct) > 20:
            shifted_regimes.append(regime)

    # Combine train + test for overall regime stats
    all_results = train_results + test_results

    return {
        "train_distribution": train_dist,
        "test_distribution": test_dist,
        "regime_distribution_shift": len(shifted_regimes) > 0,
        "shifted_regimes": shifted_regimes,
        "regime_stats": _regime_stats(all_results),
    }


# ---------------------------------------------------------------------------
# 输出文件产出
# ---------------------------------------------------------------------------


def generate_output_files(
    dynamic_results: list[dict],
    baseline_a_results: list[dict],
    evaluation: dict,
    best_params: TimingParams,
    bootstrap_ci: dict,
    regime_stability: dict,
    sensitivity: dict,
    output_dir: Path,
) -> dict:
    """产出所有输出文件。

    Files generated:
    - dynamic_timing_attribution.csv
    - dynamic_timing_trades.csv
    - dynamic_timing_summary.json

    Parameters
    ----------
    dynamic_results : list[dict]
        Dynamic timing 在 test set 上的完整结果。
    baseline_a_results : list[dict]
        Baseline A (D=5s) 在 test set 上的完整结果。
    evaluation : dict
        evaluate_on_test_set() 的返回值。
    best_params : TimingParams
        Grid search 产出的最优参数。
    bootstrap_ci : dict
        Bootstrap 置信区间估计结果。
    regime_stability : dict
        compute_regime_stability() 的返回值。
    sensitivity : dict
        run_sensitivity_analysis() 的返回值。
    output_dir : Path
        输出目录。

    Returns
    -------
    dict with:
    - data_gap_events: list[str] - event_ids with > 10s consecutive data gaps
    - pullback_benefit_marginal: bool - True if median pullback improvement < 5 bps
    """
    output_dir.mkdir(parents=True, exist_ok=True)

    # --- Attribution CSV ---
    # Match dynamic results with baseline_a results by event_id
    baseline_a_map = {r["event_id"]: r for r in baseline_a_results}

    attribution_rows = []
    for r in dynamic_results:
        event_id = r["event_id"]
        baseline_a = baseline_a_map.get(event_id, {})

        # Dynamic PnL
        pnl_pct = (
            r["trade"]["realistic_pnl_pct"] * 100 if r["trade"] else None
        )

        # Baseline A PnL
        ba_trade = baseline_a.get("trade")
        baseline_a_pnl_pct = (
            ba_trade["realistic_pnl_pct"] * 100 if ba_trade else None
        )

        # Delta
        delta_pnl_pct = None
        if pnl_pct is not None and baseline_a_pnl_pct is not None:
            delta_pnl_pct = pnl_pct - baseline_a_pnl_pct

        attribution_rows.append({
            "event_id": event_id,
            "symbol": r["symbol"],
            "touch_time": r["touch_time"],
            "entry_decision": r["entry_decision"],
            "regime": r["regime"],
            "entry_delay_actual_seconds": r["entry_delay_seconds"],
            "entry_price": r["entry_price"],
            "baseline_a_entry_price": baseline_a.get("entry_price"),
            "pnl_pct": pnl_pct,
            "baseline_a_pnl_pct": baseline_a_pnl_pct,
            "delta_pnl_pct": delta_pnl_pct,
        })

    attribution_df = pd.DataFrame(attribution_rows)
    attribution_df.to_csv(
        output_dir / "dynamic_timing_attribution.csv", index=False
    )

    # --- Trade Ledger CSV ---
    trade_rows = []
    for r in dynamic_results:
        if r["trade"] is None:
            continue
        trade = r["trade"]
        row = {
            "event_id": r["event_id"],
            "symbol": r["symbol"],
            "side": r["side"],
            "touch_time": r["touch_time"],
            "entry_decision": r["entry_decision"],
            "regime": r["regime"],
            "entry_time": trade["entry_time"],
            "exit_time": trade["exit_time"],
            "entry_p": trade["entry_p"],
            "exit_p": trade["exit_p"],
            "pnl_pct": trade["pnl_pct"],
            "realistic_pnl_pct": trade["realistic_pnl_pct"],
            "exit_reason": trade["exit_reason"],
            "mfe_r": trade["mfe_r"],
            "mae_r": trade["mae_r"],
            "hold_seconds": trade["hold_seconds"],
            "notional_share": trade["notional_share"],
            "decision_path": str(r["decision_path"]),
        }
        trade_rows.append(row)

    trades_df = pd.DataFrame(trade_rows)
    trades_df.to_csv(output_dir / "dynamic_timing_trades.csv", index=False)

    # --- Data gap events ---
    # Mark events where decision_path has consecutive DataMissing entries
    # spanning > 10s. Each step is 5s, so > 2 consecutive missing = > 10s gap.
    # We use >= 3 consecutive steps (3 * 5s = 15s > 10s threshold).
    data_gap_events: list[str] = []
    for r in dynamic_results:
        path = r.get("decision_path", [])
        consecutive_missing = 0
        max_consecutive = 0
        for _step, _decision, regime in path:
            if regime == "DataMissing":
                consecutive_missing += 1
                max_consecutive = max(max_consecutive, consecutive_missing)
            else:
                consecutive_missing = 0
        if max_consecutive >= 3:  # 3 steps * 5s = 15s > 10s
            data_gap_events.append(r["event_id"])

    # --- Pullback benefit marginal ---
    # Check if median pullback price improvement < 5 bps
    # Collect price_improvement_bps from wait_pullback events that triggered
    # We estimate from entry_price vs baseline_a entry_price for pullback events
    pullback_improvements: list[float] = []
    for r in dynamic_results:
        if r["entry_decision"] == "wait_pullback" and r["trade"] is not None:
            ba = baseline_a_map.get(r["event_id"], {})
            ba_entry = ba.get("entry_price")
            if ba_entry is not None and r["entry_price"] is not None:
                # Improvement = how much better our entry is vs baseline A
                # For long: lower entry is better → (ba_entry - entry) / ba_entry * 10000
                # For short: higher entry is better → (entry - ba_entry) / ba_entry * 10000
                if r["side"] == "long":
                    improvement_bps = (
                        (ba_entry - r["entry_price"]) / ba_entry * 10000
                    )
                else:
                    improvement_bps = (
                        (r["entry_price"] - ba_entry) / ba_entry * 10000
                    )
                pullback_improvements.append(improvement_bps)

    pullback_benefit_marginal = len(pullback_improvements) == 0 or (
        len(pullback_improvements) > 0
        and float(np.median(pullback_improvements)) < 5.0
    )

    # --- Summary JSON ---
    summary = {
        "metrics": evaluation["metrics"],
        "overfitting_flag": evaluation["overfitting_flag"],
        "train_calendar_sum": evaluation["train_calendar_sum"],
        "test_calendar_sum": evaluation["test_calendar_sum"],
        "symbol_metrics": evaluation["symbol_metrics"],
        "best_params": asdict(best_params),
        "bootstrap_ci": bootstrap_ci,
        "regime_stability": {
            "regime_distribution_shift": regime_stability[
                "regime_distribution_shift"
            ],
            "shifted_regimes": regime_stability["shifted_regimes"],
        },
        "high_sensitivity_params": sensitivity.get(
            "high_sensitivity_params", []
        ),
        "data_gap_events": data_gap_events,
        "pullback_benefit_marginal": pullback_benefit_marginal,
    }

    with open(output_dir / "dynamic_timing_summary.json", "w") as f:
        json.dump(summary, f, indent=2, default=str)

    return {
        "data_gap_events": data_gap_events,
        "pullback_benefit_marginal": pullback_benefit_marginal,
    }


# ---------------------------------------------------------------------------
# 主报告生成
# ---------------------------------------------------------------------------


def generate_report(
    evaluation: dict,
    best_params: TimingParams,
    bootstrap_ci: dict,
    regime_stability: dict,
    sensitivity: dict,
    output_files: dict,
    output_dir: Path,
) -> str:
    """生成中文主报告 dynamic_timing_report.md。

    Returns the report content as a string.
    """
    # --- 提取关键数据 ---
    metrics = evaluation["metrics"]
    dynamic_cal = metrics["dynamic"]["calendar_sum_pct"]
    baseline_a_cal = metrics["baseline_a"]["calendar_sum_pct"]
    baseline_b_cal = metrics["baseline_b"]["calendar_sum_pct"]
    baseline_c_cal = metrics["baseline_c"]["calendar_sum_pct"]
    improvement = dynamic_cal - baseline_a_cal

    overfitting_flag = evaluation["overfitting_flag"]
    high_sensitivity_params = sensitivity.get("high_sensitivity_params", [])
    has_high_sensitivity = len(high_sensitivity_params) > 0

    # --- Go/No-Go 判定 ---
    # Requirement 7.2:
    # Go: dynamic > baseline_a + 1% 且无 overfitting
    # Conditional Go: dynamic > baseline_a 但改善 < 1%，或存在 high_sensitivity_param
    # No-Go: dynamic <= baseline_a 或 overfitting_flag=true
    if overfitting_flag or improvement <= 0:
        go_decision = "No-Go"
        go_label = "❌ No-Go（不采用，回退固定 delay）"
    elif improvement > 1.0 and not has_high_sensitivity:
        go_decision = "Go"
        go_label = "✅ Go（推荐采用动态 timing）"
    else:
        # improvement > 0 but <= 1.0, or has high_sensitivity
        go_decision = "Conditional Go"
        go_label = "⚠️ Conditional Go（有条件采用）"

    # --- Flags ---
    dynamic_timing_not_superior = improvement <= 0
    pullback_benefit_marginal = output_files.get("pullback_benefit_marginal", False)
    small_sample_warning = bootstrap_ci.get("small_sample_warning", True)

    # --- 构建报告 ---
    lines: list[str] = []

    # 标题
    lines.append("# Dynamic Entry Timing 实验报告")
    lines.append("")

    # Flags 汇总
    lines.append("> **判定结果**: " + go_label)
    lines.append(">")
    if dynamic_timing_not_superior:
        lines.append("> ⚠️ `dynamic_timing_not_superior=true`：动态模型未超越 Baseline A")
        lines.append(">")
    if overfitting_flag:
        lines.append("> ⚠️ `overfitting_flag=true`：存在过拟合风险")
        lines.append(">")
    if pullback_benefit_marginal:
        lines.append("> ⚠️ `pullback_benefit_marginal=true`：回调入场收益边际不足")
        lines.append(">")
    if small_sample_warning:
        lines.append("> ⚠️ `small_sample_warning=true`：116 events 样本量有限，结论需谨慎")
        lines.append(">")
    lines.append("")

    # --- 1. 实验设计 ---
    lines.append("## 1. 实验设计")
    lines.append("")
    lines.append("- **目标**：替代固定 delay（D=5s）入场，通过实时 tick 特征动态决定入场时机")
    lines.append("- **数据**：V6 gate 筛选后 116 unique events（candidate_001）")
    lines.append("- **验证方式**：time-split 60/40（train 70 events / test 46 events）")
    lines.append("- **执行模型**：V4 execution model（initial_stop_atr=0.45, breakeven_at_r=0.8, trail_start_r=0.9, trail_buffer_atr=0.05, max_hold_hours=4）")
    lines.append("- **Cost model**：slippage=2bps/side, entry_fee=2bps, exit_fee=4bps")
    lines.append(f"- **最优参数**（train set grid search）：")
    lines.append(f"  - max_steps={best_params.max_steps}")
    lines.append(f"  - strong_momentum_threshold={best_params.strong_momentum_threshold}")
    lines.append(f"  - strong_flow_threshold={best_params.strong_flow_threshold}")
    lines.append(f"  - moderate_momentum_threshold={best_params.moderate_momentum_threshold}")
    lines.append(f"  - extension_threshold={best_params.extension_threshold}")
    lines.append(f"  - weak_flow_threshold={best_params.weak_flow_threshold}")
    lines.append(f"  - fading_threshold={best_params.fading_threshold}")
    lines.append(f"  - min_steps_for_skip={best_params.min_steps_for_skip}")
    lines.append(f"  - pullback_target_atr={best_params.pullback_target_atr}")
    lines.append(f"  - decision_window_seconds={best_params.decision_window_seconds}")
    lines.append("")

    # --- 2. 结果对比 ---
    lines.append("## 2. 结果对比")
    lines.append("")
    lines.append("| 指标 | Dynamic | Baseline A (D=5s) | Baseline B (D=60s) | Baseline C (D=0s) |")
    lines.append("|------|---------|-------------------|--------------------|--------------------|")

    metric_labels = [
        ("calendar_sum_pct", "Calendar Sum (%)"),
        ("trade_count", "Trade Count"),
        ("win_rate", "Win Rate"),
        ("avg_win_pct", "Avg Win (%)"),
        ("avg_loss_pct", "Avg Loss (%)"),
        ("payoff_ratio", "Payoff Ratio"),
        ("skip_rate", "Skip Rate"),
        ("pullback_fill_rate", "Pullback Fill Rate"),
        ("per_trade_quality_bps", "Per-Trade Quality (bps)"),
    ]

    def _fmt_val(val) -> str:
        """Format a metric value, handling inf/nan gracefully."""
        if isinstance(val, float):
            if val == float("inf") or val == float("-inf"):
                return "∞" if val > 0 else "-∞"
            if val != val:  # NaN check
                return "N/A"
            return f"{val:.2f}"
        return str(val)

    for key, label in metric_labels:
        d_val = metrics["dynamic"].get(key, 0)
        a_val = metrics["baseline_a"].get(key, 0)
        b_val = metrics["baseline_b"].get(key, 0)
        c_val = metrics["baseline_c"].get(key, 0)

        lines.append(
            f"| {label} | {_fmt_val(d_val)} | {_fmt_val(a_val)} | {_fmt_val(b_val)} | {_fmt_val(c_val)} |"
        )

    lines.append("")
    lines.append(f"**Dynamic vs Baseline A 改善**: {improvement:+.2f}%（绝对值）")
    lines.append("")

    if dynamic_timing_not_superior:
        lines.append("> ❌ **dynamic_timing_not_superior**: 动态模型 calendar sum 未超越 Baseline A。")
        lines.append(">")
        # 分析失败原因
        skip_rate = metrics["dynamic"].get("skip_rate", 0)
        pullback_fill = metrics["dynamic"].get("pullback_fill_rate", 0)
        lines.append("> 可能原因分析：")
        if skip_rate > 0.2:
            lines.append(f">   - skip 过多（skip_rate={skip_rate:.1%}），错过了有利 events")
        if pullback_fill < 0.5:
            lines.append(f">   - pullback 成交率低（fill_rate={pullback_fill:.1%}），等待回调导致错过入场")
        lines.append(">   - regime 分类阈值可能不适合当前市场结构")
        lines.append("")

    # --- 3. Regime 分析 ---
    lines.append("## 3. Regime 分析")
    lines.append("")

    regime_stats = regime_stability.get("regime_stats", {})
    if regime_stats:
        lines.append("### 3.1 Regime 分布统计")
        lines.append("")
        lines.append("| Regime | Count | Trade Count | Win Rate | Avg PnL (%) | Calendar 贡献 (%) |")
        lines.append("|--------|-------|-------------|----------|-------------|-------------------|")
        for regime, stats in sorted(regime_stats.items(), key=lambda x: -x[1]["count"]):
            lines.append(
                f"| {regime} | {stats['count']} | {stats['trade_count']} | "
                f"{stats['win_rate']:.1%} | {stats['avg_pnl_pct']:.2f} | "
                f"{stats['calendar_contribution_pct']:.2f} |"
            )
        lines.append("")

    lines.append("### 3.2 Train/Test 分布稳定性")
    lines.append("")
    train_dist = regime_stability.get("train_distribution", {})
    test_dist = regime_stability.get("test_distribution", {})
    if train_dist or test_dist:
        lines.append("| Regime | Train (%) | Test (%) | 差异 (pp) |")
        lines.append("|--------|-----------|----------|-----------|")
        all_regimes = sorted(set(list(train_dist.keys()) + list(test_dist.keys())))
        for regime in all_regimes:
            t_pct = train_dist.get(regime, 0.0) * 100
            te_pct = test_dist.get(regime, 0.0) * 100
            diff = te_pct - t_pct
            lines.append(f"| {regime} | {t_pct:.1f} | {te_pct:.1f} | {diff:+.1f} |")
        lines.append("")

    if regime_stability.get("regime_distribution_shift"):
        shifted = regime_stability.get("shifted_regimes", [])
        lines.append(f"> ⚠️ `regime_distribution_shift=true`：以下 regime 在 train/test 间分布差异超 20pp：{', '.join(shifted)}")
        lines.append("")

    # --- 4. 参数敏感性 ---
    lines.append("## 4. 参数敏感性")
    lines.append("")

    if high_sensitivity_params:
        lines.append(f"**高敏感参数**（calendar_sum 变化幅度 > ±3%）：{', '.join(high_sensitivity_params)}")
        lines.append("")

    sensitivities = sensitivity.get("sensitivities", {})
    if sensitivities:
        lines.append("| 参数 | Min Cal Sum (%) | Max Cal Sum (%) | Range (%) | 敏感性 |")
        lines.append("|------|-----------------|-----------------|-----------|--------|")
        for param_name, df in sorted(sensitivities.items()):
            # Support both pd.DataFrame and plain dict with list values
            if hasattr(df, "min"):
                # pandas DataFrame
                min_val = float(df["calendar_sum_pct"].min())
                max_val = float(df["calendar_sum_pct"].max())
            else:
                # plain dict with list
                vals = df["calendar_sum_pct"] if isinstance(df, dict) else df
                min_val = float(min(vals))
                max_val = float(max(vals))
            range_val = max_val - min_val
            flag = "⚠️ HIGH" if param_name in high_sensitivity_params else "OK"
            lines.append(f"| {param_name} | {min_val:.2f} | {max_val:.2f} | {range_val:.2f} | {flag} |")
        lines.append("")

    # --- 5. Bootstrap 置信区间 ---
    lines.append("## 5. Bootstrap 置信区间")
    lines.append("")
    lines.append(f"Bootstrap 重采样次数：1000")
    lines.append("")
    lines.append("| Symbol | P5 (%) | P95 (%) | Mean (%) |")
    lines.append("|--------|--------|---------|----------|")

    btc_ci = bootstrap_ci.get("btc_ci", {})
    eth_ci = bootstrap_ci.get("eth_ci", {})
    combined_ci = bootstrap_ci.get("combined_ci", {})

    lines.append(
        f"| BTC | {btc_ci.get('p5', 0):.2f} | {btc_ci.get('p95', 0):.2f} | {btc_ci.get('mean', 0):.2f} |"
    )
    lines.append(
        f"| ETH | {eth_ci.get('p5', 0):.2f} | {eth_ci.get('p95', 0):.2f} | {eth_ci.get('mean', 0):.2f} |"
    )
    lines.append(
        f"| Combined | {combined_ci.get('p5', 0):.2f} | {combined_ci.get('p95', 0):.2f} | {combined_ci.get('mean', 0):.2f} |"
    )
    lines.append("")

    if small_sample_warning:
        lines.append("> ⚠️ `small_sample_warning=true`：116 events 样本量下，置信区间较宽，结论需谨慎解读。")
        lines.append("")

    # --- 6. 与 V6 Baseline 差距分析 ---
    lines.append("## 6. 与 V6 Baseline 差距分析")
    lines.append("")
    v6_baseline = 33.02
    gap_to_v6 = v6_baseline - dynamic_cal
    lines.append(f"- **V6 Baseline（reentry-trigger 入场）**: {v6_baseline:.2f}%")
    lines.append(f"- **Dynamic Timing（真实 tick 入场）**: {dynamic_cal:.2f}%")
    lines.append(f"- **差距**: {gap_to_v6:.2f}%")
    lines.append("")
    lines.append("**差距来源分析**：")
    lines.append("")
    lines.append("V6 的 33.02% 来自以下组合优势，entry timing 层无法弥补：")
    lines.append("")
    lines.append("1. **Reentry-trigger 入场**：V6 使用 planned price 成交（乐观假设），dynamic timing 使用真实 tick 价格")
    lines.append("2. **极紧止损（0.05 ATR）**：V6 止损极窄，avg loss 仅 -0.08%；V4 execution 使用 0.45 ATR 止损")
    lines.append("3. **链式 Reentry**：V6 允许同一 signal bar 内多次 reentry，放大盈利 events 的收益")
    lines.append("4. **宽 Trailing**：V6 trailing 策略允许更大的盈利空间")
    lines.append("")
    lines.append("entry timing 优化仅能改善「何时入场」，无法弥补执行模型（止损宽度、reentry 次数、trailing 策略）的结构性差异。")
    lines.append("")

    # --- 7. Go/No-Go 判定 ---
    lines.append("## 7. Go/No-Go 判定")
    lines.append("")
    lines.append(f"### 判定结果：{go_label}")
    lines.append("")
    lines.append("**判定依据**：")
    lines.append("")
    lines.append(f"- Dynamic calendar_sum: {dynamic_cal:.2f}%")
    lines.append(f"- Baseline A calendar_sum: {baseline_a_cal:.2f}%")
    lines.append(f"- 改善幅度: {improvement:+.2f}%")
    lines.append(f"- Overfitting flag: {overfitting_flag}")
    lines.append(f"- High sensitivity params: {high_sensitivity_params if high_sensitivity_params else 'None'}")
    lines.append("")

    lines.append("**判定规则**：")
    lines.append("")
    lines.append("| 条件 | 判定 |")
    lines.append("|------|------|")
    lines.append("| 改善 > 1.0% 且无 overfitting 且无 high_sensitivity | ✅ Go |")
    lines.append("| 改善 > 0 但 ≤ 1.0%，或有 high_sensitivity | ⚠️ Conditional Go |")
    lines.append("| 改善 ≤ 0 或 overfitting_flag=true | ❌ No-Go |")
    lines.append("")

    # 分 symbol 验证
    symbol_metrics = evaluation.get("symbol_metrics", {})
    if symbol_metrics:
        lines.append("**分 Symbol 验证**：")
        lines.append("")
        lines.append("| Symbol | Calendar Sum (%) | Win Rate | Negative Flag |")
        lines.append("|--------|-----------------|----------|---------------|")
        for sym, sm in sorted(symbol_metrics.items()):
            neg_flag = "⚠️ YES" if sm.get("negative_flag") else "No"
            lines.append(
                f"| {sym} | {sm['calendar_sum_pct']:.2f} | {sm['win_rate']:.1%} | {neg_flag} |"
            )
        lines.append("")

    # Overfitting 详情
    if overfitting_flag:
        train_cal = evaluation.get("train_calendar_sum", 0)
        test_cal = evaluation.get("test_calendar_sum", 0)
        drop = (train_cal - test_cal) / train_cal * 100 if train_cal > 0 else 0
        lines.append(f"> ⚠️ **Overfitting 警告**：train calendar_sum={train_cal:.2f}% → test={test_cal:.2f}%，下降 {drop:.1f}%（阈值 50%）")
        lines.append(">")
        lines.append("> 建议简化模型（减少参数维度或使用更宽松的阈值）。")
        lines.append("")

    # --- 8. 下一步行动建议 ---
    lines.append("## 8. 下一步行动建议")
    lines.append("")

    if go_decision == "Go":
        lines.append("### ✅ 推荐：进入 design 阶段")
        lines.append("")
        lines.append("1. 设计 live-compatible 的动态 timing 实现")
        lines.append("2. 将规则型分类器集成到实盘执行框架")
        lines.append("3. 设计 A/B test 方案验证线上效果")
        lines.append("4. 考虑与 reentry 机制结合以进一步缩小与 V6 的差距")
    elif go_decision == "Conditional Go":
        lines.append("### ⚠️ 建议：扩大样本量验证或简化模型")
        lines.append("")
        lines.append("1. 等待更多 V6 gate events 积累（目标 200+ events）后重新验证")
        lines.append("2. 简化模型：减少参数维度，降低过拟合风险")
        if has_high_sensitivity:
            lines.append(f"3. 重点关注高敏感参数：{', '.join(high_sensitivity_params)}，考虑固定或收窄搜索范围")
        lines.append("4. 考虑仅保留 Strong Momentum → immediate 规则，去除复杂的 pullback 逻辑")
    else:  # No-Go
        lines.append("### ❌ 建议：回退到静态参数优化路径")
        lines.append("")
        lines.append("1. 回退到 `original-t2-entry-logic-redesign` spec 的静态参数优化")
        lines.append("2. 在固定 delay 框架内优化 D 值（D=5s vs D=10s vs D=15s）")
        lines.append("3. 聚焦执行模型优化（止损、trailing）而非入场时机")
        if dynamic_timing_not_superior:
            lines.append("4. 分析 dynamic timing 失败原因，为后续迭代积累经验")

    lines.append("")
    lines.append("---")
    lines.append("")
    lines.append("*报告由 dynamic_entry_timing_runner 自动生成*")
    lines.append("")

    # --- 写入文件 ---
    report_content = "\n".join(lines)
    output_dir.mkdir(parents=True, exist_ok=True)
    report_path = output_dir / "dynamic_timing_report.md"
    report_path.write_text(report_content, encoding="utf-8")

    return report_content


# ---------------------------------------------------------------------------
# 主入口
# ---------------------------------------------------------------------------


def main() -> None:
    """串联完整流程：加载数据 → time split → grid search → evaluate → 报告。

    所有输出写入 research/entry_redesign/scripts/output/dynamic_timing/
    确保确定性：不使用 datetime.now()、未 seed 随机源。
    """
    from .data_layer import load_v6_gate_events, load_bars_cache, time_split_events

    # Output directory
    output_dir = Path(__file__).resolve().parents[1] / "output" / "dynamic_timing"
    output_dir.mkdir(parents=True, exist_ok=True)

    print("=" * 60)
    print("Dynamic Entry Timing Model - Full Pipeline")
    print("=" * 60)

    # Step 1: Load data
    print("\n[1/8] Loading V6 gate events...")
    events = load_v6_gate_events()
    print(f"  Loaded {len(events)} events")

    print("\n[2/8] Loading 1s bar cache...")
    bars_cache = load_bars_cache(events)
    print(f"  Loaded {len(bars_cache)} month caches")

    # Step 2: Time split
    print("\n[3/8] Time splitting events (60/40)...")
    train_events, test_events = time_split_events(events)
    print(f"  Train: {len(train_events)} events, Test: {len(test_events)} events")

    # Step 3: Grid search layer 1
    print("\n[4/8] Grid search layer 1 (128 combinations)...")
    layer1_best, layer1_df = run_grid_search_layer1(
        train_events, bars_cache, output_dir=output_dir
    )
    train_cal_sum_l1 = compute_calendar_sum(
        run_dynamic_timing(train_events, bars_cache, layer1_best)
    )
    print(f"  Layer 1 best: max_steps={layer1_best.max_steps}, "
          f"strong_momentum={layer1_best.strong_momentum_threshold}, "
          f"extension={layer1_best.extension_threshold}")
    print(f"  Layer 1 train calendar_sum: {train_cal_sum_l1:.2f}%")

    # Step 4: Grid search layer 2
    print("\n[5/8] Grid search layer 2 (972 combinations)...")
    best_params, layer2_df = run_grid_search_layer2(
        train_events, bars_cache, layer1_best, output_dir=output_dir
    )
    train_cal_sum = compute_calendar_sum(
        run_dynamic_timing(train_events, bars_cache, best_params)
    )
    print(f"  Final best params determined")
    print(f"  Train calendar_sum: {train_cal_sum:.2f}%")

    # Step 5: Evaluate on test set
    print("\n[6/8] Evaluating on test set...")
    evaluation = evaluate_on_test_set(
        test_events, bars_cache, best_params, train_calendar_sum=train_cal_sum
    )
    print(f"  Test calendar_sum: {evaluation['test_calendar_sum']:.2f}%")
    print(f"  Overfitting flag: {evaluation['overfitting_flag']}")

    # Step 6: Additional analyses
    print("\n[7/8] Running additional analyses...")

    # Bootstrap CI
    bootstrap_ci = compute_bootstrap_ci(evaluation["dynamic_results"])
    print(f"  Bootstrap CI (combined): [{bootstrap_ci['combined_ci']['p5']:.2f}%, {bootstrap_ci['combined_ci']['p95']:.2f}%]")

    # Sensitivity analysis
    sensitivity = run_sensitivity_analysis(
        train_events, bars_cache, best_params, output_dir=output_dir
    )
    print(f"  High sensitivity params: {sensitivity['high_sensitivity_params']}")

    # Regime stability
    train_results = run_dynamic_timing(train_events, bars_cache, best_params)
    regime_stability = compute_regime_stability(train_results, evaluation["dynamic_results"])
    print(f"  Regime distribution shift: {regime_stability['regime_distribution_shift']}")

    # Step 7: Generate output files and report
    print("\n[8/8] Generating output files and report...")
    output_files = generate_output_files(
        dynamic_results=evaluation["dynamic_results"],
        baseline_a_results=evaluation["baselines"]["baseline_a"],
        evaluation=evaluation,
        best_params=best_params,
        bootstrap_ci=bootstrap_ci,
        regime_stability=regime_stability,
        sensitivity=sensitivity,
        output_dir=output_dir,
    )

    report = generate_report(
        evaluation=evaluation,
        best_params=best_params,
        bootstrap_ci=bootstrap_ci,
        regime_stability=regime_stability,
        sensitivity=sensitivity,
        output_files=output_files,
        output_dir=output_dir,
    )

    print(f"\n{'=' * 60}")
    print("Pipeline complete! Output files:")
    print(f"  {output_dir}/")
    for f in sorted(output_dir.iterdir()):
        if f.is_file():
            print(f"    {f.name}")
    print(f"{'=' * 60}")


if __name__ == "__main__":
    main()
