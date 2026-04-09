---
name: Frontend-Design-System-Skill
description: 规定 BKTrader 前端界面的暗黑主轴、Glassmorphism(玻璃拟态) 和 Tailwind 原子类标准，避免原生组件栈的生硬感。
---

# Frontend Design System Skill

当你作为 AI 助手处理 BKTrader 前端 UI 时，不管用户索要什么样的组件，都**必须严格遵从**以下视觉设计公约。切忌生成干瘪简单的“灰色方块”或默认系统样式。

## 1. 核心设计语言：极客暗黑与空间景深 (Glassmorphism)
我们希望控制台拥有一种“悬浮在数据流之上”的高级质感。
- **背景底色**：绝对禁止使用纯黑 (`#000000`) 或纯白。主画板底层应为极深灰，例如 `bg-zinc-950` 或 `bg-[#09090b]`。
- **浮动面板与 Card**：所有的容器、侧边栏、模态框或抽屉，不再使用实色背景。必须采用半透明玻璃毛玻璃效果：
  - 固定写法范本：`className="backdrop-blur-xl bg-zinc-900/40 border border-white/5"`
  - 这种设计让底层图表、K 线行情在滚动时能够隐秘地透出底色，增加空间感。

## 2. 颜色与情绪心理学 (Color Palette)
长时间盯盘的交易员对色彩饱和度极为敏感。我们的核心色彩要柔和发光，避免“红绿灯”式的刺眼。
- **盈利/上涨态势**：`text-emerald-400` 或 `text-teal-400`。在展示重要盈亏数字时，可加入发光阴影以模仿终端机荧光管：`drop-shadow-[0_0_10px_rgba(52,211,153,0.3)]`。
- **亏损/下跌态势**：`text-rose-400`。切忌使用纯粹的 `bg-red-500`。
- **主要文字**：`text-zinc-200` 或 `text-zinc-300`。次要说明文字：`text-zinc-500`。

## 3. 字体与排版布局 (Typography)
- **文本字体**：全局 UI 文字统一预设为 `Inter`（若能引入）或系统级 sans-serif。
- **数字面板**：涉及到价格、数量、倒计时的地方，**必须**使用等宽字体，如 `font-mono`，或明确引入 `JetBrains Mono`。确保价格上下跳动时排版不会因为字符宽窄产生整体宽度抖动。
- **圆角规范**：不作特殊强调时，全局微圆角统一采用 `rounded-lg` 或 `rounded-xl` 以保证亲和力。不出现绝对的直角。

## 4. UI 库选型优先度
实现这些标准的最佳方式是使用 `Tailwind CSS` 搭配 `shadcn/ui`。如果没有 shadcn，必须用纯粹的 Tailwind `className` 组合将上述质感拼装出来。
