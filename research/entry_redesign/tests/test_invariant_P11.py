"""Hypothesis property-based 测试：Entry Price Mode Fallback Explicit (P11).

**Validates: Requirements 6.11**

FOR ALL `post_touch_pullback_limit` 未成交事件：
  - 不出现在 ledger 成交行
  - attribution `pullback_limit_unfilled_count` 恰好 +1

使用 hypothesis 生成场景：post_touch_pullback_limit 模式下，在
[trigger_ts, trigger_ts + D] 窗口内没有任何 1s bar 满足成交条件
（long: 无 bar 的 low <= target_price；short: 无 bar 的 high >= target_price）。
验证 EntryPriceResolver 返回 filled=False 且
unfilled_reason="pullback_limit_not_touched_in_window"。
这确认了该事件不会进入 ledger 成交行。

Requirements: 6.11, 6.14
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timedelta, timezone
from typing import Literal, Sequence

from hypothesis import given, settings
from hypothesis import strategies as st

from research.entry_redesign.detector.entry_trigger_detector import (
    OneSecondBar,
    TriggerDecision,
)
from research.entry_redesign.price.entry_price_resolver import (
    EntryPriceResolver,
    PriceResolution,
)
from research.entry_redesign.snapshot.runner_parameter_snapshot import (
    Atr14Source,
    CostModelParams,
    FeatureSources,
    RunnerParameterSnapshot,
    SymbolFilters,
)
from research.entry_redesign.spec.entry_candidate_spec import EntryCandidateSpec


# ---------------------------------------------------------------------------
# 常量
# ---------------------------------------------------------------------------

_PULLBACK_MODES: list[str] = [
    "pullback_p000",
    "pullback_p002",
    "pullback_p005",
    "pullback_p010",
]

_PULLBACK_P_VALUES: dict[str, float] = {
    "pullback_p000": 0.00,
    "pullback_p002": 0.02,
    "pullback_p005": 0.05,
    "pullback_p010": 0.10,
}

# D 值域中 > 0 的值（D=0 时 pullback 窗口为零，也必然未成交）
_VALID_D_VALUES: list[int] = [0, 5, 15, 30, 60, 120]

_SYMBOLS: list[str] = ["BTCUSDT", "ETHUSDT"]
_SIDES: list[Literal["long", "short"]] = ["long", "short"]

_BASE_TS = datetime(2025, 6, 15, 12, 0, 0, tzinfo=timezone.utc)

_TICK_SIZE_BY_SYMBOL: dict[str, float] = {
    "BTCUSDT": 0.10,
    "ETHUSDT": 0.01,
}

_STEP_SIZE_BY_SYMBOL: dict[str, float] = {
    "BTCUSDT": 0.001,
    "ETHUSDT": 0.001,
}


# ---------------------------------------------------------------------------
# OneSecondBars 实现（用于测试）
# ---------------------------------------------------------------------------


@dataclass(frozen=True)
class SimpleOneSecondBars:
    """测试用 OneSecondBars 实现。"""

    bars: Sequence[OneSecondBar]


# ---------------------------------------------------------------------------
# 辅助函数：构建 RunnerParameterSnapshot
# ---------------------------------------------------------------------------


def _make_snapshot(
    *,
    entry_delay_seconds: int,
    entry_price_mode_id: str,
) -> RunnerParameterSnapshot:
    """构建一个最小化的 RunnerParameterSnapshot 用于测试。"""
    # H <= D 约束：H=0 总是合法的
    candidate = EntryCandidateSpec(
        entry_delay_seconds=entry_delay_seconds,
        feature_horizon_seconds=0,
        trigger_confirmation_id="none",
        entry_price_mode_id=entry_price_mode_id,  # type: ignore[arg-type]
        pretouch_state_band_id="none",
        posttouch_quality_band_id="none",
    )
    return RunnerParameterSnapshot(
        candidate=candidate,
        cost_model_baseline=CostModelParams(
            slip_bps_per_side=2.0, entry_bps=2.0, exit_bps=4.0
        ),
        cost_model_stress=CostModelParams(
            slip_bps_per_side=2.0, entry_bps=4.0, exit_bps=4.0
        ),
        seed=42,
        git_commit_sha="a" * 40,
        events_source_path="research/events_execution_labeled.csv",
        events_source_sha256="b" * 64,
        runner_version="entry_redesign_runner_v1.0.0",
        symbol_filters=SymbolFilters(
            tick_size_by_symbol=_TICK_SIZE_BY_SYMBOL,
            step_size_by_symbol=_STEP_SIZE_BY_SYMBOL,
        ),
        features=FeatureSources(
            atr14_source=Atr14Source(
                path="research/features/atr14.csv",
                sha256="c" * 64,
            )
        ),
    )


# ---------------------------------------------------------------------------
# Hypothesis strategies
# ---------------------------------------------------------------------------


@st.composite
def unfilled_pullback_scenarios(draw: st.DrawFn) -> dict:
    """生成 post_touch_pullback_limit 必然未成交的场景。

    策略：
      1. 随机选择 pullback mode (p000/p002/p005/p010)
      2. 随机选择 D (entry_delay_seconds)
      3. 随机选择 side (long/short)
      4. 随机选择 symbol
      5. 随机生成 trigger level 和 ATR
      6. 生成 [trigger_ts, trigger_ts + D] 窗口内的 1s bars，
         确保所有 bar 都不满足成交条件：
         - long: 所有 bar 的 low > target_price (= level - p * ATR)
         - short: 所有 bar 的 high < target_price (= level + p * ATR)
    """
    mode = draw(st.sampled_from(_PULLBACK_MODES))
    p = _PULLBACK_P_VALUES[mode]
    d = draw(st.sampled_from(_VALID_D_VALUES))
    side: Literal["long", "short"] = draw(st.sampled_from(_SIDES))
    symbol = draw(st.sampled_from(_SYMBOLS))

    # 生成合理的 trigger level 和 ATR
    level = draw(
        st.floats(min_value=100.0, max_value=100000.0, allow_nan=False, allow_infinity=False)
    )
    atr14 = draw(
        st.floats(min_value=0.01, max_value=1000.0, allow_nan=False, allow_infinity=False)
    )

    # 计算 target_price
    if side == "long":
        target_price = level - p * atr14
    else:
        target_price = level + p * atr14

    # 生成窗口内的 1s bars（0 到 max(d, 1) 根）
    # 确保所有 bar 都不满足成交条件
    max_bars = min(d, 30) if d > 0 else 0  # 限制 bar 数量以保持测试速度
    num_bars = draw(st.integers(min_value=0, max_value=max(max_bars, 0)))

    trigger_ts = _BASE_TS
    bars: list[OneSecondBar] = []

    for i in range(num_bars):
        bar_close_ts = trigger_ts + timedelta(seconds=i + 1)

        if side == "long":
            # long pullback 需要 low <= target_price 才成交
            # 确保不成交：所有 bar 的 low > target_price
            # 生成 low 严格大于 target_price
            bar_low = draw(
                st.floats(
                    min_value=target_price + 0.0001,
                    max_value=target_price + atr14 * 2 + 1.0,
                    allow_nan=False,
                    allow_infinity=False,
                )
            )
            bar_high = draw(
                st.floats(
                    min_value=bar_low,
                    max_value=bar_low + atr14 + 1.0,
                    allow_nan=False,
                    allow_infinity=False,
                )
            )
        else:
            # short pullback 需要 high >= target_price 才成交
            # 确保不成交：所有 bar 的 high < target_price
            bar_high = draw(
                st.floats(
                    min_value=max(target_price - atr14 * 2 - 1.0, 0.01),
                    max_value=target_price - 0.0001,
                    allow_nan=False,
                    allow_infinity=False,
                )
            )
            bar_low = draw(
                st.floats(
                    min_value=max(bar_high - atr14 - 1.0, 0.001),
                    max_value=bar_high,
                    allow_nan=False,
                    allow_infinity=False,
                )
            )

        bars.append(
            OneSecondBar(
                open_ts=bar_close_ts - timedelta(seconds=1),
                close_ts=bar_close_ts,
                high=bar_high,
                low=bar_low,
            )
        )

    return {
        "mode": mode,
        "d": d,
        "side": side,
        "symbol": symbol,
        "level": level,
        "atr14": atr14,
        "target_price": target_price,
        "trigger_ts": trigger_ts,
        "bars": bars,
    }


# ---------------------------------------------------------------------------
# Property-based tests
# ---------------------------------------------------------------------------


@given(scenario=unfilled_pullback_scenarios())
@settings(max_examples=500)
def test_p11_unfilled_pullback_not_in_ledger(scenario: dict) -> None:
    """P11: post_touch_pullback_limit 未成交事件 MUST NOT 出现在 ledger 成交行。

    **Validates: Requirements 6.11**

    FOR ALL post_touch_pullback_limit 未成交事件：
      - EntryPriceResolver.resolve() 返回 filled=False
      - unfilled_reason == "pullback_limit_not_touched_in_window"
      - entry_price 为 None
      - entry_ts 为 None

    这确认了该事件不会进入 Research_Ledger CSV 的成交行中。
    """
    mode: str = scenario["mode"]
    d: int = scenario["d"]
    side: Literal["long", "short"] = scenario["side"]
    symbol: str = scenario["symbol"]
    level: float = scenario["level"]
    atr14: float = scenario["atr14"]
    trigger_ts: datetime = scenario["trigger_ts"]
    bars: list[OneSecondBar] = scenario["bars"]

    # 构建 snapshot 和 resolver
    snapshot = _make_snapshot(entry_delay_seconds=d, entry_price_mode_id=mode)
    resolver = EntryPriceResolver(snapshot)
    resolver.set_context(atr14=atr14, symbol=symbol)

    # 构建 trigger decision
    trigger = TriggerDecision(
        trigger_ts=trigger_ts,
        side=side,
        level=level,
    )

    # 构建 onesec_window
    onesec_window = SimpleOneSecondBars(bars=bars)

    # 调用 resolve
    result: PriceResolution = resolver.resolve(trigger, onesec_window)

    # P11 断言：未成交事件不进 ledger
    assert result.filled is False, (
        f"P11 violated: post_touch_pullback_limit should NOT fill, "
        f"but got filled=True. "
        f"mode={mode}, D={d}, side={side}, symbol={symbol}, "
        f"level={level}, atr14={atr14}, target_price={scenario['target_price']}, "
        f"num_bars={len(bars)}"
    )

    assert result.unfilled_reason == "pullback_limit_not_touched_in_window", (
        f"P11 violated: unfilled_reason should be "
        f"'pullback_limit_not_touched_in_window', "
        f"got {result.unfilled_reason!r}. "
        f"mode={mode}, D={d}, side={side}"
    )

    assert result.entry_price is None, (
        f"P11 violated: entry_price should be None for unfilled event, "
        f"got {result.entry_price}. mode={mode}, D={d}, side={side}"
    )

    assert result.entry_ts is None, (
        f"P11 violated: entry_ts should be None for unfilled event, "
        f"got {result.entry_ts}. mode={mode}, D={d}, side={side}"
    )


@given(scenario=unfilled_pullback_scenarios())
@settings(max_examples=500)
def test_p11_unfilled_increments_attribution_counter(scenario: dict) -> None:
    """P11: 未成交事件 MUST 使 attribution pullback_limit_unfilled_count +1。

    **Validates: Requirements 6.11**

    FOR ALL post_touch_pullback_limit 未成交事件：
      - 每次 resolve() 返回 filled=False 时，
        attribution 的 pullback_limit_unfilled_count 应恰好 +1。

    本测试模拟 pipeline 中的计数逻辑：
      - 初始 counter = 0
      - 每次 resolve() 返回 filled=False → counter += 1
      - 验证 counter 恰好等于未成交事件数
    """
    mode: str = scenario["mode"]
    d: int = scenario["d"]
    side: Literal["long", "short"] = scenario["side"]
    symbol: str = scenario["symbol"]
    level: float = scenario["level"]
    atr14: float = scenario["atr14"]
    trigger_ts: datetime = scenario["trigger_ts"]
    bars: list[OneSecondBar] = scenario["bars"]

    # 构建 snapshot 和 resolver
    snapshot = _make_snapshot(entry_delay_seconds=d, entry_price_mode_id=mode)
    resolver = EntryPriceResolver(snapshot)
    resolver.set_context(atr14=atr14, symbol=symbol)

    # 构建 trigger decision
    trigger = TriggerDecision(
        trigger_ts=trigger_ts,
        side=side,
        level=level,
    )

    # 构建 onesec_window
    onesec_window = SimpleOneSecondBars(bars=bars)

    # 模拟 pipeline 中的 attribution 计数逻辑
    pullback_limit_unfilled_count = 0
    ledger_entries: list[PriceResolution] = []

    result = resolver.resolve(trigger, onesec_window)

    if not result.filled:
        # pipeline 逻辑：未成交 → counter +1，不落 ledger
        pullback_limit_unfilled_count += 1
    else:
        # pipeline 逻辑：成交 → 落 ledger
        ledger_entries.append(result)

    # P11 断言：未成交事件 counter 恰好 +1
    assert pullback_limit_unfilled_count == 1, (
        f"P11 violated: pullback_limit_unfilled_count should be 1, "
        f"got {pullback_limit_unfilled_count}. "
        f"mode={mode}, D={d}, side={side}, filled={result.filled}"
    )

    # P11 断言：未成交事件不进 ledger
    assert len(ledger_entries) == 0, (
        f"P11 violated: unfilled event should NOT appear in ledger, "
        f"but ledger has {len(ledger_entries)} entries. "
        f"mode={mode}, D={d}, side={side}"
    )
