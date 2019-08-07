package swaparoo

import (
	"sync/atomic"
)

// counter is a simple implementation of a wait group.
type counter struct {
	count int32  // number of pending locks
	sem   uint32 // semaphore for waiters to wait for locks
}

const (
	counterMax    = 1 << 30     // max num of outstanding Acquire calls possible
	negCounterMax = -counterMax // helps inlining
)

// Acquire increments the counter and blocks Wait calls.
func (c *counter) Acquire() {
	atomic.AddInt32(&c.count, 1)
}

// Release decrements the counter and unblocks Wait if the counter is empty.
func (c *counter) Release() {
	if atomic.AddInt32(&c.count, -1) == negCounterMax {
		runtime_Semrelease(&c.sem, false)
	}
}

// Wait blocks until the counter is zero.
func (c *counter) Wait() {
	if atomic.AddInt32(&c.count, negCounterMax) != negCounterMax {
		runtime_Semacquire(&c.sem)
	}
	atomic.AddInt32(&c.count, counterMax)
}
