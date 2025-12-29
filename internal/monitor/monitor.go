package monitor

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strconv"
	"sync/atomic"
	"time"

	"example.com/binance-pivot-monitor/internal/binance"
	"example.com/binance-pivot-monitor/internal/kline"
	"example.com/binance-pivot-monitor/internal/pattern"
	"example.com/binance-pivot-monitor/internal/pivot"
	signalpkg "example.com/binance-pivot-monitor/internal/signal"
	"example.com/binance-pivot-monitor/internal/sse"
	"github.com/gorilla/websocket"
)

type Monitor struct {
	PivotStore     *pivot.Store
	Broker         *sse.Broker[signalpkg.Signal]
	History        *signalpkg.History
	Cooldown       *signalpkg.Cooldown
	Source         string
	HeartbeatEvery time.Duration

	// K-line pattern recognition
	KlineStore      *kline.Store
	PatternDetector *pattern.Detector
	PatternHistory  *pattern.History
	PatternBroker   *sse.Broker[pattern.Signal]
	SignalCombiner  *signalpkg.Combiner

	idCounter   uint64
	lastPrice   map[string]float64
	symbolsSeen int64
}

func New(pivotStore *pivot.Store, broker *sse.Broker[signalpkg.Signal], history *signalpkg.History, cooldown *signalpkg.Cooldown) *Monitor {
	return &Monitor{
		PivotStore: pivotStore,
		Broker:     broker,
		History:    history,
		Cooldown:   cooldown,
		Source:     "markPrice",
		lastPrice:  make(map[string]float64),
	}
}

// MonitorConfig holds configuration for the monitor.
type MonitorConfig struct {
	PivotStore      *pivot.Store
	Broker          *sse.Broker[signalpkg.Signal]
	History         *signalpkg.History
	Cooldown        *signalpkg.Cooldown
	KlineStore      *kline.Store
	PatternDetector *pattern.Detector
	PatternHistory  *pattern.History
	PatternBroker   *sse.Broker[pattern.Signal]
	SignalCombiner  *signalpkg.Combiner
}

// NewWithConfig creates a new monitor with full configuration.
func NewWithConfig(cfg MonitorConfig) *Monitor {
	m := &Monitor{
		PivotStore:      cfg.PivotStore,
		Broker:          cfg.Broker,
		History:         cfg.History,
		Cooldown:        cfg.Cooldown,
		KlineStore:      cfg.KlineStore,
		PatternDetector: cfg.PatternDetector,
		PatternHistory:  cfg.PatternHistory,
		PatternBroker:   cfg.PatternBroker,
		SignalCombiner:  cfg.SignalCombiner,
		Source:          "markPrice",
		lastPrice:       make(map[string]float64),
	}

	// Set up kline close callback for pattern detection
	if m.KlineStore != nil && m.PatternDetector != nil {
		m.KlineStore.SetOnClose(m.onKlineClose)
	}

	return m
}

func decodeMarkPriceEvents(b []byte) ([]binance.MarkPriceEvent, bool) {
	if ev, ok := parseMarkPriceEventsJSON(b); ok {
		return ev, true
	}
	if dec, ok := maybeDecompress(b); ok {
		if ev, ok := parseMarkPriceEventsJSON(dec); ok {
			return ev, true
		}
	}
	return nil, false
}

func parseMarkPriceEventsJSON(b []byte) ([]binance.MarkPriceEvent, bool) {
	bb := cleanJSONBytes(b)
	if len(bb) == 0 {
		return nil, false
	}

	if bb[0] == '[' {
		var events []binance.MarkPriceEvent
		if err := json.Unmarshal(bb, &events); err == nil {
			return events, true
		}
		if cand := trimAfterJSONEnd(bb); cand != nil {
			if err := json.Unmarshal(cand, &events); err == nil {
				return events, true
			}
		}
	}

	if bb[0] == '{' {
		var wrapped struct {
			Data []binance.MarkPriceEvent `json:"data"`
		}
		if err := json.Unmarshal(bb, &wrapped); err == nil && wrapped.Data != nil {
			return wrapped.Data, true
		}
		if cand := trimAfterJSONEnd(bb); cand != nil {
			if err := json.Unmarshal(cand, &wrapped); err == nil && wrapped.Data != nil {
				return wrapped.Data, true
			}
		}

		var single binance.MarkPriceEvent
		if err := json.Unmarshal(bb, &single); err == nil {
			if single.Symbol != "" && single.MarkPrice != "" {
				return []binance.MarkPriceEvent{single}, true
			}
		}
		if cand := trimAfterJSONEnd(bb); cand != nil {
			if err := json.Unmarshal(cand, &single); err == nil {
				if single.Symbol != "" && single.MarkPrice != "" {
					return []binance.MarkPriceEvent{single}, true
				}
			}
		}
	}

	return nil, false
}

