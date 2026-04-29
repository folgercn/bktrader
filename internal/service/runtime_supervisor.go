package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	defaultRuntimeSupervisorHTTPTimeout       = 5 * time.Second
	defaultRuntimeSupervisorServiceFailThresh = 3
)

type RuntimeSupervisorOptions struct {
	EnableApplicationRestart bool
	ServiceFailureThreshold  int
	EnableContainerFallback  bool
}

type RuntimeSupervisorTarget struct {
	Name        string `json:"name"`
	BaseURL     string `json:"baseUrl"`
	BearerToken string `json:"-"`
}

type RuntimeSupervisorProbe struct {
	Path       string         `json:"path"`
	StatusCode int            `json:"statusCode,omitempty"`
	Reachable  bool           `json:"reachable"`
	Error      string         `json:"error,omitempty"`
	Payload    map[string]any `json:"payload,omitempty"`
}

type RuntimeSupervisorTargetSnapshot struct {
	Name                  string                                  `json:"name"`
	BaseURL               string                                  `json:"baseUrl"`
	CheckedAt             time.Time                               `json:"checkedAt"`
	Healthz               RuntimeSupervisorProbe                  `json:"healthz"`
	RuntimeStatus         RuntimeSupervisorProbe                  `json:"runtimeStatus"`
	ServiceState          RuntimeSupervisorServiceState           `json:"serviceState"`
	ContainerFallbackPlan *RuntimeSupervisorContainerFallbackPlan `json:"containerFallbackPlan,omitempty"`
	Status                *RuntimeStatusSnapshot                  `json:"status,omitempty"`
	ControlActions        []RuntimeSupervisorControlAction        `json:"controlActions,omitempty"`
}

type RuntimeSupervisorSnapshot struct {
	CheckedAt time.Time                         `json:"checkedAt"`
	Targets   []RuntimeSupervisorTargetSnapshot `json:"targets"`
}

type RuntimeSupervisorControlAction struct {
	Action      string    `json:"action"`
	Path        string    `json:"path"`
	RuntimeID   string    `json:"runtimeId"`
	RuntimeKind string    `json:"runtimeKind"`
	Reason      string    `json:"reason,omitempty"`
	Submitted   bool      `json:"submitted"`
	StatusCode  int       `json:"statusCode,omitempty"`
	Error       string    `json:"error,omitempty"`
	RequestedAt time.Time `json:"requestedAt"`
}

type RuntimeSupervisorServiceState struct {
	ConsecutiveFailures        int        `json:"consecutiveFailures"`
	FailureThreshold           int        `json:"failureThreshold"`
	LastFailureReason          string     `json:"lastFailureReason,omitempty"`
	LastFailureAt              *time.Time `json:"lastFailureAt,omitempty"`
	LastHealthyAt              *time.Time `json:"lastHealthyAt,omitempty"`
	ContainerFallbackCandidate bool       `json:"containerFallbackCandidate"`
	ContainerFallbackReason    string     `json:"containerFallbackReason,omitempty"`
}

type RuntimeSupervisorContainerFallbackPlan struct {
	Action        string `json:"action"`
	Candidate     bool   `json:"candidate"`
	Executable    bool   `json:"executable"`
	BlockedReason string `json:"blockedReason,omitempty"`
	Reason        string `json:"reason,omitempty"`
}

type runtimeSupervisorServiceState struct {
	ConsecutiveFailures int
	LastFailureReason   string
	LastFailureAt       time.Time
	LastHealthyAt       time.Time
}

type RuntimeSupervisor struct {
	targets []RuntimeSupervisorTarget
	client  *http.Client
	options RuntimeSupervisorOptions

	mu                sync.RWMutex
	snapshot          RuntimeSupervisorSnapshot
	submittedRestarts map[string]string
	serviceStates     map[string]runtimeSupervisorServiceState
}

func (p *Platform) SetRuntimeSupervisor(supervisor *RuntimeSupervisor) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.runtimeSupervisor = supervisor
}

