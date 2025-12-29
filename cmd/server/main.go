package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"example.com/binance-pivot-monitor/internal/binance"
	"example.com/binance-pivot-monitor/internal/httpapi"
	"example.com/binance-pivot-monitor/internal/kline"
	"example.com/binance-pivot-monitor/internal/monitor"
	"example.com/binance-pivot-monitor/internal/pattern"
	"example.com/binance-pivot-monitor/internal/pivot"
	signalpkg "example.com/binance-pivot-monitor/internal/signal"
	"example.com/binance-pivot-monitor/internal/sse"
	"example.com/binance-pivot-monitor/internal/ticker"
)

func main() {
	addr := flag.String("addr", ":8080", "")
	dataDir := flag.String("data-dir", "data", "")
	corsOrigins := flag.String("cors-origins", "*", "")
	restBase := flag.String("binance-rest", "https://fapi.binance.com", "")
	refreshWorkers := flag.Int("refresh-workers", 16, "")
	monitorHeartbeat := flag.Duration("monitor-heartbeat", 0, "")
	historyMax := flag.Int("history-max", 20000, "")
	historyFile := flag.String("history-file", "signals/history.jsonl", "")
	tickerBatchInterval := flag.Duration("ticker-batch-interval", 500*time.Millisecond, "")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Read pattern recognition config from environment
	patternEnabled := getEnvBool("PATTERN_ENABLED", true)
	klineCount := getEnvInt("KLINE_COUNT", 12)
	klineInterval := getEnvDurationOrMinutes("KLINE_INTERVAL", 5*time.Minute)
	patternMinConfidence := getEnvInt("PATTERN_MIN_CONFIDENCE", 60) // Requirement 8: default 60
	patternHistoryFile := os.Getenv("PATTERN_HISTORY_FILE")
	if patternHistoryFile == "" {
		patternHistoryFile = "patterns/history.jsonl" // Requirement 6.2: default path
	}
	patternCryptoMode := getEnvBool("PATTERN_CRYPTO_MODE", true)
	patternHistoryMax := getEnvInt("PATTERN_HISTORY_MAX", 1000) // Requirement 6.3: default 1000

	// Log configuration
	log.Printf("config: addr=%s data-dir=%s", *addr, *dataDir)
	log.Printf("config: pattern_enabled=%v kline_count=%d kline_interval=%v", patternEnabled, klineCount, klineInterval)
	log.Printf("config: pattern_min_confidence=%d pattern_crypto_mode=%v pattern_history_max=%d", patternMinConfidence, patternCryptoMode, patternHistoryMax)
	log.Printf("config: pattern_history_file=%s", patternHistoryFile)

	store := pivot.NewStore()
	rest := binance.NewRESTClient(*restBase)
	refresher := pivot.NewRefresher(*dataDir, store, rest)
	refresher.Workers = *refreshWorkers
	refresher.LoadFromDisk()

	go func() {
		ctxInit, cancel := context.WithTimeout(ctx, 15*time.Minute)
		defer cancel()

		if snap, _ := store.Snapshot(pivot.PeriodDaily); snap == nil {
			_ = refresher.Refresh(ctxInit, pivot.PeriodDaily)
		}
		if snap, _ := store.Snapshot(pivot.PeriodWeekly); snap == nil {
			_ = refresher.Refresh(ctxInit, pivot.PeriodWeekly)
		}
	}()

	refresher.StartScheduler(ctx)

	signalBroker := sse.NewBroker[signalpkg.Signal]()
	history := signalpkg.NewHistory(*historyMax)
	if *historyFile != "" {
		path := *historyFile
		if !filepath.IsAbs(path) {
			path = filepath.Join(*dataDir, path)
		}
		if err := history.EnablePersistence(path); err != nil {
			log.Fatalf("history persistence init error: %v", err)
		}
	}
	cooldown := signalpkg.NewCooldown(30 * time.Minute)

	// Initialize pattern recognition components (if enabled)
	var klineStore *kline.Store
	var patternDetector *pattern.Detector
	var patternHistory *pattern.History
	var patternBroker *sse.Broker[pattern.Signal]
	var signalCombiner *signalpkg.Combiner

	if patternEnabled {
		klineStore = kline.NewStore(klineInterval, klineCount)
		patternDetector = pattern.NewDetector(pattern.DetectorConfig{
			MinConfidence:      patternMinConfidence,
			HighEfficiencyOnly: false,
			CryptoMode:         patternCryptoMode,
			GapThreshold:       0.001,
		})
		patternBroker = sse.NewBroker[pattern.Signal]()
		signalCombiner = signalpkg.NewCombiner(15 * time.Minute)

		// Initialize pattern history
		var err error
		histPath := patternHistoryFile
		if !filepath.IsAbs(histPath) {
			histPath = filepath.Join(*dataDir, histPath)
		}
		patternHistory, err = pattern.NewHistory(histPath, patternHistoryMax)
		if err != nil {
			log.Printf("pattern history init warning: %v (continuing without persistence)", err)
			patternHistory, _ = pattern.NewHistory("", 10000)
		}

		log.Printf("pattern recognition enabled: kline_count=%d interval=%v", klineCount, klineInterval)
	}

	// Create monitor with full config
	mon := monitor.NewWithConfig(monitor.MonitorConfig{
		PivotStore:      store,
		Broker:          signalBroker,
		History:         history,
		Cooldown:        cooldown,
		KlineStore:      klineStore,
		PatternDetector: patternDetector,
		PatternHistory:  patternHistory,
		PatternBroker:   patternBroker,
		SignalCombiner:  signalCombiner,
	})
	mon.HeartbeatEvery = *monitorHeartbeat
	go mon.Run(ctx)

	// Ticker monitor
	tickerStore := ticker.NewStore()
	tickerMon := ticker.NewMonitor(tickerStore)
	tickerMon.BatchInterval = *tickerBatchInterval
	go tickerMon.Run(ctx)

	api := httpapi.New(signalBroker, history, httpapi.ParseAllowedOrigins(*corsOrigins))
	api.PivotStatus = refresher
	api.PivotStore = store
	api.TickerStore = tickerStore
	api.TickerMonitor = tickerMon
	api.PatternBroker = patternBroker
	api.PatternHistory = patternHistory
	api.KlineStore = klineStore
	api.SignalCombiner = signalCombiner

	srv := &http.Server{
		Addr:              *addr,
		Handler:           api.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		ctxShutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctxShutdown)
	}()

	log.Printf("http listening on %s", *addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("http server error: %v", err)
	}
}

// getEnvBool reads a boolean from environment variable.
func getEnvBool(key string, defaultVal bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	v = strings.ToLower(v)
	return v == "true" || v == "1" || v == "yes" || v == "on"
}

// getEnvInt reads an integer from environment variable.
func getEnvInt(key string, defaultVal int) int {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	if i, err := strconv.Atoi(v); err == nil {
		return i
	}
	return defaultVal
}

// getEnvDuration reads a duration from environment variable.
func getEnvDuration(key string, defaultVal time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	if d, err := time.ParseDuration(v); err == nil {
		return d
	}
	return defaultVal
}

// getEnvDurationOrMinutes reads a duration from environment variable.
// Supports both "5m" format and plain number "5" (interpreted as minutes).
func getEnvDurationOrMinutes(key string, defaultVal time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	// Try parsing as duration first (e.g., "5m", "1h")
	if d, err := time.ParseDuration(v); err == nil {
		return d
	}
	// Try parsing as plain number (interpreted as minutes)
	if mins, err := strconv.Atoi(v); err == nil && mins > 0 {
		return time.Duration(mins) * time.Minute
	}
	return defaultVal
}
