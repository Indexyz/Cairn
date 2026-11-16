// Package prompting loads embedded prompt templates and renders placeholders.
package prompting

import (
	"bytes"
	"embed"
	"encoding/json"
	"io/fs"
	"strings"
)

//go:embed prompts
var promptsFS embed.FS

// Load reads a prompt group resource from the embedded prompt tree.
func Load(group, name string) (string, error) {
	data, err := fs.ReadFile(promptsFS, "prompts/"+group+"/"+name)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// GroupExists reports whether a prompt group is embedded.
func GroupExists(group string) bool {
	info, err := fs.Stat(promptsFS, "prompts/"+group)
	return err == nil && info.IsDir()
}

// Render replaces `{key}` placeholders with their values.
func Render(template string, replacements map[string]string) string {
	text := template
	for key, value := range replacements {
		text = strings.ReplaceAll(text, "{"+key+"}", value)
	}
	return text
}

// FormatJSONBlock renders pretty JSON with two-space indentation and no HTML escaping.
func FormatJSONBlock(value any) string {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return ""
	}
	out := buffer.String()
	if len(out) > 0 && out[len(out)-1] == '\n' {
		out = out[:len(out)-1]
	}
	return out
}

// FormatFactIDs renders the fact id list handed to the reason prompt.
func FormatFactIDs(factIDs []string) string { return FormatJSONBlock(factIDs) }

// FormatOpenIntents renders the open intent list handed to the reason prompt.
func FormatOpenIntents(intents []map[string]any) string { return FormatJSONBlock(intents) }

// FormatHints renders the hint list handed to the bootstrap prompt.
func FormatHints(hints []map[string]any) string { return FormatJSONBlock(hints) }
