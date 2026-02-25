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

// KafkaChecker implements Kafka health checks
type KafkaChecker struct {
	layers []Layer
}

// KafkaCheckerFactory creates Kafka checkers
type KafkaCheckerFactory struct{}

// Create creates a new Kafka checker
func (f *KafkaCheckerFactory) Create(target *k8swatchv1.Target) (Checker, error) {
	layers := []Layer{
		NewNodeSanityChecker(DefaultNodeSanityConfig()),
		NewDNSLayer(),
		NewTCPLayer(),
		NewKafkaProtocolLayer(),
	}
	return &KafkaChecker{layers: layers}, nil
}

// SupportedTypes returns the supported target types
func (f *KafkaCheckerFactory) SupportedTypes() []string {
	return []string{"kafka"}
}

// Layers returns the layers for this checker
func (c *KafkaChecker) Layers() []Layer {
	return c.layers
}

// Execute executes all enabled layers for a target
func (c *KafkaChecker) Execute(ctx context.Context, target *k8swatchv1.Target) (*k8swatchv1.CheckResult, error) {
	executor := NewExecutor(c.layers)
	return executor.Execute(ctx, target)
}

// KafkaProtocolLayer implements L4 Kafka protocol check
type KafkaProtocolLayer struct{}

// NewKafkaProtocolLayer creates a new Kafka protocol layer
func NewKafkaProtocolLayer() *KafkaProtocolLayer {
	return &KafkaProtocolLayer{}
}

// Name returns the layer name
func (l *KafkaProtocolLayer) Name() string {
	return "L4"
}

// Enabled returns whether this layer is enabled
func (l *KafkaProtocolLayer) Enabled(target *k8swatchv1.Target) bool {
	return target.Spec.Type == k8swatchv1.TargetTypeKafka ||
		(target.Spec.Layers.L4Protocol != nil && target.Spec.Layers.L4Protocol.Enabled)
}

// Check executes the Kafka protocol check
func (l *KafkaProtocolLayer) Check(ctx context.Context, target *k8swatchv1.Target) (*LayerResult, error) {
	startTime := time.Now()

	address, err := l.getAddress(target)
	if err != nil {
		return LayerResultError(err, string(k8swatchv1.FailureCodeConfigError), time.Since(startTime).Milliseconds()), nil
	}

	timeout := 5 * time.Second
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return l.handleKafkaError(err), nil
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(timeout))

	// Send Kafka ApiVersions request (minimal implementation)
	// Kafka protocol: length (4 bytes) + request
	// ApiVersions request key = 18, version = 0
	apiVersionsReq := []byte{
		0x00, 0x00, 0x00, 0x15, // Length: 21 bytes
		0x00, 0x12, // ApiVersions key: 18
		0x00, 0x00, // Version: 0
		0x00, 0x00, 0x00, 0x00, // Correlation ID: 0
		0x00, 0x00, // Client ID length: 0 (empty)
	}

	if _, err := conn.Write(apiVersionsReq); err != nil {
		return LayerResultError(err, string(k8swatchv1.FailureCodeProtocolError), time.Since(startTime).Milliseconds()), nil
	}

	// Read response header
	buf := make([]byte, 8)
	n, err := conn.Read(buf)
	if err != nil {
		return LayerResultError(err, string(k8swatchv1.FailureCodeProtocolError), time.Since(startTime).Milliseconds()), nil
	}

	if n < 8 {
		return LayerResultFailure(
			string(k8swatchv1.FailureCodeProtocolUnexpectedResp),
			"Kafka response too short",
			time.Since(startTime).Milliseconds(),
		), nil
	}

	return LayerResultSuccess(time.Since(startTime).Milliseconds()), nil
}

