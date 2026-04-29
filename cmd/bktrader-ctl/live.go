package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/wuyaocheng/bktrader/internal/ctlclient"
)

func init() {
	liveCmd.AddCommand(liveListCmd)
	liveCmd.AddCommand(liveGetCmd)
	liveCmd.AddCommand(liveControlStatusCmd)
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

var liveControlStatusCmd = &cobra.Command{
	Use:   "control-status [sessionId]",
	Short: "查看实盘控制面状态 [IDEMPOTENT]",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		sessions, err := fetchLiveSessionControlViews(client)
		if err != nil {
			return err
		}
		if len(args) == 1 {
			filtered := sessions[:0]
			for _, session := range sessions {
				if session.ID == args[0] {
					filtered = append(filtered, session)
				}
			}
			sessions = filtered
			if len(sessions) == 0 {
				return fmt.Errorf("live session not found: %s", args[0])
			}
		}
		now := time.Now().UTC()
		statuses := make([]liveSessionControlStatus, 0, len(sessions))
		for _, session := range sessions {
			statuses = append(statuses, buildLiveSessionControlStatus(session, now))
		}
		if outputJSON {
			data, _ := json.Marshal(statuses)
			fmt.Println(string(data))
			return nil
		}
		printLiveSessionControlStatuses(statuses)
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
	ID         string         `json:"id"`
	Alias      string         `json:"alias,omitempty"`
	AccountID  string         `json:"accountId,omitempty"`
	StrategyID string         `json:"strategyId,omitempty"`
	Status     string         `json:"status"`
	State      map[string]any `json:"state"`
}

type liveSessionControlStatus struct {
	ID               string `json:"id"`
	Alias            string `json:"alias,omitempty"`
	AccountID        string `json:"accountId,omitempty"`
	StrategyID       string `json:"strategyId,omitempty"`
	Status           string `json:"status"`
	DesiredStatus    string `json:"desiredStatus,omitempty"`
	ActualStatus     string `json:"actualStatus,omitempty"`
	Action           string `json:"action,omitempty"`
	ControlRequestID string `json:"controlRequestId,omitempty"`
	ControlVersion   string `json:"controlVersion,omitempty"`
	Pending          bool   `json:"pending"`
	PendingSeconds   int64  `json:"pendingSeconds,omitempty"`
	RequestedAt      string `json:"requestedAt,omitempty"`
	UpdatedAt        string `json:"updatedAt,omitempty"`
	SucceededAt      string `json:"succeededAt,omitempty"`
	ErrorAt          string `json:"errorAt,omitempty"`
	ErrorCode        string `json:"errorCode,omitempty"`
	Error            string `json:"error,omitempty"`
	Hint             string `json:"hint,omitempty"`
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func fetchLiveSessionControlView(client *ctlclient.Client, sessionID string) (liveSessionControlView, error) {
	sessions, err := fetchLiveSessionControlViews(client)
	if err != nil {
		return liveSessionControlView{}, err
	}
	for _, session := range sessions {
		if session.ID == sessionID {
			return session, nil
		}
	}
	return liveSessionControlView{}, fmt.Errorf("live session not found: %s", sessionID)
}

func fetchLiveSessionControlViews(client *ctlclient.Client) ([]liveSessionControlView, error) {
	data, err := client.Request("GET", "/api/v1/live/sessions?view=summary", nil)
	if err != nil {
		return nil, err
	}
	var sessions []liveSessionControlView
	if err := json.Unmarshal(data, &sessions); err != nil {
		return nil, err
	}
	for i := range sessions {
		if sessions[i].State == nil {
			sessions[i].State = map[string]any{}
		}
	}
	return sessions, nil
}

func buildLiveSessionControlStatus(session liveSessionControlView, now time.Time) liveSessionControlStatus {
	state := session.State
	desired := strings.ToUpper(liveSessionControlString(state["desiredStatus"]))
	actual := strings.ToUpper(liveSessionControlString(state["actualStatus"]))
	if actual == "" {
		actual = strings.ToUpper(strings.TrimSpace(session.Status))
	}
	status := liveSessionControlStatus{
		ID:               session.ID,
		Alias:            session.Alias,
		AccountID:        session.AccountID,
		StrategyID:       session.StrategyID,
		Status:           session.Status,
		DesiredStatus:    desired,
		ActualStatus:     actual,
		Action:           liveSessionControlString(state["lastControlAction"]),
		ControlRequestID: liveSessionControlString(state["controlRequestId"]),
		ControlVersion:   liveSessionControlString(state["controlVersion"]),
		RequestedAt:      liveSessionControlString(state["controlRequestedAt"]),
		UpdatedAt:        liveSessionControlString(state["lastControlUpdateAt"]),
		SucceededAt:      liveSessionControlString(state["lastControlSucceededAt"]),
		ErrorAt:          liveSessionControlString(state["lastControlErrorAt"]),
		ErrorCode:        liveSessionControlString(state["lastControlErrorCode"]),
		Error:            liveSessionControlString(state["lastControlError"]),
	}
	status.Pending = liveSessionControlStatusPending(status.DesiredStatus, status.ActualStatus)
	if status.Pending {
		if since, ok := liveSessionControlPendingSinceForStatus(state, status.ActualStatus); ok && !now.IsZero() && now.After(since) {
			status.PendingSeconds = int64(now.Sub(since).Seconds())
		}
	}
	if status.ErrorCode != "" || status.ActualStatus == "ERROR" {
		status.Hint = liveSessionControlErrorHint(status.ErrorCode)
	}
	return status
}

func liveSessionControlStatusPending(desired, actual string) bool {
	actual = strings.ToUpper(strings.TrimSpace(actual))
	if actual == "" || actual == "ERROR" {
		return false
	}
	if actual == "STARTING" || actual == "STOPPING" {
		return true
	}
	desired = strings.ToUpper(strings.TrimSpace(desired))
	return desired != "" && desired != actual
}

func liveSessionControlPendingSinceForStatus(state map[string]any, actual string) (time.Time, bool) {
	requestedAt, requestedOK := parseLiveSessionControlStatusTime(state["controlRequestedAt"])
	updatedAt, updatedOK := parseLiveSessionControlStatusTime(state["lastControlUpdateAt"])
	if strings.EqualFold(actual, "STARTING") || strings.EqualFold(actual, "STOPPING") {
		if updatedOK && (!requestedOK || updatedAt.After(requestedAt)) {
			return updatedAt, true
		}
	}
	if requestedOK {
		return requestedAt, true
	}
	return updatedAt, updatedOK
}

func parseLiveSessionControlStatusTime(value any) (time.Time, bool) {
	raw := liveSessionControlString(value)
	if raw == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err == nil {
		return parsed.UTC(), true
	}
	parsed, err = time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, false
	}
	return parsed.UTC(), true
}

func printLiveSessionControlStatuses(statuses []liveSessionControlStatus) {
	var out bytes.Buffer
	for i, status := range statuses {
		if i > 0 {
			out.WriteByte('\n')
		}
		label := status.ID
		if status.Alias != "" {
			label += " (" + status.Alias + ")"
		}
		fmt.Fprintf(&out, "%s\n", label)
		fmt.Fprintf(&out, "  status=%s desired=%s actual=%s pending=%t", status.Status, firstNonEmpty(status.DesiredStatus, "-"), firstNonEmpty(status.ActualStatus, "-"), status.Pending)
		if status.PendingSeconds > 0 {
			fmt.Fprintf(&out, " pendingSeconds=%d", status.PendingSeconds)
		}
		out.WriteByte('\n')
		fmt.Fprintf(&out, "  action=%s requestId=%s version=%s\n", firstNonEmpty(status.Action, "-"), firstNonEmpty(status.ControlRequestID, "-"), firstNonEmpty(status.ControlVersion, "-"))
		if status.RequestedAt != "" || status.UpdatedAt != "" || status.SucceededAt != "" || status.ErrorAt != "" {
			fmt.Fprintf(&out, "  requestedAt=%s updatedAt=%s succeededAt=%s errorAt=%s\n",
				firstNonEmpty(status.RequestedAt, "-"),
				firstNonEmpty(status.UpdatedAt, "-"),
				firstNonEmpty(status.SucceededAt, "-"),
				firstNonEmpty(status.ErrorAt, "-"),
			)
		}
		if status.ErrorCode != "" || status.Error != "" {
			fmt.Fprintf(&out, "  errorCode=%s error=%s\n", firstNonEmpty(status.ErrorCode, "-"), firstNonEmpty(status.Error, "-"))
			fmt.Fprintf(&out, "  hint=%s\n", firstNonEmpty(status.Hint, "check live-runner logs"))
		}
	}
	fmt.Print(out.String())
}
