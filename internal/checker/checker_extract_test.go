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
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	k8swatchv1 "github.com/k8swatch/k8s-monitor/api/v1"
)

// Test extractFailureCode comprehensively
func TestExtractFailureCode_Comprehensive(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{"nil error", nil, ""},
		{"timeout", errors.New("context deadline exceeded: timeout"), "timeout"},
		{"connection refused", errors.New("connection refused"), "tcp_refused"},
		{"no route to host", errors.New("no route to host"), "tcp_no_route"},
		{"certificate error", errors.New("x509: certificate has expired"), "tls_expired"},
		{"dns error", errors.New("dns lookup failed"), "dns_timeout"},
		{"authentication error", errors.New("authentication failed"), "auth_failed"},
		{"generic error", errors.New("some error"), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractFailureCode(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test HTTP checker factory
func TestHTTPCheckerFactory(t *testing.T) {
	t.Run("SupportedTypes", func(t *testing.T) {
		factory := HTTPCheckerFactory{}
		types := factory.SupportedTypes()
		assert.Contains(t, types, "http")
		assert.Contains(t, types, "https")
	})

	t.Run("Create", func(t *testing.T) {
		factory := HTTPCheckerFactory{}
		target := &k8swatchv1.Target{
			Spec: k8swatchv1.TargetSpec{
				Type: k8swatchv1.TargetTypeHTTP,
			},
		}

		checker, err := factory.Create(target)

		assert.NoError(t, err)
		assert.NotNil(t, checker)

		layers := checker.Layers()
		assert.Greater(t, len(layers), 0)
	})
}

// Test TLS Layer
func TestTLSLayer(t *testing.T) {
	t.Run("Name", func(t *testing.T) {
		layer := NewTLSLayer()
		assert.Equal(t, "L3", layer.Name())
	})
}

// Test Executor helpers
func TestExecutorHelpers(t *testing.T) {
	t.Run("LayerResultSuccess", func(t *testing.T) {
		result := LayerResultSuccess(100)

		assert.True(t, result.Success)
		assert.Equal(t, int64(100), result.DurationMs)
		assert.Empty(t, result.FailureCode)
		assert.Empty(t, result.FailureMessage)
	})

	t.Run("LayerResultFailure", func(t *testing.T) {
		result := LayerResultFailure("tcp_refused", "connection refused", 50)

		assert.False(t, result.Success)
		assert.Equal(t, "tcp_refused", result.FailureCode)
		assert.Equal(t, "connection refused", result.FailureMessage)
		assert.Equal(t, int64(50), result.DurationMs)
	})

	t.Run("LayerResultError", func(t *testing.T) {
		err := errors.New("test error")
		result := LayerResultError(err, "unknown", 10)

		assert.False(t, result.Success)
		assert.Equal(t, int64(10), result.DurationMs)
		assert.NotEmpty(t, result.FailureCode)
	})
}

// Test FormatDuration
func TestFormatDuration_Comprehensive(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
	}{
		{"milliseconds", 150 * time.Millisecond},
		{"seconds", 2 * time.Second},
		{"minutes", 5 * time.Minute},
		{"mixed", 2*time.Second + 500*time.Millisecond},
		{"microseconds", 500 * time.Microsecond},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatDuration(tt.duration)
			assert.NotEmpty(t, result)
		})
	}
}

// Test contains helper
func TestContains_Comprehensive(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected bool
	}{
		{"found", "abc", "b", true},
		{"not found", "abc", "d", false},
		{"empty substring", "abc", "", true},
		{"exact match", "abc", "abc", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.s, tt.substr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test findSubstring helper
func TestFindSubstring_Comprehensive(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected bool
	}{
		{"found", "abcdef", "cd", true},
		{"not found", "abcdef", "xyz", false},
		{"exact match", "abc", "abc", true},
		{"empty substring", "abc", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findSubstring(tt.s, tt.substr)
			assert.Equal(t, tt.expected, result)
		})
	}
}
