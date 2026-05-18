"""Hypothesis property-based 测试：Intrabar Breakout Semantic Preserved (P1).

**Validates: Requirements 6.1**

FOR ALL 合成 (signal_bar, 1s_window)：
  - long 条件成立 ⇔ trigger_ts 非空且等于最早满足条件的 1s bar 结束时刻；
    不成立 ⇔ 返回 None
  - short 镜像

反例 dump 到 research/tmp_entry_redesign_invariant_counterexamples/P1_<candidate_id>.json

Requirements: 6.1, 6.14
"""

from __future__ import annotations

import json
import os
import pathlib
from dataclasses import dataclass
from datetime import datetime, timedelta, timezone
from typing import Literal, Sequence

from hypothesis import given, settings
from hypothesis import strategies as st

from research.entry_redesign.detector.entry_trigger_detector import (
    EntryTriggerDetector,
    OneSecondBar,
    TriggerDecision,
)

# ---------------------------------------------------------------------------
# Counterexample dump directory
# ---------------------------------------------------------------------------

_COUNTEREXAMPLE_DIR = pathlib.Path(
    "research/tmp_entry_redesign_invariant_counterexamples"
)


def _dump_counterexample(
    candidate_id: str,
    description: str,
    event_data: dict,
    bars_data: list[dict],
    result: object,
    expected: str,
) -> None:
    """Dump a P1 counterexample to JSON file."""
    _COUNTEREXAMPLE_DIR.mkdir(parents=True, exist_ok=True)
    out_path = _COUNTEREXAMPLE_DIR / f"P1_{candidate_id}.json"
    payload = {
        "property": "P1",
        "description": description,
        "event": event_data,
        "bars": bars_data,
        "actual_result": str(result),
        "expected": expected,
    }
    with open(out_path, "w", encoding="utf-8", newline="\n") as f:
        json.dump(payload, f, indent=2, ensure_ascii=False, default=str)
        f.write("\n")


# ---------------------------------------------------------------------------
# Minimal Event / OneSecondBars implementations for testing
# ---------------------------------------------------------------------------


@dataclass(frozen=True)
class _TestEvent:
    """Minimal Event implementation for P1 testing."""

    symbol: str
    side: Literal["long", "short"]
    signal_bar_start_ts: datetime
    signal_bar_end_ts: datetime
    prev_high_2: float
    prev_low_2: float


@dataclass(frozen=True)
class _TestOneSecondBars:
    """Minimal OneSecondBars implementation for P1 testing."""

    bars: Sequence[OneSecondBar]


# ---------------------------------------------------------------------------
# Hypothesis strategies
# ---------------------------------------------------------------------------

_BASE_TS = datetime(2025, 6, 1, 0, 0, 0, tzinfo=timezone.utc)
_SYMBOLS = ["BTCUSDT", "ETHUSDT"]
_TICK_SIZE_BY_SYMBOL = {"BTCUSDT": 0.10, "ETHUSDT": 0.01}


@st.composite
def onesec_bars_list(draw: st.DrawFn) -> list[OneSecondBar]:
    """Generate a list of 1-second bars (0 to 20 bars) with random highs/lows.

    Bars are chronologically ordered with non-overlapping timestamps.
    """
    n_bars = draw(st.integers(min_value=0, max_value=20))
    bars: list[OneSecondBar] = []
    for i in range(n_bars):
        # Each bar is 1 second apart
        open_ts = _BASE_TS + timedelta(seconds=i)
        close_ts = open_ts + timedelta(seconds=1)
        high = draw(
            st.floats(min_value=1.0, max_value=200000.0, allow_nan=False, allow_infinity=False)
        )
        low = draw(
            st.floats(min_value=1.0, max_value=200000.0, allow_nan=False, allow_infinity=False)
        )
        # Ensure low <= high (realistic bar constraint)
        if low > high:
            low, high = high, low
        bars.append(OneSecondBar(open_ts=open_ts, close_ts=close_ts, high=high, low=low))
    return bars


