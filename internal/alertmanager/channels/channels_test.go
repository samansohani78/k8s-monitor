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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	k8swatchv1 "github.com/k8swatch/k8s-monitor/api/v1"
	"github.com/k8swatch/k8s-monitor/internal/alertmanager"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create test alert
func createTestAlert() *alertmanager.Alert {
	return &alertmanager.Alert{
		AlertID: "test-alert-id",
		Rule:    "test-rule",
		Target: k8swatchv1.TargetRef{
			Name:      "test-target",
			Namespace: "default",
			Type:      k8swatchv1.TargetTypeHTTP,
		},
		Severity:            alertmanager.AlertSeverityCritical,
		Status:              alertmanager.AlertStateFiring,
		FiredAt:             time.Now(),
		FailureLayer:        "L2",
		FailureCode:         "tcp_refused",
		BlastRadius:         "node",
		AffectedNodes:       []string{"node-1", "node-2"},
		ConsecutiveFailures: 3,
		LastUpdatedAt:       time.Now(),
	}
}

// =============================================================================
// PagerDuty Tests
// =============================================================================

func TestPagerDutyConfigDefaults(t *testing.T) {
	cfg := &PagerDutyConfig{}
	assert.Empty(t, cfg.RoutingKey)
	assert.Empty(t, cfg.URL)
	assert.Empty(t, cfg.DefaultSeverity)
}

func TestPagerDutyChannelCreation(t *testing.T) {
	cfg := &PagerDutyConfig{
		RoutingKey: "test-routing-key",
	}

	channel := NewPagerDutyChannel(cfg)
	assert.NotNil(t, channel)
	assert.Equal(t, "pagerduty", channel.Name())
}

func TestPagerDutyChannelCreationNilConfig(t *testing.T) {
	channel := NewPagerDutyChannel(nil)
	assert.NotNil(t, channel)
	assert.Equal(t, "pagerduty", channel.Name())
}

func TestPagerDutyChannelCreationFromEnv(t *testing.T) {
	os.Setenv("PAGERDUTY_ROUTING_KEY", "env-routing-key")
	defer os.Unsetenv("PAGERDUTY_ROUTING_KEY")

	cfg := &PagerDutyConfig{}
	channel := NewPagerDutyChannel(cfg)
	assert.NotNil(t, channel)
}

func TestPagerDutySendNoRoutingKey(t *testing.T) {
	cfg := &PagerDutyConfig{}
	channel := NewPagerDutyChannel(cfg)

	alert := createTestAlert()

	err := channel.Send(context.Background(), alert)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "routing key not configured")
}

func TestPagerDutySendSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":    "success",
			"message":   "Event processed",
			"dedup_key": "test-dedup-key",
		})
	}))
	defer server.Close()

	cfg := &PagerDutyConfig{
		RoutingKey: "test-routing-key",
		URL:        server.URL,
	}

	channel := NewPagerDutyChannel(cfg)

	alert := createTestAlert()

	err := channel.Send(context.Background(), alert)
	assert.NoError(t, err)
}

func TestPagerDutySendServerFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := &PagerDutyConfig{
		RoutingKey: "test-routing-key",
		URL:        server.URL,
	}

	channel := NewPagerDutyChannel(cfg)

	alert := createTestAlert()

	err := channel.Send(context.Background(), alert)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "PagerDuty API returned status")
}

func TestPagerDutySendContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &PagerDutyConfig{
		RoutingKey: "test-routing-key",
		URL:        server.URL,
	}

	channel := NewPagerDutyChannel(cfg)

	alert := createTestAlert()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := channel.Send(ctx, alert)
	assert.Error(t, err)
}

