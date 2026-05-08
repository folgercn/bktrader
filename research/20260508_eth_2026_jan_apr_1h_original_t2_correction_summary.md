# ETH 1h original_t2 校正回测总结

范围：仅限 `research`。本轮用于修正前一批 `prev_high_8/prev_low_8` 8-bar proxy 结果，全部改为 `original_t2` 三根 bar intrabar 触发：long level 为 `prev_high_2`，short level 为 `prev_low_2`，当前 signal bar 未闭合，由 `1s high/low` 触发，成交价为触发那根 `1s close`。

成本模型统一为：滑点 `2bps/side`，手续费 maker entry `2bps` + market exit `4bps`，即 `6bps` 往返手续费。仓位 schedule 为 slot0 `20%`、slot1 `10%`，允许同一根 signal bar 最多两次真实开仓。

## 结果汇总

| Variant | 语义 | 笔数 | Realistic | Raw | 2bps Slip No Fee | Fees | 胜率 | 备注 |
|---|---|---:|---:|---:|---:|---:|---:|---|
| `direct` | T2 触发即进场 | 940 | `-9.7055%` | `+5.6316%` | `-0.7917%` | `8.9917%` | `36.70%` | raw 为正，但成本后大幅转负 |
| `skipweak_s10b4` | weak 跳过，但同 bar 后续仍可等变强 | 934 | `-9.7705%` | `+5.4922%` | `-0.8987%` | `8.9545%` | `36.94%` | 几乎不降频，只是延后入场 |
| `oneshot_s10b4` | 首次触发 weak 则整根 bar 放弃 | 845 | `-9.0969%` | `+5.3169%` | `-0.7026%` | `8.4765%` | `36.80%` | 有改善但仍远不够 |
| `strict_oneshot` | 更高 speed/eff 阈值 | 809 | `-8.8503%` | `+5.0863%` | `-0.7255%` | `8.2004%` | `36.71%` | 仍然过度交易 |
| `very_strict_oneshot` | 明显冲击形态才入场 | 727 | `-7.0333%` | `+5.7862%` | `+0.4605%` | `7.5246%` | `37.55%` | 滑点后略正，但手续费仍吞掉收益 |
| `strong_only` | 只交易 very-strict strong，base 也跳过 | 555 | `-6.0883%` | `+3.9848%` | `-0.1670%` | `5.9648%` | `38.38%` | 降频明显，但仍不足以覆盖成本 |

## 关键发现

- 前一批 `prev_high_8/prev_low_8` 结果不能用于 `original_t2` 结论；它们是低频区间突破 proxy。
- 真正 `original_t2` 在 ETH 2026 Jan-Apr 的 1h 上非常高频：direct 有 940 笔，629 根 signal bar 有开仓，311 根 bar 出现第二次开仓。
- 单纯用 `1s` speed/efficiency 过滤并不能解决问题，因为 T2 触发时很多 setup 已经表现为强冲击状态。
- `very_strict_oneshot` 能把 2bps slip no fee 拉到 `+0.4605%`，说明强度过滤有一点方向，但手续费后仍为 `-7.0333%`。
- `strong_only` 降到 555 笔，但 raw 也随之降到 `+3.9848%`，realistic 仍为 `-6.0883%`。

## 下一步建议

不要继续只调 `speed/efficiency` 阈值。`original_t2` 当前问题不是“突破瞬间有没有速度”，而是“触发后是否能产生足够延续，并且交易频率是否低到能覆盖成本”。

下一轮应改测：

- Touch 后 continuation quality：触发 `original_t2` 后，是否先到 `+0.5 ATR` 而不是先回撤 `-0.2 ATR`。
- Level 上方/下方停留时间：例如 long 触发后连续 N 秒 close 在 level 上方。
- 触发后回撤深度：过滤触发后立即回落到 level 下方的假突破。
- 只保留每根 signal bar 第一次高质量 setup，减少同 bar 反复交易。
- 仓位按质量进一步缩小，而不是默认 slot0 `20%`、slot1 `10%`。

## 文件

- Direct summary：`research/eth_2026_jan_apr_1h_original_t2_direct_breakout_summary.json`
- Micro filter summaries：
  - `research/eth_2026_jan_apr_1h_original_t2_micro_filter_summary.json`
  - `research/eth_2026_jan_apr_1h_original_t2_micro_oneshot_summary.json`
  - `research/eth_2026_jan_apr_1h_original_t2_micro_strict_oneshot_summary.json`
  - `research/eth_2026_jan_apr_1h_original_t2_micro_very_strict_oneshot_summary.json`
  - `research/eth_2026_jan_apr_1h_original_t2_micro_strong_only_summary.json`
- Runner：`research/eth_original_t2_micro_filter_replay.py`
