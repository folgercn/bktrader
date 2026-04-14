import pandas as pd
import numpy as np
import glob
import os
from tqdm import tqdm
import matplotlib.pyplot as plt


def _normalize_utc_index(df):
    normalized = df.copy(deep=False)
    normalized.index = pd.to_datetime(normalized.index, utc=True)
    return normalized


def _empty_tick_event_stream():
    return pd.DataFrame(
        {
            'price': pd.Series(dtype='float64'),
            'event': pd.Series(dtype='object'),
            'minute_ms': pd.Series(dtype='int64'),
        },
        index=pd.DatetimeIndex([], tz='UTC', name='timestamp'),
    )


def _summarize_complete_tick_chunk(df_ticks):
    if df_ticks.empty:
        return pd.DataFrame(columns=[
            'minute_ms', 'open', 'high', 'low', 'close',
            'first_ts', 'last_ts', 'high_ts', 'low_ts'
        ])

    grouped = df_ticks.groupby('minute_ms', sort=False)
    summary = grouped.agg(
        open=('price', 'first'),
        high=('price', 'max'),
        low=('price', 'min'),
        close=('price', 'last'),
        first_ts=('timestamp', 'first'),
        last_ts=('timestamp', 'last'),
    )

    high_idx = grouped['price'].idxmax()
    low_idx = grouped['price'].idxmin()

    high_ts = df_ticks.loc[high_idx, ['minute_ms', 'timestamp']].set_index('minute_ms')['timestamp']
    low_ts = df_ticks.loc[low_idx, ['minute_ms', 'timestamp']].set_index('minute_ms')['timestamp']

    summary['high_ts'] = high_ts
    summary['low_ts'] = low_ts
    summary = summary.reset_index()
    return summary


