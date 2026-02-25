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
	"testing"
	"time"

	k8swatchv1 "github.com/k8swatch/k8s-monitor/api/v1"
	"github.com/stretchr/testify/assert"
)

func TestExecutor_Execute_WithTimeout(t *testing.T) {
	layers := []Layer{
		&TestLayer{name: "L1", shouldFail: false},
		&TestLayer{name: "L2", shouldFail: false},
	}
	executor := NewExecutor(layers)

	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeNetwork,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := executor.Execute(ctx, target)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Check.Success)
}

func TestExecutor_Execute_WithFailFast(t *testing.T) {
	layers := []Layer{
		&TestLayer{name: "L1", shouldFail: true, failAt: 1},
		&TestLayer{name: "L2", shouldFail: false},
	}
	executor := NewExecutor(layers)

	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeNetwork,
		},
	}

	ctx := context.Background()
	result, err := executor.Execute(ctx, target)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.Check.Success)
	assert.Equal(t, "L1", result.Check.FailureLayer)
}

func TestExecutor_Execute_WithNilLayers(t *testing.T) {
	executor := NewExecutor(nil)

	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeNetwork,
		},
	}

	ctx := context.Background()
	result, err := executor.Execute(ctx, target)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Check.Success)
}

func TestExecutor_Execute_WithContextCancelled(t *testing.T) {
	layers := []Layer{
		&TestLayer{name: "L1", shouldFail: false, delay: 100 * time.Millisecond},
	}
	executor := NewExecutor(layers)

	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeNetwork,
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := executor.Execute(ctx, target)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestExecutor_ExtractFailureCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{"nil", nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := extractFailureCode(tt.err)
			assert.Equal(t, tt.expected, code)
		})
	}
}

func TestExecutor_FormatDuration(t *testing.T) {
	tests := []struct {
		d        time.Duration
		expected string
	}{
		{0, "0µs"},
		{100 * time.Microsecond, "100µs"},
		{500 * time.Microsecond, "500µs"},
		{1 * time.Millisecond, "1ms"},
		{100 * time.Millisecond, "100ms"},
		{1 * time.Second, "1.00s"},
		{1500 * time.Millisecond, "1.50s"},
		{60 * time.Second, "60.00s"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := FormatDuration(tt.d)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExecutor_Contains(t *testing.T) {
	assert.True(t, contains("hello world", "world"))
	assert.False(t, contains("hello world", "foo"))
	assert.True(t, contains("test", "test"))
	assert.False(t, contains("", "test"))
}

func TestExecutor_FindSubstring(t *testing.T) {
	assert.True(t, findSubstring("hello world", "world"))
	assert.False(t, findSubstring("hello world", "foo"))
}

func TestExecutor_LayerResultSuccess(t *testing.T) {
	result := LayerResultSuccess(100)
	assert.True(t, result.Success)
	assert.Equal(t, int64(100), result.DurationMs)
	assert.Empty(t, result.FailureCode)
	assert.Empty(t, result.FailureMessage)
}

func TestExecutor_LayerResultFailure(t *testing.T) {
	result := LayerResultFailure("test_code", "test message", 50)
	assert.False(t, result.Success)
	assert.Equal(t, "test_code", result.FailureCode)
	assert.Equal(t, "test message", result.FailureMessage)
	assert.Equal(t, int64(50), result.DurationMs)
}

func TestExecutor_LayerResultError(t *testing.T) {
	result := LayerResultError(assert.AnError, "test_code", 75)
	assert.False(t, result.Success)
	assert.Equal(t, "test_code", result.FailureCode)
	assert.Equal(t, int64(75), result.DurationMs)
}

func TestLayerConfigBase(t *testing.T) {
	config := k8swatchv1.LayerConfigBase{
		Enabled: true,
		Timeout: "30s",
	}

	assert.True(t, config.Enabled)
	assert.Equal(t, "30s", config.Timeout)
}

// TestLayer is a mock layer for testing
type TestLayer struct {
	name      string
	shouldFail bool
	failAt    int
	callCount int
	delay     time.Duration
}

func (l *TestLayer) Name() string {
	return l.name
}

func (l *TestLayer) Enabled(target *k8swatchv1.Target) bool {
	return true
}

func (l *TestLayer) Check(ctx context.Context, target *k8swatchv1.Target) (*LayerResult, error) {
	l.callCount++

	if l.delay > 0 {
		select {
		case <-time.After(l.delay):
		case <-ctx.Done():
			return LayerResultFailure("context_cancelled", "Context cancelled", 0), nil
		}
	}

	if l.shouldFail && l.callCount >= l.failAt {
		return LayerResultFailure("test_failure", "Test failure", 0), nil
	}

	return LayerResultSuccess(10), nil
}

func TestExecutor_Execute_WithMultipleLayers(t *testing.T) {
	layers := []Layer{
		&TestLayer{name: "L0", shouldFail: false},
		&TestLayer{name: "L1", shouldFail: false},
		&TestLayer{name: "L2", shouldFail: false},
		&TestLayer{name: "L3", shouldFail: false},
	}
	executor := NewExecutor(layers)

	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeNetwork,
		},
	}

	ctx := context.Background()
	result, err := executor.Execute(ctx, target)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Check.Success)
	assert.Equal(t, "L3", result.Check.FinalLayer)
	assert.Len(t, result.Latencies, 4)
}

