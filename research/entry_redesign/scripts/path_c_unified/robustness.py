"""robustness — 稳健性验证模块。

职责：
- Bootstrap CI（1000 次，random_state=42，5th/95th percentile）
- 分 symbol bootstrap（BTC/ETH 各 1000 次）
- Overfitting check（test_cs < 0.5 × train_cs）
- LOOCV degradation check（loocv_cs < 0.7 × train_cs 或差异 > 30%）
- Regime stability（任一 regime 差异严格 > 15pp）
- Gate quality check（原 116 vs 新增 events 的 mean pnl bootstrap 检验）
- CI width 对比（vs 原 3.58pp）
- Small sample warning（≥ 200 events → false）

复用：
- classifier_trainer._compute_calendar_sum_silo 的 silo 计算逻辑
- pre_breakout_timing.delay_simulator.DelayResult 结构
"""

from __future__ import annotations

import logging
from dataclasses import dataclass, field

import numpy as np
import pandas as pd

from pre_breakout_timing.delay_simulator import DelayResult

logger = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# Constants — 与 classifier_trainer 保持一致
# ---------------------------------------------------------------------------

_INITIAL_BALANCE = 100_000.0
_NOTIONAL_SHARE = 0.26

# 原 116 events 的 CI width（refinement A2: 5.18% - 1.60% = 3.58pp）
_ORIGINAL_CI_WIDTH = 3.58


# ---------------------------------------------------------------------------
# Data Classes
# ---------------------------------------------------------------------------


@dataclass
class BootstrapResult:
    """Bootstrap CI 结果。"""

    n_bootstrap: int
    mean: float
    ci_lower: float  # 5th percentile
    ci_upper: float  # 95th percentile
    ci_width: float
    values: list[float]  # 所有 bootstrap 样本值


@dataclass
class GateQualityResult:
    """Gate 质量检验结果。"""

    original_mean_pnl: float
    new_events_mean_pnl: float
    pnl_diff: float
    p_value: float
    degradation: bool  # p < 0.05 且 diff > 0.5%


@dataclass
class RobustnessReport:
    """完整稳健性报告。"""

    overall_bootstrap: BootstrapResult
    symbol_bootstrap: dict[str, BootstrapResult]
    overfitting_flag: bool
    loocv_degradation_flag: bool
    label_distribution_shift: bool
    gate_quality: GateQualityResult
    ci_width_comparison: dict  # {"original": 3.58, "expanded": X}
    small_sample_warning: bool


# ---------------------------------------------------------------------------
# Public API
# ---------------------------------------------------------------------------