func TestPagerDutySeverityMapping(t *testing.T) {
	tests := []struct {
		alertSeverity      alertmanager.AlertSeverity
		expectedPDSeverity string
	}{
		{alertmanager.AlertSeverityCritical, "critical"},
		{alertmanager.AlertSeverityWarning, "warning"},
		{alertmanager.AlertSeverityInfo, "info"},
	}

	for _, tt := range tests {
		t.Run(string(tt.alertSeverity), func(t *testing.T) {
			var receivedPayload map[string]interface{}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				decoder := json.NewDecoder(r.Body)
				err := decoder.Decode(&receivedPayload)
				assert.NoError(t, err)
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			cfg := &PagerDutyConfig{
				RoutingKey: "test-routing-key",
				URL:        server.URL,
			}

			channel := NewPagerDutyChannel(cfg)

			alert := createTestAlert()
			alert.Severity = tt.alertSeverity

			err := channel.Send(context.Background(), alert)
			assert.NoError(t, err)

			// Verify severity mapping in payload
			if payload, ok := receivedPayload["payload"].(map[string]interface{}); ok {
				assert.Equal(t, tt.expectedPDSeverity, payload["severity"])
			}
		})
	}
}

func TestPagerDutySendRecovery(t *testing.T) {
	var receivedPayload map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&receivedPayload)
		assert.NoError(t, err)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &PagerDutyConfig{
		RoutingKey: "test-routing-key",
		URL:        server.URL,
	}

	channel := NewPagerDutyChannel(cfg)

	alert := createTestAlert()

	// Note: Send always triggers, Resolve is separate method
	err := channel.Send(context.Background(), alert)
	assert.NoError(t, err)

	// Verify event_action is "trigger" for Send
	assert.Equal(t, "trigger", receivedPayload["event_action"])
}

func TestPagerDutySendTrigger(t *testing.T) {
	var receivedPayload map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&receivedPayload)
		assert.NoError(t, err)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &PagerDutyConfig{
		RoutingKey: "test-routing-key",
		URL:        server.URL,
	}

	channel := NewPagerDutyChannel(cfg)

	alert := createTestAlert()

	err := channel.Send(context.Background(), alert)
	assert.NoError(t, err)

	// Verify event_action is "trigger" for new alerts
	assert.Equal(t, "trigger", receivedPayload["event_action"])
}

func TestPagerDutyPayloadStructure(t *testing.T) {
	var receivedPayload map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&receivedPayload)
		assert.NoError(t, err)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &PagerDutyConfig{
		RoutingKey: "test-routing-key",
		URL:        server.URL,
	}

	channel := NewPagerDutyChannel(cfg)

	alert := createTestAlert()

	err := channel.Send(context.Background(), alert)
	require.NoError(t, err)

	// Verify payload structure
	assert.NotNil(t, receivedPayload)
	assert.Equal(t, "test-routing-key", receivedPayload["routing_key"])
	assert.Contains(t, receivedPayload, "event_action")
	assert.Contains(t, receivedPayload, "payload")

	if payload, ok := receivedPayload["payload"].(map[string]interface{}); ok {
		assert.Contains(t, payload, "summary")
		assert.Contains(t, payload, "source")
		assert.Contains(t, payload, "severity")
	}
}

// =============================================================================
// Webhook Tests
// =============================================================================

func TestWebhookConfigDefaults(t *testing.T) {
	cfg := &WebhookConfig{}
	assert.Empty(t, cfg.URL)
	assert.Nil(t, cfg.Headers)
	assert.Equal(t, time.Duration(0), cfg.Timeout)
}

func TestWebhookChannelCreation(t *testing.T) {
	cfg := &WebhookConfig{
		URL: "https://example.com/webhook",
	}

	channel := NewWebhookChannel(cfg)
	assert.NotNil(t, channel)
	assert.Equal(t, "webhook", channel.Name())
}

func TestWebhookChannelCreationNilConfig(t *testing.T) {
	channel := NewWebhookChannel(nil)
	assert.NotNil(t, channel)
	assert.Equal(t, "webhook", channel.Name())
}

func TestWebhookChannelCreationFromEnv(t *testing.T) {
	os.Setenv("WEBHOOK_URL", "https://example.com/env-webhook")
	defer os.Unsetenv("WEBHOOK_URL")

	cfg := &WebhookConfig{}
	channel := NewWebhookChannel(cfg)
	assert.NotNil(t, channel)
}

