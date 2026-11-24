package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/Indexyz/Cairn/server/internal/domain"
)

func scanMCPServer(scanner interface{ Scan(...any) error }) (*domain.MCPServer, error) {
	server := &domain.MCPServer{}
	var args, env, headers string
	if err := scanner.Scan(&server.Name, &server.Type, &server.Command, &args, &env, &server.URL, &headers, &server.CreatedAt, &server.UpdatedAt); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(args), &server.Args); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(env), &server.Env); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(headers), &server.Headers); err != nil {
		return nil, err
	}
	if server.Args == nil {
		server.Args = []string{}
	}
	if server.Env == nil {
		server.Env = map[string]string{}
	}
	if server.Headers == nil {
		server.Headers = map[string]string{}
	}
	return server, nil
}

const mcpServerColumns = "name, type, command, args, env, url, headers, created_at, updated_at"

func listMCPServers(ctx context.Context, conn *sql.Conn) ([]domain.MCPServer, error) {
	rows, err := conn.QueryContext(ctx, "SELECT "+mcpServerColumns+" FROM mcp_servers ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	servers := []domain.MCPServer{}
	for rows.Next() {
		server, err := scanMCPServer(rows)
		if err != nil {
			return nil, err
		}
		servers = append(servers, *server)
	}
	return servers, nil
}

func insertMCPServer(ctx context.Context, conn *sql.Conn, server *domain.MCPServer, now string) error {
	args, _ := json.Marshal(server.Args)
	env, _ := json.Marshal(server.Env)
	headers, _ := json.Marshal(server.Headers)
	_, err := conn.ExecContext(ctx,
		"INSERT INTO mcp_servers ("+mcpServerColumns+") VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		server.Name, server.Type, server.Command, string(args), string(env), server.URL, string(headers), now, now)
	return err
}

func (app *App) listMCP(w http.ResponseWriter, r *http.Request) error {
	return app.tx(w, r, func(ctx context.Context, conn *sql.Conn) (int, any, error) {
		servers, err := listMCPServers(ctx, conn)
		if err != nil {
			return 0, nil, err
		}
		return http.StatusOK, servers, nil
	})
}

func (app *App) createMCP(w http.ResponseWriter, r *http.Request) error {
	raw, err := readBody(r)
	if err != nil {
		return err
	}
	server, err := domain.ParseMCPServer(raw)
	if err != nil {
		return err
	}
	return app.tx(w, r, func(ctx context.Context, conn *sql.Conn) (int, any, error) {
		var one int
		err := conn.QueryRowContext(ctx, "SELECT 1 FROM mcp_servers WHERE name = ?", server.Name).Scan(&one)
		if err == nil {
			return 0, nil, domain.Conflict("mcp server " + server.Name + " already exists")
		}
		if err != sql.ErrNoRows {
			return 0, nil, err
		}
		now := domain.UtcNow()
		if err := insertMCPServer(ctx, conn, server, now); err != nil {
			return 0, nil, err
		}
		server.CreatedAt, server.UpdatedAt = now, now
		return http.StatusCreated, server, nil
	})
}

func (app *App) replaceMCP(w http.ResponseWriter, r *http.Request) error {
	raw, err := readBody(r)
	if err != nil {
		return err
	}
	var envelope struct {
		Servers []json.RawMessage `json:"servers"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return &domain.ValidationError{Errors: []domain.FieldError{{Loc: []any{"body"}, Msg: "Input should be a valid object"}}}
	}
	var presence map[string]json.RawMessage
	_ = json.Unmarshal(raw, &presence)
	if _, present := presence["servers"]; !present {
		return &domain.ValidationError{Errors: []domain.FieldError{{Loc: []any{"body", "servers"}, Msg: "Field required"}}}
	}
	request := &domain.ReplaceMCPServersRequest{}
	for _, item := range envelope.Servers {
		server, err := domain.ParseMCPServer(item)
		if err != nil {
			return err
		}
		request.Servers = append(request.Servers, *server)
	}
	if request.Servers == nil {
		request.Servers = []domain.MCPServer{}
	}
	if validation := request.Validate(); validation != nil {
		return validation
	}
	return app.tx(w, r, func(ctx context.Context, conn *sql.Conn) (int, any, error) {
		now := domain.UtcNow()
		keep := map[string]bool{}
		for _, server := range request.Servers {
			keep[server.Name] = true
		}
		rows, err := conn.QueryContext(ctx, "SELECT name FROM mcp_servers")
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
				if _, err := conn.ExecContext(ctx, "DELETE FROM mcp_servers WHERE name = ?", name); err != nil {
					return 0, nil, err
				}
			}
		}
		for _, server := range request.Servers {
			args, _ := json.Marshal(server.Args)
			env, _ := json.Marshal(server.Env)
			headers, _ := json.Marshal(server.Headers)
			if _, err := conn.ExecContext(ctx, `
				INSERT INTO mcp_servers (`+mcpServerColumns+`)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
				ON CONFLICT(name) DO UPDATE SET
					type = excluded.type, command = excluded.command, args = excluded.args,
					env = excluded.env, url = excluded.url, headers = excluded.headers, updated_at = excluded.updated_at`,
				server.Name, server.Type, server.Command, string(args), string(env), server.URL, string(headers), now, now); err != nil {
				return 0, nil, err
			}
		}
		servers, err := listMCPServers(ctx, conn)
		if err != nil {
			return 0, nil, err
		}
		return http.StatusOK, servers, nil
	})
}

func (app *App) updateMCP(w http.ResponseWriter, r *http.Request) error {
	name := r.PathValue("name")
	raw, err := readBody(r)
	if err != nil {
		return err
	}
	server, err := domain.ParseMCPServer(raw)
	if err != nil {
		return err
	}
	return app.tx(w, r, func(ctx context.Context, conn *sql.Conn) (int, any, error) {
		existing, err := requireMCPServer(ctx, conn, name)
		if err != nil {
			return 0, nil, err
		}
		if server.Name != name {
			var one int
			err := conn.QueryRowContext(ctx, "SELECT 1 FROM mcp_servers WHERE name = ?", server.Name).Scan(&one)
			if err == nil {
				return 0, nil, domain.Conflict("mcp server " + server.Name + " already exists")
			}
			if err != sql.ErrNoRows {
				return 0, nil, err
			}
		}
		args, _ := json.Marshal(server.Args)
		env, _ := json.Marshal(server.Env)
		headers, _ := json.Marshal(server.Headers)
		if _, err := conn.ExecContext(ctx,
			"UPDATE mcp_servers SET name = ?, type = ?, command = ?, args = ?, env = ?, url = ?, headers = ?, updated_at = ? WHERE name = ?",
			server.Name, server.Type, server.Command, string(args), string(env), server.URL, string(headers), domain.UtcNow(), name); err != nil {
			return 0, nil, err
		}
		server.CreatedAt, server.UpdatedAt = existing.CreatedAt, domain.UtcNow()
		return http.StatusOK, server, nil
	})
}

func (app *App) deleteMCP(w http.ResponseWriter, r *http.Request) error {
	name := r.PathValue("name")
	return app.tx(w, r, func(ctx context.Context, conn *sql.Conn) (int, any, error) {
		if _, err := requireMCPServer(ctx, conn, name); err != nil {
			return 0, nil, err
		}
		if _, err := conn.ExecContext(ctx, "DELETE FROM mcp_servers WHERE name = ?", name); err != nil {
			return 0, nil, err
		}
		return http.StatusOK, map[string]any{"deleted": name}, nil
	})
}

func requireMCPServer(ctx context.Context, conn *sql.Conn, name string) (*domain.MCPServer, error) {
	server, err := scanMCPServer(conn.QueryRowContext(ctx,
		"SELECT "+mcpServerColumns+" FROM mcp_servers WHERE name = ?", name))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.NotFound("mcp server " + name + " not found")
		}
		return nil, err
	}
	return server, nil
}
