"""Hypothesis property-based / 双跑测试：Entry Layer Bit-Identical Ledger (P7).

**Validates: Requirements 6.7, 6.14**

Property 7: 同 candidate_id + 同 seed + 同 events sha256 下两次独立运行，
比较 sha256(ledger.csv) 必须一致。

测试方法：
  1) 使用 Hypothesis 生成随机 TradeRecord 列表
  2) 使用 LedgerCsvWriter 写入两个不同的临时路径
  3) 计算两个文件的 sha256
  4) 断言两个 sha256 完全一致

这验证了 LedgerCsvWriter 的确定性：相同输入 → 字节级相同输出。
确定性保证来自：
  - 固定排序键 (trigger_ts ASC, symbol ASC, side ASC, entry_ts ASC)
  - 固定浮点格式化 (8 位定点十进制)
  - 固定时间戳格式化 (ISO-8601 UTC ms)
  - 单一 writer 入口（禁止多路径拼接）
  - 禁止 datetime.now() / os.getpid() / 未 seed 的随机源

Requirements: 6.7, 6.14
"""

from __future__ import annotations

import hashlib
import pathlib
import tempfile
from datetime import datetime, timezone

from hypothesis import given, settings
from hypothesis import strategies as st

from research.entry_redesign.ledger.ledger_csv_writer import (
    LedgerCsvWriter,
    TradeRecord,
)

# ---------------------------------------------------------------------------
# Hypothesis strategies: 生成随机 TradeRecord 实例
# ---------------------------------------------------------------------------

_SYMBOLS = ["BTCUSDT", "ETHUSDT"]
_SIDES: list[str] = ["long", "short"]
_EXIT_REASONS: list[str] = [
    "signal_exit",
    "initial_stop",
    "breakeven_stop",
    "trail_stop",
    "max_hold_timeout",
    "gate_rejected",
    "runner_aborted",
]
_GATE_MODES: list[str] = ["nogate", "gate001"]
_TRIGGER_CONFIRMATION_IDS: list[str] = [
    "none",
    "persistence_n1",
    "persistence_n3",
    "persistence_n5",
    "persistence_n10",
    "retest_tb0",
    "retest_tb1",
    "retest_tb2",
    "minvol_bps50",
    "minvol_bps100",
    "minvol_bps200",
]
_ENTRY_PRICE_MODE_IDS: list[str] = [
    "market_on_touch",
    "limit_at_level",
    "limit_tb_k0",
    "limit_tb_k1",
    "limit_tb_k2",
    "limit_tb_k4",
    "pullback_p000",
    "pullback_p002",
    "pullback_p005",
    "pullback_p010",
]
_PRETOUCH_STATE_BAND_IDS: list[str] = ["none", "fast_clean", "fast_clean_strict"]
_POSTTOUCH_QUALITY_BAND_IDS: list[str] = [
    "none",
    "cont1s_r003",
    "cont1s_r005",
    "cont1s_r008",
    "tickimb_b055",
    "tickimb_b060",
    "tickimb_b065",
    "spread_s1",
    "spread_s2",
    "spread_s4",
]
_VALID_D = [0, 5, 15, 30, 60, 120]
_VALID_H = [0, 5, 15, 30, 60]


# 生成 UTC datetime（秒级精度，范围 2025-01 ~ 2026-12）
_utc_datetimes = st.integers(
    min_value=1735689600,  # 2025-01-01 00:00:00 UTC
    max_value=1798761600,  # 2026-12-31 00:00:00 UTC
).map(lambda ts: datetime.fromtimestamp(ts, tz=timezone.utc))


# candidate_id 满足正则 ^[a-z0-9]+(?:_[a-z0-9]+)*-[0-9a-f]{12}$
_candidate_ids = st.from_regex(
    r"^[a-z0-9]+(?:_[a-z0-9]+)*-[0-9a-f]{12}$",
    fullmatch=True,
).filter(lambda s: 14 <= len(s) <= 64)


