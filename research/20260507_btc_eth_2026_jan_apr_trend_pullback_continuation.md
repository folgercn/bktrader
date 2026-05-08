# Trend Pullback Continuation 1s Replay

Scope: research-only. Execution uses continuous 1s OHLC bars built from local Binance trade ticks. Entry fills at triggering 1s close with 2 bps/side slippage. Accounting also includes maker entry 2 bps and market exit 4 bps.

| Symbol | Variant | Trades | Realistic | Raw | 2bps Slip No Fee | Win Rate | Max DD | Median Hold | Median MFE R | Exit Reasons | Setups | Touches | Invalid | Expired |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|
| `BTCUSDT` | `base` | 15 | -0.0511% | 0.2492% | 0.1289% | 60.00% | -0.5198% | 6333.00s | 1.2287 | `{'TrailingSL': 7, 'InitialSL': 6, 'BreakevenSL': 2}` | 21 | 20 | 5 | 1 |
| `BTCUSDT` | `loose_pullback` | 14 | -0.2672% | 0.0125% | -0.0995% | 57.14% | -0.6854% | 5672.50s | 1.1129 | `{'InitialSL': 6, 'TrailingSL': 6, 'BreakevenSL': 2}` | 21 | 20 | 6 | 1 |
| `BTCUSDT` | `tight_reclaim` | 20 | -0.7418% | -0.3439% | -0.5032% | 45.00% | -0.7418% | 4139.00s | 0.8014 | `{'InitialSL': 11, 'TrailingSL': 5, 'BreakevenSL': 4}` | 25 | 24 | 4 | 1 |
| `ETHUSDT` | `base` | 11 | 0.3956% | 0.6166% | 0.5281% | 63.64% | -0.2804% | 10912.00s | 1.4983 | `{'TrailingSL': 5, 'InitialSL': 4, 'BreakevenSL': 2}` | 20 | 20 | 9 | 0 |
| `ETHUSDT` | `loose_pullback` | 10 | 0.1147% | 0.3151% | 0.2349% | 60.00% | -0.4990% | 11399.00s | 1.1774 | `{'InitialSL': 4, 'TrailingSL': 4, 'BreakevenSL': 2}` | 17 | 17 | 7 | 0 |
| `ETHUSDT` | `tight_reclaim` | 11 | 0.8719% | 1.0939% | 1.0050% | 72.73% | -0.1896% | 9734.00s | 1.5281 | `{'TrailingSL': 6, 'InitialSL': 3, 'BreakevenSL': 2}` | 20 | 20 | 9 | 0 |

## Files

- Summary JSON: `research/btc_eth_2026_jan_apr_trend_pullback_continuation_summary.json`
- `BTCUSDT base` ledger: `research/tmp_btc_eth_2026_jan_apr_trend_pullback_continuation_BTCUSDT_base_ledger.csv`
- `BTCUSDT loose_pullback` ledger: `research/tmp_btc_eth_2026_jan_apr_trend_pullback_continuation_BTCUSDT_loose_pullback_ledger.csv`
- `BTCUSDT tight_reclaim` ledger: `research/tmp_btc_eth_2026_jan_apr_trend_pullback_continuation_BTCUSDT_tight_reclaim_ledger.csv`
- `ETHUSDT base` ledger: `research/tmp_btc_eth_2026_jan_apr_trend_pullback_continuation_ETHUSDT_base_ledger.csv`
- `ETHUSDT loose_pullback` ledger: `research/tmp_btc_eth_2026_jan_apr_trend_pullback_continuation_ETHUSDT_loose_pullback_ledger.csv`
- `ETHUSDT tight_reclaim` ledger: `research/tmp_btc_eth_2026_jan_apr_trend_pullback_continuation_ETHUSDT_tight_reclaim_ledger.csv`
