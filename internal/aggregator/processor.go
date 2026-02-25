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
	"sync"
	"time"

	pb "github.com/k8swatch/k8s-monitor/internal/pb"
)

// ProcessorConfig holds stream processor configuration
type ProcessorConfig struct {
	StateCleanupInterval time.Duration
	StateExpiration      time.Duration
	MaxStateSize         int
}

// DefaultProcessorConfig returns default processor configuration
func DefaultProcessorConfig() *ProcessorConfig {
	return &ProcessorConfig{
		StateCleanupInterval: 5 * time.Minute,
		StateExpiration:      24 * time.Hour,
		MaxStateSize:         10000,
	}
}

// StreamProcessor processes result streams and maintains state
type StreamProcessor struct {
	config    *ProcessorConfig
	states    map[string]*TargetState
	statesMu  sync.RWMutex
	startTime time.Time
}

// TargetState represents state of a target
type TargetState struct {
	TargetKey            string
	TargetName           string
	TargetNamespace      string
	TargetType           string
	ConsecutiveFailures  int
	ConsecutiveSuccesses int
	LastSuccessTime      time.Time
	LastFailureTime      time.Time
	LastFailureCode      string
	LastFailureLayer     string
	State                TargetStateEnum
	LastUpdatedAt        time.Time
	AgentStates          map[string]*AgentTargetState
}

// AgentTargetState represents state for agent-target combination
type AgentTargetState struct {
	AgentNode            string
	NetworkMode          string
	LastSuccess          time.Time
	LastFailure          time.Time
	LastFailureCode      string
	LastFailureLayer     string
	ConsecutiveFailures  int
	ConsecutiveSuccesses int
}

// TargetStateEnum represents target state
type TargetStateEnum string

const (
	TargetStateHealthy   TargetStateEnum = "Healthy"
	TargetStateDegraded  TargetStateEnum = "Degraded"
	TargetStateUnhealthy TargetStateEnum = "Unhealthy"
)

// NewStreamProcessor creates new stream processor
func NewStreamProcessor(config *ProcessorConfig) *StreamProcessor {
	if config == nil {
		config = DefaultProcessorConfig()
	}

	return &StreamProcessor{
		config:    config,
		states:    make(map[string]*TargetState),
		startTime: time.Now(),
	}
}

// ProcessResult processes single result and updates state
func (p *StreamProcessor) ProcessResult(ctx context.Context, result *pb.SubmitResultRequest) error {
	p.updateState(result)
	return nil
}

// updateState updates state based on result
func (p *StreamProcessor) updateState(result *pb.SubmitResultRequest) {
	targetKey := p.makeTargetKey(result.Target)
	now := time.Now()

	p.statesMu.Lock()
	defer p.statesMu.Unlock()

	state, exists := p.states[targetKey]
	if !exists {
		state = p.createInitialState(result)
		p.states[targetKey] = state
	}

	p.updateAgentState(state, result)

	if result.Check.Success {
		state.ConsecutiveSuccesses++
		state.ConsecutiveFailures = 0
		state.LastSuccessTime = now
		state.State = TargetStateHealthy
	} else {
		state.ConsecutiveFailures++
		state.ConsecutiveSuccesses = 0
		state.LastFailureTime = now
		state.LastFailureCode = result.Check.FailureCode
		state.LastFailureLayer = result.Check.FailureLayer
		state.State = TargetStateUnhealthy
	}

	state.LastUpdatedAt = now
}

// createInitialState creates initial state for target
func (p *StreamProcessor) createInitialState(result *pb.SubmitResultRequest) *TargetState {
	return &TargetState{
		TargetKey:       p.makeTargetKey(result.Target),
		TargetName:      result.Target.Name,
		TargetNamespace: result.Target.Namespace,
		TargetType:      result.Target.Type,
		State:           TargetStateHealthy,
		LastUpdatedAt:   time.Now(),
		AgentStates:     make(map[string]*AgentTargetState),
	}
}

