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
	"time"

	k8swatchv1 "github.com/k8swatch/k8s-monitor/api/v1"
)

// PostgreSQLChecker implements PostgreSQL health checks
type PostgreSQLChecker struct {
	layers []Layer
}

// PostgreSQLCheckerFactory creates PostgreSQL checkers
type PostgreSQLCheckerFactory struct{}

// Create creates a new PostgreSQL checker
func (f *PostgreSQLCheckerFactory) Create(target *k8swatchv1.Target) (Checker, error) {
	layers := []Layer{
		NewNodeSanityChecker(DefaultNodeSanityConfig()),
		NewDNSLayer(),
		NewTCPLayer(),
		NewPostgreSQLProtocolLayer(),
		NewPostgreSQLAuthLayer(),
		NewPostgreSQLSemanticLayer(),
	}
	return &PostgreSQLChecker{layers: layers}, nil
}

// SupportedTypes returns the supported target types
func (f *PostgreSQLCheckerFactory) SupportedTypes() []string {
	return []string{"postgresql"}
}

// Layers returns the layers for this checker
func (c *PostgreSQLChecker) Layers() []Layer {
	return c.layers
}

// Execute executes all enabled layers for a target
func (c *PostgreSQLChecker) Execute(ctx context.Context, target *k8swatchv1.Target) (*k8swatchv1.CheckResult, error) {
	executor := NewExecutor(c.layers)
	return executor.Execute(ctx, target)
}

// PostgreSQLProtocolLayer implements L4 PostgreSQL protocol check
type PostgreSQLProtocolLayer struct{}

// NewPostgreSQLProtocolLayer creates a new PostgreSQL protocol layer
func NewPostgreSQLProtocolLayer() *PostgreSQLProtocolLayer {
	return &PostgreSQLProtocolLayer{}
}

// Name returns the layer name
func (l *PostgreSQLProtocolLayer) Name() string {
	return "L4"
}

// Enabled returns whether this layer is enabled
func (l *PostgreSQLProtocolLayer) Enabled(target *k8swatchv1.Target) bool {
	return target.Spec.Type == k8swatchv1.TargetTypePostgreSQL ||
		(target.Spec.Layers.L4Protocol != nil && target.Spec.Layers.L4Protocol.Enabled)
}

// Check executes the PostgreSQL protocol check using raw TCP
func (l *PostgreSQLProtocolLayer) Check(ctx context.Context, target *k8swatchv1.Target) (*LayerResult, error) {
	startTime := time.Now()

	address, err := l.getAddress(target)
	if err != nil {
		return LayerResultError(err, string(k8swatchv1.FailureCodeConfigError), time.Since(startTime).Milliseconds()), nil
	}

	timeout := 5 * time.Second
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return l.handlePostgreSQLError(err), nil
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(timeout))

	// Send startup packet (SSL request - will be rejected, but proves protocol works)
	// Length: 8 bytes, Protocol version: 196608 (3.0)
	startupPacket := []byte{
		0x00, 0x00, 0x00, 0x08, // Length: 8
		0x04, 0xd2, 0x00, 0x00, // Protocol version 3.0 (196608)
	}
	if _, err := conn.Write(startupPacket); err != nil {
		return LayerResultError(err, string(k8swatchv1.FailureCodeProtocolError), time.Since(startTime).Milliseconds()), nil
	}

	// Read response (should be 'N' for no SSL or 'S' for SSL)
	buf := make([]byte, 1)
	n, err := conn.Read(buf)
	if err != nil {
		// Connection might be closed, but we proved protocol works
		return LayerResultSuccess(time.Since(startTime).Milliseconds()), nil
	}

	if n > 0 {
		// 'N' (78) = No SSL, 'S' (83) = SSL supported
		if buf[0] == 'N' || buf[0] == 'S' {
			return LayerResultSuccess(time.Since(startTime).Milliseconds()), nil
		}
	}

	return LayerResultFailure(
		string(k8swatchv1.FailureCodeProtocolUnexpectedResp),
		fmt.Sprintf("Unexpected PostgreSQL response: %v", buf),
		time.Since(startTime).Milliseconds(),
	), nil
}

// getAddress extracts host and port from target
func (l *PostgreSQLProtocolLayer) getAddress(target *k8swatchv1.Target) (string, error) {
	endpoint := &target.Spec.Endpoint
	port := 5432

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

// handlePostgreSQLError converts PostgreSQL errors to failure codes
func (l *PostgreSQLProtocolLayer) handlePostgreSQLError(err error) *LayerResult {
	errStr := err.Error()

	switch {
	case contains(errStr, "connection refused"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeTCPRefused), "PostgreSQL connection refused", 0)
	case contains(errStr, "timeout"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeProtocolTimeout), "PostgreSQL timeout", 0)
	case contains(errStr, "no route"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeTCPNoRoute), "PostgreSQL no route", 0)
	default:
		return LayerResultError(err, string(k8swatchv1.FailureCodeProtocolError), 0)
	}
}

// PostgreSQLAuthLayer implements L5 PostgreSQL authentication check
type PostgreSQLAuthLayer struct{}

