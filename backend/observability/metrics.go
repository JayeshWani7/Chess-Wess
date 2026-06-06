package observability

import (
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Registry struct {
	registerer prometheus.Registerer
	gatherer   prometheus.Gatherer

	// move_duration_seconds — histogram, labels: outcome
	moveDuration *prometheus.HistogramVec

	// move_validation_errors_total — counter, labels: reason
	moveErrors *prometheus.CounterVec

	// db_query_duration_seconds — histogram, labels: operation, table, status
	dbQueryDuration *prometheus.HistogramVec

	// ws_connections_active — gauge (no labels)
	wsConnectionsActive prometheus.Gauge

	// ws_disconnects_total — counter, labels: reason
	wsDisconnects *prometheus.CounterVec

	// http_requests_total — counter, labels: route, method, status_code
	httpRequestsTotal *prometheus.CounterVec

	// http_request_duration_seconds — histogram, labels: route, method
	httpRequestDuration *prometheus.HistogramVec
}

func New(reg prometheus.Registerer) *Registry {
	if reg == nil {
		panic("observability.New: prometheus.Registerer must not be nil")
	}

	// Obtain the matching Gatherer. If reg is a *prometheus.Registry it also
	// implements prometheus.Gatherer; otherwise fall back to the default gatherer.
	gatherer, ok := reg.(prometheus.Gatherer)
	if !ok {
		panic(fmt.Sprintf("observability.New: registerer of type %T does not implement prometheus.Gatherer; pass a *prometheus.Registry", reg))
	}

	moveDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "chesswess_move_duration_seconds",
			Help:    "End-to-end latency of a move operation.",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.0},
		},
		[]string{"outcome"},
	)

	moveErrors := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "chesswess_move_validation_errors_total",
			Help: "Count of move validation failures by reason.",
		},
		[]string{"reason"},
	)

	dbQueryDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "chesswess_db_query_duration_seconds",
			Help:    "Duration of database queries.",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5},
		},
		[]string{"operation", "table", "status"},
	)

	wsConnectionsActive := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "chesswess_ws_connections_active",
			Help: "Current number of active WebSocket connections.",
		},
	)

	wsDisconnects := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "chesswess_ws_disconnects_total",
			Help: "Total WebSocket disconnections by reason.",
		},
		[]string{"reason"},
	)

	httpRequestsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "chesswess_http_requests_total",
			Help: "Total HTTP requests by route, method, and status code.",
		},
		[]string{"route", "method", "status_code"},
	)

	httpRequestDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "chesswess_http_request_duration_seconds",
			Help:    "HTTP request latency by route and method.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"route", "method"},
	)

	reg.MustRegister(
		moveDuration,
		moveErrors,
		dbQueryDuration,
		wsConnectionsActive,
		wsDisconnects,
		httpRequestsTotal,
		httpRequestDuration,
	)

	return &Registry{
		registerer:          reg,
		gatherer:            gatherer,
		moveDuration:        moveDuration,
		moveErrors:          moveErrors,
		dbQueryDuration:     dbQueryDuration,
		wsConnectionsActive: wsConnectionsActive,
		wsDisconnects:       wsDisconnects,
		httpRequestsTotal:   httpRequestsTotal,
		httpRequestDuration: httpRequestDuration,
	}
}

// Handler returns an http.Handler that serves the Prometheus /metrics page,
// scoped to the instruments registered with this Registry's gatherer.
func (r *Registry) Handler() http.Handler {
	return promhttp.HandlerFor(r.gatherer, promhttp.HandlerOpts{})
}

// RecordMove records a completed (or failed) move attempt.
// outcome must be one of "success", "conflict", "illegal", or "error".
// game_id and timeline_id are intentionally not used as Prometheus labels
// to avoid high cardinality; they appear only in structured log lines.
func (r *Registry) RecordMove(gameID, timelineID, outcome string, latency time.Duration) {
	r.moveDuration.WithLabelValues(outcome).Observe(latency.Seconds())
}

// RecordMoveValidationError increments the move validation error counter.
// reason should be one of "missing_uci", "invalid_timeline_context",
// "illegal_move", "conflict", "game_not_active", or "invalid_timeline".
func (r *Registry) RecordMoveValidationError(reason string) {
	r.moveErrors.WithLabelValues(reason).Inc()
}

// TrackDBQuery records the latency of a database operation. It is intended to
// be called via defer immediately after a DB call:
//
//	defer s.obs.TrackDBQuery("query", "games", time.Now(), &err)
//
// The latency is computed from start to the moment TrackDBQuery executes.
// status is "ok" when errPtr is nil or *errPtr is nil, otherwise "error".
func (r *Registry) TrackDBQuery(op, table string, start time.Time, errPtr *error) {
	latency := time.Since(start)
	status := "ok"
	if errPtr != nil && *errPtr != nil {
		status = "error"
	}
	r.dbQueryDuration.WithLabelValues(op, table, status).Observe(latency.Seconds())
}

// RecordWSConnect increments the active WebSocket connections gauge.
func (r *Registry) RecordWSConnect() {
	r.wsConnectionsActive.Inc()
}

// RecordWSDisconnect decrements the active WebSocket connections gauge and
// increments the disconnects counter. reason should be "normal", "error",
// or "timeout".
func (r *Registry) RecordWSDisconnect(reason string) {
	r.wsConnectionsActive.Dec()
	r.wsDisconnects.WithLabelValues(reason).Inc()
}

// RecordHTTPRequest records a completed HTTP request.
// route is a low-cardinality label provided at registration time.
// statusCode is the HTTP response status code written by the handler.
func (r *Registry) RecordHTTPRequest(route, method string, statusCode int, latency time.Duration) {
	code := fmt.Sprintf("%d", statusCode)
	r.httpRequestsTotal.WithLabelValues(route, method, code).Inc()
	r.httpRequestDuration.WithLabelValues(route, method).Observe(latency.Seconds())
}
