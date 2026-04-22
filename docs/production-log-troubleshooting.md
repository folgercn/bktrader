# Production Log Troubleshooting

本文档是生产环境日志排查入口。Agent 遇到线上告警、Telegram 噪声、`stale-source-states`、Binance `429`、live sync 风暴、runtime 异常时，应优先按本文档进入日志，而不是只看本地下载的片段。

## 1. 远程入口

生产服务器：

```bash
ssh fujun@192.168.100.89
```

服务日志目录：

```bash
/Users/fujun/services/bktrader/logs
```

建议先只读排查，不要修改生产文件：

```bash
ssh fujun@192.168.100.89 'ls -lht /Users/fujun/services/bktrader/logs | head -20'
```

## 2. 推荐起手式

确认最新日志文件：

```bash
ssh fujun@192.168.100.89 'ls -t /Users/fujun/services/bktrader/logs | head'
```

查看最近错误和告警：

```bash
ssh fujun@192.168.100.89 "cd /Users/fujun/services/bktrader/logs && rg -n 'stale-source-states|runtime source gate|rate-limited|429|live account sync|telegram dispatch|chart/candles' . | tail -200"
```

如果需要拉回本地分析：

```bash
scp fujun@192.168.100.89:/Users/fujun/services/bktrader/logs/<log-file> /Users/fujun/Downloads/
```

## 3. stale-source-states 排查顺序

遇到 Telegram 报：

- `stale-source-states`
- `数据源过期`
- `runtime-stale-*`
- `live-warning-stale-source-states-*`

不要先调 Telegram 去抖窗口。按以下顺序查：

1. 查 `runtime source gate blocked`
   - 重点字段：`runtime_session_id`、`stale_sources`、`sourceKey`、`streamType`、`symbol`、`lastEventAt`、`maxAgeSec`。
   - 目标：确认到底是哪一路 source stale，例如 `binance-order-book` 或 `binance-kline`。
2. 查 `runtime source gate recovered`
   - 目标：判断是短暂抖动还是持续阻塞。
3. 查 `live account sync request`
   - 重点字段：`trigger`、`coalesced`、`waited_for_inflight`、`reused_recent_result`、`result_error`。
   - 目标：确认是否仍有某条 trigger 在高频触发账户同步。
4. 查 Binance REST 限流
   - 关键字：`429 Too Many Requests`、`temporarily rate-limited`、`Retry-After`。
   - 目标：确认 stale 是否和 REST 压力、sync 风暴同时间发生。

常用命令：

```bash
ssh fujun@192.168.100.89 "cd /Users/fujun/services/bktrader/logs && rg -n 'runtime source gate (blocked|recovered)|stale_sources|sourceKey|live account sync request|429 Too Many|temporarily rate-limited' . | tail -300"
```

## 4. Binance REST 压力排查

如果怀疑交易所 API 被打爆，先聚合这些关键字：

```bash
ssh fujun@192.168.100.89 "cd /Users/fujun/services/bktrader/logs && rg -n '429 Too Many|temporarily rate-limited|binance request failed|live account sync request executing|live account sync request completed|syncing live account|/api/v1/chart/candles' . | tail -300"
```

重点判断：

- 是否有 `live-terminal-order-sync` 在短时间内大量出现。
- 是否有 `sync-active-live-accounts` 被合并或复用。
- 是否仍有前端请求 `/api/v1/chart/candles?symbol=BTCUSDT&resolution=1&limit=840`。
- 是否同一时间出现 `runtime source gate blocked`。

## 5. 前端 K 线请求检查

Monitor 主图应优先使用 runtime `sourceStates.signal_bar`。生产日志中不应再持续出现固定高频的：

```text
/api/v1/chart/candles?symbol=BTCUSDT&resolution=1&...&limit=840
```

如果仍然出现，优先判断：

- 线上前端是否还没部署包含 PR #138 的版本。
- 是否有旧浏览器页面未刷新。
- 是否有其他页面仍在调用旧 candles 链路。

## 6. 安全边界

生产日志排查默认只读：

- 不要在 SSH 会话中修改配置、重启服务或删除日志，除非人类明确要求。
- 不要把生产日志里的密钥、token、账户敏感字段复制进 PR 或公开 issue。
- 涉及 `internal/service/live*.go`、reconcile、dispatch、执行策略的修复仍按 AGENTS 高风险规则执行。

## 7. 相关文档

- [20260422-binance-rest-rate-limit-and-live-sync-storm-plan.md](20260422-binance-rest-rate-limit-and-live-sync-storm-plan.md)
- [live-safety-invariants.md](live-safety-invariants.md)
- [agent-risk-model.md](agent-risk-model.md)
