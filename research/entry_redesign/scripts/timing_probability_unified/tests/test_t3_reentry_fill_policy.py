"""Tests for strict reentry fill policy used by lifecycle research replay."""

import numpy as np
import pytest

import eth_q1_breakout_t3_shape_compare as lifecycle


def test_historical_reentry_fill_allows_same_second_anchor_fill():
    triggered, price, reason = lifecycle._reentry_fill_triggered(
        side="long",
        trigger_mode="reclaim",
        high_value=105.0,
        low_value=99.0,
        close_value=104.0,
        prev_close_value=100.0,
        re_p=101.0,
        policy="historical",
        current_pos=10,
        trigger_pos=10,
        trigger_kind="zero_initial",
    )

    assert triggered is True
    assert price == pytest.approx(101.0)
    assert reason == ""


def test_strict_reentry_fill_rejects_same_second_zero_initial():
    triggered, price, reason = lifecycle._reentry_fill_triggered(
        side="long",
        trigger_mode="reclaim",
        high_value=105.0,
        low_value=99.0,
        close_value=104.0,
        prev_close_value=100.0,
        re_p=101.0,
        policy="strict_next_second_cross",
        current_pos=10,
        trigger_pos=10,
        trigger_kind="zero_initial",
    )

    assert triggered is False
    assert np.isnan(price)
    assert reason == "same_second_zero_initial"


def test_strict_reentry_fill_requires_long_reclaim_cross():
    triggered, price, reason = lifecycle._reentry_fill_triggered(
        side="long",
        trigger_mode="reclaim",
        high_value=101.5,
        low_value=100.2,
        close_value=101.2,
        prev_close_value=100.5,
        re_p=101.0,
        policy="strict_next_second_cross",
        current_pos=11,
        trigger_pos=10,
        trigger_kind="zero_initial",
    )

    assert triggered is True
    assert price == pytest.approx(101.0)
    assert reason == ""


def test_strict_reentry_fill_rejects_long_when_already_above_anchor():
    triggered, price, reason = lifecycle._reentry_fill_triggered(
        side="long",
        trigger_mode="reclaim",
        high_value=103.0,
        low_value=101.2,
        close_value=102.0,
        prev_close_value=101.5,
        re_p=101.0,
        policy="strict_next_second_cross",
        current_pos=11,
        trigger_pos=10,
        trigger_kind="zero_initial",
    )

    assert triggered is False
    assert np.isnan(price)
    assert reason == "no_reclaim_cross"


def test_strict_reentry_fill_requires_short_reclaim_cross():
    triggered, price, reason = lifecycle._reentry_fill_triggered(
        side="short",
        trigger_mode="reclaim",
        high_value=100.8,
        low_value=99.5,
        close_value=99.8,
        prev_close_value=100.5,
        re_p=100.0,
        policy="strict_next_second_cross",
        current_pos=11,
        trigger_pos=10,
        trigger_kind="exit",
    )

    assert triggered is True
    assert price == pytest.approx(100.0)
    assert reason == ""


def test_reentry_fill_policy_rejects_unknown_policy():
    with pytest.raises(ValueError, match="unknown reentry_fill_policy"):
        lifecycle._normalize_reentry_fill_policy("optimistic")
