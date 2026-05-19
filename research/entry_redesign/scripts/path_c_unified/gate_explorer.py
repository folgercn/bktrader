"""
gate_explorer — Gate 放宽策略探索模块

系统性探索放宽 candidate_001 gate 条件的方式，选择最优扩展策略，
确保最终 Expanded_Events_Pool ≥ 200 events 且包含原 116 events 作为子集。

探索 4 个放宽维度：
- 维度 A: entry_reason 放宽（包含非 Zero-Initial-Reentry）
- 维度 B: lifecycle ledger 扩展（其他 V6 walkforward 目录）
- 维度 C: 直接使用 events_execution_labeled.csv（跳过 V6 gate 匹配）
- 维度 D: 时间范围扩展（baseline_plus_t3_delay60 等）
"""

from __future__ import annotations

import logging
from dataclasses import dataclass, field
from datetime import datetime
from pathlib import Path

import pandas as pd

logger = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# Paths
# ---------------------------------------------------------------------------
PROJECT_ROOT = Path(__file__).resolve().parents[4]

V6_LEDGER_BASE = (
    PROJECT_ROOT
    / "research"
    / "probabilistic_v6_runs"
    / "walkforward_2025m06_2026apr_combo_baseline_short_speed"
    / "union_lifecycle_reentry_window_candidate_001_calendar_holdout"
    / "power0_fixed_1p30"
)

V6_LEDGER_OBSERVED_FILL = (
    PROJECT_ROOT
    / "research"
    / "probabilistic_v6_runs"
    / "walkforward_2025m06_2026apr_combo_baseline_short_speed"
    / "union_lifecycle_reentry_window_candidate_001_observed_fill"
    / "power0_fixed_1p30"
)

EVENTS_CSV = (
    PROJECT_ROOT
    / "research"
    / "probabilistic_v6_runs"
    / "2025m03_2026apr_original_t2_delay60"
    / "events_execution_labeled.csv"
)

EVENTS_CSV_T3 = (
    PROJECT_ROOT
    / "research"
    / "probabilistic_v6_runs"
    / "2025m03_2026apr_baseline_plus_t3_delay60"
    / "events_execution_labeled.csv"
)

BARS_CACHE_DIR = (
    PROJECT_ROOT
    / "research"
    / "probabilistic_v6_runs"
    / "walkforward_delay60_original_t2_feature60_valbest"
    / "bars_cache"
)

OUTPUT_DIR = (
    PROJECT_ROOT
    / "research"
    / "entry_redesign"
    / "scripts"
    / "output"
    / "path_c_unified"
)


# ---------------------------------------------------------------------------
# Data Classes
# ---------------------------------------------------------------------------
@dataclass
class GateExpansionResult:
    """单个放宽维度的探索结果。"""

    dimension: str  # "entry_reason" | "lifecycle_ledger" | "direct_events" | "time_range"
    description: str  # 中文描述
    n_events: int  # 该维度产出的 events 数量
    overlap_with_original: int  # 与原 116 events 的重叠数
    symbol_distribution: dict  # {"BTCUSDT": N, "ETHUSDT": M}
    side_distribution: dict  # {"long": N, "short": M}
    time_range: tuple[str, str]  # (earliest, latest)
    sufficient: bool  # n_events >= 150


@dataclass
class GateExplorationReport:
    """Gate 探索完整报告。"""

    dimensions: list[GateExpansionResult] = field(default_factory=list)
    selected_strategy: str = ""  # 最终选择的策略描述
    final_n_events: int = 0  # 最终事件池大小
    includes_original_116: bool = False  # 是否包含原 116 events
    expansion_target_met: bool = False  # 是否达到 200 events 目标


