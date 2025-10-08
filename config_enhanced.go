package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

// EnhancedConfig represents the application configuration with environment variable support
type EnhancedConfig struct {
	Server         ServerConfig         `json:"server" mapstructure:"server"`
	Database       DatabaseConfig       `json:"database" mapstructure:"database"`
	Redis          RedisConfig          `json:"redis" mapstructure:"redis"`
	API            APIConfig            `json:"api" mapstructure:"api"`
	HTTP           HTTPConfig           `json:"http" mapstructure:"http"`
	Monitoring     MonitoringConfig     `json:"monitoring" mapstructure:"monitoring"`
	CircuitBreaker CircuitBreakerConfig `json:"circuit_breaker" mapstructure:"circuit_breaker"`
	JWT            JWTConfig            `json:"jwt" mapstructure:"jwt"`
	RateLimiting   RateLimitingConfig   `json:"rate_limiting" mapstructure:"rate_limiting"`
	Observability  ObservabilityConfig  `json:"observability" mapstructure:"observability"`
	Security       SecurityConfig       `json:"security" mapstructure:"security"`
}

type ServerConfig struct {
	Port                    string        `json:"port" mapstructure:"port"`
	ReadTimeout             time.Duration `json:"read_timeout" mapstructure:"read_timeout"`
	WriteTimeout            time.Duration `json:"write_timeout" mapstructure:"write_timeout"`
	IdleTimeout             time.Duration `json:"idle_timeout" mapstructure:"idle_timeout"`
	GracefulShutdownTimeout time.Duration `json:"graceful_shutdown_timeout" mapstructure:"graceful_shutdown_timeout"`
}

type DatabaseConfig struct {
	Host               string        `json:"host" mapstructure:"host"`
	Port               string        `json:"port" mapstructure:"port"`
	User               string        `json:"user" mapstructure:"user"`
	Password           string        `json:"password" mapstructure:"password"`
	Name               string        `json:"name" mapstructure:"name"`
	MaxConnections     int           `json:"max_connections" mapstructure:"max_connections"`
	MaxIdleConnections int           `json:"max_idle_connections" mapstructure:"max_idle_connections"`
	ConnectionLifetime time.Duration `json:"connection_lifetime" mapstructure:"connection_lifetime"`
	RetryAttempts      int           `json:"retry_attempts" mapstructure:"retry_attempts"`
	RetryDelay         time.Duration `json:"retry_delay" mapstructure:"retry_delay"`
}

type RedisConfig struct {
	Host               string        `json:"host" mapstructure:"host"`
	Port               string        `json:"port" mapstructure:"port"`
	Username           string        `json:"username" mapstructure:"username"`
	Password           string        `json:"password" mapstructure:"password"`
	DB                 int           `json:"db" mapstructure:"db"`
	PoolSize           int           `json:"pool_size" mapstructure:"pool_size"`
	MinIdleConnections int           `json:"min_idle_connections" mapstructure:"min_idle_connections"`
	DialTimeout        time.Duration `json:"dial_timeout" mapstructure:"dial_timeout"`
	ReadTimeout        time.Duration `json:"read_timeout" mapstructure:"read_timeout"`
	WriteTimeout       time.Duration `json:"write_timeout" mapstructure:"write_timeout"`
	MaxRetries         int           `json:"max_retries" mapstructure:"max_retries"`
	RetryBackoff       time.Duration `json:"retry_backoff" mapstructure:"retry_backoff"`
	Namespace          string        `json:"namespace" mapstructure:"namespace"`
	CircuitBreaker     bool          `json:"circuit_breaker" mapstructure:"circuit_breaker"`
}

