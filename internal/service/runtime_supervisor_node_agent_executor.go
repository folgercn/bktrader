package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultRuntimeSupervisorNodeAgentTimeout = 30 * time.Second
	maxRuntimeSupervisorNodeAgentBodyBytes   = 4096
)

type NodeAgentContainerFallbackExecutorConfig struct {
	BaseURL        string
	Token          string
	Timeout        time.Duration
	AllowedTargets []string
	HTTPClient     *http.Client
}

type NodeAgentContainerFallbackExecutor struct {
	baseURL        string
	token          string
	timeout        time.Duration
	allowedTargets map[string]struct{}
	client         *http.Client
}

type NodeAgentHealth struct {
	Status             string    `json:"status"`
	Version            string    `json:"version,omitempty"`
	ExecutorKind       string    `json:"executorKind,omitempty"`
	TokenConfigured    bool      `json:"tokenConfigured"`
	AllowlistedTargets []string  `json:"allowlistedTargets,omitempty"`
	CheckedAt          time.Time `json:"checkedAt,omitempty"`
}

type nodeAgentRestartRequest struct {
	RequestID                       string `json:"requestId,omitempty"`
	TargetName                      string `json:"targetName"`
	Action                          string `json:"action"`
	Reason                          string `json:"reason"`
	PlanReason                      string `json:"planReason,omitempty"`
	ServiceFailureEpisodeStartedAt  string `json:"episodeStartedAt,omitempty"`
	ContainerFallbackCandidateSince string `json:"candidateSince,omitempty"`
	Source                          string `json:"source,omitempty"`
	Operator                        string `json:"operator,omitempty"`
}

type nodeAgentRestartResponse struct {
	RequestID    string `json:"requestId,omitempty"`
	TargetName   string `json:"targetName,omitempty"`
	Action       string `json:"action,omitempty"`
	ExecutorKind string `json:"executorKind,omitempty"`
	Executed     bool   `json:"executed"`
	ExitCode     *int   `json:"exitCode,omitempty"`
	TimedOut     bool   `json:"timedOut,omitempty"`
	Message      string `json:"message,omitempty"`
	Error        string `json:"error,omitempty"`
	StartedAt    string `json:"startedAt,omitempty"`
	FinishedAt   string `json:"finishedAt,omitempty"`
	DurationMs   int    `json:"durationMs,omitempty"`
}

