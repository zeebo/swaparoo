package swaparoo

import "sync/atomic"

// Pending represents a generation of the tracker that has been Incremented
// past. When a call to Wait returns, it can be sure that no one has any
// Tokens with the same generation.
type Pending struct {
	page *counterPage
	gen  uint64
	pgen uint64
}

// Gen returns the generation the Pending is associated to.
func (p Pending) Gen() uint64 {
	return p.gen
}

// Wait blocks until all Tokens with the same generation are Released. It
// returns the generation the Pending is associated to.
func (p Pending) Wait() uint64 {
	// acquire a read lock to signal that someone intends to read information
	// from the page. we don't bother checking pgen on the page before this
	// because that will commonly not allow us to bail early: it only helps
	// for multiple calls to Wait.
	p.page.mu.RLock()

	// if pgen doesn't match our pending saved pgen, then we know that we have
	// already Waited on this page and it may even be in the pool so we can and
	// must return early. otherwise, we have to check all the counters before we
	// drop the read lock.
	if pgen := atomic.LoadUint64(&p.page.pgen); pgen != p.pgen {
		p.page.mu.RUnlock()
		return p.gen
	}
	for i := range p.page.ctrs {
		ctr := &p.page.ctrs[i].ctr
		if !ctr.Zero() {
			ctr.Wait()
		}
	}
	p.page.mu.RUnlock()

	// race to see if we're the first ones to bump the pgen. if so, we know we're
	// free to put it into the pool. we just need to acquire the write lock which
	// waits for all other readers to be finished. we drop the lock before placing
	// it in the pool to avoid possible contention. since we were successful in
	// bumping the pgen, we know that no other reads or writes will occur after we
	// have acquired the write lock at all.
	if atomic.CompareAndSwapUint64(&p.page.pgen, p.pgen, p.pgen+1) {
		p.page.mu.Lock()
		p.page.mu.Unlock()

		pagePool.Put(p.page)
	}

	return p.gen
}