func cleanJSONBytes(b []byte) []byte {
	bb := bytes.TrimSpace(b)
	for len(bb) > 0 {
		last := bb[len(bb)-1]
		if last < 0x20 {
			bb = bb[:len(bb)-1]
			continue
		}
		break
	}
	return bb
}

func trimAfterJSONEnd(bb []byte) []byte {
	idx := bytes.LastIndexAny(bb, "]}")
	if idx < 0 {
		return nil
	}
	cand := cleanJSONBytes(bb[:idx+1])
	if len(cand) == 0 || len(cand) == len(bb) {
		return nil
	}
	return cand
}

func maybeDecompress(b []byte) ([]byte, bool) {
	bb := bytes.TrimSpace(b)
	if len(bb) == 0 {
		return nil, false
	}
	if bb[0] == '{' || bb[0] == '[' {
		return nil, false
	}

	if len(bb) >= 2 && bb[0] == 0x1f && bb[1] == 0x8b {
		if out, ok := decompressWith(func() (io.ReadCloser, error) {
			return gzip.NewReader(bytes.NewReader(bb))
		}); ok {
			return out, true
		}
	}

	if len(bb) >= 2 && bb[0] == 0x78 {
		if out, ok := decompressWith(func() (io.ReadCloser, error) {
			return zlib.NewReader(bytes.NewReader(bb))
		}); ok {
			return out, true
		}
	}

	if out, ok := decompressWith(func() (io.ReadCloser, error) {
		return io.NopCloser(flate.NewReader(bytes.NewReader(bb))), nil
	}); ok {
		return out, true
	}

	return nil, false
}

func decompressWith(newReader func() (io.ReadCloser, error)) ([]byte, bool) {
	r, err := newReader()
	if err != nil {
		return nil, false
	}
	defer r.Close()
	out, err := io.ReadAll(io.LimitReader(r, 10<<20))
	if err != nil || len(out) == 0 {
		return nil, false
	}
	return out, true
}

func (m *Monitor) Run(ctx context.Context) {
	backoff := 1 * time.Second
	for {
		if ctx.Err() != nil {
			return
		}

		conn, _, err := binance.DialMarkPriceArr1s(ctx)
		if err != nil {
			log.Printf("monitor ws dial failed: %v", err)
			if !sleepContext(ctx, backoff) {
				return
			}
			backoff = minDuration(backoff*2, 30*time.Second)
			continue
		}

		log.Printf("monitor ws connected")
		backoff = 1 * time.Second

		err = m.readLoop(ctx, conn)
		_ = conn.Close()
		if err != nil && ctx.Err() == nil {
			log.Printf("monitor ws read loop exit: %v", err)
		}

		if !sleepContext(ctx, backoff) {
			return
		}
		backoff = minDuration(backoff*2, 30*time.Second)
	}
}