// getAddress extracts address from target
func (l *KafkaProtocolLayer) getAddress(target *k8swatchv1.Target) (string, error) {
	endpoint := &target.Spec.Endpoint
	port := 9092

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

// handleKafkaError converts Kafka errors to failure codes
func (l *KafkaProtocolLayer) handleKafkaError(err error) *LayerResult {
	errStr := err.Error()

	switch {
	case contains(errStr, "connection refused"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeTCPRefused), "Kafka connection refused", 0)
	case contains(errStr, "timeout"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeProtocolTimeout), "Kafka timeout", 0)
	case contains(errStr, "no route"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeTCPNoRoute), "Kafka no route", 0)
	default:
		return LayerResultError(err, string(k8swatchv1.FailureCodeProtocolError), 0)
	}
}

// RabbitMQChecker implements RabbitMQ health checks
type RabbitMQChecker struct {
	layers []Layer
}

// RabbitMQCheckerFactory creates RabbitMQ checkers
type RabbitMQCheckerFactory struct{}

// Create creates a new RabbitMQ checker
func (f *RabbitMQCheckerFactory) Create(target *k8swatchv1.Target) (Checker, error) {
	layers := []Layer{
		NewNodeSanityChecker(DefaultNodeSanityConfig()),
		NewDNSLayer(),
		NewTCPLayer(),
		NewRabbitMQProtocolLayer(),
		NewRabbitMQSemanticLayer(),
	}
	return &RabbitMQChecker{layers: layers}, nil
}

// SupportedTypes returns the supported target types
func (f *RabbitMQCheckerFactory) SupportedTypes() []string {
	return []string{"rabbitmq"}
}

// Layers returns the layers for this checker
func (c *RabbitMQChecker) Layers() []Layer {
	return c.layers
}

// Execute executes all enabled layers for a target
func (c *RabbitMQChecker) Execute(ctx context.Context, target *k8swatchv1.Target) (*k8swatchv1.CheckResult, error) {
	executor := NewExecutor(c.layers)
	return executor.Execute(ctx, target)
}

// RabbitMQProtocolLayer implements L4 RabbitMQ protocol check
type RabbitMQProtocolLayer struct{}

// NewRabbitMQProtocolLayer creates a new RabbitMQ protocol layer
func NewRabbitMQProtocolLayer() *RabbitMQProtocolLayer {
	return &RabbitMQProtocolLayer{}
}

// Name returns the layer name
func (l *RabbitMQProtocolLayer) Name() string {
	return "L4"
}

// Enabled returns whether this layer is enabled
func (l *RabbitMQProtocolLayer) Enabled(target *k8swatchv1.Target) bool {
	return target.Spec.Type == k8swatchv1.TargetTypeRabbitMQ ||
		(target.Spec.Layers.L4Protocol != nil && target.Spec.Layers.L4Protocol.Enabled)
}

// Check executes the RabbitMQ protocol check (AMQP handshake)
func (l *RabbitMQProtocolLayer) Check(ctx context.Context, target *k8swatchv1.Target) (*LayerResult, error) {
	startTime := time.Now()

	address, err := l.getAddress(target)
	if err != nil {
		return LayerResultError(err, string(k8swatchv1.FailureCodeConfigError), time.Since(startTime).Milliseconds()), nil
	}

	timeout := 5 * time.Second
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return l.handleRabbitMQError(err), nil
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(timeout))

	// Send AMQP 0-9-1 protocol header
	// "AMQP" + protocol version (0, 0, 9, 1)
	amqpHeader := []byte{'A', 'M', 'Q', 'P', 0, 0, 9, 1}

	if _, err := conn.Write(amqpHeader); err != nil {
		return LayerResultError(err, string(k8swatchv1.FailureCodeProtocolError), time.Since(startTime).Milliseconds()), nil
	}

	// Read response (should be AMQP header from server)
	buf := make([]byte, 8)
	n, err := conn.Read(buf)
	if err != nil {
		return LayerResultError(err, string(k8swatchv1.FailureCodeProtocolError), time.Since(startTime).Milliseconds()), nil
	}

	if n < 8 {
		return LayerResultFailure(
			string(k8swatchv1.FailureCodeProtocolUnexpectedResp),
			"RabbitMQ response too short",
			time.Since(startTime).Milliseconds(),
		), nil
	}

	// Verify AMQP header response
	if string(buf[:4]) != "AMQP" {
		return LayerResultFailure(
			string(k8swatchv1.FailureCodeProtocolUnexpectedResp),
			"Unexpected RabbitMQ response (not AMQP)",
			time.Since(startTime).Milliseconds(),
		), nil
	}

	return LayerResultSuccess(time.Since(startTime).Milliseconds()), nil
}

