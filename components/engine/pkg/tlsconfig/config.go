// Package tlsconfig provides primitives to retrieve secure-enough TLS configurations for both clients and servers.
//
// As a reminder from https://golang.org/pkg/crypto/tls/#Config:
//	A Config structure is used to configure a TLS client or server. After one has been passed to a TLS function it must not be modified.
//	A Config may be reused; the tls package will also not modify it.
package tlsconfig

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/Sirupsen/logrus"
)

// Options represents the information needed to create client and server TLS configurations.
type Options struct {
	InsecureSkipVerify bool
	ClientAuth         tls.ClientAuthType
	CAFile             string
	CertFile           string
	KeyFile            string
}

// Default is a secure-enough TLS configuration.
var Default = tls.Config{
	// Avoid fallback to SSL protocols < TLS1.0
	MinVersion:               tls.VersionTLS10,
	PreferServerCipherSuites: true,
	CipherSuites: []uint16{
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
		tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
		tls.TLS_RSA_WITH_AES_128_CBC_SHA,
		tls.TLS_RSA_WITH_AES_256_CBC_SHA,
	},
}

// certPool returns an X.509 certificate pool from `caFile`, the certificate file.
func certPool(caFile string) (*x509.CertPool, error) {
	// If we should verify the server, we need to load a trusted ca
	certPool := x509.NewCertPool()
	pem, err := ioutil.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("Could not read CA certificate %s: %v", caFile, err)
	}
	if !certPool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("failed to append certificates from PEM file: %s", caFile)
	}
	s := certPool.Subjects()
	subjects := make([]string, len(s))
	for i, subject := range s {
		subjects[i] = string(subject)
	}
	logrus.Debugf("Trusting certs with subjects: %v", subjects)
	return certPool, nil
}

// Client returns a TLS configuration meant to be used by a client.
func Client(options Options) (*tls.Config, error) {
	tlsConfig := Default
	tlsConfig.InsecureSkipVerify = options.InsecureSkipVerify
	if !options.InsecureSkipVerify {
		CAs, err := certPool(options.CAFile)
		if err != nil {
			return nil, err
		}
		tlsConfig.RootCAs = CAs
	}

	if options.CertFile != "" && options.KeyFile != "" {
		tlsCert, err := tls.LoadX509KeyPair(options.CertFile, options.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("Could not load X509 key pair: %v. Make sure the key is not encrypted", err)
		}
		tlsConfig.Certificates = []tls.Certificate{tlsCert}
	}

	return &tlsConfig, nil
}

// Server returns a TLS configuration meant to be used by a server.
func Server(options Options) (*tls.Config, error) {
	tlsConfig := Default
	tlsConfig.ClientAuth = options.ClientAuth
	tlsCert, err := tls.LoadX509KeyPair(options.CertFile, options.KeyFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("Could not load X509 key pair (%s, %s): %v", options.CertFile, options.KeyFile, err)
		}
		return nil, fmt.Errorf("Error reading X509 key pair (%s, %s): %v. Make sure the key is not encrypted.", options.CertFile, options.KeyFile, err)
	}
	tlsConfig.Certificates = []tls.Certificate{tlsCert}
	if options.ClientAuth >= tls.VerifyClientCertIfGiven {
		CAs, err := certPool(options.CAFile)
		if err != nil {
			return nil, err
		}
		tlsConfig.ClientCAs = CAs
	}
	return &tlsConfig, nil
}
