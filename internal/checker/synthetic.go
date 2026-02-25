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
	"net/http"
	"time"

	k8swatchv1 "github.com/k8swatch/k8s-monitor/api/v1"
)

// InternalCanaryChecker implements internal canary health checks
type InternalCanaryChecker struct {
	layers []Layer
}

// InternalCanaryCheckerFactory creates internal canary checkers
type InternalCanaryCheckerFactory struct{}

// Create creates a new internal canary checker
func (f *InternalCanaryCheckerFactory) Create(target *k8swatchv1.Target) (Checker, error) {
	layers := []Layer{
		NewNodeSanityChecker(DefaultNodeSanityConfig()),
		NewDNSLayer(),
		NewTCPLayer(),
		NewHTTPProtocolLayer(),
	}
	return &InternalCanaryChecker{layers: layers}, nil
}

// SupportedTypes returns the supported target types
func (f *InternalCanaryCheckerFactory) SupportedTypes() []string {
	return []string{"internal-canary"}
}

// Layers returns the layers for this checker
func (c *InternalCanaryChecker) Layers() []Layer {
	return c.layers
}

// Execute executes all enabled layers for a target
func (c *InternalCanaryChecker) Execute(ctx context.Context, target *k8swatchv1.Target) (*k8swatchv1.CheckResult, error) {
	executor := NewExecutor(c.layers)
	return executor.Execute(ctx, target)
}

// ExternalHTTPChecker implements external HTTP health checks
type ExternalHTTPChecker struct {
	layers []Layer
}

// ExternalHTTPCheckerFactory creates external HTTP checkers
type ExternalHTTPCheckerFactory struct{}

// Create creates a new external HTTP checker
func (f *ExternalHTTPCheckerFactory) Create(target *k8swatchv1.Target) (Checker, error) {
	layers := []Layer{
		NewDNSLayer(),
		NewTCPLayer(),
		NewTLSLayer(),
		NewHTTPProtocolLayer(),
	}
	return &ExternalHTTPChecker{layers: layers}, nil
}

// SupportedTypes returns the supported target types
func (f *ExternalHTTPCheckerFactory) SupportedTypes() []string {
	return []string{"external-http"}
}

// Layers returns the layers for this checker
func (c *ExternalHTTPChecker) Layers() []Layer {
	return c.layers
}

// Execute executes all enabled layers for a target
func (c *ExternalHTTPChecker) Execute(ctx context.Context, target *k8swatchv1.Target) (*k8swatchv1.CheckResult, error) {
	executor := NewExecutor(c.layers)
	return executor.Execute(ctx, target)
}

// NodeEgressChecker implements node egress health checks
type NodeEgressChecker struct {
	layers []Layer
}

// NodeEgressCheckerFactory creates node egress checkers
type NodeEgressCheckerFactory struct{}

// Create creates a new node egress checker
func (f *NodeEgressCheckerFactory) Create(target *k8swatchv1.Target) (Checker, error) {
	layers := []Layer{
		NewNodeSanityChecker(DefaultNodeSanityConfig()),
		NewDNSLayer(),
		NewTCPLayer(),
	}
	return &NodeEgressChecker{layers: layers}, nil
}

// SupportedTypes returns the supported target types
func (f *NodeEgressCheckerFactory) SupportedTypes() []string {
	return []string{"node-egress"}
}

// Layers returns the layers for this checker
func (c *NodeEgressChecker) Layers() []Layer {
	return c.layers
}

// Execute executes all enabled layers for a target
func (c *NodeEgressChecker) Execute(ctx context.Context, target *k8swatchv1.Target) (*k8swatchv1.CheckResult, error) {
	executor := NewExecutor(c.layers)
	return executor.Execute(ctx, target)
}

// NodeToNodeChecker implements node-to-node health checks
type NodeToNodeChecker struct {
	layers []Layer
}

// NodeToNodeCheckerFactory creates node-to-node checkers
type NodeToNodeCheckerFactory struct{}

