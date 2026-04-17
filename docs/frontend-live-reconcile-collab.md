# 前端协作：Live Account Reconcile

> **上下文**：后端已补上 live 账户级对账入口，用于发现并补全交易所已存在、但本地缺失的订单/成交。
>
> **本轮约束**：当前 PR 不改前端页面，只提供后端 API 和协作文档，前端后续按需接入。

## 目标

前端后续需要给 LIVE 账户提供一个“全量对账”入口，用于处理以下场景：

- 平台进程异常退出后，本地缺失某笔交易所订单
- 手动 `Sync` 订单无法覆盖“本地根本没有这笔订单”的情况
- 需要一次性扫最近一段时间的交易所订单/成交，并回填本地 `orders` / `fills`

## 本轮后端已完成

### 1. 新接口

`POST /api/v1/live/accounts/:id/reconcile`

可选请求体：

```json
{
  "lookbackHours": 24
}
```

说明：

- 默认回扫最近 `24` 小时
- 当前仅支持 `executionMode=rest` 的 LIVE 账户
- 当前实现会先做一次账户同步，再按候选 symbol 拉 Binance 历史订单与成交

### 2. 返回结构

接口返回 `LiveAccountReconcileResult`：

```json
{
  "account": { "...": "最新账户对象" },
  "adapterKey": "binance-futures",
  "executionMode": "rest",
  "lookbackHours": 24,
  "symbolCount": 2,
  "symbols": ["BTCUSDT", "ETHUSDT"],
  "orderCount": 3,
  "createdOrderCount": 1,
  "updatedOrderCount": 2,
  "notes": []
}
```

### 3. 账户 metadata 新增摘要

对账成功后，账户对象会额外带：

- `metadata.lastLiveReconcileAt`
- `metadata.lastLiveReconcile`

其中 `lastLiveReconcile` 包含：

- `adapterKey`
- `executionMode`
- `lookbackHours`
- `symbolCount`
- `symbols`
- `orderCount`
- `createdOrderCount`
- `updatedOrderCount`
- `notes`

## 前端建议接法

### 1. 入口位置

建议放在 `AccountStage` 的 LIVE 账户操作区，和“同步账户”并列，但文案明确区分：

- `同步账户`
- `全量对账`

不要复用“同步账户”按钮，以免用户误解成普通快照刷新。

### 2. 按钮展示条件

建议仅在以下条件显示或启用：

- `account.mode === "LIVE"`
- `account.metadata.liveBinding.executionMode === "rest"`

对于 `mock` 或未绑定账户，建议隐藏或置灰，并给出说明：

- `仅 REST 实盘账户支持全量对账`

### 3. 成功反馈

建议 toast 文案包含摘要数字，例如：

`对账完成：扫描 2 个 symbol，补回 1 笔缺失订单，更新 2 笔已有订单`

### 4. 详情展示

如后续要在账户详情页补展示，可直接读取：

- `account.metadata.lastLiveReconcileAt`
- `account.metadata.lastLiveReconcile.createdOrderCount`
- `account.metadata.lastLiveReconcile.updatedOrderCount`

## 已知边界

- 当前候选 symbol 来自本地订单、当前持仓、live session symbol、以及 `liveSyncSnapshot.openOrders/positions`
- 因 Binance Futures 历史订单接口按 `symbol` 查询，当前实现仍是“按候选 symbol 扫描”，不是无限制的全账户全市场回溯
- 当前前端“待同步订单”页卡还未消费 `lastLiveReconcile` 摘要；若后续需要，可结合该摘要做更明确提示

## 前端验证建议

1. 对 REST LIVE 账户触发一次 reconcile
2. 确认接口成功后 dashboard 会刷新
3. 确认账户 metadata 中出现 `lastLiveReconcileAt`
4. 确认若后端补回缺失订单，本地订单/成交列表会在刷新后出现
