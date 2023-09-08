package clock

import (
	"sync"
	"time"
)

type Clock interface {
	Now() time.Time
	NewTimer(time.Duration) Timer
}

type Timer interface {
	C() <-chan time.Time
	Stop() bool
}

type timer struct {
	t *time.Timer
}

func (t *timer) C() <-chan time.Time {
	return t.t.C
}

func (t *timer) Stop() bool {
	return t.t.Stop()
}

type clock struct{}

var defaultClock Clock
var defaultClockMu sync.RWMutex

func init() {
	defaultClockMu.Lock()
	defaultClock = clock{}
	defaultClockMu.Unlock()
}

func Default() Clock {
	defaultClockMu.RLock()
	defer defaultClockMu.RUnlock()
	return defaultClock
}

func SetDefault(c Clock) {
	defaultClockMu.Lock()
	defaultClock = c
	defaultClockMu.Unlock()
}

func Now() time.Time {
	return Default().Now()
}

func Until(t time.Time) time.Duration {
	return t.Sub(Now())
}

func NewTimer(d time.Duration) Timer {
	return Default().NewTimer(d)
}

func (clock) Now() time.Time {
	return time.Now()
}

func (clock) NewTimer(d time.Duration) Timer {
	return &timer{time.NewTimer(d)}
}
