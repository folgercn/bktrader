#!/usr/bin/env python3
import json
import os
import signal
import subprocess
import sys
import time
import urllib.request

import pandas as pd

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
API_BASE = "http://127.0.0.1:8080"
if ROOT not in sys.path:
    sys.path.insert(0, ROOT)

from research.backTest import run_backtest_1min_granularity


def wait_for_healthz(timeout=20):
    deadline = time.time() + timeout
    while time.time() < deadline:
        try:
            with urllib.request.urlopen(f"{API_BASE}/healthz", timeout=1) as resp:
                if resp.status == 200:
                    return True
        except Exception:
            time.sleep(0.5)
    return False


def start_server():
    proc = subprocess.Popen(
        ["go", "run", "./cmd/platform-api"],
        cwd=ROOT,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
        preexec_fn=os.setsid,
    )
    if not wait_for_healthz():
        os.killpg(os.getpgid(proc.pid), signal.SIGTERM)
        raise RuntimeError("platform-api did not become healthy in time")
    return proc


def stop_server(proc):
    try:
        os.killpg(os.getpgid(proc.pid), signal.SIGTERM)
    except ProcessLookupError:
        pass


def run_python_reference(start_date, end_date):
    df_1min = pd.read_csv(os.path.join(ROOT, "BTC_1min_Clean.csv"), index_col=0, parse_dates=True)
    df_1min = df_1min.loc[start_date:end_date]
    df_4h = pd.read_csv(os.path.join(ROOT, "BTC_4H_Signals.csv"), index_col=0, parse_dates=True)
    df_4h = df_4h.loc[start_date:end_date]
    ledger = run_backtest_1min_granularity(
        df_1min,
        df_4h,
        initial_balance=100000.0,
        dir1_reentry_confirm=False,
        dir2_zero_initial=True,
        fixed_slippage=0.0005,
        stop_loss_atr=0.05,
        max_trades_per_bar=3,
        reentry_size_schedule=[0.10, 0.20],
        stop_mode="atr",
        profit_protect_atr=1.0,
    )
    if ledger.empty:
        return {
            "return": 0.0,
            "maxDrawdown": 0.0,
            "tradePairs": 0,
            "finalBalance": 100000.0,
        }

    final_balance = float(ledger.iloc[-1]["bal"])
    ledger["cum_max"] = ledger["bal"].cummax()
    ledger["drawdown"] = ledger["bal"] / ledger["cum_max"] - 1
    return {
        "return": final_balance / 100000.0 - 1,
        "maxDrawdown": float(ledger["drawdown"].min()),
        "tradePairs": int((ledger["type"] == "EXIT").sum()),
        "finalBalance": final_balance,
    }


def run_go_replay(start_ts, end_ts):
    payload = {
        "strategyVersionId": "strategy-version-bk-4h",
        "parameters": {
            "signalTimeframe": "4h",
            "executionDataSource": "1min",
            "symbol": "BTCUSDT",
            "from": start_ts,
            "to": end_ts,
        },
    }
    request = urllib.request.Request(
        f"{API_BASE}/api/v1/backtests",
        data=json.dumps(payload).encode("utf-8"),
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    with urllib.request.urlopen(request, timeout=60) as resp:
        data = json.loads(resp.read().decode("utf-8"))
    return data["resultSummary"]


def main():
    start_date = "2020-01-01"
    end_date = "2020-02-01"
    # pandas 的日期切片会包含 end_date 当天的全部分钟，所以 API 侧也显式跑到当天最后一分钟。
    start_ts = "2020-01-01T00:00:00Z"
    end_ts = "2020-02-01T23:59:00Z"

    proc = start_server()
    try:
        py_result = run_python_reference(start_date, end_date)
        go_result = run_go_replay(start_ts, end_ts)
    finally:
        stop_server(proc)

    comparison = {
        "python": py_result,
        "go": {
            "return": go_result.get("return", 0),
            "maxDrawdown": go_result.get("maxDrawdown", 0),
            "tradePairs": go_result.get("tradePairs", 0),
            "finalBalance": go_result.get("finalBalance", 0),
        },
    }
    print(json.dumps(comparison, indent=2, ensure_ascii=False))

    mismatches = []
    for key in ("return", "maxDrawdown", "finalBalance"):
        if abs(float(py_result[key]) - float(comparison["go"][key])) > 1e-9:
            mismatches.append(key)
    if py_result["tradePairs"] != comparison["go"]["tradePairs"]:
        mismatches.append("tradePairs")

    if mismatches:
        print(f"parity check failed: {', '.join(mismatches)}", file=sys.stderr)
        sys.exit(1)

    print("parity check passed")


if __name__ == "__main__":
    main()
