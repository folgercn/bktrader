import pandas as pd
import pandas_ta as ta
import glob
import os

# --- 配置 ---
TICK_DATA_DIR = '/Users/wuyaocheng/Downloads/archive/*' 
FINAL_4H_PATH = 'BTCUSDT_4Year_4H_Full.csv'

def build_4year_4h_database():
    all_tick_files = sorted(glob.glob(os.path.join(TICK_DATA_DIR, "*.csv")))
    if not all_tick_files:
        print("错误：未在文件夹中找到 CSV 文件！")
        return

    monthly_1m_list = []
    print(f"开始处理 {len(all_tick_files)} 个文件...")

    for file_path in all_tick_files:
        print(f"正在处理: {os.path.basename(file_path)}")
        
        # 核心修正 1：使用 sep='\s+' 匹配任何空格/制表符组合
        # 核心修正 2：增加 error_bad_lines 跳过可能损坏的行
        try:
            chunks = pd.read_csv(
                file_path, 
              # 使用正则表达式匹配一个或多个空格/制表符
                engine='python', 
                header=None, 
                usecols=[1, 4],     # 价格在索引1，时间戳在索引4
                names=['price', 'timestamp'],
                on_bad_lines='skip', # 跳过格式不正确的行
                chunksize=2000000 
            )
            
            for chunk in chunks:
                # 核心修正 3：强制转换时间戳为数字，出错则转为 NaN 并删除
                chunk['timestamp'] = pd.to_numeric(chunk['timestamp'], errors='coerce')
                chunk = chunk.dropna(subset=['timestamp'])
                
                # 核心修正 4：明确指定 unit='ms' 且确保是 int64
                chunk['ts'] = pd.to_datetime(chunk['timestamp'].astype('int64'), unit='ms')
                chunk.set_index('ts', inplace=True)
                
                # 缩减为 1min
                m1 = chunk['price'].resample('1Min').agg(['first', 'max', 'min', 'last'])
                monthly_1m_list.append(m1)
        except Exception as e:
            print(f"读取文件 {file_path} 出错: {e}")

    if not monthly_1m_list:
        print("未能提取到有效数据，请检查 CSV 内容！")
        return

    print("正在合并全量 1m 数据...")
    full_1m = pd.concat(monthly_1m_list)
    full_1m.sort_index(inplace=True)
    full_1m = full_1m[~full_1m.index.duplicated(keep='first')]

    print(f"合并后 1m 数据共 {len(full_1m)} 行，正在合成 4H...")
    
    # 核心修正 5：合成 4H，确保聚合逻辑正确
    df_4h = full_1m.resample('4h').agg({
        'first': 'first', 
        'max': 'max', 
        'min': 'min', 
        'last': 'last'
    }).dropna()
    
    df_4h.columns = ['open', 'high', 'low', 'close']

    print("正在计算全局指标...")
    df_4h['ma20'] = ta.sma(df_4h['close'], length=20)
    df_4h['atr'] = ta.atr(df_4h['high'], df_4h['low'], df_4h['close'], length=14)
    
    df_4h['prev_high_1'] = df_4h['high'].shift(1)
    df_4h['prev_high_2'] = df_4h['high'].shift(2)
    df_4h['prev_low_1'] = df_4h['low'].shift(1)
    df_4h['prev_low_2'] = df_4h['low'].shift(2)

    df_4h.to_csv(FINAL_4H_PATH)
    print(f"\n✅ 合成成功！总行数: {len(df_4h)}")
    print(f"数据范围: {df_4h.index[0]} -- {df_4h.index[-1]}")
    print("\n最后 5 行数据预览：")
    print(df_4h.tail())

if __name__ == "__main__":
    build_4year_4h_database()