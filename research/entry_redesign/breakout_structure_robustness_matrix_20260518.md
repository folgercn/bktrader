# Breakout Structure Robustness Matrix - 2026-05-18

Scope: research-only. This aligns the existing gate-search summaries across ETH late history, ETH early history, and BTC late history to separate robust structure from local overfit.

## Samples

- ETH late: `2025-06..2026-04`, 3-month train, `min_train_trades=5`
- ETH early: `2025-03..2025-06`, 1-month train, `min_train_trades=5`
- BTC late: `2025-06..2026-04`, 3-month train, `min_train_trades=5`
- Metric: `next_adverse_xslip10bps` calendar sum

## Matrix

| gate | ETH late | ETH early | BTC late | read |
|---|---:|---:|---:|---|
| `low_eff_low_atr_ctx12h_up` | 0.059167 | 0.002595 | n/a | Only ETH-positive gate across early/late, but sample is 15 + 5 trades and early worst month is still negative. |
| `low_eff_low_atr_ctx4h_up` | 0.066304 | -0.025601 | n/a | Strong late ETH risk overlay, fails early ETH. |
| `low_eff_low_atr_q20_q40` | 0.072854 | -0.045447 | -0.059469 | Main ETH late expansion, not robust cross-history/cross-asset. |
| `low_eff_low_atr_q30_q50` | 0.048193 | -0.051409 | -0.117570 | Wider gate increases fragility. |
| `low_eff_high_speed_q20_q60` | -0.014359 | -0.028700 | -0.033263 | Not a rescue gate. |
| `low_eff_q20` | 0.039887 | -0.099580 | -0.189908 | Low efficiency alone is an ETH late artifact. |

## Decision

No simple fixed gate currently passes a robust promotion threshold. The only gate positive on both ETH early and ETH late is `low_eff_low_atr_ctx12h_up`, but it is too small to be a live candidate: 5 early trades, 15 late trades, and non-flat worst months.

The useful signal is narrower:

- `eff_300s` low plus low ATR finds an ETH late pocket.
- Prior 12h side return seems to remove the worst early losses, but at the cost of almost all sample size.
- Prior 4h side return is better for late-window position scaling, but it does not survive early ETH history.
- BTC does not confirm these gates as alpha; it only shows loss reduction.

## Next Implication

Further tuning the same static gate family is unlikely to produce a durable 10-20% live candidate. The next research branch should rebuild the event source or model regime partition directly:

- include 4h/12h context as model features, not only post-hoc gates;
- train a context-aware sizing model that can output continuous scale, but require early ETH validation before trusting the scale;
- add a regime partition that is learned on trailing data and evaluated by month, not a monthly gate;
- keep rejecting candidates that only improve ETH late history while failing early ETH or BTC.

## Context Model Sizing Addendum

The context-aware RF sizing runner confirms the same message. The latest rerun includes hard-select variants (`*_000`) that can zero rejected events instead of keeping a 25% scale floor.

| sample | event pool | best variant | best adverse10 | worst_month | neg_months | active/trades | baseline adverse10 |
|---|---|---|---:|---:|---:|---:|---:|
| ETH late | `low_eff_low_atr` | `rf_rank_median_000` | 0.073127 | -0.004736 | 2 | 28/42 | 0.072854 |
| ETH early | `low_eff_low_atr` | `ctx12h_scaled025` | -0.009415 | -0.004335 | 3 | 17/17 | -0.045447 |
| BTC late | `low_eff_low_atr` | `rf_binary_000` | -0.008731 | -0.009684 | 1 | 11/38 | -0.059469 |
| ETH late | wide `baseline_model_advance` | `rf_binary_000` | 0.079500 | -0.042275 | 3 | 60/395 | -0.530945 |
| ETH early | wide `baseline_model_advance` | `rf_rank_q70_000` | -0.031129 | -0.018658 | 3 | 37/149 | -0.390524 |
| BTC late | wide `baseline_model_advance` | `rf_binary_000` | -0.078599 | -0.022265 | 8 | 46/385 | -0.622999 |

Read: the model is useful as a damage suppressor, not as alpha generation. Hard zero-selection can make the late-ETH wide pool positive, but it still has a deep worst month and does not pass early ETH or BTC. The narrow pool is only positive in ETH late history.

## Lead-Combo Addendum

The additive replay against canonical lead narrows the useful branch further:

| additive leg | active_source | extra_events | combo_adverse10 | delta_vs_lead | worst_month | neg_months | read |
|---|---:|---:|---:|---:|---:|---:|---|
| `low_eff_rf_rank_median_000` | 28 | 27 | 0.282871 | 0.053155 | -0.004736 | 2 | Best late-ETH additive shape so far; same return lift as bare `wf3`, fewer events, better tail. |
| `wide_rf_binary_000` | 60 | 57 | 0.282091 | 0.052375 | -0.042275 | 2 | Return lift exists, but tail is too deep for promotion. |
| `wide_rf_rank_q70_000` | 103 | 97 | 0.224097 | -0.005620 | -0.044589 | 2 | Fails as additive leg. |
| `wide_rf_binary_025` | 395 | 362 | 0.142841 | -0.086876 | -0.063290 | 3 | Rejected; the 25% floor carries adverse selection. |

This gives one better late-ETH candidate to falsify (`low_eff_rf_rank_median_000`), but it does not override the matrix decision because the corresponding narrow-pool early ETH model result is still negative.
