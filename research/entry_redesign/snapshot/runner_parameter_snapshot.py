"""RunnerParameterSnapshot dataclass 与 to_json() 序列化。

每个 Entry_Candidate 运行落一份 JSON 快照。字段固定，不允许 runtime 注入。
JSON 写盘要求：稳定键序（sorted keys）、UTF-8 无 BOM、LF 结尾、
数值定点（禁科学计数法）、禁 datetime.now() / os.getpid()。

runner_version 字面格式固定为 "entry_redesign_runner_vMAJOR.MINOR.PATCH"，
其中 MAJOR/MINOR/PATCH 为十进制非负整数；禁止运行时注入或通过环境变量覆盖。

Requirements: 2.8, 4.9
"""

from __future__ import annotations

import json
import re
from dataclasses import asdict, dataclass
from typing import Any

from research.entry_redesign.spec.entry_candidate_spec import EntryCandidateSpec


# ---------------------------------------------------------------------------
# runner_version 格式校验正则
# ---------------------------------------------------------------------------

_RUNNER_VERSION_REGEX = re.compile(
    r"^entry_redesign_runner_v[0-9]+\.[0-9]+\.[0-9]+$"
)


# ---------------------------------------------------------------------------
# 嵌套 dataclass：CostModelParams
# ---------------------------------------------------------------------------


@dataclass(frozen=True)
class CostModelParams:
    """成本模型参数。

    baseline: slip=2bps/side + maker_entry=2bps + taker_exit=4bps
    stress:   slip=2bps/side + taker_entry=4bps + taker_exit=4bps
    """

    slip_bps_per_side: float
    entry_bps: float
    exit_bps: float


# ---------------------------------------------------------------------------
# 嵌套 dataclass：SymbolFilters
# ---------------------------------------------------------------------------


@dataclass(frozen=True)
class SymbolFilters:
    """每个 symbol 的 tick_size 与 step_size。

    tick_size_by_symbol: 例如 {"BTCUSDT": 0.10, "ETHUSDT": 0.01}
    step_size_by_symbol: 例如 {"BTCUSDT": 0.001, "ETHUSDT": 0.001}
    """

    tick_size_by_symbol: dict[str, float]
    step_size_by_symbol: dict[str, float]


# ---------------------------------------------------------------------------
# 嵌套 dataclass：Atr14Source
# ---------------------------------------------------------------------------


@dataclass(frozen=True)
class Atr14Source:
    """1h ATR(14) CSV 路径与 sha256。"""

    path: str
    sha256: str


# ---------------------------------------------------------------------------
# 嵌套 dataclass：FeatureSources
# ---------------------------------------------------------------------------


@dataclass(frozen=True)
class FeatureSources:
    """特征数据源声明。"""

    atr14_source: Atr14Source


# ---------------------------------------------------------------------------
# RunnerParameterSnapshot
# ---------------------------------------------------------------------------


@dataclass(frozen=True)
class RunnerParameterSnapshot:
    """Runner 参数快照。每个 Entry_Candidate 运行落一份 JSON。

    字段固定，不允许 runtime 注入或环境变量覆盖。
    """

    candidate: EntryCandidateSpec
    cost_model_baseline: CostModelParams
    cost_model_stress: CostModelParams
    seed: int
    git_commit_sha: str
    events_source_path: str
    events_source_sha256: str
    runner_version: str
    symbol_filters: SymbolFilters
    features: FeatureSources

    def __post_init__(self) -> None:
        """校验 runner_version 格式。"""
        if not _RUNNER_VERSION_REGEX.match(self.runner_version):
            raise ValueError(
                f"runner_version must match format "
                f"'entry_redesign_runner_vMAJOR.MINOR.PATCH', "
                f"got: {self.runner_version!r}"
            )

    def to_json(self) -> str:
        """序列化为 JSON 字符串。

        保证：
          - 稳定键序（sorted keys）
          - UTF-8 无 BOM
          - LF 结尾（末尾恰好一个 '\\n'）
          - 数值定点（禁科学计数法）
          - 不使用 datetime.now() / os.getpid() 或任何非确定性系统调用

        Returns:
            JSON 字符串，以 LF 结尾。
        """
        return snapshot_to_json(self)


# ---------------------------------------------------------------------------
# JSON 序列化辅助：定点数值格式化
# ---------------------------------------------------------------------------


def _json_default(obj: Any) -> Any:
    """处理 json.dumps 无法直接序列化的类型。

    当前实现中 dataclasses.asdict() 已将所有嵌套 dataclass 转为 dict，
    此函数作为安全兜底。
    """
    raise TypeError(f"Object of type {type(obj).__name__} is not JSON serializable")


class _FixedPointEncoder(json.JSONEncoder):
    """自定义 JSON encoder：浮点数使用定点表示，禁止科学计数法。"""

    def encode(self, o: Any) -> str:
        return _encode_value(o)

    def iterencode(self, o: Any, _one_shot: bool = False) -> Any:  # type: ignore[override]
        yield self.encode(o)


def _format_float(value: float) -> str:
    """将浮点数格式化为定点十进制字符串（最多 8 位小数，无科学计数法）。

    去除尾部多余的零，但保留至少一位小数以区分 int。
    """
    # 使用 8 位定点
    formatted = f"{value:.8f}"
    # 去除尾部多余零，但保留小数点后至少一位
    if "." in formatted:
        formatted = formatted.rstrip("0")
        if formatted.endswith("."):
            formatted += "0"
    return formatted


def _encode_value(obj: Any, indent: int = 0, sort_keys: bool = True) -> str:
    """递归编码值为 JSON 字符串，浮点使用定点格式。"""
    if obj is None:
        return "null"
    if isinstance(obj, bool):
        return "true" if obj else "false"
    if isinstance(obj, int):
        return str(obj)
    if isinstance(obj, float):
        return _format_float(obj)
    if isinstance(obj, str):
        return json.dumps(obj, ensure_ascii=False)
    if isinstance(obj, list):
        if not obj:
            return "[]"
        items = []
        child_indent = indent + 2
        for item in obj:
            items.append(" " * child_indent + _encode_value(item, child_indent, sort_keys))
        return "[\n" + ",\n".join(items) + "\n" + " " * indent + "]"
    if isinstance(obj, dict):
        if not obj:
            return "{}"
        items = []
        child_indent = indent + 2
        keys = sorted(obj.keys()) if sort_keys else list(obj.keys())
        for key in keys:
            key_str = json.dumps(key, ensure_ascii=False)
            val_str = _encode_value(obj[key], child_indent, sort_keys)
            items.append(f"{' ' * child_indent}{key_str}: {val_str}")
        return "{\n" + ",\n".join(items) + "\n" + " " * indent + "}"
    raise TypeError(f"Object of type {type(obj).__name__} is not JSON serializable")


def snapshot_to_json(snapshot: RunnerParameterSnapshot) -> str:
    """将 RunnerParameterSnapshot 序列化为稳定键序、定点数值的 JSON 字符串。

    这是 to_json() 的底层实现，也可独立调用。

    保证：
      - 稳定键序（sorted keys）
      - UTF-8 无 BOM
      - LF 结尾
      - 数值定点（禁科学计数法）
      - 不使用 datetime.now() / os.getpid()
    """
    raw = asdict(snapshot)
    json_str = _encode_value(raw, indent=0, sort_keys=True)
    if not json_str.endswith("\n"):
        json_str += "\n"
    return json_str
