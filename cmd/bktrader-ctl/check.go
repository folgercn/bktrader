package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func init() {
	checkCmd.AddCommand(checkConsistencyCmd)
	rootCmd.AddCommand(checkCmd)
}

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "一致性与环境检查",
}

var checkConsistencyCmd = &cobra.Command{
	Use:   "consistency",
	Short: "检查持仓一致性 (CLI-side logic) [IDEMPOTENT]",
	Long: `在本地对比交易所实际持仓 (Account Positions) 与系统会话状态 (Live Sessions)。
如果发现差异 (Mismatch)，将列出详细对比并建议同步。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		fmt.Println("Running consistency check...")

		// 1. 获取所有交易所实际持仓
		posData, err := client.Request("GET", "/api/v1/positions", nil)
		if err != nil {
			return err
		}
		_ = posData

		// 2. 获取所有活跃会话
		sessionData, err := client.Request("GET", "/api/v1/live/sessions", nil)
		if err != nil {
			return err
		}
		_ = sessionData

		// 简单对比逻辑 (示意)
		// TODO: 实际实现应解析 JSON 并按 symbol 聚合对比
		fmt.Fprintf(os.Stderr, "[WIP] consistency check 逻辑尚未完整实现，当前仅验证 API 可达性。\n")
		fmt.Fprintf(os.Stderr, "  positions API: OK (%d bytes)\n", len(posData))
		fmt.Fprintf(os.Stderr, "  sessions  API: OK (%d bytes)\n", len(sessionData))

		if outputJSON {
			fmt.Println(`{"status":"wip","message":"consistency check not fully implemented"}`)
		} else {
			fmt.Println("⚠️  一致性检查逻辑尚未完整实现，仅验证了 API 可达性。")
		}
		return nil
	},
}