@st.composite
def long_signal_bar_and_window(draw: st.DrawFn) -> tuple[_TestEvent, _TestOneSecondBars]:
    """Generate a synthetic (signal_bar, 1s_window) pair for long side.

    prev_high_2 is drawn randomly; 1s bars have random highs.
    """
    symbol = draw(st.sampled_from(_SYMBOLS))
    prev_high_2 = draw(
        st.floats(min_value=10.0, max_value=100000.0, allow_nan=False, allow_infinity=False)
    )
    prev_low_2 = draw(
        st.floats(min_value=1.0, max_value=prev_high_2, allow_nan=False, allow_infinity=False)
    )
    bars = draw(onesec_bars_list())

    signal_bar_start_ts = _BASE_TS - timedelta(hours=1)
    signal_bar_end_ts = _BASE_TS + timedelta(hours=1)

    event = _TestEvent(
        symbol=symbol,
        side="long",
        signal_bar_start_ts=signal_bar_start_ts,
        signal_bar_end_ts=signal_bar_end_ts,
        prev_high_2=prev_high_2,
        prev_low_2=prev_low_2,
    )
    window = _TestOneSecondBars(bars=bars)
    return event, window


@st.composite
def short_signal_bar_and_window(draw: st.DrawFn) -> tuple[_TestEvent, _TestOneSecondBars]:
    """Generate a synthetic (signal_bar, 1s_window) pair for short side.

    prev_low_2 is drawn randomly; 1s bars have random lows.
    """
    symbol = draw(st.sampled_from(_SYMBOLS))
    prev_low_2 = draw(
        st.floats(min_value=10.0, max_value=100000.0, allow_nan=False, allow_infinity=False)
    )
    prev_high_2 = draw(
        st.floats(min_value=prev_low_2, max_value=200000.0, allow_nan=False, allow_infinity=False)
    )
    bars = draw(onesec_bars_list())

    signal_bar_start_ts = _BASE_TS - timedelta(hours=1)
    signal_bar_end_ts = _BASE_TS + timedelta(hours=1)

    event = _TestEvent(
        symbol=symbol,
        side="short",
        signal_bar_start_ts=signal_bar_start_ts,
        signal_bar_end_ts=signal_bar_end_ts,
        prev_high_2=prev_high_2,
        prev_low_2=prev_low_2,
    )
    window = _TestOneSecondBars(bars=bars)
    return event, window


# ---------------------------------------------------------------------------
# Helper: compute expected trigger for verification
# ---------------------------------------------------------------------------


def _expected_long_trigger(
    bars: Sequence[OneSecondBar], prev_high_2: float
) -> TriggerDecision | None:
    """Independently compute expected long trigger result.

    Returns TriggerDecision with trigger_ts = earliest bar.close_ts where
    bar.high >= prev_high_2, or None if no such bar exists.
    """
    for bar in bars:
        if bar.high >= prev_high_2:
            return TriggerDecision(
                trigger_ts=bar.close_ts,
                side="long",
                level=prev_high_2,
            )
    return None


def _expected_short_trigger(
    bars: Sequence[OneSecondBar], prev_low_2: float
) -> TriggerDecision | None:
    """Independently compute expected short trigger result.

    Returns TriggerDecision with trigger_ts = earliest bar.close_ts where
    bar.low <= prev_low_2, or None if no such bar exists.
    """
    for bar in bars:
        if bar.low <= prev_low_2:
            return TriggerDecision(
                trigger_ts=bar.close_ts,
                side="short",
                level=prev_low_2,
            )
    return None


# ---------------------------------------------------------------------------
# Helper: serialize for counterexample dump
# ---------------------------------------------------------------------------


