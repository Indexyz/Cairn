package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

const discordContract = "https://discord.com/api/webhooks/1/abc"

func discordChannel() map[string]any {
	return map[string]any{
		"name": "team discord", "kind": "discord", "enabled": true,
		"events": []string{"project_completed"}, "config": map[string]string{"webhook_url": discordContract},
	}
}

func TestChannelCRUDRoundTrip(t *testing.T) {
	harness := newTestApp(t)
	status, channel := harness.json(t, http.MethodPost, "/notifications", discordChannel())
	if status != http.StatusCreated || channel["id"] != "chan_001" || channel["enabled"] != true {
		t.Fatalf("create status=%d payload=%v", status, channel)
	}
	_, raw := harness.request(t, http.MethodGet, "/notifications", nil)
	var channels []map[string]any
	_ = json.Unmarshal(raw, &channels)
	if len(channels) != 1 || channels[0]["id"] != channel["id"] {
		t.Fatalf("channels = %s", raw)
	}

	updated := discordChannel()
	updated["name"] = "renamed"
	updated["enabled"] = false
	status, renamed := harness.json(t, http.MethodPut, "/notifications/chan_001", updated)
	if status != http.StatusOK || renamed["name"] != "renamed" || renamed["enabled"] != false {
		t.Fatalf("update status=%d payload=%v", status, renamed)
	}

	if status, _ := harness.request(t, http.MethodDelete, "/notifications/chan_001", nil); status != http.StatusNoContent {
		t.Fatalf("delete status = %d", status)
	}
	if status, _ := harness.request(t, http.MethodPut, "/notifications/chan_999", discordChannel()); status != http.StatusNotFound {
		t.Fatalf("missing update status = %d", status)
	}
	if status, _ := harness.request(t, http.MethodDelete, "/notifications/chan_999", nil); status != http.StatusNotFound {
		t.Fatalf("missing delete status = %d", status)
	}
}

func TestChannelValidation(t *testing.T) {
	harness := newTestApp(t)
	cases := []struct {
		override map[string]any
		field    string
	}{
		{map[string]any{"config": map[string]string{}}, "webhook_url"},
		{map[string]any{"config": map[string]string{"webhook_url": "ftp://example.com"}}, "webhook_url"},
		{map[string]any{"events": []string{}}, "events"},
		{map[string]any{"name": "  "}, "name"},
		{map[string]any{"kind": "irc"}, "kind"},
	}
	for _, item := range cases {
		body := discordChannel()
		for key, value := range item.override {
			body[key] = value
		}
		status, raw := harness.request(t, http.MethodPost, "/notifications", body)
		if status != http.StatusUnprocessableEntity {
			t.Fatalf("override %v status = %d", item.override, status)
		}
		if !containsString(string(raw), item.field) {
			t.Fatalf("override %v body = %s", item.override, raw)
		}
	}
}

func TestChannelDropsStrayConfig(t *testing.T) {
	harness := newTestApp(t)
	body := discordChannel()
	body["config"] = map[string]string{"webhook_url": "https://x.test/hook", "extra": "ignored"}
	status, channel := harness.json(t, http.MethodPost, "/notifications", body)
	if status != http.StatusCreated {
		t.Fatalf("status = %d", status)
	}
	config := channel["config"].(map[string]any)
	if len(config) != 1 || config["webhook_url"] != "https://x.test/hook" {
		t.Fatalf("config = %v", config)
	}
}

