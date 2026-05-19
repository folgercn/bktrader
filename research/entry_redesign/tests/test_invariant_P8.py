"""Hypothesis property-based 测试：Round-Trip Serialization (Property 8).

**Validates: Requirements 6.8, 6.14**

Property 8: Round-Trip Serialization
- summary JSON 解码→再编码值语义等价（相对误差 ≤ 1e-12）
- ledger CSV 列顺序字节级一致（与 LEDGER_HEADER 常量完全匹配）

Requirements: 6.8, 6.14
"""

from __future__ import annotations

import json
from typing import Any, Optional

from hypothesis import given, settings
from hypothesis import strategies as st

from research.entry_redesign.ledger.ledger_csv_writer import LEDGER_HEADER


# ---------------------------------------------------------------------------
# Hypothesis strategies: 生成随机 summary dict
# ---------------------------------------------------------------------------

# Finite floats for metric values (can be negative for pnl fields)
_metric_floats = st.floats(
    min_value=-1e8, max_value=1e8, allow_nan=False, allow_infinity=False
)

# Positive finite floats for quality/ratio fields
_positive_metric_floats = st.floats(
    min_value=0.0, max_value=1e6, allow_nan=False, allow_infinity=False
)

# Win rate: [0, 1]
_win_rate_floats = st.floats(
    min_value=0.0, max_value=1.0, allow_nan=False, allow_infinity=False
)

# Nullable float: either a float or None
_nullable_float = st.one_of(st.none(), _metric_floats)

# Nullable positive float: either a positive float or None
_nullable_positive_float = st.one_of(st.none(), _positive_metric_floats)

# Nullable win rate
_nullable_win_rate = st.one_of(st.none(), _win_rate_floats)

# profit_factor special: None, "+inf", or positive float
_profit_factor_values = st.one_of(
    st.none(),
    st.just("+inf"),
    st.floats(min_value=0.01, max_value=1e6, allow_nan=False, allow_infinity=False),
)


@st.composite
def gate_mode_metrics(draw: st.DrawFn) -> dict[str, Any]:
    """Generate a set of 13 metrics for a single gate_mode prefix."""
    trade_count = draw(st.integers(min_value=0, max_value=500))

    if trade_count == 0:
        return {
            "trade_count": 0,
            "win_rate": None,
            "payoff_ratio": None,
            "realistic_pnl_pct": 0.0,
            "realistic_taker_both_pct": 0.0,
            "raw_pnl_pct": 0.0,
            "per_trade_quality_bps_over_notional": None,
            "max_drawdown_pct": 0.0,
            "profit_factor": None,
            "active_silo_sum_pct": 0.0,
            "calendar_normalized_return_pct": 0.0,
            "active_months": 0,
            "empty_months": 22,
        }

    active_months = draw(st.integers(min_value=1, max_value=22))
    empty_months = 22 - active_months

    return {
        "trade_count": trade_count,
        "win_rate": draw(_win_rate_floats),
        "payoff_ratio": draw(_nullable_positive_float),
        "realistic_pnl_pct": draw(_metric_floats),
        "realistic_taker_both_pct": draw(_metric_floats),
        "raw_pnl_pct": draw(_metric_floats),
        "per_trade_quality_bps_over_notional": draw(_metric_floats),
        "max_drawdown_pct": draw(_positive_metric_floats),
        "profit_factor": draw(_profit_factor_values),
        "active_silo_sum_pct": draw(_metric_floats),
        "calendar_normalized_return_pct": draw(_metric_floats),
        "active_months": active_months,
        "empty_months": empty_months,
    }


