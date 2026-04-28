package main

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/wuyaocheng/bktrader/internal/ctlclient"
	"net/url"
	"os"
	"strings"
	"time"
)

func init() {
	liveCmd.AddCommand(liveListCmd)
	liveCmd.AddCommand(liveGetCmd)
	liveCmd.AddCommand(liveStartCmd)
	liveCmd.AddCommand(liveStopCmd)
	liveCmd.AddCommand(liveDispatchCmd)
	liveCmd.AddCommand(liveSyncCmd)
	liveCmd.AddCommand(liveDeleteCmd)
	liveCmd.AddCommand(liveTemplateCmd)
	rootCmd.AddCommand(liveCmd)
}

// Live Commands
var liveCmd = &cobra.Command{
	Use:   "live",
	Short: "实盘会话管理",
}

var liveListCmd = &cobra.Command{
	Use:   "list",
	Short: "获取所有实盘会话列表 [IDEMPOTENT]",
	RunE: func(cmd *cobra.Command, args []string) error {
		view, _ := cmd.Flags().GetString("view")
		client := getClient()
		path := "/api/v1/live/sessions"
		if view != "" {
			path += "?view=" + view
		}
		resp, err := client.Request("GET", path, nil)
		handleResponse(resp, err)
		return nil
	},
}

var liveGetCmd = &cobra.Command{
	Use:   "get <sessionId>",
	Short: "查看实盘会话详情 [IDEMPOTENT]",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		resp, err := client.Request("GET", "/api/v1/live/sessions/"+url.PathEscape(args[0])+"/detail", nil)
		handleResponse(resp, err)
		return nil
	},
}

var liveStartCmd = &cobra.Command{
	Use:   "start <sessionId>",
	Short: "启动实盘会话 [MUTATING]",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		confirm, _ := cmd.Flags().GetBool("confirm")
		wait, _ := cmd.Flags().GetBool("wait")
		timeout, _ := cmd.Flags().GetDuration("timeout")
		if !confirm && !dryRun {
			return fmt.Errorf("操作需要 --confirm 确认")
		}
		client := getClient()
		resp, err := client.Request("POST", "/api/v1/live/sessions/"+url.PathEscape(args[0])+"/start", nil)
		if err != nil || !wait || dryRun {
			if err == nil && !wait && !dryRun {
				fmt.Fprintln(os.Stderr, "accepted: control intent submitted only; use --wait to confirm actualStatus convergence")
			}
			handleResponse(resp, err)
			return nil
		}
		handleResponse(resp, nil)
		if err := waitLiveSessionActualStatus(client, args[0], "RUNNING", timeout); err != nil {
			return err
		}
		return nil
	},
}

var liveStopCmd = &cobra.Command{
	Use:   "stop <sessionId>",
	Short: "停止实盘会话 [MUTATING]",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		confirm, _ := cmd.Flags().GetBool("confirm")
		force, _ := cmd.Flags().GetBool("force")
		wait, _ := cmd.Flags().GetBool("wait")
		timeout, _ := cmd.Flags().GetDuration("timeout")
		if !confirm && !dryRun {
			return fmt.Errorf("操作需要 --confirm 确认")
		}
		client := getClient()
		path := "/api/v1/live/sessions/" + url.PathEscape(args[0]) + "/stop"
		if force {
			path += "?force=true"
		}
		resp, err := client.Request("POST", path, nil)
		if err != nil || !wait || dryRun {
			if err == nil && !wait && !dryRun {
				fmt.Fprintln(os.Stderr, "accepted: control intent submitted only; use --wait to confirm actualStatus convergence")
			}
			handleResponse(resp, err)
			return nil
		}
		handleResponse(resp, nil)
		if err := waitLiveSessionActualStatus(client, args[0], "STOPPED", timeout); err != nil {
			return err
		}
		return nil
	},
}

var liveDispatchCmd = &cobra.Command{
	Use:   "dispatch <sessionId>",
	Short: "手动触发实盘下单 [MUTATING]",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		confirm, _ := cmd.Flags().GetBool("confirm")
		if !confirm && !dryRun {
			return fmt.Errorf("操作需要 --confirm 确认")
		}
		client := getClient()
		resp, err := client.Request("POST", "/api/v1/live/sessions/"+url.PathEscape(args[0])+"/dispatch", nil)
		handleResponse(resp, err)
		return nil
	},
}

var liveSyncCmd = &cobra.Command{
	Use:   "sync <sessionId>",
	Short: "同步实盘状态 [MUTATING]",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		resp, err := client.Request("POST", "/api/v1/live/sessions/"+url.PathEscape(args[0])+"/sync", nil)
		handleResponse(resp, err)
		return nil
	},
}

