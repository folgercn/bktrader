# SignalKind 语义契约

> 每种 signalKind 允许产生的 OrderIntent 范围。
> 新增 signalKind 时必须同步更新此表和 Golden Case 测试。

## 语义契约表

| signalKind              | 允许的 Intent                    | 说明           |
|-------------------------|--------------------------------|----------------|
| `initial`               | `OPEN_LONG`, `OPEN_SHORT`      | 初始建仓       |
| `initial-entry`         | `OPEN_LONG`, `OPEN_SHORT`      | 初始建仓事件别名 |
| `entry`                 | `OPEN_LONG`, `OPEN_SHORT`      | 通用入场事件别名 |
| `zero-initial-reentry`  | `OPEN_LONG`, `OPEN_SHORT`      | 零仓再入场     |
| `sl-reentry`            | `OPEN_LONG`, `OPEN_SHORT`      | 止损后再入场   |
| `pt-reentry`            | `OPEN_LONG`, `OPEN_SHORT`      | 止盈后再入场   |
| `risk-exit`             | `CLOSE_LONG`, `CLOSE_SHORT`    | 风险退出       |
| `sl`                    | `CLOSE_LONG`, `CLOSE_SHORT`    | 止损           |
| `pt`                    | `CLOSE_LONG`, `CLOSE_SHORT`    | 止盈           |
| `protect-exit`          | `CLOSE_LONG`, `CLOSE_SHORT`    | 保护性退出     |
| `recovery-watchdog`     | `CLOSE_LONG`, `CLOSE_SHORT`    | 恢复看门狗平仓 |

## 规则

1. **入场类** signalKind 只能产生 `OPEN_*` 意图
2. **出场类** signalKind 只能产生 `CLOSE_*` 意图
3. 如果一个 signalKind 需要同时支持开仓和平仓，说明它的语义不清晰，应该拆分
4. 新增 signalKind 时：
   - 在此表中声明允许的 Intent 范围
   - 在 `internal/domain/order_intent_test.go` 中添加对应的 Golden Case
   - 确保 `go test ./internal/domain/... -run TestClassifyOrderIntent` 通过
