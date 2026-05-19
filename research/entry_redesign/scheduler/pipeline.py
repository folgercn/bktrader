"""EntryRedesignPipeline — 端到端 pipeline 实现。

连接所有模块：
  EntryCandidateSpec.validate → RunnerParameterSnapshot → WalkForwardDriver →
  EntryTriggerDetector → TriggerConfirmationEvaluator → PretouchStateGate →
  PosttouchQualityGate → EntryPriceResolver → CostModelApplier →
  GateCompositionLayer(nogate + gate001) → LedgerCsvWriter → MetricsAggregator →
  InvariantChecker → SummaryJsonWriter + AttributionCsvWriter

Pretouch + Posttouch 联合候选额外产出 ablation ledger。
abort / reject 路径按 Requirement 4.7 / 4.10 抑制 artifact。

Requirements: 2.10, 4.1, 4.7, 4.10
"""

from __future__ import annotations

import pathlib
from dataclasses import dataclass
from datetime import date, datetime
from typing import Any, Literal, Optional, Sequence

import pandas as pd

from research.entry_redesign.attribution.attribution_csv_writer import (
    AttributionCsvWriter,
)
from research.entry_redesign.confirmation.trigger_confirmation_evaluator import (
    TriggerConfirmationEvaluator,
)
from research.entry_redesign.cost.cost_model_applier import (
    BASELINE_COST_PARAMS,
    CostModelApplier,
    RawTrade,
    STRESS_COST_PARAMS,
)
from research.entry_redesign.detector.entry_trigger_detector import (
    EntryTriggerDetector,
    OneSecondBars,
    TriggerDecision,
)
from research.entry_redesign.gate.candidate_001_snapshot_loader import (
    Candidate001SnapshotLoader,
    DEFAULT_SNAPSHOT_PATH,
    GATE_001_THRESHOLDS,
)
from research.entry_redesign.gate.gate_composition_layer import (
    GateCompositionLayer,
    GateDecision,
    ValidationMetrics,
)
from research.entry_redesign.invariants.invariant_checker import InvariantChecker
from research.entry_redesign.ledger.ledger_csv_writer import (
    LedgerCsvWriter,
    TradeRecord,
)
from research.entry_redesign.metrics.metrics_aggregator import MetricsAggregator
from research.entry_redesign.posttouch.posttouch_quality_gate import (
    PosttouchQualityGate,
)
from research.entry_redesign.pretouch.pretouch_state_gate import PretouchStateGate
from research.entry_redesign.price.entry_price_resolver import EntryPriceResolver
from research.entry_redesign.scheduler.baseline_recompute import BaselineRecompute
from research.entry_redesign.scheduler.decision_flags import (
    compute_asymmetry_tag,
    compute_event_expectation_positive,
    compute_small_sample_flag,
)
from research.entry_redesign.snapshot.runner_aborted_writer import (
    RunnerAbortedWriter,
)
from research.entry_redesign.snapshot.runner_parameter_snapshot import (
    RunnerParameterSnapshot,
    snapshot_to_json,
)
from research.entry_redesign.snapshot.runner_rejected_combinations_writer import (
    RunnerRejectedCombinationsWriter,
)
from research.entry_redesign.spec.candidate_id import generate_candidate_id
from research.entry_redesign.spec.entry_candidate_spec import (
    EntryCandidateSpec,
    InvalidCandidateError,
)
from research.entry_redesign.summary.summary_json_writer import SummaryJsonWriter
from research.entry_redesign.walkforward.walkforward_driver import (
    InsufficientWalkforwardHistoryError,
    WalkForwardDriver,
    WalkForwardSplit,
)


# ---------------------------------------------------------------------------
# PipelineConfig — pipeline 运行配置
# ---------------------------------------------------------------------------


@dataclass(frozen=True)
class PipelineConfig:
    """Pipeline 运行配置。

    Attributes:
        seed: 随机种子（确定性约束 P7）。
        git_commit_sha: 当前 git commit sha。
        events_source_path: events CSV 文件路径。
        events_source_sha256: events CSV 文件 sha256。
        runner_version: runner 版本字符串
            （形如 "entry_redesign_runner_v1.0.0"）。
        output_dir: 产物输出根目录（通常为 research/）。
        project_root: 项目根目录（用于定位 gate 快照文件等）。
        tick_size_by_symbol: 每个 symbol 的 tick_size。
        step_size_by_symbol: 每个 symbol 的 step_size。
        atr14_source_path: 1h ATR(14) CSV 路径。
        atr14_source_sha256: 1h ATR(14) CSV sha256。
        gate_snapshot_expected_sha256: gate 快照文件预期 sha256（可选）。
            为 None 时跳过 sha256 校验。
        validation_metrics: gate001 validation 窗口指标。
            用于 GateCompositionLayer 的 gate001 判定。
        aborted_at_utc_ms: abort 时间戳（由调用方传入，禁止内部生成）。
            格式 ISO-8601 UTC ms（如 "2025-06-01T00:00:00.000Z"）。
    """

    seed: int
    git_commit_sha: str
    events_source_path: str
    events_source_sha256: str
    runner_version: str
    output_dir: pathlib.Path
    project_root: pathlib.Path
    tick_size_by_symbol: dict[str, float]
    step_size_by_symbol: dict[str, float]
    atr14_source_path: str
    atr14_source_sha256: str
    gate_snapshot_expected_sha256: Optional[str] = None
    validation_metrics: Optional[ValidationMetrics] = None
    aborted_at_utc_ms: str = "1970-01-01T00:00:00.000Z"


# ---------------------------------------------------------------------------
# PipelineResult — pipeline 运行结果
# ---------------------------------------------------------------------------


