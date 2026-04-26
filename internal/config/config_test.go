package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	// Clear all env vars.
	for _, key := range []string{
		"COPPERMIND_DB", "COPPERMIND_DATA_DIR", "COPPERMIND_HOST", "COPPERMIND_PORT",
		"COPPERMIND_LOG_LEVEL", "COPPERMIND_LOG_FORMAT", "COPPERMIND_SESSION_SECRET",
		"COPPERMIND_ALLOW_GUESTS", "COPPERMIND_CONFIG",
	} {
		t.Setenv(key, "")
	}

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Host != DefaultHost {
		t.Errorf("Host = %q, want %q", cfg.Host, DefaultHost)
	}
	if cfg.Port != DefaultPort {
		t.Errorf("Port = %d, want %d", cfg.Port, DefaultPort)
	}
	if cfg.DataDir != DefaultDataDir {
		t.Errorf("DataDir = %q, want %q", cfg.DataDir, DefaultDataDir)
	}
	if cfg.LogLevel != DefaultLogLevel {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, DefaultLogLevel)
	}
	if cfg.SessionSecret == "" {
		t.Error("SessionSecret should be auto-generated")
	}
	if cfg.AllowGuests {
		t.Error("AllowGuests should default to false")
	}
}

func TestEnvOverrides(t *testing.T) {
	t.Setenv("COPPERMIND_HOST", "0.0.0.0")
	t.Setenv("COPPERMIND_PORT", "8080")
	t.Setenv("COPPERMIND_DB", "/tmp/test.db")
	t.Setenv("COPPERMIND_ALLOW_GUESTS", "true")
	t.Setenv("COPPERMIND_SESSION_SECRET", "deadbeef")
	t.Setenv("COPPERMIND_CONFIG", "")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Host != "0.0.0.0" {
		t.Errorf("Host = %q, want %q", cfg.Host, "0.0.0.0")
	}
	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want %d", cfg.Port, 8080)
	}
	if cfg.DBPath != "/tmp/test.db" {
		t.Errorf("DBPath = %q, want %q", cfg.DBPath, "/tmp/test.db")
	}
	if !cfg.AllowGuests {
		t.Error("AllowGuests should be true")
	}
	if cfg.SessionSecret != "deadbeef" {
		t.Errorf("SessionSecret = %q, want %q", cfg.SessionSecret, "deadbeef")
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.json")
	err := os.WriteFile(cfgFile, []byte(`{"host":"10.0.0.1","port":9000,"log_level":"debug"}`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	// Clear env.
	t.Setenv("COPPERMIND_HOST", "")
	t.Setenv("COPPERMIND_PORT", "")
	t.Setenv("COPPERMIND_CONFIG", "")

	cfg, err := Load(cfgFile)
	if err != nil {
		t.Fatalf("Load(%s) error: %v", cfgFile, err)
	}
	if cfg.Host != "10.0.0.1" {
		t.Errorf("Host = %q, want %q", cfg.Host, "10.0.0.1")
	}
	if cfg.Port != 9000 {
		t.Errorf("Port = %d, want %d", cfg.Port, 9000)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "debug")
	}
}

func TestEnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.json")
	err := os.WriteFile(cfgFile, []byte(`{"host":"10.0.0.1","port":9000}`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("COPPERMIND_HOST", "0.0.0.0")
	t.Setenv("COPPERMIND_CONFIG", "")

	cfg, err := Load(cfgFile)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	// Env should override file.
	if cfg.Host != "0.0.0.0" {
		t.Errorf("Host = %q, want %q (env should win)", cfg.Host, "0.0.0.0")
	}
	// File value should remain.
	if cfg.Port != 9000 {
		t.Errorf("Port = %d, want %d (from file)", cfg.Port, 9000)
	}
}

func TestAddr(t *testing.T) {
	cfg := &Config{Host: "0.0.0.0", Port: 8080}
	if got := cfg.Addr(); got != "0.0.0.0:8080" {
		t.Errorf("Addr() = %q, want %q", got, "0.0.0.0:8080")
	}
}

func TestHasSMTP(t *testing.T) {
	cfg := &Config{}
	if cfg.HasSMTP() {
		t.Error("HasSMTP() should be false with no config")
	}
	cfg.SMTPHost = "smtp.gmail.com"
	cfg.SMTPFrom = "user@example.com"
	if !cfg.HasSMTP() {
		t.Error("HasSMTP() should be true with host and from")
	}
}
