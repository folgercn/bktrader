# Probabilistic V4 回测推进计划

范围：仅限 `research`。本轮没有修改 `live` / `internal` 逻辑，也不把 V4 结果解释成生产可用策略。

## 为什么冻结 V3

`probabilistic_baseline_runner.py` 的 V3 版本把以下内容放进同一个循环：

- pre-touch 提前进场
- breakout 触发进场
- Markov order-flow LLR
- HMM regime multiplier
- 仓位缩放
- SL / trailing / smart TP

这种写法能快速验证想法，但亏损后无法判断是哪一层错。更核心的问题是，V3 把“价格大概率会触碰 level”和“触碰后净 EV 为正”混成同一个 entry 目标。前几轮研究已经说明，touch probability 有信息量，但直接交易容易被手续费、滑点和假突破吃掉。

## V4 拆分

本轮新增三层脚本：

1. `research/probabilistic_v4_event_dataset.py`
   - 只生成事件表，不做资金曲线。
   - 事件语义：true intrabar `original_t2` / `baseline_plus_t3` touch。
   - 输出 point-in-time features：order-flow ratio、speed、efficiency、dwell、pullback、Markov state sequence。
   - 输出 post-touch outcome：是否先到 `+continuation_atr`，还是先打 `-fail_atr`。

2. `research/probabilistic_v4_quality_model.py`
   - 只训练和选择质量规则，不模拟交易。
   - 用训练段事件构造 Markov win/loss transition matrices。
   - 在验证段 sweep 显式规则：`llr_min`、`flow60_min`、`speed60_min`、`dwell_seconds`、`pullback30_max`。
   - 输出 scored event CSV 和 selected rule JSON。

3. `research/probabilistic_v4_execution_runner.py`
   - 只消费 scored events / selected rule。
   - 用连续 `1s` OHLC 执行简单资金曲线。
   - 开仓止损只使用 touch 当刻已经形成的 `touch_high_so_far/touch_low_so_far`，不使用整根 signal bar 的未来 high/low。
   - 默认一个 signal bar 只消费一个 selected setup，避免 V3 式反复交易。

另新增批处理入口：

- `research/probabilistic_v4_matrix_runner.py`
  - 编排 event dataset -> quality model -> execution runner。
  - 每个 run 写入 `research/probabilistic_v4_runs/<run-name>/`。
  - 同一 run 内使用 `bars_cache/` 复用 BTC/ETH 的连续 `1s` flow bars，避免每个 execution variant 反复重读 tick。
  - 聚合 `events`、`selected_events`、`trades`、`realistic`、`raw`、`2bps slip`、`profit_factor`、`win`、`max_dd` 到 `summary.json` / `summary.md`。

## Smoke 结果

命令：

```bash
python3 research/probabilistic_v4_event_dataset.py \
  --symbols BTCUSDT \
  --start 2026-01-01T00:00:00Z \
  --end 2026-01-07T23:59:59Z \
  --breakout-shape baseline_plus_t3 \
  --horizon-seconds 3600 \
  --chunksize 5000000 \
  --output-csv research/tmp_probabilistic_v4_smoke_events.csv \
  --summary-json research/tmp_probabilistic_v4_smoke_events_summary.json

python3 research/probabilistic_v4_quality_model.py \
  --events-csv research/tmp_probabilistic_v4_smoke_events.csv \
  --scored-csv research/tmp_probabilistic_v4_smoke_events_scored.csv \
  --rules-json research/tmp_probabilistic_v4_smoke_quality_rules.json \
  --markdown research/tmp_probabilistic_v4_smoke_quality.md \
  --train-ratio 0.6 \
  --min-events 3

python3 research/probabilistic_v4_execution_runner.py \
  --events-csv research/tmp_probabilistic_v4_smoke_events_scored.csv \
  --rules-json research/tmp_probabilistic_v4_smoke_quality_rules.json \
  --symbols BTCUSDT \
  --start 2026-01-01T00:00:00Z \
  --end 2026-01-07T23:59:59Z \
  --entry-delay-seconds 30 \
  --chunksize 5000000 \
  --summary-json research/tmp_probabilistic_v4_smoke_execution_summary.json \
  --markdown research/tmp_probabilistic_v4_smoke_execution.md \
  --ledger-prefix research/tmp_probabilistic_v4_smoke_execution
```

结果：

- Event dataset：49 个 BTC 事件，`continuation=14`、`fail=33`、`timeout=2`。
- Validation baseline：20 个验证事件，success `30.00%`，avg net first edge `-0.176454 ATR`。
- Selected smoke rule：`speed60_min=0.08` + `dwell_seconds=30`，验证段 4 个事件，success `75.00%`，avg net first edge `+0.111360 ATR`。
- Execution smoke：11 个 selected events，实际进场 10 笔，realistic `-0.1691%`，raw `+0.0308%`，profit factor `0.517344`，win `50.00%`。
- Matrix smoke：`research/probabilistic_v4_runs/smoke_btc_20260101_20260107/summary.md` 已生成同一结果的聚合表。