@dataclass
class PipelineResult:
    """Pipeline 单次运行结果。

    Attributes:
        candidate_id: Entry_Candidate 唯一标识。
        status: 运行状态。
            "completed" — 正常完成，产出全部 artifact。
            "rejected" — 静态拒绝（H > D 等），无 artifact。
            "aborted" — 运行中 abort，无 artifact。
        ledger_path: ledger CSV 路径（completed 时非 None）。
        summary_path: summary JSON 路径（completed 时非 None）。
        attribution_path: attribution CSV 路径（completed 时非 None）。
        snapshot_path: runner snapshot JSON 路径（completed 时非 None）。
        ablation_pretouch_only_path: ablation ledger 路径（联合候选时非 None）。
        ablation_posttouch_only_path: ablation ledger 路径（联合候选时非 None）。
        abort_reason: abort 原因（aborted 时非 None）。
        reject_reason: reject 原因（rejected 时非 None）。
    """

    candidate_id: str
    status: Literal["completed", "rejected", "aborted"]
    ledger_path: Optional[pathlib.Path] = None
    summary_path: Optional[pathlib.Path] = None
    attribution_path: Optional[pathlib.Path] = None
    snapshot_path: Optional[pathlib.Path] = None
    ablation_pretouch_only_path: Optional[pathlib.Path] = None
    ablation_posttouch_only_path: Optional[pathlib.Path] = None
    abort_reason: Optional[str] = None
    reject_reason: Optional[str] = None


# ---------------------------------------------------------------------------
# EntryRedesignPipeline
# ---------------------------------------------------------------------------


