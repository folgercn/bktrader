# Probabilistic V6 Union Selector Sweep

范围：仅限 `research`。本报告扫描 validation-only selector，并对入选候选执行事件级 union 回测。

## Caveat

- Gate 字段只来自 validation period；`test_edge` 不参与筛选。
- 候选排序仍然使用 execute-period 结果做探索性 post-selection；命中 10% 只能作为下一轮 holdout 候选，不是实盘结论。
- 当前 union runner 是 one-shot 1s execution，还不是完整 `reentry_window` 生命周期。

## Summary

- candidate_rows: `38`
- scanned_unique_selections: `1522`
- emitted_candidates: `8`
- union_executed: `True`

## Candidate Results

| Rank | Key | Policy | Rows | Months | Trades | Row Sum | Row Worst Month | Union Sum | Union Worst Silo | Union Trades | Target | Gate |
|---:|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| 1 | `candidate_001` | `all_sleeves` | 17 | 9 | 141 | 16.9298% | -1.3744% | 10.8467% | -1.3744% | 115 | `True` | edge>=0.05, ret>=0.5%, SL<=1.0, DD<=100.0%, ret/DD=-999.0..10.0, ret<=999.0%, markov=0.4..0.9 |
| 2 | `candidate_002` | `all_sleeves` | 16 | 8 | 136 | 16.6854% | -1.3744% | 10.6023% | -1.3744% | 110 | `True` | edge>=0.05, ret>=0.5%, SL<=1.0, DD<=100.0%, ret/DD=-999.0..10.0, ret<=999.0%, markov=0.4..0.85 |
| 3 | `candidate_003` | `all_sleeves` | 20 | 9 | 163 | 16.6290% | -1.3744% | 10.5743% | -1.3744% | 132 | `True` | edge>=0.05, ret>=0.5%, SL<=1.0, DD<=100.0%, ret/DD=-999.0..10.0, ret<=999.0%, markov=-999.0..0.9 |
| 4 | `candidate_004` | `all_sleeves` | 19 | 8 | 158 | 16.3846% | -1.3744% | 10.3299% | -1.3744% | 127 | `True` | edge>=0.05, ret>=0.5%, SL<=1.0, DD<=100.0%, ret/DD=-999.0..10.0, ret<=999.0%, markov=-999.0..0.85 |
| 5 | `candidate_005` | `all_sleeves` | 19 | 10 | 147 | 16.3843% | -1.3744% | 10.3012% | -1.3744% | 121 | `True` | edge>=0.05, ret>=0.5%, SL<=1.0, DD<=100.0%, ret/DD=-999.0..20.0, ret<=999.0%, markov=0.4..0.9 |
| 6 | `candidate_006` | `all_sleeves` | 18 | 9 | 142 | 16.1399% | -1.3744% | 10.0568% | -1.3744% | 116 | `True` | edge>=0.05, ret>=0.5%, SL<=1.0, DD<=100.0%, ret/DD=-999.0..20.0, ret<=999.0%, markov=0.4..0.85 |
| 7 | `candidate_008` | `all_sleeves` | 22 | 10 | 169 | 16.0835% | -1.3744% | 10.0288% | -1.3744% | 138 | `True` | edge>=0.05, ret>=0.5%, SL<=1.0, DD<=100.0%, ret/DD=-999.0..20.0, ret<=999.0%, markov=-999.0..0.9 |
| 8 | `candidate_007` | `all_sleeves` | 14 | 8 | 118 | 16.1127% | -1.3744% | 9.7496% | -1.3744% | 94 | `False` | edge>=0.0, ret>=0.5%, SL<=0.3, DD<=100.0%, ret/DD=-999.0..10.0, ret<=999.0%, markov=0.4..0.9 |
