# 2026-04-16 Breakout / Reentry Research Notes

Scope: research-only backtests. No live execution or `internal/service` trading path was changed.

## Shared Setup

- Data: `BTC_1min_Clean.csv`
- Window: `2020-01-01` to `2026-02-28`
- Signal timeframe: `1D`
- ATR period: `14`
- Initial balance: `100000.0`
- Slippage: `0.0005`
- Stop: `atr`, `stop_loss_atr=0.05`
- Max trades per bar: `4`
- Reentry size schedule: `[0.10, 0.05, 0.025]`
- Trailing stop: `trailing_stop_atr=0.3`
- Delayed trailing activation: `0.5`

## Experiment 1: Breakout Levels

Question: compare original wick breakout levels with candle-body breakout levels.

- Baseline `wick`: `prev_high_1/2 = high.shift(1/2)`, `prev_low_1/2 = low.shift(1/2)`.
- Variant `body`: `prev_high_1/2 = max(open, close).shift(1/2)`, `prev_low_1/2 = min(open, close).shift(1/2)`.

| Scenario | Final Balance | Return | Max DD | Trades | Win Rate | Sharpe | Buy Entries | Short Entries | Exit Reasons |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| `wick` baseline | 11,137,479.39 | 11037.48% | -0.17% | 1780 | 90.6% | 19.25 | 1319 | 1310 | `SL:1773, PT:7` |
| `body` breakout | 1,637,453.76 | 1537.45% | -0.32% | 1871 | 67.3% | 12.72 | 1408 | 1332 | `SL:1853, PT:18` |

Delta, `body - wick`:

- Final balance: `-9,500,025.63`
- Return: `-9500.03 pp`
- Max DD: `-0.14 pp`
- Trades: `+91`
- Buy entries: `+89`
- Short entries: `+22`

Takeaway: body breakout increased triggering frequency but materially reduced trade quality. Keep baseline `wick` breakout as the default.

## Experiment 2: Reentry Anchor Levels

Question: keep baseline wick breakout, then change reentry anchor from wick low/high to candle-body low/high.

- Baseline: long reentry at `prev_low_1 + 0.1ATR`; short reentry at `prev_high_1`.
- Variant A: long reentry at `prev_body_low_1`; short reentry at `prev_body_high_1`.
- Variant B: long reentry at `prev_body_low_1 + 0.1ATR`; short reentry at `prev_body_high_1 - 0.1ATR`.

| Scenario | Final Balance | Return | Max DD | Trades | Win Rate | Sharpe | Buy Entries | Short Entries | Exit Reasons |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| Baseline wick reentry | 11,137,479.39 | 11037.48% | -0.17% | 1780 | 90.6% | 19.25 | 1319 | 1310 | `SL:1773, PT:7` |
| Body reentry, no ATR offset | 3,738,242.29 | 3638.24% | -0.27% | 1779 | 81.6% | 16.30 | 1335 | 1289 | `SL:1778, PT:1` |
| Body reentry, +/- 0.1ATR | 2,172,836.90 | 2072.84% | -0.28% | 1744 | 76.5% | 14.69 | 1308 | 1263 | `SL:1743, PT:1` |

Delta vs baseline:

| Scenario | Final Balance Delta | Return Delta | Max DD Delta | Trades Delta | Buy Entries Delta | Short Entries Delta |
|---|---:|---:|---:|---:|---:|---:|
| Body reentry, no ATR offset | -7,399,237.10 | -7399.24 pp | -0.10 pp | -1 | +16 | -21 |
| Body reentry, +/- 0.1ATR | -8,964,642.49 | -8964.64 pp | -0.11 pp | -36 | -11 | -47 |

Takeaway: both body reentry variants underperform the wick reentry baseline. The no-offset body reentry is less damaging than the +/- `0.1ATR` body variant, but neither improves the baseline.

## Experiment 3: 2026 Q1 Tick-Derived 1min Reentry Check

The previous `BTC_1min_Clean.csv` only covered through `2026-02-28`, so it is not used for this Q1 check. A new file was generated directly from raw tick data:

- Output: `BTC_1min_Q1.csv`
- Inputs:
  - `dataset/archive/BTCUSDT-trades-2026-01/BTCUSDT-trades-2026-01.csv`
  - `dataset/archive/BTCUSDT-trades-2026-02/BTCUSDT-trades-2026-02.csv`
  - `dataset/archive/BTCUSDT-trades-2026-03/BTCUSDT-trades-2026-03.csv`
