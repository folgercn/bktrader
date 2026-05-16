package nodeagent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	defaultListenAddr       = "127.0.0.1:18081"
	defaultCommandTimeout   = 30 * time.Second
	minTokenLength          = 16
	maxCommandTimeout       = 5 * time.Minute
	maxCommandOutputBytes   = 512
	maxRequestBodyBytes     = 4096
	executorKindNodeAgent   = "node-agent"
	actionContainerRestart  = "container-restart"
	executorDockerCompose   = "docker-compose"
	defaultDockerExecutable = "docker"
)

type Config struct {
	Addr       string
	Token      string
	TokenFile  string
	TargetsRaw string
	Version    string
	Runner     CommandRunner
}

type Agent struct {
	addr    string
	token   string
	targets map[string]TargetSpec
	version string
	runner  CommandRunner
}

type TargetSpec struct {
	Action           string   `json:"action"`
	Executor         string   `json:"executor"`
	ProjectDirectory string   `json:"projectDirectory"`
	ComposeFiles     []string `json:"composeFiles"`
	Services         []string `json:"services"`
	TimeoutSeconds   int      `json:"timeoutSeconds,omitempty"`
	DockerPath       string   `json:"dockerPath,omitempty"`
}

type targetConfigFile struct {
	Targets map[string]TargetSpec `json:"targets"`
}

type HealthResponse struct {
	Status             string    `json:"status"`
	Version            string    `json:"version,omitempty"`
	ExecutorKind       string    `json:"executorKind,omitempty"`
	TokenConfigured    bool      `json:"tokenConfigured"`
	AllowlistedTargets []string  `json:"allowlistedTargets,omitempty"`
	CheckedAt          time.Time `json:"checkedAt,omitempty"`
}

type RestartRequest struct {
	RequestID      string `json:"requestId,omitempty"`
	TargetName     string `json:"targetName"`
	Action         string `json:"action"`
	Reason         string `json:"reason"`
	PlanReason     string `json:"planReason,omitempty"`
	EpisodeStarted string `json:"episodeStartedAt,omitempty"`
	CandidateSince string `json:"candidateSince,omitempty"`
	Source         string `json:"source,omitempty"`
	Operator       string `json:"operator,omitempty"`
}

type RestartResponse struct {
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

type CommandRunner interface {
	Run(ctx context.Context, dir string, path string, args []string) CommandResult
}

type CommandResult struct {
	ExitCode int
	Output   string
	TimedOut bool
	Err      error
}

type execCommandRunner struct{}

func New(cfg Config) (*Agent, error) {
	token := strings.TrimSpace(cfg.Token)
	if token == "" && strings.TrimSpace(cfg.TokenFile) != "" {
		data, err := os.ReadFile(strings.TrimSpace(cfg.TokenFile))
		if err != nil {
			return nil, fmt.Errorf("read node-agent token file: %w", err)
		}
		token = strings.TrimSpace(string(data))
	}
	if token == "" {
		return nil, fmt.Errorf("node-agent token is required")
	}
	if len(token) < minTokenLength || token == "agent-token" || token == "example-token" {
		return nil, fmt.Errorf("node-agent token must be a non-example secret with at least %d characters", minTokenLength)
	}
	targets, err := parseTargets(cfg.TargetsRaw)
	if err != nil {
		return nil, err
	}
	addr := strings.TrimSpace(cfg.Addr)
	if addr == "" {
		addr = defaultListenAddr
	}
	version := strings.TrimSpace(cfg.Version)
	if version == "" {
		version = "dev"
	}
	runner := cfg.Runner
	if runner == nil {
		runner = execCommandRunner{}
	}
	return &Agent{
		addr:    addr,
		token:   token,
		targets: targets,
		version: version,
		runner:  runner,
	}, nil
}

func (a *Agent) Addr() string {
	if a == nil || strings.TrimSpace(a.addr) == "" {
		return defaultListenAddr
	}
	return a.addr
}

func (a *Agent) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/health", a.handleHealth)
	mux.HandleFunc("/v1/container-fallback/restart", a.handleRestart)
	return mux
}

func (a *Agent) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if !a.authorized(r) {
		slog.Warn("node-agent health rejected unauthorized request", "remoteAddr", r.RemoteAddr)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	writeJSON(w, http.StatusOK, HealthResponse{
		Status:             "ok",
		Version:            a.version,
		ExecutorKind:       executorKindNodeAgent,
		TokenConfigured:    a.token != "",
		AllowlistedTargets: a.allowlistedTargets(),
		CheckedAt:          time.Now().UTC(),
	})
}

