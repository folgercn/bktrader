"""Hypothesis property-based 测试：Metric Normalization Both Reported (P12).

**Validates: Requirements 6.12, 6.14**

Property 12: Metric Normalization Both Reported

验证：
1. `active_silo_sum_pct` 与 `calendar_normalized_return_pct` 同时存在于 summary 中。
2. `active_months + empty_months == 22`（分母恒为 22）。
3. 空仓 silo 以 `0.0` 参与 `calendar_normalized_return_pct` 加和。
4. InvariantChecker 在字段缺失或 active+empty != 22 时检测到 P12 违反。

使用 hypothesis 生成 summary dicts，验证 InvariantChecker 正确检测 P12 违反。

Requirements: 6.12, 6.14
"""

from __future__ import annotations

from datetime import datetime, timezone

import numpy as np
import pandas as pd
from hypothesis import given, settings, assume
from hypothesis import strategies as st

from research.entry_redesign.invariants.invariant_checker import InvariantChecker
from research.entry_redesign.metrics.metrics_aggregator import MetricsAggregator

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

_TOTAL_SILOS: int = 22
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
# Hypothesis strategies
# ---------------------------------------------------------------------------


@st.composite
def valid_summary_dict(draw: st.DrawFn) -> dict:
    """Generate a valid summary dict where P12 invariant holds.

    Both `active_silo_sum_pct` and `calendar_normalized_return_pct` are present
    for both gate modes, and `active_months + empty_months == 22`.
    """
    # Generate active_months in [0, 22], empty_months = 22 - active_months
    nogate_active = draw(st.integers(min_value=0, max_value=_TOTAL_SILOS))
    nogate_empty = _TOTAL_SILOS - nogate_active

    gate001_active = draw(st.integers(min_value=0, max_value=_TOTAL_SILOS))
    gate001_empty = _TOTAL_SILOS - gate001_active

    # Generate silo sum values
    nogate_active_silo_sum = draw(
        st.floats(min_value=-100.0, max_value=100.0, allow_nan=False, allow_infinity=False)
    )
    nogate_calendar_normalized = draw(
        st.floats(min_value=-100.0, max_value=100.0, allow_nan=False, allow_infinity=False)
    )
    gate001_active_silo_sum = draw(
        st.floats(min_value=-100.0, max_value=100.0, allow_nan=False, allow_infinity=False)
    )
    gate001_calendar_normalized = draw(
        st.floats(min_value=-100.0, max_value=100.0, allow_nan=False, allow_infinity=False)
    )

    return {
        "nogate_active_silo_sum_pct": nogate_active_silo_sum,
        "nogate_calendar_normalized_return_pct": nogate_calendar_normalized,
        "nogate_active_months": nogate_active,
        "nogate_empty_months": nogate_empty,
        "gate001_active_silo_sum_pct": gate001_active_silo_sum,
        "gate001_calendar_normalized_return_pct": gate001_calendar_normalized,
        "gate001_active_months": gate001_active,
        "gate001_empty_months": gate001_empty,
    }


@st.composite
def summary_missing_fields(draw: st.DrawFn) -> dict:
    """Generate a summary dict with at least one P12-required field missing.

    Randomly removes one or more of the four key fields
    (active_silo_sum_pct / calendar_normalized_return_pct for nogate/gate001).
    """
    base = draw(valid_summary_dict())

    # Fields that P12 requires to be present
    required_field_pairs = [
        ("nogate_active_silo_sum_pct", "nogate_calendar_normalized_return_pct"),
        ("gate001_active_silo_sum_pct", "gate001_calendar_normalized_return_pct"),
    ]

    # Choose which fields to remove (at least one)
    removable_fields = [
        "nogate_active_silo_sum_pct",
        "nogate_calendar_normalized_return_pct",
        "gate001_active_silo_sum_pct",
        "gate001_calendar_normalized_return_pct",
    ]

    # Remove at least 1 field
    num_to_remove = draw(st.integers(min_value=1, max_value=len(removable_fields)))
    fields_to_remove = draw(
        st.lists(
            st.sampled_from(removable_fields),
            min_size=num_to_remove,
            max_size=num_to_remove,
            unique=True,
        )
    )

    for field in fields_to_remove:
        base.pop(field, None)

    return base


@st.composite
def summary_bad_silo_count(draw: st.DrawFn) -> dict:
    """Generate a summary dict where active_months + empty_months != 22.

    All required fields are present, but the silo count invariant is violated.
    """
    base = draw(valid_summary_dict())

    # Choose which prefix to corrupt
    prefix = draw(st.sampled_from(["nogate", "gate001"]))

    # Generate active and empty that do NOT sum to 22
    active = draw(st.integers(min_value=0, max_value=30))
    empty = draw(st.integers(min_value=0, max_value=30))
    assume(active + empty != _TOTAL_SILOS)

    base[f"{prefix}_active_months"] = active
    base[f"{prefix}_empty_months"] = empty

    return base


