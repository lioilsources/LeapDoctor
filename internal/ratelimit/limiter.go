// Package ratelimit provides rate limiting and loop detection for destructive operations.
package ratelimit

import (
	"fmt"
	"sync"
	"time"
)

type rollbackRecord struct {
	tool string
	args string
	at   time.Time
}

// Limiter tracks destructive operations and rollbacks to prevent runaway damage.
type Limiter struct {
	mu                sync.Mutex
	destructiveOps    []time.Time
	rollbacks         []rollbackRecord
	maxOpsPerWindow   int
	maxRollbacks      int
	window            time.Duration
	rollbackCooldown  time.Duration
	lockedOut         bool
}

func New(maxOps, maxRollbacks int) *Limiter {
	return &Limiter{
		maxOpsPerWindow:  maxOps,
		maxRollbacks:     maxRollbacks,
		window:           30 * time.Minute,
		rollbackCooldown: 10 * time.Minute,
	}
}

// CheckDestructive verifies that a destructive operation is allowed.
func (l *Limiter) CheckDestructive() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.lockedOut {
		return fmt.Errorf("too many rollbacks in this session - read-only mode active. Restart leapdoctor to reset")
	}

	l.pruneOps()

	if len(l.destructiveOps) >= l.maxOpsPerWindow {
		return fmt.Errorf("max %d destructive operations per %v reached", l.maxOpsPerWindow, l.window)
	}

	l.destructiveOps = append(l.destructiveOps, time.Now())
	return nil
}

// RecordRollback logs that an operation was rolled back.
func (l *Limiter) RecordRollback(tool, args string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.rollbacks = append(l.rollbacks, rollbackRecord{tool: tool, args: args, at: time.Now()})

	// Count recent rollbacks
	cutoff := time.Now().Add(-l.window)
	recent := 0
	for _, r := range l.rollbacks {
		if r.at.After(cutoff) {
			recent++
		}
	}
	if recent >= l.maxRollbacks {
		l.lockedOut = true
	}
}

// WasRolledBack checks if the same tool+args was rolled back recently.
func (l *Limiter) WasRolledBack(tool, args string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := time.Now().Add(-l.rollbackCooldown)
	for _, r := range l.rollbacks {
		if r.tool == tool && r.args == args && r.at.After(cutoff) {
			return true
		}
	}
	return false
}

func (l *Limiter) pruneOps() {
	cutoff := time.Now().Add(-l.window)
	fresh := l.destructiveOps[:0]
	for _, t := range l.destructiveOps {
		if t.After(cutoff) {
			fresh = append(fresh, t)
		}
	}
	l.destructiveOps = fresh
}
