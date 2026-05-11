package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

const maxSupervisorContainerFallbackBackoffSeconds = 24 * 60 * 60

func init() {
	supervisorCmd.AddCommand(supervisorStatusCmd)
	supervisorCmd.AddCommand(supervisorSuppressContainerFallbackCmd)
	supervisorCmd.AddCommand(supervisorResumeContainerFallbackCmd)
	supervisorCmd.AddCommand(supervisorDeferContainerFallbackCmd)
	supervisorCmd.AddCommand(supervisorClearContainerFallbackBackoffCmd)
	supervisorCmd.AddCommand(supervisorSubmitContainerFallbackCmd)
	rootCmd.AddCommand(supervisorCmd)
	supervisorSuppressContainerFallbackCmd.Flags().Bool("confirm", false, "确认抑制 container fallback")
	supervisorSuppressContainerFallbackCmd.Flags().String("reason", "", "抑制原因；必填")
	supervisorResumeContainerFallbackCmd.Flags().Bool("confirm", false, "确认恢复 container fallback")
	supervisorResumeContainerFallbackCmd.Flags().String("reason", "", "恢复原因；必填")
	supervisorDeferContainerFallbackCmd.Flags().Bool("confirm", false, "确认延后 container fallback")
	supervisorDeferContainerFallbackCmd.Flags().String("reason", "", "延后原因；必填")
	supervisorDeferContainerFallbackCmd.Flags().Int("seconds", 0, "延后秒数；必填且必须大于 0")
	supervisorClearContainerFallbackBackoffCmd.Flags().Bool("confirm", false, "确认清理 container fallback backoff")
	supervisorClearContainerFallbackBackoffCmd.Flags().String("reason", "", "清理原因；必填")
	supervisorSubmitContainerFallbackCmd.Flags().Bool("confirm", false, "确认提交 container fallback executor")
	supervisorSubmitContainerFallbackCmd.Flags().String("reason", "", "提交原因；必填")
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
		handleSupervisorStatusResponse(resp, err)
		return nil
	},
}

var supervisorSuppressContainerFallbackCmd = &cobra.Command{
	Use:   "suppress-container-fallback <targetName>",
	Short: "抑制 supervisor 容器兜底计划 [MUTATING]",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSupervisorContainerFallbackControl(cmd, args[0], "/api/v1/supervisor/container-fallback/suppress")
	},
}

var supervisorResumeContainerFallbackCmd = &cobra.Command{
	Use:   "resume-container-fallback <targetName>",
	Short: "恢复 supervisor 容器兜底计划 [MUTATING]",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSupervisorContainerFallbackControl(cmd, args[0], "/api/v1/supervisor/container-fallback/resume")
	},
}

var supervisorDeferContainerFallbackCmd = &cobra.Command{
	Use:   "defer-container-fallback <targetName>",
	Short: "为 supervisor 容器兜底计划设置 backoff [MUTATING]",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		seconds, _ := cmd.Flags().GetInt("seconds")
		return runSupervisorContainerFallbackControlWithBackoff(cmd, args[0], "/api/v1/supervisor/container-fallback/defer", seconds)
	},
}

var supervisorClearContainerFallbackBackoffCmd = &cobra.Command{
	Use:   "clear-container-fallback-backoff <targetName>",
	Short: "清理 supervisor 容器兜底 backoff [MUTATING]",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSupervisorContainerFallbackControl(cmd, args[0], "/api/v1/supervisor/container-fallback/clear-backoff")
	},
}

var supervisorSubmitContainerFallbackCmd = &cobra.Command{
	Use:   "submit-container-fallback <targetName>",
	Short: "显式提交 supervisor 容器兜底 executor [MUTATING]",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSupervisorContainerFallbackControl(cmd, args[0], "/api/v1/supervisor/container-fallback/submit")
	},
}

func runSupervisorContainerFallbackControl(cmd *cobra.Command, targetName, path string) error {
	return runSupervisorContainerFallbackControlWithBackoff(cmd, targetName, path, 0)
}

func runSupervisorContainerFallbackControlWithBackoff(cmd *cobra.Command, targetName, path string, backoffSeconds int) error {
	confirm, _ := cmd.Flags().GetBool("confirm")
	if !confirm && !dryRun {
		return fmt.Errorf("操作需要 --confirm 确认")
	}
	reason, _ := cmd.Flags().GetString("reason")
	if strings.TrimSpace(reason) == "" && !dryRun {
		return fmt.Errorf("操作需要提供 --reason")
	}
	payload := map[string]any{
		"targetName": targetName,
		"confirm":    confirm,
		"reason":     reason,
	}
	if backoffSeconds > 0 {
		if backoffSeconds > maxSupervisorContainerFallbackBackoffSeconds {
			return fmt.Errorf("--seconds 不能超过 86400")
		}
		payload["backoffSeconds"] = backoffSeconds
	} else if strings.Contains(path, "/defer") && !dryRun {
		return fmt.Errorf("操作需要提供 --seconds 且必须大于 0")
	}
	client := getClient()
	resp, err := client.Request("POST", path, payload)
	handleResponse(resp, err)
	return nil
}