class EntryRedesignPipeline:
    """端到端 pipeline：连接所有模块，编排完整的 Entry_Candidate 评估流程。

    流程：
      1. EntryCandidateSpec.validate() — 静态校验（H <= D）
         → 失败写 runner_rejected_combinations.json，抑制所有 artifact
      2. 构建 RunnerParameterSnapshot
      3. WalkForwardDriver.check_data_availability() — 数据可用性检查
         → 失败写 runner_aborted.json，抑制所有 artifact
      4. 正常执行路径：
         FOR EACH event in execute window:
           EntryTriggerDetector.detect()
           → TriggerConfirmationEvaluator.evaluate()
           → PretouchStateGate.allow()
           → PosttouchQualityGate.allow()
           → EntryPriceResolver.resolve()
           → CostModelApplier.apply() (baseline + stress)
           → GateCompositionLayer.evaluate() (nogate + gate001)
      5. LedgerCsvWriter.write()
      6. MetricsAggregator.aggregate()
      7. InvariantChecker.check()
         → 违反写 invariant_violations.md + counterexample.json，CI 非零退出
      8. SummaryJsonWriter.write() + AttributionCsvWriter.write()

    Pretouch + Posttouch 联合候选额外产出 ablation ledger：
      - ablation_pretouch_only: 仅 pretouch gate，posttouch=none
      - ablation_posttouch_only: 仅 posttouch gate，pretouch=none

    Requirements: 2.10, 4.1, 4.7, 4.10
    """

    def __init__(self, config: PipelineConfig) -> None:
        """初始化 pipeline。

        Args:
            config: Pipeline 运行配置。
        """
        self._config = config
        self._output_dir = config.output_dir
        self._aborted_writer = RunnerAbortedWriter(config.output_dir)
        self._rejected_writer = RunnerRejectedCombinationsWriter(config.output_dir)
        self._ledger_writer = LedgerCsvWriter()
        self._summary_writer = SummaryJsonWriter()
        self._attribution_writer = AttributionCsvWriter()
        self._metrics_aggregator = MetricsAggregator()
        self._invariant_checker = InvariantChecker()
        self._cost_applier = CostModelApplier()

    # ------------------------------------------------------------------
    # 公开 API
    # ------------------------------------------------------------------

    def run(
        self,
        spec: EntryCandidateSpec,
        config: PipelineConfig,
        events_loader: Optional["EventsLoader"] = None,
        baseline_trades: Optional[Sequence[TradeRecord]] = None,
    ) -> PipelineResult:
        """编排完整的 Entry_Candidate 评估流程。

        Args:
            spec: Entry_Candidate 六元组定义。
            config: Pipeline 运行配置（可覆盖 __init__ 时的 config）。
            events_loader: 事件加载器（提供 events 与 1s bar 数据）。
                为 None 时使用默认加载器。
            baseline_trades: Baseline_Entry_Candidate 运行产出的 trades。
                用于计算 baseline_reference 节点。
                为 None 时跳过 baseline_reference 计算。

        Returns:
            PipelineResult: 运行结果（completed / rejected / aborted）。
        """
        # --- Step 1: 静态校验 ---
        candidate_id = self._try_validate(spec)
        if candidate_id is None:
            # validate() 失败 → rejected
            return self._handle_reject(spec, config)

        # --- Step 2: 构建 RunnerParameterSnapshot ---
        snapshot = self._build_snapshot(spec, config)

        # --- Step 3: Gate 快照加载与 sha256 校验 ---
        gate_snapshot_ref = self._load_gate_snapshot(config)
        if gate_snapshot_ref is None:
            # sha256 不一致 → abort
            return self._handle_abort(
                candidate_id=candidate_id,
                abort_reason="parameter_mismatch",
                mismatched_fields=[
                    {
                        "field_name": "gate001_snapshot_sha256",
                        "expected": config.gate_snapshot_expected_sha256 or "",
                        "observed": "file_not_found_or_mismatch",
                    }
                ],
                config=config,
            )

        # --- Step 4: Walk-forward 数据可用性检查 ---
        wf_driver = WalkForwardDriver()
        try:
            for split in wf_driver.iter_splits():
                wf_driver.check_data_availability(
                    split, config.events_source_path
                )
        except InsufficientWalkforwardHistoryError:
            return self._handle_abort(
                candidate_id=candidate_id,
                abort_reason="insufficient_walkforward_history",
                mismatched_fields=[],
                config=config,
            )

        # --- Step 5: 正常执行路径 ---
        trades = self._execute_walkforward(
            spec=spec,
            snapshot=snapshot,
            config=config,
            wf_driver=wf_driver,
            events_loader=events_loader,
        )

        # --- Step 6: 写 ledger ---
        ledger_path = self._get_ledger_path(candidate_id)
        self._ledger_writer.write(trades, ledger_path)

        # --- Step 7: 聚合 metrics ---
        ledger_df = self._trades_to_dataframe(trades)
        metrics = self._metrics_aggregator.aggregate(ledger_df, total_silos=22)

        # --- Step 8: 构建 summary ---
        summary = self._build_summary(
            metrics=metrics,
            snapshot=snapshot,
            config=config,
            wf_driver=wf_driver,
            gate_snapshot_ref=gate_snapshot_ref,
            ledger_df=ledger_df,
            baseline_trades=baseline_trades,
        )

        # --- Step 9: InvariantChecker ---
        violations = self._invariant_checker.check(ledger_df, summary)
        summary["invariant_violations"] = violations

        if InvariantChecker.has_violations(violations):
            # 违反 → 仍然写出 summary 和 ledger（供调试），但 CI 非零退出
            # 按 design.md: "任一违反立即使 CI 非零退出"
            pass

        # --- Step 10: 写 summary JSON ---
        summary_path = self._get_summary_path(candidate_id)
        self._summary_writer.write(summary, summary_path)

        # --- Step 11: 写 attribution CSV ---
        attribution_path = self._get_attribution_path(candidate_id)
        attribution_rows = self._build_attribution_rows(
            ledger_df=ledger_df,
            metrics=metrics,
            baseline_trades=baseline_trades,
            candidate_id=candidate_id,
            spec=spec,
        )
        self._attribution_writer.write(attribution_rows, attribution_path)

        # --- Step 12: 写 runner snapshot JSON ---
        snapshot_path = self._get_snapshot_path(candidate_id)
        self._write_snapshot(snapshot, snapshot_path)

        # --- Step 13: Ablation ledger（联合 pretouch + posttouch 候选）---
        ablation_pre_path: Optional[pathlib.Path] = None
        ablation_post_path: Optional[pathlib.Path] = None

        if self._is_combined_pretouch_posttouch(spec):
            ablation_pre_path, ablation_post_path = (
                self._produce_ablation_ledgers(
                    spec=spec,
                    snapshot=snapshot,
                    config=config,
                    wf_driver=wf_driver,
                    events_loader=events_loader,
                    candidate_id=candidate_id,
                )
            )

        return PipelineResult(
            candidate_id=candidate_id,
            status="completed",
            ledger_path=ledger_path,
            summary_path=summary_path,
            attribution_path=attribution_path,
            snapshot_path=snapshot_path,
            ablation_pretouch_only_path=ablation_pre_path,
            ablation_posttouch_only_path=ablation_post_path,
        )


    # ------------------------------------------------------------------
    # 内部方法：validate / reject / abort
    # ------------------------------------------------------------------

    def _try_validate(self, spec: EntryCandidateSpec) -> Optional[str]:
        """尝试校验 spec 并生成 candidate_id。

        Returns:
            candidate_id 字符串（校验通过），或 None（校验失败）。
        """
        try:
            spec.validate()
            return generate_candidate_id(spec)
        except InvalidCandidateError:
            return None

    def _handle_reject(
        self,
        spec: EntryCandidateSpec,
        config: PipelineConfig,
    ) -> PipelineResult:
        """处理静态拒绝路径（H > D 等）。

        写 runner_rejected_combinations.json，抑制所有 artifact。
        """
        # 尝试生成 candidate_id（即使 validate 失败也需要 id 用于文件命名）
        try:
            candidate_id = generate_candidate_id(spec)
        except (KeyError, ValueError):
            candidate_id = "unknown_rejected"

        reject_reason: Literal["H_gt_D", "token_mapping_error"] = "H_gt_D"
        try:
            spec.validate()
        except InvalidCandidateError as e:
            if e.reject_reason == "H_gt_D":
                reject_reason = "H_gt_D"
            else:
                reject_reason = "token_mapping_error"

        self._rejected_writer.write(
            candidate_id=candidate_id,
            reject_reason=reject_reason,
            spec=spec,
            rejected_at_utc_ms=config.aborted_at_utc_ms,
        )

        return PipelineResult(
            candidate_id=candidate_id,
            status="rejected",
            reject_reason=reject_reason,
        )

    def _handle_abort(
        self,
        candidate_id: str,
        abort_reason: str,
        mismatched_fields: list[dict[str, str]],
        config: PipelineConfig,
    ) -> PipelineResult:
        """处理运行中 abort 路径。

        写 runner_aborted.json，抑制所有 artifact。
        """
        self._aborted_writer.write(
            candidate_id=candidate_id,
            abort_reason=abort_reason,  # type: ignore[arg-type]
            mismatched_fields=mismatched_fields,
            aborted_at_utc_ms=config.aborted_at_utc_ms,
        )

        return PipelineResult(
            candidate_id=candidate_id,
            status="aborted",
            abort_reason=abort_reason,
        )

    # ------------------------------------------------------------------
    # 内部方法：构建 snapshot
    # ------------------------------------------------------------------

    def _build_snapshot(
        self,
        spec: EntryCandidateSpec,
        config: PipelineConfig,
    ) -> RunnerParameterSnapshot:
        """构建 RunnerParameterSnapshot。"""
        from research.entry_redesign.snapshot.runner_parameter_snapshot import (
            Atr14Source,
            CostModelParams,
            FeatureSources,
            SymbolFilters,
        )

        return RunnerParameterSnapshot(
            candidate=spec,
            cost_model_baseline=CostModelParams(
                slip_bps_per_side=BASELINE_COST_PARAMS.slip_bps_per_side,
                entry_bps=BASELINE_COST_PARAMS.entry_bps,
                exit_bps=BASELINE_COST_PARAMS.exit_bps,
            ),
            cost_model_stress=CostModelParams(
                slip_bps_per_side=STRESS_COST_PARAMS.slip_bps_per_side,
                entry_bps=STRESS_COST_PARAMS.entry_bps,
                exit_bps=STRESS_COST_PARAMS.exit_bps,
            ),
            seed=config.seed,
            git_commit_sha=config.git_commit_sha,
            events_source_path=config.events_source_path,
            events_source_sha256=config.events_source_sha256,
            runner_version=config.runner_version,
            symbol_filters=SymbolFilters(
                tick_size_by_symbol=config.tick_size_by_symbol,
                step_size_by_symbol=config.step_size_by_symbol,
            ),
            features=FeatureSources(
                atr14_source=Atr14Source(
                    path=config.atr14_source_path,
                    sha256=config.atr14_source_sha256,
                )
            ),
        )

    # ------------------------------------------------------------------
    # 内部方法：gate 快照加载
    # ------------------------------------------------------------------

    def _load_gate_snapshot(
        self,
        config: PipelineConfig,
    ) -> Optional[dict[str, str]]:
        """加载 gate001 快照并校验 sha256。

        Returns:
            gate001_snapshot_ref dict（成功），或 None（失败/不一致）。
        """
        snapshot_path = config.project_root / DEFAULT_SNAPSHOT_PATH
        loader = Candidate001SnapshotLoader(snapshot_path)

        try:
            loader.load()
        except (FileNotFoundError, OSError):
            if config.gate_snapshot_expected_sha256 is not None:
                # 预期有快照但文件不存在 → abort
                return None
            # 无预期 sha256 且文件不存在 → 跳过校验，使用默认阈值
            return {"path": str(snapshot_path), "sha256": "unavailable"}

        # sha256 校验
        if config.gate_snapshot_expected_sha256 is not None:
            if not loader.verify_sha256(config.gate_snapshot_expected_sha256):
                return None

        return loader.get_snapshot_ref()


    # ------------------------------------------------------------------
    # 内部方法：walk-forward 执行
    # ------------------------------------------------------------------

    def _execute_walkforward(
        self,
        spec: EntryCandidateSpec,
        snapshot: RunnerParameterSnapshot,
        config: PipelineConfig,
        wf_driver: WalkForwardDriver,
        events_loader: Optional["EventsLoader"],
    ) -> list[TradeRecord]:
        """执行 walk-forward 循环，产出 trades 列表。

        FOR EACH execute 月:
          FOR EACH event in execute window:
            detect → confirm → pretouch → posttouch → resolve → cost → gate

        Args:
            spec: Entry_Candidate 六元组。
            snapshot: Runner 参数快照。
            config: Pipeline 配置。
            wf_driver: Walk-forward driver。
            events_loader: 事件加载器。

        Returns:
            所有 gate_mode 下的 TradeRecord 列表。
        """
        trades: list[TradeRecord] = []
        candidate_id = generate_candidate_id(spec)

        # 初始化 Entry Layer 模块
        detector = EntryTriggerDetector(
            tick_size_by_symbol=config.tick_size_by_symbol
        )
        confirmation_evaluator = TriggerConfirmationEvaluator(snapshot)
        pretouch_gate = PretouchStateGate(spec.pretouch_state_band_id)
        posttouch_gate = PosttouchQualityGate(
            band_id=spec.posttouch_quality_band_id,
            tick_size=self._get_primary_tick_size(config),
        )
        price_resolver = EntryPriceResolver(snapshot)

        # GateCompositionLayer
        validation_metrics = config.validation_metrics or ValidationMetrics(
            validation_return_over_dd=0.0,
            validation_topk_sizing_markov_score_mean=0.0,
            validation_topk_sized_return_pct=0.0,
        )
        gate_layer = GateCompositionLayer(
            gate_thresholds=GATE_001_THRESHOLDS,
            validation_metrics=validation_metrics,
        )

        # 遍历 walk-forward splits
        for split in wf_driver.iter_splits():
            # 加载 execute 窗口内的 events
            if events_loader is None:
                continue

            events = events_loader.load_events(
                split.execute_start, split.execute_end
            )

            for event in events:
                # 设置 price_resolver 上下文
                price_resolver.set_context(
                    atr14=event.atr14_prev_closed_1h,
                    symbol=event.symbol,
                )

                # 获取 1s bar 窗口
                onesec_window = events_loader.get_onesec_window(event)
                trades_window = events_loader.get_trades_window(event)

                # --- EntryTriggerDetector ---
                trigger = detector.detect(event, onesec_window)
                if trigger is None:
                    continue

                # --- TriggerConfirmationEvaluator ---
                hourly_history = events_loader.get_hourly_history(event)
                confirmation = confirmation_evaluator.evaluate(
                    trigger, onesec_window, trades_window, hourly_history
                )
                if not confirmation.confirmed:
                    continue

                # --- PretouchStateGate ---
                if not pretouch_gate.allow(event, event.atr14_prev_closed_1h):
                    continue

                # --- PosttouchQualityGate ---
                posttouch_onesec = events_loader.get_posttouch_onesec_window(
                    event, trigger
                )
                posttouch_trades = events_loader.get_posttouch_trades_window(
                    event, trigger
                )
                if not posttouch_gate.allow(
                    trigger, posttouch_onesec, posttouch_trades,
                    event.atr14_prev_closed_1h,
                ):
                    continue

                # --- EntryPriceResolver ---
                resolution = price_resolver.resolve(trigger, onesec_window)
                if not resolution.filled:
                    # 未成交 → 不进 ledger（attribution 计数由上游处理）
                    continue

                # --- CostModelApplier + GateCompositionLayer ---
                # 构建 RawTrade（exit 逻辑由上游 events_loader 提供）
                exit_info = events_loader.get_exit_info(event, resolution)
                raw_trade = RawTrade(
                    raw_pnl=exit_info.raw_pnl,
                    notional=exit_info.notional,
                    entry_price=resolution.entry_price,
                    exit_price=exit_info.exit_price,
                    symbol=event.symbol,
                    side=event.side,
                )

                # 应用 baseline 成本模型
                priced_trade = self._cost_applier.apply(
                    raw_trade, BASELINE_COST_PARAMS
                )

                # 并行评估 nogate + gate001
                for gate_mode in ("nogate", "gate001"):
                    gate_decision = gate_layer.evaluate(
                        priced_trade, gate_mode  # type: ignore[arg-type]
                    )

                    # gate_rejected 的 trade 仍然写入 ledger
                    # （exit_reason 标记为 gate_rejected）
                    exit_reason = (
                        exit_info.exit_reason
                        if gate_decision.passed
                        else "gate_rejected"
                    )

                    trade_record = TradeRecord(
                        entry_ts=resolution.entry_ts,
                        exit_ts=exit_info.exit_ts,
                        symbol=event.symbol,
                        side=event.side,
                        entry_price=resolution.entry_price,
                        exit_price=exit_info.exit_price,
                        notional=exit_info.notional,
                        raw_pnl=exit_info.raw_pnl,
                        slip_pnl=priced_trade.slip_pnl,
                        realistic_pnl=priced_trade.realistic_pnl,
                        realistic_taker_both_pnl=(
                            priced_trade.realistic_taker_both_pnl
                        ),
                        exit_reason=exit_reason,
                        entry_candidate_id=candidate_id,
                        gate_mode=gate_mode,
                        signal_bar_start_ts=event.signal_bar_start_ts,
                        trigger_ts=trigger.trigger_ts,
                        entry_delay_seconds=spec.entry_delay_seconds,
                        feature_horizon_seconds=spec.feature_horizon_seconds,
                        trigger_confirmation_id=spec.trigger_confirmation_id,
                        entry_price_mode_id=spec.entry_price_mode_id,
                        pretouch_state_band_id=spec.pretouch_state_band_id,
                        posttouch_quality_band_id=(
                            spec.posttouch_quality_band_id
                        ),
                    )
                    trades.append(trade_record)

        return trades


    # ------------------------------------------------------------------
    # 内部方法：summary 构建
    # ------------------------------------------------------------------

    def _build_summary(
        self,
        metrics: dict[str, Any],
        snapshot: RunnerParameterSnapshot,
        config: PipelineConfig,
        wf_driver: WalkForwardDriver,
        gate_snapshot_ref: dict[str, str],
        ledger_df: pd.DataFrame,
        baseline_trades: Optional[Sequence[TradeRecord]],
    ) -> dict[str, Any]:
        """构建完整的 summary dict。

        包含 metrics、判定字段、walkforward_config、gate001_snapshot_ref 等。
        """
        summary: dict[str, Any] = {}

        # 合并 metrics（nogate_* / gate001_* 前缀）
        summary.update(metrics)

        # walkforward_config
        summary["walkforward_config"] = wf_driver.get_walkforward_config()

        # gate001_snapshot_ref
        summary["gate001_snapshot_ref"] = gate_snapshot_ref

        # baseline_reference
        if baseline_trades is not None:
            recomputer = BaselineRecompute()
            baseline_ref = recomputer.compute_baseline_reference(baseline_trades)
        else:
            baseline_ref = {
                "nogate_win_rate": None,
                "nogate_payoff_ratio": None,
                "BTCUSDT": {"nogate_win_rate": None, "nogate_payoff_ratio": None},
                "ETHUSDT": {"nogate_win_rate": None, "nogate_payoff_ratio": None},
            }
        summary["baseline_reference"] = baseline_ref

        # event_expectation_positive
        summary["event_expectation_positive"] = (
            compute_event_expectation_positive(summary, baseline_ref)
        )

        # 单 symbol 版本
        btc_summary = self._compute_symbol_metrics(ledger_df, "BTCUSDT")
        eth_summary = self._compute_symbol_metrics(ledger_df, "ETHUSDT")

        summary["event_expectation_positive_btc_only"] = (
            compute_event_expectation_positive(
                btc_summary, baseline_ref.get("BTCUSDT", {})
            )
        )
        summary["event_expectation_positive_eth_only"] = (
            compute_event_expectation_positive(
                eth_summary, baseline_ref.get("ETHUSDT", {})
            )
        )

        # small_sample_flag
        # Inject btc/eth trade counts into summary for compute_small_sample_flag
        summary["btc_nogate_trade_count"] = btc_summary.get(
            "nogate_trade_count", 0
        )
        summary["eth_nogate_trade_count"] = eth_summary.get(
            "nogate_trade_count", 0
        )
        summary["small_sample_flag"] = compute_small_sample_flag(summary)

        # asymmetry_tag
        summary["asymmetry_tag"] = compute_asymmetry_tag(summary)

        # live_output_emitted 恒 false（research-only）
        summary["live_output_emitted"] = False

        return summary

    def _compute_symbol_metrics(
        self,
        ledger_df: pd.DataFrame,
        symbol: str,
    ) -> dict[str, Any]:
        """计算单 symbol 子集的 nogate 指标。"""
        if ledger_df.empty:
            return {
                "nogate_trade_count": 0,
                "nogate_win_rate": None,
                "nogate_payoff_ratio": None,
                "nogate_calendar_normalized_return_pct": 0.0,
                "nogate_per_trade_quality_bps_over_notional": None,
            }

        symbol_df = ledger_df[ledger_df["symbol"] == symbol]
        if symbol_df.empty:
            return {
                "nogate_trade_count": 0,
                "nogate_win_rate": None,
                "nogate_payoff_ratio": None,
                "nogate_calendar_normalized_return_pct": 0.0,
                "nogate_per_trade_quality_bps_over_notional": None,
            }

        # 使用 MetricsAggregator 对 symbol 子集计算
        # 单 symbol 有 11 个 execute months
        result = self._metrics_aggregator.aggregate(symbol_df, total_silos=11)
        return result

    # ------------------------------------------------------------------
    # 内部方法：attribution 构建
    # ------------------------------------------------------------------

    def _build_attribution_rows(
        self,
        ledger_df: pd.DataFrame,
        metrics: dict[str, Any],
        baseline_trades: Optional[Sequence[TradeRecord]],
        candidate_id: str,
        spec: EntryCandidateSpec,
    ) -> list[dict]:
        """构建 attribution CSV 行。

        按 (symbol, year_month) 分组计算 per-silo 指标。
        """
        import numpy as np

        # 定义 22 个 calendar silo
        symbols = ["BTCUSDT", "ETHUSDT"]
        months = [
            f"{y:04d}-{m:02d}"
            for y, m in [
                (2025, 6), (2025, 7), (2025, 8), (2025, 9),
                (2025, 10), (2025, 11), (2025, 12),
                (2026, 1), (2026, 2), (2026, 3), (2026, 4),
            ]
        ]

        # 计算 baseline reference 指标（用于 delta 计算）
        baseline_nogate_pnl_pct: dict[tuple[str, str], float] = {}
        baseline_nogate_quality_bps: dict[tuple[str, str], float] = {}
        baseline_nogate_quality_bps_overall: float = 0.0

        if baseline_trades is not None:
            baseline_df = self._trades_to_dataframe(list(baseline_trades))
            if not baseline_df.empty:
                baseline_nogate = baseline_df[
                    baseline_df["gate_mode"] == "nogate"
                ]
                if not baseline_nogate.empty:
                    r_i = (
                        baseline_nogate["realistic_pnl"].values
                        / baseline_nogate["notional"].values
                    )
                    baseline_nogate_quality_bps_overall = (
                        float(np.mean(r_i)) * 10000.0
                    )

        rows: list[dict] = []

        for symbol in symbols:
            for year_month in months:
                # 筛选当前 silo
                silo_mask = self._get_silo_mask(ledger_df, symbol, year_month)

                # nogate 指标
                nogate_silo = ledger_df[silo_mask & (ledger_df["gate_mode"] == "nogate")]
                nogate_tc = len(nogate_silo)
                nogate_pnl_pct = 0.0
                nogate_quality_bps = 0.0

                if nogate_tc > 0:
                    r_i = (
                        nogate_silo["realistic_pnl"].values.astype(np.float64)
                        / nogate_silo["notional"].values.astype(np.float64)
                    )
                    nogate_pnl_pct = float(np.sum(r_i)) * 100.0
                    nogate_quality_bps = float(np.mean(r_i)) * 10000.0

                # gate001 指标
                gate001_silo = ledger_df[
                    silo_mask & (ledger_df["gate_mode"] == "gate001")
                ]
                gate001_tc = len(gate001_silo)
                gate001_pnl_pct = 0.0
                gate001_quality_bps = 0.0

                if gate001_tc > 0:
                    r_i_g = (
                        gate001_silo["realistic_pnl"].values.astype(np.float64)
                        / gate001_silo["notional"].values.astype(np.float64)
                    )
                    gate001_pnl_pct = float(np.sum(r_i_g)) * 100.0
                    gate001_quality_bps = float(np.mean(r_i_g)) * 10000.0

                # baseline delta
                bl_pnl = baseline_nogate_pnl_pct.get((symbol, year_month), 0.0)
                bl_quality = baseline_nogate_quality_bps.get(
                    (symbol, year_month), 0.0
                )

                # entry_effect_bps / gate_effect_bps / sizing_effect_bps
                entry_effect_bps = (
                    nogate_quality_bps - baseline_nogate_quality_bps_overall
                )
                gate_effect_bps = gate001_quality_bps - nogate_quality_bps
                sizing_effect_bps = 0.0  # 恒为 0.0

                # layer_dependency 判定
                layer_dependency = self._determine_layer_dependency(
                    nogate_pnl_pct=metrics.get(
                        "nogate_calendar_normalized_return_pct", 0.0
                    ),
                    gate001_pnl_pct=metrics.get(
                        "gate001_calendar_normalized_return_pct", 0.0
                    ),
                    spec=spec,
                )

                rows.append({
                    "symbol": symbol,
                    "year_month": year_month,
                    "nogate_trade_count": nogate_tc,
                    "nogate_realistic_pnl_pct": nogate_pnl_pct,
                    "nogate_per_trade_quality_bps_over_notional": nogate_quality_bps,
                    "gate001_trade_count": gate001_tc,
                    "gate001_realistic_pnl_pct": gate001_pnl_pct,
                    "gate001_per_trade_quality_bps_over_notional": gate001_quality_bps,
                    "baseline_delta_nogate_realistic_pnl_pct": (
                        nogate_pnl_pct - bl_pnl
                    ),
                    "baseline_delta_nogate_per_trade_quality_bps_over_notional": (
                        nogate_quality_bps - bl_quality
                    ),
                    "entry_effect_bps": entry_effect_bps,
                    "gate_effect_bps": gate_effect_bps,
                    "sizing_effect_bps": sizing_effect_bps,
                    "layer_dependency": layer_dependency,
                    "pullback_limit_unfilled_count": 0,
                })

        return rows

    @staticmethod
    def _get_silo_mask(
        ledger_df: pd.DataFrame,
        symbol: str,
        year_month: str,
    ) -> "pd.Series":
        """获取 (symbol, year_month) silo 的布尔掩码。"""
        if ledger_df.empty:
            return pd.Series(dtype=bool)

        symbol_mask = ledger_df["symbol"] == symbol

        # 从 signal_bar_start_ts 推导 year_month
        ts_col = pd.to_datetime(ledger_df["signal_bar_start_ts"])
        month_str = ts_col.dt.strftime("%Y-%m")
        month_mask = month_str == year_month

        return symbol_mask & month_mask

    @staticmethod
    def _determine_layer_dependency(
        nogate_pnl_pct: float,
        gate001_pnl_pct: float,
        spec: EntryCandidateSpec,
    ) -> str:
        """判定 layer_dependency 标签。

        - "entry_only": nogate 维度独立有效（nogate_calendar >= 0）
        - "gate_dependent": 仅在 gate001 下有效
        - "combined": pretouch + posttouch 联合使用
        """
        if (
            spec.pretouch_state_band_id != "none"
            and spec.posttouch_quality_band_id != "none"
        ):
            return "combined"
        if nogate_pnl_pct >= 0.0:
            return "entry_only"
        if gate001_pnl_pct >= 0.0 and nogate_pnl_pct < 0.0:
            return "gate_dependent"
        return "entry_only"


    # ------------------------------------------------------------------
    # 内部方法：ablation ledger（联合 pretouch + posttouch）
    # ------------------------------------------------------------------

    @staticmethod
    def _is_combined_pretouch_posttouch(spec: EntryCandidateSpec) -> bool:
        """判定是否为联合 pretouch + posttouch 候选。

        Requirement 2.10: 同时启用 Pretouch_State_Band != none 与
        Posttouch_Quality_Band != none 时，额外产出 ablation ledger。
        """
        return (
            spec.pretouch_state_band_id != "none"
            and spec.posttouch_quality_band_id != "none"
        )

    def _produce_ablation_ledgers(
        self,
        spec: EntryCandidateSpec,
        snapshot: RunnerParameterSnapshot,
        config: PipelineConfig,
        wf_driver: WalkForwardDriver,
        events_loader: Optional["EventsLoader"],
        candidate_id: str,
    ) -> tuple[Optional[pathlib.Path], Optional[pathlib.Path]]:
        """产出 ablation_pretouch_only 和 ablation_posttouch_only 两个 ledger。

        ablation_pretouch_only: 使用原 spec 的 pretouch band，posttouch=none
        ablation_posttouch_only: 使用原 spec 的 posttouch band，pretouch=none

        Returns:
            (ablation_pretouch_only_path, ablation_posttouch_only_path)
        """
        # --- ablation_pretouch_only: pretouch=原值, posttouch=none ---
        pretouch_only_spec = EntryCandidateSpec(
            entry_delay_seconds=spec.entry_delay_seconds,
            feature_horizon_seconds=spec.feature_horizon_seconds,
            trigger_confirmation_id=spec.trigger_confirmation_id,
            entry_price_mode_id=spec.entry_price_mode_id,
            pretouch_state_band_id=spec.pretouch_state_band_id,
            posttouch_quality_band_id="none",
        )
        pretouch_only_snapshot = self._build_snapshot(pretouch_only_spec, config)
        pretouch_only_trades = self._execute_walkforward(
            spec=pretouch_only_spec,
            snapshot=pretouch_only_snapshot,
            config=config,
            wf_driver=wf_driver,
            events_loader=events_loader,
        )
        pretouch_only_path = (
            self._output_dir
            / f"tmp_entry_redesign_{candidate_id}_ablation_pretouch_only_ledger.csv"
        )
        self._ledger_writer.write(pretouch_only_trades, pretouch_only_path)

        # --- ablation_posttouch_only: pretouch=none, posttouch=原值 ---
        posttouch_only_spec = EntryCandidateSpec(
            entry_delay_seconds=spec.entry_delay_seconds,
            feature_horizon_seconds=spec.feature_horizon_seconds,
            trigger_confirmation_id=spec.trigger_confirmation_id,
            entry_price_mode_id=spec.entry_price_mode_id,
            pretouch_state_band_id="none",
            posttouch_quality_band_id=spec.posttouch_quality_band_id,
        )
        posttouch_only_snapshot = self._build_snapshot(posttouch_only_spec, config)
        posttouch_only_trades = self._execute_walkforward(
            spec=posttouch_only_spec,
            snapshot=posttouch_only_snapshot,
            config=config,
            wf_driver=wf_driver,
            events_loader=events_loader,
        )
        posttouch_only_path = (
            self._output_dir
            / f"tmp_entry_redesign_{candidate_id}_ablation_posttouch_only_ledger.csv"
        )
        self._ledger_writer.write(posttouch_only_trades, posttouch_only_path)

        return pretouch_only_path, posttouch_only_path

    # ------------------------------------------------------------------
    # 内部方法：辅助
    # ------------------------------------------------------------------

    @staticmethod
    def _get_primary_tick_size(config: PipelineConfig) -> float:
        """获取主要 tick_size（用于 PosttouchQualityGate 初始化）。

        优先使用 ETHUSDT 的 tick_size（精度更高），
        fallback 到字典中第一个值。
        """
        if "ETHUSDT" in config.tick_size_by_symbol:
            return config.tick_size_by_symbol["ETHUSDT"]
        if config.tick_size_by_symbol:
            return next(iter(config.tick_size_by_symbol.values()))
        return 0.01  # 安全默认值

    @staticmethod
    def _trades_to_dataframe(trades: Sequence[TradeRecord]) -> pd.DataFrame:
        """将 TradeRecord 序列转为 pandas DataFrame。"""
        if not trades:
            return pd.DataFrame(columns=[
                "entry_ts", "exit_ts", "symbol", "side",
                "entry_price", "exit_price", "notional",
                "raw_pnl", "slip_pnl", "realistic_pnl",
                "realistic_taker_both_pnl", "exit_reason",
                "entry_candidate_id", "gate_mode",
                "signal_bar_start_ts", "trigger_ts",
                "entry_delay_seconds", "feature_horizon_seconds",
                "trigger_confirmation_id", "entry_price_mode_id",
                "pretouch_state_band_id", "posttouch_quality_band_id",
            ])

        records = []
        for t in trades:
            records.append({
                "entry_ts": t.entry_ts,
                "exit_ts": t.exit_ts,
                "symbol": t.symbol,
                "side": t.side,
                "entry_price": t.entry_price,
                "exit_price": t.exit_price,
                "notional": t.notional,
                "raw_pnl": t.raw_pnl,
                "slip_pnl": t.slip_pnl,
                "realistic_pnl": t.realistic_pnl,
                "realistic_taker_both_pnl": t.realistic_taker_both_pnl,
                "exit_reason": t.exit_reason,
                "entry_candidate_id": t.entry_candidate_id,
                "gate_mode": t.gate_mode,
                "signal_bar_start_ts": t.signal_bar_start_ts,
                "trigger_ts": t.trigger_ts,
                "entry_delay_seconds": t.entry_delay_seconds,
                "feature_horizon_seconds": t.feature_horizon_seconds,
                "trigger_confirmation_id": t.trigger_confirmation_id,
                "entry_price_mode_id": t.entry_price_mode_id,
                "pretouch_state_band_id": t.pretouch_state_band_id,
                "posttouch_quality_band_id": t.posttouch_quality_band_id,
            })

        return pd.DataFrame(records)

    @staticmethod
    def _write_snapshot(
        snapshot: RunnerParameterSnapshot,
        path: pathlib.Path,
    ) -> None:
        """写 runner snapshot JSON。"""
        path.parent.mkdir(parents=True, exist_ok=True)
        json_str = snapshot_to_json(snapshot)
        with open(path, "w", encoding="utf-8", newline="") as f:
            f.write(json_str)

    # ------------------------------------------------------------------
    # 路径生成辅助
    # ------------------------------------------------------------------

    def _get_ledger_path(self, candidate_id: str) -> pathlib.Path:
        """生成 ledger CSV 路径。

        模板: research/tmp_entry_redesign_<candidate_id>_ledger.csv
        """
        return (
            self._output_dir
            / f"tmp_entry_redesign_{candidate_id}_ledger.csv"
        )

    def _get_summary_path(self, candidate_id: str) -> pathlib.Path:
        """生成 summary JSON 路径。

        模板: research/tmp_entry_redesign_<candidate_id>_summary.json
        """
        return (
            self._output_dir
            / f"tmp_entry_redesign_{candidate_id}_summary.json"
        )

    def _get_attribution_path(self, candidate_id: str) -> pathlib.Path:
        """生成 attribution CSV 路径。

        模板: research/tmp_entry_redesign_<candidate_id>_attribution.csv
        """
        return (
            self._output_dir
            / f"tmp_entry_redesign_{candidate_id}_attribution.csv"
        )

    def _get_snapshot_path(self, candidate_id: str) -> pathlib.Path:
        """生成 runner snapshot JSON 路径。

        模板: research/tmp_entry_redesign_<candidate_id>_runner_snapshot.json
        """
        return (
            self._output_dir
            / f"tmp_entry_redesign_{candidate_id}_runner_snapshot.json"
        )


