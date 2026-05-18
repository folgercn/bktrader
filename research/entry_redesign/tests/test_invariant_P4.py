"""Hypothesis property-based 测试：Cost Model Monotonicity (P4).

**Validates: Requirements 6.4**

FOR ALL trade：
    realistic_taker_both_pnl <= realistic_pnl <= slip_pnl <= raw_pnl
    （相对误差 1e-9 内成立）

使用 hypothesis 生成随机 RawTrade 实例（random raw_pnl float, positive notional,
random entry/exit prices, symbol from ["BTCUSDT","ETHUSDT"],
side from ["long","short"]）。

对每个 RawTrade 分别使用 BASELINE_COST_PARAMS 和 STRESS_COST_PARAMS 调用
CostModelApplier.apply()，验证单调性不变量在 1e-9 相对容差内成立。

Requirements: 6.4, 6.14
"""

from __future__ import annotations

from hypothesis import given, settings
from hypothesis import strategies as st

from research.entry_redesign.cost.cost_model_applier import (
    BASELINE_COST_PARAMS,
    STRESS_COST_PARAMS,
    CostModelApplier,
    CostModelParams,
    RawTrade,
)

# ---------------------------------------------------------------------------
# Hypothesis strategy: 生成随机 RawTrade 实例
# ---------------------------------------------------------------------------

_SYMBOLS = ["BTCUSDT", "ETHUSDT"]
_SIDES = ["long", "short"]


@st.composite
def raw_trades(draw: st.DrawFn) -> RawTrade:
    """Generate a random RawTrade instance.

    - raw_pnl: arbitrary finite float (can be negative, zero, or positive)
    - notional: positive finite float (> 0, as notional must be positive)
    - entry_price / exit_price: positive finite floats
    - symbol: one of BTCUSDT, ETHUSDT
    - side: one of long, short
    """
    raw_pnl = draw(
        st.floats(
            min_value=-1e12,
            max_value=1e12,
            allow_nan=False,
            allow_infinity=False,
        )
    )
    notional = draw(
        st.floats(
            min_value=1e-6,
            max_value=1e12,
            allow_nan=False,
            allow_infinity=False,
        )
    )
    entry_price = draw(
        st.floats(
            min_value=1e-6,
            max_value=1e9,
            allow_nan=False,
            allow_infinity=False,
        )
    )
    exit_price = draw(
        st.floats(
            min_value=1e-6,
            max_value=1e9,
            allow_nan=False,
            allow_infinity=False,
        )
    )
    symbol = draw(st.sampled_from(_SYMBOLS))
    side = draw(st.sampled_from(_SIDES))
    return RawTrade(
        raw_pnl=raw_pnl,
        notional=notional,
        entry_price=entry_price,
        exit_price=exit_price,
        symbol=symbol,
        side=side,
    )


# ---------------------------------------------------------------------------
# Helper: 相对误差比较（tolerant <=）
# ---------------------------------------------------------------------------


def _le_within_rel_tol(a: float, b: float, rel_tol: float = 1e-9) -> bool:
    """Check a <= b within relative tolerance.

    Returns True if a <= b + rel_tol * max(|a|, |b|, 1.0).
    This handles the case where floating point arithmetic may produce
    tiny violations of the strict inequality.
    """
    scale = max(abs(a), abs(b), 1.0)
    return a <= b + rel_tol * scale


# ---------------------------------------------------------------------------
# Property-based tests
# ---------------------------------------------------------------------------

_applier = CostModelApplier()


@given(trade=raw_trades())
@settings(max_examples=1000)
def test_p4_monotonicity_baseline(trade: RawTrade) -> None:
    """P4: Cost Model Monotonicity with BASELINE_COST_PARAMS.

    **Validates: Requirements 6.4**

    FOR ALL trade:
        realistic_taker_both_pnl <= realistic_pnl <= slip_pnl <= raw_pnl
        (relative tolerance 1e-9)
    """
    priced = _applier.apply(trade, BASELINE_COST_PARAMS)

    # realistic_taker_both_pnl <= realistic_pnl
    assert _le_within_rel_tol(
        priced.realistic_taker_both_pnl, priced.realistic_pnl
    ), (
        f"P4 violated: realistic_taker_both_pnl ({priced.realistic_taker_both_pnl}) "
        f"> realistic_pnl ({priced.realistic_pnl}) "
        f"for trade: raw_pnl={trade.raw_pnl}, notional={trade.notional}, "
        f"symbol={trade.symbol}, side={trade.side}, params=BASELINE"
    )

    # realistic_pnl <= slip_pnl
    assert _le_within_rel_tol(priced.realistic_pnl, priced.slip_pnl), (
        f"P4 violated: realistic_pnl ({priced.realistic_pnl}) "
        f"> slip_pnl ({priced.slip_pnl}) "
        f"for trade: raw_pnl={trade.raw_pnl}, notional={trade.notional}, "
        f"symbol={trade.symbol}, side={trade.side}, params=BASELINE"
    )

    # slip_pnl <= raw_pnl
    assert _le_within_rel_tol(priced.slip_pnl, priced.raw_pnl), (
        f"P4 violated: slip_pnl ({priced.slip_pnl}) "
        f"> raw_pnl ({priced.raw_pnl}) "
        f"for trade: raw_pnl={trade.raw_pnl}, notional={trade.notional}, "
        f"symbol={trade.symbol}, side={trade.side}, params=BASELINE"
    )


@given(trade=raw_trades())
@settings(max_examples=1000)
def test_p4_monotonicity_stress(trade: RawTrade) -> None:
    """P4: Cost Model Monotonicity with STRESS_COST_PARAMS.

    **Validates: Requirements 6.4**

    FOR ALL trade:
        realistic_taker_both_pnl <= realistic_pnl <= slip_pnl <= raw_pnl
        (relative tolerance 1e-9)
    """
    priced = _applier.apply(trade, STRESS_COST_PARAMS)

    # realistic_taker_both_pnl <= realistic_pnl
    assert _le_within_rel_tol(
        priced.realistic_taker_both_pnl, priced.realistic_pnl
    ), (
        f"P4 violated: realistic_taker_both_pnl ({priced.realistic_taker_both_pnl}) "
        f"> realistic_pnl ({priced.realistic_pnl}) "
        f"for trade: raw_pnl={trade.raw_pnl}, notional={trade.notional}, "
        f"symbol={trade.symbol}, side={trade.side}, params=STRESS"
    )

    # realistic_pnl <= slip_pnl
    assert _le_within_rel_tol(priced.realistic_pnl, priced.slip_pnl), (
        f"P4 violated: realistic_pnl ({priced.realistic_pnl}) "
        f"> slip_pnl ({priced.slip_pnl}) "
        f"for trade: raw_pnl={trade.raw_pnl}, notional={trade.notional}, "
        f"symbol={trade.symbol}, side={trade.side}, params=STRESS"
    )

    # slip_pnl <= raw_pnl
    assert _le_within_rel_tol(priced.slip_pnl, priced.raw_pnl), (
        f"P4 violated: slip_pnl ({priced.slip_pnl}) "
        f"> raw_pnl ({priced.raw_pnl}) "
        f"for trade: raw_pnl={trade.raw_pnl}, notional={trade.notional}, "
        f"symbol={trade.symbol}, side={trade.side}, params=STRESS"
    )
