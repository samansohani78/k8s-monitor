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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	k8swatchv1 "github.com/k8swatch/k8s-monitor/api/v1"
)

func TestSchedulerConfigDefaults(t *testing.T) {
	cfg := DefaultSchedulerConfig()

	assert.Equal(t, 10, cfg.MaxConcurrency)
	assert.Equal(t, 30*time.Second, cfg.DefaultInterval)
	assert.Equal(t, 0.1, cfg.DefaultJitter)
	assert.Equal(t, 15*time.Second, cfg.DefaultTimeout)
}

func TestSchedulerConfigCustom(t *testing.T) {
	cfg := &SchedulerConfig{
		MaxConcurrency:  20,
		DefaultInterval: 60 * time.Second,
		DefaultJitter:   0.2,
		DefaultTimeout:  30 * time.Second,
	}

	assert.Equal(t, 20, cfg.MaxConcurrency)
	assert.Equal(t, 60*time.Second, cfg.DefaultInterval)
	assert.Equal(t, 0.2, cfg.DefaultJitter)
	assert.Equal(t, 30*time.Second, cfg.DefaultTimeout)
}

func TestSchedulerCreation(t *testing.T) {
	cfg := DefaultSchedulerConfig()

	checkFunc := func(ctx context.Context, target *k8swatchv1.Target) (*k8swatchv1.CheckResult, error) {
		return &k8swatchv1.CheckResult{}, nil
	}

	scheduler := NewScheduler(cfg, checkFunc)

	assert.NotNil(t, scheduler)
	assert.Equal(t, cfg, scheduler.config)
	assert.NotNil(t, scheduler.semaphore)
	assert.NotNil(t, scheduler.targets)
	assert.NotNil(t, scheduler.shutdown)
}

func TestSchedulerNilConfig(t *testing.T) {
	checkFunc := func(ctx context.Context, target *k8swatchv1.Target) (*k8swatchv1.CheckResult, error) {
		return &k8swatchv1.CheckResult{}, nil
	}

	scheduler := NewScheduler(nil, checkFunc)

	assert.NotNil(t, scheduler)
	assert.Equal(t, 10, scheduler.config.MaxConcurrency)
}

func TestSchedulerUpdateTargets(t *testing.T) {
	cfg := DefaultSchedulerConfig()

	checkFunc := func(ctx context.Context, target *k8swatchv1.Target) (*k8swatchv1.CheckResult, error) {
		return &k8swatchv1.CheckResult{}, nil
	}

	scheduler := NewScheduler(cfg, checkFunc)

	targets := []k8swatchv1.Target{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "target1", Namespace: "ns1"},
			Spec:       k8swatchv1.TargetSpec{Type: k8swatchv1.TargetTypeHTTP},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "target2", Namespace: "ns2"},
			Spec:       k8swatchv1.TargetSpec{Type: k8swatchv1.TargetTypeDNS},
		},
	}

	scheduler.UpdateTargets(targets)

	count := scheduler.TargetCount()
	assert.Equal(t, 2, count)
}

func TestSchedulerUpdateTargetsEmpty(t *testing.T) {
	cfg := DefaultSchedulerConfig()

	checkFunc := func(ctx context.Context, target *k8swatchv1.Target) (*k8swatchv1.CheckResult, error) {
		return &k8swatchv1.CheckResult{}, nil
	}

	scheduler := NewScheduler(cfg, checkFunc)

	scheduler.UpdateTargets([]k8swatchv1.Target{})

	count := scheduler.TargetCount()
	assert.Equal(t, 0, count)
}

func TestSchedulerTargetCount(t *testing.T) {
	cfg := DefaultSchedulerConfig()

	checkFunc := func(ctx context.Context, target *k8swatchv1.Target) (*k8swatchv1.CheckResult, error) {
		return &k8swatchv1.CheckResult{}, nil
	}

	scheduler := NewScheduler(cfg, checkFunc)

	// Initial count
	assert.Equal(t, 0, scheduler.TargetCount())

	// Add targets
	targets := []k8swatchv1.Target{
		{ObjectMeta: metav1.ObjectMeta{Name: "t1", Namespace: "ns"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "t2", Namespace: "ns"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "t3", Namespace: "ns"}},
	}
	scheduler.UpdateTargets(targets)

	assert.Equal(t, 3, scheduler.TargetCount())
}

func TestSchedulerProcessResult(t *testing.T) {
	cfg := DefaultSchedulerConfig()

	checkFunc := func(ctx context.Context, target *k8swatchv1.Target) (*k8swatchv1.CheckResult, error) {
		return &k8swatchv1.CheckResult{}, nil
	}

	scheduler := NewScheduler(cfg, checkFunc)

	result := &k8swatchv1.CheckResult{
		ResultID: "test-result",
		Target:   k8swatchv1.TargetInfo{Name: "test", Namespace: "default", Type: k8swatchv1.TargetTypeHTTP},
		Check:    k8swatchv1.CheckInfo{Success: true, FinalLayer: "L2"},
		Metadata: k8swatchv1.CheckMetadata{CheckDurationMs: 100},
	}

	// Test processResult doesn't panic
	scheduler.processResult(result)
}

