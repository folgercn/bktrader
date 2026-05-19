#!/usr/bin/env python3
"""
verify_determinism.py — 验证 pipeline 确定性

Task 8.2: 同一输入两次运行产出逐行一致的 trade ledger

验证策略：
1. 保存当前输出文件（来自 Task 8.1 运行）的副本
2. 重新运行完整 pipeline
3. 逐行对比关键输出文件
4. 报告差异（预期：除 generated_at/生成时间 外完全一致）

Usage:
    cd research/entry_redesign/scripts
    python -m pre_breakout_timing.verify_determinism
"""

from __future__ import annotations

import json
import shutil
import sys
from pathlib import Path

import pandas as pd

# ---------------------------------------------------------------------------
# Path setup
# ---------------------------------------------------------------------------

SCRIPTS_DIR = Path(__file__).resolve().parent.parent
if str(SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPTS_DIR))

OUTPUT_DIR = SCRIPTS_DIR / "output" / "pre_breakout_timing"
BACKUP_DIR = SCRIPTS_DIR / "output" / "pre_breakout_timing_run1_backup"

# Key files to compare (deterministic content)
CSV_FILES = [
    "pre_breakout_timing_trades.csv",
    "delay_pnl_matrix.csv",
    "pre_breakout_timing_attribution.csv",
    "feature_importance.csv",
    "label_distribution.csv",
]

JSON_FILE = "pre_breakout_timing_summary.json"
MD_FILES = [
    "pre_breakout_timing_report.md",
    "classifier_rules.md",
]


def backup_outputs() -> bool:
    """Save copies of current output files as 'run 1' baseline."""
    print("\n" + "=" * 70)
    print("Step 1: 备份当前输出文件（Run 1）")
    print("=" * 70)

    if not OUTPUT_DIR.exists():
        print("  ❌ Output directory does not exist. Run pipeline first (Task 8.1).")
        return False

    # Check that key files exist
    missing = []
    for f in CSV_FILES + [JSON_FILE] + MD_FILES:
        if not (OUTPUT_DIR / f).exists():
            missing.append(f)

    if missing:
        print(f"  ❌ Missing output files: {missing}")
        print("  Please run the full pipeline first (Task 8.1).")
        return False

    # Create backup directory
    if BACKUP_DIR.exists():
        shutil.rmtree(BACKUP_DIR)
    BACKUP_DIR.mkdir(parents=True)

    # Copy all output files
    for f in CSV_FILES + [JSON_FILE] + MD_FILES:
        src = OUTPUT_DIR / f
        dst = BACKUP_DIR / f
        shutil.copy2(src, dst)
        print(f"  ✓ Backed up: {f}")

    print(f"\n  Backup directory: {BACKUP_DIR}")
    return True


def run_pipeline():
    """Run the full pipeline (Run 2)."""
    print("\n" + "=" * 70)
    print("Step 2: 重新运行 Pipeline（Run 2）")
    print("=" * 70)

    from pre_breakout_timing.pre_breakout_timing_runner import main
    main()


