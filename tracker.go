package swaparoo

import (
	"sync"
	"sync/atomic"
)

var (
	hint     uint64
	hintPool = sync.Pool{New: func() any {
		// we mod by numCounters because we're going to do that eventually
		// anyway and there is a small number optimization in the runtime to
		// avoid heap allocations.
		return (uint64(atomic.AddUint64(&hint, 1)) - 1) % numCounters
	}}
)

// Tracker allows one to acquire Tokens that come with a monotonically increasing
// generation number. It does so in a scalable way, and optimizes for the case where
// not many changes to the generation happen. The zero value is safe to use.
type Tracker struct {
	page atomic.Pointer[counterPage]
	mu   sync.Mutex // serializes Increment
}

// Acquire returns a Token that can be used to inspect the current generation.
// It must be Released before an Increment of the Token's generation can
// complete. It is safe to be called concurrently.
func (t *Tracker) Acquire() Token {
	// determine which counter we're going to hold
	hintI := hintPool.Get()
	hintPool.Put(hintI)
	hint, _ := hintI.(uint64)

	// load the current generation, allocating it if it's nil.
	page := t.page.Load()
	if page == nil {
		page = newCounterPage()
		page.gen = 0
		if !t.page.CompareAndSwap(nil, page) {
			page = t.page.Load()
		}
	}

	for {
		// before we acquire the counter, read the pgen of the page.
		pgen := page.pgen.Load()

		// acquire the counter.
		ctr := &page.ctrs[hint%numCounters].ctr
		ctr.Acquire()

		// load the page again. if it's still the same page and the pgen is the
		// same, then we know there was no way the page was reused or
		// Incremented while we were acquiring the counter, so it's safe to
		// return a Token with the generation we observed.
		pageNext := t.page.Load()
		if page == pageNext && pgen == pageNext.pgen.Load() {
			// since we have acquired ctr, we are able to read page.gen
			return Token{ctr: ctr, gen: page.gen, hint: hint}
		}

		// we lost the race, and can't safely return a Token. try again with
		// the current generation.
		ctr.Release()
		page = pageNext
	}
}

// Increment bumps the generation of the Tracker for future Acquire calls and
// returns a Pending that can be used to Wait until all currently Acquired
// Tokens with the same generation are Released. It is safe to be called
// concurrently.
func (t *Tracker) Increment() Pending {
	// serialize concurrent calls to Increment.
	t.mu.Lock()

	// read and lazily allocate the current page. we have to do this even with
	// the mutex because Acquire may be happening which ignores the mutex, so
	// we have to use CAS to synchronize.
	page := t.page.Load()
	if page == nil {
		page = newCounterPage()
		page.gen = 0
		if !t.page.CompareAndSwap(nil, page) {
			page = t.page.Load()
		}
	}

	// store in the next page. no need to CAS because we know we're the only
	// possible writer to the page variable since Acquire only does a CAS from
	// nil and the mutex serializes calls to Increment.
	nextPage := newCounterPage()
	nextPage.gen = page.gen + 1
	t.page.Store(nextPage)

	t.mu.Unlock()

	// no one else can be reading/writing to the page header now, so we are safe
	// to return. synchronization is provided by the atomic loads and stores of
	// the page pointer itself.
	return Pending{
		page: page,
		gen:  page.gen,
		pgen: page.pgen.Load(),
	}
}
