// Package mockmodel validates `MOCK_*` worker configuration.
package mockmodel

import (
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"strings"
)

var phases = []string{"healthcheck", "reason", "explore_execute", "explore_conclude", "bootstrap", "bootstrap_conclude"}

var allowedOutcomes = map[string][]string{
	"healthcheck":        {"ok", "fail"},
	"reason":             {"complete", "intent", "noop", "rejected", "invalid_json", "invalid_payload", "command_fail"},
	"explore_execute":    {"fact", "rejected", "invalid_json", "invalid_payload", "command_fail"},
	"explore_conclude":   {"fact", "rejected", "invalid_json", "invalid_payload", "command_fail"},
	"bootstrap":          {"complete", "fact", "rejected", "invalid_json", "invalid_payload", "command_fail"},
	"bootstrap_conclude": {"fact", "rejected", "invalid_json", "invalid_payload", "command_fail"},
}

type defaults struct {
	delay    []float64
	outcomes map[string]string
}

var defaultBehavior = map[string]defaults{
	"healthcheck":        {[]float64{0.05, 0.15}, map[string]string{"ok": "1.0", "fail": "0.0"}},
	"reason":             {[]float64{0.05, 0.3}, map[string]string{"complete": "0.0", "intent": "1.0", "noop": "0.0", "rejected": "0.0", "invalid_json": "0.0", "invalid_payload": "0.0", "command_fail": "0.0"}},
	"explore_execute":    {[]float64{0.05, 0.3}, map[string]string{"fact": "1.0", "rejected": "0.0", "invalid_json": "0.0", "invalid_payload": "0.0", "command_fail": "0.0"}},
	"explore_conclude":   {[]float64{0.05, 0.3}, map[string]string{"fact": "1.0", "rejected": "0.0", "invalid_json": "0.0", "invalid_payload": "0.0", "command_fail": "0.0"}},
	"bootstrap":          {[]float64{0.05, 0.3}, map[string]string{"complete": "1.0", "fact": "0.0", "rejected": "0.0", "invalid_json": "0.0", "invalid_payload": "0.0", "command_fail": "0.0"}},
	"bootstrap_conclude": {[]float64{0.05, 0.3}, map[string]string{"fact": "1.0", "rejected": "0.0", "invalid_json": "0.0", "invalid_payload": "0.0", "command_fail": "0.0"}},
}

// Rule forces an outcome when all conditions match.
type Rule struct {
	Force            string
	FactIDsGTE       *int
	FactIDsLTE       *int
	OpenIntentsEmpty *bool
}

// Phase is one resolved phase.
type Phase struct {
	MinDelay float64
	MaxDelay float64
	Outcomes map[string]float64
	Rules    []Rule
}

// Behavior maps phase names to behavior.
type Behavior map[string]Phase

type rawPhase struct {
	Delay    []float64              `json:"delay"`
	Outcomes map[string]json.Number `json:"outcomes"`
	Rules    []struct {
		Force            string `json:"force"`
		FactIDsGTE       *int   `json:"fact_ids_gte"`
		FactIDsLTE       *int   `json:"fact_ids_lte"`
		OpenIntentsEmpty *bool  `json:"open_intents_empty"`
	} `json:"rules"`
}

