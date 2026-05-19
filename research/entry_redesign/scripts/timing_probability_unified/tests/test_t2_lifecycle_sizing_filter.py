"""Tests for research-only lifecycle sizing filters."""

from __future__ import annotations

import pandas as pd

import eth_q1_breakout_t3_shape_compare as lifecycle


def _seconds() -> pd.DataFrame:
    index = pd.date_range("2026-01-01T00:00:00Z", periods=2, freq="1s")
    return pd.DataFrame(
        {
            "open": [100.0, 100.0],
            "high": [102.0, 102.0],
            "low": [99.0, 99.0],
            "close": [101.0, 101.0],
            "volume": [1.0, 1.0],
        },
        index=index,
    )


def _signal() -> pd.DataFrame:
    index = pd.DatetimeIndex(
        [
            pd.Timestamp("2026-01-01T00:00:00Z"),
            pd.Timestamp("2026-01-01T00:00:01Z"),
        ]
    )
    rows = [
        {
            "open": 100.0,
            "high": 102.0,
            "low": 99.0,
            "close": 105.0,
            "ma5": 100.0,
            "sma5": 100.0,
            "sma5_slope": 1.0,
            "atr": 10.0,
            "atr_percentile": 80.0,
            "prev_high_1": 100.0,
            "prev_high_2": 101.0,
            "prev_high_3": 99.0,
            "prev_low_1": 99.0,
            "prev_low_2": 98.0,
            "prev_low_3": 97.0,
            "prev_close_1": 104.0,
            "prev_close_2": 103.0,
            "prev_close_3": 102.0,
            "prev_close_4": 100.0,
            "prev_close_12": 90.0,
        },
        {
            "open": 101.0,
            "high": 102.0,
            "low": 99.0,
            "close": 101.0,
            "ma5": 100.0,
            "sma5": 100.0,
            "sma5_slope": 1.0,
            "atr": 10.0,
            "atr_percentile": 80.0,
            "prev_high_1": 100.0,
            "prev_high_2": 101.0,
            "prev_high_3": 99.0,
            "prev_low_1": 99.0,
            "prev_low_2": 98.0,
            "prev_low_3": 97.0,
            "prev_close_1": 104.0,
            "prev_close_2": 103.0,
            "prev_close_3": 102.0,
            "prev_close_4": 100.0,
            "prev_close_12": 90.0,
        },
    ]
    return pd.DataFrame(rows, index=index)


def test_original_t2_sizing_filter_scales_reentry_notional_without_rejecting_lock():
    ledger, diagnostics = lifecycle.run_second_bar_replay(
        _seconds(),
        _signal(),
        initial_balance=10_000.0,
        breakout_shape="baseline_plus_t3",
        replay_mode="same_bar_parity",
        shape_sizing_filters={"max_atr_percentile": 40.0},
        sizing_filter_shapes=["original_t2"],
        sizing_filter_fail_multiplier=0.25,
    )

    entries = ledger[ledger["type"].isin(["BUY", "SHORT"])]
    assert len(entries) == 1
    entry = entries.iloc[0]
    assert entry["breakout_shape_name"] == "original_t2"
    assert entry["reason"] == "Zero-Initial-Reentry"
    assert entry["size_multiplier"] == 0.25
    assert entry["notional"] == 500.0
    assert entry["side"] == "long"
    assert entry["signal_start"] == pd.Timestamp("2026-01-01T00:00:00Z")
    assert entry["breakout_pre_touch_seconds"] == 0.0
    assert entry["atr_percentile"] == 80.0
    assert entry["ctx4h_side_return_atr"] == 0.4
    assert entry["ctx12h_side_return_atr"] == 1.4
    assert entry["sizing_reject_reason"] == "atr_percentile_high"
    assert diagnostics["breakout_locks"]["long"]["original_t2"] == 1
    assert diagnostics["shape_sizing_filter_fails"]["long"]["atr_percentile_high"] == 1


def test_original_t2_sizing_filter_can_skip_failed_lock():
    ledger, diagnostics = lifecycle.run_second_bar_replay(
        _seconds(),
        _signal(),
        initial_balance=10_000.0,
        breakout_shape="baseline_plus_t3",
        replay_mode="same_bar_parity",
        shape_sizing_filters={"max_atr_percentile": 40.0},
        sizing_filter_shapes=["original_t2"],
        sizing_filter_fail_multiplier=0.0,
        sizing_filter_fail_action="skip_lock",
    )

    assert ledger.empty
    assert diagnostics["breakout_locks"]["long"].get("original_t2", 0) == 0
    assert diagnostics["shape_sizing_filter_fails"]["long"]["atr_percentile_high"] == 1


def test_ctx_side_return_filter_uses_side_normalized_prior_move():
    sig = _signal().iloc[0]
    assert lifecycle._ctx_side_return_atr(sig, "long", 4) == 0.4
    assert lifecycle._ctx_side_return_atr(sig, "short", 4) == -0.4


def test_sizing_filter_can_require_separate_12h_context():
    ledger, diagnostics = lifecycle.run_second_bar_replay(
        _seconds(),
        _signal(),
        initial_balance=10_000.0,
        breakout_shape="baseline_plus_t3",
        replay_mode="same_bar_parity",
        shape_sizing_filters={"min_ctx12h_side_return_atr": 2.0},
        sizing_filter_shapes=["original_t2"],
        sizing_filter_fail_multiplier=0.0,
        sizing_filter_fail_action="skip_lock",
    )

    assert ledger.empty
    assert diagnostics["breakout_locks"]["long"].get("original_t2", 0) == 0
    assert diagnostics["shape_sizing_filter_fails"]["long"]["ctx12h_side_return_atr_low"] == 1


