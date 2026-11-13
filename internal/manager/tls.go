package manager

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// TLS modes (TLS_MODE): off serves plain HTTP, self-signed generates and
// keeps a certificate next to the metadata DB, file uses TLS_CERT_FILE and
// TLS_KEY_FILE.
const (
	TLSOff        = "off"
	TLSSelfSigned = "self-signed"
	TLSFile       = "file"
)

const selfSignedValidity = 10 * 365 * 24 * time.Hour

// tlsHosts are the names and IPs the self-signed certificate is valid for:
// localhost, the container hostname and TLS_HOSTS (comma-separated).
func tlsHosts(extra string) []string {
	hosts := []string{"localhost", "127.0.0.1", "::1"}
	if h, err := os.Hostname(); err == nil && h != "" {
		hosts = append(hosts, h)
	}
	for _, h := range strings.Split(extra, ",") {
		if h = strings.TrimSpace(h); h != "" {
			hosts = append(hosts, h)
		}
	}
	slices.Sort(hosts)
	return slices.Compact(hosts)
}

// TLSConfig returns the server TLS settings, or nil when TLS is off.
func TLSConfig(c Config) (*tls.Config, error) {
	var certFile, keyFile string
	switch c.TLSMode {
	case TLSOff:
		return nil, nil
	case TLSFile:
		certFile, keyFile = c.TLSCertFile, c.TLSKeyFile
	case TLSSelfSigned:
		dir := filepath.Join(filepath.Dir(c.MetadataDB), "tls")
		certFile, keyFile = filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
		if err := ensureSelfSigned(certFile, keyFile, tlsHosts(c.TLSHosts)); err != nil {
			return nil, fmt.Errorf("self-signed certificate: %w", err)
		}
	default:
		return nil, fmt.Errorf("unknown TLS_MODE %q", c.TLSMode)
	}
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("load TLS certificate: %w", err)
	}
	return &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}}, nil
}

// ensureSelfSigned keeps the existing certificate so browsers that accepted
// it once stop warning, and replaces it only when it is missing, unreadable,
// expires within 30 days or does not cover every host.
func ensureSelfSigned(certFile, keyFile string, hosts []string) error {
	if cert, err := tls.LoadX509KeyPair(certFile, keyFile); err == nil && cert.Leaf != nil &&
		time.Now().Add(30*24*time.Hour).Before(cert.Leaf.NotAfter) && coversHosts(cert.Leaf, hosts) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(certFile), 0o700); err != nil {
		return err
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 127))
	if err != nil {
		return err
	}
	now := time.Now()
	tmpl := x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "DB Backup Manager", Organization: []string{"DB Backup Manager (self-signed)"}},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(selfSignedValidity),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		} else {
			tmpl.DNSNames = append(tmpl.DNSNames, h)
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		return err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}
	if err := writeFileAtomic(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		return err
	}
	return writeFileAtomic(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644)
}

func coversHosts(cert *x509.Certificate, hosts []string) bool {
	for _, h := range hosts {
		if cert.VerifyHostname(h) != nil {
			return false
		}
	}
	return true
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
