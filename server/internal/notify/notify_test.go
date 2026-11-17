package notify

import (
	"strings"
	"testing"
)

func TestRenderMessageAppendsTheProjectLink(t *testing.T) {
	message := RenderMessage("project_completed", "proj_001", "scan", "done", "https://cairn.test/projects/proj_001")

	lines := strings.Split(message, "\n")
	if len(lines) < 4 || lines[0] != "[Cairn] Project completed" {
		t.Fatalf("message = %q", message)
	}
	found := false
	for _, line := range lines {
		if line == "https://cairn.test/projects/proj_001" {
			found = true
		}
	}
	if !found {
		t.Fatalf("link missing: %q", message)
	}
	if lines[len(lines)-1] != strings.TrimSpace(lines[len(lines)-1]) || !strings.HasSuffix(lines[len(lines)-1], "Z") {
		t.Fatalf("timestamp should stay last: %q", message)
	}
}

func TestWebhookPayloadCarriesTheProjectURL(t *testing.T) {
	_, body := payload(Channel{Kind: "webhook", Config: map[string]string{"url": "https://x.test"}},
		"project_created", "proj_001", "scan", "text", "https://cairn.test/projects/proj_001")

	if body["project_url"] != "https://cairn.test/projects/proj_001" {
		t.Fatalf("project_url = %v", body["project_url"])
	}
}
