# T3 Probability Overlay Audit

Research-only audit for using the frozen probability model as a T3 quality layer.

- Model: `/Users/wuyaocheng/Downloads/bkTrader/data/pretouch_model.json`
- Months: 2025-06, 2025-07, 2025-08, 2025-09, 2025-10, 2025-11, 2025-12, 2026-01, 2026-02, 2026-03, 2026-04
- Symbols: ETHUSDT, BTCUSDT
- Trades: 104
- Scored events: 417
- Runtime: 1414.63s

## Gate Summary

| gate                      | trades | net_after_fee_pct | avg_trade_net_pct | win_rate_pct | worst_symbol_month_pct | negative_symbol_months |
| ------------------------- | ------ | ----------------- | ----------------- | ------------ | ---------------------- | ---------------------- |
| side_short                | 33     | 0.450954          | 0.013665          | 48.480000    | -0.090031              | 9                      |
| model_live_quality        | 69     | -0.145670         | -0.002111         | 42.030000    | -0.332522              | 13                     |
| all_t3                    | 104    | -0.171863         | -0.001653         | 41.350000    | -0.460332              | 12                     |
| matched_model_events      | 104    | -0.171863         | -0.001653         | 41.350000    | -0.460332              | 12                     |
| rf_ge_0p50                | 86     | -0.199781         | -0.002323         | 40.700000    | -0.386231              | 13                     |
| rf_ge_0p60                | 49     | -0.205327         | -0.004190         | 42.860000    | -0.229907              | 12                     |
| rf_ge_0p70                | 31     | -0.268203         | -0.008652         | 38.710000    | -0.229907              | 11                     |
| rf_ge_0p65                | 34     | -0.281862         | -0.008290         | 41.180000    | -0.229907              | 11                     |
| model_live_quality_timing | 62     | -0.338074         | -0.005453         | 41.940000    | -0.261877              | 14                     |
| rf_ge_0p45                | 99     | -0.351469         | -0.003550         | 41.410000    | -0.389687              | 13                     |
| ext_abs_le_0p05           | 91     | -0.360086         | -0.003957         | 41.760000    | -0.285718              | 13                     |
| timing_fast_or_slow       | 97     | -0.364267         | -0.003755         | 41.240000    | -0.389687              | 13                     |
| timing_fast               | 97     | -0.364267         | -0.003755         | 41.240000    | -0.389687              | 13                     |
| ext_abs_le_0p10           | 95     | -0.465949         | -0.004905         | 41.050000    | -0.359055              | 13                     |
| rf_ge_0p55                | 69     | -0.563144         | -0.008162         | 42.030000    | -0.285718              | 12                     |
| side_long                 | 71     | -0.622817         | -0.008772         | 38.030000    | -0.547555              | 13                     |

## Probability Buckets

| family            | bucket    | trades | net_after_fee_pct | avg_trade_net_pct | win_rate_pct |
| ----------------- | --------- | ------ | ----------------- | ----------------- | ------------ |
| rf_probability    | 0.50-0.55 | 17     | 0.363363          | 0.021374          | 35.290000    |
| rf_probability    | <0.35     | 5      | 0.179606          | 0.035921          | 40.000000    |
| rf_probability    | 0.60-0.70 | 18     | 0.062876          | 0.003493          | 50.000000    |
| rf_probability    | 0.45-0.50 | 13     | -0.151688         | -0.011668         | 46.150000    |
| rf_probability    | >=0.70    | 31     | -0.268203         | -0.008652         | 38.710000    |
| rf_probability    | 0.55-0.60 | 20     | -0.357816         | -0.017891         | 40.000000    |
| side              | short     | 33     | 0.450954          | 0.013665          | 48.480000    |
| side              | long      | 71     | -0.622817         | -0.008772         | 38.030000    |
| timing_prediction | skip      | 7      | 0.192404          | 0.027486          | 42.860000    |
| timing_prediction | fast      | 97     | -0.364267         | -0.003755         | 41.240000    |

## Read

- This is not a promoted gate yet; it is the evidence layer before rerunning lifecycle with skips/sizing.
- A useful probability layer should improve T3 net contribution without relying on tiny trade counts.
- If the best buckets lose against `all_t3`, the frozen T2 probability model is not portable to T3.
