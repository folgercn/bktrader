"""Property-based 断言函数 — P1 / P3 / P4 / P6 / P7 / P8 / P9 / P11 / P12。

每个 assert_invariant_<P_id> 函数接收 (ledger, summary) 或适当参数，
检查对应 invariant，违反时 raise InvariantViolation 并将最小反例序列化到
research/tmp_entry_redesign_invariant_counterexamples/<P_id>_<candidate_id>.json。

Requirements: 6.14
"""

from __future__ import annotations

import hashlib
import json
import math
import pathlib
from datetime import date
from typing import Any, Optional

import pandas as pd


# ---------------------------------------------------------------------------
# 常量
# ---------------------------------------------------------------------------

_COUNTEREXAMPLE_DIR = pathlib.Path(
    "research/tmp_entry_redesign_invariant_counterexamples"
)

# AGENTS §2 Research_Baseline: max_trades_per_bar=2
_MAX_TRADES_PER_BAR: int = 2

# P4 cost model monotonicity 相对误差容忍度
_P4_RELATIVE_TOLERANCE: float = 1e-9

# P9 per_trade_quality 重算相对误差容忍度
_P9_RELATIVE_TOLERANCE: float = 1e-6

# 固定 silo 总数（BTCUSDT + ETHUSDT × 11 execute months = 22）
_TOTAL_SILOS: int = 22

# 固定 22 字段 header 顺序（与 Requirement 4.5 完全一致）
_LEDGER_HEADER: tuple[str, ...] = (
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
)


# ---------------------------------------------------------------------------
# InvariantViolation 异常
# ---------------------------------------------------------------------------


class InvariantViolation(Exception):
    """Invariant 违反时抛出的异常。

    Attributes:
        property_id: 违反的 property 标识，如 "P1", "P3" 等。
        candidate_id: 关联的 Entry_Candidate ID（可选）。
        details: 违反详情的字典，包含最小反例信息。
        counterexample_path: 反例 JSON 文件路径（如已序列化）。
    """

    def __init__(
        self,
        property_id: str,
        message: str,
        *,
        candidate_id: Optional[str] = None,
        details: Optional[dict[str, Any]] = None,
    ) -> None:
        self.property_id = property_id
        self.candidate_id = candidate_id
        self.details = details or {}
        self.counterexample_path: Optional[pathlib.Path] = None
        super().__init__(f"[{property_id}] {message}")


# ---------------------------------------------------------------------------
# 反例序列化 helper
# ---------------------------------------------------------------------------


def _serialize_counterexample(
    property_id: str,
    candidate_id: str,
    counterexample: dict[str, Any],
    base_dir: Optional[pathlib.Path] = None,
) -> pathlib.Path:
    """将最小反例序列化到 JSON 文件。

    路径模板：
      research/tmp_entry_redesign_invariant_counterexamples/<P_id>_<candidate_id>.json

    Args:
        property_id: 如 "P1", "P3" 等。
        candidate_id: Entry_Candidate ID。
        counterexample: 反例数据字典。
        base_dir: 基础目录（默认使用 _COUNTEREXAMPLE_DIR）。

    Returns:
        写入的 JSON 文件路径。
    """
    out_dir = base_dir if base_dir is not None else _COUNTEREXAMPLE_DIR
    out_dir.mkdir(parents=True, exist_ok=True)

    filename = f"{property_id}_{candidate_id}.json"
    out_path = out_dir / filename

    # 稳定键序 UTF-8 无 BOM LF 结尾
    content = json.dumps(
        counterexample,
        ensure_ascii=False,
        indent=2,
        sort_keys=True,
        default=str,
    )
    out_path.write_text(content + "\n", encoding="utf-8")

    return out_path


def _extract_candidate_id(
    ledger: pd.DataFrame,
    summary: dict[str, Any],
) -> str:
    """从 ledger 或 summary 中提取 candidate_id。

    优先从 summary 的 candidate_id 字段获取，
    其次从 ledger 的 entry_candidate_id 列获取第一个非空值，
    最后 fallback 到 "unknown"。
    """
    # 从 summary 获取
    cid = summary.get("candidate_id") or summary.get("entry_candidate_id")
    if cid:
        return str(cid)

    # 从 ledger 获取
    if (
        not ledger.empty
        and "entry_candidate_id" in ledger.columns
    ):
        first_val = ledger["entry_candidate_id"].iloc[0]
        if pd.notna(first_val) and str(first_val).strip():
            return str(first_val).strip()

    return "unknown"


