// go-ghostbridge - React Native to Go bridge
// Copyright (c) 2019 Péter Szilágyi. All rights reserved.

// Package ghostbridge is a secure React Native to Go web bridge.
package ghostbridge

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"time"
)

// Bridge is an HTTPS server that bridges Go and React Native in a secure way,
// providing an encrypted and mutually authenticated data pathway.
type Bridge struct {
	token       string       // Client authorization token to access the HTTPS bridge
	listener    net.Listener // TCP listener accepting the HTTPS connections from React Native
	certificate string       // TLS certificate proving the server's authenticity
}

// New create a new secure web bridge into a Go HTTP server with an authentication
// wrapper built around it, ensuring mobile app security.
func New(handler http.Handler) (*Bridge, error) {
	// Generate a private key for the certificate
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	blob, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, err
	}
	pemPriv := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: blob})

	// Generate the self-signed certificate
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Ghost Bridge"},
		},
		DNSNames:  []string{"localhost"},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(time.Hour * 24 * 365),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	blob, err = x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, err
	}
	pemCert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: blob})

	// Load the certificate and start an HTTPS server with it
	cert, err := tls.X509KeyPair(pemCert, pemPriv)
	if err != nil {
		return nil, err
	}
	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}})
	if err != nil {
		return nil, err
	}
	// Create the verification middleware to authorize the client
	blob = make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, blob); err != nil {
		return nil, err
	}
	token := base64.StdEncoding.EncodeToString(blob)

	go http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}
		handler.ServeHTTP(w, r)
	}))

	return &Bridge{
		token:       token,
		listener:    listener,
		certificate: string(pemCert),
	}, nil
}

// Close terminates the underlying listener, and implicitly the bridge.
func (b *Bridge) Close() error {
	return b.listener.Close()
}

// Port returns the listener port assigned to the bridge.
func (b *Bridge) Port() int {
	return b.listener.Addr().(*net.TCPAddr).Port
}

// Cert returns the TLS certificate assigned to the bridge.
func (b *Bridge) Cert() string {
	return b.certificate
}

// Token returns the client authorization token to access the bridge.
func (b *Bridge) Token() string {
	return b.token
}
