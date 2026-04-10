import os, io, zipfile, requests, datetime
import pandas as pd
import numpy as np
from concurrent.futures import ThreadPoolExecutor, as_completed
from tqdm import tqdm
import time

# CONFIG
OUT_DIR = "eth_perp_1m_dataset"
os.makedirs(OUT_DIR, exist_ok=True)
OUTPUT_FILE = os.path.join(OUT_DIR, "ETHUSDT_perp_1m_master.parquet")
SYMBOL = "ETHUSDT"
MAX_WORKERS = 10  # Parallel download threads
RETRY_ATTEMPTS = 3
TIMEOUT = 30

# Helpers
def month_range(start, end):
    """Generate (year, month) tuples from start to end date"""
    d = start
    while d <= end:
        yield d.year, d.month
        if d.month == 12:
            d = datetime.date(d.year+1, 1, 1)
        else:
            d = datetime.date(d.year, d.month+1, 1)

def download_single_file(url, retries=RETRY_ATTEMPTS):
    """Download a single file with retry logic"""
    for attempt in range(retries):
        try:
            r = requests.get(url, timeout=TIMEOUT)
            if r.status_code == 200:
                return r.content
            elif r.status_code == 404:
                return None
            time.sleep(0.5 * (attempt + 1))
        except Exception as e:
            if attempt == retries - 1:
                print(f"Failed to download {url}: {e}")
                return None
            time.sleep(1 * (attempt + 1))
    return None

def fetch_binance_zip_parallel(symbol, category, tf="1m", start=None, has_header=False):
    """Fetch Binance data in parallel for maximum speed"""
    if start is None:
        start = datetime.date(2019, 9, 1)
    end = datetime.date.today()
    
    base_url = f"https://data.binance.vision/data/futures/um/monthly/{category}/{symbol}/{tf}/"
    
    # Generate all URLs
    urls = []
    for y, m in month_range(start, end):
        fname = f"{symbol}-{tf}-{y:04d}-{m:02d}.zip"
        urls.append((base_url + fname, y, m))
    
    print(f"Downloading {category} data: {len(urls)} files...")
    dfs = []
    
    # Parallel download
    with ThreadPoolExecutor(max_workers=MAX_WORKERS) as executor:
        future_to_url = {executor.submit(download_single_file, url): (url, y, m) 
                         for url, y, m in urls}
        
        for future in tqdm(as_completed(future_to_url), total=len(urls), desc=category):
            url, y, m = future_to_url[future]
            content = future.result()
            
            if content:
                try:
                    z = zipfile.ZipFile(io.BytesIO(content))
                    for inner in z.namelist():
                        with z.open(inner) as f:
                            # Read with appropriate header setting
                            if has_header:
                                df = pd.read_csv(f)
                            else:
                                df = pd.read_csv(f, header=None)
                            dfs.append(df)
                except Exception as e:
                    print(f"Error processing {url}: {e}")
    
    if not dfs:
        return None
    
    return pd.concat(dfs, ignore_index=True)

def validate_data(df, name):
    """Validate data quality"""
    print(f"\n📊 Validating {name}...")
    
    # Check for nulls
    null_counts = df.isnull().sum()
    if null_counts.any():
        print(f"  ⚠️  Null values found:\n{null_counts[null_counts > 0]}")
    
    # Check for duplicates
    if 'timestamp_utc' in df.columns:
        dupes = df['timestamp_utc'].duplicated().sum()
        if dupes > 0:
            print(f"  ⚠️  {dupes} duplicate timestamps found - removing...")
            df = df.drop_duplicates(subset=['timestamp_utc'], keep='first')
    
    # Check time gaps (for 1m data, should be exactly 60s apart)
    if 'timestamp_utc' in df.columns and len(df) > 1:
        df_sorted = df.sort_values('timestamp_utc')
        time_diffs = df_sorted['timestamp_utc'].diff().dt.total_seconds()
        max_gap = time_diffs.max()
        gaps = (time_diffs > 120).sum()  # More than 2 minutes
        print(f"  ℹ️  Max time gap: {max_gap/60:.1f} minutes")
        print(f"  ℹ️  Gaps > 2 min: {gaps}")
    
    print(f"  ✅ {name} validated: {len(df)} rows")
    return df

# Step 1: OHLCV + Taker Volumes
print("\n")
print("STEP 1: Fetching OHLCV + Taker Volume Data")