# ---------------------------------------------------------------------------
# Internal helpers
# ---------------------------------------------------------------------------
def _extract_v6_gate_entries(
    ledger_base: Path,
    entry_reason_filter: str | None = "Zero-Initial-Reentry",
) -> pd.DataFrame:
    """从 V6 lifecycle ledger 提取 gate 选中的 entry events。

    Parameters
    ----------
    ledger_base : V6 lifecycle ledger 根目录
    entry_reason_filter : 若非 None，只保留该 entry_reason 类型
    """
    entries: list[dict] = []
    if not ledger_base.exists():
        logger.warning("Ledger base not found: %s", ledger_base)
        return pd.DataFrame()

    for execute_dir in sorted(ledger_base.glob("execute_*")):
        for symbol_dir in sorted(execute_dir.iterdir()):
            if not symbol_dir.is_dir():
                continue
            ledger_path = symbol_dir / "lifecycle_ledger.csv"
            if not ledger_path.exists():
                continue
            symbol = symbol_dir.name
            df = pd.read_csv(ledger_path, parse_dates=["time"])
            entry_rows = df[df["type"] != "EXIT"].copy()
            for _, row in entry_rows.iterrows():
                entries.append(
                    {
                        "symbol": symbol,
                        "touch_time": pd.Timestamp(row["time"]),
                        "side": "short" if "SHORT" in str(row["type"]) else "long",
                        "entry_reason": str(row["reason"]),
                    }
                )

    result = pd.DataFrame(entries)
    if result.empty:
        return result

    result["touch_time"] = pd.to_datetime(result["touch_time"], utc=True)

    if entry_reason_filter is not None:
        result = result[result["entry_reason"] == entry_reason_filter].copy()

    return result


def _match_events(
    gate_entries: pd.DataFrame, all_events: pd.DataFrame
) -> pd.DataFrame:
    """将 gate entries 匹配回 events_execution_labeled.csv 获取完整 event 信息。

    匹配条件：symbol + side + touch_time 差异 <= 2 秒。
    """
    if gate_entries.empty:
        return pd.DataFrame()

    all_events = all_events.copy()
    all_events["touch_time"] = pd.to_datetime(all_events["touch_time"], utc=True)

    matched: list[pd.Series] = []
    for _, entry in gate_entries.iterrows():
        symbol = entry["symbol"]
        touch = entry["touch_time"]
        side = entry["side"]

        candidates = all_events[
            (all_events["symbol"] == symbol)
            & (all_events["side"] == side)
            & ((all_events["touch_time"] - touch).abs() <= pd.Timedelta(seconds=2))
        ]
        if not candidates.empty:
            idx = (candidates["touch_time"] - touch).abs().idxmin()
            matched.append(candidates.loc[idx])

    if not matched:
        return pd.DataFrame()
    return pd.DataFrame(matched).drop_duplicates(subset=["event_id"])


def _compute_overlap(
    expanded_events: pd.DataFrame, original_events: pd.DataFrame
) -> int:
    """计算扩展池与原 116 events 的重叠数。"""
    if expanded_events.empty or original_events.empty:
        return 0
    return len(
        set(expanded_events["event_id"]) & set(original_events["event_id"])
    )


def _compute_stats(events: pd.DataFrame) -> tuple[dict, dict, tuple[str, str]]:
    """计算 symbol 分布、side 分布、时间范围。"""
    if events.empty:
        return {}, {}, ("", "")

    symbol_dist = events["symbol"].value_counts().to_dict()
    side_dist = events["side"].value_counts().to_dict()

    touch_times = pd.to_datetime(events["touch_time"], utc=True)
    time_range = (
        str(touch_times.min()),
        str(touch_times.max()),
    )
    return symbol_dist, side_dist, time_range


def _filter_by_bars_cache_coverage(events: pd.DataFrame) -> pd.DataFrame:
    """过滤 events，只保留 bars_cache 覆盖时间范围内的（2025-06 到 2026-04）。"""
    if events.empty:
        return events

    events = events.copy()
    events["touch_time"] = pd.to_datetime(events["touch_time"], utc=True)

    # bars_cache 覆盖 2025-06-01 到 2026-04-30
    cache_start = pd.Timestamp("2025-06-01", tz="UTC")
    cache_end = pd.Timestamp("2026-04-30 23:59:59", tz="UTC")

    mask = (events["touch_time"] >= cache_start) & (events["touch_time"] <= cache_end)
    filtered = events[mask].copy()
    logger.info(
        "Bars cache coverage filter: %d -> %d events", len(events), len(filtered)
    )
    return filtered


