"""CostModelApplier — 成本模型计算（baseline + stress）。

成本公式来自 Requirement 6.P4 / design.md Cost Model & Metrics 章节：

    slip_bps_per_side = 2                        # 双向合计 4 bps slippage
    maker_entry_bps   = 2
    taker_entry_bps   = 4
    taker_exit_bps    = 4

    slip_pnl = raw_pnl − (slip_bps_per_side × 2) × notional / 10000
    realistic_pnl = slip_pnl − (maker_entry_bps + taker_exit_bps) × notional / 10000
    realistic_taker_both_pnl = slip_pnl − (taker_entry_bps + taker_exit_bps) × notional / 10000

不变量 P4: realistic_taker_both_pnl <= realistic_pnl <= slip_pnl <= raw_pnl
（相对误差 1e-9 内成立）。

两套参数（baseline / stress）一起挂到每个 Entry_Candidate 的 summary JSON。

Requirements: 3.1, 6.4
"""

from __future__ import annotations

from dataclasses import dataclass


# ---------------------------------------------------------------------------
# CostModelParams — 成本模型参数（frozen dataclass）
# ---------------------------------------------------------------------------


@dataclass(frozen=True)
class CostModelParams:
    """成本模型参数集。

    Attributes:
        slip_bps_per_side: 单边滑点 bps（baseline/stress 均为 2）。
        entry_bps: 入场手续费 bps（baseline=2 maker, stress=4 taker）。
        exit_bps: 出场手续费 bps（baseline/stress 均为 4 taker）。
    """

    slip_bps_per_side: float
    entry_bps: float
    exit_bps: float


# ---------------------------------------------------------------------------
# 模块级常量：baseline / stress 两套参数
# ---------------------------------------------------------------------------

BASELINE_COST_PARAMS = CostModelParams(
    slip_bps_per_side=2.0,
    entry_bps=2.0,   # maker entry
    exit_bps=4.0,    # taker exit
)
"""Baseline 成本参数：slip=2bps/side + maker_entry=2bps + taker_exit=4bps。"""

STRESS_COST_PARAMS = CostModelParams(
    slip_bps_per_side=2.0,
    entry_bps=4.0,   # taker entry
    exit_bps=4.0,    # taker exit
)
"""Stress 成本参数（realistic_taker_both_pct）：slip=2bps/side + taker_entry=4bps + taker_exit=4bps。"""


# ---------------------------------------------------------------------------
# RawTrade — 原始交易数据（成本计算前）
# ---------------------------------------------------------------------------


@dataclass(frozen=True)
class RawTrade:
    """成本计算前的原始交易记录。

    Attributes:
        raw_pnl: 原始 PnL（未扣除任何成本）。
        notional: 名义金额（entry notional）。
        entry_price: 入场价格。
        exit_price: 出场价格。
        symbol: 交易对（BTCUSDT / ETHUSDT）。
        side: 方向（long / short）。
    """

    raw_pnl: float
    notional: float
    entry_price: float
    exit_price: float
    symbol: str
    side: str


# ---------------------------------------------------------------------------
# PricedTrade — 成本计算后的交易记录
# ---------------------------------------------------------------------------


@dataclass(frozen=True)
class PricedTrade:
    """成本计算后的交易记录，包含三层 PnL。

    不变量 P4:
        realistic_taker_both_pnl <= realistic_pnl <= slip_pnl <= raw_pnl
        （相对误差 1e-9 内成立）

    Attributes:
        raw_pnl: 原始 PnL（未扣除任何成本）。
        notional: 名义金额。
        entry_price: 入场价格。
        exit_price: 出场价格。
        symbol: 交易对。
        side: 方向。
        slip_pnl: 扣除滑点后的 PnL。
        realistic_pnl: 扣除滑点 + maker entry + taker exit 后的 PnL。
        realistic_taker_both_pnl: 扣除滑点 + taker entry + taker exit 后的 PnL。
    """

    raw_pnl: float
    notional: float
    entry_price: float
    exit_price: float
    symbol: str
    side: str
    slip_pnl: float
    realistic_pnl: float
    realistic_taker_both_pnl: float


# ---------------------------------------------------------------------------
# CostModelApplier — 成本模型应用器
# ---------------------------------------------------------------------------


class CostModelApplier:
    """将成本模型应用到原始交易，产出 PricedTrade。

    公式（design.md single source of truth）：
        slip_pnl = raw_pnl − (slip_bps_per_side × 2) × notional / 10000
        realistic_pnl = slip_pnl − (entry_bps + exit_bps) × notional / 10000
        realistic_taker_both_pnl = slip_pnl − (taker_entry_bps + taker_exit_bps) × notional / 10000

    对于 baseline 参数 (entry_bps=2, exit_bps=4):
        realistic_pnl = slip_pnl − (2 + 4) × notional / 10000

    对于 stress 参数 (entry_bps=4, exit_bps=4):
        realistic_pnl = slip_pnl − (4 + 4) × notional / 10000
        此时 realistic_pnl == realistic_taker_both_pnl

    注意：realistic_taker_both_pnl 始终使用 taker_entry=4 + taker_exit=4 计算，
    无论传入的 params 是 baseline 还是 stress。这保证了 P4 不变量：
        realistic_taker_both_pnl <= realistic_pnl <= slip_pnl <= raw_pnl
    """

    def apply(self, trade: RawTrade, params: CostModelParams) -> PricedTrade:
        """应用成本模型到原始交易。

        Args:
            trade: 原始交易记录。
            params: 成本模型参数（baseline 或 stress）。

        Returns:
            PricedTrade: 包含三层 PnL 的交易记录。

        不变量 P4 保证（相对误差 1e-9）：
            realistic_taker_both_pnl <= realistic_pnl <= slip_pnl <= raw_pnl
        """
        notional = trade.notional

        # slip_pnl = raw_pnl − (slip_bps_per_side × 2) × notional / 10000
        slip_cost = (params.slip_bps_per_side * 2.0) * notional / 10000.0
        slip_pnl = trade.raw_pnl - slip_cost

        # realistic_pnl = slip_pnl − (entry_bps + exit_bps) × notional / 10000
        fee_cost = (params.entry_bps + params.exit_bps) * notional / 10000.0
        realistic_pnl = slip_pnl - fee_cost

        # realistic_taker_both_pnl = slip_pnl − (taker_entry_bps + taker_exit_bps) × notional / 10000
        # 固定使用 taker_entry=4, taker_exit=4
        taker_both_cost = (4.0 + 4.0) * notional / 10000.0
        realistic_taker_both_pnl = slip_pnl - taker_both_cost

        return PricedTrade(
            raw_pnl=trade.raw_pnl,
            notional=trade.notional,
            entry_price=trade.entry_price,
            exit_price=trade.exit_price,
            symbol=trade.symbol,
            side=trade.side,
            slip_pnl=slip_pnl,
            realistic_pnl=realistic_pnl,
            realistic_taker_both_pnl=realistic_taker_both_pnl,
        )
