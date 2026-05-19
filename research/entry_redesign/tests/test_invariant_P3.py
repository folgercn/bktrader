"""Hypothesis property-based 测试：Trades Per Bar Bound (P3).

**Validates: Requirements 6.3, 6.14**

FOR ALL signal_bar：同 signal bar 内 real-entry count ≤ max_trades_per_bar=2
（AGENTS Research_Baseline: max_trades_per_bar=2）。

使用 hypothesis 生成 ledger DataFrames，包含不同数量的 trades per signal bar。
验证 InvariantChecker.check() 在 count > 2 时正确检测 P3 违反，
在 count <= 2 时报告 P3_count == 0。

Requirements: 6.3, 6.14
"""

from __future__ import annotations

from datetime import datetime, timezone

import pandas as pd
from hypothesis import given, settings
from hypothesis import strategies as st

from research.entry_redesign.invariants.invariant_checker import InvariantChecker

# ---------------------------------------------------------------------------
# 常量
# ---------------------------------------------------------------------------

_MAX_TRADES_PER_BAR: int = 2
_SYMBOLS = ["BTCUSDT", "ETHUSDT"]
_SIDES = ["long", "short"]
_GATE_MODES = ["nogate", "gate001"]
_EXIT_REASONS = [
    "signal_exit",
    "initial_stop",
    "breakeven_stop",
    "trail_stop",
    "max_hold_timeout",
]

# 固定的 signal_bar_start_ts 池（用于生成多个 signal bar）
_BASE_TS = datetime(2025, 6, 1, 0, 0, 0, tzinfo=timezone.utc)


# ---------------------------------------------------------------------------
# Hypothesis strategies
# ---------------------------------------------------------------------------


