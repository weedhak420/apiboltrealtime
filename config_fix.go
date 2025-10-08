package main

import (
	"encoding/json"
	"fmt"
	"time"
)

// Duration is a custom type that can unmarshal from string
type Duration time.Duration

// UnmarshalJSON implements json.Unmarshaler
func (d *Duration) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	duration, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration: %s", s)
	}

	*d = Duration(duration)
	return nil
}

// MarshalJSON implements json.Marshaler
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

// String returns the string representation of the duration
func (d Duration) String() string {
	return time.Duration(d).String()
}

// ToDuration converts to time.Duration
func (d Duration) ToDuration() time.Duration {
	return time.Duration(d)
}

// FixedConfig represents the application configuration with fixed duration parsing
type FixedConfig struct {
	Server struct {
		Port         string   `json:"port"`
		ReadTimeout  Duration `json:"read_timeout"`
		WriteTimeout Duration `json:"write_timeout"`
		IdleTimeout  Duration `json:"idle_timeout"`
	} `json:"server"`

	Database struct {
		Host         string   `json:"host"`
		Port         string   `json:"port"`
		User         string   `json:"user"`
		Password     string   `json:"password"`
		Name         string   `json:"name"`
		MaxConns     int      `json:"max_connections"`
		MaxIdleConns int      `json:"max_idle_connections"`
		ConnLifetime Duration `json:"connection_lifetime"`
	} `json:"database"`

	Redis struct {
		Host         string   `json:"host"`
		Port         string   `json:"port"`
		Password     string   `json:"password"`
		DB           int      `json:"db"`
		PoolSize     int      `json:"pool_size"`
		MinIdleConns int      `json:"min_idle_connections"`
		DialTimeout  Duration `json:"dial_timeout"`
		ReadTimeout  Duration `json:"read_timeout"`
		WriteTimeout Duration `json:"write_timeout"`
	} `json:"redis"`

	API struct {
		MaxWorkers      int      `json:"max_workers"`
		FetchInterval   Duration `json:"fetch_interval"`
		Timeout         Duration `json:"timeout"`
		MaxRetries      int      `json:"max_retries"`
		RetryDelay      Duration `json:"retry_delay"`
		RateLimit       int      `json:"rate_limit"`
		RateLimitWindow Duration `json:"rate_limit_window"`
	} `json:"api"`

	HTTP struct {
		MaxIdleConns          int      `json:"max_idle_connections"`
		MaxIdleConnsPerHost   int      `json:"max_idle_connections_per_host"`
		IdleConnTimeout       Duration `json:"idle_connection_timeout"`
		MaxConnsPerHost       int      `json:"max_connections_per_host"`
		TLSHandshakeTimeout   Duration `json:"tls_handshake_timeout"`
		ResponseHeaderTimeout Duration `json:"response_header_timeout"`
		ExpectContinueTimeout Duration `json:"expect_continue_timeout"`
	} `json:"http"`

	Monitoring struct {
		HealthCheckInterval Duration `json:"health_check_interval"`
		MetricsInterval     Duration `json:"metrics_interval"`
		LogLevel            string   `json:"log_level"`
	} `json:"monitoring"`

	CircuitBreaker struct {
		MaxFailures      int      `json:"max_failures"`
		Timeout          Duration `json:"timeout"`
		HalfOpenMaxCalls int      `json:"half_open_max_calls"`
	} `json:"circuit_breaker"`
}

