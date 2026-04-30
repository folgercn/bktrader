package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

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
		handleSupervisorStatusResponse(resp, err)
		return nil
	},
}

type supervisorStatusSnapshot struct {
	CheckedAt string                     `json:"checkedAt"`
	Policy    *supervisorPolicy          `json:"policy,omitempty"`
	Targets   []supervisorTargetSnapshot `json:"targets"`
}

type supervisorPolicy struct {
	ApplicationRestartEnabled   bool   `json:"applicationRestartEnabled"`
	ServiceFailureThreshold     int    `json:"serviceFailureThreshold"`
	ContainerRestartEnabled     bool   `json:"containerRestartEnabled"`
	ContainerExecutorConfigured bool   `json:"containerExecutorConfigured"`
	ContainerExecutorKind       string `json:"containerExecutorKind"`
	ContainerExecutorDryRun     bool   `json:"containerExecutorDryRun"`
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
	ConsecutiveFailures                 int    `json:"consecutiveFailures"`
	FailureThreshold                    int    `json:"failureThreshold"`
	LastFailureReason                   string `json:"lastFailureReason,omitempty"`
	ContainerFallbackCandidate          bool   `json:"containerFallbackCandidate"`
	ContainerFallbackReason             string `json:"containerFallbackReason,omitempty"`
	ContainerFallbackSuppressed         bool   `json:"containerFallbackSuppressed"`
	ContainerFallbackBackoffUntil       string `json:"containerFallbackBackoffUntil,omitempty"`
	ContainerFallbackAttemptCount       int    `json:"containerFallbackAttemptCount"`
	LastContainerFallbackDecisionAt     string `json:"lastContainerFallbackDecisionAt,omitempty"`
	LastContainerFallbackDecisionReason string `json:"lastContainerFallbackDecisionReason,omitempty"`
}

type supervisorContainerFallbackPlan struct {
	Action             string `json:"action"`
	Candidate          bool   `json:"candidate"`
	Enabled            bool   `json:"enabled"`
	ExecutorConfigured bool   `json:"executorConfigured"`
	ExecutorKind       string `json:"executorKind"`
	ExecutorDryRun     bool   `json:"executorDryRun"`
	Executable         bool   `json:"executable"`
	Decision           string `json:"decision"`
	Suppressed         bool   `json:"suppressed"`
	BackoffActive      bool   `json:"backoffActive"`
	SafetyGateOK       bool   `json:"safetyGateOk"`
	BlockedReason      string `json:"blockedReason,omitempty"`
	EligibleReason     string `json:"eligibleReason,omitempty"`
	Reason             string `json:"reason,omitempty"`
}

type supervisorRuntimeStatusSnapshot struct {
	Service  string                    `json:"service"`
	Runtimes []supervisorRuntimeStatus `json:"runtimes"`
}

type supervisorRuntimeStatus struct {
	RuntimeID              string                            `json:"runtimeId"`
	RuntimeKind            string                            `json:"runtimeKind"`
	DesiredStatus          string                            `json:"desiredStatus"`
	ActualStatus           string                            `json:"actualStatus"`
	Health                 string                            `json:"health"`
	AutoRestartSuppressed  bool                              `json:"autoRestartSuppressed"`
	ApplicationRestartPlan *supervisorApplicationRestartPlan `json:"applicationRestartPlan,omitempty"`
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
		fmt.Fprintf(&out, "policy: applicationRestartEnabled=%t serviceFailureThreshold=%d containerRestartEnabled=%t containerExecutorConfigured=%t containerExecutorKind=%s containerExecutorDryRun=%t\n",
			snapshot.Policy.ApplicationRestartEnabled,
			snapshot.Policy.ServiceFailureThreshold,
			snapshot.Policy.ContainerRestartEnabled,
			snapshot.Policy.ContainerExecutorConfigured,
			firstNonEmpty(snapshot.Policy.ContainerExecutorKind, "--"),
			snapshot.Policy.ContainerExecutorDryRun,
		)
	}
	fmt.Fprintf(&out, "targets: total=%d fullyReachable=%d fallbackCandidates=%d fallbackExecutable=%d fallbackDryRun=%d runtimes=%d attention=%d controlActions=%d\n",
		targets, fullyReachable, fallbackCandidates, fallbackExecutable, fallbackDryRun, runtimeCount, runtimeAttention, controlActions)
	for _, target := range snapshot.Targets {
		fmt.Fprintf(&out, "\n- %s %s\n", firstNonEmpty(target.Name, "--"), firstNonEmpty(target.BaseURL, "--"))
		fmt.Fprintf(&out, "  probes: healthz=%s runtimeStatus=%s\n", supervisorProbeText(target.Healthz), supervisorProbeText(target.RuntimeStatus))
		fmt.Fprintf(&out, "  serviceState: failures=%d/%d fallback=%s attempts=%d suppressed=%t backoffUntil=%s\n",
			target.ServiceState.ConsecutiveFailures,
			target.ServiceState.FailureThreshold,
			supervisorFallbackStateText(target.ServiceState),
			target.ServiceState.ContainerFallbackAttemptCount,
			target.ServiceState.ContainerFallbackSuppressed,
			firstNonEmpty(target.ServiceState.ContainerFallbackBackoffUntil, "--"),
		)
		if strings.TrimSpace(target.ServiceState.LastFailureReason) != "" {
			fmt.Fprintf(&out, "  lastFailure=%s\n", target.ServiceState.LastFailureReason)
		}
		if strings.TrimSpace(target.ServiceState.LastContainerFallbackDecisionReason) != "" {
			fmt.Fprintf(&out, "  lastFallbackDecision=%s at=%s\n",
				target.ServiceState.LastContainerFallbackDecisionReason,
				firstNonEmpty(target.ServiceState.LastContainerFallbackDecisionAt, "--"),
			)
		}
		if target.ContainerFallbackPlan != nil {
			plan := target.ContainerFallbackPlan
			fmt.Fprintf(&out, "  fallbackPlan: action=%s decision=%s enabled=%t executorConfigured=%t executorKind=%s executorDryRun=%t executable=%t suppressed=%t backoffActive=%t safetyGateOk=%t blockedReason=%s eligibleReason=%s\n",
				firstNonEmpty(plan.Action, "--"),
				firstNonEmpty(plan.Decision, "--"),
				plan.Enabled,
				plan.ExecutorConfigured,
				firstNonEmpty(plan.ExecutorKind, "--"),
				plan.ExecutorDryRun,
				plan.Executable,
				plan.Suppressed,
				plan.BackoffActive,
				plan.SafetyGateOK,
				firstNonEmpty(plan.BlockedReason, "--"),
				firstNonEmpty(plan.EligibleReason, "--"),
			)
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
			if restartPlans > 0 {
				fmt.Fprintf(&out, "  runtimes: total=%d attention=%d restartPlans=%d restartEligible=%d service=%s\n", len(target.Status.Runtimes), attention, restartPlans, restartEligible, firstNonEmpty(target.Status.Service, "--"))
			} else {
				fmt.Fprintf(&out, "  runtimes: total=%d attention=%d service=%s\n", len(target.Status.Runtimes), attention, firstNonEmpty(target.Status.Service, "--"))
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
	return strings.TrimRight(out.String(), "\n") + "\n", nil
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
