package signal

import (
	"sync"
	"time"
)

type Cooldown struct {
	mu   sync.Mutex
	dur  time.Duration
	last map[string]time.Time
}

func NewCooldown(dur time.Duration) *Cooldown {
	if dur <= 0 {
		dur = 30 * time.Minute
	}
	return &Cooldown{dur: dur, last: make(map[string]time.Time)}
}

func (c *Cooldown) Allow(key string, now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if t, ok := c.last[key]; ok {
		if now.Sub(t) < c.dur {
			return false
		}
	}
	c.last[key] = now
	return true
}
