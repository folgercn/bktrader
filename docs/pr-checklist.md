# PR Checklist

这份清单用于把仓库里的 Harness 规则转成每次 PR 前都能重复执行的动作。

## 1. 先做改动分级

在打开 PR 前，先按 [agent-risk-model.md](./agent-risk-model.md) 给改动定级：

- `L0`：文档、研究脚本、低风险样式调整
- `L1`：非关键接口、普通 store 逻辑、辅助脚本
- `L2`：执行控制面板、数据库结构、部署/CI 调整、前端底层交互
- `L3`：`dispatchMode` 默认值、`live*.go`、`testnet -> mainnet`、`mock -> real`

如果涉及 `L2/L3`，PR 描述里必须写清：

- 改动文件
- 预期行为变化
- 为什么不会破坏 `manual-review`、`testnet`、`mock` 等默认防线
- 还没有覆盖到的剩余风险

## 2. 先跑一遍范围感知自查

优先在本地执行：

```bash
bash scripts/run_changed_scope_checks.sh --staged
```

如果是已提交分支、准备发 PR，也可以对比基线分支：

```bash
bash scripts/run_changed_scope_checks.sh --base origin/main
```

这个脚本会：

- 输出本次改动触发了哪些风险范围
- 执行高风险默认值和环境模板检查
- 在改动触及后端/前端时，提示并执行对应的基础验证

## 3. 按范围补齐验证

### 文档或 Harness 资产

- 确认链接路径正确
- 确认命令可复制执行
- 如果修改了 hook / workflow / review 规则，写明如何验证

### 后端 (`cmd/`, `internal/`, `db/migrations/`)

```bash
gofmt -w .
go test ./...
go build ./cmd/platform-api
go build ./cmd/db-migrate
```

### 前端 (`web/console/`)

```bash
cd web/console && npm ci && npm run build
```

### Live / Dispatch / Runtime / Testnet 路径

除基础构建外，还应补充：

```bash
bash scripts/testnet_live_session_smoke.sh
```

如果这轮没有执行 smoke test，PR 里必须明确写明原因和剩余风险。

## 4. 对照关键红线

提交前逐项确认：

- `dispatchMode` 默认值仍为 `manual-review`
- 没有把 `mock/testnet` 隐式切到 `real/mainnet`
- 没有把真实凭证、`.env.production` 或本地 `.env` 纳入 git
- 没有在 `db/migrations/` 中加入明显危险语句
- 没有绕过人工确认直接触发 live dispatch
- 没有让前端在缺少用户显式意图的情况下发送执行动作

## 5. 组织 PR 描述

建议按下面顺序写：

1. 目的：这次改动解决什么问题
2. 风险级别：L0/L1/L2/L3
3. 改动范围：触及了哪些目录
4. 验证证据：跑了哪些命令，结果如何
5. 剩余风险：哪些验证没做、为什么没做
6. Agent 参与声明：是否使用了 LLM/Agent

## 6. 需要双审的情况

出现以下任一项时，不要合并单审 PR：

- 改动 `internal/service/live*.go`
- 改动 `internal/service/execution_strategy.go`
- 改动 `db/migrations/`
- 改动 `.github/workflows/`
- 改动 `deployments/`
- 修改任何默认行为或环境切换逻辑

此时应依赖 [../.github/CODEOWNERS](../.github/CODEOWNERS) 和 [CONTRIBUTING.md](../CONTRIBUTING.md) 中的边界来组织 review。
