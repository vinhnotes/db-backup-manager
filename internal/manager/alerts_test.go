package manager

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDestinationDownAndRecovery(t *testing.T) {
	m := testManager(t)
	var got events
	m.notifyHook = got.add
	if _, err := m.db.Exec(`INSERT INTO destinations(id,name,type,config,remote_path,encryption,recipients,passphrase,retention_days,created_at) VALUES('d1','Drive','drive',?,'db','none','',?,7,?)`, m.sealPassword("[x]\ntype = drive"), m.sealPassword(""), now()); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var fail error = errFake("403 Forbidden")
	old := probeDestination
	probeDestination = func(context.Context, *Manager, Destination) error { mu.Lock(); defer mu.Unlock(); return fail }
	t.Cleanup(func() { probeDestination = old })

	m.checkDestinations()
	m.checkDestinations()
	m.checkDestinations()
	ev := got.all()
	if len(ev) != 1 || ev[0].Kind != eventDestination || ev[0].Recovered || ev[0].Target != "Drive" {
		t.Fatalf("down events: %+v", ev)
	}
	subject, body := m.message("en", ev[0])
	if subject != "☁️ Cloud destination unreachable" || !strings.Contains(body, "Destination: Drive") || !strings.Contains(body, "403 Forbidden") {
		t.Fatalf("message: %s / %s", subject, body)
	}
	var metrics metricWriter
	if err := m.writeMetrics(&metrics); err != nil || !strings.Contains(metrics.String(), `dbbm_destination_up{destination_id="d1",destination="Drive"} 0`) {
		t.Fatalf("metric: %v\n%s", err, metrics.String())
	}
	mu.Lock()
	fail = nil
	mu.Unlock()
	m.checkDestinations()
	if ev = got.all(); len(ev) != 2 || !ev[1].Recovered {
		t.Fatalf("recovery: %+v", ev)
	}
}

func TestDiskAlertWithHysteresis(t *testing.T) {
	m := testManager(t)
	m.cfg.DiskAlertPercent = 10
	var got events
	m.notifyHook = got.add
	free := uint64(50)
	old := diskUsage
	diskUsage = func(string) (uint64, uint64, error) { return free << 30, 1000 << 30, nil }
	t.Cleanup(func() { diskUsage = old })

	m.checkDisk() // 5% free
	m.checkDisk()
	free = 105 // 10.5%: above the threshold, inside the margin
	m.checkDisk()
	ev := got.all()
	if len(ev) != 1 || ev[0].Kind != eventDisk || ev[0].Recovered {
		t.Fatalf("low events: %+v", ev)
	}
	subject, body := m.message("vi", ev[0])
	if subject != "💾 Ổ đĩa backup sắp đầy" || !strings.Contains(body, "dưới 10%") || !strings.Contains(body, "50.0 GiB / 1000.0 GiB (5.0%)") {
		t.Fatalf("message: %s / %s", subject, body)
	}
	free = 200
	m.checkDisk()
	if ev = got.all(); len(ev) != 2 || !ev[1].Recovered {
		t.Fatalf("recovery: %+v", ev)
	}
}

func TestShrunkBackupIsReported(t *testing.T) {
	m := testManager(t)
	m.cfg.BackupShrinkPercent = 50
	var got events
	m.notifyHook = got.add
	add := func(id string, size int64, at time.Time) {
		if err := m.saveBackup(Backup{"", id, "app", size, "x", at.UTC().Format(time.RFC3339Nano), "scheduled", "postgres"}); err != nil {
			t.Fatal(err)
		}
	}
	base := time.Now().Add(-time.Hour)
	add("b1", 10<<20, base)
	add("b2", 6<<20, base.Add(time.Minute)) // -40%: fine
	m.checkBackupSize(Job{Kind: "scheduled"}, "b2")
	add("b3", 2<<20, base.Add(2*time.Minute)) // -67%
	m.checkBackupSize(Job{Kind: "scheduled"}, "b3")
	ev := got.all()
	if len(ev) != 1 || ev[0].Kind != eventBackupSize || ev[0].Size != 2<<20 || ev[0].PrevSize != 6<<20 {
		t.Fatalf("events: %+v", ev)
	}
	if _, body := m.message("en", ev[0]); !strings.Contains(body, "Size: 2.0 MiB") || !strings.Contains(body, "Previous: 6.0 MiB") {
		t.Fatalf("message: %s", body)
	}
}

