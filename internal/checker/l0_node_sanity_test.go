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
	"os"
	"path/filepath"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	k8swatchv1 "github.com/k8swatch/k8s-monitor/api/v1"
)

func TestNodeSanityCheckerCreation(t *testing.T) {
	checker := NewNodeSanityChecker(nil)

	if checker == nil {
		t.Fatal("Expected checker to be created with nil config")
	}

	if checker.Name() != "L0" {
		t.Errorf("Expected name L0, got %s", checker.Name())
	}
}

func TestNodeSanityCheckerWithConfig(t *testing.T) {
	config := &NodeSanityConfig{
		ClockSkewThreshold:  10 * time.Second,
		FDWarningThreshold:  70,
		FDCriticalThreshold: 90,
		ConntrackWarning:    70,
		ConntrackCritical:   90,
		ProcPath:            "/proc",
	}

	checker := NewNodeSanityChecker(config)

	if checker == nil {
		t.Fatal("Expected checker to be created")
	}

	if checker.config.FDWarningThreshold != 70 {
		t.Errorf("Expected FDWarningThreshold 70, got %d", checker.config.FDWarningThreshold)
	}
}

func TestNodeSanityCheckerEnabled(t *testing.T) {
	checker := NewNodeSanityChecker(nil)

	// Test with L0 enabled
	targetEnabled := &k8swatchv1.Target{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-target",
			Namespace: "default",
		},
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeHTTP,
			Layers: k8swatchv1.LayerConfig{
				L0NodeSanity: &k8swatchv1.NodeSanityConfig{
					LayerConfigBase: k8swatchv1.LayerConfigBase{
						Enabled: true,
					},
				},
			},
		},
	}

	if !checker.Enabled(targetEnabled) {
		t.Errorf("Expected checker to be enabled")
	}

	// Test with L0 disabled
	targetDisabled := &k8swatchv1.Target{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-target",
			Namespace: "default",
		},
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeHTTP,
			Layers: k8swatchv1.LayerConfig{
				L0NodeSanity: &k8swatchv1.NodeSanityConfig{
					LayerConfigBase: k8swatchv1.LayerConfigBase{
						Enabled: false,
					},
				},
			},
		},
	}

	if checker.Enabled(targetDisabled) {
		t.Errorf("Expected checker to be disabled")
	}

	// Test with L0 not configured
	targetNotConfigured := &k8swatchv1.Target{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-target",
			Namespace: "default",
		},
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeHTTP,
		},
	}

	if checker.Enabled(targetNotConfigured) {
		t.Errorf("Expected checker to be disabled when not configured")
	}
}

func TestNodeSanityCheckerCheck(t *testing.T) {
	checker := NewNodeSanityChecker(nil)

	target := &k8swatchv1.Target{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-target",
			Namespace: "default",
		},
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeHTTP,
			Layers: k8swatchv1.LayerConfig{
				L0NodeSanity: &k8swatchv1.NodeSanityConfig{
					LayerConfigBase: k8swatchv1.LayerConfigBase{
						Enabled: true,
					},
				},
			},
		},
	}

	ctx := context.Background()
	result, err := checker.Check(ctx, target)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// On most systems, the check should succeed (or be skipped if /proc is not accessible)
	// We just verify that the result is valid
	if result == nil {
		t.Fatal("Expected result to be returned")
	}

	if result.DurationMs < 0 {
		t.Errorf("Expected non-negative duration, got %d", result.DurationMs)
	}
}

func TestNodeSanityCheckerWithMockProc(t *testing.T) {
	// Create a temporary directory with mock proc files
	tmpDir := t.TempDir()
	procDir := filepath.Join(tmpDir, "proc")

	// Create mock proc files
	_ = os.MkdirAll(filepath.Join(procDir, "sys", "fs"), 0755)
	_ = os.MkdirAll(filepath.Join(procDir, "sys", "net", "netfilter"), 0755)
	_ = os.MkdirAll(filepath.Join(procDir, "net", "ipv4"), 0755)

	// Mock file-nr: allocated free max
	_ = os.WriteFile(filepath.Join(procDir, "sys", "fs", "file-nr"), []byte("1000 0 65536\n"), 0600)

	// Mock conntrack
	_ = os.WriteFile(filepath.Join(procDir, "sys", "net", "netfilter", "nf_conntrack_count"), []byte("1000\n"), 0600)
	_ = os.WriteFile(filepath.Join(procDir, "sys", "net", "netfilter", "nf_conntrack_max"), []byte("65536\n"), 0600)

	// Mock port range
	_ = os.WriteFile(filepath.Join(procDir, "net", "ipv4", "ip_local_port_range"), []byte("32768 60999\n"), 0600)

	config := &NodeSanityConfig{
		ProcPath:            procDir,
		FDWarningThreshold:  80,
		FDCriticalThreshold: 95,
		ConntrackWarning:    80,
		ConntrackCritical:   95,
	}

	checker := NewNodeSanityChecker(config)

	target := &k8swatchv1.Target{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-target",
			Namespace: "default",
		},
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeHTTP,
			Layers: k8swatchv1.LayerConfig{
				L0NodeSanity: &k8swatchv1.NodeSanityConfig{
					LayerConfigBase: k8swatchv1.LayerConfigBase{
						Enabled: true,
					},
				},
			},
		},
	}

	ctx := context.Background()
	result, err := checker.Check(ctx, target)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result == nil {
		t.Fatal("Expected result to be returned")
	}

	if !result.Success {
		t.Errorf("Expected success, got failure: %s", result.FailureMessage)
	}
}

func TestNodeSanityCheckerHighFDUsage(t *testing.T) {
	// Create a temporary directory with mock proc files
	tmpDir := t.TempDir()
	procDir := filepath.Join(tmpDir, "proc")

	_ = os.MkdirAll(filepath.Join(procDir, "sys", "fs"), 0755)

	// Mock file-nr with high usage: 62000 allocated out of 65536 (94.6%)
	_ = os.WriteFile(filepath.Join(procDir, "sys", "fs", "file-nr"), []byte("62000 0 65536\n"), 0600)

	config := &NodeSanityConfig{
		ProcPath:            procDir,
		FDWarningThreshold:  80,
		FDCriticalThreshold: 95,
	}

	checker := NewNodeSanityChecker(config)

	// The check should succeed but log a warning
	// (actual warning depends on logger configuration)
	target := &k8swatchv1.Target{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-target",
			Namespace: "default",
		},
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeHTTP,
			Layers: k8swatchv1.LayerConfig{
				L0NodeSanity: &k8swatchv1.NodeSanityConfig{
					LayerConfigBase: k8swatchv1.LayerConfigBase{
						Enabled: true,
					},
				},
			},
		},
	}

	ctx := context.Background()
	result, err := checker.Check(ctx, target)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Should still succeed since we're below critical threshold
	if !result.Success {
		t.Errorf("Expected success, got failure: %s", result.FailureMessage)
	}
}

func TestCheckClockSkew(t *testing.T) {
	checker := NewNodeSanityChecker(nil)

	err := checker.checkClockSkew()

	// This should pass on systems with correct time
	if err != nil {
		t.Logf("Clock skew check returned: %v", err)
		// Don't fail the test - clock skew check is optional
	}
}
