package swaparoo

// Token keeps track of a Tracker's generation and prevents a Pending Wait from
// unblocking until it is Released.
type Token struct {
	ctr  *counter
	gen  uint64
	hint uint64
}

// Release invalidates the Token and must be called exactly once.
func (t Token) Release() { t.ctr.Release() }

// Gen reports the current generation of the Tracker.
func (t Token) Gen() uint64 { return t.gen }

// Hint reports a thread hint associated with the Token.
func (t Token) Hint() uint64 { return t.hint }
