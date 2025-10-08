package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

// Config represents the application configuration
type Config struct {
	Server struct {
		Port         string        `json:"port"`
		ReadTimeout  time.Duration `json:"read_timeout"`
		WriteTimeout time.Duration `json:"write_timeout"`
		IdleTimeout  time.Duration `json:"idle_timeout"`
	} `json:"server"`

	Database struct {
		Host         string        `json:"host"`
		Port         string        `json:"port"`
		User         string        `json:"user"`
		Password     string        `json:"password"`
		Name         string        `json:"name"`
		MaxConns     int           `json:"max_connections"`
		MaxIdleConns int           `json:"max_idle_connections"`
		ConnLifetime time.Duration `json:"connection_lifetime"`
	} `json:"database"`

	Redis struct {
		Host         string        `json:"host"`
		Port         string        `json:"port"`
		Password     string        `json:"password"`
		DB           int           `json:"db"`
		PoolSize     int           `json:"pool_size"`
		MinIdleConns int           `json:"min_idle_connections"`
		DialTimeout  time.Duration `json:"dial_timeout"`
		ReadTimeout  time.Duration `json:"read_timeout"`
		WriteTimeout time.Duration `json:"write_timeout"`
	} `json:"redis"`

	API struct {
		MaxWorkers      int           `json:"max_workers"`
		FetchInterval   time.Duration `json:"fetch_interval"`
		Timeout         time.Duration `json:"timeout"`
		MaxRetries      int           `json:"max_retries"`
		RetryDelay      time.Duration `json:"retry_delay"`
		RateLimit       int           `json:"rate_limit"`
		RateLimitWindow time.Duration `json:"rate_limit_window"`
	} `json:"api"`

	HTTP struct {
		MaxIdleConns          int           `json:"max_idle_connections"`
		MaxIdleConnsPerHost   int           `json:"max_idle_connections_per_host"`
		IdleConnTimeout       time.Duration `json:"idle_connection_timeout"`
		MaxConnsPerHost       int           `json:"max_connections_per_host"`
		TLSHandshakeTimeout   time.Duration `json:"tls_handshake_timeout"`
		ResponseHeaderTimeout time.Duration `json:"response_header_timeout"`
		ExpectContinueTimeout time.Duration `json:"expect_continue_timeout"`
	} `json:"http"`

	Monitoring struct {
		HealthCheckInterval time.Duration `json:"health_check_interval"`
		MetricsInterval     time.Duration `json:"metrics_interval"`
		LogLevel            string        `json:"log_level"`
	} `json:"monitoring"`

	CircuitBreaker struct {
		MaxFailures      int           `json:"max_failures"`
		Timeout          time.Duration `json:"timeout"`
		HalfOpenMaxCalls int           `json:"half_open_max_calls"`
	} `json:"circuit_breaker"`
}

// ConfigManager manages application configuration
type ConfigManager struct {
	config     *Config
	mutex      sync.RWMutex
	configFile string
	lastMod    time.Time
}

// NewConfigManager creates a new configuration manager
func NewConfigManager(configFile string) *ConfigManager {
	return &ConfigManager{
		configFile: configFile,
	}
}

// LoadConfig loads configuration from file
func (cm *ConfigManager) LoadConfig() error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// Check if config file exists
	if _, err := os.Stat(cm.configFile); os.IsNotExist(err) {
		// Create default config
		cm.config = cm.getDefaultConfig()
		return cm.saveConfig()
	}

	// Read config file
	data, err := os.ReadFile(cm.configFile)
	if err != nil {
		return fmt.Errorf("failed to read config file: %v", err)
	}

	// Parse JSON with fixed duration parsing
	var fixedConfig FixedConfig
	if err := json.Unmarshal(data, &fixedConfig); err != nil {
		return fmt.Errorf("failed to parse config file: %v", err)
	}

	// Convert to regular config
	cm.config = fixedConfig.ConvertToConfig()

	// Get file modification time
	if stat, err := os.Stat(cm.configFile); err == nil {
		cm.lastMod = stat.ModTime()
	}

	log.Println("✅ Configuration loaded successfully")
	return nil
}

