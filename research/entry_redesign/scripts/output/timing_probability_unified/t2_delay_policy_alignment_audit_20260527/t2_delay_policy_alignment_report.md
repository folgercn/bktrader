# T2 Delay Policy Alignment Audit

Research-only audit for replacing ex-post selected-delay evaluation with deployable fixed delay policies.

- Scenario: `next_adverse_xslip10bps`
- Events replayed: `68`
- Current qband selected-delay lead reference: `61.070917%`
- Base lead adverse10 reference: `22.971648%`
- Ledger timing counts: `{'fast': 60, 'slow': 8}`
- Current artifact timing counts: `{'fast': 59, 'skip': 6, 'slow': 3}`
- Current artifact slow count: `3`

## Ranked Policies

| policy                               | calendar_sum | delta_vs_qband | delta_vs_base | worst_month | neg_months | trades | untraded | max_dd     |
| ------------------------------------ | ------------ | -------------- | ------------- | ----------- | ---------- | ------ | -------- | ---------- |
| fast_d0_slow_pullback                | 58.600160%   | -2.470757pp    | 35.628512pp   | 3.316968%   | 0          | 62     | 6        | -2.775247% |
| artifact_model_fast_d0_slow_pullback | 57.982349%   | -3.088568pp    | 35.010701pp   | 3.316968%   | 0          | 62     | 6        | -2.754482% |
| fixed_d0_all_non_skip                | 56.342405%   | -4.728512pp    | 33.370757pp   | 3.316968%   | 0          | 62     | 6        | -2.754482% |
| artifact_model_fixed_d0              | 56.342405%   | -4.728512pp    | 33.370757pp   | 3.316968%   | 0          | 62     | 6        | -2.754482% |
| fast_d0_slow_d10                     | 56.337503%   | -4.733414pp    | 33.365855pp   | 3.316968%   | 0          | 62     | 6        | -2.734089% |
| fast_d0_slow_d15                     | 56.307520%   | -4.763397pp    | 33.335872pp   | 3.316968%   | 0          | 62     | 6        | -2.714192% |
| fast_d5_slow_pullback                | 55.140252%   | -5.930665pp    | 32.168604pp   | 0.365480%   | 0          | 62     | 6        | -2.702918% |
| fixed_d5_all_non_skip                | 53.053083%   | -8.017834pp    | 30.081435pp   | 0.365480%   | 0          | 62     | 6        | -2.640510% |
| fast_d5_slow_d10                     | 52.877594%   | -8.193323pp    | 29.905946pp   | 0.365480%   | 0          | 62     | 6        | -2.661760% |
| fast_d5_slow_d15                     | 52.847611%   | -8.223306pp    | 29.875963pp   | 0.365480%   | 0          | 62     | 6        | -2.641863% |
| fixed_d10_all_non_skip               | 47.424282%   | -13.646635pp   | 24.452634pp   | -0.753471%  | 1          | 62     | 6        | -2.848721% |
| fixed_d15_all_non_skip               | 43.860560%   | -17.210357pp   | 20.888912pp   | 0.936907%   | 0          | 62     | 6        | -2.640415% |
| fixed_pullback_all_non_skip          | 22.639653%   | -38.431264pp   | -0.331995pp   | -1.993544%  | 1          | 62     | 6        | -4.717254% |

## Read

- `fixed_d0_all_non_skip` is the closest proxy for current live immediate-entry timing.
- The reference `lead_quantity_0p20_0p40_adverse10` uses ex-post selected delay inside each predicted timing bucket.
- If fixed policies materially underperform the selected-delay reference, live alignment needs a deployable delay policy/model before treating the headline as live-equivalent.