@st.composite
def ledger_trade_row(draw: st.DrawFn) -> dict:
    """Generate a single valid ledger trade row with positive notional."""
    symbol = draw(st.sampled_from(_SYMBOLS))
    side = draw(st.sampled_from(_SIDES))
    gate_mode = draw(st.sampled_from(_GATE_MODES))

    notional = draw(
        st.floats(min_value=1e-4, max_value=1e8, allow_nan=False, allow_infinity=False)
    )
    raw_pnl = draw(
        st.floats(min_value=-1e8, max_value=1e8, allow_nan=False, allow_infinity=False)
    )

    # Derive cost-adjusted pnl values
    slip_pnl = raw_pnl - (2 * 2) * notional / 10000
    realistic_pnl = slip_pnl - (2 + 4) * notional / 10000
    realistic_taker_both_pnl = slip_pnl - (4 + 4) * notional / 10000

    # Generate execute month for silo assignment
    month = draw(st.integers(min_value=6, max_value=12))
    signal_bar_start_ts = datetime(2025, month, 1, tzinfo=timezone.utc)
    entry_ts = datetime(2025, month, 1, draw(st.integers(min_value=0, max_value=23)), tzinfo=timezone.utc)

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
def ledger_dataframe(draw: st.DrawFn, min_rows: int = 0, max_rows: int = 30) -> pd.DataFrame:
    """Generate a ledger DataFrame with 0 to max_rows trades."""
    n_rows = draw(st.integers(min_value=min_rows, max_value=max_rows))
    if n_rows == 0:
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


# ---------------------------------------------------------------------------
# Property-based tests
# ---------------------------------------------------------------------------

_checker = InvariantChecker()
_aggregator = MetricsAggregator()


