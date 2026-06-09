package observability_test

// Property 6: Logger Concurrency Safety
//
// For any set of concurrent goroutines calling Logger.Info, each log write
// produces a complete, well-formed JSON object with no interleaving of bytes
// from other concurrent writes.
//
// **Validates: Requirements 1.7, 8.1**

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"testing/quick"

	"github.com/ChessWess/backend/observability"
)

// safeBuf is a mutex-protected bytes.Buffer. bytes.Buffer is NOT safe for
// concurrent writes, so we wrap it to allow multiple goroutines to log into
// the same buffer without data races.
type safeBuf struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *safeBuf) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *safeBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// TestLoggerConcurrencySafety_Property6 fires 100 concurrent goroutines each
// calling Logger.Info with random field pairs. It asserts that every
// newline-delimited line in the output is a complete, parseable JSON object —
// proving no partial writes or byte interleaving occurred.
//
// **Property 6: Logger Concurrency Safety**
// **Validates: Requirements 1.7, 8.1**
func TestLoggerConcurrencySafety_Property6(t *testing.T) {
	const numGoroutines = 100

	// property is the function testing/quick will call with random inputs.
	// key0/val0 … key3/val3 provide varied field pairs across runs.
	property := func(
		key0, val0 string,
		key1, val1 string,
		key2, val2 string,
		key3, val3 string,
	) bool {
		// Sanitise empty keys — slog requires non-empty keys for well-formed JSON.
		sanitise := func(k string) string {
			if k == "" {
				return "k"
			}
			return k
		}
		k0, k1, k2, k3 := sanitise(key0), sanitise(key1), sanitise(key2), sanitise(key3)

		sb := &safeBuf{}
		logger := observability.NewLogger(sb)

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(idx int) {
				defer wg.Done()
				event := fmt.Sprintf("concurrent_event_%d", idx)
				logger.Info(event,
					k0, val0,
					k1, val1,
					k2, val2,
					k3, val3,
					"goroutine_index", idx,
				)
			}(i)
		}

		wg.Wait()

		// Parse every newline-delimited line as a JSON object. Any partial
		// write or byte interleaving would cause json.Unmarshal to fail.
		output := sb.String()
		scanner := bufio.NewScanner(bytes.NewBufferString(output))
		lineCount := 0
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			lineCount++
			var obj map[string]any
			if err := json.Unmarshal([]byte(line), &obj); err != nil {
				// Line is not valid JSON — partial write or interleaving detected.
				return false
			}
			// Each line must have the mandatory fields required by Req 8.2.
			if _, ok := obj["time"]; !ok {
				return false
			}
			if _, ok := obj["level"]; !ok {
				return false
			}
			if _, ok := obj["event"]; !ok {
				return false
			}
		}

		// Every goroutine must have produced exactly one log line.
		if lineCount != numGoroutines {
			return false
		}

		return true
	}

	cfg := &quick.Config{MaxCount: 20}
	if err := quick.Check(property, cfg); err != nil {
		t.Errorf("Logger concurrency safety property violated: %v", err)
	}
}

// TestLoggerConcurrencySafety_AllLevels verifies that concurrent calls to
// Info, Warn, and Error all produce complete, parseable JSON lines with no
// data races. Uses a fixed number of iterations for reliability.
//
// **Property 6: Logger Concurrency Safety (all levels)**
// **Validates: Requirements 1.7, 8.1**
func TestLoggerConcurrencySafety_AllLevels(t *testing.T) {
	const goroutinesPerLevel = 33 // ~100 total across 3 levels

	sb := &safeBuf{}
	logger := observability.NewLogger(sb)

	var wg sync.WaitGroup
	total := goroutinesPerLevel * 3
	wg.Add(total)

	for i := 0; i < goroutinesPerLevel; i++ {
		go func(i int) {
			defer wg.Done()
			logger.Info("info_event", "index", i)
		}(i)
		go func(i int) {
			defer wg.Done()
			logger.Warn("warn_event", "index", i)
		}(i)
		go func(i int) {
			defer wg.Done()
			logger.Error("error_event", "index", i)
		}(i)
	}

	wg.Wait()

	output := sb.String()
	scanner := bufio.NewScanner(bytes.NewBufferString(output))
	lineCount := 0
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		lineCount++
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Errorf("line %d is not valid JSON (possible byte interleaving): %v\nraw: %q", lineCount, err, line)
			continue
		}
		// Verify mandatory fields per Requirement 8.2.
		for _, field := range []string{"time", "level", "event"} {
			if _, ok := obj[field]; !ok {
				t.Errorf("line %d missing mandatory field %q", lineCount, field)
			}
		}
		// Verify msg key was renamed to event (Requirement 8.2).
		if _, hasMSG := obj["msg"]; hasMSG {
			t.Errorf("line %d has 'msg' key; expected it to be renamed to 'event'", lineCount)
		}
	}

	if lineCount != total {
		t.Errorf("expected %d log lines (one per goroutine), got %d — possible lost writes", total, lineCount)
	}
}
