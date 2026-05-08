# Micro Breakout Structure 1s 回测（2025-01-01T00:00:00+00:00 至 2025-12-31T23:59:59+00:00）

范围：仅限 `research`。这是 closed-bar breakout proxy：先用聚合后的 signal-bar close 识别突破候选，再进入连续 `1s OHLC` 执行段。它不是 live 风格的三根 bar intrabar breakout；真实结构突破里第三根 bar 仍未闭合，由当前 bar 内 `1s high/low` 触碰结构 level 触发。高周期趋势过滤由 variant 控制；进场仓位根据近期 `1s` speed/efficiency 调整；结构退出在达到配置的 ATR 盈利阈值后，沿已完成 signal-bar 结构移动止损。

| Symbol | Variant | Trades | Realistic | Raw | 2bps Slip No Fee | Win Rate | Max DD | Avg Share | Med Hold | Med MFE ATR | Exits | Quality | Cands | Entries | Weak Skip | Busy |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---|---:|---:|---:|---:|
| `ETHUSDT` | `s10b4_nohold` | 112 | 12.9616% | 16.3758% | 14.9969% | 50.00% | -2.3656% | 0.2661 | 7272.00s | 0.6646 | `{'InitialSL': 41, 'BreakevenSL': 30, 'StructureSL': 23, 'NoNewHighExit': 13, 'NoNewLowExit': 5}` | `{'strong': 74, 'base': 38}` | 261 | 112 | 142 | 7 |

## Files

- Summary JSON: `research/eth_2025_micro_breakout_structure_nohold_summary.json`
- `ETHUSDT s10b4_nohold` ledger: `research/tmp_eth_2025_micro_breakout_structure_nohold_ETHUSDT_s10b4_nohold_ledger.csv`