type supervisorStatusSnapshot struct {
	CheckedAt                 string                                     `json:"checkedAt"`
	Policy                    *supervisorPolicy                          `json:"policy,omitempty"`
	Targets                   []supervisorTargetSnapshot                 `json:"targets"`
	ServiceFailureEpisodes    []supervisorServiceFailureEpisode          `json:"serviceFailureEpisodes,omitempty"`
	ContainerFallbackControls []supervisorContainerFallbackControlAction `json:"containerFallbackControls,omitempty"`
	ContainerFallbackActions  []supervisorContainerFallbackAction        `json:"containerFallbackActions,omitempty"`
}

type supervisorPolicy struct {
	ApplicationRestartEnabled   bool   `json:"applicationRestartEnabled"`
	ServiceFailureThreshold     int    `json:"serviceFailureThreshold"`
	ContainerRestartEnabled     bool   `json:"containerRestartEnabled"`
	ContainerFallbackAutoSubmit bool   `json:"containerFallbackAutoSubmit"`
	ContainerExecutorConfigured bool   `json:"containerExecutorConfigured"`
	ContainerExecutorKind       string `json:"containerExecutorKind"`
	ContainerExecutorDryRun     bool   `json:"containerExecutorDryRun"`
	ContainerExecutorArmed      bool   `json:"containerExecutorArmed"`
}

type supervisorTargetSnapshot struct {
	Name                  string                           `json:"name"`
	BaseURL               string                           `json:"baseUrl"`
	CheckedAt             string                           `json:"checkedAt"`
	Healthz               supervisorProbe                  `json:"healthz"`
	RuntimeStatus         supervisorProbe                  `json:"runtimeStatus"`
	ServiceState          supervisorServiceState           `json:"serviceState"`
	ContainerFallbackPlan *supervisorContainerFallbackPlan `json:"containerFallbackPlan,omitempty"`
	Status                *supervisorRuntimeStatusSnapshot `json:"status,omitempty"`
	ControlActions        []supervisorControlAction        `json:"controlActions,omitempty"`
}

type supervisorProbe struct {
	Path       string `json:"path"`
	StatusCode int    `json:"statusCode,omitempty"`
	Reachable  bool   `json:"reachable"`
	Error      string `json:"error,omitempty"`
}

type supervisorServiceState struct {
	ConsecutiveFailures                   int    `json:"consecutiveFailures"`
	FailureThreshold                      int    `json:"failureThreshold"`
	ServiceFailureEpisodeStartedAt        string `json:"serviceFailureEpisodeStartedAt,omitempty"`
	LastFailureReason                     string `json:"lastFailureReason,omitempty"`
	ContainerFallbackCandidate            bool   `json:"containerFallbackCandidate"`
	ContainerFallbackReason               string `json:"containerFallbackReason,omitempty"`
	ContainerFallbackCandidateSince       string `json:"containerFallbackCandidateSince,omitempty"`
	ContainerFallbackSuppressed           bool   `json:"containerFallbackSuppressed"`
	ContainerFallbackSuppressedAt         string `json:"containerFallbackSuppressedAt,omitempty"`
	ContainerFallbackSuppressedReason     string `json:"containerFallbackSuppressedReason,omitempty"`
	ContainerFallbackSuppressedSource     string `json:"containerFallbackSuppressedSource,omitempty"`
	ContainerFallbackResumedAt            string `json:"containerFallbackResumedAt,omitempty"`
	ContainerFallbackResumedReason        string `json:"containerFallbackResumedReason,omitempty"`
	ContainerFallbackResumedSource        string `json:"containerFallbackResumedSource,omitempty"`
	ContainerFallbackBackoffUntil         string `json:"containerFallbackBackoffUntil,omitempty"`
	ContainerFallbackBackoffSetAt         string `json:"containerFallbackBackoffSetAt,omitempty"`
	ContainerFallbackBackoffReason        string `json:"containerFallbackBackoffReason,omitempty"`
	ContainerFallbackBackoffSource        string `json:"containerFallbackBackoffSource,omitempty"`
	ContainerFallbackBackoffClearedAt     string `json:"containerFallbackBackoffClearedAt,omitempty"`
	ContainerFallbackBackoffClearedReason string `json:"containerFallbackBackoffClearedReason,omitempty"`
	ContainerFallbackBackoffClearedSource string `json:"containerFallbackBackoffClearedSource,omitempty"`
	ContainerFallbackAttemptCount         int    `json:"containerFallbackAttemptCount"`
	ContainerFallbackSubmitted            bool   `json:"containerFallbackSubmitted"`
	ContainerFallbackSubmittedAt          string `json:"containerFallbackSubmittedAt,omitempty"`
	ContainerFallbackSubmittedReason      string `json:"containerFallbackSubmittedReason,omitempty"`
	ContainerFallbackSubmittedMessage     string `json:"containerFallbackSubmittedMessage,omitempty"`
	ContainerFallbackSubmittedError       string `json:"containerFallbackSubmittedError,omitempty"`
	LastContainerFallbackDecisionAt       string `json:"lastContainerFallbackDecisionAt,omitempty"`
	LastContainerFallbackDecisionReason   string `json:"lastContainerFallbackDecisionReason,omitempty"`
}