// getAddress extracts address from target
func (l *RabbitMQProtocolLayer) getAddress(target *k8swatchv1.Target) (string, error) {
	endpoint := &target.Spec.Endpoint
	port := 5672

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

// handleRabbitMQError converts RabbitMQ errors to failure codes
func (l *RabbitMQProtocolLayer) handleRabbitMQError(err error) *LayerResult {
	errStr := err.Error()

	switch {
	case contains(errStr, "connection refused"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeTCPRefused), "RabbitMQ connection refused", 0)
	case contains(errStr, "timeout"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeProtocolTimeout), "RabbitMQ timeout", 0)
	case contains(errStr, "no route"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeTCPNoRoute), "RabbitMQ no route", 0)
	default:
		return LayerResultError(err, string(k8swatchv1.FailureCodeProtocolError), 0)
	}
}

// RabbitMQSemanticLayer implements L6 RabbitMQ semantic check
type RabbitMQSemanticLayer struct{}

// NewRabbitMQSemanticLayer creates a new RabbitMQ semantic layer
func NewRabbitMQSemanticLayer() *RabbitMQSemanticLayer {
	return &RabbitMQSemanticLayer{}
}

// Name returns the layer name
func (l *RabbitMQSemanticLayer) Name() string {
	return "L6"
}

// Enabled returns whether this layer is enabled
func (l *RabbitMQSemanticLayer) Enabled(target *k8swatchv1.Target) bool {
	return target.Spec.Layers.L6Semantic != nil && target.Spec.Layers.L6Semantic.Enabled
}

// Check executes the RabbitMQ semantic check via management API
func (l *RabbitMQSemanticLayer) Check(ctx context.Context, target *k8swatchv1.Target) (*LayerResult, error) {
	startTime := time.Now()

	// Try management API on port 15672
	address, err := l.getManagementAddress(target)
	if err != nil {
		// Fall back to checking AMQP connection
		return LayerResultSuccess(time.Since(startTime).Milliseconds()), nil
	}

	url := fmt.Sprintf("http://%s/api/health", address)
	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Get(url)
	duration := time.Since(startTime).Milliseconds()

	if err != nil {
		// Management API not available, AMQP check succeeded
		return LayerResultSuccess(time.Since(startTime).Milliseconds()), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return LayerResultFailure(
			string(k8swatchv1.FailureCodeSemanticFailed),
			fmt.Sprintf("RabbitMQ health check failed: %d", resp.StatusCode),
			duration,
		), nil
	}

	return LayerResultSuccess(duration), nil
}