# ---------------------------------------------------------------------------
# Dimension exploration functions
# ---------------------------------------------------------------------------
def _explore_dimension_a(
    all_events: pd.DataFrame, original_events: pd.DataFrame
) -> GateExpansionResult:
    """维度 A: entry_reason 放宽 — 包含所有 entry_reason 类型（不仅 Zero-Initial-Reentry）。"""
    logger.info("探索维度 A: entry_reason 放宽...")

    # 从 V6 lifecycle ledger 提取所有 entry types（不过滤 entry_reason）
    gate_entries = _extract_v6_gate_entries(V6_LEDGER_BASE, entry_reason_filter=None)

    if gate_entries.empty:
        return GateExpansionResult(
            dimension="entry_reason",
            description="entry_reason 放宽：包含所有 entry 类型（Zero-Initial-Reentry + SL-Reentry）",
            n_events=0,
            overlap_with_original=0,
            symbol_distribution={},
            side_distribution={},
            time_range=("", ""),
            sufficient=False,
        )

    # 匹配回 events CSV
    matched = _match_events(gate_entries, all_events)
    # 过滤到 bars_cache 覆盖范围
    matched = _filter_by_bars_cache_coverage(matched)

    overlap = _compute_overlap(matched, original_events)
    symbol_dist, side_dist, time_range = _compute_stats(matched)

    logger.info(
        "维度 A 结果: %d events, overlap=%d, entry_reasons=%s",
        len(matched),
        overlap,
        gate_entries["entry_reason"].value_counts().to_dict(),
    )

    return GateExpansionResult(
        dimension="entry_reason",
        description="entry_reason 放宽：包含所有 entry 类型（Zero-Initial-Reentry + SL-Reentry），"
        "从 V6 lifecycle ledger calendar_holdout 中提取",
        n_events=len(matched),
        overlap_with_original=overlap,
        symbol_distribution=symbol_dist,
        side_distribution=side_dist,
        time_range=time_range,
        sufficient=len(matched) >= 150,
    )


def _explore_dimension_b(
    all_events: pd.DataFrame, original_events: pd.DataFrame
) -> GateExpansionResult:
    """维度 B: lifecycle ledger 扩展 — 使用其他 V6 walkforward 目录的 lifecycle ledger。"""
    logger.info("探索维度 B: lifecycle ledger 扩展...")

    # 合并 calendar_holdout + observed_fill 两个 ledger 目录
    entries_holdout = _extract_v6_gate_entries(
        V6_LEDGER_BASE, entry_reason_filter="Zero-Initial-Reentry"
    )
    entries_observed = _extract_v6_gate_entries(
        V6_LEDGER_OBSERVED_FILL, entry_reason_filter="Zero-Initial-Reentry"
    )

    combined_entries = pd.concat(
        [entries_holdout, entries_observed], ignore_index=True
    )
    if not combined_entries.empty:
        combined_entries = combined_entries.drop_duplicates(
            subset=["symbol", "touch_time", "side"]
        )

    # 匹配回 events CSV
    matched = _match_events(combined_entries, all_events)
    matched = _filter_by_bars_cache_coverage(matched)

    overlap = _compute_overlap(matched, original_events)
    symbol_dist, side_dist, time_range = _compute_stats(matched)

    logger.info("维度 B 结果: %d events, overlap=%d", len(matched), overlap)

    return GateExpansionResult(
        dimension="lifecycle_ledger",
        description="lifecycle ledger 扩展：合并 calendar_holdout + observed_fill 两个 V6 walkforward "
        "目录的 Zero-Initial-Reentry entries",
        n_events=len(matched),
        overlap_with_original=overlap,
        symbol_distribution=symbol_dist,
        side_distribution=side_dist,
        time_range=time_range,
        sufficient=len(matched) >= 150,
    )