type supervisorContainerFallbackPlan struct {
	Action                          string                                      `json:"action"`
	Candidate                       bool                                        `json:"candidate"`
	Enabled                         bool                                        `json:"enabled"`
	ServiceFailureEpisodeStartedAt  string                                      `json:"serviceFailureEpisodeStartedAt,omitempty"`
	ContainerFallbackCandidateSince string                                      `json:"containerFallbackCandidateSince,omitempty"`
	ExecutorConfigured              bool                                        `json:"executorConfigured"`
	ExecutorKind                    string                                      `json:"executorKind"`
	ExecutorDryRun                  bool                                        `json:"executorDryRun"`
	ExecutorArmed                   bool                                        `json:"executorArmed"`
	TargetAllowed                   bool                                        `json:"targetAllowed"`
	ExecutorPreview                 *supervisorContainerFallbackExecutorPreview `json:"executorPreview,omitempty"`
	Executable                      bool                                        `json:"executable"`
	AutoSubmitEnabled               bool                                        `json:"autoSubmitEnabled"`
	AutoSubmitEligible              bool                                        `json:"autoSubmitEligible"`
	ManualSubmitRequired            bool                                        `json:"manualSubmitRequired"`
	Decision                        string                                      `json:"decision"`
	Duplicate                       bool                                        `json:"duplicate"`
	Suppressed                      bool                                        `json:"suppressed"`
	BackoffActive                   bool                                        `json:"backoffActive"`
	SafetyGateOK                    bool                                        `json:"safetyGateOk"`
	BlockedReason                   string                                      `json:"blockedReason,omitempty"`
	EligibleReason                  string                                      `json:"eligibleReason,omitempty"`
	Reason                          string                                      `json:"reason,omitempty"`
}

type supervisorContainerFallbackExecutorPreview struct {
	Kind           string   `json:"kind"`
	CommandPath    string   `json:"commandPath,omitempty"`
	CommandArgs    []string `json:"commandArgs,omitempty"`
	TimeoutSeconds int      `json:"timeoutSeconds,omitempty"`
}

type supervisorRuntimeStatusSnapshot struct {
	Service  string                    `json:"service"`
	Runtimes []supervisorRuntimeStatus `json:"runtimes"`
}

type supervisorRuntimeStatus struct {
	RuntimeID                   string                            `json:"runtimeId"`
	RuntimeKind                 string                            `json:"runtimeKind"`
	DesiredStatus               string                            `json:"desiredStatus"`
	ActualStatus                string                            `json:"actualStatus"`
	Health                      string                            `json:"health"`
	RestartRequestedAt          string                            `json:"restartRequestedAt,omitempty"`
	RestartRequestedReason      string                            `json:"restartRequestedReason,omitempty"`
	RestartRequestedSource      string                            `json:"restartRequestedSource,omitempty"`
	RestartRequestedForce       bool                              `json:"restartRequestedForce,omitempty"`
	StartRequestedAt            string                            `json:"startRequestedAt,omitempty"`
	StartRequestedReason        string                            `json:"startRequestedReason,omitempty"`
	StartRequestedSource        string                            `json:"startRequestedSource,omitempty"`
	StopRequestedAt             string                            `json:"stopRequestedAt,omitempty"`
	StopRequestedReason         string                            `json:"stopRequestedReason,omitempty"`
	StopRequestedSource         string                            `json:"stopRequestedSource,omitempty"`
	StopRequestedForce          bool                              `json:"stopRequestedForce,omitempty"`
	AutoRestartSuppressed       bool                              `json:"autoRestartSuppressed"`
	AutoRestartSuppressedAt     string                            `json:"autoRestartSuppressedAt,omitempty"`
	AutoRestartSuppressedReason string                            `json:"autoRestartSuppressedReason,omitempty"`
	AutoRestartSuppressedSource string                            `json:"autoRestartSuppressedSource,omitempty"`
	AutoRestartResumedAt        string                            `json:"autoRestartResumedAt,omitempty"`
	AutoRestartResumedReason    string                            `json:"autoRestartResumedReason,omitempty"`
	AutoRestartResumedSource    string                            `json:"autoRestartResumedSource,omitempty"`
	ApplicationRestartPlan      *supervisorApplicationRestartPlan `json:"applicationRestartPlan,omitempty"`
}

