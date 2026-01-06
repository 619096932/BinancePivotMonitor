package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"example.com/binance-pivot-monitor/internal/ranking"
)

// parseCompareDuration parses compare parameter to duration.
// Supported values: 5m, 15m, 30m, 1h, 6h, 24h
// Returns ok=false for unsupported non-empty values.
func parseCompareDuration(s string) (time.Duration, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return 0, true // Use previous snapshot
	}
	switch s {
	case "5m":
		return 5 * time.Minute, true
	case "15m":
		return 15 * time.Minute, true
	case "30m":
		return 30 * time.Minute, true
	case "1h":
		return 1 * time.Hour, true
	case "6h":
		return 6 * time.Hour, true
	case "24h":
		return 24 * time.Hour, true
	default:
		return 0, false
	}
}

// handleRankingCurrent handles GET /api/ranking/current
// Query params:
//   - type: volume|trades (default: volume)
//   - compare: 5m|15m|30m|1h|6h|24h (default: previous snapshot)
//   - limit: int (default: 100)
func (s *Server) handleRankingCurrent(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query()

	// Parse type parameter
	rankType := strings.ToLower(q.Get("type"))
	if rankType == "" {
		rankType = ranking.RankingTypeVolume
	} else if rankType != ranking.RankingTypeTrades && rankType != ranking.RankingTypeVolume {
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"error":"invalid type parameter (volume or trades)"}`))
		return
	} else if rankType != ranking.RankingTypeTrades {
		rankType = ranking.RankingTypeVolume
	}

	// Parse compare parameter
	compare, ok := parseCompareDuration(q.Get("compare"))
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"error":"invalid compare parameter"}`))
		return
	}

	// Parse limit parameter
	limit := 100
	if limitStr := q.Get("limit"); limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
			limit = v
		}
	}

	opts := ranking.CurrentOptions{
		Type:    rankType,
		Compare: compare,
		Limit:   limit,
	}

	var resp *ranking.CurrentResponse
	if s.RankingStore == nil {
		resp = &ranking.CurrentResponse{Items: []ranking.RankingItem{}}
	} else {
		resp = s.RankingStore.GetCurrent(opts)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// handleRankingHistory handles GET /api/ranking/history/{symbol}
func (s *Server) handleRankingHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Extract symbol from path: /api/ranking/history/{symbol}
	path := strings.TrimPrefix(r.URL.Path, "/api/ranking/history/")
	symbol := strings.ToUpper(strings.TrimSpace(path))
	if symbol == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"error":"symbol parameter required"}`))
		return
	}

	var resp *ranking.HistoryResponse
	if s.RankingStore == nil {
		resp = &ranking.HistoryResponse{Symbol: symbol, Snapshots: []ranking.SymbolSnapshot{}}
	} else {
		resp = s.RankingStore.GetHistory(symbol)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// handleRankingMovers handles GET /api/ranking/movers
// Query params:
//   - type: volume|trades (default: volume)
//   - direction: up|down (required)
//   - compare: 5m|15m|30m|1h|6h|24h (default: previous snapshot)
//   - limit: int (default: 20)
func (s *Server) handleRankingMovers(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query()

	// Parse direction parameter (required)
	direction := strings.ToLower(q.Get("direction"))
	if direction != ranking.DirectionUp && direction != ranking.DirectionDown {
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"error":"direction parameter required (up or down)"}`))
		return
	}

	// Parse type parameter
	rankType := strings.ToLower(q.Get("type"))
	if rankType == "" {
		rankType = ranking.RankingTypeVolume
	} else if rankType != ranking.RankingTypeTrades && rankType != ranking.RankingTypeVolume {
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"error":"invalid type parameter (volume or trades)"}`))
		return
	} else if rankType != ranking.RankingTypeTrades {
		rankType = ranking.RankingTypeVolume
	}

	// Parse compare parameter
	compare, ok := parseCompareDuration(q.Get("compare"))
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"error":"invalid compare parameter"}`))
		return
	}

	// Parse limit parameter
	limit := 20
	if limitStr := q.Get("limit"); limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
			limit = v
		}
	}

	opts := ranking.MoversOptions{
		Type:      rankType,
		Direction: direction,
		Compare:   compare,
		Limit:     limit,
	}

	var resp *ranking.MoversResponse
	if s.RankingStore == nil {
		resp = &ranking.MoversResponse{Direction: direction, Items: []ranking.RankingItem{}}
	} else {
		resp = s.RankingStore.GetMovers(opts)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
