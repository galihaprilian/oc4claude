package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
)

var envVarPattern = regexp.MustCompile(`\$\{([^}:]+)(?::-([^}]*))?\}`)

type Config struct {
	Listen           string                  `json:"listen"`
	UpstreamURL      string                  `json:"upstream_url"`
	APIKey           string                  `json:"api_key"`
	DefaultModel     string                  `json:"default_model"`
	Models           map[string]string       `json:"models"`
	FallbackChain    []string                `json:"fallback_chain"`
	CircuitBreaker   CircuitBreakerConfig     `json:"circuit_breaker"`
	TokenLimit       int                     `json:"token_limit"`
	LogLevel         string                  `json:"log_level"`
	Background       bool                    `json:"background"`
	AutoStart        bool                    `json:"auto_start"`
}

type CircuitBreakerConfig struct {
	Enabled                bool `json:"enabled"`
	FailureThreshold       int  `json:"failure_threshold"`
	RecoveryTimeoutSeconds int  `json:"recovery_timeout_seconds"`
}

func DefaultConfig() *Config {
	return &Config{
		Listen:      "127.0.0.1:8080",
		UpstreamURL: "https://api.opencode.ai/v1",
		Models: map[string]string{
			"default":       "anthropic/claude-3.5-sonnet",
			"thinking":      "anthropic/claude-3.5-haiku",
			"long_context":  "anthropic/claude-3.5-sonnet-20241022",
			"background":    "anthropic/claude-3.5-haiku",
		},
		FallbackChain: []string{
			"anthropic/claude-3.5-sonnet",
			"anthropic/claude-3.5-haiku",
		},
		CircuitBreaker: CircuitBreakerConfig{
			Enabled:                true,
			FailureThreshold:       3,
			RecoveryTimeoutSeconds: 60,
		},
		TokenLimit: 180000,
		LogLevel:   "info",
		Background: false,
		AutoStart:  false,
	}
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	interpolated, err := interpolateEnvVars(string(data))
	if err != nil {
		return nil, fmt.Errorf("failed to interpolate env vars: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal([]byte(interpolated), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

func interpolateEnvVars(content string) (string, error) {
	result := envVarPattern.ReplaceAllStringFunc(content, func(match string) string {
		parts := envVarPattern.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}

		varName := parts[1]
		defaultValue := ""
		if len(parts) >= 3 {
			defaultValue = parts[2]
		}

		if value, exists := os.LookupEnv(varName); exists {
			return value
		}
		return defaultValue
	})

	return result, nil
}

func (c *Config) Validate() error {
	if c.Listen == "" {
		return errors.New("listen address is required")
	}

	if c.UpstreamURL == "" {
		return errors.New("upstream_url is required")
	}

	if c.APIKey == "" {
		return errors.New("api_key is required")
	}

	if c.DefaultModel == "" {
		return errors.New("default_model is required")
	}

	if c.TokenLimit <= 0 {
		return errors.New("token_limit must be positive")
	}

	validLogLevels := map[string]bool{
		"debug": true, "info": true, "warn": true, "error": true,
	}
	if c.LogLevel != "" && !validLogLevels[c.LogLevel] {
		return fmt.Errorf("invalid log_level: %s (allowed: debug, info, warn, error)", c.LogLevel)
	}

	if c.CircuitBreaker.FailureThreshold <= 0 {
		return errors.New("circuit_breaker.failure_threshold must be positive")
	}

	if c.CircuitBreaker.RecoveryTimeoutSeconds <= 0 {
		return errors.New("circuit_breaker.recovery_timeout_seconds must be positive")
	}

	return nil
}

func DefaultConfigDir() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("failed to get current user: %w", err)
	}
	return filepath.Join(usr.HomeDir, ".oc4claude"), nil
}

func DefaultConfigPath() (string, error) {
	dir, err := DefaultConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func EnsureConfigFile() error {
	dir, err := DefaultConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	cfgPath, err := DefaultConfigPath()
	if err != nil {
		return err
	}

	if _, err := os.Stat(cfgPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check config file: %w", err)
	}

	cfg := DefaultConfig()
	cfg.APIKey = "${OPENCODE_API_KEY}"

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal default config: %w", err)
	}

	if err := os.WriteFile(cfgPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write default config: %w", err)
	}

	return nil
}

func (c *Config) ToJSON() (string, error) {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal config: %w", err)
	}
	return string(data), nil
}

