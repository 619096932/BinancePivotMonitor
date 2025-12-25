package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"example.com/binance-pivot-monitor/internal/binance"
	"example.com/binance-pivot-monitor/internal/httpapi"
	"example.com/binance-pivot-monitor/internal/monitor"
	"example.com/binance-pivot-monitor/internal/pivot"
	signalpkg "example.com/binance-pivot-monitor/internal/signal"
	"example.com/binance-pivot-monitor/internal/sse"
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
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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

	broker := sse.NewBroker[signalpkg.Signal]()
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
	mon := monitor.New(store, broker, history, cooldown)
	mon.HeartbeatEvery = *monitorHeartbeat
	go mon.Run(ctx)

	api := httpapi.New(broker, history, httpapi.ParseAllowedOrigins(*corsOrigins))
	api.PivotStatus = refresher
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
