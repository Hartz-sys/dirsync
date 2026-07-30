package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDisplayNameFromPath(t *testing.T) {
	cases := map[string]string{
		"/tmp/config.yaml":     "default",
		"/tmp/config.dev.yaml": "dev",
		"/tmp/prod.sync.yaml":  "prod",
		"/tmp/custom.yaml":     "custom",
	}
	for path, want := range cases {
		if got := displayNameFromPath(path); got != want {
			t.Fatalf("%s: got %s, want %s", path, got, want)
		}
	}
}

func TestListCandidatesFindsConfig(t *testing.T) {
	dir := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	content := []byte("name: demo\nhost: 1.2.3.4\nport: 22\nuser: root\nlocal_dir: /tmp\nremote_dir: /opt/app\n")
	if err := os.WriteFile(filepath.Join(dir, "config.dev.yaml"), content, 0o600); err != nil {
		t.Fatal(err)
	}

	items, err := ListCandidates()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(items))
	}
	if items[0].Name != "demo" {
		t.Fatalf("unexpected name: %s", items[0].Name)
	}
	if items[0].Host != "1.2.3.4" {
		t.Fatalf("unexpected host: %s", items[0].Host)
	}
}
