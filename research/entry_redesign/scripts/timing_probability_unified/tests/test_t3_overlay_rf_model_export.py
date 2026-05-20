from __future__ import annotations

import numpy as np
import pandas as pd

from timing_probability_unified.t3_overlay_rf_cost_sizing import FEATURE_COLUMNS, _predict_class_one
from timing_probability_unified.t3_overlay_rf_model_export import (
    build_model_bundle,
    predict_exported_forest,
    train_t3_overlay_model,
)


def _synthetic_events() -> pd.DataFrame:
    rows = []
    for idx in range(32):
        win = idx % 4 in {0, 1}
        rows.append(
            {
                "external_event_key": f"event-{idx}",
                "event_month": "2026-01" if idx < 16 else "2026-02",
                "rf_probability": 0.25 + 0.02 * idx,
                "speed_300s_abs": 0.30 + 0.01 * (idx % 8),
                "eff_300s": 0.70 + 0.02 * (idx % 5),
                "touch_extension_abs": 0.02 + 0.01 * (idx % 6),
                "pre_touch_seconds": 120.0 + 10.0 * idx,
                "roundtrip_cost_atr": 0.08 + 0.005 * (idx % 3),
                "side_is_short": float(idx % 2),
                "label_win": int(win),
            }
        )
    return pd.DataFrame(rows)


def test_exported_t3_overlay_forest_matches_sklearn_probability() -> None:
    events = _synthetic_events()
    model = train_t3_overlay_model(events, random_state=7)
    bundle = build_model_bundle(
        events,
        model=model,
        version="test",
        trained_at="2026-05-20T00:00:00Z",
        source="synthetic",
        random_state=7,
    )

    exported = predict_exported_forest(bundle["rf_model"], events, FEATURE_COLUMNS)
    sklearn = _predict_class_one(model, events, FEATURE_COLUMNS)

    assert bundle["artifact_kind"] == "pretouch_t3_overlay_rf_quality_sizing"
    assert bundle["feature_names"] == FEATURE_COLUMNS
    assert bundle["sizing_policy"]["method"] == "t3_rf_cost_quantity_band"
    assert bundle["sizing_policy"]["min_quantity"] == 0.20
    assert bundle["sizing_policy"]["max_quantity"] == 0.40
    assert bundle["sizing_policy"]["equivalent_multiplier_band"] == [2.5, 5.0]
    assert np.allclose(exported, sklearn)