# ---------------------------------------------------------------------------
# EventsLoader Protocol — 事件加载器接口
# ---------------------------------------------------------------------------


class EventsLoader:
    """事件加载器协议。

    由 pipeline 调用方实现，提供 events 与 1s bar / trades 数据。
    本协议定义 pipeline 所需的最小接口集合。

    实现者需要提供：
      - load_events(): 加载指定 execute 窗口内的 events
      - get_onesec_window(): 获取 event 对应的 1s bar 窗口
      - get_trades_window(): 获取 event 对应的 taker trades 窗口
      - get_hourly_history(): 获取 event 前的 1h bar 历史
      - get_posttouch_onesec_window(): 获取 post-touch 1s bar 窗口
      - get_posttouch_trades_window(): 获取 post-touch trades 窗口
      - get_exit_info(): 获取 exit 信息（价格、时间、原因、PnL）
    """

    def load_events(
        self,
        execute_start: "date",
        execute_end: "date",
    ) -> Sequence[Any]:
        """加载指定 execute 窗口 [start, end) 内的 events。

        Args:
            execute_start: execute 窗口起始日期（含）。
            execute_end: execute 窗口结束日期（不含）。

        Returns:
            Event 序列（满足 EntryTriggerDetector.Event 协议）。
        """
        raise NotImplementedError

    def get_onesec_window(self, event: Any) -> Any:
        """获取 event 对应的 1s bar 窗口。

        返回满足 OneSecondBars 协议的对象。
        """
        raise NotImplementedError

    def get_trades_window(self, event: Any) -> Any:
        """获取 event 对应的 taker trades 窗口。

        返回满足 TakerTradesWindow 协议的对象。
        """
        raise NotImplementedError

    def get_hourly_history(self, event: Any) -> Any:
        """获取 event 前的 1h bar 历史。

        返回满足 HourlyBarsHistory 协议的对象，或 None。
        """
        raise NotImplementedError

    def get_posttouch_onesec_window(
        self, event: Any, trigger: TriggerDecision
    ) -> Any:
        """获取 [trigger_ts, trigger_ts + H] 窗口内的 1s bar。

        返回满足 PosttouchQualityGate 所需的 OneSecondBarsProtocol 对象。
        """
        raise NotImplementedError

    def get_posttouch_trades_window(
        self, event: Any, trigger: TriggerDecision
    ) -> Any:
        """获取 [trigger_ts, trigger_ts + H] 窗口内的 taker trades。

        返回满足 PosttouchQualityGate 所需的 TakerTradesWindow 对象。
        """
        raise NotImplementedError

    def get_exit_info(self, event: Any, resolution: Any) -> "ExitInfo":
        """获取 exit 信息。

        Args:
            event: 入口事件。
            resolution: EntryPriceResolver 的 PriceResolution 结果。

        Returns:
            ExitInfo 对象。
        """
        raise NotImplementedError


# ---------------------------------------------------------------------------
# ExitInfo — exit 信息数据类
# ---------------------------------------------------------------------------


@dataclass(frozen=True)
class ExitInfo:
    """Exit 信息，由 EventsLoader.get_exit_info() 返回。

    Attributes:
        exit_ts: 出场时间戳（UTC）。
        exit_price: 出场价格。
        exit_reason: 出场原因（封闭枚举）。
        raw_pnl: 原始 PnL（未扣除成本）。
        notional: 名义金额（entry notional）。
    """

    exit_ts: datetime
    exit_price: float
    exit_reason: Literal[
        "signal_exit",
        "initial_stop",
        "breakeven_stop",
        "trail_stop",
        "max_hold_timeout",
    ]
    raw_pnl: float
    notional: float
