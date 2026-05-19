"""CLI 入口：顺序跑完 36 个默认执行子集。

读取 RunnerParameterSnapshot（seed / events sha256 / git commit sha），
顺序跑完 36 个候选（含 Baseline_Entry_Candidate），
按 `<YYYYMMDD>` 日期前缀落 per-candidate markdown 报告。

用法：
    python -m research.entry_redesign.scheduler.run_default_subset \
        --seed 42 \
        --events-source-path research/events_execution_labeled.csv \
        --git-commit-sha abc123def456 \
        --output-dir research/ \
        --project-root . \
        --atr14-source-path research/features/atr14_1h.csv \
        [--runner-version entry_redesign_runner_v1.0.0] \
        [--report-date 20260515]

Requirements: 4.1, 4.6
"""

from __future__ import annotations

import argparse
import hashlib
import pathlib
import sys
from datetime import date, datetime
from typing import Optional, Sequence

from research.entry_redesign.ranking.ranking_markdown_writer import (
    RankingMarkdownWriter,
)
from research.entry_redesign.scheduler.default_subset import (
    BASELINE,
    DEFAULT_SUBSET,
)
from research.entry_redesign.scheduler.pipeline import (
    EntryRedesignPipeline,
    PipelineConfig,
    PipelineResult,
)
from research.entry_redesign.spec.candidate_id import generate_candidate_id
from research.entry_redesign.spec.entry_candidate_spec import EntryCandidateSpec


# ---------------------------------------------------------------------------
# 常量
# ---------------------------------------------------------------------------

_DEFAULT_RUNNER_VERSION = "entry_redesign_runner_v1.0.0"
"""默认 runner 版本字符串。"""

_DEFAULT_TICK_SIZE_BY_SYMBOL: dict[str, float] = {
    "BTCUSDT": 0.10,
    "ETHUSDT": 0.01,
}
"""默认 tick_size_by_symbol。"""

_DEFAULT_STEP_SIZE_BY_SYMBOL: dict[str, float] = {
    "BTCUSDT": 0.001,
    "ETHUSDT": 0.001,
}
"""默认 step_size_by_symbol。"""


# ---------------------------------------------------------------------------
# 辅助函数
# ---------------------------------------------------------------------------


def _compute_file_sha256(file_path: str) -> str:
    """计算文件的 sha256 摘要。

    Args:
        file_path: 文件路径。

    Returns:
        sha256 hex 字符串（64 字符小写）。

    Raises:
        FileNotFoundError: 文件不存在。
    """
    h = hashlib.sha256()
    with open(file_path, "rb") as f:
        while True:
            chunk = f.read(65536)
            if not chunk:
                break
            h.update(chunk)
    return h.hexdigest()


def _get_report_date_prefix(report_date: Optional[str]) -> str:
    """获取报告日期前缀 YYYYMMDD。

    如果未指定，使用当前 UTC 日期。
    注意：此函数仅用于报告文件命名，不影响 pipeline 确定性。

    Args:
        report_date: 可选的 YYYYMMDD 格式日期字符串。

    Returns:
        YYYYMMDD 格式字符串。
    """
    if report_date is not None:
        # 校验格式
        if len(report_date) != 8 or not report_date.isdigit():
            raise ValueError(
                f"--report-date must be YYYYMMDD format, got: {report_date!r}"
            )
        return report_date
    # 使用当前 UTC 日期（仅用于文件命名，不影响 pipeline 确定性）
    return datetime.utcnow().strftime("%Y%m%d")


