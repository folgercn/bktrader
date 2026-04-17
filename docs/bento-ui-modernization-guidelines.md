# BKTrader UI 视觉体系与现代化规范

本文档是 BKTrader 控制台后续所有前端工作的默认 UI 合同。  
如果用户没有额外说明，人类工程师与 AI Agent 都应默认遵守本文件。

目标只有两个：
- 统一 BKTrader 的视觉体系，避免每次任务都重复解释边界
- 为后续 `light / dark / system` 主题切换保留稳定技术路径

## 1. 总原则

### 1.1 视觉方向不可漂移
BKTrader 当前的主视觉是 **warm paper / trading desk**，不是通用 SaaS 深色后台。

默认要求：
- 保持暖纸面、浅金属边线、克制阴影、高信息密度的交易台气质
- 不得擅自改成 shadcn 默认中性色后台
- 不得为了“现代感”引入无来源的霓虹、玻璃炫光、紫色品牌色或大面积动画背景

### 1.2 现代化不等于重做审美
“现代化”在本项目里指的是：
- 更清晰的信息层级
- 更稳定的组件语义
- 更完整的状态反馈闭环
- 更一致的响应式重排

不包括：
- 重做品牌风格
- 改变现有页面的业务语义
- 把页面刷成模板化后台

## 2. 分层职责

BKTrader 前端统一采用三层收口：

### 2.1 shadcn 负责基础组件与主题能力
统一使用 `components/ui/*` 中的基础组件承载：
- `Card`
- `Button`
- `Badge`
- `Tabs`
- `Table`
- `Dialog`
- `AlertDialog`
- `Tooltip`
- `Select`
- `Toaster`

shadcn 的职责是：
- 组件骨架
- 可访问性与交互基础
- variant 机制
- 主题切换底座

### 2.2 BKTrader token 负责视觉语义
统一在 [web/console/src/styles.css](/Users/fujun/node/bktrader/web/console/src/styles.css) 中维护项目级 token，包括但不限于：
- surface / canvas / overlay
- text primary / secondary / muted
- border / border soft / border strong
- success / warning / danger / accent
- shadow / radius / density

页面和业务组件不应再自行发明一套颜色系统。

### 2.3 业务页面只表达领域结构
业务页面的职责是表达：
- 监控
- 执行
- 风控
- 回放
- 审计

页面可以组织布局，但不应脱离通用组件和 token 自造视觉体系。

## 3. 组件使用规范

### 3.1 选型顺序
默认按以下顺序选型：
1. 先复用已有 `components/ui/*`
2. 再用 `variant`、组合和少量布局类完成需求
3. 只有确实无法承载时，才允许新增业务组件

新增业务组件也必须继续基于 shadcn 组合，而不是手写一套新的视觉壳。

### 3.2 `className` 主要负责布局
`className` 默认应优先表达：
- 布局
- 密度
- 响应式
- 少量局部修饰

禁止在业务页面里大面积散落：
- `text-[#...]`
- `bg-[#...]`
- `border-[#...]`
- 其他难以统一维护的任意值视觉类

若某个颜色、阴影、边框在 2 个以上地方重复，应上提到 token 或组件层。

### 3.3 旧壳优先迁移，不再扩散
以下旧式局部视觉壳不应继续作为新开发默认依赖：
- `modal-overlay`
- `modal-panel`
- `ActionButton`
- `StatusPill`
- `SimpleTable`
- 大量基于 `form-field` 的旧表单壳

如需继续开发相关区域，优先迁移到统一的 `Dialog / Button / Input / Select / Badge` 语义层。
不得再新增仅为兼容旧调用而存在的包装层，默认直接使用 `Button / Badge / Table` 等基础语义组件。

## 4. 主题与暗色模式约束

### 4.1 统一使用 `next-themes + CSS variables`
主题机制统一走：
- 顶层 `ThemeProvider`
- `next-themes`
- 全局 CSS variables
- shadcn 组件消费语义 token

禁止：
- 在页面里自行 `matchMedia`
- 在组件内部私自维护 `isDark`
- 用页面级硬编码颜色堆出局部暗色模式

### 4.2 亮色优先收口，暗色基于同一语义映射
BKTrader 的正确路径是：
1. 先把亮色体系收敛成统一语义
2. 再为同一组 token 提供 dark 映射
3. 最后接通 `system / dark`

暗色模式不应重写页面，只应切换 token 取值。

## 5. 信息与交互规范