def _build_tick_event_stream(tick_file, start_ts=None, end_ts=None, chunksize=2_000_000):
    start_ms = int(pd.Timestamp(start_ts).timestamp() * 1000) if start_ts is not None else None
    end_ms = int(pd.Timestamp(end_ts).timestamp() * 1000) if end_ts is not None else None

    summaries = []
    pending = None
    with open(tick_file, 'r', encoding='utf-8') as fh:
        first_line = fh.readline().strip().lower()

    if first_line.startswith('id,price,qty,quote_qty,time'):
        reader = pd.read_csv(
            tick_file,
            header=0,
            usecols=['price', 'time'],
            dtype={'price': 'float32', 'time': 'int64'},
            chunksize=chunksize,
        )
    else:
        reader = pd.read_csv(
            tick_file,
            header=None,
            usecols=[1, 4],
            names=['price', 'timestamp'],
            dtype={'price': 'float32', 'timestamp': 'int64'},
            chunksize=chunksize,
        )

    for chunk in reader:
        if 'time' in chunk.columns:
            chunk = chunk.rename(columns={'time': 'timestamp'})
        if end_ms is not None and not chunk.empty and chunk['timestamp'].iloc[0] > end_ms:
            break
        if start_ms is not None and not chunk.empty and chunk['timestamp'].iloc[-1] < start_ms:
            continue

        reached_end = False
        if end_ms is not None and not chunk.empty and chunk['timestamp'].iloc[-1] > end_ms:
            chunk = chunk[chunk['timestamp'] <= end_ms]
            reached_end = True
        if start_ms is not None:
            chunk = chunk[chunk['timestamp'] >= start_ms]
        if chunk.empty:
            if reached_end:
                break
            continue

        if pending is not None and not pending.empty:
            chunk = pd.concat([pending, chunk], ignore_index=True)
            pending = None

        chunk['minute_ms'] = (chunk['timestamp'] // 60000) * 60000
        last_minute = chunk['minute_ms'].iloc[-1]
        pending = chunk[chunk['minute_ms'] == last_minute].copy()
        complete = chunk[chunk['minute_ms'] != last_minute]

        if not complete.empty:
            summaries.append(_summarize_complete_tick_chunk(complete))

        if reached_end:
            break

    if pending is not None and not pending.empty:
        summaries.append(_summarize_complete_tick_chunk(pending))

    if not summaries:
        return _empty_tick_event_stream()

    minute_df = pd.concat(summaries, ignore_index=True)
    events = []
    for row in minute_df.itertuples(index=False):
        if row.high_ts <= row.low_ts:
            ordered = [
                ('open', row.first_ts, float(row.open)),
                ('high', row.high_ts, float(row.high)),
                ('low', row.low_ts, float(row.low)),
                ('close', row.last_ts, float(row.close)),
            ]
        else:
            ordered = [
                ('open', row.first_ts, float(row.open)),
                ('low', row.low_ts, float(row.low)),
                ('high', row.high_ts, float(row.high)),
                ('close', row.last_ts, float(row.close)),
            ]

        deduped = []
        for label, ts_ms, price in ordered:
            if deduped and deduped[-1][0] == ts_ms and deduped[-1][1] == price:
                continue
            deduped.append((ts_ms, price, label))

        for seq, (ts_ms, price, label) in enumerate(deduped):
            events.append((ts_ms, seq, price, label, row.minute_ms))

    event_df = pd.DataFrame(events, columns=['timestamp_ms', 'seq', 'price', 'event', 'minute_ms'])
    event_df.sort_values(['timestamp_ms', 'seq'], inplace=True)
    event_df['timestamp'] = pd.to_datetime(event_df['timestamp_ms'], unit='ms', utc=True)
    event_df.set_index('timestamp', inplace=True)
    return event_df[['price', 'event', 'minute_ms']]


def run_tick_full_scan_dual(df_4h, tick_file, current_bal,
                            dir1_reentry_confirm=False,
                            dir2_zero_initial=False,
                            dir3_structural_sl=False,
                            fixed_slippage=None,
                            stop_loss_atr=0.05,
                            max_trades_per_bar=4,
                            reentry_size_schedule=None,
                            long_reentry_atr=0.1,
                            short_reentry_atr=0.0,
                            stop_mode=None,
                            profit_protect_atr=1.0,
                            trailing_stop_atr=0.3,
                            delayed_trailing_activation=0.5,
                            tiered_protection=False,
                            max_drawdown_pct=None,
                            cooldown_bars=6,
                            reentry_mode='default'):
    # Keep both signal windows and tick windows on the same UTC-aware contract.
    df_4h = _normalize_utc_index(df_4h)

    replay_start = df_4h.index[0]
    replay_end = df_4h.index[-1]
    print(f"正在构建 Tick 事件流: {tick_file} ({replay_start} ~ {replay_end}) ...")
    df_tick = _build_tick_event_stream(
        tick_file,
        start_ts=replay_start,
        end_ts=replay_end,
    )
    df_tick = _normalize_utc_index(df_tick)

    balance = current_bal
    peak_balance = current_bal
    position = None
    trade_logs = []

    COMMISSION = 0.0010
    MAX_TRADES_PER_BAR = max_trades_per_bar
    SLIPPAGE = np.random.uniform(0.0005, 0.002) if fixed_slippage is None else fixed_slippage
    CASH_USAGE_INITIAL = 0.0 if dir2_zero_initial else 0.10
    REENTRY_SIZE_SCHEDULE = _normalize_reentry_sizes(reentry_size_schedule)
    PROFIT_PROTECT = profit_protect_atr
    stop_mode = 'structural' if dir3_structural_sl and stop_mode is None else (stop_mode or 'atr')

    if reentry_mode == 'fixed':
        base = REENTRY_SIZE_SCHEDULE[0] if REENTRY_SIZE_SCHEDULE else 0.10
        REENTRY_SIZE_SCHEDULE = [base] * max(len(REENTRY_SIZE_SCHEDULE), 3)
    elif reentry_mode == 'decreasing':
        base = REENTRY_SIZE_SCHEDULE[0] if REENTRY_SIZE_SCHEDULE else 0.10
        REENTRY_SIZE_SCHEDULE = [base * (0.5 ** i) for i in range(max(len(REENTRY_SIZE_SCHEDULE), 3))]

    last_exit_bar_index = -999
    REENTRY_TIMEOUT = 1
    last_exit_reason = None
    last_exit_side = None
    circuit_breaker_until = -999

    label_parts = []
    if trailing_stop_atr:
        label_parts.append(f"Trail:{trailing_stop_atr}")
    if tiered_protection:
        label_parts.append("TieredPT")
    if max_drawdown_pct:
        label_parts.append(f"CB:{max_drawdown_pct:.0%}")
    if reentry_mode != 'default':
        label_parts.append(f"Re:{reentry_mode}")
    label_parts.append(f"ReATR:{long_reentry_atr}/{short_reentry_atr}")
    label = " | ".join(label_parts) if label_parts else "TickEnhanced"

    print(
        f"🎯 Tick回放 [{label}] | Stop:{stop_mode} ATR:{stop_loss_atr} | "
        f"Slippage:{SLIPPAGE}"
    )

    for i in range(len(df_4h) - 1):
        start_t, end_t = df_4h.index[i], df_4h.index[i + 1]
        window_ticks = df_tick.loc[start_t:end_t]
        if window_ticks.empty:
            continue

        sig = df_4h.iloc[i]
        if pd.isna(sig['atr']):
            continue
        long_regime_ready, short_regime_ready = _resolve_regime_ready(
            sig, '1d' if 'ma5' in df_4h.columns else '4h'
        )

        trades_in_bar = 0
        idx = 0
        total_ticks = len(window_ticks)

        if i - last_exit_bar_index > REENTRY_TIMEOUT:
            last_exit_side = None

        while idx < total_ticks:
            current_tick = window_ticks.iloc[idx]
            current_p = float(current_tick['price'])
            current_time = window_ticks.index[idx]
            prev_p = float(window_ticks.iloc[idx - 1]['price']) if idx > 0 else np.nan

            if not position:
                if max_drawdown_pct is not None and i < circuit_breaker_until:
                    idx += 1
                    continue

                executed = False

                if long_regime_ready:
                    re_p = sig['prev_low_1'] + (long_reentry_atr * sig['atr'])
                    if trades_in_bar == 0 and sig['prev_high_2'] > sig['prev_high_1']:
                        if current_p >= sig['prev_high_2']:
                            entry_raw = max(current_p, sig['prev_high_2'])
                            entry_p = entry_raw * (1 + SLIPPAGE)
                            notional_value = balance * CASH_USAGE_INITIAL
                            initial_sl = _resolve_stop_price('long', entry_p, sig, stop_mode, stop_loss_atr)
                            position = {
                                'side': 'long',
                                'entry_p': entry_p,
                                'sl': initial_sl,
                                'protected': False,
                                'notional': notional_value,
                                'hwm': entry_p,
                            }
                            balance -= notional_value * COMMISSION
                            trade_logs.append({
                                'time': current_time,
                                'type': 'BUY',
                                'price': entry_p,
                                'reason': 'Initial',
                                'notional': notional_value,
                                'bal': balance,
                            })
                            trades_in_bar += 1
                            executed = True
                    elif last_exit_side == 'long' and (i - last_exit_bar_index <= REENTRY_TIMEOUT):
                        is_triggered = False
                        entry_p_raw = re_p
                        if dir1_reentry_confirm:
                            if not pd.isna(prev_p) and current_p > re_p and prev_p > re_p:
                                is_triggered = True
                                entry_p_raw = current_p
                        else:
                            if current_p >= re_p:
                                is_triggered = True
                        if is_triggered:
                            reason = 'SL-Reentry' if last_exit_reason == 'SL' else 'PT-Reentry'
                            if trades_in_bar < MAX_TRADES_PER_BAR:
                                current_reentry_size = _get_reentry_size(trades_in_bar, REENTRY_SIZE_SCHEDULE)
                                notional_value = balance * current_reentry_size
                                entry_price = entry_p_raw * (1 + SLIPPAGE)
                                reentry_sl = _resolve_stop_price('long', entry_price, sig, stop_mode, stop_loss_atr)
                                position = {
                                    'side': 'long',
                                    'entry_p': entry_price,
                                    'sl': reentry_sl,
                                    'protected': (reason == 'PT-Reentry'),
                                    'notional': notional_value,
                                    'hwm': entry_price,
                                }
                                balance -= notional_value * COMMISSION
                                trade_logs.append({
                                    'time': current_time,
                                    'type': 'BUY',
                                    'price': entry_price,
                                    'reason': reason,
                                    'notional': notional_value,
                                    'bal': balance,
                                })
                                trades_in_bar += 1
                                executed = True
                            last_exit_side = None

                elif short_regime_ready:
                    re_p = sig['prev_high_1'] - (short_reentry_atr * sig['atr'])
                    if trades_in_bar == 0 and sig['prev_low_2'] < sig['prev_low_1']:
                        if current_p <= sig['prev_low_2']:
                            entry_raw = min(current_p, sig['prev_low_2'])
                            entry_p = entry_raw * (1 - SLIPPAGE)
                            notional_value = balance * CASH_USAGE_INITIAL
                            initial_sl = _resolve_stop_price('short', entry_p, sig, stop_mode, stop_loss_atr)
                            position = {
                                'side': 'short',
                                'entry_p': entry_p,
                                'sl': initial_sl,
                                'protected': False,
                                'notional': notional_value,
                                'lwm': entry_p,
                            }
                            balance -= notional_value * COMMISSION
                            trade_logs.append({
                                'time': current_time,
                                'type': 'SHORT',
                                'price': entry_p,
                                'reason': 'Initial',
                                'notional': notional_value,
                                'bal': balance,
                            })
                            trades_in_bar += 1
                            executed = True
                    elif last_exit_side == 'short' and (i - last_exit_bar_index <= REENTRY_TIMEOUT):
                        is_triggered = False
                        entry_p_raw = re_p
                        if dir1_reentry_confirm:
                            if not pd.isna(prev_p) and current_p < re_p and prev_p < re_p:
                                is_triggered = True
                                entry_p_raw = current_p
                        else:
                            if current_p <= re_p:
                                is_triggered = True
                        if is_triggered:
                            reason = 'SL-Reentry' if last_exit_reason == 'SL' else 'PT-Reentry'
                            if trades_in_bar < MAX_TRADES_PER_BAR:
                                current_reentry_size = _get_reentry_size(trades_in_bar, REENTRY_SIZE_SCHEDULE)
                                notional_value = balance * current_reentry_size
                                entry_price = entry_p_raw * (1 - SLIPPAGE)
                                reentry_sl = _resolve_stop_price('short', entry_price, sig, stop_mode, stop_loss_atr)
                                position = {
                                    'side': 'short',
                                    'entry_p': entry_price,
                                    'sl': reentry_sl,
                                    'protected': (reason == 'PT-Reentry'),
                                    'notional': notional_value,
                                    'lwm': entry_price,
                                }
                                balance -= notional_value * COMMISSION
                                trade_logs.append({
                                    'time': current_time,
                                    'type': 'SHORT',
                                    'price': entry_price,
                                    'reason': reason,
                                    'notional': notional_value,
                                    'bal': balance,
                                })
                                trades_in_bar += 1
                                executed = True
                            last_exit_side = None

                if executed:
                    idx = window_ticks.index.searchsorted(current_time, side='right')
                else:
                    idx += 1
            else:
                exit_triggered = False
                exit_p = 0.0
                reason = ''

                if position['side'] == 'long':
                    prev_hwm = position.get('hwm', position['entry_p'])
                    protected_before_tick = position.get('protected', False)

                    if trailing_stop_atr is not None:
                        is_active = True
                        if delayed_trailing_activation is not None:
                            profit_atr = (prev_hwm - position['entry_p']) / sig['atr'] if sig['atr'] > 0 else 0
                            if profit_atr < delayed_trailing_activation:
                                is_active = False
                        if is_active:
                            trailing_sl = prev_hwm - trailing_stop_atr * sig['atr']
                            position['sl'] = max(position['sl'], trailing_sl)

                    if current_p <= position['sl']:
                        exit_p, reason, exit_triggered = position['sl'], 'SL', True
                    elif tiered_protection:
                        profit_atr = (prev_hwm - position['entry_p']) / sig['atr'] if sig['atr'] > 0 else 0
                        if protected_before_tick and profit_atr >= 3.0:
                            tiered_exit = max(sig['prev_low_1'], prev_hwm - 1.5 * sig['atr'])
                            if current_p <= tiered_exit:
                                exit_p, reason, exit_triggered = tiered_exit, 'PT-T3', True
                        elif protected_before_tick and profit_atr >= 2.0:
                            tiered_exit = max(sig['prev_low_1'], position['entry_p'] + 0.5 * sig['atr'])
                            if current_p <= tiered_exit:
                                exit_p, reason, exit_triggered = tiered_exit, 'PT-T2', True
                        elif protected_before_tick and current_p <= sig['prev_low_1']:
                            exit_p, reason, exit_triggered = sig['prev_low_1'], 'PT', True
                    else:
                        if protected_before_tick and current_p <= sig['prev_low_1']:
                            exit_p, reason, exit_triggered = sig['prev_low_1'], 'PT', True

                    if not exit_triggered:
                        position['hwm'] = max(prev_hwm, current_p)
                        if tiered_protection:
                            profit_atr = (position['hwm'] - position['entry_p']) / sig['atr'] if sig['atr'] > 0 else 0
                            if profit_atr >= PROFIT_PROTECT and not position['protected']:
                                position['protected'] = True
                        else:
                            if not position['protected'] and current_p >= position['entry_p'] + PROFIT_PROTECT * sig['atr']:
                                position['protected'] = True
                        if trailing_stop_atr is not None:
                            is_active = True
                            if delayed_trailing_activation is not None:
                                profit_atr = (position['hwm'] - position['entry_p']) / sig['atr'] if sig['atr'] > 0 else 0
                                if profit_atr < delayed_trailing_activation:
                                    is_active = False
                            if is_active:
                                trailing_sl = position['hwm'] - trailing_stop_atr * sig['atr']
                                position['sl'] = max(position['sl'], trailing_sl)

                else:
                    prev_lwm = position.get('lwm', position['entry_p'])
                    protected_before_tick = position.get('protected', False)

                    if trailing_stop_atr is not None:
                        is_active = True
                        if delayed_trailing_activation is not None:
                            profit_atr = (position['entry_p'] - prev_lwm) / sig['atr'] if sig['atr'] > 0 else 0
                            if profit_atr < delayed_trailing_activation:
                                is_active = False
                        if is_active:
                            trailing_sl = prev_lwm + trailing_stop_atr * sig['atr']
                            position['sl'] = min(position['sl'], trailing_sl)

                    if current_p >= position['sl']:
                        exit_p, reason, exit_triggered = position['sl'], 'SL', True
                    elif tiered_protection:
                        profit_atr = (position['entry_p'] - prev_lwm) / sig['atr'] if sig['atr'] > 0 else 0
                        if protected_before_tick and profit_atr >= 3.0:
                            tiered_exit = min(sig['prev_high_1'], prev_lwm + 1.5 * sig['atr'])
                            if current_p >= tiered_exit:
                                exit_p, reason, exit_triggered = tiered_exit, 'PT-T3', True
                        elif protected_before_tick and profit_atr >= 2.0:
                            tiered_exit = min(sig['prev_high_1'], position['entry_p'] - 0.5 * sig['atr'])
                            if current_p >= tiered_exit:
                                exit_p, reason, exit_triggered = tiered_exit, 'PT-T2', True
                        elif protected_before_tick and current_p >= sig['prev_high_1']:
                            exit_p, reason, exit_triggered = sig['prev_high_1'], 'PT', True
                    else:
                        if protected_before_tick and current_p >= sig['prev_high_1']:
                            exit_p, reason, exit_triggered = sig['prev_high_1'], 'PT', True

                    if not exit_triggered:
                        position['lwm'] = min(prev_lwm, current_p)
                        if tiered_protection:
                            profit_atr = (position['entry_p'] - position['lwm']) / sig['atr'] if sig['atr'] > 0 else 0
                            if profit_atr >= PROFIT_PROTECT and not position['protected']:
                                position['protected'] = True
                        else:
                            if not position['protected'] and current_p <= position['entry_p'] - PROFIT_PROTECT * sig['atr']:
                                position['protected'] = True
                        if trailing_stop_atr is not None:
                            is_active = True
                            if delayed_trailing_activation is not None:
                                profit_atr = (position['entry_p'] - position['lwm']) / sig['atr'] if sig['atr'] > 0 else 0
                                if profit_atr < delayed_trailing_activation:
                                    is_active = False
                            if is_active:
                                trailing_sl = position['lwm'] + trailing_stop_atr * sig['atr']
                                position['sl'] = min(position['sl'], trailing_sl)

                if exit_triggered:
                    side_mult = 1 if position['side'] == 'long' else -1
                    exit_p = exit_p * (1 - SLIPPAGE) if position['side'] == 'long' else exit_p * (1 + SLIPPAGE)
                    if position['notional'] > 0:
                        pnl = side_mult * (exit_p - position['entry_p']) / position['entry_p'] * position['notional']
                        balance += pnl - (position['notional'] * COMMISSION)
                    trade_logs.append({
                        'time': current_time,
                        'type': 'EXIT',
                        'price': exit_p,
                        'reason': reason,
                        'notional': position['notional'],
                        'bal': balance,
                    })
                    last_exit_reason = reason
                    last_exit_side = position['side']
                    last_exit_bar_index = i
                    position = None

                    if balance > peak_balance:
                        peak_balance = balance
                    if max_drawdown_pct is not None and peak_balance > 0:
                        current_dd = (peak_balance - balance) / peak_balance
                        if current_dd > max_drawdown_pct:
                            circuit_breaker_until = i + cooldown_bars
                            trade_logs.append({
                                'time': current_time,
                                'type': 'CIRCUIT_BREAKER',
                                'price': 0,
                                'reason': f'DD:{current_dd:.2%}',
                                'notional': 0,
                                'bal': balance,
                            })
                    idx = window_ticks.index.searchsorted(current_time, side='right')
                else:
                    idx += 1

    if position is not None and not df_tick.empty:
        last_tick_time = df_tick.index[-1]
        last_price = float(df_tick.iloc[-1]['price'])
        side_mult = 1 if position['side'] == 'long' else -1
        final_exit_p = last_price * (1 - SLIPPAGE) if position['side'] == 'long' else last_price * (1 + SLIPPAGE)
        if position['notional'] > 0:
            pnl = side_mult * (final_exit_p - position['entry_p']) / position['entry_p'] * position['notional']
            balance += pnl - (position['notional'] * COMMISSION)
        trade_logs.append({
            'time': last_tick_time,
            'type': 'EXIT',
            'price': final_exit_p,
            'reason': 'FinalMarkToMarket',
            'notional': position['notional'],
            'bal': balance,
        })

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

def generate_1d_signals(df_1min, atr_period=14):
    print(f"正在聚合 1Min 数据生成 1D (日线) 信号数据... (ATR 周期: {atr_period})")
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
    df_1d['atr'] = true_range.rolling(atr_period).mean()
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
                                  long_reentry_atr=0.1,
                                  short_reentry_atr=0.0,
                                  stop_mode=None,
                                  profit_protect_atr=1.0):
    """
    1min 颗粒度双向回测引擎
    df_1min: 包含 open, high, low, close 的标准化 1min 数据
    df_4h: 包含策略锚点(ma20, atr, prev_high_2 等)的 4H 数据
    """
    df_1min = _normalize_utc_index(df_1min)
    df_4h = _normalize_utc_index(df_4h)

    balance = initial_balance
    position = None # {'side', 'entry_p', 'sl', 'protected', 'notional'}
    trade_logs = []
    # 策略参数
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
        f"ReentryATR(L/S):{long_reentry_atr}/{short_reentry_atr} | "
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
                    re_p = sig['prev_low_1'] + (long_reentry_atr * sig['atr'])
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
                                    'protected': reason.startswith('PT'),
                                    'notional': notional_value
                                }
                                balance -= notional_value * COMMISSION
                                trade_logs.append({'time': bar_time, 'type': 'BUY', 'price': entry_price, 'reason': reason, 'notional': notional_value,'bal': balance})
                                if reason == 'SL-Reentry': trades_in_bar += 1
                                executed = True
                            last_exit_side = None

                # 2. 空头逻辑 (MA20 之下)
                elif short_regime_ready:
                    re_p = sig['prev_high_1'] - (short_reentry_atr * sig['atr'])
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
                                    'protected': reason.startswith('PT'),
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

    if position and len(df_1min) > 0:
        last_bar_time = df_1min.index[-1]
        last_close = float(df_1min.iloc[-1]['close'])
        exit_p = last_close * (1 - SLIPPAGE) if position['side'] == 'long' else last_close * (1 + SLIPPAGE)
        side_mult = 1 if position['side'] == 'long' else -1
        if position['notional'] > 0:
            pnl = side_mult * (exit_p - position['entry_p']) / position['entry_p'] * position['notional']
            balance += pnl - (position['notional'] * COMMISSION)
        trade_logs.append({
            'time': last_bar_time,
            'type': 'EXIT',
            'price': exit_p,
            'reason': 'FinalMarkToMarket',
            'notional': position['notional'],
            'bal': balance,
        })

    if position is not None and not df_1min.empty:
        last_bar_time = df_1min.index[-1]
        last_close = float(df_1min.iloc[-1]['close'])
        side_mult = 1 if position['side'] == 'long' else -1
        final_exit_p = last_close * (1 - SLIPPAGE) if position['side'] == 'long' else last_close * (1 + SLIPPAGE)
        if position['notional'] > 0:
            pnl = side_mult * (final_exit_p - position['entry_p']) / position['entry_p'] * position['notional']
            balance += pnl - (position['notional'] * COMMISSION)
        trade_logs.append({
            'time': last_bar_time,
            'type': 'EXIT',
            'price': final_exit_p,
            'reason': 'FinalMarkToMarket',
            'notional': position['notional'],
            'bal': balance,
        })

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