@given(summary=valid_summary_dict())
@settings(max_examples=500)
def test_p12_valid_summary_no_violation(summary: dict) -> None:
    """P12: Valid summary with both fields present and correct silo count passes.

    **Validates: Requirements 6.12**

    FOR ALL valid summary dicts where active_silo_sum_pct and
    calendar_normalized_return_pct are both present, and
    active_months + empty_months == 22:
        InvariantChecker.check() MUST return P12_count == 0.
    """
    ledger = pd.DataFrame(
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

    violations = _checker.check(ledger, summary)

    assert violations["P12_count"] == 0, (
        f"P12 violated unexpectedly: P12_count={violations['P12_count']} "
        f"for valid summary. Summary keys: {list(summary.keys())}"
    )


@given(summary=summary_missing_fields())
@settings(max_examples=500)
def test_p12_detects_missing_fields(summary: dict) -> None:
    """P12: InvariantChecker detects when required fields are missing.

    **Validates: Requirements 6.12**

    WHEN active_silo_sum_pct or calendar_normalized_return_pct is missing
    from the summary for any gate mode:
        InvariantChecker.check() MUST return P12_count > 0.
    """
    ledger = pd.DataFrame(
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

    # Determine if any pair is actually missing
    nogate_ass_present = "nogate_active_silo_sum_pct" in summary
    nogate_cnr_present = "nogate_calendar_normalized_return_pct" in summary
    gate001_ass_present = "gate001_active_silo_sum_pct" in summary
    gate001_cnr_present = "gate001_calendar_normalized_return_pct" in summary

    nogate_pair_missing = not nogate_ass_present or not nogate_cnr_present
    gate001_pair_missing = not gate001_ass_present or not gate001_cnr_present

    # At least one pair must be incomplete for this test
    assume(nogate_pair_missing or gate001_pair_missing)

    violations = _checker.check(ledger, summary)

    assert violations["P12_count"] > 0, (
        f"P12 violation NOT detected when fields are missing. "
        f"Summary keys: {list(summary.keys())}. "
        f"nogate_pair_missing={nogate_pair_missing}, "
        f"gate001_pair_missing={gate001_pair_missing}"
    )


@given(summary=summary_bad_silo_count())
@settings(max_examples=500)
def test_p12_detects_silo_count_mismatch(summary: dict) -> None:
    """P12: InvariantChecker detects when active_months + empty_months != 22.

    **Validates: Requirements 6.12**

    WHEN active_months + empty_months != 22 for any gate mode:
        InvariantChecker.check() MUST return P12_count > 0.
    """
    ledger = pd.DataFrame(
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

    violations = _checker.check(ledger, summary)

    assert violations["P12_count"] > 0, (
        f"P12 violation NOT detected when silo count mismatch. "
        f"nogate: active={summary.get('nogate_active_months')}, "
        f"empty={summary.get('nogate_empty_months')}; "
        f"gate001: active={summary.get('gate001_active_months')}, "
        f"empty={summary.get('gate001_empty_months')}"
    )


@given(df=ledger_dataframe(min_rows=0, max_rows=30))
@settings(max_examples=500)
def test_p12_aggregator_always_produces_both_fields(df: pd.DataFrame) -> None:
    """P12: MetricsAggregator always produces both normalization fields.

    **Validates: Requirements 6.12**

    FOR ALL ledger DataFrames (including empty):
        MetricsAggregator.aggregate() MUST produce both
        active_silo_sum_pct and calendar_normalized_return_pct
        for both gate modes.
    """
    result = _aggregator.aggregate(df, total_silos=_TOTAL_SILOS)

    # Both fields must exist for nogate
    assert "nogate_active_silo_sum_pct" in result, (
        "P12 violated: nogate_active_silo_sum_pct missing from aggregator output. "
        f"Keys: {list(result.keys())}"
    )
    assert "nogate_calendar_normalized_return_pct" in result, (
        "P12 violated: nogate_calendar_normalized_return_pct missing from aggregator output. "
        f"Keys: {list(result.keys())}"
    )

    # Both fields must exist for gate001
    assert "gate001_active_silo_sum_pct" in result, (
        "P12 violated: gate001_active_silo_sum_pct missing from aggregator output. "
        f"Keys: {list(result.keys())}"
    )
    assert "gate001_calendar_normalized_return_pct" in result, (
        "P12 violated: gate001_calendar_normalized_return_pct missing from aggregator output. "
        f"Keys: {list(result.keys())}"
    )


@given(df=ledger_dataframe(min_rows=0, max_rows=30))
@settings(max_examples=500)
def test_p12_active_plus_empty_equals_22(df: pd.DataFrame) -> None:
    """P12: active_months + empty_months == 22 always holds from aggregator.

    **Validates: Requirements 6.12**

    FOR ALL ledger DataFrames:
        MetricsAggregator.aggregate() MUST produce
        active_months + empty_months == 22 for both gate modes.
    """
    result = _aggregator.aggregate(df, total_silos=_TOTAL_SILOS)

    # nogate
    nogate_active = result["nogate_active_months"]
    nogate_empty = result["nogate_empty_months"]
    assert nogate_active + nogate_empty == _TOTAL_SILOS, (
        f"P12 violated: nogate active_months({nogate_active}) + "
        f"empty_months({nogate_empty}) = {nogate_active + nogate_empty} != {_TOTAL_SILOS}"
    )

    # gate001
    gate001_active = result["gate001_active_months"]
    gate001_empty = result["gate001_empty_months"]
    assert gate001_active + gate001_empty == _TOTAL_SILOS, (
        f"P12 violated: gate001 active_months({gate001_active}) + "
        f"empty_months({gate001_empty}) = {gate001_active + gate001_empty} != {_TOTAL_SILOS}"
    )


@given(df=ledger_dataframe(min_rows=0, max_rows=30))
@settings(max_examples=500)
def test_p12_invariant_checker_passes_on_aggregator_output(df: pd.DataFrame) -> None:
    """P12: InvariantChecker passes when fed MetricsAggregator output.

    **Validates: Requirements 6.12**

    FOR ALL ledger DataFrames:
        The summary produced by MetricsAggregator MUST pass
        InvariantChecker P12 check (P12_count == 0).
    """
    summary = _aggregator.aggregate(df, total_silos=_TOTAL_SILOS)
    violations = _checker.check(df, summary)

    assert violations["P12_count"] == 0, (
        f"P12 violated on aggregator output: P12_count={violations['P12_count']}. "
        f"nogate: active={summary.get('nogate_active_months')}, "
        f"empty={summary.get('nogate_empty_months')}, "
        f"active_silo_sum_pct={summary.get('nogate_active_silo_sum_pct')}, "
        f"calendar_normalized_return_pct={summary.get('nogate_calendar_normalized_return_pct')}; "
        f"gate001: active={summary.get('gate001_active_months')}, "
        f"empty={summary.get('gate001_empty_months')}"
    )


@given(df=ledger_dataframe(min_rows=1, max_rows=30))
@settings(max_examples=300)
def test_p12_empty_silo_contributes_zero(df: pd.DataFrame) -> None:
    """P12: Empty silos contribute 0.0 to calendar_normalized_return_pct.

    **Validates: Requirements 6.12**

    FOR ALL non-empty ledger DataFrames:
        calendar_normalized_return_pct == active_silo_sum_pct
        (because empty silos contribute 0.0, the sum of active silos
        equals the calendar-normalized sum).
    """
    result = _aggregator.aggregate(df, total_silos=_TOTAL_SILOS)

    # For both gate modes, calendar_normalized_return_pct should equal
    # active_silo_sum_pct since empty silos contribute 0.0
    for prefix in ("nogate", "gate001"):
        active_sum = result[f"{prefix}_active_silo_sum_pct"]
        calendar_norm = result[f"{prefix}_calendar_normalized_return_pct"]

        assert active_sum == calendar_norm, (
            f"P12 violated: {prefix}_active_silo_sum_pct ({active_sum}) != "
            f"{prefix}_calendar_normalized_return_pct ({calendar_norm}). "
            f"Empty silos should contribute 0.0, making them equal."
        )