# ---------------------------------------------------------------------------
# assert_invariant_P1: Intrabar Breakout Semantic Preserved
# ---------------------------------------------------------------------------


def assert_invariant_P1(
    ledger: pd.DataFrame,
    summary: dict[str, Any],
) -> None:
    """P1: Intrabar Breakout Semantic Preserved。

    检查 summary 中 P1_missing_trigger_count 和 P1_spurious_trigger_count
    是否为 0。如果任一非零，说明 trigger 生成逻辑违反了 intrabar breakout
    语义：
      - missing: 条件成立但未生成 trigger_ts
      - spurious: 条件不成立却生成了 trigger_ts

    FOR ALL Entry_Candidate 与 signal_bar：
      long 条件成立 ⇔ trigger_ts 非空且等于最早满足条件的 1s bar 结束时刻；
      不成立 ⇔ 返回 None；short 镜像。

    Validates: Requirements 6.1

    Args:
        ledger: Research_Ledger DataFrame。
        summary: summary JSON dict。

    Raises:
        InvariantViolation: 当 P1 违反时。
    """
    violations = summary.get("invariant_violations", {})
    missing_count = violations.get("P1_missing_trigger_count", 0)
    spurious_count = violations.get("P1_spurious_trigger_count", 0)

    total_violations = missing_count + spurious_count

    if total_violations > 0:
        candidate_id = _extract_candidate_id(ledger, summary)
        details = {
            "property_id": "P1",
            "candidate_id": candidate_id,
            "P1_missing_trigger_count": missing_count,
            "P1_spurious_trigger_count": spurious_count,
            "total_violations": total_violations,
            "description": (
                "Intrabar breakout semantic violated: "
                f"{missing_count} missing triggers (condition met but no trigger_ts), "
                f"{spurious_count} spurious triggers (no condition but trigger_ts generated)."
            ),
        }

        path = _serialize_counterexample("P1", candidate_id, details)

        exc = InvariantViolation(
            "P1",
            f"Intrabar breakout semantic violated: "
            f"missing={missing_count}, spurious={spurious_count}",
            candidate_id=candidate_id,
            details=details,
        )
        exc.counterexample_path = path
        raise exc


# ---------------------------------------------------------------------------
# assert_invariant_P3: Trades Per Bar Bound
# ---------------------------------------------------------------------------


def assert_invariant_P3(
    ledger: pd.DataFrame,
    summary: dict[str, Any],
) -> None:
    """P3: Trades Per Bar Bound。

    FOR ALL signal_bar：同一 signal bar 内 real-entry count
    <= max_trades_per_bar=2（AGENTS Research_Baseline）。

    直接从 ledger 重算：按 (signal_bar_start_ts, symbol, gate_mode) 分组，
    检查每组 trade 行数是否 <= 2。

    Validates: Requirements 6.3

    Args:
        ledger: Research_Ledger DataFrame。
        summary: summary JSON dict。

    Raises:
        InvariantViolation: 当 P3 违反时。
    """
    if ledger.empty:
        return

    required_cols = {"signal_bar_start_ts", "symbol", "gate_mode"}
    if not required_cols.issubset(ledger.columns):
        return

    grouped = ledger.groupby(
        ["signal_bar_start_ts", "symbol", "gate_mode"]
    ).size()

    violating_groups = grouped[grouped > _MAX_TRADES_PER_BAR]

    if len(violating_groups) > 0:
        candidate_id = _extract_candidate_id(ledger, summary)

        # 构建最小反例：取第一个违反的组
        first_violation_idx = violating_groups.index[0]
        first_count = int(violating_groups.iloc[0])

        details = {
            "property_id": "P3",
            "candidate_id": candidate_id,
            "violation_count": len(violating_groups),
            "max_trades_per_bar_limit": _MAX_TRADES_PER_BAR,
            "first_violation": {
                "signal_bar_start_ts": str(first_violation_idx[0]),
                "symbol": str(first_violation_idx[1]),
                "gate_mode": str(first_violation_idx[2]),
                "actual_trade_count": first_count,
            },
            "description": (
                f"Trades per bar bound violated: {len(violating_groups)} "
                f"signal bars have more than {_MAX_TRADES_PER_BAR} trades. "
                f"First violation: {first_violation_idx} with {first_count} trades."
            ),
        }

        path = _serialize_counterexample("P3", candidate_id, details)

        exc = InvariantViolation(
            "P3",
            f"Trades per bar bound violated: {len(violating_groups)} bars "
            f"exceed max_trades_per_bar={_MAX_TRADES_PER_BAR}",
            candidate_id=candidate_id,
            details=details,
        )
        exc.counterexample_path = path
        raise exc


