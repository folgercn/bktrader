"""
regime_labels — 三分类 / 二分类 regime 标签生成

基于已有 delay_pnl_matrix.csv 重新分配标签，不重跑任何 execute_trade()。
"""

from __future__ import annotations

from pathlib import Path

import pandas as pd
import numpy as np


# ---------------------------------------------------------------------------
# 常量
# ---------------------------------------------------------------------------

REGIME_3_LABELS: list[str] = ["skip", "fast", "slow"]
REGIME_2_LABELS: list[str] = ["enter", "skip"]

# fast 对应的原始 delay
FAST_DELAYS: list[str] = ["D0", "D5"]
# slow 对应的原始 delay
SLOW_DELAYS: list[str] = ["D10", "D15", "pullback"]

# Best_Global_Delay 候选（不含 pullback）
GLOBAL_DELAY_CANDIDATES: list[str] = ["D0", "D5", "D10", "D15"]

# 默认 delay_pnl_matrix.csv 路径
_SCRIPTS_DIR = Path(__file__).resolve().parent.parent
DEFAULT_DELAY_MATRIX_PATH = (
    _SCRIPTS_DIR / "output" / "pre_breakout_timing" / "delay_pnl_matrix.csv"
)

# 预期 shape
EXPECTED_ROWS = 580
EXPECTED_COLS = 15

# 必须包含的列
REQUIRED_COLUMNS: list[str] = ["event_id", "delay_label", "pnl_pct", "traded"]


# ---------------------------------------------------------------------------
# 公开接口
# ---------------------------------------------------------------------------


def load_delay_pnl_matrix(
    matrix_path: str | None = None,
) -> pd.DataFrame:
    """加载并验证 delay_pnl_matrix.csv。

    验证规则：
    - shape 应为 580 行 × 15 列
    - 必须包含 event_id, delay_label, pnl_pct, traded 列
    - 若不满足则 raise ValueError（non-zero exit）

    Parameters
    ----------
    matrix_path : str | None
        CSV 文件路径。若为 None 则使用默认路径。

    Returns
    -------
    pd.DataFrame
        验证通过的 delay_pnl_matrix。

    Raises
    ------
    ValueError
        当文件不存在、shape 不符或缺少必要列时抛出。
    """
    path = Path(matrix_path) if matrix_path else DEFAULT_DELAY_MATRIX_PATH

    if not path.exists():
        raise ValueError(
            f"delay_pnl_matrix.csv 不存在: {path}\n"
            "请先运行 pre_breakout_timing 实验生成该文件。"
        )

    df = pd.read_csv(path)

    # 验证 shape
    n_rows, n_cols = df.shape
    if n_rows != EXPECTED_ROWS or n_cols != EXPECTED_COLS:
        raise ValueError(
            f"delay_pnl_matrix.csv shape 不符: 期望 ({EXPECTED_ROWS}, {EXPECTED_COLS})，"
            f"实际 ({n_rows}, {n_cols})。"
        )

    # 验证必须包含的列
    missing_cols = [col for col in REQUIRED_COLUMNS if col not in df.columns]
    if missing_cols:
        raise ValueError(
            f"delay_pnl_matrix.csv 缺少必要列: {missing_cols}。"
            f"实际列: {list(df.columns)}"
        )

    return df