type supervisorApplicationRestartPlan struct {
	Candidate      bool   `json:"candidate"`
	Enabled        bool   `json:"enabled"`
	HealthzOK      bool   `json:"healthzOk"`
	Supported      bool   `json:"supported"`
	Due            bool   `json:"due"`
	Duplicate      bool   `json:"duplicate"`
	Decision       string `json:"decision"`
	BlockedReason  string `json:"blockedReason,omitempty"`
	EligibleReason string `json:"eligibleReason,omitempty"`
}

type supervisorControlAction struct {
	Action    string `json:"action"`
	RuntimeID string `json:"runtimeId"`
	Error     string `json:"error,omitempty"`
}

type supervisorServiceFailureEpisode struct {
	TargetName                          string `json:"targetName"`
	TargetBaseURL                       string `json:"targetBaseUrl"`
	StartedAt                           string `json:"startedAt"`
	RecoveredAt                         string `json:"recoveredAt"`
	DurationSeconds                     int    `json:"durationSeconds"`
	MaxConsecutiveFailures              int    `json:"maxConsecutiveFailures"`
	LastFailureReason                   string `json:"lastFailureReason,omitempty"`
	LastFailureAt                       string `json:"lastFailureAt,omitempty"`
	ContainerFallbackCandidate          bool   `json:"containerFallbackCandidate"`
	ContainerFallbackCandidateSince     string `json:"containerFallbackCandidateSince,omitempty"`
	ContainerFallbackAttemptCount       int    `json:"containerFallbackAttemptCount"`
	ContainerFallbackSubmitted          bool   `json:"containerFallbackSubmitted"`
	ContainerFallbackSubmittedAt        string `json:"containerFallbackSubmittedAt,omitempty"`
	ContainerFallbackSubmittedError     string `json:"containerFallbackSubmittedError,omitempty"`
	ContainerFallbackBackoffUntil       string `json:"containerFallbackBackoffUntil,omitempty"`
	LastContainerFallbackDecisionAt     string `json:"lastContainerFallbackDecisionAt,omitempty"`
	LastContainerFallbackDecisionReason string `json:"lastContainerFallbackDecisionReason,omitempty"`
}

type supervisorContainerFallbackControlAction struct {
	Action         string `json:"action"`
	TargetName     string `json:"targetName"`
	TargetBaseURL  string `json:"targetBaseUrl"`
	Suppressed     bool   `json:"suppressed"`
	BackoffUntil   string `json:"backoffUntil,omitempty"`
	BackoffSeconds int    `json:"backoffSeconds,omitempty"`
	Reason         string `json:"reason"`
	Source         string `json:"source"`
	UpdatedAt      string `json:"updatedAt"`
}

type supervisorContainerFallbackAction struct {
	Action                          string                                      `json:"action"`
	TargetName                      string                                      `json:"targetName"`
	TargetBaseURL                   string                                      `json:"targetBaseUrl"`
	Reason                          string                                      `json:"reason,omitempty"`
	PlanReason                      string                                      `json:"planReason,omitempty"`
	Source                          string                                      `json:"source,omitempty"`
	ServiceFailureEpisodeStartedAt  string                                      `json:"serviceFailureEpisodeStartedAt,omitempty"`
	ContainerFallbackCandidateSince string                                      `json:"containerFallbackCandidateSince,omitempty"`
	ExecutorKind                    string                                      `json:"executorKind"`
	ExecutorDryRun                  bool                                        `json:"executorDryRun"`
	ExecutorPreview                 *supervisorContainerFallbackExecutorPreview `json:"executorPreview,omitempty"`
	Submitted                       bool                                        `json:"submitted"`
	Executed                        bool                                        `json:"executed"`
	BackoffUntil                    string                                      `json:"backoffUntil,omitempty"`
	BackoffSeconds                  int                                         `json:"backoffSeconds,omitempty"`
	Message                         string                                      `json:"message,omitempty"`
	Error                           string                                      `json:"error,omitempty"`
	RequestedAt                     string                                      `json:"requestedAt"`
}

func handleSupervisorStatusResponse(data []byte, err error) {
	if err != nil || outputJSON {
		handleResponse(data, err)
		return
	}
	summary, decodeErr := buildSupervisorStatusSummary(data)
	if decodeErr != nil {
		handleResponse(data, nil)
		return
	}
	fmt.Print(summary)
}

