"""MetricsAggregator — 13 指标聚合（Requirement 3.2）。

为每个 Entry_Candidate 产出以下 13 个指标，同时产出 `nogate_*` 与 `gate001_*`
两前缀（Requirement 3.3）。

指标定义（字段名与数学定义唯一，来自 design.md Cost Model & Metrics 章节）：

| # | 字段                                    | 数学定义                                                                 | null / 特殊值                                |
|---|----------------------------------------|-------------------------------------------------------------------------|---------------------------------------------|
| 1 | trade_count                            | ledger 中对应 gate_mode 下的 trade 行数                                   | —                                           |
| 2 | win_rate                               | count(realistic_pnl_i > 0) / trade_count                                | trade_count == 0 → null                     |
| 3 | payoff_ratio                           | mean(R_i | R_i > 0) / abs(mean(R_i | R_i < 0)), R_i=realistic_pnl_i/notional_i | 任一侧样本数 0 → null          |
| 4 | realistic_pnl_pct                      | sum(realistic_pnl_i / notional_i) × 100                                 | —                                           |
| 5 | realistic_taker_both_pct               | 同 4 但用 realistic_taker_both_pnl                                       | —                                           |
| 6 | raw_pnl_pct                            | 同 4 但用 raw_pnl                                                        | —                                           |
| 7 | per_trade_quality_bps_over_notional    | mean_i(realistic_pnl_i / notional_i) × 10000                            | trade_count == 0 → null                     |
| 8 | max_drawdown_pct                       | C_k = sum_{i<=k}(realistic_pnl_i/notional_i)×100 (entry_ts ASC);        | —                                           |
|   |                                        | MDD = max_k(max_{j<=k} C_j − C_k)                                       |                                             |
| 9 | profit_factor                          | sum(>0) / abs(sum(<0))                                                   | 分子 0 → null; 分母 0 & 分子>0 → "+inf"     |
|10 | active_silo_sum_pct                    | 有成交 silo 逐 silo realistic_pnl_pct 简单加和                            | —                                           |
|11 | calendar_normalized_return_pct         | 22 silo 逐 silo realistic_pnl_pct（空仓 silo 记 0.0）加和                 | —                                           |
|12 | active_months                          | 粒度 (symbol, execute_month), 值域 [0, 22]                               | —                                           |
|13 | empty_months                           | 同上，且 active_months + empty_months == 22                               | —                                           |

Requirements: 3.2, 3.3, 6.9, 6.12
"""

from __future__ import annotations

from typing import Any, Optional

import numpy as np
import pandas as pd


