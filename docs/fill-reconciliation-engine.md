# Fill Reconciliation Engine

## 目标

成交一致性引擎用于把交易所权威成交明细和本地 `fills` 表收敛到同一套规则。它解决的问题来自 PR #271：订单已经 `FILLED`，但 Binance 成交明细延迟返回，系统先用 synthetic fill 更新仓位，后续真实成交到达时再替换 synthetic fill，并且不能重复更新 position。

本引擎的边界是纯计算：输入订单、本地已存在 fills、交易所新到 fills，输出删除、创建、position 增量和订单 metadata 更新计划。引擎不直接写数据库，不直接更新 position，也不直接调用交易所。

## 术语

| 术语 | 含义 |
| --- | --- |
| real fill | 交易所权威成交，有稳定 `exchange_trade_id`。手续费、realizedPnL 等权威值应来自它。 |
| synthetic fill | 本地兜底成交。用于订单已终态但交易所成交明细暂时为空时维持 filledQuantity 和 position 一致。 |
| remainder fill | synthetic 被部分 real 替换后剩余的占位数量。它代表已经应用到 position、但还没有真实成交明细覆盖的数量。 |
| appliedQty | 已经更新过 position 的数量。real 替换 synthetic/remainder 时，这部分不能再次应用。 |
| filledQuantity | 本地订单视角的成交总量，必须与 real + synthetic/remainder 的合计保持一致。 |

当前版本不新增 DB 字段，但 plan builder 不再自行猜 source。调用方必须把每条 fill 标准化为显式 `FillReconciliationInput{Fill, Source}`：

- `real` 必须带稳定 `exchange_trade_id`；
- `synthetic` 必须带 `dedup_fallback_fingerprint`；
- `remainder` 必须带 `synthetic-remainder|` 前缀；
- source 缺失或字段与 source 不匹配时直接返回 error。

这样后续执行删除计划时，只会删除被上游明确标记为 `synthetic` 或 `remainder` 的本地 fill，避免依赖间接条件误删合法 fallback fill。

当前已经增加 `fill_source` 字段，取值为 `real / synthetic / remainder / paper / manual`。`domain.Fill.Source` 是内部 reconciliation source，不通过 fill JSON 对外输出。

## 一致性规则

1. real fill 是权威来源，一旦到达就应替换 synthetic/remainder。
2. 已由 synthetic/remainder 应用到 position 的数量，后续 real fill 到达时不能重复应用。
3. real 数量小于原 synthetic/remainder 数量时，补 remainder，保证本地 filledQuantity 不倒退。
4. real 分批到达时逐步缩小 remainder，直到完全真实化。
5. real 数量超过已应用 synthetic/remainder 时，只把超出的数量作为 position 增量。
6. 完全没有 real fill 时，synthetic fallback 必须由上游 retry policy 决定；引擎只处理已经进入输入的 fill。

## 标准化输入方向

后续 adapter 应先把交易所成交归一化为统一结构，再交给引擎。字段映射示例：

| 统一字段 | Binance | OKX | Bybit |
| --- | --- | --- | --- |
| `ExchangeOrderID` | `orderId` | `ordId` | `orderId` |
| `ExchangeTradeID` | `id` / `tradeId` | `tradeId` | `execId` |
| `Price` | `price` | `fillPx` | `execPrice` |
| `Quantity` | `qty` | `fillSz` | `execQty` |
| `Fee` | `commission` | `fillFee` | `execFee` |
| `FeeAsset` | `commissionAsset` | fee currency | fee currency |
| `RealizedPnL` | `realizedPnl` | `fillPnl` | `closedPnl` |
| `TradeTime` | `time` | `fillTime` | `execTime` |

## 当前实现

当前已经完成以下收口：

- `BuildFillReconciliationPlan` 读取 `domain.Order`、existing fills 和 incoming fills；
- existing/incoming fills 必须带显式 source；
- 输出 `DeleteFillIDs`、`CreateFills`、`ApplyPositionFills`、`UpdatedMetadata`、`Warnings`；
- `remainingQuantity` 统一 clamp 到非负值，超额成交通过 `Warnings` 暴露；
- `finalizeExecutedOrder` 已按 plan 执行删除、创建、position 增量应用；
- fill/order/position settlement 已进入同一事务边界；
- 同一订单 settlement 在 Postgres 中通过 order row lock 串行化；
- `fills.fill_source` 已持久化，memory/postgres store 均读写 source；
- Binance user trades 已映射到 `ExchangeFillReport`；
- OKX / Bybit 成交 payload 已有统一 mapper 和测试，后续 live adapter 只需要调用 mapper；
- 不改变 Binance、testnet、mainnet 或 `dispatchMode` 默认行为。

后续阶段再把 OKX / Bybit live adapter 接入实际 REST/WS 成交流，并复用当前 `ExchangeFillReport` mapper，不在 adapter 内重写 synthetic upgrade 算法。