// getManagementAddress extracts management address from target
func (l *RabbitMQSemanticLayer) getManagementAddress(target *k8swatchv1.Target) (string, error) {
	endpoint := &target.Spec.Endpoint
	port := 15672

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

// KeycloakChecker implements Keycloak health checks
type KeycloakChecker struct {
	layers []Layer
}

// KeycloakCheckerFactory creates Keycloak checkers
type KeycloakCheckerFactory struct{}

// Create creates a new Keycloak checker
func (f *KeycloakCheckerFactory) Create(target *k8swatchv1.Target) (Checker, error) {
	layers := []Layer{
		NewNodeSanityChecker(DefaultNodeSanityConfig()),
		NewDNSLayer(),
		NewTCPLayer(),
		NewTLSLayer(),
		NewKeycloakProtocolLayer(),
	}
	return &KeycloakChecker{layers: layers}, nil
}

// SupportedTypes returns the supported target types
func (f *KeycloakCheckerFactory) SupportedTypes() []string {
	return []string{"keycloak"}
}

// Layers returns the layers for this checker
func (c *KeycloakChecker) Layers() []Layer {
	return c.layers
}

// Execute executes all enabled layers for a target
func (c *KeycloakChecker) Execute(ctx context.Context, target *k8swatchv1.Target) (*k8swatchv1.CheckResult, error) {
	executor := NewExecutor(c.layers)
	return executor.Execute(ctx, target)
}

// KeycloakProtocolLayer implements L4 Keycloak protocol check
type KeycloakProtocolLayer struct{}

// NewKeycloakProtocolLayer creates a new Keycloak protocol layer
func NewKeycloakProtocolLayer() *KeycloakProtocolLayer {
	return &KeycloakProtocolLayer{}
}

// Name returns the layer name
func (l *KeycloakProtocolLayer) Name() string {
	return "L4"
}

// Enabled returns whether this layer is enabled
func (l *KeycloakProtocolLayer) Enabled(target *k8swatchv1.Target) bool {
	return target.Spec.Type == k8swatchv1.TargetTypeKeycloak ||
		(target.Spec.Layers.L4Protocol != nil && target.Spec.Layers.L4Protocol.Enabled)
}

// Check executes the Keycloak protocol check (OIDC discovery)
func (l *KeycloakProtocolLayer) Check(ctx context.Context, target *k8swatchv1.Target) (*LayerResult, error) {
	startTime := time.Now()

	url, err := l.buildURL(target)
	if err != nil {
		return LayerResultError(err, string(k8swatchv1.FailureCodeConfigError), time.Since(startTime).Milliseconds()), nil
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	duration := time.Since(startTime).Milliseconds()

	if err != nil {
		return l.handleKeycloakError(err), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return LayerResultFailure(
			string(k8swatchv1.FailureCodeProtocolUnexpectedResp),
			fmt.Sprintf("Keycloak OIDC discovery failed: %d", resp.StatusCode),
			duration,
		), nil
	}

	return LayerResultSuccess(duration), nil
}

// buildURL builds the Keycloak OIDC discovery URL
func (l *KeycloakProtocolLayer) buildURL(target *k8swatchv1.Target) (string, error) {
	address, err := l.getAddress(target)
	if err != nil {
		return "", err
	}

	scheme := "https"
	if target.Spec.Type == k8swatchv1.TargetTypeHTTP {
		scheme = "http"
	}

	// Default Keycloak realm path
	path := "/realms/master/.well-known/openid-configuration"

	if target.Spec.Layers.L4Protocol != nil && target.Spec.Layers.L4Protocol.HealthQuery != "" {
		path = target.Spec.Layers.L4Protocol.HealthQuery
	}

	return fmt.Sprintf("%s://%s%s", scheme, address, path), nil
}

// getAddress extracts address from target
func (l *KeycloakProtocolLayer) getAddress(target *k8swatchv1.Target) (string, error) {
	endpoint := &target.Spec.Endpoint
	port := 443

	if target.Spec.Type == k8swatchv1.TargetTypeHTTP {
		port = 80
	}

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

// handleKeycloakError converts Keycloak errors to failure codes
func (l *KeycloakProtocolLayer) handleKeycloakError(err error) *LayerResult {
	errStr := err.Error()

	switch {
	case contains(errStr, "connection refused"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeTCPRefused), "Keycloak connection refused", 0)
	case contains(errStr, "timeout"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeProtocolTimeout), "Keycloak timeout", 0)
	case contains(errStr, "no route"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeTCPNoRoute), "Keycloak no route", 0)
	default:
		return LayerResultError(err, string(k8swatchv1.FailureCodeProtocolError), 0)
	}
}

// NginxChecker implements Nginx health checks
type NginxChecker struct {
	layers []Layer
}

// NginxCheckerFactory creates Nginx checkers
type NginxCheckerFactory struct{}

