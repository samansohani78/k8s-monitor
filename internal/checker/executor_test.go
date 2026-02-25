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
	"errors"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	k8swatchv1 "github.com/k8swatch/k8s-monitor/api/v1"
)

// mockLayer is a mock layer for testing
type mockLayer struct {
	name           string
	enabled        bool
	checkFunc      func(ctx context.Context, target *k8swatchv1.Target) (*LayerResult, error)
	checkCallCount int
}

func (m *mockLayer) Name() string {
	return m.name
}

func (m *mockLayer) Enabled(target *k8swatchv1.Target) bool {
	return m.enabled
}

func (m *mockLayer) Check(ctx context.Context, target *k8swatchv1.Target) (*LayerResult, error) {
	m.checkCallCount++
	if m.checkFunc != nil {
		return m.checkFunc(ctx, target)
	}
	return LayerResultSuccess(10), nil
}

func TestExecutorCreation(t *testing.T) {
	layers := []Layer{
		&mockLayer{name: "L0", enabled: true},
		&mockLayer{name: "L1", enabled: true},
	}

	executor := NewExecutor(layers)

	if executor == nil {
		t.Fatal("Expected executor to be created")
	}

	if len(executor.layers) != 2 {
		t.Errorf("Expected 2 layers, got %d", len(executor.layers))
	}
}

func TestExecutorExecuteSuccess(t *testing.T) {
	layers := []Layer{
		&mockLayer{name: "L0", enabled: true, checkFunc: func(ctx context.Context, target *k8swatchv1.Target) (*LayerResult, error) {
			return LayerResultSuccess(5), nil
		}},
		&mockLayer{name: "L1", enabled: true, checkFunc: func(ctx context.Context, target *k8swatchv1.Target) (*LayerResult, error) {
			return LayerResultSuccess(10), nil
		}},
		&mockLayer{name: "L2", enabled: true, checkFunc: func(ctx context.Context, target *k8swatchv1.Target) (*LayerResult, error) {
			return LayerResultSuccess(15), nil
		}},
	}

	executor := NewExecutor(layers)

	target := &k8swatchv1.Target{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-target",
			Namespace: "default",
		},
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeHTTP,
		},
	}

	ctx := context.Background()
	result, err := executor.Execute(ctx, target)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !result.Check.Success {
		t.Errorf("Expected success, got failure")
	}

	if result.Check.FailureLayer != "" {
		t.Errorf("Expected no failure layer, got %s", result.Check.FailureLayer)
	}

	if result.Check.FinalLayer != "L2" {
		t.Errorf("Expected final layer L2, got %s", result.Check.FinalLayer)
	}

	if len(result.Check.LayersEnabled) != 3 {
		t.Errorf("Expected 3 layers enabled, got %d", len(result.Check.LayersEnabled))
	}

	if result.Metadata.CheckDurationMs < 0 {
		t.Errorf("Expected non-negative duration, got %d", result.Metadata.CheckDurationMs)
	}
}

func TestExecutorExecuteFailFast(t *testing.T) {
	layers := []Layer{
		&mockLayer{name: "L0", enabled: true, checkFunc: func(ctx context.Context, target *k8swatchv1.Target) (*LayerResult, error) {
			return LayerResultSuccess(5), nil
		}},
		&mockLayer{name: "L1", enabled: true, checkFunc: func(ctx context.Context, target *k8swatchv1.Target) (*LayerResult, error) {
			return LayerResultFailure(string(k8swatchv1.FailureCodeDNSTimeout), "DNS timeout", 100), nil
		}},
		&mockLayer{name: "L2", enabled: true, checkFunc: func(ctx context.Context, target *k8swatchv1.Target) (*LayerResult, error) {
			// This should not be called due to fail-fast
			return LayerResultSuccess(15), nil
		}},
	}

	executor := NewExecutor(layers)

	target := &k8swatchv1.Target{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-target",
			Namespace: "default",
		},
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeHTTP,
		},
	}

	ctx := context.Background()
	result, err := executor.Execute(ctx, target)

	if err != nil {
		t.Fatalf("Expected no error from executor, got %v", err)
	}

	if result.Check.Success {
		t.Errorf("Expected failure, got success")
	}

	if result.Check.FailureLayer != "L1" {
		t.Errorf("Expected failure layer L1, got %s", result.Check.FailureLayer)
	}

	if result.Check.FailureCode != string(k8swatchv1.FailureCodeDNSTimeout) {
		t.Errorf("Expected failure code %s, got %s", k8swatchv1.FailureCodeDNSTimeout, result.Check.FailureCode)
	}

	if result.Check.FinalLayer != "L1" {
		t.Errorf("Expected final layer L1, got %s", result.Check.FinalLayer)
	}

	// Verify L2 was not called (fail-fast)
	l2 := layers[2].(*mockLayer)
	if l2.checkCallCount != 0 {
		t.Errorf("Expected L2 not to be called (fail-fast), but it was called %d times", l2.checkCallCount)
	}
}

