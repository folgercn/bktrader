package main

import (
	"github.com/spf13/cobra"
)

func init() {
	signalCmd.AddCommand(signalListCmd)
	signalCmd.AddCommand(signalTypesCmd)
	signalCmd.AddCommand(signalRuntimeCmd)
	rootCmd.AddCommand(signalCmd)

	signalRuntimeCmd.AddCommand(signalRuntimeListCmd)
	signalRuntimeCmd.AddCommand(signalRuntimeStartCmd)
	signalRuntimeCmd.AddCommand(signalRuntimeStopCmd)

	signalRuntimeStopCmd.Flags().Bool("force", false, "强制停止")
}

// Signal Commands
var signalCmd = &cobra.Command{
	Use:   "signal",
	Short: "信号源管理",
}

var signalListCmd = &cobra.Command{
	Use:   "list",
	Short: "获取信号源目录 [IDEMPOTENT]",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		resp, err := client.Request("GET", "/api/v1/signal-sources", nil)
		handleResponse(resp, err)
		return nil
	},
}

var signalTypesCmd = &cobra.Command{
	Use:   "types",
	Short: "获取信号源类型列表 [IDEMPOTENT]",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		resp, err := client.Request("GET", "/api/v1/signal-source-types", nil)
		handleResponse(resp, err)
		return nil
	},
}

// Signal Runtime Commands
var signalRuntimeCmd = &cobra.Command{
	Use:   "runtime",
	Short: "信号源运行时管理",
}

var signalRuntimeListCmd = &cobra.Command{
	Use:   "list",
	Short: "获取所有信号运行时会话列表 [IDEMPOTENT]",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		resp, err := client.Request("GET", "/api/v1/signal-runtime/sessions", nil)
		handleResponse(resp, err)
		return nil
	},
}

var signalRuntimeStartCmd = &cobra.Command{
	Use:   "start <sessionId>",
	Short: "启动信号运行时会话 [MUTATING]",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		resp, err := client.Request("POST", "/api/v1/signal-runtime/sessions/"+args[0]+"/start", nil)
		handleResponse(resp, err)
		return nil
	},
}

var signalRuntimeStopCmd = &cobra.Command{
	Use:   "stop <sessionId>",
	Short: "停止信号运行时会话 [MUTATING]",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")
		client := getClient()
		path := "/api/v1/signal-runtime/sessions/" + args[0] + "/stop"
		if force {
			path += "?force=true"
		}
		resp, err := client.Request("POST", path, nil)
		handleResponse(resp, err)
		return nil
	},
}
