package manager

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const googleOAuthCallbackPath = "/api/destinations/google/callback"
const googleDriveScope = "https://www.googleapis.com/auth/drive.file"

// Flows are short-lived, bounded, and tied to the initiating login session.
// Only a sealed rclone config is retained after the code exchange; tokens
// and client credentials are never returned by the API.
type googleOAuthFlow struct {
	State, Session, Verifier, Config, Error, Scope string
	Expires                                        time.Time
	Claimed                                        bool
}

func (m *Manager) googleOAuthReady() bool {
	c := m.cfg
	if c.GoogleOAuthClientID == "" || c.GoogleOAuthClientSecret == "" || len(c.GoogleOAuthClientID) > 1024 || len(c.GoogleOAuthClientSecret) > 1024 || strings.ContainsAny(c.GoogleOAuthClientID+c.GoogleOAuthClientSecret, "\r\n") {
		return false
	}
	u, err := url.Parse(c.GoogleOAuthRedirectURL)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != googleOAuthCallbackPath {
		return false
	}
	host := u.Hostname()
	loopback := host == "localhost" || host == "127.0.0.1" || host == "::1"
	return (u.Scheme == "https" && (loopback || (net.ParseIP(host) == nil && strings.Contains(host, ".")))) || (u.Scheme == "http" && loopback)
}

func oauthSession(r *http.Request) string {
	c, err := r.Cookie("dbbm_session")
	if err != nil {
		return ""
	}
	return hashToken(c.Value)
}

func (m *Manager) pruneGoogleOAuth() {
	for id, flow := range m.googleOAuth {
		if time.Now().After(flow.Expires) {
			delete(m.googleOAuth, id)
		}
	}
}

func (m *Manager) googleOAuthAPI(w http.ResponseWriter, r *http.Request, path string) {
	// Keep provider credentials and pending connections admin-only, even if
	// destination read permissions are broadened in future.
	if !principalOf(r).Admin() {
		fail(w, 403, "Admin role required")
		return
	}
	switch {
	case r.Method == "GET" && path == "google/oauth":
		respond(w, 200, map[string]any{"available": m.googleOAuthReady(), "redirect_url": m.cfg.GoogleOAuthRedirectURL})
	case r.Method == "POST" && path == "google/oauth/start":
		if !m.googleOAuthReady() {
			fail(w, 400, "Cần cấu hình GOOGLE_OAUTH_CLIENT_ID, GOOGLE_OAUTH_CLIENT_SECRET và GOOGLE_OAUTH_REDIRECT_URL trên máy chủ trước.")
			return
		}
		var body struct {
			Scope string `json:"scope"`
		}
		if !decode(w, r, &body) {
			return
		}
		if body.Scope == "" {
			body.Scope = "drive.file"
		}
		if body.Scope != "drive.file" && body.Scope != "drive" {
			fail(w, 400, "Quyền truy cập Google Drive không hợp lệ")
			return
		}
		scope := "https://www.googleapis.com/auth/" + body.Scope
		redirect, _ := url.Parse(m.cfg.GoogleOAuthRedirectURL)
		if redirect.Host != r.Host {
			fail(w, 400, "Mở ứng dụng tại "+redirect.Scheme+"://"+redirect.Host+" trước khi kết nối Google Drive (qua SSH tunnel).")
			return
		}
		id, state, verifier := token(), token()+token(), token()+token()
		m.googleOAuthMu.Lock()
		m.pruneGoogleOAuth()
		if len(m.googleOAuth) >= 128 {
			m.googleOAuthMu.Unlock()
			fail(w, 429, "Có quá nhiều kết nối OAuth đang chờ. Hãy thử lại sau.")
			return
		}
		if m.googleOAuth == nil {
			m.googleOAuth = make(map[string]*googleOAuthFlow)
		}
		m.googleOAuth[id] = &googleOAuthFlow{State: state, Session: oauthSession(r), Verifier: verifier, Scope: scope, Expires: time.Now().Add(10 * time.Minute)}
		m.googleOAuthMu.Unlock()
		// Separate cookie permits Google's top-level redirect without weakening
		// the application's SameSite=Strict login cookie. Name per flow supports tabs.
		http.SetCookie(w, &http.Cookie{Name: "dbbm_google_" + id, Value: state, Path: googleOAuthCallbackPath, HttpOnly: true, Secure: redirect.Scheme == "https", SameSite: http.SameSiteLaxMode, MaxAge: 600})
		digest := sha256.Sum256([]byte(verifier))
		q := url.Values{"client_id": {m.cfg.GoogleOAuthClientID}, "redirect_uri": {m.cfg.GoogleOAuthRedirectURL}, "response_type": {"code"}, "scope": {scope}, "access_type": {"offline"}, "prompt": {"consent"}, "state": {state}, "code_challenge": {base64.RawURLEncoding.EncodeToString(digest[:])}, "code_challenge_method": {"S256"}}
		respond(w, 200, map[string]string{"id": id, "url": "https://accounts.google.com/o/oauth2/v2/auth?" + q.Encode()})
	case r.Method == "GET" && path == "google/oauth/status":
		m.googleOAuthMu.Lock()
		defer m.googleOAuthMu.Unlock()
		m.pruneGoogleOAuth()
		flow := m.googleOAuth[r.URL.Query().Get("id")]
		if flow == nil || flow.Session != oauthSession(r) {
			fail(w, 400, "Kết nối Google OAuth đã hết hạn hoặc không hợp lệ. Hãy kết nối lại.")
			return
		}
		respond(w, 200, map[string]any{"ready": flow.Config != "", "error": flow.Error})
	default:
		fail(w, 404, "Endpoint not found")
	}
}

