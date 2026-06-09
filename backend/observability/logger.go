package observability

import (
	"io"
	"log/slog"
)

// Logger wraps slog.Logger with game-context helper methods.
// It is safe for concurrent use (slog.Logger is safe for concurrent use).
type Logger struct {
	inner *slog.Logger
}

// NewLogger returns a Logger that emits structured JSON lines to w.
// Every line is a single JSON object containing at minimum "time" (RFC3339),
// "level", and "event" fields. The underlying handler is configured at
// LevelDebug so all levels are emitted; callers choose level via
// Info/Warn/Error.
//
// The slog message key ("msg") is renamed to "event" via ReplaceAttr so
// that every log line carries the field name required by Requirement 8.2.
func NewLogger(w io.Writer) *Logger {
	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: slog.LevelDebug,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Rename the built-in "msg" key to "event" so callers see
			// {"event": "move_applied", ...} instead of {"msg": "move_applied", ...}.
			if len(groups) == 0 && a.Key == slog.MessageKey {
				a.Key = "event"
			}
			return a
		},
	})
	return &Logger{inner: slog.New(handler)}
}

// With returns a child Logger with the supplied permanent key-value fields
// attached to every subsequent log line.
func (l *Logger) With(fields ...any) *Logger {
	return &Logger{inner: l.inner.With(fields...)}
}

// Info emits a structured log line at INFO level.
// event becomes the value of the "event" key (the renamed slog message field).
// It is always the first application-level field in the JSON object after
// "time" and "level".
func (l *Logger) Info(event string, fields ...any) {
	l.inner.Info(event, fields...)
}

// Warn emits a structured log line at WARN level.
// event becomes the value of the "event" key.
func (l *Logger) Warn(event string, fields ...any) {
	l.inner.Warn(event, fields...)
}

// Error emits a structured log line at ERROR level.
// event becomes the value of the "event" key.
func (l *Logger) Error(event string, fields ...any) {
	l.inner.Error(event, fields...)
}

// GameFields returns a flat slice of alternating key-value pairs for
// game context, compatible with slog's variadic field convention.
// Keys: "game_id", "timeline_id", "node_id", "turn_number".
//
// Example usage:
//
//	s.log.Info("move_applied", observability.GameFields(gameID, tlID, nodeID, turn)...)
func GameFields(gameID, timelineID, nodeID string, seq int) []any {
	return []any{
		"game_id", gameID,
		"timeline_id", timelineID,
		"node_id", nodeID,
		"turn_number", seq,
	}
}
