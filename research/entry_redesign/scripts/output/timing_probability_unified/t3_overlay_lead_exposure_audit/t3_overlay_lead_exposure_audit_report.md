# T3 Overlay Lead Exposure Audit

Research-only risk audit for adding the ETH T3 direct-entry overlay back to the pretouch research lead.

- T3 size scale: `2.0`
- Lead exposure windows are approximate because compact lead ledgers do not store exact exit time.
- T3 overlay exposure uses actual lifecycle entry/exit rows.

## Additive Bridge

| Variant | Calendar Sum | Worst Month | Neg Months | Trades |
|---|---:|---:|---:|---:|
| `lead_same_close` | 30.217222% | 0.000000% | 0 | 62 |
| `lead_adverse10` | 22.971648% | 0.000000% | 0 | 62 |
| `t3_overlay_eth_adverse_size2` | 11.410000% | -0.830000% | 3 | 163 |
| `lead_same_close_plus_t3_overlay` | 41.627222% | -0.470000% | 1 | 225 |
| `lead_adverse10_plus_t3_overlay` | 34.381648% | -0.470000% | 1 | 225 |

## T3 Overlay Exposure

| Calendar Sum | Worst Silo | Neg Silos | T3 Trades | Fee-Net T3 PnL | Gross PnL | Fees | Ex Final Mark | Final Mark | Win Rate | T3 DD | Loss Streak | Avg Hold | P90 Hold | Max Hold | Worst MAE | Worst Net PnL |
|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 11.410000% | -0.830000% | 3 | 163 | 11.402632% | 24.503851% | 13.101219% | 11.402632% | 0.000000%/0 | 40.49% | -1.286147% | 8 | 3833.21s | 4359.80s | 8481.00s | -276.2243bp | -40.0058bp |

## Approximate Capital Overlap

| Lead windows | Overlay windows | Pairs | Lead overlapped | Overlay overlapped | Max combined notional | P95 combined notional | Overlap notional hours | Max overlap | Overlap overlay PnL | Overlap lead PnL |
|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 62 | 163 | 0 | 0 | 0 | 0.000000 | 0.000000 | 0.000000 | 0.00s | 0.000000% | 0.000000% |

## Combined Equity Approximation

- Combined realized PnL: `34.374280%`
- Combined sequential max drawdown: `-1.210504%`

## Read

- T3 overlay PnL/DD uses fee-net paired lifecycle trades to match the calendar-return accounting.
- This run found no timestamp overlap between approximate lead windows and actual overlay windows, but the lead window model is still approximate.
- T3 final-mark contribution and drawdown are reported separately to avoid treating month-end marks as normal exits.
