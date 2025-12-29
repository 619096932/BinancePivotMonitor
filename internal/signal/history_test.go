package signal

import (
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// =============================================================================
// Task 2.2: Property Test - Signal History Capacity
// Validates: Requirements 2.1, 2.2
// =============================================================================

// TestProperty_SignalHistoryCapacity tests that history respects max capacity
// and Query respects the 4000 limit.
func TestProperty_SignalHistoryCapacity(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50
	properties := gopter.NewProperties(parameters)

	properties.Property("History respects max capacity", prop.ForAll(
		func(maxCap int, numSignals int) bool {
			if maxCap < 10 {
				maxCap = 10
			}
			if maxCap > 500 {
				maxCap = 500
			}
			if numSignals < 0 {
				numSignals = 0
			}
			if numSignals > 1000 {
				numSignals = 1000
			}

			h := NewHistory(maxCap)

			// Add signals
			for i := 0; i < numSignals; i++ {
				h.Add(Signal{
					ID:          string(rune('A' + i%26)),
					Symbol:      "TESTUSDT",
					Period:      "1d",
					Level:       "R1",
					Price:       float64(i),
					Direction:   "up",
					TriggeredAt: time.Now(),
				})
			}

			// Count should not exceed max
			count := h.Count()
			if numSignals <= maxCap {
				return count == numSignals
			}
			return count == maxCap
		},
		gen.IntRange(10, 500),
		gen.IntRange(0, 1000),
	))

	properties.Property("Query limit capped at 4000", prop.ForAll(
		func(requestedLimit int) bool {
			h := NewHistory(5000)

			// Add 4500 signals
			for i := 0; i < 4500; i++ {
				h.Add(Signal{
					ID:          string(rune('A' + i%26)),
					Symbol:      "TESTUSDT",
					Period:      "1d",
					Level:       "R1",
					Price:       float64(i),
					Direction:   "up",
					TriggeredAt: time.Now(),
				})
			}

			results := h.Query("", "", "", "", "", requestedLimit)

			// If requested > 4000, should be capped at 4000
			if requestedLimit > 4000 {
				return len(results) == 4000
			}
			// If requested <= 0, default to 200
			if requestedLimit <= 0 {
				return len(results) == 200
			}
			// Otherwise should return requested amount
			return len(results) == requestedLimit
		},
		gen.IntRange(-100, 5000),
	))

	properties.TestingRun(t)
}

// TestHistory_QueryLimit4000 tests that Query can return up to 4000 signals.
func TestHistory_QueryLimit4000(t *testing.T) {
	h := NewHistory(5000)

	// Add 4500 signals
	for i := 0; i < 4500; i++ {
		h.Add(Signal{
			ID:          string(rune('A' + i%26)),
			Symbol:      "TESTUSDT",
			Period:      "1d",
			Level:       "R1",
			Price:       float64(i),
			Direction:   "up",
			TriggeredAt: time.Now(),
		})
	}

	// Query with limit 4000
	results := h.Query("", "", "", "", "", 4000)
	if len(results) != 4000 {
		t.Errorf("expected 4000 results, got %d", len(results))
	}

	// Query with limit 5000 should be capped at 4000
	results = h.Query("", "", "", "", "", 5000)
	if len(results) != 4000 {
		t.Errorf("expected 4000 results (capped), got %d", len(results))
	}

	// Query with limit 0 should default to 200
	results = h.Query("", "", "", "", "", 0)
	if len(results) != 200 {
		t.Errorf("expected 200 results (default), got %d", len(results))
	}
}
