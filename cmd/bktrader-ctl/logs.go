package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/wuyaocheng/bktrader/internal/ctlclient"
)

func init() {
	logsCmd.AddCommand(logsSystemCmd)
	logsCmd.AddCommand(logsEventCmd)
	logsCmd.AddCommand(logsLiveControlSummaryCmd)
	logsCmd.AddCommand(logsLiveControlHistoryCmd)
	logsCmd.AddCommand(logsLiveControlFailuresCmd)
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
		handleLiveControlSummaryResponse(resp, err)
		return nil
	},
}

var logsLiveControlHistoryCmd = &cobra.Command{
	Use:   "live-control-history",
	Short: "查询 LiveSession 控制历史事件 [IDEMPOTENT]",
	RunE: func(cmd *cobra.Command, args []string) error {
		v := buildLiveControlLogQuery(cmd)
		client := getClient()
		path := "/api/v1/logs/live-control/history"
		if len(v) > 0 {
			path += "?" + v.Encode()
		}
		resp, err := client.Request("GET", path, nil)
		handleLiveControlEventPageResponse(resp, err)
		return nil
	},
}

var logsLiveControlFailuresCmd = &cobra.Command{
	Use:   "live-control-failures",
	Short: "查询最近 LiveSession 控制失败事件 [IDEMPOTENT]",
	RunE: func(cmd *cobra.Command, args []string) error {
		v := buildLiveControlLogQuery(cmd)
		client := getClient()
		path := "/api/v1/logs/live-control/failures"
		if len(v) > 0 {
			path += "?" + v.Encode()
		}
		resp, err := client.Request("GET", path, nil)
		handleLiveControlEventPageResponse(resp, err)
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
	for _, cmd := range []*cobra.Command{logsLiveControlHistoryCmd, logsLiveControlFailuresCmd} {
		cmd.Flags().String("account-id", "", "账户 ID 过滤")
		cmd.Flags().String("strategy-id", "", "策略 ID 过滤")
		cmd.Flags().String("live-session-id", "", "实盘会话 ID 过滤")
		cmd.Flags().String("from", "", "开始时间 (RFC3339 或 Unix 秒/毫秒)")
		cmd.Flags().String("to", "", "结束时间 (RFC3339 或 Unix 秒/毫秒)")
		cmd.Flags().Int("limit", 20, "返回数量")
		cmd.Flags().String("cursor", "", "分页游标")
	}
	logsStreamCmd.Flags().String("source", "", "流来源 (system,http,alert,timeline)")
	logsTraceCmd.Flags().String("order-id", "", "要追踪的订单 ID")
}

func buildLiveControlLogQuery(cmd *cobra.Command) url.Values {
	v := url.Values{}
	for flag, queryKey := range map[string]string{
		"account-id":      "accountId",
		"strategy-id":     "strategyId",
		"live-session-id": "liveSessionId",
		"from":            "from",
		"to":              "to",
		"cursor":          "cursor",
	} {
		value, _ := cmd.Flags().GetString(flag)
		if value != "" {
			v.Set(queryKey, value)
		}
	}
	limit, _ := cmd.Flags().GetInt("limit")
	if limit > 0 {
		v.Set("limit", fmt.Sprint(limit))
	}
	return v
}

type liveControlEventPageResponse struct {
	Items      []liveControlEventResponse `json:"items"`
	NextCursor string                     `json:"nextCursor,omitempty"`
}

type liveControlEventResponse struct {
	ID            string         `json:"id"`
	Level         string         `json:"level"`
	Title         string         `json:"title"`
	Message       string         `json:"message"`
	EventTime     time.Time      `json:"eventTime"`
	AccountID     string         `json:"accountId,omitempty"`
	StrategyID    string         `json:"strategyId,omitempty"`
	LiveSessionID string         `json:"liveSessionId,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

type liveControlSummaryResponse struct {
	GeneratedAt                 time.Time                         `json:"generatedAt"`
	TotalEvents                 int                               `json:"totalEvents"`
	Requests                    int                               `json:"requests"`
	RunnerPickups               int                               `json:"runnerPickups"`
	Succeeded                   int                               `json:"succeeded"`
	Failed                      int                               `json:"failed"`
	StaleDiscarded              int                               `json:"staleDiscarded"`
	CurrentPending              int                               `json:"currentPending"`
	CurrentErrors               int                               `json:"currentErrors"`
	CurrentMaxPendingPickupMs   int64                             `json:"currentMaxPendingPickupMs,omitempty"`
	StaleActiveControlRequests  int                               `json:"staleActiveControlRequests,omitempty"`
	OrphanActiveControlRequests int                               `json:"orphanActiveControlRequests,omitempty"`
	Scanner                     liveControlScannerResponse        `json:"scanner"`
	Latency                     liveControlLatencyMetricsResponse `json:"latency"`
	ByErrorCode                 map[string]int                    `json:"byErrorCode,omitempty"`
}

type liveControlScannerResponse struct {
	ProcessRole      string `json:"processRole,omitempty"`
	Enabled          bool   `json:"enabled"`
	LastCancelAt     string `json:"lastCancelAt,omitempty"`
	LastTickAt       string `json:"lastTickAt,omitempty"`
	LastSuccessAt    string `json:"lastSuccessAt,omitempty"`
	LastErrorAt      string `json:"lastErrorAt,omitempty"`
	LastError        string `json:"lastError,omitempty"`
	LastDurationMs   int64  `json:"lastDurationMs,omitempty"`
	LastSessionCount int    `json:"lastSessionCount,omitempty"`
	TickCount        int64  `json:"tickCount"`
	SuccessCount     int64  `json:"successCount"`
	CancelCount      int64  `json:"cancelCount"`
	ErrorCount       int64  `json:"errorCount"`
}

type liveControlLatencyMetricsResponse struct {
	PickupMs   liveControlLatencyStatsResponse `json:"pickupMs"`
	SuccessMs  liveControlLatencyStatsResponse `json:"successMs"`
	FailureMs  liveControlLatencyStatsResponse `json:"failureMs"`
	TerminalMs liveControlLatencyStatsResponse `json:"terminalMs"`
}

type liveControlLatencyStatsResponse struct {
	Count   int     `json:"count"`
	Min     int64   `json:"min,omitempty"`
	Max     int64   `json:"max,omitempty"`
	Average float64 `json:"average,omitempty"`
}

func handleLiveControlSummaryResponse(data []byte, err error) {
	if err != nil || outputJSON {
		handleResponse(data, err)
		return
	}
	var summary liveControlSummaryResponse
	if decodeErr := json.Unmarshal(data, &summary); decodeErr != nil {
		handleResponse(data, nil)
		return
	}
	var out bytes.Buffer
	fmt.Fprintf(&out, "Live control summary\n")
	fmt.Fprintf(&out, "generatedAt: %s\n", summary.GeneratedAt.Format(time.RFC3339))
	fmt.Fprintf(&out, "note: historical counters and latency honor --from/--to; currentPending/currentErrors are current snapshots and ignore --from/--to.\n")
	fmt.Fprintf(&out, "\nHistorical events:\n")
	fmt.Fprintf(&out, "  total=%d requests=%d pickedUp=%d succeeded=%d failed=%d staleDiscarded=%d\n",
		summary.TotalEvents, summary.Requests, summary.RunnerPickups, summary.Succeeded, summary.Failed, summary.StaleDiscarded)
	fmt.Fprintf(&out, "\nLatency (ms):\n")
	printLiveControlLatencyStats(&out, "pickup", summary.Latency.PickupMs)
	printLiveControlLatencyStats(&out, "success", summary.Latency.SuccessMs)
	printLiveControlLatencyStats(&out, "failure", summary.Latency.FailureMs)
	printLiveControlLatencyStats(&out, "terminal", summary.Latency.TerminalMs)
	fmt.Fprintf(&out, "\nCurrent snapshot:\n")
	fmt.Fprintf(&out, "  pending=%d errors=%d maxPendingPickupMs=%d staleActive=%d orphanActive=%d\n",
		summary.CurrentPending, summary.CurrentErrors, summary.CurrentMaxPendingPickupMs, summary.StaleActiveControlRequests, summary.OrphanActiveControlRequests)
	fmt.Fprintf(&out, "\nScanner:\n")
	fmt.Fprintf(&out, "  enabled=%t processRole=%s\n", summary.Scanner.Enabled, firstNonEmpty(summary.Scanner.ProcessRole, "--"))
	if !summary.Scanner.Enabled {
		fmt.Fprintf(&out, "  note=scanner is not started in this process; this is expected for api-only roles.\n")
	}
	fmt.Fprintf(&out, "  ticks=%d successes=%d cancels=%d errors=%d lastSessions=%d lastDurationMs=%d\n",
		summary.Scanner.TickCount, summary.Scanner.SuccessCount, summary.Scanner.CancelCount, summary.Scanner.ErrorCount, summary.Scanner.LastSessionCount, summary.Scanner.LastDurationMs)
	fmt.Fprintf(&out, "  lastTickAt=%s lastSuccessAt=%s lastCancelAt=%s lastErrorAt=%s\n",
		firstNonEmpty(summary.Scanner.LastTickAt, "--"), firstNonEmpty(summary.Scanner.LastSuccessAt, "--"), firstNonEmpty(summary.Scanner.LastCancelAt, "--"), firstNonEmpty(summary.Scanner.LastErrorAt, "--"))
	if strings.TrimSpace(summary.Scanner.LastError) != "" {
		fmt.Fprintf(&out, "  lastError=%s\n", summary.Scanner.LastError)
	}
	if len(summary.ByErrorCode) > 0 {
		fmt.Fprintf(&out, "\nError codes:\n")
		for _, code := range sortedLiveControlSummaryKeys(summary.ByErrorCode) {
			fmt.Fprintf(&out, "  %s=%d\n", code, summary.ByErrorCode[code])
		}
	}
	fmt.Print(strings.TrimRight(out.String(), "\n") + "\n")
}

func handleLiveControlEventPageResponse(data []byte, err error) {
	if err != nil || outputJSON {
		handleResponse(data, err)
		return
	}
	var page liveControlEventPageResponse
	if decodeErr := json.Unmarshal(data, &page); decodeErr != nil {
		handleResponse(data, nil)
		return
	}
	var out bytes.Buffer
	if len(page.Items) == 0 {
		fmt.Fprintln(&out, "No live control events.")
	} else {
		for i, item := range page.Items {
			if i > 0 {
				out.WriteByte('\n')
			}
			meta := item.Metadata
			fmt.Fprintf(&out, "%s %s %s\n", item.EventTime.Format(time.RFC3339), strings.ToUpper(firstNonEmpty(item.Level, "info")), firstNonEmpty(item.Title, item.ID))
			fmt.Fprintf(&out, "  session=%s action=%s phase=%s desired=%s actual=%s\n",
				firstNonEmpty(item.LiveSessionID, "-"),
				firstNonEmpty(liveSessionControlString(meta["action"]), "-"),
				firstNonEmpty(liveSessionControlString(meta["phase"]), "-"),
				firstNonEmpty(liveSessionControlString(meta["desiredStatus"]), "-"),
				firstNonEmpty(liveSessionControlString(meta["actualStatus"]), "-"),
			)
			fmt.Fprintf(&out, "  requestId=%s version=%s errorCode=%s\n",
				firstNonEmpty(liveSessionControlString(meta["controlRequestId"]), "-"),
				firstNonEmpty(liveSessionControlString(meta["controlVersion"]), "-"),
				firstNonEmpty(liveSessionControlString(meta["errorCode"]), "-"),
			)
			if strings.TrimSpace(item.Message) != "" {
				fmt.Fprintf(&out, "  message=%s\n", item.Message)
			}
		}
	}
	if strings.TrimSpace(page.NextCursor) != "" {
		fmt.Fprintf(&out, "\nnextCursor=%s\n", page.NextCursor)
	}
	fmt.Print(strings.TrimRight(out.String(), "\n") + "\n")
}

func printLiveControlLatencyStats(out *bytes.Buffer, label string, stats liveControlLatencyStatsResponse) {
	if stats.Count == 0 {
		fmt.Fprintf(out, "  %s: count=0\n", label)
		return
	}
	fmt.Fprintf(out, "  %s: count=%d avg=%.1f min=%d max=%d\n", label, stats.Count, stats.Average, stats.Min, stats.Max)
}

func sortedLiveControlSummaryKeys(values map[string]int) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