def compare_csv(filename: str) -> tuple[bool, str]:
    """Compare a CSV file between run 1 (backup) and run 2 (current output).

    Returns (is_identical, message).
    """
    run1_path = BACKUP_DIR / filename
    run2_path = OUTPUT_DIR / filename

    if not run1_path.exists():
        return False, f"Run 1 file missing: {run1_path}"
    if not run2_path.exists():
        return False, f"Run 2 file missing: {run2_path}"

    df1 = pd.read_csv(run1_path)
    df2 = pd.read_csv(run2_path)

    # Check shape
    if df1.shape != df2.shape:
        return False, (
            f"Shape mismatch: Run1={df1.shape}, Run2={df2.shape}"
        )

    # Check columns
    if list(df1.columns) != list(df2.columns):
        return False, (
            f"Column mismatch:\n  Run1: {list(df1.columns)}\n  Run2: {list(df2.columns)}"
        )

    # Row-by-row comparison (handling NaN equality)
    # pandas treats NaN == NaN as False, so we use a special comparison
    differences = []
    for col in df1.columns:
        # Compare values, treating NaN == NaN as True
        col1 = df1[col]
        col2 = df2[col]

        # For numeric columns, use close comparison to handle floating point
        if col1.dtype in ("float64", "float32") or col2.dtype in ("float64", "float32"):
            # Both NaN → equal; one NaN → not equal; both numeric → isclose
            nan_mask1 = col1.isna()
            nan_mask2 = col2.isna()
            nan_mismatch = nan_mask1 != nan_mask2
            if nan_mismatch.any():
                diff_rows = nan_mismatch[nan_mismatch].index.tolist()
                differences.append(
                    f"  Column '{col}': NaN mismatch at rows {diff_rows[:5]}"
                )
                continue

            # Compare non-NaN values
            valid_mask = ~nan_mask1
            if valid_mask.any():
                vals1 = col1[valid_mask].values
                vals2 = col2[valid_mask].values
                import numpy as np
                if not np.allclose(vals1, vals2, rtol=1e-12, atol=1e-15, equal_nan=True):
                    # Find specific differences
                    close_mask = np.isclose(vals1, vals2, rtol=1e-12, atol=1e-15)
                    diff_indices = valid_mask[valid_mask].index[~close_mask].tolist()
                    if diff_indices:
                        sample = diff_indices[:3]
                        differences.append(
                            f"  Column '{col}': value mismatch at rows {sample} "
                            f"(e.g., {vals1[~close_mask][:3]} vs {vals2[~close_mask][:3]})"
                        )
        else:
            # String/object/int comparison
            # Fill NaN with sentinel for comparison
            s1 = col1.fillna("__NAN__").astype(str)
            s2 = col2.fillna("__NAN__").astype(str)
            mismatches = s1 != s2
            if mismatches.any():
                diff_rows = mismatches[mismatches].index.tolist()
                differences.append(
                    f"  Column '{col}': mismatch at rows {diff_rows[:5]} "
                    f"(e.g., '{s1.iloc[diff_rows[0]]}' vs '{s2.iloc[diff_rows[0]]}')"
                )

    if differences:
        return False, f"Content differences found:\n" + "\n".join(differences)

    return True, f"✓ Identical ({df1.shape[0]} rows × {df1.shape[1]} cols)"


def compare_json(filename: str) -> tuple[bool, str]:
    """Compare JSON file, ignoring 'generated_at' field."""
    run1_path = BACKUP_DIR / filename
    run2_path = OUTPUT_DIR / filename

    if not run1_path.exists():
        return False, f"Run 1 file missing: {run1_path}"
    if not run2_path.exists():
        return False, f"Run 2 file missing: {run2_path}"

    with open(run1_path) as f:
        data1 = json.load(f)
    with open(run2_path) as f:
        data2 = json.load(f)

    # Remove generated_at for comparison
    data1_clean = {k: v for k, v in data1.items() if k != "generated_at"}
    data2_clean = {k: v for k, v in data2.items() if k != "generated_at"}

    # Deep comparison with tolerance for floats
    differences = _deep_compare(data1_clean, data2_clean, path="")

    if differences:
        return False, f"Content differences (excluding generated_at):\n" + "\n".join(differences[:10])

    # Check generated_at exists (expected to differ)
    gen1 = data1.get("generated_at", "MISSING")
    gen2 = data2.get("generated_at", "MISSING")
    note = ""
    if gen1 != gen2:
        note = f" (generated_at differs as expected: '{gen1}' vs '{gen2}')"

    return True, f"✓ Identical (excluding generated_at){note}"


def _deep_compare(obj1, obj2, path: str, tol: float = 1e-12) -> list[str]:
    """Recursively compare two objects, returning list of differences."""
    diffs = []

    if type(obj1) != type(obj2):
        diffs.append(f"  {path}: type mismatch ({type(obj1).__name__} vs {type(obj2).__name__})")
        return diffs

    if isinstance(obj1, dict):
        keys1 = set(obj1.keys())
        keys2 = set(obj2.keys())
        if keys1 != keys2:
            extra1 = keys1 - keys2
            extra2 = keys2 - keys1
            if extra1:
                diffs.append(f"  {path}: extra keys in run1: {extra1}")
            if extra2:
                diffs.append(f"  {path}: extra keys in run2: {extra2}")
        for k in keys1 & keys2:
            diffs.extend(_deep_compare(obj1[k], obj2[k], f"{path}.{k}", tol))

    elif isinstance(obj1, list):
        if len(obj1) != len(obj2):
            diffs.append(f"  {path}: list length mismatch ({len(obj1)} vs {len(obj2)})")
        else:
            for i, (a, b) in enumerate(zip(obj1, obj2)):
                diffs.extend(_deep_compare(a, b, f"{path}[{i}]", tol))

    elif isinstance(obj1, float):
        if abs(obj1 - obj2) > tol and not (obj1 != obj1 and obj2 != obj2):  # NaN check
            diffs.append(f"  {path}: {obj1} vs {obj2} (diff={abs(obj1-obj2):.2e})")

    elif obj1 != obj2:
        diffs.append(f"  {path}: '{obj1}' vs '{obj2}'")

    return diffs


