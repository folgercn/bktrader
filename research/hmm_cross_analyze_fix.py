import pandas as pd
from pathlib import Path

def run_cross(regime_csv, ledger_csv):
    regime_df = pd.read_csv(regime_csv, index_col=0, parse_dates=True)
    # ensure regime_df index is tz-aware
    if regime_df.index.tz is None:
        regime_df.index = regime_df.index.tz_localize("UTC")
    else:
        regime_df.index = regime_df.index.tz_convert("UTC")
        
    regime_series = regime_df.set_index(regime_df.index.normalize())["hmm_regime"]

    ledger = pd.read_csv(ledger_csv)
    entries = ledger[ledger["type"].isin(["BUY", "SHORT"])].copy().reset_index(drop=True)
    exits = ledger[ledger["type"] == "EXIT"].copy().reset_index(drop=True)
    
    res = {}
    for i in range(min(len(entries), len(exits))):
        et = pd.to_datetime(entries.iloc[i]["time"], utc=True)
        td = et.normalize()
        
        ep = float(entries.iloc[i]["price"])
        xp = float(exits.iloc[i]["price"])
        side = "long" if entries.iloc[i]["type"] == "BUY" else "short"
        pnl = (xp - ep)/ep*100 if side=="long" else (ep - xp)/ep*100
        
        ridx = regime_series.index.get_indexer([td], method="pad")[0]
        regime = regime_series.iloc[ridx] if ridx >= 0 else "Unknown"
        
        if regime not in res: res[regime] = []
        res[regime].append(pnl)
        
    for r, pnls in res.items():
        wins = sum(1 for p in pnls if p > 0)
        print(f"  {r:30s}: Trades={len(pnls):3d}, AvgPnL={sum(pnls)/len(pnls):.4f}%, WinRate={wins/len(pnls)*100:.1f}%")

print("BTCUSDT HMM Cross Analysis:")
run_cross("research/hmm_regime_BTCUSDT_test_timeseries.csv", "research/tmp_impulse_delayed_confirm_v2_BTCUSDT_c03_f10_ledger.csv")
print("\nETHUSDT HMM Cross Analysis:")
run_cross("research/hmm_regime_ETHUSDT_test_timeseries.csv", "research/tmp_impulse_delayed_confirm_v2_ETHUSDT_c03_f10_ledger.csv")

