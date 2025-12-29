package signal

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type History struct {
	mu           sync.RWMutex
	max          int
	signals      []Signal
	symbolsUpper []string

	fileMu    sync.Mutex
	filePath  string
	fileLines int
}

func NewHistory(max int) *History {
	if max <= 0 {
		max = 10000
	}
	return &History{max: max}
}

func (h *History) EnablePersistence(filePath string) error {
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return nil
	}

	h.fileMu.Lock()
	defer h.fileMu.Unlock()

	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return err
	}

	f, err := os.Open(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			f2, err2 := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
			if err2 != nil {
				return err2
			}
			_ = f2.Close()
			h.filePath = filePath
			h.fileLines = 0
			return nil
		}
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	capHint := 1024
	if h.max < capHint {
		capHint = h.max
	}
	loaded := make([]Signal, 0, capHint)
	lines := 0
	for scanner.Scan() {
		lines += 1
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var s Signal
		if err := json.Unmarshal([]byte(line), &s); err != nil {
			continue
		}
		loaded = append(loaded, s)
		if len(loaded) > h.max {
			loaded = loaded[len(loaded)-h.max:]
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	loadedUpper := make([]string, len(loaded))
	for i := range loaded {
		loadedUpper[i] = strings.ToUpper(loaded[i].Symbol)
	}

	h.mu.Lock()
	h.signals = loaded
	h.symbolsUpper = loadedUpper
	h.mu.Unlock()

	h.filePath = filePath
	h.fileLines = lines

	if h.fileLines > h.max*2 {
		snapshot := make([]Signal, len(loaded))
		copy(snapshot, loaded)
		if err := h.compactLocked(snapshot); err == nil {
			h.fileLines = len(snapshot)
		}
	}

	return nil
}

func (h *History) Add(s Signal) {
	if h.filePath == "" {
		upper := strings.ToUpper(s.Symbol)
		h.mu.Lock()
		h.signals = append(h.signals, s)
		h.symbolsUpper = append(h.symbolsUpper, upper)
		if len(h.signals) > h.max {
			h.signals = h.signals[len(h.signals)-h.max:]
			h.symbolsUpper = h.symbolsUpper[len(h.symbolsUpper)-h.max:]
		}
		h.mu.Unlock()
		return
	}

	h.fileMu.Lock()
	defer h.fileMu.Unlock()
	upper := strings.ToUpper(s.Symbol)

	h.mu.Lock()
	h.signals = append(h.signals, s)
	h.symbolsUpper = append(h.symbolsUpper, upper)
	if len(h.signals) > h.max {
		h.signals = h.signals[len(h.signals)-h.max:]
		h.symbolsUpper = h.symbolsUpper[len(h.symbolsUpper)-h.max:]
	}
	h.mu.Unlock()

	if err := h.appendLocked(s); err == nil {
		h.fileLines += 1
		if h.fileLines > h.max*2 {
			h.mu.RLock()
			snapshot := make([]Signal, len(h.signals))
			copy(snapshot, h.signals)
			h.mu.RUnlock()
			if err := h.compactLocked(snapshot); err == nil {
				h.fileLines = len(snapshot)
			}
		}
	}
}

func (h *History) appendLocked(s Signal) error {
	f, err := os.OpenFile(h.filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	if err := enc.Encode(s); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func (h *History) compactLocked(snapshot []Signal) error {
	tmp := h.filePath + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	bw := bufio.NewWriter(f)
	enc := json.NewEncoder(bw)
	for _, s := range snapshot {
		if err := enc.Encode(s); err != nil {
			_ = bw.Flush()
			_ = f.Close()
			return err
		}
	}
	if err := bw.Flush(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, h.filePath)
}

func (h *History) Query(symbolContains, period, level, direction, source string, limit int) []Signal {
	if limit <= 0 {
		limit = 200
	}
	if limit > 4000 {
		limit = 4000
	}

	symbolContains = strings.TrimSpace(symbolContains)
	period = strings.ToLower(strings.TrimSpace(period))
	level = strings.TrimSpace(level)
	direction = strings.ToLower(strings.TrimSpace(direction))
	source = strings.TrimSpace(source)
	symbolContainsUpper := strings.ToUpper(symbolContains)

	var levelSet map[string]struct{}
	if level != "" {
		if strings.Contains(level, ",") {
			levelSet = make(map[string]struct{})
			for _, p := range strings.Split(level, ",") {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				levelSet[strings.ToUpper(p)] = struct{}{}
			}
			level = ""
		} else {
			level = strings.ToUpper(level)
		}
	}

	h.mu.RLock()
	res := make([]Signal, 0, limit)
	for i := len(h.signals) - 1; i >= 0 && len(res) < limit; i-- {
		s := h.signals[i]
		if symbolContainsUpper != "" {
			if !strings.Contains(h.symbolsUpper[i], symbolContainsUpper) {
				continue
			}
		}
		if period != "" && s.Period != period {
			continue
		}
		if level != "" && s.Level != level {
			continue
		}
		if levelSet != nil {
			if _, ok := levelSet[s.Level]; !ok {
				continue
			}
		}
		if direction != "" && s.Direction != direction {
			continue
		}
		if source != "" && !strings.EqualFold(s.Source, source) {
			continue
		}
		res = append(res, s)
	}
	h.mu.RUnlock()
	return res
}

// Count returns the number of signals in history.
func (h *History) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.signals)
}

// SymbolCount returns the number of unique symbols in history.
func (h *History) SymbolCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	seen := make(map[string]struct{})
	for _, s := range h.signals {
		seen[s.Symbol] = struct{}{}
	}
	return len(seen)
}
