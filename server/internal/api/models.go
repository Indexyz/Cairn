package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/Indexyz/Cairn/server/internal/domain"
)

func scanModel(scanner interface{ Scan(...any) error }) (*domain.WorkerModel, error) {
	model := &domain.WorkerModel{}
	var taskTypes, env string
	if err := scanner.Scan(&model.Name, &model.Type, &taskTypes, &model.MaxRunning, &model.Priority, &env); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(taskTypes), &model.TaskTypes); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(env), &model.Env); err != nil {
		return nil, err
	}
	return model, nil
}

func listModels(ctx context.Context, conn *sql.Conn) ([]domain.WorkerModel, error) {
	rows, err := conn.QueryContext(ctx, "SELECT name, type, task_types, max_running, priority, env FROM models ORDER BY priority, name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	models := []domain.WorkerModel{}
	for rows.Next() {
		model, err := scanModel(rows)
		if err != nil {
			return nil, err
		}
		models = append(models, *model)
	}
	load, err := domain.BuildLoadSnapshot(ctx, conn)
	if err != nil {
		return nil, err
	}
	for index := range models {
		models[index].Used = load.Workers[models[index].Name]
	}
	return models, nil
}

func insertModel(ctx context.Context, conn *sql.Conn, model *domain.WorkerModel, now string) error {
	taskTypes, _ := json.Marshal(model.TaskTypes)
	env, _ := json.Marshal(model.Env)
	_, err := conn.ExecContext(ctx,
		"INSERT INTO models (name, type, task_types, max_running, priority, env, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		model.Name, model.Type, string(taskTypes), model.MaxRunning, model.Priority, string(env), now, now)
	return err
}

func (app *App) listModels(w http.ResponseWriter, r *http.Request) error {
	return app.tx(w, r, func(ctx context.Context, conn *sql.Conn) (int, any, error) {
		models, err := listModels(ctx, conn)
		if err != nil {
			return 0, nil, err
		}
		return http.StatusOK, models, nil
	})
}

func (app *App) createModel(w http.ResponseWriter, r *http.Request) error {
	raw, err := readBody(r)
	if err != nil {
		return err
	}
	model, err := domain.ParseWorkerModel(raw)
	if err != nil {
		return err
	}
	if err := checkMockModel(model); err != nil {
		return err
	}
	return app.tx(w, r, func(ctx context.Context, conn *sql.Conn) (int, any, error) {
		var one int
		err := conn.QueryRowContext(ctx, "SELECT 1 FROM models WHERE name = ?", model.Name).Scan(&one)
		if err == nil {
			return 0, nil, domain.Conflict("model " + model.Name + " already exists")
		}
		if err != sql.ErrNoRows {
			return 0, nil, err
		}
		if err := insertModel(ctx, conn, model, domain.UtcNow()); err != nil {
			return 0, nil, err
		}
		return http.StatusCreated, model, nil
	})
}

func (app *App) replaceModels(w http.ResponseWriter, r *http.Request) error {
	raw, err := readBody(r)
	if err != nil {
		return err
	}
	var envelope struct {
		Models []json.RawMessage `json:"models"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return &domain.ValidationError{Errors: []domain.FieldError{{Loc: []any{"body"}, Msg: "Input should be a valid object"}}}
	}
	var presence map[string]json.RawMessage
	_ = json.Unmarshal(raw, &presence)
	if _, present := presence["models"]; !present {
		return &domain.ValidationError{Errors: []domain.FieldError{{Loc: []any{"body", "models"}, Msg: "Field required"}}}
	}
	request := &domain.ReplaceModelsRequest{}
	for _, item := range envelope.Models {
		model, err := domain.ParseWorkerModel(item)
		if err != nil {
			return err
		}
		if err := checkMockModel(model); err != nil {
			return err
		}
		request.Models = append(request.Models, *model)
	}
	if request.Models == nil {
		request.Models = []domain.WorkerModel{}
	}
	if validation := request.Validate(); validation != nil {
		return validation
	}
	return app.tx(w, r, func(ctx context.Context, conn *sql.Conn) (int, any, error) {
		now := domain.UtcNow()
		keep := map[string]bool{}
		for _, model := range request.Models {
			keep[model.Name] = true
		}
		rows, err := conn.QueryContext(ctx, "SELECT name FROM models")
		if err != nil {
			return 0, nil, err
		}
		var existing []string
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				rows.Close()
				return 0, nil, err
			}
			existing = append(existing, name)
		}
		rows.Close()
		for _, name := range existing {
			if !keep[name] {
				if _, err := conn.ExecContext(ctx, "DELETE FROM models WHERE name = ?", name); err != nil {
					return 0, nil, err
				}
			}
		}
		for _, model := range request.Models {
			taskTypes, _ := json.Marshal(model.TaskTypes)
			env, _ := json.Marshal(model.Env)
			if _, err := conn.ExecContext(ctx, `
				INSERT INTO models (name, type, task_types, max_running, priority, env, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)
				ON CONFLICT(name) DO UPDATE SET
					type = excluded.type, task_types = excluded.task_types, max_running = excluded.max_running,
					priority = excluded.priority, env = excluded.env, updated_at = excluded.updated_at`,
				model.Name, model.Type, string(taskTypes), model.MaxRunning, model.Priority, string(env), now, now); err != nil {
				return 0, nil, err
			}
		}
		models, err := listModels(ctx, conn)
		if err != nil {
			return 0, nil, err
		}
		return http.StatusOK, models, nil
	})
}

func (app *App) getLoad(w http.ResponseWriter, r *http.Request) error {
	return app.tx(w, r, func(ctx context.Context, conn *sql.Conn) (int, any, error) {
		load, err := domain.BuildLoadSnapshot(ctx, conn)
		if err != nil {
			return 0, nil, err
		}
		return http.StatusOK, load, nil
	})
}

func (app *App) updateModel(w http.ResponseWriter, r *http.Request) error {
	name := r.PathValue("name")
	raw, err := readBody(r)
	if err != nil {
		return err
	}
	model, err := domain.ParseWorkerModel(raw)
	if err != nil {
		return err
	}
	if err := checkMockModel(model); err != nil {
		return err
	}
	return app.tx(w, r, func(ctx context.Context, conn *sql.Conn) (int, any, error) {
		var one int
		err := conn.QueryRowContext(ctx, "SELECT 1 FROM models WHERE name = ?", name).Scan(&one)
		if err == sql.ErrNoRows {
			return 0, nil, domain.NotFound("model " + name + " not found")
		}
		if err != nil {
			return 0, nil, err
		}
		if model.Name != name {
			err := conn.QueryRowContext(ctx, "SELECT 1 FROM models WHERE name = ?", model.Name).Scan(&one)
			if err == nil {
				return 0, nil, domain.Conflict("model " + model.Name + " already exists")
			}
		}
		taskTypes, _ := json.Marshal(model.TaskTypes)
		env, _ := json.Marshal(model.Env)
		if _, err := conn.ExecContext(ctx,
			"UPDATE models SET name = ?, type = ?, task_types = ?, max_running = ?, priority = ?, env = ?, updated_at = ? WHERE name = ?",
			model.Name, model.Type, string(taskTypes), model.MaxRunning, model.Priority, string(env), domain.UtcNow(), name); err != nil {
			return 0, nil, err
		}
		return http.StatusOK, model, nil
	})
}

func requireModel(ctx context.Context, conn *sql.Conn, name string) (*domain.WorkerModel, error) {
	model, err := scanModel(conn.QueryRowContext(ctx,
		"SELECT name, type, task_types, max_running, priority, env FROM models WHERE name = ?", name))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.NotFound("model " + name + " not found")
		}
		return nil, err
	}
	return model, nil
}

func (app *App) deleteModel(w http.ResponseWriter, r *http.Request) error {
	name := r.PathValue("name")
	return app.tx(w, r, func(ctx context.Context, conn *sql.Conn) (int, any, error) {
		if _, err := requireModel(ctx, conn, name); err != nil {
			return 0, nil, err
		}
		if _, err := conn.ExecContext(ctx, "DELETE FROM models WHERE name = ?", name); err != nil {
			return 0, nil, err
		}
		return http.StatusNoContent, nil, nil
	})
}

func (app *App) testModel(w http.ResponseWriter, r *http.Request) error {
	name := r.PathValue("name")
	timeout := 10 * time.Second
	if raw := r.URL.Query().Get("timeout"); raw != "" {
		parsed, ok := parseTimeout(raw)
		if !ok {
			return &domain.ValidationError{Errors: []domain.FieldError{{Loc: []any{"query", "timeout"}, Msg: "Input should be greater than 0 and less than or equal to 60"}}}
		}
		timeout = parsed
	}
	var model *domain.WorkerModel
	if err := app.DB.Read(r.Context(), func(conn *sql.Conn) error {
		loaded, err := requireModel(r.Context(), conn, name)
		model = loaded
		return err
	}); err != nil {
		return err
	}
	result, err := app.checkModel(r.Context(), model, timeout)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, result)
	return nil
}

func checkMockModel(model *domain.WorkerModel) error {
	if model.Type != domain.TypeMock {
		return nil
	}
	if _, err := resolveMock(model); err != nil {
		return &domain.ValidationError{Errors: []domain.FieldError{{
			Type: "value_error", Loc: []any{"body", "env"}, Msg: err.Error(),
		}}}
	}
	return nil
}
