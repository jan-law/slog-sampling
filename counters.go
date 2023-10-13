package slogsampling

import (
	"sync"
	"sync/atomic"
	"time"

	"log/slog"
)

const countersPerLevel = 4096

type counter struct {
	resetAt atomic.Int64
	counter atomic.Uint64
}

func (c *counter) Inc(t time.Time, tick time.Duration) uint64 {
	tn := t.UnixNano()
	resetAfter := c.resetAt.Load()
	if resetAfter > tn {
		return c.counter.Add(1)
	}

	c.counter.Store(1)

	newResetAfter := tn + tick.Nanoseconds()
	if !c.resetAt.CompareAndSwap(resetAfter, newResetAfter) {
		// We raced with another goroutine trying to reset, and it also reset
		// the counter to 1, so we need to reincrement the counter.
		return c.counter.Add(1)
	}

	return 1
}

type counters struct {
	customLevelCounter map[slog.Level]*[countersPerLevel]counter
	debugCtr           [countersPerLevel]*counter
	infoCtr            [countersPerLevel]*counter
	warnCtr            [countersPerLevel]*counter
	errCtr             [countersPerLevel]*counter
	mu                 sync.RWMutex
}

func newCounters() *counters {
	return &counters{
		debugCtr: [countersPerLevel]*counter{},
		infoCtr:  [countersPerLevel]*counter{},
		warnCtr:  [countersPerLevel]*counter{},
		errCtr:   [countersPerLevel]*counter{},
	}
}

func (cs *counters) get(lvl slog.Level, record slog.Record) *counter {
	key := record.Message
	hash := fnv32a(key)
	n := hash % countersPerLevel

	switch lvl {
	case slog.LevelDebug:
		return cs.debugCtr[n]
	case slog.LevelInfo:
		return cs.infoCtr[n]
	case slog.LevelWarn:
		return cs.warnCtr[n]
	case slog.LevelError:
		return cs.errCtr[n]
	default:
		cs.mu.RLock()
		defer cs.mu.RUnlock()
		_, ok := cs.customLevelCounter[lvl]
		if !ok {
			cs.customLevelCounter[lvl] = &[countersPerLevel]counter{}
		}

		return &cs.customLevelCounter[lvl][n]
	}

}
