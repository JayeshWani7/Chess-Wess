package observability_test

import (
	"errors"
	"testing"
	"testing/quick"
	"time"

	"github.com/ChessWess/backend/observability"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/client_golang/prometheus"
)

// findStatusLabel searches gathered metric families for the
// chesswess_db_query_duration_seconds metric and returns the value of the
// "status" label on any observation whose "operation" and "table" labels
// match the given op and table arguments. It returns ("", false) when no
// matching sample is found.
func findStatusLabel(t *testing.T, g prometheus.Gatherer, op, table string) (string, bool) {
	t.Helper()
	mfs, err := g.Gather()
	if err != nil {
		t.Fatalf("Gather() failed: %v", err)
	}

	const metricName = "chesswess_db_query_duration_seconds"
	for _, mf := range mfs {
		if mf.GetName() != metricName {
			continue
		}
		for _, m := range mf.GetMetric() {
			var gotOp, gotTable, gotStatus string
			for _, lp := range m.GetLabel() {
				switch lp.GetName() {
				case "operation":
					gotOp = lp.GetValue()
				case "table":
					gotTable = lp.GetValue()
				case "status":
					gotStatus = lp.GetValue()
				}
			}
			if gotOp == op && gotTable == table {
				return gotStatus, true
			}
		}
	}
	return "", false
}

// labelPairsToMap converts a slice of dto.LabelPair to a map for easy lookup.
func labelNamesFromMetricFamily(mf *dto.MetricFamily) map[string]bool {
	names := make(map[string]bool)
	for _, m := range mf.GetMetric() {
		for _, lp := range m.GetLabel() {
			names[lp.GetName()] = true
		}
	}
	return names
}

// TestTrackDBQueryStatusLabelAccuracy is a property-based test verifying that
// the "status" label on db_query_duration_seconds is "ok" when errNil is true
// and "error" when errNil is false, for all arbitrary (op, table, errNil) triples.
//
// **Property 8: DB Query Status Label Accuracy**
// **Validates: Requirements 7.2, 7.3**
func TestTrackDBQueryStatusLabelAccuracy(t *testing.T) {
	f := func(op, table string, errNil bool) bool {
		// Use a fresh registry per iteration to avoid duplicate-registration panics
		// and to ensure Gather() returns only the current call's observation.
		promReg := prometheus.NewRegistry()
		reg := observability.New(promReg)

		start := time.Now()

		if errNil {
			// errPtr is nil — status must be "ok"
			reg.TrackDBQuery(op, table, start, nil)
		} else {
			// errPtr points to a non-nil error — status must be "error"
			someErr := errors.New("simulated db error")
			reg.TrackDBQuery(op, table, start, &someErr)
		}

		status, found := findStatusLabel(t, promReg, op, table)
		if !found {
			t.Logf("no observation found for op=%q table=%q", op, table)
			return false
		}

		if errNil {
			if status != "ok" {
				t.Logf("errNil=true but status=%q (want %q), op=%q table=%q", status, "ok", op, table)
				return false
			}
		} else {
			if status != "error" {
				t.Logf("errNil=false but status=%q (want %q), op=%q table=%q", status, "error", op, table)
				return false
			}
		}

		return true
	}

	cfg := &quick.Config{MaxCount: 200}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Property 8 (DB Query Status Label Accuracy) failed: %v", err)
	}
}

// TestTrackDBQueryStatusLabelAccuracy_ErrPtrToNilError verifies that when
// errPtr points to a nil error value (as in the defer pattern), the status
// label is still "ok".
//
// **Property 8: DB Query Status Label Accuracy (pointer-to-nil variant)**
// **Validates: Requirements 7.2**
func TestTrackDBQueryStatusLabelAccuracy_ErrPtrToNilError(t *testing.T) {
	f := func(op, table string) bool {
		promReg := prometheus.NewRegistry()
		reg := observability.New(promReg)

		start := time.Now()
		// errPtr is non-nil but *errPtr is nil — this is the typical defer usage:
		//   var err error
		//   defer s.obs.TrackDBQuery("query", "games", time.Now(), &err)
		var noErr error
		reg.TrackDBQuery(op, table, start, &noErr)

		status, found := findStatusLabel(t, promReg, op, table)
		if !found {
			t.Logf("no observation found for op=%q table=%q", op, table)
			return false
		}
		if status != "ok" {
			t.Logf("pointer-to-nil-error: status=%q (want %q), op=%q table=%q", status, "ok", op, table)
			return false
		}
		return true
	}

	cfg := &quick.Config{MaxCount: 200}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Property 8 (pointer-to-nil error) failed: %v", err)
	}
}
