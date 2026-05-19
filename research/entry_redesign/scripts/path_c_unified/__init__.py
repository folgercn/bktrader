"""
path_c_unified — Path C Unified Pretouch 实验包

基于已确认的最优组合（3-regime + DT3 + 原 10 特征），通过扩大事件池
（116 → 200+ events）验证 timing signal 的统计稳定性。

核心设计原则：
- 固定模型组合：不再探索新模型/特征，仅验证已确认最优组合在更大样本上的稳定性
- 最小化新代码：复用 pre_breakout_timing/ 基础设施
- 向后兼容：扩展池必须包含原 116 events 作为子集
- 统计严谨：所有结论附带 bootstrap CI

Usage:
    cd research/entry_redesign/scripts
    python -m path_c_unified.path_c_runner
"""

from __future__ import annotations

import sys
from pathlib import Path

# ---------------------------------------------------------------------------
# sys.path 设置：确保 pre_breakout_timing/ 和 pretouch_refinement/ 可正常 import
# ---------------------------------------------------------------------------

SCRIPTS_DIR = Path(__file__).resolve().parent.parent
if str(SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPTS_DIR))

# ---------------------------------------------------------------------------
# 公开模块导出
# ---------------------------------------------------------------------------

__all__ = [
    "gate_explorer",
    "expanded_data_layer",
    "delay_simulation_runner",
    "label_generator",
    "classifier_trainer",
    "robustness",
    "report_generator",
    "path_c_runner",
]
