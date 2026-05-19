"""ViolationsMarkdownWriter — 将 invariant violations 写出为 markdown 报告。

输出路径模板：
  research/tmp_entry_redesign_{candidate_id}_invariant_violations.md

每项违反列出：
  - property_id
  - candidate_id
  - counterexample_json_path
  - human_readable_summary

如果 violations dict 中所有 count 均为 0 且 P7_ledger_sha256_pairs 为空
且 live_output_emitted == false，则不创建文件，返回 None。

UTF-8 编码，LF 换行。

Requirements: 6.13
"""

from __future__ import annotations

import pathlib
from typing import Any, Optional


# ---------------------------------------------------------------------------
# Property 描述映射（human-readable summary 模板）
# ---------------------------------------------------------------------------

_PROPERTY_DESCRIPTIONS: dict[str, str] = {
    "P1_missing_trigger": (
        "Intrabar breakout semantic violated: "
        "trigger condition met but no trigger_ts generated."
    ),
    "P1_spurious_trigger": (
        "Intrabar breakout semantic violated: "
        "no trigger condition but trigger_ts was spuriously generated."
    ),
    "P3": (
        "Trades per bar bound violated: "
        "same signal bar has more than max_trades_per_bar=2 real entries."
    ),
    "P4": (
        "Cost model monotonicity violated: "
        "realistic_taker_both_pnl <= realistic_pnl <= slip_pnl <= raw_pnl "
        "not satisfied within relative tolerance 1e-9."
    ),
    "P5": (
        "Point-in-time features only violated: "
        "canary column (signal bar full OHLC) was accessed during entry gating."
    ),
    "P6": (
        "Walk-forward window non-overlap violated: "
        "train/validation/execute windows are not pairwise disjoint "
        "or execute.start < validation.end."
    ),
    "P7": (
        "Bit-identical ledger violated: "
        "two independent runs with same candidate_id/seed/events produced "
        "different sha256 digests for the ledger CSV."
    ),
    "P8": (
        "Round-trip serialization violated: "
        "summary JSON decode-then-encode does not produce value-equivalent content "
        "(numeric relative error > 1e-12) or ledger column order mismatch."
    ),
    "P9": (
        "Per-trade quality reporting violated: "
        "per_trade_quality_bps_over_notional field missing, null when trade_count > 0, "
        "or recomputation mismatch exceeds tolerance 1e-6."
    ),
    "P10": (
        "No live config output violated: "
        "live configuration artifacts were emitted by the research harness."
    ),
    "P11": (
        "Entry price mode fallback explicit violated: "
        "post_touch_pullback_limit unfilled event appeared in ledger trade rows."
    ),
    "P12": (
        "Metric normalization both reported violated: "
        "active_silo_sum_pct and calendar_normalized_return_pct not both present, "
        "or active_months + empty_months != 22."
    ),
}

# 反例 JSON 路径模板
_COUNTEREXAMPLE_PATH_TEMPLATE = (
    "research/tmp_entry_redesign_invariant_counterexamples/{property_id}_{candidate_id}.json"
)


# ---------------------------------------------------------------------------
# ViolationsMarkdownWriter
# ---------------------------------------------------------------------------


