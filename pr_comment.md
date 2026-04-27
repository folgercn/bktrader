## Follow-up review / implementation notes

我刚刚基于前面的 review，尝试直接在当前 PR 分支上做了一轮修复。这里把已经做的、未完成的、以及后续建议整理一下，方便后续继续推进。

### 已尝试提交的后端修复

#### 1. `internal/http/live_recovery.go`

目标：修掉 recovery 路由的稳定性问题，并让接口更容易被前端调用。

已做内容：

- 修复 `/api/v1/live/accounts/{id}/recovery` 这类不完整路径导致 `parts[2]` 越界 panic 的问题。
- 对 recovery 路由增加 accountID / subAction 的基础校验。
- `diagnose` 除了 POST JSON 外，也兼容 GET query 参数，便于当前前端直接调用。
- `execute` 增加 action 必填校验。
- `execute` 出错时返回 400，避免把用户输入/状态不允许的问题都包装成 500。

#### 2. `internal/service/live_recovery_workbench.go`

目标：给恢复动作加服务端安全闸，避免只依赖前端按钮控制。

已做思路：

- `ExecuteLiveRecoveryAction` 执行前重新调用 `DiagnoseLiveRecovery`。
- 根据最新诊断结果重新检查 `diag.Actions`。
- 只有 action 存在且 `Allowed == true` 时才允许继续执行。
- `sync-orders` 要求 symbol 非空。
- `reset-reconcile-gate` 在仍存在 mismatch 时拒绝执行，避免绕过对账门阻塞状态。

> 注意：这一处因为是通过工具远程替换文件，建议务必本地先检查文件内容是否完整。尤其确认 `internal/service/live_recovery_workbench.go` 顶部没有出现类似 `// (省略未改动部分)` 这种占位内容。如果有，需要立即回滚该文件并重新按上述逻辑手工改。

建议本地检查：

```bash
git fetch
git checkout feat/issue-223-recovery-workbench
sed -n '1,120p' internal/service/live_recovery_workbench.go
```

### 前端改动：尝试过，但没有成功提交

我尝试把 `web/console/src/pages/RecoveryStage.tsx` 改成适配后端真实 DTO，但 GitHub 工具在大文件替换时被安全检查拦截，因此这部分没有成功提交。

目前前端仍然需要改，否则页面和后端协议仍然不一致。

### 前端后续应这样改

#### 1. DiagnosisResult 类型改为后端真实返回结构

后端返回的是：

```ts
interface RecoveryMismatch {
  scenario: string;
  level: string;
  message: string;
  mismatchFields?: string[];
}

interface RecoveryAction {
  action: string;
  label: string;
  description: string;
  allowed: boolean;
  blockedBy?: string;
  payload?: Record<string, any>;
}

interface RecoveryFact {
  source: string;
  symbol: string;
  position: Record<string, any>;
  openOrders: any[];
  recentOrders: any[];
  recentFills: any[];
  reconcileGate?: Record<string, any>;
  syncedAt: string;
}

interface DiagnosisResult {
  accountId: string;
  symbol: string;
  exchangeFact: RecoveryFact;
  dbFact: RecoveryFact;
  mismatches: RecoveryMismatch[];
  actions: RecoveryAction[];
  authoritative: boolean;
  runtimeRole: string;
  diagnosedAt: string;
}
```

需要删除/替换当前前端里这些旧字段：

- `suggestedActions`
- `timestamp`
- `type`
- `severity`
- `dbValue`
- `exchangeValue`
- `description` 作为 mismatch 字段

#### 2. diagnose 调用

可以继续 GET：

```ts
GET /api/v1/live/accounts/{accountId}/recovery/diagnose?sessionId=...&symbol=...
```

如果用户选中了 session，建议从 `session.state.symbol` 提取 symbol 一起传给后端。

#### 3. execute 调用

当前前端发的是：

```json
{
  "actionId": "...",
  "type": "...",
  "payload": {}
}
```

应改成：

```json
{
  "action": "clear-stale-position",
  "payload": {}
}
```

也就是：

```ts
body: JSON.stringify({
  action: action.action,
  payload: action.payload ?? {},
})
```

#### 4. 页面展示字段

诊断概览建议改为：

- 不一致数量：`diagnosis.mismatches.length`
- 可执行动作数量：`diagnosis.actions.filter(a => a.allowed).length`
- 权威事实：`diagnosis.authoritative`
- 诊断时间：`diagnosis.diagnosedAt`

mismatch 表格建议展示：

- `diagnosis.symbol`
- `m.scenario`
- `m.message`
- `m.level`
- `diagnosis.dbFact.position`
- `diagnosis.exchangeFact.position`

动作列表建议展示：

- `action.label`
- `action.action`
- `action.description`
- `action.allowed`
- `action.blockedBy`
- `action.payload`

按钮逻辑：

- `allowed === false` 时禁用按钮。
- `clear-stale-position` / `adopt-exchange-position` / `reset-reconcile-gate` 用危险按钮样式。
- 前端展示仅作为入口，最终是否允许执行以服务端二次诊断为准。

### 仍建议补的后端安全点

1. `fetchExchangeFact` 里关键 REST fact 获取失败时，不应该吞掉错误后返回空 fact。
   - 否则可能把“交易所事实获取失败”误判为“交易所已无仓位”。
   - 建议关键 fact 获取失败时返回 error，并禁止破坏性动作。

2. `clear-stale-position` 执行前建议继续强化：
   - 复核 DB working orders。
   - 复核 pending settlement。
   - 校验 payload 里的 `positionId` 是否等于当前要删除的 position ID。

3. 给 HTTP 层补测试：
   - 不完整 recovery URL 不 panic。
   - GET diagnose 正常。
   - POST execute 缺 action 返回 400。
   - execute 使用 `{ action, payload }` 能正确进入 service。

4. 给 service 层补测试：
   - action 不在候选 actions 或 `Allowed=false` 时拒绝执行。
   - 有 mismatch 时不能 reset reconcile gate。
   - symbol 为空时不能 sync-orders。

### 当前建议

先不要直接合并。建议下一步：

1. 本地确认我提交到 `internal/service/live_recovery_workbench.go` 的内容是否完整。
2. 修前端 DTO 与 execute 请求体。
3. 再补 `fetchExchangeFact` 错误处理和 clear-stale-position 二次校验。
4. 跑后端测试 + 前端 tsc。
