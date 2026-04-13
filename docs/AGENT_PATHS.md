# bktrader 本地工具路径导览 (AGENT_PATHS)

> **致所有 Agent：** 
> 本文档记录了本开发机中常用工具的**绝对路径**。由于当前环境的部分 Shell 会话可能无法通过 `PATH` 寻找到某些工具（如 `gh`, `node`, `npm`），请在执行命令时优先使用此处记录的绝对路径，或者手动 `source ~/.zshrc`。

## 1. 核心运行时 (Runtime)

- **Node.js**: `/Users/fujun/.nvm/versions/node/v22.18.0/bin/node`
- **NPM**: `/Users/fujun/.nvm/versions/node/v22.18.0/bin/npm`
- **Go**: `/opt/homebrew/bin/go`
- **Python 3**: `/usr/bin/python3`

## 2. 协作与版本控制 (CLI Tools)

- **GitHub CLI (gh)**: `/opt/homebrew/bin/gh`
- **Git**: `/usr/bin/git`

## 3. 环境变量加载

如果命令执行失败（提示 `command not found`），请尝试在命令前执行：
```bash
source ~/.zshrc && <your_command>
```
---
*上次更新时间: 2026-04-13*
