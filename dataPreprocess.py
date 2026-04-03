import pandas as pd
import glob
import os
import pandas_ta as ta

# --- 配置 ---
INPUT_DIR = './dataset'  # 存放 15min CSV 的文件夹
OUTPUT_FILE = 'BTCUSDT_Combined_4H.csv'
OUTPUT_FILE_2 = 'BTCUSDT_Combined_15M.csv'


def synthesize_with_headers():
    all_files = glob.glob(os.path.join(INPUT_DIR, "*.csv"))
    all_files.sort()
    
    li = []
    print(f"检测到 {len(all_files)} 个带表头文件，开始处理...")

    for filename in all_files:
        df = pd.read_csv(filename)
        # 统一处理时间戳
        if 'open_time' in df.columns:
            df['timestamp'] = pd.to_datetime(df['open_time'], unit='ms')
        else:
            df['timestamp'] = pd.to_datetime(df['timestamp'])
            
        df = df[['timestamp', 'open', 'high', 'low', 'close', 'volume']]
        li.append(df)

    full_df = pd.concat(li, axis=0, ignore_index=True)
    full_df.drop_duplicates(subset=['timestamp'], inplace=True)
    full_df.sort_index(inplace=True)

    # 导出 15M 连续数据
    full_df.to_csv(OUTPUT_FILE_2, index=False)
    print(f"15M 基础数据已保存至: {OUTPUT_FILE_2}")

    # 合成 4H 数据
    full_df.set_index('timestamp', inplace=True)
    df_4h = full_df.resample('4h').agg({
        'open': 'first',
        'high': 'max',
        'low': 'min',
        'close': 'last',
        'volume': 'sum'
    }).dropna()

    # --- 计算策略所需的全套锚点 (全量 4H 指标) ---
    df_4h['ma20'] = ta.sma(df_4h['close'], length=20)
    df_4h['atr'] = ta.atr(df_4h['high'], df_4h['low'], df_4h['close'], length=14)
    
    # 核心形态参考位：
    # 我们需要 t-1 和 t-2 的数据，所以统一 shift(1)
    df_4h['prev_high_1'] = df_4h['high'].shift(1)  # High_{t-1}
    df_4h['prev_high_2'] = df_4h['high'].shift(2)  # High_{t-2}
    df_4h['prev_low_1'] = df_4h['low'].shift(1)    # Low_{t-1}
    df_4h['prev_low_2'] = df_4h['low'].shift(2)    # Low_{t-2}

    df_4h.to_csv(OUTPUT_FILE)
    print(f"4H 合成数据已保存 (含全量锚点)！周期: {df_4h.index[0]} 至 {df_4h.index[-1]}")
    return full_df, df_4h
# 执行合成
df_15m_raw, df_4h_final = synthesize_with_headers()

df_15m_raw.to_csv(OUTPUT_FILE_2)