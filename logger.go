package spork

import (
	"log"
	"os"
)

// Logger is the minimal logging interface the SDK uses for internal events
// (retries, rate-limit sleeps, etc). It is intentionally small so it can be
// satisfied by the standard library, slog, zerolog, zap, and most custom
// loggers without a wrapper.
//
// The SDK never logs request or response bodies at Info level. To capture
// full HTTP traffic for debugging, install an HTTP middleware via
// WithHTTPMiddleware — the CLI's --debug flag uses exactly that hook.
type Logger interface {
	// Debug is used for verbose internal events (per-retry sleep durations,
	// auto-pagination page transitions, rate-limit header snapshots). No
	// request or response bodies are logged here.
	Debug(format string, args ...any)
	// Info is used for events the caller might plausibly want to surface
	// (e.g., "retrying after 429, waited 2s"). Still no bodies.
	Info(format string, args ...any)
	// Warn is used for events that succeeded but deserve attention (e.g.,
	// "retried 3 times before succeeding").
	Warn(format string, args ...any)
}

// nopLogger is the default when the caller has not set WithLogger.
type nopLogger struct{}

func (nopLogger) Debug(string, ...any) {}
func (nopLogger) Info(string, ...any)  {}
func (nopLogger) Warn(string, ...any)  {}

// StdLogger is a convenience Logger built on the standard log package.
// It writes Debug/Info/Warn with a level prefix to the provided destination.
// Pass nil to write to os.Stderr.
func StdLogger(l *log.Logger) Logger {
	if l == nil {
		l = log.New(os.Stderr, "spork: ", log.LstdFlags)
	}
	return stdLogger{l}
}

type stdLogger struct{ l *log.Logger }

func (s stdLogger) Debug(format string, args ...any) { s.l.Printf("DEBUG "+format, args...) }
func (s stdLogger) Info(format string, args ...any)  { s.l.Printf("INFO  "+format, args...) }
func (s stdLogger) Warn(format string, args ...any)  { s.l.Printf("WARN  "+format, args...) }