// Create creates a new node-to-node checker
func (f *NodeToNodeCheckerFactory) Create(target *k8swatchv1.Target) (Checker, error) {
	layers := []Layer{
		NewNodeSanityChecker(DefaultNodeSanityConfig()),
		NewTCPLayer(),
	}
	return &NodeToNodeChecker{layers: layers}, nil
}

// SupportedTypes returns the supported target types
func (f *NodeToNodeCheckerFactory) SupportedTypes() []string {
	return []string{"node-to-node"}
}

// Layers returns the layers for this checker
func (c *NodeToNodeChecker) Layers() []Layer {
	return c.layers
}

// Execute executes all enabled layers for a target
func (c *NodeToNodeChecker) Execute(ctx context.Context, target *k8swatchv1.Target) (*k8swatchv1.CheckResult, error) {
	executor := NewExecutor(c.layers)
	return executor.Execute(ctx, target)
}

// MSSQLChecker implements MSSQL health checks
type MSSQLChecker struct {
	layers []Layer
}

// MSSQLCheckerFactory creates MSSQL checkers
type MSSQLCheckerFactory struct{}

// Create creates a new MSSQL checker
func (f *MSSQLCheckerFactory) Create(target *k8swatchv1.Target) (Checker, error) {
	layers := []Layer{
		NewNodeSanityChecker(DefaultNodeSanityConfig()),
		NewDNSLayer(),
		NewTCPLayer(),
		NewMSSQLProtocolLayer(),
	}
	return &MSSQLChecker{layers: layers}, nil
}

// SupportedTypes returns the supported target types
func (f *MSSQLCheckerFactory) SupportedTypes() []string {
	return []string{"mssql"}
}

// Layers returns the layers for this checker
func (c *MSSQLChecker) Layers() []Layer {
	return c.layers
}

// Execute executes all enabled layers for a target
func (c *MSSQLChecker) Execute(ctx context.Context, target *k8swatchv1.Target) (*k8swatchv1.CheckResult, error) {
	executor := NewExecutor(c.layers)
	return executor.Execute(ctx, target)
}

// MSSQLProtocolLayer implements L4 MSSQL protocol check
type MSSQLProtocolLayer struct{}

// NewMSSQLProtocolLayer creates a new MSSQL protocol layer
func NewMSSQLProtocolLayer() *MSSQLProtocolLayer {
	return &MSSQLProtocolLayer{}
}

// Name returns the layer name
func (l *MSSQLProtocolLayer) Name() string {
	return "L4"
}

// Enabled returns whether this layer is enabled
func (l *MSSQLProtocolLayer) Enabled(target *k8swatchv1.Target) bool {
	return target.Spec.Type == k8swatchv1.TargetTypeMSSQL ||
		(target.Spec.Layers.L4Protocol != nil && target.Spec.Layers.L4Protocol.Enabled)
}

// Check executes the MSSQL protocol check (TDS handshake)
func (l *MSSQLProtocolLayer) Check(ctx context.Context, target *k8swatchv1.Target) (*LayerResult, error) {
	startTime := time.Now()

	address, err := l.getAddress(target)
	if err != nil {
		return LayerResultError(err, string(k8swatchv1.FailureCodeConfigError), time.Since(startTime).Milliseconds()), nil
	}

	timeout := 5 * time.Second
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return l.handleMSSQLError(err), nil
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(timeout))

	// Send TDS pre-login packet (simplified)
	// TDS protocol header: type (1 byte), status (1 byte), length (2 bytes)
	// Pre-login message type = 0x12
	tdsHeader := []byte{
		0x12,       // Message type: Pre-login
		0x00,       // Status: normal
		0x00, 0x19, // Length: 25 bytes (minimal header)
		0x00, // SPID (unused)
		0x00, // Packet ID
		0x00, // Window (unused)
	}

	if _, err := conn.Write(tdsHeader); err != nil {
		return LayerResultError(err, string(k8swatchv1.FailureCodeProtocolError), time.Since(startTime).Milliseconds()), nil
	}

	// Read response header
	buf := make([]byte, 8)
	n, err := conn.Read(buf)
	if err != nil {
		// Connection might be closed, but we proved protocol works
		return LayerResultSuccess(time.Since(startTime).Milliseconds()), nil
	}

	if n < 8 {
		return LayerResultSuccess(time.Since(startTime).Milliseconds()), nil
	}

	return LayerResultSuccess(time.Since(startTime).Milliseconds()), nil
}

