"""Hypothesis property-based 测试：candidate_id 正则 + 确定性。

**Validates: Requirements 2.12, 6.8 (P8 round-trip)**

FOR ALL 合法六元组：
  1. candidate_id 匹配正则 ^[a-z0-9]+(?:_[a-z0-9]+)*-[0-9a-f]{12}$
  2. 同输入两次调用产出相同字符串（确定性）
  3. 长度在 [14, 64] 范围内

Requirements: 2.12, 6.8
"""

from __future__ import annotations

import re

from hypothesis import given, settings
from hypothesis import strategies as st

from research.entry_redesign.spec.candidate_id import generate_candidate_id
from research.entry_redesign.spec.entry_candidate_spec import (
    VALID_D,
    VALID_H,
    EntryCandidateSpec,
    EntryPriceModeId,
    PosttouchQualityBandId,
    PretouchStateBandId,
    TriggerConfirmationId,
)

# ---------------------------------------------------------------------------
# Hypothesis strategies for valid EntryCandidateSpec instances
# ---------------------------------------------------------------------------

# All valid Literal enum values
_TRIGGER_CONFIRMATION_VALUES: list[TriggerConfirmationId] = [
    "none",
    "persistence_n1",
    "persistence_n3",
    "persistence_n5",
    "persistence_n10",
    "retest_tb0",
    "retest_tb1",
    "retest_tb2",
    "minvol_bps50",
    "minvol_bps100",
    "minvol_bps200",
]

_ENTRY_PRICE_MODE_VALUES: list[EntryPriceModeId] = [
    "market_on_touch",
    "limit_at_level",
    "limit_tb_k0",
    "limit_tb_k1",
    "limit_tb_k2",
    "limit_tb_k4",
    "pullback_p000",
    "pullback_p002",
    "pullback_p005",
    "pullback_p010",
]

_PRETOUCH_STATE_BAND_VALUES: list[PretouchStateBandId] = [
    "none",
    "fast_clean",
    "fast_clean_strict",
]

_POSTTOUCH_QUALITY_BAND_VALUES: list[PosttouchQualityBandId] = [
    "none",
    "cont1s_r003",
    "cont1s_r005",
    "cont1s_r008",
    "tickimb_b055",
    "tickimb_b060",
    "tickimb_b065",
    "spread_s1",
    "spread_s2",
    "spread_s4",
]

# Strategy: generate valid (D, H) pairs respecting H <= D constraint
_VALID_D_SORTED = sorted(VALID_D)
_VALID_H_SORTED = sorted(VALID_H)


@st.composite
def valid_d_h_pairs(draw: st.DrawFn) -> tuple[int, int]:
    """Generate a valid (D, H) pair where H <= D."""
    d = draw(st.sampled_from(_VALID_D_SORTED))
    # Filter H values to those <= D
    valid_h_for_d = [h for h in _VALID_H_SORTED if h <= d]
    h = draw(st.sampled_from(valid_h_for_d))
    return (d, h)


@st.composite
def valid_entry_candidate_specs(draw: st.DrawFn) -> EntryCandidateSpec:
    """Generate a valid EntryCandidateSpec with H <= D constraint satisfied."""
    d, h = draw(valid_d_h_pairs())
    tc = draw(st.sampled_from(_TRIGGER_CONFIRMATION_VALUES))
    ep = draw(st.sampled_from(_ENTRY_PRICE_MODE_VALUES))
    pr = draw(st.sampled_from(_PRETOUCH_STATE_BAND_VALUES))
    po = draw(st.sampled_from(_POSTTOUCH_QUALITY_BAND_VALUES))
    return EntryCandidateSpec(
        entry_delay_seconds=d,
        feature_horizon_seconds=h,
        trigger_confirmation_id=tc,
        entry_price_mode_id=ep,
        pretouch_state_band_id=pr,
        posttouch_quality_band_id=po,
    )


# ---------------------------------------------------------------------------
# candidate_id 正则（来自 Requirement 2.12）
# ---------------------------------------------------------------------------

_CANDIDATE_ID_REGEX = re.compile(r"^[a-z0-9]+(?:_[a-z0-9]+)*-[0-9a-f]{12}$")


# ---------------------------------------------------------------------------
# Property-based tests
# ---------------------------------------------------------------------------


@given(spec=valid_entry_candidate_specs())
@settings(max_examples=500)
def test_candidate_id_matches_regex(spec: EntryCandidateSpec) -> None:
    """FOR ALL 合法六元组：candidate_id 匹配正则。

    **Validates: Requirements 2.12**

    正则: ^[a-z0-9]+(?:_[a-z0-9]+)*-[0-9a-f]{12}$
    """
    cid = generate_candidate_id(spec)
    assert _CANDIDATE_ID_REGEX.match(cid), (
        f"candidate_id {cid!r} does not match required regex "
        f"for spec: D={spec.entry_delay_seconds}, H={spec.feature_horizon_seconds}, "
        f"TC={spec.trigger_confirmation_id}, EP={spec.entry_price_mode_id}, "
        f"PR={spec.pretouch_state_band_id}, PO={spec.posttouch_quality_band_id}"
    )


@given(spec=valid_entry_candidate_specs())
@settings(max_examples=500)
def test_candidate_id_deterministic(spec: EntryCandidateSpec) -> None:
    """FOR ALL 合法六元组：同输入两次调用产出相同字符串。

    **Validates: Requirements 2.12, 6.8 (P8 round-trip)**

    确定性：同一 EntryCandidateSpec 两次调用 generate_candidate_id
    MUST 产出 byte-identical candidate_id。
    """
    cid_a = generate_candidate_id(spec)
    cid_b = generate_candidate_id(spec)
    assert cid_a == cid_b, (
        f"Non-deterministic candidate_id: {cid_a!r} != {cid_b!r} "
        f"for spec: D={spec.entry_delay_seconds}, H={spec.feature_horizon_seconds}, "
        f"TC={spec.trigger_confirmation_id}, EP={spec.entry_price_mode_id}, "
        f"PR={spec.pretouch_state_band_id}, PO={spec.posttouch_quality_band_id}"
    )


@given(spec=valid_entry_candidate_specs())
@settings(max_examples=500)
def test_candidate_id_length_in_range(spec: EntryCandidateSpec) -> None:
    """FOR ALL 合法六元组：candidate_id 长度在 [14, 64] 范围内。

    **Validates: Requirements 2.12**
    """
    cid = generate_candidate_id(spec)
    assert 14 <= len(cid) <= 64, (
        f"candidate_id length {len(cid)} not in [14, 64]: {cid!r} "
        f"for spec: D={spec.entry_delay_seconds}, H={spec.feature_horizon_seconds}, "
        f"TC={spec.trigger_confirmation_id}, EP={spec.entry_price_mode_id}, "
        f"PR={spec.pretouch_state_band_id}, PO={spec.posttouch_quality_band_id}"
    )
