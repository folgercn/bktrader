import pandas as pd
import numpy as np
import glob
import os
from tqdm import tqdm
import matplotlib.pyplot as plt


def run_tick_full_scan_dual(df_4h, tick_file,current_bal):
    # ... (加载数据部分保持不变) ...
    print(f"正在加载原始 Tick 数据: {tick_file} ...")
    df_tick = pd.read_csv(
        tick_file, engine='python', header=None, 
        usecols=[1, 4], names=['price', 'timestamp'],
        on_bad_lines='skip', dtype={'price': 'float32', 'timestamp': 'int64'}
    )
    df_tick['timestamp'] = pd.to_datetime(df_tick['timestamp'], unit='ms')
    df_tick.set_index('timestamp', inplace=True)
    
    # 策略常量
    COMMISSION = 0.0010
    STOP_LOSS_ATR_LONG = 0.1    # 多头止损
    STOP_LOSS_ATR_SHORT = 0.1  # 空头止损：较宽，防止在空头趋势中被反抽洗掉
    REENTRY_ATR = 0.1
    MAX_TRADES_PER_BAR = 2
    CASH_USAGE_RATE=0.2
    SLIPPAGE = 0.0005 

    balance = current_bal
    position = None 
    trade_logs = []
    last_exit_reason, last_exit_side = None, None
    last_exit_bar_index = -999
    REENTRY_TIMEOUT = 1

    # 4H 信号迭代
    for i in range(len(df_4h)-1):
        start_t, end_t = df_4h.index[i], df_4h.index[i+1]
        window_ticks = df_tick.loc[start_t:end_t]
        if window_ticks.empty: continue
        
        sig = df_4h.iloc[i]
        trades_in_bar = 0
        idx = 0
        total_ticks = len(window_ticks)

        if i - last_exit_bar_index > REENTRY_TIMEOUT:
            last_exit_side = None

        
        
        while idx < total_ticks:
            # 1. 获取当前 Tick 价格和时间
            current_tick = window_ticks.iloc[idx]
            current_p = current_tick['price']
            current_time = window_ticks.index[idx]
            
            if not position:
                # --- [A. 寻找进场/重入] ---
                executed = False
                
                # 多头逻辑
                if sig['close'] > sig['ma20']:
                    re_p = sig['prev_low_1'] + (REENTRY_ATR * sig['atr'])
                    # 初始进场
                    if trades_in_bar == 0 and sig['prev_high_2'] > sig['prev_high_1']:
                        if current_p >= sig['prev_high_2']:
                            entry_p = max(current_p, sig['prev_high_2'])
                            entry_p *= (1 + SLIPPAGE)
                            notional_value = balance * CASH_USAGE_RATE
                            position = {'side': 'long', 'entry_p': entry_p, 'sl': entry_p - (STOP_LOSS_ATR_LONG * sig['atr']), 'protected': False,'notional': notional_value}
                            balance -= notional_value * COMMISSION
                            trade_logs.append({'time': current_time, 'type': 'BUY', 'price': entry_p, 'reason': 'Initial', 'notional': notional_value,'bal': balance})
                            trades_in_bar += 1; executed = True
                    
                    # 重入逻辑 (增加状态清除)
                    elif last_exit_side == 'long':
                        if current_p >= re_p and (i - last_exit_bar_index <= REENTRY_TIMEOUT):
                            reason = 'SL-Reentry' if last_exit_reason == 'SL' else 'PT-Reentry'
                            if (reason == 'SL-Reentry' and trades_in_bar < MAX_TRADES_PER_BAR) or reason == 'PT-Reentry':
                                notional_value = balance * CASH_USAGE_RATE
                                entry_price = re_p * (1 + SLIPPAGE)
                                position = {'side': 'long', 'entry_p': entry_price, 'sl': sig['prev_low_1'], 'protected': (reason == 'PT'),'notional': notional_value}
                                balance -= notional_value * COMMISSION
                                trade_logs.append({'time': current_time, 'type': 'BUY', 'price': entry_price, 'reason': reason, 'bal': balance,'notional': notional_value})
                                if reason == 'SL-Reentry': trades_in_bar += 1
                                executed = True
                            # 无论是否重入成功，只要触碰了触发线或逻辑已过期，就清除状态，防止在同一位置反复刷单
                            last_exit_side = None 

                # 空头逻辑 (同样增加状态清除)
                elif sig['close'] < sig['ma20']:
                    re_p = sig['prev_high_1']
                    if trades_in_bar == 0 and sig['prev_low_2'] < sig['prev_low_1']:
                        if current_p <= sig['prev_low_2']:
                            entry_p = min(current_p, sig['prev_low_2'])
                            entry_p *= (1 - SLIPPAGE)
                            notional_value = balance * CASH_USAGE_RATE
                            position = {'side': 'short', 'entry_p': entry_p, 'sl': entry_p + (STOP_LOSS_ATR_SHORT * sig['atr']), 'protected': False,'notional': notional_value}
                            balance -= notional_value * COMMISSION
                            trade_logs.append({'time': current_time, 'type': 'SHORT', 'price': entry_p, 'reason': 'Initial','notional': notional_value, 'bal': balance})
                            trades_in_bar += 1; executed = True
                    elif last_exit_side == 'short':
                        if current_p <= re_p and (i - last_exit_bar_index <= REENTRY_TIMEOUT):
                            reason = 'SL-Reentry' if last_exit_reason == 'SL' else 'PT-Reentry'
                            if (reason == 'SL-Reentry' and trades_in_bar < MAX_TRADES_PER_BAR) or reason == 'PT-Reentry':
                                notional_value = balance * CASH_USAGE_RATE
                                entry_price=re_p*(1 - SLIPPAGE)
                                position = {'side': 'short', 'entry_p': entry_price, 'sl': sig['prev_high_1'], 'protected': (reason == 'PT'),'notional': notional_value}
                                balance -= notional_value * COMMISSION
                                trade_logs.append({'time': current_time, 'type': 'SHORT', 'price': entry_price, 'reason': reason,'notional': notional_value, 'bal': balance})
                                if reason == 'SL-Reentry': trades_in_bar += 1
                                executed = True
                            last_exit_side = None

                # 步进逻辑：如果成交了，跳过当前秒的所有 Tick；没成交则看下一个 Tick
                if executed:
                    idx = window_ticks.index.searchsorted(current_time, side='right')
                else:
                    idx += 1

            else:
                # --- [B. 监控持仓退出] ---
                exit_triggered = False
                if position['side'] == 'long':
                    # 激活保护
                    if not position['protected'] and current_p >= position['entry_p'] + sig['atr']:
                        position['protected'] = True
                    
                    # 检查止损或止盈
                    if current_p <= position['sl']:
                        exit_p, reason, exit_triggered = position['sl'], 'SL', True
                    elif position['protected'] and current_p <= sig['prev_low_1']:
                        exit_p, reason, exit_triggered = sig['prev_low_1'], 'PT', True
                
                else: # 空头持仓
                    if not position['protected'] and current_p <= position['entry_p'] - sig['atr']:
                        position['protected'] = True
                    if current_p >= position['sl']:
                        exit_p, reason, exit_triggered = position['sl'], 'SL', True
                    elif position['protected'] and current_p >= sig['prev_high_1']:
                        exit_p, reason, exit_triggered = sig['prev_high_1'], 'PT', True

                if exit_triggered:
                    side_mult = 1 if position['side'] == 'long' else -1
                    exit_p= exit_p * (1 - SLIPPAGE) if position['side'] == 'long' else exit_p* (1 + SLIPPAGE)
                    pnl = side_mult * (exit_p - position['entry_p']) / position['entry_p'] * position['notional']
                    balance += pnl - (position['notional'] * COMMISSION)
                    trade_logs.append({'time': current_time, 'type': 'EXIT', 'price': exit_p, 'reason': reason,'notional': position['notional'], 'bal': balance})
                    last_exit_reason, last_exit_side, position = reason, position['side'], None
                    idx = window_ticks.index.searchsorted(current_time, side='right')
                    last_exit_bar_index = i

                else:
                    idx += 1
    return pd.DataFrame(trade_logs), balance

