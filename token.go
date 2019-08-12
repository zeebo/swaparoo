package swaparoo

// Token keeps track of the Tracker's current generation and prevents changes
// to it while it is not Released.
type Token struct {
	ctr *counter
	gen uint64
	p   uint
}

// Release invalidates the Token and must be called exactly once.
func (t Token) Release() { t.ctr.Release() }

// Gen reports the current generation of the Tracker.
func (t Token) Gen() uint64 { return t.gen }

// Hint reports a thread hint associated with the Token.
func (t Token) Hint() uint { return t.p }
