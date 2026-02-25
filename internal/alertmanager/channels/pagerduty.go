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

// PagerDutyChannel sends notifications to PagerDuty
type PagerDutyChannel struct {
	config     *PagerDutyConfig
	httpClient *http.Client
}

// PagerDutyConfig holds PagerDuty configuration
type PagerDutyConfig struct {
	// RoutingKey is the PagerDuty Events API v2 routing key
	RoutingKey string
	// URL is the PagerDuty Events API URL
	URL string
	// DefaultSeverity maps alert severity to PagerDuty severity
	DefaultSeverity string
}

// PagerDutyEvent represents a PagerDuty Events API v2 payload
type PagerDutyEvent struct {
	RoutingKey  string     `json:"routing_key"`
	EventAction string     `json:"event_action"`
	DedupKey    string     `json:"dedup_key,omitempty"`
	Payload     *PDPayload `json:"payload"`
	Images      []PDImage  `json:"images,omitempty"`
	Links       []PDLink   `json:"links,omitempty"`
}

// PDPayload represents the PagerDuty payload
type PDPayload struct {
	Summary       string                 `json:"summary"`
	Source        string                 `json:"source"`
	Severity      string                 `json:"severity"`
	Timestamp     string                 `json:"timestamp,omitempty"`
	Component     string                 `json:"component,omitempty"`
	Group         string                 `json:"group,omitempty"`
	Class         string                 `json:"class,omitempty"`
	CustomDetails map[string]interface{} `json:"custom_details,omitempty"`
}

// PDImage represents an image attachment
type PDImage struct {
	Src  string `json:"src"`
	Alt  string `json:"alt,omitempty"`
	Href string `json:"href,omitempty"`
}

// PDLink represents a link attachment
type PDLink struct {
	Href string `json:"href"`
	Text string `json:"text"`
}

// PagerDutyResponse represents the PagerDuty API response
type PagerDutyResponse struct {
	Status   string `json:"status"`
	Message  string `json:"message"`
	DedupKey string `json:"dedup_key"`
}

// NewPagerDutyChannel creates a new PagerDuty notification channel
func NewPagerDutyChannel(config *PagerDutyConfig) *PagerDutyChannel {
	if config == nil {
		config = &PagerDutyConfig{}
	}

	// Get routing key from environment if not provided
	if config.RoutingKey == "" {
		config.RoutingKey = os.Getenv("PAGERDUTY_ROUTING_KEY")
	}

	if config.URL == "" {
		config.URL = "https://events.pagerduty.com/v2/enqueue"
	}

	return &PagerDutyChannel{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name returns the channel name
func (c *PagerDutyChannel) Name() string {
	return "pagerduty"
}

// Send sends a notification to PagerDuty
func (c *PagerDutyChannel) Send(ctx context.Context, alert *alertmanager.Alert) error {
	if c.config.RoutingKey == "" {
		return fmt.Errorf("PagerDuty routing key not configured")
	}

	event := c.buildEvent(alert)

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal PagerDuty event: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.URL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create PagerDuty request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send PagerDuty event: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		var response PagerDutyResponse
		_ = json.NewDecoder(resp.Body).Decode(&response)
		return fmt.Errorf("PagerDuty API returned status %d: %s", resp.StatusCode, response.Message)
	}

	return nil
}

// Resolve resolves a PagerDuty incident
func (c *PagerDutyChannel) Resolve(ctx context.Context, alert *alertmanager.Alert) error {
	if c.config.RoutingKey == "" {
		return fmt.Errorf("PagerDuty routing key not configured")
	}

	event := c.buildResolveEvent(alert)

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal PagerDuty event: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.URL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create PagerDuty request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send PagerDuty event: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		var response PagerDutyResponse
		_ = json.NewDecoder(resp.Body).Decode(&response)
		return fmt.Errorf("PagerDuty API returned status %d: %s", resp.StatusCode, response.Message)
	}

	return nil
}

// Close closes the channel
func (c *PagerDutyChannel) Close() error {
	return nil
}

// buildEvent builds a PagerDuty trigger event
func (c *PagerDutyChannel) buildEvent(alert *alertmanager.Alert) *PagerDutyEvent {
	return &PagerDutyEvent{
		RoutingKey:  c.config.RoutingKey,
		EventAction: "trigger",
		DedupKey:    alert.AlertID,
		Payload: &PDPayload{
			Summary:   c.getSummary(alert),
			Source:    fmt.Sprintf("k8swatch/%s/%s", alert.Target.Namespace, alert.Target.Name),
			Severity:  c.mapSeverity(alert.Severity),
			Timestamp: alert.FiredAt.Format(time.RFC3339),
			Component: string(alert.Target.Type),
			Group:     alert.BlastRadius,
			CustomDetails: map[string]interface{}{
				"target_name":          alert.Target.Name,
				"target_namespace":     alert.Target.Namespace,
				"target_type":          string(alert.Target.Type),
				"failure_layer":        alert.FailureLayer,
				"failure_code":         alert.FailureCode,
				"affected_nodes":       alert.AffectedNodes,
				"consecutive_failures": alert.ConsecutiveFailures,
				"labels":               alert.Labels,
			},
		},
	}
}

// buildResolveEvent builds a PagerDuty resolve event
func (c *PagerDutyChannel) buildResolveEvent(alert *alertmanager.Alert) *PagerDutyEvent {
	return &PagerDutyEvent{
		RoutingKey:  c.config.RoutingKey,
		EventAction: "resolve",
		DedupKey:    alert.AlertID,
		Payload: &PDPayload{
			Summary:   fmt.Sprintf("[RESOLVED] %s", c.getSummary(alert)),
			Source:    fmt.Sprintf("k8swatch/%s/%s", alert.Target.Namespace, alert.Target.Name),
			Severity:  "info",
			Timestamp: alert.ResolvedAt.Format(time.RFC3339),
			Component: string(alert.Target.Type),
			Group:     alert.BlastRadius,
		},
	}
}

// getSummary returns the PagerDuty summary
func (c *PagerDutyChannel) getSummary(alert *alertmanager.Alert) string {
	return fmt.Sprintf("[%s] K8sWatch Alert: %s/%s - %s",
		alert.Severity,
		alert.Target.Namespace,
		alert.Target.Name,
		alert.FailureCode,
	)
}

// mapSeverity maps alert severity to PagerDuty severity
func (c *PagerDutyChannel) mapSeverity(severity alertmanager.AlertSeverity) string {
	switch severity {
	case alertmanager.AlertSeverityCritical:
		return "critical"
	case alertmanager.AlertSeverityWarning:
		return "warning"
	case alertmanager.AlertSeverityInfo:
		return "info"
	default:
		return "warning"
	}
}