func buildSupervisorStatusSummary(data []byte) (string, error) {
	var snapshot supervisorStatusSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return "", err
	}
	targets := len(snapshot.Targets)
	fullyReachable := 0
	runtimeCount := 0
	runtimeAttention := 0
	fallbackCandidates := 0
	fallbackExecutable := 0
	fallbackDryRun := 0
	controlActions := 0

	for _, target := range snapshot.Targets {
		if supervisorProbeOK(target.Healthz) && supervisorProbeOK(target.RuntimeStatus) {
			fullyReachable++
		}
		if target.ServiceState.ContainerFallbackCandidate {
			fallbackCandidates++
		}
		if target.ContainerFallbackPlan != nil && target.ContainerFallbackPlan.Executable {
			fallbackExecutable++
			if target.ContainerFallbackPlan.ExecutorDryRun {
				fallbackDryRun++
			}
		}
		if target.Status != nil {
			runtimeCount += len(target.Status.Runtimes)
			for _, runtime := range target.Status.Runtimes {
				if supervisorRuntimeNeedsAttention(runtime) {
					runtimeAttention++
				}
			}
		}
		controlActions += len(target.ControlActions)
	}

	var out bytes.Buffer
	fmt.Fprintln(&out, "Runtime supervisor snapshot")
	fmt.Fprintf(&out, "checkedAt: %s\n", firstNonEmpty(snapshot.CheckedAt, "--"))
	if snapshot.Policy != nil {
		fmt.Fprintf(&out, "policy: applicationRestartEnabled=%t serviceFailureThreshold=%d containerRestartEnabled=%t containerFallbackAutoSubmit=%t containerExecutorConfigured=%t containerExecutorKind=%s containerExecutorDryRun=%t containerExecutorArmed=%t\n",
			snapshot.Policy.ApplicationRestartEnabled,
			snapshot.Policy.ServiceFailureThreshold,
			snapshot.Policy.ContainerRestartEnabled,
			snapshot.Policy.ContainerFallbackAutoSubmit,
			snapshot.Policy.ContainerExecutorConfigured,
			firstNonEmpty(snapshot.Policy.ContainerExecutorKind, "--"),
			snapshot.Policy.ContainerExecutorDryRun,
			snapshot.Policy.ContainerExecutorArmed,
		)
	}
	fmt.Fprintf(&out, "targets: total=%d fullyReachable=%d fallbackCandidates=%d fallbackExecutable=%d fallbackDryRun=%d runtimes=%d attention=%d controlActions=%d serviceFailureEpisodes=%d fallbackControls=%d fallbackActions=%d\n",
		targets, fullyReachable, fallbackCandidates, fallbackExecutable, fallbackDryRun, runtimeCount, runtimeAttention, controlActions, len(snapshot.ServiceFailureEpisodes), len(snapshot.ContainerFallbackControls), len(snapshot.ContainerFallbackActions))
	for _, target := range snapshot.Targets {
		fmt.Fprintf(&out, "\n- %s %s\n", firstNonEmpty(target.Name, "--"), firstNonEmpty(target.BaseURL, "--"))
		fmt.Fprintf(&out, "  probes: healthz=%s runtimeStatus=%s\n", supervisorProbeText(target.Healthz), supervisorProbeText(target.RuntimeStatus))
		fmt.Fprintf(&out, "  serviceState: failures=%d/%d episodeStartedAt=%s fallback=%s candidateSince=%s attempts=%d suppressed=%t backoffUntil=%s submitted=%t\n",
			target.ServiceState.ConsecutiveFailures,
			target.ServiceState.FailureThreshold,
			firstNonEmpty(target.ServiceState.ServiceFailureEpisodeStartedAt, "--"),
			supervisorFallbackStateText(target.ServiceState),
			firstNonEmpty(target.ServiceState.ContainerFallbackCandidateSince, "--"),
			target.ServiceState.ContainerFallbackAttemptCount,
			target.ServiceState.ContainerFallbackSuppressed,
			firstNonEmpty(target.ServiceState.ContainerFallbackBackoffUntil, "--"),
			target.ServiceState.ContainerFallbackSubmitted,
		)
		if strings.TrimSpace(target.ServiceState.LastFailureReason) != "" {
			fmt.Fprintf(&out, "  lastFailure=%s\n", target.ServiceState.LastFailureReason)
		}
		if strings.TrimSpace(target.ServiceState.ContainerFallbackSuppressedReason) != "" {
			fmt.Fprintf(&out, "  fallbackSuppressed reason=%s source=%s at=%s\n",
				target.ServiceState.ContainerFallbackSuppressedReason,
				firstNonEmpty(target.ServiceState.ContainerFallbackSuppressedSource, "--"),
				firstNonEmpty(target.ServiceState.ContainerFallbackSuppressedAt, "--"),
			)
		}
		if strings.TrimSpace(target.ServiceState.ContainerFallbackResumedReason) != "" {
			fmt.Fprintf(&out, "  fallbackResumed reason=%s source=%s at=%s\n",
				target.ServiceState.ContainerFallbackResumedReason,
				firstNonEmpty(target.ServiceState.ContainerFallbackResumedSource, "--"),
				firstNonEmpty(target.ServiceState.ContainerFallbackResumedAt, "--"),
			)
		}
		if strings.TrimSpace(target.ServiceState.ContainerFallbackBackoffReason) != "" {
			fmt.Fprintf(&out, "  fallbackBackoff reason=%s source=%s setAt=%s until=%s\n",
				target.ServiceState.ContainerFallbackBackoffReason,
				firstNonEmpty(target.ServiceState.ContainerFallbackBackoffSource, "--"),
				firstNonEmpty(target.ServiceState.ContainerFallbackBackoffSetAt, "--"),
				firstNonEmpty(target.ServiceState.ContainerFallbackBackoffUntil, "--"),
			)
		}
		if strings.TrimSpace(target.ServiceState.ContainerFallbackBackoffClearedReason) != "" {
			fmt.Fprintf(&out, "  fallbackBackoffCleared reason=%s source=%s at=%s\n",
				target.ServiceState.ContainerFallbackBackoffClearedReason,
				firstNonEmpty(target.ServiceState.ContainerFallbackBackoffClearedSource, "--"),
				firstNonEmpty(target.ServiceState.ContainerFallbackBackoffClearedAt, "--"),
			)
		}
		if target.ServiceState.ContainerFallbackSubmitted {
			fmt.Fprintf(&out, "  fallbackSubmitted at=%s reason=%s message=%s error=%s\n",
				firstNonEmpty(target.ServiceState.ContainerFallbackSubmittedAt, "--"),
				firstNonEmpty(target.ServiceState.ContainerFallbackSubmittedReason, "--"),
				firstNonEmpty(target.ServiceState.ContainerFallbackSubmittedMessage, "--"),
				firstNonEmpty(target.ServiceState.ContainerFallbackSubmittedError, "--"),
			)
		}
		if strings.TrimSpace(target.ServiceState.LastContainerFallbackDecisionReason) != "" {
			fmt.Fprintf(&out, "  lastFallbackDecision=%s at=%s\n",
				target.ServiceState.LastContainerFallbackDecisionReason,
				firstNonEmpty(target.ServiceState.LastContainerFallbackDecisionAt, "--"),
			)
		}
		if target.ContainerFallbackPlan != nil {
			plan := target.ContainerFallbackPlan
			fmt.Fprintf(&out, "  fallbackPlan: action=%s decision=%s enabled=%t executorConfigured=%t executorKind=%s executorDryRun=%t executorArmed=%t targetAllowed=%t executable=%t autoSubmitEnabled=%t autoSubmitEligible=%t manualSubmitRequired=%t duplicate=%t suppressed=%t backoffActive=%t safetyGateOk=%t blockedReason=%s eligibleReason=%s\n",
				firstNonEmpty(plan.Action, "--"),
				firstNonEmpty(plan.Decision, "--"),
				plan.Enabled,
				plan.ExecutorConfigured,
				firstNonEmpty(plan.ExecutorKind, "--"),
				plan.ExecutorDryRun,
				plan.ExecutorArmed,
				plan.TargetAllowed,
				plan.Executable,
				plan.AutoSubmitEnabled,
				plan.AutoSubmitEligible,
				plan.ManualSubmitRequired,
				plan.Duplicate,
				plan.Suppressed,
				plan.BackoffActive,
				plan.SafetyGateOK,
				firstNonEmpty(plan.BlockedReason, "--"),
				firstNonEmpty(plan.EligibleReason, "--"),
			)
			if preview := supervisorContainerFallbackExecutorPreviewSummary(plan.ExecutorPreview); preview != "" {
				fmt.Fprintf(&out, "  fallbackExecutorPreview: %s\n", preview)
			}
		}
		if target.Status != nil {
			attention := 0
			restartPlans := 0
			restartEligible := 0
			for _, runtime := range target.Status.Runtimes {
				if supervisorRuntimeNeedsAttention(runtime) {
					attention++
				}
				if runtime.ApplicationRestartPlan != nil {
					restartPlans++
					if runtime.ApplicationRestartPlan.Decision == "eligible" {
						restartEligible++
					}
				}
			}
			autoRestartAudit := supervisorRuntimeAutoRestartAuditSummary(target.Status.Runtimes)
			lifecycleAudit := supervisorRuntimeLifecycleAuditSummary(target.Status.Runtimes)
			if restartPlans > 0 {
				fmt.Fprintf(&out, "  runtimes: total=%d attention=%d restartPlans=%d restartEligible=%d restartBlockedReasons=%s service=%s%s%s\n",
					len(target.Status.Runtimes),
					attention,
					restartPlans,
					restartEligible,
					supervisorApplicationRestartBlockedReasons(target.Status.Runtimes),
					firstNonEmpty(target.Status.Service, "--"),
					lifecycleAudit,
					autoRestartAudit,
				)
			} else {
				fmt.Fprintf(&out, "  runtimes: total=%d attention=%d service=%s%s%s\n", len(target.Status.Runtimes), attention, firstNonEmpty(target.Status.Service, "--"), lifecycleAudit, autoRestartAudit)
			}
		}
		if len(target.ControlActions) > 0 {
			errors := 0
			for _, action := range target.ControlActions {
				if strings.TrimSpace(action.Error) != "" {
					errors++
				}
			}
			fmt.Fprintf(&out, "  controlActions: total=%d errors=%d\n", len(target.ControlActions), errors)
		}
	}
	if len(snapshot.ContainerFallbackControls) > 0 {
		fmt.Fprintf(&out, "\nfallbackControls: total=%d\n", len(snapshot.ContainerFallbackControls))
		for _, action := range snapshot.ContainerFallbackControls {
			fmt.Fprintf(&out, "  - %s target=%s suppressed=%t backoffUntil=%s backoffSeconds=%d source=%s updatedAt=%s reason=%s\n",
				firstNonEmpty(action.Action, "--"),
				firstNonEmpty(action.TargetName, "--"),
				action.Suppressed,
				firstNonEmpty(action.BackoffUntil, "--"),
				action.BackoffSeconds,
				firstNonEmpty(action.Source, "--"),
				firstNonEmpty(action.UpdatedAt, "--"),
				firstNonEmpty(action.Reason, "--"),
			)
		}
	}
	if len(snapshot.ServiceFailureEpisodes) > 0 {
		fmt.Fprintf(&out, "\nserviceFailureEpisodes: total=%d\n", len(snapshot.ServiceFailureEpisodes))
		for _, episode := range snapshot.ServiceFailureEpisodes {
			fmt.Fprintf(&out, "  - target=%s startedAt=%s recoveredAt=%s durationSeconds=%d maxFailures=%d candidate=%t candidateSince=%s attempts=%d submitted=%t submittedAt=%s lastDecision=%s lastFailure=%s",
				firstNonEmpty(episode.TargetName, "--"),
				firstNonEmpty(episode.StartedAt, "--"),
				firstNonEmpty(episode.RecoveredAt, "--"),
				episode.DurationSeconds,
				episode.MaxConsecutiveFailures,
				episode.ContainerFallbackCandidate,
				firstNonEmpty(episode.ContainerFallbackCandidateSince, "--"),
				episode.ContainerFallbackAttemptCount,
				episode.ContainerFallbackSubmitted,
				firstNonEmpty(episode.ContainerFallbackSubmittedAt, "--"),
				firstNonEmpty(episode.LastContainerFallbackDecisionReason, "--"),
				firstNonEmpty(episode.LastFailureReason, "--"),
			)
			if strings.TrimSpace(episode.ContainerFallbackSubmittedError) != "" {
				fmt.Fprintf(&out, " submittedError=%s", episode.ContainerFallbackSubmittedError)
			}
			if strings.TrimSpace(episode.ContainerFallbackBackoffUntil) != "" {
				fmt.Fprintf(&out, " backoffUntil=%s", episode.ContainerFallbackBackoffUntil)
			}
			fmt.Fprintln(&out)
		}
	}
	if len(snapshot.ContainerFallbackActions) > 0 {
		fmt.Fprintf(&out, "\nfallbackActions: total=%d\n", len(snapshot.ContainerFallbackActions))
		for _, action := range snapshot.ContainerFallbackActions {
			fmt.Fprintf(&out, "  - %s target=%s executorKind=%s executorDryRun=%t submitted=%t executed=%t requestedAt=%s episodeStartedAt=%s candidateSince=%s reason=%s",
				firstNonEmpty(action.Action, "--"),
				firstNonEmpty(action.TargetName, "--"),
				firstNonEmpty(action.ExecutorKind, "--"),
				action.ExecutorDryRun,
				action.Submitted,
				action.Executed,
				firstNonEmpty(action.RequestedAt, "--"),
				firstNonEmpty(action.ServiceFailureEpisodeStartedAt, "--"),
				firstNonEmpty(action.ContainerFallbackCandidateSince, "--"),
				firstNonEmpty(action.Reason, "--"),
			)
			if strings.TrimSpace(action.Error) != "" {
				fmt.Fprintf(&out, " error=%s", action.Error)
			}
			if strings.TrimSpace(action.BackoffUntil) != "" {
				fmt.Fprintf(&out, " backoffUntil=%s", action.BackoffUntil)
			}
			if action.BackoffSeconds > 0 {
				fmt.Fprintf(&out, " backoffSeconds=%d", action.BackoffSeconds)
			}
			if preview := supervisorContainerFallbackExecutorPreviewSummary(action.ExecutorPreview); preview != "" {
				fmt.Fprintf(&out, " executorPreview={%s}", preview)
			}
			if strings.TrimSpace(action.Message) != "" {
				fmt.Fprintf(&out, " message=%s", action.Message)
			}
			if strings.TrimSpace(action.Source) != "" {
				fmt.Fprintf(&out, " source=%s", action.Source)
			}
			if strings.TrimSpace(action.PlanReason) != "" {
				fmt.Fprintf(&out, " planReason=%s", action.PlanReason)
			}
			fmt.Fprintln(&out)
		}
	}
	return strings.TrimRight(out.String(), "\n") + "\n", nil
}

