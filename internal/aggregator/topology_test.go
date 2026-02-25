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
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestTopologyConfigDefaults(t *testing.T) {
	cfg := DefaultTopologyConfig()

	assert.Equal(t, 30, cfg.ClusterWideThreshold)
}

func TestTopologyAnalyzerCreation(t *testing.T) {
	cfg := DefaultTopologyConfig()
	analyzer := NewTopologyAnalyzer(cfg)

	assert.NotNil(t, analyzer)
	assert.NotNil(t, analyzer.zones)
	assert.NotNil(t, analyzer.nodes)
	assert.NotNil(t, analyzer.nodeZone)
	assert.Equal(t, 0, analyzer.GetTotalNodes())
}

func TestTopologyAnalyzerCreationNilConfig(t *testing.T) {
	analyzer := NewTopologyAnalyzer(nil)

	assert.NotNil(t, analyzer)
	assert.Equal(t, 30, analyzer.config.ClusterWideThreshold)
}

func TestTopologyAnalyzerUpdateNode(t *testing.T) {
	analyzer := NewTopologyAnalyzer(DefaultTopologyConfig())

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-1",
			Labels: map[string]string{
				"topology.kubernetes.io/zone": "zone-a",
			},
		},
	}

	analyzer.UpdateNode(node)

	assert.Equal(t, 1, analyzer.GetTotalNodes())
	assert.Equal(t, 1, analyzer.GetTotalZones())
	assert.Equal(t, "zone-a", analyzer.GetZone("node-1"))
}

func TestTopologyAnalyzerUpdateNodeLegacyLabel(t *testing.T) {
	analyzer := NewTopologyAnalyzer(DefaultTopologyConfig())

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-1",
			Labels: map[string]string{
				"failure-domain.beta.kubernetes.io/zone": "zone-b",
			},
		},
	}

	analyzer.UpdateNode(node)

	assert.Equal(t, "zone-b", analyzer.GetZone("node-1"))
}

func TestTopologyAnalyzerUpdateNodeUnknownZone(t *testing.T) {
	analyzer := NewTopologyAnalyzer(DefaultTopologyConfig())

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node-1",
			Labels: map[string]string{},
		},
	}

	analyzer.UpdateNode(node)

	assert.Equal(t, "unknown", analyzer.GetZone("node-1"))
}

func TestTopologyAnalyzerRemoveNode(t *testing.T) {
	analyzer := NewTopologyAnalyzer(DefaultTopologyConfig())

	// Add node
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-1",
			Labels: map[string]string{
				"topology.kubernetes.io/zone": "zone-a",
			},
		},
	}
	analyzer.UpdateNode(node)
	assert.Equal(t, 1, analyzer.GetTotalNodes())

	// Remove node
	analyzer.RemoveNode("node-1")
	assert.Equal(t, 0, analyzer.GetTotalNodes())
	assert.Equal(t, 0, analyzer.GetTotalZones())
}

func TestTopologyAnalyzerGetNodesInZone(t *testing.T) {
	analyzer := NewTopologyAnalyzer(DefaultTopologyConfig())

	// Add nodes to same zone
	node1 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-1",
			Labels: map[string]string{
				"topology.kubernetes.io/zone": "zone-a",
			},
		},
	}
	node2 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-2",
			Labels: map[string]string{
				"topology.kubernetes.io/zone": "zone-a",
			},
		},
	}

	analyzer.UpdateNode(node1)
	analyzer.UpdateNode(node2)

	nodes := analyzer.GetNodesInZone("zone-a")
	assert.Len(t, nodes, 2)
	assert.Contains(t, nodes, "node-1")
	assert.Contains(t, nodes, "node-2")
}

func TestTopologyAnalyzerClassifyBlastRadiusNode(t *testing.T) {
	analyzer := NewTopologyAnalyzer(DefaultTopologyConfig())

	// Add 10 nodes
	nodeNames := []string{"node-a", "node-b", "node-c", "node-d", "node-e", "node-f", "node-g", "node-h", "node-i", "node-j"}
	for _, name := range nodeNames {
		node := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Labels: map[string]string{
					"topology.kubernetes.io/zone": "zone-a",
				},
			},
		}
		analyzer.UpdateNode(node)
	}

	// Single node affected
	affected := []string{"node-b"}
	radius := analyzer.ClassifyBlastRadius(affected)

	assert.Equal(t, BlastRadiusNode, radius)
}

func TestTopologyAnalyzerClassifyBlastRadiusZone(t *testing.T) {
	analyzer := NewTopologyAnalyzer(DefaultTopologyConfig())

	// Add nodes to multiple zones
	nodeNames := []string{"node-a", "node-b", "node-c"}
	for _, name := range nodeNames {
		node := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Labels: map[string]string{
					"topology.kubernetes.io/zone": "zone-a",
				},
			},
		}
		analyzer.UpdateNode(node)
	}

	// Multiple nodes affected
	affected := []string{"node-a", "node-b"}
	radius := analyzer.ClassifyBlastRadius(affected)

	assert.Equal(t, BlastRadiusZone, radius)
}

