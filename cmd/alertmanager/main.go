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

// Package main is the entry point for the K8sWatch alertmanager.
// The alertmanager manages alert lifecycle and notifications.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-logr/zapr"
	"go.uber.org/zap"

	"github.com/k8swatch/k8s-monitor/internal/alertmanager"
	"github.com/k8swatch/k8s-monitor/internal/alertmanager/channels"
)

var (
	Version   = "0.1.0"
	BuildDate = "unknown"
	GitCommit = "unknown"
)

func main() {
	// Command-line flags
	var (
		kubeconfig   string
		verbose      bool
		showVersion  bool
		httpPort     int
		slackWebhook string
		pagerDutyKey string
		webhookURL   string
		smtpHost     string
		smtpPort     int
		smtpUsername string
		smtpPassword string
		smtpFrom     string
		smtpTo       string
	)

	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file")
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose logging")
	flag.BoolVar(&showVersion, "version", false, "Show version information")
	flag.IntVar(&httpPort, "http-port", 8080, "HTTP API server port")
	flag.StringVar(&slackWebhook, "slack-webhook", "", "Slack webhook URL")
	flag.StringVar(&pagerDutyKey, "pagerduty-key", "", "PagerDuty routing key")
	flag.StringVar(&webhookURL, "webhook-url", "", "Generic webhook URL")
	flag.StringVar(&smtpHost, "smtp-host", "", "SMTP server host")
	flag.IntVar(&smtpPort, "smtp-port", 587, "SMTP server port")
	flag.StringVar(&smtpUsername, "smtp-username", "", "SMTP username")
	flag.StringVar(&smtpPassword, "smtp-password", "", "SMTP password")
	flag.StringVar(&smtpFrom, "smtp-from", "", "SMTP from address")
	flag.StringVar(&smtpTo, "smtp-to", "", "SMTP to address (comma-separated)")
	flag.Parse()

	if showVersion {
		fmt.Printf("K8sWatch AlertManager\n")
		fmt.Printf("Version: %s\n", Version)
		fmt.Printf("Build Date: %s\n", BuildDate)
		fmt.Printf("Git Commit: %s\n", GitCommit)
		os.Exit(0)
	}

	// Set up logger
	zapConfig := zap.NewProductionConfig()
	if verbose {
		zapConfig.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	}
	zapLogger, err := zapConfig.Build()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	ctrlLog := zapr.NewLogger(zapLogger)
	alertmanager.SetLogger(ctrlLog)

	// Create alert manager components
	store := alertmanager.NewMemoryStore()
	router := alertmanager.NewRouter(alertmanager.DefaultRouterConfig())

	// Register notification channels
	if slackWebhook != "" || os.Getenv("SLACK_WEBHOOK_URL") != "" {
		slackChannel := channels.NewSlackChannel(&channels.SlackConfig{
			WebhookURL: slackWebhook,
		})
		router.RegisterChannel(slackChannel)
		fmt.Println("Slack channel registered")
	}

	if pagerDutyKey != "" || os.Getenv("PAGERDUTY_ROUTING_KEY") != "" {
		pagerDutyChannel := channels.NewPagerDutyChannel(&channels.PagerDutyConfig{
			RoutingKey: pagerDutyKey,
		})
		router.RegisterChannel(pagerDutyChannel)
		fmt.Println("PagerDuty channel registered")
	}

	if webhookURL != "" || os.Getenv("WEBHOOK_URL") != "" {
		webhookChannel := channels.NewWebhookChannel(&channels.WebhookConfig{
			URL: webhookURL,
		})
		router.RegisterChannel(webhookChannel)
		fmt.Println("Webhook channel registered")
	}

	if smtpHost != "" || os.Getenv("SMTP_HOST") != "" {
		to := []string{smtpTo}
		if smtpTo == "" {
			to = nil
		}
		emailChannel := channels.NewEmailChannel(&channels.EmailConfig{
			SMTPHost: smtpHost,
			SMTPPort: smtpPort,
			Username: smtpUsername,
			Password: smtpPassword,
			From:     smtpFrom,
			To:       to,
			UseTLS:   true,
		})
		router.RegisterChannel(emailChannel)
		fmt.Println("Email channel registered")
	}

	// Create escalation policies
	escalator := router.GetEscalator()
	if escalator != nil {
		escalator.CreateDefaultPolicies()
	}

	// Create manager
	manager := alertmanager.NewManager(
		alertmanager.DefaultManagerConfig(),
		store,
		router,
	)

	// Create API
	api := alertmanager.NewAPI(manager)

	// Set up signal handler
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		fmt.Printf("\nReceived signal %v, shutting down...\n", sig)
		cancel()
	}()

	// Start manager
	if err := manager.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start manager: %v\n", err)
		os.Exit(1)
	}

	// Start API server in goroutine
	go func() {
		addr := fmt.Sprintf(":%d", httpPort)
		fmt.Printf("Starting AlertManager API server on %s\n", addr)
		if err := api.Start(addr); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "API server failed: %v\n", err)
		}
	}()

	// Print startup info
	fmt.Printf("Starting K8sWatch AlertManager %s...\n", Version)
	fmt.Printf("HTTP Port: %d\n", httpPort)
	fmt.Printf("Verbose: %v\n", verbose)

	// Wait for shutdown
	<-ctx.Done()

	// Graceful shutdown
	fmt.Println("Shutting down AlertManager...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := manager.Stop(); err != nil {
		fmt.Fprintf(os.Stderr, "Manager shutdown error: %v\n", err)
	}

	// Shutdown API server
	apiServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", httpPort),
		ReadHeaderTimeout: 10 * time.Second,
	}
	if err := apiServer.Shutdown(shutdownCtx); err != nil {
		fmt.Fprintf(os.Stderr, "API server shutdown error: %v\n", err)
	}

	fmt.Println("AlertManager shutdown complete")
}
