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

package agent

import (
	"context"
	"os"

	"github.com/go-logr/logr"
	"github.com/k8swatch/k8s-monitor/internal/logging"
)

// log is the package-level logger
var log = logr.Discard()

// contextLogger provides correlation ID support
var contextLogger *logging.ContextLogger

// SetLogger sets the logger for the agent package
func SetLogger(logger logr.Logger) {
	log = logger
	contextLogger = logging.NewContextLogger(logger)
}

// GetContextLogger returns the context logger for correlation ID support
func GetContextLogger() *logging.ContextLogger {
	return contextLogger
}

// getNamespace gets the current namespace from service account or environment
func getNamespace() string {
	// Try to read from service account
	if data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		return string(data)
	}

	// Fall back to environment variable
	if ns := os.Getenv("POD_NAMESPACE"); ns != "" {
		return ns
	}

	// Default
	return "k8swatch"
}

// newCheckContext creates a new context for a check operation with correlation ID
func newCheckContext(ctx context.Context, targetName, namespace, targetType string) (context.Context, *logging.OperationLogger) {
	if contextLogger != nil {
		return contextLogger.StartOperation(
			ctx,
			"check.execute",
			"target", targetName,
			"namespace", namespace,
			"type", targetType,
		)
	}
	// Fallback for tests where logger is not initialized
	return ctx, nil
}