type APIConfig struct {
	MaxWorkers          int           `json:"max_workers" mapstructure:"max_workers"`
	FetchInterval       time.Duration `json:"fetch_interval" mapstructure:"fetch_interval"`
	Timeout             time.Duration `json:"timeout" mapstructure:"timeout"`
	MaxRetries          int           `json:"max_retries" mapstructure:"max_retries"`
	RetryDelay          time.Duration `json:"retry_delay" mapstructure:"retry_delay"`
	RateLimit           int           `json:"rate_limit" mapstructure:"rate_limit"`
	RateLimitWindow     time.Duration `json:"rate_limit_window" mapstructure:"rate_limit_window"`
	BatchSize           int           `json:"batch_size" mapstructure:"batch_size"`
	DelayBetweenBatches time.Duration `json:"delay_between_batches" mapstructure:"delay_between_batches"`
}

type HTTPConfig struct {
	MaxIdleConnections        int           `json:"max_idle_connections" mapstructure:"max_idle_connections"`
	MaxIdleConnectionsPerHost int           `json:"max_idle_connections_per_host" mapstructure:"max_idle_connections_per_host"`
	IdleConnectionTimeout     time.Duration `json:"idle_connection_timeout" mapstructure:"idle_connection_timeout"`
	MaxConnectionsPerHost     int           `json:"max_connections_per_host" mapstructure:"max_connections_per_host"`
	TLSHandshakeTimeout       time.Duration `json:"tls_handshake_timeout" mapstructure:"tls_handshake_timeout"`
	ResponseHeaderTimeout     time.Duration `json:"response_header_timeout" mapstructure:"response_header_timeout"`
	ExpectContinueTimeout     time.Duration `json:"expect_continue_timeout" mapstructure:"expect_continue_timeout"`
}

type MonitoringConfig struct {
	HealthCheckInterval time.Duration `json:"health_check_interval" mapstructure:"health_check_interval"`
	MetricsInterval     time.Duration `json:"metrics_interval" mapstructure:"metrics_interval"`
	LogLevel            string        `json:"log_level" mapstructure:"log_level"`
	EnablePrometheus    bool          `json:"enable_prometheus" mapstructure:"enable_prometheus"`
	PrometheusPort      string        `json:"prometheus_port" mapstructure:"prometheus_port"`
	EnableTracing       bool          `json:"enable_tracing" mapstructure:"enable_tracing"`
}

type CircuitBreakerConfig struct {
	MaxFailures      int           `json:"max_failures" mapstructure:"max_failures"`
	Timeout          time.Duration `json:"timeout" mapstructure:"timeout"`
	HalfOpenMaxCalls int           `json:"half_open_max_calls" mapstructure:"half_open_max_calls"`
}

type JWTConfig struct {
	AccessTokenExpiry  time.Duration `json:"access_token_expiry" mapstructure:"access_token_expiry"`
	RefreshTokenExpiry time.Duration `json:"refresh_token_expiry" mapstructure:"refresh_token_expiry"`
	KeyRotationEnabled bool          `json:"key_rotation_enabled" mapstructure:"key_rotation_enabled"`
	RevocationStore    string        `json:"revocation_store" mapstructure:"revocation_store"`
	Issuer             string        `json:"issuer" mapstructure:"issuer"`
	Audience           string        `json:"audience" mapstructure:"audience"`
}

type RateLimitingConfig struct {
	GlobalLimit    int           `json:"global_limit" mapstructure:"global_limit"`
	PerUserLimit   int           `json:"per_user_limit" mapstructure:"per_user_limit"`
	PerIPLimit     int           `json:"per_ip_limit" mapstructure:"per_ip_limit"`
	Window         time.Duration `json:"window" mapstructure:"window"`
	Distributed    bool          `json:"distributed" mapstructure:"distributed"`
	StorageBackend string        `json:"storage_backend" mapstructure:"storage_backend"`
}

type ObservabilityConfig struct {
	StructuredLogging bool   `json:"structured_logging" mapstructure:"structured_logging"`
	LogFormat         string `json:"log_format" mapstructure:"log_format"`
	CorrelationID     bool   `json:"correlation_id" mapstructure:"correlation_id"`
	EnableMetrics     bool   `json:"enable_metrics" mapstructure:"enable_metrics"`
	EnableTracing     bool   `json:"enable_tracing" mapstructure:"enable_tracing"`
	ServiceName       string `json:"service_name" mapstructure:"service_name"`
	ServiceVersion    string `json:"service_version" mapstructure:"service_version"`
}

