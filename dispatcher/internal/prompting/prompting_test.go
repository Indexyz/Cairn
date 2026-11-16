package prompting

import (
	"os"
	"path/filepath"
	"testing"
)

// The prompt tree is embedded for the Go binary but still owned by the Python
// package until the server port lands. Fail when the copies drift.
func TestEmbeddedPromptsMatchPythonPackage(t *testing.T) {
	pythonRoot := filepath.Join("..", "..", "..", "cairn", "src", "cairn", "dispatcher", "prompts")
	if _, err := os.Stat(pythonRoot); err != nil {
		t.Skipf("python prompt tree not available: %v", err)
	}

	for _, group := range []string{"default", "mock"} {
		entries, err := os.ReadDir(filepath.Join(pythonRoot, group))
		if err != nil {
			t.Fatalf("read %s: %v", group, err)
		}
		for _, entry := range entries {
			want, err := os.ReadFile(filepath.Join(pythonRoot, group, entry.Name()))
			if err != nil {
				t.Fatalf("read %s/%s: %v", group, entry.Name(), err)
			}
			got, err := Load(group, entry.Name())
			if err != nil {
				t.Fatalf("embedded %s/%s: %v", group, entry.Name(), err)
			}
			if string(want) != got {
				t.Fatalf("prompt drift in %s/%s", group, entry.Name())
			}
		}
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