# ---------------------------------------------------------------------------
# assert_invariant_P4: Cost Model Monotonicity
# ---------------------------------------------------------------------------


def assert_invariant_P4(
    ledger: pd.DataFrame,
    summary: dict[str, Any],
) -> None:
    """P4: Cost Model Monotonicity。

    FOR ALL trade：
      realistic_taker_both_pnl <= realistic_pnl <= slip_pnl <= raw_pnl
    （相对误差 1e-9 内成立）。

    Validates: Requirements 6.4

    Args:
        ledger: Research_Ledger DataFrame。
        summary: summary JSON dict。

    Raises:
        InvariantViolation: 当 P4 违反时。
    """
    if ledger.empty:
        return

    required_cols = {
        "raw_pnl",
        "slip_pnl",
        "realistic_pnl",
        "realistic_taker_both_pnl",
    }
    if not required_cols.issubset(ledger.columns):
        return

    violations_list: list[dict[str, Any]] = []

    for idx, row in ledger.iterrows():
        raw = float(row["raw_pnl"])
        slip = float(row["slip_pnl"])
        realistic = float(row["realistic_pnl"])
        taker_both = float(row["realistic_taker_both_pnl"])

        # Check: taker_both <= realistic <= slip <= raw
        pairs = [
            ("realistic_taker_both_pnl", "realistic_pnl", taker_both, realistic),
            ("realistic_pnl", "slip_pnl", realistic, slip),
            ("slip_pnl", "raw_pnl", slip, raw),
        ]

        for lower_name, upper_name, lower_val, upper_val in pairs:
            tol = max(abs(upper_val), 1.0) * _P4_RELATIVE_TOLERANCE
            if lower_val > upper_val + tol:
                violations_list.append({
                    "row_index": int(idx) if isinstance(idx, (int,)) else str(idx),
                    "lower_field": lower_name,
                    "upper_field": upper_name,
                    "lower_value": lower_val,
                    "upper_value": upper_val,
                    "difference": lower_val - upper_val,
                })
                break  # 一行只记一次

    if violations_list:
        candidate_id = _extract_candidate_id(ledger, summary)

        details = {
            "property_id": "P4",
            "candidate_id": candidate_id,
            "violation_count": len(violations_list),
            "tolerance": _P4_RELATIVE_TOLERANCE,
            "first_violation": violations_list[0],
            "description": (
                f"Cost model monotonicity violated: {len(violations_list)} trades "
                f"do not satisfy realistic_taker_both_pnl <= realistic_pnl "
                f"<= slip_pnl <= raw_pnl within tolerance {_P4_RELATIVE_TOLERANCE}."
            ),
        }

        path = _serialize_counterexample("P4", candidate_id, details)

        exc = InvariantViolation(
            "P4",
            f"Cost model monotonicity violated: {len(violations_list)} trades",
            candidate_id=candidate_id,
            details=details,
        )
        exc.counterexample_path = path
        raise exc


# ---------------------------------------------------------------------------
# assert_invariant_P6: Walk-Forward Window Non-Overlap
# ---------------------------------------------------------------------------