- Range: `2026-01-01 00:00:00+00:00` to `2026-03-31 23:59:00+00:00`
- Rows: `129600`, exactly `90 * 24 * 60`

No `2025-12` BTC tick file was available locally, so this run generated 1D signals from Q1 data only. This means the first part of January is used for MA/ATR warmup; first trade appears on `2026-01-18`.

Breakout remains baseline `wick`; only reentry anchor changes.

| Scenario | Final Balance | Return | Max DD | Trades | Win Rate | Sharpe | Buy Entries | Short Entries | First Trade | Last Trade | Exit Reasons |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|---|
| Baseline wick reentry | 117,968.59 | 17.97% | -0.17% | 75 | 86.7% | 22.17 | 53 | 54 | `2026-01-18 23:46:00+00:00` | `2026-03-30 04:02:00+00:00` | `SL:75` |
| Body reentry, no ATR offset | 113,693.77 | 13.69% | -0.19% | 73 | 82.2% | 20.95 | 51 | 54 | `2026-01-18 23:46:00+00:00` | `2026-03-30 19:50:00+00:00` | `SL:73` |
| Body reentry, +/- 0.1ATR | 112,136.78 | 12.14% | -0.17% | 71 | 83.1% | 21.07 | 51 | 52 | `2026-01-18 23:46:00+00:00` | `2026-03-29 23:21:00+00:00` | `SL:71` |

Delta vs baseline:

| Scenario | Final Balance Delta | Return Delta | Max DD Delta | Trades Delta | Buy Entries Delta | Short Entries Delta |
|---|---:|---:|---:|---:|---:|---:|
| Body reentry, no ATR offset | -4,274.83 | -4.27 pp | -0.01 pp | -2 | -2 | 0 |
| Body reentry, +/- 0.1ATR | -5,831.81 | -5.83 pp | 0.00 pp | -4 | -2 | -2 |

Takeaway: on tick-derived 2026 Q1 1min data, body reentry still does not beat the baseline. The no-offset body reentry remains the better of the two body variants, but baseline wick reentry is still ahead.

## Experiment 4: 2026 Q1 Tick-Derived 4h Reentry Check

Data source remains `BTC_1min_Q1.csv`, generated from raw BTCUSDT 2026 Q1 tick files. For this run, 4h signal bars were aggregated from that 1min file:

- Signal rows: `540`
- Signal range: `2026-01-01 00:00:00+00:00` to `2026-03-31 20:00:00+00:00`
- Valid ATR rows: `527`
- Valid MA20 rows: `521`

Breakout remains baseline `wick`; only reentry anchor changes.

| Scenario | Final Balance | Return | Max DD | Trades | Win Rate | Sharpe | Buy Entries | Short Entries | First Trade | Last Trade | Exit Reasons |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|---|
| Baseline wick reentry | 119,672.92 | 19.67% | -0.10% | 272 | 82.0% | 16.75 | 169 | 222 | `2026-01-06 14:07:00+00:00` | `2026-03-31 11:18:00+00:00` | `SL:272` |
| Body reentry, no ATR offset | 114,627.98 | 14.63% | -0.17% | 273 | 72.9% | 14.26 | 165 | 229 | `2026-01-06 14:07:00+00:00` | `2026-03-31 11:18:00+00:00` | `SL:273` |
| Body reentry, +/- 0.1ATR | 112,325.22 | 12.33% | -0.17% | 256 | 67.6% | 13.56 | 160 | 210 | `2026-01-06 14:07:00+00:00` | `2026-03-31 11:18:00+00:00` | `SL:256` |

Delta vs baseline:

| Scenario | Final Balance Delta | Return Delta | Max DD Delta | Trades Delta | Buy Entries Delta | Short Entries Delta |
|---|---:|---:|---:|---:|---:|---:|
| Body reentry, no ATR offset | -5,044.94 | -5.04 pp | -0.07 pp | +1 | -4 | +7 |
| Body reentry, +/- 0.1ATR | -7,347.70 | -7.35 pp | -0.07 pp | -16 | -9 | -12 |

Takeaway: on 4h signals, baseline wick reentry remains ahead. The 4h baseline outperformed the 1D Q1 baseline (`19.67%` vs `17.97%`), but body reentry still did not show an edge.

## Experiment 5: ETH 2026 Q1 Tick-Derived Reentry Check

