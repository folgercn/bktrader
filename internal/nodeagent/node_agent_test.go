package nodeagent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

const testToken = "test-node-agent-token"

type fakeRunner struct {
	dir      string
	path     string
	args     []string
	result   CommandResult
	calls    int
	deadline bool
}

func (r *fakeRunner) Run(ctx context.Context, dir string, path string, args []string) CommandResult {
	r.calls++
	r.dir = dir
	r.path = path
	r.args = append([]string(nil), args...)
	if _, ok := ctx.Deadline(); ok {
		r.deadline = true
	}
	return r.result
}

func TestAgentHealthRequiresAuthAndReturnsTargetNames(t *testing.T) {
	agent := newTestAgent(t, nil)
	server := httptest.NewServer(agent.Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/v1/health")
	if err != nil {
		t.Fatalf("GET health without auth failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized health without token, got %d", resp.StatusCode)
	}

	req, err := http.NewRequest(http.MethodGet, server.URL+"/v1/health", nil)
	if err != nil {
		t.Fatalf("build health request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+testToken)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET health failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected health 200, got %d", resp.StatusCode)
	}
	var health HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatalf("decode health: %v", err)
	}
	if health.Status != "ok" || health.ExecutorKind != executorKindNodeAgent || !health.TokenConfigured {
		t.Fatalf("unexpected health response: %+v", health)
	}
	if !reflect.DeepEqual(health.AllowlistedTargets, []string{"api", "worker"}) {
		t.Fatalf("expected sorted target names only, got %+v", health.AllowlistedTargets)
	}
}

func TestAgentRestartRunsFixedComposeCommand(t *testing.T) {
	runner := &fakeRunner{result: CommandResult{ExitCode: 0, Output: "restarted"}}
	agent := newTestAgent(t, runner)
	server := httptest.NewServer(agent.Handler())
	defer server.Close()

	response := postRestart(t, server.URL, testToken, RestartRequest{
		RequestID:      "req-1",
		TargetName:     "api",
		Action:         actionContainerRestart,
		Reason:         "operator reviewed static plan",
		PlanReason:     "healthz failed",
		Source:         "dashboard",
		Operator:       "folgercn",
		EpisodeStarted: "2026-05-16T03:00:00Z",
		CandidateSince: "2026-05-16T03:01:00Z",
	}, http.StatusOK)

	if runner.calls != 1 {
		t.Fatalf("expected one command call, got %d", runner.calls)
	}
	if runner.dir != "/opt/bktrader" || runner.path != "docker" {
		t.Fatalf("unexpected command location: dir=%q path=%q", runner.dir, runner.path)
	}
	wantArgs := []string{"compose", "-f", "deployments/docker-compose.prod.yml", "restart", "platform-api"}
	if !reflect.DeepEqual(runner.args, wantArgs) {
		t.Fatalf("expected fixed docker compose args %v, got %v", wantArgs, runner.args)
	}
	if !runner.deadline {
		t.Fatal("expected command runner context to include timeout")
	}
	if !response.Executed || response.ExitCode == nil || *response.ExitCode != 0 || response.DurationMs < 0 || response.Message != "restarted" {
		t.Fatalf("unexpected restart response: %+v", response)
	}
}

func TestAgentRestartRejectsUnknownTargetAndEmptyReason(t *testing.T) {
	runner := &fakeRunner{}
	agent := newTestAgent(t, runner)
	server := httptest.NewServer(agent.Handler())
	defer server.Close()

	unknown := postRestart(t, server.URL, testToken, RestartRequest{
		TargetName: "missing",
		Action:     actionContainerRestart,
		Reason:     "operator reviewed",
	}, http.StatusBadRequest)
	if !strings.Contains(unknown.Error, "not allowlisted") {
		t.Fatalf("expected allowlist error, got %+v", unknown)
	}
	emptyReason := postRestart(t, server.URL, testToken, RestartRequest{
		TargetName: "api",
		Action:     actionContainerRestart,
	}, http.StatusBadRequest)
	if !strings.Contains(emptyReason.Error, "reason is required") {
		t.Fatalf("expected reason error, got %+v", emptyReason)
	}
	if runner.calls != 0 {
		t.Fatalf("expected rejected requests not to run commands, got %d", runner.calls)
	}
}

func TestAgentRestartFailureReturnsStructuredResponse(t *testing.T) {
	runner := &fakeRunner{result: CommandResult{
		ExitCode: 1,
		Output:   "compose restart failed",
		Err:      errors.New("exit status 1"),
	}}
	agent := newTestAgent(t, runner)
	server := httptest.NewServer(agent.Handler())
	defer server.Close()

	response := postRestart(t, server.URL, testToken, RestartRequest{
		RequestID:  "req-fail",
		TargetName: "api",
		Action:     actionContainerRestart,
		Reason:     "operator reviewed",
	}, http.StatusInternalServerError)

	if response.Executed || response.ExitCode == nil || *response.ExitCode != 1 || response.Message != "compose restart failed" || response.Error != "exit status 1" || response.DurationMs < 0 {
		t.Fatalf("unexpected structured failure response: %+v", response)
	}
	if response.RequestID != "req-fail" || response.ExecutorKind != executorKindNodeAgent {
		t.Fatalf("expected request audit fields, got %+v", response)
	}
}

