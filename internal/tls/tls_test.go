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

package tls

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// generateTestCertificate generates a self-signed certificate for testing
func generateTestCertificate(t *testing.T, commonName string, isCA bool) (certPath, keyPath string, cleanup func()) {
	t.Helper()

	// Generate private key
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// Generate certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   commonName,
			Organization: []string{"K8sWatch Test"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}

	if isCA {
		template.IsCA = true
		template.KeyUsage |= x509.KeyUsageCertSign
	}

	// Self-sign the certificate
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	require.NoError(t, err)

	// Create temp files
	certFile, err := os.CreateTemp("", "cert-*.pem")
	require.NoError(t, err)
	keyFile, err := os.CreateTemp("", "key-*.pem")
	require.NoError(t, err)

	// Write certificate (PEM format)
	err = pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	require.NoError(t, err)
	certPath = certFile.Name()

	// Write key (PEM format)
	keyBytes := x509.MarshalPKCS1PrivateKey(priv)
	err = pem.Encode(keyFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyBytes})
	require.NoError(t, err)
	keyPath = keyFile.Name()

	// Close files
	certFile.Close()
	keyFile.Close()

	cleanup = func() {
		os.Remove(certPath)
		os.Remove(keyPath)
	}

	return certPath, keyPath, cleanup
}