解释：event 层和 quality 层在短样本上能筛出正的 post-touch first-edge，但执行层仍被真实 entry delay、stop、trailing、费用结构打回负值。这说明 V4 拆层是有价值的：下一步应重点测 execution design，而不是继续把更多特征塞进同一个 V3 runner。

## 下一轮矩阵

第一轮正式回测建议覆盖：

- 数据集：
  - ETH 2025 全年
  - ETH 2026 Jan-Apr
  - BTC 2026 Jan-Apr
- breakout shape：
  - `original_t2`
  - `baseline_plus_t3`
- quality sweep：
  - `llr_min`: `0 / 2 / 4 / 6`
  - `flow60_min`: `0.55 / 0.60 / 0.65`
  - `speed60_min`: `0 / 0.03 / 0.08`
  - `dwell_seconds`: `5 / 15 / 30`
  - `pullback30_max`: `none / 0.05 / 0.10 / 0.20`
- execution sweep：
  - `entry_delay_seconds`: 与 selected dwell 对齐
  - `initial_stop_atr`: `0.45 / 0.60 / 0.80`
  - `breakeven_at_r`: `0.8 / 1.0 / 1.2`
  - `trail_start_r`: `1.2 / 1.5 / 2.0`
  - `max_hold_hours`: `2 / 4 / 6`

建议先用 matrix runner 跑窄版：

```bash
python3 research/probabilistic_v4_matrix_runner.py \
  --run-name btc_eth_2026_jan_apr_v4_narrow \
  --symbols BTCUSDT ETHUSDT \
  --start 2026-01-01T00:00:00Z \
  --end 2026-04-30T23:59:59Z \
  --breakout-shapes original_t2 baseline_plus_t3 \
  --horizon-seconds 7200 \
  --min-events 20 \
  --execution-variants \
    base=delay:15,stop:0.45,be:1.0,trail:1.5,hold:6 \
    wider_stop=delay:15,stop:0.60,be:1.0,trail:1.5,hold:6 \
    fast_trail=delay:15,stop:0.45,be:0.8,trail:1.2,hold:4
```

如果窄版出现正收益候选，再扩到 ETH 2025 全年和完整 execution sweep。

晋级条件：

- train/validation 分开，不能只看 in-sample。
- ETH 2025、ETH 2026 Jan-Apr、BTC 2026 Jan-Apr 至少两个集合 realistic 为正；否则必须标注单币种候选。
- realistic profit factor 和 monthly attribution 需要补进下一版 summary。
- selected rule 不应只来自极小样本；Jan-Apr 至少 20-30 笔，全年至少 80 笔。
- raw 到 realistic 的成本折损不能吞掉大部分 edge。

## 2026 Jan-Apr 窄矩阵结果

Run：

- `research/probabilistic_v4_runs/btc_eth_2026_jan_apr_v4_narrow/summary.md`
- quality selection scope：`global`
- elapsed：`1382.02s`

全局 quality rule 的结果很分裂：BTC 两个 shape 都能跑出正 realistic，ETH 两个 shape 全部为负。

| Shape | Variant | BTC Realistic | BTC PF | BTC DD | ETH Realistic | ETH PF | ETH DD |
|---|---|---:|---:|---:|---:|---:|---:|
| `original_t2` | `base` | `+1.0251%` | `1.479005` | `-0.3861%` | `-2.3211%` | `0.385802` | `-2.3289%` |
| `original_t2` | `wider_stop` | `+0.4289%` | `1.177280` | `-0.5524%` | `-2.8079%` | `0.319415` | `-2.8157%` |
| `original_t2` | `fast_trail` | `+1.1224%` | `1.645179` | `-0.4205%` | `-1.9733%` | `0.436674` | `-2.1899%` |
| `baseline_plus_t3` | `base` | `+0.9933%` | `1.540488` | `-0.5481%` | `-1.6265%` | `0.607449` | `-1.9916%` |
| `baseline_plus_t3` | `wider_stop` | `+0.9226%` | `1.480255` | `-0.6414%` | `-2.0425%` | `0.560765` | `-2.3417%` |
| `baseline_plus_t3` | `fast_trail` | `+0.8827%` | `1.590819` | `-0.3894%` | `-1.8235%` | `0.541291` | `-2.2790%` |

观察：

