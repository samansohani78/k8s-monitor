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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	k8swatchv1 "github.com/k8swatch/k8s-monitor/api/v1"
)

// testMockLayer is a mock layer for testing
type testMockLayer struct {
	name       string
	shouldPass bool
	enabled    bool
	checkDelay time.Duration
}

func (m *testMockLayer) Name() string {
	return m.name
}

func (m *testMockLayer) Check(ctx context.Context, target *k8swatchv1.Target) (*LayerResult, error) {
	if m.checkDelay > 0 {
		select {
		case <-time.After(m.checkDelay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if m.shouldPass {
		return LayerResultSuccess(10), nil
	}
	return LayerResultFailure("test_failure", "test failed", 10), nil
}

func (m *testMockLayer) Enabled(target *k8swatchv1.Target) bool {
	return m.enabled
}

// mockCheckerFactory is a mock factory for testing
type mockCheckerFactory struct {
	types []string
}

func (f *mockCheckerFactory) Create(target *k8swatchv1.Target) (Checker, error) {
	return &mockChecker{layers: []Layer{}}, nil
}

func (f *mockCheckerFactory) SupportedTypes() []string {
	return f.types
}

// mockChecker is a mock checker for testing
type mockChecker struct {
	layers []Layer
}

func (m *mockChecker) Execute(ctx context.Context, target *k8swatchv1.Target) (*k8swatchv1.CheckResult, error) {
	executor := NewExecutor(m.layers)
	return executor.Execute(ctx, target)
}

func (m *mockChecker) Layers() []Layer {
	return m.layers
}

func TestRegistryCreation(t *testing.T) {
	reg := NewRegistry()
	assert.NotNil(t, reg)
	assert.Empty(t, reg.factories)
}

func TestRegistryRegister(t *testing.T) {
	reg := NewRegistry()
	factory := &mockCheckerFactory{types: []string{"test"}}

	reg.Register(factory, "test")

	assert.Len(t, reg.factories, 1)
	_, ok := reg.factories["test"]
	assert.True(t, ok)
}

func TestRegistryRegisterMultipleTypes(t *testing.T) {
	reg := NewRegistry()
	factory := &mockCheckerFactory{types: []string{"type1", "type2", "type3"}}

	reg.Register(factory, "type1", "type2", "type3")

	assert.Len(t, reg.factories, 3)
}

func TestRegistryGet(t *testing.T) {
	reg := NewRegistry()
	factory := &mockCheckerFactory{types: []string{"test"}}

	reg.Register(factory, "test")

	retrieved, err := reg.Get("test")
	require.NoError(t, err)
	assert.Equal(t, factory, retrieved)
}

func TestRegistryGetNotFound(t *testing.T) {
	reg := NewRegistry()

	_, err := reg.Get("nonexistent")
	assert.Error(t, err)
	assert.IsType(t, ErrUnsupportedTargetType{}, err)
}

func TestRegistryCreate(t *testing.T) {
	reg := NewRegistry()
	factory := &mockCheckerFactory{types: []string{"test"}}

	reg.Register(factory, "test")

	target := &k8swatchv1.Target{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: k8swatchv1.TargetSpec{
			Type: "test",
		},
	}

	checker, err := reg.Create(target)
	require.NoError(t, err)
	assert.NotNil(t, checker)
}

func TestRegistryCreateUnsupportedType(t *testing.T) {
	reg := NewRegistry()

	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: "unsupported",
		},
	}

	_, err := reg.Create(target)
	assert.Error(t, err)
}

func TestRegistrySupportedTypes(t *testing.T) {
	reg := NewRegistry()
	factory := &mockCheckerFactory{types: []string{"type1", "type2"}}

	reg.Register(factory, "type1", "type2")

	types := reg.SupportedTypes()
	assert.Len(t, types, 2)
	assert.Contains(t, types, "type1")
	assert.Contains(t, types, "type2")
}

func TestBaseCheckerCreation(t *testing.T) {
	layers := []Layer{&testMockLayer{name: "L0", enabled: true}}
	checker := NewBaseChecker("test", layers)

	assert.NotNil(t, checker)
	assert.Equal(t, layers, checker.Layers())
}

func TestBaseCheckerExecute(t *testing.T) {
	layers := []Layer{
		&testMockLayer{name: "L0", enabled: true, shouldPass: true},
		&testMockLayer{name: "L1", enabled: true, shouldPass: true},
	}
	checker := NewBaseChecker("test", layers)

	target := &k8swatchv1.Target{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: k8swatchv1.TargetSpec{
			Type: "test",
		},
	}

	ctx := context.Background()
	result, err := checker.Execute(ctx, target)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Check.Success)
	assert.Equal(t, "L1", result.Check.FinalLayer)
}

