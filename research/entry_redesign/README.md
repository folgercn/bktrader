# Entry Redesign Research Harness

本包实现 `original_t2` breakout **Entry Layer** 改造的研究 runner、脚本与报告。

## 范围

- **research-only**：只写入 `research/` 目录，不触碰 `internal/`、`deployments/`、`.github/workflows/`。
- **只改 Entry Layer**：六元组 `(Entry_Delay_Seconds, Feature_Horizon_Seconds, Trigger_Confirmation, Entry_Price_Mode, Pretouch_State_Band, Posttouch_Quality_Band)`。
- **跨层联合调参延后到独立 spec**

## Research_Baseline（AGENTS §2 Core Memory，本 spec 不修改）

以下四条参数逐字符串复述，作为 Sizing Layer 常量锁定：

- `dir2_zero_initial=true`
- `zero_initial_mode=reentry_window`
- `reentry_size_schedule=[0.20, 0.10]`
- `max_trades_per_bar=2`

即同一根 signal bar 内，第 1 次真实下单为 20%，第 2 次真实下单为 10%。

## 依赖

- Python >= 3.10
- numpy, scipy, scikit-learn, pandas, matplotlib
- 禁止引入 torch / tensorflow / jax（AGENTS §2：GRU 已证伪）

## 三层结构

| 层 | 职责 | 本 spec 可改 |
|---|---|---|
| Entry Layer | 触发、确认、入场价、pre/post-touch gating | 是 |
| Gate Layer | candidate_001 三阈值过滤 | 否（快照只读） |
| Sizing Layer | `reentry_size_schedule=[0.20, 0.10]`、`max_trades_per_bar=2` | 否 |

## 运行

```bash
cd research/entry_redesign
pip install -e ".[test]"
pytest tests/
```
