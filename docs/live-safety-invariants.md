# 实盘稳定性防线与关键不变量 (Live Safety Invariants)

在系统最底层的执行环节（主要涉及 `Live Session` 及其恢复、交易订单的管理），我们需要维护以下**不变量**。无论你在进行什么重构或需求调整，如果破坏了这些规则，系统就会触发 L3 级别高风险事件。

## 1. 默认行为防线
- `dispatchMode` 的默认必须为 `manual-review`：即便是一个完全成熟的量化系统，也必须能依靠人工手动派单兜底，禁止在代码层强绑定。
- **Mock -> Real 的环境切换**：当采用 Testnet/Mock 环境执行时，绝不可含有导致 `paper` 发向 `mainnet` 真实链路的意外逃逸代码。

## 2. Session 幂等与一致性
- **幂等启动与恢复**：当进程挂掉重启，或是 Session 进入 recovery (例如 `live_recovery.go`) 时，必须表现出绝对的幂等性。它不能因为重启而导致双重下单 (double-spend)。
- **Order-Fill-Position 三角**：任何执行状态的变化，都必须保持这三个维度的强一致。Fill 数据必须能精确推演到当前的 Position 变化，Position 不能出现“账外更新”却找不到 Fill。

## 3. Order Exit 条件与保护
- 任何情况下，PT (Profit Taking) Exit 都需要考虑 postOnly / reduceOnly 的限价单防呆设计。
- SL (Stop Loss) Exit 必须能以市价 (MARKET order) 无条件清仓，防止跌破流动性区间时系统陷入阻塞等待。

## 4. 业务分离 (Separation of Concerns)
- 引擎端 (`service`) 与 展现端 (`http/api`) 间绝不串越状态验证逻辑。执行策略的触发一定经过统一的 Dispatch 管理，而不能是在前端调用一个接口去绕道“私开仓位”。
