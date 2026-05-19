# Gate 放宽策略探索报告

生成时间: 2026-05-13T15:40:29.741510Z

## 概述

本报告系统性探索了放宽 `candidate_001` gate 条件的 4 个维度，目标是将事件池从 116 扩大到 200+ events。

- **最终事件池大小**: 1013
- **包含原 116 events**: 是
- **达到 200 events 目标**: 是
- **选定策略**: 选择维度 C：直接使用 events_execution_labeled.csv（original_t2_delay60），按 bars_cache 覆盖时间范围筛选。产出 1013 events，包含全部 116 个原始 events。理由：最简单直接，事件数量充足，保证向后兼容。

## 各维度探索结果

### 维度: entry_reason

**描述**: entry_reason 放宽：包含所有 entry 类型（Zero-Initial-Reentry + SL-Reentry），从 V6 lifecycle ledger calendar_holdout 中提取

| 指标 | 值 |
|------|------|
| 事件数量 | 116 |
| 与原 116 events 重叠 | 116 |
| 是否充足 (≥150) | 否 |
| Symbol 分布 | {'BTCUSDT': 60, 'ETHUSDT': 56} |
| Side 分布 | {'short': 74, 'long': 42} |
| 时间范围 | 2025-06-01 12:33:51+00:00 ~ 2026-03-27 10:37:22+00:00 |

> ⚠️ 该维度产出 116 events < 150，标注为 `insufficient_expansion`

### 维度: lifecycle_ledger

**描述**: lifecycle ledger 扩展：合并 calendar_holdout + observed_fill 两个 V6 walkforward 目录的 Zero-Initial-Reentry entries

| 指标 | 值 |
|------|------|
| 事件数量 | 117 |
| 与原 116 events 重叠 | 116 |
| 是否充足 (≥150) | 否 |
| Symbol 分布 | {'BTCUSDT': 61, 'ETHUSDT': 56} |
| Side 分布 | {'short': 74, 'long': 43} |
| 时间范围 | 2025-06-01 12:33:51+00:00 ~ 2026-03-27 10:37:22+00:00 |

> ⚠️ 该维度产出 117 events < 150，标注为 `insufficient_expansion`

### 维度: direct_events

**描述**: 直接使用 events_execution_labeled.csv（original_t2_delay60）：跳过 V6 gate 匹配，仅按 bars_cache 覆盖时间范围（2025-06 至 2026-04）筛选

| 指标 | 值 |
|------|------|
| 事件数量 | 1013 |
| 与原 116 events 重叠 | 116 |
| 是否充足 (≥150) | 是 |
| Symbol 分布 | {'BTCUSDT': 580, 'ETHUSDT': 433} |
| Side 分布 | {'long': 532, 'short': 481} |
| 时间范围 | 2025-06-01 09:09:10+00:00 ~ 2026-04-30 04:37:06+00:00 |

### 维度: time_range

**描述**: 时间范围扩展：使用 baseline_plus_t3_delay60 事件源（包含 original_t2 + t3_swing），按 bars_cache 覆盖时间范围筛选

| 指标 | 值 |
|------|------|
| 事件数量 | 1312 |
| 与原 116 events 重叠 | 116 |
| 是否充足 (≥150) | 是 |
| Symbol 分布 | {'BTCUSDT': 739, 'ETHUSDT': 573} |
| Side 分布 | {'long': 691, 'short': 621} |
| 时间范围 | 2025-06-01 09:09:10+00:00 ~ 2026-04-30 19:24:11+00:00 |

## 策略选择理由

选择维度 C：直接使用 events_execution_labeled.csv（original_t2_delay60），按 bars_cache 覆盖时间范围筛选。产出 1013 events，包含全部 116 个原始 events。理由：最简单直接，事件数量充足，保证向后兼容。