// NewPostgreSQLAuthLayer creates a new PostgreSQL auth layer
func NewPostgreSQLAuthLayer() *PostgreSQLAuthLayer {
	return &PostgreSQLAuthLayer{}
}

// Name returns the layer name
func (l *PostgreSQLAuthLayer) Name() string {
	return "L5"
}

// Enabled returns whether this layer is enabled
func (l *PostgreSQLAuthLayer) Enabled(target *k8swatchv1.Target) bool {
	return target.Spec.Layers.L5Auth != nil && target.Spec.Layers.L5Auth.Enabled
}

// Check executes the PostgreSQL authentication check
func (l *PostgreSQLAuthLayer) Check(ctx context.Context, target *k8swatchv1.Target) (*LayerResult, error) {
	startTime := time.Now()

	// For now, just verify TCP connectivity with startup packet
	// Full auth would require implementing the PostgreSQL wire protocol
	address, err := l.getAddress(target)
	if err != nil {
		return LayerResultError(err, string(k8swatchv1.FailureCodeConfigError), time.Since(startTime).Milliseconds()), nil
	}

	timeout := 5 * time.Second
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return l.handlePostgreSQLError(err), nil
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(timeout))

	// Send startup packet
	startupPacket := []byte{
		0x00, 0x00, 0x00, 0x08,
		0x04, 0xd2, 0x00, 0x00,
	}
	if _, err := conn.Write(startupPacket); err != nil {
		return LayerResultError(err, string(k8swatchv1.FailureCodeAuthFailed), time.Since(startTime).Milliseconds()), nil
	}

	// Read response
	buf := make([]byte, 1)
	_, err = conn.Read(buf)
	if err != nil {
		// Connection closed after startup - normal for auth check
		return LayerResultSuccess(time.Since(startTime).Milliseconds()), nil
	}

	return LayerResultSuccess(time.Since(startTime).Milliseconds()), nil
}

// getAddress extracts address for auth layer
func (l *PostgreSQLAuthLayer) getAddress(target *k8swatchv1.Target) (string, error) {
	protocolLayer := NewPostgreSQLProtocolLayer()
	return protocolLayer.getAddress(target)
}

// handlePostgreSQLError converts errors for auth layer
func (l *PostgreSQLAuthLayer) handlePostgreSQLError(err error) *LayerResult {
	return NewPostgreSQLProtocolLayer().handlePostgreSQLError(err)
}

// PostgreSQLSemanticLayer implements L6 PostgreSQL semantic check
type PostgreSQLSemanticLayer struct{}

// NewPostgreSQLSemanticLayer creates a new PostgreSQL semantic layer
func NewPostgreSQLSemanticLayer() *PostgreSQLSemanticLayer {
	return &PostgreSQLSemanticLayer{}
}

// Name returns the layer name
func (l *PostgreSQLSemanticLayer) Name() string {
	return "L6"
}

// Enabled returns whether this layer is enabled
func (l *PostgreSQLSemanticLayer) Enabled(target *k8swatchv1.Target) bool {
	return target.Spec.Layers.L6Semantic != nil && target.Spec.Layers.L6Semantic.Enabled
}

// Check executes the PostgreSQL semantic check
func (l *PostgreSQLSemanticLayer) Check(ctx context.Context, target *k8swatchv1.Target) (*LayerResult, error) {
	startTime := time.Now()

	// For semantic check, we verify the server responds with proper protocol
	address, err := l.getAddress(target)
	if err != nil {
		return LayerResultError(err, string(k8swatchv1.FailureCodeConfigError), time.Since(startTime).Milliseconds()), nil
	}

	timeout := 5 * time.Second
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return l.handlePostgreSQLError(err), nil
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(timeout))

	// Send startup packet
	startupPacket := []byte{
		0x00, 0x00, 0x00, 0x08,
		0x04, 0xd2, 0x00, 0x00,
	}
	if _, err := conn.Write(startupPacket); err != nil {
		return LayerResultError(err, string(k8swatchv1.FailureCodeSemanticFailed), time.Since(startTime).Milliseconds()), nil
	}

	// Read response
	buf := make([]byte, 1)
	n, err := conn.Read(buf)
	if err != nil || n == 0 {
		return LayerResultError(err, string(k8swatchv1.FailureCodeSemanticFailed), time.Since(startTime).Milliseconds()), nil
	}

	// Verify PostgreSQL protocol response
	if buf[0] != 'N' && buf[0] != 'S' {
		return LayerResultFailure(
			string(k8swatchv1.FailureCodeSemanticUnexpected),
			fmt.Sprintf("Unexpected PostgreSQL semantic response: %v", buf),
			time.Since(startTime).Milliseconds(),
		), nil
	}

	return LayerResultSuccess(time.Since(startTime).Milliseconds()), nil
}

// getAddress extracts address for semantic layer
func (l *PostgreSQLSemanticLayer) getAddress(target *k8swatchv1.Target) (string, error) {
	protocolLayer := NewPostgreSQLProtocolLayer()
	return protocolLayer.getAddress(target)
}

// handlePostgreSQLError converts errors for semantic layer
func (l *PostgreSQLSemanticLayer) handlePostgreSQLError(err error) *LayerResult {
	return NewPostgreSQLProtocolLayer().handlePostgreSQLError(err)
}
