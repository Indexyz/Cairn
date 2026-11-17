// Package api is the HTTP layer.
package api

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/Indexyz/Cairn/server/internal/domain"
	"github.com/Indexyz/Cairn/server/internal/modelhealth"
	"github.com/Indexyz/Cairn/server/internal/notify"
	"github.com/Indexyz/Cairn/server/internal/store"
)

//go:embed all:static
var staticFS embed.FS

// Version is reported by the server banner.
const Version = "0.2.1"

// App wires the database, sender and HTTP client into a handler.
type App struct {
	DB         *store.DB
	Deliverer  *notify.Deliverer
	HTTPClient *http.Client
	AccessLog  bool
	Logger     func(format string, args ...any)
}

// New builds the handler.
func (app *App) Handler() http.Handler {
	if app.Deliverer == nil {
		app.Deliverer = &notify.Deliverer{Client: app.HTTPClient}
	}
	if app.Logger == nil {
		app.Logger = func(string, ...any) {}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /settings", app.handle(app.getSettings))
	mux.HandleFunc("PUT /settings", app.handle(app.updateSettings))

	mux.HandleFunc("GET /models", app.handle(app.listModels))
	mux.HandleFunc("POST /models", app.handle(app.createModel))
	mux.HandleFunc("PUT /models", app.handle(app.replaceModels))
	mux.HandleFunc("GET /load", app.handle(app.getLoad))
	mux.HandleFunc("PUT /models/{name}", app.handle(app.updateModel))
	mux.HandleFunc("DELETE /models/{name}", app.handle(app.deleteModel))
	mux.HandleFunc("POST /models/{name}/test", app.handle(app.testModel))

	mux.HandleFunc("GET /notifications", app.handle(app.listChannels))
	mux.HandleFunc("POST /notifications", app.handle(app.createChannel))
	mux.HandleFunc("PUT /notifications/{channel}", app.handle(app.updateChannel))
	mux.HandleFunc("DELETE /notifications/{channel}", app.handle(app.deleteChannel))
	mux.HandleFunc("POST /notifications/{channel}/test", app.handle(app.testChannel))

	mux.HandleFunc("GET /projects", app.handle(app.listProjects))
	mux.HandleFunc("POST /projects", app.handle(app.createProject))
	mux.HandleFunc("POST /projects/{project}/fork", app.handle(app.forkProject))
	mux.HandleFunc("GET /projects/{project}", app.handle(app.getProject))
	mux.HandleFunc("DELETE /projects/{project}", app.handle(app.deleteProject))
	mux.HandleFunc("PUT /projects/{project}/title", app.handle(app.updateProjectTitle))
	mux.HandleFunc("PUT /projects/{project}/max_steps", app.handle(app.updateProjectMaxSteps))
	mux.HandleFunc("PUT /projects/{project}/status", app.handle(app.updateProjectStatus))
	mux.HandleFunc("POST /projects/{project}/reason/claim", app.handle(app.claimReason))
	mux.HandleFunc("POST /projects/{project}/reason/heartbeat", app.handle(app.heartbeatReason))
	mux.HandleFunc("POST /projects/{project}/reason/release", app.handle(app.releaseReason))
	mux.HandleFunc("POST /projects/{project}/complete", app.handle(app.completeProject))
	mux.HandleFunc("POST /projects/{project}/reopen", app.handle(app.reopenProject))
	mux.HandleFunc("GET /projects/{project}/export", app.handle(app.exportProject))
	mux.HandleFunc("POST /projects/{project}/hints", app.handle(app.createHint))
	mux.HandleFunc("POST /projects/{project}/intents", app.handle(app.createIntent))
	mux.HandleFunc("POST /projects/{project}/intents/{intent}/heartbeat", app.handle(app.heartbeatIntent))
	mux.HandleFunc("POST /projects/{project}/intents/{intent}/release", app.handle(app.releaseIntent))
	mux.HandleFunc("POST /projects/{project}/intents/{intent}/conclude", app.handle(app.concludeIntent))

	mux.HandleFunc("GET /openapi.json", app.serveOpenAPI)
	mux.HandleFunc("GET /docs", app.serveDocs)
	mux.HandleFunc("GET /", app.serveIndex)
	assets, _ := fs.Sub(staticFS, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(assets))))
	return app.accessLog(mux)
}

type handlerFunc func(w http.ResponseWriter, r *http.Request) error

