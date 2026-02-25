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
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	k8swatchv1 "github.com/k8swatch/k8s-monitor/api/v1"
)

// NodeSanityChecker implements L0 node sanity checks
type NodeSanityChecker struct {
	config *NodeSanityConfig
}

// NodeSanityConfig holds configuration for node sanity checks
type NodeSanityConfig struct {
	ClockSkewThreshold  time.Duration
	FDWarningThreshold  int32
	FDCriticalThreshold int32
	ConntrackWarning    int32
	ConntrackCritical   int32
	ProcPath            string
}

// DefaultNodeSanityConfig returns the default node sanity configuration
func DefaultNodeSanityConfig() *NodeSanityConfig {
	return &NodeSanityConfig{
		ClockSkewThreshold:  5 * time.Second,
		FDWarningThreshold:  80,
		FDCriticalThreshold: 95,
		ConntrackWarning:    80,
		ConntrackCritical:   95,
		ProcPath:            "/proc",
	}
}

// NewNodeSanityChecker creates a new node sanity checker
func NewNodeSanityChecker(config *NodeSanityConfig) *NodeSanityChecker {
	if config == nil {
		config = DefaultNodeSanityConfig()
	}
	return &NodeSanityChecker{config: config}
}

// Name returns the layer name
func (c *NodeSanityChecker) Name() string {
	return "L0"
}

// Enabled returns whether this layer is enabled for the target
func (c *NodeSanityChecker) Enabled(target *k8swatchv1.Target) bool {
	// L0 is enabled if the target has L0_nodeSanity configured
	return target.Spec.Layers.L0NodeSanity != nil && target.Spec.Layers.L0NodeSanity.Enabled
}

// Check executes the L0 node sanity check
func (c *NodeSanityChecker) Check(ctx context.Context, target *k8swatchv1.Target) (*LayerResult, error) {
	startTime := time.Now()

	// Check file descriptors
	if err := c.checkFileDescriptors(); err != nil {
		return LayerResultError(err, string(k8swatchv1.FailureCodeFDExhausted), time.Since(startTime).Milliseconds()), nil
	}

	// Check conntrack
	if err := c.checkConntrack(); err != nil {
		return LayerResultError(err, string(k8swatchv1.FailureCodeConntrackPressure), time.Since(startTime).Milliseconds()), nil
	}

	// Check ephemeral ports
	if err := c.checkEphemeralPorts(); err != nil {
		return LayerResultError(err, string(k8swatchv1.FailureCodeEphemeralPortsLow), time.Since(startTime).Milliseconds()), nil
	}

	// Check clock skew (optional, requires NTP server)
	if target.Spec.Layers.L0NodeSanity.ClockSkew != nil && target.Spec.Layers.L0NodeSanity.ClockSkew.Enabled {
		if err := c.checkClockSkew(target.Spec.Layers.L0NodeSanity.ClockSkew); err != nil {
			return LayerResultError(err, string(k8swatchv1.FailureCodeClockSkew), time.Since(startTime).Milliseconds()), nil
		}
	}

	return LayerResultSuccess(time.Since(startTime).Milliseconds()), nil
}

// checkFileDescriptors checks file descriptor usage
func (c *NodeSanityChecker) checkFileDescriptors() error {
	// Read /proc/sys/fs/file-nr
	// Format: allocated free max
	path := fmt.Sprintf("%s/sys/fs/file-nr", c.config.ProcPath)
	data, err := os.ReadFile(path)
	if err != nil {
		// If we can't read the file, skip this check (not running on Linux or no host access)
		log.Info("Cannot read file-nr, skipping FD check", "path", path, "error", err)
		return nil
	}

	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return fmt.Errorf("unexpected file-nr format: %s", string(data))
	}

	allocated, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil {
		return fmt.Errorf("failed to parse allocated FDs: %w", err)
	}

	maxFDs, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil {
		return fmt.Errorf("failed to parse max FDs: %w", err)
	}

	if maxFDs == 0 {
		return nil
	}

	usagePercent := int32((allocated * 100) / maxFDs) // nolint:gosec // usage percent will not overflow in practice

	if usagePercent >= c.config.FDCriticalThreshold {
		return fmt.Errorf("file descriptors at %d%% (critical threshold: %d%%)", usagePercent, c.config.FDCriticalThreshold)
	}

	if usagePercent >= c.config.FDWarningThreshold {
		log.Info("File descriptors warning", "usage", usagePercent, "threshold", c.config.FDWarningThreshold)
	}

	return nil
}

// checkConntrack checks conntrack table usage
func (c *NodeSanityChecker) checkConntrack() error {
	// Read /proc/sys/net/netfilter/nf_conntrack_count
	countPath := fmt.Sprintf("%s/sys/net/netfilter/nf_conntrack_count", c.config.ProcPath)
	countData, err := os.ReadFile(countPath)
	if err != nil {
		// Conntrack may not be available on all systems
		log.Info("Cannot read conntrack count, skipping", "path", countPath, "error", err)
		return nil
	}

	conntrackCount, err := strconv.ParseInt(strings.TrimSpace(string(countData)), 10, 64)
	if err != nil {
		return fmt.Errorf("failed to parse conntrack count: %w", err)
	}

	// Read /proc/sys/net/netfilter/nf_conntrack_max
	maxPath := fmt.Sprintf("%s/sys/net/netfilter/nf_conntrack_max", c.config.ProcPath)
	maxData, err := os.ReadFile(maxPath)
	if err != nil {
		log.Info("Cannot read conntrack max, skipping", "path", maxPath, "error", err)
		return nil
	}

	conntrackMax, err := strconv.ParseInt(strings.TrimSpace(string(maxData)), 10, 64)
	if err != nil {
		return fmt.Errorf("failed to parse conntrack max: %w", err)
	}

	if conntrackMax == 0 {
		return nil
	}

	usagePercent := int32((conntrackCount * 100) / conntrackMax) // nolint:gosec // usage percent will not overflow in practice

	if usagePercent >= c.config.ConntrackCritical {
		return fmt.Errorf("conntrack at %d%% (critical threshold: %d%%)", usagePercent, c.config.ConntrackCritical)
	}

	if usagePercent >= c.config.ConntrackWarning {
		log.Info("Conntrack warning", "usage", usagePercent, "threshold", c.config.ConntrackWarning)
	}

	return nil
}

