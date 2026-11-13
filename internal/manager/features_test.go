package manager

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// events collects notifications sent through notifyHook.
type events struct {
	mu   sync.Mutex
	list []Event
}

func (e *events) add(ev Event) { e.mu.Lock(); e.list = append(e.list, ev); e.mu.Unlock() }
func (e *events) all() []Event {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]Event(nil), e.list...)
}

func jobState(t *testing.T, m *Manager, id string) (status, message string, attempt int) {
	t.Helper()
	if err := m.db.QueryRow(`SELECT status,error,attempt FROM jobs WHERE id=?`, id).Scan(&status, &message, &attempt); err != nil {
		t.Fatal(err)
	}
	return
}

func TestFailedBackupIsRetriedThenNotifiedOnce(t *testing.T) {
	m := testManager(t)
	m.cfg.Retries, m.cfg.RetryDelay = 2, 10*time.Millisecond
	var got events
	m.notifyHook = got.add
	// The connection does not exist, so every attempt fails.
	j, err := m.enqueueForLocked("backup", "app", "", "0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		status, message, attempt := jobState(t, m, j.ID)
		if status == "failed" {
			if attempt != 2 || strings.Contains(message, "Lần thử") {
				t.Fatalf("attempt %d, message %q", attempt, message)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("job did not finish")
		}
		time.Sleep(10 * time.Millisecond)
	}
	ev := got.all()
	if len(ev) != 1 || ev[0].Kind != eventFailure || ev[0].Job.Attempt != 2 {
		t.Fatalf("notifications: %+v", ev)
	}
	var retries int
	m.db.QueryRow(`SELECT COUNT(*) FROM audit WHERE action='job.retry'`).Scan(&retries)
	if retries != 2 {
		t.Fatalf("retries audited: %d", retries)
	}
}

func TestRestoresAreNotRetried(t *testing.T) {
	m := testManager(t)
	m.cfg.Retries = 3
	for _, kind := range []string{"restore", "verify", "offsite_pull", "retention"} {
		if m.retryable(Job{Kind: kind}) {
			t.Fatalf("%s is retried", kind)
		}
	}
	if !m.retryable(Job{Kind: "scheduled"}) || m.retryable(Job{Kind: "scheduled", Attempt: 3}) {
		t.Fatal("scheduled backup retry limit")
	}
}

func TestNotifierAPIKeepsSecretsAndSendsSignedWebhook(t *testing.T) {
	m := testManager(t)
	var mu sync.Mutex
	var bodies [][]byte
	var signatures []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, b)
		signatures = append(signatures, r.Header.Get("X-DBBM-Signature"))
		mu.Unlock()
	}))
	defer srv.Close()
	admin := loginCookie(t, m, "admin", "test-admin-password")
	viewer := loginCookie(t, m, "viewer", "test-viewer-password")
	if w := request(m, "GET", "/api/notifiers", "", viewer); w.Code != 403 {
		t.Fatalf("viewer listed channels: %d", w.Code)
	}
	if w := request(m, "POST", "/api/notifiers", `{"name":"x","type":"telegram","config":{"bot_token":"bad","chat_id":"1"},"events":["failure"],"enabled":true}`, admin); w.Code != 400 {
		t.Fatalf("invalid token accepted: %d", w.Code)
	}
	if w := request(m, "POST", "/api/notifiers", `{"name":"x","type":"webhook","config":{"url":"`+srv.URL+`"},"events":["nope"],"enabled":true}`, admin); w.Code != 400 {
		t.Fatalf("invalid event accepted: %d", w.Code)
	}
	body := `{"name":"Ops","type":"webhook","config":{"url":"` + srv.URL + `/hook","secret":"s3cret"},"events":["failure","stale"],"lang":"en","enabled":true}`
	w := request(m, "POST", "/api/notifiers/test", body, admin)
	if w.Code != 200 {
		t.Fatalf("test send: %d %s", w.Code, w.Body)
	}
	w = request(m, "POST", "/api/notifiers", body, admin)
	if w.Code != 200 {
		t.Fatalf("save: %d %s", w.Code, w.Body)
	}
	var saved Notifier
	json.Unmarshal(w.Body.Bytes(), &saved)
	if strings.Contains(w.Body.String(), "s3cret") || strings.Contains(w.Body.String(), srv.URL) || len(saved.SecretsSet) != 2 {
		t.Fatalf("secrets returned: %s", w.Body)
	}
	// Editing without secrets keeps them.
	w = request(m, "PUT", "/api/notifiers/"+saved.ID, `{"name":"Ops 2","type":"webhook","config":{},"events":["failure"],"lang":"en","enabled":true}`, admin)
	if w.Code != 200 {
		t.Fatalf("edit: %d %s", w.Code, w.Body)
	}
	n, err := m.notifier(saved.ID)
	if err != nil || n.Config["url"] != srv.URL+"/hook" || n.Config["secret"] != "s3cret" || n.Name != "Ops 2" {
		t.Fatalf("stored: %+v %v", n, err)
	}
	// A failed job reaches the channel; a success does not (not subscribed).
	m.notifyJob(Job{Kind: "scheduled", Database: "app", Status: "failed", Error: "pg_dump exploded", FinishedAt: now()})
	m.notifyJob(Job{Kind: "backup", Database: "app", Status: "completed", FinishedAt: now()})
	deadline := time.Now().Add(5 * time.Second)
	for {
		mu.Lock()
		count := len(bodies)
		mu.Unlock()
		if count >= 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("webhook not called")
		}
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if len(bodies) != 2 {
		t.Fatalf("webhook calls: %d", len(bodies))
	}
	var payload map[string]any
	json.Unmarshal(bodies[1], &payload)
	if payload["event"] != "failure" || payload["title"] != "❌ Scheduled backup failed" || !strings.Contains(payload["text"].(string), "pg_dump exploded") {
		t.Fatalf("payload: %s", bodies[1])
	}
	mac := hmac.New(sha256.New, []byte("s3cret"))
	mac.Write(bodies[1])
	if signatures[1] != "sha256="+hex.EncodeToString(mac.Sum(nil)) {
		t.Fatal("bad signature")
	}
	if w = request(m, "DELETE", "/api/notifiers/"+saved.ID, "", admin); w.Code != 200 {
		t.Fatalf("delete: %d", w.Code)
	}
}

