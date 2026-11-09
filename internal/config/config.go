package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all configuration for the combined MFC Discord bot.
type Config struct {
	// Discord
	Token    string
	ClientID string

	// MFC adapter
	ListenAddress          string
	TimeoutSeconds         int
	MaxSnapshotSize        int
	WSConnectTimeout       time.Duration
	WSIdleTimeout          time.Duration
	BulkArrivalTimeout     time.Duration
	ReconnectBackoffMax    time.Duration
	ExtDataFetchTimeout    time.Duration
	LookupTTL              time.Duration
	NameCacheTTL           time.Duration
	SnapshotCountsLogEvery time.Duration

	// Online check interval
	OnlineCheckInterval time.Duration

	// Database
	DatabasePath string

	// Debug / Trace
	Debug bool
	Trace bool
}

// Load reads configuration from environment variables with sensible defaults.
// It first loads a local .env file when present, matching the old Node bot and
// the Docker Compose deployment. Real environment variables always win over
// values from the file.
func Load() *Config {
	loadDotEnv(".env")
	return &Config{
		Token:                  getEnv("TOKEN", ""),
		ClientID:               getEnv("CLIENTID", ""),
		ListenAddress:          getEnv("LISTEN_ADDRESS", "0.0.0.0:80"),
		TimeoutSeconds:         getEnvInt("TIMEOUT_SECONDS", 10),
		MaxSnapshotSize:        getEnvInt("MAX_SNAPSHOT_SIZE", 20000),
		WSConnectTimeout:       getEnvDuration("WS_CONNECT_TIMEOUT", 30*time.Second),
		WSIdleTimeout:          getEnvDuration("WS_IDLE_TIMEOUT", 30*time.Second),
		BulkArrivalTimeout:     getEnvDuration("BULK_ARRIVAL_TIMEOUT", 60*time.Second),
		ReconnectBackoffMax:    getEnvDuration("RECONNECT_BACKOFF_MAX", time.Minute),
		ExtDataFetchTimeout:    getEnvDuration("EXT_DATA_FETCH_TIMEOUT", 30*time.Second),
		LookupTTL:              getEnvDuration("LOOKUP_TTL", 5*time.Minute),
		NameCacheTTL:           getEnvDuration("NAME_CACHE_TTL", 24*time.Hour),
		SnapshotCountsLogEvery: getEnvDuration("SNAPSHOT_COUNTS_LOG_EVERY", 10*time.Minute),
		OnlineCheckInterval:    getEnvDuration("ONLINE_CHECK_INTERVAL", 2*time.Minute),
		DatabasePath:           getEnv("DATABASE_PATH", "mfcbot.db"),
		Debug:                  getEnvBool("DEBUG", false),
		Trace:                  getEnvBool("TRACE", false),
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getEnvDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func getEnvBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		b, err := strconv.ParseBool(v)
		if err == nil {
			return b
		}
	}
	return def
}

// loadDotEnv reads KEY=VALUE pairs from path into the process environment.
// Missing files are ignored. Existing environment variables are never
// overwritten, so `TOKEN=... ./mfcbot` still takes precedence over .env.
// Blank lines, comments and an optional `export ` prefix are supported, and
// single or double quotes around the value are stripped.
func loadDotEnv(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 {
			first, last := value[0], value[len(value)-1]
			if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
				value = value[1 : len(value)-1]
			}
		}
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, value)
		}
	}
}