func TestNewRejectsUnsafeConfig(t *testing.T) {
	tests := []struct {
		name    string
		token   string
		targets string
	}{
		{name: "empty token", token: "", targets: targetsJSON("api", `"projectDirectory":"/opt/bktrader","composeFiles":["deployments/docker-compose.prod.yml"],"services":["platform-api"]`)},
		{name: "example token", token: "agent-token", targets: targetsJSON("api", `"projectDirectory":"/opt/bktrader","composeFiles":["deployments/docker-compose.prod.yml"],"services":["platform-api"]`)},
		{name: "short token", token: "short", targets: targetsJSON("api", `"projectDirectory":"/opt/bktrader","composeFiles":["deployments/docker-compose.prod.yml"],"services":["platform-api"]`)},
		{name: "bad target name", token: testToken, targets: targetsJSON("bad target", `"projectDirectory":"/opt/bktrader","composeFiles":["deployments/docker-compose.prod.yml"],"services":["platform-api"]`)},
		{name: "relative project dir", token: testToken, targets: targetsJSON("api", `"projectDirectory":"opt/bktrader","composeFiles":["deployments/docker-compose.prod.yml"],"services":["platform-api"]`)},
		{name: "compose escapes project", token: testToken, targets: targetsJSON("api", `"projectDirectory":"/opt/bktrader","composeFiles":["../docker-compose.yml"],"services":["platform-api"]`)},
		{name: "absolute compose file", token: testToken, targets: targetsJSON("api", `"projectDirectory":"/opt/bktrader","composeFiles":["/tmp/docker-compose.yml"],"services":["platform-api"]`)},
		{name: "bad service name", token: testToken, targets: targetsJSON("api", `"projectDirectory":"/opt/bktrader","composeFiles":["deployments/docker-compose.prod.yml"],"services":["bad service"]`)},
		{name: "unsupported action", token: testToken, targets: targetsJSON("api", `"action":"shell","projectDirectory":"/opt/bktrader","composeFiles":["deployments/docker-compose.prod.yml"],"services":["platform-api"]`)},
		{name: "unsupported executor", token: testToken, targets: targetsJSON("api", `"executor":"docker-socket","projectDirectory":"/opt/bktrader","composeFiles":["deployments/docker-compose.prod.yml"],"services":["platform-api"]`)},
		{name: "timeout too large", token: testToken, targets: targetsJSON("api", `"projectDirectory":"/opt/bktrader","composeFiles":["deployments/docker-compose.prod.yml"],"services":["platform-api"],"timeoutSeconds":301`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := New(Config{Token: tt.token, TargetsRaw: tt.targets}); err == nil {
				t.Fatal("expected unsafe config to fail")
			}
		})
	}
}

func newTestAgent(t *testing.T, runner CommandRunner) *Agent {
	t.Helper()
	if runner == nil {
		runner = &fakeRunner{result: CommandResult{ExitCode: 0}}
	}
	agent, err := New(Config{
		Token: testToken,
		TargetsRaw: `{
			"targets": {
				"worker": {
					"projectDirectory": "/opt/bktrader",
					"composeFiles": ["deployments/docker-compose.prod.yml"],
					"services": ["platform-worker"]
				},
				"api": {
					"action": "container-restart",
					"executor": "docker-compose",
					"projectDirectory": "/opt/bktrader",
					"composeFiles": ["deployments/docker-compose.prod.yml"],
					"services": ["platform-api"],
					"timeoutSeconds": 30,
					"dockerPath": "docker"
				}
			}
		}`,
		Version: "test",
		Runner:  runner,
	})
	if err != nil {
		t.Fatalf("New agent failed: %v", err)
	}
	return agent
}

func postRestart(t *testing.T, baseURL string, token string, request RestartRequest, wantStatus int) RestartResponse {
	t.Helper()
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("marshal restart request: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, baseURL+"/v1/container-fallback/restart", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build restart request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST restart failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != wantStatus {
		t.Fatalf("expected status %d, got %d", wantStatus, resp.StatusCode)
	}
	var response RestartResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("decode restart response: %v", err)
	}
	return response
}

func targetsJSON(name string, fields string) string {
	return `{"targets":{` + strconvQuote(name) + `:{` + fields + `}}}`
}

func strconvQuote(value string) string {
	data, _ := json.Marshal(value)
	return string(data)
}
