package logging

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveLevel_Precedence(t *testing.T) {
	tests := []struct {
		name      string
		verbosity int
		flag      string
		env       string
		want      slog.Level
	}{
		{"default is info", 0, "", "", slog.LevelInfo},
		{"-v promotes to debug", 1, "", "", slog.LevelDebug},
		{"-vv promotes to trace", 2, "", "", LevelTrace},
		{"-vvv saturates at trace", 5, "", "", LevelTrace},
		{"env overrides verbosity count", 1, "", "warn", slog.LevelWarn},
		{"flag overrides env and count", 2, "error", "debug", slog.LevelError},
		{"flag accepts trace", 0, "trace", "", LevelTrace},
		{"env accepts warning alias", 0, "", "warning", slog.LevelWarn},
		{"casing is irrelevant", 0, "DEBUG", "", slog.LevelDebug},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveLevel(tc.verbosity, tc.flag, tc.env)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestResolveLevel_InvalidFlag(t *testing.T) {
	_, err := resolveLevel(0, "loud", "")
	assert.Error(t, err)
}

func TestResolveLevel_InvalidEnv(t *testing.T) {
	_, err := resolveLevel(0, "", "chatty")
	assert.Error(t, err)
}

func TestSSHLogLevel(t *testing.T) {
	tests := []struct {
		level slog.Level
		want  string
	}{
		{slog.LevelInfo, "ERROR"},
		{slog.LevelWarn, "ERROR"},
		{slog.LevelDebug, "VERBOSE"},
		{LevelTrace, "DEBUG3"},
	}
	original := current
	t.Cleanup(func() { current = original })

	for _, tc := range tests {
		current = tc.level
		assert.Equal(t, tc.want, SSHLogLevel(), "level=%v", tc.level)
	}
}

func TestConfigure_AppliesLevel(t *testing.T) {
	original := current
	t.Cleanup(func() { current = original })

	require.NoError(t, Configure(2, ""))
	assert.Equal(t, LevelTrace, Level())

	require.NoError(t, Configure(0, "warn"))
	assert.Equal(t, slog.LevelWarn, Level())
}
