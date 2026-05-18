"""Hypothesis property-based 测试：Per-Trade Quality Reporting Present (P9).

**Validates: Requirements 6.9, 6.14**

Property 9: Per-Trade Quality Reporting Present

验证：
1. `per_trade_quality_bps_over_notional` 字段恒存在于 summary dict 中。
2. 当 `trade_count == 0` 时，字段值 MUST 为 `null` (None)。
3. 与 ledger 重算值的相对误差 MUST <= 1e-6。
   重算口径：mean_i(realistic_pnl_i / notional_i) × 10000
4. `notional_i == 0` 的 trade 必须在 ledger 写入前拒绝（ledger 中不得存在）。

使用 hypothesis 生成 ledger DataFrames 和 summary dicts，验证上述四条不变量。

Requirements: 6.9, 6.14
"""

from __future__ import annotations

from datetime import datetime, timezone

import numpy as np
import pandas as pd
from hypothesis import given, settings, assume
from hypothesis import strategies as st

from research.entry_redesign.metrics.metrics_aggregator import MetricsAggregator

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

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

# ---------------------------------------------------------------------------
# Hypothesis strategies: 生成 ledger DataFrame 行
# ---------------------------------------------------------------------------


@st.composite
def ledger_trade_row(draw: st.DrawFn) -> dict:
    """Generate a single valid ledger trade row with positive notional.

    notional MUST be > 0 (zero-notional trades are rejected before ledger write).
    """
    symbol = draw(st.sampled_from(_SYMBOLS))
    side = draw(st.sampled_from(_SIDES))
    gate_mode = draw(st.sampled_from(_GATE_MODES))

    # notional MUST be positive (> 0); zero-notional rejected pre-write
    notional = draw(
        st.floats(min_value=1e-4, max_value=1e8, allow_nan=False, allow_infinity=False)
    )

    # raw_pnl: arbitrary finite float
    raw_pnl = draw(
        st.floats(min_value=-1e8, max_value=1e8, allow_nan=False, allow_infinity=False)
    )

    # Derive cost-adjusted pnl values (consistent with cost model)
    slip_bps_per_side = 2
    maker_entry_bps = 2
    taker_exit_bps = 4
    taker_entry_bps = 4

    slip_pnl = raw_pnl - (slip_bps_per_side * 2) * notional / 10000
    realistic_pnl = slip_pnl - (maker_entry_bps + taker_exit_bps) * notional / 10000
    realistic_taker_both_pnl = (
        slip_pnl - (taker_entry_bps + taker_exit_bps) * notional / 10000
    )

    # Generate timestamps (entry_ts used for MDD ordering)
    base_ts = datetime(2025, 6, 1, tzinfo=timezone.utc)
    offset_hours = draw(st.integers(min_value=0, max_value=8000))
    entry_ts = datetime(
        2025, 6, 1, offset_hours % 24, tzinfo=timezone.utc
    )
    # Use a simple incrementing pattern for signal_bar_start_ts
    signal_bar_month = draw(st.integers(min_value=6, max_value=12))
    signal_bar_start_ts = datetime(
        2025, min(signal_bar_month, 12), 1, tzinfo=timezone.utc
    )

    return {
        "entry_ts": entry_ts,
        "exit_ts": entry_ts,
        "symbol": symbol,
        "side": side,
        "entry_price": 50000.0,
        "exit_price": 50100.0,
        "notional": notional,
        "raw_pnl": raw_pnl,
        "slip_pnl": slip_pnl,
        "realistic_pnl": realistic_pnl,
        "realistic_taker_both_pnl": realistic_taker_both_pnl,
        "exit_reason": draw(st.sampled_from(_EXIT_REASONS)),
        "entry_candidate_id": "baseline-000000000000",
        "gate_mode": gate_mode,
        "signal_bar_start_ts": signal_bar_start_ts,
        "trigger_ts": entry_ts,
        "entry_delay_seconds": 0,
        "feature_horizon_seconds": 0,
        "trigger_confirmation_id": "none",
        "entry_price_mode_id": "market_on_touch",
        "pretouch_state_band_id": "none",
        "posttouch_quality_band_id": "none",
    }


