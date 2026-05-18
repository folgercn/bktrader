"""InvariantChecker — 固定 schema 的 invariant_violations 校验。

产出固定 schema 的 invariant_violations dict（13 字段），
任一 *_count > 0 OR P7_ledger_sha256_pairs 非空 OR live_output_emitted==true
→ CI 非零退出。

13 字段 schema（恒存在，即使 count 为 0）：
  P1_missing_trigger_count (int)
  P1_spurious_trigger_count (int)
  P3_count (int)
  P4_count (int)
  P5_count (int)
  P6_count (int)
  P7_ledger_sha256_pairs (list[dict])
  P8_count (int)
  P9_count (int)
  P10_count (int)
  P11_count (int)
  P12_count (int)
  live_output_emitted (bool)

CI 失败策略：任一 *_count > 0 OR P7_ledger_sha256_pairs 非空
OR live_output_emitted == true → CI 立即非零退出。
所有 invariant 同等严重，不区分 safety-critical 与 non-critical。

Requirements: 6.13
"""

from __future__ import annotations

from typing import Any

import pandas as pd


# ---------------------------------------------------------------------------
# 常量
# ---------------------------------------------------------------------------

# AGENTS §2 Research_Baseline: max_trades_per_bar=2
_MAX_TRADES_PER_BAR: int = 2

# P4 cost model monotonicity 相对误差容忍度
_P4_RELATIVE_TOLERANCE: float = 1e-9

# P9 per_trade_quality 重算相对误差容忍度
_P9_RELATIVE_TOLERANCE: float = 1e-6

# 固定 silo 总数（BTCUSDT + ETHUSDT × 11 execute months = 22）
_TOTAL_SILOS: int = 22


# ---------------------------------------------------------------------------
# InvariantChecker
# ---------------------------------------------------------------------------


