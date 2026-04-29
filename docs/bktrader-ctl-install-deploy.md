# bktrader-ctl 安装与发布说明

本文只记录 `bktrader-ctl` 命令行工具的本机安装、API 配置、发布链路和发布后验证。完整命令手册见 [bktrader-ctl-reference.md](bktrader-ctl-reference.md)。

## 1. 安装最新版二进制

当前发布目标是 macOS Apple Silicon，对应 release asset 为 `bktrader-ctl-darwin-arm64`。

```bash
mkdir -p ~/.local/bin
curl -L -o ~/.local/bin/bktrader-ctl \
  https://github.com/folgercn/bktrader/releases/latest/download/bktrader-ctl-darwin-arm64
chmod +x ~/.local/bin/bktrader-ctl
```

确认 `~/.local/bin` 已在 `PATH`：

```bash
echo "$PATH" | tr ':' '\n' | grep -x "$HOME/.local/bin"
bktrader-ctl version --json
```

## 2. 本机配置

配置文件固定放在本机用户目录，不入库：

```bash
cat > ~/.bktrader-ctl.yaml <<'YAML'
api_url: https://trade.sunnywifi.cn:3088
username: "<username>"
password: "<password>"
YAML
chmod 600 ~/.bktrader-ctl.yaml
```

`api_url` 指向公网入口 `https://trade.sunnywifi.cn:3088`。该入口由公网 Nginx 转发到 FRP，再进入 Macmini 上的 `platform-api:8080`。

也可以用环境变量覆盖配置：

```bash
export BKTRADER_API_URL=https://trade.sunnywifi.cn:3088
export BKTRADER_USERNAME="<username>"
export BKTRADER_PASSWORD="<password>"
```

## 3. 登录与基础检查

首次使用先登录，token 会缓存在 `~/.bktrader-ctl.cache.json`：

```bash
bktrader-ctl auth login --json
bktrader-ctl auth me --json
bktrader-ctl status --json
```

常用只读检查：

```bash
bktrader-ctl account list --json
bktrader-ctl account summary --json
bktrader-ctl live list --json
bktrader-ctl live control-status
bktrader-ctl order list --json
bktrader-ctl position list --json
bktrader-ctl logs system --json
bktrader-ctl logs events --json
bktrader-ctl logs live-control-summary
```

`logs live-control-summary` 的 `totalEvents` / `failed` / `latency` 等历史指标受 `--from` / `--to` 过滤；`currentPending` / `currentErrors` 是当前状态快照，不受时间过滤。需要机器读取时继续加 `--json`。

排查单个订单或链路时：

```bash
bktrader-ctl order get <order-id> --json
bktrader-ctl logs trace <order-id> --json
bktrader-ctl logs stream
```

## 4. 变更类命令约束

所有 `[MUTATING]` 命令先 dry-run，再显式确认：

```bash
bktrader-ctl order cancel <order-id> --dry-run --json
bktrader-ctl order cancel <order-id> --confirm --json
```

常见变更入口包括：

- `account bind`
- `account sync`
- `account reconcile`
- `live sync`
- `order cancel`
- `order sync`
- `position close`
- `notify test`
- `update`

## 5. 手动更新 CLI

```bash
bktrader-ctl update --confirm --json
bktrader-ctl version --json
```

`bktrader-ctl` 也会做静默更新检查。当前项目约定是：CLI 有更新就发布，最新 CLI release 可以成为 GitHub latest。

## 6. 自动发布链路

CLI 发布与后端 Docker/CD 分开：

1. CLI 相关代码合入 `main` 后触发 `Auto Release CLI`。
2. `Auto Release CLI` 只负责检查和打 tag：
   - `go test ./cmd/bktrader-ctl ./internal/ctlclient ./scripts/check-ctl-coverage`
   - `go run scripts/check-ctl-coverage/main.go`
   - 创建 `ctl-v<commit-time>-<short-sha>` tag
   - dispatch `Release CLI`
3. `Release CLI` checkout 该 tag，编译 `bktrader-ctl-darwin-arm64`，生成 `checksums.txt`，创建 GitHub Release。
4. 手动推送 `v*` 或 `ctl-v*` tag 也会触发 `Release CLI`。

后端 Docker/CD 仍由 `.github/workflows/cd.yml` 管理。CLI 只改命令行工具时，不应该混进后端容器发布；后端代码变更仍会走原来的自动 CI/CD。

## 7. 发布后验证

查看 workflow：

```bash
gh run list --workflow "Auto Release CLI" --limit 3
gh run list --workflow "Release CLI" --limit 3
```

查看 release：

```bash
gh release list --limit 5
gh release view "$(gh release list --limit 1 --json tagName -q '.[0].tagName')"
```

下载并验证版本注入：

```bash
tmpdir="$(mktemp -d)"
curl -L -o "$tmpdir/bktrader-ctl" \
  https://github.com/folgercn/bktrader/releases/latest/download/bktrader-ctl-darwin-arm64
chmod +x "$tmpdir/bktrader-ctl"
"$tmpdir/bktrader-ctl" version --json
```

输出里的 `version` 应该是 `ctl-v...` tag，`commit` 应该对应触发发布的 main commit。