def compare_md(filename: str) -> tuple[bool, str]:
    """Compare markdown file, ignoring timestamp lines."""
    run1_path = BACKUP_DIR / filename
    run2_path = OUTPUT_DIR / filename

    if not run1_path.exists():
        return False, f"Run 1 file missing: {run1_path}"
    if not run2_path.exists():
        return False, f"Run 2 file missing: {run2_path}"

    with open(run1_path) as f:
        lines1 = f.readlines()
    with open(run2_path) as f:
        lines2 = f.readlines()

    # Filter out timestamp lines (lines containing "生成时间")
    def filter_timestamp(lines):
        return [l for l in lines if "生成时间" not in l]

    filtered1 = filter_timestamp(lines1)
    filtered2 = filter_timestamp(lines2)

    if len(filtered1) != len(filtered2):
        return False, (
            f"Line count mismatch (excluding timestamps): "
            f"Run1={len(filtered1)}, Run2={len(filtered2)}"
        )

    diff_lines = []
    for i, (l1, l2) in enumerate(zip(filtered1, filtered2)):
        if l1 != l2:
            diff_lines.append(f"  Line {i+1}: '{l1.strip()[:80]}' vs '{l2.strip()[:80]}'")

    if diff_lines:
        return False, f"Content differences (excluding timestamps):\n" + "\n".join(diff_lines[:10])

    # Count timestamp lines that differ
    ts_lines1 = [l for l in lines1 if "生成时间" in l]
    ts_lines2 = [l for l in lines2 if "生成时间" in l]
    ts_note = ""
    if ts_lines1 != ts_lines2:
        ts_note = f" (timestamp line differs as expected)"

    return True, f"✓ Identical ({len(filtered1)} lines, excluding timestamps){ts_note}"


def compare_all() -> bool:
    """Compare all output files between Run 1 and Run 2."""
    print("\n" + "=" * 70)
    print("Step 3: 对比 Run 1 vs Run 2 输出")
    print("=" * 70)

    all_pass = True

    # Compare CSV files
    print("\n  ── CSV 文件对比 ──")
    for f in CSV_FILES:
        ok, msg = compare_csv(f)
        status = "✓" if ok else "❌"
        print(f"\n  {status} {f}")
        print(f"    {msg}")
        if not ok:
            all_pass = False

    # Compare JSON file
    print("\n  ── JSON 文件对比 ──")
    ok, msg = compare_json(JSON_FILE)
    status = "✓" if ok else "❌"
    print(f"\n  {status} {JSON_FILE}")
    print(f"    {msg}")
    if not ok:
        all_pass = False

    # Compare MD files
    print("\n  ── Markdown 文件对比 ──")
    for f in MD_FILES:
        ok, msg = compare_md(f)
        status = "✓" if ok else "❌"
        print(f"\n  {status} {f}")
        print(f"    {msg}")
        if not ok:
            all_pass = False

    # Final verdict
    print("\n" + "=" * 70)
    if all_pass:
        print("✅ 确定性验证通过：两次运行产出逐行一致（除 generated_at 时间戳外）")
        print("   所有随机过程使用 random_state=42，pipeline 完全确定性。")
    else:
        print("❌ 确定性验证失败：两次运行产出存在差异")
        print("   请检查是否有未 seed 的随机源或非确定性系统调用。")
    print("=" * 70)

    return all_pass


def main():
    """完整确定性验证流程。"""
    print("=" * 70)
    print("Pre-Breakout Timing Classifier — 确定性验证 (Task 8.2)")
    print("=" * 70)
    print("\n验证策略：")
    print("  1. 备份当前输出文件（来自 Task 8.1 的 Run 1）")
    print("  2. 重新运行完整 pipeline（Run 2）")
    print("  3. 逐行对比关键输出文件")
    print("  4. 预期：除 generated_at/生成时间 外完全一致")

    # Step 1: Backup
    if not backup_outputs():
        sys.exit(1)

    # Step 2: Re-run pipeline
    run_pipeline()

    # Step 3: Compare
    passed = compare_all()

    # Cleanup backup
    if passed and BACKUP_DIR.exists():
        print(f"\n  Cleaning up backup directory: {BACKUP_DIR}")
        shutil.rmtree(BACKUP_DIR)

    sys.exit(0 if passed else 1)


if __name__ == "__main__":
    main()