// checkEphemeralPorts checks ephemeral port usage
func (c *NodeSanityChecker) checkEphemeralPorts() error {
	// Read /proc/sys/net/ipv4/ip_local_port_range
	path := fmt.Sprintf("%s/sys/net/ipv4/ip_local_port_range", c.config.ProcPath)
	data, err := os.ReadFile(path)
	if err != nil {
		log.Info("Cannot read ip_local_port_range, skipping", "path", path, "error", err)
		return nil
	}

	fields := strings.Fields(string(data))
	if len(fields) < 2 {
		return fmt.Errorf("unexpected ip_local_port_range format: %s", string(data))
	}

	minPort, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil {
		return fmt.Errorf("failed to parse min port: %w", err)
	}

	maxPort, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return fmt.Errorf("failed to parse max port: %w", err)
	}

	totalPorts := maxPort - minPort + 1

	// Count allocated ports from /proc/net/tcp and /proc/net/tcp6
	allocatedPorts, err := c.countAllocatedPorts()
	if err != nil {
		log.Info("Cannot count allocated ports, skipping", "error", err)
		return nil
	}

	if totalPorts == 0 {
		return nil
	}

	usagePercent := int32((allocatedPorts * 100) / totalPorts) // nolint:gosec // usage percent will not overflow in practice

	// Warning threshold is 80% by default
	if usagePercent >= 80 {
		log.Info("Ephemeral ports warning", "usage", usagePercent, "allocated", allocatedPorts, "total", totalPorts)
	}

	return nil
}

// countAllocatedPorts counts allocated TCP ports
func (c *NodeSanityChecker) countAllocatedPorts() (int64, error) {
	var count int64

	// Check IPv4
	if err := c.countPortsFromFile(fmt.Sprintf("%s/net/tcp", c.config.ProcPath), &count); err != nil {
		return count, err
	}

	// Check IPv6
	if err := c.countPortsFromFile(fmt.Sprintf("%s/net/tcp6", c.config.ProcPath), &count); err != nil {
		return count, err
	}

	return count, nil
}

// countPortsFromFile counts ports from a /proc/net/tcp file
func (c *NodeSanityChecker) countPortsFromFile(path string, count *int64) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	// Skip header line
	scanner.Scan()

	for scanner.Scan() {
		*count++
	}

	return scanner.Err()
}

// checkClockSkew checks system clock skew
func (c *NodeSanityChecker) checkClockSkew(clockCfg *k8swatchv1.ClockSkewConfig) error {
	now := time.Now()

	// Check if time is reasonable (not in the past or far future)
	// This is a basic sanity check
	if now.Year() < 2020 || now.Year() > 2030 {
		return fmt.Errorf("system clock appears incorrect: year=%d", now.Year())
	}

	// If no NTP server is configured, keep basic sanity validation only.
	if clockCfg == nil || strings.TrimSpace(clockCfg.NTPServer) == "" {
		return nil
	}

	threshold := c.config.ClockSkewThreshold
	if clockCfg.Threshold != "" {
		if d, err := time.ParseDuration(clockCfg.Threshold); err == nil && d > 0 {
			threshold = d
		}
	}

	serverTime, err := queryNTPTime(clockCfg.NTPServer, 3*time.Second)
	if err != nil {
		return fmt.Errorf("failed to query NTP server %q: %w", clockCfg.NTPServer, err)
	}

	skew := now.Sub(serverTime)
	if math.Abs(skew.Seconds()) > threshold.Seconds() {
		return fmt.Errorf("clock skew too high: %s (threshold: %s)", skew.Round(time.Millisecond), threshold)
	}

	return nil
}

func queryNTPTime(server string, timeout time.Duration) (time.Time, error) {
	addr := server
	if !strings.Contains(server, ":") {
		addr = net.JoinHostPort(server, "123")
	}

	conn, err := net.DialTimeout("udp", addr, timeout)
	if err != nil {
		return time.Time{}, err
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(timeout))

	req := make([]byte, 48)
	req[0] = 0x1B // LI=0, VN=3, Mode=3 (client)
	if _, err := conn.Write(req); err != nil {
		return time.Time{}, err
	}

	resp := make([]byte, 48)
	if _, err := conn.Read(resp); err != nil {
		return time.Time{}, err
	}

	seconds := binary.BigEndian.Uint32(resp[40:44])
	fraction := binary.BigEndian.Uint32(resp[44:48])

	// NTP epoch starts at 1900-01-01.
	const ntpToUnix = 2208988800
	unixSeconds := int64(seconds) - ntpToUnix
	nanos := (int64(fraction) * 1e9) >> 32 // fixed-point fraction to nanoseconds

	return time.Unix(unixSeconds, nanos).UTC(), nil
}