def run_backtest_enhanced(df_1min, df_4h, initial_balance=100000.0,
                          dir1_reentry_confirm=False,
                          dir2_zero_initial=False,
                          dir3_structural_sl=False,
                          fixed_slippage=None,
                          stop_loss_atr=0.2,
                          max_trades_per_bar=3,
                          reentry_size_schedule=None,
                          long_reentry_atr=0.1,
                          short_reentry_atr=0.0,
                          stop_mode=None,
                          profit_protect_atr=1.0,
                          # ===== 新增优化参数 =====
                          trailing_stop_atr=None,
                          delayed_trailing_activation=None,
                          tiered_protection=False,
                          max_drawdown_pct=None,
                          cooldown_bars=6,
                          reentry_mode='default'):
    """
    增强版 1min 回测引擎，在原始引擎基础上新增：

    Parameters (新增):
    - trailing_stop_atr: float 或 None, 启用 ATR Trailing Stop。持仓期间
      SL 会跟随高水位/低水位动态收紧。例如 1.5 = 最高价 - 1.5*ATR。
    - tiered_protection: bool, 启用分层利润保护。浮盈越大，退出标准越紧。
    - max_drawdown_pct: float 或 None, 最大回撤熔断 (0-1)。
      例如 0.05 = 5% 回撤后暂停交易。
    - cooldown_bars: int, 熔断冷却期（信号 bar 数量）。
    - reentry_mode: str, 'default'=原始递增, 'fixed'=固定大小, 'decreasing'=递减。
    """
    df_1min = _normalize_utc_index(df_1min)
    df_4h = _normalize_utc_index(df_4h)

    balance = initial_balance
    peak_balance = initial_balance
    position = None
    trade_logs = []

    COMMISSION = 0.0010
    MAX_TRADES_PER_BAR = max_trades_per_bar
    SLIPPAGE = np.random.uniform(0.0005, 0.002) if fixed_slippage is None else fixed_slippage
    CASH_USAGE_INITIAL = 0.0 if dir2_zero_initial else 0.10
    REENTRY_SIZE_SCHEDULE = _normalize_reentry_sizes(reentry_size_schedule)
    PROFIT_PROTECT = profit_protect_atr
    stop_mode = 'structural' if dir3_structural_sl and stop_mode is None else (stop_mode or 'atr')

    # 根据 reentry_mode 调整 schedule
    if reentry_mode == 'fixed':
        base = REENTRY_SIZE_SCHEDULE[0] if REENTRY_SIZE_SCHEDULE else 0.10
        REENTRY_SIZE_SCHEDULE = [base] * max(len(REENTRY_SIZE_SCHEDULE), 3)
    elif reentry_mode == 'decreasing':
        base = REENTRY_SIZE_SCHEDULE[0] if REENTRY_SIZE_SCHEDULE else 0.10
        REENTRY_SIZE_SCHEDULE = [base * (0.5 ** i) for i in range(max(len(REENTRY_SIZE_SCHEDULE), 3))]

    last_exit_bar_index = -999
    REENTRY_TIMEOUT = 1
    last_exit_reason = None
    last_exit_side = None

    # 熔断状态
    circuit_breaker_until = -999  # bar index until which trading is paused

    label_parts = []
    if trailing_stop_atr: label_parts.append(f"Trail:{trailing_stop_atr}")
    if tiered_protection: label_parts.append("TieredPT")
    if max_drawdown_pct: label_parts.append(f"CB:{max_drawdown_pct:.0%}")
    if reentry_mode != 'default': label_parts.append(f"Re:{reentry_mode}")
    label_parts.append(f"ReATR:{long_reentry_atr}/{short_reentry_atr}")
    label = " | ".join(label_parts) if label_parts else "Enhanced"

    print(
        f"🔬 Enhanced回测 [{label}] | Stop:{stop_mode} ATR:{stop_loss_atr} | "
        f"Slippage:{SLIPPAGE}"
    )

    for i in range(len(df_4h) - 1):
        start_t, end_t = df_4h.index[i], df_4h.index[i + 1]
        window = df_1min.loc[start_t:end_t]
        if window.empty:
            continue

        sig = df_4h.iloc[i]
        if pd.isna(sig['atr']):
            continue
        long_regime_ready, short_regime_ready = _resolve_regime_ready(
            sig, '1d' if 'ma5' in df_4h.columns else '4h'
        )

        trades_in_bar = 0
        current_idx = 0
        total_steps = len(window)

        if i - last_exit_bar_index > REENTRY_TIMEOUT:
            last_exit_side = None

        while current_idx < total_steps:
            bar = window.iloc[current_idx]
            bar_time = window.index[current_idx]
            prev_bar = window.iloc[current_idx - 1] if current_idx > 0 else None

            if not position:
                # ===== 熔断检查 =====
                if max_drawdown_pct is not None and i < circuit_breaker_until:
                    current_idx += 1
                    continue

                executed = False

                # 多头逻辑
                if long_regime_ready:
                    re_p = sig['prev_low_1'] + (long_reentry_atr * sig['atr'])
                    if trades_in_bar == 0 and sig['prev_high_2'] > sig['prev_high_1']:
                        if bar['high'] >= sig['prev_high_2']:
                            entry_p = max(bar['open'], sig['prev_high_2']) * (1 + SLIPPAGE)
                            notional_value = balance * CASH_USAGE_INITIAL
                            initial_sl = _resolve_stop_price('long', entry_p, sig, stop_mode, stop_loss_atr)
                            position = {
                                'side': 'long', 'entry_p': entry_p,
                                'sl': initial_sl, 'protected': False,
                                'notional': notional_value,
                                'hwm': entry_p,  # high water mark
                            }
                            balance -= notional_value * COMMISSION
                            trade_logs.append({'time': bar_time, 'type': 'BUY', 'price': entry_p,
                                               'reason': 'Initial', 'notional': notional_value, 'bal': balance})
                            trades_in_bar += 1
                            executed = True

                    elif last_exit_side == 'long' and (i - last_exit_bar_index <= REENTRY_TIMEOUT):
                        is_triggered = False
                        entry_p_raw = re_p
                        if dir1_reentry_confirm:
                            if prev_bar is not None and bar['close'] > re_p and prev_bar['close'] > re_p:
                                is_triggered = True
                                entry_p_raw = bar['close']
                        else:
                            if bar['high'] >= re_p:
                                is_triggered = True

                        if is_triggered:
                            reason = 'SL-Reentry' if last_exit_reason == 'SL' else 'PT-Reentry'
                            if trades_in_bar < MAX_TRADES_PER_BAR:
                                current_reentry_size = _get_reentry_size(trades_in_bar, REENTRY_SIZE_SCHEDULE)
                                notional_value = balance * current_reentry_size
                                entry_price = entry_p_raw * (1 + SLIPPAGE)
                                reentry_sl = _resolve_stop_price('long', entry_price, sig, stop_mode, stop_loss_atr)
                                position = {
                                    'side': 'long', 'entry_p': entry_price,
                                    'sl': reentry_sl, 'protected': (reason == 'PT-Reentry'),
                                    'notional': notional_value,
                                    'hwm': entry_price,
                                }
                                balance -= notional_value * COMMISSION
                                trade_logs.append({'time': bar_time, 'type': 'BUY', 'price': entry_price,
                                                   'reason': reason, 'notional': notional_value, 'bal': balance})
                                trades_in_bar += 1
                                executed = True
                            last_exit_side = None

                elif short_regime_ready:
                    re_p = sig['prev_high_1'] - (short_reentry_atr * sig['atr'])
                    if trades_in_bar == 0 and sig['prev_low_2'] < sig['prev_low_1']:
                        if bar['low'] <= sig['prev_low_2']:
                            entry_p = min(bar['open'], sig['prev_low_2']) * (1 - SLIPPAGE)
                            notional_value = balance * CASH_USAGE_INITIAL
                            initial_sl = _resolve_stop_price('short', entry_p, sig, stop_mode, stop_loss_atr)
                            position = {
                                'side': 'short', 'entry_p': entry_p,
                                'sl': initial_sl, 'protected': False,
                                'notional': notional_value,
                                'lwm': entry_p,  # low water mark
                            }
                            balance -= notional_value * COMMISSION
                            trade_logs.append({'time': bar_time, 'type': 'SHORT', 'price': entry_p,
                                               'reason': 'Initial', 'notional': notional_value, 'bal': balance})
                            trades_in_bar += 1
                            executed = True

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

                        if is_triggered:
                            reason = 'SL-Reentry' if last_exit_reason == 'SL' else 'PT-Reentry'
                            if trades_in_bar < MAX_TRADES_PER_BAR:
                                current_reentry_size = _get_reentry_size(trades_in_bar, REENTRY_SIZE_SCHEDULE)
                                notional_value = balance * current_reentry_size
                                entry_price = entry_p_raw * (1 - SLIPPAGE)
                                reentry_sl = _resolve_stop_price('short', entry_price, sig, stop_mode, stop_loss_atr)
                                position = {
                                    'side': 'short', 'entry_p': entry_price,
                                    'sl': reentry_sl, 'protected': (reason == 'PT-Reentry'),
                                    'notional': notional_value,
                                    'lwm': entry_price,
                                }
                                balance -= notional_value * COMMISSION
                                trade_logs.append({'time': bar_time, 'type': 'SHORT', 'price': entry_price,
                                                   'reason': reason, 'notional': notional_value, 'bal': balance})
                                trades_in_bar += 1
                                executed = True
                            last_exit_side = None

                current_idx += 1

            else:
                # ===== 持仓管理 =====
                exit_triggered = False
                exit_p = 0.0
                reason = ''

                if position['side'] == 'long':
                    prev_hwm = position.get('hwm', position['entry_p'])
                    protected_before_bar = position.get('protected', False)

                    # ===== Trailing Stop =====
                    if trailing_stop_atr is not None:
                        is_active = True
                        if delayed_trailing_activation is not None:
                            profit_atr = (prev_hwm - position['entry_p']) / sig['atr'] if sig['atr'] > 0 else 0
                            if profit_atr < delayed_trailing_activation:
                                is_active = False
                        
                        if is_active:
                            trailing_sl = prev_hwm - trailing_stop_atr * sig['atr']
                            position['sl'] = max(position['sl'], trailing_sl)

                    # SL check (优先级最高)
                    if bar['low'] <= position['sl']:
                        exit_p, reason, exit_triggered = position['sl'], 'SL', True
                    elif tiered_protection:
                        profit_atr = (prev_hwm - position['entry_p']) / sig['atr'] if sig['atr'] > 0 else 0
                        if protected_before_bar and profit_atr >= 3.0:
                            tiered_exit = max(sig['prev_low_1'], prev_hwm - 1.5 * sig['atr'])
                            if bar['low'] <= tiered_exit:
                                exit_p, reason, exit_triggered = tiered_exit, 'PT-T3', True
                        elif protected_before_bar and profit_atr >= 2.0:
                            tiered_exit = max(sig['prev_low_1'], position['entry_p'] + 0.5 * sig['atr'])
                            if bar['low'] <= tiered_exit:
                                exit_p, reason, exit_triggered = tiered_exit, 'PT-T2', True
                        elif protected_before_bar and bar['low'] <= sig['prev_low_1']:
                            exit_p, reason, exit_triggered = sig['prev_low_1'], 'PT', True
                    else:
                        if protected_before_bar and bar['low'] <= sig['prev_low_1']:
                            exit_p, reason, exit_triggered = sig['prev_low_1'], 'PT', True

                    if not exit_triggered:
                        # 当前 bar 结束后再更新水位和保护状态，避免把同 bar 的 high/low 当作已知路径。
                        position['hwm'] = max(prev_hwm, bar['high'])
                        if tiered_protection:
                            profit_atr = (position['hwm'] - position['entry_p']) / sig['atr'] if sig['atr'] > 0 else 0
                            if profit_atr >= PROFIT_PROTECT and not position['protected']:
                                position['protected'] = True
                        else:
                            if not position['protected'] and bar['high'] >= position['entry_p'] + PROFIT_PROTECT * sig['atr']:
                                position['protected'] = True
                        if trailing_stop_atr is not None:
                            is_active = True
                            if delayed_trailing_activation is not None:
                                profit_atr = (position['hwm'] - position['entry_p']) / sig['atr'] if sig['atr'] > 0 else 0
                                if profit_atr < delayed_trailing_activation:
                                    is_active = False
                            if is_active:
                                trailing_sl = position['hwm'] - trailing_stop_atr * sig['atr']
                                position['sl'] = max(position['sl'], trailing_sl)

                else:  # short
                    prev_lwm = position.get('lwm', position['entry_p'])
                    protected_before_bar = position.get('protected', False)

                    # ===== Trailing Stop =====
                    if trailing_stop_atr is not None:
                        is_active = True
                        if delayed_trailing_activation is not None:
                            profit_atr = (position['entry_p'] - prev_lwm) / sig['atr'] if sig['atr'] > 0 else 0
                            if profit_atr < delayed_trailing_activation:
                                is_active = False

                        if is_active:
                            trailing_sl = prev_lwm + trailing_stop_atr * sig['atr']
                            position['sl'] = min(position['sl'], trailing_sl)

                    if bar['high'] >= position['sl']:
                        exit_p, reason, exit_triggered = position['sl'], 'SL', True
                    elif tiered_protection:
                        profit_atr = (position['entry_p'] - prev_lwm) / sig['atr'] if sig['atr'] > 0 else 0
                        if protected_before_bar and profit_atr >= 3.0:
                            tiered_exit = min(sig['prev_high_1'], prev_lwm + 1.5 * sig['atr'])
                            if bar['high'] >= tiered_exit:
                                exit_p, reason, exit_triggered = tiered_exit, 'PT-T3', True
                        elif protected_before_bar and profit_atr >= 2.0:
                            tiered_exit = min(sig['prev_high_1'], position['entry_p'] - 0.5 * sig['atr'])
                            if bar['high'] >= tiered_exit:
                                exit_p, reason, exit_triggered = tiered_exit, 'PT-T2', True
                        elif protected_before_bar and bar['high'] >= sig['prev_high_1']:
                            exit_p, reason, exit_triggered = sig['prev_high_1'], 'PT', True
                    else:
                        if protected_before_bar and bar['high'] >= sig['prev_high_1']:
                            exit_p, reason, exit_triggered = sig['prev_high_1'], 'PT', True

                    if not exit_triggered:
                        # 当前 bar 结束后再更新水位和保护状态，避免把同 bar 的 high/low 当作已知路径。
                        position['lwm'] = min(prev_lwm, bar['low'])
                        if tiered_protection:
                            profit_atr = (position['entry_p'] - position['lwm']) / sig['atr'] if sig['atr'] > 0 else 0
                            if profit_atr >= PROFIT_PROTECT and not position['protected']:
                                position['protected'] = True
                        else:
                            if not position['protected'] and bar['low'] <= position['entry_p'] - PROFIT_PROTECT * sig['atr']:
                                position['protected'] = True
                        if trailing_stop_atr is not None:
                            is_active = True
                            if delayed_trailing_activation is not None:
                                profit_atr = (position['entry_p'] - position['lwm']) / sig['atr'] if sig['atr'] > 0 else 0
                                if profit_atr < delayed_trailing_activation:
                                    is_active = False
                            if is_active:
                                trailing_sl = position['lwm'] + trailing_stop_atr * sig['atr']
                                position['sl'] = min(position['sl'], trailing_sl)

                if exit_triggered:
                    side_mult = 1 if position['side'] == 'long' else -1
                    exit_p = exit_p * (1 - SLIPPAGE) if position['side'] == 'long' else exit_p * (1 + SLIPPAGE)
                    if position['notional'] > 0:
                        pnl = side_mult * (exit_p - position['entry_p']) / position['entry_p'] * position['notional']
                        balance += pnl - (position['notional'] * COMMISSION)
                    else:
                        pnl = 0
                    trade_logs.append({'time': bar_time, 'type': 'EXIT', 'price': exit_p,
                                       'reason': reason, 'notional': position['notional'], 'bal': balance})
                    last_exit_reason = reason
                    last_exit_side = position['side']
                    last_exit_bar_index = i
                    position = None

                    # ===== 更新峰值余额 & 熔断检查 =====
                    if balance > peak_balance:
                        peak_balance = balance
                    if max_drawdown_pct is not None and peak_balance > 0:
                        current_dd = (peak_balance - balance) / peak_balance
                        if current_dd > max_drawdown_pct:
                            circuit_breaker_until = i + cooldown_bars
                            trade_logs.append({'time': bar_time, 'type': 'CIRCUIT_BREAKER',
                                               'price': 0, 'reason': f'DD:{current_dd:.2%}',
                                               'notional': 0, 'bal': balance})

                current_idx += 1

    if position is not None and not df_1min.empty:
        last_bar_time = df_1min.index[-1]
        last_close = float(df_1min.iloc[-1]['close'])
        side_mult = 1 if position['side'] == 'long' else -1
        final_exit_p = last_close * (1 - SLIPPAGE) if position['side'] == 'long' else last_close * (1 + SLIPPAGE)
        if position['notional'] > 0:
            pnl = side_mult * (final_exit_p - position['entry_p']) / position['entry_p'] * position['notional']
            balance += pnl - (position['notional'] * COMMISSION)
        trade_logs.append({
            'time': last_bar_time,
            'type': 'EXIT',
            'price': final_exit_p,
            'reason': 'FinalMarkToMarket',
            'notional': position['notional'],
            'bal': balance,
        })

    return pd.DataFrame(trade_logs)


