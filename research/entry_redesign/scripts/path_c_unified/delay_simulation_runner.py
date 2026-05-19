"""
delay_simulation_runner — Multi-Delay Simulation 执行模块

职责：
- 对扩展事件池执行 multi-delay simulation，产出完整 Delay_PnL_Matrix
- 一致性校验：对比原 116 events 的 pnl_pct 与已有 delay_pnl_matrix.csv
- 记录执行统计（total/simulated/skipped/per_delay_traded_rate）

复用 pre_breakout_timing.delay_simulator.simulate_all_delays()，
使用与原实验相同的 pullback_params。
"""

from __future__ import annotations

import logging
import time
from dataclasses import dataclass, field
from pathlib import Path

import numpy as np
import pandas as pd

from pre_breakout_timing.delay_simulator import (
    DelayResult,
    simulate_all_delays,
)

logger = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# Constants — 与原实验一致的 pullback 参数
# （从 pre_breakout_timing_runner.py DEFAULT_PULLBACK_PARAMS 确认）
# ---------------------------------------------------------------------------
DEFAULT_PULLBACK_PARAMS: dict = {
    "pullback_target_atr": 0.05,
    "pullback_window_seconds": 60,
    "start_offset_seconds": 5,
}

# delay_pnl_matrix.csv 的 15 列（与原格式一致）
MATRIX_COLUMNS: list[str] = [
    "event_id",
    "symbol",
    "side",
    "touch_time",
    "delay_label",
    "delay_seconds",
    "entry_time",
    "entry_price",
    "pnl_pct",
    "exit_reason",
    "exit_time",
    "hold_seconds",
    "mfe_r",
    "mae_r",
    "traded",
]

# Drift 阈值：max absolute pnl_pct 差异 > 0.01% 触发 warning
DRIFT_THRESHOLD: float = 0.0001  # 0.01% in decimal


# ---------------------------------------------------------------------------
# Data classes
# ---------------------------------------------------------------------------


@dataclass
class DriftCheckResult:
    """与原 delay_pnl_matrix 的一致性校验结果。"""

    n_compared: int
    max_pnl_diff: float
    mean_pnl_diff: float
    drift_warning: bool  # max_diff > 0.01%


@dataclass
class SimulationStats:
    """Simulation 执行统计。"""

    total_events: int
    simulated_events: int
    skipped_events: int
    per_delay_traded_rate: dict[str, float] = field(default_factory=dict)
    drift_check: DriftCheckResult | None = None


# ---------------------------------------------------------------------------
# Core simulation function
# ---------------------------------------------------------------------------


