"""Tests for WalkForwardDriver.check_data_availability abort path.

验证 train/validation 历史缺失时正确抛出
InsufficientWalkforwardHistoryError，调用方可据此调用
RunnerAbortedWriter 写 abort_reason="insufficient_walkforward_history"。

禁止静默缩小 train/validation 窗口或回填合成数据。

Requirements: 4.2, 4.7
"""

from __future__ import annotations

import pathlib
import tempfile
from datetime import date

import pytest

from research.entry_redesign.walkforward.walkforward_driver import (
    InsufficientWalkforwardHistoryError,
    WalkForwardDriver,
    WalkForwardSplit,
)


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------


@pytest.fixture
def driver() -> WalkForwardDriver:
    """创建 WalkForwardDriver 实例。"""
    return WalkForwardDriver()


@pytest.fixture
def first_split() -> WalkForwardSplit:
    """execute=2025-06 的 split，需要 2025-03 ~ 2025-05 数据。"""
    return WalkForwardSplit(
        train_start=date(2025, 3, 1),
        train_end=date(2025, 5, 1),
        validation_start=date(2025, 5, 1),
        validation_end=date(2025, 6, 1),
        execute_start=date(2025, 6, 1),
        execute_end=date(2025, 7, 1),
    )


@pytest.fixture
def later_split() -> WalkForwardSplit:
    """execute=2025-09 的 split，需要 2025-06 ~ 2025-08 数据。"""
    return WalkForwardSplit(
        train_start=date(2025, 6, 1),
        train_end=date(2025, 8, 1),
        validation_start=date(2025, 8, 1),
        validation_end=date(2025, 9, 1),
        execute_start=date(2025, 9, 1),
        execute_end=date(2025, 10, 1),
    )


def _write_events_csv(path: pathlib.Path, months: list[str]) -> None:
    """写入包含指定月份数据的 events CSV fixture。"""
    lines = ["signal_bar_start_ts,symbol,side,prev_high_2,prev_low_2\n"]
    for month in months:
        lines.append(
            f"{month}-15T12:00:00.000Z,BTCUSDT,long,100000.0,99000.0\n"
        )
    path.write_text("".join(lines), encoding="utf-8")


# ---------------------------------------------------------------------------
# Tests: 文件不存在
# ---------------------------------------------------------------------------


class TestFileNotExists:
    """events_source_path 文件不存在时应抛出异常。"""

    def test_nonexistent_file_raises(
        self, driver: WalkForwardDriver, first_split: WalkForwardSplit
    ) -> None:
        """文件不存在 → InsufficientWalkforwardHistoryError。"""
        with pytest.raises(InsufficientWalkforwardHistoryError) as exc_info:
            driver.check_data_availability(
                first_split, "/nonexistent/path/events.csv"
            )
        assert exc_info.value.split == first_split
        assert len(exc_info.value.missing_months) > 0
        assert "2025-03" in exc_info.value.missing_months


# ---------------------------------------------------------------------------
# Tests: 文件为空
# ---------------------------------------------------------------------------


class TestEmptyFile:
    """events_source_path 文件为空时应抛出异常。"""

    def test_empty_file_raises(
        self, driver: WalkForwardDriver, first_split: WalkForwardSplit
    ) -> None:
        """空文件 → InsufficientWalkforwardHistoryError。"""
        with tempfile.NamedTemporaryFile(
            mode="w", suffix=".csv", delete=False
        ) as f:
            f.write("")
            tmp_path = f.name

        try:
            with pytest.raises(InsufficientWalkforwardHistoryError):
                driver.check_data_availability(first_split, tmp_path)
        finally:
            pathlib.Path(tmp_path).unlink(missing_ok=True)


# ---------------------------------------------------------------------------
# Tests: 数据部分缺失
# ---------------------------------------------------------------------------