To mirror the BTC Q1 check, a new ETH 1min file was generated directly from raw tick data:

- Output: `ETH_1min_Q1.csv`
- Inputs:
  - `dataset/archive/ETHUSDT-trades-2026-01/ETHUSDT-trades-2026-01.csv`
  - `dataset/archive/ETHUSDT-trades-2026-02/ETHUSDT-trades-2026-02.zip`
  - `dataset/archive/ETHUSDT-trades-2026-03/ETHUSDT-trades-2026-03.zip`
- Range: `2026-01-01 00:00:00+00:00` to `2026-03-31 23:59:00+00:00`
- Rows: `129600`, exactly `90 * 24 * 60`

Breakout remains baseline `wick`; only reentry anchor changes.

### ETH 1D Signals

- Signal rows: `90`
- Signal range: `2026-01-01 00:00:00+00:00` to `2026-03-31 00:00:00+00:00`
- Valid ATR rows: `77`
- Valid MA20 rows: `71`

| Scenario | Final Balance | Return | Max DD | Trades | Win Rate | Sharpe | Buy Entries | Short Entries | First Trade | Last Trade | Exit Reasons |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|---|
| Baseline wick reentry | 122,151.87 | 22.15% | -0.04% | 55 | 98.2% | 26.32 | 45 | 38 | `2026-01-25 16:16:00+00:00` | `2026-03-30 19:11:00+00:00` | `SL:55` |
| Body reentry, no ATR offset | 117,310.58 | 17.31% | -0.15% | 54 | 88.9% | 22.74 | 44 | 38 | `2026-01-25 16:16:00+00:00` | `2026-03-30 19:11:00+00:00` | `SL:54` |
| Body reentry, +/- 0.1ATR | 115,563.18 | 15.56% | -0.10% | 51 | 92.2% | 23.09 | 44 | 34 | `2026-01-25 16:16:00+00:00` | `2026-03-30 19:11:00+00:00` | `SL:51` |

Delta vs baseline:

| Scenario | Final Balance Delta | Return Delta | Max DD Delta | Trades Delta | Buy Entries Delta | Short Entries Delta |
|---|---:|---:|---:|---:|---:|---:|
| Body reentry, no ATR offset | -4,841.30 | -4.84 pp | -0.12 pp | -1 | -1 | 0 |
| Body reentry, +/- 0.1ATR | -6,588.70 | -6.59 pp | -0.06 pp | -4 | -1 | -4 |

### ETH 4h Signals

- Signal rows: `540`
- Signal range: `2026-01-01 00:00:00+00:00` to `2026-03-31 20:00:00+00:00`
- Valid ATR rows: `527`
- Valid MA20 rows: `521`

| Scenario | Final Balance | Return | Max DD | Trades | Win Rate | Sharpe | Buy Entries | Short Entries | First Trade | Last Trade | Exit Reasons |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|---|
| Baseline wick reentry | 134,354.23 | 34.35% | -0.08% | 305 | 86.2% | 17.66 | 175 | 263 | `2026-01-04 21:11:00+00:00` | `2026-03-31 14:43:00+00:00` | `SL:305` |
| Body reentry, no ATR offset | 123,802.54 | 23.80% | -0.18% | 295 | 74.9% | 13.88 | 168 | 262 | `2026-01-04 21:11:00+00:00` | `2026-03-31 14:43:00+00:00` | `SL:295` |
| Body reentry, +/- 0.1ATR | 119,278.25 | 19.28% | -0.23% | 292 | 67.5% | 12.43 | 175 | 249 | `2026-01-04 21:11:00+00:00` | `2026-03-31 14:43:00+00:00` | `SL:292` |

Delta vs baseline:

| Scenario | Final Balance Delta | Return Delta | Max DD Delta | Trades Delta | Buy Entries Delta | Short Entries Delta |
|---|---:|---:|---:|---:|---:|---:|
| Body reentry, no ATR offset | -10,551.69 | -10.55 pp | -0.10 pp | -10 | -7 | -1 |
| Body reentry, +/- 0.1ATR | -15,075.98 | -15.08 pp | -0.15 pp | -13 | 0 | -14 |

Takeaway: ETH also favors baseline wick reentry. The 4h ETH baseline is the strongest Q1 result in this batch (`34.35%`), while both body reentry variants underperform on 1D and 4h.