def analyze_long_short_performance(trade_df):
    """
    分析多头和空头各自的绩效表现
    trade_df: 回测输出的包含 BUY, SHORT, EXIT 记录的 DataFrame
    """
    if trade_df.empty:
        print("没有交易记录可供统计。")
        return

    # 1. 提取完整的交易对 (Entry -> Exit)
    trades = []
    temp_pos = None
    
    for _, row in trade_df.iterrows():
        if row['type'] in ['BUY', 'SHORT']:
            temp_pos = row
        elif row['type'] == 'EXIT' and temp_pos is not None:
            # 计算这一笔的盈亏金额
            pnl_val = row['bal'] - temp_pos['bal']
            # 计算收益率 (基于当时的名义价值)
            pnl_pct = (row['price'] - temp_pos['price']) / temp_pos['price']
            if temp_pos['type'] == 'SHORT':
                pnl_pct = -pnl_pct
                
            trades.append({
                'side': 'Long' if temp_pos['type'] == 'BUY' else 'Short',
                'entry_time': temp_pos['time'],
                'exit_time': row['time'],
                'pnl_val': pnl_val,
                'pnl_pct': pnl_pct,
                'reason': row['reason'],
                'duration': (pd.to_datetime(row['time']) - pd.to_datetime(temp_pos['time'])).total_seconds() / 3600 # 小时
            })
            temp_pos = None

    analysis_df = pd.DataFrame(trades)

    # 2. 按多空分组计算统计指标
    summary = analysis_df.groupby('side').agg({
        'pnl_val': ['sum', 'count', 'mean'],
        'pnl_pct': ['mean', 'max', 'min'],
        'duration': 'mean'
    })
    
    # 3. 计算胜率
    win_rate = analysis_df.groupby('side')['pnl_val'].apply(lambda x: (x > 0).sum() / len(x))
    
    # 4. 计算盈亏比 (平均盈利 / 平均亏损)
    def get_rr_ratio(x):
        pos = x[x > 0].mean()
        neg = abs(x[x < 0].mean())
        return pos / neg if neg != 0 else np.nan
    
    rr_ratio = analysis_df.groupby('side')['pnl_val'].apply(get_rr_ratio)

    # --- 格式化打印 ---
    print("\n" + "="*50)
    print("📊 多/空方向绩效拆解报告")
    print("="*50)
    
    for side in ['Long', 'Short']:
        if side not in win_rate: continue
        
        s = analysis_df[analysis_df['side'] == side]
        print(f"\n【{side} 侧数据】:")
        print(f"  · 交易笔数: {len(s)}")
        print(f"  · 累计贡献利润: {s['pnl_val'].sum():.2f} USDT")
        print(f"  · 胜率: {win_rate[side]:.2%}")
        print(f"  · 盈亏比: {rr_ratio[side]:.2f}")
        print(f"  · 平均单笔收益率: {s['pnl_pct'].mean():.4%}")
        print(f"  · 平均持仓时间: {s['duration'].mean():.2f} 小时")
        print(f"  · 最大单笔盈利: {s['pnl_pct'].max():.2%}")
        print(f"  · 最大单笔亏损: {s['pnl_pct'].min():.2%}")
        
    print("\n" + "="*50)
    
    # 按平仓原因统计输出
    print("📂 平仓原因分布:")
    print(analysis_df.groupby(['side', 'reason']).size().unstack(fill_value=0))
    print("="*50)