class TestPartialDataMissing:
    """train/validation 窗口内部分月份数据缺失。"""

    def test_missing_train_month_raises(
        self, driver: WalkForwardDriver, first_split: WalkForwardSplit
    ) -> None:
        """execute=2025-06，只有 2025-04 和 2025-05 数据（缺 2025-03）。"""
        with tempfile.TemporaryDirectory() as tmpdir:
            csv_path = pathlib.Path(tmpdir) / "events.csv"
            # 只写 2025-04 和 2025-05，缺少 2025-03
            _write_events_csv(csv_path, ["2025-04", "2025-05"])

            with pytest.raises(
                InsufficientWalkforwardHistoryError
            ) as exc_info:
                driver.check_data_availability(first_split, str(csv_path))

            assert "2025-03" in exc_info.value.missing_months
            assert "2025-04" not in exc_info.value.missing_months
            assert "2025-05" not in exc_info.value.missing_months

    def test_missing_validation_month_raises(
        self, driver: WalkForwardDriver, first_split: WalkForwardSplit
    ) -> None:
        """execute=2025-06，只有 2025-03 和 2025-04 数据（缺 2025-05）。"""
        with tempfile.TemporaryDirectory() as tmpdir:
            csv_path = pathlib.Path(tmpdir) / "events.csv"
            # 只写 2025-03 和 2025-04，缺少 2025-05
            _write_events_csv(csv_path, ["2025-03", "2025-04"])

            with pytest.raises(
                InsufficientWalkforwardHistoryError
            ) as exc_info:
                driver.check_data_availability(first_split, str(csv_path))

            assert "2025-05" in exc_info.value.missing_months

    def test_all_train_validation_missing_raises(
        self, driver: WalkForwardDriver, first_split: WalkForwardSplit
    ) -> None:
        """execute=2025-06，数据只有 2025-06 以后（全部 train/val 缺失）。"""
        with tempfile.TemporaryDirectory() as tmpdir:
            csv_path = pathlib.Path(tmpdir) / "events.csv"
            # 只写 execute 月数据
            _write_events_csv(csv_path, ["2025-06", "2025-07"])

            with pytest.raises(
                InsufficientWalkforwardHistoryError
            ) as exc_info:
                driver.check_data_availability(first_split, str(csv_path))

            assert "2025-03" in exc_info.value.missing_months
            assert "2025-04" in exc_info.value.missing_months
            assert "2025-05" in exc_info.value.missing_months


# ---------------------------------------------------------------------------
# Tests: 数据完整 — 不应抛出异常
# ---------------------------------------------------------------------------


class TestDataComplete:
    """train/validation 窗口数据完整时不应抛出异常。"""

    def test_all_months_present_passes(
        self, driver: WalkForwardDriver, first_split: WalkForwardSplit
    ) -> None:
        """execute=2025-06，2025-03 ~ 2025-05 数据完整 → 正常通过。"""
        with tempfile.TemporaryDirectory() as tmpdir:
            csv_path = pathlib.Path(tmpdir) / "events.csv"
            _write_events_csv(
                csv_path, ["2025-03", "2025-04", "2025-05", "2025-06"]
            )

            # 不应抛出异常
            driver.check_data_availability(first_split, str(csv_path))

    def test_later_split_with_complete_data_passes(
        self, driver: WalkForwardDriver, later_split: WalkForwardSplit
    ) -> None:
        """execute=2025-09，2025-06 ~ 2025-08 数据完整 → 正常通过。"""
        with tempfile.TemporaryDirectory() as tmpdir:
            csv_path = pathlib.Path(tmpdir) / "events.csv"
            _write_events_csv(
                csv_path,
                ["2025-06", "2025-07", "2025-08", "2025-09"],
            )

            # 不应抛出异常
            driver.check_data_availability(later_split, str(csv_path))

    def test_extra_months_present_still_passes(
        self, driver: WalkForwardDriver, first_split: WalkForwardSplit
    ) -> None:
        """数据包含额外月份（超出所需范围）→ 仍然正常通过。"""
        with tempfile.TemporaryDirectory() as tmpdir:
            csv_path = pathlib.Path(tmpdir) / "events.csv"
            _write_events_csv(
                csv_path,
                [
                    "2025-01", "2025-02", "2025-03", "2025-04",
                    "2025-05", "2025-06", "2025-07",
                ],
            )

            # 不应抛出异常
            driver.check_data_availability(first_split, str(csv_path))