func supervisorContainerFallbackExecutorPreviewSummary(preview *supervisorContainerFallbackExecutorPreview) string {
	if preview == nil {
		return ""
	}
	args, err := json.Marshal(preview.CommandArgs)
	if err != nil {
		args = []byte("[]")
	}
	return fmt.Sprintf("kind=%s commandPath=%s commandArgs=%s timeoutSeconds=%d",
		firstNonEmpty(preview.Kind, "--"),
		firstNonEmpty(preview.CommandPath, "--"),
		string(args),
		preview.TimeoutSeconds,
	)
}

func supervisorProbeOK(probe supervisorProbe) bool {
	if !probe.Reachable || strings.TrimSpace(probe.Error) != "" {
		return false
	}
	return probe.StatusCode == 0 || (probe.StatusCode >= 200 && probe.StatusCode < 300)
}

func supervisorProbeText(probe supervisorProbe) string {
	status := "unreachable"
	if probe.Reachable {
		if probe.StatusCode > 0 {
			status = fmt.Sprintf("HTTP %d", probe.StatusCode)
		} else {
			status = "reachable"
		}
	}
	if strings.TrimSpace(probe.Error) != "" {
		return status + " error=" + probe.Error
	}
	return status
}

func supervisorFallbackStateText(state supervisorServiceState) string {
	if state.ContainerFallbackCandidate {
		return "candidate"
	}
	return "clear"
}