// generateCACertificate generates a CA certificate for testing
func generateCACertificate(t *testing.T) (caCertPath string, cleanup func()) {
	t.Helper()

	// Generate CA private key
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// Generate CA certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "K8sWatch Test CA",
			Organization: []string{"K8sWatch Test"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
	}

	// Self-sign the CA certificate
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	require.NoError(t, err)

	// Create temp file for CA cert
	caFile, err := os.CreateTemp("", "ca-*.pem")
	require.NoError(t, err)

	// Write CA certificate
	err = pem.Encode(caFile, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	require.NoError(t, err)
	caCertPath = caFile.Name()
	caFile.Close()

	cleanup = func() {
		os.Remove(caCertPath)
	}

	return caCertPath, cleanup
}

func TestTLSConfigDefaults(t *testing.T) {
	cfg := &TLSConfig{}
	assert.Empty(t, cfg.CertFile)
	assert.Empty(t, cfg.KeyFile)
	assert.Empty(t, cfg.CAFile)
	assert.Empty(t, cfg.ServerName)
	assert.False(t, cfg.InsecureSkipVerify)
}

func TestLoadClientTLSConfigInsecure(t *testing.T) {
	cfg := &TLSConfig{
		InsecureSkipVerify: true,
	}

	tlsConfig, err := LoadClientTLSConfig(cfg)
	require.NoError(t, err)
	assert.NotNil(t, tlsConfig)
	assert.True(t, tlsConfig.InsecureSkipVerify)
}

func TestLoadClientTLSConfigValid(t *testing.T) {
	// Generate test certificate
	certPath, keyPath, cleanup := generateTestCertificate(t, "client.test", false)
	defer cleanup()

	// Generate CA
	caPath, caCleanup := generateCACertificate(t)
	defer caCleanup()

	cfg := &TLSConfig{
		CertFile:   certPath,
		KeyFile:    keyPath,
		CAFile:     caPath,
		ServerName: "client.test",
	}

	tlsConfig, err := LoadClientTLSConfig(cfg)
	require.NoError(t, err)
	assert.NotNil(t, tlsConfig)
	assert.False(t, tlsConfig.InsecureSkipVerify)
	assert.Equal(t, uint16(tls.VersionTLS13), tlsConfig.MinVersion)
	assert.Equal(t, "client.test", tlsConfig.ServerName)
	assert.Len(t, tlsConfig.Certificates, 1)
	assert.NotNil(t, tlsConfig.RootCAs)
}

func TestLoadClientTLSConfigInvalidCert(t *testing.T) {
	cfg := &TLSConfig{
		CertFile: "/nonexistent/cert.pem",
		KeyFile:  "/nonexistent/key.pem",
		CAFile:   "/nonexistent/ca.pem",
	}

	_, err := LoadClientTLSConfig(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load client certificate")
}

func TestLoadClientTLSConfigInvalidCA(t *testing.T) {
	// Generate valid client cert
	certPath, keyPath, cleanup := generateTestCertificate(t, "client.test", false)
	defer cleanup()

	cfg := &TLSConfig{
		CertFile: certPath,
		KeyFile:  keyPath,
		CAFile:   "/nonexistent/ca.pem",
	}

	_, err := LoadClientTLSConfig(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load CA certificate")
}

func TestLoadServerTLSConfigInsecure(t *testing.T) {
	cfg := &TLSConfig{
		InsecureSkipVerify: true,
	}

	tlsConfig, err := LoadServerTLSConfig(cfg)
	require.NoError(t, err)
	assert.NotNil(t, tlsConfig)
	assert.True(t, tlsConfig.InsecureSkipVerify)
}

func TestLoadServerTLSConfigValid(t *testing.T) {
	// Generate test certificate
	certPath, keyPath, cleanup := generateTestCertificate(t, "server.test", false)
	defer cleanup()

	// Generate CA
	caPath, caCleanup := generateCACertificate(t)
	defer caCleanup()

	cfg := &TLSConfig{
		CertFile:   certPath,
		KeyFile:    keyPath,
		CAFile:     caPath,
		ServerName: "server.test",
	}

	tlsConfig, err := LoadServerTLSConfig(cfg)
	require.NoError(t, err)
	assert.NotNil(t, tlsConfig)
	assert.False(t, tlsConfig.InsecureSkipVerify)
	assert.Equal(t, uint16(tls.VersionTLS13), tlsConfig.MinVersion)
	assert.Equal(t, tls.RequireAndVerifyClientCert, tlsConfig.ClientAuth)
	assert.Len(t, tlsConfig.Certificates, 1)
	assert.NotNil(t, tlsConfig.ClientCAs)
}

func TestLoadServerTLSConfigInvalidCert(t *testing.T) {
	cfg := &TLSConfig{
		CertFile: "/nonexistent/cert.pem",
		KeyFile:  "/nonexistent/key.pem",
		CAFile:   "/nonexistent/ca.pem",
	}

	_, err := LoadServerTLSConfig(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load server certificate")
}

func TestLoadServerTLSConfigOptionalValid(t *testing.T) {
	// Generate test certificate
	certPath, keyPath, cleanup := generateTestCertificate(t, "server.test", false)
	defer cleanup()

	// Generate CA
	caPath, caCleanup := generateCACertificate(t)
	defer caCleanup()

	cfg := &TLSConfig{
		CertFile:   certPath,
		KeyFile:    keyPath,
		CAFile:     caPath,
		ServerName: "server.test",
	}

	tlsConfig, err := LoadServerTLSConfigOptional(cfg)
	require.NoError(t, err)
	assert.NotNil(t, tlsConfig)
	assert.False(t, tlsConfig.InsecureSkipVerify)
	assert.Equal(t, uint16(tls.VersionTLS13), tlsConfig.MinVersion)
	assert.Equal(t, tls.VerifyClientCertIfGiven, tlsConfig.ClientAuth)
	assert.Len(t, tlsConfig.Certificates, 1)
	assert.NotNil(t, tlsConfig.ClientCAs)
}

func TestLoadServerTLSConfigOptionalInvalidCert(t *testing.T) {
	cfg := &TLSConfig{
		CertFile: "/nonexistent/cert.pem",
		KeyFile:  "/nonexistent/key.pem",
		CAFile:   "/nonexistent/ca.pem",
	}

	_, err := LoadServerTLSConfigOptional(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load server certificate")
}

func TestLoadCACertPoolNotFound(t *testing.T) {
	_, err := loadCACertPool("/nonexistent/ca.pem")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read CA certificate")
}

func TestLoadCACertPoolEmptyFile(t *testing.T) {
	// Create empty temp file
	tmpFile, err := os.CreateTemp("", "empty-*.pem")
	require.NoError(t, err)
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	_, err = loadCACertPool(tmpFile.Name())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse CA certificate")
}

func TestLoadCACertPoolInvalidPEM(t *testing.T) {
	// Create temp file with invalid content
	tmpFile, err := os.CreateTemp("", "invalid-*.pem")
	require.NoError(t, err)
	_, err = tmpFile.WriteString("not a valid PEM file")
	require.NoError(t, err)
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	_, err = loadCACertPool(tmpFile.Name())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse CA certificate")
}

func TestValidateCertificateValid(t *testing.T) {
	certPath, _, cleanup := generateTestCertificate(t, "test.test", false)
	defer cleanup()

	err := ValidateCertificate(certPath)
	assert.NoError(t, err)
}

func TestValidateCertificateNotFound(t *testing.T) {
	err := ValidateCertificate("/nonexistent/cert.pem")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read certificate")
}

func TestValidateCertificateInvalid(t *testing.T) {
	// Create temp file with invalid content
	tmpFile, err := os.CreateTemp("", "invalid-*.pem")
	require.NoError(t, err)
	_, err = tmpFile.WriteString("not a valid certificate")
	require.NoError(t, err)
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	err = ValidateCertificate(tmpFile.Name())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode PEM block")
}

func TestValidateKeyPairValid(t *testing.T) {
	certPath, keyPath, cleanup := generateTestCertificate(t, "test.test", false)
	defer cleanup()

	err := ValidateKeyPair(certPath, keyPath)
	assert.NoError(t, err)
}

func TestValidateKeyPairNotFound(t *testing.T) {
	err := ValidateKeyPair("/nonexistent/cert.pem", "/nonexistent/key.pem")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid key pair")
}

func TestValidateKeyPairMismatch(t *testing.T) {
	// Generate two different certificates
	certPath1, keyPath1, cleanup1 := generateTestCertificate(t, "test1.test", false)
	defer cleanup1()
	certPath2, keyPath2, cleanup2 := generateTestCertificate(t, "test2.test", false)
	defer cleanup2()

	// Try to validate mismatched pair
	err := ValidateKeyPair(certPath1, keyPath2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid key pair")

	err = ValidateKeyPair(certPath2, keyPath1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid key pair")
}

func TestTLSConfigWithRelativePaths(t *testing.T) {
	// Generate certificates in temp directory
	tmpDir, err := os.MkdirTemp("", "tls-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Change to temp directory
	origDir, _ := os.Getwd()
	err = os.Chdir(tmpDir)
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()

	// Generate certificates with relative paths
	certPath, keyPath, cleanup := generateTestCertificate(t, "test.test", false)
	defer cleanup()

	// Get relative paths
	relCertPath, _ := filepath.Rel(tmpDir, certPath)
	relKeyPath, _ := filepath.Rel(tmpDir, keyPath)

	cfg := &TLSConfig{
		CertFile: relCertPath,
		KeyFile:  relKeyPath,
	}

	// Should work with relative paths
	_, err = tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	assert.NoError(t, err)
}

func TestTLSConfigServerName(t *testing.T) {
	certPath, keyPath, cleanup := generateTestCertificate(t, "example.com", false)
	defer cleanup()
	caPath, caCleanup := generateCACertificate(t)
	defer caCleanup()

	cfg := &TLSConfig{
		CertFile:   certPath,
		KeyFile:    keyPath,
		CAFile:     caPath,
		ServerName: "example.com",
	}

	tlsConfig, err := LoadClientTLSConfig(cfg)
	require.NoError(t, err)
	assert.Equal(t, "example.com", tlsConfig.ServerName)
}

func TestTLSConfigMinVersion(t *testing.T) {
	certPath, keyPath, cleanup := generateTestCertificate(t, "test.test", false)
	defer cleanup()
	caPath, caCleanup := generateCACertificate(t)
	defer caCleanup()

	cfg := &TLSConfig{
		CertFile: certPath,
		KeyFile:  keyPath,
		CAFile:   caPath,
	}

	tlsConfig, err := LoadClientTLSConfig(cfg)
	require.NoError(t, err)
	assert.Equal(t, uint16(tls.VersionTLS13), tlsConfig.MinVersion)
}

func TestLoadCACertPoolNoFile(t *testing.T) {
	_, err := loadCACertPool("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "CA file not specified")
}

func TestLoadCACertPoolMalformedPEM(t *testing.T) {
	// Create temp file with malformed PEM
	tmpFile, err := os.CreateTemp("", "malformed-*.pem")
	require.NoError(t, err)
	_, err = tmpFile.WriteString("-----BEGIN CERTIFICATE-----\ninvalidbase64\n-----END CERTIFICATE-----")
	require.NoError(t, err)
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	_, err = loadCACertPool(tmpFile.Name())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse CA certificate")
}

func TestLoadServerTLSConfigWithClientAuth(t *testing.T) {
	certPath, keyPath, cleanup := generateTestCertificate(t, "server.test", false)
	defer cleanup()
	caPath, caCleanup := generateCACertificate(t)
	defer caCleanup()

	cfg := &TLSConfig{
		CertFile:   certPath,
		KeyFile:    keyPath,
		CAFile:     caPath,
		ServerName: "server.test",
	}

	tlsConfig, err := LoadServerTLSConfig(cfg)
	require.NoError(t, err)
	assert.Equal(t, tls.RequireAndVerifyClientCert, tlsConfig.ClientAuth)
}

func TestLoadServerTLSConfigOptionalNoClientCert(t *testing.T) {
	certPath, keyPath, cleanup := generateTestCertificate(t, "server.test", false)
	defer cleanup()
	caPath, caCleanup := generateCACertificate(t)
	defer caCleanup()

	cfg := &TLSConfig{
		CertFile:   certPath,
		KeyFile:    keyPath,
		CAFile:     caPath,
		ServerName: "server.test",
	}

	tlsConfig, err := LoadServerTLSConfigOptional(cfg)
	require.NoError(t, err)
	assert.Equal(t, tls.VerifyClientCertIfGiven, tlsConfig.ClientAuth)
}

func TestValidateCertificateEmptyFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "empty-*.pem")
	require.NoError(t, err)
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	err = ValidateCertificate(tmpFile.Name())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode PEM block")
}

func TestValidateKeyPairEmptyFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "empty-*.pem")
	require.NoError(t, err)
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	err = ValidateKeyPair(tmpFile.Name(), tmpFile.Name())
	assert.Error(t, err)
}

func TestLoadServerTLSConfigInsecureSkipVerify(t *testing.T) {
	cfg := &TLSConfig{
		InsecureSkipVerify: true,
	}

	tlsConfig, err := LoadServerTLSConfig(cfg)
	require.NoError(t, err)
	assert.NotNil(t, tlsConfig)
	assert.True(t, tlsConfig.InsecureSkipVerify)
}

func TestLoadServerTLSConfigOptionalInsecureSkipVerify(t *testing.T) {
	cfg := &TLSConfig{
		InsecureSkipVerify: true,
	}

	tlsConfig, err := LoadServerTLSConfigOptional(cfg)
	require.NoError(t, err)
	assert.NotNil(t, tlsConfig)
	assert.True(t, tlsConfig.InsecureSkipVerify)
}

func TestValidateCertificateWithValidPEM(t *testing.T) {
	certPath, _, cleanup := generateTestCertificate(t, "test.example.com", false)
	defer cleanup()

	err := ValidateCertificate(certPath)
	assert.NoError(t, err)
}

func TestValidateCertificateReadError(t *testing.T) {
	// Try to read from a directory (should fail)
	err := ValidateCertificate("/tmp")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read certificate")
}

func TestValidateKeyPairWithValidPair(t *testing.T) {
	certPath, keyPath, cleanup := generateTestCertificate(t, "test.example.com", false)
	defer cleanup()

	err := ValidateKeyPair(certPath, keyPath)
	assert.NoError(t, err)
}

func TestLoadServerTLSConfigWithInvalidCert(t *testing.T) {
	cfg := &TLSConfig{
		CertFile: "/nonexistent/cert.pem",
		KeyFile:  "/nonexistent/key.pem",
		CAFile:   "/nonexistent/ca.pem",
	}

	_, err := LoadServerTLSConfig(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load server certificate")
}

func TestLoadServerTLSConfigWithInvalidCA(t *testing.T) {
	certPath, keyPath, cleanup := generateTestCertificate(t, "server.test", false)
	defer cleanup()

	cfg := &TLSConfig{
		CertFile: certPath,
		KeyFile:  keyPath,
		CAFile:   "/nonexistent/ca.pem",
	}

	_, err := LoadServerTLSConfig(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load CA certificate")
}

func TestLoadServerTLSConfigOptionalWithInvalidCert(t *testing.T) {
	cfg := &TLSConfig{
		CertFile: "/nonexistent/cert.pem",
		KeyFile:  "/nonexistent/key.pem",
		CAFile:   "/nonexistent/ca.pem",
	}

	_, err := LoadServerTLSConfigOptional(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load server certificate")
}

func TestLoadServerTLSConfigOptionalWithInvalidCA(t *testing.T) {
	certPath, keyPath, cleanup := generateTestCertificate(t, "server.test", false)
	defer cleanup()

	cfg := &TLSConfig{
		CertFile: certPath,
		KeyFile:  keyPath,
		CAFile:   "/nonexistent/ca.pem",
	}

	_, err := LoadServerTLSConfigOptional(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load CA certificate")
}