def analyze_worst_month(trade_df):
    """
    挖掘最大单月亏损及其原因
    """
    if trade_df.empty: return
    
    # 1. 转换时间并设置索引
    df = trade_df.copy()
    df['time'] = pd.to_datetime(df['time'])
    df.set_index('time', inplace=True)
    
    # 2. 计算每月收益
    # 我们按月重采样，计算该月最后一笔余额与上月最后一笔余额的变动
    monthly_bal = df['bal'].resample('M').last().ffill()
    monthly_return = monthly_bal.pct_change()
    
    # 3. 找出最差月份
    worst_month = monthly_return.idxmin()
    worst_val = monthly_return.min()
    
    # 4. 统计该月的交易细节
    worst_trades = df.loc[str(worst_month.year) + "-" + str(worst_month.month)]
    sl_count = (worst_trades['reason'] == 'SL').sum()
    pt_count = (worst_trades['reason'] == 'PT').sum()
    
    print("\n" + "!"*40)
    print(f"💀 策略最大单月亏损深度挖掘")
    print("!"*40)
    print(f"  · 最差月份: {worst_month.strftime('%Y-%m')}")
    print(f"  · 该月收益率: {worst_val:.2%}")
    print(f"  · 该月交易笔数: {len(worst_trades)} 笔")
    print(f"  · 止损次数(SL): {sl_count} 次")
    print(f"  · 盈利保护次数(PT): {pt_count} 次")
    print(f"  · 止损/止盈比: {sl_count/pt_count:.2f}")
    
    # 5. 计算该月的最大回撤 (Drawdown inside month)
    worst_trades['cummax'] = worst_trades['bal'].cummax()
    worst_trades['dd'] = (worst_trades['bal'] / worst_trades['cummax'] - 1)
    intra_month_dd = worst_trades['dd'].min()
    print(f"  · 月内最大瞬时回撤: {intra_month_dd:.2%}")
    print("!"*40)

def generate_1d_signals(df_1min):
    print("正在聚合 1Min 数据生成 1D (日线) 信号数据...")
    df_1d = df_1min.resample('1D').agg({
        'open': 'first',
        'high': 'max',
        'low': 'min',
        'close': 'last',
        'volume': 'sum'
    })
    df_1d.dropna(subset=['close'], inplace=True)
    df_1d['ma5'] = df_1d['close'].rolling(window=5).mean()
    df_1d['ma20'] = df_1d['close'].rolling(window=20).mean()
    high_low = df_1d['high'] - df_1d['low']
    high_close = np.abs(df_1d['high'] - df_1d['close'].shift())
    low_close = np.abs(df_1d['low'] - df_1d['close'].shift())
    ranges = pd.concat([high_low, high_close, low_close], axis=1)
    true_range = np.max(ranges, axis=1)
    df_1d['atr'] = true_range.rolling(14).mean()
    df_1d['prev_high_1'] = df_1d['high'].shift(1)
    df_1d['prev_high_2'] = df_1d['high'].shift(2)
    df_1d['prev_low_1'] = df_1d['low'].shift(1)
    df_1d['prev_low_2'] = df_1d['low'].shift(2)
    print("1D 信号生成完成，总行数: ", len(df_1d))
    return df_1d

def _resolve_regime_ready(sig, timeframe):
    normalized = str(timeframe).lower().strip()
    if normalized == '1d':
        ma5 = sig.get('ma5', np.nan)
        atr = sig.get('atr', np.nan)
        if pd.isna(ma5) or ma5 <= 0 or pd.isna(atr) or atr <= 0:
            return False, False
        early_band = 0.06 * atr
        long_ready = sig['close'] > ma5 or (
            sig['close'] >= (ma5 - early_band)
            and sig['prev_high_2'] > sig['prev_high_1']
            and sig['prev_low_1'] >= sig['prev_low_2']
        )
        short_ready = sig['close'] < ma5 or (
            sig['close'] <= (ma5 + early_band)
            and sig['prev_low_2'] < sig['prev_low_1']
            and sig['prev_high_1'] <= sig['prev_high_2']
        )
        return long_ready, short_ready
    return sig['close'] > sig['ma20'], sig['close'] < sig['ma20']

def _normalize_reentry_sizes(reentry_size_schedule):
    if reentry_size_schedule is None:
        return [0.10, 0.20]
    if isinstance(reentry_size_schedule, (int, float)):
        return [float(reentry_size_schedule)]
    sizes = [float(x) for x in reentry_size_schedule]
    if not sizes:
        raise ValueError("reentry_size_schedule 不能为空")
    return sizes

def _get_reentry_size(trades_in_bar, reentry_size_schedule):
    if trades_in_bar <= 0:
        return 0.0
    schedule = _normalize_reentry_sizes(reentry_size_schedule)
    idx = max(0, trades_in_bar - 1)
    return schedule[min(idx, len(schedule) - 1)]

def _resolve_stop_price(side, entry_p, sig, stop_mode, stop_loss_atr):
    structural_stop = sig['prev_low_1'] if side == 'long' else sig['prev_high_1']
    atr_stop = entry_p - (stop_loss_atr * sig['atr']) if side == 'long' else entry_p + (stop_loss_atr * sig['atr'])

    if stop_mode == 'structural':
        return structural_stop
    if stop_mode == 'atr':
        return atr_stop
    if stop_mode == 'hybrid_tighter':
        return max(structural_stop, atr_stop) if side == 'long' else min(structural_stop, atr_stop)
    if stop_mode == 'hybrid_wider':
        return min(structural_stop, atr_stop) if side == 'long' else max(structural_stop, atr_stop)
    raise ValueError(f"未知止损模式: {stop_mode}")

