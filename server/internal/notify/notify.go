// Package notify delivers project events to external channels.
package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Indexyz/Cairn/server/internal/domain"
)

const (
	timeout     = 10 * time.Second
	detailLimit = 300
)

// Channel is a resolved target.
type Channel struct {
	ID     string
	Name   string
	Kind   string
	Events []string
	Config map[string]string
}

// Result is a delivery outcome.
type Result struct {
	OK         bool
	Status     *int
	Detail     string
	DurationMs int
}

// Deliverer sends messages; Wait blocks until the queue drains (tests).
type Deliverer struct {
	Client *http.Client
	wg     sync.WaitGroup
}

// Notify queues one message per channel.
func (deliverer *Deliverer) Notify(channels []Channel, event, projectID, title, text, link string) {
	for _, channel := range channels {
		deliverer.wg.Add(1)
		go func(channel Channel) {
			defer deliverer.wg.Done()
			deliverer.Send(channel, event, projectID, title, text, link)
		}(channel)
	}
}

// Wait blocks until every queued delivery finished.
func (deliverer *Deliverer) Wait() { deliverer.wg.Wait() }

// Send delivers one message and never panics.
func (deliverer *Deliverer) Send(channel Channel, event, projectID, title, text, link string) Result {
	target, body := payload(channel, event, projectID, title, text, link)
	encoded, err := json.Marshal(body)
	if err != nil {
		return Result{Detail: err.Error()}
	}
	started := time.Now()
	client := deliverer.Client
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	response, err := client.Post(target, "application/json", bytes.NewReader(encoded))
	if err != nil {
		return Result{Detail: err.Error()}
	}
	defer response.Body.Close()
	raw, _ := io.ReadAll(response.Body)
	detail := string(raw)
	if len(detail) > detailLimit {
		detail = detail[:detailLimit]
	}
	status := response.StatusCode
	return Result{
		OK:         status >= 200 && status < 300,
		Status:     &status,
		Detail:     detail,
		DurationMs: int(time.Since(started).Milliseconds()),
	}
}

// EventLabels are the message headings.
var EventLabels = map[string]string{
	"project_created":   "Project created",
	"project_stopped":   "Project paused",
	"project_resumed":   "Project resumed",
	"project_blocked":   "Project blocked (step limit reached)",
	"project_completed": "Project completed",
	"project_reopened":  "Project reopened",
}

// RenderMessage builds the pushed text.
func RenderMessage(event, projectID, title, detail, link string) string {
	lines := []string{"[Cairn] " + EventLabels[event], projectID + " · " + title}
	if strings.TrimSpace(detail) != "" {
		lines = append(lines, strings.TrimSpace(detail))
	}
	if link != "" {
		lines = append(lines, link)
	}
	lines = append(lines, utcnow())
	return strings.Join(lines, "\n")
}

// RenderTest builds the sample text.
func RenderTest(channelName string) string {
	return strings.Join([]string{
		"[Cairn] Test notification",
		channelName,
		"If you can read this, the channel is configured correctly.",
		utcnow(),
	}, "\n")
}

func payload(channel Channel, event, projectID, title, text, link string) (string, map[string]any) {
	switch channel.Kind {
	case domain.KindDiscord:
		return channel.Config["webhook_url"], map[string]any{
			"content": text, "allowed_mentions": map[string]any{"parse": []any{}},
		}
	case domain.KindTelegram:
		return "https://api.telegram.org/bot" + channel.Config["bot_token"] + "/sendMessage", map[string]any{
			"chat_id": channel.Config["chat_id"], "text": text, "disable_web_page_preview": true,
		}
	default:
		return channel.Config["url"], map[string]any{
			"event": event, "project_id": projectID, "title": title, "text": text, "project_url": link,
		}
	}
}

func utcnow() string { return time.Now().UTC().Format("2006-01-02T15:04:05Z") }

var _ = fmt.Sprint