func TestWebhookSendNoURL(t *testing.T) {
	cfg := &WebhookConfig{}
	channel := NewWebhookChannel(cfg)

	alert := createTestAlert()

	err := channel.Send(context.Background(), alert)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "webhook URL not configured")
}

func TestWebhookSendSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &WebhookConfig{
		URL: server.URL,
	}

	channel := NewWebhookChannel(cfg)

	alert := createTestAlert()

	err := channel.Send(context.Background(), alert)
	assert.NoError(t, err)
}

func TestWebhookSendWithCustomHeaders(t *testing.T) {
	var receivedHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &WebhookConfig{
		URL: server.URL,
		Headers: map[string]string{
			"X-Custom-Header": "custom-value",
			"Authorization":   "Bearer token123",
		},
	}

	channel := NewWebhookChannel(cfg)

	alert := createTestAlert()

	err := channel.Send(context.Background(), alert)
	assert.NoError(t, err)

	assert.Equal(t, "custom-value", receivedHeaders.Get("X-Custom-Header"))
	assert.Equal(t, "Bearer token123", receivedHeaders.Get("Authorization"))
}

func TestWebhookSendServerFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := &WebhookConfig{
		URL: server.URL,
	}

	channel := NewWebhookChannel(cfg)

	alert := createTestAlert()

	err := channel.Send(context.Background(), alert)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "webhook returned status")
}

func TestWebhookSendContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &WebhookConfig{
		URL: server.URL,
	}

	channel := NewWebhookChannel(cfg)

	alert := createTestAlert()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := channel.Send(ctx, alert)
	assert.Error(t, err)
}

func TestWebhookSendTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &WebhookConfig{
		URL:     server.URL,
		Timeout: 100 * time.Millisecond,
	}

	channel := NewWebhookChannel(cfg)

	alert := createTestAlert()

	err := channel.Send(context.Background(), alert)
	assert.Error(t, err)
}

func TestWebhookPayloadStructure(t *testing.T) {
	var receivedPayload map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&receivedPayload)
		assert.NoError(t, err)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &WebhookConfig{
		URL: server.URL,
	}

	channel := NewWebhookChannel(cfg)

	alert := createTestAlert()

	err := channel.Send(context.Background(), alert)
	require.NoError(t, err)

	// Verify payload structure
	assert.NotNil(t, receivedPayload)
	assert.Contains(t, receivedPayload, "alertId")
	assert.Contains(t, receivedPayload, "severity")
	assert.Contains(t, receivedPayload, "status")
	assert.Contains(t, receivedPayload, "firedAt")
}

func TestWebhookSendResolved(t *testing.T) {
	var receivedPayload map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&receivedPayload)
		assert.NoError(t, err)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &WebhookConfig{
		URL: server.URL,
	}

	channel := NewWebhookChannel(cfg)

	alert := createTestAlert()
	alert.Status = alertmanager.AlertStateResolved

	err := channel.Send(context.Background(), alert)
	assert.NoError(t, err)

	// Verify status is "resolved"
	if status, ok := receivedPayload["status"].(string); ok {
		assert.Equal(t, "resolved", status)
	}
}

// =============================================================================
// Email Tests
// =============================================================================

func TestEmailConfigDefaults(t *testing.T) {
	cfg := &EmailConfig{}
	assert.Empty(t, cfg.SMTPHost)
	assert.Equal(t, 0, cfg.SMTPPort)
	assert.Empty(t, cfg.Username)
	assert.Empty(t, cfg.Password)
	assert.Empty(t, cfg.From)
	assert.Nil(t, cfg.To)
	assert.False(t, cfg.UseTLS)
}

func TestEmailChannelCreation(t *testing.T) {
	cfg := &EmailConfig{
		SMTPHost: "smtp.example.com",
		SMTPPort: 587,
		From:     "alerts@example.com",
		To:       []string{"oncall@example.com"},
	}

	channel := NewEmailChannel(cfg)
	assert.NotNil(t, channel)
	assert.Equal(t, "email", channel.Name())
}