type SecurityConfig struct {
	EnableCORS       bool     `json:"enable_cors" mapstructure:"enable_cors"`
	CORSOrigins      []string `json:"cors_origins" mapstructure:"cors_origins"`
	EnableCSRF       bool     `json:"enable_csrf" mapstructure:"enable_csrf"`
	TrustedProxies   []string `json:"trusted_proxies" mapstructure:"trusted_proxies"`
	RateLimitEnabled bool     `json:"rate_limit_enabled" mapstructure:"rate_limit_enabled"`
}

// Global config instance with mutex for thread safety
var (
	config     *EnhancedConfig
	configMu   sync.RWMutex
	configPath string
)

// LoadConfig loads configuration from file and environment variables
func LoadConfig(configFile string) (*EnhancedConfig, error) {
	configPath = configFile

	// Set up viper
	viper.SetConfigFile(configFile)
	viper.SetConfigType("json")

	// Enable environment variable support
	viper.AutomaticEnv()
	viper.SetEnvPrefix("BOLT")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Set default values
	setDefaults()

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Unmarshal into struct
	var cfg EnhancedConfig
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate config
	if err := validateConfig(&cfg); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	configMu.Lock()
	config = &cfg
	configMu.Unlock()

	return &cfg, nil
}

// GetEnhancedConfig returns the current configuration (thread-safe)
func GetEnhancedConfig() *EnhancedConfig {
	configMu.RLock()
	defer configMu.RUnlock()
	return config
}

// ReloadConfig reloads configuration from file
func ReloadConfig() error {
	if configPath == "" {
		return fmt.Errorf("no config file path set")
	}

	_, err := LoadConfig(configPath)
	return err
}

// WatchConfig watches for config file changes and reloads automatically
func WatchConfig() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}
	defer watcher.Close()

	if err := watcher.Add(configPath); err != nil {
		return fmt.Errorf("failed to watch config file: %w", err)
	}

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					if err := ReloadConfig(); err != nil {
						fmt.Printf("Failed to reload config: %v\n", err)
					} else {
						fmt.Println("Config reloaded successfully")
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				fmt.Printf("Config watcher error: %v\n", err)
			}
		}
	}()

	return nil
}

