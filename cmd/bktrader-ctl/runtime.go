package main

import "github.com/spf13/cobra"

func init() {
	runtimeCmd.AddCommand(runtimeStatusCmd)
	rootCmd.AddCommand(runtimeCmd)
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
