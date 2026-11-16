// Package contracts validates the JSON payloads workers return.
package contracts

import (
	"fmt"

	"github.com/Indexyz/Cairn/dispatcher/internal/outputparser"
)

// ParseJSONOutput extracts the JSON object from model stdout.
func ParseJSONOutput(stdout string) (map[string]any, error) {
	return outputparser.ExtractJSONObject(stdout)
}

// unwrap accepts both `{"accepted":..,"data":..}` and a bare payload object.
// It returns (rejected, data, handled).
func unwrap(payload map[string]any) (bool, map[string]any, bool, error) {
	accepted, present := payload["accepted"]
	if present {
		if flag, ok := accepted.(bool); ok {
			if !flag {
				return true, nil, true, nil
			}
			data, ok := payload["data"].(map[string]any)
			if !ok {
				return false, nil, true, fmt.Errorf("data must be an object")
			}
			return false, data, true, nil
		}
	}
	return false, nil, false, nil
}

func hasExactKeys(payload map[string]any, keys ...string) bool {
	if len(payload) != len(keys) {
		return false
	}
	for _, key := range keys {
		if _, ok := payload[key]; !ok {
			return false
		}
	}
	return true
}

func looksLikeReasonData(payload map[string]any) bool {
	if hasExactKeys(payload, "complete") {
		complete, ok := payload["complete"].(map[string]any)
		if !ok {
			return false
		}
		return complete["from"] != nil && complete["description"] != nil
	}
	if hasExactKeys(payload, "intents") {
		_, ok := payload["intents"].([]any)
		return ok
	}
	if hasExactKeys(payload, "intent") {
		intent, ok := payload["intent"].(map[string]any)
		if !ok {
			return false
		}
		return intent["from"] != nil && intent["description"] != nil
	}
	return false
}

func looksLikeBootstrapExecuteData(payload map[string]any) bool {
	if !hasExactKeys(payload, "fact", "complete") {
		return false
	}
	_, factOK := payload["fact"].(map[string]any)
	_, completeOK := payload["complete"].(map[string]any)
	return factOK && completeOK
}

func looksLikeBootstrapConcludeData(payload map[string]any) bool {
	if !hasExactKeys(payload, "fact") && !hasExactKeys(payload, "fact", "complete") {
		return false
	}
	_, ok := payload["fact"].(map[string]any)
	return ok
}

func looksLikeExploreData(payload map[string]any) bool {
	return hasExactKeys(payload, "description")
}

func unwrapOrData(payload map[string]any, looksLike func(map[string]any) bool) (bool, map[string]any, error) {
	rejected, data, handled, err := unwrap(payload)
	if err != nil {
		return false, nil, err
	}
	if handled {
		return rejected, data, nil
	}
	if !looksLike(payload) {
		return false, nil, fmt.Errorf("accepted must be true or false")
	}
	return false, payload, nil
}

// ValidateReasonPayload returns "rejected", "complete", "intents" or "noop".
func ValidateReasonPayload(payload map[string]any, openIntentsEmpty bool, maxIntents int) (string, any, error) {
	rejected, data, err := unwrapOrData(payload, looksLikeReasonData)
	if err != nil {
		return "", nil, err
	}
	if rejected {
		return "rejected", nil, nil
	}

	intents, hasIntents := data["intents"]
	if !hasIntents {
		// backward compat: accept the singular "intent" key from LLMs
		if singular, ok := data["intent"].(map[string]any); ok {
			intents = []any{singular}
			hasIntents = true
		}
	}
	complete, hasComplete := data["complete"]

	if hasComplete {
		if hasIntents {
			return "", nil, fmt.Errorf("complete and intents cannot coexist")
		}
		completeObject, ok := complete.(map[string]any)
		if !ok || completeObject["from"] == nil || completeObject["description"] == nil {
			return "", nil, fmt.Errorf("invalid complete payload")
		}
		return "complete", completeObject, nil
	}

	if hasIntents {
		list, ok := intents.([]any)
		if !ok {
			return "", nil, fmt.Errorf("intents must be an array")
		}
		for index, intent := range list {
			object, ok := intent.(map[string]any)
			if !ok || object["from"] == nil || object["description"] == nil {
				return "", nil, fmt.Errorf("invalid intent at index %d", index)
			}
		}
		if len(list) == 0 && openIntentsEmpty {
			return "", nil, fmt.Errorf("intents must not be empty when open_intents is empty")
		}
		if len(list) > maxIntents {
			list = list[:maxIntents]
		}
		if len(list) == 0 {
			return "noop", nil, nil
		}
		return "intents", list, nil
	}

	if openIntentsEmpty {
		return "", nil, fmt.Errorf("intents is required when open_intents is empty")
	}
	return "noop", nil, nil
}