// updateAgentState updates agent-specific state
func (p *StreamProcessor) updateAgentState(state *TargetState, result *pb.SubmitResultRequest) {
	agentKey := result.Agent.NodeName + ":" + string(result.Agent.NetworkMode)

	agentState, exists := state.AgentStates[agentKey]
	if !exists {
		agentState = &AgentTargetState{
			AgentNode:   result.Agent.NodeName,
			NetworkMode: string(result.Agent.NetworkMode),
		}
		state.AgentStates[agentKey] = agentState
	}

	now := time.Now()
	if result.Check.Success {
		agentState.ConsecutiveSuccesses++
		agentState.ConsecutiveFailures = 0
		agentState.LastSuccess = now
	} else {
		agentState.ConsecutiveFailures++
		agentState.ConsecutiveSuccesses = 0
		agentState.LastFailure = now
		agentState.LastFailureCode = result.Check.FailureCode
		agentState.LastFailureLayer = result.Check.FailureLayer
	}
}

// makeTargetKey creates unique key for target
func (p *StreamProcessor) makeTargetKey(target *pb.TargetInfo) string {
	return target.Namespace + "/" + target.Name
}

// GetState returns state for target
func (p *StreamProcessor) GetState(targetKey string) (*TargetState, bool) {
	p.statesMu.RLock()
	defer p.statesMu.RUnlock()

	state, exists := p.states[targetKey]
	if !exists {
		return nil, false
	}

	return p.copyState(state), true
}

// GetAllStates returns all target states
func (p *StreamProcessor) GetAllStates() map[string]*TargetState {
	p.statesMu.RLock()
	defer p.statesMu.RUnlock()

	result := make(map[string]*TargetState, len(p.states))
	for k, v := range p.states {
		result[k] = p.copyState(v)
	}
	return result
}

// copyState creates copy of state
func (p *StreamProcessor) copyState(state *TargetState) *TargetState {
	if state == nil {
		return nil
	}

	copy := &TargetState{
		TargetKey:            state.TargetKey,
		TargetName:           state.TargetName,
		TargetNamespace:      state.TargetNamespace,
		TargetType:           state.TargetType,
		ConsecutiveFailures:  state.ConsecutiveFailures,
		ConsecutiveSuccesses: state.ConsecutiveSuccesses,
		LastSuccessTime:      state.LastSuccessTime,
		LastFailureTime:      state.LastFailureTime,
		LastFailureCode:      state.LastFailureCode,
		LastFailureLayer:     state.LastFailureLayer,
		State:                state.State,
		LastUpdatedAt:        state.LastUpdatedAt,
		AgentStates:          make(map[string]*AgentTargetState),
	}

	for k, v := range state.AgentStates {
		copy.AgentStates[k] = &AgentTargetState{
			AgentNode:            v.AgentNode,
			NetworkMode:          v.NetworkMode,
			LastSuccess:          v.LastSuccess,
			LastFailure:          v.LastFailure,
			LastFailureCode:      v.LastFailureCode,
			LastFailureLayer:     v.LastFailureLayer,
			ConsecutiveFailures:  v.ConsecutiveFailures,
			ConsecutiveSuccesses: v.ConsecutiveSuccesses,
		}
	}

	return copy
}

// CleanupExpiredStates removes expired states
func (p *StreamProcessor) CleanupExpiredStates() int {
	p.statesMu.Lock()
	defer p.statesMu.Unlock()

	expired := 0
	now := time.Now()

	for key, state := range p.states {
		if now.Sub(state.LastUpdatedAt) > p.config.StateExpiration {
			delete(p.states, key)
			expired++
		}
	}

	if expired > 0 {
		log.Info("Cleaned up expired states", "count", expired)
	}

	return expired
}

// GetStats returns processor statistics
func (p *StreamProcessor) GetStats() ProcessorStats {
	p.statesMu.RLock()
	defer p.statesMu.RUnlock()

	return ProcessorStats{
		StartTime:      p.startTime,
		TotalTargets:   len(p.states),
		HealthyCount:   p.countByState(TargetStateHealthy),
		DegradedCount:  p.countByState(TargetStateDegraded),
		UnhealthyCount: p.countByState(TargetStateUnhealthy),
	}
}

// countByState counts targets in specific state
func (p *StreamProcessor) countByState(state TargetStateEnum) int {
	count := 0
	for _, s := range p.states {
		if s.State == state {
			count++
		}
	}
	return count
}

// ProcessorStats contains processor statistics
type ProcessorStats struct {
	StartTime      time.Time
	TotalTargets   int
	HealthyCount   int
	DegradedCount  int
	UnhealthyCount int
}
