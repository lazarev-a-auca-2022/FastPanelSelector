package config

import "testing"

func TestLoad_Defaults(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPPort != 8080 {
		t.Errorf("HTTPPort = %d, want 8080", cfg.HTTPPort)
	}
	if cfg.PollInterval.String() != "15m0s" {
		t.Errorf("PollInterval = %v, want 15m0s", cfg.PollInterval)
	}
}

func TestLoad_InvalidPort(t *testing.T) {
	t.Setenv("HTTP_PORT", "not-a-port")
	if _, err := Load(); err == nil {
		t.Fatal("expected an error for an invalid HTTP_PORT")
	}
}

func TestLoad_InvalidPollInterval(t *testing.T) {
	t.Setenv("POLL_INTERVAL", "soon")
	if _, err := Load(); err == nil {
		t.Fatal("expected an error for an invalid POLL_INTERVAL")
	}
}

func TestLoad_OverridesFromEnv(t *testing.T) {
	t.Setenv("HTTP_PORT", "9090")
	t.Setenv("DB_PATH", "/tmp/custom.db")
	t.Setenv("POLL_INTERVAL", "5m")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPPort != 9090 {
		t.Errorf("HTTPPort = %d, want 9090", cfg.HTTPPort)
	}
	if cfg.DBPath != "/tmp/custom.db" {
		t.Errorf("DBPath = %q, want /tmp/custom.db", cfg.DBPath)
	}
	if cfg.PollInterval.String() != "5m0s" {
		t.Errorf("PollInterval = %v, want 5m0s", cfg.PollInterval)
	}
}
