# Micro Breakout Structure 1s 回测（2025-01-01T00:00:00+00:00 至 2025-12-31T23:59:59+00:00）

范围：仅限 `research`。这是 closed-bar breakout proxy：先用聚合后的 signal-bar close 识别突破候选，再进入连续 `1s OHLC` 执行段。它不是 live 风格的三根 bar intrabar breakout；真实结构突破里第三根 bar 仍未闭合，由当前 bar 内 `1s high/low` 触碰结构 level 触发。高周期趋势过滤由 variant 控制；进场仓位根据近期 `1s` speed/efficiency 调整；结构退出在达到配置的 ATR 盈利阈值后，沿已完成 signal-bar 结构移动止损。

| Symbol | Variant | Trades | Realistic | Raw | 2bps Slip No Fee | Win Rate | Max DD | Avg Share | Med Hold | Med MFE ATR | Exits | Quality | Cands | Entries | Weak Skip | Busy |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---|---:|---:|---:|---:|
| `ETHUSDT` | `def_s10b2` | 112 | 7.5149% | 10.7631% | 9.4529% | 50.00% | -2.4915% | 0.2661 | 7272.00s | 0.6646 | `{'InitialSL': 41, 'BreakevenSL': 25, 'StructureSL': 23, 'NoNewHighExit': 13, 'NoNewLowExit': 5, 'MaxHoldExit': 5}` | `{'strong': 74, 'base': 38}` | 261 | 112 | 143 | 6 |
| `ETHUSDT` | `s08b2` | 112 | 7.5869% | 10.8372% | 9.5261% | 50.00% | -2.4915% | 0.2661 | 7272.00s | 0.6646 | `{'InitialSL': 41, 'BreakevenSL': 24, 'StructureSL': 24, 'NoNewHighExit': 13, 'NoNewLowExit': 5, 'MaxHoldExit': 5}` | `{'strong': 74, 'base': 38}` | 261 | 112 | 143 | 6 |
| `ETHUSDT` | `s12b2` | 112 | 7.5149% | 10.7631% | 9.4529% | 50.00% | -2.4915% | 0.2661 | 7272.00s | 0.6646 | `{'InitialSL': 41, 'BreakevenSL': 25, 'StructureSL': 23, 'NoNewHighExit': 13, 'NoNewLowExit': 5, 'MaxHoldExit': 5}` | `{'strong': 74, 'base': 38}` | 261 | 112 | 143 | 6 |
| `ETHUSDT` | `s08b3` | 112 | 8.2708% | 11.5417% | 10.2223% | 50.00% | -2.2724% | 0.2661 | 7272.00s | 0.6646 | `{'InitialSL': 41, 'BreakevenSL': 27, 'StructureSL': 14, 'NoNewHighExit': 13, 'MaxHoldExit': 12, 'NoNewLowExit': 5}` | `{'strong': 74, 'base': 38}` | 261 | 112 | 143 | 6 |
| `ETHUSDT` | `s10b3` | 112 | 8.2606% | 11.5312% | 10.2120% | 50.00% | -2.2724% | 0.2661 | 7272.00s | 0.6646 | `{'InitialSL': 41, 'BreakevenSL': 28, 'NoNewHighExit': 13, 'StructureSL': 13, 'MaxHoldExit': 12, 'NoNewLowExit': 5}` | `{'strong': 74, 'base': 38}` | 261 | 112 | 143 | 6 |
| `ETHUSDT` | `s12b3` | 112 | 8.2606% | 11.5312% | 10.2120% | 50.00% | -2.2724% | 0.2661 | 7272.00s | 0.6646 | `{'InitialSL': 41, 'BreakevenSL': 28, 'NoNewHighExit': 13, 'StructureSL': 13, 'MaxHoldExit': 12, 'NoNewLowExit': 5}` | `{'strong': 74, 'base': 38}` | 261 | 112 | 143 | 6 |
| `ETHUSDT` | `s10b4` | 112 | 8.9056% | 12.1957% | 10.8685% | 50.00% | -1.6690% | 0.2661 | 7272.00s | 0.6646 | `{'InitialSL': 41, 'BreakevenSL': 30, 'MaxHoldExit': 17, 'NoNewHighExit': 13, 'StructureSL': 6, 'NoNewLowExit': 5}` | `{'strong': 74, 'base': 38}` | 261 | 112 | 143 | 6 |
| `ETHUSDT` | `loose_micro` | 114 | 7.0384% | 10.3825% | 9.0334% | 50.00% | -2.7437% | 0.2702 | 7208.50s | 0.6646 | `{'InitialSL': 42, 'BreakevenSL': 26, 'StructureSL': 23, 'NoNewHighExit': 13, 'NoNewLowExit': 5, 'MaxHoldExit': 5}` | `{'strong': 80, 'base': 34}` | 261 | 114 | 141 | 6 |
| `ETHUSDT` | `tight_micro` | 108 | 7.4382% | 10.5182% | 9.2764% | 50.00% | -2.6091% | 0.2620 | 7272.00s | 0.6646 | `{'InitialSL': 39, 'BreakevenSL': 25, 'StructureSL': 21, 'NoNewHighExit': 13, 'NoNewLowExit': 5, 'MaxHoldExit': 5}` | `{'strong': 67, 'base': 41}` | 261 | 108 | 147 | 6 |
| `ETHUSDT` | `share25` | 112 | 6.3730% | 8.9757% | 7.9277% | 50.00% | -2.0083% | 0.2161 | 7272.00s | 0.6646 | `{'InitialSL': 41, 'BreakevenSL': 25, 'StructureSL': 23, 'NoNewHighExit': 13, 'NoNewLowExit': 5, 'MaxHoldExit': 5}` | `{'strong': 74, 'base': 38}` | 261 | 112 | 143 | 6 |
| `ETHUSDT` | `share35` | 112 | 9.0470% | 12.7565% | 11.2587% | 50.00% | -2.7597% | 0.2991 | 7272.00s | 0.6646 | `{'InitialSL': 41, 'BreakevenSL': 25, 'StructureSL': 23, 'NoNewHighExit': 13, 'NoNewLowExit': 5, 'MaxHoldExit': 5}` | `{'strong': 74, 'base': 38}` | 261 | 112 | 143 | 6 |
| `ETHUSDT` | `hold14` | 112 | 8.5932% | 11.8745% | 10.5504% | 50.00% | -2.4915% | 0.2661 | 7272.00s | 0.6646 | `{'InitialSL': 41, 'StructureSL': 26, 'BreakevenSL': 25, 'NoNewHighExit': 13, 'NoNewLowExit': 5, 'MaxHoldExit': 2}` | `{'strong': 74, 'base': 38}` | 261 | 112 | 143 | 6 |

