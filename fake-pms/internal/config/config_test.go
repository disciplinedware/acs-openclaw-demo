package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var complete = []string{
	"PMS_LISTEN_ADDR=:8080", "PMS_BOOKINGS_FILE=/data/bookings.json", "PMS_API_KEY=k",
	"PMS_READ_HEADER_TIMEOUT=10s", "PMS_READ_TIMEOUT=30s", "PMS_WRITE_TIMEOUT=30s",
	"PMS_IDLE_TIMEOUT=2m", "PMS_SHUTDOWN_GRACE=10s", "PMS_RELOAD_INTERVAL=2s", "OTHER=ignored",
}

func environ(env []string) func() []string { return func() []string { return env } }

func TestLoad(t *testing.T) {
	c, err := Load(environ(complete))
	require.NoError(t, err)
	assert.Equal(t, ":8080", c.ListenAddr)
	assert.Equal(t, 2*time.Minute, c.IdleTimeout)
}

func TestLoadRefuses(t *testing.T) {
	cases := []struct {
		name    string
		env     []string
		wantErr string
	}{
		{name: "missing_key", env: complete[1:], wantErr: "PMS_LISTEN_ADDR"},
		{name: "not_a_duration", env: append(append([]string{}, complete...), "PMS_READ_TIMEOUT=soon"), wantErr: `PMS_READ_TIMEOUT="soon"`},
		{name: "zero_duration", env: append(append([]string{}, complete...), "PMS_SHUTDOWN_GRACE=0s"), wantErr: "PMS_SHUTDOWN_GRACE"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(environ(tc.env))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}