def run_multi_delay_simulation(
    events: pd.DataFrame,
    bars_cache: dict[str, pd.DataFrame],
    pullback_params: dict | None = None,
    original_matrix_path: Path | None = None,
) -> tuple[pd.DataFrame, SimulationStats, list[list[DelayResult]]]:
    """对所有 events 执行 multi-delay simulation。

    Parameters
    ----------
    events : pd.DataFrame
        扩展事件池（需含 event_id, symbol, side, touch_time, atr 等字段）。
    bars_cache : dict[str, pd.DataFrame]
        1s bar cache，key 为 "{symbol}_{YYYYMM}"。
    pullback_params : dict | None
        回调入场参数。若为 None，使用 DEFAULT_PULLBACK_PARAMS。
    original_matrix_path : Path | None
        原 delay_pnl_matrix.csv 路径（用于一致性校验）。
        若为 None，跳过一致性校验。

    Returns
    -------
    tuple[pd.DataFrame, SimulationStats, list[list[DelayResult]]]
        - expanded_delay_pnl_matrix: DataFrame (N_simulated × 5 rows, 15 cols)
        - stats: 执行统计
        - all_delay_results: list[list[DelayResult]]，每个 event 的 5 个 DelayResult
    """
    if pullback_params is None:
        pullback_params = DEFAULT_PULLBACK_PARAMS

    n_total = len(events)
    logger.info(
        "开始 multi-delay simulation: %d events, pullback_params=%s",
        n_total,
        pullback_params,
    )

    all_delay_results: list[list[DelayResult]] = []
    skipped_event_ids: list[str] = []
    simulated_count = 0

    t0 = time.time()

    for idx in range(n_total):
        event = events.iloc[idx]
        event_id = str(event.get("event_id", ""))
        symbol = str(event["symbol"])
        touch_time = pd.Timestamp(event["touch_time"])
        if touch_time.tzinfo is None:
            touch_time = touch_time.tz_localize("UTC")

        # 查找 bars_cache key
        month_key = f"{symbol}_{touch_time.strftime('%Y%m')}"
        second_bars = bars_cache.get(month_key)

        if second_bars is None or (
            isinstance(second_bars, pd.DataFrame) and second_bars.empty
        ):
            # bars_cache 不可用 → 跳过
            skipped_event_ids.append(event_id)
            # 产出空 DelayResult 占位
            empty_results = _make_empty_delay_results(event_id)
            all_delay_results.append(empty_results)
            continue

        # 执行 simulation
        try:
            event_results = simulate_all_delays(
                event, second_bars, pullback_params=pullback_params
            )
        except Exception as e:
            logger.warning(
                "Event %s simulation failed: %s, skipping", event_id, e
            )
            skipped_event_ids.append(event_id)
            empty_results = _make_empty_delay_results(event_id)
            all_delay_results.append(empty_results)
            continue

        all_delay_results.append(event_results)
        simulated_count += 1

        # 进度日志（每 50 个 events 或最后一个）
        if (idx + 1) % 50 == 0 or idx == n_total - 1:
            elapsed = time.time() - t0
            rate = (idx + 1) / elapsed if elapsed > 0 else 0
            logger.info(
                "  进度: %d/%d events (%.1f events/s, elapsed %.1fs)",
                idx + 1,
                n_total,
                rate,
                elapsed,
            )

    total_elapsed = time.time() - t0
    logger.info(
        "Simulation 完成: %d simulated, %d skipped, 耗时 %.1fs",
        simulated_count,
        len(skipped_event_ids),
        total_elapsed,
    )

    # --- 构建 expanded_delay_pnl_matrix DataFrame ---
    matrix_df = _build_matrix_dataframe(events, all_delay_results)

    # --- 计算 per_delay_traded_rate ---
    per_delay_traded_rate = _compute_per_delay_traded_rate(all_delay_results)

    # --- 一致性校验 ---
    drift_check: DriftCheckResult | None = None
    if original_matrix_path is not None and original_matrix_path.exists():
        drift_check = _run_drift_check(matrix_df, original_matrix_path)
        if drift_check.drift_warning:
            logger.warning(
                "⚠ Drift detected! max_pnl_diff=%.6f%% (threshold=%.4f%%)",
                drift_check.max_pnl_diff * 100,
                DRIFT_THRESHOLD * 100,
            )
        else:
            logger.info(
                "✓ 一致性校验通过: max_pnl_diff=%.6f%%, n_compared=%d",
                drift_check.max_pnl_diff * 100,
                drift_check.n_compared,
            )
    elif original_matrix_path is not None:
        logger.warning(
            "原 delay_pnl_matrix.csv 不存在: %s, 跳过一致性校验",
            original_matrix_path,
        )

    # --- 构建 stats ---
    stats = SimulationStats(
        total_events=n_total,
        simulated_events=simulated_count,
        skipped_events=len(skipped_event_ids),
        per_delay_traded_rate=per_delay_traded_rate,
        drift_check=drift_check,
    )

    return matrix_df, stats, all_delay_results


# ---------------------------------------------------------------------------
# Internal helpers
# ---------------------------------------------------------------------------


def _make_empty_delay_results(event_id: str) -> list[DelayResult]:
    """为跳过的 event 生成 5 个空 DelayResult 占位。"""
    from pre_breakout_timing.delay_simulator import DELAY_VALUES

    results: list[DelayResult] = []
    for delay in DELAY_VALUES:
        results.append(
            DelayResult(
                event_id=event_id,
                delay_label=f"D{delay}",
                delay_seconds=delay,
                entry_time=None,
                entry_price=None,
                pnl_pct=None,
                exit_reason="NoData",
                exit_time=None,
                hold_seconds=None,
                mfe_r=None,
                mae_r=None,
                traded=False,
            )
        )
    # pullback
    results.append(
        DelayResult(
            event_id=event_id,
            delay_label="pullback",
            delay_seconds=0,
            entry_time=None,
            entry_price=None,
            pnl_pct=None,
            exit_reason="NoData",
            exit_time=None,
            hold_seconds=None,
            mfe_r=None,
            mae_r=None,
            traded=False,
        )
    )
    return results


