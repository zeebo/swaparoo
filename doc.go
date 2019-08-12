// package swaparoo provides a scalable way to ensure there are no handles to some resource.
//
// Consider the case where you have some counters that you periodically want to read and reset.
// A lock-free way to implement this might be:
//
//	var counters [1000]uint64
//
//	func AddToCounter(n int) {
//		atomic.AddUint64(&counter[n], 1)
//	}
//
//	func Reset() (out [1000]uint64) {
//		for i := 0; i < len(counters); i++ {
//			for {
//				old := atomic.LoadUint64(&counter[i])
//				if atomic.CompareAndSwapUint64(&counter[i], old, 0) {
//					out[i] = old
//				}
//			}
//		}
//		return out
//	}
//
// This solution suffers from having to do a number of atomic operations that scales
// with the number of counters. Additionally, it does not provide a consistent snapshot
// of the values in the counters because it cannot read all of them at once. Using the
// types in this package, however, both of these problems can be solved in a lock-free
// way:
//
//	var (
//		counters [2][1000]uint64
//		tracker  = swaparoo.NewTracker()
//	)
//
//	func AddToCounter(n int) {
//		token := tracker.Acquire()
//		atomic.AddUint64(&counters[token.Gen()%2][n], 1)
//		token.Release()
//	}
//
//	func Reset() (out [1000]uint64) {
//		gen := tracker.Increment().Wait()%2
//		out, counters[gen] = counters[gen], [1000]uint64{}
//		return out
//	}
//
// The Acquire and Release calls, in the common case with no outstanding Increments, will
// read some rarely-changing shared value and modify a counter in some best-effort thread
// local storage, adding little overhead. The Increment call should be more infrequent
// becasuse it changes the shared value and reads/writes all of the thread-local counters.
// It does not stop the progress of future Acquire calls, however, allowing throughput
// on Acquire to remain high.
//
// One does not even need to use the returned generation. For example, the above could be
// written in this way:
//
//	var (
//		counters = unsafe.Pointer(new([1000]uint64))
//		tracker  = swaparoo.NewTracker()
//	)
//
//	func loadCounters() *[1000]uint64 {
//		return (*[1000]uint64)(atomic.LoadPointer(&counters))
//	}
//
//	func AddToCounter(n int) {
//		token := tracker.Acquire()
//		atomic.AddUint64(&loadCounters()[n], 1)
//		token.Release()
//	}
//
//	func Reset() (out *[1000]uint64) {
//		out = loadCounters()
//		atomic.StorePointer(&counters, unsafe.Pointer(new([1000]uint64)))
//		tracker.Increment().Wait()
//		return out
//	}
//
// That way only requires atomically writing a pointer to bump the generation, but
// since the bump happens independent of the tracker's Increment, some Tokens may
// be using the new counters buffer but count when waiting for them to finish with
// the old counters.
package swaparoo