- 当前最好 BTC 候选是 `original_t2 + fast_trail`：58 笔，realistic `+1.1224%`，PF `1.645179`，win `70.69%`，DD `-0.4205%`。
- `baseline_plus_t3 + base` 的 BTC 也接近：54 笔，realistic `+0.9933%`，PF `1.540488`。
- ETH 的 raw 在部分组合并非完全没有 edge，但 realistic 全部被成本和 InitialSL 吃掉。最不差的是 `baseline_plus_t3 + base`：realistic `-1.6265%`，PF `0.607449`。
- `wider_stop` 没有解决问题，反而普遍恶化，说明主要矛盾不是 stop 太窄，而是假突破 / 入场质量 / 成本折损。
- `fast_trail` 帮 BTC 的 `original_t2`，但对 ETH 只能减亏，不能转正。

全局规则本身：

- `original_t2`: `llr_min=0.0`, `flow60_min=0.0`, `speed60_min=-999.0`, `dwell_seconds=30`, `pullback30_max=null`
- `baseline_plus_t3`: `llr_min=0.0`, `flow60_min=0.6`, `speed60_min=-999.0`, `dwell_seconds=30`, `pullback30_max=null`

这说明第一版 Markov LLR 在当前 sweep 中没有稳定成为主筛选器；真正起作用的是 touch 后 dwell，以及 `baseline_plus_t3` 上的 flow60 过滤。

## Symbol-Specific Quality 对照

本轮新增 `--selection-scope per_symbol`：

- `research/probabilistic_v4_quality_model.py` 支持每个 symbol 单独训练 Markov transition matrices，并在各自 validation subset 上选择 quality rule。
- `research/probabilistic_v4_matrix_runner.py` 支持透传 `--selection-scope`，并新增 `--bars-cache-dir`，可复用既有 `1s` bars cache。

Run：

- `research/probabilistic_v4_runs/btc_eth_2026_jan_apr_v4_per_symbol/summary.md`
- quality selection scope：`per_symbol`
- elapsed：`502.49s`，复用 `btc_eth_2026_jan_apr_v4_narrow/bars_cache`

选出的规则：

| Shape | Symbol | Rule |
|---|---|---|
| `original_t2` | `BTCUSDT` | `llr_min=0.0`, `flow60_min=0.0`, `speed60_min=-999.0`, `dwell_seconds=30`, `pullback30_max=null` |
| `original_t2` | `ETHUSDT` | `llr_min=-999.0`, `flow60_min=0.0`, `speed60_min=0.08`, `dwell_seconds=15`, `pullback30_max=0.05` |
| `baseline_plus_t3` | `BTCUSDT` | `llr_min=0.0`, `flow60_min=0.0`, `speed60_min=-999.0`, `dwell_seconds=30`, `pullback30_max=null` |
| `baseline_plus_t3` | `ETHUSDT` | `llr_min=-999.0`, `flow60_min=0.0`, `speed60_min=0.08`, `dwell_seconds=30`, `pullback30_max=0.05` |

per-symbol 结果：

| Shape | Variant | BTC Realistic | BTC PF | BTC DD | ETH Realistic | ETH PF | ETH DD |
|---|---|---:|---:|---:|---:|---:|---:|
| `original_t2` | `base` | `+0.4296%` | `1.156720` | `-0.7716%` | `-0.4083%` | `0.902528` | `-1.8486%` |
| `original_t2` | `wider_stop` | `+0.0684%` | `1.024077` | `-0.9204%` | `-0.7254%` | `0.839584` | `-1.9920%` |
| `original_t2` | `fast_trail` | `+0.4814%` | `1.194136` | `-0.8326%` | `-0.6087%` | `0.844822` | `-1.6756%` |
| `baseline_plus_t3` | `base` | `+0.5616%` | `1.147608` | `-0.9741%` | `-0.6189%` | `0.889464` | `-1.9253%` |
| `baseline_plus_t3` | `wider_stop` | `+0.3852%` | `1.094911` | `-1.0795%` | `-1.0308%` | `0.827348` | `-1.9935%` |
| `baseline_plus_t3` | `fast_trail` | `+0.3538%` | `1.104932` | `-0.8698%` | `-0.9525%` | `0.813242` | `-2.0445%` |

解释：

- per-symbol 规则把 ETH 从 `-1.63% ~ -2.81%` 的亏损区间压到 `-0.41% ~ -1.03%`，说明 ETH 需要 `speed60_min=0.08` 和 `pullback30_max=0.05` 这类更强的假突破过滤。
- 但 per-symbol 也把 BTC 的收益削弱到 `+0.07% ~ +0.56%`，低于全局规则下的 `+0.88% ~ +1.12%`。这说明 BTC 当前不应过早引入更窄的 symbol-specific 规则。
- ETH raw / 2bps-slip 口径已经接近或转正，但 realistic 仍负，下一步必须针对 ETH 降低 InitialSL 损耗或提高 selected event 的每笔期望，而不是继续只调 stop 宽度。

## Execution Rescue：早入场 + 早 trailing

