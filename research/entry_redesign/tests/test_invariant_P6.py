"""Hypothesis property-based 测试：Walk-Forward Window Non-Overlap (P6).

**Validates: Requirements 6.6**

FOR ALL `iter_splits()` 产物：三窗口两两不相交且 `execute.start >= validation.end`。

测试覆盖：
1. 对每个 split 验证 train [start, end) 与 validation [start, end) 不重叠
2. 对每个 split 验证 validation [start, end) 与 execute [start, end) 不重叠
3. 对每个 split 验证 train [start, end) 与 execute [start, end) 不重叠
4. 对每个 split 验证 execute.start >= validation.end
5. 跨 split 验证 execute windows 两两不重叠

由于 WalkForwardDriver.iter_splits() 是确定性函数（固定产出 11 个 split），
本测试同时使用 Hypothesis 对 split 索引做 property-based 验证，以及对全量
11 个 split 做完整遍历验证。

Requirements: 6.6, 6.14
"""

from __future__ import annotations

from datetime import date

from hypothesis import given, settings
from hypothesis import strategies as st

from research.entry_redesign.walkforward.walkforward_driver import (
    WalkForwardDriver,
    WalkForwardSplit,
)

# ---------------------------------------------------------------------------
# 辅助函数
# ---------------------------------------------------------------------------


def _intervals_disjoint(start_a: date, end_a: date, start_b: date, end_b: date) -> bool:
    """检查两个半开区间 [start_a, end_a) 与 [start_b, end_b) 是否不相交。

    半开区间 [a, b) 与 [c, d) 不相交 iff b <= c OR d <= a。
    """
    return end_a <= start_b or end_b <= start_a


# ---------------------------------------------------------------------------
# 预加载所有 splits（确定性，共 11 个）
# ---------------------------------------------------------------------------

_driver = WalkForwardDriver()
_ALL_SPLITS: list[WalkForwardSplit] = list(_driver.iter_splits())


# ---------------------------------------------------------------------------
# Property-based tests: 对每个 split 索引验证 P6 不变量
# ---------------------------------------------------------------------------


@given(idx=st.integers(min_value=0, max_value=len(_ALL_SPLITS) - 1))
@settings(max_examples=100)
def test_p6_train_validation_disjoint(idx: int) -> None:
    """P6: train 与 validation 窗口不相交。

    **Validates: Requirements 6.6**

    FOR ALL split in iter_splits():
        train [start, end) ∩ validation [start, end) = ∅
    """
    split = _ALL_SPLITS[idx]
    assert _intervals_disjoint(
        split.train_start, split.train_end,
        split.validation_start, split.validation_end,
    ), (
        f"P6 violated: train [{split.train_start}, {split.train_end}) "
        f"overlaps with validation [{split.validation_start}, {split.validation_end}) "
        f"at split index {idx}"
    )


@given(idx=st.integers(min_value=0, max_value=len(_ALL_SPLITS) - 1))
@settings(max_examples=100)
def test_p6_validation_execute_disjoint(idx: int) -> None:
    """P6: validation 与 execute 窗口不相交。

    **Validates: Requirements 6.6**

    FOR ALL split in iter_splits():
        validation [start, end) ∩ execute [start, end) = ∅
    """
    split = _ALL_SPLITS[idx]
    assert _intervals_disjoint(
        split.validation_start, split.validation_end,
        split.execute_start, split.execute_end,
    ), (
        f"P6 violated: validation [{split.validation_start}, {split.validation_end}) "
        f"overlaps with execute [{split.execute_start}, {split.execute_end}) "
        f"at split index {idx}"
    )


@given(idx=st.integers(min_value=0, max_value=len(_ALL_SPLITS) - 1))
@settings(max_examples=100)
def test_p6_train_execute_disjoint(idx: int) -> None:
    """P6: train 与 execute 窗口不相交。

    **Validates: Requirements 6.6**

    FOR ALL split in iter_splits():
        train [start, end) ∩ execute [start, end) = ∅
    """
    split = _ALL_SPLITS[idx]
    assert _intervals_disjoint(
        split.train_start, split.train_end,
        split.execute_start, split.execute_end,
    ), (
        f"P6 violated: train [{split.train_start}, {split.train_end}) "
        f"overlaps with execute [{split.execute_start}, {split.execute_end}) "
        f"at split index {idx}"
    )


@given(idx=st.integers(min_value=0, max_value=len(_ALL_SPLITS) - 1))
@settings(max_examples=100)
def test_p6_execute_start_ge_validation_end(idx: int) -> None:
    """P6: execute.start >= validation.end。

    **Validates: Requirements 6.6**

    FOR ALL split in iter_splits():
        execute.start >= validation.end
    """
    split = _ALL_SPLITS[idx]
    assert split.execute_start >= split.validation_end, (
        f"P6 violated: execute.start ({split.execute_start}) "
        f"< validation.end ({split.validation_end}) "
        f"at split index {idx}"
    )


