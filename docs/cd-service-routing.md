# CD 服务路由说明

本文记录 `main` 分支合并后 `.github/workflows/cd.yml` 如何根据变更文件选择 Docker Compose 服务，目标是避免后端改动后粗暴重启所有业务进程。

## 服务角色

- `platform-api`: HTTP API、dashboard SSE、查询/控制入口。
- `live-runner`: live session 恢复、实盘同步、runtime event 消费、最终交易执行。
- `signal-runtime-runner`: 行情预热、signal runtime scanner、WebSocket 行情与 runtime event 发布。
- `notification-worker`: Telegram 通知投递、交易事件播报、持仓报告。

四个服务共用同一个 Docker image，因此后端代码变化都会先构建并推送镜像；是否重启某个服务由 `DEPLOY_SERVICES` 决定。

生产部署必须使用本次 commit 对应的不可变镜像 tag（`sha-<github.sha>`），不能用 mutable `latest` 作为 Compose 部署输入。`latest` 仍可发布给人工排查或手动拉取，但 selective deploy 下如果只重启 `platform-api`，用 `latest` 会让未重启的 worker 继续运行旧 digest，同时 Compose 配置看起来仍是同一个 `latest` tag，容易掩盖 API / runner 版本漂移。使用 commit SHA tag 后，当前运行容器的镜像版本可以从 tag / label 直接追溯到具体 commit。

## 路由原则

1. 共享基础设施变化才全量重启。
2. API 查询、摘要、dashboard 相关变化只重启 `platform-api`。
3. live 执行相关变化重启 `platform-api` 和 `live-runner`。
4. signal runtime 相关变化重启 `platform-api` 和 `signal-runtime-runner`。
5. Telegram/通知相关变化只额外重启 `notification-worker`。
6. 未明确归类但属于后端的文件仍走全量 fallback，避免漏部署。

## 主要映射

| 文件范围 | 部署服务 |
| --- | --- |
| `Dockerfile`, `.dockerignore`, `deployments/docker-compose.prod.yml`, `scripts/deploy.sh` | 全部后端服务 |
| `internal/app/**`, `internal/config/**`, `internal/store/**`, `internal/domain/**`, `db/**` | 全部后端服务 |
| `cmd/platform-api/**`, `internal/http/**` | `platform-api` |
| `cmd/platform-worker/**` | `live-runner`, `signal-runtime-runner`, `notification-worker` |
| `internal/service/state_util*`, `logs.go`, `dashboard_broker*`, `chart*`, `paper*`, `pnl*`, `strategy_replay*` | `platform-api` |
| `internal/service/live*`, `order*`, `execution_strategy.go`, `precision_tolerance.go`, `strategy_registry.go`, `live_account_flow.go`, `live_launch*`, `live_trade*`, `telemetry*` | `platform-api`, `live-runner` |
| `internal/service/live_market_data.go`, `signal_runtime*`, `runtime_lease*` | `platform-api`, `signal-runtime-runner` |
| `internal/service/runtime_event_consumer*` | `live-runner`, `signal-runtime-runner` |
| `internal/service/runtime_event_bus*` | `live-runner`, `signal-runtime-runner` |
| `internal/service/telegram*`, `alerts.go`, `notifications*` | `platform-api`, `notification-worker` |
| `internal/service/platform.go`, `strategy.go`, `signal_source_registry.go` | 全部后端服务 |
| `internal/service/health*`, `safety_checks*` | `platform-api`, `live-runner`, `signal-runtime-runner` |

## 示例

`internal/service/state_util.go` 只影响 live/signal runtime session 的摘要裁剪逻辑，实际入口是 API/dashboard，因此只部署：

```text
platform-api
```

`internal/service/live.go` 同时影响 HTTP live 控制入口和 live runner 后台恢复/同步，因此部署：

```text
platform-api live-runner
```

`internal/service/telegram.go` 影响通知查询和 Telegram dispatcher，因此部署：

```text
platform-api notification-worker
```
