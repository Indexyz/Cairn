package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func mcpPayload(name string) map[string]any {
	return map[string]any{
		"name":    name,
		"type":    "stdio",
		"command": "npx",
		"args":    []string{"-y", "server"},
		"env":     map[string]string{"TOKEN": "x"},
		"url":     "",
		"headers": map[string]string{},
	}
}

func TestMCPCRUDRoundTrip(t *testing.T) {
	harness := newTestApp(t)
	if status, _ := harness.json(t, http.MethodPost, "/mcp", mcpPayload("fs")); status != http.StatusCreated {
		t.Fatalf("create status = %d", status)
	}
	if status, _ := harness.json(t, http.MethodPost, "/mcp", mcpPayload("fs")); status != http.StatusConflict {
		t.Fatalf("duplicate status = %d", status)
	}

	_, listed := harness.request(t, http.MethodGet, "/mcp", nil)
	var servers []map[string]any
	if err := json.Unmarshal(listed, &servers); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(servers) != 1 || servers[0]["name"] != "fs" || servers[0]["command"] != "npx" {
		t.Fatalf("servers = %v", servers)
	}

	updated := withOverrides(mcpPayload("fs"), map[string]any{"command": "uvx", "args": []string{"srv"}})
	if status, payload := harness.json(t, http.MethodPut, "/mcp/fs", updated); status != http.StatusOK || payload["command"] != "uvx" {
		t.Fatalf("update status=%d payload=%v", status, payload)
	}

	if status, _ := harness.json(t, http.MethodDelete, "/mcp/fs", nil); status != http.StatusOK {
		t.Fatalf("delete status = %d", status)
	}
	if status, _ := harness.json(t, http.MethodDelete, "/mcp/fs", nil); status != http.StatusNotFound {
		t.Fatalf("missing delete status = %d", status)
	}
	if status, _ := harness.json(t, http.MethodPut, "/mcp/fs", updated); status != http.StatusNotFound {
		t.Fatalf("missing update status = %d", status)
	}
}

func TestMCPRenamesAndKeepsTransport(t *testing.T) {
	harness := newTestApp(t)
	if status, _ := harness.json(t, http.MethodPost, "/mcp", mcpPayload("fs")); status != http.StatusCreated {
		t.Fatalf("create status = %d", status)
	}
	renamed := withOverrides(mcpPayload("filesystem"), map[string]any{"type": "http", "command": "", "args": []string{}, "url": "https://mcp.test/sse"})
	if status, payload := harness.json(t, http.MethodPut, "/mcp/fs", renamed); status != http.StatusOK || payload["name"] != "filesystem" || payload["type"] != "http" {
		t.Fatalf("status=%d payload=%v", status, payload)
	}
	_, raw := harness.request(t, http.MethodGet, "/mcp", nil)
	var servers []map[string]any
	if err := json.Unmarshal(raw, &servers); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(servers) != 1 || servers[0]["name"] != "filesystem" {
		t.Fatalf("servers = %v", servers)
	}
}

func TestMCPRejectsInvalidServers(t *testing.T) {
	harness := newTestApp(t)
	cases := []map[string]any{
		withOverrides(mcpPayload("blank-name"), map[string]any{"name": "  "}),
		withOverrides(mcpPayload("bad chars"), map[string]any{"name": "bad chars"}),
		withOverrides(mcpPayload("no-target"), map[string]any{"command": "", "url": ""}),
		withOverrides(mcpPayload("both"), map[string]any{"url": "https://mcp.test"}),
		withOverrides(mcpPayload("bad-type"), map[string]any{"type": "grpc"}),
		withOverrides(mcpPayload("bad-url"), map[string]any{"type": "http", "command": "", "url": "mcp.test"}),
		withOverrides(mcpPayload("stdio-url"), map[string]any{"type": "stdio", "command": "", "url": "https://mcp.test"}),
	}
	for _, body := range cases {
		if status, _ := harness.json(t, http.MethodPost, "/mcp", body); status != http.StatusUnprocessableEntity {
			t.Fatalf("body=%v status=%d", body, status)
		}
	}
	if status, _ := harness.json(t, http.MethodPost, "/mcp", withOverrides(mcpPayload("extra"), map[string]any{"nope": 1})); status != http.StatusUnprocessableEntity {
		t.Fatalf("extra key status = %d", status)
	}
}

func TestMCPReplaceReplacesTheWholeTable(t *testing.T) {
	harness := newTestApp(t)
	if status, _ := harness.json(t, http.MethodPost, "/mcp", mcpPayload("old")); status != http.StatusCreated {
		t.Fatalf("create status = %d", status)
	}
	body := map[string]any{"servers": []any{mcpPayload("a"), mcpPayload("b")}}
	if status, _ := harness.request(t, http.MethodPut, "/mcp", body); status != http.StatusOK {
		t.Fatalf("replace status = %d", status)
	}
	_, raw := harness.request(t, http.MethodGet, "/mcp", nil)
	var replaced []map[string]any
	if err := json.Unmarshal(raw, &replaced); err != nil || len(replaced) != 2 {
		t.Fatalf("replaced=%v err=%v", replaced, err)
	}
	if status, _ := harness.request(t, http.MethodPut, "/mcp", map[string]any{"servers": []any{}}); status != http.StatusOK {
		t.Fatalf("clear status = %d", status)
	}
	_, raw = harness.request(t, http.MethodGet, "/mcp", nil)
	var cleared []map[string]any
	if err := json.Unmarshal(raw, &cleared); err != nil || len(cleared) != 0 {
		t.Fatalf("cleared=%v err=%v", cleared, err)
	}
	if status, _ := harness.json(t, http.MethodPut, "/mcp", map[string]any{}); status != http.StatusUnprocessableEntity {
		t.Fatalf("missing servers status = %d", status)
	}
	if status, _ := harness.json(t, http.MethodPut, "/mcp", map[string]any{"servers": []any{mcpPayload("dup"), mcpPayload("dup")}}); status != http.StatusUnprocessableEntity {
		t.Fatalf("duplicate status = %d", status)
	}
}
