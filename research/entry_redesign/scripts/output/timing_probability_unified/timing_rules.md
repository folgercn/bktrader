# Timing Classifier Decision Rules (DT3)

规则 1: 若 roundtrip_cost_atr ≤ 0.23 且 level_to_prev_close_atr ≤ 0.34 且 level_to_prev_close_atr ≤ 0.29
  → fast
  [训练样本: 2 个, 置信度: 100.0%]

规则 2: 若 roundtrip_cost_atr ≤ 0.23 且 level_to_prev_close_atr ≤ 0.34 且 level_to_prev_close_atr > 0.29
  → slow
  [训练样本: 3 个, 置信度: 66.7%]

规则 3: 若 roundtrip_cost_atr ≤ 0.23 且 level_to_prev_close_atr > 0.34 且 signal_atr_percentile ≤ 0.63
  → fast
  [训练样本: 20 个, 置信度: 100.0%]

规则 4: 若 roundtrip_cost_atr ≤ 0.23 且 level_to_prev_close_atr > 0.34 且 signal_atr_percentile > 0.63
  → fast
  [训练样本: 13 个, 置信度: 76.9%]

规则 5: 若 roundtrip_cost_atr > 0.23
  → slow
  [训练样本: 2 个, 置信度: 100.0%]
