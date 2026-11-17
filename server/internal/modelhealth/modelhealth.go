// Package modelhealth probes saved model endpoints from the server host.
package modelhealth

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Indexyz/Cairn/server/internal/domain"
	"github.com/Indexyz/Cairn/server/internal/mockmodel"
)

const (
	detailLimit      = 200
	anthropicVersion = "2023-06-01"
)

// MissingEnvError reports an absent required env key.
type MissingEnvError struct{ Key string }

func (err MissingEnvError) Error() string { return "missing env key " + err.Key }

// Result is a probe outcome.
type Result struct {
	OK     bool
	Status *int
	Detail string
}

// Check probes one model. client may be nil for the default transport.
func Check(client *http.Client, model *domain.WorkerModel, timeout time.Duration) (Result, error) {
	env := model.Env
	switch model.Type {
	case domain.TypeClaudeCode:
		base, err := required(env, "ANTHROPIC_BASE_URL")
		if err != nil {
			return Result{}, err
		}
		token, err := required(env, "ANTHROPIC_AUTH_TOKEN")
		if err != nil {
			return Result{}, err
		}
		name, err := required(env, "ANTHROPIC_MODEL")
		if err != nil {
			return Result{}, err
		}
		return ping(client, base+"/v1/messages", map[string]string{
			"authorization": "Bearer " + token, "anthropic-version": anthropicVersion, "content-type": "application/json",
		}, map[string]any{
			"model": name, "max_tokens": 10,
			"messages": []any{map[string]any{"role": "user", "content": "ping"}},
		}, timeout, proxyFromEnv(env))
	case domain.TypeCodex:
		base, err := required(env, "CODEX_BASE_URL")
		if err != nil {
			return Result{}, err
		}
		key, err := required(env, "OPENAI_API_KEY")
		if err != nil {
			return Result{}, err
		}
		name, err := required(env, "CODEX_MODEL")
		if err != nil {
			return Result{}, err
		}
		return ping(client, base+"/responses", map[string]string{
			"authorization": "Bearer " + key, "content-type": "application/json",
		}, map[string]any{
			"model": name, "input": []any{map[string]any{"role": "user", "content": "ping"}}, "stream": false,
		}, timeout, proxyFromEnv(env))
	case domain.TypePi:
		base, err := required(env, "PI_BASE_URL")
		if err != nil {
			return Result{}, err
		}
		key, err := required(env, "PI_API_KEY")
		if err != nil {
			return Result{}, err
		}
		name, err := required(env, "PI_MODEL")
		if err != nil {
			return Result{}, err
		}
		apiType, err := required(env, "PI_PROVIDER_API")
		if err != nil {
			return Result{}, err
		}
		base = strings.TrimRight(base, "/")
		headers := map[string]string{"authorization": "Bearer " + key, "content-type": "application/json"}
		body := map[string]any{
			"model": name, "max_tokens": 10,
			"messages": []any{map[string]any{"role": "user", "content": "ping"}},
		}
		proxy := proxyFromEnv(env)
		if strings.Contains(apiType, "anthropic") {
			headers["anthropic-version"] = anthropicVersion
			return ping(client, base+"/v1/messages", headers, body, timeout, proxy)
		}
		if strings.Contains(apiType, "responses") {
			return ping(client, base+"/responses", headers, map[string]any{
				"model": name, "input": []any{map[string]any{"role": "user", "content": "ping"}}, "stream": false,
			}, timeout, proxy)
		}
		return ping(client, base+"/chat/completions", headers, body, timeout, proxy)
	case domain.TypeMock:
		return checkMock(model)
	default:
		return Result{}, fmt.Errorf("unsupported worker type: %s", model.Type)
	}
}

// Describe labels the probe.
func Describe(model *domain.WorkerModel) string {
	env := model.Env
	switch model.Type {
	case domain.TypeClaudeCode:
		return "POST " + env["ANTHROPIC_BASE_URL"] + "/v1/messages (model=" + env["ANTHROPIC_MODEL"] + ")"
	case domain.TypeCodex:
		return "POST " + env["CODEX_BASE_URL"] + "/responses (model=" + env["CODEX_MODEL"] + ")"
	case domain.TypePi:
		return "POST " + env["PI_BASE_URL"] + " (api=" + env["PI_PROVIDER_API"] + ", model=" + env["PI_MODEL"] + ")"
	case domain.TypeMock:
		return "mock in-process healthcheck"
	default:
		return "in-process API ping"
	}
}

func ping(client *http.Client, target string, headers map[string]string, body map[string]any, timeout time.Duration, proxy string) (Result, error) {
	encoded, err := json.Marshal(body)
	if err != nil {
		return Result{}, err
	}
	if client == nil {
		client = &http.Client{}
	}
	requestClient := *client
	requestClient.Timeout = timeout
	if proxy != "" {
		if parsed, err := url.Parse(proxy); err == nil {
			transport := http.DefaultTransport.(*http.Transport).Clone()
			transport.Proxy = http.ProxyURL(parsed)
			requestClient.Transport = transport
		}
	}
	request, err := http.NewRequest(http.MethodPost, target, bytes.NewReader(encoded))
	if err != nil {
		return Result{Detail: clip(err.Error())}, nil
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response, err := requestClient.Do(request)
	if err != nil {
		return Result{Detail: clip(err.Error())}, nil
	}
	defer response.Body.Close()
	status := response.StatusCode
	if status >= 200 && status < 300 {
		return Result{OK: true, Status: &status}, nil
	}
	raw, _ := io.ReadAll(response.Body)
	return Result{Status: &status, Detail: clip(string(raw))}, nil
}

func checkMock(model *domain.WorkerModel) (Result, error) {
	behavior, err := mockmodel.Resolve(model.Name, model.Env)
	if err != nil {
		return Result{}, err
	}
	ok := rand.Float64() < behavior["healthcheck"].Outcomes["ok"]
	status := 503
	if ok {
		status = 200
	}
	result := Result{OK: ok, Status: &status}
	if !ok {
		result.Detail = "mock healthcheck fail"
	}
	return result, nil
}

func required(env map[string]string, key string) (string, error) {
	if value := env[key]; value != "" {
		return value, nil
	}
	return "", MissingEnvError{Key: key}
}

func proxyFromEnv(env map[string]string) string {
	for _, keys := range [][]string{{"https_proxy", "HTTPS_PROXY"}, {"all_proxy", "ALL_PROXY"}, {"http_proxy", "HTTP_PROXY"}} {
		for _, key := range keys {
			if value := env[key]; value != "" {
				return value
			}
		}
	}
	return ""
}

func clip(text string) string {
	compact := strings.Join(strings.Fields(text), " ")
	if len(compact) <= detailLimit {
		return compact
	}
	return compact[:detailLimit] + "..."
}

var _ = errors.Is