@given(
    idx_a=st.integers(min_value=0, max_value=len(_ALL_SPLITS) - 1),
    idx_b=st.integers(min_value=0, max_value=len(_ALL_SPLITS) - 1),
)
@settings(max_examples=200)
def test_p6_cross_split_execute_windows_disjoint(idx_a: int, idx_b: int) -> None:
    """P6: 跨 split 的 execute windows 两两不重叠。

    **Validates: Requirements 6.6**

    FOR ALL pairs (split_i, split_j) where i != j:
        execute_i [start, end) ∩ execute_j [start, end) = ∅
    """
    if idx_a == idx_b:
        return  # 同一 split 不需要比较

    split_a = _ALL_SPLITS[idx_a]
    split_b = _ALL_SPLITS[idx_b]
    assert _intervals_disjoint(
        split_a.execute_start, split_a.execute_end,
        split_b.execute_start, split_b.execute_end,
    ), (
        f"P6 violated: execute window of split {idx_a} "
        f"[{split_a.execute_start}, {split_a.execute_end}) "
        f"overlaps with execute window of split {idx_b} "
        f"[{split_b.execute_start}, {split_b.execute_end})"
    )


# ---------------------------------------------------------------------------
# 确定性全量遍历测试（补充 property-based 测试，确保 11 个 split 全覆盖）
# ---------------------------------------------------------------------------


def test_p6_all_splits_count() -> None:
    """验证 iter_splits() 产出恰好 11 个 split。

    **Validates: Requirements 6.6**
    """
    assert len(_ALL_SPLITS) == 11, (
        f"Expected 11 splits, got {len(_ALL_SPLITS)}"
    )


def test_p6_all_splits_pairwise_disjoint() -> None:
    """确定性遍历：对所有 11 个 split 验证三窗口两两不相交。

    **Validates: Requirements 6.6**

    对每个 split 验证：
    - train ∩ validation = ∅
    - validation ∩ execute = ∅
    - train ∩ execute = ∅
    - execute.start >= validation.end
    """
    for i, split in enumerate(_ALL_SPLITS):
        # train ∩ validation = ∅
        assert _intervals_disjoint(
            split.train_start, split.train_end,
            split.validation_start, split.validation_end,
        ), f"Split {i}: train overlaps validation"

        # validation ∩ execute = ∅
        assert _intervals_disjoint(
            split.validation_start, split.validation_end,
            split.execute_start, split.execute_end,
        ), f"Split {i}: validation overlaps execute"

        # train ∩ execute = ∅
        assert _intervals_disjoint(
            split.train_start, split.train_end,
            split.execute_start, split.execute_end,
        ), f"Split {i}: train overlaps execute"

        # execute.start >= validation.end
        assert split.execute_start >= split.validation_end, (
            f"Split {i}: execute.start ({split.execute_start}) "
            f"< validation.end ({split.validation_end})"
        )


def test_p6_all_execute_windows_non_overlapping() -> None:
    """确定性遍历：跨 split 的 execute windows 两两不重叠。

    **Validates: Requirements 6.6**
    """
    for i in range(len(_ALL_SPLITS)):
        for j in range(i + 1, len(_ALL_SPLITS)):
            split_a = _ALL_SPLITS[i]
            split_b = _ALL_SPLITS[j]
            assert _intervals_disjoint(
                split_a.execute_start, split_a.execute_end,
                split_b.execute_start, split_b.execute_end,
            ), (
                f"Execute windows overlap: split {i} "
                f"[{split_a.execute_start}, {split_a.execute_end}) "
                f"vs split {j} "
                f"[{split_b.execute_start}, {split_b.execute_end})"
            )


def test_p6_window_ordering_within_each_split() -> None:
    """确定性遍历：每个 split 内窗口按时间顺序排列。

    **Validates: Requirements 6.6**

    验证 train.start < train.end <= validation.start < validation.end
    <= execute.start < execute.end
    """
    for i, split in enumerate(_ALL_SPLITS):
        assert split.train_start < split.train_end, (
            f"Split {i}: train_start ({split.train_start}) "
            f">= train_end ({split.train_end})"
        )
        assert split.train_end <= split.validation_start, (
            f"Split {i}: train_end ({split.train_end}) "
            f"> validation_start ({split.validation_start})"
        )
        assert split.validation_start < split.validation_end, (
            f"Split {i}: validation_start ({split.validation_start}) "
            f">= validation_end ({split.validation_end})"
        )
        assert split.validation_end <= split.execute_start, (
            f"Split {i}: validation_end ({split.validation_end}) "
            f"> execute_start ({split.execute_start})"
        )
        assert split.execute_start < split.execute_end, (
            f"Split {i}: execute_start ({split.execute_start}) "
            f">= execute_end ({split.execute_end})"
        )
