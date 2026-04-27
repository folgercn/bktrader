package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"net/url"
)

func init() {
	orderCmd.AddCommand(orderListCmd)
	orderCmd.AddCommand(orderGetCmd)
	orderCmd.AddCommand(orderCountCmd)
	orderCmd.AddCommand(orderCancelCmd)
	orderCmd.AddCommand(orderSyncCmd)
	rootCmd.AddCommand(orderCmd)

	positionCmd.AddCommand(positionListCmd)
	positionCmd.AddCommand(positionCloseCmd)
	rootCmd.AddCommand(positionCmd)

	fillCmd.AddCommand(fillListCmd)
	rootCmd.AddCommand(fillCmd)
}

// Order Commands
var orderCmd = &cobra.Command{
	Use:   "order",
	Short: "订单管理",
}

var orderListCmd = &cobra.Command{
	Use:   "list",
	Short: "查询订单列表 [IDEMPOTENT]",
	RunE: func(cmd *cobra.Command, args []string) error {
		limit, _ := cmd.Flags().GetInt("limit")
		offset, _ := cmd.Flags().GetInt("offset")
		client := getClient()
		path := fmt.Sprintf("/api/v1/orders?limit=%d&offset=%d", limit, offset)
		resp, err := client.Request("GET", path, nil)
		handleResponse(resp, err)
		return nil
	},
}

var orderGetCmd = &cobra.Command{
	Use:   "get <orderId>",
	Short: "查看单个订单详情 [IDEMPOTENT]",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		resp, err := client.Request("GET", "/api/v1/orders/"+url.PathEscape(args[0]), nil)
		handleResponse(resp, err)
		return nil
	},
}

var orderCountCmd = &cobra.Command{
	Use:   "count",
	Short: "获取订单总数 [IDEMPOTENT]",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		resp, err := client.Request("GET", "/api/v1/orders/count", nil)
		handleResponse(resp, err)
		return nil
	},
}

var orderCancelCmd = &cobra.Command{
	Use:   "cancel <orderId>",
	Short: "取消订单 [MUTATING]",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		confirm, _ := cmd.Flags().GetBool("confirm")
		if !confirm && !dryRun {
			return fmt.Errorf("操作需要 --confirm 确认")
		}
		client := getClient()
		resp, err := client.Request("POST", "/api/v1/orders/"+url.PathEscape(args[0])+"/cancel", nil)
		handleResponse(resp, err)
		return nil
	},
}

var orderSyncCmd = &cobra.Command{
	Use:   "sync <orderId>",
	Short: "同步单个订单状态 [MUTATING]",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		resp, err := client.Request("POST", "/api/v1/orders/"+url.PathEscape(args[0])+"/sync", nil)
		handleResponse(resp, err)
		return nil
	},
}

// Position Commands
var positionCmd = &cobra.Command{
	Use:   "position",
	Short: "持仓管理",
}

var positionListCmd = &cobra.Command{
	Use:   "list",
	Short: "获取所有持仓列表 [IDEMPOTENT]",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		resp, err := client.Request("GET", "/api/v1/positions", nil)
		handleResponse(resp, err)
		return nil
	},
}

var positionCloseCmd = &cobra.Command{
	Use:   "close <positionId>",
	Short: "平仓 [MUTATING]",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		confirm, _ := cmd.Flags().GetBool("confirm")
		if !confirm && !dryRun {
			return fmt.Errorf("操作需要 --confirm 确认")
		}
		client := getClient()
		resp, err := client.Request("POST", "/api/v1/positions/"+url.PathEscape(args[0])+"/close", nil)
		handleResponse(resp, err)
		return nil
	},
}

// Fill Commands
var fillCmd = &cobra.Command{
	Use:   "fill",
	Short: "成交记录管理",
}

var fillListCmd = &cobra.Command{
	Use:   "list",
	Short: "获取成交流水列表 [IDEMPOTENT]",
	RunE: func(cmd *cobra.Command, args []string) error {
		limit, _ := cmd.Flags().GetInt("limit")
		offset, _ := cmd.Flags().GetInt("offset")
		client := getClient()
		path := fmt.Sprintf("/api/v1/fills?limit=%d&offset=%d", limit, offset)
		resp, err := client.Request("GET", path, nil)
		handleResponse(resp, err)
		return nil
	},
}

func init() {
	orderListCmd.Flags().Int("limit", 500, "返回数量限制")
	orderListCmd.Flags().Int("offset", 0, "分页偏移量")
	fillListCmd.Flags().Int("limit", 500, "返回数量限制")
	fillListCmd.Flags().Int("offset", 0, "分页偏移量")

	// 安全确认标志
	orderCancelCmd.Flags().Bool("confirm", false, "确认执行取消订单操作")
	positionCloseCmd.Flags().Bool("confirm", false, "确认执行平仓操作")
}
