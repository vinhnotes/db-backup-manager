package manager

import (
	"context"
	"io"
	"mime"
	"mime/quotedprintable"
	"net/http"
	"net/http/httptest"
	"net/mail"
	"strings"
	"testing"
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
