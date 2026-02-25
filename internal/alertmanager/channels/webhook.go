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

// WebhookChannel sends notifications to a generic webhook
type WebhookChannel struct {
	config     *WebhookConfig
	httpClient *http.Client
}

// WebhookConfig holds webhook configuration
type WebhookConfig struct {
	// URL is the webhook URL
	URL string
	// Headers are custom HTTP headers
	Headers map[string]string
	// Timeout is the request timeout
	Timeout time.Duration
}

// WebhookPayload represents the webhook payload
type WebhookPayload struct {
	AlertID             string            `json:"alertId"`
	Rule                string            `json:"rule"`
	TargetNamespace     string            `json:"targetNamespace"`
	TargetName          string            `json:"targetName"`
	TargetType          string            `json:"targetType"`
	Severity            string            `json:"severity"`
	Status              string            `json:"status"`
	FiredAt             time.Time         `json:"firedAt"`
	FailureLayer        string            `json:"failureLayer,omitempty"`
	FailureCode         string            `json:"failureCode,omitempty"`
	BlastRadius         string            `json:"blastRadius"`
	AffectedNodes       []string          `json:"affectedNodes,omitempty"`
	ConsecutiveFailures int32             `json:"consecutiveFailures"`
	Labels              map[string]string `json:"labels,omitempty"`
	Annotations         map[string]string `json:"annotations,omitempty"`
}

// NewWebhookChannel creates a new webhook notification channel
func NewWebhookChannel(config *WebhookConfig) *WebhookChannel {
	if config == nil {
		config = &WebhookConfig{}
	}

	// Get URL from environment if not provided
	if config.URL == "" {
		config.URL = os.Getenv("WEBHOOK_URL")
	}

	if config.Headers == nil {
		config.Headers = make(map[string]string)
	}

	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	return &WebhookChannel{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// Name returns the channel name
func (c *WebhookChannel) Name() string {
	return "webhook"
}

// Send sends a notification to the webhook
func (c *WebhookChannel) Send(ctx context.Context, alert *alertmanager.Alert) error {
	if c.config.URL == "" {
		return fmt.Errorf("webhook URL not configured")
	}

	payload := WebhookPayload{
		AlertID:             alert.AlertID,
		Rule:                alert.Rule,
		TargetNamespace:     alert.Target.Namespace,
		TargetName:          alert.Target.Name,
		TargetType:          string(alert.Target.Type),
		Severity:            string(alert.Severity),
		Status:              string(alert.Status),
		FiredAt:             alert.FiredAt,
		FailureLayer:        alert.FailureLayer,
		FailureCode:         alert.FailureCode,
		BlastRadius:         alert.BlastRadius,
		AffectedNodes:       alert.AffectedNodes,
		ConsecutiveFailures: alert.ConsecutiveFailures,
		Labels:              alert.Labels,
		Annotations:         alert.Annotations,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Add custom headers
	for k, v := range c.config.Headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// Close closes the channel
func (c *WebhookChannel) Close() error {
	return nil
}