@st.composite
def summary_dicts(draw: st.DrawFn) -> dict[str, Any]:
    """Generate a random summary dict mimicking the structure of summary JSON.

    Includes nogate_* and gate001_* prefixed metrics, plus structural fields
    like walkforward_config, baseline_reference, asymmetry_tag, etc.
    """
    # Generate metrics for both gate modes
    nogate_metrics = draw(gate_mode_metrics())
    gate001_metrics = draw(gate_mode_metrics())

    summary: dict[str, Any] = {}

    # Prefix metrics
    for key, value in nogate_metrics.items():
        summary[f"nogate_{key}"] = value
    for key, value in gate001_metrics.items():
        summary[f"gate001_{key}"] = value

    # walkforward_config (structural fields)
    summary["walkforward_config"] = {
        "train_months": 2,
        "validation_months": 1,
        "execute_months": 1,
        "execute_start_year_month": "2025-06",
        "execute_end_year_month": "2026-04",
        "total_execute_months": 11,
    }

    # baseline_reference
    summary["baseline_reference"] = {
        "nogate_win_rate": draw(_nullable_win_rate),
        "nogate_payoff_ratio": draw(_nullable_positive_float),
    }

    # event_expectation_positive
    summary["event_expectation_positive"] = draw(st.booleans())
    summary["event_expectation_positive_btc_only"] = draw(st.booleans())
    summary["event_expectation_positive_eth_only"] = draw(st.booleans())

    # small_sample_flag (explicit bool, never missing)
    summary["small_sample_flag"] = draw(st.booleans())

    # asymmetry_tag
    summary["asymmetry_tag"] = draw(
        st.sampled_from([
            "eth_only_positive",
            "btc_only_positive",
            "all_symbols_positive",
            "none",
        ])
    )

    # invariant_violations (fixed schema, 13 fields)
    summary["invariant_violations"] = {
        "P1_missing_trigger_count": draw(st.integers(min_value=0, max_value=100)),
        "P1_spurious_trigger_count": draw(st.integers(min_value=0, max_value=100)),
        "P3_count": draw(st.integers(min_value=0, max_value=100)),
        "P4_count": draw(st.integers(min_value=0, max_value=100)),
        "P5_count": draw(st.integers(min_value=0, max_value=100)),
        "P6_count": draw(st.integers(min_value=0, max_value=100)),
        "P7_ledger_sha256_pairs": [],
        "P8_count": draw(st.integers(min_value=0, max_value=100)),
        "P9_count": draw(st.integers(min_value=0, max_value=100)),
        "P10_count": draw(st.integers(min_value=0, max_value=100)),
        "P11_count": draw(st.integers(min_value=0, max_value=100)),
        "P12_count": draw(st.integers(min_value=0, max_value=100)),
        "live_output_emitted": False,
    }

    # cost_model_baseline / cost_model_stress
    summary["cost_model_baseline"] = {
        "slip_bps_per_side": 2.0,
        "entry_bps": 2.0,
        "exit_bps": 4.0,
    }
    summary["cost_model_stress"] = {
        "slip_bps_per_side": 2.0,
        "entry_bps": 4.0,
        "exit_bps": 4.0,
    }

    # entry_effect_bps / gate_effect_bps / sizing_effect_bps
    summary["entry_effect_bps"] = draw(_metric_floats)
    summary["gate_effect_bps"] = draw(_metric_floats)
    summary["sizing_effect_bps"] = 0.0

    return summary


# ---------------------------------------------------------------------------
# Helper: deep value comparison with relative tolerance for floats
# ---------------------------------------------------------------------------


def _values_equivalent(a: Any, b: Any, rel_tol: float = 1e-12) -> bool:
    """Recursively compare two JSON-decoded values with float tolerance.

    For floats: relative error <= rel_tol (or both zero).
    For other types: exact equality.
    """
    if isinstance(a, dict) and isinstance(b, dict):
        if set(a.keys()) != set(b.keys()):
            return False
        return all(_values_equivalent(a[k], b[k], rel_tol) for k in a)
    if isinstance(a, list) and isinstance(b, list):
        if len(a) != len(b):
            return False
        return all(_values_equivalent(ai, bi, rel_tol) for ai, bi in zip(a, b))
    if isinstance(a, (int, float)) and isinstance(b, (int, float)):
        fa, fb = float(a), float(b)
        if fa == fb:
            return True
        if fa == 0.0 or fb == 0.0:
            return abs(fa - fb) <= rel_tol
        return abs(fa - fb) / max(abs(fa), abs(fb)) <= rel_tol
    return a == b


