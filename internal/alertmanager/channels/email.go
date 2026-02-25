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
	"fmt"
	"html/template"
	"net"
	"net/smtp"
	"os"
	"time"

	"github.com/k8swatch/k8s-monitor/internal/alertmanager"
)

// EmailChannel sends notifications via email
type EmailChannel struct {
	config *EmailConfig
}

// EmailConfig holds email configuration
type EmailConfig struct {
	// SMTPHost is the SMTP server host
	SMTPHost string
	// SMTPPort is the SMTP server port
	SMTPPort int
	// Username is the SMTP username
	Username string
	// Password is the SMTP password
	Password string
	// From is the sender email address
	From string
	// To is the list of recipient email addresses
	To []string
	// UseTLS enables TLS for SMTP
	UseTLS bool
}

// NewEmailChannel creates a new email notification channel
func NewEmailChannel(config *EmailConfig) *EmailChannel {
	if config == nil {
		config = &EmailConfig{}
	}

	// Get config from environment if not provided
	if config.SMTPHost == "" {
		config.SMTPHost = os.Getenv("SMTP_HOST")
	}
	if config.SMTPPort == 0 {
		config.SMTPPort = 587
	}
	if config.Username == "" {
		config.Username = os.Getenv("SMTP_USERNAME")
	}
	if config.Password == "" {
		config.Password = os.Getenv("SMTP_PASSWORD")
	}
	if config.From == "" {
		config.From = os.Getenv("SMTP_FROM")
	}

	return &EmailChannel{
		config: config,
	}
}

// Name returns the channel name
func (c *EmailChannel) Name() string {
	return "email"
}

// Send sends a notification via email
func (c *EmailChannel) Send(ctx context.Context, alert *alertmanager.Alert) error {
	if c.config.SMTPHost == "" {
		return fmt.Errorf("SMTP host not configured")
	}
	if len(c.config.To) == 0 {
		return fmt.Errorf("no recipients configured")
	}

	// Build email
	subject := fmt.Sprintf("[%s] K8sWatch Alert: %s/%s - %s",
		alert.Severity,
		alert.Target.Namespace,
		alert.Target.Name,
		alert.FailureCode,
	)

	body, err := c.buildEmailBody(alert)
	if err != nil {
		return fmt.Errorf("failed to build email body: %w", err)
	}

	// Build message
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		c.config.From,
		c.config.To[0],
		subject,
		body,
	)

	// Connect to SMTP server
	addr := fmt.Sprintf("%s:%d", c.config.SMTPHost, c.config.SMTPPort)

	if c.config.UseTLS {
		return c.sendWithTLS(addr, subject, msg)
	}

	return c.sendWithPlain(addr, subject, msg)
}

// Close closes the channel
func (c *EmailChannel) Close() error {
	return nil
}

// sendWithTLS sends email with TLS
func (c *EmailChannel) sendWithTLS(addr, subject, msg string) error {
	auth := smtp.PlainAuth("", c.config.Username, c.config.Password, c.config.SMTPHost)
	return smtp.SendMail(addr, auth, c.config.From, c.config.To, []byte(msg))
}

// sendWithPlain sends email without TLS
func (c *EmailChannel) sendWithPlain(addr, subject, msg string) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, c.config.SMTPHost)
	if err != nil {
		return err
	}
	defer client.Close()

	if c.config.Username != "" {
		auth := smtp.PlainAuth("", c.config.Username, c.config.Password, c.config.SMTPHost)
		if err := client.Auth(auth); err != nil {
			return err
		}
	}

	if err := client.Mail(c.config.From); err != nil {
		return err
	}

	for _, to := range c.config.To {
		if err := client.Rcpt(to); err != nil {
			return err
		}
	}

	w, err := client.Data()
	if err != nil {
		return err
	}

	_, err = w.Write([]byte(msg))
	if err != nil {
		return err
	}

	err = w.Close()
	if err != nil {
		return err
	}

	return client.Quit()
}

// buildEmailBody builds the HTML email body
func (c *EmailChannel) buildEmailBody(alert *alertmanager.Alert) (string, error) {
	tmpl := `
<!DOCTYPE html>
<html>
<head>
<style>
	body { font-family: Arial, sans-serif; }
	.alert { border: 1px solid #ddd; padding: 20px; margin: 10px 0; }
	.critical { border-left: 4px solid #dc3545; }
	.warning { border-left: 4px solid #ffc107; }
	.info { border-left: 4px solid #17a2b8; }
	table { border-collapse: collapse; width: 100%; }
	td, th { padding: 8px; text-align: left; border-bottom: 1px solid #ddd; }
	.label { font-weight: bold; width: 200px; }
</style>
</head>
<body>
<div class="alert {{.SeverityClass}}">
<h2>{{.Emoji}} K8sWatch Alert: {{.Status}}</h2>
<table>
<tr><td class="label">Target:</td><td>{{.TargetNamespace}}/{{.TargetName}}</td></tr>
<tr><td class="label">Type:</td><td>{{.TargetType}}</td></tr>
<tr><td class="label">Severity:</td><td>{{.Severity}}</td></tr>
<tr><td class="label">Failure:</td><td>{{.FailureLayer}} ({{.FailureCode}})</td></tr>
<tr><td class="label">Blast Radius:</td><td>{{.BlastRadius}}</td></tr>
<tr><td class="label">Affected Nodes:</td><td>{{.AffectedNodesCount}}</td></tr>
<tr><td class="label">Consecutive Failures:</td><td>{{.ConsecutiveFailures}}</td></tr>
<tr><td class="label">Fired At:</td><td>{{.FiredAt}}</td></tr>
</table>
{{if .Runbook}}
<p><strong>Runbook:</strong> <a href="{{.Runbook}}">View Runbook</a></p>
{{end}}
<p style="color: #666; font-size: 12px; margin-top: 20px;">
This alert was generated by K8sWatch.
</p>
</div>
</body>
</html>
`

	type emailData struct {
		SeverityClass       string
		Emoji               string
		Status              string
		TargetNamespace     string
		TargetName          string
		TargetType          string
		Severity            string
		FailureLayer        string
		FailureCode         string
		BlastRadius         string
		AffectedNodesCount  int
		ConsecutiveFailures int32
		FiredAt             string
		Runbook             string
	}

	severityClass := "warning"
	emoji := "⚠️"
	switch alert.Severity {
	case alertmanager.AlertSeverityCritical:
		severityClass = "critical"
		emoji = "🚨"
	case alertmanager.AlertSeverityInfo:
		severityClass = "info"
		emoji = "ℹ️"
	}

	runbook := ""
	if alert.Annotations != nil {
		runbook = alert.Annotations["runbook"]
	}

	data := emailData{
		SeverityClass:       severityClass,
		Emoji:               emoji,
		Status:              string(alert.Status),
		TargetNamespace:     alert.Target.Namespace,
		TargetName:          alert.Target.Name,
		TargetType:          string(alert.Target.Type),
		Severity:            string(alert.Severity),
		FailureLayer:        alert.FailureLayer,
		FailureCode:         alert.FailureCode,
		BlastRadius:         alert.BlastRadius,
		AffectedNodesCount:  len(alert.AffectedNodes),
		ConsecutiveFailures: alert.ConsecutiveFailures,
		FiredAt:             alert.FiredAt.Format(time.RFC3339),
		Runbook:             runbook,
	}

	t, err := template.New("email").Parse(tmpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
