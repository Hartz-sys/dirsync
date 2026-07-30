package sync

import (
	"testing"
	"time"
)

func TestBuildBackupPath(t *testing.T) {
	now := time.Date(2026, 7, 27, 15, 20, 30, 0, time.UTC)

	got := buildBackupPath("/opt/app", "", now)
	want := "/opt/app.bak.20260727152030"
	if got != want {
		t.Fatalf("empty backup_dir: got %s, want %s", got, want)
	}

	got = buildBackupPath("/opt/app", "/opt/backups", now)
	want = "/opt/backups/app-20260727152030"
	if got != want {
		t.Fatalf("custom backup_dir: got %s, want %s", got, want)
	}
}

func TestShellQuote(t *testing.T) {
	got := shellQuote("/opt/app's")
	want := `'/opt/app'\''s'`
	if got != want {
		t.Fatalf("shellQuote: got %s, want %s", got, want)
	}
}
