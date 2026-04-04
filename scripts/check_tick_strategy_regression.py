#!/usr/bin/env python3
import json
import math
import os
import signal
import subprocess
import sys
import time
import urllib.request

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
API_BASE = "http://127.0.0.1:8080"


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
    env = os.environ.copy()
    env.setdefault("TICK_DATA_DIR", "./dataset/archive")
    proc = subprocess.Popen(
        ["go", "run", "./cmd/platform-api"],
        cwd=ROOT,
        env=env,
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


def run_replay(signal_timeframe, start_ts, end_ts):
    payload = {
        "strategyVersionId": f"strategy-version-{signal_timeframe}-tick",
        "parameters": {
            "signalTimeframe": signal_timeframe,
            "executionDataSource": "tick",
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
    with urllib.request.urlopen(request, timeout=120) as resp:
        data = json.loads(resp.read().decode("utf-8"))
    return data["resultSummary"]


def assert_close(actual, expected, key, tolerance=1e-12):
    if math.isnan(actual) or abs(actual - expected) > tolerance:
        raise AssertionError(f"{key} mismatch: expected {expected}, got {actual}")


def main():
    expected = [
        {
            "label": "4h-tick-2020-01-04",
            "signalTimeframe": "4h",
            "from": "2020-01-04T00:00:00Z",
            "to": "2020-01-04T23:59:59Z",
            "tradePairs": 2,
            "finalBalance": 99968.15537868341,
            "return": -0.0003184462131659016,
            "maxDrawdown": -0.0003184462131659016,
        },
        {
            "label": "1d-tick-2020-01-26_to_2020-02-03",
            "signalTimeframe": "1d",
            "from": "2020-01-26T00:00:00Z",
            "to": "2020-02-03T23:59:59Z",
            "tradePairs": 5,
            "finalBalance": 99869.10724907617,
            "return": -0.001308927509238278,
            "maxDrawdown": -0.001308927509238278,
        },
    ]

    proc = start_server()
    try:
        results = []
        for case in expected:
            summary = run_replay(case["signalTimeframe"], case["from"], case["to"])
            result = {
                "label": case["label"],
                "tradePairs": summary.get("tradePairs", 0),
                "finalBalance": summary.get("finalBalance", 0.0),
                "return": summary.get("return", 0.0),
                "maxDrawdown": summary.get("maxDrawdown", 0.0),
            }
            results.append(result)

            if result["tradePairs"] != case["tradePairs"]:
                raise AssertionError(
                    f"{case['label']} tradePairs mismatch: expected {case['tradePairs']}, got {result['tradePairs']}"
                )
            assert_close(float(result["finalBalance"]), float(case["finalBalance"]), f"{case['label']} finalBalance")
            assert_close(float(result["return"]), float(case["return"]), f"{case['label']} return")
            assert_close(float(result["maxDrawdown"]), float(case["maxDrawdown"]), f"{case['label']} maxDrawdown")
    finally:
        stop_server(proc)

    print(json.dumps(results, indent=2, ensure_ascii=False))
    print("tick strategy regression passed")


if __name__ == "__main__":
    main()