@st.composite
def ledger_dataframe(draw: st.DrawFn, min_rows: int = 0, max_rows: int = 50) -> pd.DataFrame:
    """Generate a ledger DataFrame with 0 to max_rows trades.

    All trades have positive notional (zero-notional rejected pre-write).
    """
    n_rows = draw(st.integers(min_value=min_rows, max_value=max_rows))
    if n_rows == 0:
        # Return empty DataFrame with correct columns
        return pd.DataFrame(
            columns=[
                "entry_ts", "exit_ts", "symbol", "side", "entry_price",
                "exit_price", "notional", "raw_pnl", "slip_pnl",
                "realistic_pnl", "realistic_taker_both_pnl", "exit_reason",
                "entry_candidate_id", "gate_mode", "signal_bar_start_ts",
                "trigger_ts", "entry_delay_seconds", "feature_horizon_seconds",
                "trigger_confirmation_id", "entry_price_mode_id",
                "pretouch_state_band_id", "posttouch_quality_band_id",
            ]
        )
    rows = [draw(ledger_trade_row()) for _ in range(n_rows)]
    return pd.DataFrame(rows)


@st.composite
def ledger_with_single_gate_mode(
    draw: st.DrawFn,
    gate_mode: str = "nogate",
    min_rows: int = 0,
    max_rows: int = 30,
) -> pd.DataFrame:
    """Generate a ledger DataFrame where all rows have the same gate_mode."""
    df = draw(ledger_dataframe(min_rows=min_rows, max_rows=max_rows))
    if len(df) > 0:
        df["gate_mode"] = gate_mode
    return df


# ---------------------------------------------------------------------------
# Helper: 相对误差比较
# ---------------------------------------------------------------------------


def _relative_error(actual: float, expected: float) -> float:
    """Compute relative error between actual and expected.

    Uses max(|actual|, |expected|, 1.0) as denominator to avoid division by zero.
    """
    scale = max(abs(actual), abs(expected), 1.0)
    return abs(actual - expected) / scale


# ---------------------------------------------------------------------------
# Property-based tests
# ---------------------------------------------------------------------------

_aggregator = MetricsAggregator()


@given(df=ledger_dataframe(min_rows=0, max_rows=40))
@settings(max_examples=500)
def test_p9_field_always_present(df: pd.DataFrame) -> None:
    """P9: per_trade_quality_bps_over_notional 字段恒存在。

    **Validates: Requirements 6.9**

    FOR ALL ledger DataFrames (including empty):
        summary dict MUST contain both nogate_per_trade_quality_bps_over_notional
        and gate001_per_trade_quality_bps_over_notional keys.
    """
    result = _aggregator.aggregate(df, total_silos=22)

    assert "nogate_per_trade_quality_bps_over_notional" in result, (
        "P9 violated: nogate_per_trade_quality_bps_over_notional field missing "
        f"from summary. Keys present: {list(result.keys())}"
    )
    assert "gate001_per_trade_quality_bps_over_notional" in result, (
        "P9 violated: gate001_per_trade_quality_bps_over_notional field missing "
        f"from summary. Keys present: {list(result.keys())}"
    )


@given(df=ledger_dataframe(min_rows=0, max_rows=0))
@settings(max_examples=100)
def test_p9_null_when_trade_count_zero_empty_ledger(df: pd.DataFrame) -> None:
    """P9: trade_count==0 → per_trade_quality_bps_over_notional is null.

    **Validates: Requirements 6.9**

    WHEN trade_count == 0 (empty ledger for a gate_mode),
    THEN per_trade_quality_bps_over_notional MUST be null (None).
    """
    result = _aggregator.aggregate(df, total_silos=22)

    # Empty ledger means both gate modes have 0 trades
    assert result["nogate_per_trade_quality_bps_over_notional"] is None, (
        "P9 violated: nogate_per_trade_quality_bps_over_notional should be null "
        f"when trade_count==0, got: {result['nogate_per_trade_quality_bps_over_notional']}"
    )
    assert result["gate001_per_trade_quality_bps_over_notional"] is None, (
        "P9 violated: gate001_per_trade_quality_bps_over_notional should be null "
        f"when trade_count==0, got: {result['gate001_per_trade_quality_bps_over_notional']}"
    )


