# bktrader 本地工具路径导览 (AGENT_PATHS)

> **致所有 Agent：** 
> 本文档记录了本开发机中常用工具的**绝对路径**。由于当前环境的部分 Shell 会话可能无法通过 `PATH` 寻找到某些工具（如 `gh`, `node`, `npm`），请在执行命令时优先使用此处记录的绝对路径，或者手动 `source ~/.zshrc`。

## 1. 核心运行时 (Runtime)

- **Node.js**: `/Users/fujun/.nvm/versions/node/v22.18.0/bin/node`
- **NPM**: `/Users/fujun/.nvm/versions/node/v22.18.0/bin/npm` (推荐) 或 `/usr/local/bin/npm`
- **NPX**: `/Users/fujun/.nvm/versions/node/v22.18.0/bin/npx` (推荐) 或 `/usr/local/bin/npx`
- **Go**: `/opt/homebrew/bin/go`
- **Python 3**: `/usr/bin/python3`
- **Pip 3**: `/usr/bin/pip3`

## 2. 协作与版本控制 (CLI Tools)

- **GitHub CLI (gh)**: `/opt/homebrew/bin/gh`
- **Git**: `/usr/bin/git`

## 3. 环境变量加载

如果命令执行失败（提示 `command not found`），请尝试在命令前执行：
```bash
source ~/.zshrc && <your_command>
```
或者直接使用上方的绝对路径：
```bash
/opt/homebrew/bin/gh pr view 29
```

## 4. 常用前端命令示例

```bash
# 构建前端
cd web/console && /Users/fujun/.nvm/versions/node/v22.18.0/bin/npm run build

# 运行本地开发环境
cd web/console && /Users/fujun/.nvm/versions/node/v22.18.0/bin/npm run dev
```

---
*上次更新时间: 2026-04-13*
