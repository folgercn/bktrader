"""单元测试：36 个默认执行子集。

断言 DEFAULT_SUBSET 长度为 36，包含 baseline，且所有候选满足 H<=D 约束。

Requirements: 2.11
"""

from __future__ import annotations

from research.entry_redesign.scheduler.default_subset import (
    BASELINE,
    DEFAULT_SUBSET,
)
from research.entry_redesign.spec.entry_candidate_spec import EntryCandidateSpec


def test_default_subset_length_is_36() -> None:
    """DEFAULT_SUBSET 恰好包含 36 个候选。"""
    assert len(DEFAULT_SUBSET) == 36


def test_default_subset_contains_baseline() -> None:
    """DEFAULT_SUBSET 包含 Baseline_Entry_Candidate。"""
    assert BASELINE in DEFAULT_SUBSET


def test_default_subset_all_valid_h_le_d() -> None:
    """所有候选满足 H<=D 约束（Point_In_Time_Feature）。"""
    for candidate in DEFAULT_SUBSET:
        assert candidate.feature_horizon_seconds <= candidate.entry_delay_seconds, (
            f"H={candidate.feature_horizon_seconds} > D={candidate.entry_delay_seconds} "
            f"violates H<=D constraint"
        )


def test_default_subset_no_duplicates() -> None:
    """DEFAULT_SUBSET 中无重复候选。"""
    assert len(set(DEFAULT_SUBSET)) == len(DEFAULT_SUBSET)


def test_default_subset_all_are_entry_candidate_spec() -> None:
    """所有元素均为 EntryCandidateSpec 实例。"""
    for candidate in DEFAULT_SUBSET:
        assert isinstance(candidate, EntryCandidateSpec)


def test_default_subset_baseline_definition() -> None:
    """Baseline_Entry_Candidate 定义正确。"""
    assert BASELINE.entry_delay_seconds == 0
    assert BASELINE.feature_horizon_seconds == 0
    assert BASELINE.trigger_confirmation_id == "none"
    assert BASELINE.entry_price_mode_id == "market_on_touch"
    assert BASELINE.pretouch_state_band_id == "none"
    assert BASELINE.posttouch_quality_band_id == "none"


def test_default_subset_d_axis_coverage() -> None:
    """D 轴 5 个非 baseline 值全部覆盖。"""
    d_values = {
        c.entry_delay_seconds
        for c in DEFAULT_SUBSET
        if c.entry_delay_seconds != BASELINE.entry_delay_seconds
    }
    assert d_values == {5, 15, 30, 60, 120}


def test_default_subset_tc_axis_coverage() -> None:
    """Trigger_Confirmation 轴 10 个非 none 值全部覆盖。"""
    tc_values = {
        c.trigger_confirmation_id
        for c in DEFAULT_SUBSET
        if c.trigger_confirmation_id != "none"
    }
    expected = {
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
    }
    assert tc_values == expected


def test_default_subset_epm_axis_coverage() -> None:
    """Entry_Price_Mode 轴 9 个非 market_on_touch 值全部覆盖。"""
    epm_values = {
        c.entry_price_mode_id
        for c in DEFAULT_SUBSET
        if c.entry_price_mode_id != "market_on_touch"
    }
    expected = {
        "limit_at_level",
        "limit_tb_k0",
        "limit_tb_k1",
        "limit_tb_k2",
        "limit_tb_k4",
        "pullback_p000",
        "pullback_p002",
        "pullback_p005",
        "pullback_p010",
    }
    assert epm_values == expected


def test_default_subset_psb_axis_coverage() -> None:
    """Pretouch_State_Band 轴 2 个非 none 值全部覆盖。"""
    psb_values = {
        c.pretouch_state_band_id
        for c in DEFAULT_SUBSET
        if c.pretouch_state_band_id != "none"
    }
    assert psb_values == {"fast_clean", "fast_clean_strict"}


def test_default_subset_pqb_axis_coverage() -> None:
    """Posttouch_Quality_Band 轴 9 个非 none 值全部覆盖。"""
    pqb_values = {
        c.posttouch_quality_band_id
        for c in DEFAULT_SUBSET
        if c.posttouch_quality_band_id != "none"
    }
    expected = {
        "cont1s_r003",
        "cont1s_r005",
        "cont1s_r008",
        "tickimb_b055",
        "tickimb_b060",
        "tickimb_b065",
        "spread_s1",
        "spread_s2",
        "spread_s4",
    }
    assert pqb_values == expected


def test_default_subset_is_deterministic() -> None:
    """多次构建产出相同结果（确定性）。"""
    from research.entry_redesign.scheduler.default_subset import _build_default_subset

    subset_a = _build_default_subset()
    subset_b = _build_default_subset()
    assert subset_a == subset_b


def test_default_subset_single_axis_only() -> None:
    """除 baseline 外，每个候选仅在一个维度上与 baseline 不同。"""
    for candidate in DEFAULT_SUBSET:
        if candidate == BASELINE:
            continue
        diffs = 0
        if candidate.entry_delay_seconds != BASELINE.entry_delay_seconds:
            diffs += 1
        if candidate.feature_horizon_seconds != BASELINE.feature_horizon_seconds:
            diffs += 1
        if candidate.trigger_confirmation_id != BASELINE.trigger_confirmation_id:
            diffs += 1
        if candidate.entry_price_mode_id != BASELINE.entry_price_mode_id:
            diffs += 1
        if candidate.pretouch_state_band_id != BASELINE.pretouch_state_band_id:
            diffs += 1
        if candidate.posttouch_quality_band_id != BASELINE.posttouch_quality_band_id:
            diffs += 1
        assert diffs == 1, (
            f"Candidate differs from baseline in {diffs} dimensions "
            f"(expected 1): {candidate}"
        )