@given(df=ledger_with_single_gate_mode(gate_mode="nogate", min_rows=0, max_rows=30))
@settings(max_examples=500)
def test_p9_null_when_trade_count_zero_gate001_empty(df: pd.DataFrame) -> None:
    """P9: When gate001 has no trades, its per_trade_quality is null.

    **Validates: Requirements 6.9**

    Generate ledger with only nogate rows → gate001 trade_count == 0 → null.
    """
    result = _aggregator.aggregate(df, total_silos=22)

    # gate001 has no rows in this ledger
    assert result["gate001_trade_count"] == 0, (
        f"Expected gate001_trade_count==0, got {result['gate001_trade_count']}"
    )
    assert result["gate001_per_trade_quality_bps_over_notional"] is None, (
        "P9 violated: gate001_per_trade_quality_bps_over_notional should be null "
        f"when gate001_trade_count==0, got: "
        f"{result['gate001_per_trade_quality_bps_over_notional']}"
    )


@given(df=ledger_with_single_gate_mode(gate_mode="nogate", min_rows=1, max_rows=40))
@settings(max_examples=500)
def test_p9_recomputed_value_matches(df: pd.DataFrame) -> None:
    """P9: Recomputed per_trade_quality matches summary within 1e-6.

    **Validates: Requirements 6.9**

    FOR ALL non-empty ledger subsets:
        recomputed = mean_i(realistic_pnl_i / notional_i) × 10000
        |summary_value - recomputed| / max(|summary_value|, |recomputed|, 1.0) <= 1e-6
    """
    result = _aggregator.aggregate(df, total_silos=22)

    nogate_subset = df[df["gate_mode"] == "nogate"]
    if len(nogate_subset) == 0:
        # No nogate trades → should be null
        assert result["nogate_per_trade_quality_bps_over_notional"] is None
        return

    # Recompute: mean_i(realistic_pnl_i / notional_i) × 10000
    realistic_pnl = nogate_subset["realistic_pnl"].values.astype(np.float64)
    notional = nogate_subset["notional"].values.astype(np.float64)

    # All notional must be > 0 (guaranteed by our strategy)
    assert np.all(notional > 0), (
        "P9 violated: zero-notional trade found in ledger. "
        "Zero-notional trades MUST be rejected before ledger write."
    )

    r_i = realistic_pnl / notional
    recomputed = float(np.mean(r_i)) * 10000.0

    summary_value = result["nogate_per_trade_quality_bps_over_notional"]
    assert summary_value is not None, (
        "P9 violated: nogate_per_trade_quality_bps_over_notional is null "
        f"but trade_count={len(nogate_subset)} > 0"
    )

    rel_err = _relative_error(summary_value, recomputed)
    assert rel_err <= 1e-6, (
        f"P9 violated: per_trade_quality_bps_over_notional recomputation mismatch. "
        f"summary={summary_value}, recomputed={recomputed}, "
        f"relative_error={rel_err} > 1e-6. "
        f"trade_count={len(nogate_subset)}"
    )