func TestEventsReachSubscribedEnabledChannelsOnly(t *testing.T) {
	harness := newTestApp(t)
	harness.json(t, http.MethodPost, "/notifications", discordChannel())
	harness.json(t, http.MethodPost, "/notifications", map[string]any{
		"name": "disabled", "kind": "telegram", "enabled": false,
		"events": []string{"project_completed"}, "config": map[string]string{"bot_token": "123:abc", "chat_id": "-1"},
	})
	harness.json(t, http.MethodPost, "/notifications", map[string]any{
		"name": "created only", "kind": "webhook", "enabled": true,
		"events": []string{"project_created"}, "config": map[string]string{"url": "https://x.test/hook"},
	})

	projectID := harness.createProject(t)
	harness.json(t, http.MethodPost, "/projects/"+projectID+"/complete",
		map[string]any{"from": []string{"origin"}, "description": "goal reached", "worker": "reasoner"})
	other := budgetProject(t, harness, nil, "stopped thing")
	harness.json(t, http.MethodPut, "/projects/"+other+"/status", map[string]any{"status": "stopped"})
	harness.waitDeliveries()

	calls := harness.recorder.snapshot()
	if len(calls) != 3 {
		t.Fatalf("calls = %v", calls)
	}
	webhookCalls, discordCalls := 0, 0
	for _, call := range calls {
		if call.URL == "https://x.test/hook" {
			webhookCalls++
			if call.Body["event"] != "project_created" {
				t.Fatalf("webhook event = %v", call.Body["event"])
			}
			continue
		}
		discordCalls++
		if content, _ := call.Body["content"].(string); !containsString(content, "Project completed") {
			t.Fatalf("discord content = %q", content)
		}
	}
	if webhookCalls != 2 || discordCalls != 1 {
		t.Fatalf("webhook=%d discord=%d", webhookCalls, discordCalls)
	}
}

func TestStopAndResumeEmitTheirOwnEvents(t *testing.T) {
	harness := newTestApp(t)
	body := discordChannel()
	body["events"] = []string{"project_stopped", "project_resumed"}
	harness.json(t, http.MethodPost, "/notifications", body)
	projectID := harness.createProject(t)

	harness.json(t, http.MethodPut, "/projects/"+projectID+"/status", map[string]any{"status": "stopped"})
	harness.json(t, http.MethodPut, "/projects/"+projectID+"/status", map[string]any{"status": "active"})
	harness.waitDeliveries()

	calls := harness.recorder.snapshot()
	if len(calls) != 2 {
		t.Fatalf("calls = %v", calls)
	}
	joined := ""
	for _, call := range calls {
		content, _ := call.Body["content"].(string)
		joined += content
	}
	if !containsString(joined, "Project paused") || !containsString(joined, "Project resumed") {
		t.Fatalf("contents = %q", joined)
	}
}

func TestReopenEmitsWithTheCorrection(t *testing.T) {
	harness := newTestApp(t)
	body := discordChannel()
	body["events"] = []string{"project_reopened"}
	harness.json(t, http.MethodPost, "/notifications", body)
	projectID := harness.createProject(t)
	harness.json(t, http.MethodPost, "/projects/"+projectID+"/complete",
		map[string]any{"from": []string{"origin"}, "description": "goal reached", "worker": "reasoner"})
	status, _ := harness.json(t, http.MethodPost, "/projects/"+projectID+"/reopen",
		map[string]any{"description": "flag was wrong", "creator": "human"})
	if status != http.StatusOK {
		t.Fatalf("reopen status = %d", status)
	}
	harness.waitDeliveries()
	calls := harness.recorder.snapshot()
	if len(calls) != 1 {
		t.Fatalf("calls = %v", calls)
	}
	content, _ := calls[0].Body["content"].(string)
	if !containsString(content, "Project reopened") || !containsString(content, "flag was wrong") {
		t.Fatalf("content = %q", content)
	}
}

func TestTestEndpointSendsEvenForDisabledChannel(t *testing.T) {
	harness := newTestApp(t)
	body := discordChannel()
	body["enabled"] = false
	harness.json(t, http.MethodPost, "/notifications", body)

	status, payload := harness.json(t, http.MethodPost, "/notifications/chan_001/test", nil)
	if status != http.StatusOK || payload["ok"] != true {
		t.Fatalf("status=%d payload=%v", status, payload)
	}
	calls := harness.recorder.snapshot()
	if len(calls) != 1 || calls[0].URL != discordContract {
		t.Fatalf("calls = %v", calls)
	}
	if content, _ := calls[0].Body["content"].(string); !containsString(content, "Test notification") {
		t.Fatalf("content = %v", calls[0].Body["content"])
	}
}

