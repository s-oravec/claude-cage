// Package logging configures cage's global slog logger and exposes helpers
// that translate the active verbosity into flags for the subprocesses cage
// orchestrates (ssh, virsh, virtiofsd, ...).
//
// Precedence for resolving the level:
//
//	--log-level flag  >  CAGE_LOG_LEVEL env  >  -v count  >  info (default)
package logging

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// LevelTrace sits below slog.LevelDebug. At trace, cage propagates verbosity
// into child processes (e.g. ssh -o LogLevel=DEBUG3) — at debug it only
// affects cage's own logging.
const LevelTrace slog.Level = -8

// EnvVar is the environment variable that overrides the default level.
const EnvVar = "CAGE_LOG_LEVEL"

var current = slog.LevelInfo

// Configure sets the global slog default handler from CLI inputs.
// verbosity is the count of -v occurrences; levelFlag is the value of
// --log-level (empty if unset). Reads CAGE_LOG_LEVEL from the environment.
func Configure(verbosity int, levelFlag string) error {
	lvl, err := resolveLevel(verbosity, levelFlag, os.Getenv(EnvVar))
	if err != nil {
		return err
	}
	current = lvl

	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: lvl,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			switch a.Key {
			case slog.TimeKey:
				return slog.Attr{}
			case slog.LevelKey:
				if l, ok := a.Value.Any().(slog.Level); ok && l == LevelTrace {
					return slog.String(slog.LevelKey, "TRACE")
				}
			}
			return a
		},
	})
	slog.SetDefault(slog.New(handler))
	return nil
}

// Level returns the currently active log level. Useful for callers that
// need to gate expensive work behind verbosity.
func Level() slog.Level { return current }

// SSHLogLevel returns the value for ssh's "-o LogLevel=..." matching the
// active verbosity. ssh is normally pinned to ERROR to keep interactive
// sessions clean; at debug/trace we let it speak.
func SSHLogLevel() string {
	switch {
	case current <= LevelTrace:
		return "DEBUG3"
	case current <= slog.LevelDebug:
		return "VERBOSE"
	default:
		return "ERROR"
	}
}

func resolveLevel(verbosity int, flag, env string) (slog.Level, error) {
	if flag != "" {
		return parseLevel(flag)
	}
	if env != "" {
		return parseLevel(env)
	}
	switch {
	case verbosity >= 2:
		return LevelTrace, nil
	case verbosity == 1:
		return slog.LevelDebug, nil
	default:
		return slog.LevelInfo, nil
	}
}

func parseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "trace":
		return LevelTrace, nil
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("invalid log level %q (valid: trace, debug, info, warn, error)", s)
	}
}
