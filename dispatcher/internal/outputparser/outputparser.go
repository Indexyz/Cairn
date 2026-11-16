// Package outputparser finds the first JSON object in free-form model output.
package outputparser

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
)

var fencedBlock = regexp.MustCompile("(?is)```(?:json)?\\s*\\n?(.*?)```")

// ExtractJSONObject returns the first JSON object found, preferring the whole
// text and fenced blocks before scanning every `{` position.
func ExtractJSONObject(text string) (map[string]any, error) {
	seen := map[string]bool{}
	for _, candidate := range candidateSegments(text) {
		segment := strings.TrimSpace(candidate)
		if segment == "" || seen[segment] {
			continue
		}
		seen[segment] = true

		if object, ok := decodeObject(segment); ok {
			return object, nil
		}
		for index, char := range segment {
			if char != '{' {
				continue
			}
			if object, ok := decodePrefix(segment[index:]); ok {
				return object, nil
			}
		}
	}
	return nil, errors.New("no JSON object found in output")
}

func candidateSegments(text string) []string {
	segments := []string{strings.TrimSpace(text)}
	for _, match := range fencedBlock.FindAllStringSubmatch(text, -1) {
		segments = append(segments, strings.TrimSpace(match[1]))
	}
	return segments
}

func decodeObject(text string) (map[string]any, bool) {
	var value any
	if err := json.Unmarshal([]byte(text), &value); err != nil {
		return nil, false
	}
	object, ok := value.(map[string]any)
	return object, ok
}

func decodePrefix(text string) (map[string]any, bool) {
	decoder := json.NewDecoder(strings.NewReader(text))
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, false
	}
	object, ok := value.(map[string]any)
	return object, ok
}