func TestPayloadsAreShapedPerKind(t *testing.T) {
	harness := newTestApp(t)
	harness.json(t, http.MethodPost, "/notifications", map[string]any{
		"name": "tg", "kind": "telegram", "enabled": true, "events": []string{"project_created"},
		"config": map[string]string{"bot_token": "1:a", "chat_id": "-1"},
	})
	harness.json(t, http.MethodPost, "/notifications", map[string]any{
		"name": "hook", "kind": "webhook", "enabled": true, "events": []string{"project_created"},
		"config": map[string]string{"url": "https://w.test/hook"},
	})
	harness.createProject(t)
	harness.waitDeliveries()

	calls := harness.recorder.snapshot()
	if len(calls) != 2 {
		t.Fatalf("calls = %v", calls)
	}
	var telegram, webhook *recordedCall
	for index := range calls {
		if calls[index].URL == "https://api.telegram.org/bot1:a/sendMessage" {
			telegram = &calls[index]
		}
		if calls[index].URL == "https://w.test/hook" {
			webhook = &calls[index]
		}
	}
	if telegram == nil || telegram.Body["chat_id"] != "-1" || telegram.Body["disable_web_page_preview"] != true {
		t.Fatalf("telegram = %v", telegram)
	}
	if webhook == nil || webhook.Body["project_id"] == nil || webhook.Body["text"] == nil {
		t.Fatalf("webhook = %v", webhook)
	}
}

func TestSendSurvivesTransportError(t *testing.T) {
	harness := newTestApp(t)
	harness.app.Deliverer.Client = &http.Client{Transport: failingTransport{}}
	body := discordChannel()
	body["enabled"] = false
	harness.json(t, http.MethodPost, "/notifications", body)

	status, payload := harness.json(t, http.MethodPost, "/notifications/chan_001/test", nil)
	if status != http.StatusOK || payload["ok"] != false {
		t.Fatalf("status=%d payload=%v", status, payload)
	}
	if detail, _ := payload["detail"].(string); !containsString(detail, "connection refused") {
		t.Fatalf("detail = %q", detail)
	}
}

type failingTransport struct{}

func (failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, &transportError{}
}

type transportError struct{}

func (*transportError) Error() string { return "connection refused" }

func TestChannelDefaultsToEnabled(t *testing.T) {
	harness := newTestApp(t)
	body := discordChannel()
	delete(body, "enabled")
	status, channel := harness.json(t, http.MethodPost, "/notifications", body)
	if status != http.StatusCreated || channel["enabled"] != true {
		t.Fatalf("status=%d payload=%v", status, channel)
	}
}

func TestNotificationsCarryTheProjectLink(t *testing.T) {
	harness := newTestApp(t)
	harness.app.PublicURL = "https://cairn.example.com/"
	body := discordChannel()
	body["events"] = []string{"project_completed"}
	harness.json(t, http.MethodPost, "/notifications", body)
	harness.json(t, http.MethodPost, "/notifications", map[string]any{
		"name": "hook", "kind": "webhook", "enabled": true,
		"events": []string{"project_completed"}, "config": map[string]string{"url": "https://x.test/hook"},
	})

	projectID := harness.createProject(t)
	harness.json(t, http.MethodPost, "/projects/"+projectID+"/complete",
		map[string]any{"from": []string{"origin"}, "description": "done", "worker": "reasoner"})
	harness.waitDeliveries()

	want := "https://cairn.example.com/projects/" + projectID
	for _, call := range harness.recorder.snapshot() {
		switch call.URL {
		case "https://x.test/hook":
			if call.Body["project_url"] != want {
				t.Fatalf("project_url = %v, want %s", call.Body["project_url"], want)
			}
		default:
			if content, _ := call.Body["content"].(string); !containsString(content, want) {
				t.Fatalf("discord content = %q", content)
			}
		}
	}
}

func TestProjectLinkFallsBackToARelativePath(t *testing.T) {
	harness := newTestApp(t)
	if got := harness.app.projectLink("proj_001"); got != "/projects/proj_001" {
		t.Fatalf("link = %q", got)
	}
}
