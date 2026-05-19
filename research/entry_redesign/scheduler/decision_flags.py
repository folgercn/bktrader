"""decision_flags — event_expectation_positive / small_sample_flag / asymmetry_tag 判定。

实现 Requirement 3.4 / 3.5 / 3.7 的三个判定函数，供 SummaryJsonWriter 写入
summary JSON 对应字段。

判定逻辑：

1. event_expectation_positive = true iff:
   - nogate_calendar_normalized_return_pct >= 0.0
   - AND nogate_per_trade_quality_bps_over_notional >= 10.0
   - AND nogate_win_rate > baseline_reference.nogate_win_rate
   - AND nogate_payoff_ratio >= baseline_reference.nogate_payoff_ratio × 0.95

2. small_sample_flag = true iff:
   - nogate_trade_count < 60
   - OR btc_nogate_trade_count < 20
   - OR eth_nogate_trade_count < 20
   否则显式写 false（禁止缺省）。

3. asymmetry_tag ∈ {eth_only_positive, btc_only_positive, all_symbols_positive, none}
   按 Requirement 3.7 机器可判断条件。

Requirements: 3.4, 3.5, 3.7
"""

from __future__ import annotations

from typing import Any, Literal


# ---------------------------------------------------------------------------
# 类型别名
# ---------------------------------------------------------------------------

AsymmetryTag = Literal[
    "eth_only_positive",
    "btc_only_positive",
    "all_symbols_positive",
    "none",
]
"""asymmetry_tag 的合法值域（Requirement 3.7）。"""


# ---------------------------------------------------------------------------
# 公开函数
# ---------------------------------------------------------------------------


def compute_event_expectation_positive(
    summary: dict[str, Any],
    baseline_ref: dict[str, Any],
) -> bool:
    """判定某 Entry_Candidate 是否满足 event_expectation_positive。

    判定条件（Requirement 3.4）：
      1. nogate_calendar_normalized_return_pct >= 0.0
      2. nogate_per_trade_quality_bps_over_notional >= 10.0
      3. nogate_win_rate > baseline_reference.nogate_win_rate
      4. nogate_payoff_ratio >= baseline_reference.nogate_payoff_ratio × 0.95

    当 summary 或 baseline_ref 中任一所需字段为 None（例如 trade_count == 0
    导致 win_rate / payoff_ratio 为 null）时，该条件视为不满足，返回 False。

    Args:
        summary: MetricsAggregator 产出的指标 dict，包含 nogate_* 前缀字段。
        baseline_ref: BaselineRecompute 产出的 baseline_reference dict，
            结构为 {"nogate_win_rate": float|None, "nogate_payoff_ratio": float|None, ...}。

    Returns:
        True 当且仅当四个条件全部满足；否则 False。

    Requirements: 3.4
    """
    # 条件 1: nogate_calendar_normalized_return_pct >= 0.0
    cal_return = summary.get("nogate_calendar_normalized_return_pct")
    if cal_return is None or cal_return < 0.0:
        return False

    # 条件 2: nogate_per_trade_quality_bps_over_notional >= 10.0
    quality_bps = summary.get("nogate_per_trade_quality_bps_over_notional")
    if quality_bps is None or quality_bps < 10.0:
        return False

    # 条件 3: nogate_win_rate > baseline_reference.nogate_win_rate
    candidate_win_rate = summary.get("nogate_win_rate")
    baseline_win_rate = baseline_ref.get("nogate_win_rate")
    if candidate_win_rate is None or baseline_win_rate is None:
        return False
    if not (candidate_win_rate > baseline_win_rate):
        return False

    # 条件 4: nogate_payoff_ratio >= baseline_reference.nogate_payoff_ratio × 0.95
    candidate_payoff = summary.get("nogate_payoff_ratio")
    baseline_payoff = baseline_ref.get("nogate_payoff_ratio")
    if candidate_payoff is None or baseline_payoff is None:
        return False
    if not (candidate_payoff >= baseline_payoff * 0.95):
        return False

    return True


def compute_small_sample_flag(summary: dict[str, Any]) -> bool:
    """判定某 Entry_Candidate 是否为小样本。

    判定条件（Requirement 3.5）：
      - nogate_trade_count < 60
      - OR btc_nogate_trade_count < 20
      - OR eth_nogate_trade_count < 20
    三个条件任一为 true → small_sample_flag = true；
    三个条件全部为 false → small_sample_flag = false（显式分支，禁止缺省）。

    小样本候选 MUST NOT 直接进入 R1 晋级候选。

    Args:
        summary: MetricsAggregator 产出的指标 dict，包含以下字段：
            - nogate_trade_count: 全量 nogate 模式下的 trade 行数
            - btc_nogate_trade_count: BTCUSDT 单币种 nogate 模式下的 trade 行数
            - eth_nogate_trade_count: ETHUSDT 单币种 nogate 模式下的 trade 行数

    Returns:
        True 表示小样本；False 表示样本量充足。

    Requirements: 3.5
    """
    nogate_trade_count: int = summary.get("nogate_trade_count", 0)
    btc_nogate_trade_count: int = summary.get("btc_nogate_trade_count", 0)
    eth_nogate_trade_count: int = summary.get("eth_nogate_trade_count", 0)

    if nogate_trade_count < 60:
        return True
    if btc_nogate_trade_count < 20:
        return True
    if eth_nogate_trade_count < 20:
        return True

    # 三个条件全部为 false → 显式返回 False
    return False


def compute_asymmetry_tag(summary: dict[str, Any]) -> AsymmetryTag:
    """判定 BTCUSDT / ETHUSDT 非对称性标签。

    判定条件（Requirement 3.7）：
      - asymmetry_tag == "eth_only_positive"
            iff event_expectation_positive_eth_only == true
            AND event_expectation_positive_btc_only == false
      - asymmetry_tag == "btc_only_positive"
            iff event_expectation_positive_btc_only == true
            AND event_expectation_positive_eth_only == false
      - asymmetry_tag == "all_symbols_positive"
            iff 两者均为 true
      - 其余 asymmetry_tag == "none"

    Args:
        summary: MetricsAggregator 产出的指标 dict，包含以下字段：
            - event_expectation_positive_eth_only: bool
            - event_expectation_positive_btc_only: bool

    Returns:
        AsymmetryTag 枚举值之一。

    Requirements: 3.7
    """
    eth_positive: bool = summary.get("event_expectation_positive_eth_only", False)
    btc_positive: bool = summary.get("event_expectation_positive_btc_only", False)

    if eth_positive and not btc_positive:
        return "eth_only_positive"
    elif btc_positive and not eth_positive:
        return "btc_only_positive"
    elif btc_positive and eth_positive:
        return "all_symbols_positive"
    else:
        return "none"
