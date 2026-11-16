package health

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHTTPPingHealthyOn2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	result := HTTPPing(server.URL, nil, map[string]any{}, time.Second, "")
	if !result.OK || result.Status == nil || *result.Status != 200 {
		t.Fatalf("result = %+v", result)
	}
}

func TestHTTPPingUnhealthyOn4xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("unauthorized token"))
	}))
	defer server.Close()

	result := HTTPPing(server.URL, nil, map[string]any{}, time.Second, "")
	if result.OK || result.Status == nil || *result.Status != 401 {
		t.Fatalf("result = %+v", result)
	}
	if !strings.Contains(result.Detail, "unauthorized") {
		t.Fatalf("detail = %q", result.Detail)
	}
}

func TestHTTPPingUnhealthyOnConnectionError(t *testing.T) {
	result := HTTPPing("http://127.0.0.1:1", nil, map[string]any{}, 500*time.Millisecond, "")
	if result.OK || result.Status != nil {
		t.Fatalf("result = %+v", result)
	}
	if result.Detail == "" {
		t.Fatal("expected a detail")
	}
}

func TestHTTPPingSendsHeadersAndBody(t *testing.T) {
	var capturedHeaders http.Header
	var capturedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	HTTPPing(server.URL, map[string]string{"authorization": "Bearer tok"},
		map[string]any{"model": "m"}, time.Second, "")

	if capturedHeaders.Get("authorization") != "Bearer tok" {
		t.Fatalf("headers = %v", capturedHeaders)
	}
	if capturedBody["model"] != "m" {
		t.Fatalf("body = %v", capturedBody)
	}
}

func TestProxyFromEnv(t *testing.T) {
	if got := ProxyFromEnv(map[string]string{"https_proxy": "http://p:7897"}); got != "http://p:7897" {
		t.Fatalf("https proxy = %q", got)
	}
	if got := ProxyFromEnv(map[string]string{"all_proxy": "http://p:1"}); got != "http://p:1" {
		t.Fatalf("all proxy = %q", got)
	}
	if got := ProxyFromEnv(map[string]string{}); got != "" {
		t.Fatalf("empty proxy = %q", got)
	}
}

func TestClipCollapsesWhitespaceAndTruncates(t *testing.T) {
	if got := Clip("a\n  b"); got != "a b" {
		t.Fatalf("got %q", got)
	}
	long := strings.Repeat("x", 250)
	if got := Clip(long); len(got) != detailLimit+3 {
		t.Fatalf("truncated length = %d", len(got))
	}
}
