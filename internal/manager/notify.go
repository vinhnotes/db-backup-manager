package manager

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

// Notification channels send a message when a job fails, when it succeeds,
// when a schedule has silently not run (stale), or when a database connection
// goes down or comes back. Settings, which hold
// tokens and passwords, are stored encrypted like connection passwords.

const (
	eventFailure     = "failure"
	eventSuccess     = "success"
	eventStale       = "stale"
	eventConnection  = "connection"
	eventDisk        = "disk"
	eventBackupSize  = "backup_size"
	eventOverdue     = "overdue"
	eventDestination = "destination"
	eventRestore     = "restore"
	eventSecurity    = "security"
	eventTest        = "test"
)

var notifierTypes = []string{"telegram", "slack", "discord", "webhook", "email"}
var notifierEvents = []string{eventFailure, eventSuccess, eventStale, eventConnection, eventDisk, eventBackupSize, eventOverdue, eventDestination, eventRestore, eventSecurity}
var notifierLangs = []string{"vi", "en", "ja"}

// notifierFields lists the settings of each channel type; secret ones are
// never returned by the API and are kept when left blank on edit.
var notifierFields = map[string][]string{
	"telegram": {"bot_token", "chat_id"},
	"slack":    {"webhook_url"},
	"discord":  {"webhook_url"},
	"webhook":  {"url", "secret"},
	"email":    {"host", "port", "security", "username", "password", "from", "to"},
}
var secretFields = map[string]bool{"bot_token": true, "webhook_url": true, "url": true, "secret": true, "password": true}

// API endpoints, variables so tests can point them at a local server.
var telegramAPI = "https://api.telegram.org"

type Notifier struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Type      string            `json:"type"`
	Config    map[string]string `json:"config"`
	Events    []string          `json:"events"`
	Lang      string            `json:"lang"`
	Enabled   bool              `json:"enabled"`
	CreatedAt string            `json:"created_at"`
	// SecretsSet names the secret settings that have a stored value.
	SecretsSet []string `json:"secrets_set"`
}

// Event is something worth telling people about.
type Event struct {
	Kind         string // eventFailure, eventSuccess, eventStale, eventConnection or eventTest
	Job          Job
	ConnectionID string
	Database     string
	// Since: stale/overdue: last successful backup, if any; connection and
	// destination: start of the outage.
	Since     string
	Recovered bool   // connection, destination, disk: the problem is over
	Detail    string // connection, destination: last error; disk: free space; overdue: limit
	// Target: destination name, backup directory (disk) or client address
	// (security).
	Target         string
	DestinationID  string
	Actor          string // restore: who asked for it
	Source         Backup // restore: the backup being restored
	Size, PrevSize int64  // backup_size
}

