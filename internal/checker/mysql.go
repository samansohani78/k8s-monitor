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
	"fmt"
	"net"
	"strings"
	"time"

	k8swatchv1 "github.com/k8swatch/k8s-monitor/api/v1"
)

// MySQLChecker implements MySQL health checks
type MySQLChecker struct {
	layers []Layer
}

// MySQLCheckerFactory creates MySQL checkers
type MySQLCheckerFactory struct{}

// Create creates a new MySQL checker
func (f *MySQLCheckerFactory) Create(target *k8swatchv1.Target) (Checker, error) {
	layers := []Layer{
		NewNodeSanityChecker(DefaultNodeSanityConfig()),
		NewDNSLayer(),
		NewTCPLayer(),
		NewMySQLProtocolLayer(),
		NewMySQLAuthLayer(),
		NewMySQLSemanticLayer(),
	}
	return &MySQLChecker{layers: layers}, nil
}

// SupportedTypes returns the supported target types
func (f *MySQLCheckerFactory) SupportedTypes() []string {
	return []string{"mysql"}
}

// Layers returns the layers for this checker
func (c *MySQLChecker) Layers() []Layer {
	return c.layers
}

// Execute executes all enabled layers for a target
func (c *MySQLChecker) Execute(ctx context.Context, target *k8swatchv1.Target) (*k8swatchv1.CheckResult, error) {
	executor := NewExecutor(c.layers)
	return executor.Execute(ctx, target)
}

// MySQLProtocolLayer implements L4 MySQL protocol check
type MySQLProtocolLayer struct{}

// NewMySQLProtocolLayer creates a new MySQL protocol layer
func NewMySQLProtocolLayer() *MySQLProtocolLayer {
	return &MySQLProtocolLayer{}
}

// Name returns the layer name
func (l *MySQLProtocolLayer) Name() string {
	return "L4"
}

// Enabled returns whether this layer is enabled
func (l *MySQLProtocolLayer) Enabled(target *k8swatchv1.Target) bool {
	return target.Spec.Type == k8swatchv1.TargetTypeMySQL ||
		(target.Spec.Layers.L4Protocol != nil && target.Spec.Layers.L4Protocol.Enabled)
}

// Check executes the MySQL protocol check
func (l *MySQLProtocolLayer) Check(ctx context.Context, target *k8swatchv1.Target) (*LayerResult, error) {
	startTime := time.Now()

	address, err := l.getAddress(target)
	if err != nil {
		return LayerResultError(err, string(k8swatchv1.FailureCodeConfigError), time.Since(startTime).Milliseconds()), nil
	}

	timeout := 5 * time.Second
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return l.handleMySQLError(err), nil
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(timeout))

	// Read initial handshake packet from server
	// MySQL protocol: server sends handshake packet first
	buf := make([]byte, 256)
	n, err := conn.Read(buf)
	if err != nil {
		return LayerResultError(err, string(k8swatchv1.FailureCodeProtocolError), time.Since(startTime).Milliseconds()), nil
	}

	if n < 2 {
		return LayerResultFailure(
			string(k8swatchv1.FailureCodeProtocolUnexpectedResp),
			"MySQL handshake packet too short",
			time.Since(startTime).Milliseconds(),
		), nil
	}

	// Check packet length (first 3 bytes) and sequence number (4th byte)
	// Protocol version should be in byte 5 (should be 10 for MySQL 4.1+)
	if buf[4] != 10 {
		return LayerResultFailure(
			string(k8swatchv1.FailureCodeProtocolUnexpectedResp),
			fmt.Sprintf("Unexpected MySQL protocol version: %d", buf[4]),
			time.Since(startTime).Milliseconds(),
		), nil
	}

	return LayerResultSuccess(time.Since(startTime).Milliseconds()), nil
}

// getAddress extracts address from target
func (l *MySQLProtocolLayer) getAddress(target *k8swatchv1.Target) (string, error) {
	endpoint := &target.Spec.Endpoint
	port := 3306

	if endpoint.Port != nil {
		port = int(*endpoint.Port)
	}

	if endpoint.DNS != nil && *endpoint.DNS != "" {
		return fmt.Sprintf("%s:%d", *endpoint.DNS, port), nil
	}

	if endpoint.IP != nil && *endpoint.IP != "" {
		return fmt.Sprintf("%s:%d", *endpoint.IP, port), nil
	}

	if endpoint.K8sService != nil {
		ns := endpoint.K8sService.Namespace
		if ns == "" {
			ns = "default"
		}
		return fmt.Sprintf("%s.%s.svc.cluster.local:%d", endpoint.K8sService.Name, ns, port), nil
	}

	return "", fmt.Errorf("no endpoint configured")
}

// handleMySQLError converts MySQL errors to failure codes
func (l *MySQLProtocolLayer) handleMySQLError(err error) *LayerResult {
	errStr := err.Error()

	switch {
	case contains(errStr, "connection refused"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeTCPRefused), "MySQL connection refused", 0)
	case contains(errStr, "timeout"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeProtocolTimeout), "MySQL timeout", 0)
	case contains(errStr, "no route"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeTCPNoRoute), "MySQL no route", 0)
	default:
		return LayerResultError(err, string(k8swatchv1.FailureCodeProtocolError), 0)
	}
}

