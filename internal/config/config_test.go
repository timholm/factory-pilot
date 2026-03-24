package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	// Clear all env vars that could affect config
	envVars := []string{
		"POSTGRES_URL", "FACTORY_DATA_DIR", "FACTORY_GIT_DIR",
		"GITHUB_TOKEN", "GITHUB_USER", "CLAUDE_BINARY",
		"K8S_NAMESPACE", "CYCLE_INTERVAL", "MAX_FIXES_PER_CYCLE", "API_PORT",
	}
	for _, k := range envVars {
		os.Unsetenv(k)
	}

	cfg := Load()

	if cfg.PostgresURL != "postgres://localhost:5432/factory?sslmode=disable" {
		t.Errorf("PostgresURL = %q, want default", cfg.PostgresURL)
	}
	if cfg.FactoryDataDir != "/data/factory" {
		t.Errorf("FactoryDataDir = %q, want /data/factory", cfg.FactoryDataDir)
	}
	if cfg.FactoryGitDir != "/data/repos" {
		t.Errorf("FactoryGitDir = %q, want /data/repos", cfg.FactoryGitDir)
	}
	if cfg.GithubToken != "" {
		t.Errorf("GithubToken = %q, want empty", cfg.GithubToken)
	}
	if cfg.GithubUser != "timholm" {
		t.Errorf("GithubUser = %q, want timholm", cfg.GithubUser)
	}
	if cfg.ClaudeBinary != "claude" {
		t.Errorf("ClaudeBinary = %q, want claude", cfg.ClaudeBinary)
	}
	if cfg.K8sNamespace != "factory" {
		t.Errorf("K8sNamespace = %q, want factory", cfg.K8sNamespace)
	}
	if cfg.CycleInterval != 6*time.Hour {
		t.Errorf("CycleInterval = %v, want 6h", cfg.CycleInterval)
	}
	if cfg.MaxFixes != 10 {
		t.Errorf("MaxFixes = %d, want 10", cfg.MaxFixes)
	}
	if !cfg.DryRun {
		t.Error("DryRun should default to true")
	}
	if cfg.APIPort != "8090" {
		t.Errorf("APIPort = %q, want 8090", cfg.APIPort)
	}
}

func TestLoad_EnvOverrides(t *testing.T) {
	os.Setenv("POSTGRES_URL", "postgres://db:5432/test")
	os.Setenv("FACTORY_DATA_DIR", "/tmp/data")
	os.Setenv("FACTORY_GIT_DIR", "/tmp/repos")
	os.Setenv("GITHUB_TOKEN", "ghp_test123")
	os.Setenv("GITHUB_USER", "testuser")
	os.Setenv("CLAUDE_BINARY", "/usr/bin/claude")
	os.Setenv("K8S_NAMESPACE", "staging")
	os.Setenv("CYCLE_INTERVAL", "30m")
	os.Setenv("MAX_FIXES_PER_CYCLE", "5")
	os.Setenv("API_PORT", "9090")

	defer func() {
		for _, k := range []string{
			"POSTGRES_URL", "FACTORY_DATA_DIR", "FACTORY_GIT_DIR",
			"GITHUB_TOKEN", "GITHUB_USER", "CLAUDE_BINARY",
			"K8S_NAMESPACE", "CYCLE_INTERVAL", "MAX_FIXES_PER_CYCLE", "API_PORT",
		} {
			os.Unsetenv(k)
		}
	}()

	cfg := Load()

	if cfg.PostgresURL != "postgres://db:5432/test" {
		t.Errorf("PostgresURL = %q, want postgres://db:5432/test", cfg.PostgresURL)
	}
	if cfg.FactoryDataDir != "/tmp/data" {
		t.Errorf("FactoryDataDir = %q, want /tmp/data", cfg.FactoryDataDir)
	}
	if cfg.FactoryGitDir != "/tmp/repos" {
		t.Errorf("FactoryGitDir = %q, want /tmp/repos", cfg.FactoryGitDir)
	}
	if cfg.GithubToken != "ghp_test123" {
		t.Errorf("GithubToken = %q, want ghp_test123", cfg.GithubToken)
	}
	if cfg.GithubUser != "testuser" {
		t.Errorf("GithubUser = %q, want testuser", cfg.GithubUser)
	}
	if cfg.ClaudeBinary != "/usr/bin/claude" {
		t.Errorf("ClaudeBinary = %q, want /usr/bin/claude", cfg.ClaudeBinary)
	}
	if cfg.K8sNamespace != "staging" {
		t.Errorf("K8sNamespace = %q, want staging", cfg.K8sNamespace)
	}
	if cfg.CycleInterval != 30*time.Minute {
		t.Errorf("CycleInterval = %v, want 30m", cfg.CycleInterval)
	}
	if cfg.MaxFixes != 5 {
		t.Errorf("MaxFixes = %d, want 5", cfg.MaxFixes)
	}
	if cfg.APIPort != "9090" {
		t.Errorf("APIPort = %q, want 9090", cfg.APIPort)
	}
}

func TestEnvOr(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		envVal   string
		fallback string
		want     string
	}{
		{"env set", "TEST_ENV_OR_1", "fromenv", "default", "fromenv"},
		{"env empty", "TEST_ENV_OR_2", "", "default", "default"},
		{"env unset", "TEST_ENV_OR_3_UNSET", "", "fallback", "fallback"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVal != "" {
				os.Setenv(tt.key, tt.envVal)
				defer os.Unsetenv(tt.key)
			} else {
				os.Unsetenv(tt.key)
			}
			got := envOr(tt.key, tt.fallback)
			if got != tt.want {
				t.Errorf("envOr(%q, %q) = %q, want %q", tt.key, tt.fallback, got, tt.want)
			}
		})
	}
}

func TestEnvInt(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		envVal   string
		fallback int
		want     int
	}{
		{"valid int", "TEST_ENV_INT_1", "42", 10, 42},
		{"invalid int", "TEST_ENV_INT_2", "notanumber", 10, 10},
		{"empty", "TEST_ENV_INT_3", "", 10, 10},
		{"zero", "TEST_ENV_INT_4", "0", 10, 0},
		{"negative", "TEST_ENV_INT_5", "-3", 10, -3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVal != "" {
				os.Setenv(tt.key, tt.envVal)
				defer os.Unsetenv(tt.key)
			} else {
				os.Unsetenv(tt.key)
			}
			got := envInt(tt.key, tt.fallback)
			if got != tt.want {
				t.Errorf("envInt(%q, %d) = %d, want %d", tt.key, tt.fallback, got, tt.want)
			}
		})
	}
}

func TestEnvDuration(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		envVal   string
		fallback time.Duration
		want     time.Duration
	}{
		{"valid duration", "TEST_ENV_DUR_1", "30m", time.Hour, 30 * time.Minute},
		{"invalid duration", "TEST_ENV_DUR_2", "invalid", time.Hour, time.Hour},
		{"empty", "TEST_ENV_DUR_3", "", time.Hour, time.Hour},
		{"seconds", "TEST_ENV_DUR_4", "90s", time.Hour, 90 * time.Second},
		{"hours", "TEST_ENV_DUR_5", "2h", time.Hour, 2 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVal != "" {
				os.Setenv(tt.key, tt.envVal)
				defer os.Unsetenv(tt.key)
			} else {
				os.Unsetenv(tt.key)
			}
			got := envDuration(tt.key, tt.fallback)
			if got != tt.want {
				t.Errorf("envDuration(%q, %v) = %v, want %v", tt.key, tt.fallback, got, tt.want)
			}
		})
	}
}