在不重建事件表的前提下，追加了 `original_t2` 的执行层小矩阵。关键参数：

- `entry_delay_seconds=5`
- `initial_stop_atr=0.45`
- `breakeven_at_r=0.8`
- `trail_start_r=0.9`
- `max_hold_hours=4`

结果：

| Quality Scope | Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD |
|---|---|---:|---:|---:|---:|---:|---:|---:|
| `global` | `BTCUSDT` | 58 | `+2.0715%` | `+3.2612%` | `+2.7841%` | `2.540186` | `74.14%` | `-0.3668%` |
| `global` | `ETHUSDT` | 55 | `-0.1533%` | `+0.9509%` | `+0.5079%` | `0.954977` | `60.00%` | `-0.7869%` |
| `per_symbol` | `BTCUSDT` | 68 | `+1.5229%` | `+2.9119%` | `+2.3545%` | `1.783076` | `69.12%` | `-0.5670%` |
| `per_symbol` | `ETHUSDT` | 73 | `+0.6264%` | `+2.1059%` | `+1.5117%` | `1.167483` | `65.75%` | `-0.7498%` |

等权组合粗略口径：

- `global quality + delay5/be0.8/trail0.9`：`(+2.0715 - 0.1533) / 2 = +0.9591%`
- `per_symbol quality + delay5/be0.8/trail0.9`：`(+1.5229 + 0.6264) / 2 = +1.07465%`

对照探索：

- 更紧 quality sweep（`speed60_min` 扩到 `0.12/0.16`、`pullback30_max` 扩到 `0.02`、`min_events=10`）没有改善 ETH。`baseline_plus_t3 + ETH + base` 仍为 `-0.8151%`，`original_t2 + ETH + base` 为 `-1.5415%`。
- `entry_delay_seconds=5` 是有效方向：ETH `original_t2 + per_symbol + base` 从 `-0.4083%` 改善到 `-0.1925%`。
- `entry_delay_seconds=30` 变差，说明继续等待确认会错过 edge 或增加被动挨打概率。
- 早 trailing 是本轮真正增益点：`trail_start_r=0.9` 明显优于 `1.2`，而 `breakeven_at_r=0.6 + trail_start_r=1.2` 表现很差（ETH `-0.9775%`），说明问题不是单纯更早 breakeven，而是更早把 MFE 转成 trailing exit。

## Probability / EV Model 回归

不完全丢弃概率模型。V3 的问题不是“概率无效”，而是概率模型直接绑到了进场、仓位、HMM regime multiplier 和退出逻辑里，无法判断 edge 来自哪里。V4 新增：

- `research/probabilistic_v4_probability_model.py`
  - 训练层只做 post-touch `continuation` probability。
  - Markov transition LLR 不再直接 gate 进场，而是先转成 `markov_prob_success`，再作为 logistic feature 进入概率模型。
  - 输出 `prob_success` 和 `prob_ev_atr`，其中 `prob_ev_atr` 用训练段的 success / non-success first-edge payoff，再扣掉每个事件自己的 `roundtrip_cost_atr`。
  - 通过验证段 sweep `prob_min` 和 `ev_atr_min` 选择 `quality_pass`。

- `research/probabilistic_v4_matrix_runner.py`
  - 新增 `--quality-mode probability`。
  - 同一 matrix runner 可在 `rule` 和 `probability` quality layer 之间切换。
  - summary 新增 equal-weight portfolio rows。

Jan-Apr `original_t2 + probability global + delay5/be0.8/trail0.9`：

| Symbol | Selected | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| `BTCUSDT` | 65 | 56 | `+1.9190%` | `+3.0659%` | `+2.6059%` | `2.299405` | `78.57%` | `-0.6456%` |
| `ETHUSDT` | 60 | 49 | `+0.8553%` | `+1.8484%` | `+1.4500%` | `1.344764` | `69.39%` | `-0.7943%` |

等权组合粗略口径：`(+1.9190 + 0.8553) / 2 = +1.38715%`。

概率模型选中的全局阈值：

- `prob_min=0.55`
- `ev_atr_min=0.0`
- validation selected：47 events，success `72.3404%`，avg net first edge `+0.173923 ATR`
- validation baseline：298 events，success `34.5638%`，avg net first edge `-0.088457 ATR`

校准分桶：

| Prob Bin | Events | Avg Prob | Realized Success | Net Edge ATR |
|---|---:|---:|---:|---:|
| `0.0-0.2` | 43 | `0.112031` | `6.9767%` | `-0.301297` |
| `0.2-0.4` | 141 | `0.293453` | `25.5319%` | `-0.141831` |
| `0.4-0.6` | 78 | `0.496287` | `51.2821%` | `+0.026227` |
| `0.6-0.8` | 32 | `0.662665` | `62.5000%` | `+0.095881` |
| `0.8-1.0` | 4 | `0.893263` | `100.0000%` | `+0.369956` |