df = fetch_binance_zip_parallel(SYMBOL, "klines", "1m", has_header=False)

if df is None:
    raise Exception("Failed to fetch klines data!")

df.columns = ["open_time","open","high","low","close","volume","close_time",
              "quote_volume","num_trades","taker_buy_base_vol","taker_buy_quote_vol","ignore"]

# Convert open_time to numeric, coercing errors to NaN
df["open_time"] = pd.to_numeric(df["open_time"], errors='coerce')

# Drop rows where open_time couldn't be converted (header rows)
df = df.dropna(subset=['open_time'])

df["timestamp_utc"] = pd.to_datetime(df["open_time"], unit="ms", utc=True)
df = df.drop(columns=["ignore","close_time","open_time"])

# Convert to proper types
float_cols = ["open","high","low","close","volume","quote_volume",
              "taker_buy_base_vol","taker_buy_quote_vol"]
df[float_cols] = df[float_cols].astype(float)
df["num_trades"] = df["num_trades"].astype(int)

# Calculate taker sell volumes
df["taker_sell_base_vol"] = df["volume"] - df["taker_buy_base_vol"]
df["taker_sell_quote_vol"] = df["quote_volume"] - df["taker_buy_quote_vol"]

df = validate_data(df, "OHLCV")

# Step 2: Mark Price
print("\n")
print("STEP 2: Fetching Mark Price Data")

mark = fetch_binance_zip_parallel(SYMBOL, "markPriceKlines", "1m", has_header=False)

if mark is not None:
    mark.columns = ["open_time","mark_open","mark_high","mark_low","mark_close",
                   "ignore1","close_time","ignore2","ignore3","ignore4","ignore5","ignore6"]
    
    # Convert and clean
    mark["open_time"] = pd.to_numeric(mark["open_time"], errors='coerce')
    mark = mark.dropna(subset=['open_time'])
    
    mark["timestamp_utc"] = pd.to_datetime(mark["open_time"], unit="ms", utc=True)
    mark = mark[["timestamp_utc","mark_open","mark_high","mark_low","mark_close"]]
    mark[["mark_open","mark_high","mark_low","mark_close"]] = mark[["mark_open","mark_high","mark_low","mark_close"]].astype(float)
    mark = validate_data(mark, "Mark Price")
else:
    print("⚠️  No mark price data available")

# Step 3: Index Price
print("\n")
print("STEP 3: Fetching Index Price Data")

index = fetch_binance_zip_parallel(SYMBOL, "indexPriceKlines", "1m", has_header=False)

if index is not None:
    index.columns = ["open_time","index_open","index_high","index_low","index_close",
                    "ignore1","close_time","ignore2","ignore3","ignore4","ignore5","ignore6"]
    
    # Convert and clean
    index["open_time"] = pd.to_numeric(index["open_time"], errors='coerce')
    index = index.dropna(subset=['open_time'])
    
    index["timestamp_utc"] = pd.to_datetime(index["open_time"], unit="ms", utc=True)
    index = index[["timestamp_utc","index_open","index_high","index_low","index_close"]]
    index[["index_open","index_high","index_low","index_close"]] = index[["index_open","index_high","index_low","index_close"]].astype(float)
    index = validate_data(index, "Index Price")
else:
    print("⚠️  No index price data available")

# Step 4: Funding Rates
print("\n")
print("STEP 4: Fetching Funding Rate Data")

fund_base = f"https://data.binance.vision/data/futures/um/monthly/fundingRate/{SYMBOL}/"
urls = []
for y, m in month_range(datetime.date(2019,9,1), datetime.date.today()):
    fname = f"{SYMBOL}-fundingRate-{y:04d}-{m:02d}.zip"
    urls.append((fund_base + fname, y, m))

fund_dfs = []
with ThreadPoolExecutor(max_workers=MAX_WORKERS) as executor:
    future_to_url = {executor.submit(download_single_file, url): (url, y, m) 
                     for url, y, m in urls}
    
    for future in tqdm(as_completed(future_to_url), total=len(urls), desc="fundingRate"):
        url, y, m = future_to_url[future]
        content = future.result()
        
        if content:
            try:
                z = zipfile.ZipFile(io.BytesIO(content))
                for inner in z.namelist():
                    with z.open(inner) as f:
                        fd = pd.read_csv(f)
                        fd["fundingTime"] = pd.to_datetime(fd["fundingTime"], unit="ms", utc=True)
                        fund_dfs.append(fd)
            except Exception as e:
                print(f"Error processing {url}: {e}")