def _compute_backtest_stats(ledger, initial_balance):
    """从回测流水中提取关键绩效指标"""
    if ledger.empty:
        return {'return': 0.0, 'max_dd': 0.0, 'trades': 0, 'win_rate': 0.0,
                'avg_pnl_pct': 0.0, 'sharpe': 0.0, 'calmar': 0.0, 'final_bal': initial_balance}

    exits = ledger[ledger['type'] == 'EXIT']
    final_bal = ledger.iloc[-1]['bal']
    total_return = final_bal / initial_balance - 1

    ledger_copy = ledger[ledger['type'].isin(['BUY', 'SHORT', 'EXIT'])].copy()
    ledger_copy['cum_max'] = ledger_copy['bal'].cummax()
    ledger_copy['drawdown'] = ledger_copy['bal'] / ledger_copy['cum_max'] - 1
    max_dd = ledger_copy['drawdown'].min()

    # 配对交易统计
    def normalize_exit_reason(reason):
        if isinstance(reason, str) and reason.startswith('PT-'):
            return 'PT'
        return reason

    trades = []
    temp = None
    for _, row in ledger_copy.iterrows():
        if row['type'] in ['BUY', 'SHORT']:
            if row.get('notional', 0) <= 0:
                continue
            temp = row
        elif row['type'] == 'EXIT' and temp is not None:
            pnl_pct = (row['price'] - temp['price']) / temp['price']
            if temp['type'] == 'SHORT':
                pnl_pct = -pnl_pct
            trades.append({
                'pnl_val': row['bal'] - temp['bal'],
                'pnl_pct': pnl_pct,
                'side': 'Long' if temp['type'] == 'BUY' else 'Short',
                'reason': normalize_exit_reason(row['reason'])
            })
            temp = None

    n_trades = len(trades)
    if n_trades == 0:
        return {'return': total_return, 'max_dd': max_dd, 'trades': 0,
                'win_rate': 0.0, 'avg_pnl_pct': 0.0, 'sharpe': 0.0,
                'calmar': 0.0, 'final_bal': final_bal}

    trade_df = pd.DataFrame(trades)
    win_rate = (trade_df['pnl_val'] > 0).sum() / n_trades
    avg_pnl = trade_df['pnl_pct'].mean()

    # 简化 Sharpe (假设无风险利率 = 0)
    if trade_df['pnl_pct'].std() > 0:
        sharpe = avg_pnl / trade_df['pnl_pct'].std() * np.sqrt(252)
    else:
        sharpe = 0.0

    # Calmar Ratio
    calmar = total_return / abs(max_dd) if max_dd != 0 else 0.0

    # 按退出原因统计
    reason_counts = trade_df['reason'].value_counts().to_dict()

    return {
        'return': total_return,
        'max_dd': max_dd,
        'trades': n_trades,
        'win_rate': win_rate,
        'avg_pnl_pct': avg_pnl,
        'sharpe': sharpe,
        'calmar': calmar,
        'final_bal': final_bal,
        'reason_counts': reason_counts,
    }


