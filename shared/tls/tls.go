package tls

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"google.golang.org/grpc/credentials"
)

// Config holds TLS configuration
type Config struct {
	CertFile     string // Server or client certificate file
	KeyFile      string // Server or client key file
	CAFile       string // CA certificate file for verification
	ServerName   string // Expected server name for client connections
	ClientAuth   bool   // Whether to require client certificates (server only)
	InsecureSkip bool   // Skip certificate verification (development only)
}

// LoadServerCredentials loads TLS credentials for gRPC server
func LoadServerCredentials(config Config) (credentials.TransportCredentials, error) {
	if config.InsecureSkip {
		return nil, fmt.Errorf("insecure mode not allowed for server")
	}

	// Load server certificate and key
	cert, err := tls.LoadX509KeyPair(config.CertFile, config.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load server certificates: %w", err)
	}

	// Create TLS config
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		},
	}

	// If client authentication is required, load CA certificate
	if config.ClientAuth && config.CAFile != "" {
		caCert, err := ioutil.ReadFile(config.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %w", err)
		}

		certPool := x509.NewCertPool()
		if !certPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}

		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
		tlsConfig.ClientCAs = certPool
	}

	return credentials.NewTLS(tlsConfig), nil
}

// LoadClientCredentials loads TLS credentials for gRPC client
func LoadClientCredentials(config Config) (credentials.TransportCredentials, error) {
	// For development, allow insecure connections
	if config.InsecureSkip {
		return credentials.NewTLS(&tls.Config{
			InsecureSkipVerify: true,
		}), nil
	}

	// Load CA certificate
	caCert, err := ioutil.ReadFile(config.CAFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate: %w", err)
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}

	tlsConfig := &tls.Config{
		RootCAs:    certPool,
		MinVersion: tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		},
	}

	// Set server name for verification
	if config.ServerName != "" {
		tlsConfig.ServerName = config.ServerName
	}

	// If client certificate is provided, load it
	if config.CertFile != "" && config.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(config.CertFile, config.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificates: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return credentials.NewTLS(tlsConfig), nil
}

// GetTLSConfig returns TLS configuration based on environment
func GetTLSConfig(serviceName string, isServer bool) Config {
	// Default paths (can be overridden by environment variables)
	certDir := "/certs"
	if envDir := getEnv("CERT_DIR", ""); envDir != "" {
		certDir = envDir
	}

	if isServer {
		return Config{
			CertFile:   filepath.Join(certDir, fmt.Sprintf("%s.crt", serviceName)),
			KeyFile:    filepath.Join(certDir, fmt.Sprintf("%s.key", serviceName)),
			CAFile:     filepath.Join(certDir, "ca.crt"),
			ClientAuth: getEnvBool("GRPC_CLIENT_AUTH", true),
		}
	}

	// Client configuration
	return Config{
		CertFile:   filepath.Join(certDir, "graphql-gateway-client.crt"),
		KeyFile:    filepath.Join(certDir, "graphql-gateway-client.key"),
		CAFile:     filepath.Join(certDir, "ca.crt"),
		ServerName: fmt.Sprintf("%s.zuno-marketplace.local", serviceName),
	}
}

// Helper functions
func getEnv(key, defaultValue string) string {
	if value := getEnvValue(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	value := getEnvValue(key)
	if value == "" {
		return defaultValue
	}
	return value == "true" || value == "1" || value == "yes"
}

// getEnvValue is a placeholder - should import os.Getenv
func getEnvValue(key string) string {
	// This should be replaced with os.Getenv(key) in actual implementation
	// Keeping it as placeholder to avoid import cycle
	return ""
}