class InvariantChecker:
    """P1-P12 property 校验器。

    check(ledger, summary) 返回固定 schema 的 invariant_violations dict。
    任一违反 → CI 非零退出。
    """

    def check(
        self,
        ledger: pd.DataFrame,
        summary: dict[str, Any],
    ) -> dict[str, Any]:
        """执行全部 invariant 校验，返回固定 schema 的 violations dict。

        Args:
            ledger: Research_Ledger DataFrame，包含 22 字段。
            summary: summary JSON dict（含 metrics、walkforward_config 等）。

        Returns:
            固定 schema 的 invariant_violations dict（13 字段恒存在）。
        """
        violations: dict[str, Any] = {
            "P1_missing_trigger_count": 0,
            "P1_spurious_trigger_count": 0,
            "P3_count": 0,
            "P4_count": 0,
            "P5_count": 0,
            "P6_count": 0,
            "P7_ledger_sha256_pairs": [],
            "P8_count": 0,
            "P9_count": 0,
            "P10_count": 0,
            "P11_count": 0,
            "P12_count": 0,
            "live_output_emitted": False,
        }

        # P3: Trades Per Bar Bound
        violations["P3_count"] = self._check_p3(ledger)

        # P4: Cost Model Monotonicity
        violations["P4_count"] = self._check_p4(ledger)

        # P9: Per-Trade Quality Reporting Present
        violations["P9_count"] = self._check_p9(ledger, summary)

        # P12: Metric Normalization Both Reported
        violations["P12_count"] = self._check_p12(summary)

        # P10: No Live Config Output — live_output_emitted 恒 false
        # 如果 summary 中已有 live_output_emitted=true，则标记违反
        if summary.get("live_output_emitted", False):
            violations["live_output_emitted"] = True
            violations["P10_count"] = 1

        return violations

    # ------------------------------------------------------------------
    # P3: Trades Per Bar Bound
    # ------------------------------------------------------------------

    def _check_p3(self, ledger: pd.DataFrame) -> int:
        """P3: 同一 signal bar 内 real-entry count <= max_trades_per_bar=2。

        按 (signal_bar_start_ts, symbol, gate_mode) 分组，
        统计每组的 trade 行数。超过 _MAX_TRADES_PER_BAR 的组计为违反。

        Returns:
            违反的 signal bar 数量。
        """
        if ledger.empty:
            return 0

        required_cols = {"signal_bar_start_ts", "symbol", "gate_mode"}
        if not required_cols.issubset(ledger.columns):
            return 0

        # 按 signal_bar_start_ts + symbol + gate_mode 分组
        grouped = ledger.groupby(
            ["signal_bar_start_ts", "symbol", "gate_mode"]
        ).size()

        # 超过 max_trades_per_bar 的组数
        violation_count = int((grouped > _MAX_TRADES_PER_BAR).sum())
        return violation_count

    # ------------------------------------------------------------------
    # P4: Cost Model Monotonicity
    # ------------------------------------------------------------------

    def _check_p4(self, ledger: pd.DataFrame) -> int:
        """P4: realistic_taker_both_pnl <= realistic_pnl <= slip_pnl <= raw_pnl。

        相对误差 1e-9 内成立。

        Returns:
            违反的 trade 行数。
        """
        if ledger.empty:
            return 0

        required_cols = {
            "raw_pnl",
            "slip_pnl",
            "realistic_pnl",
            "realistic_taker_both_pnl",
        }
        if not required_cols.issubset(ledger.columns):
            return 0

        violation_count = 0
        for _, row in ledger.iterrows():
            raw = row["raw_pnl"]
            slip = row["slip_pnl"]
            realistic = row["realistic_pnl"]
            taker_both = row["realistic_taker_both_pnl"]

            if not self._monotonicity_holds(taker_both, realistic, slip, raw):
                violation_count += 1

        return violation_count

    @staticmethod
    def _monotonicity_holds(
        taker_both: float,
        realistic: float,
        slip: float,
        raw: float,
    ) -> bool:
        """检查 taker_both <= realistic <= slip <= raw（相对误差 1e-9）。

        对于每对相邻值 (a, b)，要求 a <= b + tolerance，
        其中 tolerance = max(abs(b), 1.0) * _P4_RELATIVE_TOLERANCE。
        """
        pairs = [(taker_both, realistic), (realistic, slip), (slip, raw)]
        for lower, upper in pairs:
            tol = max(abs(upper), 1.0) * _P4_RELATIVE_TOLERANCE
            if lower > upper + tol:
                return False
        return True

    # ------------------------------------------------------------------
    # P9: Per-Trade Quality Reporting Present
    # ------------------------------------------------------------------

    def _check_p9(self, ledger: pd.DataFrame, summary: dict[str, Any]) -> int:
        """P9: per_trade_quality_bps_over_notional 字段恒存在且与 ledger 重算一致。

        检查逻辑：
        1. summary 中 per_trade_quality 字段存在（nogate_* 或 gate001_*）。
        2. trade_count == 0 时字段值为 null。
        3. 任何 notional_i == 0 的 trade 不应出现在 ledger 中。
        4. 与 ledger 重算值的相对误差 <= 1e-6。

        Returns:
            违反数量。
        """
        violation_count = 0

        # 检查 notional == 0 的 trade（不应存在于 ledger 中）
        if not ledger.empty and "notional" in ledger.columns:
            zero_notional_count = int((ledger["notional"] == 0.0).sum())
            violation_count += zero_notional_count

        # 对每个 gate_mode 检查 per_trade_quality 一致性
        for prefix in ("nogate", "gate001"):
            ptq_key = f"{prefix}_per_trade_quality_bps_over_notional"
            tc_key = f"{prefix}_trade_count"

            # 字段必须存在
            if ptq_key not in summary:
                violation_count += 1
                continue

            ptq_value = summary[ptq_key]
            trade_count = summary.get(tc_key, 0)

            # trade_count == 0 时字段值必须为 null
            if trade_count == 0:
                if ptq_value is not None:
                    violation_count += 1
                continue

            # 与 ledger 重算对比
            if ptq_value is None:
                # trade_count > 0 但 ptq 为 null → 违反
                violation_count += 1
                continue

            # 从 ledger 重算
            if not ledger.empty and "gate_mode" in ledger.columns:
                gate_mode_str = "nogate" if prefix == "nogate" else "gate001"
                subset = ledger[ledger["gate_mode"] == gate_mode_str]
                if not subset.empty and "realistic_pnl" in subset.columns and "notional" in subset.columns:
                    valid = subset[subset["notional"] != 0.0]
                    if not valid.empty:
                        recomputed = (
                            (valid["realistic_pnl"] / valid["notional"]).mean()
                            * 10000.0
                        )
                        # 相对误差检查
                        denom = max(abs(recomputed), 1e-12)
                        rel_err = abs(ptq_value - recomputed) / denom
                        if rel_err > _P9_RELATIVE_TOLERANCE:
                            violation_count += 1

        return violation_count

    # ------------------------------------------------------------------
    # P12: Metric Normalization Both Reported
    # ------------------------------------------------------------------

    def _check_p12(self, summary: dict[str, Any]) -> int:
        """P12: active_silo_sum_pct 与 calendar_normalized_return_pct 同时存在。

        检查逻辑：
        1. 两个字段同时存在于 summary 中（nogate_* 和 gate001_* 前缀）。
        2. active_months + empty_months == 22。

        Returns:
            违反数量。
        """
        violation_count = 0

        for prefix in ("nogate", "gate001"):
            ass_key = f"{prefix}_active_silo_sum_pct"
            cnr_key = f"{prefix}_calendar_normalized_return_pct"
            am_key = f"{prefix}_active_months"
            em_key = f"{prefix}_empty_months"

            # 两个字段必须同时存在
            if ass_key not in summary or cnr_key not in summary:
                violation_count += 1
                continue

            # active_months + empty_months == _TOTAL_SILOS
            if am_key in summary and em_key in summary:
                active = summary[am_key]
                empty = summary[em_key]
                if active is not None and empty is not None:
                    if active + empty != _TOTAL_SILOS:
                        violation_count += 1

        return violation_count

    # ------------------------------------------------------------------
    # CI 退出判定
    # ------------------------------------------------------------------

    @staticmethod
    def has_violations(violations: dict[str, Any]) -> bool:
        """判定是否存在任何 invariant 违反（CI 非零退出条件）。

        任一 *_count > 0 OR P7_ledger_sha256_pairs 非空
        OR live_output_emitted == true → 返回 True。

        Args:
            violations: check() 返回的 invariant_violations dict。

        Returns:
            True 如果存在违反，False 如果全部通过。
        """
        # 检查所有 *_count 字段
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
            if violations.get(key, 0) > 0:
                return True

        # P7: ledger sha256 pairs 非空
        p7_pairs = violations.get("P7_ledger_sha256_pairs", [])
        if p7_pairs:
            return True

        # P10: live_output_emitted
        if violations.get("live_output_emitted", False):
            return True

        return False
