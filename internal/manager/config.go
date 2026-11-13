package manager

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds process settings from env. The PG*/Restore* fields are never
// read from env: database connections are managed in the web UI and copied
// into a per-connection Config by forConnection.
type Config struct {
	Addr, WebDir, BackupDir, MetadataDB, AdminUser, AdminPassword, ViewerPassword string
	PGHost, PGPort, PGUser, PGPassword, PGPassFile, PGDatabase, PGSSLMode         string
	RestoreUser, RestorePassword, RestorePassFile, Timezone                       string
	// Engine is the database type of the connection (see engines); ServerDir
	// and LocalDir are the SQL Server shared backup directory as seen by the
	// database server and by this container.
	Engine, ServerDir, LocalDir                                          string
	RcloneBinary                                                         string
	GoogleOAuthClientID, GoogleOAuthClientSecret, GoogleOAuthRedirectURL string
	JobTimeout                                                           time.Duration
	MaxUpload                                                            int64
	SecureCookie                                                         bool
	// TLSMode is off, self-signed or file (see tls.go). TLSHosts adds names
	// or IPs to the self-signed certificate.
	TLSMode, TLSCertFile, TLSKeyFile, TLSHosts string
	// Retries is how many times a failed backup or cloud upload is retried,
	// waiting RetryDelay, then twice as long, and so on.
	Retries    int
	RetryDelay time.Duration
	// CatchUp runs a schedule once at startup when its run was missed while
	// the manager was stopped.
	CatchUp bool
	// MetricsToken enables GET /metrics for Prometheus (Bearer token).
	MetricsToken string
	// Alert thresholds; 0 turns the alert off. DiskAlertPercent: free space
	// of BACKUP_DIR; BackupShrinkPercent: drop from the previous backup of
	// the same database; BackupMaxAge: time without a successful backup.
	DiskAlertPercent    float64
	BackupShrinkPercent float64
	BackupMaxAge        time.Duration
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

// MetadataDBPath is where the SQLite metadata lives (METADATA_DB).
func MetadataDBPath() string { return env("METADATA_DB", "data/metadata.db") }

// TLSSettings reads TLS_MODE, TLS_CERT_FILE, TLS_KEY_FILE and TLS_HOSTS.
// TLS_MODE defaults to file when a certificate is given, otherwise off.
func TLSSettings() (mode, certFile, keyFile, hosts string) {
	certFile, keyFile, hosts = os.Getenv("TLS_CERT_FILE"), os.Getenv("TLS_KEY_FILE"), os.Getenv("TLS_HOSTS")
	mode = TLSOff
	if certFile != "" || keyFile != "" {
		mode = TLSFile
	}
	return env("TLS_MODE", mode), certFile, keyFile, hosts
}

func LoadConfig() (Config, error) {
	c := Config{Addr: env("LISTEN_ADDR", "127.0.0.1:8080"), WebDir: env("WEB_DIR", "web/dist"), BackupDir: env("BACKUP_DIR", "backups"), MetadataDB: MetadataDBPath(), AdminUser: env("ADMIN_USER", "admin"), AdminPassword: os.Getenv("ADMIN_PASSWORD"), ViewerPassword: os.Getenv("VIEWER_PASSWORD"), Timezone: env("TZ", "Asia/Ho_Chi_Minh"), RcloneBinary: env("RCLONE_BINARY", "rclone")}
	c.GoogleOAuthClientID = os.Getenv("GOOGLE_OAUTH_CLIENT_ID")
	c.GoogleOAuthClientSecret = os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET")
	c.GoogleOAuthRedirectURL = os.Getenv("GOOGLE_OAUTH_REDIRECT_URL")
	if len(c.AdminPassword) < 12 || len(c.AdminPassword) > 72 {
		return c, fmt.Errorf("ADMIN_PASSWORD must contain 12–72 bytes")
	}
	if c.ViewerPassword != "" && (len(c.ViewerPassword) < 12 || len(c.ViewerPassword) > 72 || c.ViewerPassword == c.AdminPassword || c.AdminUser == "viewer") {
		return c, fmt.Errorf("viewer credentials must be distinct and contain 12–72 bytes")
	}
	c.TLSMode, c.TLSCertFile, c.TLSKeyFile, c.TLSHosts = TLSSettings()
	if c.TLSMode == TLSFile && (c.TLSCertFile == "" || c.TLSKeyFile == "") {
		return c, fmt.Errorf("TLS_MODE=file needs TLS_CERT_FILE and TLS_KEY_FILE")
	}
	if c.TLSMode != TLSOff && c.TLSMode != TLSSelfSigned && c.TLSMode != TLSFile {
		return c, fmt.Errorf("TLS_MODE must be off, self-signed or file")
	}
	// Browsers drop Secure cookies over plain HTTP, so it follows TLS unless
	// set explicitly (e.g. true behind an HTTPS reverse proxy).
	c.SecureCookie = env("COOKIE_SECURE", fmt.Sprint(c.TLSMode != TLSOff)) == "true"
	var err error
	c.JobTimeout, err = time.ParseDuration(env("JOB_TIMEOUT", "2h"))
	if err != nil || c.JobTimeout <= 0 {
		return c, fmt.Errorf("invalid JOB_TIMEOUT")
	}
	c.MaxUpload, err = strconv.ParseInt(env("MAX_UPLOAD_BYTES", "10737418240"), 10, 64)
	if err != nil || c.MaxUpload < 1 {
		return c, fmt.Errorf("invalid MAX_UPLOAD_BYTES")
	}
	c.Retries, err = strconv.Atoi(env("JOB_RETRIES", "2"))
	if err != nil || c.Retries < 0 || c.Retries > 10 {
		return c, fmt.Errorf("JOB_RETRIES must be between 0 and 10")
	}
	c.RetryDelay, err = time.ParseDuration(env("JOB_RETRY_DELAY", "2m"))
	if err != nil || c.RetryDelay < time.Second || c.RetryDelay > 24*time.Hour {
		return c, fmt.Errorf("invalid JOB_RETRY_DELAY")
	}
	c.CatchUp = env("CATCH_UP_MISSED", "true") == "true"
	c.MetricsToken = os.Getenv("METRICS_TOKEN")
	if c.MetricsToken != "" && len(c.MetricsToken) < 16 {
		return c, fmt.Errorf("METRICS_TOKEN must contain at least 16 characters")
	}
	c.DiskAlertPercent, err = strconv.ParseFloat(env("DISK_ALERT_PERCENT", "10"), 64)
	if err != nil || c.DiskAlertPercent < 0 || c.DiskAlertPercent >= 100 {
		return c, fmt.Errorf("DISK_ALERT_PERCENT must be between 0 and 99")
	}
	c.BackupShrinkPercent, err = strconv.ParseFloat(env("BACKUP_SHRINK_PERCENT", "50"), 64)
	if err != nil || c.BackupShrinkPercent < 0 || c.BackupShrinkPercent >= 100 {
		return c, fmt.Errorf("BACKUP_SHRINK_PERCENT must be between 0 and 99")
	}
	c.BackupMaxAge, err = time.ParseDuration(env("BACKUP_MAX_AGE", "24h"))
	if err != nil || c.BackupMaxAge < 0 {
		return c, fmt.Errorf("invalid BACKUP_MAX_AGE")
	}
	if _, err = time.LoadLocation(c.Timezone); err != nil {
		return c, err
	}
	return c, nil
}
