package fetch

import (
	"context"
	"sync"
	"time"

	"github.com/alyshmahell/matchmedia/src/mod/config"
)

type Circuit struct {
	mu     sync.Mutex
	streak map[string]int
	until  map[string]time.Time
	wait   map[string]int
}

func NewCircuit() *Circuit {
	return &Circuit{
		streak: map[string]int{},
		until:  map[string]time.Time{},
		wait:   map[string]int{},
	}
}

// Wait blocks until name's pause has elapsed. A cancelled context ends the wait.
func (c *Circuit) Wait(ctx context.Context, name string) error {
	if c == nil || name == "" {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	for {
		c.mu.Lock()
		until, ok := c.until[name]
		if ok && !time.Now().Before(until) {
			delete(c.until, name)
			ok = false
		}
		c.mu.Unlock()
		if !ok {
			return nil
		}
		timer := time.NewTimer(time.Until(until))
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (c *Circuit) Cooling(name string) bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	until, ok := c.until[name]
	if !ok {
		return false
	}
	if time.Now().Before(until) {
		return true
	}
	delete(c.until, name)
	return false
}

func (c *Circuit) Fail(name string, limit, minMS, maxMS int) {
	if c == nil || limit <= 0 || name == "" {
		return
	}
	if minMS < 1 {
		minMS = 1
	}
	if maxMS < minMS {
		maxMS = minMS
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.streak[name]++
	if c.streak[name] < limit {
		return
	}
	wait := c.wait[name]
	if wait == 0 {
		wait = minMS
	} else {
		wait *= 2
		if wait > maxMS || wait <= 0 {
			wait = maxMS
		}
	}
	c.wait[name] = wait
	lo := wait / 2
	if lo < 1 {
		lo = 1
	}
	c.until[name] = time.Now().Add(config.JitterMS(lo, wait))
	c.streak[name] = 0
}

func (c *Circuit) OK(name string) {
	if c == nil || name == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.streak[name] = 0
	delete(c.wait, name)
	delete(c.until, name)
}
