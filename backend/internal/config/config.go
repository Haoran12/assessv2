package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

type ServerConfig struct {
	Host string
	Port int
}

func (s ServerConfig) Address() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

type DatabaseConfig struct {
	Path                   string
	ForeignKeys            bool
	JournalMode            string
	Synchronous            string
	BusyTimeoutMS          int
	CacheSize              int
	TempStore              string
	MaxOpenConns           int
	MaxIdleConns           int
	ConnMaxLifetimeSeconds int
}

type Config struct {
	Server                    ServerConfig
	Database                  DatabaseConfig
	AccountsDatabasePath      string
	MigrationsDir             string
	BusinessMigrationsDir     string
	AccountsMigrationsDir     string
	JWTSecret                 string
	DefaultPassword           string
	EnforceMustChangePassword bool
}

func Load() Config {
	migrationsRoot := getEnv("ASSESS_MIGRATIONS_DIR", "migrations")
	return Config{
		Server: ServerConfig{
			Host: getEnv("ASSESS_SERVER_HOST", "127.0.0.1"),
			Port: getEnvAsInt("ASSESS_SERVER_PORT", 8080),
		},
		Database: DatabaseConfig{
			Path:                   getEnv("ASSESS_SQLITE_PATH", "./data/assess.db"),
			ForeignKeys:            getEnvAsBool("ASSESS_SQLITE_FOREIGN_KEYS", true),
			JournalMode:            getEnv("ASSESS_SQLITE_JOURNAL_MODE", "WAL"),
			Synchronous:            getEnv("ASSESS_SQLITE_SYNCHRONOUS", "NORMAL"),
			BusyTimeoutMS:          getEnvAsInt("ASSESS_SQLITE_BUSY_TIMEOUT_MS", 5000),
			CacheSize:              getEnvAsInt("ASSESS_SQLITE_CACHE_SIZE", -20000),
			TempStore:              getEnv("ASSESS_SQLITE_TEMP_STORE", "MEMORY"),
			MaxOpenConns:           getEnvAsInt("ASSESS_SQLITE_MAX_OPEN_CONNS", 1),
			MaxIdleConns:           getEnvAsInt("ASSESS_SQLITE_MAX_IDLE_CONNS", 1),
			ConnMaxLifetimeSeconds: getEnvAsInt("ASSESS_SQLITE_CONN_MAX_LIFETIME_SECONDS", 0),
		},
		AccountsDatabasePath:      getEnv("ASSESS_ACCOUNTS_SQLITE_PATH", "./data/accounts/accounts.db"),
		MigrationsDir:             migrationsRoot,
		BusinessMigrationsDir:     getEnv("ASSESS_BUSINESS_MIGRATIONS_DIR", filepath.Join(migrationsRoot, "business")),
		AccountsMigrationsDir:     getEnv("ASSESS_ACCOUNTS_MIGRATIONS_DIR", filepath.Join(migrationsRoot, "accounts")),
		JWTSecret:                 getEnv("ASSESS_JWT_SECRET", "assessv2-dev-secret"),
		DefaultPassword:           getEnv("ASSESS_DEFAULT_PASSWORD", "#AssessV2@Init"),
		EnforceMustChangePassword: getEnvAsBool("ASSESS_ENFORCE_MUST_CHANGE_PASSWORD", false),
	}
}

func getEnv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getEnvAsInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvAsBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
