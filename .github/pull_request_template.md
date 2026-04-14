## 目的
_这次改动要解决什么问题 / 实现了什么功能_

> 提交前请先对照 `docs/pr-checklist.md`，并优先执行 `bash scripts/run_changed_scope_checks.sh --staged`

## 本次改动风险定级 (参照 agent-risk-model.md)
- [ ] **L0** - 低风险 (无逻辑、纯样式、研究脚本、文档)
- [ ] **L1** - 中风险 (新增无害接口、辅助工具扩容)
- [ ] **L2** - 高风险 (执行面板改动、核心数据流替换、CI/部署调整) -> **需w/f双重Review**
- [ ] **L3** - 绝对极高等级 (涉及 dispatchMode / 实盘 Live / Mock出界) -> **极度敏感预警**

## AI Agent 参与声明
- [ ] 纯人工手打改动
- [ ] 这段属于由 LLM/Agent 生成的代码，但我已经确切通读并检查了

## 风险点 checklist
_是否涉及默认行为、交易路径、部署流程、环境变量_
- [ ] `dispatchMode` 默认值有否变化？(变化则不可轻率上 main)
- [ ] 存在直接调用 `mainnet` 凭证或路由地址的硬编码？
- [ ] DB migration 是否具备向下兼容幂等性？
- [ ] 配置字段有没有无意被混改？

## 验证方式与测试证据
_本地怎么测，测试环境怎么验_
- [ ] 无需跑后端或无需编译的文档性修改
- [ ] 提供单元测试证明 / 或截图/控制台打印复制，作为实证证明此次不造成雪崩
- [ ] 已执行范围感知自查 (`bash scripts/run_changed_scope_checks.sh --staged`)
