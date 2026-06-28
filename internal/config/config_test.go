package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeTemp(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadDefaults(t *testing.T) {
	path := writeTemp(t, `
endpoints:
  - url: https://example.com
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Timeout.Duration != 5*time.Second {
		t.Errorf("timeout default = %v, want 5s", cfg.Timeout.Duration)
	}
	ep := cfg.Endpoints[0]
	if ep.Name != ep.URL {
		t.Errorf("name should default to url, got %q", ep.Name)
	}
	if ep.HTTPMethod() != "GET" {
		t.Errorf("method default = %q, want GET", ep.HTTPMethod())
	}
	if ep.WantStatus() != 200 {
		t.Errorf("expect_status default = %d, want 200", ep.WantStatus())
	}
}

func TestLoadParsesValues(t *testing.T) {
	path := writeTemp(t, `
timeout: 2s
interval: 30s
endpoints:
  - name: api
    url: https://api.example.com/health
    method: POST
    expect_status: 204
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Timeout.Duration != 2*time.Second {
		t.Errorf("timeout = %v, want 2s", cfg.Timeout.Duration)
	}
	ep := cfg.Endpoints[0]
	if ep.HTTPMethod() != "POST" || ep.WantStatus() != 204 {
		t.Errorf("got method=%s status=%d", ep.HTTPMethod(), ep.WantStatus())
	}
}

func TestLoadRejectsEmpty(t *testing.T) {
	path := writeTemp(t, "timeout: 5s\n")
	if _, err := Load(path); err == nil {
		t.Error("expected an error when no endpoints are defined")
	}
}

func TestLoadBadDuration(t *testing.T) {
	path := writeTemp(t, `
timeout: notaduration
endpoints:
  - url: https://example.com
`)
	if _, err := Load(path); err == nil {
		t.Error("expected an error for an invalid duration")
	}
}
