// Package protocol is the HTTP client for the Cairn server API.
package protocol

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Indexyz/Cairn/dispatcher/internal/config"
	"github.com/Indexyz/Cairn/dispatcher/internal/logging"
)

var log = logging.New("cairn.dispatcher.protocol.client")

// ApiResult is a raw protocol response; status 0 means the request itself failed.
type ApiResult struct {
	StatusCode int
	Data       any
	Text       string
}

// Ok reports whether the server answered with a 2xx status.
func (result ApiResult) Ok() bool {
	return result.StatusCode >= 200 && result.StatusCode < 300
}

// Fact is a graph node.
type Fact struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

// Intent is a graph edge.
type Intent struct {
	ID              string   `json:"id"`
	From            []string `json:"from"`
	To              *string  `json:"to"`
	Description     string   `json:"description"`
	Creator         string   `json:"creator"`
	Worker          *string  `json:"worker"`
	LastHeartbeatAt *string  `json:"last_heartbeat_at"`
	CreatedAt       string   `json:"created_at"`
	ConcludedAt     *string  `json:"concluded_at"`
}

// Hint is an out-of-band hint.
type Hint struct {
	ID        string `json:"id"`
	Content   string `json:"content"`
	Creator   string `json:"creator"`
	CreatedAt string `json:"created_at"`
}

// ProjectReason is the project-level reason lease.
type ProjectReason struct {
	Worker          string  `json:"worker"`
	Trigger         *string `json:"trigger"`
	StartedAt       string  `json:"started_at"`
	LastHeartbeatAt string  `json:"last_heartbeat_at"`
}

// ProjectMeta is the project header.
type ProjectMeta struct {
	ID               string         `json:"id"`
	Title            string         `json:"title"`
	Status           string         `json:"status"`
	BootstrapEnabled bool           `json:"bootstrap_enabled"`
	MaxSteps         *int           `json:"max_steps"`
	CreatedAt        string         `json:"created_at"`
	Reason           *ProjectReason `json:"reason"`
}

// ProjectSummary adds graph counters to a project.
type ProjectSummary struct {
	ProjectMeta
	FactCount            int `json:"fact_count"`
	IntentCount          int `json:"intent_count"`
	WorkingIntentCount   int `json:"working_intent_count"`
	UnclaimedIntentCount int `json:"unclaimed_intent_count"`
	HintCount            int `json:"hint_count"`
}

// ProjectDetail is a full project snapshot.
type ProjectDetail struct {
	Project ProjectMeta `json:"project"`
	Facts   []Fact      `json:"facts"`
	Intents []Intent    `json:"intents"`
	Hints   []Hint      `json:"hints"`
}

// Settings are the server-side lease timeouts.
type Settings struct {
	IntentTimeout int `json:"intent_timeout"`
	ReasonTimeout int `json:"reason_timeout"`
}

// LoadSnapshot is the global lease load.
type LoadSnapshot struct {
	RunningTasks    int            `json:"running_tasks"`
	RunningProjects int            `json:"running_projects"`
	Projects        map[string]int `json:"projects"`
	Workers         map[string]int `json:"workers"`
}

// WorkerModel is a model row, including the read-only `used` counter.
type WorkerModel struct {
	config.WorkerConfig
	Used int `json:"used"`
}

// Client talks to one Cairn server.
type Client struct {
	baseURL    string
	timeout    time.Duration
	instanceID string
	http       *http.Client
}

// NewClient builds a client with a pooled transport.
func NewClient(baseURL string, timeout time.Duration, instanceID string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		timeout:    timeout,
		instanceID: instanceID,
		http: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        64,
				MaxIdleConnsPerHost: 64,
			},
		},
	}
}

// Close releases idle connections.
func (client *Client) Close() {
	client.http.CloseIdleConnections()
}

func (client *Client) who(worker string) string {
	if client.instanceID == "" {
		return worker
	}
	return worker + "@" + client.instanceID
}

func (client *Client) url(path string) string { return client.baseURL + "/api" + path }

// ListProjects returns every project summary.
func (client *Client) ListProjects() ([]ProjectSummary, error) {
	var summaries []ProjectSummary
	err := client.getJSON("/projects", &summaries)
	return summaries, err
}

// GetProject returns one project snapshot.
func (client *Client) GetProject(projectID string) (*ProjectDetail, error) {
	var detail ProjectDetail
	err := client.getJSON("/projects/"+projectID, &detail)
	if err != nil {
		return nil, err
	}
	return &detail, nil
}

// GetSettings returns the server lease timeouts.
func (client *Client) GetSettings() (*Settings, error) {
	var settings Settings
	err := client.getJSON("/settings", &settings)
	return &settings, err
}

// ListModels returns the worker model list.
func (client *Client) ListModels() ([]WorkerModel, error) {
	var models []WorkerModel
	err := client.getJSON("/models", &models)
	return models, err
}

