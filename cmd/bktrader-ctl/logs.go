package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/spf13/cobra"
	"github.com/wuyaocheng/bktrader/internal/ctlclient"
)

func init() {
	logsCmd.AddCommand(logsSystemCmd)
	logsCmd.AddCommand(logsEventCmd)
	logsCmd.AddCommand(logsLiveControlSummaryCmd)
	logsCmd.AddCommand(logsHTTPCmd)
	logsCmd.AddCommand(logsStreamCmd)
	logsCmd.AddCommand(logsTraceCmd)
	rootCmd.AddCommand(logsCmd)
}

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "日志与全链路追踪",
}

var logsSystemCmd = &cobra.Command{
	Use:   "system",
	Short: "查询系统日志 [IDEMPOTENT]",
	RunE: func(cmd *cobra.Command, args []string) error {
		limit, _ := cmd.Flags().GetInt("limit")
		level, _ := cmd.Flags().GetString("level")
		comp, _ := cmd.Flags().GetString("component")

		v := url.Values{}
		v.Set("limit", fmt.Sprint(limit))
		if level != "" {
			v.Set("level", level)
		}
		if comp != "" {
			v.Set("component", comp)
		}

		client := getClient()
		resp, err := client.Request("GET", "/api/v1/logs/system?"+v.Encode(), nil)
		handleResponse(resp, err)
		return nil
	},
}

var logsEventCmd = &cobra.Command{
	Use:   "event",
	Short: "查询统一业务事件 [IDEMPOTENT]",
	RunE: func(cmd *cobra.Command, args []string) error {
		orderId, _ := cmd.Flags().GetString("order-id")
		v := url.Values{}
		if orderId != "" {
			v.Set("orderId", orderId)
		}

		client := getClient()
		resp, err := client.Request("GET", "/api/v1/logs/events?"+v.Encode(), nil)
		handleResponse(resp, err)
		return nil
	},
}

var logsLiveControlSummaryCmd = &cobra.Command{
	Use:   "live-control-summary",
	Short: "查询 LiveSession 控制面指标汇总 [IDEMPOTENT]",
	RunE: func(cmd *cobra.Command, args []string) error {
		v := url.Values{}
		for flag, queryKey := range map[string]string{
			"account-id":      "accountId",
			"strategy-id":     "strategyId",
			"live-session-id": "liveSessionId",
			"from":            "from",
			"to":              "to",
		} {
			value, _ := cmd.Flags().GetString(flag)
			if value != "" {
				v.Set(queryKey, value)
			}
		}

		client := getClient()
		path := "/api/v1/logs/live-control/summary"
		if len(v) > 0 {
			path += "?" + v.Encode()
		}
		resp, err := client.Request("GET", path, nil)
		handleResponse(resp, err)
		return nil
	},
}

var logsHTTPCmd = &cobra.Command{
	Use:   "http",
	Short: "查询 HTTP 请求日志 [IDEMPOTENT]",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		resp, err := client.Request("GET", "/api/v1/logs/http", nil)
		handleResponse(resp, err)
		return nil
	},
}

var logsStreamCmd = &cobra.Command{
	Use:   "stream",
	Short: "实时流式日志 (远程 tail -f) [IDEMPOTENT]",
	RunE: func(cmd *cobra.Command, args []string) error {
		source, _ := cmd.Flags().GetString("source")
		client := getClient()
		path := "/api/v1/logs/stream"
		if source != "" {
			v := url.Values{}
			v.Set("source", source)
			path += "?" + v.Encode()
		}

		fmt.Printf("Connecting to log stream (source: %s)...\n", source)
		return client.StreamSSE("GET", path, func(ev ctlclient.SSEEvent) {
			if outputJSON {
				fmt.Println(ev.Data)
			} else {
				fmt.Printf("[%s] %s\n", ev.Event, ev.Data)
			}
		})
	},
}

var logsTraceCmd = &cobra.Command{
	Use:   "trace",
	Short: "全链路深度追踪 [IDEMPOTENT]",
	Long: `通过 order-id 或 session-id 自动关联并聚合全链路日志。
包括: 策略决策、下单分发、交易所回执、成交详情。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		orderId, _ := cmd.Flags().GetString("order-id")
		if orderId == "" {
			return fmt.Errorf("order-id is required for trace")
		}

		client := getClient()
		fmt.Fprintf(os.Stderr, "Tracing order %s across all logs...\n", orderId)

		result := map[string]any{
			"orderId":  orderId,
			"order":    nil,
			"timeline": []any{},
			"errors":   []string{},
		}
		var traceErrors []string

		// 1. 获取业务事件
		eventQuery := url.Values{}
		eventQuery.Set("orderId", orderId)
		eventsData, err := client.Request("GET", "/api/v1/logs/events?"+eventQuery.Encode(), nil)
		if err != nil {
			traceErrors = append(traceErrors, "events: "+err.Error())
		}
		// 2. 获取订单详情
		orderData, err := client.Request("GET", "/api/v1/orders/"+url.PathEscape(orderId), nil)
		if err != nil {
			traceErrors = append(traceErrors, "order: "+err.Error())
		}

		// 合并输出
		var ord any
		if orderData != nil {
			json.Unmarshal(orderData, &ord)
		}
		result["order"] = ord

		var events struct {
			Items []any `json:"items"`
		}
		if eventsData != nil {
			json.Unmarshal(eventsData, &events)
		}
		result["timeline"] = events.Items
		result["errors"] = traceErrors

		finalJSON, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(finalJSON))
		return nil
	},
}

func init() {
	logsSystemCmd.Flags().Int("limit", 100, "返回数量")
	logsSystemCmd.Flags().String("level", "", "日志级别")
	logsSystemCmd.Flags().String("component", "", "组件名称")
	logsEventCmd.Flags().String("order-id", "", "订单 ID 过滤")
	logsLiveControlSummaryCmd.Flags().String("account-id", "", "账户 ID 过滤")
	logsLiveControlSummaryCmd.Flags().String("strategy-id", "", "策略 ID 过滤")
	logsLiveControlSummaryCmd.Flags().String("live-session-id", "", "实盘会话 ID 过滤")
	logsLiveControlSummaryCmd.Flags().String("from", "", "开始时间 (RFC3339 或 Unix 秒/毫秒)")
	logsLiveControlSummaryCmd.Flags().String("to", "", "结束时间 (RFC3339 或 Unix 秒/毫秒)")
	logsStreamCmd.Flags().String("source", "", "流来源 (system,http,alert,timeline)")
	logsTraceCmd.Flags().String("order-id", "", "要追踪的订单 ID")
}
