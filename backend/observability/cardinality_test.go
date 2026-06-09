package observability_test

import (
	"testing"
	"testing/quick"
	"time"

	"github.com/ChessWess/backend/observability"
	"github.com/prometheus/client_golang/prometheus"
)

// TestLabelCardinalitySafety verifies that RecordMove never surfaces
// game_id or timeline_id as Prometheus label names, regardless of the
// arbitrary string values supplied for those parameters.
//
// **Validates: Requirements 2.7**
func TestLabelCardinalitySafety(t *testing.T) {
	// f is the property function: for any combination of gameID, timelineID,
	// and outcome strings, calling RecordMove must not produce metric label
	// names "game_id" or "timeline_id" in the gathered output.
	f := func(gameID, timelineID, outcome string) bool {
		// Use a fresh registry per invocation to avoid duplicate-registration
		// panics between quick.Check iterations.
		promReg := prometheus.NewRegistry()
		reg := observability.New(promReg)

		reg.RecordMove(gameID, timelineID, outcome, 10*time.Millisecond)

		mfs, err := promReg.Gather()
		if err != nil {
			// Gather can return partial results alongside an error for
			// incomplete metrics; treat any error as a property violation.
			t.Logf("Gather() error: %v", err)
			return false
		}

		for _, mf := range mfs {
			for _, m := range mf.GetMetric() {
				for _, lp := range m.GetLabel() {
					key := lp.GetName()
					if key == "game_id" || key == "timeline_id" {
						t.Logf(
							"forbidden label %q found in metric family %q (gameID=%q, timelineID=%q, outcome=%q)",
							key, mf.GetName(), gameID, timelineID, outcome,
						)
						return false
					}
				}
			}
		}

		return true
	}

	cfg := &quick.Config{MaxCount: 200}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("label cardinality safety property violated: %v", err)
	}
}
