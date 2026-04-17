# Session 停用保护与手动平仓后端接口规划

## 背景
用户反馈，当某个 `SignalRuntimeSession` 或 `LiveSession` 还在运行且带有关联此会话的未平仓头寸或活跃挂单时，如果被停用（Stop）或删除（Delete），会导致该头寸彻底失去被接管的机会并沦为“幽灵仓”。
为解决此问题，后端接口需要增加持仓拦截判断。同时，在此契机下一并增加明确的、便于前端操作的订单全生命周期接管接口（例如单独点击某持仓直接一键强平）。

## 规划细节

### 1. 生命周期接口的安全锁改造
我们将在会话删除和停用的主要入口挂载一个统一的前置预检函数，用于扫描并阻断破坏性的关闭动作。

#### [NEW] internal/service/safety_checks.go (或在 `live.go` / `service.go` 中)
- **增加核心检验方法**: `HasActivePositionsOrOrders(accountID, strategyID string) bool`
  1. 向DB轮询 `ListPositions`: 找到 `Quantity > 0` 且归属当前 Account/Strategy的未平仓头寸。
  2. 向DB轮询 `ListOrders`: 找到状态处于 `NEW`/`PARTIALLY_FILLED`/`ACCEPTED` 且归属当前的订单。

#### [MODIFY] internal/service/live.go
- **`StopLiveSession` 与 `DeleteLiveSession` 接口**： 
  在最初步增加上述预检逻辑。如果存在活跃订单，立即返回 `fmt.Errorf("存在活动中的订单或未平仓头寸，无法直接停用/删除")` 以阻断请求。

#### [MODIFY] internal/service/signal_runtime_sessions.go
- **`StopSignalRuntimeSession` 与 `DeleteSignalRuntimeSession` 接口**： 
  因为 `SignalRuntime` 统带下属 LiveSessions，因此查出属于该 Runtime 的 `LiveSession` 对应的 Account + Strategy 并抛给 `HasActivePositionsOrOrders`，有挂载持仓同样退回阻断。

> [!WARNING] 
> **硬退出后门支持 (Force Parameter)** 
> 在上述涉及到停用或删除的所有 HTTP 接口方法签名中提供对查询参数的检测。如果 HTTP 请求的 URL 中带有 `?force=true`，则完全跳过上述 `HasActivePositionsOrOrders` 拦截器。此改动兼容由于断网或其他脏数据导致的极端情况下管理员强行清理节点的需求。

---

### 2. 订单管理与手动平仓管理接管
后端平台当前原生支持通过 `POST /api/v1/orders` 手动下发市价单以完成平仓能力，但这极大要求前端完全拼凑好反身逻辑。此时我们将新增单独一键冲销平仓的语义化接管 API。

#### [MODIFY] internal/service/order.go
- **新增平仓业务方法**: `ClosePosition(positionID string) (domain.Order, error)`
  根据 Position ID 获取仓位后，向内部引擎（`p.CreateOrder`）反身提交一笔：方向相反、数量等同 `pos.Quantity`、`Type: MARKET`、带有 `reduceOnly: true` (只减仓) 属性的全新订单。
  此逻辑会让这笔接管单极其自然顺滑地汇聚到底层执行引擎与交易所，引发真正的清算且账册分毫不差。

#### [MODIFY] internal/http/accounts.go (或 orders/positions路由)
- 暴露一条新的前端友好的平仓提交点：**`POST /api/v1/positions/{id}/close`**，这会调用上述 `ClosePosition` 并将创建的平仓单结构体丢还给前端。

#### [MODIFY] internal/http/orders.go
- 新增查询指定订单明细路由：**`GET /api/v1/orders/{id}`**，方便前端页面或用户手动追踪刚刚发出的强平单或任何挂单。

这套体系能够让系统的容错性变强，在操作层规避用户断头操作。随时可以开展编写任务。
