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

	corev1 "k8s.io/api/core/v1"
)

// TopologyConfig holds topology analyzer configuration
type TopologyConfig struct {
	// ClusterWideThreshold is the percentage of nodes for cluster-wide blast radius
	ClusterWideThreshold int
}

// DefaultTopologyConfig returns default topology configuration
func DefaultTopologyConfig() *TopologyConfig {
	return &TopologyConfig{
		ClusterWideThreshold: 30, // 30% of nodes
	}
}

// TopologyAnalyzer analyzes cluster topology and blast radius
type TopologyAnalyzer struct {
	config   *TopologyConfig
	zones    map[string][]string // zone -> node names
	nodes    map[string]NodeInfo // node name -> node info
	nodeZone map[string]string   // node name -> zone
	mu       sync.RWMutex
}

// NodeInfo contains node information
type NodeInfo struct {
	Name   string
	Zone   string
	Labels map[string]string
}

// NewTopologyAnalyzer creates new topology analyzer
func NewTopologyAnalyzer(config *TopologyConfig) *TopologyAnalyzer {
	if config == nil {
		config = DefaultTopologyConfig()
	}

	return &TopologyAnalyzer{
		config:   config,
		zones:    make(map[string][]string),
		nodes:    make(map[string]NodeInfo),
		nodeZone: make(map[string]string),
	}
}

// UpdateNode updates node information
func (t *TopologyAnalyzer) UpdateNode(node *corev1.Node) {
	t.mu.Lock()
	defer t.mu.Unlock()

	zone := node.Labels["topology.kubernetes.io/zone"]
	if zone == "" {
		zone = node.Labels["failure-domain.beta.kubernetes.io/zone"]
	}
	if zone == "" {
		zone = "unknown"
	}

	info := NodeInfo{
		Name:   node.Name,
		Zone:   zone,
		Labels: node.Labels,
	}

	t.nodes[node.Name] = info
	t.nodeZone[node.Name] = zone

	// Add to zone list
	found := false
	for _, n := range t.zones[zone] {
		if n == node.Name {
			found = true
			break
		}
	}
	if !found {
		t.zones[zone] = append(t.zones[zone], node.Name)
	}
}

// RemoveNode removes node information
func (t *TopologyAnalyzer) RemoveNode(nodeName string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	zone := t.nodeZone[nodeName]
	delete(t.nodes, nodeName)
	delete(t.nodeZone, nodeName)

	// Remove from zone list
	if zone != "" {
		nodes := t.zones[zone]
		for i, n := range nodes {
			if n == nodeName {
				t.zones[zone] = append(nodes[:i], nodes[i+1:]...)
				break
			}
		}
		if len(t.zones[zone]) == 0 {
			delete(t.zones, zone)
		}
	}
}

// GetZone returns zone for a node
func (t *TopologyAnalyzer) GetZone(nodeName string) string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.nodeZone[nodeName]
}

// GetNodesInZone returns all nodes in a zone
func (t *TopologyAnalyzer) GetNodesInZone(zone string) []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	nodes := t.zones[zone]
	result := make([]string, len(nodes))
	copy(result, nodes)
	return result
}

// GetTotalNodes returns total number of nodes
func (t *TopologyAnalyzer) GetTotalNodes() int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return len(t.nodes)
}

// GetTotalZones returns total number of zones
func (t *TopologyAnalyzer) GetTotalZones() int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return len(t.zones)
}

// ClassifyBlastRadius classifies blast radius based on affected nodes
func (t *TopologyAnalyzer) ClassifyBlastRadius(affectedNodes []string) BlastRadiusType {
	t.mu.RLock()
	defer t.mu.RUnlock()

	totalNodes := len(t.nodes)
	if totalNodes == 0 {
		return BlastRadiusNode
	}

	affectedCount := len(affectedNodes)

	// Count affected zones
	affectedZones := make(map[string]bool)
	for _, node := range affectedNodes {
		if zone, exists := t.nodeZone[node]; exists {
			affectedZones[zone] = true
		}
	}

	totalZones := len(t.zones)

	// Cluster-wide: >30% of nodes AND multiple zones affected
	clusterThreshold := (totalNodes * t.config.ClusterWideThreshold) / 100
	if affectedCount > clusterThreshold && len(affectedZones) > 1 {
		return BlastRadiusCluster
	}

	// Also cluster if all zones affected AND significant node count
	if len(affectedZones) == totalZones && totalZones > 1 && affectedCount > 1 {
		return BlastRadiusCluster
	}

	// Zone-level: Multiple nodes affected OR multiple zones affected
	if affectedCount > 1 || len(affectedZones) > 1 {
		return BlastRadiusZone
	}

	// Node-local: Single node affected
	return BlastRadiusNode
}

// AnalyzeNetworkMode analyzes network mode patterns
func (t *TopologyAnalyzer) AnalyzeNetworkMode(podNetworkFailures, hostNetworkFailures []string) NetworkModeAnalysis {
	analysis := NetworkModeAnalysis{
		PodNetworkOnly:      false,
		HostNetworkOnly:     false,
		BothNetworksFailing: false,
		Conclusion:          "Unknown",
	}

	podSet := make(map[string]bool)
	hostSet := make(map[string]bool)

	for _, node := range podNetworkFailures {
		podSet[node] = true
	}
	for _, node := range hostNetworkFailures {
		hostSet[node] = true
	}

	if len(podSet) > 0 && len(hostSet) == 0 {
		analysis.PodNetworkOnly = true
		analysis.Conclusion = "CNI issue - Pod network failures only"
	} else if len(hostSet) > 0 && len(podSet) == 0 {
		analysis.HostNetworkOnly = true
		analysis.Conclusion = "Node routing issue - Host network failures only"
	} else if len(podSet) > 0 && len(hostSet) > 0 {
		analysis.BothNetworksFailing = true
		analysis.Conclusion = "Target or DNS issue - Both networks failing"
	}

	return analysis
}

// GetTopologyStats returns topology statistics
func (t *TopologyAnalyzer) GetTopologyStats() TopologyStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return TopologyStats{
		TotalNodes: len(t.nodes),
		TotalZones: len(t.zones),
		Zones:      t.zones,
	}
}

// BlastRadiusType represents blast radius classification
type BlastRadiusType string

const (
	BlastRadiusNode    BlastRadiusType = "node"
	BlastRadiusZone    BlastRadiusType = "zone"
	BlastRadiusCluster BlastRadiusType = "cluster"
)

// NetworkModeAnalysis represents network mode analysis result
type NetworkModeAnalysis struct {
	PodNetworkOnly      bool
	HostNetworkOnly     bool
	BothNetworksFailing bool
	Conclusion          string
}

// TopologyStats contains topology statistics
type TopologyStats struct {
	TotalNodes int
	TotalZones int
	Zones      map[string][]string
}

// CorrelationReport represents failure correlation report
type CorrelationReport struct {
	Target        string
	FailureLayer  string
	AffectedNodes []string
	AffectedZones []string
	BlastRadius   BlastRadiusType
	Pattern       FailurePattern
	StartTime     time.Time
	Ongoing       bool
}

// FailurePattern represents detected failure pattern
type FailurePattern string

const (
	PatternTargetOutage     FailurePattern = "target_outage"
	PatternNodeIssue        FailurePattern = "node_issue"
	PatternZoneIssue        FailurePattern = "zone_issue"
	PatternCNIIssue         FailurePattern = "cni_issue"
	PatternNodeRoutingIssue FailurePattern = "node_routing_issue"
	PatternUnknown          FailurePattern = "unknown"
)
