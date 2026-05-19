"""
pretouch_refinement — Pretouch Classifier Refinement 实验包

基于已完成的 pre-breakout-timing-classifier（Conditional Go，test calendar_sum +2.98%）
进行 refinement，通过特征增强（Path A）与 Regime 简化（Path B）两条并行路径，
验证能否将 Oracle 实现率从 31.6% 提升至 ≥40%。

核心设计原则：
- 完全复用 pre_breakout_timing/ 模块的 data_layer、delay_simulator、timing_classifier 框架
- 不重跑 multi-delay simulation，直接复用已产出的 delay_pnl_matrix.csv（580 行 × 15 列）
- 6 个实验 arm（Baseline + 5 treatment），每 arm 跑 4 种分类器
- 6 类增强特征 + 逐类消融
- 两种 regime 简化（三分类 + 二分类）

Usage:
    cd research/entry_redesign/scripts
    python -m pretouch_refinement.refinement_runner
"""

from __future__ import annotations

import sys
from pathlib import Path

# ---------------------------------------------------------------------------
# sys.path 设置：确保 pre_breakout_timing/ 可正常 import
# ---------------------------------------------------------------------------

SCRIPTS_DIR = Path(__file__).resolve().parent.parent
if str(SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPTS_DIR))

# ---------------------------------------------------------------------------
# 公开模块导出
# ---------------------------------------------------------------------------

__all__ = [
    "enhanced_features",
    "regime_labels",
    "arm_runner",
    "ablation",
    "refinement_report",
    "refinement_runner",
]
