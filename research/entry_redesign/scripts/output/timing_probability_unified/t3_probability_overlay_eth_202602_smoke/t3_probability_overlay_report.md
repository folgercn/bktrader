# T3 Probability Overlay Audit

Research-only audit for using the frozen probability model as a T3 quality layer.

- Model: `/Users/wuyaocheng/Downloads/bkTrader/data/pretouch_model.json`
- Months: 2026-02
- Symbols: ETHUSDT
- Trades: 7
- Scored events: 213
- Runtime: 83.26s

## Gate Summary

| gate                      | trades | net_after_fee_pct | avg_trade_net_pct | win_rate_pct | worst_symbol_month_pct | negative_symbol_months |
| ------------------------- | ------ | ----------------- | ----------------- | ------------ | ---------------------- | ---------------------- |
| ext_abs_le_0p05           | 5      | 0.333578          | 0.066716          | 60.000000    | 0.333578               | 0                      |
| all_t3                    | 7      | 0.280005          | 0.040001          | 57.140000    | 0.280005               | 0                      |
| matched_model_events      | 7      | 0.280005          | 0.040001          | 57.140000    | 0.280005               | 0                      |
| side_long                 | 7      | 0.280005          | 0.040001          | 57.140000    | 0.280005               | 0                      |
| ext_abs_le_0p10           | 6      | 0.271303          | 0.045217          | 50.000000    | 0.271303               | 0                      |
| model_live_quality        | 6      | 0.236888          | 0.039481          | 50.000000    | 0.236888               | 0                      |
| rf_ge_0p55                | 5      | 0.008904          | 0.001781          | 60.000000    | 0.008904               | 0                      |
| rf_ge_0p60                | 5      | 0.008904          | 0.001781          | 60.000000    | 0.008904               | 0                      |
| side_short                | 0      | 0.000000          | 0.000000          | 0.000000     | 0.000000               | 0                      |
| rf_ge_0p65                | 2      | -0.012596         | -0.006298         | 50.000000    | -0.012596              | 1                      |
| rf_ge_0p70                | 2      | -0.012596         | -0.006298         | 50.000000    | -0.012596              | 1                      |
| rf_ge_0p45                | 6      | -0.053371         | -0.008895         | 50.000000    | -0.053371              | 1                      |
| rf_ge_0p50                | 6      | -0.053371         | -0.008895         | 50.000000    | -0.053371              | 1                      |
| timing_fast_or_slow       | 4      | -0.066169         | -0.016542         | 50.000000    | -0.066169              | 1                      |
| timing_fast               | 4      | -0.066169         | -0.016542         | 50.000000    | -0.066169              | 1                      |
| model_live_quality_timing | 3      | -0.109287         | -0.036429         | 33.330000    | -0.109287              | 1                      |

## Probability Buckets

| family            | bucket    | trades | net_after_fee_pct | avg_trade_net_pct | win_rate_pct |
| ----------------- | --------- | ------ | ----------------- | ----------------- | ------------ |
| rf_probability    | <0.35     | 1      | 0.333376          | 0.333376          | 100.000000   |
| rf_probability    | 0.60-0.70 | 3      | 0.021501          | 0.007167          | 66.670000    |
| rf_probability    | >=0.70    | 2      | -0.012596         | -0.006298         | 50.000000    |
| rf_probability    | 0.50-0.55 | 1      | -0.062276         | -0.062276         | 0.000000     |
| side              | long      | 7      | 0.280005          | 0.040001          | 57.140000    |
| timing_prediction | skip      | 3      | 0.346174          | 0.115391          | 66.670000    |
| timing_prediction | fast      | 4      | -0.066169         | -0.016542         | 50.000000    |

## Read

- This is not a promoted gate yet; it is the evidence layer before rerunning lifecycle with skips/sizing.
- A useful probability layer should improve T3 net contribution without relying on tiny trade counts.
- If the best buckets lose against `all_t3`, the frozen T2 probability model is not portable to T3.