def run_robustness_checks(
    test_delay_results: list[DelayResult],
    test_events: pd.DataFrame,
    train_calendar_sum: float,
    loocv_calendar_sum: float,
    test_calendar_sum: float,
    train_labels: pd.Series,
    test_labels: pd.Series,
    original_event_ids: set[str],
    n_bootstrap: int = 1000,
    random_state: int = 42,
) -> RobustnessReport:
    """执行完整稳健性验证。

    Parameters
    ----------
    test_delay_results : list[DelayResult]
        测试集的 resolved delay results（每个 event 一个 DelayResult，
        已经过 3-regime prediction resolution）。
    test_events : pd.DataFrame
        测试集 events DataFrame（含 event_id, symbol 等列）。
    train_calendar_sum : float
        训练集 calendar_sum (%)。
    loocv_calendar_sum : float
        LOOCV calendar_sum (%)。
    test_calendar_sum : float
        测试集 calendar_sum (%)。
    train_labels : pd.Series
        训练集 3-regime 标签。
    test_labels : pd.Series
        测试集 3-regime 标签。
    original_event_ids : set[str]
        原 116 events 的 event_id 集合。
    n_bootstrap : int
        Bootstrap 重采样次数（默认 1000）。
    random_state : int
        随机种子（默认 42）。

    Returns
    -------
    RobustnessReport
        完整稳健性报告。
    """
    rng = np.random.default_rng(random_state)

    # --- 1. Overall Bootstrap CI ---
    logger.info("执行 overall bootstrap CI (%d 次)...", n_bootstrap)
    overall_bootstrap = _bootstrap_calendar_sum(
        test_delay_results, n_bootstrap, rng
    )
    logger.info(
        "Overall bootstrap: mean=%.4f%%, CI=[%.4f%%, %.4f%%], width=%.4fpp",
        overall_bootstrap.mean,
        overall_bootstrap.ci_lower,
        overall_bootstrap.ci_upper,
        overall_bootstrap.ci_width,
    )

    # --- 2. Per-symbol Bootstrap ---
    logger.info("执行 per-symbol bootstrap...")
    symbol_bootstrap = _per_symbol_bootstrap(
        test_delay_results, n_bootstrap, rng
    )
    for sym, br in symbol_bootstrap.items():
        logger.info(
            "  %s: mean=%.4f%%, CI=[%.4f%%, %.4f%%]",
            sym,
            br.mean,
            br.ci_lower,
            br.ci_upper,
        )

    # --- 3. Overfitting Check ---
    overfitting_flag = test_calendar_sum < 0.5 * train_calendar_sum
    if overfitting_flag:
        logger.warning(
            "Overfitting flag: test_cs=%.4f%% < 0.5 × train_cs=%.4f%%",
            test_calendar_sum,
            train_calendar_sum,
        )

    # --- 4. LOOCV Degradation Check ---
    loocv_degradation_flag = _check_loocv_degradation(
        loocv_calendar_sum, train_calendar_sum
    )
    if loocv_degradation_flag:
        logger.warning(
            "LOOCV degradation flag: loocv_cs=%.4f%%, train_cs=%.4f%%",
            loocv_calendar_sum,
            train_calendar_sum,
        )

    # --- 5. Regime Stability (Label Distribution Shift) ---
    label_distribution_shift = _check_label_distribution_shift(
        train_labels, test_labels
    )
    if label_distribution_shift:
        logger.warning("Label distribution shift detected (>15pp in some regime)")

    # --- 6. Gate Quality Check ---
    logger.info("执行 gate quality check...")
    gate_quality = _gate_quality_check(
        test_delay_results, test_events, original_event_ids, n_bootstrap, rng
    )
    if gate_quality.degradation:
        logger.warning(
            "Gate quality degradation: original_mean=%.4f%%, new_mean=%.4f%%, "
            "diff=%.4f%%, p=%.4f",
            gate_quality.original_mean_pnl,
            gate_quality.new_events_mean_pnl,
            gate_quality.pnl_diff,
            gate_quality.p_value,
        )

    # --- 7. CI Width Comparison ---
    ci_width_comparison = {
        "original": _ORIGINAL_CI_WIDTH,
        "expanded": overall_bootstrap.ci_width,
    }
    logger.info(
        "CI width comparison: original=%.2fpp, expanded=%.2fpp",
        _ORIGINAL_CI_WIDTH,
        overall_bootstrap.ci_width,
    )

    # --- 8. Small Sample Warning ---
    total_events = len(test_events) + len(train_labels)
    small_sample_warning = total_events < 200
    if small_sample_warning:
        logger.warning(
            "Small sample warning: total_events=%d < 200", total_events
        )

    return RobustnessReport(
        overall_bootstrap=overall_bootstrap,
        symbol_bootstrap=symbol_bootstrap,
        overfitting_flag=overfitting_flag,
        loocv_degradation_flag=loocv_degradation_flag,
        label_distribution_shift=label_distribution_shift,
        gate_quality=gate_quality,
        ci_width_comparison=ci_width_comparison,
        small_sample_warning=small_sample_warning,
    )


# ---------------------------------------------------------------------------
# Internal Helpers — Bootstrap
# ---------------------------------------------------------------------------


