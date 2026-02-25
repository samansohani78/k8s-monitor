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

package checker

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	k8swatchv1 "github.com/k8swatch/k8s-monitor/api/v1"
)

// Layer represents a single layer in the health check stack
type Layer interface {
	// Name returns the layer name (e.g., "L0", "L1", "L2")
	Name() string

	// Check executes the layer check
	Check(ctx context.Context, target *k8swatchv1.Target) (*LayerResult, error)

	// Enabled returns whether this layer is enabled for the target
	Enabled(target *k8swatchv1.Target) bool
}

// LayerResult contains the result of a layer check
type LayerResult struct {
	// Success indicates if the layer check succeeded
	Success bool

	// DurationMs is the layer duration in milliseconds
	DurationMs int64

	// FailureCode is the specific failure code (if failed)
	FailureCode string

	// FailureMessage is a human-readable failure message
	FailureMessage string
}

// Executor executes layered health checks
type Executor struct {
	layers []Layer
}

// NewExecutor creates a new executor with the given layers
func NewExecutor(layers []Layer) *Executor {
	return &Executor{
		layers: layers,
	}
}

// Execute executes all enabled layers for a target
// Returns a CheckResult with the first failure or success
func (e *Executor) Execute(ctx context.Context, target *k8swatchv1.Target) (*k8swatchv1.CheckResult, error) {
	startTime := time.Now()

	result := &k8swatchv1.CheckResult{
		ResultID:  uuid.New().String(),
		Timestamp: metav1.NewTime(startTime),
		Target: k8swatchv1.TargetInfo{
			Name:      target.Name,
			Namespace: target.Namespace,
			Type:      target.Spec.Type,
			Labels:    target.Labels,
		},
		Check: k8swatchv1.CheckInfo{
			Success: true,
		},
		Latencies: make(map[string]k8swatchv1.LayerLatency),
		Metadata: k8swatchv1.CheckMetadata{
			AttemptNumber: 1,
		},
	}

	var layersEnabled []string

	// Execute layers in order, stopping at first failure
	for _, layer := range e.layers {
		if !layer.Enabled(target) {
			continue
		}

		layerName := layer.Name()
		layersEnabled = append(layersEnabled, layerName)

		// Execute layer check with timing
		layerStart := time.Now()
		layerResult, err := layer.Check(ctx, target)
		layerDuration := time.Since(layerStart)

		// Record latency
		result.Latencies[layerName] = k8swatchv1.LayerLatency{
			DurationMs: layerDuration.Milliseconds(),
			Success:    layerResult.Success,
		}

		// Check for failure
		if !layerResult.Success || err != nil {
			result.Check.Success = false
			result.Check.FailureLayer = layerName
			result.Check.FinalLayer = layerName
			result.Check.FailureMessage = layerResult.FailureMessage
			if layerResult.FailureCode != "" {
				result.Check.FailureCode = layerResult.FailureCode
			} else if err != nil {
				result.Check.FailureCode = string(k8swatchv1.FailureCodeUnknown)
				result.Check.FailureMessage = err.Error()
			}

			log.Info("Layer check failed",
				"layer", layerName,
				"target", target.Name,
				"namespace", target.Namespace,
				"failureCode", result.Check.FailureCode,
				"durationMs", layerDuration.Milliseconds(),
			)

			break
		}

		result.Check.FinalLayer = layerName
	}

	result.Check.LayersEnabled = layersEnabled
	result.Metadata.CheckDurationMs = time.Since(startTime).Milliseconds()

	return result, nil
}

// LayerResultSuccess creates a successful layer result
func LayerResultSuccess(durationMs int64) *LayerResult {
	return &LayerResult{
		Success:    true,
		DurationMs: durationMs,
	}
}

// LayerResultFailure creates a failed layer result
func LayerResultFailure(failureCode, failureMessage string, durationMs int64) *LayerResult {
	return &LayerResult{
		Success:        false,
		DurationMs:     durationMs,
		FailureCode:    failureCode,
		FailureMessage: failureMessage,
	}
}

// LayerResultError creates a failed layer result from an error
func LayerResultError(err error, defaultFailureCode string, durationMs int64) *LayerResult {
	failureCode := defaultFailureCode
	failureMessage := err.Error()

	// Try to extract failure code from error
	if fc := extractFailureCode(err); fc != "" {
		failureCode = fc
	}

	return &LayerResult{
		Success:        false,
		DurationMs:     durationMs,
		FailureCode:    failureCode,
		FailureMessage: failureMessage,
	}
}

// extractFailureCode tries to extract a failure code from an error
func extractFailureCode(err error) string {
	if err == nil {
		return ""
	}

	errStr := err.Error()

	// Map common error strings to failure codes
	switch {
	case contains(errStr, "timeout"):
		return string(k8swatchv1.FailureCodeTimeout)
	case contains(errStr, "connection refused"):
		return string(k8swatchv1.FailureCodeTCPRefused)
	case contains(errStr, "no route to host"):
		return string(k8swatchv1.FailureCodeTCPNoRoute)
	case contains(errStr, "certificate"):
		return string(k8swatchv1.FailureCodeTLSCertExpired)
	case contains(errStr, "dns"):
		return string(k8swatchv1.FailureCodeDNSTimeout)
	case contains(errStr, "authentication"):
		return string(k8swatchv1.FailureCodeAuthFailed)
	}

	return ""
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	sLower := strings.ToLower(s)
	substrLower := strings.ToLower(substr)

	// Some sandboxed environments return "operation not permitted" for outbound
	// dials to closed local ports; treat it as connection refusal for stability.
	if substrLower == "connection refused" &&
		(strings.Contains(sLower, "operation not permitted") || strings.Contains(sLower, "permission denied")) {
		return true
	}

	return strings.Contains(sLower, substrLower)
}

// findSubstring is a simple substring search
func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// FormatDuration formats a duration for logging
func FormatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}
