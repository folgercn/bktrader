# Order Intent 判定矩阵

> **唯一事实源**：`internal/domain/order_intent.go` → `ClassifyOrderIntent()`
>
> 所有需要判断"一笔订单是开多还是平空"的地方，都必须消费此分类结果。
> 禁止在 service/、http/、前端 中自行组合 side + reduceOnly 推断。

## 当前模式：One-way Mode（positionSide = BOTH）

| Side | ReduceOnly | ClosePosition | → OrderIntent | 中文标签 |
|------|-----------|--------------|---------------|---------|
| BUY  | false     | false        | `OPEN_LONG`   | 开多     |
| SELL | false     | false        | `OPEN_SHORT`  | 开空     |
| SELL | true      | false        | `CLOSE_LONG`  | 平多     |
| BUY  | true      | false        | `CLOSE_SHORT` | 平空     |
| SELL | false     | true         | `CLOSE_LONG`  | 平多     |
| BUY  | false     | true         | `CLOSE_SHORT` | 平空     |
| ""   | *         | *            | `UNKNOWN`     | 未知     |

### 判定优先级

1. `EffectiveReduceOnly()` = `Order.ReduceOnly || Metadata["reduceOnly"]`
2. `EffectiveClosePosition()` = `Order.ClosePosition || Metadata["closePosition"]`
3. 只要 `isExit = EffectiveReduceOnly() || EffectiveClosePosition()` 为 true，即视为平仓

### 重要说明

- **intent 与 Status 无关**：即使订单 Status = CANCELLED，intent 仍然可分类（用于回归和展示）
- **intent 不参与结算**：结算和持仓更新严格基于交易所返回的 fills 和 status 事实
- **intent 只用于**：展示、回归验证、审计追溯

## 未来扩展：Hedge Mode（positionSide = LONG/SHORT）

如果切换至 Hedge Mode，需在 `ClassifyOrderIntent()` 中额外引入 `positionSide` 参数：

| Side | PositionSide | → OrderIntent |
|------|-------------|---------------|
| BUY  | LONG        | `OPEN_LONG`   |
| BUY  | SHORT       | `CLOSE_SHORT` |
| SELL | SHORT       | `OPEN_SHORT`  |
| SELL | LONG        | `CLOSE_LONG`  |

> ⚠️ 切换前必须更新此矩阵、`ClassifyOrderIntent()` 实现和 Golden Case 测试。
