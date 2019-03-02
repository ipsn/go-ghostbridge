// go-ghostbridge - React Native to Go bridge
// Copyright (c) 2019 Péter Szilágyi. All rights reserved.

package ghostbridge

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"testing"
)

// Test that a new, self-signed TLS certificate can be generated and an HTTPS
// server spun up with it.
func TestBridge(t *testing.T) {
	// Create a TLS bridge to serve some requests
	bridge, err := New(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Yay, it works!"))
	}))
	if err != nil {
		t.Fatalf("Failed to create self-signed TLS bridge")
	}
	defer bridge.Close()

	// Create a TLS client with the self-signed certificate injected
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM([]byte(bridge.Cert())) {
		t.Fatalf("Failed to load server certificate")
	}
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: roots,
			},
		},
	}
	// Assemble the authenticated HTTP request and ensure it executes ok
	req, err := http.NewRequest("GET", fmt.Sprintf("https://localhost:%d", bridge.Port()), nil)
	if err != nil {
		t.Fatalf("Failed to create HTTP request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+bridge.Token())

	res, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to execute HTTP request: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("Invalid status returned from bridge: have %v, want %v", res.StatusCode, http.StatusOK)
	}
	res.Body.Close()
}
