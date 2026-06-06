package observability

import (
	"io"
	"log/slog"
)

// Logger wraps slog.Logger with game-context helper methods.
// It is safe for concurrent use.
type Logger struct {
	inner *slog.Logger
}

// NewLogger returns a Logger that emits structured JSON lines to w.
// The underlying handler is configured at LevelDebug so all levels are
// emitted; callers choose level via Info/Warn/Error.
func NewLogger(w io.Writer) *Logger {
	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	return &Logger{inner: slog.New(handler)}
}

// With returns a child Logger with the supplied permanent key-value fields
// attached to every subsequent log line.
func (l *Logger) With(fields ...any) *Logger {
	return &Logger{inner: l.inner.With(fields...)}
}

// Info emits a structured log line at INFO level.
// event is used as the value of the "event" key and is prepended to fields.
func (l *Logger) Info(event string, fields ...any) {
	l.inner.Info(event, fields...)
}

// Warn emits a structured log line at WARN level.
func (l *Logger) Warn(event string, fields ...any) {
	l.inner.Warn(event, fields...)
}

// Error emits a structured log line at ERROR level.
func (l *Logger) Error(event string, fields ...any) {
	l.inner.Error(event, fields...)
}

// GameFields returns a flat slice of alternating key-value pairs for
// game context, compatible with slog's variadic field convention.
// Keys: "game_id", "timeline_id", "node_id", "turn_number".
func GameFields(gameID, timelineID, nodeID string, seq int) []any {
	return []any{
		"game_id", gameID,
		"timeline_id", timelineID,
		"node_id", nodeID,
		"turn_number", seq,
	}
}
