# bktrader 工具路径导览 (AGENT_PATHS)

> **注意：** 本文件提供通用路径参考。实际本地路径以 `AGENTS.local.md`（git-excluded）为准。
> 若命令执行失败（提示 `command not found`），请尝试 `source ~/.zshrc && <your_command>`。

## 常用工具

| 工具 | 通用查找方式 | 说明 |
|------|-------------|------|
| **Go** | `which go` | 后端编译 |
| **Node.js / NPM / NPX** | `which node` | 前端构建（通过 nvm 管理） |
| **Git** | `which git` | 版本控制 |
| **GitHub CLI (gh)** | `which gh` | PR / Issue 操作 |
| **PostgreSQL (psql)** | `which psql` | 数据库操作 |

## Graphify Python

用于重建知识图谱。**不要使用默认的 `python3`**，它可能指向没有安装 `graphify.watch` 的环境。
具体可用的 Python 解释器路径见 `AGENTS.local.md`。

重建命令（使用正确的 Python 路径）：
```bash
<graphify-python> -c "from graphify.watch import _rebuild_code; from pathlib import Path; _rebuild_code(Path('.'))"
```

> **提示：** `pre-push` git hook 已配置自动重建，通常不需要手动执行。
