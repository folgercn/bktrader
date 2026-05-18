"""Report Generator — Go/No-Go 报告与历史对比"""

from __future__ import annotations

import json
import logging
from dataclasses import asdict, dataclass
from datetime import datetime
from pathlib import Path

import numpy as np
import pandas as pd

from timing_probability_unified.combined_executor import (
    SensitivityRow,
    compute_calendar_sum,
    compute_worst_sm,
)
from timing_probability_unified.event_source_builder import EventPoolStats
from timing_probability_unified.probability_model import RFProbabilityResult
from timing_probability_unified.robustness import (
    AblationRow,
    BootstrapResult,
    ForwardResult,
    RobustnessResult,
)
from timing_probability_unified.speed_gate import SpeedGateResult
from timing_probability_unified.timing_classifier import TimingClassifierResult

logger = logging.getLogger(__name__)


@dataclass
class GoNoGoDecision:
    """Go/No-Go 判定"""

    decision: str  # "strong_go" / "marginal_go" / "no_go"
    calendar_sum: float
    worst_sm: float
    btc_positive: bool
    eth_positive: bool
    forward_calendar_sum: float
    overfitting_downgrade: bool  # overfitting_flag 导致降级


def compute_go_no_go_decision(
    calendar_sum: float,
    worst_sm: float,
    btc_calendar_sum: float,
    eth_calendar_sum: float,
    forward_calendar_sum: float,
    overfitting_flag: bool,
) -> GoNoGoDecision:
    """计算 Go/No-Go 判定。

    判定逻辑（按优先级）：
    1. IF overfitting_flag == True AND calendar_sum >= 10% → "marginal_go" (降级)
    2. IF calendar_sum >= 10% AND worst_sm > -0.5% AND BTC正 AND ETH正
       AND forward >= 7% → "strong_go"
    3. IF calendar_sum < 7% OR worst_sm < -1.0% → "no_go"
    4. 其余 → "marginal_go"

    Parameters
    ----------
    calendar_sum : float
        Full-window calendar_sum（百分比形式，如 0.10 表示 10%）。
    worst_sm : float
        Worst single month（百分比形式，如 -0.005 表示 -0.5%）。
    btc_calendar_sum : float
        BTC-only calendar_sum。
    eth_calendar_sum : float
        ETH-only calendar_sum。
    forward_calendar_sum : float
        Forward split calendar_sum。
    overfitting_flag : bool
        是否存在 overfitting 标记。

    Returns
    -------
    GoNoGoDecision
        Go/No-Go 判定结果。
    """
    btc_positive = btc_calendar_sum > 0
    eth_positive = eth_calendar_sum > 0
    overfitting_downgrade = False

    # Rule 1: Overfitting downgrade
    if overfitting_flag and calendar_sum >= 0.10:
        overfitting_downgrade = True
        return GoNoGoDecision(
            decision="marginal_go",
            calendar_sum=calendar_sum,
            worst_sm=worst_sm,
            btc_positive=btc_positive,
            eth_positive=eth_positive,
            forward_calendar_sum=forward_calendar_sum,
            overfitting_downgrade=True,
        )

    # Rule 2: Strong Go
    if (
        calendar_sum >= 0.10
        and worst_sm > -0.005
        and btc_positive
        and eth_positive
        and forward_calendar_sum >= 0.07
    ):
        return GoNoGoDecision(
            decision="strong_go",
            calendar_sum=calendar_sum,
            worst_sm=worst_sm,
            btc_positive=btc_positive,
            eth_positive=eth_positive,
            forward_calendar_sum=forward_calendar_sum,
            overfitting_downgrade=False,
        )

    # Rule 3: No-Go
    if calendar_sum < 0.07 or worst_sm < -0.01:
        return GoNoGoDecision(
            decision="no_go",
            calendar_sum=calendar_sum,
            worst_sm=worst_sm,
            btc_positive=btc_positive,
            eth_positive=eth_positive,
            forward_calendar_sum=forward_calendar_sum,
            overfitting_downgrade=False,
        )

    # Rule 4: Marginal Go (everything else)
    return GoNoGoDecision(
        decision="marginal_go",
        calendar_sum=calendar_sum,
        worst_sm=worst_sm,
        btc_positive=btc_positive,
        eth_positive=eth_positive,
        forward_calendar_sum=forward_calendar_sum,
        overfitting_downgrade=False,
    )


# ---------------------------------------------------------------------------
# Helper: JSON serialization
# ---------------------------------------------------------------------------