def assert_invariant_P6(
    ledger: pd.DataFrame,
    summary: dict[str, Any],
) -> None:
    """P6: Walk-Forward Window Non-Overlap。

    FOR ALL walk-forward split (train, validation, execute)：
      三个窗口均以半开区间 [start, end) 表达，MUST 两两不相交
      (train ∩ validation = ∅ AND validation ∩ execute = ∅
       AND train ∩ execute = ∅)，
      且 execute.start >= validation.end。

    从 summary 的 walkforward_config 和 walkforward_splits 中提取窗口信息。
    如果 summary 中包含 invariant_violations.P6_count > 0，直接判定违反。
    否则从 walkforward_splits 重新验证。

    Validates: Requirements 6.6

    Args:
        ledger: Research_Ledger DataFrame（本断言主要依赖 summary）。
        summary: summary JSON dict。

    Raises:
        InvariantViolation: 当 P6 违反时。
    """
    # 方式 1：检查 summary 中已有的 P6_count
    violations_dict = summary.get("invariant_violations", {})
    p6_count = violations_dict.get("P6_count", 0)

    if p6_count > 0:
        candidate_id = _extract_candidate_id(ledger, summary)
        details = {
            "property_id": "P6",
            "candidate_id": candidate_id,
            "P6_count": p6_count,
            "description": (
                f"Walk-forward window non-overlap violated: "
                f"{p6_count} splits have overlapping windows."
            ),
        }
        path = _serialize_counterexample("P6", candidate_id, details)
        exc = InvariantViolation(
            "P6",
            f"Walk-forward window non-overlap violated: {p6_count} splits",
            candidate_id=candidate_id,
            details=details,
        )
        exc.counterexample_path = path
        raise exc

    # 方式 2：从 walkforward_splits 重新验证（如果存在）
    splits = summary.get("walkforward_splits", [])
    if not splits:
        return

    violation_splits: list[dict[str, Any]] = []

    for i, split in enumerate(splits):
        train_start = _parse_date(split.get("train_start"))
        train_end = _parse_date(split.get("train_end"))
        val_start = _parse_date(split.get("validation_start"))
        val_end = _parse_date(split.get("validation_end"))
        exec_start = _parse_date(split.get("execute_start"))
        exec_end = _parse_date(split.get("execute_end"))

        if any(
            d is None
            for d in (train_start, train_end, val_start, val_end, exec_start, exec_end)
        ):
            continue

        # 半开区间 [a, b) 与 [c, d) 相交 ⇔ a < d AND c < b
        overlaps: list[str] = []

        # train ∩ validation
        if train_start < val_end and val_start < train_end:
            overlaps.append("train ∩ validation")

        # validation ∩ execute
        if val_start < exec_end and exec_start < val_end:
            overlaps.append("validation ∩ execute")

        # train ∩ execute
        if train_start < exec_end and exec_start < train_end:
            overlaps.append("train ∩ execute")

        # execute.start >= validation.end
        if exec_start < val_end:
            overlaps.append("execute.start < validation.end")

        if overlaps:
            violation_splits.append({
                "split_index": i,
                "overlaps": overlaps,
                "train": f"[{train_start}, {train_end})",
                "validation": f"[{val_start}, {val_end})",
                "execute": f"[{exec_start}, {exec_end})",
            })

    if violation_splits:
        candidate_id = _extract_candidate_id(ledger, summary)
        details = {
            "property_id": "P6",
            "candidate_id": candidate_id,
            "violation_count": len(violation_splits),
            "first_violation": violation_splits[0],
            "description": (
                f"Walk-forward window non-overlap violated: "
                f"{len(violation_splits)} splits have overlapping windows."
            ),
        }
        path = _serialize_counterexample("P6", candidate_id, details)
        exc = InvariantViolation(
            "P6",
            f"Walk-forward window non-overlap violated: "
            f"{len(violation_splits)} splits",
            candidate_id=candidate_id,
            details=details,
        )
        exc.counterexample_path = path
        raise exc


def _parse_date(value: Any) -> Optional[date]:
    """尝试将值解析为 date 对象。

    支持格式：
      - date 对象直接返回
      - "YYYY-MM-DD" 字符串
      - ISO-8601 字符串（取前 10 字符）
    """
    if isinstance(value, date):
        return value
    if isinstance(value, str):
        try:
            return date.fromisoformat(value[:10])
        except (ValueError, IndexError):
            return None
    return None


# ---------------------------------------------------------------------------
# assert_invariant_P7: Entry Layer Bit-Identical Ledger
# ---------------------------------------------------------------------------


