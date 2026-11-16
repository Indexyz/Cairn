package contracts

import (
	"strings"
	"testing"
)

func TestParseJSONOutputExtractsObjectFromMarkdownNoise(t *testing.T) {
	payload, err := ParseJSONOutput("result:\n```json\n{\"accepted\": true, \"data\": {}}\n```")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	accepted, ok := payload["accepted"].(bool)
	if !ok || !accepted {
		t.Fatalf("accepted = %v", payload["accepted"])
	}
	if _, ok := payload["data"].(map[string]any); !ok {
		t.Fatalf("data = %v", payload["data"])
	}
}

func TestReasonPayloadLimitsNumberOfIntents(t *testing.T) {
	payload := map[string]any{
		"accepted": true,
		"data": map[string]any{
			"intents": []any{
				map[string]any{"from": []any{"f001"}, "description": "one"},
				map[string]any{"from": []any{"f001"}, "description": "two"},
			},
		},
	}

	kind, data, err := ValidateReasonPayload(payload, true, 1)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if kind != "intents" {
		t.Fatalf("kind = %q", kind)
	}
	list := data.([]any)
	if len(list) != 1 || list[0].(map[string]any)["description"] != "one" {
		t.Fatalf("intents = %v", list)
	}
}

func TestReasonPayloadRequiresIntentWhenNoneAreOpen(t *testing.T) {
	_, _, err := ValidateReasonPayload(map[string]any{"accepted": true, "data": map[string]any{}}, true, 3)
	if err == nil || !strings.Contains(err.Error(), "intents is required") {
		t.Fatalf("error = %v", err)
	}
}

func TestReasonPayloadAcceptsSingularIntentKey(t *testing.T) {
	payload := map[string]any{
		"accepted": true,
		"data": map[string]any{
			"intent": map[string]any{"from": []any{"f001"}, "description": "one"},
		},
	}
	kind, _, err := ValidateReasonPayload(payload, true, 3)
	if err != nil || kind != "intents" {
		t.Fatalf("kind=%q err=%v", kind, err)
	}
}

func TestReasonPayloadRejectsCompleteWithIntents(t *testing.T) {
	payload := map[string]any{
		"accepted": true,
		"data": map[string]any{
			"complete": map[string]any{"from": []any{"f001"}, "description": "done"},
			"intents":  []any{map[string]any{"from": []any{"f001"}, "description": "one"}},
		},
	}
	if _, _, err := ValidateReasonPayload(payload, false, 3); err == nil {
		t.Fatal("expected coexistence error")
	}
}

func TestExplorePayloadRejectsPlanningText(t *testing.T) {
	if _, err := ParseJSONOutput("Need inspect files and keep working."); err == nil {
		t.Fatal("expected no-JSON error")
	}
}

func TestBootstrapExecuteRequiresFactAndComplete(t *testing.T) {
	payload := map[string]any{
		"accepted": true,
		"data": map[string]any{
			"fact":     map[string]any{"description": " found flag "},
			"complete": map[string]any{"description": " solved "},
		},
	}
	kind, data, err := ValidateBootstrapExecutePayload(payload)
	if err != nil || kind != "complete" {
		t.Fatalf("kind=%q err=%v", kind, err)
	}
	if data["fact_description"] != "found flag" || data["complete_description"] != "solved" {
		t.Fatalf("data = %v", data)
	}

	missing := map[string]any{"accepted": true, "data": map[string]any{"fact": map[string]any{"description": "x"}}}
	if _, _, err := ValidateBootstrapExecutePayload(missing); err == nil {
		t.Fatal("expected complete requirement error")
	}
}

func TestBootstrapConcludeRejectsUnexpectedKeys(t *testing.T) {
	payload := map[string]any{"accepted": true, "data": map[string]any{
		"fact":  map[string]any{"description": "x"},
		"extra": true,
	}}
	if _, _, err := ValidateBootstrapConcludePayload(payload); err == nil {
		t.Fatal("expected unexpected key error")
	}
}

func TestRejectedPayloadsShortCircuit(t *testing.T) {
	payload := map[string]any{"accepted": false}
	if kind, _, err := ValidateExplorePayload(payload); err != nil || kind != "rejected" {
		t.Fatalf("kind=%q err=%v", kind, err)
	}
	if kind, _, err := ValidateBootstrapConcludePayload(payload); err != nil || kind != "rejected" {
		t.Fatalf("kind=%q err=%v", kind, err)
	}
}
