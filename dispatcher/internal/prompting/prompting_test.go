package prompting

import (
	"strings"
	"testing"
)

func TestEmbeddedPromptGroupsAreComplete(t *testing.T) {
	required := []string{"bootstrap.md", "bootstrap_conclude.md", "reason.md", "explore.md", "explore_conclude.md"}
	for _, group := range []string{"default", "mock"} {
		if !GroupExists(group) {
			t.Fatalf("missing prompt group %s", group)
		}
		for _, name := range required {
			content, err := Load(group, name)
			if err != nil {
				t.Fatalf("load %s/%s: %v", group, name, err)
			}
			if strings.TrimSpace(content) == "" {
				t.Fatalf("empty prompt %s/%s", group, name)
			}
		}
	}
}

func TestLoadRejectsMissingGroup(t *testing.T) {
	if _, err := Load("nope", "reason.md"); err == nil {
		t.Fatal("expected missing group error")
	}
}

func TestRenderReplacesEveryOccurrence(t *testing.T) {
	rendered := Render("a {x} b {x} c", map[string]string{"x": "1"})
	if rendered != "a 1 b 1 c" {
		t.Fatalf("rendered = %q", rendered)
	}
}

func TestFormatJSONBlockIsIndentedAndUnescaped(t *testing.T) {
	got := FormatJSONBlock([]any{"a&b"})
	want := "[\n  \"a&b\"\n]"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