// setDefaults sets default configuration values
func setDefaults() {
	// Server defaults
	viper.SetDefault("server.port", ":8000")
	viper.SetDefault("server.read_timeout", "30s")
	viper.SetDefault("server.write_timeout", "30s")
	viper.SetDefault("server.idle_timeout", "120s")
	viper.SetDefault("server.graceful_shutdown_timeout", "30s")

	// Database defaults
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", "3306")
	viper.SetDefault("database.user", "root")
	viper.SetDefault("database.password", "")
	viper.SetDefault("database.name", "bolt_tracker")
	viper.SetDefault("database.max_connections", 25)
	viper.SetDefault("database.max_idle_connections", 5)
	viper.SetDefault("database.connection_lifetime", "5m")
	viper.SetDefault("database.retry_attempts", 3)
	viper.SetDefault("database.retry_delay", "1s")

	// Redis defaults
	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", "6379")
	viper.SetDefault("redis.username", "")
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("redis.pool_size", 10)
	viper.SetDefault("redis.min_idle_connections", 5)
	viper.SetDefault("redis.dial_timeout", "5s")
	viper.SetDefault("redis.read_timeout", "3s")
	viper.SetDefault("redis.write_timeout", "3s")
	viper.SetDefault("redis.max_retries", 3)
	viper.SetDefault("redis.retry_backoff", "1s")
	viper.SetDefault("redis.namespace", "bolt_tracker")
	viper.SetDefault("redis.circuit_breaker", true)

	// API defaults
	viper.SetDefault("api.max_workers", 10)
	viper.SetDefault("api.fetch_interval", "10s")
	viper.SetDefault("api.timeout", "15s")
	viper.SetDefault("api.max_retries", 3)
	viper.SetDefault("api.retry_delay", "2s")
	viper.SetDefault("api.rate_limit", 30)
	viper.SetDefault("api.rate_limit_window", "60s")
	viper.SetDefault("api.batch_size", 20)
	viper.SetDefault("api.delay_between_batches", "2s")

	// HTTP defaults
	viper.SetDefault("http.max_idle_connections", 500)
	viper.SetDefault("http.max_idle_connections_per_host", 100)
	viper.SetDefault("http.idle_connection_timeout", "120s")
	viper.SetDefault("http.max_connections_per_host", 200)
	viper.SetDefault("http.tls_handshake_timeout", "10s")
	viper.SetDefault("http.response_header_timeout", "15s")
	viper.SetDefault("http.expect_continue_timeout", "2s")

	// Monitoring defaults
	viper.SetDefault("monitoring.health_check_interval", "30s")
	viper.SetDefault("monitoring.metrics_interval", "60s")
	viper.SetDefault("monitoring.log_level", "info")
	viper.SetDefault("monitoring.enable_prometheus", true)
	viper.SetDefault("monitoring.prometheus_port", ":9090")
	viper.SetDefault("monitoring.enable_tracing", false)

	// Circuit breaker defaults
	viper.SetDefault("circuit_breaker.max_failures", 5)
	viper.SetDefault("circuit_breaker.timeout", "30s")
	viper.SetDefault("circuit_breaker.half_open_max_calls", 3)

	// JWT defaults
	viper.SetDefault("jwt.access_token_expiry", "15m")
	viper.SetDefault("jwt.refresh_token_expiry", "168h")
	viper.SetDefault("jwt.key_rotation_enabled", true)
	viper.SetDefault("jwt.revocation_store", "redis")
	viper.SetDefault("jwt.issuer", "bolt-tracker")
	viper.SetDefault("jwt.audience", "bolt-api")

	// Rate limiting defaults
	viper.SetDefault("rate_limiting.global_limit", 1000)
	viper.SetDefault("rate_limiting.per_user_limit", 100)
	viper.SetDefault("rate_limiting.per_ip_limit", 200)
	viper.SetDefault("rate_limiting.window", "1m")
	viper.SetDefault("rate_limiting.distributed", false)
	viper.SetDefault("rate_limiting.storage_backend", "memory")

	// Observability defaults
	viper.SetDefault("observability.structured_logging", true)
	viper.SetDefault("observability.log_format", "json")
	viper.SetDefault("observability.correlation_id", true)
	viper.SetDefault("observability.enable_metrics", true)
	viper.SetDefault("observability.enable_tracing", false)
	viper.SetDefault("observability.service_name", "bolt-tracker")
	viper.SetDefault("observability.service_version", "1.0.0")

	// Security defaults
	viper.SetDefault("security.enable_cors", true)
	viper.SetDefault("security.cors_origins", []string{"*"})
	viper.SetDefault("security.enable_csrf", false)
	viper.SetDefault("security.trusted_proxies", []string{})
	viper.SetDefault("security.rate_limit_enabled", true)
}

// validateConfig validates the configuration
func validateConfig(cfg *EnhancedConfig) error {
	if cfg.Server.Port == "" {
		return fmt.Errorf("server port is required")
	}

	if cfg.Database.Host == "" {
		return fmt.Errorf("database host is required")
	}

	if cfg.Redis.Host == "" {
		return fmt.Errorf("redis host is required")
	}

	if cfg.API.MaxWorkers <= 0 {
		return fmt.Errorf("api max_workers must be greater than 0")
	}

	if cfg.API.FetchInterval <= 0 {
		return fmt.Errorf("api fetch_interval must be greater than 0")
	}

	return nil
}

// GetEnvString gets environment variable with fallback
func GetEnvString(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// GetEnvInt gets environment variable as int with fallback
func GetEnvInt(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return fallback
}

// GetEnvBool gets environment variable as bool with fallback
func GetEnvBool(key string, fallback bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return fallback
}
