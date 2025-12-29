package httpapi

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	"example.com/binance-pivot-monitor/internal/kline"
	"example.com/binance-pivot-monitor/internal/pattern"
	"example.com/binance-pivot-monitor/internal/pivot"
	signalpkg "example.com/binance-pivot-monitor/internal/signal"
	"example.com/binance-pivot-monitor/internal/sse"
	"example.com/binance-pivot-monitor/internal/ticker"
)

//go:embed static/*
var staticFS embed.FS

type Server struct {
	SignalBroker   *sse.Broker[signalpkg.Signal]
	History        *signalpkg.History
	AllowedOrigins []string
	PivotStatus    PivotStatusProvider
	PivotStore     *pivot.Store
	TickerStore    *ticker.Store
	TickerMonitor  *ticker.Monitor

	// Pattern recognition
	PatternBroker   *sse.Broker[pattern.Signal]
	PatternHistory  *pattern.History
	KlineStore      *kline.Store
	SignalCombiner  *signalpkg.Combiner
}

func New(signalBroker *sse.Broker[signalpkg.Signal], history *signalpkg.History, allowedOrigins []string) *Server {
	return &Server{SignalBroker: signalBroker, History: history, AllowedOrigins: allowedOrigins}
}

type PivotStatusProvider interface {
	PivotStatus() pivot.PivotStatusResponse
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleDashboard)
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/api/sse", s.handleSSE)
	mux.HandleFunc("/api/history", s.handleHistory)
	mux.HandleFunc("/api/pivot-status", s.handlePivotStatus)
	mux.HandleFunc("/api/pivots/", s.handlePivots)
	mux.HandleFunc("/api/tickers", s.handleTickers)
	mux.HandleFunc("/api/patterns", s.handlePatterns)
	mux.HandleFunc("/api/klines", s.handleKlines)
	mux.HandleFunc("/api/klines/stats", s.handleKlineStats)
	mux.HandleFunc("/api/runtime", s.handleRuntime)

	// 嵌入的静态文件
	staticContent, _ := fs.Sub(staticFS, "static")
	mux.Handle("/static/app.js", http.StripPrefix("/static/", http.FileServer(http.FS(staticContent))))

	// 外部静态文件（图标等）
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	return s.cors(mux)
}

func (s *Server) handleTickers(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if s.TickerStore == nil {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{}"))
		return
	}

	// 可选：按 symbols 过滤
	q := r.URL.Query()
	symbolsParam := q.Get("symbols")

	var data map[string]*ticker.Ticker
	if symbolsParam != "" {
		symbols := strings.Split(symbolsParam, ",")
		data = s.TickerStore.GetBySymbols(symbols)
	} else {
		data = s.TickerStore.GetAll()
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}

// handlePatterns returns pattern signal history.
// GET /api/patterns?limit=100&symbol=BTCUSDT&pattern=hammer&direction=bullish
func (s *Server) handlePatterns(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if s.PatternHistory == nil {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
		return
	}

	q := r.URL.Query()
	symbol := q.Get("symbol")
	patternType := q.Get("pattern")
	direction := q.Get("direction")
	limitStr := q.Get("limit")

	limit := 100
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
			limit = v
		}
	}

	opts := pattern.QueryOptions{
		Symbol:    symbol,
		Pattern:   pattern.PatternType(patternType),
		Direction: pattern.Direction(direction),
		Limit:     limit,
	}

	res := s.PatternHistory.Query(opts)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

// handleKlines returns kline data for a symbol (for debugging).
// GET /api/klines?symbol=BTCUSDT
func (s *Server) handleKlines(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if s.KlineStore == nil {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("null"))
		return
	}

	q := r.URL.Query()
	symbol := q.Get("symbol")
	if symbol == "" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"symbol parameter required"}`))
		return
	}

	klines, ok := s.KlineStore.GetAllKlines(symbol)
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(klines)
}

// handleKlineStats returns statistics about kline data in memory.
// GET /api/klines/stats
func (s *Server) handleKlineStats(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if s.KlineStore == nil {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"enabled":false}`))
		return
	}

	stats := s.KlineStore.Stats()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(stats)
}

// RuntimeStats contains runtime statistics.
type RuntimeStats struct {
	Goroutines     int     `json:"goroutines"`
	HeapMB         float64 `json:"heap_mb"`
	SysMB          float64 `json:"sys_mb"`
	NumGC          uint32  `json:"num_gc"`
	KlineSymbols   int     `json:"kline_symbols"`
	Patterns       int     `json:"patterns"`
	Signals        int     `json:"signals"`
	Symbols        int     `json:"symbols"` // unique symbols in signal history
	Uptime         string  `json:"uptime"`
	SSESubscribers int     `json:"sse_subscribers"`
	Version        string  `json:"version"`
}

