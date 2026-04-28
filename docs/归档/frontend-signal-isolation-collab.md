# 前端协作：Signal Symbol 隔离

> **上下文**：后端已完成信号源稳定性修复 + 跨 Symbol 信号隔离。前端需要同步更新 derivation 函数以完成端到端隔离。
>
> **PR**: `fix/signal-runtime-stability-and-symbol-isolation`

## 背景

### 问题

当一个 signal runtime session 同时订阅了多个 symbol（如 BTCUSDT + ETHUSDT）的数据流时，前端的 derivation 函数从 `sourceStates` 中遍历所有 entries **不区分 symbol**，导致：
- `MonitorStage` 中展示的价格/K线可能是多个 symbol 混在一起
- `AccountStage` 中的 runtime panel 同理

### 后端已完成的工作

后端已在 `evaluateLiveSessionOnSignal` 中通过 `filterSourceStatesBySymbol` / `filterSignalBarStatesBySymbol` 对传递给策略评估的数据做了 symbol 过滤。但前端直接从 `runtimeSession.State.sourceStates` 读取数据展示在 UI 上，这条路径还需要前端侧做过滤。

---

## 需要修改的文件

### 1. `web/console/src/utils/derivation.ts`

#### `deriveRuntimeMarketSnapshot` (约 L388)

新增可选参数 `targetSymbol?: string`，对 sourceStates 做 symbol 过滤：

```typescript
// 改造前:
export function deriveRuntimeMarketSnapshot(
  sourceStates: Record<string, unknown>,
  summary: Record<string, unknown>
): RuntimeMarketSnapshot {
  const states = Object.values(sourceStates).map((value) => getRecord(value));
  // ...
}

// 改造后:
export function deriveRuntimeMarketSnapshot(
  sourceStates: Record<string, unknown>,
  summary: Record<string, unknown>,
  targetSymbol?: string
): RuntimeMarketSnapshot {
  const normalizedTarget = (targetSymbol ?? "").trim().toUpperCase();
  const states = Object.values(sourceStates)
    .map((value) => getRecord(value))
    .filter((state) => {
      if (!normalizedTarget) return true;
      const stateSymbol = String(state.symbol ?? "").trim().toUpperCase();
      return stateSymbol === "" || stateSymbol === normalizedTarget;
    });
  // ... 其余逻辑不变
}
```

#### `deriveRuntimeSourceSummary` (约 L418)

同样新增 `targetSymbol?: string` 参数，过滤逻辑同上。

#### `deriveSignalBarCandles` (约 L486)

同样新增 `targetSymbol?: string` 参数，在遍历 bar entries 时过滤：

```typescript
export function deriveSignalBarCandles(
  sourceStates: Record<string, unknown>,
  targetSymbol?: string
): SignalBarCandle[] {
  const normalizedTarget = (targetSymbol ?? "").trim().toUpperCase();
  // 在遍历 bars 时增加:
  // const barSymbol = String(bar.symbol ?? "").trim().toUpperCase();
  // if (normalizedTarget && barSymbol && barSymbol !== normalizedTarget) continue;
}
```

### 2. `web/console/src/pages/MonitorStage.tsx`

在调用 derivation 函数处传入当前 session 的 symbol：

```typescript
const sessionSymbol = String(
  monitorSession?.state?.symbol ?? ""
).trim().toUpperCase();

// 调用处追加参数
const marketSnapshot = deriveRuntimeMarketSnapshot(sourceStates, lastEventSummary, sessionSymbol);
const sourceSummary = deriveRuntimeSourceSummary(sourceStates, runtimePolicy, sessionSymbol);
const signalBarCandles = deriveSignalBarCandles(sourceStates, sessionSymbol);
```

### 3. `web/console/src/pages/AccountStage.tsx`

同 MonitorStage，在 runtime panel 的 derivation 调用处追加 `sessionSymbol` 参数。

---

## 验证方法

1. 同时运行 BTCUSDT 和 ETHUSDT live session
2. 在 MonitorStage 切换查看不同 session
3. 确认每个 session 只展示对应 symbol 的价格和 K 线数据
4. 确认 `deriveRuntimeMarketSnapshot` 返回的价格只来自目标 symbol

## TypeScript 验证

```bash
cd web/console
./node_modules/.bin/tsc --noEmit --jsx react-jsx --esModuleInterop --skipLibCheck \
  --target esnext --moduleResolution node --allowSyntheticDefaultImports
```

---

## 后端新增的 health 状态（前端可选展示）

后端新增了以下 health 状态值，前端可在 runtime status badge 中展示：

| health 值 | 含义 | 建议样式 |
|-----------|------|---------|
| `recovering` | WS 断连后正在自动重连 | 黄色闪烁 |
| `stale-after-reconnect` | 重连成功但可能漏掉了 K 线收盘 | 红色 |

相关 state 字段：
- `state.reconnectAttempt` / `state.reconnectMaxAttempts` — 当前重连进度
- `state.lastDisconnectError` — 上次断连原因
- `state.signalBarContinuityWarning` — K 线连续性检查结果