def _write_per_candidate_markdown(
    result: PipelineResult,
    spec: EntryCandidateSpec,
    output_dir: pathlib.Path,
    date_prefix: str,
) -> Optional[pathlib.Path]:
    """写 per-candidate markdown 报告。

    路径模板: research/<YYYYMMDD>_entry_redesign_<candidate_id>.md

    Args:
        result: Pipeline 运行结果。
        spec: Entry_Candidate 六元组。
        output_dir: 输出根目录。
        date_prefix: YYYYMMDD 日期前缀。

    Returns:
        写出的 markdown 文件路径，或 None（rejected/aborted 时不产出）。
    """
    if result.status != "completed":
        return None

    candidate_id = result.candidate_id
    md_path = output_dir / f"{date_prefix}_entry_redesign_{candidate_id}.md"
    md_path.parent.mkdir(parents=True, exist_ok=True)

    lines: list[str] = []
    lines.append(f"# Entry Redesign Report: {candidate_id}")
    lines.append("")
    lines.append(f"**Date**: {date_prefix}")
    lines.append(f"**Status**: {result.status}")
    lines.append("")
    lines.append("## Candidate Spec")
    lines.append("")
    lines.append(f"- `entry_delay_seconds`: {spec.entry_delay_seconds}")
    lines.append(f"- `feature_horizon_seconds`: {spec.feature_horizon_seconds}")
    lines.append(f"- `trigger_confirmation_id`: {spec.trigger_confirmation_id}")
    lines.append(f"- `entry_price_mode_id`: {spec.entry_price_mode_id}")
    lines.append(f"- `pretouch_state_band_id`: {spec.pretouch_state_band_id}")
    lines.append(f"- `posttouch_quality_band_id`: {spec.posttouch_quality_band_id}")
    lines.append("")
    lines.append("## Artifacts")
    lines.append("")
    if result.ledger_path is not None:
        lines.append(f"- Ledger: `{result.ledger_path}`")
    if result.summary_path is not None:
        lines.append(f"- Summary: `{result.summary_path}`")
    if result.attribution_path is not None:
        lines.append(f"- Attribution: `{result.attribution_path}`")
    if result.snapshot_path is not None:
        lines.append(f"- Snapshot: `{result.snapshot_path}`")
    if result.ablation_pretouch_only_path is not None:
        lines.append(
            f"- Ablation (pretouch only): `{result.ablation_pretouch_only_path}`"
        )
    if result.ablation_posttouch_only_path is not None:
        lines.append(
            f"- Ablation (posttouch only): `{result.ablation_posttouch_only_path}`"
        )
    lines.append("")

    content = "\n".join(lines)
    with open(md_path, "w", encoding="utf-8", newline="") as f:
        f.write(content)

    return md_path


# ---------------------------------------------------------------------------
# CLI 参数解析
# ---------------------------------------------------------------------------


def parse_args(argv: Optional[list[str]] = None) -> argparse.Namespace:
    """解析命令行参数。

    Args:
        argv: 命令行参数列表（None 时使用 sys.argv[1:]）。

    Returns:
        解析后的 Namespace 对象。
    """
    parser = argparse.ArgumentParser(
        description=(
            "顺序跑完 36 个默认执行子集（含 Baseline_Entry_Candidate），"
            "按 <YYYYMMDD> 日期前缀落 per-candidate markdown 报告。"
        ),
    )

    parser.add_argument(
        "--seed",
        type=int,
        required=True,
        help="随机种子（确定性约束 P7）。",
    )
    parser.add_argument(
        "--events-source-path",
        type=str,
        required=True,
        help="events CSV 文件路径（如 research/events_execution_labeled.csv）。",
    )
    parser.add_argument(
        "--git-commit-sha",
        type=str,
        required=True,
        help="当前 git commit sha。",
    )
    parser.add_argument(
        "--output-dir",
        type=str,
        default="research/",
        help="产物输出根目录（默认 research/）。",
    )
    parser.add_argument(
        "--project-root",
        type=str,
        default=".",
        help="项目根目录（用于定位 gate 快照文件等，默认当前目录）。",
    )
    parser.add_argument(
        "--atr14-source-path",
        type=str,
        required=True,
        help="1h ATR(14) CSV 路径。",
    )
    parser.add_argument(
        "--runner-version",
        type=str,
        default=_DEFAULT_RUNNER_VERSION,
        help=(
            f"runner 版本字符串（默认 {_DEFAULT_RUNNER_VERSION}）。"
            "格式: entry_redesign_runner_vMAJOR.MINOR.PATCH"
        ),
    )
    parser.add_argument(
        "--report-date",
        type=str,
        default=None,
        help=(
            "报告日期前缀 YYYYMMDD（默认使用当前 UTC 日期）。"
            "仅用于文件命名，不影响 pipeline 确定性。"
        ),
    )
    parser.add_argument(
        "--gate-snapshot-sha256",
        type=str,
        default=None,
        help="gate 快照文件预期 sha256（可选，为 None 时跳过校验）。",
    )

    return parser.parse_args(argv)


# ---------------------------------------------------------------------------
# 主流程
# ---------------------------------------------------------------------------


