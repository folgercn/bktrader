"""36 个默认执行子集枚举。

Baseline_Entry_Candidate + 单轴全量扫描（D 5 轴 + H 3 轴 +
Trigger_Confirmation 10 轴 + Entry_Price_Mode 9 轴 +
Pretouch_State_Band 2 轴 + Posttouch_Quality_Band 9 轴 =
1 + 38 个但去重后 36 个）。

去重来源：H 轴的 3 个候选（H∈{5,15,30}）在 baseline D=0 下违反
H<=D 约束（Point_In_Time_Feature），被过滤后剩余 36 个合法候选。

超出默认子集的组合 MUST 在 design 阶段先登记到 design.md 子集扩展表。

Requirements: 2.11
"""

from __future__ import annotations

from research.entry_redesign.spec.entry_candidate_spec import (
    EntryCandidateSpec,
    EntryPriceModeId,
    PosttouchQualityBandId,
    PretouchStateBandId,
    TriggerConfirmationId,
)

# ---------------------------------------------------------------------------
# Baseline_Entry_Candidate
# ---------------------------------------------------------------------------

BASELINE = EntryCandidateSpec(
    entry_delay_seconds=0,
    feature_horizon_seconds=0,
    trigger_confirmation_id="none",
    entry_price_mode_id="market_on_touch",
    pretouch_state_band_id="none",
    posttouch_quality_band_id="none",
)
"""Baseline_Entry_Candidate: (D=0, H=0, none, market_on_touch, none, none)。
所有其他 Entry_Candidate MUST 与该 baseline 在同一 runner、同一 events、
同一 cost model、同一 seed 下对照。Requirements: 2.9"""

# ---------------------------------------------------------------------------
# 单轴全量扫描值域
# ---------------------------------------------------------------------------

# D 轴: D ∈ {5, 15, 30, 60, 120}（排除 baseline D=0）→ 5 个候选
_D_AXIS_VALUES: list[int] = [5, 15, 30, 60, 120]

# H 轴: H ∈ {5, 15, 30}（排除 baseline H=0）→ 理论 3 个候选
# 但 baseline D=0 下 H>0 违反 H<=D 约束，全部被过滤。
# 这 3 个被过滤的候选即为 "1 + 38 个但去重后 36 个" 中被去除的 3 个。
_H_AXIS_VALUES: list[int] = [5, 15, 30]