def run_backtest_1min_granularity(df_1min, df_4h, initial_balance=100000.0,
                                  dir1_reentry_confirm=False,
                                  dir2_zero_initial=False,
                                  dir3_structural_sl=False,
                                  fixed_slippage=None,
                                  stop_loss_atr=0.2,
                                  max_trades_per_bar=3,
                                  reentry_size_schedule=None,
                                  stop_mode=None,
                                  profit_protect_atr=1.0):
    """
    1min 颗粒度双向回测引擎
    df_1min: 包含 open, high, low, close 的标准化 1min 数据
    df_4h: 包含策略锚点(ma20, atr, prev_high_2 等)的 4H 数据
    """
    balance = initial_balance
    position = None # {'side', 'entry_p', 'sl', 'protected', 'notional'}
    trade_logs = []
    # 策略参数
    REENTRY_ATR = 0.1
    COMMISSION = 0.0010 # 千分之一手续费
    MAX_TRADES_PER_BAR = max_trades_per_bar
    
    SLIPPAGE = np.random.uniform(0.0005, 0.002) if fixed_slippage is None else fixed_slippage
    
    # 仓位参数
    CASH_USAGE_INITIAL = 0.0 if dir2_zero_initial else 0.10
    REENTRY_SIZE_SCHEDULE = _normalize_reentry_sizes(reentry_size_schedule)

    PROFIT_PROTECT = profit_protect_atr
    stop_mode = 'structural' if dir3_structural_sl and stop_mode is None else (stop_mode or 'atr')

    last_exit_bar_index = -999
    REENTRY_TIMEOUT = 1  # 1个4H bar内有效
    
    last_exit_reason = None
    last_exit_side = None

    print(
        f"🚀 1min回测 | InitialSize:{CASH_USAGE_INITIAL:.0%} | "
        f"Stop:{stop_mode} | Reentry:{'Confirm' if dir1_reentry_confirm else 'Touch'} | "
        f"MaxTrades:{MAX_TRADES_PER_BAR} | ReentrySizes:{REENTRY_SIZE_SCHEDULE} | "
        f"Slippage:{SLIPPAGE}"
    )

    # 遍历 4H 信号窗口
    for i in range(len(df_4h)-1):
        start_t, end_t = df_4h.index[i], df_4h.index[i+1]
        
        # 提取当前 4H 周期内的 1min 价格序列
        # 注意：使用 slice 提高性能
        window = df_1min.loc[start_t:end_t]
        if window.empty: continue
        
        sig = df_4h.iloc[i]
        if pd.isna(sig['atr']): continue
        long_regime_ready, short_regime_ready = _resolve_regime_ready(sig, '1d' if 'ma5' in df_4h.columns else '4h')
        
        trades_in_bar = 0
        current_idx = 0
        total_steps = len(window)

        if i - last_exit_bar_index > REENTRY_TIMEOUT:
            last_exit_side = None

        while current_idx < total_steps:
            # 当前探测的 1min Bar
            bar = window.iloc[current_idx]
            bar_time = window.index[current_idx]
            prev_bar = window.iloc[current_idx - 1] if current_idx > 0 else None
            
            if not position:
                # --- [A. 寻找进场/重入] ---
                executed = False
                
                # 1. 多头逻辑 (MA20 之上)
                if long_regime_ready:
                    re_p = sig['prev_low_1'] + (REENTRY_ATR * sig['atr'])
                    # 初始进场
                    if trades_in_bar == 0 and sig['prev_high_2'] > sig['prev_high_1']:
                        if bar['high'] >= sig['prev_high_2']:
                            entry_p = max(bar['open'], sig['prev_high_2'])
                            entry_p *= (1 + SLIPPAGE)
                            notional_value = balance * CASH_USAGE_INITIAL
                            initial_sl = _resolve_stop_price('long', entry_p, sig, stop_mode, stop_loss_atr)
                            position = {
                                'side': 'long', 'entry_p': entry_p, 
                                'sl': initial_sl, 
                                'protected': False,
                                'notional': notional_value
                            }
                            balance -= notional_value * COMMISSION
                            trade_logs.append({'time': bar_time, 'type': 'BUY', 'price': entry_p, 'reason': 'Initial','notional': notional_value,'bal': balance})
                            trades_in_bar += 1; executed = True
                    
                    # 重入逻辑
                    elif last_exit_side == 'long' and (i - last_exit_bar_index <= REENTRY_TIMEOUT):
                        # 改变点1：结构确认控制
                        is_triggered = False
                        entry_p_raw = re_p # default
                        if dir1_reentry_confirm:
                            if prev_bar is not None and bar['close'] > re_p and prev_bar['close'] > re_p:
                                is_triggered = True
                                entry_p_raw = bar['close']
                        else:
                            if bar['high'] >= re_p:
                                is_triggered = True
                                entry_p_raw = re_p

                        if is_triggered:
                            reason = 'SL-Reentry' if last_exit_reason == 'SL' else 'PT-Reentry'
                            if (reason == 'SL-Reentry' and trades_in_bar < MAX_TRADES_PER_BAR) or reason == 'PT-Reentry':
                                current_reentry_size = _get_reentry_size(trades_in_bar, REENTRY_SIZE_SCHEDULE)
                                notional_value = balance * current_reentry_size
                                entry_price = entry_p_raw * (1 + SLIPPAGE)
                                reentry_sl = _resolve_stop_price('long', entry_price, sig, stop_mode, stop_loss_atr)
                                position = {
                                    'side': 'long', 'entry_p': entry_price, 
                                    'sl': reentry_sl, 
                                    'protected': (reason == 'PT'),
                                    'notional': notional_value
                                }
                                balance -= notional_value * COMMISSION
                                trade_logs.append({'time': bar_time, 'type': 'BUY', 'price': entry_price, 'reason': reason, 'notional': notional_value,'bal': balance})
                                if reason == 'SL-Reentry': trades_in_bar += 1
                                executed = True
                            last_exit_side = None

                # 2. 空头逻辑 (MA20 之下)
                elif short_regime_ready:
                    re_p = sig['prev_high_1']
                    # 初始进场
                    if trades_in_bar == 0 and sig['prev_low_2'] < sig['prev_low_1']:
                        if bar['low'] <= sig['prev_low_2']:
                            entry_p = min(bar['open'], sig['prev_low_2'])
                            entry_p *= (1 - SLIPPAGE)
                            notional_value = balance * CASH_USAGE_INITIAL
                            initial_sl = _resolve_stop_price('short', entry_p, sig, stop_mode, stop_loss_atr)
                            position = {
                                'side': 'short', 'entry_p': entry_p, 
                                'sl': initial_sl, 
                                'protected': False,
                                'notional': notional_value
                            }
                            # balance -= balance * COMMISSION
                            balance -= notional_value * COMMISSION
                            trade_logs.append({'time': bar_time, 'type': 'SHORT', 'price': entry_p, 'reason': 'Initial','notional': notional_value, 'bal': balance})
                            trades_in_bar += 1; executed = True
                    
                    # 空头重入
                    elif last_exit_side == 'short' and (i - last_exit_bar_index <= REENTRY_TIMEOUT):
                        is_triggered = False
                        entry_p_raw = re_p
                        if dir1_reentry_confirm:
                            if prev_bar is not None and bar['close'] < re_p and prev_bar['close'] < re_p:
                                is_triggered = True
                                entry_p_raw = bar['close']
                        else:
                            if bar['low'] <= re_p:
                                is_triggered = True
                                entry_p_raw = re_p

                        if is_triggered:
                            reason = 'SL-Reentry' if last_exit_reason == 'SL' else 'PT-Reentry'
                            if (reason == 'SL-Reentry' and trades_in_bar < MAX_TRADES_PER_BAR) or reason == 'PT-Reentry':
                                current_reentry_size = _get_reentry_size(trades_in_bar, REENTRY_SIZE_SCHEDULE)
                                notional_value = balance * current_reentry_size
                                entry_price = entry_p_raw * (1 - SLIPPAGE)
                                reentry_sl = _resolve_stop_price('short', entry_price, sig, stop_mode, stop_loss_atr)
                                position = {
                                    'side': 'short', 'entry_p': entry_price, 
                                    'sl': reentry_sl, 
                                    'protected': (reason == 'PT'),
                                    'notional': notional_value
                                }
                                balance -= notional_value * COMMISSION
                                trade_logs.append({'time': bar_time, 'type': 'SHORT', 'price': entry_price, 'reason': reason, 'notional': notional_value,'bal': balance})
                                if reason == 'SL-Reentry': trades_in_bar += 1
                                executed = True
                            last_exit_side = None
                
                if executed: current_idx += 1
                else: current_idx += 1 # 没触发则看下一分钟

            else:
                # --- [B. 寻找退出点] ---
                exit_triggered = False
                if position['side'] == 'long':
                    # 激活浮盈保护 (1.0 ATR)
                    if not position['protected'] and bar['high'] >= position['entry_p'] + PROFIT_PROTECT*sig['atr']:
                        position['protected'] = True
                    
                    # 检查止损或止盈 (1min Low 触碰)
                    if bar['low'] <= position['sl']:
                        exit_p, reason, exit_triggered = position['sl'], 'SL', True
                    elif position['protected'] and bar['low'] <= sig['prev_low_1']:
                        exit_p, reason, exit_triggered = sig['prev_low_1'], 'PT', True
                
                else: # 空头持仓
                    if not position['protected'] and bar['low'] <= position['entry_p'] - PROFIT_PROTECT*sig['atr']:
                        position['protected'] = True
                    
                    if bar['high'] >= position['sl']:
                        exit_p, reason, exit_triggered = position['sl'], 'SL', True
                    elif position['protected'] and bar['high'] >= sig['prev_high_1']:
                        exit_p, reason, exit_triggered = sig['prev_high_1'], 'PT', True

                if exit_triggered:
                    # 计算盈亏
                    side_mult = 1 if position['side'] == 'long' else -1
                    exit_p= exit_p * (1 - SLIPPAGE) if position['side'] == 'long' else exit_p* (1 + SLIPPAGE)
                    if position['notional'] > 0:
                        pnl = side_mult * (exit_p - position['entry_p']) / position['entry_p'] * position['notional']
                        balance += pnl - (position['notional'] * COMMISSION)
                    else:
                        pnl = 0
                    trade_logs.append({'time': bar_time, 'type': 'EXIT', 'price': exit_p, 'reason': reason,'notional': position['notional'], 'bal': balance})
                    last_exit_reason = reason
                    last_exit_side = position['side']
                    last_exit_bar_index = i
                    position = None
                    current_idx += 1
                else:
                    current_idx += 1

    return pd.DataFrame(trade_logs)