// Resolve validates and normalizes the MOCK_* env of one model.
func Resolve(name string, env map[string]string) (Behavior, error) {
	allowedKeys := map[string]bool{}
	for _, phase := range phases {
		allowedKeys["MOCK_"+strings.ToUpper(phase)] = true
	}
	var unknown []string
	for key := range env {
		if strings.HasPrefix(key, "MOCK_") && !allowedKeys[key] {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return nil, fmt.Errorf("worker %s has unsupported mock env keys: %s", name, strings.Join(unknown, ", "))
	}

	behavior := Behavior{}
	for _, phase := range phases {
		prefix := "MOCK_" + strings.ToUpper(phase)
		payload, err := parsePayload(name, env, prefix, defaultBehavior[phase])
		if err != nil {
			return nil, err
		}
		if len(payload.Delay) != 2 {
			return nil, fmt.Errorf("worker %s %s.delay must be a two-element number array", name, prefix)
		}
		if payload.Delay[0] < 0 || payload.Delay[1] < 0 {
			return nil, fmt.Errorf("worker %s %s.delay must be non-negative", name, prefix)
		}
		if payload.Delay[1] < payload.Delay[0] {
			return nil, fmt.Errorf("worker %s %s.delay[1] must be greater than or equal to delay[0]", name, prefix)
		}
		if payload.Outcomes == nil {
			return nil, fmt.Errorf("worker %s %s.outcomes must be an object", name, prefix)
		}
		allowed := allowedOutcomes[phase]
		allowedSet := map[string]bool{}
		for _, outcome := range allowed {
			allowedSet[outcome] = true
		}
		var unknownOutcomes []string
		for outcome := range payload.Outcomes {
			if !allowedSet[outcome] {
				unknownOutcomes = append(unknownOutcomes, outcome)
			}
		}
		if len(unknownOutcomes) > 0 {
			sort.Strings(unknownOutcomes)
			return nil, fmt.Errorf("worker %s %s.outcomes has unsupported keys: %s", name, prefix, strings.Join(unknownOutcomes, ", "))
		}

		outcomes := map[string]float64{}
		total := new(big.Rat)
		sortedAllowed := append([]string(nil), allowed...)
		sort.Strings(sortedAllowed)
		for _, outcome := range sortedAllowed {
			weight, err := parseProbability(name, prefix, payload.Outcomes, phase, outcome)
			if err != nil {
				return nil, err
			}
			asFloat, _ := weight.Float64()
			outcomes[outcome] = asFloat
			total.Add(total, weight)
		}
		if total.Cmp(big.NewRat(1, 1)) != 0 {
			return nil, fmt.Errorf("worker %s %s.outcomes probabilities must sum to 1.0, got %s", name, prefix, total.FloatString(4))
		}

		rules := make([]Rule, 0, len(payload.Rules))
		for index, rule := range payload.Rules {
			if !allowedSet[rule.Force] {
				return nil, fmt.Errorf("worker %s %s.rules[%d].force must be one of: %s", name, prefix, index, strings.Join(sortedAllowed, ", "))
			}
			if rule.FactIDsGTE != nil && *rule.FactIDsGTE < 0 {
				return nil, fmt.Errorf("worker %s %s.rules[%d].fact_ids_gte must be a non-negative integer", name, prefix, index)
			}
			if rule.FactIDsLTE != nil && *rule.FactIDsLTE < 0 {
				return nil, fmt.Errorf("worker %s %s.rules[%d].fact_ids_lte must be a non-negative integer", name, prefix, index)
			}
			rules = append(rules, Rule{
				Force: rule.Force, FactIDsGTE: rule.FactIDsGTE, FactIDsLTE: rule.FactIDsLTE, OpenIntentsEmpty: rule.OpenIntentsEmpty,
			})
		}

		behavior[phase] = Phase{MinDelay: payload.Delay[0], MaxDelay: payload.Delay[1], Outcomes: outcomes, Rules: rules}
	}
	return behavior, nil
}

func parsePayload(name string, env map[string]string, key string, fallback defaults) (rawPhase, error) {
	raw, present := env[key]
	if !present {
		outcomes := make(map[string]json.Number, len(fallback.outcomes))
		for outcome, weight := range fallback.outcomes {
			outcomes[outcome] = json.Number(weight)
		}
		return rawPhase{Delay: fallback.delay, Outcomes: outcomes}, nil
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	var payload rawPhase
	if err := decoder.Decode(&payload); err != nil {
		return rawPhase{}, fmt.Errorf("worker %s %s must be a JSON object", name, key)
	}
	return payload, nil
}

func parseProbability(name, prefix string, outcomes map[string]json.Number, phase, outcome string) (*big.Rat, error) {
	literal, present := "", false
	if number, ok := outcomes[outcome]; ok {
		literal, present = number.String(), true
	}
	if !present {
		literal = defaultBehavior[phase].outcomes[outcome]
	}
	weight, ok := new(big.Rat).SetString(literal)
	if !ok {
		return nil, fmt.Errorf("worker %s %s.outcomes.%s must be a decimal probability", name, prefix, outcome)
	}
	if weight.Sign() < 0 || weight.Cmp(big.NewRat(1, 1)) > 0 {
		return nil, fmt.Errorf("worker %s %s.outcomes.%s must be between 0 and 1", name, prefix, outcome)
	}
	return weight, nil
}
