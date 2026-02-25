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

// Package tls provides TLS/mTLS configuration for K8sWatch components
package tls

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

// TLSConfig holds TLS configuration
type TLSConfig struct {
	// CertFile is the path to the certificate file
	CertFile string

	// KeyFile is the path to the private key file
	KeyFile string

	// CAFile is the path to the CA certificate file
	CAFile string

	// ServerName is the expected server name for SNI
	ServerName string

	// InsecureSkipVerify skips certificate verification (development only)
	InsecureSkipVerify bool
}

// LoadClientTLSConfig loads TLS configuration for mTLS client (agent)
func LoadClientTLSConfig(config *TLSConfig) (*tls.Config, error) {
	if config.InsecureSkipVerify {
		return &tls.Config{
			InsecureSkipVerify: true, // nolint:gosec // Development mode only
		}, nil
	}

	// Load client certificate and key
	cert, err := tls.LoadX509KeyPair(config.CertFile, config.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load client certificate: %w", err)
	}

	// Load CA certificate
	certPool, err := loadCACertPool(config.CAFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load CA certificate: %w", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      certPool,
		ServerName:   config.ServerName,
		MinVersion:   tls.VersionTLS13,
	}, nil
}

// LoadServerTLSConfig loads TLS configuration for mTLS server (aggregator)
func LoadServerTLSConfig(config *TLSConfig) (*tls.Config, error) {
	if config.InsecureSkipVerify {
		return &tls.Config{
			InsecureSkipVerify: true, // nolint:gosec // Development mode only
		}, nil
	}

	// Load server certificate and key
	cert, err := tls.LoadX509KeyPair(config.CertFile, config.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load server certificate: %w", err)
	}

	// Load CA certificate for client verification
	certPool, err := loadCACertPool(config.CAFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load CA certificate: %w", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    certPool,
		MinVersion:   tls.VersionTLS13,
	}, nil
}

// LoadServerTLSConfigOptional loads TLS configuration for server with optional client cert
func LoadServerTLSConfigOptional(config *TLSConfig) (*tls.Config, error) {
	if config.InsecureSkipVerify {
		return &tls.Config{
			InsecureSkipVerify: true, // nolint:gosec // Development mode only
		}, nil
	}

	// Load server certificate and key
	cert, err := tls.LoadX509KeyPair(config.CertFile, config.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load server certificate: %w", err)
	}

	// Load CA certificate for client verification
	certPool, err := loadCACertPool(config.CAFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load CA certificate: %w", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.VerifyClientCertIfGiven,
		ClientCAs:    certPool,
		MinVersion:   tls.VersionTLS13,
	}, nil
}

// loadCACertPool loads CA certificate pool from file
func loadCACertPool(caFile string) (*x509.CertPool, error) {
	if caFile == "" {
		return nil, fmt.Errorf("CA file not specified")
	}

	caCert, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate: %w", err)
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}

	return certPool, nil
}

// ValidateCertificate validates a certificate file
func ValidateCertificate(certFile string) error {
	certBytes, err := os.ReadFile(certFile)
	if err != nil {
		return fmt.Errorf("failed to read certificate: %w", err)
	}

	// Decode PEM block
	block, _ := pem.Decode(certBytes)
	if block == nil {
		return fmt.Errorf("failed to decode PEM block")
	}

	_, err = x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("invalid certificate: %w", err)
	}

	return nil
}

// ValidateKeyPair validates a certificate and key pair
func ValidateKeyPair(certFile, keyFile string) error {
	_, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return fmt.Errorf("invalid key pair: %w", err)
	}

	return nil
}