def find_the_glory_trade(trade_df):
    """
    挖掘 6 年回测中的单笔盈利冠军
    """
    if trade_df.empty: return
    
    # 1. 提取完整的交易对
    trades = []
    temp_pos = None
    for _, row in trade_df.iterrows():
        if row['type'] in ['BUY', 'SHORT']:
            temp_pos = row
        elif row['type'] == 'EXIT' and temp_pos is not None:
            pnl_val = row['bal'] - temp_pos['bal']
            pnl_pct = (row['price'] - temp_pos['price']) / temp_pos['price']
            if temp_pos['type'] == 'SHORT': pnl_pct = -pnl_pct
            
            trades.append({
                'side': 'Long' if temp_pos['type'] == 'BUY' else 'Short',
                'entry_time': temp_pos['time'],
                'exit_time': row['time'],
                'pnl_val': pnl_val,
                'pnl_pct': pnl_pct,
                'entry_p': temp_pos['price'],
                'exit_p': row['price']
            })
            temp_pos = None

    analysis_df = pd.DataFrame(trades)
    
    # 2. 找到百分比收益最大的那一笔
    best_trade = analysis_df.loc[analysis_df['pnl_pct'].idxmax()]
    
    print("\n" + "🏆"*20)
    print(f"🌟 6 年回测单笔盈利冠军 🌟")
    print("🏆"*20)
    print(f"  · 交易方向: {best_trade['side']}")
    print(f"  · 收益率: {best_trade['pnl_pct']:.2%}")
    print(f"  · 净利润: {best_trade['pnl_val']:.2f} USDT")
    print(f"  · 进场时间: {best_trade['entry_time']}")
    print(f"  · 平仓时间: {best_trade['exit_time']}")
    print(f"  · 价格路径: {best_trade['entry_p']:.2f} -> {best_trade['exit_p']:.2f}")
    print("🏆"*20)

