# Breakout Structure Confirmation Sweep — ETH Pretouch Timing Lead

Generated: 2026-05-18T03:08:47.232194+00:00

Scope: research-only, ETHUSDT 1h, current production shape `restrictive_0p5bps`, frozen `data/pretouch_model.json` `20260515_v1`.
Each confirmation variant reprices entry to the first confirming 1s close. No live defaults are changed.

## Summary

| variant                               | confirmed_events | confirmed_advance_events | d0_traded_events | same_close_calendar_sum | same_close_worst_sm | same_close_neg_sm | adverse10_calendar_sum | adverse10_worst_sm | adverse10_neg_sm | median_confirmation_delay_seconds | avg_confirmation_close_extension_atr | avg_pre_confirm_adverse_atr |
| ------------------------------------- | ---------------- | ------------------------ | ---------------- | ----------------------- | ------------------- | ----------------- | ---------------------- | ------------------ | ---------------- | --------------------------------- | ------------------------------------ | --------------------------- |
| baseline_touch_entry                  | 736              | 561                      | 561              | -0.319441               | -0.116655           | 7                 | -1.029829              | -0.181796          | 10               | 0.000000                          | 0.026354                             | 0.000000                    |
| touch_close_reclaim                   | 577              | 451                      | 451              | -0.375397               | -0.115280           | 9                 | -0.939370              | -0.169230          | 10               | 0.000000                          | 0.042077                             | 0.041975                    |
| touch_close_ext_ge_0p03atr            | 231              | 151                      | 151              | -0.121593               | -0.051991           | 7                 | -0.313283              | -0.080542          | 10               | 0.000000                          | 0.088599                             | 0.052494                    |
| follow_60s_ext_ge_0p03atr             | 584              | 434                      | 434              | -0.301808               | -0.111363           | 8                 | -0.849613              | -0.166381          | 10               | 1.000000                          | 0.071066                             | 0.052009                    |
| follow_180s_ext_ge_0p05atr            | 581              | 430                      | 430              | -0.251674               | -0.112286           | 8                 | -0.791211              | -0.164381          | 10               | 3.000000                          | 0.090160                             | 0.060332                    |
| follow_300s_ext_ge_0p05atr            | 609              | 453                      | 453              | -0.270407               | -0.107023           | 8                 | -0.836552              | -0.161186          | 10               | 4.000000                          | 0.088898                             | 0.064064                    |
| follow_300s_ext_ge_0p10atr            | 501              | 365                      | 365              | -0.358225               | -0.137005           | 9                 | -0.811344              | -0.183872          | 10               | 15.000000                         | 0.138964                             | 0.070649                    |
| clean_180s_ext_ge_0p05_adv_le_0p05atr | 332              | 271                      | 271              | -0.330015               | -0.096972           | 8                 | -0.670453              | -0.131152          | 9                | 3.000000                          | 0.081097                             | 0.022630                    |
| clean_300s_ext_ge_0p05_adv_le_0p05atr | 333              | 272                      | 272              | -0.324961               | -0.091918           | 8                 | -0.666328              | -0.127495          | 9                | 3.000000                          | 0.081122                             | 0.022700                    |
| clean_300s_ext_ge_0p10_adv_le_0p10atr | 375              | 294                      | 294              | -0.304036               | -0.114726           | 9                 | -0.669730              | -0.153209          | 11               | 12.000000                         | 0.133573                             | 0.039054                    |

## Adverse Fill Matrix