# ---------------------------------------------------------------------------
# Tests: 异常属性
# ---------------------------------------------------------------------------


class TestExceptionAttributes:
    """验证 InsufficientWalkforwardHistoryError 的属性。"""

    def test_exception_carries_split(
        self, driver: WalkForwardDriver, first_split: WalkForwardSplit
    ) -> None:
        """异常应携带触发它的 split。"""
        with pytest.raises(InsufficientWalkforwardHistoryError) as exc_info:
            driver.check_data_availability(
                first_split, "/nonexistent/events.csv"
            )
        assert exc_info.value.split is first_split

    def test_exception_carries_events_source_path(
        self, driver: WalkForwardDriver, first_split: WalkForwardSplit
    ) -> None:
        """异常应携带 events_source_path。"""
        path = "/some/path/events.csv"
        with pytest.raises(InsufficientWalkforwardHistoryError) as exc_info:
            driver.check_data_availability(first_split, path)
        assert exc_info.value.events_source_path == path

    def test_exception_message_mentions_no_shrink(
        self, driver: WalkForwardDriver, first_split: WalkForwardSplit
    ) -> None:
        """异常消息应明确禁止静默缩小窗口或回填。"""
        with pytest.raises(InsufficientWalkforwardHistoryError) as exc_info:
            driver.check_data_availability(
                first_split, "/nonexistent/events.csv"
            )
        msg = str(exc_info.value)
        assert "MUST NOT" in msg
        assert "shrink" in msg or "backfill" in msg


# ---------------------------------------------------------------------------
# Tests: 与 RunnerAbortedWriter 集成
# ---------------------------------------------------------------------------


class TestAbortWriterIntegration:
    """验证异常可被正确捕获并用于调用 RunnerAbortedWriter。"""

    def test_abort_writer_receives_correct_reason(
        self, driver: WalkForwardDriver, first_split: WalkForwardSplit
    ) -> None:
        """捕获异常后可构造正确的 abort_reason。"""
        from research.entry_redesign.snapshot.runner_aborted_writer import (
            RunnerAbortedWriter,
        )

        with tempfile.TemporaryDirectory() as tmpdir:
            output_dir = pathlib.Path(tmpdir)
            writer = RunnerAbortedWriter(output_dir)

            try:
                driver.check_data_availability(
                    first_split, "/nonexistent/events.csv"
                )
                pytest.fail("Should have raised InsufficientWalkforwardHistoryError")
            except InsufficientWalkforwardHistoryError:
                # 模拟调用方的 abort 路径
                result_path = writer.write(
                    candidate_id="d0_h0_none_market_on_touch_none_none-abcdef012345",
                    abort_reason="insufficient_walkforward_history",
                    mismatched_fields=[],
                    aborted_at_utc_ms="2025-06-01T00:00:00.000Z",
                )

            # 验证文件已写入
            assert result_path.exists()
            import json

            data = json.loads(result_path.read_text(encoding="utf-8"))
            assert data["abort_reason"] == "insufficient_walkforward_history"
            assert data["mismatched_fields"] == []


# ---------------------------------------------------------------------------
# Tests: iter_splits 产出的第一个 split 对应 execute=2025-06
# ---------------------------------------------------------------------------


class TestFirstSplitIsJune2025:
    """确认 iter_splits() 第一个 split 的 train/val 窗口为 2025-03~05。"""

    def test_first_split_train_validation_range(
        self, driver: WalkForwardDriver
    ) -> None:
        """第一个 split 的 train 从 2025-03，validation 到 2025-06。"""
        splits = list(driver.iter_splits())
        first = splits[0]
        assert first.train_start == date(2025, 3, 1)
        assert first.train_end == date(2025, 5, 1)
        assert first.validation_start == date(2025, 5, 1)
        assert first.validation_end == date(2025, 6, 1)
        assert first.execute_start == date(2025, 6, 1)