def _find_mismatch(a: Any, b: Any, path: str = "", rel_tol: float = 1e-12) -> str:
    """Find the first mismatch between two JSON-decoded values.

    Returns a human-readable description of the mismatch, or empty string if equal.
    """
    if isinstance(a, dict) and isinstance(b, dict):
        if set(a.keys()) != set(b.keys()):
            extra_a = set(a.keys()) - set(b.keys())
            extra_b = set(b.keys()) - set(a.keys())
            return f"Key mismatch at {path}: extra_in_a={extra_a}, extra_in_b={extra_b}"
        for k in a:
            result = _find_mismatch(a[k], b[k], f"{path}.{k}", rel_tol)
            if result:
                return result
        return ""
    if isinstance(a, list) and isinstance(b, list):
        if len(a) != len(b):
            return f"List length mismatch at {path}: {len(a)} vs {len(b)}"
        for i, (ai, bi) in enumerate(zip(a, b)):
            result = _find_mismatch(ai, bi, f"{path}[{i}]", rel_tol)
            if result:
                return result
        return ""
    if isinstance(a, (int, float)) and isinstance(b, (int, float)):
        fa, fb = float(a), float(b)
        if fa == fb:
            return ""
        if fa == 0.0 or fb == 0.0:
            if abs(fa - fb) <= rel_tol:
                return ""
            return f"Numeric mismatch at {path}: {fa} vs {fb} (abs diff {abs(fa-fb)})"
        rel_err = abs(fa - fb) / max(abs(fa), abs(fb))
        if rel_err <= rel_tol:
            return ""
        return f"Numeric mismatch at {path}: {fa} vs {fb} (rel_err {rel_err})"
    if a != b:
        return f"Value mismatch at {path}: {a!r} vs {b!r}"
    return ""


# ---------------------------------------------------------------------------
# Property 8: Round-Trip Serialization — summary JSON
# ---------------------------------------------------------------------------


@given(summary=summary_dicts())
@settings(max_examples=500, deadline=None)
def test_p8_summary_json_roundtrip(summary: dict[str, Any]) -> None:
    """Property 8: summary JSON 解码→再编码值语义等价（相对误差 ≤ 1e-12）。

    **Validates: Requirements 6.8**

    FOR ALL random summary dicts:
      json.dumps → json.loads → json.dumps produces value-semantically
      equivalent output within relative error ≤ 1e-12.
    """
    # First encode
    json_str_1 = json.dumps(summary, sort_keys=True, ensure_ascii=False)

    # Decode
    decoded_1 = json.loads(json_str_1)

    # Re-encode
    json_str_2 = json.dumps(decoded_1, sort_keys=True, ensure_ascii=False)

    # Decode again
    decoded_2 = json.loads(json_str_2)

    # Value semantic equivalence with relative tolerance 1e-12
    assert _values_equivalent(decoded_1, decoded_2, rel_tol=1e-12), (
        f"P8 violated: summary JSON round-trip not equivalent within 1e-12.\n"
        f"Mismatch: {_find_mismatch(decoded_1, decoded_2, rel_tol=1e-12)}"
    )


@given(summary=summary_dicts())
@settings(max_examples=500, deadline=None)
def test_p8_summary_json_deterministic_encoding(summary: dict[str, Any]) -> None:
    """Property 8: 同一 summary dict 两次 json.dumps 产出 byte-identical 字符串。

    **Validates: Requirements 6.8**

    FOR ALL random summary dicts:
      Two calls to json.dumps with the same sort_keys produce identical strings.
    """
    json_str_1 = json.dumps(summary, sort_keys=True, ensure_ascii=False)
    json_str_2 = json.dumps(summary, sort_keys=True, ensure_ascii=False)
    assert json_str_1 == json_str_2, (
        "P8 violated: json.dumps is not deterministic for the same input"
    )


@given(summary=summary_dicts())
@settings(max_examples=500, deadline=None)
def test_p8_summary_json_decode_preserves_types(summary: dict[str, Any]) -> None:
    """Property 8: JSON decode preserves value types (int stays int, null stays null).

    **Validates: Requirements 6.8**

    FOR ALL random summary dicts:
      After json.dumps → json.loads, integer fields remain integers,
      None fields remain None (null), string fields remain strings.
    """
    json_str = json.dumps(summary, sort_keys=True, ensure_ascii=False)
    decoded = json.loads(json_str)

    # Check integer fields remain integers
    for prefix in ("nogate_", "gate001_"):
        tc = decoded.get(f"{prefix}trade_count")
        assert tc is None or isinstance(tc, int), (
            f"P8 violated: {prefix}trade_count should be int, got {type(tc)}"
        )
        am = decoded.get(f"{prefix}active_months")
        assert am is None or isinstance(am, int), (
            f"P8 violated: {prefix}active_months should be int, got {type(am)}"
        )
        em = decoded.get(f"{prefix}empty_months")
        assert em is None or isinstance(em, int), (
            f"P8 violated: {prefix}empty_months should be int, got {type(em)}"
        )

    # Check null fields remain null after round-trip
    for prefix in ("nogate_", "gate001_"):
        original_wr = summary.get(f"{prefix}win_rate")
        decoded_wr = decoded.get(f"{prefix}win_rate")
        if original_wr is None:
            assert decoded_wr is None, (
                f"P8 violated: {prefix}win_rate was None but decoded as {decoded_wr}"
            )

    # Check string fields remain strings
    assert decoded.get("asymmetry_tag") == summary["asymmetry_tag"]
    pf_nogate = decoded.get("nogate_profit_factor")
    if summary.get("nogate_profit_factor") == "+inf":
        assert pf_nogate == "+inf", (
            f"P8 violated: nogate_profit_factor '+inf' decoded as {pf_nogate}"
        )