func TestTopologyAnalyzerClassifyBlastRadiusCluster(t *testing.T) {
	analyzer := NewTopologyAnalyzer(DefaultTopologyConfig())

	// Add 10 nodes across 3 zones
	nodes := []struct {
		name string
		zone string
	}{
		{"node-a", "zone-a"}, {"node-b", "zone-a"}, {"node-c", "zone-a"},
		{"node-d", "zone-b"}, {"node-e", "zone-b"}, {"node-f", "zone-b"},
		{"node-g", "zone-c"}, {"node-h", "zone-c"}, {"node-i", "zone-c"}, {"node-j", "zone-c"},
	}
	for _, n := range nodes {
		node := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: n.name,
				Labels: map[string]string{
					"topology.kubernetes.io/zone": n.zone,
				},
			},
		}
		analyzer.UpdateNode(node)
	}

	// More than 30% affected (4 out of 10) across multiple zones
	affected := []string{"node-a", "node-d", "node-g", "node-j"}
	radius := analyzer.ClassifyBlastRadius(affected)

	assert.Equal(t, BlastRadiusCluster, radius)
}

func TestTopologyAnalyzerClassifyBlastRadiusEmpty(t *testing.T) {
	analyzer := NewTopologyAnalyzer(DefaultTopologyConfig())

	affected := []string{"node-1"}
	radius := analyzer.ClassifyBlastRadius(affected)

	assert.Equal(t, BlastRadiusNode, radius)
}

func TestTopologyAnalyzerAnalyzeNetworkModePodOnly(t *testing.T) {
	analyzer := NewTopologyAnalyzer(DefaultTopologyConfig())

	podFailures := []string{"node-1", "node-2"}
	hostFailures := []string{}

	analysis := analyzer.AnalyzeNetworkMode(podFailures, hostFailures)

	assert.True(t, analysis.PodNetworkOnly)
	assert.False(t, analysis.HostNetworkOnly)
	assert.False(t, analysis.BothNetworksFailing)
	assert.Contains(t, analysis.Conclusion, "CNI")
}

func TestTopologyAnalyzerAnalyzeNetworkModeHostOnly(t *testing.T) {
	analyzer := NewTopologyAnalyzer(DefaultTopologyConfig())

	podFailures := []string{}
	hostFailures := []string{"node-1", "node-2"}

	analysis := analyzer.AnalyzeNetworkMode(podFailures, hostFailures)

	assert.False(t, analysis.PodNetworkOnly)
	assert.True(t, analysis.HostNetworkOnly)
	assert.False(t, analysis.BothNetworksFailing)
	assert.Contains(t, analysis.Conclusion, "Node routing")
}

func TestTopologyAnalyzerAnalyzeNetworkModeBoth(t *testing.T) {
	analyzer := NewTopologyAnalyzer(DefaultTopologyConfig())

	podFailures := []string{"node-1", "node-2"}
	hostFailures := []string{"node-1", "node-2"}

	analysis := analyzer.AnalyzeNetworkMode(podFailures, hostFailures)

	assert.False(t, analysis.PodNetworkOnly)
	assert.False(t, analysis.HostNetworkOnly)
	assert.True(t, analysis.BothNetworksFailing)
	assert.Contains(t, analysis.Conclusion, "Target or DNS")
}

func TestTopologyAnalyzerGetTopologyStats(t *testing.T) {
	analyzer := NewTopologyAnalyzer(DefaultTopologyConfig())

	// Add nodes
	for i := 0; i < 3; i++ {
		node := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-" + string(rune('0'+i)),
				Labels: map[string]string{
					"topology.kubernetes.io/zone": "zone-a",
				},
			},
		}
		analyzer.UpdateNode(node)
	}

	stats := analyzer.GetTopologyStats()

	assert.Equal(t, 3, stats.TotalNodes)
	assert.Equal(t, 1, stats.TotalZones)
	assert.NotNil(t, stats.Zones)
}

func TestTopologyAnalyzerMultipleZones(t *testing.T) {
	analyzer := NewTopologyAnalyzer(DefaultTopologyConfig())

	// Add nodes to different zones
	zones := []string{"zone-a", "zone-b", "zone-c"}
	for i, zone := range zones {
		node := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-" + string(rune('0'+i)),
				Labels: map[string]string{
					"topology.kubernetes.io/zone": zone,
				},
			},
		}
		analyzer.UpdateNode(node)
	}

	assert.Equal(t, 3, analyzer.GetTotalNodes())
	assert.Equal(t, 3, analyzer.GetTotalZones())

	// All zones affected should be cluster-wide
	affected := []string{"node-0", "node-1", "node-2"}
	radius := analyzer.ClassifyBlastRadius(affected)

	assert.Equal(t, BlastRadiusCluster, radius)
}
