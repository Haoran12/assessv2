package config

import (
	"fmt"
	"os"
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
	Path string
}

type Config struct {
	Server    ServerConfig
	Database  DatabaseConfig
	JWTSecret string
}

func Load() Config {
	return Config{
		Server: ServerConfig{
			Host: getEnv("ASSESS_SERVER_HOST", "127.0.0.1"),
			Port: getEnvAsInt("ASSESS_SERVER_PORT", 8080),
		},
		Database: DatabaseConfig{
			Path: getEnv("ASSESS_SQLITE_PATH", "./data/assess.db"),
		},
		JWTSecret: getEnv("ASSESS_JWT_SECRET", "assessv2-dev-secret"),
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
