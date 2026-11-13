package manager

import (
	"strings"
	"testing"
)

func TestTranslateServer(t *testing.T) {
	pg := "Không kết nối được PostgreSQL; kiểm tra host, tài khoản, SSL và mạng từ máy chủ backup"
	cases := []struct{ lang, in, want string }{
		{"en", pg, "Cannot connect to PostgreSQL; check the host, user, SSL and network from the backup server"},
		{"vi", pg, pg},
		{"en", "Lần thử 2 lỗi, sẽ thử lại sau 1m0s: " + pg,
			"Attempt 2 failed, retrying in 1m0s: Cannot connect to PostgreSQL; check the host, user, SSL and network from the backup server"},
		{"en", "pg_dump: error: connection refused", "pg_dump: error: connection refused"},
		{"ja", "Cấu hình MongoDB không hợp lệ", "MongoDB の設定が無効です"},
	}
	for _, c := range cases {
		if got := translateServer(c.lang, c.in); got != c.want {
			t.Errorf("translateServer(%q, %q) = %q, want %q", c.lang, c.in, got, c.want)
		}
	}
}

func TestConnectionAlertUsesChannelLanguage(t *testing.T) {
	m := testManager(t)
	_, body := m.message("en", Event{Kind: eventConnection, Detail: "Không kết nối được PostgreSQL; kiểm tra host, tài khoản, SSL và mạng từ máy chủ backup"})
	if want := "Error: Cannot connect to PostgreSQL"; !strings.Contains(body, want) {
		t.Fatalf("body %q does not contain %q", body, want)
	}
}
