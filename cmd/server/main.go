package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"dbbackup/internal/manager"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
	_ "time/tzdata"
)

const usage = `Usage:
  db-backup-manager                        start the server
  db-backup-manager reset-admin-password   make ADMIN_PASSWORD from .env the admin password again
  db-backup-manager healthcheck            exit 0 when the running server answers /api/health
`

func main() {
	if len(os.Args) > 1 {
		os.Exit(command(os.Args[1:]))
	}
	logger := log.New(os.Stdout, "dbbackup ", log.LstdFlags|log.LUTC)
	cfg, err := manager.LoadConfig()
	if err != nil {
		logger.Fatal(err)
	}
	m, err := manager.New(cfg, logger)
	if err != nil {
		logger.Fatal(err)
	}
	tlsConfig, err := manager.TLSConfig(cfg)
	if err != nil {
		logger.Fatal(err)
	}
	server := &http.Server{Addr: cfg.Addr, Handler: m.Handler(), TLSConfig: tlsConfig, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	ln, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		logger.Fatal(err)
	}
	// With TLS, plain http:// requests on the same port are redirected to https://.
	redirect := &http.Server{Handler: manager.RedirectToHTTPS(), ReadHeaderTimeout: 10 * time.Second, MaxHeaderBytes: 16384, ErrorLog: log.New(io.Discard, "", 0)}
	serveErr := make(chan error, 2)
	serve := func(f func() error) {
		if err := f(); err != nil && err != http.ErrServerClosed {
			serveErr <- err
		}
	}
	if tlsConfig != nil {
		logger.Printf("listening on https://%s (TLS_MODE=%s, http:// redirects to https://)", cfg.Addr, cfg.TLSMode)
		tlsLn, plainLn := manager.SplitTLS(ln)
		go serve(func() error { return server.ServeTLS(tlsLn, "", "") })
		go serve(func() error { return redirect.Serve(plainLn) })
	} else {
		logger.Printf("listening on http://%s", cfg.Addr)
		go serve(func() error { return server.Serve(ln) })
	}
	var listenErr error
	select {
	case <-stop:
	case listenErr = <-serveErr:
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	_ = redirect.Shutdown(ctx)
	_ = server.Shutdown(ctx)
	m.Close()
	if listenErr != nil {
		logger.Fatal(listenErr)
	}
}

func command(args []string) int {
	if len(args) == 1 && args[0] == "healthcheck" {
		return healthcheck()
	}
	if len(args) != 1 || args[0] != "reset-admin-password" {
		fmt.Fprint(os.Stderr, usage)
		return 2
	}
	if err := manager.ResetAdminPassword(manager.MetadataDBPath()); err != nil {
		fmt.Fprintln(os.Stderr, "reset failed:", err)
		return 1
	}
	fmt.Println("Admin password reset: sign in with ADMIN_PASSWORD from .env.")
	fmt.Println("Existing admin sessions were signed out. No restart needed.")
	return 0
}

// healthcheck calls /api/health on the local server over HTTP or HTTPS,
// matching TLS_MODE and trusting the locally configured server certificate.
func healthcheck() int {
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "invalid LISTEN_ADDR:", err)
		return 1
	}
	if ip := net.ParseIP(host); host == "" || (ip != nil && ip.IsUnspecified()) {
		host = "127.0.0.1"
	}
	scheme := "http"
	mode, certFile, _, _ := manager.TLSSettings()
	var tlsConfig *tls.Config
	if mode != manager.TLSOff {
		scheme = "https"
		if mode == manager.TLSSelfSigned {
			certFile = filepath.Join(filepath.Dir(manager.MetadataDBPath()), "tls", "cert.pem")
		} else if mode != manager.TLSFile {
			fmt.Fprintln(os.Stderr, "invalid TLS_MODE")
			return 1
		}
		tlsConfig, err = healthcheckTLSConfig(certFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, "healthcheck TLS:", err)
			return 1
		}
	}
	transport := &http.Transport{TLSClientConfig: tlsConfig}
	defer transport.CloseIdleConnections()
	client := &http.Client{Timeout: 5 * time.Second, Transport: transport, CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Get(scheme + "://" + net.JoinHostPort(host, port) + "/api/health") // #nosec G704 -- URL is built from the operator's LISTEN_ADDR, not request input.
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "health:", resp.Status)
		return 1
	}
	return 0
}

// Trust only the first certificate from the server's local PEM file. Use its
// SAN for verification because LISTEN_ADDR can be a bind IP while a file-mode
// certificate is issued to a DNS name. Normal chain and expiry checks still run.
func healthcheckTLSConfig(certFile string) (*tls.Config, error) {
	pemBytes, err := os.ReadFile(certFile)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(pemBytes)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("invalid server certificate PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}
	var serverName string
	if len(cert.DNSNames) > 0 {
		serverName = cert.DNSNames[0]
	} else if len(cert.IPAddresses) > 0 {
		serverName = cert.IPAddresses[0].String()
	} else {
		return nil, fmt.Errorf("server certificate has no DNS or IP SAN")
	}
	// A wildcard SAN needs a concrete hostname for VerifyHostname.
	if len(serverName) > 2 && serverName[:2] == "*." {
		serverName = "healthcheck." + serverName[2:]
	}
	roots := x509.NewCertPool()
	roots.AddCert(cert)
	return &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: serverName}, nil
}