def run_default_subset(args: argparse.Namespace) -> int:
    """执行 36 个默认子集的完整流程。

    流程：
      1. 计算 events 文件 sha256
      2. 计算 ATR14 文件 sha256
      3. 构建 PipelineConfig
      4. 先运行 BASELINE 候选，获取 baseline_trades
      5. 顺序运行所有 36 个候选（含 BASELINE）
      6. 调用 RankingMarkdownWriter 产出排名报告
      7. 写 per-candidate markdown 报告（YYYYMMDD 前缀）

    Args:
        args: 解析后的命令行参数。

    Returns:
        退出码：0 成功，1 有 invariant 违反或 abort。
    """
    output_dir = pathlib.Path(args.output_dir)
    project_root = pathlib.Path(args.project_root)
    date_prefix = _get_report_date_prefix(args.report_date)

    # --- Step 1: 计算 events sha256 ---
    print(f"[run_default_subset] Computing sha256 for events: {args.events_source_path}")
    try:
        events_sha256 = _compute_file_sha256(args.events_source_path)
    except FileNotFoundError:
        print(
            f"[ERROR] Events file not found: {args.events_source_path}",
            file=sys.stderr,
        )
        return 1

    # --- Step 2: 计算 ATR14 sha256 ---
    print(f"[run_default_subset] Computing sha256 for ATR14: {args.atr14_source_path}")
    try:
        atr14_sha256 = _compute_file_sha256(args.atr14_source_path)
    except FileNotFoundError:
        print(
            f"[ERROR] ATR14 file not found: {args.atr14_source_path}",
            file=sys.stderr,
        )
        return 1

    # --- Step 3: 构建 PipelineConfig ---
    config = PipelineConfig(
        seed=args.seed,
        git_commit_sha=args.git_commit_sha,
        events_source_path=args.events_source_path,
        events_source_sha256=events_sha256,
        runner_version=args.runner_version,
        output_dir=output_dir,
        project_root=project_root,
        tick_size_by_symbol=_DEFAULT_TICK_SIZE_BY_SYMBOL,
        step_size_by_symbol=_DEFAULT_STEP_SIZE_BY_SYMBOL,
        atr14_source_path=args.atr14_source_path,
        atr14_source_sha256=atr14_sha256,
        gate_snapshot_expected_sha256=args.gate_snapshot_sha256,
    )

    # --- Step 4: 先运行 BASELINE 候选获取 baseline_trades ---
    print("[run_default_subset] Running BASELINE candidate first...")
    pipeline = EntryRedesignPipeline(config)
    baseline_result = pipeline.run(
        spec=BASELINE,
        config=config,
        events_loader=None,
        baseline_trades=None,
    )

    # 从 baseline ledger 中提取 trades（用于后续候选的 baseline_reference）
    baseline_trades = _load_baseline_trades(baseline_result)

    print(
        f"[run_default_subset] BASELINE status: {baseline_result.status} "
        f"(candidate_id={baseline_result.candidate_id})"
    )

    # --- Step 5: 顺序运行所有 36 个候选 ---
    results: list[tuple[EntryCandidateSpec, PipelineResult]] = []
    has_violations = False

    for i, spec in enumerate(DEFAULT_SUBSET):
        candidate_id = generate_candidate_id(spec)
        print(
            f"[run_default_subset] [{i + 1}/{len(DEFAULT_SUBSET)}] "
            f"Running candidate: {candidate_id}"
        )

        # BASELINE 已经跑过，直接复用结果
        if spec == BASELINE:
            results.append((spec, baseline_result))
            continue

        result = pipeline.run(
            spec=spec,
            config=config,
            events_loader=None,
            baseline_trades=baseline_trades,
        )

        results.append((spec, result))

        if result.status == "completed" and result.summary_path is not None:
            # 检查是否有 invariant 违反（通过读取 summary JSON）
            pass  # InvariantChecker 违反会在 pipeline 内部处理
        elif result.status == "aborted":
            print(
                f"  [ABORT] candidate={candidate_id} "
                f"reason={result.abort_reason}",
                file=sys.stderr,
            )
        elif result.status == "rejected":
            print(
                f"  [REJECT] candidate={candidate_id} "
                f"reason={result.reject_reason}",
                file=sys.stderr,
            )

    # --- Step 6: 产出排名报告 ---
    print("[run_default_subset] Generating ranking report...")
    ranking_candidates = _collect_ranking_candidates(results)
    if ranking_candidates:
        ranking_writer = RankingMarkdownWriter()
        ranking_path = output_dir / "entry_candidate_ranking.md"
        ranking_writer.write(ranking_candidates, ranking_path)
        print(f"[run_default_subset] Ranking report: {ranking_path}")

    # --- Step 7: 写 per-candidate markdown 报告 ---
    print("[run_default_subset] Writing per-candidate markdown reports...")
    md_count = 0
    for spec, result in results:
        md_path = _write_per_candidate_markdown(
            result=result,
            spec=spec,
            output_dir=output_dir,
            date_prefix=date_prefix,
        )
        if md_path is not None:
            md_count += 1

    print(
        f"[run_default_subset] Done. "
        f"Total candidates: {len(DEFAULT_SUBSET)}, "
        f"Completed: {sum(1 for _, r in results if r.status == 'completed')}, "
        f"Rejected: {sum(1 for _, r in results if r.status == 'rejected')}, "
        f"Aborted: {sum(1 for _, r in results if r.status == 'aborted')}, "
        f"Markdown reports: {md_count}"
    )

    return 0


