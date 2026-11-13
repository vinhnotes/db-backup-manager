package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func healthCertificate(t *testing.T, name string, expired bool) (tls.Certificate, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	end := time.Now().Add(time.Hour)
	if expired {
		end = time.Now().Add(-time.Hour)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), DNSNames: []string{name}, NotBefore: time.Now().Add(-2 * time.Hour), NotAfter: end, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	if ip := net.ParseIP(name); ip != nil {
		template.DNSNames, template.IPAddresses = nil, []net.IP{ip}
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "cert.pem")
	if err := os.WriteFile(file, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600); err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, file
}

func TestHealthcheckVerifiesLocalCertificate(t *testing.T) {
	for _, name := range []string{"backup.example.com", "*.example.com", "127.0.0.1"} {
		t.Run(name, func(t *testing.T) {
			cert, file := healthCertificate(t, name, false)
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
			server.TLS = &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
			server.StartTLS()
			defer server.Close()
			t.Setenv("LISTEN_ADDR", server.Listener.Addr().String())
			t.Setenv("TLS_MODE", "file")
			t.Setenv("TLS_CERT_FILE", file)
			if code := healthcheck(); code != 0 {
				t.Fatalf("healthcheck rejected locally trusted certificate: %d", code)
			}
			// Self-signed mode finds the certificate beside METADATA_DB.
			metaDir := t.TempDir()
			if err := os.Mkdir(filepath.Join(metaDir, "tls"), 0700); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(metaDir, "tls", "cert.pem"), data, 0600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("METADATA_DB", filepath.Join(metaDir, "metadata.db"))
			t.Setenv("TLS_MODE", "self-signed")
			if code := healthcheck(); code != 0 {
				t.Fatalf("self-signed healthcheck failed: %d", code)
			}
			// A certificate for the same name from another key is not trusted.
			_, other := healthCertificate(t, name, false)
			t.Setenv("TLS_MODE", "file")
			t.Setenv("TLS_CERT_FILE", other)
			if code := healthcheck(); code == 0 {
				t.Fatal("healthcheck accepted a different server certificate")
			}
		})
	}
}

func TestHealthcheckRejectsExpiredCertificate(t *testing.T) {
	cert, file := healthCertificate(t, "backup.example.com", true)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	server.TLS = &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
	server.StartTLS()
	defer server.Close()
	t.Setenv("LISTEN_ADDR", server.Listener.Addr().String())
	t.Setenv("TLS_MODE", "file")
	t.Setenv("TLS_CERT_FILE", file)
	if code := healthcheck(); code == 0 {
		t.Fatal("healthcheck accepted an expired certificate")
	}
}