| variant                               | scenario                | calendar_sum_gate_on | worst_sm_gate_on | neg_sm_count | trade_count_gate_on |
| ------------------------------------- | ----------------------- | -------------------- | ---------------- | ------------ | ------------------- |
| baseline_touch_entry                  | same_close_xslip0bps    | -0.319441            | -0.116655        | 7            | 561                 |
| baseline_touch_entry                  | next_close_xslip0bps    | -0.367571            | -0.115610        | 8            | 561                 |
| baseline_touch_entry                  | next_adverse_xslip0bps  | -0.442779            | -0.122389        | 9            | 561                 |
| baseline_touch_entry                  | next_adverse_xslip1bps  | -0.501484            | -0.128330        | 9            | 561                 |
| baseline_touch_entry                  | next_adverse_xslip3bps  | -0.618894            | -0.140211        | 9            | 561                 |
| baseline_touch_entry                  | next_adverse_xslip5bps  | -0.736304            | -0.152093        | 9            | 561                 |
| baseline_touch_entry                  | next_adverse_xslip7bps  | -0.853714            | -0.163974        | 9            | 561                 |
| baseline_touch_entry                  | next_adverse_xslip10bps | -1.029829            | -0.181796        | 10           | 561                 |
| touch_close_reclaim                   | same_close_xslip0bps    | -0.375397            | -0.115280        | 9            | 451                 |
| touch_close_reclaim                   | next_close_xslip0bps    | -0.411894            | -0.118181        | 9            | 451                 |
| touch_close_reclaim                   | next_adverse_xslip0bps  | -0.467746            | -0.120915        | 9            | 451                 |
| touch_close_reclaim                   | next_adverse_xslip1bps  | -0.514909            | -0.125585        | 9            | 451                 |
| touch_close_reclaim                   | next_adverse_xslip3bps  | -0.609234            | -0.135284        | 9            | 451                 |
| touch_close_reclaim                   | next_adverse_xslip5bps  | -0.703558            | -0.144983        | 9            | 451                 |
| touch_close_reclaim                   | next_adverse_xslip7bps  | -0.797883            | -0.154682        | 9            | 451                 |
| touch_close_reclaim                   | next_adverse_xslip10bps | -0.939370            | -0.169230        | 10           | 451                 |
| touch_close_ext_ge_0p03atr            | same_close_xslip0bps    | -0.121593            | -0.051991        | 7            | 151                 |
| touch_close_ext_ge_0p03atr            | next_close_xslip0bps    | -0.132268            | -0.057806        | 7            | 151                 |
| touch_close_ext_ge_0p03atr            | next_adverse_xslip0bps  | -0.160013            | -0.060312        | 9            | 151                 |
| touch_close_ext_ge_0p03atr            | next_adverse_xslip1bps  | -0.175340            | -0.062335        | 9            | 151                 |
| touch_close_ext_ge_0p03atr            | next_adverse_xslip3bps  | -0.205994            | -0.066381        | 9            | 151                 |
| touch_close_ext_ge_0p03atr            | next_adverse_xslip5bps  | -0.236648            | -0.070427        | 9            | 151                 |
| touch_close_ext_ge_0p03atr            | next_adverse_xslip7bps  | -0.267302            | -0.074473        | 9            | 151                 |
| touch_close_ext_ge_0p03atr            | next_adverse_xslip10bps | -0.313283            | -0.080542        | 10           | 151                 |
| follow_60s_ext_ge_0p03atr             | same_close_xslip0bps    | -0.301808            | -0.111363        | 8            | 434                 |
| follow_60s_ext_ge_0p03atr             | next_close_xslip0bps    | -0.325980            | -0.110816        | 8            | 434                 |
| follow_60s_ext_ge_0p03atr             | next_adverse_xslip0bps  | -0.396497            | -0.117491        | 9            | 434                 |
| follow_60s_ext_ge_0p03atr             | next_adverse_xslip1bps  | -0.441809            | -0.122380        | 9            | 434                 |
| follow_60s_ext_ge_0p03atr             | next_adverse_xslip3bps  | -0.532432            | -0.132158        | 9            | 434                 |
| follow_60s_ext_ge_0p03atr             | next_adverse_xslip5bps  | -0.623055            | -0.141936        | 10           | 434                 |
| follow_60s_ext_ge_0p03atr             | next_adverse_xslip7bps  | -0.713678            | -0.151714        | 10           | 434                 |
| follow_60s_ext_ge_0p03atr             | next_adverse_xslip10bps | -0.849613            | -0.166381        | 10           | 434                 |
| follow_180s_ext_ge_0p05atr            | same_close_xslip0bps    | -0.251674            | -0.112286        | 8            | 430                 |
| follow_180s_ext_ge_0p05atr            | next_close_xslip0bps    | -0.268258            | -0.112986        | 8            | 430                 |
| follow_180s_ext_ge_0p05atr            | next_adverse_xslip0bps  | -0.341480            | -0.117009        | 8            | 430                 |
| follow_180s_ext_ge_0p05atr            | next_adverse_xslip1bps  | -0.386453            | -0.121747        | 8            | 430                 |
| follow_180s_ext_ge_0p05atr            | next_adverse_xslip3bps  | -0.476399            | -0.131221        | 9            | 430                 |
| follow_180s_ext_ge_0p05atr            | next_adverse_xslip5bps  | -0.566345            | -0.140695        | 9            | 430                 |
| follow_180s_ext_ge_0p05atr            | next_adverse_xslip7bps  | -0.656291            | -0.150169        | 10           | 430                 |
| follow_180s_ext_ge_0p05atr            | next_adverse_xslip10bps | -0.791211            | -0.164381        | 10           | 430                 |
| follow_300s_ext_ge_0p05atr            | same_close_xslip0bps    | -0.270407            | -0.107023        | 8            | 453                 |
| follow_300s_ext_ge_0p05atr            | next_close_xslip0bps    | -0.287185            | -0.107790        | 8            | 453                 |
| follow_300s_ext_ge_0p05atr            | next_adverse_xslip0bps  | -0.362686            | -0.111839        | 8            | 453                 |
| follow_300s_ext_ge_0p05atr            | next_adverse_xslip1bps  | -0.410073            | -0.116774        | 8            | 453                 |
| follow_300s_ext_ge_0p05atr            | next_adverse_xslip3bps  | -0.504846            | -0.126643        | 9            | 453                 |
| follow_300s_ext_ge_0p05atr            | next_adverse_xslip5bps  | -0.599619            | -0.136513        | 10           | 453                 |
| follow_300s_ext_ge_0p05atr            | next_adverse_xslip7bps  | -0.694392            | -0.146382        | 10           | 453                 |
| follow_300s_ext_ge_0p05atr            | next_adverse_xslip10bps | -0.836552            | -0.161186        | 10           | 453                 |
| follow_300s_ext_ge_0p10atr            | same_close_xslip0bps    | -0.358225            | -0.137005        | 9            | 365                 |
| follow_300s_ext_ge_0p10atr            | next_close_xslip0bps    | -0.361330            | -0.138438        | 9            | 365                 |
| follow_300s_ext_ge_0p10atr            | next_adverse_xslip0bps  | -0.430179            | -0.142608        | 10           | 365                 |
| follow_300s_ext_ge_0p10atr            | next_adverse_xslip1bps  | -0.468295            | -0.146735        | 10           | 365                 |
| follow_300s_ext_ge_0p10atr            | next_adverse_xslip3bps  | -0.544528            | -0.154987        | 10           | 365                 |
| follow_300s_ext_ge_0p10atr            | next_adverse_xslip5bps  | -0.620761            | -0.163240        | 10           | 365                 |
| follow_300s_ext_ge_0p10atr            | next_adverse_xslip7bps  | -0.696994            | -0.171493        | 10           | 365                 |
| follow_300s_ext_ge_0p10atr            | next_adverse_xslip10bps | -0.811344            | -0.183872        | 10           | 365                 |
| clean_180s_ext_ge_0p05_adv_le_0p05atr | same_close_xslip0bps    | -0.330015            | -0.096972        | 8            | 271                 |
| clean_180s_ext_ge_0p05_adv_le_0p05atr | next_close_xslip0bps    | -0.344223            | -0.097265        | 8            | 271                 |
| clean_180s_ext_ge_0p05_adv_le_0p05atr | next_adverse_xslip0bps  | -0.382992            | -0.100174        | 8            | 271                 |
| clean_180s_ext_ge_0p05_adv_le_0p05atr | next_adverse_xslip1bps  | -0.411739            | -0.103271        | 9            | 271                 |
| clean_180s_ext_ge_0p05_adv_le_0p05atr | next_adverse_xslip3bps  | -0.469231            | -0.109467        | 9            | 271                 |
| clean_180s_ext_ge_0p05_adv_le_0p05atr | next_adverse_xslip5bps  | -0.526723            | -0.115663        | 9            | 271                 |
| clean_180s_ext_ge_0p05_adv_le_0p05atr | next_adverse_xslip7bps  | -0.584215            | -0.121859        | 9            | 271                 |
| clean_180s_ext_ge_0p05_adv_le_0p05atr | next_adverse_xslip10bps | -0.670453            | -0.131152        | 9            | 271                 |
| clean_300s_ext_ge_0p05_adv_le_0p05atr | same_close_xslip0bps    | -0.324961            | -0.091918        | 8            | 272                 |
| clean_300s_ext_ge_0p05_adv_le_0p05atr | next_close_xslip0bps    | -0.339137            | -0.092179        | 8            | 272                 |
| clean_300s_ext_ge_0p05_adv_le_0p05atr | next_adverse_xslip0bps  | -0.377932            | -0.095113        | 8            | 272                 |
| clean_300s_ext_ge_0p05_adv_le_0p05atr | next_adverse_xslip1bps  | -0.406771            | -0.098304        | 9            | 272                 |
| clean_300s_ext_ge_0p05_adv_le_0p05atr | next_adverse_xslip3bps  | -0.464451            | -0.104687        | 9            | 272                 |
| clean_300s_ext_ge_0p05_adv_le_0p05atr | next_adverse_xslip5bps  | -0.522130            | -0.111070        | 9            | 272                 |
| clean_300s_ext_ge_0p05_adv_le_0p05atr | next_adverse_xslip7bps  | -0.579809            | -0.117453        | 9            | 272                 |
| clean_300s_ext_ge_0p05_adv_le_0p05atr | next_adverse_xslip10bps | -0.666328            | -0.127495        | 9            | 272                 |
| clean_300s_ext_ge_0p10_adv_le_0p10atr | same_close_xslip0bps    | -0.304036            | -0.114726        | 9            | 294                 |
| clean_300s_ext_ge_0p10_adv_le_0p10atr | next_close_xslip0bps    | -0.307038            | -0.115903        | 9            | 294                 |
| clean_300s_ext_ge_0p10_adv_le_0p10atr | next_adverse_xslip0bps  | -0.360592            | -0.119369        | 9            | 294                 |
| clean_300s_ext_ge_0p10_adv_le_0p10atr | next_adverse_xslip1bps  | -0.391506            | -0.122753        | 9            | 294                 |
| clean_300s_ext_ge_0p10_adv_le_0p10atr | next_adverse_xslip3bps  | -0.453334            | -0.129521        | 9            | 294                 |
| clean_300s_ext_ge_0p10_adv_le_0p10atr | next_adverse_xslip5bps  | -0.515161            | -0.136289        | 9            | 294                 |
| clean_300s_ext_ge_0p10_adv_le_0p10atr | next_adverse_xslip7bps  | -0.576989            | -0.143057        | 9            | 294                 |
| clean_300s_ext_ge_0p10_adv_le_0p10atr | next_adverse_xslip10bps | -0.669730            | -0.153209        | 11           | 294                 |

