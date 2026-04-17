# 后端需求规范：交易对账与订单幂等性同步 (PRD)

## 1. 目标
确保 BKTrader 系统与交易所（Binance 等）之间的订单状态与成流量保持强一致性，消除人工点击“Sync”按钮可能导致的成交记录重复、持仓虚增等风险，并具备自动发现“孤儿订单”的能力。

## 2. 核心功能需求

### 2.1 成交流水（Fills）的幂等性校验
- **问题**：目前 Sync 接口会重复追加成交记录，导致本地持仓（Position）随同步次数呈倍数虚增。
- **需求**：
    - 在 `domain.Fill` 中增加 `ExchangeTradeID` (string) 字段。
    - 在写入成交记录前，系统必须根据 `(OrderID, ExchangeTradeID)` 进行查重。
    - **禁止重复处理**：如果成交 ID 已存在，跳过该记录，且**不得**再次调用 `applyExecutionFill` 更新持仓。

### 2.2 孤儿订单（Orphan Orders）自动发现
- **问题**：如果系统崩溃或网络严重闪断，可能导致某些已在交易所成交的订单在本地数据库完全缺失记录。
- **需求**：
    - 提供一个“账户全量对账”接口（如 `POST /api/v1/accounts/:id/reconcile`）。
    - 逻辑：拉取交易所最近 N 小时的所有订单历史，与本地数据库比对。
    - **补全逻辑**：发现交易所存在但本地缺失的订单，自动在本地 `orders` 表中补全（Upsert），并同步其成交明细。

### 2.3 订单生命周期状态位的严格终结
- **需求**：
    - 订单状态（Status）必须在同步确认后，从 `ACCEPTED` 强制迁移至 `FILLED` 或 `CANCELLED`。
    - 必须正确设置元数据中的同步标记：`metadata.orderLifecycle.synced = true`。
    - 只有当 `synced = false` 且状态为非终结态（如 `ACCEPTED`）时，才允许在前端“待同步列表”中展示。

## 3. 技术实施建议（供后端参考）

### 3.1 数据库方案 (Postgres)
- 为 `fills` 表增加 `exchange_trade_id` 字段。
- 建立唯一索引：`CREATE UNIQUE INDEX idx_fills_order_trade_id ON fills (order_id, exchange_trade_id);`

### 3.2 业务逻辑 (Service 层)
- 在 `finalizeExecutedOrder` 逻辑内部引入 `FilterExistingFills` 辅助函数。
- 只有通过过滤的新成交，才允许引发 Position 的增量更新。

## 4. 验收标准
1. **防虚增测试**：对同一笔 FILLED 订单连续点击 10 次 Sync，本地成交明细行数不变，持仓数量（Quantity）不发生任何变动。
2. **状态自动消除**：点击同步后，订单状态若已达终结态，会自动从前端“待同步订单”列表中消失。
3. **数据完整性**：所有通过 Sync 接口拉回的成交，必须带有交易所原始的 `tradeId`。