func TestTelegramSendRedactsToken(t *testing.T) {
	m := testManager(t)
	var path, text string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		text, _ = body["text"].(string)
		w.WriteHeader(401)
		w.Write([]byte(`{"ok":false}`))
	}))
	defer srv.Close()
	old := telegramAPI
	telegramAPI = srv.URL
	defer func() { telegramAPI = old }()
	token := "123456:ABCDEFGHIJKLMNOPQRSTUVWXYZabcdef"
	n := Notifier{Name: "tg", Type: "telegram", Config: map[string]string{"bot_token": token, "chat_id": "-1001234"}, Events: []string{"failure"}}
	if err := validateNotifier(&n, nil); err != nil {
		t.Fatal(err)
	}
	err := m.send(t.Context(), n, Event{Kind: eventTest})
	if err == nil || strings.Contains(err.Error(), token) || !strings.Contains(err.Error(), "401") {
		t.Fatalf("error: %v", err)
	}
	if path != "/bot"+token+"/sendMessage" || !strings.Contains(text, "DB Backup Manager") {
		t.Fatalf("request: %s %q", path, text)
	}
}

func TestEmailNotifierValidation(t *testing.T) {
	n := Notifier{Name: "mail", Type: "email", Config: map[string]string{"host": "smtp.example.com", "from": "Backup <backup@example.com>", "to": "a@example.com, b@example.com", "password": "x"}, Events: []string{"failure"}}
	if err := validateNotifier(&n, nil); err != nil {
		t.Fatal(err)
	}
	if n.Config["security"] != "starttls" || n.Config["port"] != "587" {
		t.Fatalf("defaults: %+v", n.Config)
	}
	n.Config["to"] = "not an address"
	if validateNotifier(&n, nil) == nil {
		t.Fatal("bad recipient accepted")
	}
}

