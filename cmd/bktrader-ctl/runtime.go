package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	runtimeCmd.AddCommand(runtimeStatusCmd)
	runtimeCmd.AddCommand(runtimeRestartCmd)
	rootCmd.AddCommand(runtimeCmd)
	runtimeRestartCmd.Flags().String("kind", "signal", "runtime 类型 (signal)")
	runtimeRestartCmd.Flags().Bool("force", false, "强制重启 signal runtime")
	runtimeRestartCmd.Flags().Bool("confirm", false, "确认执行重启操作")
	runtimeRestartCmd.Flags().String("reason", "", "重启原因；--force 时必填")
}

var runtimeCmd = &cobra.Command{
	Use:   "runtime",
	Short: "统一运行态观测",
}

var runtimeStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "查看统一 runtime 状态快照 [IDEMPOTENT]",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		resp, err := client.Request("GET", "/api/v1/runtime/status", nil)
		handleResponse(resp, err)
		return nil
	},
}

var runtimeRestartCmd = &cobra.Command{
	Use:   "restart <runtimeId>",
	Short: "重启 runtime [MUTATING]",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		confirm, _ := cmd.Flags().GetBool("confirm")
		if !confirm && !dryRun {
			return fmt.Errorf("操作需要 --confirm 确认")
		}
		kind, _ := cmd.Flags().GetString("kind")
		force, _ := cmd.Flags().GetBool("force")
		reason, _ := cmd.Flags().GetString("reason")
		if force && strings.TrimSpace(reason) == "" && !dryRun {
			return fmt.Errorf("--force 需要提供 --reason")
		}
		payload := map[string]any{
			"runtimeId":   args[0],
			"runtimeKind": kind,
			"force":       force,
			"confirm":     confirm,
			"reason":      reason,
		}
		client := getClient()
		resp, err := client.Request("POST", "/api/v1/runtime/restart", payload)
		handleResponse(resp, err)
		return nil
	},
}
