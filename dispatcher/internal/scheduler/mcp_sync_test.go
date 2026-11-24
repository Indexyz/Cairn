package scheduler

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Indexyz/Cairn/dispatcher/internal/config"
	"github.com/Indexyz/Cairn/dispatcher/internal/protocol"
)

func TestEncodeMCPConfigOmitsEmptyFields(t *testing.T) {
	payload, err := encodeMCPConfig([]protocol.MCPServer{
		{Name: "fs", Type: "stdio", Command: "mcp-fs", Args: []string{"--root", "/work"}, Env: map[string]string{"A": "b"}},
		{Name: "remote", Type: "sse", URL: "https://mcp.test/sse", Headers: map[string]string{"X": "y"}},
	})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	var decoded struct {
		Servers map[string]map[string]any `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	fs := decoded.Servers["fs"]
	if fs["command"] != "mcp-fs" || fs["type"] != "stdio" || fs["url"] != nil || fs["headers"] != nil {
		t.Fatalf("fs = %v", fs)
	}
	remote := decoded.Servers["remote"]
	if remote["url"] != "https://mcp.test/sse" || remote["type"] != "sse" || remote["command"] != nil || remote["args"] != nil {
		t.Fatalf("remote = %v", remote)
	}
}

func mcpServer(t *testing.T, stored *[]map[string]any, modelEnv *map[string]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/mcp":
			_ = json.NewEncoder(w).Encode(*stored)
		case r.Method == http.MethodPut && r.URL.Path == "/api/mcp":
			raw, _ := io.ReadAll(r.Body)
			var envelope struct {
				Servers []map[string]any `json:"servers"`
			}
			if err := json.Unmarshal(raw, &envelope); err != nil {
				t.Errorf("decode: %v", err)
			}
			*stored = envelope.Servers
			_ = json.NewEncoder(w).Encode(*stored)
		case r.Method == http.MethodGet && r.URL.Path == "/api/models":
			item := map[string]any{
				"name": "only", "type": "mock", "task_types": []string{"explore"},
				"max_running": 1, "priority": 0, "env": *modelEnv,
			}
			_ = json.NewEncoder(w).Encode([]map[string]any{item})
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestSyncMCPSeedsFromConfigOnce(t *testing.T) {
	stored := []map[string]any{}
	modelEnv := map[string]string{}
	server := mcpServer(t, &stored, &modelEnv)
	defer server.Close()

	loop := testLoop(t)
	loop.client = protocol.NewClient(server.URL, 5*time.Second, "test")
	loop.Config.MCP = &config.MCPConfig{Servers: map[string]config.MCPServer{
		"fs": {Type: "stdio", Command: "mcp-fs"},
	}}

	if err := loop.syncMCP(true); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if len(stored) != 1 || stored[0]["name"] != "fs" {
		t.Fatalf("stored = %v", stored)
	}
	if !strings.Contains(loop.mcpSnapshot(), `"fs"`) {
		t.Fatalf("payload = %s", loop.mcpSnapshot())
	}

	stored = []map[string]any{{"name": "remote", "type": "sse", "url": "https://mcp.test/sse", "args": []string{}, "env": map[string]string{}, "headers": map[string]string{}}}
	if err := loop.syncMCP(true); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(stored) != 1 || stored[0]["name"] != "remote" || !strings.Contains(loop.mcpSnapshot(), "remote") {
		t.Fatalf("stored=%v payload=%s", stored, loop.mcpSnapshot())
	}
}

func TestSyncWorkersInjectsMCPConfigUnlessWorkerOverrides(t *testing.T) {
	stored := []map[string]any{
		{"name": "fs", "type": "stdio", "command": "mcp-fs", "args": []string{}, "env": map[string]string{}, "url": "", "headers": map[string]string{}},
	}
	modelEnv := map[string]string{}
	server := mcpServer(t, &stored, &modelEnv)
	defer server.Close()

	loop := testLoop(t)
	loop.client = protocol.NewClient(server.URL, 5*time.Second, "test")
	loop.Config.Workers[0].Env = map[string]string{}
	if err := loop.syncMCP(false); err != nil {
		t.Fatalf("sync mcp: %v", err)
	}
	if err := loop.syncWorkers(false); err != nil {
		t.Fatalf("sync workers: %v", err)
	}
	if !strings.Contains(loop.Config.Workers[0].Env["MCP_CONFIG"], "mcp-fs") {
		t.Fatalf("env = %v", loop.Config.Workers[0].Env)
	}

	own := `{"mcpServers":{"own":{"command":"x"}}}`
	modelEnv = map[string]string{"MCP_CONFIG": own}
	if err := loop.syncWorkers(false); err != nil {
		t.Fatalf("sync workers: %v", err)
	}
	if loop.Config.Workers[0].Env["MCP_CONFIG"] != own {
		t.Fatalf("override = %v", loop.Config.Workers[0].Env["MCP_CONFIG"])
	}
}