def _event_to_dict(event: _TestEvent) -> dict:
    return {
        "symbol": event.symbol,
        "side": event.side,
        "signal_bar_start_ts": event.signal_bar_start_ts.isoformat(),
        "signal_bar_end_ts": event.signal_bar_end_ts.isoformat(),
        "prev_high_2": event.prev_high_2,
        "prev_low_2": event.prev_low_2,
    }


def _bars_to_list(bars: Sequence[OneSecondBar]) -> list[dict]:
    return [
        {
            "open_ts": b.open_ts.isoformat(),
            "close_ts": b.close_ts.isoformat(),
            "high": b.high,
            "low": b.low,
        }
        for b in bars
    ]


# ---------------------------------------------------------------------------
# Property-based tests
# ---------------------------------------------------------------------------

_detector = EntryTriggerDetector(tick_size_by_symbol=_TICK_SIZE_BY_SYMBOL)

# Counter for generating unique counterexample filenames
_counterexample_counter = 0


def _next_candidate_id() -> str:
    """Generate a unique candidate_id for counterexample dumps."""
    global _counterexample_counter
    _counterexample_counter += 1
    return f"p1_test_{_counterexample_counter:06d}"


@given(data=long_signal_bar_and_window())
@settings(max_examples=1000)
def test_p1_long_trigger_semantic(
    data: tuple[_TestEvent, _TestOneSecondBars],
) -> None:
    """P1: Intrabar Breakout Semantic Preserved — long side.

    **Validates: Requirements 6.1**

    FOR ALL 合成 (signal_bar, 1s_window) with side=long:
      - 若存在至少一个 1s bar 的 high >= prev_high_2，
        则 detect() 返回 TriggerDecision，trigger_ts 等于最早满足条件的
        1s bar 结束时刻。
      - 若不存在这样的 bar，则 detect() 返回 None。
    """
    event, window = data
    actual = _detector.detect(event, window)
    expected = _expected_long_trigger(window.bars, event.prev_high_2)

    if expected is None:
        # No bar satisfies condition → detect() MUST return None
        if actual is not None:
            _dump_counterexample(
                candidate_id=_next_candidate_id(),
                description=(
                    "P1 spurious trigger: no 1s bar has high >= prev_high_2, "
                    "but detect() returned a non-None TriggerDecision"
                ),
                event_data=_event_to_dict(event),
                bars_data=_bars_to_list(window.bars),
                result=actual,
                expected="None",
            )
            assert actual is None, (
                f"P1 spurious trigger (long): no bar.high >= prev_high_2 "
                f"({event.prev_high_2}), but got trigger_ts={actual.trigger_ts}. "
                f"Bars highs: {[b.high for b in window.bars]}"
            )
    else:
        # At least one bar satisfies → detect() MUST return TriggerDecision
        if actual is None:
            _dump_counterexample(
                candidate_id=_next_candidate_id(),
                description=(
                    "P1 missing trigger: at least one 1s bar has high >= prev_high_2, "
                    "but detect() returned None"
                ),
                event_data=_event_to_dict(event),
                bars_data=_bars_to_list(window.bars),
                result=actual,
                expected=str(expected),
            )
            assert actual is not None, (
                f"P1 missing trigger (long): bar.high >= prev_high_2 "
                f"({event.prev_high_2}) exists, but detect() returned None. "
                f"Bars highs: {[b.high for b in window.bars]}"
            )

        # trigger_ts MUST equal earliest satisfying bar's close_ts
        if actual.trigger_ts != expected.trigger_ts:
            _dump_counterexample(
                candidate_id=_next_candidate_id(),
                description=(
                    "P1 wrong trigger_ts: detect() returned a trigger but "
                    "trigger_ts does not match the earliest satisfying bar's close_ts"
                ),
                event_data=_event_to_dict(event),
                bars_data=_bars_to_list(window.bars),
                result=actual,
                expected=str(expected),
            )
            assert actual.trigger_ts == expected.trigger_ts, (
                f"P1 wrong trigger_ts (long): expected {expected.trigger_ts}, "
                f"got {actual.trigger_ts}. prev_high_2={event.prev_high_2}, "
                f"bars highs: {[b.high for b in window.bars]}"
            )

        # side MUST be "long"
        assert actual.side == "long", (
            f"P1 wrong side: expected 'long', got '{actual.side}'"
        )

        # level MUST be prev_high_2
        assert actual.level == event.prev_high_2, (
            f"P1 wrong level: expected {event.prev_high_2}, got {actual.level}"
        )


