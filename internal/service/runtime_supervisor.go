package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	defaultRuntimeSupervisorHTTPTimeout          = 5 * time.Second
	defaultRuntimeSupervisorServiceFailThresh    = 3
	defaultRuntimeSupervisorControlHistoryLimit  = 50
	defaultRuntimeSupervisorFallbackErrorBackoff = 5 * time.Minute
	maxRuntimeSupervisorContainerFallbackBackoff = 24 * time.Hour

	runtimeSupervisorApplicationRestartDecisionBlocked  = "blocked"
	runtimeSupervisorApplicationRestartDecisionEligible = "eligible"

	runtimeSupervisorContainerFallbackDecisionBlocked  = "blocked"
	runtimeSupervisorContainerFallbackDecisionEligible = "eligible"

	runtimeSupervisorContainerExecutorKindCustom  = "custom"
	runtimeSupervisorContainerExecutorKindNone    = "none"
	runtimeSupervisorContainerExecutorKindNoop    = "noop"
	runtimeSupervisorContainerExecutorKindCommand = "command"
)

var (
	ErrRuntimeSupervisorControlConfirmRequired   = errors.New("runtime supervisor control confirm required")
	ErrRuntimeSupervisorControlReasonRequired    = errors.New("runtime supervisor control reason required")
	ErrRuntimeSupervisorBackoffDurationRequired  = errors.New("runtime supervisor backoff duration must be positive")
	ErrRuntimeSupervisorBackoffDurationTooLarge  = errors.New("runtime supervisor backoff duration exceeds maximum")
	ErrRuntimeSupervisorTargetRequired           = errors.New("runtime supervisor target is required")
	ErrRuntimeSupervisorTargetNotFound           = errors.New("runtime supervisor target not found")
	ErrRuntimeSupervisorTargetAmbiguous          = errors.New("runtime supervisor target is ambiguous")
	ErrRuntimeSupervisorContainerFallbackBlocked = errors.New("runtime supervisor container fallback is blocked")
)

type RuntimeSupervisorContainerFallbackBlockedError struct {
	Reason string
}

func (e RuntimeSupervisorContainerFallbackBlockedError) Error() string {
	reason := strings.TrimSpace(e.Reason)
	if reason == "" {
		return ErrRuntimeSupervisorContainerFallbackBlocked.Error()
	}
	return ErrRuntimeSupervisorContainerFallbackBlocked.Error() + ": " + reason
}

func (e RuntimeSupervisorContainerFallbackBlockedError) Unwrap() error {
	return ErrRuntimeSupervisorContainerFallbackBlocked
}

type RuntimeSupervisorOptions struct {
	EnableApplicationRestart       bool
	ServiceFailureThreshold        int
	EnableContainerFallback        bool
	ContainerFallbackAutoSubmit    bool
	ContainerFallbackExecutor      ContainerFallbackExecutor
	ContainerFallbackExecutorArmed bool
}

type ContainerFallbackExecutorDescriptor struct {
	Kind   string `json:"kind"`
	DryRun bool   `json:"dryRun"`
}

type ContainerFallbackExecutor interface {
	Configured() bool
	Descriptor() ContainerFallbackExecutorDescriptor
	Restart(ctx context.Context, target RuntimeSupervisorTarget, reason string) (ContainerFallbackExecutionResult, error)
}

type ContainerFallbackTargetAllowlist interface {
	ContainerFallbackTargetAllowed(target RuntimeSupervisorTarget) bool
}

type ContainerFallbackExecutorPreviewer interface {
	ContainerFallbackExecutorPreview(target RuntimeSupervisorTarget) (*RuntimeSupervisorContainerFallbackExecutorPreview, bool)
}

type ContainerFallbackExecutionResult struct {
	Executed bool   `json:"executed"`
	Message  string `json:"message,omitempty"`
	ExitCode *int   `json:"exitCode,omitempty"`
	TimedOut bool   `json:"timedOut,omitempty"`
}

type NoopContainerFallbackExecutor struct {
	configured bool
}

func NewNoopContainerFallbackExecutor(configured bool) NoopContainerFallbackExecutor {
	return NoopContainerFallbackExecutor{configured: configured}
}

func (e NoopContainerFallbackExecutor) Configured() bool {
	return e.configured
}

func (e NoopContainerFallbackExecutor) Descriptor() ContainerFallbackExecutorDescriptor {
	return ContainerFallbackExecutorDescriptor{
		Kind:   runtimeSupervisorContainerExecutorKindNoop,
		DryRun: true,
	}
}

func (e NoopContainerFallbackExecutor) Restart(_ context.Context, _ RuntimeSupervisorTarget, _ string) (ContainerFallbackExecutionResult, error) {
	return ContainerFallbackExecutionResult{
		Executed: false,
		Message:  "noop container fallback executor",
	}, nil
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
	CheckedAt                 time.Time                                         `json:"checkedAt"`
	Policy                    RuntimeSupervisorPolicy                           `json:"policy"`
	Targets                   []RuntimeSupervisorTargetSnapshot                 `json:"targets"`
	ServiceFailureEpisodes    []RuntimeSupervisorServiceFailureEpisode          `json:"serviceFailureEpisodes,omitempty"`
	ContainerFallbackControls []RuntimeSupervisorContainerFallbackControlAction `json:"containerFallbackControls,omitempty"`
	ContainerFallbackActions  []RuntimeSupervisorContainerFallbackAction        `json:"containerFallbackActions,omitempty"`
}

type RuntimeSupervisorPolicy struct {
	ApplicationRestartEnabled   bool                                  `json:"applicationRestartEnabled"`
	ServiceFailureThreshold     int                                   `json:"serviceFailureThreshold"`
	ContainerRestartEnabled     bool                                  `json:"containerRestartEnabled"`
	ContainerFallbackAutoSubmit bool                                  `json:"containerFallbackAutoSubmit"`
	ContainerExecutorConfigured bool                                  `json:"containerExecutorConfigured"`
	ContainerExecutorKind       string                                `json:"containerExecutorKind"`
	ContainerExecutorDryRun     bool                                  `json:"containerExecutorDryRun"`
	ContainerExecutorArmed      bool                                  `json:"containerExecutorArmed"`
	DashboardPermissions        RuntimeSupervisorDashboardPermissions `json:"dashboardPermissions"`
}

type RuntimeSupervisorDashboardPermissions struct {
	CanView                              bool   `json:"canView"`
	CanRuntimeControl                    bool   `json:"canRuntimeControl"`
	CanContainerFallbackGate             bool   `json:"canContainerFallbackGate"`
	CanContainerFallbackSubmit           bool   `json:"canContainerFallbackSubmit"`
	ViewBlockedReason                    string `json:"viewBlockedReason,omitempty"`
	RuntimeControlBlockedReason          string `json:"runtimeControlBlockedReason,omitempty"`
	ContainerFallbackGateBlockedReason   string `json:"containerFallbackGateBlockedReason,omitempty"`
	ContainerFallbackSubmitBlockedReason string `json:"containerFallbackSubmitBlockedReason,omitempty"`
}

type RuntimeSupervisorControlAction struct {
	Action      string    `json:"action"`
	Path        string    `json:"path"`
	RuntimeID   string    `json:"runtimeId"`
	RuntimeKind string    `json:"runtimeKind"`
	Reason      string    `json:"reason,omitempty"`
	Source      string    `json:"source,omitempty"`
	Operator    string    `json:"operator,omitempty"`
	Submitted   bool      `json:"submitted"`
	StatusCode  int       `json:"statusCode,omitempty"`
	Error       string    `json:"error,omitempty"`
	RequestedAt time.Time `json:"requestedAt"`
}

type RuntimeSupervisorServiceFailureEpisode struct {
	TargetName                          string     `json:"targetName"`
	TargetBaseURL                       string     `json:"targetBaseUrl"`
	StartedAt                           time.Time  `json:"startedAt"`
	RecoveredAt                         time.Time  `json:"recoveredAt"`
	DurationSeconds                     int        `json:"durationSeconds"`
	MaxConsecutiveFailures              int        `json:"maxConsecutiveFailures"`
	LastFailureReason                   string     `json:"lastFailureReason,omitempty"`
	LastFailureAt                       *time.Time `json:"lastFailureAt,omitempty"`
	ContainerFallbackCandidate          bool       `json:"containerFallbackCandidate"`
	ContainerFallbackCandidateSince     *time.Time `json:"containerFallbackCandidateSince,omitempty"`
	ContainerFallbackAttemptCount       int        `json:"containerFallbackAttemptCount"`
	ContainerFallbackSubmitted          bool       `json:"containerFallbackSubmitted"`
	ContainerFallbackSubmittedAt        *time.Time `json:"containerFallbackSubmittedAt,omitempty"`
	ContainerFallbackSubmittedError     string     `json:"containerFallbackSubmittedError,omitempty"`
	ContainerFallbackBackoffUntil       *time.Time `json:"containerFallbackBackoffUntil,omitempty"`
	LastContainerFallbackDecisionAt     *time.Time `json:"lastContainerFallbackDecisionAt,omitempty"`
	LastContainerFallbackDecisionReason string     `json:"lastContainerFallbackDecisionReason,omitempty"`
}

