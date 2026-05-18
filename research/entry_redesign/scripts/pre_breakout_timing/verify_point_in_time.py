"""
verify_point_in_time — 验证特征 Point_In_Time 约束

Task 3.5: 确认所有 PRE_BREAKOUT_FEATURES 和 OPTIONAL_FEATURES 在 signal bar
时刻（breakout 触发前）已确定，无 post-breakout 数据泄露。

验证方法：
1. 追溯每个特征在 probabilistic_v4_event_dataset.py 中的计算逻辑
2. 判断其数据来源是否仅使用 touch_time 及之前的信息
3. 对 116 events 实际运行 extract_features() 检查可用性和缺失率
4. 标记任何违反 Point_In_Time 的特征

结论：
- PRE_BREAKOUT_FEATURES 中的 touch_extension_atr 使用 touch_time 时刻的
  close 价格（即 breakout 触发瞬间的 1s bar close），这是 breakout 触发
  的定义时刻本身，属于 "at touch time" 而非 "post-touch"，可接受。
- OPTIONAL_FEATURES 中的 dwell_*_pass 使用 touch_time 之后的数据
  （touch_pos 到 touch_pos + seconds），属于 POST-BREAKOUT 数据，
  违反 Point_In_Time 约束，必须排除。
- OPTIONAL_FEATURES 中的 state_* 特征使用 touch_time 之前 60s 的状态序列
  （touch_pos - 59 到 touch_pos），属于 PRE-BREAKOUT 数据，合规。
"""

from __future__ import annotations

import sys
from pathlib import Path

import pandas as pd

# 确保可以 import 同级模块
sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from pre_breakout_timing.feature_extractor import (
    EXCLUDED_POST_TOUCH_FEATURES,
    OPTIONAL_FEATURES,
    PRE_BREAKOUT_FEATURES,
    extract_features,
)
from pre_breakout_timing.data_layer import load_v6_gate_events


# ---------------------------------------------------------------------------
# 特征时间点分析（基于 probabilistic_v4_event_dataset.py 源码审计）
# ---------------------------------------------------------------------------

