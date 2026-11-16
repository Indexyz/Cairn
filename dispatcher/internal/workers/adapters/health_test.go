package adapters

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Indexyz/Cairn/dispatcher/internal/config"
)

func recordingServer(t *testing.T) (*httptest.Server, *http.Request, *http.Header) {
	t.Helper()
	captured := new(http.Request)
	headers := new(http.Header)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*captured = *r.Clone(r.Context())
		*headers = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	return server, captured, headers
}

func TestClaudeCodeCheckHealthHitsMessagesWithBearer(t *testing.T) {
	server, request, headers := recordingServer(t)
	value := worker(config.WorkerClaudeCode, map[string]string{
		"ANTHROPIC_BASE_URL":   server.URL,
		"ANTHROPIC_AUTH_TOKEN": "tok",
		"ANTHROPIC_MODEL":      "m",
	})

	result := ClaudeCode{}.CheckHealth(value, time.Second)

	if !result.OK {
		t.Fatalf("result = %+v", result)
	}
	if request.URL.Path != "/v1/messages" {
		t.Fatalf("path = %q", request.URL.Path)
	}
	if headers.Get("authorization") != "Bearer tok" {
		t.Fatalf("headers = %v", *headers)
	}
}

func TestCodexCheckHealthHitsResponses(t *testing.T) {
	server, request, headers := recordingServer(t)
	value := worker(config.WorkerCodex, map[string]string{
		"CODEX_BASE_URL": server.URL + "/v1",
		"OPENAI_API_KEY": "k",
		"CODEX_MODEL":    "m",
	})

	NewCodex(false).CheckHealth(value, time.Second)

	if request.URL.Path != "/v1/responses" {
		t.Fatalf("path = %q", request.URL.Path)
	}
	if headers.Get("authorization") != "Bearer k" {
		t.Fatalf("headers = %v", *headers)
	}
}

func TestPiCheckHealthUsesProviderAPIShape(t *testing.T) {
	server, request, headers := recordingServer(t)
	value := worker(config.WorkerPi, map[string]string{
		"PI_BASE_URL":     server.URL + "/v1",
		"PI_API_KEY":      "k",
		"PI_MODEL":        "m",
		"PI_PROVIDER_API": "openai-completions",
	})

	NewPi(false).CheckHealth(value, time.Second)
	if request.URL.Path != "/v1/chat/completions" {
		t.Fatalf("openai path = %q", request.URL.Path)
	}

	anthropic := worker(config.WorkerPi, map[string]string{
		"PI_BASE_URL":     server.URL,
		"PI_API_KEY":      "k",
		"PI_MODEL":        "m",
		"PI_PROVIDER_API": "anthropic-messages",
	})
	NewPi(false).CheckHealth(anthropic, time.Second)
	if request.URL.Path != "/v1/messages" {
		t.Fatalf("anthropic path = %q", request.URL.Path)
	}
	if headers.Get("anthropic-version") == "" {
		t.Fatal("anthropic-version header missing")
	}
}

func TestPiCheckHealthUsesWorkerProxy(t *testing.T) {
	var proxied bool
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxied = true
		w.WriteHeader(http.StatusOK)
	}))
	defer proxy.Close()

	value := worker(config.WorkerClaudeCode, map[string]string{
		"ANTHROPIC_BASE_URL":   "http://unreachable.invalid",
		"ANTHROPIC_AUTH_TOKEN": "t",
		"ANTHROPIC_MODEL":      "m",
		"https_proxy":          proxy.URL,
		"http_proxy":           proxy.URL,
	})

	ClaudeCode{}.CheckHealth(value, time.Second)

	if !proxied {
		t.Fatal("proxy was not used")
	}
}