funding = pd.concat(fund_dfs, ignore_index=True) if fund_dfs else None

if funding is not None:
    funding = funding[["fundingTime","fundingRate"]].rename(columns={"fundingTime":"timestamp_utc"})
    funding["fundingRate"] = funding["fundingRate"].astype(float)
    print(f"  ✅ Funding rates: {len(funding)} rows")

# Step 5: Recent Open Interest
print("\n")
print("STEP 5: Fetching Open Interest Data (~30 days)")

url = "https://fapi.binance.com/futures/data/openInterestHist"
params = {"symbol": SYMBOL, "period": "1m", "limit": 500}
oi_data = []

with tqdm(desc="Open Interest") as pbar:
    while True:
        try:
            r = requests.get(url, params=params, timeout=TIMEOUT)
            data = r.json()
            if not data:
                break
            oi_data.extend(data)
            pbar.update(len(data))
            
            if len(data) < 500:
                break
            params["startTime"] = data[-1]["timestamp"] + 60_000
        except Exception as e:
            print(f"Error fetching OI: {e}")
            break

df_oi = None
if oi_data:
    df_oi = pd.DataFrame(oi_data)
    df_oi["timestamp_utc"] = pd.to_datetime(df_oi["timestamp"], unit="ms", utc=True)
    df_oi["open_interest"] = df_oi["sumOpenInterest"].astype(float)
    df_oi = df_oi[["timestamp_utc", "open_interest"]]
    print(f"  ✅ Open Interest: {len(df_oi)} rows")

# Step 6: Merge All Data
print("\n")
print("STEP 6: Merging All Datasets")

full = df.copy()
initial_count = len(full)

if mark is not None:
    full = full.merge(mark, on="timestamp_utc", how="left")
    print(f"  ✅ Merged mark price")

if index is not None:
    full = full.merge(index, on="timestamp_utc", how="left")
    print(f"  ✅ Merged index price")

if funding is not None:
    full = full.merge(funding, on="timestamp_utc", how="left")
    full["fundingRate"] = full["fundingRate"].ffill()
    print(f"  ✅ Merged funding rate (forward-filled)")

if df_oi is not None:
    full = full.merge(df_oi, on="timestamp_utc", how="left")
    print(f"  ✅ Merged open interest")

# Step 7: Final Validation & Save
print("\n")
print("STEP 7: Final Validation & Save")

# Sort by timestamp
full = full.sort_values("timestamp_utc").reset_index(drop=True)

# Remove any duplicate timestamps
full = full.drop_duplicates(subset=['timestamp_utc'], keep='first')

# Verify OHLC logic
invalid_ohlc = (
    (full['high'] < full['low']) | 
    (full['high'] < full['open']) | 
    (full['high'] < full['close']) |
    (full['low'] > full['open']) |
    (full['low'] > full['close'])
)
if invalid_ohlc.any():
    print(f"  ⚠️  {invalid_ohlc.sum()} rows with invalid OHLC relationships")

# Save
full.to_parquet(OUTPUT_FILE, index=False, compression='snappy')

# Final report
print(f"\n")
print("✅ DOWNLOAD COMPLETE!")
print(f"📁 File: {OUTPUT_FILE}")
print(f"📊 Total rows: {len(full):,}")
print(f"📅 Date range: {full['timestamp_utc'].min()} to {full['timestamp_utc'].max()}")
print(f"💾 File size: {os.path.getsize(OUTPUT_FILE) / 1024 / 1024:.2f} MB")
print(f"\n📋 Columns ({len(full.columns)}):")
for col in full.columns:
    null_pct = (full[col].isnull().sum() / len(full)) * 100
    print(f"  • {col:25s} - {null_pct:5.2f}% null")
print(f"{'='*60}\n")

# Save summary
summary = {
    'total_rows': len(full),
    'date_range': f"{full['timestamp_utc'].min()} to {full['timestamp_utc'].max()}",
    'columns': list(full.columns),
    'file_size_mb': os.path.getsize(OUTPUT_FILE) / 1024 / 1024
}

import json
summary_file = os.path.join(OUT_DIR, "dataset_summary.json")
with open(summary_file, 'w') as f:
    json.dump(summary, f, indent=2, default=str)

print(f"📄 Summary saved to {summary_file}")
