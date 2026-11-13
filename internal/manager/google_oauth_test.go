package manager

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

type oauthTestTransport func(*http.Request) (*http.Response, error)

func (f oauthTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type oauthTestStart struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

func oauthTestRequest(m *Manager, method, path, body string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://localhost:8088"+path, strings.NewReader(body))
	r.Header.Set("X-Requested-With", "DBBackupManager")
	for _, c := range cookies {
		r.AddCookie(c)
	}
	w := httptest.NewRecorder()
	m.Handler().ServeHTTP(w, r)
	return w
}

func oauthTestBegin(t *testing.T, m *Manager, admin *http.Cookie, body string) (oauthTestStart, *http.Cookie) {
	t.Helper()
	w := oauthTestRequest(m, "POST", "/api/destinations/google/oauth/start", body, admin)
	if w.Code != 200 {
		t.Fatalf("start: %d %s", w.Code, w.Body)
	}
	var flow oauthTestStart
	if err := json.Unmarshal(w.Body.Bytes(), &flow); err != nil {
		t.Fatal(err)
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode || cookies[0].Path != googleOAuthCallbackPath {
		t.Fatal("callback cookie not bound safely")
	}
	return flow, cookies[0]
}

func oauthTestConfigure(m *Manager) {
	m.cfg.GoogleOAuthClientID = "test.apps.googleusercontent.com"
	m.cfg.GoogleOAuthClientSecret = "test-client-secret"
	m.cfg.GoogleOAuthRedirectURL = "http://localhost:8088" + googleOAuthCallbackPath
}

func TestGoogleOAuthEndToEnd(t *testing.T) {
	m := testManager(t)
	oauthTestConfigure(m)
	admin := loginCookie(t, m, "admin", "test-admin-password")
	other := loginCookie(t, m, "admin", "test-admin-password")
	viewer := loginCookie(t, m, "viewer", "test-viewer-password")
	for _, path := range []string{"/api/destinations/google/oauth", "/api/destinations/google/oauth/status?id=unknown"} {
		if w := oauthTestRequest(m, "GET", path, "", viewer); w.Code != 403 {
			t.Fatalf("viewer allowed: %d", w.Code)
		}
		if w := oauthTestRequest(m, "GET", path, ""); w.Code != 401 {
			t.Fatal("anonymous OAuth access")
		}
	}
	flow, cookie := oauthTestBegin(t, m, admin, `{}`)
	u, _ := url.Parse(flow.URL)
	q := u.Query()
	if u.Host != "accounts.google.com" || q.Get("access_type") != "offline" || q.Get("prompt") != "consent" || q.Get("scope") != googleDriveScope || q.Get("redirect_uri") != m.cfg.GoogleOAuthRedirectURL || q.Get("code_challenge_method") != "S256" {
		t.Fatalf("bad authorization request: %v", q)
	}
	if w := oauthTestRequest(m, "GET", "/api/destinations/google/oauth/status?id="+flow.ID, "", other); w.Code != 400 {
		t.Fatal("flow accessible from another session")
	}
	callback := googleOAuthCallbackPath + "?state=" + q.Get("state") + "&code=code-from-google"
	if w := oauthTestRequest(m, "GET", callback, ""); w.Code != 400 {
		t.Fatal("callback without binding cookie accepted")
	}
	if w := oauthTestRequest(m, "GET", googleOAuthCallbackPath+"?state=invalid&code=anything", "", cookie); w.Code != 400 {
		t.Fatal("invalid state accepted")
	}
	transport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = transport })
	exchanges := 0
	http.DefaultTransport = oauthTestTransport(func(r *http.Request) (*http.Response, error) {
		exchanges++
		if r.URL.String() != "https://oauth2.googleapis.com/token" || r.Method != "POST" {
			t.Fatal("unexpected token endpoint")
		}
		r.ParseForm()
		challenge := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
		if r.Form.Get("code") != "code-from-google" || r.Form.Get("client_secret") != m.cfg.GoogleOAuthClientSecret || r.Form.Get("redirect_uri") != m.cfg.GoogleOAuthRedirectURL || base64.RawURLEncoding.EncodeToString(challenge[:]) != q.Get("code_challenge") {
			t.Fatal("bad token exchange / PKCE")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"access_token":"private-access-token","refresh_token":"private-refresh-token","expires_in":3600,"token_type":"Bearer","scope":"` + q.Get("scope") + `"}`)), Header: make(http.Header)}, nil
	})
	// Only the Lax binding cookie is sent, matching a cross-site Google redirect.
	w := oauthTestRequest(m, "GET", callback, "", cookie)
	if w.Code != 200 {
		t.Fatalf("callback: %d %s", w.Code, w.Body)
	}
	if w := oauthTestRequest(m, "GET", callback, "", cookie); w.Code != 400 || exchanges != 1 {
		t.Fatal("replayed callback exchanged again")
	}
	status := oauthTestRequest(m, "GET", "/api/destinations/google/oauth/status?id="+flow.ID, "", admin)
	if !strings.Contains(status.Body.String(), `"ready":true`) {
		t.Fatal(status.Body)
	}
	if strings.Contains(status.Body.String(), "private-") || strings.Contains(w.Body.String(), "private-") || strings.Contains(m.googleOAuth[flow.ID].Config, "private-") {
		t.Fatal("token exposed")
	}
	// A bad save must leave the completed authorization available for correction.
	bad := oauthTestRequest(m, "POST", "/api/destinations", `{"google_oauth_id":"`+flow.ID+`"}`, admin)
	if bad.Code != 400 || m.googleOAuth[flow.ID] == nil {
		t.Fatal("validation failure consumed flow")
	}
	body := `{"name":"OAuth Drive","google_oauth_id":"` + flow.ID + `","remote_path":"db-backups","encryption":"passphrase","passphrase":"strong-backup-passphrase","retention_days":30}`
	if w := oauthTestRequest(m, "POST", "/api/destinations", body, other); w.Code != 400 {
		t.Fatal("another session saved flow")
	}
	saved := oauthTestRequest(m, "POST", "/api/destinations", body, admin)
	if saved.Code != 200 {
		t.Fatalf("save: %d %s", saved.Code, saved.Body)
	}
	if strings.Contains(saved.Body.String(), "private-") || strings.Contains(saved.Body.String(), flow.ID) {
		t.Fatal("save returned secrets or temporary handle")
	}
	var d Destination
	json.Unmarshal(saved.Body.Bytes(), &d)
	stored, err := m.destination(d.ID)
	if err != nil || stored.Type != "drive" || !strings.Contains(stored.Config, "private-refresh-token") || !strings.Contains(stored.Config, "client_secret = test-client-secret") {
		t.Fatalf("rclone config not persisted: %v", err)
	}
	var sealed string
	m.db.QueryRow(`SELECT config FROM destinations WHERE id=?`, d.ID).Scan(&sealed)
	if strings.Contains(sealed, "private-") {
		t.Fatal("plaintext token in SQLite")
	}
	if w := oauthTestRequest(m, "POST", "/api/destinations", body, admin); w.Code != 400 {
		t.Fatal("consumed flow reused")
	}
	// Editing other settings without reconnecting retains existing credentials.
	if w := oauthTestRequest(m, "PUT", "/api/destinations/"+d.ID, `{"name":"Renamed Drive","encryption":"passphrase","has_passphrase":true,"retention_days":15}`, admin); w.Code != 200 {
		t.Fatalf("keep existing config: %d %s", w.Code, w.Body)
	}
	// Reconnect an existing destination using the same OAuth UI, including full scope.
	m.db.Exec(`UPDATE destinations SET config=? WHERE id=?`, m.sealPassword(stored.Config+"root_folder_id = original-folder\nteam_drive = original-shared-drive\n"), d.ID)
	second, binding := oauthTestBegin(t, m, admin, `{"scope":"drive"}`)
	secondURL, _ := url.Parse(second.URL)
	if secondURL.Query().Get("scope") != "https://www.googleapis.com/auth/drive" {
		t.Fatal("full Drive not requested")
	}
	q = secondURL.Query()
	fullCallback := oauthTestRequest(m, "GET", googleOAuthCallbackPath+"?state="+q.Get("state")+"&code=code-from-google", "", binding)
	if fullCallback.Code != 200 {
		t.Fatalf("full-scope callback: %d %s", fullCallback.Code, fullCallback.Body)
	}
	reconnected := oauthTestRequest(m, "PUT", "/api/destinations/"+d.ID, `{"name":"Reconnected","google_oauth_id":"`+second.ID+`","encryption":"passphrase","has_passphrase":true}`, admin)
	if reconnected.Code != 200 {
		t.Fatalf("reconnect: %d %s", reconnected.Code, reconnected.Body)
	}
	updated, err := m.destination(d.ID)
	if err != nil || !strings.Contains(updated.Config, "scope = drive\n") {
		t.Fatal("full-scope reconnect not persisted")
	}
	if !strings.Contains(updated.Config, "root_folder_id = original-folder") || !strings.Contains(updated.Config, "team_drive = original-shared-drive") {
		t.Fatal("reconnect discarded original Drive folder settings")
	}
}

func TestGoogleOAuthFailures(t *testing.T) {
	m := testManager(t)
	admin := loginCookie(t, m, "admin", "test-admin-password")
	if w := oauthTestRequest(m, "POST", "/api/destinations/google/oauth/start", `{}`, admin); w.Code != 400 {
		t.Fatal("unconfigured OAuth started")
	}
	oauthTestConfigure(m)
	for _, redirect := range []string{"http://192.168.1.3:8088" + googleOAuthCallbackPath, "https://192.168.1.3" + googleOAuthCallbackPath, "http://backup.example.com" + googleOAuthCallbackPath, "https://user:pass@backup.example.com" + googleOAuthCallbackPath, "http://localhost:8088/wrong", "http://localhost:8088" + googleOAuthCallbackPath + "?query=yes"} {
		m.cfg.GoogleOAuthRedirectURL = redirect
		if m.googleOAuthReady() {
			t.Errorf("invalid callback accepted: %s", redirect)
		}
	}
	oauthTestConfigure(m)
	if w := request(m, "POST", "/api/destinations/google/oauth/start", `{}`, admin); w.Code != 400 {
		t.Fatal("mismatched browser host accepted")
	}
	if w := oauthTestRequest(m, "POST", "/api/destinations/google/oauth/start", `{"scope":"other"}`, admin); w.Code != 400 {
		t.Fatal("invalid scope accepted")
	}
	transport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = transport })
	for _, tc := range []struct {
		name, query, response string
		logout, expired       bool
	}{
		{name: "denied", query: "error=access_denied"},
		{name: "missing-code"},
		{name: "no-refresh", query: "code=x", response: `{"access_token":"secret","token_type":"Bearer","expires_in":3600}`},
		{name: "missing-scope", query: "code=x", response: `{"access_token":"secret","refresh_token":"refresh","token_type":"Bearer","expires_in":3600,"scope":"openid"}`},
		{name: "malformed", query: "code=x", response: `invalid-json`},
		{name: "expired", query: "code=x", expired: true},
		{name: "logout", query: "code=x", logout: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			flow, cookie := oauthTestBegin(t, m, admin, `{}`)
			u, _ := url.Parse(flow.URL)
			if tc.expired {
				m.googleOAuth[flow.ID].Expires = time.Now().Add(-time.Second)
			}
			if tc.logout {
				m.db.Exec(`DELETE FROM sessions WHERE token=?`, hashToken(admin.Value))
			}
			calls := 0
			http.DefaultTransport = oauthTestTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(tc.response))}, nil
			})
			w := oauthTestRequest(m, "GET", googleOAuthCallbackPath+"?state="+u.Query().Get("state")+"&"+tc.query, "", cookie)
			if w.Code != 400 {
				t.Fatalf("invalid callback: %d %s", w.Code, w.Body)
			}
			if tc.response == "" && calls != 0 {
				t.Fatal("invalid flow contacted token endpoint")
			}
			if f := m.googleOAuth[flow.ID]; f != nil && f.Config != "" {
				t.Fatal("invalid response saved config")
			}
		})
	}
}