class ViolationsMarkdownWriter:
    """将 invariant violations 写出为 markdown 报告。

    write(violations, candidate_id, output_dir) -> Optional[pathlib.Path]

    如果无违反（所有 count 为 0、P7 pairs 为空、live_output_emitted 为 false），
    返回 None 且不创建文件。

    否则写出 markdown 文件，每项违反列出：
      property_id / candidate_id / counterexample_json_path / human_readable_summary
    """

    def write(
        self,
        violations: dict[str, Any],
        candidate_id: str,
        output_dir: pathlib.Path,
    ) -> Optional[pathlib.Path]:
        """将 violations 写出为 markdown 文件。

        Args:
            violations: InvariantChecker.check() 返回的固定 schema dict（13 字段）。
            candidate_id: Entry_Candidate ID。
            output_dir: 输出目录（通常为 research/ 或 pathlib.Path("research")）。

        Returns:
            写入的 markdown 文件路径，如果无违反则返回 None。
        """
        # 收集所有非零违反项
        violation_entries = self._collect_violations(violations, candidate_id)

        if not violation_entries:
            return None

        # 构建 markdown 内容
        lines = self._build_markdown(violation_entries, candidate_id)

        # 写出文件
        output_dir.mkdir(parents=True, exist_ok=True)
        filename = f"tmp_entry_redesign_{candidate_id}_invariant_violations.md"
        out_path = output_dir / filename

        # UTF-8 编码，LF 换行（使用 write_bytes 确保跨平台 LF）
        content = "\n".join(lines) + "\n"
        out_path.write_bytes(content.encode("utf-8"))

        return out_path

    # ------------------------------------------------------------------
    # 内部方法
    # ------------------------------------------------------------------

    def _collect_violations(
        self,
        violations: dict[str, Any],
        candidate_id: str,
    ) -> list[dict[str, str]]:
        """从 violations dict 中收集所有非零违反项。

        Returns:
            违反项列表，每项包含 property_id / candidate_id /
            counterexample_json_path / human_readable_summary。
        """
        entries: list[dict[str, str]] = []

        # P1: missing trigger
        p1_missing = violations.get("P1_missing_trigger_count", 0)
        if p1_missing > 0:
            entries.append({
                "property_id": "P1",
                "candidate_id": candidate_id,
                "counterexample_json_path": _COUNTEREXAMPLE_PATH_TEMPLATE.format(
                    property_id="P1", candidate_id=candidate_id
                ),
                "human_readable_summary": (
                    f"{_PROPERTY_DESCRIPTIONS['P1_missing_trigger']} "
                    f"Count: {p1_missing}."
                ),
            })

        # P1: spurious trigger
        p1_spurious = violations.get("P1_spurious_trigger_count", 0)
        if p1_spurious > 0:
            entries.append({
                "property_id": "P1",
                "candidate_id": candidate_id,
                "counterexample_json_path": _COUNTEREXAMPLE_PATH_TEMPLATE.format(
                    property_id="P1", candidate_id=candidate_id
                ),
                "human_readable_summary": (
                    f"{_PROPERTY_DESCRIPTIONS['P1_spurious_trigger']} "
                    f"Count: {p1_spurious}."
                ),
            })

        # P3 through P12 (count-based)
        count_properties = [
            ("P3_count", "P3"),
            ("P4_count", "P4"),
            ("P5_count", "P5"),
            ("P6_count", "P6"),
            ("P8_count", "P8"),
            ("P9_count", "P9"),
            ("P10_count", "P10"),
            ("P11_count", "P11"),
            ("P12_count", "P12"),
        ]

        for key, prop_id in count_properties:
            count = violations.get(key, 0)
            if count > 0:
                entries.append({
                    "property_id": prop_id,
                    "candidate_id": candidate_id,
                    "counterexample_json_path": _COUNTEREXAMPLE_PATH_TEMPLATE.format(
                        property_id=prop_id, candidate_id=candidate_id
                    ),
                    "human_readable_summary": (
                        f"{_PROPERTY_DESCRIPTIONS[prop_id]} Count: {count}."
                    ),
                })

        # P7: ledger sha256 pairs (list-based)
        p7_pairs = violations.get("P7_ledger_sha256_pairs", [])
        if p7_pairs:
            entries.append({
                "property_id": "P7",
                "candidate_id": candidate_id,
                "counterexample_json_path": _COUNTEREXAMPLE_PATH_TEMPLATE.format(
                    property_id="P7", candidate_id=candidate_id
                ),
                "human_readable_summary": (
                    f"{_PROPERTY_DESCRIPTIONS['P7']} "
                    f"Mismatched pairs: {len(p7_pairs)}."
                ),
            })

        # P10: live_output_emitted (bool)
        live_emitted = violations.get("live_output_emitted", False)
        if live_emitted:
            # 只在 P10_count 未覆盖时添加（避免重复）
            already_has_p10 = any(e["property_id"] == "P10" for e in entries)
            if not already_has_p10:
                entries.append({
                    "property_id": "P10",
                    "candidate_id": candidate_id,
                    "counterexample_json_path": _COUNTEREXAMPLE_PATH_TEMPLATE.format(
                        property_id="P10", candidate_id=candidate_id
                    ),
                    "human_readable_summary": (
                        f"{_PROPERTY_DESCRIPTIONS['P10']} "
                        f"live_output_emitted=true."
                    ),
                })

        return entries

    def _build_markdown(
        self,
        violation_entries: list[dict[str, str]],
        candidate_id: str,
    ) -> list[str]:
        """构建 markdown 内容行列表。

        Args:
            violation_entries: 违反项列表。
            candidate_id: Entry_Candidate ID。

        Returns:
            markdown 行列表（不含末尾换行，由调用方统一追加）。
        """
        lines: list[str] = []

        # 标题
        lines.append(f"# Invariant Violations: {candidate_id}")
        lines.append("")
        lines.append(
            f"Entry_Candidate `{candidate_id}` has "
            f"{len(violation_entries)} invariant violation(s)."
        )
        lines.append("")

        # 每项违反一个 section
        for i, entry in enumerate(violation_entries, start=1):
            lines.append(f"## {i}. Property {entry['property_id']}")
            lines.append("")
            lines.append(f"- **property_id**: `{entry['property_id']}`")
            lines.append(f"- **candidate_id**: `{entry['candidate_id']}`")
            lines.append(
                f"- **counterexample_json_path**: "
                f"`{entry['counterexample_json_path']}`"
            )
            lines.append(
                f"- **human_readable_summary**: {entry['human_readable_summary']}"
            )
            lines.append("")

        return lines
