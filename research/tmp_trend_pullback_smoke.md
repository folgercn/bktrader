# Trend Pullback Continuation 1s Replay

Scope: research-only. Execution uses continuous 1s OHLC bars built from local Binance trade ticks. Entry fills at triggering 1s close with 2 bps/side slippage. Accounting also includes maker entry 2 bps and market exit 4 bps.

| Symbol | Variant | Trades | Realistic | Raw | 2bps Slip No Fee | Win Rate | Max DD | Median Hold | Median MFE R | Exit Reasons | Setups | Touches | Invalid | Expired |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|
| `BTCUSDT` | `base` | 0 | 0.0000% | 0.0000% | 0.0000% | 0.00% | 0.0000% | 0.00s | 0.0000 | `{}` | 0 | 0 | 0 | 0 |

## Files

- Summary JSON: `research/tmp_trend_pullback_smoke_summary.json`
- `BTCUSDT base` ledger: `research/tmp_trend_pullback_smoke_BTCUSDT_base_ledger.csv`