func TestEmailChannelCreationNilConfig(t *testing.T) {
	channel := NewEmailChannel(nil)
	assert.NotNil(t, channel)
	assert.Equal(t, "email", channel.Name())
}

func TestEmailChannelCreationFromEnv(t *testing.T) {
	os.Setenv("SMTP_HOST", "smtp.env.example.com")
	os.Setenv("SMTP_PORT", "587")
	os.Setenv("SMTP_USERNAME", "env-user")
	os.Setenv("SMTP_PASSWORD", "env-password")
	os.Setenv("SMTP_FROM", "alerts@env.example.com")
	defer func() {
		os.Unsetenv("SMTP_HOST")
		os.Unsetenv("SMTP_PORT")
		os.Unsetenv("SMTP_USERNAME")
		os.Unsetenv("SMTP_PASSWORD")
		os.Unsetenv("SMTP_FROM")
	}()

	cfg := &EmailConfig{}
	channel := NewEmailChannel(cfg)
	assert.NotNil(t, channel)
}

func TestEmailSendNoSMTPHost(t *testing.T) {
	cfg := &EmailConfig{}
	channel := NewEmailChannel(cfg)

	alert := createTestAlert()

	err := channel.Send(context.Background(), alert)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SMTP host not configured")
}

func TestEmailSendNoRecipients(t *testing.T) {
	cfg := &EmailConfig{
		SMTPHost: "smtp.example.com",
		SMTPPort: 587,
		From:     "alerts@example.com",
	}

	channel := NewEmailChannel(cfg)

	alert := createTestAlert()

	err := channel.Send(context.Background(), alert)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no recipients configured")
}

func TestEmailSendContextCancellation(t *testing.T) {
	cfg := &EmailConfig{
		SMTPHost: "smtp.example.com",
		SMTPPort: 587,
		From:     "alerts@example.com",
		To:       []string{"oncall@example.com"},
	}

	channel := NewEmailChannel(cfg)

	alert := createTestAlert()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := channel.Send(ctx, alert)
	assert.Error(t, err)
}

func TestEmailSubjectFormat(t *testing.T) {
	alert := createTestAlert()

	// Test that subject is formatted correctly (won't actually send)
	subject := fmt.Sprintf("[%s] K8sWatch Alert: %s/%s - %s",
		alert.Severity,
		alert.Target.Namespace,
		alert.Target.Name,
		alert.Rule)

	assert.Contains(t, subject, "[critical]")
	assert.Contains(t, subject, "default/test-target")
}

// =============================================================================
// Slack Channel Tests
// =============================================================================

func TestSlackChannelCreation(t *testing.T) {
	cfg := &SlackConfig{
		WebhookURL: "https://hooks.slack.com/services/TEST",
		Channel:    "#alerts",
		Username:   "K8sWatch",
		IconEmoji:  ":warning:",
	}

	channel := NewSlackChannel(cfg)
	assert.NotNil(t, channel)
	assert.Equal(t, "slack", channel.Name())
}

func TestSlackChannelCreationNilConfig(t *testing.T) {
	channel := NewSlackChannel(nil)
	assert.NotNil(t, channel)
	assert.Equal(t, "slack", channel.Name())
}

func TestSlackChannelCreationFromEnv(t *testing.T) {
	os.Setenv("SLACK_WEBHOOK_URL", "https://hooks.slack.com/services/ENV")
	defer os.Unsetenv("SLACK_WEBHOOK_URL")

	cfg := &SlackConfig{}
	channel := NewSlackChannel(cfg)
	assert.NotNil(t, channel)
}

func TestSlackSendNoWebhookURL(t *testing.T) {
	cfg := &SlackConfig{}
	channel := NewSlackChannel(cfg)

	alert := createTestAlert()

	err := channel.Send(context.Background(), alert)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "webhook URL not configured")
}

func TestSlackSendSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &SlackConfig{
		WebhookURL: server.URL,
		Channel:    "#alerts",
		Username:   "K8sWatch",
		IconEmoji:  ":warning:",
	}

	channel := NewSlackChannel(cfg)

	alert := createTestAlert()

	err := channel.Send(context.Background(), alert)
	assert.NoError(t, err)
}

func TestSlackSendServerFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := &SlackConfig{
		WebhookURL: server.URL,
	}

	channel := NewSlackChannel(cfg)

	alert := createTestAlert()

	err := channel.Send(context.Background(), alert)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Slack API returned status")
}

func TestSlackSendContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &SlackConfig{
		WebhookURL: server.URL,
	}

	channel := NewSlackChannel(cfg)

	alert := createTestAlert()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := channel.Send(ctx, alert)
	assert.Error(t, err)
}

func TestSlackClose(t *testing.T) {
	cfg := &SlackConfig{
		WebhookURL: "https://hooks.slack.com/services/TEST",
	}

	channel := NewSlackChannel(cfg)
	err := channel.Close()
	assert.NoError(t, err)
}

func TestSlackBuildMessage(t *testing.T) {
	cfg := &SlackConfig{
		WebhookURL: "https://hooks.slack.com/services/TEST",
		Channel:    "#alerts",
		Username:   "K8sWatch",
		IconEmoji:  ":warning:",
	}

	channel := &SlackChannel{
		config:     cfg,
		httpClient: &http.Client{},
	}

	alert := createTestAlert()

	msg := channel.buildMessage(alert)
	assert.NotNil(t, msg)
	assert.Equal(t, "#alerts", msg.Channel)
	assert.Equal(t, "K8sWatch", msg.Username)
	assert.NotEmpty(t, msg.Attachments)
	if len(msg.Attachments) > 0 {
		assert.NotEmpty(t, msg.Attachments[0].Text)
	}
}

func TestSlackGetAlertText(t *testing.T) {
	cfg := &SlackConfig{
		WebhookURL: "https://hooks.slack.com/services/TEST",
	}

	channel := &SlackChannel{
		config: cfg,
	}

	alert := createTestAlert()

	text := channel.getAlertText(alert)
	assert.NotEmpty(t, text)
	assert.Contains(t, text, "test-target")
	assert.Contains(t, text, "tcp_refused")
}

func TestSlackGetSeverityColor(t *testing.T) {
	cfg := &SlackConfig{
		WebhookURL: "https://hooks.slack.com/services/TEST",
	}

	channel := &SlackChannel{
		config: cfg,
	}

	tests := []struct {
		severity alertmanager.AlertSeverity
		color    string
	}{
		{alertmanager.AlertSeverityCritical, "#dc3545"},
		{alertmanager.AlertSeverityWarning, "#ffc107"},
		{alertmanager.AlertSeverityInfo, "#17a2b8"},
		{alertmanager.AlertSeverity("unknown"), "#808080"},
	}

	for _, tt := range tests {
		t.Run(string(tt.severity), func(t *testing.T) {
			color := channel.getSeverityColor(tt.severity)
			assert.Equal(t, tt.color, color)
		})
	}
}

func TestSlackGetSeverityEmoji(t *testing.T) {
	cfg := &SlackConfig{
		WebhookURL: "https://hooks.slack.com/services/TEST",
	}

	channel := &SlackChannel{
		config: cfg,
	}

	tests := []struct {
		severity alertmanager.AlertSeverity
		emoji    string
	}{
		{alertmanager.AlertSeverityCritical, ":rotating_light:"},
		{alertmanager.AlertSeverityWarning, ":warning:"},
		{alertmanager.AlertSeverityInfo, ":information_source:"},
		{alertmanager.AlertSeverity("unknown"), ":bell:"},
	}

	for _, tt := range tests {
		t.Run(string(tt.severity), func(t *testing.T) {
			emoji := channel.getSeverityEmoji(tt.severity)
			assert.Equal(t, tt.emoji, emoji)
		})
	}
}