def run_1d_pyramid_stop_matrix():
    SIGNALS_1Min_PATH = 'BTC_1min_Clean.csv'
    if not os.path.exists(SIGNALS_1Min_PATH):
        print("错误：找不到 1min 数据文件！")
        return pd.DataFrame()

    df_1min = pd.read_csv(SIGNALS_1Min_PATH, index_col=0, parse_dates=True)
    start_date = "2020-01-01"
    end_date = "2026-02-28"
    initial_balance = 100000.0

    df_1min_sliced = df_1min.loc[start_date:end_date]
    df_1d_sliced = generate_1d_signals(df_1min_sliced)

    scenarios = [
        ("Baseline", {"dir2_zero_initial": True, "fixed_slippage": 0.0005, "stop_loss_atr": 0.05, "stop_mode": "atr", "max_trades_per_bar": 3, "reentry_size_schedule": [0.10, 0.20]}),
        ("MTB4 SameSize", {"dir2_zero_initial": True, "fixed_slippage": 0.0005, "stop_loss_atr": 0.05, "stop_mode": "atr", "max_trades_per_bar": 4, "reentry_size_schedule": [0.10, 0.20, 0.20]}),
        ("MTB4 Pyramid Mild", {"dir2_zero_initial": True, "fixed_slippage": 0.0005, "stop_loss_atr": 0.05, "stop_mode": "atr", "max_trades_per_bar": 4, "reentry_size_schedule": [0.10, 0.15, 0.25]}),
        ("MTB4 Pyramid Aggressive", {"dir2_zero_initial": True, "fixed_slippage": 0.0005, "stop_loss_atr": 0.05, "stop_mode": "atr", "max_trades_per_bar": 4, "reentry_size_schedule": [0.10, 0.20, 0.30]}),
        ("ATR 0.10", {"dir2_zero_initial": True, "fixed_slippage": 0.0005, "stop_loss_atr": 0.10, "stop_mode": "atr", "max_trades_per_bar": 3, "reentry_size_schedule": [0.10, 0.20]}),
        ("Structural", {"dir2_zero_initial": True, "fixed_slippage": 0.0005, "stop_loss_atr": 0.05, "stop_mode": "structural", "max_trades_per_bar": 3, "reentry_size_schedule": [0.10, 0.20]}),
        ("Hybrid Tighter", {"dir2_zero_initial": True, "fixed_slippage": 0.0005, "stop_loss_atr": 0.05, "stop_mode": "hybrid_tighter", "max_trades_per_bar": 3, "reentry_size_schedule": [0.10, 0.20]}),
        ("Hybrid Wider", {"dir2_zero_initial": True, "fixed_slippage": 0.0005, "stop_loss_atr": 0.05, "stop_mode": "hybrid_wider", "max_trades_per_bar": 3, "reentry_size_schedule": [0.10, 0.20]}),
    ]

    results = []
    print("\n" + "="*60)
    print("🌟 1D + Zero Initial | 金字塔仓位/止损模式正式矩阵")
    print("="*60)

    for name, kwargs in scenarios:
        print(f"\n---> 开始执行场景: {name}")
        ledger = run_backtest_1min_granularity(df_1min_sliced, df_1d_sliced, initial_balance, **kwargs)
        if ledger.empty:
            final_bal = initial_balance
            total_return = 0.0
            max_drawdown = 0.0
            trades = 0
        else:
            final_bal = ledger.iloc[-1]['bal']
            total_return = (final_bal / initial_balance - 1)
            ledger['cum_max'] = ledger['bal'].cummax()
            ledger['drawdown'] = (ledger['bal'] / ledger['cum_max'] - 1)
            max_drawdown = ledger['drawdown'].min()
            trades = (ledger['type'] == 'EXIT').sum()
        print(f"[{name}] 收益率: {total_return:.2%}, 最大回撤: {max_drawdown:.2%}, 交易对数: {trades}")
        results.append((name, total_return, max_drawdown, trades))

    df_res = pd.DataFrame(results, columns=["Scenario", "Return", "Max DD", "Trade Pairs"])
    print("\n" + "="*60)
    print("🏆 正式矩阵汇总")
    print("="*60)
    print(df_res.to_string(index=False, formatters={'Return': '{:.2%}'.format, 'Max DD': '{:.2%}'.format}))
    print("="*60)
    return df_res

