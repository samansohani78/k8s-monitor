/*
Copyright 2026 K8sWatch.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/k8swatch/k8s-monitor/internal/alertmanager"
)

// SlackChannel sends notifications to Slack
type SlackChannel struct {
	config     *SlackConfig
	httpClient *http.Client
}

// SlackConfig holds Slack configuration
type SlackConfig struct {
	// WebhookURL is the Slack incoming webhook URL
	WebhookURL string
	// Channel is the default channel (overrides webhook default)
	Channel string
	// Username is the bot username
	Username string
	// IconEmoji is the bot icon emoji
	IconEmoji string
	// IconURL is the bot icon URL
	IconURL string
}

// SlackMessage represents a Slack message payload
type SlackMessage struct {
	Text        string        `json:"text,omitempty"`
	Channel     string        `json:"channel,omitempty"`
	Username    string        `json:"username,omitempty"`
	IconEmoji   string        `json:"icon_emoji,omitempty"`
	IconURL     string        `json:"icon_url,omitempty"`
	Attachments []Attachment  `json:"attachments,omitempty"`
	Blocks      []interface{} `json:"blocks,omitempty"`
}

// Attachment represents a Slack attachment
type Attachment struct {
	Color      string   `json:"color,omitempty"`
	AuthorName string   `json:"author_name,omitempty"`
	Title      string   `json:"title,omitempty"`
	Text       string   `json:"text,omitempty"`
	Fields     []Field  `json:"fields,omitempty"`
	Footer     string   `json:"footer,omitempty"`
	Ts         int64    `json:"ts,omitempty"`
	Actions    []Action `json:"actions,omitempty"`
}

// Field represents a Slack attachment field
type Field struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

// Action represents a Slack interactive action
type Action struct {
	Type  string `json:"type"`
	Text  string `json:"text"`
	Name  string `json:"name"`
	Value string `json:"value"`
	Style string `json:"style,omitempty"`
}

// NewSlackChannel creates a new Slack notification channel
func NewSlackChannel(config *SlackConfig) *SlackChannel {
	if config == nil {
		config = &SlackConfig{}
	}

	// Get webhook URL from environment if not provided
	if config.WebhookURL == "" {
		config.WebhookURL = os.Getenv("SLACK_WEBHOOK_URL")
	}

	if config.Username == "" {
		config.Username = "K8sWatch Alert"
	}

	if config.IconEmoji == "" {
		config.IconEmoji = ":warning:"
	}

	return &SlackChannel{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name returns the channel name
func (c *SlackChannel) Name() string {
	return "slack"
}

// Send sends a notification to Slack
func (c *SlackChannel) Send(ctx context.Context, alert *alertmanager.Alert) error {
	if c.config.WebhookURL == "" {
		return fmt.Errorf("Slack webhook URL not configured")
	}

	message := c.buildMessage(alert)

	payload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal Slack message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.WebhookURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create Slack request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send Slack message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Slack API returned status %d", resp.StatusCode)
	}

	return nil
}

// Close closes the channel
func (c *SlackChannel) Close() error {
	return nil
}

// buildMessage builds a Slack message from an alert
func (c *SlackChannel) buildMessage(alert *alertmanager.Alert) *SlackMessage {
	color := c.getSeverityColor(alert.Severity)
	emoji := c.getSeverityEmoji(alert.Severity)

	// Build attachment fields
	fields := []Field{
		{
			Title: "Target",
			Value: fmt.Sprintf("%s/%s", alert.Target.Namespace, alert.Target.Name),
			Short: true,
		},
		{
			Title: "Severity",
			Value: string(alert.Severity),
			Short: true,
		},
		{
			Title: "Failure",
			Value: fmt.Sprintf("%s (%s)", alert.FailureLayer, alert.FailureCode),
			Short: true,
		},
		{
			Title: "Blast Radius",
			Value: alert.BlastRadius,
			Short: true,
		},
		{
			Title: "Affected Nodes",
			Value: fmt.Sprintf("%d", len(alert.AffectedNodes)),
			Short: true,
		},
		{
			Title: "Consecutive Failures",
			Value: fmt.Sprintf("%d", alert.ConsecutiveFailures),
			Short: true,
		},
	}

	// Add action buttons for acknowledged alerts
	var actions []Action
	if alert.Status == alertmanager.AlertStateFiring {
		actions = []Action{
			{
				Type:  "button",
				Text:  ":white_check_mark: Acknowledge",
				Name:  "acknowledge",
				Value: alert.AlertID,
				Style: "primary",
			},
			{
				Type:  "button",
				Text:  ":mute: Silence",
				Name:  "silence",
				Value: alert.AlertID,
				Style: "default",
			},
		}
	}

	attachment := Attachment{
		Color:      color,
		AuthorName: "K8sWatch Alert",
		Title:      fmt.Sprintf("%s Alert: %s", emoji, alert.Status),
		Text:       c.getAlertText(alert),
		Fields:     fields,
		Footer:     "K8sWatch",
		Ts:         alert.FiredAt.Unix(),
		Actions:    actions,
	}

	message := &SlackMessage{
		Channel:     c.config.Channel,
		Username:    c.config.Username,
		IconEmoji:   emoji,
		Attachments: []Attachment{attachment},
	}

	return message
}

// getAlertText returns the alert text for Slack
func (c *SlackChannel) getAlertText(alert *alertmanager.Alert) string {
	text := fmt.Sprintf("*Target:* %s/%s\n", alert.Target.Namespace, alert.Target.Name)
	text += fmt.Sprintf("*Type:* %s\n", alert.Target.Type)

	if alert.FailureCode != "" {
		text += fmt.Sprintf("*Failure:* %s - %s\n", alert.FailureLayer, alert.FailureCode)
	}

	if len(alert.AffectedNodes) > 0 {
		text += fmt.Sprintf("*Affected Nodes:* %v\n", alert.AffectedNodes)
	}

	if alert.Annotations != nil {
		if runbook, ok := alert.Annotations["runbook"]; ok {
			text += fmt.Sprintf("*Runbook:* <%s|View Runbook>\n", runbook)
		}
	}

	return text
}

// getSeverityColor returns the Slack color for a severity
func (c *SlackChannel) getSeverityColor(severity alertmanager.AlertSeverity) string {
	switch severity {
	case alertmanager.AlertSeverityCritical:
		return "#dc3545" // Red
	case alertmanager.AlertSeverityWarning:
		return "#ffc107" // Yellow
	case alertmanager.AlertSeverityInfo:
		return "#17a2b8" // Blue
	default:
		return "#808080" // Gray
	}
}

// getSeverityEmoji returns the emoji for a severity
func (c *SlackChannel) getSeverityEmoji(severity alertmanager.AlertSeverity) string {
	switch severity {
	case alertmanager.AlertSeverityCritical:
		return ":rotating_light:"
	case alertmanager.AlertSeverityWarning:
		return ":warning:"
	case alertmanager.AlertSeverityInfo:
		return ":information_source:"
	default:
		return ":bell:"
	}
}