func (p *Platform) RuntimeSupervisorSnapshot() (RuntimeSupervisorSnapshot, bool) {
	p.mu.Lock()
	supervisor := p.runtimeSupervisor
	p.mu.Unlock()
	if supervisor == nil {
		return RuntimeSupervisorSnapshot{}, false
	}
	return supervisor.LastSnapshot(), true
}

func NewRuntimeSupervisor(targets []RuntimeSupervisorTarget, client *http.Client) *RuntimeSupervisor {
	return NewRuntimeSupervisorWithOptions(targets, client, RuntimeSupervisorOptions{})
}

func NewRuntimeSupervisorWithOptions(targets []RuntimeSupervisorTarget, client *http.Client, options RuntimeSupervisorOptions) *RuntimeSupervisor {
	options = normalizeRuntimeSupervisorOptions(options)
	normalized := make([]RuntimeSupervisorTarget, 0, len(targets))
	for i, target := range targets {
		baseURL := strings.TrimRight(strings.TrimSpace(target.BaseURL), "/")
		if baseURL == "" {
			continue
		}
		name := strings.TrimSpace(target.Name)
		if name == "" {
			name = runtimeSupervisorTargetName(i, baseURL)
		}
		target.BaseURL = baseURL
		target.Name = name
		normalized = append(normalized, target)
	}
	if client == nil {
		client = &http.Client{Timeout: defaultRuntimeSupervisorHTTPTimeout}
	}
	return &RuntimeSupervisor{
		targets:           normalized,
		client:            client,
		options:           options,
		submittedRestarts: make(map[string]string),
		serviceStates:     make(map[string]runtimeSupervisorServiceState),
	}
}

func normalizeRuntimeSupervisorOptions(options RuntimeSupervisorOptions) RuntimeSupervisorOptions {
	if options.ServiceFailureThreshold <= 0 {
		options.ServiceFailureThreshold = defaultRuntimeSupervisorServiceFailThresh
	}
	return options
}

func ParseRuntimeSupervisorTargets(rawTargets []string, bearerToken ...string) []RuntimeSupervisorTarget {
	token := ""
	if len(bearerToken) > 0 {
		token = strings.TrimSpace(bearerToken[0])
	}
	targets := make([]RuntimeSupervisorTarget, 0, len(rawTargets))
	for _, raw := range rawTargets {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		target := RuntimeSupervisorTarget{BaseURL: raw}
		if name, baseURL, ok := strings.Cut(raw, "="); ok {
			target.Name = strings.TrimSpace(name)
			target.BaseURL = strings.TrimSpace(baseURL)
		}
		target.BearerToken = token
		targets = append(targets, target)
	}
	return targets
}

func (s *RuntimeSupervisor) Targets() []RuntimeSupervisorTarget {
	if s == nil {
		return nil
	}
	out := make([]RuntimeSupervisorTarget, len(s.targets))
	copy(out, s.targets)
	return out
}

func (s *RuntimeSupervisor) Collect(ctx context.Context) RuntimeSupervisorSnapshot {
	now := time.Now().UTC()
	snapshot := RuntimeSupervisorSnapshot{
		CheckedAt: now,
		Targets:   make([]RuntimeSupervisorTargetSnapshot, 0, len(s.targets)),
	}
	for _, target := range s.targets {
		targetSnapshot := RuntimeSupervisorTargetSnapshot{
			Name:      target.Name,
			BaseURL:   target.BaseURL,
			CheckedAt: now,
		}
		var healthPayload map[string]any
		targetSnapshot.Healthz = s.fetchJSON(ctx, target, "/healthz", &healthPayload)
		targetSnapshot.Healthz.Payload = healthPayload

		var status RuntimeStatusSnapshot
		targetSnapshot.RuntimeStatus = s.fetchJSON(ctx, target, "/api/v1/runtime/status", &status)
		targetSnapshot.ServiceState = s.updateServiceState(target, targetSnapshot.Healthz, targetSnapshot.RuntimeStatus, now)
		targetSnapshot.ContainerFallbackPlan = runtimeSupervisorContainerFallbackPlan(targetSnapshot.ServiceState, s.options)
		if targetSnapshot.RuntimeStatus.Error == "" && targetSnapshot.RuntimeStatus.Reachable {
			targetSnapshot.Status = &status
			targetSnapshot.ControlActions = s.submitApplicationRestarts(ctx, target, status, targetSnapshot.Healthz, now)
		}
		snapshot.Targets = append(snapshot.Targets, targetSnapshot)
	}
	s.mu.Lock()
	s.snapshot = snapshot
	s.mu.Unlock()
	return snapshot
}

