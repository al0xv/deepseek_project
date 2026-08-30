// Package config reads configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"time"
)

// APIKey returns the DeepSeek API key from DEEPSEEK_API_KEY.
// It returns an error when the variable is empty.
func APIKey() (string, error) {
	k := os.Getenv("DEEPSEEK_API_KEY")
	if k == "" {
		return "", errors.New("DEEPSEEK_API_KEY is not set")
	}
	return k, nil
}

// Model returns the DeepSeek model name from DS_MODEL, default "deepseek-chat".
func Model() string {
	if m := os.Getenv("DS_MODEL"); m != "" {
		return m
	}
	return "deepseek-chat"
}

// GatewayURL returns the gateway base URL from DS_GATEWAY_URL,
// default "http://localhost:8080".
func GatewayURL() string {
	if u := os.Getenv("DS_GATEWAY_URL"); u != "" {
		return u
	}
	return "http://localhost:8080"
}

// GatewayAddr returns the listen address from DS_GATEWAY_ADDR.
// The safe default binds only the loopback interface; use -listen (see
// ListenAddr) to expose the gateway on the LAN for cross-device tests.
func GatewayAddr() string {
	if a := os.Getenv("DS_GATEWAY_ADDR"); a != "" {
		return a
	}
	return "127.0.0.1:8080"
}

// ListenAddr resolves the final listen address. Precedence:
//  1. explicit CLI flag (e.g. "-listen 0.0.0.0:8080"),
//  2. DS_GATEWAY_ADDR environment variable,
//  3. loopback-only default "127.0.0.1:8080".
func ListenAddr(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	return GatewayAddr()
}

// Duration returns the parsed duration for key, or def when the variable is
// missing or invalid.
func Duration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return def
}

// Int returns the parsed integer for key, or def when missing or invalid.
func Int(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			return n
		}
	}
	return def
}
