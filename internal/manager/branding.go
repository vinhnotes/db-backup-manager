package manager

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

// Branding: administrators can replace the product name and logo shown in
// the web UI (also on the sign-in page, so reading them needs no session).

const (
	defaultBrandName = "DB Backup Manager"
	settingBrandName = "brand_name"
	settingBrandLogo = "brand_logo"      // base64 image
	settingLogoType  = "brand_logo_type" // its MIME type
	settingBrandVer  = "brand_version"   // changes on every update, for caching
	maxLogoBytes     = 256 * 1024
)

func (m *Manager) setting(key string) string {
	var v string
	if err := m.db.QueryRow(`SELECT value FROM settings WHERE key=?`, key).Scan(&v); err != nil && !errors.Is(err, sql.ErrNoRows) {
		m.logger.Printf("setting %s: %v", key, err)
	}
	return v
}

func (m *Manager) brandName() string {
	if v := m.setting(settingBrandName); v != "" {
		return v
	}
	return defaultBrandName
}

// branding serves GET /api/branding.
func (m *Manager) branding(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	respond(w, 200, map[string]any{"name": m.brandName(), "default_name": defaultBrandName, "logo": m.setting(settingBrandLogo) != "", "version": m.setting(settingBrandVer)})
}

// brandLogo serves GET /api/branding/logo.
func (m *Manager) brandLogo(w http.ResponseWriter, r *http.Request) {
	data, err := base64.StdEncoding.DecodeString(m.setting(settingBrandLogo))
	if err != nil || len(data) == 0 {
		http.NotFound(w, r)
		return
	}
	kind := m.setting(settingLogoType)
	w.Header().Set("Content-Type", kind)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	// An SVG opened directly must not run scripts or load anything.
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	w.Write(data)
}

// defaultFavicon is the sidebar's default logo (white database icon on the
// brand orange), used as favicon until a logo is uploaded.
const defaultFavicon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">` +
	`<rect width="32" height="32" rx="7" fill="#f6821f"/>` +
	`<g transform="translate(6 6) scale(.8333)" fill="none" stroke="#fff" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round">` +
	`<ellipse cx="12" cy="5" rx="9" ry="3"/><path d="M3 5V19A9 3 0 0 0 21 19V5"/><path d="M3 12A9 3 0 0 0 21 12"/></g></svg>`

// favicon serves GET /favicon.ico and /api/branding/favicon: the brand logo,
// or the default icon. Served from the server (not only set by the page
// script) so the icon shows at once and for clients that run no script.
func (m *Manager) favicon(w http.ResponseWriter, r *http.Request) {
	data, err := base64.StdEncoding.DecodeString(m.setting(settingBrandLogo))
	kind := m.setting(settingLogoType)
	if err != nil || len(data) == 0 || kind == "" {
		data, kind = []byte(defaultFavicon), "image/svg+xml"
	}
	w.Header().Set("Content-Type", kind)
	// Revalidate every time: the ETag changes whenever the logo does.
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("ETag", `"`+m.setting(settingBrandVer)+`"`)
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
}

// brandingAPI serves the admin-only /api/settings/branding endpoints.
func (m *Manager) brandingAPI(w http.ResponseWriter, r *http.Request) {
	if !principalOf(r).Admin() {
		fail(w, 403, "Admin role required")
		return
	}
	logo := strings.HasSuffix(r.URL.Path, "/logo")
	switch {
	case r.Method == "PUT" && !logo:
		var body struct {
			Name string `json:"name"`
		}
		if !decode(w, r, &body) {
			return
		}
		name := strings.TrimSpace(body.Name)
		if utf8.RuneCountInString(name) > 60 || strings.IndexFunc(name, func(c rune) bool { return c < 0x20 || c == 0x7f }) >= 0 {
			fail(w, 400, "Tên hiển thị tối đa 60 ký tự")
			return
		}
		if name == defaultBrandName {
			name = ""
		}
		if err := m.saveSettings(map[string]string{settingBrandName: name}); err != nil {
			fail(w, 500, "Không lưu được thương hiệu")
			return
		}
		m.auditReq(r, "branding.name", name)
		m.branding(w, r)
	case r.Method == "PUT" && logo:
		data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxLogoBytes))
		if err != nil {
			fail(w, 413, "Logo tối đa 256 KB")
			return
		}
		kind := imageType(data)
		if kind == "" {
			fail(w, 400, "Logo phải là ảnh PNG, JPEG, WebP, GIF hoặc SVG")
			return
		}
		if err = m.saveSettings(map[string]string{settingBrandLogo: base64.StdEncoding.EncodeToString(data), settingLogoType: kind}); err != nil {
			fail(w, 500, "Không lưu được thương hiệu")
			return
		}
		m.auditReq(r, "branding.logo", kind)
		m.branding(w, r)
	case r.Method == "DELETE" && logo:
		if err := m.saveSettings(map[string]string{settingBrandLogo: "", settingLogoType: ""}); err != nil {
			fail(w, 500, "Không lưu được thương hiệu")
			return
		}
		m.auditReq(r, "branding.logo_removed", "")
		m.branding(w, r)
	default:
		fail(w, 404, "Endpoint not found")
	}
}

// saveSettings stores values ("" deletes the key) and bumps the branding version.
func (m *Manager) saveSettings(values map[string]string) error {
	tx, err := m.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	values[settingBrandVer] = token()[:8]
	for k, v := range values {
		if v == "" {
			_, err = tx.Exec(`DELETE FROM settings WHERE key=?`, k)
		} else {
			_, err = tx.Exec(`INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, k, v)
		}
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// imageType returns the MIME type of a supported logo image, or "".
func imageType(data []byte) string {
	switch kind := http.DetectContentType(data); kind {
	case "image/png", "image/jpeg", "image/webp", "image/gif":
		return kind
	}
	head := bytes.ToLower(data[:min(len(data), 1024)])
	if bytes.Contains(head, []byte("<svg")) && utf8.Valid(data) {
		return "image/svg+xml"
	}
	return ""
}