def _bootstrap_calendar_sum(
    delay_results: list[DelayResult],
    n_bootstrap: int,
    rng: np.random.Generator,
) -> BootstrapResult:
    """对 delay_results 执行 bootstrap 重采样，每次计算 silo calendar_sum。"""
    n = len(delay_results)
    if n == 0:
        return BootstrapResult(
            n_bootstrap=n_bootstrap,
            mean=0.0,
            ci_lower=0.0,
            ci_upper=0.0,
            ci_width=0.0,
            values=[],
        )

    bootstrap_values: list[float] = []
    for _ in range(n_bootstrap):
        indices = rng.integers(0, n, size=n)
        sample = [delay_results[i] for i in indices]
        cs = _compute_calendar_sum_silo(sample)
        bootstrap_values.append(cs)

    mean = float(np.mean(bootstrap_values))
    ci_lower = float(np.percentile(bootstrap_values, 5))
    ci_upper = float(np.percentile(bootstrap_values, 95))
    ci_width = ci_upper - ci_lower

    return BootstrapResult(
        n_bootstrap=n_bootstrap,
        mean=mean,
        ci_lower=ci_lower,
        ci_upper=ci_upper,
        ci_width=ci_width,
        values=bootstrap_values,
    )


def _per_symbol_bootstrap(
    delay_results: list[DelayResult],
    n_bootstrap: int,
    rng: np.random.Generator,
) -> dict[str, BootstrapResult]:
    """分 symbol 独立 bootstrap（BTC/ETH 各 n_bootstrap 次）。"""
    symbol_results: dict[str, list[DelayResult]] = {}
    for dr in delay_results:
        symbol = _infer_symbol_from_event_id(dr.event_id)
        if symbol not in symbol_results:
            symbol_results[symbol] = []
        symbol_results[symbol].append(dr)

    result: dict[str, BootstrapResult] = {}
    for symbol, results in symbol_results.items():
        br = _bootstrap_calendar_sum(results, n_bootstrap, rng)
        result[symbol] = br

    return result


# ---------------------------------------------------------------------------
# Internal Helpers — Overfitting & LOOCV Degradation
# ---------------------------------------------------------------------------


def _check_loocv_degradation(
    loocv_cs: float, train_cs: float
) -> bool:
    """LOOCV degradation check.

    Flag = True iff:
    - loocv_cs < 0.7 × train_cs, OR
    - |loocv_cs - train_cs| / train_cs > 0.3 (差异 > 30%)

    Special case: if train_cs == 0, avoid division by zero.
    """
    if train_cs == 0:
        # If train_cs is 0, degradation only if loocv_cs is significantly negative
        return loocv_cs < 0.0

    condition_1 = loocv_cs < 0.7 * train_cs
    condition_2 = abs(loocv_cs - train_cs) / abs(train_cs) > 0.3

    return condition_1 or condition_2


# ---------------------------------------------------------------------------
# Internal Helpers — Label Distribution Shift
# ---------------------------------------------------------------------------


def _check_label_distribution_shift(
    train_labels: pd.Series,
    test_labels: pd.Series,
) -> bool:
    """检查 train/test 标签分布差异。

    标记 label_distribution_shift=True 当且仅当任一 regime 的比例差异
    **严格大于** 15pp。
    """
    regimes = ["skip", "fast", "slow"]

    train_total = len(train_labels)
    test_total = len(test_labels)

    if train_total == 0 or test_total == 0:
        return False

    for regime in regimes:
        train_pct = (train_labels == regime).sum() / train_total * 100.0
        test_pct = (test_labels == regime).sum() / test_total * 100.0
        diff_pp = abs(train_pct - test_pct)
        if diff_pp > 15.0:
            logger.info(
                "Regime '%s' distribution shift: train=%.1f%%, test=%.1f%%, "
                "diff=%.1fpp > 15pp",
                regime,
                train_pct,
                test_pct,
                diff_pp,
            )
            return True

    return False


# ---------------------------------------------------------------------------
# Internal Helpers — Gate Quality Check
# ---------------------------------------------------------------------------