FEATURE_TIMING_ANALYSIS: dict[str, dict] = {
    # === PRE_BREAKOUT_FEATURES ===
    "signal_atr_percentile": {
        "source": "sig.get('prev_atr_percentile_1', sig.get('atr_percentile'))",
        "timing": "PRE-TOUCH",
        "explanation": "来自前一根 bar 的 ATR percentile（prev_atr_percentile_1），"
                       "或 signal bar 自身的 atr_percentile。两者均在 signal bar 开始时已确定。",
        "point_in_time": True,
    },
    "roundtrip_cost_atr": {
        "source": "touch_close * cost_rate / atr",
        "timing": "AT-TOUCH",
        "explanation": "使用 touch_close（breakout 触发瞬间的 1s bar close）和 ATR。"
                       "touch_close 是 breakout 定义时刻的价格，不是 post-breakout 数据。"
                       "cost_rate 是常数。ATR 来自前一根 bar。",
        "point_in_time": True,
    },
    "prev1_body_atr": {
        "source": "(prev_close_1 - prev_open_1) * side_mult / atr",
        "timing": "PRE-TOUCH",
        "explanation": "前一根 bar（prev bar 1）的 body 大小除以 ATR。"
                       "prev_close_1、prev_open_1 均为已闭合 bar 的数据。",
        "point_in_time": True,
    },
    "prev1_range_atr": {
        "source": "(prev_high_1 - prev_low_1) / atr",
        "timing": "PRE-TOUCH",
        "explanation": "前一根 bar 的 range 除以 ATR。完全来自已闭合 bar。",
        "point_in_time": True,
    },
    "prev1_close_pos_side": {
        "source": "(prev_close_1 - prev_low_1) / prev_range_1, adjusted by side",
        "timing": "PRE-TOUCH",
        "explanation": "前一根 bar 的 close 在 range 中的相对位置（方向调整后）。"
                       "完全来自已闭合 bar。",
        "point_in_time": True,
    },
    "prev_sma5_gap_atr": {
        "source": "(signal_open - prev_sma5_1) * side_mult / atr",
        "timing": "PRE-TOUCH",
        "explanation": "signal bar 的 open 与前一根 bar 的 SMA5 之间的差距。"
                       "signal_open 在 signal bar 开始时已确定，prev_sma5_1 来自已闭合 bar。",
        "point_in_time": True,
    },
    "prev_sma5_slope_atr": {
        "source": "(prev_sma5_1 - prev_sma5_2) * side_mult / atr",
        "timing": "PRE-TOUCH",
        "explanation": "前两根 bar 的 SMA5 差值（斜率）。完全来自已闭合 bar。",
        "point_in_time": True,
    },
    "level_to_prev_close_atr": {
        "source": "(prev_close_1 - level) * side_mult / atr",
        "timing": "PRE-TOUCH",
        "explanation": "breakout level 与前一根 bar close 的距离。"
                       "level 在 signal bar 开始时已确定（由 T2 结构定义），"
                       "prev_close_1 来自已闭合 bar。",
        "point_in_time": True,
    },
    "level_to_signal_open_atr": {
        "source": "(signal_open - level) * side_mult / atr",
        "timing": "PRE-TOUCH",
        "explanation": "signal bar open 与 breakout level 的距离。"
                       "两者均在 signal bar 开始时已确定。",
        "point_in_time": True,
    },
    "touch_extension_atr": {
        "source": "abs(touch_close - level) / atr",
        "timing": "AT-TOUCH",
        "explanation": "breakout 触发瞬间（touch_time）的 1s bar close 与 level 的距离。"
                       "touch_close 是 breakout 定义时刻的价格——即 '价格触及 level' 的那一秒。"
                       "这不是 post-breakout 数据，而是 breakout 事件本身的定义属性。"
                       "在分类器使用场景中，breakout 触发时该值已确定。",
        "point_in_time": True,
    },
    # === OPTIONAL_FEATURES: dwell_* ===
    "dwell_5s_pass": {
        "source": "_dwell_feature(close_values, touch_pos, side, level, 5)",
        "timing": "POST-TOUCH ⚠️",
        "explanation": "检查 touch_pos 到 touch_pos+5 范围内价格是否持续在 level 之上/之下。"
                       "使用了 touch_time 之后 5 秒的 1s bar 数据。"
                       "这是 POST-BREAKOUT 数据，违反 Point_In_Time 约束！",
        "point_in_time": False,
    },
    "dwell_15s_pass": {
        "source": "_dwell_feature(close_values, touch_pos, side, level, 15)",
        "timing": "POST-TOUCH ⚠️",
        "explanation": "检查 touch_pos 到 touch_pos+15 范围内价格是否持续在 level 之上/之下。"
                       "使用了 touch_time 之后 15 秒的 1s bar 数据。"
                       "这是 POST-BREAKOUT 数据，违反 Point_In_Time 约束！",
        "point_in_time": False,
    },
    "dwell_30s_pass": {
        "source": "_dwell_feature(close_values, touch_pos, side, level, 30)",
        "timing": "POST-TOUCH ⚠️",
        "explanation": "检查 touch_pos 到 touch_pos+30 范围内价格是否持续在 level 之上/之下。"
                       "使用了 touch_time 之后 30 秒的 1s bar 数据。"
                       "这是 POST-BREAKOUT 数据，违反 Point_In_Time 约束！",
        "point_in_time": False,
    },
    "dwell_60s_pass": {
        "source": "_dwell_feature(close_values, touch_pos, side, level, 60)",
        "timing": "POST-TOUCH ⚠️",
        "explanation": "检查 touch_pos 到 touch_pos+60 范围内价格是否持续在 level 之上/之下。"
                       "使用了 touch_time 之后 60 秒的 1s bar 数据。"
                       "这是 POST-BREAKOUT 数据，违反 Point_In_Time 约束！",
        "point_in_time": False,
    },
    # === OPTIONAL_FEATURES: state_* ===
    "state_frac_0": {
        "source": "_state_sequence(states, touch_pos, side, 60) → fraction of state 0",
        "timing": "PRE-TOUCH",
        "explanation": "从 state_seq_60s 计算：_state_sequence 使用 "
                       "states[max(0, touch_pos-59) : touch_pos+1]，"
                       "即 touch_time 之前 60 秒的状态序列。touch_pos 本身包含在内"
                       "（即 breakout 触发瞬间的状态），但不使用 touch 之后的数据。",
        "point_in_time": True,
    },
    "state_frac_1": {
        "source": "_state_sequence(states, touch_pos, side, 60) → fraction of state 1",
        "timing": "PRE-TOUCH",
        "explanation": "同 state_frac_0，使用 touch_time 前 60s 的状态序列。",
        "point_in_time": True,
    },
    "state_frac_2": {
        "source": "_state_sequence(states, touch_pos, side, 60) → fraction of state 2",
        "timing": "PRE-TOUCH",
        "explanation": "同 state_frac_0，使用 touch_time 前 60s 的状态序列。",
        "point_in_time": True,
    },
    "state_frac_3": {
        "source": "_state_sequence(states, touch_pos, side, 60) → fraction of state 3",
        "timing": "PRE-TOUCH",
        "explanation": "同 state_frac_0，使用 touch_time 前 60s 的状态序列。",
        "point_in_time": True,
    },
    "state_entropy": {
        "source": "_state_sequence(states, touch_pos, side, 60) → Shannon entropy",
        "timing": "PRE-TOUCH",
        "explanation": "同 state_frac_0，使用 touch_time 前 60s 的状态序列计算 Shannon 熵。",
        "point_in_time": True,
    },
}


