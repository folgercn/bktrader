package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type runtimeStatusSnapshot struct {
	Service   string              `json:"service"`
	CheckedAt string              `json:"checkedAt"`
	Runtimes  []runtimeStatusItem `json:"runtimes"`
}

type runtimeStatusItem struct {
	RuntimeID                   string `json:"runtimeId"`
	RuntimeKind                 string `json:"runtimeKind"`
	AccountID                   string `json:"accountId,omitempty"`
	StrategyID                  string `json:"strategyId,omitempty"`
	DesiredStatus               string `json:"desiredStatus,omitempty"`
	ActualStatus                string `json:"actualStatus,omitempty"`
	Health                      string `json:"health,omitempty"`
	RestartAttempt              int    `json:"restartAttempt"`
	NextRestartAt               string `json:"nextRestartAt,omitempty"`
	RestartBackoff              string `json:"restartBackoff,omitempty"`
	RestartReason               string `json:"restartReason,omitempty"`
	RestartSeverity             string `json:"restartSeverity,omitempty"`
	LastRestartError            string `json:"lastRestartError,omitempty"`
	RestartRequestedAt          string `json:"restartRequestedAt,omitempty"`
	RestartRequestedReason      string `json:"restartRequestedReason,omitempty"`
	RestartRequestedSource      string `json:"restartRequestedSource,omitempty"`
	RestartRequestedForce       bool   `json:"restartRequestedForce,omitempty"`
	StartRequestedAt            string `json:"startRequestedAt,omitempty"`
	StartRequestedReason        string `json:"startRequestedReason,omitempty"`
	StartRequestedSource        string `json:"startRequestedSource,omitempty"`
	StopRequestedAt             string `json:"stopRequestedAt,omitempty"`
	StopRequestedReason         string `json:"stopRequestedReason,omitempty"`
	StopRequestedSource         string `json:"stopRequestedSource,omitempty"`
	StopRequestedForce          bool   `json:"stopRequestedForce,omitempty"`
	AutoRestartSuppressed       bool   `json:"autoRestartSuppressed"`
	AutoRestartSuppressedAt     string `json:"autoRestartSuppressedAt,omitempty"`
	AutoRestartSuppressedReason string `json:"autoRestartSuppressedReason,omitempty"`
	AutoRestartSuppressedSource string `json:"autoRestartSuppressedSource,omitempty"`
	AutoRestartResumedAt        string `json:"autoRestartResumedAt,omitempty"`
	AutoRestartResumedReason    string `json:"autoRestartResumedReason,omitempty"`
	AutoRestartResumedSource    string `json:"autoRestartResumedSource,omitempty"`
	LastHealthyAt               string `json:"lastHealthyAt,omitempty"`
	LastCheckedAt               string `json:"lastCheckedAt,omitempty"`
	UpdatedAt                   string `json:"updatedAt,omitempty"`
}

func handleRuntimeStatusResponse(data []byte, err error) {
	if err != nil || outputJSON {
		handleResponse(data, err)
		return
	}
	summary, decodeErr := buildRuntimeStatusSummary(data)
	if decodeErr != nil {
		handleResponse(data, nil)
		return
	}
	fmt.Print(summary)
}

