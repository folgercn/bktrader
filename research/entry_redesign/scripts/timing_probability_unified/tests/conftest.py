"""Shared fixtures and strategies for timing_probability_unified tests."""

import sys
from pathlib import Path

import numpy as np
import pandas as pd
import pytest
from hypothesis import strategies as st

# Ensure the scripts directory is on sys.path so that
# both timing_probability_unified and pre_breakout_timing
# can be imported as top-level packages.
_scripts_dir = Path(__file__).resolve().parents[2]
if str(_scripts_dir) not in sys.path:
    sys.path.insert(0, str(_scripts_dir))


# ===========================================================================
# Shared Hypothesis Strategies
# ===========================================================================

# Strategy for generating symbol names
symbols_strategy = st.sampled_from(["BTCUSDT", "ETHUSDT"])

# Strategy for generating side values
sides_strategy = st.sampled_from(["long", "short"])

# Strategy for generating timing predictions
timing_prediction_strategy = st.sampled_from(["skip", "fast", "slow"])

# Strategy for generating PnL values (realistic range for percentage PnL)
pnl_strategy = st.floats(min_value=-0.05, max_value=0.05, allow_nan=False, allow_infinity=False)

# Strategy for generating probability values in [0, 1]
probability_strategy = st.floats(min_value=0.0, max_value=1.0, allow_nan=False, allow_infinity=False)

# Strategy for generating sizing multipliers in [0, 2]
multiplier_strategy = st.floats(min_value=0.0, max_value=2.0, allow_nan=False, allow_infinity=False)

# Strategy for generating speed_300s_atr values
speed_atr_strategy = st.floats(min_value=0.0, max_value=10.0, allow_nan=False, allow_infinity=False)

# Strategy for generating base_share values
base_share_strategy = st.floats(min_value=0.01, max_value=1.0, allow_nan=False, allow_infinity=False)


# ===========================================================================
# Shared Fixtures
# ===========================================================================


@pytest.fixture
def sample_events_df() -> pd.DataFrame:
    """Create a sample events DataFrame for testing."""
    return pd.DataFrame(
        {
            "event_id": ["evt_001", "evt_002", "evt_003"],
            "symbol": ["BTCUSDT", "ETHUSDT", "BTCUSDT"],
            "side": ["long", "short", "long"],
            "touch_time": pd.date_range("2025-01-01", periods=3, freq="1D", tz="UTC"),
            "speed_300s_atr": [0.5, 1.0, 1.5],
        }
    )


@pytest.fixture
def original_10_features() -> list[str]:
    """Return the Original_10_Features list."""
    return [
        "signal_atr_percentile",
        "roundtrip_cost_atr",
        "prev1_body_atr",
        "prev1_range_atr",
        "prev1_close_pos_side",
        "prev_sma5_gap_atr",
        "prev_sma5_slope_atr",
        "level_to_prev_close_atr",
        "level_to_signal_open_atr",
        "touch_extension_atr",
    ]
