package swaparoo

import (
	"sync"
	"sync/atomic"
	"unsafe"
)

const (
	cacheLine   = 64 // typical size of a cache line
	numCounters = 64 // number of padded counters per page to shard
)

// counterPageHeader contains some metadata in the page before all of the counters.
// they are grouped into a struct that we embed so that we can easily calculate
// how many bytes to use to pad it to a cache line.
type counterPageHeader struct {
	// the generation represented by the page
	gen uint64
	// the generation of this page through the page pool. it is bumped on entry
	// into the pool to ensure that we only place it into the pool once per
	// trip through Tracker.Increment and Pending.Wait.
	pgen atomic.Uint64
	// this helps Pending.Wait ensure that we only place into the pool when
	// there are no other calls active.
	ctr counter
}

// counterPage keeps track of a generation and a set of counters tracking how many
// acquired tokens exist for the generation.
type counterPage struct {
	counterPageHeader
	_    [cacheLine - unsafe.Sizeof(counter{})]byte
	ctrs [numCounters]struct {
		ctr counter
		_   [cacheLine - unsafe.Sizeof(counter{})]byte
	}
}

// pagePool is a pool for the counterPages.
var pagePool = sync.Pool{New: func() any { return new(counterPage) }}

// newCounterPage returns an allocated counterPage. It may be reused from a pool.
func newCounterPage() *counterPage {
	page, _ := pagePool.Get().(*counterPage)
	return page
}

// Release returns the counterPage to the pool for newCounterPage. It is important
// to not perform any operations on the counter page after it has been Released.
func (p *counterPage) Release() { pagePool.Put(p) }
