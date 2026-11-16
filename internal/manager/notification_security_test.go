package manager

import (
	"context"
	"fmt"
	"io"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/http"
	"net/http/httptest"
	"net/mail"
	"net/textproto"
	"strings"
	"testing"
	"time"
)

func TestNotificationDoesNotFollowRedirects(t *testing.T) {
	for _, status := range []int{http.StatusFound, http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			unexpected := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Error("notification reached a redirected destination")
			}))
			defer unexpected.Close()
			source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, unexpected.URL, status)
			}))
			defer source.Close()
			t.Setenv("NOTIFY_ALLOWED_ORIGINS", source.URL)
			if err := postJSON(t.Context(), source.URL, map[string]string{"secret": "backup details"}, []byte("key")); err == nil {
				t.Fatal("redirect reported as successful delivery")
			}
		})
	}
	for _, target := range []string{"file:///etc/passwd", "http://user:password@localhost/", "/relative"} {
		if err := postJSON(context.Background(), target, nil, nil); err == nil {
			t.Errorf("accepted invalid target %q", target)
		}
	}
}

func TestNotificationOriginAllowlist(t *testing.T) {
	t.Setenv("NOTIFY_ALLOWED_ORIGINS", "https://hooks.example.com:8443")
	for _, target := range []string{
		"http://hooks.example.com:8443/hook", "https://hooks.example.com/hook",
		"https://hooks.example.com.evil.test:8443/hook", "http://127.0.0.1/hook",
		"http://169.254.169.254/latest/meta-data/", "http://[::1]/hook",
		"https://hooks.example.com:8443@evil.test/hook", "https://hooks.example.com:8443/hook#fragment",
	} {
		if _, err := notificationRequest(t.Context(), target, nil); err == nil {
			t.Errorf("unauthorized origin accepted: %q", target)
		}
	}
	target := "https://HOOKS.example.com:8443/escaped%2Fpath?q=x%26y"
	req, err := notificationRequest(t.Context(), target, []byte("body"))
	if err != nil {
		t.Fatal(err)
	}
	if req.URL.Scheme != "https" || req.URL.Host != "hooks.example.com:8443" || req.URL.RequestURI() != "/escaped%2Fpath?q=x%26y" {
		t.Fatalf("request did not retain trusted origin and input path: %s", req.URL)
	}
}

func TestMailHeadersRejectInjectionBeforeDial(t *testing.T) {
	for _, field := range []string{"from", "to", "subject"} {
		for _, control := range []string{"\r", "\n", "\r\n", "\x00"} {
			t.Run(field+"/"+strings.ReplaceAll(control, "\r", "CR"), func(t *testing.T) {
				cfg := map[string]string{"host": "invalid", "from": "sender@example.com", "to": "recipient@example.com"}
				subject := "Backup complete"
				if field == "subject" {
					subject += control + "Bcc: injected@example.com"
				} else {
					cfg[field] += control + "Bcc: injected@example.com"
				}
				if err := sendMail(t.Context(), cfg, subject, "body"); err == nil || err.Error() != "invalid email header" {
					t.Fatalf("expected header rejection before SMTP dial, got %v", err)
				}
			})
		}
	}
}

func TestMailMessageKeepsContentInBody(t *testing.T) {
	cfg := map[string]string{"from": "Backup <sender@example.com>", "to": "One <one@example.com>, two@example.com"}
	subject := "✅ Backup thành công"
	body := "Backup result\r\nBcc: attacker@example.com\n.\nContent-Type: text/html\n<b>text</b>"
	data, _, recipients, err := mailMessage(cfg, subject, body)
	if err != nil {
		t.Fatal(err)
	}
	message, err := mail.ReadMessage(strings.NewReader(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	decodedSubject, err := new(mime.WordDecoder).DecodeHeader(message.Header.Get("Subject"))
	if err != nil || decodedSubject != subject || len(recipients) != 2 || message.Header.Get("Bcc") != "" || message.Header.Get("Content-Type") != "text/plain; charset=UTF-8" {
		t.Fatalf("invalid message headers: %v %v", message.Header, err)
	}
	decodedBody, err := io.ReadAll(quotedprintable.NewReader(message.Body))
	if err != nil || strings.ReplaceAll(string(decodedBody), "\r\n", "\n") != strings.ReplaceAll(body, "\r\n", "\n") {
		t.Fatalf("body did not round trip: %q %v", decodedBody, err)
	}
	for _, field := range []string{"from", "to"} {
		bad := map[string]string{"from": cfg["from"], "to": cfg["to"]}
		bad[field] = "not an address"
		if _, _, _, err := mailMessage(bad, subject, body); err == nil {
			t.Fatalf("invalid %s accepted", field)
		}
	}
}

func TestSMTPInjectionCannotAddHeadersOrEndData(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	type result struct {
		data []byte
		err  error
	}
	done := make(chan result, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			done <- result{err: err}
			return
		}
		defer conn.Close()
		conn.SetDeadline(time.Now().Add(5 * time.Second))
		wire := textproto.NewConn(conn)
		serve := func() ([]byte, error) {
			if err := wire.PrintfLine("220 test SMTP"); err != nil {
				return nil, err
			}
			for _, expected := range []string{"EHLO ", "MAIL FROM:<sender@example.com>", "RCPT TO:<recipient@example.com>", "DATA"} {
				line, err := wire.ReadLine()
				if err != nil {
					return nil, err
				}
				if (expected == "EHLO " && !strings.HasPrefix(line, expected)) || (expected != "EHLO " && line != expected) {
					return nil, fmt.Errorf("unexpected SMTP command %q; want %q", line, expected)
				}
				if expected == "DATA" {
					err = wire.PrintfLine("354 send data")
				} else {
					err = wire.PrintfLine("250 OK")
				}
				if err != nil {
					return nil, err
				}
			}
			data, err := wire.ReadDotBytes()
			if err != nil {
				return nil, err
			}
			if err = wire.PrintfLine("250 queued"); err != nil {
				return nil, err
			}
			line, err := wire.ReadLine()
			if err != nil || line != "QUIT" {
				return nil, fmt.Errorf("unexpected command after DATA: %q (%v)", line, err)
			}
			return data, wire.PrintfLine("221 bye")
		}
		data, err := serve()
		done <- result{data, err}
	}()
	host, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	cfg := map[string]string{"host": host, "port": port, "security": "none", "from": "sender@example.com", "to": "recipient@example.com"}
	body := "result\r\n.\r\nBcc: attacker@example.com\r\nContent-Type: text/html\r\n\r\n<script>alert(1)</script>\r\nQUIT"
	if err := sendMail(t.Context(), cfg, "Backup result", body); err != nil {
		t.Fatal(err)
	}
	got := <-done
	if got.err != nil {
		t.Fatal(got.err)
	}
	message, err := mail.ReadMessage(strings.NewReader(string(got.data)))
	if err != nil {
		t.Fatal(err)
	}
	if message.Header.Get("Bcc") != "" || message.Header.Get("Content-Type") != "text/plain; charset=UTF-8" {
		t.Fatalf("injected headers: %v", message.Header)
	}
	decoded, err := io.ReadAll(quotedprintable.NewReader(message.Body))
	// SMTP DATA is line-oriented and appends a final newline when absent.
	if err != nil || strings.ReplaceAll(string(decoded), "\r\n", "\n") != strings.ReplaceAll(body, "\r\n", "\n")+"\n" {
		t.Fatalf("SMTP DATA changed or terminated early: %q (%v)", decoded, err)
	}
}
