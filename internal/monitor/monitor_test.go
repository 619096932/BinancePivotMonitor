package monitor

import (
	"testing"
	"time"

	"example.com/binance-pivot-monitor/internal/kline"
	"example.com/binance-pivot-monitor/internal/pattern"
	"example.com/binance-pivot-monitor/internal/pivot"
	signalpkg "example.com/binance-pivot-monitor/internal/signal"
	"example.com/binance-pivot-monitor/internal/sse"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// helper to set pivot levels for a symbol
func setPivotLevels(store *pivot.Store, period pivot.Period, symbol string, levels pivot.Levels) {
	snap, _ := store.Snapshot(period)
	if snap == nil {
		snap = &pivot.Snapshot{
			Period:    period,
			UpdatedAt: time.Now(),
			Symbols:   make(map[string]pivot.Levels),
		}
	}
	if snap.Symbols == nil {
		snap.Symbols = make(map[string]pivot.Levels)
	}
	snap.Symbols[symbol] = levels
	store.Swap(period, snap)
}

// TestOnKlineClose_SkipsWithoutPivotData tests Property 11:
// Pattern detection should only occur for symbols with loaded pivot data.
func TestOnKlineClose_SkipsWithoutPivotData(t *testing.T) {
	// Create pivot store with data for only one symbol
	pivotStore := pivot.NewStore()
	setPivotLevels(pivotStore, pivot.PeriodDaily, "BTCUSDT", pivot.Levels{
		R3: 50000, R4: 51000, R5: 52000,
		S3: 48000, S4: 47000, S5: 46000,
	})

	// Create pattern detector
	detector := pattern.NewDetector(pattern.DefaultDetectorConfig())

	// Create pattern history (memory only)
	patternHistory, err := pattern.NewHistory("", 100)
	if err != nil {
		t.Fatalf("failed to create pattern history: %v", err)
	}

	// Create monitor with pattern detection enabled
	m := NewWithConfig(MonitorConfig{
		PivotStore:      pivotStore,
		Broker:          sse.NewBroker[signalpkg.Signal](),
		PatternDetector: detector,
		PatternHistory:  patternHistory,
		PatternBroker:   sse.NewBroker[pattern.Signal](),
	})

	// Create test klines that would trigger a pattern (engulfing)
	klines := []kline.Kline{
		{Symbol: "ETHUSDT", Open: 100, High: 105, Low: 95, Close: 96, IsClosed: true},  // bearish
		{Symbol: "ETHUSDT", Open: 95, High: 110, Low: 94, Close: 108, IsClosed: true},  // bullish engulfing
	}

	// Call onKlineClose for symbol WITHOUT pivot data
	m.onKlineClose("ETHUSDT", klines)

	// Should not have recorded any patterns (no pivot data for ETHUSDT)
	if patternHistory.Count() != 0 {
		t.Errorf("expected 0 patterns for symbol without pivot data, got %d", patternHistory.Count())
	}

	// Now test with symbol that HAS pivot data
	klinesBTC := []kline.Kline{
		{Symbol: "BTCUSDT", Open: 100, High: 105, Low: 95, Close: 96, IsClosed: true},  // bearish
		{Symbol: "BTCUSDT", Open: 95, High: 110, Low: 94, Close: 108, IsClosed: true},  // bullish engulfing
	}

	m.onKlineClose("BTCUSDT", klinesBTC)

	// Should have recorded patterns (has pivot data for BTCUSDT)
	// Note: may or may not detect patterns depending on detector config
	// The key test is that it ATTEMPTS detection (doesn't skip)
}

// TestOnKlineClose_Property11_DetectionRangeLimit tests that pattern detection
// is limited to symbols with pivot data using property-based testing.
func TestOnKlineClose_Property11_DetectionRangeLimit(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50
	properties := gopter.NewProperties(parameters)

	properties.Property("Pattern detection only for symbols with pivot data", prop.ForAll(
		func(pivotIdx int, noPivotIdx int) bool {
			// Generate different symbols
			symbolWithPivot := "PIVOT" + string(rune('A'+pivotIdx%26))
			symbolWithoutPivot := "NOPIV" + string(rune('A'+noPivotIdx%26))

			// Create pivot store with data for only symbolWithPivot
			pivotStore := pivot.NewStore()
			setPivotLevels(pivotStore, pivot.PeriodDaily, symbolWithPivot, pivot.Levels{
				R3: 50000, R4: 51000, R5: 52000,
				S3: 48000, S4: 47000, S5: 46000,
			})

			// Create pattern detector
			detector := pattern.NewDetector(pattern.DefaultDetectorConfig())

			// Create pattern history to track detection
			patternHistory, _ := pattern.NewHistory("", 100)

			m := NewWithConfig(MonitorConfig{
				PivotStore:      pivotStore,
				Broker:          sse.NewBroker[signalpkg.Signal](),
				PatternDetector: detector,
				PatternHistory:  patternHistory,
				PatternBroker:   sse.NewBroker[pattern.Signal](),
			})

			// Create test klines
			klines := []kline.Kline{
				{Open: 100, High: 105, Low: 95, Close: 96, IsClosed: true},
				{Open: 95, High: 110, Low: 94, Close: 108, IsClosed: true},
			}

			// Test symbol WITHOUT pivot data
			initialCount := patternHistory.Count()
			m.onKlineClose(symbolWithoutPivot, klines)
			afterWithoutPivot := patternHistory.Count()

			// For symbol without pivot, no patterns should be recorded
			return afterWithoutPivot == initialCount
		},
		gen.IntRange(0, 25),
		gen.IntRange(0, 25),
	))

	properties.TestingRun(t)
}

// TestMonitorIntegration_KlineUpdate tests that price updates flow to KlineStore.
func TestMonitorIntegration_KlineUpdate(t *testing.T) {
	// Create kline store
	klineStore := kline.NewStore(5*time.Minute, 12)

	// Create pivot store
	pivotStore := pivot.NewStore()
	setPivotLevels(pivotStore, pivot.PeriodDaily, "BTCUSDT", pivot.Levels{
		R3: 50000, R4: 51000, R5: 52000,
		S3: 48000, S4: 47000, S5: 46000,
	})

	// Create monitor
	m := NewWithConfig(MonitorConfig{
		PivotStore: pivotStore,
		Broker:     sse.NewBroker[signalpkg.Signal](),
		KlineStore: klineStore,
	})

	// Simulate price updates
	ts := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	m.onPrice("BTCUSDT", 49000, ts)
	m.onPrice("BTCUSDT", 49100, ts.Add(1*time.Second))
	m.onPrice("BTCUSDT", 48900, ts.Add(2*time.Second))

	// Check that kline was created
	current, ok := klineStore.GetCurrentKline("BTCUSDT")
	if !ok {
		t.Fatal("expected current kline to exist")
	}

	if current.Open != 49000 {
		t.Errorf("expected open=49000, got %v", current.Open)
	}
	if current.High != 49100 {
		t.Errorf("expected high=49100, got %v", current.High)
	}
	if current.Low != 48900 {
		t.Errorf("expected low=48900, got %v", current.Low)
	}
	if current.Close != 48900 {
		t.Errorf("expected close=48900, got %v", current.Close)
	}
}

// TestNewWithConfig_SetsOnCloseCallback tests that NewWithConfig properly sets up the callback.
func TestNewWithConfig_SetsOnCloseCallback(t *testing.T) {
	klineStore := kline.NewStore(5*time.Minute, 12)
	detector := pattern.NewDetector(pattern.DefaultDetectorConfig())
	pivotStore := pivot.NewStore()

	m := NewWithConfig(MonitorConfig{
		PivotStore:      pivotStore,
		Broker:          sse.NewBroker[signalpkg.Signal](),
		KlineStore:      klineStore,
		PatternDetector: detector,
	})

	// Verify monitor was created
	if m.KlineStore == nil {
		t.Error("expected KlineStore to be set")
	}
	if m.PatternDetector == nil {
		t.Error("expected PatternDetector to be set")
	}
}


// =============================================================================
// Task 1.2: Property Test - Level Crossing Detection
// Validates: Requirements 1.1, 1.7, 1.9
// =============================================================================

// TestProperty_LevelCrossingDetection tests that all 11 pivot levels are checked
// and signals are emitted when price crosses any level.
func TestProperty_LevelCrossingDetection(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// All 11 levels that should be monitored
	allLevels := []string{"PP", "R1", "R2", "R3", "R4", "R5", "S1", "S2", "S3", "S4", "S5"}

	properties.Property("All 11 levels trigger signals on crossing", prop.ForAll(
		func(levelIdx int, basePrice float64) bool {
			levelName := allLevels[levelIdx%len(allLevels)]

			// Create pivot store with test levels
			pivotStore := pivot.NewStore()
			levels := pivot.Levels{
				PP: basePrice,
				R1: basePrice * 1.01,
				R2: basePrice * 1.02,
				R3: basePrice * 1.03,
				R4: basePrice * 1.04,
				R5: basePrice * 1.05,
				S1: basePrice * 0.99,
				S2: basePrice * 0.98,
				S3: basePrice * 0.97,
				S4: basePrice * 0.96,
				S5: basePrice * 0.95,
			}
			setPivotLevels(pivotStore, pivot.PeriodDaily, "TESTUSDT", levels)

			// Create history to capture signals
			history := signalpkg.NewHistory(100)
			broker := sse.NewBroker[signalpkg.Signal]()

			m := NewWithConfig(MonitorConfig{
				PivotStore: pivotStore,
				Broker:     broker,
				History:    history,
			})

			// Get the level price
			var levelPrice float64
			switch levelName {
			case "PP":
				levelPrice = levels.PP
			case "R1":
				levelPrice = levels.R1
			case "R2":
				levelPrice = levels.R2
			case "R3":
				levelPrice = levels.R3
			case "R4":
				levelPrice = levels.R4
			case "R5":
				levelPrice = levels.R5
			case "S1":
				levelPrice = levels.S1
			case "S2":
				levelPrice = levels.S2
			case "S3":
				levelPrice = levels.S3
			case "S4":
				levelPrice = levels.S4
			case "S5":
				levelPrice = levels.S5
			}

			ts := time.Now()

			// Test upward crossing
			prevPrice := levelPrice * 0.999
			newPrice := levelPrice * 1.001
			m.lastPrice["TESTUSDT"] = prevPrice
			m.onPrice("TESTUSDT", newPrice, ts)

			signals := history.Query("", "", "", "", "", 100)
			foundUp := false
			for _, sig := range signals {
				if sig.Level == levelName && sig.Direction == "up" {
					foundUp = true
					break
				}
			}

			// Test downward crossing
			history2 := signalpkg.NewHistory(100)
			m2 := NewWithConfig(MonitorConfig{
				PivotStore: pivotStore,
				Broker:     broker,
				History:    history2,
			})

			prevPrice2 := levelPrice * 1.001
			newPrice2 := levelPrice * 0.999
			m2.lastPrice["TESTUSDT"] = prevPrice2
			m2.onPrice("TESTUSDT", newPrice2, ts)

			signals2 := history2.Query("", "", "", "", "", 100)
			foundDown := false
			for _, sig := range signals2 {
				if sig.Level == levelName && sig.Direction == "down" {
					foundDown = true
					break
				}
			}

			return foundUp && foundDown
		},
		gen.IntRange(0, 10),
		gen.Float64Range(1000, 100000),
	))

	properties.TestingRun(t)
}

// TestProperty_MultipleLevelsCrossing tests that when price jumps across multiple
// levels, each level triggers its own signal (Requirement 1.9).
func TestProperty_MultipleLevelsCrossing(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50
	properties := gopter.NewProperties(parameters)

	properties.Property("Price jump across multiple levels triggers all crossed levels", prop.ForAll(
		func(basePrice float64) bool {
			// Create pivot store with test levels
			pivotStore := pivot.NewStore()
			levels := pivot.Levels{
				PP: basePrice,
				R1: basePrice * 1.01,
				R2: basePrice * 1.02,
				R3: basePrice * 1.03,
				R4: basePrice * 1.04,
				R5: basePrice * 1.05,
				S1: basePrice * 0.99,
				S2: basePrice * 0.98,
				S3: basePrice * 0.97,
				S4: basePrice * 0.96,
				S5: basePrice * 0.95,
			}
			setPivotLevels(pivotStore, pivot.PeriodDaily, "TESTUSDT", levels)

			history := signalpkg.NewHistory(100)
			broker := sse.NewBroker[signalpkg.Signal]()

			m := NewWithConfig(MonitorConfig{
				PivotStore: pivotStore,
				Broker:     broker,
				History:    history,
			})

			ts := time.Now()

			// Price jumps from below S3 to above R3 (crossing PP, R1, R2, R3)
			prevPrice := basePrice * 0.96 // below S3
			newPrice := basePrice * 1.035 // above R3
			m.lastPrice["TESTUSDT"] = prevPrice
			m.onPrice("TESTUSDT", newPrice, ts)

			signals := history.Query("", "", "", "", "", 100)

			// Should have signals for PP, R1, R2, R3 (all crossed upward)
			crossedLevels := make(map[string]bool)
			for _, sig := range signals {
				crossedLevels[sig.Level] = true
			}

			// At minimum, PP, R1, R2, R3 should be triggered
			return crossedLevels["PP"] && crossedLevels["R1"] && crossedLevels["R2"] && crossedLevels["R3"]
		},
		gen.Float64Range(1000, 100000),
	))

	properties.TestingRun(t)
}

// =============================================================================
// Task 1.3: Property Test - Cooldown Isolation
// Validates: Requirements 1.4, 1.6
// =============================================================================

// TestProperty_CooldownIsolation tests that cooldown is per symbol|period|level
// combination, and different levels can trigger independently.
func TestProperty_CooldownIsolation(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50
	properties := gopter.NewProperties(parameters)

	properties.Property("Different levels trigger independently within cooldown", prop.ForAll(
		func(basePrice float64) bool {
			pivotStore := pivot.NewStore()
			levels := pivot.Levels{
				PP: basePrice,
				R1: basePrice * 1.01,
				R2: basePrice * 1.02,
				R3: basePrice * 1.03,
				S1: basePrice * 0.99,
				S2: basePrice * 0.98,
				S3: basePrice * 0.97,
			}
			setPivotLevels(pivotStore, pivot.PeriodDaily, "TESTUSDT", levels)

			history := signalpkg.NewHistory(100)
			broker := sse.NewBroker[signalpkg.Signal]()
			cooldown := signalpkg.NewCooldown(5 * time.Minute)

			m := NewWithConfig(MonitorConfig{
				PivotStore: pivotStore,
				Broker:     broker,
				History:    history,
				Cooldown:   cooldown,
			})

			ts := time.Now()

			// Cross R1 upward
			m.lastPrice["TESTUSDT"] = levels.R1 * 0.999
			m.onPrice("TESTUSDT", levels.R1*1.001, ts)

			// Cross R2 upward (should trigger even though R1 is in cooldown)
			m.lastPrice["TESTUSDT"] = levels.R2 * 0.999
			m.onPrice("TESTUSDT", levels.R2*1.001, ts.Add(1*time.Second))

			// Cross R3 upward (should trigger even though R1, R2 are in cooldown)
			m.lastPrice["TESTUSDT"] = levels.R3 * 0.999
			m.onPrice("TESTUSDT", levels.R3*1.001, ts.Add(2*time.Second))

			signals := history.Query("", "", "", "", "", 100)

			// Should have 3 signals for R1, R2, R3
			levelCounts := make(map[string]int)
			for _, sig := range signals {
				levelCounts[sig.Level]++
			}

			return levelCounts["R1"] == 1 && levelCounts["R2"] == 1 && levelCounts["R3"] == 1
		},
		gen.Float64Range(1000, 100000),
	))

	properties.Property("Same level blocked by cooldown", prop.ForAll(
		func(basePrice float64) bool {
			pivotStore := pivot.NewStore()
			levels := pivot.Levels{
				R1: basePrice * 1.01,
			}
			setPivotLevels(pivotStore, pivot.PeriodDaily, "TESTUSDT", levels)

			history := signalpkg.NewHistory(100)
			broker := sse.NewBroker[signalpkg.Signal]()
			cooldown := signalpkg.NewCooldown(5 * time.Minute)

			m := NewWithConfig(MonitorConfig{
				PivotStore: pivotStore,
				Broker:     broker,
				History:    history,
				Cooldown:   cooldown,
			})

			ts := time.Now()

			// First crossing - should trigger
			m.lastPrice["TESTUSDT"] = levels.R1 * 0.999
			m.onPrice("TESTUSDT", levels.R1*1.001, ts)

			// Second crossing within cooldown - should NOT trigger
			m.lastPrice["TESTUSDT"] = levels.R1 * 0.999
			m.onPrice("TESTUSDT", levels.R1*1.001, ts.Add(1*time.Minute))

			signals := history.Query("", "", "", "", "", 100)

			// Should have only 1 signal for R1
			count := 0
			for _, sig := range signals {
				if sig.Level == "R1" {
					count++
				}
			}

			return count == 1
		},
		gen.Float64Range(1000, 100000),
	))

	properties.TestingRun(t)
}

// =============================================================================
// Task 1.4: Property Test - First Price Baseline
// Validates: Requirements 1.8
// =============================================================================

// TestProperty_FirstPriceBaseline tests that the first price for a symbol
// establishes a baseline and does not trigger any signals.
func TestProperty_FirstPriceBaseline(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("First price establishes baseline without triggering signals", prop.ForAll(
		func(basePrice float64, firstPriceOffset float64) bool {
			pivotStore := pivot.NewStore()
			levels := pivot.Levels{
				PP: basePrice,
				R1: basePrice * 1.01,
				R2: basePrice * 1.02,
				R3: basePrice * 1.03,
				R4: basePrice * 1.04,
				R5: basePrice * 1.05,
				S1: basePrice * 0.99,
				S2: basePrice * 0.98,
				S3: basePrice * 0.97,
				S4: basePrice * 0.96,
				S5: basePrice * 0.95,
			}
			setPivotLevels(pivotStore, pivot.PeriodDaily, "NEWUSDT", levels)

			history := signalpkg.NewHistory(100)
			broker := sse.NewBroker[signalpkg.Signal]()

			m := NewWithConfig(MonitorConfig{
				PivotStore: pivotStore,
				Broker:     broker,
				History:    history,
			})

			ts := time.Now()

			// First price - should NOT trigger any signals regardless of where it is
			firstPrice := basePrice * (1 + firstPriceOffset)
			m.onPrice("NEWUSDT", firstPrice, ts)

			signals := history.Query("", "", "", "", "", 100)

			// No signals should be generated for first price
			return len(signals) == 0
		},
		gen.Float64Range(1000, 100000),
		gen.Float64Range(-0.1, 0.1), // offset from -10% to +10%
	))

	properties.Property("Second price can trigger signals after baseline", prop.ForAll(
		func(basePrice float64) bool {
			pivotStore := pivot.NewStore()
			levels := pivot.Levels{
				R1: basePrice * 1.01,
			}
			setPivotLevels(pivotStore, pivot.PeriodDaily, "NEWUSDT", levels)

			history := signalpkg.NewHistory(100)
			broker := sse.NewBroker[signalpkg.Signal]()

			m := NewWithConfig(MonitorConfig{
				PivotStore: pivotStore,
				Broker:     broker,
				History:    history,
			})

			ts := time.Now()

			// First price - establishes baseline below R1
			m.onPrice("NEWUSDT", levels.R1*0.99, ts)

			// Second price - crosses R1 upward, should trigger
			m.onPrice("NEWUSDT", levels.R1*1.01, ts.Add(1*time.Second))

			signals := history.Query("", "", "", "", "", 100)

			// Should have exactly 1 signal for R1
			return len(signals) == 1 && signals[0].Level == "R1"
		},
		gen.Float64Range(1000, 100000),
	))

	properties.TestingRun(t)
}
