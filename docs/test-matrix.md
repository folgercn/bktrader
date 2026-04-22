# 测试矩阵 (Test Matrix)

不同目录的改动需要通过不同级别的验证。以下矩阵定义了**最小**必须通过的验证项。

## 后端

| 改动目录 | gofmt | go test ./... | go build platform-api | go build db-migrate | migration 安全检查 | 风险检查脚本 |
|---------|-------|--------------|----------------------|--------------------|--------------------|-------------|
| `internal/service/` | ✅ | ✅ | ✅ | — | — | ✅ |
| `internal/http/` | ✅ | ✅ | ✅ | — | — | — |
| `internal/store/` | ✅ | ✅ | ✅ | — | — | — |
| `internal/adapter/` | ✅ | ✅ | ✅ | — | — | ✅ |
| `cmd/` | ✅ | ✅ | ✅ | ✅ | — | — |
| `db/migrations/` | — | — | — | ✅ | ✅ | — |

## 前端

| 改动目录 | tsc --noEmit | npm run build |
|---------|-------------|---------------|
| `web/console/src/pages/` | ✅ | ✅ |
| `web/console/src/components/` | ✅ | ✅ |
| `web/console/src/stores/` | ✅ | ✅ |

## 基础设施

| 改动目录 | Docker build 校验 | 风险检查脚本 | 人工确认 |
|---------|------------------|-------------|---------|
| `deployments/` | ✅ | ✅ | ✅ L2 |
| `.github/workflows/` | — | ✅ | ✅ L2 |
| `scripts/deploy.sh` | — | ✅ | ✅ L2 |
| `configs/` | — | ✅ | — |

## 其他

| 改动目录 | 必须验证 |
|---------|---------|
| `docs/` | 无（L0） |
| `research/` | 手动确认脚本可运行（L0） |

## 高风险附加要求

当改动涉及以下区域时，除上表外还必须满足：

| 区域 | 附加要求 |
|------|---------|
| `internal/service/live*.go` | 必须补 failure path 测试；必须人工双审（L3） |
| `internal/service/execution_strategy.go` | 必须补双向测试（BUY/SELL）；必须人工双审（L3） |
| `db/migrations/` 涉及 ALTER/DROP | 必须评估锁表影响；必须 `IF EXISTS` 保证幂等（L2） |
| 任何涉及 `dispatchMode` 的改动 | CI 自动拦截 `auto-dispatch` 硬编码（L3） |