func supervisorRuntimeNeedsAttention(runtime supervisorRuntimeStatus) bool {
	if runtime.AutoRestartSuppressed {
		return true
	}
	actual := strings.ToUpper(strings.TrimSpace(runtime.ActualStatus))
	health := strings.ToLower(strings.TrimSpace(runtime.Health))
	return actual == "ERROR" || health == "error" || health == "suppressed" || health == "unreachable" || health == "stale"
}

func supervisorRuntimeLifecycleAuditSummary(runtimes []supervisorRuntimeStatus) string {
	restartAudit := 0
	startAudit := 0
	stopAudit := 0
	for _, runtime := range runtimes {
		if strings.TrimSpace(runtime.RestartRequestedAt) != "" ||
			strings.TrimSpace(runtime.RestartRequestedReason) != "" ||
			strings.TrimSpace(runtime.RestartRequestedSource) != "" ||
			runtime.RestartRequestedForce {
			restartAudit++
		}
		if strings.TrimSpace(runtime.StartRequestedAt) != "" ||
			strings.TrimSpace(runtime.StartRequestedReason) != "" ||
			strings.TrimSpace(runtime.StartRequestedSource) != "" {
			startAudit++
		}
		if strings.TrimSpace(runtime.StopRequestedAt) != "" ||
			strings.TrimSpace(runtime.StopRequestedReason) != "" ||
			strings.TrimSpace(runtime.StopRequestedSource) != "" ||
			runtime.StopRequestedForce {
			stopAudit++
		}
	}
	if restartAudit == 0 && startAudit == 0 && stopAudit == 0 {
		return ""
	}
	return fmt.Sprintf(" restartAudit=%d startAudit=%d stopAudit=%d", restartAudit, startAudit, stopAudit)
}