class _NumpyEncoder(json.JSONEncoder):
    """JSON encoder that handles numpy types and pandas Timestamps."""

    def default(self, obj):
        if isinstance(obj, (np.integer,)):
            return int(obj)
        if isinstance(obj, (np.floating,)):
            return float(obj)
        if isinstance(obj, np.ndarray):
            return obj.tolist()
        if isinstance(obj, (pd.Timestamp,)):
            return obj.isoformat()
        if isinstance(obj, (np.bool_,)):
            return bool(obj)
        if hasattr(obj, "isoformat"):
            return obj.isoformat()
        return super().default(obj)


def _safe_pct(value: float) -> str:
    """Format a decimal value as percentage string."""
    return f"{value * 100:.2f}%"


# ---------------------------------------------------------------------------
# Helper: Generate unified_report.md content
# ---------------------------------------------------------------------------


def _generate_report_md(
    decision: GoNoGoDecision,
    timing_result: TimingClassifierResult,
    rf_result: RFProbabilityResult,
    speed_gate_result: SpeedGateResult,
    robustness_result: RobustnessResult,
    sensitivity_rows: list,
    event_pool_stats: EventPoolStats,
    execution_stats: dict,
) -> str:
    """Generate the Chinese markdown report content."""
    bootstrap = robustness_result.bootstrap
    forward = robustness_result.forward
    ablation = robustness_result.ablation

    decision_map = {
        "strong_go": "Strong Go \u2705 \u2014 \u63a8\u8fdb\u751f\u4ea7\u96c6\u6210",
        "marginal_go": "Marginal Go \u26a0\ufe0f \u2014 \u7ee7\u7eed\u4f18\u5316\u6216\u6269\u5c55\u6570\u636e",
        "no_go": "No-Go \u274c \u2014 \u65b9\u6848\u4e0d\u53ef\u884c",
    }
    decision_text = decision_map.get(decision.decision, decision.decision)

    full_cs = decision.calendar_sum * 100

    # Ablation values
    timing_only_cs = 0.0
    prob_only_cs = 0.0
    for row in ablation:
        if row.config_name == "timing_only":
            timing_only_cs = row.calendar_sum * 100
        elif row.config_name == "probability_only":
            prob_only_cs = row.calendar_sum * 100

    # Hypothesis validation
    h1_confirmed = full_cs > timing_only_cs and full_cs > prob_only_cs
    h1_status = "confirmed" if h1_confirmed else "rejected"

    gate_on_cs = speed_gate_result.gate_on_calendar_sum * 100
    gate_off_cs = speed_gate_result.gate_off_calendar_sum * 100
    h2_confirmed = gate_on_cs > gate_off_cs and speed_gate_result.gate_pass_rate >= 0.70
    if h2_confirmed:
        h2_status = "confirmed"
    elif speed_gate_result.aggressive_gate_warning:
        h2_status = "inconclusive"
    else:
        h2_status = "rejected"

    h3_confirmed = decision.calendar_sum > 0
    h3_status = "confirmed" if h3_confirmed else "rejected"

    fwd_cs = forward.forward_calendar_sum * 100
    h4_confirmed = fwd_cs >= 7.0 and decision.btc_positive and decision.eth_positive
    h4_status = "confirmed" if h4_confirmed else "rejected"

    lines: list[str] = []
    lines.append("# Timing-Probability Unified Framework \u2014 Go/No-Go \u62a5\u544a")
    lines.append("")
    lines.append("> **\u58f0\u660e**\uff1a\u672c\u5b9e\u9a8c\u4e0d\u6539\u53d8\u524d\u5e8f spec \u7684\u65e2\u6709\u90e8\u7f72/\u53d1\u73b0\uff1b")
    lines.append("> \u7ed3\u8bba\u4ec5\u4f5c\u4e3a\u751f\u4ea7\u96c6\u6210\u51b3\u7b56\u7684\u8f93\u5165\u3002")
    lines.append("")
    lines.append(f"**\u751f\u6210\u65f6\u95f4**: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    lines.append("")

    # Section 1: Go/No-Go
    lines.append("## 1. Go/No-Go \u5224\u5b9a")
    lines.append("")
    lines.append(f"### \u5224\u5b9a\u7ed3\u679c: {decision_text}")
    lines.append("")
    lines.append("| \u6307\u6807 | \u6570\u503c | \u9608\u503c |")
    lines.append("|------|------|------|")
    lines.append(f"| Calendar Sum | {full_cs:.2f}% | \u2265 10% (Strong) / \u2265 7% (Marginal) |")
    lines.append(f"| Worst SM | {decision.worst_sm*100:.2f}% | > -0.5% (Strong) / \u2265 -1.0% (Marginal) |")
    btc_icon = "\u2705" if decision.btc_positive else "\u274c"
    eth_icon = "\u2705" if decision.eth_positive else "\u274c"
    lines.append(f"| BTC Positive | {btc_icon} | Required for Strong Go |")
    lines.append(f"| ETH Positive | {eth_icon} | Required for Strong Go |")
    lines.append(f"| Forward CS | {decision.forward_calendar_sum*100:.2f}% | \u2265 7% (Strong) |")
    ovf_icon = "\u2705" if decision.overfitting_downgrade else "\u274c"
    lines.append(f"| Overfitting Downgrade | {ovf_icon} | |")
    lines.append("")

    # Section 2: Hypothesis Validation
    lines.append("## 2. \u5047\u8bbe\u9a8c\u8bc1")
    lines.append("")
    lines.append("### H1: \u7ec4\u5408\u4f18\u52bf\u5047\u8bbe")
    lines.append("")
    lines.append(f"- Full Unified CS: {full_cs:.2f}%")
    lines.append(f"- Timing Only CS: {timing_only_cs:.2f}%")
    lines.append(f"- Probability Only CS: {prob_only_cs:.2f}%")
    lines.append(f"- **\u7ed3\u8bba: {h1_status}**")
    lines.append("")
    lines.append("### H2: Speed Gate \u5047\u8bbe")
    lines.append("")
    lines.append(f"- Gate ON CS: {gate_on_cs:.2f}%")
    lines.append(f"- Gate OFF CS: {gate_off_cs:.2f}%")
    lines.append(f"- Gate Pass Rate: {speed_gate_result.gate_pass_rate*100:.1f}%")
    lines.append(f"- Worst SM (Gate ON): {speed_gate_result.gate_on_worst_sm*100:.2f}%")
    lines.append(f"- **\u7ed3\u8bba: {h2_status}**")
    lines.append("")
    lines.append("### H3: Event-Exact \u6267\u884c\u5047\u8bbe")
    lines.append("")
    lines.append(f"- Unified Calendar Sum: {full_cs:.2f}% (event-exact V4)")
    lines.append(f"- **\u7ed3\u8bba: {h3_status}** (\u6b63\u6536\u76ca = confirmed)")
    lines.append("")
    lines.append("### H4: \u7a33\u5065\u6027\u5047\u8bbe")
    lines.append("")
    lines.append(f"- Forward CS: {fwd_cs:.2f}% (\u9608\u503c \u2265 7%)")
    lines.append(f"- BTC Positive: {decision.btc_positive}")
    lines.append(f"- ETH Positive: {decision.eth_positive}")
    lines.append(f"- **\u7ed3\u8bba: {h4_status}**")
    lines.append("")

    # Section 3: Historical Comparison
    n_events = event_pool_stats.total_events
    lines.append("## 3. \u5386\u53f2\u5bf9\u6bd4\u8868")
    lines.append("")
    lines.append(f"| \u6307\u6807 | Path C (1013 events) | Tick-Flow (481 events) | Unified ({n_events} events) |")
    lines.append("|------|-----|-----|-----|")
    lines.append(f"| calendar_sum | +7.40% | +16.21% | {full_cs:+.2f}% |")
    lines.append(f"| worst SM | - | -0.48% | {decision.worst_sm*100:+.2f}% |")
    lines.append(f"| bootstrap CI | [+3.67%, +11.43%] | - | [{bootstrap.calendar_sum_ci_5*100:+.2f}%, {bootstrap.calendar_sum_ci_95*100:+.2f}%] |")
    lines.append(f"| CI width | 7.76pp | - | {bootstrap.ci_width*100:.2f}pp |")
    lines.append(f"| BTC | +2.20% | +4.84% | {bootstrap.btc_calendar_sum*100:+.2f}% |")
    lines.append(f"| ETH | +5.12% | +11.36% | {bootstrap.eth_calendar_sum*100:+.2f}% |")
    lines.append(f"| trade count | - | 481 | {n_events} |")
    lines.append("| execution model | lifecycle | event-exact | event-exact |")
    lines.append("| sizing | fixed | RF overlay | timing \u00d7 RF |")
    lines.append("")

    # Section 4: Ablation
    lines.append("## 4. Ablation Study")
    lines.append("")
    lines.append("| \u914d\u7f6e | Calendar Sum | Worst SM | Trade Count | Avg PnL/Trade |")
    lines.append("|------|-------------|----------|-------------|---------------|")
    for row in ablation:
        lines.append(
            f"| {row.config_name} | {row.calendar_sum*100:.2f}% "
            f"| {row.worst_sm*100:.2f}% | {row.trade_count} "
            f"| {row.avg_pnl_per_trade*100:.4f}% |"
        )
    lines.append("")

    # Section 5: Sensitivity
    lines.append("## 5. Base Share \u654f\u611f\u6027\u5206\u6790")
    lines.append("")
    lines.append("| Base Share | Calendar Sum | Worst SM | Trade Count | Avg PnL/Trade |")
    lines.append("|-----------|-------------|----------|-------------|---------------|")
    for row in sensitivity_rows:
        if isinstance(row, SensitivityRow):
            lines.append(
                f"| {row.base_share:.2f} | {row.calendar_sum*100:.2f}% "
                f"| {row.worst_sm*100:.2f}% | {row.trade_count} "
                f"| {row.avg_pnl_per_trade*100:.4f}% |"
            )
        elif isinstance(row, dict):
            lines.append(
                f"| {row['base_share']:.2f} | {row['calendar_sum']*100:.2f}% "
                f"| {row['worst_sm']*100:.2f}% | {row['trade_count']} "
                f"| {row['avg_pnl_per_trade']*100:.4f}% |"
            )
    lines.append("")

    # Section 6: Bootstrap
    lines.append("## 6. Bootstrap \u7f6e\u4fe1\u533a\u95f4")
    lines.append("")
    lines.append(f"- Overall: [{bootstrap.calendar_sum_ci_5*100:+.2f}%, {bootstrap.calendar_sum_ci_95*100:+.2f}%] (width: {bootstrap.ci_width*100:.2f}pp)")
    lines.append(f"- BTC: [{bootstrap.btc_ci_5*100:+.2f}%, {bootstrap.btc_ci_95*100:+.2f}%]")
    lines.append(f"- ETH: [{bootstrap.eth_ci_5*100:+.2f}%, {bootstrap.eth_ci_95*100:+.2f}%]")
    lines.append(f"- Path C CI width \u5bf9\u6bd4: 7.76pp \u2192 {bootstrap.ci_width*100:.2f}pp")
    lines.append("")

    # Section 7: Forward
    lines.append("## 7. Forward Split \u9a8c\u8bc1")
    lines.append("")
    lines.append(f"- Forward Calendar Sum: {fwd_cs:.2f}%")
    lines.append(f"- Forward Worst SM: {forward.forward_worst_sm*100:.2f}%")
    lines.append(f"- Forward Trade Count: {forward.forward_trade_count}")
    lines.append(f"- Overfitting Flag: {forward.overfitting_flag}")
    lines.append(f"- Forward Risk Flag: {forward.forward_risk_flag}")
    lines.append(f"- Forward Underperformance: {forward.forward_underperformance}")
    lines.append("")

    # Section 8: Timing Classifier
    lines.append("## 8. Timing Classifier \u7ed3\u679c")
    lines.append("")
    lines.append(f"- Selected Depth: DT{timing_result.selected_depth}")
    lines.append(f"- DT3 LOOCV CS: {timing_result.dt3_loocv_calendar_sum*100:.2f}%")
    lines.append(f"- DT4 LOOCV CS: {timing_result.dt4_loocv_calendar_sum*100:.2f}%")
    lines.append(f"- Test CS: {timing_result.test_calendar_sum*100:.2f}%")
    lines.append(f"- Regime Distribution: {timing_result.regime_distribution}")
    lines.append("")

    # Section 9: RF Model
    lines.append("## 9. RF \u6982\u7387\u6a21\u578b\u7ed3\u679c")
    lines.append("")
    lines.append(f"- Train AUC: {rf_result.train_auc:.4f}")
    lines.append(f"- Test AUC: {rf_result.test_auc:.4f}")
    lines.append(f"- RF No Signal Warning: {rf_result.rf_no_signal_warning}")
    lines.append(f"- Probability Stats: mean={rf_result.prob_mean:.4f}, median={rf_result.prob_median:.4f}, std={rf_result.prob_std:.4f}")
    lines.append("- Feature Importance Top-5:")
    for feat, imp in rf_result.feature_importance_top5:
        lines.append(f"  - {feat}: {imp:.4f}")
    lines.append("")

    # Section 10: Speed Gate
    lines.append("## 10. Speed Gate \u5206\u6790")
    lines.append("")
    lines.append(f"- Threshold (train q10): {speed_gate_result.threshold:.6f}")
    lines.append(f"- Gate Pass Rate: {speed_gate_result.gate_pass_rate*100:.1f}%")
    lines.append(f"- Aggressive Gate Warning: {speed_gate_result.aggressive_gate_warning}")
    lines.append(f"- Retained Avg PnL: {speed_gate_result.retained_avg_pnl*100:.4f}%")
    lines.append(f"- Filtered Avg PnL: {speed_gate_result.filtered_avg_pnl*100:.4f}%")
    lines.append("")

    # Section 11: Event Pool
    lines.append("## 11. \u4e8b\u4ef6\u6c60\u7edf\u8ba1")
    lines.append("")
    lines.append(f"- Total Events: {event_pool_stats.total_events}")
    lines.append(f"- BTC: {event_pool_stats.btc_count} ({event_pool_stats.btc_pct:.1f}%)")
    lines.append(f"- ETH: {event_pool_stats.eth_count} ({event_pool_stats.eth_pct:.1f}%)")
    lines.append(f"- Long: {event_pool_stats.long_count} ({event_pool_stats.long_pct:.1f}%)")
    lines.append(f"- Short: {event_pool_stats.short_count} ({event_pool_stats.short_pct:.1f}%)")
    lines.append(f"- Small Pool Warning: {event_pool_stats.small_pool_warning}")
    lines.append("")

    # Section 12: Next Steps
    lines.append("## 12. \u4e0b\u4e00\u6b65\u884c\u52a8\u5efa\u8bae")
    lines.append("")
    if decision.decision == "strong_go":
        lines.append("- \u2705 \u4ea7\u51fa production integration spec")
        lines.append("- \u56fa\u5316 timing rules + RF model + speed gate \u4e3a live \u5165\u573a\u903b\u8f91")
        lines.append("- \u8bbe\u8ba1 live \u6267\u884c\u53c2\u6570\u6620\u5c04\uff08delay \u2192 \u5b9e\u76d8 entry timing\uff09")
    elif decision.decision == "marginal_go":
        lines.append("- \u26a0\ufe0f \u5206\u6790\u74f6\u9888\u7ec4\u4ef6\uff08\u53c2\u8003 Ablation Study \u7ed3\u679c\uff09")
        lines.append("- \u8003\u8651\u8c03\u6574 sizing \u6620\u5c04\u6216 speed gate \u9608\u503c")
        lines.append("- \u6269\u5c55 forward \u9a8c\u8bc1\u7a97\u53e3\u4ee5\u589e\u5f3a\u7edf\u8ba1\u4fe1\u5fc3")
        if decision.overfitting_downgrade:
            lines.append("- \u26a0\ufe0f Overfitting \u964d\u7ea7\uff1a\u5efa\u8bae\u6269\u5c55 forward \u9a8c\u8bc1\u7a97\u53e3")
    else:
        lines.append("- \u274c \u5206\u6790\u5931\u8d25\u539f\u56e0")
        lines.append("- \u8bc4\u4f30\u662f\u5426\u56de\u9000\u5230 tick-flow \u7684\u7eaf probability sizing \u65b9\u6848")
        lines.append("- \u68c0\u67e5 timing classifier \u662f\u5426\u5728\u5f53\u524d\u4e8b\u4ef6\u6c60\u4e0a\u6709\u6548")
    lines.append("")

    return "\n".join(lines)


# ---------------------------------------------------------------------------
# Helper: Build unified_summary.json content
# ---------------------------------------------------------------------------


def _build_summary_dict(
    decision: GoNoGoDecision,
    timing_result: TimingClassifierResult,
    rf_result: RFProbabilityResult,
    speed_gate_result: SpeedGateResult,
    robustness_result: RobustnessResult,
    sensitivity_rows: list,
    event_pool_stats: EventPoolStats,
    execution_stats: dict,
) -> dict:
    """Build the structured summary dictionary for unified_summary.json."""
    bootstrap = robustness_result.bootstrap
    forward = robustness_result.forward
    ablation = robustness_result.ablation

    summary = {
        "go_no_go_decision": {
            "decision": decision.decision,
            "calendar_sum": decision.calendar_sum,
            "worst_sm": decision.worst_sm,
            "btc_positive": decision.btc_positive,
            "eth_positive": decision.eth_positive,
            "forward_calendar_sum": decision.forward_calendar_sum,
            "overfitting_downgrade": decision.overfitting_downgrade,
        },
        "timing_classifier": {
            "selected_depth": timing_result.selected_depth,
            "dt3_loocv_calendar_sum": timing_result.dt3_loocv_calendar_sum,
            "dt4_loocv_calendar_sum": timing_result.dt4_loocv_calendar_sum,
            "test_calendar_sum": timing_result.test_calendar_sum,
            "regime_distribution": timing_result.regime_distribution,
        },
        "rf_probability": {
            "train_auc": rf_result.train_auc,
            "test_auc": rf_result.test_auc,
            "feature_importance_top5": [
                {"feature": f, "importance": float(i)}
                for f, i in rf_result.feature_importance_top5
            ],
            "prob_mean": rf_result.prob_mean,
            "prob_median": rf_result.prob_median,
            "prob_std": rf_result.prob_std,
            "rf_no_signal_warning": rf_result.rf_no_signal_warning,
        },
        "speed_gate": {
            "threshold": speed_gate_result.threshold,
            "gate_pass_rate": speed_gate_result.gate_pass_rate,
            "gate_on_calendar_sum": speed_gate_result.gate_on_calendar_sum,
            "gate_off_calendar_sum": speed_gate_result.gate_off_calendar_sum,
            "gate_on_worst_sm": speed_gate_result.gate_on_worst_sm,
            "gate_off_worst_sm": speed_gate_result.gate_off_worst_sm,
            "aggressive_gate_warning": speed_gate_result.aggressive_gate_warning,
        },
        "bootstrap": {
            "calendar_sum_mean": bootstrap.calendar_sum_mean,
            "calendar_sum_ci_5": bootstrap.calendar_sum_ci_5,
            "calendar_sum_ci_95": bootstrap.calendar_sum_ci_95,
            "ci_width": bootstrap.ci_width,
            "btc_calendar_sum": bootstrap.btc_calendar_sum,
            "btc_ci_5": bootstrap.btc_ci_5,
            "btc_ci_95": bootstrap.btc_ci_95,
            "eth_calendar_sum": bootstrap.eth_calendar_sum,
            "eth_ci_5": bootstrap.eth_ci_5,
            "eth_ci_95": bootstrap.eth_ci_95,
        },
        "forward": {
            "forward_calendar_sum": forward.forward_calendar_sum,
            "forward_worst_sm": forward.forward_worst_sm,
            "forward_trade_count": forward.forward_trade_count,
            "overfitting_flag": forward.overfitting_flag,
            "forward_risk_flag": forward.forward_risk_flag,
            "forward_underperformance": forward.forward_underperformance,
        },
        "ablation": [
            {
                "config_name": row.config_name,
                "calendar_sum": row.calendar_sum,
                "worst_sm": row.worst_sm,
                "trade_count": row.trade_count,
                "avg_pnl_per_trade": row.avg_pnl_per_trade,
            }
            for row in ablation
        ],
        "sensitivity": [
            {
                "base_share": row.base_share if isinstance(row, SensitivityRow) else row["base_share"],
                "calendar_sum": row.calendar_sum if isinstance(row, SensitivityRow) else row["calendar_sum"],
                "worst_sm": row.worst_sm if isinstance(row, SensitivityRow) else row["worst_sm"],
                "trade_count": row.trade_count if isinstance(row, SensitivityRow) else row["trade_count"],
                "avg_pnl_per_trade": row.avg_pnl_per_trade if isinstance(row, SensitivityRow) else row["avg_pnl_per_trade"],
            }
            for row in sensitivity_rows
        ],
        "event_pool_stats": {
            "total_events": event_pool_stats.total_events,
            "btc_count": event_pool_stats.btc_count,
            "eth_count": event_pool_stats.eth_count,
            "btc_pct": event_pool_stats.btc_pct,
            "eth_pct": event_pool_stats.eth_pct,
            "long_count": event_pool_stats.long_count,
            "short_count": event_pool_stats.short_count,
            "long_pct": event_pool_stats.long_pct,
            "short_pct": event_pool_stats.short_pct,
            "small_pool_warning": event_pool_stats.small_pool_warning,
        },
        "execution_stats": execution_stats,
    }
    return summary


# ---------------------------------------------------------------------------
# Main: generate_report()
# ---------------------------------------------------------------------------


def generate_report(
    output_dir: Path,
    trades: pd.DataFrame,
    timing_result: TimingClassifierResult,
    rf_result: RFProbabilityResult,
    speed_gate_result: SpeedGateResult,
    robustness_result: RobustnessResult,
    sensitivity_rows: list,
    event_pool_stats: EventPoolStats,
    events_pool: pd.DataFrame,
    execution_stats: dict,
) -> GoNoGoDecision:
    """Generate the complete report suite (13 output files).

    Output files:
    1. unified_report.md
    2. unified_summary.json
    3. unified_trades.csv
    4. timing_classifier_results.json
    5. rf_probability_results.json
    6. speed_gate_analysis.json
    7. execution_stats.json
    8. ablation_results.json
    9. bootstrap_results.json
    10. sensitivity_analysis.json
    11. events_pool_stats.json
    12. events_pool.csv
    13. timing_rules.md

    Parameters
    ----------
    output_dir : Path
        Output directory path.
    trades : pd.DataFrame
        Unified trades DataFrame.
    timing_result : TimingClassifierResult
        Timing classifier results.
    rf_result : RFProbabilityResult
        RF probability model results.
    speed_gate_result : SpeedGateResult
        Speed gate analysis results.
    robustness_result : RobustnessResult
        Robustness validation results (bootstrap, forward, ablation).
    sensitivity_rows : list
        Sensitivity analysis rows (list of SensitivityRow or dicts).
    event_pool_stats : EventPoolStats
        Event pool statistics.
    events_pool : pd.DataFrame
        Full events pool DataFrame.
    execution_stats : dict
        Execution statistics dictionary.

    Returns
    -------
    GoNoGoDecision
        The Go/No-Go decision.
    """
    # Create output directory
    output_dir = Path(output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)

    logger.info("Generating report to %s", output_dir)

    # --- Compute Go/No-Go Decision ---
    bootstrap = robustness_result.bootstrap
    forward = robustness_result.forward

    # calendar_sum from trades (gate ON)
    calendar_sum = compute_calendar_sum(trades, gate_filter=True)
    worst_sm = compute_worst_sm(trades, gate_filter=True)

    decision = compute_go_no_go_decision(
        calendar_sum=calendar_sum,
        worst_sm=worst_sm,
        btc_calendar_sum=bootstrap.btc_calendar_sum,
        eth_calendar_sum=bootstrap.eth_calendar_sum,
        forward_calendar_sum=forward.forward_calendar_sum,
        overfitting_flag=forward.overfitting_flag,
    )

    # --- File 1: unified_report.md ---
    report_md = _generate_report_md(
        decision=decision,
        timing_result=timing_result,
        rf_result=rf_result,
        speed_gate_result=speed_gate_result,
        robustness_result=robustness_result,
        sensitivity_rows=sensitivity_rows,
        event_pool_stats=event_pool_stats,
        execution_stats=execution_stats,
    )
    (output_dir / "unified_report.md").write_text(report_md, encoding="utf-8")
    logger.info("Written: unified_report.md")

    # --- File 2: unified_summary.json ---
    summary = _build_summary_dict(
        decision=decision,
        timing_result=timing_result,
        rf_result=rf_result,
        speed_gate_result=speed_gate_result,
        robustness_result=robustness_result,
        sensitivity_rows=sensitivity_rows,
        event_pool_stats=event_pool_stats,
        execution_stats=execution_stats,
    )
    with open(output_dir / "unified_summary.json", "w", encoding="utf-8") as f:
        json.dump(summary, f, indent=2, ensure_ascii=False, cls=_NumpyEncoder)
    logger.info("Written: unified_summary.json")

    # --- File 3: unified_trades.csv ---
    trades.to_csv(output_dir / "unified_trades.csv", index=False)
    logger.info("Written: unified_trades.csv")

    # --- File 4: timing_classifier_results.json ---
    timing_json = {
        "selected_depth": timing_result.selected_depth,
        "dt3_loocv_calendar_sum": timing_result.dt3_loocv_calendar_sum,
        "dt4_loocv_calendar_sum": timing_result.dt4_loocv_calendar_sum,
        "test_calendar_sum": timing_result.test_calendar_sum,
        "regime_distribution": timing_result.regime_distribution,
        "rules_text": timing_result.rules_text,
    }
    with open(output_dir / "timing_classifier_results.json", "w", encoding="utf-8") as f:
        json.dump(timing_json, f, indent=2, ensure_ascii=False, cls=_NumpyEncoder)
    logger.info("Written: timing_classifier_results.json")

    # --- File 5: rf_probability_results.json ---
    rf_json = {
        "train_auc": rf_result.train_auc,
        "test_auc": rf_result.test_auc,
        "feature_importance_top5": [
            {"feature": f, "importance": float(i)}
            for f, i in rf_result.feature_importance_top5
        ],
        "prob_mean": rf_result.prob_mean,
        "prob_median": rf_result.prob_median,
        "prob_std": rf_result.prob_std,
        "rf_no_signal_warning": rf_result.rf_no_signal_warning,
    }
    with open(output_dir / "rf_probability_results.json", "w", encoding="utf-8") as f:
        json.dump(rf_json, f, indent=2, ensure_ascii=False, cls=_NumpyEncoder)
    logger.info("Written: rf_probability_results.json")

    # --- File 6: speed_gate_analysis.json ---
    speed_gate_json = {
        "threshold": speed_gate_result.threshold,
        "gate_pass_rate": speed_gate_result.gate_pass_rate,
        "gate_on_calendar_sum": speed_gate_result.gate_on_calendar_sum,
        "gate_off_calendar_sum": speed_gate_result.gate_off_calendar_sum,
        "gate_on_worst_sm": speed_gate_result.gate_on_worst_sm,
        "gate_off_worst_sm": speed_gate_result.gate_off_worst_sm,
        "filtered_avg_pnl": speed_gate_result.filtered_avg_pnl,
        "retained_avg_pnl": speed_gate_result.retained_avg_pnl,
        "aggressive_gate_warning": speed_gate_result.aggressive_gate_warning,
    }
    with open(output_dir / "speed_gate_analysis.json", "w", encoding="utf-8") as f:
        json.dump(speed_gate_json, f, indent=2, ensure_ascii=False, cls=_NumpyEncoder)
    logger.info("Written: speed_gate_analysis.json")

    # --- File 7: execution_stats.json ---
    with open(output_dir / "execution_stats.json", "w", encoding="utf-8") as f:
        json.dump(execution_stats, f, indent=2, ensure_ascii=False, cls=_NumpyEncoder)
    logger.info("Written: execution_stats.json")

    # --- File 8: ablation_results.json ---
    ablation_json = [
        {
            "config_name": row.config_name,
            "calendar_sum": row.calendar_sum,
            "worst_sm": row.worst_sm,
            "trade_count": row.trade_count,
            "avg_pnl_per_trade": row.avg_pnl_per_trade,
        }
        for row in robustness_result.ablation
    ]
    with open(output_dir / "ablation_results.json", "w", encoding="utf-8") as f:
        json.dump(ablation_json, f, indent=2, ensure_ascii=False, cls=_NumpyEncoder)
    logger.info("Written: ablation_results.json")

    # --- File 9: bootstrap_results.json ---
    bootstrap_json = {
        "calendar_sum_mean": bootstrap.calendar_sum_mean,
        "calendar_sum_ci_5": bootstrap.calendar_sum_ci_5,
        "calendar_sum_ci_95": bootstrap.calendar_sum_ci_95,
        "ci_width": bootstrap.ci_width,
        "btc_calendar_sum": bootstrap.btc_calendar_sum,
        "btc_ci_5": bootstrap.btc_ci_5,
        "btc_ci_95": bootstrap.btc_ci_95,
        "eth_calendar_sum": bootstrap.eth_calendar_sum,
        "eth_ci_5": bootstrap.eth_ci_5,
        "eth_ci_95": bootstrap.eth_ci_95,
    }
    with open(output_dir / "bootstrap_results.json", "w", encoding="utf-8") as f:
        json.dump(bootstrap_json, f, indent=2, ensure_ascii=False, cls=_NumpyEncoder)
    logger.info("Written: bootstrap_results.json")

    # --- File 10: sensitivity_analysis.json ---
    sensitivity_json = []
    for row in sensitivity_rows:
        if isinstance(row, SensitivityRow):
            sensitivity_json.append({
                "base_share": row.base_share,
                "calendar_sum": row.calendar_sum,
                "worst_sm": row.worst_sm,
                "trade_count": row.trade_count,
                "avg_pnl_per_trade": row.avg_pnl_per_trade,
            })
        elif isinstance(row, dict):
            sensitivity_json.append(row)
    with open(output_dir / "sensitivity_analysis.json", "w", encoding="utf-8") as f:
        json.dump(sensitivity_json, f, indent=2, ensure_ascii=False, cls=_NumpyEncoder)
    logger.info("Written: sensitivity_analysis.json")

    # --- File 11: events_pool_stats.json ---
    pool_stats_json = {
        "total_events": event_pool_stats.total_events,
        "btc_count": event_pool_stats.btc_count,
        "eth_count": event_pool_stats.eth_count,
        "btc_pct": event_pool_stats.btc_pct,
        "eth_pct": event_pool_stats.eth_pct,
        "long_count": event_pool_stats.long_count,
        "short_count": event_pool_stats.short_count,
        "long_pct": event_pool_stats.long_pct,
        "short_pct": event_pool_stats.short_pct,
        "earliest_touch_time": str(event_pool_stats.earliest_touch_time),
        "latest_touch_time": str(event_pool_stats.latest_touch_time),
        "small_pool_warning": event_pool_stats.small_pool_warning,
    }
    with open(output_dir / "events_pool_stats.json", "w", encoding="utf-8") as f:
        json.dump(pool_stats_json, f, indent=2, ensure_ascii=False, cls=_NumpyEncoder)
    logger.info("Written: events_pool_stats.json")

    # --- File 12: events_pool.csv ---
    events_pool.to_csv(output_dir / "events_pool.csv", index=False)
    logger.info("Written: events_pool.csv")

    # --- File 13: timing_rules.md ---
    rules_content = f"# Timing Classifier Decision Rules (DT{timing_result.selected_depth})\n\n"
    rules_content += timing_result.rules_text
    (output_dir / "timing_rules.md").write_text(rules_content, encoding="utf-8")
    logger.info("Written: timing_rules.md")

    logger.info("Report generation complete. Decision: %s", decision.decision)
    return decision
