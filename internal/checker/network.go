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
	"strconv"
	"time"

	k8swatchv1 "github.com/k8swatch/k8s-monitor/api/v1"
)

// NetworkChecker implements network connectivity checks
type NetworkChecker struct {
	layers []Layer
}

// NetworkCheckerFactory creates network checkers
type NetworkCheckerFactory struct{}

// Create creates a new network checker
func (f *NetworkCheckerFactory) Create(target *k8swatchv1.Target) (Checker, error) {
	layers := []Layer{
		NewNodeSanityChecker(DefaultNodeSanityConfig()),
		NewDNSLayer(),
		NewTCPLayer(),
	}
	return &NetworkChecker{layers: layers}, nil
}

// SupportedTypes returns the supported target types
func (f *NetworkCheckerFactory) SupportedTypes() []string {
	return []string{"network"}
}

// Layers returns the layers for this checker
func (c *NetworkChecker) Layers() []Layer {
	return c.layers
}

// Execute executes all enabled layers for a target
func (c *NetworkChecker) Execute(ctx context.Context, target *k8swatchv1.Target) (*k8swatchv1.CheckResult, error) {
	executor := NewExecutor(c.layers)
	return executor.Execute(ctx, target)
}

// DNSLayer implements L1 DNS resolution check
type DNSLayer struct{}

// NewDNSLayer creates a new DNS layer
func NewDNSLayer() *DNSLayer {
	return &DNSLayer{}
}

// Name returns the layer name
func (l *DNSLayer) Name() string {
	return "L1"
}

// Enabled returns whether this layer is enabled for the target
func (l *DNSLayer) Enabled(target *k8swatchv1.Target) bool {
	return target.Spec.Layers.L1DNS != nil && target.Spec.Layers.L1DNS.Enabled
}

// Check executes the DNS resolution check
func (l *DNSLayer) Check(ctx context.Context, target *k8swatchv1.Target) (*LayerResult, error) {
	startTime := time.Now()

	// Get the hostname to resolve
	hostname := l.getHostname(target)
	if hostname == "" {
		// No hostname to resolve, skip DNS check
		return LayerResultSuccess(time.Since(startTime).Milliseconds()), nil
	}

	// Create a resolver
	resolver := net.DefaultResolver

	// Try to resolve the hostname
	_, err := resolver.LookupHost(ctx, hostname)
	duration := time.Since(startTime).Milliseconds()

	if err != nil {
		return l.handleDNSError(err), nil
	}

	return LayerResultSuccess(duration), nil
}

// getHostname extracts the hostname from the target endpoint
func (l *DNSLayer) getHostname(target *k8swatchv1.Target) string {
	endpoint := &target.Spec.Endpoint

	if endpoint.DNS != nil && *endpoint.DNS != "" {
		return *endpoint.DNS
	}

	if endpoint.K8sService != nil {
		// Construct k8s service FQDN
		ns := endpoint.K8sService.Namespace
		if ns == "" {
			ns = "default"
		}
		return fmt.Sprintf("%s.%s.svc.cluster.local", endpoint.K8sService.Name, ns)
	}

	// IP endpoints don't need DNS resolution
	return ""
}

// handleDNSError converts DNS errors to failure codes
func (l *DNSLayer) handleDNSError(err error) *LayerResult {
	errStr := err.Error()

	switch {
	case contains(errStr, "no such host"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeDNSNXDomain), "DNS NXDOMAIN: no such host", 0)
	case contains(errStr, "server misbehaving"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeDNSServFail), "DNS SERVFAIL: server misbehaving", 0)
	case contains(errStr, "connection refused"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeDNSRefused), "DNS connection refused", 0)
	case contains(errStr, "timeout"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeDNSTimeout), "DNS query timeout", 0)
	case contains(errStr, "no servers"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeDNSNoServers), "No DNS servers available", 0)
	default:
		return LayerResultError(err, string(k8swatchv1.FailureCodeDNSTimeout), 0)
	}
}

// TCPLayer implements L2 TCP connectivity check
type TCPLayer struct{}

// NewTCPLayer creates a new TCP layer
func NewTCPLayer() *TCPLayer {
	return &TCPLayer{}
}

// Name returns the layer name
func (l *TCPLayer) Name() string {
	return "L2"
}

// Enabled returns whether this layer is enabled for the target
func (l *TCPLayer) Enabled(target *k8swatchv1.Target) bool {
	return target.Spec.Layers.L2TCP != nil && target.Spec.Layers.L2TCP.Enabled
}

