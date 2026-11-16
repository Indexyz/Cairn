package adapters

import (
	"encoding/json"
	"math/rand"
	"time"

	"github.com/Indexyz/Cairn/dispatcher/internal/config"
	"github.com/Indexyz/Cairn/dispatcher/internal/health"
	"github.com/Indexyz/Cairn/dispatcher/internal/workers"
)

// mockScript is the same helper the Python dispatcher shipped: the worker image
// has a python3 interpreter in both container and local mode.
const mockScript = `
import json,random,sys,time

try:
    cfg=json.loads(sys.argv[1])
    prompt=json.loads(sys.argv[2])
    phase=prompt["phase"]
    phase_cfg=cfg[phase]
except Exception as exc:
    print(f"mock setup failed: {exc}", file=sys.stderr)
    raise SystemExit(1)
delay=phase_cfg["delay"]
time.sleep(random.uniform(delay["min"],delay["max"]))

weights=dict(phase_cfg["outcomes"])
if phase=="reason":
    if not prompt.get("open_intents"):
        weights.pop("noop",None)
    if not prompt.get("fact_ids"):
        weights.pop("complete",None)
        weights.pop("intent",None)
choices=[(name,weight) for name,weight in weights.items() if weight>0]
if not choices:
    print(f"mock {phase} has no legal outcomes for prompt context", file=sys.stderr)
    raise SystemExit(2)

def _rule_matches(rule, prompt):
    fact_ids = prompt.get("fact_ids") or []
    open_intents = prompt.get("open_intents") or []
    if "fact_ids_gte" in rule and len(fact_ids) < rule["fact_ids_gte"]:
        return False
    if "fact_ids_lte" in rule and len(fact_ids) > rule["fact_ids_lte"]:
        return False
    if "open_intents_empty" in rule and (len(open_intents) == 0) != rule["open_intents_empty"]:
        return False
    return True

rules = phase_cfg.get("rules") or []
forced = None
for rule in rules:
    if _rule_matches(rule, prompt):
        forced = rule["force"]
        break

if forced is not None:
    outcome = forced
else:
    pick=random.uniform(0,sum(weight for _,weight in choices))
    total=0
    outcome=choices[-1][0]
    for name,weight in choices:
        total+=weight
        if pick<=total:
            outcome=name
            break

if phase=="healthcheck":
    raise SystemExit(0 if outcome=="ok" else 1)
if outcome=="command_fail":
    print(f"mock {phase} command failed", file=sys.stderr)
    raise SystemExit(1)
if outcome=="invalid_json":
    print("{invalid json")
    raise SystemExit(0)
if phase=="reason":
    fact_ids=prompt.get("fact_ids") or []
    max_i=prompt.get("max_intents",3)
    from_ids=[random.choice(fact_ids)] if fact_ids else []
    if outcome=="complete":
        print(json.dumps({"accepted":True,"data":{"complete":{"from":from_ids,"description":f"mock complete from {from_ids[0]}"}}}, ensure_ascii=False))
    elif outcome=="intent":
        count=random.randint(1,max(1,max_i))
        intents=[]
        for idx in range(count):
            fi=[random.choice(fact_ids)] if fact_ids else []
            intents.append({"from":fi,"description":f"mock intent {idx+1} from {fi[0] if fi else 'none'}"})
        print(json.dumps({"accepted":True,"data":{"intents":intents}}, ensure_ascii=False))
    elif outcome=="noop":
        print(json.dumps({"accepted":True,"data":{}}, ensure_ascii=False))
    elif outcome=="rejected":
        print(json.dumps({"accepted":False,"reason":"mock_rejected"}, ensure_ascii=False))
    else:
        print(json.dumps({"accepted":True,"data":{"complete":{"description":"mock invalid payload"}}}, ensure_ascii=False))
    raise SystemExit(0)

if phase=="bootstrap":
    if outcome=="complete":
        print(json.dumps({"accepted":True,"data":{"fact":{"description":"mock fact for bootstrap"},"complete":{"description":"mock bootstrap complete from fact"}}}, ensure_ascii=False))
    elif outcome=="fact":
        print(json.dumps({"accepted":True,"data":{"fact":{"description":"mock fact-only bootstrap result"}}}, ensure_ascii=False))
    elif outcome=="rejected":
        print(json.dumps({"accepted":False,"reason":"mock_rejected"}, ensure_ascii=False))
    else:
        print(json.dumps({"accepted":True,"data":{"fact":{"description":"mock invalid payload"}}}, ensure_ascii=False))
    raise SystemExit(0)

if phase=="bootstrap_conclude":
    if outcome=="fact":
        print(json.dumps({"accepted":True,"data":{"fact":{"description":"mock fact for bootstrap_conclude"}}}, ensure_ascii=False))
    elif outcome=="rejected":
        print(json.dumps({"accepted":False,"reason":"mock_rejected"}, ensure_ascii=False))
    else:
        print(json.dumps({"accepted":True,"data":{"complete":{"description":"mock invalid payload"}}}, ensure_ascii=False))
    raise SystemExit(0)

if outcome=="fact":
    label = prompt.get("intent_id") or phase
    print(json.dumps({"accepted":True,"data":{"description":f"mock fact for {label}"}} , ensure_ascii=False))
elif outcome=="rejected":
    print(json.dumps({"accepted":False,"reason":"mock_rejected"}, ensure_ascii=False))
else:
    print(json.dumps({"accepted":True,"data":{}}, ensure_ascii=False))
`