// saveConfig saves configuration to file
func (cm *ConfigManager) saveConfig() error {
	// Convert to fixed config for saving
	fixedConfig := &FixedConfig{
		Server: struct {
			Port         string   `json:"port"`
			ReadTimeout  Duration `json:"read_timeout"`
			WriteTimeout Duration `json:"write_timeout"`
			IdleTimeout  Duration `json:"idle_timeout"`
		}{
			Port:         cm.config.Server.Port,
			ReadTimeout:  Duration(cm.config.Server.ReadTimeout),
			WriteTimeout: Duration(cm.config.Server.WriteTimeout),
			IdleTimeout:  Duration(cm.config.Server.IdleTimeout),
		},
		Database: struct {
			Host         string   `json:"host"`
			Port         string   `json:"port"`
			User         string   `json:"user"`
			Password     string   `json:"password"`
			Name         string   `json:"name"`
			MaxConns     int      `json:"max_connections"`
			MaxIdleConns int      `json:"max_idle_connections"`
			ConnLifetime Duration `json:"connection_lifetime"`
		}{
			Host:         cm.config.Database.Host,
			Port:         cm.config.Database.Port,
			User:         cm.config.Database.User,
			Password:     cm.config.Database.Password,
			Name:         cm.config.Database.Name,
			MaxConns:     cm.config.Database.MaxConns,
			MaxIdleConns: cm.config.Database.MaxIdleConns,
			ConnLifetime: Duration(cm.config.Database.ConnLifetime),
		},
		Redis: struct {
			Host         string   `json:"host"`
			Port         string   `json:"port"`
			Password     string   `json:"password"`
			DB           int      `json:"db"`
			PoolSize     int      `json:"pool_size"`
			MinIdleConns int      `json:"min_idle_connections"`
			DialTimeout  Duration `json:"dial_timeout"`
			ReadTimeout  Duration `json:"read_timeout"`
			WriteTimeout Duration `json:"write_timeout"`
		}{
			Host:         cm.config.Redis.Host,
			Port:         cm.config.Redis.Port,
			Password:     cm.config.Redis.Password,
			DB:           cm.config.Redis.DB,
			PoolSize:     cm.config.Redis.PoolSize,
			MinIdleConns: cm.config.Redis.MinIdleConns,
			DialTimeout:  Duration(cm.config.Redis.DialTimeout),
			ReadTimeout:  Duration(cm.config.Redis.ReadTimeout),
			WriteTimeout: Duration(cm.config.Redis.WriteTimeout),
		},
		API: struct {
			MaxWorkers      int      `json:"max_workers"`
			FetchInterval   Duration `json:"fetch_interval"`
			Timeout         Duration `json:"timeout"`
			MaxRetries      int      `json:"max_retries"`
			RetryDelay      Duration `json:"retry_delay"`
			RateLimit       int      `json:"rate_limit"`
			RateLimitWindow Duration `json:"rate_limit_window"`
		}{
			MaxWorkers:      cm.config.API.MaxWorkers,
			FetchInterval:   Duration(cm.config.API.FetchInterval),
			Timeout:         Duration(cm.config.API.Timeout),
			MaxRetries:      cm.config.API.MaxRetries,
			RetryDelay:      Duration(cm.config.API.RetryDelay),
			RateLimit:       cm.config.API.RateLimit,
			RateLimitWindow: Duration(cm.config.API.RateLimitWindow),
		},
		HTTP: struct {
			MaxIdleConns          int      `json:"max_idle_connections"`
			MaxIdleConnsPerHost   int      `json:"max_idle_connections_per_host"`
			IdleConnTimeout       Duration `json:"idle_connection_timeout"`
			MaxConnsPerHost       int      `json:"max_connections_per_host"`
			TLSHandshakeTimeout   Duration `json:"tls_handshake_timeout"`
			ResponseHeaderTimeout Duration `json:"response_header_timeout"`
			ExpectContinueTimeout Duration `json:"expect_continue_timeout"`
		}{
			MaxIdleConns:          cm.config.HTTP.MaxIdleConns,
			MaxIdleConnsPerHost:   cm.config.HTTP.MaxIdleConnsPerHost,
			IdleConnTimeout:       Duration(cm.config.HTTP.IdleConnTimeout),
			MaxConnsPerHost:       cm.config.HTTP.MaxConnsPerHost,
			TLSHandshakeTimeout:   Duration(cm.config.HTTP.TLSHandshakeTimeout),
			ResponseHeaderTimeout: Duration(cm.config.HTTP.ResponseHeaderTimeout),
			ExpectContinueTimeout: Duration(cm.config.HTTP.ExpectContinueTimeout),
		},
		Monitoring: struct {
			HealthCheckInterval Duration `json:"health_check_interval"`
			MetricsInterval     Duration `json:"metrics_interval"`
			LogLevel            string   `json:"log_level"`
		}{
			HealthCheckInterval: Duration(cm.config.Monitoring.HealthCheckInterval),
			MetricsInterval:     Duration(cm.config.Monitoring.MetricsInterval),
			LogLevel:            cm.config.Monitoring.LogLevel,
		},
		CircuitBreaker: struct {
			MaxFailures      int      `json:"max_failures"`
			Timeout          Duration `json:"timeout"`
			HalfOpenMaxCalls int      `json:"half_open_max_calls"`
		}{
			MaxFailures:      cm.config.CircuitBreaker.MaxFailures,
			Timeout:          Duration(cm.config.CircuitBreaker.Timeout),
			HalfOpenMaxCalls: cm.config.CircuitBreaker.HalfOpenMaxCalls,
		},
	}

	data, err := json.MarshalIndent(fixedConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}

	return os.WriteFile(cm.configFile, data, 0644)
}

