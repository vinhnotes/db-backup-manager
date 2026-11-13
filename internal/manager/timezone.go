package manager

import (
	"net/http"
	"strings"
	"time"
)

// settingTimezone overrides TZ from the environment once an admin picks a
// timezone in the UI. Deleting the row falls back to TZ again.
const settingTimezone = "scheduler_timezone"

func (m *Manager) timezone() string {
	m.tzMu.RLock()
	defer m.tzMu.RUnlock()
	return m.tz
}

func (m *Manager) loadTimezoneSetting() {
	var tz string
	if err := m.db.QueryRow(`SELECT value FROM settings WHERE key=?`, settingTimezone).Scan(&tz); err != nil || tz == "" {
		return
	}
	if _, err := time.LoadLocation(tz); err != nil {
		m.logger.Printf("ignoring stored timezone %q: %v", tz, err)
		return
	}
	m.tzMu.Lock()
	m.tz = tz
	m.tzMu.Unlock()
}

// setTimezone changes the scheduler timezone and reloads every schedule.
// An empty timezone resets to TZ from the environment.
func (m *Manager) setTimezone(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Timezone string `json:"timezone"`
	}
	if !decode(w, r, &body) {
		return
	}
	tz := strings.TrimSpace(body.Timezone)
	if tz == "" {
		tz = m.cfg.Timezone
	}
	if tz == "Local" || len(tz) > 64 {
		fail(w, 400, "Múi giờ không hợp lệ")
		return
	}
	if _, err := time.LoadLocation(tz); err != nil {
		fail(w, 400, "Múi giờ không hợp lệ")
		return
	}
	m.schedMu.Lock()
	defer m.schedMu.Unlock()
	var err error
	if tz == m.cfg.Timezone {
		_, err = m.db.Exec(`DELETE FROM settings WHERE key=?`, settingTimezone)
	} else {
		_, err = m.db.Exec(`INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, settingTimezone, tz)
	}
	if err != nil {
		fail(w, 500, "Không lưu được múi giờ")
		return
	}
	m.tzMu.Lock()
	old := m.tz
	m.tz = tz
	m.tzMu.Unlock()
	if err = m.loadSchedules(); err != nil {
		m.logger.Print(err)
		fail(w, 500, "Không nạp lại được lịch chạy")
		return
	}
	m.auditReq(r, "settings.timezone", old+" -> "+tz)
	respond(w, 200, map[string]string{"timezone": tz})
}