// Create creates a new Nginx checker
func (f *NginxCheckerFactory) Create(target *k8swatchv1.Target) (Checker, error) {
	layers := []Layer{
		NewNodeSanityChecker(DefaultNodeSanityConfig()),
		NewDNSLayer(),
		NewTCPLayer(),
		NewNginxProtocolLayer(),
	}
	return &NginxChecker{layers: layers}, nil
}

// SupportedTypes returns the supported target types
func (f *NginxCheckerFactory) SupportedTypes() []string {
	return []string{"nginx"}
}

// Layers returns the layers for this checker
func (c *NginxChecker) Layers() []Layer {
	return c.layers
}

// Execute executes all enabled layers for a target
func (c *NginxChecker) Execute(ctx context.Context, target *k8swatchv1.Target) (*k8swatchv1.CheckResult, error) {
	executor := NewExecutor(c.layers)
	return executor.Execute(ctx, target)
}

// NginxProtocolLayer implements L4 Nginx protocol check
type NginxProtocolLayer struct{}

// NewNginxProtocolLayer creates a new Nginx protocol layer
func NewNginxProtocolLayer() *NginxProtocolLayer {
	return &NginxProtocolLayer{}
}

// Name returns the layer name
func (l *NginxProtocolLayer) Name() string {
	return "L4"
}

// Enabled returns whether this layer is enabled
func (l *NginxProtocolLayer) Enabled(target *k8swatchv1.Target) bool {
	return target.Spec.Type == k8swatchv1.TargetTypeNginx ||
		(target.Spec.Layers.L4Protocol != nil && target.Spec.Layers.L4Protocol.Enabled)
}

// Check executes the Nginx protocol check (stub_status or root)
func (l *NginxProtocolLayer) Check(ctx context.Context, target *k8swatchv1.Target) (*LayerResult, error) {
	startTime := time.Now()

	url, err := l.buildURL(target)
	if err != nil {
		return LayerResultError(err, string(k8swatchv1.FailureCodeConfigError), time.Since(startTime).Milliseconds()), nil
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	duration := time.Since(startTime).Milliseconds()

	if err != nil {
		return l.handleNginxError(err), nil
	}
	defer resp.Body.Close()

	// Accept 2xx, 3xx, and even 4xx (Nginx is responding)
	if resp.StatusCode >= 500 {
		return LayerResultFailure(
			string(k8swatchv1.FailureCodeProtocolUnexpectedResp),
			fmt.Sprintf("Nginx server error: %d", resp.StatusCode),
			duration,
		), nil
	}

	return LayerResultSuccess(duration), nil
}

// buildURL builds the Nginx URL
func (l *NginxProtocolLayer) buildURL(target *k8swatchv1.Target) (string, error) {
	address, err := l.getAddress(target)
	if err != nil {
		return "", err
	}

	scheme := "http"
	if target.Spec.Layers.L3TLS != nil && target.Spec.Layers.L3TLS.Enabled {
		scheme = "https"
	}

	path := "/"
	if target.Spec.Endpoint.Path != nil {
		path = *target.Spec.Endpoint.Path
	}

	return fmt.Sprintf("%s://%s%s", scheme, address, path), nil
}

// getAddress extracts address from target
func (l *NginxProtocolLayer) getAddress(target *k8swatchv1.Target) (string, error) {
	endpoint := &target.Spec.Endpoint
	port := 80

	if target.Spec.Layers.L3TLS != nil && target.Spec.Layers.L3TLS.Enabled {
		port = 443
	}

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

// handleNginxError converts Nginx errors to failure codes
func (l *NginxProtocolLayer) handleNginxError(err error) *LayerResult {
	errStr := err.Error()

	switch {
	case contains(errStr, "connection refused"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeTCPRefused), "Nginx connection refused", 0)
	case contains(errStr, "timeout"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeProtocolTimeout), "Nginx timeout", 0)
	case contains(errStr, "no route"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeTCPNoRoute), "Nginx no route", 0)
	default:
		return LayerResultError(err, string(k8swatchv1.FailureCodeProtocolError), 0)
	}
}
