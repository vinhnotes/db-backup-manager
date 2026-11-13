package manager

import (
	"io"
	"log"
	"strings"
	"testing"
)

func TestTimezoneSetting(t *testing.T) {
	m := testManager(t)
	admin := loginCookie(t, m, "admin", "test-admin-password")
	viewer := loginCookie(t, m, "viewer", "test-viewer-password")
	if w := request(m, "PUT", "/api/settings/timezone", `{"timezone":"UTC"}`, viewer); w.Code != 403 {
		t.Fatalf("viewer changed timezone: %d", w.Code)
	}
	for _, bad := range []string{"Mars/Base", "Local", "../etc"} {
		if w := request(m, "PUT", "/api/settings/timezone", `{"timezone":"`+bad+`"}`, admin); w.Code != 400 {
			t.Fatalf("%s accepted: %d", bad, w.Code)
		}
	}
	if w := request(m, "PUT", "/api/settings/timezone", `{"timezone":"Europe/Berlin"}`, admin); w.Code != 200 {
		t.Fatalf("set: %d %s", w.Code, w.Body)
	}
	if w := request(m, "GET", "/api/me", "", admin); !strings.Contains(w.Body.String(), `"timezone":"Europe/Berlin"`) {
		t.Fatal(w.Body)
	}
	// Survives restart.
	m2, err := New(m.cfg, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	if m2.timezone() != "Europe/Berlin" {
		t.Fatal(m2.timezone())
	}
	m2.Close()
	admin = loginCookie(t, m, "admin", "test-admin-password")
	// Empty resets to TZ from env.
	if w := request(m, "PUT", "/api/settings/timezone", `{"timezone":""}`, admin); w.Code != 200 || m.timezone() != "Asia/Ho_Chi_Minh" {
		t.Fatalf("reset: %d %s", w.Code, m.timezone())
	}
}