// ValidateBootstrapExecutePayload returns "rejected" or "complete" with fact/complete text.
func ValidateBootstrapExecutePayload(payload map[string]any) (string, map[string]string, error) {
	rejected, data, err := unwrapOrData(payload, looksLikeBootstrapExecuteData)
	if err != nil {
		return "", nil, err
	}
	if rejected {
		return "rejected", nil, nil
	}

	fact, ok := data["fact"].(map[string]any)
	if !ok {
		return "", nil, fmt.Errorf("fact is required")
	}
	factDescription, ok := fact["description"].(string)
	if !ok || trimSpace(factDescription) == "" {
		return "", nil, fmt.Errorf("fact.description is required")
	}

	complete, present := data["complete"]
	if !present || complete == nil {
		return "", nil, fmt.Errorf("complete is required")
	}
	completeObject, ok := complete.(map[string]any)
	if !ok {
		return "", nil, fmt.Errorf("complete must be an object")
	}
	completeDescription, ok := completeObject["description"].(string)
	if !ok || trimSpace(completeDescription) == "" {
		return "", nil, fmt.Errorf("complete.description is required")
	}

	return "complete", map[string]string{
		"fact_description":     trimSpace(factDescription),
		"complete_description": trimSpace(completeDescription),
	}, nil
}

// ValidateBootstrapConcludePayload returns "rejected" or "fact" with the fact text.
func ValidateBootstrapConcludePayload(payload map[string]any) (string, *string, error) {
	rejected, data, err := unwrapOrData(payload, looksLikeBootstrapConcludeData)
	if err != nil {
		return "", nil, err
	}
	if rejected {
		return "rejected", nil, nil
	}
	for key := range data {
		if key != "fact" && key != "complete" {
			return "", nil, fmt.Errorf("unexpected keys in conclude payload")
		}
	}
	fact, ok := data["fact"].(map[string]any)
	if !ok {
		return "", nil, fmt.Errorf("fact is required")
	}
	description, ok := fact["description"].(string)
	if !ok || trimSpace(description) == "" {
		return "", nil, fmt.Errorf("fact.description is required")
	}
	trimmed := trimSpace(description)
	return "fact", &trimmed, nil
}

// ValidateExplorePayload returns "rejected" or "fact" with the fact text.
func ValidateExplorePayload(payload map[string]any) (string, *string, error) {
	rejected, data, err := unwrapOrData(payload, looksLikeExploreData)
	if err != nil {
		return "", nil, err
	}
	if rejected {
		return "rejected", nil, nil
	}
	description, ok := data["description"].(string)
	if !ok || trimSpace(description) == "" {
		return "", nil, fmt.Errorf("description is required")
	}
	trimmed := trimSpace(description)
	return "fact", &trimmed, nil
}

func trimSpace(value string) string {
	start, end := 0, len(value)
	for start < end && isSpace(value[start]) {
		start++
	}
	for end > start && isSpace(value[end-1]) {
		end--
	}
	return value[start:end]
}

func isSpace(char byte) bool {
	return char == ' ' || char == '\t' || char == '\n' || char == '\r' || char == '\v' || char == '\f'
}
