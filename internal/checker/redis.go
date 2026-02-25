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

// RedisChecker implements Redis health checks
type RedisChecker struct {
	layers []Layer
}

// RedisCheckerFactory creates Redis checkers
type RedisCheckerFactory struct{}

// Create creates a new Redis checker
func (f *RedisCheckerFactory) Create(target *k8swatchv1.Target) (Checker, error) {
	layers := []Layer{
		NewNodeSanityChecker(DefaultNodeSanityConfig()),
		NewDNSLayer(),
		NewTCPLayer(),
		NewRedisProtocolLayer(),
		NewRedisAuthLayer(),
		NewRedisSemanticLayer(),
	}
	return &RedisChecker{layers: layers}, nil
}

// SupportedTypes returns the supported target types
func (f *RedisCheckerFactory) SupportedTypes() []string {
	return []string{"redis"}
}

// Layers returns the layers for this checker
func (c *RedisChecker) Layers() []Layer {
	return c.layers
}

// Execute executes all enabled layers for a target
func (c *RedisChecker) Execute(ctx context.Context, target *k8swatchv1.Target) (*k8swatchv1.CheckResult, error) {
	executor := NewExecutor(c.layers)
	return executor.Execute(ctx, target)
}

// RedisProtocolLayer implements L4 Redis protocol check
type RedisProtocolLayer struct{}

// NewRedisProtocolLayer creates a new Redis protocol layer
func NewRedisProtocolLayer() *RedisProtocolLayer {
	return &RedisProtocolLayer{}
}

// Name returns the layer name
func (l *RedisProtocolLayer) Name() string {
	return "L4"
}

// Enabled returns whether this layer is enabled
func (l *RedisProtocolLayer) Enabled(target *k8swatchv1.Target) bool {
	return target.Spec.Type == k8swatchv1.TargetTypeRedis ||
		(target.Spec.Layers.L4Protocol != nil && target.Spec.Layers.L4Protocol.Enabled)
}

// Check executes the Redis protocol check using raw TCP
func (l *RedisProtocolLayer) Check(ctx context.Context, target *k8swatchv1.Target) (*LayerResult, error) {
	startTime := time.Now()

	address, err := l.getAddress(target)
	if err != nil {
		return LayerResultError(err, string(k8swatchv1.FailureCodeConfigError), time.Since(startTime).Milliseconds()), nil
	}

	// Get timeout
	timeout := 5 * time.Second
	if target.Spec.Schedule.Timeout != "" {
		if d, err := time.ParseDuration(target.Spec.Schedule.Timeout); err == nil {
			timeout = d
		}
	}

	// Connect to Redis
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return l.handleRedisError(err), nil
	}
	defer conn.Close()

	// Set deadline
	_ = conn.SetDeadline(time.Now().Add(timeout))

	// Send PING command (Redis protocol: *1\r\n$4\r\nPING\r\n)
	pingCmd := []byte("*1\r\n$4\r\nPING\r\n")
	if _, err := conn.Write(pingCmd); err != nil {
		return LayerResultError(err, string(k8swatchv1.FailureCodeProtocolError), time.Since(startTime).Milliseconds()), nil
	}

	// Read response
	buf := make([]byte, 256)
	n, err := conn.Read(buf)
	if err != nil {
		return LayerResultError(err, string(k8swatchv1.FailureCodeProtocolError), time.Since(startTime).Milliseconds()), nil
	}

	response := string(buf[:n])

	// Check for PONG response
	if !strings.Contains(response, "PONG") {
		return LayerResultFailure(
			string(k8swatchv1.FailureCodeProtocolUnexpectedResp),
			fmt.Sprintf("Unexpected Redis response: %s", response),
			time.Since(startTime).Milliseconds(),
		), nil
	}

	return LayerResultSuccess(time.Since(startTime).Milliseconds()), nil
}

// getAddress extracts address from target
func (l *RedisProtocolLayer) getAddress(target *k8swatchv1.Target) (string, error) {
	endpoint := &target.Spec.Endpoint
	port := 6379

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

// handleRedisError converts Redis errors to failure codes
func (l *RedisProtocolLayer) handleRedisError(err error) *LayerResult {
	errStr := err.Error()

	switch {
	case contains(errStr, "connection refused"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeTCPRefused), "Redis connection refused", 0)
	case contains(errStr, "timeout"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeProtocolTimeout), "Redis timeout", 0)
	case contains(errStr, "no route"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeTCPNoRoute), "Redis no route", 0)
	default:
		return LayerResultError(err, string(k8swatchv1.FailureCodeProtocolError), 0)
	}
}

// RedisAuthLayer implements L5 Redis authentication check
type RedisAuthLayer struct{}

// NewRedisAuthLayer creates a new Redis auth layer
func NewRedisAuthLayer() *RedisAuthLayer {
	return &RedisAuthLayer{}
}

// Name returns the layer name
func (l *RedisAuthLayer) Name() string {
	return "L5"
}

// Enabled returns whether this layer is enabled
func (l *RedisAuthLayer) Enabled(target *k8swatchv1.Target) bool {
	return target.Spec.Layers.L5Auth != nil && target.Spec.Layers.L5Auth.Enabled
}