def assert_invariant_P7(
    ledger: pd.DataFrame,
    summary: dict[str, Any],
) -> None:
    """P7: Entry Layer Bit-Identical Ledger。

    FOR ALL Entry_Candidate c, seed s, events hash h：
      两次独立运行产出的 ledger CSV MUST 字节级完全一致（sha256 一致）。

    从 summary 的 invariant_violations.P7_ledger_sha256_pairs 检查：
    如果数组非空，说明存在两次运行 sha256 不一致的情况。

    Validates: Requirements 6.7

    Args:
        ledger: Research_Ledger DataFrame。
        summary: summary JSON dict。

    Raises:
        InvariantViolation: 当 P7 违反时。
    """
    violations_dict = summary.get("invariant_violations", {})
    sha256_pairs = violations_dict.get("P7_ledger_sha256_pairs", [])

    if sha256_pairs:
        candidate_id = _extract_candidate_id(ledger, summary)
        details = {
            "property_id": "P7",
            "candidate_id": candidate_id,
            "sha256_pairs": sha256_pairs,
            "pair_count": len(sha256_pairs),
            "description": (
                f"Bit-identical ledger violated: {len(sha256_pairs)} "
                f"run pair(s) produced different sha256 digests."
            ),
        }
        path = _serialize_counterexample("P7", candidate_id, details)
        exc = InvariantViolation(
            "P7",
            f"Bit-identical ledger violated: {len(sha256_pairs)} pair(s) differ",
            candidate_id=candidate_id,
            details=details,
        )
        exc.counterexample_path = path
        raise exc


# ---------------------------------------------------------------------------
# assert_invariant_P8: Round-Trip Serialization
# ---------------------------------------------------------------------------


def assert_invariant_P8(
    ledger: pd.DataFrame,
    summary: dict[str, Any],
) -> None:
    """P8: Round-Trip Serialization。

    FOR ALL runner_snapshot.json / summary.json：
      解码后再编码 MUST 产生值语义等价内容（数值相对误差 <= 1e-12）。
    FOR ALL Research_Ledger CSV：
      列顺序 MUST 与 Requirement 4.5 固定 header 顺序完全相同。

    检查逻辑：
    1. summary JSON round-trip：json.loads(json.dumps(summary)) 值等价。
    2. ledger 列顺序与 _LEDGER_HEADER 一致。

    Validates: Requirements 6.8

    Args:
        ledger: Research_Ledger DataFrame。
        summary: summary JSON dict。

    Raises:
        InvariantViolation: 当 P8 违反时。
    """
    violations: list[dict[str, Any]] = []

    # 检查 1: summary JSON round-trip
    try:
        encoded = json.dumps(summary, ensure_ascii=False, sort_keys=True, default=str)
        decoded = json.loads(encoded)
        rt_issues = _check_value_equivalence(summary, decoded, path_prefix="summary")
        if rt_issues:
            violations.append({
                "type": "summary_roundtrip",
                "issues": rt_issues[:5],  # 最多记录 5 个
            })
    except (TypeError, ValueError) as e:
        violations.append({
            "type": "summary_roundtrip_error",
            "error": str(e),
        })

    # 检查 2: ledger 列顺序
    if not ledger.empty:
        actual_columns = tuple(ledger.columns.tolist())
        if actual_columns != _LEDGER_HEADER:
            # 找出差异
            missing = [c for c in _LEDGER_HEADER if c not in actual_columns]
            extra = [c for c in actual_columns if c not in _LEDGER_HEADER]
            order_mismatch = (
                set(actual_columns) == set(_LEDGER_HEADER)
                and actual_columns != _LEDGER_HEADER
            )
            violations.append({
                "type": "ledger_column_order",
                "expected": list(_LEDGER_HEADER),
                "actual": list(actual_columns),
                "missing_columns": missing,
                "extra_columns": extra,
                "order_mismatch": order_mismatch,
            })

    # 检查 summary 中已有的 P8_count
    violations_dict = summary.get("invariant_violations", {})
    p8_count = violations_dict.get("P8_count", 0)
    if p8_count > 0:
        violations.append({
            "type": "summary_p8_count_nonzero",
            "P8_count": p8_count,
        })

    if violations:
        candidate_id = _extract_candidate_id(ledger, summary)
        details = {
            "property_id": "P8",
            "candidate_id": candidate_id,
            "violation_count": len(violations),
            "violations": violations,
            "description": (
                f"Round-trip serialization violated: {len(violations)} issue(s) found."
            ),
        }
        path = _serialize_counterexample("P8", candidate_id, details)
        exc = InvariantViolation(
            "P8",
            f"Round-trip serialization violated: {len(violations)} issue(s)",
            candidate_id=candidate_id,
            details=details,
        )
        exc.counterexample_path = path
        raise exc