func TestSlackSendWithAllSeverities(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &SlackConfig{
		WebhookURL: server.URL,
	}

	channel := NewSlackChannel(cfg)

	severities := []alertmanager.AlertSeverity{
		alertmanager.AlertSeverityCritical,
		alertmanager.AlertSeverityWarning,
		alertmanager.AlertSeverityInfo,
	}

	for _, severity := range severities {
		t.Run(string(severity), func(t *testing.T) {
			alert := createTestAlert()
			alert.Severity = severity

			err := channel.Send(context.Background(), alert)
			assert.NoError(t, err)
		})
	}
}

func TestSlackSendResolved(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &SlackConfig{
		WebhookURL: server.URL,
	}

	channel := NewSlackChannel(cfg)

	alert := createTestAlert()
	alert.Status = alertmanager.AlertStateResolved

	err := channel.Send(context.Background(), alert)
	assert.NoError(t, err)
}

// =============================================================================
// Email Channel Extended Tests
// =============================================================================

func TestEmailClose(t *testing.T) {
	cfg := &EmailConfig{
		SMTPHost: "smtp.example.com",
		SMTPPort: 587,
		From:     "alerts@example.com",
		To:       []string{"oncall@example.com"},
	}

	channel := NewEmailChannel(cfg)
	err := channel.Close()
	assert.NoError(t, err)
}

func TestEmailSendWithTLS(t *testing.T) {
	cfg := &EmailConfig{
		SMTPHost: "smtp.example.com",
		SMTPPort: 587,
		From:     "alerts@example.com",
		To:       []string{"oncall@example.com"},
		UseTLS:   true,
	}

	channel := NewEmailChannel(cfg)

	alert := createTestAlert()

	// This will fail (no actual SMTP server), but tests the sendWithTLS path
	err := channel.Send(context.Background(), alert)
	assert.Error(t, err)
}

func TestEmailSendWithoutTLS(t *testing.T) {
	cfg := &EmailConfig{
		SMTPHost: "smtp.example.com",
		SMTPPort: 25,
		From:     "alerts@example.com",
		To:       []string{"oncall@example.com"},
		UseTLS:   false,
	}

	channel := NewEmailChannel(cfg)

	alert := createTestAlert()

	// This will fail (no actual SMTP server), but tests the sendWithPlain path
	err := channel.Send(context.Background(), alert)
	assert.Error(t, err)
}

func TestEmailBuildBody(t *testing.T) {
	cfg := &EmailConfig{
		SMTPHost: "smtp.example.com",
		From:     "alerts@example.com",
		To:       []string{"oncall@example.com"},
	}

	channel := &EmailChannel{
		config: cfg,
	}

	alert := createTestAlert()

	body, err := channel.buildEmailBody(alert)
	assert.NoError(t, err)
	assert.NotEmpty(t, body)
	assert.Contains(t, body, "test-target")
	assert.Contains(t, body, "critical")
	assert.Contains(t, body, "tcp_refused")
}

func TestEmailBuildBodyResolved(t *testing.T) {
	cfg := &EmailConfig{
		SMTPHost: "smtp.example.com",
		From:     "alerts@example.com",
		To:       []string{"oncall@example.com"},
	}

	channel := &EmailChannel{
		config: cfg,
	}

	resolvedTime := time.Now()
	alert := createTestAlert()
	alert.Status = alertmanager.AlertStateResolved
	alert.ResolvedAt = &resolvedTime

	body, err := channel.buildEmailBody(alert)
	assert.NoError(t, err)
	assert.NotEmpty(t, body)
	assert.Contains(t, body, "resolved")
}

func TestEmailConfigFromEnv(t *testing.T) {
	os.Setenv("SMTP_HOST", "smtp.env.example.com")
	os.Setenv("SMTP_PORT", "465")
	os.Setenv("SMTP_USERNAME", "env-user")
	os.Setenv("SMTP_PASSWORD", "env-pass")
	os.Setenv("SMTP_FROM", "alerts@env.example.com")
	defer func() {
		os.Unsetenv("SMTP_HOST")
		os.Unsetenv("SMTP_PORT")
		os.Unsetenv("SMTP_USERNAME")
		os.Unsetenv("SMTP_PASSWORD")
		os.Unsetenv("SMTP_FROM")
	}()

	cfg := &EmailConfig{}
	channel := NewEmailChannel(cfg)

	assert.NotNil(t, channel)
	assert.Equal(t, "email", channel.Name())
}