## Experiment 6: True Pullback Reentry Trigger

Previous reentry tests changed the anchor but kept the existing reclaim trigger:

- Long reclaim: `bar.high >= re_p`
- Short reclaim: `bar.low <= re_p`

This experiment adds a true pullback trigger while leaving the default behavior unchanged:

- Long pullback: `bar.low <= re_p`
- Short pullback: `bar.high >= re_p`

Baseline remains the existing reclaim mode with wick anchors. Variants use body anchors with pullback trigger.

### BTC Pullback Reentry

| Timeframe | Scenario | Final Balance | Return | Max DD | Trades | Win Rate | Sharpe | Buy Entries | Short Entries | Exit Reasons |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| 1D | Baseline reclaim wick | 117,968.59 | 17.97% | -0.17% | 75 | 86.7% | 22.17 | 53 | 54 | `SL:75` |
| 1D | Body pullback, no ATR offset | 99,791.60 | -0.21% | -0.25% | 11 | 9.1% | -1.19 | 16 | 15 | `SL:11` |
| 1D | Body pullback, +/- 0.1ATR | 99,566.34 | -0.43% | -0.43% | 15 | 0.0% | -61.60 | 20 | 14 | `SL:15` |
| 4h | Baseline reclaim wick | 119,672.92 | 19.67% | -0.10% | 272 | 82.0% | 16.75 | 169 | 222 | `SL:272` |
| 4h | Body pullback, no ATR offset | 98,745.16 | -1.25% | -1.25% | 77 | 7.8% | -3.17 | 73 | 89 | `SL:77` |
| 4h | Body pullback, +/- 0.1ATR | 98,252.06 | -1.75% | -1.75% | 103 | 3.9% | -6.67 | 89 | 105 | `SL:103` |

BTC delta vs baseline:

| Timeframe | Scenario | Return Delta | Max DD Delta | Trades Delta |
|---|---|---:|---:|---:|
| 1D | Body pullback, no ATR offset | -18.18 pp | -0.07 pp | -64 |
| 1D | Body pullback, +/- 0.1ATR | -18.40 pp | -0.26 pp | -60 |
| 4h | Body pullback, no ATR offset | -20.93 pp | -1.16 pp | -195 |
| 4h | Body pullback, +/- 0.1ATR | -21.42 pp | -1.65 pp | -169 |

### ETH Pullback Reentry

| Timeframe | Scenario | Final Balance | Return | Max DD | Trades | Win Rate | Sharpe | Buy Entries | Short Entries | Exit Reasons |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| 1D | Baseline reclaim wick | 122,151.87 | 22.15% | -0.04% | 55 | 98.2% | 26.32 | 45 | 38 | `SL:55` |
| 1D | Body pullback, no ATR offset | 99,752.73 | -0.25% | -0.25% | 6 | 0.0% | -79.73 | 12 | 13 | `SL:6` |
| 1D | Body pullback, +/- 0.1ATR | 99,499.79 | -0.50% | -0.50% | 15 | 0.0% | -107.78 | 16 | 19 | `SL:15` |
| 4h | Baseline reclaim wick | 134,354.23 | 34.35% | -0.08% | 305 | 86.2% | 17.66 | 175 | 263 | `SL:305` |
| 4h | Body pullback, no ATR offset | 98,466.71 | -1.53% | -1.53% | 81 | 4.9% | -10.60 | 65 | 112 | `SL:81` |
| 4h | Body pullback, +/- 0.1ATR | 98,698.80 | -1.30% | -1.41% | 115 | 6.1% | -0.92 | 79 | 140 | `SL:115` |

ETH delta vs baseline:

| Timeframe | Scenario | Return Delta | Max DD Delta | Trades Delta |
|---|---|---:|---:|---:|
| 1D | Body pullback, no ATR offset | -22.40 pp | -0.21 pp | -49 |
| 1D | Body pullback, +/- 0.1ATR | -22.65 pp | -0.46 pp | -40 |
| 4h | Body pullback, no ATR offset | -35.89 pp | -1.45 pp | -224 |
| 4h | Body pullback, +/- 0.1ATR | -35.66 pp | -1.33 pp | -190 |

Takeaway: true pullback reentry performs substantially worse in this Q1 batch. It creates far fewer completed trades and much lower win rates under the current stop/trailing/reentry sizing setup. The existing reclaim-style reentry remains the better behavior for BTC and ETH on both 1D and 4h.
