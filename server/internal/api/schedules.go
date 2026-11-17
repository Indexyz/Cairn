package api

import (
	"net/http"
	"time"

	"github.com/Indexyz/Cairn/server/internal/domain"
	"github.com/Indexyz/Cairn/server/internal/scheduler"
)

func (app *App) listSchedules(w http.ResponseWriter, r *http.Request) error {
	schedules, err := scheduler.List(r.Context(), app.DB)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, schedules)
	return nil
}

func (app *App) createSchedule(w http.ResponseWriter, r *http.Request) error {
	raw, err := readBody(r)
	if err != nil {
		return err
	}
	payload, err := domain.ParseSchedule(raw)
	if err != nil {
		return err
	}
	schedule, err := scheduler.Create(r.Context(), app.DB, payload, time.Now())
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, schedule)
	return nil
}

func (app *App) updateSchedule(w http.ResponseWriter, r *http.Request) error {
	scheduleID := r.PathValue("schedule")
	raw, err := readBody(r)
	if err != nil {
		return err
	}
	payload, err := domain.ParseSchedule(raw)
	if err != nil {
		return err
	}
	schedule, err := scheduler.Update(r.Context(), app.DB, scheduleID, payload, time.Now())
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, schedule)
	return nil
}

func (app *App) deleteSchedule(w http.ResponseWriter, r *http.Request) error {
	if err := scheduler.Delete(r.Context(), app.DB, r.PathValue("schedule")); err != nil {
		return err
	}
	writeNoContent(w)
	return nil
}

func (app *App) runSchedule(w http.ResponseWriter, r *http.Request) error {
	result, err := scheduler.Execute(r.Context(), app.DB, r.PathValue("schedule"), time.Now(), app.NotifyProjectCreated, true)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, result)
	return nil
}
