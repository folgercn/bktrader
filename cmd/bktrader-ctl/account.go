package main

import (
	"fmt"
	"github.com/spf13/cobra"
)

func init() {
	accountCmd.AddCommand(accountListCmd)
	accountCmd.AddCommand(accountSummaryCmd)
	accountCmd.AddCommand(accountEquityCmd)
	accountCmd.AddCommand(accountSyncCmd)
	accountCmd.AddCommand(accountReconcileCmd)
	accountCmd.AddCommand(accountBindCmd)
	rootCmd.AddCommand(accountCmd)
}

var accountCmd = &cobra.Command{
	Use:   "account",
	Short: "账户与资产管理",
}

var accountListCmd = &cobra.Command{
	Use:   "list",
	Short: "获取所有账户列表 [IDEMPOTENT]",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		resp, err := client.Request("GET", "/api/v1/accounts", nil)
		handleResponse(resp, err)
		return nil
	},
}

var accountSummaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "获取账户资产摘要 [IDEMPOTENT]",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		resp, err := client.Request("GET", "/api/v1/account-summaries", nil)
		handleResponse(resp, err)
		return nil
	},
}

var accountEquityCmd = &cobra.Command{
	Use:   "equity",
	Short: "查看账户权益快照 [IDEMPOTENT]",
	RunE: func(cmd *cobra.Command, args []string) error {
		accountId, _ := cmd.Flags().GetString("account-id")
		client := getClient()
		path := "/api/v1/account-equity-snapshots"
		if accountId != "" {
			path += "?accountId=" + accountId
		}
		resp, err := client.Request("GET", path, nil)
		handleResponse(resp, err)
		return nil
	},
}

var accountSyncCmd = &cobra.Command{
	Use:   "sync <accountId>",
	Short: "同步账户状态 [MUTATING]",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		resp, err := client.Request("POST", "/api/v1/live/accounts/"+args[0]+"/sync", nil)
		handleResponse(resp, err)
		return nil
	},
}

var accountReconcileCmd = &cobra.Command{
	Use:   "reconcile <accountId>",
	Short: "账户对账 [MUTATING]",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		confirm, _ := cmd.Flags().GetBool("confirm")
		if !confirm && !dryRun {
			return fmt.Errorf("操作需要 --confirm 确认")
		}
		client := getClient()
		resp, err := client.Request("POST", "/api/v1/live/accounts/"+args[0]+"/reconcile", nil)
		handleResponse(resp, err)
		return nil
	},
}

var accountBindCmd = &cobra.Command{
	Use:   "bind <accountId>",
	Short: "绑定实盘适配器 [MUTATING]",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		adapter, _ := cmd.Flags().GetString("adapter")
		apiKey, _ := cmd.Flags().GetString("api-key")
		apiSecret, _ := cmd.Flags().GetString("api-secret")
		passphrase, _ := cmd.Flags().GetString("passphrase")
		isTestnet, _ := cmd.Flags().GetBool("testnet")

		if adapter == "" {
			return fmt.Errorf("adapter is required")
		}

		payload := map[string]any{
			"adapter":    adapter,
			"apiKey":     apiKey,
			"apiSecret":  apiSecret,
			"passphrase": passphrase,
			"isTestnet":  isTestnet,
		}

		client := getClient()
		resp, err := client.Request("POST", "/api/v1/live/accounts/"+args[0]+"/binding", payload)
		handleResponse(resp, err)
		return nil
	},
}

func init() {
	accountEquityCmd.Flags().String("account-id", "", "过滤特定账户 ID")

	// Bind flags
	accountBindCmd.Flags().String("adapter", "", "适配器名称 (e.g. binance_futures)")
	accountBindCmd.Flags().String("api-key", "", "API Key")
	accountBindCmd.Flags().String("api-secret", "", "API Secret")
	accountBindCmd.Flags().String("passphrase", "", "Passphrase (if required)")
	accountBindCmd.Flags().Bool("testnet", false, "是否为测试网")

	// 安全确认标志
	accountReconcileCmd.Flags().Bool("confirm", false, "确认执行对账操作")
}
