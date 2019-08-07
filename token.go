package swaparoo

// Token keeps track of the Tracker's current generation and prevents changes
// to it while it is not Released. Depending on how the Token was acquired, there
// may be many or only one allowed to exist at once.
type Token struct {
	ctr *counter
	gen uint64
	p   uint
}

// Release invalidates the Token and must be called exactly once.
func (t Token) Release() { t.ctr.Release() }

// Gen reports the current generation of the Tracker.
func (t Token) Gen() uint64 { return t.gen }
