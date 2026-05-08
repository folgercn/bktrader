# Micro Breakout Structure 1s Replay (2026-01-01T00:00:00+00:00 to 2026-01-03T23:59:59+00:00)

Scope: research-only. Signals use closed 1h breakout bars; execution uses continuous 1s OHLC bars. Higher timeframe trend filtering is variant-controlled. Entry sizing is based on recent 1s speed/efficiency, and structure exits trail behind completed 1h bar structure after the configured ATR profit threshold.

| Symbol | Variant | Trades | Realistic | Raw | 2bps Slip No Fee | Win Rate | Max DD | Avg Share | Med Hold | Med MFE ATR | Exits | Quality | Cands | Entries | Weak Skip | Busy |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---|---:|---:|---:|---:|
| `ETHUSDT` | `signal_struct_skipweak` | 1 | 0.0060% | 0.0360% | 0.0240% | 100.00% | 0.0000% | 0.3000 | 2404.00s | 1.1691 | `{'BreakevenSL': 1}` | `{'strong': 1}` | 1 | 1 | 0 | 0 |

## Files

- Summary JSON: `research/tmp_micro_cache_smoke_summary.json`
- `ETHUSDT signal_struct_skipweak` ledger: `research/tmp_micro_cache_smoke_ETHUSDT_signal_struct_skipweak_ledger.csv`