@st.composite
def trade_records(draw: st.DrawFn) -> TradeRecord:
    """Generate a random TradeRecord instance with valid field values."""
    # 生成四个时间戳，确保逻辑顺序：signal_bar_start <= trigger <= entry <= exit
    ts_base = draw(
        st.integers(min_value=1735689600, max_value=1798700000)
    )
    signal_bar_start_ts = datetime.fromtimestamp(ts_base, tz=timezone.utc)
    trigger_ts = datetime.fromtimestamp(
        ts_base + draw(st.integers(min_value=0, max_value=3600)),
        tz=timezone.utc,
    )
    entry_ts = datetime.fromtimestamp(
        ts_base + draw(st.integers(min_value=0, max_value=7200)),
        tz=timezone.utc,
    )
    exit_ts = datetime.fromtimestamp(
        ts_base + draw(st.integers(min_value=1, max_value=14400)),
        tz=timezone.utc,
    )

    # 浮点字段：正有限值
    entry_price = draw(
        st.floats(min_value=0.01, max_value=1e8, allow_nan=False, allow_infinity=False)
    )
    exit_price = draw(
        st.floats(min_value=0.01, max_value=1e8, allow_nan=False, allow_infinity=False)
    )
    notional = draw(
        st.floats(min_value=0.01, max_value=1e8, allow_nan=False, allow_infinity=False)
    )
    raw_pnl = draw(
        st.floats(
            min_value=-1e8, max_value=1e8, allow_nan=False, allow_infinity=False
        )
    )
    slip_pnl = draw(
        st.floats(
            min_value=-1e8, max_value=1e8, allow_nan=False, allow_infinity=False
        )
    )
    realistic_pnl = draw(
        st.floats(
            min_value=-1e8, max_value=1e8, allow_nan=False, allow_infinity=False
        )
    )
    realistic_taker_both_pnl = draw(
        st.floats(
            min_value=-1e8, max_value=1e8, allow_nan=False, allow_infinity=False
        )
    )

    # 枚举字段
    symbol = draw(st.sampled_from(_SYMBOLS))
    side = draw(st.sampled_from(_SIDES))
    exit_reason = draw(st.sampled_from(_EXIT_REASONS))
    gate_mode = draw(st.sampled_from(_GATE_MODES))
    trigger_confirmation_id = draw(st.sampled_from(_TRIGGER_CONFIRMATION_IDS))
    entry_price_mode_id = draw(st.sampled_from(_ENTRY_PRICE_MODE_IDS))
    pretouch_state_band_id = draw(st.sampled_from(_PRETOUCH_STATE_BAND_IDS))
    posttouch_quality_band_id = draw(st.sampled_from(_POSTTOUCH_QUALITY_BAND_IDS))

    # D 和 H 需满足 H <= D
    d = draw(st.sampled_from(_VALID_D))
    valid_h = [h for h in _VALID_H if h <= d]
    h = draw(st.sampled_from(valid_h))

    # candidate_id: 使用简单确定性格式
    candidate_id = draw(_candidate_ids)

    return TradeRecord(
        entry_ts=entry_ts,
        exit_ts=exit_ts,
        symbol=symbol,
        side=side,
        entry_price=entry_price,
        exit_price=exit_price,
        notional=notional,
        raw_pnl=raw_pnl,
        slip_pnl=slip_pnl,
        realistic_pnl=realistic_pnl,
        realistic_taker_both_pnl=realistic_taker_both_pnl,
        exit_reason=exit_reason,
        entry_candidate_id=candidate_id,
        gate_mode=gate_mode,
        signal_bar_start_ts=signal_bar_start_ts,
        trigger_ts=trigger_ts,
        entry_delay_seconds=d,
        feature_horizon_seconds=h,
        trigger_confirmation_id=trigger_confirmation_id,
        entry_price_mode_id=entry_price_mode_id,
        pretouch_state_band_id=pretouch_state_band_id,
        posttouch_quality_band_id=posttouch_quality_band_id,
    )


def _sha256_file(path: pathlib.Path) -> str:
    """Compute sha256 hex digest of a file."""
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(8192), b""):
            h.update(chunk)
    return h.hexdigest()


# ---------------------------------------------------------------------------
# Property-based test: P7 Bit-Identical Ledger
# ---------------------------------------------------------------------------


