"""WalkForwardDriver — 固定宽度滑动窗口 walk-forward splitter。

train = 2 个月 / validation = 1 个月 / execute = 1 个月。
execute 月覆盖 2025-06 ~ 2026-04（共 11 个 execute 月）。
所有区间为半开 [start, end)；三窗口两两不相交；execute.start >= validation.end。

对于 execute=2025-06：
  train     = [2025-03-01, 2025-05-01)
  validation= [2025-05-01, 2025-06-01)
  execute   = [2025-06-01, 2025-07-01)

模式：对于每个 execute 月 M：
  train      = [M-3, M-1)
  validation = [M-1, M)
  execute    = [M, M+1)

walkforward_config 字段写入 summary JSON：
  train_months=2 / validation_months=1 / execute_months=1 /
  execute_start_year_month=2025-06 / execute_end_year_month=2026-04 /
  total_execute_months=11

IF execute=2025-06 时 train/validation 窗口 (2025-03 ~ 2025-05)
   所需数据缺失或记录不完整,
THEN 终止该 Entry_Candidate 运行并在 runner_aborted.json 写入
     abort_reason="insufficient_walkforward_history"；
     禁止静默缩小 train/validation 窗口或回填合成数据。

Requirements: 4.2, 4.3, 4.7, 6.6
"""

from __future__ import annotations

import pathlib
from dataclasses import dataclass
from datetime import date
from typing import Iterator


# ---------------------------------------------------------------------------
# WalkForwardSplit frozen dataclass
# ---------------------------------------------------------------------------


@dataclass(frozen=True)
class WalkForwardSplit:
    """单个 walk-forward split，所有日期字段为半开区间 [start, end)。

    Attributes:
        train_start: 训练窗口起始日期（含）。
        train_end: 训练窗口结束日期（不含）。
        validation_start: 验证窗口起始日期（含）。
        validation_end: 验证窗口结束日期（不含）。
        execute_start: 执行窗口起始日期（含）。
        execute_end: 执行窗口结束日期（不含）。
    """

    train_start: date
    train_end: date
    validation_start: date
    validation_end: date
    execute_start: date
    execute_end: date


# ---------------------------------------------------------------------------
# 异常
# ---------------------------------------------------------------------------


class InsufficientWalkforwardHistoryError(Exception):
    """train/validation 窗口所需历史数据缺失时抛出。

    调用方应捕获此异常并调用 RunnerAbortedWriter 写入
    abort_reason="insufficient_walkforward_history"。

    禁止静默缩小 train/validation 窗口或回填合成数据。

    Attributes:
        split: 触发异常的 WalkForwardSplit。
        missing_months: 缺失的年月列表，格式 "YYYY-MM"。
        events_source_path: 检查的事件数据源路径。
    """

    def __init__(
        self,
        split: WalkForwardSplit,
        missing_months: list[str],
        events_source_path: str,
    ) -> None:
        self.split = split
        self.missing_months = missing_months
        self.events_source_path = events_source_path
        super().__init__(
            f"Insufficient walkforward history for execute month "
            f"{split.execute_start.strftime('%Y-%m')}: "
            f"missing data for months {missing_months} "
            f"in events source '{events_source_path}'. "
            f"MUST NOT silently shrink train/validation window or backfill "
            f"synthetic data."
        )


# ---------------------------------------------------------------------------
# 常量
# ---------------------------------------------------------------------------

_TRAIN_MONTHS: int = 2
_VALIDATION_MONTHS: int = 1
_EXECUTE_MONTHS: int = 1

# execute 月覆盖范围：2025-06 ~ 2026-04（共 11 个月）
_EXECUTE_START_YEAR: int = 2025
_EXECUTE_START_MONTH: int = 6
_EXECUTE_END_YEAR: int = 2026
_EXECUTE_END_MONTH: int = 4
_TOTAL_EXECUTE_MONTHS: int = 11


# ---------------------------------------------------------------------------
# 辅助函数
# ---------------------------------------------------------------------------