// Mock is a deterministic-ish fake worker driven by MOCK_* env.
type Mock struct{ workers.SeedSessionDriver }

// TypeName returns the config discriminator.
func (Mock) TypeName() string { return "mock" }

// LocalBinary reports the interpreter the script needs.
func (Mock) LocalBinary() *string { return workers.StringPtr("python3") }

// CheckHealth rolls the configured healthcheck outcome.
func (Mock) CheckHealth(worker *config.WorkerConfig, _ time.Duration) health.Result {
	behavior, err := config.ResolveMockBehavior(worker.Name, worker.Env)
	if err != nil {
		return health.Result{Detail: err.Error()}
	}
	ok := rand.Float64() < behavior["healthcheck"].Outcomes["ok"]
	status := 503
	if ok {
		status = 200
	}
	result := health.Result{OK: ok, Status: &status}
	if !ok {
		result.Detail = "mock healthcheck fail"
	}
	return result
}

// DescribeHealth labels the in-process roll.
func (Mock) DescribeHealth(*config.WorkerConfig) string { return "mock in-process healthcheck" }

// BuildExecute builds the mock argv.
func (Mock) BuildExecute(worker *config.WorkerConfig, prompt string, session *string) workers.DriverResult {
	return workers.DriverResult{Argv: mockArgv(worker, prompt), Session: session}
}

// BuildConclude builds the mock argv for a conclusion.
func (Mock) BuildConclude(worker *config.WorkerConfig, prompt, _ string) []string {
	return mockArgv(worker, prompt)
}

func mockArgv(worker *config.WorkerConfig, prompt string) []string {
	behavior, err := config.ResolveMockBehavior(worker.Name, worker.Env)
	if err != nil {
		behavior = config.MockBehavior{}
	}
	encoded, err := json.Marshal(encodeBehavior(behavior))
	if err != nil {
		encoded = []byte("{}")
	}
	return []string{"python3", "-c", mockScript, string(encoded), prompt}
}

type mockDelay struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

type mockPhase struct {
	Delay    mockDelay          `json:"delay"`
	Outcomes map[string]float64 `json:"outcomes"`
	Rules    []map[string]any   `json:"rules,omitempty"`
}

func encodeBehavior(behavior config.MockBehavior) map[string]mockPhase {
	encoded := map[string]mockPhase{}
	for phase, value := range behavior {
		entry := mockPhase{
			Delay:    mockDelay{Min: value.MinDelay, Max: value.MaxDelay},
			Outcomes: value.Outcomes,
		}
		for _, rule := range value.Rules {
			item := map[string]any{"force": rule.Force}
			if rule.FactIDsGTE != nil {
				item["fact_ids_gte"] = *rule.FactIDsGTE
			}
			if rule.FactIDsLTE != nil {
				item["fact_ids_lte"] = *rule.FactIDsLTE
			}
			if rule.OpenIntentsEmpty != nil {
				item["open_intents_empty"] = *rule.OpenIntentsEmpty
			}
			entry.Rules = append(entry.Rules, item)
		}
		encoded[phase] = entry
	}
	return encoded
}