// ReplaceModels writes a model snapshot and returns the stored rows.
func (client *Client) ReplaceModels(models []config.WorkerConfig) ([]WorkerModel, error) {
	payload := make([]map[string]any, 0, len(models))
	for _, model := range models {
		payload = append(payload, map[string]any{
			"name":        model.Name,
			"type":        model.Type,
			"task_types":  model.TaskTypes,
			"max_running": model.MaxRunning,
			"priority":    model.Priority,
			"env":         model.Env,
		})
	}
	var stored []WorkerModel
	err := client.doJSON(http.MethodPut, "/models", map[string]any{"models": payload}, &stored)
	return stored, err
}

// GetLoad returns the global lease load.
func (client *Client) GetLoad() (*LoadSnapshot, error) {
	var load LoadSnapshot
	err := client.getJSON("/load", &load)
	return &load, err
}

// ExportProject returns the YAML graph snapshot.
func (client *Client) ExportProject(projectID string) (string, error) {
	response, err := client.do(http.MethodGet, "/projects/"+projectID+"/export?format=yaml", nil)
	if err != nil {
		return "", err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("export failed with status %d", response.StatusCode)
	}
	return response.Text, nil
}

// Heartbeat renews (or claims) an intent lease.
func (client *Client) Heartbeat(projectID, intentID, worker string) ApiResult {
	return client.postJSON("/projects/"+projectID+"/intents/"+intentID+"/heartbeat",
		map[string]any{"worker": client.who(worker)})
}

// ClaimReason claims the project reason lease.
func (client *Client) ClaimReason(projectID, worker, trigger string) ApiResult {
	return client.postJSON("/projects/"+projectID+"/reason/claim",
		map[string]any{"worker": client.who(worker), "trigger": trigger})
}

// ReasonHeartbeat renews the project reason lease.
func (client *Client) ReasonHeartbeat(projectID, worker string) ApiResult {
	return client.postJSON("/projects/"+projectID+"/reason/heartbeat",
		map[string]any{"worker": client.who(worker)})
}

// ReleaseReason drops the project reason lease.
func (client *Client) ReleaseReason(projectID, worker string) ApiResult {
	return client.postJSON("/projects/"+projectID+"/reason/release",
		map[string]any{"worker": client.who(worker)})
}

// Release drops an intent claim.
func (client *Client) Release(projectID, intentID, worker string) ApiResult {
	return client.postJSON("/projects/"+projectID+"/intents/"+intentID+"/release",
		map[string]any{"worker": client.who(worker)})
}

// Conclude concludes an intent with a new fact.
func (client *Client) Conclude(projectID, intentID, worker, description string) ApiResult {
	return client.postJSON("/projects/"+projectID+"/intents/"+intentID+"/conclude",
		map[string]any{"worker": client.who(worker), "description": description})
}

// Complete marks the project completed.
func (client *Client) Complete(projectID string, from []string, description, worker string) ApiResult {
	return client.postJSON("/projects/"+projectID+"/complete",
		map[string]any{"from": from, "description": description, "worker": client.who(worker)})
}

// CreateIntent declares an unclaimed intent.
func (client *Client) CreateIntent(projectID string, from []string, description, creator string) ApiResult {
	return client.postJSON("/projects/"+projectID+"/intents",
		map[string]any{"from": from, "description": description, "creator": creator, "worker": nil})
}

func (client *Client) getJSON(path string, target any) error {
	response, err := client.do(http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("%s returned %d: %s", path, response.StatusCode, response.Text)
	}
	return decodeData(response.Data, target)
}

func (client *Client) doJSON(method, path string, body any, target any) error {
	response, err := client.do(method, path, body)
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("%s returned %d: %s", path, response.StatusCode, response.Text)
	}
	return decodeData(response.Data, target)
}

func decodeData(data any, target any) error {
	raw, ok := data.(json.RawMessage)
	if !ok {
		return nil
	}
	return json.Unmarshal(raw, target)
}

func (client *Client) postJSON(path string, body any) ApiResult {
	response, err := client.do(http.MethodPost, path, body)
	if err != nil {
		log.Warnf("request failed method=POST path=%s error=%s", path, err)
		return ApiResult{StatusCode: 0, Text: err.Error()}
	}
	return *response
}

type rawResponse = ApiResult

func (client *Client) do(method, path string, body any) (*ApiResult, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(encoded)
	}
	request, err := http.NewRequest(method, client.url(path), reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		request.Header.Set("content-type", "application/json")
	}
	response, err := client.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	text, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	result := &ApiResult{StatusCode: response.StatusCode, Text: string(text)}
	if strings.HasPrefix(response.Header.Get("content-type"), "application/json") {
		result.Data = json.RawMessage(text)
	}
	return result, nil
}