func (m *Monitor) readLoop(ctx context.Context, conn *websocket.Conn) error {
	_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	hbEvery := m.HeartbeatEvery
	var hbMsgs int64
	var hbEvents int64
	var hbUnmarshalErr int64
	var hbLastMsgUnixNano int64
	atomic.StoreInt64(&hbLastMsgUnixNano, time.Now().UnixNano())

	hbDone := make(chan struct{})
	if hbEvery > 0 {
		go func() {
			t := time.NewTicker(hbEvery)
			defer t.Stop()
			for {
				select {
				case <-hbDone:
					return
				case <-ctx.Done():
					return
				case <-t.C:
					msgs := atomic.SwapInt64(&hbMsgs, 0)
					events := atomic.SwapInt64(&hbEvents, 0)
					bad := atomic.SwapInt64(&hbUnmarshalErr, 0)
					last := time.Unix(0, atomic.LoadInt64(&hbLastMsgUnixNano))
					symbols := atomic.LoadInt64(&m.symbolsSeen)
					log.Printf("monitor ws heartbeat msgs=%d events=%d unmarshal_err=%d last_msg_ago=%s symbols_seen=%d", msgs, events, bad, time.Since(last).Round(time.Second), symbols)
				}
			}
		}()
	}
	defer close(hbDone)

	done := make(chan struct{})
	go func() {
		t := time.NewTicker(20 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-t.C:
				_ = conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(5*time.Second))
			}
		}
	}()
	defer close(done)

	unmarshalSampleLogged := 0
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		mt, b, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		if hbEvery > 0 {
			atomic.AddInt64(&hbMsgs, 1)
			atomic.StoreInt64(&hbLastMsgUnixNano, time.Now().UnixNano())
		}

		events, ok := decodeMarkPriceEvents(b)
		if !ok {
			if hbEvery > 0 {
				atomic.AddInt64(&hbUnmarshalErr, 1)
			}
			if unmarshalSampleLogged < 3 {
				unmarshalSampleLogged += 1
				head := b
				if len(head) > 32 {
					head = head[:32]
				}
				tail := b
				if len(tail) > 32 {
					tail = tail[len(tail)-32:]
				}
				log.Printf("monitor ws unmarshal sample mt=%d len=%d head_hex=%x tail_hex=%x", mt, len(b), head, tail)
				trimmed := bytes.TrimSpace(b)
				if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
					prefix := string(trimmed)
					if len(prefix) > 160 {
						prefix = prefix[:160]
					}
					log.Printf("monitor ws unmarshal sample prefix=%q", prefix)
				}

				bb := cleanJSONBytes(b)
				if len(bb) > 0 && (bb[0] == '[' || bb[0] == '{') {
					var tmp []binance.MarkPriceEvent
					if err0 := json.Unmarshal(bb, &tmp); err0 != nil {
						log.Printf("monitor ws unmarshal err_clean=%v", err0)
					}
					if cand := trimAfterJSONEnd(bb); cand != nil {
						if err1 := json.Unmarshal(cand, &tmp); err1 != nil {
							log.Printf("monitor ws unmarshal err_trim=%v", err1)
						}
					}
				}
			}
			continue
		}
		if hbEvery > 0 {
			atomic.AddInt64(&hbEvents, int64(len(events)))
		}

		now := time.Now().UTC()
		for _, ev := range events {
			price, err := strconv.ParseFloat(ev.MarkPrice, 64)
			if err != nil {
				continue
			}
			ts := now
			if ev.EventTime > 0 {
				ts = time.UnixMilli(ev.EventTime).UTC()
			}
			m.onPrice(ev.Symbol, price, ts)
		}
	}
}

func (m *Monitor) onPrice(symbol string, price float64, ts time.Time) {
	prev, ok := m.lastPrice[symbol]
	m.lastPrice[symbol] = price
	if !ok {
		atomic.AddInt64(&m.symbolsSeen, 1)
	}

	// Update kline data (if enabled)
	if m.KlineStore != nil {
		m.KlineStore.Update(symbol, price, ts)
	}

	// Check pivot levels (only if we have previous price)
	if !ok {
		return
	}

	m.checkPeriod(symbol, pivot.PeriodDaily, prev, price, ts)
	m.checkPeriod(symbol, pivot.PeriodWeekly, prev, price, ts)
}

func (m *Monitor) checkPeriod(symbol string, period pivot.Period, prev, price float64, ts time.Time) {
	lv, ok := m.PivotStore.GetLevels(period, symbol)
	if !ok {
		return
	}

	// Check all 11 pivot levels: PP, R1-R5, S1-S5
	m.checkLevel(symbol, period, "PP", lv.PP, prev, price, ts)

	m.checkLevel(symbol, period, "R1", lv.R1, prev, price, ts)
	m.checkLevel(symbol, period, "R2", lv.R2, prev, price, ts)
	m.checkLevel(symbol, period, "R3", lv.R3, prev, price, ts)
	m.checkLevel(symbol, period, "R4", lv.R4, prev, price, ts)
	m.checkLevel(symbol, period, "R5", lv.R5, prev, price, ts)

	m.checkLevel(symbol, period, "S1", lv.S1, prev, price, ts)
	m.checkLevel(symbol, period, "S2", lv.S2, prev, price, ts)
	m.checkLevel(symbol, period, "S3", lv.S3, prev, price, ts)
	m.checkLevel(symbol, period, "S4", lv.S4, prev, price, ts)
	m.checkLevel(symbol, period, "S5", lv.S5, prev, price, ts)
}