func supervisorRuntimeAutoRestartAuditSummary(runtimes []supervisorRuntimeStatus) string {
	suppressed := 0
	suppressAudit := 0
	resumeAudit := 0
	for _, runtime := range runtimes {
		if runtime.AutoRestartSuppressed {
			suppressed++
		}
		if strings.TrimSpace(runtime.AutoRestartSuppressedAt) != "" ||
			strings.TrimSpace(runtime.AutoRestartSuppressedReason) != "" ||
			strings.TrimSpace(runtime.AutoRestartSuppressedSource) != "" {
			suppressAudit++
		}
		if strings.TrimSpace(runtime.AutoRestartResumedAt) != "" ||
			strings.TrimSpace(runtime.AutoRestartResumedReason) != "" ||
			strings.TrimSpace(runtime.AutoRestartResumedSource) != "" {
			resumeAudit++
		}
	}
	if suppressed == 0 && suppressAudit == 0 && resumeAudit == 0 {
		return ""
	}
	return fmt.Sprintf(" autoRestartSuppressed=%d suppressAudit=%d resumeAudit=%d", suppressed, suppressAudit, resumeAudit)
}

func supervisorApplicationRestartBlockedReasons(runtimes []supervisorRuntimeStatus) string {
	counts := make(map[string]int)
	for _, runtime := range runtimes {
		if runtime.ApplicationRestartPlan == nil {
			continue
		}
		reason := strings.TrimSpace(runtime.ApplicationRestartPlan.BlockedReason)
		if reason == "" {
			continue
		}
		counts[reason]++
	}
	if len(counts) == 0 {
		return "--"
	}
	reasons := make([]string, 0, len(counts))
	for reason := range counts {
		reasons = append(reasons, reason)
	}
	sort.Strings(reasons)
	parts := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		parts = append(parts, fmt.Sprintf("%s:%d", reason, counts[reason]))
	}
	return strings.Join(parts, ",")
}
