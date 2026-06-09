package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// HealthResponse is the top-level JSON body returned by GET /health.
type HealthResponse struct {
	Status             string                    `json:"status"`
	Checks             map[string]ComponentCheck `json:"checks"`
	ActiveWSConnections int                      `json:"active_ws_connections"`
	Timestamp          string                    `json:"timestamp"`
}

// ComponentCheck describes the health of a single dependency.
type ComponentCheck struct {
	Status  string `json:"status"`
	Latency string `json:"latency"`
	Error   string `json:"error,omitempty"`
}

// probeResult carries the outcome of a single health probe goroutine.
type probeResult struct {
	name    string
	status  string
	latency time.Duration
	err     error
}

// handleHealth performs parallel dependency checks and returns a structured
// JSON health report.
//
// Overall status rules:
//   - "unhealthy" (503) if postgres is in error
//   - "degraded"  (200) if only redis is in error
//   - "ok"        (200) otherwise
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	results := make(chan probeResult, 3)

	// --- postgres probe ---
	go func() {
		start := time.Now()
		err := s.db.Ping(ctx)
		results <- probeResult{
			name:    "postgres",
			status:  statusFromErr(err),
			latency: time.Since(start),
			err:     err,
		}
	}()

	// --- redis probe ---
	go func() {
		if s.rdb == nil {
			results <- probeResult{
				name:   "redis",
				status: "error",
				err:    fmt.Errorf("redis not configured"),
			}
			return
		}
		start := time.Now()
		err := s.rdb.Ping(ctx).Err()
		results <- probeResult{
			name:    "redis",
			status:  statusFromErr(err),
			latency: time.Since(start),
			err:     err,
		}
	}()

	// --- websocket hub probe (in-process, zero I/O) ---
	go func() {
		start := time.Now()
		_ = s.hub.ActiveConnections() // just exercise the call
		results <- probeResult{
			name:    "websocket_hub",
			status:  "ok",
			latency: time.Since(start),
		}
	}()

	// Collect all three results.
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

	// Determine overall status.
	activeConns := s.hub.ActiveConnections()
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

// determineHealthStatus maps the postgres/redis error booleans to the overall
// status string and HTTP status code.
//
// Rules (per Requirements 3.3, 3.4, 3.5):
//   - pgErr=true                → "unhealthy" + 503
//   - pgErr=false, redisErr=true → "degraded"  + 200
//   - both false                 → "ok"        + 200
func determineHealthStatus(pgErr, redisErr bool) (status string, httpCode int) {
	if pgErr {
		return "unhealthy", http.StatusServiceUnavailable
	}
	if redisErr {
		return "degraded", http.StatusOK
	}
	return "ok", http.StatusOK
}

// statusFromErr maps a nil/non-nil error to "ok" / "error".
func statusFromErr(err error) string {
	if err != nil {
		return "error"
	}
	return "ok"
}

// formatLatency renders a duration as a compact human-readable string
// (e.g. "1.2ms", "0s").  time.Duration.String() already does this well.
func formatLatency(d time.Duration) string {
	return d.String()
}