type RuntimeSupervisorContainerFallbackControlAction struct {
	Action         string     `json:"action"`
	TargetName     string     `json:"targetName"`
	TargetBaseURL  string     `json:"targetBaseUrl"`
	Suppressed     bool       `json:"suppressed"`
	BackoffUntil   *time.Time `json:"backoffUntil,omitempty"`
	BackoffSeconds int        `json:"backoffSeconds,omitempty"`
	Reason         string     `json:"reason"`
	Source         string     `json:"source"`
	Operator       string     `json:"operator,omitempty"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

type RuntimeSupervisorContainerFallbackAction struct {
	Action                          string                                             `json:"action"`
	TargetName                      string                                             `json:"targetName"`
	TargetBaseURL                   string                                             `json:"targetBaseUrl"`
	Reason                          string                                             `json:"reason,omitempty"`
	PlanReason                      string                                             `json:"planReason,omitempty"`
	Source                          string                                             `json:"source,omitempty"`
	Operator                        string                                             `json:"operator,omitempty"`
	ServiceFailureEpisodeStartedAt  *time.Time                                         `json:"serviceFailureEpisodeStartedAt,omitempty"`
	ContainerFallbackCandidateSince *time.Time                                         `json:"containerFallbackCandidateSince,omitempty"`
	ExecutorKind                    string                                             `json:"executorKind"`
	ExecutorDryRun                  bool                                               `json:"executorDryRun"`
	ExecutorPreview                 *RuntimeSupervisorContainerFallbackExecutorPreview `json:"executorPreview,omitempty"`
	Submitted                       bool                                               `json:"submitted"`
	Executed                        bool                                               `json:"executed"`
	ExitCode                        *int                                               `json:"exitCode,omitempty"`
	TimedOut                        bool                                               `json:"timedOut,omitempty"`
	BackoffUntil                    *time.Time                                         `json:"backoffUntil,omitempty"`
	BackoffSeconds                  int                                                `json:"backoffSeconds,omitempty"`
	Message                         string                                             `json:"message,omitempty"`
	Error                           string                                             `json:"error,omitempty"`
	RequestedAt                     time.Time                                          `json:"requestedAt"`
}

type RuntimeSupervisorContainerFallbackExecutorPreview struct {
	Kind           string   `json:"kind"`
	CommandPath    string   `json:"commandPath,omitempty"`
	CommandArgs    []string `json:"commandArgs,omitempty"`
	TimeoutSeconds int      `json:"timeoutSeconds,omitempty"`
}

type RuntimeSupervisorApplicationRestartPlan struct {
	RuntimeID      string `json:"runtimeId"`
	RuntimeKind    string `json:"runtimeKind"`
	Candidate      bool   `json:"candidate"`
	Enabled        bool   `json:"enabled"`
	HealthzOK      bool   `json:"healthzOk"`
	Supported      bool   `json:"supported"`
	Due            bool   `json:"due"`
	Duplicate      bool   `json:"duplicate"`
	Decision       string `json:"decision"`
	BlockedReason  string `json:"blockedReason,omitempty"`
	EligibleReason string `json:"eligibleReason,omitempty"`
	Reason         string `json:"reason,omitempty"`
	NextRestartAt  string `json:"nextRestartAt,omitempty"`
}

type RuntimeSupervisorServiceState struct {
	ConsecutiveFailures                   int        `json:"consecutiveFailures"`
	FailureThreshold                      int        `json:"failureThreshold"`
	ServiceFailureEpisodeStartedAt        *time.Time `json:"serviceFailureEpisodeStartedAt,omitempty"`
	LastFailureReason                     string     `json:"lastFailureReason,omitempty"`
	LastFailureAt                         *time.Time `json:"lastFailureAt,omitempty"`
	LastHealthyAt                         *time.Time `json:"lastHealthyAt,omitempty"`
	ContainerFallbackCandidate            bool       `json:"containerFallbackCandidate"`
	ContainerFallbackReason               string     `json:"containerFallbackReason,omitempty"`
	ContainerFallbackCandidateSince       *time.Time `json:"containerFallbackCandidateSince,omitempty"`
	ContainerFallbackSuppressed           bool       `json:"containerFallbackSuppressed"`
	ContainerFallbackSuppressedAt         *time.Time `json:"containerFallbackSuppressedAt,omitempty"`
	ContainerFallbackSuppressedReason     string     `json:"containerFallbackSuppressedReason,omitempty"`
	ContainerFallbackSuppressedSource     string     `json:"containerFallbackSuppressedSource,omitempty"`
	ContainerFallbackResumedAt            *time.Time `json:"containerFallbackResumedAt,omitempty"`
	ContainerFallbackResumedReason        string     `json:"containerFallbackResumedReason,omitempty"`
	ContainerFallbackResumedSource        string     `json:"containerFallbackResumedSource,omitempty"`
	ContainerFallbackBackoffUntil         *time.Time `json:"containerFallbackBackoffUntil,omitempty"`
	ContainerFallbackBackoffSetAt         *time.Time `json:"containerFallbackBackoffSetAt,omitempty"`
	ContainerFallbackBackoffReason        string     `json:"containerFallbackBackoffReason,omitempty"`
	ContainerFallbackBackoffSource        string     `json:"containerFallbackBackoffSource,omitempty"`
	ContainerFallbackBackoffClearedAt     *time.Time `json:"containerFallbackBackoffClearedAt,omitempty"`
	ContainerFallbackBackoffClearedReason string     `json:"containerFallbackBackoffClearedReason,omitempty"`
	ContainerFallbackBackoffClearedSource string     `json:"containerFallbackBackoffClearedSource,omitempty"`
	ContainerFallbackAttemptCount         int        `json:"containerFallbackAttemptCount"`
	ContainerFallbackSubmitted            bool       `json:"containerFallbackSubmitted"`
	ContainerFallbackSubmittedAt          *time.Time `json:"containerFallbackSubmittedAt,omitempty"`
	ContainerFallbackSubmittedReason      string     `json:"containerFallbackSubmittedReason,omitempty"`
	ContainerFallbackSubmittedMessage     string     `json:"containerFallbackSubmittedMessage,omitempty"`
	ContainerFallbackSubmittedError       string     `json:"containerFallbackSubmittedError,omitempty"`
	LastContainerFallbackDecisionAt       *time.Time `json:"lastContainerFallbackDecisionAt,omitempty"`
	LastContainerFallbackDecisionReason   string     `json:"lastContainerFallbackDecisionReason,omitempty"`
}

type RuntimeSupervisorContainerFallbackPlan struct {
	Action                          string                                             `json:"action"`
	Candidate                       bool                                               `json:"candidate"`
	Enabled                         bool                                               `json:"enabled"`
	ServiceFailureEpisodeStartedAt  *time.Time                                         `json:"serviceFailureEpisodeStartedAt,omitempty"`
	ContainerFallbackCandidateSince *time.Time                                         `json:"containerFallbackCandidateSince,omitempty"`
	ExecutorConfigured              bool                                               `json:"executorConfigured"`
	ExecutorKind                    string                                             `json:"executorKind"`
	ExecutorDryRun                  bool                                               `json:"executorDryRun"`
	ExecutorArmed                   bool                                               `json:"executorArmed"`
	TargetAllowed                   bool                                               `json:"targetAllowed"`
	ExecutorPreview                 *RuntimeSupervisorContainerFallbackExecutorPreview `json:"executorPreview,omitempty"`
	Executable                      bool                                               `json:"executable"`
	AutoSubmitEnabled               bool                                               `json:"autoSubmitEnabled"`
	AutoSubmitEligible              bool                                               `json:"autoSubmitEligible"`
	ManualSubmitRequired            bool                                               `json:"manualSubmitRequired"`
	Decision                        string                                             `json:"decision"`
	Duplicate                       bool                                               `json:"duplicate"`
	Suppressed                      bool                                               `json:"suppressed"`
	BackoffActive                   bool                                               `json:"backoffActive"`
	SafetyGateOK                    bool                                               `json:"safetyGateOk"`
	BlockedReason                   string                                             `json:"blockedReason,omitempty"`
	EligibleReason                  string                                             `json:"eligibleReason,omitempty"`
	Reason                          string                                             `json:"reason,omitempty"`
}

type runtimeSupervisorServiceState struct {
	ConsecutiveFailures                   int
	ServiceFailureEpisodeStartedAt        time.Time
	LastFailureReason                     string
	LastFailureAt                         time.Time
	LastHealthyAt                         time.Time
	ContainerFallbackCandidateSince       time.Time
	ContainerFallbackSuppressed           bool
	ContainerFallbackSuppressedAt         time.Time
	ContainerFallbackSuppressedReason     string
	ContainerFallbackSuppressedSource     string
	ContainerFallbackResumedAt            time.Time
	ContainerFallbackResumedReason        string
	ContainerFallbackResumedSource        string
	ContainerFallbackBackoffUntil         time.Time
	ContainerFallbackBackoffSetAt         time.Time
	ContainerFallbackBackoffReason        string
	ContainerFallbackBackoffSource        string
	ContainerFallbackBackoffClearedAt     time.Time
	ContainerFallbackBackoffClearedReason string
	ContainerFallbackBackoffClearedSource string
	ContainerFallbackAttemptCount         int
	ContainerFallbackSubmittedAt          time.Time
	ContainerFallbackSubmittedReason      string
	ContainerFallbackSubmittedMessage     string
	ContainerFallbackSubmittedError       string
	LastContainerFallbackDecisionAt       time.Time
	LastContainerFallbackDecisionReason   string
}

type runtimeSupervisorContainerFallbackDecisionInput struct {
	Candidate          bool
	Enabled            bool
	ExecutorConfigured bool
	ExecutorDryRun     bool
	ExecutorArmed      bool
	TargetAllowed      bool
	Duplicate          bool
	Suppressed         bool
	BackoffActive      bool
	SafetyGateOK       bool
}

type runtimeSupervisorContainerFallbackDecisionResult struct {
	Decision       string
	Executable     bool
	BlockedReason  string
	EligibleReason string
}

type runtimeSupervisorApplicationRestartDecisionInput struct {
	Candidate            bool
	Enabled              bool
	HealthzOK            bool
	Supported            bool
	DesiredRunning       bool
	ActualError          bool
	Suppressed           bool
	Fatal                bool
	NextRestartAtPresent bool
	Due                  bool
	Duplicate            bool
}

type runtimeSupervisorApplicationRestartDecisionResult struct {
	Decision       string
	BlockedReason  string
	EligibleReason string
}

type RuntimeSupervisorContainerFallbackControlOptions struct {
	Confirm         bool
	Reason          string
	Source          string
	Operator        string
	BackoffDuration time.Duration
}

type RuntimeSupervisorContainerFallbackControlResult struct {
	TargetName    string                        `json:"targetName"`
	TargetBaseURL string                        `json:"targetBaseUrl"`
	Suppressed    bool                          `json:"suppressed"`
	BackoffUntil  *time.Time                    `json:"backoffUntil,omitempty"`
	Reason        string                        `json:"reason"`
	Source        string                        `json:"source"`
	Operator      string                        `json:"operator,omitempty"`
	UpdatedAt     time.Time                     `json:"updatedAt"`
	ServiceState  RuntimeSupervisorServiceState `json:"serviceState"`
}

type RuntimeSupervisorContainerFallbackSubmitResult struct {
	TargetName    string                                    `json:"targetName"`
	TargetBaseURL string                                    `json:"targetBaseUrl"`
	Reason        string                                    `json:"reason"`
	Source        string                                    `json:"source"`
	Operator      string                                    `json:"operator,omitempty"`
	UpdatedAt     time.Time                                 `json:"updatedAt"`
	ServiceState  RuntimeSupervisorServiceState             `json:"serviceState"`
	Plan          *RuntimeSupervisorContainerFallbackPlan   `json:"containerFallbackPlan,omitempty"`
	Action        *RuntimeSupervisorContainerFallbackAction `json:"action,omitempty"`
}

type runtimeSupervisorContainerFallbackSubmitAudit struct {
	Reason   string
	Source   string
	Operator string
}

type RuntimeSupervisor struct {
	targets []RuntimeSupervisorTarget
	client  *http.Client
	options RuntimeSupervisorOptions

	mu                 sync.RWMutex
	snapshot           RuntimeSupervisorSnapshot
	submittedRestarts  map[string]string
	submittedFallbacks map[string]bool
	serviceStates      map[string]runtimeSupervisorServiceState
	failureEpisodes    []RuntimeSupervisorServiceFailureEpisode
	fallbackControls   []RuntimeSupervisorContainerFallbackControlAction
	fallbackActions    []RuntimeSupervisorContainerFallbackAction
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

func (p *Platform) SuppressRuntimeSupervisorContainerFallback(targetName string, options RuntimeSupervisorContainerFallbackControlOptions) (RuntimeSupervisorContainerFallbackControlResult, error) {
	p.mu.Lock()
	supervisor := p.runtimeSupervisor
	p.mu.Unlock()
	if supervisor == nil {
		return RuntimeSupervisorContainerFallbackControlResult{}, ErrRuntimeSupervisorTargetNotFound
	}
	return supervisor.SuppressContainerFallback(targetName, options)
}

func (p *Platform) ResumeRuntimeSupervisorContainerFallback(targetName string, options RuntimeSupervisorContainerFallbackControlOptions) (RuntimeSupervisorContainerFallbackControlResult, error) {
	p.mu.Lock()
	supervisor := p.runtimeSupervisor
	p.mu.Unlock()
	if supervisor == nil {
		return RuntimeSupervisorContainerFallbackControlResult{}, ErrRuntimeSupervisorTargetNotFound
	}
	return supervisor.ResumeContainerFallback(targetName, options)
}

func (p *Platform) DeferRuntimeSupervisorContainerFallback(targetName string, options RuntimeSupervisorContainerFallbackControlOptions) (RuntimeSupervisorContainerFallbackControlResult, error) {
	p.mu.Lock()
	supervisor := p.runtimeSupervisor
	p.mu.Unlock()
	if supervisor == nil {
		return RuntimeSupervisorContainerFallbackControlResult{}, ErrRuntimeSupervisorTargetNotFound
	}
	return supervisor.DeferContainerFallback(targetName, options)
}

func (p *Platform) ClearRuntimeSupervisorContainerFallbackBackoff(targetName string, options RuntimeSupervisorContainerFallbackControlOptions) (RuntimeSupervisorContainerFallbackControlResult, error) {
	p.mu.Lock()
	supervisor := p.runtimeSupervisor
	p.mu.Unlock()
	if supervisor == nil {
		return RuntimeSupervisorContainerFallbackControlResult{}, ErrRuntimeSupervisorTargetNotFound
	}
	return supervisor.ClearContainerFallbackBackoff(targetName, options)
}

func (p *Platform) SubmitRuntimeSupervisorContainerFallback(ctx context.Context, targetName string, options RuntimeSupervisorContainerFallbackControlOptions) (RuntimeSupervisorContainerFallbackSubmitResult, error) {
	p.mu.Lock()
	supervisor := p.runtimeSupervisor
	p.mu.Unlock()
	if supervisor == nil {
		return RuntimeSupervisorContainerFallbackSubmitResult{}, ErrRuntimeSupervisorTargetNotFound
	}
	return supervisor.SubmitContainerFallback(ctx, targetName, options)
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
		targets:            normalized,
		client:             client,
		options:            options,
		submittedRestarts:  make(map[string]string),
		submittedFallbacks: make(map[string]bool),
		serviceStates:      make(map[string]runtimeSupervisorServiceState),
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
		Policy:    runtimeSupervisorPolicy(s.options),
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
		targetSnapshot.ContainerFallbackPlan, targetSnapshot.ServiceState = s.updateContainerFallbackPlan(target, targetSnapshot.ServiceState, now)
		if s.options.ContainerFallbackAutoSubmit && s.submitContainerFallback(ctx, target, targetSnapshot.ContainerFallbackPlan, now) {
			targetSnapshot.ServiceState = s.serviceStateSnapshot(target)
			targetSnapshot.ContainerFallbackPlan = runtimeSupervisorContainerFallbackPlan(target, targetSnapshot.ServiceState, s.options, now)
		}
		if targetSnapshot.RuntimeStatus.Error == "" && targetSnapshot.RuntimeStatus.Reachable {
			status = s.attachApplicationRestartPlans(target, status, targetSnapshot.Healthz, now)
			targetSnapshot.Status = &status
			targetSnapshot.ControlActions = s.submitApplicationRestarts(ctx, target, status, targetSnapshot.Healthz, now)
		}
		snapshot.Targets = append(snapshot.Targets, targetSnapshot)
	}
	s.mu.Lock()
	snapshot.ServiceFailureEpisodes = append([]RuntimeSupervisorServiceFailureEpisode(nil), s.failureEpisodes...)
	snapshot.ContainerFallbackControls = append([]RuntimeSupervisorContainerFallbackControlAction(nil), s.fallbackControls...)
	snapshot.ContainerFallbackActions = append([]RuntimeSupervisorContainerFallbackAction(nil), s.fallbackActions...)
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
		now := time.Now().UTC()
		for i := range out.Targets {
			target := RuntimeSupervisorTarget{
				Name:    out.Targets[i].Name,
				BaseURL: out.Targets[i].BaseURL,
			}
			state, ok := s.serviceStates[runtimeSupervisorServiceKey(target)]
			if !ok {
				continue
			}
			serviceState := runtimeSupervisorServiceStateSnapshot(state, s.options.ServiceFailureThreshold)
			out.Targets[i].ServiceState = serviceState
			out.Targets[i].ContainerFallbackPlan = runtimeSupervisorContainerFallbackPlan(target, serviceState, s.options, now)
		}
	}
	out.ServiceFailureEpisodes = append([]RuntimeSupervisorServiceFailureEpisode(nil), s.failureEpisodes...)
	out.ContainerFallbackControls = append([]RuntimeSupervisorContainerFallbackControlAction(nil), s.fallbackControls...)
	out.ContainerFallbackActions = append([]RuntimeSupervisorContainerFallbackAction(nil), s.fallbackActions...)
	return out
}

func (s *RuntimeSupervisor) serviceStateSnapshot(target RuntimeSupervisorTarget) RuntimeSupervisorServiceState {
	if s == nil {
		return RuntimeSupervisorServiceState{FailureThreshold: defaultRuntimeSupervisorServiceFailThresh}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return runtimeSupervisorServiceStateSnapshot(s.serviceStates[runtimeSupervisorServiceKey(target)], s.options.ServiceFailureThreshold)
}

func (s *RuntimeSupervisor) SuppressContainerFallback(targetName string, options RuntimeSupervisorContainerFallbackControlOptions) (RuntimeSupervisorContainerFallbackControlResult, error) {
	return s.setContainerFallbackSuppressed(targetName, true, options)
}

func (s *RuntimeSupervisor) ResumeContainerFallback(targetName string, options RuntimeSupervisorContainerFallbackControlOptions) (RuntimeSupervisorContainerFallbackControlResult, error) {
	return s.setContainerFallbackSuppressed(targetName, false, options)
}

func (s *RuntimeSupervisor) DeferContainerFallback(targetName string, options RuntimeSupervisorContainerFallbackControlOptions) (RuntimeSupervisorContainerFallbackControlResult, error) {
	return s.setContainerFallbackBackoff(targetName, options, false)
}

func (s *RuntimeSupervisor) ClearContainerFallbackBackoff(targetName string, options RuntimeSupervisorContainerFallbackControlOptions) (RuntimeSupervisorContainerFallbackControlResult, error) {
	return s.setContainerFallbackBackoff(targetName, options, true)
}

func (s *RuntimeSupervisor) SubmitContainerFallback(ctx context.Context, targetName string, options RuntimeSupervisorContainerFallbackControlOptions) (RuntimeSupervisorContainerFallbackSubmitResult, error) {
	if s == nil {
		return RuntimeSupervisorContainerFallbackSubmitResult{}, ErrRuntimeSupervisorTargetNotFound
	}
	if !options.Confirm {
		return RuntimeSupervisorContainerFallbackSubmitResult{}, ErrRuntimeSupervisorControlConfirmRequired
	}
	reason := strings.TrimSpace(options.Reason)
	if reason == "" {
		return RuntimeSupervisorContainerFallbackSubmitResult{}, ErrRuntimeSupervisorControlReasonRequired
	}
	target, err := s.targetByName(targetName)
	if err != nil {
		return RuntimeSupervisorContainerFallbackSubmitResult{}, err
	}
	source := strings.TrimSpace(options.Source)
	if source == "" {
		source = "api"
	}
	operator := strings.TrimSpace(options.Operator)
	now := time.Now().UTC()
	key := runtimeSupervisorServiceKey(target)

	s.mu.Lock()
	if s.serviceStates == nil {
		s.serviceStates = make(map[string]runtimeSupervisorServiceState)
	}
	state := s.serviceStates[key]
	if state.ConsecutiveFailures >= s.options.ServiceFailureThreshold && state.LastFailureReason != "" {
		state.ContainerFallbackAttemptCount++
	}
	serviceState := runtimeSupervisorServiceStateSnapshot(state, s.options.ServiceFailureThreshold)
	plan := runtimeSupervisorContainerFallbackPlan(target, serviceState, s.options, now)
	if plan != nil {
		decisionReason := runtimeSupervisorContainerFallbackDecisionReason(plan)
		state.LastContainerFallbackDecisionAt = now
		state.LastContainerFallbackDecisionReason = decisionReason
		s.serviceStates[key] = state
		serviceState = runtimeSupervisorServiceStateSnapshot(state, s.options.ServiceFailureThreshold)
		plan = runtimeSupervisorContainerFallbackPlan(target, serviceState, s.options, now)
	} else {
		s.serviceStates[key] = state
	}
	s.mu.Unlock()

	result := RuntimeSupervisorContainerFallbackSubmitResult{
		TargetName:    target.Name,
		TargetBaseURL: target.BaseURL,
		Reason:        reason,
		Source:        source,
		Operator:      operator,
		UpdatedAt:     now,
		ServiceState:  serviceState,
		Plan:          plan,
	}
	if plan == nil {
		return result, RuntimeSupervisorContainerFallbackBlockedError{Reason: "container-fallback-not-candidate"}
	}
	if !plan.Executable {
		return result, RuntimeSupervisorContainerFallbackBlockedError{Reason: runtimeSupervisorContainerFallbackDecisionReason(plan)}
	}
	action, submitted := s.submitContainerFallbackAction(ctx, target, plan, now, runtimeSupervisorContainerFallbackSubmitAudit{
		Reason:   reason,
		Source:   source,
		Operator: operator,
	})
	if !submitted {
		result.ServiceState = s.serviceStateSnapshot(target)
		result.Plan = runtimeSupervisorContainerFallbackPlan(target, result.ServiceState, s.options, now)
		return result, RuntimeSupervisorContainerFallbackBlockedError{Reason: "container-fallback-already-submitted"}
	}
	result.Action = &action
	result.ServiceState = s.serviceStateSnapshot(target)
	result.Plan = runtimeSupervisorContainerFallbackPlan(target, result.ServiceState, s.options, now)
	return result, nil
}

func (s *RuntimeSupervisor) setContainerFallbackSuppressed(targetName string, suppressed bool, options RuntimeSupervisorContainerFallbackControlOptions) (RuntimeSupervisorContainerFallbackControlResult, error) {
	if s == nil {
		return RuntimeSupervisorContainerFallbackControlResult{}, ErrRuntimeSupervisorTargetNotFound
	}
	if !options.Confirm {
		return RuntimeSupervisorContainerFallbackControlResult{}, ErrRuntimeSupervisorControlConfirmRequired
	}
	reason := strings.TrimSpace(options.Reason)
	if reason == "" {
		return RuntimeSupervisorContainerFallbackControlResult{}, ErrRuntimeSupervisorControlReasonRequired
	}
	target, err := s.targetByName(targetName)
	if err != nil {
		return RuntimeSupervisorContainerFallbackControlResult{}, err
	}
	source := strings.TrimSpace(options.Source)
	if source == "" {
		source = "api"
	}
	operator := strings.TrimSpace(options.Operator)
	now := time.Now().UTC()
	key := runtimeSupervisorServiceKey(target)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.serviceStates == nil {
		s.serviceStates = make(map[string]runtimeSupervisorServiceState)
	}
	state := s.serviceStates[key]
	state.ContainerFallbackSuppressed = suppressed
	if suppressed {
		state.ContainerFallbackSuppressedAt = now
		state.ContainerFallbackSuppressedReason = reason
		state.ContainerFallbackSuppressedSource = source
	} else {
		state.ContainerFallbackSuppressedAt = time.Time{}
		state.ContainerFallbackSuppressedReason = ""
		state.ContainerFallbackSuppressedSource = ""
		state.ContainerFallbackResumedAt = now
		state.ContainerFallbackResumedReason = reason
		state.ContainerFallbackResumedSource = source
	}
	s.serviceStates[key] = state
	action := "suppress-container-fallback"
	if !suppressed {
		action = "resume-container-fallback"
	}
	s.recordContainerFallbackControlLocked(RuntimeSupervisorContainerFallbackControlAction{
		Action:        action,
		TargetName:    target.Name,
		TargetBaseURL: target.BaseURL,
		Suppressed:    suppressed,
		Reason:        reason,
		Source:        source,
		Operator:      operator,
		UpdatedAt:     now,
	})
	return RuntimeSupervisorContainerFallbackControlResult{
		TargetName:    target.Name,
		TargetBaseURL: target.BaseURL,
		Suppressed:    suppressed,
		Reason:        reason,
		Source:        source,
		Operator:      operator,
		UpdatedAt:     now,
		ServiceState:  runtimeSupervisorServiceStateSnapshot(state, s.options.ServiceFailureThreshold),
	}, nil
}

func (s *RuntimeSupervisor) setContainerFallbackBackoff(targetName string, options RuntimeSupervisorContainerFallbackControlOptions, clear bool) (RuntimeSupervisorContainerFallbackControlResult, error) {
	if s == nil {
		return RuntimeSupervisorContainerFallbackControlResult{}, ErrRuntimeSupervisorTargetNotFound
	}
	if !options.Confirm {
		return RuntimeSupervisorContainerFallbackControlResult{}, ErrRuntimeSupervisorControlConfirmRequired
	}
	reason := strings.TrimSpace(options.Reason)
	if reason == "" {
		return RuntimeSupervisorContainerFallbackControlResult{}, ErrRuntimeSupervisorControlReasonRequired
	}
	if !clear && options.BackoffDuration <= 0 {
		return RuntimeSupervisorContainerFallbackControlResult{}, ErrRuntimeSupervisorBackoffDurationRequired
	}
	if !clear && options.BackoffDuration > maxRuntimeSupervisorContainerFallbackBackoff {
		return RuntimeSupervisorContainerFallbackControlResult{}, ErrRuntimeSupervisorBackoffDurationTooLarge
	}
	target, err := s.targetByName(targetName)
	if err != nil {
		return RuntimeSupervisorContainerFallbackControlResult{}, err
	}
	source := strings.TrimSpace(options.Source)
	if source == "" {
		source = "api"
	}
	operator := strings.TrimSpace(options.Operator)
	now := time.Now().UTC()
	key := runtimeSupervisorServiceKey(target)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.serviceStates == nil {
		s.serviceStates = make(map[string]runtimeSupervisorServiceState)
	}
	state := s.serviceStates[key]
	var backoffUntil *time.Time
	if clear {
		state.ContainerFallbackBackoffUntil = time.Time{}
		state.ContainerFallbackBackoffClearedAt = now
		state.ContainerFallbackBackoffClearedReason = reason
		state.ContainerFallbackBackoffClearedSource = source
		state.ContainerFallbackSubmittedAt = time.Time{}
		state.ContainerFallbackSubmittedReason = ""
		state.ContainerFallbackSubmittedMessage = ""
		state.ContainerFallbackSubmittedError = ""
		if s.submittedFallbacks != nil {
			delete(s.submittedFallbacks, key)
		}
	} else {
		until := now.Add(options.BackoffDuration).UTC()
		state.ContainerFallbackBackoffUntil = until
		state.ContainerFallbackBackoffSetAt = now
		state.ContainerFallbackBackoffReason = reason
		state.ContainerFallbackBackoffSource = source
		backoffUntil = &until
	}
	s.serviceStates[key] = state
	action := "defer-container-fallback"
	if clear {
		action = "clear-container-fallback-backoff"
	}
	event := RuntimeSupervisorContainerFallbackControlAction{
		Action:        action,
		TargetName:    target.Name,
		TargetBaseURL: target.BaseURL,
		Suppressed:    state.ContainerFallbackSuppressed,
		BackoffUntil:  backoffUntil,
		Reason:        reason,
		Source:        source,
		Operator:      operator,
		UpdatedAt:     now,
	}
	if !clear {
		event.BackoffSeconds = int(options.BackoffDuration / time.Second)
	}
	s.recordContainerFallbackControlLocked(event)
	return RuntimeSupervisorContainerFallbackControlResult{
		TargetName:    target.Name,
		TargetBaseURL: target.BaseURL,
		Suppressed:    state.ContainerFallbackSuppressed,
		BackoffUntil:  backoffUntil,
		Reason:        reason,
		Source:        source,
		Operator:      operator,
		UpdatedAt:     now,
		ServiceState:  runtimeSupervisorServiceStateSnapshot(state, s.options.ServiceFailureThreshold),
	}, nil
}

func (s *RuntimeSupervisor) recordContainerFallbackControlLocked(event RuntimeSupervisorContainerFallbackControlAction) {
	if s == nil {
		return
	}
	s.fallbackControls = append([]RuntimeSupervisorContainerFallbackControlAction{event}, s.fallbackControls...)
	if len(s.fallbackControls) > defaultRuntimeSupervisorControlHistoryLimit {
		s.fallbackControls = s.fallbackControls[:defaultRuntimeSupervisorControlHistoryLimit]
	}
}

func (s *RuntimeSupervisor) recordContainerFallbackActionLocked(action RuntimeSupervisorContainerFallbackAction) {
	if s == nil {
		return
	}
	s.fallbackActions = append([]RuntimeSupervisorContainerFallbackAction{action}, s.fallbackActions...)
	if len(s.fallbackActions) > defaultRuntimeSupervisorControlHistoryLimit {
		s.fallbackActions = s.fallbackActions[:defaultRuntimeSupervisorControlHistoryLimit]
	}
}

func (s *RuntimeSupervisor) targetByName(targetName string) (RuntimeSupervisorTarget, error) {
	name := strings.TrimSpace(targetName)
	if name == "" {
		return RuntimeSupervisorTarget{}, ErrRuntimeSupervisorTargetRequired
	}
	var match RuntimeSupervisorTarget
	found := 0
	for _, target := range s.targets {
		if target.Name == name {
			match = target
			found++
		}
	}
	if found == 0 {
		return RuntimeSupervisorTarget{}, fmt.Errorf("%w: %s", ErrRuntimeSupervisorTargetNotFound, name)
	}
	if found > 1 {
		return RuntimeSupervisorTarget{}, fmt.Errorf("%w: %s", ErrRuntimeSupervisorTargetAmbiguous, name)
	}
	return match, nil
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
		"container_fallback_auto_submit", s.options.ContainerFallbackAutoSubmit,
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
		if state.ConsecutiveFailures == 0 || state.ServiceFailureEpisodeStartedAt.IsZero() {
			state.ServiceFailureEpisodeStartedAt = now
		}
		state.ConsecutiveFailures++
		state.LastFailureReason = reason
		state.LastFailureAt = now
		if state.ConsecutiveFailures >= s.options.ServiceFailureThreshold && state.ContainerFallbackCandidateSince.IsZero() {
			state.ContainerFallbackCandidateSince = now
		}
	} else {
		s.recordServiceFailureEpisodeLocked(target, state, now)
		state.ConsecutiveFailures = 0
		state.ServiceFailureEpisodeStartedAt = time.Time{}
		state.LastFailureReason = ""
		state.LastHealthyAt = now
		state.ContainerFallbackCandidateSince = time.Time{}
		state.ContainerFallbackAttemptCount = 0
		state.ContainerFallbackBackoffUntil = time.Time{}
		state.ContainerFallbackBackoffSetAt = time.Time{}
		state.ContainerFallbackBackoffReason = ""
		state.ContainerFallbackBackoffSource = ""
		state.ContainerFallbackBackoffClearedAt = time.Time{}
		state.ContainerFallbackBackoffClearedReason = ""
		state.ContainerFallbackBackoffClearedSource = ""
		state.ContainerFallbackSubmittedAt = time.Time{}
		state.ContainerFallbackSubmittedReason = ""
		state.ContainerFallbackSubmittedMessage = ""
		state.ContainerFallbackSubmittedError = ""
		state.LastContainerFallbackDecisionAt = time.Time{}
		state.LastContainerFallbackDecisionReason = ""
		if s.submittedFallbacks != nil {
			delete(s.submittedFallbacks, key)
		}
	}
	s.serviceStates[key] = state
	return runtimeSupervisorServiceStateSnapshot(state, s.options.ServiceFailureThreshold)
}

func (s *RuntimeSupervisor) recordServiceFailureEpisodeLocked(target RuntimeSupervisorTarget, state runtimeSupervisorServiceState, recoveredAt time.Time) {
	if s == nil || state.ConsecutiveFailures == 0 || state.ServiceFailureEpisodeStartedAt.IsZero() {
		return
	}
	if recoveredAt.IsZero() {
		recoveredAt = time.Now().UTC()
	}
	recoveredAt = recoveredAt.UTC()
	startedAt := state.ServiceFailureEpisodeStartedAt.UTC()
	durationSeconds := int(recoveredAt.Sub(startedAt) / time.Second)
	if durationSeconds < 0 {
		durationSeconds = 0
	}
	episode := RuntimeSupervisorServiceFailureEpisode{
		TargetName:                          target.Name,
		TargetBaseURL:                       target.BaseURL,
		StartedAt:                           startedAt,
		RecoveredAt:                         recoveredAt,
		DurationSeconds:                     durationSeconds,
		MaxConsecutiveFailures:              state.ConsecutiveFailures,
		LastFailureReason:                   state.LastFailureReason,
		ContainerFallbackCandidate:          !state.ContainerFallbackCandidateSince.IsZero(),
		ContainerFallbackAttemptCount:       state.ContainerFallbackAttemptCount,
		ContainerFallbackSubmitted:          !state.ContainerFallbackSubmittedAt.IsZero(),
		ContainerFallbackSubmittedError:     state.ContainerFallbackSubmittedError,
		LastContainerFallbackDecisionReason: state.LastContainerFallbackDecisionReason,
	}
	if !state.LastFailureAt.IsZero() {
		lastFailureAt := state.LastFailureAt.UTC()
		episode.LastFailureAt = &lastFailureAt
	}
	if !state.ContainerFallbackCandidateSince.IsZero() {
		candidateSince := state.ContainerFallbackCandidateSince.UTC()
		episode.ContainerFallbackCandidateSince = &candidateSince
	}
	if !state.ContainerFallbackSubmittedAt.IsZero() {
		submittedAt := state.ContainerFallbackSubmittedAt.UTC()
		episode.ContainerFallbackSubmittedAt = &submittedAt
	}
	if !state.ContainerFallbackBackoffUntil.IsZero() {
		backoffUntil := state.ContainerFallbackBackoffUntil.UTC()
		episode.ContainerFallbackBackoffUntil = &backoffUntil
	}
	if !state.LastContainerFallbackDecisionAt.IsZero() {
		lastDecisionAt := state.LastContainerFallbackDecisionAt.UTC()
		episode.LastContainerFallbackDecisionAt = &lastDecisionAt
	}
	s.failureEpisodes = append([]RuntimeSupervisorServiceFailureEpisode{episode}, s.failureEpisodes...)
	if len(s.failureEpisodes) > defaultRuntimeSupervisorControlHistoryLimit {
		s.failureEpisodes = s.failureEpisodes[:defaultRuntimeSupervisorControlHistoryLimit]
	}
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
		ConsecutiveFailures:                   state.ConsecutiveFailures,
		FailureThreshold:                      threshold,
		LastFailureReason:                     state.LastFailureReason,
		ContainerFallbackSuppressed:           state.ContainerFallbackSuppressed,
		ContainerFallbackSuppressedReason:     state.ContainerFallbackSuppressedReason,
		ContainerFallbackSuppressedSource:     state.ContainerFallbackSuppressedSource,
		ContainerFallbackResumedReason:        state.ContainerFallbackResumedReason,
		ContainerFallbackResumedSource:        state.ContainerFallbackResumedSource,
		ContainerFallbackBackoffReason:        state.ContainerFallbackBackoffReason,
		ContainerFallbackBackoffSource:        state.ContainerFallbackBackoffSource,
		ContainerFallbackBackoffClearedReason: state.ContainerFallbackBackoffClearedReason,
		ContainerFallbackBackoffClearedSource: state.ContainerFallbackBackoffClearedSource,
		ContainerFallbackAttemptCount:         state.ContainerFallbackAttemptCount,
		ContainerFallbackSubmittedReason:      state.ContainerFallbackSubmittedReason,
		ContainerFallbackSubmittedMessage:     state.ContainerFallbackSubmittedMessage,
		ContainerFallbackSubmittedError:       state.ContainerFallbackSubmittedError,
		LastContainerFallbackDecisionReason:   state.LastContainerFallbackDecisionReason,
	}
	if !state.ServiceFailureEpisodeStartedAt.IsZero() {
		episodeStartedAt := state.ServiceFailureEpisodeStartedAt.UTC()
		out.ServiceFailureEpisodeStartedAt = &episodeStartedAt
	}
	if !state.LastFailureAt.IsZero() {
		lastFailureAt := state.LastFailureAt.UTC()
		out.LastFailureAt = &lastFailureAt
	}
	if !state.LastHealthyAt.IsZero() {
		lastHealthyAt := state.LastHealthyAt.UTC()
		out.LastHealthyAt = &lastHealthyAt
	}
	if !state.ContainerFallbackSuppressedAt.IsZero() {
		suppressedAt := state.ContainerFallbackSuppressedAt.UTC()
		out.ContainerFallbackSuppressedAt = &suppressedAt
	}
	if !state.ContainerFallbackResumedAt.IsZero() {
		resumedAt := state.ContainerFallbackResumedAt.UTC()
		out.ContainerFallbackResumedAt = &resumedAt
	}
	if !state.ContainerFallbackBackoffUntil.IsZero() {
		backoffUntil := state.ContainerFallbackBackoffUntil.UTC()
		out.ContainerFallbackBackoffUntil = &backoffUntil
	}
	if !state.ContainerFallbackBackoffSetAt.IsZero() {
		backoffSetAt := state.ContainerFallbackBackoffSetAt.UTC()
		out.ContainerFallbackBackoffSetAt = &backoffSetAt
	}
	if !state.ContainerFallbackBackoffClearedAt.IsZero() {
		backoffClearedAt := state.ContainerFallbackBackoffClearedAt.UTC()
		out.ContainerFallbackBackoffClearedAt = &backoffClearedAt
	}
	if !state.ContainerFallbackSubmittedAt.IsZero() {
		submittedAt := state.ContainerFallbackSubmittedAt.UTC()
		out.ContainerFallbackSubmitted = true
		out.ContainerFallbackSubmittedAt = &submittedAt
	}
	if !state.LastContainerFallbackDecisionAt.IsZero() {
		lastDecisionAt := state.LastContainerFallbackDecisionAt.UTC()
		out.LastContainerFallbackDecisionAt = &lastDecisionAt
	}
	if state.ConsecutiveFailures >= threshold && state.LastFailureReason != "" {
		out.ContainerFallbackCandidate = true
		out.ContainerFallbackReason = fmt.Sprintf("service probes failed %d/%d: %s", state.ConsecutiveFailures, threshold, state.LastFailureReason)
		if !state.ContainerFallbackCandidateSince.IsZero() {
			candidateSince := state.ContainerFallbackCandidateSince.UTC()
			out.ContainerFallbackCandidateSince = &candidateSince
		}
	}
	return out
}

func (s *RuntimeSupervisor) updateContainerFallbackPlan(target RuntimeSupervisorTarget, state RuntimeSupervisorServiceState, now time.Time) (*RuntimeSupervisorContainerFallbackPlan, RuntimeSupervisorServiceState) {
	if s == nil || !state.ContainerFallbackCandidate {
		return nil, state
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	now = now.UTC()
	key := runtimeSupervisorServiceKey(target)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.serviceStates == nil {
		s.serviceStates = make(map[string]runtimeSupervisorServiceState)
	}
	stored := s.serviceStates[key]
	stored.ContainerFallbackAttemptCount++
	state = runtimeSupervisorServiceStateSnapshot(stored, s.options.ServiceFailureThreshold)
	plan := runtimeSupervisorContainerFallbackPlan(target, state, s.options, now)
	decisionReason := runtimeSupervisorContainerFallbackDecisionReason(plan)
	stored.LastContainerFallbackDecisionAt = now
	stored.LastContainerFallbackDecisionReason = decisionReason
	s.serviceStates[key] = stored
	state = runtimeSupervisorServiceStateSnapshot(stored, s.options.ServiceFailureThreshold)
	return plan, state
}

func (s *RuntimeSupervisor) submitContainerFallback(ctx context.Context, target RuntimeSupervisorTarget, plan *RuntimeSupervisorContainerFallbackPlan, now time.Time) bool {
	_, submitted := s.submitContainerFallbackAction(ctx, target, plan, now, runtimeSupervisorContainerFallbackSubmitAudit{
		Source: "supervisor",
	})
	return submitted
}

func (s *RuntimeSupervisor) submitContainerFallbackAction(ctx context.Context, target RuntimeSupervisorTarget, plan *RuntimeSupervisorContainerFallbackPlan, now time.Time, audit runtimeSupervisorContainerFallbackSubmitAudit) (RuntimeSupervisorContainerFallbackAction, bool) {
	if s == nil || plan == nil || !plan.Executable || s.options.ContainerFallbackExecutor == nil {
		return RuntimeSupervisorContainerFallbackAction{}, false
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	now = now.UTC()
	key := runtimeSupervisorServiceKey(target)
	s.mu.Lock()
	if s.submittedFallbacks == nil {
		s.submittedFallbacks = make(map[string]bool)
	}
	if s.submittedFallbacks[key] {
		s.mu.Unlock()
		return RuntimeSupervisorContainerFallbackAction{}, false
	}
	s.submittedFallbacks[key] = true
	s.mu.Unlock()

	planReason := strings.TrimSpace(plan.Reason)
	reason := strings.TrimSpace(audit.Reason)
	if reason == "" {
		reason = planReason
	}
	source := strings.TrimSpace(audit.Source)
	if source == "" {
		source = "supervisor"
	}
	operator := strings.TrimSpace(audit.Operator)
	action := RuntimeSupervisorContainerFallbackAction{
		Action:                          firstNonEmpty(plan.Action, "container-restart"),
		TargetName:                      target.Name,
		TargetBaseURL:                   target.BaseURL,
		Reason:                          reason,
		Source:                          source,
		Operator:                        operator,
		ServiceFailureEpisodeStartedAt:  plan.ServiceFailureEpisodeStartedAt,
		ContainerFallbackCandidateSince: plan.ContainerFallbackCandidateSince,
		ExecutorKind:                    plan.ExecutorKind,
		ExecutorDryRun:                  plan.ExecutorDryRun,
		ExecutorPreview:                 cloneRuntimeSupervisorContainerFallbackExecutorPreview(plan.ExecutorPreview),
		Submitted:                       true,
		RequestedAt:                     now,
	}
	if planReason != "" && planReason != reason {
		action.PlanReason = planReason
	}
	result, err := s.options.ContainerFallbackExecutor.Restart(ctx, target, reason)
	action.Executed = result.Executed
	action.Message = result.Message
	action.ExitCode = result.ExitCode
	action.TimedOut = result.TimedOut
	if err != nil {
		action.Error = err.Error()
		backoffUntil := now.Add(defaultRuntimeSupervisorFallbackErrorBackoff).UTC()
		action.BackoffUntil = &backoffUntil
		action.BackoffSeconds = int(defaultRuntimeSupervisorFallbackErrorBackoff / time.Second)
	}
	s.mu.Lock()
	s.recordContainerFallbackSubmissionLocked(target, action, err, now)
	s.recordContainerFallbackActionLocked(action)
	s.mu.Unlock()
	return action, true
}

func (s *RuntimeSupervisor) recordContainerFallbackSubmissionLocked(target RuntimeSupervisorTarget, action RuntimeSupervisorContainerFallbackAction, err error, now time.Time) {
	if s == nil {
		return
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if s.serviceStates == nil {
		s.serviceStates = make(map[string]runtimeSupervisorServiceState)
	}
	key := runtimeSupervisorServiceKey(target)
	state := s.serviceStates[key]
	state.ContainerFallbackSubmittedAt = now.UTC()
	state.ContainerFallbackSubmittedReason = action.Reason
	state.ContainerFallbackSubmittedMessage = action.Message
	state.ContainerFallbackSubmittedError = action.Error
	state.LastContainerFallbackDecisionAt = now.UTC()
	state.LastContainerFallbackDecisionReason = "container-fallback-already-submitted"
	if err != nil {
		state.ContainerFallbackBackoffUntil = now.Add(defaultRuntimeSupervisorFallbackErrorBackoff).UTC()
		state.ContainerFallbackBackoffSetAt = now.UTC()
		state.ContainerFallbackBackoffReason = "container fallback executor error: " + err.Error()
		state.ContainerFallbackBackoffSource = "supervisor"
		state.ContainerFallbackBackoffClearedAt = time.Time{}
		state.ContainerFallbackBackoffClearedReason = ""
		state.ContainerFallbackBackoffClearedSource = ""
		state.LastContainerFallbackDecisionReason = "container-fallback-backoff-active"
	}
	s.serviceStates[key] = state
}

func runtimeSupervisorContainerFallbackPlan(target RuntimeSupervisorTarget, state RuntimeSupervisorServiceState, options RuntimeSupervisorOptions, now time.Time) *RuntimeSupervisorContainerFallbackPlan {
	if !state.ContainerFallbackCandidate {
		return nil
	}
	executorConfigured := runtimeSupervisorContainerExecutorConfigured(options.ContainerFallbackExecutor)
	executorDescriptor := runtimeSupervisorContainerExecutorDescriptor(options.ContainerFallbackExecutor)
	executorArmed := runtimeSupervisorContainerExecutorArmed(options, executorDescriptor, executorConfigured)
	targetAllowed := runtimeSupervisorContainerFallbackTargetAllowed(options.ContainerFallbackExecutor, target)
	executorPreview := runtimeSupervisorContainerFallbackExecutorPreview(options.ContainerFallbackExecutor, target)
	suppressed := state.ContainerFallbackSuppressed
	backoffActive := runtimeSupervisorContainerFallbackBackoffActive(state.ContainerFallbackBackoffUntil, now)
	duplicate := state.ContainerFallbackSubmitted
	safetyGateOK := strings.TrimSpace(state.ContainerFallbackReason) != ""
	decision := evaluateRuntimeSupervisorContainerFallbackDecision(runtimeSupervisorContainerFallbackDecisionInput{
		Candidate:          state.ContainerFallbackCandidate,
		Enabled:            options.EnableContainerFallback,
		ExecutorConfigured: executorConfigured,
		ExecutorDryRun:     executorDescriptor.DryRun,
		ExecutorArmed:      executorArmed,
		TargetAllowed:      targetAllowed,
		Duplicate:          duplicate,
		Suppressed:         suppressed,
		BackoffActive:      backoffActive,
		SafetyGateOK:       safetyGateOK,
	})
	autoSubmitEnabled := options.ContainerFallbackAutoSubmit
	return &RuntimeSupervisorContainerFallbackPlan{
		Action:                          "container-restart",
		Candidate:                       true,
		Enabled:                         options.EnableContainerFallback,
		ServiceFailureEpisodeStartedAt:  state.ServiceFailureEpisodeStartedAt,
		ContainerFallbackCandidateSince: state.ContainerFallbackCandidateSince,
		ExecutorConfigured:              executorConfigured,
		ExecutorKind:                    executorDescriptor.Kind,
		ExecutorDryRun:                  executorDescriptor.DryRun,
		ExecutorArmed:                   executorArmed,
		TargetAllowed:                   targetAllowed,
		ExecutorPreview:                 executorPreview,
		Executable:                      decision.Executable,
		AutoSubmitEnabled:               autoSubmitEnabled,
		AutoSubmitEligible:              decision.Executable && autoSubmitEnabled,
		ManualSubmitRequired:            decision.Executable && !autoSubmitEnabled,
		Decision:                        decision.Decision,
		Duplicate:                       duplicate,
		Suppressed:                      suppressed,
		BackoffActive:                   backoffActive,
		SafetyGateOK:                    safetyGateOK,
		BlockedReason:                   decision.BlockedReason,
		EligibleReason:                  decision.EligibleReason,
		Reason:                          state.ContainerFallbackReason,
	}
}

func runtimeSupervisorContainerFallbackBackoffActive(backoffUntil *time.Time, now time.Time) bool {
	if backoffUntil == nil || backoffUntil.IsZero() {
		return false
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return backoffUntil.UTC().After(now.UTC())
}

func runtimeSupervisorContainerFallbackDecisionReason(plan *RuntimeSupervisorContainerFallbackPlan) string {
	if plan == nil {
		return ""
	}
	if strings.TrimSpace(plan.BlockedReason) != "" {
		return strings.TrimSpace(plan.BlockedReason)
	}
	return strings.TrimSpace(plan.EligibleReason)
}

func evaluateRuntimeSupervisorContainerFallbackDecision(input runtimeSupervisorContainerFallbackDecisionInput) runtimeSupervisorContainerFallbackDecisionResult {
	blocked := func(reason string) runtimeSupervisorContainerFallbackDecisionResult {
		return runtimeSupervisorContainerFallbackDecisionResult{
			Decision:      runtimeSupervisorContainerFallbackDecisionBlocked,
			BlockedReason: reason,
		}
	}
	if !input.Candidate {
		return blocked("container-fallback-not-candidate")
	}
	if !input.Enabled {
		return blocked("container-restart-disabled")
	}
	if !input.ExecutorConfigured {
		return blocked("container-executor-not-configured")
	}
	if !input.ExecutorDryRun && !input.ExecutorArmed {
		return blocked("container-executor-not-armed")
	}
	if !input.ExecutorDryRun && !input.TargetAllowed {
		return blocked("container-executor-target-not-allowlisted")
	}
	if input.Suppressed {
		return blocked("container-fallback-suppressed")
	}
	if input.BackoffActive {
		return blocked("container-fallback-backoff-active")
	}
	if input.Duplicate {
		return blocked("container-fallback-already-submitted")
	}
	if !input.SafetyGateOK {
		return blocked("container-fallback-safety-gate-blocked")
	}
	return runtimeSupervisorContainerFallbackDecisionResult{
		Decision:       runtimeSupervisorContainerFallbackDecisionEligible,
		Executable:     true,
		EligibleReason: "container-fallback-eligible",
	}
}

func runtimeSupervisorPolicy(options RuntimeSupervisorOptions) RuntimeSupervisorPolicy {
	options = normalizeRuntimeSupervisorOptions(options)
	executorDescriptor := runtimeSupervisorContainerExecutorDescriptor(options.ContainerFallbackExecutor)
	executorConfigured := runtimeSupervisorContainerExecutorConfigured(options.ContainerFallbackExecutor)
	return RuntimeSupervisorPolicy{
		ApplicationRestartEnabled:   options.EnableApplicationRestart,
		ServiceFailureThreshold:     options.ServiceFailureThreshold,
		ContainerRestartEnabled:     options.EnableContainerFallback,
		ContainerFallbackAutoSubmit: options.ContainerFallbackAutoSubmit,
		ContainerExecutorConfigured: executorConfigured,
		ContainerExecutorKind:       executorDescriptor.Kind,
		ContainerExecutorDryRun:     executorDescriptor.DryRun,
		ContainerExecutorArmed:      runtimeSupervisorContainerExecutorArmed(options, executorDescriptor, executorConfigured),
		DashboardPermissions: RuntimeSupervisorDashboardPermissions{
			CanView:                    true,
			CanRuntimeControl:          true,
			CanContainerFallbackGate:   true,
			CanContainerFallbackSubmit: false,
		},
	}
}

func runtimeSupervisorContainerExecutorConfigured(executor ContainerFallbackExecutor) bool {
	return executor != nil && executor.Configured()
}

func runtimeSupervisorContainerExecutorDescriptor(executor ContainerFallbackExecutor) ContainerFallbackExecutorDescriptor {
	if executor == nil {
		return ContainerFallbackExecutorDescriptor{
			Kind:   runtimeSupervisorContainerExecutorKindNone,
			DryRun: true,
		}
	}
	descriptor := executor.Descriptor()
	descriptor.Kind = strings.TrimSpace(descriptor.Kind)
	if descriptor.Kind == "" {
		descriptor.Kind = runtimeSupervisorContainerExecutorKindCustom
	}
	return descriptor
}

func runtimeSupervisorContainerExecutorArmed(options RuntimeSupervisorOptions, descriptor ContainerFallbackExecutorDescriptor, configured bool) bool {
	if !configured {
		return false
	}
	if descriptor.DryRun {
		return true
	}
	return options.ContainerFallbackExecutorArmed
}

func runtimeSupervisorContainerFallbackTargetAllowed(executor ContainerFallbackExecutor, target RuntimeSupervisorTarget) bool {
	allowlist, ok := executor.(ContainerFallbackTargetAllowlist)
	if !ok {
		return true
	}
	return allowlist.ContainerFallbackTargetAllowed(target)
}

func runtimeSupervisorContainerFallbackExecutorPreview(executor ContainerFallbackExecutor, target RuntimeSupervisorTarget) *RuntimeSupervisorContainerFallbackExecutorPreview {
	previewer, ok := executor.(ContainerFallbackExecutorPreviewer)
	if !ok {
		return nil
	}
	preview, ok := previewer.ContainerFallbackExecutorPreview(target)
	if !ok || preview == nil {
		return nil
	}
	return cloneRuntimeSupervisorContainerFallbackExecutorPreview(preview)
}

func cloneRuntimeSupervisorContainerFallbackExecutorPreview(preview *RuntimeSupervisorContainerFallbackExecutorPreview) *RuntimeSupervisorContainerFallbackExecutorPreview {
	if preview == nil {
		return nil
	}
	clone := *preview
	if preview.CommandArgs != nil {
		clone.CommandArgs = append([]string(nil), preview.CommandArgs...)
	}
	return &clone
}

func runtimeSupervisorServiceKey(target RuntimeSupervisorTarget) string {
	return strings.TrimSpace(target.Name) + "|" + strings.TrimSpace(target.BaseURL)
}

func (s *RuntimeSupervisor) attachApplicationRestartPlans(target RuntimeSupervisorTarget, status RuntimeStatusSnapshot, healthz RuntimeSupervisorProbe, now time.Time) RuntimeStatusSnapshot {
	for i := range status.Runtimes {
		plan := s.runtimeSupervisorApplicationRestartPlan(target, status.Runtimes[i], healthz, now)
		if plan != nil {
			status.Runtimes[i].ApplicationRestartPlan = plan
		}
	}
	return status
}

func (s *RuntimeSupervisor) runtimeSupervisorApplicationRestartPlan(target RuntimeSupervisorTarget, runtime RuntimeStatus, healthz RuntimeSupervisorProbe, now time.Time) *RuntimeSupervisorApplicationRestartPlan {
	if !runtimeSupervisorApplicationRestartCandidate(runtime) {
		return nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	now = now.UTC()
	nextRestartAt := strings.TrimSpace(runtime.NextRestartAt)
	_, nextRestartAtPresent := ParseRestartTime(map[string]any{"nextRestartAt": nextRestartAt}, "nextRestartAt")
	due := runtimeSupervisorRestartDueAt(runtime, now)
	duplicate := due && s.runtimeRestartAlreadySubmitted(target, runtime)
	reason := runtimeSupervisorRestartReason(target, runtime)
	decision := evaluateRuntimeSupervisorApplicationRestartDecision(runtimeSupervisorApplicationRestartDecisionInput{
		Candidate:            true,
		Enabled:              s != nil && s.options.EnableApplicationRestart,
		HealthzOK:            runtimeSupervisorHealthzOK(healthz),
		Supported:            runtimeSupervisorRestartSupportedKind(runtime.RuntimeKind),
		DesiredRunning:       strings.EqualFold(strings.TrimSpace(runtime.DesiredStatus), "RUNNING"),
		ActualError:          strings.EqualFold(strings.TrimSpace(runtime.ActualStatus), "ERROR"),
		Suppressed:           runtime.AutoRestartSuppressed,
		Fatal:                strings.EqualFold(strings.TrimSpace(runtime.RestartSeverity), "fatal"),
		NextRestartAtPresent: nextRestartAtPresent,
		Due:                  due,
		Duplicate:            duplicate,
	})
	return &RuntimeSupervisorApplicationRestartPlan{
		RuntimeID:      strings.TrimSpace(runtime.RuntimeID),
		RuntimeKind:    strings.TrimSpace(runtime.RuntimeKind),
		Candidate:      true,
		Enabled:        s != nil && s.options.EnableApplicationRestart,
		HealthzOK:      runtimeSupervisorHealthzOK(healthz),
		Supported:      runtimeSupervisorRestartSupportedKind(runtime.RuntimeKind),
		Due:            due,
		Duplicate:      duplicate,
		Decision:       decision.Decision,
		BlockedReason:  decision.BlockedReason,
		EligibleReason: decision.EligibleReason,
		Reason:         reason,
		NextRestartAt:  nextRestartAt,
	}
}

func runtimeSupervisorApplicationRestartCandidate(runtime RuntimeStatus) bool {
	if strings.TrimSpace(runtime.RuntimeID) == "" {
		return false
	}
	if strings.TrimSpace(runtime.NextRestartAt) != "" || runtime.AutoRestartSuppressed {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(runtime.RestartSeverity), "fatal") {
		return true
	}
	actual := strings.ToUpper(strings.TrimSpace(runtime.ActualStatus))
	health := strings.ToLower(strings.TrimSpace(runtime.Health))
	return actual == "ERROR" || health == "error" || health == "suppressed" || health == "unreachable" || health == "stale"
}

func evaluateRuntimeSupervisorApplicationRestartDecision(input runtimeSupervisorApplicationRestartDecisionInput) runtimeSupervisorApplicationRestartDecisionResult {
	blocked := func(reason string) runtimeSupervisorApplicationRestartDecisionResult {
		return runtimeSupervisorApplicationRestartDecisionResult{
			Decision:      runtimeSupervisorApplicationRestartDecisionBlocked,
			BlockedReason: reason,
		}
	}
	if !input.Candidate {
		return blocked("runtime-restart-not-candidate")
	}
	if !input.Enabled {
		return blocked("runtime-restart-disabled")
	}
	if !input.HealthzOK {
		return blocked("runtime-restart-healthz-unhealthy")
	}
	if !input.Supported {
		return blocked("runtime-restart-unsupported-kind")
	}
	if !input.DesiredRunning {
		return blocked("runtime-restart-desired-not-running")
	}
	if !input.ActualError {
		return blocked("runtime-restart-actual-not-error")
	}
	if input.Suppressed {
		return blocked("runtime-restart-suppressed")
	}
	if input.Fatal {
		return blocked("runtime-restart-fatal")
	}
	if !input.NextRestartAtPresent {
		return blocked("runtime-restart-next-at-missing")
	}
	if !input.Due {
		return blocked("runtime-restart-not-due")
	}
	if input.Duplicate {
		return blocked("runtime-restart-duplicate")
	}
	return runtimeSupervisorApplicationRestartDecisionResult{
		Decision:       runtimeSupervisorApplicationRestartDecisionEligible,
		EligibleReason: "runtime-restart-eligible",
	}
}

func runtimeSupervisorHealthzOK(healthz RuntimeSupervisorProbe) bool {
	if !healthz.Reachable || strings.TrimSpace(healthz.Error) != "" {
		return false
	}
	return healthz.StatusCode == 0 || (healthz.StatusCode >= http.StatusOK && healthz.StatusCode < http.StatusMultipleChoices)
}

func (s *RuntimeSupervisor) submitApplicationRestarts(ctx context.Context, target RuntimeSupervisorTarget, status RuntimeStatusSnapshot, healthz RuntimeSupervisorProbe, now time.Time) []RuntimeSupervisorControlAction {
	if s == nil || !s.options.EnableApplicationRestart {
		return nil
	}
	if !runtimeSupervisorHealthzOK(healthz) {
		return nil
	}
	actions := make([]RuntimeSupervisorControlAction, 0)
	for _, runtime := range status.Runtimes {
		if runtime.ApplicationRestartPlan == nil || runtime.ApplicationRestartPlan.Decision != runtimeSupervisorApplicationRestartDecisionEligible {
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
	if !runtimeSupervisorRestartSupportedKind(runtime.RuntimeKind) {
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
	return runtimeSupervisorRestartDueAt(runtime, now)
}

func runtimeSupervisorRestartSupportedKind(kind string) bool {
	normalized := strings.ToLower(strings.TrimSpace(kind))
	return normalized == "signal" || normalized == "signal-runtime"
}

func runtimeSupervisorRestartDueAt(runtime RuntimeStatus, now time.Time) bool {
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
		Source:      "supervisor",
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
	req.Header.Set("X-BKTRADER-Control-Source", "supervisor")
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