def run_4year_master_tick_backtest():
    # 1. 加载预先合成好的 4H 信号数据库
    SIGNALS_4H_PATH = 'BTCUSDT_4Year_4H_Full.csv'
    if not os.path.exists(SIGNALS_4H_PATH):
        print("错误：找不到 4H 信号文件，请先运行合成脚本！")
        return
    
    df_4h = pd.read_csv(SIGNALS_4H_PATH, index_col=0, parse_dates=True)
    
    # 2. 获取所有待处理的 Tick 文件
    TICK_DATA_DIR = '/Users/wuyaocheng/Downloads/bkTrader/dataset/archive/BTCUSDT-trades-202*'
    tick_files = sorted(glob.glob(os.path.join(TICK_DATA_DIR, "*.csv")))
    
    all_monthly_results = []
    global_balance = 100000.0

    
    print(f"🚀 开始 4 年全量回测，共 {len(tick_files)} 个月份...")

    # 3. 逐月迭代回测
    for file in tqdm(tick_files):
        month_name = os.path.basename(file)
        try:
            # 调用之前定义的双向回测引擎
            # 注意：引擎内部会根据 tick 文件的范围自动截取 df_4h
            monthly_trades, global_balance = run_tick_full_scan_dual(df_4h, file,global_balance)
            
            if not monthly_trades.empty:
                all_monthly_results.append(monthly_trades)
                print(f"✅ {month_name} 处理完成，交易笔数: {len(monthly_trades)}")
                print(f"月度结束余额: {global_balance:.2f}")

            else:
                print(f"⚠️ {month_name} 未产生有效交易。")
        except Exception as e:
            print(f"❌ 处理 {month_name} 时出错: {e}")

    # 4. 汇总所有交易流水
    if not all_monthly_results:
        print("回测结束：未产生任何交易。")
        return

    full_trade_ledger = pd.concat(all_monthly_results).sort_values('time')
    full_trade_ledger.to_csv('FINAL_4YEAR_LEDGER.csv', index=False)

    # 5. 计算 4 年最终绩效
    final_bal = full_trade_ledger.iloc[-1]['bal']
    total_return = (final_bal / 100000.0 - 1)
    
    # 计算最大回撤 (MDD)
    full_trade_ledger['cum_max'] = full_trade_ledger['bal'].cummax()
    full_trade_ledger['drawdown'] = (full_trade_ledger['bal'] / full_trade_ledger['cum_max'] - 1)
    max_drawdown = full_trade_ledger['drawdown'].min()

    print("\n" + "="*40)
    print("🏆 4年全量 Tick 回测最终绩效报告")
    print("="*40)
    print(f"最终净值: {final_bal:.2f} USDT")
    print(f"总收益率: {total_return:.2%}")
    print(f"最大回撤: {max_drawdown:.2%}")
    print(f"总交易笔数: {len(full_trade_ledger)}")
    print(f"数据覆盖: {full_trade_ledger.iloc[0]['time']} 至 {full_trade_ledger.iloc[-1]['time']}")
    print("="*40)


def run_evaluation_matrix():
    # 1. 加载预先合成好的 4H 信号数据库
    SIGNALS_4H_PATH = 'BTC_4H_Signals.csv'
    SIGNALS_1Min_PATH = 'BTC_1min_Clean.csv'

    if not os.path.exists(SIGNALS_4H_PATH):
        print("错误：找不到 4H 信号文件，请先运行合成脚本！")
        return
    
    df_4h = pd.read_csv(SIGNALS_4H_PATH, index_col=0, parse_dates=True)
    df_1min = pd.read_csv(SIGNALS_1Min_PATH, index_col=0, parse_dates=True)
    global_balance = 100000.0

    start_date = "2020-01-01"
    end_date = "2026-02-28"

    # 对数据进行切片
    df_1min_sliced = df_1min.loc[start_date : end_date]
    df_4h_sliced = df_4h.loc[start_date : end_date]

    print("\n" + "="*50)
    print("🌟 启动独立变异交叉评估矩阵 (Evaluation Matrix)")
    print("="*50)

    results = []

    def run_scenario(name, kwargs):
        print(f"\n---> 开始执行场景: {name}")
        ledger = run_backtest_1min_granularity(df_1min_sliced, df_4h_sliced, global_balance, **kwargs)
        if ledger.empty:
            final_bal = global_balance
            total_return = 0.0
            max_drawdown = 0.0
        else:
            final_bal = ledger.iloc[-1]['bal']
            total_return = (final_bal / global_balance - 1)
            ledger['cum_max'] = ledger['bal'].cummax()
            ledger['drawdown'] = (ledger['bal'] / ledger['cum_max'] - 1)
            max_drawdown = ledger['drawdown'].min()
        print(f"[{name}] 收益率: {total_return:.2%}, 最大回撤: {max_drawdown:.2%}")
        results.append((name, total_return, max_drawdown))
        return ledger
    
    # 运行对照组
    run_scenario("1. Baseline (固定滑点0.0005)", {"fixed_slippage": 0.0005})
    run_scenario("2. Dir 1 Only (Reentry结构确认)", {"fixed_slippage": 0.0005, "dir1_reentry_confirm": True})
    run_scenario("3. Dir 2 Only (Initial无仓位)", {"fixed_slippage": 0.0005, "dir2_zero_initial": True})
    run_scenario("4. Dir 3 Only (纯结构止损)", {"fixed_slippage": 0.0005, "dir3_structural_sl": True})

    # Dir 4 跑滑点衰减
    print("\n---> 开始执行场景: 5. 矩阵滑点衰减曲线测试")
    slippages = [0.0005, 0.001, 0.002, 0.003]
    return_rates = []
    
    for slip in slippages:
        ledger = run_backtest_1min_granularity(df_1min_sliced, df_4h_sliced, global_balance, fixed_slippage=slip)
        if ledger.empty:
            final_bal = global_balance
            total_return = 0.0
        else:
            final_bal = ledger.iloc[-1]['bal']
            total_return = (final_bal / global_balance - 1)
        print(f"  · [Baseline] + 滑点 [{slip}] 收益率: {total_return:.2%}")
        return_rates.append(total_return)

    # 画图
    plt.figure(figsize=(10, 6))
    plt.title("Baseline Slippage Robustness Curve")
    plt.plot(slippages, return_rates, marker='o', linestyle='-', linewidth=2, color='r')
    plt.xlabel("Slippage")
    plt.ylabel("Total Return Rate")
    plt.grid(True)
    for i, txt in enumerate(return_rates):
        plt.annotate(f"{txt:.2%}", (slippages[i], return_rates[i]), textcoords="offset points", xytext=(0,10), ha='center')
    plt.savefig('slippage_robustness_curve.png', dpi=300)
    print("✅ 滑点鲁棒性曲线图已生成: slippage_robustness_curve.png")

    print("\n" + "="*50)
    print("🏆 评估矩阵汇总")
    print("="*50)
    df_res = pd.DataFrame(results, columns=["Scenario", "Return", "Max DD"])
    print(df_res.to_string(index=False, formatters={'Return': '{:.2%}'.format, 'Max DD': '{:.2%}'.format}))
    print("="*50)