def _add_months(d: date, months: int) -> date:
    """对日期的月份做加减运算，日固定为 1。

    Args:
        d: 输入日期（日部分会被忽略，始终返回该月 1 日）。
        months: 要增加的月数（可为负数）。

    Returns:
        新日期，日固定为 1。
    """
    # 将年月转为总月数进行算术运算
    total_months = d.year * 12 + (d.month - 1) + months
    new_year = total_months // 12
    new_month = total_months % 12 + 1
    return date(new_year, new_month, 1)


def _enumerate_months(start: date, end: date) -> list[str]:
    """枚举 [start, end) 半开区间内的所有月份，返回 "YYYY-MM" 格式列表。

    Args:
        start: 起始日期（含），日部分忽略。
        end: 结束日期（不含），日部分忽略。

    Returns:
        月份字符串列表，例如 ["2025-03", "2025-04", "2025-05"]。
    """
    months: list[str] = []
    current = date(start.year, start.month, 1)
    end_normalized = date(end.year, end.month, 1)
    while current < end_normalized:
        months.append(f"{current.year:04d}-{current.month:02d}")
        current = _add_months(current, 1)
    return months


def _get_covered_months_from_csv(source_path: pathlib.Path) -> set[str]:
    """从 events CSV 文件中提取已覆盖的月份集合。

    扫描 CSV 文件的时间戳列（signal_bar_start_ts），提取所有出现的
    年月组合。支持 ISO-8601 格式（如 "2025-06-01T00:00:00.000Z"）。

    如果文件无法解析或不包含时间戳列，返回空集合（视为数据缺失）。

    Args:
        source_path: CSV 文件路径。

    Returns:
        已覆盖月份的集合，格式 "YYYY-MM"。
    """
    covered: set[str] = set()
    try:
        with open(source_path, "r", encoding="utf-8") as f:
            # 读取 header 行确定时间戳列索引
            header_line = f.readline().strip()
            if not header_line:
                return covered
            headers = header_line.split(",")

            # 优先查找 signal_bar_start_ts 列
            ts_col_idx: int | None = None
            for candidate_col in ("signal_bar_start_ts", "entry_ts", "trigger_ts"):
                if candidate_col in headers:
                    ts_col_idx = headers.index(candidate_col)
                    break

            if ts_col_idx is None:
                # 无法确定时间戳列，视为数据缺失
                return covered

            # 扫描数据行提取月份
            for line in f:
                line = line.strip()
                if not line:
                    continue
                fields = line.split(",")
                if ts_col_idx >= len(fields):
                    continue
                ts_value = fields[ts_col_idx].strip()
                # 提取 YYYY-MM（支持 ISO-8601 和 YYYY-MM-DD 格式）
                if len(ts_value) >= 7:
                    year_month = ts_value[:7]
                    # 基本格式校验 YYYY-MM
                    if (
                        len(year_month) == 7
                        and year_month[4] == "-"
                        and year_month[:4].isdigit()
                        and year_month[5:7].isdigit()
                    ):
                        covered.add(year_month)
    except (OSError, UnicodeDecodeError):
        # 文件读取失败，视为数据缺失
        return set()

    return covered


# ---------------------------------------------------------------------------
# WalkForwardDriver
# ---------------------------------------------------------------------------