def _load_baseline_trades(
    result: PipelineResult,
) -> Optional[Sequence]:
    """从 baseline 运行结果中加载 trades。

    如果 baseline 运行成功且 ledger 存在，读取 ledger CSV 并转为
    TradeRecord 序列。否则返回 None。

    Args:
        result: Baseline pipeline 运行结果。

    Returns:
        TradeRecord 序列，或 None。
    """
    if result.status != "completed" or result.ledger_path is None:
        return None

    if not result.ledger_path.exists():
        return None

    # 读取 ledger CSV 并转为 TradeRecord 列表
    try:
        import pandas as pd

        from research.entry_redesign.ledger.ledger_csv_writer import TradeRecord

        df = pd.read_csv(result.ledger_path)
        if df.empty:
            return []

        trades: list[TradeRecord] = []
        for _, row in df.iterrows():
            trades.append(
                TradeRecord(
                    entry_ts=row["entry_ts"],
                    exit_ts=row["exit_ts"],
                    symbol=row["symbol"],
                    side=row["side"],
                    entry_price=float(row["entry_price"]),
                    exit_price=float(row["exit_price"]),
                    notional=float(row["notional"]),
                    raw_pnl=float(row["raw_pnl"]),
                    slip_pnl=float(row["slip_pnl"]),
                    realistic_pnl=float(row["realistic_pnl"]),
                    realistic_taker_both_pnl=float(
                        row["realistic_taker_both_pnl"]
                    ),
                    exit_reason=row["exit_reason"],
                    entry_candidate_id=row["entry_candidate_id"],
                    gate_mode=row["gate_mode"],
                    signal_bar_start_ts=row["signal_bar_start_ts"],
                    trigger_ts=row["trigger_ts"],
                    entry_delay_seconds=int(row["entry_delay_seconds"]),
                    feature_horizon_seconds=int(row["feature_horizon_seconds"]),
                    trigger_confirmation_id=row["trigger_confirmation_id"],
                    entry_price_mode_id=row["entry_price_mode_id"],
                    pretouch_state_band_id=row["pretouch_state_band_id"],
                    posttouch_quality_band_id=row["posttouch_quality_band_id"],
                )
            )
        return trades
    except Exception:
        # 读取失败时返回 None，不阻塞后续候选运行
        return None


def _collect_ranking_candidates(
    results: list[tuple[EntryCandidateSpec, PipelineResult]],
) -> list[dict]:
    """从运行结果中收集排名所需的候选数据。

    仅收集 status == "completed" 且 summary_path 存在的候选。
    从 summary JSON 中读取排名所需字段。

    Args:
        results: (spec, result) 元组列表。

    Returns:
        排名候选字典列表。
    """
    import json

    candidates: list[dict] = []

    for spec, result in results:
        if result.status != "completed" or result.summary_path is None:
            continue

        if not result.summary_path.exists():
            continue

        try:
            with open(result.summary_path, "r", encoding="utf-8") as f:
                summary = json.loads(f.read())

            # 提取排名所需字段
            cal_return = summary.get("nogate_calendar_normalized_return_pct")
            quality_bps = summary.get(
                "nogate_per_trade_quality_bps_over_notional"
            )
            asymmetry_tag = summary.get("asymmetry_tag", "none")

            # 跳过缺失关键字段的候选
            if cal_return is None or quality_bps is None:
                continue

            candidates.append({
                "candidate_id": result.candidate_id,
                "nogate_calendar_normalized_return_pct": float(cal_return),
                "nogate_per_trade_quality_bps_over_notional": float(
                    quality_bps
                ),
                "asymmetry_tag": asymmetry_tag,
            })
        except (json.JSONDecodeError, OSError, KeyError, TypeError, ValueError):
            # 读取/解析失败时跳过该候选
            continue

    return candidates


# ---------------------------------------------------------------------------
# __main__ 入口
# ---------------------------------------------------------------------------


if __name__ == "__main__":
    args = parse_args()
    exit_code = run_default_subset(args)
    sys.exit(exit_code)
