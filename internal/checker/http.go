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
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	k8swatchv1 "github.com/k8swatch/k8s-monitor/api/v1"
)

// HTTPChecker implements HTTP/HTTPS health checks
type HTTPChecker struct {
	layers []Layer
}

// HTTPCheckerFactory creates HTTP checkers
type HTTPCheckerFactory struct{}

// Create creates a new HTTP checker
func (f *HTTPCheckerFactory) Create(target *k8swatchv1.Target) (Checker, error) {
	layers := []Layer{
		NewNodeSanityChecker(DefaultNodeSanityConfig()),
		NewDNSLayer(),
		NewTCPLayer(),
		NewTLSLayer(),
		NewHTTPProtocolLayer(),
		NewHTTPAuthLayer(),
		NewHTTPSemanticLayer(),
	}
	return &HTTPChecker{layers: layers}, nil
}

// SupportedTypes returns the supported target types
func (f *HTTPCheckerFactory) SupportedTypes() []string {
	return []string{"http", "https"}
}

// Layers returns the layers for this checker
func (c *HTTPChecker) Layers() []Layer {
	return c.layers
}

// Execute executes all enabled layers for a target
func (c *HTTPChecker) Execute(ctx context.Context, target *k8swatchv1.Target) (*k8swatchv1.CheckResult, error) {
	executor := NewExecutor(c.layers)
	return executor.Execute(ctx, target)
}

// TLSLayer implements L3 TLS handshake check
type TLSLayer struct{}

// NewTLSLayer creates a new TLS layer
func NewTLSLayer() *TLSLayer {
	return &TLSLayer{}
}

// Name returns the layer name
func (l *TLSLayer) Name() string {
	return "L3"
}

// Enabled returns whether this layer is enabled for the target
func (l *TLSLayer) Enabled(target *k8swatchv1.Target) bool {
	// Enable for HTTPS targets or when L3_tls is configured
	return target.Spec.Type == k8swatchv1.TargetTypeHTTPS ||
		(target.Spec.Layers.L3TLS != nil && target.Spec.Layers.L3TLS.Enabled)
}

// Check executes the TLS handshake check
func (l *TLSLayer) Check(ctx context.Context, target *k8swatchv1.Target) (*LayerResult, error) {
	startTime := time.Now()

	// Get target URL
	targetURL, err := l.getTargetURL(target)
	if err != nil {
		return LayerResultError(err, string(k8swatchv1.FailureCodeConfigError), time.Since(startTime).Milliseconds()), nil
	}

	// Only check TLS for HTTPS URLs
	if targetURL.Scheme != "https" {
		return LayerResultSuccess(time.Since(startTime).Milliseconds()), nil
	}

	// Get TLS config
	tlsConfig, err := l.getTLSConfig(target)
	if err != nil {
		return LayerResultError(err, string(k8swatchv1.FailureCodeConfigError), time.Since(startTime).Milliseconds()), nil
	}

	// Get timeout
	timeout := 5 * time.Second
	if target.Spec.Layers.L3TLS != nil && target.Spec.Layers.L3TLS.Timeout != "" {
		if d, err := time.ParseDuration(target.Spec.Layers.L3TLS.Timeout); err == nil {
			timeout = d
		}
	}

	// Create connection
	host := targetURL.Host
	if !strings.Contains(host, ":") {
		host = host + ":443"
	}

	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", host)
	if err != nil {
		return l.handleTCPError(err), nil
	}
	defer conn.Close()

	// Set deadline
	_ = conn.SetDeadline(time.Now().Add(timeout))

	// Perform TLS handshake
	tlsConn := tls.Client(conn, tlsConfig)
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		return l.handleTLSError(err), nil
	}

	// Verify certificate if not skipping
	if !tlsConfig.InsecureSkipVerify {
		if err := tlsConn.VerifyHostname(targetURL.Hostname()); err != nil {
			return LayerResultFailure(string(k8swatchv1.FailureCodeTLSWrongHost), err.Error(), time.Since(startTime).Milliseconds()), nil
		}
	}

	return LayerResultSuccess(time.Since(startTime).Milliseconds()), nil
}

