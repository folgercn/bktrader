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

# 检查是否在业务代码中硬编码了 mainnet 路由地址
# 排除测试文件；排除注释行（grep 输出格式为 path:line:content，
# 需要用 awk 提取 content 部分再判断是否以 // 开头）
if grep -rnw 'internal/' -e '"mainnet"' | grep -v '_test\.go' | awk -F: '{content=$0; sub(/^[^:]*:[^:]*:/, "", content); if (content !~ /^[[:space:]]*\/\//) print}' | grep -q .; then
  echo ""
  echo "🚨 [CRITICAL RISK DETECTED] 🚨"
  echo "Found hardcoded 'mainnet' string in internal package source code."
  echo "Production network references must come from environment config, not hardcoded."
  echo "Do not commit this change unless you have explicit double approval."
  exit 1
fi

echo "✅ High risk defaults check passed."