func TestStaleScheduleIsReportedOnce(t *testing.T) {
	m := testManager(t)
	var got events
	m.notifyHook = got.add
	m.started = time.Now().Add(-3 * time.Hour)
	if _, err := m.db.Exec(`INSERT INTO schedules(connection_id,database_name,cron,retention_days,enabled,updated_at) VALUES('','app','0 * * * *',7,1,?)`, time.Now().Add(-3*time.Hour).UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	m.checkStale()
	m.checkStale()
	ev := got.all()
	if len(ev) != 1 || ev[0].Kind != eventStale || ev[0].Database != "app" {
		t.Fatalf("events: %+v", ev)
	}
	subject, body := m.message("vi", ev[0])
	if subject != "⚠️ Lịch backup không chạy" || !strings.Contains(body, "chưa có") {
		t.Fatalf("message: %s / %s", subject, body)
	}
	// Any job since the due run (even a failed one) means the schedule ran.
	m.recordScheduleFailure("app", errFake("boom"), "")
	m.checkStale()
	m.staleMu.Lock()
	n := len(m.stale)
	m.staleMu.Unlock()
	if n != 0 {
		t.Fatal("stale state not cleared")
	}
}

type errFake string

func (e errFake) Error() string { return string(e) }

func TestMissedScheduleIsCaughtUpOnce(t *testing.T) {
	m := testManager(t)
	var got events
	m.notifyHook = got.add
	twoDaysAgo := time.Now().Add(-49 * time.Hour).UTC().Format(time.RFC3339Nano)
	// Missed (daily, last run two days ago), not missed (yearly) and paused.
	m.db.Exec(`INSERT INTO schedules(connection_id,database_name,cron,retention_days,enabled,last_run_at) VALUES('0123456789abcdef0123456789abcdef','app','0 2 * * *',7,1,?)`, twoDaysAgo)
	m.db.Exec(`INSERT INTO schedules(connection_id,database_name,cron,retention_days,enabled,last_run_at) VALUES('','yearly','0 0 1 1 *',7,1,?)`, time.Now().UTC().Format(time.RFC3339Nano))
	m.db.Exec(`INSERT INTO schedules(connection_id,database_name,cron,retention_days,enabled,last_run_at) VALUES('','paused','0 2 * * *',7,0,?)`, twoDaysAgo)
	m.catchUp()
	jobs, _ := m.jobs()
	if len(jobs) != 1 || jobs[0].Database != "app" || jobs[0].Kind != "scheduled" {
		t.Fatalf("jobs: %+v", jobs)
	}
	ss, _ := m.schedules()
	if ss[0].LastRunAt == twoDaysAgo {
		t.Fatal("last run not recorded")
	}
	m.catchUp()
	if jobs, _ = m.jobs(); len(jobs) != 1 {
		t.Fatal("caught up twice")
	}
}

func TestMetricsEndpoint(t *testing.T) {
	m := testManager(t)
	if w := request(m, "GET", "/metrics", "", nil); w.Code != 404 {
		t.Fatalf("metrics enabled without token: %d", w.Code)
	}
	m.cfg.MetricsToken = "metrics-token-for-tests"
	if w := request(m, "GET", "/metrics", "", nil); w.Code != 401 {
		t.Fatalf("no token: %d", w.Code)
	}
	id := token()
	m.saveBackup(Backup{ID: id, Database: `we"ird`, Size: 42, Checksum: "x", CreatedAt: now(), Source: "manual"})
	m.db.Exec(`UPDATE backups SET verify_status='passed',verified_at=? WHERE id=?`, now(), id)
	r := httptest.NewRequest("GET", "/metrics", nil)
	r.Header.Set("Authorization", "Bearer metrics-token-for-tests")
	w := httptest.NewRecorder()
	m.Handler().ServeHTTP(w, r)
	body := w.Body.String()
	for _, want := range []string{"dbbm_up 1", `dbbm_backup_last_size_bytes{connection_id="",connection="",database="we\"ird"} 42`, "dbbm_verify_last_success{", "dbbm_backup_disk_free_bytes ", `dbbm_schedules{enabled="true"} 0`} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in:\n%s", want, body)
		}
	}
}