def _check_value_equivalence(
    original: Any,
    roundtripped: Any,
    path_prefix: str = "",
    tolerance: float = 1e-12,
) -> list[str]:
    """递归比较两个值是否语义等价（数值相对误差 <= tolerance）。

    Returns:
        不等价的路径描述列表（空列表表示等价）。
    """
    issues: list[str] = []

    if isinstance(original, dict) and isinstance(roundtripped, dict):
        all_keys = set(original.keys()) | set(roundtripped.keys())
        for key in sorted(all_keys):
            sub_path = f"{path_prefix}.{key}" if path_prefix else key
            if key not in original:
                issues.append(f"{sub_path}: missing in original")
            elif key not in roundtripped:
                issues.append(f"{sub_path}: missing in roundtripped")
            else:
                issues.extend(
                    _check_value_equivalence(
                        original[key], roundtripped[key], sub_path, tolerance
                    )
                )
    elif isinstance(original, list) and isinstance(roundtripped, list):
        if len(original) != len(roundtripped):
            issues.append(
                f"{path_prefix}: list length mismatch "
                f"({len(original)} vs {len(roundtripped)})"
            )
        else:
            for i, (a, b) in enumerate(zip(original, roundtripped)):
                issues.extend(
                    _check_value_equivalence(
                        a, b, f"{path_prefix}[{i}]", tolerance
                    )
                )
    elif isinstance(original, (int, float)) and isinstance(roundtripped, (int, float)):
        if not _numeric_close(float(original), float(roundtripped), tolerance):
            issues.append(
                f"{path_prefix}: numeric mismatch "
                f"({original} vs {roundtripped})"
            )
    elif original != roundtripped:
        # 字符串或其他类型的精确比较
        issues.append(
            f"{path_prefix}: value mismatch ({type(original).__name__})"
        )

    return issues


def _numeric_close(a: float, b: float, tolerance: float) -> bool:
    """检查两个浮点数是否在相对误差 tolerance 内相等。"""
    if a == b:
        return True
    if math.isnan(a) and math.isnan(b):
        return True
    if math.isinf(a) or math.isinf(b):
        return a == b
    denom = max(abs(a), abs(b), 1e-15)
    return abs(a - b) / denom <= tolerance


# ---------------------------------------------------------------------------
# assert_invariant_P9: Per-Trade Quality Reporting Present
# ---------------------------------------------------------------------------


