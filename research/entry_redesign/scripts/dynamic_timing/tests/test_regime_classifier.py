"""
regime_classifier 单元测试

验证：
- 每条 regime 规则的触发条件（7 条规则各一个测试）
- 优先级顺序（高优先级规则先匹配）
- max_steps 保底逻辑

Validates: Requirements 2.3, 2.6
"""

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from regime_classifier import classify, EntryDecision, TimingParams, DecisionResult
from feature_engine import StepFeatures


def _make_features(**kwargs) -> StepFeatures:
    """创建 StepFeatures，使用不触发任何高优先级规则的安全默认值。"""
    defaults = {
        "step_index": 1,
        "extension_atr": 0.05,
        "speed_cumulative_atr": 0.05,
        "max_extension_atr": 0.05,
        "pullback_from_max_atr": 0.05,
        "flow_imbalance_cumulative": 0.5,
        "flow_imbalance_last_step": 0.5,
        "speed_last_step_atr": 0.05,
        "continuation_ratio": 0.5,
        "dwell_ratio": 0.0,
    }
    defaults.update(kwargs)
    return StepFeatures(**defaults)


# ============================================================
# 1. 每条 regime 规则触发条件测试
# ============================================================


class TestRegimeRuleTriggers:
    """验证每条 regime 规则在满足条件时正确触发。"""

    def test_strong_momentum_triggers_immediate(self):
        """Strong Momentum: speed_cumulative_atr >= 0.15 AND flow >= 0.58 → IMMEDIATE"""
        features = _make_features(
            speed_cumulative_atr=0.20,
            flow_imbalance_cumulative=0.60,
        )
        params = TimingParams()
        result = classify(features, params)

        assert result.decision == EntryDecision.IMMEDIATE
        assert result.regime == "Strong Momentum"

    def test_weak_signal_triggers_skip(self):
        """Weak Signal: flow < 0.48 AND step_index >= 3 → SKIP"""
        features = _make_features(
            flow_imbalance_cumulative=0.40,
            step_index=3,
        )
        params = TimingParams()
        result = classify(features, params)

        assert result.decision == EntryDecision.SKIP
        assert result.regime == "Weak Signal"

    def test_over_extended_triggers_wait_pullback(self):
        """Over-Extended: extension_atr >= 0.15 AND pullback_from_max_atr <= 0.02 → WAIT_PULLBACK"""
        features = _make_features(
            extension_atr=0.20,
            pullback_from_max_atr=0.01,
        )
        params = TimingParams()
        result = classify(features, params)

        assert result.decision == EntryDecision.WAIT_PULLBACK
        assert result.regime == "Over-Extended"

    def test_fading_momentum_triggers_skip(self):
        """Fading Momentum: speed_last_step < 0.02 AND step >= 3 AND flow_last < 0.50 → SKIP"""
        features = _make_features(
            speed_last_step_atr=0.01,
            step_index=3,
            flow_imbalance_last_step=0.45,
        )
        params = TimingParams()
        result = classify(features, params)

        assert result.decision == EntryDecision.SKIP
        assert result.regime == "Fading Momentum"

    def test_moderate_momentum_triggers_immediate(self):
        """Moderate Momentum: speed >= 0.08 AND continuation_ratio >= 0.6 → IMMEDIATE"""
        features = _make_features(
            speed_cumulative_atr=0.10,
            continuation_ratio=0.7,
        )
        params = TimingParams()
        result = classify(features, params)

        assert result.decision == EntryDecision.IMMEDIATE
        assert result.regime == "Moderate Momentum"

    def test_developing_triggers_continue_observe(self):
        """Developing: step_index < max_steps → CONTINUE_OBSERVE"""
        features = _make_features(step_index=2)
        params = TimingParams()  # max_steps=4
        result = classify(features, params)

        assert result.decision == EntryDecision.CONTINUE_OBSERVE
        assert result.regime == "Developing"

    def test_default_triggers_immediate_at_max_steps(self):
        """Default: step_index == max_steps, no other rule matches → IMMEDIATE"""
        features = _make_features(step_index=4)
        params = TimingParams()  # max_steps=4
        result = classify(features, params)

        assert result.decision == EntryDecision.IMMEDIATE
        assert result.regime == "Default"


# ============================================================
# 2. 优先级顺序测试
# ============================================================


class TestPriorityOrder:
    """验证高优先级规则先匹配。"""

    def test_strong_momentum_beats_over_extended(self):
        """当同时满足 Strong Momentum 和 Over-Extended 条件时，Strong Momentum 优先。"""
        features = _make_features(
            # Strong Momentum 条件
            speed_cumulative_atr=0.20,
            flow_imbalance_cumulative=0.60,
            # Over-Extended 条件
            extension_atr=0.20,
            pullback_from_max_atr=0.01,
        )
        params = TimingParams()
        result = classify(features, params)

        assert result.decision == EntryDecision.IMMEDIATE
        assert result.regime == "Strong Momentum"

    def test_weak_signal_beats_fading_momentum(self):
        """当同时满足 Weak Signal 和 Fading Momentum 条件时，Weak Signal 优先（更高优先级）。"""
        features = _make_features(
            # Weak Signal 条件: flow < 0.48 AND step >= 3
            flow_imbalance_cumulative=0.40,
            step_index=3,
            # Fading Momentum 条件: speed_last < 0.02 AND step >= 3 AND flow_last < 0.50
            speed_last_step_atr=0.01,
            flow_imbalance_last_step=0.45,
        )
        params = TimingParams()
        result = classify(features, params)

        assert result.decision == EntryDecision.SKIP
        assert result.regime == "Weak Signal"


# ============================================================
# 3. max_steps 保底逻辑测试
# ============================================================


class TestMaxStepsFallback:
    """验证 max_steps 保底逻辑。"""

    def test_max_steps_reached_no_match_gives_default_immediate(self):
        """step_index == max_steps 且无其他规则匹配 → Default IMMEDIATE"""
        features = _make_features(step_index=4)
        params = TimingParams()  # max_steps=4
        result = classify(features, params)

        assert result.decision == EntryDecision.IMMEDIATE
        assert result.regime == "Default"
        assert result.step_index == 4

    def test_below_max_steps_no_match_gives_developing(self):
        """step_index < max_steps 且无其他规则匹配 → Developing CONTINUE_OBSERVE"""
        features = _make_features(step_index=2)
        params = TimingParams()  # max_steps=4
        result = classify(features, params)

        assert result.decision == EntryDecision.CONTINUE_OBSERVE
        assert result.regime == "Developing"
        assert result.step_index == 2