重要解释：

- 概率分桶是单调的：预测概率越高，真实 success rate 和 net edge 都越高。这说明概率模型有可利用信息。
- 主要系数里 `markov_llr` 和 `markov_prob_success` 都是正权重，但不是唯一主导。更强的信号来自 `pullback_60s_atr`、`dwell_30s_pass`、`speed_60s_atr` 等结构/执行相关特征。
- 这正是 V3 应该被改造的地方：Markov 概率不是 entry trigger，而是 event probability 的一个输入；最终用 probability + EV 阈值做质量筛选。
- per-symbol probability 这轮反而较弱：BTC `+0.8314%`、ETH `-0.8649%`。因此当前晋级候选应先用 global probability，而不是 per-symbol probability。

## 下一阶段推进计划

1. BTC 候选先晋级验证，不再继续在 Jan-Apr 内微调：
   - 主候选：`original_t2 + probability global + delay5/be0.8/trail0.9`
   - 对照候选：`original_t2 + rule global + delay5/be0.8/trail0.9`
   - 下一步跑 BTC 2025 全年、BTC 2026 walk-forward split、以及至少一个反向市场段，验证是否只是 2026 Q1/Q2 的单窗口收益。

2. ETH 走单独救火线，不和 BTC 共用候选：
   - 当前主候选也切到 `original_t2 + probability global + delay5/be0.8/trail0.9`，ETH Jan-Apr 已到 `+0.8553%`。
   - 下一步不是继续加紧 hard rule，而是把 execution-aware target 加进 probability model：当前 probability EV 仍只看 first-edge ATR，下一步要把 InitialSL 风险或 simulated R outcome 纳入 selection objective。
   - 不再优先加宽 stop；本轮 `wider_stop` 已经系统性恶化。

3. 执行层下一刀：
   - 把 `delay5/be0.8/trail0.9` 加入 matrix runner 的标准 execution variant。
   - 加一个 `initial_sl_risk_filter` 或 `mae_proxy`：只交易 touch 后 5/15/30 秒内回撤小、且 entry-delay 后仍有足够净 R 的事件。
   - summary 增加 combined portfolio row，避免只看单 symbol row 时误判组合收益。

4. 晋级 / 淘汰标准：
   - BTC 候选必须跨年度保持 realistic 为正，且 PF 不低于 `1.2`。
   - ETH 候选已经在 Jan-Apr 拉到 `>= 0`，但必须先过 ETH 2025 全年 / 2026 walk-forward；不过线就只作为执行层线索保留。
   - 如果 Markov LLR 在下一轮仍不进入 top rule，应把 Markov 从“主模型”降级为诊断特征，避免为了概率模型名义继续堆复杂度。

## OOS 验证进展

为避免把训练段收益混入回测，本轮补了 out-of-sample 执行过滤：

- `research/probabilistic_v4_execution_runner.py`
  - 新增 `--execute-start` / `--execute-end`，执行层只消费指定 touch-time 窗口内的 selected events。
- `research/probabilistic_v4_matrix_runner.py`
  - 透传 `--execute-start` / `--execute-end`，并在 summary 中写出执行窗口。

同时修复 2025 zip 数据兼容：

- `research/order_flow_imbalance_breakout.py`
  - 2025 Binance trade zip 的 `time` 是微秒级，原 reader 按毫秒比较导致 2025 数据被全部跳过。
  - 现在 loader 检测到微秒级 timestamp 后会转成毫秒，已验证 `BTCUSDT-trades-2025-10.zip` 第一笔正确落到 `2025-10-01 00:00:00.181Z`。

### 2026 Mar-Apr OOS

设置：

- events：`research/probabilistic_v4_runs/btc_eth_2026_jan_apr_v4_narrow/original_t2/events.csv`
- train：`2026-01-01` 到 `2026-02-28`
- execute：`2026-03-01` 到 `2026-04-30`
- quality：`probability global`
- selected threshold：`prob_min=0.55`, `ev_atr_min=0.05`
- execution：`delay5/be0.8/trail0.9`

结果：

| Symbol | Selected | Trades | Realistic | PF | Win | DD |
|---|---:|---:|---:|---:|---:|---:|
| `BTCUSDT` | 22 | 22 | `-0.0449%` | `0.958659` | `63.64%` | `-0.6048%` |
| `ETHUSDT` | 19 | 17 | `+0.5985%` | `1.954516` | `70.59%` | `-0.3175%` |

等权：`+0.2768%`。

解释：模型校准仍然单调，selected validation 41 events 的 success `70.7317%`、avg net first edge `+0.176427 ATR`；但 BTC 执行收益只打平，说明 Jan-Apr 全段收益里有一部分来自训练段或窗口结构。不能把 `+1.38715%` 直接当稳定收益。

