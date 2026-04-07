# CI/CD 维护与故障排除指南

本文档涵盖了 bktrader 项目中 CI/CD 的基础维护操作和常见问题的解决方法。

## 工作流说明

项目目前包含两个主要的 GitHub Actions 工作流：
- **CI (`ci.yml`)**: 自动执行后端 (Go) 格式检查、编译、前端 (Vite) 构建以及 Docker 镜像构建验证。
- **CD (`cd.yml`)**: 自动构建并推送 Docker 镜像，随后在自托管 (Self-hosted) 的 Macmini 节点上执行部署脚本。

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
**原因**：自托管的 Runner 服务掉线或由于权限问题无法启动。
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

---

## 常用路径参考

- **Runner 本地日志**：Runner 目录下的 `_diag/` 文件夹。
- **macOS 系统服务日志**：`~/Library/Logs/actions.runner.<identifier>/`
- **Runner 服务配置**：`~/Library/LaunchAgents/` 下对应的 `.plist` 文件。
