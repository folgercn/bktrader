import pandas as pd
import pandas_ta as ta
import os

def process_1min_and_gen_4h(input_file, output_1min_file, output_4h_file):
    print(f"正在加载 1min 原始数据: {input_file}...")
    
    # 1. 加载数据
    # 根据你提供的格式，读取必要的 OHLCV 和时间戳
    df = pd.read_csv(input_file)
    
    # 2. 时间戳对齐
    # 将 timestamp_utc 转换为 datetime 并设为索引
    df['timestamp'] = pd.to_datetime(df['timestamp_utc'])
    df.set_index('timestamp', inplace=True)
    df.sort_index(inplace=True)
    
    # 只保留回测需要的列，节省内存
    cols_to_keep = ['open', 'high', 'low', 'close', 'volume']
    df_1min = df[cols_to_keep].copy()
    
    # 保存处理后的 1min 数据（用于 1min 级回测）
    df_1min.to_csv(output_1min_file)
    print(f"✅ 已保存标准化 1min 数据: {output_1min_file}")

    # 3. 合成 4H 数据
    print("正在聚合 4H 周期数据...")
    df_4h = df_1min.resample('4h').agg({
        'open': 'first',
        'high': 'max',
        'low': 'min',
        'close': 'last',
        'volume': 'sum'
    }).dropna()

    # 4. 计算策略锚点指标 (4H 级别)
    print("正在计算 4H 策略指标...")
    df_4h['ma20'] = ta.sma(df_4h['close'], length=20)
    df_4h['atr'] = ta.atr(df_4h['high'], df_4h['low'], df_4h['close'], length=14)
    
    # 形态过滤位 (shift 确保无未来函数)
    df_4h['prev_high_1'] = df_4h['high'].shift(1)
    df_4h['prev_high_2'] = df_4h['high'].shift(2)
    df_4h['prev_low_1'] = df_4h['low'].shift(1)
    df_4h['prev_low_2'] = df_4h['low'].shift(2)

    # 5. 保存 4H 信号文件
    df_4h.to_csv(output_4h_file)
    print(f"✅ 已保存 4H 信号数据: {output_4h_file}")
    
    return df_1min, df_4h

# 使用示例
process_1min_and_gen_4h('btc_perp_1m_dataset/BTCUSDT_perp_1m_master.csv', 'BTC_1min_Clean.csv', 'BTC_4H_Signals.csv')