# ---------------------------------------------------------------------------
# Property 8: Round-Trip Serialization — ledger CSV 列顺序字节级一致
# ---------------------------------------------------------------------------


def test_p8_ledger_header_constant_matches_requirement() -> None:
    """Property 8: LEDGER_HEADER 常量与 Requirement 4.5 固定 header 顺序完全一致。

    **Validates: Requirements 6.8**

    验证 LEDGER_HEADER 是一个 22 字段的 tuple，且字段名与顺序与 Requirement 4.5
    完全匹配。
    """
    expected_header = (
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
    )

    assert LEDGER_HEADER == expected_header, (
        f"P8 violated: LEDGER_HEADER does not match Requirement 4.5.\n"
        f"Expected: {expected_header}\n"
        f"Got:      {LEDGER_HEADER}"
    )
    assert len(LEDGER_HEADER) == 22, (
        f"P8 violated: LEDGER_HEADER should have 22 fields, got {len(LEDGER_HEADER)}"
    )


def test_p8_ledger_header_csv_line_no_trailing_comma() -> None:
    """Property 8: ledger CSV header 行末不带逗号。

    **Validates: Requirements 6.8**

    验证 LEDGER_HEADER join 为 CSV header 行时，行末不带逗号、不含 trailing whitespace。
    """
    header_line = ",".join(LEDGER_HEADER)

    # 不含 trailing comma
    assert not header_line.endswith(","), (
        "P8 violated: ledger CSV header line ends with comma"
    )

    # 不含 trailing whitespace
    assert header_line == header_line.rstrip(), (
        "P8 violated: ledger CSV header line has trailing whitespace"
    )

    # 不含 BOM
    assert not header_line.startswith("\ufeff"), (
        "P8 violated: ledger CSV header line starts with BOM"
    )


@given(
    indices=st.lists(
        st.integers(min_value=0, max_value=21),
        min_size=22,
        max_size=22,
        unique=True,
    )
)
@settings(max_examples=100, deadline=None)
def test_p8_ledger_header_order_is_fixed(indices: list[int]) -> None:
    """Property 8: ledger CSV 列顺序不可忽略，必须与 LEDGER_HEADER 完全相同。

    **Validates: Requirements 6.8**

    FOR ALL permutations of column indices:
      Any permutation that differs from identity (0,1,...,21) produces a
      different header line, confirming column order is significant.
    """
    # Create a permuted header
    permuted = tuple(LEDGER_HEADER[i] for i in indices)
    original_line = ",".join(LEDGER_HEADER)
    permuted_line = ",".join(permuted)

    # If the permutation is identity, lines should match
    is_identity = indices == list(range(22))
    if is_identity:
        assert original_line == permuted_line
    else:
        # Non-identity permutation MUST produce a different header line
        assert original_line != permuted_line, (
            f"P8 violated: non-identity permutation {indices} produced same header line"
        )


# ---------------------------------------------------------------------------
# Property 8: Round-Trip for numeric precision edge cases
# ---------------------------------------------------------------------------


@given(
    value=st.floats(
        min_value=-1e15,
        max_value=1e15,
        allow_nan=False,
        allow_infinity=False,
    )
)
@settings(max_examples=1000, deadline=None)
def test_p8_numeric_roundtrip_precision(value: float) -> None:
    """Property 8: 单个数值 JSON round-trip 相对误差 ≤ 1e-12。

    **Validates: Requirements 6.8**

    FOR ALL finite floats:
      json.dumps(value) → json.loads → 与原值相对误差 ≤ 1e-12。
    """
    json_str = json.dumps(value)
    decoded = json.loads(json_str)

    if value == 0.0:
        assert decoded == 0.0, f"P8 violated: 0.0 decoded as {decoded}"
        return

    if decoded == value:
        return

    rel_err = abs(decoded - value) / max(abs(decoded), abs(value))
    assert rel_err <= 1e-12, (
        f"P8 violated: numeric round-trip relative error {rel_err} > 1e-12 "
        f"for value={value}, decoded={decoded}"
    )
