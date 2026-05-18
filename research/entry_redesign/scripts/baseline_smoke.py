#!/usr/bin/env python3
"""Baseline_Entry_Candidate 基线回归 smoke test。

用一份最小确定性 fixture 跑 Baseline_Entry_Candidate 全 pipeline：
  - 空 events → 空 ledger
  - MetricsAggregator 对空 ledger 产出 13 指标
  - InvariantChecker.check() 返回 invariant_violations 的所有 count 均为 0、
    P7_ledger_sha256_pairs 为空、live_output_emitted==false

输出写 research/tmp_entry_redesign_baseline_smoke_<YYYYMMDD>.md。
退出码：0 成功，1 失败。

Requirements: 6.10, 6.13
"""

from __future__ import annotations

import pathlib
import sys
from datetime import date, datetime, timezone

import pandas as pd

# ---------------------------------------------------------------------------
# 项目根目录定位（确保 import 路径正确）
# ---------------------------------------------------------------------------

_SCRIPT_DIR = pathlib.Path(__file__).resolve().parent
_ENTRY_REDESIGN_ROOT = _SCRIPT_DIR.parent
_RESEARCH_DIR = _ENTRY_REDESIGN_ROOT.parent
_PROJECT_ROOT = _RESEARCH_DIR.parent

# 确保 project root 在 sys.path 中
if str(_PROJECT_ROOT) not in sys.path:
    sys.path.insert(0, str(_PROJECT_ROOT))

from research.entry_redesign.invariants.invariant_checker import InvariantChecker
from research.entry_redesign.metrics.metrics_aggregator import MetricsAggregator
from research.entry_redesign.scheduler.default_subset import BASELINE
from research.entry_redesign.spec.candidate_id import generate_candidate_id


# ---------------------------------------------------------------------------
# 最小确定性 fixture：空 events → 空 ledger
# ---------------------------------------------------------------------------


def _build_empty_ledger() -> pd.DataFrame:
    """构建空 ledger DataFrame（22 字段 header，0 行数据）。

    这是 Baseline_Entry_Candidate 在无 events 输入时的预期产出：
    空 events → 空 ledger。
    """
    columns = [
        "entry_ts",
        "exit_ts",
        "symbol",
        "side",
        "entry_price",
        "exit_price",
        "notional",
        "raw_pnl",
        "slip_pnl",
        "realistic_pnl",
        "realistic_taker_both_pnl",
        "exit_reason",
        "entry_candidate_id",
        "gate_mode",
        "signal_bar_start_ts",
        "trigger_ts",
        "entry_delay_seconds",
        "feature_horizon_seconds",
        "trigger_confirmation_id",
        "entry_price_mode_id",
        "pretouch_state_band_id",
        "posttouch_quality_band_id",
    ]
    return pd.DataFrame(columns=columns)


def _build_baseline_summary(metrics: dict) -> dict:
    """构建 Baseline_Entry_Candidate 的 summary dict。

    包含 metrics + 必要的 summary 字段（walkforward_config、
    gate001_snapshot_ref、baseline_reference、live_output_emitted 等）。
    """
    summary = dict(metrics)

    # walkforward_config（Requirement 4.3）
    summary["walkforward_config"] = {
        "train_months": 2,
        "validation_months": 1,
        "execute_months": 1,
        "execute_start_year_month": "2025-06",
        "execute_end_year_month": "2026-04",
        "total_execute_months": 11,
    }

    # gate001_snapshot_ref（Requirement 4.4）
    summary["gate001_snapshot_ref"] = {
        "path": "research/20260511_probabilistic_v6_calendar_holdout_validation.md",
        "sha256": "fixture_not_available",
    }

    # baseline_reference（Requirement 3.4）
    summary["baseline_reference"] = {
        "nogate_win_rate": None,
        "nogate_payoff_ratio": None,
        "BTCUSDT": {"nogate_win_rate": None, "nogate_payoff_ratio": None},
        "ETHUSDT": {"nogate_win_rate": None, "nogate_payoff_ratio": None},
    }

    # 判定字段
    summary["event_expectation_positive"] = False
    summary["event_expectation_positive_btc_only"] = False
    summary["event_expectation_positive_eth_only"] = False
    summary["small_sample_flag"] = True
    summary["asymmetry_tag"] = "none"

    # live_output_emitted 恒 false（research-only，Requirement 6.10）
    summary["live_output_emitted"] = False

    return summary


