"""Hypothesis property-based 测试：P1 counters — missing vs spurious.

**Validates: Requirements 6.1, 6.14**

Property 1 counters: missing vs spurious
  P1_missing_trigger_count + P1_spurious_trigger_count == 总违反数

使用 hypothesis 生成 summary dicts，包含随机的 P1_missing_trigger_count
和 P1_spurious_trigger_count 值。验证：
  1. assert_invariant_P1 在任一 count > 0 时 raise InvariantViolation。
  2. InvariantViolation.details 中 total_violations == missing + spurious。
  3. 当两个 count 均为 0 时，assert_invariant_P1 不 raise。

Requirements: 6.1, 6.14
"""

from __future__ import annotations

import pandas as pd
import pytest
from hypothesis import given, settings
from hypothesis import strategies as st

from research.entry_redesign.invariants.assertions import (
    InvariantViolation,
    assert_invariant_P1,
)


# ---------------------------------------------------------------------------
# Hypothesis strategies
# ---------------------------------------------------------------------------


@st.composite
def summary_with_p1_counters(draw: st.DrawFn) -> dict:
    """Generate a summary dict with random P1 counter values.

    P1_missing_trigger_count: non-negative integer [0, 1000]
    P1_spurious_trigger_count: non-negative integer [0, 1000]
    """
    missing = draw(st.integers(min_value=0, max_value=1000))
    spurious = draw(st.integers(min_value=0, max_value=1000))
    # Simple deterministic candidate_id matching the required regex pattern
    hex_suffix = draw(st.text(alphabet="0123456789abcdef", min_size=12, max_size=12))
    candidate_id = f"d0_h0_none_market-{hex_suffix}"

    summary = {
        "candidate_id": candidate_id,
        "invariant_violations": {
            "P1_missing_trigger_count": missing,
            "P1_spurious_trigger_count": spurious,
            "P3_count": 0,
            "P4_count": 0,
            "P5_count": 0,
            "P6_count": 0,
            "P7_ledger_sha256_pairs": [],
            "P8_count": 0,
            "P9_count": 0,
            "P10_count": 0,
            "P11_count": 0,
            "P12_count": 0,
            "live_output_emitted": False,
        },
    }
    return summary


@st.composite
def summary_with_violations(draw: st.DrawFn) -> dict:
    """Generate a summary dict where at least one P1 counter > 0.

    Ensures total_violations = missing + spurious > 0.
    """
    # At least one must be > 0
    missing = draw(st.integers(min_value=0, max_value=1000))
    spurious = draw(st.integers(min_value=0, max_value=1000))
    # Ensure at least one is > 0
    if missing == 0 and spurious == 0:
        # Force at least one to be positive
        if draw(st.booleans()):
            missing = draw(st.integers(min_value=1, max_value=1000))
        else:
            spurious = draw(st.integers(min_value=1, max_value=1000))

    hex_suffix = draw(st.text(alphabet="0123456789abcdef", min_size=12, max_size=12))
    candidate_id = f"d0_h0_none_market-{hex_suffix}"

    summary = {
        "candidate_id": candidate_id,
        "invariant_violations": {
            "P1_missing_trigger_count": missing,
            "P1_spurious_trigger_count": spurious,
            "P3_count": 0,
            "P4_count": 0,
            "P5_count": 0,
            "P6_count": 0,
            "P7_ledger_sha256_pairs": [],
            "P8_count": 0,
            "P9_count": 0,
            "P10_count": 0,
            "P11_count": 0,
            "P12_count": 0,
            "live_output_emitted": False,
        },
    }
    return summary


# ---------------------------------------------------------------------------
# Property-based tests
# ---------------------------------------------------------------------------


@given(summary=summary_with_violations())
@settings(max_examples=500)
def test_p1_counters_violation_raises(summary: dict) -> None:
    """P1 counters: assert_invariant_P1 raises when either count > 0.

    **Validates: Requirements 6.1**

    WHEN P1_missing_trigger_count > 0 OR P1_spurious_trigger_count > 0,
    THEN assert_invariant_P1 MUST raise InvariantViolation.
    """
    ledger = pd.DataFrame()  # empty ledger is fine for P1 check

    with pytest.raises(InvariantViolation) as exc_info:
        assert_invariant_P1(ledger, summary)

    exc = exc_info.value
    assert exc.property_id == "P1"


@given(summary=summary_with_violations())
@settings(max_examples=500)
def test_p1_counters_total_equals_missing_plus_spurious(summary: dict) -> None:
    """P1 counters: total_violations == missing + spurious.

    **Validates: Requirements 6.1**

    P1_missing_trigger_count + P1_spurious_trigger_count == 总违反数
    """
    ledger = pd.DataFrame()

    violations = summary["invariant_violations"]
    missing = violations["P1_missing_trigger_count"]
    spurious = violations["P1_spurious_trigger_count"]
    expected_total = missing + spurious

    with pytest.raises(InvariantViolation) as exc_info:
        assert_invariant_P1(ledger, summary)

    exc = exc_info.value
    # The details dict should contain total_violations == missing + spurious
    assert exc.details["total_violations"] == expected_total
    assert exc.details["P1_missing_trigger_count"] == missing
    assert exc.details["P1_spurious_trigger_count"] == spurious


@given(summary=summary_with_p1_counters())
@settings(max_examples=500)
def test_p1_counters_no_violation_when_both_zero(summary: dict) -> None:
    """P1 counters: no raise when both counts are 0.

    **Validates: Requirements 6.1**

    WHEN P1_missing_trigger_count == 0 AND P1_spurious_trigger_count == 0,
    THEN assert_invariant_P1 MUST NOT raise.
    """
    violations = summary["invariant_violations"]
    missing = violations["P1_missing_trigger_count"]
    spurious = violations["P1_spurious_trigger_count"]

    ledger = pd.DataFrame()

    if missing == 0 and spurious == 0:
        # Should NOT raise
        assert_invariant_P1(ledger, summary)  # no exception
    else:
        # Should raise — just verify it does
        with pytest.raises(InvariantViolation):
            assert_invariant_P1(ledger, summary)
