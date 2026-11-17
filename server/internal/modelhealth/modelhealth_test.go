package modelhealth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Indexyz/Cairn/server/internal/domain"
)

func model(workerType string, env map[string]string) *domain.WorkerModel {
	return &domain.WorkerModel{Name: workerType, Type: workerType, TaskTypes: []string{"reason"}, MaxRunning: 1, Env: env}
}

func recordingServer(t *testing.T) (*httptest.Server, *http.Request, *http.Header) {
	t.Helper()
	request := new(http.Request)
	headers := new(http.Header)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*request = *r.Clone(r.Context())
		*headers = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	return server, request, headers
}

func TestHTTPPingHealthyOn2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	result, err := Check(nil, model(domain.TypeClaudeCode, map[string]string{
		"ANTHROPIC_BASE_URL": server.URL, "ANTHROPIC_AUTH_TOKEN": "tok", "ANTHROPIC_MODEL": "m",
	}), time.Second)
	if err != nil || !result.OK {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestHTTPPingUnhealthyOn4xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("unauthorized token"))
	}))
	defer server.Close()

	result, _ := Check(nil, model(domain.TypeClaudeCode, map[string]string{
		"ANTHROPIC_BASE_URL": server.URL, "ANTHROPIC_AUTH_TOKEN": "tok", "ANTHROPIC_MODEL": "m",
	}), time.Second)
	if result.OK || result.Status == nil || *result.Status != 401 || !strings.Contains(result.Detail, "unauthorized") {
		t.Fatalf("result = %+v", result)
	}
}

func TestHTTPPingUnhealthyOnConnectionError(t *testing.T) {
	result, _ := Check(nil, model(domain.TypeClaudeCode, map[string]string{
		"ANTHROPIC_BASE_URL": "http://127.0.0.1:1", "ANTHROPIC_AUTH_TOKEN": "tok", "ANTHROPIC_MODEL": "m",
	}), 500*time.Millisecond)
	if result.OK || result.Status != nil || result.Detail == "" {
		t.Fatalf("result = %+v", result)
	}
}

func TestClaudeCodeCheckHitsMessagesWithBearer(t *testing.T) {
	server, request, headers := recordingServer(t)
	result, err := Check(nil, model(domain.TypeClaudeCode, map[string]string{
		"ANTHROPIC_BASE_URL": server.URL, "ANTHROPIC_AUTH_TOKEN": "tok", "ANTHROPIC_MODEL": "m",
	}), 7*time.Second)
	if err != nil || !result.OK {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if request.URL.Path != "/v1/messages" || headers.Get("authorization") != "Bearer tok" || headers.Get("anthropic-version") == "" {
		t.Fatalf("path=%q headers=%v", request.URL.Path, *headers)
	}
}

func TestCodexCheckHitsResponses(t *testing.T) {
	server, request, headers := recordingServer(t)
	_, _ = Check(nil, model(domain.TypeCodex, map[string]string{
		"CODEX_BASE_URL": server.URL + "/v1", "OPENAI_API_KEY": "k", "CODEX_MODEL": "m",
	}), time.Second)
	if request.URL.Path != "/v1/responses" || headers.Get("authorization") != "Bearer k" {
		t.Fatalf("path=%q headers=%v", request.URL.Path, *headers)
	}
}

func TestPiCheckUsesProviderAPIShape(t *testing.T) {
	server, request, headers := recordingServer(t)
	base := map[string]string{"PI_BASE_URL": server.URL + "/v1", "PI_API_KEY": "k", "PI_MODEL": "m"}

	base["PI_PROVIDER_API"] = "openai-completions"
	_, _ = Check(nil, model(domain.TypePi, base), time.Second)
	if request.URL.Path != "/v1/chat/completions" {
		t.Fatalf("openai path = %q", request.URL.Path)
	}

	base["PI_PROVIDER_API"] = "anthropic-messages"
	base["PI_BASE_URL"] = server.URL
	_, _ = Check(nil, model(domain.TypePi, base), time.Second)
	if request.URL.Path != "/v1/messages" || headers.Get("anthropic-version") == "" {
		t.Fatalf("anthropic path=%q headers=%v", request.URL.Path, *headers)
	}
}

func TestMockCheckReflectsConfiguredOutcome(t *testing.T) {
	ok, err := Check(nil, model(domain.TypeMock, map[string]string{
		"MOCK_HEALTHCHECK": `{"delay":[0,0],"outcomes":{"ok":1.0,"fail":0.0}}`,
	}), time.Second)
	if err != nil || !ok.OK {
		t.Fatalf("ok result=%+v err=%v", ok, err)
	}
	fail, _ := Check(nil, model(domain.TypeMock, map[string]string{
		"MOCK_HEALTHCHECK": `{"delay":[0,0],"outcomes":{"ok":0.0,"fail":1.0}}`,
	}), time.Second)
	if fail.OK || fail.Status == nil || *fail.Status != 503 || fail.Detail != "mock healthcheck fail" {
		t.Fatalf("fail result = %+v", fail)
	}
}

func TestMissingEnvReturnsTypedError(t *testing.T) {
	_, err := Check(nil, model(domain.TypeClaudeCode, map[string]string{}), time.Second)
	missing, ok := err.(MissingEnvError)
	if !ok || missing.Key != "ANTHROPIC_BASE_URL" {
		t.Fatalf("err = %v", err)
	}
}

func TestDescribeNamesTheProbe(t *testing.T) {
	value := model(domain.TypeCodex, map[string]string{"CODEX_BASE_URL": "http://api", "CODEX_MODEL": "m"})
	if got := Describe(value); got != "POST http://api/responses (model=m)" {
		t.Fatalf("describe = %q", got)
	}
}

func TestProxyFromEnvPrefersHTTPS(t *testing.T) {
	env := map[string]string{"https_proxy": "http://p:7897", "all_proxy": "http://all:1"}
	if got := proxyFromEnv(env); got != "http://p:7897" {
		t.Fatalf("proxy = %q", got)
	}
	if got := proxyFromEnv(map[string]string{"all_proxy": "http://all:1"}); got != "http://all:1" {
		t.Fatalf("all proxy = %q", got)
	}
}

func TestMockInvalidConfigSurfacesError(t *testing.T) {
	body, _ := json.Marshal(map[string]any{"delay": []int{0, 1}, "outcomes": map[string]float64{"intent": 0.5}})
	_, err := Check(nil, model(domain.TypeMock, map[string]string{"MOCK_REASON": string(body)}), time.Second)
	if err == nil || !strings.Contains(err.Error(), "probabilities must sum to 1.0") {
		t.Fatalf("err = %v", err)
	}
}
