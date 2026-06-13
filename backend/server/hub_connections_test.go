package server

import (
	"fmt"
	"testing"
	"testing/quick"
)

// TestHubActiveConnectionsAccuracy is a property-based test for Property 9:
// Hub Connection Count Accuracy.
//
// It generates a random set of N join events and M leave events (with M ≤ N),
// directly populates the hub's rooms map under the mutex (white-box, same
// package), and asserts that ActiveConnections() equals N - M and is never
// negative.
//
// **Property 9: Hub Connection Count Accuracy**
// **Validates: Requirements 6.1, 6.2**
func TestHubActiveConnectionsAccuracy(t *testing.T) {
	// f receives uint8 values to keep N and M small so tests run fast.
	// joinCount is N; leaveCount raw is M' where M = M' % (N+1) ensures M ≤ N.
	f := func(joinCount uint8, leaveCountRaw uint8) bool {
		n := int(joinCount)
		// Ensure M ≤ N.
		m := 0
		if n > 0 {
			m = int(leaveCountRaw) % (n + 1)
		}

		h := NewHub(nil)

		// Create N clients and add them directly under the lock, simulating
		// what Hub.Run does for join events — without starting a goroutine.
		clients := make([]*Client, n)
		for i := 0; i < n; i++ {
			c := &Client{
				hub:    h,
				gameID: fmt.Sprintf("game-%d", i%3), // spread across a few rooms
				userID: fmt.Sprintf("user-%d", i),
				send:   make(chan []byte, 1),
			}
			clients[i] = c

			h.mu.Lock()
			if h.rooms[c.gameID] == nil {
				h.rooms[c.gameID] = &GameRoom{
					clients: make(map[*Client]struct{}),
				}
			}
			h.rooms[c.gameID].clients[c] = struct{}{}
			h.mu.Unlock()
		}

		// After N joins, ActiveConnections must equal N.
		if got := h.ActiveConnections(); got != n {
			t.Errorf("after %d joins: ActiveConnections() = %d, want %d", n, got, n)
			return false
		}

		// Remove M clients directly under the lock, simulating leave events.
		for i := 0; i < m; i++ {
			c := clients[i]
			h.mu.Lock()
			if room, ok := h.rooms[c.gameID]; ok {
				delete(room.clients, c)
				if len(room.clients) == 0 {
					delete(h.rooms, c.gameID)
				}
			}
			h.mu.Unlock()
		}

		// After M leaves, ActiveConnections must equal N - M and be non-negative.
		want := n - m
		got := h.ActiveConnections()

		if got < 0 {
			t.Errorf("ActiveConnections() = %d, must never be negative (n=%d, m=%d)", got, n, m)
			return false
		}
		if got != want {
			t.Errorf("ActiveConnections() = %d after %d joins and %d leaves, want %d", got, n, m, want)
			return false
		}

		return true
	}

	cfg := &quick.Config{MaxCount: 200}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("property 9 (Hub connection count accuracy) failed: %v", err)
	}
}

// TestHubActiveConnectionsZeroOnEmpty verifies the base case: a newly
// created hub with no clients reports zero active connections.
func TestHubActiveConnectionsZeroOnEmpty(t *testing.T) {
	h := NewHub(nil)
	if got := h.ActiveConnections(); got != 0 {
		t.Errorf("empty hub: ActiveConnections() = %d, want 0", got)
	}
}