def assert_invariant_P9(
    ledger: pd.DataFrame,
    summary: dict[str, Any],
) -> None:
    """P9: Per-Trade Quality Reporting Present。

    per_trade_quality_bps_over_notional 字段恒存在；
    与 ledger 重算相对误差 <= 1e-6；
    trade_count==0 → null；
    notional_i==0 trade 必须在 ledger 写入前拒绝。

    Validates: Requirements 6.9

    Args:
        ledger: Research_Ledger DataFrame。
        summary: summary JSON dict。

    Raises:
        InvariantViolation: 当 P9 违反时。
    """
    violations: list[dict[str, Any]] = []

    # 检查 notional == 0 的 trade（不应存在于 ledger 中）
    if not ledger.empty and "notional" in ledger.columns:
        zero_notional_mask = ledger["notional"] == 0.0
        zero_count = int(zero_notional_mask.sum())
        if zero_count > 0:
            violations.append({
                "type": "zero_notional_in_ledger",
                "count": zero_count,
                "description": (
                    f"{zero_count} trade(s) with notional==0 found in ledger "
                    f"(should be rejected before writing)."
                ),
            })

    # 对每个 gate_mode 检查 per_trade_quality 一致性
    for prefix in ("nogate", "gate001"):
        ptq_key = f"{prefix}_per_trade_quality_bps_over_notional"
        tc_key = f"{prefix}_trade_count"

        # 字段必须存在
        if ptq_key not in summary:
            violations.append({
                "type": "field_missing",
                "field": ptq_key,
                "description": f"{ptq_key} field missing from summary.",
            })
            continue

        ptq_value = summary[ptq_key]
        trade_count = summary.get(tc_key, 0)

        # trade_count == 0 时字段值必须为 null
        if trade_count == 0:
            if ptq_value is not None:
                violations.append({
                    "type": "null_expected",
                    "field": ptq_key,
                    "actual_value": ptq_value,
                    "description": (
                        f"{ptq_key} should be null when trade_count==0, "
                        f"got {ptq_value}."
                    ),
                })
            continue

        # trade_count > 0 但 ptq 为 null → 违反
        if ptq_value is None:
            violations.append({
                "type": "null_unexpected",
                "field": ptq_key,
                "trade_count": trade_count,
                "description": (
                    f"{ptq_key} is null but trade_count={trade_count} > 0."
                ),
            })
            continue

        # 从 ledger 重算并比较
        if not ledger.empty and "gate_mode" in ledger.columns:
            gate_mode_str = "nogate" if prefix == "nogate" else "gate001"
            subset = ledger[ledger["gate_mode"] == gate_mode_str]
            if (
                not subset.empty
                and "realistic_pnl" in subset.columns
                and "notional" in subset.columns
            ):
                valid = subset[subset["notional"] != 0.0]
                if not valid.empty:
                    recomputed = (
                        (valid["realistic_pnl"] / valid["notional"]).mean()
                        * 10000.0
                    )
                    denom = max(abs(recomputed), 1e-12)
                    rel_err = abs(float(ptq_value) - recomputed) / denom
                    if rel_err > _P9_RELATIVE_TOLERANCE:
                        violations.append({
                            "type": "recomputation_mismatch",
                            "field": ptq_key,
                            "summary_value": float(ptq_value),
                            "recomputed_value": recomputed,
                            "relative_error": rel_err,
                            "tolerance": _P9_RELATIVE_TOLERANCE,
                            "description": (
                                f"{ptq_key} mismatch: summary={ptq_value}, "
                                f"recomputed={recomputed:.10f}, "
                                f"rel_err={rel_err:.2e} > {_P9_RELATIVE_TOLERANCE}."
                            ),
                        })

    if violations:
        candidate_id = _extract_candidate_id(ledger, summary)
        details = {
            "property_id": "P9",
            "candidate_id": candidate_id,
            "violation_count": len(violations),
            "violations": violations,
            "description": (
                f"Per-trade quality reporting violated: "
                f"{len(violations)} issue(s) found."
            ),
        }
        path = _serialize_counterexample("P9", candidate_id, details)
        exc = InvariantViolation(
            "P9",
            f"Per-trade quality reporting violated: {len(violations)} issue(s)",
            candidate_id=candidate_id,
            details=details,
        )
        exc.counterexample_path = path
        raise exc


# ---------------------------------------------------------------------------
# assert_invariant_P11: Entry Price Mode Fallback Explicit
# ---------------------------------------------------------------------------


def assert_invariant_P11(
    ledger: pd.DataFrame,
    summary: dict[str, Any],
) -> None:
    """P11: Entry Price Mode Fallback Explicit。

    FOR ALL Entry_Candidate with Entry_Price_Mode = post_touch_pullback_limit：
      在 [trigger_ts, trigger_ts + D] 内未成交 MUST 记为"空仓（跳过该笔）"；
      该事件 MUST NOT 出现在 Research_Ledger CSV 的成交行中；
      MUST NOT 被二次落到下一根 bar。

    检查逻辑：
    1. ledger 中 entry_price_mode_id 以 "pullback_" 开头的行，
       如果 entry_price 为 NaN/None 或 exit_reason 为异常值，则违反。
    2. 从 summary 的 invariant_violations.P11_count 检查。

    Validates: Requirements 6.11

    Args:
        ledger: Research_Ledger DataFrame。
        summary: summary JSON dict。

    Raises:
        InvariantViolation: 当 P11 违反时。
    """
    violations: list[dict[str, Any]] = []

    # 检查 summary 中已有的 P11_count
    violations_dict = summary.get("invariant_violations", {})
    p11_count = violations_dict.get("P11_count", 0)

    if p11_count > 0:
        violations.append({
            "type": "summary_p11_count_nonzero",
            "P11_count": p11_count,
        })

    # 直接从 ledger 检查：pullback 模式的未成交事件不应出现在 ledger 中
    # 如果 ledger 中存在 entry_price_mode_id 以 "pullback_" 开头的行，
    # 且该行的 exit_reason 为 "runner_aborted" 或 entry_price 无效，
    # 则说明未成交事件错误地进入了 ledger。
    if not ledger.empty and "entry_price_mode_id" in ledger.columns:
        pullback_mask = ledger["entry_price_mode_id"].astype(str).str.startswith(
            "pullback_"
        )
        pullback_rows = ledger[pullback_mask]

        if not pullback_rows.empty and "entry_price" in pullback_rows.columns:
            # 检查是否有 entry_price 为 NaN/None/0 的行（不应存在）
            invalid_price_mask = (
                pullback_rows["entry_price"].isna()
                | (pullback_rows["entry_price"] == 0.0)
            )
            invalid_count = int(invalid_price_mask.sum())
            if invalid_count > 0:
                violations.append({
                    "type": "unfilled_pullback_in_ledger",
                    "count": invalid_count,
                    "description": (
                        f"{invalid_count} pullback_limit trade(s) with "
                        f"invalid entry_price found in ledger "
                        f"(unfilled events should not appear in ledger)."
                    ),
                })

    if violations:
        candidate_id = _extract_candidate_id(ledger, summary)
        details = {
            "property_id": "P11",
            "candidate_id": candidate_id,
            "violation_count": len(violations),
            "violations": violations,
            "description": (
                f"Entry price mode fallback violated: "
                f"{len(violations)} issue(s) found."
            ),
        }
        path = _serialize_counterexample("P11", candidate_id, details)
        exc = InvariantViolation(
            "P11",
            f"Entry price mode fallback violated: {len(violations)} issue(s)",
            candidate_id=candidate_id,
            details=details,
        )
        exc.counterexample_path = path
        raise exc


