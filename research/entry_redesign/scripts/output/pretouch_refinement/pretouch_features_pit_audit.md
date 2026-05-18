# Point-In-Time 特征校验报告


## 摘要


- **特征总数**: 13
- **通过校验**: 13 ✅
- **未通过校验**: 0 ❌

> 所有特征均满足 Point-In-Time 约束：仅使用 touch_time 之前的数据。

## 校验汇总表


| 特征名 | 数据来源 | 计算逻辑 | 时间戳边界 | PIT 校验 |
|--------|----------|----------|------------|----------|
| time_of_day_hour_utc | touch 时刻聚合 | pd.to_datetime(touch_time, utc=True).dt.hour → int 0-23 | touch_time 本身（breakout 触发时刻，不使用 post-touch 数据） | ✅ 通过 |
| time_of_day_session_overlap | touch 时刻聚合 | 按 SESSION_OVERLAP_MAP 将 UTC hour 映射为 session overlap 枚举值 | touch_time 本身（breakout 触发时刻，不使用 post-touch 数据） | ✅ 通过 |
| volume_regime_ratio | 前 20 根 bar | 见对应 compute 函数实现 | touch_time 之前 | ✅ 通过 |
| volume_regime_percentile | 前 20 根 bar | 见对应 compute 函数实现 | touch_time 之前 | ✅ 通过 |
| realized_vol_30min | 前 30 分钟 1s bar | 见对应 compute 函数实现 | touch_time 之前 | ✅ 通过 |
| volatility_regime_cluster | 前 30 分钟 1s bar | 见对应 compute 函数实现 | touch_time 之前 | ✅ 通过 |
| level_prior_touch_count | 24h lookback | 见对应 compute 函数实现 | touch_time 之前 | ✅ 通过 |
| level_type | 24h lookback | 见对应 compute 函数实现 | touch_time 之前 | ✅ 通过 |
| prev5_bars_range_derivative | 前 N 根 bar | 见对应 compute 函数实现 | touch_time 之前 | ✅ 通过 |
| prev5_bars_body_wick_ratio | 前 N 根 bar | 见对应 compute 函数实现 | touch_time 之前 | ✅ 通过 |
| prev10_bars_direction_consistency | 前 N 根 bar | 见对应 compute 函数实现 | touch_time 之前 | ✅ 通过 |
| regime_transition_adx_30min | 前 30 分钟 1s→1min 聚合 | 见对应 compute 函数实现 | touch_time 之前 | ✅ 通过 |
| regime_transition_state | 前 30 分钟 1s→1min 聚合 | 见对应 compute 函数实现 | touch_time 之前 | ✅ 通过 |

## 逐特征详情


### time_of_day_hour_utc ✅


- **数据来源**: touch 时刻聚合
- **计算逻辑**: pd.to_datetime(touch_time, utc=True).dt.hour → int 0-23
- **时间戳边界**: touch_time 本身（breakout 触发时刻，不使用 post-touch 数据）
- **PIT 校验结果**: 通过

### time_of_day_session_overlap ✅


- **数据来源**: touch 时刻聚合
- **计算逻辑**: 按 SESSION_OVERLAP_MAP 将 UTC hour 映射为 session overlap 枚举值
- **时间戳边界**: touch_time 本身（breakout 触发时刻，不使用 post-touch 数据）
- **PIT 校验结果**: 通过

### volume_regime_ratio ✅


- **数据来源**: 前 20 根 bar
- **计算逻辑**: 见对应 compute 函数实现
- **时间戳边界**: touch_time 之前
- **PIT 校验结果**: 通过

### volume_regime_percentile ✅


- **数据来源**: 前 20 根 bar
- **计算逻辑**: 见对应 compute 函数实现
- **时间戳边界**: touch_time 之前
- **PIT 校验结果**: 通过

### realized_vol_30min ✅


- **数据来源**: 前 30 分钟 1s bar
- **计算逻辑**: 见对应 compute 函数实现
- **时间戳边界**: touch_time 之前
- **PIT 校验结果**: 通过

### volatility_regime_cluster ✅


- **数据来源**: 前 30 分钟 1s bar
- **计算逻辑**: 见对应 compute 函数实现
- **时间戳边界**: touch_time 之前
- **PIT 校验结果**: 通过

### level_prior_touch_count ✅


- **数据来源**: 24h lookback
- **计算逻辑**: 见对应 compute 函数实现
- **时间戳边界**: touch_time 之前
- **PIT 校验结果**: 通过

### level_type ✅


- **数据来源**: 24h lookback
- **计算逻辑**: 见对应 compute 函数实现
- **时间戳边界**: touch_time 之前
- **PIT 校验结果**: 通过

### prev5_bars_range_derivative ✅


- **数据来源**: 前 N 根 bar
- **计算逻辑**: 见对应 compute 函数实现
- **时间戳边界**: touch_time 之前
- **PIT 校验结果**: 通过

### prev5_bars_body_wick_ratio ✅


- **数据来源**: 前 N 根 bar
- **计算逻辑**: 见对应 compute 函数实现
- **时间戳边界**: touch_time 之前
- **PIT 校验结果**: 通过

### prev10_bars_direction_consistency ✅


- **数据来源**: 前 N 根 bar
- **计算逻辑**: 见对应 compute 函数实现
- **时间戳边界**: touch_time 之前
- **PIT 校验结果**: 通过

### regime_transition_adx_30min ✅


- **数据来源**: 前 30 分钟 1s→1min 聚合
- **计算逻辑**: 见对应 compute 函数实现
- **时间戳边界**: touch_time 之前
- **PIT 校验结果**: 通过

### regime_transition_state ✅


- **数据来源**: 前 30 分钟 1s→1min 聚合
- **计算逻辑**: 见对应 compute 函数实现
- **时间戳边界**: touch_time 之前
- **PIT 校验结果**: 通过
