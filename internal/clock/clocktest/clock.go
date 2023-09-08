package clocktest

import (
	"sync"
	"time"

	"github.com/hgl/acmehugger/internal/clock"
)

type Clock struct {
	now      *time.Time
	nowMu    sync.RWMutex
	timers   []*Timer
	timersMu sync.RWMutex
}

func NewClock(now time.Time) *Clock {
	return &Clock{
		now: &now,
	}
}

func (c *Clock) Now() time.Time {
	c.nowMu.RLock()
	defer c.nowMu.RUnlock()
	return *c.now
}

func (c *Clock) Tick(d time.Duration) {
	if d < 0 {
		panic("time cannot flow backwards")
	}
	c.nowMu.Lock()
	now := c.now.Add(d)
	c.now = &now
	c.nowMu.Unlock()

	c.timersMu.RLock()
	timers := c.timers
	c.timersMu.RUnlock()
	for _, t := range timers {
		t.mu.RLock()
		fired := t.fired
		t.mu.RUnlock()
		if !fired && t.t.Before(now) {
			t.fire()
		}
	}
}

type Timer struct {
	clock *Clock
	t     time.Time
	c     chan time.Time
	fired bool
	mu    sync.RWMutex
}

func (c *Clock) NewTimer(d time.Duration) clock.Timer {
	t := &Timer{
		clock: c,
		t:     c.now.Add(d),
		c:     make(chan time.Time),
	}
	if d <= 0 {
		t.fire()
	}
	c.timersMu.Lock()
	c.timers = append(c.timers, t)
	c.timersMu.Unlock()
	return t
}

func (t *Timer) C() <-chan time.Time {
	return t.c
}

func (t *Timer) Stop() bool {
	t.c = nil
	t.mu.Lock()
	defer func() {
		t.fired = true
		t.mu.Unlock()
	}()
	return t.fired
}

func (t *Timer) fire() {
	if t.fired {
		panic("timer already fired")
	}
	close(t.c)
	t.mu.Lock()
	defer t.mu.Unlock()
	t.fired = true
}