## Monthly PnL

| year_month | baseline_touch_entry | clean_180s_ext_ge_0p05_adv_le_0p05atr | clean_300s_ext_ge_0p05_adv_le_0p05atr | clean_300s_ext_ge_0p10_adv_le_0p10atr | follow_180s_ext_ge_0p05atr | follow_300s_ext_ge_0p05atr | follow_300s_ext_ge_0p10atr | follow_60s_ext_ge_0p03atr | touch_close_ext_ge_0p03atr | touch_close_reclaim |
| ---------- | -------------------- | ------------------------------------- | ------------------------------------- | ------------------------------------- | -------------------------- | -------------------------- | -------------------------- | ------------------------- | -------------------------- | ------------------- |
| 2025-06    | -0.089237            | -0.055714                             | -0.055714                             | -0.046598                             | -0.069317                  | -0.066542                  | -0.048991                  | -0.054738                 | -0.051991                  | -0.102665           |
| 2025-07    | -0.116655            | -0.096972                             | -0.091918                             | -0.114726                             | -0.112286                  | -0.107023                  | -0.137005                  | -0.111363                 | -0.048489                  | -0.115280           |
| 2025-08    | -0.099919            | -0.081073                             | -0.081073                             | -0.044878                             | -0.074438                  | -0.081148                  | -0.059416                  | -0.071675                 | -0.015970                  | -0.113505           |
| 2025-09    | -0.030508            | -0.026388                             | -0.026388                             | -0.027614                             | -0.011281                  | -0.016492                  | -0.045803                  | -0.040611                 | 0.011317                   | -0.043880           |
| 2025-10    | -0.080559            | -0.030448                             | -0.030448                             | -0.037972                             | -0.011697                  | -0.012949                  | -0.051943                  | -0.025577                 | -0.027312                  | -0.071211           |
| 2025-11    | 0.002162             | -0.081236                             | -0.081236                             | -0.054412                             | -0.050383                  | -0.045225                  | -0.010621                  | -0.024220                 | -0.001208                  | -0.000218           |
| 2025-12    | -0.065459            | -0.026714                             | -0.026714                             | -0.008489                             | -0.034658                  | -0.034658                  | -0.007464                  | -0.054601                 | -0.012672                  | -0.036622           |
| 2026-01    | -0.039055            | -0.009300                             | -0.009300                             | -0.016381                             | -0.026549                  | -0.025104                  | -0.041338                  | -0.039530                 | -0.026229                  | -0.025661           |
| 2026-02    | 0.128830             | 0.043296                              | 0.043296                              | 0.032094                              | 0.097155                   | 0.084334                   | 0.043970                   | 0.092321                  | 0.009580                   | 0.096136            |
| 2026-03    | 0.070376             | 0.031628                              | 0.031628                              | 0.024861                              | 0.033507                   | 0.027339                   | 0.003312                   | 0.026777                  | 0.040566                   | 0.040493            |
| 2026-04    | 0.000582             | 0.002905                              | 0.002905                              | -0.009922                             | 0.008272                   | 0.007062                   | -0.002926                  | 0.001408                  | 0.000816                   | -0.002984           |

