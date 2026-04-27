package main

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
)

func init() {
	strategyCmd.AddCommand(strategyListCmd)
	strategyCmd.AddCommand(strategyCreateCmd)
	strategyCmd.AddCommand(strategyUpdateCmd)
	strategyCmd.AddCommand(strategyEnginesCmd)
	rootCmd.AddCommand(strategyCmd)

	strategyCreateCmd.Flags().String("name", "", "策略名称")
	strategyCreateCmd.Flags().String("description", "", "策略描述")
	strategyCreateCmd.Flags().String("parameters", "{}", "初始参数 (JSON)")
	strategyUpdateCmd.Flags().String("parameters", "{}", "更新参数 (JSON)")
}

// Strategy Commands
var strategyCmd = &cobra.Command{
	Use:   "strategy",
	Short: "策略管理",
}

var strategyListCmd = &cobra.Command{
	Use:   "list",
	Short: "获取所有策略列表 [IDEMPOTENT]",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		resp, err := client.Request("GET", "/api/v1/strategies", nil)
		handleResponse(resp, err)
		return nil
	},
}

var strategyCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "创建新策略 [MUTATING]",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		desc, _ := cmd.Flags().GetString("description")
		paramsStr, _ := cmd.Flags().GetString("parameters")

		if name == "" {
			return fmt.Errorf("name is required")
		}

		var parameters map[string]any
		if paramsStr != "" {
			json.Unmarshal([]byte(paramsStr), &parameters)
		}

		payload := map[string]any{
			"name":        name,
			"description": desc,
			"parameters":  parameters,
		}

		client := getClient()
		resp, err := client.Request("POST", "/api/v1/strategies", payload)
		handleResponse(resp, err)
		return nil
	},
}

var strategyUpdateCmd = &cobra.Command{
	Use:   "update <strategyId>",
	Short: "更新策略参数 [MUTATING]",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		paramsStr, _ := cmd.Flags().GetString("parameters")
		var parameters map[string]any
		if paramsStr != "" {
			if err := json.Unmarshal([]byte(paramsStr), &parameters); err != nil {
				return fmt.Errorf("invalid parameters JSON: %w", err)
			}
		}

		payload := map[string]any{
			"parameters": parameters,
		}

		client := getClient()
		resp, err := client.Request("POST", "/api/v1/strategies/"+args[0]+"/parameters", payload)
		handleResponse(resp, err)
		return nil
	},
}

var strategyEnginesCmd = &cobra.Command{
	Use:   "engines",
	Short: "获取可用策略引擎列表 [IDEMPOTENT]",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		resp, err := client.Request("GET", "/api/v1/strategy-engines", nil)
		handleResponse(resp, err)
		return nil
	},
}