func TestExecutor_Execute_WithEmptyTarget(t *testing.T) {
	layers := []Layer{
		&TestLayer{name: "L1", shouldFail: false},
	}
	executor := NewExecutor(layers)

	target := &k8swatchv1.Target{}

	ctx := context.Background()
	result, err := executor.Execute(ctx, target)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestExecutor_Execute_LatencyRecording(t *testing.T) {
	layers := []Layer{
		&TestLayer{name: "L1", shouldFail: false, delay: 10 * time.Millisecond},
		&TestLayer{name: "L2", shouldFail: false, delay: 20 * time.Millisecond},
	}
	executor := NewExecutor(layers)

	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeNetwork,
		},
	}

	ctx := context.Background()
	result, err := executor.Execute(ctx, target)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.GreaterOrEqual(t, result.Metadata.CheckDurationMs, int64(30))
}

func TestExecutor_Execute_WithLayerFailure(t *testing.T) {
	layers := []Layer{
		&TestLayer{name: "L1", shouldFail: false},
		&TestLayer{name: "L2", shouldFail: true, failAt: 1},
		&TestLayer{name: "L3", shouldFail: false},
	}
	executor := NewExecutor(layers)

	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeNetwork,
		},
	}

	ctx := context.Background()
	result, err := executor.Execute(ctx, target)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.Check.Success)
	assert.Equal(t, "L2", result.Check.FailureLayer)
	assert.Equal(t, "test_failure", result.Check.FailureCode)
}

func TestExecutor_Execute_ContextTimeout(t *testing.T) {
	layers := []Layer{
		&TestLayer{name: "L1", shouldFail: false, delay: 200 * time.Millisecond},
	}
	executor := NewExecutor(layers)

	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeNetwork,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	result, err := executor.Execute(ctx, target)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestExecutor_Execute_MultipleFailures(t *testing.T) {
	layers := []Layer{
		&TestLayer{name: "L1", shouldFail: true, failAt: 1},
	}
	executor := NewExecutor(layers)

	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeNetwork,
		},
	}

	ctx := context.Background()
	
	// Execute multiple times
	for i := 0; i < 3; i++ {
		result, err := executor.Execute(ctx, target)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.Check.Success)
	}
}

func TestExecutor_Execute_CheckInfo(t *testing.T) {
	layers := []Layer{
		&TestLayer{name: "L1", shouldFail: false},
		&TestLayer{name: "L2", shouldFail: false},
	}
	executor := NewExecutor(layers)

	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeNetwork,
		},
	}

	ctx := context.Background()
	result, err := executor.Execute(ctx, target)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.Check)
	assert.True(t, result.Check.Success)
	assert.Equal(t, "L2", result.Check.FinalLayer)
	assert.Empty(t, result.Check.FailureLayer)
	assert.Empty(t, result.Check.FailureCode)
}
