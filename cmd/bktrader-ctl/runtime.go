package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	runtimeCmd.AddCommand(runtimeStatusCmd)
	runtimeCmd.AddCommand(runtimeRestartCmd)
	runtimeCmd.AddCommand(runtimeSuppressAutoRestartCmd)
	runtimeCmd.AddCommand(runtimeResumeAutoRestartCmd)
	rootCmd.AddCommand(runtimeCmd)
	runtimeRestartCmd.Flags().String("kind", "signal", "runtime 类型 (signal)")
	runtimeRestartCmd.Flags().Bool("force", false, "强制重启 signal runtime")
	runtimeRestartCmd.Flags().Bool("confirm", false, "确认执行重启操作")
	runtimeRestartCmd.Flags().String("reason", "", "重启原因；--force 时必填")
	runtimeSuppressAutoRestartCmd.Flags().String("kind", "signal", "runtime 类型 (signal)")
	runtimeSuppressAutoRestartCmd.Flags().Bool("confirm", false, "确认执行 suppress 操作")
	runtimeSuppressAutoRestartCmd.Flags().String("reason", "", "suppress 原因；必填")
	runtimeResumeAutoRestartCmd.Flags().String("kind", "signal", "runtime 类型 (signal)")
	runtimeResumeAutoRestartCmd.Flags().Bool("confirm", false, "确认执行 resume 操作")
	runtimeResumeAutoRestartCmd.Flags().String("reason", "", "resume 原因；必填")
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

var runtimeSuppressAutoRestartCmd = &cobra.Command{
	Use:   "suppress-auto-restart <runtimeId>",
	Short: "抑制 runtime 自动恢复 [MUTATING]",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runRuntimeAutoRestartControl(cmd, args[0], "/api/v1/runtime/suppress-auto-restart")
	},
}

var runtimeResumeAutoRestartCmd = &cobra.Command{
	Use:   "resume-auto-restart <runtimeId>",
	Short: "恢复 runtime 自动恢复 [MUTATING]",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runRuntimeAutoRestartControl(cmd, args[0], "/api/v1/runtime/resume-auto-restart")
	},
}

func runRuntimeAutoRestartControl(cmd *cobra.Command, runtimeID, path string) error {
	confirm, _ := cmd.Flags().GetBool("confirm")
	if !confirm && !dryRun {
		return fmt.Errorf("操作需要 --confirm 确认")
	}
	reason, _ := cmd.Flags().GetString("reason")
	if strings.TrimSpace(reason) == "" && !dryRun {
		return fmt.Errorf("操作需要提供 --reason")
	}
	kind, _ := cmd.Flags().GetString("kind")
	payload := map[string]any{
		"runtimeId":   runtimeID,
		"runtimeKind": kind,
		"confirm":     confirm,
		"reason":      reason,
	}
	client := getClient()
	resp, err := client.Request("POST", path, payload)
	handleResponse(resp, err)
	return nil
}