func (m *Manager) googleOAuthConfig(r *http.Request, id string) (string, error) {
	m.googleOAuthMu.Lock()
	defer m.googleOAuthMu.Unlock()
	m.pruneGoogleOAuth()
	f := m.googleOAuth[id]
	if f == nil || f.Session != oauthSession(r) || f.Config == "" {
		return "", errors.New("invalid OAuth connection")
	}
	return m.openSealed(f.Config)
}

func (m *Manager) googleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	m.googleOAuthMu.Lock()
	m.pruneGoogleOAuth()
	var id string
	var flow *googleOAuthFlow
	state := r.URL.Query().Get("state")
	for key, f := range m.googleOAuth {
		if state != "" && f.State == state && !f.Claimed {
			id, flow = key, f
			break
		}
	}
	if flow == nil {
		m.googleOAuthMu.Unlock()
		http.Error(w, "Phiên cấp quyền OAuth không hợp lệ hoặc đã hết hạn. Hãy kết nối lại từ ứng dụng.", 400)
		return
	}
	cookie, err := r.Cookie("dbbm_google_" + id)
	if err != nil || cookie.Value != state {
		m.googleOAuthMu.Unlock()
		http.Error(w, "Không xác minh được trình duyệt OAuth. Hãy kết nối lại từ ứng dụng.", 400)
		return
	}
	flow.Claimed = true
	snapshot := *flow
	m.googleOAuthMu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "dbbm_google_" + id, Value: "", Path: googleOAuthCallbackPath, HttpOnly: true, Secure: strings.HasPrefix(m.cfg.GoogleOAuthRedirectURL, "https://"), SameSite: http.SameSiteLaxMode, MaxAge: -1})
	// The callback has no login cookie. Check the initiating session directly,
	// including logout, expiry, account removal and changes in admin privileges.
	var userID string
	err = m.db.QueryRowContext(r.Context(), `SELECT user_id FROM sessions WHERE token=? AND expires_at>?`, snapshot.Session, time.Now().Unix()).Scan(&userID)
	if err == nil {
		p, e := m.loadPrincipal(r.Context(), userID)
		if e != nil || !p.Admin() {
			err = errors.New("admin session revoked")
		}
	}
	var config string
	message := ""
	if err != nil {
		message = "Phiên đăng nhập đã hết hạn. Hãy đăng nhập và kết nối lại."
	} else if r.URL.Query().Get("error") != "" {
		message = "Chưa được Google cấp quyền. Hãy kết nối lại để cấp quyền truy cập."
	} else if r.URL.Query().Get("code") == "" {
		message = "Google không trả về mã cấp quyền. Hãy kết nối lại."
	} else {
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		config, err = m.exchangeGoogleCode(ctx, r.URL.Query().Get("code"), snapshot.Verifier, snapshot.Scope)
		if err != nil {
			message = "Không hoàn tất được cấp quyền Google. Kiểm tra cấu hình OAuth Client và thử lại."
		}
	}
	m.googleOAuthMu.Lock()
	if current := m.googleOAuth[id]; current == flow {
		current.Error = message
		current.Verifier = ""
		if message == "" {
			current.Config = m.sealPassword(config)
			current.Expires = time.Now().Add(30 * time.Minute)
		}
	}
	m.googleOAuthMu.Unlock()
	if message != "" {
		http.Error(w, message, 400)
		return
	}
	// No inline script or third-party assets: do not expose the callback code.
	fmt.Fprintln(w, "Google Drive connected. You can close this window and return to the app. Save the cloud destination to keep this connection. / Đã kết nối Google Drive. Đóng cửa sổ này, quay lại ứng dụng và lưu đích cloud.")
}