@st.composite
def signal_bar_ts(draw: st.DrawFn) -> str:
    """Generate a signal_bar_start_ts as ISO-8601 UTC ms string.

    从固定的 hour offset 池中选取，模拟不同的 signal bar。
    """
    hour_offset = draw(st.integers(min_value=0, max_value=200))
    ts = datetime(
        2025, 6, 1, hour_offset % 24, 0, 0, tzinfo=timezone.utc
    ).replace(day=1 + hour_offset // 24 % 28)
    return ts.strftime("%Y-%m-%dT%H:%M:%S.000Z")


@st.composite
def ledger_within_bound(draw: st.DrawFn) -> pd.DataFrame:
    """Generate a ledger DataFrame where each (signal_bar_start_ts, symbol, gate_mode)
    group has at most max_trades_per_bar=2 trades.

    This represents a VALID ledger that should NOT trigger P3 violations.
    """
    # 生成 1-5 个不同的 signal bar
    n_bars = draw(st.integers(min_value=1, max_value=5))
    rows = []

    for bar_idx in range(n_bars):
        bar_ts = datetime(
            2025, 6, 1 + bar_idx % 28, bar_idx % 24, 0, 0, tzinfo=timezone.utc
        ).strftime("%Y-%m-%dT%H:%M:%S.000Z")

        symbol = draw(st.sampled_from(_SYMBOLS))
        gate_mode = draw(st.sampled_from(_GATE_MODES))

        # 每个 (bar, symbol, gate_mode) 组合最多 2 笔 trade
        n_trades = draw(st.integers(min_value=1, max_value=_MAX_TRADES_PER_BAR))

        for trade_idx in range(n_trades):
            rows.append(_make_trade_row(bar_ts, symbol, gate_mode, trade_idx))

    if not rows:
        return pd.DataFrame(columns=_LEDGER_COLUMNS)

    return pd.DataFrame(rows, columns=_LEDGER_COLUMNS)


@st.composite
def ledger_exceeding_bound(draw: st.DrawFn) -> pd.DataFrame:
    """Generate a ledger DataFrame where at least one (signal_bar_start_ts, symbol,
    gate_mode) group has MORE than max_trades_per_bar=2 trades.

    This represents an INVALID ledger that MUST trigger P3 violations.
    """
    rows = []

    # 生成一个违反 P3 的 signal bar 组合
    violating_bar_ts = datetime(
        2025, 7, 15, 10, 0, 0, tzinfo=timezone.utc
    ).strftime("%Y-%m-%dT%H:%M:%S.000Z")
    violating_symbol = draw(st.sampled_from(_SYMBOLS))
    violating_gate_mode = draw(st.sampled_from(_GATE_MODES))

    # 超过 max_trades_per_bar 的 trade 数量 (3-6)
    n_violating_trades = draw(st.integers(min_value=3, max_value=6))

    for trade_idx in range(n_violating_trades):
        rows.append(
            _make_trade_row(
                violating_bar_ts, violating_symbol, violating_gate_mode, trade_idx
            )
        )

    # 可选：添加一些合规的 signal bar（0-3 个）
    n_valid_bars = draw(st.integers(min_value=0, max_value=3))
    for bar_idx in range(n_valid_bars):
        bar_ts = datetime(
            2025, 8, 1 + bar_idx, 12, 0, 0, tzinfo=timezone.utc
        ).strftime("%Y-%m-%dT%H:%M:%S.000Z")
        symbol = draw(st.sampled_from(_SYMBOLS))
        gate_mode = draw(st.sampled_from(_GATE_MODES))
        n_trades = draw(st.integers(min_value=1, max_value=_MAX_TRADES_PER_BAR))
        for trade_idx in range(n_trades):
            rows.append(_make_trade_row(bar_ts, symbol, gate_mode, trade_idx))

    return pd.DataFrame(rows, columns=_LEDGER_COLUMNS)


@st.composite
def ledger_with_known_violation_count(
    draw: st.DrawFn,
) -> tuple[pd.DataFrame, int]:
    """Generate a ledger DataFrame with a known number of P3 violations.

    Returns (ledger_df, expected_violation_count).
    """
    rows = []
    expected_violations = 0

    # 生成 1-4 个 signal bar 组合
    n_groups = draw(st.integers(min_value=1, max_value=4))

    for group_idx in range(n_groups):
        bar_ts = datetime(
            2025, 6, 1 + group_idx % 28, group_idx * 2 % 24, 0, 0,
            tzinfo=timezone.utc,
        ).strftime("%Y-%m-%dT%H:%M:%S.000Z")
        symbol = draw(st.sampled_from(_SYMBOLS))
        gate_mode = draw(st.sampled_from(_GATE_MODES))

        # 随机决定该组是否违反 P3
        violates = draw(st.booleans())
        if violates:
            n_trades = draw(st.integers(min_value=3, max_value=5))
            expected_violations += 1
        else:
            n_trades = draw(st.integers(min_value=1, max_value=_MAX_TRADES_PER_BAR))

        for trade_idx in range(n_trades):
            rows.append(_make_trade_row(bar_ts, symbol, gate_mode, trade_idx))

    if not rows:
        return pd.DataFrame(columns=_LEDGER_COLUMNS), 0

    return pd.DataFrame(rows, columns=_LEDGER_COLUMNS), expected_violations


# ---------------------------------------------------------------------------
# Helper: 构造 trade 行
# ---------------------------------------------------------------------------

_LEDGER_COLUMNS = [
    "entry_ts",
    "exit_ts",
    "symbol",
    "side",
    "entry_price",
    "exit_price",
    "notional",
    "raw_pnl",
    "slip_pnl",
    "realistic_pnl",
    "realistic_taker_both_pnl",
    "exit_reason",
    "entry_candidate_id",
    "gate_mode",
    "signal_bar_start_ts",
    "trigger_ts",
    "entry_delay_seconds",
    "feature_horizon_seconds",
    "trigger_confirmation_id",
    "entry_price_mode_id",
    "pretouch_state_band_id",
    "posttouch_quality_band_id",
]


def _make_trade_row(
    signal_bar_start_ts: str,
    symbol: str,
    gate_mode: str,
    trade_idx: int,
) -> dict:
    """Create a minimal valid trade row for P3 testing."""
    # Use trade_idx to offset entry_ts for uniqueness
    entry_ts = signal_bar_start_ts.replace(
        ":00:00.000Z", f":{trade_idx:02d}:00.000Z"
    )
    exit_ts = signal_bar_start_ts.replace(
        ":00:00.000Z", f":{trade_idx:02d}:30.000Z"
    )
    trigger_ts = signal_bar_start_ts

    return {
        "entry_ts": entry_ts,
        "exit_ts": exit_ts,
        "symbol": symbol,
        "side": "long",
        "entry_price": 50000.0,
        "exit_price": 50100.0,
        "notional": 10000.0,
        "raw_pnl": 20.0,
        "slip_pnl": 16.0,
        "realistic_pnl": 10.0,
        "realistic_taker_both_pnl": 8.0,
        "exit_reason": "signal_exit",
        "entry_candidate_id": "d0_h0_none_market_on_touch_none_none-abcdef012345",
        "gate_mode": gate_mode,
        "signal_bar_start_ts": signal_bar_start_ts,
        "trigger_ts": trigger_ts,
        "entry_delay_seconds": 0,
        "feature_horizon_seconds": 0,
        "trigger_confirmation_id": "none",
        "entry_price_mode_id": "market_on_touch",
        "pretouch_state_band_id": "none",
        "posttouch_quality_band_id": "none",
    }


# ---------------------------------------------------------------------------
# Property-based tests
# ---------------------------------------------------------------------------

_checker = InvariantChecker()

# 最小 summary dict（P3 检查不依赖 summary 内容）
_MINIMAL_SUMMARY: dict = {
    "nogate_per_trade_quality_bps_over_notional": 15.0,
    "nogate_trade_count": 10,
    "gate001_per_trade_quality_bps_over_notional": 12.0,
    "gate001_trade_count": 8,
    "nogate_active_silo_sum_pct": 1.0,
    "nogate_calendar_normalized_return_pct": 0.5,
    "nogate_active_months": 11,
    "nogate_empty_months": 11,
    "gate001_active_silo_sum_pct": 0.8,
    "gate001_calendar_normalized_return_pct": 0.3,
    "gate001_active_months": 10,
    "gate001_empty_months": 12,
    "live_output_emitted": False,
}


@given(ledger=ledger_within_bound())
@settings(max_examples=500)
def test_p3_no_violation_when_within_bound(ledger: pd.DataFrame) -> None:
    """P3: No violation when trades per bar <= max_trades_per_bar=2.

    **Validates: Requirements 6.3**

    FOR ALL signal_bar：同 signal bar 内 real-entry count ≤ max_trades_per_bar=2
    → InvariantChecker.check() 报告 P3_count == 0。
    """
    violations = _checker.check(ledger, _MINIMAL_SUMMARY)
    assert violations["P3_count"] == 0, (
        f"P3 false positive: P3_count={violations['P3_count']} "
        f"but all groups have <= {_MAX_TRADES_PER_BAR} trades. "
        f"Ledger shape: {ledger.shape}"
    )


@given(ledger=ledger_exceeding_bound())
@settings(max_examples=500)
def test_p3_violation_when_exceeding_bound(ledger: pd.DataFrame) -> None:
    """P3: Violation detected when trades per bar > max_trades_per_bar=2.

    **Validates: Requirements 6.3**

    FOR ALL signal_bar with real-entry count > max_trades_per_bar=2
    → InvariantChecker.check() 报告 P3_count > 0。
    """
    violations = _checker.check(ledger, _MINIMAL_SUMMARY)
    assert violations["P3_count"] > 0, (
        f"P3 false negative: P3_count=0 but ledger contains groups "
        f"with > {_MAX_TRADES_PER_BAR} trades. "
        f"Ledger shape: {ledger.shape}"
    )


@given(data=ledger_with_known_violation_count())
@settings(max_examples=500)
def test_p3_violation_count_matches_expected(
    data: tuple[pd.DataFrame, int],
) -> None:
    """P3: Violation count matches the number of groups exceeding bound.

    **Validates: Requirements 6.3**

    InvariantChecker.check() 的 P3_count 应等于超过 max_trades_per_bar=2
    的 (signal_bar_start_ts, symbol, gate_mode) 组合数。
    """
    ledger, expected_violations = data
    violations = _checker.check(ledger, _MINIMAL_SUMMARY)
    assert violations["P3_count"] == expected_violations, (
        f"P3 count mismatch: got {violations['P3_count']}, "
        f"expected {expected_violations}. "
        f"Ledger shape: {ledger.shape}"
    )


# ---------------------------------------------------------------------------
# 确定性边界测试（补充 property-based 测试）
# ---------------------------------------------------------------------------


def test_p3_empty_ledger() -> None:
    """P3: Empty ledger → P3_count == 0.

    **Validates: Requirements 6.3**
    """
    ledger = pd.DataFrame(columns=_LEDGER_COLUMNS)
    violations = _checker.check(ledger, _MINIMAL_SUMMARY)
    assert violations["P3_count"] == 0


def test_p3_exactly_at_bound() -> None:
    """P3: Exactly max_trades_per_bar=2 trades per group → P3_count == 0.

    **Validates: Requirements 6.3**
    """
    bar_ts = "2025-06-01T10:00:00.000Z"
    rows = [
        _make_trade_row(bar_ts, "BTCUSDT", "nogate", 0),
        _make_trade_row(bar_ts, "BTCUSDT", "nogate", 1),
    ]
    ledger = pd.DataFrame(rows, columns=_LEDGER_COLUMNS)
    violations = _checker.check(ledger, _MINIMAL_SUMMARY)
    assert violations["P3_count"] == 0


def test_p3_one_over_bound() -> None:
    """P3: Exactly max_trades_per_bar+1=3 trades in one group → P3_count == 1.

    **Validates: Requirements 6.3**
    """
    bar_ts = "2025-06-01T10:00:00.000Z"
    rows = [
        _make_trade_row(bar_ts, "ETHUSDT", "gate001", 0),
        _make_trade_row(bar_ts, "ETHUSDT", "gate001", 1),
        _make_trade_row(bar_ts, "ETHUSDT", "gate001", 2),
    ]
    ledger = pd.DataFrame(rows, columns=_LEDGER_COLUMNS)
    violations = _checker.check(ledger, _MINIMAL_SUMMARY)
    assert violations["P3_count"] == 1


def test_p3_different_symbols_same_bar_independent() -> None:
    """P3: Different symbols in same bar are counted independently.

    **Validates: Requirements 6.3**

    2 trades for BTCUSDT + 2 trades for ETHUSDT in same bar → P3_count == 0
    (each symbol group has <= 2).
    """
    bar_ts = "2025-06-01T10:00:00.000Z"
    rows = [
        _make_trade_row(bar_ts, "BTCUSDT", "nogate", 0),
        _make_trade_row(bar_ts, "BTCUSDT", "nogate", 1),
        _make_trade_row(bar_ts, "ETHUSDT", "nogate", 0),
        _make_trade_row(bar_ts, "ETHUSDT", "nogate", 1),
    ]
    ledger = pd.DataFrame(rows, columns=_LEDGER_COLUMNS)
    violations = _checker.check(ledger, _MINIMAL_SUMMARY)
    assert violations["P3_count"] == 0


def test_p3_different_gate_modes_same_bar_independent() -> None:
    """P3: Different gate_modes in same bar are counted independently.

    **Validates: Requirements 6.3**

    2 trades for nogate + 2 trades for gate001 in same bar/symbol → P3_count == 0.
    """
    bar_ts = "2025-06-01T10:00:00.000Z"
    rows = [
        _make_trade_row(bar_ts, "BTCUSDT", "nogate", 0),
        _make_trade_row(bar_ts, "BTCUSDT", "nogate", 1),
        _make_trade_row(bar_ts, "BTCUSDT", "gate001", 0),
        _make_trade_row(bar_ts, "BTCUSDT", "gate001", 1),
    ]
    ledger = pd.DataFrame(rows, columns=_LEDGER_COLUMNS)
    violations = _checker.check(ledger, _MINIMAL_SUMMARY)
    assert violations["P3_count"] == 0