def generate_2regime_labels(
    matrix: pd.DataFrame,
    train_events: pd.DataFrame,
    all_events: pd.DataFrame,
) -> tuple[pd.Series, str]:
    """基于 Best_Global_Delay 生成二分类标签。

    Best_Global_Delay 从 train set 统计得出：
    对每个 delay D ∈ {D0, D5, D10, D15}，计算仅使用该 delay 的
    train set silo-based calendar_sum，选 argmax。

    标签规则：
    - IF event 在 Best_Global_Delay 下 pnl_pct > 0 → enter
    - ELSE → skip

    Parameters
    ----------
    matrix : pd.DataFrame
        delay_pnl_matrix（580行）。
    train_events : pd.DataFrame
        训练集 events。
    all_events : pd.DataFrame
        全部 116 events。

    Returns
    -------
    tuple[pd.Series, str]
        - labels: index 与 all_events 对齐，值为 "enter" / "skip"
        - best_global_delay: 选出的最优全局 delay（如 "D0"）
    """
    # --- Step 1: 确定 train event_ids ---
    if "event_id" in train_events.columns:
        train_event_ids = set(train_events["event_id"].tolist())
    else:
        raise ValueError("train_events 必须包含 event_id 列")

    # --- Step 2: 对每个候选 delay 计算 train set silo-based calendar_sum ---
    # Silo = (symbol, month)，每个 silo 独立从 100k 开始
    _INITIAL_BALANCE = 100_000.0
    _NOTIONAL_SHARE = 0.26

    delay_calendar_sums: dict[str, float] = {}

    for delay in GLOBAL_DELAY_CANDIDATES:
        # 筛选 train events + 该 delay 的行
        mask = (
            (matrix["event_id"].isin(train_event_ids))
            & (matrix["delay_label"] == delay)
        )
        delay_rows = matrix[mask].copy()

        # 按 (symbol, month) 分组计算 silo-based calendar_sum
        delay_rows["entry_time_ts"] = pd.to_datetime(delay_rows["entry_time"], utc=True)
        delay_rows["silo_key"] = (
            delay_rows["symbol"]
            + "_"
            + delay_rows["entry_time_ts"].dt.strftime("%Y-%m")
        )

        total_return_pct = 0.0
        for _silo_key, silo_df in delay_rows.groupby("silo_key"):
            balance = _INITIAL_BALANCE
            # 按 entry_time 排序
            sorted_silo = silo_df.sort_values("entry_time_ts")
            for _, row in sorted_silo.iterrows():
                notional = balance * _NOTIONAL_SHARE
                pnl = notional * row["pnl_pct"]
                balance += pnl
            silo_return = (balance - _INITIAL_BALANCE) / _INITIAL_BALANCE * 100.0
            total_return_pct += silo_return

        delay_calendar_sums[delay] = total_return_pct

    # --- Step 3: 选 argmax 作为 Best_Global_Delay ---
    # 确定性：按 GLOBAL_DELAY_CANDIDATES 顺序，相同值取第一个
    best_global_delay = max(
        GLOBAL_DELAY_CANDIDATES,
        key=lambda d: delay_calendar_sums[d],
    )

    # --- Step 4: 为所有 events 分配标签 ---
    if "event_id" in all_events.columns:
        event_ids = all_events["event_id"].tolist()
    else:
        raise ValueError("all_events 必须包含 event_id 列")

    # 构建 event_id → pnl_pct 映射（在 Best_Global_Delay 下）
    best_delay_rows = matrix[matrix["delay_label"] == best_global_delay]
    pnl_map: dict[str, float] = {}
    for _, row in best_delay_rows.iterrows():
        pnl_map[row["event_id"]] = row["pnl_pct"]

    labels: list[str] = []
    for eid in event_ids:
        pnl = pnl_map.get(eid, 0.0)
        if pnl > 0:
            labels.append("enter")
        else:
            labels.append("skip")

    result = pd.Series(labels, index=all_events.index, name="regime_2_label")
    return result, best_global_delay


def compute_label_distributions(
    labels_5regime: pd.Series,
    labels_3regime: pd.Series,
    labels_2regime: pd.Series,
    train_mask: pd.Series,
) -> tuple[pd.DataFrame, bool]:
    """产出三种 regime 体系的标签分布对比。

    对每种 regime schema × 每个 split (train/test)，统计各标签的 count 和占比。
    同时检查 2-regime 的 enter 标签在 train set 上是否失衡。

    Parameters
    ----------
    labels_5regime : pd.Series
        5-regime 标签（值为 D0/D5/D10/D15/pullback/skip 等）。
    labels_3regime : pd.Series
        3-regime 标签（值为 skip/fast/slow）。
    labels_2regime : pd.Series
        2-regime 标签（值为 enter/skip）。
    train_mask : pd.Series
        布尔 Series，True 表示该 event 属于 train set。

    Returns
    -------
    tuple[pd.DataFrame, bool]
        - DataFrame with columns: regime_schema, label, split, count, pct
        - regime2_imbalanced: True 当 2-regime 的 enter 在 train set 占比 <40% 或 >90%
    """
    rows: list[dict] = []

    schema_map: dict[str, pd.Series] = {
        "5-regime": labels_5regime,
        "3-regime": labels_3regime,
        "2-regime": labels_2regime,
    }

    for schema_name, labels in schema_map.items():
        for split_name, mask in [("train", train_mask), ("test", ~train_mask)]:
            split_labels = labels[mask]
            total = len(split_labels)
            if total == 0:
                continue
            counts = split_labels.value_counts()
            for label_val, count in counts.items():
                rows.append({
                    "regime_schema": schema_name,
                    "label": label_val,
                    "split": split_name,
                    "count": int(count),
                    "pct": round(count / total * 100, 2),
                })

    dist_df = pd.DataFrame(rows, columns=["regime_schema", "label", "split", "count", "pct"])

    # 检查 2-regime 类别失衡
    regime2_imbalanced = False
    train_2regime = labels_2regime[train_mask]
    total_train = len(train_2regime)
    if total_train > 0:
        enter_count = (train_2regime == "enter").sum()
        enter_pct = enter_count / total_train
        if enter_pct < 0.40 or enter_pct > 0.90:
            regime2_imbalanced = True

    return dist_df, regime2_imbalanced


