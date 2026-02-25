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
	"sync"
	"time"

	pb "github.com/k8swatch/k8s-monitor/internal/pb"
)

// CorrelationConfig holds failure correlation configuration
type CorrelationConfig struct {
	// TimeWindow is the window for correlating failures
	TimeWindow time.Duration
	// MinNodesForPattern is minimum nodes to detect a pattern
	MinNodesForPattern int
}

// DefaultCorrelationConfig returns default correlation configuration
func DefaultCorrelationConfig() *CorrelationConfig {
	return &CorrelationConfig{
		TimeWindow:         60 * time.Second,
		MinNodesForPattern: 2,
	}
}

// CorrelationEngine correlates failures across nodes and targets
type CorrelationEngine struct {
	config        *CorrelationConfig
	failureWindow map[string][]*FailureEvent // target -> failures in window
	mu            sync.RWMutex
}

// FailureEvent represents a single failure event
type FailureEvent struct {
	Node         string
	NetworkMode  pb.NetworkMode
	FailureCode  string
	FailureLayer string
	Timestamp    time.Time
}

// NewCorrelationEngine creates new correlation engine
func NewCorrelationEngine(config *CorrelationConfig) *CorrelationEngine {
	if config == nil {
		config = DefaultCorrelationConfig()
	}

	return &CorrelationEngine{
		config:        config,
		failureWindow: make(map[string][]*FailureEvent),
	}
}

// RecordFailure records a failure event
func (c *CorrelationEngine) RecordFailure(target string, result *pb.SubmitResultRequest) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if result.Check.Success {
		return
	}

	event := &FailureEvent{
		Node:         result.Agent.NodeName,
		NetworkMode:  result.Agent.NetworkMode,
		FailureCode:  result.Check.FailureCode,
		FailureLayer: result.Check.FailureLayer,
		Timestamp:    time.Now(),
	}

	c.failureWindow[target] = append(c.failureWindow[target], event)
	c.cleanupOldEvents(target)
}

// cleanupOldEvents removes events outside the time window
func (c *CorrelationEngine) cleanupOldEvents(target string) {
	events := c.failureWindow[target]
	cutoff := time.Now().Add(-c.config.TimeWindow)

	cleaned := make([]*FailureEvent, 0)
	for _, e := range events {
		if e.Timestamp.After(cutoff) {
			cleaned = append(cleaned, e)
		}
	}

	c.failureWindow[target] = cleaned
}

// GetFailureCount returns failure count for a target in the time window
func (c *CorrelationEngine) GetFailureCount(target string) int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.failureWindow[target])
}

// GetAffectedNodes returns nodes affected by failures for a target
func (c *CorrelationEngine) GetAffectedNodes(target string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	nodeSet := make(map[string]bool)
	for _, event := range c.failureWindow[target] {
		nodeSet[event.Node] = true
	}

	nodes := make([]string, 0, len(nodeSet))
	for node := range nodeSet {
		nodes = append(nodes, node)
	}

	return nodes
}

// DetectPattern detects failure patterns
func (c *CorrelationEngine) DetectPattern(target string, topology *TopologyAnalyzer) FailurePattern {
	c.mu.RLock()
	events := c.failureWindow[target]
	c.mu.RUnlock()

	if len(events) < c.config.MinNodesForPattern {
		return PatternUnknown
	}

	// Count failures by node and network mode
	nodeFailures := make(map[string]int)
	podNetworkFailures := make(map[string]bool)
	hostNetworkFailures := make(map[string]bool)

	for _, event := range events {
		nodeFailures[event.Node]++
		if event.NetworkMode == pb.NetworkMode_NETWORK_MODE_POD {
			podNetworkFailures[event.Node] = true
		} else if event.NetworkMode == pb.NetworkMode_NETWORK_MODE_HOST {
			hostNetworkFailures[event.Node] = true
		}
	}

	// Analyze network mode FIRST (more specific patterns)
	podNodes := make([]string, 0)
	hostNodes := make([]string, 0)
	for node := range podNetworkFailures {
		podNodes = append(podNodes, node)
	}
	for node := range hostNetworkFailures {
		hostNodes = append(hostNodes, node)
	}

	if len(podNodes) > 0 && len(hostNodes) == 0 {
		return PatternCNIIssue
	}
	if len(hostNodes) > 0 && len(podNodes) == 0 {
		return PatternNodeRoutingIssue
	}

	// Check if all nodes failing same target = target outage
	if len(nodeFailures) >= c.config.MinNodesForPattern {
		// Check if same failure code
		failureCode := events[0].FailureCode
		allSame := true
		for _, e := range events {
			if e.FailureCode != failureCode {
				allSame = false
				break
			}
		}
		if allSame {
			return PatternTargetOutage
		}
	}

	// Check for zone issue
	affectedNodes := c.GetAffectedNodes(target)
	if topology != nil {
		radius := topology.ClassifyBlastRadius(affectedNodes)
		if radius == BlastRadiusZone {
			return PatternZoneIssue
		}
	}

	// Single node failing all targets = node issue
	if len(nodeFailures) == 1 {
		return PatternNodeIssue
	}

	return PatternUnknown
}

// GenerateReport generates correlation report for a target
func (c *CorrelationEngine) GenerateReport(target string, topology *TopologyAnalyzer) *CorrelationReport {
	c.mu.RLock()
	events := c.failureWindow[target]
	c.mu.RUnlock()

	if len(events) == 0 {
		return nil
	}

	affectedNodes := c.GetAffectedNodes(target)
	affectedZones := make([]string, 0)
	zoneSet := make(map[string]bool)

	if topology != nil {
		for _, node := range affectedNodes {
			if zone := topology.GetZone(node); zone != "" && !zoneSet[zone] {
				zoneSet[zone] = true
				affectedZones = append(affectedZones, zone)
			}
		}
	}

	pattern := c.DetectPattern(target, topology)
	radius := BlastRadiusNode
	if topology != nil {
		radius = topology.ClassifyBlastRadius(affectedNodes)
	}

	// Find earliest event
	startTime := events[0].Timestamp
	for _, e := range events {
		if e.Timestamp.Before(startTime) {
			startTime = e.Timestamp
		}
	}

	return &CorrelationReport{
		Target:        target,
		FailureLayer:  events[0].FailureLayer,
		AffectedNodes: affectedNodes,
		AffectedZones: affectedZones,
		BlastRadius:   radius,
		Pattern:       pattern,
		StartTime:     startTime,
		Ongoing:       true,
	}
}

// ClearTarget clears failure events for a target
func (c *CorrelationEngine) ClearTarget(target string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.failureWindow, target)
}

// GetStats returns correlation engine statistics
func (c *CorrelationEngine) GetStats() CorrelationStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	totalTargets := len(c.failureWindow)
	totalEvents := 0
	for _, events := range c.failureWindow {
		totalEvents += len(events)
	}

	return CorrelationStats{
		TotalTargets:  totalTargets,
		TotalEvents:   totalEvents,
		WindowSeconds: int(c.config.TimeWindow.Seconds()),
	}
}

// CorrelationStats contains correlation statistics
type CorrelationStats struct {
	TotalTargets  int
	TotalEvents   int
	WindowSeconds int
}