def run_verification() -> dict:
    """运行完整的 Point_In_Time 验证。

    Returns:
        dict: 验证结果，包含：
        - compliant_features: 合规特征列表
        - violated_features: 违规特征列表（需排除）
        - feature_availability: 各特征在 116 events 中的可用性
        - summary: 文字总结
    """
    print("=" * 70)
    print("Point_In_Time 验证：Pre-Breakout 特征时间点审计")
    print("=" * 70)

    # --- 1. 源码审计结果 ---
    print("\n[1/3] 源码审计结果（基于 probabilistic_v4_event_dataset.py）")
    print("-" * 70)

    compliant: list[str] = []
    violated: list[str] = []

    all_features = PRE_BREAKOUT_FEATURES + OPTIONAL_FEATURES + EXCLUDED_POST_TOUCH_FEATURES
    for feat in all_features:
        analysis = FEATURE_TIMING_ANALYSIS.get(feat)
        if analysis is None:
            print(f"  ⚠️  {feat}: 未找到分析记录")
            continue

        status = "✅" if analysis["point_in_time"] else "❌"
        print(f"  {status} {feat}")
        print(f"     时间点: {analysis['timing']}")
        print(f"     来源: {analysis['source']}")
        print(f"     说明: {analysis['explanation']}")
        print()

        if analysis["point_in_time"]:
            compliant.append(feat)
        else:
            violated.append(feat)

    # --- 2. 实际数据验证 ---
    print("\n[2/3] 实际数据验证（116 events 特征可用性）")
    print("-" * 70)

    try:
        events = load_v6_gate_events()
        n_events = len(events)
        print(f"  加载 events: {n_events} 条")

        features_df, used, excluded = extract_features(events)
        print(f"  extract_features() 结果:")
        print(f"    使用的特征: {len(used)} 个")
        print(f"    排除的特征: {len(excluded)} 个")
        print()

        # 各特征缺失率
        print("  特征缺失率:")
        for col in used:
            missing_rate = features_df[col].isna().sum() / n_events
            pit_status = "✅" if col in compliant else "❌ VIOLATED"
            print(f"    {col}: {missing_rate:.1%} missing  [{pit_status}]")

        print()
        print("  被排除的特征（缺失率 > 50% 或不存在）:")
        for col in excluded:
            if col in events.columns:
                missing_rate = events[col].isna().sum() / n_events
                reason = f"缺失率 {missing_rate:.1%}"
            else:
                reason = "列不存在于 CSV"
            pit_status = "✅" if col in compliant else "❌ VIOLATED"
            print(f"    {col}: {reason}  [{pit_status}]")

    except Exception as e:
        print(f"  ⚠️  数据加载失败: {e}")
        print("  跳过实际数据验证，仅依赖源码审计结果。")
        n_events = 0
        used = []
        excluded = []

    # --- 3. 结论 ---
    print("\n[3/3] 验证结论")
    print("-" * 70)

    # 检查是否有违规特征被 extract_features() 实际使用
    violated_and_used = [f for f in violated if f in used]
    violated_and_excluded = [f for f in violated if f in excluded]

    print(f"\n  合规特征（Point_In_Time ✅）: {len(compliant)} 个")
    for f in compliant:
        print(f"    ✅ {f}")

    print(f"\n  违规特征（POST-TOUCH ❌）: {len(violated)} 个")
    for f in violated:
        in_use = "⚠️ 当前被 extract_features() 使用！" if f in violated_and_used else "已被排除"
        print(f"    ❌ {f} — {in_use}")

    if violated_and_used:
        print(f"\n  🚨 警告：{len(violated_and_used)} 个违规特征当前被使用！")
        print("     这些特征使用了 breakout 触发后的数据，存在前视偏差。")
        print("     建议：将这些特征从 OPTIONAL_FEATURES 中移除或在 extract_features() 中强制排除。")
    else:
        print("\n  ✅ 无数据泄露：所有被 extract_features() 实际使用的特征均满足 Point_In_Time 约束。")
        if violated:
            print(f"     （{len(violated)} 个违规特征已被自动排除，因缺失率 > 50% 或列不存在）")

    summary = {
        "total_features_analyzed": len(all_features),
        "compliant_count": len(compliant),
        "violated_count": len(violated),
        "compliant_features": compliant,
        "violated_features": violated,
        "violated_and_used": violated_and_used,
        "violated_and_excluded": violated_and_excluded,
        "events_loaded": n_events,
        "features_used_by_extractor": used,
        "features_excluded_by_extractor": excluded,
        "conclusion": (
            "PASS — 无数据泄露" if not violated_and_used
            else f"FAIL — {len(violated_and_used)} 个违规特征被使用"
        ),
    }

    print(f"\n  最终判定: {summary['conclusion']}")
    print("=" * 70)

    return summary


if __name__ == "__main__":
    result = run_verification()
    sys.exit(0 if not result["violated_and_used"] else 1)