### 2025 Q4 OOS

设置：

- run：`research/probabilistic_v4_runs/btc_eth_2025_q4_oos_probability_original_t2/summary.md`
- train：`2025-10-01` 到 `2025-11-30`
- execute：`2025-12-01` 到 `2025-12-31`
- quality：`probability global`
- selected threshold：`prob_min=0.55`, `ev_atr_min=0.05`
- execution：`delay5/be0.8/trail0.9`

严格阈值结果：

| Symbol | Selected | Trades | Realistic | PF | Win | DD |
|---|---:|---:|---:|---:|---:|---:|
| `BTCUSDT` | 5 | 5 | `-0.0893%` | `0.723361` | `40.00%` | `-0.3215%` |
| `ETHUSDT` | 21 | 19 | `+0.5706%` | `1.990462` | `68.42%` | `-0.3636%` |

等权：`+0.24065%`。BTC 样本太小，不能单独裁决。

放宽样本约束后：

- run：`research/probabilistic_v4_runs/btc_eth_2025_q4_oos_probability_original_t2_relaxed/`
- selection：`prob_min=0.4`, `ev_atr_min=0.05`, `min_events=40`

| Symbol | Selected | Trades | Realistic | PF | Win | DD |
|---|---:|---:|---:|---:|---:|---:|
| `BTCUSDT` | 9 | 8 | `+0.0434%` | `1.135201` | `62.50%` | `-0.3215%` |
| `ETHUSDT` | 33 | 28 | `+0.8569%` | `1.895066` | `67.86%` | `-0.4142%` |

等权：`+0.45015%`。

当前判断：

- 概率模型在 OOS 上没有崩：2026 Mar-Apr、2025 Dec 两个外样本等权都为正。
- 但 BTC 不够强：OOS 里 BTC 是打平或小正，PF 未稳定超过 `1.2`。
- ETH 反而更稳定：两个 OOS 都明显为正，说明 probability + early trailing 对 ETH 的修复更可信。
- 下一步必须做多折 walk-forward，不能只看 Jan-Apr 全段；尤其要确认 BTC 是不是只在 2026 Feb 的训练窗口里贡献了主要收益。

## 注意

当前 V4 execution runner 仍是第一版简单执行层：只消费 selected scored events，使用 1s OHLC、fixed slippage/fees、breakeven 与 per-second trailing。它已经补齐 monthly attribution、profit factor、exit reason contribution 和参数矩阵输出，但还不是最终退出模型。下一步重点应放在 BTC 跨期验证、ETH entry-risk 过滤、以及 portfolio-level 汇总，而不是继续把更多 regime/HMM/Markov 逻辑塞回一个大 runner。

## V5/V6：概率模型与仓位控制推进

用户明确指出：不能只围绕原 Markov 阈值继续做 `1%` 附近的小优化，概率模型应该真正参与组合和仓位，实盘候选也必须朝 `10%~20%` 回测收益推进。本轮因此新增两层：

- `research/probabilistic_v5_ml_probability_model.py`
  - 模型族扩到 `logistic`、`random_forest`、`extra_trees`、`gradient_boosting`、`svm_rbf`。
  - Markov 改为 `5/15/30/60s` 小窗口 order-flow state LLR / probability 特征，不再只是单一 quality threshold。
  - 输出 `model_notional_share`，用 EV、probability、Markov score 做连续 sizing。
  - sizing calibration 只使用 train+validation selected events，避免 test 分布泄漏。

- `research/probabilistic_v6_execution_labeler.py`
  - 用 V4 1s execution runner 对每个 event 独立标注真实执行收益。
  - 概率模型训练目标从 first-edge continuation 改成 execution win/loss。
  - 原始 first-edge 字段保存到 `original_*` 列，模型用 `execution_return_pct` 作为新的 edge target。

关键结果见：

- `research/20260509_probabilistic_v5_v6_execution_aware.md`

摘要：

| 模型 | OOS | Symbol | Trades | Realistic | 结论 |
|---|---|---|---:|---:|---|
| V5 ML dynamic | 2026 Mar | `BTCUSDT` | 4 | `-0.5480%` | 不通过 |
| V5 ML dynamic | 2026 Mar | `ETHUSDT` | 9 | `-0.2590%` | 不通过 |
| V5 ML dynamic | 2025 Dec | `BTCUSDT` | 3 | `+0.3649%` | 样本太小 |
| V5 ML dynamic | 2025 Dec | `ETHUSDT` | 10 | `-1.5010%` | 不通过 |
| V6 execution-aware per-symbol dynamic | 2026 Mar | `BTCUSDT` | 33 | `-1.1130%` | BTC 禁用 dynamic |
| V6 execution-aware per-symbol dynamic | 2026 Mar | `ETHUSDT` | 17 | `+1.5450%` | 单窗有效，需跨期验证 |
| V6 execution-aware per-symbol fixed 20% | 2026 Mar | `BTCUSDT` | 33 | `-0.1649%` | 接近打平但不通过 |
| V6 execution-aware per-symbol fixed 20% | 2026 Mar | `ETHUSDT` | 17 | `+0.1438%` | edge 主要来自 dynamic sizing |

