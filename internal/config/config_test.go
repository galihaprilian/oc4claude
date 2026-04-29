package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Listen != "127.0.0.1:8080" {
		t.Errorf("expected Listen '127.0.0.1:8080', got '%s'", cfg.Listen)
	}
	if cfg.UpstreamURL != "https://api.opencode.ai/v1" {
		t.Errorf("expected UpstreamURL 'https://api.opencode.ai/v1', got '%s'", cfg.UpstreamURL)
	}
	if cfg.TokenLimit != 180000 {
		t.Errorf("expected TokenLimit 180000, got %d", cfg.TokenLimit)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("expected LogLevel 'info', got '%s'", cfg.LogLevel)
	}
	if !cfg.CircuitBreaker.Enabled {
		t.Error("expected CircuitBreaker.Enabled to be true")
	}
	if cfg.CircuitBreaker.FailureThreshold != 3 {
		t.Errorf("expected FailureThreshold 3, got %d", cfg.CircuitBreaker.FailureThreshold)
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			cfg: &Config{
				Listen:      "127.0.0.1:8080",
				UpstreamURL: "https://api.opencode.ai/v1",
				APIKey:      "test-key",
				DefaultModel: "anthropic/claude-3.5-sonnet",
				TokenLimit:  180000,
				LogLevel:    "info",
				CircuitBreaker: CircuitBreakerConfig{
					Enabled:                true,
					FailureThreshold:       3,
					RecoveryTimeoutSeconds: 60,
				},
			},
			wantErr: false,
		},
		{
			name: "missing listen",
			cfg: &Config{
				UpstreamURL: "https://api.opencode.ai/v1",
				APIKey:      "test-key",
				DefaultModel: "anthropic/claude-3.5-sonnet",
				TokenLimit:  180000,
				CircuitBreaker: CircuitBreakerConfig{
					Enabled:                true,
					FailureThreshold:       3,
					RecoveryTimeoutSeconds: 60,
				},
			},
			wantErr: true,
			errMsg:  "listen address is required",
		},
		{
			name: "missing api_key",
			cfg: &Config{
				Listen:      "127.0.0.1:8080",
				UpstreamURL: "https://api.opencode.ai/v1",
				DefaultModel: "anthropic/claude-3.5-sonnet",
				TokenLimit:  180000,
				CircuitBreaker: CircuitBreakerConfig{
					Enabled:                true,
					FailureThreshold:       3,
					RecoveryTimeoutSeconds: 60,
				},
			},
			wantErr: true,
			errMsg:  "api_key is required",
		},
		{
			name: "invalid token_limit",
			cfg: &Config{
				Listen:      "127.0.0.1:8080",
				UpstreamURL: "https://api.opencode.ai/v1",
				APIKey:      "test-key",
				DefaultModel: "anthropic/claude-3.5-sonnet",
				TokenLimit:  0,
				CircuitBreaker: CircuitBreakerConfig{
					Enabled:                true,
					FailureThreshold:       3,
					RecoveryTimeoutSeconds: 60,
				},
			},
			wantErr: true,
			errMsg:  "token_limit must be positive",
		},
		{
			name: "invalid log_level",
			cfg: &Config{
				Listen:      "127.0.0.1:8080",
				UpstreamURL: "https://api.opencode.ai/v1",
				APIKey:      "test-key",
				DefaultModel: "anthropic/claude-3.5-sonnet",
				TokenLimit:  180000,
				LogLevel:    "invalid",
				CircuitBreaker: CircuitBreakerConfig{
					Enabled:                true,
					FailureThreshold:       3,
					RecoveryTimeoutSeconds: 60,
				},
			},
			wantErr: true,
			errMsg:  "invalid log_level: invalid (allowed: debug, info, warn, error)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error '%s', got nil", tt.errMsg)
				} else if err.Error() != tt.errMsg {
					t.Errorf("expected error '%s', got '%s'", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestEnvVarInterpolation(t *testing.T) {
	os.Setenv("TEST_VAR", "test-value")
	defer os.Unsetenv("TEST_VAR")

	os.Setenv("TEST_DEFAULT", "")
	defer os.Unsetenv("TEST_DEFAULT")

	content := `{"key": "${TEST_VAR}", "defaulted": "${UNSET_VAR:-fallback}"}`
	result, err := interpolateEnvVars(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := `{"key": "test-value", "defaulted": "fallback"}`
	if result != expected {
		t.Errorf("expected '%s', got '%s'", expected, result)
	}
}

func TestLoad(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	cfg := DefaultConfig()
	cfg.APIKey = "test-key"
	cfg.DefaultModel = "anthropic/claude-3.5-sonnet"

	data, err := cfg.ToJSON()
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}

	if err := os.WriteFile(cfgPath, []byte(data), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	loaded, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if loaded.APIKey != "test-key" {
		t.Errorf("expected APIKey 'test-key', got '%s'", loaded.APIKey)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load("/nonexistent/config.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestDefaultConfigDir(t *testing.T) {
	dir, err := DefaultConfigDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	usr, _ := os.UserHomeDir()
	expected := filepath.Join(usr, ".oc4claude")
	if dir != expected {
		t.Errorf("expected '%s', got '%s'", expected, dir)
	}
}

func TestDefaultConfigPath(t *testing.T) {
	path, err := DefaultConfigPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	usr, _ := os.UserHomeDir()
	expected := filepath.Join(usr, ".oc4claude", "config.json")
	if path != expected {
		t.Errorf("expected '%s', got '%s'", expected, path)
	}
}

func TestEnsureConfigFile(t *testing.T) {
	tmpDir := t.TempDir()

	originalHome := os.Getenv("HOME")
	tmpHome := filepath.Join(tmpDir, "home")
	os.MkdirAll(tmpHome, 0755)
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", originalHome)

	err := EnsureConfigFile()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfgPath, _ := DefaultConfigPath()
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		t.Error("config file should exist")
	}
}

func TestGetModelForContext(t *testing.T) {
	cfg := DefaultConfig()

	if model := cfg.GetModelForContext("thinking"); model != "anthropic/claude-3.5-haiku" {
		t.Errorf("expected 'anthropic/claude-3.5-haiku', got '%s'", model)
	}

	if model := cfg.GetModelForContext("nonexistent"); model != cfg.DefaultModel {
		t.Errorf("expected default model '%s', got '%s'", cfg.DefaultModel, model)
	}
}

func TestFallbackChainOperations(t *testing.T) {
	cfg := DefaultConfig()

	if !cfg.HasFallbackChain() {
		t.Error("expected HasFallbackChain to return true")
	}

	cfg.AddToFallbackChain("new-model")
	found := false
	for _, m := range cfg.FallbackChain {
		if m == "new-model" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'new-model' in fallback chain")
	}

	initialLen := len(cfg.FallbackChain)
	cfg.AddToFallbackChain(cfg.FallbackChain[0])
	if len(cfg.FallbackChain) != initialLen {
		t.Error("AddToFallbackChain should not add duplicates")
	}

	cfg.RemoveFromFallbackChain("new-model")
	found = false
	for _, m := range cfg.FallbackChain {
		if m == "new-model" {
			found = true
			break
		}
	}
	if found {
		t.Error("expected 'new-model' removed from fallback chain")
	}
}

func TestApplyEnvOverrides(t *testing.T) {
	cfg := DefaultConfig()
	originalListen := cfg.Listen

	os.Setenv("OC4CLAUDE_LISTEN", "0.0.0.0:9090")
	os.Setenv("OC4CLAUDE_UPSTREAM", "https://custom.api.com/v1")
	os.Setenv("OC4CLAUDE_LOG_LEVEL", "debug")
	defer func() {
		os.Unsetenv("OC4CLAUDE_LISTEN")
		os.Unsetenv("OC4CLAUDE_UPSTREAM")
		os.Unsetenv("OC4CLAUDE_LOG_LEVEL")
	}()

	cfg.ApplyEnvOverrides()

	if cfg.Listen != "0.0.0.0:9090" {
		t.Errorf("expected Listen '0.0.0.0:9090', got '%s'", cfg.Listen)
	}
	if cfg.UpstreamURL != "https://custom.api.com/v1" {
		t.Errorf("expected UpstreamURL 'https://custom.api.com/v1', got '%s'", cfg.UpstreamURL)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected LogLevel 'debug', got '%s'", cfg.LogLevel)
	}

	cfg.Listen = originalListen
}

func TestClone(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIKey = "test-key"

	cloned := cfg.Clone()
	if cloned.APIKey != cfg.APIKey {
		t.Error("clone should have same APIKey")
	}

	cloned.APIKey = "changed"
	if cfg.APIKey == "changed" {
		t.Error("clone should be independent")
	}
}

func TestMerge(t *testing.T) {
	cfg1 := &Config{
		Listen:      "127.0.0.1:8080",
		UpstreamURL: "https://api.opencode.ai/v1",
		APIKey:      "key1",
		TokenLimit:  100000,
	}

	cfg2 := &Config{
		Listen:  "0.0.0.0:9090",
		APIKey:  "key2",
		TokenLimit: 200000,
	}

	merged := cfg1.Merge(cfg2)

	if merged.Listen != "0.0.0.0:9090" {
		t.Errorf("expected Listen '0.0.0.0:9090', got '%s'", merged.Listen)
	}
	if merged.APIKey != "key2" {
		t.Errorf("expected APIKey 'key2', got '%s'", merged.APIKey)
	}
	if merged.TokenLimit != 200000 {
		t.Errorf("expected TokenLimit 200000, got %d", merged.TokenLimit)
	}
	if merged.UpstreamURL != "https://api.opencode.ai/v1" {
		t.Errorf("UpstreamURL should not change")
	}
}

func TestFormatLogLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"debug", "debug"},
		{"info", "info"},
		{"warn", "warn"},
		{"warning", "warn"},
		{"error", "error"},
		{"err", "error"},
		{"invalid", "info"},
		{"", "info"},
	}

	for _, tt := range tests {
		result := FormatLogLevel(tt.input)
		if result != tt.expected {
			t.Errorf("FormatLogLevel(%s) = %s, want %s", tt.input, result, tt.expected)
		}
	}
}

func TestToFileAndLoadConfigFile(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIKey = "test-key"

	file := cfg.ToFile()
	if file.Version != "1.0" {
		t.Errorf("expected version '1.0', got '%s'", file.Version)
	}

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.json")

	data, _ := json.MarshalIndent(file, "", "  ")
	os.WriteFile(path, data, 0644)

	loaded, err := LoadConfigFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if loaded.APIKey != "test-key" {
		t.Errorf("expected APIKey 'test-key', got '%s'", loaded.APIKey)
	}
}

func TestConfigWithEnvVarInterpolation(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	os.Setenv("OC4CLAUDE_TEST_API_KEY", "interpolated-key")
	defer os.Unsetenv("OC4CLAUDE_TEST_API_KEY")

	content := `{
		"listen": "127.0.0.1:8080",
		"upstream_url": "https://api.opencode.ai/v1",
		"api_key": "${OC4CLAUDE_TEST_API_KEY}",
		"default_model": "anthropic/claude-3.5-sonnet",
		"token_limit": 180000,
		"log_level": "info",
		"circuit_breaker": {
			"enabled": true,
			"failure_threshold": 3,
			"recovery_timeout_seconds": 60
		}
	}`

	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.APIKey != "interpolated-key" {
		t.Errorf("expected APIKey 'interpolated-key', got '%s'", cfg.APIKey)
	}
}

func TestConfigWithDefaultEnvVar(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	os.Unsetenv("OC4CLAUDE_UNSET_VAR")
	content := `{
		"listen": "127.0.0.1:8080",
		"upstream_url": "https://api.opencode.ai/v1",
		"api_key": "key",
		"default_model": "anthropic/claude-3.5-sonnet",
		"token_limit": 180000,
		"log_level": "info",
		"circuit_breaker": {
			"enabled": true,
			"failure_threshold": 3,
			"recovery_timeout_seconds": 60
		},
		"background": ${UNSET_VAR:-false}
	}`

	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Background != false {
		t.Errorf("expected Background false, got %v", cfg.Background)
	}
}