func (m *Manager) notifiers() ([]Notifier, error) {
	rows, err := m.db.Query(`SELECT id,name,type,config,events,lang,enabled,created_at FROM notifiers ORDER BY name,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Notifier{}
	for rows.Next() {
		var n Notifier
		var sealed, events string
		if err = rows.Scan(&n.ID, &n.Name, &n.Type, &sealed, &events, &n.Lang, &n.Enabled, &n.CreatedAt); err != nil {
			return nil, err
		}
		raw, err := m.openSealed(sealed)
		if err != nil {
			return nil, errors.New("cannot decrypt notification settings")
		}
		n.Config = map[string]string{}
		if err = json.Unmarshal([]byte(raw), &n.Config); err != nil {
			return nil, err
		}
		n.Events = strings.Split(events, ",")
		out = append(out, n)
	}
	return out, rows.Err()
}

func (m *Manager) notifier(id string) (Notifier, error) {
	list, err := m.notifiers()
	if err != nil {
		return Notifier{}, err
	}
	for _, n := range list {
		if n.ID == id {
			return n, nil
		}
	}
	return Notifier{}, errors.New("Kênh thông báo không tồn tại")
}

// public returns n without secret settings.
func (n Notifier) public() Notifier {
	cfg := map[string]string{}
	n.SecretsSet = []string{}
	for k, v := range n.Config {
		if secretFields[k] {
			if v != "" {
				n.SecretsSet = append(n.SecretsSet, k)
			}
			continue
		}
		cfg[k] = v
	}
	slices.Sort(n.SecretsSet)
	n.Config = cfg
	return n
}

var (
	telegramToken = regexp.MustCompile(`^[0-9]{3,20}:[A-Za-z0-9_-]{20,100}$`)
	telegramChat  = regexp.MustCompile(`^(-?[0-9]{1,20}|@[A-Za-z0-9_]{4,64})$`)
)

// validateNotifier normalises n; secret settings left blank are taken from old.
func validateNotifier(n *Notifier, old *Notifier) error {
	n.Name = strings.TrimSpace(n.Name)
	if n.Name == "" || len(n.Name) > 100 || strings.ContainsAny(n.Name, "\x00\r\n") {
		return errors.New("Tên kênh thông báo gồm 1–100 ký tự")
	}
	if !slices.Contains(notifierTypes, n.Type) {
		return errors.New("Loại kênh thông báo không được hỗ trợ")
	}
	if n.Lang == "" {
		n.Lang = "vi"
	}
	if !slices.Contains(notifierLangs, n.Lang) {
		return errors.New("Ngôn ngữ thông báo không hợp lệ")
	}
	events := []string{}
	for _, e := range notifierEvents {
		if slices.Contains(n.Events, e) {
			events = append(events, e)
		}
	}
	if len(events) == 0 || len(events) != len(n.Events) {
		return errors.New("Chọn ít nhất một sự kiện hợp lệ để thông báo")
	}
	n.Events = events
	cfg := map[string]string{}
	for _, k := range notifierFields[n.Type] {
		v := strings.TrimSpace(n.Config[k])
		if v == "" && secretFields[k] && old != nil && old.Type == n.Type {
			v = old.Config[k]
		}
		if len(v) > 2048 || strings.ContainsAny(v, "\x00\r\n") {
			return errors.New("Cấu hình kênh thông báo không hợp lệ")
		}
		cfg[k] = v
	}
	for k := range n.Config {
		if !slices.Contains(notifierFields[n.Type], k) {
			return errors.New("Cấu hình kênh thông báo không hợp lệ")
		}
	}
	n.Config = cfg
	switch n.Type {
	case "telegram":
		if !telegramToken.MatchString(cfg["bot_token"]) {
			return errors.New("Bot token Telegram không hợp lệ (dạng 123456:ABC…)")
		}
		if !telegramChat.MatchString(cfg["chat_id"]) {
			return errors.New("Chat ID Telegram không hợp lệ (số, ví dụ -1001234567890, hoặc @tên_kênh)")
		}
	case "slack", "discord":
		if !validURL(cfg["webhook_url"], true) {
			return errors.New("Webhook URL phải là địa chỉ https:// hợp lệ")
		}
	case "webhook":
		if !validURL(cfg["url"], false) {
			return errors.New("URL webhook phải bắt đầu bằng http:// hoặc https://")
		}
	case "email":
		if cfg["host"] == "" || strings.ContainsAny(cfg["host"], " /@") {
			return errors.New("Máy chủ SMTP không hợp lệ")
		}
		if cfg["security"] == "" {
			cfg["security"] = "starttls"
		}
		if !slices.Contains([]string{"starttls", "tls", "none"}, cfg["security"]) {
			return errors.New("Kiểu bảo mật SMTP không hợp lệ")
		}
		if cfg["port"] == "" {
			cfg["port"] = map[string]string{"starttls": "587", "tls": "465", "none": "25"}[cfg["security"]]
		}
		if p, err := strconv.Atoi(cfg["port"]); err != nil || p < 1 || p > 65535 {
			return errors.New("Port SMTP không hợp lệ")
		}
		if _, err := mail.ParseAddress(cfg["from"]); err != nil {
			return errors.New("Địa chỉ người gửi không hợp lệ")
		}
		if to, err := mail.ParseAddressList(cfg["to"]); err != nil || len(to) == 0 || len(to) > 20 {
			return errors.New("Danh sách người nhận không hợp lệ (tối đa 20 địa chỉ, cách nhau bằng dấu phẩy)")
		}
	}
	return nil
}

func validURL(raw string, httpsOnly bool) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil {
		return false
	}
	return u.Scheme == "https" || (!httpsOnly && u.Scheme == "http")
}

// notifierAPI serves /api/notifiers. Only administrators manage channels.
func (m *Manager) notifierAPI(w http.ResponseWriter, r *http.Request) {
	if !principalOf(r).Admin() {
		fail(w, 403, "Admin role required")
		return
	}
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/notifiers"), "/")
	switch {
	case r.Method == "GET" && rest == "":
		list, err := m.notifiers()
		if err != nil {
			fail(w, 500, "Cannot read notification channels")
			return
		}
		for i := range list {
			list[i] = list[i].public()
		}
		respond(w, 200, list)
	case r.Method == "POST" && (rest == "" || rest == "test"):
		m.saveNotifier(w, r, "", rest == "test")
	case r.Method == "PUT" && idPattern.MatchString(rest):
		m.saveNotifier(w, r, rest, false)
	case r.Method == "DELETE" && idPattern.MatchString(rest):
		res, err := m.db.Exec(`DELETE FROM notifiers WHERE id=?`, rest)
		if err != nil {
			fail(w, 500, "Cannot delete notification channel")
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			fail(w, 404, "Kênh thông báo không tồn tại")
			return
		}
		m.auditReq(r, "notifier.deleted", rest)
		respond(w, 200, map[string]bool{"ok": true})
	default:
		fail(w, 404, "Endpoint not found")
	}
}

func (m *Manager) saveNotifier(w http.ResponseWriter, r *http.Request, id string, test bool) {
	var n Notifier
	if !decode(w, r, &n) {
		return
	}
	if test {
		id = n.ID
	}
	var old *Notifier
	if id != "" {
		if !idPattern.MatchString(id) {
			fail(w, 404, "Kênh thông báo không tồn tại")
			return
		}
		stored, err := m.notifier(id)
		if err != nil {
			fail(w, 404, err.Error())
			return
		}
		old = &stored
		n.CreatedAt = stored.CreatedAt
	}
	if test && len(n.Events) == 0 {
		n.Events = []string{eventFailure}
	}
	if err := validateNotifier(&n, old); err != nil {
		fail(w, 400, err.Error())
		return
	}
	if test {
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		if err := m.send(ctx, n, Event{Kind: eventTest}); err != nil {
			fail(w, 400, "Gửi thử thất bại: "+err.Error())
			return
		}
		m.auditReq(r, "notifier.tested", n.Name)
		respond(w, 200, map[string]bool{"ok": true})
		return
	}
	if id == "" {
		id, n.CreatedAt = token(), now()
	}
	n.ID = id
	cfg, _ := json.Marshal(n.Config)
	_, err := m.db.Exec(`INSERT INTO notifiers(id,name,type,config,events,lang,enabled,created_at) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET name=excluded.name,type=excluded.type,config=excluded.config,events=excluded.events,lang=excluded.lang,enabled=excluded.enabled`,
		n.ID, n.Name, n.Type, m.sealPassword(string(cfg)), strings.Join(n.Events, ","), n.Lang, n.Enabled, n.CreatedAt)
	if err != nil {
		fail(w, 500, "Cannot save notification channel")
		return
	}
	m.auditReq(r, "notifier.saved", n.ID+" "+n.Name+" ("+n.Type+")")
	respond(w, 200, n.public())
}

// ---- dispatch ---------------------------------------------------------------

// notifyJob reports a finished job: failures always, successes for the kinds
// people act on.
func (m *Manager) notifyJob(j Job) {
	switch {
	case j.Status == "failed":
		m.notify(Event{Kind: eventFailure, Job: j, ConnectionID: j.ConnectionID, Database: j.Database})
	case j.Status == "completed" && slices.Contains([]string{"backup", "scheduled", "restore", "verify"}, j.Kind):
		m.notify(Event{Kind: eventSuccess, Job: j, ConnectionID: j.ConnectionID, Database: j.Database})
	}
}

// notify sends ev to every enabled channel subscribed to it, in the
// background. Errors are logged and recorded in the audit log.
func (m *Manager) notify(ev Event) {
	if m.notifyHook != nil {
		m.notifyHook(ev)
		return
	}
	list, err := m.notifiers()
	if err != nil {
		m.logger.Printf("notification channels unavailable: %v", err)
		return
	}
	for _, n := range list {
		if !n.Enabled || !slices.Contains(n.Events, ev.Kind) {
			continue
		}
		n := n
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := m.send(ctx, n, ev); err != nil {
				m.notifyFailures.Add(1)
				m.logger.Printf("notification %s failed: %v", n.Name, err)
				m.audit("notifier.failed", n.Name+": "+truncate(err.Error(), 300))
			}
		}()
	}
}

// ---- messages ---------------------------------------------------------------

var notifyText = map[string]map[string]string{
	"vi": {
		"failure": "❌ %s thất bại", "success": "✅ %s hoàn tất", "stale": "⚠️ Lịch backup không chạy",
		"test": "🔔 Thông báo thử từ DB Backup Manager", "testBody": "Kênh thông báo hoạt động bình thường.",
		"connection": "Kết nối", "database": "Database", "error": "Lỗi", "time": "Thời gian", "attempts": "Số lần thử",
		"staleBody": "Đã đến giờ chạy theo lịch nhưng không có job backup nào được tạo.", "lastOK": "Backup thành công gần nhất", "never": "chưa có",
		"backup": "Backup", "scheduled": "Backup theo lịch", "restore": "Restore", "verify": "Kiểm thử restore", "offsite": "Upload lên cloud", "offsite_pull": "Tải từ cloud", "retention": "Dọn bản cũ",
		"all": "tất cả", "deleted": "(đã xóa)",
		"connDown": "🔴 Mất kết nối database", "connUp": "🟢 Kết nối database đã khôi phục", "downSince": "Mất kết nối từ",
		"connDownBody": "Không kết nối được tới database ở các lần kiểm tra liên tiếp. Backup sẽ thất bại cho tới khi kết nối trở lại.",
		"diskLow":      "💾 Ổ đĩa backup sắp đầy", "diskOK": "💾 Ổ đĩa backup đã đủ chỗ trở lại", "diskBody": "Dung lượng trống dưới %s%%. Backup sẽ thất bại khi ổ đầy.", "free": "Còn trống", "folder": "Thư mục",
		"shrink": "📉 Bản backup nhỏ bất thường", "shrinkBody": "Bản mới nhỏ hơn bản trước quá %s%%. Job vẫn hoàn tất nhưng dữ liệu có thể bị thiếu.", "size": "Dung lượng", "prevSize": "Bản trước",
		"overdue": "⏰ Lâu không có backup thành công", "overdueBody": "Đã quá %s không có bản backup thành công nào.",
		"destDown": "☁️ Không truy cập được nơi lưu cloud", "destUp": "☁️ Nơi lưu cloud đã hoạt động lại", "destDownBody": "Ghi/đọc thử lên cloud thất bại ở các lần kiểm tra liên tiếp. Upload bản sao sẽ lỗi cho tới khi khắc phục.", "destination": "Nơi lưu",
		"restoreReq": "♻️ Có yêu cầu restore", "actor": "Người thực hiện", "target": "Database đích", "source": "Bản backup",
		"locked": "🔒 Đăng nhập sai quá nhiều lần", "lockedBody": "Một địa chỉ đã đăng nhập sai %d lần và bị khóa 15 phút.", "ip": "Địa chỉ IP",
	},
	"en": {
		"failure": "❌ %s failed", "success": "✅ %s completed", "stale": "⚠️ Scheduled backup did not run",
		"test": "🔔 Test notification from DB Backup Manager", "testBody": "This notification channel works.",
		"connection": "Connection", "database": "Database", "error": "Error", "time": "Time", "attempts": "Attempts",
		"staleBody": "A scheduled run was due but no backup job was created.", "lastOK": "Last successful backup", "never": "none",
		"backup": "Backup", "scheduled": "Scheduled backup", "restore": "Restore", "verify": "Restore test", "offsite": "Cloud upload", "offsite_pull": "Cloud download", "retention": "Retention cleanup",
		"all": "all", "deleted": "(deleted)",
		"connDown": "🔴 Database connection lost", "connUp": "🟢 Database connection restored", "downSince": "Down since",
		"connDownBody": "Consecutive checks could not reach the database. Backups will fail until the connection is back.",
		"diskLow":      "💾 Backup disk almost full", "diskOK": "💾 Backup disk has room again", "diskBody": "Free space is below %s%%. Backups will fail once the disk is full.", "free": "Free", "folder": "Folder",
		"shrink": "📉 Backup unusually small", "shrinkBody": "The new backup is more than %s%% smaller than the previous one. The job completed, but data may be missing.", "size": "Size", "prevSize": "Previous",
		"overdue": "⏰ No successful backup for too long", "overdueBody": "No successful backup for more than %s.",
		"destDown": "☁️ Cloud destination unreachable", "destUp": "☁️ Cloud destination works again", "destDownBody": "Consecutive test writes to the cloud failed. Uploads will fail until this is fixed.", "destination": "Destination",
		"restoreReq": "♻️ Restore requested", "actor": "Requested by", "target": "Target database", "source": "Backup",
		"locked": "🔒 Too many failed logins", "lockedBody": "An address failed to log in %d times and is locked out for 15 minutes.", "ip": "IP address",
	},
	"ja": {
		"failure": "❌ %s に失敗しました", "success": "✅ %s が完了しました", "stale": "⚠️ スケジュールバックアップが実行されていません",
		"test": "🔔 DB Backup Manager からのテスト通知", "testBody": "この通知チャネルは正常に動作しています。",
		"connection": "接続", "database": "データベース", "error": "エラー", "time": "時刻", "attempts": "試行回数",
		"staleBody": "スケジュールの実行時刻を過ぎましたが、バックアップジョブが作成されていません。", "lastOK": "最後に成功したバックアップ", "never": "なし",
		"backup": "バックアップ", "scheduled": "スケジュールバックアップ", "restore": "リストア", "verify": "リストアテスト", "offsite": "クラウドへのアップロード", "offsite_pull": "クラウドからのダウンロード", "retention": "古いバックアップの削除",
		"all": "すべて", "deleted": "(削除済み)",
		"connDown": "🔴 データベースに接続できません", "connUp": "🟢 データベース接続が復旧しました", "downSince": "接続断の開始",
		"connDownBody": "連続したチェックでデータベースに接続できませんでした。接続が戻るまでバックアップは失敗します。",
		"diskLow":      "💾 バックアップディスクの空きが不足しています", "diskOK": "💾 バックアップディスクの空き容量が回復しました", "diskBody": "空き容量が %s%% を下回りました。ディスクが満杯になるとバックアップは失敗します。", "free": "空き容量", "folder": "フォルダー",
		"shrink": "📉 バックアップのサイズが異常に小さい", "shrinkBody": "新しいバックアップが前回より %s%% 以上小さくなりました。ジョブは完了しましたが、データが欠けている可能性があります。", "size": "サイズ", "prevSize": "前回",
		"overdue": "⏰ 長時間バックアップが成功していません", "overdueBody": "%s 以上、成功したバックアップがありません。",
		"destDown": "☁️ クラウド保存先に接続できません", "destUp": "☁️ クラウド保存先が復旧しました", "destDownBody": "クラウドへのテスト書き込みが連続して失敗しました。解決するまでアップロードは失敗します。", "destination": "保存先",
		"restoreReq": "♻️ リストアが要求されました", "actor": "実行者", "target": "リストア先データベース", "source": "バックアップ",
		"locked": "🔒 ログイン失敗が多すぎます", "lockedBody": "あるアドレスからのログインが %d 回失敗し、15 分間ロックされました。", "ip": "IP アドレス",
	},
}

// message renders ev as a subject line and a plain-text body.
func (m *Manager) message(lang string, ev Event) (string, string) {
	tr := notifyText[lang]
	if tr == nil {
		tr = notifyText["vi"]
	}
	loc, err := time.LoadLocation(m.timezone())
	if err != nil {
		loc = time.UTC
	}
	stamp := func(s string) string {
		t, err := time.Parse(time.RFC3339Nano, s)
		if err != nil {
			return s
		}
		return t.In(loc).Format("2006-01-02 15:04:05 MST")
	}
	var subject string
	lines := []string{}
	field := func(k, v string) {
		if v != "" {
			lines = append(lines, tr[k]+": "+v)
		}
	}
	pct := func(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
	switch ev.Kind {
	case eventTest:
		return strings.ReplaceAll(tr["test"], defaultBrandName, m.brandName()), tr["testBody"]
	case eventStale:
		subject = tr["stale"]
		lines = append(lines, tr["staleBody"])
	case eventConnection:
		subject = tr["connDown"]
		if ev.Recovered {
			subject = tr["connUp"]
		} else {
			lines = append(lines, tr["connDownBody"])
		}
	case eventDestination:
		subject = tr["destDown"]
		if ev.Recovered {
			subject = tr["destUp"]
		} else {
			lines = append(lines, tr["destDownBody"])
		}
	case eventDisk:
		subject = tr["diskOK"]
		if !ev.Recovered {
			subject = tr["diskLow"]
			lines = append(lines, fmt.Sprintf(tr["diskBody"], pct(m.cfg.DiskAlertPercent)))
		}
	case eventBackupSize:
		subject = tr["shrink"]
		lines = append(lines, fmt.Sprintf(tr["shrinkBody"], pct(m.cfg.BackupShrinkPercent)))
	case eventOverdue:
		subject = tr["overdue"]
		lines = append(lines, fmt.Sprintf(tr["overdueBody"], ev.Detail))
	case eventRestore:
		subject = tr["restoreReq"]
	case eventSecurity:
		subject = tr["locked"]
		lines = append(lines, fmt.Sprintf(tr["lockedBody"], loginLimit))
	default:
		kind := tr[ev.Job.Kind]
		if kind == "" {
			kind = ev.Job.Kind
		}
		subject = fmt.Sprintf(tr[ev.Kind], kind)
	}
	if ev.ConnectionID != "" || ev.Database != "" || ev.Kind == eventConnection {
		conn := m.connectionName(ev.ConnectionID)
		if conn == "" && ev.ConnectionID == "" {
			conn = m.cfg.PGHost // the connection from the environment
		}
		if conn == "" {
			conn = tr["deleted"]
		}
		field("connection", conn)
	}
	db := ev.Database
	if db == "*" {
		db = tr["all"]
	}
	if ev.Kind == eventRestore {
		field("target", db)
	} else {
		field("database", db)
	}
	switch ev.Kind {
	case eventConnection, eventDestination:
		field("destination", ev.Target)
		field("time", stamp(now()))
		if ev.Since != "" {
			field("downSince", stamp(ev.Since))
		}
		field("error", truncate(translateServer(lang, ev.Detail), 1500))
	case eventStale, eventOverdue:
		since := tr["never"]
		if ev.Since != "" {
			since = stamp(ev.Since)
		}
		field("lastOK", since)
	case eventDisk:
		field("folder", ev.Target)
		field("free", ev.Detail)
		field("time", stamp(now()))
	case eventBackupSize:
		field("size", formatBytes(ev.Size))
		field("prevSize", formatBytes(ev.PrevSize))
		field("time", stamp(now()))
	case eventRestore:
		field("source", ev.Source.Database+" ("+stamp(ev.Source.CreatedAt)+")")
		field("actor", ev.Actor)
		field("time", stamp(now()))
	case eventSecurity:
		field("ip", ev.Target)
		field("time", stamp(now()))
	default:
		when := ev.Job.FinishedAt
		if when == "" {
			when = now()
		}
		field("time", stamp(when))
		if ev.Job.Attempt > 0 {
			field("attempts", strconv.Itoa(ev.Job.Attempt+1))
		}
		if ev.Job.Error != "" {
			field("error", truncate(translateServer(lang, ev.Job.Error), 1500))
		}
	}
	return subject, strings.Join(lines, "\n")
}

// connectionName is the display name of a connection, "" if it is gone.
func (m *Manager) connectionName(id string) string {
	var name string
	_ = m.db.QueryRow(`SELECT name FROM connections WHERE id=?`, id).Scan(&name)
	return name
}

// ---- senders ----------------------------------------------------------------

// Channels are admin-selected destinations, including private webhooks. Never
// let that destination redirect a notification (and its signature) elsewhere.
var notifyClient = &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
	return http.ErrUseLastResponse
}}

func (m *Manager) send(ctx context.Context, n Notifier, ev Event) error {
	subject, body := m.message(n.Lang, ev)
	text := subject + "\n" + body
	var err error
	switch n.Type {
	case "telegram":
		err = postJSON(ctx, telegramAPI+"/bot"+n.Config["bot_token"]+"/sendMessage", map[string]any{"chat_id": n.Config["chat_id"], "text": truncate(text, 4000), "disable_web_page_preview": true}, nil)
	case "slack":
		err = postJSON(ctx, n.Config["webhook_url"], map[string]string{"text": text}, nil)
	case "discord":
		err = postJSON(ctx, n.Config["webhook_url"], map[string]string{"content": truncate(text, 1900)}, nil)
	case "webhook":
		event := ev.Kind
		switch ev.Kind {
		case eventConnection, eventDestination:
			event += map[bool]string{false: "_down", true: "_up"}[ev.Recovered]
		case eventDisk:
			event += map[bool]string{false: "_low", true: "_ok"}[ev.Recovered]
		}
		payload := map[string]any{"event": event, "title": subject, "text": body, "connection_id": ev.ConnectionID, "connection": m.connectionName(ev.ConnectionID), "database": ev.Database, "time": now()}
		switch ev.Kind {
		case eventFailure, eventSuccess:
			payload["job"] = ev.Job
		case eventStale, eventOverdue:
			payload["last_success"] = ev.Since
		case eventConnection, eventDestination:
			payload["down_since"] = ev.Since
			if ev.Detail != "" {
				payload["error"] = ev.Detail
			}
			if ev.Kind == eventDestination {
				payload["destination_id"], payload["destination"] = ev.DestinationID, ev.Target
			}
		case eventDisk:
			payload["free"] = ev.Detail
		case eventBackupSize:
			payload["job"], payload["size"], payload["previous_size"] = ev.Job, ev.Size, ev.PrevSize
		case eventRestore:
			payload["job"], payload["backup"], payload["requested_by"] = ev.Job, ev.Source, ev.Actor
		case eventSecurity:
			payload["ip"] = ev.Target
		}
		err = postJSON(ctx, n.Config["url"], payload, []byte(n.Config["secret"]))
	case "email":
		err = sendMail(ctx, n.Config, subject, body)
	default:
		err = errors.New("unsupported channel type")
	}
	if err != nil {
		// Tokens can be part of URLs and therefore of error messages.
		msg := err.Error()
		for k, v := range n.Config {
			if secretFields[k] && v != "" {
				msg = strings.ReplaceAll(msg, v, "[redacted]")
			}
		}
		return errors.New(msg)
	}
	return nil
}

// postJSON posts v; with a secret, the body is signed in X-DBBM-Signature
// as "sha256=" + hex(HMAC-SHA256(secret, body)).
func postJSON(ctx context.Context, target string, v any, secret []byte) error {
	if !validURL(target, false) {
		return errors.New("invalid notification URL")
	}
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", target, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "db-backup-manager")
	if len(secret) > 0 {
		mac := hmac.New(sha256.New, secret)
		mac.Write(body)
		req.Header.Set("X-DBBM-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}
	res, err := notifyClient.Do(req)
	if err != nil {
		var ue *url.Error
		if errors.As(err, &ue) {
			return ue.Err // the URL may contain a token
		}
		return err
	}
	defer res.Body.Close()
	if res.StatusCode/100 != 2 {
		detail, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return fmt.Errorf("HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(detail)))
	}
	return nil
}

func sendMail(ctx context.Context, cfg map[string]string, subject, body string) error {
	message, from, to, err := mailMessage(cfg, subject, body)
	if err != nil {
		return err
	}
	host := cfg["host"]
	addr := net.JoinHostPort(host, cfg["port"])
	dialer := &net.Dialer{Timeout: 15 * time.Second}
	var conn net.Conn
	tlsConfig := &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
	if cfg["security"] == "tls" {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, tlsConfig)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return err
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(30 * time.Second)
	}
	conn.SetDeadline(deadline)
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		conn.Close()
		return err
	}
	defer c.Close()
	if cfg["security"] == "starttls" {
		if err = c.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("STARTTLS: %w", err)
		}
	}
	if cfg["username"] != "" {
		if err = c.Auth(smtp.PlainAuth("", cfg["username"], cfg["password"], host)); err != nil {
			return fmt.Errorf("SMTP auth: %w", err)
		}
	}
	if err = c.Mail(from.Address); err != nil {
		return err
	}
	for _, a := range to {
		if err = c.Rcpt(a.Address); err != nil {
			return err
		}
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err = w.Write(message); err != nil {
		return errors.Join(err, w.Close())
	}
	if err = w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

// mailMessage validates header values at the send boundary, including stored
// configuration, before opening an SMTP connection. The body stays MIME encoded.
func mailMessage(cfg map[string]string, subject, body string) ([]byte, *mail.Address, []*mail.Address, error) {
	for _, value := range []string{cfg["from"], cfg["to"], subject} {
		if strings.ContainsAny(value, "\r\n\x00") {
			return nil, nil, nil, errors.New("invalid email header")
		}
	}
	from, err := mail.ParseAddress(cfg["from"])
	if err != nil {
		return nil, nil, nil, errors.New("invalid email sender")
	}
	to, err := mail.ParseAddressList(cfg["to"])
	if err != nil || len(to) == 0 || len(to) > 20 {
		return nil, nil, nil, errors.New("invalid email recipients")
	}
	recipients := make([]string, 0, len(to))
	for _, a := range to {
		recipients = append(recipients, a.String())
	}
	var msg bytes.Buffer
	fmt.Fprintf(&msg, "From: %s\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n",
		from.String(), strings.Join(recipients, ", "), mime.QEncoding.Encode("utf-8", subject), time.Now().Format(time.RFC1123Z))
	qp := quotedprintable.NewWriter(&msg)
	if _, err = qp.Write([]byte(strings.ReplaceAll(strings.ReplaceAll(body, "\r\n", "\n"), "\n", "\r\n"))); err != nil {
		return nil, nil, nil, err
	}
	if err = qp.Close(); err != nil {
		return nil, nil, nil, err
	}
	return msg.Bytes(), from, to, nil
}

// ---- stale schedules --------------------------------------------------------

type staleState struct {
	ConnectionID, Database string
	Due                    time.Time // the run that did not happen
	Since                  string    // last successful backup
}

// staleGrace is how long after a due run the job may still appear.
const staleGrace = 15 * time.Minute

// checkStale warns when an enabled schedule was due but created no job at
// all, e.g. because the manager was down or the scheduler stuck. Failed jobs
// are reported by their own notification. Each missed run is reported once.
func (m *Manager) checkStale() {
	m.schedMu.Lock()
	ss, err := m.schedules()
	m.schedMu.Unlock()
	if err != nil {
		m.logger.Print(err)
		return
	}
	nowT := time.Now()
	current := map[string]staleState{}
	for _, s := range ss {
		if !s.Enabled {
			continue
		}
		key := s.ConnectionID + "|" + s.Database
		if _, seen := current[key]; seen {
			continue
		}
		var last string
		query, args := `SELECT COALESCE(MAX(created_at),'') FROM backups WHERE connection_id=? AND source IN ('scheduled','manual')`, []any{s.ConnectionID}
		if s.Database != "*" {
			query, args = query+` AND database_name=?`, append(args, s.Database)
		}
		if err = m.db.QueryRow(query, args...).Scan(&last); err != nil {
			continue
		}
		// Start from the latest of: last backup, last change of the schedule,
		// manager start (missed runs while stopped are caught up instead).
		base := m.started
		for _, v := range []string{last, s.UpdatedAt} {
			if t, e := time.Parse(time.RFC3339Nano, v); e == nil && t.After(base) {
				base = t
			}
		}
		spec, e := cron.ParseStandard("CRON_TZ=" + m.timezone() + " " + s.Cron)
		if e != nil {
			continue
		}
		due := spec.Next(base)
		if due.Add(staleGrace).After(nowT) {
			continue
		}
		var jobs int
		query, args = `SELECT COUNT(*) FROM jobs WHERE connection_id=? AND kind='scheduled' AND created_at>=?`, []any{s.ConnectionID, due.Add(-time.Minute).UTC().Format(time.RFC3339Nano)}
		if s.Database != "*" {
			query, args = query+` AND database_name IN (?,'*')`, append(args, s.Database)
		}
		if err = m.db.QueryRow(query, args...).Scan(&jobs); err != nil || jobs > 0 {
			continue
		}
		current[key] = staleState{ConnectionID: s.ConnectionID, Database: s.Database, Due: due, Since: last}
	}
	m.staleMu.Lock()
	previous := m.stale
	m.stale = current
	m.staleMu.Unlock()
	for key, st := range current {
		if p, ok := previous[key]; ok && p.Due.Equal(st.Due) {
			continue
		}
		m.audit("schedule.stale", st.Database+" @ "+st.Due.UTC().Format(time.RFC3339))
		m.notify(Event{Kind: eventStale, ConnectionID: st.ConnectionID, Database: st.Database, Since: st.Since})
	}
}