func (a *Agent) handleRestart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if !a.authorized(r) {
		slog.Warn("node-agent restart rejected unauthorized request", "remoteAddr", r.RemoteAddr)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	var request RestartRequest
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxRequestBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid restart request: " + err.Error()})
		return
	}
	spec, err := a.validateRestartRequest(request)
	if err != nil {
		slog.Warn("node-agent restart rejected by allowlist", "remoteAddr", r.RemoteAddr, "target", strings.TrimSpace(request.TargetName), "action", strings.TrimSpace(request.Action), "error", err)
		writeJSON(w, http.StatusBadRequest, RestartResponse{
			RequestID:    strings.TrimSpace(request.RequestID),
			TargetName:   strings.TrimSpace(request.TargetName),
			Action:       firstNonEmpty(strings.TrimSpace(request.Action), actionContainerRestart),
			ExecutorKind: executorKindNodeAgent,
			Error:        err.Error(),
		})
		return
	}
	started := time.Now().UTC()
	timeout := time.Duration(spec.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = defaultCommandTimeout
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	commandPath, args := dockerComposeRestartCommand(spec)
	result := a.runner.Run(ctx, spec.ProjectDirectory, commandPath, args)
	finished := time.Now().UTC()
	response := RestartResponse{
		RequestID:    strings.TrimSpace(request.RequestID),
		TargetName:   strings.TrimSpace(request.TargetName),
		Action:       actionContainerRestart,
		ExecutorKind: executorKindNodeAgent,
		Executed:     result.Err == nil && result.ExitCode == 0 && !result.TimedOut,
		TimedOut:     result.TimedOut,
		Message:      strings.TrimSpace(result.Output),
		StartedAt:    started.Format(time.RFC3339Nano),
		FinishedAt:   finished.Format(time.RFC3339Nano),
		DurationMs:   int(finished.Sub(started) / time.Millisecond),
	}
	if result.ExitCode >= 0 {
		response.ExitCode = &result.ExitCode
	}
	if !response.Executed {
		if result.Err != nil {
			response.Error = result.Err.Error()
		}
		if response.Message == "" {
			response.Message = response.Error
		}
		if response.Error == "" {
			if result.TimedOut {
				response.Error = "node-agent command timed out"
			} else {
				response.Error = fmt.Sprintf("node-agent command exited with code %d", result.ExitCode)
			}
			if response.Message == "" {
				response.Message = response.Error
			}
		}
		slog.Warn("node-agent restart command failed", "target", response.TargetName, "action", response.Action, "executorKind", response.ExecutorKind, "services", spec.Services, "exitCode", result.ExitCode, "timedOut", result.TimedOut, "durationMs", response.DurationMs, "error", response.Error)
		writeJSON(w, http.StatusInternalServerError, response)
		return
	}
	if response.Message == "" {
		response.Message = "container restart completed"
	}
	slog.Info("node-agent restart command completed", "target", response.TargetName, "action", response.Action, "executorKind", response.ExecutorKind, "services", spec.Services, "exitCode", result.ExitCode, "durationMs", response.DurationMs)
	writeJSON(w, http.StatusOK, response)
}

func (a *Agent) authorized(r *http.Request) bool {
	if a == nil || a.token == "" {
		return false
	}
	return r.Header.Get("Authorization") == "Bearer "+a.token
}

func (a *Agent) validateRestartRequest(request RestartRequest) (TargetSpec, error) {
	if a == nil {
		return TargetSpec{}, fmt.Errorf("node-agent is not configured")
	}
	targetName := strings.TrimSpace(request.TargetName)
	if targetName == "" {
		return TargetSpec{}, fmt.Errorf("targetName is required")
	}
	spec, ok := a.targets[targetName]
	if !ok {
		return TargetSpec{}, fmt.Errorf("target %q is not allowlisted", targetName)
	}
	if action := strings.TrimSpace(request.Action); action != "" && action != actionContainerRestart {
		return TargetSpec{}, fmt.Errorf("unsupported action %q", action)
	}
	if strings.TrimSpace(request.Reason) == "" {
		return TargetSpec{}, fmt.Errorf("reason is required")
	}
	return spec, nil
}

