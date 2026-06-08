package observability_test

import (
	"errors"
	"math/rand"
	"sync"
	"testing"
	"testing/quick"
	"time"

	"github.com/ChessWess/backend/observability"
	"github.com/prometheus/client_golang/prometheus"
)

// outcomes and reasons mirror the label values used in metrics.go.
var (
	outcomes       = []string{"success", "conflict", "illegal", "error"}
	disconnReasons = []string{"normal", "error", "timeout"}
	dbOps          = []string{"query", "exec", "tx_begin", "tx_commit"}
	dbTables       = []string{"games", "nodes", "users", "energy"}
)

func pick(src []string, n uint8) string {
	return src[int(n)%len(src)]
}


func TestRegistryConcurrentSafety(t *testing.T) {
	promReg := prometheus.NewRegistry()
	reg := observability.New(promReg)

	const numGoroutines = 20

	f := func(
		outcomeIdx uint8,
		disconnIdx uint8,
		opIdx uint8,
		tableIdx uint8,
		latencyMicros uint16,
		hasErr bool,
	) bool {
		outcome := pick(outcomes, outcomeIdx)
		disconnReason := pick(disconnReasons, disconnIdx)
		op := pick(dbOps, opIdx)
		table := pick(dbTables, tableIdx)
		latency := time.Duration(latencyMicros) * time.Microsecond

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(i int) {
				defer wg.Done()

				// Add a tiny random jitter so goroutines interleave.
				time.Sleep(time.Duration(rand.Intn(100)) * time.Microsecond)


				reg.RecordMove("game-123", "tl-1", outcome, latency)

				reg.RecordWSConnect()
				reg.RecordWSDisconnect(disconnReason)

				// TrackDBQuery with both ok and error status paths.
				start := time.Now()
				var queryErr error
				if hasErr {
					queryErr = errors.New("simulated db error")
				}
				reg.TrackDBQuery(op, table, start, &queryErr)
			}(i)
		}

		wg.Wait()
		return true
	}

	cfg := &quick.Config{MaxCount: 50}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("concurrent Registry access produced an error: %v", err)
	}
}

// TestRegistryConcurrentHandlerSafety verifies that calling Registry.Handler()
// concurrently with record methods does not produce data races.
//
// **Validates: Requirements 1.6**
func TestRegistryConcurrentHandlerSafety(t *testing.T) {
	promReg := prometheus.NewRegistry()
	reg := observability.New(promReg)

	var wg sync.WaitGroup
	const goroutines = 10

	wg.Add(goroutines * 2)

	// Half the goroutines call record methods.
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				reg.RecordMove("g1", "t1", "success", time.Millisecond)
				reg.RecordWSConnect()
				reg.RecordWSDisconnect("normal")
				start := time.Now()
				var noErr error
				reg.TrackDBQuery("query", "games", start, &noErr)
			}
		}()
	}

	// The other half call Handler() to simulate concurrent scrape requests.
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				h := reg.Handler()
				if h == nil {
					t.Errorf("Handler() returned nil")
				}
			}
		}()
	}

	wg.Wait()
}