def _build_matrix_dataframe(
    events: pd.DataFrame,
    all_delay_results: list[list[DelayResult]],
) -> pd.DataFrame:
    """将 all_delay_results 转换为 delay_pnl_matrix DataFrame（15 列格式）。"""
    rows: list[dict] = []
    for event_idx, event_results in enumerate(all_delay_results):
        event_row = events.iloc[event_idx]
        for dr in event_results:
            rows.append(
                {
                    "event_id": dr.event_id,
                    "symbol": event_row["symbol"],
                    "side": event_row["side"],
                    "touch_time": event_row["touch_time"],
                    "delay_label": dr.delay_label,
                    "delay_seconds": dr.delay_seconds,
                    "entry_time": dr.entry_time,
                    "entry_price": dr.entry_price,
                    "pnl_pct": dr.pnl_pct,
                    "exit_reason": dr.exit_reason,
                    "exit_time": dr.exit_time,
                    "hold_seconds": dr.hold_seconds,
                    "mfe_r": dr.mfe_r,
                    "mae_r": dr.mae_r,
                    "traded": dr.traded,
                }
            )

    matrix_df = pd.DataFrame(rows, columns=MATRIX_COLUMNS)
    return matrix_df


def _compute_per_delay_traded_rate(
    all_delay_results: list[list[DelayResult]],
) -> dict[str, float]:
    """计算每种 delay 的 traded 率。"""
    delay_labels = ["D0", "D5", "D10", "D15", "pullback"]
    traded_counts: dict[str, int] = {lbl: 0 for lbl in delay_labels}
    total_counts: dict[str, int] = {lbl: 0 for lbl in delay_labels}

    for event_results in all_delay_results:
        for dr in event_results:
            if dr.delay_label in total_counts:
                total_counts[dr.delay_label] += 1
                if dr.traded:
                    traded_counts[dr.delay_label] += 1

    rates: dict[str, float] = {}
    for lbl in delay_labels:
        if total_counts[lbl] > 0:
            rates[lbl] = traded_counts[lbl] / total_counts[lbl]
        else:
            rates[lbl] = 0.0

    return rates


def _run_drift_check(
    new_matrix: pd.DataFrame,
    original_matrix_path: Path,
) -> DriftCheckResult:
    """对比原 116 events 的 pnl_pct 与新 simulation 结果。

    匹配逻辑：按 (event_id, delay_label) 对齐，比较 pnl_pct 差异。
    仅比较双方都有 traded=True 且 pnl_pct 非 null 的行。
    """
    original_matrix = pd.read_csv(original_matrix_path)

    # 确保 traded 列为 bool
    original_matrix["traded"] = original_matrix["traded"].astype(bool)
    new_matrix_copy = new_matrix.copy()
    new_matrix_copy["traded"] = new_matrix_copy["traded"].astype(bool)

    # 只比较 traded=True 且 pnl_pct 非 null 的行
    orig_traded = original_matrix[
        original_matrix["traded"] & original_matrix["pnl_pct"].notna()
    ].copy()
    new_traded = new_matrix_copy[
        new_matrix_copy["traded"] & new_matrix_copy["pnl_pct"].notna()
    ].copy()

    # 按 (event_id, delay_label) 合并
    merged = orig_traded.merge(
        new_traded[["event_id", "delay_label", "pnl_pct"]],
        on=["event_id", "delay_label"],
        how="inner",
        suffixes=("_orig", "_new"),
    )

    n_compared = len(merged)

    if n_compared == 0:
        logger.warning("一致性校验: 无可比较的行（原 matrix 与新 matrix 无交集）")
        return DriftCheckResult(
            n_compared=0,
            max_pnl_diff=0.0,
            mean_pnl_diff=0.0,
            drift_warning=False,
        )

    # 计算差异
    diffs = (merged["pnl_pct_new"] - merged["pnl_pct_orig"]).abs()
    max_diff = float(diffs.max())
    mean_diff = float(diffs.mean())

    drift_warning = max_diff > DRIFT_THRESHOLD

    return DriftCheckResult(
        n_compared=n_compared,
        max_pnl_diff=max_diff,
        mean_pnl_diff=mean_diff,
        drift_warning=drift_warning,
    )
