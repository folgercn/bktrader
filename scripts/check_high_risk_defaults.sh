#!/usr/bin/env bash
set -e

echo "Running high risk defaults check..."

# 我们要找的危险特征：直接在业务逻辑甚至工厂代码里写死 dispatchMode 为 auto-dispatch
# grep 返回 0 代表找到，1 代表没找到
if grep -rnw 'internal/' -e '"auto-dispatch"' | grep -v '_test\.go'; then
  echo ""
  echo "🚨 [CRITICAL RISK DETECTED] 🚨"
  echo "Found hardcoded 'auto-dispatch' string in internal package source code."
  echo "The default dispatchMode MUST ALWAYS be initialized as 'manual-review'."
  echo "Do not commit this change unless you have explicit double approval."
  exit 1
fi

echo "✅ High risk defaults check passed."
