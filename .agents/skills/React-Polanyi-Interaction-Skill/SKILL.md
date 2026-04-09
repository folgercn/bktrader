---
name: React-Polanyi-Interaction-Skill
description: 基于波兰尼默会知识理论，在 React 中处理实时交易流交互和 WebSocket 限流更新的微动效开发准则。
---

# React Polanyi Interaction Skill

当你作为 AI 助手处理 BKTrader 与核心状态变更、行情推送、异常错误相关的逻辑与前端组件时，**必须严格遵从**以下波兰尼直觉交互公约。

## 1. 消除打断式傲慢 (Anti-Mechanical Interruptions)
不要假设发生异常时用户一定希望被弹窗阻挡进程。
- **绝对禁止**：使用原生的 `window.alert` 或模态中心弹窗 (Modal) 来提示 WebSocket 闪断、行情延时、缓存写入失败。
- **正确做法**：采用“状态光晕（Glow）或平滑降级”的默会式暗示。例如，页面边缘或特定容器的边框通过过渡动画缓缓染为暗红色 `transition-colors duration-1000 border-rose-500/50`。让交易员的余光感知异常，而非阻断其当前手部的操作动作。

## 2. 让数据流具有生命力 (Breath & Pulse)
静态词汇（如显示“已连接”）无法传达系统的“活跃度”。
- **连接灯号**：系统健康状态可以通过圆点（Pulsing dot）表示。频繁交互阶段，指示灯应能捕捉数据频率进行隐秘的颤动；静默阶段则是均匀休眠呼吸（`animate-pulse`）。
- **流式递进**：对于行情日志、系统错误信息的更新，必须保证其能够柔和滑入（例如结合 `Framer Motion` 的 `initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }}`）。旧消息需要带有渐渐消失的视窗层次感。

## 3. 高频行情渲染的节流直觉 (Render Throttling and Off-loading)
人类反应时间通常在 200ms 以上，强制 React 一秒渲染百次是毫无意义的阻塞行为。
- **Websocket 处理层**：将高频的价格 Tick 数据接收放置于 `useRef` 中进行无重绘状态维护，或由 Vanilla JS/Zustand 等层接管。
- **视窗派发层**：仅通过 `requestAnimationFrame` 或者 `lodash/throttle` 设置 150-300ms 级别的帧缓冲窗口期，再去通过 `setState` 刷新 UI 界面，或者直接调用 Lightweight Charts 的 `update` Api。这样不仅能保证帧率稳定如丝，更能减少界面的毛刺感。

## 4. 肌肉记忆的延续性 (State Continuity)
涉及 Session 编辑或图表时间周期 (1d/4h signal bars) 切换时，必须尊重“潜意识动作”。
- 将用户关闭前的界面焦距点、展开折叠状态同步留存到 `localStorage` 中。当用户重新打开浏览器时，瞬间恢复离开前的一模一样的视图上下文。
