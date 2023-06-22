package swaparoo

import (
	"sync"
	"sync/atomic"
	"unsafe"
)

var thread uint64
var threadPool = sync.Pool{
	New: func() interface{} { return uint64(atomic.AddUint64(&thread, 1)) },
}

// Tracker allows one to acquire Tokens that come with a monotonically increasing
// generation number. It does so in a scalable way, and optimizes for the case where
// not many changes to the generation happen. The zero value is safe to use.
type Tracker struct {
	page unsafe.Pointer // *counterPage
	mu   sync.Mutex     // serializes Increment
}

// Acquire returns a Token that can be used to inspect the current generation.
// It must be Released before an Increment of the Token's generation can complete.
// It is safe to be called concurrently.
func (t *Tracker) Acquire() Token {
	// determine which counter we're going to hold
	pi := threadPool.Get()
	threadPool.Put(pi)
	p, _ := pi.(uint64)

	// load the current generation, allocating it if it's nil.
	page := (*counterPage)(atomic.LoadPointer(&t.page))
	if page == nil {
		page = newCounterPage()
		page.gen = 0
		if !atomic.CompareAndSwapPointer(&t.page, nil, unsafe.Pointer(page)) {
			page.Release()
			page = (*counterPage)(atomic.LoadPointer(&t.page))
		}
	}

	for {
		// acquire the counter
		ctr := &page.ctrs[p%numCounters].ctr
		ctr.Acquire()

		// double check that the generation didn't change to ensure that any
		// Increment calls are aware of our potential outstanding Token.
		pageNext := (*counterPage)(atomic.LoadPointer(&t.page))
		if page == pageNext {
			return Token{ctr: ctr, gen: page.gen, p: p}
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
	page := (*counterPage)(atomic.LoadPointer(&t.page))
	if page == nil {
		page = newCounterPage()
		page.gen = 0
		if !atomic.CompareAndSwapPointer(&t.page, nil, unsafe.Pointer(page)) {
			page.Release()
			page = (*counterPage)(atomic.LoadPointer(&t.page))
		}
	}

	// store in the next page. no need to CAS because we know we're the only
	// possible writer to the page variable since Acquire only does a CAS from
	// nil and the mutex serializes calls to Increment.
	nextPage := newCounterPage()
	nextPage.gen = page.gen + 1
	atomic.StorePointer(&t.page, unsafe.Pointer(nextPage))

	t.mu.Unlock()

	// no one else can be reading/writing to the page header now, so we are safe
	// to do unsynchronized reads. synchronization is provided by the atomic
	// loads and stores of the page pointer itself.
	return Pending{
		page: page,
		gen:  page.gen,
		pgen: page.pgen,
	}
}
