package pivot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"example.com/binance-pivot-monitor/internal/binance"
)

type Refresher struct {
	DataDir string
	Store   *Store
	Client  *binance.RESTClient
	Workers int

	mu sync.Mutex
}

func NewRefresher(dataDir string, store *Store, client *binance.RESTClient) *Refresher {
	return &Refresher{
		DataDir: dataDir,
		Store:   store,
		Client:  client,
		Workers: 16,
		mu:      sync.Mutex{},
	}
}

func (r *Refresher) pivotFilePath(period Period) (string, error) {
	switch period {
	case PeriodDaily:
		return filepath.Join(r.DataDir, "pivots", "daily.json"), nil
	case PeriodWeekly:
		return filepath.Join(r.DataDir, "pivots", "weekly.json"), nil
	default:
		return "", errors.New("unknown period")
	}
}

func (r *Refresher) LoadFromDisk() {
	for _, p := range []Period{PeriodDaily, PeriodWeekly} {
		path, err := r.pivotFilePath(p)
		if err != nil {
			continue
		}

		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var snap Snapshot
		if err := json.Unmarshal(b, &snap); err != nil {
			log.Printf("pivot load %s failed: %v", path, err)
			continue
		}
		if snap.Symbols == nil {
			continue
		}
		if err := r.Store.Swap(p, &snap); err != nil {
			log.Printf("pivot swap %s failed: %v", p, err)
			continue
		}
		log.Printf("pivot loaded %s symbols=%d updated_at=%s", p, len(snap.Symbols), snap.UpdatedAt.Format(time.RFC3339))
	}
}

func (r *Refresher) Refresh(ctx context.Context, period Period) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	interval := ""
	switch period {
	case PeriodDaily:
		interval = "1d"
	case PeriodWeekly:
		interval = "1w"
	default:
		return errors.New("unknown period")
	}

	ctxSymbols, cancelSymbols := context.WithTimeout(ctx, 20*time.Second)
	defer cancelSymbols()

	symbols, err := r.Client.ExchangeInfoUSDTPERP(ctxSymbols)
	if err != nil {
		return err
	}

	type result struct {
		symbol string
		lv     Levels
		err    error
	}

	jobs := make(chan string)
	results := make(chan result, r.Workers)

	workers := r.Workers
	if workers <= 0 {
		workers = 16
	}

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for sym := range jobs {
				if ctx.Err() != nil {
					return
				}
				ctxKline, cancel := context.WithTimeout(ctx, 15*time.Second)
				h, l, c, err := r.Client.PrevKline(ctxKline, sym, interval)
				cancel()
				if err != nil {
					results <- result{symbol: sym, err: err}
					continue
				}
				lv, err := Calculate(h, l, c)
				results <- result{symbol: sym, lv: lv, err: err}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	go func() {
		defer close(jobs)
		for _, sym := range symbols {
			select {
			case <-ctx.Done():
				return
			case jobs <- sym:
			}
		}
	}()

	levelsBySymbol := make(map[string]Levels, len(symbols))
	fail := 0
	for res := range results {
		if res.err != nil {
			fail++
			continue
		}
		levelsBySymbol[res.symbol] = res.lv
	}

	expected := len(symbols)
	minCount := expected / 2
	if minCount < 1 {
		minCount = 1
	}
	if oldSnap, _ := r.Store.Snapshot(period); oldSnap != nil {
		oldMin := len(oldSnap.Symbols) * 8 / 10
		if oldMin > minCount {
			minCount = oldMin
		}
	}
	if len(levelsBySymbol) < minCount {
		return fmt.Errorf("pivots computed too few symbols: got=%d expected=%d min=%d", len(levelsBySymbol), expected, minCount)
	}

	snap := &Snapshot{
		Period:    period,
		UpdatedAt: time.Now().UTC(),
		Symbols:   levelsBySymbol,
	}

	b, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}

	path, err := r.pivotFilePath(period)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}

	if err := r.Store.Swap(period, snap); err != nil {
		return err
	}

	log.Printf("pivot refreshed %s symbols=%d fail=%d", period, len(levelsBySymbol), fail)
	return nil
}