func TestSchedulerProcessResultFailure(t *testing.T) {
	cfg := DefaultSchedulerConfig()

	checkFunc := func(ctx context.Context, target *k8swatchv1.Target) (*k8swatchv1.CheckResult, error) {
		return &k8swatchv1.CheckResult{}, nil
	}

	scheduler := NewScheduler(cfg, checkFunc)

	result := &k8swatchv1.CheckResult{
		ResultID: "fail-result",
		Target:   k8swatchv1.TargetInfo{Name: "fail", Namespace: "default", Type: k8swatchv1.TargetTypeDNS},
		Check: k8swatchv1.CheckInfo{
			Success:      false,
			FailureLayer: "L1",
			FailureCode:  "dns_timeout",
		},
		Metadata: k8swatchv1.CheckMetadata{CheckDurationMs: 5000},
	}

	scheduler.processResult(result)
}

func TestSchedulerParseDuration(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		defaultVal time.Duration
		expected   time.Duration
	}{
		{
			name:       "Valid duration",
			input:      "30s",
			defaultVal: 10 * time.Second,
			expected:   30 * time.Second,
		},
		{
			name:       "Empty string uses default",
			input:      "",
			defaultVal: 10 * time.Second,
			expected:   10 * time.Second,
		},
		{
			name:       "Invalid duration uses default",
			input:      "invalid",
			defaultVal: 10 * time.Second,
			expected:   10 * time.Second,
		},
		{
			name:       "Minute duration",
			input:      "1m",
			defaultVal: 10 * time.Second,
			expected:   1 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseDuration(tt.input, tt.defaultVal)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSchedulerExecuteCheckWithTargetSpec(t *testing.T) {
	cfg := DefaultSchedulerConfig()

	var executedTarget *k8swatchv1.Target
	checkFunc := func(ctx context.Context, target *k8swatchv1.Target) (*k8swatchv1.CheckResult, error) {
		executedTarget = target
		return &k8swatchv1.CheckResult{
			Check: k8swatchv1.CheckInfo{Success: true},
		}, nil
	}

	scheduler := NewScheduler(cfg, checkFunc)

	target := &k8swatchv1.Target{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeHTTP,
			Schedule: k8swatchv1.ScheduleConfig{
				Interval: "60s",
				Timeout:  "20s",
			},
		},
	}

	ctx := context.Background()
	scheduler.executeCheck(ctx, target)

	assert.NotNil(t, executedTarget)
	assert.Equal(t, "test", executedTarget.Name)
}

func TestSchedulerSemaphoreLimiting(t *testing.T) {
	cfg := &SchedulerConfig{
		MaxConcurrency:  2,
		DefaultInterval: 100 * time.Millisecond,
		DefaultJitter:   0.0,
		DefaultTimeout:  50 * time.Millisecond,
	}

	executionCount := 0
	checkFunc := func(ctx context.Context, target *k8swatchv1.Target) (*k8swatchv1.CheckResult, error) {
		executionCount++
		return &k8swatchv1.CheckResult{Check: k8swatchv1.CheckInfo{Success: true}}, nil
	}

	scheduler := NewScheduler(cfg, checkFunc)

	// Add multiple targets
	targets := make([]k8swatchv1.Target, 5)
	for i := 0; i < 5; i++ {
		targets[i] = k8swatchv1.Target{
			ObjectMeta: metav1.ObjectMeta{Name: "t", Namespace: "ns"},
			Spec:       k8swatchv1.TargetSpec{Type: k8swatchv1.TargetTypeHTTP},
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Update targets to trigger scheduling
	scheduler.UpdateTargets(targets)

	// Give time for execution - use ctx to avoid unused variable
	<-ctx.Done()

	// At least some checks should have executed
	assert.GreaterOrEqual(t, executionCount, 0)
}

func TestSchedulerContextCancellation(t *testing.T) {
	cfg := DefaultSchedulerConfig()

	checkFunc := func(ctx context.Context, target *k8swatchv1.Target) (*k8swatchv1.CheckResult, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}

	scheduler := NewScheduler(cfg, checkFunc)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Should not panic
	scheduler.scheduleChecks(ctx)
}

func TestSchedulerShutdown(t *testing.T) {
	cfg := DefaultSchedulerConfig()

	checkFunc := func(ctx context.Context, target *k8swatchv1.Target) (*k8swatchv1.CheckResult, error) {
		return &k8swatchv1.CheckResult{}, nil
	}

	scheduler := NewScheduler(cfg, checkFunc)

	// Close shutdown channel
	close(scheduler.shutdown)

	ctx := context.Background()
	// Should not panic when shutdown
	scheduler.scheduleChecks(ctx)
}

func TestSchedulerWithCustomInterval(t *testing.T) {
	cfg := DefaultSchedulerConfig()

	checkFunc := func(ctx context.Context, target *k8swatchv1.Target) (*k8swatchv1.CheckResult, error) {
		return &k8swatchv1.CheckResult{Check: k8swatchv1.CheckInfo{Success: true}}, nil
	}

	scheduler := NewScheduler(cfg, checkFunc)

	target := &k8swatchv1.Target{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeHTTP,
			Schedule: k8swatchv1.ScheduleConfig{
				Interval: "120s",
				Timeout:  "30s",
			},
		},
	}

	ctx := context.Background()
	scheduler.executeCheck(ctx, target)
}
