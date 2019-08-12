package swaparoo

import (
	"sync/atomic"
	"unsafe"
)

const (
	cacheLine   = 64 // typical size of a cache line
	numCounters = 32 // number of padded counters per page to shard
)

// Tracker allows one to acquire Tokens that come with a monotonically increasing
// generation number. It does so in a scalable way, and optimizes for the case where
// not many changes to the generation happen. The zero value is safe to use.
type Tracker struct {
	gen uint64
	_   [cacheLine - unsafe.Sizeof(uint64(0))]byte

	buf [2][numCounters]struct {
		ctr counter
		_   [cacheLine - unsafe.Sizeof(counter{})]byte
	}
}

// getCounter returns the pth mutex modulo the maximum number of mutexes from the
// buffer chosen by gen's parity.
func (t *Tracker) getCounter(gen uint64, p uint) *counter {
	return &t.buf[gen%2][p%numCounters].ctr
}

// Acquire returns a Token that can be used to inspect the current generation.
// It must be Released before an Increment of the Token's generation can complete.
func (t *Tracker) Acquire() Token {
	// determine which counter we're going to hold
	p := uint(procPin())
	procUnpin()

	// load the current generation
	gen := atomic.LoadUint64(&t.gen)
	for {
		// acquire the counter
		ctr := t.getCounter(gen, p)
		ctr.Acquire()

		// double check that the generation didn't change to ensure that any
		// Increment calls are aware of our potential outstanding Token.
		genNext := atomic.LoadUint64(&t.gen)
		if gen == genNext {
			return Token{ctr: ctr, gen: gen, p: p}
		}

		// we lost the race, and can't safely return a Token. try again with
		// the current generation.
		ctr.Release()
		gen = genNext
	}
}

// Increment increments the generation and waits for any Acquired Tokens with the
// old generation to be Released. It reports what the old generation was. It is
// not safe to call concurrently.
func (t *Tracker) Increment() uint64 {
	gen := atomic.AddUint64(&t.gen, 1) - 1
	for p := uint(0); p < numCounters; p++ {
		ctr := t.getCounter(gen, p)
		if !ctr.Zero() {
			ctr.Wait()
		}
	}
	return gen
}