func TestErrUnsupportedTargetType(t *testing.T) {
	err := ErrUnsupportedTargetType{TargetType: "test"}
	assert.Equal(t, "unsupported target type: test", err.Error())
}

// TestExecutorWithMockLayers tests the executor with mock layers
func TestExecutorWithMockLayers(t *testing.T) {
	layers := []Layer{
		&testMockLayer{name: "L0", enabled: true, shouldPass: true},
		&testMockLayer{name: "L1", enabled: true, shouldPass: true},
		&testMockLayer{name: "L2", enabled: true, shouldPass: false},
	}

	executor := NewExecutor(layers)

	target := &k8swatchv1.Target{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: k8swatchv1.TargetSpec{
			Type: "test",
		},
	}

	ctx := context.Background()
	result, err := executor.Execute(ctx, target)

	require.NoError(t, err)
	assert.False(t, result.Check.Success)
	assert.Equal(t, "L2", result.Check.FailureLayer)
	assert.Equal(t, "test_failure", result.Check.FailureCode)
}

// TestExecutorLatencyRecording tests that latencies are recorded correctly
func TestExecutorLatencyRecording(t *testing.T) {
	layers := []Layer{
		&testMockLayer{name: "L0", enabled: true, shouldPass: true, checkDelay: 10 * time.Millisecond},
		&testMockLayer{name: "L1", enabled: true, shouldPass: true, checkDelay: 10 * time.Millisecond},
	}

	executor := NewExecutor(layers)

	target := &k8swatchv1.Target{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: k8swatchv1.TargetSpec{
			Type: "test",
		},
	}

	ctx := context.Background()
	result, err := executor.Execute(ctx, target)

	require.NoError(t, err)
	assert.True(t, result.Check.Success)
	assert.Contains(t, result.Latencies, "L0")
	assert.Contains(t, result.Latencies, "L1")
	assert.GreaterOrEqual(t, result.Latencies["L0"].DurationMs, int64(0))
	assert.GreaterOrEqual(t, result.Latencies["L1"].DurationMs, int64(0))
}

// TestExecutorSkipsDisabledLayers tests that disabled layers are skipped
func TestExecutorSkipsDisabledLayersInterface(t *testing.T) {
	layers := []Layer{
		&testMockLayer{name: "L0", enabled: true, shouldPass: true},
		&testMockLayer{name: "L1", enabled: false, shouldPass: false},
		&testMockLayer{name: "L2", enabled: true, shouldPass: true},
	}

	executor := NewExecutor(layers)

	target := &k8swatchv1.Target{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: k8swatchv1.TargetSpec{
			Type: "test",
		},
	}

	ctx := context.Background()
	result, err := executor.Execute(ctx, target)

	require.NoError(t, err)
	assert.True(t, result.Check.Success)
	assert.Equal(t, "L2", result.Check.FinalLayer)
	assert.NotContains(t, result.Latencies, "L1")
}

// TestContains tests the contains helper function
func TestContains(t *testing.T) {
	tests := []struct {
		s        string
		substr   string
		expected bool
	}{
		{"hello world", "world", true},
		{"hello world", "hello", true},
		{"hello world", "foo", false},
		{"", "", true},
		{"abc", "", true},
	}

	for _, tt := range tests {
		result := contains(tt.s, tt.substr)
		assert.Equal(t, tt.expected, result)
	}
}