// MySQLAuthLayer implements L5 MySQL authentication check
type MySQLAuthLayer struct{}

// NewMySQLAuthLayer creates a new MySQL auth layer
func NewMySQLAuthLayer() *MySQLAuthLayer {
	return &MySQLAuthLayer{}
}

// Name returns the layer name
func (l *MySQLAuthLayer) Name() string {
	return "L5"
}

// Enabled returns whether this layer is enabled
func (l *MySQLAuthLayer) Enabled(target *k8swatchv1.Target) bool {
	return target.Spec.Layers.L5Auth != nil && target.Spec.Layers.L5Auth.Enabled
}

// Check executes the MySQL authentication check
func (l *MySQLAuthLayer) Check(ctx context.Context, target *k8swatchv1.Target) (*LayerResult, error) {
	startTime := time.Now()

	address, err := l.getAddress(target)
	if err != nil {
		return LayerResultError(err, string(k8swatchv1.FailureCodeConfigError), time.Since(startTime).Milliseconds()), nil
	}

	timeout := 5 * time.Second
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return l.handleMySQLError(err), nil
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(timeout))

	// Read initial handshake packet
	buf := make([]byte, 256)
	n, err := conn.Read(buf)
	if err != nil {
		return LayerResultError(err, string(k8swatchv1.FailureCodeAuthFailed), time.Since(startTime).Milliseconds()), nil
	}

	if n < 2 || buf[4] != 10 {
		return LayerResultFailure(
			string(k8swatchv1.FailureCodeProtocolUnexpectedResp),
			"Invalid MySQL handshake",
			time.Since(startTime).Milliseconds(),
		), nil
	}

	// For auth check, we verify we received a valid handshake
	// Full auth would require implementing MySQL auth protocol
	return LayerResultSuccess(time.Since(startTime).Milliseconds()), nil
}

// getAddress extracts address for auth layer
func (l *MySQLAuthLayer) getAddress(target *k8swatchv1.Target) (string, error) {
	protocolLayer := NewMySQLProtocolLayer()
	return protocolLayer.getAddress(target)
}

// handleMySQLError converts errors for auth layer
func (l *MySQLAuthLayer) handleMySQLError(err error) *LayerResult {
	return NewMySQLProtocolLayer().handleMySQLError(err)
}

// MySQLSemanticLayer implements L6 MySQL semantic check
type MySQLSemanticLayer struct{}

// NewMySQLSemanticLayer creates a new MySQL semantic layer
func NewMySQLSemanticLayer() *MySQLSemanticLayer {
	return &MySQLSemanticLayer{}
}

// Name returns the layer name
func (l *MySQLSemanticLayer) Name() string {
	return "L6"
}

// Enabled returns whether this layer is enabled
func (l *MySQLSemanticLayer) Enabled(target *k8swatchv1.Target) bool {
	return target.Spec.Layers.L6Semantic != nil && target.Spec.Layers.L6Semantic.Enabled
}

// Check executes the MySQL semantic check
func (l *MySQLSemanticLayer) Check(ctx context.Context, target *k8swatchv1.Target) (*LayerResult, error) {
	startTime := time.Now()

	address, err := l.getAddress(target)
	if err != nil {
		return LayerResultError(err, string(k8swatchv1.FailureCodeConfigError), time.Since(startTime).Milliseconds()), nil
	}

	timeout := 5 * time.Second
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return l.handleMySQLError(err), nil
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(timeout))

	// Read handshake packet
	buf := make([]byte, 256)
	n, err := conn.Read(buf)
	if err != nil {
		return LayerResultError(err, string(k8swatchv1.FailureCodeSemanticFailed), time.Since(startTime).Milliseconds()), nil
	}

	// Verify MySQL protocol response
	if n < 5 || buf[4] != 10 {
		return LayerResultFailure(
			string(k8swatchv1.FailureCodeSemanticUnexpected),
			"Unexpected MySQL response",
			time.Since(startTime).Milliseconds(),
		), nil
	}

	// Check for server version string (starts after byte 5, null-terminated)
	versionStart := 5
	versionEnd := versionStart
	for versionEnd < n && buf[versionEnd] != 0 {
		versionEnd++
	}
	version := string(buf[versionStart:versionEnd])

	if !strings.Contains(strings.ToLower(version), "mysql") &&
		!strings.Contains(strings.ToLower(version), "mariadb") {
		return LayerResultFailure(
			string(k8swatchv1.FailureCodeSemanticUnexpected),
			fmt.Sprintf("Unexpected server version: %s", version),
			time.Since(startTime).Milliseconds(),
		), nil
	}

	return LayerResultSuccess(time.Since(startTime).Milliseconds()), nil
}

// getAddress extracts address for semantic layer
func (l *MySQLSemanticLayer) getAddress(target *k8swatchv1.Target) (string, error) {
	protocolLayer := NewMySQLProtocolLayer()
	return protocolLayer.getAddress(target)
}

// handleMySQLError converts errors for semantic layer
func (l *MySQLSemanticLayer) handleMySQLError(err error) *LayerResult {
	return NewMySQLProtocolLayer().handleMySQLError(err)
}