def _gate_quality_check(
    test_delay_results: list[DelayResult],
    test_events: pd.DataFrame,
    original_event_ids: set[str],
    n_bootstrap: int,
    rng: np.random.Generator,
) -> GateQualityResult:
    """Gate quality check: 对比原 116 events 子集与新增 events 子集的 per-event 平均 pnl。

    使用 bootstrap 检验两组均值差异是否显著（p < 0.05）。
    degradation = True iff p < 0.05 且 diff > 0.5%。
    """
    # 分离原始 events 和新增 events 的 pnl
    original_pnls: list[float] = []
    new_pnls: list[float] = []

    for dr in test_delay_results:
        pnl = dr.pnl_pct if (dr.traded and dr.pnl_pct is not None) else 0.0
        if dr.event_id in original_event_ids:
            original_pnls.append(pnl)
        else:
            new_pnls.append(pnl)

    original_mean = float(np.mean(original_pnls)) if original_pnls else 0.0
    new_mean = float(np.mean(new_pnls)) if new_pnls else 0.0
    pnl_diff = original_mean - new_mean  # positive means original is better

    # Bootstrap p-value: 检验 original_mean > new_mean 是否显著
    p_value = _bootstrap_mean_diff_pvalue(
        original_pnls, new_pnls, n_bootstrap, rng
    )

    # degradation: p < 0.05 且 diff > 0.5%
    degradation = p_value < 0.05 and pnl_diff > 0.5

    return GateQualityResult(
        original_mean_pnl=original_mean,
        new_events_mean_pnl=new_mean,
        pnl_diff=pnl_diff,
        p_value=p_value,
        degradation=degradation,
    )


def _bootstrap_mean_diff_pvalue(
    group_a: list[float],
    group_b: list[float],
    n_bootstrap: int,
    rng: np.random.Generator,
) -> float:
    """Bootstrap permutation test for mean difference.

    Tests H0: mean(group_a) == mean(group_b)
    Returns two-sided p-value.
    """
    if not group_a or not group_b:
        return 1.0  # 无法检验

    observed_diff = float(np.mean(group_a)) - float(np.mean(group_b))

    # 合并两组，进行 permutation test
    combined = np.array(group_a + group_b)
    n_a = len(group_a)
    n_total = len(combined)

    count_extreme = 0
    for _ in range(n_bootstrap):
        perm = rng.permutation(n_total)
        perm_a = combined[perm[:n_a]]
        perm_b = combined[perm[n_a:]]
        perm_diff = float(np.mean(perm_a)) - float(np.mean(perm_b))
        if abs(perm_diff) >= abs(observed_diff):
            count_extreme += 1

    p_value = count_extreme / n_bootstrap
    return p_value


# ---------------------------------------------------------------------------
# Internal Helpers — Silo-based Calendar Sum (复用 classifier_trainer 逻辑)
# ---------------------------------------------------------------------------


def _compute_calendar_sum_silo(results: list[DelayResult]) -> float:
    """计算 silo-based calendar sum (%)。

    每个 (symbol, month) 独立从 100k 开始，各 silo return 简单加和。
    使用 notional_share=0.26（与 execute_trade 默认值一致）。

    从 event_id 推断 symbol（与 classifier_trainer._infer_symbol_from_event_id 一致）。
    """
    silos: dict[str, list[DelayResult]] = {}
    for r in results:
        if not r.traded or r.pnl_pct is None:
            continue
        symbol = _infer_symbol_from_event_id(r.event_id)
        entry_time = pd.Timestamp(r.entry_time)
        month_key = f"{symbol}_{entry_time.strftime('%Y-%m')}"
        if month_key not in silos:
            silos[month_key] = []
        silos[month_key].append(r)

    total_return_pct = 0.0
    for _silo_key, silo_results in silos.items():
        balance = _INITIAL_BALANCE
        sorted_results = sorted(silo_results, key=lambda r: r.entry_time)
        for r in sorted_results:
            notional = balance * _NOTIONAL_SHARE
            pnl = notional * r.pnl_pct
            balance += pnl
        silo_return = (balance - _INITIAL_BALANCE) / _INITIAL_BALANCE * 100.0
        total_return_pct += silo_return

    return total_return_pct


def _infer_symbol_from_event_id(event_id: str) -> str:
    """从 event_id 推断 symbol。"""
    eid_upper = event_id.upper()
    if "BTC" in eid_upper:
        return "BTCUSDT"
    elif "ETH" in eid_upper:
        return "ETHUSDT"
    return "unknown"
