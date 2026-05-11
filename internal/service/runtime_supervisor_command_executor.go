package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultRuntimeSupervisorCommandTimeout = 30 * time.Second
	maxRuntimeSupervisorCommandTimeout     = 5 * time.Minute
	maxRuntimeSupervisorCommandOutputBytes = 512
)

type CommandContainerFallbackSpec struct {
	Path    string
	Args    []string
	Timeout time.Duration
}

type CommandContainerFallbackExecutor struct {
	specs map[string]CommandContainerFallbackSpec
}

type commandContainerFallbackSpecJSON struct {
	Path           string   `json:"path"`
	Args           []string `json:"args,omitempty"`
	TimeoutSeconds int      `json:"timeoutSeconds,omitempty"`
}

func NewCommandContainerFallbackExecutor(specs map[string]CommandContainerFallbackSpec) (CommandContainerFallbackExecutor, error) {
	normalized := make(map[string]CommandContainerFallbackSpec, len(specs))
	for targetName, spec := range specs {
		name := strings.TrimSpace(targetName)
		if !validContainerFallbackCommandTargetName(name) {
			return CommandContainerFallbackExecutor{}, fmt.Errorf("container fallback command target name %q is invalid", targetName)
		}
		command, err := normalizeContainerFallbackCommandSpec(spec)
		if err != nil {
			return CommandContainerFallbackExecutor{}, fmt.Errorf("container fallback command target %s: %w", name, err)
		}
		if _, exists := normalized[name]; exists {
			return CommandContainerFallbackExecutor{}, fmt.Errorf("container fallback command target %s is duplicated", name)
		}
		normalized[name] = command
	}
	if len(normalized) == 0 {
		return CommandContainerFallbackExecutor{}, fmt.Errorf("container fallback command allowlist is required")
	}
	return CommandContainerFallbackExecutor{specs: normalized}, nil
}

func NewCommandContainerFallbackExecutorFromJSON(raw string) (CommandContainerFallbackExecutor, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return CommandContainerFallbackExecutor{}, fmt.Errorf("container fallback command allowlist is required")
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	parsed := map[string]commandContainerFallbackSpecJSON{}
	if err := decoder.Decode(&parsed); err != nil {
		return CommandContainerFallbackExecutor{}, fmt.Errorf("container fallback command allowlist JSON is invalid: %w", err)
	}
	specs := make(map[string]CommandContainerFallbackSpec, len(parsed))
	for targetName, spec := range parsed {
		if spec.TimeoutSeconds < 0 {
			return CommandContainerFallbackExecutor{}, fmt.Errorf("container fallback command target %s: timeoutSeconds must be >= 0", strings.TrimSpace(targetName))
		}
		specs[targetName] = CommandContainerFallbackSpec{
			Path:    spec.Path,
			Args:    spec.Args,
			Timeout: time.Duration(spec.TimeoutSeconds) * time.Second,
		}
	}
	return NewCommandContainerFallbackExecutor(specs)
}

func (e CommandContainerFallbackExecutor) Configured() bool {
	return len(e.specs) > 0
}

func (e CommandContainerFallbackExecutor) Descriptor() ContainerFallbackExecutorDescriptor {
	return ContainerFallbackExecutorDescriptor{
		Kind:   runtimeSupervisorContainerExecutorKindCommand,
		DryRun: false,
	}
}

func (e CommandContainerFallbackExecutor) ContainerFallbackTargetAllowed(target RuntimeSupervisorTarget) bool {
	_, ok := e.specs[strings.TrimSpace(target.Name)]
	return ok
}

func (e CommandContainerFallbackExecutor) Restart(ctx context.Context, target RuntimeSupervisorTarget, _ string) (ContainerFallbackExecutionResult, error) {
	spec, ok := e.specs[strings.TrimSpace(target.Name)]
	if !ok {
		return ContainerFallbackExecutionResult{}, fmt.Errorf("container fallback target %q is not allowlisted", strings.TrimSpace(target.Name))
	}
	if ctx == nil {
		ctx = context.Background()
	}
	timeout := spec.Timeout
	if timeout <= 0 {
		timeout = defaultRuntimeSupervisorCommandTimeout
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, spec.Path, spec.Args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	output := trimContainerFallbackCommandOutput(stdout.String(), stderr.String())
	if runCtx.Err() == context.DeadlineExceeded {
		return ContainerFallbackExecutionResult{}, fmt.Errorf("container fallback command timed out after %s", timeout)
	}
	if err != nil {
		if output != "" {
			return ContainerFallbackExecutionResult{}, fmt.Errorf("container fallback command failed: %w: %s", err, output)
		}
		return ContainerFallbackExecutionResult{}, fmt.Errorf("container fallback command failed: %w", err)
	}
	if output == "" {
		output = "command container fallback executor completed"
	}
	return ContainerFallbackExecutionResult{
		Executed: true,
		Message:  output,
	}, nil
}

func normalizeContainerFallbackCommandSpec(spec CommandContainerFallbackSpec) (CommandContainerFallbackSpec, error) {
	path := strings.TrimSpace(spec.Path)
	if path == "" {
		return CommandContainerFallbackSpec{}, fmt.Errorf("path is required")
	}
	if !filepath.IsAbs(path) {
		return CommandContainerFallbackSpec{}, fmt.Errorf("path must be absolute")
	}
	if strings.ContainsRune(path, 0) {
		return CommandContainerFallbackSpec{}, fmt.Errorf("path contains NUL byte")
	}
	timeout := spec.Timeout
	if timeout <= 0 {
		timeout = defaultRuntimeSupervisorCommandTimeout
	}
	if timeout > maxRuntimeSupervisorCommandTimeout {
		return CommandContainerFallbackSpec{}, fmt.Errorf("timeout must be <= %s", maxRuntimeSupervisorCommandTimeout)
	}
	args := make([]string, 0, len(spec.Args))
	for _, arg := range spec.Args {
		if strings.ContainsRune(arg, 0) {
			return CommandContainerFallbackSpec{}, fmt.Errorf("arg contains NUL byte")
		}
		args = append(args, arg)
	}
	return CommandContainerFallbackSpec{
		Path:    path,
		Args:    args,
		Timeout: timeout,
	}, nil
}

func validContainerFallbackCommandTargetName(name string) bool {
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

func trimContainerFallbackCommandOutput(stdout, stderr string) string {
	output := strings.TrimSpace(strings.Join([]string{strings.TrimSpace(stdout), strings.TrimSpace(stderr)}, "\n"))
	runes := []rune(output)
	if len(runes) <= maxRuntimeSupervisorCommandOutputBytes {
		return output
	}
	return strings.TrimSpace(string(runes[:maxRuntimeSupervisorCommandOutputBytes])) + "..."
}