@given(data=short_signal_bar_and_window())
@settings(max_examples=1000)
def test_p1_short_trigger_semantic(
    data: tuple[_TestEvent, _TestOneSecondBars],
) -> None:
    """P1: Intrabar Breakout Semantic Preserved — short side.

    **Validates: Requirements 6.1**

    FOR ALL 合成 (signal_bar, 1s_window) with side=short:
      - 若存在至少一个 1s bar 的 low <= prev_low_2，
        则 detect() 返回 TriggerDecision，trigger_ts 等于最早满足条件的
        1s bar 结束时刻。
      - 若不存在这样的 bar，则 detect() 返回 None。
    """
    event, window = data
    actual = _detector.detect(event, window)
    expected = _expected_short_trigger(window.bars, event.prev_low_2)

    if expected is None:
        # No bar satisfies condition → detect() MUST return None
        if actual is not None:
            _dump_counterexample(
                candidate_id=_next_candidate_id(),
                description=(
                    "P1 spurious trigger: no 1s bar has low <= prev_low_2, "
                    "but detect() returned a non-None TriggerDecision"
                ),
                event_data=_event_to_dict(event),
                bars_data=_bars_to_list(window.bars),
                result=actual,
                expected="None",
            )
            assert actual is None, (
                f"P1 spurious trigger (short): no bar.low <= prev_low_2 "
                f"({event.prev_low_2}), but got trigger_ts={actual.trigger_ts}. "
                f"Bars lows: {[b.low for b in window.bars]}"
            )
    else:
        # At least one bar satisfies → detect() MUST return TriggerDecision
        if actual is None:
            _dump_counterexample(
                candidate_id=_next_candidate_id(),
                description=(
                    "P1 missing trigger: at least one 1s bar has low <= prev_low_2, "
                    "but detect() returned None"
                ),
                event_data=_event_to_dict(event),
                bars_data=_bars_to_list(window.bars),
                result=actual,
                expected=str(expected),
            )
            assert actual is not None, (
                f"P1 missing trigger (short): bar.low <= prev_low_2 "
                f"({event.prev_low_2}) exists, but detect() returned None. "
                f"Bars lows: {[b.low for b in window.bars]}"
            )

        # trigger_ts MUST equal earliest satisfying bar's close_ts
        if actual.trigger_ts != expected.trigger_ts:
            _dump_counterexample(
                candidate_id=_next_candidate_id(),
                description=(
                    "P1 wrong trigger_ts: detect() returned a trigger but "
                    "trigger_ts does not match the earliest satisfying bar's close_ts"
                ),
                event_data=_event_to_dict(event),
                bars_data=_bars_to_list(window.bars),
                result=actual,
                expected=str(expected),
            )
            assert actual.trigger_ts == expected.trigger_ts, (
                f"P1 wrong trigger_ts (short): expected {expected.trigger_ts}, "
                f"got {actual.trigger_ts}. prev_low_2={event.prev_low_2}, "
                f"bars lows: {[b.low for b in window.bars]}"
            )

        # side MUST be "short"
        assert actual.side == "short", (
            f"P1 wrong side: expected 'short', got '{actual.side}'"
        )

        # level MUST be prev_low_2
        assert actual.level == event.prev_low_2, (
            f"P1 wrong level: expected {event.prev_low_2}, got {actual.level}"
        )