func (s *RuntimeSupervisor) LastSnapshot() RuntimeSupervisorSnapshot {
	if s == nil {
		return RuntimeSupervisorSnapshot{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := s.snapshot
	if out.Targets != nil {
		out.Targets = append([]RuntimeSupervisorTargetSnapshot(nil), out.Targets...)
	}
	return out
}

func (s *RuntimeSupervisor) Start(ctx context.Context, interval time.Duration) {
	if s == nil {
		return
	}
	if interval <= 0 {
		interval = 30 * time.Second
	}
	logger := slog.Default().With("component", "service.runtime_supervisor")
	go func() {
		s.collectAndLog(ctx, logger)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.collectAndLog(ctx, logger)
			}
		}
	}()
}

func (s *RuntimeSupervisor) collectAndLog(ctx context.Context, logger *slog.Logger) {
	snapshot := s.Collect(ctx)
	unreachable := 0
	runtimeErrors := 0
	controlActions := 0
	containerFallbackCandidates := 0
	for _, target := range snapshot.Targets {
		if !target.Healthz.Reachable || target.Healthz.Error != "" {
			unreachable++
		}
		if target.RuntimeStatus.Error != "" {
			runtimeErrors++
		}
		controlActions += len(target.ControlActions)
		if target.ServiceState.ContainerFallbackCandidate {
			containerFallbackCandidates++
		}
	}
	logger.Info("runtime supervisor snapshot collected",
		"target_count", len(snapshot.Targets),
		"unreachable_count", unreachable,
		"runtime_error_count", runtimeErrors,
		"control_action_count", controlActions,
		"application_restart_enabled", s.options.EnableApplicationRestart,
		"service_failure_threshold", s.options.ServiceFailureThreshold,
		"container_restart_enabled", s.options.EnableContainerFallback,
		"container_fallback_candidate_count", containerFallbackCandidates,
	)
}