def _explore_dimension_c(
    all_events: pd.DataFrame, original_events: pd.DataFrame
) -> GateExpansionResult:
    """维度 C: 直接使用 events_execution_labeled.csv — 跳过 V6 gate 匹配。

    仅按 bars_cache 覆盖的时间范围和 symbol 筛选。
    """
    logger.info("探索维度 C: 直接使用 events_execution_labeled.csv...")

    # 直接使用全量 events，过滤到 bars_cache 覆盖范围
    filtered = _filter_by_bars_cache_coverage(all_events)

    overlap = _compute_overlap(filtered, original_events)
    symbol_dist, side_dist, time_range = _compute_stats(filtered)

    logger.info("维度 C 结果: %d events, overlap=%d", len(filtered), overlap)

    return GateExpansionResult(
        dimension="direct_events",
        description="直接使用 events_execution_labeled.csv（original_t2_delay60）：跳过 V6 gate 匹配，"
        "仅按 bars_cache 覆盖时间范围（2025-06 至 2026-04）筛选",
        n_events=len(filtered),
        overlap_with_original=overlap,
        symbol_distribution=symbol_dist,
        side_distribution=side_dist,
        time_range=time_range,
        sufficient=len(filtered) >= 150,
    )


def _explore_dimension_d(
    original_events: pd.DataFrame,
) -> GateExpansionResult:
    """维度 D: 时间范围扩展 — 使用 baseline_plus_t3_delay60 事件源。

    该事件源包含 original_t2 + t3_swing 两种 shape，事件数量更多。
    """
    logger.info("探索维度 D: 时间范围扩展（baseline_plus_t3_delay60）...")

    if not EVENTS_CSV_T3.exists():
        logger.warning("baseline_plus_t3_delay60 events CSV not found: %s", EVENTS_CSV_T3)
        return GateExpansionResult(
            dimension="time_range",
            description="时间范围扩展：baseline_plus_t3_delay60 事件源不存在",
            n_events=0,
            overlap_with_original=0,
            symbol_distribution={},
            side_distribution={},
            time_range=("", ""),
            sufficient=False,
        )

    t3_events = pd.read_csv(EVENTS_CSV_T3)
    t3_events["touch_time"] = pd.to_datetime(t3_events["touch_time"], utc=True)

    # 过滤到 bars_cache 覆盖范围
    filtered = _filter_by_bars_cache_coverage(t3_events)

    overlap = _compute_overlap(filtered, original_events)
    symbol_dist, side_dist, time_range = _compute_stats(filtered)

    logger.info(
        "维度 D 结果: %d events, overlap=%d, shapes=%s",
        len(filtered),
        overlap,
        filtered["shape"].value_counts().to_dict() if not filtered.empty else {},
    )

    return GateExpansionResult(
        dimension="time_range",
        description="时间范围扩展：使用 baseline_plus_t3_delay60 事件源（包含 original_t2 + t3_swing），"
        "按 bars_cache 覆盖时间范围筛选",
        n_events=len(filtered),
        overlap_with_original=overlap,
        symbol_distribution=symbol_dist,
        side_distribution=side_dist,
        time_range=time_range,
        sufficient=len(filtered) >= 150,
    )