func TestEmailSendContextCancelled(t *testing.T) {
	cfg := &EmailConfig{
		SMTPHost: "smtp.example.com",
		SMTPPort: 587,
		From:     "alerts@example.com",
		To:       []string{"oncall@example.com"},
	}

	channel := NewEmailChannel(cfg)

	alert := createTestAlert()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := channel.Send(ctx, alert)
	assert.Error(t, err)
}

func TestEmailSendWithMultipleRecipients(t *testing.T) {
	cfg := &EmailConfig{
		SMTPHost: "smtp.example.com",
		SMTPPort: 587,
		From:     "alerts@example.com",
		To:       []string{"oncall1@example.com", "oncall2@example.com", "oncall3@example.com"},
	}

	channel := NewEmailChannel(cfg)

	alert := createTestAlert()

	// Will fail (no server), but tests multiple recipients path
	err := channel.Send(context.Background(), alert)
	assert.Error(t, err)
}

// =============================================================================
// PagerDuty Channel Extended Tests
// =============================================================================

func TestPagerDutyResolve(t *testing.T) {
	var receivedPayload map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&receivedPayload)
		assert.NoError(t, err)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":  "success",
			"message": "Event processed",
		})
	}))
	defer server.Close()

	cfg := &PagerDutyConfig{
		RoutingKey: "test-routing-key",
		URL:        server.URL,
	}

	channel := NewPagerDutyChannel(cfg)

	alert := createTestAlert()
	alert.Status = alertmanager.AlertStateResolved
	resolvedTime := time.Now()
	alert.ResolvedAt = &resolvedTime

	err := channel.Resolve(context.Background(), alert)
	assert.NoError(t, err)

	// Verify resolve event
	assert.Equal(t, "resolve", receivedPayload["event_action"])
}

func TestPagerDutyResolveNoRoutingKey(t *testing.T) {
	cfg := &PagerDutyConfig{}
	channel := NewPagerDutyChannel(cfg)

	alert := createTestAlert()
	alert.Status = alertmanager.AlertStateResolved

	err := channel.Resolve(context.Background(), alert)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "routing key not configured")
}

func TestPagerDutyClose(t *testing.T) {
	cfg := &PagerDutyConfig{
		RoutingKey: "test-routing-key",
	}

	channel := NewPagerDutyChannel(cfg)
	err := channel.Close()
	assert.NoError(t, err)
}

func TestPagerDutyBuildResolveEvent(t *testing.T) {
	cfg := &PagerDutyConfig{
		RoutingKey: "test-routing-key",
	}

	channel := &PagerDutyChannel{
		config: cfg,
	}

	resolvedTime := time.Now()
	alert := createTestAlert()
	alert.Status = alertmanager.AlertStateResolved
	alert.ResolvedAt = &resolvedTime

	event := channel.buildResolveEvent(alert)
	assert.NotNil(t, event)
	assert.Equal(t, "resolve", event.EventAction)
	assert.Equal(t, "test-alert-id", event.DedupKey)
	assert.NotNil(t, event.Payload)
	assert.Equal(t, "info", event.Payload.Severity)
}

func TestPagerDutyMapSeverity(t *testing.T) {
	cfg := &PagerDutyConfig{
		RoutingKey: "test-routing-key",
	}

	channel := &PagerDutyChannel{
		config: cfg,
	}

	tests := []struct {
		severity         alertmanager.AlertSeverity
		expectedSeverity string
	}{
		{alertmanager.AlertSeverityCritical, "critical"},
		{alertmanager.AlertSeverityWarning, "warning"},
		{alertmanager.AlertSeverityInfo, "info"},
		{alertmanager.AlertSeverity("unknown"), "warning"},
	}

	for _, tt := range tests {
		t.Run(string(tt.severity), func(t *testing.T) {
			severity := channel.mapSeverity(tt.severity)
			assert.Equal(t, tt.expectedSeverity, severity)
		})
	}
}

