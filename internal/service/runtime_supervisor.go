package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const defaultRuntimeSupervisorHTTPTimeout = 5 * time.Second

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
	Name          string                 `json:"name"`
	BaseURL       string                 `json:"baseUrl"`
	CheckedAt     time.Time              `json:"checkedAt"`
	Healthz       RuntimeSupervisorProbe `json:"healthz"`
	RuntimeStatus RuntimeSupervisorProbe `json:"runtimeStatus"`
	Status        *RuntimeStatusSnapshot `json:"status,omitempty"`
}

type RuntimeSupervisorSnapshot struct {
	CheckedAt time.Time                         `json:"checkedAt"`
	Targets   []RuntimeSupervisorTargetSnapshot `json:"targets"`
}

type RuntimeSupervisor struct {
	targets []RuntimeSupervisorTarget
	client  *http.Client

	mu       sync.RWMutex
	snapshot RuntimeSupervisorSnapshot
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
		targets: normalized,
		client:  client,
	}
}

func ParseRuntimeSupervisorTargets(rawTargets []string) []RuntimeSupervisorTarget {
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
		if targetSnapshot.RuntimeStatus.Error == "" && targetSnapshot.RuntimeStatus.Reachable {
			targetSnapshot.Status = &status
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
	for _, target := range snapshot.Targets {
		if !target.Healthz.Reachable || target.Healthz.Error != "" {
			unreachable++
		}
		if target.RuntimeStatus.Error != "" {
			runtimeErrors++
		}
	}
	logger.Info("read-only runtime supervisor snapshot collected",
		"target_count", len(snapshot.Targets),
		"unreachable_count", unreachable,
		"runtime_error_count", runtimeErrors,
	)
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
