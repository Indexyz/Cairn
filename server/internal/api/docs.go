package api

import (
	"net/http"
	"strings"
)

var openAPIPaths = map[string][]string{
	"/api/settings":                                      {"get", "put"},
	"/api/models":                                        {"get", "post", "put"},
	"/api/load":                                          {"get"},
	"/api/models/{name}":                                 {"put", "delete"},
	"/api/models/{name}/test":                            {"post"},
	"/api/mcp":                                           {"get", "post", "put"},
	"/api/mcp/{name}":                                    {"put", "delete"},
	"/api/notifications":                                 {"get", "post"},
	"/api/notifications/{channel}":                       {"put", "delete"},
	"/api/notifications/{channel}/test":                  {"post"},
	"/api/schedules":                                     {"get", "post"},
	"/api/schedules/{schedule}":                          {"put", "delete"},
	"/api/schedules/{schedule}/run":                      {"post"},
	"/api/projects":                                      {"get", "post"},
	"/api/projects/{project}":                            {"get", "delete"},
	"/api/projects/{project}/fork":                       {"post"},
	"/api/projects/{project}/title":                      {"put"},
	"/api/projects/{project}/max_steps":                  {"put"},
	"/api/projects/{project}/status":                     {"put"},
	"/api/projects/{project}/reason/claim":               {"post"},
	"/api/projects/{project}/reason/heartbeat":           {"post"},
	"/api/projects/{project}/reason/release":             {"post"},
	"/api/projects/{project}/complete":                   {"post"},
	"/api/projects/{project}/reopen":                     {"post"},
	"/api/projects/{project}/export":                     {"get"},
	"/api/projects/{project}/hints":                      {"post"},
	"/api/projects/{project}/intents":                    {"post"},
	"/api/projects/{project}/intents/{intent}/heartbeat": {"post"},
	"/api/projects/{project}/intents/{intent}/release":   {"post"},
	"/api/projects/{project}/intents/{intent}/conclude":  {"post"},
}

func (app *App) openAPISpec() map[string]any {
	paths := map[string]any{}
	for route, methods := range openAPIPaths {
		operations := map[string]any{}
		for _, method := range methods {
			operations[method] = map[string]any{"responses": map[string]any{"200": map[string]any{"description": "OK"}}}
		}
		paths[route] = operations
	}
	return map[string]any{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title": "Cairn", "description": "Fact-graph based collaborative exploration protocol", "version": Version,
		},
		"paths": paths,
	}
}

func (app *App) serveOpenAPI(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, app.openAPISpec())
}

const docsPage = `<!doctype html>
<html>
<head>
  <meta charset="utf-8" />
  <title>Cairn API docs</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
</head>
<body>
<div id="swagger-ui"></div>
<script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
<script>
  window.ui = SwaggerUIBundle({ url: "/api/openapi.json", dom_id: "#swagger-ui" });
</script>
</body>
</html>
`

func (app *App) serveDocs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("content-type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(strings.TrimSpace(docsPage) + "\n"))
}