func TestPagerDutyGetSummary(t *testing.T) {
	cfg := &PagerDutyConfig{
		RoutingKey: "test-routing-key",
	}

	channel := &PagerDutyChannel{
		config: cfg,
	}

	alert := createTestAlert()

	summary := channel.getSummary(alert)
	assert.NotEmpty(t, summary)
	assert.Contains(t, summary, "critical")
	assert.Contains(t, summary, "test-target")
}

func TestPagerDutySendWithContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &PagerDutyConfig{
		RoutingKey: "test-routing-key",
		URL:        server.URL,
	}

	channel := NewPagerDutyChannel(cfg)

	alert := createTestAlert()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := channel.Send(ctx, alert)
	assert.Error(t, err)
}

func TestPagerDutyResolveWithContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &PagerDutyConfig{
		RoutingKey: "test-routing-key",
		URL:        server.URL,
	}

	channel := NewPagerDutyChannel(cfg)

	alert := createTestAlert()
	alert.Status = alertmanager.AlertStateResolved
	resolvedTime := time.Now()
	alert.ResolvedAt = &resolvedTime

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := channel.Resolve(ctx, alert)
	assert.Error(t, err)
}

func TestPagerDutyBuildEventWithAllFields(t *testing.T) {
	cfg := &PagerDutyConfig{
		RoutingKey: "test-routing-key",
	}

	channel := &PagerDutyChannel{
		config: cfg,
	}

	alert := createTestAlert()

	event := channel.buildEvent(alert)
	assert.NotNil(t, event)
	assert.Equal(t, "trigger", event.EventAction)
	assert.Equal(t, "test-alert-id", event.DedupKey)
	assert.NotNil(t, event.Payload)
	assert.Equal(t, "critical", event.Payload.Severity)
}

// =============================================================================
// Webhook Channel Extended Tests
// =============================================================================

func TestWebhookClose(t *testing.T) {
	cfg := &WebhookConfig{
		URL: "https://example.com/webhook",
	}

	channel := NewWebhookChannel(cfg)
	err := channel.Close()
	assert.NoError(t, err)
}

func TestWebhookSendWithContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &WebhookConfig{
		URL: server.URL,
	}

	channel := NewWebhookChannel(cfg)

	alert := createTestAlert()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := channel.Send(ctx, alert)
	assert.Error(t, err)
}

func TestWebhookSendWithTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &WebhookConfig{
		URL:     server.URL,
		Timeout: 100 * time.Millisecond,
	}

	channel := NewWebhookChannel(cfg)

	alert := createTestAlert()

	err := channel.Send(context.Background(), alert)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestWebhookSendWithEmptyHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &WebhookConfig{
		URL:     server.URL,
		Headers: map[string]string{},
		Timeout: 30 * time.Second,
	}

	channel := NewWebhookChannel(cfg)

	alert := createTestAlert()

	err := channel.Send(context.Background(), alert)
	assert.NoError(t, err)
}

func TestWebhookSendHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": "bad request"}`))
	}))
	defer server.Close()

	cfg := &WebhookConfig{
		URL: server.URL,
	}

	channel := NewWebhookChannel(cfg)

	alert := createTestAlert()

	err := channel.Send(context.Background(), alert)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "webhook returned status")
}

func TestWebhookConfigFromEnv(t *testing.T) {
	os.Setenv("WEBHOOK_URL", "https://hooks.example.com/webhook")
	defer os.Unsetenv("WEBHOOK_URL")

	cfg := &WebhookConfig{}
	channel := NewWebhookChannel(cfg)

	assert.NotNil(t, channel)
	assert.Equal(t, "webhook", channel.Name())
}

func TestWebhookSendWithNilConfig(t *testing.T) {
	channel := NewWebhookChannel(nil)
	assert.NotNil(t, channel)
	assert.Equal(t, "webhook", channel.Name())
}
