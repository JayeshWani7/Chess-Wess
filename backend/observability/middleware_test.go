package observability_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ChessWess/backend/observability"
	"github.com/prometheus/client_golang/prometheus"
)

// countHTTPRequestObservations gathers metrics from the registry and returns
// the total number of observations recorded for chesswess_http_requests_total.
func countHTTPRequestObservations(t *testing.T, g prometheus.Gatherer) int {
	t.Helper()
	mfs, err := g.Gather()
	if err != nil {
		t.Fatalf("Gather() failed: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() != "chesswess_http_requests_total" {
			continue
		}
		count := 0
		for _, m := range mf.GetMetric() {
			if c := m.GetCounter(); c != nil {
				count += int(c.GetValue())
			}
		}
		return count
	}
	return 0
}

// findHTTPStatusLabel gathers metrics and returns the status_code label value
// for chesswess_http_requests_total filtered by route and method.
// Returns ("", false) if no matching sample is found.
func findHTTPStatusLabel(t *testing.T, g prometheus.Gatherer, route, method string) (string, bool) {
	t.Helper()
	mfs, err := g.Gather()
	if err != nil {
		t.Fatalf("Gather() failed: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() != "chesswess_http_requests_total" {
			continue
		}
		for _, m := range mf.GetMetric() {
			var gotRoute, gotMethod, gotStatus string
			for _, lp := range m.GetLabel() {
				switch lp.GetName() {
				case "route":
					gotRoute = lp.GetValue()
				case "method":
					gotMethod = lp.GetValue()
				case "status_code":
					gotStatus = lp.GetValue()
				}
			}
			if gotRoute == route && gotMethod == method {
				return gotStatus, true
			}
		}
	}
	return "", false
}

// TestInstrumentHandler_ExplicitStatus404 verifies that when the inner handler
// calls WriteHeader(404), the recorded status_code label is "404".
//
// Requirements: 5.3
func TestInstrumentHandler_ExplicitStatus404(t *testing.T) {
	promReg := prometheus.NewRegistry()
	reg := observability.New(promReg)

	route := "/test"
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	handler := observability.InstrumentHandler(reg, route, inner)

	req := httptest.NewRequest(http.MethodGet, route, nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	statusCode, found := findHTTPStatusLabel(t, promReg, route, http.MethodGet)
	if !found {
		t.Fatal("expected an observation for chesswess_http_requests_total but found none")
	}
	if statusCode != "404" {
		t.Errorf("recorded status_code = %q, want %q", statusCode, "404")
	}
}

// TestInstrumentHandler_NoStatusWritten verifies that when the inner handler
// never calls WriteHeader, the recorded status_code defaults to "200".
//
// Requirements: 5.5
func TestInstrumentHandler_NoStatusWritten(t *testing.T) {
	promReg := prometheus.NewRegistry()
	reg := observability.New(promReg)

	route := "/silent"
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Deliberately write no status code.
	})

	handler := observability.InstrumentHandler(reg, route, inner)

	req := httptest.NewRequest(http.MethodPost, route, nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	statusCode, found := findHTTPStatusLabel(t, promReg, route, http.MethodPost)
	if !found {
		t.Fatal("expected an observation for chesswess_http_requests_total but found none")
	}
	if statusCode != "200" {
		t.Errorf("recorded status_code = %q, want %q (default when no WriteHeader)", statusCode, "200")
	}
}

// TestInstrumentHandler_PanicPropagates verifies two properties when the inner
// handler panics:
//  1. The panic propagates out of InstrumentHandler unmodified.
//  2. No metric observation is recorded for that request.
//
// Requirements: 5.6
func TestInstrumentHandler_PanicPropagates(t *testing.T) {
	promReg := prometheus.NewRegistry()
	reg := observability.New(promReg)

	route := "/panic"
	panicValue := "intentional test panic"
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(panicValue)
	})

	handler := observability.InstrumentHandler(reg, route, inner)

	req := httptest.NewRequest(http.MethodGet, route, nil)
	rr := httptest.NewRecorder()

	// Verify the panic re-propagates.
	var recovered interface{}
	func() {
		defer func() {
			recovered = recover()
		}()
		handler.ServeHTTP(rr, req)
	}()

	if recovered == nil {
		t.Fatal("expected panic to propagate but recover() returned nil")
	}
	if recovered != panicValue {
		t.Errorf("recovered panic value = %v, want %v", recovered, panicValue)
	}

	// Verify no metric observation was recorded.
	count := countHTTPRequestObservations(t, promReg)
	if count != 0 {
		t.Errorf("expected 0 metric observations after panic, got %d", count)
	}
}