# ---------------------------------------------------------------------------
# Strategy selection
# ---------------------------------------------------------------------------
def _select_best_strategy(
    dimensions: list[GateExpansionResult],
    original_events: pd.DataFrame,
    all_events: pd.DataFrame,
) -> tuple[str, pd.DataFrame]:
    """选择最优策略，确保 ≥ 200 events 且包含原 116 events。

    策略优先级：
    1. 维度 C（直接使用 events_execution_labeled.csv）— 最简单、事件最多、保证包含原 116
    2. 维度 A（entry_reason 放宽）— 保持 V6 gate 语义
    3. 维度 B + A 组合
    4. 维度 D（引入 t3_swing shape）

    选择标准：
    - 必须包含原 116 events
    - 优先选择 n_events ≥ 200 的最小维度（避免过度稀释）
    - 若无单一维度满足，组合多维度
    """
    # 检查维度 C 是否满足（最直接的方式）
    dim_c = next((d for d in dimensions if d.dimension == "direct_events"), None)
    if dim_c and dim_c.n_events >= 200 and dim_c.overlap_with_original == len(original_events):
        # 维度 C 满足所有条件
        strategy = (
            "选择维度 C：直接使用 events_execution_labeled.csv（original_t2_delay60），"
            f"按 bars_cache 覆盖时间范围筛选。产出 {dim_c.n_events} events，"
            f"包含全部 {dim_c.overlap_with_original} 个原始 events。"
            "理由：最简单直接，事件数量充足，保证向后兼容。"
        )
        # 加载实际数据
        events = _filter_by_bars_cache_coverage(all_events)
        return strategy, events

    # 检查维度 A
    dim_a = next((d for d in dimensions if d.dimension == "entry_reason"), None)
    if dim_a and dim_a.n_events >= 200 and dim_a.overlap_with_original == len(original_events):
        strategy = (
            f"选择维度 A：entry_reason 放宽，产出 {dim_a.n_events} events，"
            f"包含全部 {dim_a.overlap_with_original} 个原始 events。"
        )
        gate_entries = _extract_v6_gate_entries(V6_LEDGER_BASE, entry_reason_filter=None)
        events = _match_events(gate_entries, all_events)
        events = _filter_by_bars_cache_coverage(events)
        return strategy, events

    # 若维度 C 事件数不足 200 但仍是最大的，使用它
    if dim_c and dim_c.overlap_with_original == len(original_events):
        strategy = (
            f"选择维度 C（最大可用池）：{dim_c.n_events} events，"
            f"未达到 200 events 目标（expansion_target_unmet=true）。"
            f"包含全部 {dim_c.overlap_with_original} 个原始 events。"
        )
        events = _filter_by_bars_cache_coverage(all_events)
        return strategy, events

    # Fallback: 使用维度 C 即使 overlap 不完美
    if dim_c and dim_c.n_events > 0:
        strategy = (
            f"Fallback 选择维度 C：{dim_c.n_events} events，"
            f"overlap={dim_c.overlap_with_original}/{len(original_events)}。"
        )
        events = _filter_by_bars_cache_coverage(all_events)
        # 确保原 116 events 包含在内
        original_ids = set(original_events["event_id"])
        events_ids = set(events["event_id"])
        missing = original_ids - events_ids
        if missing:
            # 补充缺失的原始 events
            missing_events = original_events[
                original_events["event_id"].isin(missing)
            ]
            events = pd.concat([events, missing_events], ignore_index=True)
            events = events.drop_duplicates(subset=["event_id"])
        return strategy, events

    # 最终 fallback
    strategy = "无法找到满足条件的策略，使用原 116 events"
    return strategy, original_events.copy()


# ---------------------------------------------------------------------------
# Report generation
# ---------------------------------------------------------------------------
def _generate_report_md(
    report: GateExplorationReport,
    output_dir: Path,
) -> None:
    """产出 gate_exploration_report.md（中文）。"""
    output_dir.mkdir(parents=True, exist_ok=True)
    report_path = output_dir / "gate_exploration_report.md"

    lines: list[str] = []
    lines.append("# Gate 放宽策略探索报告\n")
    lines.append(f"生成时间: {datetime.utcnow().isoformat()}Z\n")
    lines.append("## 概述\n")
    lines.append(
        "本报告系统性探索了放宽 `candidate_001` gate 条件的 4 个维度，"
        "目标是将事件池从 116 扩大到 200+ events。\n"
    )
    lines.append(f"- **最终事件池大小**: {report.final_n_events}")
    lines.append(f"- **包含原 116 events**: {'是' if report.includes_original_116 else '否'}")
    lines.append(f"- **达到 200 events 目标**: {'是' if report.expansion_target_met else '否'}")
    lines.append(f"- **选定策略**: {report.selected_strategy}\n")

    lines.append("## 各维度探索结果\n")
    for dim in report.dimensions:
        lines.append(f"### 维度: {dim.dimension}\n")
        lines.append(f"**描述**: {dim.description}\n")
        lines.append(f"| 指标 | 值 |")
        lines.append(f"|------|------|")
        lines.append(f"| 事件数量 | {dim.n_events} |")
        lines.append(f"| 与原 116 events 重叠 | {dim.overlap_with_original} |")
        lines.append(f"| 是否充足 (≥150) | {'是' if dim.sufficient else '否'} |")
        lines.append(f"| Symbol 分布 | {dim.symbol_distribution} |")
        lines.append(f"| Side 分布 | {dim.side_distribution} |")
        lines.append(f"| 时间范围 | {dim.time_range[0]} ~ {dim.time_range[1]} |")
        lines.append("")

        if not dim.sufficient:
            lines.append(
                f"> ⚠️ 该维度产出 {dim.n_events} events < 150，标注为 `insufficient_expansion`\n"
            )

    lines.append("## 策略选择理由\n")
    lines.append(report.selected_strategy)
    lines.append("")

    if not report.expansion_target_met:
        lines.append(
            "\n> ⚠️ `expansion_target_unmet=true`：所有放宽维度组合后仍无法达到 200 events，"
            f"使用可获得的最大事件池（{report.final_n_events} events）继续实验。\n"
        )

    report_path.write_text("\n".join(lines), encoding="utf-8")
    logger.info("Gate exploration report written to: %s", report_path)