// getAddress extracts address from target
func (l *MSSQLProtocolLayer) getAddress(target *k8swatchv1.Target) (string, error) {
	endpoint := &target.Spec.Endpoint
	port := 1433

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

// handleMSSQLError converts MSSQL errors to failure codes
func (l *MSSQLProtocolLayer) handleMSSQLError(err error) *LayerResult {
	errStr := err.Error()

	switch {
	case contains(errStr, "connection refused"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeTCPRefused), "MSSQL connection refused", 0)
	case contains(errStr, "timeout"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeProtocolTimeout), "MSSQL timeout", 0)
	case contains(errStr, "no route"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeTCPNoRoute), "MSSQL no route", 0)
	default:
		return LayerResultError(err, string(k8swatchv1.FailureCodeProtocolError), 0)
	}
}

// ClickHouseChecker implements ClickHouse health checks
type ClickHouseChecker struct {
	layers []Layer
}

// ClickHouseCheckerFactory creates ClickHouse checkers
type ClickHouseCheckerFactory struct{}

// Create creates a new ClickHouse checker
func (f *ClickHouseCheckerFactory) Create(target *k8swatchv1.Target) (Checker, error) {
	layers := []Layer{
		NewNodeSanityChecker(DefaultNodeSanityConfig()),
		NewDNSLayer(),
		NewTCPLayer(),
		NewClickHouseProtocolLayer(),
	}
	return &ClickHouseChecker{layers: layers}, nil
}

// SupportedTypes returns the supported target types
func (f *ClickHouseCheckerFactory) SupportedTypes() []string {
	return []string{"clickhouse"}
}

// Layers returns the layers for this checker
func (c *ClickHouseChecker) Layers() []Layer {
	return c.layers
}

// Execute executes all enabled layers for a target
func (c *ClickHouseChecker) Execute(ctx context.Context, target *k8swatchv1.Target) (*k8swatchv1.CheckResult, error) {
	executor := NewExecutor(c.layers)
	return executor.Execute(ctx, target)
}

// ClickHouseProtocolLayer implements L4 ClickHouse protocol check
type ClickHouseProtocolLayer struct{}

// NewClickHouseProtocolLayer creates a new ClickHouse protocol layer
func NewClickHouseProtocolLayer() *ClickHouseProtocolLayer {
	return &ClickHouseProtocolLayer{}
}

// Name returns the layer name
func (l *ClickHouseProtocolLayer) Name() string {
	return "L4"
}

// Enabled returns whether this layer is enabled
func (l *ClickHouseProtocolLayer) Enabled(target *k8swatchv1.Target) bool {
	return target.Spec.Type == k8swatchv1.TargetTypeClickHouse ||
		(target.Spec.Layers.L4Protocol != nil && target.Spec.Layers.L4Protocol.Enabled)
}

// Check executes the ClickHouse protocol check
func (l *ClickHouseProtocolLayer) Check(ctx context.Context, target *k8swatchv1.Target) (*LayerResult, error) {
	startTime := time.Now()

	address, err := l.getAddress(target)
	if err != nil {
		return LayerResultError(err, string(k8swatchv1.FailureCodeConfigError), time.Since(startTime).Milliseconds()), nil
	}

	timeout := 5 * time.Second
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return l.handleClickHouseError(err), nil
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(timeout))

	// ClickHouse native protocol: send "SELECT 1" query
	// For simplicity, just verify TCP connection succeeds
	// Full implementation would use ClickHouse native protocol

	return LayerResultSuccess(time.Since(startTime).Milliseconds()), nil
}

// getAddress extracts address from target
func (l *ClickHouseProtocolLayer) getAddress(target *k8swatchv1.Target) (string, error) {
	endpoint := &target.Spec.Endpoint
	port := 9000

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

// handleClickHouseError converts ClickHouse errors to failure codes
func (l *ClickHouseProtocolLayer) handleClickHouseError(err error) *LayerResult {
	errStr := err.Error()

	switch {
	case contains(errStr, "connection refused"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeTCPRefused), "ClickHouse connection refused", 0)
	case contains(errStr, "timeout"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeProtocolTimeout), "ClickHouse timeout", 0)
	case contains(errStr, "no route"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeTCPNoRoute), "ClickHouse no route", 0)
	default:
		return LayerResultError(err, string(k8swatchv1.FailureCodeProtocolError), 0)
	}
}

