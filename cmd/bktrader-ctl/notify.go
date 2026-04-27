package main

import (
	"github.com/spf13/cobra"
)

func init() {
	notifyCmd.AddCommand(notifyListCmd)
	rootCmd.AddCommand(notifyCmd)

	alertCmd.AddCommand(alertListCmd)
	rootCmd.AddCommand(alertCmd)
}

var notifyCmd = &cobra.Command{
	Use:   "notify",
	Short: "通知管理",
}

var notifyListCmd = &cobra.Command{
	Use:   "list",
	Short: "获取通知列表 [IDEMPOTENT]",
	RunE: func(cmd *cobra.Command, args []string) error {
		includeAcked, _ := cmd.Flags().GetBool("include-acked")
		client := getClient()
		path := "/api/v1/notifications"
		if includeAcked {
			path += "?includeAcked=true"
		}
		resp, err := client.Request("GET", path, nil)
		handleResponse(resp, err)
		return nil
	},
}

var alertCmd = &cobra.Command{
	Use:   "alert",
	Short: "系统警告管理",
}

var alertListCmd = &cobra.Command{
	Use:   "list",
	Short: "获取系统警告列表 [IDEMPOTENT]",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		resp, err := client.Request("GET", "/api/v1/alerts", nil)
		handleResponse(resp, err)
		return nil
	},
}

func init() {
	notifyListCmd.Flags().Bool("include-acked", false, "包含已确认的通知")
}