func (app *App) handle(fn handlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := fn(w, r); err != nil {
			writeError(w, err)
		}
	}
}

func (app *App) serveIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		writeError(w, domain.NotFound("Not Found"))
		return
	}
	page, err := staticFS.ReadFile("static/index.html")
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("content-type", "text/html; charset=utf-8")
	_, _ = w.Write(page)
}

func (app *App) accessLog(next http.Handler) http.Handler {
	if !app.AccessLog {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		app.Logger("%s %s %s", r.Method, r.URL.Path, time.Since(started))
	})
}

// tx writes directly; handlers embed the writer through request context to keep
// signatures small.
func (app *App) tx(w http.ResponseWriter, r *http.Request, fn func(ctx context.Context, conn *sql.Conn) (int, any, error)) error {
	var status int
	var body any
	err := app.DB.WithTx(r.Context(), func(conn *sql.Conn) error {
		var inner error
		status, body, inner = fn(r.Context(), conn)
		return inner
	})
	if err != nil {
		return err
	}
	if status == http.StatusNoContent {
		writeNoContent(w)
		return nil
	}
	writeJSON(w, status, body)
	return nil
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	if body != nil {
		_ = json.NewEncoder(w).Encode(body)
	}
}

func writeNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

func writeError(w http.ResponseWriter, err error) {
	var validation *domain.ValidationError
	if errors.As(err, &validation) {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"detail": validation.Errors})
		return
	}
	var httpError *domain.HTTPError
	if errors.As(err, &httpError) {
		writeJSON(w, httpError.Status, map[string]any{"detail": httpError.Detail})
		return
	}
	writeJSON(w, http.StatusInternalServerError, map[string]any{"detail": err.Error()})
}

func decodeBody(r *http.Request, target any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 8<<20))
	if err := decoder.Decode(target); err != nil {
		return &domain.ValidationError{Errors: []domain.FieldError{{
			Type: "value_error", Loc: []any{"body"}, Msg: "Input should be a valid object",
		}}}
	}
	return nil
}

// decodeOptional decodes an empty body as nil.
func decodeOptional(r *http.Request, target any) error {
	raw, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(raw)) == "" {
		return nil
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return &domain.ValidationError{Errors: []domain.FieldError{{
			Type: "value_error", Loc: []any{"body"}, Msg: "Input should be a valid object",
		}}}
	}
	return nil
}

func (app *App) checkModel(ctx context.Context, model *domain.WorkerModel, timeout time.Duration) (domain.ModelTestResult, error) {
	started := time.Now()
	result, err := modelhealth.Check(app.HTTPClient, model, timeout)
	duration := int(time.Since(started).Milliseconds())
	if err != nil {
		var missing modelhealth.MissingEnvError
		if errors.As(err, &missing) {
			return domain.ModelTestResult{}, &domain.HTTPError{Status: http.StatusUnprocessableEntity, Detail: "model " + model.Name + " is missing env key " + missing.Key}
		}
		return domain.ModelTestResult{}, &domain.HTTPError{Status: http.StatusUnprocessableEntity, Detail: err.Error()}
	}
	return domain.ModelTestResult{
		OK: result.OK, Status: result.Status, Detail: result.Detail,
		Endpoint: modelhealth.Describe(model), DurationMs: duration,
	}, nil
}

// pendingNotify collects matching channels inside the transaction.
func (app *App) pendingNotify(ctx context.Context, conn *sql.Conn, event, projectID, title, detail string) {
	rows, err := conn.QueryContext(ctx,
		"SELECT id, name, kind, events, config FROM notification_channels WHERE enabled = 1 ORDER BY created_at, id")
	if err != nil {
		return
	}
	defer rows.Close()
	var targets []notify.Channel
	for rows.Next() {
		channel := notify.Channel{}
		var events, config string
		if err := rows.Scan(&channel.ID, &channel.Name, &channel.Kind, &events, &config); err != nil {
			return
		}
		if err := json.Unmarshal([]byte(events), &channel.Events); err != nil {
			continue
		}
		if err := json.Unmarshal([]byte(config), &channel.Config); err != nil {
			continue
		}
		if !contains(channel.Events, event) {
			continue
		}
		targets = append(targets, channel)
	}
	if len(targets) == 0 {
		return
	}
	text := notify.RenderMessage(event, projectID, title, detail)
	app.Deliverer.Notify(targets, event, projectID, title, text)
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
