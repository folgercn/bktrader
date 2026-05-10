# Probabilistic V6 Union Sizing Calibration

范围：仅限 `research`。本阶段固定 `candidate_001` 的 probability sleeve selection，只改变 `model_notional_share` 的执行侧仓位变换，验证概率模型到底适合做“逐事件仓位”还是更适合做“候选筛选 + 风险预算”。

## 结论

概率模型可以继续用，但不应该无校准地相信原始事件级 `model_notional_share`。

本轮最好结果已进入 10-20% 候选区间：

| 配置 | Return | Trades | Active Months | Worst Silo | 说明 |
|---|---:|---:|---:|---:|---|
| `power0p50_mult_1p40_cap_1p80` | 14.7215% | 115 | 9 | -1.8247% | 最高收益，压平原始事件 share 后再放大 |
| `power0p25_mult_1p35_cap_1p80` | 14.0382% | 115 | 9 | -1.7145% | 收益略低，尾部略好 |
| `mult_1p30_cap_1p80` | 13.8919% | 115 | 9 | -1.7848% | 原始动态 share 直接放大，收益好但尾部较硬 |
| `power0p50_mult_1p30_cap_1p80` | 13.6649% | 115 | 9 | -1.6950% | 折中校准点 |
| `power0_fixed_1p30` | 13.4179% | 115 | 9 | -1.6084% | 推荐进入下一轮复测的稳健点 |
| `quality_edge_return_mult_1p20_cap_1p80` | 12.8486% | 115 | 9 | -1.5747% | source validation 质量参与 sizing，尾部更稳 |
| `baseline` | 10.8467% | 115 | 9 | -1.3744% | `candidate_001` 原始动态 share |

推荐下一阶段优先把 `power0_fixed_1p30` 和 `quality_edge_return_mult_1p20_cap_1p80` 作为候选，而不是直接拿最高收益点。原因是最高收益点的 worst silo 已到 `-1.8247%`，而 `power0_fixed_1p30` 已经把收益推到 `13.4179%`，同时比原始动态放大 `mult_1p30_cap_1p80` 的 worst silo 好 `0.1764%`。

## 关键观察

`2025-07/08 BTCUSDT` 仍是主要负贡献，但它们在 validation 上并不差：

| Silo | Baseline | `mult_1p30_cap_1p80` | `power0_fixed_1p30` |
|---|---:|---:|---:|
| `2025-07 BTCUSDT` | -0.7250% | -0.8781% | -0.1311% |
| `2025-08 BTCUSDT` | -1.3744% | -1.7848% | -1.6084% |
| `2026-02 BTCUSDT` | 4.3173% | 5.5227% | 4.8933% |
| `2026-03 ETHUSDT` | 4.8165% | 6.2524% | 5.4893% |

这说明 pass/fail gate 已经很难继续过滤 `2025-07/08 BTC`；更合理的方向是对概率模型的仓位输出做校准，而不是继续堆 gate。`share_power=0` 的含义是保留概率模型筛出来的事件，但把事件级动态 share 压平成固定倍率；这反而显著改善了 `2025-07 BTC`，说明原始逐事件 share 在跨 regime 上有校准偏差。

补充诊断：17 个 selected sleeve 上，事件级均值 `model_notional_share_mean` 与 execute sleeve return 的相关性为 `-0.4350`。这个数字只用于解释，不作为筛选条件；它提醒我们原始 share 不能直接当成“仓位越大越好”的强信号。

## 产物

- Runner 扩展：`research/probabilistic_v6_combo_union_runner.py`
  - 新增 `--quiet`
  - 新增 `--sizing-profile source_quality`
  - 新增 `--share-multiplier`
  - 新增 `--share-power`
  - 新增 `--share-cap` / `--share-floor`
  - 输出 group 级 mean/min/max share 和 source scale
- Sweep 工具：`research/probabilistic_v6_union_sizing_sweep.py`
- 主结果：`research/probabilistic_v6_runs/walkforward_2025m06_2026apr_combo_baseline_short_speed/union_sizing_calibration_sweep_candidate_001/summary.json`
- 可读结果：`research/probabilistic_v6_runs/walkforward_2025m06_2026apr_combo_baseline_short_speed/union_sizing_calibration_sweep_candidate_001/summary.md`

## 下一步

1. 用 `power0_fixed_1p30` 做完整 `dir2_zero_initial=true` / `zero_initial_mode=reentry_window` 生命周期复测，不能只停留在当前 one-shot 1s execution。
2. 对 `quality_edge_return_mult_1p20_cap_1p80` 做同样复测，判断是否值得牺牲约 `0.57%` 收益换更好的 tail。
3. 如果生命周期复测仍站在 10% 以上，再做 cross-asset / cross-year 验证，至少覆盖 BTC/ETH 2025、2026 非重叠窗口。

## 注意

- 本轮没有使用 execute-period return 作为筛选条件；`share_power` 与 multiplier 是固定 transform sweep。
- 当前结果仍是探索性 post-selection，不是实盘候选。
- 当前 runner 仍是 one-shot 1s execution，不是完整 baseline 生命周期；不能直接和 `dir2_zero_initial=true` 的 live/research baseline 等价。