def run_strategy_optimization_matrix():
    """运行策略优化对比矩阵：Baseline vs 各优化方案"""
    SIGNALS_1Min_PATH = 'BTC_1min_Clean.csv'
    if not os.path.exists(SIGNALS_1Min_PATH):
        print("错误：找不到 1min 数据文件！")
        return

    df_1min = pd.read_csv(SIGNALS_1Min_PATH, index_col=0, parse_dates=True)
    initial_balance = 100000.0
    start_date = "2020-01-01"
    end_date = "2026-02-28"

    df_1min_sliced = df_1min.loc[start_date:end_date]
    df_1d_sliced = generate_1d_signals(df_1min_sliced)

    # 公共基础参数
    base_kwargs = {
        'dir2_zero_initial': True,
        'fixed_slippage': 0.0005,
        'stop_loss_atr': 0.05,
        'stop_mode': 'atr',
        'max_trades_per_bar': 3,
        'reentry_size_schedule': [0.10, 0.20],
    }

    scenarios = [
        # === 对照组 ===
        ("① Baseline (当前策略)", {**base_kwargs}),

        # === P0-1: Trailing Stop 测试 ===
        ("② Trail ATR=1.0", {**base_kwargs, 'trailing_stop_atr': 1.0}),
        ("③ Trail ATR=1.5", {**base_kwargs, 'trailing_stop_atr': 1.5}),
        ("④ Trail ATR=2.0", {**base_kwargs, 'trailing_stop_atr': 2.0}),

        # === P0-2: 分层 Protection ===
        ("⑤ Tiered Protection", {**base_kwargs, 'tiered_protection': True}),

        # === P0-3: Trailing + Tiered 组合 ===
        ("⑥ Trail 1.5 + Tiered", {**base_kwargs, 'trailing_stop_atr': 1.5, 'tiered_protection': True}),

        # === P1-1: 熔断 ===
        ("⑦ Circuit Breaker 5%", {**base_kwargs, 'max_drawdown_pct': 0.05, 'cooldown_bars': 6}),

        # === P1-2: Reentry Sizing ===
        ("⑧ Reentry Fixed", {**base_kwargs, 'reentry_mode': 'fixed'}),
        ("⑨ Reentry Decreasing", {**base_kwargs, 'reentry_mode': 'decreasing'}),

        # === 最优组合 ===
        ("⑩ Full Enhancement", {
            **base_kwargs,
            'trailing_stop_atr': 1.5,
            'tiered_protection': True,
            'max_drawdown_pct': 0.05,
            'cooldown_bars': 6,
            'reentry_mode': 'fixed',
        }),
    ]

    results = []
    equity_curves = {}

    print("\n" + "=" * 70)
    print("🔬 策略优化对比矩阵 — Baseline vs 改进方案")
    print("=" * 70)

    for name, kwargs in scenarios:
        print(f"\n{'─'*50}")
        print(f"▶ {name}")

        if name.startswith("①"):
            # Baseline 使用原始引擎
            ledger = run_backtest_1min_granularity(
                df_1min_sliced, df_1d_sliced, initial_balance, **kwargs
            )
        else:
            ledger = run_backtest_enhanced(
                df_1min_sliced, df_1d_sliced, initial_balance, **kwargs
            )

        stats = _compute_backtest_stats(ledger, initial_balance)
        results.append({
            'Scenario': name,
            'Return': stats['return'],
            'Max DD': stats['max_dd'],
            'Trades': stats['trades'],
            'Win Rate': stats['win_rate'],
            'Sharpe': stats['sharpe'],
            'Calmar': stats['calmar'],
        })

        # 收集权益曲线
        if not ledger.empty:
            exits_only = ledger[ledger['type'].isin(['EXIT', 'BUY', 'SHORT'])].copy()
            equity_curves[name] = exits_only[['time', 'bal']].copy()

        reason_str = ', '.join(f"{k}:{v}" for k, v in stats.get('reason_counts', {}).items())
        print(f"  收益: {stats['return']:.2%} | MaxDD: {stats['max_dd']:.2%} | "
              f"交易: {stats['trades']} | 胜率: {stats['win_rate']:.1%} | "
              f"Sharpe: {stats['sharpe']:.2f} | Calmar: {stats['calmar']:.2f}")
        if reason_str:
            print(f"  退出分布: {reason_str}")

    # ===== 汇总表格 =====
    print("\n" + "=" * 70)
    print("🏆 策略优化对比矩阵汇总")
    print("=" * 70)

    df_res = pd.DataFrame(results)
    formatters = {
        'Return': '{:.2%}'.format,
        'Max DD': '{:.2%}'.format,
        'Win Rate': '{:.1%}'.format,
        'Sharpe': '{:.2f}'.format,
        'Calmar': '{:.2f}'.format,
    }
    print(df_res.to_string(index=False, formatters=formatters))
    print("=" * 70)

    # ===== 权益曲线对比图 =====
    try:
        fig, axes = plt.subplots(2, 1, figsize=(16, 12))

        # 上图：权益曲线
        ax1 = axes[0]
        for name, curve in equity_curves.items():
            if curve.empty:
                continue
            curve_plot = curve.copy()
            curve_plot['time'] = pd.to_datetime(curve_plot['time'])
            label = name.split(' ', 1)[1] if ' ' in name else name
            linewidth = 2.5 if '①' in name or '⑩' in name else 1.0
            alpha = 1.0 if '①' in name or '⑩' in name else 0.6
            ax1.plot(curve_plot['time'], curve_plot['bal'], label=label, linewidth=linewidth, alpha=alpha)

        ax1.set_title('Strategy Optimization: Equity Curves Comparison', fontsize=14)
        ax1.set_ylabel('Balance (USDT)')
        ax1.legend(fontsize=8, loc='upper left')
        ax1.grid(True, alpha=0.3)

        # 下图：收益/风险散点图
        ax2 = axes[1]
        returns = [r['Return'] for r in results]
        max_dds = [abs(r['Max DD']) for r in results]
        labels = [r['Scenario'].split(' ', 1)[0] for r in results]

        scatter = ax2.scatter(max_dds, returns, s=100, c=range(len(results)), cmap='viridis', zorder=5)
        for i, label in enumerate(labels):
            ax2.annotate(label, (max_dds[i], returns[i]), textcoords="offset points",
                         xytext=(8, 5), fontsize=9)

        ax2.set_title('Risk-Return Tradeoff: Strategy Variants', fontsize=14)
        ax2.set_xlabel('Max Drawdown (absolute)')
        ax2.set_ylabel('Total Return')
        ax2.grid(True, alpha=0.3)

        plt.tight_layout()
        plt.savefig('strategy_optimization_comparison.png', dpi=300)
        print("\n✅ 对比图已生成: strategy_optimization_comparison.png")
    except Exception as e:
        print(f"\n⚠️ 图表生成失败: {e}")

    df_res.to_csv('strategy_optimization_results.csv', index=False)
    print("✅ 结果已保存: strategy_optimization_results.csv")

    return df_res