// getTargetURL constructs the target URL
func (l *TLSLayer) getTargetURL(target *k8swatchv1.Target) (*url.URL, error) {
	endpoint := &target.Spec.Endpoint
	port := getPort(target, 443)

	var scheme string
	if target.Spec.Type == k8swatchv1.TargetTypeHTTPS {
		scheme = "https"
	} else {
		scheme = "http"
	}

	if endpoint.DNS != nil && *endpoint.DNS != "" {
		return &url.URL{
			Scheme: scheme,
			Host:   fmt.Sprintf("%s:%d", *endpoint.DNS, port),
			Path:   getPath(target, "/"),
		}, nil
	}

	if endpoint.IP != nil && *endpoint.IP != "" {
		return &url.URL{
			Scheme: scheme,
			Host:   fmt.Sprintf("%s:%d", *endpoint.IP, port),
			Path:   getPath(target, "/"),
		}, nil
	}

	if endpoint.K8sService != nil {
		ns := endpoint.K8sService.Namespace
		if ns == "" {
			ns = "default"
		}
		hostname := fmt.Sprintf("%s.%s.svc.cluster.local", endpoint.K8sService.Name, ns)
		return &url.URL{
			Scheme: scheme,
			Host:   fmt.Sprintf("%s:%d", hostname, port),
			Path:   getPath(target, "/"),
		}, nil
	}

	return nil, fmt.Errorf("no endpoint configured")
}

// getTLSConfig creates TLS configuration
func (l *TLSLayer) getTLSConfig(target *k8swatchv1.Target) (*tls.Config, error) {
	config := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: false,
	}

	// Check for custom TLS config
	if target.Spec.Layers.L3TLS != nil {
		if target.Spec.Layers.L3TLS.InsecureSkipVerify {
			config.InsecureSkipVerify = true
		}

		if target.Spec.Layers.L3TLS.CABundleRef != nil {
			caBundle, err := loadSecretKeyRef(context.Background(), target.Namespace, target.Spec.Layers.L3TLS.CABundleRef)
			if err != nil {
				return nil, fmt.Errorf("failed to load CA bundle from secret: %w", err)
			}

			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM([]byte(caBundle)) {
				return nil, fmt.Errorf("failed to parse CA bundle PEM data")
			}
			config.RootCAs = pool
		}

		if target.Spec.Layers.L3TLS.ClientCertRef != nil {
			certPEM, keyPEM, err := loadTLSCertRef(context.Background(), target.Namespace, target.Spec.Layers.L3TLS.ClientCertRef)
			if err != nil {
				return nil, fmt.Errorf("failed to load client certificate from secret: %w", err)
			}

			cert, err := tls.X509KeyPair(certPEM, keyPEM)
			if err != nil {
				return nil, fmt.Errorf("failed to parse client certificate key pair: %w", err)
			}
			config.Certificates = []tls.Certificate{cert}
		}
	}

	return config, nil
}

// handleTLSError converts TLS errors to failure codes
func (l *TLSLayer) handleTLSError(err error) *LayerResult {
	errStr := err.Error()

	switch {
	case contains(errStr, "certificate has expired"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeTLSCertExpired), "TLS certificate expired", 0)
	case contains(errStr, "certificate is not yet valid"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeTLSCertNotYetValid), "TLS certificate not yet valid", 0)
	case contains(errStr, "certificate signed by unknown authority"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeTLSUntrustedIssuer), "TLS untrusted issuer", 0)
	case contains(errStr, "certificate is valid for"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeTLSWrongHost), "TLS wrong host", 0)
	case contains(errStr, "handshake failure"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeTLSHandshakeFailed), "TLS handshake failure", 0)
	default:
		return LayerResultError(err, string(k8swatchv1.FailureCodeTLSHandshakeFailed), 0)
	}
}

// handleTCPError converts TCP errors for TLS layer
func (l *TLSLayer) handleTCPError(err error) *LayerResult {
	errStr := err.Error()
	switch {
	case contains(errStr, "connection refused"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeTCPRefused), "TCP connection refused", 0)
	case contains(errStr, "timeout"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeTCPTimeout), "TCP timeout", 0)
	default:
		return LayerResultError(err, string(k8swatchv1.FailureCodeTCPTimeout), 0)
	}
}

// HTTPProtocolLayer implements L4 HTTP protocol check
type HTTPProtocolLayer struct{}

// NewHTTPProtocolLayer creates a new HTTP protocol layer
func NewHTTPProtocolLayer() *HTTPProtocolLayer {
	return &HTTPProtocolLayer{}
}