// Check executes the Redis authentication check
func (l *RedisAuthLayer) Check(ctx context.Context, target *k8swatchv1.Target) (*LayerResult, error) {
	startTime := time.Now()

	address, err := l.getAddress(target)
	if err != nil {
		return LayerResultError(err, string(k8swatchv1.FailureCodeConfigError), time.Since(startTime).Milliseconds()), nil
	}

	timeout := 5 * time.Second
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return l.handleRedisError(err), nil
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(timeout))

	// Get password from config
	password := ""
	if target.Spec.Layers.L5Auth != nil && target.Spec.Layers.L5Auth.Token != "" {
		password = target.Spec.Layers.L5Auth.Token
	}
	if password == "" && target.Spec.Layers.L5Auth != nil && target.Spec.Layers.L5Auth.CredentialsRef != nil {
		creds, credErr := loadCredentialsRef(context.Background(), target.Namespace, target.Spec.Layers.L5Auth.CredentialsRef)
		if credErr != nil {
			return LayerResultError(credErr, string(k8swatchv1.FailureCodeConfigError), time.Since(startTime).Milliseconds()), nil
		}
		if creds.token != "" {
			password = creds.token
		} else if creds.password != "" {
			password = creds.password
		}
	}

	if password != "" {
		// Send AUTH command
		authCmd := fmt.Sprintf("*2\r\n$4\r\nAUTH\r\n$%d\r\n%s\r\n", len(password), password)
		if _, err := conn.Write([]byte(authCmd)); err != nil {
			return LayerResultError(err, string(k8swatchv1.FailureCodeAuthFailed), time.Since(startTime).Milliseconds()), nil
		}

		// Read response
		buf := make([]byte, 256)
		n, err := conn.Read(buf)
		if err != nil {
			return LayerResultError(err, string(k8swatchv1.FailureCodeAuthFailed), time.Since(startTime).Milliseconds()), nil
		}

		response := string(buf[:n])
		if strings.Contains(response, "ERR") || strings.Contains(response, "NOPERM") {
			return LayerResultFailure(
				string(k8swatchv1.FailureCodeAuthFailed),
				fmt.Sprintf("Redis auth failed: %s", response),
				time.Since(startTime).Milliseconds(),
			), nil
		}
	}

	return LayerResultSuccess(time.Since(startTime).Milliseconds()), nil
}

// getAddress extracts address for auth layer
func (l *RedisAuthLayer) getAddress(target *k8swatchv1.Target) (string, error) {
	protocolLayer := NewRedisProtocolLayer()
	return protocolLayer.getAddress(target)
}

// handleRedisError converts errors for auth layer
func (l *RedisAuthLayer) handleRedisError(err error) *LayerResult {
	return NewRedisProtocolLayer().handleRedisError(err)
}

// RedisSemanticLayer implements L6 Redis semantic check
type RedisSemanticLayer struct{}

// NewRedisSemanticLayer creates a new Redis semantic layer
func NewRedisSemanticLayer() *RedisSemanticLayer {
	return &RedisSemanticLayer{}
}

// Name returns the layer name
func (l *RedisSemanticLayer) Name() string {
	return "L6"
}

// Enabled returns whether this layer is enabled
func (l *RedisSemanticLayer) Enabled(target *k8swatchv1.Target) bool {
	return target.Spec.Layers.L6Semantic != nil && target.Spec.Layers.L6Semantic.Enabled
}

// Check executes the Redis semantic check (INFO server)
func (l *RedisSemanticLayer) Check(ctx context.Context, target *k8swatchv1.Target) (*LayerResult, error) {
	startTime := time.Now()

	address, err := l.getAddress(target)
	if err != nil {
		return LayerResultError(err, string(k8swatchv1.FailureCodeConfigError), time.Since(startTime).Milliseconds()), nil
	}

	timeout := 5 * time.Second
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return l.handleRedisError(err), nil
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(timeout))

	// Send INFO server command
	infoCmd := []byte("*2\r\n$4\r\nINFO\r\n$6\r\nserver\r\n")
	if _, err := conn.Write(infoCmd); err != nil {
		return LayerResultError(err, string(k8swatchv1.FailureCodeSemanticFailed), time.Since(startTime).Milliseconds()), nil
	}

	// Read response
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return LayerResultError(err, string(k8swatchv1.FailureCodeSemanticFailed), time.Since(startTime).Milliseconds()), nil
	}

	response := string(buf[:n])

	// Check for redis_version in response
	if !strings.Contains(response, "redis_version:") {
		return LayerResultFailure(
			string(k8swatchv1.FailureCodeSemanticUnexpected),
			"Redis INFO response missing redis_version",
			time.Since(startTime).Milliseconds(),
		), nil
	}

	return LayerResultSuccess(time.Since(startTime).Milliseconds()), nil
}

// getAddress extracts address for semantic layer
func (l *RedisSemanticLayer) getAddress(target *k8swatchv1.Target) (string, error) {
	protocolLayer := NewRedisProtocolLayer()
	return protocolLayer.getAddress(target)
}

// handleRedisError converts errors for semantic layer
func (l *RedisSemanticLayer) handleRedisError(err error) *LayerResult {
	return NewRedisProtocolLayer().handleRedisError(err)
}
