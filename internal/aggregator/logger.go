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

package aggregator

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/k8swatch/k8s-monitor/internal/logging"
)

var log logr.Logger = logr.Discard()

// contextLogger provides correlation ID support
var contextLogger *logging.ContextLogger

func SetLogger(logger logr.Logger) {
	log = logger
	contextLogger = logging.NewContextLogger(logger)
}

// GetContextLogger returns the context logger for correlation ID support
func GetContextLogger() *logging.ContextLogger {
	return contextLogger
}

// newProcessContext creates a new context for processing a result
// nolint:unused
func newProcessContext(ctx context.Context, targetKey string) (context.Context, *logging.OperationLogger) {
	return contextLogger.StartOperation(
		ctx,
		"result.process",
		"targetKey", targetKey,
	)
}

// newCorrelationContext creates a new context for correlation analysis
// nolint:unused
func newCorrelationContext(ctx context.Context, targetKey, pattern string) (context.Context, *logging.OperationLogger) {
	return contextLogger.StartOperation(
		ctx,
		"correlation.analyze",
		"targetKey", targetKey,
		"pattern", pattern,
	)
}