## Interpretation

- `baseline_touch_entry` is the current production-shape D0 lens from the prior sweep.
- Confirmation variants are post-touch structure filters, not live defaults.
- A variant is only interesting if it improves calendar sum and worst single month under same-close and remains tolerable under next-second adverse stress.
- Positive subset results here would still need canonical event-source alignment before promotion, because this lens rebuilds a broader live-like event pool.

## Diagnostics

```json
{
  "base_events_path": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_shape_expansion_events_restrictive_0p5bps.csv",
  "base_events": 736,
  "eval_start": "2025-06-01T00:00:00+00:00",
  "eval_end_exclusive": "2026-05-01T00:00:00+00:00",
  "base_share": 0.8,
  "exec_params": {
    "breakeven_at_r": 0.8,
    "cost_lock_bps": 10.0,
    "entry_fee": 0.0002,
    "exit_fee": 0.0004,
    "initial_stop_atr": 0.45,
    "max_hold_hours": 2.0,
    "min_stop_bps": 12.0,
    "slippage": 0.0002,
    "stop_buffer_atr": 0.05,
    "stop_cap_atr": 0.8,
    "trail_buffer_atr": 0.05,
    "trail_start_r": 1.5
  },
  "variants": {
    "baseline_touch_entry": {
      "confirmed_events": 736,
      "confirmed_rate": 1.0,
      "confirmed_advance_events": 561,
      "description": "current production shape replay: enter at first touch close"
    },
    "touch_close_reclaim": {
      "confirmed_events": 577,
      "confirmed_rate": 0.7839673913043478,
      "confirmed_advance_events": 451,
      "description": "touch second must close beyond the breakout level"
    },
    "touch_close_ext_ge_0p03atr": {
      "confirmed_events": 231,
      "confirmed_rate": 0.3138586956521739,
      "confirmed_advance_events": 151,
      "description": "touch second close extension >= 0.03 ATR"
    },
    "follow_60s_ext_ge_0p03atr": {
      "confirmed_events": 584,
      "confirmed_rate": 0.7934782608695652,
      "confirmed_advance_events": 434,
      "description": "within 60s, first close extension >= 0.03 ATR"
    },
    "follow_180s_ext_ge_0p05atr": {
      "confirmed_events": 581,
      "confirmed_rate": 0.7894021739130435,
      "confirmed_advance_events": 430,
      "description": "within 180s, first close extension >= 0.05 ATR"
    },
    "follow_300s_ext_ge_0p05atr": {
      "confirmed_events": 609,
      "confirmed_rate": 0.8274456521739131,
      "confirmed_advance_events": 453,
      "description": "within 300s, first close extension >= 0.05 ATR"
    },
    "follow_300s_ext_ge_0p10atr": {
      "confirmed_events": 501,
      "confirmed_rate": 0.6807065217391305,
      "confirmed_advance_events": 365,
      "description": "within 300s, first close extension >= 0.10 ATR"
    },
    "clean_180s_ext_ge_0p05_adv_le_0p05atr": {
      "confirmed_events": 332,
      "confirmed_rate": 0.45108695652173914,
      "confirmed_advance_events": 271,
      "description": "within 180s, close extension >= 0.05 ATR and adverse before confirmation <= 0.05 ATR"
    },
    "clean_300s_ext_ge_0p05_adv_le_0p05atr": {
      "confirmed_events": 333,
      "confirmed_rate": 0.452445652173913,
      "confirmed_advance_events": 272,
      "description": "within 300s, close extension >= 0.05 ATR and adverse before confirmation <= 0.05 ATR"
    },
    "clean_300s_ext_ge_0p10_adv_le_0p10atr": {
      "confirmed_events": 375,
      "confirmed_rate": 0.5095108695652174,
      "confirmed_advance_events": 294,
      "description": "within 300s, close extension >= 0.10 ATR and adverse before confirmation <= 0.10 ATR"
    }
  },
  "runtime_seconds": 60.241759061813354
}
```
