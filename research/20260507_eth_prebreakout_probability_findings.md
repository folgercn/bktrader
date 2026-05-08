# ETH 突破前概率与延续性结论

范围：仅限 `research`。策略语义只在 `research` 和 `live` 两条事实源之间讨论；本文不引用已移除的 replay 模块。下方所有交易模拟都使用本地 tick 聚合出的连续 `1s OHLC` 执行。

成本模型：

- 滑点：单边 `2bps`。
- 手续费：maker entry `2bps` + market exit `4bps`，往返 `6bps`。
- `realistic` 收益同时包含滑点和手续费。

## 当前参考基线

这里需要区分两件事：

- 项目长期 `research` baseline 仍是 `dir2_zero_initial=true` + `zero_initial_mode=reentry_window`，并使用 `reentry_size_schedule=[0.20, 0.10]`、`max_trades_per_bar=2`。
- 本文中反复对比的 `s10b4_nohold` 是这轮新做的 ETH 1h micro-strength + structure-trailing 实验候选，不等同于长期 baseline，也不等同于 live 的三根 bar intrabar breakout 语义。

真实 breakout 语义应按三根 bar 结构理解：第三根是当前未闭合 signal bar，不是等待 `1h` 收盘 close 确认。以 `original_t2` 为例，long 的结构 level 是 `prev_high_2`，并要求 T2 结构 ready（例如 `prev_high_2 > prev_high_1`，可带容差），触发条件是当前未闭合 bar 内 `1s high >= level`；short 的结构 level 是 `prev_low_2`，触发条件是当前未闭合 bar 内 `1s low <= level`。`baseline_plus_t3` 则可额外启用 `t3_swing` 结构。

`s10b4_nohold` runner 当前是 closed-bar proxy：它用聚合后的 `1h close` 判断突破，并在 `bar_end` 后进入 `1s` 执行段。更重要的是，它的突破 level 不是 T2/T3 三根 bar 结构，而是 `prev_high_8/prev_low_8`，即前 8 根已完成 signal bar 的 Donchian-style 区间高低点。因此这批结果只能作为方向性研究候选，不能作为 live-aligned baseline。

这轮 pre-breakout 实验前，ETH 1h micro-strength 实验候选为：

- 变体：`s10b4_nohold`
- 语义：closed-bar proxy 的 `1h` breakout 候选，`1s` 执行，跳过 weak micro strength，结构移动止损从 `1.0 ATR` 启动，`structure_bars=4`，无硬性 max hold。
- ETH 2025：112 笔，realistic `+12.9616%`，raw `+16.3758%`，最大回撤 `-2.3656%`。
- ETH 2026 Jan-Apr：27 笔，realistic `+2.3964%`，raw `+3.1068%`，最大回撤 `-1.4451%`。

产物：

- `research/20260507_eth_2025_micro_breakout_structure_nohold.md`
- `research/20260507_eth_2026_jan_apr_micro_breakout_structure_nohold.md`

## 突破前触碰概率

`1m` 经验状态表可以识别价格短时间内是否更容易触碰 `1h` breakout level，尤其是在距离 `0-0.10 ATR` 以内时。

注意：本节和后续多变量表沿用了 `prev_high_8/prev_low_8` level，因此评估的是 8-bar Donchian-style proxy，不是当前长期 baseline 的 `original_t2` / `baseline_plus_t3` 三根 bar 结构。下一轮若要用于 baseline 决策，必须改成 `prev_high_2/prev_low_2` 的 intrabar T2/T3 结构触发。

按距离汇总：

- ETH 2025 `0-0.10 ATR`：long hit `74.00%`，short hit `70.76%`。
- ETH 2026 Jan-Apr `0-0.10 ATR`：long hit `67.18%`，short hit `67.34%`。
- `0.10-0.20 ATR` 在 2026 降到约 `50%` 出头。
- 超过 `0.20 ATR` 后，触碰概率明显衰减。

解释：触碰 breakout 的概率确实存在，但它不等于扣除成本后的趋势延续概率。

产物：

- `research/20260507_eth_2025_prebreakout_markov_probability.md`
- `research/20260507_eth_2026_jan_apr_prebreakout_markov_probability.md`

## 多变量延续性表

只看距离是不够的。同样离 breakout `0.1 ATR`，可能是正在加速突破，也可能是平移漂移，或者已经触碰后又回撤到 level 下方。因此多变量表增加以下分桶：

- phase：`fresh` vs `post_touch_pullback`
- 到 breakout 的距离
- `1m/5m/15m` 涨速，按 ATR 归一
- `15m` efficiency
- `15m` close position
- `60m` volume z-score

目标也不再只是“是否触碰 breakout”。新的 outcome 是：触碰 breakout 之后，价格是否先走到 `+0.5 ATR` continuation，而不是先打到 `-0.2 ATR` post-touch fail。outcome 使用连续 `1s` bar 计算。

较稳定的高信号状态：

- ETH 2025 `fresh / 0.10-0.20 ATR / speed5>=0.08 / eff>=0.6`：118 样本，touch `52.54%`，continuation `19.49%`，post-fail `33.05%`。
- ETH 2026 Jan-Apr 同状态：41 样本，touch `68.29%`，continuation `29.27%`，post-fail `39.02%`。
- ETH 2025 `fresh / 0.20-0.30 ATR / speed5>=0.08 / eff=0.2-0.4`：43 样本，continuation `20.93%`。
- ETH 2026 Jan-Apr `fresh / 0.20-0.30 ATR / speed5>=0.08 / eff>=0.6`：99 样本，continuation `23.23%`。

