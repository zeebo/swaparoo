package swaparoo

import (
	"sync"
	"sync/atomic"
)

// counter is a simple implementation of a wait group.
type counter struct {
	mu    sync.RWMutex
	count int32
}

// Acquire increments the counter and blocks Wait calls.
func (c *counter) Acquire() {
	atomic.AddInt32(&c.count, 1)
	c.mu.RLock()
}

// Release decrements the counter and unblocks Wait if the counter is empty.
func (c *counter) Release() {
	c.mu.RUnlock()
	atomic.AddInt32(&c.count, -1)
}

// Zero returns if the counter is not Acquired.
func (c *counter) Zero() bool {
	return atomic.LoadInt32(&c.count) == 0
}

// Wait blocks until the counter is zero.
func (c *counter) Wait() {
	c.mu.Lock()
	c.mu.Unlock()
}