# ---------------------------------------------------------------------------
# 断言逻辑
# ---------------------------------------------------------------------------


def _assert_no_violations(violations: dict) -> list[str]:
    """检查 invariant_violations 是否全部通过。

    返回失败消息列表（空列表 = 全部通过）。
    """
    failures: list[str] = []

    # 所有 *_count 字段必须为 0
    count_keys = [
        "P1_missing_trigger_count",
        "P1_spurious_trigger_count",
        "P3_count",
        "P4_count",
        "P5_count",
        "P6_count",
        "P8_count",
        "P9_count",
        "P10_count",
        "P11_count",
        "P12_count",
    ]
    for key in count_keys:
        value = violations.get(key, -1)
        if value != 0:
            failures.append(f"FAIL: {key} = {value} (expected 0)")

    # P7_ledger_sha256_pairs 必须为空
    p7_pairs = violations.get("P7_ledger_sha256_pairs", None)
    if p7_pairs is None:
        failures.append("FAIL: P7_ledger_sha256_pairs is missing")
    elif len(p7_pairs) != 0:
        failures.append(
            f"FAIL: P7_ledger_sha256_pairs is non-empty (len={len(p7_pairs)})"
        )

    # live_output_emitted 必须为 false
    live_emitted = violations.get("live_output_emitted", None)
    if live_emitted is not False:
        failures.append(
            f"FAIL: live_output_emitted = {live_emitted} (expected false)"
        )

    return failures


# ---------------------------------------------------------------------------
# 报告生成
# ---------------------------------------------------------------------------