// Name returns the layer name
func (l *HTTPProtocolLayer) Name() string {
	return "L4"
}

// Enabled returns whether this layer is enabled for the target
func (l *HTTPProtocolLayer) Enabled(target *k8swatchv1.Target) bool {
	return target.Spec.Type == k8swatchv1.TargetTypeHTTP ||
		target.Spec.Type == k8swatchv1.TargetTypeHTTPS ||
		(target.Spec.Layers.L4Protocol != nil && target.Spec.Layers.L4Protocol.Enabled)
}

// Check executes the HTTP protocol check
func (l *HTTPProtocolLayer) Check(ctx context.Context, target *k8swatchv1.Target) (*LayerResult, error) {
	startTime := time.Now()

	// Build HTTP client
	client, err := l.buildHTTPClient(target)
	if err != nil {
		return LayerResultError(err, string(k8swatchv1.FailureCodeConfigError), time.Since(startTime).Milliseconds()), nil
	}

	// Build request
	req, err := l.buildHTTPRequest(target)
	if err != nil {
		return LayerResultError(err, string(k8swatchv1.FailureCodeConfigError), time.Since(startTime).Milliseconds()), nil
	}

	// Execute request
	req = req.WithContext(ctx)
	resp, err := client.Do(req)
	duration := time.Since(startTime).Milliseconds()

	if err != nil {
		return l.handleHTTPError(err), nil
	}
	defer resp.Body.Close()

	// Check status code
	expectedCode := l.getExpectedStatusCode(target)
	if !l.isStatusCodeExpected(resp.StatusCode, expectedCode) {
		return LayerResultFailure(
			string(k8swatchv1.FailureCodeProtocolUnexpectedResp),
			fmt.Sprintf("HTTP status %d, expected %d", resp.StatusCode, expectedCode),
			duration,
		), nil
	}

	return LayerResultSuccess(duration), nil
}

// buildHTTPClient builds an HTTP client with appropriate configuration
func (l *HTTPProtocolLayer) buildHTTPClient(target *k8swatchv1.Target) (*http.Client, error) {
	timeout := 10 * time.Second
	if target.Spec.Schedule.Timeout != "" {
		if d, err := time.ParseDuration(target.Spec.Schedule.Timeout); err == nil {
			timeout = d
		}
	}

	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   timeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   timeout,
		ExpectContinueTimeout: timeout,
	}

	// Configure TLS for HTTPS
	if target.Spec.Type == k8swatchv1.TargetTypeHTTPS {
		tlsConfig := &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: false,
		}
		if target.Spec.Layers.L3TLS != nil && target.Spec.Layers.L3TLS.InsecureSkipVerify {
			tlsConfig.InsecureSkipVerify = true
		}
		transport.TLSClientConfig = tlsConfig
	}

	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Don't follow redirects by default
			return http.ErrUseLastResponse
		},
	}, nil
}

// buildHTTPRequest builds an HTTP request
func (l *HTTPProtocolLayer) buildHTTPRequest(target *k8swatchv1.Target) (*http.Request, error) {
	targetURL, err := l.getTargetURL(target)
	if err != nil {
		return nil, err
	}

	method := http.MethodGet
	if target.Spec.Layers.L4Protocol != nil && target.Spec.Layers.L4Protocol.Method != "" {
		method = target.Spec.Layers.L4Protocol.Method
	}

	var body io.Reader
	if target.Spec.Layers.L4Protocol != nil && target.Spec.Layers.L4Protocol.Body != "" {
		body = strings.NewReader(target.Spec.Layers.L4Protocol.Body)
	}

	req, err := http.NewRequest(method, targetURL.String(), body)
	if err != nil {
		return nil, err
	}

	// Add headers
	req.Header.Set("User-Agent", "K8sWatch/1.0")
	if target.Spec.Layers.L4Protocol != nil {
		for k, v := range target.Spec.Layers.L4Protocol.Headers {
			req.Header.Set(k, v)
		}
	}

	return req, nil
}