func TestOverdueBackupIsReportedOnce(t *testing.T) {
	m := testManager(t)
	m.cfg.BackupMaxAge = 24 * time.Hour
	var got events
	m.notifyHook = got.add
	old := time.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339Nano)
	if _, err := m.db.Exec(`INSERT INTO schedules(connection_id,database_name,cron,retention_days,enabled,updated_at) VALUES('','app','0 2 * * *',7,1,?)`, old); err != nil {
		t.Fatal(err)
	}
	m.saveBackup(Backup{"", "b1", "app", 1, "x", old, "scheduled", "postgres"})
	m.checkOverdue()
	m.checkOverdue()
	ev := got.all()
	if len(ev) != 1 || ev[0].Kind != eventOverdue || ev[0].Database != "app" || ev[0].Detail != "24h1m" { // daily + 1m JobTimeout
		t.Fatalf("events: %+v", ev)
	}
	if subject, body := m.message("vi", ev[0]); subject != "⏰ Lâu không có backup thành công" || !strings.Contains(body, "Đã quá 24h1m") {
		t.Fatalf("message: %s / %s", subject, body)
	}
	// A fresh backup clears the state; the next outage is reported again.
	m.saveBackup(Backup{"", "b2", "app", 1, "x", now(), "scheduled", "postgres"})
	m.checkOverdue()
	m.overdueMu.Lock()
	n := len(m.overdue)
	m.overdueMu.Unlock()
	if n != 0 {
		t.Fatal("overdue state not cleared")
	}
}

func TestWeeklyScheduleIsNotOverdueAfterOneDay(t *testing.T) {
	m := testManager(t)
	m.cfg.BackupMaxAge = 24 * time.Hour
	var got events
	m.notifyHook = got.add
	threeDays := time.Now().Add(-72 * time.Hour).UTC().Format(time.RFC3339Nano)
	m.db.Exec(`INSERT INTO schedules(connection_id,database_name,cron,retention_days,enabled,updated_at) VALUES('','app','0 2 * * 0',7,1,?)`, threeDays)
	m.saveBackup(Backup{"", "b1", "app", 1, "x", threeDays, "scheduled", "postgres"})
	m.checkOverdue()
	if ev := got.all(); len(ev) != 0 {
		t.Fatalf("weekly schedule reported: %+v", ev)
	}
}

func TestLoginLockoutIsReportedOnce(t *testing.T) {
	m := testManager(t)
	var got events
	m.notifyHook = got.add
	for range loginLimit + 2 {
		request(m, "POST", "/api/login", `{"username":"admin","password":"wrong-password"}`, nil)
	}
	ev := got.all()
	if len(ev) != 1 || ev[0].Kind != eventSecurity || ev[0].Target == "" {
		t.Fatalf("events: %+v", ev)
	}
	if _, body := m.message("en", ev[0]); !strings.Contains(body, "10 times") || !strings.Contains(body, "IP address: "+ev[0].Target) {
		t.Fatalf("message: %s", body)
	}
}

func TestShortDuration(t *testing.T) {
	for d, want := range map[time.Duration]string{24 * time.Hour: "24h", 30 * time.Minute: "30m", 90 * time.Minute: "1h30m", 336 * time.Hour: "336h"} {
		if got := shortDuration(d); got != want {
			t.Errorf("%v: %q, want %q", d, got, want)
		}
	}
}
