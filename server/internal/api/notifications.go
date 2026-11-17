package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/Indexyz/Cairn/server/internal/domain"
	"github.com/Indexyz/Cairn/server/internal/notify"
)

func scanChannel(scanner interface{ Scan(...any) error }) (*domain.NotificationChannel, error) {
	channel := &domain.NotificationChannel{}
	var enabled int
	var events, config string
	if err := scanner.Scan(&channel.ID, &channel.Name, &channel.Kind, &enabled, &events, &config, &channel.CreatedAt, &channel.UpdatedAt); err != nil {
		return nil, err
	}
	channel.Enabled = enabled != 0
	if err := json.Unmarshal([]byte(events), &channel.Events); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(config), &channel.Config); err != nil {
		return nil, err
	}
	return channel, nil
}

func requireChannel(ctx context.Context, conn *sql.Conn, channelID string) (*domain.NotificationChannel, error) {
	channel, err := scanChannel(conn.QueryRowContext(ctx,
		"SELECT id, name, kind, enabled, events, config, created_at, updated_at FROM notification_channels WHERE id = ?", channelID))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.NotFound("channel " + channelID + " not found")
		}
		return nil, err
	}
	return channel, nil
}

func (app *App) listChannels(w http.ResponseWriter, r *http.Request) error {
	return app.tx(w, r, func(ctx context.Context, conn *sql.Conn) (int, any, error) {
		rows, err := conn.QueryContext(ctx,
			"SELECT id, name, kind, enabled, events, config, created_at, updated_at FROM notification_channels ORDER BY created_at, id")
		if err != nil {
			return 0, nil, err
		}
		defer rows.Close()
		channels := []domain.NotificationChannel{}
		for rows.Next() {
			channel, err := scanChannel(rows)
			if err != nil {
				return 0, nil, err
			}
			channels = append(channels, *channel)
		}
		return http.StatusOK, channels, nil
	})
}

func (app *App) createChannel(w http.ResponseWriter, r *http.Request) error {
	raw, err := readBody(r)
	if err != nil {
		return err
	}
	payload, err := domain.ParseNotificationChannel(raw)
	if err != nil {
		return err
	}
	return app.tx(w, r, func(ctx context.Context, conn *sql.Conn) (int, any, error) {
		now := domain.UtcNow()
		channelID, err := domain.NextChannelID(ctx, conn)
		if err != nil {
			return 0, nil, err
		}
		events, _ := json.Marshal(payload.Events)
		config, _ := json.Marshal(payload.Config)
		if _, err := conn.ExecContext(ctx,
			"INSERT INTO notification_channels (id, name, kind, enabled, events, config, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
			channelID, payload.Name, payload.Kind, boolInt(payload.EnabledValue()), string(events), string(config), now, now); err != nil {
			return 0, nil, err
		}
		channel, err := requireChannel(ctx, conn, channelID)
		if err != nil {
			return 0, nil, err
		}
		return http.StatusCreated, channel, nil
	})
}

func (app *App) updateChannel(w http.ResponseWriter, r *http.Request) error {
	channelID := r.PathValue("channel")
	raw, err := readBody(r)
	if err != nil {
		return err
	}
	payload, err := domain.ParseNotificationChannel(raw)
	if err != nil {
		return err
	}
	return app.tx(w, r, func(ctx context.Context, conn *sql.Conn) (int, any, error) {
		if _, err := requireChannel(ctx, conn, channelID); err != nil {
			return 0, nil, err
		}
		events, _ := json.Marshal(payload.Events)
		config, _ := json.Marshal(payload.Config)
		if _, err := conn.ExecContext(ctx,
			"UPDATE notification_channels SET name = ?, kind = ?, enabled = ?, events = ?, config = ?, updated_at = ? WHERE id = ?",
			payload.Name, payload.Kind, boolInt(payload.EnabledValue()), string(events), string(config), domain.UtcNow(), channelID); err != nil {
			return 0, nil, err
		}
		channel, err := requireChannel(ctx, conn, channelID)
		if err != nil {
			return 0, nil, err
		}
		return http.StatusOK, channel, nil
	})
}

func (app *App) deleteChannel(w http.ResponseWriter, r *http.Request) error {
	channelID := r.PathValue("channel")
	return app.tx(w, r, func(ctx context.Context, conn *sql.Conn) (int, any, error) {
		if _, err := requireChannel(ctx, conn, channelID); err != nil {
			return 0, nil, err
		}
		if _, err := conn.ExecContext(ctx, "DELETE FROM notification_channels WHERE id = ?", channelID); err != nil {
			return 0, nil, err
		}
		return http.StatusNoContent, nil, nil
	})
}

func (app *App) testChannel(w http.ResponseWriter, r *http.Request) error {
	channelID := r.PathValue("channel")
	var channel *domain.NotificationChannel
	if err := app.DB.Read(r.Context(), func(conn *sql.Conn) error {
		loaded, err := requireChannel(r.Context(), conn, channelID)
		channel = loaded
		return err
	}); err != nil {
		return err
	}
	target := notify.Channel{
		ID: channel.ID, Name: channel.Name, Kind: channel.Kind, Events: channel.Events, Config: channel.Config,
	}
	text := notify.RenderTest(channel.Name)
	result := app.Deliverer.Send(target, "test", "proj_000", channel.Name, text)
	writeJSON(w, http.StatusOK, domain.NotificationTestResult{
		OK: result.OK, Status: result.Status, Detail: result.Detail, DurationMs: result.DurationMs,
	})
	return nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