func NewNodeAgentContainerFallbackExecutor(cfg NodeAgentContainerFallbackExecutorConfig) (NodeAgentContainerFallbackExecutor, error) {
	baseURL, err := normalizeNodeAgentBaseURL(cfg.BaseURL)
	if err != nil {
		return NodeAgentContainerFallbackExecutor{}, err
	}
	token := strings.TrimSpace(cfg.Token)
	if token == "" {
		return NodeAgentContainerFallbackExecutor{}, fmt.Errorf("node-agent token is required")
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultRuntimeSupervisorNodeAgentTimeout
	}
	if timeout > maxRuntimeSupervisorCommandTimeout {
		return NodeAgentContainerFallbackExecutor{}, fmt.Errorf("node-agent timeout must be <= %s", maxRuntimeSupervisorCommandTimeout)
	}
	allowedTargets := make(map[string]struct{}, len(cfg.AllowedTargets))
	for _, raw := range cfg.AllowedTargets {
		name := strings.TrimSpace(raw)
		if !validContainerFallbackCommandTargetName(name) {
			return NodeAgentContainerFallbackExecutor{}, fmt.Errorf("node-agent target name %q is invalid", raw)
		}
		if _, exists := allowedTargets[name]; exists {
			return NodeAgentContainerFallbackExecutor{}, fmt.Errorf("node-agent target %s is duplicated", name)
		}
		allowedTargets[name] = struct{}{}
	}
	if len(allowedTargets) == 0 {
		return NodeAgentContainerFallbackExecutor{}, fmt.Errorf("node-agent target allowlist is required")
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	return NodeAgentContainerFallbackExecutor{
		baseURL:        baseURL,
		token:          token,
		timeout:        timeout,
		allowedTargets: allowedTargets,
		client:         client,
	}, nil
}

func (e NodeAgentContainerFallbackExecutor) Configured() bool {
	return e.baseURL != "" && e.token != "" && len(e.allowedTargets) > 0
}

func (e NodeAgentContainerFallbackExecutor) Descriptor() ContainerFallbackExecutorDescriptor {
	return ContainerFallbackExecutorDescriptor{
		Kind:   runtimeSupervisorContainerExecutorKindNodeAgent,
		DryRun: false,
	}
}

func (e NodeAgentContainerFallbackExecutor) ContainerFallbackTargetAllowed(target RuntimeSupervisorTarget) bool {
	_, ok := e.allowedTargets[strings.TrimSpace(target.Name)]
	return ok
}

func (e NodeAgentContainerFallbackExecutor) ContainerFallbackExecutorPreview(target RuntimeSupervisorTarget) (*RuntimeSupervisorContainerFallbackExecutorPreview, bool) {
	if !e.ContainerFallbackTargetAllowed(target) {
		return nil, false
	}
	return &RuntimeSupervisorContainerFallbackExecutorPreview{
		Kind:           runtimeSupervisorContainerExecutorKindNodeAgent,
		TimeoutSeconds: int(e.timeout / time.Second),
	}, true
}

func (e NodeAgentContainerFallbackExecutor) Restart(ctx context.Context, target RuntimeSupervisorTarget, reason string) (ContainerFallbackExecutionResult, error) {
	return e.RestartWithRequest(ctx, ContainerFallbackExecutionRequest{
		Target: target,
		Action: "container-restart",
		Reason: reason,
	})
}

func (e NodeAgentContainerFallbackExecutor) RestartWithRequest(ctx context.Context, request ContainerFallbackExecutionRequest) (ContainerFallbackExecutionResult, error) {
	targetName := strings.TrimSpace(request.Target.Name)
	if _, ok := e.allowedTargets[targetName]; !ok {
		return ContainerFallbackExecutionResult{}, fmt.Errorf("node-agent target %q is not allowlisted", targetName)
	}
	reason := strings.TrimSpace(request.Reason)
	if reason == "" {
		return ContainerFallbackExecutionResult{}, fmt.Errorf("node-agent restart reason is required")
	}
	action := strings.TrimSpace(request.Action)
	if action == "" {
		action = "container-restart"
	}
	payload := nodeAgentRestartRequest{
		TargetName: targetName,
		Action:     action,
		Reason:     reason,
		PlanReason: strings.TrimSpace(request.PlanReason),
		Source:     strings.TrimSpace(request.Source),
		Operator:   strings.TrimSpace(request.Operator),
	}
	if !request.RequestedAt.IsZero() {
		payload.RequestID = fmt.Sprintf("%s-%d", targetName, request.RequestedAt.UTC().UnixNano())
	}
	if request.ServiceFailureEpisodeStartedAt != nil && !request.ServiceFailureEpisodeStartedAt.IsZero() {
		payload.ServiceFailureEpisodeStartedAt = request.ServiceFailureEpisodeStartedAt.UTC().Format(time.RFC3339Nano)
	}
	if request.ContainerFallbackCandidateSince != nil && !request.ContainerFallbackCandidateSince.IsZero() {
		payload.ContainerFallbackCandidateSince = request.ContainerFallbackCandidateSince.UTC().Format(time.RFC3339Nano)
	}
	var response nodeAgentRestartResponse
	statusCode, elapsed, err := e.doJSON(ctx, http.MethodPost, "/v1/container-fallback/restart", payload, &response)
	result := nodeAgentRestartResult(response, statusCode, elapsed)
	if err != nil {
		return result, err
	}
	if strings.TrimSpace(response.Error) != "" {
		return result, fmt.Errorf("node-agent restart failed: %s", strings.TrimSpace(response.Error))
	}
	if !response.Executed {
		return result, fmt.Errorf("node-agent restart did not execute")
	}
	return result, nil
}

func (e NodeAgentContainerFallbackExecutor) Health(ctx context.Context) (NodeAgentHealth, error) {
	var health NodeAgentHealth
	_, _, err := e.doJSON(ctx, http.MethodGet, "/v1/health", nil, &health)
	return health, err
}

func (e NodeAgentContainerFallbackExecutor) doJSON(ctx context.Context, method, path string, payload any, out any) (int, time.Duration, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	endpoint := strings.TrimRight(e.baseURL, "/") + path
	var body io.Reader
	if payload != nil {
		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(payload); err != nil {
			return 0, 0, err
		}
		body = &buf
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return 0, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+e.token)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	started := time.Now()
	resp, err := e.client.Do(req)
	elapsed := time.Since(started)
	if err != nil {
		return 0, elapsed, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxRuntimeSupervisorNodeAgentBodyBytes))
	if err != nil {
		return resp.StatusCode, elapsed, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		if len(data) > 0 && out != nil {
			_ = json.Unmarshal(data, out)
		}
		message := strings.TrimSpace(string(data))
		if message == "" {
			message = http.StatusText(resp.StatusCode)
		}
		return resp.StatusCode, elapsed, fmt.Errorf("node-agent http status %d: %s", resp.StatusCode, message)
	}
	if len(data) > 0 && out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			return resp.StatusCode, elapsed, fmt.Errorf("node-agent response JSON is invalid: %w", err)
		}
	}
	return resp.StatusCode, elapsed, nil
}

func nodeAgentRestartResult(response nodeAgentRestartResponse, statusCode int, elapsed time.Duration) ContainerFallbackExecutionResult {
	durationMs := response.DurationMs
	if durationMs <= 0 && elapsed > 0 {
		durationMs = int(elapsed / time.Millisecond)
	}
	result := ContainerFallbackExecutionResult{
		Executed:   response.Executed,
		Message:    strings.TrimSpace(response.Message),
		ExitCode:   response.ExitCode,
		TimedOut:   response.TimedOut,
		StatusCode: statusCode,
		DurationMs: durationMs,
	}
	if result.Message == "" && strings.TrimSpace(response.Error) != "" {
		result.Message = strings.TrimSpace(response.Error)
	}
	return result
}

func normalizeNodeAgentBaseURL(raw string) (string, error) {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	if raw == "" {
		return "", fmt.Errorf("node-agent base URL is required")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("node-agent base URL is invalid: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("node-agent base URL scheme must be http or https")
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return "", fmt.Errorf("node-agent base URL host is required")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("node-agent base URL must not include userinfo, query, or fragment")
	}
	return parsed.String(), nil
}