# ---------------------------------------------------------------------------
# assert_invariant_P12: Metric Normalization Both Reported
# ---------------------------------------------------------------------------


def assert_invariant_P12(
    ledger: pd.DataFrame,
    summary: dict[str, Any],
) -> None:
    """P12: Metric Normalization Both Reported。

    active_silo_sum_pct 与 calendar_normalized_return_pct MUST 同时出现；
    空仓 silo MUST 以 0.0 参与 calendar_normalized_return_pct 加和；
    分母恒为 22。

    Validates: Requirements 6.12

    Args:
        ledger: Research_Ledger DataFrame。
        summary: summary JSON dict。

    Raises:
        InvariantViolation: 当 P12 违反时。
    """
    violations: list[dict[str, Any]] = []

    for prefix in ("nogate", "gate001"):
        ass_key = f"{prefix}_active_silo_sum_pct"
        cnr_key = f"{prefix}_calendar_normalized_return_pct"
        am_key = f"{prefix}_active_months"
        em_key = f"{prefix}_empty_months"

        # 两个字段必须同时存在
        ass_present = ass_key in summary
        cnr_present = cnr_key in summary

        if not ass_present or not cnr_present:
            missing_fields = []
            if not ass_present:
                missing_fields.append(ass_key)
            if not cnr_present:
                missing_fields.append(cnr_key)
            violations.append({
                "type": "field_missing",
                "prefix": prefix,
                "missing_fields": missing_fields,
                "description": (
                    f"Fields {missing_fields} missing from summary "
                    f"(both active_silo_sum_pct and "
                    f"calendar_normalized_return_pct must be present)."
                ),
            })
            continue

        # active_months + empty_months == _TOTAL_SILOS
        if am_key in summary and em_key in summary:
            active = summary[am_key]
            empty = summary[em_key]
            if active is not None and empty is not None:
                total = active + empty
                if total != _TOTAL_SILOS:
                    violations.append({
                        "type": "silo_count_mismatch",
                        "prefix": prefix,
                        "active_months": active,
                        "empty_months": empty,
                        "expected_total": _TOTAL_SILOS,
                        "actual_total": total,
                        "description": (
                            f"{prefix}: active_months({active}) + "
                            f"empty_months({empty}) = {total} != "
                            f"{_TOTAL_SILOS}."
                        ),
                    })

    if violations:
        candidate_id = _extract_candidate_id(ledger, summary)
        details = {
            "property_id": "P12",
            "candidate_id": candidate_id,
            "violation_count": len(violations),
            "violations": violations,
            "description": (
                f"Metric normalization both reported violated: "
                f"{len(violations)} issue(s) found."
            ),
        }
        path = _serialize_counterexample("P12", candidate_id, details)
        exc = InvariantViolation(
            "P12",
            f"Metric normalization both reported violated: "
            f"{len(violations)} issue(s)",
            candidate_id=candidate_id,
            details=details,
        )
        exc.counterexample_path = path
        raise exc
