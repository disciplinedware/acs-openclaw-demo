// Package config reads the fake PMS's settings from PMS_* environment
// variables. Every setting is required: compose.yaml declares each value.
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/v2"
)

type Config struct {
	ListenAddr        string
	BookingsFile      string
	APIKey            string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownGrace     time.Duration
	ReloadInterval    time.Duration
}

// Load reads PMS_LISTEN_ADDR, PMS_BOOKINGS_FILE, PMS_API_KEY and the server
// durations PMS_READ_HEADER_TIMEOUT, PMS_READ_TIMEOUT, PMS_WRITE_TIMEOUT,
// PMS_IDLE_TIMEOUT, PMS_SHUTDOWN_GRACE, and PMS_RELOAD_INTERVAL, how often the
// bookings file is checked for a change (Go durations, such as 10s).
func Load(environ func() []string) (*Config, error) {
	k := koanf.New(".")
	err := k.Load(env.Provider(".", env.Opt{
		Prefix:        "PMS_",
		EnvironFunc:   environ,
		TransformFunc: func(key, value string) (string, any) { return strings.ToLower(strings.TrimPrefix(key, "PMS_")), value },
	}), nil)
	if err != nil {
		return nil, err
	}
	var missing []string
	text := func(key string) string {
		v := k.String(key)
		if v == "" {
			missing = append(missing, "PMS_"+strings.ToUpper(key))
		}
		return v
	}
	var invalid []string
	duration := func(key string) time.Duration {
		v := text(key)
		if v == "" {
			return 0
		}
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			invalid = append(invalid, fmt.Sprintf("PMS_%s=%q is no positive duration", strings.ToUpper(key), v))
		}
		return d
	}
	c := &Config{
		ListenAddr:        text("listen_addr"),
		BookingsFile:      text("bookings_file"),
		APIKey:            text("api_key"),
		ReadHeaderTimeout: duration("read_header_timeout"),
		ReadTimeout:       duration("read_timeout"),
		WriteTimeout:      duration("write_timeout"),
		IdleTimeout:       duration("idle_timeout"),
		ShutdownGrace:     duration("shutdown_grace"),
		ReloadInterval:    duration("reload_interval"),
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("required settings are not set: %s", strings.Join(missing, ", "))
	}
	if len(invalid) > 0 {
		return nil, fmt.Errorf("invalid settings: %s", strings.Join(invalid, "; "))
	}
	return c, nil
}
