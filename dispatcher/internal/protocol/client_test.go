package protocol

import (
	"strings"
	"testing"
	"time"
)

func TestClientRequestFailureReturnsStatusZero(t *testing.T) {
	client := NewClient("http://127.0.0.1:1/", 300*time.Millisecond, "")
	defer client.Close()

	result := client.CreateIntent("proj_001", []string{"f001"}, "investigate", "reasoner")

	if result.StatusCode != 0 {
		t.Fatalf("status = %d", result.StatusCode)
	}
	if result.Text == "" {
		t.Fatal("expected error text")
	}
}

func TestClientQualifiesWorkerWithInstance(t *testing.T) {
	client := NewClient("http://server/", time.Second, "abc")

	if got := client.who("model"); got != "model@abc" {
		t.Fatalf("who = %q", got)
	}
	bare := NewClient("http://server/", time.Second, "")
	if got := bare.who("model"); got != "model" {
		t.Fatalf("bare who = %q", got)
	}
}

func TestClientTrimsBaseURL(t *testing.T) {
	client := NewClient("http://server///", time.Second, "")

	if client.baseURL != "http://server" {
		t.Fatalf("baseURL = %q", client.baseURL)
	}
	if !strings.HasSuffix(client.url("/projects"), "http://server/api/projects") {
		t.Fatalf("url = %q", client.url("/projects"))
	}
}
