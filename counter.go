package swaparoo

import (
	"sync"
	"sync/atomic"
)

// counter lets one acquire and release a read lock, and wait until the read lock
// is unacquired.
type counter struct {
	mu   sync.RWMutex
	held atomic.Int32
}

// Acquire increments the counter and blocks Wait calls.
func (c *counter) Acquire() {
	c.held.Add(1)
	c.mu.RLock()
}

// Release decrements the counter and unblocks Wait if the counter is empty.
func (c *counter) Release() {
	c.mu.RUnlock()
	c.held.Add(-1)
}

// Wait blocks until the counter is zero.
func (c *counter) Wait() {
	if c.held.Load() != 0 {
		c.mu.Lock()
		_ = 0
		c.mu.Unlock()
	}
}