## Files

- Summary JSON: `research/eth_2025_micro_breakout_structure_narrow_sweep_summary.json`
- `ETHUSDT def_s10b2` ledger: `research/tmp_eth_2025_micro_breakout_structure_narrow_sweep_ETHUSDT_def_s10b2_ledger.csv`
- `ETHUSDT s08b2` ledger: `research/tmp_eth_2025_micro_breakout_structure_narrow_sweep_ETHUSDT_s08b2_ledger.csv`
- `ETHUSDT s12b2` ledger: `research/tmp_eth_2025_micro_breakout_structure_narrow_sweep_ETHUSDT_s12b2_ledger.csv`
- `ETHUSDT s08b3` ledger: `research/tmp_eth_2025_micro_breakout_structure_narrow_sweep_ETHUSDT_s08b3_ledger.csv`
- `ETHUSDT s10b3` ledger: `research/tmp_eth_2025_micro_breakout_structure_narrow_sweep_ETHUSDT_s10b3_ledger.csv`
- `ETHUSDT s12b3` ledger: `research/tmp_eth_2025_micro_breakout_structure_narrow_sweep_ETHUSDT_s12b3_ledger.csv`
- `ETHUSDT s10b4` ledger: `research/tmp_eth_2025_micro_breakout_structure_narrow_sweep_ETHUSDT_s10b4_ledger.csv`
- `ETHUSDT loose_micro` ledger: `research/tmp_eth_2025_micro_breakout_structure_narrow_sweep_ETHUSDT_loose_micro_ledger.csv`
- `ETHUSDT tight_micro` ledger: `research/tmp_eth_2025_micro_breakout_structure_narrow_sweep_ETHUSDT_tight_micro_ledger.csv`
- `ETHUSDT share25` ledger: `research/tmp_eth_2025_micro_breakout_structure_narrow_sweep_ETHUSDT_share25_ledger.csv`
- `ETHUSDT share35` ledger: `research/tmp_eth_2025_micro_breakout_structure_narrow_sweep_ETHUSDT_share35_ledger.csv`
- `ETHUSDT hold14` ledger: `research/tmp_eth_2025_micro_breakout_structure_narrow_sweep_ETHUSDT_hold14_ledger.csv`
