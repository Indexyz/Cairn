package api

import (
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/Indexyz/Cairn/server/internal/domain"
	"github.com/Indexyz/Cairn/server/internal/mockmodel"
)

const maxBody = 8 << 20

func readBody(r *http.Request) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxBody))
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, &domain.ValidationError{Errors: []domain.FieldError{{
			Type: "value_error", Loc: []any{"body"}, Msg: "Field required",
		}}}
	}
	return raw, nil
}

func resolveMock(model *domain.WorkerModel) (mockmodel.Behavior, error) {
	return mockmodel.Resolve(model.Name, model.Env)
}

func parseTimeout(raw string) (time.Duration, bool) {
	if raw == "" {
		return 10 * time.Second, true
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || value <= 0 || value > 60 {
		return 0, false
	}
	return time.Duration(value * float64(time.Second)), true
}
