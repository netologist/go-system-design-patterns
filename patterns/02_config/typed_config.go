package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Environment represents deployment target (dev, staging, prod).
type Environment string

const (
	EnvDev     Environment = "dev"
	EnvStaging Environment = "staging"
	EnvProd    Environment = "prod"
)

// ServerConfig holds validated, strongly-typed configuration.
type ServerConfig struct {
	Env          Environment
	Port         int
	DatabaseURL  string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	MaxWorkers   int
}

// EnvGetter is a function to lookup environment variables.
type EnvGetter func(key string) string

// LoadFromEnv parses and validates ServerConfig from environment variables.
func LoadFromEnv(get EnvGetter) (*ServerConfig, error) {
	if get == nil {
		get = os.Getenv
	}

	var errs []error

	// Environment
	envStr := get("APP_ENV")
	if envStr == "" {
		envStr = "dev" // Default
	}
	env := Environment(envStr)
	if env != EnvDev && env != EnvStaging && env != EnvProd {
		errs = append(errs, fmt.Errorf("invalid APP_ENV '%s': must be dev, staging, or prod", envStr))
	}

	// Port
	portStr := get("APP_PORT")
	port := 8080 // Default
	if portStr != "" {
		p, err := strconv.Atoi(portStr)
		if err != nil || p < 1 || p > 65535 {
			errs = append(errs, fmt.Errorf("invalid APP_PORT '%s': must be between 1 and 65535", portStr))
		} else {
			port = p
		}
	}

	// Database URL (Required)
	dbURL := get("DATABASE_URL")
	if dbURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is required but not set"))
	}

	// Read Timeout
	readTimeout := 5 * time.Second
	if rt := get("APP_READ_TIMEOUT"); rt != "" {
		d, err := time.ParseDuration(rt)
		if err != nil || d <= 0 {
			errs = append(errs, fmt.Errorf("invalid APP_READ_TIMEOUT '%s': %w", rt, err))
		} else {
			readTimeout = d
		}
	}

	// Write Timeout
	writeTimeout := 10 * time.Second
	if wt := get("APP_WRITE_TIMEOUT"); wt != "" {
		d, err := time.ParseDuration(wt)
		if err != nil || d <= 0 {
			errs = append(errs, fmt.Errorf("invalid APP_WRITE_TIMEOUT '%s': %w", wt, err))
		} else {
			writeTimeout = d
		}
	}

	// Max Workers
	workers := 10
	if wStr := get("APP_MAX_WORKERS"); wStr != "" {
		w, err := strconv.Atoi(wStr)
		if err != nil || w <= 0 {
			errs = append(errs, fmt.Errorf("invalid APP_MAX_WORKERS '%s': must be > 0", wStr))
		} else {
			workers = w
		}
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("config validation failed: %w", errors.Join(errs...))
	}

	return &ServerConfig{
		Env:          env,
		Port:         port,
		DatabaseURL:  dbURL,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		MaxWorkers:   workers,
	}, nil
}