// getDefaultConfig returns the default configuration
func (cm *ConfigManager) getDefaultConfig() *Config {
	config := &Config{}

	// Server defaults
	config.Server.Port = ":8000"
	config.Server.ReadTimeout = 30 * time.Second
	config.Server.WriteTimeout = 30 * time.Second
	config.Server.IdleTimeout = 120 * time.Second

	// Database defaults
	config.Database.Host = "localhost"
	config.Database.Port = "3306"
	config.Database.User = "root"
	config.Database.Password = ""
	config.Database.Name = "bolt_tracker"
	config.Database.MaxConns = 25
	config.Database.MaxIdleConns = 5
	config.Database.ConnLifetime = 5 * time.Minute

	// Redis defaults
	config.Redis.Host = "localhost"
	config.Redis.Port = "6379"
	config.Redis.Password = ""
	config.Redis.DB = 0
	config.Redis.PoolSize = 10
	config.Redis.MinIdleConns = 5
	config.Redis.DialTimeout = 5 * time.Second
	config.Redis.ReadTimeout = 3 * time.Second
	config.Redis.WriteTimeout = 3 * time.Second

	// API defaults
	config.API.MaxWorkers = 3
	config.API.FetchInterval = 5 * time.Second
	config.API.Timeout = 10 * time.Second
	config.API.MaxRetries = 3
	config.API.RetryDelay = 1 * time.Second
	config.API.RateLimit = 10
	config.API.RateLimitWindow = 60 * time.Second

	// HTTP defaults
	config.HTTP.MaxIdleConns = 200
	config.HTTP.MaxIdleConnsPerHost = 50
	config.HTTP.IdleConnTimeout = 120 * time.Second
	config.HTTP.MaxConnsPerHost = 100
	config.HTTP.TLSHandshakeTimeout = 5 * time.Second
	config.HTTP.ResponseHeaderTimeout = 10 * time.Second
	config.HTTP.ExpectContinueTimeout = 1 * time.Second

	// Monitoring defaults
	config.Monitoring.HealthCheckInterval = 30 * time.Second
	config.Monitoring.MetricsInterval = 60 * time.Second
	config.Monitoring.LogLevel = "info"

	// Circuit breaker defaults
	config.CircuitBreaker.MaxFailures = 5
	config.CircuitBreaker.Timeout = 30 * time.Second
	config.CircuitBreaker.HalfOpenMaxCalls = 3

	return config
}

// GetConfig returns the current configuration
func (cm *ConfigManager) GetConfig() *Config {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	return cm.config
}

// UpdateConfig updates the configuration
func (cm *ConfigManager) UpdateConfig(newConfig *Config) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	cm.config = newConfig
	return cm.saveConfig()
}

// ReloadConfig reloads configuration from file
func (cm *ConfigManager) ReloadConfig() error {
	// Check if file has been modified
	if stat, err := os.Stat(cm.configFile); err == nil {
		if stat.ModTime().After(cm.lastMod) {
			log.Println("🔄 Configuration file modified, reloading...")
			return cm.LoadConfig()
		}
	}
	return nil
}

// WatchConfig starts watching for configuration changes
func (cm *ConfigManager) WatchConfig() {
	ticker := time.NewTicker(10 * time.Second) // Check every 10 seconds
	defer ticker.Stop()

	for range ticker.C {
		if err := cm.ReloadConfig(); err != nil {
			log.Printf("⚠️ Failed to reload config: %v", err)
		}
	}
}

// Global configuration manager
var globalConfigManager *ConfigManager

// InitializeConfigManager initializes the global configuration manager
func InitializeConfigManager(configFile string) error {
	globalConfigManager = NewConfigManager(configFile)
	return globalConfigManager.LoadConfig()
}

// GetConfig returns the global configuration
func GetConfig() *Config {
	if globalConfigManager == nil {
		// Return default config if not initialized
		cm := NewConfigManager("")
		return cm.getDefaultConfig()
	}
	return globalConfigManager.GetConfig()
}

// UpdateConfig updates the global configuration
func UpdateConfig(newConfig *Config) error {
	if globalConfigManager == nil {
		return fmt.Errorf("configuration manager not initialized")
	}
	return globalConfigManager.UpdateConfig(newConfig)
}
