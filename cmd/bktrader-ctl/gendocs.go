package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func init() {
	rootCmd.AddCommand(gendocsCmd)
}

var gendocsCmd = &cobra.Command{
	Use:    "gendocs",
	Short:  "自动生成命令参考文档 [INTERNAL]",
	Hidden: true, // 不在普通帮助列表中显示
	RunE: func(cmd *cobra.Command, args []string) error {
		outputDir := "docs"
		if len(args) > 0 {
			outputDir = args[0]
		}

		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return err
		}

		fmt.Printf("Generating documentation in %s/...\n", outputDir)

		// 生成单页 Markdown 参考手册
		targetFile := outputDir + "/bktrader-ctl-reference.md"
		f, err := os.Create(targetFile)
		if err != nil {
			return err
		}
		defer f.Close()

		// 写入标题和 LLM 接入指南内容
		fmt.Fprintln(f, "# bktrader-ctl 完整手册 (LLM/Agent-first)")
		fmt.Fprintln(f, "\n> 本文档由 `bktrader-ctl gendocs` 自动生成。包含接入指南与全量命令参考。")

		fmt.Fprintln(f, `
## 第一部分：LLM / AI Agent 接入指南

bktrader-ctl 在设计上优先考虑了机器调用的友好性。

### 1. 核心原则
- **始终使用 --json**：获取结构化数据。
- **零交互 (Non-interactive)**：对于变更操作，必须显式传递 --confirm。
- **stdout vs stderr**：数据在 stdout，提示和错误在 stderr。
- **退出码语义**：0=成功, 1=通用错误, 2=业务逻辑错误, 3=鉴权失败, 4=安全拦截。

### 2. 常用解析逻辑 (jq 示例)
- 获取运行中的会话: bktrader-ctl live list --json | jq -r '.items[] | select(.status == "RUNNING") | .id'
- 计算未实现盈亏: bktrader-ctl position list --json | jq '[.items[].unrealizedPnl | tonumber] | add'

### 3. 安全约束
- **[IDEMPOTENT]**：查询类命令，可安全重复调用。
- **[MUTATING]**：变更类命令，建议先用 --dry-run 预览。

---

## 第二部分：命令参考手册`)

		// 递归生成所有子命令文档
		return doc.GenMarkdown(rootCmd, f)
	},
}
