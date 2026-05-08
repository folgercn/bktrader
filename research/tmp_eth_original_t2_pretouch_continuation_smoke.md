# ETH original_t2 pre-touch 延续概率表（2026-01-01T00:00:00+00:00 至 2026-01-03T23:59:59+00:00）

范围：仅限 `research`。本报告使用真正 `original_t2`：long level 为 `prev_high_2`，short level 为 `prev_low_2`；当前 1h signal bar 未闭合。候选点来自尚未 touch 的 1m close，后续标签使用连续 `1s high/low` 判定。

- 候选距离：`0.05` 到 `0.2` ATR
- pre-fail：`0.2` ATR
- 成本参考：滑点 `2bps/side` + 手续费 `6bps`，约 `10bps/notional`
- 候选样本：`31`
- 去重：`dedupe_one_per_bar_side_bucket=1`

## Overall

| Label | Samples | Continuation | Avg Return | Median Return | PreFail | PostFail | Timeout | Outcome Counts |
|---|---:|---:|---:|---:|---:|---:|---:|---|
| `c0p5_f0p2` | 31 | 16.13% | 0.0899bps | -5.9708bps | 41.94% | 29.03% | 12.90% | `{'pre_fail': 13, 'post_fail': 9, 'continuation': 5, 'post_timeout': 2, 'pre_timeout': 2}` |
| `c0p5_f0p3` | 31 | 16.13% | -0.4430bps | -6.2212bps | 41.94% | 22.58% | 19.35% | `{'pre_fail': 13, 'post_fail': 7, 'continuation': 5, 'post_timeout': 4, 'pre_timeout': 2}` |
| `c1p0_f0p2` | 31 | 12.90% | 3.3798bps | -5.9708bps | 41.94% | 29.03% | 16.13% | `{'pre_fail': 13, 'post_fail': 9, 'continuation': 4, 'post_timeout': 3, 'pre_timeout': 2}` |
| `c1p0_f0p3` | 31 | 12.90% | 2.8468bps | -6.2212bps | 41.94% | 22.58% | 22.58% | `{'pre_fail': 13, 'post_fail': 7, 'post_timeout': 5, 'continuation': 4, 'pre_timeout': 2}` |

## Top Compact States `c0p5_f0p2`

| State | Samples | Continuation | Avg Return | Median Return | PreFail | PostFail | Timeout |
|---|---:|---:|---:|---:|---:|---:|---:|
| `dist=0.10-0.15, speed300=>=0.20, pullback=0-0.02` | 2 | 50.00% | 13.9741bps | 13.9741bps | 0.00% | 50.00% | 0.00% |
| `dist=0.15-0.20, speed300=0.10-0.20, pullback=0.05-0.10` | 2 | 50.00% | 11.1324bps | 11.1324bps | 50.00% | 0.00% | 0.00% |
| `dist=0.05-0.10, speed300=0.10-0.20, pullback=0.02-0.05` | 2 | 50.00% | 10.7161bps | 10.7161bps | 0.00% | 50.00% | 0.00% |
| `dist=0.15-0.20, speed300=0.10-0.20, pullback=0.02-0.05` | 3 | 33.33% | 7.2450bps | -7.9766bps | 66.67% | 0.00% | 0.00% |
| `dist=0.15-0.20, speed300=>=0.20, pullback=0-0.02` | 4 | 25.00% | 2.2176bps | -1.0884bps | 25.00% | 50.00% | 0.00% |
| `dist=0.10-0.15, speed300=0.10-0.20, pullback=0.02-0.05` | 2 | 0.00% | -13.4104bps | -13.4104bps | 50.00% | 50.00% | 0.00% |
| `dist=0.05-0.10, speed300=>=0.20, pullback=0-0.02` | 3 | 0.00% | -13.4533bps | -11.3613bps | 66.67% | 33.33% | 0.00% |

## Side x Distance `c0p5_f0p2`

