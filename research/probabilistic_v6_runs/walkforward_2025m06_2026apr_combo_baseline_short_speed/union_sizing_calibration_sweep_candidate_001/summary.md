# Probabilistic V6 Union Sizing Sweep

范围：仅限 `research`。固定 selection，只改变 per-event `model_notional_share` transform。

## Results

| Rank | Config | Return | Trades | Active Months | Worst Silo | Negative Silos | Args |
|---:|---|---:|---:|---:|---:|---:|---|
| 1 | `power0p50_mult_1p40_cap_1p80` | 14.7215% | 115 | 9 | -1.8247% | 2 | `--share-power 0.50 --share-multiplier 1.40 --share-cap 1.80` |
| 2 | `power0p25_mult_1p35_cap_1p80` | 14.0382% | 115 | 9 | -1.7145% | 2 | `--share-power 0.25 --share-multiplier 1.35 --share-cap 1.80` |
| 3 | `mult_1p30_cap_1p80` | 13.8919% | 115 | 9 | -1.7848% | 2 | `--share-multiplier 1.30 --share-cap 1.80` |
| 4 | `power0p50_mult_1p30_cap_1p80` | 13.6649% | 115 | 9 | -1.6950% | 2 | `--share-power 0.50 --share-multiplier 1.30 --share-cap 1.80` |
| 5 | `power0_fixed_1p30` | 13.4179% | 115 | 9 | -1.6084% | 2 | `--share-power 0.00 --share-multiplier 1.30` |
| 6 | `mult_1p20` | 13.0282% | 115 | 9 | -1.6481% | 2 | `--share-multiplier 1.20` |
| 7 | `quality_edge_return_mult_1p20_cap_1p80` | 12.8486% | 115 | 9 | -1.5747% | 2 | `--sizing-profile source_quality --sizing-min-scale 0.60 --sizing-max-scale 1.35 --sizing-edge-weight 0.35 --sizing-return-weight 0.35 --sizing-return-dd-weight 0.15 --sizing-markov-weight 0.15 --share-multiplier 1.20 --share-cap 1.80` |
| 8 | `power0_fixed_1p20` | 12.3823% | 115 | 9 | -1.4852% | 2 | `--share-power 0.00 --share-multiplier 1.20` |
| 9 | `quality_return_heavy_mult_1p15_cap_1p80` | 12.1964% | 115 | 9 | -1.6197% | 2 | `--sizing-profile source_quality --sizing-min-scale 0.60 --sizing-max-scale 1.35 --sizing-edge-weight 0.10 --sizing-return-weight 0.35 --sizing-return-dd-weight 0.35 --sizing-markov-weight 0.20 --share-multiplier 1.15 --share-cap 1.80` |
| 10 | `mult_1p10` | 11.9370% | 115 | 9 | -1.5113% | 2 | `--share-multiplier 1.10` |
| 11 | `quality_edge_return_0p6_1p35` | 10.9535% | 115 | 9 | -1.3131% | 2 | `--sizing-profile source_quality --sizing-min-scale 0.60 --sizing-max-scale 1.35 --sizing-edge-weight 0.35 --sizing-return-weight 0.35 --sizing-return-dd-weight 0.15 --sizing-markov-weight 0.15` |
| 12 | `baseline` | 10.8467% | 115 | 9 | -1.3744% | 2 | `baseline` |
| 13 | `quality_return_heavy_0p6_1p35` | 10.7111% | 115 | 9 | -1.4092% | 2 | `--sizing-profile source_quality --sizing-min-scale 0.60 --sizing-max-scale 1.35 --sizing-edge-weight 0.10 --sizing-return-weight 0.35 --sizing-return-dd-weight 0.35 --sizing-markov-weight 0.20` |
| 14 | `quality_balanced_0p5_1p40` | 10.4168% | 115 | 9 | -1.3095% | 2 | `--sizing-profile source_quality --sizing-min-scale 0.50 --sizing-max-scale 1.40` |
| 15 | `power0_fixed_1p00` | 10.3132% | 115 | 9 | -1.2385% | 2 | `--share-power 0.00` |
| 16 | `cap_1p20` | 10.3021% | 115 | 9 | -1.3920% | 2 | `--share-cap 1.20` |
| 17 | `quality_balanced_0p6_1p25` | 10.0844% | 115 | 9 | -1.2743% | 2 | `--sizing-profile source_quality --sizing-min-scale 0.60 --sizing-max-scale 1.25` |
| 18 | `cap_1p00` | 9.4533% | 115 | 9 | -1.2385% | 2 | `--share-cap 1.00` |
| 19 | `cap_0p80` | 8.1421% | 115 | 9 | -0.9915% | 2 | `--share-cap 0.80` |

## Best Groups

| Month | Symbol | Return | Trades | Mean Share | Mean Scale |
|---|---|---:|---:|---:|---:|
| `2025-06` | `ETHUSDT` | 0.1508% | 3 | 1.3095 | 1.0000 |
| `2025-07` | `BTCUSDT` | -0.5742% | 18 | 1.4162 | 1.0000 |
| `2025-08` | `BTCUSDT` | -1.8247% | 5 | 1.4236 | 1.0000 |
| `2025-09` | `ETHUSDT` | 0.4031% | 5 | 1.5060 | 1.0000 |
| `2025-11` | `BTCUSDT` | 0.5169% | 6 | 1.3915 | 1.0000 |
| `2025-12` | `ETHUSDT` | 0.8909% | 24 | 1.4127 | 1.0000 |
| `2026-01` | `BTCUSDT` | 0.6733% | 16 | 1.3503 | 1.0000 |
| `2026-02` | `BTCUSDT` | 5.6221% | 15 | 1.3680 | 1.0000 |
| `2026-02` | `ETHUSDT` | 2.5257% | 12 | 1.3762 | 1.0000 |
| `2026-03` | `ETHUSDT` | 6.3376% | 11 | 1.3845 | 1.0000 |