def run_atr_period_comparison():
    """比对不同 ATR 周期 (7, 14, 21, 28) 在最优逻辑下的表现"""
    SIGNALS_1Min_PATH = 'BTC_1min_Clean.csv'
    if not os.path.exists(SIGNALS_1Min_PATH):
        print("错误：找不到 1min 数据文件！")
        return

    df_1min = pd.read_csv(SIGNALS_1Min_PATH, index_col=0, parse_dates=True)
    initial_balance = 100000.0
    start_date = "2020-01-01"
    end_date = "2026-02-28"

    df_1min_sliced = df_1min.loc[start_date:end_date]
    
    atr_periods_to_test = [7, 14, 21, 28, 35]

    base_kwargs = {
        'dir2_zero_initial': True,
        'fixed_slippage': 0.0005,
        'stop_loss_atr': 0.05,
        'stop_mode': 'atr',
        'max_trades_per_bar': 4,
        'reentry_size_schedule': [0.10, 0.05, 0.025],
        'trailing_stop_atr': 0.3,
        'delayed_trailing_activation': 0.5, # 最优方案
    }

    print("\n" + "=" * 70)
    print("🔬 启动 ATR 周期宽度测试 (ATR7 vs 14 vs 21 vs 28 vs 35)")
    print("=" * 70)

    results = []

    for period in atr_periods_to_test:
        print(f"\n---> 开始生成 1D 信号和执行回测: ATR({period})")
        df_1d_sliced = generate_1d_signals(df_1min_sliced, atr_period=period)
        
        ledger = run_backtest_enhanced(
            df_1min_sliced, df_1d_sliced, initial_balance, 
            **base_kwargs
        )
        
        stats = _compute_backtest_stats(ledger, initial_balance)
        reason_str = ', '.join(f"{k}:{v}" for k, v in stats.get('reason_counts', {}).items())

        results.append({
            'ATR Period': f'ATR({period})',
            'Return': stats['return'],
            'Max DD': stats['max_dd'],
            'Trades': stats['trades'],
            'Win Rate': stats['win_rate'],
            'Sharpe': stats['sharpe'],
            'Calmar': stats['calmar']
        })
        
        print(f"  ATR({period}) 收益: {stats['return']:.2%} | MaxDD: {stats['max_dd']:.2%} | "
              f"交易: {stats['trades']} | Sharpe: {stats['sharpe']:.2f}")

    # ===== 汇总表格 =====
    print("\n" + "=" * 70)
    print("🏆 ATR 周期比对结果")
    print("=" * 70)
    df_res = pd.DataFrame(results)
    formatters = {
        'Return': '{:.2%}'.format,
        'Max DD': '{:.2%}'.format,
        'Win Rate': '{:.1%}'.format,
        'Sharpe': '{:.2f}'.format,
        'Calmar': '{:.2f}'.format,
    }
    print(df_res.to_string(index=False, formatters=formatters))
    print("=" * 70)

if __name__ == "__main__":
    run_atr_period_comparison()