func TestExecutorExecuteWithError(t *testing.T) {
	testError := errors.New("connection timeout")

	layers := []Layer{
		&mockLayer{name: "L0", enabled: true, checkFunc: func(ctx context.Context, target *k8swatchv1.Target) (*LayerResult, error) {
			return LayerResultSuccess(5), nil
		}},
		&mockLayer{name: "L1", enabled: true, checkFunc: func(ctx context.Context, target *k8swatchv1.Target) (*LayerResult, error) {
			return LayerResultError(testError, string(k8swatchv1.FailureCodeTimeout), 100), nil
		}},
	}

	executor := NewExecutor(layers)

	target := &k8swatchv1.Target{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-target",
			Namespace: "default",
		},
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeHTTP,
		},
	}

	ctx := context.Background()
	result, err := executor.Execute(ctx, target)

	if err != nil {
		t.Fatalf("Expected no error from executor, got %v", err)
	}

	if result.Check.Success {
		t.Errorf("Expected failure, got success")
	}

	if result.Check.FailureCode != string(k8swatchv1.FailureCodeTimeout) {
		t.Errorf("Expected failure code %s, got %s", k8swatchv1.FailureCodeTimeout, result.Check.FailureCode)
	}
}

func TestExecutorSkipsDisabledLayers(t *testing.T) {
	layers := []Layer{
		&mockLayer{name: "L0", enabled: true, checkFunc: func(ctx context.Context, target *k8swatchv1.Target) (*LayerResult, error) {
			return LayerResultSuccess(5), nil
		}},
		&mockLayer{name: "L1", enabled: false}, // Disabled
		&mockLayer{name: "L2", enabled: true, checkFunc: func(ctx context.Context, target *k8swatchv1.Target) (*LayerResult, error) {
			return LayerResultSuccess(15), nil
		}},
	}

	executor := NewExecutor(layers)

	target := &k8swatchv1.Target{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-target",
			Namespace: "default",
		},
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeHTTP,
		},
	}

	ctx := context.Background()
	result, err := executor.Execute(ctx, target)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !result.Check.Success {
		t.Errorf("Expected success, got failure")
	}

	// L1 should not be in enabled layers
	for _, layer := range result.Check.LayersEnabled {
		if layer == "L1" {
			t.Errorf("Expected L1 to be skipped (disabled), but it was executed")
		}
	}
}

func TestLayerResultHelpers(t *testing.T) {
	// Test LayerResultSuccess
	result := LayerResultSuccess(100)
	if !result.Success {
		t.Errorf("Expected success")
	}
	if result.DurationMs != 100 {
		t.Errorf("Expected duration 100, got %d", result.DurationMs)
	}
	if result.FailureCode != "" {
		t.Errorf("Expected no failure code")
	}

	// Test LayerResultFailure
	result = LayerResultFailure(string(k8swatchv1.FailureCodeTCPRefused), "Connection refused", 50)
	if result.Success {
		t.Errorf("Expected failure")
	}
	if result.FailureCode != string(k8swatchv1.FailureCodeTCPRefused) {
		t.Errorf("Expected failure code %s, got %s", k8swatchv1.FailureCodeTCPRefused, result.FailureCode)
	}

	// Test LayerResultError
	err := errors.New("timeout")
	result = LayerResultError(err, string(k8swatchv1.FailureCodeTimeout), 75)
	if result.Success {
		t.Errorf("Expected failure")
	}
	if result.FailureMessage != "timeout" {
		t.Errorf("Expected failure message 'timeout', got %s", result.FailureMessage)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{"microseconds", 500 * time.Microsecond, "500µs"},
		{"milliseconds", 50 * time.Millisecond, "50ms"},
		{"seconds", 2 * time.Second, "2.00s"},
		{"fractional seconds", 2500 * time.Millisecond, "2.50s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatDuration(tt.duration)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}