| Side | Distance | Samples | Continuation | Avg Return | Median Return |
|---|---|---:|---:|---:|---:|
| `long` | `0.05-0.10` | 5 | 20.00% | 2.5205bps | -0.7088bps |
| `long` | `0.10-0.15` | 1 | 100.00% | 30.3357bps | 30.3357bps |
| `long` | `0.15-0.20` | 6 | 33.33% | 7.8578bps | 7.3629bps |
| `short` | `0.05-0.10` | 4 | 0.00% | -8.7545bps | -8.6122bps |
| `short` | `0.10-0.15` | 7 | 0.00% | -6.7615bps | -5.6383bps |
| `short` | `0.15-0.20` | 8 | 12.50% | -0.6187bps | -6.9737bps |

## Top Compact States `c0p5_f0p3`

| State | Samples | Continuation | Avg Return | Median Return | PreFail | PostFail | Timeout |
|---|---:|---:|---:|---:|---:|---:|---:|
| `dist=0.10-0.15, speed300=>=0.20, pullback=0-0.02` | 2 | 50.00% | 11.8214bps | 11.8214bps | 0.00% | 50.00% | 0.00% |
| `dist=0.15-0.20, speed300=0.10-0.20, pullback=0.05-0.10` | 2 | 50.00% | 11.1324bps | 11.1324bps | 50.00% | 0.00% | 0.00% |
| `dist=0.05-0.10, speed300=0.10-0.20, pullback=0.02-0.05` | 2 | 50.00% | 8.5436bps | 8.5436bps | 0.00% | 50.00% | 0.00% |
| `dist=0.15-0.20, speed300=0.10-0.20, pullback=0.02-0.05` | 3 | 33.33% | 7.2450bps | -7.9766bps | 66.67% | 0.00% | 0.00% |
| `dist=0.15-0.20, speed300=>=0.20, pullback=0-0.02` | 4 | 25.00% | 0.4136bps | -4.6963bps | 25.00% | 50.00% | 0.00% |
| `dist=0.10-0.15, speed300=0.10-0.20, pullback=0.02-0.05` | 2 | 0.00% | -10.2604bps | -10.2604bps | 50.00% | 0.00% | 50.00% |
| `dist=0.05-0.10, speed300=>=0.20, pullback=0-0.02` | 3 | 0.00% | -11.3527bps | -9.3464bps | 66.67% | 0.00% | 33.33% |

## Side x Distance `c0p5_f0p3`

| Side | Distance | Samples | Continuation | Avg Return | Median Return |
|---|---|---:|---:|---:|---:|
| `long` | `0.05-0.10` | 5 | 20.00% | 2.5205bps | -0.7088bps |
| `long` | `0.10-0.15` | 1 | 100.00% | 30.3357bps | 30.3357bps |
| `long` | `0.15-0.20` | 6 | 33.33% | 7.3063bps | 5.7084bps |
| `short` | `0.05-0.10` | 4 | 0.00% | -8.2653bps | -8.6122bps |
| `short` | `0.10-0.15` | 7 | 0.00% | -7.7031bps | -6.6929bps |
| `short` | `0.15-0.20` | 8 | 12.50% | -1.6910bps | -7.0989bps |

## Top Compact States `c1p0_f0p2`

| State | Samples | Continuation | Avg Return | Median Return | PreFail | PostFail | Timeout |
|---|---:|---:|---:|---:|---:|---:|---:|
| `dist=0.10-0.15, speed300=>=0.20, pullback=0-0.02` | 2 | 50.00% | 26.1800bps | 26.1800bps | 0.00% | 50.00% | 0.00% |
| `dist=0.15-0.20, speed300=0.10-0.20, pullback=0.05-0.10` | 2 | 50.00% | 23.3804bps | 23.3804bps | 50.00% | 0.00% | 0.00% |
| `dist=0.05-0.10, speed300=0.10-0.20, pullback=0.02-0.05` | 2 | 50.00% | 22.9939bps | 22.9939bps | 0.00% | 50.00% | 0.00% |
| `dist=0.15-0.20, speed300=0.10-0.20, pullback=0.02-0.05` | 3 | 33.33% | 19.6801bps | -7.9766bps | 66.67% | 0.00% | 0.00% |
| `dist=0.15-0.20, speed300=>=0.20, pullback=0-0.02` | 4 | 0.00% | 0.0219bps | -1.0884bps | 25.00% | 50.00% | 25.00% |
| `dist=0.10-0.15, speed300=0.10-0.20, pullback=0.02-0.05` | 2 | 0.00% | -13.4104bps | -13.4104bps | 50.00% | 50.00% | 0.00% |
| `dist=0.05-0.10, speed300=>=0.20, pullback=0-0.02` | 3 | 0.00% | -13.4533bps | -11.3613bps | 66.67% | 33.33% | 0.00% |

