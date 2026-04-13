import pandas as pd
import os

# Load the parquet file
print("Loading parquet file...")
parquet_file = "eth_perp_1m_dataset/ETHUSDT_perp_1m_master.parquet"
try:
    df = pd.read_parquet(parquet_file)
except Exception as e:
    print(f"pandas.read_parquet unavailable ({e}), falling back to pyarrow")
    import pyarrow.parquet as pq

    df = pq.read_table(parquet_file).to_pandas()

print(f"Loaded {len(df):,} rows")

# Convert to CSV
output_file = "eth_perp_1m_dataset/ETHUSDT_perp_1m_master.csv"
print(f"Converting to CSV: {output_file}")
df.to_csv(output_file, index=False)

# Show file size
file_size_mb = os.path.getsize(output_file) / 1024 / 1024
print(f"✅ Done! CSV saved: {output_file}")
print(f"📊 Rows: {len(df):,}")
print(f"💾 Size: {file_size_mb:.2f} MB")
