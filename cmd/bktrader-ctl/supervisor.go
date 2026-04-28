package main

import "github.com/spf13/cobra"

func init() {
	supervisorCmd.AddCommand(supervisorStatusCmd)
	rootCmd.AddCommand(supervisorCmd)
}

var supervisorCmd = &cobra.Command{
	Use:   "supervisor",
	Short: "Runtime Supervisor 观测",
}

var supervisorStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "查看 read-only supervisor 最近采集快照 [IDEMPOTENT]",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		resp, err := client.Request("GET", "/api/v1/supervisor/status", nil)
		handleResponse(resp, err)
		return nil
	},
}