// getTargetURL constructs target URL for HTTP layer
func (l *HTTPProtocolLayer) getTargetURL(target *k8swatchv1.Target) (*url.URL, error) {
	endpoint := &target.Spec.Endpoint
	port := getPort(target, 80)
	if target.Spec.Type == k8swatchv1.TargetTypeHTTPS {
		port = getPort(target, 443)
	}

	var scheme string
	if target.Spec.Type == k8swatchv1.TargetTypeHTTPS {
		scheme = "https"
	} else {
		scheme = "http"
	}

	path := getPath(target, "/")

	if endpoint.DNS != nil && *endpoint.DNS != "" {
		return &url.URL{
			Scheme: scheme,
			Host:   fmt.Sprintf("%s:%d", *endpoint.DNS, port),
			Path:   path,
		}, nil
	}

	if endpoint.IP != nil && *endpoint.IP != "" {
		return &url.URL{
			Scheme: scheme,
			Host:   fmt.Sprintf("%s:%d", *endpoint.IP, port),
			Path:   path,
		}, nil
	}

	if endpoint.K8sService != nil {
		ns := endpoint.K8sService.Namespace
		if ns == "" {
			ns = "default"
		}
		hostname := fmt.Sprintf("%s.%s.svc.cluster.local", endpoint.K8sService.Name, ns)
		return &url.URL{
			Scheme: scheme,
			Host:   fmt.Sprintf("%s:%d", hostname, port),
			Path:   path,
		}, nil
	}

	return nil, fmt.Errorf("no endpoint configured")
}

// getExpectedStatusCode returns the expected status code
func (l *HTTPProtocolLayer) getExpectedStatusCode(target *k8swatchv1.Target) int {
	if target.Spec.Layers.L4Protocol != nil && target.Spec.Layers.L4Protocol.StatusCode != nil {
		return int(*target.Spec.Layers.L4Protocol.StatusCode)
	}
	// Default: accept 2xx and 3xx
	return 200
}

// isStatusCodeExpected checks if status code is in expected range
func (l *HTTPProtocolLayer) isStatusCodeExpected(code, expected int) bool {
	if expected > 0 {
		return code == expected
	}
	// Default: accept 2xx and 3xx
	return (code >= 200 && code < 400)
}

// handleHTTPError converts HTTP errors to failure codes
func (l *HTTPProtocolLayer) handleHTTPError(err error) *LayerResult {
	errStr := err.Error()

	switch {
	case contains(errStr, "timeout"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeProtocolTimeout), "HTTP timeout", 0)
	case contains(errStr, "connection refused"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeTCPRefused), "HTTP connection refused", 0)
	default:
		return LayerResultError(err, string(k8swatchv1.FailureCodeProtocolError), 0)
	}
}

// HTTPAuthLayer implements L5 HTTP authentication check
type HTTPAuthLayer struct{}

// NewHTTPAuthLayer creates a new HTTP auth layer
func NewHTTPAuthLayer() *HTTPAuthLayer {
	return &HTTPAuthLayer{}
}

// Name returns the layer name
func (l *HTTPAuthLayer) Name() string {
	return "L5"
}

// Enabled returns whether this layer is enabled for the target
func (l *HTTPAuthLayer) Enabled(target *k8swatchv1.Target) bool {
	return target.Spec.Layers.L5Auth != nil && target.Spec.Layers.L5Auth.Enabled
}

// Check executes the HTTP authentication check
func (l *HTTPAuthLayer) Check(ctx context.Context, target *k8swatchv1.Target) (*LayerResult, error) {
	startTime := time.Now()

	// Build HTTP client
	client, err := l.buildHTTPClient(target)
	if err != nil {
		return LayerResultError(err, string(k8swatchv1.FailureCodeConfigError), time.Since(startTime).Milliseconds()), nil
	}

	// Build request with auth
	req, err := l.buildAuthHTTPRequest(target)
	if err != nil {
		return LayerResultError(err, string(k8swatchv1.FailureCodeConfigError), time.Since(startTime).Milliseconds()), nil
	}

	req = req.WithContext(ctx)
	resp, err := client.Do(req)
	duration := time.Since(startTime).Milliseconds()

	if err != nil {
		return l.handleHTTPError(err), nil
	}
	defer resp.Body.Close()

	// Check for auth failure
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return LayerResultFailure(
			string(k8swatchv1.FailureCodeAuthFailed),
			fmt.Sprintf("HTTP auth failed: %d", resp.StatusCode),
			duration,
		), nil
	}

	return LayerResultSuccess(duration), nil
}

