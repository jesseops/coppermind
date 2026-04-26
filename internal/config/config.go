package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config holds all application configuration.
type Config struct {
	// Database path.
	DBPath string `json:"db_path"`

	// Data directory for library files.
	DataDir string `json:"data_dir"`

	// Server bind address.
	Host string `json:"host"`
	Port int    `json:"port"`

	// Logging.
	LogLevel  string `json:"log_level"`
	LogFormat string `json:"log_format"`

	// Session signing secret (hex-encoded). Auto-generated if empty.
	SessionSecret string `json:"session_secret"`

	// Allow unauthenticated guest browsing.
	AllowGuests bool `json:"allow_guests"`

	// SMTP settings for send-to-Kindle.
	SMTPHost string `json:"smtp_host"`
	SMTPPort int    `json:"smtp_port"`
	SMTPUser string `json:"smtp_user"`
	SMTPPass string `json:"smtp_pass"`
	SMTPFrom string `json:"smtp_from"`
}

// Defaults.
const (
	DefaultHost     = "127.0.0.1"
	DefaultPort     = 5000
	DefaultDataDir  = "./data"
	DefaultLogLevel = "info"
	DefaultLogFormat = "text"
	DefaultSMTPPort = 587
)

// Load resolves configuration by loading an optional JSON config file
// and then applying environment variable overrides. Env vars always win.
func Load(configPath string) (*Config, error) {
	cfg := &Config{}

	// 1. Find and load JSON config file.
	path, err := resolveConfigPath(configPath)
	if err != nil {
		return nil, fmt.Errorf("resolve config path: %w", err)
	}
	if path != "" {
		if err := loadFile(path, cfg); err != nil {
			return nil, fmt.Errorf("load config file %s: %w", path, err)
		}
	}

	// 2. Apply environment variable overrides.
	applyEnvOverrides(cfg)

	// 3. Apply defaults.
	applyDefaults(cfg)

	return cfg, nil
}

// resolveConfigPath determines the config file path.
// Priority: explicit flag → COPPERMIND_CONFIG env → ./config.json → ~/.config/coppermind/config.json
func resolveConfigPath(explicit string) (string, error) {
	if explicit != "" {
		if _, err := os.Stat(explicit); err != nil {
			return "", fmt.Errorf("config file not found: %s", explicit)
		}
		return explicit, nil
	}

	if envPath := os.Getenv("COPPERMIND_CONFIG"); envPath != "" {
		if _, err := os.Stat(envPath); err != nil {
			return "", fmt.Errorf("COPPERMIND_CONFIG file not found: %s", envPath)
		}
		return envPath, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	localPath := filepath.Join(cwd, "config.json")
	if _, err := os.Stat(localPath); err == nil {
		return localPath, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", nil // can't find home dir, skip
	}
	homePath := filepath.Join(home, ".config", "coppermind", "config.json")
	if _, err := os.Stat(homePath); err == nil {
		return homePath, nil
	}

	return "", nil // no config file found, use defaults + env
}

func loadFile(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return json.Unmarshal(data, cfg)
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("COPPERMIND_DB"); v != "" {
		cfg.DBPath = v
	}
	if v := os.Getenv("COPPERMIND_DATA_DIR"); v != "" {
		cfg.DataDir = v
	}
	if v := os.Getenv("COPPERMIND_HOST"); v != "" {
		cfg.Host = v
	}
	if v := os.Getenv("COPPERMIND_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Port = port
		}
	}
	if v := os.Getenv("COPPERMIND_LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
	if v := os.Getenv("COPPERMIND_LOG_FORMAT"); v != "" {
		cfg.LogFormat = v
	}
	if v := os.Getenv("COPPERMIND_SESSION_SECRET"); v != "" {
		cfg.SessionSecret = v
	}
	if v := os.Getenv("COPPERMIND_ALLOW_GUESTS"); v != "" {
		cfg.AllowGuests = parseBool(v)
	}
	if v := os.Getenv("COPPERMIND_SMTP_HOST"); v != "" {
		cfg.SMTPHost = v
	}
	if v := os.Getenv("COPPERMIND_SMTP_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.SMTPPort = port
		}
	}
	if v := os.Getenv("COPPERMIND_SMTP_USER"); v != "" {
		cfg.SMTPUser = v
	}
	if v := os.Getenv("COPPERMIND_SMTP_PASS"); v != "" {
		cfg.SMTPPass = v
	}
	if v := os.Getenv("COPPERMIND_SMTP_FROM"); v != "" {
		cfg.SMTPFrom = v
	}
}

func applyDefaults(cfg *Config) {
	if cfg.Host == "" {
		cfg.Host = DefaultHost
	}
	if cfg.Port <= 0 {
		cfg.Port = DefaultPort
	}
	if cfg.DataDir == "" {
		cfg.DataDir = DefaultDataDir
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = DefaultLogLevel
	}
	if cfg.LogFormat == "" {
		cfg.LogFormat = DefaultLogFormat
	}
	if cfg.SMTPPort <= 0 {
		cfg.SMTPPort = DefaultSMTPPort
	}
	if cfg.SessionSecret == "" {
		cfg.SessionSecret = generateSecret()
	}
	if cfg.DBPath == "" {
		cfg.DBPath = filepath.Join(cfg.DataDir, "coppermind.db")
	}
}

func parseBool(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	return v == "true" || v == "1" || v == "yes"
}

func generateSecret() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// Fallback — should never happen.
		return "insecure-default-change-me"
	}
	return hex.EncodeToString(b)
}

// SessionSecretBytes returns the session secret as raw bytes.
func (c *Config) SessionSecretBytes() []byte {
	b, err := hex.DecodeString(c.SessionSecret)
	if err != nil {
		// If it's not valid hex, use it as raw bytes.
		return []byte(c.SessionSecret)
	}
	return b
}

// Addr returns the bind address as "host:port".
func (c *Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// HasSMTP returns true if SMTP is configured for send-to-Kindle.
func (c *Config) HasSMTP() bool {
	return c.SMTPHost != "" && c.SMTPFrom != ""
}
