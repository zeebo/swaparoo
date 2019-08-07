package swaparoo

import (
	"sync/atomic"
	"unsafe"
)

const (
	cacheLine  = 128 // typical size of a cache line (with prefetch)
	numMutexes = 16  // number of padded counters per page to shard
)

// Tracker allows one to acquire Tokens that come with a monotonically increasing
// generation number. It does so in a scalable way, and optimizes for the case where
// not many changes to the generation happen.
type Tracker struct {
	gen uint64
	_   [cacheLine - unsafe.Sizeof(uint64(0))]byte
	buf [2][numMutexes]struct {
		mu counter
		_  [cacheLine - unsafe.Sizeof(counter{})]byte
	}
}

// NewTracker constructs a Tracker.
func NewTracker() *Tracker {
	return new(Tracker)
}

// getLock returns the pth mutex modulo the maximum number of mutexes from the
// buffer chosen by gen's parity.
func (t *Tracker) getLock(gen uint64, p uint) *counter {
	return &t.buf[gen%2][p%numMutexes].mu
}

// Acquire returns a Token that can be used to inspect the current generation.
// It must be Released before an Increment of the Token's generation can complete.
func (t *Tracker) Acquire() Token {
	for {
		p := uint(procPin())
		procUnpin()

		gen := atomic.LoadUint64(&t.gen)
		ctr := t.getLock(gen, p)
		ctr.Acquire()
		if atomic.LoadUint64(&t.gen) == gen {
			return Token{ctr: ctr, gen: gen, p: p}
		}
		ctr.Release()
	}
}

// Increment increments the generation and waits for any Acquired Tokens with the
// old generation to be Released. It reports what the old generation was. It is
// not safe to call concurrently.
func (t *Tracker) Increment() uint64 {
	gen := atomic.AddUint64(&t.gen, 1) - 1
	for p := uint(0); p < numMutexes; p++ {
		t.getLock(gen, p).Wait()
	}
	return gen
}