// buildHTTPClient builds HTTP client for auth layer
func (l *HTTPAuthLayer) buildHTTPClient(target *k8swatchv1.Target) (*http.Client, error) {
	timeout := 10 * time.Second
	if target.Spec.Schedule.Timeout != "" {
		if d, err := time.ParseDuration(target.Spec.Schedule.Timeout); err == nil {
			timeout = d
		}
	}

	return &http.Client{
		Timeout: timeout,
	}, nil
}

// buildAuthHTTPRequest builds HTTP request with authentication
func (l *HTTPAuthLayer) buildAuthHTTPRequest(target *k8swatchv1.Target) (*http.Request, error) {
	protocolLayer := NewHTTPProtocolLayer()
	req, err := protocolLayer.buildHTTPRequest(target)
	if err != nil {
		return nil, err
	}

	// Add auth headers based on config
	authConfig := target.Spec.Layers.L5Auth
	if authConfig == nil {
		return req, nil
	}

	// Bearer token
	if authConfig.Token != "" {
		req.Header.Set("Authorization", "Bearer "+authConfig.Token)
	}

	if authConfig.CredentialsRef != nil {
		creds, err := loadCredentialsRef(context.Background(), target.Namespace, authConfig.CredentialsRef)
		if err != nil {
			return nil, fmt.Errorf("failed to load credentials from secret: %w", err)
		}

		authType := strings.ToLower(authConfig.AuthType)
		switch authType {
		case "basic":
			req.SetBasicAuth(creds.username, creds.password)
		case "apikey":
			if creds.token != "" {
				req.Header.Set("X-API-Key", creds.token)
			}
		case "", "bearer", "mtls":
			if creds.token != "" {
				req.Header.Set("Authorization", "Bearer "+creds.token)
			}
		default:
			return nil, fmt.Errorf("unsupported auth type: %s", authConfig.AuthType)
		}
	}

	return req, nil
}

// handleHTTPError converts HTTP errors for auth layer
func (l *HTTPAuthLayer) handleHTTPError(err error) *LayerResult {
	errStr := err.Error()
	switch {
	case contains(errStr, "timeout"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeAuthTimeout), "Auth timeout", 0)
	default:
		return LayerResultError(err, string(k8swatchv1.FailureCodeAuthFailed), 0)
	}
}

// HTTPSemanticLayer implements L6 HTTP semantic check
type HTTPSemanticLayer struct{}

// NewHTTPSemanticLayer creates a new HTTP semantic layer
func NewHTTPSemanticLayer() *HTTPSemanticLayer {
	return &HTTPSemanticLayer{}
}

// Name returns the layer name
func (l *HTTPSemanticLayer) Name() string {
	return "L6"
}

// Enabled returns whether this layer is enabled for the target
func (l *HTTPSemanticLayer) Enabled(target *k8swatchv1.Target) bool {
	return target.Spec.Layers.L6Semantic != nil && target.Spec.Layers.L6Semantic.Enabled
}

// Check executes the HTTP semantic check
func (l *HTTPSemanticLayer) Check(ctx context.Context, target *k8swatchv1.Target) (*LayerResult, error) {
	startTime := time.Now()

	// Build HTTP client
	client, err := l.buildHTTPClient(target)
	if err != nil {
		return LayerResultError(err, string(k8swatchv1.FailureCodeConfigError), time.Since(startTime).Milliseconds()), nil
	}

	// Build request
	protocolLayer := NewHTTPProtocolLayer()
	req, err := protocolLayer.buildHTTPRequest(target)
	if err != nil {
		return LayerResultError(err, string(k8swatchv1.FailureCodeConfigError), time.Since(startTime).Milliseconds()), nil
	}

	req = req.WithContext(ctx)
	resp, err := client.Do(req)
	duration := time.Since(startTime).Milliseconds()

	if err != nil {
		return l.handleHTTPError(err), nil
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return LayerResultError(err, string(k8swatchv1.FailureCodeSemanticFailed), duration), nil
	}

	// Check expected content
	semConfig := target.Spec.Layers.L6Semantic
	if semConfig == nil {
		return LayerResultSuccess(duration), nil
	}

	if semConfig.ExpectedContent != "" {
		if !strings.Contains(string(body), semConfig.ExpectedContent) {
			return LayerResultFailure(
				string(k8swatchv1.FailureCodeSemanticUnexpected),
				"Expected content not found",
				duration,
			), nil
		}
	}

	if semConfig.JSONPath != "" {
		value, err := evaluateSimpleJSONPath(body, semConfig.JSONPath)
		if err != nil {
			return LayerResultFailure(
				string(k8swatchv1.FailureCodeSemanticUnexpected),
				fmt.Sprintf("JSONPath evaluation failed: %v", err),
				duration,
			), nil
		}

		if semConfig.ExpectedValue != "" && value != semConfig.ExpectedValue {
			return LayerResultFailure(
				string(k8swatchv1.FailureCodeSemanticUnexpected),
				fmt.Sprintf("JSONPath value mismatch: got %q expected %q", value, semConfig.ExpectedValue),
				duration,
			), nil
		}
	}

	if semConfig.Regex != "" {
		re, err := regexp.Compile(semConfig.Regex)
		if err != nil {
			return LayerResultFailure(
				string(k8swatchv1.FailureCodeConfigError),
				fmt.Sprintf("invalid regex: %v", err),
				duration,
			), nil
		}
		if !re.Match(body) {
			return LayerResultFailure(
				string(k8swatchv1.FailureCodeSemanticUnexpected),
				"regex did not match response body",
				duration,
			), nil
		}
	}

	return LayerResultSuccess(duration), nil
}