class MetricsAggregator:
    """聚合 Research_Ledger 产出 13 个指标。

    每个指标同时产出 `nogate_*` 与 `gate001_*` 两前缀（Requirement 3.3）。
    `trade_count == 0` 时 null 分支显式处理。

    Usage:
        aggregator = MetricsAggregator()
        result = aggregator.aggregate(ledger_df, total_silos=22)
        # result 包含 nogate_trade_count, nogate_win_rate, ...,
        #              gate001_trade_count, gate001_win_rate, ...
    """

    def aggregate(
        self,
        ledger: pd.DataFrame,
        total_silos: int = 22,
    ) -> dict[str, Any]:
        """聚合 ledger 产出 13 × 2 = 26 个指标字段。

        Args:
            ledger: Research_Ledger DataFrame，MUST 包含以下列：
                - gate_mode: "nogate" | "gate001"
                - realistic_pnl: float
                - realistic_taker_both_pnl: float
                - raw_pnl: float
                - notional: float
                - entry_ts: datetime（用于 max_drawdown_pct 排序）
                - symbol: str
                - signal_bar_start_ts: datetime（用于 execute_month 推导）
            total_silos: calendar silo 总数（默认 22 = 2 symbols × 11 execute_months）。

        Returns:
            dict: 包含 `nogate_*` 与 `gate001_*` 两前缀的 26 个指标字段。
        """
        result: dict[str, Any] = {}

        for gate_mode in ("nogate", "gate001"):
            prefix = f"{gate_mode}_"
            subset = ledger[ledger["gate_mode"] == gate_mode].copy()
            metrics = self._compute_metrics(subset, total_silos)
            for key, value in metrics.items():
                result[f"{prefix}{key}"] = value

        return result

    def _compute_metrics(
        self,
        df: pd.DataFrame,
        total_silos: int,
    ) -> dict[str, Any]:
        """对单个 gate_mode 子集计算 13 个指标。

        Args:
            df: 单个 gate_mode 的 ledger 子集。
            total_silos: calendar silo 总数。

        Returns:
            dict: 13 个指标字段。
        """
        trade_count = len(df)

        # --- trade_count == 0 时的 null 分支显式处理 ---
        if trade_count == 0:
            active_months = 0
            empty_months = total_silos
            return {
                "trade_count": 0,
                "win_rate": None,
                "payoff_ratio": None,
                "realistic_pnl_pct": 0.0,
                "realistic_taker_both_pct": 0.0,
                "raw_pnl_pct": 0.0,
                "per_trade_quality_bps_over_notional": None,
                "max_drawdown_pct": 0.0,
                "profit_factor": None,
                "active_silo_sum_pct": 0.0,
                "calendar_normalized_return_pct": 0.0,
                "active_months": active_months,
                "empty_months": empty_months,
            }

        # --- 基础向量计算 ---
        realistic_pnl = df["realistic_pnl"].values.astype(np.float64)
        realistic_taker_both_pnl = df["realistic_taker_both_pnl"].values.astype(
            np.float64
        )
        raw_pnl = df["raw_pnl"].values.astype(np.float64)
        notional = df["notional"].values.astype(np.float64)

        # R_i = realistic_pnl_i / notional_i
        r_i = realistic_pnl / notional

        # --- 1. trade_count ---
        # 已计算

        # --- 2. win_rate = count(realistic_pnl_i > 0) / trade_count ---
        win_count = int(np.sum(realistic_pnl > 0))
        win_rate: Optional[float] = win_count / trade_count

        # --- 3. payoff_ratio = mean(R_i | R_i > 0) / abs(mean(R_i | R_i < 0)) ---
        r_positive = r_i[r_i > 0]
        r_negative = r_i[r_i < 0]
        if len(r_positive) == 0 or len(r_negative) == 0:
            payoff_ratio: Any = None
        else:
            mean_positive = float(np.mean(r_positive))
            mean_negative = float(np.mean(r_negative))
            payoff_ratio = mean_positive / abs(mean_negative)

        # --- 4. realistic_pnl_pct = sum(realistic_pnl_i / notional_i) × 100 ---
        realistic_pnl_pct = float(np.sum(r_i)) * 100.0

        # --- 5. realistic_taker_both_pct = 同公式但用 realistic_taker_both_pnl ---
        r_taker_both_i = realistic_taker_both_pnl / notional
        realistic_taker_both_pct = float(np.sum(r_taker_both_i)) * 100.0

        # --- 6. raw_pnl_pct = 同公式但用 raw_pnl ---
        r_raw_i = raw_pnl / notional
        raw_pnl_pct = float(np.sum(r_raw_i)) * 100.0

        # --- 7. per_trade_quality_bps_over_notional = mean_i(R_i) × 10000 ---
        per_trade_quality_bps_over_notional: Optional[float] = (
            float(np.mean(r_i)) * 10000.0
        )

        # --- 8. max_drawdown_pct ---
        # C_k = sum_{i <= k} (realistic_pnl_i / notional_i) × 100 (entry_ts ASC)
        df_sorted = df.sort_values("entry_ts").reset_index(drop=True)
        r_i_sorted = (
            df_sorted["realistic_pnl"].values.astype(np.float64)
            / df_sorted["notional"].values.astype(np.float64)
        )
        cumulative = np.cumsum(r_i_sorted) * 100.0
        # MDD = max_k (max_{j <= k} C_j − C_k)
        running_max = np.maximum.accumulate(cumulative)
        drawdowns = running_max - cumulative
        max_drawdown_pct = float(np.max(drawdowns)) if len(drawdowns) > 0 else 0.0

        # --- 9. profit_factor = sum(realistic_pnl_i | >0) / abs(sum(realistic_pnl_i | <0)) ---
        sum_positive = float(np.sum(realistic_pnl[realistic_pnl > 0]))
        sum_negative = float(np.sum(realistic_pnl[realistic_pnl < 0]))
        if sum_positive == 0.0:
            # 分子 0 → null（无论分母如何）
            profit_factor: Any = None
        elif sum_negative == 0.0:
            # 分母 0 且分子 > 0 → "+inf"
            profit_factor = "+inf"
        else:
            profit_factor = sum_positive / abs(sum_negative)

        # --- 10, 11, 12, 13: silo 相关指标 ---
        # 推导 execute_month: 使用 signal_bar_start_ts 的年月
        df_with_month = df.copy()
        df_with_month["_execute_month"] = pd.to_datetime(
            df_with_month["signal_bar_start_ts"]
        ).dt.to_period("M")
        df_with_month["_silo_key"] = (
            df_with_month["symbol"].astype(str)
            + "_"
            + df_with_month["_execute_month"].astype(str)
        )

        # 逐 silo 计算 realistic_pnl_pct
        silo_metrics: dict[str, float] = {}
        for silo_key, silo_df in df_with_month.groupby("_silo_key"):
            silo_r_i = (
                silo_df["realistic_pnl"].values.astype(np.float64)
                / silo_df["notional"].values.astype(np.float64)
            )
            silo_metrics[str(silo_key)] = float(np.sum(silo_r_i)) * 100.0

        # --- 12. active_months ---
        active_months = len(silo_metrics)

        # --- 13. empty_months (active + empty == total_silos) ---
        empty_months = total_silos - active_months

        # --- 10. active_silo_sum_pct ---
        active_silo_sum_pct = sum(silo_metrics.values())

        # --- 11. calendar_normalized_return_pct ---
        # 空仓 silo 记 0.0，加和 = active_silo_sum_pct + 0.0 × empty_months
        calendar_normalized_return_pct = active_silo_sum_pct

        return {
            "trade_count": trade_count,
            "win_rate": win_rate,
            "payoff_ratio": payoff_ratio,
            "realistic_pnl_pct": realistic_pnl_pct,
            "realistic_taker_both_pct": realistic_taker_both_pct,
            "raw_pnl_pct": raw_pnl_pct,
            "per_trade_quality_bps_over_notional": per_trade_quality_bps_over_notional,
            "max_drawdown_pct": max_drawdown_pct,
            "profit_factor": profit_factor,
            "active_silo_sum_pct": active_silo_sum_pct,
            "calendar_normalized_return_pct": calendar_normalized_return_pct,
            "active_months": active_months,
            "empty_months": empty_months,
        }