## Side x Distance `c1p0_f0p2`

| Side | Distance | Samples | Continuation | Avg Return | Median Return |
|---|---|---:|---:|---:|---:|
| `long` | `0.05-0.10` | 5 | 20.00% | 7.4316bps | -0.7088bps |
| `long` | `0.10-0.15` | 1 | 100.00% | 54.7476bps | 54.7476bps |
| `long` | `0.15-0.20` | 6 | 16.67% | 10.4767bps | 7.3629bps |
| `short` | `0.05-0.10` | 4 | 0.00% | -8.7545bps | -8.6122bps |
| `short` | `0.10-0.15` | 7 | 0.00% | -6.7615bps | -5.6383bps |
| `short` | `0.15-0.20` | 8 | 12.50% | 4.0444bps | -6.9737bps |

## Top Compact States `c1p0_f0p3`

| State | Samples | Continuation | Avg Return | Median Return | PreFail | PostFail | Timeout |
|---|---:|---:|---:|---:|---:|---:|---:|
| `dist=0.10-0.15, speed300=>=0.20, pullback=0-0.02` | 2 | 50.00% | 24.0273bps | 24.0273bps | 0.00% | 50.00% | 0.00% |
| `dist=0.15-0.20, speed300=0.10-0.20, pullback=0.05-0.10` | 2 | 50.00% | 23.3804bps | 23.3804bps | 50.00% | 0.00% | 0.00% |
| `dist=0.05-0.10, speed300=0.10-0.20, pullback=0.02-0.05` | 2 | 50.00% | 20.8214bps | 20.8214bps | 0.00% | 50.00% | 0.00% |
| `dist=0.15-0.20, speed300=0.10-0.20, pullback=0.02-0.05` | 3 | 33.33% | 19.6801bps | -7.9766bps | 66.67% | 0.00% | 0.00% |
| `dist=0.15-0.20, speed300=>=0.20, pullback=0-0.02` | 4 | 0.00% | -1.7821bps | -4.6963bps | 25.00% | 50.00% | 25.00% |
| `dist=0.10-0.15, speed300=0.10-0.20, pullback=0.02-0.05` | 2 | 0.00% | -10.2604bps | -10.2604bps | 50.00% | 0.00% | 50.00% |
| `dist=0.05-0.10, speed300=>=0.20, pullback=0-0.02` | 3 | 0.00% | -11.3527bps | -9.3464bps | 66.67% | 0.00% | 33.33% |

## Side x Distance `c1p0_f0p3`

| Side | Distance | Samples | Continuation | Avg Return | Median Return |
|---|---|---:|---:|---:|---:|
| `long` | `0.05-0.10` | 5 | 20.00% | 7.4316bps | -0.7088bps |
| `long` | `0.10-0.15` | 1 | 100.00% | 54.7476bps | 54.7476bps |
| `long` | `0.15-0.20` | 6 | 16.67% | 9.9251bps | 5.7084bps |
| `short` | `0.05-0.10` | 4 | 0.00% | -8.2653bps | -8.6122bps |
| `short` | `0.10-0.15` | 7 | 0.00% | -7.7031bps | -6.6929bps |
| `short` | `0.15-0.20` | 8 | 12.50% | 2.9722bps | -7.0989bps |

## 文件

- Summary JSON：`research/tmp_eth_original_t2_pretouch_continuation_smoke_summary.json`
- Candidate CSV：`research/tmp_eth_original_t2_pretouch_continuation_smoke_candidates.csv`
