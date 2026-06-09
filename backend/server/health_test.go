package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/quick"
	"time"
)

// ---------------------------------------------------------------------------
// Testable health logic extracted from handleHealth.
//
// Rather than constructing a full *Server (which requires a live pgxpool and
// redis client), we extract the two pure pieces of the health handler that
// the properties actually care about:
//
//  1. determineHealthStatus — the status/code decision rule
//  2. handleHealthFunc — the full handler re-expressed as a function that
//     accepts probe callbacks, so we can inject instant mock probes in tests.
// ---------------------------------------------------------------------------

// handleHealthFunc is a re-expression of handleHealth that accepts probe
// callbacks instead of using s.db / s.rdb directly.  It lets tests drive
// the probe outcomes without requiring a real database or Redis server.
func handleHealthFunc(
	pgProbe func(ctx context.Context) error,
	redisProbe func(ctx context.Context) error,
	activeConns int,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		results := make(chan probeResult, 3)

		// postgres probe
		go func() {
			start := time.Now()
			err := pgProbe(ctx)
			results <- probeResult{
				name:    "postgres",
				status:  statusFromErr(err),
				latency: time.Since(start),
				err:     err,
			}
		}()

		// redis probe
		go func() {
			start := time.Now()
			var err error
			if redisProbe == nil {
				err = fmt.Errorf("redis not configured")
			} else {
				err = redisProbe(ctx)
			}
			results <- probeResult{
				name:    "redis",
				status:  statusFromErr(err),
				latency: time.Since(start),
				err:     err,
			}
		}()

		// websocket_hub probe — in-process, zero I/O
		go func() {
			start := time.Now()
			results <- probeResult{
				name:    "websocket_hub",
				status:  "ok",
				latency: time.Since(start),
			}
		}()

		checks := make(map[string]ComponentCheck, 3)
		for i := 0; i < 3; i++ {
			res := <-results
			cc := ComponentCheck{
				Status:  res.status,
				Latency: formatLatency(res.latency),
			}
			if res.err != nil {
				cc.Error = res.err.Error()
			}
			checks[res.name] = cc
		}

		overallStatus, httpStatus := determineHealthStatus(
			checks["postgres"].Status == "error",
			checks["redis"].Status == "error",
		)

		resp := HealthResponse{
			Status:              overallStatus,
			Checks:              checks,
			ActiveWSConnections: activeConns,
			Timestamp:           time.Now().UTC().Format(time.RFC3339),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(httpStatus)
		_ = json.NewEncoder(w).Encode(resp)
	}
}

// ---------------------------------------------------------------------------
// Property 3: Health Response Timeliness
//
// For any pgErr / redisErr combination, when the probes return instantly
// (in-process closures with no I/O), the HTTP response must arrive in
// less than 100ms.
//
// **Property 3: Health Response Timeliness**
// **Validates: Requirements 3.2**
// ---------------------------------------------------------------------------

