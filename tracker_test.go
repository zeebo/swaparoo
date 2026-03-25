package swaparoo

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/zeebo/assert"
)

func TestTracker(t *testing.T) {
	tr := new(Tracker)

	for i := range 10 {
		token := tr.Acquire()
		assert.Equal(t, token.Gen(), i)
		token.Release()
		assert.Equal(t, tr.Increment().Gen(), i)
	}

	assert.Equal(t, tr.Increment().Gen(), 10)
	assert.Equal(t, tr.Increment().Gen(), 11)
	tr.Acquire().Release()
	assert.Equal(t, tr.Increment().Gen(), 12)
	assert.Equal(t, tr.Increment().Gen(), 13)
}

func TestTrackerRace(t *testing.T) {
	num := 10000
	tr := new(Tracker)
	np := runtime.GOMAXPROCS(-1)
	ch := make(chan uint64, 100*np)
	got := make(map[uint64]struct{}, num*np+1)

	// launch a bunch of goroutines concurrently using the tracker along with
	// a goroutine to close the send channel when all the workers are done.
	var wg sync.WaitGroup
	wg.Add(2 * np)
	for range np {
		go func() {
			defer wg.Done()
			for range num {
				ch <- tr.Increment().Wait()
			}
		}()
		go func() {
			defer wg.Done()
			for i := 0; i < 10*num; i++ {
				token := tr.Acquire()
				gen := token.Gen()
				token.Release()
				ch <- gen
			}
		}()
	}
	go func() {
		wg.Wait()
		close(ch)
	}()

	// collect the results and  do one last acquire to ensure that we observed
	// a token after the last increment.
	for gen := range ch {
		got[gen] = struct{}{}
	}
	got[tr.Acquire().Gen()] = struct{}{}

	// make sure we saw every value.
	assert.Equal(t, len(got), num*np+1)
	for i := 0; i < num*np+1; i++ {
		_, ok := got[uint64(i)]
		assert.That(t, ok)
	}
}

func BenchmarkSwaparoo(b *testing.B) {
	b.Run("Acquire", func(b *testing.B) {
		tr := new(Tracker)
		b.ReportAllocs()

		for b.Loop() {
			tr.Acquire().Release()
		}
	})

	b.Run("Increment", func(b *testing.B) {
		tr := new(Tracker)
		b.ReportAllocs()

		for b.Loop() {
			tr.Increment().Wait()
		}
	})

	b.Run("Parallel", func(b *testing.B) {
		b.Run("Acquire", func(b *testing.B) {
			tr := new(Tracker)
			b.ReportAllocs()

			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					tr.Acquire().Release()
				}
			})
		})

		b.Run("Increment", func(b *testing.B) {
			var first atomic.Bool
			tr := new(Tracker)
			b.ReportAllocs()

			b.RunParallel(func(pb *testing.PB) {
				// only have a single thread call Increment
				if first.CompareAndSwap(false, true) {
					for pb.Next() {
						tr.Increment().Wait()
					}
				} else {
					for pb.Next() {
						tr.Acquire().Release()
					}
				}
			})
		})
	})
}
