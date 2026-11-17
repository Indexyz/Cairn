package api

import (
	"net/http"
	"strings"
)

var openAPIPaths = map[string][]string{
	"/settings":                                      {"get", "put"},
	"/models":                                        {"get", "post", "put"},
	"/load":                                          {"get"},
	"/models/{name}":                                 {"put", "delete"},
	"/models/{name}/test":                            {"post"},
	"/notifications":                                 {"get", "post"},
	"/notifications/{channel}":                       {"put", "delete"},
	"/notifications/{channel}/test":                  {"post"},
	"/projects":                                      {"get", "post"},
	"/projects/{project}":                            {"get", "delete"},
	"/projects/{project}/fork":                       {"post"},
	"/projects/{project}/title":                      {"put"},
	"/projects/{project}/max_steps":                  {"put"},
	"/projects/{project}/status":                     {"put"},
	"/projects/{project}/reason/claim":               {"post"},
	"/projects/{project}/reason/heartbeat":           {"post"},
	"/projects/{project}/reason/release":             {"post"},
	"/projects/{project}/complete":                   {"post"},
	"/projects/{project}/reopen":                     {"post"},
	"/projects/{project}/export":                     {"get"},
	"/projects/{project}/hints":                      {"post"},
	"/projects/{project}/intents":                    {"post"},
	"/projects/{project}/intents/{intent}/heartbeat": {"post"},
	"/projects/{project}/intents/{intent}/release":   {"post"},
	"/projects/{project}/intents/{intent}/conclude":  {"post"},
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
  window.ui = SwaggerUIBundle({ url: "/openapi.json", dom_id: "#swagger-ui" });
</script>
</body>
</html>
`

func (app *App) serveDocs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("content-type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(strings.TrimSpace(docsPage) + "\n"))
}
