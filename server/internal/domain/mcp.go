package domain

import (
	"encoding/json"
	"regexp"
	"strings"
)

// NamePattern is the allowed shape of an MCP server name.
var namePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

// MCPServer is one MCP server row.
type MCPServer struct {
	Name      string            `json:"name"`
	Type      string            `json:"type"`
	Command   string            `json:"command"`
	Args      []string          `json:"args"`
	Env       map[string]string `json:"env"`
	URL       string            `json:"url"`
	Headers   map[string]string `json:"headers"`
	CreatedAt string            `json:"created_at"`
	UpdatedAt string            `json:"updated_at"`
}

// ParseMCPServer decodes a strict server payload (unknown keys rejected).
func ParseMCPServer(raw []byte) (*MCPServer, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, fieldError("body", "Input should be a valid object", nil)
	}
	allowed := map[string]bool{
		"name": true, "type": true, "command": true, "args": true,
		"env": true, "url": true, "headers": true, "created_at": true, "updated_at": true,
	}
	for key := range fields {
		if !allowed[key] {
			return nil, fieldError(key, "Extra inputs are not permitted", key)
		}
	}
	server := &MCPServer{Env: map[string]string{}, Headers: map[string]string{}, Args: []string{}}
	if err := decodeField(fields, "name", &server.Name); err != nil {
		return nil, err
	}
	if err := decodeField(fields, "type", &server.Type); err != nil {
		return nil, err
	}
	if err := decodeField(fields, "command", &server.Command); err != nil {
		return nil, err
	}
	if err := decodeField(fields, "args", &server.Args); err != nil {
		return nil, err
	}
	if err := decodeField(fields, "env", &server.Env); err != nil {
		return nil, err
	}
	if err := decodeField(fields, "url", &server.URL); err != nil {
		return nil, err
	}
	if err := decodeField(fields, "headers", &server.Headers); err != nil {
		return nil, err
	}
	if server.Env == nil {
		server.Env = map[string]string{}
	}
	if server.Headers == nil {
		server.Headers = map[string]string{}
	}
	if server.Args == nil {
		server.Args = []string{}
	}
	server.Normalize()
	if validation := server.Validate(); validation != nil {
		return nil, validation
	}
	return server, nil
}

// Normalize trims the transport fields and infers the type from the target.
func (server *MCPServer) Normalize() {
	server.Name = strings.TrimSpace(server.Name)
	server.Command = strings.TrimSpace(server.Command)
	server.URL = strings.TrimSpace(server.URL)
	if server.Type == "" {
		if server.Command != "" {
			server.Type = "stdio"
		} else {
			server.Type = "http"
		}
	}
}

// Validate checks the server payload.
func (server *MCPServer) Validate() *ValidationError {
	if server.Name == "" {
		return fieldError("name", "Field required", nil)
	}
	if !namePattern.MatchString(server.Name) {
		return fieldError("name", "Input should match ^[A-Za-z0-9][A-Za-z0-9_.-]*$", server.Name)
	}
	if server.Type != "stdio" && server.Type != "http" && server.Type != "sse" {
		return fieldError("type", "Input should be 'stdio', 'http' or 'sse'", server.Type)
	}
	if server.Command == "" && server.URL == "" {
		return fieldError("command", "command or url is required", server.Command)
	}
	if server.Command != "" && server.URL != "" {
		return fieldError("url", "Use either command or url, not both", server.URL)
	}
	if server.URL != "" && !strings.HasPrefix(server.URL, "http://") && !strings.HasPrefix(server.URL, "https://") {
		return fieldError("url", "Input should be an http(s) URL", server.URL)
	}
	if server.Type == "stdio" && server.Command == "" {
		return fieldError("command", "stdio servers need a command", server.Command)
	}
	if server.Type != "stdio" && server.URL == "" {
		return fieldError("url", "http and sse servers need a url", server.URL)
	}
	return nil
}

// ReplaceMCPServersRequest is the dispatcher seed snapshot.
type ReplaceMCPServersRequest struct {
	Servers []MCPServer
}

// Validate rejects duplicate names inside one snapshot.
func (request *ReplaceMCPServersRequest) Validate() *ValidationError {
	seen := map[string]bool{}
	for _, server := range request.Servers {
		if seen[server.Name] {
			return fieldError("servers", "names must be unique", server.Name)
		}
		seen[server.Name] = true
	}
	return nil
}