// Version can be set at build time via -ldflags
var Version = "dev"

var startTime = time.Now()

// handleRuntime returns runtime statistics.
// GET /api/runtime
func (s *Server) handleRuntime(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	stats := RuntimeStats{
		Goroutines: runtime.NumGoroutine(),
		HeapMB:     float64(m.HeapAlloc) / 1024 / 1024,
		SysMB:      float64(m.Sys) / 1024 / 1024,
		NumGC:      m.NumGC,
		Uptime:     time.Since(startTime).Round(time.Second).String(),
		Version:    Version,
	}

	if s.KlineStore != nil {
		stats.KlineSymbols = s.KlineStore.SymbolCount()
	}
	if s.PatternHistory != nil {
		stats.Patterns = s.PatternHistory.Count()
	}
	if s.History != nil {
		stats.Signals = s.History.Count()
		stats.Symbols = s.History.SymbolCount()
	}
	if s.SignalBroker != nil {
		stats.SSESubscribers = s.SignalBroker.SubscriberCount()
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(stats)
}

func (s *Server) handlePivotStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if s.PivotStatus == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	resp := s.PivotStatus.PivotStatus()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// PivotResponse is the response for /api/pivots/{symbol}
type PivotResponse struct {
	Symbol string        `json:"symbol"`
	Daily  *pivot.Levels `json:"daily,omitempty"`
	Weekly *pivot.Levels `json:"weekly,omitempty"`
}

// handlePivots returns pivot levels for a specific symbol.
// GET /api/pivots/{symbol}?period=1d|1w (optional, returns both if omitted)
func (s *Server) handlePivots(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if s.PivotStore == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"pivot store not available"}`))
		return
	}

	// Extract symbol from path: /api/pivots/{symbol}
	path := strings.TrimPrefix(r.URL.Path, "/api/pivots/")
	symbol := strings.ToUpper(strings.TrimSpace(path))
	if symbol == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"error":"symbol parameter required"}`))
		return
	}

	q := r.URL.Query()
	period := strings.ToLower(q.Get("period"))

	resp := PivotResponse{Symbol: symbol}

	// Get daily levels
	if period == "" || period == "1d" || period == "daily" {
		if levels, ok := s.PivotStore.GetLevels(pivot.PeriodDaily, symbol); ok {
			resp.Daily = &levels
		}
	}

	// Get weekly levels
	if period == "" || period == "1w" || period == "weekly" {
		if levels, ok := s.PivotStore.GetLevels(pivot.PeriodWeekly, symbol); ok {
			resp.Weekly = &levels
		}
	}

	// Return 404 if no data found
	if resp.Daily == nil && resp.Weekly == nil {
		w.WriteHeader(http.StatusNotFound)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"error":"no pivot data found for symbol"}`))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if s.History == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	q := r.URL.Query()
	getFirstCI := func(key string) string {
		if v := q.Get(key); v != "" {
			return v
		}
		for k, vs := range q {
			if strings.EqualFold(k, key) && len(vs) > 0 {
				return vs[0]
			}
		}
		return ""
	}
	getAllCI := func(key string) string {
		var all []string
		for k, vs := range q {
			if strings.EqualFold(k, key) {
				all = append(all, vs...)
			}
		}
		return strings.Join(all, ",")
	}

	symbol := getFirstCI("symbol")
	period := getFirstCI("period")
	level := getAllCI("level")
	if level == "" {
		level = getAllCI("levels")
	}
	direction := getFirstCI("direction")
	source := getFirstCI("source")
	limitStr := getFirstCI("limit")
	limit := 200
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil {
			limit = v
		}
	}

	res := s.History.Query(symbol, period, level, direction, source, limit)

	// Enrich signals with related pattern information from PatternHistory
	if s.PatternHistory != nil {
		type EnrichedSignal struct {
			signalpkg.Signal
			RelatedPattern *RelatedPatternInfo `json:"related_pattern,omitempty"`
		}

		enriched := make([]EnrichedSignal, len(res))
		for i, sig := range res {
			enriched[i] = EnrichedSignal{Signal: sig}

			// Find related patterns for this symbol within 60 minutes (before or after signal)
			patterns := s.PatternHistory.QueryBySymbolAndTime(sig.Symbol, sig.TriggeredAt, 60*time.Minute)
			if len(patterns) > 0 {
				pat := patterns[0] // Use the closest pattern

				// Determine correlation strength
				correlation := "moderate"
				if pat.Direction == pattern.DirectionNeutral {
					correlation = "moderate"
				} else {
					pivotUp := sig.Direction == "up"
					patternBullish := pat.Direction == pattern.DirectionBullish
					if (pivotUp && patternBullish) || (!pivotUp && !patternBullish) {
						correlation = "strong"
					} else {
						correlation = "weak"
					}
				}

				// Calculate time difference
				timeDiff := sig.TriggeredAt.Sub(pat.DetectedAt)
				timeDiffStr := formatTimeDiff(timeDiff)

				enriched[i].RelatedPattern = &RelatedPatternInfo{
					ID:             pat.ID,
					Pattern:        string(pat.Pattern),
					PatternCN:      pat.PatternCN,
					Direction:      string(pat.Direction),
					Confidence:     pat.Confidence,
					UpPercent:      pat.UpPercent,
					DownPercent:    pat.DownPercent,
					EfficiencyRank: pat.EfficiencyRank,
					Correlation:    correlation,
					DetectedAt:     pat.DetectedAt,
					Count:          len(patterns),
					TimeDiff:       timeDiffStr,
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(enriched)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

// RelatedPatternInfo contains pattern information for enriched signals.
type RelatedPatternInfo struct {
	ID             string    `json:"id"`
	Pattern        string    `json:"pattern"`
	PatternCN      string    `json:"pattern_cn"`
	Direction      string    `json:"direction"`
	Confidence     int       `json:"confidence"`
	UpPercent      int       `json:"up_percent"`
	DownPercent    int       `json:"down_percent"`
	EfficiencyRank string    `json:"efficiency_rank"`
	Correlation    string    `json:"correlation"`
	DetectedAt     time.Time `json:"detected_at"`
	Count          int       `json:"count"`     // Number of patterns in time window
	TimeDiff       string    `json:"time_diff"` // Human readable time difference
}

// formatTimeDiff formats a duration as a human readable string (e.g., "5m ago", "1h 30m ago")
func formatTimeDiff(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds前", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm前", int(d.Minutes()))
	}
	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	if mins > 0 {
		return fmt.Sprintf("%dh%dm前", hours, mins)
	}
	return fmt.Sprintf("%dh前", hours)
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// 从嵌入的文件系统读取 index.html
	data, err := staticFS.ReadFile("static/index.html")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if s.SignalBroker == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// 订阅信号
	signalCh := s.SignalBroker.Subscribe(256)
	defer s.SignalBroker.Unsubscribe(signalCh)

	// 订阅 ticker（如果可用）
	var tickerCh chan ticker.TickerBatch
	if s.TickerMonitor != nil {
		tickerCh = s.TickerMonitor.Subscribe(64)
		defer s.TickerMonitor.Unsubscribe(tickerCh)
	}

	// 订阅 pattern 信号（如果可用）
	var patternCh chan pattern.Signal
	if s.PatternBroker != nil {
		patternCh = s.PatternBroker.Subscribe(256)
		defer s.PatternBroker.Unsubscribe(patternCh)
	}

	_, _ = fmt.Fprintf(w, ": connected %s\n\n", time.Now().UTC().Format(time.RFC3339))
	flusher.Flush()

	keepAlive := time.NewTicker(15 * time.Second)
	defer keepAlive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return

		case <-keepAlive.C:
			_, _ = fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()

		case sig, ok := <-signalCh:
			if !ok {
				return
			}
			b, err := json.Marshal(sig)
			if err != nil {
				continue
			}
			_, _ = fmt.Fprintf(w, "event: signal\n")
			_, _ = fmt.Fprintf(w, "data: %s\n\n", strings.ReplaceAll(string(b), "\n", ""))
			flusher.Flush()

		case batch, ok := <-tickerCh:
			if !ok {
				tickerCh = nil
				continue
			}
			b, err := json.Marshal(batch)
			if err != nil {
				continue
			}
			_, _ = fmt.Fprintf(w, "event: ticker\n")
			_, _ = fmt.Fprintf(w, "data: %s\n\n", strings.ReplaceAll(string(b), "\n", ""))
			flusher.Flush()

		case pat, ok := <-patternCh:
			if !ok {
				patternCh = nil
				continue
			}
			b, err := json.Marshal(pat)
			if err != nil {
				continue
			}
			_, _ = fmt.Fprintf(w, "event: pattern\n")
			_, _ = fmt.Fprintf(w, "data: %s\n\n", strings.ReplaceAll(string(b), "\n", ""))
			flusher.Flush()
		}
	}
}

func ParseAllowedOrigins(v string) []string {
	v = strings.TrimSpace(v)
	if v == "" {
		return []string{"*"}
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		return []string{"*"}
	}
	return out
}

func (s *Server) cors(next http.Handler) http.Handler {
	allowed := s.AllowedOrigins
	if len(allowed) == 0 {
		allowed = []string{"*"}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}

		allowOrigin := ""
		for _, o := range allowed {
			if o == "*" {
				allowOrigin = "*"
				break
			}
			if o == origin {
				allowOrigin = origin
				break
			}
		}

		if allowOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
			w.Header().Add("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