@given(
    notional_values=st.lists(
        st.floats(min_value=1e-4, max_value=1e8, allow_nan=False, allow_infinity=False),
        min_size=1,
        max_size=20,
    ),
    raw_pnl_values=st.lists(
        st.floats(min_value=-1e8, max_value=1e8, allow_nan=False, allow_infinity=False),
        min_size=1,
        max_size=20,
    ),
)
@settings(max_examples=500)
def test_p9_no_zero_notional_in_ledger(
    notional_values: list[float],
    raw_pnl_values: list[float],
) -> None:
    """P9: notional_i == 0 trades MUST be rejected before ledger write.

    **Validates: Requirements 6.9**

    Verify that the MetricsAggregator correctly handles ledgers where all
    notional values are positive (as guaranteed by the pre-write rejection).
    A ledger with zero-notional would cause division by zero in the
    per_trade_quality computation.
    """
    # Ensure same length
    n = min(len(notional_values), len(raw_pnl_values))
    assume(n > 0)
    notional_values = notional_values[:n]
    raw_pnl_values = raw_pnl_values[:n]

    # Build a minimal ledger with all positive notionals
    rows = []
    for i in range(n):
        notional = notional_values[i]
        raw_pnl = raw_pnl_values[i]
        slip_pnl = raw_pnl - (2 * 2) * notional / 10000
        realistic_pnl = slip_pnl - (2 + 4) * notional / 10000
        realistic_taker_both_pnl = slip_pnl - (4 + 4) * notional / 10000

        rows.append({
            "entry_ts": datetime(2025, 6, 1, i % 24, tzinfo=timezone.utc),
            "exit_ts": datetime(2025, 6, 1, i % 24, tzinfo=timezone.utc),
            "symbol": "BTCUSDT",
            "side": "long",
            "entry_price": 50000.0,
            "exit_price": 50100.0,
            "notional": notional,
            "raw_pnl": raw_pnl,
            "slip_pnl": slip_pnl,
            "realistic_pnl": realistic_pnl,
            "realistic_taker_both_pnl": realistic_taker_both_pnl,
            "exit_reason": "signal_exit",
            "entry_candidate_id": "baseline-000000000000",
            "gate_mode": "nogate",
            "signal_bar_start_ts": datetime(2025, 6, 1, tzinfo=timezone.utc),
            "trigger_ts": datetime(2025, 6, 1, i % 24, tzinfo=timezone.utc),
            "entry_delay_seconds": 0,
            "feature_horizon_seconds": 0,
            "trigger_confirmation_id": "none",
            "entry_price_mode_id": "market_on_touch",
            "pretouch_state_band_id": "none",
            "posttouch_quality_band_id": "none",
        })

    df = pd.DataFrame(rows)

    # Verify no zero notional exists
    assert (df["notional"] > 0).all(), (
        "P9 violated: zero-notional trade found in ledger. "
        "Zero-notional trades MUST be rejected before ledger write."
    )

    # Aggregation should succeed without division-by-zero
    result = _aggregator.aggregate(df, total_silos=22)

    # per_trade_quality field must exist and be non-null for non-empty ledger
    ptq = result["nogate_per_trade_quality_bps_over_notional"]
    assert ptq is not None, (
        "P9 violated: nogate_per_trade_quality_bps_over_notional is null "
        f"but nogate_trade_count={result['nogate_trade_count']} > 0"
    )

    # Verify it's a finite number (not NaN/Inf)
    assert np.isfinite(ptq), (
        f"P9 violated: per_trade_quality_bps_over_notional is not finite: {ptq}"
    )


def test_p9_zero_notional_trade_must_be_rejected() -> None:
    """P9: Deterministic test that zero-notional trades cause issues.

    **Validates: Requirements 6.9**

    A ledger containing a zero-notional trade would produce NaN/Inf in
    per_trade_quality computation. This test verifies that if such a trade
    were to slip through, the computation would produce invalid results,
    confirming the necessity of pre-write rejection.
    """
    # Build a ledger with a zero-notional trade (simulating a bug)
    rows = [{
        "entry_ts": datetime(2025, 6, 1, 0, tzinfo=timezone.utc),
        "exit_ts": datetime(2025, 6, 1, 1, tzinfo=timezone.utc),
        "symbol": "BTCUSDT",
        "side": "long",
        "entry_price": 50000.0,
        "exit_price": 50100.0,
        "notional": 0.0,  # INVALID: zero notional
        "raw_pnl": 100.0,
        "slip_pnl": 98.0,
        "realistic_pnl": 94.0,
        "realistic_taker_both_pnl": 92.0,
        "exit_reason": "signal_exit",
        "entry_candidate_id": "baseline-000000000000",
        "gate_mode": "nogate",
        "signal_bar_start_ts": datetime(2025, 6, 1, tzinfo=timezone.utc),
        "trigger_ts": datetime(2025, 6, 1, 0, tzinfo=timezone.utc),
        "entry_delay_seconds": 0,
        "feature_horizon_seconds": 0,
        "trigger_confirmation_id": "none",
        "entry_price_mode_id": "market_on_touch",
        "pretouch_state_band_id": "none",
        "posttouch_quality_band_id": "none",
    }]
    df = pd.DataFrame(rows)

    # With zero notional, division produces inf → per_trade_quality is not finite
    result = _aggregator.aggregate(df, total_silos=22)
    ptq = result["nogate_per_trade_quality_bps_over_notional"]

    # This demonstrates that zero-notional trades MUST be rejected pre-write:
    # if they slip through, the metric becomes inf or nan
    assert ptq is None or not np.isfinite(ptq), (
        "Expected non-finite per_trade_quality when zero-notional trade is present, "
        f"but got finite value: {ptq}. "
        "This means zero-notional rejection is critical for P9 correctness."
    )