def generate_3regime_labels(
    matrix: pd.DataFrame,
    events: pd.DataFrame,
    tolerance_bps: float = 5.0,
) -> pd.Series:
    """基于 Delay_PnL_Matrix 生成三分类标签。

    规则：
    - fast_pnl = max(D0_pnl, D5_pnl)
    - slow_pnl = max(D10_pnl, D15_pnl, pullback_pnl)
    - IF fast_pnl < 0 AND slow_pnl < 0 → skip
    - ELSE IF fast_pnl >= slow_pnl → fast（差距 < 5bps 时优先 fast）
    - ELSE IF slow_pnl - fast_pnl < tolerance (5bps) → fast（优先 fast）
    - ELSE → slow

    Parameters
    ----------
    matrix : pd.DataFrame
        delay_pnl_matrix（580行），必须包含 event_id, delay_label, pnl_pct 列。
    events : pd.DataFrame
        116 events（用于对齐 index）。
    tolerance_bps : float
        容差阈值（basis points），默认 5.0。

    Returns
    -------
    pd.Series
        index 与 events 对齐，值为 "skip" / "fast" / "slow"。
    """
    tolerance = tolerance_bps / 10000.0  # 转换为小数（5bps = 0.0005）

    # 按 event_id 分组，提取各 delay 的 pnl_pct
    labels: list[str] = []

    # 获取 events 中的 event_id 列表，保持顺序
    if "event_id" in events.columns:
        event_ids = events["event_id"].tolist()
    else:
        # 如果 events 没有 event_id 列，使用 matrix 中的唯一 event_id（保持顺序）
        event_ids = matrix["event_id"].unique().tolist()

    for eid in event_ids:
        event_rows = matrix[matrix["event_id"] == eid]

        # 提取各 delay 的 pnl_pct
        pnl_by_delay: dict[str, float] = {}
        for _, row in event_rows.iterrows():
            pnl_by_delay[row["delay_label"]] = row["pnl_pct"]

        # 计算 fast_pnl 和 slow_pnl
        fast_pnls = [pnl_by_delay.get(d, np.nan) for d in FAST_DELAYS]
        slow_pnls = [pnl_by_delay.get(d, np.nan) for d in SLOW_DELAYS]

        fast_pnl = float(np.nanmax(fast_pnls)) if any(not np.isnan(v) for v in fast_pnls) else 0.0
        slow_pnl = float(np.nanmax(slow_pnls)) if any(not np.isnan(v) for v in slow_pnls) else 0.0

        # 标签分配逻辑
        if fast_pnl < 0 and slow_pnl < 0:
            labels.append("skip")
        elif fast_pnl >= slow_pnl:
            # fast_pnl >= slow_pnl → fast（包含相等情况，优先 fast）
            labels.append("fast")
        elif (slow_pnl - fast_pnl) < tolerance:
            # 差距 < 5bps 时优先 fast
            labels.append("fast")
        else:
            labels.append("slow")

    # 构建 Series，index 与 events 对齐
    result = pd.Series(labels, index=events.index, name="regime_3_label")
    return result
