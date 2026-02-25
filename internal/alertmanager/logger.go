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

package alertmanager

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/k8swatch/k8s-monitor/internal/logging"
)

var log logr.Logger = logr.Discard()

// contextLogger provides correlation ID support
var contextLogger *logging.ContextLogger

// SetLogger sets the logger for the alertmanager package
func SetLogger(logger logr.Logger) {
	log = logger.WithName("alertmanager")
	contextLogger = logging.NewContextLogger(log)
}

// GetLogger returns the logger for the alertmanager package
func GetLogger() logr.Logger {
	return log
}

// GetContextLogger returns the context logger for correlation ID support
func GetContextLogger() *logging.ContextLogger {
	return contextLogger
}

// newAlertContext creates a new context for alert processing
// nolint:unused
func newAlertContext(ctx context.Context, alertID, severity string) (context.Context, *logging.OperationLogger) {
	return contextLogger.StartOperation(
		ctx,
		"alert.process",
		"alertID", alertID,
		"severity", severity,
	)
}

// newNotificationContext creates a new context for notification sending
// nolint:unused
func newNotificationContext(ctx context.Context, alertID, channel string) (context.Context, *logging.OperationLogger) {
	return contextLogger.StartOperation(
		ctx,
		"notification.send",
		"alertID", alertID,
		"channel", channel,
	)
}
