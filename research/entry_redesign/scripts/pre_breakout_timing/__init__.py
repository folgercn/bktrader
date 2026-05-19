"""
pre_breakout_timing — Pre-Breakout Timing Classifier 模型包

利用 breakout 触发前已知的 pre-breakout 特征（ATR percentile、结构特征、
动量指标、level 强度）将每个 V6 gate event 预分类到最优入场延迟 regime，
替代固定统一 delay 策略。
"""