func buildRuntimeStatusSummary(data []byte) (string, error) {
	var snapshot runtimeStatusSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return "", err
	}
	attention := 0
	kindCounts := make(map[string]int)
	for _, runtime := range snapshot.Runtimes {
		if runtimeStatusNeedsAttention(runtime) {
			attention++
		}
		kindCounts[firstNonEmpty(strings.TrimSpace(runtime.RuntimeKind), "--")]++
	}

	var out bytes.Buffer
	fmt.Fprintln(&out, "Runtime status snapshot")
	fmt.Fprintf(&out, "service: %s\n", firstNonEmpty(snapshot.Service, "--"))
	fmt.Fprintf(&out, "checkedAt: %s\n", firstNonEmpty(snapshot.CheckedAt, "--"))
	fmt.Fprintf(&out, "runtimes: total=%d attention=%d byKind=%s%s%s\n",
		len(snapshot.Runtimes),
		attention,
		runtimeStatusKindSummary(kindCounts),
		runtimeStatusLifecycleAuditSummary(snapshot.Runtimes),
		runtimeStatusAutoRestartAuditSummary(snapshot.Runtimes),
	)
	for _, runtime := range snapshot.Runtimes {
		fmt.Fprintf(&out, "\n- %s %s\n", firstNonEmpty(runtime.RuntimeKind, "--"), firstNonEmpty(runtime.RuntimeID, "--"))
		fmt.Fprintf(&out, "  account=%s strategy=%s desired=%s actual=%s health=%s\n",
			firstNonEmpty(runtime.AccountID, "--"),
			firstNonEmpty(runtime.StrategyID, "--"),
			firstNonEmpty(runtime.DesiredStatus, "--"),
			firstNonEmpty(runtime.ActualStatus, "--"),
			firstNonEmpty(runtime.Health, "--"),
		)
		fmt.Fprintf(&out, "  restart: attempt=%d next=%s backoff=%s reason=%s severity=%s lastError=%s\n",
			runtime.RestartAttempt,
			firstNonEmpty(runtime.NextRestartAt, "--"),
			firstNonEmpty(runtime.RestartBackoff, "--"),
			firstNonEmpty(runtime.RestartReason, "--"),
			firstNonEmpty(runtime.RestartSeverity, "--"),
			firstNonEmpty(runtime.LastRestartError, "--"),
		)
		if runtimeHasLifecycleAudit(runtime) {
			fmt.Fprintf(&out, "  lifecycle: restartAt=%s restartSource=%s restartForce=%t startAt=%s startSource=%s stopAt=%s stopSource=%s stopForce=%t\n",
				firstNonEmpty(runtime.RestartRequestedAt, "--"),
				firstNonEmpty(runtime.RestartRequestedSource, "--"),
				runtime.RestartRequestedForce,
				firstNonEmpty(runtime.StartRequestedAt, "--"),
				firstNonEmpty(runtime.StartRequestedSource, "--"),
				firstNonEmpty(runtime.StopRequestedAt, "--"),
				firstNonEmpty(runtime.StopRequestedSource, "--"),
				runtime.StopRequestedForce,
			)
		}
		if runtime.AutoRestartSuppressed || runtimeHasAutoRestartAudit(runtime) {
			fmt.Fprintf(&out, "  autoRestart: suppressed=%t suppressAt=%s suppressSource=%s resumeAt=%s resumeSource=%s\n",
				runtime.AutoRestartSuppressed,
				firstNonEmpty(runtime.AutoRestartSuppressedAt, "--"),
				firstNonEmpty(runtime.AutoRestartSuppressedSource, "--"),
				firstNonEmpty(runtime.AutoRestartResumedAt, "--"),
				firstNonEmpty(runtime.AutoRestartResumedSource, "--"),
			)
		}
		fmt.Fprintf(&out, "  lastHealthy=%s lastChecked=%s updated=%s\n",
			firstNonEmpty(runtime.LastHealthyAt, "--"),
			firstNonEmpty(runtime.LastCheckedAt, "--"),
			firstNonEmpty(runtime.UpdatedAt, "--"),
		)
	}
	return strings.TrimRight(out.String(), "\n") + "\n", nil
}

func runtimeStatusNeedsAttention(runtime runtimeStatusItem) bool {
	if runtime.AutoRestartSuppressed {
		return true
	}
	actual := strings.ToUpper(strings.TrimSpace(runtime.ActualStatus))
	health := strings.ToLower(strings.TrimSpace(runtime.Health))
	return actual == "ERROR" || health == "error" || health == "suppressed" || health == "unreachable" || health == "stale"
}

func runtimeStatusKindSummary(counts map[string]int) string {
	if len(counts) == 0 {
		return "--"
	}
	kinds := make([]string, 0, len(counts))
	for kind := range counts {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	parts := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		parts = append(parts, fmt.Sprintf("%s:%d", kind, counts[kind]))
	}
	return strings.Join(parts, ",")
}

func runtimeStatusLifecycleAuditSummary(runtimes []runtimeStatusItem) string {
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

func runtimeStatusAutoRestartAuditSummary(runtimes []runtimeStatusItem) string {
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

func runtimeHasLifecycleAudit(runtime runtimeStatusItem) bool {
	return strings.TrimSpace(runtime.RestartRequestedAt) != "" ||
		strings.TrimSpace(runtime.RestartRequestedReason) != "" ||
		strings.TrimSpace(runtime.RestartRequestedSource) != "" ||
		runtime.RestartRequestedForce ||
		strings.TrimSpace(runtime.StartRequestedAt) != "" ||
		strings.TrimSpace(runtime.StartRequestedReason) != "" ||
		strings.TrimSpace(runtime.StartRequestedSource) != "" ||
		strings.TrimSpace(runtime.StopRequestedAt) != "" ||
		strings.TrimSpace(runtime.StopRequestedReason) != "" ||
		strings.TrimSpace(runtime.StopRequestedSource) != "" ||
		runtime.StopRequestedForce
}

func runtimeHasAutoRestartAudit(runtime runtimeStatusItem) bool {
	return strings.TrimSpace(runtime.AutoRestartSuppressedAt) != "" ||
		strings.TrimSpace(runtime.AutoRestartSuppressedReason) != "" ||
		strings.TrimSpace(runtime.AutoRestartSuppressedSource) != "" ||
		strings.TrimSpace(runtime.AutoRestartResumedAt) != "" ||
		strings.TrimSpace(runtime.AutoRestartResumedReason) != "" ||
		strings.TrimSpace(runtime.AutoRestartResumedSource) != ""
}
