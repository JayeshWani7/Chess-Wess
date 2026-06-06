package observability

import (
	"net/http"
	"time"
)

// responseWriter wraps http.ResponseWriter to capture the first status code
// written by the inner handler, defaulting to 200 if WriteHeader is never called.
type responseWriter struct {
	http.ResponseWriter
	statusCode  int
	wroteHeader bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.wroteHeader {
		rw.statusCode = code
		rw.wroteHeader = true
	}
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

// InstrumentHandler wraps h, recording per-route request counts and latencies
// into obs. route is a low-cardinality label, e.g. "/api/games".
//
// When the inner handler panics, the panic propagates unmodified and no metric
// observation is recorded for that request.
func InstrumentHandler(obs *Registry, route string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		start := time.Now()

		// panicked tracks whether the inner handler panicked. It starts true and
		// is set to false only after ServeHTTP returns normally, ensuring that
		// any panic path skips the metric observation.
		panicked := true
		defer func() {
			if !panicked {
				obs.RecordHTTPRequest(route, r.Method, rw.statusCode, time.Since(start))
			}
			// re-panic is handled automatically since we do not recover here.
		}()

		h.ServeHTTP(rw, r)
		panicked = false
	})
}