2025 Dec 的 execution-aware per-symbol model 没有找到正的外样本子集：BTC test label sum `-1.909753`，ETH test label sum `-0.613579`。

当前判断：

- 概率模型有用，尤其是 execution-aware target + ETH dynamic sizing。
- 但还没到实盘候选：没有跨期稳定，也远不到 `10%~20%`。
- BTC 和 ETH 必须分开处理；BTC 当前不能启用 aggressive dynamic sizing。
- 下一阶段应做 2025 全年月度 walk-forward、per-symbol portfolio gating、以及 `InitialSL` 风险 classifier，而不是继续只调 Markov threshold。

## V7 Follow-up：概率模型继续推进后的修正结论

本轮继续把概率模型用于 execution-aware selection、top-K、动态仓位和 InitialSL 风险，但额外发现并清理了一个关键研究语义问题：当前未闭合 signal bar 的完整 `signal_close/signal_high/signal_low`、以及完整当前 bar ATR，不能作为 intrabar touch 当刻的模型输入。

已落地的 research-only 改动：

- V5 ML probability model 增加 point-in-time touch 上下文和上一根已闭合 bar 上下文。
- V6 walk-forward runner 增加 validation-based top-K 选择与验证 top-K 风险字段。
- V4 event dataset 改为优先使用 `prev_atr_1`，并输出已闭合 bar 上下文。
- V4 execution runner / order-flow helper 增加缺失旧 helper 时的 fallback，保证 research harness 可复跑。

验证结论：

| 口径 | Run | 结果 | 判断 |
|---|---|---:|---|
| 使用当前 signal 完整 OHLC | `2025_q3/q4, 2026_q1 v7_signalctx` | silo sum `+4.8130% / +13.7941% / +12.7492%` | 作废，存在 lookahead |
| point-in-time feature | `walkforward_2025_q3_v7_pointintime_valbest_probev` | ETH `-1.9905%` | 不通过 |
| point-in-time feature | `walkforward_2026_q1_v7_pointintime_valbest_probev` | BTC `-1.4667%`, ETH `+1.4629%` | 组合打平，不通过 |
| point-in-time feature | `walkforward_2025_q4_v7_pointintime_valbest_probev` | BTC `-3.7057%`, ETH gated out | 不通过 |
| `prev_atr_1` event rebuild | `walkforward_2025_q3_v7_prevatr_valbest_probev` | ETH `-2.6501%` | 不通过 |

因此，当前推进计划调整为：

1. `original_t2` 单结构不再作为 10%~20% 候选主线，除非后续 regime gate 能明确过滤 2025-09 / 2025-12。
2. 下一阶段应重建事件来源组合：`baseline_plus_t3`、volatility-regime 独立事件表、以及 portfolio-level no-trade gate。
3. 概率模型继续保留为 execution-aware selector / sizing controller，但不能用当前 signal bar 完整 OHLC 作为特征。
4. 所有后续报告必须显式区分 `作废 lookahead result` 与 `point-in-time result`。

## V8 Direction：delay60 合法概率路线

用户再次强调“概率模型要用起来”，且不应只在 `1%` 附近反复微调。本轮因此把两条事分开：

1. `baseline_plus_t3` 是否能作为更强事件源。
2. 原 `original_t2` 是否能通过合法小窗口确认和动态 sizing 改善。

### baseline_plus_t3 复核

Q3 `baseline_plus_t3` point-in-time ATR 事件重建后，`t3_swing` 原始分布仍偏弱：

| Symbol | Shape | continuation | fail | timeout |
|---|---|---:|---:|---:|
| `BTCUSDT` | `t3_swing` | 38 | 86 | 2 |
| `ETHUSDT` | `t3_swing` | 51 | 103 | 0 |

execution label 也没有转正：

| Symbol | Shape | Tradable | Wins | Avg Execution Return |
|---|---|---:|---:|---:|
| `BTCUSDT` | `t3_swing` | 91 | 42 | `-0.060973%` |
| `ETHUSDT` | `t3_swing` | 74 | 40 | `-0.041829%` |

对应 walk-forward：

- `research/probabilistic_v6_runs/walkforward_2025_q3_baseline_plus_t3_v7_prevatr_valbest_probev`
- ETH validation 选中 top15，但 2025-09 真实执行 `-3.9517%`。

结论：此前“`baseline_plus_t3` 信号偏弱”的判断仍成立。概率模型不能把这批 Q3 事件源救成候选。