var liveDeleteCmd = &cobra.Command{
	Use:   "delete <sessionId>",
	Short: "删除实盘会话 [MUTATING]",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		confirm, _ := cmd.Flags().GetBool("confirm")
		if !confirm && !dryRun {
			return fmt.Errorf("操作需要 --confirm 确认")
		}
		client := getClient()
		resp, err := client.Request("DELETE", "/api/v1/live/sessions/"+url.PathEscape(args[0]), nil)
		handleResponse(resp, err)
		return nil
	},
}

// Template Commands
var liveTemplateCmd = &cobra.Command{
	Use:   "template",
	Short: "启动模板管理",
}

var liveTemplateListCmd = &cobra.Command{
	Use:   "list",
	Short: "获取启动模板列表 [IDEMPOTENT]",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		resp, err := client.Request("GET", "/api/v1/live/launch-templates", nil)
		handleResponse(resp, err)
		return nil
	},
}

func init() {
	liveTemplateCmd.AddCommand(liveTemplateListCmd)

	liveListCmd.Flags().String("view", "", "视图类型 (e.g. summary)")

	// 安全确认标志
	liveStartCmd.Flags().Bool("confirm", false, "确认执行启动操作")
	liveStartCmd.Flags().Bool("wait", false, "等待 actualStatus 收敛")
	liveStartCmd.Flags().Duration("timeout", 60*time.Second, "等待超时时间")
	liveStopCmd.Flags().Bool("confirm", false, "确认执行停止操作")
	liveStopCmd.Flags().Bool("force", false, "强制停止")
	liveStopCmd.Flags().Bool("wait", false, "等待 actualStatus 收敛")
	liveStopCmd.Flags().Duration("timeout", 60*time.Second, "等待超时时间")
	liveDispatchCmd.Flags().Bool("confirm", false, "确认执行下单操作")
	liveDeleteCmd.Flags().Bool("confirm", false, "确认执行删除操作")
}

type liveSessionControlView struct {
	ID     string         `json:"id"`
	Status string         `json:"status"`
	State  map[string]any `json:"state"`
}

func waitLiveSessionActualStatus(client *ctlclient.Client, sessionID, target string, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	deadline := time.Now().Add(timeout)
	target = strings.ToUpper(strings.TrimSpace(target))
	for {
		session, err := fetchLiveSessionControlView(client, sessionID)
		if err != nil {
			return err
		}
		actual := strings.ToUpper(liveSessionControlString(session.State["actualStatus"]))
		if actual == "" {
			actual = strings.ToUpper(strings.TrimSpace(session.Status))
		}
		if actual == target {
			if outputJSON {
				data, _ := json.Marshal(session)
				fmt.Println(string(data))
			} else {
				fmt.Fprintf(os.Stderr, "actualStatus converged: %s\n", target)
			}
			return nil
		}
		if actual == "ERROR" {
			errorCode := liveSessionControlString(session.State["lastControlErrorCode"])
			return fmt.Errorf("live session control failed: desiredStatus=%s actualStatus=%s errorCode=%s error=%s hint=%s",
				liveSessionControlString(session.State["desiredStatus"]),
				actual,
				errorCode,
				liveSessionControlString(session.State["lastControlError"]),
				liveSessionControlErrorHint(errorCode),
			)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for live session %s actualStatus=%s, last actualStatus=%s desiredStatus=%s", sessionID, target, actual, liveSessionControlString(session.State["desiredStatus"]))
		}
		time.Sleep(time.Second)
	}
}

func liveSessionControlString(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func liveSessionControlErrorHint(code string) string {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case "ACTIVE_POSITIONS_OR_ORDERS":
		return "close positions/orders first or retry stop with --force"
	case "RUNTIME_LEASE_NOT_ACQUIRED", "CONTROL_OPERATION_IN_PROGRESS":
		return "retry after the current runner/control operation finishes"
	case "CONFIG_ERROR":
		return "check live session/account/runtime configuration"
	case "ADAPTER_ERROR":
		return "check exchange adapter connectivity and logs"
	default:
		return "check live-runner logs"
	}
}

func fetchLiveSessionControlView(client *ctlclient.Client, sessionID string) (liveSessionControlView, error) {
	data, err := client.Request("GET", "/api/v1/live/sessions?view=summary", nil)
	if err != nil {
		return liveSessionControlView{}, err
	}
	var sessions []liveSessionControlView
	if err := json.Unmarshal(data, &sessions); err != nil {
		return liveSessionControlView{}, err
	}
	for _, session := range sessions {
		if session.ID == sessionID {
			if session.State == nil {
				session.State = map[string]any{}
			}
			return session, nil
		}
	}
	return liveSessionControlView{}, fmt.Errorf("live session not found: %s", sessionID)
}