def _generate_report(
    candidate_id: str,
    violations: dict,
    failures: list[str],
    passed: bool,
) -> str:
    """生成 markdown 报告内容。"""
    now_utc = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    status = "✅ PASSED" if passed else "❌ FAILED"

    lines = [
        "# Baseline_Entry_Candidate Smoke Test Report",
        "",
        f"- **Status**: {status}",
        f"- **Candidate ID**: `{candidate_id}`",
        f"- **Timestamp (UTC)**: {now_utc}",
        f"- **Fixture**: empty events → empty ledger (minimal deterministic)",
        "",
        "## Baseline_Entry_Candidate 六元组",
        "",
        "| 字段 | 值 |",
        "|------|-----|",
        f"| entry_delay_seconds (D) | {BASELINE.entry_delay_seconds} |",
        f"| feature_horizon_seconds (H) | {BASELINE.feature_horizon_seconds} |",
        f"| trigger_confirmation_id | {BASELINE.trigger_confirmation_id} |",
        f"| entry_price_mode_id | {BASELINE.entry_price_mode_id} |",
        f"| pretouch_state_band_id | {BASELINE.pretouch_state_band_id} |",
        f"| posttouch_quality_band_id | {BASELINE.posttouch_quality_band_id} |",
        "",
        "## InvariantChecker 结果",
        "",
        "| Property | Count | Status |",
        "|----------|-------|--------|",
    ]

    count_keys = [
        ("P1_missing_trigger_count", "P1 Missing Trigger"),
        ("P1_spurious_trigger_count", "P1 Spurious Trigger"),
        ("P3_count", "P3 Trades Per Bar Bound"),
        ("P4_count", "P4 Cost Model Monotonicity"),
        ("P5_count", "P5 Point-in-Time Features"),
        ("P6_count", "P6 Walk-Forward Non-Overlap"),
        ("P7_ledger_sha256_pairs", "P7 Bit-Identical Ledger"),
        ("P8_count", "P8 Round-Trip Serialization"),
        ("P9_count", "P9 Per-Trade Quality Reporting"),
        ("P10_count", "P10 No Live Config Output"),
        ("P11_count", "P11 Entry Price Mode Fallback"),
        ("P12_count", "P12 Metric Normalization"),
    ]

    for key, label in count_keys:
        value = violations.get(key, "N/A")
        if key == "P7_ledger_sha256_pairs":
            display_val = f"len={len(value)}" if isinstance(value, list) else str(value)
            ok = isinstance(value, list) and len(value) == 0
        else:
            display_val = str(value)
            ok = value == 0
        status_icon = "✅" if ok else "❌"
        lines.append(f"| {label} | {display_val} | {status_icon} |")

    # live_output_emitted
    live_val = violations.get("live_output_emitted", "N/A")
    live_ok = live_val is False
    lines.append(
        f"| live_output_emitted | {live_val} | {'✅' if live_ok else '❌'} |"
    )

    lines.append("")

    if failures:
        lines.append("## Failures")
        lines.append("")
        for f in failures:
            lines.append(f"- {f}")
        lines.append("")

    lines.append("## 说明")
    lines.append("")
    lines.append(
        "本 smoke test 使用最小确定性 fixture（空 events → 空 ledger）验证 "
        "Baseline_Entry_Candidate 全 pipeline 的 InvariantChecker 输出。"
    )
    lines.append(
        "所有 invariant violation count 均为 0、P7_ledger_sha256_pairs 为空、"
        "live_output_emitted==false 即为通过。"
    )
    lines.append("")
    lines.append("---")
    lines.append("")
    lines.append("Requirements: 6.10, 6.13")
    lines.append("")

    return "\n".join(lines)


# ---------------------------------------------------------------------------
# 主入口
# ---------------------------------------------------------------------------


def main() -> int:
    """执行 Baseline_Entry_Candidate 基线回归 smoke test。

    Returns:
        0 成功，1 失败。
    """
    # 1. 验证 BASELINE 六元组合法性
    BASELINE.validate()
    candidate_id = generate_candidate_id(BASELINE)

    # 2. 构建最小确定性 fixture：空 events → 空 ledger
    empty_ledger = _build_empty_ledger()

    # 3. 运行 MetricsAggregator 对空 ledger 产出 13 指标
    aggregator = MetricsAggregator()
    metrics = aggregator.aggregate(empty_ledger, total_silos=22)

    # 4. 构建 summary（含 live_output_emitted=false 等必要字段）
    summary = _build_baseline_summary(metrics)

    # 5. 运行 InvariantChecker.check()
    checker = InvariantChecker()
    violations = checker.check(empty_ledger, summary)

    # 6. 断言所有 violation counts 为 0、P7 pairs 为空、live_output_emitted==false
    failures = _assert_no_violations(violations)
    passed = len(failures) == 0

    # 7. 生成报告
    report_content = _generate_report(candidate_id, violations, failures, passed)

    # 8. 写报告到 research/tmp_entry_redesign_baseline_smoke_<YYYYMMDD>.md
    today_str = date.today().strftime("%Y%m%d")
    report_filename = f"tmp_entry_redesign_baseline_smoke_{today_str}.md"
    report_path = _RESEARCH_DIR / report_filename
    report_path.parent.mkdir(parents=True, exist_ok=True)
    report_path.write_text(report_content, encoding="utf-8")

    # 9. 输出结果
    if passed:
        print(f"[PASS] Baseline smoke test passed. Report: {report_path}")
        return 0
    else:
        print(f"[FAIL] Baseline smoke test failed. Report: {report_path}")
        for f in failures:
            print(f"  {f}")
        return 1


if __name__ == "__main__":
    sys.exit(main())