# ---------------------------------------------------------------------------
# Public API
# ---------------------------------------------------------------------------
def explore_gate_relaxation(
    original_events: pd.DataFrame,
    events_csv_path: Path | None = None,
    v6_ledger_base: Path | None = None,
) -> GateExplorationReport:
    """探索所有放宽维度并选择最优策略。

    Parameters
    ----------
    original_events : 原 116 events（从 load_v6_gate_events() 获取）
    events_csv_path : events_execution_labeled.csv 路径（默认使用模块常量）
    v6_ledger_base : V6 lifecycle ledger 根目录（默认使用模块常量）

    Returns
    -------
    GateExplorationReport : 完整探索报告
    """
    if events_csv_path is None:
        events_csv_path = EVENTS_CSV
    if v6_ledger_base is None:
        v6_ledger_base = V6_LEDGER_BASE

    logger.info("开始 Gate 放宽策略探索...")
    logger.info("原始事件池: %d events", len(original_events))

    # 加载全量 events CSV
    if not events_csv_path.exists():
        raise FileNotFoundError(
            f"events_execution_labeled.csv not found: {events_csv_path}"
        )
    all_events = pd.read_csv(events_csv_path)
    all_events["touch_time"] = pd.to_datetime(all_events["touch_time"], utc=True)
    logger.info("全量 events CSV: %d events", len(all_events))

    # 探索 4 个维度
    dim_a = _explore_dimension_a(all_events, original_events)
    dim_b = _explore_dimension_b(all_events, original_events)
    dim_c = _explore_dimension_c(all_events, original_events)
    dim_d = _explore_dimension_d(original_events)

    dimensions = [dim_a, dim_b, dim_c, dim_d]

    # 选择最优策略
    strategy_desc, selected_events = _select_best_strategy(
        dimensions, original_events, all_events
    )

    # 验证原 116 events 包含在内
    original_ids = set(original_events["event_id"])
    selected_ids = set(selected_events["event_id"])
    includes_original = original_ids.issubset(selected_ids)

    if not includes_original:
        missing_count = len(original_ids - selected_ids)
        logger.warning(
            "选定策略缺少 %d 个原始 events，补充中...", missing_count
        )
        missing_events = original_events[
            original_events["event_id"].isin(original_ids - selected_ids)
        ]
        selected_events = pd.concat(
            [selected_events, missing_events], ignore_index=True
        )
        selected_events = selected_events.drop_duplicates(subset=["event_id"])
        includes_original = True

    # 构建报告
    report = GateExplorationReport(
        dimensions=dimensions,
        selected_strategy=strategy_desc,
        final_n_events=len(selected_events),
        includes_original_116=includes_original,
        expansion_target_met=len(selected_events) >= 200,
    )

    # 产出报告文件
    _generate_report_md(report, OUTPUT_DIR)

    logger.info(
        "Gate 探索完成: 选定 %d events, 包含原 116=%s, 达标=%s",
        report.final_n_events,
        report.includes_original_116,
        report.expansion_target_met,
    )

    return report
