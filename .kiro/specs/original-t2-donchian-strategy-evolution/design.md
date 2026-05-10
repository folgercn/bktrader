# Design Document

## Scope

本 design 只覆盖 research-only 的 R0/R1/R2。它不修改 `internal/`、`live`、`deployments/`、`.github/workflows/`，不生成 live `session.config`，不定义 live shadow / 灰度 / 全量流程。若 R2 通过，后续再另开 live migration spec。

AGENTS §2 Core Memory 的 Research_Baseline 固定为：`dir2_zero_initial=true`、`zero_initial_mode=reentry_window`、`reentry_size_schedule=[0.20, 0.10]`、`max_trades_per_bar=2`。当前 V4/V6 概率 runner 默认不是完整 reentry-window lifecycle，而是 event selection + 单次 1s execution + fixed `notional_share=0.20` 或 `model_notional_share` dynamic sizing；所有报告必须显式标注这一点。

当前已知 research baseline：`delay60 + feature60 + post_selection gate` 在 5 个 active months silo sum 为 `+6.09%`，尚未达到可下沉 live 的要求。本 design 的目标是用 Scheme B 的 regime/no-trade gate 去解释并过滤 BTCUSDT 2025-12 这类 validation pass 但 execute loss 的状态。

## Scheme Semantic Contract

| Scheme | Priority | Entry Source | Feature Horizon | Execution | Sizing | Lifecycle Claim | Current Gap |
|---|---:|---|---|---|---|---|---|
| B: `delay60 + feature60 + post_selection + regime_gate` | 1 | true `original_t2` intrabar touch | `feature_horizon_seconds=60 <= entry_delay_seconds=60` | V4 1s execution runner | ETH `hybrid_markov`; BTC dynamic only if validation InitialSL gate passes, otherwise fixed 20% | Baseline_Derived_Sizing | Need per-symbol fallback output and normalized portfolio metrics |
| A: `pretouch fast-clean + V4 probability` | 2 | true `original_t2` pre-touch state band | 5s family only | V4 1s execution runner | ETH optional dynamic; BTC fixed 20% | Baseline_Derived_Sizing | Secondary control only; not main path |
| C: `pretouch fast-clean + structure exit` | 3 | same as A | n/a | structure exit replay | fixed 20% only | Baseline_Derived_Sizing | Fail-fast only; small-sample overfit risk |

No scheme in this spec may claim Full_Reentry_Window_Lifecycle unless the runner explicitly implements current + next signal-bar reentry windows and slot0/slot1 lifecycle.

## Primary Flow

1. R0 hardening: keep requirements/design/tasks aligned on research-only scope, scheme contract, metric definitions, and non-goals.
2. R1 implementation: update the Scheme B runner/reporting surface so each run emits Active_Silo_Sum, Calendar_Normalized_Return, active/empty months, per-symbol sizing mode, and regime gate details.
3. R1 run: rerun Scheme B over at least 2025-06 through 2026-04 execute months with the current `+6.09%` post-selection gate as the baseline row.
4. R2 regime gate: add validation-only or pre-execute observable regime/no-trade gates, then rerun and compare against baseline. Gate candidates may use validation top-K metrics, prior closed-bar volatility state, prior-month realized trend/chop statistics, and model confidence dispersion. They must not use execute labels or complete current signal-bar OHLC.
5. R2 decision: promote only if Active_Silo_Sum and Calendar_Normalized_Return improve, PF >= 1.3, MaxDD <= 3%, active months >= 6, and no active month is below -2%.

## Metrics

All Scheme B reports must include:

- `active_silo_sum_pct`: simple sum across active symbol-month silos.
- `calendar_normalized_return_pct`: fixed calendar/symbol grid return with empty silos counted as 0%.
- `active_months`: number of execute months with at least one active symbol.
- `empty_months`: execute months with no active symbols.
- `symbol_silo_rows`: per symbol/month/topK rows with gate reason, sizing mode, selected events, trades, realistic return, PF, win rate, MaxDD, validation topK fields, and regime gate fields.
- `baseline_comparison`: delta versus the archived `delay60 + feature60 + post_selection gate` result (`+6.09%` active silo sum over 5 active months).

## Safety

The research harness must not emit live config or operational commands. In particular, `bktrader-ctl live control-reset` is out of scope; it is an exceptional production repair tool, not a research rollback primitive.

## Verification

R0 verification is static: check the Kiro docs for forbidden live migration wording and required contract fields.

R1/R2 verification is research-run based:

- Python compile for changed research scripts.
- A smoke run over a short period to prove output schemas.
- Full Scheme B matrix after smoke passes.
