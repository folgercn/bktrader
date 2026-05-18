"""Materialize the wf3 breakout-structure expansion as an auditable artifact.

Research-only. This script does not change live defaults; it packages the
current stable promotion candidate (`wf3_low_eff_low_atr`) with provenance,
split counts, and validation metrics so it can be reviewed separately from the
broader exploratory sweep outputs.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import logging
import sys
import time
from pathlib import Path
from typing import Any

import numpy as np
import pandas as pd

SCRIPTS_DIR = Path(__file__).resolve().parents[1]
PROJECT_ROOT = Path(__file__).resolve().parents[4]
if str(SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPTS_DIR))

from timing_probability_unified.breakout_shape_expansion import OUTPUT_DIR  # noqa: E402
from timing_probability_unified.breakout_structure_lead_expansion_combo import (  # noqa: E402
    BASE_EVENTS_CSV,
    CANONICAL_EVENTS_CSV,
    CONDITION_REPLAY_EPS,
    WALKFORWARD_FIXED,
    _candidate_events,
    _canonical_overlap_keys,
    _filter_noncanonical,
    _markdown_table,
)
from timing_probability_unified.breakout_structure_model_retrain_validation import (  # noqa: E402
    FORWARD_START,
    PRODUCTION_FEATURES,
    _load_base_events,
    _load_canonical_events,
    _split_train_test_forward,
)

logger = logging.getLogger(__name__)

CANDIDATE_NAME = "wf3_low_eff_low_atr"
GATE_NAME = "low_eff_low_atr_q20_q40"
MODEL_POOL = "combo_wf3_low_eff_low_atr"
FEATURE_SET = "production8"

WALKFORWARD_CSV = OUTPUT_DIR / "breakout_structure_walkforward_train3m_min20_candidate_forward.csv"
COMBO_SUMMARY_CSV = OUTPUT_DIR / "breakout_structure_lead_expansion_combo_summary.csv"
RETRAIN_SUMMARY_CSV = OUTPUT_DIR / "breakout_structure_model_retrain_summary.csv"
COMBO_REPORT = OUTPUT_DIR / "breakout_structure_lead_expansion_combo_report.md"
RETRAIN_REPORT = OUTPUT_DIR / "breakout_structure_model_retrain_report.md"

EVENTS_CSV = OUTPUT_DIR / "breakout_structure_wf3_candidate_events.csv"
EXPANSION_EVENTS_CSV = OUTPUT_DIR / "breakout_structure_wf3_candidate_expansion_events.csv"
MANIFEST_JSON = OUTPUT_DIR / "breakout_structure_wf3_candidate_manifest.json"
REPORT_MD = OUTPUT_DIR / "breakout_structure_wf3_candidate_promotion_report.md"


def _sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as fh:
        for chunk in iter(lambda: fh.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def _file_meta(path: Path) -> dict[str, Any]:
    return {
        "path": str(path),
        "exists": path.exists(),
        "sha256": _sha256(path) if path.exists() else None,
        "bytes": path.stat().st_size if path.exists() else None,
    }


def _load_csv(path: Path) -> pd.DataFrame:
    if not path.exists():
        raise FileNotFoundError(path)
    df = pd.read_csv(path)
    for column in ("touch_time", "signal_start", "signal_end"):
        if column in df.columns:
            df[column] = pd.to_datetime(df[column], utc=True)
    return df


def _candidate_spec():
    for candidate in WALKFORWARD_FIXED:
        if candidate.name == CANDIDATE_NAME:
            return candidate
    raise ValueError(f"missing candidate spec: {CANDIDATE_NAME}")


def _source_counts(df: pd.DataFrame) -> dict[str, int]:
    if df.empty or "source_leg" not in df.columns:
        return {}
    return {str(key): int(value) for key, value in df["source_leg"].value_counts().sort_index().items()}


def _time_range(df: pd.DataFrame) -> dict[str, str | None]:
    if df.empty:
        return {"start": None, "end": None}
    times = pd.to_datetime(df["touch_time"], utc=True)
    return {"start": times.min().isoformat(), "end": times.max().isoformat()}


def _split_rows(events: pd.DataFrame) -> pd.DataFrame:
    train, test, forward = _split_train_test_forward(events)
    rows = []
    for split_name, split_df in (
        ("train", train),
        ("test", test),
        ("forward", forward),
        ("full_pre_forward", pd.concat([train, test], ignore_index=True, sort=False)),
    ):
        counts = _source_counts(split_df)
        rows.append(
            {
                "split": split_name,
                "events": int(len(split_df)),
                "canonical_events": counts.get("canonical", 0),
                CANDIDATE_NAME: counts.get(CANDIDATE_NAME, 0),
                "start": _time_range(split_df)["start"],
                "end": _time_range(split_df)["end"],
            }
        )
    return pd.DataFrame(rows)


def _materialize_events() -> tuple[pd.DataFrame, pd.DataFrame, pd.DataFrame, int]:
    canonical = _load_canonical_events().copy()
    canonical["source_leg"] = "canonical"

    base_events = _load_base_events()
    candidate_source = _candidate_events(_candidate_spec(), base_events).copy()
    candidate_source["source_leg"] = CANDIDATE_NAME

    extra_events, overlap_removed = _filter_noncanonical(candidate_source, _canonical_overlap_keys())
    extra_events = extra_events.copy()
    extra_events["source_leg"] = CANDIDATE_NAME

    pooled = pd.concat([canonical, extra_events], ignore_index=True, sort=False)
    pooled["touch_time"] = pd.to_datetime(pooled["touch_time"], utc=True)
    pooled = pooled.sort_values(["touch_time", "source_leg", "side"]).reset_index(drop=True)
    return pooled, candidate_source.reset_index(drop=True), extra_events.reset_index(drop=True), overlap_removed


def _validation_rows() -> tuple[pd.DataFrame, dict[str, Any]]:
    combo = _load_csv(COMBO_SUMMARY_CSV)
    retrain = _load_csv(RETRAIN_SUMMARY_CSV)
    combo_row = combo[combo["variant"] == CANDIDATE_NAME]
    retrain_row = retrain[(retrain["pool"] == MODEL_POOL) & (retrain["feature_set"] == FEATURE_SET)]
    canonical_retrain = retrain[(retrain["pool"] == "canonical_only") & (retrain["feature_set"] == FEATURE_SET)]
    if combo_row.empty:
        raise ValueError(f"combo summary missing {CANDIDATE_NAME}")
    if retrain_row.empty:
        raise ValueError(f"retrain summary missing {MODEL_POOL}/{FEATURE_SET}")
    if canonical_retrain.empty:
        raise ValueError("retrain summary missing canonical_only/production8")

    combo_item = combo_row.iloc[0]
    retrain_item = retrain_row.iloc[0]
    canonical_item = canonical_retrain.iloc[0]
    rows = pd.DataFrame(
        [
            {
                "check": "lead_combo_replay",
                "events": int(combo_item["candidate_source_events"]),
                "extra_events": int(combo_item["extra_events"]),
                "same_close": float(combo_item["combo_same_close_calendar_sum"]),
                "adverse10": float(combo_item["combo_adverse10_calendar_sum_exact"]),
                "adverse10_lift": float(combo_item["combo_adverse10_delta_vs_lead_exact"]),
                "worst_sm": float(combo_item["combo_adverse10_worst_sm_exact"]),
                "neg_sm": int(combo_item["combo_adverse10_neg_sm_exact"]),
                "trades": int(combo_item["combo_adverse10_trade_count_exact"]),
            },
            {
                "check": "retrain_forward_production8",
                "events": int(retrain_item["forward_events"]),
                "extra_events": "",
                "same_close": float(retrain_item["forward_same_close_calendar_sum"]),
                "adverse10": float(retrain_item["forward_adverse10_calendar_sum"]),
                "adverse10_lift": float(retrain_item["forward_adverse10_calendar_sum"])
                - float(canonical_item["forward_adverse10_calendar_sum"]),
                "worst_sm": float(retrain_item["forward_adverse10_worst_sm"]),
                "neg_sm": int(retrain_item["forward_adverse10_neg_sm"]),
                "trades": int(retrain_item["forward_trade_count"]),
            },
        ]
    )
    diagnostics = {
        "combo_row": combo_item.to_dict(),
        "retrain_row": retrain_item.to_dict(),
        "canonical_retrain_row": canonical_item.to_dict(),
    }
    return rows, diagnostics


def _walkforward_gate_rows() -> pd.DataFrame:
    rows = _load_csv(WALKFORWARD_CSV)
    rows = rows[rows["gate"] == GATE_NAME].copy()
    return rows[
        [
            "forward_month",
            "conditions",
            "forward_events",
            "same_close_calendar_sum",
            "adverse10_calendar_sum",
            "trade_count",
        ]
    ].reset_index(drop=True)


def _write_report(
    *,
    pooled: pd.DataFrame,
    candidate_source: pd.DataFrame,
    extra_events: pd.DataFrame,
    overlap_removed: int,
    split_df: pd.DataFrame,
    validation_df: pd.DataFrame,
    gate_df: pd.DataFrame,
    manifest: dict[str, Any],
) -> None:
    lines: list[str] = [
        "# WF3 Breakout Structure Candidate Artifact",
        "",
        f"Generated: {pd.Timestamp.utcnow().isoformat()}",
        "",
        "Scope: research-only. This packages `wf3_low_eff_low_atr` as an auditable event-source candidate; it does not modify live defaults.",
        "",
        "## Recommendation",
        "",
        (
            "`wf3_low_eff_low_atr` is the current stable promotion candidate, but it is not live-ready yet. "
            "It should be treated as the next research lead for cross-asset and longer-history validation, not as a default strategy change."
        ),
        "",
        "## Candidate Identity",
        "",
        f"- Canonical lead events: `{CANONICAL_EVENTS_CSV}`",
        f"- Expansion base events: `{BASE_EVENTS_CSV}`",
        f"- Walk-forward gate: `{GATE_NAME}` from `{WALKFORWARD_CSV}`",
        f"- Overlap key: canonical `(signal_start, side)`; removed events: `{overlap_removed}`",
        f"- Numeric replay tolerance for legacy rounded gate strings: `{CONDITION_REPLAY_EPS}`",
        f"- Model feature contract for retrain check: `{FEATURE_SET}` / `{', '.join(PRODUCTION_FEATURES)}`",
        f"- Forward split starts at `{FORWARD_START.isoformat()}`",
        "",
        "## Event Counts",
        "",
        _markdown_table(
            pd.DataFrame(
                [
                    {
                        "bucket": "canonical",
                        "events": int((pooled["source_leg"] == "canonical").sum()),
                        "start": _time_range(pooled[pooled["source_leg"] == "canonical"])["start"],
                        "end": _time_range(pooled[pooled["source_leg"] == "canonical"])["end"],
                    },
                    {
                        "bucket": "wf3_source_before_overlap",
                        "events": int(len(candidate_source)),
                        "start": _time_range(candidate_source)["start"],
                        "end": _time_range(candidate_source)["end"],
                    },
                    {
                        "bucket": "wf3_extra_after_overlap",
                        "events": int(len(extra_events)),
                        "start": _time_range(extra_events)["start"],
                        "end": _time_range(extra_events)["end"],
                    },
                    {
                        "bucket": "combined_candidate",
                        "events": int(len(pooled)),
                        "start": _time_range(pooled)["start"],
                        "end": _time_range(pooled)["end"],
                    },
                ]
            ),
            ["bucket", "events", "start", "end"],
        ),
        "",
        "## Train/Test/Forward Split",
        "",
        _markdown_table(
            split_df,
            ["split", "events", "canonical_events", CANDIDATE_NAME, "start", "end"],
        ),
        "",
        "## Validation Metrics",
        "",
        _markdown_table(
            validation_df,
            ["check", "events", "extra_events", "same_close", "adverse10", "adverse10_lift", "worst_sm", "neg_sm", "trades"],
        ),
        "",
        "## Walk-Forward Gate Conditions",
        "",
        _markdown_table(
            gate_df,
            [
                "forward_month",
                "conditions",
                "forward_events",
                "same_close_calendar_sum",
                "adverse10_calendar_sum",
                "trade_count",
            ],
        ),
        "",
        "## Blockers Before Live Promotion",
        "",
        "- Current exact event-source artifacts cover 2025-06 through 2026-04 only.",
        "- BTC exists in the canonical robust CSV, but this expansion path still depends on ETH-specific current-shape/replay artifacts.",
        "- Longer bars caches exist, yet the exact pretouch + current-shape + wf3 event-source builder is not rebuilt for older history.",
        "- The 20260515 hand-written decision report still has provenance drift versus current production-template replay numbers.",
        "",
        "## Manifest",
        "",
        "```json",
        json.dumps(manifest, indent=2, ensure_ascii=False, default=str),
        "```",
    ]
    REPORT_MD.write_text("\n".join(lines) + "\n", encoding="utf-8")


def run() -> None:
    started = time.time()
    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)

    pooled, candidate_source, extra_events, overlap_removed = _materialize_events()
    split_df = _split_rows(pooled)
    validation_df, validation_diagnostics = _validation_rows()
    gate_df = _walkforward_gate_rows()

    pooled.to_csv(EVENTS_CSV, index=False)
    extra_events.to_csv(EXPANSION_EVENTS_CSV, index=False)

    input_files = {
        "canonical_events_csv": _file_meta(CANONICAL_EVENTS_CSV),
        "base_events_csv": _file_meta(BASE_EVENTS_CSV),
        "walkforward_candidate_csv": _file_meta(WALKFORWARD_CSV),
        "combo_summary_csv": _file_meta(COMBO_SUMMARY_CSV),
        "retrain_summary_csv": _file_meta(RETRAIN_SUMMARY_CSV),
        "combo_report": _file_meta(COMBO_REPORT),
        "retrain_report": _file_meta(RETRAIN_REPORT),
    }
    output_files = {
        "candidate_events_csv": _file_meta(EVENTS_CSV),
        "expansion_events_csv": _file_meta(EXPANSION_EVENTS_CSV),
        "report_md": {"path": str(REPORT_MD)},
        "manifest_json": {"path": str(MANIFEST_JSON)},
    }
    manifest: dict[str, Any] = {
        "candidate": CANDIDATE_NAME,
        "gate": GATE_NAME,
        "scope": "research-only",
        "event_counts": {
            "canonical": int((pooled["source_leg"] == "canonical").sum()),
            "candidate_source_before_overlap": int(len(candidate_source)),
            "overlap_removed": int(overlap_removed),
            "candidate_extra_after_overlap": int(len(extra_events)),
            "combined_candidate": int(len(pooled)),
        },
        "source_counts": _source_counts(pooled),
        "split_counts": split_df.to_dict(orient="records"),
        "validation_metrics": validation_df.replace({np.nan: None}).to_dict(orient="records"),
        "walkforward_gate_rows": gate_df.replace({np.nan: None}).to_dict(orient="records"),
        "input_files": input_files,
        "output_files": output_files,
        "numeric_replay_tolerance": CONDITION_REPLAY_EPS,
        "forward_start": FORWARD_START.isoformat(),
        "model_feature_contract": {"name": FEATURE_SET, "features": PRODUCTION_FEATURES},
        "diagnostics": validation_diagnostics,
        "runtime_seconds": time.time() - started,
    }
    MANIFEST_JSON.write_text(json.dumps(manifest, indent=2, ensure_ascii=False, default=str) + "\n", encoding="utf-8")
    manifest["output_files"] = {
        **output_files,
        "report_md": _file_meta(REPORT_MD) if REPORT_MD.exists() else output_files["report_md"],
        "manifest_json": _file_meta(MANIFEST_JSON),
    }
    _write_report(
        pooled=pooled,
        candidate_source=candidate_source,
        extra_events=extra_events,
        overlap_removed=overlap_removed,
        split_df=split_df,
        validation_df=validation_df,
        gate_df=gate_df,
        manifest=manifest,
    )
    manifest["output_files"]["report_md"] = _file_meta(REPORT_MD)
    MANIFEST_JSON.write_text(json.dumps(manifest, indent=2, ensure_ascii=False, default=str) + "\n", encoding="utf-8")

    logger.info("written %s", EVENTS_CSV)
    logger.info("written %s", REPORT_MD)


def main() -> None:
    parser = argparse.ArgumentParser(description="Materialize wf3 breakout-structure candidate artifact")
    parser.add_argument("--log-level", default="INFO")
    args = parser.parse_args()
    logging.basicConfig(
        level=getattr(logging, str(args.log_level).upper(), logging.INFO),
        format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
    )
    run()


if __name__ == "__main__":
    main()