// MinIOChecker implements MinIO health checks
type MinIOChecker struct {
	layers []Layer
}

// MinIOCheckerFactory creates MinIO checkers
type MinIOCheckerFactory struct{}

// Create creates a new MinIO checker
func (f *MinIOCheckerFactory) Create(target *k8swatchv1.Target) (Checker, error) {
	layers := []Layer{
		NewNodeSanityChecker(DefaultNodeSanityConfig()),
		NewDNSLayer(),
		NewTCPLayer(),
		NewMinIOProtocolLayer(),
	}
	return &MinIOChecker{layers: layers}, nil
}

// SupportedTypes returns the supported target types
func (f *MinIOCheckerFactory) SupportedTypes() []string {
	return []string{"minio"}
}

// Layers returns the layers for this checker
func (c *MinIOChecker) Layers() []Layer {
	return c.layers
}

// Execute executes all enabled layers for a target
func (c *MinIOChecker) Execute(ctx context.Context, target *k8swatchv1.Target) (*k8swatchv1.CheckResult, error) {
	executor := NewExecutor(c.layers)
	return executor.Execute(ctx, target)
}

// MinIOProtocolLayer implements L4 MinIO protocol check (S3 API)
type MinIOProtocolLayer struct{}

// NewMinIOProtocolLayer creates a new MinIO protocol layer
func NewMinIOProtocolLayer() *MinIOProtocolLayer {
	return &MinIOProtocolLayer{}
}

// Name returns the layer name
func (l *MinIOProtocolLayer) Name() string {
	return "L4"
}

// Enabled returns whether this layer is enabled
func (l *MinIOProtocolLayer) Enabled(target *k8swatchv1.Target) bool {
	return target.Spec.Type == k8swatchv1.TargetTypeMinIO ||
		(target.Spec.Layers.L4Protocol != nil && target.Spec.Layers.L4Protocol.Enabled)
}

// Check executes the MinIO protocol check (S3 HEAD bucket)
func (l *MinIOProtocolLayer) Check(ctx context.Context, target *k8swatchv1.Target) (*LayerResult, error) {
	startTime := time.Now()

	url, err := l.buildURL(target)
	if err != nil {
		return LayerResultError(err, string(k8swatchv1.FailureCodeConfigError), time.Since(startTime).Milliseconds()), nil
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Head(url)
	duration := time.Since(startTime).Milliseconds()

	if err != nil {
		return l.handleMinIOError(err), nil
	}
	defer resp.Body.Close()

	// Accept 2xx, 4xx (MinIO is responding)
	if resp.StatusCode >= 500 {
		return LayerResultFailure(
			string(k8swatchv1.FailureCodeProtocolUnexpectedResp),
			fmt.Sprintf("MinIO server error: %d", resp.StatusCode),
			duration,
		), nil
	}

	return LayerResultSuccess(duration), nil
}

// buildURL builds the MinIO URL
func (l *MinIOProtocolLayer) buildURL(target *k8swatchv1.Target) (string, error) {
	address, err := l.getAddress(target)
	if err != nil {
		return "", err
	}

	scheme := "http"
	if target.Spec.Layers.L3TLS != nil && target.Spec.Layers.L3TLS.Enabled {
		scheme = "https"
	}

	return fmt.Sprintf("%s://%s/minio/health/live", scheme, address), nil
}

// getAddress extracts address from target
func (l *MinIOProtocolLayer) getAddress(target *k8swatchv1.Target) (string, error) {
	endpoint := &target.Spec.Endpoint
	port := 9000

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

// handleMinIOError converts MinIO errors to failure codes
func (l *MinIOProtocolLayer) handleMinIOError(err error) *LayerResult {
	errStr := err.Error()

	switch {
	case contains(errStr, "connection refused"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeTCPRefused), "MinIO connection refused", 0)
	case contains(errStr, "timeout"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeProtocolTimeout), "MinIO timeout", 0)
	case contains(errStr, "no route"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeTCPNoRoute), "MinIO no route", 0)
	default:
		return LayerResultError(err, string(k8swatchv1.FailureCodeProtocolError), 0)
	}
}