func (a *Agent) allowlistedTargets() []string {
	names := make([]string, 0, len(a.targets))
	for name := range a.targets {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func parseTargets(raw string) (map[string]TargetSpec, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("node-agent targets JSON is required")
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	var parsed targetConfigFile
	if err := decoder.Decode(&parsed); err != nil {
		return nil, fmt.Errorf("node-agent targets JSON is invalid: %w", err)
	}
	if len(parsed.Targets) == 0 {
		return nil, fmt.Errorf("node-agent targets allowlist is required")
	}
	targets := make(map[string]TargetSpec, len(parsed.Targets))
	for rawName, rawSpec := range parsed.Targets {
		name := strings.TrimSpace(rawName)
		if !validTargetName(name) {
			return nil, fmt.Errorf("target name %q is invalid", rawName)
		}
		spec, err := normalizeTargetSpec(rawSpec)
		if err != nil {
			return nil, fmt.Errorf("target %s: %w", name, err)
		}
		if _, exists := targets[name]; exists {
			return nil, fmt.Errorf("target %s is duplicated", name)
		}
		targets[name] = spec
	}
	return targets, nil
}

func normalizeTargetSpec(spec TargetSpec) (TargetSpec, error) {
	action := strings.TrimSpace(spec.Action)
	if action == "" {
		action = actionContainerRestart
	}
	if action != actionContainerRestart {
		return TargetSpec{}, fmt.Errorf("unsupported action %q", action)
	}
	executor := strings.TrimSpace(spec.Executor)
	if executor == "" {
		executor = executorDockerCompose
	}
	if executor != executorDockerCompose {
		return TargetSpec{}, fmt.Errorf("unsupported executor %q", executor)
	}
	projectDirectory := strings.TrimSpace(spec.ProjectDirectory)
	if projectDirectory == "" {
		return TargetSpec{}, fmt.Errorf("projectDirectory is required")
	}
	if !filepath.IsAbs(projectDirectory) {
		return TargetSpec{}, fmt.Errorf("projectDirectory must be absolute")
	}
	if strings.ContainsRune(projectDirectory, 0) {
		return TargetSpec{}, fmt.Errorf("projectDirectory contains NUL byte")
	}
	composeFiles := make([]string, 0, len(spec.ComposeFiles))
	for _, file := range spec.ComposeFiles {
		normalized, err := normalizeComposeFile(file)
		if err != nil {
			return TargetSpec{}, err
		}
		composeFiles = append(composeFiles, normalized)
	}
	if len(composeFiles) == 0 {
		return TargetSpec{}, fmt.Errorf("composeFiles must contain at least one file")
	}
	services := make([]string, 0, len(spec.Services))
	for _, service := range spec.Services {
		name := strings.TrimSpace(service)
		if !validTargetName(name) {
			return TargetSpec{}, fmt.Errorf("service name %q is invalid", service)
		}
		services = append(services, name)
	}
	if len(services) == 0 {
		return TargetSpec{}, fmt.Errorf("services must contain at least one service")
	}
	timeoutSeconds := spec.TimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = int(defaultCommandTimeout / time.Second)
	}
	if timeoutSeconds > int(maxCommandTimeout/time.Second) {
		return TargetSpec{}, fmt.Errorf("timeoutSeconds must be <= %d", int(maxCommandTimeout/time.Second))
	}
	dockerPath := strings.TrimSpace(spec.DockerPath)
	if dockerPath == "" {
		dockerPath = defaultDockerExecutable
	}
	if strings.ContainsRune(dockerPath, 0) {
		return TargetSpec{}, fmt.Errorf("dockerPath contains NUL byte")
	}
	return TargetSpec{
		Action:           action,
		Executor:         executor,
		ProjectDirectory: projectDirectory,
		ComposeFiles:     composeFiles,
		Services:         services,
		TimeoutSeconds:   timeoutSeconds,
		DockerPath:       dockerPath,
	}, nil
}

func normalizeComposeFile(raw string) (string, error) {
	file := strings.TrimSpace(raw)
	if file == "" {
		return "", fmt.Errorf("compose file path is required")
	}
	if strings.ContainsRune(file, 0) {
		return "", fmt.Errorf("compose file path contains NUL byte")
	}
	cleaned := filepath.Clean(file)
	if filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("compose file path must be relative to projectDirectory")
	}
	if strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) || cleaned == ".." {
		return "", fmt.Errorf("compose file path must not escape project directory")
	}
	return cleaned, nil
}

func validTargetName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '.', r == '_', r == '-':
		default:
			return false
		}
	}
	return true
}

func dockerComposeRestartCommand(spec TargetSpec) (string, []string) {
	args := []string{"compose"}
	for _, file := range spec.ComposeFiles {
		args = append(args, "-f", file)
	}
	args = append(args, "restart")
	args = append(args, spec.Services...)
	return spec.DockerPath, args
}

func (execCommandRunner) Run(ctx context.Context, dir string, path string, args []string) CommandResult {
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Dir = dir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	output := trimOutput(stdout.String(), stderr.String())
	if ctx.Err() == context.DeadlineExceeded {
		return CommandResult{ExitCode: -1, Output: output, TimedOut: true, Err: fmt.Errorf("node-agent command timed out")}
	}
	if err != nil {
		exitCode := -1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
		return CommandResult{ExitCode: exitCode, Output: output, Err: err}
	}
	return CommandResult{ExitCode: 0, Output: output}
}

func trimOutput(stdout, stderr string) string {
	output := strings.TrimSpace(strings.Join([]string{strings.TrimSpace(stdout), strings.TrimSpace(stderr)}, "\n"))
	runes := []rune(output)
	if len(runes) <= maxCommandOutputBytes {
		return output
	}
	return strings.TrimSpace(string(runes[:maxCommandOutputBytes])) + "..."
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
