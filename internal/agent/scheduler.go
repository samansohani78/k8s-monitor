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
	"math/rand"
	"sync"
	"time"

	k8swatchv1 "github.com/k8swatch/k8s-monitor/api/v1"
)

// SchedulerConfig holds the scheduler configuration
type SchedulerConfig struct {
	// MaxConcurrency is the maximum number of concurrent checks
	MaxConcurrency int

	// DefaultInterval is the default check interval
	DefaultInterval time.Duration

	// DefaultJitter is the default jitter percentage (0.1 = 10%)
	DefaultJitter float64

	// DefaultTimeout is the default check timeout
	DefaultTimeout time.Duration
}

// DefaultSchedulerConfig returns the default scheduler configuration
func DefaultSchedulerConfig() *SchedulerConfig {
	return &SchedulerConfig{
		MaxConcurrency:  10,
		DefaultInterval: 30 * time.Second,
		DefaultJitter:   0.1, // 10%
		DefaultTimeout:  15 * time.Second,
	}
}

// Scheduler schedules and manages health check execution
type Scheduler struct {
	config    *SchedulerConfig
	semaphore chan struct{}
	targets   map[string]*k8swatchv1.Target
	targetsMu sync.RWMutex
	checkFunc CheckFunc
	shutdown  chan struct{}
	wg        sync.WaitGroup
}

// CheckFunc is the function signature for executing a check
type CheckFunc func(ctx context.Context, target *k8swatchv1.Target) (*k8swatchv1.CheckResult, error)

// NewScheduler creates a new scheduler
func NewScheduler(config *SchedulerConfig, checkFunc CheckFunc) *Scheduler {
	if config == nil {
		config = DefaultSchedulerConfig()
	}

	return &Scheduler{
		config:    config,
		semaphore: make(chan struct{}, config.MaxConcurrency),
		targets:   make(map[string]*k8swatchv1.Target),
		checkFunc: checkFunc,
		shutdown:  make(chan struct{}),
	}
}

// Start starts the scheduler
func (s *Scheduler) Start(ctx context.Context) error {
	log.Info("Starting scheduler",
		"maxConcurrency", s.config.MaxConcurrency,
		"defaultInterval", s.config.DefaultInterval,
		"defaultJitter", s.config.DefaultJitter,
	)

	// Start target watcher in a goroutine
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.watchTargets(ctx)
	}()

	// Wait for shutdown
	<-ctx.Done()
	log.Info("Scheduler context done, initiating shutdown")

	// Signal shutdown
	close(s.shutdown)

	// Wait for all goroutines to finish
	s.wg.Wait()

	log.Info("Scheduler shutdown complete")
	return nil
}

// watchTargets watches for target changes and updates the scheduler
func (s *Scheduler) watchTargets(ctx context.Context) {
	// Initial staggered start - randomize initial delay
	initialDelay := time.Duration(rand.Float64() * float64(s.config.DefaultInterval)) // nolint:gosec // cryptographically secure randomness not needed for jitter
	log.Info("Initial staggered start", "delay", initialDelay)

	select {
	case <-time.After(initialDelay):
	case <-ctx.Done():
		return
	}

	// Start scheduling loop
	ticker := time.NewTicker(s.config.DefaultInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.shutdown:
			return
		case <-ticker.C:
			s.scheduleChecks(ctx)
		}
	}
}

// scheduleChecks schedules checks for all targets
func (s *Scheduler) scheduleChecks(ctx context.Context) {
	s.targetsMu.RLock()
	defer s.targetsMu.RUnlock()

	for _, target := range s.targets {
		select {
		case <-ctx.Done():
			return
		case <-s.shutdown:
			return
		default:
			// Schedule check in a goroutine
			s.wg.Add(1)
			go func(t *k8swatchv1.Target) {
				defer s.wg.Done()
				s.executeCheck(ctx, t)
			}(target)
		}
	}
}

// executeCheck executes a single check with semaphore limiting
func (s *Scheduler) executeCheck(ctx context.Context, target *k8swatchv1.Target) {
	// Acquire semaphore
	select {
	case s.semaphore <- struct{}{}:
	case <-ctx.Done():
		return
	case <-s.shutdown:
		return
	}
	defer func() { <-s.semaphore }()

	// Get interval and timeout from target spec
	interval := s.config.DefaultInterval
	timeout := s.config.DefaultTimeout

	if target.Spec.Schedule.Interval != "" {
		if d, err := time.ParseDuration(target.Spec.Schedule.Interval); err == nil {
			interval = d
		}
	}

	if target.Spec.Schedule.Timeout != "" {
		if d, err := time.ParseDuration(target.Spec.Schedule.Timeout); err == nil {
			timeout = d
		}
	}

	// Add jitter to interval
	jitter := time.Duration(float64(interval) * s.config.DefaultJitter * (rand.Float64()*2 - 1)) // nolint:gosec // cryptographically secure randomness not needed for jitter
	executionDelay := time.Duration(rand.Float64() * float64(jitter))                            // nolint:gosec // cryptographically secure randomness not needed for jitter

	if executionDelay > 0 {
		select {
		case <-time.After(executionDelay):
		case <-ctx.Done():
			return
		case <-s.shutdown:
			return
		}
	}

	// Create context with timeout
	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Execute check
	log.Info("Executing check",
		"target", target.Name,
		"namespace", target.Namespace,
		"type", target.Spec.Type,
	)

	result, err := s.checkFunc(checkCtx, target)
	if err != nil {
		log.Error(err, "Check failed",
			"target", target.Name,
			"namespace", target.Namespace,
		)
		return
	}

	// Process result (send to aggregator, etc.)
	s.processResult(result)
}

// processResult processes a check result
// In the current implementation, results are sent directly in executeCheck
// This method is kept for future enhancements (metrics, local logging, etc.)
func (s *Scheduler) processResult(result *k8swatchv1.CheckResult) {
	log.Info("Check completed",
		"resultId", result.ResultID,
		"target", result.Target.Name,
		"success", result.Check.Success,
		"failureLayer", result.Check.FailureLayer,
		"latencyMs", result.Metadata.CheckDurationMs,
	)
}

// UpdateTargets updates the list of targets to monitor
func (s *Scheduler) UpdateTargets(targets []k8swatchv1.Target) {
	s.targetsMu.Lock()
	defer s.targetsMu.Unlock()

	s.targets = make(map[string]*k8swatchv1.Target)
	for i := range targets {
		key := targets[i].Namespace + "/" + targets[i].Name
		s.targets[key] = &targets[i]
	}

	log.Info("Targets updated", "count", len(s.targets))
}

// TargetCount returns the number of targets being monitored
func (s *Scheduler) TargetCount() int {
	s.targetsMu.RLock()
	defer s.targetsMu.RUnlock()
	return len(s.targets)
}

// parseDuration parses a duration string with a default value
func parseDuration(s string, defaultVal time.Duration) time.Duration {
	if s == "" {
		return defaultVal
	}
	if d, err := time.ParseDuration(s); err == nil {
		return d
	}
	return defaultVal
}