func (c *Config) GetModelForContext(ctx string) string {
	if model, ok := c.Models[ctx]; ok {
		return model
	}
	return c.DefaultModel
}

func (c *Config) HasFallbackChain() bool {
	return len(c.FallbackChain) > 0
}

func (c *Config) AddToFallbackChain(model string) {
	for _, m := range c.FallbackChain {
		if m == model {
			return
		}
	}
	c.FallbackChain = append(c.FallbackChain, model)
}

func (c *Config) RemoveFromFallbackChain(model string) {
	var newChain []string
	for _, m := range c.FallbackChain {
		if m != model {
			newChain = append(newChain, m)
		}
	}
	c.FallbackChain = newChain
}

func (c *Config) Setenv(key, value string) error {
	return os.Setenv(key, value)
}

func (c *Config) Getenv(key string) string {
	return os.Getenv(key)
}

func (c *Config) ApplyEnvOverrides() {
	if v := os.Getenv("OC4CLAUDE_LISTEN"); v != "" {
		c.Listen = v
	}
	if v := os.Getenv("OC4CLAUDE_UPSTREAM"); v != "" {
		c.UpstreamURL = v
	}
	if v := os.Getenv("OC4CLAUDE_LOG_LEVEL"); v != "" {
		c.LogLevel = v
	}
}

func (c *Config) Clone() *Config {
	models := make(map[string]string)
	for k, v := range c.Models {
		models[k] = v
	}

	fallbackChain := make([]string, len(c.FallbackChain))
	copy(fallbackChain, c.FallbackChain)

	return &Config{
		Listen:           c.Listen,
		UpstreamURL:      c.UpstreamURL,
		APIKey:           c.APIKey,
		DefaultModel:     c.DefaultModel,
		Models:           models,
		FallbackChain:    fallbackChain,
		CircuitBreaker:   c.CircuitBreaker,
		TokenLimit:       c.TokenLimit,
		LogLevel:         c.LogLevel,
		Background:       c.Background,
		AutoStart:        c.AutoStart,
	}
}

func (c *Config) Merge(other *Config) *Config {
	result := c.Clone()

	if other.Listen != "" {
		result.Listen = other.Listen
	}
	if other.UpstreamURL != "" {
		result.UpstreamURL = other.UpstreamURL
	}
	if other.APIKey != "" {
		result.APIKey = other.APIKey
	}
	if other.DefaultModel != "" {
		result.DefaultModel = other.DefaultModel
	}
	if other.TokenLimit > 0 {
		result.TokenLimit = other.TokenLimit
	}
	if other.LogLevel != "" {
		result.LogLevel = other.LogLevel
	}
	result.Background = other.Background
	result.AutoStart = other.AutoStart

	if len(other.Models) > 0 {
		for k, v := range other.Models {
			result.Models[k] = v
		}
	}

	if len(other.FallbackChain) > 0 {
		result.FallbackChain = other.FallbackChain
	}

	if other.CircuitBreaker.Enabled {
		result.CircuitBreaker = other.CircuitBreaker
	}

	return result
}

type ModelConfig struct {
	Name           string `json:"name"`
	Context        string `json:"context"`
	FrequencyPenalty float64 `json:"frequency_penalty,omitempty"`
	PresencePenalty float64 `json:"presence_penalty,omitempty"`
	TopP           float64 `json:"top_p,omitempty"`
	Temperature    float64 `json:"temperature,omitempty"`
	MaxTokens      int     `json:"max_tokens,omitempty"`
}

type ConfigFile struct {
	Version string `json:"version,omitempty"`
	Config  `json:",inline"`
}

func (c *Config) ToFile() *ConfigFile {
	return &ConfigFile{
		Version: "1.0",
		Config:  *c,
	}
}

func LoadConfigFile(path string) (*ConfigFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	interpolated, err := interpolateEnvVars(string(data))
	if err != nil {
		return nil, fmt.Errorf("failed to interpolate env vars: %w", err)
	}

	var cf ConfigFile
	if err := json.Unmarshal([]byte(interpolated), &cf); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}

	return &cf, nil
}

func (cf *ConfigFile) ToConfig() *Config {
	return &cf.Config
}

func FormatLogLevel(level string) string {
	switch strings.ToLower(level) {
	case "debug":
		return "debug"
	case "info":
		return "info"
	case "warn", "warning":
		return "warn"
	case "error", "err":
		return "error"
	default:
		return "info"
	}
}