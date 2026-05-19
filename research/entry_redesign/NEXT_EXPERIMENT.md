# 下一步实验：V6 Gate 过滤后 D=5 Tick 回测

## 背景

- `original-t2-entry-logic-redesign` spec 已证伪：entry 层单独改造无法让事件期望变正
- V6 `power0_fixed_1p30` 在 D=60 下产出 calendar sum +33.02%（310 trades，10 active silos）
- Tick 回测证实 D=0 比 D=60 好很多（全量 1303 events：D=0 raw +54% vs D=60 raw -67%）
- 假设：如果把 V6 的 D=60 换成 D=5，在 gate 过滤后的高质量 events 上可能超过 33%

## 实验目标

在 V6 candidate_001 gate 过滤后的 events 上，用 tick 数据跑 D=5（替代 D=60），验证缩短 delay 能否提升收益。

## 数据位置

- V6 lifecycle ledger: `research/probabilistic_v6_runs/walkforward_2025m06_2026apr_combo_baseline_short_speed/union_lifecycle_reentry_window_candidate_001_calendar_holdout/power0_fixed_1p30/execute_*/*/lifecycle_ledger.csv`
- 全量 events: `research/probabilistic_v6_runs/2025m03_2026apr_original_t2_delay60/events_execution_labeled.csv`（1303 events）
- Tick 数据: `dataset/archive/{SYMBOL}-trades-{YYYY}-{MM}/` (csv 或 zip)
- V4 execution runner: `research/probabilistic_v4_execution_runner.py`

## 实验步骤

1. 从 V6 lifecycle ledger 中提取被 gate 选中的 event 列表（186 entries 对应的 touch_time）
2. 匹配回 `events_execution_labeled.csv` 中的原始 event（通过 symbol + touch_time 匹配）
3. 对这些 events 用 tick 数据跑 D=5 的 V4 execution model（params: initial_stop_atr=0.45, breakeven_at_r=0.8, trail_start_r=0.9, max_hold_hours=4）
4. 套上 reentry_window lifecycle sizing（slot0=20%, slot1=10%, max_trades_per_bar=2）
5. 计算 calendar sum 并与 33.02% 对比

## V4 Execution 参数

```python
exec_params = {
    'initial_stop_atr': 0.45,
    'breakeven_at_r': 0.8,
    'trail_start_r': 0.9,
    'trail_buffer_atr': 0.05,
    'max_hold_hours': 4.0,
    'min_stop_bps': 12.0,  # 重要！之前漏了这个
    'slippage': 0.0002,  # 2 bps/side
    'entry_fee': 0.0002,  # maker 2 bps
    'exit_fee': 0.0004,  # taker 4 bps
}
```

## Tick 数据格式

- 2026 CSV (有 header): `id,price,qty,quote_qty,time(ms),is_buyer_maker`
- 2025 ZIP (无 header): `id,price,qty,quote_qty,time_us,is_buyer_maker,is_best_match`
- 部分 2026 ZIP 有 header（如 ETHUSDT-2026-02）

## 关键注意事项

1. **必须加 min_stop_bps=12 过滤**：stop distance < 12 bps 的 event 不入场
2. **entry_time = touch_time + D 秒**：不是 touch_time + 0
3. **entry_price = D 秒后第一个 tick 的 price**：不是 level
4. **不要前视偏差**：任何 gate/filter 使用的信息必须在 entry_time 之前已知
5. **Sizing**: reentry_window 模式下 slot0=20%, slot1=10%

## 新会话 Prompt

```
读一下 research/entry_redesign/NEXT_EXPERIMENT.md，按照里面的实验计划执行。
用 tick 数据对 V6 gate 过滤后的 events 跑 D=5 回测，对比 V6 的 33% baseline。
```