if __name__ == "__main__":
    def run_1d_stoploss_matrix():
        SIGNALS_1Min_PATH = 'BTC_1min_Clean.csv'
        if not os.path.exists(SIGNALS_1Min_PATH):
            print("错误：找不到 1min 数据文件！")
            return
            
        df_1min = pd.read_csv(SIGNALS_1Min_PATH, index_col=0, parse_dates=True)
        global_balance = 100000.0
        start_date = "2020-01-01"
        end_date = "2026-02-28"

        # 切片
        df_1min_sliced = df_1min.loc[start_date : end_date]
        
        # 构建 1D 信号
        df_1d_sliced = generate_1d_signals(df_1min_sliced)

        print("\n" + "="*50)
        print("🌟 启动基于方向 2 (Initial=0%) 的 1D 周期多维止损测试")
        print("="*50)
        
        # 测试配置: 因为是 1D, atr 非常大, 所以通过参数控制测试: 0.05 到 0.5 之间
        test_atrs = [0.05, 0.1, 0.2, 0.3, 0.4, 0.5]
        return_rates = []
        
        results = []
        best_ledger = pd.DataFrame()
        best_return = -999
        
        for atr in test_atrs:
            print(f"\n---> 开始执行场景: [1D 周期] ATR = {atr}")
            # 固定开启 dir2_zero_initial = True, 其他为 False
            ledger = run_backtest_1min_granularity(
                df_1min_sliced, df_1d_sliced, global_balance, 
                dir1_reentry_confirm=False, 
                dir2_zero_initial=True, 
                dir3_structural_sl=False, 
                fixed_slippage=0.0005,
                stop_loss_atr=atr
            )
            
            if ledger.empty:
                final_bal = global_balance
                total_return = 0.0
                max_drawdown = 0.0
            else:
                final_bal = ledger.iloc[-1]['bal']
                total_return = (final_bal / global_balance - 1)
                ledger['cum_max'] = ledger['bal'].cummax()
                ledger['drawdown'] = (ledger['bal'] / ledger['cum_max'] - 1)
                max_drawdown = ledger['drawdown'].min()
                
            print(f"[ATR={atr}] 收益率: {total_return:.2%}, 最大回撤: {max_drawdown:.2%}")
            results.append((atr, total_return, max_drawdown))
            return_rates.append(total_return)
            
            if total_return > best_return and not ledger.empty:
                best_return = total_return
                best_ledger = ledger

        if not best_ledger.empty:
             best_ledger.to_csv('FINAL_1D_LEDGER_BEST_SL.csv', index=False)

        plt.figure(figsize=(10, 6))
        plt.title("1D Timeframe Stop-Loss ATR Robustness (Initial=0%)")
        plt.plot(test_atrs, return_rates, marker='o', linestyle='-', linewidth=2, color='green')
        plt.xlabel("Stop Loss ATR Multiplier")
        plt.ylabel("Total Return Rate")
        plt.grid(True)
        for i, txt in enumerate(return_rates):
            plt.annotate(f"{txt:.2%}", (test_atrs[i], return_rates[i]), textcoords="offset points", xytext=(0,10), ha='center')
        plt.savefig('sl_robustness_1D.png', dpi=300)
        print("✅ 1D 止损衰减曲线图已生成: sl_robustness_1D.png")

        print("\n" + "="*50)
        print("🏆 1D 止损评估矩阵汇总")
        print("="*50)
        df_res = pd.DataFrame(results, columns=["Stop Loss ATR", "Return", "Max DD"])
        print(df_res.to_string(index=False, formatters={'Return': '{:.2%}'.format, 'Max DD': '{:.2%}'.format}))
        print("="*50)

    run_1d_stoploss_matrix()
