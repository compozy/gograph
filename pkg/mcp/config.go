package mcp

import (
	"time"
)

// Config represents the MCP server configuration
type Config struct {
	Server      ServerConfig      `yaml:"server"`
	Auth        AuthConfig        `yaml:"auth"`
	Performance PerformanceConfig `yaml:"performance"`
	Security    SecurityConfig    `yaml:"security"`
	Features    FeaturesConfig    `yaml:"features"`
}

// ServerConfig defines server settings
type ServerConfig struct {
	Port           int    `yaml:"port"`
	Host           string `yaml:"host"`
	MaxConnections int    `yaml:"max_connections"`
}

// AuthConfig defines authentication settings
type AuthConfig struct {
	Enabled bool   `yaml:"enabled"`
	Method  string `yaml:"method"`
	Token   string `yaml:"token,omitempty"`
}

// PerformanceConfig defines performance settings
type PerformanceConfig struct {
	MaxContextSize  int           `yaml:"max_context_size"`
	CacheTTL        int           `yaml:"cache_ttl"`
	EnableStreaming bool          `yaml:"enable_streaming"`
	BatchSize       int           `yaml:"batch_size"`
	RequestTimeout  time.Duration `yaml:"request_timeout"`
}

// SecurityConfig defines security settings
type SecurityConfig struct {
	AllowedPaths   []string `yaml:"allowed_paths"`
	ForbiddenPaths []string `yaml:"forbidden_paths"`
	RateLimit      int      `yaml:"rate_limit"`
	MaxQueryTime   int      `yaml:"max_query_time"`
}

// FeaturesConfig defines feature toggles
type FeaturesConfig struct {
	EnableIncremental bool `yaml:"enable_incremental"`
	EnableValidation  bool `yaml:"enable_validation"`
	EnablePatterns    bool `yaml:"enable_patterns"`
	EnableCaching     bool `yaml:"enable_caching"`
}

// DefaultConfig returns default MCP configuration
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:           8080,
			Host:           "localhost",
			MaxConnections: 100,
		},
		Auth: AuthConfig{
			Enabled: false,
			Method:  "token",
		},
		Performance: PerformanceConfig{
			MaxContextSize:  1000000,
			CacheTTL:        3600,
			EnableStreaming: true,
			BatchSize:       100,
			RequestTimeout:  30 * time.Second,
		},
		Security: SecurityConfig{
			AllowedPaths:   []string{"."},
			ForbiddenPaths: []string{".git", "vendor", "node_modules"},
			RateLimit:      100,
			MaxQueryTime:   30,
		},
		Features: FeaturesConfig{
			EnableIncremental: true,
			EnableValidation:  true,
			EnablePatterns:    true,
			EnableCaching:     true,
		},
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return &ConfigError{Field: "server.port", Message: "port must be between 1 and 65535"}
	}
	if c.Server.Host == "" {
		return &ConfigError{Field: "server.host", Message: "host cannot be empty"}
	}
	if c.Server.MaxConnections <= 0 {
		return &ConfigError{Field: "server.max_connections", Message: "max_connections must be positive"}
	}
	if c.Performance.MaxContextSize <= 0 {
		return &ConfigError{Field: "performance.max_context_size", Message: "max_context_size must be positive"}
	}
	if c.Performance.BatchSize <= 0 {
		return &ConfigError{Field: "performance.batch_size", Message: "batch_size must be positive"}
	}
	if c.Security.RateLimit <= 0 {
		return &ConfigError{Field: "security.rate_limit", Message: "rate_limit must be positive"}
	}
	if c.Security.MaxQueryTime <= 0 {
		return &ConfigError{Field: "security.max_query_time", Message: "max_query_time must be positive"}
	}
	return nil
}

// ConfigError represents a configuration validation error
type ConfigError struct {
	Field   string
	Message string
}

func (e *ConfigError) Error() string {
	return "config validation error: " + e.Field + " - " + e.Message
}