@given(trades=st.lists(trade_records(), min_size=0, max_size=50))
@settings(max_examples=200, deadline=None)
def test_p7_bit_identical_ledger(trades: list[TradeRecord]) -> None:
    """P7: Entry Layer Bit-Identical Ledger.

    **Validates: Requirements 6.7**

    同 candidate_id + 同 seed + 同 events sha256 下两次独立运行，
    比较 sha256(ledger.csv) 必须一致。

    测试方法：对同一组 TradeRecord 列表，使用两个独立的 LedgerCsvWriter
    实例分别写入两个不同的临时文件，然后比较两个文件的 sha256。
    """
    with tempfile.TemporaryDirectory() as tmpdir:
        path_a = pathlib.Path(tmpdir) / "run_a" / "ledger.csv"
        path_b = pathlib.Path(tmpdir) / "run_b" / "ledger.csv"

        # 两次独立运行：使用两个独立的 LedgerCsvWriter 实例
        writer_a = LedgerCsvWriter()
        writer_b = LedgerCsvWriter()

        writer_a.write(trades, path_a)
        writer_b.write(trades, path_b)

        # 计算 sha256
        sha256_a = _sha256_file(path_a)
        sha256_b = _sha256_file(path_b)

        # 断言字节级一致
        assert sha256_a == sha256_b, (
            f"P7 violated: Bit-Identical Ledger 不变量失败。\n"
            f"  run_a sha256: {sha256_a}\n"
            f"  run_b sha256: {sha256_b}\n"
            f"  trade_count: {len(trades)}\n"
            f"两次独立运行产出的 ledger CSV sha256 不一致，"
            f"违反 Requirement 6.7 (Bit_Identical_Ledger)。"
        )


@given(trades=st.lists(trade_records(), min_size=1, max_size=30))
@settings(max_examples=100, deadline=None)
def test_p7_shuffled_input_unique_keys_same_output(
    trades: list[TradeRecord],
) -> None:
    """P7 补充：输入顺序不同但排序键唯一时，输出 sha256 仍一致。

    **Validates: Requirements 6.7**

    当所有 trades 的排序键 (trigger_ts, symbol, side, entry_ts) 唯一时，
    LedgerCsvWriter 内部排序保证：即使输入 trades 顺序不同，
    输出文件的 sha256 仍然一致。

    注意：当排序键存在重复时，stable sort 保持输入顺序，
    因此只有排序键唯一的情况下才能保证不同输入顺序产出相同输出。
    P7 的核心保证是"同一输入 → 同一输出"，此测试验证排序的确定性。
    """
    # 去重：只保留排序键唯一的 trades
    seen_keys: set[tuple] = set()
    unique_trades: list[TradeRecord] = []
    for t in trades:
        key = (t.trigger_ts, t.symbol, t.side, t.entry_ts)
        if key not in seen_keys:
            seen_keys.add(key)
            unique_trades.append(t)

    if len(unique_trades) < 2:
        # 不足 2 条唯一记录，跳过（无法验证排序差异）
        return

    with tempfile.TemporaryDirectory() as tmpdir:
        path_original = pathlib.Path(tmpdir) / "original" / "ledger.csv"
        path_shuffled = pathlib.Path(tmpdir) / "shuffled" / "ledger.csv"

        # 原始顺序写入
        writer_original = LedgerCsvWriter()
        writer_original.write(unique_trades, path_original)

        # 反转顺序写入
        reversed_trades = list(reversed(unique_trades))

        writer_shuffled = LedgerCsvWriter()
        writer_shuffled.write(reversed_trades, path_shuffled)

        # 计算 sha256
        sha256_original = _sha256_file(path_original)
        sha256_shuffled = _sha256_file(path_shuffled)

        # 断言字节级一致
        assert sha256_original == sha256_shuffled, (
            f"P7 violated: 排序键唯一时输入顺序不同但输出 sha256 应一致。\n"
            f"  original sha256: {sha256_original}\n"
            f"  shuffled sha256: {sha256_shuffled}\n"
            f"  trade_count: {len(unique_trades)}\n"
            f"LedgerCsvWriter 的排序键未能保证确定性输出，"
            f"违反 Requirement 6.7 (Bit_Identical_Ledger)。"
        )


@given(trades=st.lists(trade_records(), min_size=0, max_size=20))
@settings(max_examples=50, deadline=None)
def test_p7_empty_and_single_trade_determinism(
    trades: list[TradeRecord],
) -> None:
    """P7 边界：空列表和单条 trade 的确定性。

    **Validates: Requirements 6.7**

    即使 trades 为空列表（只有 header），三次写入的 sha256 仍一致。
    """
    with tempfile.TemporaryDirectory() as tmpdir:
        paths = [
            pathlib.Path(tmpdir) / f"run_{i}" / "ledger.csv" for i in range(3)
        ]

        # 三次独立写入
        for path in paths:
            writer = LedgerCsvWriter()
            writer.write(trades, path)

        # 计算所有 sha256
        hashes = [_sha256_file(p) for p in paths]

        # 断言全部一致
        assert hashes[0] == hashes[1] == hashes[2], (
            f"P7 violated: 三次独立运行 sha256 不一致。\n"
            f"  hashes: {hashes}\n"
            f"  trade_count: {len(trades)}\n"
            f"违反 Requirement 6.7 (Bit_Identical_Ledger)。"
        )
