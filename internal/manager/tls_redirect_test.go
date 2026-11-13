package manager

import (
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"testing"
)

func TestSplitTLSServesHTTPSAndRedirectsHTTP(t *testing.T) {
	cfg, err := TLSConfig(Config{TLSMode: TLSSelfSigned, MetadataDB: filepath.Join(t.TempDir(), "metadata.db")})
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	tlsLn, plainLn := SplitTLS(ln)
	app := &http.Server{TLSConfig: cfg, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "secure") })}
	redirect := &http.Server{Handler: RedirectToHTTPS()}
	go app.ServeTLS(tlsLn, "", "")
	go redirect.Serve(plainLn)
	defer app.Close()
	defer redirect.Close()
	addr := ln.Addr().String()

	client := &http.Client{
		Transport:     &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := client.Get("http://" + addr + "/login?next=%2F")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusTemporaryRedirect || resp.Header.Get("Location") != "https://"+addr+"/login?next=%2F" {
		t.Fatalf("redirect: %d %q", resp.StatusCode, resp.Header.Get("Location"))
	}
	resp, err = client.Get("https://" + addr + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != "secure" {
		t.Fatalf("https body %q", body)
	}
}