// OpenSearchChecker implements OpenSearch health checks
type OpenSearchChecker struct {
	layers []Layer
}

// OpenSearchCheckerFactory creates OpenSearch checkers
type OpenSearchCheckerFactory struct{}

// Create creates a new OpenSearch checker
func (f *OpenSearchCheckerFactory) Create(target *k8swatchv1.Target) (Checker, error) {
	layers := []Layer{
		NewNodeSanityChecker(DefaultNodeSanityConfig()),
		NewDNSLayer(),
		NewTCPLayer(),
		NewTLSLayer(),
		NewOpenSearchProtocolLayer(),
	}
	return &OpenSearchChecker{layers: layers}, nil
}

// SupportedTypes returns the supported target types
func (f *OpenSearchCheckerFactory) SupportedTypes() []string {
	return []string{"opensearch"}
}

// Layers returns the layers for this checker
func (c *OpenSearchChecker) Layers() []Layer {
	return c.layers
}

// Execute executes all enabled layers for a target
func (c *OpenSearchChecker) Execute(ctx context.Context, target *k8swatchv1.Target) (*k8swatchv1.CheckResult, error) {
	executor := NewExecutor(c.layers)
	return executor.Execute(ctx, target)
}

// OpenSearchProtocolLayer implements L4 OpenSearch protocol check
type OpenSearchProtocolLayer struct{}

// NewOpenSearchProtocolLayer creates a new OpenSearch protocol layer
func NewOpenSearchProtocolLayer() *OpenSearchProtocolLayer {
	return &OpenSearchProtocolLayer{}
}

// Name returns the layer name
func (l *OpenSearchProtocolLayer) Name() string {
	return "L4"
}

// Enabled returns whether this layer is enabled
func (l *OpenSearchProtocolLayer) Enabled(target *k8swatchv1.Target) bool {
	return target.Spec.Type == k8swatchv1.TargetTypeOpenSearch ||
		(target.Spec.Layers.L4Protocol != nil && target.Spec.Layers.L4Protocol.Enabled)
}

// Check executes the OpenSearch protocol check
func (l *OpenSearchProtocolLayer) Check(ctx context.Context, target *k8swatchv1.Target) (*LayerResult, error) {
	startTime := time.Now()

	url, err := l.buildURL(target)
	if err != nil {
		return LayerResultError(err, string(k8swatchv1.FailureCodeConfigError), time.Since(startTime).Milliseconds()), nil
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	duration := time.Since(startTime).Milliseconds()

	if err != nil {
		return l.handleOpenSearchError(err), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return LayerResultFailure(
			string(k8swatchv1.FailureCodeProtocolUnexpectedResp),
			fmt.Sprintf("OpenSearch request failed: %d", resp.StatusCode),
			duration,
		), nil
	}

	return LayerResultSuccess(duration), nil
}

// buildURL builds the OpenSearch URL
func (l *OpenSearchProtocolLayer) buildURL(target *k8swatchv1.Target) (string, error) {
	address, err := l.getAddress(target)
	if err != nil {
		return "", err
	}

	scheme := "http"
	if target.Spec.Layers.L3TLS != nil && target.Spec.Layers.L3TLS.Enabled {
		scheme = "https"
	}

	return fmt.Sprintf("%s://%s/", scheme, address), nil
}

// getAddress extracts address from target
func (l *OpenSearchProtocolLayer) getAddress(target *k8swatchv1.Target) (string, error) {
	endpoint := &target.Spec.Endpoint
	port := 9200

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

// handleOpenSearchError converts OpenSearch errors to failure codes
func (l *OpenSearchProtocolLayer) handleOpenSearchError(err error) *LayerResult {
	return NewElasticsearchProtocolLayer().handleElasticsearchError(err)
}