### 5.1 信息密度优先，但必须可扫描
BKTrader 是交易控制台，不是营销站。

默认要求：
- 首屏优先展示状态、风险、执行、数据新鲜度
- 一个卡片只回答一个主要决策问题
- 指标区域优先采用短标签、强数字、辅助说明三层结构
- 日志、Timeline、审计区优先保证连续性和可滚动性

### 5.2 视觉强调只服务于状态分级
默认语义：
- 绿色用于成功、健康、可执行、正向收益
- 红色用于危险、阻断、失败、不可逆动作
- 中性说明使用 muted 体系
- 同一屏最多保留一个主要视觉焦点

### 5.3 操作必须“近数据、可确认、有反馈”
涉及交易动作、会话控制、策略切换、通知配置时，默认满足：
- 按钮靠近作用对象
- 操作附近能看到关键前置状态
- 危险动作必须有确认机制
- 执行后必须有明确结果反馈

### 5.4 异步交互必须闭环
凡涉异步操作，必须具备：
- loading / disabled 状态
- toast 或等价通知
- 错误可见反馈

禁止：
- 静默失败
- 只在 console 报错
- 按钮可重复暴力提交

### 5.5 禁止使用原生确认框
风险或不可逆操作禁止使用：
- `window.confirm`
- `alert()`

统一走项目确认体系，如 [ConfirmModal.tsx](/Users/fujun/node/bktrader/web/console/src/modals/ConfirmModal.tsx)。

### 5.6 表格截断内容必须可追溯
表格允许在高信息密度场景下做单行截断，但截断不等于丢信息。

默认要求：
- 长字符串可使用 ellipsis 保持版面整洁
- 被截断的表头或单元格必须提供悬停查看完整值的能力
- 优先复用统一 `Table` 体系内的 tooltip / hover 方案，不要为单个页面再造一套提示交互

## 6. 响应式与布局规则

### 6.1 响应式是重排，不是压缩
`xl / lg / md / sm` 的处理应体现在：
- 区块重排
- 列数变化
- 主次信息折叠

不应通过粗暴压缩来保留桌面布局。

### 6.2 Bento 对齐禁止靠固定 margin
禁止通过固定 `mt-*`、固定高度、绝对定位去“抹平”卡片高度差。

需要贴底对齐时，优先使用：

```tsx
<div className="flex h-full flex-col">
  <div>上部内容</div>
  <div className="mt-auto">底部对齐内容</div>
</div>
```

## 7. 页面状态要求

默认不接受只有正常态完整的页面。

### 7.1 空状态
空状态必须回答两件事：
- 为什么为空
- 下一步能做什么

### 7.2 加载态
优先使用：
- skeleton
- 按钮 loading
- 局部加载文案

不允许整页无提示等待。

### 7.3 错误态
错误必须贴近出错区域可见，不允许只在 console 中可见。

### 7.4 实时流状态
对实时模块，至少区分：
- 从未收到数据
- 数据暂时中断
- 请求失败

## 8. 默认文案语义

BKTrader 文案应体现交易系统语义，优先使用：
- 执行
- 会话
- 信号
- 运行时
- 风控
- 回放
- 审计

避免大量使用无语义命名，如：
- Module A
- item list
- panel content

标题应回答“这里管什么”，按钮应回答“按下去做什么”，状态标签应回答“现在处于什么约束”。

## 9. 禁止项

无明确人类授权时，以下做法一律视为不合格：
- 把现有暖色纸面主题整体改回通用深色控制台
- 把页面刷成 shadcn 默认审美
- 在业务页面里自建第二套主题系统
- 用页面级硬编码颜色实现所谓暗色适配
- 用固定高度、固定 margin、硬编码绝对定位处理 Bento 布局问题
- 用 `window.confirm`、`alert()`、静默按钮、console 报错代替正式反馈
- 新增页面时绕开 `Card / Badge / Button / Dialog / AlertDialog / sonner`
- 把高风险动作做成与普通次级操作几乎无差别的样式

## 10. 给 AI Agent 的默认理解

如果用户只说“改现代一点”“优化一下 UI”“收一下这个页面”，默认理解为：
- 保持 BKTrader 现有 warm paper / trading desk 视觉方向
- 以 shadcn 为组件底座，以 BKTrader token 为视觉语义底座
- 优先优化信息层级、状态可读性、交互反馈和响应式重排
- 不擅自改品牌气质
- 不发明第三套视觉体系
- 不绕开现有确认、通知和组件机制

本文件优先于“通用现代后台审美”。
