package observability_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ChessWess/backend/observability"
)

// logLine parses a single JSON log line emitted by Logger into a map.
func logLine(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	line := strings.TrimSpace(buf.String())
	if line == "" {
		t.Fatal("no log output written")
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		t.Fatalf("log line is not valid JSON: %v\nraw: %s", err, line)
	}
	return m
}

// TestNewLogger_JSONOutput verifies that NewLogger emits a valid JSON object.
func TestNewLogger_JSONOutput(t *testing.T) {
	var buf bytes.Buffer
	l := observability.NewLogger(&buf)
	l.Info("test_event")

	m := logLine(t, &buf)
	if _, ok := m["time"]; !ok {
		t.Error("expected 'time' field in log output")
	}
	if _, ok := m["level"]; !ok {
		t.Error("expected 'level' field in log output")
	}
}

// TestLogger_EventKey verifies that the "event" key is present and "msg" is absent.
// Validates: Requirement 8.2 — every log line must have an "event" field, not "msg".
func TestLogger_EventKey(t *testing.T) {
	var buf bytes.Buffer
	l := observability.NewLogger(&buf)
	l.Info("move_applied")

	m := logLine(t, &buf)

	event, ok := m["event"]
	if !ok {
		t.Fatalf("expected 'event' key in log output, got keys: %v", mapKeys(m))
	}
	if event != "move_applied" {
		t.Errorf("event = %q, want %q", event, "move_applied")
	}
	if _, hasMSG := m["msg"]; hasMSG {
		t.Error("unexpected 'msg' key in log output; should be renamed to 'event'")
	}
}

// TestLogger_LevelFields verifies that Info/Warn/Error emit the correct level strings.
func TestLogger_LevelFields(t *testing.T) {
	levels := []struct {
		logFn func(*observability.Logger, string, ...any)
		want  string
	}{
		{(*observability.Logger).Info, "INFO"},
		{(*observability.Logger).Warn, "WARN"},
		{(*observability.Logger).Error, "ERROR"},
	}

	for _, tc := range levels {
		t.Run(tc.want, func(t *testing.T) {
			var buf bytes.Buffer
			l := observability.NewLogger(&buf)
			tc.logFn(l, "some_event")
			m := logLine(t, &buf)
			if m["level"] != tc.want {
				t.Errorf("level = %v, want %q", m["level"], tc.want)
			}
		})
	}
}

// TestLogger_ExtraFields verifies that additional key-value pairs are included.
func TestLogger_ExtraFields(t *testing.T) {
	var buf bytes.Buffer
	l := observability.NewLogger(&buf)
	l.Info("move_applied", "user_id", "u-alice", "uci", "e2e4")

	m := logLine(t, &buf)
	if m["user_id"] != "u-alice" {
		t.Errorf("user_id = %v, want %q", m["user_id"], "u-alice")
	}
	if m["uci"] != "e2e4" {
		t.Errorf("uci = %v, want %q", m["uci"], "e2e4")
	}
}

// TestLogger_With verifies that With attaches permanent fields to child loggers.
func TestLogger_With(t *testing.T) {
	var buf bytes.Buffer
	l := observability.NewLogger(&buf).With("game_id", "g-123")
	l.Info("some_event")

	m := logLine(t, &buf)
	if m["game_id"] != "g-123" {
		t.Errorf("game_id = %v, want %q", m["game_id"], "g-123")
	}
}

// TestLogger_WithDoesNotMutateParent verifies that With returns an independent child.
func TestLogger_WithDoesNotMutateParent(t *testing.T) {
	var buf bytes.Buffer
	parent := observability.NewLogger(&buf)
	child := parent.With("child_key", "child_val")

	// parent log — should NOT have child_key
	parent.Info("parent_event")
	parentLine := logLine(t, &buf)
	if _, ok := parentLine["child_key"]; ok {
		t.Error("parent logger should not have 'child_key' set by child With()")
	}

	// child log — should have child_key
	buf.Reset()
	child.Info("child_event")
	childLine := logLine(t, &buf)
	if childLine["child_key"] != "child_val" {
		t.Errorf("child_key = %v, want %q", childLine["child_key"], "child_val")
	}
}

// TestGameFields verifies the shape and values of GameFields output.
// Validates: Requirement 8.7 — keys "game_id", "timeline_id", "node_id", "turn_number".
func TestGameFields(t *testing.T) {
	fields := observability.GameFields("g-1", "tl-2", "nd-3", 7)

	if len(fields) != 8 {
		t.Fatalf("GameFields returned %d elements, want 8 (4 key-value pairs)", len(fields))
	}

	// Verify keys and values in order.
	expected := []any{
		"game_id", "g-1",
		"timeline_id", "tl-2",
		"node_id", "nd-3",
		"turn_number", 7,
	}
	for i, v := range expected {
		if fields[i] != v {
			t.Errorf("fields[%d] = %v (%T), want %v (%T)", i, fields[i], fields[i], v, v)
		}
	}
}

// TestGameFields_IntegrationWithLogger verifies GameFields works with Logger.Info.
func TestGameFields_IntegrationWithLogger(t *testing.T) {
	var buf bytes.Buffer
	l := observability.NewLogger(&buf)
	l.Info("move_applied", observability.GameFields("g-abc", "tl-xyz", "nd-001", 3)...)

	m := logLine(t, &buf)

	checks := map[string]any{
		"game_id":     "g-abc",
		"timeline_id": "tl-xyz",
		"node_id":     "nd-001",
		"turn_number": float64(3), // JSON numbers decode as float64
	}
	for k, want := range checks {
		got, ok := m[k]
		if !ok {
			t.Errorf("missing key %q in log output", k)
			continue
		}
		if got != want {
			t.Errorf("%s = %v (%T), want %v (%T)", k, got, got, want, want)
		}
	}
}

// mapKeys returns the keys of a map as a slice (for error messages).
func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
