"""BaselineRecompute — Baseline_Entry_Candidate 重算模块。

固定六元组 (D=0, H=0, Trigger_Confirmation=none, Entry_Price_Mode=market_on_touch,
Pretouch_State_Band=none, Posttouch_Quality_Band=none)。

在同 runner / 同 events / 同 cost model / 同 seed 下重算 nogate_win_rate /
nogate_payoff_ratio 及单 symbol 版本，写入 summary JSON 的 baseline_reference 节点。

baseline_reference 节点结构：
{
    "nogate_win_rate": float | null,
    "nogate_payoff_ratio": float | null,
    "BTCUSDT": {
        "nogate_win_rate": float | null,
        "nogate_payoff_ratio": float | null,
    },
    "ETHUSDT": {
        "nogate_win_rate": float | null,
        "nogate_payoff_ratio": float | null,
    },
}

Requirements: 2.9, 3.4, 5.1
"""

from __future__ import annotations

from typing import Optional, Sequence

from research.entry_redesign.ledger.ledger_csv_writer import TradeRecord
from research.entry_redesign.scheduler.default_subset import BASELINE


# ---------------------------------------------------------------------------
# 常量
# ---------------------------------------------------------------------------

_SYMBOLS: tuple[str, ...] = ("BTCUSDT", "ETHUSDT")
"""评估涉及的 symbol 集合。"""


# ---------------------------------------------------------------------------
# BaselineRecompute
# ---------------------------------------------------------------------------


class BaselineRecompute:
    """Baseline_Entry_Candidate 重算器。

    职责：
      1. 使用 BASELINE spec (D=0, H=0, none, market_on_touch, none, none)。
      2. 从 baseline 运行产出的 ledger 中提取 nogate 模式下的 trades。
      3. 计算 nogate_win_rate / nogate_payoff_ratio（全量 + 单 symbol）。
      4. 返回 baseline_reference dict 供 summary JSON 使用。

    所有其他 Entry_Candidate MUST 与该 baseline 在同一 runner、同一 events、
    同一 cost model、同一 seed 下对照（Requirement 2.9）。

    用法：
        recomputer = BaselineRecompute()
        baseline_ref = recomputer.compute_baseline_reference(baseline_trades)
        # baseline_ref 写入 summary JSON 的 "baseline_reference" 节点
    """

    def __init__(self) -> None:
        """初始化 BaselineRecompute。

        使用 default_subset.py 中的 BASELINE 常量作为六元组定义。
        """
        self._spec = BASELINE

    @property
    def spec(self):
        """返回 Baseline_Entry_Candidate 的 EntryCandidateSpec。"""
        return self._spec

    def compute_baseline_reference(
        self,
        baseline_trades: Sequence[TradeRecord],
    ) -> dict:
        """从 baseline ledger trades 中计算 baseline_reference 节点。

        仅使用 gate_mode == "nogate" 的 trades 进行计算。

        Args:
            baseline_trades: Baseline_Entry_Candidate 运行产出的全部
                TradeRecord 序列（包含 nogate 和 gate001 两种 gate_mode）。

        Returns:
            baseline_reference dict，结构如下：
            {
                "nogate_win_rate": float | None,
                "nogate_payoff_ratio": float | None,
                "BTCUSDT": {
                    "nogate_win_rate": float | None,
                    "nogate_payoff_ratio": float | None,
                },
                "ETHUSDT": {
                    "nogate_win_rate": float | None,
                    "nogate_payoff_ratio": float | None,
                },
            }

        Requirements: 2.9, 3.4, 5.1
        """
        # 过滤出 nogate 模式的 trades
        nogate_trades = [
            t for t in baseline_trades if t.gate_mode == "nogate"
        ]

        # 全量计算
        all_win_rate = self._compute_win_rate(nogate_trades)
        all_payoff_ratio = self._compute_payoff_ratio(nogate_trades)

        # 单 symbol 计算
        per_symbol: dict[str, dict[str, Optional[float]]] = {}
        for symbol in _SYMBOLS:
            symbol_trades = [t for t in nogate_trades if t.symbol == symbol]
            per_symbol[symbol] = {
                "nogate_win_rate": self._compute_win_rate(symbol_trades),
                "nogate_payoff_ratio": self._compute_payoff_ratio(symbol_trades),
            }

        return {
            "nogate_win_rate": all_win_rate,
            "nogate_payoff_ratio": all_payoff_ratio,
            **per_symbol,
        }

    @staticmethod
    def _compute_win_rate(
        trades: Sequence[TradeRecord],
    ) -> Optional[float]:
        """计算 win_rate = count(realistic_pnl_i > 0) / trade_count。

        数学定义（Requirement 3.2）：
            win_rate = count(realistic_pnl_i > 0) / trade_count
            trade_count == 0 时写 null。

        Args:
            trades: nogate 模式下的 TradeRecord 序列。

        Returns:
            win_rate 浮点值，或 None（trade_count == 0）。
        """
        trade_count = len(trades)
        if trade_count == 0:
            return None
        win_count = sum(1 for t in trades if t.realistic_pnl > 0.0)
        return win_count / trade_count

    @staticmethod
    def _compute_payoff_ratio(
        trades: Sequence[TradeRecord],
    ) -> Optional[float]:
        """计算 payoff_ratio = mean(R_i | R_i > 0) / abs(mean(R_i | R_i < 0))。

        数学定义（Requirement 3.2）：
            R_i = realistic_pnl_i / notional_i
            payoff_ratio = mean(R_i | R_i > 0) / abs(mean(R_i | R_i < 0))
            若分子或分母任一侧样本数为 0，THEN 写 null。

        Args:
            trades: nogate 模式下的 TradeRecord 序列。

        Returns:
            payoff_ratio 浮点值，或 None（任一侧样本数为 0）。
        """
        # 计算每笔 R_i = realistic_pnl_i / notional_i
        positive_rs: list[float] = []
        negative_rs: list[float] = []

        for t in trades:
            if t.notional == 0.0:
                # notional 为 0 的 trade 不应出现在 ledger 中（P9），
                # 但防御性跳过
                continue
            r_i = t.realistic_pnl / t.notional
            if r_i > 0.0:
                positive_rs.append(r_i)
            elif r_i < 0.0:
                negative_rs.append(r_i)
            # r_i == 0.0 不计入任一侧

        # 任一侧样本数为 0 → null
        if len(positive_rs) == 0 or len(negative_rs) == 0:
            return None

        mean_positive = sum(positive_rs) / len(positive_rs)
        mean_negative = sum(negative_rs) / len(negative_rs)

        # abs(mean_negative) 作为分母
        abs_mean_negative = abs(mean_negative)
        if abs_mean_negative == 0.0:
            # 理论上不可能（negative_rs 非空意味着 mean < 0），
            # 但防御性返回 null
            return None

        return mean_positive / abs_mean_negative
