#!/usr/bin/env bash
set -e

echo "Running env safety check..."

# 这里拦截如果公用的模板 `configs/*.env` 被人顺手写成了 real 模式
# 在本地调试可以自己改 .env，但是不能合入 github 的模板，以免 CI 误跑实盘
if grep -rnE "^BINANCE_FUTURES_EXECUTION_MODE=real" configs/ 2>/dev/null; then
  echo ""
  echo "🚨 [CRITICAL RISK DETECTED] 🚨"
  echo "Found BINANCE_FUTURES_EXECUTION_MODE=real in config templates."
  echo "Config templates in 'configs/' must remain set to 'mock' or 'rest'."
  exit 1
fi

# 检查是否把本地含有敏感信息的 .env.production 或真实的 .env 给推送到 remote 了
if git ls-files | grep -E "^\.env(\.production)?$"; then
  echo ""
  echo "🚨 [CRITICAL RISK DETECTED] 🚨"
  echo "A production .env file is tracked by git!"
  echo "Please remove it from git tracking immediately to prevent secret leakage."
  exit 1
fi

echo "✅ Environment safety check passed."