func TestVerifyAPIChecksTarget(t *testing.T) {
	m := testManager(t)
	id := token()
	m.saveBackup(Backup{ID: id, Database: "app", Size: 1, Checksum: "x", CreatedAt: now(), Source: "manual"})
	admin := loginCookie(t, m, "admin", "test-admin-password")
	viewer := loginCookie(t, m, "viewer", "test-viewer-password")
	if w := request(m, "POST", "/api/backups/"+id+"/verify", `{}`, viewer); w.Code != 403 {
		t.Fatalf("viewer verify: %d", w.Code)
	}
	m.cfg.RestoreUser = ""
	if w := request(m, "POST", "/api/backups/"+id+"/verify", `{}`, admin); w.Code != 400 || !strings.Contains(w.Body.String(), "tài khoản restore") {
		t.Fatalf("verify without restore login: %d %s", w.Code, w.Body)
	}
	m.cfg.RestoreUser = "restore_user"
	if w := request(m, "POST", "/api/backups/"+id+"/verify", `{"connection_id":"0123456789abcdef0123456789abcdef"}`, admin); w.Code != 400 {
		t.Fatalf("unknown target: %d", w.Code)
	}
	if w := request(m, "POST", "/api/backups/"+id+"/verify", `{}`, admin); w.Code != 202 {
		t.Fatalf("verify: %d %s", w.Code, w.Body)
	}
}

func TestScheduleVerifyNeedsRestoreLogin(t *testing.T) {
	m := testManager(t)
	admin := loginCookie(t, m, "admin", "test-admin-password")
	m.cfg.RestoreUser = ""
	w := request(m, "POST", "/api/schedules", `{"connection_id":"","database":"app","cron":"0 2 * * *","retention_days":7,"enabled":true,"verify":true}`, admin)
	if w.Code != 400 {
		t.Fatalf("verify without restore login accepted: %d %s", w.Code, w.Body)
	}
	m.cfg.RestoreUser = "restore_user"
	w = request(m, "POST", "/api/schedules", `{"connection_id":"","database":"app","cron":"0 2 * * *","retention_days":7,"enabled":true,"verify":true}`, admin)
	if w.Code != 200 {
		t.Fatalf("save: %d %s", w.Code, w.Body)
	}
	ss, _ := m.schedules()
	if len(ss) != 1 || !ss[0].Verify || ss[0].UpdatedAt == "" {
		t.Fatalf("schedule: %+v", ss)
	}
}

func TestConnectionDownAndRecovery(t *testing.T) {
	m := testManager(t)
	var got events
	m.notifyHook = got.add
	var mu sync.Mutex
	failing := errFake("dial tcp 127.0.0.1:5432: connection refused")
	var fail error = failing
	old := testConnection
	testConnection = func(context.Context, *Manager) error { mu.Lock(); defer mu.Unlock(); return fail }
	t.Cleanup(func() { testConnection = old })

	m.checkConnections() // one blip stays quiet
	if ev := got.all(); len(ev) != 0 {
		t.Fatalf("first failure reported: %+v", ev)
	}
	m.checkConnections()
	m.checkConnections() // still down, reported once
	ev := got.all()
	if len(ev) != 1 || ev[0].Kind != eventConnection || ev[0].Recovered || ev[0].Since == "" || !strings.Contains(ev[0].Detail, "refused") {
		t.Fatalf("down events: %+v", ev)
	}
	subject, body := m.message("vi", ev[0])
	if subject != "🔴 Mất kết nối database" || !strings.Contains(body, "127.0.0.1") || !strings.Contains(body, "connection refused") {
		t.Fatalf("message: %s / %s", subject, body)
	}
	var metrics metricWriter
	if err := m.writeMetrics(&metrics); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(metrics.String(), `dbbm_connection_up{connection_id="",connection=""} 0`) {
		t.Fatalf("metric missing:\n%s", metrics.String())
	}

	mu.Lock()
	fail = nil
	mu.Unlock()
	m.checkConnections()
	m.checkConnections()
	ev = got.all()
	if len(ev) != 2 || !ev[1].Recovered || ev[1].Since != ev[0].Since {
		t.Fatalf("recovery events: %+v", ev)
	}
	if subject, _ = m.message("en", ev[1]); subject != "🟢 Database connection restored" {
		t.Fatalf("recovery subject: %s", subject)
	}
}