// ConvertToConfig converts FixedConfig to Config
func (fc *FixedConfig) ConvertToConfig() *Config {
	return &Config{
		Server: struct {
			Port         string        `json:"port"`
			ReadTimeout  time.Duration `json:"read_timeout"`
			WriteTimeout time.Duration `json:"write_timeout"`
			IdleTimeout  time.Duration `json:"idle_timeout"`
		}{
			Port:         fc.Server.Port,
			ReadTimeout:  fc.Server.ReadTimeout.ToDuration(),
			WriteTimeout: fc.Server.WriteTimeout.ToDuration(),
			IdleTimeout:  fc.Server.IdleTimeout.ToDuration(),
		},
		Database: struct {
			Host         string        `json:"host"`
			Port         string        `json:"port"`
			User         string        `json:"user"`
			Password     string        `json:"password"`
			Name         string        `json:"name"`
			MaxConns     int           `json:"max_connections"`
			MaxIdleConns int           `json:"max_idle_connections"`
			ConnLifetime time.Duration `json:"connection_lifetime"`
		}{
			Host:         fc.Database.Host,
			Port:         fc.Database.Port,
			User:         fc.Database.User,
			Password:     fc.Database.Password,
			Name:         fc.Database.Name,
			MaxConns:     fc.Database.MaxConns,
			MaxIdleConns: fc.Database.MaxIdleConns,
			ConnLifetime: fc.Database.ConnLifetime.ToDuration(),
		},
		Redis: struct {
			Host         string        `json:"host"`
			Port         string        `json:"port"`
			Password     string        `json:"password"`
			DB           int           `json:"db"`
			PoolSize     int           `json:"pool_size"`
			MinIdleConns int           `json:"min_idle_connections"`
			DialTimeout  time.Duration `json:"dial_timeout"`
			ReadTimeout  time.Duration `json:"read_timeout"`
			WriteTimeout time.Duration `json:"write_timeout"`
		}{
			Host:         fc.Redis.Host,
			Port:         fc.Redis.Port,
			Password:     fc.Redis.Password,
			DB:           fc.Redis.DB,
			PoolSize:     fc.Redis.PoolSize,
			MinIdleConns: fc.Redis.MinIdleConns,
			DialTimeout:  fc.Redis.DialTimeout.ToDuration(),
			ReadTimeout:  fc.Redis.ReadTimeout.ToDuration(),
			WriteTimeout: fc.Redis.WriteTimeout.ToDuration(),
		},
		API: struct {
			MaxWorkers      int           `json:"max_workers"`
			FetchInterval   time.Duration `json:"fetch_interval"`
			Timeout         time.Duration `json:"timeout"`
			MaxRetries      int           `json:"max_retries"`
			RetryDelay      time.Duration `json:"retry_delay"`
			RateLimit       int           `json:"rate_limit"`
			RateLimitWindow time.Duration `json:"rate_limit_window"`
		}{
			MaxWorkers:      fc.API.MaxWorkers,
			FetchInterval:   fc.API.FetchInterval.ToDuration(),
			Timeout:         fc.API.Timeout.ToDuration(),
			MaxRetries:      fc.API.MaxRetries,
			RetryDelay:      fc.API.RetryDelay.ToDuration(),
			RateLimit:       fc.API.RateLimit,
			RateLimitWindow: fc.API.RateLimitWindow.ToDuration(),
		},
		HTTP: struct {
			MaxIdleConns          int           `json:"max_idle_connections"`
			MaxIdleConnsPerHost   int           `json:"max_idle_connections_per_host"`
			IdleConnTimeout       time.Duration `json:"idle_connection_timeout"`
			MaxConnsPerHost       int           `json:"max_connections_per_host"`
			TLSHandshakeTimeout   time.Duration `json:"tls_handshake_timeout"`
			ResponseHeaderTimeout time.Duration `json:"response_header_timeout"`
			ExpectContinueTimeout time.Duration `json:"expect_continue_timeout"`
		}{
			MaxIdleConns:          fc.HTTP.MaxIdleConns,
			MaxIdleConnsPerHost:   fc.HTTP.MaxIdleConnsPerHost,
			IdleConnTimeout:       fc.HTTP.IdleConnTimeout.ToDuration(),
			MaxConnsPerHost:       fc.HTTP.MaxConnsPerHost,
			TLSHandshakeTimeout:   fc.HTTP.TLSHandshakeTimeout.ToDuration(),
			ResponseHeaderTimeout: fc.HTTP.ResponseHeaderTimeout.ToDuration(),
			ExpectContinueTimeout: fc.HTTP.ExpectContinueTimeout.ToDuration(),
		},
		Monitoring: struct {
			HealthCheckInterval time.Duration `json:"health_check_interval"`
			MetricsInterval     time.Duration `json:"metrics_interval"`
			LogLevel            string        `json:"log_level"`
		}{
			HealthCheckInterval: fc.Monitoring.HealthCheckInterval.ToDuration(),
			MetricsInterval:     fc.Monitoring.MetricsInterval.ToDuration(),
			LogLevel:            fc.Monitoring.LogLevel,
		},
		CircuitBreaker: struct {
			MaxFailures      int           `json:"max_failures"`
			Timeout          time.Duration `json:"timeout"`
			HalfOpenMaxCalls int           `json:"half_open_max_calls"`
		}{
			MaxFailures:      fc.CircuitBreaker.MaxFailures,
			Timeout:          fc.CircuitBreaker.Timeout.ToDuration(),
			HalfOpenMaxCalls: fc.CircuitBreaker.HalfOpenMaxCalls,
		},
	}
}
