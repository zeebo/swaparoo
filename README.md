# swaparoo

    import "github.com/zeebo/swaparoo"

package swaparoo provides a scalable way to ensure there are no handles to some
resource.

Consider the case where you have some counters that you periodically want to
read and reset. A lock-free way to implement this might be:

```go
var counters [1000]uint64

func AddToCounter(n int) {
	atomic.AddUint64(&counter[n], 1)
}

func Reset() (out [1000]uint64) {
	for i := 0; i < len(counters); i++ {
		for {
			old := atomic.LoadUint64(&counter[i])
			if atomic.CompareAndSwapUint64(&counter[i], old, 0) {
				out[i] = old
			}
		}
	}
	return out
}
```

This solution suffers from having to do a number of atomic operations that
scales with the number of counters. Additionally, it does not provide a
consistent snapshot of the values in the counters because it cannot read all of
them at once. Using the types in this package, however, both of these problems
can be solved in a lock-free way:

```go
var (
	counters [2][1000]uint64
	tracker  = swaparoo.NewTracker()
)

func AddToCounter(n int) {
	token := tracker.Acquire()
	atomic.AddUint64(&counters[token.Gen()%2][n], 1)
	token.Release()
}

func Reset() (out [1000]uint64) {
	gen := tracker.Increment().Wait()%2
	out, counters[gen] = counters[gen], [1000]uint64{}
	return out
}
```

The Acquire and Release calls, in the common case with no outstanding
Increments, will read some rarely-changing shared value and modify a counter in
some best-effort thread local storage, adding little overhead. The Increment
call should be more infrequent becasuse it changes the shared value and
reads/writes all of the thread-local counters. It does not stop the progress of
future Acquire calls, however, allowing throughput on Acquire to remain high.

One does not even need to use the returned generation. For example, the above
could be written in this way:

```go
var (
	counters = unsafe.Pointer(new([1000]uint64))
	tracker  = swaparoo.NewTracker()
)

func loadCounters() *[1000]uint64 {
	return (*[1000]uint64)(atomic.LoadPointer(&counters))
}

func AddToCounter(n int) {
	token := tracker.Acquire()
	atomic.AddUint64(&loadCounters()[n], 1)
	token.Release()
}

func Reset() (out *[1000]uint64) {
	out = loadCounters()
	atomic.StorePointer(&counters, unsafe.Pointer(new([1000]uint64)))
	tracker.Increment().Wait()
	return out
}
```

That way only requires atomically writing a pointer to bump the generation, but
since the bump happens independent of the tracker's Increment, some Tokens may
be using the new counters buffer but count when waiting for them to finish with
the old counters.

## Usage

#### type Pending

```go
type Pending struct {
}
```

Pending represents a generation of the tracker that has been Incremented past.
When a call to Wait returns, it can be sure that no one has any Tokens with the
same generation.

#### func (Pending) Gen

```go
func (p Pending) Gen() uint64
```
Gen returns the generation the Pending is associated to.

#### func (Pending) Wait

```go
func (p Pending) Wait() uint64
```
Wait blocks until all Tokens with the same generation are Released. It returns
the generation the Pending is associated to.

#### type Token

```go
type Token struct {
}
```

Token keeps track of the Tracker's current generation and prevents changes to it
while it is not Released.

#### func (Token) Gen

```go
func (t Token) Gen() uint64
```
Gen reports the current generation of the Tracker.

#### func (Token) Hint

```go
func (t Token) Hint() uint
```
Hint reports a thread hint associated with the Token.

#### func (Token) Release

```go
func (t Token) Release()
```
Release invalidates the Token and must be called exactly once.

#### type Tracker

```go
type Tracker struct {
}
```

Tracker allows one to acquire Tokens that come with a monotonically increasing
generation number. It does so in a scalable way, and optimizes for the case
where not many changes to the generation happen. The zero value is safe to use.

#### func (*Tracker) Acquire

```go
func (t *Tracker) Acquire() Token
```
Acquire returns a Token that can be used to inspect the current generation. It
must be Released before an Increment of the Token's generation can complete. It
is safe to be called concurrently.

#### func (*Tracker) Increment

```go
func (t *Tracker) Increment() Pending
```
Increment bumps the generation of the Tracker for future Acquire calls and
returns a Pending that can be used to Wait until all currently Acquired Tokens
with the same generation are Released. It is safe to be called concurrently.