func (m *Monitor) checkLevel(symbol string, period pivot.Period, levelName string, levelPrice float64, prev, price float64, ts time.Time) {
	if levelPrice <= 0 {
		return
	}

	if prev < levelPrice && price >= levelPrice {
		m.emit(symbol, period, levelName, price, "up", ts)
		return
	}

	if prev > levelPrice && price <= levelPrice {
		m.emit(symbol, period, levelName, price, "down", ts)
		return
	}
}

func (m *Monitor) emit(symbol string, period pivot.Period, levelName string, price float64, direction string, ts time.Time) {
	key := symbol + "|" + string(period) + "|" + levelName
	if m.Cooldown != nil {
		if !m.Cooldown.Allow(key, ts) {
			return
		}
	}

	log.Printf("signal %s %s %s %s price=%g", symbol, period, levelName, direction, price)

	seq := atomic.AddUint64(&m.idCounter, 1)
	id := fmt.Sprintf("%d-%d", ts.UnixNano(), seq)

	sig := signalpkg.Signal{
		ID:          id,
		Symbol:      symbol,
		Period:      string(period),
		Level:       levelName,
		Price:       price,
		Direction:   direction,
		TriggeredAt: ts,
		Source:      m.Source,
	}

	if m.History != nil {
		m.History.Add(sig)
	}
	if m.Broker != nil {
		m.Broker.Publish(sig)
	}

	// Add to signal combiner for correlation with pattern signals
	if m.SignalCombiner != nil {
		m.SignalCombiner.AddPivotSignal(sig)
	}
}

func sleepContext(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return true
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

// onKlineClose is called when a kline closes.
// It triggers pattern detection asynchronously.
// klines is a deep copy snapshot, safe for async use.
func (m *Monitor) onKlineClose(symbol string, klines []kline.Kline) {
	// Skip if pattern detection is not enabled
	if m.PatternDetector == nil {
		return
	}

	// Skip if we don't have pivot data for this symbol (per design: Property 11)
	// This limits detection to symbols we're actively monitoring
	hasPivot := false
	if _, ok := m.PivotStore.GetLevels(pivot.PeriodDaily, symbol); ok {
		hasPivot = true
	}
	if !hasPivot {
		if _, ok := m.PivotStore.GetLevels(pivot.PeriodWeekly, symbol); ok {
			hasPivot = true
		}
	}
	if !hasPivot {
		return
	}

	// Detect patterns with timing (Requirement 7.5: warn if >100ms)
	startTime := time.Now()
	patterns := m.PatternDetector.Detect(klines)
	elapsed := time.Since(startTime)
	if elapsed > 100*time.Millisecond {
		log.Printf("pattern detection slow: symbol=%s elapsed=%v", symbol, elapsed)
	}

	if len(patterns) == 0 {
		return
	}

	// Get kline close time from the last kline
	var klineTime time.Time
	if len(klines) > 0 {
		lastKline := klines[len(klines)-1]
		klineTime = lastKline.CloseTime
		if klineTime.IsZero() {
			klineTime = lastKline.OpenTime
		}
	}

	// Emit signals for each detected pattern
	for _, p := range patterns {
		m.emitPatternSignal(symbol, p, klineTime)
	}
}

// emitPatternSignal creates and emits a pattern signal.
func (m *Monitor) emitPatternSignal(symbol string, p pattern.DetectedPattern, klineTime time.Time) {
	sig := pattern.NewSignal(symbol, p.Type, p.Direction, p.Confidence, klineTime)

	log.Printf("pattern %s %s %s confidence=%d", symbol, p.Type, p.Direction, p.Confidence)

	// Record to history
	if m.PatternHistory != nil {
		if err := m.PatternHistory.Add(sig); err != nil {
			log.Printf("pattern history add error: %v", err)
		}
	}

	// Publish via SSE
	if m.PatternBroker != nil {
		m.PatternBroker.Publish(sig)
	}

	// Add to signal combiner for correlation with pivot signals
	if m.SignalCombiner != nil {
		m.SignalCombiner.AddPatternSignal(sig)
	}
}
