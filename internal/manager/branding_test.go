package manager

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBranding(t *testing.T) {
	m := testManager(t)
	var b struct {
		Name string `json:"name"`
		Logo bool   `json:"logo"`
	}
	// Readable without a session (sign-in page).
	w := request(m, "GET", "/api/branding", "", nil)
	json.Unmarshal(w.Body.Bytes(), &b)
	if w.Code != 200 || b.Name != "DB Backup Manager" || b.Logo {
		t.Fatalf("default: %d %s", w.Code, w.Body)
	}
	// Without a logo the favicon is the default icon.
	if w = request(m, "GET", "/favicon.ico", "", nil); w.Code != 200 || w.Header().Get("Content-Type") != "image/svg+xml" || w.Body.String() != defaultFavicon {
		t.Fatalf("default favicon: %d %v", w.Code, w.Header())
	}
	admin := loginCookie(t, m, "admin", "test-admin-password")
	viewer := loginCookie(t, m, "viewer", "test-viewer-password")
	if w = request(m, "PUT", "/api/settings/branding", `{"name":"Hacked"}`, viewer); w.Code != 403 {
		t.Fatalf("viewer renamed: %d", w.Code)
	}
	if w = request(m, "PUT", "/api/settings/branding", `{"name":"`+strings.Repeat("x", 61)+`"}`, admin); w.Code != 400 {
		t.Fatalf("long name: %d", w.Code)
	}
	if w = request(m, "PUT", "/api/settings/branding", `{"name":"  Example Backup  "}`, admin); w.Code != 200 || !strings.Contains(w.Body.String(), `"name":"Example Backup"`) {
		t.Fatalf("rename: %d %s", w.Code, w.Body)
	}
	if subject, _ := m.message("en", Event{Kind: eventTest}); subject != "🔔 Test notification from Example Backup" {
		t.Fatal(subject)
	}
	if w = request(m, "PUT", "/api/settings/branding/logo", "plain text", admin); w.Code != 400 {
		t.Fatalf("text logo: %d", w.Code)
	}
	png := "\x89PNG\r\n\x1a\n" + strings.Repeat("\x00", 32)
	if w = request(m, "PUT", "/api/settings/branding/logo", png, admin); w.Code != 200 {
		t.Fatalf("logo: %d %s", w.Code, w.Body)
	}
	w = httptest.NewRecorder()
	m.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/api/branding/logo", nil))
	if w.Code != 200 || w.Header().Get("Content-Type") != "image/png" || w.Body.String() != png {
		t.Fatalf("serve logo: %d %s", w.Code, w.Header())
	}
	// The favicon follows the logo, and revalidates by version.
	w = request(m, "GET", "/api/branding/favicon", "", nil)
	if w.Code != 200 || w.Header().Get("Content-Type") != "image/png" || w.Body.String() != png {
		t.Fatalf("logo favicon: %d %v", w.Code, w.Header())
	}
	r := httptest.NewRequest("GET", "/favicon.ico", nil)
	r.Header.Set("If-None-Match", w.Header().Get("ETag"))
	w = httptest.NewRecorder()
	m.Handler().ServeHTTP(w, r)
	if w.Code != 304 {
		t.Fatalf("favicon revalidation: %d", w.Code)
	}
	svg := `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`
	request(m, "PUT", "/api/settings/branding/logo", svg, admin)
	w = request(m, "GET", "/api/branding/logo", "", nil)
	if w.Header().Get("Content-Type") != "image/svg+xml" || !strings.Contains(w.Header().Get("Content-Security-Policy"), "sandbox") {
		t.Fatalf("svg headers: %v", w.Header())
	}
	if w = request(m, "DELETE", "/api/settings/branding/logo", "", admin); w.Code != 200 {
		t.Fatalf("remove: %d", w.Code)
	}
	if w = request(m, "GET", "/api/branding/logo", "", nil); w.Code != 404 {
		t.Fatalf("removed logo served: %d", w.Code)
	}
	if w = request(m, "GET", "/favicon.ico", "", nil); w.Body.String() != defaultFavicon {
		t.Fatal("favicon not reset to the default")
	}
	// Saving the default name or "" resets it.
	request(m, "PUT", "/api/settings/branding", `{"name":""}`, admin)
	if m.brandName() != "DB Backup Manager" {
		t.Fatal("reset failed")
	}
}