func (m *Manager) exchangeGoogleCode(ctx context.Context, code, verifier, scope string) (string, error) {
	values := url.Values{"code": {code}, "client_id": {m.cfg.GoogleOAuthClientID}, "client_secret": {m.cfg.GoogleOAuthClientSecret}, "redirect_uri": {m.cfg.GoogleOAuthRedirectURL}, "grant_type": {"authorization_code"}, "code_verifier": {verifier}}
	req, err := http.NewRequestWithContext(ctx, "POST", "https://oauth2.googleapis.com/token", strings.NewReader(values.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// Never follow redirects carrying the code and client secret.
	client := &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Do(req)
	if err != nil {
		return "", errors.New("Google token request failed")
	}
	defer res.Body.Close()
	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
		Scope        string `json:"scope"`
	}
	if res.StatusCode != http.StatusOK {
		return "", errors.New("Google rejected token exchange")
	}
	if err = json.NewDecoder(io.LimitReader(res.Body, 65536)).Decode(&result); err != nil {
		return "", errors.New("invalid Google token response")
	}
	if result.AccessToken == "" || result.RefreshToken == "" || result.ExpiresIn <= 0 || result.ExpiresIn > 86400 || !strings.EqualFold(result.TokenType, "Bearer") {
		return "", errors.New("Google did not issue an offline token")
	}
	if result.Scope != "" {
		allowed := false
		for _, granted := range strings.Fields(result.Scope) {
			if granted == scope {
				allowed = true
			}
		}
		if !allowed {
			return "", errors.New("Google Drive permission not granted")
		}
	}
	tokenJSON, _ := json.Marshal(map[string]any{"access_token": result.AccessToken, "refresh_token": result.RefreshToken, "token_type": result.TokenType, "expiry": time.Now().Add(time.Duration(result.ExpiresIn) * time.Second).UTC().Format(time.RFC3339Nano)})
	opts := []rcloneOption{{"type", "drive"}, {"client_id", m.cfg.GoogleOAuthClientID}, {"client_secret", m.cfg.GoogleOAuthClientSecret}, {"scope", strings.TrimPrefix(scope, "https://www.googleapis.com/auth/")}, {"token", string(tokenJSON)}}
	config := renderRcloneConfig(opts)
	if _, _, err := parseRcloneConfig(config, false); err != nil {
		return "", errors.New("invalid Google configuration")
	}
	return config, nil
}
