"""Unit tests for OosArchiveAvailabilityWriter.

确定性 fixture 覆盖三种分支:
  1) archive 全在 → 返回 None，不创建文件
  2) 部分缺失 → 返回路径，JSON 内容正确
  3) 全缺失 → 返回路径，missing_symbol_months 列出全部 34 项

Requirements: 5.6
"""

from __future__ import annotations

import json
import pathlib
import tempfile

import pytest

from research.entry_redesign.oos.oos_archive_availability import (
    OOS_SYMBOLS,
    OOS_YEAR_MONTHS,
    OosArchiveAvailabilityWriter,
)


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

_CANDIDATE_ID = "d0_h0_none_market_on_touch_none_none-abcdef012345"
_QUERY_TS = "2025-06-15T12:00:00.000Z"


def _create_archive_dirs(
    archive_root: pathlib.Path,
    symbols: list[str],
    year_months: list[str],
) -> None:
    """在 archive_root 下为指定 (symbol, year_month) 创建非空目录。"""
    for symbol in symbols:
        for ym in year_months:
            d = archive_root / symbol / ym
            d.mkdir(parents=True, exist_ok=True)
            # 放一个占位文件使目录非空
            (d / "data.parquet").write_text("placeholder")


# ---------------------------------------------------------------------------
# Test 1: All archives present → returns None, no file created
# ---------------------------------------------------------------------------


def test_all_archives_present_returns_none() -> None:
    """当所有 (symbol, year_month) archive 均存在时，返回 None 且不创建文件。"""
    with tempfile.TemporaryDirectory() as tmpdir:
        archive_root = pathlib.Path(tmpdir) / "archives"
        output_dir = pathlib.Path(tmpdir) / "output"
        output_dir.mkdir()

        # 创建全部 archive 目录
        _create_archive_dirs(archive_root, OOS_SYMBOLS, OOS_YEAR_MONTHS)

        writer = OosArchiveAvailabilityWriter(output_dir=output_dir)
        result = writer.check_and_write(
            candidate_id=_CANDIDATE_ID,
            archive_root=str(archive_root),
            query_timestamp_utc=_QUERY_TS,
        )

        # 返回 None
        assert result is None

        # 输出目录下不应有 oos_archive_availability.json 文件
        expected_filename = (
            f"tmp_entry_redesign_{_CANDIDATE_ID}_oos_archive_availability.json"
        )
        assert not (output_dir / expected_filename).exists()


# ---------------------------------------------------------------------------
# Test 2: Partial missing → returns path, JSON correct
# ---------------------------------------------------------------------------


def test_partial_missing_returns_path_with_correct_json() -> None:
    """部分 archive 缺失时，返回文件路径且 JSON 内容正确。"""
    with tempfile.TemporaryDirectory() as tmpdir:
        archive_root = pathlib.Path(tmpdir) / "archives"
        output_dir = pathlib.Path(tmpdir) / "output"
        output_dir.mkdir()

        # 只创建 BTCUSDT 的全部月份，ETHUSDT 只创建前 5 个月
        _create_archive_dirs(archive_root, ["BTCUSDT"], OOS_YEAR_MONTHS)
        _create_archive_dirs(archive_root, ["ETHUSDT"], OOS_YEAR_MONTHS[:5])

        writer = OosArchiveAvailabilityWriter(output_dir=output_dir)
        result = writer.check_and_write(
            candidate_id=_CANDIDATE_ID,
            archive_root=str(archive_root),
            query_timestamp_utc=_QUERY_TS,
        )

        # 返回非 None 路径
        assert result is not None
        assert result.exists()

        # 读取 JSON 并验证结构
        data = json.loads(result.read_text(encoding="utf-8"))

        # 必须包含所有必需字段
        assert "query_command" in data
        assert "query_timestamp_utc" in data
        assert "archive_root" in data
        assert "missing_symbol_months" in data
        assert "evidence_snippet" in data

        # query_timestamp_utc 应与传入值一致
        assert data["query_timestamp_utc"] == _QUERY_TS

        # archive_root 应与传入值一致
        assert data["archive_root"] == str(archive_root)

        # missing_symbol_months 应只包含 ETHUSDT 缺失的 12 个月
        missing = data["missing_symbol_months"]
        expected_missing_count = len(OOS_YEAR_MONTHS) - 5  # 17 - 5 = 12
        assert len(missing) == expected_missing_count

        # 所有缺失项应为 ETHUSDT
        for item in missing:
            assert item["symbol"] == "ETHUSDT"
            assert "year_month" in item

        # 缺失的 year_month 应为 OOS_YEAR_MONTHS[5:]
        missing_yms = sorted(item["year_month"] for item in missing)
        expected_yms = sorted(OOS_YEAR_MONTHS[5:])
        assert missing_yms == expected_yms


# ---------------------------------------------------------------------------
# Test 3: All missing → returns path, all 34 listed
# ---------------------------------------------------------------------------


def test_all_missing_returns_path_with_all_34_listed() -> None:
    """全部 archive 缺失时，返回文件路径且 missing_symbol_months 列出全部 34 项。"""
    with tempfile.TemporaryDirectory() as tmpdir:
        archive_root = pathlib.Path(tmpdir) / "archives"
        output_dir = pathlib.Path(tmpdir) / "output"
        output_dir.mkdir()

        # 不创建任何 archive 目录（archive_root 本身也不存在）

        writer = OosArchiveAvailabilityWriter(output_dir=output_dir)
        result = writer.check_and_write(
            candidate_id=_CANDIDATE_ID,
            archive_root=str(archive_root),
            query_timestamp_utc=_QUERY_TS,
        )

        # 返回非 None 路径
        assert result is not None
        assert result.exists()

        # 读取 JSON
        data = json.loads(result.read_text(encoding="utf-8"))

        # missing_symbol_months 应包含全部 2 symbols × 17 months = 34 项
        missing = data["missing_symbol_months"]
        expected_total = len(OOS_SYMBOLS) * len(OOS_YEAR_MONTHS)  # 2 × 17 = 34
        assert len(missing) == expected_total

        # 验证所有 symbol 和 year_month 组合都在
        missing_pairs = {(item["symbol"], item["year_month"]) for item in missing}
        expected_pairs = {
            (symbol, ym) for symbol in OOS_SYMBOLS for ym in OOS_YEAR_MONTHS
        }
        assert missing_pairs == expected_pairs

        # 验证 JSON 字段完整性
        assert data["query_command"]  # 非空字符串
        assert data["query_timestamp_utc"] == _QUERY_TS
        assert data["archive_root"] == str(archive_root)
        assert "evidence_snippet" in data
        assert data["evidence_snippet"]  # 非空字符串

        # 验证文件使用 UTF-8 编码、LF 换行
        raw_bytes = result.read_bytes()
        assert b"\r\n" not in raw_bytes  # 无 CRLF
        assert raw_bytes.endswith(b"\n")  # LF 结尾
        # 无 BOM
        assert not raw_bytes.startswith(b"\xef\xbb\xbf")
