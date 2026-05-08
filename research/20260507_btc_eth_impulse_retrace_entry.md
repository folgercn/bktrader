# Impulse Retrace Entry 1s Replay

Scope: research-only. Same impulse bar signals as impulse_bar_run but enters on first pullback (retrace) instead of at bar close. 2 bps/side slippage, maker entry 2 bps + market exit 4 bps.

| Symbol | Variant | Trades | Realistic | Raw | 2bps Slip | Win Rate | Max DD | Med Hold | Med MFE R | Exits | Cands | Entries | NoRetrace | RanAway | Busy |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|---:|
| `BTCUSDT` | `retrace25_vol12` | 26 | 0.6925% | 1.2178% | 1.0070% | 65.38% | -0.2986% | 1367s | 1.1424 | `{'BreakevenSL': 13, 'InitialSL': 9, 'TrailingSL': 4}` | 33 | 26 | 0 | 6 | 0 |
| `BTCUSDT` | `retrace15_vol12` | 27 | -0.2309% | 0.3095% | 0.0929% | 48.15% | -0.5437% | 1074s | 0.8779 | `{'InitialSL': 14, 'BreakevenSL': 11, 'TrailingSL': 2}` | 33 | 27 | 0 | 5 | 0 |
| `BTCUSDT` | `retrace25_novol` | 34 | 0.6408% | 1.3279% | 1.0522% | 58.82% | -0.4686% | 1646s | 1.1248 | `{'InitialSL': 14, 'BreakevenSL': 14, 'TrailingSL': 6}` | 51 | 34 | 0 | 15 | 0 |
| `ETHUSDT` | `retrace25_vol12` | 23 | -0.6762% | -0.2182% | -0.4016% | 60.87% | -0.6960% | 1362s | 1.1381 | `{'BreakevenSL': 13, 'InitialSL': 9, 'TrailingSL': 1}` | 27 | 23 | 0 | 3 | 0 |
| `ETHUSDT` | `retrace15_vol12` | 23 | -0.3778% | 0.0814% | -0.1024% | 56.52% | -0.6024% | 1537s | 1.0840 | `{'BreakevenSL': 12, 'InitialSL': 10, 'TrailingSL': 1}` | 27 | 23 | 0 | 2 | 1 |
| `ETHUSDT` | `retrace25_novol` | 31 | -0.6461% | -0.0283% | -0.2758% | 64.52% | -0.7048% | 1905s | 1.1567 | `{'BreakevenSL': 17, 'InitialSL': 11, 'TrailingSL': 3}` | 42 | 31 | 0 | 9 | 1 |

## Files

- Summary JSON: `research/btc_eth_impulse_retrace_entry_summary.json`
- `BTCUSDT retrace25_vol12` ledger: `research/tmp_impulse_retrace_entry_BTCUSDT_retrace25_vol12_ledger.csv`
- `BTCUSDT retrace15_vol12` ledger: `research/tmp_impulse_retrace_entry_BTCUSDT_retrace15_vol12_ledger.csv`
- `BTCUSDT retrace25_novol` ledger: `research/tmp_impulse_retrace_entry_BTCUSDT_retrace25_novol_ledger.csv`
- `ETHUSDT retrace25_vol12` ledger: `research/tmp_impulse_retrace_entry_ETHUSDT_retrace25_vol12_ledger.csv`
- `ETHUSDT retrace15_vol12` ledger: `research/tmp_impulse_retrace_entry_ETHUSDT_retrace15_vol12_ledger.csv`
- `ETHUSDT retrace25_novol` ledger: `research/tmp_impulse_retrace_entry_ETHUSDT_retrace25_novol_ledger.csv`
