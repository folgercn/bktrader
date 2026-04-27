package main

import (
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(statusCmd)
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "查看系统状态 [IDEMPOTENT]",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()

		// 获取健康检查快照
		resp, err := client.Request("GET", "/api/v1/monitor/health", nil)
		handleResponse(resp, err)
		return nil
	},
}
