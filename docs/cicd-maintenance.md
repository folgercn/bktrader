# CI/CD 维护与故障排除指南

本文档涵盖了 bktrader 项目中 CI/CD 的基础维护操作和常见问题的解决方法。

## 工作流说明

项目目前包含两个主要的 GitHub Actions 工作流：
- **CI (`ci.yml`)**: 自动执行后端 (Go) 格式检查、编译、前端 (Vite) 构建以及 Docker 镜像构建验证。
- **CD (`cd.yml`)**: 自动构建并推送后端 Docker 镜像，在自托管 (Self-hosted) 的 Macmini 节点上执行后端部署脚本，并构建前端静态文件后同步到远端 Nginx 目录。

---

## 常见问题与解决方法

### 1. 后端格式检查失败 (Verify formatting)

**现象**：CI 任务在 `Verify formatting` 步骤失败。
**原因**：提交的 Go 代码未经过 `gofmt` 格式化。
**修复方法**：
在提交代码前，在项目根目录下执行以下命令：
```bash
gofmt -w .
```
执行完毕后，提交被修改（格式化）的文件即可。

### 2. Macmini 部署节点处于 Queued 状态

**现象**：CD 任务的 `Deploy on Macmini runner` 环节显示 `Queued` (排队中)，长时间不执行。
**原因**：自托管的 Runner 服务掉线或由于权限问题无法启动。这会同时影响后端部署和前端静态发布。
**维护注意事项**：
- **目录位置**：不要将 Runner 目录放在 `~/Downloads` 或 `~/Desktop` 文件夹下。受 macOS 系统权限限制（TCC），后台服务（LaunchAgent）无法静默访问这些隐私目录。建议放在 `~/actions-runner/` 下。
- **查看状态**：
  进入 Runner 目录（例如 `~/actions-runner-bktrader`）：
  ```bash
  ./svc.sh status
  ```
- **重启服务**：
  ```bash
  ./svc.sh stop
  ./svc.sh start
  ```
- **手动唤醒（紧急避险）**：
  如果后台服务启动失败，可以手动在当前会话中运行（确保当前终端有权限）：
  ```bash
  nohup ./run.sh &
  ```

### 3. 前端静态发布失败

**现象**：CD 中 `Build and deploy frontend` 失败，通常出现在 `npm ci`、`npm run build`、`ssh` 或 `rsync` 步骤。

**排查顺序**：

1. 确认 Runner 上 Node.js 版本正常，且能访问 npm registry。
2. 确认 `web/console/package-lock.json` 与依赖没有漂移，必要时本地重新生成并提交。
3. 确认 Runner 到目标机的 SSH 免密是通的：
   ```bash
   ssh root@1.95.71.247 'echo ok'
   ```
4. 确认目标机发布目录存在且可写：
   ```bash
   ssh root@1.95.71.247 'mkdir -p /var/www/bktrader && test -w /var/www/bktrader'
   ```
5. 确认目标机安装了 `rsync`，并且 Runner 本机也能执行 `rsync`。

**当前前端发布约定**：

- 构建目录：`web/console/dist`
- 远端目录：`/var/www/bktrader`
- 发布方式：`rsync -av --delete`
- 线上访问：由 Nginx 提供静态资源，`/api/` 和 `/healthz` 反代到后端

**常见原因**：

- Runner SSH key 没有权限登录远端机器
- 远端目录权限不对
- `rsync --delete` 删除了手工放置但未纳入构建产物的文件
- 前端构建时仍把 `VITE_API_BASE` 写死到本地地址，导致线上页面请求错地址

### 4. 前端页面能打开，但接口报错

**现象**：`trade.sunnywifi.cn` 页面能加载，但数据接口返回 401、404、502 或超时。

**排查顺序**：

1. 确认 Nginx 的 `/api/` 和 `/healthz` 已反代到正确后端端口。
2. 确认 FRP 隧道已建立，远端反代目标端口可访问。
3. 确认后端容器内 `/healthz` 返回 200。
4. 确认前端生产构建没有把 API 地址写死成 `http://127.0.0.1:8080`。
5. 如果开启了鉴权，确认浏览器请求带上了正确的登录态或 Bearer token。

---

## 常用路径参考

- **Runner 本地日志**：Runner 目录下的 `_diag/` 文件夹。
- **macOS 系统服务日志**：`~/Library/Logs/actions.runner.<identifier>/`
- **Runner 服务配置**：`~/Library/LaunchAgents/` 下对应的 `.plist` 文件。