### entry5 下的 pullback60 作废

feature-slice 诊断发现 `side=short & pullback_60s_atr:q2` label 累计接近 `+9.7049%`，但 `pullback_60s_atr` 对 `entry_delay=5s` 是未来信息。因此该结果只能提示“延迟确认可能有用”，不能作为 5 秒入场证据。

真实 fixed20 执行验证也没有通过：

| Window | Silo Sum |
|---|---:|
| 2025 Q3 | `-1.4623%` |
| 2025 Q4 | `-0.3355%` |
| 2026 Jan-Mar | `+0.9422%` |

### delay60 合法重建

新增/调整：

- V4 event dataset 复用 `--dwell-seconds 60` 生成合法 60 秒确认字段。
- V5 ML model 支持 `dwell_60s_*` / `pullback_60s_atr`。
- V6 walk-forward runner 增加 `--feature-horizon-seconds`，并禁止 `feature_horizon_seconds > entry_delay_seconds`。
- 新增 feature-slice 与 no-trade gate analyzer。

delay60 事件与 label：

| Dataset | Events | Delay60 Tradable Labels |
|---|---:|---:|
| 2025 Q3 | 1023 | 314 |
| 2025 Q4 | 1063 | 293 |
| 2026 Jan-Apr | 993 | 194 |

最佳合法 label slice：

- `side=short & speed_60s_atr:q4>0.259108`
- label sum `+10.1355%`
- positive months `7/9`
- InitialSL rate `33.33%`

固定规则 fixed20 执行仍不足：

| Window | Silo Sum |
|---|---:|
| 2025 Q3 | `-0.1642%` |
| 2025 Q4 | `+0.1582%` |
| 2026 Jan-Apr | `+0.8427%` |

概率模型 walk-forward：

- Run: `research/probabilistic_v6_runs/walkforward_delay60_original_t2_feature60_valbest`
- train 2 months / validation 1 month / execute 1 month
- `entry_delay_seconds=60`
- `feature_horizon_seconds=60`
- `hybrid_markov` dynamic sizing

active 月度结果：

| Execute Month | Silo Return |
|---|---:|
| 2025-11 | `+0.4103%` |
| 2025-12 | `-0.7900%` |
| 2026-01 | `+0.1137%` |
| 2026-02 | `+2.1110%` |
| 2026-03 | `+1.7533%` |

合计 `+3.5983%`，69 笔。按 symbol：BTC `+1.5930%`，ETH `+2.0053%`。

post-hoc no-trade gate 诊断：

- `research/probabilistic_v6_runs/walkforward_delay60_original_t2_feature60_valbest/no_trade_gate_scan_overfit.md`
- baseline `+3.5983%`
- best non-empty `+6.0939%`
- 主要通过挡掉 ETH 2026-01/02 与 BTC 2026-03 改善。

已把 gate 正式接入 V6 runner：

- `--min-validation-topk-return-over-dd`
- `--max-validation-topk-return-over-dd`
- `--max-validation-topk-return-pct`
- `--validation-topk-gate-stage`

两种 gate stage 结果：

| Stage | Run | Active Rows | Trades | Silo Sum | 说明 |
|---|---|---:|---:|---:|---|
| `candidate_filter` | `walkforward_delay60_original_t2_feature60_formal_gate` | 6 | 57 | `+4.7201%` | 过热 top15 被过滤后允许 fallback 到 BTC 2026-03 top10，仍亏 |
| `post_selection` | `walkforward_delay60_original_t2_feature60_postselect_gate` | 5 | 51 | `+6.0939%` | 先选 topK，再对被选 topK 做 gate；与 post-hoc 诊断一致 |

`post_selection` 月度：

| Execute Month | Silo Return |
|---|---:|
| 2025-11 | `+0.4103%` |
| 2025-12 | `-0.7900%` |
| 2026-01 | `+0.3375%` |
| 2026-02 | `+3.0090%` |
| 2026-03 | `+3.1271%` |

关键解释：

- `post_selection` 能挡掉 ETH 2026-01/02：validation return/DD 不足。
- `post_selection` 能挡掉 BTC 2026-03：validation topK return 过热。
- BTC 2025-12 仍挡不住：validation topK return、return/DD、InitialSL rate 都不极端，但执行月转负。这需要市场 regime 特征。

下一阶段计划：

1. 给 delay60 runner 增加 market-regime 特征，只允许使用 execute 前已闭合的日线/小时线统计。
2. 用更长时间窗重跑 delay60，至少覆盖 2025 全年和 2026 Jan-Apr。
3. 对 ETH 小样本 sleeve 继续保留 `return/DD` gate，但不要再只靠 InitialSL gate。
4. 若正式 OOS gate 仍只能到 `6%` 左右，则必须增加新的事件源，而不是继续在 `original_t2` 上调模型。
