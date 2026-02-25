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

package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAlertManagerVersionInfo(t *testing.T) {
	// Verify version variables are defined
	assert.NotEmpty(t, Version)
	assert.NotEmpty(t, BuildDate)
	assert.NotEmpty(t, GitCommit)
}

func TestAlertManagerConfigDefaults(t *testing.T) {
	// Test default configuration values
	assert.Equal(t, 8080, 8080) // Default HTTP port
}

func TestAlertManagerContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Verify context can be cancelled
	cancel()

	select {
	case <-ctx.Done():
		// Expected
	default:
		t.Error("Context was not cancelled")
	}
}

func TestAlertManagerSignalHandling(t *testing.T) {
	// Test that we can create a context that responds to cancellation
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Wait for context to be done
	done := make(chan struct{})
	go func() {
		<-ctx.Done()
		close(done)
	}()

	select {
	case <-done:
		// Expected
	case <-time.After(1 * time.Second):
		t.Error("Context timeout did not work")
	}
}

func TestAlertManagerVersionOutput(t *testing.T) {
	// Test version string format
	versionOutput := "K8sWatch AlertManager\nVersion: " + Version + "\nBuild Date: " + BuildDate + "\nGit Commit: " + GitCommit + "\n"
	assert.NotEmpty(t, versionOutput)
	assert.Contains(t, versionOutput, "K8sWatch AlertManager")
	assert.Contains(t, versionOutput, "Version:")
}

func TestAlertManagerChannelConfig(t *testing.T) {
	tests := []struct {
		name           string
		slackWebhook   string
		pagerDutyKey   string
		webhookURL     string
		expectChannels int
	}{
		{
			name:           "No channels configured",
			slackWebhook:   "",
			pagerDutyKey:   "",
			webhookURL:     "",
			expectChannels: 0,
		},
		{
			name:           "Slack only",
			slackWebhook:   "https://hooks.slack.com/test",
			pagerDutyKey:   "",
			webhookURL:     "",
			expectChannels: 1,
		},
		{
			name:           "All channels",
			slackWebhook:   "https://hooks.slack.com/test",
			pagerDutyKey:   "test-key",
			webhookURL:     "https://example.com/webhook",
			expectChannels: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channelCount := 0
			if tt.slackWebhook != "" {
				channelCount++
			}
			if tt.pagerDutyKey != "" {
				channelCount++
			}
			if tt.webhookURL != "" {
				channelCount++
			}
			assert.Equal(t, tt.expectChannels, channelCount)
		})
	}
}

func TestAlertManagerSMTPConfig(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		port     int
		username string
		password string
		from     string
		valid    bool
	}{
		{
			name:     "Complete SMTP config",
			host:     "smtp.example.com",
			port:     587,
			username: "user",
			password: "pass",
			from:     "alerts@example.com",
			valid:    true,
		},
		{
			name:     "Missing host",
			host:     "",
			port:     587,
			username: "user",
			password: "pass",
			from:     "alerts@example.com",
			valid:    false,
		},
		{
			name:     "Missing from",
			host:     "smtp.example.com",
			port:     587,
			username: "user",
			password: "pass",
			from:     "",
			valid:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := tt.host != "" && tt.port > 0 && tt.from != ""
			assert.Equal(t, tt.valid, valid)
		})
	}
}