// buildHTTPClient builds HTTP client for semantic layer
func (l *HTTPSemanticLayer) buildHTTPClient(target *k8swatchv1.Target) (*http.Client, error) {
	timeout := 10 * time.Second
	if target.Spec.Schedule.Timeout != "" {
		if d, err := time.ParseDuration(target.Spec.Schedule.Timeout); err == nil {
			timeout = d
		}
	}

	return &http.Client{
		Timeout: timeout,
	}, nil
}

// handleHTTPError converts HTTP errors for semantic layer
func (l *HTTPSemanticLayer) handleHTTPError(err error) *LayerResult {
	errStr := err.Error()
	switch {
	case contains(errStr, "timeout"):
		return LayerResultFailure(string(k8swatchv1.FailureCodeSemanticTimeout), "Semantic check timeout", 0)
	default:
		return LayerResultError(err, string(k8swatchv1.FailureCodeSemanticFailed), 0)
	}
}

// Helper functions
func getPort(target *k8swatchv1.Target, defaultPort int) int {
	if target.Spec.Endpoint.Port != nil {
		return int(*target.Spec.Endpoint.Port)
	}
	return defaultPort
}

func getPath(target *k8swatchv1.Target, defaultPath string) string {
	if target.Spec.Endpoint.Path != nil {
		return *target.Spec.Endpoint.Path
	}
	return defaultPath
}

func evaluateSimpleJSONPath(data []byte, path string) (string, error) {
	var parsed interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", fmt.Errorf("invalid JSON response: %w", err)
	}

	trimmed := strings.TrimSpace(path)
	if trimmed == "" || trimmed == "$" {
		return jsonValueToString(parsed), nil
	}
	trimmed = strings.TrimPrefix(trimmed, "$.")

	current := parsed
	segments := strings.Split(trimmed, ".")
	for _, segment := range segments {
		key := segment
		index := -1

		if left := strings.Index(segment, "["); left != -1 && strings.HasSuffix(segment, "]") {
			key = segment[:left]
			idxStr := segment[left+1 : len(segment)-1]
			i, err := strconv.Atoi(idxStr)
			if err != nil {
				return "", fmt.Errorf("invalid JSONPath index %q", idxStr)
			}
			index = i
		}

		if key != "" {
			obj, ok := current.(map[string]interface{})
			if !ok {
				return "", fmt.Errorf("path segment %q is not an object", key)
			}
			val, ok := obj[key]
			if !ok {
				return "", fmt.Errorf("path segment %q not found", key)
			}
			current = val
		}

		if index >= 0 {
			arr, ok := current.([]interface{})
			if !ok {
				return "", fmt.Errorf("path segment %q is not an array", segment)
			}
			if index >= len(arr) {
				return "", fmt.Errorf("array index %d out of range", index)
			}
			current = arr[index]
		}
	}

	return jsonValueToString(current), nil
}

func jsonValueToString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(val)
	case nil:
		return ""
	default:
		b, err := json.Marshal(val)
		if err != nil {
			return fmt.Sprintf("%v", val)
		}
		return string(b)
	}
}