func TestHealthResponseTimeliness(t *testing.T) {
	f := func(pgErr bool, redisErr bool) bool {
		pgProbe := func(_ context.Context) error {
			if pgErr {
				return fmt.Errorf("postgres unavailable")
			}
			return nil
		}
		redisProbe := func(_ context.Context) error {
			if redisErr {
				return fmt.Errorf("redis unavailable")
			}
			return nil
		}

		handler := handleHealthFunc(pgProbe, redisProbe, 0)
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rr := httptest.NewRecorder()

		start := time.Now()
		handler.ServeHTTP(rr, req)
		elapsed := time.Since(start)

		if elapsed >= 100*time.Millisecond {
			t.Errorf("response took %v, want < 100ms (pgErr=%v, redisErr=%v)",
				elapsed, pgErr, redisErr)
			return false
		}
		return true
	}

	cfg := &quick.Config{MaxCount: 200}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("property 3 (health response timeliness) failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Property 4: Redis Degradation Safety
//
// For any pgErr=false / redisErr=true, the response MUST be HTTP 200 with
// status "degraded" — never HTTP 503.
//
// **Property 4: Redis Degradation Safety**
// **Validates: Requirements 3.5**
// ---------------------------------------------------------------------------

func TestHealthRedisDegradation(t *testing.T) {
	// Pure function test — deterministic, no network.
	f := func(redisErr bool) bool {
		// pgErr=false, redisErr=<generated>
		statusStr, code := determineHealthStatus(false, redisErr)

		if redisErr {
			// Redis error with pg healthy → degraded 200
			if code != http.StatusOK {
				t.Errorf("pgErr=false, redisErr=true: got HTTP %d, want 200", code)
				return false
			}
			if statusStr != "degraded" {
				t.Errorf("pgErr=false, redisErr=true: got status %q, want \"degraded\"", statusStr)
				return false
			}
		} else {
			// No errors → ok 200
			if code != http.StatusOK {
				t.Errorf("pgErr=false, redisErr=false: got HTTP %d, want 200", code)
				return false
			}
			if statusStr != "ok" {
				t.Errorf("pgErr=false, redisErr=false: got status %q, want \"ok\"", statusStr)
				return false
			}
		}
		return true
	}

	cfg := &quick.Config{MaxCount: 500}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("property 4 (redis degradation safety) failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Property 5: DB Probe Criticality
//
// For any pgErr=true (regardless of redisErr), the response MUST be HTTP 503
// with status "unhealthy".
//
// **Property 5: DB Probe Criticality**
// **Validates: Requirements 3.4**
// ---------------------------------------------------------------------------

func TestHealthDBProbeCriticality(t *testing.T) {
	// Pure function test — deterministic, no network.
	f := func(redisErr bool) bool {
		// pgErr=true, redisErr=<generated>
		statusStr, code := determineHealthStatus(true, redisErr)

		if code != http.StatusServiceUnavailable {
			t.Errorf("pgErr=true, redisErr=%v: got HTTP %d, want 503", redisErr, code)
			return false
		}
		if statusStr != "unhealthy" {
			t.Errorf("pgErr=true, redisErr=%v: got status %q, want \"unhealthy\"", redisErr, statusStr)
			return false
		}
		return true
	}

	cfg := &quick.Config{MaxCount: 500}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("property 5 (DB probe criticality) failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Integration property test: all three properties together via HTTP layer
//
// Uses handleHealthFunc (mock probes) and inspects the decoded JSON body to
// verify the status field and HTTP code obey all three rules simultaneously.
// ---------------------------------------------------------------------------

func TestHealthStatusRulesIntegration(t *testing.T) {
	f := func(pgErr bool, redisErr bool) bool {
		pgProbe := func(_ context.Context) error {
			if pgErr {
				return fmt.Errorf("postgres down")
			}
			return nil
		}
		redisProbe := func(_ context.Context) error {
			if redisErr {
				return fmt.Errorf("redis down")
			}
			return nil
		}

		handler := handleHealthFunc(pgProbe, redisProbe, 5)
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rr := httptest.NewRecorder()

		start := time.Now()
		handler.ServeHTTP(rr, req)
		elapsed := time.Since(start)

		// Timeliness check (Property 3)
		if elapsed >= 100*time.Millisecond {
			t.Errorf("response took %v ≥ 100ms (pgErr=%v, redisErr=%v)", elapsed, pgErr, redisErr)
			return false
		}

		result := rr.Result()
		defer result.Body.Close()

		var body HealthResponse
		if err := json.NewDecoder(result.Body).Decode(&body); err != nil {
			t.Errorf("failed to decode JSON body: %v", err)
			return false
		}

		// Check all three component keys are present
		for _, key := range []string{"postgres", "redis", "websocket_hub"} {
			if _, ok := body.Checks[key]; !ok {
				t.Errorf("missing checks key %q", key)
				return false
			}
		}

		// active_ws_connections should match what we passed in
		if body.ActiveWSConnections != 5 {
			t.Errorf("active_ws_connections = %d, want 5", body.ActiveWSConnections)
			return false
		}

		// Property 5: pgErr=true → 503 + "unhealthy"
		if pgErr {
			if result.StatusCode != http.StatusServiceUnavailable {
				t.Errorf("pgErr=true: HTTP %d, want 503", result.StatusCode)
				return false
			}
			if body.Status != "unhealthy" {
				t.Errorf("pgErr=true: status=%q, want \"unhealthy\"", body.Status)
				return false
			}
			return true
		}

		// Property 4: pgErr=false, redisErr=true → 200 + "degraded"
		if redisErr {
			if result.StatusCode != http.StatusOK {
				t.Errorf("pgErr=false,redisErr=true: HTTP %d, want 200", result.StatusCode)
				return false
			}
			if body.Status != "degraded" {
				t.Errorf("pgErr=false,redisErr=true: status=%q, want \"degraded\"", body.Status)
				return false
			}
			return true
		}

		// Both healthy → 200 + "ok"
		if result.StatusCode != http.StatusOK {
			t.Errorf("all ok: HTTP %d, want 200", result.StatusCode)
			return false
		}
		if body.Status != "ok" {
			t.Errorf("all ok: status=%q, want \"ok\"", body.Status)
			return false
		}
		return true
	}

	cfg := &quick.Config{MaxCount: 300}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("integration property test (health status rules) failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Table-driven unit tests covering the three canonical rules explicitly
// ---------------------------------------------------------------------------

func TestHealthStatusRulesTableDriven(t *testing.T) {
	cases := []struct {
		name         string
		pgErr        bool
		redisErr     bool
		wantStatus   string
		wantHTTPCode int
	}{
		{"both ok", false, false, "ok", http.StatusOK},
		{"redis error only", false, true, "degraded", http.StatusOK},
		{"pg error only", true, false, "unhealthy", http.StatusServiceUnavailable},
		{"both errors", true, true, "unhealthy", http.StatusServiceUnavailable},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			gotStatus, gotCode := determineHealthStatus(tc.pgErr, tc.redisErr)
			if gotStatus != tc.wantStatus {
				t.Errorf("status = %q, want %q", gotStatus, tc.wantStatus)
			}
			if gotCode != tc.wantHTTPCode {
				t.Errorf("code = %d, want %d", gotCode, tc.wantHTTPCode)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Edge case: nil redis probe behaves like a redis error (degraded, not 503)
// ---------------------------------------------------------------------------

func TestHealthNilRedisTreatedAsDegraded(t *testing.T) {
	// nil redisProbe → handler uses fmt.Errorf("redis not configured")
	pgProbe := func(_ context.Context) error { return nil }
	handler := handleHealthFunc(pgProbe, nil, 0)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	result := rr.Result()
	defer result.Body.Close()

	if result.StatusCode != http.StatusOK {
		t.Errorf("nil redis: HTTP %d, want 200", result.StatusCode)
	}

	var body HealthResponse
	if err := json.NewDecoder(result.Body).Decode(&body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if body.Status != "degraded" {
		t.Errorf("nil redis: status=%q, want \"degraded\"", body.Status)
	}
}