func (r *Refresher) StartScheduler(ctx context.Context) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("UTC+8", 8*60*60)
	}

	go r.loop(ctx, PeriodDaily, loc)
	go r.loop(ctx, PeriodWeekly, loc)
}

func (r *Refresher) needsRefresh(period Period, loc *time.Location) bool {
	snap, _ := r.Store.Snapshot(period)
	if snap == nil {
		return true
	}

	now := time.Now().In(loc)

	switch period {
	case PeriodDaily:
		today8am := time.Date(now.Year(), now.Month(), now.Day(), 8, 0, 0, 0, loc)
		if !now.Before(today8am) && snap.UpdatedAt.In(loc).Before(today8am) {
			return true
		}
	case PeriodWeekly:
		today := time.Date(now.Year(), now.Month(), now.Day(), 8, 0, 0, 0, loc)
		delta := (int(time.Monday) - int(now.Weekday()) + 7) % 7
		thisMonday8am := today.AddDate(0, 0, -((7 - delta) % 7))
		if now.Weekday() == time.Monday {
			thisMonday8am = today
		} else {
			thisMonday8am = today.AddDate(0, 0, -int(now.Weekday()-time.Monday))
		}
		if !now.Before(thisMonday8am) && snap.UpdatedAt.In(loc).Before(thisMonday8am) {
			return true
		}
	}
	return false
}

func (r *Refresher) loop(ctx context.Context, period Period, loc *time.Location) {
	for {
		if ctx.Err() != nil {
			return
		}

		// 检查数据是否过期，过期则立即刷新
		if r.needsRefresh(period, loc) {
			log.Printf("pivot %s data is stale, refreshing now", period)
			ctxRun, cancel := context.WithTimeout(ctx, 10*time.Minute)
			err := r.Refresh(ctxRun, period)
			cancel()
			if err != nil {
				log.Printf("pivot refresh %s failed: %v", period, err)
			}
		}

		now := time.Now().In(loc)
		next := nextRun(now, period, loc)
		d := time.Until(next)
		if d < time.Minute {
			d = time.Minute // 避免过于频繁的循环
		}

		t := time.NewTimer(d)
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
		}
	}
}

func nextRun(now time.Time, period Period, loc *time.Location) time.Time {
	switch period {
	case PeriodDaily:
		t := time.Date(now.Year(), now.Month(), now.Day(), 8, 0, 0, 0, loc)
		if !now.Before(t) {
			t = t.AddDate(0, 0, 1)
		}
		return t
	case PeriodWeekly:
		today := time.Date(now.Year(), now.Month(), now.Day(), 8, 0, 0, 0, loc)
		delta := (int(time.Monday) - int(now.Weekday()) + 7) % 7
		t := today.AddDate(0, 0, delta)
		if delta == 0 && !now.Before(today) {
			t = t.AddDate(0, 0, 7)
		}
		return t
	default:
		return now.Add(24 * time.Hour)
	}
}

type PivotPeriodStatus struct {
	UpdatedAt     *time.Time `json:"updated_at"`
	NextRefreshAt time.Time  `json:"next_refresh_at"`
	SecondsUntil  int64      `json:"seconds_until"`
	IsStale       bool       `json:"is_stale"`
	SymbolCount   int        `json:"symbol_count"`
}

type PivotStatusResponse struct {
	Daily  PivotPeriodStatus `json:"daily"`
	Weekly PivotPeriodStatus `json:"weekly"`
}

func (r *Refresher) PivotStatus() PivotStatusResponse {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("UTC+8", 8*60*60)
	}

	now := time.Now().In(loc)

	buildStatus := func(period Period) PivotPeriodStatus {
		snap, _ := r.Store.Snapshot(period)
		next := nextRun(now, period, loc)
		status := PivotPeriodStatus{
			NextRefreshAt: next.UTC(),
			SecondsUntil:  int64(time.Until(next).Seconds()),
			IsStale:       r.needsRefresh(period, loc),
		}
		if snap != nil {
			t := snap.UpdatedAt
			status.UpdatedAt = &t
			status.SymbolCount = len(snap.Symbols)
		}
		return status
	}

	return PivotStatusResponse{
		Daily:  buildStatus(PeriodDaily),
		Weekly: buildStatus(PeriodWeekly),
	}
}