// Check executes the TCP connectivity check
func (l *TCPLayer) Check(ctx context.Context, target *k8swatchv1.Target) (*LayerResult, error) {
	startTime := time.Now()

	// Get target address
	address, err := l.getTargetAddress(target)
	if err != nil {
		return LayerResultError(err, string(k8swatchv1.FailureCodeConfigError), time.Since(startTime).Milliseconds()), nil
	}

	// Get timeout from layer config
	timeout := 5 * time.Second
	if target.Spec.Layers.L2TCP != nil && target.Spec.Layers.L2TCP.Timeout != "" {
		if d, err := time.ParseDuration(target.Spec.Layers.L2TCP.Timeout); err == nil {
			timeout = d
		}
	}

	// Create dialer
	dialer := net.Dialer{
		Timeout: timeout,
	}

	// Try to connect
	conn, err := dialer.DialContext(ctx, "tcp", address)
	duration := time.Since(startTime).Milliseconds()

	if err != nil {
		return l.handleTCPError(err), nil
	}
	defer conn.Close()

	return LayerResultSuccess(duration), nil
}

// getTargetAddress extracts the address from the target endpoint
func (l *TCPLayer) getTargetAddress(target *k8swatchv1.Target) (string, error) {
	endpoint := &target.Spec.Endpoint
	port := l.getPort(target)

	if endpoint.DNS != nil && *endpoint.DNS != "" {
		return fmt.Sprintf("%s:%d", *endpoint.DNS, port), nil
	}

	if endpoint.IP != nil && *endpoint.IP != "" {
		return fmt.Sprintf("%s:%d", *endpoint.IP, port), nil
	}

	if endpoint.K8sService != nil {
		// For k8s service, construct FQDN
		ns := endpoint.K8sService.Namespace
		if ns == "" {
			ns = "default"
		}
		hostname := fmt.Sprintf("%s.%s.svc.cluster.local", endpoint.K8sService.Name, ns)
		return fmt.Sprintf("%s:%d", hostname, port), nil
	}

	return "", fmt.Errorf("no endpoint configured")
}

// getPort extracts the port from the target
func (l *TCPLayer) getPort(target *k8swatchv1.Target) int {
	if target.Spec.Endpoint.Port != nil {
		return int(*target.Spec.Endpoint.Port)
	}

	// Default ports based on target type
	switch target.Spec.Type {
	case k8swatchv1.TargetTypeHTTP:
		return 80
	case k8swatchv1.TargetTypeHTTPS:
		return 443
	case k8swatchv1.TargetTypePostgreSQL:
		return 5432
	case k8swatchv1.TargetTypeMySQL:
		return 3306
	case k8swatchv1.TargetTypeMSSQL:
		return 1433
	case k8swatchv1.TargetTypeRedis:
		return 6379
	case k8swatchv1.TargetTypeMongoDB:
		return 27017
	case k8swatchv1.TargetTypeElasticsearch, k8swatchv1.TargetTypeOpenSearch:
		return 9200
	case k8swatchv1.TargetTypeKafka:
		return 9092
	case k8swatchv1.TargetTypeRabbitMQ:
		return 5672
	case k8swatchv1.TargetTypeKeycloak:
		return 443
	case k8swatchv1.TargetTypeNginx:
		return 80
	case k8swatchv1.TargetTypeMinIO:
		return 9000
	case k8swatchv1.TargetTypeClickHouse:
		return 9000
	default:
		return 80
	}
}

// handleTCPError converts TCP errors to failure codes
func (l *TCPLayer) handleTCPError(err error) *LayerResult {
	errStr := err.Error()

	switch {
	case contains(errStr, "connection refused"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeTCPRefused), "TCP connection refused", 0)
	case contains(errStr, "no route to host"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeTCPNoRoute), "TCP no route to host", 0)
	case contains(errStr, "network is unreachable"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeTCPNoRoute), "Network unreachable", 0)
	case contains(errStr, "timeout"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeTCPTimeout), "TCP connection timeout", 0)
	case contains(errStr, "i/o timeout"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeTCPTimeout), "TCP I/O timeout", 0)
	case contains(errStr, "connection reset"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeTCPReset), "TCP connection reset", 0)
	default:
		return LayerResultError(err, string(k8swatchv1.FailureCodeTCPTimeout), 0)
	}
}

// resolvePort resolves port from string (supports named ports)
func resolvePort(portStr string, defaultPort int) (int, error) {
	// Try to parse as number first
	if port, err := strconv.Atoi(portStr); err == nil {
		return port, nil
	}

	// Named port - use default
	return defaultPort, nil
}
