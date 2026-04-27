package main

import (
	"fmt"
	"github.com/spf13/cobra"
)

func init() {
	liveCmd.AddCommand(liveListCmd)
	liveCmd.AddCommand(liveGetCmd)
	liveCmd.AddCommand(liveStartCmd)
	liveCmd.AddCommand(liveStopCmd)
	liveCmd.AddCommand(liveDispatchCmd)
	liveCmd.AddCommand(liveSyncCmd)
	liveCmd.AddCommand(liveDeleteCmd)
	liveCmd.AddCommand(liveTemplateCmd)
	rootCmd.AddCommand(liveCmd)
}

// Live Commands
var liveCmd = &cobra.Command{
	Use:   "live",
	Short: "实盘会话管理",
}

var liveListCmd = &cobra.Command{
	Use:   "list",
	Short: "获取所有实盘会话列表 [IDEMPOTENT]",
	RunE: func(cmd *cobra.Command, args []string) error {
		view, _ := cmd.Flags().GetString("view")
		client := getClient()
		path := "/api/v1/live/sessions"
		if view != "" {
			path += "?view=" + view
		}
		resp, err := client.Request("GET", path, nil)
		handleResponse(resp, err)
		return nil
	},
}

var liveGetCmd = &cobra.Command{
	Use:   "get <sessionId>",
	Short: "查看实盘会话详情 [IDEMPOTENT]",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		resp, err := client.Request("GET", "/api/v1/live/sessions/"+args[0]+"/detail", nil)
		handleResponse(resp, err)
		return nil
	},
}

var liveStartCmd = &cobra.Command{
	Use:   "start <sessionId>",
	Short: "启动实盘会话 [MUTATING]",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		confirm, _ := cmd.Flags().GetBool("confirm")
		if !confirm && !dryRun {
			return fmt.Errorf("操作需要 --confirm 确认")
		}
		client := getClient()
		resp, err := client.Request("POST", "/api/v1/live/sessions/"+args[0]+"/start", nil)
		handleResponse(resp, err)
		return nil
	},
}

var liveStopCmd = &cobra.Command{
	Use:   "stop <sessionId>",
	Short: "停止实盘会话 [MUTATING]",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		confirm, _ := cmd.Flags().GetBool("confirm")
		force, _ := cmd.Flags().GetBool("force")
		if !confirm && !dryRun {
			return fmt.Errorf("操作需要 --confirm 确认")
		}
		client := getClient()
		path := "/api/v1/live/sessions/" + args[0] + "/stop"
		if force {
			path += "?force=true"
		}
		resp, err := client.Request("POST", path, nil)
		handleResponse(resp, err)
		return nil
	},
}

var liveDispatchCmd = &cobra.Command{
	Use:   "dispatch <sessionId>",
	Short: "手动触发实盘下单 [MUTATING]",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		confirm, _ := cmd.Flags().GetBool("confirm")
		if !confirm && !dryRun {
			return fmt.Errorf("操作需要 --confirm 确认")
		}
		client := getClient()
		resp, err := client.Request("POST", "/api/v1/live/sessions/"+args[0]+"/dispatch", nil)
		handleResponse(resp, err)
		return nil
	},
}

var liveSyncCmd = &cobra.Command{
	Use:   "sync <sessionId>",
	Short: "同步实盘状态 [MUTATING]",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		resp, err := client.Request("POST", "/api/v1/live/sessions/"+args[0]+"/sync", nil)
		handleResponse(resp, err)
		return nil
	},
}

var liveDeleteCmd = &cobra.Command{
	Use:   "delete <sessionId>",
	Short: "删除实盘会话 [MUTATING]",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		confirm, _ := cmd.Flags().GetBool("confirm")
		if !confirm && !dryRun {
			return fmt.Errorf("操作需要 --confirm 确认")
		}
		client := getClient()
		resp, err := client.Request("DELETE", "/api/v1/live/sessions/"+args[0], nil)
		handleResponse(resp, err)
		return nil
	},
}

// Template Commands
var liveTemplateCmd = &cobra.Command{
	Use:   "template",
	Short: "启动模板管理",
}

var liveTemplateListCmd = &cobra.Command{
	Use:   "list",
	Short: "获取启动模板列表 [IDEMPOTENT]",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		resp, err := client.Request("GET", "/api/v1/live/launch-templates", nil)
		handleResponse(resp, err)
		return nil
	},
}

func init() {
	liveTemplateCmd.AddCommand(liveTemplateListCmd)

	liveListCmd.Flags().String("view", "", "视图类型 (e.g. summary)")

	// 安全确认标志
	liveStartCmd.Flags().Bool("confirm", false, "确认执行启动操作")
	liveStopCmd.Flags().Bool("confirm", false, "确认执行停止操作")
	liveStopCmd.Flags().Bool("force", false, "强制停止")
	liveDispatchCmd.Flags().Bool("confirm", false, "确认执行下单操作")
	liveDeleteCmd.Flags().Bool("confirm", false, "确认执行删除操作")
}
