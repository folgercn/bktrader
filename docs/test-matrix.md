# 测试覆盖与验证矩阵 (Test Matrix)

针对 `bktrader` 的改动行为，必须按照范围匹配对应强度的测试方法。

## 基于范围的验证要求

建议先运行：
```sh
bash scripts/run_changed_scope_checks.sh --staged
```
用它先识别这次改动落在哪些风险范围，再补齐下面的验证。

### 后端纯逻辑变更 (`internal/domain/`, `internal/store/`)
- **Go Unit Test**: `go test ./...`
- 若有新的接口存取，相关 package 的 coverage 不应出现大滑坡。

### 后端 API 与核心引擎 (`internal/http/`, `internal/service/`)
- **构建测试**: 必须能通过 `go build ./cmd/platform-api` 检定编译未崩溃。
- **集成测试**: API 的路由是否出现 404 / panic。建议手动抓一下 cURL / Postman 结果证明接口吐回的 JSON 合规。
-涉及 `live*.go` 则进入 L3 警戒线，**必须人工核对和回归**。

### 前端功能及样式 (`web/console/`)
- **Lint与编译**: 必须先跑通 `npm ci && npm run build`（依赖 Vite/Rollup 内置的类型检查卡口）。
- 如果修改动效/流状态 (`Frontend-Design-System-Skill`)，请用截图或网页运行检查玻璃拟态 (`Glassmorphism`) 适配正确。

### 数据库结构与部署脚本 (`db/migrations/`, `deployments/`)
- 必须确保 `go build ./cmd/db-migrate` 无误。
- 如果改动了 migration，额外执行 `python3 scripts/check_migration_safety.py`。
- 在本地能顺利起一组 docker-compose 检查连接串没有写死成“线上环境的静态地址”。

## 回归部署：Smoke Test
对于包含环境调度、核心参数修正、新功能发布的大型工作线，我们通过以下命令在测试网校验一整个“建立策略 -> 创建 session -> 模拟 dispatch”的完整闭环：
```sh
bash scripts/testnet_live_session_smoke.sh
```
这不在 CI 自动链中。是由 `folgercn` 部署后**手动触发/检查**的核实利器。
