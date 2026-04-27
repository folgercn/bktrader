package main

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
)

func init() {
	backtestCmd.AddCommand(backtestListCmd)
	backtestCmd.AddCommand(backtestRunCmd)
	backtestCmd.AddCommand(backtestReportCmd)
	backtestCmd.AddCommand(backtestOptionsCmd)
	rootCmd.AddCommand(backtestCmd)
}

var backtestCmd = &cobra.Command{
	Use:   "backtest",
	Short: "回测管理",
}

var backtestListCmd = &cobra.Command{
	Use:   "list",
	Short: "获取所有回测记录列表 [IDEMPOTENT]",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		resp, err := client.Request("GET", "/api/v1/backtests", nil)
		handleResponse(resp, err)
		return nil
	},
}

var backtestRunCmd = &cobra.Command{
	Use:   "run",
	Short: "启动一个新回测 [MUTATING]",
	RunE: func(cmd *cobra.Command, args []string) error {
		strategyVersionId, _ := cmd.Flags().GetString("strategy-version-id")
		paramsStr, _ := cmd.Flags().GetString("parameters")

		if strategyVersionId == "" {
			return fmt.Errorf("strategy-version-id is required")
		}

		var parameters map[string]any
		if paramsStr != "" {
			if err := json.Unmarshal([]byte(paramsStr), &parameters); err != nil {
				return fmt.Errorf("invalid parameters JSON: %w", err)
			}
		}

		payload := map[string]any{
			"strategyVersionId": strategyVersionId,
			"parameters":        parameters,
		}

		client := getClient()
		resp, err := client.Request("POST", "/api/v1/backtests", payload)
		handleResponse(resp, err)
		return nil
	},
}

var backtestReportCmd = &cobra.Command{
	Use:   "report <backtestId>",
	Short: "获取回测成交报告 (CSV 格式) [IDEMPOTENT]",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		// 注意：这里的响应是 CSV，handleResponse 需要能处理非 JSON
		resp, err := client.Request("GET", "/api/v1/backtests/"+args[0]+"/execution-trades.csv", nil)
		if err != nil {
			handleResponse(nil, err)
			return nil
		}
		fmt.Println(string(resp))
		return nil
	},
}

var backtestOptionsCmd = &cobra.Command{
	Use:   "options",
	Short: "获取回测可用配置选项 [IDEMPOTENT]",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		resp, err := client.Request("GET", "/api/v1/backtests/options", nil)
		handleResponse(resp, err)
		return nil
	},
}

func init() {
	backtestRunCmd.Flags().String("strategy-version-id", "", "策略版本 ID")
	backtestRunCmd.Flags().String("parameters", "{}", "回测参数 (JSON 字符串)")
}
