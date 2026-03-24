package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for factory-pilot.
type Config struct {
	PostgresURL    string
	FactoryDataDir string
	FactoryGitDir  string
	GithubToken    string
	GithubUser     string
	ClaudeBinary   string
	K8sNamespace   string
	CycleInterval  time.Duration
	MaxFixes       int
	DryRun         bool
	APIPort        string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	return &Config{
		PostgresURL:    envOr("POSTGRES_URL", "postgres://localhost:5432/factory?sslmode=disable"),
		FactoryDataDir: envOr("FACTORY_DATA_DIR", "/data/factory"),
		FactoryGitDir:  envOr("FACTORY_GIT_DIR", "/data/repos"),
		GithubToken:    os.Getenv("GITHUB_TOKEN"),
		GithubUser:     envOr("GITHUB_USER", "timholm"),
		ClaudeBinary:   envOr("CLAUDE_BINARY", "claude"),
		K8sNamespace:   envOr("K8S_NAMESPACE", "factory"),
		CycleInterval:  envDuration("CYCLE_INTERVAL", 6*time.Hour),
		MaxFixes:       envInt("MAX_FIXES_PER_CYCLE", 10),
		DryRun:         true, // safe by default, override with --execute
		APIPort:        envOr("API_PORT", "8090"),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil {
			return n
		}
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		d, err := time.ParseDuration(v)
		if err == nil {
			return d
		}
	}
	return fallback
}
