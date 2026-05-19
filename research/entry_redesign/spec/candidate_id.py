"""确定性 candidate_id 生成器。

按 D > H > Trigger_Confirmation > Entry_Price_Mode > Pretouch_State_Band >
Posttouch_Quality_Band 顺序，将六元组各字段通过固定 TOKEN_MAP 映射为小写
ASCII token，以下划线拼接后追加 `-` + sha256(规范化 JSON)[:12]。

产出 MUST 满足正则 ^[a-z0-9]+(?:_[a-z0-9]+)*-[0-9a-f]{12}$，长度 14-64。
同一输入 MUST 产出 byte-identical candidate_id。

Requirements: 2.12
"""

from __future__ import annotations

import hashlib
import json
import re
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from research.entry_redesign.spec.entry_candidate_spec import EntryCandidateSpec

# ---------------------------------------------------------------------------
# 固定 token 映射表（常量字典）
# 每个字段值映射为固定的小写 ASCII token（token 内部使用数字和字母，
# 不含下划线——下划线仅用于 token 之间的拼接分隔符）。
# ---------------------------------------------------------------------------

# D (Entry_Delay_Seconds) token: "d{value}"
_D_TOKEN_MAP: dict[int, str] = {
    0: "d0",
    5: "d5",
    15: "d15",
    30: "d30",
    60: "d60",
    120: "d120",
}

# H (Feature_Horizon_Seconds) token: "h{value}"
_H_TOKEN_MAP: dict[int, str] = {
    0: "h0",
    5: "h5",
    15: "h15",
    30: "h30",
    60: "h60",
}

# Trigger_Confirmation token
_TRIGGER_CONFIRMATION_TOKEN_MAP: dict[str, str] = {
    "none": "tcnone",
    "persistence_n1": "tcpn1",
    "persistence_n3": "tcpn3",
    "persistence_n5": "tcpn5",
    "persistence_n10": "tcpn10",
    "retest_tb0": "tcrtb0",
    "retest_tb1": "tcrtb1",
    "retest_tb2": "tcrtb2",
    "minvol_bps50": "tcmv50",
    "minvol_bps100": "tcmv100",
    "minvol_bps200": "tcmv200",
}

# Entry_Price_Mode token
_ENTRY_PRICE_MODE_TOKEN_MAP: dict[str, str] = {
    "market_on_touch": "epmot",
    "limit_at_level": "eplal",
    "limit_tb_k0": "eptbk0",
    "limit_tb_k1": "eptbk1",
    "limit_tb_k2": "eptbk2",
    "limit_tb_k4": "eptbk4",
    "pullback_p000": "eppb000",
    "pullback_p002": "eppb002",
    "pullback_p005": "eppb005",
    "pullback_p010": "eppb010",
}

# Pretouch_State_Band token
_PRETOUCH_STATE_BAND_TOKEN_MAP: dict[str, str] = {
    "none": "prnone",
    "fast_clean": "prfc",
    "fast_clean_strict": "prfcs",
}

# Posttouch_Quality_Band token
_POSTTOUCH_QUALITY_BAND_TOKEN_MAP: dict[str, str] = {
    "none": "ponone",
    "cont1s_r003": "poc003",
    "cont1s_r005": "poc005",
    "cont1s_r008": "poc008",
    "tickimb_b055": "poti055",
    "tickimb_b060": "poti060",
    "tickimb_b065": "poti065",
    "spread_s1": "posp1",
    "spread_s2": "posp2",
    "spread_s4": "posp4",
}

# 合并导出（供外部测试或调试使用）
TOKEN_MAP = {
    "entry_delay_seconds": _D_TOKEN_MAP,
    "feature_horizon_seconds": _H_TOKEN_MAP,
    "trigger_confirmation_id": _TRIGGER_CONFIRMATION_TOKEN_MAP,
    "entry_price_mode_id": _ENTRY_PRICE_MODE_TOKEN_MAP,
    "pretouch_state_band_id": _PRETOUCH_STATE_BAND_TOKEN_MAP,
    "posttouch_quality_band_id": _POSTTOUCH_QUALITY_BAND_TOKEN_MAP,
}

# candidate_id 正则校验
_CANDIDATE_ID_REGEX = re.compile(r"^[a-z0-9]+(?:_[a-z0-9]+)*-[0-9a-f]{12}$")


# ---------------------------------------------------------------------------
# 公开 API
# ---------------------------------------------------------------------------


def generate_candidate_id(spec: "EntryCandidateSpec") -> str:
    """为给定的 EntryCandidateSpec 生成确定性 candidate_id。

    算法：
    1. 按固定顺序 D > H > Trigger_Confirmation > Entry_Price_Mode >
       Pretouch_State_Band > Posttouch_Quality_Band 查 TOKEN_MAP 得到 token。
    2. 以下划线拼接所有 token 形成 prefix。
    3. 计算六元组规范化 JSON（sorted keys, separators=(',', ':')）的 sha256，
       取前 12 位 hex（零填充）。
    4. 最终 candidate_id = prefix + "-" + hex12。

    Returns:
        满足正则 ^[a-z0-9]+(?:_[a-z0-9]+)*-[0-9a-f]{12}$ 的字符串，
        长度 14-64。

    Raises:
        KeyError: 如果六元组中某字段值不在 TOKEN_MAP 中。
        ValueError: 如果生成的 candidate_id 不满足正则或长度约束。
    """
    # Step 1: 查 token
    d_token = _D_TOKEN_MAP[spec.entry_delay_seconds]
    h_token = _H_TOKEN_MAP[spec.feature_horizon_seconds]
    tc_token = _TRIGGER_CONFIRMATION_TOKEN_MAP[spec.trigger_confirmation_id]
    ep_token = _ENTRY_PRICE_MODE_TOKEN_MAP[spec.entry_price_mode_id]
    pr_token = _PRETOUCH_STATE_BAND_TOKEN_MAP[spec.pretouch_state_band_id]
    po_token = _POSTTOUCH_QUALITY_BAND_TOKEN_MAP[spec.posttouch_quality_band_id]

    # Step 2: 下划线拼接
    prefix = "_".join([d_token, h_token, tc_token, ep_token, pr_token, po_token])

    # Step 3: 规范化 JSON → sha256 前 12 位 hex
    canonical_dict = {
        "entry_delay_seconds": spec.entry_delay_seconds,
        "entry_price_mode_id": spec.entry_price_mode_id,
        "feature_horizon_seconds": spec.feature_horizon_seconds,
        "posttouch_quality_band_id": spec.posttouch_quality_band_id,
        "pretouch_state_band_id": spec.pretouch_state_band_id,
        "trigger_confirmation_id": spec.trigger_confirmation_id,
    }
    canonical_json = json.dumps(canonical_dict, sort_keys=True, separators=(",", ":"))
    sha_hex = hashlib.sha256(canonical_json.encode("utf-8")).hexdigest()[:12]

    # Step 4: 拼接最终 candidate_id
    candidate_id = f"{prefix}-{sha_hex}"

    # 校验正则与长度
    if not _CANDIDATE_ID_REGEX.match(candidate_id):
        raise ValueError(
            f"Generated candidate_id does not match required regex: {candidate_id!r}"
        )
    if not (14 <= len(candidate_id) <= 64):
        raise ValueError(
            f"Generated candidate_id length {len(candidate_id)} not in [14, 64]: "
            f"{candidate_id!r}"
        )

    return candidate_id