class WalkForwardDriver:
    """Walk-forward splitter。

    固定配置：train=2m / validation=1m / execute=1m。
    execute 月覆盖 2025-06 ~ 2026-04（共 11 个 execute 月）。
    三窗口两两不相交（P6）；execute.start >= validation.end。

    IF execute=2025-06 时 train/validation 窗口 (2025-03 ~ 2025-05)
       所需数据缺失或记录不完整,
    THEN 终止该 Entry_Candidate 运行并在 runner_aborted.json 写入
         abort_reason="insufficient_walkforward_history"；
         禁止静默缩小 train/validation 窗口或回填合成数据。
    """

    def iter_splits(self) -> Iterator[WalkForwardSplit]:
        """生成 11 个 WalkForwardSplit 对象。

        execute 月从 2025-06 到 2026-04（含），每月一个 split。
        对于每个 execute 月 M（M 的第 1 天）：
          - train      = [M - 3 months, M - 1 month)
          - validation = [M - 1 month,  M)
          - execute    = [M,            M + 1 month)

        Yields:
            WalkForwardSplit: 包含 train/validation/execute 三个半开区间。
        """
        execute_date = date(_EXECUTE_START_YEAR, _EXECUTE_START_MONTH, 1)
        end_date = _add_months(
            date(_EXECUTE_END_YEAR, _EXECUTE_END_MONTH, 1), 1
        )  # 2026-05-01，用于终止循环

        while execute_date < end_date:
            # execute = [M, M+1)
            execute_start = execute_date
            execute_end = _add_months(execute_date, _EXECUTE_MONTHS)

            # validation = [M-1, M)
            validation_start = _add_months(execute_date, -_VALIDATION_MONTHS)
            validation_end = execute_date

            # train = [M-3, M-1)  即 [M - train_months - validation_months, M - validation_months)
            train_start = _add_months(
                execute_date, -(_TRAIN_MONTHS + _VALIDATION_MONTHS)
            )
            train_end = validation_start

            yield WalkForwardSplit(
                train_start=train_start,
                train_end=train_end,
                validation_start=validation_start,
                validation_end=validation_end,
                execute_start=execute_start,
                execute_end=execute_end,
            )

            execute_date = _add_months(execute_date, 1)

    def get_walkforward_config(self) -> dict:
        """返回 walkforward_config 字典，写入 summary JSON。

        Returns:
            dict: 包含以下字段：
                - train_months: 2
                - validation_months: 1
                - execute_months: 1
                - execute_start_year_month: "2025-06"
                - execute_end_year_month: "2026-04"
                - total_execute_months: 11
        """
        return {
            "train_months": _TRAIN_MONTHS,
            "validation_months": _VALIDATION_MONTHS,
            "execute_months": _EXECUTE_MONTHS,
            "execute_start_year_month": f"{_EXECUTE_START_YEAR:04d}-{_EXECUTE_START_MONTH:02d}",
            "execute_end_year_month": f"{_EXECUTE_END_YEAR:04d}-{_EXECUTE_END_MONTH:02d}",
            "total_execute_months": _TOTAL_EXECUTE_MONTHS,
        }

    def check_data_availability(
        self,
        split: WalkForwardSplit,
        events_source_path: str,
    ) -> None:
        """验证给定 split 的 train/validation 窗口所需数据是否存在。

        对于 execute=2025-06，所需 train/validation 窗口为 2025-03 ~ 2025-05。
        本方法检查 events_source_path 指向的数据文件是否存在且非空。

        检查逻辑：
          1. events_source_path 文件必须存在且非空。
          2. 如果 events_source_path 是 CSV 文件，则读取其中的时间戳列
             以验证 train_start ~ validation_end 范围内有数据记录。
          3. 如果上述任一条件不满足，抛出
             InsufficientWalkforwardHistoryError。

        禁止静默缩小 train/validation 窗口或回填合成数据。

        Args:
            split: 要检查的 WalkForwardSplit。
            events_source_path: 事件数据源文件路径（CSV）。

        Raises:
            InsufficientWalkforwardHistoryError: 当 train/validation 窗口
                所需数据缺失或记录不完整时抛出。调用方应捕获此异常并调用
                RunnerAbortedWriter 写入
                abort_reason="insufficient_walkforward_history"。

        Requirements: 4.2, 4.7
        """
        source_path = pathlib.Path(events_source_path)

        # 计算 train/validation 覆盖的所有月份 [train_start, validation_end)
        required_months = _enumerate_months(split.train_start, split.validation_end)

        # 检查 1: 事件源文件必须存在且非空
        if not source_path.exists():
            raise InsufficientWalkforwardHistoryError(
                split=split,
                missing_months=required_months,
                events_source_path=events_source_path,
            )

        if source_path.stat().st_size == 0:
            raise InsufficientWalkforwardHistoryError(
                split=split,
                missing_months=required_months,
                events_source_path=events_source_path,
            )

        # 检查 2: 验证文件中包含 train/validation 窗口内的数据记录
        # 通过扫描 CSV 中的时间戳列确认覆盖范围
        covered_months = _get_covered_months_from_csv(source_path)

        missing = [m for m in required_months if m not in covered_months]
        if missing:
            raise InsufficientWalkforwardHistoryError(
                split=split,
                missing_months=missing,
                events_source_path=events_source_path,
            )