# Trigger_Confirmation 轴: 10 个非 none 值 → 10 个候选
_TC_AXIS_VALUES: list[TriggerConfirmationId] = [
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

# Entry_Price_Mode 轴: 9 个非 market_on_touch 值 → 9 个候选
_EPM_AXIS_VALUES: list[EntryPriceModeId] = [
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

# Pretouch_State_Band 轴: 2 个非 none 值 → 2 个候选
_PSB_AXIS_VALUES: list[PretouchStateBandId] = [
    "fast_clean",
    "fast_clean_strict",
]

# Posttouch_Quality_Band 轴: 9 个非 none 值 → 9 个候选
_PQB_AXIS_VALUES: list[PosttouchQualityBandId] = [
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


def _build_default_subset() -> list[EntryCandidateSpec]:
    """构建 36 个默认执行子集。

    生成策略：baseline + 单轴全量扫描，仅保留满足 H<=D 约束的候选。

    单轴扫描定义：从 baseline 出发，仅改变一个维度的值，其余维度保持
    baseline 值不变。对于 H 轴，由于 baseline D=0，任何 H>0 都违反
    H<=D 约束，因此 H 轴的 3 个候选被过滤。

    最终合法候选：
    - Baseline: 1
    - D 轴 (D∈{5,15,30,60,120}, H=0, rest=baseline): 5
    - H 轴 (H∈{5,15,30}, D=0, rest=baseline): 0（全部违反 H<=D）
    - Trigger_Confirmation 轴: 10
    - Entry_Price_Mode 轴: 9
    - Pretouch_State_Band 轴: 2
    - Posttouch_Quality_Band 轴: 9
    合计: 1 + 5 + 0 + 10 + 9 + 2 + 9 = 36
    """
    candidates: set[EntryCandidateSpec] = set()

    # 1. Baseline
    candidates.add(BASELINE)

    # 2. D 轴扫描: vary D, H=0, rest = baseline
    for d in _D_AXIS_VALUES:
        candidates.add(
            EntryCandidateSpec(
                entry_delay_seconds=d,
                feature_horizon_seconds=0,
                trigger_confirmation_id="none",
                entry_price_mode_id="market_on_touch",
                pretouch_state_band_id="none",
                posttouch_quality_band_id="none",
            )
        )

    # 3. H 轴扫描: vary H, D=baseline(0), rest = baseline
    #    由于 baseline D=0，H>0 违反 H<=D 约束，全部被过滤。
    #    这是 "1 + 38 个但去重后 36 个" 中被去除的 3 个。
    for h in _H_AXIS_VALUES:
        spec = EntryCandidateSpec(
            entry_delay_seconds=BASELINE.entry_delay_seconds,
            feature_horizon_seconds=h,
            trigger_confirmation_id="none",
            entry_price_mode_id="market_on_touch",
            pretouch_state_band_id="none",
            posttouch_quality_band_id="none",
        )
        # 仅保留满足 H<=D 约束的候选
        if spec.feature_horizon_seconds <= spec.entry_delay_seconds:
            candidates.add(spec)
        # else: 被过滤（H>D=0 违反 Point_In_Time_Feature 约束）

    # 4. Trigger_Confirmation 轴扫描: vary TC, rest = baseline
    for tc in _TC_AXIS_VALUES:
        candidates.add(
            EntryCandidateSpec(
                entry_delay_seconds=0,
                feature_horizon_seconds=0,
                trigger_confirmation_id=tc,
                entry_price_mode_id="market_on_touch",
                pretouch_state_band_id="none",
                posttouch_quality_band_id="none",
            )
        )

    # 5. Entry_Price_Mode 轴扫描: vary EPM, rest = baseline
    for epm in _EPM_AXIS_VALUES:
        candidates.add(
            EntryCandidateSpec(
                entry_delay_seconds=0,
                feature_horizon_seconds=0,
                trigger_confirmation_id="none",
                entry_price_mode_id=epm,
                pretouch_state_band_id="none",
                posttouch_quality_band_id="none",
            )
        )

    # 6. Pretouch_State_Band 轴扫描: vary PSB, rest = baseline
    for psb in _PSB_AXIS_VALUES:
        candidates.add(
            EntryCandidateSpec(
                entry_delay_seconds=0,
                feature_horizon_seconds=0,
                trigger_confirmation_id="none",
                entry_price_mode_id="market_on_touch",
                pretouch_state_band_id=psb,
                posttouch_quality_band_id="none",
            )
        )

    # 7. Posttouch_Quality_Band 轴扫描: vary PQB, rest = baseline
    for pqb in _PQB_AXIS_VALUES:
        candidates.add(
            EntryCandidateSpec(
                entry_delay_seconds=0,
                feature_horizon_seconds=0,
                trigger_confirmation_id="none",
                entry_price_mode_id="market_on_touch",
                pretouch_state_band_id="none",
                posttouch_quality_band_id=pqb,
            )
        )

    # 转为排序后的 list（按 candidate 字段值的自然顺序排列，保证确定性）
    sorted_candidates = sorted(
        candidates,
        key=lambda c: (
            c.entry_delay_seconds,
            c.feature_horizon_seconds,
            c.trigger_confirmation_id,
            c.entry_price_mode_id,
            c.pretouch_state_band_id,
            c.posttouch_quality_band_id,
        ),
    )
    return sorted_candidates


DEFAULT_SUBSET: list[EntryCandidateSpec] = _build_default_subset()
"""36 个默认执行子集常量 list。

包含 Baseline_Entry_Candidate + 单轴全量扫描去重后的候选集合。
超出默认子集的组合 MUST 在 design 阶段先登记到 design.md 子集扩展表。
Requirements: 2.11
"""