func (s *RuntimeSupervisor) updateServiceState(target RuntimeSupervisorTarget, healthz, runtimeStatus RuntimeSupervisorProbe, now time.Time) RuntimeSupervisorServiceState {
	if s == nil {
		return RuntimeSupervisorServiceState{FailureThreshold: defaultRuntimeSupervisorServiceFailThresh}
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	now = now.UTC()
	failed, reason := runtimeSupervisorServiceProbeFailure(healthz, runtimeStatus)
	key := runtimeSupervisorServiceKey(target)

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.serviceStates == nil {
		s.serviceStates = make(map[string]runtimeSupervisorServiceState)
	}
	state := s.serviceStates[key]
	if failed {
		state.ConsecutiveFailures++
		state.LastFailureReason = reason
		state.LastFailureAt = now
	} else {
		state.ConsecutiveFailures = 0
		state.LastFailureReason = ""
		state.LastHealthyAt = now
	}
	s.serviceStates[key] = state
	return runtimeSupervisorServiceStateSnapshot(state, s.options.ServiceFailureThreshold)
}

func runtimeSupervisorServiceProbeFailure(healthz, runtimeStatus RuntimeSupervisorProbe) (bool, string) {
	if !healthz.Reachable {
		return true, runtimeSupervisorProbeFailureReason("healthz-unreachable", healthz)
	}
	if healthz.Error != "" {
		return true, runtimeSupervisorProbeFailureReason("healthz-unhealthy", healthz)
	}
	if !runtimeStatus.Reachable {
		return true, runtimeSupervisorProbeFailureReason("runtime-status-unreachable", runtimeStatus)
	}
	return false, ""
}

func runtimeSupervisorProbeFailureReason(prefix string, probe RuntimeSupervisorProbe) string {
	err := strings.TrimSpace(probe.Error)
	if err == "" {
		return prefix
	}
	return prefix + ": " + err
}

func runtimeSupervisorServiceStateSnapshot(state runtimeSupervisorServiceState, threshold int) RuntimeSupervisorServiceState {
	if threshold <= 0 {
		threshold = defaultRuntimeSupervisorServiceFailThresh
	}
	out := RuntimeSupervisorServiceState{
		ConsecutiveFailures: state.ConsecutiveFailures,
		FailureThreshold:    threshold,
		LastFailureReason:   state.LastFailureReason,
	}
	if !state.LastFailureAt.IsZero() {
		lastFailureAt := state.LastFailureAt.UTC()
		out.LastFailureAt = &lastFailureAt
	}
	if !state.LastHealthyAt.IsZero() {
		lastHealthyAt := state.LastHealthyAt.UTC()
		out.LastHealthyAt = &lastHealthyAt
	}
	if state.ConsecutiveFailures >= threshold && state.LastFailureReason != "" {
		out.ContainerFallbackCandidate = true
		out.ContainerFallbackReason = fmt.Sprintf("service probes failed %d/%d: %s", state.ConsecutiveFailures, threshold, state.LastFailureReason)
	}
	return out
}

func runtimeSupervisorContainerFallbackPlan(state RuntimeSupervisorServiceState, options RuntimeSupervisorOptions) *RuntimeSupervisorContainerFallbackPlan {
	if !state.ContainerFallbackCandidate {
		return nil
	}
	blockedReason := "container-restart-disabled"
	if options.EnableContainerFallback {
		blockedReason = "container-executor-not-configured"
	}
	return &RuntimeSupervisorContainerFallbackPlan{
		Action:        "container-restart",
		Candidate:     true,
		Executable:    false,
		BlockedReason: blockedReason,
		Reason:        state.ContainerFallbackReason,
	}
}

func runtimeSupervisorServiceKey(target RuntimeSupervisorTarget) string {
	return strings.TrimSpace(target.Name) + "|" + strings.TrimSpace(target.BaseURL)
}

func (s *RuntimeSupervisor) submitApplicationRestarts(ctx context.Context, target RuntimeSupervisorTarget, status RuntimeStatusSnapshot, healthz RuntimeSupervisorProbe, now time.Time) []RuntimeSupervisorControlAction {
	if s == nil || !s.options.EnableApplicationRestart {
		return nil
	}
	if !healthz.Reachable || healthz.Error != "" {
		return nil
	}
	actions := make([]RuntimeSupervisorControlAction, 0)
	for _, runtime := range status.Runtimes {
		if !runtimeSupervisorRestartDue(runtime, now) {
			continue
		}
		if s.runtimeRestartAlreadySubmitted(target, runtime) {
			continue
		}
		action := s.submitRuntimeRestart(ctx, target, runtime, now)
		if action.Submitted {
			s.markRuntimeRestartSubmitted(target, runtime)
		}
		actions = append(actions, action)
	}
	return actions
}

func (s *RuntimeSupervisor) runtimeRestartAlreadySubmitted(target RuntimeSupervisorTarget, runtime RuntimeStatus) bool {
	if s == nil {
		return false
	}
	nextRestartAt := strings.TrimSpace(runtime.NextRestartAt)
	if nextRestartAt == "" {
		return false
	}
	key := runtimeSupervisorRestartKey(target, runtime)
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.submittedRestarts[key] == nextRestartAt
}

func (s *RuntimeSupervisor) markRuntimeRestartSubmitted(target RuntimeSupervisorTarget, runtime RuntimeStatus) {
	if s == nil {
		return
	}
	nextRestartAt := strings.TrimSpace(runtime.NextRestartAt)
	if nextRestartAt == "" {
		return
	}
	key := runtimeSupervisorRestartKey(target, runtime)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.submittedRestarts == nil {
		s.submittedRestarts = make(map[string]string)
	}
	s.submittedRestarts[key] = nextRestartAt
}

func runtimeSupervisorRestartKey(target RuntimeSupervisorTarget, runtime RuntimeStatus) string {
	return strings.TrimSpace(target.Name) + "|" + strings.TrimSpace(target.BaseURL) + "|" + strings.TrimSpace(runtime.RuntimeKind) + "|" + strings.TrimSpace(runtime.RuntimeID)
}

func runtimeSupervisorRestartDue(runtime RuntimeStatus, now time.Time) bool {
	if strings.TrimSpace(runtime.RuntimeID) == "" {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(runtime.RuntimeKind), "signal") {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(runtime.DesiredStatus), "RUNNING") {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(runtime.ActualStatus), "ERROR") {
		return false
	}
	if runtime.AutoRestartSuppressed {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(runtime.RestartSeverity), "fatal") {
		return false
	}
	nextRestartAt, ok := ParseRestartTime(map[string]any{"nextRestartAt": runtime.NextRestartAt}, "nextRestartAt")
	if !ok {
		return false
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return !nextRestartAt.After(now.UTC())
}

func (s *RuntimeSupervisor) submitRuntimeRestart(ctx context.Context, target RuntimeSupervisorTarget, runtime RuntimeStatus, now time.Time) RuntimeSupervisorControlAction {
	reason := runtimeSupervisorRestartReason(target, runtime)
	action := RuntimeSupervisorControlAction{
		Action:      "restart",
		Path:        "/api/v1/runtime/restart",
		RuntimeID:   runtime.RuntimeID,
		RuntimeKind: runtime.RuntimeKind,
		Reason:      reason,
		RequestedAt: now.UTC(),
	}
	payload := map[string]any{
		"runtimeId":   runtime.RuntimeID,
		"runtimeKind": runtime.RuntimeKind,
		"confirm":     true,
		"force":       false,
		"reason":      reason,
	}
	statusCode, err := s.postJSON(ctx, target, action.Path, payload)
	action.StatusCode = statusCode
	if err != nil {
		action.Error = err.Error()
		return action
	}
	action.Submitted = true
	return action
}

func runtimeSupervisorRestartReason(target RuntimeSupervisorTarget, runtime RuntimeStatus) string {
	reason := strings.TrimSpace(runtime.RestartReason)
	if reason == "" {
		reason = "runtime-error"
	}
	return fmt.Sprintf("supervisor scheduled application restart: target=%s runtime=%s reason=%s", strings.TrimSpace(target.Name), runtime.RuntimeID, reason)
}

func (s *RuntimeSupervisor) fetchJSON(ctx context.Context, target RuntimeSupervisorTarget, path string, out any) RuntimeSupervisorProbe {
	probe := RuntimeSupervisorProbe{Path: path}
	endpoint, err := runtimeSupervisorEndpoint(target.BaseURL, path)
	if err != nil {
		probe.Error = err.Error()
		return probe
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		probe.Error = err.Error()
		return probe
	}
	if token := strings.TrimSpace(target.BearerToken); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		probe.Error = err.Error()
		return probe
	}
	defer resp.Body.Close()
	probe.Reachable = true
	probe.StatusCode = resp.StatusCode
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		probe.Error = fmt.Sprintf("http status %d", resp.StatusCode)
		return probe
	}
	if out == nil {
		return probe
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		probe.Error = err.Error()
	}
	return probe
}

func (s *RuntimeSupervisor) postJSON(ctx context.Context, target RuntimeSupervisorTarget, path string, payload any) (int, error) {
	endpoint, err := runtimeSupervisorEndpoint(target.BaseURL, path)
	if err != nil {
		return 0, err
	}
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(payload); err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &body)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token := strings.TrimSpace(target.BearerToken); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return resp.StatusCode, fmt.Errorf("http status %d: %s", resp.StatusCode, strings.TrimSpace(string(message)))
	}
	return resp.StatusCode, nil
}

func runtimeSupervisorEndpoint(baseURL, path string) (string, error) {
	parsed, err := url.Parse(strings.TrimRight(strings.TrimSpace(baseURL), "/"))
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("unsupported supervisor target scheme: %s", parsed.Scheme)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("supervisor target host is required")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/" + strings.TrimLeft(path, "/")
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func runtimeSupervisorTargetName(index int, baseURL string) string {
	parsed, err := url.Parse(baseURL)
	if err == nil && strings.TrimSpace(parsed.Host) != "" {
		return parsed.Host
	}
	return fmt.Sprintf("target-%d", index+1)
}