解释：涨速和 efficiency 确实能把“正在突破”的状态和弱状态分开。但 continuation 概率仍偏低，post-fail 也偏高，不足以作为独立开仓规则。更适合先作为仓位调节或二级确认特征。

产物：

- `research/eth_prebreakout_multifeature_continuation.py`
- `research/20260507_eth_2025_prebreakout_multifeature_continuation.md`
- `research/20260507_eth_2026_jan_apr_prebreakout_multifeature_continuation.md`

## 突破前直接进场

根据突破前概率状态直接市价进场，会明显过度交易，扣除真实成本后亏损。

代表结果：

- ETH 2025 `pre_d005_p70`：573 笔，raw `+9.5556%`，realistic `-4.7642%`，最大回撤 `-13.4072%`。
- ETH 2025 `pre_d010_p70`：1114 笔，raw `+7.3563%`，realistic `-16.9782%`。
- ETH 2026 Jan-Apr `pre_d005_p70`：127 笔，raw `+3.0869%`，realistic `-0.1004%`。
- ETH 2026 Jan-Apr `pre_d010_p80`：9 笔，realistic `-0.5351%`。

结论：不要把突破前市价进场作为 baseline。

产物：

- `research/20260507_eth_2025_prebreakout_entry_replay.md`
- `research/20260507_eth_2026_jan_apr_prebreakout_entry_replay.md`

## Armed Touch 进场

高概率突破前状态只负责 arm，然后等待价格触碰 breakout level。这比立即提前进场更好，但扣除成本后仍然失败。

代表结果：

- ETH 2025 `arm_d010_p70`：929 笔，raw `+13.3942%`，realistic `-8.7797%`。
- ETH 2025 `arm_d010_p75`：577 笔，raw `+4.4330%`，realistic `-7.6168%`。
- ETH 2026 Jan-Apr `arm_d010_p70`：232 笔，raw `+2.5295%`，realistic `-2.9781%`。
- ETH 2026 Jan-Apr `arm_d010_p80`：8 笔，realistic `-0.4280%`。

结论：单纯触碰 breakout level 作为进场事件仍然太弱。

产物：

- `research/20260507_eth_2025_prebreakout_armed_entry_replay.md`
- `research/20260507_eth_2026_jan_apr_prebreakout_armed_entry_replay.md`

## Touch 后确认进场

新增 runner：

- `research/eth_prebreakout_posttouch_confirm_replay.py`

语义：

- 突破前状态只负责 arm。
- 每个 `1h` level 只消费一次 setup。
- 等待 breakout touch 后，还要求 `1s close` 按 `confirm_atr` 穿过 level。
- 进场价使用确认那根 `1s close`，不是 breakout level，也不是 `re_p`。
- 退出沿用当前 baseline 的结构移动止损逻辑。

ETH 2026 Jan-Apr 结果：

| Variant | Trades | Realistic | Raw | Win | Max DD |
|---|---:|---:|---:|---:|---:|
| `pt_d010_p70_c002` | 149 | `-0.7969%` | `+2.7390%` | `44.97%` | `-4.1897%` |
| `pt_d010_p70_c005` | 130 | `-2.4762%` | `+0.5560%` | `46.15%` | `-3.7525%` |
| `pt_d010_p75_c005` | 70 | `+0.7162%` | `+2.2790%` | `54.29%` | `-1.5151%` |
| `pt_d020_p65_c005` | 148 | `-3.0682%` | `+0.2461%` | `45.27%` | `-4.2064%` |

验证集里最好的 `pt_d010_p75_c005` 扣成本后为正，但仍弱于 ETH 1h 当前 micro breakout baseline：同一 2026 Jan-Apr 窗口下 `+0.7162%` vs `+2.3964%`。

ETH 2025 诊断：

- `pt_d010_p75_c005_h10`：375 笔，raw `+4.9779%`，realistic `-3.0352%`，最大回撤 `-8.6680%`。

无硬性 max hold 的 2025 全年版本因为部分仓位需要无界 `1s` 退出扫描，runner 过慢。这组 `hold=10h` 诊断足够说明进场 edge 不稳，但不能解读成最终退出设计。

产物：

- `research/20260507_eth_2026_jan_apr_prebreakout_posttouch_confirm_replay.md`
- `research/20260507_eth_2025_prebreakout_posttouch_confirm_replay_hold10.md`

## 建议

不要用突破前进场或 touch 后确认进场替换当前 baseline。

当前仍应保留 ETH 1h breakout + micro strength + structure trailing 作为领先候选。突破前概率表可以继续作为 watch signal 和诊断工具，但下一步应使用多变量 post-touch continuation quality，而不是距离单变量的 probability-to-touch。

下一轮实验建议：

- 触碰 breakout 后，价格是否在 `1h/2h` 内先到 `+0.5 ATR`，而不是先打 `-0.2 ATR`。
- 特征至少包含 `1s/1m` speed、efficiency、volume burst、level 上方停留时间、首次 touch 后回撤深度。
- 进场仍必须是 `1s close` 或接近市价成交的现实 fill，不能用 `re_p` 或历史 breakout level。
- 继续强制每个 `1h` level 只消费一次 setup，避免围绕同一 level 分钟级反复过度交易。