def test_external_breakout_event_can_enter_strict_lifecycle_with_native_t2_disabled():
    seconds = _seconds()
    seconds.loc[seconds.index[0], "close"] = 99.0
    seconds.loc[seconds.index[1], "open"] = 99.0
    ledger, diagnostics = lifecycle.run_second_bar_replay(
        seconds,
        _signal(),
        initial_balance=10_000.0,
        breakout_shape="baseline_plus_t3",
        replay_mode="same_bar_parity",
        shape_sizing_filters={"allowed_sides": []},
        sizing_filter_shapes=["original_t2"],
        sizing_filter_fail_multiplier=0.0,
        sizing_filter_fail_action="skip_lock",
        external_breakout_events=[
            {
                "symbol": "ETHUSDT",
                "event_key": "ETHUSDT|2026-01-01T00:00:00+00:00|2026-01-01T00:00:00+00:00|long",
                "signal_start": "2026-01-01T00:00:00Z",
                "touch_time": "2026-01-01T00:00:00Z",
                "side": "long",
                "level": 101.0,
                "context_combo_spec": "low_eff_rf_rank_median_000",
                "rf_probability": 0.63,
                "context_model_probability": 0.58,
                "speed_300s_atr": 0.31,
                "eff_300s": 0.72,
            }
        ],
        external_breakout_shape_name="low_eff_rf_rank_median_000",
        reentry_fill_policy="strict_next_second_cross",
    )

    entries = ledger[ledger["type"].isin(["BUY", "SHORT"])]
    assert len(entries) == 1
    entry = entries.iloc[0]
    assert entry["breakout_shape_name"] == "low_eff_rf_rank_median_000"
    assert entry["external_event_key"] == "ETHUSDT|2026-01-01T00:00:00+00:00|2026-01-01T00:00:00+00:00|long"
    assert entry["rf_probability"] == 0.63
    assert diagnostics["breakout_locks"]["long"].get("original_t2", 0) == 0
    assert diagnostics["external_breakout_locks"]["long"]["low_eff_rf_rank_median_000"] == 1


def test_external_breakout_event_next_second_open_enters_without_reentry_cross():
    seconds = _seconds()
    seconds.loc[seconds.index[0], "close"] = 99.0
    seconds.loc[seconds.index[1], ["open", "high", "low", "close"]] = [103.0, 103.0, 103.0, 103.0]

    ledger, diagnostics = lifecycle.run_second_bar_replay(
        seconds,
        _signal(),
        initial_balance=10_000.0,
        breakout_shape="baseline_plus_t3",
        replay_mode="same_bar_parity",
        shape_sizing_filters={"allowed_sides": []},
        sizing_filter_shapes=["original_t2"],
        sizing_filter_fail_multiplier=0.0,
        sizing_filter_fail_action="skip_lock",
        external_breakout_events=[
            {
                "symbol": "ETHUSDT",
                "event_key": "external-next-open",
                "signal_start": "2026-01-01T00:00:00Z",
                "touch_time": "2026-01-01T00:00:00Z",
                "side": "long",
                "level": 101.0,
                "context_combo_spec": "low_eff_rf_rank_median_000",
                "rf_probability": 0.63,
            }
        ],
        external_breakout_shape_name="low_eff_rf_rank_median_000",
        external_entry_mode="next_second_open",
        reentry_fill_policy="strict_next_second_cross",
    )

    entries = ledger[ledger["type"].isin(["BUY", "SHORT"])]
    assert len(entries) == 1
    entry = entries.iloc[0]
    assert entry["time"] == pd.Timestamp("2026-01-01T00:00:01Z")
    assert entry["reason"] == "External-NextSecond-Open"
    assert entry["breakout_shape_name"] == "low_eff_rf_rank_median_000"
    assert entry["notional"] == 2_000.0
    assert entry["price"] == 103.0 * (1.0 + lifecycle.COMMON_REPLAY_KWARGS["fixed_slippage"])
    assert diagnostics["external_breakout_locks"]["long"]["low_eff_rf_rank_median_000"] == 1
    assert diagnostics["reentry_fill_rejects"]["long"].get("same_second_zero_initial", 0) == 0


def test_external_breakout_next_second_open_does_not_fill_same_second_event():
    seconds = _seconds()

    ledger, diagnostics = lifecycle.run_second_bar_replay(
        seconds,
        _signal(),
        initial_balance=10_000.0,
        breakout_shape="baseline_plus_t3",
        replay_mode="same_bar_parity",
        shape_sizing_filters={"allowed_sides": []},
        sizing_filter_shapes=["original_t2"],
        sizing_filter_fail_multiplier=0.0,
        sizing_filter_fail_action="skip_lock",
        external_breakout_events=[
            {
                "symbol": "ETHUSDT",
                "event_key": "external-same-second",
                "signal_start": "2026-01-01T00:00:00Z",
                "touch_time": "2026-01-01T00:00:01Z",
                "side": "long",
                "level": 101.0,
                "context_combo_spec": "low_eff_rf_rank_median_000",
            }
        ],
        external_breakout_shape_name="low_eff_rf_rank_median_000",
        external_entry_mode="next_second_open",
        reentry_fill_policy="strict_next_second_cross",
    )

    assert ledger.empty
    assert diagnostics["external_breakout_locks"]["long"].get("low_eff_rf_rank_median_000", 0) == 0
