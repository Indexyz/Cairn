// Package health checks LLM endpoints and probes host CLIs.
package health

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const detailLimit = 200

// Result is a health probe outcome.
type Result struct {
	OK     bool
	Status *int
	Detail string
}

// HTTPPing posts a tiny request and judges the endpoint by its status code.
func HTTPPing(target string, headers map[string]string, body map[string]any, timeout time.Duration, proxy string) Result {
	encoded, err := json.Marshal(body)
	if err != nil {
		return Result{Detail: err.Error()}
	}
	transport := &http.Transport{}
	if proxy != "" {
		if parsed, err := url.Parse(proxy); err == nil {
			transport.Proxy = http.ProxyURL(parsed)
		}
	}
	client := &http.Client{Timeout: timeout, Transport: transport}
	request, err := http.NewRequest(http.MethodPost, target, bytes.NewReader(encoded))
	if err != nil {
		return Result{Detail: Clip(err.Error())}
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response, err := client.Do(request)
	if err != nil {
		return Result{Detail: Clip(err.Error())}
	}
	defer response.Body.Close()
	status := response.StatusCode
	if status >= 200 && status < 300 {
		return Result{OK: true, Status: &status}
	}
	raw, _ := io.ReadAll(response.Body)
	return Result{Status: &status, Detail: Clip(string(raw))}
}

// ProxyFromEnv picks the outbound proxy the worker itself would use.
func ProxyFromEnv(env map[string]string) string {
	all := first(env, "all_proxy", "ALL_PROXY")
	https := first(env, "https_proxy", "HTTPS_PROXY")
	if https != "" {
		return https
	}
	if all != "" {
		return all
	}
	return first(env, "http_proxy", "HTTP_PROXY")
}

// Clip collapses whitespace and truncates to the detail limit.
func Clip(text string) string {
	compact := strings.Join(strings.Fields(text), " ")
	if len(compact) <= detailLimit {
		return compact
	}
	return compact[:detailLimit] + "..."
}

func first(env map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := env[key]; value != "" {
			return value
		}
	}
	return ""
}
