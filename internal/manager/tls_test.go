package manager

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestSelfSignedCertificateIsKeptAndRenewedForNewHosts(t *testing.T) {
	dir := t.TempDir()
	c := Config{TLSMode: TLSSelfSigned, MetadataDB: filepath.Join(dir, "metadata.db"), TLSHosts: "backup.local, 10.0.0.5"}
	cfg, err := TLSConfig(c)
	if err != nil {
		t.Fatal(err)
	}
	leaf := cfg.Certificates[0].Leaf
	for _, h := range []string{"localhost", "127.0.0.1", "backup.local", "10.0.0.5"} {
		if err := leaf.VerifyHostname(h); err != nil {
			t.Errorf("certificate does not cover %s: %v", h, err)
		}
	}
	keyFile := filepath.Join(dir, "tls", "key.pem")
	if st, err := os.Stat(keyFile); err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("key file mode: %v %v", st, err)
	}
	certPEM, _ := os.ReadFile(filepath.Join(dir, "tls", "cert.pem"))

	// Restart with the same hosts: the certificate must not change.
	if _, err := TLSConfig(c); err != nil {
		t.Fatal(err)
	}
	if again, _ := os.ReadFile(filepath.Join(dir, "tls", "cert.pem")); !bytes.Equal(again, certPEM) {
		t.Fatal("certificate was regenerated without a reason")
	}

	// A new host in TLS_HOSTS regenerates it.
	c.TLSHosts = "backup.example.com"
	cfg, err = TLSConfig(c)
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Certificates[0].Leaf.VerifyHostname("backup.example.com"); err != nil {
		t.Fatal(err)
	}
}

func TestTLSOffAndModeDefaults(t *testing.T) {
	if cfg, err := TLSConfig(Config{TLSMode: TLSOff}); cfg != nil || err != nil {
		t.Fatalf("off: %v %v", cfg, err)
	}
	t.Setenv("TLS_MODE", "")
	t.Setenv("TLS_CERT_FILE", "/c.pem")
	t.Setenv("TLS_KEY_FILE", "/k.pem")
	if mode, _, _, _ := TLSSettings(); mode != TLSFile {
		t.Fatalf("mode with certificate files = %s", mode)
	}
	t.Setenv("ADMIN_PASSWORD", "correct-horse-battery")
	t.Setenv("TLS_CERT_FILE", "")
	t.Setenv("TLS_KEY_FILE", "")
	for mode, secure := range map[string]bool{"off": false, "self-signed": true} {
		t.Setenv("TLS_MODE", mode)
		c, err := LoadConfig()
		if err != nil || c.SecureCookie != secure {
			t.Fatalf("TLS_MODE=%s: secure=%v err=%v", mode, c.SecureCookie, err)
		}
	}
	t.Setenv("COOKIE_SECURE", "false")
	if c, _ := LoadConfig(); c.SecureCookie {
		t.Fatal("COOKIE_SECURE=false must win")
	}
}
