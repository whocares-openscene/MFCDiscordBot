package config

import (
	"os"
	"strconv"
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
func Load() *Config {
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
